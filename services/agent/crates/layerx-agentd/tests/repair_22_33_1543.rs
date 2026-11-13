//! Finding-to-test mapping and deferred domain wiring: `prepare/repair_22_33_1543.kvx`.

fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use layerx_agentd::budget::{reserve, restore, LimitId, ReservationRequest};
use layerx_agentd::prepare::{
    disclose, prepare_bound, sweep_retention, verify_call_binding, CallBindingError,
    DurablePreparation, EvmPrepareError, LifecycleError, LifecycleState, PreparationLifecycle,
    EXTENSION_CALL,
};
use layerx_agentd::session::{SessionId, SessionRef};
use layerx_agentd::store::TenantId;
use layerx_agentd::test_support::preparation::*;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};

#[test]
fn f14_call_disclosure_refuses_calldata_actor_nonce_fees_and_digest_mutations() {
    let original = prepared(14);
    let decoded = require_ok(
        disclose::restore_call(
            &original.call.encode_unsigned(),
            OWNER,
            original.observed_head_height,
            original.disclosure_digest,
            original.signing_hash,
        ),
        "required test value",
    );
    assert_eq!(decoded, original);
    assert_eq!(
        require_ok(disclose::bind(&original.call, OWNER), "required test value").disclosure,
        original.disclosure
    );
    let raw = signed(&decoded);
    let (call, signature) = require_ok(decode_signed(&raw), "required test value");
    assert_eq!(call, original.call);
    assert_eq!(
        require_ok(recover_sender(&call, &signature), "required test value"),
        OWNER
    );
    for field in 0..9 {
        let mut changed = decoded.clone();
        match field {
            0 => changed.call.data[100] ^= 1,
            1 => changed.from[0] ^= 1,
            2 => changed.call.nonce += 1,
            3 => changed.call.max_priority_fee_per_gas += 1,
            4 => changed.call.max_fee_per_gas += 1,
            5 => changed.disclosure_digest[0] ^= 1,
            6 => changed.disclosure.actor[0] ^= 1,
            7 => changed.call.gas_limit += 1,
            8 => changed.signing_hash[0] ^= 1,
            _ => unreachable!(),
        }
        assert!(verify_call_binding(&changed).is_err(), "mutation {field}");
        if matches!(field, 0 | 2 | 3 | 4 | 7) {
            changed.signing_hash = changed.call.signing_hash();
            assert!(
                verify_call_binding(&changed).is_err(),
                "stale disclosure {field}"
            );
        }
    }
    let mut digest_changed = original.clone();
    digest_changed.disclosure_digest[0] ^= 1;
    assert_eq!(
        verify_call_binding(&digest_changed),
        Err(CallBindingError::DigestMismatch)
    );
    assert!(disclose::restore_call(
        &raw,
        OWNER,
        80,
        original.disclosure_digest,
        original.signing_hash
    )
    .is_err());
    let mut trailing = original.call.encode_unsigned();
    trailing.push(0);
    assert!(disclose::restore_call(
        &trailing,
        OWNER,
        80,
        original.disclosure_digest,
        original.signing_hash
    )
    .is_err());
}

