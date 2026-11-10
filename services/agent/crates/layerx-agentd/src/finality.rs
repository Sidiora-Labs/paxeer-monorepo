//! Receipt-key and consensus-commit gated submission finality.
//!
//! The public wait contract is [`wait_for_rung`], parameterized by
//! [`VerificationProgress`]. It accepts `&mut P`, a 32-byte idempotency key,
//! [`Rung`], and u64 start/deadline/poll-interval values in milliseconds; it
//! returns `Result<WaitResult, FinalityError>`. The deadline is absolute and
//! inclusive, and the interval must be nonzero. A deadline result reports the
//! actual reached rung and never upgrades evidence.
//!
//! Consumers use [`SubmissionProgress::accepted`] or [`SubmissionProgress::submit`],
//! then [`SubmissionProgress::observe_receipt`] with published [`ReceiptKeyEntry`]
//! height windows. [`VerifiedReceipt::verify`] binds a 32-byte transaction hash
//! to exact signed receipt bytes. [`VerifiedBlock::verify_with_header`] is the
//! block-commit/inclusion producer; [`VerifiedBlock::verify`] alone cannot
//! establish transaction coverage. [`augment_verified`] persists the separate
//! evidence; [`SubmissionProgress::restore`] rechecks it after reopening the store.
//!
//! Retired API names: `wait_for_level` and `InclusionBundle` have no current
//! aliases. Callers must migrate to `wait_for_rung`, `VerifiedReceipt` and
//! `VerifiedBlock`; archive verification levels cannot substitute for rungs.

#[path = "inference_market/enrol.rs"]
pub mod inference_enrol;

use std::collections::BTreeSet;

use layerx_client::sequencer::{ReceiptKeyEntry, SequencerBlock, SequencerReceipt};
use layerx_crypto::ed25519;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_types::rung::Rung;
use sha2::{Digest, Sha256};

use crate::read::CheckpointHeader;
use layerx_crypto::evm_transaction::{decode_signed, transaction_hash};
use layerx_programs::chain_proof::merkle::hash_from_byte_slices;

use crate::node_rpc::{NodeRpc, NodeRpcError};
use crate::store::{ObjectKind, Store, StoreError, TenantId, TenantKey};

const RECORD_MAGIC: &[u8; 4] = b"LXF3";

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

/// One `CometBFT` precommit, including its per-validator timestamp.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommitSignature {
    pub validator_address: [u8; 20],
    pub timestamp_seconds: i64,
    pub timestamp_nanos: u32,
    pub signature: [u8; 64],
}

/// A `CometBFT` commit for the block returned by `eth_getBlockByNumber`.
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
    /// Produces the length-delimited protobuf `CanonicalVote` bytes signed by `CometBFT`.
    /// # Errors
    /// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
    /// is corrupt, or a required node or store operation fails.
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
    /// # Errors
    /// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
    /// is corrupt, or a required node or store operation fails.
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
    header: Option<CheckpointHeader>,
}

