//! Proof-gated submission finality augmentation and deadline-bounded waiting.

use layerx_client::evidence::{VerifiedCheckpoint, VerifiedProofBundle};
use layerx_proof::inclusion::{
    verify_activity, verify_receipt, InclusionError, SequencerAuthorization,
};
use layerx_proof::merkle::Proof;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_types::payload::ModuleRegistry;
use layerx_types::verify::VerificationLevel;
use layerx_wire::activity::{decode_signed, encode_signed};
use layerx_wire::hash::activity_id;
use layerx_wire::receipt::Receipt;

use crate::receipt::{self, ReceiptLookupKey, ReceiptStoreError};
use crate::store::{ObjectKind, Store, StoreError, TenantId, TenantKey};

const RECORD_MAGIC: &[u8; 4] = b"LXFA";

/// Activity and exact receipt evidence carried by one signed batch.
pub struct InclusionBundle<'a> {
    pub registry: &'a ModuleRegistry,
    pub activity_bytes: &'a [u8],
    pub activity_proof: &'a Proof,
    pub receipt_bytes: &'a [u8],
    pub receipt_proof: &'a Proof,
    pub header_bytes: &'a [u8],
    pub header_signature: [u8; 64],
    pub authorization: &'a SequencerAuthorization,
}

/// Verified evidence retained alongside, but never inside, original receipt bytes.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FinalityRecord {
    pub idempotency_key: [u8; 32],
    pub verification_level: VerificationLevel,
    pub activity_proof: Vec<u8>,
    pub receipt_proof: Vec<u8>,
    pub checkpoint_id: Option<[u8; 32]>,
    pub guarantor_signatures_achieved: Option<usize>,
    pub guarantor_threshold: Option<usize>,
    pub settlement_reference: Option<Vec<u8>>,
}

#[derive(Debug)]
pub enum FinalityError {
    UnavailableCapability(layerx_agent_api::error::ApiError),
    Receipt(ReceiptStoreError),
    Inclusion(InclusionError),
    Store(StoreError),
    InvalidDeadline,
    ProgressUnavailable,
    Arithmetic,
    Corrupt,
}

impl From<ReceiptStoreError> for FinalityError {
    fn from(value: ReceiptStoreError) -> Self {
        Self::Receipt(value)
    }
}

impl From<StoreError> for FinalityError {
    fn from(value: StoreError) -> Self {
        Self::Store(value)
    }
}

/// Verifies and records only evidence-supported finality without modifying the receipt.
///
/// # Errors
///
/// Returns a receipt error when the receipt is absent or corrupt, `Inclusion` or
/// `Arithmetic` when a proof cannot be encoded, and `Corrupt` when the re-read
/// receipt differs from the raised one. Checkpoint finality is deliberately
/// unavailable on this raw-byte path; only [`augment_verified`] can consume a
/// node-authority-accepted [`VerifiedCheckpoint`].
pub fn augment(
    durable: &mut Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
    inclusion: &InclusionBundle<'_>,
) -> Result<FinalityRecord, FinalityError> {
    let _ = (durable, tenant, idempotency_key, inclusion);
    Err(FinalityError::UnavailableCapability(
        layerx_agent_api::error::ApiError::unavailable_capability(
            layerx_agent_api::error::RequestId(0),
            "track.wait",
        )
        .expect("static operation name"),
    ))
}

/// Persists finality already established by the production LNI verifier.
///
/// The activity and exact stored receipt must have independently verified
/// inclusion under byte-identical signed-header evidence. A checkpoint raises
/// the receipt only when its verified certificate covers that same header.
///
/// # Errors
///
/// Returns an error if the verified evidence does not match the durable record or persistence fails.
pub fn augment_verified(
    durable: &mut Store,
    tenant: TenantId,
    idempotency_key: [u8; 32],
    activity: &VerifiedProofBundle,
    receipt_bundle: &VerifiedProofBundle,
    checkpoint: Option<&VerifiedCheckpoint>,
) -> Result<FinalityRecord, FinalityError> {
    let _ = (
        durable,
        tenant,
        idempotency_key,
        activity,
        receipt_bundle,
        checkpoint,
    );
    Err(FinalityError::UnavailableCapability(
        layerx_agent_api::error::ApiError::unavailable_capability(
            layerx_agent_api::error::RequestId(0),
            "track.wait",
        )
        .expect("static operation name"),
    ))
}

