//! Proof-only offline exports of receipts at one committed block height.

use std::collections::BTreeSet;
use std::sync::Mutex;
use std::time::Duration;

use layerx_agent_api::export::{FactRef, OfflineExport};
use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::proof::ProofBundle;
use layerx_agent_api::read::{Freshness, RelativeTo, VerifiedRead};
use layerx_client::availability::FetchError;
use layerx_client::evidence::EvidenceError;
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_client::Client;
use layerx_crypto::evm_transaction::transaction_hash;
use layerx_proof::export::{CompleteExportError, OfflineTrustError};
use layerx_proof::export_codec::{
    parse_fact_set, ArtifactDecodeError, ExportCodecError, FactRefError, FactSelector,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{decode_summary, verify_logs, SequencerLog};
use layerx_types::payload::ModuleRegistry;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use layerx_wire::hash::receipt_digest;
use layerx_wire::receipt::encode_unsigned;
use serde_json::{json, Value};

use crate::protocol_evidence::{EvidenceAuthority, ReceiptEvidenceError};
use crate::read::{
    checkpoint, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator,
    CommitVote,
};
use crate::store::{ObjectKind, Store, TenantId, TenantKey};

pub const MAX_EXPORT_BYTES: usize = crate::agent_rpc::MAX_BODY_BYTES;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ExportTrustSource {
    receipt_keys: Vec<ReceiptKeyEntry>,
    validator_set: Vec<CommitValidator>,
}

impl ExportTrustSource {
    pub fn new(
        receipt_keys: Vec<ReceiptKeyEntry>,
        validator_set: Vec<CommitValidator>,
    ) -> Result<Self, OfflineTrustError> {
        let mut keys = BTreeSet::new();
        let mut validators = BTreeSet::new();
        if receipt_keys.is_empty()
            || validator_set.is_empty()
            || receipt_keys.iter().any(|key| {
                key.key_id == [0; 32]
                    || key.public_key == [0; 32]
                    || key.first_height > key.last_height
                    || !keys.insert(key.key_id)
            })
            || validator_set.iter().any(|validator| {
                validator.voting_power == 0
                    || validator.address != CommitValidator::address_for_key(&validator.public_key)
                    || !validators.insert(validator.address)
            })
        {
            return Err(OfflineTrustError::NetworkId);
        }
        Ok(Self {
            receipt_keys,
            validator_set,
        })
    }

    pub fn receipt_keys(&self) -> &[ReceiptKeyEntry] {
        &self.receipt_keys
    }
    pub fn validator_set(&self) -> &[CommitValidator] {
        &self.validator_set
    }

    pub fn trust(&self, _registry: ModuleRegistry) -> Result<Self, OfflineTrustError> {
        Ok(self.clone())
    }
}

pub struct ExportOwnerContext<'a> {
    pub store: &'a Mutex<Store>,
    pub tenant: &'a TenantId,
    pub bound_account: Option<[u8; 32]>,
    pub registry: &'a ModuleRegistry,
    pub authority: &'a EvidenceAuthority,
    pub trust: &'a ExportTrustSource,
    pub first_correlation: u64,
    pub availability_deadline: Duration,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerificationReport {
    pub verified_receipts: usize,
    pub verified_inclusions: usize,
    pub verified_checkpoints: usize,
    pub height: u64,
    pub block_hash: [u8; 32],
    pub achieved_rung: Rung,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BuiltExport {
    pub artifact: OfflineExport,
    pub local_verification: VerificationReport,
}

pub type ExportBuildError = ExportProduceError;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProducedExport {
    pub response: VerifiedRead<OfflineExport>,
    pub binary_bytes: usize,
    pub json_value_bytes: usize,
}

#[derive(Debug)]
pub enum ExportProduceError {
    UnavailableCapability(layerx_agent_api::error::ApiError),
    Facts(FactRefError),
    SettlementAnchoringUnavailable,
    NotOwned {
        fact: usize,
    },
    AccountNotBound {
        fact: usize,
    },
    Store,
    Correlation,
    Evidence {
        fact: usize,
        error: EvidenceError,
    },
    ReceiptEvidence {
        fact: usize,
        error: ReceiptEvidenceError,
    },
    Binding {
        fact: usize,
    },
    MaintenanceLink {
        fact: usize,
    },
    Availability {
        fact: usize,
        error: FetchError,
    },
    AvailabilityIncomplete {
        fact: usize,
    },
    Codec(ExportCodecError),
    Artifact(ArtifactDecodeError),
    ConflictingRecord,
    MixedSnapshot,
    StaleHead,
    /// Explicit bounded-response refusal: the complete export exceeds the frame bound.
    Oversize,
    Verification(CompleteExportError),
    Contract,
}

pub const fn require_frame_within_bound(
    serialized_frame_bytes: usize,
) -> Result<(), ExportProduceError> {
    if serialized_frame_bytes > MAX_EXPORT_BYTES {
        Err(ExportProduceError::Oversize)
    } else {
        Ok(())
    }
}

pub fn build(
    artifact: OfflineExport,
    trust: &ExportTrustSource,
) -> Result<BuiltExport, ExportBuildError> {
    let local_verification = verify(&artifact, trust)?;
    require_frame_within_bound(serialize_export(&artifact)?.len())?;
    Ok(BuiltExport {
        artifact,
        local_verification,
    })
}

pub fn verify(
    artifact: &OfflineExport,
    trust: &ExportTrustSource,
) -> Result<VerificationReport, ExportBuildError> {
    artifact
        .clone()
        .validate()
        .map_err(|_| ExportProduceError::Contract)?;
    let texts: Vec<_> = artifact.facts.iter().map(FactRef::as_str).collect();
    let selectors = parse_fact_set(&texts).map_err(ExportProduceError::Facts)?;
    if artifact.height == 0 || selectors.len() != artifact.bundles.len() {
        return Err(ExportProduceError::Contract);
    }
    let mut block = None;
    for (fact, (selector, bundle)) in selectors.iter().zip(&artifact.bundles).enumerate() {
        let refusal = || ExportProduceError::Binding { fact };
        if bundle.verification_level != Rung::Committed
            || !trust.receipt_keys.contains(&bundle.receipt_key)
        {
            return Err(refusal());
        }
        let keys: Vec<_> = trust
            .receipt_keys
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
        let receipt = verify_sequencer_signature(bundle.signed_receipt.as_bytes(), keys.as_slice())
            .map_err(|_| refusal())?;
        let protocol = receipt.protocol().ok_or_else(refusal)?;
        let mut height = [0; 32];
        height[24..].copy_from_slice(&artifact.height.to_be_bytes());
        if protocol.activity_id() != selector.activity_id()
            || transaction_hash(bundle.raw_transaction.as_bytes()) != protocol.activity_id()
            || protocol.batch_id() != height
            || protocol.context_hash() != bundle.receipt_key.key_id
            || receipt_digest(&encode_unsigned(&receipt).map_err(|_| refusal())?)
                .map_err(|_| refusal())?
                != bundle.receipt_digest
        {
            return Err(refusal());
        }
        let header = decode_header(required(&bundle.block_header)?)?;
        let validators = decode_validators(required(&bundle.validator_set)?)?;
        if validators != trust.validator_set {
            return Err(refusal());
        }
        let commit = decode_commit(required(&bundle.commit)?, &validators)?;
        let checkpoint = checkpoint(
            artifact.height,
            &header.chain_id,
            &header,
            &commit,
            &validators,
        )
        .map_err(|_| refusal())?;
        if block.is_some_and(|hash| hash != checkpoint.block_hash) {
            return Err(ExportProduceError::MixedSnapshot);
        }
        block = Some(checkpoint.block_hash);
        let inclusion = list_items(required(&bundle.receipt_inclusion)?)?;
        if inclusion.len() != 2 {
            return Err(refusal());
        }
        let evm_receipt = inclusion[0].bytes()?;
        let proof: Vec<_> = list_items(inclusion[1].raw)?
            .iter()
            .map(|item| item.bytes().map(<[u8]>::to_vec))
            .collect::<Result<_, _>>()?;
        let summary = decode_summary(&receipt).map_err(|_| refusal())?;
        let root = header.last_results_hash.ok_or_else(refusal)?;
        if trie_value(root, &rlp_index(u64::from(summary.tx_index())), &proof)?.as_deref()
            != Some(evm_receipt)
        {
            return Err(refusal());
        }
        let payload = match evm_receipt.first() {
            Some(1..=4) => &evm_receipt[1..],
            Some(0xc0..=0xff) => evm_receipt,
            _ => return Err(refusal()),
        };
        let fields = list_items(payload)?;
        if fields.len() != 4
            || fields[0].bytes()?
                != if protocol.result_code() == 0 {
                    &[1][..]
                } else {
                    &[][..]
                }
            || word(fields[1].bytes()?)? != u64_word(summary.cumulative_gas_used())
            || fields[2].bytes()?.len() != 256
        {
            return Err(refusal());
        }
        let mut logs = Vec::new();
        for (index, log) in list_items(fields[3].raw)?.iter().enumerate() {
            let fields = list_items(log.raw)?;
            if fields.len() != 3 {
                return Err(refusal());
            }
            let address = fields[0].bytes()?.try_into().map_err(|_| refusal())?;
            let topics = list_items(fields[1].raw)?
                .iter()
                .map(|topic| topic.bytes()?.try_into().map_err(|_| refusal()))
                .collect::<Result<_, _>>()?;
            logs.push(SequencerLog::new(
                address,
                topics,
                fields[2].bytes()?.to_vec(),
                index as u64,
            ));
        }
        verify_logs(&receipt, logs).map_err(|_| refusal())?;
        match selector {
            FactSelector::State { account_id, .. } => {
                verify_account(
                    required(&bundle.account_proof)?,
                    *account_id,
                    header.app_hash,
                )?;
            }
            FactSelector::Checkpoint { batch_number, .. } if *batch_number != artifact.height => {
                return Err(refusal())
            }
            _ if bundle.account_proof.is_some() => return Err(refusal()),
            _ => {}
        }
    }
    Ok(VerificationReport {
        verified_receipts: selectors.len(),
        verified_inclusions: selectors.len(),
        verified_checkpoints: 1,
        height: artifact.height,
        block_hash: block.ok_or(ExportProduceError::Contract)?,
        achieved_rung: Rung::Committed,
    })
}

fn required(value: &Option<CanonicalBytes>) -> Result<&[u8], ExportProduceError> {
    value
        .as_ref()
        .map(CanonicalBytes::as_bytes)
        .ok_or(ExportProduceError::Contract)
}

pub fn produce(
    _node: &mut Client,
    context: &ExportOwnerContext<'_>,
    facts: &[FactRef],
    requested: VerificationLevel,
) -> Result<ProducedExport, ExportProduceError> {
    requested.rung().ok_or(ExportProduceError::Contract)?;
    produce_retained(
        context.store,
        context.tenant,
        context.bound_account,
        context.trust,
        facts,
    )
}

pub fn produce_retained(
    store: &Mutex<Store>,
    tenant: &TenantId,
    bound_account: Option<[u8; 32]>,
    trust: &ExportTrustSource,
    facts: &[FactRef],
) -> Result<ProducedExport, ExportProduceError> {
    let texts: Vec<_> = facts.iter().map(FactRef::as_str).collect();
    let selectors = parse_fact_set(&texts).map_err(ExportProduceError::Facts)?;
    let mut bundles = Vec::new();
    let mut height = None;
    {
        let store = store.lock().map_err(|_| ExportProduceError::Store)?;
        for (fact, selector) in selectors.iter().enumerate() {
            if let FactSelector::State { account_id, .. } = selector {
                if bound_account != Some(*account_id) {
                    return Err(ExportProduceError::AccountNotBound { fact });
                }
            }
            let value = store
                .get(&bundle_key(tenant.clone(), selector)?)
                .ok_or(ExportProduceError::NotOwned { fact })?;
            let artifact = deserialize_export(value.bytes())?;
            if artifact.facts != [facts[fact].clone()] || artifact.bundles.len() != 1 {
                return Err(ExportProduceError::Binding { fact });
            }
            if height.is_some_and(|previous| previous != artifact.height) {
                return Err(ExportProduceError::MixedSnapshot);
            }
            height = Some(artifact.height);
            bundles.extend(artifact.bundles);
        }
    }
    let built = build(
        OfflineExport {
            facts: facts.to_vec(),
            bundles,
            height: height.ok_or(ExportProduceError::Contract)?,
        },
        trust,
    )?;
    let json_value_bytes = serialize_export(&built.artifact)?.len();
    let binary_bytes = built
        .artifact
        .bundles
        .iter()
        .try_fold(0_usize, |total, bundle| {
            [
                Some(&bundle.raw_transaction),
                Some(&bundle.signed_receipt),
                bundle.block_header.as_ref(),
                bundle.receipt_inclusion.as_ref(),
                bundle.commit.as_ref(),
                bundle.validator_set.as_ref(),
                bundle.account_proof.as_ref(),
            ]
            .into_iter()
            .flatten()
            .try_fold(total, |sum, bytes| {
                sum.checked_add(bytes.as_bytes().len())
                    .ok_or(ExportProduceError::Oversize)
            })
        })?;
    let height = built.local_verification.height;
    Ok(ProducedExport {
        response: VerifiedRead::new(
            built.artifact,
            Rung::Committed,
            Freshness {
                chain_head: height,
                value_height: height,
                relative_to: RelativeTo::Block {
                    height,
                    hash: built.local_verification.block_hash,
                },
            },
        ),
        binary_bytes,
        json_value_bytes,
    })
}

pub fn retain_bundle(
    store: &mut Store,
    tenant: TenantId,
    artifact: OfflineExport,
    trust: &ExportTrustSource,
) -> Result<(), ExportProduceError> {
    let built = build(artifact, trust)?;
    if built.artifact.facts.len() != 1 {
        return Err(ExportProduceError::Contract);
    }
    let selector = built.artifact.facts[0]
        .selector()
        .map_err(ExportProduceError::Facts)?;
    let key = bundle_key(tenant, &selector)?;
    let bytes = serialize_export(&built.artifact)?;
    if let Some(existing) = store.get(&key) {
        return if existing.bytes() == bytes {
            Ok(())
        } else {
            Err(ExportProduceError::ConflictingRecord)
        };
    }
    store
        .put_local(key, bytes)
        .map_err(|_| ExportProduceError::Store)
}

fn bundle_key(tenant: TenantId, selector: &FactSelector) -> Result<TenantKey, ExportProduceError> {
    TenantKey::new(
        tenant,
        ObjectKind::Configuration,
        [
            b"export-proof-v2:".as_slice(),
            selector.canonical_text().as_bytes(),
        ]
        .concat(),
    )
    .map_err(|_| ExportProduceError::Store)
}

pub fn serialize_export(artifact: &OfflineExport) -> Result<Vec<u8>, ExportProduceError> {
    let hex = |bytes: &[u8]| format!("0x{}", layerx_programs::hex::encode(bytes));
    let bundles: Vec<_> = artifact.bundles.iter().map(|bundle| {
        let mut value = json!({"format":bundle.format, "verificationLevel":bundle.verification_level.name(),
            "rawTransaction":hex(bundle.raw_transaction.as_bytes()), "signedReceipt":hex(bundle.signed_receipt.as_bytes()),
            "receiptDigest":hex(&bundle.receipt_digest), "receiptKey":{"keyId":hex(&bundle.receipt_key.key_id), "publicKey":hex(&bundle.receipt_key.public_key),
                "firstHeight":bundle.receipt_key.first_height.to_string(), "lastHeight":bundle.receipt_key.last_height.to_string()}});
        for (name, bytes) in [("blockHeader", &bundle.block_header), ("receiptInclusion", &bundle.receipt_inclusion), ("commit", &bundle.commit), ("validatorSet", &bundle.validator_set), ("accountProof", &bundle.account_proof)] {
            if let Some(bytes) = bytes { value[name] = Value::String(hex(bytes.as_bytes())); }
        }
        value
    }).collect();
    let value = json!({"facts": artifact.facts.iter().map(FactRef::as_str).collect::<Vec<_>>(), "bundles":bundles, "height":artifact.height.to_string()});
    let bytes = serde_json::to_vec(&value).map_err(|_| ExportProduceError::Contract)?;
    require_frame_within_bound(bytes.len())?;
    Ok(bytes)
}

pub fn deserialize_export(bytes: &[u8]) -> Result<OfflineExport, ExportProduceError> {
    require_frame_within_bound(bytes.len())?;
    let value: Value = serde_json::from_slice(bytes).map_err(|_| ExportProduceError::Contract)?;
    // The SDK decoder enforces the major-two field set. Keep the durable codec equally strict.
    let object = value.as_object().ok_or(ExportProduceError::Contract)?;
    if object.len() != 3
        || !["facts", "bundles", "height"]
            .iter()
            .all(|key| object.contains_key(*key))
    {
        return Err(ExportProduceError::Contract);
    }
    let facts = value["facts"]
        .as_array()
        .ok_or(ExportProduceError::Contract)?
        .iter()
        .map(|value| {
            FactRef::new(value.as_str().ok_or(ExportProduceError::Contract)?)
                .map_err(|_| ExportProduceError::Contract)
        })
        .collect::<Result<_, _>>()?;
    let mut bundles = Vec::new();
    for value in value["bundles"]
        .as_array()
        .ok_or(ExportProduceError::Contract)?
    {
        let object = value.as_object().ok_or(ExportProduceError::Contract)?;
        let names = [
            "format",
            "verificationLevel",
            "rawTransaction",
            "signedReceipt",
            "receiptDigest",
            "receiptKey",
            "blockHeader",
            "receiptInclusion",
            "commit",
            "validatorSet",
        ];
        if object.len() != names.len() + usize::from(object.contains_key("accountProof"))
            || names.iter().any(|key| !object.contains_key(*key))
        {
            return Err(ExportProduceError::Contract);
        }
        let key = &value["receiptKey"];
        let fields = key.as_object().ok_or(ExportProduceError::Contract)?;
        if fields.len() != 4
            || ["keyId", "publicKey", "firstHeight", "lastHeight"]
                .iter()
                .any(|name| !fields.contains_key(*name))
        {
            return Err(ExportProduceError::Contract);
        }
        let optional = |name: &str| object.get(name).map(json_bytes).transpose();
        bundles.push(ProofBundle {
            format: value["format"]
                .as_str()
                .ok_or(ExportProduceError::Contract)?
                .to_owned(),
            verification_level: Rung::from_name(
                value["verificationLevel"]
                    .as_str()
                    .ok_or(ExportProduceError::Contract)?,
            )
            .ok_or(ExportProduceError::Contract)?,
            raw_transaction: json_bytes(&value["rawTransaction"])?,
            signed_receipt: json_bytes(&value["signedReceipt"])?,
            receipt_digest: json_array(&value["receiptDigest"])?,
            receipt_key: ReceiptKeyEntry {
                key_id: json_array(&key["keyId"])?,
                public_key: json_array(&key["publicKey"])?,
                first_height: decimal(&key["firstHeight"])?,
                last_height: decimal(&key["lastHeight"])?,
            },
            block_header: optional("blockHeader")?,
            receipt_inclusion: optional("receiptInclusion")?,
            commit: optional("commit")?,
            validator_set: optional("validatorSet")?,
            account_proof: optional("accountProof")?,
        });
    }
    let artifact = OfflineExport {
        facts,
        bundles,
        height: decimal(&value["height"])?,
    };
    artifact
        .validate()
        .map_err(|_| ExportProduceError::Contract)
}
fn decimal(value: &Value) -> Result<u64, ExportProduceError> {
    let text = value.as_str().ok_or(ExportProduceError::Contract)?;
    let number: u64 = text.parse().map_err(|_| ExportProduceError::Contract)?;
    if number.to_string() != text {
        return Err(ExportProduceError::Contract);
    }
    Ok(number)
}
fn json_bytes(value: &Value) -> Result<CanonicalBytes, ExportProduceError> {
    let text = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .ok_or(ExportProduceError::Contract)?;
    if text.len() % 2 != 0
        || !text
            .bytes()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
    {
        return Err(ExportProduceError::Contract);
    }
    let bytes = text
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let digit = |byte| {
                if byte <= b'9' {
                    byte - b'0'
                } else {
                    byte - b'a' + 10
                }
            };
            digit(pair[0]) * 16 + digit(pair[1])
        })
        .collect();
    CanonicalBytes::new(bytes).map_err(|_| ExportProduceError::Contract)
}
fn json_array<const N: usize>(value: &Value) -> Result<[u8; N], ExportProduceError> {
    json_bytes(value)?
        .as_bytes()
        .try_into()
        .map_err(|_| ExportProduceError::Contract)
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

fn word(bytes: &[u8]) -> Result<[u8; 32], ExportProduceError> {
    if bytes.len() > 32 || bytes.first() == Some(&0) {
        return Err(ExportProduceError::Contract);
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
    fn bytes(self) -> Result<&'a [u8], ExportProduceError> {
        if self.list {
            Err(ExportProduceError::Contract)
        } else {
            Ok(self.payload)
        }
    }
}

fn take_item<'a>(input: &mut &'a [u8]) -> Result<RlpItem<'a>, ExportProduceError> {
    let original = *input;
    let (&prefix, rest) = original.split_first().ok_or(ExportProduceError::Contract)?;
    let list = prefix >= 0xc0;
    let (offset, length) = match prefix {
        0..=0x7f => (0_usize, 1_usize),
        0x80..=0xb7 => (1, usize::from(prefix - 0x80)),
        0xc0..=0xf7 => (1, usize::from(prefix - 0xc0)),
        _ => {
            let width = usize::from(prefix - if list { 0xf7 } else { 0xb7 });
            let bytes = rest.get(..width).ok_or(ExportProduceError::Contract)?;
            if bytes.first() == Some(&0) {
                return Err(ExportProduceError::Contract);
            }
            let length = bytes
                .iter()
                .try_fold(0_usize, |length, byte| {
                    length.checked_mul(256)?.checked_add(usize::from(*byte))
                })
                .ok_or(ExportProduceError::Contract)?;
            if length < 56 {
                return Err(ExportProduceError::Contract);
            }
            (1 + width, length)
        }
    };
    let end = offset
        .checked_add(length)
        .ok_or(ExportProduceError::Contract)?;
    let raw = original.get(..end).ok_or(ExportProduceError::Contract)?;
    let payload = &raw[offset..];
    if prefix == 0x81 && payload[0] < 0x80 {
        return Err(ExportProduceError::Contract);
    }
    *input = &original[end..];
    Ok(RlpItem { raw, payload, list })
}

