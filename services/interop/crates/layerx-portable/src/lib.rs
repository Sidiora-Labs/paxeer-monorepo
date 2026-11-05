#![forbid(unsafe_code)]

//! Portable, authority-free evidence boundaries for interoperability adapters.
//!
//! A [`PortableReceiptV2`] carries exact signed receipt bytes that are checked
//! against independently pinned receipt keys, allowing a
//! `LayerX` receipt to be verified without a node, gateway, daemon, database,
//! clock, or network connection. External evidence travels
//! in the opposite direction through [`verify_external_evidence`]: the
//! adapter keeps ownership of protocol-specific cryptography and constraints,
//! while this crate binds its typed result to the exact presentation and
//! pinned upstream specification that were verified.

mod external;
mod receipt;

pub use external::{
    verify_external_evidence, ExternalEvidenceKind, ExternalEvidenceVerifier, ExternalPresentation,
    ExternalPresentationError, ExternalVerificationError, VerifiedExternalEvidence,
};
pub use receipt::{
    verify_portable_json, PortableReceiptError, PortableReceiptV2, PortableVerified,
    PortableVerifiedReceiptV2, ReceiptKeyEntry, PORTABLE_RECEIPT_FORMAT,
};

/// Codify anchor for portable receipt export and typed external verification.
#[must_use]
pub const fn interop_portable_verification() -> &'static str {
    "layerx-receipt-proof-v2-and-pinned-external-evidence-v1"
}
