use std::collections::BTreeMap;

use crate::read::committed::SourceAccount;
use crate::read::CommittedCaps;
use layerx_crypto::disclosure::DisclosedNativeOperation;
use layerx_programs::{ProgramId, ProgramLifecycle, VerifiedProgramBalanceRead};
#[cfg(test)]
use layerx_proof::state::CanonicalAccount;
use layerx_types::ids::Did;
use layerx_wire::hash::did_id_for_protocol;
use sha2::{Digest as _, Sha256};

use crate::capability::{derive_effects, ProgramValueSource, VerifiedInputs};
use crate::ops::program::ProgramOperations;
use crate::prepare::{verify_call_binding, EvmPrepared};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProgramSourceError {
    Preparation,
    Unsupported,
    Snapshot,
    MissingSource,
    AmbiguousSource,
    SourceOwnership,
    SourceAsset,
    FrozenSource,
    ProgramBinding,
    ProgramUnavailable,
    FeeUnavailable,
    Arithmetic,
}

pub(super) fn bind_program_presentation_budget_proof(
    reservation: &super::ProgramBudgetReservation,
    actor: &[u8],
    proof: &super::budget_proof::VerifiedBudgetProof,
    sequence: u64,
) -> Result<(), ProgramSourceError> {
    reservation
        .validate()
        .map_err(|_| ProgramSourceError::Preparation)?;
    let actor = Did::new(actor).map_err(|_| ProgramSourceError::SourceOwnership)?;
    if actor.as_bytes() != proof.owner().as_bytes()
        || proof.observed_head_sequence() != sequence
        || reservation
            .allocation_sequence()
            .is_none_or(|allocated| allocated > sequence)
        || proof.age_sequences() > proof.maximum_age_sequences()
        || proof.maximum_age_sequences() == 0
        || super::budget_state_is_committed(proof.verification().wire_rank()).is_err()
    {
        return Err(ProgramSourceError::Snapshot);
    }
    let rows = reservation
        .allocations()
        .ok_or(ProgramSourceError::MissingSource)?;
    if !rows.iter().any(|row| {
        row.asset == proof.asset()
            && row.source == proof.source_account()
            && !row.applicable_limits.is_empty()
    }) {
        return Err(ProgramSourceError::SourceAsset);
    }
    Ok(())
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ResolvedProgramSource {
    Principal {
        account: [u8; 32],
    },
    Program {
        owner_program: [u8; 32],
        seed: Vec<u8>,
        account: [u8; 32],
    },
    Fee {
        account: [u8; 32],
    },
}

impl ResolvedProgramSource {
    #[must_use]
    pub const fn account(&self) -> [u8; 32] {
        match self {
            Self::Principal { account } | Self::Program { account, .. } | Self::Fee { account } => {
                *account
            }
        }
    }

    #[must_use]
    pub const fn kind(&self) -> super::ProgramChargeKind {
        match self {
            Self::Principal { .. } => super::ProgramChargeKind::Principal,
            Self::Program { .. } => super::ProgramChargeKind::ProgramSpend,
            Self::Fee { .. } => super::ProgramChargeKind::Fee,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ResolvedProgramCharge {
    pub source: ResolvedProgramSource,
    pub asset: [u8; 32],
    pub destination: Option<[u8; 32]>,
    pub maximum_amount: u128,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedResolvedProgramCharges {
    actor: [u8; 32],
    owner_did: Did,
    state_root: [u8; 32],
    batch_number: u64,
    global_sequence: u64,
    preparation_digest: [u8; 32],
    envelope_digest: [u8; 32],
    charges: Vec<ResolvedProgramCharge>,
}

impl VerifiedResolvedProgramCharges {
    pub(super) fn owner_did(&self) -> &Did {
        &self.owner_did
    }
    #[must_use]
    pub const fn actor(&self) -> [u8; 32] {
        self.actor
    }
    #[must_use]
    pub const fn state_root(&self) -> [u8; 32] {
        self.state_root
    }
    #[must_use]
    pub const fn batch_number(&self) -> u64 {
        self.batch_number
    }
    #[must_use]
    pub const fn global_sequence(&self) -> u64 {
        self.global_sequence
    }
    #[must_use]
    pub fn charges(&self) -> &[ResolvedProgramCharge] {
        &self.charges
    }
    #[must_use]
    pub fn matches_prepared(&self, prepared: &EvmPrepared) -> bool {
        validate_prepared(prepared).is_ok()
            && self.global_sequence == prepared.observed_head_height
            && self.preparation_digest
                == <[u8; 32]>::from(Sha256::digest(prepared.call.encode_unsigned()))
    }

    /// # Errors
    /// Returns an error if limit configuration, denomination, reservation or persisted accounting
    /// is invalid, conflicting or unavailable.
    pub fn budget_allocations(
        &self,
        applicable: impl FnMut(
            &ResolvedProgramCharge,
        ) -> Result<Vec<super::LimitId>, super::DaemonLimitError>,
    ) -> Result<VerifiedProgramBudgetAllocations, super::DaemonLimitError> {
        let allocations = budget_allocation_rows(&self.charges, applicable)?;
        let charges = super::reservations::allocation_charges(&allocations)?;
        Ok(VerifiedProgramBudgetAllocations {
            actor: self.actor,
            state_root: self.state_root,
            batch_number: self.batch_number,
            global_sequence: self.global_sequence,
            preparation_digest: self.preparation_digest,
            allocations,
            charges,
        })
    }

    /// # Errors
    /// Returns an error if limit configuration, denomination, reservation or persisted accounting
    /// is invalid, conflicting or unavailable.
    pub fn budget_charges(
        &self,
        applicable: impl FnMut(
            &ResolvedProgramCharge,
        ) -> Result<Vec<super::LimitId>, super::DaemonLimitError>,
    ) -> Result<Vec<super::ProgramBudgetCharge>, super::DaemonLimitError> {
        budget_charge_rows(&self.charges, applicable)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedProgramBudgetAllocations {
    actor: [u8; 32],
    state_root: [u8; 32],
    batch_number: u64,
    global_sequence: u64,
    preparation_digest: [u8; 32],
    allocations: Vec<super::ProgramBudgetAllocation>,
    charges: Vec<super::ProgramBudgetCharge>,
}

impl VerifiedProgramBudgetAllocations {
    #[must_use]
    pub const fn actor(&self) -> [u8; 32] {
        self.actor
    }
    #[must_use]
    pub const fn state_root(&self) -> [u8; 32] {
        self.state_root
    }
    #[must_use]
    pub const fn batch_number(&self) -> u64 {
        self.batch_number
    }
    #[must_use]
    pub const fn global_sequence(&self) -> u64 {
        self.global_sequence
    }
    #[must_use]
    pub const fn preparation_digest(&self) -> [u8; 32] {
        self.preparation_digest
    }
    #[must_use]
    pub fn allocations(&self) -> &[super::ProgramBudgetAllocation] {
        &self.allocations
    }
    #[must_use]
    pub fn charges(&self) -> &[super::ProgramBudgetCharge] {
        &self.charges
    }
}

#[cfg(test)]
pub(super) fn presentation_budget_reservation(
    proof: &super::VerifiedBudgetProof,
    sequence: u64,
) -> super::ProgramBudgetReservation {
    use super::{
        BudgetLimiter, CoreTimestampMs, LimitConfig, LimitId, LimitScope, ProgramLimitDenomination,
        ProgramReservationRequest,
    };

    let limit_id = LimitId([1; 16]);
    let actor = Did::new(proof.owner().as_bytes()).expect("proof owner");
    let charges = VerifiedResolvedProgramCharges {
        actor: did_id_for_protocol(&actor, 3).expect("actor ID"),
        owner_did: actor.clone(),
        state_root: proof.checkpoint_digest(),
        batch_number: proof.committed_height(),
        global_sequence: sequence,
        preparation_digest: proof.digest(),
        envelope_digest: proof.digest(),
        charges: vec![ResolvedProgramCharge {
            source: ResolvedProgramSource::Principal {
                account: proof.source_account(),
            },
            asset: proof.asset(),
            destination: Some([9; 32]),
            maximum_amount: 1,
        }],
    };
    let allocations = charges
        .budget_allocations(|_| Ok(vec![limit_id]))
        .expect("budget allocations");
    let limiter = BudgetLimiter::new(vec![LimitConfig {
        id: limit_id,
        name: "presentation".to_owned(),
        scope: LimitScope::Tenant([1; 32]),
        ceiling: 100,
        consumed: 0,
    }])
    .expect("budget limiter");
    limiter
        .bind_program_denominations(
            &[(
                limit_id,
                ProgramLimitDenomination {
                    asset: proof.asset(),
                    source: Some(proof.source_account()),
                },
                false,
                false,
            )],
            || Ok::<(), super::LimitRefusal>(()),
        )
        .expect("limit denomination");
    let request = ProgramReservationRequest {
        id: [7; 32],
        charges: allocations.charges().to_vec(),
        expiry_sequence: sequence.checked_add(10).expect("expiry sequence"),
        current_sequence: sequence,
        core_deadline: Some(CoreTimestampMs(500)),
    };
    let reservation = limiter
        .stage_program_allocation(&request, &allocations, CoreTimestampMs(1))
        .expect("stage allocated reservation")
        .publish();
    super::ProgramBudgetReservation::decode(&reservation.encode().expect("encode reservation"))
        .expect("decode reservation")
}

#[cfg(test)]
impl VerifiedResolvedProgramCharges {
    pub(crate) fn assert_committed_inventory(
        caps: &CommittedCaps,
        actor: &Did,
        asset: [u8; 32],
        expected: [u8; 32],
    ) {
        assert_eq!(
            principal_source(caps.all_accounts(), actor, 3, asset),
            Ok(expected)
        );
        assert_eq!(
            caps.did(),
            layerx_wire::hash::did_id_for_protocol(actor, 3).expect("owner")
        );
        assert_eq!(
            caps.observed_height(),
            caps.freshness().observed_head_sequence
        );
        assert!(caps.level() >= layerx_types::verify::VerificationLevel::COMMITTED);
    }
}

fn budget_allocation_rows(
    charges: &[ResolvedProgramCharge],
    mut applicable: impl FnMut(
        &ResolvedProgramCharge,
    ) -> Result<Vec<super::LimitId>, super::DaemonLimitError>,
) -> Result<Vec<super::ProgramBudgetAllocation>, super::DaemonLimitError> {
    let mut rows: BTreeMap<_, super::ProgramBudgetAllocation> = BTreeMap::new();
    for charge in charges {
        let mut limits = applicable(charge)?;
        limits.sort_unstable();
        limits.dedup();
        if limits.is_empty() {
            return Err(super::DaemonLimitError::AmbiguousDenomination);
        }
        let key = (
            charge.asset,
            charge.source.account(),
            charge.source.kind(),
            charge.destination,
        );
        if let Some(row) = rows.get_mut(&key) {
            if row.applicable_limits != limits {
                return Err(super::DaemonLimitError::Conflict);
            }
            row.maximum_amount = row
                .maximum_amount
                .checked_add(charge.maximum_amount)
                .ok_or(super::DaemonLimitError::Arithmetic)?;
        } else {
            rows.insert(
                key,
                super::ProgramBudgetAllocation {
                    kind: charge.source.kind(),
                    source: charge.source.account(),
                    asset: charge.asset,
                    destination: charge.destination,
                    maximum_amount: charge.maximum_amount,
                    applicable_limits: limits,
                },
            );
        }
    }
    let rows: Vec<_> = rows.into_values().collect();
    super::reservations::allocation_charges(&rows)?;
    Ok(rows)
}

fn budget_charge_rows(
    charges: &[ResolvedProgramCharge],
    mut applicable: impl FnMut(
        &ResolvedProgramCharge,
    ) -> Result<Vec<super::LimitId>, super::DaemonLimitError>,
) -> Result<Vec<super::ProgramBudgetCharge>, super::DaemonLimitError> {
    let mut rows: BTreeMap<ProgramChargeKey, super::ProgramBudgetCharge> = BTreeMap::new();
    for charge in charges {
        let mut limits = applicable(charge)?;
        limits.sort_unstable();
        limits.dedup();
        if limits.is_empty() {
            return Err(super::DaemonLimitError::AmbiguousDenomination);
        }
        let key = (
            charge.asset,
            charge.source.account(),
            charge.source.kind(),
            limits.clone(),
        );
        if let Some(row) = rows.get_mut(&key) {
            row.amount = row
                .amount
                .checked_add(charge.maximum_amount)
                .ok_or(super::DaemonLimitError::Arithmetic)?;
        } else {
            rows.insert(
                key,
                super::ProgramBudgetCharge {
                    kind: charge.source.kind(),
                    source: charge.source.account(),
                    asset: charge.asset,
                    amount: charge.maximum_amount,
                    applicable_limits: limits,
                },
            );
        }
    }
    Ok(rows.into_values().collect())
}

/// # Errors
/// Returns an error if the native call, source ownership or snapshot is invalid, a program is
/// unavailable, or charge arithmetic overflows.
pub fn read_program_budget_sources_call(
    node: &mut layerx_client::sequencer::SequencerEndpoint,
    programs: &mut ProgramOperations,
    prepared: &EvmPrepared,
    caps: &CommittedCaps,
    now: u64,
    _fee_correlation_id: u64,
) -> Result<VerifiedResolvedProgramCharges, ProgramSourceError> {
    validate_prepared(prepared)?;
    let owner_did = committed_owner_did(caps)?;
    verify_fee_payer(node, prepared, &owner_did, caps.state_root())?;
    crate::reject_if(
        !matches!(
            prepared.disclosure.native_operation,
            Some(
                DisclosedNativeOperation::ProgramDeploy(_)
                    | DisclosedNativeOperation::ProgramUpgrade(_)
                    | DisclosedNativeOperation::ProgramCall(_)
                    | DisclosedNativeOperation::ProgramWindDown(_)
                    | DisclosedNativeOperation::LegacyProgramCall(_)
            )
        ),
        || ProgramSourceError::Unsupported,
    )?;
    let actor = caps.did();
    let freshness = caps.freshness();
    crate::reject_if(
        now == 0
            || caps.did() != actor
            || caps.state_root() == [0; 32]
            || caps.level() < layerx_types::verify::VerificationLevel::STATE_PROVEN
            || freshness.global_sequence != prepared.observed_head_height
            || freshness.observed_head_sequence != prepared.observed_head_height
            || freshness.batch_number != caps.observed_height()
            || node
                .block_number()
                .map_err(|_| ProgramSourceError::Snapshot)?
                != prepared.observed_head_height
            || node
                .block_number()
                .map_err(|_| ProgramSourceError::Snapshot)?
                != caps.observed_height(),
        || ProgramSourceError::Snapshot,
    )?;
    let plan = derive_effects(&prepared.disclosure, &VerifiedInputs::default())
        .map_err(|_| ProgramSourceError::Preparation)?;
    let mut charges = resolve_program_spend_charges(programs, caps, now, &owner_did, &plan)?;
    let maximum_fee = prepared.disclosure.fee_limit;
    if let Some(charge) = read_fee_charge_for(
        node,
        caps.all_accounts(),
        &owner_did,
        &FeeChargeContext {
            protocol: 3,
            height: prepared.observed_head_height,
            state_root: caps.state_root(),
            maximum_fee,
            payer: prepared.from,
            expected_asset: None,
        },
    )? {
        charges.push(charge);
    }
    crate::reject_if(
        node.block_number()
            .map_err(|_| ProgramSourceError::Snapshot)?
            != prepared.observed_head_height
            || node
                .block_number()
                .map_err(|_| ProgramSourceError::Snapshot)?
                != caps.observed_height(),
        || ProgramSourceError::Snapshot,
    )?;
    Ok(VerifiedResolvedProgramCharges {
        actor,
        owner_did,
        state_root: caps.state_root(),
        batch_number: caps.observed_height(),
        global_sequence: prepared.observed_head_height,
        preparation_digest: Sha256::digest(prepared.call.encode_unsigned()).into(),
        envelope_digest: Sha256::digest(prepared.call.encode_unsigned()).into(),
        charges,
    })
}

/// # Errors
/// Returns an error if the native call, source ownership or snapshot is invalid, a program is
/// unavailable, or charge arithmetic overflows.
pub fn read_native_effect_budget_sources_call(
    node: &mut layerx_client::sequencer::SequencerEndpoint,
    prepared: &EvmPrepared,
    caps: &CommittedCaps,
    now: u64,
    _fee_correlation_id: u64,
    profile: crate::capability::NativeAdmissionProfile,
) -> Result<VerifiedResolvedProgramCharges, ProgramSourceError> {
    validate_prepared(prepared)?;
    let owner_did = committed_owner_did(caps)?;
    verify_fee_payer(node, prepared, &owner_did, caps.state_root())?;
    crate::reject_if(
        prepared.disclosure.activity_type.module() == layerx_types::payload::ModuleId::Programs,
        || ProgramSourceError::Unsupported,
    )?;
    let actor = caps.did();
    let freshness = caps.freshness();
    crate::reject_if(
        now == 0
            || caps.did() != actor
            || caps.state_root() == [0; 32]
            || caps.level() < layerx_types::verify::VerificationLevel::STATE_PROVEN
            || freshness.global_sequence != prepared.observed_head_height
            || freshness.observed_head_sequence != prepared.observed_head_height
            || freshness.batch_number != caps.observed_height()
            || node
                .block_number()
                .map_err(|_| ProgramSourceError::Snapshot)?
                != prepared.observed_head_height
            || node
                .block_number()
                .map_err(|_| ProgramSourceError::Snapshot)?
                != caps.observed_height(),
        || ProgramSourceError::Snapshot,
    )?;
    let plan = crate::capability::derive_native_profile_effects(&prepared.disclosure, profile)
        .map_err(|_| ProgramSourceError::Preparation)?;
    let mut charges = resolve_native_effect_charges(prepared, caps, &owner_did, &plan)?;
    let maximum_fee = prepared.disclosure.fee_limit;
    if let Some(charge) = read_fee_charge_for(
        node,
        caps.all_accounts(),
        &owner_did,
        &FeeChargeContext {
            protocol: 3,
            height: prepared.observed_head_height,
            state_root: caps.state_root(),
            maximum_fee,
            payer: prepared.from,
            expected_asset: match profile {
                crate::capability::NativeAdmissionProfile::Registration { fee_asset } => {
                    Some(fee_asset)
                }
                crate::capability::NativeAdmissionProfile::Effect => None,
            },
        },
    )? {
        charges.push(charge);
    }
    crate::reject_if(
        node.block_number()
            .map_err(|_| ProgramSourceError::Snapshot)?
            != prepared.observed_head_height
            || node
                .block_number()
                .map_err(|_| ProgramSourceError::Snapshot)?
                != caps.observed_height(),
        || ProgramSourceError::Snapshot,
    )?;
    Ok(VerifiedResolvedProgramCharges {
        actor,
        owner_did,
        state_root: caps.state_root(),
        batch_number: caps.observed_height(),
        global_sequence: prepared.observed_head_height,
        preparation_digest: Sha256::digest(prepared.call.encode_unsigned()).into(),
        envelope_digest: Sha256::digest(prepared.call.encode_unsigned()).into(),
        charges,
    })
}

fn read_fee_charge_for<A: SourceAccount>(
    node: &layerx_client::sequencer::SequencerEndpoint,
    accounts: &BTreeMap<[u8; 32], A>,
    actor: &Did,
    input: &FeeChargeContext,
) -> Result<Option<ResolvedProgramCharge>, ProgramSourceError> {
    let FeeChargeContext {
        protocol,
        height,
        state_root,
        maximum_fee,
        payer,
        expected_asset,
    } = *input;

    if maximum_fee == 0 {
        return Ok(None);
    }
    let fee = super::fee_policy::native_fee_policy_for(node, height, payer)
        .map_err(|_| ProgramSourceError::FeeUnavailable)?;
    let asset = fee.value.asset.asset_id;
    if fee.state_root != state_root
        || fee.observed_sequence != height
        || expected_asset.is_some_and(|expected| expected != asset)
    {
        return Err(ProgramSourceError::Snapshot);
    }
    if fee.value.asset.paused {
        return Err(ProgramSourceError::FrozenSource);
    }
    let account = principal_source(accounts, actor, protocol, asset)?;
    Ok(Some(ResolvedProgramCharge {
        source: ResolvedProgramSource::Fee { account },
        asset,
        destination: None,
        maximum_amount: maximum_fee,
    }))
}

fn validate_prepared(prepared: &EvmPrepared) -> Result<(), ProgramSourceError> {
    verify_call_binding(prepared).map_err(|_| ProgramSourceError::Preparation)?;
    if prepared.disclosure.actor.as_slice() != prepared.from {
        return Err(ProgramSourceError::Preparation);
    }
    Ok(())
}

pub(super) fn principal_source<A: SourceAccount>(
    accounts: &BTreeMap<[u8; 32], A>,
    actor: &Did,
    protocol: u16,
    asset: [u8; 32],
) -> Result<[u8; 32], ProgramSourceError> {
    if asset == [0; 32] {
        return Err(ProgramSourceError::SourceAsset);
    }
    let principal =
        did_id_for_protocol(actor, protocol).map_err(|_| ProgramSourceError::Preparation)?;
    let mut selected = None;
    match protocol {
        1 | 2 => {
            if let Some(account) = accounts
                .get(&principal)
                .filter(|account| account.kind() == 1)
            {
                selected = Some(account);
            }
        }
        3 => {
            for account in accounts.values() {
                if !matches!(account.kind(), 1 | 14) || account.asset_id() != asset {
                    continue;
                }
                if account_owner(account)? != principal {
                    continue;
                }
                if selected.replace(account).is_some() {
                    return Err(ProgramSourceError::AmbiguousSource);
                }
            }
        }
        _ => return Err(ProgramSourceError::Unsupported),
    }
    let account = selected.ok_or(ProgramSourceError::MissingSource)?;
    if !account.has_asset() || account.asset_id() != asset {
        return Err(ProgramSourceError::SourceAsset);
    }
    if account.frozen() {
        return Err(ProgramSourceError::FrozenSource);
    }
    Ok(account.account_id())
}

fn account_owner_did<A: SourceAccount>(account: &A) -> Result<Did, ProgramSourceError> {
    let name = account.name();
    let end = match account.kind() {
        1 if name.starts_with(b"agent:") && name.ends_with(b":main") => name.len().checked_sub(5),
        14 if name.starts_with(b"agent:")
            && name.len() > 77
            && &name[name.len() - 71..name.len() - 64] == b":asset:" =>
        {
            name.len().checked_sub(71)
        }
        _ => None,
    }
    .ok_or(ProgramSourceError::SourceOwnership)?;
    let did = Did::new(
        name.get(6..end)
            .ok_or(ProgramSourceError::SourceOwnership)?,
    )
    .map_err(|_| ProgramSourceError::SourceOwnership)?;
    Ok(did)
}

fn account_owner<A: SourceAccount>(account: &A) -> Result<[u8; 32], ProgramSourceError> {
    did_id_for_protocol(&account_owner_did(account)?, 3)
        .map_err(|_| ProgramSourceError::SourceOwnership)
}

fn committed_owner_did(caps: &CommittedCaps) -> Result<Did, ProgramSourceError> {
    let mut owner = None;
    for account in caps.all_accounts().values() {
        if !matches!(account.kind, 1 | 14) {
            continue;
        }
        let did = account_owner_did(account)?;
        if did_id_for_protocol(&did, 3).map_err(|_| ProgramSourceError::SourceOwnership)?
            != caps.did()
            || owner.as_ref().is_some_and(|previous| previous != &did)
        {
            return Err(ProgramSourceError::SourceOwnership);
        }
        owner = Some(did);
    }
    owner.ok_or(ProgramSourceError::SourceOwnership)
}

fn program_source(
    caps: &CommittedCaps,
    read: &VerifiedProgramBalanceRead,
    owner: [u8; 32],
    seed: &[u8],
    source: [u8; 32],
    asset: [u8; 32],
) -> Result<(), ProgramSourceError> {
    if read.program().bytes() != owner
        || read.lifecycle() != ProgramLifecycle::Active
        || read.state_root() != caps.state_root()
        || read.freshness().observed_sequence != caps.freshness().global_sequence
    {
        return Err(ProgramSourceError::ProgramBinding);
    }
    let mut bindings = read
        .bindings()
        .iter()
        .filter(|binding| binding.program.bytes() == owner && binding.seed == seed);
    let binding = bindings.next().ok_or(ProgramSourceError::ProgramBinding)?;
    if bindings.next().is_some() || binding.account_id != source || binding.asset_id != asset {
        return Err(ProgramSourceError::ProgramBinding);
    }
    let mut values = read
        .value_accounts()
        .iter()
        .filter(|value| value.seed == seed && value.account_id == source);
    let value = values.next().ok_or(ProgramSourceError::ProgramBinding)?;
    if values.next().is_some()
        || value.asset_id != asset
        || value.state_root != caps.state_root()
        || value.observed_sequence != caps.freshness().global_sequence
    {
        return Err(ProgramSourceError::ProgramBinding);
    }
    let account = caps
        .all_accounts()
        .get(&source)
        .ok_or(ProgramSourceError::MissingSource)?;
    if account.kind != 13 || account.authority_key.is_some() {
        return Err(ProgramSourceError::SourceOwnership);
    }
    if !account.has_asset() || account.asset_id() != asset || account.balance() != value.balance {
        return Err(ProgramSourceError::SourceAsset);
    }
    if account.frozen || value.frozen {
        return Err(ProgramSourceError::FrozenSource);
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
        value.unwrap_or_else(|error| panic!("Program source: {error:?}"))
    }

    #[test]
    fn program_budget_sources_bind_the_sequencer_endpoint() {
        let endpoint = layerx_client::sequencer::SequencerEndpoint::new("http://127.0.0.1:1", &[])
            .expect("loopback sequencer endpoint");
        let _: fn(
            &mut layerx_client::sequencer::SequencerEndpoint,
            &mut ProgramOperations,
            &EvmPrepared,
            &CommittedCaps,
            u64,
            u64,
        ) -> Result<VerifiedResolvedProgramCharges, ProgramSourceError> =
            read_program_budget_sources_call;
        assert!(matches!(
            layerx_client::sequencer::SequencerEndpoint::new("http://example.com", &[]),
            Err(layerx_client::sequencer::SequencerError::Endpoint)
        ));
        drop(endpoint);
    }

    fn canonical_account(did: &str, asset: [u8; 32], kind: u8, frozen: bool) -> CanonicalAccount {
        let name = if kind == 1 {
            format!("agent:{did}:main")
        } else {
            format!("agent:{did}:asset:{}", hex::encode(asset))
        };
        let mut identity = b"LX:ACCOUNT:v1".to_vec();
        identity.extend(must(u32::try_from(name.len())).to_be_bytes());
        identity.extend(name.as_bytes());
        let id = Sha256::digest(identity).into();
        let mut bytes = must(u16::try_from(name.len())).to_be_bytes().to_vec();
        bytes.extend(name.as_bytes());
        bytes.push(kind);
        bytes.extend(100_u128.to_be_bytes());
        bytes.extend(asset);
        bytes.push(1);
        bytes.extend(1_u64.to_be_bytes());
        bytes.extend(1_u64.to_be_bytes());
        bytes.push(u8::from(frozen));
        bytes.push(0);
        bytes.extend([7_u8; 32]);
        bytes.push(1);
        must(layerx_proof::state::decode_account_value(id, &bytes))
    }

    #[test]
    fn native_principal_selection_uses_committed_ownership_asset_and_unique_namespace() {
        let actor = must(Did::new(b"did:layerx:alice"));
        let asset = [1; 32];
        let main = canonical_account("did:layerx:alice", asset, 1, false);
        let other = canonical_account("did:layerx:other", asset, 1, false);
        let foreign_asset = canonical_account("did:layerx:alice", [2; 32], 14, false);
        let mut accounts = BTreeMap::from([
            (main.account_id, main.clone()),
            (other.account_id, other),
            (foreign_asset.account_id, foreign_asset),
        ]);
        assert_eq!(
            principal_source(&accounts, &actor, 3, asset),
            Ok(main.account_id)
        );
        assert_eq!(
            principal_source(&accounts, &actor, 3, [3; 32]),
            Err(ProgramSourceError::MissingSource)
        );
        let second = canonical_account("did:layerx:alice", asset, 14, true);
        accounts.insert(second.account_id, second.clone());
        assert_eq!(
            principal_source(&accounts, &actor, 3, asset),
            Err(ProgramSourceError::AmbiguousSource)
        );
        accounts.remove(&main.account_id);
        assert_eq!(
            principal_source(&accounts, &actor, 3, asset),
            Err(ProgramSourceError::FrozenSource)
        );
        accounts.remove(&second.account_id);
        assert_eq!(
            principal_source(&accounts, &actor, 3, asset),
            Err(ProgramSourceError::MissingSource)
        );
    }

    #[test]
    fn legacy_lookup_does_not_guess_a_main_account_and_asset_zero_refuses() {
        let actor = must(Did::new(b"did:layerx:alice"));
        let main = canonical_account("did:layerx:alice", [1; 32], 1, false);
        let accounts = BTreeMap::from([(main.account_id, main)]);
        assert_eq!(
            principal_source(&accounts, &actor, 1, [1; 32]),
            Err(ProgramSourceError::MissingSource)
        );
        assert_eq!(
            principal_source(&accounts, &actor, 2, [1; 32]),
            Err(ProgramSourceError::MissingSource)
        );
        assert_eq!(
            principal_source(&accounts, &actor, 3, [0; 32]),
            Err(ProgramSourceError::SourceAsset)
        );
        assert_eq!(
            principal_source(&accounts, &actor, 4, [1; 32]),
            Err(ProgramSourceError::Preparation)
        );
    }

    #[test]
    fn denomination_grouping_retains_counterparty_scope_and_separate_fee_exposure() {
        let row = |destination, amount, source| ResolvedProgramCharge {
            source,
            asset: [1; 32],
            destination,
            maximum_amount: amount,
        };
        let rows = vec![
            row(
                Some([5; 32]),
                20,
                ResolvedProgramSource::Principal { account: [6; 32] },
            ),
            row(
                Some([7; 32]),
                30,
                ResolvedProgramSource::Principal { account: [6; 32] },
            ),
            row(None, 4, ResolvedProgramSource::Fee { account: [6; 32] }),
        ];
        let charges = must(budget_charge_rows(&rows, |charge| {
            let mut limits = vec![super::super::LimitId([1; 16])];
            if charge.destination == Some([5; 32]) {
                limits.push(super::super::LimitId([2; 16]));
            }
            Ok(limits)
        }));
        assert_eq!(charges.len(), 3);
        assert_eq!(
            charges
                .iter()
                .filter(|row| row.kind == super::super::ProgramChargeKind::Principal)
                .map(|row| row.amount)
                .sum::<u128>(),
            50
        );
        assert_eq!(
            charges
                .iter()
                .find(|row| row.kind == super::super::ProgramChargeKind::Fee)
                .map(|row| row.amount),
            Some(4)
        );
        assert_eq!(
            charges
                .iter()
                .find(|row| row.applicable_limits.len() == 2)
                .map(|row| row.amount),
            Some(20)
        );
        assert!(budget_charge_rows(&rows, |_| Ok(Vec::new())).is_err());
    }
    #[test]
    fn program_allocations_keep_destinations_when_grouped_hold_limits_are_identical() {
        let row = |destination, amount| ResolvedProgramCharge {
            source: ResolvedProgramSource::Principal { account: [1; 32] },
            asset: [2; 32],
            destination: Some(destination),
            maximum_amount: amount,
        };
        let input = vec![row([3; 32], 7), row([4; 32], 11), row([3; 32], 2)];
        let rows = must(budget_allocation_rows(&input, |_| {
            Ok(vec![super::super::LimitId([5; 16])])
        }));
        assert_eq!(rows.len(), 2);
        assert_eq!(rows[0].destination, Some([3; 32]));
        assert_eq!(rows[0].maximum_amount, 9);
        assert_eq!(rows[1].destination, Some([4; 32]));
        assert_eq!(rows[1].maximum_amount, 11);
        let grouped = must(super::super::reservations::allocation_charges(&rows));
        assert_eq!(grouped.len(), 1);
        assert_eq!(grouped[0].amount, 20);
        let mut ordinal = 0_u8;
        assert!(budget_allocation_rows(&input, |_| {
            ordinal += 1;
            Ok(vec![super::super::LimitId([ordinal; 16])])
        })
        .is_err());
        assert!(
            budget_allocation_rows(&[row([3; 32], u128::MAX), row([4; 32], 1)], |_| Ok(vec![
                super::super::LimitId([5; 16])
            ]))
            .is_err()
        );
    }

    #[test]
    fn program_fee_sources_resolve_nonzero_fees_from_registry_and_bank() {
        use super::super::fee_policy::tests::{asset_result, FeeFixture};
        let actor = must(Did::new(b"did:layerx:alice"));
        let registry_id = [1; 32];
        let bank_id = layerx_crypto::evm_transaction::transaction_hash(b"upax");
        for (denom, asset) in [
            (format!("layerx/{}", hex::encode(registry_id)), registry_id),
            ("upax".to_owned(), bank_id),
        ] {
            let account = canonical_account("did:layerx:alice", asset, 1, false);
            let accounts = BTreeMap::from([(account.account_id, account.clone())]);
            let fixture = FeeFixture::new(&denom, [7; 32], asset_result(asset, b"PAX", 18, false));
            let charge = must(read_fee_charge(
                &fixture.endpoint,
                &accounts,
                &actor,
                3,
                7,
                [7; 32],
                17,
                None,
            ))
            .expect("nonzero fee charge");
            assert_eq!(
                charge,
                ResolvedProgramCharge {
                    source: ResolvedProgramSource::Fee {
                        account: account.account_id
                    },
                    asset,
                    destination: None,
                    maximum_amount: 17,
                }
            );
            let rows = must(budget_charge_rows(&[charge], |_| {
                Ok(vec![super::super::LimitId([8; 16])])
            }));
            assert_eq!(rows.len(), 1);
            assert_eq!(rows[0].kind, super::super::ProgramChargeKind::Fee);
            assert_eq!(rows[0].source, account.account_id);
            assert_eq!(rows[0].asset, asset);
            assert_eq!(rows[0].amount, 17);
            assert_eq!(
                rows[0].applicable_limits,
                vec![super::super::LimitId([8; 16])]
            );
        }
    }

    #[test]
    fn program_fee_sources_refuse_snapshot_registration_and_ownership_mismatches() {
        use super::super::fee_policy::tests::{asset_result, FeeFixture};
        let actor = must(Did::new(b"did:layerx:alice"));
        let asset = [1; 32];
        let denom = format!("layerx/{}", hex::encode(asset));
        let fixture = FeeFixture::new(&denom, [7; 32], asset_result(asset, b"PAX", 18, false));
        let account = canonical_account("did:layerx:alice", asset, 1, false);
        let accounts = BTreeMap::from([(account.account_id, account.clone())]);
        assert_eq!(
            read_fee_charge(
                &fixture.endpoint,
                &accounts,
                &actor,
                3,
                7,
                [8; 32],
                17,
                None
            ),
            Err(ProgramSourceError::Snapshot)
        );
        assert_eq!(
            read_fee_charge(
                &fixture.endpoint,
                &accounts,
                &actor,
                3,
                7,
                [7; 32],
                17,
                Some([2; 32])
            ),
            Err(ProgramSourceError::Snapshot)
        );
        assert!(must(read_fee_charge(
            &fixture.endpoint,
            &accounts,
            &actor,
            3,
            7,
            [7; 32],
            17,
            Some(asset)
        ))
        .is_some());
        assert_eq!(
            read_fee_charge(
                &fixture.endpoint,
                &BTreeMap::<[u8; 32], CanonicalAccount>::new(),
                &actor,
                3,
                7,
                [7; 32],
                17,
                None
            ),
            Err(ProgramSourceError::MissingSource)
        );
        let foreign = canonical_account("did:layerx:other", asset, 1, false);
        let foreign_accounts = BTreeMap::from([(foreign.account_id, foreign)]);
        assert_eq!(
            read_fee_charge(
                &fixture.endpoint,
                &foreign_accounts,
                &actor,
                3,
                7,
                [7; 32],
                17,
                None
            ),
            Err(ProgramSourceError::MissingSource)
        );
        let frozen = canonical_account("did:layerx:alice", asset, 1, true);
        let frozen_accounts = BTreeMap::from([(frozen.account_id, frozen)]);
        assert_eq!(
            read_fee_charge(
                &fixture.endpoint,
                &frozen_accounts,
                &actor,
                3,
                7,
                [7; 32],
                17,
                None
            ),
            Err(ProgramSourceError::FrozenSource)
        );
        let second = canonical_account("did:layerx:alice", asset, 14, false);
        let ambiguous =
            BTreeMap::from([(account.account_id, account), (second.account_id, second)]);
        assert_eq!(
            read_fee_charge(
                &fixture.endpoint,
                &ambiguous,
                &actor,
                3,
                7,
                [7; 32],
                17,
                None
            ),
            Err(ProgramSourceError::AmbiguousSource)
        );
        let fixture = FeeFixture::new(&denom, [7; 32], asset_result(asset, b"PAX", 18, true));
        assert_eq!(
            read_fee_charge(
                &fixture.endpoint,
                &accounts,
                &actor,
                3,
                7,
                [7; 32],
                17,
                None
            ),
            Err(ProgramSourceError::FrozenSource)
        );
        let fixture = FeeFixture::new(&denom, [7; 32], Vec::new());
        assert_eq!(
            read_fee_charge(
                &fixture.endpoint,
                &accounts,
                &actor,
                3,
                7,
                [7; 32],
                17,
                None
            ),
            Err(ProgramSourceError::FeeUnavailable)
        );
    }

    #[test]
    fn program_zero_fee_does_not_require_a_fee_read() {
        let endpoint = must(layerx_client::sequencer::SequencerEndpoint::new(
            "http://127.0.0.1:1",
            &[],
        ));
        let actor = must(Did::new(b"did:layerx:alice"));
        assert_eq!(
            read_fee_charge(
                &endpoint,
                &BTreeMap::<[u8; 32], CanonicalAccount>::new(),
                &actor,
                3,
                7,
                [7; 32],
                0,
                None
            ),
            Ok(None)
        );
    }
}

fn verify_fee_payer(
    node: &layerx_client::sequencer::SequencerEndpoint,
    prepared: &EvmPrepared,
    actor: &Did,
    state_root: [u8; 32],
) -> Result<(), ProgramSourceError> {
    let height = prepared.observed_head_height;
    let payer = prepared.from;
    let reads = crate::read::CommittedReads::open(node, height)
        .map_err(|_| ProgramSourceError::Snapshot)?;
    let actor_text =
        std::str::from_utf8(actor.as_bytes()).map_err(|_| ProgramSourceError::SourceOwnership)?;
    let public = actor_text
        .strip_prefix("did:layerx:")
        .and_then(|value| layerx_programs::hex::decode(value).ok())
        .filter(|value| value.len() == 32)
        .ok_or(ProgramSourceError::SourceOwnership)?;
    let mut data = layerx_crypto::evm_transaction::transaction_hash(b"getEvmAddrByLayerX(bytes32)")
        [..4]
        .to_vec();
    data.extend_from_slice(&public);
    let binding = reads
        .capabilities(layerx_client::sequencer::ADDR_PRECOMPILE, &data)
        .map_err(|_| ProgramSourceError::SourceOwnership)?;
    let address = binding.canonical_bytes();
    if payer == [0; 20]
        || address.len() != 32
        || address[..12] != [0; 12]
        || address[12..] != payer
        || reads.checkpoint().app_hash != state_root
    {
        return Err(ProgramSourceError::SourceOwnership);
    }
    Ok(())
}

#[cfg(test)]
fn read_fee_charge<A: SourceAccount>(
    node: &layerx_client::sequencer::SequencerEndpoint,
    accounts: &BTreeMap<[u8; 32], A>,
    actor: &Did,
    protocol: u16,
    height: u64,
    state_root: [u8; 32],
    maximum_fee: u128,
    expected_asset: Option<[u8; 32]>,
) -> Result<Option<ResolvedProgramCharge>, ProgramSourceError> {
    read_fee_charge_for(
        node,
        accounts,
        actor,
        &FeeChargeContext {
            protocol,
            height,
            state_root,
            maximum_fee,
            payer: [0; 20],
            expected_asset,
        },
    )
}

/// Resolves native lifecycle debits at the preparation checkpoint, keeping
/// value-account existence separate from its authenticated frozen status.
/// # Errors
/// Returns an error if the prepared transaction, fee payer or committed checkpoint disagrees
/// with the proofs, or a lifecycle debit cannot be resolved from authenticated accounts.
pub fn read_native_lifecycle_budget_sources_call(
    node: &mut layerx_client::sequencer::SequencerEndpoint,
    prepared: &EvmPrepared,
    caps: &CommittedCaps,
    proofs: &BTreeMap<[u8; 32], layerx_proof::state::VerifiedAccountState>,
) -> Result<VerifiedResolvedProgramCharges, ProgramSourceError> {
    validate_prepared(prepared)?;
    let owner_did = committed_owner_did(caps)?;
    verify_fee_payer(node, prepared, &owner_did, caps.state_root())?;
    let actor = caps.did();
    crate::reject_if(
        caps.did() != actor
            || caps.observed_height() != prepared.observed_head_height
            || caps.level() < layerx_types::verify::VerificationLevel::STATE_PROVEN
            || caps.state_root() == [0; 32]
            || node
                .block_number()
                .map_err(|_| ProgramSourceError::Snapshot)?
                != prepared.observed_head_height,
        || ProgramSourceError::Snapshot,
    )?;
    let mut charges = Vec::new();
    match &prepared.disclosure.native_operation {
        Some(
            DisclosedNativeOperation::ProgramDeploy(_)
            | DisclosedNativeOperation::ProgramUpgrade(_),
        ) => {}
        Some(DisclosedNativeOperation::ProgramWindDown(value)) => {
            use layerx_types::program_lifecycle::ProgramWindDownOperation;
            let value = value.native();
            if let ProgramWindDownOperation::Exit { account }
            | ProgramWindDownOperation::BoundedExit { account, .. } = value.operation
            {
                charges.push(native_lifecycle_exit_source(
                    node, prepared, caps, proofs, value, account,
                )?);
            }
        }
        _ => return Err(ProgramSourceError::Unsupported),
    }
    if let Some(fee) = read_fee_charge_for(
        node,
        caps.all_accounts(),
        &owner_did,
        &FeeChargeContext {
            protocol: 3,
            height: prepared.observed_head_height,
            state_root: caps.state_root(),
            maximum_fee: prepared.disclosure.fee_limit,
            payer: prepared.from,
            expected_asset: None,
        },
    )? {
        charges.push(fee);
    }
    Ok(VerifiedResolvedProgramCharges {
        actor,
        owner_did,
        state_root: caps.state_root(),
        batch_number: caps.observed_height(),
        global_sequence: prepared.observed_head_height,
        preparation_digest: Sha256::digest(prepared.call.encode_unsigned()).into(),
        envelope_digest: Sha256::digest(prepared.call.encode_unsigned()).into(),
        charges,
    })
}
fn native_lifecycle_exit_source(
    node: &mut layerx_client::sequencer::SequencerEndpoint,
    prepared: &EvmPrepared,
    caps: &CommittedCaps,
    proofs: &BTreeMap<[u8; 32], layerx_proof::state::VerifiedAccountState>,
    value: layerx_types::program_lifecycle::NativeProgramWindDown<'_>,
    account: [u8; 32],
) -> Result<ResolvedProgramCharge, ProgramSourceError> {
    use layerx_types::program_lifecycle::ProgramWindDownOperation;
    let actor = caps.did();
    let checkpoint = crate::read::CommittedReads::open(node, caps.observed_height())
        .map_err(|_| ProgramSourceError::Snapshot)?;
    crate::reject_if(
        checkpoint.checkpoint().app_hash != caps.state_root(),
        || ProgramSourceError::Snapshot,
    )?;
    let records = super::execution_state::read_program_records_with_account_proofs(
        node,
        checkpoint.checkpoint(),
        value.program_id.bytes(),
        &[account],
        proofs,
    )
    .map_err(|_| ProgramSourceError::ProgramBinding)?;
    let source = records
        .accounts
        .get(&account)
        .ok_or(ProgramSourceError::MissingSource)?;
    let route = records
        .exit_routes
        .get(&account)
        .ok_or(ProgramSourceError::MissingSource)?;
    let lifecycle = records
        .wind_down
        .as_ref()
        .ok_or(ProgramSourceError::ProgramBinding)?;
    let target = caps
        .all_accounts()
        .get(&route.destination)
        .ok_or(ProgramSourceError::MissingSource)?;
    crate::reject_if(
        records.owner != actor
            || source.frozen
            || source.balance == 0
            || source.asset != route.asset
            || source.seed != route.seed
            || source.registered_sequence > prepared.observed_head_height
            || source.program != value.program_id.bytes()
            || target.frozen
            || target.asset_id() != route.asset
            || route.destination == account
            || !matches!(lifecycle.status, 2 | 3)
            || lifecycle.exit_program != value.program_id.bytes()
            || lifecycle.deadline == 0
            || lifecycle.effective_sequence == 0
            || lifecycle.effective_sequence > prepared.observed_head_height
            || matches!(value.operation, ProgramWindDownOperation::BoundedExit { maximum_exit_amount, .. } if maximum_exit_amount == 0 || source.balance > maximum_exit_amount),
        || ProgramSourceError::ProgramBinding,
    )?;
    Ok(ResolvedProgramCharge {
        source: ResolvedProgramSource::Program {
            owner_program: source.program,
            seed: source.seed.clone(),
            account,
        },
        asset: source.asset,
        destination: Some(route.destination),
        maximum_amount: source.balance,
    })
}

/// # Errors
/// Returns an error if the native call, source ownership or snapshot is invalid, a program is
/// unavailable, or charge arithmetic overflows.
pub fn read_program_budget_sources(
    node: &mut layerx_client::sequencer::SequencerEndpoint,
    programs: &mut ProgramOperations,
    prepared: &crate::prepare::EvmPrepared,
    signing: Option<&crate::sign::SigningPreparation>,
    caps: &CommittedCaps,
    now: u64,
    correlation: u64,
) -> Result<VerifiedResolvedProgramCharges, ProgramSourceError> {
    super::native_binding::validate_retained_preparation(prepared, signing)
        .map_err(|_| ProgramSourceError::Preparation)?;
    read_program_budget_sources_call(
        node,
        programs,
        &signing.ok_or(ProgramSourceError::Preparation)?.transaction,
        caps,
        now,
        correlation,
    )
}
/// # Errors
/// Returns an error if the native call, source ownership or snapshot is invalid, a program is
/// unavailable, or charge arithmetic overflows.
pub fn read_native_effect_budget_sources(
    node: &mut layerx_client::sequencer::SequencerEndpoint,
    prepared: &crate::prepare::EvmPrepared,
    signing: Option<&crate::sign::SigningPreparation>,
    caps: &CommittedCaps,
    now: u64,
    correlation: u64,
    profile: crate::capability::NativeAdmissionProfile,
) -> Result<VerifiedResolvedProgramCharges, ProgramSourceError> {
    super::native_binding::validate_retained_preparation(prepared, signing)
        .map_err(|_| ProgramSourceError::Preparation)?;
    read_native_effect_budget_sources_call(
        node,
        &signing.ok_or(ProgramSourceError::Preparation)?.transaction,
        caps,
        now,
        correlation,
        profile,
    )
}
/// # Errors
/// Returns an error if the native call, source ownership or snapshot is invalid, a program is
/// unavailable, or charge arithmetic overflows.
pub fn read_native_lifecycle_budget_sources(
    node: &mut layerx_client::sequencer::SequencerEndpoint,
    prepared: &crate::prepare::EvmPrepared,
    signing: Option<&crate::sign::SigningPreparation>,
    caps: &CommittedCaps,
    proofs: &BTreeMap<[u8; 32], layerx_proof::state::VerifiedAccountState>,
) -> Result<VerifiedResolvedProgramCharges, ProgramSourceError> {
    super::native_binding::validate_retained_preparation(prepared, signing)
        .map_err(|_| ProgramSourceError::Preparation)?;
    read_native_lifecycle_budget_sources_call(
        node,
        &signing.ok_or(ProgramSourceError::Preparation)?.transaction,
        caps,
        proofs,
    )
}

type ProgramChargeKey = (
    [u8; 32],
    [u8; 32],
    super::ProgramChargeKind,
    Vec<super::LimitId>,
);

pub(crate) struct FeeChargeContext {
    pub(crate) protocol: u16,
    pub(crate) height: u64,
    pub(crate) state_root: [u8; 32],
    pub(crate) maximum_fee: u128,
    pub(crate) payer: [u8; 20],
    pub(crate) expected_asset: Option<[u8; 32]>,
}

fn resolve_program_spend_charges(
    programs: &mut ProgramOperations,
    caps: &CommittedCaps,
    now: u64,
    owner_did: &layerx_types::ids::Did,
    plan: &crate::capability::SemanticPlan,
) -> Result<Vec<ResolvedProgramCharge>, ProgramSourceError> {
    let mut charges = Vec::new();
    let mut program_reads = BTreeMap::new();
    let mut gross = BTreeMap::new();
    for bound in plan.program_spend_bounds() {
        crate::reject_if(
            bound.asset == [0; 32] || bound.destination == [0; 32] || bound.maximum_amount == 0,
            || ProgramSourceError::Preparation,
        )?;
        let source = match &bound.source {
            ProgramValueSource::Principal => {
                let account = principal_source(caps.all_accounts(), owner_did, 3, bound.asset)?;
                ResolvedProgramSource::Principal { account }
            }
            ProgramValueSource::Program {
                owner_program,
                seed,
                source_account,
            } => {
                if !program_reads.contains_key(owner_program) {
                    let program = ProgramId::new(*owner_program)
                        .map_err(|_| ProgramSourceError::ProgramBinding)?;
                    let read = programs
                        .current_value_accounts(program, now)
                        .map_err(|_| ProgramSourceError::ProgramUnavailable)?;
                    program_reads.insert(*owner_program, read);
                }
                let read = program_reads
                    .get(owner_program)
                    .ok_or(ProgramSourceError::ProgramBinding)?;
                program_source(
                    caps,
                    read,
                    *owner_program,
                    seed,
                    *source_account,
                    bound.asset,
                )?;
                ResolvedProgramSource::Program {
                    owner_program: *owner_program,
                    seed: seed.clone(),
                    account: *source_account,
                }
            }
        };
        let amount = gross.entry(bound.asset).or_insert(0_u128);
        *amount = amount
            .checked_add(bound.maximum_amount)
            .ok_or(ProgramSourceError::Arithmetic)?;
        charges.push(ResolvedProgramCharge {
            source,
            asset: bound.asset,
            destination: Some(bound.destination),
            maximum_amount: bound.maximum_amount,
        });
    }
    crate::reject_if(&gross != plan.gross_per_asset(), || {
        ProgramSourceError::Preparation
    })?;
    Ok(charges)
}

fn resolve_native_effect_charges(
    prepared: &EvmPrepared,
    caps: &CommittedCaps,
    owner_did: &layerx_types::ids::Did,
    plan: &crate::capability::SemanticPlan,
) -> Result<Vec<ResolvedProgramCharge>, ProgramSourceError> {
    let mut charges = Vec::new();
    let mut gross = BTreeMap::new();
    for effect in plan.effects() {
        match effect {
            crate::capability::Effect::Transfer {
                from,
                to,
                asset,
                amount,
            } => {
                let account = principal_source(caps.all_accounts(), owner_did, 3, *asset)?;
                crate::reject_if(*to == [0; 32] || *amount == 0, || {
                    ProgramSourceError::SourceOwnership
                })?;
                let total = gross.entry(*asset).or_insert(0_u128);
                *total = total
                    .checked_add(*amount)
                    .ok_or(ProgramSourceError::Arithmetic)?;
                if account == *from {
                    charges.push(ResolvedProgramCharge {
                        source: ResolvedProgramSource::Principal { account },
                        asset: *asset,
                        destination: Some(*to),
                        maximum_amount: *amount,
                    });
                } else {
                    crate::reject_if(
                        !crate::capability::native_grant_draw_covers(
                            &prepared.disclosure,
                            account,
                            (*from, *to, *asset, *amount),
                        ),
                        || ProgramSourceError::SourceOwnership,
                    )?;
                }
            }
            crate::capability::Effect::Destruction {
                account,
                asset,
                amount,
            } => {
                let source = principal_source(caps.all_accounts(), owner_did, 3, *asset)?;
                crate::reject_if(source != *account || *amount == 0, || {
                    ProgramSourceError::SourceOwnership
                })?;
                let total = gross.entry(*asset).or_insert(0_u128);
                *total = total
                    .checked_add(*amount)
                    .ok_or(ProgramSourceError::Arithmetic)?;
                charges.push(ResolvedProgramCharge {
                    source: ResolvedProgramSource::Principal { account: source },
                    asset: *asset,
                    destination: Some(
                        layerx_wire::hash::asset_issuance_account_id(asset)
                            .map_err(|_| ProgramSourceError::Preparation)?,
                    ),
                    maximum_amount: *amount,
                });
            }
            crate::capability::Effect::Issuance {
                account,
                asset,
                amount,
            } => {
                let issuance = caps
                    .issuance(*asset)
                    .map_err(|_| ProgramSourceError::SourceOwnership)?;
                crate::reject_if(
                    *account == [0; 32]
                        || *account == issuance.account()
                        || *amount == 0
                        || *amount > issuance.headroom(),
                    || ProgramSourceError::SourceOwnership,
                )?;
                let total = gross.entry(*asset).or_insert(0_u128);
                *total = total
                    .checked_add(*amount)
                    .ok_or(ProgramSourceError::Arithmetic)?;
                charges.push(ResolvedProgramCharge {
                    source: ResolvedProgramSource::Principal {
                        account: issuance.account(),
                    },
                    asset: *asset,
                    destination: Some(*account),
                    maximum_amount: *amount,
                });
            }
            crate::capability::Effect::Authorization { .. } => {}
        }
    }
    crate::reject_if(&gross != plan.gross_per_asset(), || {
        ProgramSourceError::Preparation
    })?;
    Ok(charges)
}
