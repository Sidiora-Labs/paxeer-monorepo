//! Block commit checkpoints and root-provenance proof bundles.

use std::collections::BTreeSet;

use layerx_agent_api::proof::ProofBundleTarget;
use layerx_client::sequencer::StorageWitness;
use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
use layerx_types::rung::Rung;

use crate::receipt::TransactionReceiptEvidence;
use sha2::{Digest as _, Sha256};

#[derive(Clone, Copy, Debug, Eq, PartialEq, serde::Serialize, serde::Deserialize)]
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

#[derive(Clone, Copy, Debug, Eq, PartialEq, serde::Serialize, serde::Deserialize)]
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

#[derive(Clone, Debug, Eq, PartialEq, serde::Serialize, serde::Deserialize)]
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
    pub header: CheckpointHeader,
    pub validators: Vec<CommitValidator>,
}

impl ServedCheckpoint {
    pub fn observed_block_hash(&self) -> [u8; 32] {
        self.header.block_hash()
    }

    /// Re-reads a saved, authenticated checkpoint from the configured node.
    /// The consensus identity and block remain pinned across reader restarts.
    pub fn reacquire(&self, endpoint: &SequencerEndpoint) -> Result<Self, CheckpointReadError> {
        let trusted = serve_checkpoint(
            self.height,
            &self.header.chain_id,
            &self.header,
            &self.commit,
            &self.validators,
        )?;
        if trusted != *self {
            return Err(CheckpointReadError::CheckpointBlockMismatch);
        }
        let observed = Self::acquire_at_committed_block(
            endpoint,
            self.height,
            Some(self.block_hash),
            &self.header.chain_id,
        )?;
        if observed != trusted {
            return Err(CheckpointReadError::CheckpointBlockMismatch);
        }
        Ok(observed)
    }

    pub fn acquire_at_committed_block(
        endpoint: &SequencerEndpoint,
        height: u64,
        identifier: Option<[u8; 32]>,
        expected_chain_id: &str,
    ) -> Result<Self, CheckpointReadError> {
        if identifier.is_none()
            && endpoint.block_number().map_err(CheckpointReadError::Rpc)? != height
        {
            return Err(CheckpointReadError::TargetMismatch);
        }
        let block = endpoint
            .block_by_number(height)
            .map_err(CheckpointReadError::Rpc)?
            .ok_or(CheckpointReadError::NotCommitted)?;
        let material = endpoint
            .commit_material(height)
            .map_err(CheckpointReadError::Rpc)?;
        let validator_set = endpoint
            .validator_set(height)
            .map_err(CheckpointReadError::Rpc)?;
        let validators = validator_set["validators"]
            .as_array()
            .ok_or(CheckpointReadError::Commit)?
            .iter()
            .map(|validator| {
                Ok(CommitValidator {
                    address: json_hex(&validator["address"])?,
                    public_key: json_base64(&validator["pubKey"]["value"])?,
                    voting_power: json_u64(&validator["votingPower"])?,
                })
            })
            .collect::<Result<Vec<_>, CheckpointReadError>>()?;
        let checkpoint = Self::from_comet_json(
            height,
            expected_chain_id,
            &material["signedHeader"],
            &validators,
        )?;
        let state_root = endpoint
            .state_root(height)
            .map_err(CheckpointReadError::Rpc)?;
        if block.hash != checkpoint.observed_block_hash()
            || state_root != checkpoint.app_hash
            || identifier.is_some_and(|id| id != block.hash)
            || u64::try_from(checkpoint.header.time.seconds).ok() != Some(block.timestamp)
        {
            return Err(CheckpointReadError::CheckpointBlockMismatch);
        }
        Ok(checkpoint)
    }

