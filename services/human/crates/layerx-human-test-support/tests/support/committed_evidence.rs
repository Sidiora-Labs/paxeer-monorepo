use std::fmt::Write as _;
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::TcpListener;
use std::thread;
use std::time::Duration;

use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::read::{
    BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator, CommitVote,
};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerReceipt};
use layerx_human_service::activity::{
    CommittedReceiptEvidence, ConsensusAuthority, EvidenceBundle, ReceiptFact, ReceiptHeaderBinding,
};
use layerx_human_service::notify::ActivityEntryId;
use layerx_human_service::store::{PrincipalStore, Table};
use layerx_human_test_support as support;
use layerx_intents::canonical::{decode_batch_header, decode_receipt};
use layerx_proof::merkle::Proof;
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const CONFORMANCE: &str = include_str!("../fixtures/committed/vector-v4-conformance.json");
const RECORDED: &str = include_str!("../fixtures/committed/vector-v4-committed.json");

fn bytes(value: &Value) -> Vec<u8> {
    let text = value
        .as_str()
        .unwrap_or_else(|| panic!("hex string"))
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("0x prefix"));
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|i| {
            u8::from_str_radix(&text[i..i + 2], 16)
                .unwrap_or_else(|error| panic!("hex byte: {error:?}"))
        })
        .collect()
}
fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value)
        .try_into()
        .unwrap_or_else(|error| panic!("fixed-width field: {error:?}"))
}
fn time(value: &Value) -> CommitTimestamp {
    CommitTimestamp {
        seconds: value["seconds"]
            .as_i64()
            .unwrap_or_else(|| panic!("seconds")),
        nanos: value["nanos"]
            .as_u64()
            .unwrap_or_else(|| panic!("nanos"))
            .try_into()
            .unwrap_or_else(|error| panic!("u32 nanos: {error:?}")),
    }
}
fn block_id(value: &Value) -> BlockId {
    BlockId {
        hash: fixed(&value["hash"]),
        part_set_total: value["parts"]["total"]
            .as_u64()
            .unwrap_or_else(|| panic!("parts"))
            .try_into()
            .unwrap_or_else(|error| panic!("u32 parts: {error:?}")),
        part_set_hash: fixed(&value["parts"]["hash"]),
    }
}
fn header(v: &Value) -> CheckpointHeader {
    let optional = |key: &str| {
        if bytes(&v[key]).is_empty() {
            None
        } else {
            Some(fixed(&v[key]))
        }
    };
    CheckpointHeader {
        version_block: v["version"]["block"]
            .as_u64()
            .unwrap_or_else(|| panic!("version")),
        version_app: v["version"]["app"]
            .as_u64()
            .unwrap_or_else(|| panic!("app version")),
        chain_id: v["chain_id"]
            .as_str()
            .unwrap_or_else(|| panic!("chain id"))
            .to_owned(),
        height: v["height"].as_u64().unwrap_or_else(|| panic!("height")),
        time: time(&v["time"]),
        last_block_id: Some(block_id(&v["last_block_id"])),
        last_commit_hash: optional("last_commit_hash"),
        data_hash: optional("data_hash"),
        validators_hash: fixed(&v["validators_hash"]),
        next_validators_hash: fixed(&v["next_validators_hash"]),
        consensus_hash: fixed(&v["consensus_hash"]),
        app_hash: fixed(&v["app_hash"]),
        last_results_hash: optional("last_results_hash"),
        evidence_hash: optional("evidence_hash"),
        proposer_address: fixed(&v["proposer_address"]),
    }
}

fn recording() -> Value {
    let vector: Value = serde_json::from_str(CONFORMANCE)
        .unwrap_or_else(|error| panic!("committed conformance recording: {error:?}"));
    let recorded: Value = serde_json::from_str(RECORDED)
        .unwrap_or_else(|error| panic!("original paxd recording: {error:?}"));
    let digest =
        Sha256::digest(RECORDED.as_bytes())
            .iter()
            .fold(String::new(), |mut text, byte| {
                write!(text, "{byte:02x}").unwrap_or_else(|error| panic!("digest hex: {error:?}"));
                text
            });
    assert_eq!(vector["provenance"]["sourceSha256"], digest);
    assert_eq!(
        vector["cases"][0]["receiptJson"],
        recorded["settlement"]["receipt"]
    );
    assert_eq!(
        vector["cases"][0]["signedReceipt"],
        recorded["positive"]["receipt"]
    );
    vector
}

