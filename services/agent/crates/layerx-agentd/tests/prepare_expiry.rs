#[path = "support/send_authorization.rs"]
mod send_authorization;

use layerx_agentd::budget::{
    reserve, BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest,
};
use layerx_agentd::prepare::evm::{
    self, EvmPreparationState, PrecompileCall, PreparationDefaults, PrepareRequest,
};
use layerx_agentd::prepare::{
    expire, retention_sweep, LifecycleError, LifecycleState, PayloadRedaction, PreparationLifecycle,
};
use layerx_wire::encode::Encoder;

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("expiry: {error:?}"))
}

fn send_payload() -> Vec<u8> {
    let mut encoder = Encoder::new(512);
    encoder
        .u16(0x5301)
        .unwrap_or_else(|error| panic!("tag: {error:?}"));
    encoder
        .u16(10)
        .unwrap_or_else(|error| panic!("fields: {error:?}"));
    encoder
        .fixed(&[0x11; 32])
        .unwrap_or_else(|error| panic!("from: {error:?}"));
    encoder
        .fixed(&[0x22; 32])
        .unwrap_or_else(|error| panic!("to: {error:?}"));
    encoder
        .fixed(&[0x33; 32])
        .unwrap_or_else(|error| panic!("asset: {error:?}"));
    encoder
        .u128(25)
        .unwrap_or_else(|error| panic!("amount: {error:?}"));
    encoder
        .u64(5)
        .unwrap_or_else(|error| panic!("sequence: {error:?}"));
    encoder
        .fixed(&[4; 32])
        .unwrap_or_else(|error| panic!("idempotency: {error:?}"));
    encoder
        .u64(1_010)
        .unwrap_or_else(|error| panic!("expiry: {error:?}"));
    encoder
        .fixed(&[0x55; 32])
        .unwrap_or_else(|error| panic!("context: {error:?}"));
    encoder
        .u8(0)
        .unwrap_or_else(|error| panic!("conditions: {error:?}"));
    encoder
        .u8(1)
        .unwrap_or_else(|error| panic!("authority kind: {error:?}"));
    encoder
        .fixed(&[0x11; 32])
        .unwrap_or_else(|error| panic!("controller: {error:?}"));
    encoder
        .fixed(&[0x66; 32])
        .unwrap_or_else(|error| panic!("payload key: {error:?}"));
    encoder
        .fixed(&[0x77; 64])
        .unwrap_or_else(|error| panic!("payload signature: {error:?}"));
    encoder
        .fixed(&[0x55; 32])
        .unwrap_or_else(|error| panic!("signed context: {error:?}"));
    encoder
        .u32(17)
        .unwrap_or_else(|error| panic!("network: {error:?}"));
    encoder
        .u16(layerx_wire::limits::PROTOCOL_VERSION)
        .unwrap_or_else(|error| panic!("version: {error:?}"));
    send_authorization::sign(encoder.finish())
}

fn evm_prepared() -> evm::Prepared {
    let prepared = must(evm::prepare_bound(
        EvmPreparationState {
            chain_id: 17,
            pending_nonce: 5,
            base_fee_per_gas: 1,
            head_height: 88,
        },
        PreparationDefaults {
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 7,
            maximum_payload_bytes: 1024,
        },
        PrepareRequest {
            from: [0x11; 20],
            call: PrecompileCall::ledger_transfer(send_payload()),
            expected_nonce: Some(5),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 1024,
        },
        100_000,
    ));
    assert_eq!(evm::verify_call_binding(&prepared), Ok(()));
    assert_eq!(
        prepared.disclosure,
        must(layerx_crypto::disclosure::call::bind_call(
            &prepared.call,
            prepared.from
        ))
    );
    assert_eq!(prepared.signing_hash, prepared.call.signing_hash());
    assert_eq!(prepared.disclosure.idempotency_key, [4; 32]);
    assert_eq!(prepared.disclosure.expiry.payload_expires_at, 1010);
    prepared
}

fn prepared() -> evm::Prepared {
    evm_prepared()
}

fn limiter() -> BudgetLimiter {
    BudgetLimiter::new(vec![LimitConfig {
        id: LimitId([1; 16]),
        name: "tenant-limit".to_owned(),
        scope: LimitScope::Tenant([1; 32]),
        ceiling: 1_000,
        consumed: 0,
    }])
    .unwrap_or_else(|error| panic!("limiter: {error:?}"))
}

fn reserve_one(limiter: &BudgetLimiter, id: [u8; 32]) {
    reserve(
        limiter,
        &ReservationRequest {
            id,
            amount: 100,
            expiry_sequence: 1_010,
            current_sequence: 1_000,
            applicable_limits: vec![LimitId([1; 16])],
        },
    )
    .unwrap_or_else(|error| panic!("reserve: {error:?}"));
}

