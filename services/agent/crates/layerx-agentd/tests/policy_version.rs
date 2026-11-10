#[path = "support/repair_22_33_1529.rs"]
mod repair_support;
mod support;

use std::collections::BTreeSet;

use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
use layerx_agentd::identity::{register, ProtocolAuthority};
use layerx_agentd::policy::{
    activate, evaluate, evaluate_admission_with_context, load_policy_source, DecisionReason,
    EvaluationInput, Outcome, PolicyRegistry, PolicyRequest, PolicySet, PolicyValidationError,
    Rule, RuleConstraints, RuleEffect, VerifiedPolicyContext,
};
use layerx_agentd::session::{open, OpenRequest, SessionId, SessionRecord, SessionRegistry};
use layerx_agentd::store::{Store, TenantId};
use layerx_types::ids::Did;

fn policy(version: &str, effect: RuleEffect) -> PolicySet {
    PolicySet {
        version: version.to_owned(),
        rules: vec![Rule {
            id: format!("{version}-rule"),
            effect,
            constraints: RuleConstraints::default(),
        }],
        evaluation_step_limit: 10,
    }
}

fn tenant() -> TenantId {
    TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}"))
}

fn session(
    store: &mut Store,
    sessions: &mut SessionRegistry,
    version: &str,
    id: u8,
) -> SessionRecord {
    let authority = ProtocolAuthority::SessionKey([1; 32]);
    let did = lint_success(Did::new(b"did:layerx:versioned"), "policy agent");
    let (mut node_authority, worker) = repair_support::authority_peer(vec![(
        200,
        repair_support::identity_document(120, false, std::slice::from_ref(&authority)),
    )]);
    let peer = repair_support::peer();
    let mut resolver = repair_support::AuthorityIdentity {
        authority: &mut node_authority,
        peer: &peer,
    };
    let identity = lint_success(
        register(store, tenant(), did.clone(), &mut resolver),
        "verified policy-session identity",
    );
    lint_success(worker.join(), "identity transcript");
    let token = lint_success(
        open(
            store,
            sessions,
            &identity,
            OpenRequest {
                session_id: SessionId([id; 32]),
                token_id: [id + 10; 32],
                tenant: tenant(),
                agent: did.clone(),
                authority,
                permitted_activity_types: BTreeSet::from([7]),
                scopes: BTreeSet::from(["prepare".to_owned()]),
                expiry_seconds: None,
                expiry_sequence: 200,
                opening_client: "version-test".to_owned(),
                policy_version: version.to_owned(),
            },
            120,
        ),
        "production session open",
    );
    assert_eq!(
        token.authorize(sessions, &tenant(), &did, "prepare", 120),
        Ok(token.session_id())
    );
    lint_present(
        sessions.get(&tenant(), token.session_id()),
        "opened session",
    )
    .clone()
}

fn capability() -> Capability {
    Capability::new(
        CapabilityId([4; 32]),
        tenant(),
        CapabilityDimensions {
            activity_types: BTreeSet::from([7]),
            counterparties: BTreeSet::from([[8; 32]]),
            assets: BTreeSet::from([[9; 32]]),
            amount_ceiling: 500,
            rate_ceiling: RateCeiling {
                maximum_uses: 10,
                window_sequences: 100,
            },
            purposes: BTreeSet::from(["research".to_owned()]),
            expiry_sequence: 200,
        },
    )
    .unwrap_or_else(|error| panic!("capability: {error:?}"))
}

fn request() -> PolicyRequest {
    PolicyRequest {
        activity_type: 7,
        counterparty: [8; 32],
        asset: [9; 32],
        amount: 100,
        purpose: "research".to_owned(),
        core_sequence: 120,
    }
}

