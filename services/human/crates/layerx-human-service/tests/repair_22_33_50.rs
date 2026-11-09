//! Task 22.33.50; audit 22.30.106 finding 12; acceptance 22998.1.
//! Finding 12: finding12_recorded_paxd_receipt_transport_recovers_before_replay,
//! finding12_committed_label_requires_retained_consensus_evidence, and
//! finding12_control_transport_preserves_checkpoint_verification.
//! Closure is held if the real control interface cannot carry the verified evidence.
//! The original local adversarial cases remain in agent_controls_rungs.rs.
//! The inherited baseline failures are retained in repair_22_33_50.kvx; this
//! continuation uses only the declared build and hf-done gates for qualification.
//! FinalizationEvidence.verification carries the committed rung rank without VerificationLevel translation.
//! Wiring request (node/consensus domain): daemon/committed-fixture must retain
//! paxd control transactions, signed receipts, pinned keys, committed headers,
//! validator votes and inclusion proofs for pause, resume, limit and archive.
//! The available committed-payExact recording is never relabeled as a control.
//! Scope forbids spec/GOTCHA edits; this record retains the mapping and wiring.
//! Exact candidate revision and qualification evidence are recorded by the gates.
//! Setup repair: paxd receipts use AuthenticatedReceipt ingress, not the legacy
//! AuthorizedBatch outcome store, which rejects this operation shape.

#[path = "support/committed_controls.rs"]
mod committed_controls;

use std::fmt::Debug;
use std::fs;
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::TcpListener;
use std::os::unix::net::UnixListener;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::thread;
use std::time::{Duration, Instant};

use layerx_agentd::human::{self, HumanOperationError, HumanRefusal, HumanRequest, HumanResponse};
use layerx_agentd::human_receipt_frame::{ReceiptLookupAnswer, ReceiptLookupFrame};
use layerx_agentd::managed_agent::{self, FinalizationEvidence, ManagedAgent, ManagedAgentCheck};
use layerx_agentd::receipt::{self, ReceiptLookupKey};
use layerx_agentd::store::{ObjectKind, Store as AgentStore, TenantId, TenantKey};
use layerx_client::rpc::receipt::{
    lookup_authenticated_with_commit, AuthenticatedReceipt, Lookup, LookupContext,
};
use layerx_client::rpc::receipt_commit::ReceiptCommitEvidence;
use layerx_client::sequencer::SequencerEndpoint;
use layerx_human_service::activity::{
    verify_receipt_export_with_consensus, ConsensusAuthority, EvidenceBundle, ReceiptFact,
    UnverifiedReason, VerifyError,
};
use layerx_human_service::frame::{read_frame, write_frame, Limits};
use layerx_human_service::journeys::{
    AgentBoundary, AgentBoundaryError, ReceiptKeyEntry, ReceiptLookup, ReceiptMaterial,
};
use layerx_human_service::server::agent_runtime::{
    AgentFinalizationEvidence, AgentRuntime, TraceContext,
};
use layerx_human_service::store::{PrincipalStore, RowKey, Table};
use layerx_human_test_support as support;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::payload::ModuleRegistry;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const TRACE: &str = "trc_22335022335022335022335022335000";
const MAX_FRAME: usize = 1_048_576;

#[track_caller]
fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("recorded control evidence: {error:?}"))
}

struct Directory(PathBuf);

