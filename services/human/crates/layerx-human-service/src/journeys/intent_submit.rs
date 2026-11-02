//! Execution of a signed unified plan through the existing movement journeys.

use std::collections::BTreeSet;
use std::fmt::{Display, Formatter};

use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_sdk::Client as AgentClient;
use layerx_types::payload::ModuleRegistry;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

use crate::custody::{CustodySigner, KeyId, Operation};
use crate::notify::JourneyId;
use crate::store::PrincipalScope;
use crate::trace::TraceId;

use super::{
    AgentBoundary, ExecutableShape, JourneyEngine, JourneyError, JourneyLeg, JourneyPlan,
    JourneyState, JourneyStatus, LegMechanism, Mechanism, RequiredAuthority, RouteRequest,
    RouteResolver, UnifiedPlan,
};

/// The submit classifier is the planner's executable shape contract.
pub use super::ExecutableShape as IntentShape;

const JOURNEY_DOMAIN: &[u8] = b"layerx-human-intent-journey/v1";
const SELF_RELATIONSHIP: &str = "self";
const JOURNEY_ID_BYTES: usize = 13;
const DRIVE_STEPS: usize = 6;
const TEXT_LIMIT: usize = 256;

/// Returns the signing authority label the plan operation publishes for a leg.
#[must_use]
pub const fn authority_label(authority: RequiredAuthority) -> &'static str {
    match authority {
        RequiredAuthority::PaxeerWalletKey => "paxeer-wallet-key",
        RequiredAuthority::AccountOwner => "account-owner",
        RequiredAuthority::Allowance(_) => "allowance",
    }
}

/// One submitted leg binding exactly as the contract carries it.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct IntentLegBinding {
    pub leg_index: usize,
    pub action_key: [u8; 32],
    pub actor: String,
    pub authority: String,
    pub relationship: String,
    /// The sender's EVM pending-tag nonce, retaining the contract field name.
    pub account_sequence: u64,
    pub not_before: u64,
    pub not_after: u64,
    pub fee_limit: u128,
    pub fee_currency: String,
}

impl IntentLegBinding {
    /// Decodes one `IntentLegBinding` document.
    ///
    /// # Errors
    ///
    /// Refuses a missing, mistyped or out-of-range field.
    pub fn from_json(value: &Value) -> Result<Self, SubmitRefusal> {
        let leg_index = usize::try_from(unsigned(value, "leg_index")?)
            .map_err(|_| SubmitRefusal::Malformed { field: "leg_index" })?;
        let fee = value
            .get("fee_limit")
            .ok_or(SubmitRefusal::Malformed { field: "fee_limit" })?;
        let fee_limit = text(fee, "amount", "fee_limit")?
            .parse::<u128>()
            .map_err(|_| SubmitRefusal::Malformed { field: "fee_limit" })?;
        Ok(Self {
            leg_index,
            action_key: hex32(text(value, "action_key", "action_key")?).ok_or(
                SubmitRefusal::Malformed {
                    field: "action_key",
                },
            )?,
            actor: text(value, "actor", "actor")?.to_owned(),
            authority: text(value, "authority", "authority")?.to_owned(),
            relationship: text(value, "relationship", "relationship")?.to_owned(),
            account_sequence: unsigned(value, "account_sequence")?,
            not_before: unsigned(value, "not_before")?,
            not_after: unsigned(value, "not_after")?,
            fee_limit,
            fee_currency: text(fee, "currency", "fee_limit")?.to_owned(),
        })
    }
}

/// The signed submission of one plan: its digest, the signed digest and one
/// binding per leg.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SubmitPlanRequest {
    pub plan_digest: [u8; 32],
    pub signed_digest: [u8; 32],
    pub bindings: Vec<IntentLegBinding>,
}

