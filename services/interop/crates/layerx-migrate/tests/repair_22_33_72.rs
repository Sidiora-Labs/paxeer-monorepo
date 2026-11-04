//! Task record 22.33.72 (audit 22.30.109).
//! gateway-deadline-budget: gateway_deadline_and_same_idempotency_retry.
//! migration-rpc-deadline: migration_rpc_deadline_stalled_and_streaming_https,
//! migration_rpc_healthy_majority_and_complete_frames.
//! rollback-authority-unwired: authority_journal_append_reopen_and_rollback_refusal,
//! authority_concurrent_cas_authentication_and_restart.
//! Baseline: this target did not exist at the starting revision. No baseline
//! verify ran: the explicit gate order permits only hf-build then hf-done after
//! complete implementation. The gate ledger records revision and private logs.
//! Wiring request (domain 8): services/platform/hosted/authority must register
//! the migration_head::MigrationHeadAuthority authenticated JSON-RPC handler,
//! enroll principal-owned anchor namespaces, and deploy independent majority
//! endpoints on durable volumes separate from journal backups; routing, control
//! plane enrolment, credentials and deployment are outside this task's scope.

use std::collections::BTreeMap;
use std::fs::{self, OpenOptions};
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::{DirBuilderExt as _, OpenOptionsExt as _};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicU64, AtomicUsize, Ordering};
use std::sync::{Arc, Barrier, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_migrate::ethereum::{EthereumConfig, EthereumHistoryClaim, EthereumVerifier};
use layerx_migrate::gateway_client::{GatewayClient, GatewayClientConfig};
use layerx_migrate::history::DurableExternalHistory;
use layerx_migrate::migration_head::{
    AdvanceMigrationJournalHead, MigrationHeadAuthority, ADVANCE_MIGRATION_JOURNAL_HEAD,
    GET_MIGRATION_JOURNAL_HEAD,
};
use layerx_migrate::ramp_v2::SourceSettlementResponseV2;
use layerx_migrate::{
    ExternalHistorySink, JournalConfig, MigrationError, RpcEndpointConfig, RpcQuorumConfig,
    SourceVerifier,
};
use rustls::pki_types::{CertificateDer, PrivatePkcs8KeyDer, ServerName};
use rustls::{
    ClientConfig, ClientConnection, RootCertStore, ServerConfig, ServerConnection, StreamOwned,
};
use serde_json::{json, Value};
use sha3::{Digest as _, Keccak256};

const CERT: &[u8] = include_bytes!("fixtures/repair_22_31_163/tls-cert.der");
const KEY: &[u8] = include_bytes!("fixtures/repair_22_31_163/tls-key.der");
const CASE: &str = include_str!("fixtures/repair_22_31_163/history-case.json");
const TOKEN: &str = "public-repair-fixture-token";
const BUDGET_MS: u64 = 300;
const UPPER_BOUND: Duration = Duration::from_millis(450);
static NEXT: AtomicU64 = AtomicU64::new(1);

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "layerx-repair-22-33-72-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::DirBuilder::new().mode(0o700).create(&path).unwrap();
        Self(fs::canonicalize(path).unwrap())
    }
    fn write(&self, name: &str, bytes: &[u8]) -> PathBuf {
        let path = self.0.join(name);
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&path)
            .unwrap();
        file.write_all(bytes).unwrap();
        path
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}

fn owner() -> PrincipalId {
    PrincipalId::new("migration-owner".to_owned()).unwrap()
}
fn other() -> PrincipalId {
    PrincipalId::new("other-owner".to_owned()).unwrap()
}

#[derive(Clone, Copy, Debug)]
enum Delay {
    None,
    TlsStall,
    LateHeader,
    LateBody,
    StreamHeader,
    StreamBody,
    StreamChunked,
    KeepOpen,
}

