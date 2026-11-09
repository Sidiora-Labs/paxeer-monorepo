//! Bounded proofs for audit 22.30.96; mapping and wiring live in
//! src/repair_22_33_1541.kvx. All peers are local; the crash test reexecutes
//! this already-built test binary, with no external tool or service.
use layerx_client::ai_market::{AiClientError, OperationJournal, OperationRecord, OperationState};
use layerx_client::frame::framing::{read_frame, write_frame};
use layerx_client::px::PxEndpoint;
use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
use layerx_client::subscription::{self, SubscriptionTopic, WaitOptions};
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_types::payload::ModuleRegistry;
use serde_json::{json, Value};
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{mpsc, Arc, Barrier};
use std::time::{Duration, Instant};
use tungstenite::Message;

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}
fn registry() -> ModuleRegistry {
    must(ModuleRegistry::new(&[]))
}
fn signed(nonce: u64) -> OperationRecord {
    let raw = must(
        Eip1559Call {
            chain_id: 125,
            nonce,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 1,
            gas_limit: 90_000,
            to: [7; 20],
            data: vec![1, 2, 3],
        }
        .sign(&[0x46; 32]),
    );
    must(OperationRecord::from_signed_transaction(
        raw, [0x81; 32], 200,
    ))
}
fn hex(bytes: &[u8]) -> String {
    format!(
        "0x{}",
        bytes.iter().map(|b| format!("{b:02x}")).collect::<String>()
    )
}
struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-1541-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        must(fs::create_dir(&path));
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn read_request(stream: TcpStream) -> (TcpStream, Value) {
    must(stream.set_read_timeout(Some(Duration::from_secs(5))));
    must(stream.set_write_timeout(Some(Duration::from_secs(5))));
    let mut reader = BufReader::new(stream);
    let mut length = None;
    loop {
        let mut line = String::new();
        assert_ne!(must(reader.read_line(&mut line)), 0);
        if line == "\r\n" {
            break;
        }
        if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
            length = Some(must(value.trim().parse::<usize>()));
        }
    }
    let mut body = vec![0; length.expect("content length")];
    must(reader.read_exact(&mut body));
    (reader.into_inner(), must(serde_json::from_slice(&body)))
}
fn answer(mut stream: TcpStream, request: &Value, result: Value) {
    let body = json!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
    must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()));
    must(stream.flush());
}
fn listener() -> (TcpListener, String) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    (listener, url)
}

#[test]
fn journal_concurrent_opens_and_stale_writes() {
    let directory = Directory::new();
    let barrier = Arc::new(Barrier::new(8));
    let workers: Vec<_> = (0..8)
        .map(|_| {
            let path = directory.0.clone();
            let barrier = barrier.clone();
            std::thread::spawn(move || {
                let journal = must(OperationJournal::open(&path));
                let record = signed(4);
                barrier.wait();
                match journal.record_signed(&record) {
                    Ok(()) => true,
                    Err(AiClientError::Journal(std::io::ErrorKind::WouldBlock)) => false,
                    other => panic!("unexpected concurrent registration: {other:?}"),
                }
            })
        })
        .collect();
    let successes = workers
        .into_iter()
        .map(|worker| must(worker.join()))
        .filter(|success| *success)
        .count();
    assert!(successes >= 1);
    let journal = must(OperationJournal::open(&directory.0));
    let mut current = must(journal.load(&signed(4).activity_id(), &registry()));
    let stale = current.clone();
    assert_eq!(
        must(journal.expire(&mut current, 201)),
        OperationState::Failed
    );
    let terminal_bytes = must(fs::read(journal.record_path(&current.activity_id())));
    must(journal.record_signed(&stale));
    assert_eq!(journal.persist(&stale), Err(AiClientError::StaleRecord));
    let mut stale_mutation = stale.clone();
    assert_eq!(
        journal.expire(&mut stale_mutation, 201),
        Err(AiClientError::StaleRecord)
    );
    let conflict = must(OperationRecord::from_signed_transaction(
        stale.signed_bytes().to_vec(),
        [99; 32],
        200,
    ));
    assert_eq!(
        journal.record_signed(&conflict),
        Err(AiClientError::CorruptRecord)
    );
    drop(journal);
    let reopened = must(OperationJournal::open(&directory.0));
    let recovered = must(reopened.load(&current.activity_id(), &registry()));
    assert_eq!(recovered.state(), OperationState::Failed);
    assert_eq!(recovered.signed_bytes(), stale.signed_bytes());
    assert_eq!(
        must(fs::read(reopened.record_path(&current.activity_id()))),
        terminal_bytes
    );
    assert!(must(fs::read_dir(&directory.0)).all(|entry| must(entry)
        .path()
        .extension()
        .is_none_or(|ext| ext != "partial")));
}