fn authorities(case: &Value) -> (Vec<ReceiptKeyEntry>, Vec<ConsensusAuthority>) {
    let key = &case["pinnedKey"];
    let keys = vec![ReceiptKeyEntry {
        key_id: fixed(&key["keyId"]),
        public_key: fixed(&key["publicKey"]),
        first_height: key["firstHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("pin first height")),
        last_height: key["lastHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("pin last height")),
    }];
    let validators = case["validators"]
        .as_array()
        .unwrap_or_else(|| panic!("recorded validator set"))
        .iter()
        .map(|v| {
            let public_key = fixed(&v["pub_key"]);
            CommitValidator {
                address: CommitValidator::address_for_key(&public_key),
                public_key,
                voting_power: v["voting_power"]
                    .as_u64()
                    .unwrap_or_else(|| panic!("validator power")),
            }
        })
        .collect();
    let h = header(&case["header"]);
    (
        keys,
        vec![ConsensusAuthority {
            chain_id: h.chain_id,
            first_height: h.height,
            last_height: h.height,
            validators,
        }],
    )
}

fn committed_fact(
    case: &Value,
    canonical: Vec<u8>,
    consensus: &[ConsensusAuthority],
) -> ReceiptFact {
    let entry = ActivityEntryId::new("act_repair223352")
        .unwrap_or_else(|error| panic!("activity id: {error:?}"));
    let mut fact = EvidenceBundle::receipt_fact(&entry, canonical, Rung::Committed)
        .unwrap_or_else(|error| panic!("human receipt codec: {error:?}"));
    let c = &case["commit"];
    let signatures = c["signatures"]
        .as_array()
        .unwrap_or_else(|| panic!("commit signatures"));
    assert_eq!(signatures.len(), consensus[0].validators.len());
    let votes = signatures
        .iter()
        .enumerate()
        .filter(|(_, vote)| {
            if vote["block_id_flag"] == 1 {
                assert_eq!(vote["validator_address"], "0x");
                assert!(vote["signature"].is_null());
                assert_eq!(vote["timestamp"]["seconds"], -62_135_596_800_i64);
                assert_eq!(vote["timestamp"]["nanos"], 0);
                false
            } else {
                true
            }
        })
        .map(|(index, vote)| {
            assert_eq!(vote["block_id_flag"], 2);
            let address = fixed(&vote["validator_address"]);
            assert_eq!(address, consensus[0].validators[index].address);
            CommitVote {
                validator_address: address,
                voting_power: consensus[0]
                    .validators
                    .iter()
                    .find(|v| v.address == address)
                    .unwrap_or_else(|| panic!("pinned vote"))
                    .voting_power,
                timestamp: time(&vote["timestamp"]),
                signature: fixed(&vote["signature"]),
            }
        })
        .collect();
    fact.committed = Some(CommittedReceiptEvidence {
        header: header(&case["header"]),
        execution_header: header(&case["executionHeader"]),
        commit: BlockCommit {
            height: c["height"]
                .as_u64()
                .unwrap_or_else(|| panic!("commit height")),
            round: c["round"]
                .as_u64()
                .unwrap_or_else(|| panic!("round"))
                .try_into()
                .unwrap_or_else(|error| panic!("round u32: {error:?}")),
            block_id: block_id(&c["block_id"]),
            votes,
        },
        batch_transaction: bytes(&case["batchProof"]["data"]),
        batch_total: case["batchProof"]["total"]
            .as_u64()
            .unwrap_or_else(|| panic!("batch total")),
        batch_aunts: case["batchProof"]["aunts"]
            .as_array()
            .unwrap_or_else(|| panic!("batch aunts"))
            .iter()
            .map(fixed)
            .collect(),
        activity_ids: vec![fact.tx_hash],
    });
    let proof = &case["receiptProof"];
    fact.header_binding = Some(ReceiptHeaderBinding {
        canonical_header_bytes: bytes(&case["receiptHeader"]),
        header_signature: fixed(&case["receiptHeaderSignature"]),
        receipt_proof: Proof::new(
            proof["LeafIndex"]
                .as_u64()
                .unwrap_or_else(|| panic!("index"))
                .try_into()
                .unwrap_or_else(|error| panic!("u32 index: {error:?}")),
            proof["LeafCount"]
                .as_u64()
                .unwrap_or_else(|| panic!("count"))
                .try_into()
                .unwrap_or_else(|error| panic!("u32 count: {error:?}")),
            proof["Siblings"]
                .as_array()
                .unwrap_or_else(|| panic!("siblings"))
                .iter()
                .map(|v| {
                    serde_json::from_value(v.clone())
                        .unwrap_or_else(|error| panic!("recorded sibling: {error:?}"))
                })
                .collect(),
        )
        .unwrap_or_else(|error| panic!("recorded proof codec: {error:?}")),
    });
    fact
}

// This peer serves the committed recording unchanged. It never signs a receipt,
// synthesizes a commitment, or substitutes the production RPC decoder.
fn observe(case: &Value) -> (SequencerReceipt, Vec<ReceiptKeyEntry>) {
    let listener = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("loopback transcript server: {error:?}"));
    listener
        .set_nonblocking(true)
        .unwrap_or_else(|error| panic!("bounded accept: {error:?}"));
    let chain_id = u64::from(
        decode_batch_header(&bytes(&case["receiptHeader"]))
            .unwrap_or_else(|error| panic!("recorded network: {error:?}"))
            .network_id(),
    );
    let config = NodeRpcConfig {
        url: format!(
            "http://{}/",
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("address: {error:?}"))
        ),
        chain_id,
        deadline: Duration::from_secs(3),
    };
    let case = case.clone();
    let hash = fixed(&case["receiptJson"]["txHash"]);
    let server = serve_recorded_rpc(listener, case, chain_id);
    let node = NodeRpc::connect(&config)
        .unwrap_or_else(|error| panic!("production NodeRpc connects: {error:?}"));
    let keys = node
        .receipt_keys()
        .unwrap_or_else(|error| panic!("production receipt-key decoder: {error:?}"));
    let receipt = node
        .sequencer_receipt(&hash)
        .unwrap_or_else(|error| panic!("production committed receipt decoder: {error:?}"))
        .unwrap_or_else(|| panic!("known receipt"));
    server
        .join()
        .unwrap_or_else(|error| panic!("recorded RPC peer finished: {error:?}"));
    (receipt, keys)
}

