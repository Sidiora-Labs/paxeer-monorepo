#[path = "gaps.rs"]
mod gap_integration;
use gap_integration::{HistoryServer, SubscriptionServer};

use std::collections::BTreeSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agent_api::identity::{
    ActivityType, AgentDid, Asset, CapabilityId as ApiCapabilityId, Counterparty, ExplicitSet,
    TenantId as ApiTenantId,
};
use layerx_agent_api::read::{AccountRef, ModuleRef};
use layerx_agent_api::subscription::{
    Cursor as ApiCursor, CursorAcknowledgement, DeliveryTarget, SubscriptionCreate,
    SubscriptionFilter, SubscriptionId, SubscriptionScope, SubscriptionTarget, TenantObject,
};
use layerx_agentd::approval::{
    ApprovalEmission, ApprovalEventKind, ApprovalEvents, ApprovalLifecycle,
    APPROVAL_ENFORCEMENT_NOTICE,
};
use layerx_agentd::audit::{
    export, Coverage, Decision, EventClass, EvidenceStore, Log, PayloadEvidence, Query,
};
use layerx_agentd::events::gap::{
    admit, apply_backfill, attempt_backfill, detect, BackfillResolution,
};
use layerx_agentd::events::subscription::Store as SubscriptionStore;
use layerx_agentd::events::{
    backfill, deliver, DeliveryEngine, DeliveryError, DeliveryItem, EventAttributes, EventIngestor,
    RetryPolicy,
};
use layerx_agentd::read::HistoryLimits;
use layerx_agentd::session::SessionId;
use layerx_agentd::store::{Store, TenantId};
use layerx_agentd::tenant::{Config, RedactionPolicy, Retention};
use layerx_client::subscription::{subscribe, SubscriptionTopic};
use layerx_types::ids::Did;
use layerx_types::result::ResultCode;
use layerx_types::verify::VerificationLevel;
use serde_json::json;
use sha2::{Digest as _, Sha256};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

fn directory() -> PathBuf {
    let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-approval-events-{}-{sequence}",
        std::process::id()
    ))
}

fn tenant() -> TenantId {
    TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}"))
}

fn api_tenant() -> ApiTenantId {
    ApiTenantId::new("tenant-a").unwrap_or_else(|error| panic!("API tenant: {error:?}"))
}

fn agent() -> Did {
    Did::new(b"did:layerx:approval-events").unwrap_or_else(|error| panic!("agent: {error:?}"))
}

fn prepared(id: u8) -> layerx_agent_api::prepare::Prepared {
    use layerx_agent_api::identity::{Asset, AuthorityRef, ExplicitSet};
    use layerx_agent_api::prepare::{
        CanonicalBytes, Disclosure, IdempotencyRef, PreparationRef, Prepared, SigningPreimage,
    };
    use layerx_agent_api::{Amount, TimestampSeconds};

    let call = layerx_crypto::evm_transaction::Eip1559Call {
        chain_id: 125,
        nonce: u64::from(id),
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: [3; 20],
        data: [
            layerx_crypto::disclosure::call::ledger_selector("transfer(address,uint256)").to_vec(),
            [vec![0; 12], vec![5; 20]].concat(),
            [vec![0; 31], vec![50]].concat(),
        ]
        .concat(),
    };
    let bytes = call.encode_unsigned();
    Prepared {
        preparation_ref: PreparationRef::new(format!("prepared-{id}"))
            .unwrap_or_else(|error| panic!("preparation: {error:?}")),
        unsigned_canonical_bytes: CanonicalBytes::new(bytes.clone())
            .unwrap_or_else(|error| panic!("bytes: {error:?}")),
        signing_preimage: SigningPreimage::new(call.signing_hash().to_vec())
            .unwrap_or_else(|error| panic!("preimage: {error:?}")),
        disclosure: Disclosure {
            canonical_digest: Sha256::digest(bytes).into(),
            activity_type: ActivityType(7),
            actor: AgentDid::new("0404040404040404040404040404040404040404")
                .unwrap_or_else(|error| panic!("actor: {error:?}")),
            authority: AuthorityRef::new("session-key")
                .unwrap_or_else(|error| panic!("authority: {error:?}")),
            counterparties: ExplicitSet::deny_all(),
            amounts: ExplicitSet::deny_all(),
            asset: Asset::new("LXP").unwrap_or_else(|error| panic!("asset: {error:?}")),
            fee_limit: Amount(2),
            expiry: TimestampSeconds(500),
            idempotency_key: IdempotencyRef::new(format!("idempotency-{id}"))
                .unwrap_or_else(|error| panic!("key: {error:?}")),
        },
        expiry: TimestampSeconds(500),
    }
}

