//! Task 22.33.1487 / audit 22.30.88 repair record.
//! unused-custody-cadence -> custody_cadence_controls_real_pending_reconciliation.
//! listener-timeouts -> slow_tls_releases_permits_and_serves_healthy_clients,
//!                      slow_reader_releases_permits_and_serves_healthy_clients.
//! order-purpose-idempotency -> order_purpose_replay_preserves_the_http_binding.
//! Baseline: not run; the explicit implementation-first, hf-build/hf-done-only
//! gate order supersedes do_7's conflicting unchanged-tree verification request.
//! No pre-change test failures are claimed. No out-of-path wiring is required.
//! Initial qualification: five tests passed; the slow-reader case failed because
//! the TLS backend translated the expired write deadline into unexpected EOF.
//! The response writer now preserves the deadline's TimedOut classification;
//! the original error remains its cause and every test assertion is unchanged.
//! Qualification revision and private evidence are recorded by hf-done's ledger.
//! TLS fixture: committed localhost test identity, valid 2020-2100; public test
//! password only. Tests neither invoke openssl nor read any runtime environment.

use std::fs;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::sync::{
    atomic::{AtomicBool, AtomicUsize, Ordering},
    mpsc, Arc,
};
use std::thread;
use std::time::{Duration, Instant};

use layerx_ramp_toolkit::CreateOrder;
use native_tls::{Certificate, Identity, TlsAcceptor, TlsConnector, TlsStream};
use serde_json::{json, Value};

#[path = "../src/main.rs"]
mod reference;
use reference::repair_support::Server;

const CERT: &[u8] = include_bytes!("fixtures/repair_fixtures/localhost.pem");
const IDENTITY: &[u8] = include_bytes!("fixtures/repair_fixtures/localhost.p12");
const ANCHOR: &[u8] = include_bytes!("fixtures/repair_fixtures/localhost.der");
const PASSWORD: &str = "repair-fixture";
const OPERATOR: &str = "agent:did:layerx:repair-operator:main";
static NEXT_WORKSPACE: AtomicUsize = AtomicUsize::new(0);

struct Workspace(PathBuf);

impl Workspace {
    fn new() -> Self {
        let id = NEXT_WORKSPACE.fetch_add(1, Ordering::Relaxed);
        let dir =
            std::env::temp_dir().join(format!("repair-22-33-1487-{}-{id}", std::process::id()));
        fs::create_dir(&dir).expect("private workspace");
        fs::set_permissions(&dir, fs::Permissions::from_mode(0o700))
            .expect("directory permissions");
        let workspace = Self(dir);
        for (name, bytes) in [
            ("ca.pem", CERT),
            ("identity.p12", IDENTITY),
            ("anchor.der", ANCHOR),
            ("password", PASSWORD.as_bytes()),
            ("service-token", b"repair-service"),
            ("operator-token", b"repair-operator"),
        ] {
            let path = workspace.0.join(name);
            fs::write(&path, bytes).expect("fixture file");
            fs::set_permissions(path, fs::Permissions::from_mode(0o600))
                .expect("fixture permissions");
        }
        workspace
    }

    fn config(&self, peer: SocketAddr, custody_cadence: u64) -> Value {
        let p = |name: &str| self.0.join(name);
        let endpoint = format!("https://localhost:{}", peer.port());
        json!({
            "listen": "127.0.0.1:0", "listener": "tls",
            "server_identity_pkcs12": p("identity.p12"),
            "server_identity_password_file": p("password"),
            "journal_path": p("journal.jsonl"), "worker_id": "repair-worker",
            "lease_seconds": 60, "reconcile_seconds": 20,
            "operator": {"principal_id": "repair-operator", "account": OPERATOR,
                         "signer_key_handle": "kms://repair/layerx"},
            "quotes": [{
                "quote_id": "repair-quote", "direction": "off_ramp",
                "layerx_asset": ([1_u8; 32]), "layerx_amount": 1_000_000,
                "external_currency": "EUR", "external_amount_minor": 100,
                "rate_numerator": 1_000_000, "rate_denominator": 100,
                "fee_minor": 2, "maximum_slippage_bps": 25, "context": ([8_u8; 32]),
                "provider_token": "product", "payout_token": "beneficiary",
                "expires_at": 4_000_000_000_u64
            }],
            "client_tls": {"ca_pem": p("ca.pem"), "identity_pkcs12": p("identity.p12"),
                "identity_password_file": p("password"), "timeout_seconds": 2},
            "identity": {"endpoint": endpoint, "service_token_file": p("service-token"),
                "audience": "repair-ramp"},
            "compliance": {"endpoint": endpoint, "service_token_file": p("service-token"),
                "public_key": "22".repeat(32)},
            "provider": {"endpoint": endpoint, "credential_file": p("service-token"),
                "settlement_path": "/settlements", "status_path": "/settlements"},
            "layerx": {
                "rpc_endpoint": endpoint, "rpc_trust_anchor_der": p("anchor.der"),
                "chain_id": 125, "operator_address": format!("0x{}", "12".repeat(20)),
                "receipt_keys": [{"keyId": "01".repeat(32),
                    "publicKey": "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a",
                    "firstHeight": 1, "lastHeight": 10}],
                "max_fee_per_gas": 1000, "max_priority_fee_per_gas": 2, "gas_limit_cap": 100000,
                "signer_endpoint": endpoint, "signer_token_file": p("service-token"),
                "actor_did": "did:layerx:repair-operator"
            },
            "paxeer": {
                "custody_endpoint": endpoint, "custody_credential_file": p("service-token"),
                "broadcast_path": "/rebalances", "status_path": "/rebalances",
                "operator_account": OPERATOR, "wallet_address": "repair-wallet",
                "vault_id": "repair-vault", "signer_key_handle": "kms://repair/paxeer",
                "poll_cadence_seconds": custody_cadence
            },
            "provider_callback_public_key": "44".repeat(32),
            "operator_control_token_file": p("operator-token")
        })
    }
}

