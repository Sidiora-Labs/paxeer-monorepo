use layerx_crypto::{ed25519, SignatureMessage};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistry};
use layerx_types::result::KnownResult;
use layerx_wire::activity::{decode_signed, signing_bytes, Activity};
use layerx_wire::hash::{payload_hash, Domain};
use layerx_wire::WireError;

pub(super) fn decode(raw: &[u8], registry: &ModuleRegistry) -> Result<Activity, WireError> {
    let activity = decode_signed(raw, registry)?;
    let refused = |result: KnownResult, offset| WireError {
        result: result.into(),
        offset,
    };
    let call_type = ActivityType::new(ModuleId::Programs, 3)
        .map_err(|_| refused(KnownResult::UnknownActivity, 14))?;
    if activity.activity_type() != call_type {
        return Err(refused(KnownResult::UnknownActivity, 14));
    }
    layerx_types::ids::Did::new(activity.actor_did())
        .map_err(|_| refused(KnownResult::MalformedEnvelope, 23))?;
    let authority_offset = 28 + activity.actor_did().len();
    if payload_hash(&activity)? != activity.payload_hash() {
        return Err(refused(
            KnownResult::PayloadHashMismatch,
            authority_offset + activity.authority().len() + 85,
        ));
    }
    let public_key = activity
        .authority()
        .try_into()
        .map_err(|_| refused(KnownResult::BadSignature, authority_offset))?;
    let signature = activity
        .signature()
        .and_then(|bytes| bytes.try_into().ok())
        .ok_or_else(|| refused(KnownResult::BadSignature, raw.len()))?;
    let unsigned = signing_bytes(&activity)?;
    let message = SignatureMessage::new(
        Domain::SignaturePreimage,
        activity.protocol_version(),
        activity.network_id(),
        unsigned.as_bytes(),
    )
    .map_err(|error| WireError {
        result: error.result_code(),
        offset: 9,
    })?;
    ed25519::verify(&public_key, &signature, message).map_err(|error| WireError {
        result: error.result_code(),
        offset: raw.len().saturating_sub(64),
    })?;
    Ok(activity)
}
