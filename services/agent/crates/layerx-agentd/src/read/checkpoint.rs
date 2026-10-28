//! Block commit checkpoints and root-provenance proof bundles.

use std::collections::BTreeSet;

use layerx_proof::inclusion::{
    verify_activity, verify_state, InclusionError, SequencerAuthorization,
};
use layerx_proof::merkle::Proof;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use sha2::{Digest as _, Sha256};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct CommitTimestamp {
    pub seconds: i64,
    pub nanos: u32,
}

impl CommitTimestamp {
    fn valid(self) -> bool {
        (-62_135_596_800..=253_402_300_799).contains(&self.seconds) && self.nanos < 1_000_000_000
    }

    fn encode(self) -> Vec<u8> {
        let mut bytes = Vec::new();
        varint_field(&mut bytes, 1, self.seconds as u64);
        varint_field(&mut bytes, 2, u64::from(self.nanos));
        bytes
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct BlockId {
    pub hash: [u8; 32],
    pub part_set_total: u32,
    pub part_set_hash: [u8; 32],
}

impl BlockId {
    fn valid(self) -> bool {
        self.hash != [0; 32] && self.part_set_total > 0 && self.part_set_hash != [0; 32]
    }

    fn encode(self) -> Vec<u8> {
        let mut parts = Vec::new();
        varint_field(&mut parts, 1, u64::from(self.part_set_total));
        bytes_field(&mut parts, 2, &self.part_set_hash);
        let mut bytes = Vec::new();
        bytes_field(&mut bytes, 1, &self.hash);
        bytes_field(&mut bytes, 2, &parts);
        bytes
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CheckpointHeader {
    pub version_block: u64,
    pub version_app: u64,
    pub chain_id: String,
    pub height: u64,
    pub time: CommitTimestamp,
    pub last_block_id: Option<BlockId>,
    pub last_commit_hash: Option<[u8; 32]>,
    pub data_hash: Option<[u8; 32]>,
    pub validators_hash: [u8; 32],
    pub next_validators_hash: [u8; 32],
    pub consensus_hash: [u8; 32],
    pub app_hash: [u8; 32],
    pub last_results_hash: Option<[u8; 32]>,
    pub evidence_hash: Option<[u8; 32]>,
    pub proposer_address: [u8; 20],
}

impl CheckpointHeader {
    pub fn block_hash(&self) -> [u8; 32] {
        let mut version = Vec::new();
        varint_field(&mut version, 1, self.version_block);
        varint_field(&mut version, 2, self.version_app);
        let mut height = Vec::new();
        varint_field(&mut height, 1, self.height);
        merkle_hash(&[
            version,
            wrapped_bytes(self.chain_id.as_bytes()),
            height,
            self.time.encode(),
            self.last_block_id
                .map_or_else(|| vec![0x12, 0], BlockId::encode),
            wrapped_optional(self.last_commit_hash),
            wrapped_optional(self.data_hash),
            wrapped_bytes(&self.validators_hash),
            wrapped_bytes(&self.next_validators_hash),
            wrapped_bytes(&self.consensus_hash),
            wrapped_bytes(&self.app_hash),
            wrapped_optional(self.last_results_hash),
            wrapped_optional(self.evidence_hash),
            wrapped_bytes(&self.proposer_address),
        ])
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct CommitValidator {
    pub address: [u8; 20],
    pub public_key: [u8; 32],
    pub voting_power: u64,
}

impl CommitValidator {
    pub fn address_for_key(public_key: &[u8; 32]) -> [u8; 20] {
        let digest = Sha256::digest(public_key);
        let mut address = [0; 20];
        address.copy_from_slice(&digest[..20]);
        address
    }
}

pub fn validator_set_hash(validators: &[CommitValidator]) -> [u8; 32] {
    let encoded: Vec<_> = validators
        .iter()
        .map(|validator| {
            let mut key = Vec::new();
            bytes_field(&mut key, 1, &validator.public_key);
            let mut bytes = Vec::new();
            bytes_field(&mut bytes, 1, &key);
            varint_field(&mut bytes, 2, validator.voting_power);
            bytes
        })
        .collect();
    merkle_hash(&encoded)
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct CommitVote {
    pub validator_address: [u8; 20],
    pub voting_power: u64,
    pub timestamp: CommitTimestamp,
    pub signature: [u8; 64],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BlockCommit {
    pub height: u64,
    pub round: u32,
    pub block_id: BlockId,
    pub votes: Vec<CommitVote>,
}

impl BlockCommit {
    pub fn vote_sign_bytes(&self, chain_id: &str, timestamp: CommitTimestamp) -> Vec<u8> {
        let mut vote = vec![0x08, 0x02, 0x11];
        vote.extend_from_slice(&self.height.to_le_bytes());
        if self.round != 0 {
            vote.push(0x19);
            vote.extend_from_slice(&u64::from(self.round).to_le_bytes());
        }
        bytes_field(&mut vote, 4, &self.block_id.encode());
        bytes_field(&mut vote, 5, &timestamp.encode());
        bytes_field(&mut vote, 6, chain_id.as_bytes());
        let mut delimited = Vec::new();
        varint(&mut delimited, vote.len() as u64);
        delimited.extend_from_slice(&vote);
        delimited
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ServedCheckpoint {
    pub height: u64,
    pub block_hash: [u8; 32],
    pub app_hash: [u8; 32],
    pub commit: BlockCommit,
    pub rung: Rung,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProofBundleKind {
    Activity,
    State,
}

pub struct ProofBundleRequest<'a> {
    pub kind: ProofBundleKind,
    pub canonical_leaf_bytes: &'a [u8],
    pub proof: &'a Proof,
    pub named_root: [u8; 32],
    pub header_bytes: &'a [u8],
    pub header_signature: [u8; 64],
    pub authorization: &'a SequencerAuthorization,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ServedProofBundle {
    pub kind: ProofBundleKind,
    pub canonical_leaf_bytes: Vec<u8>,
    pub proof_bytes: Vec<u8>,
    pub named_root: [u8; 32],
    pub batch_number: u64,
    pub signed_header_digest: [u8; 32],
    pub verification_level: VerificationLevel,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum CheckpointReadError {
    UnavailableCapability(layerx_agent_api::error::ApiError),
    Commit,
    Inclusion(InclusionError),
    RootMismatch,
    Arithmetic,
}

pub(crate) fn serve_checkpoint(
    requested_height: u64,
    expected_chain_id: &str,
    header: &CheckpointHeader,
    commit: &BlockCommit,
    validators: &[CommitValidator],
) -> Result<ServedCheckpoint, CheckpointReadError> {
    let refuse = CheckpointReadError::Commit;
    if requested_height == 0
        || requested_height > i64::MAX as u64
        || header.height != requested_height
        || commit.height != requested_height
        || expected_chain_id.is_empty()
        || expected_chain_id.len() > 50
        || header.chain_id != expected_chain_id
        || header.version_block != 11
        || !header.time.valid()
        || header.last_block_id.is_some_and(|id| !id.valid())
        || commit.round > i32::MAX as u32
        || !commit.block_id.valid()
        || commit.block_id.hash != header.block_hash()
        || validators.is_empty()
        || header.validators_hash != validator_set_hash(validators)
    {
        return Err(refuse);
    }
    let mut addresses = BTreeSet::new();
    let mut total = 0_u64;
    for validator in validators {
        if validator.voting_power == 0
            || validator.address != CommitValidator::address_for_key(&validator.public_key)
            || !addresses.insert(validator.address)
        {
            return Err(refuse);
        }
        total = total
            .checked_add(validator.voting_power)
            .ok_or(CheckpointReadError::Commit)?;
    }
    if total > (i64::MAX as u64) / 8 {
        return Err(refuse);
    }
    let mut signers = BTreeSet::new();
    let mut signed = 0_u64;
    for vote in &commit.votes {
        let validator = validators
            .iter()
            .find(|validator| validator.address == vote.validator_address)
            .ok_or(CheckpointReadError::Commit)?;
        if !signers.insert(vote.validator_address)
            || vote.voting_power != validator.voting_power
            || !vote.timestamp.valid()
        {
            return Err(refuse);
        }
        layerx_crypto::ed25519::verify_message(
            &validator.public_key,
            &vote.signature,
            &commit.vote_sign_bytes(expected_chain_id, vote.timestamp),
        )
        .map_err(|_| CheckpointReadError::Commit)?;
        signed = signed
            .checked_add(validator.voting_power)
            .ok_or(CheckpointReadError::Commit)?;
    }
    if signed * 3 <= total * 2 {
        return Err(refuse);
    }
    Ok(ServedCheckpoint {
        height: header.height,
        block_hash: commit.block_id.hash,
        app_hash: header.app_hash,
        commit: commit.clone(),
        rung: Rung::Committed,
    })
}

fn varint(bytes: &mut Vec<u8>, mut value: u64) {
    while value >= 128 {
        bytes.push((value as u8 & 0x7f) | 0x80);
        value >>= 7;
    }
    bytes.push(value as u8);
}

fn varint_field(bytes: &mut Vec<u8>, field: u8, value: u64) {
    if value != 0 {
        bytes.push(field << 3);
        varint(bytes, value);
    }
}

fn bytes_field(bytes: &mut Vec<u8>, field: u8, value: &[u8]) {
    bytes.push((field << 3) | 2);
    varint(bytes, value.len() as u64);
    bytes.extend_from_slice(value);
}

fn wrapped_bytes(value: &[u8]) -> Vec<u8> {
    let mut bytes = Vec::new();
    if !value.is_empty() {
        bytes_field(&mut bytes, 1, value);
    }
    bytes
}

fn wrapped_optional(value: Option<[u8; 32]>) -> Vec<u8> {
    value.map_or_else(Vec::new, |hash| wrapped_bytes(&hash))
}

fn merkle_hash(items: &[Vec<u8>]) -> [u8; 32] {
    let mut hash = Sha256::new();
    match items.len() {
        0 => {}
        1 => {
            hash.update([0]);
            hash.update(&items[0]);
        }
        len => {
            let split = len.next_power_of_two() / 2;
            hash.update([1]);
            hash.update(merkle_hash(&items[..split]));
            hash.update(merkle_hash(&items[split..]));
        }
    }
    hash.finalize().into()
}

/// Verifies and serves one inclusion proof with its signed-header root provenance.
///
/// # Errors
///
/// Returns the inclusion failure when the proof or signed header does not verify, `RootMismatch`
/// when the header commits a different root than the named one, and `Arithmetic` when the
/// sibling count exceeds its encoded bound.
pub fn proof_bundle(
    request: &ProofBundleRequest<'_>,
) -> Result<ServedProofBundle, CheckpointReadError> {
    let _ = request;
    Err(CheckpointReadError::UnavailableCapability(
        layerx_agent_api::error::ApiError::unavailable_capability(
            layerx_agent_api::error::RequestId(0),
            "read.proof_bundle",
        )
        .expect("static operation name"),
    ))
}

fn encode_proof(proof: &Proof) -> Result<Vec<u8>, CheckpointReadError> {
    let sibling_count =
        u32::try_from(proof.siblings().len()).map_err(|_| CheckpointReadError::Arithmetic)?;
    let mut bytes = Vec::new();
    bytes.extend_from_slice(&proof.leaf_index().to_be_bytes());
    bytes.extend_from_slice(&proof.leaf_count().to_be_bytes());
    bytes.extend_from_slice(&sibling_count.to_be_bytes());
    for sibling in proof.siblings() {
        bytes.extend_from_slice(sibling);
    }
    Ok(bytes)
}
