//! Decision 2601: current committed records and recovery of legacy verification ranks.
fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

#[path = "native_send_preview_evm.rs"]
mod native_preview;

use layerx_agentd::{
    human::{HumanNativeSendOwnerPreviewRequestV1, HumanOperationError, HumanOperations},
    human_receipt_frame::ReceiptLookupAnswer,
    human_runtime::receipt_lookup_found_bytes,
    receipt::{self, ReceiptLookupKey},
    store::{ObjectKind, Store, TenantId, TenantKey},
};
use layerx_client::{
    rpc::{
        commit::decode_validators,
        receipt::{self as client_receipt, AuthenticatedReceipt, Lookup, LookupContext},
        receipt_commit::ReceiptCommitEvidence,
    },
    sequencer::{decode_receipt_keys, ReceiptKeyEntry, SequencerEndpoint},
};
use layerx_programs::chain_proof::{merkle::SimpleProof, ChainProofVerifier};
use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::{rung::Rung, verify::VerificationLevel};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    io::{BufRead, BufReader, Read, Write},
    net::TcpListener,
    path::Path,
    process::Command,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc,
    },
    thread,
    time::Duration,
};

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("committed recovery: {error:?}"))
}
fn unhex(value: &Value) -> Vec<u8> {
    must(hex::decode(
        require_some(value.as_str(), "hex").trim_start_matches("0x"),
    ))
}
fn number(value: &Value) -> u64 {
    value
        .as_u64()
        .unwrap_or_else(|| must(require_some(value.as_str(), "number").parse()))
}

// The Go producer runs the real MemIAVL, CometBFT and node receipt codecs. This
// transport forwards its exact replies to the production receipt authenticator.
fn committed_receipt() -> (AuthenticatedReceipt, Vec<ReceiptKeyEntry>) {
    let root = must(
        Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("../../../..")
            .canonicalize(),
    );
    let output = must(
        Command::new("go")
            .current_dir(root)
            .args([
                "run",
                "./services/agent/crates/layerx-client/src/rpc/repair_22_31_222_producer",
            ])
            .output(),
    );
    assert!(
        output.status.success(),
        "producer: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    let produced: Value = must(serde_json::from_slice(&output.stdout));
    let record = &produced["receipt"];
    let pins = must(decode_receipt_keys(&json!({"keys":[record["key"]]})));
    let trust = ChainProofVerifier {
        chain_id: require_some(
            record["material"]["signedHeader"]["header"]["chain_id"].as_str(),
            "chain",
        )
        .to_owned(),
        validators: must(decode_validators(&record["validators"])),
    };
    let batch = unhex(&record["batch"]);
    let b64 = |v: &Value| {
        must(openssl::base64::decode_block(require_some(
            v.as_str(),
            "base64",
        )))
    };
    let proof = SimpleProof {
        total: require_ok(
            i64::try_from(number(&record["proof"]["total"])),
            "test value fits i64",
        ),
        index: require_ok(
            i64::try_from(number(&record["proof"]["index"])),
            "test value fits i64",
        ),
        leaf_hash: must(b64(&record["proof"]["leaf_hash"]).try_into()),
        aunts: record["proof"]["aunts"]
            .as_array()
            .map(|aunts| {
                aunts
                    .iter()
                    .map(|aunt| must(b64(aunt).try_into()))
                    .collect()
            })
            .unwrap_or_default(),
    };
    let ids: Vec<[u8; 32]> = require_some(record["activityIds"].as_array(), "activity ids")
        .iter()
        .map(|v| must(unhex(v).try_into()))
        .collect();
    let evidence = ReceiptCommitEvidence {
        verifier: &trust,
        covering_height: number(&record["height"]),
        batch_transaction: &batch,
        transaction_proof: &proof,
        activity_ids: &ids,
    };
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    must(listener.set_nonblocking(true));
    finish_committed_receipt_1_repair_22_31_539(record, pins, &evidence, listener)
}

fn finish_committed_receipt_1_repair_22_31_539(
    record: &Value,
    pins: Vec<ReceiptKeyEntry>,
    evidence: &ReceiptCommitEvidence<'_>,
    listener: TcpListener,
) -> (AuthenticatedReceipt, Vec<ReceiptKeyEntry>) {
    let endpoint = must(SequencerEndpoint::new(
        &format!("http://{}/", must(listener.local_addr())),
        &[],
    ));
    let stop = Arc::new(AtomicBool::new(false));
    let stopped = Arc::clone(&stop);
    let replies = record.clone();
    let worker = thread::spawn(move || {
        while !stopped.load(Ordering::Acquire) {
            let (stream, _) = match listener.accept() {
                Ok(value) => value,
                Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                    thread::sleep(Duration::from_millis(1));
                    continue;
                }
                Err(e) => panic!("{e}"),
            };
            must(stream.set_read_timeout(Some(Duration::from_secs(10))));
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(v) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(must(v.trim().parse::<usize>()));
                }
            }
            let mut bytes = vec![0; require_some(length, "content length")];
            must(reader.read_exact(&mut bytes));
            let request: Value = must(serde_json::from_slice(&bytes));
            let result = match require_some(request["method"].as_str(), "method") {
                "pax_sequencerReceipt" => {
                    assert_eq!(request["params"], json!([replies["view"]["txHash"]]));
                    replies["view"].clone()
                }
                "eth_getTransactionReceipt" => replies["landing"].clone(),
                "px_getCommit" => replies["material"].clone(),
                "px_getValidators" => replies["validators"].clone(),
                "eth_getBlockByNumber" => replies["block"].clone(),
                method => panic!("unexpected receipt request {method}"),
            };
            let body = json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()));
            must(stream.flush());
        }
    });
    let result = client_receipt::lookup_authenticated_with_commit(
        &endpoint,
        must(unhex(&record["view"]["txHash"]).try_into()),
        LookupContext {
            receipt_keys: &pins,
        },
        evidence,
    );
    stop.store(true, Ordering::Release);
    must(worker.join());
    let Lookup::Verified(receipt) = must(result) else {
        panic!("authenticated receipt")
    };
    assert_eq!(receipt.rung(), Rung::Committed);
    (receipt, pins)
}

