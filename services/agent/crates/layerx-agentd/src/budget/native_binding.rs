//! Native call and disclosure bindings shared by source selection and settlement.

use crate::prepare::{EvmPrepared, Prepared};
use crate::sign::VerifiedSubmission;
use layerx_crypto::disclosure::call::{validate_call, validate_canonical};
use layerx_crypto::evm_transaction::{
    decode_signed, decode_unsigned, recover_sender, transaction_hash, Eip1559Call,
};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum NativeBindingError {
    Preparation,
    Submission,
    Sender,
}

/// Verifies the real preparation producer's call, disclosure and signing digest.
pub fn validate_native_preparation(prepared: &EvmPrepared) -> Result<(), NativeBindingError> {
    crate::prepare::verify_call_binding(prepared).map_err(|_| NativeBindingError::Preparation)?;
    if prepared.observed_head_height == 0 {
        return Err(NativeBindingError::Preparation);
    }
    Ok(())
}

/// Verifies native bytes retained in the durable preparation carrier.
pub fn validate_retained_preparation(
    prepared: &Prepared,
) -> Result<Eip1559Call, NativeBindingError> {
    let call =
        decode_unsigned(&prepared.canonical_bytes).map_err(|_| NativeBindingError::Preparation)?;
    let sender = prepared
        .disclosure
        .actor
        .as_slice()
        .try_into()
        .map_err(|_| NativeBindingError::Sender)?;
    validate_call(&call, &prepared.disclosure, sender)
        .map_err(|_| NativeBindingError::Preparation)?;
    validate_canonical(&prepared.disclosure, &prepared.canonical_bytes)
        .map_err(|_| NativeBindingError::Preparation)?;
    let digest = prepared
        .disclosure
        .audit_digest()
        .map_err(|_| NativeBindingError::Preparation)?;
    if call.signing_hash() != prepared.signing_preimage
        || prepared.observed_head_sequence == 0
        || prepared.audit.observed_head_sequence != prepared.observed_head_sequence
        || prepared.audit.idempotency_key != prepared.disclosure.idempotency_key
        || prepared.disclosure_digest.0 != digest
        || prepared.audit.disclosure_digest.0 != digest
        || prepared.envelope.protocol_version() != 3
        || u64::from(prepared.envelope.network_id()) != call.chain_id
        || prepared.envelope.idempotency_key().bytes() != prepared.disclosure.idempotency_key
        || prepared.envelope.timestamp_bound().not_before() != prepared.disclosure.expiry.not_before
        || prepared.envelope.timestamp_bound().not_after() != prepared.disclosure.expiry.not_after
        || prepared.envelope.activity_type() != prepared.disclosure.activity_type
        || prepared.envelope.account_sequence() != call.nonce
        || prepared.envelope.fee_limit().value() != prepared.disclosure.fee_limit
        || prepared.envelope.authority() != &retained_authority(prepared.envelope.actor_did())?
        || prepared.envelope.payload().as_bytes()
            != retained_payload(&prepared.disclosure, prepared.envelope.actor_did())?
        || prepared.envelope.payload_hash()
            != layerx_wire::hash::payload_hash_for(prepared.envelope.payload())
                .map_err(|_| NativeBindingError::Preparation)?
    {
        return Err(NativeBindingError::Preparation);
    }
    Ok(call)
}

pub fn verify_native_budget_submission(
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
) -> Result<Eip1559Call, NativeBindingError> {
    validate_native_preparation(prepared)?;
    let (call, signature) =
        decode_signed(submission.exact_bytes()).map_err(|_| NativeBindingError::Submission)?;
    if call != prepared.call
        || transaction_hash(submission.exact_bytes()) != submission.activity_id()
        || submission.idempotency_key() != prepared.disclosure.idempotency_key
    {
        return Err(NativeBindingError::Submission);
    }
    if recover_sender(&call, &signature).map_err(|_| NativeBindingError::Sender)? != prepared.from {
        return Err(NativeBindingError::Sender);
    }
    Ok(call)
}

pub(super) fn verify_retained_submission(
    prepared: &Prepared,
    submission: &VerifiedSubmission,
) -> Result<Eip1559Call, NativeBindingError> {
    let expected = validate_retained_preparation(prepared)?;
    let (call, signature) =
        decode_signed(submission.exact_bytes()).map_err(|_| NativeBindingError::Submission)?;
    if call != expected
        || transaction_hash(submission.exact_bytes()) != submission.activity_id()
        || submission.idempotency_key() != prepared.disclosure.idempotency_key
    {
        return Err(NativeBindingError::Submission);
    }
    if recover_sender(&call, &signature)
        .map_err(|_| NativeBindingError::Sender)?
        .as_slice()
        != prepared.disclosure.actor
    {
        return Err(NativeBindingError::Sender);
    }
    Ok(call)
}

