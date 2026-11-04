//! Task 22.31.151, finding native-onboarding-refused: native_onboarding_identity_producer_transport_and_restart
//! proves the chain identity producer, signed-call binding and durable restart interface.
//! Baseline verification was not run: the gate order requires implementation first.
//! Wiring request (domain 10): services/human/crates/layerx-human-service/src/server/{agent_creation_native.rs,production_components.rs} must drive prepare_native_identity -> retain_native_identity_submission -> chain submission/receipt -> accept_native_identity before resume_native; HTTP creation-to-receipt and restart qualification remains outstanding.

use std::fmt::Debug;
use std::fs;
use std::path::PathBuf;

use layerx_agentd::prepare::{
    prepare_bound, EvmPreparationDefaults, EvmPreparationState, EvmPrepareRequest,
};
use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_human_service::agents::{
    AgentCreationError, AgentFailure, CreateAgentRequest, CreationContext, CreationJourney,
    CreationStage, NativeIdentityCreationRequest, ProtocolEvidence, PurposePresetCatalog,
    StageState,
};
use layerx_human_service::custody::{EnvelopeKms, Keystore};
use layerx_human_service::store::{PrincipalStore, Table, TenancyDigest};
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, tenancy,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::rung::Rung;
use serde_json::json;
use sha2::{Digest as _, Sha256};

const NETWORK: u32 = 77;
const ASSET: [u8; 32] = [0x22; 32];

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("repair 22.31.151: {error:?}"))
}

struct Fixture {
    root: PathBuf,
    digest: TenancyDigest,
}

impl Fixture {
    fn new(label: &str) -> (Self, PrincipalStore, Keystore) {
        let root = directory(label);
        checked(fs::create_dir_all(&root));
        checked(fs::write(root.join("kms-root"), [0x42; 64]));
        let (store, digest) = install_and_open(
            &root.join("store"),
            &tenancy(&[("alice", "tenant-alice")]),
            retention_uniform(100),
        );
        let fixture = Self { root, digest };
        let custody = fixture.custody();
        (fixture, store, custody)
    }

    fn custody(&self) -> Keystore {
        checked(Keystore::open_development(
            self.root.join("custody"),
            NETWORK,
            checked(EnvelopeKms::new(
                "kms://repair-native-identity",
                self.root.join("kms-root"),
            )),
        ))
    }

    fn store(&self) -> PrincipalStore {
        checked(PrincipalStore::open(
            self.root.join("store"),
            retention_uniform(100),
            self.digest,
        ))
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

fn registry() -> ModuleRegistry {
    checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
        ModuleId::Governance,
        &[
            checked(ActivityType::new(ModuleId::Governance, 1)),
            checked(ActivityType::new(ModuleId::Governance, 3)),
        ],
    ))]))
}

fn catalog() -> PurposePresetCatalog {
    let counterparty = [0x11; 32];
    checked(PurposePresetCatalog::from_json(&checked(
        serde_json::to_vec(&json!({
            "version": "repair-native-identity-v1",
            "presets": [{
                "id": "treasury",
                "call_selectors": ["0xa9059cbb"],
                "counterparties": [counterparty],
                "assets": [ASSET],
                "amount_ceiling": 2000,
                "rate_maximum_uses": 12,
                "rate_window_sequences": 50,
                "purposes": ["treasury"],
                "expiry_sequence": 50000,
                "session_scopes": ["prepare", "submit"],
                "session_lifetime_seconds": 5000,
                "budget_asset": ASSET,
                "budget_period_seconds": 2592000,
                "budget_expiry_seconds": 31536000,
                "initial_funding": 250
            }]
        })),
    )))
}

fn request() -> CreateAgentRequest {
    checked(
        checked(CreateAgentRequest::new("Treasury", "treasury", 1000, "LXP"))
            .with_activity_types(vec![checked(ActivityType::new(ModuleId::Asset, 5))]),
    )
}

fn context() -> CreationContext {
    CreationContext {
        idempotency_key: [0x31; 32],
        owner_account: "agent:did:layerx:alice:main".to_owned(),
        human_recovery_root: [0x43; 32],
        recovery_threshold: 2,
        network_id: NETWORK,
        protocol_time: 1000,
    }
}

fn signed_identity(request: &NativeIdentityCreationRequest, nonce: u64) -> Vec<u8> {
    let key = checked(k256::ecdsa::SigningKey::from_slice(&[0x68; 32]));
    let from = checked(layerx_crypto::secp256k1::evm_address(
        key.verifying_key().to_encoded_point(false).as_bytes(),
    ));
    let prepared = checked(prepare_bound(
        EvmPreparationState {
            chain_id: u64::from(NETWORK),
            pending_nonce: nonce,
            base_fee_per_gas: 1,
            head_height: 10,
        },
        EvmPreparationDefaults {
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            maximum_payload_bytes: 4096,
        },
        EvmPrepareRequest {
            from,
            call: request.call(),
            expected_nonce: Some(nonce),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 4096,
        },
        100000,
    ));
    checked(prepared.call.sign(&[0x68; 32]))
}

