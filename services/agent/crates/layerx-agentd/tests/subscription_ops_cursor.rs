use std::fs;
use std::os::unix::net::UnixListener;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Duration;

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
use layerx_client::evidence::RootSelector;
use layerx_client::head::Head;
use layerx_client::lni::schema::Version;
use layerx_client::lni::transport::{ConnectionGate, Limits, Uds};
use layerx_client::read::{ReadContext, Requested};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_types::verify::VerificationLevel;
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
        let tenant = TenantId::new("tenant-a").expect("tenant");
        let scope = SubscriptionScope {
            tenant: ApiTenantId::new("tenant-a").expect("API tenant"),
            agent: AgentDid::new("agent-a").expect("agent"),
            capability: CapabilityId::new("capability-a").expect("capability"),
        };
        let id = SubscriptionId::new("subscription-a").expect("subscription ID");
        let mut subscriptions =
            SubscriptionStore::open(Store::open(&root).expect("durable store"), tenant)
                .expect("subscription store");
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
                        .expect("delivery target"),
                },
            )
            .expect("create subscription with a block-position cursor");
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
            .expect("open subscription store")
    }

    fn snapshot(&self) -> SubscriptionCursorSnapshot {
        let durable = self
            .subscriptions
            .as_ref()
            .expect("subscriptions")
            .durable()
            .expect("durable view");
        SubscriptionCursorSnapshot::read(&durable, &self.target, None, RequestId(7))
            .expect("read cursor state through the RPC adapter")
    }

    fn ingest(&mut self, positions: &[Cursor]) {
        let subscriptions = self.subscriptions.take().expect("subscriptions");
        let durable = subscriptions.into_durable().expect("owned durable store");
        let tenant = TenantId::new("tenant-a").expect("tenant");
        let mut ingestor = EventIngestor::open(durable, tenant.clone(), 8, 0).expect("ingestor");
        for cursor in positions {
            ingestor
                .ingest_log(&log(*cursor), attributes())
                .expect("ingest Ethereum log");
        }
        let socket = self.root.join("history.sock");
        let listener = UnixListener::bind(&socket).expect("history listener");
        let mut transport = Uds::connect(
            &socket,
            &ConnectionGate::new(1),
            Limits {
                maximum_frame_bytes: 1_048_576,
                maximum_connections: 1,
                maximum_streams: 1,
                maximum_queued_bytes: 1_048_576,
                deadline: Duration::from_secs(2),
            },
        )
        .expect("real history transport");
        let (_peer, _) = listener.accept().expect("history connection");
        ingestor
            .new_head(
                14,
                &mut transport,
                HistoryLimits {
                    maximum_items: 8,
                    maximum_response_bytes: 1_048_576,
                },
                ReadContext {
                    interface_version: Version::V1_0,
                    correlation_id: 7,
                    expected_protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
                    expected_network_id: 77,
                    requested: Requested::new(VerificationLevel::UNVERIFIED),
                    head: Head {
                        chain_sequence: 14,
                        sealed_batch: 0,
                        finalised_checkpoint: [0; 32],
                    },
                    sequencer_authorization: SequencerAuthorization::new(
                        [1; 32],
                        [2; 32],
                        0,
                        u64::MAX,
                    ),
                    handshake_sequencer_key: [2; 32],
                    root_selector: RootSelector::Latest,
                },
                8,
                attributes(),
            )
            .expect("flush logs from closed blocks");
        assert_eq!(
            ingestor.watermark().last_position.map(|cursor| position(
                cursor.block_height,
                cursor.tx_index,
                cursor.log_index
            )),
            positions.last().copied()
        );
        self.subscriptions = Some(
            SubscriptionStore::open(ingestor.into_store(), tenant).expect("restore subscriptions"),
        );
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        self.subscriptions.take();
        fs::remove_dir_all(&self.root).expect("remove test state");
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
        .expect("deliver cursor");
    let id = RequestId(7);
    for cursor in [position(12, 1, 5), position(12, 2, 0), position(13, 0, 0)] {
        let error = fixture
            .subscriptions()
            .acknowledge(&CursorAcknowledgement {
                scope: target.scope.clone(),
                subscription_id: target.subscription_id.clone(),
                cursor: cursor.into(),
            })
            .expect_err("an ingested or future position was never delivered");
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
                .expect("record after refusal")
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
        .expect("acknowledge the exact delivered triple");
    assert_eq!(
        fixture
            .subscriptions()
            .resume_cursor(&target)
            .expect("resume cursor"),
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
        .expect("first delivery");
    fixture
        .subscriptions()
        .mark_delivered(&target, delivered)
        .expect("second delivery");
    fixture
        .subscriptions()
        .acknowledge(&CursorAcknowledgement {
            scope: target.scope.clone(),
            subscription_id: target.subscription_id.clone(),
            cursor: acknowledged.into(),
        })
        .expect("acknowledge only the first delivery");
    let expected = json!({
        "last_delivered": {"block_height":"12", "tx_index":"1", "log_index":"4"},
        "ingest_watermark": {"block_height":"13", "tx_index":"0", "log_index":"2"},
    });
    assert_eq!(Value::Object(fixture.snapshot().health_fields()), expected);
    let subscriptions = fixture.subscriptions.take().expect("subscriptions");
    drop(subscriptions);
    fixture.subscriptions = Some(
        SubscriptionStore::open(
            Store::open(&fixture.root).expect("reopen durable store"),
            TenantId::new("tenant-a").expect("tenant"),
        )
        .expect("restore subscriptions"),
    );
    assert_eq!(Value::Object(fixture.snapshot().health_fields()), expected);
    let durable = fixture
        .subscriptions
        .as_ref()
        .expect("subscriptions")
        .durable()
        .expect("durable view");
    let mut foreign_target = target.clone();
    foreign_target.scope.capability = CapabilityId::new("another-capability").expect("capability");
    assert_eq!(
        SubscriptionCursorSnapshot::read(&durable, &foreign_target, None, RequestId(7))
            .err()
            .expect("foreign scope refused")
            .reason,
        "owner.refused"
    );
    let credential = SessionCredential::new(
        TenantId::new("tenant-a").expect("tenant"),
        SessionId([1; 32]),
        [2; 32],
        1,
    );
    assert_eq!(
        SubscriptionCursorSnapshot::read(&durable, &target, Some(&credential), RequestId(7))
            .err()
            .expect("wrong session binding refused")
            .reason,
        "owner.refused"
    );
    let key = TenantKey::new(
        TenantId::new("tenant-a").expect("tenant"),
        ObjectKind::Subscription,
        target.subscription_id.as_str().as_bytes().to_vec(),
    )
    .expect("record key");
    assert!(durable
        .get(&key)
        .expect("durable cursor record")
        .bytes()
        .starts_with(b"LXS3"));
}