fn list_items(bytes: &[u8]) -> Result<Vec<RlpItem<'_>>, ExportProduceError> {
    let mut remaining = bytes;
    let item = take_item(&mut remaining)?;
    if !remaining.is_empty() || !item.list {
        return Err(ExportProduceError::Contract);
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
) -> Result<Option<Vec<u8>>, ExportProduceError> {
    let path = nibbles(key);
    if proof.len() > path.len() + 1
        || proof.iter().any(|node| {
            node.is_empty() || node.len() > layerx_agent_api::proof::MAX_PROOF_BUNDLE_BYTES
        })
    {
        return Err(ExportProduceError::Contract);
    }
    let mut offset = 0;
    let mut used = 0;
    let mut reference = root.to_vec();
    let mut hashed = true;
    for _ in 0..=path.len() {
        let node = if hashed {
            let node = proof.get(used).ok_or(ExportProduceError::Contract)?;
            if transaction_hash(node).as_slice() != reference || used > 0 && node.len() < 32 {
                return Err(ExportProduceError::Contract);
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
                        return Err(ExportProduceError::Contract);
                    }
                    return Ok((!value.is_empty()).then(|| value.to_vec()));
                }
                let child = fields[usize::from(path[offset])];
                offset += 1;
                child
            }
            2 => {
                let compact = fields[0].bytes()?;
                let (&first, _) = compact.split_first().ok_or(ExportProduceError::Contract)?;
                if first >> 4 > 3 || first & 0x10 == 0 && first & 15 != 0 {
                    return Err(ExportProduceError::Contract);
                }
                let expanded = nibbles(compact);
                let segment = &expanded[if first & 0x10 == 0 { 2 } else { 1 }..];
                let leaf = first & 0x20 != 0;
                if !leaf && segment.is_empty() {
                    return Err(ExportProduceError::Contract);
                }
                if !path[offset..].starts_with(segment) {
                    return Err(ExportProduceError::Contract);
                }
                offset += segment.len();
                if leaf {
                    if used != proof.len() || offset != path.len() {
                        return Err(ExportProduceError::Contract);
                    }
                    return Ok(Some(fields[1].bytes()?.to_vec()));
                }
                fields[1]
            }
            _ => return Err(ExportProduceError::Contract),
        };
        if child.list {
            if child.raw.len() >= 32 {
                return Err(ExportProduceError::Contract);
            }
            reference = child.raw.to_vec();
            hashed = false;
        } else {
            if child.payload.len() != 32 {
                return Err(ExportProduceError::Contract);
            }
            reference = child.payload.to_vec();
            hashed = true;
        }
    }
    Err(ExportProduceError::Contract)
}

