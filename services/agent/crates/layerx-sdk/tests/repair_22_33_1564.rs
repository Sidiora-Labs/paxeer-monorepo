//! Audit 22.30.99 regression proofs. Peer fixtures use loopback HTTP/TLS, a
//! recorded node receipt, an in-memory event log and explicitly supplied clocks.

use std::collections::{BTreeMap, VecDeque};
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::TcpListener;
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::JoinHandle;
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::signer::LocalSigner;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_sdk::production::{
    ResumableStream, StreamCursor, StreamEvent, StreamPage, STREAM_EVENT_ID_BYTES,
    STREAM_PAGE_EVENTS, STREAM_REPLAY_IDENTITIES,
};
use layerx_sdk::programs::{
    verify_signed_discovery, HttpProgramTransport, ProgramLifecycle, ProgramOperationError,
    ProgramTransport,
};
use layerx_sdk::rpc::{Commitment, RpcClient, RpcError};
use layerx_sdk::rpc_verification::ReceiptPolicy;
use layerx_sdk::wallet::Wallet;
use layerx_types::clock::{Clock, ClockError, ClockReading};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const RECORDED: &str = include_str!("../src/repair_22_31_227_fixture/committed.json");
const CA: &[u8] = include_bytes!("fixtures/repair_22_33_1564_fixture/ca.der");

fn hex(bytes: &[u8]) -> String {
    hex::encode(bytes)
}

fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    let text = value.as_str().unwrap_or_else(|| panic!("recorded hex"));
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len(), N * 2);
    let mut result = [0; N];
    for (byte, pair) in result.iter_mut().zip(text.as_bytes().chunks_exact(2)) {
        *byte = u8::from_str_radix(
            std::str::from_utf8(pair).unwrap_or_else(|error| panic!("hex: {error:?}")),
            16,
        )
        .unwrap_or_else(|error| panic!("recorded byte: {error:?}"));
    }
    result
}

trait PeerIo: std::io::Read + std::io::Write {}
impl<T: std::io::Read + std::io::Write> PeerIo for T {}

struct Server {
    endpoint: String,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
    requests: Arc<Mutex<Vec<(String, Value)>>>,
}

impl Server {
    fn open(tls: bool, handler: impl Fn(&Value) -> Value + Send + 'static) -> Self {
        let config = tls.then(|| {
            use rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer};
            Arc::new(
                rustls::ServerConfig::builder_with_provider(Arc::new(
                    rustls::crypto::ring::default_provider(),
                ))
                .with_safe_default_protocol_versions()
                .unwrap_or_else(|error| panic!("TLS versions: {error:?}"))
                .with_no_client_auth()
                .with_single_cert(
                    vec![CertificateDer::from(
                        include_bytes!("fixtures/repair_22_31_227_fixture/server.der").to_vec(),
                    )],
                    PrivateKeyDer::Pkcs8(PrivatePkcs8KeyDer::from(
                        include_bytes!("fixtures/repair_22_31_227_fixture/server-key.der").to_vec(),
                    )),
                )
                .unwrap_or_else(|error| panic!("committed test certificate: {error:?}")),
            )
        });
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("loopback listener: {error:?}"));
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("nonblocking listener: {error:?}"));
        let endpoint = format!(
            "{}://{}",
            if tls { "https" } else { "http" },
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("peer address: {error:?}"))
        );
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let requests = Arc::new(Mutex::new(Vec::new()));
        let received = requests.clone();
        let worker = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let (socket, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("peer accept: {error}"),
                };
                socket
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .unwrap_or_else(|error| panic!("read bound: {error:?}"));
                socket
                    .set_write_timeout(Some(Duration::from_secs(3)))
                    .unwrap_or_else(|error| panic!("write bound: {error:?}"));
                let stream: Box<dyn PeerIo> = match &config {
                    Some(config) => Box::new(rustls::StreamOwned::new(
                        rustls::ServerConnection::new(config.clone())
                            .unwrap_or_else(|error| panic!("TLS peer: {error:?}")),
                        socket,
                    )),
                    None => Box::new(socket),
                };
                let PeerRequest {
                    reader,
                    path,
                    request,
                } = read_peer_request(stream);
                received
                    .lock()
                    .unwrap_or_else(|error| panic!("request log: {error:?}"))
                    .push((path, request.clone()));
                let result = handler(&request);
                let bytes = serde_json::to_vec(&json!({
                    "jsonrpc":"2.0", "id":request["id"], "result": result
                }))
                .unwrap_or_else(|error| panic!("JSON response: {error:?}"));
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", bytes.len()).unwrap_or_else(|error| panic!("response headers: {error:?}"));
                stream
                    .write_all(&bytes)
                    .unwrap_or_else(|error| panic!("response body: {error:?}"));
                stream
                    .flush()
                    .unwrap_or_else(|error| panic!("response flush: {error:?}"));
            }
        });
        Self {
            endpoint,
            stop,
            worker: Some(worker),
            requests,
        }
    }

    fn requests(&self) -> Vec<(String, Value)> {
        self.requests
            .lock()
            .unwrap_or_else(|error| panic!("request log: {error:?}"))
            .clone()
    }
}

