use layerx_agentd::read::{AbciResultProof, CommitValidator, ServedCheckpoint};
use layerx_client::rpc::receipt::{
    lookup_authenticated, AuthenticatedReceipt, Lookup, LookupContext,
};
use layerx_client::rpc::receipt_commit::SequenceBatch;
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_types::rung::Rung;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest as _, Sha256};

use super::JourneyError;

/// A pinned consensus trust set and the node's exact sequence-batch inclusion proof.
/// The proof fields use the daemon's Comet simple-proof transport encoding.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct BankCommitEvidence {
    pub chain_id: String,
    pub trusted_validators: Value,
    pub committed_header: Value,
    pub covering_height: u64,
    pub batch_transaction: Vec<u8>,
    pub transaction_proof: Vec<Vec<u8>>,
    pub activity_ids: Vec<[u8; 32]>,
}

/// A receipt whose sequence batch is covered by a verified consensus quorum.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BankCommittedReceipt {
    receipt: AuthenticatedReceipt,
}

impl BankCommittedReceipt {
    #[must_use]
    pub fn receipt(&self) -> &AuthenticatedReceipt {
        &self.receipt
    }

    #[must_use]
    pub const fn rung(&self) -> Rung {
        Rung::Committed
    }
}

impl BankCommitEvidence {
    /// Authenticates the receipt, governed key, sequence batch and committed inclusion.
    pub fn authenticate(
        &self,
        endpoint: &SequencerEndpoint,
        hash: [u8; 32],
        keys: &[ReceiptKeyEntry],
    ) -> Result<BankCommittedReceipt, JourneyError> {
        let Lookup::Verified(receipt) =
            lookup_authenticated(endpoint, hash, LookupContext { receipt_keys: keys })
                .map_err(|_| JourneyError::ReceiptMismatch)?
        else {
            return Err(JourneyError::ReceiptMismatch);
        };
        self.verify(endpoint, &receipt, keys)?;
        Ok(BankCommittedReceipt { receipt })
    }

    pub(super) fn verify(
        &self,
        endpoint: &SequencerEndpoint,
        receipt: &AuthenticatedReceipt,
        keys: &[ReceiptKeyEntry],
    ) -> Result<(), JourneyError> {
        let bad = || JourneyError::ReceiptMismatch;
        if self.chain_id.is_empty()
            || self.covering_height < receipt.height()
            || receipt
                .covering_block()
                .is_some_and(|(height, _)| height != self.covering_height)
            || receipt
                .landing()
                .is_some_and(|landing| landing.block_number > self.covering_height)
        {
            return Err(bad());
        }
        self.verify_batch(receipt, keys)?;
        if endpoint
            .commit_material(self.covering_height)
            .map_err(|_| bad())?["signedHeader"]
            != self.committed_header
        {
            return Err(bad());
        }
        let block = endpoint
            .block_by_number(self.covering_height)
            .map_err(|_| bad())?
            .ok_or_else(bad)?;
        if receipt
            .covering_block()
            .is_some_and(|(_, hash)| hash != block.hash)
        {
            return Err(bad());
        }
        let trusted = layerx_client::rpc::commit::decode_validators(&self.trusted_validators)
            .map_err(|_| bad())?;
        let validators: Vec<_> = trusted
            .validators()
            .iter()
            .map(|validator| CommitValidator {
                address: validator.address,
                public_key: validator.public_key,
                voting_power: validator.voting_power,
            })
            .collect();
        let checkpoint = ServedCheckpoint::acquire_at_committed_block(
            endpoint,
            self.covering_height,
            Some(block.hash),
            &self.chain_id,
        )
        .map_err(|_| bad())?;
        if checkpoint.validators != validators {
            return Err(bad());
        }
        let verified = layerx_agentd::read::checkpoint(
            self.covering_height,
            &self.chain_id,
            &checkpoint.header,
            &checkpoint.commit,
            &validators,
        )
        .map_err(|_| bad())?;
        if verified != checkpoint {
            return Err(bad());
        }
        let proof = AbciResultProof::decode(&self.transaction_proof).map_err(|_| bad())?;
        let transaction_hash: [u8; 32] = Sha256::digest(&self.batch_transaction).into();
        if proof.result.as_slice() != transaction_hash.as_slice() {
            return Err(bad());
        }
        proof
            .verify(checkpoint.header.data_hash.ok_or_else(bad)?, 0)
            .map_err(|_| bad())?;
        Ok(())
    }

