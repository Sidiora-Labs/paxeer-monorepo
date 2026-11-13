use super::*;
use crate::outbox::{Outbox, SubmissionState};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use std::fmt::Debug;

use crate as agentd;

#[path = "registration_tests.rs"]
mod registration_tests;

#[test]
fn managed_registration_resolves_restored_subject_reads_and_writes() {
    registration_tests::prove_registration_resolution(|store, alice, bob, restart| {
        if !restart {
            must(retain(store, alice));
            must(retain(store, bob));
            return (alice.clone(), bob.clone());
        }
        let configured = BTreeMap::from([(
            7,
            ("human-transport".to_owned(), "human-service".to_owned()),
        )]);
        let restored = must(restore_peers(store, &configured));
        assert_eq!(restored.len(), 3);
        let restored_peer = |expected: &HumanPeer| {
            let actual = restored
                .iter()
                .find(|peer| peer.tenant == expected.tenant)
                .unwrap_or_else(|| panic!("restored managed subject"));
            assert_eq!(actual, expected);
            actual.clone()
        };
        (restored_peer(alice), restored_peer(bob))
    });
}

fn must<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("subject invariant: {error:?}"))
}
fn scoped(principal: &str) -> HumanPeer {
    HumanPeer {
        uid: 7,
        tenant: must(layerx_identity_binding::subject_namespace(
            "service-tenant",
            principal,
        )),
        principal: principal.to_owned(),
        subject: Some(HumanSubject {
            transport_tenant: "service-tenant".to_owned(),
            transport_principal: "human-service".to_owned(),
            owner: format!("did:layerx:{principal}"),
            account: format!("agent:did:layerx:{principal}:main"),
            asset: [3; 32],
            registration: None,
        }),
    }
}

#[test]
fn durable_subject_bindings_preserve_service_identity_and_refuse_remap_after_restart() {
    let root = std::env::temp_dir().join(format!("lxp-subject-bindings-{}", std::process::id()));
    let mut store = must(Store::open(&root));
    let configured =
        BTreeMap::from([(7, ("human-service".to_owned(), "service-tenant".to_owned()))]);
    let alice = scoped("alice");
    let bob = scoped("bob");
    must(retain(&mut store, &alice));
    must(retain(&mut store, &bob));
    must(retain(&mut store, &alice));
    let mut changed = alice.clone();
    changed
        .subject
        .as_mut()
        .unwrap_or_else(|| panic!("scope"))
        .owner = "did:layerx:bob".to_owned();
    assert!(retain(&mut store, &changed).is_err());
    changed = alice.clone();
    changed.tenant = bob.tenant.clone();
    assert!(retain(&mut store, &changed).is_err());
    drop(store);
    let mut store = must(Store::open(&root));
    let restored = must(restore_peers(&store, &configured));
    assert_eq!(restored.len(), 3);
    assert!(restored.contains(&alice));
    assert!(restored.contains(&bob));
    assert!(restore_peers(
        &store,
        &BTreeMap::from([(
            7,
            ("different-service".to_owned(), "service-tenant".to_owned())
        )])
    )
    .is_err());
    let key = must(TenantKey::new(
        must(TenantId::new(alice.tenant.clone())),
        ObjectKind::Configuration,
        KEY,
    ));
    let raw = store
        .get(&key)
        .unwrap_or_else(|| panic!("durable scope"))
        .bytes()
        .to_vec();
    let mut value: serde_json::Value = must(serde_json::from_slice(&raw));
    value["principal"] = serde_json::json!("bob");
    must(store.put_local(key, must(serde_json::to_vec(&value))));
    assert!(restore_peers(&store, &configured).is_err());
}

