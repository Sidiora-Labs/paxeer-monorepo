use std::collections::BTreeMap;

use layerx_programs_runtime::terminal::TerminalAttachment;
use layerx_programs_runtime::OccupancySettlement;
use layerx_proof::program::{
    verify_authorized_program_execution_with_payers, AuthorizedProgramExecutionExpectation,
    OccupancyPayer,
};

use layerx_types::payload::{ModuleId, ModuleRegistry};
use layerx_types::verify::VerificationLevel;
use sha2::{Digest as _, Sha256};

use crate::ops::program::ProgramOperations;
use crate::prepare::EvmPrepared;
use crate::protocol_evidence::VerifiedReceiptEvidence;
use crate::sign::VerifiedSubmission;
use crate::store::{Store, TenantId};
use layerx_proof::receipt::AuthorizedBatch;

use super::program_sources::VerifiedResolvedProgramCharges;
use super::{execution_state, program_lifecycle_settlement as lifecycle};
use super::{ProgramBudgetReservation, ProgramChargeKind};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProgramSettlementError {
    Preparation,
    UnsupportedOperation,
    Receipt,
    ArtifactsUnavailable,
    Terminal,
    SourceSnapshot,
    AccountStatusUnavailable,
    MissingAllocation,
    Allocation,
    UnreservedDebit,
    FeeProvenance,
    Occupancy,
    Arithmetic,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProgramPresentationBudgetError {
    Ownership,
    Binding,
    Missing,
    Lineage,
}

pub struct VerifiedProgramApprovalBudgetRow {
    approval_id: [u8; 32],
    held_digest: [u8; 32],
    budget_id: [u8; 32],
    asset: [u8; 32],
    source: [u8; 32],
    observed_sequence: u64,
    remaining_after_reservations: u128,
    terminal: bool,
    verification: VerificationLevel,
    evidence_digest: [u8; 32],
    receipt_digest: [u8; 32],
    checkpoint_digest: [u8; 32],
    age_sequences: u64,
    maximum_age_sequences: u64,
    proof_digest: [u8; 32],
    proof_bytes: Vec<u8>,
}

impl VerifiedProgramApprovalBudgetRow {
    #[cfg(test)]
    #[expect(
        clippy::too_many_arguments,
        reason = "the shared test constructor preserves its call sites outside this task's scope"
    )]
    pub(crate) fn for_test(
        approval_id: [u8; 32],
        held_digest: [u8; 32],
        budget_id: [u8; 32],
        asset: [u8; 32],
        source: [u8; 32],
        observed_sequence: u64,
        remaining_after_reservations: u128,
        terminal: bool,
        verification: VerificationLevel,
        evidence_digest: [u8; 32],
        receipt_digest: [u8; 32],
        checkpoint_digest: [u8; 32],
        age_sequences: u64,
        maximum_age_sequences: u64,
        proof_digest: [u8; 32],
        proof_bytes: Vec<u8>,
    ) -> Self {
        Self {
            approval_id,
            held_digest,
            budget_id,
            asset,
            source,
            observed_sequence,
            remaining_after_reservations,
            terminal,
            verification,
            evidence_digest,
            receipt_digest,
            checkpoint_digest,
            age_sequences,
            maximum_age_sequences,
            proof_digest,
            proof_bytes,
        }
    }

    #[must_use]
    pub const fn approval_id(&self) -> [u8; 32] {
        self.approval_id
    }
    #[must_use]
    pub const fn held_digest(&self) -> [u8; 32] {
        self.held_digest
    }
    #[must_use]
    pub const fn budget_id(&self) -> [u8; 32] {
        self.budget_id
    }
    #[must_use]
    pub const fn asset(&self) -> [u8; 32] {
        self.asset
    }
    #[must_use]
    pub const fn source(&self) -> [u8; 32] {
        self.source
    }
    #[must_use]
    pub const fn observed_sequence(&self) -> u64 {
        self.observed_sequence
    }
    #[must_use]
    pub const fn remaining_after_reservations(&self) -> u128 {
        self.remaining_after_reservations
    }
    #[must_use]
    pub const fn terminal(&self) -> bool {
        self.terminal
    }
    #[must_use]
    pub const fn verification(&self) -> VerificationLevel {
        self.verification
    }
    #[must_use]
    pub const fn evidence_digest(&self) -> [u8; 32] {
        self.evidence_digest
    }
    #[must_use]
    pub const fn receipt_digest(&self) -> [u8; 32] {
        self.receipt_digest
    }
    #[must_use]
    pub const fn checkpoint_digest(&self) -> [u8; 32] {
        self.checkpoint_digest
    }
    #[must_use]
    pub const fn age_sequences(&self) -> u64 {
        self.age_sequences
    }
    #[must_use]
    pub const fn maximum_age_sequences(&self) -> u64 {
        self.maximum_age_sequences
    }
    #[must_use]
    pub const fn proof_digest(&self) -> [u8; 32] {
        self.proof_digest
    }
    #[must_use]
    pub fn verified_proof_bytes(&self) -> &[u8] {
        &self.proof_bytes
    }
}

