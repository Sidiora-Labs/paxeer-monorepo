//! Module state reads over JSON-RPC.

use layerx_crypto::evm_transaction::transaction_hash as keccak256;
use layerx_proof::inclusion::verify_header;
use layerx_types::verify::VerificationLevel;
use layerx_wire::receipt::decode_batch_header;

use crate::evidence::{AccountEvidencePolicy, EvidenceError, RootSelector, SignedHeader};
use crate::handover::SequencerHistory;
use crate::sequencer::{AccountProof, SequencerEndpoint, SequencerError};

pub type StorageProof = crate::sequencer::StorageProof;

#[derive(Clone, Copy, Debug)]
pub struct ModuleReadContext<'a> {
    pub precompile: [u8; 20],
    pub committed_height: u64,
    pub signed_header: &'a SignedHeader,
    pub policy: AccountEvidencePolicy,
}

#[derive(Clone, Debug)]
pub struct VerifiedModuleEvidence {
    state_root: [u8; 32],
    level: VerificationLevel,
    signed_header: SignedHeader,
    checkpoint_id: Option<[u8; 32]>,
}

impl VerifiedModuleEvidence {
    #[must_use]
    pub const fn state_root(&self) -> [u8; 32] {
        self.state_root
    }

    #[must_use]
    pub const fn level(&self) -> VerificationLevel {
        self.level
    }

    #[must_use]
    pub const fn signed_header(&self) -> &SignedHeader {
        &self.signed_header
    }

    #[must_use]
    pub const fn checkpoint_id(&self) -> Option<[u8; 32]> {
        self.checkpoint_id
    }
}

#[derive(Clone, Debug)]
pub struct ModuleStateValue {
    canonical_bytes: [u8; 32],
    proof: AccountProof,
    evidence: VerifiedModuleEvidence,
}

impl ModuleStateValue {
    #[must_use]
    pub const fn canonical_bytes(&self) -> &[u8; 32] {
        &self.canonical_bytes
    }

    #[must_use]
    pub const fn proof(&self) -> &AccountProof {
        &self.proof
    }

