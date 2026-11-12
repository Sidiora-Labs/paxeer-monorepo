#[allow(dead_code)]
#[path = "support/production_fixture.rs"]
mod production_fixture;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_agent_api::subscription::{Cursor, EventDelivery, EventIdentity, ReceiptReference};
use layerx_agent_api::track::ReceiptRef;
use layerx_agentd::read::{
    BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator, CommitVote,
};
use layerx_client::rpc::receipt::{
    lookup_authenticated, AuthenticatedReceipt, Lookup, LookupContext, ReceiptError,
};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint, SequencerError};
use layerx_human_service::activity::{
    ActivityKind, AgentActivity, CommittedReceiptEvidence, ConsensusAuthority, EvidenceBundle,
    Feed, FilterDraft, PageRequest, PendingStatus, ReceiptFact, VerifiedStatus,
};
use layerx_human_service::notify::ActivityEntryId;
use layerx_human_service::store::{PrincipalStore, RowKey, Table};
use layerx_human_test_support as support;
use layerx_proof::receipt::{ReceiptCheck, VerificationFailure};
use layerx_types::rung::Rung;
use production_fixture::{recorded_node::NodeRecording, PeerState, RpcServer};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::sync::{Arc, Mutex};

fn start_recorded(state: Arc<Mutex<PeerState>>) -> production_fixture::Result<RpcServer> {
    let peer = state.lock().or_fail("peer state");
    assert!(
        peer.node_recording.is_some()
            || peer.receipt_rung == Some(layerx_types::rung::Rung::Committed)
    );
    drop(peer);
    RpcServer::serve(state, None)
}

fn transactions(recording: &NodeRecording) -> Vec<Vec<u8>> {
    ["sequence_batch_proof", "transaction_proof"]
        .iter()
        .map(|key| {
            STANDARD
                .decode(
                    recording.settlement[key]["data"]
                        .as_str()
                        .or_fail("node tx"),
                )
                .or_fail("node tx base64")
        })
        .collect()
}

