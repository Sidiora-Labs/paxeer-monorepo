use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};
use ciborium::value::Value as CborValue;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_human_service::auth::{AccountIdentity, AuthConfig, Device, Passkeys, RateLimit};
use layerx_human_service::frame::{read_frame, write_frame, Limits};
use layerx_human_service::journeys::AgentBoundaryError;
use layerx_human_service::server::agent_runtime::AgentRuntime;
use layerx_human_service::server::production_components::AccountBalance;
use layerx_human_service::server::schema::ApiSchema;
use layerx_human_service::server::{
    default_component_limits, ApiFailure, AuthorizationGrantPolicy, AuthorizedSession,
    BackendResponse, ComponentOperationRequest, ComponentServerConfig, ComponentShutdown,
    HttpConfig, HumanComponentServer, IdentityServices, PrincipalLimits, PrivilegedHumanComponents,
    PrivilegedHumanServices, ProvisionedAccount, ProvisionedAccounts, Readiness, Router,
    UnixComponents,
};
use layerx_human_service::store::{PrincipalScope, PrincipalStore, Table};
use layerx_human_test_support as support;
use layerx_proof::checkpoint::SettlementDomain;
use layerx_types::account::AccountId;
use layerx_types::clock::{Clock, ClockError, ClockReading};
use layerx_types::ids::Did;
use layerx_types::payload::ModuleRegistry;
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::fmt::Debug;
use std::fs;
use std::io::{Read as _, Write as _};
use std::os::unix::fs::PermissionsExt as _;
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

const RP_ID: &str = "wallet.example";
const ORIGIN: &str = "https://wallet.example";
const ACCOUNT_ID: &str = "act_balance";
const OWNER: &str = "did:layerx:balance-owner";
const ASSET: [u8; 32] = [0x22; 32];
const FLAG_UP: u8 = 1;
const FLAG_UV: u8 = 4;
const FLAG_AT: u8 = 64;
const MAX_FRAME: usize = 65_536;

fn required<T, E: Debug>(result: Result<T, E>, label: &str) -> T {
    result.unwrap_or_else(|error| panic!("{label}: {error:?}"))
}
struct SystemClock {
    origin: Instant,
    generation: [u8; 16],
}

