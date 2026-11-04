use layerx_agent_api::error::{ApiError, ErrorClass, ReasonCode, RequestId, Retriability};
use layerx_agent_api::proof::{ProofBundle, ProofBundleTarget};
use layerx_agent_api::read::VerifiedRead;
use layerx_agent_api::verify::Rung;
use layerx_types::payload::ModuleRegistry;

use layerx_agent_api::proof::MAX_PROOF_BUNDLE_BYTES;
use layerx_client::sequencer::{SequencerEndpoint, StorageWitness};
use layerx_client::storage_witness::{verify_storage_witness, StorageWitnessError};

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct AccountState {
    pub storage: Vec<([u8; 32], [u8; 32])>,
}
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
    node: &mut SequencerEndpoint,
    _registry: &ModuleRegistry,
    target: ProofBundleTarget,
    requested: Rung,
    scope: ProofBundleScope<'_>,
) -> Result<VerifiedRead<ProofBundle>, ApiError> {
    if scope.account == [0; 32]
        || scope.asset == [0; 32]
        || matches!(target, ProofBundleTarget::AccountState { account_id, .. } if account_id != scope.account)
    {
        return Err(read_refusal(
            scope.correlation,
            ErrorClass::PolicyRefusal,
            "read.target_mismatch",
        ));
    }
    acquire_native_receipt(
        node,
        target,
        requested,
        scope.retained_activity,
        scope.served_receipt,
        scope.correlation,
    )
}

fn read_refusal(correlation: u64, class: ErrorClass, reason: &'static str) -> ApiError {
    ApiError {
        class,
        protocol_result_code: None,
        retriability: Retriability::Terminal,
        request_id: RequestId(correlation),
        reason: ReasonCode::new(reason).expect("static read reason"),
    }
}

