#[path = "support/send_authorization.rs"]
mod send_authorization;

use k256::ecdsa::SigningKey;
use layerx_agentd::sign::SigningError;
use layerx_agentd::sign::SigningPreparation;
use layerx_crypto::evm_transaction::decode_signed;
use layerx_crypto::secp256k1::evm_address;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_wire::encode::Encoder;

fn send_type() -> ActivityType {
    ActivityType::new(ModuleId::Asset, 5).unwrap_or_else(|error| panic!("activity: {error:?}"))
}

fn registry() -> ModuleRegistry {
    ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Asset, &[send_type()])
        .unwrap_or_else(|error| panic!("registration: {error:?}"))])
    .unwrap_or_else(|error| panic!("registry: {error:?}"))
}

fn send_payload() -> Vec<u8> {
    let mut encoder = Encoder::new(512);
    for result in [encoder.u16(0x5301), encoder.u16(10)] {
        result.unwrap_or_else(|error| panic!("header: {error:?}"));
    }
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

fn session_address(seed: [u8; 32]) -> [u8; 20] {
    let key = SigningKey::from_slice(&seed).unwrap_or_else(|error| panic!("key: {error:?}"));
    evm_address(key.verifying_key().to_encoded_point(false).as_bytes())
        .unwrap_or_else(|error| panic!("address: {error:?}"))
}

fn prepared() -> SigningPreparation {
    let actor_address = session_address([0xa5; 32]);
    let transaction = layerx_agentd::prepare::prepare_bound(
        layerx_agentd::prepare::EvmPreparationState {
            chain_id: 17,
            pending_nonce: 5,
            base_fee_per_gas: 3,
            head_height: 88,
        },
        layerx_agentd::prepare::EvmPreparationDefaults {
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 7,
            maximum_payload_bytes: 1_024,
        },
        layerx_agentd::prepare::EvmPrepareRequest {
            from: actor_address,
            call: layerx_agentd::prepare::PrecompileCall::ledger_transfer(send_payload()),
            expected_nonce: Some(5),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 1_024,
        },
        100_000,
    )
    .unwrap_or_else(|error| panic!("test operation failed: {error:?}"));
    SigningPreparation {
        transaction,
        actor_address,
    }
}

#[test]
fn verified_wrapper_preserves_the_exact_signed_input_and_activity_id() {
    let prepared = prepared();
    let signed_bytes = prepared
        .transaction
        .call
        .sign(&[0xa5; 32])
        .unwrap_or_else(|error| panic!("test operation failed: {error:?}"));
    let (_, signature) = decode_signed(&signed_bytes)
        .unwrap_or_else(|error| panic!("test operation failed: {error:?}"));
    let attached = layerx_agentd::sign::attach_external_signature(&prepared, *signature.as_bytes())
        .unwrap_or_else(|error| panic!("test operation failed: {error:?}"));
    assert_eq!(attached, signed_bytes);
    let verified = layerx_agentd::sign::verify_before_submit(
        &attached,
        &prepared,
        &session_address([0xa5; 32]),
        &registry(),
    )
    .unwrap_or_else(|error| panic!("test operation failed: {error:?}"));
    assert_eq!(verified.exact_bytes(), signed_bytes);
    assert_eq!(verified.signed_byte_length(), signed_bytes.len());
    assert_ne!(verified.activity_id(), [0; 32]);
    assert_eq!(
        verified.activity_id(),
        layerx_crypto::evm_transaction::transaction_hash(&signed_bytes)
    );
    assert_eq!(
        verified.idempotency_key(),
        prepared.transaction.disclosure.idempotency_key
    );
}

#[test]
fn amount_recipient_and_fee_alterations_cannot_ride_an_old_signature() {
    let original = prepared();
    let signed_bytes = original
        .transaction
        .call
        .sign(&[0xa5; 32])
        .unwrap_or_else(|error| panic!("test operation failed: {error:?}"));
    let (_, signature) = decode_signed(&signed_bytes)
        .unwrap_or_else(|error| panic!("test operation failed: {error:?}"));
    // Payload fields contain the 32-byte sender, recipient and asset, then the amount.
    let mut amount = original.clone();
    amount.transaction.call.data[68 + 4 + 32 * 3 + 15] ^= 1;
    let mut recipient = original.clone();
    recipient.transaction.call.data[68 + 4 + 32] ^= 1;
    let mut fee = original.clone();
    fee.transaction.call.max_fee_per_gas += 1;
    for mut altered in [amount, recipient, fee] {
        altered.transaction.signing_hash = altered.transaction.call.signing_hash();
        assert_eq!(
            layerx_agentd::sign::attach_external_signature(&altered, *signature.as_bytes()),
            Err(SigningError::SignerMismatch),
        );
        assert_eq!(
            layerx_agentd::sign::verify_before_submit(
                &signed_bytes,
                &altered,
                &session_address([0xa5; 32]),
                &registry(),
            ),
            Err(SigningError::PreparedBytesChanged),
        );
    }
}