#[test]
fn f13_eip1559_validation_and_explicit_durable_version_handling() {
    let produced = prepared(13);
    assert_eq!(produced.call.encode_unsigned()[0], 2);
    assert_eq!(produced.call.max_fee_per_gas, 5);
    assert_eq!(produced.signing_hash, produced.disclosure_digest);
    require_ok(verify_call_binding(&produced), "required test value");
    let mut stale = request(13);
    stale.expected_nonce = Some(8);
    assert_eq!(
        prepare_bound(state(), defaults(), stale, 100_000),
        Err(EvmPrepareError::StaleNonce {
            expected: 8,
            pending: 7
        })
    );
    let mut oversized = request(13);
    oversized.declared_payload_limit = 195;
    assert_eq!(
        prepare_bound(state(), defaults(), oversized, 100_000),
        Err(EvmPrepareError::PayloadLimitExceeded {
            actual: 196,
            maximum: 195
        })
    );
    let mut insufficient_fee = request(13);
    insufficient_fee.max_fee_per_gas = Some(4);
    assert_eq!(
        prepare_bound(state(), defaults(), insufficient_fee, 100_000),
        Err(EvmPrepareError::InvalidDefaults)
    );
    assert_eq!(
        prepare_bound(state(), defaults(), request(13), 0),
        Err(EvmPrepareError::InvalidDefaults)
    );
    let mut wrong_chain = state();
    wrong_chain.chain_id = 0;
    assert_eq!(
        prepare_bound(wrong_chain, defaults(), request(13), 100_000),
        Err(EvmPrepareError::InvalidDefaults)
    );
    let owner = session();
    let record = require_ok(
        DurablePreparation::from_call(&owner, GENERATION, [13; 32], &produced, vec![]),
        "required test value",
    );
    let bytes = require_ok(record.encode(), "required test value");
    assert_eq!(bytes[4], 3, "call records use an explicit format version");
    let decoded = require_ok(
        DurablePreparation::decode(owner.tenant.clone(), &bytes),
        "required test value",
    );
    assert_eq!(
        require_ok(
            decoded.restore_call(&owner, GENERATION),
            "required test value"
        ),
        produced
    );
    assert_eq!(require_ok(decoded.encode(), "required test value"), bytes);
    let mut future = bytes.clone();
    future[4] = 4;
    assert_eq!(
        DurablePreparation::decode(owner.tenant.clone(), &future),
        Err(LifecycleError::InvalidSignedBytes)
    );
    let mut unversioned_call = bytes;
    unversioned_call[4] = 1;
    assert_eq!(
        DurablePreparation::decode(owner.tenant.clone(), &unversioned_call),
        Err(LifecycleError::InvalidSignedBytes)
    );
    let mut old = record;
    old.extensions.clear();
    let old_bytes = require_ok(old.encode(), "required test value");
    assert_eq!(old_bytes[4], 1);
    let old = require_ok(
        DurablePreparation::decode(owner.tenant.clone(), &old_bytes),
        "required test value",
    );
    assert_eq!(require_ok(old.encode(), "required test value"), old_bytes);
    assert_eq!(
        old.restore_call(&owner, GENERATION),
        Err(LifecycleError::MigrationRequired)
    );
}

#[test]
fn f15_call_expiry_releases_reservations_and_reopens_unresolved_signed_bytes() {
    let directory = Directory::new();
    let owner = session();
    let mut store = directory.open();
    let lifecycle = PreparationLifecycle::default();
    let budget = limiter();
    for identity in [11, 12, 13] {
        let (id, prepared) = verify_f15_call_expiry_releases_reservations_and_reopens_unresolved_signed_bytes_1_repair_22_33_1543(&owner, &mut store, &lifecycle, &budget, identity);
        if identity != 11 {
            let mut wrong_call = prepared.clone();
            wrong_call.call.nonce += 1;
            assert_eq!(
                lifecycle.retain_signed_call(
                    &mut store,
                    &owner,
                    GENERATION,
                    id,
                    signed(&wrong_call)
                ),
                Err(LifecycleError::InvalidSignedBytes)
            );
            let wrong_signer = require_ok(prepared.call.sign(&[2; 32]), "required test value");
            assert_eq!(
                lifecycle.retain_signed_call(&mut store, &owner, GENERATION, id, wrong_signer),
                Err(LifecycleError::InvalidSignedBytes)
            );
            let raw = signed(&prepared);
            assert_eq!(
                require_ok(
                    lifecycle.retain_signed_call(&mut store, &owner, GENERATION, id, raw.clone()),
                    "required test value"
                ),
                transaction_hash(&raw)
            );
            if identity == 12 {
                assert_eq!(
                    require_ok(
                        lifecycle.admit_call_submission(&mut store, &owner, GENERATION, id, 900),
                        "required test value"
                    ),
                    raw
                );
                require_ok(
                    lifecycle.transition_call(
                        &mut store,
                        &owner,
                        GENERATION,
                        id,
                        LifecycleState::Unknown,
                        82,
                    ),
                    "required test value",
                );
            } else {
                assert_eq!(
                    lifecycle.admit_call_submission(&mut store, &owner, GENERATION, id, EXPIRY + 1),
                    Err(LifecycleError::PreparationExpired)
                );
            }
        }
        assert_eq!(
            lifecycle.transition(id, LifecycleState::Signing, 80),
            Err(LifecycleError::AuthorizationRequired)
        );
    }
    finish_f15_call_expiry_releases_reservations_and_reopens_unresolved_signed_bytes_2_repair_22_33_1543(&directory, &owner, store, lifecycle, budget);
}

