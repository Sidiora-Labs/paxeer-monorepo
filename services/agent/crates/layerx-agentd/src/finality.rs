//! Receipt-key and consensus-commit gated submission finality.

use std::collections::BTreeSet;

use layerx_client::sequencer::{ReceiptKeyEntry, SequencerBlock, SequencerReceipt};
use layerx_crypto::ed25519;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_types::rung::Rung;
use sha2::{Digest, Sha256};

use crate::node_rpc::{NodeRpc, NodeRpcError};
use crate::store::{ObjectKind, Store, StoreError, TenantId, TenantKey};

const RECORD_MAGIC: &[u8; 4] = b"LXF2";

#[derive(Debug)]
pub enum FinalityError {
    Store(StoreError),
    Rpc(NodeRpcError),
    InvalidReceipt,
    InvalidCommit,
    InvalidDeadline,
    ProgressUnavailable,
    Arithmetic,
    Corrupt,
}

impl From<StoreError> for FinalityError {
    fn from(value: StoreError) -> Self {
        Self::Store(value)
    }
}

impl From<NodeRpcError> for FinalityError {
    fn from(value: NodeRpcError) -> Self {
        Self::Rpc(value)
    }
}

/// A validator from the independently trusted consensus validator set at the receipt height.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Validator {
    pub public_key: [u8; 32],
    pub voting_power: u64,
}

/// One CometBFT precommit, including its per-validator timestamp.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommitSignature {
    pub validator_address: [u8; 20],
    pub timestamp_seconds: i64,
    pub timestamp_nanos: u32,
    pub signature: [u8; 64],
}

/// A CometBFT commit for the block returned by `eth_getBlockByNumber`.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BlockCommit {
    pub height: u64,
    pub round: u32,
    pub block_hash: [u8; 32],
    pub part_set_total: u32,
    pub part_set_hash: [u8; 32],
    pub signatures: Vec<CommitSignature>,
}

impl BlockCommit {
    /// Produces the length-delimited protobuf CanonicalVote bytes signed by CometBFT.
    pub fn vote_sign_bytes(
        &self,
        chain_id: &str,
        seconds: i64,
        nanos: u32,
    ) -> Result<Vec<u8>, FinalityError> {
        if chain_id.is_empty()
            || chain_id.len() > 50
            || self.height == 0
            || self.height > i64::MAX as u64
            || self.round > i32::MAX as u32
            || nanos >= 1_000_000_000
            || !(-62_135_596_800..=253_402_300_799).contains(&seconds)
            || self.part_set_total == 0
            || self.block_hash == [0; 32]
            || self.part_set_hash == [0; 32]
        {
            return Err(FinalityError::InvalidCommit);
        }
        let mut parts = vec![8];
        varint(&mut parts, u64::from(self.part_set_total));
        field(&mut parts, 18, &self.part_set_hash)?;
        let mut block = Vec::new();
        field(&mut block, 10, &self.block_hash)?;
        field(&mut block, 18, &parts)?;
        let mut timestamp = Vec::new();
        if seconds != 0 {
            timestamp.push(8);
            varint(&mut timestamp, seconds.cast_unsigned());
        }
        if nanos != 0 {
            timestamp.push(16);
            varint(&mut timestamp, u64::from(nanos));
        }
        let mut vote = vec![8, 2, 17];
        vote.extend_from_slice(&self.height.to_le_bytes());
        if self.round != 0 {
            vote.push(25);
            vote.extend_from_slice(&u64::from(self.round).to_le_bytes());
        }
        field(&mut vote, 34, &block)?;
        field(&mut vote, 42, &timestamp)?;
        field(&mut vote, 50, chain_id.as_bytes())?;
        let mut delimited = Vec::new();
        varint(
            &mut delimited,
            u64::try_from(vote.len()).map_err(|_| FinalityError::Arithmetic)?,
        );
        delimited.extend(vote);
        Ok(delimited)
    }