fn lifecycle(kind: ApprovalEventKind, id: u8, observed_at_ms: u64) -> ApprovalLifecycle {
    ApprovalLifecycle {
        tenant: tenant(),
        agent: agent(),
        session: SessionId([2; 32]),
        capability: [3; 32],
        policy_version: "policy-v4".to_owned(),
        approval_id: [id; 32],
        canonical_digest: prepared(id).disclosure.canonical_digest,
        activity_type: 7,
        asset: "LXP".to_owned(),
        kind,
        principal: matches!(
            kind,
            ApprovalEventKind::Granted | ApprovalEventKind::Rejected
        )
        .then(|| "human:operator".to_owned()),
        observed_at_ms,
    }
}

fn scope() -> SubscriptionScope {
    SubscriptionScope {
        tenant: api_tenant(),
        agent: AgentDid::new("did:layerx:approval-events")
            .unwrap_or_else(|error| panic!("API agent: {error:?}")),
        capability: ApiCapabilityId::new("approval-events")
            .unwrap_or_else(|error| panic!("API capability: {error:?}")),
    }
}

fn target() -> SubscriptionTarget {
    SubscriptionTarget {
        scope: scope(),
        subscription_id: SubscriptionId::new("approval-lifecycle")
            .unwrap_or_else(|error| panic!("subscription: {error:?}")),
    }
}

fn filter() -> SubscriptionFilter {
    SubscriptionFilter {
        agents: ExplicitSet::allow(vec![TenantObject {
            tenant: api_tenant(),
            value: AgentDid::new("did:layerx:approval-events")
                .unwrap_or_else(|error| panic!("filter agent: {error:?}")),
        }]),
        accounts: ExplicitSet::allow(vec![TenantObject {
            tenant: api_tenant(),
            value: AccountRef::new("tenant-a").unwrap_or_else(|error| panic!("account: {error:?}")),
        }]),
        activity_types: ExplicitSet::allow(vec![ActivityType(7)]),
        modules: ExplicitSet::allow(vec![TenantObject {
            tenant: api_tenant(),
            value: ModuleRef::new("approval").unwrap_or_else(|error| panic!("module: {error:?}")),
        }]),
        assets: ExplicitSet::allow(vec![TenantObject {
            tenant: api_tenant(),
            value: Asset::new("LXP").unwrap_or_else(|error| panic!("asset: {error:?}")),
        }]),
        counterparties: ExplicitSet::allow(vec![TenantObject {
            tenant: api_tenant(),
            value: Counterparty::new("human-approval")
                .unwrap_or_else(|error| panic!("counterparty: {error:?}")),
        }]),
        result_classes: ExplicitSet::allow(vec![
            ResultCode::from_raw(0),
            ResultCode::from_raw(-1),
            ResultCode::from_raw(-2),
        ]),
    }
}

fn config() -> Config {
    Config {
        tenant: tenant(),
        policy_version: "policy-v4".to_owned(),
        redaction: RedactionPolicy::Standard,
        retention: Retention {
            events: 100,
            audit: 100,
            receipts: 100,
        },
        verification_default: VerificationLevel::STATE_PROVEN,
        approval_required_for: BTreeSet::from([7]),
    }
}

fn emit_lifecycles(
    ingestor: &mut EventIngestor,
    audit: &mut Log,
    coverage: &mut Coverage,
    lifecycles: &[ApprovalLifecycle],
) -> Vec<ApprovalEmission> {
    lifecycles
        .iter()
        .map(|event| {
            ApprovalEvents::emit(ingestor, audit, coverage, event)
                .unwrap_or_else(|error| panic!("emit: {error}"))
        })
        .collect()
}