impl VerifiedBlock {
    /// # Errors
    /// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
    /// is corrupt, or a required node or store operation fails.
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
            header: None,
        })
    }
    /// Authenticates the complete transaction list against the Comet header's data root.
    /// Comet leaves are SHA-256 transaction hashes, with its simple Merkle domain tags.
    /// # Errors
    /// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
    /// is corrupt, or a required node or store operation fails.
    pub fn verify_with_header(
        block: &SequencerBlock,
        commit: &BlockCommit,
        header: &CheckpointHeader,
        chain_id: &str,
        validators: &[Validator],
    ) -> Result<Self, FinalityError> {
        let mut verified = Self::verify(block, commit, chain_id, validators)?;
        let trusted: Vec<_> = validators
            .iter()
            .map(|v| crate::read::CommitValidator {
                address: validator_address(&v.public_key),
                public_key: v.public_key,
                voting_power: v.voting_power,
            })
            .collect();
        if header.height != block.number
            || header.chain_id != chain_id
            || header.version_block != 11
            || header.validators_hash != crate::read::validator_set_hash(&trusted)
            || header.block_hash() != commit.block_hash
            || block.transactions.is_empty()
            || block.transactions.len() > 4096
            || block.transactions.iter().map(Vec::len).sum::<usize>() > 16 * 1024 * 1024
        {
            return Err(FinalityError::InvalidCommit);
        }
        let native_commit = crate::read::BlockCommit {
            height: commit.height,
            round: commit.round,
            block_id: crate::read::BlockId {
                hash: commit.block_hash,
                part_set_total: commit.part_set_total,
                part_set_hash: commit.part_set_hash,
            },
            votes: commit
                .signatures
                .iter()
                .map(|signature| {
                    let validator = trusted
                        .iter()
                        .find(|v| v.address == signature.validator_address)
                        .ok_or(FinalityError::InvalidCommit)?;
                    Ok(crate::read::CommitVote {
                        validator_address: signature.validator_address,
                        voting_power: validator.voting_power,
                        timestamp: crate::read::CommitTimestamp {
                            seconds: signature.timestamp_seconds,
                            nanos: signature.timestamp_nanos,
                        },
                        signature: signature.signature,
                    })
                })
                .collect::<Result<Vec<_>, FinalityError>>()?,
        };
        crate::read::checkpoint(block.number, chain_id, header, &native_commit, &trusted)
            .map_err(|_| FinalityError::InvalidCommit)?;
        let leaves: Vec<Vec<u8>> = block
            .transactions
            .iter()
            .map(|tx| Sha256::digest(tx).to_vec())
            .collect();
        let leaves: Vec<&[u8]> = leaves.iter().map(Vec::as_slice).collect();
        if header.data_hash != Some(hash_from_byte_slices(&leaves)) {
            return Err(FinalityError::InvalidCommit);
        }
        verified.header = Some(header.clone());
        Ok(verified)
    }

    fn binds(&self, receipt: &VerifiedReceipt) -> bool {
        self.header.is_some()
            && self.block.number == receipt.height
            && self
                .block
                .transactions
                .iter()
                .any(|raw| transaction_hash(raw) == receipt.tx_hash && decode_signed(raw).is_ok())
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

/// # Errors
/// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
/// is corrupt, or a required node or store operation fails.
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

/// # Errors
/// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
/// is corrupt, or a required node or store operation fails.
pub fn augment_verified(
    durable: &mut Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
    receipt: &VerifiedReceipt,
    block: Option<&VerifiedBlock>,
) -> Result<FinalityRecord, FinalityError> {
    if block.is_some_and(|block| !block.binds(receipt)) {
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
        let previous = decode_record(
            previous.bytes(),
            &receipt.key,
            block.map(|b| (b.chain_id.as_str(), b.validators.as_slice())),
        )?;
        if previous.idempotency_key != record.idempotency_key
            || previous.tx_hash != record.tx_hash
            || previous.receipt_bytes != record.receipt_bytes
            || previous.receipt_key != record.receipt_key
            || previous.height != record.height
            || previous.rung > record.rung
            || previous.block.as_ref().is_some_and(|old| {
                record
                    .block
                    .as_ref()
                    .is_none_or(|new| old.block.hash != new.block.hash)
            })
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

    /// # Errors
    /// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
    /// is corrupt, or a required node or store operation fails.
    pub fn restore(
        store: &Store,
        tenant: TenantId,
        idempotency_key: [u8; 32],
        receipt_key: &ReceiptKeyEntry,
        chain_id: &str,
        validators: &[Validator],
    ) -> Result<Self, FinalityError> {
        let record = restore_finality(
            store,
            tenant,
            idempotency_key,
            receipt_key,
            chain_id,
            validators,
        )?;
        let receipt =
            VerifiedReceipt::verify(record.tx_hash, &record.receipt_bytes, &[*receipt_key])?;
        Ok(Self {
            idempotency_key,
            tx_hash: record.tx_hash,
            receipt: Some(receipt),
            block: record.block,
        })
    }

    /// # Errors
    /// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
    /// is corrupt, or a required node or store operation fails.
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
    /// # Errors
    /// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
    /// is corrupt, or a required node or store operation fails.
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

    /// # Errors
    /// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
    /// is corrupt, or a required node or store operation fails.
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
        self.observe_verified_block(&verified)
    }

    /// # Errors
    /// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
    /// is corrupt, or a required node or store operation fails.
    pub fn observe_block_with_header(
        &mut self,
        block: &SequencerBlock,
        commit: &BlockCommit,
        header: &CheckpointHeader,
        chain_id: &str,
        validators: &[Validator],
    ) -> Result<(), FinalityError> {
        let verified =
            VerifiedBlock::verify_with_header(block, commit, header, chain_id, validators)?;
        self.observe_verified_block(&verified)
    }

    /// # Errors
    /// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
    /// is corrupt, or a required node or store operation fails.
    pub fn observe_verified_block(
        &mut self,
        verified: &VerifiedBlock,
    ) -> Result<(), FinalityError> {
        let receipt = self.receipt.as_ref().ok_or(FinalityError::InvalidReceipt)?;
        if !verified.binds(receipt) {
            return Err(FinalityError::InvalidCommit);
        }
        if self
            .block
            .as_ref()
            .is_some_and(|previous| previous.block.hash != verified.block.hash)
        {
            return Err(FinalityError::Corrupt);
        }
        self.block = Some(verified.clone());
        Ok(())
    }

    /// Refreshes receipts and blocks over the node RPC; consensus evidence is supplied
    /// separately because the current block client does not expose a commit field.
    /// # Errors
    /// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
    /// is corrupt, or a required node or store operation fails.
    pub fn refresh(
        &mut self,
        node: &NodeRpc,
        commit: Option<&BlockCommit>,
        chain_id: &str,
        validators: &[Validator],
    ) -> Result<(), FinalityError> {
        let observed = node.sequencer_receipt(&self.tx_hash)?;
        let Some(
            SequencerReceipt::Receipted(executed)
            | SequencerReceipt::Committed {
                receipt: executed, ..
            },
        ) = &observed
        else {
            return Ok(());
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
    /// # Errors
    /// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
    /// is corrupt, or a required node or store operation fails.
    pub fn refresh_with_header(
        &mut self,
        node: &NodeRpc,
        commit: &BlockCommit,
        header: &CheckpointHeader,
        chain_id: &str,
        validators: &[Validator],
    ) -> Result<(), FinalityError> {
        self.refresh(node, None, chain_id, validators)?;
        let height = self
            .receipt
            .as_ref()
            .ok_or(FinalityError::InvalidReceipt)?
            .height;
        let block = node
            .block_by_number(height)?
            .ok_or(FinalityError::InvalidCommit)?;
        self.observe_block_with_header(&block, commit, header, chain_id, validators)
    }
}

pub trait VerificationProgress {
    /// # Errors
    /// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
    /// is corrupt, or a required node or store operation fails.
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

/// # Errors
/// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
/// is corrupt, or a required node or store operation fails.
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
        let header = block.header.as_ref().ok_or(FinalityError::InvalidCommit)?;
        push_bytes(
            &mut bytes,
            &serde_json::to_vec(header).map_err(|_| FinalityError::Corrupt)?,
        )?;
        bytes.extend_from_slice(&block.block.parent_hash);
        bytes.extend_from_slice(&block.block.timestamp.to_be_bytes());
        bytes.extend_from_slice(&block.block.transactions_root);
        bytes.extend_from_slice(&block.block.receipts_root);
        bytes.extend_from_slice(
            &(u32::try_from(block.block.transactions.len())
                .map_err(|_| FinalityError::Arithmetic)?)
            .to_be_bytes(),
        );
        for raw in &block.block.transactions {
            push_bytes(&mut bytes, raw)?;
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
        output.push((value.to_le_bytes()[0] & 127) | 128);
        value >>= 7;
    }
    output.push(value.to_le_bytes()[0]);
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

/// Reopens retained evidence using the caller's independently trusted key and validator set.
/// # Errors
/// Returns an error if receipt or commit evidence is invalid or inconsistent, retained state
/// is corrupt, or a required node or store operation fails.
pub fn restore_finality(
    store: &Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
    receipt_key: &ReceiptKeyEntry,
    chain_id: &str,
    validators: &[Validator],
) -> Result<FinalityRecord, FinalityError> {
    let value = store
        .get(&record_key(tenant, idempotency_key)?)
        .ok_or(FinalityError::ProgressUnavailable)?;
    if value.class() != crate::store::StorageClass::LocalOnly {
        return Err(FinalityError::Corrupt);
    }
    let record = decode_record(value.bytes(), receipt_key, Some((chain_id, validators)))?;
    if record.idempotency_key != idempotency_key {
        return Err(FinalityError::Corrupt);
    }
    Ok(record)
}

fn decode_record(
    bytes: &[u8],
    key: &ReceiptKeyEntry,
    trust: Option<(&str, &[Validator])>,
) -> Result<FinalityRecord, FinalityError> {
    let mut r = Reader(bytes);
    crate::reject_if(&r.array::<4>()? != RECORD_MAGIC, || FinalityError::Corrupt)?;
    let idempotency_key = r.array()?;
    let tx_hash = r.array()?;
    let rank = r.array::<1>()?[0];
    let height = r.u64()?;
    let stored_key = ReceiptKeyEntry {
        key_id: r.array()?,
        public_key: r.array()?,
        first_height: r.u64()?,
        last_height: r.u64()?,
    };
    crate::reject_if(&stored_key != key, || FinalityError::InvalidReceipt)?;
    let receipt_bytes = r.data()?;
    let receipt = VerifiedReceipt::verify(tx_hash, &receipt_bytes, &[*key])?;
    crate::reject_if(receipt.height != height, || FinalityError::Corrupt)?;
    let block = match r.array::<1>()?[0] {
        0 if rank == Rung::Receipted.wire_rank() => None,
        1 if rank == Rung::Committed.wire_rank() => {
            Some(decode_committed_block(&mut r, height, trust, &receipt)?)
        }
        _ => return Err(FinalityError::Corrupt),
    };
    crate::reject_if(!r.0.is_empty(), || FinalityError::Corrupt)?;
    Ok(FinalityRecord {
        idempotency_key,
        tx_hash,
        rung: if block.is_some() {
            Rung::Committed
        } else {
            Rung::Receipted
        },
        receipt_bytes,
        receipt_key: *key,
        height,
        block,
    })
}

/// Waits for the journal's authenticated market receipt at the requested deadline.
/// # Errors
/// Returns an error if the deadline is invalid or expires, receipt authentication or
/// transaction binding fails, or the market operation is refused.
pub fn wait_for_market_acceptance(
    journal: &layerx_client::ai_market::OperationJournal,
    record: &mut layerx_client::ai_market::OperationRecord,
    endpoint: &layerx_client::sequencer::SequencerEndpoint,
    receipt_keys: &[ReceiptKeyEntry],
    timeout: std::time::Duration,
) -> Result<layerx_client::rpc::receipt::AuthenticatedReceipt, inference_enrol::EnrolError> {
    use layerx_client::ai_market::{
        market_envelope, market_program::codec, program_refusal, AiClientError,
    };
    use layerx_client::rpc::receipt::AuthenticatedLookupContext;
    use std::time::{Duration, Instant};
    if timeout.is_zero() || timeout > Duration::from_secs(30) {
        return Err(inference_enrol::EnrolError::Finality(
            FinalityError::InvalidDeadline,
        ));
    }
    let started = Instant::now();
    loop {
        if started.elapsed() >= timeout {
            return Err(inference_enrol::EnrolError::Deadline);
        }
        if let Some(receipt) = journal.resolve_receipt(
            record,
            endpoint,
            AuthenticatedLookupContext { receipt_keys },
        )? {
            let (call, signature) =
                decode_signed(record.signed_bytes()).map_err(AiClientError::from)?;
            if receipt.execution().to != call.to
                || receipt.execution().nonce != call.nonce
                || receipt.execution().from
                    != layerx_crypto::evm_transaction::recover_sender(&call, &signature)
                        .map_err(AiClientError::from)?
            {
                return Err(AiClientError::ReceiptMismatch.into());
            }
            let mut progress =
                SubmissionProgress::accepted(record.idempotency_key(), record.activity_id());
            progress
                .observe_receipt(receipt.canonical_bytes(), receipt_keys)
                .map_err(inference_enrol::EnrolError::Finality)?;
            let reached = wait_for_rung(
                &mut progress,
                record.idempotency_key(),
                Rung::Receipted,
                0,
                0,
                1,
            )
            .map_err(inference_enrol::EnrolError::Finality)?;
            if reached.deadline_elapsed {
                return Err(inference_enrol::EnrolError::Deadline);
            }
            if !receipt.execution().success {
                let envelope = market_envelope(record)?;
                let operation = codec::decode_envelope(&envelope)?.envelope.operation;
                if let Ok(Some(error)) =
                    program_refusal(operation, &receipt.execution().return_data)
                {
                    return Err(error.into());
                }
                return Err(inference_enrol::EnrolError::ExecutionRefused);
            }
            return Ok(receipt);
        }
        let remaining = timeout.saturating_sub(started.elapsed());
        if remaining.is_zero() {
            return Err(inference_enrol::EnrolError::Deadline);
        }
        std::thread::sleep(remaining.min(Duration::from_millis(20)));
    }
}

fn decode_committed_block(
    r: &mut Reader<'_>,
    height: u64,
    trust: Option<(&str, &[Validator])>,
    receipt: &VerifiedReceipt,
) -> Result<VerifiedBlock, FinalityError> {
    let hash = r.array()?;
    let chain_id = String::from_utf8(r.data()?).map_err(|_| FinalityError::Corrupt)?;
    let round = r.u32()?;
    let part_set_total = r.u32()?;
    let part_set_hash = r.array()?;
    let count = r.u32()?;
    crate::reject_if(count > 10_000, || FinalityError::Corrupt)?;
    let mut validators = Vec::new();
    for _ in 0..count {
        validators.push(Validator {
            public_key: r.array()?,
            voting_power: r.u64()?,
        });
    }
    let (trusted_chain, trusted_validators) = trust.ok_or(FinalityError::InvalidCommit)?;
    crate::reject_if(
        chain_id != trusted_chain || validators != trusted_validators,
        || FinalityError::InvalidCommit,
    )?;
    let count = r.u32()?;
    crate::reject_if(
        usize::try_from(count).map_err(|_| FinalityError::Corrupt)? > validators.len(),
        || FinalityError::Corrupt,
    )?;
    let mut signatures = Vec::new();
    for _ in 0..count {
        signatures.push(CommitSignature {
            validator_address: r.array()?,
            timestamp_seconds: i64::from_be_bytes(r.array()?),
            timestamp_nanos: r.u32()?,
            signature: r.array()?,
        });
    }
    let header: CheckpointHeader =
        serde_json::from_slice(&r.data()?).map_err(|_| FinalityError::Corrupt)?;
    let parent_hash = r.array()?;
    let timestamp = r.u64()?;
    let transactions_root = r.array()?;
    let receipts_root = r.array()?;
    let count = r.u32()?;
    crate::reject_if(count > 4096, || FinalityError::Corrupt)?;
    let mut transactions = Vec::new();
    for _ in 0..count {
        transactions.push(r.data()?);
    }
    let block = SequencerBlock {
        number: height,
        hash,
        parent_hash,
        timestamp,
        transactions_root,
        receipts_root,
        transactions,
    };
    let commit = BlockCommit {
        height,
        round,
        block_hash: hash,
        part_set_total,
        part_set_hash,
        signatures,
    };
    let verified =
        VerifiedBlock::verify_with_header(&block, &commit, &header, &chain_id, &validators)?;
    crate::reject_if(!verified.binds(receipt), || FinalityError::InvalidCommit)?;
    Ok(verified)
}

struct Reader<'a>(&'a [u8]);
impl Reader<'_> {
    fn array<const N: usize>(&mut self) -> Result<[u8; N], FinalityError> {
        let (head, rest) = self.0.split_at_checked(N).ok_or(FinalityError::Corrupt)?;
        self.0 = rest;
        head.try_into().map_err(|_| FinalityError::Corrupt)
    }
    fn u32(&mut self) -> Result<u32, FinalityError> {
        Ok(u32::from_be_bytes(self.array()?))
    }
    fn u64(&mut self) -> Result<u64, FinalityError> {
        Ok(u64::from_be_bytes(self.array()?))
    }
    fn data(&mut self) -> Result<Vec<u8>, FinalityError> {
        let len = self.u32()? as usize;
        let (head, rest) = self.0.split_at_checked(len).ok_or(FinalityError::Corrupt)?;
        self.0 = rest;
        Ok(head.to_vec())
    }
}