struct CrashChild(Child);
impl Drop for CrashChild {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

#[test]
fn journal_recovers_only_after_process_owner_stops() {
    const CHILD_DIRECTORY: &str = "REPAIR_1541_CHILD_DIRECTORY";
    const CHILD_ENDPOINT: &str = "REPAIR_1541_CHILD_ENDPOINT";
    if let Some(path) = std::env::var_os(CHILD_DIRECTORY) {
        let journal = must(OperationJournal::open(&PathBuf::from(path)));
        let mut record = must(journal.load(&signed(5).activity_id(), &registry()));
        let endpoint = must(SequencerEndpoint::new(
            &must(std::env::var(CHILD_ENDPOINT)),
            &[],
        ));
        // The parent stops this process while the real journal owns Submitting
        // and the real HTTP client is waiting for the server's acknowledgment.
        let result = journal.submit(&mut record, &endpoint);
        panic!("child unexpectedly survived submission: {result:?}");
    }
    let directory = Directory::new();
    let journal = must(OperationJournal::open(&directory.0));
    let stale = signed(5);
    must(journal.record_signed(&stale));
    let (server, url) = listener();
    must(server.set_nonblocking(true));
    let mut child = CrashChild(must(
        Command::new(must(std::env::current_exe()))
            .args([
                "--exact",
                "journal_recovers_only_after_process_owner_stops",
                "--nocapture",
            ])
            .env(CHILD_DIRECTORY, &directory.0)
            .env(CHILD_ENDPOINT, &url)
            .stdout(Stdio::null())
            .stderr(Stdio::inherit())
            .spawn(),
    ));
    let deadline = Instant::now() + Duration::from_secs(10);
    let stream = loop {
        match server.accept() {
            Ok((stream, _)) => break stream,
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                assert!(Instant::now() < deadline, "child did not submit");
                assert!(
                    must(child.0.try_wait()).is_none(),
                    "child exited before transmission"
                );
                std::thread::sleep(Duration::from_millis(2));
            }
            Err(error) => panic!("{error}"),
        }
    };
    let (stream, request) = read_request(stream);
    assert_eq!(request["method"], "eth_sendRawTransaction");
    assert_eq!(request["params"], json!([hex(stale.signed_bytes())]));
    let bytes = must(fs::read(journal.record_path(&stale.activity_id())));
    let submitting = must(OperationRecord::decode(&bytes, &registry()));
    assert_eq!(submitting.state(), OperationState::Submitting);
    assert_eq!(submitting.attempt(), 1);
    let concurrent = must(OperationJournal::open(&directory.0));
    assert_eq!(
        concurrent.load(&stale.activity_id(), &registry()),
        Err(AiClientError::Journal(std::io::ErrorKind::WouldBlock))
    );
    assert_eq!(
        concurrent.record_signed(&stale),
        Err(AiClientError::Journal(std::io::ErrorKind::WouldBlock))
    );
    assert_eq!(
        concurrent.persist(&stale),
        Err(AiClientError::Journal(std::io::ErrorKind::WouldBlock))
    );
    assert_eq!(
        must(fs::read(journal.record_path(&stale.activity_id()))),
        bytes
    );
    must(child.0.kill());
    assert!(!must(child.0.wait()).success());
    drop(stream);
    drop(concurrent);
    drop(journal);
    let reopened = must(OperationJournal::open(&directory.0));
    let mut recovered = must(reopened.load(&stale.activity_id(), &registry()));
    assert_eq!(recovered.state(), OperationState::Unknown);
    assert_eq!(recovered.attempt(), 1);
    assert_eq!(recovered.signed_bytes(), stale.signed_bytes());
    assert_eq!(
        must(OperationRecord::decode(
            &must(fs::read(reopened.record_path(&stale.activity_id()))),
            &registry()
        ))
        .state(),
        OperationState::Unknown
    );
    must(reopened.record_signed(&stale));
    assert_eq!(reopened.persist(&stale), Err(AiClientError::StaleRecord));
    let (server, url) = listener();
    let expected = stale.clone();
    let worker = std::thread::spawn(move || {
        let (stream, _) = must(server.accept());
        let (stream, request) = read_request(stream);
        assert_eq!(request["params"], json!([hex(expected.signed_bytes())]));
        answer(stream, &request, json!(hex(&expected.activity_id())));
    });
    let endpoint = must(SequencerEndpoint::new(&url, &[]));
    assert_eq!(
        must(reopened.resend_exact(&mut recovered, &endpoint)),
        OperationState::Pending
    );
    assert_eq!(recovered.attempt(), 2);
    must(worker.join());
    drop(reopened);
    let reopened = must(OperationJournal::open(&directory.0));
    assert_eq!(
        must(reopened.load(&stale.activity_id(), &registry())),
        recovered
    );
}