    fn verify(
        &self,
        block: &SequencerBlock,
        chain_id: &str,
        validators: &[Validator],
    ) -> Result<(), FinalityError> {
        if self.height != block.number || self.block_hash != block.hash || validators.is_empty() {
            return Err(FinalityError::InvalidCommit);
        }
        let mut addresses = BTreeSet::new();
        let mut total = 0_u128;
        for validator in validators {
            if validator.voting_power == 0
                || !addresses.insert(validator_address(&validator.public_key))
            {
                return Err(FinalityError::InvalidCommit);
            }
            total = total
                .checked_add(u128::from(validator.voting_power))
                .ok_or(FinalityError::Arithmetic)?;
        }
        let mut seen = BTreeSet::new();
        let mut signed = 0_u128;
        for signature in &self.signatures {
            if !seen.insert(signature.validator_address) {
                return Err(FinalityError::InvalidCommit);
            }
            let validator = validators
                .iter()
                .find(|validator| {
                    validator_address(&validator.public_key) == signature.validator_address
                })
                .ok_or(FinalityError::InvalidCommit)?;
            let bytes = self.vote_sign_bytes(
                chain_id,
                signature.timestamp_seconds,
                signature.timestamp_nanos,
            )?;
            ed25519::verify_message(&validator.public_key, &signature.signature, &bytes)
                .map_err(|_| FinalityError::InvalidCommit)?;
            signed = signed
                .checked_add(u128::from(validator.voting_power))
                .ok_or(FinalityError::Arithmetic)?;
        }
        if signed.checked_mul(3).ok_or(FinalityError::Arithmetic)?
            <= total.checked_mul(2).ok_or(FinalityError::Arithmetic)?
        {
            return Err(FinalityError::InvalidCommit);
        }
        Ok(())
    }
}

#[must_use]
pub fn validator_address(public_key: &[u8; 32]) -> [u8; 20] {
    let digest = Sha256::digest(public_key);
    let mut address = [0; 20];
    address.copy_from_slice(&digest[..20]);
    address
}

/// Receipt evidence whose signature and published height-bound key have been verified.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedReceipt {
    canonical_bytes: Vec<u8>,
    tx_hash: [u8; 32],
    height: u64,
    key: ReceiptKeyEntry,
}

impl VerifiedReceipt {
    pub fn verify(
        tx_hash: [u8; 32],
        receipt: &[u8],
        keys: &[ReceiptKeyEntry],
    ) -> Result<Self, FinalityError> {
        if keys
            .iter()
            .any(|key| key.key_id == [0; 32] || key.first_height > key.last_height)
        {
            return Err(FinalityError::InvalidReceipt);
        }
        let mut ids = BTreeSet::new();
        if keys.iter().any(|key| !ids.insert(key.key_id)) {
            return Err(FinalityError::InvalidReceipt);
        }
        let pins: Vec<_> = keys
            .iter()
            .map(|key| {
                SequencerAuthorization::new(
                    key.key_id,
                    key.public_key,
                    key.first_height,
                    key.last_height,
                )
            })
            .collect();
        let decoded = verify_sequencer_signature(receipt, pins.as_slice())
            .map_err(|_| FinalityError::InvalidReceipt)?;
        let protocol = decoded.protocol().ok_or(FinalityError::InvalidReceipt)?;
        let batch = protocol.batch_id();
        if protocol.activity_id() != tx_hash || batch[..24] != [0; 24] || protocol.module_id() != 12
        {
            return Err(FinalityError::InvalidReceipt);
        }
        let height = u64::from_be_bytes(
            batch[24..]
                .try_into()
                .map_err(|_| FinalityError::InvalidReceipt)?,
        );
        Ok(Self {
            canonical_bytes: receipt.to_vec(),
            tx_hash,
            height,
            key: *keys
                .iter()
                .find(|key| key.key_id == protocol.context_hash())
                .ok_or(FinalityError::InvalidReceipt)?,
        })
    }

    #[must_use]
    pub fn canonical_bytes(&self) -> &[u8] {
        &self.canonical_bytes
    }
}

