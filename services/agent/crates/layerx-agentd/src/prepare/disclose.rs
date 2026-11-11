//! Disclosure binding for the exact type-2 transaction and its sender.

use layerx_crypto::disclosure::DisclosureError;
use layerx_crypto::evm_transaction::{decode_unsigned, Eip1559Call, EvmTransactionError};
use layerx_crypto::payments::{bind_call, Disclosure};

use super::evm::{CallBindingError, Prepared};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DisclosedCall {
    pub disclosure: Disclosure,
    pub digest: [u8; 32],
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum RestoreCallError {
    Transaction(EvmTransactionError),
    Binding(CallBindingError),
    InvalidCall,
}

/// # Errors
/// Returns a malformed-payload error for an invalid chain, gas limit or fee ordering,
/// and propagates disclosure construction and digest errors.
pub fn bind(call: &Eip1559Call, from: [u8; 20]) -> Result<DisclosedCall, DisclosureError> {
    if call.chain_id == 0
        || call.gas_limit == 0
        || call.max_priority_fee_per_gas > call.max_fee_per_gas
    {
        return Err(DisclosureError::MalformedPayload);
    }
    let disclosure = bind_call(call, from)?;
    let digest = disclosure.signing_digest()?;
    Ok(DisclosedCall { disclosure, digest })
}

/// # Errors
/// Returns the binding error when the call, disclosure, signing hash or digest differs
/// from the retained preparation, or when disclosure construction fails.
pub fn verify_binding(prepared: &Prepared) -> Result<(), CallBindingError> {
    if prepared.call.signing_hash() != prepared.signing_hash {
        return Err(CallBindingError::SigningHashMismatch);
    }
    let bound = bind(&prepared.call, prepared.from).map_err(CallBindingError::Disclosure)?;
    if bound.disclosure != prepared.disclosure {
        return Err(CallBindingError::Disclosure(
            DisclosureError::FieldMismatch("call"),
        ));
    }
    if bound.digest != prepared.disclosure_digest {
        return Err(CallBindingError::DigestMismatch);
    }
    Ok(())
}

/// # Errors
/// Returns a transaction error for invalid unsigned bytes, `InvalidCall` for invalid
/// gas or fees, or a binding error when the supplied hashes do not match the call.
pub fn restore_call(
    bytes: &[u8],
    from: [u8; 20],
    observed_head_height: u64,
    disclosure_digest: [u8; 32],
    signing_hash: [u8; 32],
) -> Result<Prepared, RestoreCallError> {
    let call = decode_unsigned(bytes).map_err(RestoreCallError::Transaction)?;
    if call.gas_limit == 0 || call.max_priority_fee_per_gas > call.max_fee_per_gas {
        return Err(RestoreCallError::InvalidCall);
    }
    let bound = bind(&call, from)
        .map_err(|error| RestoreCallError::Binding(CallBindingError::Disclosure(error)))?;
    let prepared = Prepared {
        call,
        from,
        disclosure: bound.disclosure,
        disclosure_digest,
        signing_hash,
        observed_head_height,
    };
    verify_binding(&prepared).map_err(RestoreCallError::Binding)?;
    Ok(prepared)
}
