//! Exact-byte signature verification gate before submission.

use layerx_crypto::evm_transaction::{assemble, decode_signed, recover_sender, transaction_hash};
use layerx_crypto::signer::AgentSignature;
use layerx_types::payload::ModuleRegistry;

use crate::prepare::verify_disclosure_binding;

use super::{SigningError, SigningPreparation};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct SubmissionBindingAudit {
    pub activity_id: [u8; 32],
    pub idempotency_key: [u8; 32],
    pub signed_byte_length: usize,
}

/// Bytes admitted to submission only after exact signature verification.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedSubmission {
    pub(super) signed_canonical_bytes: Vec<u8>,
    pub(super) audit: SubmissionBindingAudit,
}

impl VerifiedSubmission {
    #[must_use]
    pub fn exact_bytes(&self) -> &[u8] {
        &self.signed_canonical_bytes
    }

    #[must_use]
    pub fn into_exact_bytes(self) -> Vec<u8> {
        self.signed_canonical_bytes
    }

    #[must_use]
    pub const fn activity_id(&self) -> [u8; 32] {
        self.audit.activity_id
    }

    #[must_use]
    pub const fn idempotency_key(&self) -> [u8; 32] {
        self.audit.idempotency_key
    }

    #[must_use]
    pub const fn signed_byte_length(&self) -> usize {
        self.audit.signed_byte_length
    }
}

pub(crate) fn attach_signature(
    prepared: &SigningPreparation,
    signature: [u8; 65],
) -> Result<Vec<u8>, SigningError> {
    if prepared.transaction.signing_hash != prepared.transaction.call.signing_hash() {
        return Err(SigningError::PreparedBytesChanged);
    }
    let signature = AgentSignature::new(signature).map_err(|_| SigningError::SignatureInvalid)?;
    let sender = recover_sender(&prepared.transaction.call, &signature)
        .map_err(|_| SigningError::SignatureInvalid)?;
    if sender != prepared.actor_address {
        return Err(SigningError::SignerMismatch);
    }
    Ok(assemble(&prepared.transaction.call, &signature))
}

pub(crate) fn verify_exact(
    signed_canonical_bytes: &[u8],
    prepared: &SigningPreparation,
    signer_address: &[u8; 20],
    _registry: &ModuleRegistry,
) -> Result<VerifiedSubmission, SigningError> {
    verify_disclosure_binding(&prepared.authorization).map_err(SigningError::Disclosure)?;
    if prepared.transaction.signing_hash != prepared.transaction.call.signing_hash() {
        return Err(SigningError::PreparedBytesChanged);
    }
    let (call, signature) =
        decode_signed(signed_canonical_bytes).map_err(|_| SigningError::SignatureInvalid)?;
    if call != prepared.transaction.call {
        return Err(SigningError::PreparedBytesChanged);
    }
    let sender = recover_sender(&call, &signature).map_err(|_| SigningError::SignatureInvalid)?;
    if sender != prepared.actor_address || &sender != signer_address {
        return Err(SigningError::SignerMismatch);
    }
    Ok(VerifiedSubmission {
        signed_canonical_bytes: signed_canonical_bytes.to_vec(),
        audit: SubmissionBindingAudit {
            activity_id: transaction_hash(signed_canonical_bytes),
            idempotency_key: prepared.authorization.audit.idempotency_key,
            signed_byte_length: signed_canonical_bytes.len(),
        },
    })
}
