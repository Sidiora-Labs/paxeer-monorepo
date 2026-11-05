//! Verified, byte-preserving receipt storage and protocol-code classification.

use layerx_proof::merkle::{decode_proof, encode_proof};
use layerx_proof::receipt::{verify_outcome, AuthorizedBatch, ReceiptCheck, VerifiedReceipt};
use layerx_types::result::{KnownResult, ResultCode, ResultDomain, Retriability};
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use sha2::{Digest, Sha256};

use crate::protocol_evidence::RawReceiptEvidence;
use crate::store::{ObjectKind, Store, StoreError, TenantId, TenantKey};

const METADATA_MAGIC: &[u8; 4] = b"LXRM";
const EVIDENCE_MAGIC: &[u8; 4] = b"LXRE";
const EVIDENCE_VERSION: u8 = 2;
const EVIDENCE_PREFIX: &[u8] = b"receipt-evidence:";
const LEGACY_EVIDENCE_PREFIX: &[u8] = b"receipt-legacy-evidence:";
/// Algorithm tag in byte 64 of a node Ed25519 signature; EVM recovery ids remain unchanged.
pub const NODE_ED25519_SIGNATURE_TAG: u8 = 255;
const IDEMPOTENCY_INDEX_PREFIX: &[u8] = b"receipt:idempotency:";

/// Persisted evidence and index identities used by restart recovery.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReceiptEvidenceRecord<E = RawReceiptEvidence> {
    pub idempotency_key: [u8; 32],
    pub activity_id: [u8; 32],
    pub global_sequence: u64,
    pub evidence: E,
}

pub type CurrentReceiptEvidenceRecord = ReceiptEvidenceRecord<TransactionReceiptEvidence>;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TransactionReceiptMetadata {
    pub transaction_hash: [u8; 32],
    pub block_height: u64,
    pub tx_index: u64,
    pub status: bool,
    pub gas_used: u64,
    pub logs_bloom: [u8; 256],
    pub receipt_signature: [u8; 65],
    pub rung: Rung,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TransactionReceiptEvidence {
    pub metadata: TransactionReceiptMetadata,
    pub canonical_receipt: Vec<u8>,
}

/// A v2 ingress capability derived only from the node verifier's pinned-key result.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthenticatedTransactionReceipt {
    evidence: TransactionReceiptEvidence,
}

impl AuthenticatedTransactionReceipt {
    pub fn from_authenticated(
        receipt: &layerx_client::rpc::receipt::AuthenticatedReceipt,
    ) -> Result<Self, ReceiptStoreError> {
        if (receipt.rung() == Rung::Committed || receipt.covering_block().is_some())
            && receipt.committed_header().is_none()
        {
            return Err(ReceiptStoreError::UnauthenticatedIngress);
        }
        Ok(Self {
            evidence: TransactionReceiptEvidence::from_authenticated(receipt)?,
        })
    }

    #[must_use]
    pub fn evidence(&self) -> &TransactionReceiptEvidence {
        &self.evidence
    }
}

/// Every served receipt of one tenant split by its persisted evidence kind.
/// A receipt without evidence is an older store record that
/// recovery must hold rather than skip.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct ReceiptEvidenceInventory {
    pub with_evidence: Vec<ReceiptEvidenceRecord>,
    pub with_current_evidence: Vec<CurrentReceiptEvidenceRecord>,
    pub without_evidence: Vec<ReceiptMetadata>,
}

/// One of the three durable receipt indexes.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ReceiptLookupKey {
    Activity([u8; 32]),
    Idempotency([u8; 32]),
    GlobalSequence(u64),
}

/// Lossless protocol rejection classification.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ResultClassification {
    pub code: ResultCode,
    pub canonical: Option<KnownResult>,
    pub domain: ResultDomain,
    pub retriability: Retriability,
    pub retry_permitted: bool,
}

/// Metadata recorded only after local proof verification succeeds.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ReceiptMetadata {
    pub activity_id: [u8; 32],
    pub idempotency_key: [u8; 32],
    pub global_sequence: u64,
    pub verification_level: VerificationLevel,
    pub result: ResultClassification,
}

/// Exact stored bytes coupled to their achieved evidence and protocol result.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ServedReceipt {
    pub canonical_bytes: Vec<u8>,
    pub metadata: ReceiptMetadata,
}

#[derive(Debug)]
pub enum ReceiptStoreError {
    Verification(ReceiptCheck),
    Store(StoreError),
    Corrupt,
    Missing,
    EvidenceVersion,
    BelowReceipted,
    UnauthenticatedIngress,
}

impl ReceiptStoreError {
    #[must_use]
    pub const fn reason(&self) -> &'static str {
        match self {
            Self::UnauthenticatedIngress => "receipt.unauthenticated_ingress",
            Self::EvidenceVersion => "receipt.evidence_version",
            Self::BelowReceipted => "receipt.not_receipted",
            Self::Verification(_) => "receipt.verification",
            Self::Store(_) => "receipt.store",
            Self::Corrupt => "receipt.corrupt",
            Self::Missing => "receipt.missing",
        }
    }
}

impl From<StoreError> for ReceiptStoreError {
    fn from(value: StoreError) -> Self {
        Self::Store(value)
    }
}

/// Classifies one exact protocol code without replacing its numeric value.
#[must_use]
pub const fn classify(code: ResultCode) -> ResultClassification {
    let retriability = code.retriability();
    ResultClassification {
        code,
        canonical: code.known(),
        domain: code.domain(),
        retriability,
        retry_permitted: code.raw() != 0 && matches!(retriability, Retriability::Retriable),
    }
}

/// Verifies through `layerx-proof`, then atomically stores exact bytes under
/// activity, idempotency, and global-sequence indexes.
///
/// # Errors
///
/// Returns the failed `layerx-proof` check when verification refuses the receipt, `Corrupt` when
/// the verified receipt carries no protocol section, and the store failure when an index key is
/// invalid or a durable index already holds conflicting bytes.
pub fn store(
    durable: &mut Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
    receipt_bytes: &[u8],
    authorised: &AuthorizedBatch,
) -> Result<ReceiptMetadata, ReceiptStoreError> {
    let verified = verify_outcome(receipt_bytes, authorised)
        .map_err(|failure| ReceiptStoreError::Verification(failure.check))?;
    persist_verified(durable, tenant, idempotency_key, &verified)
}

