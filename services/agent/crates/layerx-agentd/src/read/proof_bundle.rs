use layerx_agent_api::error::{ApiError, ErrorClass, ReasonCode, RequestId, Retriability};
use layerx_agent_api::proof::{ProofBundle, ProofBundleTarget};
use layerx_agent_api::read::VerifiedRead;
use layerx_agent_api::verify::Rung;
use layerx_client::Client;
use layerx_types::payload::ModuleRegistry;

use layerx_agent_api::proof::MAX_PROOF_BUNDLE_BYTES;
use layerx_client::sequencer::{AccountProof, SequencerEndpoint};
use layerx_crypto::evm_transaction::transaction_hash as keccak256;
use sha2::{Digest as _, Sha256};

use super::checkpoint_impl::{
    serve_checkpoint, CheckpointReadError, ProofBundleKind, ProofBundleRequest, ServedCheckpoint,
    ServedProofBundle,
};

pub(crate) struct ProofBundleScope<'a> {
    pub account: [u8; 32],
    pub asset: [u8; 32],
    pub retained_activity: &'a [u8],
    pub served_receipt: &'a [u8],
    pub correlation: u64,
}

pub(crate) fn acquire(
    node: &mut Client,
    registry: &ModuleRegistry,
    target: ProofBundleTarget,
    requested: Rung,
    scope: ProofBundleScope<'_>,
) -> Result<VerifiedRead<ProofBundle>, ApiError> {
    let refuse = |class, reason| ApiError {
        class,
        protocol_result_code: None,
        retriability: Retriability::Terminal,
        request_id: RequestId(scope.correlation),
        reason: ReasonCode::new(reason).expect("static read reason"),
    };
    if let ProofBundleTarget::AccountState { account_id, .. } = target {
        if account_id != scope.account {
            return Err(refuse(ErrorClass::PolicyRefusal, "read.target_mismatch"));
        }
        // PB1 supplies no Comet block commit with which to authenticate eth_getProof.
        return Err(refuse(
            ErrorClass::VerificationFailure,
            "read.not_committed",
        ));
    }
    if requested > Rung::Receipted {
        return Err(refuse(
            ErrorClass::VerificationFailure,
            "read.below_requested_rung",
        ));
    }
    let activity = layerx_wire::activity::decode_signed(scope.retained_activity, registry)
        .map_err(|_| refuse(ErrorClass::VerificationFailure, "read.target_mismatch"))?;
    if layerx_wire::hash::activity_id(&activity)
        .map_err(|_| refuse(ErrorClass::VerificationFailure, "read.target_mismatch"))?
        != target.activity_id()
        || scope.account == [0; 32]
        || scope.asset == [0; 32]
    {
        return Err(refuse(ErrorClass::PolicyRefusal, "read.target_mismatch"));
    }
    let bundle = node
        .proof_bundle(
            layerx_client::evidence::ProofBundleSelector::Receipt(target.activity_id()),
            scope.correlation,
            registry,
        )
        .map_err(|_| refuse(ErrorClass::VerificationFailure, "read.invalid_proof"))?;
    let header = layerx_wire::receipt::decode_batch_header(&bundle.signed_header().canonical_bytes)
        .map_err(|_| refuse(ErrorClass::VerificationFailure, "read.invalid_proof"))?;
    if bundle.canonical_bytes() != scope.served_receipt || header.batch_number() == 0 {
        return Err(refuse(
            ErrorClass::VerificationFailure,
            "read.target_mismatch",
        ));
    }
    // A native PB1 Merkle path cannot establish an EVM receipts-trie root.
    Err(refuse(
        ErrorClass::ProtocolIncompatibility,
        "read.proof_bundle_v2_material_missing",
    ))
}

pub(crate) fn acquire_at_committed_block(
    endpoint: &SequencerEndpoint,
    request: &ProofBundleRequest<'_>,
) -> Result<ServedProofBundle, CheckpointReadError> {
    let account = match request.target {
        ProofBundleTarget::AccountState { account_id, .. } => {
            committed(request)?.ok_or(CheckpointReadError::NotCommitted)?;
            Some(account_address(account_id)?)
        }
        _ => None,
    };
    if let Some(account) = account {
        let proof = endpoint
            .account_proof(&account, &[], request.header.height)
            .map_err(CheckpointReadError::Rpc)?;
        verify_bundle(&ProofBundleRequest {
            account_proof: Some(&proof),
            ..*request
        })
    } else {
        verify_bundle(request)
    }
}