    /// Encodes the authenticated native header for the Programs chain verifier.
    pub fn commitment_proof(
        &self,
    ) -> Result<layerx_programs::chain_proof::decode::CommitmentProof, CheckpointReadError> {
        let verified = serve_checkpoint(
            self.height,
            &self.header.chain_id,
            &self.header,
            &self.commit,
            &self.validators,
        )?;
        if verified != *self {
            return Err(CheckpointReadError::CheckpointBlockMismatch);
        }
        let h = &self.header;
        let mut version = Vec::new();
        varint_field(&mut version, 1, h.version_block);
        varint_field(&mut version, 2, h.version_app);
        let mut header = Vec::new();
        bytes_field(&mut header, 1, &version);
        bytes_field(&mut header, 2, h.chain_id.as_bytes());
        varint_field(&mut header, 3, h.height);
        bytes_field(&mut header, 4, &h.time.encode());
        bytes_field(
            &mut header,
            5,
            &h.last_block_id
                .map_or_else(|| vec![0x12, 0], BlockId::encode),
        );
        for (field, bytes) in [
            (
                6,
                h.last_commit_hash
                    .as_ref()
                    .map_or(&[][..], |h| h.as_slice()),
            ),
            (7, h.data_hash.as_ref().map_or(&[][..], |h| h.as_slice())),
            (8, h.validators_hash.as_slice()),
            (9, h.next_validators_hash.as_slice()),
            (10, h.consensus_hash.as_slice()),
            (11, h.app_hash.as_slice()),
            (
                12,
                h.last_results_hash
                    .as_ref()
                    .map_or(&[][..], |h| h.as_slice()),
            ),
            (
                13,
                h.evidence_hash.as_ref().map_or(&[][..], |h| h.as_slice()),
            ),
            (14, h.proposer_address.as_slice()),
        ] {
            bytes_field(&mut header, field, bytes);
        }
        let mut commit = Vec::new();
        varint_field(&mut commit, 1, self.commit.height);
        varint_field(&mut commit, 2, u64::from(self.commit.round));
        bytes_field(&mut commit, 3, &self.commit.block_id.encode());
        let mut validators = Vec::new();
        let mut total = 0;
        let mut proposer = Vec::new();
        for validator in &self.validators {
            let mut signature = Vec::new();
            if let Some(vote) = self
                .commit
                .votes
                .iter()
                .find(|vote| vote.validator_address == validator.address)
            {
                varint_field(&mut signature, 1, 2);
                bytes_field(&mut signature, 2, &vote.validator_address);
                bytes_field(&mut signature, 3, &vote.timestamp.encode());
                bytes_field(&mut signature, 4, &vote.signature);
            } else {
                varint_field(&mut signature, 1, 1);
                bytes_field(
                    &mut signature,
                    3,
                    &CommitTimestamp {
                        seconds: -62_135_596_800,
                        nanos: 0,
                    }
                    .encode(),
                );
            }
            bytes_field(&mut commit, 4, &signature);
            let mut encoded = Vec::new();
            bytes_field(&mut encoded, 1, &validator.address);
            let mut key = Vec::new();
            bytes_field(&mut key, 1, &validator.public_key);
            bytes_field(&mut encoded, 2, &key);
            varint_field(&mut encoded, 3, validator.voting_power);
            bytes_field(&mut validators, 1, &encoded);
            if validator.address == h.proposer_address {
                proposer = encoded;
            }
            total += validator.voting_power;
        }
        bytes_field(&mut validators, 2, &proposer);
        varint_field(&mut validators, 3, total);
        Ok(layerx_programs::chain_proof::decode::CommitmentProof {
            signed_header: header,
            commit,
            validator_set: validators,
            store_name: "lxphost".to_owned(),
            key: Vec::new(),
            value: None,
            proof_ops: Vec::new(),
        })
    }