fn persist_verified(
    durable: &mut Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
    verified: &VerifiedReceipt,
) -> Result<ReceiptMetadata, ReceiptStoreError> {
    let protocol = verified
        .receipt()
        .protocol()
        .ok_or(ReceiptStoreError::Corrupt)?;
    let metadata = ReceiptMetadata {
        activity_id: protocol.activity_id(),
        idempotency_key,
        global_sequence: protocol.global_sequence(),
        verification_level: verified.level(),
        result: classify(ResultCode::from_raw(protocol.result_code())),
    };
    let digest: [u8; 32] = Sha256::digest(verified.canonical_bytes()).into();
    let indexes = [
        lookup_key(
            tenant.clone(),
            ReceiptLookupKey::Activity(metadata.activity_id),
        )?,
        lookup_key(
            tenant.clone(),
            ReceiptLookupKey::Idempotency(metadata.idempotency_key),
        )?,
        lookup_key(
            tenant.clone(),
            ReceiptLookupKey::GlobalSequence(metadata.global_sequence),
        )?,
    ];
    durable.record_verified_receipt(
        &indexes,
        verified.canonical_bytes(),
        metadata_key(tenant, digest)?,
        encode_metadata(metadata),
    )?;
    Ok(metadata)
}

/// Serves the exact core-produced receipt bytes through any durable index.
///
/// # Errors
///
/// Returns `Missing` when the index holds no receipt, and `Corrupt` when the paired metadata
/// record is absent or does not decode.
pub fn serve(
    durable: &Store,
    tenant: TenantId,
    lookup: ReceiptLookupKey,
) -> Result<ServedReceipt, ReceiptStoreError> {
    let index = lookup_key(tenant.clone(), lookup)?;
    let bytes = durable
        .get(&index)
        .ok_or(ReceiptStoreError::Missing)?
        .bytes()
        .to_vec();
    let digest: [u8; 32] = Sha256::digest(&bytes).into();
    let metadata = durable
        .get(&metadata_key(tenant, digest)?)
        .ok_or(ReceiptStoreError::Corrupt)
        .and_then(|value| decode_metadata(value.bytes()))?;
    Ok(ServedReceipt {
        canonical_bytes: bytes,
        metadata,
    })
}

/// Persists a newly observed proof-verified receipt exactly once, or validates
/// that an existing durable receipt is the same observation without lowering
/// any independently augmented finality level.
///
/// # Errors
///
/// Returns the underlying proof or store error for a first observation and
/// `Corrupt` when an existing index conflicts with the verified ingress.
pub fn store_verified_if_absent<V: ReceiptIngress>(
    durable: &mut Store,
    tenant: TenantId,
    key: [u8; 32],
    receipt_bytes: &[u8],
    verified: &V,
) -> Result<V::Stored, ReceiptStoreError> {
    verified.store_if_absent(durable, tenant, key, receipt_bytes)
}

mod ingress_seal {
    pub trait Sealed {}
}
impl ingress_seal::Sealed for AuthorizedBatch {}
impl ingress_seal::Sealed for TransactionReceiptEvidence {}
impl ingress_seal::Sealed for AuthenticatedTransactionReceipt {}
impl ingress_seal::Sealed for layerx_client::rpc::receipt::AuthenticatedReceipt {}

pub trait ReceiptIngress: ingress_seal::Sealed {
    type Stored;

    fn store_if_absent(
        &self,
        durable: &mut Store,
        tenant: TenantId,
        key: [u8; 32],
        receipt_bytes: &[u8],
    ) -> Result<Self::Stored, ReceiptStoreError>;
}

impl ReceiptIngress for AuthorizedBatch {
    type Stored = ServedReceipt;

    fn store_if_absent(
        &self,
        durable: &mut Store,
        tenant: TenantId,
        key: [u8; 32],
        receipt_bytes: &[u8],
    ) -> Result<Self::Stored, ReceiptStoreError> {
        let verified = verify_outcome(receipt_bytes, self)
            .map_err(|failure| ReceiptStoreError::Verification(failure.check))?;
        if !matches!(
            verified.level().rung(),
            Some(Rung::Receipted | Rung::Committed)
        ) {
            return Err(ReceiptStoreError::BelowReceipted);
        }
        store_kernel_verified_if_absent(durable, tenant, key, receipt_bytes, self)
    }
}

fn store_kernel_verified_if_absent(
    durable: &mut Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
    receipt_bytes: &[u8],
    authorised: &AuthorizedBatch,
) -> Result<ServedReceipt, ReceiptStoreError> {
    match serve(
        durable,
        tenant.clone(),
        ReceiptLookupKey::Idempotency(idempotency_key),
    ) {
        Ok(existing) => {
            let verified = verify_outcome(receipt_bytes, authorised)
                .map_err(|failure| ReceiptStoreError::Verification(failure.check))?;
            let protocol = verified
                .receipt()
                .protocol()
                .ok_or(ReceiptStoreError::Corrupt)?;
            if existing.canonical_bytes != verified.canonical_bytes()
                || existing.metadata.activity_id != protocol.activity_id()
                || existing.metadata.idempotency_key != idempotency_key
                || existing.metadata.global_sequence != protocol.global_sequence()
                || existing.metadata.result
                    != classify(ResultCode::from_raw(protocol.result_code()))
                || existing.metadata.verification_level < verified.level()
            {
                return Err(ReceiptStoreError::Corrupt);
            }
            Ok(existing)
        }
        Err(ReceiptStoreError::Missing) => {
            let metadata = store(
                durable,
                tenant.clone(),
                idempotency_key,
                receipt_bytes,
                authorised,
            )?;
            let served = serve(
                durable,
                tenant,
                ReceiptLookupKey::Idempotency(idempotency_key),
            )?;
            if served.canonical_bytes != receipt_bytes || served.metadata != metadata {
                return Err(ReceiptStoreError::Corrupt);
            }
            Ok(served)
        }
        Err(error) => Err(error),
    }
}

pub(crate) fn raise_verification_level(
    durable: &mut Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
    expected_receipt_bytes: &[u8],
    achieved: VerificationLevel,
) -> Result<ReceiptMetadata, ReceiptStoreError> {
    let served = serve(
        durable,
        tenant.clone(),
        ReceiptLookupKey::Idempotency(idempotency_key),
    )?;
    if served.canonical_bytes != expected_receipt_bytes {
        return Err(ReceiptStoreError::Corrupt);
    }
    let mut metadata = served.metadata;
    if achieved > metadata.verification_level {
        metadata.verification_level = achieved;
        let digest: [u8; 32] = Sha256::digest(expected_receipt_bytes).into();
        durable.put_local(metadata_key(tenant, digest)?, encode_metadata(metadata))?;
    }
    Ok(metadata)
}