#[test]
fn expiry_during_and_after_signing_releases_every_reservation() {
    let prepared = prepared();
    let limiter = limiter();
    reserve_one(&limiter, [1; 32]);
    reserve_one(&limiter, [2; 32]);
    let lifecycle = PreparationLifecycle::default();
    lifecycle
        .register([11; 32], &prepared, vec![[1; 32]])
        .unwrap_or_else(|error| panic!("register: {error:?}"));
    lifecycle
        .transition([11; 32], LifecycleState::Signing, 1_001)
        .unwrap_or_else(|error| panic!("signing: {error:?}"));
    lifecycle
        .register([12; 32], &prepared, vec![[2; 32]])
        .unwrap_or_else(|error| panic!("register: {error:?}"));
    lifecycle
        .transition([12; 32], LifecycleState::Signing, 1_001)
        .unwrap_or_else(|error| panic!("signing: {error:?}"));
    lifecycle
        .retain_signed_bytes([12; 32], vec![9; 64], [0x44; 32])
        .unwrap_or_else(|error| panic!("signed: {error:?}"));

    let report = expire(&lifecycle, &limiter, 1_011, 1_011)
        .unwrap_or_else(|error| panic!("expire: {error:?}"));
    assert_eq!(report.expired_preparations, vec![[11; 32], [12; 32]]);
    assert_eq!(report.released_reservations, vec![[1; 32], [2; 32]]);
    assert_eq!(limiter.held_reservations(), Ok(0));
    assert_eq!(lifecycle.state([11; 32]), Ok(LifecycleState::Expired));
    assert_eq!(
        lifecycle.admit_submission([12; 32], 1_011),
        Err(LifecycleError::PreparationExpired)
    );
}

#[test]
fn retention_discards_terminal_bytes_but_preserves_unknown_and_redacts_payload() {
    let prepared = prepared();
    let lifecycle = PreparationLifecycle::default();
    for id in [21_u8, 22] {
        lifecycle
            .register([id; 32], &prepared, Vec::new())
            .unwrap_or_else(|error| panic!("register: {error:?}"));
        lifecycle
            .transition([id; 32], LifecycleState::Signing, 90)
            .unwrap_or_else(|error| panic!("signing: {error:?}"));
        lifecycle
            .retain_signed_bytes([id; 32], vec![id; 64], [id; 32])
            .unwrap_or_else(|error| panic!("signed: {error:?}"));
        lifecycle
            .transition([id; 32], LifecycleState::Submitted, 91)
            .unwrap_or_else(|error| panic!("submitted: {error:?}"));
    }
    lifecycle
        .transition([21; 32], LifecycleState::Unknown, 92)
        .unwrap_or_else(|error| panic!("unknown: {error:?}"));
    lifecycle
        .transition([22; 32], LifecycleState::Failed, 100)
        .unwrap_or_else(|error| panic!("failed: {error:?}"));

    let report =
        retention_sweep(&lifecycle, 110, 5).unwrap_or_else(|error| panic!("sweep: {error:?}"));
    assert_eq!(report.discarded_terminal_signed_bytes, 1);
    assert_eq!(report.preserved_unresolved_signed_bytes, 1);
    assert_eq!(lifecycle.has_signed_bytes([21; 32]), Ok(true));
    assert_eq!(lifecycle.has_signed_bytes([22; 32]), Ok(false));
    let log = lifecycle
        .redacted_log([21; 32], PayloadRedaction::DigestOnly)
        .unwrap_or_else(|error| panic!("log: {error:?}"));
    assert!(log.contains("activity_id="));
    assert!(log.contains("payload=[redacted]"));
    assert!(log.contains("payload_hash="));
    assert!(!log.contains("did:layerx"));
}

#[test]
fn expiry_equality_sequence_and_binding_refusals_preserve_preparation() {
    let prepared = prepared();
    let lifecycle = PreparationLifecycle::default();
    let limiter = limiter();
    reserve_one(&limiter, [3; 32]);
    must(lifecycle.register([13; 32], &prepared, vec![[3; 32]]));
    assert_eq!(lifecycle.check_unexpired([13; 32], 1010), Ok(()));
    let report = must(expire(&lifecycle, &limiter, 1010, 1010));
    assert!(report.expired_preparations.is_empty());
    assert_eq!(limiter.held_reservations(), Ok(1));
    assert_eq!(lifecycle.state([13; 32]), Ok(LifecycleState::Prepared));
    let original = evm_prepared();
    let mut changed = original.clone();
    changed.call.nonce += 1;
    assert!(evm::verify_call_binding(&changed).is_err());
    assert_eq!(original, evm_prepared());
    assert_eq!(lifecycle.state([13; 32]), Ok(LifecycleState::Prepared));
    let report = must(expire(&lifecycle, &limiter, 1011, 1011));
    assert_eq!(report.expired_preparations, vec![[13; 32]]);
    assert_eq!(report.released_reservations, vec![[3; 32]]);
    assert_eq!(limiter.held_reservations(), Ok(0));
}
