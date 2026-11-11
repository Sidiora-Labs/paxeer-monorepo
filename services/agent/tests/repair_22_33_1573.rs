// Task 22.33.1573: 22.30.99/policy-control-denies is proved by
// policy_control_denies_independently_varied_constraints below.
// Baseline: not run; the explicit gate order forbids verification before implementation.
// No out-of-path wiring is required. The runner records the verified revision and gate evidence.

#[path = "policy/adversarial.rs"]
mod adversarial;

use std::collections::BTreeSet;

use adversarial::{
    agent_policy_adversarial_corpus, required_blocked_constraints, ConstraintDimension,
};
use layerx_agentd::identity::register;
use layerx_agentd::policy::{
    activate, evaluate, Decision, DecisionReason, EvaluationInput, Outcome, PolicyRegistry,
    RuleConstraints,
};
use layerx_agentd::protocol_evidence::{AuthenticatedCumulativeUse, CumulativeUseWindow};
use layerx_agentd::session::{open, SessionRegistry};
use layerx_agentd::store::Store;

use super::{repair_support, support};

fn provisioned_corpus() -> (Vec<adversarial::CorpusCase>, AuthenticatedCumulativeUse) {
    let mut cases = agent_policy_adversarial_corpus();
    let request = cases[0].session.request.clone();
    let root = support::directory("repair-22-33-1573-policy");
    let mut store = Store::open(&root)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "production session store"));
    let (mut authority, worker) = repair_support::authority_peer(vec![(
        200,
        repair_support::identity_document(120, false, std::slice::from_ref(&request.authority)),
    )]);
    let peer = repair_support::peer();
    let identity = register(
        &mut store,
        request.tenant.clone(),
        request.agent.clone(),
        &mut repair_support::AuthorityIdentity {
            authority: &mut authority,
            peer: &peer,
        },
    )
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "verified identity through production authority client"
        )
    });
    worker
        .join()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "identity transcript"));
    let mut sessions = SessionRegistry::default();
    let token = open(&mut store, &mut sessions, &identity, request.clone(), 120)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "production session admission"));
    assert_eq!(
        token.authorize(&sessions, &request.tenant, &request.agent, "prepare", 120),
        Ok(request.session_id)
    );
    let session = sessions
        .get(&request.tenant, request.session_id)
        .unwrap_or_else(|| panic!("{}", "admitted session"))
        .clone();
    for case in &mut cases {
        case.session = session.clone();
    }
    let (authority, entries) = repair_support::cumulative_window(&request.agent);
    let cumulative = authority
        .authenticate_cumulative_use(
            &request.agent,
            CumulativeUseWindow {
                first: 20,
                last: 119,
            },
            &entries,
        )
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "complete signed activity and receipt window"
            )
        });
    assert_eq!(cumulative.amount(), 25);
    assert_eq!(cumulative.count(), 1);
    let mut wrong_actor = cases[0].clone();
    wrong_actor.session.request.agent = layerx_types::ids::Did::new(b"did:layerx:other-agent")
        .unwrap_or_else(|error| panic!("{}: {error:?}", "different actor"));
    assert_decision(
        &wrong_actor.evaluate(&cumulative),
        Outcome::Deny,
        DecisionReason::InvalidContext,
    );
    let mut wrong_window = cases[0].clone();
    wrong_window.request.core_sequence += 1;
    assert_decision(
        &wrong_window.evaluate(&cumulative),
        Outcome::Deny,
        DecisionReason::InvalidContext,
    );
    drop(store);
    std::fs::remove_dir_all(root)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "remove test session store"));
    (cases, cumulative)
}

fn assert_decision(actual: &Decision, outcome: Outcome, reason: DecisionReason) {
    assert_eq!(actual.outcome, outcome);
    assert_eq!(actual.reason, reason);
    assert_eq!(actual.policy_version, "corpus-v1");
    if matches!(
        reason,
        DecisionReason::PermittedByRule | DecisionReason::ApprovalRequired
    ) {
        assert_eq!(actual.matched_rules, vec!["intended-research"]);
        assert_eq!(actual.deciding_rule.as_deref(), Some("intended-research"));
    } else {
        assert!(actual.matched_rules.is_empty());
        assert_eq!(actual.deciding_rule, None);
    }
}