fn verify_f15_call_expiry_releases_reservations_and_reopens_unresolved_signed_bytes_1_repair_22_33_1543(
    owner: &SessionRef,
    store: &mut layerx_agentd::store::Store,
    lifecycle: &PreparationLifecycle,
    budget: &layerx_agentd::budget::BudgetLimiter,
    identity: u8,
) -> ([u8; 32], layerx_agentd::prepare::Prepared) {
    let id = [identity; 32];
    let hold = require_ok(
        reserve(
            budget,
            &ReservationRequest {
                id,
                amount: 100,
                expiry_sequence: 2_000,
                current_sequence: 80,
                applicable_limits: vec![LimitId([1; 16])],
            },
        ),
        "required test value",
    );
    let prepared = prepared(identity);
    require_ok(
        lifecycle.register_call(
            store,
            owner,
            GENERATION,
            id,
            &prepared,
            hold.durable.into_iter().map(|hold| (hold, None)).collect(),
        ),
        "required test value",
    );
    require_ok(
        lifecycle.transition_call(store, owner, GENERATION, id, LifecycleState::Signing, 81),
        "required test value",
    );
    (id, prepared)
}

fn finish_f15_call_expiry_releases_reservations_and_reopens_unresolved_signed_bytes_2_repair_22_33_1543(
    directory: &Directory,
    owner: &SessionRef,
    store: layerx_agentd::store::Store,
    lifecycle: PreparationLifecycle,
    budget: layerx_agentd::budget::BudgetLimiter,
) {
    assert_eq!(budget.held_reservations(), Ok(3));
    drop(lifecycle);
    drop(budget);
    drop(store);
    let store = directory.open();
    let lifecycle = PreparationLifecycle::default();
    assert_eq!(
        require_ok(
            lifecycle.restore_calls(&store, &owner.tenant),
            "required test value"
        ),
        3
    );
    let budget = limiter();
    let durable = require_ok(
        DurablePreparation::load_all(&store, &owner.tenant),
        "required test value",
    );
    for record in &durable {
        require_ok(
            restore(
                &budget,
                &record
                    .holds
                    .iter()
                    .map(|(hold, _)| hold.clone())
                    .collect::<Vec<_>>(),
            ),
            "required test value",
        );
    }
    finish_f15_call_expiry_releases_reservations_and_reopens_unresolved_signed_bytes_1_repair_22_33_1543(directory, owner, store, lifecycle, &budget);
}

