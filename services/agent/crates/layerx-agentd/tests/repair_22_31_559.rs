use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::JoinHandle;
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::budget::{
    self, BudgetReceiptEvidence, PersistedReceipt, ProtocolBudgetState, RestartError,
};
use layerx_agentd::human_runtime::{
    recover_tenant_budget_with_evidence, BudgetRecoveryRequest, RecoveryRefusal,
};
use layerx_agentd::protocol_evidence::{EvidenceAuthority, ReceiptEvidenceError};
use layerx_agentd::receipt::{
    self, ReceiptEvidenceInventory, ReceiptEvidenceRecord, ReceiptStoreError,
};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::rpc::receipt::{
    lookup_authenticated, AuthenticatedReceipt, Lookup, LookupContext,
};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_proof::receipt::{AuthorizedBatch, ReceiptCheck};
use layerx_wire::hash::program_execution_batch_id;
use serde_json::{json, Value};
use sha2::Digest as _;

mod support;

const BUDGET_ID: [u8; 32] = [0x51; 32];
const LIMIT: u128 = 1_000_000_000_000_000_000_000_000;

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn producer(name: &str) -> Value {
    let transcript: Value = must(serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )));
    assert_eq!(
        transcript["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    let id = match name {
        "success" => "V1-pax-transfer",
        "reverted" => "V2-layerxledger-revert",
        _ => panic!("unknown producer case"),
    };
    transcript["cases"]
        .as_array()
        .expect("producer cases")
        .iter()
        .find(|case| case["id"] == id)
        .expect("producer case")
        .clone()
}

fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    let text = value.as_str().expect("hex string");
    must(must(hex::decode(text.strip_prefix("0x").unwrap_or(text))).try_into())
}

struct NodeTranscript {
    url: String,
    stop: Arc<AtomicBool>,
    requests: Arc<Mutex<Vec<Value>>>,
    worker: Option<JoinHandle<()>>,
}
impl NodeTranscript {
    fn open(case: &Value) -> Self {
        let case = case.clone();
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let url = format!("http://{}/", must(listener.local_addr()));
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let requests = Arc::new(Mutex::new(Vec::new()));
        let recorded = Arc::clone(&requests);
        let worker = std::thread::spawn(move || {
            while !stopping.load(Ordering::Acquire) {
                let stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("{error}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(5))));
                must(stream.set_write_timeout(Some(Duration::from_secs(5))));
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(must(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(must(value.trim().parse::<usize>()));
                    }
                }
                let length = length.unwrap_or_else(|| panic!("content length"));
                assert!(length <= 65_536);
                let mut body = vec![0; length];
                must(reader.read_exact(&mut body));
                let request: Value = must(serde_json::from_slice(&body));
                assert_eq!(request["jsonrpc"], "2.0");
                let result = match request["method"]
                    .as_str()
                    .unwrap_or_else(|| panic!("method"))
                {
                    "eth_chainId" => json!("0x7d"),
                    "px_getReceiptKeys" => json!({"keys": [case["pinnedKey"].clone()]}),
                    "pax_sequencerReceipt" => {
                        assert_eq!(request["params"], json!([case["receiptJson"]["txHash"]]));
                        case["receiptJson"].clone()
                    }
                    "eth_getTransactionReceipt" => {
                        assert_eq!(request["params"], json!([case["receiptJson"]["txHash"]]));
                        Value::Null
                    }
                    method => panic!("unexpected node method {method}"),
                };
                let response =
                    json!({"jsonrpc":"2.0", "id": request["id"], "result": result}).to_string();
                must(write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()));
                must(reader.get_mut().flush());
                must(recorded.lock()).push(request);
            }
        });
        Self {
            url,
            stop,
            requests,
            worker: Some(worker),
        }
    }
    fn endpoint(&self) -> SequencerEndpoint {
        must(SequencerEndpoint::new(&self.url, &[]))
    }
    fn authenticated(&self, case: &Value) -> AuthenticatedReceipt {
        let keys = must(self.endpoint().receipt_keys());
        let Lookup::Verified(receipt) = must(lookup_authenticated(
            &self.endpoint(),
            fixed(&case["receiptJson"]["txHash"]),
            LookupContext {
                receipt_keys: &keys,
            },
        )) else {
            panic!("authenticated node receipt")
        };
        receipt
    }
}
impl Drop for NodeTranscript {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            must(worker.join());
        }
    }
}