impl SubmitPlanRequest {
    /// Decodes the `SubmitPlanRequest` fields of a submission body.
    ///
    /// # Errors
    ///
    /// Refuses a missing or malformed digest and any malformed binding.
    pub fn from_json(body: &Value) -> Result<Self, SubmitRefusal> {
        let bindings = body
            .get("bindings")
            .and_then(Value::as_array)
            .ok_or(SubmitRefusal::Malformed { field: "bindings" })?
            .iter()
            .map(IntentLegBinding::from_json)
            .collect::<Result<Vec<_>, _>>()?;
        Ok(Self {
            plan_digest: hex32(text(body, "plan_digest", "plan_digest")?).ok_or(
                SubmitRefusal::Malformed {
                    field: "plan_digest",
                },
            )?,
            signed_digest: hex32(text(body, "signed_digest", "signed_digest")?).ok_or(
                SubmitRefusal::Malformed {
                    field: "signed_digest",
                },
            )?,
            bindings,
        })
    }
}

/// What the service itself observed for the signed-in owner at submission
/// time. The caller's bindings are checked against this, never trusted.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BindingExpectation {
    pub actor: AgentDid,
    pub authority: AuthorityRef,
    /// The pending-tag nonce returned by the agent runtime's ACCOUNT_SEQUENCE answer.
    pub account_sequence: u64,
    pub currency: String,
    pub now: u64,
}

/// Verifies the signed digest and every leg binding against the plan the
/// service re-planned at submission time.
///
/// # Errors
///
/// Refuses a digest that no longer matches, a binding count that differs
/// from the leg count, a binding naming another leg, action key, actor,
/// authority, relationship or currency, a fee limit outside the leg fee and
/// the intent ceiling, a nonce differing from the sender's pending nonce,
/// repeated sender/nonce pairs and a validity window that does not contain
/// the submission time.
pub fn verify_bindings(
    plan: &UnifiedPlan,
    request: &SubmitPlanRequest,
    expectation: &BindingExpectation,
) -> Result<(), SubmitRefusal> {
    if request.plan_digest != plan.digest() {
        return Err(SubmitRefusal::PlanDigestMismatch);
    }
    if request.signed_digest != plan.digest() {
        return Err(SubmitRefusal::SignedDigestMismatch);
    }
    if request.bindings.len() != plan.legs().len() {
        return Err(SubmitRefusal::UnboundLegs {
            expected: plan.legs().len(),
            bound: request.bindings.len(),
        });
    }
    let constraints = plan.intent().constraints();
    let mut sender_nonces = BTreeSet::new();
    for (leg, binding) in plan.legs().iter().zip(&request.bindings) {
        let index = leg.index();
        let action_key = plan
            .action_key(index)
            .map_err(|_| SubmitRefusal::LegMismatch { index })?;
        if binding.leg_index != index
            || binding.action_key != action_key
            || binding.authority != authority_label(leg.authority())
            || binding.actor != expectation.actor.as_str()
            || binding.relationship != SELF_RELATIONSHIP
            || binding.fee_currency != expectation.currency
            || binding.not_before > binding.not_after
            || binding.not_after > constraints.deadline().value()
        {
            return Err(SubmitRefusal::LegMismatch { index });
        }
        if binding.fee_limit < leg.fee() || binding.fee_limit > constraints.max_fee() {
            return Err(SubmitRefusal::FeeLimitMismatch { index });
        }
        if binding.account_sequence
            != expectation
                .account_sequence
                .checked_add(
                    u64::try_from(index).map_err(|_| SubmitRefusal::StaleSequence { index })?,
                )
                .ok_or(SubmitRefusal::StaleSequence { index })?
            || !sender_nonces.insert((binding.actor.as_str(), binding.account_sequence))
        {
            return Err(SubmitRefusal::StaleSequence { index });
        }
        if expectation.now < binding.not_before || expectation.now > binding.not_after {
            return Err(SubmitRefusal::WindowExpired { index });
        }
    }
    Ok(())
}