fn finish_f15_call_expiry_releases_reservations_and_reopens_unresolved_signed_bytes_1_repair_22_33_1543(
    directory: &Directory,
    owner: &SessionRef,
    mut store: layerx_agentd::store::Store,
    lifecycle: PreparationLifecycle,
    budget: &layerx_agentd::budget::BudgetLimiter,
) {
    assert_eq!(budget.held_reservations(), Ok(3));
    assert_eq!(lifecycle.state([13; 32]), Ok(LifecycleState::Signed));
    assert_eq!(
        require_ok(
            lifecycle.recovered_signed_call(&store, owner, GENERATION, [13; 32]),
            "required test value"
        ),
        signed(&prepared(13))
    );
    assert_eq!(
        require_ok(
            lifecycle.prepared_call(&store, owner, GENERATION, [12; 32]),
            "required test value"
        ),
        prepared(12)
    );
    assert_eq!(
        require_ok(
            lifecycle.recovered_signed_call(&store, owner, GENERATION, [12; 32]),
            "required test value"
        ),
        signed(&prepared(12))
    );
    let expired = require_ok(
        lifecycle.expire_calls(&mut store, &owner.tenant, budget, EXPIRY + 1, 200),
        "required test value",
    );
    assert_eq!(expired.expired_preparations, vec![[11; 32], [13; 32]]);
    assert_eq!(expired.released_reservations, vec![[11; 32], [13; 32]]);
    assert_eq!(budget.held_reservations(), Ok(1));
    assert_eq!(lifecycle.state([12; 32]), Ok(LifecycleState::Unknown));
    assert_eq!(lifecycle.state([13; 32]), Ok(LifecycleState::Expired));
    assert_eq!(
        lifecycle.admit_call_submission(&mut store, owner, GENERATION, [13; 32], EXPIRY + 1),
        Err(LifecycleError::PreparationExpired)
    );
    drop(store);
    drop(lifecycle);
    let mut store = directory.open();
    let recovered = PreparationLifecycle::default();
    assert_eq!(
        require_ok(
            recovered.restore_calls(&store, &owner.tenant),
            "required test value"
        ),
        1
    );
    assert_eq!(
        require_ok(
            recovered.recovered_signed_call(&store, owner, GENERATION, [12; 32]),
            "required test value"
        ),
        signed(&prepared(12))
    );
    let report = require_ok(
        recovered.sweep_call_retention(&mut store, &owner.tenant, 300, 10),
        "required test value",
    );
    assert_eq!(report.preserved_unresolved_signed_bytes, 1);
    assert_eq!(
        recovered.register_call(
            &mut store,
            owner,
            GENERATION,
            [11; 32],
            &prepared(11),
            vec![]
        ),
        Err(LifecycleError::Duplicate)
    );
}

#[test]
fn f16_terminal_history_is_evicted_with_durable_duplicate_protection_after_restarts() {
    let directory = Directory::new();
    let owner = session();
    let mut store = directory.open();
    let mut lifecycle = PreparationLifecycle::default();
    let unknown = prepared(200);
    retain_unresolved_call(&mut store, &lifecycle, &owner, &unknown);
    for round in 0..8_u8 {
        let terminal = 100 + u64::from(round) * 20;
        for offset in 0..2_u8 {
            let identity = 30 + round * 2 + offset;
            verify_f16_terminal_history_is_evicted_with_durable_duplicate_protection_after_restarts_1_repair_22_33_1543(VerifyF16TerminalHistoryIsEvictedWithDurableDuplicateProtectionAfterRestarts1Repair22331543Context { owner: &owner, store: &mut store, lifecycle: &lifecycle, terminal, offset, identity });
        }
        let before = require_ok(
            lifecycle.sweep_call_retention(&mut store, &owner.tenant, terminal + 9, 10),
            "required test value",
        );
        assert_eq!(before.evicted_terminal_preparations, 0);
        assert_eq!(lifecycle.retained_count(), Ok(3));
        let swept = require_ok(
            lifecycle.sweep_call_retention(&mut store, &owner.tenant, terminal + 10, 10),
            "required test value",
        );
        assert_eq!(swept.evicted_terminal_preparations, 2);
        assert_eq!(swept.discarded_terminal_signed_bytes, 2);
        assert_eq!(swept.preserved_unresolved_signed_bytes, 1);
        assert_eq!(lifecycle.retained_count(), Ok(1));
        let again = require_ok(
            lifecycle.sweep_call_retention(&mut store, &owner.tenant, terminal + 10, 10),
            "required test value",
        );
        assert_eq!(again.evicted_terminal_preparations, 0);
        assert_eq!(again.discarded_terminal_signed_bytes, 0);
        drop(store);
        drop(lifecycle);
        store = directory.open();
        lifecycle = PreparationLifecycle::default();
        assert_eq!(
            require_ok(
                lifecycle.restore_calls(&store, &owner.tenant),
                "required test value"
            ),
            1
        );
        assert_eq!(lifecycle.retained_count(), Ok(1));
        assert_eq!(
            require_ok(
                lifecycle.recovered_signed_call(&store, &owner, GENERATION, [200; 32]),
                "required test value"
            ),
            signed(&unknown)
        );
        for offset in 0..2_u8 {
            let identity = 30 + round * 2 + offset;
            let call = prepared(identity);
            assert_eq!(
                lifecycle.register_call(
                    &mut store,
                    &owner,
                    GENERATION,
                    [identity; 32],
                    &call,
                    vec![]
                ),
                Err(LifecycleError::Duplicate)
            );
            assert_eq!(
                lifecycle.register_call(
                    &mut store,
                    &owner,
                    GENERATION + 1,
                    [identity + 100; 32],
                    &call,
                    vec![]
                ),
                Err(LifecycleError::Duplicate)
            );
            let key = require_ok(
                DurablePreparation::store_key(&owner.tenant, [identity; 32]),
                "required test value",
            );
            let compact = require_some(store.get(&key), "required test value");
            assert!(
                compact.bytes().len() < 256,
                "terminal metadata is compacted"
            );
            let decoded = require_ok(
                DurablePreparation::decode(owner.tenant.clone(), compact.bytes()),
                "required test value",
            );
            assert!(decoded.terminal());
            assert!(decoded.holds.is_empty());
            assert!(!decoded.extensions.contains_key(&EXTENSION_CALL));
        }
    }
}