fn subscriptions_restored_after_gap(durable: Store) -> (SubscriptionStore, Vec<u8>) {
    let recovered_bytes = serde_json::to_vec(&json!({
        "blockNumber": "0x1", "transactionIndex": "0x0", "logIndex": "0x0",
        "blockHash": format!("0x{}", "ab".repeat(32)),
        "address": format!("0x{}", "12".repeat(20)),
        "topics": [format!("0x{}", "34".repeat(32))],
        "data": "0x00ff", "removed": false,
    }))
    .unwrap_or_else(|error| panic!("recovered log bytes: {error}"));
    let attributes = EventAttributes {
        agent: "did:layerx:approval-events".to_owned(),
        account: "tenant-a".to_owned(),
        activity_type: 7,
        module: "approval".to_owned(),
        asset: "LXP".to_owned(),
        counterparty: "human-approval".to_owned(),
        result_code: 0,
    };
    let wire_log = serde_json::from_slice(&recovered_bytes).unwrap();
    let logs_server = SubscriptionServer::new(json!(["logs", {"topics": []}]), vec![wire_log]);
    let mut logs = subscribe(
        &logs_server.url,
        &[],
        SubscriptionTopic::Logs {
            address: None,
            topics: vec![],
        },
    )
    .unwrap();
    let mut ingestor = EventIngestor::open(durable, tenant(), 16, 1)
        .unwrap_or_else(|error| panic!("recovery ingestor: {error}"));
    ingestor.read_log(&mut logs, attributes.clone()).unwrap();
    logs_server.finish();
    let history = HistoryServer::new(vec![(
        1,
        json!([serde_json::from_slice::<serde_json::Value>(&recovered_bytes).unwrap()]),
    )]);
    let source = history.connect();
    let heads_server = SubscriptionServer::new(json!(["newHeads"]), vec![json!({"number":"0x2"})]);
    let mut heads = subscribe(&heads_server.url, &[], SubscriptionTopic::NewHeads).unwrap();
    ingestor
        .read_head(
            &mut heads,
            &source,
            HistoryLimits {
                maximum_items: 2,
                maximum_response_bytes: 1_048_576,
            },
            2,
            attributes,
        )
        .unwrap_or_else(|error| panic!("close recovered block: {error}"));
    heads_server.finish();
    let mut subscriptions = SubscriptionStore::open(ingestor.into_store(), tenant())
        .unwrap_or_else(|error| panic!("subscriptions: {error}"));
    subscriptions
        .create(
            target().subscription_id,
            SubscriptionCreate {
                scope: scope(),
                filter: filter(),
                start: ApiCursor {
                    block_height: 0,
                    tx_index: 0,
                    log_index: 0,
                },
                delivery_target: DeliveryTarget::new("uds://approval-consumer")
                    .unwrap_or_else(|error| panic!("target: {error:?}")),
            },
        )
        .unwrap_or_else(|error| panic!("create: {error}"));
    let gap = detect(&mut subscriptions, &target(), 1, 2)
        .unwrap_or_else(|error| panic!("detect gap: {error}"))
        .unwrap_or_else(|| panic!("gap missing"));
    assert!(admit(&subscriptions, &target()).is_err());
    let report = attempt_backfill(
        &source,
        gap,
        HistoryLimits {
            maximum_items: 2,
            maximum_response_bytes: 1_048_576,
        },
        2,
    );
    history.finish();
    assert!(matches!(
        apply_backfill(&mut subscriptions, &target(), gap, &report),
        Ok(BackfillResolution::Restored)
    ));
    assert!(admit(&subscriptions, &target()).is_ok());
    (subscriptions, recovered_bytes)
}

fn delivery_engine(subscriptions: SubscriptionStore) -> DeliveryEngine {
    DeliveryEngine::open(
        subscriptions,
        target(),
        4,
        8,
        RetryPolicy {
            base_delay_ms: 10,
            maximum_delay_ms: 100,
            jitter_percent: 0,
            maximum_attempts: 3,
        },
    )
    .unwrap_or_else(|error| panic!("delivery: {error}"))
}

fn refill_backfill(delivery: &mut DeliveryEngine, label: &str) {
    match backfill(delivery) {
        Ok(_) | Err(DeliveryError::Backpressure { .. }) => {}
        Err(error) => panic!("{label}: {error}"),
    }
}

