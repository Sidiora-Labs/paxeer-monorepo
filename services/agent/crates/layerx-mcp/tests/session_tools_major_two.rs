use layerx_mcp_test_support::option_assertion;
use layerx_mcp_test_support::result_assertion;

use layerx_mcp_test_support::test_clock;
use layerx_types::clock::Clock as _;

use layerx_agent_api::subscription::Cursor;
use layerx_mcp::catalogue::{input_schema, validate};
use serde_json::{json, Value};

#[test]
fn subscription_acknowledge_tool_takes_a_cursor_triple() {
    use layerx_mcp::catalogue::subscription_acknowledge_tool;

    let cursor = Cursor {
        block_height: u64::MAX,
        tx_index: u32::MAX,
        log_index: u32::MAX,
    };
    let request = json!({
        "scope": {"tenant": "tenant-a", "agent": "did:layerx:agent-a", "capability": "capability-a"},
        "subscription_id": "subscription-a",
        "cursor": {
            "block_height": cursor.block_height.to_string(),
            "tx_index": cursor.tx_index.to_string(),
            "log_index": cursor.log_index.to_string(),
        },
    });
    assert_subscription_cursor_schema(&request);
    let create = json!({
        "scope": request["scope"],
        "filter": {"agents": [], "accounts": [], "activity_types": [], "modules": [], "assets": [], "counterparties": [], "result_classes": []},
        "start": request["cursor"],
        "delivery_target": "uds://consumer-a",
    });
    assert_eq!(validate("subscription.create", &create), Ok(()));
    let root = std::path::PathBuf::from("/tmp").join(format!(
        "layerx-mcp-major-two-subscription-{}",
        std::process::id()
    ));
    let (mut store, target) = open_cursor_subscription(&root);
    assert!(
        subscription_acknowledge_tool(&mut store, &request).is_err(),
        "undelivered cursor refused"
    );
    result_assertion::required(
        store.mark_delivered(&target, cursor.into()),
        "mark cursor delivered",
    );
    let record = result_assertion::required(
        subscription_acknowledge_tool(&mut store, &request),
        "acknowledge cursor triple",
    );
    assert_eq!(record.last_acknowledged, cursor);
    assert_eq!(
        result_assertion::required(store.get(&target), "stored subscription").last_acknowledged,
        cursor
    );
    for scalar in [json!(12), json!("12")] {
        let mut scalar_request = request.clone();
        scalar_request["cursor"] = scalar;
        assert!(subscription_acknowledge_tool(&mut store, &scalar_request).is_err());
        assert_eq!(
            result_assertion::required(store.get(&target), "unchanged subscription")
                .last_acknowledged,
            cursor
        );
    }
    drop(store);
    result_assertion::required(std::fs::remove_dir_all(root), "remove durable store");
    let mut invalid = create;
    invalid["start"] = json!("12");
    assert!(validate("subscription.create", &invalid).is_err());
}

use layerx_mcp_test_support::approval_fixture as approval;

