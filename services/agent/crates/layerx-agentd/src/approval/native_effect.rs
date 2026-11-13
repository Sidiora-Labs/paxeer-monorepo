use crate::budget::ProgramBudgetReservation;
use crate::human::{HumanPeer, HumanSubject};
use crate::policy::native_program::NativeLocalOutcome;
use crate::policy::native_program::{NativeEffectIntent, NativeEffectPolicy};
use crate::prepare::{EvmPrepared, PreparationAuthorization};
use crate::store::{ObjectKind, StorageClass, Store, TenantId, TenantKey};
use layerx_crypto::disclosure::{BudgetStateContext, DisclosedNativeOperation, Disclosure};
use layerx_crypto::evm_transaction::transaction_hash;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use serde::{Deserialize, Serialize};
use sha2::{Digest as _, Sha256};

const PREFIX: &[u8] = b"native-effect-approval-carrier-v1:";
const MAX_CARRIER: usize = 4 * 1024 * 1024;
const MAX_RECORDS: usize = 4096;
pub(crate) const DURABLE_EXTENSION: u16 = 9;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum NativeEffectError {
    Missing,
    Binding,
    Policy,
    Budget,
    Corrupt,
}
#[derive(Clone, Copy, Debug, Serialize, Deserialize, Eq, PartialEq)]
pub(crate) enum NativeEffectApprovalState {
    Awaiting,
    Granted,
    Rejected,
    Expired,
    NotRequired,
}

pub(crate) struct VerifiedUnsignedNativeEffectRelease {
    reservation_id: [u8; 32],
    reservation_digest: [u8; 32],
}
impl VerifiedUnsignedNativeEffectRelease {
    pub(crate) const fn reservation_id(&self) -> [u8; 32] {
        self.reservation_id
    }
    pub(crate) const fn reservation_digest(&self) -> [u8; 32] {
        self.reservation_digest
    }
}

#[derive(Clone, Serialize, Deserialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct NativeEffectApprovalCarrier {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    call_bound: Option<super::call_bound::RetainedNativeCall>,
    version: u8,
    tenant: String,
    principal: String,
    actor: Vec<u8>,
    session: [u8; 32],
    generation: u64,
    preparation: [u8; 32],
    activity_module: u16,
    activity_ordinal: u16,
    canonical_bytes: Vec<u8>,
    sender: [u8; 20],
    disclosure_digest: [u8; 32],
    fee_asset: [u8; 32],
    fee_state_root: [u8; 32],
    fee_observed_sequence: u64,
    budget_context: Option<RetainedBudgetContext>,
    capability: [u8; 32],
    purpose_hash: [u8; 32],
    created_at_sequence: u64,
    created_at_unix_seconds: u64,
    clock_generation: [u8; 16],
    budget_expiry_sequence: u64,
    budget: Vec<u8>,
    policy_source: Vec<u8>,
    requires_approval: bool,
    state: NativeEffectApprovalState,
    terminal: Option<NativeEffectTerminal>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    queued_submission: Option<RetainedQueuedSubmission>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    admission_profile: Option<String>,
}

#[derive(Clone, Serialize, Deserialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
struct RetainedQueuedSubmission {
    submission_id: [u8; 32],
    activity_id: [u8; 32],
    signed_digest: [u8; 32],
}

#[derive(Clone, Serialize, Deserialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
struct NativeEffectTerminal {
    key: [u8; 32],
    principal: String,
    session: [u8; 32],
    generation: u64,
    sequence: u64,
    core_ms: u64,
    grant: bool,
    release_ref: Option<[u8; 32]>,
    submission_ref: Option<[u8; 32]>,
}

#[derive(Clone, Serialize, Deserialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
struct RetainedBudgetContext {
    budget_id: [u8; 32],
    owner: [u8; 32],
    budget_account: [u8; 32],
    asset: [u8; 32],
    source_account: [u8; 32],
    native_source: bool,
    revocation_sequence: u64,
    balance: u128,
    state_digest: [u8; 32],
    observed_head_sequence: u64,
    purpose_hash: [u8; 32],
}
impl RetainedBudgetContext {
    fn from_actual(c: &BudgetStateContext) -> Self {
        Self {
            budget_id: c.budget_id,
            owner: c.owner,
            budget_account: c.budget_account,
            asset: c.asset,
            source_account: c.source_account,
            native_source: c.native_source,
            revocation_sequence: c.revocation_sequence,
            balance: c.balance,
            state_digest: c.state_digest,
            observed_head_sequence: c.observed_head_sequence,
            purpose_hash: c.purpose_hash,
        }
    }
}