fn account_address(account: [u8; 32]) -> Result<[u8; 20], CheckpointReadError> {
    if account[..12] != [0; 12] || account[12..] == [0; 20] {
        return Err(CheckpointReadError::TargetMismatch);
    }
    account[12..]
        .try_into()
        .map_err(|_| CheckpointReadError::TargetMismatch)
}

fn committed(
    request: &ProofBundleRequest<'_>,
) -> Result<Option<ServedCheckpoint>, CheckpointReadError> {
    let Some(commit) = request.commit else {
        return if matches!(request.target, ProofBundleTarget::AccountState { .. }) {
            Err(CheckpointReadError::NotCommitted)
        } else {
            Ok(None)
        };
    };
    if request.receipt.metadata.rung != Rung::Committed {
        return Err(CheckpointReadError::NotCommitted);
    }
    serve_checkpoint(
        request.receipt.metadata.block_height,
        request.chain_id,
        request.header,
        commit,
        request.validators,
    )
    .map(Some)
}

pub(crate) fn verify_bundle(
    request: &ProofBundleRequest<'_>,
) -> Result<ServedProofBundle, CheckpointReadError> {
    let checkpoint = committed(request)?;
    let receipt = request.receipt;
    let kind = match request.target {
        ProofBundleTarget::Activity(_) => ProofBundleKind::Activity,
        ProofBundleTarget::Receipt(_) => ProofBundleKind::Receipt,
        ProofBundleTarget::AccountState { .. } => ProofBundleKind::State,
    };
    let rung = if kind == ProofBundleKind::State {
        Rung::Committed
    } else {
        Rung::Receipted
    };
    if rung < request.requested_rung {
        return Err(CheckpointReadError::BelowRequestedRung);
    }
    if receipt.metadata.rung < Rung::Receipted
        || request.target.activity_id() == [0; 32]
        || request.target.activity_id() != receipt.metadata.transaction_hash
        || keccak256(request.raw_transaction) != receipt.metadata.transaction_hash
        || request.header.height == 0
        || request.header.height > i64::MAX as u64
        || request.chain_id.is_empty()
        || request.chain_id.len() > 50
        || request.header.height != receipt.metadata.block_height
        || request.header.chain_id != request.chain_id
        || request.header.last_results_hash != Some(request.receipts_root)
    {
        return Err(CheckpointReadError::TargetMismatch);
    }
    let mut size = request.raw_transaction.len();
    for length in [
        receipt.canonical_receipt.len(),
        request.receipt_public_key.len(),
        1024,
    ]
    .into_iter()
    .chain(request.receipt_proof.iter().map(Vec::len))
    .chain(request.validators.iter().map(|_| 60))
    .chain(
        request
            .commit
            .into_iter()
            .flat_map(|commit| commit.votes.iter().map(|_| 128)),
    )
    .chain(
        request
            .account_proof
            .into_iter()
            .flat_map(|proof| proof.account_proof.iter().map(Vec::len)),
    ) {
        size = size
            .checked_add(length)
            .ok_or(CheckpointReadError::Arithmetic)?;
    }
    if size > MAX_PROOF_BUNDLE_BYTES {
        return Err(CheckpointReadError::Arithmetic);
    }
    let digest: [u8; 32] = Sha256::digest(&receipt.canonical_receipt).into();
    let signature = receipt.metadata.receipt_signature[..64]
        .try_into()
        .map_err(|_| CheckpointReadError::InvalidProof)?;
    layerx_crypto::secp256k1::verify_recoverable_digest(
        request.receipt_public_key,
        &signature,
        receipt.metadata.receipt_signature[64],
        &digest,
    )
    .map_err(|_| CheckpointReadError::InvalidProof)?;
    verify_receipt(receipt)?;
    let key = rlp_index(receipt.metadata.tx_index);
    let included = trie_value(request.receipts_root, &key, request.receipt_proof)?;
    if included.as_deref() != Some(receipt.canonical_receipt.as_slice()) {
        return Err(CheckpointReadError::RootMismatch);
    }
    let (account_proof, state_root) =
        if let ProofBundleTarget::AccountState { account_id, .. } = request.target {
            let account = account_address(account_id)?;
            let proof = request
                .account_proof
                .ok_or(CheckpointReadError::InvalidProof)?;
            if checkpoint.is_none() {
                return Err(CheckpointReadError::NotCommitted);
            }
            verify_account(proof, account, request.header.app_hash)?;
            (Some(proof.clone()), Some(request.header.app_hash))
        } else {
            if request.account_proof.is_some() {
                return Err(CheckpointReadError::TargetMismatch);
            }
            (None, None)
        };
    Ok(ServedProofBundle {
        format: "layerx-receipt-proof-v2",
        kind,
        raw_transaction: request.raw_transaction.to_vec(),
        receipt: receipt.clone(),
        receipt_proof: request.receipt_proof.to_vec(),
        receipts_root: request.receipts_root,
        block_header: request.header.clone(),
        checkpoint,
        validator_set: request.validators.to_vec(),
        account_proof,
        state_root,
        verification_level: rung,
    })
}

