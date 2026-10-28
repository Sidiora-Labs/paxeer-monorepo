#![forbid(unsafe_code)]

use ed25519_dalek::{Signature, VerifyingKey};
use sha2::{Digest, Sha256};

pub mod commit;
pub mod header;
pub mod merkle;
pub mod proto;

/// A committed proof could not establish the claimed commitment.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProofError {
    /// The leaf position or number of aunts is inconsistent with the tree.
    InvalidProof,
    /// The proof reconstructs a different root.
    RootMismatch,
    /// The supplied validator set differs from the header commitment.
    ValidatorsHashMismatch,
    /// The commit metadata or signature slots are malformed.
    InvalidCommit,
    /// Validator powers or keys do not form a valid consensus set.
    InvalidValidatorSet,
    /// A present precommit does not verify against its indexed validator.
    InvalidCommitSignature,
    /// The block's precommits do not carry strictly more than two thirds power.
    InsufficientVotingPower,
    /// The receipt's encoding or required fields are not canonical.
    InvalidReceipt,
    /// The JSON view differs from the fields or output digests in the receipt.
    ReceiptViewMismatch,
    /// The receipt does not identify the supplied raw transaction.
    ReceiptActivityMismatch,
    /// The receipt does not extend the supplied sequence hash.
    ReceiptSequenceMismatch,
    /// No pinned key verifies the signature within its inclusive height range.
    UnpinnedReceiptSignature,
}

impl std::fmt::Display for ProofError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(match self {
            Self::InvalidProof => "invalid Merkle proof position or aunt count",
            Self::RootMismatch => "Merkle proof root mismatch",
            Self::ValidatorsHashMismatch => "header validator set hash mismatch",
            Self::InvalidCommit => "invalid block commit",
            Self::InvalidValidatorSet => "invalid validator set",
            Self::InvalidCommitSignature => "invalid commit signature",
            Self::InsufficientVotingPower => "insufficient commit voting power",
            Self::InvalidReceipt => "invalid canonical sequencer receipt",
            Self::ReceiptViewMismatch => "receipt JSON view differs from signed bytes",
            Self::ReceiptActivityMismatch => "receipt activity differs from raw transaction hash",
            Self::ReceiptSequenceMismatch => "receipt sequence chain mismatch",
            Self::UnpinnedReceiptSignature => "receipt signature is not pinned at its height",
        })
    }
}

impl std::error::Error for ProofError {}

