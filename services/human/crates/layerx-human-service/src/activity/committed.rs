use layerx_agentd::read::{
    checkpoint, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator,
    CommitVote,
};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_proof::merkle::{decode_proof, encode_proof, Proof};
use layerx_wire::receipt::ProtocolReceipt;
use sha2::{Digest as _, Sha256};

use super::{
    pinned_authorization, push_bytes, push_u32, BundleReader, ExportError, UnverifiedReason,
    VerifyError,
};

const BATCH_DOMAIN: &[u8] = b"PAXEER/v1/sequence-batch\0";
const SEQUENCE_DOMAIN: &[u8] = b"PAXEER/v1/sequence\0";

/// Independently installed consensus authority, never taken from an export.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ConsensusAuthority {
    pub chain_id: String,
    pub first_height: u64,
    pub last_height: u64,
    pub validators: Vec<CommitValidator>,
}

/// The actual transaction at block index zero and its consensus inclusion.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommittedReceiptEvidence {
    pub header: CheckpointHeader,
    pub execution_header: CheckpointHeader,
    pub commit: BlockCommit,
    pub batch_transaction: Vec<u8>,
    pub batch_total: u64,
    pub batch_aunts: Vec<[u8; 32]>,
    pub activity_ids: Vec<[u8; 32]>,
}

/// Binds a selected receipt to the exact signed header used by inclusion facts.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReceiptHeaderBinding {
    pub canonical_header_bytes: Vec<u8>,
    pub header_signature: [u8; 64],
    pub receipt_proof: Proof,
}

fn invalid() -> VerifyError {
    VerifyError::Unverified(UnverifiedReason::Tampered)
}

impl CommittedReceiptEvidence {
    pub(super) fn verify(
        &self,
        receipt: &ProtocolReceipt,
        receipt_height: u64,
        keys: &[ReceiptKeyEntry],
        authorities: &[ConsensusAuthority],
    ) -> Result<(), VerifyError> {
        let matching: Vec<_> = authorities
            .iter()
            .filter(|authority| {
                authority.chain_id == self.header.chain_id
                    && authority.first_height <= self.header.height
                    && self.header.height <= authority.last_height
            })
            .collect();
        if matching.is_empty() {
            return Err(VerifyError::Unavailable(
                ExportError::AuthorityUnavailable {
                    reference: format!("consensus:{}:{}", self.header.chain_id, self.header.height),
                },
            ));
        }
        if matching.len() != 1 {
            return Err(VerifyError::Unverified(UnverifiedReason::AuthorityMismatch));
        }
        checkpoint(
            self.header.height,
            &matching[0].chain_id,
            &self.header,
            &self.commit,
            &matching[0].validators,
        )
        .map_err(|_| invalid())?;
        if self.execution_header.height == 0
            || self.execution_header.height.checked_add(1) != Some(self.header.height)
            || self.execution_header.chain_id != self.header.chain_id
            || self.execution_header.next_validators_hash != self.header.validators_hash
            || self.header.last_block_id.map(|id| id.hash)
                != Some(self.execution_header.block_hash())
        {
            return Err(invalid());
        }
        if self.batch_total == 0
            || self.batch_total > i64::MAX as u64
            || self.batch_total.checked_sub(1) != Some(self.activity_ids.len() as u64)
            || self.batch_aunts.len() > 63
        {
            return Err(invalid());
        }
        let digest: [u8; 32] = Sha256::digest(&self.batch_transaction).into();
        let mut leaf = Sha256::new();
        leaf.update([0]);
        leaf.update(digest);
        let root = first_leaf_root(self.batch_total, leaf.finalize().into(), &self.batch_aunts)?;
        if self.execution_header.data_hash != Some(root) {
            return Err(invalid());
        }
        let body = self
            .batch_transaction
            .strip_prefix(BATCH_DOMAIN)
            .filter(|body| body.len() == 216)
            .ok_or_else(invalid)?;
        let mut reader = BundleReader {
            bytes: body,
            offset: 0,
        };
        let mut read = || {
            reader
                .array::<8>()
                .map(u64::from_be_bytes)
                .map_err(VerifyError::from)
        };
        let (height, first, last) = (read()?, read()?, read()?);
        let previous: [u8; 32] = reader.array().map_err(VerifyError::from)?;
        let resulting: [u8; 32] = reader.array().map_err(VerifyError::from)?;
        let tx_root: [u8; 32] = reader.array().map_err(VerifyError::from)?;
        let key_id = reader.array().map_err(VerifyError::from)?;
        let signature = reader.array().map_err(VerifyError::from)?;
        if height != self.execution_header.height
            || receipt_height == 0
            || receipt_height > height
            || first == 0
            || last.checked_sub(first).and_then(|n| n.checked_add(1))
                != Some(self.activity_ids.len() as u64)
            || !(first..=last).contains(&receipt.global_sequence())
        {
            return Err(invalid());
        }
        let authorization = pinned_authorization(key_id, height, keys)?;
        let mut message = Sha256::new();
        message.update(BATCH_DOMAIN);
        message.update(self.header.chain_id.as_bytes());
        message.update(&body[..120]);
        layerx_crypto::ed25519::verify_digest(
            &authorization.public_key(),
            &signature,
            &message.finalize().into(),
        )
        .map_err(|_| invalid())?;
        self.verify_activity_chain(first, previous, resulting, tx_root, receipt)
    }