fn hex(bytes: &[u8]) -> String {
    format!("0x{}", layerx_human_service::hex::lower(bytes))
}
fn bytes(v: &Value) -> Vec<u8> {
    let text = v.as_str().or_fail("hex string");
    let digits = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(digits.len() % 2, 0);
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|p| {
            u8::from_str_radix(std::str::from_utf8(p).or_fail("ASCII"), 16).or_fail("hex byte")
        })
        .collect()
}
fn fixed<const N: usize>(v: &Value) -> [u8; N] {
    bytes(v).try_into().or_fail("fixed-width node field")
}
fn number(v: &Value) -> u64 {
    v.as_u64().unwrap_or_else(|| {
        v.as_str()
            .or_fail("decimal")
            .parse()
            .or_fail("node integer")
    })
}
fn quantity(v: &Value) -> u64 {
    u64::from_str_radix(
        v.as_str()
            .or_fail("quantity")
            .strip_prefix("0x")
            .or_fail("quantity prefix"),
        16,
    )
    .or_fail("quantity integer")
}
fn time(v: &Value) -> CommitTimestamp {
    let text = v.as_str().or_fail("UTC time");
    let (whole, fraction) = text
        .strip_suffix('Z')
        .or_fail("UTC")
        .split_once('.')
        .unwrap_or((text.strip_suffix('Z').or_fail("UTC"), ""));
    assert!(fraction.len() <= 9 && fraction.bytes().all(|b| b.is_ascii_digit()));
    CommitTimestamp {
        seconds: production_fixture::time::seconds_from_rfc3339(&format!("{whole}Z"))
            .or_fail("production timestamp parser")
            .try_into()
            .or_fail("i64 time"),
        nanos: if fraction.is_empty() {
            0
        } else {
            format!("{fraction:0<9}").parse().or_fail("nanoseconds")
        },
    }
}
fn block_id(v: &Value) -> BlockId {
    BlockId {
        hash: fixed(&v["hash"]),
        part_set_total: number(&v["parts"]["total"]).try_into().or_fail("parts"),
        part_set_hash: fixed(&v["parts"]["hash"]),
    }
}
fn header(v: &Value) -> CheckpointHeader {
    let optional = |key: &str| (!bytes(&v[key]).is_empty()).then(|| fixed(&v[key]));
    CheckpointHeader {
        version_block: number(&v["version"]["block"]),
        version_app: number(&v["version"]["app"]),
        chain_id: v["chain_id"].as_str().or_fail("chain id").to_owned(),
        height: number(&v["height"]),
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
fn validators(v: &Value) -> Vec<CommitValidator> {
    v.as_array()
        .or_fail("validators")
        .iter()
        .map(|v| {
            let key = STANDARD
                .decode(v["pub_key"]["value"].as_str().or_fail("public key"))
                .or_fail("base64 key")
                .try_into()
                .or_fail("32 byte key");
            let validator = CommitValidator {
                address: fixed(&v["address"]),
                public_key: key,
                voting_power: number(&v["voting_power"]),
            };
            assert_eq!(validator.address, CommitValidator::address_for_key(&key));
            validator
        })
        .collect()
}
fn commit(v: &Value, trusted: &[CommitValidator]) -> BlockCommit {
    BlockCommit {
        height: number(&v["height"]),
        round: number(&v["round"]).try_into().or_fail("round"),
        block_id: block_id(&v["block_id"]),
        votes: v["signatures"]
            .as_array()
            .or_fail("votes")
            .iter()
            .map(|vote| {
                assert_eq!(vote["block_id_flag"], 2);
                let address = fixed(&vote["validator_address"]);
                CommitVote {
                    validator_address: address,
                    voting_power: trusted
                        .iter()
                        .find(|v| v.address == address)
                        .or_fail("pinned validator")
                        .voting_power,
                    timestamp: time(&vote["timestamp"]),
                    signature: STANDARD
                        .decode(vote["signature"].as_str().or_fail("signature"))
                        .or_fail("base64 signature")
                        .try_into()
                        .or_fail("64 byte signature"),
                }
            })
            .collect(),
    }
}
fn rpc(server: &RpcServer, method: &str, params: &Value) -> Value {
    let request =
        serde_json::to_vec(&json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}))
            .or_fail("RPC request");
    let mut response = ureq::post(&server.url)
        .header("Content-Type", "application/json")
        .send(request.as_slice())
        .or_fail("loopback HTTP");
    let answer: Value =
        serde_json::from_slice(&response.body_mut().read_to_vec().or_fail("RPC body"))
            .or_fail("JSON RPC response");
    assert!(answer.get("error").is_none(), "{answer}");
    answer["result"].clone()
}
fn start() -> (Arc<Mutex<PeerState>>, RpcServer) {
    let state = Arc::new(Mutex::new(PeerState {
        receipt_rung: Some(Rung::Committed),
        ..PeerState::default()
    }));
    let server = start_recorded(state.clone()).or_fail("recorded node transport");
    (state, server)
}
fn trusted() -> Vec<ConsensusAuthority> {
    let source = NodeRecording::open();
    let h = &source.settlement["header"];
    vec![ConsensusAuthority {
        chain_id: h["chain_id"].as_str().or_fail("chain").to_owned(),
        first_height: number(&h["height"]),
        last_height: number(&h["height"]),
        validators: validators(&source.settlement["validators"]),
    }]
}
fn authenticated_block(
    server: &RpcServer,
    original: &NodeRecording,
    receipt: &AuthenticatedReceipt,
    view: &Value,
    height: u64,
) -> (CheckpointHeader, Vec<Vec<u8>>, Value) {
    let block = rpc(server, "block", &json!([height.to_string()]));
    assert_eq!(
        block["block"]["header"],
        original.settlement["execution_header"]
    );
    let execution = header(&block["block"]["header"]);
    assert_eq!(execution.height, height);
    assert_eq!(execution.block_hash(), fixed::<32>(&view["blockHash"]));
    assert_eq!(
        receipt.covering_block(),
        Some((height, execution.block_hash()))
    );
    let txs: Vec<Vec<u8>> = block["block"]["data"]["txs"]
        .as_array()
        .or_fail("node transactions")
        .iter()
        .map(|v| {
            STANDARD
                .decode(v.as_str().or_fail("base64 tx"))
                .or_fail("tx bytes")
        })
        .collect();
    assert_eq!(
        txs.len(),
        2,
        "the real block contains a batch and an EVM transaction"
    );
    assert_eq!(txs, transactions(original));
    // Comet hashes each raw transaction before its simple-Merkle leaf.
    let leaves: Vec<[u8; 32]> = txs
        .iter()
        .map(|tx| {
            let mut hash = Sha256::new();
            hash.update([0]);
            hash.update(Sha256::digest(tx));
            hash.finalize().into()
        })
        .collect();
    let mut root = Sha256::new();
    root.update([1]);
    root.update(leaves[0]);
    root.update(leaves[1]);
    assert_eq!(execution.data_hash, Some(root.finalize().into()));
    let batch = rpc(server, "tx", &json!([hex(&Sha256::digest(&txs[0])), true]));
    let payment = rpc(server, "tx", &json!([hex(&Sha256::digest(&txs[1])), true]));
    assert_eq!(number(&batch["index"]), 0);
    assert_eq!(number(&payment["index"]), 1);
    assert_eq!(batch["proof"], original.settlement["sequence_batch_proof"]);
    assert_eq!(payment["proof"], original.settlement["transaction_proof"]);
    assert_eq!(
        STANDARD
            .decode(batch["tx"].as_str().or_fail("batch"))
            .or_fail("base64"),
        txs[0]
    );
    assert_eq!(
        STANDARD
            .decode(payment["tx"].as_str().or_fail("payment"))
            .or_fail("base64"),
        txs[1]
    );
    assert_eq!(
        fixed::<32>(&batch["proof"]["root_hash"]),
        execution.data_hash.or_fail("authenticated root")
    );
    assert_eq!(
        fixed::<32>(&payment["proof"]["root_hash"]),
        execution.data_hash.or_fail("authenticated root")
    );
    (execution, txs, batch)
}

