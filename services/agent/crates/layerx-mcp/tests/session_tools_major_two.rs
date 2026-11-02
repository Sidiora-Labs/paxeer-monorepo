use layerx_agent_api::subscription::Cursor;
use layerx_mcp::catalogue::{input_schema, validate};
use serde_json::{json, Value};

#[test]
fn subscription_acknowledge_tool_takes_a_cursor_triple() {
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
    let schema = input_schema("subscription.acknowledge").expect("acknowledgement schema");
    assert_eq!(schema["properties"]["cursor"]["type"], "object");
    assert_eq!(
        schema["properties"]["cursor"]["required"],
        json!(["block_height", "tx_index", "log_index"])
    );
    assert_eq!(validate("subscription.acknowledge", &request), Ok(()));
    for scalar in [json!(12), json!("12"), Value::Null] {
        let mut invalid = request.clone();
        invalid["cursor"] = scalar;
        assert!(validate("subscription.acknowledge", &invalid).is_err());
    }
    for field in ["block_height", "tx_index", "log_index"] {
        let mut missing = request.clone();
        missing["cursor"]
            .as_object_mut()
            .expect("cursor object")
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
    let create = json!({
        "scope": request["scope"],
        "filter": {"agents": [], "accounts": [], "activity_types": [], "modules": [], "assets": [], "counterparties": [], "result_classes": []},
        "start": request["cursor"],
        "delivery_target": "uds://consumer-a",
    });
    assert_eq!(validate("subscription.create", &create), Ok(()));
    use layerx_agent_api::identity::{
        AgentDid, CapabilityId, ExplicitSet, TenantId as ApiTenantId,
    };
    use layerx_agent_api::subscription::{
        DeliveryTarget, SubscriptionCreate, SubscriptionFilter, SubscriptionId, SubscriptionScope,
        SubscriptionTarget,
    };
    use layerx_agentd::events::subscription::Store as SubscriptionStore;
    use layerx_agentd::store::{Store, TenantId};
    use layerx_mcp::catalogue::subscription_acknowledge_tool;

    let root = std::path::PathBuf::from("/tmp").join(format!(
        "layerx-mcp-major-two-subscription-{}",
        std::process::id()
    ));
    let durable = Store::open(&root).expect("new durable store");
    let mut store = SubscriptionStore::open(durable, TenantId::new("tenant-a").expect("tenant"))
        .expect("subscription store");
    let scope = SubscriptionScope {
        tenant: ApiTenantId::new("tenant-a").expect("API tenant"),
        agent: AgentDid::new("did:layerx:agent-a").expect("agent"),
        capability: CapabilityId::new("capability-a").expect("capability"),
    };
    let subscription_id = SubscriptionId::new("subscription-a").expect("subscription ID");
    let start = Cursor {
        block_height: 0,
        tx_index: 0,
        log_index: 0,
    };
    store
        .create(
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
                delivery_target: DeliveryTarget::new("uds://consumer-a").expect("delivery target"),
            },
        )
        .expect("create subscription");
    let target = SubscriptionTarget {
        scope,
        subscription_id,
    };
    assert!(
        subscription_acknowledge_tool(&mut store, &request).is_err(),
        "undelivered cursor refused"
    );
    store
        .mark_delivered(&target, cursor.into())
        .expect("mark cursor delivered");
    let record =
        subscription_acknowledge_tool(&mut store, &request).expect("acknowledge cursor triple");
    assert_eq!(record.last_acknowledged, cursor);
    assert_eq!(
        store
            .get(&target)
            .expect("stored subscription")
            .last_acknowledged,
        cursor
    );
    for scalar in [json!(12), json!("12")] {
        let mut scalar_request = request.clone();
        scalar_request["cursor"] = scalar;
        assert!(subscription_acknowledge_tool(&mut store, &scalar_request).is_err());
        assert_eq!(
            store
                .get(&target)
                .expect("unchanged subscription")
                .last_acknowledged,
            cursor
        );
    }
    drop(store);
    std::fs::remove_dir_all(root).expect("remove durable store");
    let mut invalid = create;
    invalid["start"] = json!("12");
    assert!(validate("subscription.create", &invalid).is_err());
}

#[path = "approval.rs"]
mod approval;

#[test]
fn approval_get_tool_reports_tx_hash_and_rung() {
    use layerx_agent_api::track::{SubmissionRef, SubmissionState, TrackedSubmission};
    use layerx_agentd::approval::{
        ApprovalExpiry, ApprovalOutcome, ApprovalService, ApprovalSubmissionQueue, DecisionKey,
        DecisionRequest,
    };
    use layerx_agentd::budget::BudgetLimiter;
    use layerx_agentd::policy::approval::{ApprovalRegistry, ApproverId};
    use layerx_mcp::approval::{require, ApprovalPolicy, Requirement};
    use layerx_mcp::catalogue::approval_get_tool;
    use layerx_types::rung::Rung;
    use std::collections::BTreeMap;

    let registry = ApprovalRegistry::default();
    let context = approval::context(15);
    let prepared = approval::prepared(101, 15);
    let Requirement::Required(ticket) = require(
        &registry,
        ApprovalPolicy {
            amount_threshold: 100,
        },
        context.clone(),
        prepared.clone(),
        50,
    )
    .expect("register exact approval") else {
        panic!("approval required");
    };
    let root = std::path::PathBuf::from("/tmp").join(format!(
        "layerx-mcp-major-two-approval-{}",
        std::process::id()
    ));
    let expiry = ApprovalExpiry::open(&root).expect("new approval decision store");
    let limiter = BudgetLimiter::new(Vec::new()).expect("budget limiter");
    let service = ApprovalService::new(&registry, &limiter, &expiry);
    let queue = ApprovalSubmissionQueue::default();
    let decision = service
        .approve(
            DecisionRequest {
                tenant: &context.tenant,
                approval_id: ticket.hold_id,
                idempotency_key: &DecisionKey::new("major-two-approve").expect("decision key"),
                approver: ApproverId::new("human:alice").expect("approver"),
                current_sequence: 60,
            },
            &prepared,
            &queue,
        )
        .expect("release registered approval");
    assert_eq!(decision.outcome, ApprovalOutcome::Granted);
    let release_ref = decision.submission_ref.expect("approval release reference");
    assert_eq!(queue.prepared(release_ref), Some(prepared.clone()));
    let approval_id = ticket
        .hold_id
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<String>();
    let tx_hash = "ab".repeat(32);
    assert_ne!(
        tx_hash,
        release_ref
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    );
    let tracked = TrackedSubmission {
        submission_ref: SubmissionRef::new(tx_hash.clone()).expect("transaction hash"),
        state: SubmissionState::Submitted,
        evidence: Vec::new(),
        rung: Rung::Submitted,
        transitions: Vec::new(),
    };
    let submissions = BTreeMap::from([(release_ref, tracked)]);
    let arguments = json!({"approval_id": approval_id, "current_sequence": "60"});
    assert_eq!(validate("approval.get", &arguments), Ok(()));
    let facts = approval_get_tool(&service, &context.tenant, &submissions, &arguments)
        .expect("approval.get tool response");
    approval::assert_major_two_approval_facts(
        &facts,
        &approval_id,
        &tx_hash,
        Rung::Submitted.name(),
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
    drop(service);
    drop(expiry);
    std::fs::remove_dir_all(root).expect("remove approval decision store");
}