#[test]
fn approval_get_tool_reports_tx_hash_and_rung() {
    use layerx_agent_api::track::{SubmissionRef, SubmissionState, TrackedSubmission};
    use layerx_agentd::approval::{ApprovalExpiry, ApprovalService};
    use layerx_agentd::budget::BudgetLimiter;
    use layerx_agentd::policy::approval::ApprovalRegistry;
    use layerx_mcp::approval::{require, ApprovalPolicy, Requirement};
    use layerx_mcp::catalogue::approval_get_tool;
    use layerx_types::rung::Rung;
    use std::collections::BTreeMap;

    let registry = ApprovalRegistry::default();
    let context = approval::context(15);
    let prepared = approval::prepared(101, 15);
    let Requirement::Required(ticket) = result_assertion::required(
        require(
            &registry,
            ApprovalPolicy {
                amount_threshold: 100,
            },
            context.clone(),
            prepared.clone(),
            50,
            &test_clock::TestClock::new(50),
        ),
        "register exact approval",
    ) else {
        panic!("approval required");
    };
    let root = std::path::PathBuf::from("/tmp").join(format!(
        "layerx-mcp-major-two-approval-{}",
        std::process::id()
    ));
    let expiry =
        result_assertion::required(ApprovalExpiry::open(&root), "new approval decision store");
    let limiter = result_assertion::required(BudgetLimiter::new(Vec::new()), "budget limiter");
    let service = ApprovalService::new(&registry, &limiter, &expiry);
    let release_ref = approve_native_hold(&service, &context.tenant, &ticket, &prepared);
    let approval_id = layerx_programs::hex::encode(&ticket.hold_id);
    let tx_hash = "ab".repeat(32);
    assert_ne!(tx_hash, layerx_programs::hex::encode(&release_ref));
    let tracked = TrackedSubmission {
        submission_ref: result_assertion::required(
            SubmissionRef::new(tx_hash.clone()),
            "transaction hash",
        ),
        state: SubmissionState::Submitted,
        evidence: Vec::new(),
        rung: Rung::Submitted,
        transitions: Vec::new(),
    };
    let submissions = BTreeMap::from([(release_ref, tracked)]);
    let arguments = json!({"approval_id": approval_id, "current_sequence": "60", "current_time":test_clock::TestClock::new(60).now().to_string()});
    assert_eq!(validate("approval.get", &arguments), Ok(()));
    let facts = result_assertion::required(
        approval_get_tool(&service, &context.tenant, &submissions, &arguments),
        "approval.get tool response",
    );
    approval::assert_major_two_approval_facts(
        &facts,
        &approval_id,
        &tx_hash,
        Rung::Submitted.name(),
    );
    session_transport::approval_result_over_transport(
        &service,
        &context.tenant,
        &submissions,
        &arguments,
        &facts,
    );
    assert_eq!(facts["state"], "Approved");
    assert_eq!(facts["actor"], prepared.disclosure.actor.as_str());
    assert_eq!(facts["amounts"][0]["amount"], "101");
    assert!(approval_get_tool(&service, &context.tenant, &BTreeMap::new(), &arguments).is_err());
    let mut cross_tenant = arguments.clone();
    cross_tenant["tenant"] = json!("tenant-b");
    assert!(approval_get_tool(&service, &context.tenant, &submissions, &cross_tenant).is_err());
    let mut cross_agent = arguments;
    cross_agent["agent"] = json!("did:layerx:other");
    assert!(approval_get_tool(&service, &context.tenant, &submissions, &cross_agent).is_err());
    drop(expiry);
    result_assertion::required(
        std::fs::remove_dir_all(root),
        "remove approval decision store",
    );
}

use layerx_mcp_test_support::session_transport;

#[test]
fn session_transport_preserves_cursor_triples_and_reopened_approval_shapes() {
    session_transport::session_tools_authenticated_daemon_codec();
    session_transport::session_tools_bind_describe_and_acknowledge();
    session_transport::all_session_tools_through_daemon_and_mcp();
}

fn assert_subscription_cursor_schema(request: &Value) {
    let schema = option_assertion::required(
        input_schema("subscription.acknowledge"),
        "acknowledgement schema",
    );
    assert_eq!(schema["properties"]["cursor"]["type"], "object");
    assert_eq!(
        schema["properties"]["cursor"]["required"],
        json!(["block_height", "tx_index", "log_index"])
    );
    assert_eq!(validate("subscription.acknowledge", request), Ok(()));
    for scalar in [json!(12), json!("12"), Value::Null] {
        let mut invalid = request.clone();
        invalid["cursor"] = scalar;
        assert!(validate("subscription.acknowledge", &invalid).is_err());
    }
    for field in ["block_height", "tx_index", "log_index"] {
        let mut missing = request.clone();
        option_assertion::required(missing["cursor"].as_object_mut(), "cursor object")
            .remove(field);
        assert!(
            validate("subscription.acknowledge", &missing).is_err(),
            "missing {field}"
        );
        for malformed in [
            json!("01"),
            json!("-1"),
            json!(1),
            json!("18446744073709551616"),
        ] {
            let mut invalid = request.clone();
            invalid["cursor"][field] = malformed;
            assert!(
                validate("subscription.acknowledge", &invalid).is_err(),
                "malformed {field}"
            );
        }
    }
    for field in ["tx_index", "log_index"] {
        let mut invalid = request.clone();
        invalid["cursor"][field] = json!("4294967296");
        assert!(
            validate("subscription.acknowledge", &invalid).is_err(),
            "overflowing {field}"
        );
    }
    let mut extra = request.clone();
    extra["cursor"]["sequence"] = json!("1");
    assert!(validate("subscription.acknowledge", &extra).is_err());
}

