//! Evidence-gated verification-level production.

use layerx_types::{rung::Rung, verify::VerificationLevel};

use crate::evidence::Evidence;

/// Evidence kinds constructible only by proof routines in this crate.
/// Receipt, inclusion and state proofs cannot authenticate a Comet commit.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum Achieved {
    SequencerSigned,
    PinnedSequencerSigned,
    BatchIncluded,
    StateProven,
}

impl Achieved {
    pub(crate) const fn sequencer_signed() -> Self {
        Self::SequencerSigned
    }

    pub(crate) const fn pinned_sequencer_signed() -> Self {
        Self::PinnedSequencerSigned
    }

    pub(crate) const fn rung(self) -> Rung {
        match self {
            Self::SequencerSigned => Rung::Submitted,
            Self::PinnedSequencerSigned | Self::BatchIncluded | Self::StateProven => {
                Rung::Receipted
            }
        }
    }

    pub(crate) const fn batch_included() -> Self {
        Self::BatchIncluded
    }

    pub(crate) const fn state_proven() -> Self {
        Self::StateProven
    }

    pub(crate) const fn level(self) -> VerificationLevel {
        match self {
            Self::SequencerSigned | Self::PinnedSequencerSigned => {
                VerificationLevel::SEQUENCER_SIGNED
            }
            Self::BatchIncluded => VerificationLevel::BATCH_INCLUDED,
            Self::StateProven => VerificationLevel::STATE_PROVEN,
        }
    }
}

/// Returns the exact level inseparably carried by verified evidence.
///
/// Callers can request and compare public lattice values, but cannot construct
/// an `Evidence` or raise its private achieved token.
#[must_use]
pub const fn achieved(evidence: &Evidence) -> VerificationLevel {
    evidence.achieved().level()
}

#[must_use]
pub const fn achieved_rung(evidence: &Evidence) -> Rung {
    evidence.achieved().rung()
}
