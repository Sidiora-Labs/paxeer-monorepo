fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use std::collections::BTreeSet;
use std::fs;
use std::os::unix::fs::MetadataExt;
use std::os::unix::net::UnixListener;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::thread;
use std::time::Duration;

use layerx_agent_api::identity::{
    ActivityType, AgentDid, Asset, CapabilityId, Counterparty, ExplicitSet, TenantId as ApiTenantId,
};
use layerx_agent_api::read::{AccountRef, ModuleRef};
use layerx_agent_api::subscription::{
    Cursor, CursorAcknowledgement, DeliveryTarget, SubscriptionCreate, SubscriptionFilter,
    SubscriptionId, SubscriptionScope, SubscriptionTarget, TenantObject,
};
use layerx_agentd::events::gap::EthLogSource;
use layerx_agentd::events::outbound::{
    self, Authenticator, Endpoint, PeerIdentity, StopSignal, VerifiedItem,
};
use layerx_agentd::events::subscription::Store as SubscriptionStore;
use layerx_agentd::events::{
    deliver, health, DeliveryEngine, DeliveryItem, DeliveryPhase, EventAttributes, EventIngestor,
    RetryPolicy,
};
use layerx_agentd::read::HistoryLimits;
use layerx_agentd::store::{Store, TenantId};
use layerx_client::frame::framing::{read_frame, write_frame};
use layerx_client::subscription::SubscriptionTopic;
use layerx_types::result::ResultCode;
use serde_json::{json, Value};