struct VerifyF16TerminalHistoryIsEvictedWithDurableDuplicateProtectionAfterRestarts1Repair22331543Context<
    'a,
> {
    owner: &'a SessionRef,
    store: &'a mut layerx_agentd::store::Store,
    lifecycle: &'a PreparationLifecycle,
    terminal: u64,
    offset: u8,
    identity: u8,
}

fn verify_f16_terminal_history_is_evicted_with_durable_duplicate_protection_after_restarts_1_repair_22_33_1543(
    context: VerifyF16TerminalHistoryIsEvictedWithDurableDuplicateProtectionAfterRestarts1Repair22331543Context<'_>,
) {
    let VerifyF16TerminalHistoryIsEvictedWithDurableDuplicateProtectionAfterRestarts1Repair22331543Context { owner, store, lifecycle, terminal, offset, identity } = context;

    let call = prepared(identity);
    let id = [identity; 32];
    require_ok(
        lifecycle.register_call(store, owner, GENERATION, id, &call, vec![]),
        "required test value",
    );
    require_ok(
        lifecycle.transition_call(
            store,
            owner,
            GENERATION,
            id,
            LifecycleState::Signing,
            terminal - 2,
        ),
        "required test value",
    );
    require_ok(
        lifecycle.retain_signed_call(store, owner, GENERATION, id, signed(&call)),
        "required test value",
    );
    require_ok(
        lifecycle.admit_call_submission(store, owner, GENERATION, id, 500),
        "required test value",
    );
    require_ok(
        lifecycle.transition_call(
            store,
            owner,
            GENERATION,
            id,
            if offset == 0 {
                LifecycleState::Executed
            } else {
                LifecycleState::Failed
            },
            terminal,
        ),
        "required test value",
    );
}

