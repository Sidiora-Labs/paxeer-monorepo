use layerx_platform_internal::{
    events::{enrollment_snapshot_mac, Fact, Kind, ProducerCredential, Record, Service},
    gateway_http::{Client as GatewayClient, Endpoint, OutboundRequest},
    http::{self, Request, Response},
    journal::Journal,
    producer::{Client, Observation, QueueState, MAX_RETAINED_OBSERVATIONS, MAX_TRACKED_RESOURCES},
    secret::{random_hex, read_secret_file, MAX_SECRET_BYTES},
    tls::{Origin, Upstream},
};
use native_tls::{Certificate, Identity};
use rustls::{
    pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer},
    RootCertStore, ServerConfig, ServerConnection, StreamOwned,
};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::{
    collections::BTreeMap,
    fs,
    io::Write,
    net::TcpListener,
    os::unix::fs::PermissionsExt,
    path::{Path, PathBuf},
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex,
    },
    thread::JoinHandle,
    time::Duration,
};
use zeroize::Zeroizing;

const CA: &[u8] = include_bytes!("fixtures/repair_22_33_1452_tls/configured-ca.der");
const OTHER_CA: &[u8] = include_bytes!("fixtures/repair_22_33_1452_tls/unconfigured-ca.der");
const SERVER: &[u8] = include_bytes!("fixtures/repair_22_33_1452_tls/configured-server.der");
const SERVER_KEY: &[u8] =
    include_bytes!("fixtures/repair_22_33_1452_tls/configured-server-key.der");
const OTHER_SERVER: &[u8] =
    include_bytes!("fixtures/repair_22_33_1452_tls/unconfigured-server.der");
const OTHER_SERVER_KEY: &[u8] =
    include_bytes!("fixtures/repair_22_33_1452_tls/unconfigured-server-key.der");
const IDENTITY: &[u8] = include_bytes!("fixtures/repair_22_33_1452_tls/configured-client.p12");
const OTHER_IDENTITY: &[u8] =
    include_bytes!("fixtures/repair_22_33_1452_tls/unconfigured-client.p12");
const PRINCIPAL: &str = "repair-principal";
const ENROLLMENT_KEY: &str = "repair-enrollment-authentication-key";

fn required<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-1452-{}",
            required(random_hex(16)).as_str()
        ));
        required(fs::create_dir(&path));
        required(fs::set_permissions(
            &path,
            fs::Permissions::from_mode(0o700),
        ));
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        required(fs::remove_dir_all(&self.0));
    }
}

fn server_config(cert: &[u8], key: &[u8]) -> Arc<ServerConfig> {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let mut roots = RootCertStore::empty();
    required(roots.add(CertificateDer::from(CA.to_vec())));
    let verifier = required(rustls::server::WebPkiClientVerifier::builder(Arc::new(roots)).build());
    Arc::new(required(
        ServerConfig::builder()
            .with_client_cert_verifier(verifier)
            .with_single_cert(
                vec![CertificateDer::from(cert.to_vec())],
                PrivateKeyDer::from(PrivatePkcs8KeyDer::from(key.to_vec())),
            ),
    ))
}

struct Server {
    port: u16,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl Server {
    fn start(
        config: Arc<ServerConfig>,
        handler: impl Fn(&Request) -> Response + Send + 'static,
    ) -> Self {
        let listener = required(TcpListener::bind("127.0.0.1:0"));
        let port = required(listener.local_addr()).port();
        required(listener.set_nonblocking(true));
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((tcp, _)) => {
                        required(tcp.set_read_timeout(Some(Duration::from_secs(5))));
                        required(tcp.set_write_timeout(Some(Duration::from_secs(5))));
                        let mut stream = StreamOwned::new(
                            required(ServerConnection::new(Arc::clone(&config))),
                            tcp,
                        );
                        if let Ok(mut request) = http::parse_client_request(&mut stream) {
                            request.peer_verified = stream
                                .conn
                                .peer_certificates()
                                .is_some_and(|chain| !chain.is_empty());
                            assert!(request.peer_verified);
                            required(http::write_response(&mut stream, &handler(&request)));
                            required(stream.flush());
                            stream.conn.send_close_notify();
                            let _ = stream.flush();
                        }
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("{error}"),
                }
            }
        });
        Self {
            port,
            stop,
            worker: Some(worker),
        }
    }
}
impl Drop for Server {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            required(worker.join());
        }
    }
}