impl Drop for Server {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let result = worker.join();
            if !std::thread::panicking() {
                result.unwrap_or_else(|error| panic!("peer worker: {error:?}"));
            }
        }
    }
}

// An explicit protocol clock transcript is a peer of the production deadline,
// not a replacement for Wallet, RpcClient or Deadline.
struct TranscriptClock {
    readings: Mutex<VecDeque<Result<ClockReading, ClockError>>>,
    budgets: Mutex<Vec<Duration>>,
}

impl TranscriptClock {
    fn readings(values: impl IntoIterator<Item = Result<ClockReading, ClockError>>) -> Arc<Self> {
        Arc::new(Self {
            readings: Mutex::new(values.into_iter().collect()),
            budgets: Mutex::new(vec![]),
        })
    }

    fn advancing() -> Arc<Self> {
        Self::readings((0..1000).map(|n| Ok(reading(n * 10_000_000))))
    }
}

fn reading(nanos: u64) -> ClockReading {
    ClockReading {
        generation: [1; 16],
        unix_milliseconds: 1_000 + nanos / 1_000_000,
        monotonic_nanoseconds: nanos,
    }
}

impl Clock for TranscriptClock {
    fn sample(&self, budget: Duration) -> Result<ClockReading, ClockError> {
        assert!(!budget.is_zero());
        self.budgets
            .lock()
            .unwrap_or_else(|error| panic!("clock budgets: {error:?}"))
            .push(budget);
        self.readings
            .lock()
            .unwrap_or_else(|error| panic!("clock transcript: {error:?}"))
            .pop_front()
            .unwrap_or_else(|| panic!("bounded clock samples"))
    }
}

fn recorded_policy(first: u64, last: u64) -> ReceiptPolicy {
    let recorded: Value = serde_json::from_str(RECORDED)
        .unwrap_or_else(|error| panic!("recorded node transcript: {error:?}"));
    ReceiptPolicy {
        protocol_version: 2,
        network_id: 125,
        sequencer: SequencerAuthorization::new(
            fixed(&recorded["pinnedKey"]["keyId"]),
            fixed(&recorded["pinnedKey"]["publicKey"]),
            first,
            last,
        ),
        trusted_checkpoint_context_digest: None,
    }
}