fn receipt_keys(endpoint: &SequencerEndpoint, original: &NodeRecording) -> Vec<ReceiptKeyEntry> {
    let keys = endpoint.receipt_keys().or_fail("node receipt keys");
    assert_eq!(keys.len(), 1);
    let pinned_public: [u8; 32] =
        serde_json::from_value(original.authorization["PublicKey"].clone())
            .or_fail("independently recorded public key");
    assert_eq!(keys[0].public_key, pinned_public);
    assert_eq!(
        keys[0].key_id,
        <[u8; 32]>::from(Sha256::digest(pinned_public))
    );
    assert_eq!(keys[0].key_id, fixed::<32>(&original.receipt()["keyId"]));
    assert_eq!(
        keys[0].key_id.as_slice(),
        STANDARD
            .decode(
                original.settlement["sequence_batch"]["KeyId"]
                    .as_str()
                    .or_fail("batch key id")
            )
            .or_fail("base64 key id")
    );
    keys
}

fn acquire(server: &RpcServer) -> (ActivityEntryId, ReceiptFact, Vec<ReceiptKeyEntry>) {
    let original = NodeRecording::open();
    let endpoint = SequencerEndpoint::new(&server.url, &[]).or_fail("production RPC client");
    let hash = fixed(original.receipt().get("txHash").or_fail("tx hash"));
    let raw = bytes(&rpc(
        server,
        "eth_getRawTransactionByHash",
        &json!([hex(&hash)]),
    ));
    assert_eq!(layerx_crypto::evm_transaction::transaction_hash(&raw), hash);
    let keys = receipt_keys(&endpoint, &original);
    let Lookup::Verified(receipt) = lookup_authenticated(
        &endpoint,
        hash,
        LookupContext {
            receipt_keys: &keys,
        },
    )
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "real signature and execution view verification"
        )
    }) else {
        panic!("missing node receipt")
    };
    // An RPC commitment label alone is not consensus verification.
    assert_eq!(receipt.rung(), Rung::Receipted);
    let view = rpc(server, "pax_sequencerReceipt", &json!([hex(&hash)]));
    assert_eq!(
        view,
        *original.receipt(),
        "node-emitted view round trips unchanged"
    );
    assert_eq!(receipt.canonical_bytes(), bytes(&view["receipt"]));
    assert_eq!(quantity(&view["height"]), quantity(&view["blockNumber"]));
    let height = quantity(&view["height"]);
    assert_eq!(receipt.height(), height);
    let signed = receipt
        .receipt()
        .protocol()
        .or_fail("protocol receipt")
        .batch_id();
    assert_eq!(
        u64::from_be_bytes(signed[24..].try_into().or_fail("signed height")),
        height
    );
    let (execution, txs, batch) = authenticated_block(server, &original, &receipt, &view, height);
    let successor = rpc(server, "commit", &json!([(height + 1).to_string()]));
    let successor_header = header(&successor["signed_header"]["header"]);
    let authorities = trusted();
    assert_eq!(
        validators(&rpc(server, "validators", &json!([(height + 1).to_string()]))["validators"]),
        authorities[0].validators
    );
    let id = ActivityEntryId::new("act_repair223351").or_fail("entry id");
    let mut fact =
        EvidenceBundle::receipt_fact(&id, receipt.canonical_bytes().to_vec(), Rung::Committed)
            .or_fail("production receipt decoder");
    fact.committed = Some(CommittedReceiptEvidence {
        header: successor_header,
        execution_header: execution,
        commit: commit(
            &successor["signed_header"]["commit"],
            &authorities[0].validators,
        ),
        batch_transaction: txs[0].clone(),
        batch_total: number(&batch["proof"]["proof"]["total"]),
        batch_aunts: batch["proof"]["proof"]["aunts"]
            .as_array()
            .or_fail("aunts")
            .iter()
            .map(|a| {
                STANDARD
                    .decode(a.as_str().or_fail("base64 aunt"))
                    .or_fail("aunt")
                    .try_into()
                    .or_fail("32-byte aunt")
            })
            .collect(),
        activity_ids: vec![hash],
    });
    assert_eq!(fact.block_height, height);
    EvidenceBundle::verify_receipt_with_consensus(&fact, &keys, &authorities).unwrap_or_else(
        |error| {
            panic!(
                "{}: {error:?}",
                "production consumer authenticates recorded consensus and batch roots"
            )
        },
    );
    (id, fact, keys)
}