impl NativeEffectApprovalCarrier {
    pub(crate) fn bind(input: &NativeEffectBinding<'_>) -> Result<Self, NativeEffectError> {
        let preparation_id = input.validate_binding()?;
        let NativeEffectBinding {
            peer,
            owner,
            prepared,
            origin,
            capability,
            purpose_hash,
            clock,
            policy,
            reservation,
            fee,
            profile,
        } = *input;

        let intent = NativeEffectIntent::from_prepared(owner, prepared, fee, purpose_hash, profile)
            .map_err(|_| refused("approval carrier intent", NativeEffectError::Policy))?;
        let decision = policy.evaluate_local(&intent).map_err(|_| {
            refused(
                "approval carrier policy evaluation",
                NativeEffectError::Policy,
            )
        })?;
        let reading = clock
            .sample(std::time::Duration::from_secs(1))
            .map_err(|_| refused("approval carrier clock sample", NativeEffectError::Binding))?;
        crate::reject_if(reading.generation == [0; 16], || {
            refused(
                "approval carrier clock generation",
                NativeEffectError::Binding,
            )
        })?;
        crate::reject_if(
            reading.unix_milliseconds >= prepared.disclosure.expiry.not_after,
            || {
                refused(
                    "approval carrier clock reading past bound",
                    NativeEffectError::Binding,
                )
            },
        )?;
        let context = match &prepared.disclosure.native_operation {
            Some(DisclosedNativeOperation::BudgetFund(v)) => Some(v.context),
            Some(DisclosedNativeOperation::BudgetDefund(v)) => Some(v.context),
            Some(DisclosedNativeOperation::BudgetRevoke(v)) => Some(v.context),
            _ => None,
        };
        let required = decision.outcome == NativeLocalOutcome::ApprovalRequired;
        let version = 3;
        let result = Self {
            call_bound: None,
            version,
            tenant: peer.tenant.clone(),
            principal: peer.principal.clone(),
            actor: owner.as_bytes().to_vec(),
            session: origin.session.session_id.0,
            generation: origin.generation,
            preparation: preparation_id,
            activity_module: prepared.disclosure.activity_type.module() as u16,
            activity_ordinal: prepared.disclosure.activity_type.ordinal(),
            canonical_bytes: prepared.call.encode_unsigned(),
            sender: prepared.from,
            disclosure_digest: prepared.disclosure_digest,
            fee_asset: fee.value.asset.asset_id,
            fee_state_root: fee.state_root,
            fee_observed_sequence: fee.observed_sequence,
            budget_context: context.as_ref().map(RetainedBudgetContext::from_actual),
            capability: capability.0,
            purpose_hash,
            created_at_sequence: prepared.observed_head_height,
            created_at_unix_seconds: reading.unix_seconds(),
            clock_generation: reading.generation,
            budget_expiry_sequence: reservation.expiry_sequence,
            budget: reservation.encode().map_err(|_| {
                refused(
                    "approval carrier budget encoding",
                    NativeEffectError::Budget,
                )
            })?,
            policy_source: policy.source().to_vec(),
            requires_approval: required,
            state: if required {
                NativeEffectApprovalState::Awaiting
            } else {
                NativeEffectApprovalState::NotRequired
            },
            terminal: None,
            queued_submission: None,
            admission_profile: profile.label().map(str::to_owned),
        };
        result
            .validate()
            .map_err(|error| refused("approval carrier validation", error))?;
        Ok(result)
    }