impl Directory {
    fn new(label: &str) -> Self {
        let path = support::directory(label);
        checked(fs::create_dir_all(&path));
        Self(path)
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn tenant() -> TenantId {
    checked(TenantId::new("tenant-recorded-controls"))
}

fn evidence_key() -> TenantKey {
    checked(TenantKey::new(
        tenant(),
        ObjectKind::Configuration,
        b"recorded-paxd-committed-evidence".to_vec(),
    ))
}

fn authorized(fact: &ReceiptFact, keys: &[ReceiptKeyEntry]) -> AuthorizedBatch {
    let receipt = checked(layerx_wire::receipt::decode(&fact.canonical_receipt_bytes));
    let protocol = receipt.protocol().expect("paxd protocol receipt");
    let pins: Vec<_> = keys
        .iter()
        .map(|key| {
            SequencerAuthorization::new(
                key.key_id,
                key.public_key,
                key.first_height,
                key.last_height,
            )
        })
        .collect();
    checked(AuthorizedBatch::from_receipt_keys(
        &fact.canonical_receipt_bytes,
        pins.as_slice(),
        protocol.asset(),
        protocol.previous_state_root(),
        protocol.resulting_state_root(),
    ))
}

fn authenticate_recording(fact: &ReceiptFact, keys: &[ReceiptKeyEntry]) -> AuthenticatedReceipt {
    let recorded = committed_controls::recorded_rpc_receipt();
    let commit = committed_controls::recorded_commit();
    let listener = checked(TcpListener::bind("127.0.0.1:0"));
    checked(listener.set_nonblocking(true));
    let endpoint = checked(SequencerEndpoint::new(
        &format!("http://{}/", checked(listener.local_addr())),
        &[],
    ));
    let hash = fact
        .tx_hash
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<String>();
    let mut replies = vec![
        (
            "pax_sequencerReceipt",
            json!([format!("0x{hash}")]),
            recorded,
        ),
        (
            "eth_getTransactionReceipt",
            json!([format!("0x{hash}")]),
            Value::Null,
        ),
    ];
    replies.extend(commit.replies);
    let server = thread::spawn(move || {
        // The exact recorded Comet header, votes and batch inclusion establish
        // commitment even without an Ethereum landing receipt.
        for (method, params, answer) in replies {
            let deadline = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "recorded RPC request deadline");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("recorded RPC accept: {error}"),
                }
            };
            checked(stream.set_read_timeout(Some(Duration::from_secs(5))));
            checked(stream.set_write_timeout(Some(Duration::from_secs(5))));
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(checked(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(checked(value.trim().parse::<usize>()));
                }
            }
            let length = length.expect("bounded JSON-RPC content length");
            assert!(length <= 64 * 1024);
            let mut body = vec![0; length];
            checked(reader.read_exact(&mut body));
            let request: Value = checked(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], params);
            let body = json!({"jsonrpc": "2.0", "id": request["id"], "result": answer}).to_string();
            let mut stream = reader.into_inner();
            checked(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()));
            checked(stream.flush());
        }
    });
    let activity_ids = [fact.tx_hash];
    let evidence = ReceiptCommitEvidence {
        verifier: &commit.verifier,
        covering_height: commit.height,
        batch_transaction: &commit.batch_transaction,
        transaction_proof: &commit.batch.proof,
        activity_ids: &activity_ids,
    };
    let result = lookup_authenticated_with_commit(
        &endpoint,
        fact.tx_hash,
        LookupContext { receipt_keys: keys },
        &evidence,
    );
    checked(server.join());
    let Lookup::Verified(receipt) = checked(result) else {
        panic!("recorded paxd sequencer signature must authenticate");
    };
    assert_eq!(receipt.canonical_bytes(), fact.canonical_receipt_bytes);
    assert_eq!(receipt.rung(), Rung::Committed);
    assert!(receipt.landing().is_none());
    let header = receipt
        .committed_header()
        .expect("authenticated committed header");
    assert_eq!(
        header.height(),
        i64::try_from(commit.height).expect("height")
    );
    assert_eq!(
        receipt.covering_block(),
        Some((commit.height, header.header_hash()))
    );
    receipt
}

fn persist_recording(root: &Path) -> (Vec<u8>, [u8; 32]) {
    let (id, fact, keys, consensus) = committed_controls::recorded_receipt();
    let principal = support::principal("alice");
    let bundle = checked(EvidenceBundle::receipt(id, fact.clone(), &principal));
    let digest = checked(bundle.digest());
    checked(bundle.verify_with_consensus(digest, &principal, &keys, &consensus));
    let bytes = checked(bundle.encode());
    let mut store = checked(AgentStore::open(root));
    let authenticated = authenticate_recording(&fact, &keys);
    let served = checked(receipt::store_verified_if_absent(
        &mut store,
        tenant(),
        fact.tx_hash,
        &fact.canonical_receipt_bytes,
        &authenticated,
    ));
    // receipt.rs:699 assigns COMMITTED after the commit-capable lookup verifies it.
    assert_eq!(
        served.metadata.verification_level.rung(),
        Some(Rung::Committed)
    );
    checked(store.put_local(evidence_key(), bytes.clone()));
    drop(store);
    (bytes, digest)
}

