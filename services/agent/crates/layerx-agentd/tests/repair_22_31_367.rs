mod support;

use layerx_agentd::budget::ProtocolBudgetState;
use layerx_agentd::outbox::{recover, Outbox, OutboxError, RecoveryInputs, SubmissionState};
use layerx_agentd::store::{ObjectKind, Store, TenantKey};
use layerx_client::sequencer::SequencerBlock;

fn recovery_inputs() -> RecoveryInputs<'static> {
    RecoveryInputs {
        verifier: support::pinned_evidence_verifier(),
        unknown_budget_ids: &[],
        budget_receipts: &[],
        protocol_budget: ProtocolBudgetState {
            evidence: support::raw_state_leaf(0_u128.to_be_bytes().to_vec(), 50),
        },
        ceiling_maximum: 1_000,
        ceiling_receipts: &[],
        unknown_ceiling_reservations: &[],
        current_sequence: 1,
    }
}

fn outbox_key(id: [u8; 32]) -> TenantKey {
    TenantKey::new(support::tenant(), ObjectKind::Outbox, id.to_vec())
        .unwrap_or_else(|error| panic!("outbox key: {error:?}"))
}

#[test]
fn outbox_history_roundtrip_bound_preserves_recovery_and_receipt_evidence() {
    let root = support::directory("repair-22-31-367-history");
    let exact = support::signed_transaction(1);
    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("open: {error:?}"));
    let mut outbox = Outbox::default();
    let id = outbox
        .enqueue(&mut store, support::tenant(), exact.clone())
        .unwrap_or_else(|error| panic!("enqueue: {error:?}"));
    for index in 2..1_021 {
        let status = outbox
            .transition(
                &mut store,
                id,
                SubmissionState::Submitted,
                format!("submission observation {index}"),
                None,
            )
            .unwrap_or_else(|error| panic!("transition {index}: {error:?}"));
        assert_eq!(status.transitions.len(), index + 1);
    }
    let before = outbox
        .status(id)
        .cloned()
        .unwrap_or_else(|| panic!("status"));
    assert_eq!(before.state, SubmissionState::Submitted);
    assert_eq!(before.transitions.len(), 1_021);
    assert_eq!(before.transitions[0].from, SubmissionState::Prepared);
    assert_eq!(before.transitions[1].to, SubmissionState::Queued);
    for (index, event) in before.transitions.iter().enumerate().skip(2) {
        assert_eq!(event.cause, format!("submission observation {index}"));
        assert_eq!(event.to, SubmissionState::Submitted);
        assert_eq!(event.receipt, None);
    }
    let encoded = store
        .get(&outbox_key(id))
        .unwrap_or_else(|| panic!("record"))
        .bytes()
        .to_vec();
    finish_outbox_history_roundtrip_bound_preserves_recovery_and_receipt_evidence_2_repair_22_31_367(FinishOutboxHistoryRoundtripBoundPreservesRecoveryAndReceiptEvidence2Repair2231367Context { root, exact, store, outbox, id, before, encoded });
}

struct FinishOutboxHistoryRoundtripBoundPreservesRecoveryAndReceiptEvidence2Repair2231367Context {
    root: std::path::PathBuf,
    exact: Vec<u8>,
    store: Store,
    outbox: Outbox,
    id: [u8; 32],
    before: layerx_agentd::outbox::SubmissionStatus,
    encoded: Vec<u8>,
}