fn authority(node: &NodeTranscript, version: u16) -> EvidenceAuthority {
    let endpoint = node.endpoint();
    assert_eq!(must(endpoint.chain_id()), 125);
    must(
        must(EvidenceAuthority::pinned_to_handshake(
            version,
            125,
            SigningKey::from_bytes(&[0x3a; 32])
                .verifying_key()
                .to_bytes(),
        ))
        .with_receipt_keys(must(endpoint.receipt_keys())),
    )
}

fn protocol(spent: u128, start: u64, length: u64) -> ProtocolBudgetState {
    let mut bytes = vec![0; 278];
    bytes[1] = 1;
    bytes[2..34].copy_from_slice(&BUDGET_ID);
    for (offset, value) in [
        (34, [0x52; 32]),
        (66, [0x53; 32]),
        (98, [0x24; 32]),
        (130, [0x54; 32]),
    ] {
        bytes[offset..offset + 32].copy_from_slice(&value);
    }
    bytes[162..178].copy_from_slice(&LIMIT.to_be_bytes());
    bytes[178..194].copy_from_slice(&LIMIT.to_be_bytes());
    bytes[210..226].copy_from_slice(&spent.to_be_bytes());
    bytes[242..250].copy_from_slice(&length.to_be_bytes());
    bytes[250..258].copy_from_slice(&start.to_be_bytes());
    bytes[258..266].copy_from_slice(&5_000_u64.to_be_bytes());
    ProtocolBudgetState {
        evidence: support::raw_state_leaf_with(
            bytes,
            99,
            support::StateHeaderIdentity {
                signing_seed: [0x3a; 32],
                protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
                network_id: 125,
                epoch: 2,
                batch_number: 7,
            },
        ),
    }
}

fn evidence_key(tenant: &TenantId, hash: [u8; 32], prefix: &[u8]) -> TenantKey {
    must(TenantKey::new(
        tenant.clone(),
        ObjectKind::Configuration,
        [prefix, hash.as_slice()].concat(),
    ))
}

fn request<'a>(
    inventory: &'a ReceiptEvidenceInventory,
    verifier: &EvidenceAuthority,
    protocol: ProtocolBudgetState,
) -> BudgetRecoveryRequest<'a, receipt::TransactionReceiptEvidence> {
    BudgetRecoveryRequest {
        budget_id: BUDGET_ID,
        protocol_budget: protocol,
        verifier: verifier.clone(),
        receipts_with_evidence: &inventory.with_current_evidence,
        receipts_without_evidence: &inventory.without_evidence,
        ceiling_maximum: LIMIT,
        current_sequence: 99,
    }
}