    fn registry(&self) -> Result<ModuleRegistry, NativeEffectError> {
        let module =
            ModuleId::from_u16(self.activity_module).map_err(|_| NativeEffectError::Corrupt)?;
        if module == ModuleId::Programs {
            return Err(NativeEffectError::Binding);
        }
        let activity = ActivityType::new(module, self.activity_ordinal)
            .map_err(|_| NativeEffectError::Corrupt)?;
        ModuleRegistry::new(&[
            ModuleRegistration::new(module, &[activity]).map_err(|_| NativeEffectError::Corrupt)?
        ])
        .map_err(|_| NativeEffectError::Corrupt)
    }
    pub(crate) fn disclosure(&self) -> Result<Disclosure, NativeEffectError> {
        let call = layerx_crypto::evm_transaction::decode_unsigned(&self.canonical_bytes)
            .map_err(|_| NativeEffectError::Corrupt)?;
        let disclosure = layerx_crypto::disclosure::call::bind_call(&call, self.sender)
            .map_err(|_| NativeEffectError::Corrupt)?;
        let context = match &disclosure.native_operation {
            Some(DisclosedNativeOperation::BudgetFund(v)) => Some(v.context),
            Some(DisclosedNativeOperation::BudgetDefund(v)) => Some(v.context),
            Some(DisclosedNativeOperation::BudgetRevoke(v)) => Some(v.context),
            _ => None,
        };
        if disclosure
            .signing_digest()
            .map_err(|_| NativeEffectError::Corrupt)?
            != self.disclosure_digest
            || disclosure.actor.as_slice() != self.sender
            || disclosure.activity_type.module() as u16 != self.activity_module
            || disclosure.activity_type.ordinal() != self.activity_ordinal
            || context.as_ref().map(RetainedBudgetContext::from_actual) != self.budget_context
        {
            return Err(NativeEffectError::Binding);
        }
        Ok(disclosure)
    }
    fn validate(&self) -> Result<(), NativeEffectError> {
        if super::call_bound::validate(self).map_err(|_| NativeEffectError::Binding)? {
            return Ok(());
        }
        crate::reject_if(self.version != 3, || NativeEffectError::Corrupt)?;
        layerx_types::ids::Did::new(&self.actor).map_err(|_| NativeEffectError::Corrupt)?;
        let not_after = self.disclosure()?.expiry.not_after;
        let tenant = TenantId::new(self.tenant.clone()).map_err(|_| NativeEffectError::Corrupt)?;
        let _ = tenant;
        crate::reject_if(
            !matches!(
                (self.version, self.admission_profile.as_deref()),
                (
                    3,
                    None | Some(crate::capability::NATIVE_REGISTRATION_PROFILE)
                )
            ) || self.principal.is_empty()
                || self.principal.len() > 255
                || self.session == [0; 32]
                || self.generation == 0
                || self.fee_asset == [0; 32]
                || self.fee_state_root == [0; 32]
                || self.fee_observed_sequence != self.created_at_sequence
                || self.capability == [0; 32]
                || self.purpose_hash == [0; 32]
                || self.clock_generation == [0; 16]
                || self.canonical_bytes.is_empty()
                || self.canonical_bytes.len() > MAX_CARRIER
                || self.created_at_sequence >= self.budget_expiry_sequence
                || self.created_at_unix_seconds == 0
                || self
                    .created_at_unix_seconds
                    .checked_mul(1000)
                    .is_none_or(|time| time >= not_after),
            || NativeEffectError::Corrupt,
        )?;
        let disclosure = self.disclosure()?;
        let decoded = layerx_crypto::evm_transaction::decode_unsigned(&self.canonical_bytes)
            .map_err(|_| NativeEffectError::Corrupt)?;
        if let crate::capability::NativeAdmissionProfile::Registration { fee_asset } =
            self.profile()?
        {
            let commitment = crate::capability::native_registration_commitment(
                &disclosure,
                3,
                u32::try_from(decoded.chain_id).map_err(|_| NativeEffectError::Binding)?,
                self.preparation,
                fee_asset,
            )
            .map_err(|_| NativeEffectError::Binding)?;
            crate::capability::derive_native_registration(&disclosure, fee_asset)
                .map_err(|_| NativeEffectError::Binding)?;
            crate::reject_if(
                (self.activity_module, self.activity_ordinal) != (ModuleId::Asset as u16, 1)
                    || self.budget_context.is_some()
                    || commitment != self.purpose_hash,
                || NativeEffectError::Binding,
            )?;
        }
        let budget = self.budget()?;
        let owner =
            layerx_types::ids::Did::new(&self.actor).map_err(|_| NativeEffectError::Corrupt)?;
        let owner_id = layerx_wire::hash::did_id_for_protocol(&owner, 3)
            .map_err(|_| NativeEffectError::Corrupt)?;
        crate::reject_if(
            budget.allocation_actor() != Some(owner_id)
                || budget.id != self.preparation
                || budget.expiry_sequence != self.budget_expiry_sequence
                || budget.core_deadline.is_none_or(|time| time.0 != not_after)
                || budget.encode().map_err(|_| NativeEffectError::Budget)? != self.budget,
            || NativeEffectError::Binding,
        )?;
        NativeEffectPolicy::load(&self.policy_source).map_err(|_| NativeEffectError::Policy)?;
        self.validate_terminal()
    }
    fn validate_terminal(&self) -> Result<(), NativeEffectError> {
        if let Some(queued) = &self.queued_submission {
            crate::reject_if(
                queued.submission_id == [0; 32]
                    || queued.activity_id == [0; 32]
                    || queued.signed_digest == [0; 32]
                    || !matches!(
                        self.state,
                        NativeEffectApprovalState::Granted | NativeEffectApprovalState::NotRequired
                    )
                    || self
                        .terminal
                        .as_ref()
                        .is_some_and(|t| t.submission_ref != Some(queued.submission_id)),
                || NativeEffectError::Binding,
            )?;
        } else {
            crate::reject_if(
                self.terminal
                    .as_ref()
                    .is_some_and(|t| t.submission_ref.is_some()),
                || NativeEffectError::Binding,
            )?;
        }
        match (self.requires_approval, self.state, &self.terminal) {
            (true, NativeEffectApprovalState::Awaiting, None)
            | (false, NativeEffectApprovalState::NotRequired, None)
            | (_, NativeEffectApprovalState::Expired, None) => {}
            (_, state, Some(t))
                if matches!(
                    state,
                    NativeEffectApprovalState::Granted
                        | NativeEffectApprovalState::Rejected
                        | NativeEffectApprovalState::Expired
                ) && t.key != [0; 32]
                    && !t.principal.is_empty()
                    && t.session == self.session
                    && t.generation == self.generation
                    && t.sequence >= self.created_at_sequence
                    && (t.release_ref.is_some()
                        == (state == NativeEffectApprovalState::Granted))
                    && (t.submission_ref.is_none()
                        || state == NativeEffectApprovalState::Granted)
                    && (state == NativeEffectApprovalState::Expired
                        || t.grant == (state == NativeEffectApprovalState::Granted)) => {}
            _ => return Err(NativeEffectError::Corrupt),
        }
        Ok(())
    }

    pub(crate) fn encoded(&self) -> Result<Vec<u8>, NativeEffectError> {
        self.validate()?;
        let bytes = serde_json::to_vec(self).map_err(|_| NativeEffectError::Corrupt)?;
        if bytes.len() > MAX_CARRIER {
            return Err(NativeEffectError::Corrupt);
        }
        Ok(bytes)
    }
    pub(crate) fn companion(&self) -> Result<(TenantKey, Vec<u8>), NativeEffectError> {
        let key = TenantKey::new(
            TenantId::new(self.tenant.clone()).map_err(|_| NativeEffectError::Corrupt)?,
            ObjectKind::PreparedActivity,
            [PREFIX, self.preparation.as_slice()].concat(),
        )
        .map_err(|_| NativeEffectError::Corrupt)?;
        Ok((key, self.encoded()?))
    }
    pub(crate) fn immutable_hold_bytes(&self) -> Result<Vec<u8>, NativeEffectError> {
        let mut held = self.clone();
        held.terminal = None;
        held.queued_submission = None;
        held.state = if held.requires_approval {
            NativeEffectApprovalState::Awaiting
        } else {
            NativeEffectApprovalState::NotRequired
        };
        held.encoded()
    }
    pub(crate) fn held_digest(&self) -> Result<[u8; 32], NativeEffectError> {
        if let Some(call) = &self.call_bound {
            return call.held_digest().map_err(|_| NativeEffectError::Binding);
        }
        Ok(Sha256::digest(self.immutable_hold_bytes()?).into())
    }
    pub(crate) fn budget(&self) -> Result<ProgramBudgetReservation, NativeEffectError> {
        ProgramBudgetReservation::decode(&self.budget).map_err(|_| NativeEffectError::Budget)
    }
    pub(crate) fn preparation_id(&self) -> [u8; 32] {
        self.preparation
    }
    pub(crate) fn actor(&self) -> &[u8] {
        &self.actor
    }
    pub(crate) fn fee_asset(&self) -> [u8; 32] {
        self.fee_asset
    }
    pub(crate) fn profile(
        &self,
    ) -> Result<crate::capability::NativeAdmissionProfile, NativeEffectError> {
        match (self.version, self.admission_profile.as_deref()) {
            (3, None) => Ok(crate::capability::NativeAdmissionProfile::Effect),
            (3, Some(crate::capability::NATIVE_REGISTRATION_PROFILE)) => {
                Ok(crate::capability::NativeAdmissionProfile::Registration {
                    fee_asset: self.fee_asset,
                })
            }
            _ => Err(NativeEffectError::Binding),
        }
    }
    pub(crate) fn session_id(&self) -> [u8; 32] {
        self.session
    }

