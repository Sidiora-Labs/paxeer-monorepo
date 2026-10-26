pub mod proto;
pub mod merkle {}
pub mod ics23 {}
pub mod ics23_absent {}
pub mod multistore {}
pub mod ed25519;
pub mod header;
pub mod validators;
pub mod vote;
pub mod decode {}
pub mod head_verify;
pub mod state {}
pub mod tx {}
pub mod results {}
pub mod trust {}
pub mod trust_store {}

pub use head_verify::ChainProofVerifier;

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
    pub const fn time_ms(&self) -> i64 {
        self.time_ms
    }

    #[must_use]
    pub const fn app_hash(&self) -> [u8; 32] {
        self.app_hash
    }

    #[must_use]
    pub const fn data_hash(&self) -> [u8; 32] {
        self.data_hash
    }

    #[must_use]
    pub const fn last_results_hash(&self) -> [u8; 32] {
        self.last_results_hash
    }

    #[must_use]
    pub const fn next_validators_hash(&self) -> [u8; 32] {
        self.next_validators_hash
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ChainProofError {
    Truncated,
    NonCanonical,
    UnknownField,
    Mismatch,
    InsufficientPower,
    Expired,
    Absent,
}