#[test]
fn finding13_node_heights_multiple_transactions_and_authenticated_roots() {
    let (state, server) = start();
    let (_, fact, keys) = acquire(&server);
    for target in ["height", "data_root", "result_root", "batch", "commit"] {
        let mut changed = fact.clone();
        let evidence = changed.committed.as_mut().or_fail("committed evidence");
        match target {
            "height" => changed.block_height += 1,
            "data_root" => {
                evidence
                    .execution_header
                    .data_hash
                    .as_mut()
                    .or_fail("data root")[0] ^= 1;
            }
            "result_root" => {
                evidence
                    .header
                    .last_results_hash
                    .as_mut()
                    .or_fail("results root")[0] ^= 1;
            }
            "batch" => evidence.batch_transaction[40] ^= 1,
            "commit" => evidence.commit.votes[0].signature[0] ^= 1,
            _ => unreachable!(),
        }
        assert!(
            EvidenceBundle::verify_receipt_with_consensus(&changed, &keys, &trusted()).is_err(),
            "accepted changed {target}"
        );
    }
    let peer = state.lock().or_fail("peer state");
    for method in [
        "pax_sequencerReceipt",
        "eth_getTransactionReceipt",
        "block",
        "tx",
        "commit",
    ] {
        assert!(
            peer.rpc_served.iter().any(|served| served == method),
            "missing real transport call {method}"
        );
    }
}