#[test]
fn native_onboarding_identity_producer_transport_and_restart() {
    let (fixture, mut store, custody) = Fixture::new("repair-native-identity-restart");
    let registry = registry();
    let mut scope = checked(store.principal(&principal("alice")));
    let mut journey = checked(CreationJourney::start_native(
        &mut scope,
        &request(),
        &context(),
        &catalog(),
        1000,
        ASSET,
        300,
    ));
    let identity = checked(journey.prepare_native_identity(&mut scope, &custody, &registry, 1001));
    let signed = signed_identity(&identity, 1);
    let (call, signature) = checked(decode_signed(&signed));
    let disclosure = checked(bind_call(&call, checked(recover_sender(&call, &signature))));
    assert_eq!(disclosure.activity_type, identity.compiled.activity_type());
    assert_eq!(disclosure.idempotency_key, identity.action_key);
    let Some(ConstructMethod::AuthLifecycle { payload, .. }) =
        checked(disclosure.construct_method())
    else {
        panic!("producer did not emit the current chain authentication call");
    };
    assert_eq!(payload, identity.compiled.payload().as_bytes());
    assert_eq!(&payload[..4], &[0x71, 1, 0, 2]);
    assert_eq!(
        &payload[4..36],
        &checked(layerx_intents::canonical::did_id_for_protocol(
            &identity.did,
            3
        ))
    );
    assert_eq!(&payload[36..], &identity.primary_key);
    let transaction =
        checked(journey.retain_native_identity_submission(&mut scope, &registry, &signed, 1002));
    assert_eq!(transaction, transaction_hash(&signed));
    assert_eq!(checked(identity.verify_signed(&signed)), transaction);
    assert_eq!(
        checked(journey.native_identity_submission(&scope, &registry)),
        Some(signed.clone())
    );
    assert_eq!(
        journey.status().stages[0],
        (CreationStage::Custody, StageState::LocalComplete)
    );
    assert_eq!(
        journey.status().stages[1],
        (CreationStage::DidRegistration, StageState::Pending)
    );
    let status = journey.status();
    let id = journey.agent_id();
    drop(scope);
    drop(journey);
    drop(store);
    drop(custody);

    let mut store = fixture.store();
    let custody = fixture.custody();
    let mut scope = checked(store.principal(&principal("alice")));
    let mut recovered = checked(CreationJourney::load(&scope, id)).expect("durable creation");
    assert_eq!(recovered.status(), status);
    assert_eq!(
        checked(recovered.prepare_native_identity(&mut scope, &custody, &registry, 1003)),
        identity
    );
    assert_eq!(
        checked(recovered.native_identity_submission(&scope, &registry)),
        Some(signed.clone())
    );
    assert_eq!(
        checked(recovered.retain_native_identity_submission(&mut scope, &registry, &signed, 1004)),
        transaction
    );
    let changed_nonce = signed_identity(&identity, 2);
    assert!(matches!(
        recovered.retain_native_identity_submission(&mut scope, &registry, &changed_nonce, 1005),
        Err(AgentCreationError::EvidenceConflict)
    ));
    assert_eq!(
        checked(recovered.native_identity_submission(&scope, &registry)),
        Some(signed)
    );
    assert_eq!(checked(CreationJourney::list(&scope)), vec![recovered]);
}

#[test]
fn native_onboarding_identity_refuses_cross_journey_network_and_payload() {
    let (_fixture, mut store, custody) = Fixture::new("repair-native-identity-bindings");
    let mut scope = checked(store.principal(&principal("alice")));
    let registry = registry();
    let mut journey = checked(CreationJourney::start_native(
        &mut scope,
        &request(),
        &context(),
        &catalog(),
        1000,
        ASSET,
        300,
    ));
    let identity = checked(journey.prepare_native_identity(&mut scope, &custody, &registry, 1001));
    for mutation in 0..3 {
        let mut other = identity.clone();
        match mutation {
            0 => other.action_key[0] ^= 1,
            1 => other.network_id += 1,
            _ => {
                other.compiled = checked(
                    layerx_intents::NativeOwnerBootstrap::Identity {
                        did: identity.did.clone(),
                        primary_key: layerx_types::intent::PublicKey::new([0x51; 32]),
                    }
                    .compile(&registry),
                )
            }
        }
        let signed = if mutation == 1 {
            let (mut call, _) = checked(decode_signed(&signed_identity(&other, 1)));
            call.chain_id = u64::from(other.network_id);
            checked(call.sign(&[0x68; 32]))
        } else {
            signed_identity(&other, 1)
        };
        assert!(identity.verify_signed(&signed).is_err());
        assert!(journey
            .retain_native_identity_submission(&mut scope, &registry, &signed, 1002)
            .is_err());
        assert_eq!(
            checked(journey.native_identity_submission(&scope, &registry)),
            None
        );
    }
    assert_eq!(journey.status().stages[1].1, StageState::Pending);
}