fn metadata_key(tenant: &TenantId, canonical: &[u8]) -> TenantKey {
    must(TenantKey::new(
        tenant.clone(),
        ObjectKind::Configuration,
        [
            b"receipt-metadata:".as_slice(),
            Sha256::digest(canonical).as_slice(),
        ]
        .concat(),
    ))
}

#[test]
fn committed_receipt_store_restart_and_owner_frames_accept_current_and_legacy_ranks() {
    let (authenticated, keys) = committed_receipt();
    let root = std::env::temp_dir().join(format!("committed-539-receipt-{}", std::process::id()));
    let tenant = must(TenantId::new("committed-539"));
    let hash = authenticated.activity_id();
    let mut store = must(Store::open(&root));
    let served = must(receipt::store_verified_if_absent(
        &mut store,
        tenant.clone(),
        hash,
        authenticated.canonical_bytes(),
        &authenticated,
    ));
    assert_eq!(
        served.metadata.verification_level,
        VerificationLevel::COMMITTED
    );
    let key = metadata_key(&tenant, &served.canonical_bytes);
    assert_eq!(
        require_some(store.get(&key), "stored metadata").bytes()[76],
        6
    );
    drop(store);
    let mut store = must(Store::open(&root));
    assert_eq!(
        must(receipt::serve(
            &store,
            tenant.clone(),
            ReceiptLookupKey::Idempotency(hash)
        )),
        served
    );
    let protocol = require_some(authenticated.receipt().protocol(), "protocol receipt");
    let authority = AuthorizedBatch::new(
        protocol.batch_id(),
        protocol.asset(),
        protocol.previous_state_root(),
        protocol.resulting_state_root(),
        authenticated.execution().signing_key,
    );
    let finality = layerx_agentd::finality::FinalityRecord {
        idempotency_key: hash,
        tx_hash: hash,
        height: authenticated.height(),
        rung: Rung::Committed,
        receipt_bytes: served.canonical_bytes.clone(),
        receipt_key: keys[0],
        block: None,
    };
    let frame = must(receipt_lookup_found_bytes(
        &served, &finality, authority, &keys,
    ));
    let mut decoder = layerx_wire::decode::Decoder::new(&frame, 1_048_576);
    assert_eq!(must(decoder.u8()), 1);
    assert_eq!(must(decoder.bytes_owned(1_048_576)), served.canonical_bytes);
    must(decoder.fixed(5 * 32));
    let offset = decoder.offset();
    assert_eq!(frame[offset], 6);
    for rank in [6, 4, 5] {
        let mut bytes = frame.clone();
        bytes[offset] = rank;
        let ReceiptLookupAnswer::Found(answer) = must(ReceiptLookupAnswer::decode(&bytes)) else {
            panic!("owner receipt")
        };
        assert_eq!(answer.verification_level, VerificationLevel::COMMITTED);
        assert_eq!(answer.canonical_bytes, served.canonical_bytes);
        let mut metadata = require_some(store.get(&key), "metadata").bytes().to_vec();
        metadata[76] = rank;
        must(store.put_local(key.clone(), metadata));
        drop(store);
        store = must(Store::open(&root));
        assert_eq!(
            must(receipt::serve(
                &store,
                tenant.clone(),
                ReceiptLookupKey::Idempotency(hash)
            ))
            .metadata
            .verification_level,
            VerificationLevel::COMMITTED
        );
    }
    drop(store);
    must(std::fs::remove_dir_all(root));
}

