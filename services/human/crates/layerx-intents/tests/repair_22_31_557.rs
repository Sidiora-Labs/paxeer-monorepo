use layerx_crypto::signer::{LocalSigner, Signer};
use layerx_intents::{
    compile, golden, DisclosureCheck, DisclosureCheckError, DisclosureField, Intent, IntentKind,
    KeyRotation,
};
use layerx_types::activity::TimestampBound;
use layerx_types::ids::Did;
use layerx_types::intent::{PublicKey, Sequence};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_wire::hash;
use sha2::{Digest, Sha256};

fn registry() -> ModuleRegistry {
    let activity = ActivityType::new(ModuleId::Governance, 2)
        .unwrap_or_else(|error| panic!("rotation activity: {error:?}"));
    let governance = ModuleRegistration::new(ModuleId::Governance, &[activity])
        .unwrap_or_else(|error| panic!("governance registration: {error:?}"));
    ModuleRegistry::new(&[governance])
        .unwrap_or_else(|error| panic!("rotation registry: {error:?}"))
}

fn rotation(did: &Did) -> IntentKind {
    IntentKind::KeyRotation(
        KeyRotation::new(
            did.clone(),
            PublicKey::new([2; 32]),
            TimestampBound::new(10, 20)
                .unwrap_or_else(|error| panic!("challenge window: {error:?}")),
            Sequence::from_u64(3),
        )
        .unwrap_or_else(|error| panic!("rotation intent: {error:?}")),
    )
}

fn kernel_did_id(did: &Did) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/did-id\0");
    digest.update(
        u16::try_from(did.as_bytes().len())
            .unwrap_or_else(|error| panic!("bounded DID length: {error:?}"))
            .to_be_bytes(),
    );
    digest.update(did.as_bytes());
    digest.finalize().into()
}

fn rotation_bytes(identifier: &[u8; 32]) -> Vec<u8> {
    let mut bytes = vec![0x71, 0x02, 0x00, 0x04];
    bytes.extend_from_slice(identifier);
    bytes.extend_from_slice(&[2; 32]);
    bytes.extend_from_slice(&10_u64.to_be_bytes());
    bytes.extend_from_slice(&20_u64.to_be_bytes());
    bytes.extend_from_slice(&3_u64.to_be_bytes());
    bytes
}

fn vector_bytes(source: &str, field: &str) -> Vec<u8> {
    let section = source
        .split_once("[vector.key_rotation]\n")
        .unwrap_or_else(|| panic!("committed rotation vector"))
        .1
        .split("\n[")
        .next()
        .unwrap_or_else(|| panic!("rotation section"));
    let prefix = format!("{field} = \"");
    let hex = section
        .lines()
        .find_map(|line| line.strip_prefix(&prefix))
        .and_then(|value| value.strip_suffix('"'))
        .unwrap_or_else(|| panic!("committed vector field"));
    assert_eq!(hex.len() % 2, 0);
    (0..hex.len())
        .step_by(2)
        .map(|offset| {
            u8::from_str_radix(&hex[offset..offset + 2], 16)
                .unwrap_or_else(|error| panic!("vector hex: {error:?}"))
        })
        .collect()
}

#[test]
fn v3_rotation_carries_the_length_prefixed_kernel_did_identifier() {
    let did = Did::new(b"did:layerx:golden").unwrap_or_else(|error| panic!("DID: {error:?}"));
    let intent = Intent::v3(rotation(&did));
    let compiled = compile(&intent, &registry())
        .unwrap_or_else(|error| panic!("V3 rotation admitted: {error:?}"));
    let expected = kernel_did_id(&did);
    assert_eq!(
        expected,
        hash::did_id_for_protocol(&did, 3).unwrap_or_else(|error| panic!("V3 DID id: {error:?}"))
    );
    assert_ne!(
        expected,
        hash::did_id(&did).unwrap_or_else(|error| panic!("legacy DID id: {error:?}"))
    );
    assert_eq!(&compiled.payload().as_bytes()[4..36], &expected);
    assert_eq!(compiled.payload().as_bytes(), rotation_bytes(&expected));
    assert_eq!(compiled.activity_type().value(), 0x0007_0002);
}