/// Persists the raw receipt evidence for a receipt already served under
/// `idempotency_key`, exactly once. Re-persisting identical evidence is a no-op.
///
/// # Errors
///
/// Returns `Missing` when no receipt is served under the key, `Corrupt` when
/// the evidence does not carry the served receipt bytes or a differing
/// evidence record already exists, and the store failure otherwise.
pub fn persist_evidence(
    durable: &mut Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
    evidence: &RawReceiptEvidence,
) -> Result<ReceiptEvidenceRecord, ReceiptStoreError> {
    let served = serve(
        durable,
        tenant.clone(),
        ReceiptLookupKey::Idempotency(idempotency_key),
    )?;
    if served.canonical_bytes != evidence.canonical_receipt()
        || served.metadata.idempotency_key != idempotency_key
    {
        return Err(ReceiptStoreError::Corrupt);
    }
    let record = ReceiptEvidenceRecord {
        idempotency_key,
        activity_id: served.metadata.activity_id,
        global_sequence: served.metadata.global_sequence,
        evidence: evidence.clone(),
    };
    let public_key = evidence_key(tenant.clone(), idempotency_key)?;
    let key = match durable.get(&public_key) {
        Some(value) if value.bytes().get(4) == Some(&EVIDENCE_VERSION) => {
            legacy_evidence_key(tenant, idempotency_key)?
        }
        _ => public_key,
    };
    if let Some(existing) = durable.get(&key) {
        if decode_evidence(existing.bytes())? == record {
            return Ok(record);
        }
        return Err(ReceiptStoreError::Corrupt);
    }
    durable.put_local(key, encode_evidence(&record)?)?;
    Ok(record)
}

/// Serves version 2 receipt evidence by transaction hash, or `None` when absent.
///
/// # Errors
///
/// Returns `EvidenceVersion` for stale evidence and `Corrupt` when the evidence
/// does not decode or disagrees with the stored transaction receipt.
pub fn serve_evidence(
    durable: &Store,
    tenant: TenantId,
    transaction_hash: [u8; 32],
) -> Result<Option<TransactionReceiptEvidence>, ReceiptStoreError> {
    let Some(value) = durable.get(&evidence_key(tenant.clone(), transaction_hash)?) else {
        return Ok(None);
    };
    let record = decode_transaction_evidence(value.bytes())?;
    if record.metadata.transaction_hash != transaction_hash {
        return Err(ReceiptStoreError::Corrupt);
    }
    let receipt = durable
        .get(&transaction_key(tenant, transaction_hash)?)
        .ok_or(ReceiptStoreError::Corrupt)?;
    if receipt.bytes() != record.canonical_receipt {
        return Err(ReceiptStoreError::Corrupt);
    }
    Ok(Some(record))
}

fn serve_kernel_evidence(
    durable: &Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
) -> Result<Option<ReceiptEvidenceRecord>, ReceiptStoreError> {
    let served = serve(
        durable,
        tenant.clone(),
        ReceiptLookupKey::Idempotency(idempotency_key),
    )?;
    let legacy_key = legacy_evidence_key(tenant.clone(), idempotency_key)?;
    let public_key = evidence_key(tenant, idempotency_key)?;
    let value = if let Some(value) = durable.get(&legacy_key) {
        value
    } else if let Some(value) = durable.get(&public_key) {
        if value.bytes().get(..4) != Some(EVIDENCE_MAGIC.as_slice()) {
            return Err(ReceiptStoreError::Corrupt);
        }
        match value.bytes().get(4) {
            Some(1) => value,
            Some(&EVIDENCE_VERSION) => return Ok(None),
            Some(_) => return Err(ReceiptStoreError::EvidenceVersion),
            None => return Err(ReceiptStoreError::Corrupt),
        }
    } else {
        return Ok(None);
    };
    let record = decode_evidence(value.bytes())?;
    if record.idempotency_key != idempotency_key
        || record.activity_id != served.metadata.activity_id
        || record.global_sequence != served.metadata.global_sequence
        || record.evidence.canonical_receipt() != served.canonical_bytes.as_slice()
    {
        return Err(ReceiptStoreError::Corrupt);
    }
    Ok(Some(record))
}

/// Lists every served receipt of `tenant` with or without persisted evidence,
/// ordered by idempotency key.
///
/// # Errors
///
/// Returns `Corrupt` when an idempotency index or evidence record does not
/// decode, and the store failure otherwise.
pub fn evidence_inventory(
    durable: &Store,
    tenant: &TenantId,
) -> Result<ReceiptEvidenceInventory, ReceiptStoreError> {
    let mut inventory = ReceiptEvidenceInventory::default();
    for object_id in durable.list_object_ids(tenant, ObjectKind::Receipt) {
        let Some(suffix) = object_id.strip_prefix(IDEMPOTENCY_INDEX_PREFIX) else {
            continue;
        };
        let idempotency_key: [u8; 32] =
            suffix.try_into().map_err(|_| ReceiptStoreError::Corrupt)?;
        if durable
            .get(&legacy_evidence_key(tenant.clone(), idempotency_key)?)
            .is_some()
        {
            return Err(ReceiptStoreError::EvidenceVersion);
        }
        let served = serve(
            durable,
            tenant.clone(),
            ReceiptLookupKey::Idempotency(idempotency_key),
        )?;
        if let Some(value) =
            durable.get(&evidence_key(tenant.clone(), served.metadata.activity_id)?)
        {
            if value.bytes().get(..4) != Some(EVIDENCE_MAGIC.as_slice()) {
                return Err(ReceiptStoreError::Corrupt);
            }
            match value.bytes().get(4) {
                Some(&1 | &EVIDENCE_VERSION) => {}
                Some(_) => return Err(ReceiptStoreError::EvidenceVersion),
                None => return Err(ReceiptStoreError::Corrupt),
            }
            if value.bytes().get(4) == Some(&EVIDENCE_VERSION) {
                let evidence = decode_transaction_evidence(value.bytes())?;
                if evidence.canonical_receipt != served.canonical_bytes
                    || evidence.metadata.transaction_hash != served.metadata.activity_id
                {
                    return Err(ReceiptStoreError::Corrupt);
                }
                let decoded = layerx_wire::receipt::decode(&evidence.canonical_receipt)
                    .map_err(|_| ReceiptStoreError::Corrupt)?;
                let protocol = decoded.protocol().ok_or(ReceiptStoreError::Corrupt)?;
                if protocol.activity_id() != served.metadata.activity_id
                    || protocol.global_sequence() != served.metadata.global_sequence
                    || served.metadata.idempotency_key != idempotency_key
                    || served.metadata.result
                        != classify(ResultCode::from_raw(protocol.result_code()))
                {
                    return Err(ReceiptStoreError::Corrupt);
                }
                inventory
                    .with_current_evidence
                    .push(CurrentReceiptEvidenceRecord {
                        idempotency_key,
                        activity_id: served.metadata.activity_id,
                        global_sequence: served.metadata.global_sequence,
                        evidence,
                    });
                continue;
            }
        }
        if let Some(record) = serve_kernel_evidence(durable, tenant.clone(), idempotency_key)? {
            inventory.with_evidence.push(record);
        } else {
            let served = serve(
                durable,
                tenant.clone(),
                ReceiptLookupKey::Idempotency(idempotency_key),
            )?;
            inventory.without_evidence.push(served.metadata);
        }
    }
    inventory
        .with_evidence
        .sort_by(|left, right| left.idempotency_key.cmp(&right.idempotency_key));
    inventory
        .with_current_evidence
        .sort_by(|left, right| left.idempotency_key.cmp(&right.idempotency_key));
    inventory
        .without_evidence
        .sort_by(|left, right| left.idempotency_key.cmp(&right.idempotency_key));
    Ok(inventory)
}