/// Derives the stable journey identifier of one submission, so a repeated
/// submission under the same idempotency key names the original journey.
///
/// # Errors
///
/// Refuses an empty or oversized idempotency key.
pub fn intent_journey_id(
    idempotency_key: &str,
    plan_digest: [u8; 32],
) -> Result<JourneyId, SubmitRefusal> {
    if idempotency_key.is_empty() || idempotency_key.len() > TEXT_LIMIT {
        return Err(SubmitRefusal::Malformed {
            field: "Idempotency-Key",
        });
    }
    let length = u16::try_from(idempotency_key.len()).map_err(|_| SubmitRefusal::Malformed {
        field: "Idempotency-Key",
    })?;
    let mut digest = Sha256::new();
    digest.update(JOURNEY_DOMAIN);
    digest.update(length.to_be_bytes());
    digest.update(idempotency_key.as_bytes());
    digest.update(plan_digest);
    let derived: [u8; 32] = digest.finalize().into();
    JourneyId::new(format!("jrn_{}", hex(&derived[..JOURNEY_ID_BYTES])))
        .map_err(|_| SubmitRefusal::JourneyUnavailable)
}

/// Everything a kernel intent journey needs besides the plan and its bindings.
pub struct KernelStart<'a> {
    pub routes: &'a [RouteRequest],
    pub journey_id: JourneyId,
    pub custody_key: KeyId,
    pub registry: &'a ModuleRegistry,
}

/// Persists a verified plan's signed EIP-1559 legs with their nonces and hashes.
/// Each signed transaction must match its exact authorization and submitted binding.
///
/// # Errors
///
/// Refuses any binding defect, a plan of another shape, a route that does not
/// resolve to its planned leg, a submission already used under another
/// idempotency key, and a journey the engine refuses to persist.
pub fn start_kernel_journey(
    scope: &mut PrincipalScope<'_>,
    plan: &UnifiedPlan,
    request: &SubmitPlanRequest,
    expectation: &BindingExpectation,
    journey_id: JourneyId,
    transactions: &[crate::custody::EvmAction],
) -> Result<IntentSubmission, SubmitRefusal> {
    verify_bindings(plan, request, expectation)?;
    if plan.executable_shape() != ExecutableShape::Kernel || transactions.len() != plan.legs().len()
    {
        return Err(SubmitRefusal::UnsupportedPlan);
    }
    let mut legs = Vec::with_capacity(transactions.len());
    for (index, (action, binding)) in transactions.iter().zip(&request.bindings).enumerate() {
        let authorization = &action.authorization;
        let (call, signature) =
            layerx_crypto::evm_transaction::decode_signed(&action.raw_transaction)
                .map_err(|_| SubmitRefusal::LegMismatch { index })?;
        let hash = layerx_crypto::evm_transaction::transaction_hash(&action.raw_transaction);
        if authorization.principal != scope.principal().as_str()
            || authorization.tenant != scope.tenant().as_str()
            || authorization.plan_id != plan.digest()
            || authorization.action_key != binding.action_key
            || authorization.not_before != binding.not_before
            || authorization.not_after != binding.not_after
            || authorization.transaction.nonce != binding.account_sequence
            || call.nonce != binding.account_sequence
            || call.to != authorization.transaction.to
            || call.data != authorization.transaction.calldata
            || call.chain_id != authorization.transaction.chain_id
            || call.gas_limit != authorization.transaction.gas_limit
            || call.max_fee_per_gas != u128::from(authorization.transaction.max_fee_per_gas)
            || call.max_priority_fee_per_gas
                != u128::from(authorization.transaction.max_priority_fee_per_gas)
            || authorization.transaction.value != [0; 32]
            || action.transaction_hash != Some(hash)
            || layerx_crypto::evm_transaction::recover_sender(&call, &signature)
                .map_err(|_| SubmitRefusal::LegMismatch { index })?
                != authorization.wallet
        {
            return Err(SubmitRefusal::LegMismatch { index });
        }
        legs.push(
            json!({"index": index, "nonce": call.nonce, "tx_hash": format!("0x{}", hex(&hash)),
            "action_key": hex(&binding.action_key), "submitted": action.acknowledged,
            "raw_transaction": action.raw_transaction}),
        );
    }
    let record = json!({"journey_id": journey_id.as_str(), "plan_digest": hex(&plan.digest()),
        "state": "processing", "state_copy_key": "journey.state.processing", "legs": legs,
        "started_at": crate::time::rfc3339(expectation.now),
        "updated_at": crate::time::rfc3339(expectation.now)});
    let key = crate::store::RowKey::new(format!("intent-evm-{}", journey_id.as_str()))
        .map_err(|_| SubmitRefusal::JourneyUnavailable)?;
    let bytes = serde_json::to_vec(&record).map_err(|_| SubmitRefusal::JourneyUnavailable)?;
    if let Some(previous) = scope.get(crate::store::Table::Journeys, &key) {
        let retained: Value = serde_json::from_slice(previous.bytes())
            .map_err(|_| SubmitRefusal::JourneyUnavailable)?;
        let retained_legs = retained["legs"]
            .as_array()
            .ok_or(SubmitRefusal::JourneyUnavailable)?;
        let new_legs = record["legs"]
            .as_array()
            .ok_or(SubmitRefusal::JourneyUnavailable)?;
        if retained["journey_id"] != record["journey_id"]
            || retained["plan_digest"] != record["plan_digest"]
            || retained_legs.len() != new_legs.len()
            || retained_legs.iter().zip(new_legs).any(|(old, new)| {
                ["index", "nonce", "tx_hash", "action_key", "raw_transaction"]
                    .iter()
                    .any(|field| old[field] != new[field])
            })
        {
            return Err(SubmitRefusal::AlreadySubmitted);
        }
    } else {
        scope
            .put(crate::store::Table::Journeys, key, expectation.now, bytes)
            .map_err(|_| SubmitRefusal::JourneyUnavailable)?;
    }
    Ok(IntentSubmission {
        journey_id,
        plan_digest: plan.digest(),
        state: "processing",
    })
}

