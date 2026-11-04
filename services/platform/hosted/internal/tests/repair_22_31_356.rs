extern crate self as layerx_platform_identity;

#[path = "../../identity/src/mailer.rs"]
pub mod mailer;
#[path = "../../registry/src/event_producer.rs"]
mod registry_producer;
#[path = "../../identity/src/seal.rs"]
pub mod seal;
#[path = "../../identity/src/signup.rs"]
pub mod signup;
#[path = "../../identity/src/store.rs"]
pub mod store;
#[path = "../src/producer_test_support.rs"]
mod transport;
#[path = "../../identity/src/verification.rs"]
pub mod verification;

use layerx_platform_internal::{
    events::{enrollment_snapshot_mac, Kind, ProducerCredential, Record, Service},
    http::{self, Request, Response},
    journal::Journal,
    producer::{
        Client, DeliveryFailure, DeliveryState, Health, Outbox, Pending, QueueState,
        MAX_DELIVERY_ATTEMPTS,
    },
    secret::{random_hex, sha256_hex, unix_seconds},
    tls::{Origin, Upstream},
};
use registry_producer::ProgramOutbox;
use serde_json::{json, Value};
use std::collections::BTreeMap;
use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};
use zeroize::Zeroizing;

const PRINCIPAL: &str = "program-principal";
const ENROLLMENT_KEY: &str = "0123456789abcdef0123456789abcdef";
const PRODUCER_TOKEN: &str = "producer-token";
const READER_TOKEN: &str = "consumer-token";
const NOTIFICATION_TOKEN: &str = "notification-token";

fn required<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "repair-22-31-356-{}",
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

fn protected(path: &Path, contents: &[u8]) {
    required(fs::write(path, contents));
    required(fs::set_permissions(path, fs::Permissions::from_mode(0o600)));
}

fn listener(
    tls: &transport::Tls,
    port: u16,
    handler: impl Fn(&Request) -> Response + Send + 'static,
) -> transport::Listener {
    transport::Listener::start(Arc::clone(&tls.config), port, move |stream| {
        if let Ok(mut request) = http::parse_client_request(stream) {
            request.peer_verified = stream
                .conn
                .peer_certificates()
                .is_some_and(|chain| !chain.is_empty());
            required(http::write_response(stream, &handler(&request)));
        }
    })
}

// Include the production identity authority so enrollment binds to a real,
// durably provisioned/revoked publication key rather than a canned identity.
mod publication_authority {
    include!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../identity/src/main.rs"
    ));

    pub struct Authority(Shared);
    impl Authority {
        pub fn new(directory: &Path, tls: Arc<ServerConfig>) -> Self {
            Self(Shared {
                config: Config {
                    listen: "127.0.0.1:0"
                        .parse()
                        .unwrap_or_else(|error| panic!("{error}")),
                    health: "127.0.0.1:0"
                        .parse()
                        .unwrap_or_else(|error| panic!("{error}")),
                    tls,
                    state_dir: directory.to_path_buf(),
                    store_key: StoreKey::derive(b"repair-enrollment-authority"),
                    service_tokens: vec![ServiceToken {
                        service: Service::Registry,
                        token: Zeroizing::new("registry-repair-authority-token".to_owned()),
                    }],
                    default_ttl_seconds: 60,
                    signup_limits: Limits::default(),
                    disposable_domains: Vec::new(),
                    outbox_dir: directory.join("mail"),
                    smtp: None,
                },
                store: Mutex::new(Store::open(directory).unwrap_or_else(|error| panic!("{error}"))),
                desk: Mutex::new(Desk::new(Limits::default(), Vec::new())),
            })
        }

        pub fn provision(&self, principal: &str, key: &str) {
            let mut store = self
                .0
                .store
                .lock()
                .unwrap_or_else(|error| panic!("{error}"));
            store
                .put_principal(Principal {
                    tenant: "repair".to_owned(),
                    sub: principal.to_owned(),
                    allowed_signer_public_keys: vec!["ab".repeat(32)],
                    account: None,
                    audiences: Vec::new(),
                })
                .unwrap_or_else(|error| panic!("{error}"));
            store
                .bind_publication_key(sha256_hex(key.as_bytes()), principal.to_owned(), false)
                .unwrap_or_else(|error| panic!("{error}"));
        }

        pub fn revoke(&self, principal: &str, key: &str) {
            self.0
                .store
                .lock()
                .unwrap_or_else(|error| panic!("{error}"))
                .bind_publication_key(sha256_hex(key.as_bytes()), principal.to_owned(), true)
                .unwrap_or_else(|error| panic!("{error}"));
        }

        pub fn route(
            &self,
            incoming: &layerx_platform_internal::http::Request,
        ) -> layerx_platform_internal::http::Response {
            // Adapt the gateway's principal credential header to the identity
            // authority's service-authenticated publication-key lookup.
            let Some(key) = incoming
                .headers
                .get("authorization")
                .and_then(|value| value.strip_prefix("LayerX-Key "))
            else {
                return layerx_platform_internal::http::refusal(401, "unauthorized", None);
            };
            if !incoming.peer_verified {
                return layerx_platform_internal::http::refusal(401, "unauthorized", None);
            }
            let request = Request {
                method: incoming.method.clone(),
                path: incoming.path.clone(),
                headers: BTreeMap::from([
                    (
                        "authorization".to_owned(),
                        "Bearer registry-repair-authority-token".to_owned(),
                    ),
                    ("layerx-key".to_owned(), key.to_owned()),
                ]),
                body: incoming.body.clone(),
            };
            let response = route(&self.0, &request);
            layerx_platform_internal::http::Response {
                status: response.status,
                body: response.body,
                retry_after: response.retry_after,
            }
        }
    }
}

