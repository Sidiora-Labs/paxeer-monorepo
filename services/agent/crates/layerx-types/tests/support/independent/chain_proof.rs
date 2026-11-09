// Source-derived native proof rules; see PROVENANCE.md.
pub mod head_verify;
pub mod header;
pub mod merkle;
pub mod proto;
pub mod tx;
pub mod validators;
pub mod vote;

pub use head_verify::ChainProofVerifier;
mod ed25519 {
    pub fn verify_zip215(key: &[u8; 32], message: &[u8], signature: &[u8; 64]) -> bool {
        super::super::verify_ed25519(key, message, signature)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Commit {
    pub height: i64,
    pub round: i32,
    pub block_id: vote::BlockId,
    pub signatures: Vec<vote::CommitSig>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedChainHead {
    pub(crate) chain_id: String,
    pub(crate) height: i64,
    pub(crate) time_ms: i64,
    pub(crate) header_hash: [u8; 32],
    pub(crate) app_hash: [u8; 32],
    pub(crate) data_hash: [u8; 32],
    pub(crate) last_results_hash: [u8; 32],
    pub(crate) next_validators_hash: [u8; 32],
}

impl VerifiedChainHead {
    #[must_use]
    pub fn chain_id(&self) -> &str {
        &self.chain_id
    }

    #[must_use]
    pub const fn height(&self) -> i64 {
        self.height
    }

    #[must_use]
    pub const fn header_hash(&self) -> [u8; 32] {
        self.header_hash
    }

    #[must_use]
    pub const fn data_hash(&self) -> [u8; 32] {
        self.data_hash
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ChainProofError {
    Truncated,
    NonCanonical,
    UnknownField,
    Mismatch,
    InsufficientPower,
}

impl std::fmt::Display for ChainProofError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Truncated => f.write_str("truncated proof"),
            Self::NonCanonical => f.write_str("non-canonical encoding"),
            Self::UnknownField => f.write_str("unknown field"),
            Self::Mismatch => f.write_str("mismatch"),
            Self::InsufficientPower => f.write_str("insufficient power"),
        }
    }
}

impl std::error::Error for ChainProofError {}