static NEXT: AtomicU64 = AtomicU64::new(0);

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        Self(std::env::temp_dir().join(format!(
            "lx-22-31-215-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        )))
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn tenant() -> TenantId {
    require_ok(TenantId::new("tenant-a"), "required test value")
}

fn scope() -> SubscriptionScope {
    SubscriptionScope {
        tenant: require_ok(ApiTenantId::new("tenant-a"), "required test value"),
        agent: require_ok(AgentDid::new("agent-a"), "required test value"),
        capability: require_ok(CapabilityId::new("capability-a"), "required test value"),
    }
}

fn target() -> SubscriptionTarget {
    SubscriptionTarget {
        scope: scope(),
        subscription_id: require_ok(SubscriptionId::new("subscription-a"), "required test value"),
    }
}

fn attributes() -> EventAttributes {
    EventAttributes {
        agent: "agent-a".into(),
        account: "account-a".into(),
        activity_type: 9,
        module: "asset".into(),
        asset: "LXR".into(),
        counterparty: "counterparty-a".into(),
        result_code: 0,
    }
}

fn cursor(height: u64, tx: u32, log: u32) -> Cursor {
    Cursor {
        block_height: height,
        tx_index: tx,
        log_index: log,
    }
}

fn node_log(height: u64, tx: u32, index: u32) -> Value {
    json!({
        "blockNumber": format!("0x{height:x}"),
        "transactionIndex": format!("0x{tx:x}"),
        "logIndex": format!("0x{index:x}"),
        "transactionHash": format!("0x{}", hex::encode([require_ok(u8::try_from(tx), "test value fits u8") + 1; 32])),
        "blockHash": format!("0x{}", hex::encode([require_ok(u8::try_from(height), "test value fits u8"); 32])),
        "address": format!("0x{}", "12".repeat(20)),
        "topics": [format!("0x{}", "34".repeat(32))],
        "data": "0x00ff", "removed": false
    })
}

fn open_ingestor(root: &Path, initial_sequence: u64) -> EventIngestor {
    require_ok(
        EventIngestor::open(
            require_ok(Store::open(root), "required test value"),
            tenant(),
            32,
            initial_sequence,
        ),
        "required test value",
    )
}

fn close_block(ingestor: &mut EventIngestor, head: u64) {
    // Adjacent heads flush the actual pending-log journal without a history gap.
    let source = require_ok(
        EthLogSource::new(
            "http://127.0.0.1:9",
            &[],
            SubscriptionTopic::Logs {
                address: None,
                topics: vec![],
            },
        ),
        "required test value",
    );
    require_ok(
        ingestor.new_head(
            head,
            &source,
            HistoryLimits {
                maximum_items: 32,
                maximum_response_bytes: 1_048_576,
            },
            32,
            attributes(),
        ),
        "required test value",
    );
}

fn create(ingestor: EventIngestor, start: Cursor) -> SubscriptionStore {
    let mut store = require_ok(
        SubscriptionStore::open(ingestor.into_store(), tenant()),
        "required test value",
    );
    let owned = |value| TenantObject {
        tenant: scope().tenant,
        value,
    };
    require_ok(
        store.create(
            target().subscription_id,
            SubscriptionCreate {
                scope: scope(),
                filter: SubscriptionFilter {
                    agents: ExplicitSet::allow(vec![owned(require_ok(
                        AgentDid::new("agent-a"),
                        "required test value",
                    ))]),
                    accounts: ExplicitSet::allow(vec![TenantObject {
                        tenant: scope().tenant,
                        value: require_ok(AccountRef::new("account-a"), "required test value"),
                    }]),
                    activity_types: ExplicitSet::allow(vec![ActivityType(9)]),
                    modules: ExplicitSet::allow(vec![TenantObject {
                        tenant: scope().tenant,
                        value: require_ok(ModuleRef::new("asset"), "required test value"),
                    }]),
                    assets: ExplicitSet::allow(vec![TenantObject {
                        tenant: scope().tenant,
                        value: require_ok(Asset::new("LXR"), "required test value"),
                    }]),
                    counterparties: ExplicitSet::allow(vec![TenantObject {
                        tenant: scope().tenant,
                        value: require_ok(
                            Counterparty::new("counterparty-a"),
                            "required test value",
                        ),
                    }]),
                    result_classes: ExplicitSet::allow(vec![ResultCode::from_raw(0)]),
                },
                start,
                delivery_target: require_ok(
                    DeliveryTarget::new("uds://consumer"),
                    "required test value",
                ),
            },
        ),
        "required test value",
    );
    store
}

fn open_engine(store: SubscriptionStore, live_start: u64) -> DeliveryEngine {
    require_ok(
        DeliveryEngine::open(
            store,
            target(),
            live_start,
            3,
            RetryPolicy {
                base_delay_ms: 10,
                maximum_delay_ms: 100,
                jitter_percent: 0,
                maximum_attempts: 3,
            },
        ),
        "required test value",
    )
}

fn reopen(root: &Path, live_start: u64) -> DeliveryEngine {
    open_engine(
        require_ok(
            SubscriptionStore::open(
                require_ok(Store::open(root), "required test value"),
                tenant(),
            ),
            "required test value",
        ),
        live_start,
    )
}

struct Consumer {
    listener: UnixListener,
    endpoint: Endpoint,
    effects: BTreeSet<[u8; 32]>,
    attempts: usize,
}

impl Consumer {
    fn new(directory: &Path) -> Self {
        require_ok(fs::create_dir_all(directory), "required test value");
        let socket = directory.join("consumer.sock");
        let listener = require_ok(UnixListener::bind(&socket), "required test value");
        let metadata = require_ok(fs::metadata(&socket), "required test value");
        let endpoint = require_ok(
            Endpoint::new(
                socket,
                PeerIdentity {
                    uid: metadata.uid(),
                    gid: metadata.gid(),
                },
                16_384,
                Duration::from_secs(3),
                Duration::from_millis(2),
            ),
            "required test value",
        );
        Self {
            listener,
            endpoint,
            effects: BTreeSet::new(),
            attempts: 0,
        }
    }

    fn receive(&mut self, engine: &mut DeliveryEngine) -> (DeliveryItem, VerifiedItem) {
        let listener = require_ok(self.listener.try_clone(), "required test value");
        let peer = self.endpoint.expected_peer;
        let server = thread::spawn(move || {
            let (mut stream, _) = require_ok(listener.accept(), "required test value");
            require_ok(
                stream.set_read_timeout(Some(Duration::from_secs(3))),
                "required test value",
            );
            require_ok(
                stream.set_write_timeout(Some(Duration::from_secs(3))),
                "required test value",
            );
            let frame = require_ok(read_frame(&mut stream, 16_384), "required test value");
            let auth = authenticator();
            let verified = require_ok(auth.verify(&frame, peer), "required test value");
            assert_eq!(verified.tenant, "tenant-a");
            assert_eq!(verified.subscription_id, "subscription-a");
            require_ok(
                write_frame(&mut stream, &auth.acknowledgement(verified.binding), 16_384),
                "required test value",
            );
            verified.item
        });
        let sent = require_ok(
            outbound::deliver(
                engine,
                &self.endpoint,
                &authenticator(),
                &StopSignal::active(),
                100,
            ),
            "required test value",
        );
        let received = require_ok(server.join(), "required test value");
        if let VerifiedItem::Event {
            deduplication_id,
            event_identity,
            ..
        } = &received
        {
            assert_eq!(deduplication_id, event_identity);
            self.attempts += 1;
            self.effects.insert(*deduplication_id);
        }
        (sent.item, received)
    }
}

fn authenticator() -> Authenticator {
    require_ok(
        Authenticator::new("consumer-key", [0x8a; 32]),
        "required test value",
    )
}

fn acknowledge(engine: &mut DeliveryEngine, position: Cursor) {
    let record = require_ok(
        engine.acknowledge(&CursorAcknowledgement {
            scope: scope(),
            subscription_id: target().subscription_id,
            cursor: position,
        }),
        "required test value",
    );
    assert_eq!(record.last_acknowledged, position);
}

fn assert_event(
    item: &(DeliveryItem, VerifiedItem),
    position: Cursor,
    ordinal: u64,
    phase: DeliveryPhase,
    bytes: &Value,
) {
    let DeliveryItem::Event(event) = &item.0 else {
        panic!("expected delivered event")
    };
    assert_eq!(event.global_sequence, ordinal);
    assert_eq!(event.phase, phase);
    assert_eq!(event.delivery.cursor, position);
    let VerifiedItem::Event {
        cursor,
        global_sequence,
        phase: wire_phase,
        event_bytes,
        ..
    } = &item.1
    else {
        panic!("expected wire event")
    };
    assert_eq!(*cursor, position);
    assert_eq!(*global_sequence, ordinal);
    assert_eq!(*wire_phase, u8::from(phase != DeliveryPhase::Backfill));
    assert_eq!(
        require_ok(
            serde_json::from_slice::<Value>(event_bytes),
            "required test value"
        ),
        *bytes
    );
}

#[test]
fn event_cursor_preserves_node_positions_across_ack_reopen_and_live_backfill_seam() {
    let root = Directory::new();
    let mut ingestor = open_ingestor(&root.0, 70);
    let logs = [
        node_log(42, 0, 0),
        node_log(42, 0, 1),
        node_log(42, 1, 2),
        node_log(42, 1, 3),
    ];
    // Ingestion sorts the node's real position independently of arrival order.
    for index in [3, 1, 2, 0] {
        require_ok(
            ingestor.ingest_log(&logs[index], attributes()),
            "required test value",
        );
    }
    close_block(&mut ingestor, 43);
    assert_eq!(ingestor.watermark().next_expected, 74);
    let mut engine = open_engine(create(ingestor, cursor(41, 0, 0)), 72);
    let mut consumer = Consumer::new(&root.0);
    let first = consumer.receive(&mut engine);
    assert_event(
        &first,
        cursor(42, 0, 0),
        70,
        DeliveryPhase::Backfill,
        &logs[0],
    );
    acknowledge(&mut engine, cursor(42, 0, 0));
    drop(engine);

    let mut engine = reopen(&root.0, 72);
    let second = consumer.receive(&mut engine);
    assert_event(
        &second,
        cursor(42, 0, 1),
        71,
        DeliveryPhase::Backfill,
        &logs[1],
    );
    acknowledge(&mut engine, cursor(42, 0, 1));
    let seam = consumer.receive(&mut engine);
    assert!(matches!(seam.0, DeliveryItem::BackfillComplete(transition)
        if transition.live_starts_at == 72 && transition.resume_cursor == cursor(42, 0, 1)));
    assert!(
        matches!(seam.1, VerifiedItem::BackfillComplete { live_starts_at: 72, resume_cursor }
        if resume_cursor == cursor(42, 0, 1))
    );
    assert!(engine.seam_confirmed());
    for (index, position) in [(2, cursor(42, 1, 2)), (3, cursor(42, 1, 3))] {
        let item = consumer.receive(&mut engine);
        assert_event(
            &item,
            position,
            70 + index as u64,
            DeliveryPhase::Live,
            &logs[index],
        );
        acknowledge(&mut engine, position);
    }
    assert!(require_ok(deliver(&mut engine), "required test value").is_none());
    assert_eq!(consumer.effects.len(), 4);
    assert_eq!(
        require_ok(health(&engine), "required test value").lag_sequences,
        0
    );
    drop(engine);

    // An acknowledged node cursor also selects an event appended after reopening.
    let mut ingestor = open_ingestor(&root.0, 0);
    let live = node_log(43, 0, 0);
    require_ok(
        ingestor.ingest_log(&live, attributes()),
        "required test value",
    );
    close_block(&mut ingestor, 44);
    drop(ingestor);
    let mut engine = reopen(&root.0, 72);
    let resumed_seam = consumer.receive(&mut engine);
    assert!(
        matches!(resumed_seam.1, VerifiedItem::BackfillComplete { resume_cursor, .. }
        if resume_cursor == cursor(42, 1, 3))
    );
    let item = consumer.receive(&mut engine);
    assert_event(&item, cursor(43, 0, 0), 74, DeliveryPhase::Live, &live);
    acknowledge(&mut engine, cursor(43, 0, 0));
    assert_eq!(consumer.effects.len(), 5);
    assert_eq!(consumer.attempts, 5);
    assert!(require_ok(deliver(&mut engine), "required test value").is_none());
}

#[test]
fn event_replay_identity_survives_receipt_promotion_reopen_and_one_consumer_effect() {
    let root = Directory::new();
    let mut ingestor = open_ingestor(&root.0, 900);
    let mut pending = node_log(42, 0, 0);
    pending["blockHash"] = Value::Null;
    pending["seq"] = json!("0x63");
    require_ok(
        ingestor.ingest_log(&pending, attributes()),
        "required test value",
    );
    close_block(&mut ingestor, 43);
    let mut engine = open_engine(create(ingestor, cursor(41, 0, 0)), 901);
    let mut consumer = Consumer::new(&root.0);
    let first = consumer.receive(&mut engine);
    assert_event(
        &first,
        cursor(42, 0, 0),
        900,
        DeliveryPhase::Backfill,
        &pending,
    );
    let DeliveryItem::Event(first_event) = first.0 else {
        panic!("expected event")
    };
    // The consumer applied its effect and accepted transport, but its durable
    // cursor acknowledgement is lost before the daemon stops.
    assert_eq!(consumer.effects.len(), 1);
    assert_eq!(
        require_ok(health(&engine), "required test value")
            .acknowledged_cursor
            .block_height,
        41
    );
    drop(engine);

    let mut ingestor = open_ingestor(&root.0, 0);
    let mut promoted = pending.clone();
    promoted["blockHash"] = node_log(42, 0, 0)["blockHash"].clone();
    promoted["transactionIndex"] = json!("0x2");
    require_ok(
        ingestor.ingest_log(&promoted, attributes()),
        "required test value",
    );
    assert_eq!(ingestor.watermark().next_expected, 901);
    assert_eq!(
        require_ok(
            serde_json::from_slice::<Value>(
                &require_ok(ingestor.retained_event(900), "required test value").canonical_bytes
            ),
            "required test value"
        ),
        promoted
    );
    drop(ingestor);

    let mut engine = reopen(&root.0, 901);
    let replay = consumer.receive(&mut engine);
    assert_event(
        &replay,
        cursor(42, 2, 0),
        900,
        DeliveryPhase::Backfill,
        &promoted,
    );
    let DeliveryItem::Event(replayed_event) = replay.0 else {
        panic!("expected event")
    };
    assert_ne!(
        first_event.delivery.event_bytes,
        replayed_event.delivery.event_bytes
    );
    assert_eq!(
        first_event.delivery.event_identity,
        replayed_event.delivery.event_identity
    );
    assert_eq!(
        first_event.delivery.deduplication_id,
        replayed_event.delivery.deduplication_id
    );
    assert_eq!(consumer.attempts, 2);
    assert_eq!(
        consumer.effects.len(),
        1,
        "replay must produce one consumer effect"
    );
    acknowledge(&mut engine, replayed_event.delivery.cursor);
    drop(engine);
    let mut engine = reopen(&root.0, 901);
    assert!(matches!(
        consumer.receive(&mut engine).1,
        VerifiedItem::BackfillComplete { .. }
    ));
    assert!(require_ok(deliver(&mut engine), "required test value").is_none());
    assert_eq!(
        require_ok(health(&engine), "required test value").lag_sequences,
        0
    );
}

#[test]
fn event_replay_identity_without_transaction_hash_uses_fixed_block_position() {
    let root = Directory::new();
    let mut ingestor = open_ingestor(&root.0, 300);
    let mut first = node_log(42, 0, 0);
    require_some(first.as_object_mut(), "required test value").remove("transactionHash");
    let mut second = first.clone();
    second["logIndex"] = json!("0x1");
    require_ok(
        ingestor.ingest_log(&first, attributes()),
        "required test value",
    );
    require_ok(
        ingestor.ingest_log(&second, attributes()),
        "required test value",
    );
    close_block(&mut ingestor, 43);
    let mut engine = open_engine(create(ingestor, cursor(41, 0, 0)), 302);
    let mut consumer = Consumer::new(&root.0);
    let original = consumer.receive(&mut engine);
    let DeliveryItem::Event(original) = original.0 else {
        panic!("expected event")
    };
    drop(engine);
    let mut engine = reopen(&root.0, 302);
    let replay = consumer.receive(&mut engine);
    let DeliveryItem::Event(replay) = replay.0 else {
        panic!("expected event")
    };
    assert_eq!(
        original.delivery.deduplication_id,
        replay.delivery.deduplication_id
    );
    acknowledge(&mut engine, replay.delivery.cursor);
    let next = consumer.receive(&mut engine);
    let DeliveryItem::Event(next) = next.0 else {
        panic!("expected event")
    };
    assert_ne!(
        next.delivery.deduplication_id,
        original.delivery.deduplication_id
    );
    acknowledge(&mut engine, next.delivery.cursor);
    assert_eq!(consumer.attempts, 3);
    assert_eq!(consumer.effects.len(), 2);
}
