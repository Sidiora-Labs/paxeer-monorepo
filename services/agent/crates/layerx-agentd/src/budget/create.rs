//! Budget creation through layerxpay EIP-1559 calls and committed receipts.

use crate::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
use layerx_client::sequencer::{ExecutedReceipt, SequencerEndpoint, SequencerReceipt};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_types::payload::ModuleRegistry;

use super::accounting::{ProtocolBudgetRecord, ProtocolBudgetState};
use crate::protocol_evidence::{EvidenceAuthority, RawReceiptEvidence};
use crate::sign::VerifiedSubmission;
use crate::store::{Store, TenantId};

const LOCAL_BYPASS_STATEMENT: &str =
    "daemon-enforced only; bypassing layerx-agentd bypasses this limit";
const BUDGET_CREATE: &str = "budgetCreate(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8)";
const BUDGET_CREATE_NATIVE: &str = "budgetCreateNative(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8,bytes32,uint64)";
const PROTOCOL_ENFORCEMENT: &str = "protocol-enforced";

/// Protocol object offered for a spending limit.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum BudgetKind {
    ProtocolBudget,
    CapabilityGrant,
}

/// Complete input to a protocol-enforced limit creation activity.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BudgetRequest {
    pub tenant: TenantId,
    pub request_id: [u8; 32],
    pub kind: BudgetKind,
    pub asset: [u8; 32],
    pub ceiling: u128,
    /// Core budget expiry in milliseconds of core batch time.
    pub expiry_ms: u64,
    /// Verified core batch time the request is admitted against; the budget
    /// must expire strictly after it, as core refuses `expiry <= batch time`.
    pub core_time_ms: u64,
    pub canonical_activity: Vec<u8>,
    pub verified_submission: Option<VerifiedSubmission>,
}

/// Receipt evidence returned for the exact submitted budget transaction.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CoreBudgetReceipt<E = RawReceiptEvidence> {
    pub evidence: E,
}

/// The mandatory exact-submission, raw-evidence and proven-state seam.
pub trait BudgetPipeline<R = RawReceiptEvidence, S = crate::protocol_evidence::RawStateEvidence> {
    /// Submits the byte-identical verifier-bound budget activity and returns raw core evidence.
    ///
    /// # Errors
    ///
    /// Returns `Submission` when the exact signed canonical activity does not reach core and
    /// produce a receipt.
    fn submit_budget(
        &mut self,
        request: &BudgetRequest,
    ) -> Result<CoreBudgetReceipt<R>, BudgetCreationError>;

    /// Reads the proven budget module state stored under `budget_state_key(budget_id)`.
    ///
    /// # Errors
    ///
    /// Returns `CreatedBudgetUnconfirmed` when the node serves no proven state for the key.
    fn budget_state(
        &mut self,
        budget_id: [u8; 32],
    ) -> Result<ProtocolBudgetState<S>, BudgetCreationError>;
}

/// ABI creation fields and the recovered EVM sender.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct BudgetCreateIdentity {
    pub encoding_version: u16,
    pub budget_id: [u8; 32],
    pub sender: [u8; 20],
    pub budget_account: [u8; 32],
    pub asset: [u8; 32],
    pub purpose: [u8; 32],
    pub per_period_limit: u128,
    pub carry_cap: u128,
    pub initial_amount: u128,
    pub period_length_ms: u64,
    pub period_start_ms: u64,
    pub expiry_ms: u64,
    pub revocation_sequence: u64,
    pub rollover: u8,
    pub source_account: [u8; 32],
    pub source_sequence: u64,
}

pub trait BudgetIdentitySource {
    fn identity(&self) -> Result<BudgetCreateIdentity, BudgetCreationError>;
}

pub fn budget_create_identity<T: BudgetIdentitySource + ?Sized>(
    source: &T,
    _registry: &ModuleRegistry,
) -> Result<BudgetCreateIdentity, BudgetCreationError> {
    source.identity()
}

pub(crate) fn selector(signature: &str) -> [u8; 4] {
    transaction_hash(signature.as_bytes())[..4]
        .try_into()
        .expect("selector width")
}

pub(crate) fn word(data: &[u8], index: usize) -> Result<[u8; 32], BudgetCreationError> {
    data.get(4 + index * 32..4 + (index + 1) * 32)
        .and_then(|value| value.try_into().ok())
        .ok_or(BudgetCreationError::NotBudgetCreation)
}

