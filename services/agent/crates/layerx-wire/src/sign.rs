//! Historical kernel signature digests for checking archived envelopes.
//!
//! Current transaction signers use `Eip1559Call::signing_hash` in layerx-crypto.
//! Supported receipt signatures use `crate::receipt::encode_unsigned` and
//! `crate::hash::receipt_digest`, independently of this historical module.

use layerx_types::activity::UnsignedEnvelope;

use crate::hash::{domain, CanonicalBytes, Domain};
use crate::historical::activity::{encode_unsigned, encode_unsigned_envelope, Activity};
use crate::WireError;

/// The exact 32-byte domain-separated digest a signer covers.
#[derive(Clone, Copy, Eq, PartialEq)]
pub struct SigningPreimage([u8; 32]);

impl SigningPreimage {
    /// Borrows the sole byte string supplied to a signer.
    #[must_use]
    pub const fn as_bytes(&self) -> &[u8; 32] {
        &self.0
    }
}

impl std::fmt::Debug for SigningPreimage {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.write_str("SigningPreimage([canonical digest])")
    }
}

/// Computes the signature preimage from the canonical unsigned activity form.
///
/// # Errors
///
/// Returns a typed canonical encoding or hash length error.
pub fn preimage(activity: &Activity) -> Result<SigningPreimage, WireError> {
    let unsigned = CanonicalBytes::from_wire(encode_unsigned(activity)?);
    domain(Domain::SignaturePreimage, &unsigned).map(SigningPreimage)
}

/// Computes the signature preimage from a newly constructed typed envelope.
///
/// # Errors
///
/// Returns a typed canonical encoding or hash length error.
pub fn preimage_unsigned(envelope: &UnsignedEnvelope) -> Result<SigningPreimage, WireError> {
    let unsigned = CanonicalBytes::from_wire(encode_unsigned_envelope(envelope)?);
    domain(Domain::SignaturePreimage, &unsigned).map(SigningPreimage)
}
