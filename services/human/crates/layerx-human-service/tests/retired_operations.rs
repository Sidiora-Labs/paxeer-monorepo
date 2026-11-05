use std::fmt::Debug;
use std::fs::{self, DirBuilder};
use std::io::{self, Read as _, Write as _};
use std::os::unix::fs::DirBuilderExt as _;
use std::os::unix::net::UnixStream;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use ciborium::value::Value as CborValue;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_human_service::auth::{AccountIdentity, AuthConfig, Device, Passkeys, RateLimit};
use layerx_human_service::server::schema::{ApiSchema, Operation};
use layerx_human_service::server::{
    default_component_limits, ApiFailure, AuthorizationGrantPolicy, AuthorizedSession,
    BackendResponse, ComponentOperationRequest, ComponentServerConfig, ComponentShutdown,
    HttpConfig, HumanApiComponents, HumanComponentServer, IdentityServices, PrincipalLimits,
    PrivilegedHumanComponents, PrivilegedHumanServices, ProvisionedAccount, ProvisionedAccounts,
    Readiness, Router, ScopedRequest, UnixComponents,
};
use layerx_human_service::store::{PrincipalScope, PrincipalStore};
use layerx_human_test_support::{install_and_open, principal, retention_uniform, tenancy};
use layerx_types::clock::Clock as _;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod clock_server;

const RP_ID: &str = "paxportwallet.com";
const ORIGIN: &str = "https://paxportwallet.com";
const ACCOUNT_ID: &str = "act_00112233445566778899aabbccddeeff";
const TRACE: &str = "trc_00112233445566778899aabbccddeeff";
const FLAG_UP: u8 = 1 << 0;
const FLAG_UV: u8 = 1 << 2;
const FLAG_AT: u8 = 1 << 6;
const RETIRED: [&str; 11] = [
    "binding.rebind",
    "binding.rebind.action",
    "binding.statement",
    "binding.status",
    "binding.submit",
    "deposit.confirm",
    "deposit.start",
    "exit.eligibility",
    "exit.start",
    "withdraw.claim",
    "withdraw.start",
];

fn required<T, E: Debug>(result: Result<T, E>, label: &str) -> T {
    result.unwrap_or_else(|error| panic!("{label}: {error:?}"))
}

struct TestDirectory(PathBuf);

impl TestDirectory {
    fn new() -> io::Result<Self> {
        let mut entropy = [0_u8; 16];
        getrandom::fill(&mut entropy).map_err(|error| io::Error::other(error.to_string()))?;
        let path = std::env::temp_dir().join(format!("hcf-{:032x}", u128::from_be_bytes(entropy)));
        DirBuilder::new().mode(0o700).create(&path)?;
        Ok(Self(path))
    }
}

impl Drop for TestDirectory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

struct RunningClock {
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<io::Result<()>>>,
}

impl RunningClock {
    fn start(root: &Path) -> io::Result<(Self, ClockBinding)> {
        let mut server = clock_server::Server::bind(root)?;
        let binding = ClockBinding {
            socket: server.socket().to_path_buf(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        };
        let shutdown = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&shutdown);
        let worker = thread::spawn(move || {
            while !stop.load(Ordering::Acquire) {
                server.poll()?;
                thread::sleep(Duration::from_millis(1));
            }
            Ok(())
        });
        Ok((
            Self {
                shutdown,
                worker: Some(worker),
            },
            binding,
        ))
    }
}

impl Drop for RunningClock {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

struct SoftwareAuthenticator {
    signing_key: SigningKey,
    credential_id: Vec<u8>,
    counter: u32,
    user_handle: Option<String>,
}

impl SoftwareAuthenticator {
    fn new() -> Self {
        let mut seed = [0_u8; 32];
        required(getrandom::fill(&mut seed), "authenticator entropy");
        let mut credential_id = vec![0_u8; 32];
        required(
            getrandom::fill(&mut credential_id),
            "credential identifier entropy",
        );
        Self {
            signing_key: SigningKey::from_bytes(&seed),
            credential_id,
            counter: 0,
            user_handle: None,
        }
    }

