#![forbid(unsafe_code)]

pub mod merkle;
pub mod proto;

/// A committed proof could not establish the claimed commitment.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProofError {
    /// The leaf position or number of aunts is inconsistent with the tree.
    InvalidProof,
    /// The proof reconstructs a different root.
    RootMismatch,
}

impl std::fmt::Display for ProofError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(match self {
            Self::InvalidProof => "invalid Merkle proof position or aunt count",
            Self::RootMismatch => "Merkle proof root mismatch",
        })
    }
}

impl std::error::Error for ProofError {}