fn verify_receipt(
    receipt: &crate::receipt::TransactionReceiptEvidence,
) -> Result<(), CheckpointReadError> {
    let bytes = receipt.canonical_receipt.as_slice();
    let payload = match bytes.first() {
        Some(1..=4) => &bytes[1..],
        Some(0xc0..=0xff) => bytes,
        _ => return Err(CheckpointReadError::InvalidProof),
    };
    let fields = list_items(payload)?;
    if fields.len() != 4
        || fields[0].bytes()?
            != if receipt.metadata.status {
                &[1][..]
            } else {
                &[][..]
            }
        || fields[2].bytes()? != receipt.metadata.logs_bloom
        || word(fields[1].bytes()?)? < u64_word(receipt.metadata.gas_used)
    {
        return Err(CheckpointReadError::InvalidProof);
    }
    for log in list_items(fields[3].raw)? {
        let fields = list_items(log.raw)?;
        if fields.len() != 3 || fields[0].bytes()?.len() != 20 {
            return Err(CheckpointReadError::InvalidProof);
        }
        let topics = list_items(fields[1].raw)?;
        if topics.len() > 4
            || topics
                .iter()
                .any(|topic| topic.bytes().map_or(true, |bytes| bytes.len() != 32))
        {
            return Err(CheckpointReadError::InvalidProof);
        }
        fields[2].bytes()?;
    }
    Ok(())
}

fn verify_account(
    proof: &AccountProof,
    account: [u8; 20],
    root: [u8; 32],
) -> Result<(), CheckpointReadError> {
    if proof.address != account || !proof.storage.is_empty() {
        return Err(CheckpointReadError::TargetMismatch);
    }
    let value = trie_value(root, &keccak256(&account), &proof.account_proof)?
        .ok_or(CheckpointReadError::InvalidProof)?;
    let fields = list_items(&value)?;
    if fields.len() != 4
        || word(fields[0].bytes()?)? != u64_word(proof.nonce)
        || word(fields[1].bytes()?)? != proof.balance
        || fields[2].bytes()? != proof.storage_hash
        || fields[3].bytes()? != proof.code_hash
    {
        return Err(CheckpointReadError::InvalidProof);
    }
    Ok(())
}

fn u64_word(value: u64) -> [u8; 32] {
    let mut word = [0; 32];
    word[24..].copy_from_slice(&value.to_be_bytes());
    word
}

fn rlp_index(value: u64) -> Vec<u8> {
    if value == 0 {
        return vec![0x80];
    }
    if value < 128 {
        return vec![value as u8];
    }
    let bytes = value.to_be_bytes();
    let bytes = &bytes[bytes.iter().position(|byte| *byte != 0).unwrap_or(7)..];
    let mut encoded = vec![0x80 + bytes.len() as u8];
    encoded.extend_from_slice(bytes);
    encoded
}

fn word(bytes: &[u8]) -> Result<[u8; 32], CheckpointReadError> {
    if bytes.len() > 32 || bytes.first() == Some(&0) {
        return Err(CheckpointReadError::InvalidProof);
    }
    let mut value = [0; 32];
    value[32 - bytes.len()..].copy_from_slice(bytes);
    Ok(value)
}

#[derive(Clone, Copy)]
struct RlpItem<'a> {
    raw: &'a [u8],
    payload: &'a [u8],
    list: bool,
}

impl<'a> RlpItem<'a> {
    fn bytes(self) -> Result<&'a [u8], CheckpointReadError> {
        if self.list {
            Err(CheckpointReadError::InvalidProof)
        } else {
            Ok(self.payload)
        }
    }
}