fn legacy_evidence_key(tenant: TenantId, key: [u8; 32]) -> Result<TenantKey, StoreError> {
    TenantKey::new(
        tenant,
        ObjectKind::Configuration,
        [LEGACY_EVIDENCE_PREFIX, key.as_slice()].concat(),
    )
}

impl TransactionReceiptEvidence {
    /// Converts only an authenticated node observation, preserving its exact signed wire bytes.
    /// The fixed signature slot carries the original Ed25519 signature followed by its algorithm tag.
    ///
    /// # Errors
    /// Refuses an observation below Receipted or without its authenticated signature.
    pub fn from_authenticated(
        receipt: &layerx_client::rpc::receipt::AuthenticatedReceipt,
    ) -> Result<Self, ReceiptStoreError> {
        if receipt.rung() < Rung::Receipted {
            return Err(ReceiptStoreError::BelowReceipted);
        }
        let signature = receipt
            .receipt()
            .protocol()
            .and_then(|protocol| protocol.sequencer_signature())
            .ok_or(ReceiptStoreError::Corrupt)?;
        let mut receipt_signature = [0; 65];
        receipt_signature[..64].copy_from_slice(&signature);
        receipt_signature[64] = NODE_ED25519_SIGNATURE_TAG;
        let execution = receipt.execution();
        let mut logs_bloom = [0; 256];
        for log in &execution.logs {
            for bytes in std::iter::once(log.address.as_slice())
                .chain(log.topics.iter().map(|topic| topic.as_slice()))
            {
                // transaction_hash is the crate's public Keccak-256 byte hash.
                let hash = layerx_crypto::evm_transaction::transaction_hash(bytes);
                for pair in hash[..6].chunks_exact(2) {
                    let bit = usize::from(u16::from_be_bytes([pair[0], pair[1]]) & 2047);
                    logs_bloom[255 - bit / 8] |= 1 << (bit % 8);
                }
            }
        }
        Ok(Self {
            metadata: TransactionReceiptMetadata {
                transaction_hash: receipt.activity_id(),
                block_height: execution.height,
                tx_index: execution.index,
                status: execution.success,
                gas_used: execution.gas_used,
                logs_bloom,
                receipt_signature,
                rung: receipt.rung(),
            },
            canonical_receipt: receipt.canonical_bytes().to_vec(),
        })
    }
}

/// Migrates retained legacy evidence only after the same receipt has been authenticated by the node client.
/// Legacy recovery bytes are preserved under their declared recovery key in the same transaction.
///
/// # Errors
/// Refuses absent legacy evidence, mismatched receipt identities or bytes, and conflicting indexes.
pub fn migrate_authenticated_evidence(
    durable: &mut Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
    receipt: &layerx_client::rpc::receipt::AuthenticatedReceipt,
) -> Result<ServedReceipt, ReceiptStoreError> {
    serve_kernel_evidence(durable, tenant.clone(), idempotency_key)?
        .ok_or(ReceiptStoreError::Missing)?;
    persist_authenticated_receipt(
        durable,
        tenant,
        idempotency_key,
        receipt.canonical_bytes(),
        receipt,
        true,
    )
}

