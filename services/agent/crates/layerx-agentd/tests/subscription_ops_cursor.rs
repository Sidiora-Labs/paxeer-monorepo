#[path = "gaps.rs"]
mod gap_integration;
use gap_integration::{HistoryServer, SubscriptionServer};

use std::fs;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agent_api::error::{ErrorClass, RequestId, Retriability};
use layerx_agent_api::identity::{AgentDid, CapabilityId, ExplicitSet, TenantId as ApiTenantId};
use layerx_agent_api::subscription::{
    CursorAcknowledgement, DeliveryTarget, SubscriptionCreate, SubscriptionFilter, SubscriptionId,
    SubscriptionScope, SubscriptionTarget,
};
use layerx_agentd::agent_rpc_adapters::SubscriptionCursorSnapshot;
use layerx_agentd::events::subscription::{Cursor, Store as SubscriptionStore, SubscriptionError};
use layerx_agentd::events::{EventAttributes, EventIngestor};
use layerx_agentd::read::HistoryLimits;
use layerx_agentd::session::{SessionCredential, SessionId};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::subscription::{subscribe, SubscriptionTopic};
use serde_json::{json, Value};

static NEXT: AtomicU64 = AtomicU64::new(1);

struct Fixture {
    root: PathBuf,
    subscriptions: Option<SubscriptionStore>,
    target: SubscriptionTarget,
}

impl Fixture {
    fn new() -> Self {
        let root = std::env::temp_dir().join(format!(
            "subscription-ops-cursor-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed),
        ));
        let tenant = TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error:?}"));
        let scope = SubscriptionScope {
            tenant: ApiTenantId::new("tenant-a")
                .unwrap_or_else(|error| panic!("API tenant: {error:?}")),
            agent: AgentDid::new("agent-a").unwrap_or_else(|error| panic!("agent: {error:?}")),
            capability: CapabilityId::new("capability-a")
                .unwrap_or_else(|error| panic!("capability: {error:?}")),
        };
        let id = SubscriptionId::new("subscription-a")
            .unwrap_or_else(|error| panic!("subscription ID: {error:?}"));
        let mut subscriptions = SubscriptionStore::open(
            Store::open(&root).unwrap_or_else(|error| panic!("durable store: {error:?}")),
            tenant,
        )
        .unwrap_or_else(|error| panic!("subscription store: {error:?}"));
        subscriptions
            .create(
                id.clone(),
                SubscriptionCreate {
                    scope: scope.clone(),
                    filter: SubscriptionFilter {
                        agents: ExplicitSet::allow(Vec::new()),
                        accounts: ExplicitSet::allow(Vec::new()),
                        activity_types: ExplicitSet::allow(Vec::new()),
                        modules: ExplicitSet::allow(Vec::new()),
                        assets: ExplicitSet::allow(Vec::new()),
                        counterparties: ExplicitSet::allow(Vec::new()),
                        result_classes: ExplicitSet::allow(Vec::new()),
                    },
                    start: position(12, 0, 0).into(),
                    delivery_target: DeliveryTarget::new("uds://consumer-a")
                        .unwrap_or_else(|error| panic!("delivery target: {error:?}")),
                },
            )
            .unwrap_or_else(|error| {
                panic!("create subscription with a block-position cursor: {error:?}")
            });
        Self {
            root,
            subscriptions: Some(subscriptions),
            target: SubscriptionTarget {
                scope,
                subscription_id: id,
            },
        }
    }

    fn subscriptions(&mut self) -> &mut SubscriptionStore {
        self.subscriptions
            .as_mut()
            .unwrap_or_else(|| panic!("open subscription store"))
    }

    fn snapshot(&self) -> SubscriptionCursorSnapshot {
        let durable = self
            .subscriptions
            .as_ref()
            .unwrap_or_else(|| panic!("subscriptions"))
            .durable()
            .unwrap_or_else(|error| panic!("durable view: {error:?}"));
        SubscriptionCursorSnapshot::read(&durable, &self.target, None, RequestId(7))
            .unwrap_or_else(|error| panic!("read cursor state through the RPC adapter: {error:?}"))
    }

    fn ingest(&mut self, positions: &[Cursor]) {
        let subscriptions = self
            .subscriptions
            .take()
            .unwrap_or_else(|| panic!("subscriptions"));
        let durable = subscriptions
            .into_durable()
            .unwrap_or_else(|error| panic!("owned durable store: {error:?}"));
        let tenant = TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error:?}"));
        let mut ingestor = EventIngestor::open(durable, tenant.clone(), 8, 0)
            .unwrap_or_else(|error| panic!("ingestor: {error:?}"));
        let wire_logs = positions.iter().map(|cursor| log(*cursor)).collect();
        let logs_server = SubscriptionServer::new(json!(["logs", {"topics": []}]), wire_logs);
        let mut logs = subscribe(
            &logs_server.url,
            &[],
            SubscriptionTopic::Logs {
                address: None,
                topics: vec![],
            },
        )
        .unwrap_or_else(|error| panic!("log subscription: {error:?}"));
        for _ in positions {
            ingestor
                .read_log(&mut logs, attributes())
                .unwrap_or_else(|error| panic!("ingest Ethereum log: {error:?}"));
        }
        logs_server.finish();
        let history = HistoryServer::new(vec![]);
        let source = history.connect();
        let heads_server =
            SubscriptionServer::new(json!(["newHeads"]), vec![json!({"number":"0xe"})]);
        let mut heads = subscribe(&heads_server.url, &[], SubscriptionTopic::NewHeads)
            .unwrap_or_else(|error| panic!("head subscription: {error:?}"));
        ingestor
            .read_head(
                &mut heads,
                &source,
                HistoryLimits {
                    maximum_items: 8,
                    maximum_response_bytes: 1_048_576,
                },
                8,
                attributes(),
            )
            .unwrap_or_else(|error| panic!("flush logs from closed blocks: {error:?}"));
        heads_server.finish();
        history.finish();
        assert_eq!(
            ingestor.watermark().last_position.map(|cursor| position(
                cursor.block_height,
                cursor.tx_index,
                cursor.log_index
            )),
            positions.last().copied()
        );
        self.subscriptions = Some(
            SubscriptionStore::open(ingestor.into_store(), tenant)
                .unwrap_or_else(|error| panic!("restore subscriptions: {error:?}")),
        );
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        self.subscriptions.take();
        fs::remove_dir_all(&self.root)
            .unwrap_or_else(|error| panic!("remove test state: {error:?}"));
    }
}