fn snapshot(root: &Path, kind: Kind, generation: u64, entries: &[(&str, &str)]) {
    let principals: Vec<Value> = entries
        .iter()
        .enumerate()
        .map(|(index, (principal, credential))| {
            let path = root.join(format!("credential-{generation}-{index}"));
            protected(&path, credential.as_bytes());
            json!({"principal": principal, "credential_file": path})
        })
        .collect();
    protected(
        &root.join("snapshot.json"),
        &required(serde_json::to_vec(&json!({
            "version": 1, "generation": generation, "principals": principals,
            "mac": enrollment_snapshot_mac(kind, generation, entries, ENROLLMENT_KEY),
        }))),
    );
}

fn source(tls: &transport::Tls, root: &Path, kind: Kind, authority_port: u16) -> Arc<Service> {
    let service = Arc::new(required(
        required(Service::open(
            kind,
            tls.upstream(authority_port, "unused"),
            &root.join("snapshot.json"),
            ENROLLMENT_KEY,
            Zeroizing::new(READER_TOKEN.to_owned()),
            &root.join("source"),
        ))
        .with_producers(vec![
            ProducerCredential {
                token: Zeroizing::new(PRODUCER_TOKEN.to_owned()),
                allow_principal_digest: true,
            },
            ProducerCredential {
                token: Zeroizing::new("principal-only-producer".to_owned()),
                allow_principal_digest: false,
            },
        ]),
    ));
    required(service.refresh());
    service
}

fn serve_source(tls: &transport::Tls, port: u16, service: &Arc<Service>) -> transport::Listener {
    let service = Arc::clone(service);
    listener(tls, port, move |request| service.route(request))
}

fn publication(outbox: &ProgramOutbox, version: u64) -> Pending {
    // This is the registry's real publication-to-observation producer, not a
    // separately assembled Observation or wire payload.
    let body = json!({
        "program_id": "cd".repeat(32), "lifecycle": "active",
        "versions": [{ "version": version, "code_hash": "ab".repeat(32),
            "deployment_receipt_digest": "ef".repeat(32) }],
    });
    required(outbox.enqueue_publication(
        &body.to_string(),
        &sha256_hex(PRINCIPAL.as_bytes()),
        required(unix_seconds()),
    ));
    required(outbox.pending()).unwrap_or_else(|| panic!("publication not queued"))
}

fn get_record(reader: &Upstream, id: &str) -> Record {
    let response = required(reader.get(&format!("/internal/v1/events/{id}")));
    assert_eq!(
        response.status,
        200,
        "{}",
        String::from_utf8_lossy(&response.body)
    );
    required(serde_json::from_slice(&response.body))
}

// A durable reference consumer: it retrieves each immutable record through the
// real source reader, then fsyncs it before acknowledging the notification.
struct Inbox {
    source: Upstream,
    journal: Journal,
    records: Vec<Record>,
}
impl Inbox {
    fn open(root: &Path, source: Upstream) -> Self {
        let mut records = Vec::new();
        let journal = required(Journal::open(root, |record: Record| records.push(record)));
        Self {
            source,
            journal,
            records,
        }
    }