    pub(crate) fn state(&self) -> NativeEffectApprovalState {
        self.state
    }
    pub(crate) fn requires_approval(&self) -> bool {
        self.requires_approval
    }
    pub(crate) fn created_at_sequence(&self) -> u64 {
        self.created_at_sequence
    }
    pub(crate) fn budget_expiry_sequence(&self) -> u64 {
        self.budget_expiry_sequence
    }
    pub(crate) fn created_at_unix_seconds(&self) -> u64 {
        self.created_at_unix_seconds
    }
    pub(crate) fn activity_expires_at_unix_milliseconds(&self) -> Result<u64, NativeEffectError> {
        Ok(self.disclosure()?.expiry.not_after)
    }
    pub(crate) fn submission_ref(&self) -> Option<[u8; 32]> {
        self.queued_submission.as_ref().map(|q| q.submission_id)
    }
    pub(crate) fn release_ref(&self) -> Option<[u8; 32]> {
        self.terminal.as_ref().and_then(|t| t.release_ref)
    }
    pub(crate) fn canonical_bytes(&self) -> &[u8] {
        &self.canonical_bytes
    }

    pub(crate) fn read(
        store: &Store,
        tenant: &TenantId,
        id: [u8; 32],
    ) -> Result<Self, NativeEffectError> {
        let key = TenantKey::new(
            tenant.clone(),
            ObjectKind::PreparedActivity,
            [PREFIX, id.as_slice()].concat(),
        )
        .map_err(|_| NativeEffectError::Corrupt)?;
        let value = store.get(&key).ok_or(NativeEffectError::Missing)?;
        if value.class() != StorageClass::LocalOnly || value.bytes().len() > MAX_CARRIER {
            return Err(NativeEffectError::Corrupt);
        }
        let held: Self =
            serde_json::from_slice(value.bytes()).map_err(|_| NativeEffectError::Corrupt)?;
        if held.encoded()?.as_slice() != value.bytes()
            || held.preparation != id
            || held.tenant != tenant.as_str()
        {
            return Err(NativeEffectError::Binding);
        }
        let key = crate::prepare::DurablePreparation::store_key(tenant, id)
            .map_err(|_| NativeEffectError::Corrupt)?;
        let value = store.get(&key).ok_or(NativeEffectError::Missing)?;
        if value.class() != StorageClass::LocalOnly {
            return Err(NativeEffectError::Corrupt);
        }
        let durable = crate::prepare::DurablePreparation::decode(tenant.clone(), value.bytes())
            .map_err(|_| NativeEffectError::Corrupt)?;
        if durable.preparation_id != id
            || durable.session_id != held.session
            || durable.generation != held.generation
            || durable.not_after != held.disclosure()?.expiry.not_after
            || durable.extensions.get(&6) != Some(&held.budget)
            || durable
                .extensions
                .get(&DURABLE_EXTENSION)
                .map(Vec::as_slice)
                != Some(held.held_digest()?.as_slice())
        {
            return Err(NativeEffectError::Binding);
        }
        let call = layerx_crypto::evm_transaction::decode_unsigned(&held.canonical_bytes)
            .map_err(|_| NativeEffectError::Corrupt)?;
        if durable.payload_hash != layerx_crypto::evm_transaction::transaction_hash(&call.data)
            || durable.tenant != *tenant
        {
            return Err(NativeEffectError::Binding);
        }
        if let Some(queued) = &held.queued_submission {
            held.verify_owned_queue(store, &durable, queued, None)?;
        }
        let budget = held.budget()?;
        if budget.allocations().is_none()
            && (durable.holds.len() != budget.holds.len()
                || budget.holds.iter().any(|h| {
                    !durable
                        .holds
                        .iter()
                        .any(|(saved, _)| saved == &h.reservation)
                }))
        {
            return Err(NativeEffectError::Binding);
        }
        Ok(held)
    }
    fn verify_owned_queue(
        &self,
        store: &Store,
        durable: &crate::prepare::DurablePreparation,
        queued: &RetainedQueuedSubmission,
        expected: Option<&crate::sign::VerifiedSubmission>,
    ) -> Result<(), NativeEffectError> {
        let tenant = TenantId::new(self.tenant.clone()).map_err(|_| NativeEffectError::Binding)?;
        if queued.submission_id == [0; 32]
            || queued.activity_id == [0; 32]
            || queued.signed_digest == [0; 32]
            || !matches!(
                self.state,
                NativeEffectApprovalState::Granted | NativeEffectApprovalState::NotRequired
            )
            || durable.activity_id != Some(queued.activity_id)
            || durable.preparation_id != self.preparation
            || durable.session_id != self.session
            || durable.generation != self.generation
        {
            return Err(NativeEffectError::Binding);
        }
        for kind in [ObjectKind::Outbox, ObjectKind::PreparedActivity] {
            let key = TenantKey::new(tenant.clone(), kind, queued.submission_id.to_vec())
                .map_err(|_| NativeEffectError::Corrupt)?;
            if store.get(&key).ok_or(NativeEffectError::Missing)?.class() != StorageClass::LocalOnly
            {
                return Err(NativeEffectError::Binding);
            }
        }
        let mut outbox = crate::outbox::Outbox::default();
        outbox
            .restore(store, tenant.clone(), queued.submission_id)
            .map_err(|_| NativeEffectError::Corrupt)?;
        let origin = outbox
            .origin(queued.submission_id)
            .map_err(|_| NativeEffectError::Corrupt)?
            .ok_or(NativeEffectError::Binding)?;
        let status = outbox
            .status(queued.submission_id)
            .ok_or(NativeEffectError::Missing)?;
        let exact = outbox
            .exact_signed_bytes(queued.submission_id)
            .map_err(|_| NativeEffectError::Corrupt)?;
        let signed = layerx_wire::historical::activity::decode_signed(exact, &self.registry()?)
            .map_err(|_| NativeEffectError::Corrupt)?;
        if origin.session.tenant != tenant
            || origin.session.session_id.0 != self.session
            || origin.generation != self.generation
            || status.submission_id != queued.submission_id
            || status.activity_id != queued.activity_id
            || queued.submission_id != transaction_hash(exact)
            || matches!(
                status.state,
                crate::outbox::SubmissionState::Prepared | crate::outbox::SubmissionState::Signed
            )
            || Sha256::digest(exact).as_slice() != queued.signed_digest.as_slice()
            || layerx_wire::historical::activity::encode_signed(&signed)
                .map_err(|_| NativeEffectError::Corrupt)?
                != exact
            || layerx_wire::historical::activity::encode_unsigned(&signed)
                .map_err(|_| NativeEffectError::Corrupt)?
                != self.canonical_bytes
            || layerx_wire::historical::hash::activity_id(&signed)
                .map_err(|_| NativeEffectError::Corrupt)?
                != queued.activity_id
            || durable
                .extensions
                .get(&crate::prepare::EXTENSION_IDEMPOTENCY)
                .map(Vec::as_slice)
                != Some(signed.idempotency_key().as_slice())
        {
            return Err(NativeEffectError::Binding);
        }
        if let Some(expected) = expected {
            if expected.exact_bytes() != exact
                || expected.activity_id() != queued.activity_id
                || expected.idempotency_key() != signed.idempotency_key()
                || durable
                    .signed_bytes()
                    .map_err(|_| NativeEffectError::Corrupt)?
                    .as_deref()
                    != Some(exact)
            {
                return Err(NativeEffectError::Binding);
            }
        }
        Ok(())
    }