    fn verify_activity_chain(
        &self,
        first: u64,
        previous: [u8; 32],
        resulting: [u8; 32],
        tx_root: [u8; 32],
        receipt: &ProtocolReceipt,
    ) -> Result<(), VerifyError> {
        let mut activities = Sha256::new();
        let mut chain = previous;
        for (index, id) in self.activity_ids.iter().enumerate() {
            let sequence = first.checked_add(index as u64).ok_or_else(invalid)?;
            let before = chain;
            let mut hash = Sha256::new();
            hash.update(SEQUENCE_DOMAIN);
            hash.update(before);
            hash.update(sequence.to_be_bytes());
            hash.update(id);
            chain = hash.finalize().into();
            activities.update(id);
            if sequence == receipt.global_sequence()
                && (*id != receipt.activity_id()
                    || before != receipt.previous_state_root()
                    || chain != receipt.resulting_state_root())
            {
                return Err(invalid());
            }
        }
        if chain != resulting || <[u8; 32]>::from(activities.finalize()) != tx_root {
            return Err(invalid());
        }
        Ok(())
    }

    pub(super) fn encode(&self, out: &mut Vec<u8>) -> Result<(), ExportError> {
        encode_header(out, &self.header)?;
        encode_header(out, &self.execution_header)?;
        out.extend_from_slice(&self.commit.height.to_be_bytes());
        out.extend_from_slice(&self.commit.round.to_be_bytes());
        encode_block_id(out, self.commit.block_id);
        push_u32(out, self.commit.votes.len())?;
        for vote in &self.commit.votes {
            out.extend_from_slice(&vote.validator_address);
            out.extend_from_slice(&vote.voting_power.to_be_bytes());
            encode_time(out, vote.timestamp);
            out.extend_from_slice(&vote.signature);
        }
        push_bytes(out, &self.batch_transaction)?;
        out.extend_from_slice(&self.batch_total.to_be_bytes());
        for hashes in [&self.batch_aunts, &self.activity_ids] {
            push_u32(out, hashes.len())?;
            for hash in hashes {
                out.extend_from_slice(hash);
            }
        }
        Ok(())
    }

    pub(super) fn decode(reader: &mut BundleReader<'_>) -> Result<Self, ExportError> {
        let header = decode_header(reader)?;
        let execution_header = decode_header(reader)?;
        let height = u64::from_be_bytes(reader.array()?);
        let round = u32::from_be_bytes(reader.array()?);
        let block_id = decode_block_id(reader)?;
        let count = reader.count()?;
        let mut votes = Vec::new();
        for _ in 0..count {
            votes.push(CommitVote {
                validator_address: reader.array()?,
                voting_power: u64::from_be_bytes(reader.array()?),
                timestamp: decode_time(reader)?,
                signature: reader.array()?,
            });
        }
        let batch_transaction = reader.bytes()?.to_vec();
        let batch_total = u64::from_be_bytes(reader.array()?);
        let mut hashes = || -> Result<Vec<[u8; 32]>, ExportError> {
            let count = reader.count()?;
            (0..count).map(|_| reader.array()).collect()
        };
        let batch_aunts = hashes()?;
        let activity_ids = hashes()?;
        Ok(Self {
            header,
            execution_header,
            commit: BlockCommit {
                height,
                round,
                block_id,
                votes,
            },
            batch_transaction,
            batch_total,
            batch_aunts,
            activity_ids,
        })
    }
}

// Consensus proof aunts run from the leaf to the root; tx zero is always on
// the left. The exact split sizes reject extra or missing aunts.
fn first_leaf_root(
    total: u64,
    leaf: [u8; 32],
    aunts: &[[u8; 32]],
) -> Result<[u8; 32], VerifyError> {
    if total == 1 {
        return if aunts.is_empty() {
            Ok(leaf)
        } else {
            Err(invalid())
        };
    }
    let (right, rest) = aunts.split_last().ok_or_else(invalid)?;
    let split = 1_u64 << (total - 1).ilog2();
    let left = first_leaf_root(split, leaf, rest)?;
    let mut hash = Sha256::new();
    hash.update([1]);
    hash.update(left);
    hash.update(right);
    Ok(hash.finalize().into())
}

