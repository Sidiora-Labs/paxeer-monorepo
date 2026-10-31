mod support;

use layerx_agentd::budget::{
    reserve, BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest,
};
use layerx_agentd::capability::{consume, Ceiling, CeilingError};
use layerx_agentd::outbox::{
    resolve_unknown, Outbox, ResendObservation, ResolutionObservation, SubmissionState,
};
use layerx_agentd::store::Store;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
use serde_json::{json, Value};
use support::{directory, rpc_hex, sequencer_receipt, serve, tenant};

fn unknown_outbox(root: &std::path::Path, id: u8) -> (Store, Outbox, Vec<u8>, [u8; 32]) {
    let exact = support::signed_transaction(id);
    let mut store = Store::open(root).unwrap_or_else(|error| panic!("store: {error}"));
    let mut outbox = Outbox::default();
    let hash = outbox
        .enqueue(&mut store, tenant(), exact.clone())
        .unwrap_or_else(|error| panic!("enqueue: {error:?}"));
    outbox
        .transition(
            &mut store,
            hash,
            SubmissionState::Submitted,
            "transport started",
            None,
        )
        .unwrap_or_else(|error| panic!("submit: {error:?}"));
    outbox
        .transition(
            &mut store,
            hash,
            SubmissionState::Unknown,
            "transport outcome indeterminate",
            None,
        )
        .unwrap_or_else(|error| panic!("unknown: {error:?}"));
    (store, outbox, exact, hash)
}