    pub(crate) fn stage_retained_queue(&self, store: &Store) -> Result<Self, NativeEffectError> {
        let tenant = TenantId::new(self.tenant.clone()).map_err(|_| NativeEffectError::Binding)?;
        let retained = Self::read(store, &tenant, self.preparation)?;
        if retained.encoded()? != self.encoded()? {
            return Err(NativeEffectError::Binding);
        }
        let key = crate::prepare::DurablePreparation::store_key(&tenant, self.preparation)
            .map_err(|_| NativeEffectError::Corrupt)?;
        let raw = store.get(&key).ok_or(NativeEffectError::Missing)?;
        let durable = crate::prepare::DurablePreparation::decode(tenant.clone(), raw.bytes())
            .map_err(|_| NativeEffectError::Corrupt)?;
        let signed = durable
            .signed_bytes()
            .map_err(|_| NativeEffectError::Corrupt)?
            .ok_or(NativeEffectError::Missing)?;
        let id = transaction_hash(&signed);
        let mut outbox = crate::outbox::Outbox::default();
        outbox
            .restore(store, tenant, id)
            .map_err(|_| NativeEffectError::Missing)?;
        let status = outbox.status(id).ok_or(NativeEffectError::Missing)?;
        let exact = outbox
            .exact_signed_bytes(id)
            .map_err(|_| NativeEffectError::Corrupt)?;
        let queued = RetainedQueuedSubmission {
            submission_id: id,
            activity_id: status.activity_id,
            signed_digest: Sha256::digest(exact).into(),
        };
        self.verify_owned_queue(store, &durable, &queued, None)?;
        if self
            .queued_submission
            .as_ref()
            .is_some_and(|old| old != &queued)
        {
            return Err(NativeEffectError::Binding);
        }
        let mut staged = self.clone();
        staged.queued_submission = Some(queued);
        if let Some(t) = &mut staged.terminal {
            t.submission_ref = Some(id);
        }
        staged.validate()?;
        Ok(staged)
    }
    pub(crate) fn read_for_human(
        store: &Store,
        peer: &HumanPeer,
        id: [u8; 32],
    ) -> Result<Self, NativeEffectError> {
        let tenant = human_tenant(peer)?;
        let held = Self::read(store, &tenant, id)?;
        if held.principal != peer.principal
            || !held.owner_matches(
                store,
                &tenant,
                peer.subject.as_ref().ok_or(NativeEffectError::Binding)?,
            )?
        {
            return Err(NativeEffectError::Binding);
        }
        Ok(held)
    }
    pub(crate) fn list_for_human(
        store: &Store,
        peer: &HumanPeer,
    ) -> Result<Vec<Self>, NativeEffectError> {
        let tenant = human_tenant(peer)?;
        let subject = peer.subject.as_ref().ok_or(NativeEffectError::Binding)?;
        let ids = store
            .list_object_ids(&tenant, ObjectKind::PreparedActivity)
            .into_iter()
            .filter(|id| id.starts_with(PREFIX))
            .collect::<Vec<_>>();
        if ids.len() > MAX_RECORDS {
            return Err(NativeEffectError::Corrupt);
        }
        let mut result = Vec::new();
        for id in ids {
            let id = id[PREFIX.len()..]
                .try_into()
                .map_err(|_| NativeEffectError::Corrupt)?;
            let held = Self::read(store, &tenant, id)?;
            if held.principal == peer.principal && held.owner_matches(store, &tenant, subject)? {
                result.push(held);
            }
        }
        result.sort_by_key(Self::preparation_id);
        Ok(result)
    }
    fn owner_matches(
        &self,
        store: &Store,
        tenant: &TenantId,
        subject: &HumanSubject,
    ) -> Result<bool, NativeEffectError> {
        if self.actor == subject.owner.as_bytes() {
            return Ok(true);
        }
        crate::managed_agent::authenticates_native_owner(
            store,
            tenant,
            &subject.owner,
            &subject.account,
            &self.actor,
        )
        .map_err(|_| NativeEffectError::Binding)
    }
    pub(crate) fn stage_decision(
        &self,
        peer: &HumanPeer,
        held_digest: [u8; 32],
        key: [u8; 32],
        grant: bool,
        observation: &crate::protocol_evidence::AuthenticatedCoreTime,
        session: &crate::session::SessionRecord,
    ) -> Result<Self, NativeEffectError> {
        let tenant = human_tenant(peer)?;
        let sequence = observation.through_sequence();
        let core_ms = observation.observed_core_ms();
        if tenant.as_str() != self.tenant
            || peer.principal != self.principal
            || self.held_digest()? != held_digest
            || key == [0; 32]
            || !session.open
            || session.request.tenant != tenant
            || session.request.session_id.0 != self.session
            || session.request.agent.as_bytes() != self.actor
            || session.generation != self.generation
            || sequence >= session.request.expiry_sequence
            || session
                .request
                .expiry_seconds
                .is_some_and(|expiry| core_ms / 1000 >= expiry)
            || sequence < self.created_at_sequence
        {
            return Err(NativeEffectError::Binding);
        }
        if let Some(t) = &self.terminal {
            if t.key == key && t.principal == peer.principal && t.grant == grant {
                return Ok(self.clone());
            }
            return Err(NativeEffectError::Binding);
        }
        if self.state != NativeEffectApprovalState::Awaiting || !self.requires_approval {
            return Err(NativeEffectError::Binding);
        }
        let expired = sequence >= self.budget_expiry_sequence
            || core_ms >= self.disclosure()?.expiry.not_after;
        let mut result = self.clone();
        result.state = if expired {
            NativeEffectApprovalState::Expired
        } else if grant {
            NativeEffectApprovalState::Granted
        } else {
            NativeEffectApprovalState::Rejected
        };
        let granted = result.state == NativeEffectApprovalState::Granted;
        let release_ref = if granted {
            let mut h = Sha256::new();
            h.update(b"layerx/native-effect-release/v1\0");
            h.update(held_digest);
            h.update(self.preparation);
            Some(h.finalize().into())
        } else {
            None
        };
        result.terminal = Some(NativeEffectTerminal {
            key,
            principal: peer.principal.clone(),
            session: self.session,
            generation: self.generation,
            sequence,
            core_ms,
            grant,
            release_ref,
            submission_ref: None,
        });
        result.validate()?;
        Ok(result)
    }