pub(crate) fn integer(
    data: &[u8],
    index: usize,
    width: usize,
) -> Result<u128, BudgetCreationError> {
    let value = word(data, index)?;
    if value[..32 - width].iter().any(|byte| *byte != 0) {
        return Err(BudgetCreationError::NotBudgetCreation);
    }
    let mut bytes = [0; 16];
    bytes[16 - width..].copy_from_slice(&value[32 - width..]);
    Ok(u128::from_be_bytes(bytes))
}

impl BudgetIdentitySource for [u8] {
    fn identity(&self) -> Result<BudgetCreateIdentity, BudgetCreationError> {
        let (call, signature) =
            decode_signed(self).map_err(|_| BudgetCreationError::NotBudgetCreation)?;
        let native = call.data.starts_with(&selector(BUDGET_CREATE_NATIVE));
        if call.to != PrecompileTarget::Pay.address()
            || (!native && !call.data.starts_with(&selector(BUDGET_CREATE)))
            || call.data.len() != 4 + if native { 14 * 32 } else { 12 * 32 }
        {
            return Err(BudgetCreationError::NotBudgetCreation);
        }
        let data = &call.data;
        let rollover = integer(data, 11, 1)? as u8;
        let carry_cap = integer(data, 5, 16)?;
        let period_length_ms = integer(data, 7, 8)? as u64;
        let period_start_ms = integer(data, 8, 8)? as u64;
        let expiry_ms = integer(data, 9, 8)? as u64;
        if word(data, 0)? == [0; 32]
            || period_length_ms == 0
            || period_start_ms.checked_add(period_length_ms).is_none()
            || expiry_ms <= period_start_ms
            || !(1..=2).contains(&rollover)
            || (rollover == 1 && carry_cap != 0)
        {
            return Err(BudgetCreationError::InvalidLimit);
        }
        Ok(BudgetCreateIdentity {
            encoding_version: if native { 2 } else { 1 },
            budget_id: word(data, 0)?,
            sender: recover_sender(&call, &signature)
                .map_err(|_| BudgetCreationError::NotBudgetCreation)?,
            budget_account: word(data, 1)?,
            asset: word(data, 2)?,
            purpose: word(data, 3)?,
            per_period_limit: integer(data, 4, 16)?,
            carry_cap,
            initial_amount: integer(data, 6, 16)?,
            period_length_ms,
            period_start_ms,
            expiry_ms,
            revocation_sequence: integer(data, 10, 8)? as u64,
            rollover,
            source_account: if native { word(data, 12)? } else { [0; 32] },
            source_sequence: if native {
                integer(data, 13, 8)? as u64
            } else {
                0
            },
        })
    }
}

impl BudgetIdentitySource for (&[u8], &SequencerReceipt) {
    fn identity(&self) -> Result<BudgetCreateIdentity, BudgetCreationError> {
        let mut identity = self.0.identity()?;
        let (call, _) =
            decode_signed(self.0).map_err(|_| BudgetCreationError::NotBudgetCreation)?;
        let receipt = committed_receipt(self.1, transaction_hash(self.0))?;
        let event = budget_log(receipt, "BudgetCreated(bytes32,bytes32,uint128)")?;
        if event.data.len() != 64
            || event.data[..32] != identity.asset
            || event.data[32..48].iter().any(|byte| *byte != 0)
            || event.data[48..] != identity.initial_amount.to_be_bytes()
            || event.topics[1] != identity.budget_id
            || receipt.from != identity.sender
            || receipt.nonce != call.nonce
        {
            return Err(BudgetCreationError::IdentityMismatch);
        }
        identity.budget_id = event.topics[1];
        Ok(identity)
    }
}

pub(crate) fn committed_receipt(
    receipt: &SequencerReceipt,
    expected: [u8; 32],
) -> Result<&ExecutedReceipt, BudgetCreationError> {
    let SequencerReceipt::Committed {
        receipt,
        block_number,
        block_hash,
    } = receipt
    else {
        return Err(BudgetCreationError::UnverifiedReceipt);
    };
    if receipt.tx_hash != expected {
        return Err(BudgetCreationError::ReceiptActivityMismatch);
    }
    if !receipt.success {
        return Err(BudgetCreationError::CoreRejected);
    }
    if *block_number == 0
        || receipt.height != *block_number
        || *block_hash == [0; 32]
        || receipt.to != PrecompileTarget::Pay.address()
        || !receipt.vm_error.is_empty()
    {
        return Err(BudgetCreationError::UnverifiedReceipt);
    }
    Ok(receipt)
}