#[test]
fn acknowledgement_loss_is_resolved_only_by_the_existing_receipt() {
    let root = directory("ack-loss");
    let (mut store, mut outbox, exact, hash) = unknown_outbox(&root, 1);
    let (receipt, key) = sequencer_receipt(&exact, true);
    let (mut endpoint, server) = serve(vec![
        (
            "pax_sequencerReceipt",
            json!([rpc_hex(&hash)]),
            Some(receipt),
        ),
        (
            "eth_getTransactionReceipt",
            json!([rpc_hex(&hash)]),
            Some(Value::Null),
        ),
    ]);

    let result = resolve_unknown(&mut outbox, &mut store, hash, 10_000, &[key], &mut endpoint)
        .unwrap_or_else(|error| panic!("resolve: {error:?}"));
    let node = server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    assert_eq!(result.state, SubmissionState::Executed);
    assert_eq!(result.observation, ResolutionObservation::ExecutedReceipt);
    assert_eq!(result.resend, ResendObservation::NotWarranted);
    assert!(node.transmitted.is_empty());
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn lost_resend_response_keeps_budget_held_while_unreconciled_ceiling_refuses_admission() {
    let root = directory("response-loss");
    let (mut store, mut outbox, exact, hash) = unknown_outbox(&root, 2);
    let limit_id = LimitId([7; 16]);
    let limiter = BudgetLimiter::new(vec![LimitConfig {
        id: limit_id,
        name: "tenant spend".to_owned(),
        scope: LimitScope::Tenant([8; 32]),
        ceiling: 1_000,
        consumed: 0,
    }])
    .unwrap_or_else(|error| panic!("limiter: {error:?}"));
    reserve(
        &limiter,
        &ReservationRequest {
            id: [2; 32],
            amount: 400,
            expiry_sequence: 5,
            current_sequence: 1,
            applicable_limits: vec![limit_id],
        },
    )
    .unwrap_or_else(|error| panic!("budget reserve: {error:?}"));
    let ceiling = Ceiling::new(1_000, support::pinned_evidence_verifier());
    assert_eq!(
        consume(&ceiling, [2; 32], [2; 32], 400, 5, 1),
        Err(CeilingError::Unreconciled)
    );

    let (late_receipt, key) = sequencer_receipt(&exact, true);
    let (transaction, signature) =
        decode_signed(&exact).unwrap_or_else(|error| panic!("transaction: {error:?}"));
    let sender = recover_sender(&transaction, &signature)
        .unwrap_or_else(|error| panic!("sender: {error:?}"));
    let (mut endpoint, server) = serve(vec![
        (
            "pax_sequencerReceipt",
            json!([rpc_hex(&hash)]),
            Some(Value::Null),
        ),
        (
            "eth_getTransactionCount",
            json!([rpc_hex(&sender), "pending"]),
            Some(json!(format!("{:#x}", transaction.nonce))),
        ),
        ("eth_sendRawTransaction", json!([rpc_hex(&exact)]), None),
    ]);
    let first = resolve_unknown(&mut outbox, &mut store, hash, 20_000, &[key], &mut endpoint)
        .unwrap_or_else(|error| panic!("first resolve: {error:?}"));
    let node = server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    assert_eq!(first.state, SubmissionState::Unknown);
    assert_eq!(first.observation, ResolutionObservation::ReceiptMissing);
    assert_eq!(first.resend, ResendObservation::Indeterminate);
    assert_eq!(node.transmitted, vec![(hash, exact)]);
    assert_eq!(limiter.held_reservations(), Ok(1));
    assert_eq!(ceiling.release_expired(50), Ok(0));
    assert_eq!(
        ceiling
            .snapshot()
            .map(|value| (value.held, value.reconciled)),
        Ok((0, false))
    );

    let (mut endpoint, server) = serve(vec![
        (
            "pax_sequencerReceipt",
            json!([rpc_hex(&hash)]),
            Some(late_receipt),
        ),
        (
            "eth_getTransactionReceipt",
            json!([rpc_hex(&hash)]),
            Some(Value::Null),
        ),
    ]);
    let second = resolve_unknown(
        &mut outbox,
        &mut store,
        hash,
        first.age.next_attempt_at_ms,
        &[key],
        &mut endpoint,
    )
    .unwrap_or_else(|error| panic!("second resolve: {error:?}"));
    server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    assert_eq!(second.state, SubmissionState::Executed);
    assert_eq!(second.age.attempt_count, 2);
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn restart_preserves_backoff_age_and_a_receipt_can_appear_minutes_later() {
    let root = directory("restart");
    let (mut store, mut outbox, exact, hash) = unknown_outbox(&root, 3);
    let (receipt, key) = sequencer_receipt(&exact, false);
    let (transaction, signature) =
        decode_signed(&exact).unwrap_or_else(|error| panic!("transaction: {error:?}"));
    let sender = recover_sender(&transaction, &signature)
        .unwrap_or_else(|error| panic!("sender: {error:?}"));
    let (mut endpoint, server) = serve(vec![
        (
            "pax_sequencerReceipt",
            json!([rpc_hex(&hash)]),
            Some(Value::Null),
        ),
        (
            "eth_getTransactionCount",
            json!([rpc_hex(&sender), "pending"]),
            Some(json!(format!("{:#x}", transaction.nonce))),
        ),
        (
            "eth_sendRawTransaction",
            json!([rpc_hex(&exact)]),
            Some(json!(rpc_hex(&hash))),
        ),
    ]);
    let first = resolve_unknown(&mut outbox, &mut store, hash, 30_000, &[key], &mut endpoint)
        .unwrap_or_else(|error| panic!("first resolve: {error:?}"));
    let node = server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    assert_eq!(first.age.attempt_count, 1);
    assert_eq!(node.transmitted, vec![(hash, exact)]);
    drop(outbox);
    drop(store);

    let mut reopened = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
    let mut restored = Outbox::default();
    restored
        .restore(&reopened, tenant(), hash)
        .unwrap_or_else(|error| panic!("restore: {error:?}"));
    let deferred = resolve_unknown(
        &mut restored,
        &mut reopened,
        hash,
        first.age.next_attempt_at_ms - 1,
        &[key],
        &mut endpoint,
    )
    .unwrap_or_else(|error| panic!("deferred: {error:?}"));
    assert_eq!(deferred.observation, ResolutionObservation::Backoff);
    assert_eq!(deferred.age.attempt_count, 1);

    let (mut endpoint, server) = serve(vec![
        (
            "pax_sequencerReceipt",
            json!([rpc_hex(&hash)]),
            Some(receipt),
        ),
        (
            "eth_getTransactionReceipt",
            json!([rpc_hex(&hash)]),
            Some(Value::Null),
        ),
    ]);
    let later = resolve_unknown(
        &mut restored,
        &mut reopened,
        hash,
        210_000,
        &[key],
        &mut endpoint,
    )
    .unwrap_or_else(|error| panic!("later: {error:?}"));
    server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    assert_eq!(later.state, SubmissionState::Failed);
    assert_eq!(later.age.age_ms, 180_000);
    assert_eq!(later.age.attempt_count, 2);
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn lookup_failure_and_unverified_receipt_never_infer_a_terminal_outcome() {
    let root = directory("lookup-loss");
    let (mut store, mut outbox, exact, hash) = unknown_outbox(&root, 4);
    let (mut raw, key) = sequencer_receipt(&exact, true);
    let (mut endpoint, server) = serve(vec![(
        "pax_sequencerReceipt",
        json!([rpc_hex(&hash)]),
        None,
    )]);
    let first = resolve_unknown(&mut outbox, &mut store, hash, 40_000, &[key], &mut endpoint)
        .unwrap_or_else(|error| panic!("lookup loss: {error:?}"));
    assert_eq!(first.state, SubmissionState::Unknown);
    let node = server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    assert_eq!(first.observation, ResolutionObservation::LookupUnavailable);
    assert!(node.transmitted.is_empty());

    let encoded = raw["receipt"]
        .as_str()
        .unwrap_or_else(|| panic!("receipt required"));
    let mut corrupt = hex::decode(encoded.strip_prefix("0x").unwrap_or(encoded))
        .unwrap_or_else(|error| panic!("receipt: {error:?}"));
    corrupt[0] ^= 1;
    raw["receipt"] = json!(rpc_hex(&corrupt));
    let (mut endpoint, server) = serve(vec![(
        "pax_sequencerReceipt",
        json!([rpc_hex(&hash)]),
        Some(raw),
    )]);
    let second = resolve_unknown(
        &mut outbox,
        &mut store,
        hash,
        first.age.next_attempt_at_ms,
        &[key],
        &mut endpoint,
    )
    .unwrap_or_else(|error| panic!("unverified: {error:?}"));
    assert_eq!(second.state, SubmissionState::Unknown);
    assert_eq!(second.observation, ResolutionObservation::UnverifiedReceipt);
    server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    let _ = std::fs::remove_dir_all(root);
}
