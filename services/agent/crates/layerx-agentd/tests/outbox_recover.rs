use ed25519_dalek::SigningKey;
use layerx_agentd::budget::{hold_unknown, ProtocolBudgetState, UnknownReservation};
use layerx_agentd::human_runtime::{
    recover_tenant_budget_with_evidence, BudgetRecoveryRequest, RecoveryRefusal,
};
use layerx_agentd::outbox::RecoveryError;
use layerx_agentd::protocol_evidence::{EvidenceAuthority, RawReceiptEvidence};
use layerx_agentd::receipt::{
    evidence_inventory, persist_evidence, serve_evidence, store as store_receipt,
    store_verified_if_absent, ReceiptEvidenceInventory, ReceiptStoreError,
    TransactionReceiptEvidence,
};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_proof::receipt::AuthorizedBatch;
use layerx_wire::hash::program_execution_batch_id;

use support::{directory, tenant};

mod support;

const BUDGET_ID: [u8; 32] = [0x51; 32];
const SEQUENCER_SEED: [u8; 32] = [0x3a; 32];

fn authorised_batch(global_sequence: u64) -> AuthorizedBatch {
    let batch_id =
        program_execution_batch_id([0x21; 32], [0x25; 32], global_sequence, global_sequence, 7)
            .unwrap_or_else(|error| panic!("execution batch id: {error:?}"));
    AuthorizedBatch::new(
        batch_id,
        [0x24; 32],
        [0x21; 32],
        [0x22; 32],
        SigningKey::from_bytes(&SEQUENCER_SEED)
            .verifying_key()
            .to_bytes(),
    )
}

fn store_served_receipt(
    durable: &mut Store,
    tenant: &TenantId,
    idempotency_key: [u8; 32],
    raw: &RawReceiptEvidence,
    global_sequence: u64,
) {
    let metadata = store_receipt(
        durable,
        tenant.clone(),
        idempotency_key,
        raw.canonical_receipt(),
        &authorised_batch(global_sequence),
    )
    .unwrap_or_else(|error| panic!("store receipt: {error:?}"));
    assert_eq!(metadata.global_sequence, global_sequence);
    assert_eq!(metadata.idempotency_key, idempotency_key);
}

#[test]
fn legacy_receipt_evidence_is_refused_after_restart() {
    let root = directory("receipt-evidence");
    let tenant = tenant();
    let raw = support::raw_receipt_at([0x11; 32], 0, 25, 100);
    let older = support::raw_receipt_at([0x12; 32], 0, 10, 101);
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    store_served_receipt(&mut durable, &tenant, [0x44; 32], &raw, 100);
    store_served_receipt(&mut durable, &tenant, [0x45; 32], &older, 101);
    assert_eq!(
        serve_evidence(&durable, tenant.clone(), [0x44; 32])
            .unwrap_or_else(|error| panic!("serve before persist: {error:?}")),
        None
    );
    let record = persist_evidence(&mut durable, tenant.clone(), [0x44; 32], &raw)
        .unwrap_or_else(|error| panic!("persist evidence: {error:?}"));
    assert_eq!(record.idempotency_key, [0x44; 32]);
    assert_eq!(record.activity_id, [0x11; 32]);
    assert_eq!(record.global_sequence, 100);
    assert_eq!(record.evidence, raw);
    assert_eq!(
        persist_evidence(&mut durable, tenant.clone(), [0x44; 32], &raw)
            .unwrap_or_else(|error| panic!("persist evidence again: {error:?}")),
        record
    );
    let altered_signature = RawReceiptEvidence::new(
        raw.canonical_receipt().to_vec(),
        raw.proof().clone(),
        raw.canonical_header().to_vec(),
        [0x99; 64],
    );
    assert!(matches!(
        persist_evidence(&mut durable, tenant.clone(), [0x44; 32], &altered_signature),
        Err(ReceiptStoreError::Corrupt)
    ));
    assert!(matches!(
        persist_evidence(&mut durable, tenant.clone(), [0x45; 32], &raw),
        Err(ReceiptStoreError::Corrupt)
    ));
    assert!(matches!(
        persist_evidence(&mut durable, tenant.clone(), [0x46; 32], &raw),
        Err(ReceiptStoreError::Missing)
    ));
    drop(durable);

    let reopened = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
    assert!(matches!(
        serve_evidence(&reopened, tenant.clone(), [0x44; 32]),
        Err(ReceiptStoreError::EvidenceVersion)
    ));
    assert_eq!(
        serve_evidence(&reopened, tenant.clone(), [0x45; 32])
            .unwrap_or_else(|error| panic!("serve older after reopen: {error:?}")),
        None
    );
    let inventory = evidence_inventory(&reopened, &tenant)
        .unwrap_or_else(|error| panic!("legacy receipt inventory after restart: {error:?}"));
    assert_eq!(inventory.with_evidence, vec![record]);
    assert!(inventory.with_current_evidence.is_empty());
    assert_eq!(inventory.without_evidence.len(), 1);
    assert_eq!(inventory.without_evidence[0].idempotency_key, [0x45; 32]);
    assert_eq!(inventory.without_evidence[0].activity_id, [0x12; 32]);
    assert_eq!(inventory.without_evidence[0].global_sequence, 101);
    let _ = std::fs::remove_dir_all(&root);
}

