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
    TenantId::new("tenant-a").unwrap()
}

fn scope() -> SubscriptionScope {
    SubscriptionScope {
        tenant: ApiTenantId::new("tenant-a").unwrap(),
        agent: AgentDid::new("agent-a").unwrap(),
        capability: CapabilityId::new("capability-a").unwrap(),
    }
}

fn target() -> SubscriptionTarget {
    SubscriptionTarget {
        scope: scope(),
        subscription_id: SubscriptionId::new("subscription-a").unwrap(),
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
        "transactionHash": format!("0x{}", hex::encode([tx as u8 + 1; 32])),
        "blockHash": format!("0x{}", hex::encode([height as u8; 32])),
        "address": format!("0x{}", "12".repeat(20)),
        "topics": [format!("0x{}", "34".repeat(32))],
        "data": "0x00ff", "removed": false
    })
}

fn open_ingestor(root: &Path, initial_sequence: u64) -> EventIngestor {
    EventIngestor::open(Store::open(root).unwrap(), tenant(), 32, initial_sequence).unwrap()
}

fn close_block(ingestor: &mut EventIngestor, head: u64) {
    // Adjacent heads flush the actual pending-log journal without a history gap.
    let source = EthLogSource::new(
        "http://127.0.0.1:9",
        &[],
        SubscriptionTopic::Logs {
            address: None,
            topics: vec![],
        },
    )
    .unwrap();
    ingestor
        .new_head(
            head,
            &source,
            HistoryLimits {
                maximum_items: 32,
                maximum_response_bytes: 1_048_576,
            },
            32,
            attributes(),
        )
        .unwrap();
}

fn create(ingestor: EventIngestor, start: Cursor) -> SubscriptionStore {
    let mut store = SubscriptionStore::open(ingestor.into_store(), tenant()).unwrap();
    let owned = |value| TenantObject {
        tenant: scope().tenant,
        value,
    };
    store
        .create(
            target().subscription_id,
            SubscriptionCreate {
                scope: scope(),
                filter: SubscriptionFilter {
                    agents: ExplicitSet::allow(vec![owned(AgentDid::new("agent-a").unwrap())]),
                    accounts: ExplicitSet::allow(vec![TenantObject {
                        tenant: scope().tenant,
                        value: AccountRef::new("account-a").unwrap(),
                    }]),
                    activity_types: ExplicitSet::allow(vec![ActivityType(9)]),
                    modules: ExplicitSet::allow(vec![TenantObject {
                        tenant: scope().tenant,
                        value: ModuleRef::new("asset").unwrap(),
                    }]),
                    assets: ExplicitSet::allow(vec![TenantObject {
                        tenant: scope().tenant,
                        value: Asset::new("LXR").unwrap(),
                    }]),
                    counterparties: ExplicitSet::allow(vec![TenantObject {
                        tenant: scope().tenant,
                        value: Counterparty::new("counterparty-a").unwrap(),
                    }]),
                    result_classes: ExplicitSet::allow(vec![ResultCode::from_raw(0)]),
                },
                start,
                delivery_target: DeliveryTarget::new("uds://consumer").unwrap(),
            },
        )
        .unwrap();
    store
}

fn open_engine(store: SubscriptionStore, live_start: u64) -> DeliveryEngine {
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
    )
    .unwrap()
}

fn reopen(root: &Path, live_start: u64) -> DeliveryEngine {
    open_engine(
        SubscriptionStore::open(Store::open(root).unwrap(), tenant()).unwrap(),
        live_start,
    )
}

fn authenticator() -> Authenticator {
    Authenticator::new("consumer-key", [0x8a; 32]).unwrap()
}

fn fixture(root: &Path) -> (Arc<Mutex<Store>>, SubscriptionStore, DeliveryEngine) {
    let mut ingestor = open_ingestor(root, 70);
    for log in 0..4 {
        ingestor
            .ingest_log(&node_log(42, 0, log), attributes())
            .unwrap();
    }
    close_block(&mut ingestor, 43);
    let durable = create(ingestor, cursor(41, 0, 0)).into_durable().unwrap();
    let shared = Arc::new(Mutex::new(durable));
    let controller = SubscriptionStore::open_shared(shared.clone(), tenant()).unwrap();
    let engine = open_engine(
        SubscriptionStore::open_shared(shared.clone(), tenant()).unwrap(),
        74,
    );
    (shared, controller, engine)
}

