#[path = "../src/time.rs"]
mod api_time;
use layerx_human_service as service;
use serde_json::{json, Value};
use service::auth::Passkeys;
use service::custody::{CustodyError, KeyId, Keystore};
use service::security::{KeyExportCeremony, SecurityError};
use service::server::key_export::{acknowledge, KeyExportServices};
use service::trace::TraceId;

use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::OpenOptionsExt as _;
use std::path::PathBuf;
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_types::clock::Clock;

use service::audit::{AuditChain, AuditEvent, SecurityChangeKind};
use service::auth::{
    AccountIdentity, AuthConfig, Device, OperationDigest, RateLimit, SessionGrant, StepUpEvidence,
};
use service::custody::{EnvelopeKms, KeyClass, KeyEntropy};
use service::security::{key_export_delivery_token, security_action_digest, SecurityAction};
use service::server::{
    default_component_limits, AuthorizationGrantPolicy, ComponentServerConfig, ComponentShutdown,
    HttpConfig, HumanComponentServer, IdentityProjector, IdentityServices, PrincipalLimits,
    PrivilegedHumanComponents, ProvisionedAccount, ProvisionedAccounts, Router, UnixComponents,
};
use service::store::{
    AgentTenantId, PrincipalId, PrincipalStore, RetentionPeriod, RetentionPolicy, TenancyDigest,
    TenancyMap,
};

const OWNER: &str = "act_00112233445566778899aabbccddeeff";
const OTHER: &str = "act_ffeeddccbbaa99887766554433221100";
const ORIGIN: &str = "https://paxportwallet.com";
const TRACE: &str = "trc_00112233445566778899aabbccddeeff";
const SEED: [u8; 32] = [0xa5; 32];

#[track_caller]
fn checked<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("key export repair: {error:?}"))
}

fn string<'a>(value: &'a Value, field: &str) -> &'a str {
    value[field]
        .as_str()
        .unwrap_or_else(|| panic!("missing string {field}"))
}

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use ciborium::value::Value as CborValue;
use ed25519_dalek::{Signer as _, SigningKey};
use sha2::{Digest as _, Sha256};
const RP_ID: &str = "paxportwallet.com";
const FLAG_UP: u8 = 1;
const FLAG_UV: u8 = 4;
const FLAG_AT: u8 = 64;
fn required<T, E: std::fmt::Debug>(result: Result<T, E>, label: &str) -> T {
    result.unwrap_or_else(|error| panic!("{label}: {error:?}"))
}
fn decode_ceremony(value: &str) -> Value {
    let bytes = required(URL_SAFE_NO_PAD.decode(value), "decode ceremony");
    required(serde_json::from_slice(&bytes), "parse ceremony")
}

fn required_text<'a>(value: &'a Value, pointer: &str) -> &'a str {
    value
        .pointer(pointer)
        .and_then(Value::as_str)
        .unwrap_or_else(|| panic!("missing ceremony field {pointer}"))
}

fn encode_response(value: &Value) -> String {
    URL_SAFE_NO_PAD.encode(required(serde_json::to_vec(value), "serialize response"))
}

/// A software `WebAuthn` authenticator exercising the same Ed25519 signature,
/// authenticator-data, CBOR attestation and monotonic-counter path as a
/// hardware/platform authenticator.
struct Authenticator {
    signing_key: SigningKey,
    credential_id: Vec<u8>,
    counter: u32,
    user_handle: Option<String>,
}