#[test]
fn restart_recovers_current_receipts_holds_missing_evidence_and_refuses_corruption() {
    let case = producer("success");
    let node = NodeTranscript::open(&case);
    let authenticated = node.authenticated(&case);
    let verifier = authority(&node, layerx_wire::limits::PROTOCOL_VERSION);
    let amount = authenticated
        .receipt()
        .protocol()
        .expect("protocol")
        .amount();
    assert!(amount > 0);
    let hash = authenticated.activity_id();
    let tenant_a = must(TenantId::new("tenant-a"));
    let tenant_b = must(TenantId::new("tenant-b"));
    let root = support::directory("repair-559-restart");
    {
        let mut store = must(Store::open(&root));
        // Current authenticated ingress binds idempotency to the transaction hash.
        must(receipt::store_verified_if_absent(
            &mut store,
            tenant_a.clone(),
            hash,
            authenticated.canonical_bytes(),
            &authenticated,
        ));
        must(receipt::store_verified_if_absent(
            &mut store,
            tenant_b.clone(),
            hash,
            authenticated.canonical_bytes(),
            &authenticated,
        ));
        assert!(must(store.remove_local(&evidence_key(
            &tenant_b,
            hash,
            b"receipt-evidence:"
        ))));
    }
    let mut store = must(Store::open(&root));
    let inventory = must(receipt::evidence_inventory(&store, &tenant_a));
    assert!(inventory.with_evidence.is_empty());
    assert!(inventory.without_evidence.is_empty());
    assert_eq!(inventory.with_current_evidence.len(), 1);
    let record = &inventory.with_current_evidence[0];
    assert_eq!(record.idempotency_key, hash);
    assert_eq!(record.activity_id, hash);
    assert_eq!(record.global_sequence, authenticated.global_sequence());
    assert_eq!(
        record.evidence.canonical_receipt,
        authenticated.canonical_bytes()
    );
    let verified = must(verifier.verify_current_receipt(&record.evidence));
    assert_eq!(
        verified.receipt_ref(),
        <[u8; 32]>::from(sha2::Sha256::digest(authenticated.canonical_bytes()))
    );
    assert_eq!(verified.activity_id(), hash);
    assert_eq!(verified.global_sequence(), record.global_sequence);
    assert_eq!(verified.amount(), amount);
    assert_eq!(verified.result_code(), 0);
    assert_eq!(
        verified.module_id(),
        authenticated
            .receipt()
            .protocol()
            .expect("protocol")
            .module_id()
    );
    assert_eq!(
        verified.operation(),
        authenticated
            .receipt()
            .protocol()
            .expect("protocol")
            .operation()
    );
    let recovery = must(recover_tenant_budget_with_evidence(
        &mut store,
        &tenant_a,
        &request(&inventory, &verifier, protocol(amount, 1, 100)),
    ));
    assert!(recovery.admission.is_ok());
    must(recovery.recovered.require_write_ready());
    assert_eq!(
        recovery.recovered.budget_accounting.receipt_consumed,
        amount
    );
    assert_eq!(
        recovery.recovered.budget_accounting.protocol_consumed,
        Some(amount)
    );
    assert!(recovery.recovered.budget_accounting.reconciled);
    // Recovery supplies no reservation-bound ceiling receipt applications.
    let ceiling = must(recovery.recovered.ceiling.snapshot());
    assert_eq!(ceiling.consumed, 0);
    assert!(ceiling.reconciled);

    let missing = must(receipt::evidence_inventory(&store, &tenant_b));
    assert!(missing.with_evidence.is_empty());
    assert!(missing.with_current_evidence.is_empty());
    assert_eq!(missing.without_evidence.len(), 1);
    let recovery = must(recover_tenant_budget_with_evidence(
        &mut store,
        &tenant_b,
        &request(&missing, &verifier, protocol(0, 1, 100)),
    ));
    assert!(matches!(
        recovery.admission,
        Err(RecoveryRefusal::EvidenceMissing {
            budget_id: BUDGET_ID,
            count: 1
        })
    ));

    let key = evidence_key(&tenant_a, hash, b"receipt-evidence:");
    let mut bytes = store.get(&key).expect("stored evidence").bytes().to_vec();
    // LXRE/v2 + LXRM/v2 + hash + height + index + status + gas + bloom.
    let signature_offset = 5 + 5 + 32 + 8 + 8 + 1 + 8 + 256;
    bytes[signature_offset] ^= 1;
    must(store.put_local(key, bytes));
    drop(store);
    let mut store = must(Store::open(&root));
    let corrupt = must(receipt::evidence_inventory(&store, &tenant_a));
    assert!(verifier
        .verify_current_receipt(&corrupt.with_current_evidence[0].evidence)
        .is_err());
    assert!(matches!(
        recover_tenant_budget_with_evidence(
            &mut store,
            &tenant_a,
            &request(&corrupt, &verifier, protocol(amount, 1, 100))
        ),
        Err(RecoveryRefusal::Recovery {
            budget_id: BUDGET_ID,
            ..
        })
    ));
    assert!(must(node.requests.lock())
        .iter()
        .any(|req| req["method"] == "px_getReceiptKeys"));
    drop(store);
    must(std::fs::remove_dir_all(root));
}

