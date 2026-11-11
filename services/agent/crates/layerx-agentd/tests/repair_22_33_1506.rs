fn require_err<T: std::fmt::Debug, E>(result: Result<T, E>, message: &str) -> E {
    result
        .err()
        .unwrap_or_else(|| panic!("{message}: expected an error"))
}

fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::os::unix::fs::{MetadataExt, PermissionsExt};
use std::os::unix::net::UnixListener;
use std::os::unix::process::CommandExt;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{mpsc, Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant};

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
    self, Authenticator, Endpoint, EndpointFailure, OutboundError, PeerIdentity, StopSignal,
    VerifiedItem,
};
use layerx_agentd::events::subscription::{
    Store as SubscriptionStore, SubscriptionError, Termination,
};
use layerx_agentd::events::{
    backfill, deliver, DeliveryEngine, DeliveryError, DeliveryItem, EventAttributes, EventIngestor,
    RetryPolicy,
};
use layerx_agentd::read::HistoryLimits;
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::frame::framing::{read_frame, write_frame};
use layerx_client::subscription::SubscriptionTopic;
use layerx_types::result::ResultCode;
use serde_json::{json, Value};

static NEXT: AtomicU64 = AtomicU64::new(0);

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        Self(std::env::temp_dir().join(format!(
            "lx-22-33-1506-{}-{}",
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
            16,
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

fn authenticator() -> Authenticator {
    require_ok(
        Authenticator::new("consumer-key", [0x8a; 32]),
        "required test value",
    )
}

fn fixture(root: &Path) -> (Arc<Mutex<Store>>, SubscriptionStore, DeliveryEngine) {
    let mut ingestor = open_ingestor(root, 70);
    for log in 0..4 {
        require_ok(
            ingestor.ingest_log(&node_log(42, 0, log), attributes()),
            "required test value",
        );
    }
    close_block(&mut ingestor, 43);
    let durable = require_ok(
        create(ingestor, cursor(41, 0, 0)).into_durable(),
        "required test value",
    );
    let shared = Arc::new(Mutex::new(durable));
    let controller = require_ok(
        SubscriptionStore::open_shared(shared.clone(), tenant()),
        "required test value",
    );
    let engine = open_engine(
        require_ok(
            SubscriptionStore::open_shared(shared.clone(), tenant()),
            "required test value",
        ),
        74,
    );
    (shared, controller, engine)
}

fn endpoint(path: PathBuf, deadline: Duration) -> Endpoint {
    let metadata = require_ok(fs::metadata("/proc/self"), "required test value");
    require_ok(
        Endpoint::new(
            path,
            PeerIdentity {
                uid: metadata.uid(),
                gid: metadata.gid(),
            },
            16384,
            deadline,
            Duration::from_millis(2),
        ),
        "required test value",
    )
}

fn listener(root: &Path) -> (UnixListener, Endpoint) {
    let path = root.join("consumer.sock");
    let listener = require_ok(UnixListener::bind(&path), "required test value");
    (listener, endpoint(path, Duration::from_secs(2)))
}

fn accept_bounded(listener: &UnixListener) -> std::os::unix::net::UnixStream {
    require_ok(listener.set_nonblocking(true), "required test value");
    let started = Instant::now();
    loop {
        match listener.accept() {
            Ok((stream, _)) => {
                require_ok(listener.set_nonblocking(false), "required test value");
                require_ok(stream.set_nonblocking(false), "required test value");
                return stream;
            }
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                assert!(
                    started.elapsed() < Duration::from_secs(2),
                    "receiver did not get a connection"
                );
                thread::sleep(Duration::from_millis(2));
            }
            Err(error) => panic!("receiver accept: {error}"),
        }
    }
}

fn receive_one(
    listener: &UnixListener,
    endpoint: &Endpoint,
    engine: &mut DeliveryEngine,
) -> VerifiedItem {
    let listener = require_ok(listener.try_clone(), "required test value");
    let peer = endpoint.expected_peer;
    let receiver = thread::spawn(move || {
        let mut stream = accept_bounded(&listener);
        require_ok(
            stream.set_read_timeout(Some(Duration::from_secs(2))),
            "required test value",
        );
        let frame = require_ok(read_frame(&mut stream, 16384), "required test value");
        let auth = authenticator();
        let verified = require_ok(auth.verify(&frame, peer), "required test value");
        assert_eq!(verified.subscription_id, "subscription-a");
        assert_eq!(verified.tenant, "tenant-a");
        require_ok(
            write_frame(&mut stream, &auth.acknowledgement(verified.binding), 16384),
            "required test value",
        );
        verified.item
    });
    let receipt = require_ok(
        outbound::deliver(
            engine,
            endpoint,
            &authenticator(),
            &StopSignal::active(),
            100,
        ),
        "required test value",
    );
    assert_eq!(receipt.peer, endpoint.expected_peer);
    require_ok(receiver.join(), "required test value")
}

fn assert_event(item: VerifiedItem, log: u32) {
    let VerifiedItem::Event {
        cursor: position,
        global_sequence,
        event_bytes,
        deduplication_id,
        event_identity,
        ..
    } = item
    else {
        panic!("expected real event frame");
    };
    assert_eq!(position, cursor(42, 0, log));
    assert_eq!(global_sequence, 70 + u64::from(log));
    assert_eq!(deduplication_id, event_identity);
    assert_eq!(
        require_ok(
            serde_json::from_slice::<Value>(&event_bytes),
            "required test value"
        ),
        node_log(42, 0, log)
    );
}

fn acknowledge(engine: &mut DeliveryEngine, log: u32) {
    let record = require_ok(
        engine.acknowledge(&CursorAcknowledgement {
            scope: scope(),
            subscription_id: target().subscription_id,
            cursor: cursor(42, 0, log),
        }),
        "required test value",
    );
    assert_eq!(record.last_acknowledged, cursor(42, 0, log));
}

fn assert_no_connection(listener: &UnixListener) {
    require_ok(listener.set_nonblocking(true), "required test value");
    assert_eq!(
        require_err(listener.accept(), "required test value").kind(),
        std::io::ErrorKind::WouldBlock
    );
    require_ok(listener.set_nonblocking(false), "required test value");
}

fn paused(error: &OutboundError) {
    assert!(
        matches!(
            error,
            OutboundError::Delivery(DeliveryError::Subscription(SubscriptionError::Paused))
        ),
        "{error:?}"
    );
}

#[test]
fn paused_delivery_after_buffering_has_no_effect_and_resumes_once() {
    let root = Directory::new();
    let (shared_store, mut controller, mut engine) = fixture(&root.0);
    let (listener, endpoint) = listener(&root.0);
    require_ok(backfill(&mut engine), "required test value");
    assert_eq!(engine.buffered_len(), 5);
    require_ok(controller.pause(&target()), "required test value");
    assert!(matches!(
        deliver(&mut engine),
        Err(DeliveryError::Subscription(SubscriptionError::Paused))
    ));
    paused(&require_err(
        outbound::deliver(
            &mut engine,
            &endpoint,
            &authenticator(),
            &StopSignal::active(),
            0,
        ),
        "required test value",
    ));
    assert_no_connection(&listener);
    assert_eq!(engine.buffered_len(), 5);
    require_ok(controller.resume(&target()), "required test value");
    assert_event(receive_one(&listener, &endpoint, &mut engine), 0);
    acknowledge(&mut engine, 0);
    assert_eq!(engine.buffered_len(), 4);
    assert_no_connection(&listener);
    drop(engine);
    drop(controller);
    drop(shared_store);
    let mut engine = reopen(&root.0, 74);
    assert_event(receive_one(&listener, &endpoint, &mut engine), 1);
}

#[test]
fn pause_waits_for_inflight_acknowledgement_and_retains_its_delivery() {
    let root = Directory::new();
    let (_shared_store, mut controller, engine) = fixture(&root.0);
    let (listener, endpoint) = listener(&root.0);
    let (routed_tx, routed_rx) = mpsc::channel();
    let (release_tx, release_rx) = mpsc::channel();
    let peer = endpoint.expected_peer;
    let receiver = thread::spawn(move || {
        let mut stream = accept_bounded(&listener);
        require_ok(
            stream.set_read_timeout(Some(Duration::from_secs(2))),
            "required test value",
        );
        let auth = authenticator();
        let frame = require_ok(read_frame(&mut stream, 16384), "required test value");
        let verified = require_ok(auth.verify(&frame, peer), "required test value");
        assert_event(verified.item, 0);
        require_ok(routed_tx.send(()), "required test value");
        require_ok(
            release_rx.recv_timeout(Duration::from_secs(2)),
            "required test value",
        );
        require_ok(
            write_frame(&mut stream, &auth.acknowledgement(verified.binding), 16384),
            "required test value",
        );
    });
    let sender_endpoint = endpoint.clone();
    let sender = thread::spawn(move || {
        let mut engine = engine;
        require_ok(
            outbound::deliver(
                &mut engine,
                &sender_endpoint,
                &authenticator(),
                &StopSignal::active(),
                0,
            ),
            "required test value",
        );
        engine
    });
    require_ok(
        routed_rx.recv_timeout(Duration::from_secs(2)),
        "required test value",
    );
    let (paused_tx, paused_rx) = mpsc::channel();
    let (started_tx, started_rx) = mpsc::channel();
    let pauser = thread::spawn(move || {
        require_ok(started_tx.send(()), "required test value");
        let record = require_ok(controller.pause(&target()), "required test value");
        assert!(record.paused);
        require_ok(paused_tx.send(()), "required test value");
        controller
    });
    require_ok(
        started_rx.recv_timeout(Duration::from_secs(2)),
        "required test value",
    );
    assert!(matches!(
        paused_rx.recv_timeout(Duration::from_millis(30)),
        Err(mpsc::RecvTimeoutError::Timeout)
    ));
    require_ok(release_tx.send(()), "required test value");
    let mut engine = require_ok(sender.join(), "required test value");
    require_ok(receiver.join(), "required test value");
    let controller = require_ok(pauser.join(), "required test value");
    require_ok(
        paused_rx.recv_timeout(Duration::from_secs(2)),
        "required test value",
    );
    acknowledge(&mut engine, 0);
    assert_eq!(
        require_ok(controller.get(&target()), "required test value").last_acknowledged,
        cursor(42, 0, 0)
    );
    paused(&require_err(
        outbound::deliver(
            &mut engine,
            &endpoint,
            &authenticator(),
            &StopSignal::active(),
            0,
        ),
        "required test value",
    ));
}

fn saturated(root: &Path) -> (UnixListener, std::os::unix::net::UnixStream, Endpoint) {
    let path = root.join("backlog.sock");
    let fd = require_ok(
        rustix::net::socket_with(
            rustix::net::AddressFamily::UNIX,
            rustix::net::SocketType::STREAM,
            rustix::net::SocketFlags::CLOEXEC,
            None,
        ),
        "required test value",
    );
    require_ok(
        rustix::net::bind(
            &fd,
            &require_ok(
                rustix::net::SocketAddrUnix::new(&path),
                "required test value",
            ),
        ),
        "required test value",
    );
    require_ok(rustix::net::listen(&fd, 0), "required test value");
    let listener = UnixListener::from(fd);
    let blocker = require_ok(
        std::os::unix::net::UnixStream::connect(&path),
        "required test value",
    );
    // Prove the queue is actually full before driving the production connector.
    let probe = require_ok(
        rustix::net::socket_with(
            rustix::net::AddressFamily::UNIX,
            rustix::net::SocketType::STREAM,
            rustix::net::SocketFlags::CLOEXEC | rustix::net::SocketFlags::NONBLOCK,
            None,
        ),
        "required test value",
    );
    assert_eq!(
        require_err(
            rustix::net::connect(
                &probe,
                &require_ok(
                    rustix::net::SocketAddrUnix::new(&path),
                    "required test value"
                )
            ),
            "required test value"
        ),
        rustix::io::Errno::AGAIN
    );
    (listener, blocker, endpoint(path, Duration::from_millis(80)))
}

#[test]
fn outbound_connect_deadline_bounds_real_saturated_backlog_and_preserves_retry() {
    let root = Directory::new();
    let (_shared_store, _controller, mut engine) = fixture(&root.0);
    let (listener, blocker, endpoint) = saturated(&root.0);
    let started = Instant::now();
    let error = require_err(
        outbound::deliver(
            &mut engine,
            &endpoint,
            &authenticator(),
            &StopSignal::active(),
            100,
        ),
        "required test value",
    );
    assert!(
        matches!(error, OutboundError::RetryScheduled { failure: EndpointFailure::Timeout, plan } if plan.attempt == 1 && plan.retry_at_ms == 110)
    );
    assert!(started.elapsed() >= endpoint.deadline);
    assert!(started.elapsed() < Duration::from_secs(1));
    assert_eq!(engine.buffered_len(), 5);
    let (queued, _) = require_ok(listener.accept(), "required test value");
    drop(queued);
    drop(blocker);
    assert_event(receive_one(&listener, &endpoint, &mut engine), 0);
}

#[test]
fn outbound_connect_revocation_interrupts_real_saturated_backlog() {
    let root = Directory::new();
    let (_shared_store, _controller, mut engine) = fixture(&root.0);
    let (listener, _blocker, mut endpoint) = saturated(&root.0);
    endpoint.deadline = Duration::from_secs(2);
    let stop = StopSignal::active();
    let signal = stop.clone();
    let revoker = thread::spawn(move || {
        thread::sleep(Duration::from_millis(20));
        signal.stop(Termination::SessionRevoked);
    });
    let started = Instant::now();
    let error = require_err(
        outbound::deliver(&mut engine, &endpoint, &authenticator(), &stop, 0),
        "required test value",
    );
    assert!(matches!(
        error,
        OutboundError::Stopped(Termination::SessionRevoked)
    ));
    assert!(started.elapsed() < Duration::from_millis(500));
    require_ok(revoker.join(), "required test value");
    assert_eq!(engine.buffered_len(), 0);
    assert_eq!(
        require_ok(
            engine.into_subscriptions().termination(&target()),
            "required test value"
        ),
        Some(Termination::SessionRevoked)
    );
    let (_queued, _) = require_ok(listener.accept(), "required test value");
    assert_no_connection(&listener);
}

#[test]
fn pause_during_connection_establishment_is_rechecked_before_routing() {
    let root = Directory::new();
    let (_shared_store, mut controller, mut engine) = fixture(&root.0);
    require_ok(backfill(&mut engine), "required test value");
    let (listener, blocker, mut endpoint) = saturated(&root.0);
    endpoint.deadline = Duration::from_secs(2);
    let sender_endpoint = endpoint.clone();
    let (started_tx, started_rx) = mpsc::channel();
    let sender = thread::spawn(move || {
        let mut engine = engine;
        require_ok(started_tx.send(()), "required test value");
        let result = outbound::deliver(
            &mut engine,
            &sender_endpoint,
            &authenticator(),
            &StopSignal::active(),
            0,
        );
        (engine, result)
    });
    require_ok(
        started_rx.recv_timeout(Duration::from_secs(2)),
        "required test value",
    );
    thread::sleep(Duration::from_millis(20));
    require_ok(controller.pause(&target()), "required test value");
    let (queued, _) = require_ok(listener.accept(), "required test value");
    drop(queued);
    drop(blocker);
    let (mut engine, result) = require_ok(sender.join(), "required test value");
    paused(&require_err(result, "required test value"));
    let mut abandoned = accept_bounded(&listener);
    require_ok(
        abandoned.set_read_timeout(Some(Duration::from_secs(1))),
        "required test value",
    );
    assert_eq!(
        require_ok(abandoned.read(&mut [0; 1]), "required test value"),
        0,
        "no frame routed after pause"
    );
    require_ok(controller.resume(&target()), "required test value");
    assert_event(receive_one(&listener, &endpoint, &mut engine), 0);
}

#[test]
fn unacknowledged_window_survives_reopen_and_replays_in_order_with_slow_cursor_ack() {
    let root = Directory::new();
    let (shared, mut controller, mut engine) = fixture(&root.0);
    require_ok(
        controller.set_outstanding_window(&target(), 2),
        "required test value",
    );
    assert!(matches!(
        controller.set_outstanding_window(&target(), 0),
        Err(SubscriptionError::InvalidWindow)
    ));
    let (listener, endpoint) = listener(&root.0);
    for log in 0..2 {
        assert_event(receive_one(&listener, &endpoint, &mut engine), log);
    }
    assert!(matches!(
        controller.set_outstanding_window(&target(), 1),
        Err(SubscriptionError::InvalidWindow)
    ));
    let before = require_ok(fs::read(root.0.join("store.bin")), "required test value");
    let error = require_err(
        outbound::deliver(
            &mut engine,
            &endpoint,
            &authenticator(),
            &StopSignal::active(),
            0,
        ),
        "required test value",
    );
    assert!(matches!(
        error,
        OutboundError::Delivery(DeliveryError::Subscription(
            SubscriptionError::OutstandingWindow { limit: 2 }
        ))
    ));
    assert_no_connection(&listener);
    assert_eq!(
        require_ok(fs::read(root.0.join("store.bin")), "required test value"),
        before
    );
    drop(engine);
    drop(controller);
    drop(shared);
    let subscriptions = require_ok(
        SubscriptionStore::open(
            require_ok(Store::open(&root.0), "required test value"),
            tenant(),
        ),
        "required test value",
    );
    assert_eq!(
        require_ok(
            subscriptions.outstanding_window(&target()),
            "required test value"
        ),
        2
    );
    let mut engine = open_engine(subscriptions, 74);
    for log in 0..2 {
        assert_event(receive_one(&listener, &endpoint, &mut engine), log);
    }
    let error = require_err(
        outbound::deliver(
            &mut engine,
            &endpoint,
            &authenticator(),
            &StopSignal::active(),
            0,
        ),
        "required test value",
    );
    assert!(matches!(
        error,
        OutboundError::Delivery(DeliveryError::Subscription(
            SubscriptionError::OutstandingWindow { limit: 2 }
        ))
    ));
    assert_no_connection(&listener);
    acknowledge(&mut engine, 0);
    assert_event(receive_one(&listener, &endpoint, &mut engine), 2);
    acknowledge(&mut engine, 1);
    assert_event(receive_one(&listener, &endpoint, &mut engine), 3);
    acknowledge(&mut engine, 3);
    drop(engine);
    let mut engine = reopen(&root.0, 74);
    assert!(
        matches!(receive_one(&listener, &endpoint, &mut engine), VerifiedItem::BackfillComplete { resume_cursor, .. } if resume_cursor == cursor(42, 0, 3))
    );
    assert!(require_ok(deliver(&mut engine), "required test value").is_none());
}

#[test]
fn outbound_peer_authenticates_cross_uid_and_refuses_wrong_uid_processes() {
    let uid = require_ok(fs::metadata("/proc/self"), "required test value").uid();
    if uid == 65534 {
        let mut input = String::new();
        require_ok(
            std::io::stdin().read_line(&mut input),
            "required test value",
        );
        let (mode, path) = require_some(input.trim().split_once(' '), "required test value");
        let listener = require_ok(UnixListener::bind(path), "required test value");
        println!("RECEIVER_READY");
        require_ok(std::io::stdout().flush(), "required test value");
        let mut stream = accept_bounded(&listener);
        require_ok(
            stream.set_read_timeout(Some(Duration::from_secs(2))),
            "required test value",
        );
        assert_eq!(
            require_ok(outbound::peer_identity(&stream), "required test value").uid,
            0
        );
        if mode == "accept" {
            let frame = require_ok(read_frame(&mut stream, 16384), "required test value");
            let auth = authenticator();
            let verified = require_ok(
                auth.verify(
                    &frame,
                    PeerIdentity {
                        uid: 65534,
                        gid: 65534,
                    },
                ),
                "required test value",
            );
            assert_event(verified.item, 0);
            require_ok(
                write_frame(&mut stream, &auth.acknowledgement(verified.binding), 16384),
                "required test value",
            );
        } else {
            assert_eq!(mode, "reject");
            assert_eq!(
                require_ok(stream.read(&mut [0; 1]), "required test value"),
                0
            );
        }
        return;
    }
    assert_eq!(
        uid, 0,
        "cross-UID receiver fixture requires a sandbox that can launch another UID"
    );
    let root = Directory::new();
    let (_shared_store, _controller, mut engine) = fixture(&root.0);
    assert_eq!(
        require_ok(fs::metadata(&root.0), "required test value").mode() & 0o7777,
        0o700
    );
    let sockets = Directory::new();
    require_ok(fs::create_dir(&sockets.0), "required test value");
    require_ok(
        fs::set_permissions(&sockets.0, fs::Permissions::from_mode(0o777)),
        "required test value",
    );
    for mode in ["reject", "accept"] {
        let path = sockets.0.join(format!("{mode}.sock"));
        let child = require_ok(
            Command::new(require_ok(std::env::current_exe(), "required test value"))
                .args([
                    "--exact",
                    "outbound_peer_authenticates_cross_uid_and_refuses_wrong_uid_processes",
                    "--nocapture",
                ])
                .uid(65534)
                .gid(65534)
                .stdin(Stdio::piped())
                .stdout(Stdio::piped())
                .stderr(Stdio::piped())
                .spawn(),
            "required test value",
        );
        finish_outbound_peer_authenticates_cross_uid_and_refuses_wrong_uid_processes_1_repair_22_33_1506(&mut engine, mode, path, child);
    }
    acknowledge(&mut engine, 0);
}

fn finish_outbound_peer_authenticates_cross_uid_and_refuses_wrong_uid_processes_1_repair_22_33_1506(
    engine: &mut DeliveryEngine,
    mode: &'static str,
    path: PathBuf,
    mut child: std::process::Child,
) {
    require_ok(
        writeln!(
            require_some(child.stdin.take(), "required test value"),
            "{mode} {}",
            path.display()
        ),
        "required test value",
    );
    let mut output = BufReader::new(require_some(child.stdout.take(), "required test value"));
    loop {
        let mut line = String::new();
        assert_ne!(
            require_ok(output.read_line(&mut line), "required test value"),
            0,
            "receiver failed to start"
        );
        if line.contains("RECEIVER_READY") {
            break;
        }
    }
    let mut endpoint = endpoint(path, Duration::from_secs(2));
    if mode == "accept" {
        endpoint.expected_peer = PeerIdentity {
            uid: 65534,
            gid: 65534,
        };
    }
    let result = outbound::deliver(
        engine,
        &endpoint,
        &authenticator(),
        &StopSignal::active(),
        0,
    );
    if mode == "reject" {
        assert!(matches!(
            result,
            Err(OutboundError::RetryScheduled {
                failure: EndpointFailure::Authentication,
                ..
            })
        ));
        assert_eq!(engine.buffered_len(), 5);
    } else {
        let receipt = require_ok(result, "required test value");
        assert_eq!(
            receipt.peer,
            PeerIdentity {
                uid: 65534,
                gid: 65534
            }
        );
        assert!(matches!(receipt.item, DeliveryItem::Event(_)));
    }
    let result = require_ok(child.wait_with_output(), "required test value");
    assert!(
        result.status.success(),
        "receiver process: {}",
        String::from_utf8_lossy(&result.stderr)
    );
}

fn key(id: &SubscriptionId) -> TenantKey {
    require_ok(
        TenantKey::new(
            tenant(),
            ObjectKind::Subscription,
            id.as_str().as_bytes().to_vec(),
        ),
        "required test value",
    )
}

fn downgrade(bytes: &[u8], magic: [u8; 4]) -> Vec<u8> {
    let magic = &magic;
    assert_eq!(&bytes[..4], b"LXS3");
    let string_end = |offset: usize| {
        let length = u32::from_be_bytes(require_ok(
            bytes[offset..offset + 4].try_into(),
            "required test value",
        )) as usize;
        offset + 4 + length
    };
    let mut binding = 4;
    for _ in 0..4 {
        binding = string_end(binding);
    }
    assert_eq!(bytes[binding], 0);
    let cursors = string_end(binding + 1);
    let mut old = magic.to_vec();
    if magic == b"LXSB" {
        old.extend_from_slice(&bytes[4..binding]);
        old.extend_from_slice(&bytes[binding + 1..]);
    } else {
        assert_eq!(magic, b"LXS2");
        old.extend_from_slice(&bytes[4..cursors]);
        old.extend_from_slice(&bytes[cursors..cursors + 8]);
        old.extend_from_slice(&bytes[cursors + 16..cursors + 24]);
        let count_at = cursors + 32;
        let count = u32::from_be_bytes(require_ok(
            bytes[count_at..count_at + 4].try_into(),
            "required test value",
        )) as usize;
        old.extend_from_slice(&bytes[count_at..count_at + 4]);
        for index in 0..count {
            let start = count_at + 4 + index * 16;
            old.extend_from_slice(&bytes[start..start + 8]);
        }
        old.extend_from_slice(&bytes[count_at + 4 + count * 16..]);
    }
    old
}

#[test]
fn subscription_upgrade_recovery_identifies_records_backs_up_reset_and_reopens() {
    let root = Directory::new();
    let (shared, mut controller, engine) = fixture(&root.0);
    drop(engine);
    let request = SubscriptionCreate {
        scope: scope(),
        filter: require_ok(controller.get(&target()), "required test value").filter,
        start: cursor(41, 0, 0),
        delivery_target: require_ok(DeliveryTarget::new("uds://consumer"), "required test value"),
    };
    let mut originals = Vec::new();
    for (id, magic) in [("old-scalar", b"LXS2"), ("old-unbound", b"LXSB")] {
        let id = require_ok(SubscriptionId::new(id), "required test value");
        require_ok(
            controller.create(id.clone(), request.clone()),
            "required test value",
        );
        require_ok(
            controller.mark_delivered(
                &SubscriptionTarget {
                    scope: scope(),
                    subscription_id: id.clone(),
                },
                cursor(42, 0, 0).into(),
            ),
            "required test value",
        );
        let mut durable = require_ok(shared.lock(), "required test value");
        let bytes = downgrade(
            require_some(durable.get(&key(&id)), "required test value").bytes(),
            *magic,
        );
        require_ok(
            durable.put_local(key(&id), bytes.clone()),
            "required test value",
        );
        originals.push((id, bytes));
    }
    finish_subscription_upgrade_recovery_identifies_records_backs_up_reset_and_reopens_2_repair_22_33_1506(&root, shared, controller, &originals);
}

fn finish_subscription_upgrade_recovery_identifies_records_backs_up_reset_and_reopens_2_repair_22_33_1506(
    root: &Directory,
    shared: Arc<Mutex<Store>>,
    controller: SubscriptionStore,
    originals: &Vec<(SubscriptionId, Vec<u8>)>,
) {
    drop(controller);
    let current = require_some(
        shared
            .lock()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
            .get(&key(&target().subscription_id)),
        "required test value",
    )
    .bytes()
    .to_vec();
    let error = require_err(
        SubscriptionStore::open_shared(shared.clone(), tenant()),
        "required test value",
    );
    assert!(
        matches!(&error, SubscriptionError::UpgradeRequired { tenant, subscription_id } if tenant == "tenant-a" && subscription_id == "old-scalar")
    );
    assert!(error.to_string().contains("replay boundary"));
    let mut backups = Vec::new();
    for (id, original) in originals {
        let target = SubscriptionTarget {
            scope: scope(),
            subscription_id: id.clone(),
        };
        let mut durable = require_ok(shared.lock(), "required test value");
        let backup = require_ok(
            SubscriptionStore::recover_legacy(&mut durable, &target, cursor(41, 0, 0)),
            "required test value",
        );
        let audit = require_some(durable.get(&backup), "required test value").bytes();
        assert_eq!(&audit[..5], b"LXSR1");
        let length =
            u32::from_be_bytes(require_ok(audit[5..9].try_into(), "required test value")) as usize;
        assert_eq!(&audit[9..9 + length], original);
        assert_eq!(
            &audit[9 + length..],
            require_some(durable.get(&key(id)), "required test value").bytes()
        );
        backups.push(backup);
    }
    drop(shared);
    let durable = require_ok(Store::open(&root.0), "required test value");
    assert_eq!(
        require_some(
            durable.get(&key(&target().subscription_id)),
            "required test value"
        )
        .bytes(),
        current
    );
    for (backup, (_, original)) in backups.iter().zip(originals) {
        assert_eq!(
            &require_some(durable.get(backup), "required test value").bytes()
                [9..9 + original.len()],
            original
        );
    }
    let mut subscriptions = require_ok(
        SubscriptionStore::open(durable, tenant()),
        "required test value",
    );
    assert_eq!(subscriptions.list(&scope()).len(), 3);
    for (id, _) in originals {
        let target = SubscriptionTarget {
            scope: scope(),
            subscription_id: id.clone(),
        };
        let record = require_ok(subscriptions.get(&target), "required test value");
        assert!(record.paused);
        assert_eq!(record.last_acknowledged, cursor(41, 0, 0));
        require_ok(subscriptions.resume(&target), "required test value");
    }
    // Real ingestion -> outbound codec -> consumer replay after operator recovery.
    finish_subscription_upgrade_recovery_identifies_records_backs_up_reset_and_reopens_1_repair_22_33_1506(root, originals, subscriptions);
}

fn finish_subscription_upgrade_recovery_identifies_records_backs_up_reset_and_reopens_1_repair_22_33_1506(
    root: &Directory,
    originals: &[(SubscriptionId, Vec<u8>)],
    subscriptions: SubscriptionStore,
) {
    let id = originals[0].0.clone();
    let recovered_target = SubscriptionTarget {
        scope: scope(),
        subscription_id: id.clone(),
    };
    let mut engine = require_ok(
        DeliveryEngine::open(
            subscriptions,
            recovered_target.clone(),
            74,
            16,
            RetryPolicy {
                base_delay_ms: 10,
                maximum_delay_ms: 100,
                jitter_percent: 0,
                maximum_attempts: 3,
            },
        ),
        "required test value",
    );
    let (listener, endpoint) = listener(&root.0);
    let peer = endpoint.expected_peer;
    let receiver = thread::spawn(move || {
        let mut stream = accept_bounded(&listener);
        require_ok(
            stream.set_read_timeout(Some(Duration::from_secs(2))),
            "required test value",
        );
        let auth = authenticator();
        let verified = require_ok(
            auth.verify(
                &require_ok(read_frame(&mut stream, 16384), "required test value"),
                peer,
            ),
            "required test value",
        );
        assert_eq!(verified.subscription_id, "old-scalar");
        assert_event(verified.item, 0);
        require_ok(
            write_frame(&mut stream, &auth.acknowledgement(verified.binding), 16384),
            "required test value",
        );
    });
    require_ok(
        outbound::deliver(
            &mut engine,
            &endpoint,
            &authenticator(),
            &StopSignal::active(),
            0,
        ),
        "required test value",
    );
    require_ok(receiver.join(), "required test value");
    require_ok(
        engine.acknowledge(&CursorAcknowledgement {
            scope: scope(),
            subscription_id: id,
            cursor: cursor(42, 0, 0),
        }),
        "required test value",
    );
    drop(engine);
    let subscriptions = require_ok(
        SubscriptionStore::open(
            require_ok(Store::open(&root.0), "required test value"),
            tenant(),
        ),
        "required test value",
    );
    assert_eq!(
        require_ok(subscriptions.get(&recovered_target), "required test value").last_acknowledged,
        cursor(42, 0, 0)
    );
    assert_eq!(subscriptions.list(&scope()).len(), 3);
}
