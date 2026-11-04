use std::collections::{BTreeMap, BTreeSet};

use layerx_agentd::budget::{LocalAccounting, ProtocolBudgetState, ReconciliationState};
use layerx_agentd::capability::timed::TimedCapability;
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::policy::{
    evaluate_timed_admission_with_context, Decision, DecisionReason, Outcome, PolicyRegistry,
    PolicyRequest, PolicySet, RuleConstraints, SequenceWindow,
};
use layerx_agentd::protocol_evidence::{AuthenticatedCoreTime, EvidenceAuthority};
use layerx_agentd::read::ServedCheckpoint;
use layerx_agentd::session::{SessionId, SessionRecord};
use layerx_agentd::store::TenantId;
use layerx_types::ids::Did;

#[path = "../../../tests/policy/adversarial.rs"]
pub mod adversarial;
pub mod captured;

pub use adversarial::{required_blocked_constraints, ConstraintDimension};

pub struct HarnessEvidence {
    pub budget: ReconciliationState,
    pub observed: AuthenticatedCoreTime,
}

impl HarnessEvidence {
    pub fn decode(checkpoint: &ServedCheckpoint, result: &[u8]) -> Result<Self, String> {
        let validator = checkpoint
            .validators
            .first()
            .ok_or("no checkpoint validator")?;
        let observed = EvidenceAuthority::pinned_to_handshake(3, 125, validator.public_key)
            .map_err(|error| format!("budget authority: {error:?}"))?
            .pin_committed_validators("125".to_owned(), checkpoint.validators.clone())
            .map_err(|error| format!("budget validators: {error:?}"))?
            .authenticate_committed_time(checkpoint)
            .map_err(|error| format!("budget observation: {error:?}"))?;
        let protocol =
            ProtocolBudgetState::decode_get_budget(captured::BUDGET_ID, checkpoint.height, result)
                .map_err(|error| format!("getBudget: {error:?}"))?;
        let record = protocol.record();
        let now = observed.observed_core_ms();
        if record.closed
            || record.revoked
            || now < record.period_start
            || now >= record.window_end_sequence()
            || now >= record.expiry
        {
            return Err("budget is not current at the observed checkpoint".to_owned());
        }
        let mut local = LocalAccounting {
            consumed: 0,
            window_start_sequence: 0,
            last_receipt: None,
        };
        let budget = protocol
            .reconcile(&mut local, &[])
            .map_err(|error| format!("budget reconciliation: {error:?}"))?;
        Ok(Self { budget, observed })
    }

