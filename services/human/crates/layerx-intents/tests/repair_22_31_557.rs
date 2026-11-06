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
    let activity = ActivityType::new(ModuleId::Governance, 2).expect("rotation activity");
    let governance = ModuleRegistration::new(ModuleId::Governance, &[activity])
        .expect("governance registration");
    ModuleRegistry::new(&[governance]).expect("rotation registry")
}

fn rotation(did: &Did) -> IntentKind {
    IntentKind::KeyRotation(
        KeyRotation::new(
            did.clone(),
            PublicKey::new([2; 32]),
            TimestampBound::new(10, 20).expect("challenge window"),
            Sequence::from_u64(3),
        )
        .expect("rotation intent"),
    )
}

fn kernel_did_id(did: &Did) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/did-id\0");
    digest.update(
        u16::try_from(did.as_bytes().len())
            .expect("bounded DID length")
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
        .expect("committed rotation vector")
        .1
        .split("\n[")
        .next()
        .expect("rotation section");
    let prefix = format!("{field} = \"");
    let hex = section
        .lines()
        .find_map(|line| line.strip_prefix(&prefix))
        .and_then(|value| value.strip_suffix('"'))
        .expect("committed vector field");
    assert_eq!(hex.len() % 2, 0);
    (0..hex.len())
        .step_by(2)
        .map(|offset| u8::from_str_radix(&hex[offset..offset + 2], 16).expect("vector hex"))
        .collect()
}

#[test]
fn v3_rotation_carries_the_length_prefixed_kernel_did_identifier() {
    let did = Did::new(b"did:layerx:golden").expect("DID");
    let intent = Intent::v3(rotation(&did));
    let compiled = compile(&intent, &registry()).expect("V3 rotation admitted");
    let expected = kernel_did_id(&did);
    assert_eq!(
        expected,
        hash::did_id_for_protocol(&did, 3).expect("V3 DID id")
    );
    assert_ne!(expected, hash::did_id(&did).expect("legacy DID id"));
    assert_eq!(&compiled.payload().as_bytes()[4..36], &expected);
    assert_eq!(compiled.payload().as_bytes(), rotation_bytes(&expected));
    assert_eq!(compiled.activity_type().value(), 0x0007_0002);
}

#[test]
fn v1_and_v2_rotation_bytes_and_hashes_match_the_committed_vectors() {
    let golden_did = Did::new(b"did:layerx:golden").expect("golden DID");
    for (intent, source) in [
        (Intent::v1(rotation(&golden_did)), golden::V1_SOURCE),
        (Intent::v2(rotation(&golden_did)), golden::V2_SOURCE),
    ] {
        let compiled = compile(&intent, &registry()).expect("legacy rotation");
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
            rotation_bytes(&hash::did_id(&golden_did).expect("legacy DID id"))
        );
        let checked = DisclosureCheck::verify(&intent, &compiled).expect("legacy disclosure");
        assert_eq!(checked.canonical_payload(), compiled.payload().as_bytes());
    }

    let did = Did::new(b"did:layerx:human-compiler").expect("compile.rs fixture DID");
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/did-id\0");
    digest.update(did.as_bytes());
    let identifier: [u8; 32] = digest.finalize().into();
    for intent in [Intent::v1(rotation(&did)), Intent::v2(rotation(&did))] {
        let compiled = compile(&intent, &registry()).expect("compile.rs legacy fixture");
        assert_eq!(compiled.payload().as_bytes(), rotation_bytes(&identifier));
    }
}

#[test]
fn v3_rotation_disclosure_reproduces_the_same_bytes_and_refuses_a_legacy_identifier() {
    let did = Did::new(b"did:layerx:golden").expect("DID");
    let intent = Intent::v3(rotation(&did));
    let compiled = compile(&intent, &registry()).expect("V3 rotation");
    let checked = DisclosureCheck::verify(&intent, &compiled).expect("V3 disclosure round trip");
    assert_eq!(checked.activity_type(), compiled.activity_type());
    assert_eq!(checked.canonical_payload(), compiled.payload().as_bytes());
    assert_eq!(checked.payload_hash(), compiled.payload_hash());

    let legacy = compile(&Intent::v1(rotation(&did)), &registry()).expect("legacy rotation");
    assert_eq!(
        DisclosureCheck::verify(&intent, &legacy),
        Err(DisclosureCheckError::FieldMismatch(DisclosureField::Did))
    );
}

#[test]
fn v3_public_key_did_rotation_matches_the_governance_actor_derivation() {
    let owner_key = LocalSigner::new([0x57; 32]).public_key();
    let key_hex: String = owner_key.iter().map(|byte| format!("{byte:02x}")).collect();
    let did = Did::new(format!("did:layerx:{key_hex}").as_bytes()).expect("public-key DID");
    assert_eq!(did.as_bytes().len(), b"did:layerx:".len() + 64);
    let intent = Intent::v3(rotation(&did));
    let compiled = compile(&intent, &registry()).expect("public-key DID V3 rotation");
    let actor = kernel_did_id(&did);
    assert_eq!(actor, hash::did_id_for_protocol(&did, 3).expect("V3 actor"));
    assert_eq!(&compiled.payload().as_bytes()[4..36], &actor);
    assert_ne!(actor, hash::did_id(&did).expect("legacy actor"));
    let checked = DisclosureCheck::verify(&intent, &compiled).expect("public-key DID disclosure");
    assert_eq!(checked.canonical_payload(), rotation_bytes(&actor));
}