pub(crate) fn persist_authenticated_receipt(
    durable: &mut Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
    receipt_bytes: &[u8],
    receipt: &layerx_client::rpc::receipt::AuthenticatedReceipt,
    migrate_legacy: bool,
) -> Result<ServedReceipt, ReceiptStoreError> {
    if idempotency_key != receipt.activity_id() || receipt_bytes != receipt.canonical_bytes() {
        return Err(ReceiptStoreError::Corrupt);
    }
    let mut evidence = AuthenticatedTransactionReceipt::from_authenticated(receipt)?.evidence;
    let mut metadata = ReceiptMetadata {
        activity_id: receipt.activity_id(),
        idempotency_key,
        global_sequence: receipt.global_sequence(),
        verification_level: if receipt.rung() == Rung::Committed {
            VerificationLevel::CHECKPOINT_FINALISED
        } else {
            VerificationLevel::SEQUENCER_SIGNED
        },
        result: classify(receipt.result_code()),
    };
    match serve(
        durable,
        tenant.clone(),
        ReceiptLookupKey::Idempotency(idempotency_key),
    ) {
        Ok(existing) => {
            if existing.canonical_bytes != receipt_bytes
                || existing.metadata.activity_id != metadata.activity_id
                || existing.metadata.idempotency_key != idempotency_key
                || existing.metadata.global_sequence != metadata.global_sequence
                || existing.metadata.result != metadata.result
            {
                return Err(ReceiptStoreError::Corrupt);
            }
            metadata.verification_level = metadata
                .verification_level
                .max(existing.metadata.verification_level);
        }
        Err(ReceiptStoreError::Missing) => {}
        Err(error) => return Err(error),
    }
    let evidence_key = evidence_key(tenant.clone(), receipt.activity_id())?;
    let previous = durable
        .get(&evidence_key)
        .map(|value| value.bytes().to_vec());
    let mut companions = Vec::new();
    if let Some(bytes) = &previous {
        if bytes.get(..4) == Some(EVIDENCE_MAGIC.as_slice())
            && bytes.get(4) == Some(&1)
            && migrate_legacy
        {
            let legacy = serve_kernel_evidence(durable, tenant.clone(), idempotency_key)?
                .ok_or(ReceiptStoreError::Missing)?;
            if encode_evidence(&legacy)? != *bytes {
                return Err(ReceiptStoreError::Corrupt);
            }
            let archived_key = legacy_evidence_key(tenant.clone(), idempotency_key)?;
            let archived = durable
                .get(&archived_key)
                .map(|value| value.bytes().to_vec());
            if archived.as_ref().is_some_and(|existing| existing != bytes) {
                return Err(ReceiptStoreError::Corrupt);
            }
            companions.push((archived_key, archived, bytes.clone()));
        } else {
            let existing = serve_evidence(durable, tenant.clone(), receipt.activity_id())?
                .ok_or(ReceiptStoreError::Corrupt)?;
            let mut comparable = existing.clone();
            comparable.metadata.rung = evidence.metadata.rung;
            if comparable != evidence {
                return Err(ReceiptStoreError::Corrupt);
            }
            evidence.metadata.rung = evidence.metadata.rung.max(existing.metadata.rung);
        }
    }
    let digest: [u8; 32] = Sha256::digest(receipt_bytes).into();
    let metadata_key = metadata_key(tenant.clone(), digest)?;
    let previous_metadata = durable
        .get(&metadata_key)
        .map(|value| value.bytes().to_vec());
    if previous_metadata.is_some()
        && serve(
            durable,
            tenant.clone(),
            ReceiptLookupKey::Idempotency(idempotency_key),
        )
        .is_err()
    {
        return Err(ReceiptStoreError::Corrupt);
    }
    companions.push((metadata_key, previous_metadata, encode_metadata(metadata)));
    companions.push((
        evidence_key,
        previous,
        encode_transaction_evidence(&evidence)?,
    ));
    let indexes = [
        lookup_key(
            tenant.clone(),
            ReceiptLookupKey::Activity(metadata.activity_id),
        )?,
        lookup_key(
            tenant.clone(),
            ReceiptLookupKey::Idempotency(idempotency_key),
        )?,
        lookup_key(
            tenant.clone(),
            ReceiptLookupKey::GlobalSequence(metadata.global_sequence),
        )?,
        transaction_key(tenant, metadata.activity_id)?,
    ];
    durable.record_authenticated_receipt(&indexes, receipt_bytes, companions)?;
    Ok(ServedReceipt {
        canonical_bytes: receipt_bytes.to_vec(),
        metadata,
    })
}

fn evidence_key(tenant: TenantId, idempotency_key: [u8; 32]) -> Result<TenantKey, StoreError> {
    let mut object_id = EVIDENCE_PREFIX.to_vec();
    object_id.extend_from_slice(&idempotency_key);
    TenantKey::new(tenant, ObjectKind::Configuration, object_id)
}

fn encode_evidence(record: &ReceiptEvidenceRecord) -> Result<Vec<u8>, ReceiptStoreError> {
    let receipt = record.evidence.canonical_receipt();
    let proof = encode_proof(record.evidence.proof());
    let header = record.evidence.canonical_header();
    let mut bytes = Vec::with_capacity(
        4 + 1 + 32 + 32 + 8 + 12 + receipt.len() + proof.len() + header.len() + 64,
    );
    bytes.extend_from_slice(EVIDENCE_MAGIC);
    bytes.push(1);
    bytes.extend_from_slice(&record.idempotency_key);
    bytes.extend_from_slice(&record.activity_id);
    bytes.extend_from_slice(&record.global_sequence.to_be_bytes());
    for section in [receipt, proof.as_slice(), header] {
        let length = u32::try_from(section.len()).map_err(|_| ReceiptStoreError::Corrupt)?;
        bytes.extend_from_slice(&length.to_be_bytes());
        bytes.extend_from_slice(section);
    }
    bytes.extend_from_slice(&record.evidence.header_signature());
    Ok(bytes)
}

fn decode_evidence(bytes: &[u8]) -> Result<ReceiptEvidenceRecord, ReceiptStoreError> {
    let corrupt = ReceiptStoreError::Corrupt;
    if bytes.len() < 4 + 1 + 32 + 32 + 8 || &bytes[..4] != EVIDENCE_MAGIC || bytes[4] != 1 {
        return Err(corrupt);
    }
    let mut offset = 5;
    let mut idempotency_key = [0_u8; 32];
    idempotency_key.copy_from_slice(&bytes[offset..offset + 32]);
    offset += 32;
    let mut activity_id = [0_u8; 32];
    activity_id.copy_from_slice(&bytes[offset..offset + 32]);
    offset += 32;
    let mut sequence = [0_u8; 8];
    sequence.copy_from_slice(&bytes[offset..offset + 8]);
    offset += 8;
    let section = |offset: &mut usize| -> Result<Vec<u8>, ReceiptStoreError> {
        let end = offset.checked_add(4).ok_or(ReceiptStoreError::Corrupt)?;
        let length_bytes = bytes.get(*offset..end).ok_or(ReceiptStoreError::Corrupt)?;
        let mut length = [0_u8; 4];
        length.copy_from_slice(length_bytes);
        let length =
            usize::try_from(u32::from_be_bytes(length)).map_err(|_| ReceiptStoreError::Corrupt)?;
        let body_end = end.checked_add(length).ok_or(ReceiptStoreError::Corrupt)?;
        let body = bytes.get(end..body_end).ok_or(ReceiptStoreError::Corrupt)?;
        *offset = body_end;
        Ok(body.to_vec())
    };
    let receipt = section(&mut offset)?;
    let proof = section(&mut offset)?;
    let header = section(&mut offset)?;
    if bytes.len() != offset + 64 {
        return Err(ReceiptStoreError::Corrupt);
    }
    let mut signature = [0_u8; 64];
    signature.copy_from_slice(&bytes[offset..]);
    let proof = decode_proof(&proof).map_err(|_| ReceiptStoreError::Corrupt)?;
    Ok(ReceiptEvidenceRecord {
        idempotency_key,
        activity_id,
        global_sequence: u64::from_be_bytes(sequence),
        evidence: RawReceiptEvidence::new(receipt, proof, header, signature),
    })
}