    pub(crate) fn from_comet_json(
        height: u64,
        expected_chain_id: &str,
        signed: &serde_json::Value,
        validators: &[CommitValidator],
    ) -> Result<Self, CheckpointReadError> {
        let h = &signed["header"];
        let optional = |key: &str| -> Result<Option<[u8; 32]>, CheckpointReadError> {
            match h[key].as_str() {
                Some("") | None if h[key].is_null() || h[key] == "" => Ok(None),
                _ => json_hex(&h[key]).map(Some),
            }
        };
        let header = CheckpointHeader {
            version_block: json_u64(&h["version"]["block"])?,
            version_app: json_u64(&h["version"]["app"])?,
            chain_id: h["chain_id"]
                .as_str()
                .ok_or(CheckpointReadError::Commit)?
                .to_owned(),
            height: json_u64(&h["height"])?,
            time: json_timestamp(&h["time"])?,
            last_block_id: if h["last_block_id"]["hash"] == "" {
                None
            } else {
                Some(json_block_id(&h["last_block_id"])?)
            },
            last_commit_hash: optional("last_commit_hash")?,
            data_hash: optional("data_hash")?,
            validators_hash: json_hex(&h["validators_hash"])?,
            next_validators_hash: json_hex(&h["next_validators_hash"])?,
            consensus_hash: json_hex(&h["consensus_hash"])?,
            app_hash: json_hex(&h["app_hash"])?,
            last_results_hash: optional("last_results_hash")?,
            evidence_hash: optional("evidence_hash")?,
            proposer_address: json_hex(&h["proposer_address"])?,
        };
        let c = &signed["commit"];
        let signatures = c["signatures"]
            .as_array()
            .ok_or(CheckpointReadError::Commit)?;
        if signatures.len() != validators.len() {
            return Err(CheckpointReadError::Commit);
        }
        let mut commit = BlockCommit {
            height: json_u64(&c["height"])?,
            round: u32::try_from(json_u64(&c["round"])?)
                .map_err(|_| CheckpointReadError::Commit)?,
            block_id: json_block_id(&c["block_id"])?,
            votes: Vec::new(),
        };
        for (signature, validator) in signatures.iter().zip(validators) {
            match json_u64(&signature["block_id_flag"])? {
                1 if signature["validator_address"] == "" && signature["signature"].is_null() => {}
                2 | 3 => {
                    if json_hex::<20>(&signature["validator_address"])? != validator.address {
                        return Err(CheckpointReadError::Commit);
                    }
                    let timestamp = json_timestamp(&signature["timestamp"])?;
                    let bytes = json_base64(&signature["signature"])?;
                    if json_u64(&signature["block_id_flag"])? == 2 {
                        commit.votes.push(CommitVote {
                            validator_address: validator.address,
                            voting_power: validator.voting_power,
                            timestamp,
                            signature: bytes,
                        });
                    }
                }
                _ => return Err(CheckpointReadError::Commit),
            }
        }
        serve_checkpoint(height, expected_chain_id, &header, &commit, validators)
    }
}

fn json_u64(value: &serde_json::Value) -> Result<u64, CheckpointReadError> {
    value
        .as_u64()
        .or_else(|| value.as_str()?.parse().ok())
        .ok_or(CheckpointReadError::Commit)
}

fn json_hex<const N: usize>(value: &serde_json::Value) -> Result<[u8; N], CheckpointReadError> {
    layerx_programs::hex::decode(value.as_str().ok_or(CheckpointReadError::Commit)?)
        .map_err(|_| CheckpointReadError::Commit)?
        .try_into()
        .map_err(|_| CheckpointReadError::Commit)
}

fn json_block_id(value: &serde_json::Value) -> Result<BlockId, CheckpointReadError> {
    Ok(BlockId {
        hash: json_hex(&value["hash"])?,
        part_set_total: u32::try_from(json_u64(&value["parts"]["total"])?)
            .map_err(|_| CheckpointReadError::Commit)?,
        part_set_hash: json_hex(&value["parts"]["hash"])?,
    })
}

fn json_base64<const N: usize>(value: &serde_json::Value) -> Result<[u8; N], CheckpointReadError> {
    let text = value
        .as_str()
        .ok_or(CheckpointReadError::Commit)?
        .as_bytes();
    let padding = (3 - N % 3) % 3;
    let encoded_len = N.div_ceil(3) * 4;
    let data_len = encoded_len - padding;
    if text.len() != encoded_len || text[data_len..].iter().any(|byte| *byte != b'=') {
        return Err(CheckpointReadError::Commit);
    }
    let mut bytes = Vec::new();
    let mut bits = 0_u32;
    let mut count = 0;
    for byte in &text[..data_len] {
        let digit = match byte {
            b'A'..=b'Z' => byte - b'A',
            b'a'..=b'z' => byte - b'a' + 26,
            b'0'..=b'9' => byte - b'0' + 52,
            b'+' => 62,
            b'/' => 63,
            _ => return Err(CheckpointReadError::Commit),
        };
        bits = (bits << 6) | u32::from(digit);
        count += 6;
        if count >= 8 {
            count -= 8;
            bytes.push((bits >> count) as u8);
        }
    }
    if bits & ((1 << count) - 1) != 0 {
        return Err(CheckpointReadError::Commit);
    }
    bytes.try_into().map_err(|_| CheckpointReadError::Commit)
}

