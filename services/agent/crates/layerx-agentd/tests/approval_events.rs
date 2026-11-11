#[path = "gaps.rs"]
mod gap_integration;
use gap_integration::{HistoryServer, SubscriptionServer};

use std::collections::BTreeSet;
use std::fs;
use std::os::unix::fs::DirBuilderExt;
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
use layerx_agentd::read::{Cursor as ChainCursor, HistoryLimits};
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
    let wire_log = assert_ok(serde_json::from_slice(&recovered_bytes), "unexpected error");
    let logs_server = SubscriptionServer::new(json!(["logs", {"topics": []}]), vec![wire_log]);
    let mut logs = assert_ok(
        subscribe(
            &logs_server.url,
            &[],
            SubscriptionTopic::Logs {
                address: None,
                topics: vec![],
            },
        ),
        "unexpected error",
    );
    let mut ingestor = EventIngestor::open(durable, tenant(), 16, 1)
        .unwrap_or_else(|error| panic!("recovery ingestor: {error}"));
    assert_ok(
        ingestor.read_log(&mut logs, attributes.clone()),
        "unexpected error",
    );
    logs_server.finish();
    let history = HistoryServer::new(vec![(
        1,
        json!([assert_ok(
            serde_json::from_slice::<serde_json::Value>(&recovered_bytes),
            "unexpected error"
        )]),
    )]);
    let source = history.connect();
    let heads_server = SubscriptionServer::new(json!(["newHeads"]), vec![json!({"number":"0x2"})]);
    let mut heads = assert_ok(
        subscribe(&heads_server.url, &[], SubscriptionTopic::NewHeads),
        "unexpected error",
    );
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
    let mut subscriptions = stream_subscriptions(ingestor.into_store());
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

fn stream_subscriptions(durable: Store) -> SubscriptionStore {
    let mut subscriptions = SubscriptionStore::open(durable, tenant())
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
    subscriptions
}

fn delivery_engine(subscriptions: SubscriptionStore) -> DeliveryEngine {
    delivery_engine_at(subscriptions, 4)
}