pub(crate) fn budget_log<'a>(
    receipt: &'a ExecutedReceipt,
    signature: &str,
) -> Result<&'a layerx_client::sequencer::SequencerLog, BudgetCreationError> {
    let topic = transaction_hash(signature.as_bytes());
    let mut logs = receipt.logs.iter().filter(|log| {
        log.address == PrecompileTarget::Pay.address() && log.topics.first() == Some(&topic)
    });
    let log = logs.next().ok_or(BudgetCreationError::UnverifiedReceipt)?;
    if logs.next().is_some() || log.topics.len() != 2 || log.topics[1] == [0; 32] {
        return Err(BudgetCreationError::UnverifiedReceipt);
    }
    Ok(log)
}

impl BudgetRequest {
    pub fn create_protocol_budget(
        &self,
        identity: &BudgetCreateIdentity,
    ) -> Result<PrecompileCall, BudgetCreationError> {
        if self.ceiling == 0 || self.expiry_ms == 0 {
            return Err(BudgetCreationError::InvalidLimit);
        }
        if self.expiry_ms <= self.core_time_ms {
            return Err(BudgetCreationError::Expired);
        }
        if identity.encoding_version != 1 && identity.encoding_version != 2 {
            return Err(BudgetCreationError::NotBudgetCreation);
        }
        if identity.budget_id == [0; 32]
            || identity.budget_account == [0; 32]
            || identity.period_length_ms == 0
            || identity.period_start_ms >= self.expiry_ms
            || identity
                .period_start_ms
                .checked_add(identity.period_length_ms)
                .is_none()
            || !(1..=2).contains(&identity.rollover)
            || (identity.rollover == 1 && identity.carry_cap != 0)
            || (identity.encoding_version == 2
                && (identity.source_account == [0; 32] || identity.source_sequence == 0))
        {
            return Err(BudgetCreationError::InvalidLimit);
        }
        let number = |value: u128| {
            let mut word = [0; 32];
            word[16..].copy_from_slice(&value.to_be_bytes());
            AbiArgument::Word(word)
        };
        let mut arguments = vec![
            AbiArgument::Word(identity.budget_id),
            AbiArgument::Word(identity.budget_account),
            AbiArgument::Word(self.asset),
            AbiArgument::Word(identity.purpose),
            number(self.ceiling),
            number(identity.carry_cap),
            number(identity.initial_amount),
            number(identity.period_length_ms.into()),
            number(identity.period_start_ms.into()),
            number(self.expiry_ms.into()),
            number(identity.revocation_sequence.into()),
            number(identity.rollover.into()),
        ];
        if identity.encoding_version == 2 {
            arguments.extend([
                AbiArgument::Word(identity.source_account),
                number(identity.source_sequence.into()),
            ]);
        }
        Ok(PrecompileCall {
            target: PrecompileTarget::Pay,
            selector: selector(if identity.encoding_version == 2 {
                BUDGET_CREATE_NATIVE
            } else {
                BUDGET_CREATE
            }),
            arguments,
        })
    }
    pub fn pipeline(
        endpoint: SequencerEndpoint,
    ) -> impl BudgetPipeline<SequencerReceipt, super::accounting::CommittedBudgetEvidence>
           + super::mutate::BudgetMutationPipeline<
        SequencerReceipt,
        super::accounting::CommittedBudgetEvidence,
    > {
        SequencerBudgetPipeline {
            endpoint,
            committed_height: None,
        }
    }
}