/// # Errors
/// Returns an error if the owned approval budget or its committed source evidence cannot be
/// validated or read.
#[allow(
    clippy::too_many_arguments,
    reason = "Preserve the public API signature used by daemon consumers."
)]
pub fn read_owned_program_budget(
    store: &Store,
    peer: &crate::human::HumanPeer,
    id: [u8; 32],
    held_digest: [u8; 32],
    registry: &ModuleRegistry,
    budgets: &super::BudgetLimiter,
    proof: &super::budget_proof::VerifiedBudgetProof,
    expected_sequence: u64,
) -> Result<VerifiedProgramApprovalBudgetRow, ProgramPresentationBudgetError> {
    use crate::approval::native_program_presentation::read_owned;
    use crate::prepare::DurablePreparation;
    let held = read_owned(store, peer, id, registry)
        .map_err(|_| ProgramPresentationBudgetError::Ownership)?;
    if held.held_digest() != held_digest {
        return Err(ProgramPresentationBudgetError::Binding);
    }
    let tenant = TenantId::new(peer.tenant.clone())
        .map_err(|_| ProgramPresentationBudgetError::Ownership)?;
    let owners = crate::managed_agent::budget_owners(store, &tenant)
        .map_err(|_| ProgramPresentationBudgetError::Ownership)?;
    let actor =
        std::str::from_utf8(held.actor()).map_err(|_| ProgramPresentationBudgetError::Ownership)?;
    let mut selected = owners.iter().filter(|owner| owner.agent_did == actor);
    let (Some(owner), None) = (selected.next(), selected.next()) else {
        return Err(ProgramPresentationBudgetError::Ownership);
    };
    if owner.active_budget_id != proof.budget_id() || owner.agent_did != proof.owner() {
        return Err(ProgramPresentationBudgetError::Binding);
    }
    super::program_sources::bind_program_presentation_budget_proof(
        held.reservation(),
        held.actor(),
        proof,
        expected_sequence,
    )
    .map_err(|_| ProgramPresentationBudgetError::Binding)?;
    let key = DurablePreparation::store_key(&tenant, id)
        .map_err(|_| ProgramPresentationBudgetError::Missing)?;
    let raw = store
        .get(&key)
        .ok_or(ProgramPresentationBudgetError::Missing)?;
    if raw.class() != crate::store::StorageClass::LocalOnly {
        return Err(ProgramPresentationBudgetError::Binding);
    }
    let durable = DurablePreparation::decode(tenant, raw.bytes())
        .map_err(|_| ProgramPresentationBudgetError::Binding)?;
    let encoded = held
        .reservation()
        .encode()
        .map_err(|_| ProgramPresentationBudgetError::Binding)?;
    if durable.preparation_id != id
        || durable.extensions.get(&6) != Some(&encoded)
        || durable.extensions.get(&7).map(Vec::as_slice) != Some(held_digest.as_slice())
    {
        return Err(ProgramPresentationBudgetError::Binding);
    }
    let terminal = durable.terminal();
    if !terminal && held.reservation().expiry_sequence <= expected_sequence {
        return Err(ProgramPresentationBudgetError::Binding);
    }
    let remaining = budgets
        .remaining_after_allocation_bound(
            held.reservation(),
            proof.asset(),
            proof.source_account(),
            proof.remaining(),
            terminal,
        )
        .map_err(|_| ProgramPresentationBudgetError::Lineage)?;
    Ok(VerifiedProgramApprovalBudgetRow {
        approval_id: id,
        held_digest,
        budget_id: proof.budget_id(),
        asset: proof.asset(),
        source: proof.source_account(),
        observed_sequence: expected_sequence,
        remaining_after_reservations: remaining,
        terminal,
        verification: proof.verification(),
        evidence_digest: proof.evidence_digest(),
        receipt_digest: proof.receipt_digest(),
        checkpoint_digest: proof.checkpoint_digest(),
        age_sequences: proof.age_sequences(),
        maximum_age_sequences: proof.maximum_age_sequences(),
        proof_digest: proof.digest(),
        proof_bytes: proof.canonical_export_bytes().to_vec(),
    })
}