/// Bind the header to the supplied validator set before returning its hash.
///
/// # Errors
/// Returns [`ProofError::ValidatorsHashMismatch`] if the set is not committed
/// by this header.
///
/// # Panics
/// Panics if the header timestamp is outside protobuf's permitted range.
pub fn verify_header(
    header: &header::Header,
    set: &[header::Validator],
) -> Result<[u8; 32], ProofError> {
    if header.validators_hash != header::validators_hash(set) {
        return Err(ProofError::ValidatorsHashMismatch);
    }
    Ok(header.hash())
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommittedProof {
    pub header: header::Header,
    pub commit: commit::Commit,
    pub execution_header: header::Header,
    pub batch_transaction: Vec<u8>,
    pub batch_total: u64,
    pub batch_index: u64,
    pub batch_aunts: Vec<[u8; 32]>,
    pub signed_receipt: Vec<u8>,
    pub activity_ids: Vec<[u8; 32]>,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct CommittedBlock {
    pub number: u64,
    pub hash: [u8; 32],
}

pub mod committed {
    use super::{commit::verify_commit, header::Validator, merkle, verify_header, ProofError};
    pub use super::{CommittedBlock, CommittedProof};
    use sha2::{Digest, Sha256};

    /// Establish batch inclusion for a digest already authenticated by receipt verification.
    /// The committed successor header authenticates the execution header through LastBlockID.
    /// This namespace preserves the existing receipt-view verifier at the crate root.
    ///
    /// # Errors
    /// Returns an error for invalid commits, header links, receipt bindings or batch proofs.
    pub fn verify_committed(
        receipt_digest: [u8; 32],
        proof: &CommittedProof,
        set: &[Validator],
        chain_id: &str,
    ) -> Result<CommittedBlock, ProofError> {
        let header = &proof.header;
        let execution = &proof.execution_header;
        if chain_id.is_empty()
            || chain_id.len() > 50
            || header.chain_id != chain_id
            || execution.chain_id != chain_id
            || execution.height <= 0
            || execution.height.checked_add(1) != Some(header.height)
            || execution.next_validators_hash.as_slice() != header.validators_hash
        {
            return Err(ProofError::InvalidCommit);
        }
        for time in [header.time, execution.time] {
            if !(-62_135_596_800..=253_402_300_799).contains(&time.seconds)
                || time.nanos >= 1_000_000_000
            {
                return Err(ProofError::InvalidCommit);
            }
        }
        let header_hash = verify_header(header, set)?;
        verify_commit(
            chain_id,
            set,
            header_hash,
            header.height as u64,
            &proof.commit,
        )?;
        let block = CommittedBlock {
            number: execution.height as u64,
            hash: execution.hash(),
        };
        if header.last_block_id.hash.as_slice() != block.hash {
            return Err(ProofError::InvalidCommit);
        }
        if proof.batch_index != 0
            || proof.batch_total > i64::MAX as u64
            || proof.batch_total.checked_sub(1) != Some(proof.activity_ids.len() as u64)
        {
            return Err(ProofError::InvalidProof);
        }
        let data_hash = execution
            .data_hash
            .as_slice()
            .try_into()
            .map_err(|_| ProofError::InvalidProof)?;
        merkle::verify_proof(
            data_hash,
            proof.batch_total,
            proof.batch_index,
            &Sha256::digest(&proof.batch_transaction),
            &proof.batch_aunts,
        )?;
        verify_batch(receipt_digest, proof, block.number)?;
        Ok(block)
    }

    fn verify_batch(
        digest: [u8; 32],
        proof: &CommittedProof,
        block_number: u64,
    ) -> Result<(), ProofError> {
        const DOMAIN: &[u8] = b"PAXEER/v1/sequence-batch\0";
        let batch = proof
            .batch_transaction
            .strip_prefix(DOMAIN)
            .filter(|body| body.len() == 216)
            .ok_or(ProofError::InvalidProof)?;
        let mut reader = super::ReceiptReader(batch);
        let height = u64::from_be_bytes(reader.fixed()?);
        let first = u64::from_be_bytes(reader.fixed()?);
        let last = u64::from_be_bytes(reader.fixed()?);
        let previous = reader.fixed::<32>()?;
        let resulting = reader.fixed::<32>()?;
        let root = reader.fixed::<32>()?;
        if height != block_number
            || first == 0
            || last
                .checked_sub(first)
                .and_then(|count| count.checked_add(1))
                != Some(proof.activity_ids.len() as u64)
        {
            return Err(ProofError::InvalidProof);
        }
        let receipt = receipt_position(digest, &proof.signed_receipt)?;
        if receipt.height == 0
            || receipt.height > height
            || !(first..=last).contains(&receipt.sequence)
        {
            return Err(ProofError::InvalidProof);
        }
        let mut activities_hash = Sha256::new();
        let mut sequence_hash = previous;
        for (index, activity) in proof.activity_ids.iter().enumerate() {
            let sequence = first + index as u64;
            let before = sequence_hash;
            let mut chain = Sha256::new();
            chain.update(b"PAXEER/v1/sequence\0");
            chain.update(before);
            chain.update(sequence.to_be_bytes());
            chain.update(activity);
            sequence_hash = chain.finalize().into();
            activities_hash.update(activity);
            if sequence == receipt.sequence
                && (*activity != receipt.activity
                    || before != receipt.previous
                    || sequence_hash != receipt.resulting)
            {
                return Err(ProofError::ReceiptSequenceMismatch);
            }
        }
        if <[u8; 32]>::from(activities_hash.finalize()) != root || sequence_hash != resulting {
            return Err(ProofError::RootMismatch);
        }
        Ok(())
    }

    struct ReceiptPosition {
        activity: [u8; 32],
        sequence: u64,
        previous: [u8; 32],
        resulting: [u8; 32],
        height: u64,
    }

    fn receipt_position(digest: [u8; 32], receipt: &[u8]) -> Result<ReceiptPosition, ProofError> {
        if receipt.len() > 1_048_576 {
            return Err(ProofError::InvalidReceipt);
        }
        let unsigned_length = receipt
            .len()
            .checked_sub(69)
            .ok_or(ProofError::InvalidReceipt)?;
        let (unsigned, signature) = receipt.split_at(unsigned_length);
        if signature[..5] != [1, 0, 0, 0, 64] {
            return Err(ProofError::InvalidReceipt);
        }
        let mut hash = Sha256::new();
        hash.update(b"LXP/v1/receipt\0");
        hash.update(unsigned);
        hash.update([0]);
        if <[u8; 32]>::from(hash.finalize()) != digest {
            return Err(ProofError::InvalidReceipt);
        }
        let mut reader = super::ReceiptReader(unsigned);
        if reader.take(6)? != [0, 2, 0x52, 1, 0, 2] {
            return Err(ProofError::InvalidReceipt);
        }
        let activity = reader.array()?;
        let sequence = u64::from_be_bytes(reader.fixed()?);
        let previous = reader.array()?;
        let resulting = reader.array()?;
        reader.array::<32>()?;
        if u32::from_be_bytes(reader.fixed()?) > 1 {
            return Err(ProofError::InvalidReceipt);
        }
        let effects = u32::from_be_bytes(reader.fixed()?);
        if effects == 0 || effects > 512 {
            return Err(ProofError::InvalidReceipt);
        }
        for _ in 0..effects {
            reader.take(8)?;
            reader.array::<32>()?;
            let length = u32::from_be_bytes(reader.fixed()?);
            if length > 256 {
                return Err(ProofError::InvalidReceipt);
            }
            reader.take(length as usize)?;
        }
        reader.take(16)?;
        let batch_id = reader.array::<32>()?;
        if batch_id[..24] != [0; 24] {
            return Err(ProofError::InvalidReceipt);
        }
        let height = u64::from_be_bytes(
            batch_id[24..]
                .try_into()
                .map_err(|_| ProofError::InvalidReceipt)?,
        );
        Ok(ReceiptPosition {
            activity,
            sequence,
            previous,
            resulting,
            height,
        })
    }
}

/// Verify a canonical signed receipt and return its domain-separated digest.
///
/// Pins are `(key id, Ed25519 public key, first height, last height)`, with inclusive
/// height bounds. The activity must identify the raw transaction by Keccak-256,
/// and the receipt must extend `prev_seq_hash` using its sequence and activity.
///
/// Every JSON field and execution output must match the signed receipt (R5).
///
/// # Errors
/// Returns [`ProofError::InvalidReceipt`] for malformed receipts,
/// [`ProofError::UnpinnedReceiptSignature`] when no eligible pin verifies,
/// [`ProofError::ReceiptViewMismatch`] for an inconsistent JSON view,
/// [`ProofError::ReceiptActivityMismatch`] for a transaction mismatch, or
/// [`ProofError::ReceiptSequenceMismatch`] for an inconsistent sequence chain.
pub fn verify_receipted(
    raw_transaction: &[u8],
    signed_receipt: &[u8],
    receipt_json: &[u8],
    prev_seq_hash: [u8; 32],
    keys: &[([u8; 32], [u8; 32], u64, u64)],
) -> Result<[u8; 32], ProofError> {
    verify_receipt(
        raw_transaction,
        signed_receipt,
        receipt_json,
        prev_seq_hash,
        keys,
        ReceiptRung::Receipted,
    )
}

/// Verify a committed receipt view using the block hash returned by [`verify_header`].
/// The view's block number must equal the signed receipt's height.
///
/// # Errors
/// Returns the same errors as [`verify_receipted`], including
/// [`ProofError::ReceiptViewMismatch`] for inconsistent block fields or rung.
pub fn verify_committed(
    raw_transaction: &[u8],
    signed_receipt: &[u8],
    receipt_json: &[u8],
    prev_seq_hash: [u8; 32],
    keys: &[([u8; 32], [u8; 32], u64, u64)],
    block_hash: [u8; 32],
) -> Result<[u8; 32], ProofError> {
    verify_receipt(
        raw_transaction,
        signed_receipt,
        receipt_json,
        prev_seq_hash,
        keys,
        ReceiptRung::Committed(block_hash),
    )
}

enum ReceiptRung {
    Receipted,
    Committed([u8; 32]),
}

fn verify_receipt(
    raw_transaction: &[u8],
    signed_receipt: &[u8],
    receipt_json: &[u8],
    prev_seq_hash: [u8; 32],
    keys: &[([u8; 32], [u8; 32], u64, u64)],
    expected_rung: ReceiptRung,
) -> Result<[u8; 32], ProofError> {
    if raw_transaction.is_empty()
        || raw_transaction.len() > 1_048_576
        || signed_receipt.len() > 1_048_576
        || receipt_json.len() > 4_194_304
    {
        return Err(ProofError::InvalidReceipt);
    }
    let mut reader = ReceiptReader(signed_receipt);
    if reader.take(6)? != [0, 2, 0x52, 1, 0, 2] {
        return Err(ProofError::InvalidReceipt);
    }
    let activity = reader.array::<32>()?;
    let sequence = u64::from_be_bytes(reader.fixed()?);
    let previous = reader.array::<32>()?;
    let resulting = reader.array::<32>()?;
    let activity_root = reader.array::<32>()?;
    let result_code = u32::from_be_bytes(reader.fixed()?);
    if result_code > 1 {
        return Err(ProofError::InvalidReceipt);
    }
    let effects = u32::from_be_bytes(reader.fixed()?);
    if effects == 0 || effects > 512 {
        return Err(ProofError::InvalidReceipt);
    }
    let mut decoded_effects = Vec::new();
    for _ in 0..effects {
        let header = reader.fixed::<6>()?;
        let kind = reader.take(1)?[0];
        let monetary = reader.take(1)?[0];
        if !(1..=3).contains(&kind) || monetary > 1 || (monetary == 1 && kind != 2) {
            return Err(ProofError::InvalidReceipt);
        }
        let root = reader.array::<32>()?;
        let length = u32::from_be_bytes(reader.fixed()?);
        if length > 256 {
            return Err(ProofError::InvalidReceipt);
        }
        let body = reader.take(length as usize)?;
        decoded_effects.push(ReceiptEffect {
            header,
            kind,
            monetary,
            root,
            body,
        });
    }
    let fee = u128::from_be_bytes(reader.fixed()?);
    let batch_id = reader.array::<32>()?;
    if batch_id[..24] != [0; 24] {
        return Err(ProofError::InvalidReceipt);
    }
    let height = u64::from_be_bytes(
        batch_id[24..]
            .try_into()
            .map_err(|_| ProofError::InvalidReceipt)?,
    );
    if height == 0 {
        return Err(ProofError::InvalidReceipt);
    }
    if reader.take(6)? != [0, 12, 0, 0, 0, 1] {
        return Err(ProofError::InvalidReceipt);
    }
    if reader.take(4)? != [0; 4] {
        return Err(ProofError::InvalidReceipt);
    }
    reader.take(1)?;
    let asset = reader.array::<32>()?;
    reader.take(16)?;
    let from = reader.array::<32>()?;
    reader.take(16 + 16)?;
    let nonce = u64::from_be_bytes(reader.fixed()?);
    let to = reader.array::<32>()?;
    reader.take(16 + 16)?;
    reader.array::<32>()?;
    reader.array::<32>()?;
    let key_id = reader.array::<32>()?;
    let timestamp = u64::from_be_bytes(reader.fixed()?);
    if timestamp == 0 {
        return Err(ProofError::InvalidReceipt);
    }
    let unsigned_length = signed_receipt.len() - reader.0.len();
    if reader.take(1)? != [1] {
        return Err(ProofError::InvalidReceipt);
    }
    let signature = Signature::from_bytes(&reader.array::<64>()?);
    if !reader.0.is_empty() {
        return Err(ProofError::InvalidReceipt);
    }
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    digest.update(&signed_receipt[..unsigned_length]);
    digest.update([0]);
    let digest: [u8; 32] = digest.finalize().into();
    let (_, signing_key, _, _) = keys
        .iter()
        .find(|(id, key, first, last)| {
            *id == key_id
                && *id != [0; 32]
                && (*first..=*last).contains(&height)
                && VerifyingKey::from_bytes(key)
                    .is_ok_and(|key| key.verify_strict(&digest, &signature).is_ok())
        })
        .ok_or(ProofError::UnpinnedReceiptSignature)?;
    if activity != transaction_hash(raw_transaction) {
        return Err(ProofError::ReceiptActivityMismatch);
    }
    if from[..12] != [0; 12] || to[..12] != [0; 12] {
        return Err(ProofError::InvalidReceipt);
    }
    let mut expected = serde_json::json!({
        "rung": "receipted",
        "txHash": receipt_hex(&activity),
        "nonce": format!("0x{nonce:x}"),
        "seq": format!("0x{sequence:x}"),
        "height": format!("0x{height:x}"),
        "timestamp": format!("0x{timestamp:x}"),
        "status": format!("0x{:x}", 1 - result_code),
        "feeAmount": format!("0x{fee:x}"),
        "from": receipt_hex(&from[12..]),
        "to": receipt_hex(&to[12..]),
        "keyId": receipt_hex(&key_id),
        "signingKey": receipt_hex(signing_key),
        "receipt": receipt_hex(signed_receipt),
    });
    if let ReceiptRung::Committed(block_hash) = expected_rung {
        expected["rung"] = "committed".into();
        expected["blockNumber"] = format!("0x{height:x}").into();
        expected["blockHash"] = receipt_hex(&block_hash).into();
    }
    verify_receipt_view(
        receipt_json,
        expected,
        activity_root,
        asset,
        &decoded_effects,
    )?;
    let mut chain = Sha256::new();
    chain.update(b"PAXEER/v1/sequence\0");
    chain.update(prev_seq_hash);
    chain.update(sequence.to_be_bytes());
    chain.update(activity);
    let next: [u8; 32] = chain.finalize().into();
    if sequence == 0 || previous != prev_seq_hash || resulting != next {
        return Err(ProofError::ReceiptSequenceMismatch);
    }
    Ok(digest)
}

struct ReceiptEffect<'a> {
    header: [u8; 6],
    kind: u8,
    monetary: u8,
    root: [u8; 32],
    body: &'a [u8],
}

fn receipt_hex(bytes: &[u8]) -> String {
    use std::fmt::Write;
    let mut hex = String::from("0x");
    for byte in bytes {
        write!(hex, "{byte:02x}").unwrap_or_else(|_| unreachable!("writing into String"));
    }
    hex
}

fn view_string<'a>(view: &'a serde_json::Value, field: &str) -> Result<&'a str, ProofError> {
    view.get(field)
        .and_then(serde_json::Value::as_str)
        .ok_or(ProofError::ReceiptViewMismatch)
}