#[test]
fn governed_current_receipt_authority_enforces_signature_version_and_height_windows() {
    let case = producer("success");
    let node = NodeTranscript::open(&case);
    let authenticated = node.authenticated(&case);
    let raw = must(receipt::TransactionReceiptEvidence::from_authenticated(
        &authenticated,
    ));
    let verifier = authority(&node, layerx_wire::limits::PROTOCOL_VERSION);
    must(verifier.verify_current_receipt(&raw));
    // The handshake key belongs to state proof verification, not node receipt authority.
    let ungoverned = must(EvidenceAuthority::pinned_to_handshake(
        layerx_wire::limits::PROTOCOL_VERSION,
        125,
        SigningKey::from_bytes(&[0x3a; 32])
            .verifying_key()
            .to_bytes(),
    ));
    assert!(ungoverned.verify_current_receipt(&raw).is_err());
    let mut keys = must(node.endpoint().receipt_keys());
    keys[0].public_key = SigningKey::from_bytes(&[0x99; 32])
        .verifying_key()
        .to_bytes();
    let other_key = must(verifier.clone().with_receipt_keys(keys));
    assert!(
        matches!(other_key.verify_current_receipt(&raw), Err(ReceiptEvidenceError::Receipt(failure)) if failure.check == ReceiptCheck::SequencerSignature)
    );
    let mut keys = must(node.endpoint().receipt_keys());
    keys[0].first_height = raw.metadata.block_height + 1;
    let outside = must(verifier.clone().with_receipt_keys(keys));
    assert!(outside.verify_current_receipt(&raw).is_err());
    let mut keys = must(node.endpoint().receipt_keys());
    keys[0].first_height = raw.metadata.block_height - 1;
    keys[0].last_height = raw.metadata.block_height - 1;
    assert!(must(verifier.clone().with_receipt_keys(keys))
        .verify_current_receipt(&raw)
        .is_err());
    let mut keys = must(node.endpoint().receipt_keys());
    keys[0].first_height = raw.metadata.block_height;
    keys[0].last_height = raw.metadata.block_height;
    must(must(verifier.clone().with_receipt_keys(keys)).verify_current_receipt(&raw));
    let wrong_version = authority(
        &node,
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
    );
    assert!(matches!(
        wrong_version.verify_current_receipt(&raw),
        Err(ReceiptEvidenceError::ProtocolVersion)
    ));
    let mut changed_height = raw.clone();
    changed_height.metadata.block_height += 1;
    assert!(verifier.verify_current_receipt(&changed_height).is_err());
    let mut forged = raw.clone();
    let signature = &raw.metadata.receipt_signature[..64];
    let offset = forged
        .canonical_receipt
        .windows(64)
        .position(|bytes| bytes == signature)
        .expect("canonical signature");
    forged.canonical_receipt[offset] ^= 1;
    forged.metadata.receipt_signature[0] ^= 1;
    assert!(
        matches!(verifier.verify_current_receipt(&forged), Err(ReceiptEvidenceError::Receipt(failure)) if failure.check == ReceiptCheck::SequencerSignature)
    );
}

fn legacy(store: &mut Store, tenant: &TenantId) -> ReceiptEvidenceRecord {
    let original = support::raw_receipt_at([0x11; 32], 0, 25, 4);
    let mut header = original.canonical_header().to_vec();
    assert_eq!(
        must(layerx_wire::receipt::decode_batch_header(&header)).network_id(),
        42
    );
    // The canonical header places network_id after its version, count and first two tags.
    assert_eq!(&header[9..13], &42_u32.to_be_bytes());
    header[9..13].copy_from_slice(&125_u32.to_be_bytes());
    assert_eq!(
        must(layerx_wire::receipt::decode_batch_header(&header)).network_id(),
        125
    );
    let digest = must(layerx_wire::hash::batch_header_digest(&header));
    let signature = SigningKey::from_bytes(&[0x3a; 32]).sign(&digest).to_bytes();
    let raw = layerx_agentd::protocol_evidence::RawReceiptEvidence::new(
        original.canonical_receipt().to_vec(),
        original.proof().clone(),
        header,
        signature,
    );
    let authorized = AuthorizedBatch::new(
        must(program_execution_batch_id([0x21; 32], [0x25; 32], 4, 4, 7)),
        [0x24; 32],
        [0x21; 32],
        [0x22; 32],
        SigningKey::from_bytes(&[0x3a; 32])
            .verifying_key()
            .to_bytes(),
    );
    must(receipt::store(
        store,
        tenant.clone(),
        [0x45; 32],
        raw.canonical_receipt(),
        &authorized,
    ));
    must(receipt::persist_evidence(
        store,
        tenant.clone(),
        [0x45; 32],
        &raw,
    ))
}