    #[must_use]
    pub const fn evidence(&self) -> &VerifiedModuleEvidence {
        &self.evidence
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum ModuleReadError {
    Rpc(SequencerError),
    SelectorMismatch,
    InvalidProof,
    Header(EvidenceError),
    History,
}

/// # Errors
/// Refuses invalid selectors, substituted accounts and invalid trie or header proofs.
pub fn module_state(
    endpoint: &SequencerEndpoint,
    module_id: u16,
    key: &[u8],
    context: ModuleReadContext<'_>,
) -> Result<ModuleStateValue, ModuleReadError> {
    read(endpoint, module_id, key, context, None)
}

/// # Errors
/// Also refuses headers outside the genesis-authenticated signing history.
pub fn module_state_with_history(
    endpoint: &SequencerEndpoint,
    module_id: u16,
    key: &[u8],
    context: ModuleReadContext<'_>,
    history: &SequencerHistory,
) -> Result<ModuleStateValue, ModuleReadError> {
    read(endpoint, module_id, key, context, Some(history))
}

fn read(
    endpoint: &SequencerEndpoint,
    module_id: u16,
    key: &[u8],
    context: ModuleReadContext<'_>,
    history: Option<&SequencerHistory>,
) -> Result<ModuleStateValue, ModuleReadError> {
    let slot = module_slot(module_id, key)?;
    let proof = endpoint
        .account_proof(&context.precompile, &[slot], context.committed_height)
        .map_err(ModuleReadError::Rpc)?;
    let evidence = if let Some(history) = history {
        verify_module_evidence_with_history(&proof, module_id, key, context, history)?
    } else {
        verify_evidence(&proof, module_id, key, context, None)?
    };
    Ok(ModuleStateValue {
        canonical_bytes: proof.storage[0].value,
        proof,
        evidence,
    })
}

/// # Errors
/// Refuses any slot, account, value, root, height or historical signer substitution.
pub fn verify_module_evidence_with_history(
    proof: &AccountProof,
    module_id: u16,
    key: &[u8],
    context: ModuleReadContext<'_>,
    history: &SequencerHistory,
) -> Result<VerifiedModuleEvidence, ModuleReadError> {
    verify_evidence(proof, module_id, key, context, Some(history))
}

fn module_slot(module_id: u16, key: &[u8]) -> Result<[u8; 32], ModuleReadError> {
    if module_id > 9 || key.is_empty() || key.len() > 129 {
        return Err(ModuleReadError::SelectorMismatch);
    }
    let mut preimage = module_id.to_be_bytes().to_vec();
    preimage.extend_from_slice(key);
    Ok(keccak256(&preimage))
}

fn verify_evidence(
    proof: &AccountProof,
    module_id: u16,
    key: &[u8],
    context: ModuleReadContext<'_>,
    history: Option<&SequencerHistory>,
) -> Result<VerifiedModuleEvidence, ModuleReadError> {
    let slot = module_slot(module_id, key)?;
    if proof.address != context.precompile
        || proof.storage.len() != 1
        || proof.storage[0].key != slot
    {
        return Err(ModuleReadError::SelectorMismatch);
    }
    let signed = context.signed_header;
    let header =
        decode_batch_header(&signed.canonical_bytes).map_err(|_| ModuleReadError::InvalidProof)?;
    if header.batch_number() != context.committed_height
        || context.committed_height == 0
        || !matches!(context.policy.root_selector, RootSelector::Latest)
            && context.policy.root_selector != RootSelector::Batch(context.committed_height)
    {
        return Err(ModuleReadError::SelectorMismatch);
    }
    let public_key = if let Some(history) = history {
        history
            .verify_header(&signed.canonical_bytes, &signed.signature)
            .map_err(|_| ModuleReadError::History)?;
        history
            .authorization_for_batch(context.committed_height)
            .map_err(|_| ModuleReadError::History)?
            .public_key()
    } else {
        context.policy.handshake_sequencer_key
    };
    let authorization = signed
        .pinned_key_authorization(
            public_key,
            context.policy.expected_protocol_version,
            context.policy.expected_network_id,
        )
        .map_err(ModuleReadError::Header)?;
    let verified = verify_header(&signed.canonical_bytes, &signed.signature, &authorization)
        .map_err(|error| ModuleReadError::Header(EvidenceError::Inclusion(error)))?;
    let state_root = verified.header().resulting_state_root();
    let account = trie_value(state_root, keccak256(&proof.address), &proof.account_proof)?
        .ok_or(ModuleReadError::InvalidProof)?;
    let fields = list_items(&account)?;
    if fields.len() != 4
        || word(fields[0].bytes()?)? != u64_word(proof.nonce)
        || word(fields[1].bytes()?)? != proof.balance
        || fields[2].bytes()? != proof.storage_hash
        || fields[3].bytes()? != proof.code_hash
    {
        return Err(ModuleReadError::InvalidProof);
    }
    let storage: &StorageProof = &proof.storage[0];
    let stored = trie_value(proof.storage_hash, keccak256(&slot), &storage.proof)?;
    let value = match stored {
        Some(bytes) => {
            let item = exact_item(&bytes)?;
            word(item.bytes()?)?
        }
        None => [0; 32],
    };
    if value != storage.value {
        return Err(ModuleReadError::InvalidProof);
    }
    Ok(VerifiedModuleEvidence {
        state_root,
        level: VerificationLevel::STATE_PROVEN,
        signed_header: signed.clone(),
        checkpoint_id: None,
    })
}

fn u64_word(value: u64) -> [u8; 32] {
    let mut word = [0; 32];
    word[24..].copy_from_slice(&value.to_be_bytes());
    word
}

fn word(bytes: &[u8]) -> Result<[u8; 32], ModuleReadError> {
    if bytes.len() > 32 || bytes.first() == Some(&0) {
        return Err(ModuleReadError::InvalidProof);
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
    fn bytes(self) -> Result<&'a [u8], ModuleReadError> {
        if self.list {
            Err(ModuleReadError::InvalidProof)
        } else {
            Ok(self.payload)
        }
    }
}

fn take_item<'a>(input: &mut &'a [u8]) -> Result<RlpItem<'a>, ModuleReadError> {
    let original = *input;
    let (&prefix, rest) = original
        .split_first()
        .ok_or(ModuleReadError::InvalidProof)?;
    let list = prefix >= 0xc0;
    let (offset, length) = match prefix {
        0..=0x7f => (0, 1),
        0x80..=0xb7 => (1, usize::from(prefix - 0x80)),
        0xc0..=0xf7 => (1, usize::from(prefix - 0xc0)),
        _ => {
            let width = usize::from(prefix - if list { 0xf7 } else { 0xb7 });
            let bytes = rest.get(..width).ok_or(ModuleReadError::InvalidProof)?;
            if bytes.first() == Some(&0) {
                return Err(ModuleReadError::InvalidProof);
            }
            let length = bytes
                .iter()
                .try_fold(0_usize, |length, byte| {
                    length.checked_mul(256)?.checked_add(usize::from(*byte))
                })
                .ok_or(ModuleReadError::InvalidProof)?;
            if length < 56 {
                return Err(ModuleReadError::InvalidProof);
            }
            (1 + width, length)
        }
    };
    let end = offset
        .checked_add(length)
        .ok_or(ModuleReadError::InvalidProof)?;
    let raw = original.get(..end).ok_or(ModuleReadError::InvalidProof)?;
    let payload = &raw[offset..];
    if prefix == 0x81 && payload[0] < 0x80 {
        return Err(ModuleReadError::InvalidProof);
    }
    *input = &original[end..];
    Ok(RlpItem { raw, payload, list })
}