fn identity(bytes: &[u8]) -> Identity {
    required(Identity::from_pkcs12(bytes, "repair-fixture"))
}
fn upstream(port: u16, token: &str) -> Upstream {
    Upstream::new(
        required(Origin::parse(&format!("https://localhost:{port}"))),
        required(Certificate::from_der(CA)),
        Some(identity(IDENTITY)),
        Some(Zeroizing::new(token.to_owned())),
    )
}

#[test]
fn gateway_root_trust_accepts_configured_ca_and_refuses_unconfigured_roots() {
    let configured = Server::start(server_config(SERVER, SERVER_KEY), |request| {
        assert_eq!(request.bearer(), Some("repair-bearer"));
        assert_eq!(request.path, "/boundary");
        http::json(200, &json!({"verified": request.peer_verified}))
    });
    let unrelated = Server::start(server_config(OTHER_SERVER, OTHER_SERVER_KEY), |_| {
        panic!("unconfigured root reached HTTP")
    });
    let endpoint = |port| required(Endpoint::parse(&format!("https://localhost:{port}")));
    let client = GatewayClient::new(required(Certificate::from_der(CA)), identity(IDENTITY));
    let request = OutboundRequest {
        method: "GET",
        path: "/boundary",
        idempotency: None,
        content_type: "application/json",
        body: b"",
    };
    let response = required(client.request(&endpoint(configured.port), "repair-bearer", &request));
    assert_eq!(response.status, 200);
    assert_eq!(
        required(serde_json::from_slice::<Value>(&response.body)),
        json!({"verified":true})
    );
    assert!(client
        .request(&endpoint(unrelated.port), "repair-bearer", &request)
        .is_err());
    let wrong_ca = GatewayClient::new(
        required(Certificate::from_der(OTHER_CA)),
        identity(IDENTITY),
    );
    assert!(wrong_ca
        .request(&endpoint(configured.port), "repair-bearer", &request)
        .is_err());
    let wrong_identity = GatewayClient::new(
        required(Certificate::from_der(CA)),
        identity(OTHER_IDENTITY),
    );
    assert!(wrong_identity
        .request(&endpoint(configured.port), "repair-bearer", &request)
        .is_err());
    let mut wrong_name = endpoint(configured.port);
    wrong_name.host = "127.0.0.1".to_owned();
    assert!(client
        .request(&wrong_name, "repair-bearer", &request)
        .is_err());
}