/// # Errors
/// Returns an error if the owned approval budget or its committed source evidence cannot be
/// validated or read.
#[allow(
    clippy::too_many_arguments,
    reason = "Preserve the public API signature used by daemon consumers."
)]
pub fn read_owned_native_effect_budget(
    store: &Store,
    peer: &crate::human::HumanPeer,
    id: [u8; 32],
    held_digest: [u8; 32],
    registry: &ModuleRegistry,
    budgets: &super::BudgetLimiter,
    proof: &super::budget_proof::VerifiedBudgetProof,
    expected_sequence: u64,
) -> Result<VerifiedProgramApprovalBudgetRow, ProgramPresentationBudgetError> {
    use crate::approval::native_effect::{NativeEffectApprovalCarrier, DURABLE_EXTENSION};
    use crate::prepare::DurablePreparation;
    let held = NativeEffectApprovalCarrier::read_for_human(store, peer, id)
        .map_err(|_| ProgramPresentationBudgetError::Ownership)?;
    if held
        .held_digest()
        .map_err(|_| ProgramPresentationBudgetError::Binding)?
        != held_digest
    {
        return Err(ProgramPresentationBudgetError::Binding);
    }
    let prepared = held
        .restore_prepared(registry)
        .map_err(|_| ProgramPresentationBudgetError::Binding)?;
    let tenant = TenantId::new(peer.tenant.clone())
        .map_err(|_| ProgramPresentationBudgetError::Ownership)?;
    let owners = crate::managed_agent::budget_owners(store, &tenant)
        .map_err(|_| ProgramPresentationBudgetError::Ownership)?;
    let actor =
        std::str::from_utf8(held.actor()).map_err(|_| ProgramPresentationBudgetError::Ownership)?;
    let mut selected = owners.iter().filter(|owner| owner.agent_did == actor);
    let (Some(owner), None) = (selected.next(), selected.next()) else {
        return Err(ProgramPresentationBudgetError::Ownership);
    };
    if owner.active_budget_id != proof.budget_id()
        || owner.agent_did != proof.owner()
        || prepared.disclosure.actor.as_slice() != prepared.from
    {
        return Err(ProgramPresentationBudgetError::Binding);
    }
    let reservation = held
        .budget()
        .map_err(|_| ProgramPresentationBudgetError::Binding)?;
    super::program_sources::bind_program_presentation_budget_proof(
        &reservation,
        held.actor(),
        proof,
        expected_sequence,
    )
    .map_err(|_| ProgramPresentationBudgetError::Binding)?;
    let key = DurablePreparation::store_key(&tenant, id)
        .map_err(|_| ProgramPresentationBudgetError::Missing)?;
    let raw = store
        .get(&key)
        .ok_or(ProgramPresentationBudgetError::Missing)?;
    if raw.class() != crate::store::StorageClass::LocalOnly {
        return Err(ProgramPresentationBudgetError::Binding);
    }
    let durable = DurablePreparation::decode(tenant, raw.bytes())
        .map_err(|_| ProgramPresentationBudgetError::Binding)?;
    if durable.preparation_id != id
        || durable.extensions.get(&6)
            != Some(
                &reservation
                    .encode()
                    .map_err(|_| ProgramPresentationBudgetError::Binding)?,
            )
        || durable
            .extensions
            .get(&DURABLE_EXTENSION)
            .map(Vec::as_slice)
            != Some(held_digest.as_slice())
    {
        return Err(ProgramPresentationBudgetError::Binding);
    }
    let terminal = durable.terminal();
    if !terminal && reservation.expiry_sequence <= expected_sequence {
        return Err(ProgramPresentationBudgetError::Binding);
    }
    let remaining = budgets
        .remaining_after_allocation_bound(
            &reservation,
            proof.asset(),
            proof.source_account(),
            proof.remaining(),
            terminal,
        )
        .map_err(|_| ProgramPresentationBudgetError::Lineage)?;
    Ok(VerifiedProgramApprovalBudgetRow {
        approval_id: id,
        held_digest,
        budget_id: proof.budget_id(),
        asset: proof.asset(),
        source: proof.source_account(),
        observed_sequence: expected_sequence,
        remaining_after_reservations: remaining,
        terminal,
        verification: proof.verification(),
        evidence_digest: proof.evidence_digest(),
        receipt_digest: proof.receipt_digest(),
        checkpoint_digest: proof.checkpoint_digest(),
        age_sequences: proof.age_sequences(),
        maximum_age_sequences: proof.maximum_age_sequences(),
        proof_digest: proof.digest(),
        proof_bytes: proof.canonical_export_bytes().to_vec(),
    })
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramExecutedDebit {
    pub kind: ProgramChargeKind,
    pub source: [u8; 32],
    pub asset: [u8; 32],
    pub destination: Option<[u8; 32]>,
    pub actual_amount: u128,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedProgramDebitSettlement {
    reservation_id: [u8; 32],
    reservation_digest: [u8; 32],
    terminal_receipt: [u8; 32],
    activity_id: [u8; 32],
    global_sequence: u64,
    debits: Vec<ProgramExecutedDebit>,
}

impl VerifiedProgramDebitSettlement {
    pub(super) fn from_native(
        reservation: &ProgramBudgetReservation,
        receipt: &[u8],
        activity_id: [u8; 32],
        global_sequence: u64,
        debits: Vec<ProgramExecutedDebit>,
    ) -> Result<Self, ProgramSettlementError> {
        Ok(Self {
            reservation_id: reservation.id,
            reservation_digest: reservation
                .settlement_binding()
                .map_err(|_| ProgramSettlementError::Allocation)?,
            terminal_receipt: Sha256::digest(receipt).into(),
            activity_id,
            global_sequence,
            debits,
        })
    }

    #[must_use]
    pub const fn reservation_id(&self) -> [u8; 32] {
        self.reservation_id
    }
    #[must_use]
    pub const fn reservation_digest(&self) -> [u8; 32] {
        self.reservation_digest
    }
    #[must_use]
    pub const fn terminal_receipt(&self) -> [u8; 32] {
        self.terminal_receipt
    }
    #[must_use]
    pub const fn activity_id(&self) -> [u8; 32] {
        self.activity_id
    }
    #[must_use]
    pub const fn global_sequence(&self) -> u64 {
        self.global_sequence
    }
    #[must_use]
    pub fn debits(&self) -> &[ProgramExecutedDebit] {
        &self.debits
    }
}

type DebitKey = ([u8; 32], [u8; 32], ProgramChargeKind, Option<[u8; 32]>);

/// Reads the node's execution observation for an exact native preparation.
/// An unknown transaction cannot supply receipt evidence or release a reservation.
/// # Errors
/// Returns an error if the preparation, receipt, source snapshot or retained allocation does
/// not prove the requested settlement.
pub fn read_native_execution_observation(
    node: &layerx_client::sequencer::SequencerEndpoint,
    prepared: &crate::prepare::EvmPrepared,
    submission: &VerifiedSubmission,
) -> Result<layerx_client::sequencer::SequencerReceipt, ProgramSettlementError> {
    super::native_binding::verify_retained_submission(prepared, submission)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    read_signed_native_execution_observation(node, submission.exact_bytes())
}

/// Reads execution by the native transaction hash without requiring submission
/// admission. This observation alone never authorizes debit settlement.
/// # Errors
/// Returns an error if the preparation, receipt, source snapshot or retained allocation does
/// not prove the requested settlement.
pub fn read_signed_native_execution_observation(
    node: &layerx_client::sequencer::SequencerEndpoint,
    signed_transaction: &[u8],
) -> Result<layerx_client::sequencer::SequencerReceipt, ProgramSettlementError> {
    use layerx_client::sequencer::SequencerReceipt;
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
    let (call, signature) =
        decode_signed(signed_transaction).map_err(|_| ProgramSettlementError::Preparation)?;
    if call.chain_id == 0
        || call.gas_limit == 0
        || call.max_priority_fee_per_gas > call.max_fee_per_gas
    {
        return Err(ProgramSettlementError::Preparation);
    }
    let sender =
        recover_sender(&call, &signature).map_err(|_| ProgramSettlementError::Preparation)?;
    let observation = node
        .sequencer_receipt(&transaction_hash(signed_transaction))
        .map_err(|_| ProgramSettlementError::Receipt)?
        .ok_or(ProgramSettlementError::Receipt)?;
    let executed = match &observation {
        SequencerReceipt::Receipted(executed)
        | SequencerReceipt::Committed {
            receipt: executed, ..
        } => executed,
        SequencerReceipt::Submitted { .. } => return Err(ProgramSettlementError::Receipt),
    };
    if executed.from != sender || executed.to != call.to || executed.nonce != call.nonce {
        return Err(ProgramSettlementError::Receipt);
    }
    Ok(observation)
}

/// # Errors
/// Returns an error if the preparation, receipt, source snapshot or retained allocation does
/// not prove the requested settlement.
#[allow(
    clippy::too_many_arguments,
    reason = "Preserve the public API signature used by daemon consumers."
)]
pub fn read_program_debit_settlement(
    programs: &ProgramOperations,
    registry: &ModuleRegistry,
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
    receipt: &VerifiedReceiptEvidence,
    authority: &AuthorizedBatch,
    sources: &VerifiedResolvedProgramCharges,
    reservation: &ProgramBudgetReservation,
) -> Result<VerifiedProgramDebitSettlement, ProgramSettlementError> {
    if !sources.matches_prepared(prepared)
        || reservation.allocation_state_root() != Some(sources.state_root())
        || reservation.allocation_sequence() != Some(sources.global_sequence())
        || reservation.allocation_actor() != Some(sources.actor())
    {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    let mut expected = BTreeMap::<DebitKey, u128>::new();
    for row in sources.charges() {
        add(
            &mut expected,
            (
                row.asset,
                row.source.account(),
                row.source.kind(),
                row.destination,
            ),
            row.maximum_amount,
        )?;
    }
    if retained_allocations(reservation)? != expected {
        return Err(ProgramSettlementError::Allocation);
    }
    verify_program_debit_settlement(
        programs,
        registry,
        prepared,
        submission,
        &SettlementReceipt { receipt, authority },
        reservation,
        sources.owner_did(),
    )
}

/// # Errors
/// Returns an error if the preparation, receipt, source snapshot or retained allocation does
/// not prove the requested settlement.
#[allow(
    clippy::too_many_arguments,
    reason = "Preserve the public API signature used by daemon consumers."
)]
pub fn read_retained_program_debit_settlement(
    programs: &ProgramOperations,
    registry: &ModuleRegistry,
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
    receipt: &VerifiedReceiptEvidence,
    authority: &AuthorizedBatch,
    store: &Store,
    tenant: &TenantId,
) -> Result<(ProgramBudgetReservation, VerifiedProgramDebitSettlement), ProgramSettlementError> {
    read_retained_program_debit_settlement_inner(
        programs,
        registry,
        prepared,
        submission,
        receipt,
        authority,
        &SettlementStore { store, tenant },
    )
}

