use crate::chain_proof::{
    decode::CommitmentProof, header::ChainHeader, validators::ValidatorSet, ChainProofError, Commit,
};
use crate::protocol_evidence::PROGRAMS_MODULE_ID;

pub mod deployment;
pub mod freshness;
pub mod lifecycle;
pub mod program_state;
pub mod protocol_head;
pub mod trust_root;
pub mod wind_down;

pub(crate) mod verify_bundle;

pub fn commitment_trust_step(
    commitment: &CommitmentProof,
) -> Result<(ChainHeader, Commit, ValidatorSet), ChainProofError> {
    Ok((
        ChainHeader::decode(&commitment.signed_header)?,
        verify_bundle::decode_commit(&commitment.commit)?,
        verify_bundle::decode_validators(&commitment.validator_set)?,
    ))
}

pub fn programs_state_key(kernel_key: &[u8]) -> Vec<u8> {
    let mut key = Vec::with_capacity(3 + kernel_key.len());
    key.push(0x01);
    key.extend_from_slice(&PROGRAMS_MODULE_ID.to_be_bytes());
    key.extend_from_slice(kernel_key);
    key
}