#[test]
fn default_unified_route_uses_exact_node_and_hosted_paths_including_private_ca() {
    for tls in [false, true] {
        let peer = Server::open(tls, |request| {
            assert_eq!(request["method"], "eth_getTransactionCount");
            json!("0x7")
        });
        let root = if tls {
            RpcClient::connect_with_ca_der(&peer.endpoint, None, CA)
        } else {
            RpcClient::connect(&peer.endpoint, None)
        }
        .unwrap_or_else(|error| panic!("root client: {error:?}"));
        assert_eq!(
            root.get_identity_sequence(&format!("0x{}", "11".repeat(20)))
                .unwrap_or_else(|error| panic!("root RPC: {error:?}")),
            7
        );
        let exact = format!("{}/published/agent", peer.endpoint);
        let hosted = if tls {
            RpcClient::connect_with_ca_der(&exact, None, CA)
        } else {
            RpcClient::connect_hosted(&exact, None)
        }
        .unwrap_or_else(|error| panic!("explicit hosted client: {error:?}"));
        assert_eq!(
            hosted
                .get_identity_sequence(&format!("0x{}", "11".repeat(20)))
                .unwrap_or_else(|error| panic!("explicit path RPC: {error:?}")),
            7
        );
        let node = if tls {
            RpcClient::connect_node_with_ca_der(&peer.endpoint, None, CA)
        } else {
            RpcClient::connect_node(&peer.endpoint, None)
        }
        .unwrap_or_else(|error| panic!("explicit node client: {error:?}"));
        assert_eq!(
            node.get_identity_sequence(&format!("0x{}", "11".repeat(20)))
                .unwrap_or_else(|error| panic!("node RPC: {error:?}")),
            7
        );
        let paths: Vec<_> = peer.requests().into_iter().map(|(path, _)| path).collect();
        assert_eq!(paths, ["/", "/published/agent", "/"]);
    }
    for endpoint in [
        "http://example.com",
        "ftp://localhost",
        "http://user:password@localhost",
        "http://localhost/?query",
        "http://localhost/#fragment",
    ] {
        assert!(RpcClient::connect(endpoint, None).is_err(), "{endpoint}");
        assert!(
            RpcClient::connect_node(endpoint, None).is_err(),
            "{endpoint}"
        );
        assert!(
            RpcClient::connect_hosted(endpoint, None).is_err(),
            "{endpoint}"
        );
        assert!(
            RpcClient::connect_with_ca_der(endpoint, None, CA).is_err(),
            "{endpoint}"
        );
    }
    assert!(RpcClient::connect_with_ca_der("https://localhost", None, &[]).is_err());
    assert!(RpcClient::connect_with_ca_der("https://localhost", None, &[0; 10]).is_err());
}

fn cursor(position: u64) -> StreamCursor {
    StreamCursor::new(format!("{position:016x}{:08x}{:08x}", 0, 0))
        .unwrap_or_else(|error| panic!("producer cursor: {error:?}"))
}

struct EventLog(BTreeMap<u64, String>);
impl EventLog {
    fn page(&self, after: u64, count: usize) -> StreamPage<u64> {
        let mut previous = cursor(after);
        let events = self
            .0
            .range((after + 1)..)
            .take(count)
            .map(|(position, id)| {
                let event = StreamEvent {
                    event_id: id.clone(),
                    previous_cursor: previous.clone(),
                    cursor: cursor(*position),
                    value: *position,
                };
                previous = event.cursor.clone();
                event
            })
            .collect();
        StreamPage {
            requested_cursor: cursor(after),
            events,
            next_cursor: previous,
        }
    }
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-1564-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::create_dir(&path).unwrap_or_else(|error| panic!("new test directory: {error:?}"));
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.0)
            .unwrap_or_else(|error| panic!("remove test directory: {error:?}"));
    }
}

