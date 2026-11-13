//! `LayerX` JSON-RPC client modules.

pub use layerx_programs_registry::chain_proof::{ChainProofVerifier, VerifiedChainHead};

/// Coordinates authenticated by a verified node commit. Observed receipt
/// sequences are carried separately and never stand in for a block height.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct CommittedBlockContext {
    committed_height: u64,
    block_hash: [u8; 32],
    app_hash: [u8; 32],
}

impl CommittedBlockContext {
    /// # Errors
    ///
    /// Returns `commit::CommitError::Invalid` when the verified height cannot be represented as an unsigned height.
    pub fn from_verified(head: &VerifiedChainHead) -> Result<Self, commit::CommitError> {
        let committed_height = u64::try_from(head.height())
            .ok()
            .filter(|height| *height > 0)
            .ok_or(commit::CommitError::Invalid)?;
        Ok(Self {
            committed_height,
            block_hash: head.header_hash(),
            app_hash: head.app_hash(),
        })
    }

    #[must_use]
    pub const fn committed_height(self) -> u64 {
        self.committed_height
    }

    #[must_use]
    pub const fn block_hash(self) -> [u8; 32] {
        self.block_hash
    }

    #[must_use]
    pub const fn app_hash(self) -> [u8; 32] {
        self.app_hash
    }
}

/// A verified committed block and an independently observed receipt sequence.
/// Sequence observations are never used to construct or verify block evidence.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ObservedBlockContext {
    committed: CommittedBlockContext,
    /// Compatibility spelling used by the out-of-path fee projection. This is
    /// an observed sequence, not the committed height or a finality authority.
    pub chain_sequence: u64,
}

impl ObservedBlockContext {
    #[must_use]
    pub const fn new(committed: CommittedBlockContext, observed_sequence: u64) -> Self {
        Self {
            committed,
            chain_sequence: observed_sequence,
        }
    }

    #[must_use]
    pub const fn committed(self) -> CommittedBlockContext {
        self.committed
    }

    #[must_use]
    pub const fn observed_sequence(self) -> u64 {
        self.chain_sequence
    }
}

/// Compatibility name for out-of-path fee consumers. No sealed-batch or
/// checkpoint coordinates remain; sequence and committed evidence are separate.
pub type Head = ObservedBlockContext;

pub mod account;
pub mod batch;
pub mod commit;
pub mod evidence;
pub mod history;
pub mod module_state;
pub mod payments;
pub mod receipt;
pub mod receipt_commit;
pub mod stream;
pub mod submit;
pub mod withdrawal;