fn json_timestamp(value: &serde_json::Value) -> Result<CommitTimestamp, CheckpointReadError> {
    let text = value.as_str().ok_or(CheckpointReadError::Commit)?;
    let bad = || CheckpointReadError::Commit;
    let number = |start, end| {
        let digits = text.get(start..end).ok_or_else(bad)?;
        if !digits.bytes().all(|byte| byte.is_ascii_digit()) {
            return Err(bad());
        }
        digits.parse::<i64>().map_err(|_| bad())
    };
    if text.len() < 20
        || text.get(4..5) != Some("-")
        || text.get(7..8) != Some("-")
        || text.get(10..11) != Some("T")
        || text.get(13..14) != Some(":")
        || text.get(16..17) != Some(":")
        || !text.ends_with('Z')
    {
        return Err(bad());
    }
    let (mut year, month, day) = (number(0, 4)?, number(5, 7)?, number(8, 10)?);
    let leap = year % 4 == 0 && (year % 100 != 0 || year % 400 == 0);
    let days = [
        31,
        if leap { 29 } else { 28 },
        31,
        30,
        31,
        30,
        31,
        31,
        30,
        31,
        30,
        31,
    ];
    if !(1..=12).contains(&month) || day < 1 || day > days[(month - 1) as usize] {
        return Err(bad());
    }
    let (hour, minute, second) = (number(11, 13)?, number(14, 16)?, number(17, 19)?);
    if hour > 23 || minute > 59 || second > 59 {
        return Err(bad());
    }
    let nanos = if text.len() == 20 {
        0
    } else {
        let fraction = text.get(20..text.len() - 1).ok_or_else(bad)?;
        if text.get(19..20) != Some(".")
            || fraction.is_empty()
            || fraction.len() > 9
            || !fraction.bytes().all(|b| b.is_ascii_digit())
        {
            return Err(bad());
        }
        fraction.parse::<u32>().map_err(|_| bad())? * 10_u32.pow(9 - fraction.len() as u32)
    };
    year -= i64::from(month <= 2);
    let era = year.div_euclid(400);
    let yoe = year - era * 400;
    let mp = month + if month > 2 { -3 } else { 9 };
    let doy = (153 * mp + 2) / 5 + day - 1;
    let days = era * 146097 + yoe * 365 + yoe / 4 - yoe / 100 + doy - 719468;
    let timestamp = CommitTimestamp {
        seconds: days * 86400 + hour * 3600 + minute * 60 + second,
        nanos,
    };
    if !timestamp.valid() {
        return Err(bad());
    }
    Ok(timestamp)
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProofBundleKind {
    Activity,
    Receipt,
    State,
}

#[derive(Clone, Copy)]
pub struct ProofBundleRequest<'a> {
    pub target: ProofBundleTarget,
    pub requested_rung: Rung,
    pub raw_transaction: &'a [u8],
    pub receipt: &'a TransactionReceiptEvidence,
    pub receipt_public_key: &'a [u8],
    pub receipt_proof: &'a [Vec<u8>],
    pub receipts_root: [u8; 32],
    pub chain_id: &'a str,
    pub header: &'a CheckpointHeader,
    pub commit: Option<&'a BlockCommit>,
    pub validators: &'a [CommitValidator],
    pub account_proof: Option<&'a [StorageWitness]>,
}

