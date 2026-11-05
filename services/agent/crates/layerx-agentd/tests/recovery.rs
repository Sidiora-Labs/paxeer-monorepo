mod support;

use layerx_agentd::budget::{
    hold_unknown, PersistedReceipt, ProtocolBudgetState, UnknownReservation,
};
use layerx_agentd::capability::{CeilingError, ReceiptApplication as CeilingReceipt};
use layerx_agentd::outbox::{
    recover, resolve_unknown, Outbox, RecoveryError, RecoveryInputs, SubmissionState,
    UnknownCeilingReservation, UnknownResolutionError,
};
use layerx_agentd::shutdown::{graceful, DaemonLifecycle, ShutdownError};
use layerx_agentd::store::{ObjectKind, Store};

use serde_json::{json, Value};
use support::{directory, rpc_hex, sequencer_receipt, serve, signed_transaction, tenant};

fn protocol(consumed: u128) -> ProtocolBudgetState {
    ProtocolBudgetState {
        evidence: support::raw_state_leaf(consumed.to_be_bytes().to_vec(), 50),
    }
}

fn recovery_inputs<'a>(
    unknown_budget_ids: &'a [[u8; 32]],
    budget_receipts: &'a [PersistedReceipt],
    unknown_ceilings: &'a [UnknownCeilingReservation],
) -> RecoveryInputs<'a> {
    RecoveryInputs {
        verifier: support::pinned_evidence_verifier(),
        unknown_budget_ids,
        budget_receipts,
        protocol_budget: protocol(0),
        ceiling_maximum: 1_000,
        ceiling_receipts: &[] as &[CeilingReceipt],
        unknown_ceiling_reservations: unknown_ceilings,
        current_sequence: 1,
    }
}

fn enqueue(outbox: &mut Outbox, store: &mut Store, id: u8) -> ([u8; 32], Vec<u8>) {
    let exact = signed_transaction(id);
    let hash = outbox
        .enqueue(store, tenant(), exact.clone())
        .unwrap_or_else(|error| panic!("enqueue {id}: {error:?}"));
    (hash, exact)
}

fn transition(outbox: &mut Outbox, store: &mut Store, hash: [u8; 32], to: SubmissionState) {
    let result = if to == SubmissionState::Executed {
        let activity_id = outbox
            .status(hash)
            .map_or_else(|| panic!("activity missing"), |status| status.activity_id);
        let evidence = support::pinned_evidence_verifier()
            .verify_receipt(&support::raw_receipt(activity_id, 0, 25))
            .unwrap_or_else(|error| panic!("receipt: {error:?}"));
        outbox.record_execution(store, hash, evidence, 9, 0)
    } else {
        outbox.transition(store, hash, to, format!("advance to {to:?}"), None)
    };
    result.unwrap_or_else(|error| panic!("transition {hash:?}: {error:?}"));
}

fn populate_every_stage(store: &mut Store) -> (Outbox, Vec<u8>, Vec<[u8; 32]>) {
    let mut outbox = Outbox::default();
    let (queued_id, queued_exact) = enqueue(&mut outbox, store, 1);
    let mut ids = vec![queued_id];
    for id in 2..=5 {
        let (hash, _) = enqueue(&mut outbox, store, id);
        ids.push(hash);
        transition(&mut outbox, store, hash, SubmissionState::Submitted);
    }
    transition(&mut outbox, store, ids[2], SubmissionState::Submitted);
    transition(&mut outbox, store, ids[3], SubmissionState::Unknown);
    transition(&mut outbox, store, ids[4], SubmissionState::Submitted);
    transition(&mut outbox, store, ids[4], SubmissionState::Executed);
    (outbox, queued_exact, ids)
}