#[test]
fn f16_shared_sweep_evicts_only_durably_recorded_terminal_history() {
    let directory = Directory::new();
    let owner = session();
    let mut store = directory.open();
    let lifecycle = PreparationLifecycle::default();
    let call = prepared(99);
    require_ok(
        lifecycle.register_call(&mut store, &owner, GENERATION, [99; 32], &call, vec![]),
        "required test value",
    );
    require_ok(
        lifecycle.transition_call(
            &mut store,
            &owner,
            GENERATION,
            [99; 32],
            LifecycleState::Signing,
            80,
        ),
        "required test value",
    );
    require_ok(
        lifecycle.retain_signed_call(&mut store, &owner, GENERATION, [99; 32], signed(&call)),
        "required test value",
    );
    require_ok(
        lifecycle.admit_call_submission(&mut store, &owner, GENERATION, [99; 32], 500),
        "required test value",
    );
    require_ok(
        lifecycle.transition_call(
            &mut store,
            &owner,
            GENERATION,
            [99; 32],
            LifecycleState::Executed,
            100,
        ),
        "required test value",
    );
    assert_eq!(
        require_ok(sweep_retention(&lifecycle, 109, 10), "required test value")
            .evicted_terminal_preparations,
        0
    );
    assert_eq!(
        require_ok(sweep_retention(&lifecycle, 110, 10), "required test value")
            .evicted_terminal_preparations,
        1
    );
    assert_eq!(lifecycle.retained_count(), Ok(0));
    assert_eq!(
        lifecycle.register_call(&mut store, &owner, GENERATION, [99; 32], &call, vec![]),
        Err(LifecycleError::Duplicate)
    );
    drop(store);
    let mut store = directory.open();
    assert_eq!(
        require_ok(
            PreparationLifecycle::default().restore_calls(&store, &owner.tenant),
            "required test value"
        ),
        0
    );
    assert_eq!(
        lifecycle.register_call(&mut store, &owner, GENERATION, [98; 32], &call, vec![]),
        Err(LifecycleError::Duplicate)
    );
}

#[test]
fn f17_stored_call_recovery_refuses_modified_bytes_digest_tenant_and_generation() {
    let directory = Directory::new();
    let owner = session();
    let call = prepared(17);
    let id = [17; 32];
    let mut store = directory.open();
    let lifecycle = PreparationLifecycle::default();
    require_ok(
        lifecycle.register_call(&mut store, &owner, GENERATION, id, &call, vec![]),
        "required test value",
    );
    let key = require_ok(
        DurablePreparation::store_key(&owner.tenant, id),
        "required test value",
    );
    let pristine = require_some(store.get(&key), "required test value")
        .bytes()
        .to_vec();
    drop(lifecycle);
    drop(store);
    let store = directory.open();
    let recovered = PreparationLifecycle::default();
    assert_eq!(
        require_ok(
            recovered.restore_calls(&store, &owner.tenant),
            "required test value"
        ),
        1
    );
    assert_eq!(
        require_ok(
            recovered.prepared_call(&store, &owner, GENERATION, id),
            "required test value"
        ),
        call
    );
    assert_eq!(
        recovered.prepared_call(&store, &owner, GENERATION + 1, id),
        Err(LifecycleError::AuthorizationMismatch)
    );
    let other_session = SessionRef::new(owner.tenant.clone(), SessionId([8; 32]));
    assert_eq!(
        recovered.prepared_call(&store, &other_session, GENERATION, id),
        Err(LifecycleError::AuthorizationMismatch)
    );
    let other_tenant = SessionRef::new(
        require_ok(TenantId::new("another-tenant"), "required test value"),
        owner.session_id,
    );
    let copied_key = require_ok(
        DurablePreparation::store_key(&other_tenant.tenant, id),
        "required test value",
    );
    complete_f17_stored_call_recovery_refuses_modified_bytes_digest_tenant_and_generation_2_repair_22_33_1543(CompleteF17StoredCallRecoveryRefusesModifiedBytesDigestTenantAndGeneration2Repair22331543Context { directory, owner, call, id, key, pristine, store, recovered, other_tenant, copied_key });
}

struct CompleteF17StoredCallRecoveryRefusesModifiedBytesDigestTenantAndGeneration2Repair22331543Context
{
    directory: Directory,
    owner: SessionRef,
    call: layerx_agentd::prepare::Prepared,
    id: [u8; 32],
    key: layerx_agentd::store::TenantKey,
    pristine: Vec<u8>,
    store: layerx_agentd::store::Store,
    recovered: PreparationLifecycle,
    other_tenant: SessionRef,
    copied_key: layerx_agentd::store::TenantKey,
}