    /// Revalidates retained consensus and receipt coverage without a live node.
    pub(crate) fn verify_retained(
        &self,
        receipt: &layerx_intents::canonical::ProtocolReceipt,
        keys: &[ReceiptKeyEntry],
    ) -> Result<(), JourneyError> {
        let bad = || JourneyError::ReceiptMismatch;
        let mut inclusion = [0; 8];
        inclusion.copy_from_slice(&receipt.batch_id()[24..]);
        if self.chain_id.is_empty() || self.covering_height < u64::from_be_bytes(inclusion) {
            return Err(bad());
        }
        self.verify_protocol_batch(receipt, keys)?;
        let checkpoint = retained_checkpoint(self)?;
        let proof = AbciResultProof::decode(&self.transaction_proof).map_err(|_| bad())?;
        let transaction_hash: [u8; 32] = Sha256::digest(&self.batch_transaction).into();
        if proof.result.as_slice() != transaction_hash.as_slice() {
            return Err(bad());
        }
        proof
            .verify(checkpoint.header.data_hash.ok_or_else(bad)?, 0)
            .map_err(|_| bad())?;
        Ok(())
    }

    pub(crate) fn checkpoint_id(&self) -> Result<[u8; 32], JourneyError> {
        Ok(retained_checkpoint(self)?.header.block_hash())
    }

    fn verify_batch(
        &self,
        receipt: &AuthenticatedReceipt,
        keys: &[ReceiptKeyEntry],
    ) -> Result<(), JourneyError> {
        self.verify_protocol_batch(
            receipt
                .receipt()
                .protocol()
                .ok_or(JourneyError::ReceiptShape)?,
            keys,
        )
    }

    fn verify_protocol_batch(
        &self,
        protocol: &layerx_intents::canonical::ProtocolReceipt,
        keys: &[ReceiptKeyEntry],
    ) -> Result<(), JourneyError> {
        let bad = || JourneyError::ReceiptMismatch;
        let batch = SequenceBatch::decode(&self.batch_transaction).map_err(|_| bad())?;
        if batch.height != self.covering_height
            || batch.from_sequence > protocol.global_sequence()
            || batch.to_sequence < protocol.global_sequence()
            || batch
                .to_sequence
                .checked_sub(batch.from_sequence)
                .and_then(|count| count.checked_add(1))
                != u64::try_from(self.activity_ids.len()).ok()
        {
            return Err(bad());
        }
        let mut matching = keys.iter().filter(|key| {
            key.key_id == batch.key_id
                && (key.first_height..=key.last_height).contains(&batch.height)
        });
        let key = matching.next().ok_or_else(bad)?;
        if matching.next().is_some() {
            return Err(bad());
        }
        layerx_crypto::ed25519::verify_digest(
            &key.public_key,
            &batch.signature,
            &batch.signing_digest(&self.chain_id).map_err(|_| bad())?,
        )
        .map_err(|_| bad())?;
        let mut activity_hash = Sha256::new();
        let mut root = batch.previous_root;
        for (offset, id) in self.activity_ids.iter().enumerate() {
            activity_hash.update(id);
            let sequence = batch
                .from_sequence
                .checked_add(u64::try_from(offset).map_err(|_| bad())?)
                .ok_or_else(bad)?;
            let previous = root;
            let mut hash = Sha256::new();
            hash.update(b"PAXEER/v1/sequence\0");
            hash.update(previous);
            hash.update(sequence.to_be_bytes());
            hash.update(id);
            root = hash.finalize().into();
            if sequence == protocol.global_sequence()
                && (*id != protocol.activity_id()
                    || previous != protocol.previous_state_root()
                    || root != protocol.resulting_state_root())
            {
                return Err(bad());
            }
        }
        if root != batch.resulting_root
            || <[u8; 32]>::from(activity_hash.finalize()) != batch.activity_root
        {
            return Err(bad());
        }
        Ok(())
    }
}