#[allow(
    deprecated,
    reason = "the legacy sponsored activity fixture proves retained compatibility"
)]
fn signed_submission() -> crate::sign::VerifiedSubmission {
    let bytes = include_bytes!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../tests/fixtures/authority/provider-subject/sponsored.activity"
    ));
    let kind = must(ActivityType::new(ModuleId::Governance, 1));
    let registry = must(ModuleRegistry::new(&[must(ModuleRegistration::new(
        ModuleId::Governance,
        &[kind],
    ))]));
    let activity = must(layerx_wire::historical::activity::decode_signed(
        bytes, &registry,
    ));
    assert_eq!(
        must(layerx_wire::historical::activity::encode_signed(&activity)),
        bytes
    );
    let registration = must(layerx_crypto::payments::SponsoredRegistration::decode(
        activity.payload(),
    ));
    must(registration.validate_outer(&activity));
    assert_eq!(
        registration.signed_consent(),
        include_bytes!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../../../tests/fixtures/authority/provider-subject/consent.activity"
        )),
    );
    let call = Eip1559Call {
        chain_id: u64::from(activity.network_id()),
        nonce: activity.account_sequence(),
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 7,
        gas_limit: 100_000,
        to: crate::prepare::evm::PrecompileTarget::Auth.address(),
        data: must(
            crate::prepare::evm::PrecompileCall {
                target: crate::prepare::evm::PrecompileTarget::Auth,
                selector: layerx_crypto::disclosure::call::ledger_selector(
                    "recordLifecycle(bytes,bytes32)",
                ),
                arguments: vec![
                    crate::prepare::evm::AbiArgument::Bytes(activity.payload().to_vec()),
                    crate::prepare::evm::AbiArgument::Word(activity.idempotency_key()),
                ],
            }
            .encode(),
        ),
    };
    let signed = must(call.sign(&[1; 32]));
    let (decoded, signature) = must(decode_signed(&signed));
    assert_eq!(decoded, call);
    let actor_address = must(recover_sender(&call, &signature));
    let disclosure = must(layerx_crypto::disclosure::call::bind_call(
        &call,
        actor_address,
    ));
    let prepared = crate::sign::SigningPreparation {
        transaction: crate::prepare::EvmPrepared {
            from: actor_address,
            disclosure_digest: must(disclosure.signing_digest()),
            disclosure,
            signing_hash: call.signing_hash(),
            call,
            observed_head_height: 0,
        },
        actor_address,
    };
    must(crate::sign::verify_before_submit(
        &signed,
        &prepared,
        &actor_address,
        &registry,
    ))
}

#[test]
fn exact_signed_queue_is_partitioned_before_transmission_and_restores_independently() {
    let root = std::env::temp_dir().join(format!("lxp-subject-outbox-{}", std::process::id()));
    let mut store = must(Store::open(&root));
    let alice = scoped("alice");
    let bob = scoped("bob");
    must(retain(&mut store, &alice));
    must(retain(&mut store, &bob));
    let submission = signed_submission();
    let original = submission.exact_bytes().to_vec();
    let mut outboxes = BTreeMap::<String, Outbox>::new();
    let [id, bob_id] = [&alice, &bob].map(|peer| {
        must(
            outboxes
                .entry(peer.tenant.clone())
                .or_default()
                .enqueue_with_origin(
                    &mut store,
                    &must(TenantId::new(peer.tenant.clone())),
                    submission.idempotency_key(),
                    submission.clone(),
                    None,
                    None,
                ),
        )
    });
    assert_eq!(id, bob_id);
    let sent = must(begin_transmission(
        outboxes
            .get_mut(&alice.tenant)
            .unwrap_or_else(|| panic!("alice queue")),
        &mut store,
        id,
    ));
    assert_eq!(sent, original);
    assert_eq!(
        outboxes[&alice.tenant]
            .status(id)
            .unwrap_or_else(|| panic!("alice status"))
            .state,
        SubmissionState::Submitted
    );
    assert_eq!(
        outboxes[&bob.tenant]
            .status(id)
            .unwrap_or_else(|| panic!("bob status"))
            .state,
        SubmissionState::Queued
    );
    assert!(begin_transmission(
        outboxes
            .get_mut(&alice.tenant)
            .unwrap_or_else(|| panic!("alice queue")),
        &mut store,
        id
    )
    .is_err());
    drop(store);
    let mut store = must(Store::open(&root));
    let mut alice_queue = Outbox::default();
    let mut bob_queue = Outbox::default();
    must(alice_queue.restore(&store, must(TenantId::new(alice.tenant)), id));
    must(bob_queue.restore(&store, must(TenantId::new(bob.tenant)), id));
    assert_eq!(must(alice_queue.exact_signed_bytes(id)), original);
    assert_eq!(must(bob_queue.bytes_for_transmission(id)), original);
    assert_eq!(
        alice_queue
            .status(id)
            .unwrap_or_else(|| panic!("restored alice"))
            .state,
        SubmissionState::Submitted
    );
    assert_eq!(
        bob_queue
            .status(id)
            .unwrap_or_else(|| panic!("restored bob"))
            .state,
        SubmissionState::Queued
    );
    assert_eq!(
        must(begin_transmission(&mut bob_queue, &mut store, id)),
        original
    );
    assert_eq!(
        bob_queue
            .status(id)
            .unwrap_or_else(|| panic!("resumed Bob queue"))
            .state,
        SubmissionState::Submitted
    );
}