#[test]
fn served_native_owner_requires_committed_identity() {
    let _serial = must(native_preview::FIXTURE_LOCK.lock());
    let mut fixture = native_preview::Fixture::new();
    let (prepare, local_grant) = fixture.request();
    let credential = fixture.credential.clone();
    let owner_public_key = fixture.account.layerx_public_key();
    let request = || HumanNativeSendOwnerPreviewRequestV1 {
        credential: credential.clone(),
        request_id: 539,
        owner_public_key,
        purpose_expires_at_ms: prepare.not_after_height,
        commitment: [11; 32],
        economic_action: [8; 32],
        prepare: prepare.clone(),
        local_grant: local_grant.clone(),
    };
    must(fixture.identity.lock()).verification_level = VerificationLevel::STATE_PROVEN;
    assert!(matches!(
        fixture
            .owner
            .native_send_owner_preview_v1(&fixture.peer, request()),
        Err(HumanOperationError::Refused)
    ));
    must(fixture.identity.lock()).verification_level = VerificationLevel::COMMITTED;
    let response = must(
        fixture
            .owner
            .native_send_owner_preview_v1(&fixture.peer, request()),
    );
    assert!(!response.bytes().is_empty());
}

#[path = "../src/session_control/mcp_record.rs"]
mod mcp_record;
use layerx_agentd::session_control::{mcp_store_evidence_for_tool, McpInvocationError};
use mcp_record::{
    McpInvocationPhase, McpInvocationRecord, McpNativeAssociation, McpReceiptAssociation,
};

#[test]
fn persisted_mcp_receipt_ranks_normalize_after_restart() {
    let tenant = require_ok(TenantId::new("committed-mcp-codec"), "tenant");
    let root = std::env::temp_dir().join(format!("committed-mcp-codec-{}", std::process::id()));
    let store = require_ok(Store::open(&root), "store");
    let tool = "layerx.native_send";
    let idempotency_key = [2_u8; 32];
    let receipt_bytes = include_bytes!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    );
    let vectors: serde_json::Value = require_ok(
        serde_json::from_slice(receipt_bytes),
        "node producer vectors",
    );
    let canonical = require_some(
        vectors["cases"][0]["signedReceipt"].as_str(),
        "signed receipt",
    );
    let canonical = require_ok(
        hex::decode(canonical.trim_start_matches("0x")),
        "receipt bytes",
    );
    let mut record = McpInvocationRecord {
        schema: "layerx.mcp.invocation.v1".to_owned(),
        tool_name: Some(tool.to_owned()),
        native_preparation: Some(McpNativeAssociation {
            preparation_id: [4; 32],
            idempotency_key,
        }),
        reconciled_receipt: Some(McpReceiptAssociation {
            activity_id: [5; 32],
            receipt_digest: Sha256::digest(&canonical).into(),
            receipt_bytes: canonical,
            global_sequence: 7,
            verification_rank: layerx_types::verify::VerificationLevel::COMMITTED.wire_rank(),
            result_code: 0,
        }),
        tenant: tenant.as_str().to_owned(),
        actor: b"did:layerx:owner".to_vec(),
        session: [1; 32],
        generation: 1,
        idempotency_key,
        request_digest: [3; 32],
        core_sequence: 10,
        phase: McpInvocationPhase::Settled,
        response: Some(b"committed".to_vec()),
        events: Vec::new(),
    };
    // Use the invocation writer and index, then reopen the durable consumer.
    record.append_event(1, "native_receipt_reconciled", 1, 10);
    let mut input = (tool.len() as u64).to_be_bytes().to_vec();
    input.extend_from_slice(tool.as_bytes());
    input.extend_from_slice(&idempotency_key);
    continue_persisted_mcp_receipt_ranks_normalize_after_restart_1_repair_22_31_539(
        ContinuePersistedMcpReceiptRanksNormalizeAfterRestart1Repair2231539Context {
            tenant,
            root,
            store,
            tool,
            idempotency_key,
            record,
            input,
        },
    );
}