/// # Errors
/// Returns an error if the preparation, receipt, source snapshot or retained allocation does
/// not prove the requested settlement.
pub fn read_retained_wind_down_debit_settlement_at_execution(
    registry: &ModuleRegistry,
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
    receipt: &VerifiedReceiptEvidence,
    store: &Store,
    tenant: &TenantId,
    prestate: &execution_state::CommittedExecutionState,
) -> Result<(ProgramBudgetReservation, VerifiedProgramDebitSettlement), ProgramSettlementError> {
    use crate::prepare::{DurablePreparation, LifecycleState};
    let id: [u8; 32] = Sha256::digest(prepared.call.encode_unsigned()).into();
    let key = DurablePreparation::store_key(tenant, id)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    let stored = store
        .get(&key)
        .ok_or(ProgramSettlementError::MissingAllocation)?;
    if stored.class() != crate::store::StorageClass::LocalOnly {
        return Err(ProgramSettlementError::Preparation);
    }
    let durable = DurablePreparation::decode(tenant.clone(), stored.bytes())
        .map_err(|_| ProgramSettlementError::Preparation)?;
    if durable.preparation_id != id
        || durable.payload_hash
            != layerx_crypto::evm_transaction::transaction_hash(&prepared.call.data)
        || durable.activity_id != Some(submission.activity_id())
        || !durable.holds.is_empty()
        || !matches!(
            durable.state,
            LifecycleState::Signed
                | LifecycleState::Submitted
                | LifecycleState::Acknowledged
                | LifecycleState::Unknown
        )
        || durable
            .signed_bytes()
            .map_err(|_| ProgramSettlementError::Preparation)?
            .as_deref()
            != Some(submission.exact_bytes())
    {
        return Err(ProgramSettlementError::Preparation);
    }
    let encoded = durable
        .extensions
        .get(&6)
        .ok_or(ProgramSettlementError::MissingAllocation)?;
    let reservation = ProgramBudgetReservation::decode(encoded)
        .map_err(|_| ProgramSettlementError::Allocation)?;
    if reservation.id != id {
        return Err(ProgramSettlementError::Allocation);
    }
    let carrier =
        crate::approval::native_program::NativeProgramApprovalCarrier::retained_for_tenant(
            store, tenant,
        )
        .map_err(|_| ProgramSettlementError::Preparation)?
        .into_iter()
        .find(|carrier| carrier.preparation_id() == id)
        .ok_or(ProgramSettlementError::Preparation)?;
    let restored = carrier
        .restore_prepared(registry)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    if restored.call.encode_unsigned() != prepared.call.encode_unsigned()
        || restored.observed_head_height != prepared.observed_head_height
        || restored.disclosure.authority != prepared.disclosure.authority
        || carrier
            .budget()
            .map_err(|_| ProgramSettlementError::Allocation)?
            != reservation
    {
        return Err(ProgramSettlementError::Preparation);
    }
    let debits = lifecycle::verify_wind_down_debits(
        prepared,
        submission,
        receipt,
        registry,
        prestate,
        &reservation,
        &layerx_types::ids::Did::new(carrier.retained_actor())
            .map_err(|_| ProgramSettlementError::Preparation)?,
    )?;
    let witness = VerifiedProgramDebitSettlement {
        reservation_id: reservation.id,
        reservation_digest: reservation
            .settlement_binding()
            .map_err(|_| ProgramSettlementError::Allocation)?,
        terminal_receipt: receipt.receipt_ref(),
        activity_id: submission.activity_id(),
        global_sequence: receipt.global_sequence(),
        debits,
    };
    Ok((reservation, witness))
}

/// # Errors
/// Returns an error if the preparation, receipt, source snapshot or retained allocation does
/// not prove the requested settlement.
pub fn read_retained_lifecycle_debit_settlement_at_execution(
    registry: &ModuleRegistry,
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
    receipt: &VerifiedReceiptEvidence,
    store: &Store,
    tenant: &TenantId,
    prestate: &execution_state::CommittedExecutionState,
) -> Result<(ProgramBudgetReservation, VerifiedProgramDebitSettlement), ProgramSettlementError> {
    use crate::prepare::{DurablePreparation, LifecycleState};
    let id: [u8; 32] = Sha256::digest(prepared.call.encode_unsigned()).into();
    let key = DurablePreparation::store_key(tenant, id)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    let stored = store
        .get(&key)
        .ok_or(ProgramSettlementError::MissingAllocation)?;
    if stored.class() != crate::store::StorageClass::LocalOnly {
        return Err(ProgramSettlementError::Preparation);
    }
    let durable = DurablePreparation::decode(tenant.clone(), stored.bytes())
        .map_err(|_| ProgramSettlementError::Preparation)?;
    if durable.preparation_id != id
        || durable.payload_hash
            != layerx_crypto::evm_transaction::transaction_hash(&prepared.call.data)
        || durable.activity_id != Some(submission.activity_id())
        || !durable.holds.is_empty()
        || !matches!(
            durable.state,
            LifecycleState::Signed
                | LifecycleState::Submitted
                | LifecycleState::Acknowledged
                | LifecycleState::Unknown
        )
        || durable
            .signed_bytes()
            .map_err(|_| ProgramSettlementError::Preparation)?
            .as_deref()
            != Some(submission.exact_bytes())
    {
        return Err(ProgramSettlementError::Preparation);
    }
    let encoded = durable
        .extensions
        .get(&6)
        .ok_or(ProgramSettlementError::MissingAllocation)?;
    let reservation = ProgramBudgetReservation::decode(encoded)
        .map_err(|_| ProgramSettlementError::Allocation)?;
    if reservation.id != id {
        return Err(ProgramSettlementError::Allocation);
    }
    let carrier =
        crate::approval::native_program::NativeProgramApprovalCarrier::retained_for_tenant(
            store, tenant,
        )
        .map_err(|_| ProgramSettlementError::Preparation)?
        .into_iter()
        .find(|carrier| carrier.preparation_id() == id)
        .ok_or(ProgramSettlementError::Preparation)?;
    let restored = carrier
        .restore_prepared(registry)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    if restored.call.encode_unsigned() != prepared.call.encode_unsigned()
        || restored.observed_head_height != prepared.observed_head_height
        || restored.disclosure.authority != prepared.disclosure.authority
        || carrier
            .budget()
            .map_err(|_| ProgramSettlementError::Allocation)?
            != reservation
    {
        return Err(ProgramSettlementError::Preparation);
    }
    let debits = lifecycle::verify_remaining_lifecycle_debits(
        prepared,
        submission,
        receipt,
        registry,
        prestate,
        &reservation,
        &layerx_types::ids::Did::new(carrier.retained_actor())
            .map_err(|_| ProgramSettlementError::Preparation)?,
    )?;
    let witness = VerifiedProgramDebitSettlement {
        reservation_id: reservation.id,
        reservation_digest: reservation
            .settlement_binding()
            .map_err(|_| ProgramSettlementError::Allocation)?,
        terminal_receipt: receipt.receipt_ref(),
        activity_id: submission.activity_id(),
        global_sequence: receipt.global_sequence(),
        debits,
    };
    Ok((reservation, witness))
}