fn view_bytes(view: &serde_json::Value, field: &str) -> Result<Vec<u8>, ProofError> {
    let hex = view_string(view, field)?
        .strip_prefix("0x")
        .ok_or(ProofError::ReceiptViewMismatch)?;
    if !hex.is_ascii() || hex.len() % 2 != 0 {
        return Err(ProofError::ReceiptViewMismatch);
    }
    (0..hex.len())
        .step_by(2)
        .map(|i| {
            u8::from_str_radix(&hex[i..i + 2], 16).map_err(|_| ProofError::ReceiptViewMismatch)
        })
        .collect()
}

fn verify_receipt_view(
    receipt_json: &[u8],
    mut expected: serde_json::Value,
    activity_root: [u8; 32],
    asset: [u8; 32],
    effects: &[ReceiptEffect<'_>],
) -> Result<(), ProofError> {
    let view: serde_json::Value =
        serde_json::from_slice(receipt_json).map_err(|_| ProofError::ReceiptViewMismatch)?;
    let summary = &effects[0];
    let body = summary.body;
    if summary.header != [0, 12, 0, 0, 0, 0]
        || summary.kind != 1
        || summary.monetary != 0
        || summary.root != [0; 32]
        || body.len() < 109
    {
        return Err(ProofError::InvalidReceipt);
    }
    let denom_length = usize::from(body[108]);
    if denom_length == 0 || denom_length > 64 || body.len() != 109 + denom_length {
        return Err(ProofError::InvalidReceipt);
    }
    let denom = std::str::from_utf8(&body[109..]).map_err(|_| ProofError::InvalidReceipt)?;
    if <[u8; 32]>::from(Sha256::digest(denom.as_bytes())) != asset {
        return Err(ProofError::InvalidReceipt);
    }
    let mut summary_reader = ReceiptReader(body);
    for name in ["gasUsed", "cumulativeGasUsed", "effectiveGasPrice"] {
        let quantity = u64::from_be_bytes(summary_reader.fixed()?);
        expected[name] = format!("0x{quantity:x}").into();
    }
    let index = u32::from_be_bytes(summary_reader.fixed()?);
    expected["index"] = format!("0x{index:x}").into();
    expected["contractAddress"] = receipt_hex(summary_reader.take(20)?).into();
    expected["sponsor"] = receipt_hex(summary_reader.take(20)?).into();
    let vm_digest = summary_reader.fixed::<32>()?;
    let return_length = u32::from_be_bytes(summary_reader.fixed()?);
    let log_count = u32::from_be_bytes(summary_reader.fixed()?);
    if log_count as usize != effects.len() - 1 {
        return Err(ProofError::InvalidReceipt);
    }
    let vm_error = view_string(&view, "vmError")?;
    let return_data = view_bytes(&view, "returnData")?;
    if <[u8; 32]>::from(Sha256::digest(vm_error.as_bytes())) != vm_digest
        || return_data.len() != return_length as usize
        || <[u8; 32]>::from(Sha256::digest(&return_data)) != activity_root
    {
        return Err(ProofError::ReceiptViewMismatch);
    }
    expected["vmError"] = vm_error.into();
    expected["returnData"] = receipt_hex(&return_data).into();
    expected["feeDenom"] = denom.into();
    let logs = view
        .get("logs")
        .and_then(serde_json::Value::as_array)
        .ok_or(ProofError::ReceiptViewMismatch)?;
    if logs.len() != effects.len() - 1 {
        return Err(ProofError::ReceiptViewMismatch);
    }
    let mut expected_logs = Vec::new();
    for (i, (log, effect)) in logs.iter().zip(&effects[1..]).enumerate() {
        let ordinal = u16::try_from(i + 1)
            .map_err(|_| ProofError::InvalidReceipt)?
            .to_be_bytes();
        let body = effect.body;
        if effect.header != [0, 12, ordinal[0], ordinal[1], 0, 0]
            || effect.kind != 3
            || effect.monetary != 0
            || effect.root != [0; 32]
            || body.len() < 21
        {
            return Err(ProofError::InvalidReceipt);
        }
        let topic_count = usize::from(body[20]);
        if topic_count > 4 || body.len() != 21 + 32 * topic_count + 32 {
            return Err(ProofError::InvalidReceipt);
        }
        let data = view_bytes(log, "data")?;
        let digest: [u8; 32] = Sha256::digest(&data).into();
        if digest.as_ref() != &body[body.len() - 32..] {
            return Err(ProofError::ReceiptViewMismatch);
        }
        let topics: Vec<_> = body[21..21 + 32 * topic_count]
            .chunks_exact(32)
            .map(receipt_hex)
            .collect();
        expected_logs.push(serde_json::json!({
            "address": receipt_hex(&body[..20]),
            "topics": topics,
            "data": receipt_hex(&data),
            "logIndex": format!("0x{i:x}"),
        }));
    }
    expected["logs"] = expected_logs.into();
    if view != expected {
        return Err(ProofError::ReceiptViewMismatch);
    }
    Ok(())
}

struct ReceiptReader<'a>(&'a [u8]);