fn restore_constraint(
    actual: &mut RuleConstraints,
    control: &RuleConstraints,
    dimension: ConstraintDimension,
    name: &str,
) {
    match dimension {
        ConstraintDimension::ActivityType => {
            actual.activity_types.clone_from(&control.activity_types);
        }
        ConstraintDimension::Counterparty => {
            actual.counterparties.clone_from(&control.counterparties);
        }
        ConstraintDimension::Asset => actual.assets.clone_from(&control.assets),
        ConstraintDimension::Amount => actual.maximum_amount = control.maximum_amount,
        ConstraintDimension::CumulativeRate if name == "cumulative-amount-boundary-plus-one" => {
            actual.maximum_cumulative_amount = control.maximum_cumulative_amount;
        }
        ConstraintDimension::CumulativeRate => {
            assert_eq!(name, "cumulative-count-boundary-plus-one");
            actual.maximum_cumulative_count = control.maximum_cumulative_count;
        }
        ConstraintDimension::Purpose => actual.purposes.clone_from(&control.purposes),
        ConstraintDimension::Capability => {
            actual.capability_ids.clone_from(&control.capability_ids);
        }
        ConstraintDimension::Session => actual.session_ids.clone_from(&control.session_ids),
        ConstraintDimension::Agent => actual.agents.clone_from(&control.agents),
        ConstraintDimension::Tenant => actual.tenants.clone_from(&control.tenants),
        ConstraintDimension::TimeWindow => actual.sequence_window = control.sequence_window,
        ConstraintDimension::RequiredApproval => {
            actual.required_approval = control.required_approval;
        }
    }
}

#[test]
fn policy_control_denies_independently_varied_constraints() {
    let (cases, cumulative) = provisioned_corpus();
    assert_eq!(cases.len(), 15);
    let control = &cases[0];
    assert_eq!(control.name, "intended-control");
    assert!(control.blocked_dimensions.is_empty());
    assert_eq!(control.expected, Outcome::Allow);
    assert_eq!(control.expected_reason, DecisionReason::PermittedByRule);
    assert_decision(
        &control.evaluate(&cumulative),
        Outcome::Allow,
        DecisionReason::PermittedByRule,
    );

    let mut covered = BTreeSet::new();
    let mut names = BTreeSet::new();
    for case in &cases[1..] {
        assert!(names.insert(case.name), "duplicate case {}", case.name);
        assert_eq!(case.blocked_dimensions.len(), 1, "{}", case.name);
        let dimension = *case
            .blocked_dimensions
            .first()
            .unwrap_or_else(|| panic!("{}", "one varied constraint"));
        assert_eq!(case.request, control.request, "{}", case.name);
        assert_eq!(case.session, control.session, "{}", case.name);
        assert_eq!(case.capability, control.capability, "{}", case.name);
        assert_eq!(case.expected, Outcome::Deny, "{}", case.name);
        let reason = if dimension == ConstraintDimension::RequiredApproval {
            DecisionReason::ApprovalRequired
        } else {
            DecisionReason::NoPermittingRule
        };
        assert_eq!(case.expected_reason, reason, "{}", case.name);
        assert_ne!(case.policy, control.policy, "{}", case.name);
        assert_decision(&case.evaluate(&cumulative), Outcome::Deny, reason);

        let mut restored = case.clone();
        restore_constraint(
            &mut restored.policy.rules[0].constraints,
            &control.policy.rules[0].constraints,
            dimension,
            case.name,
        );
        assert_eq!(
            restored.policy, control.policy,
            "only one changed constraint: {}",
            case.name
        );
        assert_decision(
            &restored.evaluate(&cumulative),
            Outcome::Allow,
            DecisionReason::PermittedByRule,
        );
        covered.insert(dimension);
    }
    assert_eq!(covered, required_blocked_constraints());
}

#[test]
fn missing_evidence_is_distinct_from_constraint_denial_and_versions_remain_bound() {
    let (cases, cumulative) = provisioned_corpus();
    for case in &cases {
        let missing = EvaluationInput::without_protocol_budget(
            &case.request,
            &case.session,
            &case.capability,
        );
        assert_decision(
            &evaluate(&case.policy, &missing),
            Outcome::Deny,
            DecisionReason::InvalidContext,
        );
    }
    let control = &cases[0];
    let mut registry = PolicyRegistry::new(control.policy.clone())
        .unwrap_or_else(|error| panic!("{}: {error:?}", "valid control policy"));
    let snapshot = registry.begin_request();
    let mut later = control.policy.clone();
    later.version = "corpus-v2".to_owned();
    activate(&mut registry, later)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "activate next policy version"));
    let input = EvaluationInput::with_authenticated_cumulative_use(
        &control.request,
        &control.session,
        &control.capability,
        &cumulative,
    );
    assert_decision(
        &evaluate(snapshot.policy(), &input),
        Outcome::Allow,
        DecisionReason::PermittedByRule,
    );
    for case in &cases[1..] {
        assert_decision(
            &case.evaluate(&cumulative),
            Outcome::Deny,
            case.expected_reason,
        );
    }
    let current = registry.begin_request();
    let refused = evaluate(current.policy(), &input);
    assert_eq!(refused.outcome, Outcome::Deny);
    assert_eq!(refused.reason, DecisionReason::InvalidContext);
    assert_eq!(refused.policy_version, "corpus-v2");
}