fn delivery_engine_at(subscriptions: SubscriptionStore, live_start: u64) -> DeliveryEngine {
    DeliveryEngine::open(
        subscriptions,
        target(),
        live_start,
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
    let mut previous_cursor = None;
    refill_backfill(delivery, "backfill");
    while let Some(item) =
        deliver(delivery).unwrap_or_else(|error| panic!("delivery attempt: {error}"))
    {
        let accepted = delivery
            .accept_front(1_000)
            .unwrap_or_else(|error| panic!("accept: {error}"));
        assert_eq!(accepted, item);
        if let DeliveryItem::Event(event) = accepted {
            assert!(previous_cursor.is_none_or(|previous| previous < event.delivery.cursor));
            previous_cursor = Some(event.delivery.cursor);
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
    fs::DirBuilder::new()
        .mode(0o700)
        .create(&root)
        .unwrap_or_else(|error| panic!("root: {error}"));
    let root = fs::canonicalize(root).unwrap_or_else(|error| panic!("canonical root: {error}"));
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

#[test]
fn local_approval_between_closed_blocks_keeps_journal_order_after_restart() {
    assert_approval_at_block_boundary(false);
}

#[test]
fn local_approval_while_next_block_is_pending_keeps_journal_order_after_restart() {
    assert_approval_at_block_boundary(true);
}

fn assert_approval_at_block_boundary(queue_next_block_first: bool) {
    let root = directory();
    let mut ingestor = assert_ok(
        EventIngestor::open(
            assert_ok(Store::open(&root), "unexpected error"),
            tenant(),
            8,
            0,
        ),
        "unexpected error",
    );
    let attributes = boundary_attributes();
    let BoundaryLogs {
        first,
        second,
        history,
        source,
        limits,
    } = first_boundary_block(&mut ingestor, queue_next_block_first, &attributes);
    let mut audit = assert_ok(Log::open(&root, &tenant()), "unexpected error");
    let emission = assert_ok(
        ApprovalEvents::emit(
            &mut ingestor,
            &mut audit,
            &mut Coverage::default(),
            &lifecycle(ApprovalEventKind::Created, 1, 100),
        ),
        "unexpected error",
    );
    assert_eq!(emission.global_sequence, 2);
    drop(audit);
    drop(ingestor);

    let mut ingestor = assert_ok(
        EventIngestor::open(
            assert_ok(Store::open(&root), "unexpected error"),
            tenant(),
            8,
            0,
        ),
        "unexpected error",
    );
    assert_eq!(
        ingestor.watermark().last_position,
        Some(ChainCursor {
            block_height: 12,
            tx_index: 0,
            log_index: 1,
        })
    );
    let mut audit = assert_ok(Log::open(&root, &tenant()), "unexpected error");
    let next_emission = assert_ok(
        ApprovalEvents::emit(
            &mut ingestor,
            &mut audit,
            &mut Coverage::default(),
            &lifecycle(ApprovalEventKind::Granted, 1, 101),
        ),
        "unexpected error",
    );
    assert_eq!(next_emission.global_sequence, 3);
    if !queue_next_block_first {
        for log in &second {
            assert_ok(
                ingestor.ingest_log(log, attributes.clone()),
                "unexpected error",
            );
        }
    }
    assert_eq!(ingestor.watermark().next_expected, 4);
    assert_ok(
        ingestor.new_head(14, &source, limits, 8, attributes),
        "unexpected error",
    );
    history.finish();
    assert_eq!(ingestor.watermark().next_expected, 6);
    let final_emission = assert_ok(
        ApprovalEvents::emit(
            &mut ingestor,
            &mut audit,
            &mut Coverage::default(),
            &lifecycle(ApprovalEventKind::Created, 2, 102),
        ),
        "unexpected error",
    );
    drop(audit);
    drop(ingestor);

    assert_boundary_delivery(
        &root,
        &first,
        &second,
        emission,
        next_emission,
        final_emission,
    );
    assert_ok(fs::remove_dir_all(root), "unexpected error");
}

#[track_caller]
fn assert_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{message}: {error:?}"),
    }
}

#[track_caller]
fn assert_some<T>(value: Option<T>, message: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{message}"),
    }
}

fn assert_boundary_delivery(
    root: &Path,
    first: &[serde_json::Value; 2],
    second: &[serde_json::Value; 2],
    emission: ApprovalEmission,
    next_emission: ApprovalEmission,
    final_emission: ApprovalEmission,
) {
    let subscriptions = stream_subscriptions(assert_ok(Store::open(root), "unexpected error"));
    let mut engine = delivery_engine_at(subscriptions, 7);
    // 2985: raw log positions stay exact; local counters start at zero for each closed block.
    let expected = [
        (12, 0, 0),
        (12, 0, 1),
        (12, u32::MAX, 0),
        (12, u32::MAX, 1),
        (13, 0, 0),
        (13, 0, 1),
        (13, u32::MAX, 0),
    ];
    let mut delivered = Vec::new();
    let mut previous = None;
    for (sequence, (block_height, tx_index, log_index)) in expected.into_iter().enumerate() {
        let item = assert_some(
            assert_ok(deliver(&mut engine), "unexpected error"),
            "ordered event",
        );
        let DeliveryItem::Event(event) = &item else {
            panic!("event before seam")
        };
        assert_eq!(event.global_sequence, sequence as u64);
        assert_eq!(
            event.delivery.cursor,
            ApiCursor {
                block_height,
                tx_index,
                log_index
            }
        );
        assert!(previous.is_none_or(|cursor| cursor < event.delivery.cursor));
        previous = Some(event.delivery.cursor);
        if sequence == 2 {
            let original = event.clone();
            assert_eq!(
                assert_ok(engine.accept_front(100), "unexpected error"),
                item
            );
            drop(engine);
            engine = delivery_engine_at(
                assert_ok(
                    SubscriptionStore::open(
                        assert_ok(Store::open(root), "unexpected error"),
                        tenant(),
                    ),
                    "unexpected error",
                ),
                7,
            );
            let replay = assert_some(
                assert_ok(deliver(&mut engine), "unexpected error"),
                "unacknowledged approval",
            );
            assert_eq!(replay, DeliveryItem::Event(original));
            assert_eq!(
                assert_ok(engine.accept_front(101), "unexpected error"),
                replay
            );
        } else {
            assert_eq!(
                assert_ok(engine.accept_front(100), "unexpected error"),
                item
            );
        }
        let DeliveryItem::Event(event) = item else {
            unreachable!()
        };
        assert_ok(
            engine.acknowledge(&CursorAcknowledgement {
                scope: scope(),
                subscription_id: target().subscription_id,
                cursor: event.delivery.cursor,
            }),
            "unexpected error",
        );
        delivered.push(event.delivery.event_bytes);
    }
    assert_eq!(
        delivered,
        vec![
            assert_ok(serde_json::to_vec(&first[0]), "unexpected error"),
            assert_ok(serde_json::to_vec(&first[1]), "unexpected error"),
            emission.canonical_event_bytes,
            next_emission.canonical_event_bytes,
            assert_ok(serde_json::to_vec(&second[0]), "unexpected error"),
            assert_ok(serde_json::to_vec(&second[1]), "unexpected error"),
            final_emission.canonical_event_bytes
        ]
    );
    assert!(matches!(
        assert_ok(deliver(&mut engine), "unexpected error"),
        Some(DeliveryItem::BackfillComplete(_))
    ));
    assert_ok(engine.accept_front(102), "unexpected error");
    assert!(assert_ok(deliver(&mut engine), "unexpected error").is_none());
}

struct BoundaryLogs {
    first: [serde_json::Value; 2],
    second: [serde_json::Value; 2],
    history: HistoryServer,
    source: layerx_agentd::events::gap::EthLogSource,
    limits: HistoryLimits,
}
fn first_boundary_block(
    ingestor: &mut EventIngestor,
    queue_next_block_first: bool,
    attributes: &EventAttributes,
) -> BoundaryLogs {
    let first = [
        gap_integration::log(12, 0, 0),
        gap_integration::log(12, 0, 1),
    ];
    let second = [
        gap_integration::log(13, 0, 0),
        gap_integration::log(13, 0, 1),
    ];
    let history = HistoryServer::new(vec![]);
    let source = history.connect();
    let limits = HistoryLimits {
        maximum_items: 8,
        maximum_response_bytes: 1_048_576,
    };
    for log in &first {
        assert_ok(
            ingestor.ingest_log(log, attributes.clone()),
            "unexpected error",
        );
    }
    assert_eq!(ingestor.watermark().next_expected, 0);
    assert_ok(
        ingestor.new_head(13, &source, limits, 8, attributes.clone()),
        "unexpected error",
    );
    assert_eq!(ingestor.watermark().next_expected, 2);
    if queue_next_block_first {
        for log in &second {
            assert_ok(
                ingestor.ingest_log(log, attributes.clone()),
                "unexpected error",
            );
        }
        assert_eq!(ingestor.watermark().next_expected, 2);
    }
    BoundaryLogs {
        first,
        second,
        history,
        source,
        limits,
    }
}

fn boundary_attributes() -> EventAttributes {
    EventAttributes {
        agent: "did:layerx:approval-events".into(),
        account: "tenant-a".into(),
        activity_type: 7,
        module: "approval".into(),
        asset: "LXP".into(),
        counterparty: "human-approval".into(),
        result_code: 0,
    }
}