    fn route(&mut self, request: &Request) -> Response {
        if !request.peer_verified || !request.bearer_matches(NOTIFICATION_TOKEN) {
            return http::refusal(401, "unauthorized", None);
        }
        if request.method == "GET" && request.path == "/internal/v1/readiness/program" {
            let response = required(self.source.get("/internal/v1/reader-readiness"));
            let status: Value = required(serde_json::from_slice(&response.body));
            return http::json(
                response.status,
                &json!({
                    "schema": "layerx.event-admission.v1", "role": "webhook-trigger",
                    "kind": "program", "ready": status["ready"],
                    "generation": status["generation"], "principals": status["principals"],
                    "principal_digest": false, "fresh_for_ms": 10_000,
                }),
            );
        }
        if request.method != "POST" || !request.json_body() || request.body != b"{}" {
            return http::refusal(400, "invalid_notification", None);
        }
        let Some(id) = request.path.strip_prefix("/internal/v1/events/program/") else {
            return http::refusal(404, "not_found", None);
        };
        let record = get_record(&self.source, id);
        let duplicate = self
            .records
            .iter()
            .position(|previous| previous.id == record.id);
        let position = if let Some(position) = duplicate {
            assert_eq!(self.records[position], record);
            position + 1
        } else {
            required(self.journal.append(&record));
            self.records.push(record);
            self.records.len()
        };
        http::json(
            202,
            &json!({"position": position, "duplicate": duplicate.is_some(), "queued": []}),
        )
    }
}

// Persist the actual QueueState after every operation. Scheduling uses wall
// time and the real retry bound, including after close/reopen.
struct DurableQueue(Mutex<(Journal, QueueState)>);
impl DurableQueue {
    fn open(root: &Path) -> Self {
        let mut state = QueueState::default();
        let journal = required(Journal::open(root, |replayed: QueueState| state = replayed));
        required(state.validate());
        Self(Mutex::new((journal, state)))
    }
    fn update<T>(
        &self,
        action: impl FnOnce(&mut QueueState) -> Result<T, String>,
    ) -> Result<T, String> {
        let mut locked = self.0.lock().map_err(|error| error.to_string())?;
        let (journal, state) = &mut *locked;
        let mut next = state.clone();
        let result = action(&mut next)?;
        next.validate()?;
        journal.append(&next)?;
        *state = next;
        Ok(result)
    }
    fn enqueue(&self, transition: &str, pending: &Pending) {
        let emitted =
            required(self.update(|state| state.enqueue(transition, pending.observation.clone())));
        assert_eq!(emitted, pending.observation);
    }
    fn state(&self) -> QueueState {
        required(self.0.lock()).1.clone()
    }
    fn wait_eligible(&self) {
        let deadline = Instant::now() + Duration::from_secs(7);
        while !self.state().delivery().eligible(required(unix_seconds())) {
            assert!(
                Instant::now() < deadline,
                "delivery did not become eligible"
            );
            std::thread::sleep(Duration::from_millis(20));
        }
    }
}
impl Outbox for DurableQueue {
    fn pending(&self) -> Result<Option<Pending>, String> {
        self.0
            .lock()
            .map(|state| state.1.pending())
            .map_err(|error| error.to_string())
    }
    fn select(&self) -> Result<Option<Pending>, String> {
        self.update(|state| {
            if !state.delivery().eligible(unix_seconds()?) {
                return Ok(None);
            }
            let pending = state.pending();
            if pending.is_some() {
                state.selected(state.delivery().selected_turn + 1)?;
            }
            Ok(pending)
        })
    }
    fn scheduling(&self, id: &str) -> Result<Option<DeliveryState>, String> {
        let state = self.state();
        if state
            .pending()
            .is_none_or(|pending| pending.observation.id != id)
        {
            return Err("scheduling outside queue order".to_owned());
        }
        Ok(Some(state.delivery().clone()))
    }
    fn acknowledge(&self, id: &str, observed: bool) -> Result<(), String> {
        self.update(|state| state.acknowledge(id, observed))
    }
    fn failed_delivery(
        &self,
        id: &str,
        generation: Option<u64>,
        reason: DeliveryFailure,
    ) -> Result<(), String> {
        self.update(|state| state.failed_delivery(id, generation, unix_seconds()?, reason))
    }
    fn resume_delivery(&self, id: &str, generation: u64) -> Result<(), String> {
        self.update(|state| state.resume_delivery(id, generation))
    }
}

