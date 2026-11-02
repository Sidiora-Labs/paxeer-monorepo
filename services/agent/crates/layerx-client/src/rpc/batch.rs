//! Batch operations over JSON-RPC.

use crate::sequencer::SequencerError;

pub type SequencerEndpoint = crate::sequencer::SequencerEndpoint;

pub type SequencerBlock = crate::sequencer::SequencerBlock;
pub type BatchHeaderError = SequencerError;

pub fn lookup(
    endpoint: &SequencerEndpoint,
    height: u64,
) -> Result<SequencerBlock, BatchHeaderError> {
    endpoint
        .block_by_number(height)?
        .ok_or(BatchHeaderError::Unavailable)
}

pub fn lookup_untrusted(
    endpoint: &SequencerEndpoint,
    height: u64,
) -> Result<SequencerBlock, BatchHeaderError> {
    lookup(endpoint, height)
}

pub fn lookup_with_history(
    endpoint: &SequencerEndpoint,
    height: u64,
) -> Result<SequencerBlock, BatchHeaderError> {
    lookup(endpoint, height)
}