fn approve_native_hold(
    service: &layerx_agentd::approval::ApprovalService<'_>,
    tenant: &layerx_agentd::store::TenantId,
    ticket: &layerx_agentd::policy::approval::ApprovalTicket,
    prepared: &layerx_agent_api::prepare::Prepared,
) -> [u8; 32] {
    use layerx_agentd::approval::{
        ApprovalOutcome, ApprovalSubmissionQueue, DecisionKey, DecisionRequest,
    };
    use layerx_agentd::policy::approval::ApproverId;
    let queue = ApprovalSubmissionQueue::default();
    let decision = result_assertion::required(
        service.approve(
            DecisionRequest {
                tenant,
                approval_id: ticket.hold_id,
                idempotency_key: &result_assertion::required(
                    DecisionKey::new("major-two-approve"),
                    "decision key",
                ),
                approver: result_assertion::required(ApproverId::new("human:alice"), "approver"),
                current_sequence: 60,
                current_time: layerx_agent_api::TimestampSeconds(
                    result_assertion::required(
                        test_clock::TestClock::new(60).sample(std::time::Duration::from_secs(1)),
                        "clock",
                    )
                    .unix_seconds(),
                ),
            },
            prepared,
            &queue,
        ),
        "release registered approval",
    );
    assert_eq!(decision.outcome, ApprovalOutcome::Granted);
    let release_ref =
        option_assertion::required(decision.submission_ref, "approval release reference");
    assert_eq!(queue.prepared(release_ref), Some(prepared.clone()));
    release_ref
}

fn open_cursor_subscription(
    root: &std::path::Path,
) -> (
    layerx_agentd::events::subscription::Store,
    layerx_agent_api::subscription::SubscriptionTarget,
) {
    use layerx_agent_api::identity::{
        AgentDid, CapabilityId, ExplicitSet, TenantId as ApiTenantId,
    };
    use layerx_agent_api::subscription::{
        DeliveryTarget, SubscriptionCreate, SubscriptionFilter, SubscriptionId, SubscriptionScope,
        SubscriptionTarget,
    };
    use layerx_agentd::events::subscription::Store as SubscriptionStore;
    use layerx_agentd::store::{Store, TenantId};
    let durable = result_assertion::required(Store::open(root), "new durable store");
    let mut store = result_assertion::required(
        SubscriptionStore::open(
            durable,
            result_assertion::required(TenantId::new("tenant-a"), "tenant"),
        ),
        "subscription store",
    );
    let scope = SubscriptionScope {
        tenant: result_assertion::required(ApiTenantId::new("tenant-a"), "API tenant"),
        agent: result_assertion::required(AgentDid::new("did:layerx:agent-a"), "agent"),
        capability: result_assertion::required(CapabilityId::new("capability-a"), "capability"),
    };
    let subscription_id =
        result_assertion::required(SubscriptionId::new("subscription-a"), "subscription ID");
    let start = Cursor {
        block_height: 0,
        tx_index: 0,
        log_index: 0,
    };
    result_assertion::required(
        store.create(
            subscription_id.clone(),
            SubscriptionCreate {
                scope: scope.clone(),
                filter: SubscriptionFilter {
                    agents: ExplicitSet::deny_all(),
                    accounts: ExplicitSet::deny_all(),
                    activity_types: ExplicitSet::deny_all(),
                    modules: ExplicitSet::deny_all(),
                    assets: ExplicitSet::deny_all(),
                    counterparties: ExplicitSet::deny_all(),
                    result_classes: ExplicitSet::deny_all(),
                },
                start,
                delivery_target: result_assertion::required(
                    DeliveryTarget::new("uds://consumer-a"),
                    "delivery target",
                ),
            },
        ),
        "create subscription",
    );
    let target = SubscriptionTarget {
        scope,
        subscription_id,
    };
    (store, target)
}