#[test]
fn native_onboarding_identity_cannot_complete_without_its_committed_receipt() {
    let (_fixture, mut store, custody) = Fixture::new("repair-native-identity-receipt");
    let mut scope = checked(store.principal(&principal("alice")));
    let registry = registry();
    let mut journey = checked(CreationJourney::start_native(
        &mut scope,
        &request(),
        &context(),
        &catalog(),
        1000,
        ASSET,
        300,
    ));
    let identity = checked(journey.prepare_native_identity(&mut scope, &custody, &registry, 1001));
    let signed = signed_identity(&identity, 1);
    let (call, signature) = checked(decode_signed(&signed));
    let sender = checked(recover_sender(&call, &signature));
    let disclosure = checked(bind_call(&call, sender));
    let mut authority = [0; 32];
    authority[12..].copy_from_slice(&sender);
    // A real kernel-produced receipt for another transaction must never close this creation.
    let receipt = include_bytes!(
        "../../../../../tests/fixtures/authority/native-sessions/authentication/grant.receipt"
    )
    .to_vec();
    let decoded = checked(layerx_wire::receipt::decode(&receipt));
    let protocol = decoded.protocol().expect("native receipt");
    let mut evidence = ProtocolEvidence {
        signed_activity: signed.clone(),
        actor: disclosure.actor,
        owner_public_key: authority,
        network_id: NETWORK,
        action_key: identity.action_key,
        activity_id: transaction_hash(&signed),
        receipt_hash: Sha256::digest(&receipt).into(),
        tx_hash: transaction_hash(&signed),
        block_height: u64::from_be_bytes(
            protocol.batch_id()[24..]
                .try_into()
                .expect("receipt height"),
        ),
        receipt_bytes: receipt,
        rung: Rung::Receipted,
    };
    let before = journey.status();
    assert!(matches!(
        journey.accept_native_identity(&mut scope, &registry, &evidence, 1002),
        Err(AgentCreationError::EvidenceConflict)
    ));
    checked(journey.retain_native_identity_submission(&mut scope, &registry, &signed, 1002));
    assert!(matches!(
        journey.accept_native_identity(&mut scope, &registry, &evidence, 1003),
        Err(AgentCreationError::Agent(AgentFailure::Unavailable))
    ));
    evidence.rung = Rung::Committed;
    assert!(journey
        .accept_native_identity(&mut scope, &registry, &evidence, 1004)
        .is_err());
    assert_eq!(journey.status(), before);
    assert_eq!(
        checked(CreationJourney::load(&scope, journey.agent_id()))
            .expect("durable journey")
            .status(),
        before
    );
}

#[test]
fn native_onboarding_historical_records_remain_readable_and_are_not_upgraded() {
    let (fixture, mut store, custody) = Fixture::new("repair-native-identity-history");
    let mut scope = checked(store.principal(&principal("alice")));
    let mut journey = checked(CreationJourney::start_native(
        &mut scope,
        &request(),
        &context(),
        &catalog(),
        1000,
        ASSET,
        300,
    ));
    let registry = registry();
    checked(journey.prepare_native_identity(&mut scope, &custody, &registry, 1001));
    let id = journey.agent_id();
    let key = scope
        .keys(Table::Journeys)
        .into_iter()
        .find(|key| key.as_str().starts_with("agent-create-"))
        .expect("creation row");
    let mut historical: serde_json::Value = checked(serde_json::from_slice(
        scope
            .get(Table::Journeys, &key)
            .expect("creation row")
            .bytes(),
    ));
    // Version 2 has the same durable fields and stage/action identifiers.
    historical["version"] = json!(2);
    historical["stages"][1]["state"] = json!("Refused");
    let bytes = checked(serde_json::to_vec(&historical));
    checked(scope.put(Table::Journeys, key.clone(), 1002, bytes.clone()));
    let status = checked(CreationJourney::load(&scope, id))
        .expect("historical native journey")
        .status();
    drop(scope);
    drop(store);
    drop(custody);

    let mut store = fixture.store();
    let custody = fixture.custody();
    let mut scope = checked(store.principal(&principal("alice")));
    let mut recovered = checked(CreationJourney::start_native(
        &mut scope,
        &request(),
        &context(),
        &catalog(),
        1003,
        ASSET,
        300,
    ));
    assert_eq!(recovered.status(), status);
    assert!(matches!(
        recovered.prepare_native_identity(&mut scope, &custody, &registry, 1004),
        Err(AgentCreationError::InvalidStage)
    ));
    assert_eq!(
        scope
            .get(Table::Journeys, &key)
            .expect("historical row")
            .bytes(),
        bytes
    );
    assert_eq!(checked(CreationJourney::list(&scope)), vec![recovered]);
}