    fn register(&mut self, ceremony: &str) -> String {
        let options = decode_ceremony(ceremony);
        let challenge = required_text(&options, "/challenge");
        self.user_handle = Some(required_text(&options, "/user/id").to_owned());
        let client_data = client_data("webauthn.create", challenge);
        encode_response(&json!({
            "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
            "transports": ["internal"],
            "attestationObject": URL_SAFE_NO_PAD.encode(self.attestation_object()),
            "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
        }))
    }

    fn assert(&mut self, ceremony: &str) -> String {
        let options = decode_ceremony(ceremony);
        let challenge = required_text(&options, "/challenge");
        self.counter = self.counter.saturating_add(1);
        let authenticator_data = self.authenticator_data(self.counter, false);
        let client_data = client_data("webauthn.get", challenge);
        let client_hash = Sha256::digest(&client_data);
        let mut signed = Vec::with_capacity(authenticator_data.len() + client_hash.len());
        signed.extend_from_slice(&authenticator_data);
        signed.extend_from_slice(&client_hash);
        let signature = self.signing_key.sign(&signed).to_bytes();
        encode_response(&json!({
            "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
            "authenticatorData": URL_SAFE_NO_PAD.encode(authenticator_data),
            "signature": URL_SAFE_NO_PAD.encode(signature),
            "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
            "userHandle": self.user_handle,
        }))
    }

    fn attestation_object(&self) -> Vec<u8> {
        let map = CborValue::Map(vec![
            (
                CborValue::Text("fmt".to_owned()),
                CborValue::Text("none".to_owned()),
            ),
            (
                CborValue::Text("attStmt".to_owned()),
                CborValue::Map(Vec::new()),
            ),
            (
                CborValue::Text("authData".to_owned()),
                CborValue::Bytes(self.authenticator_data(0, true)),
            ),
        ]);
        let mut bytes = Vec::new();
        required(
            ciborium::ser::into_writer(&map, &mut bytes),
            "encode attestation",
        );
        bytes
    }

    fn authenticator_data(&self, counter: u32, attested: bool) -> Vec<u8> {
        let mut bytes = Vec::new();
        bytes.extend_from_slice(&Sha256::digest(RP_ID.as_bytes()));
        bytes.push(FLAG_UP | FLAG_UV | if attested { FLAG_AT } else { 0 });
        bytes.extend_from_slice(&counter.to_be_bytes());
        if attested {
            bytes.extend_from_slice(&[0_u8; 16]);
            let length = u16::try_from(self.credential_id.len())
                .unwrap_or_else(|_| panic!("credential identifier too long"));
            bytes.extend_from_slice(&length.to_be_bytes());
            bytes.extend_from_slice(&self.credential_id);
            bytes.extend_from_slice(&self.cose_public_key());
        }
        bytes
    }

    fn cose_public_key(&self) -> Vec<u8> {
        let map = CborValue::Map(vec![
            (CborValue::Integer(1.into()), CborValue::Integer(1.into())),
            (
                CborValue::Integer(3.into()),
                CborValue::Integer((-8).into()),
            ),
            (
                CborValue::Integer((-1).into()),
                CborValue::Integer(6.into()),
            ),
            (
                CborValue::Integer((-2).into()),
                CborValue::Bytes(self.signing_key.verifying_key().to_bytes().to_vec()),
            ),
        ]);
        let mut bytes = Vec::new();
        required(
            ciborium::ser::into_writer(&map, &mut bytes),
            "encode public key",
        );
        bytes
    }
}

fn decode_ceremony(value: &str) -> Value {
    let bytes = required(URL_SAFE_NO_PAD.decode(value), "decode ceremony");
    required(serde_json::from_slice(&bytes), "parse ceremony")
}

fn required_text<'value>(value: &'value Value, pointer: &str) -> &'value str {
    value
        .pointer(pointer)
        .and_then(Value::as_str)
        .unwrap_or_else(|| panic!("missing ceremony field {pointer}"))
}

fn encode_response(value: &Value) -> String {
    URL_SAFE_NO_PAD.encode(required(serde_json::to_vec(value), "serialize response"))
}

