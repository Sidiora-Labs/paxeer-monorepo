use ed25519_dalek::SigningKey;
use layerx_agentd::budget::{hold_unknown, ProtocolBudgetState, UnknownReservation};
use layerx_agentd::human_runtime::{recover_tenant_budget, BudgetRecoveryRequest, RecoveryRefusal};
use layerx_agentd::outbox::RecoveryError;
use layerx_agentd::protocol_evidence::{EvidenceAuthority, RawReceiptEvidence};
use layerx_agentd::receipt::{
    evidence_inventory, persist_evidence, serve_evidence, store as store_receipt,
    ReceiptEvidenceInventory, ReceiptStoreError,
};
use layerx_agentd::store::{Store, TenantId};
use layerx_proof::receipt::AuthorizedBatch;
use layerx_wire::hash::program_execution_batch_id;

use support::{directory, tenant};

mod support;

const BUDGET_ID: [u8; 32] = [0x51; 32];
const SEQUENCER_SEED: [u8; 32] = [0x3a; 32];

fn core_budget_record(per_period_limit: u128, spent_this_period: u128) -> Vec<u8> {
    let mut bytes = vec![0_u8; 278];
    bytes[1] = 1;
    bytes[2..34].copy_from_slice(&BUDGET_ID);
    bytes[34..66].copy_from_slice(&[0x52; 32]);
    bytes[66..98].copy_from_slice(&[0x53; 32]);
    bytes[98..130].copy_from_slice(&[0x24; 32]);
    bytes[130..162].copy_from_slice(&[0x54; 32]);
    bytes[162..178].copy_from_slice(&per_period_limit.to_be_bytes());
    bytes[178..194].copy_from_slice(&per_period_limit.to_be_bytes());
    bytes[194..210].copy_from_slice(&0_u128.to_be_bytes());
    bytes[210..226].copy_from_slice(&spent_this_period.to_be_bytes());
    bytes[226..242].copy_from_slice(&0_u128.to_be_bytes());
    bytes[242..250].copy_from_slice(&1_000_u64.to_be_bytes());
    bytes[250..258].copy_from_slice(&80_u64.to_be_bytes());
    bytes[258..266].copy_from_slice(&5_000_u64.to_be_bytes());
    bytes[266..274].copy_from_slice(&0_u64.to_be_bytes());
    bytes
}

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
    assert!(matches!(
        evidence_inventory(&reopened, &tenant),
        Err(ReceiptStoreError::EvidenceVersion)
    ));
    let _ = std::fs::remove_dir_all(&root);
}

fn seed_startup_recovery(root: &std::path::Path, tenant_a: &TenantId, tenant_b: &TenantId) {
    let mut durable = Store::open(root).unwrap_or_else(|error| panic!("store: {error}"));
    let evidenced = support::raw_receipt_at([0x11; 32], 0, 25, 100);
    store_served_receipt(&mut durable, tenant_a, [0x44; 32], &evidenced, 100);
    persist_evidence(&mut durable, tenant_a.clone(), [0x44; 32], &evidenced)
        .unwrap_or_else(|error| panic!("persist evidence: {error:?}"));
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
) -> BudgetRecoveryRequest<'a> {
    BudgetRecoveryRequest {
        budget_id,
        protocol_budget: protocol.clone(),
        verifier: verifier.clone(),
        receipts_with_evidence: &inventory.with_evidence,
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
) {
    let recovery = recover_tenant_budget(
        reopened,
        tenant,
        &recovery_request(BUDGET_ID, protocol, verifier, inventory),
    )
    .unwrap_or_else(|error| panic!("recover tenant a: {error:?}"));
    let accounting = recovery.recovered.budget_accounting;
    assert_eq!(accounting.protocol_consumed, Some(25));
    assert_eq!(accounting.receipt_consumed, 25);
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
    let recovery = recover_tenant_budget(
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
    let recovery = recover_tenant_budget(
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
        recover_tenant_budget(
            reopened,
            tenant,
            &recovery_request([0x52; 32], protocol, verifier, inventory),
        ),
        Err(RecoveryRefusal::BudgetState { budget_id, .. }) if budget_id == [0x52; 32]
    ));
    let corrupted = ProtocolBudgetState {
        evidence: support::corrupt_raw_state(&protocol.evidence, vec![1, 2, 3]),
    };
    assert!(matches!(
        recover_tenant_budget(
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
    let verifier = support::evidence_verifier();
    let protocol = ProtocolBudgetState {
        evidence: support::raw_state_leaf(core_budget_record(500, 25), 99),
    };
    let inventory_a = evidence_inventory(&reopened, &tenant_a)
        .unwrap_or_else(|error| panic!("inventory a: {error:?}"));
    assert_eq!(inventory_a.with_evidence.len(), 1);
    assert!(inventory_a.without_evidence.is_empty());
    assert_evidenced_tenant_admits_writes(
        &mut reopened,
        &tenant_a,
        &protocol,
        &verifier,
        &inventory_a,
    );

    let inventory_b = evidence_inventory(&reopened, &tenant_b)
        .unwrap_or_else(|error| panic!("inventory b: {error:?}"));
    assert!(inventory_b.with_evidence.is_empty());
    assert_eq!(inventory_b.without_evidence.len(), 1);
    assert_unevidenced_tenant_holds_its_reservation(
        &mut reopened,
        &tenant_b,
        &protocol,
        &verifier,
        &inventory_b,
    );

    assert_tenant_without_receipts_blocks_writes(&mut reopened, &protocol, &verifier);
    assert_budget_state_refusals(&mut reopened, &tenant_a, &protocol, &verifier, &inventory_a);
    let _ = std::fs::remove_dir_all(&root);
}