impl Clock for SystemClock {
    fn sample(&self, budget: Duration) -> Result<ClockReading, ClockError> {
        if budget.is_zero() {
            return Err(ClockError::Unavailable);
        }
        Ok(ClockReading {
            generation: self.generation,
            unix_milliseconds: SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map_err(|_| ClockError::Regression)?
                .as_millis()
                .try_into()
                .map_err(|_| ClockError::Overflow)?,
            monotonic_nanoseconds: self
                .origin
                .elapsed()
                .as_nanos()
                .try_into()
                .map_err(|_| ClockError::Overflow)?,
        })
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

fn bytes(target: &mut Vec<u8>, value: &[u8]) {
    target.extend_from_slice(&u32::try_from(value.len()).expect("length").to_be_bytes());
    target.extend_from_slice(value);
}
fn frame(op: u8, body: &[u8]) -> Vec<u8> {
    let mut value = b"LXHAGT01".to_vec();
    value.push(op);
    value.extend_from_slice(body);
    value
}
fn account() -> AccountId {
    AccountId::parse(&format!("agent:{OWNER}:main")).expect("account")
}
fn account_hash() -> [u8; 32] {
    layerx_intents::canonical::account_id_for_protocol(&account(), 3).expect("account hash")
}
fn hex(value: &[u8]) -> String {
    value.iter().map(|b| format!("{b:02x}")).collect()
}
fn answer(rung: Option<Rung>) -> Value {
    json!({"value": {"account": hex(&account_hash()), "asset": hex(&ASSET), "amount": "123", "canonical_state": "0000000000000000000000000000007b", "height": "7"},
        "achieved_verification_level": rung.map(Rung::name),
        "freshness": {"chain_head": "9", "value_height": "7", "relative_to": "7"}})
}
fn limits() -> Limits {
    Limits {
        maximum_frame_bytes: MAX_FRAME,
        maximum_connections: 1,
        maximum_streams: 1,
        maximum_queued_bytes: MAX_FRAME,
        deadline: Duration::from_secs(5),
    }
}
struct Peer {
    socket: PathBuf,
    stop: Arc<AtomicBool>,
    answer: Arc<Mutex<Value>>,
    reads: Arc<AtomicUsize>,
    worker: Option<JoinHandle<()>>,
}
impl Peer {
    fn start(root: &std::path::Path, rung: Option<Rung>) -> Self {
        let socket = root.join("agent.sock");
        let listener = UnixListener::bind(&socket).expect("bind agent peer");
        listener
            .set_nonblocking(true)
            .expect("nonblocking agent peer");
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = stop.clone();
        let answer = Arc::new(Mutex::new(answer(rung)));
        let response = answer.clone();
        let reads = Arc::new(AtomicUsize::new(0));
        let observed = reads.clone();
        let worker = thread::spawn(move || {
            let mut prefix = Vec::new();
            bytes(&mut prefix, ACCOUNT_ID.as_bytes());
            bytes(&mut prefix, OWNER.as_bytes());
            bytes(&mut prefix, account().canonical().as_bytes());
            prefix.extend_from_slice(&ASSET);
            while !stopping.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(value) => value,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("accept agent: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("agent deadline");
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .expect("agent write deadline");
                let request = read_frame(&mut stream, MAX_FRAME).expect("agent request");
                assert_eq!(&request[..9], frame(44, &[]));
                assert_eq!(&request[9..9 + prefix.len()], &prefix);
                let nested = &request[9 + prefix.len()..];
                let length =
                    u32::from_be_bytes(nested[..4].try_into().expect("nested length")) as usize;
                assert_eq!(length, nested.len() - 4);
                let request = &nested[4..];
                assert_eq!(&request[..8], b"LXHAGT01");
                let mut body = Vec::new();
                match request[8] {
                    6 => {
                        assert_eq!(request.len(), 9);
                        observed.fetch_add(1, Ordering::AcqRel);
                        bytes(
                            &mut body,
                            &serde_json::to_vec(&*response.lock().expect("balance answer"))
                                .expect("JSON"),
                        );
                    }
                    39 => {
                        assert_eq!(request.len(), 9);
                        body.push(1);
                        body.extend_from_slice(&ASSET);
                        bytes(&mut body, b"LXP");
                        body.push(18);
                        body.extend_from_slice(&9u64.to_be_bytes());
                        body.extend_from_slice(&[0x33; 32]);
                    }
                    24 | 52 => {
                        assert_eq!(&request[9..], &[0, 100]);
                        body.extend_from_slice(&[0, 0]);
                    }
                    7 => {
                        assert_eq!(request.len(), 9);
                        body.extend_from_slice(&9u64.to_be_bytes());
                        body.extend_from_slice(&1u64.to_be_bytes());
                        body.extend_from_slice(&[0x33; 32]);
                    }
                    46 => {
                        let mut expected = 9u64.to_be_bytes().to_vec();
                        expected.extend_from_slice(&[0, 100]);
                        assert_eq!(&request[9..], &expected);
                        body.extend_from_slice(&[0, 0]);
                    }
                    58 => {
                        assert_eq!(&request[9..], &[0, 5, 0, 100]);
                        body.extend_from_slice(&[0, 5, 0, 0]);
                    }
                    opcode => panic!("unexpected agent opcode {opcode}"),
                }
                write_frame(&mut stream, &frame(0, &body), MAX_FRAME).expect("agent answer");
            }
        });
        Self {
            socket,
            stop,
            answer,
            reads,
            worker: Some(worker),
        }
    }
    fn runtime(&self) -> AgentRuntime {
        AgentRuntime::new(
            &self.socket,
            limits(),
            ModuleRegistry::new(&[]).expect("registry"),
        )
        .expect("agent runtime")
        .for_subject(
            &support::principal(ACCOUNT_ID),
            &Did::new(OWNER.as_bytes()).expect("owner"),
            &account(),
            ASSET,
        )
        .expect("scoped runtime")
    }
}
impl Drop for Peer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            worker.join().expect("agent peer");
        }
    }
}