fn client_data(kind: &str, challenge: &str) -> Vec<u8> {
    required(
        serde_json::to_vec(&json!({
            "type": kind,
            "challenge": challenge,
            "origin": ORIGIN,
            "crossOrigin": false,
        })),
        "encode client data",
    )
}

struct ObservedServices {
    inner: IdentityServices,
    executed: Arc<Mutex<Vec<String>>>,
}

impl PrivilegedHumanServices for ObservedServices {
    fn execute_public(
        &mut self,
        store: &mut PrincipalStore,
        passkeys: &Passkeys,
        request: ComponentOperationRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        required(self.executed.lock(), "execution log").push(request.operation.name.clone());
        self.inner.execute_public(store, passkeys, request, now)
    }

    fn execute_authorized(
        &mut self,
        scope: &mut PrincipalScope<'_>,
        passkeys: &Passkeys,
        session: AuthorizedSession<'_>,
        request: ComponentOperationRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        required(self.executed.lock(), "execution log").push(request.operation.name.clone());
        self.inner
            .execute_authorized(scope, passkeys, session, request, now)
    }

    fn readiness(&mut self, store: &mut PrincipalStore, now: u64) -> Result<Readiness, ApiFailure> {
        self.inner.readiness(store, now)
    }
}

struct Harness {
    router: Arc<Router<UnixComponents>>,
    client: UnixComponents,
    executed: Arc<Mutex<Vec<String>>>,
    access_token: String,
    csrf_token: String,
    shutdown: ComponentShutdown,
    worker: Option<JoinHandle<Result<(), layerx_human_service::server::ComponentServerError>>>,
    _clock: RunningClock,
    _directory: TestDirectory,
}

impl Harness {
    fn new() -> Self {
        let directory = required(TestDirectory::new(), "test directory");
        let (clock_server, binding) = required(RunningClock::start(&directory.0), "clock server");
        let clock = required(RuntimeClock::connect(binding), "clock client");
        let now = required(clock.sample(Duration::from_secs(1)), "current time").unix_seconds();
        let map = tenancy(&[(ACCOUNT_ID, "retired-operations")]);
        let (mut store, _) =
            install_and_open(&directory.0.join("store"), &map, retention_uniform(86_400));
        let passkeys = required(
            Passkeys::new(AuthConfig {
                rp_id: RP_ID.to_owned(),
                rp_name: "LayerX".to_owned(),
                origin: ORIGIN.to_owned(),
                ceremony_ttl_secs: 300,
                assertion_ttl_secs: 60,
                session_ttl_secs: 300,
                refresh_ttl_secs: 3_600,
                step_up_ttl_secs: 60,
                rate_limit: RateLimit {
                    attempts: 100,
                    window_secs: 60,
                },
            }),
            "passkeys",
        );
        let session = {
            let mut scope = required(store.principal(&principal(ACCOUNT_ID)), "principal scope");
            let mut authenticator = SoftwareAuthenticator::new();
            let identity = required(
                AccountIdentity::new("owner@example.test", "Owner"),
                "account identity",
            );
            let registration = required(
                passkeys.begin_registration(&mut scope, &identity, "Passkey", now),
                "begin registration",
            );
            let credential = authenticator.register(&registration.ceremony);
            required(
                passkeys.finish_registration(
                    &mut scope,
                    &registration.registration_id,
                    &credential,
                    now,
                ),
                "finish registration",
            );
            let assertion = required(passkeys.begin_assertion(&mut scope, now), "begin assertion");
            let credential = authenticator.assert(&assertion.ceremony);
            required(
                passkeys.finish_assertion(&mut scope, &assertion.assertion_id, &credential, now),
                "finish assertion",
            );
            required(
                passkeys.open_session(
                    &mut scope,
                    &assertion.assertion_id,
                    required(Device::mint("Retirement test", "web"), "device"),
                    now,
                ),
                "open session",
            )
        };
        let access_token = session.access_token().expose().to_owned();
        let csrf_token = session.csrf_token().expose().to_owned();
        let executed = Arc::new(Mutex::new(Vec::new()));
        let accounts = required(
            ProvisionedAccounts::new([required(
                ProvisionedAccount::new(ACCOUNT_ID, "owner@example.test", "Owner", "Passkey"),
                "provisioned account",
            )]),
            "accounts",
        );
        let services = ObservedServices {
            inner: IdentityServices::new(accounts),
            executed: Arc::clone(&executed),
        };
        let components = required(
            PrivilegedHumanComponents::new(
                store,
                passkeys,
                services,
                AuthorizationGrantPolicy {
                    lifetime_seconds: 30,
                    maximum_outstanding: 32,
                },
                clock.clone(),
            ),
            "components",
        );
        let socket = directory.0.join("human.sock");
        let limits = default_component_limits();
        let bound = required(
            HumanComponentServer::new(Arc::new(components)).bind(ComponentServerConfig {
                socket_path: socket.clone(),
                allowed_uid: rustix::process::getuid().as_raw(),
                worker_count: 1,
                queue_capacity: 1,
                limits,
            }),
            "bind components",
        );
        let shutdown = bound.shutdown();
        let worker = Some(thread::spawn(move || bound.run()));
        let client = required(UnixComponents::new(&socket, limits), "component client");
        let backend = Arc::new(required(
            UnixComponents::new(&socket, limits),
            "router backend",
        ));
        let router = Arc::new(required(
            Router::new(
                backend,
                required(PrincipalLimits::new(100, 60, 100), "principal limits"),
                HttpConfig {
                    maximum_header_bytes: 32_768,
                    maximum_body_bytes: 1_048_576,
                    allowed_origin: ORIGIN.to_owned(),
                    service_version: "retired-operations-test".to_owned(),
                },
                clock,
            ),
            "router",
        ));
        Self {
            router,
            client,
            executed,
            access_token,
            csrf_token,
            shutdown,
            worker,
            _clock: clock_server,
            _directory: directory,
        }
    }