fn serve_recorded_rpc(listener: TcpListener, case: Value, chain_id: u64) -> thread::JoinHandle<()> {
    thread::spawn(move || {
        let replies = [
            ("eth_chainId", json!([]), json!(format!("{chain_id:#x}"))),
            (
                "px_getReceiptKeys",
                json!([]),
                json!({"keys":[case["pinnedKey"]]}),
            ),
            (
                "pax_sequencerReceipt",
                json!([case["receiptJson"]["txHash"]]),
                case["receiptJson"].clone(),
            ),
        ];
        let expires = std::time::Instant::now() + Duration::from_secs(10);
        for (method, params, result) in replies {
            let mut stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(std::time::Instant::now() < expires, "missing RPC {method}");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(e) => panic!("accept RPC: {e}"),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(3)))
                .unwrap_or_else(|error| panic!("read deadline: {error:?}"));
            stream
                .set_write_timeout(Some(Duration::from_secs(3)))
                .unwrap_or_else(|error| panic!("write deadline: {error:?}"));
            let mut reader = BufReader::new(&stream);
            let mut line = String::new();
            reader
                .read_line(&mut line)
                .unwrap_or_else(|error| panic!("request line: {error:?}"));
            assert_eq!(line, "POST / HTTP/1.1\r\n");
            let mut length = None;
            loop {
                line.clear();
                assert_ne!(
                    reader
                        .read_line(&mut line)
                        .unwrap_or_else(|error| panic!("HTTP header: {error:?}")),
                    0
                );
                if line == "\r\n" {
                    break;
                }
                if let Some((name, value)) = line.split_once(':') {
                    if name.eq_ignore_ascii_case("content-length") {
                        length = Some(
                            value
                                .trim()
                                .parse::<usize>()
                                .unwrap_or_else(|error| panic!("request length: {error:?}")),
                        );
                    }
                }
            }
            let length = length.unwrap_or_else(|| panic!("bounded request body"));
            assert!(length <= 4096);
            let mut body = vec![0; length];
            reader
                .read_exact(&mut body)
                .unwrap_or_else(|error| panic!("request body: {error:?}"));
            let request: Value = serde_json::from_slice(&body)
                .unwrap_or_else(|error| panic!("RPC request: {error:?}"));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], params);
            let response = json!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()).unwrap_or_else(|error| panic!("transcript reply: {error:?}"));
        }
    })
}