fn client(tls: &transport::Tls, source_port: u16, inbox_port: u16) -> Client {
    required(Client::new(
        BTreeMap::from([(
            "program".to_owned(),
            tls.upstream(source_port, PRODUCER_TOKEN),
        )]),
        tls.upstream(inbox_port, NOTIFICATION_TOKEN),
    ))
}

#[test]
fn program_source_upstream_retires_pull_and_delivers_registry_observations() {
    let root = Directory::new();
    let tls = transport::Tls::new(&root.0);
    let authority = Arc::new(publication_authority::Authority::new(
        &root.0.join("identity"),
        Arc::clone(&tls.config),
    ));
    authority.provision(PRINCIPAL, "program-key:first");
    let served = Arc::clone(&authority);
    let upstream = listener(&tls, 0, move |request| served.route(request));
    snapshot(
        &root.0,
        Kind::Program,
        1,
        &[(PRINCIPAL, "program-key:first")],
    );
    let source = source(&tls, &root.0, Kind::Program, upstream.port);
    let server = serve_source(&tls, 0, &source);
    let reader = tls.upstream(server.port, READER_TOKEN);
    let pull = required(reader.post(
        "/internal/v1/observe",
        &required(serde_json::to_vec(&json!({
            "principal": PRINCIPAL, "resource": "cd".repeat(32),
        }))),
    ));
    assert_eq!(pull.status, 410);
    assert_eq!(
        required::<Value, _>(serde_json::from_slice(&pull.body))["error"]["code"],
        "program_pull_retired"
    );

    let outbox = ProgramOutbox::new(&root.0.join("registry"));
    let pending = publication(&outbox, 7);
    assert!(pending.observation.principal.is_none());
    let inbox = Arc::new(Mutex::new(Inbox::open(
        &root.0.join("inbox"),
        tls.upstream(server.port, READER_TOKEN),
    )));
    let received = Arc::clone(&inbox);
    let consumer = listener(&tls, 0, move |request| {
        required(received.lock()).route(request)
    });
    let client = client(&tls, server.port, consumer.port);
    required(client.step(&outbox, &outbox.health));
    let record = get_record(&reader, &pending.observation.id);
    assert_eq!(record, pending.observation.record(PRINCIPAL.to_owned()));
    required(client.step(&outbox, &outbox.health));
    assert!(required(outbox.pending()).is_none());
    assert_eq!(required(inbox.lock()).records, [record]);

    // A producer without digest authority cannot use digest observations.
    let unauthorized = required(
        tls.upstream(server.port, "principal-only-producer")
            .post("/internal/v1/observe", &pending.body),
    );
    assert_eq!(unauthorized.status, 403);
    authority.revoke(PRINCIPAL, "program-key:first");
    assert_eq!(
        required(reader.get(&format!("/internal/v1/events/{}", pending.observation.id))).status,
        503
    );
    assert_eq!(
        required(
            tls.upstream(server.port, PRODUCER_TOKEN)
                .post("/internal/v1/observe", &pending.body)
        )
        .status,
        403
    );
}

