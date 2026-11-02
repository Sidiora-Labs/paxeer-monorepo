use layerx_crypto::disclosure::{bind, DisclosureError};
use layerx_crypto::signer::{LocalSigner, Signer as _};
use layerx_intents::NativeOwnerBootstrap;
use layerx_types::activity::{Authority, EnvelopeBuilder, TimestampBound};
use layerx_types::amount::Amount;
use layerx_types::ids::{Did, IdempotencyKey};
use layerx_types::intent::{ApprovalThreshold, PublicKey, RecoveryRoot};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_wire::activity::encode_unsigned_envelope;

fn registry(ordinals: &[u16]) -> ModuleRegistry {
    let activities: Vec<_> = ordinals
        .iter()
        .map(|ordinal| {
            ActivityType::new(ModuleId::Governance, *ordinal)
                .unwrap_or_else(|error| panic!("native Governance ordinal: {error:?}"))
        })
        .collect();
    ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Governance, &activities)
        .unwrap_or_else(|error| panic!("native Governance registration: {error:?}"))])
    .unwrap_or_else(|error| panic!("native module registry: {error:?}"))
}

#[test]
fn native_identity_disclosure_refuses_foreign_subject_and_signing_key() -> Result<(), String> {
    let checked = |error| format!("owner DID: {error:?}");
    let owner = Did::new(b"did:layerx:owner-bootstrap").map_err(checked)?;
    let foreign = Did::new(b"did:layerx:foreign-owner").map_err(checked)?;
    let public = LocalSigner::new([71; 32]).public_key();
    let registry = registry(&[1]);
    let canonical = |compiled: &layerx_intents::CompiledIntent| -> Result<Vec<u8>, String> {
        let mut builder = EnvelopeBuilder::new();
        builder
            .protocol_version(3)
            .and_then(|builder| builder.network_id(77))
            .and_then(|builder| builder.activity_type(compiled.activity_type()))
            .and_then(|builder| builder.actor_did(owner.clone()))
            .and_then(|builder| builder.authority(Authority::owner(&public)?))
            .and_then(|builder| builder.account_sequence(10))
            .and_then(|builder| builder.timestamp_bound(TimestampBound::new(1_000, 61_000)?))
            .and_then(|builder| builder.idempotency_key(IdempotencyKey::new([1; 32])))
            .and_then(|builder| builder.fee_limit(Amount::from_u128(100)))
            .and_then(|builder| builder.payload_hash(compiled.payload_hash()))
            .and_then(|builder| builder.payload(compiled.payload().clone()))
            .map_err(|error| format!("identity disclosure envelope: {error:?}"))?;
        let envelope = builder
            .build()
            .map_err(|error| format!("envelope: {error:?}"))?;
        encode_unsigned_envelope(&envelope).map_err(|error| format!("canonical bytes: {error:?}"))
    };
    let identity = NativeOwnerBootstrap::Identity {
        did: owner.clone(),
        primary_key: PublicKey::new(public),
    }
    .compile(&registry)
    .map_err(|error| format!("native identity: {error:?}"))?;
    let unsigned = canonical(&identity)?;
    let disclosure =
        bind(&unsigned, &registry).map_err(|error| format!("disclosure: {error:?}"))?;
    assert_eq!(
        disclosure
            .reencode()
            .map_err(|error| format!("reencode: {error:?}"))?,
        unsigned
    );
    for (did, primary_key) in [
        (foreign, public),
        (owner.clone(), LocalSigner::new([72; 32]).public_key()),
    ] {
        let compiled = NativeOwnerBootstrap::Identity {
            did,
            primary_key: PublicKey::new(primary_key),
        }
        .compile(&registry)
        .map_err(|error| format!("native identity: {error:?}"))?;
        assert!(matches!(
            bind(&canonical(&compiled)?, &registry),
            Err(DisclosureError::MalformedPayload)
        ));
    }
    Ok(())
}

#[test]
fn native_bootstrap_refuses_missing_module_and_invalid_authority_policy() {
    let did =
        Did::new(b"did:layerx:owner-bootstrap").unwrap_or_else(|error| panic!("DID: {error:?}"));
    let key = PublicKey::new(LocalSigner::new([71; 32]).public_key());
    let identity = NativeOwnerBootstrap::Identity {
        did: did.clone(),
        primary_key: key,
    };
    assert!(identity.compile(&registry(&[2, 3])).is_err());
    assert!(NativeOwnerBootstrap::Identity {
        did: did.clone(),
        primary_key: PublicKey::new([0; 32])
    }
    .compile(&registry(&[1, 2, 3]))
    .is_err());
    assert!(NativeOwnerBootstrap::RotationPolicy {
        did: did.clone(),
        pending_key: key,
        challenge: TimestampBound::new(1, 2).unwrap_or_else(|error| panic!("challenge: {error:?}")),
        effective_sequence: 0
    }
    .compile(&registry(&[1, 2, 3]))
    .is_err());
    for (root, minimum_delay, maximum_delay) in
        [([0; 32], 30, 60), ([19; 32], 0, 60), ([19; 32], 60, 30)]
    {
        assert!(NativeOwnerBootstrap::RecoveryPolicy {
            did: did.clone(),
            root: RecoveryRoot::new(root),
            threshold: ApprovalThreshold::new(2)
                .unwrap_or_else(|error| panic!("threshold: {error:?}")),
            minimum_delay,
            maximum_delay
        }
        .compile(&registry(&[1, 2, 3]))
        .is_err());
    }
}
