use crate::rpc::payments::{CommittedSnapshot, FeeEstimate};
use crate::rpc::Head;
use crate::sequencer::SequencerError;

/// Largest canonical encoded size the native fee meter accepts.
pub const MAX_FEE_METER_CANONICAL_BYTES: u64 = 1_048_576;

/// Hypothetical activity meter priced by the committed native fee schedule.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct FeeMeter {
    pub activity_type: u32,
    pub canonical_bytes: u64,
    pub execution_units: u64,
    pub storage_units: u64,
}

/// One committed fee estimate bound to the single authenticated head captured
/// before the read; head fields are that observation's, not the estimate's.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FeeObservation {
    head: Head,
    snapshot: CommittedSnapshot<FeeEstimate>,
}

impl FeeObservation {
    #[must_use]
    pub const fn head(&self) -> Head {
        self.head
    }

    #[must_use]
    pub const fn observed_sequence(&self) -> u64 {
        self.snapshot.observed_sequence
    }

    #[must_use]
    pub const fn state_root(&self) -> [u8; 32] {
        self.snapshot.state_root
    }

    #[must_use]
    pub const fn estimate(&self) -> &FeeEstimate {
        &self.snapshot.value
    }

    #[must_use]
    pub fn into_parts(self) -> (Head, CommittedSnapshot<FeeEstimate>) {
        (self.head, self.snapshot)
    }
}

/// Refusal of one head-bound fee estimate.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum FeeEstimateError {
    MeterOutOfRange {
        canonical_bytes: u64,
    },
    SnapshotSkew {
        head_sequence: u64,
        observed_sequence: u64,
    },
    Read(SequencerError),
}