#[test]
fn journal_torn_tail_reopens_every_interrupted_append_and_preserves_prior_records() {
    let root = Directory::new();
    let emitted = root.0.join("emitted");
    let mut journal = required(Journal::open::<Record>(&emitted, |_| {}));
    let first = observation("journal", 1).record(PRINCIPAL.to_owned());
    let second = observation("journal", 2).record(PRINCIPAL.to_owned());
    required(journal.append(&first));
    let prefix = required(fs::read(emitted.join("journal.log")));
    required(journal.append(&second));
    drop(journal);
    let complete = required(fs::read(emitted.join("journal.log")));
    let suffix = &complete[prefix.len()..];
    let recovery = root.0.join("recovery");
    required(fs::create_dir(&recovery));
    for interrupted_after in 0..suffix.len() {
        required(fs::write(
            recovery.join("journal.log"),
            &complete[..prefix.len() + interrupted_after],
        ));
        let mut records = Vec::new();
        let mut reopened = required(Journal::open::<Record>(&recovery, |record| {
            records.push(record);
        }));
        assert_eq!(records, vec![first.clone()], "cut at {interrupted_after}");
        assert_eq!(reopened.len(), 1);
        assert_eq!(required(fs::read(recovery.join("journal.log"))), prefix);
        required(reopened.append(&second));
        drop(reopened);
        let mut records = Vec::new();
        let reopened = required(Journal::open::<Record>(&recovery, |record| {
            records.push(record);
        }));
        assert_eq!(records, vec![first.clone(), second.clone()]);
        assert_eq!(reopened.len(), 2);
        drop(reopened);
    }
    let oversized = vec![b'x'; 64 * 1024];
    for bad in [b"not json\n".as_slice(), b"{}\n", oversized.as_slice()] {
        let mut corrupt = prefix.clone();
        corrupt.extend_from_slice(bad);
        required(fs::write(recovery.join("journal.log"), &corrupt));
        assert!(Journal::open::<Record>(&recovery, |_| {}).is_err());
        assert_eq!(required(fs::read(recovery.join("journal.log"))), corrupt);
    }
    // An interior missing delimiter is corruption, even with a valid final line.
    let mut corrupt = complete[..complete.len() - 1].to_vec();
    corrupt.extend_from_slice(suffix);
    required(fs::write(recovery.join("journal.log"), &corrupt));
    assert!(Journal::open::<Record>(&recovery, |_| {}).is_err());
}

#[test]
fn secret_prefix_truncation_refuses_hidden_suffixes_before_normalization() {
    let root = Directory::new();
    let path = root.0.join("credential");
    let maximum = "s".repeat(MAX_SECRET_BYTES);
    for ending in ["", "\n", "\r\n"] {
        required(fs::write(&path, format!("{maximum}{ending}")));
        assert_eq!(required(read_secret_file(&path)).as_str(), maximum);
    }
    for suffix in [
        b"\nX".as_slice(),
        b"\ntrailing-hidden-credential",
        b"\r\nX",
        b"X",
        b"\0",
        b"\n\xff",
        b"\n\n\n",
    ] {
        let mut bytes = maximum.as_bytes().to_vec();
        bytes.extend_from_slice(suffix);
        required(fs::write(&path, bytes));
        assert!(
            read_secret_file(&path).is_err(),
            "accepted suffix {suffix:?}"
        );
    }
    for refused in [
        b"".as_slice(),
        b"\r\n",
        b"abc\0def",
        b"abc\xff",
        b"abc\ndef",
    ] {
        required(fs::write(&path, refused));
        assert!(read_secret_file(&path).is_err());
    }
    required(fs::write(&path, b"fixture-token\r\n"));
    assert_eq!(required(read_secret_file(&path)).as_str(), "fixture-token");
}

fn observation(resource: &str, source_sequence: u64) -> Observation {
    Observation {
        kind: "journey".to_owned(),
        id: layerx_platform_internal::producer::event_id("journey", resource, source_sequence),
        principal: Some(PRINCIPAL.to_owned()),
        principal_digest: None,
        resource: resource.to_owned(),
        sequence: source_sequence,
        source_sequence,
        occurred_at: 123,
        facts: vec![Fact {
            name: "state".to_owned(),
            value: format!("delivered-{source_sequence}"),
        }],
        activity_id: None,
        amount: None,
        asset: None,
    }
}