fn verify_account(
    bytes: &[u8],
    account_id: [u8; 32],
    root: [u8; 32],
) -> Result<(), ExportProduceError> {
    if account_id[..12] != [0; 12] || account_id[12..] == [0; 20] {
        return Err(ExportProduceError::Contract);
    }
    let fields = list_items(bytes)?;
    if fields.len() != 6 || fields[0].bytes()? != &account_id[12..] {
        return Err(ExportProduceError::Contract);
    }
    let proof = list_items(fields[5].raw)?
        .iter()
        .map(|item| item.bytes().map(<[u8]>::to_vec))
        .collect::<Result<Vec<_>, _>>()?;
    let value = trie_value(root, &transaction_hash(&account_id[12..]), &proof)?
        .ok_or(ExportProduceError::Contract)?;
    let account = list_items(&value)?;
    if account.len() != 4
        || account[0].bytes()? != fields[2].bytes()?
        || account[1].bytes()? != fields[1].bytes()?
        || account[2].bytes()? != fields[3].bytes()?
        || account[3].bytes()? != fields[4].bytes()?
    {
        return Err(ExportProduceError::Contract);
    }
    word(account[0].bytes()?)?;
    word(account[1].bytes()?)?;
    if account[2].bytes()?.len() != 32 || account[3].bytes()?.len() != 32 {
        return Err(ExportProduceError::Contract);
    }
    Ok(())
}