pub fn committed_receipt_crosses_rpc_and_recovers() {
    let vector = recording();
    let case = &vector["cases"][0];
    let (pins, consensus) = authorities(case);
    let (observed, keys) = observe(case);
    assert_eq!(keys, pins);
    let SequencerReceipt::Committed {
        receipt,
        block_number,
        block_hash,
    } = &observed
    else {
        panic!("recording must be committed");
    };
    assert_eq!(receipt.receipt, bytes(&case["signedReceipt"]));
    assert_eq!(
        receipt.tx_hash,
        layerx_crypto::evm_transaction::transaction_hash(&bytes(&case["rawTransaction"]))
    );
    let decoded = decode_receipt(&receipt.receipt)
        .unwrap_or_else(|error| panic!("production canonical receipt: {error:?}"));
    let protocol = decoded
        .protocol()
        .unwrap_or_else(|| panic!("protocol receipt"));
    assert_eq!(protocol.activity_id(), receipt.tx_hash);
    assert_eq!(protocol.global_sequence(), receipt.seq);
    let mut signed_height = [0; 32];
    signed_height[24..].copy_from_slice(&receipt.height.to_be_bytes());
    assert_eq!(protocol.batch_id(), signed_height);
    assert_eq!(receipt.height, *block_number);
    let execution = header(&case["executionHeader"]);
    assert_eq!(execution.height, *block_number);
    assert_eq!(execution.block_hash(), *block_hash);
    let fact = committed_fact(case, receipt.receipt.clone(), &consensus);
    let report = EvidenceBundle::verify_receipt_with_consensus(&fact, &pins, &consensus)
        .unwrap_or_else(|error| {
            panic!("real paxd batch and consensus verify in human consumer: {error:?}")
        });
    assert_eq!(report.verified_receipts(), 1);
    assert_eq!(report.verified_inclusions(), 1);
    let root = support::directory("repair223352-committed");
    let retention = support::retention_uniform(10000);
    let principal = support::principal("alice");
    let (mut store, digest) = support::install_and_open(
        &root,
        &support::tenancy(&[("alice", "tenant-a")]),
        retention,
    );
    let id =
        ActivityEntryId::new("act_repair223352").unwrap_or_else(|error| panic!("entry: {error:?}"));
    let bundle = EvidenceBundle::receipt(id, fact.clone(), &principal)
        .unwrap_or_else(|error| panic!("human evidence bundle: {error:?}"));
    let encoded = bundle
        .encode()
        .unwrap_or_else(|error| panic!("production evidence encoder: {error:?}"));
    let expected_digest = bundle
        .digest()
        .unwrap_or_else(|error| panic!("bundle digest: {error:?}"));
    let row_key = support::row_key("committed-evidence");
    store
        .principal(&principal)
        .unwrap_or_else(|error| panic!("principal scope: {error:?}"))
        .put(Table::Cache, row_key.clone(), 100, encoded.clone())
        .unwrap_or_else(|error| panic!("persist verified evidence: {error:?}"));
    drop(store);
    let (reobserved, recovered_keys) = observe(case);
    assert_eq!(reobserved, observed);
    assert_eq!(recovered_keys, pins);
    let mut reopened = PrincipalStore::open(&root, retention, digest)
        .unwrap_or_else(|error| panic!("restart real human store: {error:?}"));
    {
        let scope = reopened
            .principal(&principal)
            .unwrap_or_else(|error| panic!("recovered scope: {error:?}"));
        let row = scope
            .get(Table::Cache, &row_key)
            .unwrap_or_else(|| panic!("recovered evidence"));
        assert_eq!(row.bytes(), encoded);
        let recovered = EvidenceBundle::decode(row.bytes())
            .unwrap_or_else(|error| panic!("production recovery decoder: {error:?}"));
        assert_eq!(recovered.protocol_evidence()[0].receipts[0], fact);
        assert_eq!(recovered.entries()[0].rung(), Rung::Committed);
        assert_eq!(recovered.entries()[0].block_height(), receipt.height);
        let report = recovered
            .verify_with_consensus(expected_digest, &principal, &recovered_keys, &consensus)
            .unwrap_or_else(|error| panic!("recovered committed evidence verifies: {error:?}"));
        assert_eq!(report.verified_receipts(), 1);
        assert_eq!(report.verified_inclusions(), 1);
        assert!(
            recovered
                .verify(expected_digest, &principal, &recovered_keys)
                .is_err(),
            "receipt keys alone cannot prove consensus commitment"
        );
    }
    drop(reopened);
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("remove store: {error:?}"));
}

