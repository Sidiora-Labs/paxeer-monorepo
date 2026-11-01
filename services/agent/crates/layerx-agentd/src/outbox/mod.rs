//! Durable exact-byte submission outbox and explicit state machine.

use std::collections::BTreeMap;

use layerx_client::sequencer::SequencerBlock;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, EvmTransactionError,
};

use crate::prepare::PreparationAuthorization;
use crate::protocol_evidence::VerifiedReceiptEvidence;
use crate::session::{SessionId, SessionRef};
use crate::sign::VerifiedSubmission;
use crate::store::{ObjectKind, Store, StoreError, TenantId, TenantKey};

#[path = "unknown.rs"]
mod resolution;
#[path = "recover.rs"]
mod restart;

pub use resolution::{
    resolve_unknown, ReceiptLookup, ResendObservation, ResolutionObservation, UnknownAge,
    UnknownBoundaryError, UnknownResolution, UnknownResolutionError,
};
pub use restart::{
    recover, RecoveredOutbox, RecoveryError, RecoveryInputs, UnknownCeilingReservation,
};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SubmissionState {
    Prepared,
    Signed,
    Queued,
    Submitted,
    Unknown,
    Executed,
    Failed,
    Expired,
    Committed,
}

impl SubmissionState {
    #[must_use]
    pub const fn terminal(self) -> bool {
        matches!(self, Self::Committed | Self::Failed | Self::Expired)
    }

    const fn code(self) -> u8 {
        match self {
            Self::Prepared => 1,
            Self::Signed => 2,
            Self::Queued => 3,
            Self::Submitted => 4,
            Self::Unknown => 6,
            Self::Executed => 7,
            Self::Failed => 8,
            Self::Expired => 9,
            Self::Committed => 11,
        }
    }

