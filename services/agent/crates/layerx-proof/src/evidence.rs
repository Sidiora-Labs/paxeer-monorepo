//! Identifiers for the exact bytes and commitments supporting a level.

use layerx_types::{rung::Rung, verify::VerificationLevel};

use crate::level::Achieved;

/// Receipt, inclusion and state identifiers inseparably paired with an achieved
/// level. These verifiers establish at most [`Rung::Receipted`].
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Evidence {
    achieved: Achieved,
    receipt_digest: Option<[u8; 32]>,
    header_digest: Option<[u8; 32]>,
    proof_root: Option<[u8; 32]>,
}

impl Evidence {
    pub(crate) const fn sequencer(receipt_digest: [u8; 32]) -> Self {
        Self {
            achieved: Achieved::sequencer_signed(),
            receipt_digest: Some(receipt_digest),
            header_digest: None,
            proof_root: None,
        }
    }

    pub(crate) const fn with_pinned_receipt_keys(mut self) -> Self {
        self.achieved = Achieved::pinned_sequencer_signed();
        self
    }

    pub(crate) const fn batch(header_digest: [u8; 32], proof_root: [u8; 32]) -> Self {
        Self {
            achieved: Achieved::batch_included(),
            receipt_digest: None,
            header_digest: Some(header_digest),
            proof_root: Some(proof_root),
        }
    }

    pub(crate) const fn state(header_digest: [u8; 32], proof_root: [u8; 32]) -> Self {
        Self {
            achieved: Achieved::state_proven(),
            receipt_digest: None,
            header_digest: Some(header_digest),
            proof_root: Some(proof_root),
        }
    }

    pub(crate) const fn achieved(&self) -> Achieved {
        self.achieved
    }

    /// Returns the exact level established by the producing verifier.
    #[must_use]
    pub const fn level(&self) -> VerificationLevel {
        self.achieved.level()
    }

    /// Returns the rung established by this evidence's verifier, capped at Receipted.
    #[must_use]
    pub const fn rung(&self) -> Rung {
        self.achieved.rung()
    }

    /// Returns the signed receipt digest when receipt verification produced it.
    #[must_use]
    pub const fn receipt_digest(&self) -> Option<[u8; 32]> {
        self.receipt_digest
    }

    /// Returns the signed batch-header digest when inclusion produced it.
    #[must_use]
    pub const fn header_digest(&self) -> Option<[u8; 32]> {
        self.header_digest
    }

    /// Returns the root against which the path was verified.
    #[must_use]
    pub const fn proof_root(&self) -> Option<[u8; 32]> {
        self.proof_root
    }
}