    fn request(&self, golden: &Value, authenticated: bool) -> String {
        let method = text(golden, "method");
        let path = text(golden, "path");
        let mut request =
            format!("{method} {path} HTTP/1.1\r\nHost: paxportwallet.com\r\nOrigin: {ORIGIN}\r\n");
        if authenticated {
            request.push_str(&format!(
                "Cookie: __Host-layerx_access={}; __Host-layerx_csrf={}\r\nX-LayerX-CSRF: {}\r\n",
                self.access_token, self.csrf_token, self.csrf_token
            ));
        }
        if let Some(headers) = golden.get("headers").and_then(Value::as_object) {
            for (name, value) in headers {
                let value = value.as_str().unwrap_or_else(|| panic!("header {name}"));
                request.push_str(&format!("{name}: {value}\r\n"));
            }
        }
        let body = golden
            .get("body")
            .map_or_else(String::new, Value::to_string);
        request.push_str(&format!(
            "Content-Type: application/json\r\nContent-Length: {}\r\n\r\n{body}",
            body.len()
        ));
        request
    }

    fn exchange(&self, request: &str) -> (u16, Value) {
        let (mut client, mut server) = required(UnixStream::pair(), "HTTP socket pair");
        required(
            client.set_read_timeout(Some(Duration::from_secs(10))),
            "read deadline",
        );
        required(
            server.set_read_timeout(Some(Duration::from_secs(10))),
            "server deadline",
        );
        let router = Arc::clone(&self.router);
        let worker = thread::spawn(move || {
            required(
                router.serve_one(&mut server, "retired-operations"),
                "serve HTTP",
            )
        });
        required(client.write_all(request.as_bytes()), "write HTTP");
        let mut raw = String::new();
        required(client.read_to_string(&mut raw), "read HTTP");
        required(worker.join(), "HTTP worker");
        let (head, body) = raw
            .split_once("\r\n\r\n")
            .unwrap_or_else(|| panic!("HTTP response: {raw}"));
        let status = required(
            head.split_whitespace()
                .nth(1)
                .unwrap_or_else(|| panic!("HTTP status"))
                .parse(),
            "status code",
        );
        (
            status,
            required(serde_json::from_str(body), "response JSON"),
        )
    }
}