#[test]
fn gateway_account_keeps_served_height_after_later_head() {
    // Committed current gateway producer contract, including its original height.
    // No reconstruction of the old, height-less wallet fixture is involved.
    let fixture: Value = must(serde_json::from_str(include_str!(
        "../../../../../sdk/wallet/test/fixtures/endpoint/merged-contract.json"
    )));
    assert_eq!(
        fixture["provenance"]["source"],
        "services/platform/hosted/gateway/tests/one_source_reads.rs"
    );
    let (server, url) = listener();
    let produced = fixture["account"].clone();
    let expected = produced.clone();
    let address = produced["paxeer"]["address"]
        .as_str()
        .expect("address")
        .to_owned();
    let worker = std::thread::spawn(move || {
        let (stream, _) = must(server.accept());
        let (stream, request) = read_request(stream);
        assert_eq!(request["method"], "px_getAccount");
        assert_eq!(request["params"], json!([address]));
        answer(stream, &request, produced);
        let (stream, _) = must(server.accept());
        let (stream, request) = read_request(stream);
        assert_eq!(
            request["method"], "eth_blockNumber",
            "account must not resample the head"
        );
        answer(stream, &request, json!("0x20"));
    });
    let endpoint = must(SequencerEndpoint::new(&url, &[]));
    let px = must(PxEndpoint::new(&url, &[]));
    let joined = must(px.account(expected["paxeer"]["address"].as_str().expect("address")));
    assert_eq!(must(endpoint.block_number()), 32);
    let account = joined.paxeer.expect("Paxeer half");
    assert_eq!(
        account.height,
        expected["paxeer"]["height"]
            .as_u64()
            .expect("served height")
    );
    assert_eq!(account.height, 16);
    assert_eq!(account.nonce, 42);
    assert_eq!(account.balance.decimal(), "81985529216486895");
    must(worker.join());
}