#[test]
fn mixed_inventory_rebuilds_both_formats_and_preserves_replay_and_archive_refusals() {
    let case = producer("success");
    let node = NodeTranscript::open(&case);
    let authenticated = node.authenticated(&case);
    let verifier = authority(&node, layerx_wire::limits::PROTOCOL_VERSION);
    let amount = authenticated
        .receipt()
        .protocol()
        .expect("protocol")
        .amount();
    let tenant = must(TenantId::new("mixed"));
    let root = support::directory("repair-559-mixed");
    let mut store = must(Store::open(&root));
    let old = legacy(&mut store, &tenant);
    must(receipt::store_verified_if_absent(
        &mut store,
        tenant.clone(),
        authenticated.activity_id(),
        authenticated.canonical_bytes(),
        &authenticated,
    ));
    drop(store);
    let mut store = must(Store::open(&root));
    let inventory = must(receipt::evidence_inventory(&store, &tenant));
    assert_eq!(inventory.with_evidence, vec![old]);
    assert_eq!(inventory.with_current_evidence.len(), 1);
    assert!(inventory.without_evidence.is_empty());
    let receipts: Vec<ReceiptEvidenceRecord<BudgetReceiptEvidence>> = inventory
        .with_evidence
        .iter()
        .map(|record| ReceiptEvidenceRecord {
            idempotency_key: record.idempotency_key,
            activity_id: record.activity_id,
            global_sequence: record.global_sequence,
            evidence: record.evidence.clone().into(),
        })
        .chain(
            inventory
                .with_current_evidence
                .iter()
                .map(|record| ReceiptEvidenceRecord {
                    idempotency_key: record.idempotency_key,
                    activity_id: record.activity_id,
                    global_sequence: record.global_sequence,
                    evidence: record.evidence.clone().into(),
                }),
        )
        .collect();
    let request = BudgetRecoveryRequest {
        budget_id: BUDGET_ID,
        protocol_budget: protocol(amount + 25, 1, 100),
        verifier: verifier.clone(),
        receipts_with_evidence: &receipts,
        receipts_without_evidence: &inventory.without_evidence,
        ceiling_maximum: LIMIT,
        current_sequence: 99,
    };
    let recovery = must(recover_tenant_budget_with_evidence(
        &mut store, &tenant, &request,
    ));
    assert!(recovery.admission.is_ok());
    assert_eq!(
        recovery.recovered.budget_accounting.receipt_consumed,
        amount + 25
    );
    must(recovery.recovered.require_write_ready());
    let duplicate = PersistedReceipt {
        expected_activity_id: authenticated.activity_id(),
        evidence: BudgetReceiptEvidence::Current(
            inventory.with_current_evidence[0].evidence.clone(),
        ),
    };
    assert!(matches!(
        budget::rebuild_with_evidence(
            &store,
            &tenant,
            &[],
            &[duplicate.clone(), duplicate],
            &protocol(amount, 1, 100),
            &verifier
        ),
        Err(RestartError::DuplicateReceipt)
    ));
    let windowed = BudgetRecoveryRequest {
        protocol_budget: protocol(25, 4, 1),
        ..request
    };
    let recovery = must(recover_tenant_budget_with_evidence(
        &mut store, &tenant, &windowed,
    ));
    assert!(recovery.admission.is_ok());
    assert_eq!(recovery.recovered.budget_accounting.receipt_consumed, 25);
    let archive = evidence_key(&tenant, [0x45; 32], b"receipt-legacy-evidence:");
    let public = evidence_key(&tenant, [0x45; 32], b"receipt-evidence:");
    let bytes = store.get(&public).expect("legacy record").bytes().to_vec();
    must(store.put_local(archive, bytes));
    assert!(matches!(
        receipt::evidence_inventory(&store, &tenant),
        Err(ReceiptStoreError::EvidenceVersion)
    ));
    drop(store);
    must(std::fs::remove_dir_all(root));
}

#[test]
fn failed_current_receipt_does_not_consume_budget() {
    let case = producer("reverted");
    let node = NodeTranscript::open(&case);
    let authenticated = node.authenticated(&case);
    let verifier = authority(&node, layerx_wire::limits::PROTOCOL_VERSION);
    let tenant = must(TenantId::new("failed"));
    let root = support::directory("repair-559-failed");
    let mut store = must(Store::open(&root));
    must(receipt::store_verified_if_absent(
        &mut store,
        tenant.clone(),
        authenticated.activity_id(),
        authenticated.canonical_bytes(),
        &authenticated,
    ));
    let inventory = must(receipt::evidence_inventory(&store, &tenant));
    let verified =
        must(verifier.verify_current_receipt(&inventory.with_current_evidence[0].evidence));
    assert_ne!(verified.result_code(), 0);
    let recovery = must(recover_tenant_budget_with_evidence(
        &mut store,
        &tenant,
        &request(&inventory, &verifier, protocol(0, 1, 100)),
    ));
    assert_eq!(recovery.recovered.budget_accounting.receipt_consumed, 0);
    assert!(recovery.admission.is_ok());
    let key = evidence_key(&tenant, authenticated.activity_id(), b"receipt-evidence:");
    let mut bytes = store.get(&key).expect("current evidence").bytes().to_vec();
    *bytes.last_mut().expect("canonical signature") ^= 1;
    must(store.put_local(key, bytes));
    assert!(matches!(
        receipt::evidence_inventory(&store, &tenant),
        Err(ReceiptStoreError::Corrupt)
    ));
    drop(store);
    must(std::fs::remove_dir_all(root));
}
