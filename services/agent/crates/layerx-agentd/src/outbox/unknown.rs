//! Sequencer receipt resolution for submissions whose transport outcome is unknown.

use layerx_client::rpc::receipt::{
    self, AuthenticatedReceipt, Lookup, LookupContext, ReceiptError,
};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint, SequencerError};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use sha2::{Digest, Sha256};

use crate::store::{ObjectKind, Store, StoreError, TenantId, TenantKey};

use super::{
    encode_record, Outbox, OutboxError, ReceiptEvidence, StateTransition, SubmissionState,
};

const STATE_MAGIC: &[u8; 4] = b"LXU2";
const INITIAL_BACKOFF_MS: u64 = 1_000;
const MAX_BACKOFF_MS: u64 = 60_000;

/// Caller-clocked age and retry state for one unresolved submission.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct UnknownAge {
    pub first_observed_at_ms: u64,
    pub age_ms: u64,
    pub attempt_count: u32,
    pub next_attempt_at_ms: u64,
}

/// A stable failure classification for receipt lookup and exact-byte resend.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum UnknownBoundaryError {
    Unavailable,
    InvalidResponse,
}

/// The authenticated sequencer connection used for unknown-state resolution.
pub trait ReceiptLookup {
    fn sequencer_endpoint(&self) -> &SequencerEndpoint;
}