#[test]
fn gateway_account_rejects_missing_or_invalid_height() {
    let fixture: Value = must(serde_json::from_str(include_str!(
        "../../../../../sdk/wallet/test/fixtures/endpoint/merged-contract.json"
    )));
    let account = fixture["account"].clone();
    let address = account["paxeer"]["address"]
        .as_str()
        .expect("address")
        .to_owned();
    let (server, url) = listener();
    let worker = std::thread::spawn(move || {
        for height in [
            None,
            Some(Value::Null),
            Some(json!(-1)),
            Some(json!(1.5)),
            Some(json!("16")),
            Some(json!("0x10")),
            Some(must(serde_json::from_str::<Value>("18446744073709551616"))),
        ] {
            let mut bad = account.clone();
            match height {
                Some(value) => bad["paxeer"]["height"] = value,
                None => {
                    bad["paxeer"]
                        .as_object_mut()
                        .expect("Paxeer object")
                        .remove("height");
                }
            }
            let (stream, _) = must(server.accept());
            let (stream, request) = read_request(stream);
            assert_eq!(request["method"], "px_getAccount");
            answer(stream, &request, bad);
        }
    });
    let px = must(PxEndpoint::new(&url, &[]));
    for _ in 0..7 {
        assert_eq!(px.account(&address), Err(SequencerError::Malformed));
    }
    must(worker.join());
}

#[test]
fn retired_interface_uses_generic_frame_boundary() {
    let (server, url) = listener();
    let worker = std::thread::spawn(move || {
        let (mut stream, _) = must(server.accept());
        let payload = must(read_frame(&mut stream, 64));
        assert_eq!(payload, b"generic canonical bytes");
        must(write_frame(&mut stream, &payload, 64));
    });
    let address = url
        .strip_prefix("http://")
        .expect("scheme")
        .trim_end_matches('/');
    let mut stream = must(TcpStream::connect(address));
    must(stream.set_read_timeout(Some(Duration::from_secs(2))));
    must(write_frame(&mut stream, b"generic canonical bytes", 64));
    assert_eq!(
        must(read_frame(&mut stream, 64)),
        b"generic canonical bytes"
    );
    must(worker.join());
    let crate_root = include_str!("../src/lib.rs");
    assert!(!crate_root.contains("pub mod lni"));
    assert!(crate_root.contains("hosted gateway"));
    assert!(crate_root.contains("root JSON-RPC"));
}

#[derive(Clone, Copy)]
enum ControlPhase {
    Subscribe,
    Next,
    Close,
    CloseNotifications,
    Idle,
}
struct WebSocketPeer {
    url: String,
    stop: Arc<AtomicBool>,
    worker: Option<std::thread::JoinHandle<usize>>,
}
impl WebSocketPeer {
    fn start(phase: ControlPhase) -> Self {
        let (server, url) = listener();
        let url = url.replacen("http://", "ws://", 1);
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let worker = std::thread::spawn(move || {
            let (stream, _) = must(server.accept());
            must(stream.set_read_timeout(Some(Duration::from_secs(2))));
            must(stream.set_write_timeout(Some(Duration::from_millis(100))));
            let mut socket = must(tungstenite::accept(stream));
            let request = must(socket.read());
            let request: Value = must(serde_json::from_str(must(request.to_text())));
            assert_eq!(request["method"], "eth_subscribe");
            if !matches!(phase, ControlPhase::Subscribe) {
                must(
                    socket.send(Message::Text(
                        json!({"jsonrpc":"2.0","id":1,"result":"sub-1541"})
                            .to_string()
                            .into(),
                    )),
                );
            }
            if matches!(
                phase,
                ControlPhase::Close | ControlPhase::CloseNotifications
            ) {
                let request = must(socket.read());
                let request: Value = must(serde_json::from_str(must(request.to_text())));
                assert_eq!(request["method"], "eth_unsubscribe");
            }
            let mut count = 0;
            while !stopped.load(Ordering::Acquire) && count < 200 {
                if !matches!(phase, ControlPhase::Idle) {
                    let message = if matches!(phase, ControlPhase::CloseNotifications)
                        && count % 4 == 0
                    {
                        Message::Text(json!({"jsonrpc":"2.0","method":"eth_subscription","params":{"subscription":"sub-1541","result":{"number":"0x10"}}}).to_string().into())
                    } else if count % 2 == 0 {
                        Message::Ping(vec![1, 2].into())
                    } else {
                        Message::Pong(vec![3, 4].into())
                    };
                    if socket.send(message).is_err() {
                        break;
                    }
                }
                count += 1;
                std::thread::sleep(Duration::from_millis(10));
            }
            count
        });
        Self {
            url,
            stop,
            worker: Some(worker),
        }
    }
    fn finish(mut self) -> usize {
        self.stop.store(true, Ordering::Release);
        must(self.worker.take().expect("worker").join())
    }
}
impl Drop for WebSocketPeer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}
fn budget() -> WaitOptions {
    WaitOptions {
        timeout: Duration::from_millis(150),
        cancellation: None,
    }
}
fn assert_bounded(start: Instant) {
    assert!(start.elapsed() >= Duration::from_millis(100));
    assert!(
        start.elapsed() < Duration::from_secs(1),
        "operation reset its response deadline"
    );
}