impl ProofBundleRequest<'_> {
    pub fn acquire(
        &self,
        endpoint: &SequencerEndpoint,
    ) -> Result<ServedProofBundle, CheckpointReadError> {
        super::proof_bundle_owner::acquire_at_committed_block(endpoint, self)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ServedProofBundle {
    pub format: &'static str,
    pub kind: ProofBundleKind,
    pub raw_transaction: Vec<u8>,
    pub receipt: TransactionReceiptEvidence,
    pub receipt_proof: Vec<Vec<u8>>,
    pub receipts_root: [u8; 32],
    pub block_header: CheckpointHeader,
    pub checkpoint: Option<ServedCheckpoint>,
    pub validator_set: Vec<CommitValidator>,
    pub account_proof: Option<super::proof_bundle_owner::AccountState>,
    pub state_root: Option<[u8; 32]>,
    pub verification_level: Rung,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum CheckpointReadError {
    UnavailableCapability(layerx_agent_api::error::ApiError),
    Commit,
    NotCommitted,
    InvalidProof,
    RootMismatch,
    TargetMismatch,
    CheckpointBlockMismatch,
    BelowRequestedRung,
    Arithmetic,
    Rpc(SequencerError),
}

impl CheckpointReadError {
    pub const fn reason(&self) -> &'static str {
        match self {
            Self::UnavailableCapability(_) => "read.unavailable",
            Self::NotCommitted => "read.not_committed",
            Self::Commit => "read.commit",
            Self::InvalidProof | Self::RootMismatch => "read.invalid_proof",
            Self::TargetMismatch => "read.target_mismatch",
            Self::CheckpointBlockMismatch => "read.checkpoint_block_mismatch",
            Self::BelowRequestedRung => "read.below_requested_rung",
            Self::Arithmetic => "read.proof_size",
            Self::Rpc(_) => "read.unavailable",
        }
    }
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
        header: header.clone(),
        validators: validators.to_vec(),
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

/// Verifies receipt-root inclusion and, for accounts, the committed state root.
///
/// # Errors
/// Refuses substituted receipts, roots, accounts, invalid commits and insufficient rungs.
pub fn proof_bundle(
    request: &ProofBundleRequest<'_>,
) -> Result<ServedProofBundle, CheckpointReadError> {
    super::proof_bundle_owner::verify_bundle(request)
}

#[cfg(test)]
impl ServedCheckpoint {
    pub(crate) fn test_fixture(height: u64, seconds: i64, nanos: u32) -> Self {
        tests::signed_checkpoint(height, seconds, nanos)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::{Signer as _, SigningKey};
    use serde_json::{json, Value};
    use std::io::{Read as _, Write as _};

    pub(super) fn signed_checkpoint(height: u64, seconds: i64, nanos: u32) -> ServedCheckpoint {
        let keys: Vec<_> = [1, 2, 3]
            .map(|seed| SigningKey::from_bytes(&[seed; 32]))
            .into();
        let validators: Vec<_> = keys
            .iter()
            .zip([4, 3, 2])
            .map(|(key, voting_power)| {
                let public_key = key.verifying_key().to_bytes();
                CommitValidator {
                    address: CommitValidator::address_for_key(&public_key),
                    public_key,
                    voting_power,
                }
            })
            .collect();
        let header = CheckpointHeader {
            version_block: 11,
            version_app: 1,
            chain_id: "125".into(),
            height,
            time: CommitTimestamp { seconds, nanos },
            last_block_id: Some(BlockId {
                hash: [1; 32],
                part_set_total: 1,
                part_set_hash: [2; 32],
            }),
            last_commit_hash: Some([3; 32]),
            data_hash: Some([4; 32]),
            validators_hash: validator_set_hash(&validators),
            next_validators_hash: validator_set_hash(&validators),
            consensus_hash: [5; 32],
            app_hash: [6; 32],
            last_results_hash: Some([7; 32]),
            evidence_hash: Some([8; 32]),
            proposer_address: validators[0].address,
        };
        let mut commit = BlockCommit {
            height,
            round: 1,
            block_id: BlockId {
                hash: header.block_hash(),
                part_set_total: 1,
                part_set_hash: [9; 32],
            },
            votes: Vec::new(),
        };
        for (key, validator) in keys.iter().zip(&validators).take(2) {
            let timestamp = CommitTimestamp { seconds, nanos };
            commit.votes.push(CommitVote {
                validator_address: validator.address,
                voting_power: validator.voting_power,
                timestamp,
                signature: key
                    .sign(&commit.vote_sign_bytes(&header.chain_id, timestamp))
                    .to_bytes(),
            });
        }
        serve_checkpoint(height, "125", &header, &commit, &validators).expect("signed block commit")
    }

    fn rpc_block(checkpoint: &ServedCheckpoint) -> Value {
        let hex = |bytes: &[u8]| format!("0x{}", layerx_programs::hex::encode(bytes));
        json!({"number": format!("{:#x}", checkpoint.height), "hash": hex(&checkpoint.block_hash),
            "parentHash": hex(&[1; 32]), "timestamp": format!("{:#x}", checkpoint.header.time.seconds),
            "transactionsRoot": hex(&[4; 32]), "receiptsRoot": hex(&[7; 32]),
            "stateRoot": hex(&checkpoint.app_hash), "transactions": []})
    }

    fn endpoint(
        answers: Vec<(&'static str, Value)>,
    ) -> (SequencerEndpoint, std::thread::JoinHandle<()>) {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").expect("bind RPC fixture");
        let address = listener.local_addr().expect("fixture address");
        let worker = std::thread::spawn(move || {
            for (method, answer) in answers {
                let (mut stream, _) = listener.accept().expect("RPC request");
                stream
                    .set_read_timeout(Some(std::time::Duration::from_secs(5)))
                    .expect("read deadline");
                let mut bytes = Vec::new();
                let (offset, length) = loop {
                    let mut byte = [0];
                    stream.read_exact(&mut byte).expect("request header");
                    bytes.push(byte[0]);
                    if bytes.ends_with(b"\r\n\r\n") {
                        let headers = String::from_utf8(bytes.clone()).expect("HTTP headers");
                        let length = headers
                            .lines()
                            .find_map(|line| {
                                let (name, value) = line.split_once(':')?;
                                name.eq_ignore_ascii_case("content-length")
                                    .then(|| value.trim().parse::<usize>().expect("length"))
                            })
                            .expect("request length");
                        break (bytes.len(), length);
                    }
                };
                bytes.resize(offset + length, 0);
                stream
                    .read_exact(&mut bytes[offset..])
                    .expect("request body");
                let request: Value = serde_json::from_slice(&bytes[offset..]).expect("JSON RPC");
                assert_eq!(request["method"], method);
                match method {
                    "eth_blockNumber" => assert_eq!(request["params"], json!([])),
                    "px_getCommit" | "px_getValidators" => {
                        assert_eq!(request["params"], json!(["8"]))
                    }
                    "eth_getBlockByNumber" => {
                        assert_eq!(request["params"][0], "0x8");
                        assert!(request["params"][1].is_boolean());
                    }
                    _ => panic!("unexpected RPC method {method}"),
                }
                let body =
                    json!({"jsonrpc": "2.0", "id": request["id"], "result": answer}).to_string();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).expect("RPC reply");
            }
        });
        (
            SequencerEndpoint::new(&format!("http://{address}"), &[]).expect("RPC endpoint"),
            worker,
        )
    }

    fn base64(bytes: &[u8]) -> String {
        const ALPHABET: &[u8; 64] =
            b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
        let mut encoded = String::new();
        for chunk in bytes.chunks(3) {
            let word = (u32::from(chunk[0]) << 16)
                | (u32::from(*chunk.get(1).unwrap_or(&0)) << 8)
                | u32::from(*chunk.get(2).unwrap_or(&0));
            encoded.push(ALPHABET[(word >> 18) as usize] as char);
            encoded.push(ALPHABET[((word >> 12) & 63) as usize] as char);
            encoded.push(if chunk.len() > 1 {
                ALPHABET[((word >> 6) & 63) as usize] as char
            } else {
                '='
            });
            encoded.push(if chunk.len() > 2 {
                ALPHABET[(word & 63) as usize] as char
            } else {
                '='
            });
        }
        encoded
    }

    fn rpc_commit(signed: &ServedCheckpoint) -> Value {
        assert_eq!(signed.header.time.seconds, 1_700_000_000);
        let timestamp = format!("2023-11-14T22:13:20.{:09}Z", signed.header.time.nanos);
        let hex = |bytes: &[u8]| layerx_programs::hex::encode(bytes);
        let id = |id: BlockId| {
            json!({"hash": hex(&id.hash), "parts": {
            "total": id.part_set_total, "hash": hex(&id.part_set_hash)}})
        };
        let h = &signed.header;
        let mut signatures: Vec<_> = signed
            .commit
            .votes
            .iter()
            .map(|vote| {
                json!({
                    "block_id_flag": 2, "validator_address": hex(&vote.validator_address),
                    "timestamp": timestamp, "signature": base64(&vote.signature),
                })
            })
            .collect();
        signatures.push(
            json!({"block_id_flag": 1, "validator_address": "", "signature": null,
            "timestamp": "0001-01-01T00:00:00Z"}),
        );
        json!({"header": {
            "version": {"block": "11", "app": "1"}, "chain_id": "125", "height": signed.height.to_string(),
            "time": timestamp, "last_block_id": id(h.last_block_id.expect("parent")),
            "last_commit_hash": hex(&[3;32]), "data_hash": hex(&[4;32]),
            "validators_hash": hex(&h.validators_hash), "next_validators_hash": hex(&h.next_validators_hash),
            "consensus_hash": hex(&[5;32]), "app_hash": hex(&[6;32]),
            "last_results_hash": hex(&[7;32]), "evidence_hash": hex(&[8;32]),
            "proposer_address": hex(&h.proposer_address),
        }, "commit": {"height": signed.height.to_string(), "round": 1,
            "block_id": id(signed.commit.block_id), "signatures": signatures}})
    }

    fn rpc_validators(signed: &ServedCheckpoint) -> Value {
        json!({"height": signed.height.to_string(), "total": signed.validators.len().to_string(),
            "validators": signed.validators.iter().map(|validator| json!({
                "address": layerx_programs::hex::encode(&validator.address),
                "pubKey": {"type": "tendermint/PubKeyEd25519", "value": base64(&validator.public_key)},
                "votingPower": validator.voting_power.to_string(),
            })).collect::<Vec<_>>()})
    }

    fn acquisition_answers(signed: &ServedCheckpoint, block: Value) -> Vec<(&'static str, Value)> {
        vec![
            ("eth_getBlockByNumber", block),
            (
                "px_getCommit",
                json!({"height": signed.height.to_string(), "signedHeader": rpc_commit(signed)}),
            ),
            ("px_getValidators", rpc_validators(signed)),
            ("eth_getBlockByNumber", rpc_block(signed)),
        ]
    }

    #[test]
    fn committed_checkpoint_decodes_the_comet_signed_header_and_uses_pinned_validators() {
        let signed = signed_checkpoint(8, 1_700_000_000, 123_000_000);
        let material = rpc_commit(&signed);
        assert_eq!(
            ServedCheckpoint::from_comet_json(8, "125", &material, &signed.validators),
            Ok(signed.clone())
        );
        let mut tampered = material.clone();
        tampered["header"]["time"] = json!("2023-11-14T22:13:21.123Z");
        assert!(
            ServedCheckpoint::from_comet_json(8, "125", &tampered, &signed.validators).is_err()
        );
        let mut validators = signed.validators;
        validators.reverse();
        assert!(ServedCheckpoint::from_comet_json(8, "125", &material, &validators).is_err());
        assert!(json_timestamp(&json!("2023-02-29T22:13:20Z")).is_err());
        assert!(json_timestamp(&json!("2023-11-14T22:13:60Z")).is_err());
    }

    #[test]
    fn committed_head_and_identifier_read_the_recorded_height() {
        let signed = signed_checkpoint(8, 1_700_000_000, 123_000_000);
        let mut answers = vec![
            ("eth_blockNumber", json!("0x8")),
            ("eth_blockNumber", json!("0x8")),
        ];
        answers.extend(acquisition_answers(&signed, rpc_block(&signed)));
        answers.extend(acquisition_answers(&signed, rpc_block(&signed)));
        let (endpoint, worker) = endpoint(answers);
        let height = endpoint.block_number().expect("committed height");
        let head = ServedCheckpoint::acquire_at_committed_block(&endpoint, height, None, "125")
            .expect("committed head");
        let by_id = ServedCheckpoint::acquire_at_committed_block(
            &endpoint,
            height,
            Some(head.block_hash),
            "125",
        )
        .expect("checkpoint identifier");
        assert_eq!(head, signed);
        assert_eq!(by_id, signed);
        assert_eq!(head.observed_block_hash(), signed.header.block_hash());
        worker.join().expect("RPC fixture");
    }

    #[test]
    fn committed_head_refuses_a_height_other_than_the_nodes_committed_head() {
        let (endpoint, worker) = endpoint(vec![("eth_blockNumber", json!("0x9"))]);
        assert_eq!(
            ServedCheckpoint::acquire_at_committed_block(&endpoint, 8, None, "125",),
            Err(CheckpointReadError::TargetMismatch)
        );
        worker.join().expect("RPC fixture");
    }

    #[test]
    fn committed_checkpoint_refuses_identifier_hash_time_and_height_substitution() {
        let signed = signed_checkpoint(8, 1_700_000_000, 0);
        for changed in ["identifier", "hash", "time", "height", "missing"] {
            let mut block = rpc_block(&signed);
            match changed {
                "hash" => block["hash"] = json!(format!("0x{}", "aa".repeat(32))),
                "time" => block["timestamp"] = json!("0x1"),
                "height" => block["number"] = json!("0x9"),
                "missing" => block = Value::Null,
                _ => {}
            }
            let answers = if changed == "height" || changed == "missing" {
                vec![("eth_getBlockByNumber", block)]
            } else {
                acquisition_answers(&signed, block)
            };
            let (endpoint, worker) = endpoint(answers);
            let identifier = if changed == "identifier" {
                [0xaa; 32]
            } else {
                signed.block_hash
            };
            assert!(
                ServedCheckpoint::acquire_at_committed_block(
                    &endpoint,
                    8,
                    Some(identifier),
                    "125",
                )
                .is_err(),
                "{changed}"
            );
            worker.join().expect("RPC fixture");
        }
    }

    #[test]
    fn committed_checkpoint_refuses_block_hash_and_state_root_mismatch() {
        let signed = signed_checkpoint(8, 1_700_000_000, 0);
        for changed in ["hash", "stateRoot"] {
            let mut answers = acquisition_answers(&signed, rpc_block(&signed));
            let index = if changed == "hash" { 0 } else { 3 };
            answers[index].1[changed] = json!(format!("0x{}", "aa".repeat(32)));
            let (endpoint, worker) = endpoint(answers);
            let error = ServedCheckpoint::acquire_at_committed_block(
                &endpoint,
                8,
                Some(signed.block_hash),
                "125",
            )
            .expect_err("block and signed checkpoint must agree");
            assert_eq!(error, CheckpointReadError::CheckpointBlockMismatch);
            assert_eq!(error.reason(), "read.checkpoint_block_mismatch");
            worker.join().expect("RPC fixture");
        }
    }

    #[test]
    fn committed_checkpoint_accessors_refuse_null_and_malformed_material() {
        let signed = signed_checkpoint(8, 1_700_000_000, 0);
        for changed in ["null", "height", "root"] {
            let mut block = rpc_block(&signed);
            match changed {
                "null" => block["stateRoot"] = Value::Null,
                "height" => block["number"] = json!("0x9"),
                "root" => block["stateRoot"] = json!("0x01"),
                _ => unreachable!(),
            }
            let (endpoint, worker) = endpoint(vec![("eth_getBlockByNumber", block)]);
            assert_eq!(endpoint.state_root(8), Err(SequencerError::Malformed));
            worker.join().expect("RPC fixture");
        }
        let (endpoint, worker) = endpoint(vec![(
            "px_getCommit",
            json!({"height": "9", "signedHeader": rpc_commit(&signed)}),
        )]);
        assert_eq!(endpoint.commit_material(8), Err(SequencerError::Malformed));
        worker.join().expect("RPC fixture");
        let mut validators = rpc_validators(&signed);
        validators["total"] = json!("4");
        let (endpoint, worker) = self::endpoint(vec![("px_getValidators", validators)]);
        assert_eq!(endpoint.validator_set(8), Err(SequencerError::Malformed));
        worker.join().expect("RPC fixture");
    }

    #[test]
    fn committed_checkpoint_refuses_invalid_signature_and_insufficient_power() {
        let signed = signed_checkpoint(8, 1_700_000_000, 0);
        let mut commit = signed.commit.clone();
        commit.votes[0].signature[0] ^= 1;
        assert_eq!(
            serve_checkpoint(8, "125", &signed.header, &commit, &signed.validators),
            Err(CheckpointReadError::Commit)
        );
        let mut commit = signed.commit.clone();
        commit.votes.pop();
        assert_eq!(
            serve_checkpoint(8, "125", &signed.header, &commit, &signed.validators),
            Err(CheckpointReadError::Commit)
        );
        assert_eq!(
            serve_checkpoint(9, "125", &signed.header, &signed.commit, &signed.validators),
            Err(CheckpointReadError::Commit)
        );
        assert_eq!(
            serve_checkpoint(
                8,
                "other-chain",
                &signed.header,
                &signed.commit,
                &signed.validators
            ),
            Err(CheckpointReadError::Commit)
        );
    }
}