#[derive(Serialize, Deserialize)]
enum QueueEntry {
    Enqueue {
        transition: String,
        observation: Box<Observation>,
    },
    Acknowledge {
        id: String,
        observed: bool,
    },
    Compact {
        resource: String,
    },
}
impl QueueEntry {
    fn apply(&self, state: &mut QueueState) {
        match self {
            Self::Enqueue {
                transition,
                observation,
            } => assert_eq!(
                required(state.enqueue(transition, (**observation).clone())),
                **observation
            ),
            Self::Acknowledge { id, observed } => required(state.acknowledge(id, *observed)),
            Self::Compact { resource } => required(state.compact_through("journey", resource)),
        }
    }
}
#[derive(Serialize, Deserialize)]
struct Checkpoint {
    records: usize,
    state: QueueState,
}
struct QueueStore {
    state: QueueState,
    journal: Journal,
    directory: PathBuf,
}
impl QueueStore {
    fn open(path: &Path) -> Self {
        let mut entries = Vec::new();
        let journal = required(Journal::open::<QueueEntry>(path, |entry| {
            entries.push(entry);
        }));
        let checkpoint = path.join("checkpoint.json");
        let Checkpoint { records, mut state } = if checkpoint.exists() {
            required(serde_json::from_slice(&required(fs::read(checkpoint))))
        } else {
            Checkpoint {
                records: 0,
                state: QueueState::default(),
            }
        };
        assert!(records <= entries.len());
        for entry in &entries[records..] {
            entry.apply(&mut state);
        }
        required(state.validate());
        Self {
            state,
            journal,
            directory: path.to_path_buf(),
        }
    }
    fn checkpoint(&self) {
        let bytes = required(serde_json::to_vec(&Checkpoint {
            records: self.journal.len(),
            state: self.state.clone(),
        }));
        let temporary = self.directory.join("checkpoint.tmp");
        let mut file = required(fs::File::create(&temporary));
        required(file.write_all(&bytes));
        required(file.sync_all());
        required(fs::rename(
            temporary,
            self.directory.join("checkpoint.json"),
        ));
        required(required(fs::File::open(&self.directory)).sync_all());
    }
    fn enqueue(&mut self, transition: &str, observation: Observation) -> Observation {
        let emitted = required(self.state.enqueue(transition, observation));
        required(self.journal.append(&QueueEntry::Enqueue {
            transition: transition.to_owned(),
            observation: Box::new(emitted.clone()),
        }));
        emitted
    }
    fn acknowledge(&mut self, id: &str, observed: bool) {
        let command = QueueEntry::Acknowledge {
            id: id.to_owned(),
            observed,
        };
        command.apply(&mut self.state);
        required(self.journal.append(&command));
    }
    fn compact(&mut self, resource: &str) {
        let command = QueueEntry::Compact {
            resource: resource.to_owned(),
        };
        command.apply(&mut self.state);
        required(self.journal.append(&command));
    }
}