struct HandshakePeer {
    url: String,
    release: mpsc::Sender<()>,
    worker: Option<std::thread::JoinHandle<()>>,
}
impl HandshakePeer {
    fn start(complete: bool, cancellation: Option<Arc<AtomicBool>>) -> Self {
        let (server, url) = listener();
        let url = url.replacen("http://", "ws://", 1);
        must(server.set_nonblocking(true));
        let (release, released) = mpsc::channel();
        let worker = std::thread::spawn(move || {
            let accept_deadline = Instant::now() + Duration::from_secs(2);
            let stream = loop {
                match server.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(
                            Instant::now() < accept_deadline,
                            "handshake never connected"
                        );
                        std::thread::sleep(Duration::from_millis(5));
                    }
                    Err(error) => panic!("accept: {error:?}"),
                }
            };
            must(stream.set_read_timeout(Some(Duration::from_secs(2))));
            must(stream.set_write_timeout(Some(Duration::from_secs(2))));
            let handshake = tungstenite::accept_hdr(
                stream,
                |request: &tungstenite::handshake::server::Request, response| {
                    assert_eq!(request.method(), "GET");
                    assert_eq!(request.uri().path(), "/");
                    if complete {
                        std::thread::sleep(Duration::from_millis(200));
                    } else {
                        if let Some(flag) = &cancellation {
                            std::thread::sleep(Duration::from_millis(150));
                            flag.store(true, Ordering::Release);
                        }
                        must(released.recv_timeout(Duration::from_secs(2)));
                    }
                    Ok(response)
                },
            );
            if complete {
                let mut socket = must(handshake);
                let request = must(socket.read());
                let request: Value = must(serde_json::from_str(must(request.to_text())));
                assert_eq!(
                    request,
                    json!({"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newHeads"]})
                );
                must(
                    socket.send(Message::Text(
                        json!({"jsonrpc":"2.0","id":1,"result":"delayed-sub"})
                            .to_string()
                            .into(),
                    )),
                );
                must(socket.send(Message::Text(json!({"jsonrpc":"2.0","method":"eth_subscription","params":{"subscription":"delayed-sub","result":{"number":"0x10"}}}).to_string().into())));
                let request = must(socket.read());
                let request: Value = must(serde_json::from_str(must(request.to_text())));
                assert_eq!(
                    request,
                    json!({"jsonrpc":"2.0","id":2,"method":"eth_unsubscribe","params":["delayed-sub"]})
                );
                must(
                    socket.send(Message::Text(
                        json!({"jsonrpc":"2.0","id":2,"result":true})
                            .to_string()
                            .into(),
                    )),
                );
                assert!(matches!(must(socket.read()), Message::Close(_)));
            } else {
                drop(handshake);
            }
            assert!(
                matches!(server.accept(), Err(error) if error.kind() == std::io::ErrorKind::WouldBlock)
            );
        });
        Self {
            url,
            release,
            worker: Some(worker),
        }
    }

    fn finish(mut self) {
        let _ = self.release.send(());
        must(self.worker.take().expect("worker").join());
    }
}
impl Drop for HandshakePeer {
    fn drop(&mut self) {
        let _ = self.release.send(());
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

#[test]
fn subscription_handshake_survives_cancellation_polls() {
    for cancellation in [None, Some(Arc::new(AtomicBool::new(false)))] {
        let peer = HandshakePeer::start(true, None);
        let options = WaitOptions {
            timeout: Duration::from_secs(2),
            cancellation: cancellation.clone(),
        };
        let start = Instant::now();
        let mut subscription = must(subscription::subscribe_with_options(
            &peer.url,
            &[],
            SubscriptionTopic::NewHeads,
            &options,
        ));
        assert!(start.elapsed() >= Duration::from_millis(150));
        assert_eq!(
            must(subscription.next_with_options(&options)),
            json!({"number":"0x10"})
        );
        must(subscription.close_with_options(&options));
        assert!(start.elapsed() < Duration::from_secs(2));
        if let Some(flag) = cancellation {
            assert!(!flag.load(Ordering::Acquire));
        }
        peer.finish();
    }
}

#[test]
fn subscription_handshake_can_be_cancelled() {
    let cancellation = Arc::new(AtomicBool::new(false));
    let peer = HandshakePeer::start(false, Some(cancellation.clone()));
    let options = WaitOptions {
        timeout: Duration::from_secs(2),
        cancellation: Some(cancellation.clone()),
    };
    let start = Instant::now();
    let result =
        subscription::subscribe_with_options(&peer.url, &[], SubscriptionTopic::NewHeads, &options);
    let elapsed = start.elapsed();
    assert!(cancellation.load(Ordering::Acquire));
    assert!(matches!(result, Err(SequencerError::Unavailable)));
    assert!(elapsed >= Duration::from_millis(150));
    assert!(elapsed < Duration::from_secs(1));
    peer.finish();
}

#[test]
fn subscription_handshake_honors_absolute_deadline() {
    let peer = HandshakePeer::start(false, None);
    let cancellation = Arc::new(AtomicBool::new(false));
    let options = WaitOptions {
        timeout: Duration::from_millis(300),
        cancellation: Some(cancellation.clone()),
    };
    let start = Instant::now();
    let result =
        subscription::subscribe_with_options(&peer.url, &[], SubscriptionTopic::NewHeads, &options);
    let elapsed = start.elapsed();
    assert!(matches!(result, Err(SequencerError::Unavailable)));
    assert!(!cancellation.load(Ordering::Acquire));
    assert!(elapsed >= Duration::from_millis(250));
    assert!(elapsed < Duration::from_secs(1));
    peer.finish();
}

#[test]
fn subscription_control_traffic_cannot_extend_subscribe_deadline() {
    let peer = WebSocketPeer::start(ControlPhase::Subscribe);
    let start = Instant::now();
    assert!(matches!(
        subscription::subscribe_with_options(
            &peer.url,
            &[],
            SubscriptionTopic::NewHeads,
            &budget()
        ),
        Err(SequencerError::Unavailable)
    ));
    assert_bounded(start);
    assert!(peer.finish() >= 2);
}

#[test]
fn subscription_control_traffic_cannot_extend_next_deadline() {
    let peer = WebSocketPeer::start(ControlPhase::Next);
    let mut subscription = must(subscription::subscribe(
        &peer.url,
        &[],
        SubscriptionTopic::NewHeads,
    ));
    let start = Instant::now();
    assert_eq!(
        subscription.next_with_options(&budget()),
        Err(SequencerError::Unavailable)
    );
    assert_bounded(start);
    drop(subscription);
    assert!(peer.finish() >= 2);
}

#[test]
fn subscription_control_and_notifications_share_close_deadline() {
    for phase in [ControlPhase::Close, ControlPhase::CloseNotifications] {
        let peer = WebSocketPeer::start(phase);
        let mut subscription = must(subscription::subscribe(
            &peer.url,
            &[],
            SubscriptionTopic::NewHeads,
        ));
        let start = Instant::now();
        assert_eq!(
            subscription.close_with_options(&budget()),
            Err(SequencerError::Unavailable)
        );
        assert_bounded(start);
        drop(subscription);
        assert!(peer.finish() >= 2);
    }
}

#[test]
fn subscription_wait_can_be_cancelled_during_control_traffic_or_idle() {
    for phase in [ControlPhase::Next, ControlPhase::Idle] {
        let peer = WebSocketPeer::start(phase);
        let mut subscription = must(subscription::subscribe(
            &peer.url,
            &[],
            SubscriptionTopic::NewHeads,
        ));
        let cancellation = Arc::new(AtomicBool::new(false));
        let flag = cancellation.clone();
        let (ready, started) = mpsc::channel();
        let canceller = std::thread::spawn(move || {
            must(started.recv());
            std::thread::sleep(Duration::from_millis(80));
            flag.store(true, Ordering::Release);
        });
        let start = Instant::now();
        must(ready.send(()));
        let options = WaitOptions {
            timeout: Duration::from_secs(5),
            cancellation: Some(cancellation.clone()),
        };
        assert_eq!(
            subscription.next_with_options(&options),
            Err(SequencerError::Unavailable)
        );
        assert!(cancellation.load(Ordering::Acquire));
        assert!(start.elapsed() < Duration::from_secs(1));
        must(canceller.join());
        drop(subscription);
        assert!(peer.finish() >= 2);
    }
}

#[test]
fn subscription_flushes_pong_and_delivers_real_notification() {
    let (server, url) = listener();
    let url = url.replacen("http://", "ws://", 1);
    let worker = std::thread::spawn(move || {
        let (stream, _) = must(server.accept());
        must(stream.set_read_timeout(Some(Duration::from_secs(2))));
        must(stream.set_write_timeout(Some(Duration::from_secs(2))));
        let mut socket = must(tungstenite::accept(stream));
        let request = must(socket.read());
        let request: Value = must(serde_json::from_str(must(request.to_text())));
        assert_eq!(request["method"], "eth_subscribe");
        must(
            socket.send(Message::Text(
                json!({"jsonrpc":"2.0","id":1,"result":"real-sub"})
                    .to_string()
                    .into(),
            )),
        );
        must(socket.send(Message::Ping(vec![9, 8, 7].into())));
        assert_eq!(must(socket.read()), Message::Pong(vec![9, 8, 7].into()));
        must(socket.send(Message::Text(json!({"jsonrpc":"2.0","method":"eth_subscription","params":{"subscription":"real-sub","result":{"number":"0x10"}}}).to_string().into())));
        let request = must(socket.read());
        let request: Value = must(serde_json::from_str(must(request.to_text())));
        assert_eq!(request["method"], "eth_unsubscribe");
        assert_eq!(request["params"], json!(["real-sub"]));
        must(
            socket.send(Message::Text(
                json!({"jsonrpc":"2.0","id":2,"result":true})
                    .to_string()
                    .into(),
            )),
        );
        assert!(matches!(must(socket.read()), Message::Close(_)));
    });
    let mut subscription = must(subscription::subscribe(
        &url,
        &[],
        SubscriptionTopic::NewHeads,
    ));
    let options = WaitOptions {
        timeout: Duration::from_secs(2),
        cancellation: None,
    };
    assert_eq!(
        must(subscription.next_with_options(&options)),
        json!({"number":"0x10"})
    );
    must(subscription.close_with_options(&options));
    must(worker.join());
}