fn drain_delivered(delivery: &mut DeliveryEngine) -> Vec<Vec<u8>> {
    let mut delivered = Vec::new();
    refill_backfill(delivery, "backfill");
    while let Some(item) =
        deliver(delivery).unwrap_or_else(|error| panic!("delivery attempt: {error}"))
    {
        let accepted = delivery
            .accept_front(1_000)
            .unwrap_or_else(|error| panic!("accept: {error}"));
        assert_eq!(accepted, item);
        if let DeliveryItem::Event(event) = accepted {
            delivered.push(event.delivery.event_bytes.clone());
            delivery
                .acknowledge(&CursorAcknowledgement {
                    scope: scope(),
                    subscription_id: target().subscription_id,
                    cursor: event.delivery.cursor,
                })
                .unwrap_or_else(|error| panic!("acknowledge: {error}"));
        }
        refill_backfill(delivery, "continued backfill");
    }
    delivered
}

fn assert_digest_evidence_exported(audit_path: &Path, lifecycles: &[ApprovalLifecycle]) {
    let exported = export(
        audit_path,
        &config(),
        Query {
            tenant: tenant(),
            agent: None,
            from_observed_at_ms: None,
            through_observed_at_ms: None,
        },
        &EvidenceStore::new(tenant()),
    )
    .unwrap_or_else(|error| panic!("export: {error}"));
    assert_eq!(exported.entries.len(), 4);
    for (entry, lifecycle) in exported.entries.iter().zip(lifecycles) {
        assert_eq!(entry.entry.class, EventClass::PolicyDecision);
        assert_eq!(
            entry.entry.submitted_bytes,
            Some(PayloadEvidence::Digest(lifecycle.canonical_digest))
        );
        assert!(entry.entry.protocol_authority.is_none());
        assert!(entry.entry.reason.as_str().contains("daemon-enforced"));
        assert!(entry
            .entry
            .reason
            .as_str()
            .contains("no protocol authority"));
    }
    assert_eq!(
        exported
            .entries
            .iter()
            .map(|entry| entry.entry.decision)
            .collect::<Vec<_>>(),
        vec![
            Decision::Requested,
            Decision::Allowed,
            Decision::Refused,
            Decision::Failed,
        ]
    );
}

#[test]
fn approval_lifecycle_stream_resumes_after_gap_and_exports_digest_evidence() {
    let root = directory();
    fs::create_dir_all(&root).unwrap_or_else(|error| panic!("root: {error}"));
    let durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let mut ingestor = EventIngestor::open(durable, tenant(), 16, 0)
        .unwrap_or_else(|error| panic!("ingestor: {error}"));
    let mut audit = Log::open(&root, &tenant()).unwrap_or_else(|error| panic!("audit: {error}"));
    let audit_path = audit.path().to_path_buf();
    let mut coverage = Coverage::default();
    let lifecycles = [
        lifecycle(ApprovalEventKind::Created, 1, 100),
        lifecycle(ApprovalEventKind::Granted, 1, 101),
        lifecycle(ApprovalEventKind::Rejected, 2, 102),
        lifecycle(ApprovalEventKind::Expired, 3, 103),
    ];
    let emissions = emit_lifecycles(&mut ingestor, &mut audit, &mut coverage, &lifecycles);
    assert_eq!(ingestor.watermark().next_expected, 4);
    assert_eq!(coverage.count(EventClass::PolicyDecision), 4);
    assert!(emissions
        .iter()
        .all(|event| event.enforcement_notice == APPROVAL_ENFORCEMENT_NOTICE));
    drop(audit);

    let (subscriptions, recovered_bytes) = subscriptions_restored_after_gap(ingestor.into_store());
    let mut delivery = delivery_engine(subscriptions);
    let delivered = drain_delivered(&mut delivery);
    assert_eq!(
        delivered,
        emissions
            .iter()
            .map(|event| event.canonical_event_bytes.clone())
            .chain(std::iter::once(recovered_bytes))
            .collect::<Vec<_>>()
    );

    assert_digest_evidence_exported(&audit_path, &lifecycles);
    let _ = fs::remove_dir_all(root);
}