fn read_retained_program_debit_settlement_inner(
    programs: &ProgramOperations,
    registry: &ModuleRegistry,
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
    receipt: &VerifiedReceiptEvidence,
    authority: &AuthorizedBatch,
    input: &SettlementStore<'_>,
) -> Result<(ProgramBudgetReservation, VerifiedProgramDebitSettlement), ProgramSettlementError> {
    use crate::prepare::{DurablePreparation, LifecycleState};

    let SettlementStore { store, tenant } = *input;

    let id: [u8; 32] = Sha256::digest(prepared.call.encode_unsigned()).into();
    let key = DurablePreparation::store_key(tenant, id)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    let stored = store
        .get(&key)
        .ok_or(ProgramSettlementError::MissingAllocation)?;
    if stored.class() != crate::store::StorageClass::LocalOnly {
        return Err(ProgramSettlementError::Preparation);
    }
    let durable = DurablePreparation::decode(tenant.clone(), stored.bytes())
        .map_err(|_| ProgramSettlementError::Preparation)?;
    if durable.preparation_id != id
        || durable.payload_hash
            != layerx_crypto::evm_transaction::transaction_hash(&prepared.call.data)
        || durable.activity_id != Some(submission.activity_id())
        || !durable.holds.is_empty()
        || !matches!(
            durable.state,
            LifecycleState::Signed
                | LifecycleState::Submitted
                | LifecycleState::Acknowledged
                | LifecycleState::Unknown
        )
        || durable
            .signed_bytes()
            .map_err(|_| ProgramSettlementError::Preparation)?
            .as_deref()
            != Some(submission.exact_bytes())
    {
        return Err(ProgramSettlementError::Preparation);
    }
    let encoded = durable
        .extensions
        .get(&6)
        .ok_or(ProgramSettlementError::MissingAllocation)?;
    let reservation = ProgramBudgetReservation::decode(encoded)
        .map_err(|_| ProgramSettlementError::Allocation)?;
    if reservation.id != id {
        return Err(ProgramSettlementError::Allocation);
    }
    let carrier =
        crate::approval::native_program::NativeProgramApprovalCarrier::retained_for_tenant(
            store, tenant,
        )
        .map_err(|_| ProgramSettlementError::Preparation)?
        .into_iter()
        .find(|carrier| carrier.preparation_id() == id)
        .ok_or(ProgramSettlementError::Preparation)?;
    let restored = carrier
        .restore_prepared(registry)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    if restored.call.encode_unsigned() != prepared.call.encode_unsigned()
        || restored.observed_head_height != prepared.observed_head_height
        || restored.disclosure.authority != prepared.disclosure.authority
        || carrier
            .budget()
            .map_err(|_| ProgramSettlementError::Allocation)?
            != reservation
    {
        return Err(ProgramSettlementError::Preparation);
    }
    let witness = verify_program_debit_settlement(
        programs,
        registry,
        prepared,
        submission,
        &SettlementReceipt { receipt, authority },
        &reservation,
        &layerx_types::ids::Did::new(carrier.retained_actor())
            .map_err(|_| ProgramSettlementError::Preparation)?,
    )?;
    Ok((reservation, witness))
}

/// # Errors
/// Returns an error if the preparation, receipt, source snapshot or retained allocation does
/// not prove the requested settlement.
pub fn read_retained_native_effect_debit_settlement(
    registry: &ModuleRegistry,
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
    receipt: &VerifiedReceiptEvidence,
    authority: &AuthorizedBatch,
    store: &Store,
    tenant: &TenantId,
) -> Result<(ProgramBudgetReservation, VerifiedProgramDebitSettlement), ProgramSettlementError> {
    read_retained_native_effect_debit_settlement_inner(
        registry, prepared, submission, receipt, authority, store, tenant,
    )
}

fn read_retained_native_effect_debit_settlement_inner(
    registry: &ModuleRegistry,
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
    receipt: &VerifiedReceiptEvidence,
    authority: &AuthorizedBatch,
    store: &Store,
    tenant: &TenantId,
) -> Result<(ProgramBudgetReservation, VerifiedProgramDebitSettlement), ProgramSettlementError> {
    use crate::prepare::{DurablePreparation, LifecycleState};
    super::native_binding::verify_native_budget_submission(prepared, submission)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    crate::reject_if(
        prepared.disclosure.activity_type.module() != ModuleId::Asset,
        || ProgramSettlementError::UnsupportedOperation,
    )?;
    let id: [u8; 32] = Sha256::digest(prepared.call.encode_unsigned()).into();
    let carrier =
        crate::approval::native_effect::NativeEffectApprovalCarrier::read(store, tenant, id)
            .map_err(|_| ProgramSettlementError::Preparation)?;
    let restored = carrier
        .restore_prepared(registry)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    crate::reject_if(
        restored.call.encode_unsigned() != prepared.call.encode_unsigned()
            || restored.disclosure_digest != prepared.disclosure_digest,
        || ProgramSettlementError::Preparation,
    )?;
    let key = DurablePreparation::store_key(tenant, id)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    let raw = store
        .get(&key)
        .ok_or(ProgramSettlementError::MissingAllocation)?;
    let durable = DurablePreparation::decode(tenant.clone(), raw.bytes())
        .map_err(|_| ProgramSettlementError::Preparation)?;
    crate::reject_if(
        raw.class() != crate::store::StorageClass::LocalOnly
            || durable.activity_id != Some(submission.activity_id())
            || !matches!(
                durable.state,
                LifecycleState::Signed
                    | LifecycleState::Submitted
                    | LifecycleState::Acknowledged
                    | LifecycleState::Unknown
            )
            || durable
                .signed_bytes()
                .map_err(|_| ProgramSettlementError::Preparation)?
                .as_deref()
                != Some(submission.exact_bytes()),
        || ProgramSettlementError::Preparation,
    )?;
    let reservation = carrier
        .budget()
        .map_err(|_| ProgramSettlementError::Allocation)?;
    let protocol = receipt
        .execution_receipt()
        .receipt()
        .protocol()
        .ok_or(ProgramSettlementError::Receipt)?;
    crate::reject_if(
        receipt.level() < VerificationLevel::BATCH_INCLUDED
            || receipt.activity_id() != submission.activity_id()
            || protocol.activity_id() != submission.activity_id()
            || protocol.protocol_version() != 3
            || protocol.module_id() != 1
            || protocol.global_sequence() != receipt.global_sequence()
            || protocol.batch_id() != authority.batch_id()
            || protocol.program_outcome().is_some(),
        || ProgramSettlementError::Receipt,
    )?;
    crate::reject_if(
        reservation.allocation_state_root() != Some(protocol.previous_state_root())
            || prepared.observed_head_height.checked_add(1) != Some(protocol.global_sequence()),
        || ProgramSettlementError::SourceSnapshot,
    )?;
    let profile = carrier
        .profile()
        .map_err(|_| ProgramSettlementError::Preparation)?;
    let plan = crate::capability::derive_native_profile_effects(&prepared.disclosure, profile)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    let transfer = native_effect_transfer(prepared, profile, &plan)?;
    let debits = native_effect_debits(
        prepared,
        &reservation,
        receipt,
        transfer,
        carrier.fee_asset(),
    )?;
    let witness = VerifiedProgramDebitSettlement {
        reservation_id: id,
        reservation_digest: reservation
            .settlement_binding()
            .map_err(|_| ProgramSettlementError::Allocation)?,
        terminal_receipt: Sha256::digest(receipt.canonical_receipt()).into(),
        activity_id: submission.activity_id(),
        global_sequence: receipt.global_sequence(),
        debits,
    };
    Ok((reservation, witness))
}

