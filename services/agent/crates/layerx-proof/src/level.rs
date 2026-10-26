//! Evidence-gated verification-level production.

use layerx_types::{rung::Rung, verify::VerificationLevel};

use crate::evidence::Evidence;

/// A level token constructible only by proof routines in this crate.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) struct Achieved(VerificationLevel, bool);

impl Achieved {
    pub(crate) const fn sequencer_signed() -> Self {
        Self(VerificationLevel::SEQUENCER_SIGNED, false)
    }

    pub(crate) const fn pinned_sequencer_signed() -> Self {
        Self(VerificationLevel::SEQUENCER_SIGNED, true)
    }

    pub(crate) const fn rung(self) -> Rung {
        if self.1 {
            return Rung::Receipted;
        }
        match self.0.rung() {
            Some(rung) => rung,
            None => panic!("settlement anchoring cannot produce an achieved token"),
        }
    }

    pub(crate) const fn batch_included() -> Self {
        Self(VerificationLevel::BATCH_INCLUDED, false)
    }

    pub(crate) const fn state_proven() -> Self {
        Self(VerificationLevel::STATE_PROVEN, false)
    }

    pub(crate) const fn checkpoint_finalised() -> Self {
        Self(VerificationLevel::CHECKPOINT_FINALISED, false)
    }

    pub(crate) const fn level(self) -> VerificationLevel {
        self.0
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