#[test]
fn unbounded_stream_identities_are_bounded_atomic_and_resume_from_persisted_cursor() {
    let total = u64::try_from(STREAM_REPLAY_IDENTITIES * 8)
        .unwrap_or_else(|error| panic!("test history size: {error:?}"));
    let log = EventLog((1..=total).map(|n| (n, format!("event-{n}"))).collect());
    let mut stream = ResumableStream::new(cursor(0));
    let mut after = 0;
    while after < total / 2 {
        let page = log.page(after, 64);
        let events = stream
            .accept(page.clone())
            .unwrap_or_else(|error| panic!("producer page: {error:?}"));
        assert_eq!(events.len(), 64);
        after = events
            .last()
            .unwrap_or_else(|| panic!("nonempty page"))
            .value;
        assert_eq!(
            stream.retained_identities(),
            (usize::try_from(after)
                .unwrap_or_else(|error| panic!("fixture integer must fit usize: {error:?}")))
            .min(STREAM_REPLAY_IDENTITIES)
        );
        let before = stream.clone();
        assert!(stream.accept(page).is_err(), "replayed page");
        assert_eq!(stream, before);
    }
    let mut repeated_id = log.page(after, 1);
    repeated_id.events[0].event_id = format!("event-{after}");
    let before = stream.clone();
    assert!(
        stream.accept(repeated_id).is_err(),
        "recent identity at a new cursor"
    );
    assert_eq!(stream, before);
    let mut within_page = log.page(after, 2);
    within_page.events[1].event_id = within_page.events[0].event_id.clone();
    assert!(stream.accept(within_page).is_err());
    assert_eq!(stream, before);
    let mut invalid_tail = log.page(after, 2);
    invalid_tail.next_cursor = cursor(after + 3);
    assert!(stream.accept(invalid_tail).is_err());
    assert_eq!(stream, before);
    let mut overlong = log.page(after, 1);
    overlong.events[0].event_id = "x".repeat(STREAM_EVENT_ID_BYTES + 1);
    assert!(stream.accept(overlong).is_err());
    assert!(stream
        .accept(log.page(after, STREAM_PAGE_EVENTS + 1))
        .is_err());
    assert_eq!(stream, before);

    let directory = Directory::new();
    let path = directory.0.join("cursor");
    std::fs::write(&path, stream.cursor().as_str())
        .unwrap_or_else(|error| panic!("persist processed cursor: {error:?}"));
    drop(stream);
    let persisted = StreamCursor::new(
        std::fs::read_to_string(&path)
            .unwrap_or_else(|error| panic!("reopen cursor store: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("durable cursor: {error:?}"));
    let mut stream = ResumableStream::new(persisted);
    assert_eq!(stream.retained_identities(), 0);
    assert!(
        stream.accept(log.page(0, 1)).is_err(),
        "replay after recovery"
    );
    let mut old_position = log.page(after, 1);
    old_position.events[0].cursor = cursor(1);
    old_position.next_cursor = cursor(1);
    assert!(
        stream.accept(old_position).is_err(),
        "cursor cannot regress after recovery"
    );
    while after < total {
        let events = stream
            .accept(log.page(after, 64))
            .unwrap_or_else(|error| panic!("resume strictly after durable cursor: {error:?}"));
        assert_eq!(events[0].value, after + 1);
        after = events
            .last()
            .unwrap_or_else(|| panic!("nonempty page"))
            .value;
        assert!(stream.retained_identities() <= STREAM_REPLAY_IDENTITIES);
    }
    assert_eq!(stream.cursor(), &cursor(total));
    assert_eq!(stream.retained_identities(), STREAM_REPLAY_IDENTITIES);
    let empty = log.page(total, 1);
    assert!(stream
        .accept(empty)
        .unwrap_or_else(|error| panic!("idle page: {error:?}"))
        .is_empty());
    assert_eq!(stream.cursor(), &cursor(total));

    assert_stream_boundary(&log);
}

fn registry_document(abi: u16, lifecycle: &str, tag: u8, sequence: u64) -> (Value, [u8; 32]) {
    let key = SigningKey::from_bytes(&[0x31; 32]);
    // Registry producer's v1 layout, with lifecycle discriminants 1/2/3.
    let mut material = b"LayerX/program-discovery-proof/v1\0".to_vec();
    material.extend_from_slice(&[0x11; 32]);
    material.push(tag);
    material.extend_from_slice(&3_u32.to_be_bytes());
    material.extend_from_slice(&[0x22; 32]);
    material.extend_from_slice(&abi.to_be_bytes());
    material.extend_from_slice(&sequence.to_be_bytes());
    material.extend_from_slice(&1000_u64.to_be_bytes());
    material.extend_from_slice(&2000_u64.to_be_bytes());
    material.extend_from_slice(&[0x33; 32]);
    let digest: [u8; 32] = Sha256::digest(&material).into();
    (
        json!({
            "program_id":hex(&[0x11;32]), "lifecycle":lifecycle, "version":3,
            "code_hash":hex(&[0x22;32]), "abi_version":abi, "receipt_digest":hex(&digest),
            "state_root":hex(&[0x33;32]), "observed_sequence":sequence.to_string(),
            "observed_at":"1000", "valid_through":"2000",
            "verification":"registry-receipt-and-current-head-verified",
            "discovery_public_key":hex(key.verifying_key().as_bytes()),
            "discovery_signature":hex(&key.sign(&digest).to_bytes())
        }),
        key.verifying_key().to_bytes(),
    )
}

#[test]
fn discovery_lifecycle_binding_authenticates_transitions_and_every_supported_profile() {
    for abi in 1..=layerx_types::guest_abi::MAX_VERSION {
        for (sequence, (lifecycle, tag, expected)) in [
            ("active", 1, ProgramLifecycle::Active),
            ("deprecated", 2, ProgramLifecycle::Deprecated),
            ("tombstoned", 3, ProgramLifecycle::Tombstoned),
        ]
        .into_iter()
        .enumerate()
        {
            let (value, key) = registry_document(
                abi,
                lifecycle,
                tag,
                u64::try_from(sequence)
                    .unwrap_or_else(|error| panic!("fixture integer must fit u64: {error:?}"))
                    + 10,
            );
            // Cross the JSON codec just as a recorded registry response does.
            let encoded = serde_json::to_vec(&value)
                .unwrap_or_else(|error| panic!("producer document: {error:?}"));
            let transported = serde_json::from_slice(&encoded)
                .unwrap_or_else(|error| panic!("transport JSON: {error:?}"));
            let verified = verify_signed_discovery(&transported, [0x11; 32], 1500, key)
                .unwrap_or_else(|error| panic!("signed lifecycle: {error:?}"));
            assert_eq!(verified.lifecycle(), expected);
            assert_eq!(verified.abi_version(), abi);
            assert_eq!(
                verified.observed_sequence(),
                u64::try_from(sequence)
                    .unwrap_or_else(|error| panic!("fixture integer must fit u64: {error:?}"))
                    + 10
            );
            for other in ["active", "deprecated", "tombstoned"]
                .into_iter()
                .filter(|other| *other != lifecycle)
            {
                let mut changed = value.clone();
                changed["lifecycle"] = json!(other);
                assert!(verify_signed_discovery(&changed, [0x11; 32], 1500, key).is_err());
            }
            for (field, changed) in [
                ("program_id", json!(hex(&[0x44; 32]))),
                ("version", json!(4)),
                ("code_hash", json!(hex(&[0x44; 32]))),
                ("abi_version", json!(if abi == 1 { 2 } else { 1 })),
                ("receipt_digest", json!(hex(&[0x44; 32]))),
                ("state_root", json!(hex(&[0x44; 32]))),
                ("observed_sequence", json!("100")),
                ("observed_at", json!("1001")),
                ("valid_through", json!("2001")),
                ("discovery_public_key", json!(hex(&[0x44; 32]))),
                ("discovery_signature", json!(hex(&[0; 64]))),
            ] {
                let mut altered = value.clone();
                altered[field] = changed;
                assert!(
                    verify_signed_discovery(&altered, [0x11; 32], 1500, key).is_err(),
                    "ABI {abi} field {field}"
                );
            }
            for field in ["discovery_public_key", "discovery_signature"] {
                let mut unsigned = value.clone();
                unsigned
                    .as_object_mut()
                    .unwrap_or_else(|| panic!("document"))
                    .remove(field);
                assert!(
                    verify_signed_discovery(&unsigned, [0x11; 32], 1500, key).is_err(),
                    "ABI {abi} unsigned {field}"
                );
            }
            assert!(verify_signed_discovery(&value, [0x11; 32], 1500, [0x55; 32]).is_err());
            assert!(verify_signed_discovery(&value, [0x11; 32], 999, key).is_err());
            assert!(verify_signed_discovery(&value, [0x11; 32], 2001, key).is_err());
        }
    }
}

#[test]
fn discovery_live_node_read_requires_independent_state_authority() {
    let peer = Server::open(false, |request| {
        match request["method"]
            .as_str()
            .unwrap_or_else(|| panic!("method"))
        {
            "eth_chainId" => json!("0x7d"),
            "eth_getBlockByNumber" => {
                json!({"number":"0xa", "hash":format!("0x{}", "12".repeat(32))})
            }
            "eth_call" => json!("0x"),
            method => panic!("unexpected live read {method}"),
        }
    });
    let client = HttpProgramTransport::connect_node_with_policy(
        &peer.endpoint,
        None,
        recorded_policy(10, 10),
        || Ok(1500),
    )
    .unwrap_or_else(|error| panic!("node trust: {error:?}"));
    assert!(matches!(
        client.discover([0x11; 32]),
        Err(ProgramOperationError::MissingNodeReadAuthority)
    ));
    let requests = peer.requests();
    assert_eq!(requests.len(), 5);
    assert!(requests.iter().all(|(path, _)| path == "/"));
    assert_eq!(requests[3].1["method"], "eth_call");
    assert_eq!(requests[3].1["params"][1], "0xa");
}

#[test]
fn receipt_key_range_checks_same_real_receipt_at_boundaries_and_after_rotation() {
    let recorded: Value = serde_json::from_str(RECORDED)
        .unwrap_or_else(|error| panic!("recorded node transcript: {error:?}"));
    let receipt = recorded["receiptJson"].clone();
    let activity = fixed(&receipt["txHash"]);
    let peer = Server::open(false, move |request| {
        match request["method"]
            .as_str()
            .unwrap_or_else(|| panic!("method"))
        {
            "eth_chainId" => json!("0x7d"),
            "pax_sequencerReceipt" => receipt.clone(),
            method => panic!("unexpected receipt request {method}"),
        }
    });
    let rpc = RpcClient::connect(&peer.endpoint, None)
        .unwrap_or_else(|error| panic!("node RPC: {error:?}"))
        .with_clock(TranscriptClock::advancing());
    for (first, last) in [(10, 10), (1, 10), (10, u64::MAX)] {
        let verified = rpc
            .wait_for(
                activity,
                Commitment::Receipted,
                &recorded_policy(first, last),
                Duration::from_secs(1),
            )
            .unwrap_or_else(|error| panic!("authorized receipt height: {error:?}"));
        assert_eq!(
            verified
                .receipt()
                .protocol()
                .unwrap_or_else(|| panic!("protocol receipt"))
                .activity_id(),
            activity
        );
    }
    for (first, last) in [(1, 9), (11, u64::MAX), (11, 9)] {
        assert!(
            matches!(
                rpc.wait_for(
                    activity,
                    Commitment::Receipted,
                    &recorded_policy(first, last),
                    Duration::from_secs(1)
                ),
                Err(RpcError::Verification)
            ),
            "range {first}..{last}"
        );
    }
    let key = SigningKey::from_bytes(&[0x66; 32]);
    let rotated = ReceiptPolicy {
        sequencer: SequencerAuthorization::new(
            [0x77; 32],
            key.verifying_key().to_bytes(),
            11,
            u64::MAX,
        ),
        ..recorded_policy(10, 10)
    };
    assert!(matches!(
        rpc.wait_for(
            activity,
            Commitment::Receipted,
            &rotated,
            Duration::from_secs(1)
        ),
        Err(RpcError::Verification)
    ));
    assert_wrong_receipt_authorities(&rpc, &recorded, activity, &key);
    // Historical authority remains usable after configuring the next epoch separately.
    let verified = rpc
        .wait_for(
            activity,
            Commitment::Receipted,
            &recorded_policy(10, 10),
            Duration::from_secs(1),
        )
        .unwrap_or_else(|error| panic!("retained historical pin: {error:?}"));
    assert_eq!(
        verified.canonical_bytes(),
        fixed_bytes(&recorded["receiptJson"]["receipt"])
    );
    assert!(peer.requests().iter().all(|(path, _)| path == "/"));
}

fn fixed_bytes(value: &Value) -> Vec<u8> {
    let text = value
        .as_str()
        .unwrap_or_else(|| panic!("hex bytes"))
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("node hex"));
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(
                std::str::from_utf8(pair).unwrap_or_else(|error| panic!("hex: {error:?}")),
                16,
            )
            .unwrap_or_else(|error| panic!("byte: {error:?}"))
        })
        .collect()
}