pub fn recorded_malformed_and_foreign_evidence_is_refused() {
    let vector = recording();
    let case = &vector["cases"][0];
    let (keys, consensus) = authorities(case);
    let (observed, rpc_keys) = observe(case);
    assert_eq!(rpc_keys, keys);
    let SequencerReceipt::Committed { receipt, .. } = observed else {
        panic!("committed receipt required");
    };
    let fact = committed_fact(case, receipt.receipt, &consensus);
    EvidenceBundle::verify_receipt_with_consensus(&fact, &keys, &consensus)
        .unwrap_or_else(|error| panic!("valid control: {error:?}"));
    assert_damaged_receipts_refused(&fact, &keys, &consensus);
    let mut foreign_keys = keys.clone();
    foreign_keys[0].public_key = ed25519_dalek::SigningKey::from_bytes(&[0x85; 32])
        .verifying_key()
        .to_bytes();
    assert!(
        EvidenceBundle::verify_receipt_with_consensus(&fact, &foreign_keys, &consensus).is_err()
    );
    let mut foreign_consensus = consensus.clone();
    let public_key = foreign_keys[0].public_key;
    foreign_consensus[0].validators[0].public_key = public_key;
    foreign_consensus[0].validators[0].address = CommitValidator::address_for_key(&public_key);
    assert!(
        EvidenceBundle::verify_receipt_with_consensus(&fact, &keys, &foreign_consensus).is_err()
    );
}

fn assert_damaged_receipts_refused(
    fact: &ReceiptFact,
    keys: &[ReceiptKeyEntry],
    consensus: &[ConsensusAuthority],
) {
    let mut no_quorum = fact.clone();
    no_quorum
        .committed
        .as_mut()
        .unwrap_or_else(|| panic!("commit"))
        .commit
        .votes
        .clear();
    assert!(
        EvidenceBundle::verify_receipt_with_consensus(&no_quorum, keys, consensus).is_err(),
        "absent validator slots cannot supply voting power"
    );
    for target in [
        "receipt",
        "header",
        "proof",
        "batch",
        "batch-aunt",
        "commit",
        "sequence",
    ] {
        let mut damaged = fact.clone();
        match target {
            "receipt" => {
                *damaged
                    .canonical_receipt_bytes
                    .last_mut()
                    .unwrap_or_else(|| panic!("signature byte")) ^= 1;
                damaged.receipt_hash = Sha256::digest(&damaged.canonical_receipt_bytes).into();
            }
            "header" => {
                damaged
                    .header_binding
                    .as_mut()
                    .unwrap_or_else(|| panic!("header"))
                    .canonical_header_bytes
                    .pop();
            }
            "proof" => {
                let binding = damaged
                    .header_binding
                    .as_mut()
                    .unwrap_or_else(|| panic!("binding"));
                let mut siblings = binding.receipt_proof.siblings().to_vec();
                siblings[0][0] ^= 1;
                binding.receipt_proof = Proof::new(0, 2, siblings)
                    .unwrap_or_else(|error| panic!("malformed sibling proof: {error:?}"));
            }
            "batch" => {
                damaged
                    .committed
                    .as_mut()
                    .unwrap_or_else(|| panic!("batch"))
                    .batch_transaction[40] ^= 1;
            }
            "batch-aunt" => {
                damaged
                    .committed
                    .as_mut()
                    .unwrap_or_else(|| panic!("batch proof"))
                    .batch_aunts[0][0] ^= 1;
            }
            "commit" => {
                damaged
                    .committed
                    .as_mut()
                    .unwrap_or_else(|| panic!("commit"))
                    .commit
                    .votes[0]
                    .signature[0] ^= 1;
            }
            "sequence" => {
                damaged
                    .committed
                    .as_mut()
                    .unwrap_or_else(|| panic!("sequence"))
                    .activity_ids[0][0] ^= 1;
            }
            _ => unreachable!(),
        }
        assert!(
            EvidenceBundle::verify_receipt_with_consensus(&damaged, keys, consensus).is_err(),
            "accepted malformed {target}"
        );
    }
}