fn retained_recording(
    store: &AgentStore,
    keys: &[ReceiptKeyEntry],
    consensus: &[ConsensusAuthority],
) -> (ReceiptLookupFrame, Vec<u8>) {
    let bytes = store
        .get(&evidence_key())
        .expect("retained full evidence")
        .bytes()
        .to_vec();
    let bundle = checked(EvidenceBundle::decode(&bytes));
    checked(bundle.verify_with_consensus(
        checked(bundle.digest()),
        &support::principal("alice"),
        keys,
        consensus,
    ));
    let export = &bundle.protocol_evidence()[0];
    let report = checked(verify_receipt_export_with_consensus(
        export, keys, consensus,
    ));
    assert_eq!(report.verified_receipts, 1);
    let fact = &export.receipts[0];
    let served = checked(receipt::serve(
        store,
        tenant(),
        ReceiptLookupKey::Idempotency(fact.tx_hash),
    ));
    assert_eq!(
        report.achieved_levels,
        vec![served.metadata.verification_level]
    );
    assert_eq!(report.achieved_levels, vec![VerificationLevel::COMMITTED]);
    assert_eq!(
        served.metadata.verification_level,
        VerificationLevel::COMMITTED
    );
    assert_eq!(served.canonical_bytes, fact.canonical_receipt_bytes);
    assert_eq!(
        served.metadata.verification_level.rung(),
        Some(Rung::Committed),
        "reopened receipt metadata must retain its authenticated node state"
    );
    let evidence = checked(receipt::serve_evidence(store, tenant(), fact.tx_hash))
        .expect("retained authenticated transaction evidence");
    assert_eq!(evidence.canonical_receipt, fact.canonical_receipt_bytes);
    assert_eq!(evidence.metadata.rung, Rung::Committed);
    let inventory = checked(receipt::evidence_inventory(store, &tenant()));
    assert!(inventory.without_evidence.is_empty());
    assert_eq!(inventory.with_current_evidence.len(), 1);
    assert_eq!(
        inventory.with_current_evidence[0]
            .evidence
            .canonical_receipt,
        served.canonical_bytes
    );
    let key = keys[0];
    let level = report.achieved_levels[0];
    (
        ReceiptLookupFrame {
            canonical_bytes: served.canonical_bytes.clone(),
            authorised_batch: authorized(fact, keys),
            verification_level: level,
            tx_hash: served.metadata.activity_id,
            receipt_hash: Sha256::digest(&served.canonical_bytes).into(),
            block_height: fact.block_height,
            rung: fact.rung,
            signed_receipt: served.canonical_bytes,
            receipt_key_id: key.key_id,
            public_key: key.public_key,
            first_height: key.first_height,
            last_height: key.last_height,
            inclusion_height: fact.block_height,
        },
        bytes,
    )
}

