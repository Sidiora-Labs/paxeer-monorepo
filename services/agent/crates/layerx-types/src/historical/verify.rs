//! Historical wire labels. Decoding never yields a current verification level.

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct HistoricalVerificationLevel(u8);

impl HistoricalVerificationLevel {
    /// Reads the original six-rank label without promoting it to current evidence.
    ///
    /// # Errors
    ///
    /// Returns [`crate::verify::InvalidVerificationRank`] for ranks above five.
    pub const fn from_wire_rank(rank: u8) -> Result<Self, crate::verify::InvalidVerificationRank> {
        if rank <= 5 {
            Ok(Self(rank))
        } else {
            Err(crate::verify::InvalidVerificationRank(rank))
        }
    }

    #[must_use]
    pub const fn wire_rank(self) -> u8 {
        self.0
    }

    /// Archive observations never establish a current write rung.
    ///
    /// # Panics
    ///
    /// Panics if an internal value violates the six-rank invariant enforced by
    /// [`Self::from_wire_rank`].
    #[must_use]
    pub const fn rung(self) -> Option<crate::rung::Rung> {
        match self.0 {
            0..=5 => None,
            _ => panic!("invalid historical verification rank"),
        }
    }
}