pub(crate) struct SequencerBudgetPipeline {
    pub(crate) endpoint: SequencerEndpoint,
    pub(crate) committed_height: Option<u64>,
}
impl SequencerBudgetPipeline {
    pub(crate) fn submit(
        &mut self,
        submission: &VerifiedSubmission,
    ) -> Result<CoreBudgetReceipt<SequencerReceipt>, BudgetCreationError> {
        self.committed_height = None;
        let hash = self
            .endpoint
            .send_raw_transaction(submission.exact_bytes())
            .map_err(|_| BudgetCreationError::Submission)?;
        if hash != submission.activity_id() {
            return Err(BudgetCreationError::ReceiptActivityMismatch);
        }
        for attempt in 0..20 {
            let evidence = self
                .endpoint
                .sequencer_receipt(&hash)
                .map_err(|_| BudgetCreationError::Submission)?;
            if let Some(evidence @ SequencerReceipt::Committed { .. }) = evidence {
                let receipt = committed_receipt(&evidence, hash)?;
                self.committed_height = Some(receipt.height);
                return Ok(CoreBudgetReceipt { evidence });
            }
            if attempt < 19 {
                std::thread::sleep(std::time::Duration::from_millis(250));
            }
        }
        Err(BudgetCreationError::Submission)
    }
    pub(crate) fn read(
        &self,
        budget_id: [u8; 32],
    ) -> Result<ProtocolBudgetState<super::accounting::CommittedBudgetEvidence>, BudgetCreationError>
    {
        let height = self
            .committed_height
            .ok_or(BudgetCreationError::CreatedBudgetUnconfirmed)?;
        ProtocolBudgetState::read_at(&self.endpoint, budget_id, height)
            .map_err(|_| BudgetCreationError::CreatedBudgetUnconfirmed)
    }
}
impl BudgetPipeline<SequencerReceipt, super::accounting::CommittedBudgetEvidence>
    for SequencerBudgetPipeline
{
    fn submit_budget(
        &mut self,
        request: &BudgetRequest,
    ) -> Result<CoreBudgetReceipt<SequencerReceipt>, BudgetCreationError> {
        self.submit(
            request
                .verified_submission
                .as_ref()
                .ok_or(BudgetCreationError::ActivityBindingUnavailable)?,
        )
    }
    fn budget_state(
        &mut self,
        budget_id: [u8; 32],
    ) -> Result<ProtocolBudgetState<super::accounting::CommittedBudgetEvidence>, BudgetCreationError>
    {
        self.read(budget_id)
    }
}

pub trait BudgetReceiptEvidence {
    fn layerxpay_receipt(&self) -> Result<&SequencerReceipt, BudgetCreationError>;
}
impl BudgetReceiptEvidence for SequencerReceipt {
    fn layerxpay_receipt(&self) -> Result<&SequencerReceipt, BudgetCreationError> {
        Ok(self)
    }
}
impl BudgetReceiptEvidence for RawReceiptEvidence {
    fn layerxpay_receipt(&self) -> Result<&SequencerReceipt, BudgetCreationError> {
        Err(BudgetCreationError::UnverifiedReceipt)
    }
}

pub trait BudgetStateEvidence {
    fn verified_record(
        &self,
        verifier: &EvidenceAuthority,
    ) -> Result<(ProtocolBudgetRecord, u64), BudgetCreationError>;
}
impl BudgetStateEvidence for ProtocolBudgetState {
    fn verified_record(
        &self,
        verifier: &EvidenceAuthority,
    ) -> Result<(ProtocolBudgetRecord, u64), BudgetCreationError> {
        let state = verifier
            .verify_state(&self.evidence)
            .map_err(|_| BudgetCreationError::CreatedBudgetUnconfirmed)?;
        let record = ProtocolBudgetRecord::decode(state.canonical_state())
            .map_err(|_| BudgetCreationError::CreatedBudgetUnconfirmed)?;
        Ok((record, state.observed_head_sequence()))
    }
}
impl BudgetStateEvidence for ProtocolBudgetState<super::accounting::CommittedBudgetEvidence> {
    fn verified_record(
        &self,
        _verifier: &EvidenceAuthority,
    ) -> Result<(ProtocolBudgetRecord, u64), BudgetCreationError> {
        Ok((self.record().clone(), self.committed_height()))
    }
}
/// Converts a public expiry in seconds to the core millisecond clock.
///
/// # Errors
///
/// Returns `InvalidLimit` for a zero expiry or one whose milliseconds overflow `u64`.
pub fn core_expiry_ms(seconds: u64) -> Result<u64, BudgetCreationError> {
    match seconds.checked_mul(1000) {
        Some(milliseconds) if milliseconds != 0 => Ok(milliseconds),
        _ => Err(BudgetCreationError::InvalidLimit),
    }
}

/// Protocol budget confirmed by a layerxpay log and resulting committed state.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProtocolBudget {
    object_id: [u8; 32],
    kind: BudgetKind,
    receipt_bytes: Vec<u8>,
    enforcement: &'static str,
    record: ProtocolBudgetRecord,
    observed_head_sequence: u64,
}

impl ProtocolBudget {
    #[must_use]
    pub const fn object_id(&self) -> [u8; 32] {
        self.object_id
    }

    #[must_use]
    pub const fn kind(&self) -> BudgetKind {
        self.kind
    }

    #[must_use]
    pub fn receipt_bytes(&self) -> &[u8] {
        &self.receipt_bytes
    }