struct DeliveryPath {
    client: Client,
    reader: Upstream,
    _notifications: Server,
    _source: Server,
    _authority: Server,
}
impl DeliveryPath {
    fn open(root: &Path) -> Self {
        let config = server_config(SERVER, SERVER_KEY);
        let authority = Server::start(Arc::clone(&config), |request| {
            assert_eq!(request.path, "/internal/v1/principal");
            assert_eq!(
                request.headers.get("cookie").map(String::as_str),
                Some("__Host-layerx_access=repair-session")
            );
            http::json(
                200,
                &json!({"ok":true,"result":{"active":true,"sub":PRINCIPAL}}),
            )
        });
        let service = required(
            required(Service::open(
                Kind::Journey,
                upstream(authority.port, "unused"),
                &root.join("snapshot.json"),
                ENROLLMENT_KEY,
                Zeroizing::new("reader-token".to_owned()),
                &root.join("source"),
            ))
            .with_producers(vec![ProducerCredential {
                token: Zeroizing::new("producer-token".to_owned()),
                allow_principal_digest: false,
            }]),
        );
        required(service.refresh());
        let source = Server::start(Arc::clone(&config), move |request| service.route(request));
        let reader = upstream(source.port, "reader-token");
        let notification_reader = upstream(source.port, "reader-token");
        let mut received = BTreeMap::<String, Record>::new();
        let journal = required(Journal::open::<Record>(
            &root.join("notifications"),
            |record| {
                received.insert(record.id.clone(), record);
            },
        ));
        let inbox = Mutex::new((journal, received));
        let notifications = Server::start(config, move |request| {
            assert!(request.bearer_matches("notification-token"));
            let id = request
                .path
                .strip_prefix("/internal/v1/events/journey/")
                .unwrap_or_else(|| panic!("notification path"));
            let response = required(notification_reader.get(&format!("/internal/v1/events/{id}")));
            assert_eq!(response.status, 200);
            let record: Record = required(serde_json::from_slice(&response.body));
            assert_eq!(record.id, id);
            let (journal, records) = &mut *required(inbox.lock());
            let duplicate = records.contains_key(id);
            if let Some(prior) = records.get(id) {
                assert_eq!(prior, &record);
            } else {
                required(journal.append(&record));
                records.insert(id.to_owned(), record);
            }
            http::json(
                202,
                &json!({"position":records.len(), "duplicate":duplicate, "queued":[]}),
            )
        });
        let client = required(Client::new(
            BTreeMap::from([(
                "journey".to_owned(),
                upstream(source.port, "producer-token"),
            )]),
            upstream(notifications.port, "notification-token"),
        ));
        Self {
            client,
            reader,
            _notifications: notifications,
            _source: source,
            _authority: authority,
        }
    }
    fn deliver(&self, store: &mut QueueStore) -> Record {
        let pending = store
            .state
            .pending()
            .unwrap_or_else(|| panic!("queue empty"));
        if !pending.observed {
            assert!(required(self.client.deliver(&pending)));
            store.acknowledge(&pending.observation.id, true);
        }
        let pending = store
            .state
            .pending()
            .unwrap_or_else(|| panic!("queue empty"));
        assert!(!required(self.client.deliver(&pending)));
        let response = required(
            self.reader
                .get(&format!("/internal/v1/events/{}", pending.observation.id)),
        );
        assert_eq!(response.status, 200);
        let record = required(serde_json::from_slice::<Record>(&response.body));
        assert_eq!(record, pending.observation.record(PRINCIPAL.to_owned()));
        store.acknowledge(&record.id, false);
        record
    }
}

fn enrollment(root: &Path) {
    let credential = root.join("session");
    required(fs::write(&credential, "repair-session"));
    required(fs::set_permissions(
        &credential,
        fs::Permissions::from_mode(0o600),
    ));
    required(fs::write(
        root.join("snapshot.json"),
        required(serde_json::to_vec(&json!({
            "version":1,"generation":1,"principals":[{"principal":PRINCIPAL,"credential_file":credential}],
            "mac":enrollment_snapshot_mac(Kind::Journey, 1, &[(PRINCIPAL,"repair-session")], ENROLLMENT_KEY)
        }))),
    ));
    required(fs::set_permissions(
        root.join("snapshot.json"),
        fs::Permissions::from_mode(0o600),
    ));
}