fn finish_outbox_history_roundtrip_bound_preserves_recovery_and_receipt_evidence_2_repair_22_31_367(
    context: FinishOutboxHistoryRoundtripBoundPreservesRecoveryAndReceiptEvidence2Repair2231367Context,
) {
    let FinishOutboxHistoryRoundtripBoundPreservesRecoveryAndReceiptEvidence2Repair2231367Context {
        root,
        exact,
        mut store,
        mut outbox,
        id,
        before,
        encoded,
    } = context;

    let disk =
        std::fs::read(root.join("store.bin")).unwrap_or_else(|error| panic!("read: {error:?}"));
    assert!(matches!(
        outbox.transition(
            &mut store,
            id,
            SubmissionState::Submitted,
            "one too many",
            None
        ),
        Err(OutboxError::HistoryFull)
    ));
    assert_eq!(outbox.status(id), Some(&before));
    assert_eq!(
        store
            .get(&outbox_key(id))
            .unwrap_or_else(|| panic!("record"))
            .bytes(),
        encoded
    );
    assert_eq!(
        std::fs::read(root.join("store.bin")).unwrap_or_else(|error| panic!("read: {error:?}")),
        disk
    );
    drop(outbox);
    drop(store);

    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error:?}"));
    let mut restored = Outbox::default();
    restored
        .restore(&store, support::tenant(), id)
        .unwrap_or_else(|error| panic!("restore: {error:?}"));
    assert_eq!(restored.status(id), Some(&before));
    assert_eq!(
        restored
            .exact_signed_bytes(id)
            .unwrap_or_else(|error| panic!("bytes: {error:?}")),
        exact
    );
    drop(restored);

    let recovered = recover(&mut store, &support::tenant(), &recovery_inputs())
        .unwrap_or_else(|error| panic!("recover: {error:?}"));
    assert_eq!(recovered.awaiting_receipt_resolution, vec![id]);
    let unknown = recovered
        .outbox
        .status(id)
        .unwrap_or_else(|| panic!("unknown"));
    assert_eq!(unknown.state, SubmissionState::Unknown);
    assert_eq!(unknown.transitions.len(), 1_022);
    assert_eq!(&unknown.transitions[..1_021], before.transitions.as_slice());
    assert_eq!(
        unknown.transitions[1_021].cause,
        "restart made transport outcome indeterminate"
    );
    drop(recovered);
    drop(store);

    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("reopen unknown: {error:?}"));
    let recovered = recover(&mut store, &support::tenant(), &recovery_inputs())
        .unwrap_or_else(|error| panic!("recover unknown: {error:?}"));
    finish_outbox_history_roundtrip_bound_preserves_recovery_and_receipt_evidence_1_repair_22_31_367(FinishOutboxHistoryRoundtripBoundPreservesRecoveryAndReceiptEvidence1Repair2231367Context { root, exact, id, before, store, recovered });
}

struct FinishOutboxHistoryRoundtripBoundPreservesRecoveryAndReceiptEvidence1Repair2231367Context {
    root: std::path::PathBuf,
    exact: Vec<u8>,
    id: [u8; 32],
    before: layerx_agentd::outbox::SubmissionStatus,
    store: Store,
    recovered: layerx_agentd::outbox::RecoveredOutbox,
}

fn finish_outbox_history_roundtrip_bound_preserves_recovery_and_receipt_evidence_1_repair_22_31_367(
    context: FinishOutboxHistoryRoundtripBoundPreservesRecoveryAndReceiptEvidence1Repair2231367Context,
) {
    let FinishOutboxHistoryRoundtripBoundPreservesRecoveryAndReceiptEvidence1Repair2231367Context {
        root,
        exact,
        id,
        before,
        mut store,
        mut recovered,
    } = context;

    assert_eq!(
        recovered
            .outbox
            .status(id)
            .unwrap_or_else(|| panic!("unknown"))
            .transitions
            .len(),
        1_022
    );
    let receipt = support::pinned_evidence_verifier()
        .verify_receipt(&support::raw_receipt(id, 0, 1))
        .unwrap_or_else(|error| panic!("verify receipt: {error:?}"));
    let receipt_ref = receipt.receipt_ref();
    let executed = recovered
        .outbox
        .record_execution(&mut store, id, receipt, 9, 0)
        .unwrap_or_else(|error| panic!("execute: {error:?}"));
    assert_eq!(executed.transitions.len(), 1_023);
    let evidence = executed
        .evidence
        .unwrap_or_else(|| panic!("execution evidence"));
    assert_eq!(evidence.receipt_ref(), receipt_ref);
    assert_eq!(evidence.block_height(), 9);
    assert_eq!(evidence.transaction_index(), 0);
    let block = SequencerBlock {
        number: 9,
        hash: [0x55; 32],
        parent_hash: [0x44; 32],
        timestamp: 1_000,
        transactions_root: [0x33; 32],
        receipts_root: [0x66; 32],
        transactions: vec![exact.clone()],
    };
    let committed = recovered
        .outbox
        .reconcile_committed_block(&mut store, id, &block)
        .unwrap_or_else(|error| panic!("commit: {error:?}"));
    assert_eq!(committed.state, SubmissionState::Committed);
    assert_eq!(committed.transitions.len(), 1_024);
    assert_eq!(committed.evidence, Some(evidence));
    assert_eq!(committed.transitions[1_022].receipt, Some(evidence));
    assert_eq!(committed.transitions[1_023].receipt, Some(evidence));
    assert_eq!(
        &committed.transitions[..1_021],
        before.transitions.as_slice()
    );
    drop(recovered);
    drop(store);

    let mut store =
        Store::open(&root).unwrap_or_else(|error| panic!("reopen committed: {error:?}"));
    let recovered = recover(&mut store, &support::tenant(), &recovery_inputs())
        .unwrap_or_else(|error| panic!("recover committed: {error:?}"));
    assert!(recovered.awaiting_receipt_resolution.is_empty());
    assert_eq!(recovered.outbox.status(id), Some(&committed));
    assert_eq!(
        recovered
            .outbox
            .exact_signed_bytes(id)
            .unwrap_or_else(|error| panic!("bytes: {error:?}")),
        exact
    );
    drop(recovered);
    drop(store);
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("cleanup: {error:?}"));
}

