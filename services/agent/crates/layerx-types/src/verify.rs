//! Verification levels, evidence provenance, and non-authoritative projections.

use std::cmp::Ordering;

use crate::chain::BlockReference;
use crate::rung::Rung;

/// Internal rank for the ordered verification lattice.
#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd)]
#[repr(u8)]
enum VerificationRank {
    Unverified = 0,
    SequencerSigned = 1,
    BatchIncluded = 2,
    StateProven = 3,
    CheckpointFinalised = 4,
    SettlementAnchored = 5,
    Committed = 6,
}

/// An ordered verification level. The internal rank is private so callers can
/// request and compare levels but cannot invent a new level representation.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct VerificationLevel(VerificationRank);

/// A wire label outside the declared verification lattice.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct InvalidVerificationRank(pub u8);

/// A string label outside the current verification lattice.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct InvalidVerificationLabel(pub String);

impl VerificationLevel {
    /// No local evidence has passed.
    pub const UNVERIFIED: Self = Self(VerificationRank::Unverified);
    /// The authorised sequencer signature passed.
    pub const SEQUENCER_SIGNED: Self = Self(VerificationRank::SequencerSigned);
    /// Activity inclusion in the signed batch passed.
    pub const BATCH_INCLUDED: Self = Self(VerificationRank::BatchIncluded);
    /// State inclusion against the resulting state root passed.
    pub const STATE_PROVEN: Self = Self(VerificationRank::StateProven);
    /// Retired archive label; never a current policy level.
    #[deprecated(note = "archive only; use historical::verify for decoding")]
    pub const CHECKPOINT_FINALISED: Self = Self(VerificationRank::CheckpointFinalised);
    /// Retired archive label; never a current policy level.
    #[deprecated(note = "archive only; settlement cannot establish current finality")]
    pub const SETTLEMENT_ANCHORED: Self = Self(VerificationRank::SettlementAnchored);

    /// Verified covering-block commitment, receipt coverage and the relevant state witness passed.
    /// A wire label is only an observation; only a proof-owned `Verified` value carries achievement.
    pub const COMMITTED: Self = Self(VerificationRank::Committed);

    /// Compares current evidence strength with a requested level.
    /// Retired labels always fail the policy comparison, including against themselves.
    #[must_use]
    pub const fn compare(self, requested: Self) -> Ordering {
        if !self.is_current() || !requested.is_current() {
            return Ordering::Less;
        }
        let achieved = self.0 as u8;
        let requested = requested.0 as u8;
        if achieved < requested {
            Ordering::Less
        } else if achieved > requested {
            Ordering::Greater
        } else {
            Ordering::Equal
        }
    }

    #[must_use]
    pub const fn is_current(self) -> bool {
        !matches!(
            self.0,
            VerificationRank::CheckpointFinalised | VerificationRank::SettlementAnchored
        )
    }

    /// Returns the wire label. Ranks 4 and 5 are reserved historical labels.
    #[must_use]
    pub const fn wire_rank(self) -> u8 {
        self.0 as u8
    }

    /// Decodes an observation's stable wire label, without constructing verified evidence.
    ///
    /// # Errors
    /// Rejects retired checkpoint/settlement ranks and unknown labels.
    /// Rank 6 identifies current committed evidence without reinterpreting archive rank 4.
    pub const fn from_wire_rank(rank: u8) -> Result<Self, InvalidVerificationRank> {
        match rank {
            0 => Ok(Self::UNVERIFIED),
            1 => Ok(Self::SEQUENCER_SIGNED),
            2 => Ok(Self::BATCH_INCLUDED),
            3 => Ok(Self::STATE_PROVEN),
            6 => Ok(Self::COMMITTED),
            _ => Err(InvalidVerificationRank(rank)),
        }
    }

    /// Returns the stable label, including reserved archive spellings.
    #[must_use]
    pub const fn wire_label(self) -> &'static str {
        match self.0 {
            VerificationRank::Unverified => "unverified",
            VerificationRank::SequencerSigned => "sequencer_signed",
            VerificationRank::BatchIncluded => "batch_included",
            VerificationRank::StateProven => "state_proven",
            VerificationRank::CheckpointFinalised => "checkpoint_finalised",
            VerificationRank::SettlementAnchored => "settlement_anchored",
            VerificationRank::Committed => "committed",
        }
    }

    /// Decodes a current observation label without constructing verified evidence.
    ///
    /// # Errors
    /// Rejects retired and unknown labels, retaining the offending string.
    pub fn from_wire_label(label: &str) -> Result<Self, InvalidVerificationLabel> {
        match label {
            "unverified" => Ok(Self::UNVERIFIED),
            "sequencer_signed" => Ok(Self::SEQUENCER_SIGNED),
            "batch_included" => Ok(Self::BATCH_INCLUDED),
            "state_proven" => Ok(Self::STATE_PROVEN),
            "committed" => Ok(Self::COMMITTED),
            _ => Err(InvalidVerificationLabel(label.to_owned())),
        }
    }

    /// Maps current levels to write rungs. Retired labels never confer a rung.
    #[must_use]
    pub const fn rung(self) -> Option<Rung> {
        match self.0 {
            VerificationRank::Unverified | VerificationRank::SequencerSigned => {
                Some(Rung::Submitted)
            }
            VerificationRank::BatchIncluded | VerificationRank::StateProven => {
                Some(Rung::Receipted)
            }
            VerificationRank::Committed => Some(Rung::Committed),
            VerificationRank::CheckpointFinalised | VerificationRank::SettlementAnchored => None,
        }
    }
}