/// Binds retained native EIP-1559 bytes to the node's authenticated receipt.
/// Committed targets still require inclusion and storage material; a signed
/// receipt alone never promotes the returned verification level.
pub fn acquire_native_receipt(
    node: &SequencerEndpoint,
    target: ProofBundleTarget,
    requested: Rung,
    raw: &[u8],
    served_receipt: &[u8],
    correlation: u64,
) -> Result<VerifiedRead<ProofBundle>, ApiError> {
    let refuse = |class, reason| read_refusal(correlation, class, reason);
    target
        .encode()
        .map_err(|_| refuse(ErrorClass::PolicyRefusal, "read.target_mismatch"))?;
    let (call, _) = layerx_crypto::evm_transaction::decode_signed(raw)
        .map_err(|_| refuse(ErrorClass::VerificationFailure, "read.target_mismatch"))?;
    let chain_id = node
        .chain_id()
        .map_err(|_| refuse(ErrorClass::VerificationFailure, "read.invalid_proof"))?;
    if call.chain_id != chain_id || keccak256(raw) != target.activity_id() {
        return Err(refuse(
            ErrorClass::VerificationFailure,
            "read.target_mismatch",
        ));
    }
    let keys = node
        .receipt_keys()
        .map_err(|_| refuse(ErrorClass::VerificationFailure, "read.invalid_proof"))?;
    let layerx_client::rpc::receipt::Lookup::Verified(receipt) =
        layerx_client::rpc::receipt::lookup_authenticated(
            node,
            target.activity_id(),
            layerx_client::rpc::receipt::LookupContext {
                receipt_keys: &keys,
            },
        )
        .map_err(|_| refuse(ErrorClass::VerificationFailure, "read.invalid_proof"))?
    else {
        return Err(refuse(
            ErrorClass::VerificationFailure,
            "read.invalid_proof",
        ));
    };
    if receipt.canonical_bytes() != served_receipt || receipt.height() == 0 {
        return Err(refuse(
            ErrorClass::VerificationFailure,
            "read.target_mismatch",
        ));
    }
    if matches!(target, ProofBundleTarget::AccountState { .. }) {
        return Err(refuse(
            ErrorClass::ProtocolIncompatibility,
            "read.account_proof_material_missing",
        ));
    }
    if requested > Rung::Receipted {
        return Err(refuse(
            ErrorClass::VerificationFailure,
            "read.below_requested_rung",
        ));
    }
    let height = node
        .block_number()
        .map_err(|_| refuse(ErrorClass::VerificationFailure, "read.invalid_proof"))?;
    let block = node
        .block_by_number(height)
        .map_err(|_| refuse(ErrorClass::VerificationFailure, "read.invalid_proof"))?
        .ok_or_else(|| refuse(ErrorClass::VerificationFailure, "read.invalid_proof"))?;
    crate::human_runtime::receipt_proof_operation(
        target,
        requested,
        raw,
        served_receipt,
        &keys,
        chain_id,
        height,
        block.hash,
    )
    .map_err(|_| refuse(ErrorClass::VerificationFailure, "read.invalid_proof"))
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
        let requested = request
            .account_proof
            .ok_or(CheckpointReadError::InvalidProof)?;
        let slots: Vec<_> = requested.iter().map(|witness| witness.slot).collect();
        let proof = endpoint
            .storage_witness(&account, &slots, request.receipt.metadata.block_height)
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
        request.header.height,
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
    request
        .target
        .encode()
        .map_err(|_| CheckpointReadError::TargetMismatch)?;
    let checkpoint = committed(request)?;
    let receipt = request.receipt;
    let kind = match request.target {
        ProofBundleTarget::Activity(_) => ProofBundleKind::Activity,
        ProofBundleTarget::Receipt(_) => ProofBundleKind::Receipt,
        ProofBundleTarget::AccountState { .. } => ProofBundleKind::State,
    };
    let rung = if checkpoint.is_some() {
        Rung::Committed
    } else {
        Rung::Receipted
    };
    if rung < request.requested_rung || rung < request.target.required_rung() {
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
        || receipt.metadata.block_height.checked_add(1) != Some(request.header.height)
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
    .chain(request.account_proof.into_iter().flat_map(|proofs| {
        proofs.iter().flat_map(|witness| {
            witness
                .ops
                .iter()
                .map(|op| op.key.len() + op.data.len() + op.op_type.len())
        })
    })) {
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
    let proof = super::AbciResultProof::decode(request.receipt_proof)?;
    proof.verify(request.receipts_root, receipt.metadata.tx_index)?;
    verify_result_receipt(&proof.result, receipt)?;

    let (account_proof, state_root) =
        if let ProofBundleTarget::AccountState { account_id, .. } = request.target {
            let account = account_address(account_id)?;
            let proof = request
                .account_proof
                .ok_or(CheckpointReadError::InvalidProof)?;
            let checkpoint = checkpoint
                .as_ref()
                .ok_or(CheckpointReadError::NotCommitted)?;
            let state = verify_account(proof, account, checkpoint)?;
            (Some(state), Some(request.header.app_hash))
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

fn verify_result_receipt(
    result: &[u8],
    receipt: &crate::receipt::TransactionReceiptEvidence,
) -> Result<(), CheckpointReadError> {
    use super::abci_result::{fields, ProtoValue};
    let response = super::abci_result::evm_response(result)?;
    let mut gas = 0;
    let mut vm_error = &[][..];
    let mut hash = None;
    let mut logs = Vec::new();
    let mut previous = 0;
    for (number, value) in fields(response)? {
        if number < previous || number == previous && number != 5 {
            return Err(CheckpointReadError::InvalidProof);
        }
        previous = number;
        match (number, value) {
            (1, ProtoValue::Integer(value)) => gas = value,
            (2, ProtoValue::Bytes(value)) => vm_error = value,
            (3, ProtoValue::Bytes(_)) => {}
            (4, ProtoValue::Bytes(value)) => hash = Some(value),
            (5, ProtoValue::Bytes(value)) => logs.push(value),
            _ => return Err(CheckpointReadError::InvalidProof),
        }
    }
    let text = std::str::from_utf8(hash.ok_or(CheckpointReadError::InvalidProof)?)
        .map_err(|_| CheckpointReadError::InvalidProof)?;
    let hash = layerx_programs::hex::decode_digest(
        text.strip_prefix("0x")
            .ok_or(CheckpointReadError::InvalidProof)?,
    )
    .map_err(|_| CheckpointReadError::InvalidProof)?;
    if hash != receipt.metadata.transaction_hash
        || gas != receipt.metadata.gas_used
        || vm_error.is_empty() != receipt.metadata.status
    {
        return Err(CheckpointReadError::TargetMismatch);
    }
    let bytes = receipt.canonical_receipt.as_slice();
    let fields_rlp = list_items(if matches!(bytes.first(), Some(1..=4)) {
        &bytes[1..]
    } else {
        bytes
    })?;
    if fields_rlp.len() != 4 {
        return Err(CheckpointReadError::InvalidProof);
    }
    let receipt_logs = list_items(fields_rlp[3].raw)?;
    if receipt_logs.len() != logs.len() {
        return Err(CheckpointReadError::TargetMismatch);
    }
    for (wire, expected) in logs.into_iter().zip(receipt_logs) {
        let expected = list_items(expected.raw)?;
        if expected.len() != 3 {
            return Err(CheckpointReadError::InvalidProof);
        }
        let mut address = None;
        let mut topics = Vec::new();
        let mut data = &[][..];
        let mut previous = 0;
        for (number, value) in fields(wire)? {
            if number < previous || number == previous && number != 2 {
                return Err(CheckpointReadError::InvalidProof);
            }
            previous = number;
            let hex = |bytes| -> Result<Vec<u8>, CheckpointReadError> {
                let text =
                    std::str::from_utf8(bytes).map_err(|_| CheckpointReadError::InvalidProof)?;
                layerx_programs::hex::decode(text.strip_prefix("0x").unwrap_or(text))
                    .map_err(|_| CheckpointReadError::InvalidProof)
            };
            match (number, value) {
                (1, ProtoValue::Bytes(bytes)) => address = Some(hex(bytes)?),
                (2, ProtoValue::Bytes(bytes)) => topics.push(hex(bytes)?),
                (3, ProtoValue::Bytes(bytes)) => data = bytes,
                (4, ProtoValue::Integer(_)) | (5, ProtoValue::Integer(0 | 1)) => {}
                _ => return Err(CheckpointReadError::InvalidProof),
            }
        }
        let expected_topics = list_items(expected[1].raw)?;
        if address.as_deref() != Some(expected[0].bytes()?)
            || data != expected[2].bytes()?
            || topics.len() != expected_topics.len()
        {
            return Err(CheckpointReadError::TargetMismatch);
        }
        for (topic, expected) in topics.iter().zip(expected_topics) {
            if topic.as_slice() != expected.bytes()? {
                return Err(CheckpointReadError::TargetMismatch);
            }
        }
    }
    Ok(())
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
    witnesses: &[StorageWitness],
    account: [u8; 20],
    checkpoint: &ServedCheckpoint,
) -> Result<AccountState, CheckpointReadError> {
    if witnesses.is_empty() {
        return Err(CheckpointReadError::InvalidProof);
    }
    let mut storage = Vec::with_capacity(witnesses.len());
    for witness in witnesses {
        if witness.address != account || storage.iter().any(|(slot, _)| *slot == witness.slot) {
            return Err(CheckpointReadError::TargetMismatch);
        }
        verify_storage_witness(witness, checkpoint.app_hash).map_err(|error| match error {
            StorageWitnessError::StoreRoot | StorageWitnessError::AppHash => {
                CheckpointReadError::RootMismatch
            }
            _ => CheckpointReadError::InvalidProof,
        })?;
        storage.push((witness.slot, witness.value));
    }
    Ok(AccountState { storage })
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

/// Native acquisition uses the node's Comet result producer and, for account
/// targets, explicit storage slots at the receipt's committed state height.
pub struct NativeProofRequest<'a> {
    pub target: ProofBundleTarget,
    pub requested_rung: Rung,
    pub raw_transaction: &'a [u8],
    pub served_receipt: &'a [u8],
    pub chain_id: &'a str,
    pub validators: &'a [super::CommitValidator],
    pub account_slots: &'a [[u8; 32]],
}

impl NativeProofRequest<'_> {
    pub fn acquire(
        &self,
        endpoint: &SequencerEndpoint,
        source: &super::NativeResultSource,
    ) -> Result<VerifiedRead<ProofBundle>, CheckpointReadError> {
        use layerx_agent_api::prepare::CanonicalBytes;
        use layerx_agent_api::read::{Freshness, RelativeTo};
        self.target
            .encode()
            .map_err(|_| CheckpointReadError::TargetMismatch)?;
        let (call, signature) = layerx_crypto::evm_transaction::decode_signed(self.raw_transaction)
            .map_err(|_| CheckpointReadError::TargetMismatch)?;
        let numeric_chain = endpoint.chain_id().map_err(CheckpointReadError::Rpc)?;
        let sender = layerx_crypto::evm_transaction::recover_sender(&call, &signature)
            .map_err(|_| CheckpointReadError::TargetMismatch)?;
        if self.requested_rung > Rung::Committed
            || call.chain_id != numeric_chain
            || keccak256(self.raw_transaction) != self.target.activity_id()
        {
            return Err(CheckpointReadError::TargetMismatch);
        }
        let keys = endpoint.receipt_keys().map_err(CheckpointReadError::Rpc)?;
        let layerx_client::rpc::receipt::Lookup::Verified(receipt) =
            layerx_client::rpc::receipt::lookup_authenticated(
                endpoint,
                self.target.activity_id(),
                layerx_client::rpc::receipt::LookupContext {
                    receipt_keys: &keys,
                },
            )
            .map_err(|_| CheckpointReadError::InvalidProof)?
        else {
            return Err(CheckpointReadError::InvalidProof);
        };
        if receipt.canonical_bytes() != self.served_receipt
            || receipt.height() == 0
            || receipt.execution().from != sender
            || receipt.execution().nonce != call.nonce
        {
            return Err(CheckpointReadError::TargetMismatch);
        }
        let block = endpoint
            .block_by_number(receipt.height())
            .map_err(CheckpointReadError::Rpc)?
            .ok_or(CheckpointReadError::NotCommitted)?;
        let commitment_height = receipt
            .height()
            .checked_add(1)
            .ok_or(CheckpointReadError::Arithmetic)?;
        let checkpoint = source.checkpoint(commitment_height, self.chain_id, self.validators)?;
        if checkpoint.header.last_block_id.map(|id| id.hash) != Some(block.hash)
            || checkpoint.header.last_results_hash != Some(block.receipts_root)
        {
            return Err(CheckpointReadError::CheckpointBlockMismatch);
        }
        let inclusion = source.acquire(receipt.height(), receipt.execution().index)?;
        inclusion.verify_committed(
            receipt.height(),
            receipt.execution().index,
            &checkpoint,
            self.chain_id,
            self.validators,
        )?;
        super::abci_result::verify_native_receipt(&inclusion.result, &receipt)?;
        let canonical = |bytes: Vec<u8>| {
            CanonicalBytes::new(bytes).map_err(|_| CheckpointReadError::Arithmetic)
        };
        let account_proof = if let ProofBundleTarget::AccountState { account_id, .. } = self.target
        {
            if self.account_slots.is_empty() || self.account_slots.len() > 1024 {
                return Err(CheckpointReadError::InvalidProof);
            }
            let address = account_address(account_id)?;
            let witnesses = endpoint
                .storage_witness(&address, self.account_slots, receipt.height())
                .map_err(CheckpointReadError::Rpc)?;
            verify_account(&witnesses, address, &checkpoint)?;
            let rows: Vec<_> = witnesses
                .iter()
                .map(|witness| {
                    serde_json::json!({
                        "address": layerx_programs::hex::encode(&witness.address),
                        "slot": layerx_programs::hex::encode(&witness.slot),
                        "value": layerx_programs::hex::encode(&witness.value),
                        "ops": witness.ops.iter().map(|op| serde_json::json!({
                            "type": op.op_type, "key": layerx_programs::hex::encode(&op.key),
                            "data": layerx_programs::hex::encode(&op.data)
                        })).collect::<Vec<_>>()
                    })
                })
                .collect();
            Some(canonical(
                serde_json::to_vec(&rows).map_err(|_| CheckpointReadError::InvalidProof)?,
            )?)
        } else {
            if !self.account_slots.is_empty() {
                return Err(CheckpointReadError::TargetMismatch);
            }
            None
        };
        let commitment = checkpoint.commitment_proof()?;
        let native_proof = serde_json::json!({
            "algorithm": "comet-abci-results-v1", "height": receipt.height(),
            "total": inclusion.proof.total, "index": inclusion.proof.index,
            "result": layerx_programs::hex::encode(&inclusion.result),
            "leaf_hash": layerx_programs::hex::encode(&inclusion.proof.leaf_hash),
            "aunts": inclusion.proof.aunts.iter().map(|aunt| layerx_programs::hex::encode(aunt)).collect::<Vec<_>>()
        });
        let key = *keys
            .iter()
            .find(|key| {
                key.key_id == receipt.execution().key_id
                    && key.first_height <= receipt.height()
                    && receipt.height() <= key.last_height
            })
            .ok_or(CheckpointReadError::InvalidProof)?;
        let unsigned = layerx_wire::receipt::encode_unsigned(receipt.receipt())
            .map_err(|_| CheckpointReadError::InvalidProof)?;
        let bundle = ProofBundle {
            format: "layerx-receipt-proof-v2".into(),
            verification_level: Rung::Committed,
            raw_transaction: canonical(self.raw_transaction.to_vec())?,
            signed_receipt: canonical(receipt.canonical_bytes().to_vec())?,
            receipt_digest: layerx_wire::hash::receipt_digest(&unsigned)
                .map_err(|_| CheckpointReadError::InvalidProof)?,
            receipt_key: key,
            block_header: Some(canonical(commitment.signed_header)?),
            receipt_inclusion: Some(canonical(
                serde_json::to_vec(&native_proof).map_err(|_| CheckpointReadError::InvalidProof)?,
            )?),
            commit: Some(canonical(commitment.commit)?),
            validator_set: Some(canonical(commitment.validator_set)?),
            account_proof,
        };
        bundle
            .validate()
            .map_err(|_| CheckpointReadError::InvalidProof)?;
        let head = endpoint.block_number().map_err(CheckpointReadError::Rpc)?;
        if head < commitment_height {
            return Err(CheckpointReadError::NotCommitted);
        }
        Ok(VerifiedRead::new(
            bundle,
            Rung::Committed,
            Freshness {
                chain_head: head,
                value_height: receipt.height(),
                relative_to: RelativeTo::Block {
                    height: block.number,
                    hash: block.hash,
                },
            },
        ))
    }
}