    pub(crate) fn unsigned_release_proof(
        &self,
        store: &Store,
        durable: &crate::prepare::DurablePreparation,
    ) -> Result<VerifiedUnsignedNativeEffectRelease, NativeEffectError> {
        let tenant = TenantId::new(self.tenant.clone()).map_err(|_| NativeEffectError::Binding)?;
        let retained = Self::read(store, &tenant, self.preparation)?;
        let key = crate::prepare::DurablePreparation::store_key(&tenant, self.preparation)
            .map_err(|_| NativeEffectError::Binding)?;
        let raw = store.get(&key).ok_or(NativeEffectError::Missing)?;
        let actual = crate::prepare::DurablePreparation::decode(tenant.clone(), raw.bytes())
            .map_err(|_| NativeEffectError::Corrupt)?;
        if actual != *durable
            || raw.class() != StorageClass::LocalOnly
            || retained.held_digest()? != self.held_digest()?
            || !matches!(
                self.state,
                NativeEffectApprovalState::Rejected | NativeEffectApprovalState::Expired
            )
            || durable.state != crate::prepare::LifecycleState::Prepared
            || durable.activity_id.is_some()
            || durable
                .signed_bytes()
                .map_err(|_| NativeEffectError::Corrupt)?
                .is_some()
            || durable.session_id != self.session
            || durable.generation != self.generation
            || durable.extensions.get(&6) != Some(&self.budget)
            || durable
                .extensions
                .get(&DURABLE_EXTENSION)
                .map(Vec::as_slice)
                != Some(self.held_digest()?.as_slice())
        {
            return Err(NativeEffectError::Binding);
        }
        let record = self.budget()?;
        Ok(VerifiedUnsignedNativeEffectRelease {
            reservation_id: self.preparation,
            reservation_digest: record
                .settlement_binding()
                .map_err(|_| NativeEffectError::Budget)?,
        })
    }
    pub(crate) fn authorize_human_submit(
        &self,
        peer: &HumanPeer,
        prepared: &EvmPrepared,
        origin: &PreparationAuthorization,
        submission_ref: Option<[u8; 32]>,
        sequence: u64,
        core_ms: u64,
    ) -> Result<(), NativeEffectError> {
        if peer.tenant != self.tenant
            || peer.principal != self.principal
            || origin.session.tenant.as_str() != self.tenant
            || origin.session.session_id.0 != self.session
            || origin.generation != self.generation
            || prepared.call.encode_unsigned() != self.canonical_bytes
            || prepared.disclosure_digest != self.disclosure_digest
            || sequence < self.created_at_sequence
            || sequence >= self.budget_expiry_sequence
            || core_ms >= self.disclosure()?.expiry.not_after
            || !matches!(
                self.state,
                NativeEffectApprovalState::Granted | NativeEffectApprovalState::NotRequired
            )
            || (self.requires_approval
                && (submission_ref.is_none() || submission_ref != self.release_ref()))
            || (!self.requires_approval && submission_ref.is_some())
        {
            return Err(NativeEffectError::Binding);
        }
        Ok(())
    }
    pub(crate) fn read_id(
        store: &Store,
        context: &crate::agent_rpc_peer::RpcOwnerContext<'_>,
        id: [u8; 32],
    ) -> Result<Self, NativeEffectError> {
        let held = Self::read(store, &context.principal().tenant, id)?;
        if held.actor != context.principal().agent.as_bytes()
            || held.principal != context.peer().principal
        {
            return Err(NativeEffectError::Binding);
        }
        Ok(held)
    }