fn lookup_key(tenant: TenantId, lookup: ReceiptLookupKey) -> Result<TenantKey, StoreError> {
    let mut object_id = match lookup {
        ReceiptLookupKey::Activity(_) => b"receipt:activity:".to_vec(),
        ReceiptLookupKey::Idempotency(_) => b"receipt:idempotency:".to_vec(),
        ReceiptLookupKey::GlobalSequence(_) => b"receipt:sequence:".to_vec(),
    };
    match lookup {
        ReceiptLookupKey::Activity(value) | ReceiptLookupKey::Idempotency(value) => {
            object_id.extend_from_slice(&value);
        }
        ReceiptLookupKey::GlobalSequence(value) => {
            object_id.extend_from_slice(&value.to_be_bytes());
        }
    }
    TenantKey::new(tenant, ObjectKind::Receipt, object_id)
}

fn metadata_key(tenant: TenantId, digest: [u8; 32]) -> Result<TenantKey, StoreError> {
    let mut object_id = b"receipt-metadata:".to_vec();
    object_id.extend_from_slice(&digest);
    TenantKey::new(tenant, ObjectKind::Configuration, object_id)
}

fn encode_metadata(metadata: ReceiptMetadata) -> Vec<u8> {
    let mut bytes = Vec::with_capacity(81);
    bytes.extend_from_slice(METADATA_MAGIC);
    bytes.extend_from_slice(&metadata.activity_id);
    bytes.extend_from_slice(&metadata.idempotency_key);
    bytes.extend_from_slice(&metadata.global_sequence.to_be_bytes());
    bytes.push(metadata.verification_level.wire_rank());
    bytes.extend_from_slice(&metadata.result.code.raw().to_be_bytes());
    bytes
}

fn decode_metadata(bytes: &[u8]) -> Result<ReceiptMetadata, ReceiptStoreError> {
    if bytes.len() != 81 || &bytes[..4] != METADATA_MAGIC {
        return Err(ReceiptStoreError::Corrupt);
    }
    let mut activity_id = [0_u8; 32];
    activity_id.copy_from_slice(&bytes[4..36]);
    let mut idempotency_key = [0_u8; 32];
    idempotency_key.copy_from_slice(&bytes[36..68]);
    let mut sequence = [0_u8; 8];
    sequence.copy_from_slice(&bytes[68..76]);
    let verification_level = match bytes[76] {
        1 => VerificationLevel::SEQUENCER_SIGNED,
        2 => VerificationLevel::BATCH_INCLUDED,
        3 => VerificationLevel::STATE_PROVEN,
        4 => VerificationLevel::CHECKPOINT_FINALISED,
        5 => VerificationLevel::SETTLEMENT_ANCHORED,
        _ => return Err(ReceiptStoreError::Corrupt),
    };
    let mut result = [0_u8; 4];
    result.copy_from_slice(&bytes[77..81]);
    Ok(ReceiptMetadata {
        activity_id,
        idempotency_key,
        global_sequence: u64::from_be_bytes(sequence),
        verification_level,
        result: classify(ResultCode::from_raw(i32::from_be_bytes(result))),
    })
}

fn check_evidence_version(bytes: &[u8]) -> Result<(), ReceiptStoreError> {
    if bytes.get(..4) != Some(EVIDENCE_MAGIC.as_slice()) {
        return Err(ReceiptStoreError::Corrupt);
    }
    match bytes.get(4) {
        Some(&EVIDENCE_VERSION) => Ok(()),
        Some(_) => Err(ReceiptStoreError::EvidenceVersion),
        None => Err(ReceiptStoreError::Corrupt),
    }
}

fn transaction_key(tenant: TenantId, hash: [u8; 32]) -> Result<TenantKey, StoreError> {
    let mut object_id = b"receipt:transaction:".to_vec();
    object_id.extend_from_slice(&hash);
    TenantKey::new(tenant, ObjectKind::Receipt, object_id)
}

impl ReceiptIngress for TransactionReceiptEvidence {
    type Stored = Self;

    fn store_if_absent(
        &self,
        _durable: &mut Store,
        _tenant: TenantId,
        _transaction_hash: [u8; 32],
        _receipt_bytes: &[u8],
    ) -> Result<Self, ReceiptStoreError> {
        Err(if self.metadata.rung < Rung::Receipted {
            ReceiptStoreError::BelowReceipted
        } else {
            ReceiptStoreError::UnauthenticatedIngress
        })
    }
}

impl ReceiptIngress for AuthenticatedTransactionReceipt {
    type Stored = TransactionReceiptEvidence;

    fn store_if_absent(
        &self,
        durable: &mut Store,
        tenant: TenantId,
        transaction_hash: [u8; 32],
        receipt_bytes: &[u8],
    ) -> Result<Self::Stored, ReceiptStoreError> {
        persist_transaction_evidence(
            &self.evidence,
            durable,
            tenant,
            transaction_hash,
            receipt_bytes,
        )
    }
}

fn persist_transaction_evidence(
    evidence: &TransactionReceiptEvidence,
    durable: &mut Store,
    tenant: TenantId,
    transaction_hash: [u8; 32],
    receipt_bytes: &[u8],
) -> Result<TransactionReceiptEvidence, ReceiptStoreError> {
    if evidence.metadata.rung < Rung::Receipted {
        return Err(ReceiptStoreError::BelowReceipted);
    }
    if evidence.metadata.transaction_hash != transaction_hash
        || evidence.canonical_receipt != receipt_bytes
        || receipt_bytes.is_empty()
    {
        return Err(ReceiptStoreError::Corrupt);
    }
    let key = evidence_key(tenant.clone(), transaction_hash)?;
    if let Some(existing) = serve_evidence(durable, tenant.clone(), transaction_hash)? {
        let mut comparable = existing.clone();
        comparable.metadata.rung = evidence.metadata.rung;
        if comparable != *evidence {
            return Err(ReceiptStoreError::Store(StoreError::Corrupt(
                "conflicting receipt metadata",
            )));
        }
        if evidence.metadata.rung > existing.metadata.rung {
            durable.put_local(key, encode_transaction_evidence(evidence)?)?;
            return Ok(evidence.clone());
        }
        return Ok(existing);
    }
    durable.record_verified_receipt(
        &[transaction_key(tenant, transaction_hash)?],
        receipt_bytes,
        key,
        encode_transaction_evidence(evidence)?,
    )?;
    Ok(evidence.clone())
}