#[test]
fn wallet_ambient_clock_is_replaced_by_injected_deadline_and_bounded_polling() {
    let peer = Server::open(false, |request| {
        match request["method"]
            .as_str()
            .unwrap_or_else(|| panic!("method"))
        {
            "eth_chainId" => json!("0x7d"),
            "pax_sequencerReceipt" => Value::Null,
            method => panic!("unexpected poll {method}"),
        }
    });
    let account =
        derive_from_seed(&[0x42; 32], 0).unwrap_or_else(|error| panic!("test wallet: {error:?}"));
    let signer = LocalSigner::from_derived_account(&account);
    let policy = recorded_policy(10, 10);
    let clock = TranscriptClock::advancing();
    let rpc = RpcClient::connect(&peer.endpoint, None)
        .unwrap_or_else(|error| panic!("RPC: {error:?}"))
        .with_clock(clock.clone());
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [1; 32],
    };
    let activity = [0x11; 32];
    assert!(
        matches!(wallet.wait_for(activity, Commitment::Receipted, Duration::ZERO), Err(RpcError::Pending { activity_id }) if activity_id == activity)
    );
    assert!(matches!(
        wallet.wait_for(activity, Commitment::Receipted, Duration::from_secs(301)),
        Err(RpcError::InvalidRequest)
    ));
    assert!(clock
        .budgets
        .lock()
        .unwrap_or_else(|error| panic!("clock budgets: {error:?}"))
        .is_empty());
    assert!(peer.requests().is_empty());
    assert!(
        matches!(wallet.wait_for(activity, Commitment::Receipted, Duration::from_millis(120)), Err(RpcError::Pending { activity_id }) if activity_id == activity)
    );
    let requests = peer.requests();
    assert_eq!(requests[0].1["method"], "eth_chainId");
    let polls = requests
        .iter()
        .filter(|(_, request)| request["method"] == "pax_sequencerReceipt")
        .count();
    assert!((1..=2).contains(&polls), "bounded polls: {polls}");
    let budgets = clock
        .budgets
        .lock()
        .unwrap_or_else(|error| panic!("clock budgets: {error:?}"));
    assert_eq!(
        budgets.len(),
        13,
        "deadline follows injected transcript to 120ms"
    );
    assert!(budgets
        .iter()
        .all(|budget| *budget <= Duration::from_millis(120)));
}