impl Ord for VerificationLevel {
    fn cmp(&self, other: &Self) -> Ordering {
        let key = |level: Self| match level.0 {
            VerificationRank::CheckpointFinalised => 0,
            VerificationRank::SettlementAnchored => 1,
            _ => level.wire_rank() + 2,
        };
        key(*self).cmp(&key(*other))
    }
}

impl PartialOrd for VerificationLevel {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

/// Identifiers for the exact evidence supporting a verified value.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct EvidenceIds {
    receipt_digest: [u8; 32],
    batch_id: Option<[u8; 32]>,
    state_root: Option<[u8; 32]>,
    committed_block: Option<BlockReference>,
    storage_witness_digest: Option<[u8; 32]>,
}

impl EvidenceIds {
    /// Returns the core receipt digest.
    #[must_use]
    pub const fn receipt_digest(&self) -> [u8; 32] {
        self.receipt_digest
    }

    /// Returns the batch evidence identifier when inclusion passed.
    #[must_use]
    pub const fn batch_id(&self) -> Option<[u8; 32]> {
        self.batch_id
    }

    /// Returns the proven state root when state inclusion passed.
    #[must_use]
    pub const fn state_root(&self) -> Option<[u8; 32]> {
        self.state_root
    }

    /// Returns the verified covering block, retaining the separate execution height.
    #[must_use]
    pub const fn committed_block(&self) -> Option<&BlockReference> {
        self.committed_block.as_ref()
    }

    /// Returns the digest of the storage witness verified against the committed state root.
    #[must_use]
    pub const fn storage_witness_digest(&self) -> Option<[u8; 32]> {
        self.storage_witness_digest
    }
}

/// A decoded core value bound to proof-achieved evidence and freshness.
///
/// This type intentionally has no public constructor or level mutator.
/// Committed requires verified block commitment, receipt coverage and the relevant
/// storage witness. No label decoder, archive reader or trust-pin parser can mint it.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Verified<T> {
    value: T,
    level: VerificationLevel,
    evidence: EvidenceIds,
    head_sequence: u64,
}

impl<T> Verified<T> {
    /// Borrows the core-produced value.
    #[must_use]
    pub const fn value(&self) -> &T {
        &self.value
    }

    /// Returns the level actually achieved by proof verification.
    #[must_use]
    pub const fn level(&self) -> VerificationLevel {
        self.level
    }

    /// Returns the exact evidence identifiers.
    #[must_use]
    pub const fn evidence(&self) -> &EvidenceIds {
        &self.evidence
    }

    /// Returns the core head sequence this value is relative to.
    #[must_use]
    pub const fn head_sequence(&self) -> u64 {
        self.head_sequence
    }
}

/// A locally estimated value that can never inhabit a verified-value field.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Projection<T> {
    value: T,
    rationale: String,
}

impl<T> Projection<T> {
    /// Labels a locally estimated value with its non-authoritative rationale.
    pub fn new(value: T, rationale: impl Into<String>) -> Self {
        Self {
            value,
            rationale: rationale.into(),
        }
    }

    /// Borrows the estimated value.
    #[must_use]
    pub const fn value(&self) -> &T {
        &self.value
    }

    /// Returns the explanation for the estimate.
    #[must_use]
    pub fn rationale(&self) -> &str {
        &self.rationale
    }

    /// Consumes the projection without changing its non-authoritative meaning.
    #[must_use]
    pub fn into_value(self) -> T {
        self.value
    }
}

#[cfg(test)]
mod wire_label_tests {
    use super::{InvalidVerificationLabel, VerificationLevel};

    #[test]
    fn wire_label_round_trips_current_levels() {
        for (level, label) in [
            (VerificationLevel::UNVERIFIED, "unverified"),
            (VerificationLevel::SEQUENCER_SIGNED, "sequencer_signed"),
            (VerificationLevel::BATCH_INCLUDED, "batch_included"),
            (VerificationLevel::STATE_PROVEN, "state_proven"),
            (VerificationLevel::COMMITTED, "committed"),
        ] {
            assert_eq!(level.wire_label(), label);
            assert_eq!(VerificationLevel::from_wire_label(label), Ok(level));
        }
    }

    #[test]
    #[allow(deprecated)]
    fn wire_label_names_archives_but_refuses_retired_and_unknown_labels() {
        assert_eq!(
            VerificationLevel::CHECKPOINT_FINALISED.wire_label(),
            "checkpoint_finalised"
        );
        assert_eq!(
            VerificationLevel::SETTLEMENT_ANCHORED.wire_label(),
            "settlement_anchored"
        );
        for label in [
            "checkpoint_finalised",
            "settlement_anchored",
            "unknown",
            "",
            "COMMITTED",
            " committed",
        ] {
            assert_eq!(
                VerificationLevel::from_wire_label(label),
                Err(InvalidVerificationLabel(label.to_owned()))
            );
        }
    }
}
