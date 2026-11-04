//! Verification of retained kernel artifacts; these cannot authorize an EVM submission.

use super::{SigningError, SubmissionBindingAudit};
use crate::prepare::{verify_disclosure_binding, Prepared};
use layerx_crypto::SignatureMessage;
use layerx_types::activity::{Authority, Signature};
use layerx_types::payload::ModuleRegistry;
use layerx_wire::activity::{
    decode_signed as decode_kernel_signed, encode_signed, encode_signed_envelope, encode_unsigned,
};
use layerx_wire::hash::{activity_id, Domain};

/// A checked historical artifact, deliberately distinct from `VerifiedSubmission`.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedKernelArtifact {
    signed_canonical_bytes: Vec<u8>,
    audit: SubmissionBindingAudit,
}

impl VerifiedKernelArtifact {
    pub fn exact_bytes(&self) -> &[u8] {
        &self.signed_canonical_bytes
    }
    pub const fn activity_id(&self) -> [u8; 32] {
        self.audit.activity_id
    }
    pub const fn idempotency_key(&self) -> [u8; 32] {
        self.audit.idempotency_key
    }

    pub const fn signed_byte_length(&self) -> usize {
        self.audit.signed_byte_length
    }
}

/// Attaches an owner signature to the exact kernel preparation retained by Human.
///
/// # Errors
/// Refuses inconsistent typed/canonical preparation state and malformed signatures.
pub fn attach_kernel_signature(
    prepared: &Prepared,
    signature: [u8; 64],
) -> Result<Vec<u8>, SigningError> {
    verify_disclosure_binding(prepared).map_err(SigningError::Disclosure)?;
    let signature = Signature::new(&signature).map_err(SigningError::SignatureEncoding)?;
    let signed = prepared.envelope.clone().attach_signature(signature);
    encode_signed_envelope(&signed).map_err(SigningError::Wire)
}

pub fn verify_kernel_before_submit(
    signed_canonical_bytes: &[u8],
    prepared: &Prepared,
    signer_public_key: &[u8; 32],
    registry: &ModuleRegistry,
) -> Result<VerifiedKernelArtifact, SigningError> {
    verify_disclosure_binding(prepared).map_err(SigningError::Disclosure)?;
    let activity =
        decode_kernel_signed(signed_canonical_bytes, registry).map_err(SigningError::Wire)?;
    if encode_signed(&activity).map_err(SigningError::Wire)? != signed_canonical_bytes {
        return Err(SigningError::PreparedBytesChanged);
    }
    let unsigned = encode_unsigned(&activity).map_err(SigningError::Wire)?;
    if unsigned != prepared.canonical_bytes {
        return Err(SigningError::PreparedBytesChanged);
    }
    let signature = activity.signature().ok_or(SigningError::SignatureInvalid)?;
    let signature: &[u8; 64] = signature
        .try_into()
        .map_err(|_| SigningError::SignatureInvalid)?;
    let message = SignatureMessage::new(
        Domain::SignaturePreimage,
        activity.protocol_version(),
        activity.network_id(),
        &unsigned,
    )
    .map_err(|_| SigningError::SignatureInvalid)?;
    let Authority::Owner(owner) = prepared.envelope.authority() else {
        return Err(SigningError::AuthorityMismatch);
    };
    if owner.as_ref() != signer_public_key.as_slice() {
        return Err(SigningError::AuthorityMismatch);
    }
    layerx_crypto::ed25519::verify(signer_public_key, signature, message)
        .map_err(|_| SigningError::SignatureInvalid)?;
    let activity_id = activity_id(&activity).map_err(SigningError::Wire)?;
    Ok(VerifiedKernelArtifact {
        signed_canonical_bytes: signed_canonical_bytes.to_vec(),
        audit: SubmissionBindingAudit {
            activity_id,
            idempotency_key: activity.idempotency_key(),
            signed_byte_length: signed_canonical_bytes.len(),
        },
    })
}
