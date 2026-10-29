#[path = "support/send_authorization.rs"]
mod send_authorization;

use k256::ecdsa::SigningKey;
use layerx_agentd::prepare::{restore_canonical, EvmPrepared};
use layerx_agentd::sign::SigningError;
use layerx_agentd::sign::SigningPreparation;
use layerx_crypto::evm_transaction::{decode_signed, Eip1559Call};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::secp256k1::evm_address;
use layerx_crypto::session::{issue_session_key, IssuedSessionKey, SessionKeyRequest};
use layerx_crypto::signer::Signer;
use layerx_types::activity::{Authority, EnvelopeBuilder, TimestampBound};
use layerx_types::amount::Amount;
use layerx_types::ids::{Did, IdempotencyKey};
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

fn issued(seed: [u8; 32], permitted: ActivityType, expires_at: u64) -> IssuedSessionKey {
    let public_key = LocalSigner::new(seed).public_key();
    issue_session_key(&SessionKeyRequest {
        fee_budget: None,
        purpose: layerx_crypto::session::SessionPurpose::Activity,
        grantor: [1; 32],
        session_public_key: public_key,
        not_before: 900,
        expires_at: Some(expires_at),
        permitted_activity_types: vec![permitted],
        revocation_sequence: Some(5),
    })
    .unwrap_or_else(|error| panic!("issue session key: {error:?}"))
}

fn session_address(seed: [u8; 32]) -> [u8; 20] {
    let key = SigningKey::from_slice(&seed).unwrap_or_else(|error| panic!("key: {error:?}"));
    evm_address(key.verifying_key().to_encoded_point(false).as_bytes())
        .unwrap_or_else(|error| panic!("address: {error:?}"))
}

fn prepared(authority: Authority) -> SigningPreparation {
    let payload = layerx_types::payload::Payload::new(&registry(), send_type(), &send_payload())
        .unwrap_or_else(|error| panic!("payload: {error:?}"));
    let payload_hash = layerx_wire::hash::payload_hash_for(&payload)
        .unwrap_or_else(|error| panic!("payload hash: {error:?}"));
    let mut builder = EnvelopeBuilder::new();
    builder
        .protocol_version(layerx_wire::limits::PROTOCOL_VERSION)
        .and_then(|builder| builder.network_id(17))
        .and_then(|builder| builder.activity_type(send_type()))
        .and_then(|builder| builder.actor_did(Did::new(b"did:layerx:prepare-agent").unwrap()))
        .and_then(|builder| builder.authority(authority.clone()))
        .and_then(|builder| builder.account_sequence(5))
        .and_then(|builder| builder.timestamp_bound(TimestampBound::new(995, 1_010).unwrap()))
        .and_then(|builder| builder.idempotency_key(IdempotencyKey::new([4; 32])))
        .and_then(|builder| builder.fee_limit(Amount::from_u128(7)))
        .and_then(|builder| builder.payload_hash(payload_hash))
        .and_then(|builder| builder.payload(payload))
        .unwrap_or_else(|error| panic!("envelope: {error:?}"));
    let envelope = builder
        .build()
        .unwrap_or_else(|error| panic!("envelope: {error:?}"));
    let canonical = layerx_wire::activity::encode_unsigned_envelope(&envelope)
        .unwrap_or_else(|error| panic!("encode: {error:?}"));
    let authorization = restore_canonical(&canonical, 88, &registry(), Some(&authority))
        .unwrap_or_else(|error| panic!("restore: {error:?}"));
    let call = Eip1559Call {
        chain_id: 17,
        nonce: 5,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 7,
        gas_limit: 100_000,
        to: [0x1a; 20],
        data: send_payload(),
    };
    SigningPreparation {
        transaction: EvmPrepared {
            signing_hash: call.signing_hash(),
            call,
            observed_head_height: 88,
        },
        actor_address: session_address([0xa5; 32]),
        authorization,
    }
}

#[test]
fn verified_wrapper_preserves_the_exact_signed_input_and_activity_id() {
    let prepared = prepared(issued([0xa5; 32], send_type(), 1_100).authority);
    let signed_bytes = prepared.transaction.call.sign(&[0xa5; 32]).unwrap();
    let (_, signature) = decode_signed(&signed_bytes).unwrap();
    let attached =
        layerx_agentd::sign::attach_external_signature(&prepared, *signature.as_bytes()).unwrap();
    assert_eq!(attached, signed_bytes);
    let verified = layerx_agentd::sign::verify_before_submit(
        &attached,
        &prepared,
        &session_address([0xa5; 32]),
        &registry(),
    )
    .unwrap();
    assert_eq!(verified.exact_bytes(), signed_bytes);
    assert_eq!(verified.signed_byte_length(), signed_bytes.len());
    assert_ne!(verified.activity_id(), [0; 32]);
    assert_eq!(
        verified.activity_id(),
        layerx_crypto::evm_transaction::transaction_hash(&signed_bytes)
    );
    assert_eq!(
        verified.idempotency_key(),
        prepared.authorization.audit.idempotency_key
    );
}

#[test]
fn amount_recipient_and_fee_alterations_cannot_ride_an_old_signature() {
    let original = prepared(issued([0xa5; 32], send_type(), 1_100).authority);
    let signed_bytes = original.transaction.call.sign(&[0xa5; 32]).unwrap();
    let (_, signature) = decode_signed(&signed_bytes).unwrap();
    // Payload fields contain the 32-byte sender, recipient and asset, then the amount.
    let mut amount = original.clone();
    amount.transaction.call.data[4 + 32 * 3 + 15] ^= 1;
    let mut recipient = original.clone();
    recipient.transaction.call.data[4 + 32] ^= 1;
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