fn retained_allocations(
    reservation: &ProgramBudgetReservation,
) -> Result<BTreeMap<DebitKey, u128>, ProgramSettlementError> {
    let mut retained = BTreeMap::<DebitKey, u128>::new();
    for row in reservation
        .allocations()
        .ok_or(ProgramSettlementError::MissingAllocation)?
    {
        add(
            &mut retained,
            (row.asset, row.source, row.kind, row.destination),
            row.maximum_amount,
        )?;
    }
    Ok(retained)
}

/// Terminal bytes and call graph emitted by the native execution producer.
/// All fields are authenticated against the receipt before any debit is returned.
pub struct NativeProgramArtifacts {
    pub terminal_payload: Vec<u8>,
    pub call_graph: Vec<u8>,
}

fn verify_program_debit_settlement(
    programs: &ProgramOperations,
    registry: &ModuleRegistry,
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
    input: &SettlementReceipt<'_>,
    reservation: &ProgramBudgetReservation,
    owner: &layerx_types::ids::Did,
) -> Result<VerifiedProgramDebitSettlement, ProgramSettlementError> {
    let SettlementReceipt { receipt, authority } = *input;

    super::native_binding::verify_native_budget_submission(prepared, submission)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    let artifacts = programs
        .activity_execution(
            registry,
            submission.exact_bytes(),
            receipt.canonical_receipt(),
            *authority,
        )
        .map_err(|_| ProgramSettlementError::ArtifactsUnavailable)?;
    verify_native_program_debit_settlement(
        registry,
        prepared,
        submission,
        receipt,
        authority,
        reservation,
        &NativeProgramArtifacts {
            terminal_payload: artifacts.terminal_payload,
            call_graph: artifacts.execution.call_graph().to_vec(),
        },
        owner,
    )
}

/// Settles a native program call from its bound disclosure and authenticated artifacts.
/// # Errors
/// Returns an error if the preparation, receipt, source snapshot or retained allocation does
/// not prove the requested settlement.
#[allow(
    clippy::too_many_arguments,
    reason = "Preserve the public API signature used by daemon consumers."
)]
pub fn verify_native_program_debit_settlement(
    registry: &ModuleRegistry,
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
    receipt: &VerifiedReceiptEvidence,
    authority: &AuthorizedBatch,
    reservation: &ProgramBudgetReservation,
    artifacts: &NativeProgramArtifacts,
    owner: &layerx_types::ids::Did,
) -> Result<VerifiedProgramDebitSettlement, ProgramSettlementError> {
    super::native_binding::verify_native_budget_submission(prepared, submission)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    crate::reject_if(
        prepared.disclosure.authority.as_slice() != prepared.from,
        || ProgramSettlementError::Preparation,
    )?;
    crate::reject_if(
        prepared.disclosure.activity_type.module() != ModuleId::Programs
            || prepared.disclosure.activity_type.ordinal() != 3,
        || ProgramSettlementError::UnsupportedOperation,
    )?;
    let Some(layerx_crypto::disclosure::DisclosedNativeOperation::ProgramCall(value)) =
        &prepared.disclosure.native_operation
    else {
        return Err(ProgramSettlementError::UnsupportedOperation);
    };
    let call = value.native();
    let canonical_receipt = receipt.canonical_receipt();
    let protocol = receipt
        .execution_receipt()
        .receipt()
        .protocol()
        .ok_or(ProgramSettlementError::Receipt)?;
    crate::reject_if(
        receipt.level() < VerificationLevel::BATCH_INCLUDED
            || receipt.activity_id() != submission.activity_id()
            || protocol.activity_id() != submission.activity_id()
            || protocol.module_id() != 9
            || receipt.global_sequence() != protocol.global_sequence(),
        || ProgramSettlementError::Receipt,
    )?;
    let actor = layerx_wire::hash::did_id_for_protocol(owner, 3)
        .map_err(|_| ProgramSettlementError::Preparation)?;
    reservation
        .validate()
        .map_err(|_| ProgramSettlementError::Allocation)?;
    let preparation_digest: [u8; 32] = Sha256::digest(prepared.call.encode_unsigned()).into();
    crate::reject_if(
        reservation.id != preparation_digest
            || reservation.allocation_sequence() != Some(prepared.observed_head_height)
            || reservation.allocation_actor() != Some(actor)
            || reservation.allocation_preparation_digest() != Some(preparation_digest),
        || ProgramSettlementError::SourceSnapshot,
    )?;
    let expected = retained_allocations(reservation)?;
    crate::reject_if(
        reservation.allocation_state_root() != Some(protocol.previous_state_root())
            || prepared.observed_head_height.checked_add(1) != Some(protocol.global_sequence()),
        || ProgramSettlementError::SourceSnapshot,
    )?;
    let outcome = protocol
        .program_outcome()
        .ok_or(ProgramSettlementError::Terminal)?;
    crate::reject_if(outcome.encoding_version() != 4, || {
        ProgramSettlementError::Terminal
    })?;
    let verified = verify_authorized_program_execution_with_payers(
        canonical_receipt,
        &artifacts.terminal_payload,
        &artifacts.call_graph,
        &AuthorizedProgramExecutionExpectation {
            authority: *authority,
            activity_id: submission.activity_id(),
            payload_hash: layerx_wire::hash::payload_hash_for(
                &layerx_types::payload::Payload::new(
                    registry,
                    prepared.disclosure.activity_type,
                    &call
                        .encode()
                        .map_err(|_| ProgramSettlementError::Preparation)?,
                )
                .map_err(|_| ProgramSettlementError::Preparation)?,
            )
            .map_err(|_| ProgramSettlementError::Preparation)?,
            program_id: call.program_id.bytes(),
            guest_abi_version: call.guest_abi,
        },
        &[OccupancyPayer {
            did: owner.as_bytes(),
            account: None,
        }],
    )
    .map_err(|_| ProgramSettlementError::Terminal)?;
    let debits = settled_program_debits(prepared, receipt, artifacts, &verified, &expected)?;
    Ok(VerifiedProgramDebitSettlement {
        reservation_id: reservation.id,
        reservation_digest: reservation
            .settlement_binding()
            .map_err(|_| ProgramSettlementError::Allocation)?,
        terminal_receipt: Sha256::digest(canonical_receipt).into(),
        activity_id: submission.activity_id(),
        global_sequence: protocol.global_sequence(),
        debits,
    })
}