fn delivery_and_meaning(
    id: &ActivityEntryId,
    fact: &ReceiptFact,
) -> (EventDelivery, AgentActivity) {
    let delivery = EventDelivery::new(
        EventIdentity::new(fact.receipt_hash),
        fact.canonical_receipt_bytes.clone(),
        Cursor {
            block_height: fact.block_height,
            tx_index: 1,
            log_index: 0,
        },
        ReceiptReference::Verified {
            receipt_ref: ReceiptRef::new(hex(&fact.receipt_hash)).or_fail("receipt ref"),
            rung: fact.rung,
        },
    )
    .or_fail("agent delivery");
    let meaning = AgentActivity::new(
        (*id).clone(),
        ActivityKind::AgentAction,
        None,
        10,
        PendingStatus::Processing,
        VerifiedStatus::Done,
    )
    .or_fail("activity meaning");
    (delivery, meaning)
}

#[test]
fn finding13_node_evidence_survives_store_restart_and_transport_replay() {
    let (_, server) = start();
    let (id, fact, keys) = acquire(&server);
    let root = support::directory("repair223351-restart");
    let retention = support::retention_uniform(10000);
    let (mut store, tenancy) = support::install_and_open(
        &root,
        &support::tenancy(&[("alice", "tenant-a")]),
        retention,
    );
    let principal = support::principal("alice");
    let (delivery, meaning) = delivery_and_meaning(&id, &fact);
    let encoded = EvidenceBundle::receipt(id, fact.clone(), &principal)
        .or_fail("evidence")
        .encode()
        .or_fail("production export codec");
    {
        let mut scope = store.principal(&principal).or_fail("scope");
        Feed::record_agent_event(&mut scope, &meaning, &delivery, 100)
            .or_fail("consumer projection");
        scope
            .put(
                Table::Cache,
                RowKey::new("recorded-evidence").or_fail("key"),
                100,
                encoded.clone(),
            )
            .or_fail("durable evidence");
    }
    drop(store);
    drop(server);
    let (_, restarted) = start();
    let (_, replay, replay_keys) = acquire(&restarted);
    assert_eq!(replay, fact);
    assert_eq!(replay_keys, keys);
    let mut store =
        PrincipalStore::open(&root, retention, tenancy).or_fail("reopen production store");
    {
        let mut scope = store.principal(&principal).or_fail("recovered scope");
        let recovered = scope
            .get(
                Table::Cache,
                &RowKey::new("recorded-evidence").or_fail("key"),
            )
            .or_fail("recovered row");
        assert_eq!(recovered.bytes(), encoded);
        let decoded = EvidenceBundle::decode(recovered.bytes()).or_fail("production decode");
        assert_eq!(decoded.protocol_evidence()[0].receipts[0], replay);
        assert_eq!(
            decoded
                .verify_with_consensus(
                    decoded.digest().or_fail("digest"),
                    &principal,
                    &keys,
                    &trusted()
                )
                .or_fail("recovered consensus verification")
                .verified_receipts(),
            1
        );
        Feed::record_agent_event(&mut scope, &meaning, &delivery, 101)
            .or_fail("replay after restart");
        assert_eq!(
            Feed::projected_agent_cursor(&scope).or_fail("durable cursor"),
            Some(delivery.cursor)
        );
        let page = Feed::new(5)
            .or_fail("feed")
            .page(
                &scope,
                PageRequest::new(
                    10,
                    Feed::apply_filters(FilterDraft::new()).or_fail("filters"),
                ),
                101,
                fact.block_height,
            )
            .or_fail("consumer page");
        assert_eq!(
            page.entries().len(),
            1,
            "replay must not duplicate committed activity"
        );
    }
    drop(store);
    std::fs::remove_dir_all(root).or_fail("cleanup");
}