struct BalanceServices {
    identity: IdentityServices,
    agent: AgentRuntime,
    material: Arc<Mutex<Vec<Vec<u8>>>>,
}
impl PrivilegedHumanServices for BalanceServices {
    fn execute_public(
        &mut self,
        store: &mut PrincipalStore,
        passkeys: &Passkeys,
        request: ComponentOperationRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        self.identity.execute_public(store, passkeys, request, now)
    }
    fn execute_authorized(
        &mut self,
        scope: &mut PrincipalScope<'_>,
        passkeys: &Passkeys,
        session: AuthorizedSession<'_>,
        request: ComponentOperationRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        let result = match request.operation.name.as_str() {
            "account.balance" => AccountBalance::read(&mut self.agent, scope, now, 10)?,
            "home.summary" => AccountBalance::read_home_summary(
                &mut self.agent,
                scope,
                now,
                10,
                SettlementDomain::new(125, [0x11; 20]),
            )?,
            _ => {
                return self
                    .identity
                    .execute_authorized(scope, passkeys, session, request, now)
            }
        };
        let balance = if request.operation.name == "home.summary" {
            &result["balance"]
        } else {
            &result
        };
        let id = balance["evidence"][0]["evidence_id"]
            .as_str()
            .expect("evidence id")
            .strip_prefix("evd_")
            .expect("evidence prefix");
        let key = support::row_key(&format!("state-proof-{id}"));
        let bytes = scope
            .get(Table::Cache, &key)
            .expect("stored balance material")
            .bytes()
            .to_vec();
        self.material.lock().expect("observed material").push(bytes);
        Ok(BackendResponse {
            result,
            session: None,
        })
    }
    fn readiness(&mut self, store: &mut PrincipalStore, now: u64) -> Result<Readiness, ApiFailure> {
        self.identity.readiness(store, now)
    }
}
struct LiveRouter {
    router: Arc<Router<UnixComponents>>,
    access_token: String,
    shutdown: ComponentShutdown,
    worker: Option<JoinHandle<()>>,
    peer: Option<Peer>,
    material: Arc<Mutex<Vec<Vec<u8>>>>,
    root: PathBuf,
}
impl LiveRouter {
    fn connect(rung: Option<Rung>) -> Self {
        let root = support::directory("balance-rungs");
        fs::create_dir_all(&root).expect("test root");
        fs::set_permissions(&root, fs::Permissions::from_mode(0o700)).expect("private root");
        let tenancy = support::tenancy(&[(ACCOUNT_ID, "balance-tenant")]);
        let (mut store, _) = support::install_and_open(
            &root.join("store"),
            &tenancy,
            support::retention_uniform(86_400),
        );
        let mut generation = [0; 16];
        getrandom::fill(&mut generation).expect("clock generation");
        let clock = Arc::new(SystemClock {
            origin: Instant::now(),
            generation,
        });
        let now = clock
            .sample(Duration::from_secs(1))
            .expect("clock")
            .unix_seconds();
        let passkeys = Passkeys::new(AuthConfig {
            rp_id: RP_ID.to_owned(),
            rp_name: "LayerX".to_owned(),
            origin: ORIGIN.to_owned(),
            ceremony_ttl_secs: 300,
            assertion_ttl_secs: 60,
            session_ttl_secs: 300,
            refresh_ttl_secs: 3600,
            step_up_ttl_secs: 60,
            rate_limit: RateLimit {
                attempts: 100,
                window_secs: 60,
            },
        })
        .expect("passkeys");
        let session = {
            let mut scope = store
                .principal(&support::principal(ACCOUNT_ID))
                .expect("principal");
            let mut authenticator = SoftwareAuthenticator::new();
            let identity =
                AccountIdentity::new("balance@example.test", "Balance").expect("identity");
            let registration = passkeys
                .begin_registration(&mut scope, &identity, "Passkey", now)
                .expect("registration");
            let credential = authenticator.register(&registration.ceremony);
            passkeys
                .finish_registration(&mut scope, &registration.registration_id, &credential, now)
                .expect("register passkey");
            let assertion = passkeys
                .begin_assertion(&mut scope, now)
                .expect("assertion");
            let credential = authenticator.assert(&assertion.ceremony);
            passkeys
                .finish_assertion(&mut scope, &assertion.assertion_id, &credential, now)
                .expect("verify passkey");
            passkeys
                .open_session(
                    &mut scope,
                    &assertion.assertion_id,
                    Device::mint("Balance test", "web").expect("device"),
                    now,
                )
                .expect("session")
        };
        let access_token = session.access_token().expose().to_owned();
        let accounts = ProvisionedAccounts::new([ProvisionedAccount::new(
            ACCOUNT_ID,
            "balance@example.test",
            "Balance",
            "Passkey",
        )
        .expect("account")])
        .expect("accounts");
        let peer = Peer::start(&root, rung);
        let material = Arc::new(Mutex::new(Vec::new()));
        let services = BalanceServices {
            identity: IdentityServices::new(accounts),
            agent: peer.runtime(),
            material: material.clone(),
        };
        let components = PrivilegedHumanComponents::new(
            store,
            passkeys,
            services,
            AuthorizationGrantPolicy {
                lifetime_seconds: 30,
                maximum_outstanding: 32,
            },
            clock.clone(),
        )
        .expect("privileged components");
        let socket = root.join("human.sock");
        let bound = HumanComponentServer::new(Arc::new(components))
            .bind(ComponentServerConfig {
                socket_path: socket.clone(),
                allowed_uid: rustix::process::getuid().as_raw(),
                worker_count: 1,
                queue_capacity: 1,
                limits: default_component_limits(),
            })
            .expect("component socket");
        let shutdown = bound.shutdown();
        let worker = thread::spawn(move || bound.run().expect("component server"));
        let router = Router::new(
            Arc::new(UnixComponents::new(&socket, default_component_limits()).expect("components")),
            PrincipalLimits::new(100, 60, 100).expect("limits"),
            HttpConfig {
                maximum_header_bytes: 32_768,
                maximum_body_bytes: 1_048_576,
                allowed_origin: ORIGIN.to_owned(),
                service_version: "balance-rungs-test".to_owned(),
            },
            clock,
        )
        .expect("router");
        Self {
            router: Arc::new(router),
            access_token,
            shutdown,
            worker: Some(worker),
            peer: Some(peer),
            material,
            root,
        }
    }
    fn get(&self, path: &str) -> Value {
        let (mut client, mut server) = UnixStream::pair().expect("HTTP socket pair");
        client
            .set_read_timeout(Some(Duration::from_secs(10)))
            .expect("HTTP read deadline");
        server
            .set_read_timeout(Some(Duration::from_secs(10)))
            .expect("HTTP serve deadline");
        let router = self.router.clone();
        let worker = thread::spawn(move || {
            router
                .serve_one(&mut server, "balance-rungs")
                .expect("serve balance request");
        });
        write!(client, "GET {path} HTTP/1.1\r\nHost: wallet.example\r\nOrigin: https://wallet.example\r\nCookie: __Host-layerx_access={}\r\n\r\n", self.access_token).expect("write balance request");
        let mut response = String::new();
        client
            .read_to_string(&mut response)
            .expect("read balance response");
        worker.join().expect("HTTP worker");
        let (head, body) = response.split_once("\r\n\r\n").expect("HTTP response body");
        let body: Value = serde_json::from_str(body).expect("balance JSON");
        assert!(head.starts_with("HTTP/1.1 200 "), "{head}: {body}");
        assert_eq!(body["ok"], true, "{body}");
        body["result"].clone()
    }
}

