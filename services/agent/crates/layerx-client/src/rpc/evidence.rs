//! Evidence reads over JSON-RPC.

use crate::sequencer::{SequencerBlock, SequencerError, StorageWitness};
use crate::storage_witness::{verify_storage_witness, StorageWitnessError};
use layerx_programs_registry::chain_proof::VerifiedChainHead;

pub type SequencerEndpoint = crate::sequencer::SequencerEndpoint;
pub type PxEndpoint = crate::px::PxEndpoint;
pub type Rung = layerx_types::rung::Rung;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum RootSelector {
    Latest,
    Batch(u64),
    Checkpoint([u8; 32]),
}

/// Legacy landing-block carrier. A height alone cannot establish storage evidence.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProofBundle {
    pub landing_block: SequencerBlock,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedProofBundle {
    pub landing_block: SequencerBlock,
    pub committed_header: VerifiedChainHead,
    pub witnesses: Vec<StorageWitness>,
}

mod sealed {
    pub trait Target {}
    impl Target for u64 {}
    impl Target for &layerx_programs_registry::chain_proof::VerifiedChainHead {}
}

/// A verified header is required to return a bundle with storage evidence.
/// The height-only interface remains source compatible and explicitly refuses
/// to treat an unauthenticated root as committed evidence.
pub trait ProofBundleTarget: sealed::Target {
    type Bundle;
    fn read_bundle(
        self,
        endpoint: &SequencerEndpoint,
        address: [u8; 20],
        slots: &[[u8; 32]],
    ) -> Result<Self::Bundle, EvidenceError>;
}

impl ProofBundleTarget for u64 {
    type Bundle = ProofBundle;
    fn read_bundle(
        self,
        _endpoint: &SequencerEndpoint,
        _address: [u8; 20],
        _slots: &[[u8; 32]],
    ) -> Result<Self::Bundle, EvidenceError> {
        Err(EvidenceError::MissingCommittedHeader)
    }
}

impl ProofBundleTarget for &VerifiedChainHead {
    type Bundle = VerifiedProofBundle;
    fn read_bundle(
        self,
        endpoint: &SequencerEndpoint,
        address: [u8; 20],
        slots: &[[u8; 32]],
    ) -> Result<Self::Bundle, EvidenceError> {
        witnessed_bundle(endpoint, address, slots, self)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum EvidenceError {
    Rpc(SequencerError),
    MissingCommittedHeader,
    BlockMismatch,
    MissingWitness,
    WitnessMismatch,
    Witness(StorageWitnessError),
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

pub fn proof_bundle<T: ProofBundleTarget>(
    endpoint: &SequencerEndpoint,
    address: [u8; 20],
    slots: &[[u8; 32]],
    target: T,
) -> Result<T::Bundle, EvidenceError> {
    target.read_bundle(endpoint, address, slots)
}

fn witnessed_bundle(
    endpoint: &SequencerEndpoint,
    address: [u8; 20],
    slots: &[[u8; 32]],
    head: &VerifiedChainHead,
) -> Result<VerifiedProofBundle, EvidenceError> {
    let height = u64::try_from(head.height()).map_err(|_| EvidenceError::BlockMismatch)?;
    let landing_block = checkpoint(endpoint, height)?;
    if landing_block.hash != head.header_hash() || endpoint.state_root(height)? != head.app_hash() {
        return Err(EvidenceError::BlockMismatch);
    }
    let witnesses = if slots.is_empty() {
        Vec::new()
    } else {
        endpoint.storage_witness(&address, slots, height)?
    };
    if witnesses.len() != slots.len() {
        return Err(EvidenceError::MissingWitness);
    }
    for (slot, witness) in slots.iter().zip(&witnesses) {
        if witness.address != address || witness.slot != *slot {
            return Err(EvidenceError::WitnessMismatch);
        }
        verify_storage_witness(witness, head.app_hash()).map_err(EvidenceError::Witness)?;
    }
    Ok(VerifiedProofBundle {
        landing_block,
        committed_header: head.clone(),
        witnesses,
    })
}

pub fn proof_bundle_with_history<T: ProofBundleTarget>(
    endpoint: &SequencerEndpoint,
    address: [u8; 20],
    slots: &[[u8; 32]],
    target: T,
) -> Result<T::Bundle, EvidenceError> {
    proof_bundle(endpoint, address, slots, target)
}

#[must_use]
pub const fn verification_label(rung: Rung) -> &'static str {
    rung.name()
}
