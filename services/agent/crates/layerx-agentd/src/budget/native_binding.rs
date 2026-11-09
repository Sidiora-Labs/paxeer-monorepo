//! Native call and disclosure bindings shared by source selection and settlement.

use crate::prepare::EvmPrepared;
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

/// Requires the retained preparation to equal the transaction authorized for signing.
pub fn validate_retained_preparation(
    prepared: &EvmPrepared,
    signing: Option<&SigningPreparation>,
) -> Result<Eip1559Call, NativeBindingError> {
    let signing = signing.ok_or(NativeBindingError::Preparation)?;
    validate_native_preparation(prepared)?;
    validate_native_preparation(&signing.transaction)?;
    if signing.actor_address != prepared.from || signing.transaction != *prepared {
        return Err(NativeBindingError::Preparation);
    }
    Ok(prepared.call.clone())
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
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
) -> Result<Eip1559Call, NativeBindingError> {
    let call = verify_native_budget_submission(prepared, submission)?;
    if submission.recovered_sender() != prepared.from {
        return Err(NativeBindingError::Sender);
    }
    Ok(call)
}

/// Reopens the exact native transaction with its call-bound disclosure.
pub fn restore_native_preparation(
    canonical_bytes: &[u8],
    sender: [u8; 20],
    observed_height: u64,
) -> Result<EvmPrepared, NativeBindingError> {
    let call = layerx_crypto::evm_transaction::decode_unsigned(canonical_bytes)
        .map_err(|_| NativeBindingError::Preparation)?;
    let bound = crate::prepare::disclose::bind(&call, sender)
        .map_err(|_| NativeBindingError::Preparation)?;
    let prepared = crate::prepare::disclose::restore_call(
        canonical_bytes,
        sender,
        observed_height,
        bound.digest,
        call.signing_hash(),
    )
    .map_err(|_| NativeBindingError::Preparation)?;
    validate_native_preparation(&prepared)?;
    Ok(prepared)
}
