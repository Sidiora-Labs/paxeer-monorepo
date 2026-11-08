//! Native call and disclosure bindings shared by source selection and settlement.

use crate::prepare::{EvmPrepared, Prepared};
use crate::sign::{SigningPreparation, VerifiedSubmission};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
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

/// Binds a wire carrier to the signing preparation retained under its preparation identity.
pub fn validate_retained_preparation(
    prepared: &Prepared,
    signing: Option<&SigningPreparation>,
) -> Result<Eip1559Call, NativeBindingError> {
    let signing = signing.ok_or(NativeBindingError::Preparation)?;
    validate_native_preparation(&signing.transaction)?;
    if signing.actor_address != signing.transaction.from
        || signing.transaction.observed_head_height != prepared.observed_head_sequence
    {
        return Err(NativeBindingError::Preparation);
    }
    validate_retained_call(prepared, &signing.transaction.call, signing.actor_address)?;
    Ok(signing.transaction.call.clone())
}

/// Checks the wire representation independently of the native transaction representation.
pub(super) fn validate_retained_carrier(prepared: &Prepared) -> Result<(), NativeBindingError> {
    let native_carrier = prepared
        .disclosure
        .construct_method()
        .map_err(|_| NativeBindingError::Preparation)?
        .is_some();
    prepared
        .disclosure
        .validate_canonical(&prepared.canonical_bytes)
        .map_err(|_| NativeBindingError::Preparation)?;
    let digest = prepared
        .disclosure
        .audit_digest()
        .map_err(|_| NativeBindingError::Preparation)?;
    if native_carrier {
        // Call-bound approvals retain a typed transaction disclosure. Its producer
        // re-encodes the call; the wire carrier is never sent to a transaction decoder.
        if prepared
            .disclosure
            .signing_digest()
            .map_err(|_| NativeBindingError::Preparation)?
            != prepared.signing_preimage
            || prepared.envelope.payload().as_bytes()
                != retained_payload(&prepared.disclosure, prepared.envelope.actor_did())?
        {
            return Err(NativeBindingError::Preparation);
        }
    } else {
        crate::prepare::verify_disclosure_binding(prepared)
            .map_err(|_| NativeBindingError::Preparation)?;
    }
    if prepared.observed_head_sequence == 0
        || prepared.audit.observed_head_sequence != prepared.observed_head_sequence
        || prepared.audit.idempotency_key != prepared.disclosure.idempotency_key
        || prepared.disclosure_digest.0 != digest
        || prepared.audit.disclosure_digest.0 != digest
        || prepared.envelope.protocol_version() != 3
        || prepared.envelope.idempotency_key().bytes() != prepared.disclosure.idempotency_key
        || prepared.envelope.timestamp_bound().not_before() != prepared.disclosure.expiry.not_before
        || prepared.envelope.timestamp_bound().not_after() != prepared.disclosure.expiry.not_after
        || prepared.envelope.activity_type() != prepared.disclosure.activity_type
        || prepared.envelope.fee_limit().value() != prepared.disclosure.fee_limit
        || prepared.envelope.authority() != &retained_authority(prepared.envelope.actor_did())?
        || prepared.envelope.payload_hash()
            != layerx_wire::hash::payload_hash_for(prepared.envelope.payload())
                .map_err(|_| NativeBindingError::Preparation)?
    {
        return Err(NativeBindingError::Preparation);
    }
    Ok(())
}

fn validate_retained_call(
    prepared: &Prepared,
    call: &Eip1559Call,
    sender: [u8; 20],
) -> Result<(), NativeBindingError> {
    validate_retained_carrier(prepared)?;
    let native = crate::prepare::disclose::bind(call, sender)
        .map_err(|_| NativeBindingError::Preparation)?;
    if prepared
        .disclosure
        .construct_method()
        .map_err(|_| NativeBindingError::Preparation)?
        .is_some()
        && (prepared.disclosure != native.disclosure
            || prepared.signing_preimage != call.signing_hash())
    {
        return Err(NativeBindingError::Preparation);
    }
    if u64::from(prepared.envelope.network_id()) != call.chain_id
        || prepared.envelope.account_sequence() != call.nonce
        || prepared.envelope.activity_type() != native.disclosure.activity_type
        || prepared.envelope.fee_limit().value() != native.disclosure.fee_limit
        || prepared.envelope.payload().as_bytes()
            != retained_payload(&native.disclosure, prepared.envelope.actor_did())?
    {
        return Err(NativeBindingError::Preparation);
    }
    Ok(())
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
    let (call, signature) =
        decode_signed(submission.exact_bytes()).map_err(|_| NativeBindingError::Submission)?;
    let sender = recover_sender(&call, &signature).map_err(|_| NativeBindingError::Sender)?;
    validate_retained_call(prepared, &call, sender)?;
    let native = crate::prepare::disclose::bind(&call, sender)
        .map_err(|_| NativeBindingError::Preparation)?;
    if transaction_hash(submission.exact_bytes()) != submission.activity_id()
        || submission.idempotency_key() != native.disclosure.idempotency_key
        || submission.recovered_sender() != sender
    {
        return Err(NativeBindingError::Submission);
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

/// Builds the approval path's wire carrier from a validated native transaction.
/// Callers must retain and supply the corresponding signing preparation separately.
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
    let call = layerx_crypto::evm_transaction::decode_unsigned(canonical_bytes)
        .map_err(|_| NativeBindingError::Preparation)?;
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
    let envelope = builder
        .build()
        .map_err(|_| NativeBindingError::Preparation)?;
    let canonical = layerx_wire::activity::encode_unsigned_envelope(&envelope)
        .map_err(|_| NativeBindingError::Preparation)?;
    let result = crate::prepare::restore_canonical(
        &canonical,
        observed_height,
        registry,
        Some(envelope.authority()),
    )
    .map_err(|_| NativeBindingError::Preparation)?;
    validate_retained_call(&result, &call, sender)?;
    Ok(result)
}