#[test]
fn archived_legacy_receipt_evidence_is_refused_after_restart() {
    let root = directory("archived-receipt-evidence");
    let tenant = tenant();
    let idempotency_key = [0x44; 32];
    let raw = support::raw_receipt_at([0x11; 32], 0, 25, 100);
    let mut durable = lint_success(Store::open(&root), "store");
    store_served_receipt(&mut durable, &tenant, idempotency_key, &raw, 100);
    lint_success(
        persist_evidence(&mut durable, tenant.clone(), idempotency_key, &raw),
        "persist legacy evidence",
    );
    let public_key = lint_success(
        TenantKey::new(
            tenant.clone(),
            ObjectKind::Configuration,
            [b"receipt-evidence:".as_slice(), &idempotency_key].concat(),
        ),
        "public evidence key",
    );
    let legacy_key = lint_success(
        TenantKey::new(
            tenant.clone(),
            ObjectKind::Configuration,
            [b"receipt-legacy-evidence:".as_slice(), &idempotency_key].concat(),
        ),
        "archived evidence key",
    );
    let encoded = lint_present(durable.get(&public_key), "encoded evidence")
        .bytes()
        .to_vec();
    lint_success(durable.put_local(legacy_key, encoded), "archive evidence");
    assert!(lint_success(
        durable.remove_local(&public_key),
        "remove public evidence"
    ));
    drop(durable);

    let reopened = lint_success(Store::open(&root), "reopen");
    assert!(matches!(
        evidence_inventory(&reopened, &tenant),
        Err(ReceiptStoreError::EvidenceVersion)
    ));
    drop(reopened);
    lint_success(std::fs::remove_dir_all(root), "cleanup");
}