pub fn start_native_kernel_journey(
    scope: &mut PrincipalScope<'_>,
    plan: &UnifiedPlan,
    request: &SubmitPlanRequest,
    expectation: &BindingExpectation,
    start: KernelStart<'_>,
) -> Result<JourneyEngine, SubmitRefusal> {
    start_kernel_profile(scope, plan, request, expectation, start, true)
}

fn start_kernel_profile(
    scope: &mut PrincipalScope<'_>,
    plan: &UnifiedPlan,
    request: &SubmitPlanRequest,
    expectation: &BindingExpectation,
    start: KernelStart<'_>,
    native: bool,
) -> Result<JourneyEngine, SubmitRefusal> {
    verify_bindings(plan, request, expectation)?;
    if plan.executable_shape() != ExecutableShape::Kernel {
        return Err(SubmitRefusal::UnsupportedPlan);
    }
    let legs = kernel_journey_legs(plan, request, expectation, start.routes)?;
    let journey_plan = JourneyPlan::new(
        start.journey_id,
        plan.journey_kind(),
        plan.digest(),
        start.custody_key,
        Operation::ProtocolMutation,
        legs,
    )
    .map_err(|_| SubmitRefusal::JourneyUnavailable)?;
    let engine = if native {
        if plan
            .legs()
            .iter()
            .any(|leg| leg.mechanism() != LegMechanism::Protocol(Mechanism::Send))
        {
            return Err(SubmitRefusal::UnsupportedPlan);
        }
        JourneyEngine::start_native(scope, &journey_plan, start.registry, expectation.now)
    } else {
        JourneyEngine::start(scope, &journey_plan, start.registry, expectation.now)
    };
    engine.map_err(|error| match error {
        JourneyError::IdempotencyConflict => SubmitRefusal::AlreadySubmitted,
        _ => SubmitRefusal::JourneyUnavailable,
    })
}