// The loopback peer uses the real request decoder, receipt store, managed-agent
// consumer and response encoders. It has no scripted successful control result.
fn exchange<T: Send + 'static>(
    socket: &Path,
    request: impl FnOnce(HumanRequest) -> (Result<HumanResponse, HumanOperationError>, T)
        + Send
        + 'static,
    call: impl FnOnce(&mut AgentRuntime) -> Result<ReceiptLookup, AgentBoundaryError>,
) -> (Result<ReceiptLookup, AgentBoundaryError>, T) {
    let listener = checked(UnixListener::bind(socket));
    checked(listener.set_nonblocking(true));
    let server = thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(5);
        let mut stream = loop {
            match listener.accept() {
                Ok((stream, _)) => break stream,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(Instant::now() < deadline, "control peer request deadline");
                    thread::sleep(Duration::from_millis(1));
                }
                Err(error) => panic!("control peer accept: {error}"),
            }
        };
        checked(stream.set_read_timeout(Some(Duration::from_secs(5))));
        checked(stream.set_write_timeout(Some(Duration::from_secs(5))));
        let frame = checked(read_frame(&mut stream, MAX_FRAME));
        assert_eq!(&frame[..8], b"LXHAGT02");
        assert_eq!(&frame[8..44], TRACE.as_bytes());
        let length = u32::from_be_bytes(checked(frame[44..48].try_into())) as usize;
        assert_eq!(length, frame.len() - 48);
        let (result, returned) = request(checked(human::decode_request(frame[48..].to_vec())));
        let response = human::encode_response(result);
        let mut envelope = frame[..44].to_vec();
        envelope.extend_from_slice(&checked(u32::try_from(response.len())).to_be_bytes());
        envelope.extend_from_slice(&response);
        checked(write_frame(&mut stream, &envelope, MAX_FRAME));
        returned
    });
    let mut runtime = checked(AgentRuntime::new_with_clock(
        socket,
        Limits {
            maximum_frame_bytes: MAX_FRAME,
            maximum_connections: 1,
            maximum_streams: 1,
            maximum_queued_bytes: MAX_FRAME,
            deadline: Duration::from_secs(5),
        },
        checked(ModuleRegistry::new(&[])),
        Arc::new(support::StepClock::new(100_000)),
    ));
    let _trace = checked(TraceContext::enter(TRACE));
    let result = call(&mut runtime);
    let returned = checked(server.join());
    drop(runtime);
    checked(fs::remove_file(socket));
    (result, returned)
}

#[test]
fn finding12_recorded_paxd_receipt_transport_recovers_before_replay() {
    let root = Directory::new("repair223350-replay");
    let agent_root = root.0.join("agent");
    let human_root = root.0.join("human");
    let (encoded, digest) = persist_recording(&agent_root);
    let (_, fact, keys, consensus) = committed_controls::recorded_receipt();
    let retention = support::retention_uniform(10_000);
    let (store, tenancy) = support::install_and_open(
        &human_root,
        &support::tenancy(&[("alice", "tenant-recorded-controls")]),
        retention,
    );
    drop(store);
    let key = checked(RowKey::new("recorded-paxd-control-evidence"));

    // No live store, server or runtime survives between the original delivery
    // and replay; the second delivery is produced entirely from reopened state.
    for replay in [false, true] {
        let store = checked(AgentStore::open(&agent_root));
        let (frame, retained) = retained_recording(&store, &keys, &consensus);
        assert_eq!(retained, encoded);
        drop(store);
        let expected = frame.clone();
        let root_for_peer = agent_root.clone();
        let peer_keys = keys.clone();
        let peer_consensus = consensus.clone();
        let (answer, ()) = exchange(
            &root.0.join("receipt.sock"),
            move |request| {
                let HumanRequest::ReceiptLookup {
                    idempotency_key,
                    expected_activity_id,
                } = request
                else {
                    panic!("expected real receipt lookup request");
                };
                assert_eq!(idempotency_key, expected.tx_hash);
                assert_eq!(expected_activity_id, expected.tx_hash);
                let store = checked(AgentStore::open(&root_for_peer));
                let (frame, _) = retained_recording(&store, &peer_keys, &peer_consensus);
                let encoded = checked(ReceiptLookupAnswer::Found(frame).encode());
                (Ok(checked(HumanResponse::new(encoded))), ())
            },
            |runtime| runtime.receipt_by_idempotency_key(fact.tx_hash, fact.tx_hash),
        );
        let ReceiptLookup::Found(material) = checked(answer) else {
            panic!("recorded receipt must be found after reopen");
        };
        assert_material(&material, &frame);
        let mut store = checked(PrincipalStore::open(&human_root, retention, tenancy));
        let mut scope = checked(store.principal(&support::principal("alice")));
        if replay {
            assert_eq!(
                scope
                    .get(Table::Support, &key)
                    .expect("retained human evidence")
                    .bytes(),
                encoded
            );
            assert_eq!(scope.keys(Table::Support), vec![key.clone()]);
        }
        let bundle = checked(EvidenceBundle::decode(&retained));
        assert_eq!(
            bundle.protocol_evidence()[0].receipts[0].canonical_receipt_bytes,
            material.canonical_bytes
        );
        checked(bundle.verify_with_consensus(digest, scope.principal(), &keys, &consensus));
        checked(scope.put(Table::Support, key.clone(), 100, retained));
        assert_eq!(scope.keys(Table::Support), vec![key.clone()]);
        drop(scope);
        drop(store);
    }
}