impl Drop for Workspace {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove fixture workspace");
    }
}

fn identity() -> Identity {
    Identity::from_pkcs12(IDENTITY, PASSWORD).expect("committed test identity")
}

fn connect(address: SocketAddr) -> TlsStream<TcpStream> {
    let tcp = TcpStream::connect(address).expect("connect loopback");
    tcp.set_read_timeout(Some(Duration::from_secs(15)))
        .expect("client read bound");
    tcp.set_write_timeout(Some(Duration::from_secs(3)))
        .expect("client write bound");
    let mut builder = TlsConnector::builder();
    builder.disable_built_in_roots(true);
    builder.add_root_certificate(Certificate::from_pem(CERT).expect("fixture CA"));
    builder.identity(identity());
    builder
        .build()
        .expect("TLS connector")
        .connect("localhost", tcp)
        .unwrap_or_else(|_| panic!("authenticated TLS connection"))
}

fn request(
    address: SocketAddr,
    method: &str,
    path: &str,
    token: &str,
    body: &[u8],
) -> (u16, Value) {
    let mut tls = connect(address);
    write!(tls, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {token}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).expect("request headers");
    tls.write_all(body).expect("request body");
    tls.flush().expect("request flush");
    let mut response = Vec::new();
    let boundary = loop {
        let mut byte = [0];
        tls.read_exact(&mut byte)
            .expect("bounded HTTP response headers");
        response.push(byte[0]);
        assert!(response.len() <= 32 * 1024);
        if response.ends_with(b"\r\n\r\n") {
            break response.len() - 4;
        }
    };
    let header = std::str::from_utf8(&response[..boundary]).expect("response headers");
    let status = header
        .split_ascii_whitespace()
        .nth(1)
        .expect("status")
        .parse()
        .expect("status code");
    let length: usize = header
        .lines()
        .find_map(|line| line.strip_prefix("Content-Length: "))
        .expect("response length")
        .parse()
        .expect("length integer");
    response.resize(boundary + 4 + length, 0);
    tls.read_exact(&mut response[boundary + 4..])
        .expect("bounded HTTP response body");
    (
        status,
        serde_json::from_slice(&response[boundary + 4..]).expect("response JSON"),
    )
}

struct Peer {
    address: SocketAddr,
    events: mpsc::Receiver<(String, Instant)>,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}

