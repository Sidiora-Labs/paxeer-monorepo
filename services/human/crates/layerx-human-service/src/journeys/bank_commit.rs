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

    fn verify_batch(
        &self,
        receipt: &AuthenticatedReceipt,
        keys: &[ReceiptKeyEntry],
    ) -> Result<(), JourneyError> {
        let bad = || JourneyError::ReceiptMismatch;
        let batch = SequenceBatch::decode(&self.batch_transaction).map_err(|_| bad())?;
        let protocol = receipt.receipt().protocol().ok_or_else(bad)?;
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