fn assert_material(material: &ReceiptMaterial, frame: &ReceiptLookupFrame) {
    assert_eq!(material.canonical_bytes, frame.canonical_bytes);
    assert_eq!(material.signed_receipt, frame.signed_receipt);
    assert_eq!(material.tx_hash, frame.tx_hash);
    assert_eq!(material.receipt_hash, frame.receipt_hash);
    assert_eq!(material.block_height, frame.block_height);
    assert_eq!(material.rung, Rung::Committed);
    assert_eq!(material.verification_level, VerificationLevel::COMMITTED);
    assert_eq!(material.receipt_key.public_key, frame.public_key);
    assert_eq!(material.inclusion_height, frame.inclusion_height);
}

#[test]
fn finding12_committed_label_requires_retained_consensus_evidence() {
    let (_, fact, keys, consensus) = committed_controls::recorded_receipt();
    checked(EvidenceBundle::verify_receipt_with_consensus(
        &fact, &keys, &consensus,
    ));
    let mut missing_commit = fact.clone();
    missing_commit.committed = None;
    assert!(matches!(
        EvidenceBundle::verify_receipt_with_consensus(&missing_commit, &keys, &consensus),
        Err(VerifyError::Unverified(UnverifiedReason::Tampered))
    ));
    let mut invalid_vote = fact.clone();
    invalid_vote
        .committed
        .as_mut()
        .expect("commit")
        .commit
        .votes[0]
        .signature[0] ^= 1;
    assert!(
        EvidenceBundle::verify_receipt_with_consensus(&invalid_vote, &keys, &consensus).is_err()
    );
    let mut wrong_chain = consensus.clone();
    wrong_chain[0].chain_id.push_str("-foreign");
    assert!(EvidenceBundle::verify_receipt_with_consensus(&fact, &keys, &wrong_chain).is_err());
    let mut invalid_batch = fact.clone();
    invalid_batch
        .committed
        .as_mut()
        .expect("commit")
        .batch_transaction[0] ^= 1;
    assert!(
        EvidenceBundle::verify_receipt_with_consensus(&invalid_batch, &keys, &consensus).is_err()
    );
    let mut substituted = fact.clone();
    substituted.canonical_receipt_bytes[0] ^= 1;
    // Recomputing a local digest cannot authenticate substituted producer bytes.
    substituted.receipt_hash = Sha256::digest(&substituted.canonical_receipt_bytes).into();
    assert!(
        EvidenceBundle::verify_receipt_with_consensus(&substituted, &keys, &consensus).is_err()
    );
    assert!(EvidenceBundle::verify_receipt_with_consensus(&fact, &keys, &[]).is_err());
}