fn complete_f17_stored_call_recovery_refuses_modified_bytes_digest_tenant_and_generation_2_repair_22_33_1543(
    context: CompleteF17StoredCallRecoveryRefusesModifiedBytesDigestTenantAndGeneration2Repair22331543Context,
) {
    let CompleteF17StoredCallRecoveryRefusesModifiedBytesDigestTenantAndGeneration2Repair22331543Context { directory, owner, call, id, key, pristine, mut store, recovered, other_tenant, copied_key } = context;

    require_ok(
        store.put_local(copied_key, pristine.clone()),
        "required test value",
    );
    assert_eq!(
        recovered.prepared_call(&store, &other_tenant, GENERATION, id),
        Err(LifecycleError::AuthorizationMismatch)
    );
    for mutation in 0..6 {
        let mut record = require_ok(
            DurablePreparation::decode(owner.tenant.clone(), &pristine),
            "required test value",
        );
        match mutation {
            0 => {
                let encoded = require_some(
                    record.extensions.get_mut(&EXTENSION_CALL),
                    "required test value",
                );
                let last = encoded.len() - 1;
                encoded[last] ^= 1;
            }
            1 => {
                let digest_offset = 1 + 4 + owner.tenant.as_str().len() + 32 + 8 + 32 + 20 + 8;
                require_some(
                    record.extensions.get_mut(&EXTENSION_CALL),
                    "required test value",
                )[digest_offset] ^= 1;
            }
            2 => record.generation += 1,
            3 => record.session_id[0] ^= 1,
            4 => record.not_after += 1,
            5 => record.payload_hash[0] ^= 1,
            _ => unreachable!(),
        }
        require_ok(
            store.put_local(
                key.clone(),
                require_ok(record.encode(), "required test value"),
            ),
            "required test value",
        );
        drop(store);
        store = directory.open();
        let consumer = PreparationLifecycle::default();
        assert!(
            consumer.restore_calls(&store, &owner.tenant).is_err(),
            "recovery mutation {mutation}"
        );
        assert_eq!(
            consumer.retained_count(),
            Ok(0),
            "failed recovery publishes no records"
        );
    }
    complete_f17_stored_call_recovery_refuses_modified_bytes_digest_tenant_and_generation_1_repair_22_33_1543(CompleteF17StoredCallRecoveryRefusesModifiedBytesDigestTenantAndGeneration1Repair22331543Context { directory, owner, call, id, key, pristine, store });
}

struct CompleteF17StoredCallRecoveryRefusesModifiedBytesDigestTenantAndGeneration1Repair22331543Context
{
    directory: Directory,
    owner: SessionRef,
    call: layerx_agentd::prepare::Prepared,
    id: [u8; 32],
    key: layerx_agentd::store::TenantKey,
    pristine: Vec<u8>,
    store: layerx_agentd::store::Store,
}

fn complete_f17_stored_call_recovery_refuses_modified_bytes_digest_tenant_and_generation_1_repair_22_33_1543(
    context: CompleteF17StoredCallRecoveryRefusesModifiedBytesDigestTenantAndGeneration1Repair22331543Context,
) {
    let CompleteF17StoredCallRecoveryRefusesModifiedBytesDigestTenantAndGeneration1Repair22331543Context { directory, owner, call, id, key, pristine, mut store } = context;

    require_ok(
        store.put_local(key.clone(), pristine),
        "required test value",
    );
    let recovered = PreparationLifecycle::default();
    assert_eq!(
        require_ok(
            recovered.restore_calls(&store, &owner.tenant),
            "required test value"
        ),
        1
    );
    require_ok(
        recovered.transition_call(
            &mut store,
            &owner,
            GENERATION,
            id,
            LifecycleState::Signing,
            81,
        ),
        "required test value",
    );
    require_ok(
        recovered.retain_signed_call(&mut store, &owner, GENERATION, id, signed(&call)),
        "required test value",
    );
    let mut record = require_ok(
        DurablePreparation::decode(
            owner.tenant.clone(),
            require_some(store.get(&key), "required test value").bytes(),
        ),
        "required test value",
    );
    require_some(record.extensions.get_mut(&4), "required test value")[10] ^= 1;
    require_ok(
        store.put_local(key, require_ok(record.encode(), "required test value")),
        "required test value",
    );
    drop(store);
    let store = directory.open();
    assert!(PreparationLifecycle::default()
        .restore_calls(&store, &owner.tenant)
        .is_err());
}