#[test]
fn v1_and_v2_rotation_bytes_and_hashes_match_the_committed_vectors() {
    let golden_did =
        Did::new(b"did:layerx:golden").unwrap_or_else(|error| panic!("golden DID: {error:?}"));
    for (intent, source) in [
        (Intent::v1(rotation(&golden_did)), golden::V1_SOURCE),
        (Intent::v2(rotation(&golden_did)), golden::V2_SOURCE),
    ] {
        let compiled = compile(&intent, &registry())
            .unwrap_or_else(|error| panic!("legacy rotation: {error:?}"));
        assert_eq!(
            compiled.payload().as_bytes(),
            vector_bytes(source, "payload_hex")
        );
        assert_eq!(
            compiled.payload_hash().as_slice(),
            vector_bytes(source, "payload_hash_hex")
        );
        assert_eq!(
            compiled.payload().as_bytes(),
            rotation_bytes(
                &hash::did_id(&golden_did)
                    .unwrap_or_else(|error| panic!("legacy DID id: {error:?}"))
            )
        );
        let checked = DisclosureCheck::verify(&intent, &compiled)
            .unwrap_or_else(|error| panic!("legacy disclosure: {error:?}"));
        assert_eq!(checked.canonical_payload(), compiled.payload().as_bytes());
    }

    let did = Did::new(b"did:layerx:human-compiler")
        .unwrap_or_else(|error| panic!("compile.rs fixture DID: {error:?}"));
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/did-id\0");
    digest.update(did.as_bytes());
    let identifier: [u8; 32] = digest.finalize().into();
    for intent in [Intent::v1(rotation(&did)), Intent::v2(rotation(&did))] {
        let compiled = compile(&intent, &registry())
            .unwrap_or_else(|error| panic!("compile.rs legacy fixture: {error:?}"));
        assert_eq!(compiled.payload().as_bytes(), rotation_bytes(&identifier));
    }
}

#[test]
fn v3_rotation_disclosure_reproduces_the_same_bytes_and_refuses_a_legacy_identifier() {
    let did = Did::new(b"did:layerx:golden").unwrap_or_else(|error| panic!("DID: {error:?}"));
    let intent = Intent::v3(rotation(&did));
    let compiled =
        compile(&intent, &registry()).unwrap_or_else(|error| panic!("V3 rotation: {error:?}"));
    let checked = DisclosureCheck::verify(&intent, &compiled)
        .unwrap_or_else(|error| panic!("V3 disclosure round trip: {error:?}"));
    assert_eq!(checked.activity_type(), compiled.activity_type());
    assert_eq!(checked.canonical_payload(), compiled.payload().as_bytes());
    assert_eq!(checked.payload_hash(), compiled.payload_hash());

    let legacy = compile(&Intent::v1(rotation(&did)), &registry())
        .unwrap_or_else(|error| panic!("legacy rotation: {error:?}"));
    assert_eq!(
        DisclosureCheck::verify(&intent, &legacy),
        Err(DisclosureCheckError::FieldMismatch(DisclosureField::Did))
    );
}

#[test]
fn v3_public_key_did_rotation_matches_the_governance_actor_derivation() {
    use std::fmt::Write as _;

    let owner_key = LocalSigner::new([0x57; 32]).public_key();
    let mut key_hex = String::with_capacity(owner_key.len() * 2);
    for byte in owner_key {
        write!(&mut key_hex, "{byte:02x}").unwrap_or_else(|error| panic!("key hex: {error:?}"));
    }
    let did = Did::new(format!("did:layerx:{key_hex}").as_bytes())
        .unwrap_or_else(|error| panic!("public-key DID: {error:?}"));
    assert_eq!(did.as_bytes().len(), b"did:layerx:".len() + 64);
    let intent = Intent::v3(rotation(&did));
    let compiled = compile(&intent, &registry())
        .unwrap_or_else(|error| panic!("public-key DID V3 rotation: {error:?}"));
    let actor = kernel_did_id(&did);
    assert_eq!(
        actor,
        hash::did_id_for_protocol(&did, 3).unwrap_or_else(|error| panic!("V3 actor: {error:?}"))
    );
    assert_eq!(&compiled.payload().as_bytes()[4..36], &actor);
    assert_ne!(
        actor,
        hash::did_id(&did).unwrap_or_else(|error| panic!("legacy actor: {error:?}"))
    );
    let checked = DisclosureCheck::verify(&intent, &compiled)
        .unwrap_or_else(|error| panic!("public-key DID disclosure: {error:?}"));
    assert_eq!(checked.canonical_payload(), rotation_bytes(&actor));
}
