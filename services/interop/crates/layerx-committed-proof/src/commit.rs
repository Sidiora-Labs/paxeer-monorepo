//! Commit verification using the consensus canonical precommit encoding.

use std::collections::BTreeSet;

use ed25519_dalek::{Signature, VerifyingKey};
use sha2::{Digest, Sha256};

use crate::{
    header::{BlockId, Timestamp, Validator},
    proto::{encode_length_delimited, encode_varint, encode_varint_field},
    ProofError,
};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum BlockIdFlag {
    Absent,
    Commit,
    Nil,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommitSig {
    pub block_id_flag: BlockIdFlag,
    pub validator_address: Vec<u8>,
    pub timestamp: Timestamp,
    pub signature: Option<[u8; 64]>,
}

impl CommitSig {
    #[must_use]
    pub fn absent() -> Self {
        Self {
            block_id_flag: BlockIdFlag::Absent,
            validator_address: Vec::new(),
            timestamp: Timestamp {
                seconds: -62_135_596_800,
                nanos: 0,
            },
            signature: None,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Commit {
    pub height: u64,
    pub round: i32,
    pub block_id: BlockId,
    pub signatures: Vec<CommitSig>,
}

/// Verify every present precommit and return the power voting for this block.
///
/// # Errors
/// Returns an error for malformed commits or sets, invalid signatures, or power
/// that does not strictly exceed two thirds of the supplied validator set.
pub fn verify_commit(
    chain_id: &str,
    set: &[Validator],
    header_hash: [u8; 32],
    height: u64,
    commit: &Commit,
) -> Result<u64, ProofError> {
    if height == 0
        || height > i64::MAX as u64
        || commit.height != height
        || commit.round < 0
        || commit.block_id.hash.as_slice() != header_hash
        || commit.block_id.part_set_header.total == 0
        || commit.block_id.part_set_header.total > 101
        || commit.block_id.part_set_header.hash.len() != 32
        || set.is_empty()
        || commit.signatures.len() != set.len()
    {
        return Err(ProofError::InvalidCommit);
    }

    let mut keys = BTreeSet::new();
    let mut total_power = 0_u64;
    for validator in set {
        let power =
            u64::try_from(validator.voting_power).map_err(|_| ProofError::InvalidValidatorSet)?;
        if power == 0 || !keys.insert(validator.pub_key) {
            return Err(ProofError::InvalidValidatorSet);
        }
        total_power = total_power
            .checked_add(power)
            .filter(|total| *total <= (i64::MAX / 8) as u64)
            .ok_or(ProofError::InvalidValidatorSet)?;
    }

    let mut signed_power = 0_u64;
    for (validator, vote) in set.iter().zip(&commit.signatures) {
        if vote.block_id_flag == BlockIdFlag::Absent {
            if *vote != CommitSig::absent() {
                return Err(ProofError::InvalidCommit);
            }
            continue;
        }
        if !(-62_135_596_800..=253_402_300_799).contains(&vote.timestamp.seconds)
            || vote.timestamp.nanos >= 1_000_000_000
        {
            return Err(ProofError::InvalidCommit);
        }
        let signature = vote.signature.ok_or(ProofError::InvalidCommit)?;
        let address = Sha256::digest(validator.pub_key);
        if vote.validator_address.as_slice() != &address[..20] {
            return Err(ProofError::InvalidCommitSignature);
        }
        let key = VerifyingKey::from_bytes(&validator.pub_key)
            .map_err(|_| ProofError::InvalidCommitSignature)?;
        key.verify_strict(
            &vote_sign_bytes(chain_id, commit, vote),
            &Signature::from_bytes(&signature),
        )
        .map_err(|_| ProofError::InvalidCommitSignature)?;
        if vote.block_id_flag == BlockIdFlag::Commit {
            // All powers are positive and the total was checked above.
            signed_power += validator.voting_power as u64;
        }
    }

    if u128::from(signed_power) * 3 <= u128::from(total_power) * 2 {
        return Err(ProofError::InsufficientVotingPower);
    }
    Ok(signed_power)
}

fn vote_sign_bytes(chain_id: &str, commit: &Commit, vote: &CommitSig) -> Vec<u8> {
    let mut encoded = Vec::new();
    encode_varint_field(1, 2, &mut encoded);
    encoded.push(0x11);
    encoded.extend_from_slice(&commit.height.to_le_bytes());
    if commit.round != 0 {
        encoded.push(0x19);
        encoded.extend_from_slice(&i64::from(commit.round).to_le_bytes());
    }
    if vote.block_id_flag == BlockIdFlag::Commit {
        let mut parts = Vec::new();
        encode_varint_field(
            1,
            u64::from(commit.block_id.part_set_header.total),
            &mut parts,
        );
        encode_length_delimited(2, &commit.block_id.part_set_header.hash, &mut parts);
        let mut block_id = Vec::new();
        encode_length_delimited(1, &commit.block_id.hash, &mut block_id);
        encode_length_delimited(2, &parts, &mut block_id);
        encode_length_delimited(4, &block_id, &mut encoded);
    }
    let mut timestamp = Vec::new();
    if vote.timestamp.seconds != 0 {
        encode_varint_field(1, vote.timestamp.seconds as u64, &mut timestamp);
    }
    if vote.timestamp.nanos != 0 {
        encode_varint_field(2, u64::from(vote.timestamp.nanos), &mut timestamp);
    }
    encode_length_delimited(5, &timestamp, &mut encoded);
    if !chain_id.is_empty() {
        encode_length_delimited(6, chain_id.as_bytes(), &mut encoded);
    }
    let mut delimited = Vec::new();
    encode_varint(encoded.len() as u64, &mut delimited);
    delimited.extend_from_slice(&encoded);
    delimited
}