fn encode_transaction_evidence(
    record: &TransactionReceiptEvidence,
) -> Result<Vec<u8>, ReceiptStoreError> {
    let metadata = &record.metadata;
    let length =
        u32::try_from(record.canonical_receipt.len()).map_err(|_| ReceiptStoreError::Corrupt)?;
    let mut bytes = Vec::with_capacity(393 + record.canonical_receipt.len());
    bytes.extend_from_slice(EVIDENCE_MAGIC);
    bytes.push(EVIDENCE_VERSION);
    bytes.extend_from_slice(METADATA_MAGIC);
    bytes.push(EVIDENCE_VERSION);
    bytes.extend_from_slice(&metadata.transaction_hash);
    bytes.extend_from_slice(&metadata.block_height.to_be_bytes());
    bytes.extend_from_slice(&metadata.tx_index.to_be_bytes());
    bytes.push(u8::from(metadata.status));
    bytes.extend_from_slice(&metadata.gas_used.to_be_bytes());
    bytes.extend_from_slice(&metadata.logs_bloom);
    bytes.extend_from_slice(&metadata.receipt_signature);
    bytes.push(metadata.rung.wire_rank());
    bytes.extend_from_slice(&length.to_be_bytes());
    bytes.extend_from_slice(&record.canonical_receipt);
    Ok(bytes)
}

fn decode_transaction_evidence(
    bytes: &[u8],
) -> Result<TransactionReceiptEvidence, ReceiptStoreError> {
    check_evidence_version(bytes)?;
    let mut reader = EvidenceReader { bytes, offset: 5 };
    if reader.array::<4>()? != *METADATA_MAGIC || reader.array::<1>()?[0] != EVIDENCE_VERSION {
        return Err(ReceiptStoreError::Corrupt);
    }
    let transaction_hash = reader.array()?;
    let block_height = u64::from_be_bytes(reader.array()?);
    let tx_index = u64::from_be_bytes(reader.array()?);
    let status = match reader.array::<1>()?[0] {
        0 => false,
        1 => true,
        _ => return Err(ReceiptStoreError::Corrupt),
    };
    let gas_used = u64::from_be_bytes(reader.array()?);
    let logs_bloom = reader.array()?;
    let receipt_signature = reader.array()?;
    let rung = match reader.array::<1>()?[0] {
        0 => return Err(ReceiptStoreError::BelowReceipted),
        1 => Rung::Receipted,
        2 => Rung::Committed,
        _ => return Err(ReceiptStoreError::Corrupt),
    };
    let length = usize::try_from(u32::from_be_bytes(reader.array()?))
        .map_err(|_| ReceiptStoreError::Corrupt)?;
    let canonical_receipt = bytes
        .get(reader.offset..)
        .ok_or(ReceiptStoreError::Corrupt)?;
    if length == 0 || canonical_receipt.len() != length {
        return Err(ReceiptStoreError::Corrupt);
    }
    Ok(TransactionReceiptEvidence {
        metadata: TransactionReceiptMetadata {
            transaction_hash,
            block_height,
            tx_index,
            status,
            gas_used,
            logs_bloom,
            receipt_signature,
            rung,
        },
        canonical_receipt: canonical_receipt.to_vec(),
    })
}

struct EvidenceReader<'a> {
    bytes: &'a [u8],
    offset: usize,
}

impl EvidenceReader<'_> {
    fn array<const N: usize>(&mut self) -> Result<[u8; N], ReceiptStoreError> {
        let end = self
            .offset
            .checked_add(N)
            .ok_or(ReceiptStoreError::Corrupt)?;
        let value = self
            .bytes
            .get(self.offset..end)
            .ok_or(ReceiptStoreError::Corrupt)?;
        self.offset = end;
        value.try_into().map_err(|_| ReceiptStoreError::Corrupt)
    }
}

#[derive(serde::Serialize, serde::Deserialize)]
struct NativeCommitRecord {
    version: u8,
    height: u64,
    index: u64,
    header: Vec<u8>,
    commit: Vec<u8>,
    validators: Vec<u8>,
    inclusion: Vec<Vec<u8>>,
    key_id: [u8; 32],
    public_key: [u8; 32],
    first_height: u64,
    last_height: u64,
}
fn native_commit_key(tenant: TenantId, hash: [u8; 32]) -> Result<TenantKey, StoreError> {
    TenantKey::new(
        tenant,
        ObjectKind::Configuration,
        [b"receipt:committed-native:".as_slice(), &hash].concat(),
    )
}
fn verify_result_identity(
    result: &[u8],
    hash: [u8; 32],
    success: bool,
) -> Result<(), ReceiptStoreError> {
    use crate::read::abci_result::{fields, ProtoValue};
    if crate::read::abci_result::transaction_hash(result).map_err(|_| ReceiptStoreError::Corrupt)?
        != Some(hash)
    {
        return Err(ReceiptStoreError::Corrupt);
    }
    let response =
        crate::read::abci_result::evm_response(result).map_err(|_| ReceiptStoreError::Corrupt)?;
    let mut vm_error = &[][..];
    for (number, value) in fields(response).map_err(|_| ReceiptStoreError::Corrupt)? {
        if let (2, ProtoValue::Bytes(error)) = (number, value) {
            vm_error = error;
        }
    }
    if vm_error.is_empty() != success {
        return Err(ReceiptStoreError::Corrupt);
    }
    Ok(())
}