fn exact_item(bytes: &[u8]) -> Result<RlpItem<'_>, ModuleReadError> {
    let mut remaining = bytes;
    let item = take_item(&mut remaining)?;
    if !remaining.is_empty() {
        return Err(ModuleReadError::InvalidProof);
    }
    Ok(item)
}

fn list_items(bytes: &[u8]) -> Result<Vec<RlpItem<'_>>, ModuleReadError> {
    let item = exact_item(bytes)?;
    if !item.list {
        return Err(ModuleReadError::InvalidProof);
    }
    let mut remaining = item.payload;
    let mut items = Vec::new();
    while !remaining.is_empty() {
        if items.len() == 17 {
            return Err(ModuleReadError::InvalidProof);
        }
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
    key: [u8; 32],
    proof: &[Vec<u8>],
) -> Result<Option<Vec<u8>>, ModuleReadError> {
    if proof.len() > 65 || proof.iter().any(|node| node.len() > 1024) {
        return Err(ModuleReadError::InvalidProof);
    }
    if root == keccak256(&[0x80]) && proof.is_empty() {
        return Ok(None);
    }
    let path = nibbles(&key);
    let mut offset = 0;
    let mut reference = root.to_vec();
    let mut hashed = true;
    for _ in 0..65 {
        let node = if hashed {
            let node = proof
                .iter()
                .find(|node| keccak256(node) == reference.as_slice())
                .ok_or(ModuleReadError::InvalidProof)?;
            if offset != 0 && node.len() < 32 {
                return Err(ModuleReadError::InvalidProof);
            }
            node.as_slice()
        } else {
            reference.as_slice()
        };
        let fields = list_items(node)?;
        let child = match fields.len() {
            17 => {
                if offset == path.len() {
                    let value = fields[16].bytes()?;
                    return Ok((!value.is_empty()).then(|| value.to_vec()));
                }
                let child = fields[usize::from(path[offset])];
                offset += 1;
                child
            }
            2 => {
                let compact = fields[0].bytes()?;
                let (&first, _) = compact.split_first().ok_or(ModuleReadError::InvalidProof)?;
                if first >> 4 > 3 || first & 0x10 == 0 && first & 15 != 0 {
                    return Err(ModuleReadError::InvalidProof);
                }
                let expanded = nibbles(compact);
                let segment = &expanded[if first & 0x10 == 0 { 2 } else { 1 }..];
                let leaf = first & 0x20 != 0;
                if !leaf && segment.is_empty() {
                    return Err(ModuleReadError::InvalidProof);
                }
                if !path[offset..].starts_with(segment) {
                    return Ok(None);
                }
                offset += segment.len();
                if leaf {
                    return Ok((offset == path.len())
                        .then(|| fields[1].bytes().map(<[u8]>::to_vec))
                        .transpose()?);
                }
                fields[1]
            }
            _ => return Err(ModuleReadError::InvalidProof),
        };
        if child.list {
            if child.raw.len() >= 32 {
                return Err(ModuleReadError::InvalidProof);
            }
            reference = child.raw.to_vec();
            hashed = false;
        } else {
            match child.payload.len() {
                0 if fields.len() == 17 => return Ok(None),
                32 => {
                    reference = child.payload.to_vec();
                    hashed = true;
                }
                _ => return Err(ModuleReadError::InvalidProof),
            }
        }
    }
    Err(ModuleReadError::InvalidProof)
}