#[test]
fn outbox_history_roundtrip_bound_enforces_cause_bytes_atomically() {
    let root = support::directory("repair-22-31-367-cause");
    let exact = support::signed_transaction(2);
    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("open: {error:?}"));
    let mut outbox = Outbox::default();
    let id = outbox
        .enqueue(&mut store, support::tenant(), exact.clone())
        .unwrap_or_else(|error| panic!("enqueue: {error:?}"));
    let cause = "é".repeat(524_288);
    assert_eq!(cause.len(), 1_048_576);
    let before = outbox
        .transition(
            &mut store,
            id,
            SubmissionState::Submitted,
            cause.clone(),
            None,
        )
        .unwrap_or_else(|error| panic!("maximum cause: {error:?}"));
    let encoded = store
        .get(&outbox_key(id))
        .unwrap_or_else(|| panic!("record"))
        .bytes()
        .to_vec();
    let disk =
        std::fs::read(root.join("store.bin")).unwrap_or_else(|error| panic!("read: {error:?}"));
    for excessive in [format!("{cause}a"), "a".repeat(1_048_577)] {
        assert_eq!(excessive.len(), 1_048_577);
        assert!(matches!(
            outbox.transition(&mut store, id, SubmissionState::Unknown, excessive, None),
            Err(OutboxError::CauseTooLong)
        ));
        assert_eq!(outbox.status(id), Some(&before));
        assert_eq!(
            store
                .get(&outbox_key(id))
                .unwrap_or_else(|| panic!("record"))
                .bytes(),
            encoded
        );
        assert_eq!(
            std::fs::read(root.join("store.bin")).unwrap_or_else(|error| panic!("read: {error:?}")),
            disk
        );
    }
    drop(outbox);
    drop(store);

    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error:?}"));
    let mut restored = Outbox::default();
    restored
        .restore(&store, support::tenant(), id)
        .unwrap_or_else(|error| panic!("restore: {error:?}"));
    assert_eq!(restored.status(id), Some(&before));
    let failed = restored
        .transition(
            &mut store,
            id,
            SubmissionState::Failed,
            "definitive rejection",
            None,
        )
        .unwrap_or_else(|error| panic!("fail: {error:?}"));
    assert_eq!(failed.transitions[2].cause, cause);
    drop(restored);
    drop(store);

    let store = Store::open(&root).unwrap_or_else(|error| panic!("reopen failed: {error:?}"));
    let mut restored = Outbox::default();
    restored
        .restore(&store, support::tenant(), id)
        .unwrap_or_else(|error| panic!("restore failed: {error:?}"));
    assert_eq!(restored.status(id), Some(&failed));
    assert_eq!(
        restored
            .exact_signed_bytes(id)
            .unwrap_or_else(|error| panic!("bytes: {error:?}")),
        exact
    );
    drop(restored);
    drop(store);
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("cleanup: {error:?}"));
}