impl<'a> ReceiptReader<'a> {
    fn take(&mut self, length: usize) -> Result<&'a [u8], ProofError> {
        let bytes = self.0.get(..length).ok_or(ProofError::InvalidReceipt)?;
        self.0 = &self.0[length..];
        Ok(bytes)
    }

    fn fixed<const N: usize>(&mut self) -> Result<[u8; N], ProofError> {
        self.take(N)?
            .try_into()
            .map_err(|_| ProofError::InvalidReceipt)
    }

    fn array<const N: usize>(&mut self) -> Result<[u8; N], ProofError> {
        if u32::from_be_bytes(self.fixed()?) as usize != N {
            return Err(ProofError::InvalidReceipt);
        }
        self.fixed()
    }
}

fn transaction_hash(bytes: &[u8]) -> [u8; 32] {
    let mut state = [0_u64; 25];
    let mut blocks = bytes.chunks_exact(136);
    for block in blocks.by_ref() {
        keccak_absorb(&mut state, block);
    }
    let remainder = blocks.remainder();
    let mut last = [0_u8; 136];
    last[..remainder.len()].copy_from_slice(remainder);
    last[remainder.len()] = 0x01;
    last[135] |= 0x80;
    keccak_absorb(&mut state, &last);
    let mut hash = [0; 32];
    for (out, lane) in hash.chunks_exact_mut(8).zip(state) {
        out.copy_from_slice(&lane.to_le_bytes());
    }
    hash
}