fn take_item<'a>(input: &mut &'a [u8]) -> Result<RlpItem<'a>, CheckpointReadError> {
    let original = *input;
    let (&prefix, rest) = original
        .split_first()
        .ok_or(CheckpointReadError::InvalidProof)?;
    let list = prefix >= 0xc0;
    let (offset, length) = match prefix {
        0..=0x7f => (0_usize, 1_usize),
        0x80..=0xb7 => (1, usize::from(prefix - 0x80)),
        0xc0..=0xf7 => (1, usize::from(prefix - 0xc0)),
        _ => {
            let width = usize::from(prefix - if list { 0xf7 } else { 0xb7 });
            let bytes = rest.get(..width).ok_or(CheckpointReadError::InvalidProof)?;
            if bytes.first() == Some(&0) {
                return Err(CheckpointReadError::InvalidProof);
            }
            let length = bytes
                .iter()
                .try_fold(0_usize, |length, byte| {
                    length.checked_mul(256)?.checked_add(usize::from(*byte))
                })
                .ok_or(CheckpointReadError::InvalidProof)?;
            if length < 56 {
                return Err(CheckpointReadError::InvalidProof);
            }
            (1 + width, length)
        }
    };
    let end = offset
        .checked_add(length)
        .ok_or(CheckpointReadError::InvalidProof)?;
    let raw = original
        .get(..end)
        .ok_or(CheckpointReadError::InvalidProof)?;
    let payload = &raw[offset..];
    if prefix == 0x81 && payload[0] < 0x80 {
        return Err(CheckpointReadError::InvalidProof);
    }
    *input = &original[end..];
    Ok(RlpItem { raw, payload, list })
}

fn list_items(bytes: &[u8]) -> Result<Vec<RlpItem<'_>>, CheckpointReadError> {
    let mut remaining = bytes;
    let item = take_item(&mut remaining)?;
    if !remaining.is_empty() || !item.list {
        return Err(CheckpointReadError::InvalidProof);
    }
    remaining = item.payload;
    let mut items = Vec::new();
    while !remaining.is_empty() {
        items.push(take_item(&mut remaining)?);
    }
    Ok(items)
}

fn nibbles(bytes: &[u8]) -> Vec<u8> {
    bytes
        .iter()
        .flat_map(|byte| [byte >> 4, byte & 15])
        .collect()
}

fn trie_value(
    root: [u8; 32],
    key: &[u8],
    proof: &[Vec<u8>],
) -> Result<Option<Vec<u8>>, CheckpointReadError> {
    let path = nibbles(key);
    if proof.len() > path.len() + 1
        || proof
            .iter()
            .any(|node| node.is_empty() || node.len() > MAX_PROOF_BUNDLE_BYTES)
    {
        return Err(CheckpointReadError::InvalidProof);
    }
    let mut offset = 0;
    let mut used = 0;
    let mut reference = root.to_vec();
    let mut hashed = true;
    for _ in 0..=path.len() {
        let node = if hashed {
            let node = proof.get(used).ok_or(CheckpointReadError::InvalidProof)?;
            if keccak256(node).as_slice() != reference || used > 0 && node.len() < 32 {
                return Err(CheckpointReadError::RootMismatch);
            }
            used += 1;
            node.as_slice()
        } else {
            reference.as_slice()
        };
        let fields = list_items(node)?;
        let child = match fields.len() {
            17 => {
                if offset == path.len() {
                    let value = fields[16].bytes()?;
                    if used != proof.len() {
                        return Err(CheckpointReadError::InvalidProof);
                    }
                    return Ok((!value.is_empty()).then(|| value.to_vec()));
                }
                let child = fields[usize::from(path[offset])];
                offset += 1;
                child
            }
            2 => {
                let compact = fields[0].bytes()?;
                let (&first, _) = compact
                    .split_first()
                    .ok_or(CheckpointReadError::InvalidProof)?;
                if first >> 4 > 3 || first & 0x10 == 0 && first & 15 != 0 {
                    return Err(CheckpointReadError::InvalidProof);
                }
                let expanded = nibbles(compact);
                let segment = &expanded[if first & 0x10 == 0 { 2 } else { 1 }..];
                let leaf = first & 0x20 != 0;
                if !leaf && segment.is_empty() {
                    return Err(CheckpointReadError::InvalidProof);
                }
                if !path[offset..].starts_with(segment) {
                    return Err(CheckpointReadError::InvalidProof);
                }
                offset += segment.len();
                if leaf {
                    if used != proof.len() || offset != path.len() {
                        return Err(CheckpointReadError::InvalidProof);
                    }
                    return Ok(Some(fields[1].bytes()?.to_vec()));
                }
                fields[1]
            }
            _ => return Err(CheckpointReadError::InvalidProof),
        };
        if child.list {
            if child.raw.len() >= 32 {
                return Err(CheckpointReadError::InvalidProof);
            }
            reference = child.raw.to_vec();
            hashed = false;
        } else {
            if child.payload.len() != 32 {
                return Err(CheckpointReadError::InvalidProof);
            }
            reference = child.payload.to_vec();
            hashed = true;
        }
    }
    Err(CheckpointReadError::InvalidProof)
}