fn seed_startup_recovery(root: &std::path::Path, tenant_a: &TenantId, tenant_b: &TenantId) {
    let mut durable = Store::open(root).unwrap_or_else(|error| panic!("store: {error}"));
    let capture = support::current_node::restart_capture();
    let receipt = support::current_node::authenticated_receipt(&capture);
    lint_success(
        durable.put_local(
            lint_success(
                TenantKey::new(
                    tenant_a.clone(),
                    ObjectKind::PreparedActivity,
                    receipt.activity_id().to_vec(),
                ),
                "transaction key",
            ),
            capture.bytes("signed_transaction"),
        ),
        "retain exact signed node transaction",
    );
    let evidence = TransactionReceiptEvidence::from_authenticated(&receipt)
        .unwrap_or_else(|error| panic!("current transaction evidence: {error:?}"));
    store_verified_if_absent(
        &mut durable,
        tenant_a.clone(),
        receipt.activity_id(),
        receipt.canonical_bytes(),
        &receipt,
    )
    .unwrap_or_else(|error| panic!("persist current transaction receipt: {error:?}"));
    assert_eq!(
        serve_evidence(&durable, tenant_a.clone(), receipt.activity_id())
            .unwrap_or_else(|error| panic!("serve current evidence: {error:?}")),
        Some(evidence)
    );
    let unevidenced = support::raw_receipt_at([0x12; 32], 0, 10, 101);
    store_served_receipt(&mut durable, tenant_b, [0x45; 32], &unevidenced, 101);
    hold_unknown(
        &mut durable,
        &UnknownReservation {
            tenant: tenant_b.clone(),
            id: [1; 32],
            amount: 300,
            expiry_sequence: 10,
            resolved: None,
        },
    )
    .unwrap_or_else(|error| panic!("hold unknown: {error:?}"));
}

fn recovery_request<'a>(
    budget_id: [u8; 32],
    protocol: &ProtocolBudgetState,
    verifier: &EvidenceAuthority,
    inventory: &'a ReceiptEvidenceInventory,
) -> BudgetRecoveryRequest<'a, TransactionReceiptEvidence> {
    BudgetRecoveryRequest {
        budget_id,
        protocol_budget: protocol.clone(),
        verifier: verifier.clone(),
        receipts_with_evidence: &inventory.with_current_evidence,
        receipts_without_evidence: &inventory.without_evidence,
        ceiling_maximum: 1_000,
        current_sequence: 1,
    }
}

fn assert_evidenced_tenant_admits_writes(
    reopened: &mut Store,
    tenant: &TenantId,
    protocol: &ProtocolBudgetState,
    verifier: &EvidenceAuthority,
    inventory: &ReceiptEvidenceInventory,
    capture: &support::current_node::RestartCapture,
) {
    let recovery = recover_tenant_budget_with_evidence(
        reopened,
        tenant,
        &recovery_request(BUDGET_ID, protocol, verifier, inventory),
    )
    .unwrap_or_else(|error| panic!("recover tenant a: {error:?}"));
    let accounting = recovery.recovered.budget_accounting;
    let requested_amount = capture.amount("budget_spent");
    assert_eq!(requested_amount, 25);
    assert_eq!(accounting.protocol_consumed, Some(requested_amount));
    assert_eq!(accounting.receipt_consumed, requested_amount,
        "node producer: transaction value {}, receipt amount {}, requested spend {}; accounting={accounting:?}, admission={:?}",
        capture.amount("transaction_value"), capture.amount("receipt_amount"), requested_amount, recovery.admission);
    assert_eq!(accounting.held_unresolved, 0);
    assert_eq!(accounting.unresolved_count, 0);
    assert!(accounting.reconciled);
    assert!(recovery.recovered.queued_for_transmission.is_empty());
    assert!(recovery.recovered.awaiting_receipt_resolution.is_empty());
    assert!(recovery
        .recovered
        .ceiling
        .snapshot()
        .is_ok_and(|snapshot| snapshot.reconciled));
    assert!(recovery.recovered.require_write_ready().is_ok());
    assert!(recovery.admission.is_ok());
}