impl Authenticator {
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
        let client_data = client_data("webauthn.create", challenge, ORIGIN);
        encode_response(&json!({
            "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
            "transports": ["internal"],
            "attestationObject": URL_SAFE_NO_PAD.encode(self.attestation_object()),
            "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
        }))
    }

    fn assert(&mut self, ceremony: &str) -> String {
        self.assert_for_origin(ceremony, ORIGIN)
    }

    fn assert_for_origin(&mut self, ceremony: &str, origin: &str) -> String {
        let options = decode_ceremony(ceremony);
        let challenge = required_text(&options, "/challenge");
        self.counter = self.counter.saturating_add(1);
        let authenticator_data = self.authenticator_data(self.counter, false);
        let client_data = client_data("webauthn.get", challenge, origin);
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
            let credential_length = u16::try_from(self.credential_id.len())
                .unwrap_or_else(|_| panic!("credential identifier too long"));
            bytes.extend_from_slice(&credential_length.to_be_bytes());
            bytes.extend_from_slice(&self.credential_id);
            bytes.extend_from_slice(&self.cose_public_key());
        }
        bytes
    }

    fn cose_public_key(&self) -> Vec<u8> {
        let public_key = self.signing_key.verifying_key().to_bytes().to_vec();
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
                CborValue::Bytes(public_key),
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

fn client_data(kind: &str, challenge: &str, origin: &str) -> Vec<u8> {
    required(
        serde_json::to_vec(&json!({
            "type": kind,
            "challenge": challenge,
            "origin": origin,
            "crossOrigin": false,
        })),
        "encode client data",
    )
}

struct Fixture {
    root: PathBuf,
    digest: TenancyDigest,
    principal: PrincipalId,
    other: PrincipalId,
    key: KeyId,
}

impl Fixture {
    fn new() -> Self {
        use std::os::unix::fs::DirBuilderExt as _;
        let mut entropy = [0; 8];
        checked(getrandom::fill(&mut entropy));
        let root = std::env::temp_dir().join(format!("kx-{:016x}", u64::from_be_bytes(entropy)));
        checked(fs::DirBuilder::new().mode(0o700).create(&root));
        checked(fs::write(root.join("kms-root"), [0x42; 64]));
        let map = checked(TenancyMap::new(
            [(OWNER, "owner-tenant"), (OTHER, "other-tenant")].map(|(principal, tenant)| {
                (
                    checked(PrincipalId::new(principal)),
                    checked(AgentTenantId::new(tenant)),
                )
            }),
        ));
        let digest = checked(map.install(&root.join("store")));
        let fixture = Self {
            root,
            digest,
            principal: checked(PrincipalId::new(OWNER)),
            other: checked(PrincipalId::new(OTHER)),
            key: checked(KeyId::new("human-primary")),
        };
        let keystore = fixture.keystore();
        for principal in [&fixture.principal, &fixture.other] {
            checked(keystore.generate(
                principal,
                &fixture.key,
                KeyClass::HumanPrimary,
                checked(KeyEntropy::new(SEED, [0x11; 16], [0x22; 24])),
            ));
        }
        fixture
    }

    fn keystore(&self) -> Keystore {
        checked(Keystore::open_development(
            self.root.join("custody"),
            125,
            checked(EnvelopeKms::new(
                "file-kms://repair-export",
                self.root.join("kms-root"),
            )),
        ))
    }

    fn store(&self) -> PrincipalStore {
        checked(PrincipalStore::open(
            self.root.join("store"),
            RetentionPolicy {
                journeys: RetentionPeriod::new(86_400),
                notifications: RetentionPeriod::new(86_400),
                audit: RetentionPeriod::new(86_400),
                telemetry: RetentionPeriod::new(86_400),
                cache: RetentionPeriod::new(86_400),
            },
            self.digest,
        ))
    }

    fn setup(&self, now: u64) -> (Passkeys, SessionGrant, Authenticator) {
        let passkeys = checked(Passkeys::new(AuthConfig {
            rp_id: "paxportwallet.com".to_owned(),
            rp_name: "LayerX".to_owned(),
            origin: ORIGIN.to_owned(),
            ceremony_ttl_secs: 300,
            assertion_ttl_secs: 60,
            session_ttl_secs: 3_600,
            refresh_ttl_secs: 7_200,
            step_up_ttl_secs: 60,
            rate_limit: RateLimit {
                attempts: 1_000,
                window_secs: 60,
            },
        }));
        let mut browser = Authenticator::new();
        let mut store = self.store();
        let mut scope = checked(store.principal(&self.principal));
        let registration = checked(passkeys.begin_registration(
            &mut scope,
            &checked(AccountIdentity::new("owner@paxportwallet.com", "Owner")),
            "Primary passkey",
            now,
        ));
        let credential = browser.register(&registration.ceremony);
        checked(passkeys.finish_registration(
            &mut scope,
            &registration.registration_id,
            &credential,
            now + 1,
        ));
        let assertion = checked(passkeys.begin_assertion(&mut scope, now + 2));
        let credential = browser.assert(&assertion.ceremony);
        checked(passkeys.finish_assertion(
            &mut scope,
            &assertion.assertion_id,
            &credential,
            now + 3,
        ));
        let session = checked(passkeys.open_session(
            &mut scope,
            &assertion.assertion_id,
            checked(Device::mint("Owner browser", "desktop")),
            now + 4,
        ));
        (passkeys, session, browser)
    }

    fn evidence(
        &self,
        passkeys: &Passkeys,
        browser: &mut Authenticator,
        digest: OperationDigest,
        now: u64,
    ) -> StepUpEvidence {
        let mut store = self.store();
        let mut scope = checked(store.principal(&self.principal));
        let challenge = checked(passkeys.begin_step_up_authorized(&mut scope, digest, now));
        let credential = browser.assert(&challenge.ceremony);
        checked(passkeys.finish_step_up(&mut scope, &challenge.challenge_id, &credential, now))
    }

    fn action(&self) -> OperationDigest {
        checked(security_action_digest(
            &self.principal,
            SecurityAction::ExportPrimaryKey,
            None,
        ))
    }

    fn export_count(&self, kind: SecurityChangeKind) -> usize {
        let mut store = self.store();
        let scope = checked(store.principal(&self.principal));
        let chain = checked(AuditChain::open(&scope));
        checked(chain.entries(&scope))
            .iter()
            .filter(|entry| {
                matches!(entry.event(),
            AuditEvent::SecurityChange { change, .. } if *change == kind)
            })
            .count()
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

#[test]
fn repair_3_export_handoff() {
    let fixture = Fixture::new();
    let (passkeys, _session, mut browser) = fixture.setup(100);
    let trace = checked(TraceId::parse(TRACE));
    let keystore = fixture.keystore();
    let short = checked(KeyExportCeremony::new(fixture.key.clone(), 10));
    let expired = checked(short.begin(&keystore, &fixture.principal, 110));
    let evidence = fixture.evidence(&passkeys, &mut browser, fixture.action(), 121);
    {
        let mut store = fixture.store();
        let mut scope = checked(store.principal(&fixture.principal));
        assert!(matches!(
            short.finish(
                &keystore,
                &mut scope,
                &expired.export_id,
                &evidence,
                &trace,
                121
            ),
            Err(SecurityError::Custody(CustodyError::StepUpExpired))
        ));
        assert!(!checked(
            keystore.self_custodied(&fixture.principal, &fixture.key)
        ));
    }
    let ceremony = checked(KeyExportCeremony::new(fixture.key.clone(), 300));
    let challenge = checked(ceremony.begin(&keystore, &fixture.principal, 122));
    assert_ne!(challenge.export_id, expired.export_id);
    let evidence = fixture.evidence(&passkeys, &mut browser, challenge.confirms, 123);
    let mut store = fixture.store();
    let mut scope = checked(store.principal(&fixture.principal));
    let key_stage = fixture
        .root
        .join(format!("custody/principals/{OWNER}/human-primary.key.tmp"));
    checked(fs::create_dir(&key_stage));
    assert!(matches!(
        ceremony.finish(
            &keystore,
            &mut scope,
            &challenge.export_id,
            &evidence,
            &trace,
            123
        ),
        Err(SecurityError::Custody(CustodyError::Io(_)))
    ));
    assert!(!checked(
        keystore.self_custodied(&fixture.principal, &fixture.key)
    ));
    checked(fs::remove_dir(&key_stage));

    // A real failed audit write after the durable custody marker returns no key.
    let audit_stage = fixture
        .root
        .join(format!("store/principals/{OWNER}/store.bin.tmp"));
    checked(fs::create_dir(&audit_stage));
    assert!(matches!(
        ceremony.finish(
            &keystore,
            &mut scope,
            &challenge.export_id,
            &evidence,
            &trace,
            124
        ),
        Err(SecurityError::Audit(service::audit::AuditError::Store(
            service::store::StoreError::Io(_)
        )))
    ));
    assert!(checked(
        keystore.self_custodied(&fixture.principal, &fixture.key)
    ));
    drop(scope);
    drop(store);
    drop(keystore);
    checked(fs::remove_dir(&audit_stage));
    assert!(matches!(
        signing_attempt(&fixture, 125),
        Err(CustodyError::SelfCustodied)
    ));
    assert_eq!(fixture.export_count(SecurityChangeKind::KeyExported), 0);

    // Stop and reopen both stores; fail the provider after the audit was persisted.
    let keystore = fixture.keystore();
    let mut store = fixture.store();
    let mut scope = checked(store.principal(&fixture.principal));
    let durable = checked(passkeys.load_step_up_evidence(&mut scope, evidence.challenge_id(), 125));
    checked(fs::remove_file(fixture.root.join("kms-root")));
    assert!(matches!(
        ceremony.finish(
            &keystore,
            &mut scope,
            &challenge.export_id,
            &durable,
            &trace,
            125
        ),
        Err(SecurityError::Custody(CustodyError::Kms(
            service::custody::KmsError::Unavailable
        )))
    ));
    drop(scope);
    drop(store);
    drop(keystore);
    assert_eq!(fixture.export_count(SecurityChangeKind::KeyExported), 1);
    checked(fs::write(fixture.root.join("kms-root"), [0x42; 64]));
    // An interrupted replacement's staging file must not wedge the resumed handoff.
    let mut interrupted = checked(
        fs::OpenOptions::new()
            .create_new(true)
            .write(true)
            .mode(0o600)
            .open(&key_stage),
    );
    checked(interrupted.write_all(b"interrupted record"));
    checked(interrupted.sync_all());
    drop(interrupted);
    let keystore = fixture.keystore();
    let mut store = fixture.store();
    let mut scope = checked(store.principal(&fixture.principal));
    let delivered = checked(ceremony.finish(
        &keystore,
        &mut scope,
        &challenge.export_id,
        &durable,
        &trace,
        126,
    ));
    assert_eq!(delivered.secret.expose(), "a5".repeat(32));
    assert_eq!(delivered.self_custodied_at, 124);
    // Lose the response before acknowledgement and restart again.
    drop(delivered);
    drop(scope);
    drop(store);
    drop(keystore);
    let fresh = fixture.evidence(&passkeys, &mut browser, challenge.confirms, 500);
    let keystore = fixture.keystore();
    let resumed = checked(ceremony.begin(&keystore, &fixture.principal, 500));
    assert_eq!(resumed.export_id, challenge.export_id);
    let mut store = fixture.store();
    {
        let mut other = checked(store.principal(&fixture.other));
        assert!(matches!(
            ceremony.finish(
                &keystore,
                &mut other,
                &challenge.export_id,
                &fresh,
                &trace,
                500
            ),
            Err(SecurityError::StepUpMismatch)
        ));
        assert!(!checked(
            keystore.self_custodied(&fixture.other, &fixture.key)
        ));
    }
    let mut scope = checked(store.principal(&fixture.principal));
    let delivered = checked(ceremony.finish(
        &keystore,
        &mut scope,
        &resumed.export_id,
        &fresh,
        &trace,
        500,
    ));
    assert_eq!(delivered.secret.expose(), "a5".repeat(32));
    assert_eq!(delivered.self_custodied_at, 124);
    assert_eq!(
        export_count_in_scope(&scope, SecurityChangeKind::KeyExported),
        1
    );
    assert!(ceremony
        .acknowledge(
            &keystore,
            &mut scope,
            &resumed.export_id,
            [0; 32],
            &fresh,
            &trace,
            500
        )
        .is_err());
    assert_eq!(
        export_count_in_scope(&scope, SecurityChangeKind::KeyExportAcknowledged),
        0
    );
    let token = checked(key_export_delivery_token(&SEED, &resumed.export_id));
    let acknowledged = checked(acknowledge(
        &passkeys,
        &keystore,
        &mut scope,
        &json!({"export_id": resumed.export_id,
            "delivery_token": OperationDigest::new(token).to_schema().trim_start_matches("opd_"),
            "step_up": IdentityProjector::step_up(&fresh)}),
        TRACE,
        500,
    ));
    assert_eq!(acknowledged.result, json!({}));
    drop(delivered);
    drop(scope);
    drop(store);
    drop(keystore);
    let keystore = fixture.keystore();
    let mut store = fixture.store();
    let mut scope = checked(store.principal(&fixture.principal));
    checked(ceremony.acknowledge(
        &keystore,
        &mut scope,
        &resumed.export_id,
        token,
        &fresh,
        &trace,
        501,
    ));
    assert!(matches!(
        ceremony.begin(&keystore, &fixture.principal, 501),
        Err(SecurityError::KeyExported)
    ));
    assert!(matches!(
        ceremony.finish(
            &keystore,
            &mut scope,
            &resumed.export_id,
            &fresh,
            &trace,
            501
        ),
        Err(SecurityError::Custody(CustodyError::SelfCustodied))
    ));
    drop(scope);
    drop(store);
    assert!(matches!(
        signing_attempt(&fixture, 501),
        Err(CustodyError::SelfCustodied)
    ));
    let mut store = fixture.store();
    let scope = checked(store.principal(&fixture.principal));
    assert!(matches!(
        keystore.rotate(&fixture.principal, &fixture.key),
        Err(CustodyError::SelfCustodied)
    ));
    assert_eq!(
        export_count_in_scope(&scope, SecurityChangeKind::KeyExported),
        1
    );
    assert_eq!(
        export_count_in_scope(&scope, SecurityChangeKind::KeyExportAcknowledged),
        1
    );
    let audit = checked(checked(AuditChain::open(&scope)).export(&scope));
    let sealed = checked(fs::read(
        fixture
            .root
            .join(format!("custody/principals/{OWNER}/human-primary.key")),
    ));
    for bytes in [&audit, &sealed] {
        assert!(!bytes.windows(32).any(|window| window == SEED));
        assert!(!bytes
            .windows(64)
            .any(|window| window == "a5".repeat(32).as_bytes()));
    }
}

struct RunningComponent {
    router: Arc<Router<UnixComponents>>,
    shutdown: ComponentShutdown,
    worker: Option<JoinHandle<Result<(), service::server::ComponentServerError>>>,
}

impl RunningComponent {
    fn start(fixture: &Fixture, passkeys: Passkeys, clock: Arc<dyn Clock>) -> Self {
        let accounts = checked(ProvisionedAccounts::new([checked(
            ProvisionedAccount::new(OWNER, "owner@paxportwallet.com", "Owner", "Passkey"),
        )]));
        let services = KeyExportServices::new(fixture.keystore(), IdentityServices::new(accounts));
        let components = checked(PrivilegedHumanComponents::new(
            fixture.store(),
            passkeys,
            services,
            AuthorizationGrantPolicy {
                lifetime_seconds: 30,
                maximum_outstanding: 32,
            },
            Arc::clone(&clock),
        ));
        let socket = fixture.root.join("component.sock");
        let limits = default_component_limits();
        let bound = checked(
            HumanComponentServer::new_with_clock(Arc::new(components), Arc::clone(&clock)).bind(
                ComponentServerConfig {
                    socket_path: socket.clone(),
                    allowed_uid: rustix::process::getuid().as_raw(),
                    worker_count: 1,
                    queue_capacity: 4,
                    limits,
                },
            ),
        );
        let shutdown = bound.shutdown();
        let worker = thread::spawn(move || bound.run());
        let backend = checked(UnixComponents::new_with_clock(
            socket,
            limits,
            Arc::clone(&clock),
        ));
        let router = Arc::new(checked(Router::new(
            Arc::new(backend),
            checked(PrincipalLimits::new(100, 60, 10)),
            HttpConfig {
                maximum_header_bytes: 16_384,
                maximum_body_bytes: 65_536,
                allowed_origin: ORIGIN.to_owned(),
                service_version: "repair-3".to_owned(),
            },
            clock,
        )));
        Self {
            router,
            shutdown,
            worker: Some(worker),
        }
    }

    fn post(
        &self,
        path: &str,
        body: &Value,
        session: Option<&SessionGrant>,
        csrf: bool,
    ) -> (u16, Value) {
        let listener = checked(TcpListener::bind("127.0.0.1:0"));
        let address = checked(listener.local_addr());
        let router = Arc::clone(&self.router);
        let worker = thread::spawn(move || {
            let (mut stream, _) = checked(listener.accept());
            checked(stream.set_read_timeout(Some(Duration::from_secs(10))));
            checked(stream.set_write_timeout(Some(Duration::from_secs(10))));
            checked(router.serve_one(&mut stream, "repair-export"));
        });
        let mut stream = checked(TcpStream::connect(address));
        checked(stream.set_read_timeout(Some(Duration::from_secs(10))));
        let body = checked(serde_json::to_string(body));
        let mut headers = String::new();
        if let Some(session) = session {
            headers.push_str(&format!(
                "Cookie: __Host-layerx_access={}; __Host-layerx_csrf={}\r\n",
                session.access_token().expose(),
                session.csrf_token().expose()
            ));
            if csrf {
                headers.push_str(&format!(
                    "X-LayerX-CSRF: {}\r\n",
                    session.csrf_token().expose()
                ));
            }
        }
        checked(write!(stream, "POST {path} HTTP/1.1\r\nHost: paxportwallet.com\r\nOrigin: {ORIGIN}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nX-LayerX-Trace: {TRACE}\r\n{headers}Connection: close\r\n\r\n{body}", body.len()));
        checked(stream.flush());
        let mut response = String::new();
        checked(stream.read_to_string(&mut response));
        checked(worker.join());
        let (head, body) = response
            .split_once("\r\n\r\n")
            .unwrap_or_else(|| panic!("HTTP response header"));
        let status = checked(
            head.split_whitespace()
                .nth(1)
                .unwrap_or_else(|| panic!("status"))
                .parse(),
        );
        (status, checked(serde_json::from_str(body)))
    }
}

impl Drop for RunningComponent {
    fn drop(&mut self) {
        self.shutdown.request();
        if let Some(worker) = self.worker.take() {
            checked(checked(worker.join()));
        }
    }
}

#[derive(serde::Deserialize, serde::Serialize)]
#[serde(deny_unknown_fields)]
struct BrowserChallenge {
    export_id: String,
    confirms: String,
    expires_at: String,
}

fn browser_decode(challenge: &Value) -> Value {
    // The browser contract is string-valued. Consume the production HTTP result
    // with those exact field types, then enforce the owning executable schema.
    let generated = include_str!("../../../../../apps/web/src/api/generated/index.ts");
    let decoder = generated
        .split_once("export function decodeKeyExportChallenge(")
        .expect("generated decoder")
        .1
        .split_once("export function encodeKeyExportChallenge(")
        .expect("generated encoder")
        .0;
    for field in ["export_id", "confirms", "expires_at"] {
        assert!(decoder.contains(&format!("{field}: expectString(object[\"{field}\"]")));
    }
    let decoded: BrowserChallenge = checked(serde_json::from_value(challenge.clone()));
    let schema = checked(service::server::schema::ApiSchema::v1());
    let operation = schema
        .operation("security.key-export.begin")
        .expect("begin operation");
    checked(schema.encode_response(operation, challenge));
    checked(OperationDigest::parse_schema(&decoded.confirms));
    let encoded = checked(serde_json::to_value(decoded));
    assert_eq!(&encoded, challenge);
    encoded
}

#[test]
fn repair_3_export_challenge_wire() {
    let fixture = Fixture::new();
    let clock: Arc<dyn Clock> =
        Arc::new(layerx_human_test_support::StepClock::new(1_700_000_000_000));
    let now = checked(clock.sample(Duration::from_secs(1))).unix_seconds();
    let (passkeys, session, mut browser) = fixture.setup(now - 200);
    let expired = fixture.evidence(&passkeys, &mut browser, fixture.action(), now - 100);
    let wrong = fixture.evidence(
        &passkeys,
        &mut browser,
        checked(security_action_digest(
            &fixture.principal,
            SecurityAction::RevealRecoveryEvidence,
            None,
        )),
        now,
    );
    let evidence = fixture.evidence(&passkeys, &mut browser, fixture.action(), now);
    // Exercise the production authorization reader as well as the shared handlers.
    let index = checked(service::server::production_auth::AuthDiscoveryIndex::open(
        fixture.root.join("auth-index"),
        checked(service::server::production_auth::IndexAuthenticationKey::new([0x31; 32])),
    ));
    checked(index.bind_session(&session, &fixture.principal));
    let schema = checked(service::server::schema::ApiSchema::v1());
    let operation = schema
        .operation("security.key-export.finish")
        .unwrap_or_else(|| panic!("finish operation"));
    let parameters = std::collections::BTreeMap::new();
    let authorized_body = json!({"export_id": "authorization-disclosure", "step_up": IdentityProjector::step_up(&evidence)});
    for (proof, admitted) in [
        (Some(&evidence), true),
        (Some(&wrong), false),
        (Some(&expired), false),
        (None, false),
    ] {
        let mut store = fixture.store();
        let result = service::server::production_auth::authorize_execution(
            &mut store,
            &passkeys,
            &index,
            service::server::production_auth::ExecutionCredentials {
                access: session.access_token().expose(),
                csrf: Some(session.csrf_token().expose()),
                step_up: proof,
            },
            service::server::production_auth::AuthorizationDisclosure {
                operation,
                destination: "/v1/security/key-export/finish",
                path_parameters: &parameters,
                body: &authorized_body,
                idempotency_key: None,
                trace: TRACE,
            },
            now,
            30,
        );
        assert_eq!(result.is_ok(), admitted);
    }
    let service = RunningComponent::start(&fixture, passkeys, Arc::clone(&clock));
    let begin_body = json!({"step_up": IdentityProjector::step_up(&evidence)});
    assert_eq!(
        service
            .post("/v1/security/key-export/begin", &begin_body, None, false)
            .0,
        401
    );
    assert_eq!(
        service
            .post(
                "/v1/security/key-export/begin",
                &begin_body,
                Some(&session),
                false
            )
            .0,
        403
    );
    let before_begin = checked(clock.sample(Duration::from_secs(1))).unix_seconds();
    let (status, wire) = service.post(
        "/v1/security/key-export/begin",
        &begin_body,
        Some(&session),
        true,
    );
    let after_begin = checked(clock.sample(Duration::from_secs(1))).unix_seconds();
    assert_eq!(status, 200, "{wire}");
    let decoded = browser_decode(&wire["result"]);
    assert_eq!(string(&decoded, "confirms"), fixture.action().to_schema());
    let expiry =
        checked(api_time::seconds_from_rfc3339(string(&decoded, "expires_at")).ok_or("expiry"));
    assert!(expiry >= before_begin + 300 && expiry <= after_begin + 300);
    let (status, stepup) = service.post(
        "/v1/step-up",
        &json!({"confirms": decoded["confirms"]}),
        Some(&session),
        true,
    );
    assert_eq!(status, 200, "{stepup}");
    assert_eq!(stepup["result"]["confirms"], decoded["confirms"]);
    assert!(api_time::seconds_from_rfc3339(string(&stepup["result"], "expires_at")).is_some());
    let credential = browser.assert(string(&stepup["result"], "ceremony"));
    let finish_path = format!("/v1/step-up/{}", string(&stepup["result"], "challenge_id"));
    let (status, fresh) = service.post(
        &finish_path,
        &json!({"credential": credential}),
        Some(&session),
        true,
    );
    assert_eq!(status, 200, "{fresh}");
    for refused in [&wrong, &expired] {
        let body = json!({"export_id": decoded["export_id"], "step_up": IdentityProjector::step_up(refused)});
        assert_eq!(
            service
                .post(
                    "/v1/security/key-export/finish",
                    &body,
                    Some(&session),
                    true
                )
                .0,
            403
        );
        assert!(!checked(
            fixture
                .keystore()
                .self_custodied(&fixture.principal, &fixture.key)
        ));
    }
    let body = json!({"export_id": decoded["export_id"], "step_up": fresh["result"]});
    let (status, exported) = service.post(
        "/v1/security/key-export/finish",
        &body,
        Some(&session),
        true,
    );
    assert_eq!(status, 200, "{exported}");
    assert_eq!(exported["result"]["secret"]["value"], "a5".repeat(32));
    assert!(
        api_time::seconds_from_rfc3339(string(&exported["result"], "self_custodied_at")).is_some()
    );
    assert!(
        api_time::seconds_from_rfc3339(string(&exported["result"]["secret"], "remask_at"))
            .is_some()
    );
    assert!(checked(
        fixture
            .keystore()
            .self_custodied(&fixture.principal, &fixture.key)
    ));
    drop(service);
    assert_eq!(fixture.export_count(SecurityChangeKind::KeyExported), 1);
}

fn export_count_in_scope(
    scope: &service::store::PrincipalScope<'_>,
    kind: SecurityChangeKind,
) -> usize {
    checked(checked(AuditChain::open(scope)).entries(scope))
        .iter()
        .filter(|entry| matches!(entry.event(), AuditEvent::SecurityChange { change, .. } if *change == kind))
        .count()
}

fn signing_attempt(fixture: &Fixture, now: u64) -> Result<(), CustodyError> {
    use layerx_crypto::evm_transaction::Eip1559Call;
    use service::custody::{
        CustodySigner, Operation, SignAuthorization, SignRequest, SigningLimits,
    };
    use std::future::Future;
    use std::task::{Context, Poll, Wake, Waker};
    struct WakeReady;
    impl Wake for WakeReady {
        fn wake(self: Arc<Self>) {}
    }
    let mut data = vec![0xa9, 0x05, 0x9c, 0xbb];
    data.extend_from_slice(&[0; 12]);
    data.extend_from_slice(&[0x33; 20]);
    data.extend_from_slice(&[0; 31]);
    data.push(1);
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 0,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: [0x44; 20],
        data,
    };
    let key = checked(k256::ecdsa::SigningKey::from_slice(&SEED));
    let from = checked(layerx_crypto::secp256k1::evm_address(
        key.verifying_key().to_encoded_point(false).as_bytes(),
    ));
    let disclosure = checked(layerx_crypto::disclosure::call::bind_call(&call, from));
    let signer = CustodySigner::new(
        fixture.keystore(),
        fixture.store(),
        checked(layerx_types::payload::ModuleRegistry::new(&[])),
        checked(SigningLimits::new(100, 60)),
    );
    let bytes = call.encode_unsigned();
    let trace = checked(TraceId::parse(TRACE));
    let future = signer.sign(SignRequest::new(
        &fixture.principal,
        &fixture.key,
        &trace,
        SignAuthorization::new(Operation::ProtocolMutation, None),
        &bytes,
        &disclosure,
        now,
    ));
    let mut future = std::pin::pin!(future);
    let waker = Waker::from(Arc::new(WakeReady));
    match future.as_mut().poll(&mut Context::from_waker(&waker)) {
        Poll::Ready(result) => result.map(|_| ()),
        Poll::Pending => panic!("development signing unexpectedly blocked"),
    }
}