/// The receipt producer promotes only locally verified result/commit evidence.
/// Action replay identities are assigned by lifecycle callers, independently of the transaction hash.
pub fn persist_committed_protocol_receipt(
    durable: &mut Store,
    tenant: TenantId,
    canonical: &[u8],
    keys: &[layerx_client::sequencer::ReceiptKeyEntry],
    height: u64,
    index: u64,
    inclusion: &crate::read::AbciResultProof,
    checkpoint: &crate::read::ServedCheckpoint,
    chain_id: &str,
    validators: &[crate::read::CommitValidator],
) -> Result<TransactionReceiptEvidence, ReceiptStoreError> {
    let pins: Vec<_> = keys
        .iter()
        .map(|key| {
            layerx_proof::inclusion::SequencerAuthorization::new(
                key.key_id,
                key.public_key,
                key.first_height,
                key.last_height,
            )
        })
        .collect();
    let decoded = layerx_proof::receipt::verify_sequencer_signature(canonical, pins.as_slice())
        .map_err(|_| ReceiptStoreError::Corrupt)?;
    let protocol = decoded.protocol().ok_or(ReceiptStoreError::Corrupt)?;
    let mut expected_height = [0; 32];
    expected_height[24..].copy_from_slice(&height.to_be_bytes());
    if protocol.batch_id() != expected_height {
        return Err(ReceiptStoreError::Corrupt);
    }
    inclusion
        .verify_committed(height, index, checkpoint, chain_id, validators)
        .map_err(|_| ReceiptStoreError::Corrupt)?;
    verify_result_identity(
        &inclusion.result,
        protocol.activity_id(),
        protocol.result_code() == 0,
    )?;
    let mut matching = keys.iter().filter(|key| {
        key.key_id == protocol.context_hash()
            && key.first_height <= height
            && height <= key.last_height
    });
    let key = matching.next().ok_or(ReceiptStoreError::Corrupt)?;
    if matching.next().is_some() {
        return Err(ReceiptStoreError::Corrupt);
    }
    let mut signature = [0; 65];
    signature[..64].copy_from_slice(
        &protocol
            .sequencer_signature()
            .ok_or(ReceiptStoreError::Corrupt)?,
    );
    signature[64] = NODE_ED25519_SIGNATURE_TAG;
    let mut record = TransactionReceiptEvidence {
        metadata: TransactionReceiptMetadata {
            transaction_hash: protocol.activity_id(),
            block_height: height,
            tx_index: index,
            status: protocol.result_code() == 0,
            gas_used: 0,
            logs_bloom: [0; 256],
            receipt_signature: signature,
            rung: Rung::Committed,
        },
        canonical_receipt: canonical.to_vec(),
    };
    if let Some(previous) = serve_evidence(durable, tenant.clone(), protocol.activity_id())? {
        record.metadata.gas_used = previous.metadata.gas_used;
        record.metadata.logs_bloom = previous.metadata.logs_bloom;
    }
    let proof = NativeCommitRecord {
        version: 1,
        height,
        index,
        header: crate::export::encode_header(&checkpoint.header),
        commit: crate::export::encode_commit(&checkpoint.commit, validators),
        validators: crate::export::encode_validators(validators),
        inclusion: inclusion.encode().map_err(|_| ReceiptStoreError::Corrupt)?,
        key_id: key.key_id,
        public_key: key.public_key,
        first_height: key.first_height,
        last_height: key.last_height,
    };
    let bytes = serde_json::to_vec(&proof).map_err(|_| ReceiptStoreError::Corrupt)?;
    durable.put_local(
        native_commit_key(tenant.clone(), protocol.activity_id())?,
        bytes,
    )?;
    persist_transaction_evidence(&record, durable, tenant, protocol.activity_id(), canonical)
}

pub fn persist_committed_native_receipt(
    durable: &mut Store,
    tenant: TenantId,
    receipt: &layerx_client::rpc::receipt::AuthenticatedReceipt,
    inclusion: &crate::read::AbciResultProof,
    checkpoint: &crate::read::ServedCheckpoint,
    chain_id: &str,
    validators: &[crate::read::CommitValidator],
    keys: &[layerx_client::sequencer::ReceiptKeyEntry],
) -> Result<TransactionReceiptEvidence, ReceiptStoreError> {
    crate::read::abci_result::verify_native_receipt(&inclusion.result, receipt)
        .map_err(|_| ReceiptStoreError::Corrupt)?;
    persist_committed_protocol_receipt(
        durable,
        tenant,
        receipt.canonical_bytes(),
        keys,
        receipt.height(),
        receipt.execution().index,
        inclusion,
        checkpoint,
        chain_id,
        validators,
    )
}

/// Re-verifies retained commit and inclusion bytes on the restored store.
pub fn serve_committed_evidence(
    durable: &Store,
    tenant: TenantId,
    hash: [u8; 32],
) -> Result<TransactionReceiptEvidence, ReceiptStoreError> {
    let record =
        serve_evidence(durable, tenant.clone(), hash)?.ok_or(ReceiptStoreError::Missing)?;
    if record.metadata.rung != Rung::Committed {
        return Err(ReceiptStoreError::BelowReceipted);
    }
    let bytes = durable
        .get(&native_commit_key(tenant, hash)?)
        .ok_or(ReceiptStoreError::Missing)?
        .bytes();
    let proof: NativeCommitRecord =
        serde_json::from_slice(bytes).map_err(|_| ReceiptStoreError::Corrupt)?;
    let header =
        crate::export::decode_header(&proof.header).map_err(|_| ReceiptStoreError::Corrupt)?;
    let validators = crate::export::decode_validators(&proof.validators)
        .map_err(|_| ReceiptStoreError::Corrupt)?;
    let commit = crate::export::decode_commit(&proof.commit, &validators)
        .map_err(|_| ReceiptStoreError::Corrupt)?;
    let checkpoint = crate::read::checkpoint(
        header.height,
        &header.chain_id,
        &header,
        &commit,
        &validators,
    )
    .map_err(|_| ReceiptStoreError::Corrupt)?;
    let inclusion = crate::read::AbciResultProof::decode(&proof.inclusion)
        .map_err(|_| ReceiptStoreError::Corrupt)?;
    inclusion
        .verify_committed(
            proof.height,
            proof.index,
            &checkpoint,
            &header.chain_id,
            &validators,
        )
        .map_err(|_| ReceiptStoreError::Corrupt)?;
    if proof.version != 1
        || proof.height != record.metadata.block_height
        || proof.index != record.metadata.tx_index
    {
        return Err(ReceiptStoreError::Corrupt);
    }
    let pins = [layerx_proof::inclusion::SequencerAuthorization::new(
        proof.key_id,
        proof.public_key,
        proof.first_height,
        proof.last_height,
    )];
    layerx_proof::receipt::verify_sequencer_signature(&record.canonical_receipt, pins.as_slice())
        .map_err(|_| ReceiptStoreError::Corrupt)?;
    verify_result_identity(&inclusion.result, hash, record.metadata.status)?;
    Ok(record)
}