fn assert_unevidenced_tenant_holds_its_reservation(
    reopened: &mut Store,
    tenant: &TenantId,
    protocol: &ProtocolBudgetState,
    verifier: &EvidenceAuthority,
    inventory: &ReceiptEvidenceInventory,
) {
    let recovery = recover_tenant_budget_with_evidence(
        reopened,
        tenant,
        &recovery_request(BUDGET_ID, protocol, verifier, inventory),
    )
    .unwrap_or_else(|error| panic!("recover tenant b: {error:?}"));
    let accounting = recovery.recovered.budget_accounting;
    assert_eq!(accounting.protocol_consumed, Some(25));
    assert_eq!(accounting.receipt_consumed, 0);
    assert_eq!(accounting.held_unresolved, 300);
    assert_eq!(accounting.unresolved_count, 1);
    assert!(accounting.reconciled);
    assert!(recovery
        .recovered
        .ceiling
        .snapshot()
        .is_ok_and(|snapshot| snapshot.reconciled));
    assert!(matches!(
        recovery.admission,
        Err(RecoveryRefusal::EvidenceMissing { budget_id, count })
            if budget_id == BUDGET_ID && count == 1
    ));
}

fn assert_tenant_without_receipts_blocks_writes(
    reopened: &mut Store,
    protocol: &ProtocolBudgetState,
    verifier: &EvidenceAuthority,
) {
    let tenant_d = TenantId::new("tenant-d").unwrap_or_else(|error| panic!("tenant: {error}"));
    let empty = ReceiptEvidenceInventory::default();
    let recovery = recover_tenant_budget_with_evidence(
        reopened,
        &tenant_d,
        &recovery_request(BUDGET_ID, protocol, verifier, &empty),
    )
    .unwrap_or_else(|error| panic!("recover tenant d: {error:?}"));
    let accounting = recovery.recovered.budget_accounting;
    assert_eq!(accounting.protocol_consumed, Some(25));
    assert_eq!(accounting.receipt_consumed, 0);
    assert_eq!(accounting.held_unresolved, 0);
    assert!(!accounting.reconciled);
    assert_eq!(
        recovery
            .recovered
            .ceiling
            .snapshot()
            .map(|snapshot| snapshot.reconciled),
        Ok(false)
    );
    assert!(matches!(
        recovery.recovered.require_write_ready(),
        Err(RecoveryError::WritesBlocked)
    ));
    assert!(matches!(
        recovery.admission,
        Err(RecoveryRefusal::WritesBlocked { budget_id, accounting: reported })
            if budget_id == BUDGET_ID && reported == accounting
    ));
}

fn assert_budget_state_refusals(
    reopened: &mut Store,
    tenant: &TenantId,
    protocol: &ProtocolBudgetState,
    verifier: &EvidenceAuthority,
    inventory: &ReceiptEvidenceInventory,
) {
    assert!(matches!(
        recover_tenant_budget_with_evidence(
            reopened,
            tenant,
            &recovery_request([0x52; 32], protocol, verifier, inventory),
        ),
        Err(RecoveryRefusal::BudgetState { budget_id, .. }) if budget_id == [0x52; 32]
    ));
    let corrupted = ProtocolBudgetState {
        evidence: support::raw_state_leaf(vec![1, 2, 3], 150),
    };
    assert!(matches!(
        recover_tenant_budget_with_evidence(
            reopened,
            tenant,
            &recovery_request(BUDGET_ID, &corrupted, verifier, inventory),
        ),
        Err(RecoveryRefusal::BudgetState { budget_id, .. }) if budget_id == BUDGET_ID
    ));
}

