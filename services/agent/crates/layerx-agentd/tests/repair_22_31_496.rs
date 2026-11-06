use ed25519_dalek::SigningKey;
use layerx_agentd::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
use layerx_crypto::disclosure::call::{bind_call, ledger_selector, ConstructMethod};
use layerx_crypto::disclosure::DisclosureError;
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_intents::{compile, CompiledIntent, Intent, IntentKind, KeyRotation};
use layerx_types::activity::TimestampBound;
use layerx_types::ids::Did;
use layerx_types::intent::{PublicKey, Sequence};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use std::io::Write as _;

fn compiled_rotation() -> CompiledIntent {
    let owner = SigningKey::from_bytes(&[74; 32]).verifying_key().to_bytes();
    let pending = SigningKey::from_bytes(&[75; 32]).verifying_key().to_bytes();
    let did = Did::new(format!("did:layerx:{}", hex::encode(owner)).as_bytes()).unwrap();
    assert_ne!(owner, pending);
    let intent = Intent::v3(IntentKind::KeyRotation(
        KeyRotation::new(
            did,
            PublicKey::new(pending),
            TimestampBound::new(1_800_000_001_000, 1_800_000_002_000).unwrap(),
            Sequence::from_u64(100),
        )
        .unwrap(),
    ));
    let kind = ActivityType::new(ModuleId::Governance, 2).unwrap();
    let registry =
        ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Governance, &[kind]).unwrap()])
            .unwrap();
    let compiled = compile(&intent, &registry).unwrap();
    compiled.verify_disclosure(&intent).unwrap();
    compiled
}

fn lifecycle_call(payload: Vec<u8>, idempotency_key: [u8; 32]) -> Eip1559Call {
    Eip1559Call {
        chain_id: 125,
        nonce: 3,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 100_000,
        to: PrecompileTarget::Auth.address(),
        data: PrecompileCall {
            target: PrecompileTarget::Auth,
            selector: ledger_selector("recordLifecycle(bytes,bytes32)"),
            arguments: vec![
                AbiArgument::Bytes(payload),
                AbiArgument::Word(idempotency_key),
            ],
        }
        .encode()
        .unwrap(),
    }
}

#[test]
fn compiled_key_rotation_binds_exact_payload_and_idempotency_key() {
    let compiled = compiled_rotation();
    let payload = compiled.payload().as_bytes();
    assert_eq!(compiled.activity_type().value(), 0x0007_0002);
    assert_eq!(payload.len(), 92);
    assert_eq!(&payload[..4], &[0x71, 2, 0, 4]);

    // hf-done records the real compiler output and checks the keeper's pinned copy.
    let encoded = hex::encode(payload);
    writeln!(
        std::io::stdout().lock(),
        "compiled_key_rotation_hex={encoded}"
    )
    .unwrap();
    let keeper_test = include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../modules/layerxauth/keeper/repair_22_31_496_test.go"
    ));
    assert!(keeper_test.contains(&format!("const repair2231496RotationHex = \"{encoded}\"")));

    for key in [[0; 32], [0x49; 32]] {
        let call = lifecycle_call(payload.to_vec(), key);
        let disclosure = bind_call(&call, [0x26; 20]).unwrap();
        assert_eq!(disclosure.activity_type, compiled.activity_type());
        assert_eq!(disclosure.idempotency_key, key);
        assert_eq!(
            disclosure.construct_method().unwrap(),
            Some(ConstructMethod::AuthLifecycle {
                payload: payload.to_vec(),
                idempotency_key: key,
            })
        );
    }
}

#[test]
fn key_rotation_wrong_lengths_and_headers_are_malformed() {
    let compiled = compiled_rotation();
    let payload = compiled.payload().as_bytes();
    for length in [4, 68, 70, 86, 91, 93, 1024, 1025] {
        let mut malformed = payload.to_vec();
        malformed.resize(length, 0);
        assert_eq!(
            bind_call(&lifecycle_call(malformed, [0x49; 32]), [0x26; 20]),
            Err(DisclosureError::MalformedPayload),
            "rotation payload length {length}"
        );
    }
    for (index, value) in [(0, 0x70), (1, 4), (2, 1), (2, 2), (3, 3), (3, 5)] {
        let mut malformed = payload.to_vec();
        malformed[index] = value;
        assert_eq!(
            bind_call(&lifecycle_call(malformed, [0x49; 32]), [0x26; 20]),
            Err(DisclosureError::MalformedPayload),
            "rotation header byte {index} = {value}"
        );
    }
}