#[test]
fn wallet_clock_unavailability_regression_and_late_success_are_refused() {
    let account =
        derive_from_seed(&[0x42; 32], 0).unwrap_or_else(|error| panic!("test wallet: {error:?}"));
    let signer = LocalSigner::from_derived_account(&account);
    let policy = recorded_policy(10, 10);
    let rpc = RpcClient::connect("http://127.0.0.1:1", None)
        .unwrap_or_else(|error| panic!("configuration: {error:?}"));
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [1; 32],
    };
    assert!(matches!(
        wallet.wait_for([1; 32], Commitment::Receipted, Duration::from_secs(1)),
        Err(RpcError::Clock(ClockError::Unavailable))
    ));
    for (samples, expected) in [
        (vec![Err(ClockError::Unavailable)], ClockError::Unavailable),
        (
            vec![Ok(reading(10)), Ok(reading(9))],
            ClockError::Regression,
        ),
        (
            vec![
                Ok(reading(10)),
                Ok(ClockReading {
                    generation: [2; 16],
                    ..reading(11)
                }),
            ],
            ClockError::Regression,
        ),
    ] {
        let rpc = RpcClient::connect("http://127.0.0.1:1", None)
            .unwrap_or_else(|error| panic!("configuration: {error:?}"))
            .with_clock(TranscriptClock::readings(samples));
        let wallet = Wallet {
            rpc: &rpc,
            signer: &signer,
            policy: &policy,
            native_asset: [1; 32],
        };
        assert!(
            matches!(wallet.wait_for([1;32], Commitment::Receipted, Duration::from_secs(1)), Err(RpcError::Clock(error)) if error == expected)
        );
    }
    let recorded: Value =
        serde_json::from_str(RECORDED).unwrap_or_else(|error| panic!("recorded node: {error:?}"));
    let activity = fixed(&recorded["receiptJson"]["txHash"]);
    let value = recorded["receiptJson"].clone();
    let peer = Server::open(false, move |request| {
        match request["method"]
            .as_str()
            .unwrap_or_else(|| panic!("method"))
        {
            "eth_chainId" => json!("0x7d"),
            "pax_sequencerReceipt" => value.clone(),
            method => panic!("unexpected request {method}"),
        }
    });
    // Start, chain pre/post, loop, receipt pre/post: valid receipt arrives after expiry.
    let clock = TranscriptClock::readings(
        [0, 1, 2, 3, 4, 100]
            .into_iter()
            .map(|ms| Ok(reading(ms * 1_000_000))),
    );
    let rpc = RpcClient::connect(&peer.endpoint, None)
        .unwrap_or_else(|error| panic!("RPC: {error:?}"))
        .with_clock(clock);
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [1; 32],
    };
    assert!(
        matches!(wallet.wait_for(activity, Commitment::Receipted, Duration::from_millis(100)), Err(RpcError::Pending { activity_id }) if activity_id == activity)
    );
    assert_eq!(
        peer.requests().len(),
        2,
        "late response cannot start another poll"
    );
}