#[test]
fn f16_retention_refuses_eviction_without_durable_idempotency() {
    use layerx_agentd::store::{ObjectKind, TenantKey};
    let directory = Directory::new();
    let owner = session();
    let mut store = directory.open();
    let lifecycle = PreparationLifecycle::default();
    let call = prepared(88);
    require_ok(
        lifecycle.register_call(&mut store, &owner, GENERATION, [88; 32], &call, vec![]),
        "required test value",
    );
    require_ok(
        lifecycle.transition_call(
            &mut store,
            &owner,
            GENERATION,
            [88; 32],
            LifecycleState::Signing,
            80,
        ),
        "required test value",
    );
    require_ok(
        lifecycle.retain_signed_call(&mut store, &owner, GENERATION, [88; 32], signed(&call)),
        "required test value",
    );
    require_ok(
        lifecycle.admit_call_submission(&mut store, &owner, GENERATION, [88; 32], 500),
        "required test value",
    );
    require_ok(
        lifecycle.transition_call(
            &mut store,
            &owner,
            GENERATION,
            [88; 32],
            LifecycleState::Executed,
            100,
        ),
        "required test value",
    );
    let ids = store.list_object_ids(&owner.tenant, ObjectKind::Idempotency);
    assert_eq!(ids.len(), 1);
    let key = require_ok(
        TenantKey::new(
            owner.tenant.clone(),
            ObjectKind::Idempotency,
            ids[0].clone(),
        ),
        "required test value",
    );
    let identity = require_some(store.get(&key), "required test value")
        .bytes()
        .to_vec();
    assert!(require_ok(store.remove_local(&key), "required test value"));
    assert_eq!(
        lifecycle.sweep_call_retention(&mut store, &owner.tenant, 110, 10),
        Err(LifecycleError::InvalidSignedBytes)
    );
    assert_eq!(lifecycle.retained_count(), Ok(1));
    assert_eq!(lifecycle.has_signed_bytes([88; 32]), Ok(true));
    require_ok(store.put_local(key, identity), "required test value");
    assert_eq!(
        require_ok(
            lifecycle.sweep_call_retention(&mut store, &owner.tenant, 110, 10),
            "required test value"
        )
        .evicted_terminal_preparations,
        1
    );
    assert_eq!(lifecycle.retained_count(), Ok(0));
}

fn retain_unresolved_call(
    store: &mut layerx_agentd::store::Store,
    lifecycle: &PreparationLifecycle,
    owner: &SessionRef,
    unknown: &layerx_agentd::prepare::Prepared,
) {
    require_ok(
        lifecycle.register_call(store, owner, GENERATION, [200; 32], unknown, vec![]),
        "required test value",
    );
    require_ok(
        lifecycle.transition_call(
            store,
            owner,
            GENERATION,
            [200; 32],
            LifecycleState::Signing,
            80,
        ),
        "required test value",
    );
    require_ok(
        lifecycle.retain_signed_call(store, owner, GENERATION, [200; 32], signed(unknown)),
        "required test value",
    );
    require_ok(
        lifecycle.admit_call_submission(store, owner, GENERATION, [200; 32], 500),
        "required test value",
    );
    require_ok(
        lifecycle.transition_call(
            store,
            owner,
            GENERATION,
            [200; 32],
            LifecycleState::Unknown,
            81,
        ),
        "required test value",
    );
}