impl ReceiptLookup for SequencerEndpoint {
    fn sequencer_endpoint(&self) -> &SequencerEndpoint {
        self
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ResolutionObservation {
    Backoff,
    ReceiptMissing,
    LookupUnavailable,
    UnverifiedReceipt,
    ExecutedReceipt,
    FailedReceipt,
    NonceConsumed,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ResendObservation {
    NotWarranted,
    Sent,
    Indeterminate,
}

/// Operator-visible result of one resolution pass.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct UnknownResolution {
    pub state: SubmissionState,
    pub age: UnknownAge,
    pub observation: ResolutionObservation,
    pub resend: ResendObservation,
}

#[derive(Debug)]
pub enum UnknownResolutionError {
    Outbox(OutboxError),
    Store(StoreError),
    NotUnknown,
    TimeRegressed,
    Arithmetic,
    Corrupt,
    StateVersion,
}

impl From<OutboxError> for UnknownResolutionError {
    fn from(value: OutboxError) -> Self {
        Self::Outbox(value)
    }
}

impl From<StoreError> for UnknownResolutionError {
    fn from(value: StoreError) -> Self {
        Self::Store(value)
    }
}

impl UnknownResolutionError {
    #[must_use]
    pub const fn reason(&self) -> &'static str {
        match self {
            Self::StateVersion => "unknown.state_version",
            Self::Corrupt => "unknown.state_corrupt",
            Self::NotUnknown => "unknown.not_unknown",
            Self::TimeRegressed => "unknown.time_regressed",
            Self::Arithmetic => "unknown.arithmetic",
            Self::Outbox(_) => "unknown.outbox",
            Self::Store(_) => "unknown.store",
        }
    }
}

/// Resolves an unknown transaction using published receipt keys and the sender's pending nonce.
///
/// # Errors
/// Refuses legacy or corrupt resolution state, invalid stored transactions, regressed clocks,
/// non-unknown submissions and failed durable writes.
pub fn resolve_unknown<B: ReceiptLookup>(
    outbox: &mut Outbox,
    store: &mut Store,
    submission_id: [u8; 32],
    observed_at_ms: u64,
    receipt_keys: &[ReceiptKeyEntry],
    boundary: &mut B,
) -> Result<UnknownResolution, UnknownResolutionError> {
    let record = outbox
        .records
        .get(&submission_id)
        .cloned()
        .ok_or(OutboxError::NotFound)?;
    if record.status.state != SubmissionState::Unknown {
        return Err(UnknownResolutionError::NotUnknown);
    }

    let (transaction, signature) = decode_signed(&record.signed_canonical_bytes)
        .map_err(|_| UnknownResolutionError::Corrupt)?;
    let sender =
        recover_sender(&transaction, &signature).map_err(|_| UnknownResolutionError::Corrupt)?;
    if transaction_hash(&record.signed_canonical_bytes) != submission_id {
        return Err(UnknownResolutionError::Corrupt);
    }
    let key = state_key(record.tenant.clone(), submission_id)?;
    let mut age = match store.get(&key) {
        Some(value) => decode_state(value.bytes(), submission_id, observed_at_ms)?,
        None => UnknownAge {
            first_observed_at_ms: observed_at_ms,
            age_ms: 0,
            attempt_count: 0,
            next_attempt_at_ms: observed_at_ms,
        },
    };
    if observed_at_ms < age.first_observed_at_ms {
        return Err(UnknownResolutionError::TimeRegressed);
    }
    age.age_ms = observed_at_ms - age.first_observed_at_ms;
    if observed_at_ms < age.next_attempt_at_ms {
        return Ok(UnknownResolution {
            state: SubmissionState::Unknown,
            age,
            observation: ResolutionObservation::Backoff,
            resend: ResendObservation::NotWarranted,
        });
    }

    age.attempt_count = age
        .attempt_count
        .checked_add(1)
        .ok_or(UnknownResolutionError::Arithmetic)?;
    age.next_attempt_at_ms = observed_at_ms
        .checked_add(backoff_ms(submission_id, age.attempt_count))
        .ok_or(UnknownResolutionError::Arithmetic)?;
    store.put_local(key.clone(), encode_state(submission_id, age))?;

    let endpoint = boundary.sequencer_endpoint();
    let lookup =
        receipt::lookup_authenticated(endpoint, submission_id, LookupContext { receipt_keys });
    let observation = match lookup {
        Ok(Lookup::Verified(receipt)) => {
            if receipt.execution().nonce != transaction.nonce || receipt.execution().from != sender
            {
                ResolutionObservation::UnverifiedReceipt
            } else {
                let state = record_authenticated_receipt(outbox, store, submission_id, &receipt)?;
                store.remove_local(&key)?;
                return Ok(UnknownResolution {
                    state,
                    age,
                    observation: if state == SubmissionState::Executed {
                        ResolutionObservation::ExecutedReceipt
                    } else {
                        ResolutionObservation::FailedReceipt
                    },
                    resend: ResendObservation::NotWarranted,
                });
            }
        }
        Ok(Lookup::Submitted { tx_hash, nonce })
            if tx_hash != submission_id || nonce != transaction.nonce =>
        {
            ResolutionObservation::UnverifiedReceipt
        }
        Ok(Lookup::Absent | Lookup::Submitted { .. }) => ResolutionObservation::ReceiptMissing,
        Err(ReceiptError::Sequencer(
            SequencerError::Unavailable | SequencerError::Tls | SequencerError::HttpStatus(_),
        )) => ResolutionObservation::LookupUnavailable,
        Err(_) => ResolutionObservation::UnverifiedReceipt,
    };
    if observation != ResolutionObservation::ReceiptMissing {
        return Ok(UnknownResolution {
            state: SubmissionState::Unknown,
            age,
            observation,
            resend: ResendObservation::NotWarranted,
        });
    }

    let Ok(pending_nonce) = endpoint.pending_nonce(&sender) else {
        return Ok(UnknownResolution {
            state: SubmissionState::Unknown,
            age,
            observation: ResolutionObservation::LookupUnavailable,
            resend: ResendObservation::NotWarranted,
        });
    };
    if pending_nonce > transaction.nonce {
        outbox.transition(
            store,
            submission_id,
            SubmissionState::Failed,
            "submission.nonce_consumed",
            None,
        )?;
        store.remove_local(&key)?;
        return Ok(UnknownResolution {
            state: SubmissionState::Failed,
            age,
            observation: ResolutionObservation::NonceConsumed,
            resend: ResendObservation::NotWarranted,
        });
    }
    let resend = if pending_nonce == transaction.nonce {
        match endpoint.send_raw_transaction(&record.signed_canonical_bytes) {
            Ok(hash) if hash == submission_id => ResendObservation::Sent,
            Ok(_) | Err(_) => ResendObservation::Indeterminate,
        }
    } else {
        ResendObservation::NotWarranted
    };
    Ok(UnknownResolution {
        state: SubmissionState::Unknown,
        age,
        observation: ResolutionObservation::ReceiptMissing,
        resend,
    })
}

fn record_authenticated_receipt(
    outbox: &mut Outbox,
    store: &mut Store,
    submission_id: [u8; 32],
    receipt: &AuthenticatedReceipt,
) -> Result<SubmissionState, UnknownResolutionError> {
    let mut record = outbox
        .records
        .get(&submission_id)
        .cloned()
        .ok_or(OutboxError::NotFound)?;
    if record.status.state != SubmissionState::Unknown || receipt.activity_id() != submission_id {
        return Err(UnknownResolutionError::Corrupt);
    }
    let state = if receipt.execution().success {
        SubmissionState::Executed
    } else {
        SubmissionState::Failed
    };
    let evidence = ReceiptEvidence {
        receipt_ref: Sha256::digest(receipt.canonical_bytes()).into(),
        block_height: receipt.height(),
        transaction_index: receipt.execution().index,
    };
    record.status.state = state;
    record.status.evidence = Some(evidence);
    record.status.transitions.push(StateTransition {
        from: SubmissionState::Unknown,
        to: state,
        cause: "verified sequencer receipt found by transaction hash".to_owned(),
        receipt: Some(evidence),
    });
    let receipt_key = TenantKey::new(
        record.tenant.clone(),
        ObjectKind::Receipt,
        evidence.receipt_ref.to_vec(),
    )?;
    store.put_local(receipt_key, receipt.canonical_bytes().to_vec())?;
    let key = TenantKey::new(
        record.tenant.clone(),
        ObjectKind::Outbox,
        submission_id.to_vec(),
    )?;
    store.put_local(key, encode_record(&record)?)?;
    outbox.records.insert(submission_id, record);
    Ok(state)
}

fn state_key(tenant: TenantId, submission_id: [u8; 32]) -> Result<TenantKey, StoreError> {
    let mut object_id = b"unknown-resolution:".to_vec();
    object_id.extend_from_slice(&submission_id);
    TenantKey::new(tenant, ObjectKind::Configuration, object_id)
}

fn backoff_ms(submission_id: [u8; 32], attempt_count: u32) -> u64 {
    let exponent = attempt_count.saturating_sub(1).min(6);
    let base = INITIAL_BACKOFF_MS
        .saturating_mul(1_u64 << exponent)
        .min(MAX_BACKOFF_MS);
    let digest = Sha256::digest([submission_id.as_slice(), &attempt_count.to_be_bytes()].concat());
    let jitter_window = base / 4;
    let jitter = u64::from_be_bytes(digest[..8].try_into().unwrap_or([0; 8])) % (jitter_window + 1);
    base.saturating_add(jitter).min(MAX_BACKOFF_MS)
}

fn encode_state(transaction_hash: [u8; 32], age: UnknownAge) -> Vec<u8> {
    let mut bytes = Vec::with_capacity(56);
    bytes.extend_from_slice(STATE_MAGIC);
    bytes.extend_from_slice(&transaction_hash);
    bytes.extend_from_slice(&age.first_observed_at_ms.to_be_bytes());
    bytes.extend_from_slice(&age.attempt_count.to_be_bytes());
    bytes.extend_from_slice(&age.next_attempt_at_ms.to_be_bytes());
    bytes
}

fn decode_state(
    bytes: &[u8],
    transaction_hash: [u8; 32],
    observed_at_ms: u64,
) -> Result<UnknownAge, UnknownResolutionError> {
    if bytes.starts_with(b"LXUR") {
        return Err(UnknownResolutionError::StateVersion);
    }
    if bytes.len() != 56 || &bytes[..4] != STATE_MAGIC || bytes[4..36] != transaction_hash {
        return Err(UnknownResolutionError::Corrupt);
    }
    let mut first = [0_u8; 8];
    first.copy_from_slice(&bytes[36..44]);
    let mut attempts = [0_u8; 4];
    attempts.copy_from_slice(&bytes[44..48]);
    let mut next = [0_u8; 8];
    next.copy_from_slice(&bytes[48..56]);
    let first_observed_at_ms = u64::from_be_bytes(first);
    if observed_at_ms < first_observed_at_ms {
        return Err(UnknownResolutionError::TimeRegressed);
    }
    Ok(UnknownAge {
        first_observed_at_ms,
        age_ms: observed_at_ms - first_observed_at_ms,
        attempt_count: u32::from_be_bytes(attempts),
        next_attempt_at_ms: u64::from_be_bytes(next),
    })
}