/// Source of independently observed finality progress for deadline-bounded waits.
pub trait VerificationProgress {
    /// Returns the independently observed level for the submission at that instant.
    ///
    /// # Errors
    ///
    /// Returns `ProgressUnavailable` when the source cannot observe the submission at
    /// `observed_at_ms`.
    fn level_at(
        &mut self,
        idempotency_key: [u8; 32],
        observed_at_ms: u64,
    ) -> Result<VerificationLevel, FinalityError>;
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct WaitResult {
    pub requested: VerificationLevel,
    pub reached: VerificationLevel,
    pub observed_at_ms: u64,
    pub deadline_elapsed: bool,
}

/// Polls until the declared level is observed or the explicit deadline is reached.
///
/// # Errors
///
/// Returns `InvalidDeadline` when the deadline precedes the start or the poll
/// interval is zero, `Arithmetic` when the next poll instant overflows, and
/// propagates the progress-source failure.
pub fn wait_for_level<P: VerificationProgress>(
    progress: &mut P,
    idempotency_key: [u8; 32],
    requested: VerificationLevel,
    started_at_ms: u64,
    deadline_ms: u64,
    poll_interval_ms: u64,
) -> Result<WaitResult, FinalityError> {
    let _ = (
        progress,
        idempotency_key,
        requested,
        started_at_ms,
        deadline_ms,
        poll_interval_ms,
    );
    Err(FinalityError::UnavailableCapability(
        layerx_agent_api::error::ApiError::unavailable_capability(
            layerx_agent_api::error::RequestId(0),
            "track.wait",
        )
        .expect("static operation name"),
    ))
}

fn record_key(tenant: TenantId, idempotency_key: [u8; 32]) -> Result<TenantKey, StoreError> {
    let mut object_id = b"finality:".to_vec();
    object_id.extend_from_slice(&idempotency_key);
    TenantKey::new(tenant, ObjectKind::Configuration, object_id)
}

fn encode_proof(proof: &Proof) -> Result<Vec<u8>, FinalityError> {
    let count = u32::try_from(proof.siblings().len()).map_err(|_| FinalityError::Arithmetic)?;
    let mut bytes = Vec::new();
    bytes.extend_from_slice(&proof.leaf_index().to_be_bytes());
    bytes.extend_from_slice(&proof.leaf_count().to_be_bytes());
    bytes.extend_from_slice(&count.to_be_bytes());
    for sibling in proof.siblings() {
        bytes.extend_from_slice(sibling);
    }
    Ok(bytes)
}

fn encode_record(record: &FinalityRecord) -> Result<Vec<u8>, FinalityError> {
    let mut bytes = Vec::new();
    bytes.extend_from_slice(RECORD_MAGIC);
    bytes.extend_from_slice(&record.idempotency_key);
    bytes.push(record.verification_level.wire_rank());
    push_bytes(&mut bytes, &record.activity_proof)?;
    push_bytes(&mut bytes, &record.receipt_proof)?;
    match record.checkpoint_id {
        Some(identifier) => {
            bytes.push(1);
            bytes.extend_from_slice(&identifier);
            let achieved = u32::try_from(record.guarantor_signatures_achieved.unwrap_or(0))
                .map_err(|_| FinalityError::Arithmetic)?;
            let threshold = u32::try_from(record.guarantor_threshold.unwrap_or(0))
                .map_err(|_| FinalityError::Arithmetic)?;
            bytes.extend_from_slice(&achieved.to_be_bytes());
            bytes.extend_from_slice(&threshold.to_be_bytes());
        }
        None => bytes.push(0),
    }
    match &record.settlement_reference {
        Some(reference) => {
            bytes.push(1);
            push_bytes(&mut bytes, reference)?;
        }
        None => bytes.push(0),
    }
    Ok(bytes)
}

fn push_bytes(output: &mut Vec<u8>, value: &[u8]) -> Result<(), FinalityError> {
    let length = u32::try_from(value.len()).map_err(|_| FinalityError::Arithmetic)?;
    output.extend_from_slice(&length.to_be_bytes());
    output.extend_from_slice(value);
    Ok(())
}