/// Resolves the kernel legs the plan's shape runs through the native journey
/// engine. Each route must resolve to exactly the planned mechanism, term,
/// endpoints, asset and amount of the leg at the same position.
fn kernel_journey_legs(
    plan: &UnifiedPlan,
    request: &SubmitPlanRequest,
    expectation: &BindingExpectation,
    routes: &[RouteRequest],
) -> Result<Vec<JourneyLeg>, SubmitRefusal> {
    let (first, end) = plan.executable_shape().kernel_legs(plan.legs().len());
    let planned = plan.legs().get(first..end).unwrap_or_default();
    let bound = request.bindings.get(first..end).unwrap_or_default();
    if planned.is_empty() || routes.len() != planned.len() || bound.len() != planned.len() {
        return Err(SubmitRefusal::UnboundLegs {
            expected: planned.len(),
            bound: routes.len(),
        });
    }
    let mut legs = Vec::with_capacity(planned.len());
    for ((leg, binding), route) in planned.iter().zip(bound).zip(routes) {
        let index = leg.index();
        if !ExecutableShape::is_kernel(leg.mechanism()) || binding.leg_index != index {
            return Err(SubmitRefusal::LegMismatch { index });
        }
        if &route.source != leg.source()
            || &route.destination != leg.destination()
            || route.asset != leg.asset()
            || route.amount != leg.amount()
        {
            return Err(SubmitRefusal::RouteMismatch { index });
        }
        let resolved =
            RouteResolver::resolve(route).map_err(|_| SubmitRefusal::RouteMismatch { index })?;
        let [single] = resolved.legs() else {
            return Err(SubmitRefusal::RouteMismatch { index });
        };
        if Some(single.mechanism()) != leg.mechanism().protocol() || single.term() != leg.term() {
            return Err(SubmitRefusal::RouteMismatch { index });
        }
        legs.push(
            JourneyLeg::new(
                single.intent().clone(),
                binding.action_key,
                expectation.actor.clone(),
                expectation.authority.clone(),
                binding.account_sequence,
                binding.account_sequence,
                binding.not_before,
                binding.not_after,
                binding.fee_limit,
            )
            .map_err(|_| SubmitRefusal::LegMismatch { index })?,
        );
    }
    Ok(legs)
}

/// The service boundaries one kernel journey advances through.
pub struct IntentDriver<'a> {
    pub agent_contract: &'a AgentClient,
    pub custody: &'a CustodySigner,
    pub registry: &'a ModuleRegistry,
    pub trace: &'a TraceId,
    pub now: u64,
}

/// Advances a kernel intent journey through getting ready, signing and
/// sending, stopping once it is processing or terminal. Later progress is
/// read and continued like every other engine journey.
///
/// # Errors
///
/// Returns the engine's typed agent, custody, receipt and storage failures.
pub async fn drive_intent_journey(
    engine: &mut JourneyEngine,
    scope: &mut PrincipalScope<'_>,
    agent: &mut dyn AgentBoundary,
    driver: &IntentDriver<'_>,
) -> Result<JourneyStatus, JourneyError> {
    let mut status = engine.status()?;
    for _ in 0..DRIVE_STEPS {
        if !matches!(
            status.state(),
            JourneyState::GettingReady | JourneyState::Sending
        ) {
            break;
        }
        status = engine
            .advance(
                scope,
                driver.agent_contract,
                agent,
                driver.custody,
                driver.registry,
                driver.trace,
                driver.now,
            )
            .await?;
    }
    Ok(status)
}

/// The `IntentSubmission` the submit operation returns.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct IntentSubmission {
    journey_id: JourneyId,
    plan_digest: [u8; 32],
    state: &'static str,
}

impl IntentSubmission {
    /// Describes a kernel intent journey at its current state.
    #[must_use]
    pub fn from_kernel(status: &JourneyStatus, plan_digest: [u8; 32]) -> Self {
        let state = match status.state() {
            JourneyState::GettingReady => "getting-ready",
            JourneyState::Sending => "sending",
            JourneyState::Processing => "processing",
            JourneyState::StillChecking => "still-checking",
            JourneyState::Done => "done",
            JourneyState::Refused => "refused",
            JourneyState::Closed => "closed",
        };
        Self {
            journey_id: status.journey_id().clone(),
            plan_digest,
            state,
        }
    }

