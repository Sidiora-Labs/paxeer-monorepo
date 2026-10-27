//! Evidence reads over JSON-RPC.

use crate::rpc::batch;
use crate::sequencer::{AccountProof, SequencerBlock, SequencerEndpoint, SequencerError};

pub type Rung = layerx_types::rung::Rung;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProofBundle {
    pub account_proof: AccountProof,
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
    Ok(batch::lookup(endpoint, height)?)
}

pub fn proof_bundle(
    endpoint: &SequencerEndpoint,
    address: [u8; 20],
    slots: &[[u8; 32]],
    height: u64,
) -> Result<ProofBundle, EvidenceError> {
    let account_proof = endpoint.account_proof(&address, slots, height)?;
    let landing_block = checkpoint(endpoint, height)?;
    Ok(ProofBundle {
        account_proof,
        landing_block,
    })
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