    fn from_code(code: u8) -> Result<Self, OutboxError> {
        match code {
            1 => Ok(Self::Prepared),
            2 => Ok(Self::Signed),
            3 => Ok(Self::Queued),
            4 => Ok(Self::Submitted),
            6 => Ok(Self::Unknown),
            7 => Ok(Self::Executed),
            8 => Ok(Self::Failed),
            9 => Ok(Self::Expired),
            11 => Ok(Self::Committed),
            _ => Err(OutboxError::Corrupt),
        }
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ReceiptEvidence {
    receipt_ref: [u8; 32],
    block_height: u64,
    transaction_index: u64,
}

impl ReceiptEvidence {
    #[must_use]
    pub const fn receipt_ref(self) -> [u8; 32] {
        self.receipt_ref
    }

    #[must_use]
    pub const fn block_height(self) -> u64 {
        self.block_height
    }

    #[must_use]
    pub const fn transaction_index(self) -> u64 {
        self.transaction_index
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StateTransition {
    pub from: SubmissionState,
    pub to: SubmissionState,
    pub cause: String,
    pub receipt: Option<ReceiptEvidence>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SubmissionStatus {
    pub submission_id: [u8; 32],
    pub state: SubmissionState,
    pub activity_id: [u8; 32],
    pub evidence: Option<ReceiptEvidence>,
    pub transitions: Vec<StateTransition>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct OutboxRecord {
    tenant: TenantId,
    status: SubmissionStatus,
    signed_canonical_bytes: Vec<u8>,
    origin: Option<PreparationAuthorization>,
}

#[derive(Default)]
pub struct Outbox {
    records: BTreeMap<[u8; 32], OutboxRecord>,
}

impl Outbox {
    /// Durably queues exact raw transaction bytes and returns their keccak hash.
    ///
    /// # Errors
    /// Returns `InvalidTransaction` for invalid signed bytes, `Duplicate` if the hash is
    /// already stored, or a persistence error.
    pub fn enqueue(
        &mut self,
        store: &mut Store,
        tenant: TenantId,
        raw: Vec<u8>,
    ) -> Result<[u8; 32], OutboxError> {
        let (call, signature) = decode_signed(&raw).map_err(OutboxError::InvalidTransaction)?;
        recover_sender(&call, &signature).map_err(OutboxError::InvalidTransaction)?;
        self.enqueue_exact(store, tenant, raw, None, None)
    }

    /// Durably queues exact verified bytes together with the preparation authorization that
    /// admitted them, so recovery can classify a queued record that carries no authority.
    ///
    /// # Errors
    ///
    /// Returns the same errors as [`Outbox::enqueue`].
    pub(crate) fn enqueue_with_origin(
        &mut self,
        store: &mut Store,
        tenant: TenantId,
        submission_id: [u8; 32],
        verified: VerifiedSubmission,
        origin: Option<PreparationAuthorization>,
        owner: Option<&str>,
    ) -> Result<[u8; 32], OutboxError> {
        if verified.idempotency_key() != submission_id {
            return Err(OutboxError::IdempotencyMismatch);
        }
        self.enqueue_exact(store, tenant, verified.into_exact_bytes(), origin, owner)
    }

    fn enqueue_exact(
        &mut self,
        store: &mut Store,
        tenant: TenantId,
        signed_canonical_bytes: Vec<u8>,
        origin: Option<PreparationAuthorization>,
        owner: Option<&str>,
    ) -> Result<[u8; 32], OutboxError> {
        let submission_id = transaction_hash(&signed_canonical_bytes);
        let key = TenantKey::new(tenant.clone(), ObjectKind::Outbox, submission_id.to_vec())
            .map_err(OutboxError::Store)?;
        if self.records.contains_key(&submission_id) || store.get(&key).is_some() {
            return Err(OutboxError::Duplicate);
        }
        let activity_id = submission_id;
        let transitions = vec![
            StateTransition {
                from: SubmissionState::Prepared,
                to: SubmissionState::Signed,
                cause: "exact signature verified".to_owned(),
                receipt: None,
            },
            StateTransition {
                from: SubmissionState::Signed,
                to: SubmissionState::Queued,
                cause: "durable outbox record created".to_owned(),
                receipt: None,
            },
        ];
        let record = OutboxRecord {
            tenant: tenant.clone(),
            status: SubmissionStatus {
                submission_id,
                state: SubmissionState::Queued,
                activity_id,
                evidence: None,
                transitions,
            },
            signed_canonical_bytes: signed_canonical_bytes.clone(),
            origin,
        };
        store
            .record_submission_with_activity_owner(
                tenant,
                submission_id.to_vec(),
                signed_canonical_bytes,
                encode_record(&record)?,
                owner.map(|principal| (activity_id, principal)),
            )
            .map_err(OutboxError::Store)?;
        self.records.insert(submission_id, record);
        Ok(submission_id)
    }

    /// Restores one durable record by its tenant-scoped transaction hash.
    ///
    /// # Errors
    ///
    /// Returns `NotFound` when no durable outbox record exists, `Corrupt` for a missing signed-bytes
    /// record, undecodable state or a record naming a different submission, and `Store` when either
    /// tenant key cannot be built.
    pub fn restore(
        &mut self,
        store: &Store,
        tenant: TenantId,
        submission_id: [u8; 32],
    ) -> Result<(), OutboxError> {
        let outbox_key = TenantKey::new(tenant.clone(), ObjectKind::Outbox, submission_id.to_vec())
            .map_err(OutboxError::Store)?;
        let signed_key = TenantKey::new(
            tenant.clone(),
            ObjectKind::PreparedActivity,
            submission_id.to_vec(),
        )
        .map_err(OutboxError::Store)?;
        let encoded = store.get(&outbox_key).ok_or(OutboxError::NotFound)?;
        let signed = store.get(&signed_key).ok_or(OutboxError::Corrupt)?;
        let mut record = decode_record(encoded.bytes(), tenant)?;
        if record.status.submission_id != submission_id
            || transaction_hash(signed.bytes()) != submission_id
        {
            return Err(OutboxError::Corrupt);
        }
        record.signed_canonical_bytes = signed.bytes().to_vec();
        self.records.insert(submission_id, record);
        Ok(())
    }

    /// Returns the session generation that authorized the durable submission, if one was recorded.
    ///
    /// # Errors
    ///
    /// Returns `NotFound` for a submission this outbox never enqueued or restored.
    pub(crate) fn origin(
        &self,
        submission_id: [u8; 32],
    ) -> Result<Option<PreparationAuthorization>, OutboxError> {
        self.records
            .get(&submission_id)
            .map(|record| record.origin.clone())
            .ok_or(OutboxError::NotFound)
    }

    /// Returns exact stored bytes only after the queued record is durable.
    ///
    /// # Errors
    ///
    /// Returns `NotFound` for a submission this outbox never enqueued or restored, and `NotQueued`
    /// once it has left `Queued` for any later state.
    pub fn bytes_for_transmission(&self, submission_id: [u8; 32]) -> Result<&[u8], OutboxError> {
        let record = self
            .records
            .get(&submission_id)
            .ok_or(OutboxError::NotFound)?;
        if record.status.state != SubmissionState::Queued {
            return Err(OutboxError::NotQueued);
        }
        Ok(&record.signed_canonical_bytes)
    }

    /// Returns the exact durable signed activity at any later lifecycle state.
    ///
    /// # Errors
    ///
    /// Returns an error if the submission does not exist or its retained bytes are invalid.
    pub fn exact_signed_bytes(&self, submission_id: [u8; 32]) -> Result<&[u8], OutboxError> {
        self.records
            .get(&submission_id)
            .map(|record| record.signed_canonical_bytes.as_slice())
            .ok_or(OutboxError::NotFound)
    }

    /// Applies one legal state change and durably records it with its cause.
    ///
    /// # Errors
    ///
    /// Returns `EmptyCause` for a blank cause, `NotFound` for an untracked submission,
    /// `InvalidTransition` for a move `legal_transition` forbids, `SuccessWithoutVerifiedReceipt`
    /// for `Executed` without verified evidence, `ReceiptMismatch` for evidence naming another
    /// activity, and `Corrupt` or `Store` when the updated record cannot be encoded or persisted.
    pub fn transition(
        &mut self,
        store: &mut Store,
        submission_id: [u8; 32],
        to: SubmissionState,
        cause: impl Into<String>,
        receipt: Option<VerifiedReceiptEvidence>,
    ) -> Result<SubmissionStatus, OutboxError> {
        self.transition_at(store, submission_id, to, cause.into(), receipt, None, None)
    }

    /// Records verified execution together with its block position.
    ///
    /// # Errors
    /// Returns the same transition and persistence errors as `transition`.
    pub fn record_execution(
        &mut self,
        store: &mut Store,
        submission_id: [u8; 32],
        receipt: VerifiedReceiptEvidence,
        block_height: u64,
        transaction_index: u64,
    ) -> Result<SubmissionStatus, OutboxError> {
        if receipt.result_code() != 0 {
            return Err(OutboxError::ReceiptFailed);
        }
        self.transition_at(
            store,
            submission_id,
            SubmissionState::Executed,
            "verified execution receipt".to_owned(),
            Some(receipt),
            Some((block_height, transaction_index)),
            None,
        )
    }

    /// Records a verified failure receipt at its block position.
    ///
    /// # Errors
    /// Refuses a successful receipt or an illegal transition, or returns a persistence error.
    pub fn record_failure(
        &mut self,
        store: &mut Store,
        submission_id: [u8; 32],
        receipt: VerifiedReceiptEvidence,
        block_height: u64,
        transaction_index: u64,
    ) -> Result<SubmissionStatus, OutboxError> {
        if receipt.result_code() == 0 {
            return Err(OutboxError::ReceiptMismatch);
        }
        self.transition_at(
            store,
            submission_id,
            SubmissionState::Failed,
            "verified failure receipt".to_owned(),
            Some(receipt),
            Some((block_height, transaction_index)),
            None,
        )
    }

    /// Reconciles a committed block, including a transaction that consumed our nonce.
    ///
    /// # Errors
    /// Refuses malformed signed transactions, inconsistent receipt locations or persistence errors.
    pub fn reconcile_committed_block(
        &mut self,
        store: &mut Store,
        submission_id: [u8; 32],
        block: &SequencerBlock,
    ) -> Result<SubmissionStatus, OutboxError> {
        let record = self
            .records
            .get(&submission_id)
            .ok_or(OutboxError::NotFound)?;
        if record.status.state.terminal() {
            return Ok(record.status.clone());
        }
        if record.status.state == SubmissionState::Executed {
            return self.transition_at(
                store,
                submission_id,
                SubmissionState::Committed,
                "receipt block committed".to_owned(),
                None,
                None,
                Some(block),
            );
        }
        let (ours, signature) =
            decode_signed(&record.signed_canonical_bytes).map_err(|_| OutboxError::Corrupt)?;
        let sender = recover_sender(&ours, &signature).map_err(|_| OutboxError::Corrupt)?;
        for raw in &block.transactions {
            let (other, signature) = decode_signed(raw).map_err(|_| OutboxError::Corrupt)?;
            if other.chain_id == ours.chain_id
                && other.nonce == ours.nonce
                && recover_sender(&other, &signature).map_err(|_| OutboxError::Corrupt)? == sender
                && transaction_hash(raw) != submission_id
            {
                return self.transition(
                    store,
                    submission_id,
                    SubmissionState::Failed,
                    "submission.nonce_consumed",
                    None,
                );
            }
        }
        Ok(record.status.clone())
    }

    fn transition_at(
        &mut self,
        store: &mut Store,
        submission_id: [u8; 32],
        to: SubmissionState,
        cause: String,
        receipt: Option<VerifiedReceiptEvidence>,
        location: Option<(u64, u64)>,
        committed: Option<&SequencerBlock>,
    ) -> Result<SubmissionStatus, OutboxError> {
        if cause.is_empty() {
            return Err(OutboxError::EmptyCause);
        }
        let current = self
            .records
            .get(&submission_id)
            .cloned()
            .ok_or(OutboxError::NotFound)?;
        let from = current.status.state;
        if !legal_transition(from, to) {
            return Err(OutboxError::InvalidTransition { from, to });
        }
        if to == SubmissionState::Executed && receipt.is_none() {
            return Err(OutboxError::SuccessWithoutVerifiedReceipt);
        }
        if to == SubmissionState::Executed
            && receipt
                .as_ref()
                .is_some_and(|evidence| evidence.result_code() != 0)
        {
            return Err(OutboxError::ReceiptFailed);
        }
        if receipt
            .as_ref()
            .is_some_and(|evidence| evidence.activity_id() != current.status.activity_id)
        {
            return Err(OutboxError::ReceiptMismatch);
        }
        if to == SubmissionState::Committed {
            let evidence = current
                .status
                .evidence
                .ok_or(OutboxError::SuccessWithoutVerifiedReceipt)?;
            let block = committed.ok_or(OutboxError::BlockNotCommitted)?;
            let index = usize::try_from(evidence.transaction_index)
                .map_err(|_| OutboxError::ReceiptMismatch)?;
            if block.number != evidence.block_height
                || block
                    .transactions
                    .get(index)
                    .map(|raw| transaction_hash(raw))
                    != Some(submission_id)
            {
                return Err(OutboxError::ReceiptMismatch);
            }
        }
        let receipt = if let Some(evidence) = receipt {
            let (block_height, transaction_index) =
                location.ok_or(OutboxError::ReceiptLocationRequired)?;
            Some(ReceiptEvidence {
                receipt_ref: evidence.receipt_ref(),
                block_height,
                transaction_index,
            })
        } else {
            current.status.evidence
        };
        let mut updated = current;
        updated.status.state = to;
        updated.status.evidence = receipt;
        updated.status.transitions.push(StateTransition {
            from,
            to,
            cause,
            receipt,
        });
        let key = TenantKey::new(
            updated.tenant.clone(),
            ObjectKind::Outbox,
            submission_id.to_vec(),
        )
        .map_err(OutboxError::Store)?;
        store
            .put_local(key, encode_record(&updated)?)
            .map_err(OutboxError::Store)?;
        let status = updated.status.clone();
        self.records.insert(submission_id, updated);
        Ok(status)
    }

    #[must_use]
    pub fn status(&self, submission_id: [u8; 32]) -> Option<&SubmissionStatus> {
        self.records
            .get(&submission_id)
            .map(|record| &record.status)
    }
}

#[derive(Debug)]
pub enum OutboxError {
    Duplicate,
    IdempotencyMismatch,
    NotFound,
    NotQueued,
    EmptyCause,
    Corrupt,
    SuccessWithoutVerifiedReceipt,
    ReceiptMismatch,
    ReceiptLocationRequired,
    BlockNotCommitted,
    ReceiptFailed,
    InvalidTransaction(EvmTransactionError),
    InvalidTransition {
        from: SubmissionState,
        to: SubmissionState,
    },
    Store(StoreError),
}

pub fn legal_transition(from: SubmissionState, to: SubmissionState) -> bool {
    matches!(
        (from, to),
        (
            SubmissionState::Queued,
            SubmissionState::Submitted | SubmissionState::Expired | SubmissionState::Failed
        ) | (
            SubmissionState::Submitted,
            SubmissionState::Submitted
                | SubmissionState::Unknown
                | SubmissionState::Executed
                | SubmissionState::Failed
        ) | (
            SubmissionState::Unknown,
            SubmissionState::Executed | SubmissionState::Failed
        ) | (SubmissionState::Executed, SubmissionState::Committed)
    )
}

fn encode_record(record: &OutboxRecord) -> Result<Vec<u8>, OutboxError> {
    let mut bytes = Vec::new();
    bytes.extend_from_slice(b"LXOB");
    bytes.push(4);
    bytes.extend_from_slice(&record.status.submission_id);
    bytes.push(record.status.state.code());
    bytes.extend_from_slice(&record.status.activity_id);
    encode_receipt(&mut bytes, record.status.evidence);
    push_u32(&mut bytes, record.status.transitions.len())?;
    for transition in &record.status.transitions {
        bytes.push(transition.from.code());
        bytes.push(transition.to.code());
        push_bytes(&mut bytes, transition.cause.as_bytes())?;
        encode_receipt(&mut bytes, transition.receipt);
    }
    match &record.origin {
        Some(origin) => {
            bytes.push(1);
            push_bytes(&mut bytes, origin.session.tenant.as_str().as_bytes())?;
            bytes.extend_from_slice(&origin.session.session_id.0);
            bytes.extend_from_slice(&origin.generation.to_be_bytes());
        }
        None => bytes.push(0),
    }
    Ok(bytes)
}

fn decode_record(bytes: &[u8], tenant: TenantId) -> Result<OutboxRecord, OutboxError> {
    let mut decoder = RecordDecoder { bytes, offset: 0 };
    if decoder.take(4)? != b"LXOB" {
        return Err(OutboxError::Corrupt);
    }
    let version = decoder.u8()?;
    if version != 4 {
        return Err(OutboxError::Corrupt);
    }
    let submission_id = decoder.fixed()?;
    let state = SubmissionState::from_code(decoder.u8()?)?;
    let activity_id = decoder.fixed()?;
    let evidence = decoder.receipt()?;
    let transition_count = decoder.u32()? as usize;
    if transition_count > 1_024 {
        return Err(OutboxError::Corrupt);
    }
    let mut transitions = Vec::with_capacity(transition_count);
    for _ in 0..transition_count {
        let from = SubmissionState::from_code(decoder.u8()?)?;
        let to = SubmissionState::from_code(decoder.u8()?)?;
        let cause =
            String::from_utf8(decoder.bytes()?.to_vec()).map_err(|_| OutboxError::Corrupt)?;
        let receipt = decoder.receipt()?;
        transitions.push(StateTransition {
            from,
            to,
            cause,
            receipt,
        });
    }
    let origin = decoder.origin()?;
    if decoder.offset != bytes.len() {
        return Err(OutboxError::Corrupt);
    }
    Ok(OutboxRecord {
        tenant,
        status: SubmissionStatus {
            submission_id,
            state,
            activity_id,
            evidence,
            transitions,
        },
        signed_canonical_bytes: Vec::new(),
        origin,
    })
}

fn encode_receipt(bytes: &mut Vec<u8>, receipt: Option<ReceiptEvidence>) {
    match receipt {
        Some(receipt) => {
            bytes.push(1);
            bytes.extend_from_slice(&receipt.receipt_ref);
            bytes.extend_from_slice(&receipt.block_height.to_be_bytes());
            bytes.extend_from_slice(&receipt.transaction_index.to_be_bytes());
        }
        None => bytes.push(0),
    }
}

fn push_u32(bytes: &mut Vec<u8>, value: usize) -> Result<(), OutboxError> {
    let value = u32::try_from(value).map_err(|_| OutboxError::Corrupt)?;
    bytes.extend_from_slice(&value.to_be_bytes());
    Ok(())
}

fn push_bytes(bytes: &mut Vec<u8>, value: &[u8]) -> Result<(), OutboxError> {
    push_u32(bytes, value.len())?;
    bytes.extend_from_slice(value);
    Ok(())
}

struct RecordDecoder<'a> {
    bytes: &'a [u8],
    offset: usize,
}

impl<'a> RecordDecoder<'a> {
    fn take(&mut self, length: usize) -> Result<&'a [u8], OutboxError> {
        let end = self
            .offset
            .checked_add(length)
            .ok_or(OutboxError::Corrupt)?;
        let value = self
            .bytes
            .get(self.offset..end)
            .ok_or(OutboxError::Corrupt)?;
        self.offset = end;
        Ok(value)
    }

    fn u8(&mut self) -> Result<u8, OutboxError> {
        Ok(*self.take(1)?.first().ok_or(OutboxError::Corrupt)?)
    }

    fn u32(&mut self) -> Result<u32, OutboxError> {
        let mut value = [0; 4];
        value.copy_from_slice(self.take(4)?);
        Ok(u32::from_be_bytes(value))
    }

    fn fixed(&mut self) -> Result<[u8; 32], OutboxError> {
        let mut value = [0; 32];
        value.copy_from_slice(self.take(32)?);
        Ok(value)
    }

    fn bytes(&mut self) -> Result<&'a [u8], OutboxError> {
        let length = self.u32()? as usize;
        if length > 1_048_576 {
            return Err(OutboxError::Corrupt);
        }
        self.take(length)
    }

    fn u64(&mut self) -> Result<u64, OutboxError> {
        let mut value = [0; 8];
        value.copy_from_slice(self.take(8)?);
        Ok(u64::from_be_bytes(value))
    }

    fn origin(&mut self) -> Result<Option<PreparationAuthorization>, OutboxError> {
        match self.u8()? {
            0 => Ok(None),
            1 => {
                let tenant = std::str::from_utf8(self.bytes()?)
                    .map_err(|_| OutboxError::Corrupt)
                    .and_then(|text| TenantId::new(text).map_err(|_| OutboxError::Corrupt))?;
                let session_id = SessionId(self.fixed()?);
                let generation = self.u64()?;
                Ok(Some(PreparationAuthorization {
                    session: SessionRef::new(tenant, session_id),
                    generation,
                }))
            }
            _ => Err(OutboxError::Corrupt),
        }
    }

    fn receipt(&mut self) -> Result<Option<ReceiptEvidence>, OutboxError> {
        match self.u8()? {
            0 => Ok(None),
            1 => Ok(Some(ReceiptEvidence {
                receipt_ref: self.fixed()?,
                block_height: self.u64()?,
                transaction_index: self.u64()?,
            })),
            _ => Err(OutboxError::Corrupt),
        }
    }
}

#[cfg(test)]
mod origin_codec_tests {
    use super::{
        decode_record, encode_record, OutboxRecord, PreparationAuthorization, ReceiptEvidence,
        SessionId, SessionRef, StateTransition, SubmissionState, SubmissionStatus, TenantId,
    };

    fn tenant() -> TenantId {
        TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}"))
    }

    fn status() -> SubmissionStatus {
        SubmissionStatus {
            submission_id: [7; 32],
            state: SubmissionState::Queued,
            activity_id: [9; 32],
            evidence: None,
            transitions: vec![
                StateTransition {
                    from: SubmissionState::Prepared,
                    to: SubmissionState::Signed,
                    cause: "exact signature verified".to_owned(),
                    receipt: None,
                },
                StateTransition {
                    from: SubmissionState::Signed,
                    to: SubmissionState::Queued,
                    cause: "durable outbox record created".to_owned(),
                    receipt: Some(ReceiptEvidence {
                        receipt_ref: [4; 32],
                        block_height: 8,
                        transaction_index: 2,
                    }),
                },
            ],
        }
    }

    #[test]
    fn version_four_round_trips_origin() {
        let record = OutboxRecord {
            tenant: tenant(),
            status: status(),
            signed_canonical_bytes: Vec::new(),
            origin: Some(PreparationAuthorization {
                session: SessionRef::new(tenant(), SessionId([3; 32])),
                generation: 0x0102_0304_0506_0708,
            }),
        };
        let encoded = encode_record(&record).unwrap_or_else(|error| panic!("encode: {error:?}"));
        assert_eq!(&encoded[..5], b"LXOB\x04");
        let decoded =
            decode_record(&encoded, tenant()).unwrap_or_else(|error| panic!("decode: {error:?}"));
        assert_eq!(decoded, record);

        let mut trailing = encoded.clone();
        trailing.push(0);
        assert!(decode_record(&trailing, tenant()).is_err());
        let mut unknown_version = encoded;
        unknown_version[4] = 5;
        assert!(decode_record(&unknown_version, tenant()).is_err());
    }

    #[test]
    fn version_two_layout_is_refused_without_transaction_identity() {
        let status = status();
        let mut v2 = Vec::new();
        v2.extend_from_slice(b"LXOB");
        v2.push(2);
        v2.extend_from_slice(&status.submission_id);
        v2.push(3);
        v2.extend_from_slice(&status.activity_id);
        v2.push(0);
        v2.extend_from_slice(&2_u32.to_be_bytes());
        for (from, to, cause, receipt) in [
            (1_u8, 2_u8, "exact signature verified", None),
            (2, 3, "durable outbox record created", Some([4_u8; 32])),
        ] {
            v2.push(from);
            v2.push(to);
            let length = u32::try_from(cause.len()).unwrap_or_else(|error| panic!("{error}"));
            v2.extend_from_slice(&length.to_be_bytes());
            v2.extend_from_slice(cause.as_bytes());
            match receipt {
                Some(reference) => {
                    v2.push(1);
                    v2.extend_from_slice(&reference);
                }
                None => v2.push(0),
            }
        }
        assert!(decode_record(&v2, tenant()).is_err());
        let mut with_origin_tag = v2;
        with_origin_tag.push(0);
        assert!(decode_record(&with_origin_tag, tenant()).is_err());
    }
}