#[test]
fn finding13_unrecorded_transactions_cannot_receive_fabricated_commitment() {
    let (_, server) = start();
    let endpoint = SequencerEndpoint::new(&server.url, &[]).or_fail("endpoint");
    assert!(
        matches!(endpoint.sequencer_receipt(&[0x55; 32]), Err(SequencerError::Rpc(refusal)) if refusal.code == -32001)
    );
    assert!(
        matches!(endpoint.block_by_number(88), Err(SequencerError::Rpc(refusal)) if refusal.code == -32001)
    );
}

#[test]
fn finding13_rpc_height_mutation_is_rejected_by_the_production_reader() {
    for (height, refusal) in [
        ("0x58", ReceiptError::Sequencer(SequencerError::Malformed)),
        ("0x9", ReceiptError::FieldMismatch("height")),
    ] {
        let mut recording = NodeRecording::open();
        let hash = fixed(&recording.receipt()["txHash"]);
        recording.settlement["receipt"]["height"] = json!(height);
        let state = Arc::new(Mutex::new(PeerState {
            node_recording: Some(recording),
            ..PeerState::default()
        }));
        let server = start_recorded(state).or_fail("damaged recording transport");
        let endpoint = SequencerEndpoint::new(&server.url, &[]).or_fail("endpoint");
        let keys = endpoint.receipt_keys().or_fail("keys");
        assert_eq!(
            lookup_authenticated(
                &endpoint,
                hash,
                LookupContext {
                    receipt_keys: &keys
                }
            ),
            Err(refusal),
            "RPC height {height} must match the signed height"
        );
    }
}

#[test]
fn finding13_receipt_key_identity_and_authority_are_required() {
    let (_, server) = start();
    let endpoint = SequencerEndpoint::new(&server.url, &[]).or_fail("endpoint");
    let original = NodeRecording::open();
    let hash = fixed(&original.receipt()["txHash"]);
    let keys = endpoint.receipt_keys().or_fail("keys");
    for target in ["raw_public_key_id", "wrong_public_key", "uncovered_height"] {
        let mut changed = keys.clone();
        match target {
            "raw_public_key_id" => changed[0].key_id = changed[0].public_key,
            "wrong_public_key" => {
                changed[0].public_key = ed25519_dalek::SigningKey::from_bytes(&[0x51; 32])
                    .verifying_key()
                    .to_bytes();
            }
            "uncovered_height" => changed[0].first_height += 1,
            _ => unreachable!(),
        }
        assert_eq!(
            lookup_authenticated(
                &endpoint,
                hash,
                LookupContext {
                    receipt_keys: &changed,
                },
            ),
            Err(ReceiptError::Verification(VerificationFailure {
                check: ReceiptCheck::SequencerSignature,
            })),
            "accepted {target}",
        );
    }
}

#[test]
fn finding13_corrupted_node_signature_is_rejected_over_transport() {
    let mut recording = NodeRecording::open();
    let hash = fixed(&recording.receipt()["txHash"]);
    let mut signed = bytes(&recording.receipt()["receipt"]);
    *signed.last_mut().or_fail("recorded signature") ^= 1;
    recording.settlement["receipt"]["receipt"] = json!(hex(&signed));
    let state = Arc::new(Mutex::new(PeerState {
        node_recording: Some(recording),
        ..PeerState::default()
    }));
    let server = start_recorded(state).or_fail("damaged signature transport");
    let endpoint = SequencerEndpoint::new(&server.url, &[]).or_fail("endpoint");
    let keys = endpoint.receipt_keys().or_fail("keys");
    assert_eq!(
        lookup_authenticated(
            &endpoint,
            hash,
            LookupContext {
                receipt_keys: &keys,
            },
        ),
        Err(ReceiptError::Verification(VerificationFailure {
            check: ReceiptCheck::SequencerSignature,
        })),
    );
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