// Replay the retained Comet transport through the daemon checkpoint verifier.
// Its JSON constructor is crate-private, so the journey owns only this decoding
// adapter; quorum, header hashing, signatures and inclusion stay in real codecs.
fn retained_checkpoint(evidence: &BankCommitEvidence) -> Result<ServedCheckpoint, JourneyError> {
    use base64::Engine as _;
    use layerx_agentd::read::{
        BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitVote,
    };
    let bad = || JourneyError::ReceiptMismatch;
    fn number(value: &Value) -> Result<u64, JourneyError> {
        value
            .as_u64()
            .or_else(|| value.as_str()?.parse().ok())
            .ok_or(JourneyError::ReceiptMismatch)
    }
    fn hex<const N: usize>(value: &Value) -> Result<[u8; N], JourneyError> {
        let text = value.as_str().ok_or(JourneyError::ReceiptMismatch)?;
        let text = text.strip_prefix("0x").unwrap_or(text);
        if text.len() != N * 2 || !text.is_ascii() {
            return Err(JourneyError::ReceiptMismatch);
        }
        let mut out = [0; N];
        for (index, pair) in text.as_bytes().chunks_exact(2).enumerate() {
            out[index] = u8::from_str_radix(
                std::str::from_utf8(pair).map_err(|_| JourneyError::ReceiptMismatch)?,
                16,
            )
            .map_err(|_| JourneyError::ReceiptMismatch)?;
        }
        Ok(out)
    }
    fn timestamp(value: &Value) -> Result<CommitTimestamp, JourneyError> {
        let text = value.as_str().ok_or(JourneyError::ReceiptMismatch)?;
        let (base, nanos) = if let Some((base, fraction)) = text.split_once('.') {
            let digits = fraction
                .strip_suffix('Z')
                .ok_or(JourneyError::ReceiptMismatch)?;
            if digits.is_empty() || digits.len() > 9 || !digits.bytes().all(|b| b.is_ascii_digit())
            {
                return Err(JourneyError::ReceiptMismatch);
            }
            (
                format!("{base}Z"),
                digits
                    .parse::<u32>()
                    .map_err(|_| JourneyError::ReceiptMismatch)?
                    * 10_u32.pow(9 - digits.len() as u32),
            )
        } else {
            (text.to_owned(), 0)
        };
        let seconds =
            crate::time::seconds_from_rfc3339(&base).ok_or(JourneyError::ReceiptMismatch)?;
        Ok(CommitTimestamp {
            seconds: i64::try_from(seconds).map_err(|_| JourneyError::ReceiptMismatch)?,
            nanos,
        })
    }
    fn block_id(value: &Value) -> Result<BlockId, JourneyError> {
        Ok(BlockId {
            hash: hex(&value["hash"])?,
            part_set_total: u32::try_from(number(&value["parts"]["total"])?)
                .map_err(|_| JourneyError::ReceiptMismatch)?,
            part_set_hash: hex(&value["parts"]["hash"])?,
        })
    }
    let trusted = layerx_client::rpc::commit::decode_validators(&evidence.trusted_validators)
        .map_err(|_| bad())?;
    let validators: Vec<_> = trusted
        .validators()
        .iter()
        .map(|v| CommitValidator {
            address: v.address,
            public_key: v.public_key,
            voting_power: v.voting_power,
        })
        .collect();
    let h = &evidence.committed_header["header"];
    let optional = |key: &str| -> Result<Option<[u8; 32]>, JourneyError> {
        if h[key].is_null() || h[key] == "" {
            Ok(None)
        } else {
            hex(&h[key]).map(Some)
        }
    };
    let header = CheckpointHeader {
        version_block: number(&h["version"]["block"])?,
        version_app: number(&h["version"]["app"])?,
        chain_id: h["chain_id"].as_str().ok_or_else(bad)?.to_owned(),
        height: number(&h["height"])?,
        time: timestamp(&h["time"])?,
        last_block_id: if h["last_block_id"]["hash"] == "" {
            None
        } else {
            Some(block_id(&h["last_block_id"])?)
        },
        last_commit_hash: optional("last_commit_hash")?,
        data_hash: optional("data_hash")?,
        validators_hash: hex(&h["validators_hash"])?,
        next_validators_hash: hex(&h["next_validators_hash"])?,
        consensus_hash: hex(&h["consensus_hash"])?,
        app_hash: hex(&h["app_hash"])?,
        last_results_hash: optional("last_results_hash")?,
        evidence_hash: optional("evidence_hash")?,
        proposer_address: hex(&h["proposer_address"])?,
    };
    let c = &evidence.committed_header["commit"];
    let signatures = c["signatures"].as_array().ok_or_else(bad)?;
    if signatures.len() != validators.len() {
        return Err(bad());
    }
    let mut commit = BlockCommit {
        height: number(&c["height"])?,
        round: u32::try_from(number(&c["round"])?).map_err(|_| bad())?,
        block_id: block_id(&c["block_id"])?,
        votes: Vec::new(),
    };
    for (signature, validator) in signatures.iter().zip(&validators) {
        match number(&signature["block_id_flag"])? {
            1 if signature["validator_address"] == "" && signature["signature"].is_null() => {}
            flag @ (2 | 3) => {
                if hex::<20>(&signature["validator_address"])? != validator.address {
                    return Err(bad());
                }
                let stamp = timestamp(&signature["timestamp"])?;
                let bytes: [u8; 64] = base64::engine::general_purpose::STANDARD
                    .decode(signature["signature"].as_str().ok_or_else(bad)?)
                    .map_err(|_| bad())?
                    .try_into()
                    .map_err(|_| bad())?;
                if flag == 2 {
                    commit.votes.push(CommitVote {
                        validator_address: validator.address,
                        voting_power: validator.voting_power,
                        timestamp: stamp,
                        signature: bytes,
                    });
                }
            }
            _ => return Err(bad()),
        }
    }
    layerx_agentd::read::checkpoint(
        evidence.covering_height,
        &evidence.chain_id,
        &header,
        &commit,
        &validators,
    )
    .map_err(|_| bad())
}