    pub(crate) fn authorize_submit(
        &self,
        context: &crate::agent_rpc_peer::RpcOwnerContext<'_>,
        prepared: &EvmPrepared,
        release_ref: Option<[u8; 32]>,
        sequence: u64,
        core_ms: u64,
    ) -> Result<(), NativeEffectError> {
        let origin = context.permit().preparation_authorization();
        if self.actor != context.principal().agent.as_bytes() {
            return Err(NativeEffectError::Binding);
        }
        self.authorize_human_submit(
            context.peer(),
            prepared,
            &origin,
            release_ref,
            sequence,
            core_ms,
        )
    }

    pub(crate) fn restore_prepared(
        &self,
        registry: &ModuleRegistry,
    ) -> Result<EvmPrepared, NativeEffectError> {
        let _ = registry;
        let call = layerx_crypto::evm_transaction::decode_unsigned(&self.canonical_bytes)
            .map_err(|_| NativeEffectError::Corrupt)?;
        let disclosure = self.disclosure()?;
        let prepared = EvmPrepared {
            signing_hash: call.signing_hash(),
            call,
            from: self.sender,
            disclosure,
            disclosure_digest: self.disclosure_digest,
            observed_head_height: self.created_at_sequence,
        };
        verify_call_binding(&prepared).map_err(|_| NativeEffectError::Binding)?;
        Ok(prepared)
    }
}
fn human_tenant(peer: &HumanPeer) -> Result<TenantId, NativeEffectError> {
    let s = peer.subject.as_ref().ok_or(NativeEffectError::Binding)?;
    let expected = layerx_identity_binding::subject_namespace(&s.transport_tenant, &peer.principal)
        .map_err(|_| NativeEffectError::Binding)?;
    let account = layerx_types::account::AccountId::parse(&s.account)
        .map_err(|_| NativeEffectError::Binding)?;
    layerx_types::ids::Did::new(s.owner.as_bytes()).map_err(|_| NativeEffectError::Binding)?;
    if peer.uid == 0
        || s.transport_principal.is_empty()
        || expected != peer.tenant
        || account.canonical() != s.account
        || !s.account.starts_with(&format!("agent:{}:", s.owner))
    {
        return Err(NativeEffectError::Binding);
    }
    TenantId::new(peer.tenant.clone()).map_err(|_| NativeEffectError::Binding)
}

