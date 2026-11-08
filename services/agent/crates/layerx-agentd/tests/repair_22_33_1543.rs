//! Finding-to-test mapping and deferred domain wiring: prepare/repair_22_33_1543.kvx.

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
    let decoded = disclose::restore_call(
        &original.call.encode_unsigned(),
        OWNER,
        original.observed_head_height,
        original.disclosure_digest,
        original.signing_hash,
    )
    .unwrap();
    assert_eq!(decoded, original);
    assert_eq!(
        disclose::bind(&original.call, OWNER).unwrap().disclosure,
        original.disclosure
    );
    let raw = signed(&decoded);
    let (call, signature) = decode_signed(&raw).unwrap();
    assert_eq!(call, original.call);
    assert_eq!(recover_sender(&call, &signature).unwrap(), OWNER);
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
    verify_call_binding(&produced).unwrap();
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
    let record =
        DurablePreparation::from_call(&owner, GENERATION, [13; 32], &produced, vec![]).unwrap();
    let bytes = record.encode().unwrap();
    assert_eq!(bytes[4], 3, "call records use an explicit format version");
    let decoded = DurablePreparation::decode(owner.tenant.clone(), &bytes).unwrap();
    assert_eq!(decoded.restore_call(&owner, GENERATION).unwrap(), produced);
    assert_eq!(decoded.encode().unwrap(), bytes);
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
    let old_bytes = old.encode().unwrap();
    assert_eq!(old_bytes[4], 1);
    let old = DurablePreparation::decode(owner.tenant.clone(), &old_bytes).unwrap();
    assert_eq!(old.encode().unwrap(), old_bytes);
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
        let id = [identity; 32];
        let hold = reserve(
            &budget,
            &ReservationRequest {
                id,
                amount: 100,
                expiry_sequence: 2_000,
                current_sequence: 80,
                applicable_limits: vec![LimitId([1; 16])],
            },
        )
        .unwrap();
        let prepared = prepared(identity);
        lifecycle
            .register_call(
                &mut store,
                &owner,
                GENERATION,
                id,
                &prepared,
                hold.durable.into_iter().map(|hold| (hold, None)).collect(),
            )
            .unwrap();
        lifecycle
            .transition_call(
                &mut store,
                &owner,
                GENERATION,
                id,
                LifecycleState::Signing,
                81,
            )
            .unwrap();
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
            let wrong_signer = prepared.call.sign(&[2; 32]).unwrap();
            assert_eq!(
                lifecycle.retain_signed_call(&mut store, &owner, GENERATION, id, wrong_signer),
                Err(LifecycleError::InvalidSignedBytes)
            );
            let raw = signed(&prepared);
            assert_eq!(
                lifecycle
                    .retain_signed_call(&mut store, &owner, GENERATION, id, raw.clone())
                    .unwrap(),
                transaction_hash(&raw)
            );
            if identity == 12 {
                assert_eq!(
                    lifecycle
                        .admit_call_submission(&mut store, &owner, GENERATION, id, 900)
                        .unwrap(),
                    raw
                );
                lifecycle
                    .transition_call(
                        &mut store,
                        &owner,
                        GENERATION,
                        id,
                        LifecycleState::Unknown,
                        82,
                    )
                    .unwrap();
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
    assert_eq!(budget.held_reservations(), Ok(3));
    drop(lifecycle);
    drop(budget);
    drop(store);
    let mut store = directory.open();
    let lifecycle = PreparationLifecycle::default();
    assert_eq!(lifecycle.restore_calls(&store, &owner.tenant).unwrap(), 3);
    let budget = limiter();
    let durable = DurablePreparation::load_all(&store, &owner.tenant).unwrap();
    for record in &durable {
        restore(
            &budget,
            &record
                .holds
                .iter()
                .map(|(hold, _)| hold.clone())
                .collect::<Vec<_>>(),
        )
        .unwrap();
    }
    assert_eq!(budget.held_reservations(), Ok(3));
    assert_eq!(lifecycle.state([13; 32]), Ok(LifecycleState::Signed));
    assert_eq!(
        lifecycle
            .recovered_signed_call(&store, &owner, GENERATION, [13; 32])
            .unwrap(),
        signed(&prepared(13))
    );
    assert_eq!(
        lifecycle
            .prepared_call(&store, &owner, GENERATION, [12; 32])
            .unwrap(),
        prepared(12)
    );
    assert_eq!(
        lifecycle
            .recovered_signed_call(&store, &owner, GENERATION, [12; 32])
            .unwrap(),
        signed(&prepared(12))
    );
    let expired = lifecycle
        .expire_calls(&mut store, &owner.tenant, &budget, EXPIRY + 1, 200)
        .unwrap();
    assert_eq!(expired.expired_preparations, vec![[11; 32], [13; 32]]);
    assert_eq!(expired.released_reservations, vec![[11; 32], [13; 32]]);
    assert_eq!(budget.held_reservations(), Ok(1));
    assert_eq!(lifecycle.state([12; 32]), Ok(LifecycleState::Unknown));
    assert_eq!(lifecycle.state([13; 32]), Ok(LifecycleState::Expired));
    assert_eq!(
        lifecycle.admit_call_submission(&mut store, &owner, GENERATION, [13; 32], EXPIRY + 1),
        Err(LifecycleError::PreparationExpired)
    );
    drop(store);
    drop(lifecycle);
    let mut store = directory.open();
    let recovered = PreparationLifecycle::default();
    assert_eq!(recovered.restore_calls(&store, &owner.tenant).unwrap(), 1);
    assert_eq!(
        recovered
            .recovered_signed_call(&store, &owner, GENERATION, [12; 32])
            .unwrap(),
        signed(&prepared(12))
    );
    let report = recovered
        .sweep_call_retention(&mut store, &owner.tenant, 300, 10)
        .unwrap();
    assert_eq!(report.preserved_unresolved_signed_bytes, 1);
    assert_eq!(
        recovered.register_call(
            &mut store,
            &owner,
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
    lifecycle
        .register_call(&mut store, &owner, GENERATION, [200; 32], &unknown, vec![])
        .unwrap();
    lifecycle
        .transition_call(
            &mut store,
            &owner,
            GENERATION,
            [200; 32],
            LifecycleState::Signing,
            80,
        )
        .unwrap();
    lifecycle
        .retain_signed_call(&mut store, &owner, GENERATION, [200; 32], signed(&unknown))
        .unwrap();
    lifecycle
        .admit_call_submission(&mut store, &owner, GENERATION, [200; 32], 500)
        .unwrap();
    lifecycle
        .transition_call(
            &mut store,
            &owner,
            GENERATION,
            [200; 32],
            LifecycleState::Unknown,
            81,
        )
        .unwrap();
    for round in 0..8_u8 {
        let terminal = 100 + u64::from(round) * 20;
        for offset in 0..2_u8 {
            let identity = 30 + round * 2 + offset;
            let call = prepared(identity);
            let id = [identity; 32];
            lifecycle
                .register_call(&mut store, &owner, GENERATION, id, &call, vec![])
                .unwrap();
            lifecycle
                .transition_call(
                    &mut store,
                    &owner,
                    GENERATION,
                    id,
                    LifecycleState::Signing,
                    terminal - 2,
                )
                .unwrap();
            lifecycle
                .retain_signed_call(&mut store, &owner, GENERATION, id, signed(&call))
                .unwrap();
            lifecycle
                .admit_call_submission(&mut store, &owner, GENERATION, id, 500)
                .unwrap();
            lifecycle
                .transition_call(
                    &mut store,
                    &owner,
                    GENERATION,
                    id,
                    if offset == 0 {
                        LifecycleState::Executed
                    } else {
                        LifecycleState::Failed
                    },
                    terminal,
                )
                .unwrap();
        }
        let before = lifecycle
            .sweep_call_retention(&mut store, &owner.tenant, terminal + 9, 10)
            .unwrap();
        assert_eq!(before.evicted_terminal_preparations, 0);
        assert_eq!(lifecycle.retained_count(), Ok(3));
        let swept = lifecycle
            .sweep_call_retention(&mut store, &owner.tenant, terminal + 10, 10)
            .unwrap();
        assert_eq!(swept.evicted_terminal_preparations, 2);
        assert_eq!(swept.discarded_terminal_signed_bytes, 2);
        assert_eq!(swept.preserved_unresolved_signed_bytes, 1);
        assert_eq!(lifecycle.retained_count(), Ok(1));
        let again = lifecycle
            .sweep_call_retention(&mut store, &owner.tenant, terminal + 10, 10)
            .unwrap();
        assert_eq!(again.evicted_terminal_preparations, 0);
        assert_eq!(again.discarded_terminal_signed_bytes, 0);
        drop(store);
        drop(lifecycle);
        store = directory.open();
        lifecycle = PreparationLifecycle::default();
        assert_eq!(lifecycle.restore_calls(&store, &owner.tenant).unwrap(), 1);
        assert_eq!(lifecycle.retained_count(), Ok(1));
        assert_eq!(
            lifecycle
                .recovered_signed_call(&store, &owner, GENERATION, [200; 32])
                .unwrap(),
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
            let key = DurablePreparation::store_key(&owner.tenant, [identity; 32]).unwrap();
            let compact = store.get(&key).unwrap();
            assert!(
                compact.bytes().len() < 256,
                "terminal metadata is compacted"
            );
            let decoded =
                DurablePreparation::decode(owner.tenant.clone(), compact.bytes()).unwrap();
            assert!(decoded.terminal());
            assert!(decoded.holds.is_empty());
            assert!(!decoded.extensions.contains_key(&EXTENSION_CALL));
        }
    }
}

#[test]
fn f16_shared_sweep_evicts_only_durably_recorded_terminal_history() {
    let directory = Directory::new();
    let owner = session();
    let mut store = directory.open();
    let lifecycle = PreparationLifecycle::default();
    let call = prepared(99);
    lifecycle
        .register_call(&mut store, &owner, GENERATION, [99; 32], &call, vec![])
        .unwrap();
    lifecycle
        .transition_call(
            &mut store,
            &owner,
            GENERATION,
            [99; 32],
            LifecycleState::Signing,
            80,
        )
        .unwrap();
    lifecycle
        .retain_signed_call(&mut store, &owner, GENERATION, [99; 32], signed(&call))
        .unwrap();
    lifecycle
        .admit_call_submission(&mut store, &owner, GENERATION, [99; 32], 500)
        .unwrap();
    lifecycle
        .transition_call(
            &mut store,
            &owner,
            GENERATION,
            [99; 32],
            LifecycleState::Executed,
            100,
        )
        .unwrap();
    assert_eq!(
        sweep_retention(&lifecycle, 109, 10)
            .unwrap()
            .evicted_terminal_preparations,
        0
    );
    assert_eq!(
        sweep_retention(&lifecycle, 110, 10)
            .unwrap()
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
        PreparationLifecycle::default()
            .restore_calls(&store, &owner.tenant)
            .unwrap(),
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
    lifecycle
        .register_call(&mut store, &owner, GENERATION, id, &call, vec![])
        .unwrap();
    let key = DurablePreparation::store_key(&owner.tenant, id).unwrap();
    let pristine = store.get(&key).unwrap().bytes().to_vec();
    drop(lifecycle);
    drop(store);
    let mut store = directory.open();
    let recovered = PreparationLifecycle::default();
    assert_eq!(recovered.restore_calls(&store, &owner.tenant).unwrap(), 1);
    assert_eq!(
        recovered
            .prepared_call(&store, &owner, GENERATION, id)
            .unwrap(),
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
    let other_tenant = SessionRef::new(TenantId::new("another-tenant").unwrap(), owner.session_id);
    let copied_key = DurablePreparation::store_key(&other_tenant.tenant, id).unwrap();
    store.put_local(copied_key, pristine.clone()).unwrap();
    assert_eq!(
        recovered.prepared_call(&store, &other_tenant, GENERATION, id),
        Err(LifecycleError::AuthorizationMismatch)
    );
    for mutation in 0..6 {
        let mut record = DurablePreparation::decode(owner.tenant.clone(), &pristine).unwrap();
        match mutation {
            0 => {
                let encoded = record.extensions.get_mut(&EXTENSION_CALL).unwrap();
                let last = encoded.len() - 1;
                encoded[last] ^= 1;
            }
            1 => {
                let digest_offset = 1 + 4 + owner.tenant.as_str().len() + 32 + 8 + 32 + 20 + 8;
                record.extensions.get_mut(&EXTENSION_CALL).unwrap()[digest_offset] ^= 1;
            }
            2 => record.generation += 1,
            3 => record.session_id[0] ^= 1,
            4 => record.not_after += 1,
            5 => record.payload_hash[0] ^= 1,
            _ => unreachable!(),
        }
        store
            .put_local(key.clone(), record.encode().unwrap())
            .unwrap();
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
    store.put_local(key.clone(), pristine).unwrap();
    let recovered = PreparationLifecycle::default();
    assert_eq!(recovered.restore_calls(&store, &owner.tenant).unwrap(), 1);
    recovered
        .transition_call(
            &mut store,
            &owner,
            GENERATION,
            id,
            LifecycleState::Signing,
            81,
        )
        .unwrap();
    recovered
        .retain_signed_call(&mut store, &owner, GENERATION, id, signed(&call))
        .unwrap();
    let mut record =
        DurablePreparation::decode(owner.tenant.clone(), store.get(&key).unwrap().bytes()).unwrap();
    record.extensions.get_mut(&4).unwrap()[10] ^= 1;
    store.put_local(key, record.encode().unwrap()).unwrap();
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
    lifecycle
        .register_call(&mut store, &owner, GENERATION, [88; 32], &call, vec![])
        .unwrap();
    lifecycle
        .transition_call(
            &mut store,
            &owner,
            GENERATION,
            [88; 32],
            LifecycleState::Signing,
            80,
        )
        .unwrap();
    lifecycle
        .retain_signed_call(&mut store, &owner, GENERATION, [88; 32], signed(&call))
        .unwrap();
    lifecycle
        .admit_call_submission(&mut store, &owner, GENERATION, [88; 32], 500)
        .unwrap();
    lifecycle
        .transition_call(
            &mut store,
            &owner,
            GENERATION,
            [88; 32],
            LifecycleState::Executed,
            100,
        )
        .unwrap();
    let ids = store.list_object_ids(&owner.tenant, ObjectKind::Idempotency);
    assert_eq!(ids.len(), 1);
    let key = TenantKey::new(
        owner.tenant.clone(),
        ObjectKind::Idempotency,
        ids[0].clone(),
    )
    .unwrap();
    let identity = store.get(&key).unwrap().bytes().to_vec();
    assert!(store.remove_local(&key).unwrap());
    assert_eq!(
        lifecycle.sweep_call_retention(&mut store, &owner.tenant, 110, 10),
        Err(LifecycleError::InvalidSignedBytes)
    );
    assert_eq!(lifecycle.retained_count(), Ok(1));
    assert_eq!(lifecycle.has_signed_bytes([88; 32]), Ok(true));
    store.put_local(key, identity).unwrap();
    assert_eq!(
        lifecycle
            .sweep_call_retention(&mut store, &owner.tenant, 110, 10)
            .unwrap()
            .evicted_terminal_preparations,
        1
    );
    assert_eq!(lifecycle.retained_count(), Ok(0));
}