impl Peer {
    fn start() -> Self {
        // Bind localhost's IPv4 address; Endpoint resolves localhost the same way
        // on this host, and the certificate covers both localhost and loopback.
        let tcp = TcpListener::bind("localhost:0").expect("fixture peer");
        let address = tcp.local_addr().expect("peer address");
        tcp.set_nonblocking(true).expect("peer nonblocking accept");
        let (events_tx, events) = mpsc::channel();
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            let acceptor = TlsAcceptor::new(identity()).expect("peer TLS");
            while !stopped.load(Ordering::Acquire) {
                let (stream, _) = match tcp.accept() {
                    Ok(pair) => pair,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("peer accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .expect("peer read bound");
                stream
                    .set_write_timeout(Some(Duration::from_secs(3)))
                    .expect("peer write bound");
                let mut tls = acceptor
                    .accept(stream)
                    .unwrap_or_else(|_| panic!("peer handshake"));
                let mut bytes = Vec::new();
                let boundary = loop {
                    let mut byte = [0];
                    tls.read_exact(&mut byte).expect("peer HTTP headers");
                    bytes.push(byte[0]);
                    assert!(bytes.len() <= 32 * 1024);
                    if bytes.ends_with(b"\r\n\r\n") {
                        break bytes.len();
                    }
                };
                let headers = std::str::from_utf8(&bytes).expect("peer header text");
                let line = headers
                    .lines()
                    .next()
                    .expect("peer request line")
                    .to_owned();
                assert!(headers.contains("Authorization: Bearer repair-service\r\n"));
                let length: usize = headers
                    .lines()
                    .find_map(|line| line.strip_prefix("Content-Length: "))
                    .expect("request length")
                    .parse()
                    .expect("peer length");
                bytes.resize(boundary + length, 0);
                tls.read_exact(&mut bytes[boundary..])
                    .expect("peer request body");
                let (status, response) = if line == "POST /v1/introspect HTTP/1.1" {
                    let body: Value =
                        serde_json::from_slice(&bytes[boundary..]).expect("introspection request");
                    assert_eq!(
                        body,
                        json!({"token": "customer-token", "audience": "repair-ramp"})
                    );
                    (
                        200,
                        json!({"active": true, "principal_id": "repair-customer",
                        "account": "agent:did:layerx:repair-customer:main",
                        "audience": "repair-ramp", "expires_at": 4_000_000_000_u64}),
                    )
                } else {
                    assert!(
                        line == "POST /rebalances HTTP/1.1"
                            || line.starts_with("GET /rebalances/by-idempotency/"),
                        "{line}"
                    );
                    if line.starts_with("POST") {
                        let body: Value =
                            serde_json::from_slice(&bytes[boundary..]).expect("custody request");
                        assert_eq!(body["operator_account"], OPERATOR);
                        assert_eq!(body["amount"], 100);
                        assert_eq!(body["idempotency_key"], json!(([7_u8; 32])));
                    }
                    // Recorded unavailable custody response: production must keep
                    // its durable intent pending and retry on the configured clock.
                    (503, json!({"error": "custody_unavailable"}))
                };
                events_tx
                    .send((line, Instant::now()))
                    .expect("record peer request");
                let body = serde_json::to_vec(&response).expect("peer response");
                write!(tls, "HTTP/1.1 {status} Fixture\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).expect("peer headers");
                tls.write_all(&body).expect("peer body");
                tls.flush().expect("peer flush");
                tls.shutdown().expect("peer TLS close notify");
            }
        });
        Self {
            address,
            events,
            stop,
            worker: Some(worker),
        }
    }
}

impl Drop for Peer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            worker.join().expect("fixture peer shutdown");
        }
    }
}

fn wait_active(server: &Server, expected: usize, budget: Duration) {
    let deadline = Instant::now() + budget;
    while server.active() != expected {
        assert!(
            Instant::now() < deadline,
            "active permits: {}, expected {expected}",
            server.active()
        );
        thread::sleep(Duration::from_millis(5));
    }
}

fn healthy(server: &Server) {
    let (status, body) = request(server.address, "GET", "/livez", "unused", &[]);
    assert_eq!(status, 200, "{body}");
    assert_eq!(body, json!({"live": true}));
}

#[test]
fn order_purpose_replay_preserves_the_http_binding() {
    let peer = Peer::start();
    let workspace = Workspace::new();
    let server = Server::start(workspace.config(peer.address, 1), 2, false);
    let create = CreateOrder {
        order_id: "purpose-replay".into(),
        quote_id: "repair-quote".into(),
        payer_grant: Some([4; 32]),
        payer_grant_purpose: Some([5; 32]),
    };
    let send = |value: &Value| {
        request(
            server.address,
            "POST",
            "/v1/orders",
            "customer-token",
            &serde_json::to_vec(value).expect("encode CreateOrder"),
        )
    };
    let original = serde_json::to_value(&create).expect("real order producer");
    let (status, initial) = send(&original);
    assert_eq!(status, 201, "{initial}");
    let bound = server.order(&create.order_id);
    assert_eq!(bound.payer_grant_purpose, create.payer_grant_purpose);
    assert_eq!(bound.digest(), bound.order_digest);
    assert_eq!(send(&original), (201, initial.clone()));
    for purpose in [Some([6; 32]), None] {
        let mut conflicting = create.clone();
        conflicting.payer_grant_purpose = purpose;
        let mut value = serde_json::to_value(&conflicting).expect("changed request");
        if purpose.is_none() {
            value
                .as_object_mut()
                .expect("request object")
                .remove("customer_budget_purpose");
        }
        let (status, body) = send(&value);
        assert_eq!(status, 409, "{body}");
        assert_eq!(body["error"], "order_id_conflict");
        assert_eq!(server.order(&create.order_id), bound);
    }
    // Both maintained wire names and the supported legacy aliases bind purpose.
    let mut legacy = original.clone();
    let object = legacy.as_object_mut().expect("order object");
    let purpose = object.remove("customer_budget_purpose").expect("purpose");
    object.insert("payer_grant_purpose".into(), purpose);
    assert_eq!(send(&legacy), (201, initial.clone()));
    legacy["payer_grant_purpose"] = json!(([9_u8; 32]));
    assert_eq!(send(&legacy).0, 409);
    assert_eq!(send(&original), (201, initial));
    let (status, order) = request(
        server.address,
        "GET",
        &format!(
            "/v1/orders/{}",
            layerx_ramp_toolkit::clients::hex(&bound.order_digest)
        ),
        "customer-token",
        &[],
    );
    assert_eq!(status, 200, "{order}");
    assert_eq!(order["order"], json!(bound));
}