    pub fn read(config: &NodeRpcConfig, checkpoint: &ServedCheckpoint) -> Result<Self, String> {
        if config.chain_id != 125 || checkpoint.header.chain_id != "125" {
            return Err("budget network mismatch".to_owned());
        }
        let rpc = NodeRpc::connect(config).map_err(|error| format!("budget RPC: {error:?}"))?;
        let mut address = [0; 20];
        address[18..].copy_from_slice(&[0x10, 0x1b]);
        let result = rpc
            .call_at(checkpoint.height, &address, &captured::CALL_DATA)
            .map_err(|error| format!("getBudget read: {error:?}"))?;
        Self::decode(checkpoint, &result)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HarnessDecision {
    pub case: &'static str,
    pub outcome: Outcome,
    pub reason: DecisionReason,
    pub policy_version: String,
    pub matched_rules: Vec<String>,
    pub deciding_rule: Option<String>,
    pub dimension: Option<ConstraintDimension>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HarnessReport {
    pub decisions: Vec<HarnessDecision>,
    pub blocked_constraints: BTreeSet<ConstraintDimension>,
    pub observed_height: u64,
    pub protocol_consumed: u128,
}

#[derive(Clone)]
pub struct HarnessCase {
    pub name: &'static str,
    pub policy: PolicySet,
    pub request: PolicyRequest,
    pub session: SessionRecord,
    pub capability: TimedCapability,
    pub dimension: ConstraintDimension,
}

fn control(evidence: &HarnessEvidence) -> Result<HarnessCase, String> {
    let seed = adversarial::agent_policy_adversarial_corpus().remove(0);
    let mut session = seed.session;
    session.request.expiry_seconds = Some(1_800_000_000);
    session.request.expiry_sequence = u64::MAX;
    session.request.permitted_activity_types.insert(8);
    let capability = TimedCapability {
        id: seed.capability.id.0,
        parent: None,
        tenant: session.request.tenant.clone(),
        agent: String::from_utf8(session.request.agent.as_bytes().to_vec())
            .map_err(|error| error.to_string())?,
        authority: session.request.authority.clone(),
        activity_types: BTreeSet::from([7, 8]),
        counterparties: BTreeSet::from([[8; 32], [0xff; 32]]),
        assets: BTreeSet::from([[9; 32], [0xaa; 32]]),
        amount_ceilings: BTreeMap::from([([9; 32], 500), ([0xaa; 32], 500)]),
        rate_ceilings: BTreeMap::new(),
        purposes: BTreeSet::from([
            "research".to_owned(),
            "reѕearch".to_owned(),
            "research".repeat(8_192),
        ]),
        expiry_seconds: 1_800_000_000,
        grant_not_after_ms: 1_800_000_000_000,
        created_at_ms: 0,
        created_at_sequence: 0,
        revoked: None,
    };
    let mut request = seed.request;
    request.amount = 50;
    request.core_sequence = evidence.observed.through_sequence();
    let mut policy = seed.policy;
    policy.rules[0].constraints = RuleConstraints {
        maximum_cumulative_amount: Some(300),
        ..RuleConstraints::default()
    };
    Ok(HarnessCase {
        name: "intended-control",
        policy,
        request,
        session,
        capability,
        dimension: ConstraintDimension::ActivityType,
    })
}

pub fn harness_cases(evidence: &HarnessEvidence) -> Result<Vec<HarnessCase>, String> {
    let base = control(evidence)?;
    let mut cases = Vec::new();
    for dimension in required_blocked_constraints() {
        let mut case = base.clone();
        case.dimension = dimension;
        case.policy.rules[0].constraints = RuleConstraints::default();
        let constraints = &mut case.policy.rules[0].constraints;
        match dimension {
            ConstraintDimension::ActivityType => {
                case.name = "activity-type-widening";
                constraints.activity_types.insert(7);
                case.request.activity_type = 8;
            }
            ConstraintDimension::Counterparty => {
                case.name = "crafted-counterparty";
                constraints.counterparties.insert([8; 32]);
                case.request.counterparty = [0xff; 32];
            }
            ConstraintDimension::Asset => {
                case.name = "asset-substitution";
                constraints.assets.insert([9; 32]);
                case.request.asset = [0xaa; 32];
            }
            ConstraintDimension::Amount => {
                case.name = "amount-boundary-plus-one";
                constraints.maximum_amount = Some(50);
                case.request.amount = 51;
            }
            ConstraintDimension::CumulativeRate => {
                case.name = "cumulative-amount-boundary-plus-one";
                constraints.maximum_cumulative_amount = Some(
                    evidence
                        .budget
                        .local_after()
                        .checked_add(50)
                        .ok_or("cumulative amount overflow")?,
                );
                case.request.amount = 51;
            }
            ConstraintDimension::Purpose => {
                case.name = "unicode-purpose-confusable";
                constraints.purposes.insert("research".to_owned());
                case.request.purpose = "reѕearch".to_owned();
            }
            ConstraintDimension::Capability => {
                case.name = "capability-substitution";
                constraints.capability_ids.insert(seed_capability_id());
                case.capability.id = [0x44; 32];
            }
            ConstraintDimension::Session => {
                case.name = "session-substitution";
                constraints.session_ids.insert(SessionId([3; 32]));
                case.session.request.session_id = SessionId([0x33; 32]);
            }
            ConstraintDimension::Agent => {
                case.name = "agent-substitution";
                constraints
                    .agents
                    .insert(base.session.request.agent.clone());
                case.session.request.agent = Did::new(b"did:layerx:other-agent")
                    .map_err(|error| format!("agent: {error:?}"))?;
                case.capability.agent = "did:layerx:other-agent".to_owned();
            }
            ConstraintDimension::Tenant => {
                case.name = "cross-tenant-substitution";
                constraints
                    .tenants
                    .insert(base.session.request.tenant.clone());
                case.session.request.tenant =
                    TenantId::new("tenant-b").map_err(|error| format!("tenant: {error:?}"))?;
                case.capability.tenant = case.session.request.tenant.clone();
            }
            ConstraintDimension::TimeWindow => {
                case.name = "outside-sequence-window";
                let next = case
                    .request
                    .core_sequence
                    .checked_add(1)
                    .ok_or("sequence overflow")?;
                constraints.sequence_window = Some(SequenceWindow {
                    first: next,
                    last: next,
                });
            }
            ConstraintDimension::RequiredApproval => {
                case.name = "approval-required";
                constraints.required_approval = true;
            }
        }
        case.policy.rules[0].id = format!("constraint-{dimension:?}");
        cases.push(case);
    }
    let mut oversized = cases
        .iter()
        .find(|case| case.dimension == ConstraintDimension::Purpose)
        .ok_or("missing purpose case")?
        .clone();
    oversized.name = "oversized-purpose";
    oversized.request.purpose = "research".repeat(8_192);
    cases.push(oversized);
    Ok(cases)
}

fn seed_capability_id() -> layerx_agentd::capability::CapabilityId {
    layerx_agentd::capability::CapabilityId([4; 32])
}

fn decision(
    case: &HarnessCase,
    evidence: &HarnessEvidence,
    budget: bool,
) -> Result<Decision, String> {
    let registry = PolicyRegistry::new(case.policy.clone())
        .map_err(|error| format!("policy {}: {error:?}", case.name))?;
    Ok(evaluate_timed_admission_with_context(
        &registry.begin_request(),
        &(&case.request).into(),
        &case.session,
        &case.capability,
        &evidence.observed,
        &[],
        budget.then_some(&evidence.budget),
    ))
}

fn require_decision(
    case: &HarnessCase,
    actual: &Decision,
    reason: DecisionReason,
) -> Result<(), String> {
    let (outcome, matched, deciding) = match reason {
        DecisionReason::PermittedByRule => (
            Outcome::Allow,
            vec![case.policy.rules[0].id.clone()],
            Some(case.policy.rules[0].id.clone()),
        ),
        DecisionReason::ApprovalRequired => (
            Outcome::Deny,
            vec![case.policy.rules[0].id.clone()],
            Some(case.policy.rules[0].id.clone()),
        ),
        DecisionReason::NoPermittingRule => (Outcome::Deny, Vec::new(), None),
        _ => return Err("unsupported coverage reason".to_owned()),
    };
    if actual.outcome != outcome
        || actual.reason != reason
        || actual.matched_rules != matched
        || actual.deciding_rule != deciding
        || actual.policy_version != case.policy.version
    {
        return Err(format!(
            "case {}: expected {reason:?}, observed {actual:?}",
            case.name
        ));
    }
    Ok(())
}

pub fn run_cases(
    evidence: &HarnessEvidence,
    cases: &[HarnessCase],
) -> Result<HarnessReport, String> {
    let base = control(evidence)?;
    let allowed = decision(&base, evidence, true)?;
    require_decision(&base, &allowed, DecisionReason::PermittedByRule)?;
    let mut report = HarnessReport {
        decisions: vec![reported(&base, allowed, None)],
        blocked_constraints: BTreeSet::new(),
        observed_height: evidence.observed.through_sequence(),
        protocol_consumed: evidence.budget.local_after(),
    };
    let expected_cases = harness_cases(evidence)?;
    for case in cases {
        // A single constrained rule makes NoPermittingRule attributable to its one dimension.
        let expected_case = expected_cases
            .iter()
            .find(|expected| expected.name == case.name)
            .ok_or_else(|| format!("unknown coverage case {}", case.name))?;
        if case.dimension != expected_case.dimension
            || case.policy != expected_case.policy
            || case.request != expected_case.request
            || case.session != expected_case.session
            || case.capability != expected_case.capability
        {
            return Err(format!(
                "case {} does not isolate its declared constraint",
                case.name
            ));
        }
        let mut permitted = base.clone();
        permitted.policy = case.policy.clone();
        if case.dimension == ConstraintDimension::TimeWindow {
            permitted.policy.rules[0].constraints.sequence_window = Some(SequenceWindow {
                first: base.request.core_sequence,
                last: base.request.core_sequence,
            });
        }
        if case.dimension == ConstraintDimension::RequiredApproval {
            permitted.policy.rules[0].constraints.required_approval = false;
        }
        require_decision(
            &permitted,
            &decision(&permitted, evidence, true)?,
            DecisionReason::PermittedByRule,
        )?;
        let actual = decision(case, evidence, true)?;
        let reason = if case.dimension == ConstraintDimension::RequiredApproval {
            DecisionReason::ApprovalRequired
        } else {
            DecisionReason::NoPermittingRule
        };
        require_decision(case, &actual, reason)?;
        let mut empty = case.clone();
        empty.policy.rules.clear();
        let denied = decision(&empty, evidence, true)?;
        if denied.outcome != Outcome::Deny || denied.reason != DecisionReason::NoPermittingRule {
            return Err(format!(
                "deny-by-default escaped for {}: {denied:?}",
                case.name
            ));
        }
        report.blocked_constraints.insert(case.dimension);
        report
            .decisions
            .push(reported(case, actual, Some(case.dimension)));
    }
    if report.blocked_constraints != required_blocked_constraints() {
        return Err(format!(
            "observed constraint coverage incomplete: {:?}",
            report.blocked_constraints
        ));
    }
    Ok(report)
}

fn reported(
    case: &HarnessCase,
    actual: Decision,
    dimension: Option<ConstraintDimension>,
) -> HarnessDecision {
    HarnessDecision {
        case: case.name,
        outcome: actual.outcome,
        reason: actual.reason,
        policy_version: actual.policy_version,
        matched_rules: actual.matched_rules,
        deciding_rule: actual.deciding_rule,
        dimension,
    }
}

pub fn agent_policy_harness() -> Result<HarnessReport, String> {
    let evidence = HarnessEvidence::decode(&captured::checkpoint()?, &captured::budget_result()?)?;
    run_cases(&evidence, &harness_cases(&evidence)?)
}
