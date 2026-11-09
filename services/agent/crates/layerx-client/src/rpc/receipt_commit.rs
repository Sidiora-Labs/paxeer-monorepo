use ed25519_dalek::{Signature, VerifyingKey};
use layerx_programs_registry::chain_proof::{
    merkle::SimpleProof, tx::verify_tx, ChainProofVerifier, VerifiedChainHead,
};
use layerx_wire::receipt::ProtocolReceipt;
use sha2::{Digest as _, Sha256};

use crate::sequencer::ReceiptKeyEntry;

const BATCH_DOMAIN: &[u8] = b"PAXEER/v1/sequence-batch\0";

/// Comet inclusion evidence for the real `MsgSequenceBatch` transaction. The
/// node transport owning the raw block supplies the transaction and its path.
pub struct ReceiptCommitEvidence<'a> {
    pub verifier: &'a ChainProofVerifier,
    pub covering_height: u64,
    pub batch_transaction: &'a [u8],
    pub transaction_proof: &'a SimpleProof,
    pub activity_ids: &'a [[u8; 32]],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum CoverageError {
    Encoding,
    Inclusion,
    Height,
    Sequence,
    Signature,
    ActivityRoot,
    StateRoot,
}

/// The exact node `MsgSequenceBatch` wire envelope, including its receipt-key signature.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SequenceBatch {
    pub height: u64,
    pub from_sequence: u64,
    pub to_sequence: u64,
    pub previous_root: [u8; 32],
    pub resulting_root: [u8; 32],
    pub activity_root: [u8; 32],
    pub key_id: [u8; 32],
    pub signature: [u8; 64],
}

impl SequenceBatch {
    /// # Errors
    ///
    /// Returns `CoverageError::Encoding` for an invalid batch domain, length, height, sequence range or empty-batch roots.
    pub fn decode(bytes: &[u8]) -> Result<Self, CoverageError> {
        fn take<const N: usize>(rest: &mut &[u8]) -> Result<[u8; N], CoverageError> {
            let (value, next) = rest.split_at_checked(N).ok_or(CoverageError::Encoding)?;
            *rest = next;
            value.try_into().map_err(|_| CoverageError::Encoding)
        }
        if bytes.len() != 241 || !bytes.starts_with(BATCH_DOMAIN) {
            return Err(CoverageError::Encoding);
        }
        let mut rest = &bytes[BATCH_DOMAIN.len()..];

        let batch = Self {
            height: u64::from_be_bytes(take(&mut rest)?),
            from_sequence: u64::from_be_bytes(take(&mut rest)?),
            to_sequence: u64::from_be_bytes(take(&mut rest)?),
            previous_root: take(&mut rest)?,
            resulting_root: take(&mut rest)?,
            activity_root: take(&mut rest)?,
            key_id: take(&mut rest)?,
            signature: take(&mut rest)?,
        };
        batch.validate()?;
        Ok(batch)
    }

    fn validate(&self) -> Result<(), CoverageError> {
        if self.height == 0 || self.from_sequence == 0 || self.to_sequence < self.from_sequence - 1
        {
            return Err(CoverageError::Encoding);
        }
        if self.to_sequence == self.from_sequence - 1
            && (self.previous_root != self.resulting_root
                || self.activity_root != <[u8; 32]>::from(Sha256::digest([])))
        {
            return Err(CoverageError::Encoding);
        }
        Ok(())
    }

    /// # Errors
    ///
    /// Returns `CoverageError::Encoding` for an invalid height, sequence range or empty-batch roots.
    pub fn encode(&self) -> Result<Vec<u8>, CoverageError> {
        self.validate()?;
        let mut bytes = BATCH_DOMAIN.to_vec();
        bytes.extend_from_slice(&self.height.to_be_bytes());
        bytes.extend_from_slice(&self.from_sequence.to_be_bytes());
        bytes.extend_from_slice(&self.to_sequence.to_be_bytes());
        bytes.extend_from_slice(&self.previous_root);
        bytes.extend_from_slice(&self.resulting_root);
        bytes.extend_from_slice(&self.activity_root);
        bytes.extend_from_slice(&self.key_id);
        bytes.extend_from_slice(&self.signature);
        Ok(bytes)
    }