/// Commit verified against a separately pinned chain id and validator set.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedBlock {
    block: SequencerBlock,
    commit: BlockCommit,
    chain_id: String,
    validators: Vec<Validator>,
}

impl VerifiedBlock {
    pub fn verify(
        block: &SequencerBlock,
        commit: &BlockCommit,
        chain_id: &str,
        validators: &[Validator],
    ) -> Result<Self, FinalityError> {
        commit.verify(block, chain_id, validators)?;
        Ok(Self {
            block: block.clone(),
            commit: commit.clone(),
            chain_id: chain_id.to_owned(),
            validators: validators.to_vec(),
        })
    }
}

/// Exact receipt and consensus evidence, kept outside the original signed bytes.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FinalityRecord {
    pub idempotency_key: [u8; 32],
    pub tx_hash: [u8; 32],
    pub rung: Rung,
    pub receipt_bytes: Vec<u8>,
    pub receipt_key: ReceiptKeyEntry,
    pub height: u64,
    pub block: Option<VerifiedBlock>,
}

pub struct ReceiptEvidence<'a> {
    pub tx_hash: [u8; 32],
    pub canonical_bytes: &'a [u8],
    pub published_keys: &'a [ReceiptKeyEntry],
}

pub fn augment(
    durable: &mut Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
    receipt: &ReceiptEvidence<'_>,
    block: Option<(&SequencerBlock, &BlockCommit)>,
    chain_id: &str,
    validators: &[Validator],
) -> Result<FinalityRecord, FinalityError> {
    let receipt = VerifiedReceipt::verify(
        receipt.tx_hash,
        receipt.canonical_bytes,
        receipt.published_keys,
    )?;
    let verified = block
        .map(|(block, commit)| VerifiedBlock::verify(block, commit, chain_id, validators))
        .transpose()?;
    augment_verified(
        durable,
        tenant,
        idempotency_key,
        &receipt,
        verified.as_ref(),
    )
}

pub fn augment_verified(
    durable: &mut Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
    receipt: &VerifiedReceipt,
    block: Option<&VerifiedBlock>,
) -> Result<FinalityRecord, FinalityError> {
    if block.is_some_and(|block| block.block.number != receipt.height) {
        return Err(FinalityError::InvalidCommit);
    }
    let record = FinalityRecord {
        idempotency_key,
        tx_hash: receipt.tx_hash,
        rung: if block.is_some() {
            Rung::Committed
        } else {
            Rung::Receipted
        },
        receipt_bytes: receipt.canonical_bytes.clone(),
        receipt_key: receipt.key,
        height: receipt.height,
        block: block.cloned(),
    };
    let key = record_key(tenant, idempotency_key)?;
    let bytes = encode_record(&record)?;
    if let Some(previous) = durable.get(&key) {
        let previous = previous.bytes();
        let receipt_end = 161_usize
            .checked_add(record.receipt_bytes.len())
            .ok_or(FinalityError::Arithmetic)?;
        if previous.len() <= receipt_end
            || previous.get(..68) != bytes.get(..68)
            || previous.get(69..receipt_end) != bytes.get(69..receipt_end)
            || previous[68] > record.rung.wire_rank()
            || (previous[68] == Rung::Committed.wire_rank()
                && previous.get(receipt_end..receipt_end + 33)
                    != bytes.get(receipt_end..receipt_end + 33))
        {
            return Err(FinalityError::Corrupt);
        }
    }
    durable.put_local(key, bytes)?;
    Ok(record)
}

/// A submission hash accepted by the sequencer, with independently checked evidence.
#[derive(Clone, Debug)]
pub struct SubmissionProgress {
    idempotency_key: [u8; 32],
    tx_hash: [u8; 32],
    receipt: Option<VerifiedReceipt>,
    block: Option<VerifiedBlock>,
}

impl SubmissionProgress {
    /// The hash must be the successful result of `eth_sendRawTransaction`.
    #[must_use]
    pub fn accepted(idempotency_key: [u8; 32], tx_hash: [u8; 32]) -> Self {
        Self {
            idempotency_key,
            tx_hash,
            receipt: None,
            block: None,
        }
    }

