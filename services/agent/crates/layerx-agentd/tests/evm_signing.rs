#[path = "support/send_authorization.rs"]
mod send_authorization;

use std::future::Future;
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};

use k256::ecdsa::SigningKey;
use layerx_agentd::prepare::{restore_canonical, EvmPrepared};
use layerx_agentd::sign::SigningPreparation;
use layerx_agentd::sign::{self_sign, ProvisionedSessionKey, SigningError, SigningMode};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::secp256k1::evm_address;
use layerx_crypto::session::{issue_session_key, IssuedSessionKey, SessionKeyRequest};
use layerx_crypto::signer::Signer;
use layerx_types::activity::{Authority, EnvelopeBuilder, TimestampBound};
use layerx_types::amount::Amount;
use layerx_types::ids::{Did, IdempotencyKey};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_wire::encode::Encoder;

struct NoopWake;

impl Wake for NoopWake {
    fn wake(self: Arc<Self>) {}
}

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(NoopWake));
    let mut context = Context::from_waker(&waker);
    match future.as_mut().poll(&mut context) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("local signing unexpectedly blocked"),
    }
}

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

fn session_public_key(seed: [u8; 32]) -> [u8; 32] {
    LocalSigner::new(seed).public_key()
}

fn issued_with_grantor(seed: [u8; 32], grantor: [u8; 32]) -> IssuedSessionKey {
    issue_session_key(&SessionKeyRequest {
        grantor,
        session_public_key: session_public_key(seed),
        not_before: 900,
        expires_at: Some(1_100),
        permitted_activity_types: vec![send_type()],
        revocation_sequence: Some(5),
        fee_budget: None,
        purpose: layerx_crypto::session::SessionPurpose::Activity,
    })
    .unwrap()
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
fn self_signed_transaction_recovers_to_the_session_address() {
    let seed = [0xa5; 32];
    let issued = issued(seed, send_type(), 1_100);
    let prepared = prepared(issued.authority.clone());
    let session = ProvisionedSessionKey::new(seed, issued).unwrap();
    let signed = ready(self_sign(Some(&session), &prepared, &registry(), 1_000, 5)).unwrap();
    assert_eq!(signed.signature.len(), 65);
    let (call, signature) = decode_signed(&signed.signed_canonical_bytes).unwrap();
    assert_eq!(call, prepared.transaction.call);
    assert_eq!(signature.as_bytes(), &signed.signature);
    let expected = session_address(seed);
    assert_eq!(recover_sender(&call, &signature).unwrap(), expected);
    assert_eq!(signed.signer_address, expected);
    assert_eq!(expected, prepared.actor_address);
    assert_eq!(signed.mode, SigningMode::ProtocolSessionKey);
    let verified = layerx_agentd::sign::verify_before_submit(
        &signed.signed_canonical_bytes,
        &prepared,
        &expected,
        &registry(),
    )
    .unwrap();
    assert_eq!(verified.exact_bytes(), signed.signed_canonical_bytes);
    assert_eq!(signed.audit.session_public_key, session_public_key(seed));
    assert_eq!(signed.audit.revocation_sequence, 5);
    assert_eq!(signed.audit.expires_at, 1_100);
    assert_eq!(
        ready(self_sign(Some(&session), &prepared, &registry(), 1_000, 6)),
        Err(SigningError::Revoked),
    );
    let mut changed_hash = prepared.clone();
    changed_hash.transaction.signing_hash[0] ^= 1;
    assert_eq!(
        ready(self_sign(
            Some(&session),
            &changed_hash,
            &registry(),
            1_000,
            5
        )),
        Err(SigningError::PreparedBytesChanged),
    );
    let mismatched_authority =
        ProvisionedSessionKey::new(seed, issued_with_grantor(seed, [2; 32])).unwrap();
    assert_eq!(
        ready(self_sign(
            Some(&mismatched_authority),
            &prepared,
            &registry(),
            1_000,
            5
        )),
        Err(SigningError::AuthorityMismatch),
    );
}

#[test]
fn external_signature_from_another_key_is_refused() {
    let prepared = prepared(issued([0xa5; 32], send_type(), 1_100).authority);
    let foreign_raw = prepared.transaction.call.sign(&[0xa6; 32]).unwrap();
    let (call, signature) = decode_signed(&foreign_raw).unwrap();
    assert_eq!(signature.as_bytes().len(), 65);
    assert_eq!(
        recover_sender(&call, &signature).unwrap(),
        session_address([0xa6; 32])
    );
    assert_ne!(session_address([0xa6; 32]), prepared.actor_address);
    assert_eq!(
        layerx_agentd::sign::attach_external_signature(&prepared, *signature.as_bytes()),
        Err(SigningError::SignerMismatch),
    );
    assert_eq!(
        layerx_agentd::sign::attach_external_signature(&prepared, [0; 65]),
        Err(SigningError::SignatureInvalid),
    );
    let valid_raw = prepared.transaction.call.sign(&[0xa5; 32]).unwrap();
    let (_, valid_signature) = decode_signed(&valid_raw).unwrap();
    assert_eq!(
        layerx_agentd::sign::verify_before_submit(
            &valid_raw,
            &prepared,
            &session_address([0xa6; 32]),
            &registry(),
        ),
        Err(SigningError::SignerMismatch),
    );
    let mut trailing_bytes = valid_raw.clone();
    trailing_bytes.push(0);
    assert_eq!(
        layerx_agentd::sign::verify_before_submit(
            &trailing_bytes,
            &prepared,
            &prepared.actor_address,
            &registry(),
        ),
        Err(SigningError::SignatureInvalid),
    );

    assert_eq!(
        layerx_agentd::sign::attach_external_signature(&prepared, *valid_signature.as_bytes())
            .unwrap(),
        valid_raw,
    );
    assert_eq!(
        layerx_agentd::sign::verify_before_submit(
            &foreign_raw,
            &prepared,
            &prepared.actor_address,
            &registry(),
        ),
        Err(SigningError::SignerMismatch),
    );
}

#[test]
fn verify_before_submit_rejects_mutated_calldata() {
    let prepared = prepared(issued([0xa5; 32], send_type(), 1_100).authority);
    let mut changed = prepared.transaction.call.clone();
    changed.data[0] ^= 1;
    // Sign the changed call with the correct key, so rejection proves call binding.
    let mutated_raw = changed.sign(&[0xa5; 32]).unwrap();
    let (decoded, signature) = decode_signed(&mutated_raw).unwrap();
    assert_ne!(decoded.data, prepared.transaction.call.data);
    assert_eq!(
        recover_sender(&decoded, &signature).unwrap(),
        prepared.actor_address
    );
    assert_eq!(
        layerx_agentd::sign::verify_before_submit(
            &mutated_raw,
            &prepared,
            &prepared.actor_address,
            &registry(),
        ),
        Err(SigningError::PreparedBytesChanged),
    );
}