#[test]
fn producer_retention_growth_sustains_delivery_and_reopens_at_durable_boundaries() {
    let root = Directory::new();
    enrollment(&root.0);
    let mut store = QueueStore::open(&root.0.join("producer"));
    let path = DeliveryPath::open(&root.0);
    let deliveries = MAX_RETAINED_OBSERVATIONS + 8;
    let mut first = None;
    for number in 1..=deliveries as u64 {
        let emitted = store.enqueue(&format!("live-{number}"), observation("live", number));
        if number == 1 {
            first = Some(emitted.clone());
        }
        assert_eq!(path.deliver(&mut store).subject_sequence, number);
    }
    required(store.state.validate());
    let state = required(serde_json::to_value(&store.state));
    assert_eq!(
        state["retained"].as_object().map(serde_json::Map::len),
        Some(MAX_RETAINED_OBSERVATIONS)
    );
    assert_eq!(
        state["counters"].as_object().map(serde_json::Map::len),
        Some(1)
    );
    let expired = first.unwrap_or_else(|| panic!("first observation missing"));
    assert!(store.state.enqueue("live-1", expired.clone()).is_err());
    assert!(store.state.enqueue("renamed-expired", expired).is_err());
    let recent = observation("live", deliveries as u64);
    assert_eq!(
        required(
            store
                .state
                .enqueue(&format!("live-{deliveries}"), recent.clone())
        )
        .sequence,
        deliveries as u64
    );
    let mut changed = recent;
    changed.facts[0].value = "substituted".to_owned();
    assert!(store
        .state
        .enqueue(&format!("live-{deliveries}"), changed)
        .is_err());
    let last = store.enqueue(
        "pending-at-restart",
        observation("live", deliveries as u64 + 1),
    );
    let pending = store
        .state
        .pending()
        .unwrap_or_else(|| panic!("pending missing"));
    assert!(required(path.client.deliver(&pending)));
    store.acknowledge(&last.id, true);
    assert!(store.state.compact_through("journey", "live").is_err());
    let saved = required(serde_json::to_vec(&store.state));
    store.checkpoint();
    drop(store);
    drop(path);

    let mut store = QueueStore::open(&root.0.join("producer"));
    assert_eq!(required(serde_json::to_vec(&store.state)), saved);
    assert_eq!(
        store
            .state
            .pending()
            .map(|entry| (entry.observation, entry.observed)),
        Some((last.clone(), true))
    );
    let path = DeliveryPath::open(&root.0);
    assert_eq!(path.deliver(&mut store).subject_sequence, last.sequence);
    store.compact("live");
    store.checkpoint();
    required(store.state.validate());
    let state = required(serde_json::to_value(&store.state));
    for field in ["retained", "counters", "compacted"] {
        assert_eq!(state[field].as_object().map(serde_json::Map::len), Some(0));
    }
    drop(store);
    let mut store = QueueStore::open(&root.0.join("producer"));
    assert!(store.state.enqueue("closed-retry", last).is_err());
    assert!(store
        .state
        .enqueue("closed-new", observation("live", deliveries as u64 + 2))
        .is_err());
    assert!(store.state.compact_through("journey", "aaa").is_err());
    let next = store.enqueue("next-resource", observation("next", 1));
    assert_eq!(path.deliver(&mut store), next.record(PRINCIPAL.to_owned()));
    drop(path);
    let mut delivered = Vec::new();
    let journal = required(Journal::open::<Record>(
        &root.0.join("notifications"),
        |record| delivered.push(record),
    ));
    assert_eq!(journal.len(), deliveries + 2);
    assert_eq!(delivered.last(), Some(&next.record(PRINCIPAL.to_owned())));
}

#[test]
fn producer_retention_growth_bounds_counters_and_compacts_only_closed_acknowledged_streams() {
    let root = Directory::new();
    let mut store = QueueStore::open(&root.0.join("producer"));
    for number in 0..MAX_TRACKED_RESOURCES {
        let resource = format!("resource-{number:05}");
        let entry = store.enqueue(&resource, observation(&resource, 1));
        store.acknowledge(&entry.id, true);
        store.acknowledge(&entry.id, false);
    }
    let before = required(serde_json::to_vec(&store.state));
    assert!(store
        .state
        .enqueue("over-capacity", observation("resource-next", 1))
        .is_err());
    assert_eq!(required(serde_json::to_vec(&store.state)), before);
    required(store.state.validate());
    store.compact("resource-09999");
    drop(store);
    let mut store = QueueStore::open(&root.0.join("producer"));
    required(store.state.validate());
    assert!(store
        .state
        .enqueue("expired", observation("resource-00000", 2))
        .is_err());
    let next = store.enqueue("next", observation("resource-next", 1));
    assert_eq!(next.sequence, 1);
    assert_eq!(
        required(serde_json::to_value(&store.state))["counters"]
            .as_object()
            .map(serde_json::Map::len),
        Some(1)
    );
}