fn add(
    rows: &mut BTreeMap<DebitKey, u128>,
    key: DebitKey,
    amount: u128,
) -> Result<(), ProgramSettlementError> {
    if amount == 0 {
        return Ok(());
    }
    let total = rows.entry(key).or_default();
    *total = total
        .checked_add(amount)
        .ok_or(ProgramSettlementError::Arithmetic)?;
    Ok(())
}

fn array(bytes: &[u8]) -> Result<[u8; 32], ProgramSettlementError> {
    bytes
        .try_into()
        .map_err(|_| ProgramSettlementError::Terminal)
}

pub(crate) struct SettlementStore<'a> {
    pub(crate) store: &'a Store,
    pub(crate) tenant: &'a TenantId,
}

pub(crate) struct SettlementReceipt<'a> {
    pub(crate) receipt: &'a VerifiedReceiptEvidence,
    pub(crate) authority: &'a AuthorizedBatch,
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum NativeEffectLeg {
    Send,
    Mint,
    Draw,
}

type NativeEffectTransfer = (NativeEffectLeg, ([u8; 32], [u8; 32], [u8; 32], u128));
fn native_effect_transfer(
    prepared: &EvmPrepared,
    profile: crate::capability::NativeAdmissionProfile,
    plan: &crate::capability::SemanticPlan,
) -> Result<Option<NativeEffectTransfer>, ProgramSettlementError> {
    let ordinal = prepared.disclosure.activity_type.ordinal();
    let effect = profile == crate::capability::NativeAdmissionProfile::Effect;
    let transfer = match plan.effects() {
        [crate::capability::Effect::Transfer {
            from,
            to,
            asset,
            amount,
        }] if ordinal == 5 && effect => {
            Some((NativeEffectLeg::Send, (*from, *to, *asset, *amount)))
        }
        [crate::capability::Effect::Issuance {
            account,
            asset,
            amount,
        }] if ordinal == 10 && effect => {
            let from = layerx_wire::hash::asset_issuance_account_id(asset)
                .map_err(|_| ProgramSettlementError::Preparation)?;
            Some((NativeEffectLeg::Mint, (from, *account, *asset, *amount)))
        }
        [crate::capability::Effect::Transfer {
            from,
            to,
            asset,
            amount,
        }, grants @ ..]
            if ordinal == 6
                && effect
                && !grants.is_empty()
                && grants.iter().all(|effect| {
                    matches!(effect, crate::capability::Effect::Authorization { .. })
                })
                && crate::capability::native_grant_draw_covers(
                    &prepared.disclosure,
                    *to,
                    (*from, *to, *asset, *amount),
                ) =>
        {
            Some((NativeEffectLeg::Draw, (*from, *to, *asset, *amount)))
        }
        effects
            if !effects.is_empty()
                && effects.iter().all(|effect| {
                    matches!(effect, crate::capability::Effect::Authorization { .. })
                })
                && match profile {
                    crate::capability::NativeAdmissionProfile::Effect => ordinal == 7,
                    crate::capability::NativeAdmissionProfile::Registration { .. } => ordinal == 1,
                } =>
        {
            None
        }
        _ => return Err(ProgramSettlementError::UnsupportedOperation),
    };
    Ok(transfer)
}

fn native_effect_debits(
    prepared: &EvmPrepared,
    reservation: &ProgramBudgetReservation,
    receipt: &VerifiedReceiptEvidence,
    transfer: Option<NativeEffectTransfer>,
    fee_asset: [u8; 32],
) -> Result<Vec<ProgramExecutedDebit>, ProgramSettlementError> {
    let protocol = receipt
        .execution_receipt()
        .receipt()
        .protocol()
        .ok_or(ProgramSettlementError::Receipt)?;
    let rows = reservation
        .allocations()
        .ok_or(ProgramSettlementError::MissingAllocation)?;
    verify_native_effect_allocations(rows, transfer)?;
    let mut debits = Vec::new();
    if let (0, Some((shape, (from, to, asset, amount)))) = (protocol.result_code(), transfer) {
        crate::reject_if(
            protocol.from() != from
                || protocol.to() != to
                || protocol.asset() != asset
                || protocol.amount() != amount,
            || ProgramSettlementError::Terminal,
        )?;
        let mut leg = Vec::with_capacity(115);
        leg.push(0);
        leg.extend(from);
        leg.extend(to);
        leg.extend(asset);
        leg.extend(amount.to_be_bytes());
        leg.extend(1_u16.to_be_bytes());
        layerx_programs_runtime::transfer::verify_applied_kernel_legs(
            &leg,
            protocol.transfer_set_root(),
        )
        .map_err(|_| ProgramSettlementError::Terminal)?;
        if shape != NativeEffectLeg::Draw {
            debits.push(ProgramExecutedDebit {
                kind: ProgramChargeKind::Principal,
                source: from,
                asset,
                destination: Some(to),
                actual_amount: amount,
            });
        }
    } else {
        crate::reject_if(
            protocol.transfer_set_root() != [0; 32]
                || protocol
                    .effects()
                    .iter()
                    .any(layerx_wire::receipt::Effect::monetary),
            || ProgramSettlementError::Terminal,
        )?;
    }
    crate::reject_if(
        protocol.fee_charged()
            > prepared
                .call
                .max_fee_per_gas
                .checked_mul(u128::from(prepared.call.gas_limit))
                .ok_or(ProgramSettlementError::Preparation)?,
        || ProgramSettlementError::FeeProvenance,
    )?;
    if protocol.fee_charged() != 0 {
        let mut fees = rows.iter().filter(|row| row.kind == ProgramChargeKind::Fee);
        let (Some(fee), None) = (fees.next(), fees.next()) else {
            return Err(ProgramSettlementError::FeeProvenance);
        };
        crate::reject_if(
            fee.asset != fee_asset
                || fee.destination.is_some()
                || protocol.fee_charged() > fee.maximum_amount,
            || ProgramSettlementError::FeeProvenance,
        )?;
        debits.push(ProgramExecutedDebit {
            kind: ProgramChargeKind::Fee,
            source: fee.source,
            asset: fee.asset,
            destination: None,
            actual_amount: protocol.fee_charged(),
        });
    }
    Ok(debits)
}
fn verify_native_effect_allocations(
    rows: &[crate::budget::ProgramBudgetAllocation],
    transfer: Option<NativeEffectTransfer>,
) -> Result<(), ProgramSettlementError> {
    if let Some((NativeEffectLeg::Send | NativeEffectLeg::Mint, (from, to, asset, amount))) =
        transfer
    {
        let principal = rows
            .iter()
            .find(|row| {
                row.kind == ProgramChargeKind::Principal
                    && row.source == from
                    && row.destination == Some(to)
                    && row.asset == asset
            })
            .ok_or(ProgramSettlementError::Allocation)?;
        crate::reject_if(
            principal.maximum_amount != amount
                || rows
                    .iter()
                    .filter(|row| row.kind != ProgramChargeKind::Fee)
                    .count()
                    != 1,
            || ProgramSettlementError::Allocation,
        )?;
    } else {
        crate::reject_if(
            rows.iter().any(|row| row.kind != ProgramChargeKind::Fee),
            || ProgramSettlementError::Allocation,
        )?;
    }

    Ok(())
}