#[test]
fn finding12_control_transport_preserves_checkpoint_verification() {
    let root = Directory::new("repair223350-control-wire");
    let agent_root = root.0.join("agent");
    persist_recording(&agent_root);
    let (_, fact, keys, consensus) = committed_controls::recorded_receipt();
    let mut store = checked(AgentStore::open(&agent_root));
    let (frame, _) = retained_recording(&store, &keys, &consensus);
    let seed = human::HumanAgentLifecycleSeed {
        agent_id: format!("agt_{}", "08".repeat(32)),
        name: "recorded controls".into(),
        purpose: "committed payment refusal".into(),
        currency: "PAX".into(),
        monthly_limit: 100,
        period_start: 1,
        period_end: 10_000,
        created_at: 1,
        updated_at: 1,
        verified_evidence: vec![frame.receipt_hash],
        actor: format!("did:layerx:{}", "03".repeat(32)),
        primary_authority: "03".repeat(32),
        custody_key: "recorded-controls-owner".into(),
        custody_public_key: [3; 32],
        owner_account: "recorded-controls:main".into(),
        budget_account: "recorded-controls:budget:managed".into(),
        budget_asset: [4; 32],
        purpose_hash: [5; 32],
        recovery_root: [6; 32],
        recovery_threshold: 1,
        capability_id: [7; 32],
        activity_types: vec![7],
        counterparties: vec![[4; 32]],
        assets: vec![[4; 32]],
        amount_ceiling: 100,
        rate_maximum_uses: 10,
        rate_window_sequences: 10,
        purposes: vec!["recorded controls".into()],
        capability_expiry_sequence: 10_000,
        session_scopes: vec!["prepare".into()],
        session_expiry_unix_seconds: 10_000,
        protocol_grant_id: [7; 32],
        budget_period_seconds: 100,
        budget_expiry_seconds: 10_000,
        initial_funding: 100,
        network_id: 125,
        creation_receipt_roots: vec![frame.receipt_hash],
    };
    let mut agent = checked(ManagedAgent::from_creation(
        &seed,
        &seed.actor,
        [10; 32],
        [11; 32],
        1,
    ));
    agent.agent_id = "agt_recorded_controls".into();
    agent.context.agent_id = agent.agent_id.clone();
    checked(managed_agent::publish_creation(
        &mut store,
        &tenant(),
        &agent,
    ));
    drop(store);
    let evidence = AgentFinalizationEvidence {
        action_key: frame.tx_hash,
        activity_id: frame.tx_hash,
        receipt_digest: frame.receipt_hash,
        observed_sequence: frame.block_height,
        verification: frame.rung.wire_rank(),
        finalized_at: 100,
    };
    let (response, consumed) = exchange(
        &root.0.join("control.sock"),
        move |request| {
            let HumanRequest::AgentLimit {
                agent_id,
                monthly_limit,
                currency,
                replacement_budget_id,
                evidence,
            } = request
            else {
                panic!("expected production limit request");
            };
            assert_eq!(agent_id, "agt_recorded_controls");
            assert_eq!(monthly_limit, 500);
            assert_eq!(currency, "PAX");
            assert_eq!(evidence.rung, Rung::Committed.wire_rank());
            let consumed = FinalizationEvidence::from(evidence);
            let mut store = checked(AgentStore::open(&agent_root));
            let before = checked(fs::read(agent_root.join("store.bin")));
            let result = managed_agent::finalize_limit(
                &mut store,
                &tenant(),
                &agent_id,
                monthly_limit,
                &currency,
                replacement_budget_id,
                consumed,
            );
            // A genuine committed payment must never authorize a budget change.
            assert_eq!(
                result,
                Err(HumanOperationError::Typed(HumanRefusal::ManagedAgent(
                    ManagedAgentCheck::FinalizationEvidenceLookup,
                )))
            );
            assert_eq!(checked(fs::read(agent_root.join("store.bin"))), before);
            (result, consumed)
        },
        |runtime| {
            runtime
                .agent_limit("agt_recorded_controls", 500, "PAX", [0x31; 32], evidence)
                .map(|_| panic!("payment receipt cannot finalize a control"))
        },
    );
    assert_eq!(response, Err(AgentBoundaryError::Refused));
    assert_eq!(consumed.receipt_digest, fact.receipt_hash);
    assert_eq!(consumed.activity_id, fact.tx_hash);
    // FinalizationEvidence::from copies the request's rung rank into verification;
    // valid_finalization_shape requires the committed rung rank.
    assert_eq!(
        consumed.verification,
        Rung::Committed.wire_rank(),
        "Committed control transport lost checkpoint verification before the managed-agent consumer",
    );
}