struct PeerRequest {
    reader: BufReader<Box<dyn PeerIo>>,
    path: String,
    request: Value,
}
fn read_peer_request(stream: Box<dyn PeerIo>) -> PeerRequest {
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    reader
        .read_line(&mut line)
        .unwrap_or_else(|error| panic!("request line: {error:?}"));
    assert!(line.starts_with("POST "), "{line}");
    let path = line
        .split_whitespace()
        .nth(1)
        .unwrap_or_else(|| panic!("request path"))
        .to_owned();
    let mut length = None;
    let mut header_bytes = line.len();
    loop {
        line.clear();
        assert!(
            reader
                .read_line(&mut line)
                .unwrap_or_else(|error| panic!("HTTP header: {error:?}"))
                > 0
        );
        header_bytes += line.len();
        assert!(header_bytes <= 8192);
        if line == "\r\n" {
            break;
        }
        if let Some((name, value)) = line.split_once(':') {
            if name.eq_ignore_ascii_case("content-length") {
                length = Some(
                    value
                        .trim()
                        .parse::<usize>()
                        .unwrap_or_else(|error| panic!("content length: {error:?}")),
                );
            }
        }
    }
    let length = length.unwrap_or_else(|| panic!("length-delimited request"));
    assert!(length <= 1_048_576);
    let mut bytes = vec![0; length];
    reader
        .read_exact(&mut bytes)
        .unwrap_or_else(|error| panic!("request body: {error:?}"));
    let request: Value =
        serde_json::from_slice(&bytes).unwrap_or_else(|error| panic!("JSON request: {error:?}"));
    PeerRequest {
        reader,
        path,
        request,
    }
}