#[test]
fn restart_preserves_recovery_actions_but_blocks_writes_without_budget_schema() {
    let root = directory("stages");
    let mut before = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let (before_outbox, queued_exact, ids) = populate_every_stage(&mut before);
    let unknown_activity_id = before_outbox.status(ids[3]).map_or_else(
        || panic!("unknown activity missing"),
        |status| status.activity_id,
    );
    hold_unknown(
        &mut before,
        &UnknownReservation {
            tenant: tenant(),
            id: ids[3],
            amount: 300,
            expiry_sequence: 100,
            resolved: None,
        },
    )
    .unwrap_or_else(|error| panic!("budget hold: {error:?}"));
    drop(before);

    let mut after = Store::open(&root).unwrap_or_else(|error| panic!("restart: {error}"));
    assert!(matches!(
        recover(
            &mut after,
            &tenant(),
            &recovery_inputs(
                &[ids[3]],
                &[],
                &[UnknownCeilingReservation {
                    id: ids[3],
                    expected_activity_id: unknown_activity_id,
                    amount: 300,
                    expiry_sequence: 100,
                }],
            ),
        ),
        Err(RecoveryError::Ceiling(CeilingError::Unreconciled))
    ));
    let recovered = recover(&mut after, &tenant(), &recovery_inputs(&[ids[3]], &[], &[]))
        .unwrap_or_else(|error| panic!("recover: {error:?}"));
    assert_eq!(recovered.queued_without_authority, vec![ids[0]]);
    assert!(recovered.queued_for_transmission.is_empty());
    let mut awaiting = vec![ids[1], ids[2], ids[3]];
    awaiting.sort_unstable();
    assert_eq!(recovered.awaiting_receipt_resolution, awaiting);
    assert!(!recovered.queued_for_transmission.contains(&ids[4]));
    assert!(!recovered.awaiting_receipt_resolution.contains(&ids[4]));
    assert_eq!(
        recovered
            .outbox
            .bytes_for_transmission(ids[0])
            .unwrap_or_else(|error| panic!("queued bytes: {error:?}")),
        queued_exact
    );
    assert_eq!(recovered.budget_accounting.held_unresolved, 300);
    assert_eq!(
        recovered
            .ceiling
            .snapshot()
            .map(|value| (value.held, value.reconciled)),
        Ok((0, false))
    );
    assert!(matches!(
        recovered.require_write_ready(),
        Err(RecoveryError::WritesBlocked)
    ));
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn restart_resolution_uses_receipts_without_duplicate_delivery() {
    let root = directory("exactly-once");
    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let (_, _, ids) = populate_every_stage(&mut store);
    drop(store);
    let mut reopened = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
    let mut recovered = recover(&mut reopened, &tenant(), &recovery_inputs(&[], &[], &[]))
        .unwrap_or_else(|error| panic!("recover: {error:?}"));
    let pending = recovered.awaiting_receipt_resolution.clone();
    let mut exchanges = Vec::new();
    let mut keys = Vec::new();
    for submission_id in &pending {
        let exact = recovered
            .outbox
            .exact_signed_bytes(*submission_id)
            .unwrap_or_else(|error| panic!("exact bytes: {error:?}"));
        let (receipt, key) = sequencer_receipt(exact, true);
        keys.push(key);
        exchanges.push((
            "pax_sequencerReceipt",
            json!([rpc_hex(submission_id)]),
            Some(receipt),
        ));
        exchanges.push((
            "eth_getTransactionReceipt",
            json!([rpc_hex(submission_id)]),
            Some(Value::Null),
        ));
    }
    keys.dedup();
    let (mut endpoint, server) = serve(exchanges);
    for (attempt, submission_id) in pending.into_iter().enumerate() {
        resolve_unknown(
            &mut recovered.outbox,
            &mut reopened,
            submission_id,
            10_000 + u64::try_from(attempt).unwrap_or(0),
            &keys,
            &mut endpoint,
        )
        .unwrap_or_else(|error| panic!("resolve {submission_id:?}: {error:?}"));
    }
    let core = server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    assert_eq!(core.lookups, 3);
    assert_eq!(core.resends, 0);
    assert_eq!(
        recovered.outbox.status(ids[4]).map(|status| status.state),
        Some(SubmissionState::Executed)
    );
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn unreconciled_accounting_blocks_writes_after_outbox_recovery() {
    let root = directory("blocked");
    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let _ = populate_every_stage(&mut store);
    let inputs = RecoveryInputs {
        verifier: support::pinned_evidence_verifier(),
        unknown_budget_ids: &[],
        budget_receipts: &[],
        protocol_budget: protocol(100),
        ceiling_maximum: 1_000,
        ceiling_receipts: &[],
        unknown_ceiling_reservations: &[],
        current_sequence: 1,
    };
    let recovered = recover(&mut store, &tenant(), &inputs)
        .unwrap_or_else(|error| panic!("recover: {error:?}"));
    assert!(matches!(
        recovered.require_write_ready(),
        Err(RecoveryError::WritesBlocked)
    ));
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn graceful_shutdown_stops_admission_and_flushes_inflight_and_audit() {
    let root = directory("shutdown");
    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let mut outbox = Outbox::default();
    let (hash, _) = enqueue(&mut outbox, &mut store, 1);
    let mut lifecycle = DaemonLifecycle::running();
    lifecycle
        .begin_work(hash, b"queued-before-exit".to_vec())
        .unwrap_or_else(|error| panic!("begin: {error:?}"));
    lifecycle
        .append_audit(b"submission queued".to_vec())
        .unwrap_or_else(|error| panic!("audit: {error:?}"));
    let report = graceful(&mut store, tenant(), &outbox, &mut lifecycle)
        .unwrap_or_else(|error| panic!("shutdown: {error:?}"));
    assert_eq!(report.in_flight_recorded, 1);
    assert_eq!(report.audit_entries_flushed, 1);
    assert!(!report.accepting_work);
    assert!(!lifecycle.accepting_work());
    assert!(matches!(
        lifecycle.begin_work([2; 32], Vec::new()),
        Err(ShutdownError::NotAccepting)
    ));
    drop(store);
    let reopened = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
    assert_eq!(
        reopened.list_object_ids(&tenant(), ObjectKind::Audit).len(),
        1
    );
    assert!(reopened
        .list_object_ids(&tenant(), ObjectKind::Configuration)
        .iter()
        .any(|identifier| identifier == b"shutdown-complete"));
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn clock_regression_during_restarted_resolution_fails_closed() {
    let root = directory("clock");
    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let mut outbox = Outbox::default();
    let (hash, _) = enqueue(&mut outbox, &mut store, 1);
    transition(&mut outbox, &mut store, hash, SubmissionState::Submitted);
    transition(&mut outbox, &mut store, hash, SubmissionState::Unknown);
    let (mut endpoint, server) = serve(vec![(
        "pax_sequencerReceipt",
        json!([rpc_hex(&hash)]),
        None,
    )]);
    let _ = resolve_unknown(&mut outbox, &mut store, hash, 10_000, &[], &mut endpoint)
        .unwrap_or_else(|error| panic!("initial resolution: {error:?}"));
    server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    drop(outbox);
    drop(store);
    let mut reopened = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
    let mut recovered = recover(&mut reopened, &tenant(), &recovery_inputs(&[], &[], &[]))
        .unwrap_or_else(|error| panic!("recover: {error:?}"));
    assert!(matches!(
        resolve_unknown(
            &mut recovered.outbox,
            &mut reopened,
            hash,
            9_999,
            &[],
            &mut endpoint
        ),
        Err(UnknownResolutionError::TimeRegressed)
    ));
    assert_eq!(
        recovered.outbox.status(hash).map(|status| status.state),
        Some(SubmissionState::Unknown)
    );
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn queued_record_without_origin_is_restored_but_never_offered_for_transmission() {
    let root = directory("queued-without-origin");
    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let mut outbox = Outbox::default();
    let (hash, exact) = enqueue(&mut outbox, &mut store, 1);
    drop(store);
    let mut reopened = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
    let recovered = recover(&mut reopened, &tenant(), &recovery_inputs(&[], &[], &[]))
        .unwrap_or_else(|error| panic!("recover: {error:?}"));
    assert_eq!(recovered.queued_without_authority, vec![hash]);
    assert!(recovered.queued_for_transmission.is_empty());
    assert!(recovered.awaiting_receipt_resolution.is_empty());
    assert_eq!(
        recovered.outbox.status(hash).map(|status| status.state),
        Some(SubmissionState::Queued)
    );
    assert_eq!(
        recovered
            .outbox
            .exact_signed_bytes(hash)
            .unwrap_or_else(|error| panic!("exact bytes: {error:?}")),
        exact.as_slice()
    );
    let _ = std::fs::remove_dir_all(root);
}
