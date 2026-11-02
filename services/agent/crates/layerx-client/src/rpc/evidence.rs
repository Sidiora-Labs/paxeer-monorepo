//! Evidence reads over JSON-RPC.

use crate::sequencer::{SequencerBlock, SequencerError};

pub type SequencerEndpoint = crate::sequencer::SequencerEndpoint;
pub type PxEndpoint = crate::px::PxEndpoint;

pub type Rung = layerx_types::rung::Rung;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum RootSelector {
    Latest,
    Batch(u64),
    Checkpoint([u8; 32]),
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProofBundle {
    pub landing_block: SequencerBlock,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum EvidenceError {
    Rpc(SequencerError),
}

impl From<SequencerError> for EvidenceError {
    fn from(error: SequencerError) -> Self {
        Self::Rpc(error)
    }
}

pub fn checkpoint(
    endpoint: &SequencerEndpoint,
    height: u64,
) -> Result<SequencerBlock, EvidenceError> {
    endpoint
        .block_by_number(height)?
        .ok_or(EvidenceError::Rpc(SequencerError::Unavailable))
}

pub fn proof_bundle(
    endpoint: &SequencerEndpoint,
    _address: [u8; 20],
    _slots: &[[u8; 32]],
    height: u64,
) -> Result<ProofBundle, EvidenceError> {
    let landing_block = checkpoint(endpoint, height)?;
    Ok(ProofBundle { landing_block })
}

pub fn proof_bundle_with_history(
    endpoint: &SequencerEndpoint,
    address: [u8; 20],
    slots: &[[u8; 32]],
    height: u64,
) -> Result<ProofBundle, EvidenceError> {
    proof_bundle(endpoint, address, slots, height)
}

#[must_use]
pub const fn verification_label(rung: Rung) -> &'static str {
    rung.name()
}