fn assert_stream_boundary(log: &EventLog) {
    let mut boundary = ResumableStream::new(cursor(0));
    let mut page = log.page(0, STREAM_PAGE_EVENTS);
    page.events[0].event_id = "x".repeat(STREAM_EVENT_ID_BYTES);
    assert_eq!(
        boundary
            .accept(page)
            .unwrap_or_else(|error| panic!("inclusive bounds: {error:?}"))
            .len(),
        STREAM_PAGE_EVENTS
    );
    assert_eq!(boundary.retained_identities(), STREAM_REPLAY_IDENTITIES);
}

fn assert_wrong_receipt_authorities(
    rpc: &RpcClient,
    recorded: &Value,
    activity: [u8; 32],
    key: &SigningKey,
) {
    let wrong_identity = ReceiptPolicy {
        sequencer: SequencerAuthorization::new(
            [0x77; 32],
            fixed(&recorded["pinnedKey"]["publicKey"]),
            10,
            10,
        ),
        ..recorded_policy(10, 10)
    };
    assert!(matches!(
        rpc.wait_for(
            activity,
            Commitment::Receipted,
            &wrong_identity,
            Duration::from_secs(1)
        ),
        Err(RpcError::Verification)
    ));
    let wrong_key = ReceiptPolicy {
        sequencer: SequencerAuthorization::new(
            fixed(&recorded["pinnedKey"]["keyId"]),
            key.verifying_key().to_bytes(),
            10,
            10,
        ),
        ..recorded_policy(10, 10)
    };
    assert!(matches!(
        rpc.wait_for(
            activity,
            Commitment::Receipted,
            &wrong_key,
            Duration::from_secs(1)
        ),
        Err(RpcError::Verification)
    ));
}