fn position(block_height: u64, tx_index: u32, log_index: u32) -> Cursor {
    Cursor {
        block_height,
        tx_index,
        log_index,
    }
}

fn attributes() -> EventAttributes {
    EventAttributes {
        agent: "agent-a".into(),
        account: "account-a".into(),
        activity_type: 1,
        module: "module-a".into(),
        asset: "asset-a".into(),
        counterparty: "counterparty-a".into(),
        result_code: 0,
    }
}

fn log(cursor: Cursor) -> Value {
    json!({
        "blockNumber": format!("0x{:x}", cursor.block_height),
        "transactionIndex": format!("0x{:x}", cursor.tx_index),
        "logIndex": format!("0x{:x}", cursor.log_index),
        "blockHash": format!("0x{}", "12".repeat(32)),
        "address": format!("0x{}", "34".repeat(20)),
        "topics": [], "data": "0x", "removed": false,
    })
}

#[test]
fn subscription_acknowledge_beyond_delivery_is_refused() {
    let mut fixture = Fixture::new();
    let target = fixture.target.clone();
    let delivered = position(12, 1, 4);
    fixture.ingest(&[delivered, position(13, 0, 0)]);
    fixture
        .subscriptions()
        .mark_delivered(&target, delivered)
        .unwrap_or_else(|error| panic!("deliver cursor: {error:?}"));
    let id = RequestId(7);
    for cursor in [position(12, 1, 5), position(12, 2, 0), position(13, 0, 0)] {
        let error = fixture
            .subscriptions()
            .acknowledge(&CursorAcknowledgement {
                scope: target.scope.clone(),
                subscription_id: target.subscription_id.clone(),
                cursor: cursor.into(),
            })
            .err()
            .unwrap_or_else(|| panic!("an ingested or future position was never delivered"));
        assert!(
            matches!(error, SubscriptionError::CursorNeverDelivered { cursor: refused } if refused == cursor)
        );
        let rejection = fixture.snapshot().acknowledgement_error(cursor, id);
        assert_eq!(rejection.reason, "subscription.cursor_ahead");
        assert_eq!(rejection.class, ErrorClass::PolicyRefusal);
        assert_eq!(rejection.retriability, Retriability::Terminal);
        assert_eq!(rejection.request_id, id);
        assert_eq!(
            fixture
                .subscriptions()
                .get(&target)
                .unwrap_or_else(|error| panic!("record after refusal: {error:?}"))
                .last_acknowledged,
            position(12, 0, 0).into()
        );
    }
    fixture
        .subscriptions()
        .acknowledge(&CursorAcknowledgement {
            scope: target.scope.clone(),
            subscription_id: target.subscription_id.clone(),
            cursor: delivered.into(),
        })
        .unwrap_or_else(|error| panic!("acknowledge the exact delivered triple: {error:?}"));
    assert_eq!(
        fixture
            .subscriptions()
            .resume_cursor(&target)
            .unwrap_or_else(|error| panic!("resume cursor: {error:?}")),
        delivered
    );
    assert_eq!(
        fixture
            .snapshot()
            .acknowledgement_error(position(12, 0, u32::MAX), id)
            .reason,
        "owner.refused"
    );
}

