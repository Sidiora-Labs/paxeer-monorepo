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

const CONFORMANCE: &str =
    include_str!("../../../../../interop/specs/conformance/committed/vector-v4-committed.json");
const RECORDED: &str =
    include_str!("../../../../../../daemon/committed-fixture/vectors/vector-v4-committed.json");

fn bytes(value: &Value) -> Vec<u8> {
    let text = value
        .as_str()
        .expect("hex string")
        .strip_prefix("0x")
        .expect("0x prefix");
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&text[i..i + 2], 16).expect("hex byte"))
        .collect()
}
fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value).try_into().expect("fixed-width field")
}
fn time(value: &Value) -> CommitTimestamp {
    CommitTimestamp {
        seconds: value["seconds"].as_i64().expect("seconds"),
        nanos: value["nanos"]
            .as_u64()
            .expect("nanos")
            .try_into()
            .expect("u32 nanos"),
    }
}
fn block_id(value: &Value) -> BlockId {
    BlockId {
        hash: fixed(&value["hash"]),
        part_set_total: value["parts"]["total"]
            .as_u64()
            .expect("parts")
            .try_into()
            .expect("u32 parts"),
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
        version_block: v["version"]["block"].as_u64().expect("version"),
        version_app: v["version"]["app"].as_u64().expect("app version"),
        chain_id: v["chain_id"].as_str().expect("chain id").to_owned(),
        height: v["height"].as_u64().expect("height"),
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
    let vector: Value = serde_json::from_str(CONFORMANCE).expect("committed conformance recording");
    let recorded: Value = serde_json::from_str(RECORDED).expect("original paxd recording");
    let digest: String = Sha256::digest(RECORDED.as_bytes())
        .iter()
        .map(|b| format!("{b:02x}"))
        .collect();
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
        first_height: key["firstHeight"].as_u64().expect("pin first height"),
        last_height: key["lastHeight"].as_u64().expect("pin last height"),
    }];
    let validators = case["validators"]
        .as_array()
        .expect("recorded validator set")
        .iter()
        .map(|v| {
            let public_key = fixed(&v["pub_key"]);
            CommitValidator {
                address: CommitValidator::address_for_key(&public_key),
                public_key,
                voting_power: v["voting_power"].as_u64().expect("validator power"),
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
    let entry = ActivityEntryId::new("act_repair223352").expect("activity id");
    let mut fact = EvidenceBundle::receipt_fact(&entry, canonical, Rung::Committed)
        .expect("human receipt codec");
    let c = &case["commit"];
    let votes = c["signatures"]
        .as_array()
        .expect("commit signatures")
        .iter()
        .map(|vote| {
            assert_eq!(vote["block_id_flag"], 2);
            let address = fixed(&vote["validator_address"]);
            CommitVote {
                validator_address: address,
                voting_power: consensus[0]
                    .validators
                    .iter()
                    .find(|v| v.address == address)
                    .expect("pinned vote")
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
            height: c["height"].as_u64().expect("commit height"),
            round: c["round"]
                .as_u64()
                .expect("round")
                .try_into()
                .expect("round u32"),
            block_id: block_id(&c["block_id"]),
            votes,
        },
        batch_transaction: bytes(&case["batchProof"]["data"]),
        batch_total: case["batchProof"]["total"].as_u64().expect("batch total"),
        batch_aunts: case["batchProof"]["aunts"]
            .as_array()
            .expect("batch aunts")
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
                .expect("index")
                .try_into()
                .expect("u32 index"),
            proof["LeafCount"]
                .as_u64()
                .expect("count")
                .try_into()
                .expect("u32 count"),
            proof["Siblings"]
                .as_array()
                .expect("siblings")
                .iter()
                .map(|v| serde_json::from_value(v.clone()).expect("recorded sibling"))
                .collect(),
        )
        .expect("recorded proof codec"),
    });
    fact
}

// This peer serves the committed recording unchanged. It never signs a receipt,
// synthesizes a commitment, or substitutes the production RPC decoder.
fn observe(case: &Value) -> (SequencerReceipt, Vec<ReceiptKeyEntry>) {
    let listener = TcpListener::bind("127.0.0.1:0").expect("loopback transcript server");
    listener.set_nonblocking(true).expect("bounded accept");
    let chain_id = u64::from(
        decode_batch_header(&bytes(&case["receiptHeader"]))
            .expect("recorded network")
            .network_id(),
    );
    let config = NodeRpcConfig {
        url: format!("http://{}/", listener.local_addr().expect("address")),
        chain_id,
        deadline: Duration::from_secs(3),
    };
    let case = case.clone();
    let hash = fixed(&case["receiptJson"]["txHash"]);
    let server = thread::spawn(move || {
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
                .expect("read deadline");
            stream
                .set_write_timeout(Some(Duration::from_secs(3)))
                .expect("write deadline");
            let mut reader = BufReader::new(&stream);
            let mut line = String::new();
            reader.read_line(&mut line).expect("request line");
            assert_eq!(line, "POST / HTTP/1.1\r\n");
            let mut length = None;
            loop {
                line.clear();
                assert_ne!(reader.read_line(&mut line).expect("HTTP header"), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some((name, value)) = line.split_once(':') {
                    if name.eq_ignore_ascii_case("content-length") {
                        length = Some(value.trim().parse::<usize>().expect("request length"));
                    }
                }
            }
            let length = length.expect("bounded request body");
            assert!(length <= 4096);
            let mut body = vec![0; length];
            reader.read_exact(&mut body).expect("request body");
            let request: Value = serde_json::from_slice(&body).expect("RPC request");
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], params);
            let response = json!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()).expect("transcript reply");
        }
    });
    let node = NodeRpc::connect(&config).expect("production NodeRpc connects");
    let keys = node.receipt_keys().expect("production receipt-key decoder");
    let receipt = node
        .sequencer_receipt(&hash)
        .expect("production committed receipt decoder")
        .expect("known receipt");
    server.join().expect("recorded RPC peer finished");
    (receipt, keys)
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
    let decoded = decode_receipt(&receipt.receipt).expect("production canonical receipt");
    let protocol = decoded.protocol().expect("protocol receipt");
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
        .expect("real paxd batch and consensus verify in human consumer");
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
    let id = ActivityEntryId::new("act_repair223352").expect("entry");
    let bundle =
        EvidenceBundle::receipt(id, fact.clone(), &principal).expect("human evidence bundle");
    let encoded = bundle.encode().expect("production evidence encoder");
    let expected_digest = bundle.digest().expect("bundle digest");
    let row_key = support::row_key("committed-evidence");
    store
        .principal(&principal)
        .expect("principal scope")
        .put(Table::Cache, row_key.clone(), 100, encoded.clone())
        .expect("persist verified evidence");
    drop(store);
    let (reobserved, recovered_keys) = observe(case);
    assert_eq!(reobserved, observed);
    assert_eq!(recovered_keys, pins);
    let mut reopened =
        PrincipalStore::open(&root, retention, digest).expect("restart real human store");
    {
        let scope = reopened.principal(&principal).expect("recovered scope");
        let row = scope
            .get(Table::Cache, &row_key)
            .expect("recovered evidence");
        assert_eq!(row.bytes(), encoded);
        let recovered = EvidenceBundle::decode(row.bytes()).expect("production recovery decoder");
        assert_eq!(recovered.protocol_evidence()[0].receipts[0], fact);
        assert_eq!(recovered.entries()[0].rung(), Rung::Committed);
        assert_eq!(recovered.entries()[0].block_height(), receipt.height);
        let report = recovered
            .verify_with_consensus(expected_digest, &principal, &recovered_keys, &consensus)
            .expect("recovered committed evidence verifies");
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
    std::fs::remove_dir_all(root).expect("remove store");
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
    EvidenceBundle::verify_receipt_with_consensus(&fact, &keys, &consensus).expect("valid control");
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
                    .expect("signature byte") ^= 1;
                damaged.receipt_hash = Sha256::digest(&damaged.canonical_receipt_bytes).into();
            }
            "header" => {
                damaged
                    .header_binding
                    .as_mut()
                    .expect("header")
                    .canonical_header_bytes
                    .pop();
            }
            "proof" => {
                let binding = damaged.header_binding.as_mut().expect("binding");
                let mut siblings = binding.receipt_proof.siblings().to_vec();
                siblings[0][0] ^= 1;
                binding.receipt_proof =
                    Proof::new(0, 2, siblings).expect("malformed sibling proof");
            }
            "batch" => damaged.committed.as_mut().expect("batch").batch_transaction[40] ^= 1,
            "batch-aunt" => damaged.committed.as_mut().expect("batch proof").batch_aunts[0][0] ^= 1,
            "commit" => {
                damaged.committed.as_mut().expect("commit").commit.votes[0].signature[0] ^= 1
            }
            "sequence" => damaged.committed.as_mut().expect("sequence").activity_ids[0][0] ^= 1,
            _ => unreachable!(),
        }
        assert!(
            EvidenceBundle::verify_receipt_with_consensus(&damaged, &keys, &consensus).is_err(),
            "accepted malformed {target}"
        );
    }
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