impl Drop for Harness {
    fn drop(&mut self) {
        self.shutdown.request();
        if let Some(worker) = self.worker.take() {
            required(
                required(worker.join(), "component worker"),
                "component completion",
            );
        }
    }
}

fn text<'a>(value: &'a Value, field: &str) -> &'a str {
    value[field]
        .as_str()
        .unwrap_or_else(|| panic!("missing {field}"))
}

fn golden(name: &str) -> Value {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../schema/human-api/golden")
        .join(format!("{name}.request.json"));
    required(
        serde_json::from_str(&required(fs::read_to_string(path), "golden request")),
        "golden JSON",
    )
}

fn assert_retired(status: u16, body: &Value, operation: &str) {
    assert_eq!(status, 410, "{operation}: {body}");
    assert_eq!(body["ok"], false, "{operation}: {body}");
    assert_eq!(body["error"]["code"], "operation-retired", "{operation}");
    assert_eq!(
        body["error"]["copy_key"], "error.operation.retired",
        "{operation}"
    );
    assert_eq!(body["error"]["retry"], "final", "{operation}");
}

#[test]
fn every_retired_operation_answers_operation_retired() {
    let harness = Harness::new();
    let schema = required(ApiSchema::v1(), "schema");
    let retired = schema
        .operations()
        .iter()
        .filter(|operation| operation.retired)
        .map(|operation| operation.name.as_str())
        .collect::<Vec<_>>();
    assert_eq!(retired, RETIRED);
    for name in RETIRED {
        let golden = golden(name);
        let (status, body) = harness.exchange(&harness.request(&golden, true));
        assert_retired(status, &body, name);
        let matched = required(
            schema.route(text(&golden, "method"), text(&golden, "path")),
            "golden route",
        )
        .unwrap_or_else(|| panic!("route {name}"));
        let Err(failure) = harness.client.execute(ScopedRequest {
            operation: matched.operation,
            principal: None,
            path_parameters: matched.path_parameters,
            body: golden.get("body").cloned().unwrap_or_else(|| json!({})),
            idempotency_key: golden["headers"]["Idempotency-Key"]
                .as_str()
                .map(str::to_owned),
            trace: TRACE.to_owned(),
        }) else {
            panic!("component socket must refuse retired execution: {name}");
        };
        assert_eq!(failure, ApiFailure::operation_retired(), "{name}");
    }
    assert!(required(harness.executed.lock(), "execution log").is_empty());
}

#[test]
fn retired_operation_is_refused_before_authorization() {
    let harness = Harness::new();
    for name in RETIRED {
        let golden = golden(name);
        let (status, body) = harness.exchange(&harness.request(&golden, false));
        assert_retired(status, &body, name);
        let method = text(&golden, "method");
        let path = text(&golden, "path");
        let request = format!("{method} {path} HTTP/1.1\r\nHost: paxportwallet.com\r\nContent-Type: application/json\r\nContent-Length: 1\r\n\r\n{{");
        let (status, body) = harness.exchange(&request);
        assert_retired(status, &body, name);
    }
    let live = golden("move.quote");
    let (status, body) = harness.exchange(&harness.request(&live, false));
    assert_eq!(status, 401, "{body}");
    assert_eq!(body["error"]["code"], "unauthenticated");
    assert!(required(harness.executed.lock(), "execution log").is_empty());
}

#[test]
fn live_operation_is_still_dispatched() {
    let harness = Harness::new();
    let golden = golden("move.quote");
    let operation: &Operation = harness
        .router
        .schema()
        .operation("move.quote")
        .unwrap_or_else(|| panic!("move.quote"));
    assert!(!operation.retired);
    let (status, body) = harness.exchange(&harness.request(&golden, true));
    assert_eq!(status, 503, "{body}");
    assert_eq!(body["error"]["code"], "unavailable");
    assert_eq!(
        *required(harness.executed.lock(), "execution log"),
        ["move.quote"]
    );
}