type Tls = StreamOwned<ServerConnection, TcpStream>;
type Handler = dyn Fn(String, Value) -> Value + Send + Sync;
struct Server {
    address: SocketAddr,
    stopping: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl Server {
    fn start(delay: Delay, first_only: bool, handler: Arc<Handler>) -> Self {
        let tls = Arc::new(
            ServerConfig::builder()
                .with_no_client_auth()
                .with_single_cert(
                    vec![CertificateDer::from(CERT.to_vec())],
                    PrivatePkcs8KeyDer::from(KEY.to_vec()).into(),
                )
                .unwrap(),
        );
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let stopping = Arc::new(AtomicBool::new(false));
        let stop = stopping.clone();
        let worker = thread::spawn(move || {
            let count = AtomicUsize::new(0);
            let mut workers = Vec::new();
            for socket in listener.incoming() {
                let socket = socket.unwrap();
                if stop.load(Ordering::Acquire) {
                    break;
                }
                let delay = if !first_only || count.fetch_add(1, Ordering::Relaxed) == 0 {
                    delay
                } else {
                    Delay::None
                };
                let tls = tls.clone();
                let handler = handler.clone();
                workers.push(thread::spawn(move || {
                    socket
                        .set_read_timeout(Some(Duration::from_secs(2)))
                        .unwrap();
                    socket
                        .set_write_timeout(Some(Duration::from_secs(2)))
                        .unwrap();
                    if matches!(delay, Delay::TlsStall) {
                        thread::sleep(Duration::from_millis(650));
                        return;
                    }
                    let mut stream = StreamOwned::new(ServerConnection::new(tls).unwrap(), socket);
                    let (headers, request) = read_request(&mut stream);
                    let response = handler(headers, request);
                    send_response(&mut stream, &response, delay);
                }));
            }
            for worker in workers {
                worker.join().unwrap();
            }
        });
        Self {
            address,
            stopping,
            worker: Some(worker),
        }
    }
}
impl Drop for Server {
    fn drop(&mut self) {
        self.stopping.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        let result = self.worker.take().unwrap().join();
        if !thread::panicking() {
            result.unwrap();
        }
    }
}

fn read_request(stream: &mut impl Read) -> (String, Value) {
    let mut header = Vec::new();
    while !header.ends_with(b"\r\n\r\n") {
        assert!(header.len() < 32 * 1024);
        let mut byte = [0];
        stream.read_exact(&mut byte).unwrap();
        header.push(byte[0]);
    }
    let header = String::from_utf8(header).unwrap();
    assert!(header.contains(&format!("Authorization: Bearer {TOKEN}\r\n")));
    let size: usize = header
        .lines()
        .find_map(|line| line.strip_prefix("Content-Length: "))
        .unwrap()
        .parse()
        .unwrap();
    assert!(size <= 1024 * 1024);
    let mut body = vec![0; size];
    stream.read_exact(&mut body).unwrap();
    (header, serde_json::from_slice(&body).unwrap())
}

fn send_response(stream: &mut Tls, response: &Value, delay: Delay) {
    let body = serde_json::to_vec(response).unwrap();
    let header = format!(
        "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n",
        body.len()
    );
    let write =
        |stream: &mut Tls, bytes: &[u8]| stream.write_all(bytes).and_then(|()| stream.flush());
    match delay {
        Delay::LateHeader => {
            thread::sleep(Duration::from_millis(200));
            let _ = write(stream, b"HTTP/1.1 200 OK\r\n");
            thread::sleep(Duration::from_millis(450));
            return;
        }
        Delay::LateBody => {
            if write(stream, header.as_bytes()).is_err() {
                return;
            }
            thread::sleep(Duration::from_millis(200));
            let _ = write(stream, &body[..1]);
            thread::sleep(Duration::from_millis(450));
            return;
        }
        Delay::StreamHeader => {
            for byte in header.bytes() {
                if write(stream, &[byte]).is_err() {
                    return;
                }
                thread::sleep(Duration::from_millis(35));
            }
            return;
        }
        Delay::StreamBody | Delay::StreamChunked => {
            let chunked = matches!(delay, Delay::StreamChunked);
            let header = if chunked {
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nTransfer-Encoding: chunked\r\n\r\n".to_owned()
            } else {
                header
            };
            if write(stream, header.as_bytes()).is_err() {
                return;
            }
            for byte in body {
                let bytes = if chunked {
                    vec![b'1', b'\r', b'\n', byte, b'\r', b'\n']
                } else {
                    vec![byte]
                };
                if write(stream, &bytes).is_err() {
                    return;
                }
                thread::sleep(Duration::from_millis(35));
            }
            if chunked {
                let _ = write(stream, b"0\r\n\r\n");
            }
        }
        _ => {
            if write(stream, header.as_bytes())
                .and_then(|()| write(stream, &body))
                .is_err()
            {
                return;
            }
            if matches!(delay, Delay::KeepOpen) {
                thread::sleep(Duration::from_millis(650));
            }
        }
    }
    stream.conn.send_close_notify();
    let _ = stream.flush();
}

fn rpc_config(directory: &Directory, servers: &[Server], budget: u64) -> RpcQuorumConfig {
    let suffix = NEXT.fetch_add(1, Ordering::Relaxed);
    let cert = directory.write(&format!("ca-{suffix}"), CERT);
    let token = directory.write(&format!("token-{suffix}"), TOKEN.as_bytes());
    RpcQuorumConfig {
        endpoints: servers
            .iter()
            .enumerate()
            .map(|(index, server)| RpcEndpointConfig {
                url: format!("https://{}/", server.address),
                ca_certificate_der: cert.clone(),
                bearer_token_file: token.clone(),
                independent_backend: format!("authority-{index}"),
            })
            .collect(),
        quorum: servers.len() / 2 + 1,
        connect_timeout_ms: 100,
        request_timeout_ms: budget,
        maximum_response_bytes: 1024 * 1024,
    }
}
fn journal_config(
    directory: &Directory,
    rpc: RpcQuorumConfig,
    namespace: &str,
    anchor: &str,
) -> JournalConfig {
    JournalConfig {
        directory: directory.0.clone(),
        authentication_key_file: directory.write(
            &format!("key-{namespace}"),
            b"public-repair-22-33-72-journal-key",
        ),
        namespace: namespace.to_owned(),
        rollback_anchor_id: anchor.to_owned(),
        rollback_anchor_rpc: rpc,
    }
}
fn zero_response(_: String, request: Value) -> Value {
    json!({"jsonrpc":"2.0", "id":request["id"], "result":{"sequence":0, "digest":"00".repeat(32)}})
}

#[test]
fn gateway_deadline_and_same_idempotency_retry() {
    for delay in [
        Delay::LateHeader,
        Delay::LateBody,
        Delay::StreamHeader,
        Delay::StreamBody,
        Delay::StreamChunked,
    ] {
        let directory = Directory::new();
        let evidence = EthereumHistoryClaim {
            chain_id: 1,
            address: [17; 20],
            from_block: 100,
            to_block: 100,
            previous_cursor: None,
        }
        .evidence()
        .unwrap();
        let accepted = Arc::new(Mutex::new(BTreeMap::<String, Value>::new()));
        let observed = Arc::new(Mutex::new(Vec::new()));
        let accepted_peer = accepted.clone();
        let observed_peer = observed.clone();
        let digest = evidence.digest();
        let server = Server::start(
            delay,
            true,
            Arc::new(move |header, request| {
                assert!(header.starts_with("POST /v2/migration/assets HTTP/1.1\r\n"));
                let idempotency = header
                    .lines()
                    .find_map(|line| line.strip_prefix("Idempotency-Key: "))
                    .unwrap()
                    .to_owned();
                let trace = header
                    .lines()
                    .find_map(|line| line.strip_prefix("X-LayerX-Trace-Id: "))
                    .unwrap();
                observed_peer
                    .lock()
                    .unwrap()
                    .push((idempotency.clone(), request.clone()));
                let settled = SourceSettlementResponseV2 {
                    version: "layerx-migration-source-v2".to_owned(),
                    order_digest: [7; 32],
                    state: "source_pending".to_owned(),
                    source_evidence_digest: digest,
                    source_claim_id: None,
                };
                let mut accepted = accepted_peer.lock().unwrap();
                accepted.entry(idempotency).or_insert_with(|| json!({"ok":true, "operation":"migration:one", "trace":trace,
                "result":{"source_settlement":settled, "provenance":"external-custody", "custody_label":"External custody: this independent market maker controls the off-platform funds and payout.", "layerx_receipt":false}})).clone()
            }),
        );
        let client = GatewayClient::new(&GatewayClientConfig {
            endpoint: format!("https://{}", server.address),
            ca_certificate_der: directory.write("ca", CERT),
            customer_authorization_file: directory
                .write("authorization", format!("Bearer {TOKEN}").as_bytes()),
            connect_timeout_ms: 100,
            request_timeout_ms: BUDGET_MS,
            maximum_response_bytes: 8192,
        })
        .unwrap();
        let trace = TraceId::mint([72; 16]);
        let start = Instant::now();
        assert!(
            matches!(
                client.migrate_asset([7; 32], "ethereum", &evidence, "same-operation", &trace),
                Err(MigrationError::GatewaySubmissionUnknown)
            ),
            "{delay:?}"
        );
        assert!(
            start.elapsed() < UPPER_BOUND,
            "gateway {delay:?} exceeded total budget: {:?}",
            start.elapsed()
        );
        let retried = client
            .migrate_asset([7; 32], "ethereum", &evidence, "same-operation", &trace)
            .unwrap();
        let value = serde_json::to_value(retried).unwrap();
        assert_eq!(value["operation"], "migration:one");
        assert_eq!(value["layerx_credit_verified"], false);
        assert_eq!(accepted.lock().unwrap().len(), 1);
        let requests = observed.lock().unwrap();
        assert_eq!(requests.len(), 2);
        assert_eq!(requests[0], requests[1]);
    }
}

#[test]
fn migration_rpc_deadline_stalled_and_streaming_https() {
    for delay in [
        Delay::TlsStall,
        Delay::LateHeader,
        Delay::LateBody,
        Delay::StreamHeader,
        Delay::StreamBody,
        Delay::StreamChunked,
    ] {
        let directory = Directory::new();
        let servers: Vec<_> = (0..3)
            .map(|_| Server::start(delay, false, Arc::new(zero_response)))
            .collect();
        let config = journal_config(
            &directory,
            rpc_config(&directory, &servers, BUDGET_MS),
            "journal",
            "head",
        );
        let start = Instant::now();
        assert!(
            matches!(
                DurableExternalHistory::new(&config),
                Err(MigrationError::RpcUnavailable)
            ),
            "{delay:?}"
        );
        assert!(
            start.elapsed() < UPPER_BOUND,
            "RPC {delay:?} exceeded total budget: {:?}",
            start.elapsed()
        );
    }
}

#[test]
fn migration_rpc_healthy_majority_and_complete_frames() {
    let directory = Directory::new();
    let servers = vec![
        Server::start(Delay::TlsStall, false, Arc::new(zero_response)),
        Server::start(Delay::KeepOpen, false, Arc::new(zero_response)),
        Server::start(Delay::KeepOpen, false, Arc::new(zero_response)),
    ];
    let config = journal_config(
        &directory,
        rpc_config(&directory, &servers, BUDGET_MS),
        "journal",
        "head",
    );
    let start = Instant::now();
    let sink = DurableExternalHistory::new(&config).unwrap();
    assert!(
        start.elapsed() < UPPER_BOUND,
        "healthy quorum waited beyond the shared deadline"
    );
    assert!(sink.read(&owner(), None, 10).unwrap().records().is_empty());
}

fn authority_server(
    directory: &Path,
    anchor: &str,
    delay: Delay,
    transcript: Arc<Vec<Value>>,
) -> Server {
    let authority = MigrationHeadAuthority::open(directory, owner(), anchor.to_owned()).unwrap();
    Server::start(
        delay,
        false,
        Arc::new(move |_, request| {
            if request["method"]
                .as_str()
                .is_some_and(|method| method.starts_with("layerx_"))
            {
                match authority.handle(&owner(), &request) {
                    Ok(response) => response,
                    Err(error) => {
                        json!({"jsonrpc":"2.0", "id":request["id"], "error":{"code":-32000,"message":error.to_string()}})
                    }
                }
            } else {
                let exchange = transcript
                    .iter()
                    .find(|exchange| {
                        exchange["method"] == request["method"]
                            && exchange["params"] == request["params"]
                    })
                    .unwrap_or_else(|| panic!("unrecorded transcript {request}"));
                json!({"jsonrpc":"2.0", "id":request["id"], "result":exchange["result"]})
            }
        }),
    )
}

#[test]
fn authority_journal_append_reopen_and_rollback_refusal() {
    let directory = Directory::new();
    let case: Value = serde_json::from_str(CASE).unwrap();
    let transcript = Arc::new(case["transcript"].as_array().unwrap().clone());
    let paths: Vec<_> = (0..2)
        .map(|index| directory.0.join(format!("authority-{index}")))
        .collect();
    let mut servers: Vec<_> = paths
        .iter()
        .map(|path| authority_server(path, "head", Delay::None, transcript.clone()))
        .collect();
    let rpc = rpc_config(&directory, &servers, 2000);
    let config = journal_config(&directory, rpc.clone(), "journal", "head");
    let mut source: EthereumConfig = serde_json::from_value(case["source"].clone()).unwrap();
    source.rpc = rpc;
    source.journal = config.clone();
    let code = transcript
        .iter()
        .find(|exchange| exchange["method"] == "eth_getCode")
        .unwrap()["result"]
        .as_str()
        .unwrap()
        .trim_start_matches("0x")
        .to_owned();
    let code: Vec<_> = (0..code.len())
        .step_by(2)
        .map(|offset| u8::from_str_radix(&code[offset..offset + 2], 16).unwrap())
        .collect();
    source.custody_code_hash = Keccak256::digest(code).into();
    let verifier = EthereumVerifier::new(source.clone()).unwrap();
    let claim = EthereumHistoryClaim {
        chain_id: 1,
        address: [17; 20],
        from_block: 100,
        to_block: 100,
        previous_cursor: None,
    }
    .evidence()
    .unwrap();
    let trace = TraceId::mint([73; 16]);
    let page = verifier.verify_history(&claim, &trace).unwrap();
    assert_eq!(
        page.records().len(),
        case["expected_record_count"].as_u64().unwrap() as usize
    );
    let mut sink = DurableExternalHistory::new(&config).unwrap();
    sink.store_external(&owner(), &page, &trace).unwrap();
    let stored = sink.read(&owner(), None, 256).unwrap();
    assert_eq!(stored.records().len(), page.records().len());
    drop(sink);
    drop(verifier);
    let heads: Vec<_> = paths
        .iter()
        .map(|path| {
            MigrationHeadAuthority::open(path, owner(), "head".to_owned())
                .unwrap()
                .current()
                .unwrap()
        })
        .collect();
    assert!(heads[0].sequence >= 2);
    assert_eq!(heads[0], heads[1]);
    // Stop all HTTPS authority instances, release their stores, and reopen the same volumes.
    servers.clear();
    for path in &paths {
        servers.push(authority_server(
            path,
            "head",
            Delay::None,
            transcript.clone(),
        ));
    }
    let mut recovered_config = config.clone();
    recovered_config.rollback_anchor_rpc = rpc_config(&directory, &servers, 2000);
    let recovered = DurableExternalHistory::new(&recovered_config).unwrap();
    assert_eq!(recovered.read(&owner(), None, 256).unwrap(), stored);
    drop(recovered);
    for path in &paths {
        assert_eq!(
            MigrationHeadAuthority::open(path, owner(), "head".to_owned())
                .unwrap()
                .current()
                .unwrap(),
            heads[0]
        );
    }
    // Restore a valid earlier local journal prefix, including removing its seals.
    // The independent restarted authorities retain the later committed head.
    let journal = directory.0.join("journal");
    for entry in fs::read_dir(&journal).unwrap() {
        let entry = entry.unwrap();
        if entry
            .file_name()
            .to_str()
            .is_some_and(|name| name.ends_with(".record") || name.ends_with(".seal"))
        {
            fs::remove_file(entry.path()).unwrap();
        }
    }
    assert!(matches!(
        DurableExternalHistory::new(&recovered_config),
        Err(MigrationError::CheckpointIntegrity)
    ));
}

fn https_rpc(address: SocketAddr, request: &Value) -> Value {
    let mut roots = RootCertStore::empty();
    roots.add(CertificateDer::from(CERT.to_vec())).unwrap();
    let tls = Arc::new(
        ClientConfig::builder()
            .with_root_certificates(roots)
            .with_no_client_auth(),
    );
    let socket = TcpStream::connect(address).unwrap();
    socket
        .set_read_timeout(Some(Duration::from_secs(2)))
        .unwrap();
    socket
        .set_write_timeout(Some(Duration::from_secs(2)))
        .unwrap();
    let mut stream = StreamOwned::new(
        ClientConnection::new(tls, ServerName::try_from("127.0.0.1").unwrap()).unwrap(),
        socket,
    );
    let body = serde_json::to_vec(request).unwrap();
    write!(stream, "POST / HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {TOKEN}\r\nContent-Length: {}\r\n\r\n", body.len()).unwrap();
    stream.write_all(&body).unwrap();
    stream.flush().unwrap();
    let mut bytes = Vec::new();
    stream.read_to_end(&mut bytes).unwrap();
    let end = bytes
        .windows(4)
        .position(|part| part == b"\r\n\r\n")
        .unwrap()
        + 4;
    serde_json::from_slice(&bytes[end..]).unwrap()
}

#[test]
fn authority_concurrent_cas_authentication_and_restart() {
    let directory = Directory::new();
    let path = directory.0.join("authority");
    let authority = MigrationHeadAuthority::open(&path, owner(), "head".to_owned()).unwrap();
    let get =
        json!({"jsonrpc":"2.0", "id":1, "method":GET_MIGRATION_JOURNAL_HEAD, "params":["head"]});
    assert!(matches!(
        authority.handle(&other(), &get),
        Err(MigrationError::PlaneRefused)
    ));
    let bad = json!({"jsonrpc":"2.0", "id":1, "method":GET_MIGRATION_JOURNAL_HEAD, "params":["another-namespace"]});
    assert!(matches!(
        authority.handle(&owner(), &bad),
        Err(MigrationError::PlaneRefused)
    ));
    assert!(MigrationHeadAuthority::open(&path, other(), "head".to_owned()).is_err());
    drop(authority);
    // Two independently opened store handles behind different HTTPS listeners
    // race on the same durable authority volume and OS lock.
    let servers: Vec<_> = (0..2)
        .map(|_| authority_server(&path, "head", Delay::None, Arc::new(Vec::new())))
        .collect();
    let barrier = Arc::new(Barrier::new(3));
    let workers: Vec<_> = servers.iter().enumerate().map(|(index, server)| {
        let barrier = barrier.clone();
        let address = server.address;
        thread::spawn(move || {
            let request = json!({"jsonrpc":"2.0", "id":index + 2, "method":ADVANCE_MIGRATION_JOURNAL_HEAD, "params":[{"anchor_id":"head", "expected_sequence":0, "expected_digest":"00".repeat(32), "sequence":1, "digest":format!("{:02x}", index + 1).repeat(32)}]});
            barrier.wait();
            (https_rpc(address, &request), request)
        })
    }).collect();
    barrier.wait();
    let results: Vec<_> = workers
        .into_iter()
        .map(|worker| worker.join().unwrap())
        .collect();
    assert_eq!(
        results
            .iter()
            .filter(|(response, _)| response.get("result").is_some())
            .count(),
        1
    );
    assert_eq!(
        results
            .iter()
            .filter(|(response, _)| response.get("error").is_some())
            .count(),
        1
    );
    let (winner, request) = results
        .iter()
        .find(|(response, _)| response.get("result").is_some())
        .unwrap();
    assert_eq!(
        https_rpc(servers[0].address, request)["result"],
        winner["result"],
        "same CAS retry is idempotent"
    );
    drop(servers);
    let reopened = MigrationHeadAuthority::open(&path, owner(), "head".to_owned()).unwrap();
    assert_eq!(
        serde_json::to_value(reopened.current().unwrap()).unwrap(),
        winner["result"]
    );
    let stale = AdvanceMigrationJournalHead {
        anchor_id: "head".to_owned(),
        expected_sequence: 0,
        expected_digest: "00".repeat(32),
        sequence: 1,
        digest: "03".repeat(32),
    };
    assert!(matches!(
        reopened.advance(&owner(), &stale),
        Err(MigrationError::CheckpointConflict)
    ));
    assert!(matches!(
        reopened.advance(&other(), &stale),
        Err(MigrationError::PlaneRefused)
    ));
    drop(reopened);
    fs::remove_file(path.join("head.json")).unwrap();
    assert!(
        MigrationHeadAuthority::open(&path, owner(), "head".to_owned()).is_err(),
        "missing durable state must never reset a namespace"
    );
}