#[test]
fn custody_cadence_controls_real_pending_reconciliation() {
    for seconds in [1, 3] {
        let peer = Peer::start();
        let workspace = Workspace::new();
        let server = Server::start(workspace.config(peer.address, seconds), 2, true);
        let started = Instant::now();
        let (status, body) = request(server.address, "POST", "/internal/v1/rebalances", "repair-operator",
            &serde_json::to_vec(&json!({"action": "submit", "asset": ([1_u8;32]), "amount": 100, "idempotency_key": ([7_u8;32])})).expect("rebalance request"));
        assert_eq!(status, 503, "{body}");
        assert!(server.pending_custody([7; 32]));
        assert_eq!(
            peer.events
                .recv_timeout(Duration::from_secs(1))
                .expect("broadcast")
                .0,
            "POST /rebalances HTTP/1.1"
        );
        let mut last = None;
        for _ in 0..2 {
            let (line, observed) = peer
                .events
                .recv_timeout(Duration::from_secs(seconds + 2))
                .expect("configured custody poll");
            assert_eq!(
                line,
                format!(
                    "GET /rebalances/by-idempotency/{} HTTP/1.1",
                    "07".repeat(32)
                )
            );
            let elapsed = observed.duration_since(last.unwrap_or(started));
            assert!(
                elapsed >= Duration::from_millis(seconds * 1000 - 250),
                "early custody poll: {elapsed:?}"
            );
            assert!(
                elapsed < Duration::from_millis(seconds * 1000 + 1500),
                "custody followed order cadence instead: {elapsed:?}"
            );
            last = Some(observed);
            assert!(
                server.pending_custody([7; 32]),
                "unavailable custody must retain intent"
            );
        }
        let (status, retained) = request(
            server.address,
            "GET",
            &format!("/internal/v1/rebalances/{}", "07".repeat(32)),
            "repair-operator",
            &[],
        );
        assert_eq!(status, 200, "{retained}");
        assert_eq!(retained["status"], "submission_planned");
        healthy(&server);
    }
}

#[test]
fn slow_tls_releases_permits_and_serves_healthy_clients() {
    let peer = Peer::start();
    let workspace = Workspace::new();
    let server = Server::start(workspace.config(peer.address, 1), 2, false);
    let mut stalled = TcpStream::connect(server.address).expect("slow TLS peer");
    let started = Instant::now();
    // An incomplete TLS record stalls negotiation before any HTTP parser runs.
    stalled
        .write_all(&[0x16, 0x03, 0x03, 0x00, 0x80])
        .expect("partial TLS hello");
    wait_active(&server, 1, Duration::from_secs(1));
    healthy(&server);
    wait_active(&server, 1, Duration::from_secs(1));
    let _second = TcpStream::connect(server.address).expect("second slow TLS peer");
    wait_active(&server, 2, Duration::from_secs(1));
    wait_active(&server, 0, Duration::from_secs(12));
    assert!(started.elapsed() >= Duration::from_secs(9));
    assert!(started.elapsed() < Duration::from_secs(12));
    // Both clients remain open: only the listener deadline can free the permits.
    healthy(&server);
}

#[test]
fn slow_reader_releases_permits_and_serves_healthy_clients() {
    let peer = Peer::start();
    let workspace = Workspace::new();
    let server = Server::start(workspace.config(peer.address, 1), 2, false);
    let (address, worker) = server.response_writer();
    let started = Instant::now();
    let mut stalled = connect(address);
    stalled
        .write_all(b"GET /livez HTTP/1.1\r\nHost: localhost\r\n\r\n")
        .expect("slow reader request");
    stalled.flush().expect("send request");
    wait_active(&server, 1, Duration::from_secs(1));
    healthy(&server);
    // Keep the TLS peer alive without draining its TCP receive window.
    wait_active(&server, 0, Duration::from_secs(12));
    let failure = worker
        .join()
        .expect("response writer")
        .expect_err("slow reader must hit the deadline");
    assert!(
        matches!(
            failure.kind(),
            std::io::ErrorKind::TimedOut | std::io::ErrorKind::WouldBlock
        ),
        "{failure}"
    );
    assert!(started.elapsed() >= Duration::from_secs(9));
    assert!(started.elapsed() < Duration::from_secs(12));
    healthy(&server);
    drop(stalled);
}