    #[must_use]
    pub const fn journey_id(&self) -> &JourneyId {
        &self.journey_id
    }

    #[must_use]
    pub const fn plan_digest(&self) -> [u8; 32] {
        self.plan_digest
    }

    #[must_use]
    pub const fn state(&self) -> &'static str {
        self.state
    }

    #[must_use]
    pub fn state_copy_key(&self) -> String {
        format!("status.{}", self.state)
    }

    #[must_use]
    pub fn to_json(&self) -> Value {
        json!({
            "journey_id": self.journey_id.as_str(),
            "plan_digest": hex(&self.plan_digest),
            "state": self.state,
            "state_copy_key": self.state_copy_key(),
        })
    }
}

/// Typed refusal of an intent submission. No variant creates a journey.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SubmitRefusal {
    Malformed { field: &'static str },
    PlanDigestMismatch,
    SignedDigestMismatch,
    UnboundLegs { expected: usize, bound: usize },
    LegMismatch { index: usize },
    FeeLimitMismatch { index: usize },
    StaleSequence { index: usize },
    WindowExpired { index: usize },
    RouteMismatch { index: usize },
    DepositMismatch,
    WithdrawalMismatch,
    UnsupportedPlan,
    WalletNotBound,
    AlreadySubmitted,
    JourneyUnavailable,
}

impl SubmitRefusal {
    /// Returns the HTTP status the contract pairs with the refusal code.
    #[must_use]
    pub const fn status(self) -> u16 {
        match self {
            Self::Malformed { .. } => 400,
            Self::StaleSequence { .. }
            | Self::WindowExpired { .. }
            | Self::AlreadySubmitted
            | Self::WalletNotBound => 409,
            Self::JourneyUnavailable => 503,
            Self::PlanDigestMismatch
            | Self::SignedDigestMismatch
            | Self::UnboundLegs { .. }
            | Self::LegMismatch { .. }
            | Self::FeeLimitMismatch { .. }
            | Self::RouteMismatch { .. }
            | Self::DepositMismatch
            | Self::WithdrawalMismatch
            | Self::UnsupportedPlan => 403,
        }
    }

    /// Returns the stable `ErrorCode` from the errors contract.
    #[must_use]
    pub const fn code(self) -> &'static str {
        match self {
            Self::Malformed { .. } => "invalid-request",
            Self::StaleSequence { .. } | Self::AlreadySubmitted => "conflict",
            Self::WindowExpired { .. } => "quote-expired",
            Self::WalletNotBound => "wallet-not-bound",
            Self::JourneyUnavailable => "unavailable",
            Self::PlanDigestMismatch
            | Self::SignedDigestMismatch
            | Self::UnboundLegs { .. }
            | Self::LegMismatch { .. }
            | Self::FeeLimitMismatch { .. }
            | Self::RouteMismatch { .. }
            | Self::DepositMismatch
            | Self::WithdrawalMismatch
            | Self::UnsupportedPlan => "forbidden",
        }
    }

    /// Returns the copy-catalog key naming the human message.
    #[must_use]
    pub const fn copy_key(self) -> &'static str {
        match self {
            Self::Malformed { .. } => "error.request.invalid",
            Self::StaleSequence { .. } | Self::WindowExpired { .. } | Self::AlreadySubmitted => {
                "error.move.quote-expired"
            }
            Self::WalletNotBound => "error.wallet.not-bound",
            Self::JourneyUnavailable => "error.service.unavailable",
            Self::PlanDigestMismatch
            | Self::SignedDigestMismatch
            | Self::UnboundLegs { .. }
            | Self::LegMismatch { .. }
            | Self::FeeLimitMismatch { .. }
            | Self::RouteMismatch { .. }
            | Self::DepositMismatch
            | Self::WithdrawalMismatch
            | Self::UnsupportedPlan => "error.request.forbidden",
        }
    }

    /// Returns the retriability classification.
    #[must_use]
    pub const fn retry(self) -> &'static str {
        match self {
            Self::StaleSequence { .. }
            | Self::WindowExpired { .. }
            | Self::AlreadySubmitted
            | Self::WalletNotBound => "structural",
            Self::JourneyUnavailable => "retriable",
            Self::Malformed { .. }
            | Self::PlanDigestMismatch
            | Self::SignedDigestMismatch
            | Self::UnboundLegs { .. }
            | Self::LegMismatch { .. }
            | Self::FeeLimitMismatch { .. }
            | Self::RouteMismatch { .. }
            | Self::DepositMismatch
            | Self::WithdrawalMismatch
            | Self::UnsupportedPlan => "final",
        }
    }

    /// Returns the request field a malformed submission named.
    #[must_use]
    pub const fn field(self) -> Option<&'static str> {
        match self {
            Self::Malformed { field } => Some(field),
            _ => None,
        }
    }
}

