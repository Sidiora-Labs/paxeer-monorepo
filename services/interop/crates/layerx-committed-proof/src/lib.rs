#![forbid(unsafe_code)]

pub mod commit;
pub mod header;
pub mod merkle;
pub mod proto;

/// A committed proof could not establish the claimed commitment.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProofError {
    /// The leaf position or number of aunts is inconsistent with the tree.
    InvalidProof,
    /// The proof reconstructs a different root.
    RootMismatch,
    /// The supplied validator set differs from the header commitment.
    ValidatorsHashMismatch,
    /// The commit metadata or signature slots are malformed.
    InvalidCommit,
    /// Validator powers or keys do not form a valid consensus set.
    InvalidValidatorSet,
    /// A present precommit does not verify against its indexed validator.
    InvalidCommitSignature,
    /// The block's precommits do not carry strictly more than two thirds power.
    InsufficientVotingPower,
}

impl std::fmt::Display for ProofError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(match self {
            Self::InvalidProof => "invalid Merkle proof position or aunt count",
            Self::RootMismatch => "Merkle proof root mismatch",
            Self::ValidatorsHashMismatch => "header validator set hash mismatch",
            Self::InvalidCommit => "invalid block commit",
            Self::InvalidValidatorSet => "invalid validator set",
            Self::InvalidCommitSignature => "invalid commit signature",
            Self::InsufficientVotingPower => "insufficient commit voting power",
        })
    }
}

impl std::error::Error for ProofError {}

/// Bind the header to the supplied validator set before returning its hash.
///
/// # Errors
/// Returns [`ProofError::ValidatorsHashMismatch`] if the set is not committed
/// by this header.
///
/// # Panics
/// Panics if the header timestamp is outside protobuf's permitted range.
pub fn verify_header(
    header: &header::Header,
    set: &[header::Validator],
) -> Result<[u8; 32], ProofError> {
    if header.validators_hash != header::validators_hash(set) {
        return Err(ProofError::ValidatorsHashMismatch);
    }
    Ok(header.hash())
}
