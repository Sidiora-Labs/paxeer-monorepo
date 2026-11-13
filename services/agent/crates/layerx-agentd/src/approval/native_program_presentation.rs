use std::collections::BTreeMap;

use layerx_crypto::disclosure::{
    DisclosedNativeOperation, DisclosedProgramCall, DisclosedProgramDeploy,
    DisclosedProgramUpgrade, DisclosedProgramWindDown,
};
use layerx_types::payload::{ModuleId, ModuleRegistry};
use sha2::{Digest as _, Sha256};

use super::native_program::{CarrierError, NativeApprovalState, NativeProgramApprovalCarrier};
use crate::budget::{ProgramBudgetAllocation, ProgramBudgetReservation, ProgramChargeKind};
use crate::capability::{derive_effects, ProgramValueSource, VerifiedInputs};
use crate::human::HumanPeer;
use crate::store::Store;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProgramPresentationError {
    Missing,
    Binding,
    Corrupt,
    MissingAuthority,
    Unsupported,
}

impl From<CarrierError> for ProgramPresentationError {
    fn from(error: CarrierError) -> Self {
        match error {
            CarrierError::Missing => Self::Missing,
            CarrierError::Binding => Self::Binding,
            CarrierError::Policy | CarrierError::Budget | CarrierError::Corrupt => Self::Corrupt,
        }
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProgramPresentationState {
    Awaiting,
    Granted,
    Rejected,
    Expired,
    Defective,
    NotRequired,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProgramPresentationOperation {
    Deploy(DisclosedProgramDeploy),
    Upgrade(DisclosedProgramUpgrade),
    Call(DisclosedProgramCall),
    WindDown(DisclosedProgramWindDown),
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProgramPresentationSemantics {
    OperationOnly,
    AuthorizedLimits(Vec<ProgramBudgetAllocation>),
}

pub struct OwnedNativeProgramPresentation {
    call: Option<layerx_crypto::evm_transaction::Eip1559Call>,
    id: [u8; 32],
    held_digest: [u8; 32],
    actor: Vec<u8>,
    owner: String,
    state: ProgramPresentationState,
    activity_ordinal: u16,
    canonical_unsigned_bytes: Vec<u8>,
    immutable_carrier_bytes: Vec<u8>,
    reservation: ProgramBudgetReservation,
    created_at_sequence: u64,
    budget_expiry_sequence: u64,
    created_at_unix_seconds: u64,
    activity_expires_at_unix_milliseconds: u64,
    release_ref: Option<[u8; 32]>,
    fee_asset: Option<[u8; 32]>,
    operation: ProgramPresentationOperation,
    semantics: ProgramPresentationSemantics,
}

impl OwnedNativeProgramPresentation {
    #[must_use]
    pub fn call(&self) -> Option<&layerx_crypto::evm_transaction::Eip1559Call> {
        self.call.as_ref()
    }
    #[must_use]
    pub fn id(&self) -> [u8; 32] {
        self.id
    }
    #[must_use]
    pub fn held_digest(&self) -> [u8; 32] {
        self.held_digest
    }
    #[must_use]
    pub fn actor(&self) -> &[u8] {
        &self.actor
    }
    #[must_use]
    pub fn owner(&self) -> &str {
        &self.owner
    }
    #[must_use]
    pub fn state(&self) -> ProgramPresentationState {
        self.state
    }
    #[must_use]
    #[allow(
        clippy::unused_self,
        reason = "Preserve the public API signature used by daemon consumers."
    )]
    pub fn activity_module(&self) -> u16 {
        9
    }
    #[must_use]
    pub fn activity_ordinal(&self) -> u16 {
        self.activity_ordinal
    }
    #[must_use]
    pub fn reservation(&self) -> &ProgramBudgetReservation {
        &self.reservation
    }
    #[must_use]
    pub fn canonical_unsigned_bytes(&self) -> &[u8] {
        &self.canonical_unsigned_bytes
    }
    #[must_use]
    pub fn immutable_carrier_bytes(&self) -> &[u8] {
        &self.immutable_carrier_bytes
    }
    #[must_use]
    pub fn created_at_sequence(&self) -> u64 {
        self.created_at_sequence
    }
    #[must_use]
    pub fn budget_expiry_sequence(&self) -> u64 {
        self.budget_expiry_sequence
    }
    #[must_use]
    pub fn created_at_unix_seconds(&self) -> u64 {
        self.created_at_unix_seconds
    }
    #[must_use]
    pub fn activity_expires_at_unix_milliseconds(&self) -> u64 {
        self.activity_expires_at_unix_milliseconds
    }
    #[must_use]
    pub fn release_ref(&self) -> Option<[u8; 32]> {
        self.release_ref
    }
    #[must_use]
    pub fn fee_asset(&self) -> Option<[u8; 32]> {
        self.fee_asset
    }
    #[must_use]
    pub fn operation(&self) -> &ProgramPresentationOperation {
        &self.operation
    }
    #[must_use]
    pub fn semantics(&self) -> &ProgramPresentationSemantics {
        &self.semantics
    }
}

type AllocationKey = (ProgramChargeKind, [u8; 32], [u8; 32], Option<[u8; 32]>);

/// # Errors
/// Returns an error if the owned approval, preparation, budget or protocol evidence cannot be
/// validated or read.
pub fn read_owned(
    store: &Store,
    peer: &HumanPeer,
    id: [u8; 32],
    registry: &ModuleRegistry,
) -> Result<OwnedNativeProgramPresentation, ProgramPresentationError> {
    let carrier = NativeProgramApprovalCarrier::read_presentation_carrier(store, peer, id)?;
    if carrier.retained_call().is_some() {
        return read_owned_call(&carrier, peer, id);
    }
    let presentation = carrier.presentation(store)?;
    let prepared = carrier.restore_prepared(registry)?;
    crate::prepare::verify_call_binding(&prepared)
        .map_err(|_| ProgramPresentationError::Binding)?;
    crate::reject_if(
        prepared.disclosure.activity_type.module() != ModuleId::Programs
            || prepared.disclosure.actor.as_slice() != prepared.from,
        || ProgramPresentationError::Binding,
    )?;
    let operation = match &prepared.disclosure.native_operation {
        Some(DisclosedNativeOperation::ProgramDeploy(value)) => {
            ProgramPresentationOperation::Deploy((**value).clone())
        }
        Some(DisclosedNativeOperation::ProgramUpgrade(value)) => {
            ProgramPresentationOperation::Upgrade((**value).clone())
        }
        Some(DisclosedNativeOperation::ProgramCall(value)) => {
            ProgramPresentationOperation::Call((**value).clone())
        }
        Some(DisclosedNativeOperation::ProgramWindDown(value)) => {
            ProgramPresentationOperation::WindDown((**value).clone())
        }
        _ => return Err(ProgramPresentationError::Unsupported),
    };
    let plan = derive_effects(
        &prepared.disclosure,
        &VerifiedInputs {
            revoke_balance: None,
        },
    )
    .map_err(|error| match error {
        crate::capability::EffectsError::ProgramExitContextRequired
        | crate::capability::EffectsError::UnboundedProgramExit => {
            ProgramPresentationError::MissingAuthority
        }
        _ => ProgramPresentationError::Corrupt,
    })?;
    let reservation = carrier.budget()?;
    let (fee_asset, limits) =
        legacy_presentation_allocations(&carrier, &prepared, &reservation, &plan)?;
    let immutable_carrier_bytes = carrier.immutable_hold_bytes()?;
    let held_digest = carrier.held_digest()?;
    crate::reject_if(
        <[u8; 32]>::from(Sha256::digest(&immutable_carrier_bytes)) != held_digest
            || <[u8; 32]>::from(Sha256::digest(prepared.call.encode_unsigned())) != id,
        || ProgramPresentationError::Binding,
    )?;
    let state = match carrier.state() {
        NativeApprovalState::Awaiting => ProgramPresentationState::Awaiting,
        NativeApprovalState::Granted => ProgramPresentationState::Granted,
        NativeApprovalState::Rejected => ProgramPresentationState::Rejected,
        NativeApprovalState::Expired => ProgramPresentationState::Expired,
        NativeApprovalState::Defective => ProgramPresentationState::Defective,
        NativeApprovalState::NotRequired => ProgramPresentationState::NotRequired,
    };
    Ok(OwnedNativeProgramPresentation {
        call: None,
        id,
        held_digest,
        actor: carrier.retained_actor().to_vec(),
        owner: peer
            .subject
            .as_ref()
            .ok_or(ProgramPresentationError::Binding)?
            .owner
            .clone(),
        state,
        activity_ordinal: prepared.disclosure.activity_type.ordinal(),
        canonical_unsigned_bytes: prepared.call.encode_unsigned(),
        immutable_carrier_bytes,
        reservation,
        created_at_sequence: presentation.created_at_sequence,
        budget_expiry_sequence: presentation.budget_expiry_sequence,
        created_at_unix_seconds: presentation.created_at_unix_seconds,
        activity_expires_at_unix_milliseconds: presentation.activity_expires_at_unix_milliseconds,
        release_ref: carrier.response()?.submission_ref,
        fee_asset,
        operation,
        semantics: if limits.is_empty() {
            ProgramPresentationSemantics::OperationOnly
        } else {
            ProgramPresentationSemantics::AuthorizedLimits(limits)
        },
    })
}

fn principal_source_matches(
    owner: &[u8],
    source: [u8; 32],
    asset: [u8; 32],
) -> Result<bool, ProgramPresentationError> {
    let actor =
        &layerx_types::ids::Did::new(owner).map_err(|_| ProgramPresentationError::Binding)?;
    let did =
        std::str::from_utf8(actor.as_bytes()).map_err(|_| ProgramPresentationError::Binding)?;
    let main = layerx_types::account::AccountId::parse(&format!("agent:{did}:main"))
        .map_err(|_| ProgramPresentationError::Binding)?;
    let asset_hex: String = layerx_programs::hex::encode(&asset);
    let asset_account =
        layerx_types::account::AccountId::parse(&format!("agent:{did}:asset:{asset_hex}"))
            .map_err(|_| ProgramPresentationError::Binding)?;
    Ok(layerx_wire::hash::account_id_for_protocol(&main, 3)
        .map_err(|_| ProgramPresentationError::Binding)?
        == source
        || layerx_wire::hash::account_id_for_protocol(&asset_account, 3)
            .map_err(|_| ProgramPresentationError::Binding)?
            == source)
}

fn read_owned_call(
    carrier: &NativeProgramApprovalCarrier,
    peer: &HumanPeer,
    id: [u8; 32],
) -> Result<OwnedNativeProgramPresentation, ProgramPresentationError> {
    use super::CallApprovalState;
    fn bind<T>(_: T) -> ProgramPresentationError {
        ProgramPresentationError::Binding
    }
    let retained = carrier
        .retained_call()
        .ok_or(ProgramPresentationError::Binding)?;
    let prepared = carrier.restore_call().map_err(bind)?;
    crate::prepare::verify_call_binding(&prepared).map_err(bind)?;
    crate::reject_if(
        prepared.disclosure.activity_type.module() != ModuleId::Programs
            || prepared.disclosure.actor != prepared.from,
        || ProgramPresentationError::Binding,
    )?;
    let operation = match &prepared.disclosure.native_operation {
        Some(DisclosedNativeOperation::ProgramDeploy(value)) => {
            ProgramPresentationOperation::Deploy((**value).clone())
        }
        Some(DisclosedNativeOperation::ProgramUpgrade(value)) => {
            ProgramPresentationOperation::Upgrade((**value).clone())
        }
        Some(DisclosedNativeOperation::ProgramCall(value)) => {
            ProgramPresentationOperation::Call((**value).clone())
        }
        Some(DisclosedNativeOperation::ProgramWindDown(value)) => {
            ProgramPresentationOperation::WindDown((**value).clone())
        }
        _ => return Err(ProgramPresentationError::Unsupported),
    };
    let plan = derive_effects(
        &prepared.disclosure,
        &VerifiedInputs {
            revoke_balance: None,
        },
    )
    .map_err(|_| ProgramPresentationError::MissingAuthority)?;
    let reservation = carrier.call_budget().map_err(bind)?;
    let subject = peer
        .subject
        .as_ref()
        .ok_or(ProgramPresentationError::Binding)?;
    let limits = call_presentation_allocations(&prepared, &reservation, &plan, id, subject.asset)?;
    let immutable_carrier_bytes = retained.immutable_bytes().map_err(bind)?;
    let held_digest = carrier.call_held_digest().map_err(bind)?;
    crate::reject_if(
        <[u8; 32]>::from(Sha256::digest(&immutable_carrier_bytes)) != held_digest
            || <[u8; 32]>::from(Sha256::digest(prepared.call.encode_unsigned())) != id,
        || ProgramPresentationError::Binding,
    )?;
    let state = match carrier.call_state().map_err(bind)? {
        CallApprovalState::Awaiting => ProgramPresentationState::Awaiting,
        CallApprovalState::Rejected => ProgramPresentationState::Rejected,
        CallApprovalState::Expired => ProgramPresentationState::Expired,
        _ => ProgramPresentationState::Granted,
    };
    Ok(OwnedNativeProgramPresentation {
        call: Some(prepared.call.clone()),
        id,
        held_digest,
        actor: prepared.from.to_vec(),
        owner: subject.owner.clone(),
        state,
        activity_ordinal: prepared.disclosure.activity_type.ordinal(),
        canonical_unsigned_bytes: prepared.call.encode_unsigned(),
        immutable_carrier_bytes,
        created_at_sequence: prepared.observed_head_height,
        budget_expiry_sequence: reservation.expiry_sequence,
        created_at_unix_seconds: retained.admitted_at_ms / 1000,
        activity_expires_at_unix_milliseconds: retained.deadline().map_err(bind)?,
        release_ref: carrier.response()?.submission_ref,
        fee_asset: Some(subject.asset),
        operation,
        semantics: if limits.is_empty() {
            ProgramPresentationSemantics::OperationOnly
        } else {
            ProgramPresentationSemantics::AuthorizedLimits(limits)
        },
        reservation,
    })
}

fn legacy_presentation_allocations(
    carrier: &NativeProgramApprovalCarrier,
    prepared: &crate::prepare::EvmPrepared,
    reservation: &ProgramBudgetReservation,
    plan: &crate::capability::SemanticPlan,
) -> Result<(Option<[u8; 32]>, Vec<ProgramBudgetAllocation>), ProgramPresentationError> {
    let preparation_digest: [u8; 32] = Sha256::digest(prepared.call.encode_unsigned()).into();
    let actor = layerx_wire::hash::did_id_for_protocol(
        &layerx_types::ids::Did::new(carrier.retained_actor())
            .map_err(|_| ProgramPresentationError::Binding)?,
        3,
    )
    .map_err(|_| ProgramPresentationError::Binding)?;
    crate::reject_if(
        reservation.id != preparation_digest
            || reservation.allocation_preparation_digest() != Some(preparation_digest)
            || reservation.allocation_sequence() != Some(prepared.observed_head_height)
            || reservation.allocation_actor() != Some(actor),
        || ProgramPresentationError::Binding,
    )?;
    let allocations = reservation
        .allocations()
        .ok_or(ProgramPresentationError::MissingAuthority)?;
    let mut actual = BTreeMap::<AllocationKey, u128>::new();
    let mut fee_asset = None;
    let mut limits = Vec::new();
    for allocation in allocations {
        if allocation.kind == ProgramChargeKind::Fee {
            crate::reject_if(
                fee_asset.replace(allocation.asset).is_some()
                    || allocation.destination.is_some()
                    || allocation.maximum_amount
                        != prepared
                            .call
                            .max_fee_per_gas
                            .checked_mul(u128::from(prepared.call.gas_limit))
                            .ok_or(ProgramPresentationError::Binding)?
                    || !principal_source_matches(
                        carrier.retained_actor(),
                        allocation.source,
                        allocation.asset,
                    )?,
                || ProgramPresentationError::Binding,
            )?;
        } else {
            let key = (
                allocation.kind,
                allocation.source,
                allocation.asset,
                allocation.destination,
            );
            crate::reject_if(
                actual.insert(key, allocation.maximum_amount).is_some(),
                || ProgramPresentationError::Binding,
            )?;
            limits.push(allocation.clone());
        }
    }
    crate::reject_if(
        fee_asset.is_some()
            != (prepared
                .call
                .max_fee_per_gas
                .checked_mul(u128::from(prepared.call.gas_limit))
                .ok_or(ProgramPresentationError::Binding)?
                != 0),
        || ProgramPresentationError::Binding,
    )?;
    let expected = legacy_expected_allocations(carrier, plan, &limits)?;
    crate::reject_if(expected != actual, || ProgramPresentationError::Binding)?;
    Ok((fee_asset, limits))
}
fn legacy_expected_allocations(
    carrier: &NativeProgramApprovalCarrier,
    plan: &crate::capability::SemanticPlan,
    limits: &[ProgramBudgetAllocation],
) -> Result<BTreeMap<AllocationKey, u128>, ProgramPresentationError> {
    let mut expected = BTreeMap::<AllocationKey, u128>::new();
    for bound in plan.program_spend_bounds() {
        let (kind, source) = match &bound.source {
            ProgramValueSource::Principal => {
                let mut sources = limits.iter().filter(|allocation| {
                    allocation.kind == ProgramChargeKind::Principal
                        && allocation.asset == bound.asset
                        && allocation.destination == Some(bound.destination)
                });
                let source = sources
                    .next()
                    .ok_or(ProgramPresentationError::Binding)?
                    .source;
                crate::reject_if(
                    sources.next().is_some()
                        || !principal_source_matches(
                            carrier.retained_actor(),
                            source,
                            bound.asset,
                        )?,
                    || ProgramPresentationError::Binding,
                )?;
                (ProgramChargeKind::Principal, source)
            }
            ProgramValueSource::Program {
                owner_program,
                seed,
                source_account,
            } => {
                let program = layerx_programs_runtime::ProgramId::new(*owner_program)
                    .map_err(|_| ProgramPresentationError::Binding)?;
                let derived =
                    layerx_programs_runtime::accounts::derive_program_account(program, seed)
                        .map_err(|_| ProgramPresentationError::Binding)?;
                crate::reject_if(!derived.matches(source_account), || {
                    ProgramPresentationError::Binding
                })?;
                (ProgramChargeKind::ProgramSpend, *source_account)
            }
        };
        let amount = expected
            .entry((kind, source, bound.asset, Some(bound.destination)))
            .or_default();
        *amount = amount
            .checked_add(bound.maximum_amount)
            .ok_or(ProgramPresentationError::Corrupt)?;
    }

    Ok(expected)
}

fn call_presentation_allocations(
    prepared: &crate::prepare::EvmPrepared,
    reservation: &ProgramBudgetReservation,
    plan: &crate::capability::SemanticPlan,
    id: [u8; 32],
    fee_asset: [u8; 32],
) -> Result<Vec<ProgramBudgetAllocation>, ProgramPresentationError> {
    let mut sender = [0; 32];
    sender[12..].copy_from_slice(&prepared.from);
    let fee = u128::from(prepared.call.gas_limit)
        .checked_mul(prepared.call.max_fee_per_gas)
        .ok_or(ProgramPresentationError::Binding)?;
    crate::reject_if(
        fee != prepared.disclosure.fee_limit || reservation.id != id,
        || ProgramPresentationError::Binding,
    )?;
    let fees = reservation
        .charges
        .iter()
        .filter(|charge| charge.kind == ProgramChargeKind::Fee)
        .collect::<Vec<_>>();
    crate::reject_if(
        fees.len() != 1
            || fees[0].source != sender
            || fees[0].asset != fee_asset
            || fees[0].amount != fee,
        || ProgramPresentationError::Binding,
    )?;
    let mut actual = BTreeMap::<AllocationKey, u128>::new();
    let mut limits = Vec::new();
    if let Some(allocations) = reservation.allocations() {
        crate::reject_if(
            reservation.allocation_preparation_digest() != Some(id)
                || reservation.allocation_sequence() != Some(prepared.observed_head_height)
                || reservation.allocation_actor() != Some(sender),
            || ProgramPresentationError::Binding,
        )?;
        let mut fee_count = 0;
        for allocation in allocations {
            if allocation.kind == ProgramChargeKind::Fee {
                fee_count += 1;
                crate::reject_if(
                    allocation.source != sender
                        || allocation.asset != fee_asset
                        || allocation.destination.is_some()
                        || allocation.maximum_amount != fee,
                    || ProgramPresentationError::Binding,
                )?;
            } else {
                crate::reject_if(
                    actual
                        .insert(
                            (
                                allocation.kind,
                                allocation.source,
                                allocation.asset,
                                allocation.destination,
                            ),
                            allocation.maximum_amount,
                        )
                        .is_some(),
                    || ProgramPresentationError::Binding,
                )?;
                limits.push(allocation.clone());
            }
        }
        crate::reject_if(fee_count != 1, || ProgramPresentationError::Binding)?;
    } else {
        crate::reject_if(
            reservation
                .charges
                .iter()
                .any(|charge| charge.kind != ProgramChargeKind::Fee),
            || ProgramPresentationError::MissingAuthority,
        )?;
    }
    let mut expected = BTreeMap::<AllocationKey, u128>::new();
    for bound in plan.program_spend_bounds() {
        let (kind, source) = match &bound.source {
            ProgramValueSource::Principal => (ProgramChargeKind::Principal, sender),
            ProgramValueSource::Program {
                owner_program,
                seed,
                source_account,
            } => {
                let program = layerx_programs_runtime::ProgramId::new(*owner_program)
                    .map_err(|_| ProgramPresentationError::Binding)?;
                let derived =
                    layerx_programs_runtime::accounts::derive_program_account(program, seed)
                        .map_err(|_| ProgramPresentationError::Binding)?;
                crate::reject_if(!derived.matches(source_account), || {
                    ProgramPresentationError::Binding
                })?;
                (ProgramChargeKind::ProgramSpend, *source_account)
            }
        };
        let amount = expected
            .entry((kind, source, bound.asset, Some(bound.destination)))
            .or_default();
        *amount = amount
            .checked_add(bound.maximum_amount)
            .ok_or(ProgramPresentationError::Binding)?;
    }
    crate::reject_if(expected != actual, || ProgramPresentationError::Binding)?;
    Ok(limits)
}