impl Display for SubmitRefusal {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Malformed { field } => {
                write!(formatter, "the submission field {field} is malformed")
            }
            Self::PlanDigestMismatch => {
                formatter.write_str("the re-planned digest differs from the submitted plan digest")
            }
            Self::SignedDigestMismatch => {
                formatter.write_str("the signed digest differs from the re-planned digest")
            }
            Self::UnboundLegs { expected, bound } => write!(
                formatter,
                "the plan has {expected} legs and {bound} were bound"
            ),
            Self::LegMismatch { index } => {
                write!(
                    formatter,
                    "the binding for leg {index} does not match the plan"
                )
            }
            Self::FeeLimitMismatch { index } => write!(
                formatter,
                "the fee limit for leg {index} is outside the leg fee and the intent ceiling"
            ),
            Self::StaleSequence { index } => {
                write!(
                    formatter,
                    "the account sequence bound to leg {index} is stale"
                )
            }
            Self::WindowExpired { index } => write!(
                formatter,
                "the validity window bound to leg {index} does not contain the submission time"
            ),
            Self::RouteMismatch { index } => {
                write!(
                    formatter,
                    "leg {index} does not resolve to its planned route"
                )
            }
            Self::DepositMismatch => {
                formatter.write_str("the deposit plan differs from the signed plan")
            }
            Self::WithdrawalMismatch => {
                formatter.write_str("the withdrawal plan differs from the signed plan")
            }
            Self::UnsupportedPlan => {
                formatter.write_str("no movement journey executes this plan shape")
            }
            Self::WalletNotBound => formatter.write_str("no Paxeer wallet is bound"),
            Self::AlreadySubmitted => {
                formatter.write_str("this plan was already submitted under another key")
            }
            Self::JourneyUnavailable => formatter.write_str("the journey could not be created"),
        }
    }
}

impl std::error::Error for SubmitRefusal {}

fn text<'a>(value: &'a Value, name: &str, field: &'static str) -> Result<&'a str, SubmitRefusal> {
    value
        .get(name)
        .and_then(Value::as_str)
        .filter(|text| !text.is_empty() && text.len() <= TEXT_LIMIT)
        .ok_or(SubmitRefusal::Malformed { field })
}

fn unsigned(value: &Value, field: &'static str) -> Result<u64, SubmitRefusal> {
    value
        .get(field)
        .and_then(Value::as_u64)
        .ok_or(SubmitRefusal::Malformed { field })
}

fn hex32(text: &str) -> Option<[u8; 32]> {
    if text.len() != 64 {
        return None;
    }
    let nibble = |byte: u8| match byte {
        b'0'..=b'9' => Some(byte - b'0'),
        b'a'..=b'f' => Some(byte - b'a' + 10),
        _ => None,
    };
    let mut out = [0_u8; 32];
    for (slot, pair) in out.iter_mut().zip(text.as_bytes().chunks_exact(2)) {
        *slot = (nibble(pair[0])? << 4) | nibble(pair[1])?;
    }
    Some(out)
}

fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut out = String::with_capacity(bytes.len().saturating_mul(2));
    for byte in bytes {
        out.push(char::from(DIGITS[usize::from(byte >> 4)]));
        out.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    out
}