fn keccak_absorb(state: &mut [u64; 25], block: &[u8]) {
    const ROTATIONS: [u32; 25] = [
        0, 1, 62, 28, 27, 36, 44, 6, 55, 20, 3, 10, 43, 25, 39, 41, 45, 15, 21, 8, 18, 2, 61, 56,
        14,
    ];
    const ROUND_CONSTANTS: [u64; 24] = [
        0x0000000000000001,
        0x0000000000008082,
        0x800000000000808a,
        0x8000000080008000,
        0x000000000000808b,
        0x0000000080000001,
        0x8000000080008081,
        0x8000000000008009,
        0x000000000000008a,
        0x0000000000000088,
        0x0000000080008009,
        0x000000008000000a,
        0x000000008000808b,
        0x800000000000008b,
        0x8000000000008089,
        0x8000000000008003,
        0x8000000000008002,
        0x8000000000000080,
        0x000000000000800a,
        0x800000008000000a,
        0x8000000080008081,
        0x8000000000008080,
        0x0000000080000001,
        0x8000000080008008,
    ];
    for (index, byte) in block.iter().enumerate() {
        state[index / 8] ^= u64::from(*byte) << (8 * (index % 8));
    }
    for constant in ROUND_CONSTANTS {
        let columns: [u64; 5] = std::array::from_fn(|x| {
            state[x] ^ state[x + 5] ^ state[x + 10] ^ state[x + 15] ^ state[x + 20]
        });
        for x in 0..5 {
            let delta = columns[(x + 4) % 5] ^ columns[(x + 1) % 5].rotate_left(1);
            for y in 0..5 {
                state[x + 5 * y] ^= delta;
            }
        }
        let mut rotated = [0; 25];
        for x in 0..5 {
            for y in 0..5 {
                rotated[y + 5 * ((2 * x + 3 * y) % 5)] =
                    state[x + 5 * y].rotate_left(ROTATIONS[x + 5 * y]);
            }
        }
        for x in 0..5 {
            for y in 0..5 {
                state[x + 5 * y] = rotated[x + 5 * y]
                    ^ ((!rotated[(x + 1) % 5 + 5 * y]) & rotated[(x + 2) % 5 + 5 * y]);
            }
        }
        state[0] ^= constant;
    }
}