    pub fn submit(
        node: &NodeRpc,
        idempotency_key: [u8; 32],
        raw: &[u8],
    ) -> Result<Self, FinalityError> {
        Ok(Self::accepted(
            idempotency_key,
            node.send_raw_transaction(raw)?,
        ))
    }

    /// A failed check leaves the prior verified rung intact.
    pub fn observe_receipt(
        &mut self,
        receipt: &[u8],
        keys: &[ReceiptKeyEntry],
    ) -> Result<(), FinalityError> {
        let verified = VerifiedReceipt::verify(self.tx_hash, receipt, keys)?;
        self.retain_receipt(verified)
    }

    fn retain_receipt(&mut self, verified: VerifiedReceipt) -> Result<(), FinalityError> {
        if self
            .receipt
            .as_ref()
            .is_some_and(|previous| previous != &verified)
        {
            return Err(FinalityError::Corrupt);
        }
        self.receipt = Some(verified);
        Ok(())
    }

    pub fn observe_block(
        &mut self,
        block: &SequencerBlock,
        commit: &BlockCommit,
        chain_id: &str,
        validators: &[Validator],
    ) -> Result<(), FinalityError> {
        let receipt = self.receipt.as_ref().ok_or(FinalityError::InvalidReceipt)?;
        if block.number != receipt.height {
            return Err(FinalityError::InvalidCommit);
        }
        let verified = VerifiedBlock::verify(block, commit, chain_id, validators)?;
        if self
            .block
            .as_ref()
            .is_some_and(|previous| previous.block.hash != block.hash)
        {
            return Err(FinalityError::Corrupt);
        }
        self.block = Some(verified);
        Ok(())
    }

    /// Refreshes receipts and blocks over the node RPC; consensus evidence is supplied
    /// separately because the current block client does not expose a commit field.
    pub fn refresh(
        &mut self,
        node: &NodeRpc,
        commit: Option<&BlockCommit>,
        chain_id: &str,
        validators: &[Validator],
    ) -> Result<(), FinalityError> {
        let observed = node.sequencer_receipt(&self.tx_hash)?;
        let executed = match &observed {
            Some(
                SequencerReceipt::Receipted(receipt) | SequencerReceipt::Committed { receipt, .. },
            ) => receipt,
            _ => return Ok(()),
        };
        let keys = node.receipt_keys()?;
        let receipt = VerifiedReceipt::verify(self.tx_hash, &executed.receipt, &keys)?;
        if executed.tx_hash != receipt.tx_hash
            || executed.height != receipt.height
            || executed.key_id != receipt.key.key_id
        {
            return Err(FinalityError::Corrupt);
        }
        let height = receipt.height;
        self.retain_receipt(receipt)?;
        if let Some(commit) = commit {
            if let Some(block) = node.block_by_number(height)? {
                self.observe_block(&block, commit, chain_id, validators)?;
            }
        }
        Ok(())
    }
}

pub trait VerificationProgress {
    fn rung_at(
        &mut self,
        idempotency_key: [u8; 32],
        observed_at_ms: u64,
    ) -> Result<Rung, FinalityError>;
}