impl ReceiptHeaderBinding {
    pub(super) fn encode(&self, out: &mut Vec<u8>) -> Result<(), ExportError> {
        push_bytes(out, &self.canonical_header_bytes)?;
        out.extend_from_slice(&self.header_signature);
        push_bytes(out, &encode_proof(&self.receipt_proof))
    }
    pub(super) fn decode(reader: &mut BundleReader<'_>) -> Result<Self, ExportError> {
        Ok(Self {
            canonical_header_bytes: reader.bytes()?.to_vec(),
            header_signature: reader.array()?,
            receipt_proof: decode_proof(reader.bytes()?)
                .map_err(|_| ExportError::UnboundProtocolEvidence)?,
        })
    }
}

fn encode_time(out: &mut Vec<u8>, time: CommitTimestamp) {
    out.extend_from_slice(&time.seconds.to_be_bytes());
    out.extend_from_slice(&time.nanos.to_be_bytes());
}
fn decode_time(reader: &mut BundleReader<'_>) -> Result<CommitTimestamp, ExportError> {
    Ok(CommitTimestamp {
        seconds: i64::from_be_bytes(reader.array()?),
        nanos: u32::from_be_bytes(reader.array()?),
    })
}
fn encode_block_id(out: &mut Vec<u8>, id: BlockId) {
    out.extend_from_slice(&id.hash);
    out.extend_from_slice(&id.part_set_total.to_be_bytes());
    out.extend_from_slice(&id.part_set_hash);
}
fn decode_block_id(reader: &mut BundleReader<'_>) -> Result<BlockId, ExportError> {
    Ok(BlockId {
        hash: reader.array()?,
        part_set_total: u32::from_be_bytes(reader.array()?),
        part_set_hash: reader.array()?,
    })
}
fn encode_header(out: &mut Vec<u8>, h: &CheckpointHeader) -> Result<(), ExportError> {
    out.extend_from_slice(&h.version_block.to_be_bytes());
    out.extend_from_slice(&h.version_app.to_be_bytes());
    push_bytes(out, h.chain_id.as_bytes())?;
    out.extend_from_slice(&h.height.to_be_bytes());
    encode_time(out, h.time);
    match h.last_block_id {
        Some(id) => {
            out.push(1);
            encode_block_id(out, id);
        }
        None => out.push(0),
    }
    for hash in [h.last_commit_hash, h.data_hash] {
        encode_optional_hash(out, hash);
    }
    for hash in [
        h.validators_hash,
        h.next_validators_hash,
        h.consensus_hash,
        h.app_hash,
    ] {
        out.extend_from_slice(&hash);
    }
    for hash in [h.last_results_hash, h.evidence_hash] {
        encode_optional_hash(out, hash);
    }
    out.extend_from_slice(&h.proposer_address);
    Ok(())
}
fn decode_header(r: &mut BundleReader<'_>) -> Result<CheckpointHeader, ExportError> {
    Ok(CheckpointHeader {
        version_block: u64::from_be_bytes(r.array()?),
        version_app: u64::from_be_bytes(r.array()?),
        chain_id: r.string()?,
        height: u64::from_be_bytes(r.array()?),
        time: decode_time(r)?,
        last_block_id: match r.byte()? {
            0 => None,
            1 => Some(decode_block_id(r)?),
            _ => return Err(ExportError::UnboundProtocolEvidence),
        },
        last_commit_hash: decode_optional_hash(r)?,
        data_hash: decode_optional_hash(r)?,
        validators_hash: r.array()?,
        next_validators_hash: r.array()?,
        consensus_hash: r.array()?,
        app_hash: r.array()?,
        last_results_hash: decode_optional_hash(r)?,
        evidence_hash: decode_optional_hash(r)?,
        proposer_address: r.array()?,
    })
}
fn encode_optional_hash(out: &mut Vec<u8>, hash: Option<[u8; 32]>) {
    match hash {
        Some(hash) => {
            out.push(1);
            out.extend_from_slice(&hash);
        }
        None => out.push(0),
    }
}
fn decode_optional_hash(r: &mut BundleReader<'_>) -> Result<Option<[u8; 32]>, ExportError> {
    match r.byte()? {
        0 => Ok(None),
        1 => r.array().map(Some),
        _ => Err(ExportError::UnboundProtocolEvidence),
    }
}