    #[must_use]
    pub const fn enforcement(&self) -> &'static str {
        self.enforcement
    }

    /// Returns the canonical core record the confirmation decoded.
    #[must_use]
    pub const fn record(&self) -> &ProtocolBudgetRecord {
        &self.record
    }

    /// Returns the committed block height of the confirming state.
    #[must_use]
    pub const fn observed_head_sequence(&self) -> u64 {
        self.observed_head_sequence
    }
}

/// Honest daemon-only limit.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LocalLimit {
    pub tenant: TenantId,
    pub id: [u8; 32],
    pub asset: [u8; 32],
    pub ceiling: u128,
    pub enforcement: &'static str,
    pub bypass_statement: &'static str,
}

impl LocalLimit {
    #[must_use]
    pub fn new(tenant: TenantId, id: [u8; 32], asset: [u8; 32], ceiling: u128) -> Self {
        Self {
            tenant,
            id,
            asset,
            ceiling,
            enforcement: "daemon-enforced",
            bypass_statement: LOCAL_BYPASS_STATEMENT,
        }
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum BudgetCreationError {
    EmptyActivity,
    InvalidLimit,
    ActivityBindingUnavailable,
    ActivityBindingMismatch,
    NotBudgetCreation,
    IdentityMismatch,
    Submission,
    UnverifiedReceipt,
    ReceiptActivityMismatch,
    CoreRejected,
    CreatedBudgetUnconfirmed,
    Expired,
    StaleRevocation,
    BudgetNotLive,
    ContextMismatch,
}

pub fn create_protocol_budget<R: BudgetReceiptEvidence, S>(
    _store: &mut Store,
    request: &BudgetRequest,
    registry: &ModuleRegistry,
    verifier: &EvidenceAuthority,
    pipeline: &mut dyn BudgetPipeline<R, S>,
) -> Result<ProtocolBudget, BudgetCreationError>
where
    ProtocolBudgetState<S>: BudgetStateEvidence,
{
    if request.ceiling == 0 || request.expiry_ms == 0 {
        return Err(BudgetCreationError::InvalidLimit);
    }
    if request.expiry_ms <= request.core_time_ms {
        return Err(BudgetCreationError::Expired);
    }
    if request.canonical_activity.is_empty() {
        return Err(BudgetCreationError::EmptyActivity);
    }
    let submission = request
        .verified_submission
        .as_ref()
        .ok_or(BudgetCreationError::ActivityBindingUnavailable)?;
    if submission.exact_bytes() != request.canonical_activity
        || submission.idempotency_key() != request.request_id
    {
        return Err(BudgetCreationError::ActivityBindingMismatch);
    }
    let requested_identity = budget_create_identity(submission.exact_bytes(), registry)?;
    let prepared = request.create_protocol_budget(&requested_identity)?;
    let expected = prepared
        .encode()
        .map_err(|_| BudgetCreationError::NotBudgetCreation)?;
    let (signed, _) = decode_signed(submission.exact_bytes())
        .map_err(|_| BudgetCreationError::NotBudgetCreation)?;
    if signed.to != prepared.target.address() || signed.data != expected {
        return Err(BudgetCreationError::IdentityMismatch);
    }
    let receipt = pipeline.submit_budget(request)?;
    let receipt = receipt.evidence.layerxpay_receipt()?;
    let identity = budget_create_identity(&(submission.exact_bytes(), receipt), registry)?;
    let executed = committed_receipt(receipt, submission.activity_id())?;
    let state = pipeline.budget_state(identity.budget_id)?;
    let (record, height) = state.verified_record(verifier)?;
    if height != executed.height
        || record.budget_id != identity.budget_id
        || record.budget_account != identity.budget_account
        || record.asset_id != request.asset
        || record.per_period_limit != request.ceiling
        || record.configured_period_limit != request.ceiling
        || record.expiry != request.expiry_ms
        || record.carry_cap != identity.carry_cap
        || record.period_length != identity.period_length_ms
        || record.period_start != identity.period_start_ms
        || record.revocation_sequence != identity.revocation_sequence
        || record.rollover_policy != identity.rollover
        || record.purpose_hash != identity.purpose
        || record.spent_this_period != 0
        || record.carried != 0
        || record.source_account
            != (identity.encoding_version == 2).then_some(identity.source_account)
        || record.closed
        || record.revoked
    {
        return Err(BudgetCreationError::CreatedBudgetUnconfirmed);
    }
    Ok(ProtocolBudget {
        object_id: identity.budget_id,
        kind: request.kind,
        receipt_bytes: executed.receipt.clone(),
        enforcement: PROTOCOL_ENFORCEMENT,
        observed_head_sequence: height,
        record,
    })
}