fn endpoint(path: PathBuf, deadline: Duration) -> Endpoint {
    let metadata = fs::metadata("/proc/self").unwrap();
    Endpoint::new(
        path,
        PeerIdentity {
            uid: metadata.uid(),
            gid: metadata.gid(),
        },
        16384,
        deadline,
        Duration::from_millis(2),
    )
    .unwrap()
}

fn listener(root: &Path) -> (UnixListener, Endpoint) {
    let path = root.join("consumer.sock");
    let listener = UnixListener::bind(&path).unwrap();
    (listener, endpoint(path, Duration::from_secs(2)))
}

fn accept_bounded(listener: &UnixListener) -> std::os::unix::net::UnixStream {
    listener.set_nonblocking(true).unwrap();
    let started = Instant::now();
    loop {
        match listener.accept() {
            Ok((stream, _)) => {
                listener.set_nonblocking(false).unwrap();
                stream.set_nonblocking(false).unwrap();
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
    let listener = listener.try_clone().unwrap();
    let peer = endpoint.expected_peer;
    let receiver = thread::spawn(move || {
        let mut stream = accept_bounded(&listener);
        stream
            .set_read_timeout(Some(Duration::from_secs(2)))
            .unwrap();
        let frame = read_frame(&mut stream, 16384).unwrap();
        let auth = authenticator();
        let verified = auth.verify(&frame, peer).unwrap();
        assert_eq!(verified.subscription_id, "subscription-a");
        assert_eq!(verified.tenant, "tenant-a");
        write_frame(&mut stream, &auth.acknowledgement(verified.binding), 16384).unwrap();
        verified.item
    });
    let receipt = outbound::deliver(
        engine,
        endpoint,
        &authenticator(),
        &StopSignal::active(),
        100,
    )
    .unwrap();
    assert_eq!(receipt.peer, endpoint.expected_peer);
    receiver.join().unwrap()
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
        serde_json::from_slice::<Value>(&event_bytes).unwrap(),
        node_log(42, 0, log)
    );
}

fn acknowledge(engine: &mut DeliveryEngine, log: u32) {
    let record = engine
        .acknowledge(&CursorAcknowledgement {
            scope: scope(),
            subscription_id: target().subscription_id,
            cursor: cursor(42, 0, log),
        })
        .unwrap();
    assert_eq!(record.last_acknowledged, cursor(42, 0, log));
}

fn assert_no_connection(listener: &UnixListener) {
    listener.set_nonblocking(true).unwrap();
    assert_eq!(
        listener.accept().unwrap_err().kind(),
        std::io::ErrorKind::WouldBlock
    );
    listener.set_nonblocking(false).unwrap();
}

fn paused(error: OutboundError) {
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
    let (_shared, mut controller, mut engine) = fixture(&root.0);
    let (listener, endpoint) = listener(&root.0);
    backfill(&mut engine).unwrap();
    assert_eq!(engine.buffered_len(), 5);
    controller.pause(&target()).unwrap();
    assert!(matches!(
        deliver(&mut engine),
        Err(DeliveryError::Subscription(SubscriptionError::Paused))
    ));
    paused(
        outbound::deliver(
            &mut engine,
            &endpoint,
            &authenticator(),
            &StopSignal::active(),
            0,
        )
        .unwrap_err(),
    );
    assert_no_connection(&listener);
    assert_eq!(engine.buffered_len(), 5);
    controller.resume(&target()).unwrap();
    assert_event(receive_one(&listener, &endpoint, &mut engine), 0);
    acknowledge(&mut engine, 0);
    assert_eq!(engine.buffered_len(), 4);
    assert_no_connection(&listener);
    drop(engine);
    drop(controller);
    drop(_shared);
    let mut engine = reopen(&root.0, 74);
    assert_event(receive_one(&listener, &endpoint, &mut engine), 1);
}

#[test]
fn pause_waits_for_inflight_acknowledgement_and_retains_its_delivery() {
    let root = Directory::new();
    let (_shared, mut controller, engine) = fixture(&root.0);
    let (listener, endpoint) = listener(&root.0);
    let (routed_tx, routed_rx) = mpsc::channel();
    let (release_tx, release_rx) = mpsc::channel();
    let peer = endpoint.expected_peer;
    let receiver = thread::spawn(move || {
        let mut stream = accept_bounded(&listener);
        stream
            .set_read_timeout(Some(Duration::from_secs(2)))
            .unwrap();
        let auth = authenticator();
        let frame = read_frame(&mut stream, 16384).unwrap();
        let verified = auth.verify(&frame, peer).unwrap();
        assert_event(verified.item, 0);
        routed_tx.send(()).unwrap();
        release_rx.recv_timeout(Duration::from_secs(2)).unwrap();
        write_frame(&mut stream, &auth.acknowledgement(verified.binding), 16384).unwrap();
    });
    let sender_endpoint = endpoint.clone();
    let sender = thread::spawn(move || {
        let mut engine = engine;
        outbound::deliver(
            &mut engine,
            &sender_endpoint,
            &authenticator(),
            &StopSignal::active(),
            0,
        )
        .unwrap();
        engine
    });
    routed_rx.recv_timeout(Duration::from_secs(2)).unwrap();
    let (paused_tx, paused_rx) = mpsc::channel();
    let (started_tx, started_rx) = mpsc::channel();
    let pauser = thread::spawn(move || {
        started_tx.send(()).unwrap();
        let record = controller.pause(&target()).unwrap();
        assert!(record.paused);
        paused_tx.send(()).unwrap();
        controller
    });
    started_rx.recv_timeout(Duration::from_secs(2)).unwrap();
    assert!(matches!(
        paused_rx.recv_timeout(Duration::from_millis(30)),
        Err(mpsc::RecvTimeoutError::Timeout)
    ));
    release_tx.send(()).unwrap();
    let mut engine = sender.join().unwrap();
    receiver.join().unwrap();
    let controller = pauser.join().unwrap();
    paused_rx.recv_timeout(Duration::from_secs(2)).unwrap();
    acknowledge(&mut engine, 0);
    assert_eq!(
        controller.get(&target()).unwrap().last_acknowledged,
        cursor(42, 0, 0)
    );
    paused(
        outbound::deliver(
            &mut engine,
            &endpoint,
            &authenticator(),
            &StopSignal::active(),
            0,
        )
        .unwrap_err(),
    );
}

fn saturated(root: &Path) -> (UnixListener, std::os::unix::net::UnixStream, Endpoint) {
    let path = root.join("backlog.sock");
    let fd = rustix::net::socket_with(
        rustix::net::AddressFamily::UNIX,
        rustix::net::SocketType::STREAM,
        rustix::net::SocketFlags::CLOEXEC,
        None,
    )
    .unwrap();
    rustix::net::bind(&fd, &rustix::net::SocketAddrUnix::new(&path).unwrap()).unwrap();
    rustix::net::listen(&fd, 0).unwrap();
    let listener = UnixListener::from(fd);
    let blocker = std::os::unix::net::UnixStream::connect(&path).unwrap();
    // Prove the queue is actually full before driving the production connector.
    let probe = rustix::net::socket_with(
        rustix::net::AddressFamily::UNIX,
        rustix::net::SocketType::STREAM,
        rustix::net::SocketFlags::CLOEXEC | rustix::net::SocketFlags::NONBLOCK,
        None,
    )
    .unwrap();
    assert_eq!(
        rustix::net::connect(&probe, &rustix::net::SocketAddrUnix::new(&path).unwrap())
            .unwrap_err(),
        rustix::io::Errno::AGAIN
    );
    (listener, blocker, endpoint(path, Duration::from_millis(80)))
}

#[test]
fn outbound_connect_deadline_bounds_real_saturated_backlog_and_preserves_retry() {
    let root = Directory::new();
    let (_shared, _controller, mut engine) = fixture(&root.0);
    let (listener, blocker, endpoint) = saturated(&root.0);
    let started = Instant::now();
    let error = outbound::deliver(
        &mut engine,
        &endpoint,
        &authenticator(),
        &StopSignal::active(),
        100,
    )
    .unwrap_err();
    assert!(
        matches!(error, OutboundError::RetryScheduled { failure: EndpointFailure::Timeout, plan } if plan.attempt == 1 && plan.retry_at_ms == 110)
    );
    assert!(started.elapsed() >= endpoint.deadline);
    assert!(started.elapsed() < Duration::from_secs(1));
    assert_eq!(engine.buffered_len(), 5);
    let (queued, _) = listener.accept().unwrap();
    drop(queued);
    drop(blocker);
    assert_event(receive_one(&listener, &endpoint, &mut engine), 0);
}

#[test]
fn outbound_connect_revocation_interrupts_real_saturated_backlog() {
    let root = Directory::new();
    let (_shared, _controller, mut engine) = fixture(&root.0);
    let (listener, _blocker, mut endpoint) = saturated(&root.0);
    endpoint.deadline = Duration::from_secs(2);
    let stop = StopSignal::active();
    let signal = stop.clone();
    let revoker = thread::spawn(move || {
        thread::sleep(Duration::from_millis(20));
        signal.stop(Termination::SessionRevoked);
    });
    let started = Instant::now();
    let error = outbound::deliver(&mut engine, &endpoint, &authenticator(), &stop, 0).unwrap_err();
    assert!(matches!(
        error,
        OutboundError::Stopped(Termination::SessionRevoked)
    ));
    assert!(started.elapsed() < Duration::from_millis(500));
    revoker.join().unwrap();
    assert_eq!(engine.buffered_len(), 0);
    assert_eq!(
        engine.into_subscriptions().termination(&target()).unwrap(),
        Some(Termination::SessionRevoked)
    );
    let (_queued, _) = listener.accept().unwrap();
    assert_no_connection(&listener);
}

#[test]
fn pause_during_connection_establishment_is_rechecked_before_routing() {
    let root = Directory::new();
    let (_shared, mut controller, mut engine) = fixture(&root.0);
    backfill(&mut engine).unwrap();
    let (listener, blocker, mut endpoint) = saturated(&root.0);
    endpoint.deadline = Duration::from_secs(2);
    let sender_endpoint = endpoint.clone();
    let (started_tx, started_rx) = mpsc::channel();
    let sender = thread::spawn(move || {
        let mut engine = engine;
        started_tx.send(()).unwrap();
        let result = outbound::deliver(
            &mut engine,
            &sender_endpoint,
            &authenticator(),
            &StopSignal::active(),
            0,
        );
        (engine, result)
    });
    started_rx.recv_timeout(Duration::from_secs(2)).unwrap();
    thread::sleep(Duration::from_millis(20));
    controller.pause(&target()).unwrap();
    let (queued, _) = listener.accept().unwrap();
    drop(queued);
    drop(blocker);
    let (mut engine, result) = sender.join().unwrap();
    paused(result.unwrap_err());
    let mut abandoned = accept_bounded(&listener);
    abandoned
        .set_read_timeout(Some(Duration::from_secs(1)))
        .unwrap();
    assert_eq!(
        abandoned.read(&mut [0; 1]).unwrap(),
        0,
        "no frame routed after pause"
    );
    controller.resume(&target()).unwrap();
    assert_event(receive_one(&listener, &endpoint, &mut engine), 0);
}

#[test]
fn unacknowledged_window_survives_reopen_and_replays_in_order_with_slow_cursor_ack() {
    let root = Directory::new();
    let (shared, mut controller, mut engine) = fixture(&root.0);
    controller.set_outstanding_window(&target(), 2).unwrap();
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
    let before = fs::read(root.0.join("store.bin")).unwrap();
    let error = outbound::deliver(
        &mut engine,
        &endpoint,
        &authenticator(),
        &StopSignal::active(),
        0,
    )
    .unwrap_err();
    assert!(matches!(
        error,
        OutboundError::Delivery(DeliveryError::Subscription(
            SubscriptionError::OutstandingWindow { limit: 2 }
        ))
    ));
    assert_no_connection(&listener);
    assert_eq!(fs::read(root.0.join("store.bin")).unwrap(), before);
    drop(engine);
    drop(controller);
    drop(shared);
    let subscriptions = SubscriptionStore::open(Store::open(&root.0).unwrap(), tenant()).unwrap();
    assert_eq!(subscriptions.outstanding_window(&target()).unwrap(), 2);
    let mut engine = open_engine(subscriptions, 74);
    for log in 0..2 {
        assert_event(receive_one(&listener, &endpoint, &mut engine), log);
    }
    let error = outbound::deliver(
        &mut engine,
        &endpoint,
        &authenticator(),
        &StopSignal::active(),
        0,
    )
    .unwrap_err();
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
    assert!(deliver(&mut engine).unwrap().is_none());
}

#[test]
fn outbound_peer_authenticates_cross_uid_and_refuses_wrong_uid_processes() {
    let uid = fs::metadata("/proc/self").unwrap().uid();
    if uid == 65534 {
        let mut input = String::new();
        std::io::stdin().read_line(&mut input).unwrap();
        let (mode, path) = input.trim().split_once(' ').unwrap();
        let listener = UnixListener::bind(path).unwrap();
        println!("RECEIVER_READY");
        std::io::stdout().flush().unwrap();
        let mut stream = accept_bounded(&listener);
        stream
            .set_read_timeout(Some(Duration::from_secs(2)))
            .unwrap();
        assert_eq!(outbound::peer_identity(&stream).unwrap().uid, 0);
        if mode == "accept" {
            let frame = read_frame(&mut stream, 16384).unwrap();
            let auth = authenticator();
            let verified = auth
                .verify(
                    &frame,
                    PeerIdentity {
                        uid: 65534,
                        gid: 65534,
                    },
                )
                .unwrap();
            assert_event(verified.item, 0);
            write_frame(&mut stream, &auth.acknowledgement(verified.binding), 16384).unwrap();
        } else {
            assert_eq!(mode, "reject");
            assert_eq!(stream.read(&mut [0; 1]).unwrap(), 0);
        }
        return;
    }
    assert_eq!(
        uid, 0,
        "cross-UID receiver fixture requires a sandbox that can launch another UID"
    );
    let root = Directory::new();
    let (_shared, _controller, mut engine) = fixture(&root.0);
    assert_eq!(fs::metadata(&root.0).unwrap().mode() & 0o7777, 0o700);
    let sockets = Directory::new();
    fs::create_dir(&sockets.0).unwrap();
    fs::set_permissions(&sockets.0, fs::Permissions::from_mode(0o777)).unwrap();
    for mode in ["reject", "accept"] {
        let path = sockets.0.join(format!("{mode}.sock"));
        let mut child = Command::new(std::env::current_exe().unwrap())
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
            .spawn()
            .unwrap();
        writeln!(child.stdin.take().unwrap(), "{mode} {}", path.display()).unwrap();
        let mut output = BufReader::new(child.stdout.take().unwrap());
        loop {
            let mut line = String::new();
            assert_ne!(
                output.read_line(&mut line).unwrap(),
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
            &mut engine,
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
            let receipt = result.unwrap();
            assert_eq!(
                receipt.peer,
                PeerIdentity {
                    uid: 65534,
                    gid: 65534
                }
            );
            assert!(matches!(receipt.item, DeliveryItem::Event(_)));
        }
        let result = child.wait_with_output().unwrap();
        assert!(
            result.status.success(),
            "receiver process: {}",
            String::from_utf8_lossy(&result.stderr)
        );
    }
    acknowledge(&mut engine, 0);
}

fn key(id: &SubscriptionId) -> TenantKey {
    TenantKey::new(
        tenant(),
        ObjectKind::Subscription,
        id.as_str().as_bytes().to_vec(),
    )
    .unwrap()
}

fn downgrade(bytes: &[u8], magic: &[u8; 4]) -> Vec<u8> {
    assert_eq!(&bytes[..4], b"LXS3");
    let string_end = |offset: usize| {
        let length = u32::from_be_bytes(bytes[offset..offset + 4].try_into().unwrap()) as usize;
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
        let count = u32::from_be_bytes(bytes[count_at..count_at + 4].try_into().unwrap()) as usize;
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
        filter: controller.get(&target()).unwrap().filter,
        start: cursor(41, 0, 0),
        delivery_target: DeliveryTarget::new("uds://consumer").unwrap(),
    };
    let mut originals = Vec::new();
    for (id, magic) in [("old-scalar", b"LXS2"), ("old-unbound", b"LXSB")] {
        let id = SubscriptionId::new(id).unwrap();
        controller.create(id.clone(), request.clone()).unwrap();
        controller
            .mark_delivered(
                &SubscriptionTarget {
                    scope: scope(),
                    subscription_id: id.clone(),
                },
                cursor(42, 0, 0).into(),
            )
            .unwrap();
        let mut durable = shared.lock().unwrap();
        let bytes = downgrade(durable.get(&key(&id)).unwrap().bytes(), magic);
        durable.put_local(key(&id), bytes.clone()).unwrap();
        originals.push((id, bytes));
    }
    drop(controller);
    let current = shared
        .lock()
        .unwrap()
        .get(&key(&target().subscription_id))
        .unwrap()
        .bytes()
        .to_vec();
    let error = SubscriptionStore::open_shared(shared.clone(), tenant()).unwrap_err();
    assert!(
        matches!(&error, SubscriptionError::UpgradeRequired { tenant, subscription_id } if tenant == "tenant-a" && subscription_id == "old-scalar")
    );
    assert!(error.to_string().contains("replay boundary"));
    let mut backups = Vec::new();
    for (id, original) in &originals {
        let target = SubscriptionTarget {
            scope: scope(),
            subscription_id: id.clone(),
        };
        let mut durable = shared.lock().unwrap();
        let backup =
            SubscriptionStore::recover_legacy(&mut durable, &target, cursor(41, 0, 0)).unwrap();
        let audit = durable.get(&backup).unwrap().bytes();
        assert_eq!(&audit[..5], b"LXSR1");
        let length = u32::from_be_bytes(audit[5..9].try_into().unwrap()) as usize;
        assert_eq!(&audit[9..9 + length], original);
        assert_eq!(&audit[9 + length..], durable.get(&key(id)).unwrap().bytes());
        backups.push(backup);
    }
    drop(shared);
    let durable = Store::open(&root.0).unwrap();
    assert_eq!(
        durable
            .get(&key(&target().subscription_id))
            .unwrap()
            .bytes(),
        current
    );
    for (backup, (_, original)) in backups.iter().zip(&originals) {
        assert_eq!(
            &durable.get(backup).unwrap().bytes()[9..9 + original.len()],
            original
        );
    }
    let mut subscriptions = SubscriptionStore::open(durable, tenant()).unwrap();
    assert_eq!(subscriptions.list(&scope()).len(), 3);
    for (id, _) in &originals {
        let target = SubscriptionTarget {
            scope: scope(),
            subscription_id: id.clone(),
        };
        let record = subscriptions.get(&target).unwrap();
        assert!(record.paused);
        assert_eq!(record.last_acknowledged, cursor(41, 0, 0));
        subscriptions.resume(&target).unwrap();
    }
    // Real ingestion -> outbound codec -> consumer replay after operator recovery.
    let id = originals[0].0.clone();
    let recovered_target = SubscriptionTarget {
        scope: scope(),
        subscription_id: id.clone(),
    };
    let mut engine = DeliveryEngine::open(
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
    )
    .unwrap();
    let (listener, endpoint) = listener(&root.0);
    let peer = endpoint.expected_peer;
    let receiver = thread::spawn(move || {
        let mut stream = accept_bounded(&listener);
        stream
            .set_read_timeout(Some(Duration::from_secs(2)))
            .unwrap();
        let auth = authenticator();
        let verified = auth
            .verify(&read_frame(&mut stream, 16384).unwrap(), peer)
            .unwrap();
        assert_eq!(verified.subscription_id, "old-scalar");
        assert_event(verified.item, 0);
        write_frame(&mut stream, &auth.acknowledgement(verified.binding), 16384).unwrap();
    });
    outbound::deliver(
        &mut engine,
        &endpoint,
        &authenticator(),
        &StopSignal::active(),
        0,
    )
    .unwrap();
    receiver.join().unwrap();
    engine
        .acknowledge(&CursorAcknowledgement {
            scope: scope(),
            subscription_id: id,
            cursor: cursor(42, 0, 0),
        })
        .unwrap();
    drop(engine);
    let subscriptions = SubscriptionStore::open(Store::open(&root.0).unwrap(), tenant()).unwrap();
    assert_eq!(
        subscriptions
            .get(&recovered_target)
            .unwrap()
            .last_acknowledged,
        cursor(42, 0, 0)
    );
    assert_eq!(subscriptions.list(&scope()).len(), 3);
}