struct ContinuePersistedMcpReceiptRanksNormalizeAfterRestart1Repair2231539Context {
    tenant: TenantId,
    root: std::path::PathBuf,
    store: Store,
    tool: &'static str,
    idempotency_key: [u8; 32],
    record: McpInvocationRecord,
    input: Vec<u8>,
}

fn continue_persisted_mcp_receipt_ranks_normalize_after_restart_1_repair_22_31_539(
    context: ContinuePersistedMcpReceiptRanksNormalizeAfterRestart1Repair2231539Context,
) {
    let ContinuePersistedMcpReceiptRanksNormalizeAfterRestart1Repair2231539Context {
        tenant,
        root,
        mut store,
        tool,
        idempotency_key,
        record,
        input,
    } = context;

    let key = must(TenantKey::new(
        tenant.clone(),
        ObjectKind::Idempotency,
        [
            b"mcp-invocation-v1/".as_slice(),
            Sha256::digest(&input).as_slice(),
        ]
        .concat(),
    ));
    must(store.put_local(key.clone(), must(record.encode())));
    let original = require_some(store.get(&key), "written invocation")
        .bytes()
        .to_vec();
    drop(store);
    for rank in [6, 4, 5, 3, 2, 0, 7, u8::MAX] {
        let mut store = require_ok(Store::open(&root), "reopen");
        let mut bytes: serde_json::Value =
            require_ok(serde_json::from_slice(&original), "written JSON");
        bytes["reconciled_receipt"]["verification_rank"] = serde_json::json!(rank);
        let encoded = must(serde_json::to_vec(&bytes));
        if matches!(rank, 0 | 2 | 7 | u8::MAX) {
            assert!(
                matches!(
                    McpInvocationRecord::decode(&encoded),
                    Err(McpInvocationError::Corrupt)
                ),
                "rank {rank}"
            );
            assert!(
                matches!(
                    store.put_local(key.clone(), encoded),
                    Err(layerx_agentd::store::StoreError::Corrupt(_))
                ),
                "rank {rank}"
            );
        } else {
            must(store.put_local(key.clone(), encoded));
            drop(store);
            let reopened = must(Store::open(&root));
            let decoded = require_some(
                must(mcp_store_evidence_for_tool(
                    &reopened,
                    &tenant,
                    idempotency_key,
                    Some(tool),
                )),
                "found invocation",
            );
            assert_eq!(
                decoded["reconciled_receipt"]["verification_rank"],
                if rank == 3 { 3 } else { 6 }
            );
            assert_eq!(
                decoded["response"],
                serde_json::json!(b"committed".to_vec())
            );
        }
    }
    require_ok(std::fs::remove_dir_all(root), "cleanup");
}

#[path = "repair_22_31_219.rs"]
mod managed_lifecycle;

#[test]
fn persisted_key_policy_current_and_legacy_levels_reach_served_lifecycle_after_restart() {
    for rank in [6, 4, 5, 3, 2, 0, 7, u8::MAX] {
        managed_lifecycle::assert_persisted_key_policy_rank_over_socket(rank);
    }
}
