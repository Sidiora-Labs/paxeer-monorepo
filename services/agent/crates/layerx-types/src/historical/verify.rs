//! Historical wire labels. Decoding never yields a current verification level.

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct HistoricalVerificationLevel(u8);

impl HistoricalVerificationLevel {
    /// Reads the original six-rank label without promoting it to current evidence.
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
    #[must_use]
    pub const fn rung(self) -> Option<crate::rung::Rung> {
        None
    }
}