impl super::call_bound::CallCarrier for NativeEffectApprovalCarrier {
    const PROGRAM: bool = false;
    const PREFIX: &'static [u8] = PREFIX;
    fn call(&self) -> Option<&super::call_bound::RetainedNativeCall> {
        self.call_bound.as_ref()
    }
    fn stage_unsigned_release<'a>(
        &self,
        budgets: &'a crate::budget::BudgetLimiter,
    ) -> Result<crate::budget::StagedRelease<'a>, super::call_bound::CallApprovalError> {
        use super::call_bound::{CallApprovalError, CallApprovalState};
        let call = self.call_bound.as_ref().ok_or(CallApprovalError::Version)?;
        if call.signed.is_some()
            || !matches!(
                call.state()?,
                CallApprovalState::Rejected | CallApprovalState::Expired
            )
        {
            return Err(CallApprovalError::Binding);
        }
        let budget = call.budget()?;
        let result = if budget.allocations().is_some() {
            let proof = VerifiedUnsignedNativeEffectRelease {
                reservation_id: budget.id,
                reservation_digest: budget
                    .settlement_binding()
                    .map_err(|_| CallApprovalError::Budget)?,
            };
            budgets.stage_native_effect_unsigned_release(&budget, &proof)
        } else {
            crate::budget::stage_release(
                budgets,
                budget.id,
                crate::budget::ReleaseKind::Failed,
                call.decision
                    .as_ref()
                    .ok_or(CallApprovalError::Binding)?
                    .sequence,
            )
        };
        result.map_err(|_| CallApprovalError::Budget)
    }
    fn from_call(
        call: super::call_bound::RetainedNativeCall,
    ) -> Result<Self, super::call_bound::CallApprovalError> {
        use super::call_bound::{CallApprovalError, CallApprovalState};
        let prepared = call.restore()?;
        let budget = call.budget()?;
        let state = match call.state()? {
            CallApprovalState::Awaiting => NativeEffectApprovalState::Awaiting,
            CallApprovalState::Expired => NativeEffectApprovalState::Expired,
            CallApprovalState::Rejected => NativeEffectApprovalState::Rejected,
            _ => NativeEffectApprovalState::Granted,
        };
        let release_ref = if state == NativeEffectApprovalState::Granted {
            Some(
                Sha256::new()
                    .chain_update(b"layerx/native-effect-release/v1\0")
                    .chain_update(call.held_digest()?)
                    .chain_update(call.preparation)
                    .finalize()
                    .into(),
            )
        } else {
            None
        };
        Ok(Self {
            version: 3,
            tenant: call.tenant.clone(),
            principal: call.principal.clone(),
            actor: call.owner_did.as_bytes().to_vec(),
            session: call.session,
            generation: call.generation,
            preparation: call.preparation,
            activity_module: prepared.disclosure.activity_type.module() as u16,
            activity_ordinal: prepared.disclosure.activity_type.ordinal(),
            canonical_bytes: call.canonical_bytes.clone(),
            sender: call.sender,
            disclosure_digest: call.disclosure_digest,
            fee_asset: budget
                .charges
                .iter()
                .find(|charge| charge.kind == crate::budget::ProgramChargeKind::Fee)
                .ok_or(CallApprovalError::Budget)?
                .asset,
            fee_state_root: [0; 32],
            fee_observed_sequence: call.observed_height,
            budget_context: None,
            capability: [0; 32],
            purpose_hash: [0; 32],
            created_at_sequence: call.observed_height,
            created_at_unix_seconds: call.admitted_at_ms / 1000,
            clock_generation: [0; 16],
            budget_expiry_sequence: budget.expiry_sequence,
            budget: call.budget.clone(),
            policy_source: Vec::new(),
            requires_approval: true,
            state,
            terminal: call.decision.as_ref().map(|decision| NativeEffectTerminal {
                key: decision.key,
                principal: call.principal.clone(),
                session: call.session,
                generation: call.generation,
                sequence: decision.sequence,
                core_ms: decision.core_ms,
                grant: decision.grant,
                release_ref,
                submission_ref: call
                    .signed
                    .as_ref()
                    .filter(|_| call.queued)
                    .map(|bytes| transaction_hash(bytes)),
            }),
            queued_submission: call.signed.as_ref().filter(|_| call.queued).map(|bytes| {
                RetainedQueuedSubmission {
                    submission_id: transaction_hash(bytes),
                    activity_id: transaction_hash(bytes),
                    signed_digest: Sha256::digest(bytes).into(),
                }
            }),
            admission_profile: None,
            call_bound: Some(call),
        })
    }
}
super::call_bound::impl_call_approval!(NativeEffectApprovalCarrier);

fn verify_call_binding(
    prepared: &EvmPrepared,
) -> Result<(), crate::prepare::evm::CallBindingError> {
    crate::prepare::verify_call_binding(prepared)
}

pub(crate) struct NativeEffectBinding<'a> {
    pub(crate) peer: &'a HumanPeer,
    pub(crate) owner: &'a layerx_types::ids::Did,
    pub(crate) prepared: &'a EvmPrepared,
    pub(crate) origin: &'a PreparationAuthorization,
    pub(crate) capability: crate::capability::CapabilityId,
    pub(crate) purpose_hash: [u8; 32],
    pub(crate) clock: &'a dyn layerx_types::clock::Clock,
    pub(crate) policy: &'a NativeEffectPolicy,
    pub(crate) reservation: &'a ProgramBudgetReservation,
    pub(crate) fee: &'a layerx_client::rpc::payments::CommittedSnapshot<
        layerx_client::rpc::payments::NativeFeePolicy,
    >,
    pub(crate) profile: crate::capability::NativeAdmissionProfile,
}

fn refused(site: &str, error: NativeEffectError) -> NativeEffectError {
    eprintln!("layerx-agentd: native send prepare refused at {site}");
    error
}

impl NativeEffectBinding<'_> {
    fn validate_binding(&self) -> Result<[u8; 32], NativeEffectError> {
        let Self {
            peer,
            prepared,
            origin,
            capability,
            purpose_hash,
            reservation,
            fee,
            profile,
            ..
        } = *self;
        crate::prepare::verify_call_binding(prepared).map_err(|_| {
            refused(
                "approval carrier disclosure binding",
                NativeEffectError::Binding,
            )
        })?;
        let preparation_id = Sha256::digest(prepared.call.encode_unsigned()).into();
        let tenant =
            human_tenant(peer).map_err(|error| refused("approval carrier tenant", error))?;
        for (invalid, site) in [
            (
                origin.session.tenant != tenant,
                "approval carrier origin tenant",
            ),
            (origin.generation == 0, "approval carrier generation"),
            (
                origin.session.session_id.0 == [0; 32],
                "approval carrier session",
            ),
            (capability.0 == [0; 32], "approval carrier zero capability"),
            (
                purpose_hash == [0; 32],
                "approval carrier zero purpose commitment",
            ),
            (
                prepared.disclosure.activity_type.module() == ModuleId::Programs,
                "approval carrier programs module",
            ),
            (
                reservation.id != preparation_id,
                "approval carrier reservation identity",
            ),
            (
                reservation.expiry_sequence <= prepared.observed_head_height,
                "approval carrier reservation expiry sequence",
            ),
            (
                reservation
                    .core_deadline
                    .is_none_or(|deadline| deadline.0 != prepared.disclosure.expiry.not_after),
                "approval carrier reservation deadline",
            ),
        ] {
            crate::reject_if(invalid, || refused(site, NativeEffectError::Binding))?;
        }
        if let crate::capability::NativeAdmissionProfile::Registration { fee_asset } = profile {
            crate::reject_if(fee_asset != fee.value.asset.asset_id, || {
                refused(
                    "approval carrier registration fee asset",
                    NativeEffectError::Binding,
                )
            })?;
        }
        Ok(preparation_id)
    }
}