#[test]
fn digest_producer_recovery_reenrolls_and_replays_after_restart() {
    let root = Directory::new();
    let tls = transport::Tls::new(&root.0);
    let authority = Arc::new(publication_authority::Authority::new(
        &root.0.join("identity"),
        Arc::clone(&tls.config),
    ));
    authority.provision(PRINCIPAL, "program-key:first");
    let served = Arc::clone(&authority);
    let upstream = listener(&tls, 0, move |request| served.route(request));
    snapshot(
        &root.0,
        Kind::Program,
        1,
        &[(PRINCIPAL, "program-key:first")],
    );
    let source = source(&tls, &root.0, Kind::Program, upstream.port);
    let server = serve_source(&tls, 0, &source);
    let source_port = server.port;
    let digest = sha256_hex(PRINCIPAL.as_bytes());
    let status_path = format!("/internal/v1/principal-digests/{digest}/enrollment");
    let query = tls.upstream(source_port, PRODUCER_TOKEN);
    let response = required(query.get(&status_path));
    assert_eq!(response.status, 200);
    assert_eq!(
        required::<Value, _>(serde_json::from_slice(&response.body)),
        json!({
            "kind": "program", "principal_digest": digest, "generation": 1, "bound": true,
        })
    );
    assert!(!String::from_utf8_lossy(&response.body).contains(PRINCIPAL));
    let raw_status =
        required(query.get(&format!("/internal/v1/principals/{PRINCIPAL}/enrollment")));
    assert_eq!(raw_status.status, 200);
    assert_eq!(
        required::<Value, _>(serde_json::from_slice(&raw_status.body)),
        json!({
            "principal": PRINCIPAL, "generation": 1, "bound": true,
        })
    );
    for token in [READER_TOKEN, "principal-only-producer", "wrong-token"] {
        assert_eq!(
            required(tls.upstream(source_port, token).get(&status_path)).status,
            401
        );
    }
    let anonymous = Upstream::new(
        required(Origin::parse(&format!("https://localhost:{source_port}"))),
        tls.ca.clone(),
        None,
        Some(Zeroizing::new(PRODUCER_TOKEN.to_owned())),
    );
    assert_eq!(required(anonymous.get(&status_path)).status, 401);
    assert_eq!(
        required(query.get("/internal/v1/principal-digests/not-a-digest/enrollment")).status,
        400
    );
    let unknown = required(query.get(&format!(
        "/internal/v1/principal-digests/{}/enrollment",
        "00".repeat(32)
    )));
    assert_eq!(unknown.status, 200);
    assert_eq!(
        required::<Value, _>(serde_json::from_slice(&unknown.body))["bound"],
        false
    );

    let registry = ProgramOutbox::new(&root.0.join("registry"));
    let first = publication(&registry, 7);
    required(registry.acknowledge(&first.observation.id, true));
    required(registry.acknowledge(&first.observation.id, false));
    let second = publication(&registry, 8);
    assert_eq!(second.observation.sequence, 2);
    let queue_path = root.0.join("scheduled-outbox");
    let queue = DurableQueue::open(&queue_path);
    queue.enqueue("first-publication", &first);
    queue.enqueue("second-publication", &second);
    let inbox_path = root.0.join("inbox");
    let inbox = Arc::new(Mutex::new(Inbox::open(
        &inbox_path,
        tls.upstream(source_port, READER_TOKEN),
    )));
    let received = Arc::clone(&inbox);
    let consumer = listener(&tls, 0, move |request| {
        required(received.lock()).route(request)
    });
    let inbox_port = consumer.port;
    let client = client(&tls, source_port, inbox_port);
    let health = Health::default();
    required(client.step(&queue, &health));
    assert!(
        required(queue.pending())
            .unwrap_or_else(|| panic!("head missing"))
            .observed
    );
    drop(consumer);
    drop(inbox);
    for _ in 0..MAX_DELIVERY_ATTEMPTS {
        queue.wait_eligible();
        assert!(client.step(&queue, &health).is_err());
    }
    let exhausted = queue.state();
    assert_eq!(exhausted.delivery().attempts, MAX_DELIVERY_ATTEMPTS);
    assert!(exhausted.delivery().redelivery_required);
    assert_eq!(exhausted.delivery().enrollment_generation, Some(1));
    assert_eq!(
        exhausted.delivery().last_refusal,
        Some(DeliveryFailure::NotificationUnavailable)
    );
    assert_eq!(exhausted.pending_count(), 2);
    drop(client);
    drop(queue);
    drop(server);
    drop(source);
    drop(registry);

    // Reopen both sides: replay the immutable source journal, the adopted
    // enrollment generation, and the exhausted producer schedule and bytes.
    let source = self::source(&tls, &root.0, Kind::Program, upstream.port);
    let server = serve_source(&tls, source_port, &source);
    let queue = DurableQueue::open(&queue_path);
    assert_eq!(queue.state().delivery(), exhausted.delivery());
    assert_eq!(required(queue.pending()), exhausted.pending());
    assert_eq!(
        get_record(
            &tls.upstream(source_port, READER_TOKEN),
            &first.observation.id
        ),
        first.observation.record(PRINCIPAL.to_owned())
    );
    assert_eq!(
        required(
            tls.upstream(source_port, READER_TOKEN)
                .get(&format!("/internal/v1/events/{}", second.observation.id))
        )
        .status,
        404
    );
    let inbox = Arc::new(Mutex::new(Inbox::open(
        &inbox_path,
        tls.upstream(source_port, READER_TOKEN),
    )));
    let received = Arc::clone(&inbox);
    let _consumer = listener(&tls, inbox_port, move |request| {
        required(received.lock()).route(request)
    });
    let client = self::client(&tls, source_port, inbox_port);
    queue.wait_eligible();
    assert_eq!(
        client.step(&queue, &health),
        Err("event redelivery requires authenticated recovery".to_owned())
    );
    assert!(required(inbox.lock()).records.is_empty());

    // Merely incrementing the snapshot must not bypass a revoked binding.
    authority.revoke(PRINCIPAL, "program-key:first");
    let response = required(tls.upstream(source_port, PRODUCER_TOKEN).get(&status_path));
    assert_eq!(
        required::<Value, _>(serde_json::from_slice(&response.body))["bound"],
        false
    );
    snapshot(&root.0, Kind::Program, 2, &[]);
    required(source.refresh());
    queue.wait_eligible();
    assert!(client.step(&queue, &health).is_err());
    assert!(queue.state().delivery().redelivery_required);
    assert_eq!(queue.state().delivery().enrollment_generation, Some(1));

    authority.provision(PRINCIPAL, "program-key:rotated");
    snapshot(
        &root.0,
        Kind::Program,
        3,
        &[(PRINCIPAL, "program-key:rotated")],
    );
    required(source.refresh());
    // Persist the reenrollment, stop the source, and require the reopened
    // source to authenticate it again before producer recovery is accepted.
    drop(server);
    drop(source);
    let source = self::source(&tls, &root.0, Kind::Program, upstream.port);
    let _server = serve_source(&tls, source_port, &source);
    assert_eq!(source.generation().generation(), 3);
    queue.wait_eligible();
    required(client.step(&queue, &health));
    assert_eq!(queue.state().delivery().last_recovery_generation, Some(3));
    assert!(!queue.state().delivery().redelivery_required);
    assert_eq!(queue.state().pending_count(), 1);
    required(client.step(&queue, &health));
    required(client.step(&queue, &health));
    assert!(required(queue.pending()).is_none());
    let records = required(inbox.lock()).records.clone();
    assert_eq!(
        records,
        [
            first.observation.record(PRINCIPAL.to_owned()),
            second.observation.record(PRINCIPAL.to_owned())
        ]
    );
    assert_eq!(
        queue.state().delivery().last_delivered_id.as_deref(),
        Some(second.observation.id.as_str())
    );
    drop(queue);
    assert!(required(DurableQueue::open(&queue_path).pending()).is_none());
    assert!(!required(fs::read_to_string(queue_path.join("journal.log"))).contains(PRINCIPAL));
    drop(_consumer);
    drop(inbox);
    let recovered_inbox = Inbox::open(&inbox_path, tls.upstream(source_port, READER_TOKEN));
    assert_eq!(recovered_inbox.records, records);
}