fn settled_program_debits(
    prepared: &EvmPrepared,
    receipt: &VerifiedReceiptEvidence,
    artifacts: &NativeProgramArtifacts,
    verified: &layerx_proof::program::VerifiedProgramExecution,
    expected: &BTreeMap<DebitKey, u128>,
) -> Result<Vec<ProgramExecutedDebit>, ProgramSettlementError> {
    let protocol = receipt
        .execution_receipt()
        .receipt()
        .protocol()
        .ok_or(ProgramSettlementError::Receipt)?;
    let outcome = protocol
        .program_outcome()
        .ok_or(ProgramSettlementError::Terminal)?;
    let mut actual = BTreeMap::<DebitKey, u128>::new();
    if protocol.result_code() == 0 {
        let (_, legs) = layerx_wire::receipt::decode_applied_terminal(&artifacts.terminal_payload)
            .map_err(|_| ProgramSettlementError::Terminal)?;
        layerx_programs_runtime::transfer::verify_applied_kernel_legs(
            legs,
            outcome.transfer_root(),
        )
        .map_err(|_| ProgramSettlementError::Terminal)?;
        for leg in legs.chunks_exact(115) {
            let source = array(&leg[1..33])?;
            let destination = array(&leg[33..65])?;
            let asset = array(&leg[65..97])?;
            let amount = u128::from_be_bytes(
                leg[97..113]
                    .try_into()
                    .map_err(|_| ProgramSettlementError::Terminal)?,
            );
            let mut matching = expected.keys().filter(|(a, s, k, d)| {
                *a == asset
                    && *s == source
                    && *k != ProgramChargeKind::Fee
                    && *d == Some(destination)
            });
            let key = *matching
                .next()
                .ok_or(ProgramSettlementError::UnreservedDebit)?;
            crate::reject_if(matching.next().is_some(), || {
                ProgramSettlementError::UnreservedDebit
            })?;
            add(&mut actual, key, amount)?;
        }
    }
    let fee = protocol.fee_charged();
    crate::reject_if(fee > prepared.disclosure.fee_limit, || {
        ProgramSettlementError::FeeProvenance
    })?;
    if fee != 0 {
        let mut fee_rows = expected.keys().filter(|(_, _, kind, destination)| {
            *kind == ProgramChargeKind::Fee && destination.is_none()
        });
        let key = *fee_rows
            .next()
            .ok_or(ProgramSettlementError::FeeProvenance)?;
        crate::reject_if(
            fee_rows.next().is_some()
                || (protocol.result_code() == 0 && key.0 != outcome.occupancy_asset_id()),
            || ProgramSettlementError::FeeProvenance,
        )?;
        add(&mut actual, key, fee)?;
    }
    verify_program_occupancy(verified, outcome, expected, &mut actual)?;
    let mut debits = Vec::with_capacity(actual.len());
    for ((asset, source, kind, destination), actual_amount) in actual {
        crate::reject_if(
            actual_amount
                > *expected
                    .get(&(asset, source, kind, destination))
                    .ok_or(ProgramSettlementError::UnreservedDebit)?,
            || ProgramSettlementError::UnreservedDebit,
        )?;
        debits.push(ProgramExecutedDebit {
            kind,
            source,
            asset,
            destination,
            actual_amount,
        });
    }
    Ok(debits)
}
fn verify_program_occupancy(
    verified: &layerx_proof::program::VerifiedProgramExecution,
    outcome: &layerx_wire::receipt::ProgramOutcome,
    expected: &BTreeMap<DebitKey, u128>,
    actual: &mut BTreeMap<DebitKey, u128>,
) -> Result<(), ProgramSettlementError> {
    let mut occupancy_seen = false;
    for attachment in &verified.terminal().attachments {
        if let TerminalAttachment::Occupancy(bytes) = attachment {
            crate::reject_if(occupancy_seen, || ProgramSettlementError::Occupancy)?;
            occupancy_seen = true;
            let settlement = OccupancySettlement::canonical_decode(bytes)
                .map_err(|_| ProgramSettlementError::Occupancy)?;
            for (payer, (_, paid, _, _)) in settlement
                .payer_dispositions()
                .map_err(|_| ProgramSettlementError::Occupancy)?
            {
                if paid == 0 {
                    continue;
                }
                let mut accounts = verified
                    .occupancy_payment_accounts()
                    .iter()
                    .filter(|account| {
                        account.payer() == payer
                            && account.asset_id() == outcome.occupancy_asset_id()
                    });
                let account = accounts.next().ok_or(ProgramSettlementError::Occupancy)?;
                crate::reject_if(accounts.next().is_some(), || {
                    ProgramSettlementError::Occupancy
                })?;
                let key = (
                    account.asset_id(),
                    account.account(),
                    ProgramChargeKind::Fee,
                    None,
                );
                crate::reject_if(!expected.contains_key(&key), || {
                    ProgramSettlementError::UnreservedDebit
                })?;
                add(actual, key, paid)?;
            }
        }
    }
    crate::reject_if(
        !occupancy_seen && outcome.occupancy_fee_units() != 0,
        || ProgramSettlementError::Occupancy,
    )?;
    Ok(())
}