// CometBFT protobuf messages are decoded with bounded, canonical wire primitives.
#[derive(Clone, Copy)]
struct ProtoField<'a> {
    number: u64,
    integer: Option<u64>,
    bytes: &'a [u8],
}
fn varint(input: &mut &[u8]) -> Result<u64, ExportProduceError> {
    let mut value = 0;
    for index in 0..10 {
        let (&byte, rest) = input.split_first().ok_or(ExportProduceError::Contract)?;
        *input = rest;
        if index == 9 && byte > 1 {
            return Err(ExportProduceError::Contract);
        }
        value |= u64::from(byte & 127) << (index * 7);
        if byte < 128 {
            if index > 0 && byte == 0 {
                return Err(ExportProduceError::Contract);
            }
            return Ok(value);
        }
    }
    Err(ExportProduceError::Contract)
}
fn proto(mut bytes: &[u8]) -> Result<Vec<ProtoField<'_>>, ExportProduceError> {
    let mut fields = Vec::new();
    while !bytes.is_empty() {
        let tag = varint(&mut bytes)?;
        if tag >> 3 == 0 {
            return Err(ExportProduceError::Contract);
        }
        let (integer, data) = match tag & 7 {
            0 => (Some(varint(&mut bytes)?), &[][..]),
            2 => {
                let length = usize::try_from(varint(&mut bytes)?)
                    .map_err(|_| ExportProduceError::Contract)?;
                let data = bytes.get(..length).ok_or(ExportProduceError::Contract)?;
                bytes = &bytes[length..];
                (None, data)
            }
            _ => return Err(ExportProduceError::Contract),
        };
        fields.push(ProtoField {
            number: tag >> 3,
            integer,
            bytes: data,
        });
    }
    Ok(fields)
}
fn message<'a>(
    bytes: &'a [u8],
    allowed: &[u64],
) -> Result<Vec<ProtoField<'a>>, ExportProduceError> {
    let fields = proto(bytes)?;
    let mut seen = BTreeSet::new();
    if fields
        .iter()
        .any(|field| !allowed.contains(&field.number) || !seen.insert(field.number))
    {
        return Err(ExportProduceError::Contract);
    }
    Ok(fields)
}
fn pbytes<'a>(fields: &[ProtoField<'a>], number: u64) -> Result<&'a [u8], ExportProduceError> {
    match fields.iter().find(|field| field.number == number) {
        Some(field) if field.integer.is_none() => Ok(field.bytes),
        None => Ok(&[]),
        _ => Err(ExportProduceError::Contract),
    }
}
fn pint(fields: &[ProtoField<'_>], number: u64) -> Result<u64, ExportProduceError> {
    match fields.iter().find(|field| field.number == number) {
        Some(field) => field.integer.ok_or(ExportProduceError::Contract),
        None => Ok(0),
    }
}
fn fixed<const N: usize>(bytes: &[u8]) -> Result<[u8; N], ExportProduceError> {
    bytes.try_into().map_err(|_| ExportProduceError::Contract)
}
fn optional_hash(
    fields: &[ProtoField<'_>],
    number: u64,
) -> Result<Option<[u8; 32]>, ExportProduceError> {
    let bytes = pbytes(fields, number)?;
    if bytes.is_empty() {
        Ok(None)
    } else {
        fixed(bytes).map(Some)
    }
}
fn timestamp(bytes: &[u8]) -> Result<CommitTimestamp, ExportProduceError> {
    let fields = message(bytes, &[1, 2])?;
    Ok(CommitTimestamp {
        seconds: pint(&fields, 1)? as i64,
        nanos: u32::try_from(pint(&fields, 2)?).map_err(|_| ExportProduceError::Contract)?,
    })
}
fn block_id(bytes: &[u8]) -> Result<Option<BlockId>, ExportProduceError> {
    if bytes.is_empty() {
        return Ok(None);
    }
    let fields = message(bytes, &[1, 2])?;
    let parts = message(pbytes(&fields, 2)?, &[1, 2])?;
    if pbytes(&fields, 1)?.is_empty() && pint(&parts, 1)? == 0 && pbytes(&parts, 2)?.is_empty() {
        return Ok(None);
    }
    Ok(Some(BlockId {
        hash: fixed(pbytes(&fields, 1)?)?,
        part_set_total: u32::try_from(pint(&parts, 1)?)
            .map_err(|_| ExportProduceError::Contract)?,
        part_set_hash: fixed(pbytes(&parts, 2)?)?,
    }))
}
pub fn decode_header(bytes: &[u8]) -> Result<CheckpointHeader, ExportProduceError> {
    let fields = message(bytes, &[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14])?;
    let version = message(pbytes(&fields, 1)?, &[1, 2])?;
    Ok(CheckpointHeader {
        version_block: pint(&version, 1)?,
        version_app: pint(&version, 2)?,
        chain_id: std::str::from_utf8(pbytes(&fields, 2)?)
            .map_err(|_| ExportProduceError::Contract)?
            .to_owned(),
        height: pint(&fields, 3)?,
        time: timestamp(pbytes(&fields, 4)?)?,
        last_block_id: block_id(pbytes(&fields, 5)?)?,
        last_commit_hash: optional_hash(&fields, 6)?,
        data_hash: optional_hash(&fields, 7)?,
        validators_hash: fixed(pbytes(&fields, 8)?)?,
        next_validators_hash: fixed(pbytes(&fields, 9)?)?,
        consensus_hash: fixed(pbytes(&fields, 10)?)?,
        app_hash: fixed(pbytes(&fields, 11)?)?,
        last_results_hash: optional_hash(&fields, 12)?,
        evidence_hash: optional_hash(&fields, 13)?,
        proposer_address: fixed(pbytes(&fields, 14)?)?,
    })
}
pub fn decode_validators(bytes: &[u8]) -> Result<Vec<CommitValidator>, ExportProduceError> {
    let fields = proto(bytes)?;
    let mut validators = Vec::new();
    let mut proposer = None;
    let mut declared_total = None;
    for field in &fields {
        match (field.number, field.integer) {
            (1, None) => validators.push(decode_validator(field.bytes)?),
            (2, None) if proposer.is_none() => proposer = Some(decode_validator(field.bytes)?),
            (3, Some(total)) if declared_total.is_none() => declared_total = Some(total),
            _ => return Err(ExportProduceError::Contract),
        }
    }
    if validators.is_empty() || proposer.is_some_and(|value| !validators.contains(&value)) {
        return Err(ExportProduceError::Contract);
    }
    let total = validators.iter().try_fold(0_u64, |total, validator| {
        total
            .checked_add(validator.voting_power)
            .ok_or(ExportProduceError::Contract)
    })?;
    if declared_total.is_some_and(|declared| declared != total) {
        return Err(ExportProduceError::Contract);
    }
    Ok(validators)
}
fn decode_validator(bytes: &[u8]) -> Result<CommitValidator, ExportProduceError> {
    let fields = message(bytes, &[1, 2, 3, 4])?;
    let key = message(pbytes(&fields, 2)?, &[1])?;
    // Proposer priority is signed int64 and is outside validator identity.
    pint(&fields, 4)?;
    Ok(CommitValidator {
        address: fixed(pbytes(&fields, 1)?)?,
        public_key: fixed(pbytes(&key, 1)?)?,
        voting_power: pint(&fields, 3)?,
    })
}
pub fn decode_commit(
    bytes: &[u8],
    validators: &[CommitValidator],
) -> Result<BlockCommit, ExportProduceError> {
    let fields = proto(bytes)?;
    let mut singular = BTreeSet::new();
    if fields.iter().any(|field| {
        ![1, 2, 3, 4].contains(&field.number) || field.number != 4 && !singular.insert(field.number)
    }) {
        return Err(ExportProduceError::Contract);
    }
    let mut commit = BlockCommit {
        height: pint(&fields, 1)?,
        round: u32::try_from(pint(&fields, 2)?).map_err(|_| ExportProduceError::Contract)?,
        block_id: block_id(pbytes(&fields, 3)?)?.ok_or(ExportProduceError::Contract)?,
        votes: Vec::new(),
    };
    for (index, field) in fields.iter().filter(|field| field.number == 4).enumerate() {
        if field.integer.is_some() {
            return Err(ExportProduceError::Contract);
        }
        let vote = message(field.bytes, &[1, 2, 3, 4])?;
        let validator = validators.get(index).ok_or(ExportProduceError::Contract)?;
        match pint(&vote, 1)? {
            1 if pbytes(&vote, 2)?.is_empty() && pbytes(&vote, 4)?.is_empty() => {
                timestamp(pbytes(&vote, 3)?)?;
            }
            2 => {
                let address = fixed(pbytes(&vote, 2)?)?;
                if address != validator.address {
                    return Err(ExportProduceError::Contract);
                }
                commit.votes.push(CommitVote {
                    validator_address: address,
                    voting_power: validator.voting_power,
                    timestamp: timestamp(pbytes(&vote, 3)?)?,
                    signature: fixed(pbytes(&vote, 4)?)?,
                });
            }
            // A nil precommit is not a vote for the exported block.
            3 => {
                if pbytes(&vote, 2)? != validator.address || pbytes(&vote, 4)?.len() != 64 {
                    return Err(ExportProduceError::Contract);
                }
                timestamp(pbytes(&vote, 3)?)?;
            }
            _ => return Err(ExportProduceError::Contract),
        }
    }
    if fields.iter().filter(|field| field.number == 4).count() != validators.len() {
        return Err(ExportProduceError::Contract);
    }
    Ok(commit)
}
fn put_varint(out: &mut Vec<u8>, mut value: u64) {
    while value >= 128 {
        out.push((value as u8) | 128);
        value >>= 7;
    }
    out.push(value as u8);
}
fn put_int(out: &mut Vec<u8>, number: u64, value: u64) {
    if value != 0 {
        put_varint(out, number << 3);
        put_varint(out, value);
    }
}
fn put_bytes(out: &mut Vec<u8>, number: u64, value: &[u8]) {
    put_varint(out, number << 3 | 2);
    put_varint(out, value.len() as u64);
    out.extend_from_slice(value);
}
fn encode_time(time: CommitTimestamp) -> Vec<u8> {
    let mut out = Vec::new();
    put_int(&mut out, 1, time.seconds as u64);
    put_int(&mut out, 2, u64::from(time.nanos));
    out
}
fn encode_id(id: Option<BlockId>) -> Vec<u8> {
    let mut out = Vec::new();
    let mut parts = Vec::new();
    if let Some(id) = id {
        put_bytes(&mut out, 1, &id.hash);
        put_int(&mut parts, 1, u64::from(id.part_set_total));
        put_bytes(&mut parts, 2, &id.part_set_hash);
    }
    put_bytes(&mut out, 2, &parts);
    out
}
pub fn encode_header(header: &CheckpointHeader) -> Vec<u8> {
    let mut out = Vec::new();
    let mut version = Vec::new();
    put_int(&mut version, 1, header.version_block);
    put_int(&mut version, 2, header.version_app);
    put_bytes(&mut out, 1, &version);
    put_bytes(&mut out, 2, header.chain_id.as_bytes());
    put_int(&mut out, 3, header.height);
    put_bytes(&mut out, 4, &encode_time(header.time));
    put_bytes(&mut out, 5, &encode_id(header.last_block_id));
    for (number, hash) in [
        (6, header.last_commit_hash),
        (7, header.data_hash),
        (8, Some(header.validators_hash)),
        (9, Some(header.next_validators_hash)),
        (10, Some(header.consensus_hash)),
        (11, Some(header.app_hash)),
        (12, header.last_results_hash),
        (13, header.evidence_hash),
    ] {
        if let Some(hash) = hash {
            put_bytes(&mut out, number, &hash);
        }
    }
    put_bytes(&mut out, 14, &header.proposer_address);
    out
}
pub fn encode_validators(validators: &[CommitValidator]) -> Vec<u8> {
    let mut out = Vec::new();
    for validator in validators {
        let mut value = Vec::new();
        let mut key = Vec::new();
        put_bytes(&mut key, 1, &validator.public_key);
        put_bytes(&mut value, 1, &validator.address);
        put_bytes(&mut value, 2, &key);
        put_int(&mut value, 3, validator.voting_power);
        put_bytes(&mut out, 1, &value);
    }
    out
}
pub fn encode_commit(commit: &BlockCommit, validators: &[CommitValidator]) -> Vec<u8> {
    let mut out = Vec::new();
    put_int(&mut out, 1, commit.height);
    put_int(&mut out, 2, u64::from(commit.round));
    put_bytes(&mut out, 3, &encode_id(Some(commit.block_id)));
    for validator in validators {
        let mut value = Vec::new();
        if let Some(vote) = commit
            .votes
            .iter()
            .find(|vote| vote.validator_address == validator.address)
        {
            put_int(&mut value, 1, 2);
            put_bytes(&mut value, 2, &vote.validator_address);
            put_bytes(&mut value, 3, &encode_time(vote.timestamp));
            put_bytes(&mut value, 4, &vote.signature);
        } else {
            put_int(&mut value, 1, 1);
            put_bytes(
                &mut value,
                3,
                &encode_time(CommitTimestamp {
                    seconds: -62_135_596_800,
                    nanos: 0,
                }),
            );
        }
        put_bytes(&mut out, 4, &value);
    }
    out
}