#[test]
fn activation_only_changes_requests_received_after_it() {
    let mut registry = PolicyRegistry::new(policy("v1", RuleEffect::Permit))
        .unwrap_or_else(|error| panic!("registry: {error:?}"));
    let in_flight = registry.begin_request();
    let activation = activate(&mut registry, policy("v2", RuleEffect::Deny))
        .unwrap_or_else(|error| panic!("activate: {error:?}"));
    let later = registry.begin_request();
    assert_eq!(activation.previous_version, "v1");
    assert_eq!(in_flight.version(), "v1");
    assert_eq!(later.version(), "v2");

    let root = support::directory("policy-version-activation");
    let mut store = lint_success(Store::open(&root), "policy session store");
    let mut sessions = SessionRegistry::default();
    let request = request();
    let capability = capability();
    let old_session = session(&mut store, &mut sessions, "v1", 3);
    let missing = EvaluationInput::without_protocol_budget(&request, &old_session, &capability);
    assert_eq!(
        evaluate(in_flight.policy(), &missing).reason,
        DecisionReason::InvalidContext
    );
    assert_eq!(
        evaluate(in_flight.policy(), &missing).outcome,
        Outcome::Deny
    );
    let (authority, entries) = repair_support::cumulative_window(&old_session.request.agent);
    let cumulative = lint_success(
        authority.authenticate_cumulative_use(
            &old_session.request.agent,
            layerx_agentd::protocol_evidence::CumulativeUseWindow {
                first: 20,
                last: 119,
            },
            &entries,
        ),
        "complete producer window",
    );
    assert_eq!(cumulative.amount(), 25);
    assert_eq!(cumulative.count(), 1);
    let old_input = EvaluationInput::with_authenticated_cumulative_use(
        &request,
        &old_session,
        &capability,
        &cumulative,
    );
    assert_projected_usage_is_bounded(in_flight.policy(), &old_input);
    let old_decision = evaluate(in_flight.policy(), &old_input);
    assert_eq!(old_decision.outcome, Outcome::Allow, "{old_decision:?}");
    assert_eq!(
        evaluate_admission_with_context(
            &in_flight,
            &(&request).into(),
            &old_session,
            (&capability).into(),
            VerifiedPolicyContext::Authenticated(&cumulative),
        ),
        old_decision
    );
    assert_eq!(old_decision.reason, DecisionReason::PermittedByRule);
    assert_eq!(old_decision.deciding_rule.as_deref(), Some("v1-rule"));

    let new_session = session(&mut store, &mut sessions, "v2", 4);
    let new_input = EvaluationInput::with_authenticated_cumulative_use(
        &request,
        &new_session,
        &capability,
        &cumulative,
    );
    let new_decision = evaluate(later.policy(), &new_input);
    assert_eq!(new_decision.outcome, Outcome::Deny);
    assert_eq!(
        evaluate_admission_with_context(
            &later,
            &(&request).into(),
            &new_session,
            (&capability).into(),
            VerifiedPolicyContext::Authenticated(&cumulative),
        ),
        new_decision
    );
    assert_eq!(new_decision.reason, DecisionReason::ExplicitDeny);
    assert_eq!(new_decision.deciding_rule.as_deref(), Some("v2-rule"));
    assert_recorded_policy_decisions(&mut registry, &old_decision, &new_decision);
    assert!(registry.retained("v1").is_some());
    assert!(registry.retained("v2").is_some());
    drop(sessions);
    drop(store);
    lint_success(std::fs::remove_dir_all(root), "remove policy store");
}

#[test]
fn invalid_activation_preserves_the_previous_version() {
    let mut registry = PolicyRegistry::new(policy("v1", RuleEffect::Permit))
        .unwrap_or_else(|error| panic!("registry: {error:?}"));
    let mut invalid = policy("v2", RuleEffect::Deny);
    invalid.rules.push(invalid.rules[0].clone());
    assert!(matches!(
        activate(&mut registry, invalid),
        Err(PolicyValidationError::DuplicateRuleId(_))
    ));
    assert_eq!(registry.active_version(), "v1");
    assert!(registry.retained("v2").is_none());
}

#[test]
fn bounded_policy_source_loader_rejects_malformed_input() {
    let loaded = load_policy_source(b"version=v3\nsteps=2\nrule=permit-all,permit\n")
        .unwrap_or_else(|error| panic!("load: {error:?}"));
    assert_eq!(loaded.version, "v3");
    assert!(load_policy_source(b"version=v3\nsteps=nope\n").is_err());
    assert!(load_policy_source(&vec![b'x'; 1_048_577]).is_err());
}

#[track_caller]
fn lint_success<T, E: std::fmt::Debug>(result: Result<T, E>, context: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{context}: {error:?}"),
    }
}

#[track_caller]
fn lint_present<T>(value: Option<T>, context: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{context}: required value is absent"),
    }
}

fn assert_projected_usage_is_bounded(policy: &PolicySet, old_input: &EvaluationInput<'_>) {
    // Historical usage alone exhausts these bounds: admission projects the
    // current request's amount and one additional use before matching.
    let mut exact_usage = policy.clone();
    exact_usage.rules[0].constraints.maximum_cumulative_amount = Some(25);
    exact_usage.rules[0].constraints.maximum_cumulative_count = Some(1);
    assert_eq!(evaluate(&exact_usage, old_input).outcome, Outcome::Deny);
    let mut too_little_amount = exact_usage.clone();
    too_little_amount.rules[0]
        .constraints
        .maximum_cumulative_amount = Some(24);
    assert_eq!(
        evaluate(&too_little_amount, old_input).outcome,
        Outcome::Deny
    );
    let mut too_little_count = exact_usage;
    too_little_count.rules[0]
        .constraints
        .maximum_cumulative_count = Some(0);
    assert_eq!(
        evaluate(&too_little_count, old_input).outcome,
        Outcome::Deny
    );
}

fn assert_recorded_policy_decisions(
    registry: &mut PolicyRegistry,
    old_decision: &layerx_agentd::policy::Decision,
    new_decision: &layerx_agentd::policy::Decision,
) {
    lint_success(
        registry.record_decision([8; 32], old_decision.clone()),
        "record retained v1 decision",
    );
    assert_eq!(
        registry.audit_entry([8; 32]).map(|entry| &entry.decision),
        Some(old_decision)
    );

    lint_success(
        registry.record_decision([9; 32], new_decision.clone()),
        "record active v2 decision",
    );
    assert_eq!(
        registry.audit_entry([9; 32]).map(|entry| &entry.decision),
        Some(new_decision)
    );
}