#[test]
fn subscription_health_reports_delivered_and_watermark_triples() {
    let mut fixture = Fixture::new();
    assert_eq!(
        fixture.snapshot().health_fields()["ingest_watermark"],
        Value::Null
    );
    let acknowledged = position(12, 1, 0);
    let delivered = position(12, 1, 4);
    let watermark = position(13, 0, 2);
    fixture.ingest(&[acknowledged, delivered, watermark]);
    let target = fixture.target.clone();
    fixture
        .subscriptions()
        .mark_delivered(&target, acknowledged)
        .unwrap_or_else(|error| panic!("first delivery: {error:?}"));
    fixture
        .subscriptions()
        .mark_delivered(&target, delivered)
        .unwrap_or_else(|error| panic!("second delivery: {error:?}"));
    fixture
        .subscriptions()
        .acknowledge(&CursorAcknowledgement {
            scope: target.scope.clone(),
            subscription_id: target.subscription_id.clone(),
            cursor: acknowledged.into(),
        })
        .unwrap_or_else(|error| panic!("acknowledge only the first delivery: {error:?}"));
    let expected = json!({
        "last_delivered": {"block_height":"12", "tx_index":"1", "log_index":"4"},
        "ingest_watermark": {"block_height":"13", "tx_index":"0", "log_index":"2"},
    });
    assert_eq!(Value::Object(fixture.snapshot().health_fields()), expected);
    let subscriptions = fixture
        .subscriptions
        .take()
        .unwrap_or_else(|| panic!("subscriptions"));
    drop(subscriptions);
    fixture.subscriptions = Some(
        SubscriptionStore::open(
            Store::open(&fixture.root)
                .unwrap_or_else(|error| panic!("reopen durable store: {error:?}")),
            TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("restore subscriptions: {error:?}")),
    );
    assert_eq!(Value::Object(fixture.snapshot().health_fields()), expected);
    let durable = fixture
        .subscriptions
        .as_ref()
        .unwrap_or_else(|| panic!("subscriptions"))
        .durable()
        .unwrap_or_else(|error| panic!("durable view: {error:?}"));
    let mut foreign_target = target.clone();
    foreign_target.scope.capability = CapabilityId::new("another-capability")
        .unwrap_or_else(|error| panic!("capability: {error:?}"));
    assert_eq!(
        SubscriptionCursorSnapshot::read(&durable, &foreign_target, None, RequestId(7))
            .err()
            .unwrap_or_else(|| panic!("foreign scope refused"))
            .reason,
        "owner.refused"
    );
    let credential = SessionCredential::new(
        TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error:?}")),
        SessionId([1; 32]),
        [2; 32],
        1,
    );
    assert_eq!(
        SubscriptionCursorSnapshot::read(&durable, &target, Some(&credential), RequestId(7))
            .err()
            .unwrap_or_else(|| panic!("wrong session binding refused"))
            .reason,
        "owner.refused"
    );
    let key = TenantKey::new(
        TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error:?}")),
        ObjectKind::Subscription,
        target.subscription_id.as_str().as_bytes().to_vec(),
    )
    .unwrap_or_else(|error| panic!("record key: {error:?}"));
    assert!(durable
        .get(&key)
        .unwrap_or_else(|| panic!("durable cursor record"))
        .bytes()
        .starts_with(b"LXS3"));
}

#[test]
fn local_event_anchor_does_not_advance_the_chain_watermark() {
    use layerx_agentd::events::{ingest_local_restriction, CoreEvent};
    use layerx_types::verify::VerificationLevel;
    let mut fixture = Fixture::new();
    fixture.ingest(&[position(13, 0, 2)]);
    let before = fixture.snapshot().health_fields()["ingest_watermark"].clone();
    let subscriptions = fixture
        .subscriptions
        .take()
        .unwrap_or_else(|| panic!("test value missing"));
    let mut ingestor = EventIngestor::open(
        subscriptions
            .into_durable()
            .unwrap_or_else(|error| panic!("test operation failed: {error:?}")),
        TenantId::new("tenant-a")
            .unwrap_or_else(|error| panic!("test operation failed: {error:?}")),
        8,
        0,
    )
    .unwrap_or_else(|error| panic!("test operation failed: {error:?}"));
    let sequence = ingestor.watermark().next_expected;
    ingest_local_restriction(
        &mut ingestor,
        CoreEvent {
            global_sequence: sequence,
            canonical_bytes: b"local-policy-restriction".to_vec(),
            receipt_reference: None,
            receipt_verification_level: VerificationLevel::UNVERIFIED,
            attributes: attributes(),
        },
    )
    .unwrap_or_else(|error| panic!("test operation failed: {error:?}"));
    assert_eq!(
        ingestor
            .watermark()
            .last_position
            .unwrap_or_else(|| panic!("test value missing"))
            .block_height,
        13
    );
    assert_eq!(ingestor.watermark().next_expected, sequence + 1);
    drop(ingestor);
    fixture.subscriptions = Some(
        SubscriptionStore::open(
            Store::open(&fixture.root)
                .unwrap_or_else(|error| panic!("test operation failed: {error:?}")),
            TenantId::new("tenant-a")
                .unwrap_or_else(|error| panic!("test operation failed: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("test operation failed: {error:?}")),
    );
    assert_eq!(
        fixture.snapshot().health_fields()["ingest_watermark"],
        before
    );
}
