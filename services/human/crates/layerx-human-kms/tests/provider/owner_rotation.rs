use super::{
    checked, construct_signing_call, encoded_disclosure, evm_signing_request, facts, registry,
    request, Host, Result,
};
use layerx_crypto::disclosure::{bind, DisclosedNativeOperation};
use layerx_crypto::rotation::{OwnerRotation, OwnerRotationConsent};
use layerx_intents::canonical as hash;
use layerx_intents::canonical::{
    signed_envelope_bytes as encode_signed_envelope,
    unsigned_envelope_bytes as encode_unsigned_envelope,
};
use layerx_types::activity::{Authority, EnvelopeBuilder, Signature, TimestampBound};
use layerx_types::amount::Amount;
use layerx_types::ids::{Did, IdempotencyKey};
use layerx_types::payload::{ActivityType, ModuleId, Payload};

struct Key {
    binding: [u8; 32],
    handle: [u8; 32],
    public: [u8; 32],
}

impl Key {
    fn create(host: &Host, binding: u8) -> Result<Self> {
        let binding = [binding; 32];
        let (handle, public) = facts(&host.call(&request(1, binding, &[], None)?)?)?;
        Ok(Self {
            binding,
            handle,
            public,
        })
    }
}

#[derive(Clone, Copy)]
struct Envelope {
    sequence: u64,
    action: [u8; 32],
    begin: u64,
    end: u64,
    fee: u128,
}

fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut encoded = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        encoded.push(char::from(DIGITS[usize::from(byte >> 4)]));
        encoded.push(char::from(DIGITS[usize::from(byte & 15)]));
    }
    encoded
}

fn did(key: &Key) -> Result<Did> {
    checked(Did::new(
        format!("did:layerx:{}", hex(&key.public)).as_bytes(),
    ))
}

fn payload(ordinal: u16, value: &[u8]) -> Result<Payload> {
    checked(Payload::new(
        &registry()?,
        checked(ActivityType::new(ModuleId::Governance, ordinal))?,
        value,
    ))
}

fn sign(
    host: &Host,
    key: &Key,
    owner: &Did,
    payload: Payload,
    values: Envelope,
) -> Result<Vec<u8>> {
    let mut builder = EnvelopeBuilder::new();
    checked(builder.protocol_version(3))?;
    checked(builder.network_id(125))?;
    checked(builder.activity_type(payload.activity_type()))?;
    checked(builder.actor_did(owner.clone()))?;
    checked(builder.authority(checked(Authority::owner(&key.public))?))?;
    checked(builder.account_sequence(values.sequence))?;
    checked(builder.timestamp_bound(checked(TimestampBound::new(values.begin, values.end))?))?;
    checked(builder.idempotency_key(IdempotencyKey::new(values.action)))?;
    checked(builder.fee_limit(Amount::from_u128(values.fee)))?;
    checked(builder.payload_hash(checked(hash::payload_hash_for(&payload))?))?;
    checked(builder.payload(payload))?;
    let envelope = checked(builder.build())?;
    let canonical = checked(encode_unsigned_envelope(&envelope))?;
    let disclosure = checked(bind(&canonical, &registry()?))?;
    let encoded = encoded_disclosure(&disclosure)?;
    let (canonical, call_disclosure) = construct_signing_call(&canonical)?;
    let (request, digest) = evm_signing_request(
        key.binding,
        &key.handle,
        &canonical,
        &encoded_disclosure(&call_disclosure)?,
    )?;
    let response = host.call(&request)?;
    assert_eq!(response.len(), 72);
    assert_eq!(response[7], 0);
    checked(
        ring::signature::UnparsedPublicKey::new(&ring::signature::ED25519, key.public)
            .verify(&digest, &response[8..]),
    )?;
    let mut corrupted = encoded;
    let last = corrupted.len() - 1;
    corrupted[last] ^= 1;
    assert_eq!(
        host.call(&evm_signing_request(key.binding, &key.handle, &canonical, &corrupted)?.0)?[7],
        1
    );
    checked(encode_signed_envelope(
        &envelope.attach_signature(checked(Signature::new(&response[8..]))?),
    ))
}

fn rotation(host: &Host, key: &Key, value: &OwnerRotation, envelope: Envelope) -> Result<Vec<u8>> {
    sign(
        host,
        key,
        value.owner(),
        payload(2, &checked(value.payload())?)?,
        envelope,
    )
}

fn provider_cases(host: &Host, old: &Key, next: &Key) -> Result<Vec<Vec<u8>>> {
    let owner = did(old)?;
    let envelope = Envelope {
        sequence: 7,
        action: [61; 32],
        begin: 1000,
        end: 1010,
        fee: 4,
    };
    let consent = OwnerRotationConsent {
        owner: owner.clone(),
        current_public_key: old.public,
        pending_public_key: next.public,
        announcement: [62; 32],
        action_key: envelope.action,
        expires_at: envelope.end,
    };
    let signed = rotation(
        host,
        next,
        &OwnerRotation::Consent(consent.clone()),
        Envelope {
            sequence: 0,
            fee: 0,
            ..envelope
        },
    )?;
    let mut result = vec![signed];
    for value in [
        OwnerRotation::Announce {
            owner: owner.clone(),
            pending_public_key: next.public,
            begin: 1005,
            end: 1009,
            effective_sequence: 20,
        },
        OwnerRotation::Cancel {
            owner,
            announcement: [62; 32],
        },
    ] {
        let signed = rotation(host, old, &value, envelope)?;
        let decoded = checked(layerx_intents::canonical::decode_signed_activity(
            &signed,
            &registry()?,
        ))?;
        assert_eq!(checked(OwnerRotation::from_activity(&decoded))?, value);
        let unsigned = checked(layerx_intents::canonical::unsigned_activity_bytes(&decoded))?;
        let disclosed = checked(bind(&unsigned, &registry()?))?;
        assert_eq!(
            disclosed.native_operation,
            Some(DisclosedNativeOperation::OwnerRotation(Box::new(value)))
        );
        result.push(signed);
    }
    Ok(result)
}

#[test]
fn kms_owner_rotation_disclosure() -> Result<()> {
    let mut host = Host::new()?;
    let old = Key::create(&host, 81)?;
    let next = Key::create(&host, 82)?;
    assert_ne!(old.public, next.public);
    let original = provider_cases(&host, &old, &next)?;
    host.stop();
    host.start()?;
    assert_eq!(provider_cases(&host, &old, &next)?, original);
    Ok(())
}