fn retained_payload(
    disclosure: &layerx_crypto::disclosure::Disclosure,
    actor: &layerx_types::ids::Did,
) -> Result<Vec<u8>, NativeBindingError> {
    use layerx_crypto::disclosure::DisclosedNativeOperation;
    Ok(match &disclosure.native_operation {
        Some(DisclosedNativeOperation::ProgramCall(value)) => value
            .native()
            .encode()
            .map_err(|_| NativeBindingError::Preparation)?,
        Some(DisclosedNativeOperation::ProgramDeploy(value)) => value
            .native()
            .encode()
            .map_err(|_| NativeBindingError::Preparation)?,
        Some(DisclosedNativeOperation::ProgramUpgrade(value)) => value
            .native()
            .encode()
            .map_err(|_| NativeBindingError::Preparation)?,
        Some(DisclosedNativeOperation::ProgramWindDown(value)) => value
            .native()
            .encode()
            .map_err(|_| NativeBindingError::Preparation)?,
        None => disclosure
            .payment
            .as_ref()
            .ok_or(NativeBindingError::Preparation)?
            .encode(actor.as_bytes())
            .map_err(|_| NativeBindingError::Preparation)?,
        _ => return Err(NativeBindingError::Preparation),
    })
}

fn retained_authority(
    actor: &layerx_types::ids::Did,
) -> Result<layerx_types::activity::Authority, NativeBindingError> {
    let actor_text =
        std::str::from_utf8(actor.as_bytes()).map_err(|_| NativeBindingError::Preparation)?;
    let public = actor_text
        .strip_prefix("did:layerx:")
        .and_then(|text| layerx_programs::hex::decode(text).ok())
        .filter(|bytes| bytes.len() == 32)
        .ok_or(NativeBindingError::Preparation)?;
    layerx_types::activity::Authority::owner(&public).map_err(|_| NativeBindingError::Preparation)
}

/// Restores a native preparation into the budget carrier without a kernel signing envelope.
/// The actor DID is joined to the native sender by source selection at the held checkpoint.
pub fn restore_native_preparation(
    canonical_bytes: &[u8],
    sender: [u8; 20],
    observed_height: u64,
    actor: layerx_types::ids::Did,
    registry: &layerx_types::payload::ModuleRegistry,
) -> Result<Prepared, NativeBindingError> {
    use layerx_crypto::disclosure::call::bind_call;
    use layerx_types::activity::{EnvelopeBuilder, TimestampBound};
    use layerx_types::amount::Amount;
    use layerx_types::ids::IdempotencyKey;
    use layerx_types::payload::Payload;
    let call = decode_unsigned(canonical_bytes).map_err(|_| NativeBindingError::Preparation)?;
    let disclosure = bind_call(&call, sender).map_err(|_| NativeBindingError::Preparation)?;
    let payload = retained_payload(&disclosure, &actor)?;
    let authority = retained_authority(&actor)?;
    let payload = Payload::new(registry, disclosure.activity_type, &payload)
        .map_err(|_| NativeBindingError::Preparation)?;
    let payload_hash = layerx_wire::hash::payload_hash_for(&payload)
        .map_err(|_| NativeBindingError::Preparation)?;
    let bound = TimestampBound::new(disclosure.expiry.not_before, disclosure.expiry.not_after)
        .map_err(|_| NativeBindingError::Preparation)?;
    let mut builder = EnvelopeBuilder::new();
    builder
        .protocol_version(3)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .network_id(u32::try_from(call.chain_id).map_err(|_| NativeBindingError::Preparation)?)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .activity_type(disclosure.activity_type)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .actor_did(actor)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .authority(authority)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .account_sequence(call.nonce)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .timestamp_bound(bound)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .idempotency_key(IdempotencyKey::new(disclosure.idempotency_key))
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .fee_limit(Amount::from_u128(disclosure.fee_limit))
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .payload_hash(payload_hash)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .payload(payload)
        .map_err(|_| NativeBindingError::Preparation)?;
    let digest = crate::prepare::DisclosureDigest(
        disclosure
            .audit_digest()
            .map_err(|_| NativeBindingError::Preparation)?,
    );
    let result = Prepared {
        envelope: builder
            .build()
            .map_err(|_| NativeBindingError::Preparation)?,
        canonical_bytes: canonical_bytes.to_vec(),
        signing_preimage: call.signing_hash(),
        observed_head_sequence: observed_height,
        audit: crate::prepare::PreparationAuditEntry {
            idempotency_key: disclosure.idempotency_key,
            observed_head_sequence: observed_height,
            disclosure_digest: digest,
        },
        disclosure,
        disclosure_digest: digest,
    };
    validate_retained_preparation(&result)?;
    Ok(result)
}