#[test]
fn digest_enrollment_status_binds_payment_sources_without_disclosing_principals() {
    let root = Directory::new();
    let tls = transport::Tls::new(&root.0);
    let authority = Arc::new(publication_authority::Authority::new(
        &root.0.join("identity"),
        Arc::clone(&tls.config),
    ));
    authority.provision(PRINCIPAL, "payment-key:first");
    let served = Arc::clone(&authority);
    let upstream = listener(&tls, 0, move |request| served.route(request));
    snapshot(
        &root.0,
        Kind::Payment,
        1,
        &[(PRINCIPAL, "payment-key:first")],
    );
    let service = source(&tls, &root.0, Kind::Payment, upstream.port);
    let server = serve_source(&tls, 0, &service);
    let query = tls.upstream(server.port, PRODUCER_TOKEN);
    let digest = sha256_hex(PRINCIPAL.as_bytes());
    let path = format!("/internal/v1/principal-digests/{digest}/enrollment");
    let response = required(query.get(&path));
    assert_eq!(response.status, 200);
    assert_eq!(
        required::<Value, _>(serde_json::from_slice(&response.body)),
        json!({
            "kind": "payment", "principal_digest": digest, "generation": 1, "bound": true,
        })
    );
    assert!(!String::from_utf8_lossy(&response.body).contains(PRINCIPAL));
    authority.revoke(PRINCIPAL, "payment-key:first");
    let response = required(query.get(&path));
    assert_eq!(response.status, 200);
    assert_eq!(
        required::<Value, _>(serde_json::from_slice(&response.body))["bound"],
        false
    );
}