impl VerificationProgress for SubmissionProgress {
    fn rung_at(
        &mut self,
        idempotency_key: [u8; 32],
        _observed_at_ms: u64,
    ) -> Result<Rung, FinalityError> {
        if idempotency_key != self.idempotency_key {
            return Err(FinalityError::ProgressUnavailable);
        }
        Ok(if self.block.is_some() {
            Rung::Committed
        } else if self.receipt.is_some() {
            Rung::Receipted
        } else {
            Rung::Submitted
        })
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct WaitResult {
    pub requested: Rung,
    pub reached: Rung,
    pub observed_at_ms: u64,
    pub deadline_elapsed: bool,
}

pub fn wait_for_rung<P: VerificationProgress>(
    progress: &mut P,
    idempotency_key: [u8; 32],
    requested: Rung,
    started_at_ms: u64,
    deadline_ms: u64,
    poll_interval_ms: u64,
) -> Result<WaitResult, FinalityError> {
    if deadline_ms < started_at_ms || poll_interval_ms == 0 {
        return Err(FinalityError::InvalidDeadline);
    }
    let mut observed_at_ms = started_at_ms;
    loop {
        let reached = progress.rung_at(idempotency_key, observed_at_ms)?;
        if reached >= requested || observed_at_ms == deadline_ms {
            return Ok(WaitResult {
                requested,
                reached,
                observed_at_ms,
                deadline_elapsed: reached < requested,
            });
        }
        observed_at_ms = observed_at_ms
            .checked_add(poll_interval_ms)
            .ok_or(FinalityError::Arithmetic)?
            .min(deadline_ms);
    }
}

fn record_key(tenant: TenantId, idempotency_key: [u8; 32]) -> Result<TenantKey, StoreError> {
    let mut object_id = b"finality:".to_vec();
    object_id.extend_from_slice(&idempotency_key);
    TenantKey::new(tenant, ObjectKind::Configuration, object_id)
}

fn encode_record(record: &FinalityRecord) -> Result<Vec<u8>, FinalityError> {
    let mut bytes = RECORD_MAGIC.to_vec();
    bytes.extend_from_slice(&record.idempotency_key);
    bytes.extend_from_slice(&record.tx_hash);
    bytes.push(record.rung.wire_rank());
    bytes.extend_from_slice(&record.height.to_be_bytes());
    bytes.extend_from_slice(&record.receipt_key.key_id);
    bytes.extend_from_slice(&record.receipt_key.public_key);
    bytes.extend_from_slice(&record.receipt_key.first_height.to_be_bytes());
    bytes.extend_from_slice(&record.receipt_key.last_height.to_be_bytes());
    push_bytes(&mut bytes, &record.receipt_bytes)?;
    if let Some(block) = &record.block {
        bytes.push(1);
        bytes.extend_from_slice(&block.block.hash);
        push_bytes(&mut bytes, block.chain_id.as_bytes())?;
        bytes.extend_from_slice(&block.commit.round.to_be_bytes());
        bytes.extend_from_slice(&block.commit.part_set_total.to_be_bytes());
        bytes.extend_from_slice(&block.commit.part_set_hash);
        bytes.extend_from_slice(
            &u32::try_from(block.validators.len())
                .map_err(|_| FinalityError::Arithmetic)?
                .to_be_bytes(),
        );
        for validator in &block.validators {
            bytes.extend_from_slice(&validator.public_key);
            bytes.extend_from_slice(&validator.voting_power.to_be_bytes());
        }
        bytes.extend_from_slice(
            &u32::try_from(block.commit.signatures.len())
                .map_err(|_| FinalityError::Arithmetic)?
                .to_be_bytes(),
        );
        for signature in &block.commit.signatures {
            bytes.extend_from_slice(&signature.validator_address);
            bytes.extend_from_slice(&signature.timestamp_seconds.to_be_bytes());
            bytes.extend_from_slice(&signature.timestamp_nanos.to_be_bytes());
            bytes.extend_from_slice(&signature.signature);
        }
    } else {
        bytes.push(0);
    }
    Ok(bytes)
}

fn push_bytes(output: &mut Vec<u8>, value: &[u8]) -> Result<(), FinalityError> {
    output.extend_from_slice(
        &u32::try_from(value.len())
            .map_err(|_| FinalityError::Arithmetic)?
            .to_be_bytes(),
    );
    output.extend_from_slice(value);
    Ok(())
}

fn varint(output: &mut Vec<u8>, mut value: u64) {
    while value >= 128 {
        output.push((value as u8 & 127) | 128);
        value >>= 7;
    }
    output.push(value as u8);
}

fn field(output: &mut Vec<u8>, tag: u8, value: &[u8]) -> Result<(), FinalityError> {
    output.push(tag);
    varint(
        output,
        u64::try_from(value.len()).map_err(|_| FinalityError::Arithmetic)?,
    );
    output.extend_from_slice(value);
    Ok(())
}