impl Drop for LiveRouter {
    fn drop(&mut self) {
        self.shutdown.request();
        if let Some(worker) = self.worker.take() {
            worker.join().expect("component worker");
        }
        drop(self.peer.take());
        fs::remove_dir_all(&self.root).expect("remove test root");
    }
}
fn assert_balance(result: &Value, rung: Option<Rung>) {
    assert_eq!(result["money"], json!({"amount":"123","currency":"LXP"}));
    assert_eq!(
        result["account_id"],
        format!("act_{}", hex(&account_hash()))
    );
    let verification = rung
        .map(|r| match r {
            Rung::Submitted => "unverified",
            Rung::Receipted => "receipt-verified",
            Rung::Committed => "checkpoint-finalised",
        })
        .unwrap_or("unverified");
    assert_eq!(result["verification"], verification);
    assert_eq!(result["evidence"][0]["verification"], verification);
    assert_eq!(result["freshness"]["source_head"], "9");
    assert_eq!(result["freshness"]["within_bound"], false);
    assert!(result["freshness"].get("checkpoint").is_none());
    match rung {
        Some(r) => assert_eq!(result["rung"], r.name()),
        None => assert!(result.get("rung").is_none()),
    }
}
fn assert_material(material: &[u8], rung: Option<Rung>) {
    assert_eq!(&material[..5], b"LXHB1");
    let start = if let Some(rung) = rung {
        assert_eq!(material[5], rung.wire_rank());
        6
    } else {
        5
    };
    let len = u32::from_be_bytes(
        material[start..start + 4]
            .try_into()
            .expect("timestamp length"),
    ) as usize;
    assert_eq!(len, 20);
    let offset = start + 4 + len + 8;
    let len = u32::from_be_bytes(
        material[offset..offset + 4]
            .try_into()
            .expect("canonical length"),
    ) as usize;
    assert_eq!(
        &material[offset + 4..offset + 4 + len],
        &123u128.to_be_bytes()
    );
    let response: Value =
        serde_json::from_slice(&material[offset + 4 + len..]).expect("original daemon answer");
    assert_eq!(response, answer(rung));
}
#[test]
fn account_balance_answers_the_balance_rung() {
    for rung in [
        None,
        Some(Rung::Submitted),
        Some(Rung::Receipted),
        Some(Rung::Committed),
    ] {
        let router = LiveRouter::connect(rung);
        let result = router.get("/v1/account/balance");
        assert_balance(&result, rung);
        assert_eq!(
            router
                .peer
                .as_ref()
                .expect("peer")
                .reads
                .load(Ordering::Acquire),
            1
        );
        let material = router.material.lock().expect("balance material");
        assert_eq!(material.len(), 1);
        assert_material(&material[0], rung);
        assert_eq!(
            result["evidence"][0]["evidence_id"],
            format!("evd_{}", hex(&Sha256::digest(&material[0])))
        );
    }
    let router = LiveRouter::connect(None);
    let peer = router.peer.as_ref().expect("peer");
    let mut runtime = peer.runtime();
    let original = answer(None);
    let mut cases = Vec::new();
    for level in [json!("unknown"), json!(4), json!(true), json!({})] {
        let mut bad = original.clone();
        bad["achieved_verification_level"] = level;
        cases.push(bad);
    }
    let mut bad = original.clone();
    bad.as_object_mut()
        .expect("object")
        .remove("achieved_verification_level");
    cases.push(bad);
    for pointer in [
        "/value/account",
        "/value/asset",
        "/value/amount",
        "/value/canonical_state",
        "/value/height",
        "/freshness/chain_head",
        "/freshness/value_height",
        "/freshness/relative_to",
    ] {
        let mut bad = original.clone();
        *bad.pointer_mut(pointer).expect("field") = Value::Null;
        cases.push(bad);
    }
    let mut bad = original.clone();
    bad["value"]["account"] = json!(hex(&[0x99; 32]));
    cases.push(bad);
    let mut bad = original.clone();
    bad["value"]["asset"] = json!(hex(&[0x99; 32]));
    cases.push(bad);
    let mut bad = original.clone();
    bad["value"]["height"] = json!("8");
    cases.push(bad);
    for bad in cases {
        *peer.answer.lock().expect("answer") = bad;
        assert!(matches!(
            runtime.balance(),
            Err(AgentBoundaryError::CorruptResponse)
        ));
    }
}
#[test]
fn home_summary_balance_matches_account_balance() {
    for rung in [None, Some(Rung::Receipted)] {
        let router = LiveRouter::connect(rung);
        let account = router.get("/v1/account/balance");
        let home = router.get("/v1/home");
        assert_balance(&home["balance"], rung);
        assert_eq!(home.get("rung"), account.get("rung"));
        for field in ["rung", "verification", "account_id", "money"] {
            assert_eq!(home["balance"].get(field), account.get(field));
        }
        for field in [
            "agents",
            "approvals",
            "program_approvals",
            "recent_activity",
        ] {
            assert_eq!(home[field], json!([]));
        }
        assert_eq!(
            router
                .peer
                .as_ref()
                .expect("peer")
                .reads
                .load(Ordering::Acquire),
            2
        );
        let material = router.material.lock().expect("material");
        assert_eq!(material.len(), 2);
        for bytes in material.iter() {
            assert_material(bytes, rung);
        }
        let schema = ApiSchema::v1().expect("schema");
        let operation = schema.operation("home.summary").expect("operation");
        let mut inconsistent = home.clone();
        inconsistent["rung"] = json!("committed");
        assert!(schema.encode_response(operation, &inconsistent).is_err());
        if rung.is_some() {
            let mut inconsistent = home.clone();
            inconsistent["balance"]["rung"] = json!("committed");
            assert!(schema.encode_response(operation, &inconsistent).is_err());
        }
    }
}