    /// # Errors
    ///
    /// Returns `CoverageError::Encoding` for an empty chain id or invalid batch fields.
    pub fn signing_digest(&self, chain_id: &str) -> Result<[u8; 32], CoverageError> {
        self.validate()?;
        if chain_id.is_empty() {
            return Err(CoverageError::Encoding);
        }
        let mut hash = Sha256::new();
        hash.update(BATCH_DOMAIN);
        hash.update(chain_id.as_bytes());
        hash.update(self.height.to_be_bytes());
        hash.update(self.from_sequence.to_be_bytes());
        hash.update(self.to_sequence.to_be_bytes());
        hash.update(self.previous_root);
        hash.update(self.resulting_root);
        hash.update(self.activity_root);
        Ok(hash.finalize().into())
    }
}

/// # Errors
///
/// Returns encoding, inclusion, height, sequence, signature or root errors when the receipt is not covered by the authenticated batch.
pub fn verify_receipt_coverage(
    receipt: &ProtocolReceipt,
    execution_height: u64,
    head: &VerifiedChainHead,
    evidence: &ReceiptCommitEvidence<'_>,
    keys: &[ReceiptKeyEntry],
) -> Result<(), CoverageError> {
    if evidence.transaction_proof.index != 0 {
        return Err(CoverageError::Inclusion);
    }
    verify_tx(head, evidence.batch_transaction, evidence.transaction_proof)
        .map_err(|_| CoverageError::Inclusion)?;
    let batch = SequenceBatch::decode(evidence.batch_transaction)?;
    if u64::try_from(head.height()).ok() != Some(batch.height) || batch.height < execution_height {
        return Err(CoverageError::Height);
    }
    if receipt.global_sequence() < batch.from_sequence
        || receipt.global_sequence() > batch.to_sequence
    {
        return Err(CoverageError::Sequence);
    }
    let count = batch
        .to_sequence
        .checked_sub(batch.from_sequence)
        .and_then(|n| n.checked_add(1))
        .ok_or(CoverageError::Sequence)?;
    if u64::try_from(evidence.activity_ids.len()).ok() != Some(count) {
        return Err(CoverageError::Sequence);
    }
    let mut matching = keys.iter().filter(|key| {
        key.key_id == batch.key_id
            && key.first_height <= batch.height
            && batch.height <= key.last_height
    });
    let key = matching.next().ok_or(CoverageError::Signature)?;
    if matching.next().is_some() {
        return Err(CoverageError::Signature);
    }
    VerifyingKey::from_bytes(&key.public_key)
        .map_err(|_| CoverageError::Signature)?
        .verify_strict(
            &batch.signing_digest(head.chain_id())?,
            &Signature::from_bytes(&batch.signature),
        )
        .map_err(|_| CoverageError::Signature)?;
    let mut activity_hash = Sha256::new();
    let mut root = batch.previous_root;
    for (offset, activity_id) in evidence.activity_ids.iter().enumerate() {
        activity_hash.update(activity_id);
        let sequence = batch
            .from_sequence
            .checked_add(u64::try_from(offset).map_err(|_| CoverageError::Sequence)?)
            .ok_or(CoverageError::Sequence)?;
        let previous = root;
        let mut hash = Sha256::new();
        hash.update(b"PAXEER/v1/sequence\0");
        hash.update(previous);
        hash.update(sequence.to_be_bytes());
        hash.update(activity_id);
        root = hash.finalize().into();
        if sequence == receipt.global_sequence()
            && (*activity_id != receipt.activity_id()
                || previous != receipt.previous_state_root()
                || root != receipt.resulting_state_root())
        {
            return Err(CoverageError::StateRoot);
        }
    }
    if <[u8; 32]>::from(activity_hash.finalize()) != batch.activity_root {
        return Err(CoverageError::ActivityRoot);
    }
    if root != batch.resulting_root {
        return Err(CoverageError::StateRoot);
    }
    Ok(())
}