#[test]
fn startup_recovery_reconciles_evidenced_tenant_and_holds_tenant_without_evidence() {
    let root = directory("startup-recovery");
    let tenant_a = tenant();
    let tenant_b = TenantId::new("tenant-b").unwrap_or_else(|error| panic!("tenant: {error}"));
    seed_startup_recovery(&root, &tenant_a, &tenant_b);

    let mut reopened = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
    let capture = support::current_node::restart_capture();
    assert_eq!(capture.budget_id(), BUDGET_ID);
    assert_eq!(capture.amount("budget_spent"), 25);
    let verifier = capture.authority();
    let (protocol, _) = support::current_node::committed_restart_budget(&capture);
    let receipt = support::current_node::authenticated_receipt(&capture);
    let transaction_key = lint_success(
        TenantKey::new(
            tenant_a.clone(),
            ObjectKind::PreparedActivity,
            receipt.activity_id().to_vec(),
        ),
        "transaction key",
    );
    let retained_transaction = lint_present(
        reopened.get(&transaction_key),
        "signed transaction survives restart",
    )
    .bytes();
    assert_eq!(retained_transaction, capture.bytes("signed_transaction"));
    assert_eq!(
        layerx_crypto::evm_transaction::transaction_hash(retained_transaction),
        receipt.activity_id()
    );
    lint_success(
        layerx_crypto::evm_transaction::decode_signed(retained_transaction),
        "retained canonical zero-value signed transaction",
    );
    assert_eq!(capture.amount("transaction_value"), 0);
    let recovered_evidence = lint_present(
        serve_evidence(&reopened, tenant_a.clone(), receipt.activity_id())
            .unwrap_or_else(|error| panic!("reopen current evidence: {error:?}")),
        "current transaction evidence survives restart",
    );
    assert_eq!(
        recovered_evidence.canonical_receipt,
        receipt.canonical_bytes()
    );
    let inventory_a = evidence_inventory(&reopened, &tenant_a)
        .unwrap_or_else(|error| panic!("inventory a: {error:?}"));
    assert!(inventory_a.with_evidence.is_empty());
    assert_eq!(inventory_a.with_current_evidence.len(), 1);
    let current = &inventory_a.with_current_evidence[0];
    assert_eq!(current.idempotency_key, receipt.activity_id());
    assert_eq!(current.activity_id, receipt.activity_id());
    assert_eq!(current.global_sequence, capture.sequence());
    assert_eq!(current.evidence, recovered_evidence);
    let authenticated = lint_success(
        verifier.verify_current_receipt(&current.evidence),
        "reopened receipt verifies under governed node keys",
    );
    assert_eq!(authenticated.activity_id(), receipt.activity_id());
    assert_eq!(authenticated.global_sequence(), capture.sequence());
    assert_eq!(authenticated.amount(), capture.amount("receipt_amount"));
    assert_eq!(authenticated.amount(), capture.amount("transaction_value"));
    assert_eq!(authenticated.result_code(), 0);
    assert!(inventory_a.without_evidence.is_empty());

    let inventory_b = evidence_inventory(&reopened, &tenant_b)
        .unwrap_or_else(|error| panic!("inventory b: {error:?}"));
    assert!(inventory_b.with_evidence.is_empty());
    assert!(inventory_b.with_current_evidence.is_empty());
    assert_eq!(inventory_b.without_evidence.len(), 1);
    assert_eq!(inventory_b.without_evidence[0].idempotency_key, [0x45; 32]);
    assert_eq!(inventory_b.without_evidence[0].activity_id, [0x12; 32]);
    assert_eq!(inventory_b.without_evidence[0].global_sequence, 101);
    let (reserved_budget, _) = support::current_node::committed_budget(BUDGET_ID, 25);
    assert_unevidenced_tenant_holds_its_reservation(
        &mut reopened,
        &tenant_b,
        &reserved_budget,
        &verifier,
        &inventory_b,
    );

    assert_tenant_without_receipts_blocks_writes(&mut reopened, &reserved_budget, &verifier);
    assert_budget_state_refusals(&mut reopened, &tenant_a, &protocol, &verifier, &inventory_a);
    assert_evidenced_tenant_admits_writes(
        &mut reopened,
        &tenant_a,
        &protocol,
        &verifier,
        &inventory_a,
        &capture,
    );
    let _ = std::fs::remove_dir_all(&root);
}

#[track_caller]
fn lint_success<T, E: std::fmt::Debug>(result: Result<T, E>, context: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{context}: {error:?}"),
    }
}

#[track_caller]
fn lint_present<T>(value: Option<T>, context: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{context}: required value is absent"),
    }
}
