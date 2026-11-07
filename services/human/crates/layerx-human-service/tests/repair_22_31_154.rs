//! Audit 22.30.104 regressions using durable stores, genuine WebAuthn signatures,
//! the production session consumer, Unix framing and strict API response validation.
use base64::engine::general_purpose::{STANDARD, URL_SAFE_NO_PAD};
use base64::Engine as _;
use ciborium::value::Value as CborValue;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_human_service::auth::{
    AccountIdentity, AuthConfig, Device, OperationDigest, Passkeys, RateLimit,
};
use layerx_human_service::server::backend::PrincipalContext;
use layerx_human_service::server::production_auth::{
    authorize_execution, authorize_refresh_execution, consume_context, AuthDiscoveryIndex,
    AuthorizationDisclosure, ExecutionCredentials, IndexAuthenticationKey,
};
use layerx_human_service::server::production_components::{
    activity_filter_json, activity_filters, activity_page_json, cache_activity_export,
    AccountBalance,
};
use layerx_human_service::server::schema::{ApiSchema, Operation};
use layerx_human_service::server::{
    default_component_limits, ApiFailure, BackendResponse, BearerCredentials,
    ComponentServerConfig, HttpConfig, HumanApiComponents, HumanComponentServer, IdentityProjector,
    PrincipalLimits, ProductionComponents, Readiness, Router, ScopedRequest, SessionCredentials,
    UnixComponents,
};
use layerx_human_service::store::{PrincipalScope, PrincipalStore, Table, TenancyDigest};
use layerx_human_test_support as support;
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::collections::BTreeMap;
use std::fmt::Debug;
use std::fs;
use std::io::{Read as _, Write as _};
use std::os::unix::fs::PermissionsExt as _;
use std::os::unix::net::UnixStream;
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;
const RP_ID: &str = "paxportwallet.com";
const ORIGIN: &str = "https://paxportwallet.com";
const ACCOUNT_ID: &str = "act_00112233445566778899aabbccddeeff";
const NOW: u64 = 1_700_000_000;
const FLAG_UP: u8 = 1;
const FLAG_UV: u8 = 4;
const FLAG_AT: u8 = 64;
fn required<T, E: Debug>(result: Result<T, E>, label: &str) -> T {
    result.unwrap_or_else(|error| panic!("{label}: {error:?}"))
}
fn auth_config() -> AuthConfig {
    AuthConfig {
        rp_id: RP_ID.into(),
        rp_name: "LayerX".into(),
        origin: ORIGIN.into(),
        ceremony_ttl_secs: 300,
        assertion_ttl_secs: 60,
        session_ttl_secs: 300,
        refresh_ttl_secs: 3600,
        step_up_ttl_secs: 60,
        rate_limit: RateLimit {
            attempts: 100,
            window_secs: 60,
        },
    }
}
// Independent fixed-date expectations for this test's bounded clock window.
fn stamp(seconds: u64) -> String {
    assert!((NOW - 60..=NOW + 600).contains(&seconds));
    let within_hour = 800i64 + (seconds as i64 - NOW as i64);
    format!(
        "2023-11-14T22:{:02}:{:02}Z",
        within_hour / 60,
        within_hour % 60
    )
}
fn validate(operation: &str, value: &Value) {
    let schema = ApiSchema::v1().expect("schema");
    schema
        .encode_response(schema.operation(operation).expect(operation), value)
        .unwrap_or_else(|e| panic!("{operation}: {e:?}: {value}"));
}
struct Durable {
    root: std::path::PathBuf,
    digest: TenancyDigest,
    store: Option<PrincipalStore>,
}
impl Durable {
    fn new(label: &str) -> Self {
        let root = support::directory(label);
        let (store, digest) = support::install_and_open(
            &root,
            &support::tenancy(&[(ACCOUNT_ID, "tenant-a")]),
            support::retention_uniform(86400),
        );
        Self {
            root,
            digest,
            store: Some(store),
        }
    }
    fn scope(&mut self) -> PrincipalScope<'_> {
        self.store
            .as_mut()
            .expect("store")
            .principal(&support::principal(ACCOUNT_ID))
            .expect("scope")
    }
    fn reopen(&mut self) {
        drop(self.store.take());
        self.store = Some(
            PrincipalStore::open(&self.root, support::retention_uniform(86400), self.digest)
                .expect("reopen store"),
        );
    }
}
impl Drop for Durable {
    fn drop(&mut self) {
        drop(self.store.take());
        fs::remove_dir_all(&self.root).expect("remove store");
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

fn register(
    scope: &mut PrincipalScope<'_>,
    passkeys: &Passkeys,
    authenticator: &mut SoftwareAuthenticator,
) {
    let challenge = passkeys
        .begin_registration(
            scope,
            &AccountIdentity::new("mara@example.com", "Mara").expect("account"),
            "Primary",
            NOW,
        )
        .expect("begin registration");
    validate(
        "passkey.register.begin",
        &IdentityProjector::checked_registration_challenge(&challenge)
            .expect("project registration"),
    );
    let credential = authenticator.register(&challenge.ceremony);
    let passkey = passkeys
        .finish_registration(scope, &challenge.registration_id, &credential, NOW)
        .expect("registration signature");
    validate(
        "passkey.register.finish",
        &IdentityProjector::checked_passkey(&passkey).expect("project passkey"),
    );
    assert!(passkeys
        .finish_registration(scope, &challenge.registration_id, &credential, NOW)
        .is_err());
}

#[test]
fn passkey_and_stepup_timestamps_survive_reopen_and_keep_confirmation_binding() {
    let mut durable = Durable::new("repair-passkeys");
    let passkeys = Passkeys::new(auth_config()).expect("passkeys");
    let mut authenticator = SoftwareAuthenticator::new();
    register(&mut durable.scope(), &passkeys, &mut authenticator);
    let challenge = passkeys
        .begin_assertion(&mut durable.scope(), NOW)
        .expect("begin assertion");
    let projected =
        IdentityProjector::checked_assertion_challenge(&challenge).expect("project assertion");
    validate("passkey.assert.begin", &projected);
    assert_eq!(projected["expires_at"], stamp(NOW + 300));
    let credential = authenticator.assert(&challenge.ceremony);
    durable.reopen();
    let proof = passkeys
        .finish_assertion(
            &mut durable.scope(),
            &challenge.assertion_id,
            &credential,
            NOW + 1,
        )
        .expect("finish reopened assertion");
    validate(
        "passkey.assert.finish",
        &IdentityProjector::checked_assertion(&proof).expect("proof"),
    );
    assert_eq!(proof.completed_at, NOW + 1);
    assert!(passkeys
        .finish_assertion(
            &mut durable.scope(),
            &challenge.assertion_id,
            &credential,
            NOW + 1
        )
        .is_err());
    let grant = passkeys
        .open_session(
            &mut durable.scope(),
            &proof.assertion_id,
            Device::mint("Browser", "web").expect("device"),
            NOW + 1,
        )
        .expect("session");
    let opened = IdentityProjector::checked_session(&grant.session()).expect("session projection");
    validate("session.open", &opened);
    let confirms = OperationDigest::new(Sha256::digest(b"disclosed security mutation").into());
    let challenge = passkeys
        .begin_step_up_authorized(&mut durable.scope(), confirms, NOW + 2)
        .expect("stepup");
    let projected = IdentityProjector::checked_step_up_challenge(&challenge).expect("challenge");
    validate("stepup.begin", &projected);
    assert_eq!(projected["confirms"], confirms.to_schema());
    let credential = authenticator.assert(&challenge.ceremony);
    durable.reopen();
    let evidence = passkeys
        .finish_step_up(
            &mut durable.scope(),
            &challenge.challenge_id,
            &credential,
            NOW + 3,
        )
        .expect("finish reopened stepup");
    let projected = IdentityProjector::checked_step_up(&evidence).expect("stepup proof");
    validate("stepup.finish", &projected);
    assert_eq!(projected["confirms"], confirms.to_schema());
    assert_eq!(projected["completed_at"], stamp(NOW + 3));
    passkeys
        .revalidate_step_up(&mut durable.scope(), &evidence, confirms, NOW + 4)
        .expect("same mutation");
    assert!(passkeys
        .revalidate_step_up(
            &mut durable.scope(),
            &evidence,
            OperationDigest::new([17; 32]),
            NOW + 4
        )
        .is_err());
    assert!(passkeys
        .revalidate_step_up(&mut durable.scope(), &evidence, confirms, NOW + 400)
        .is_err());
    let expired = passkeys
        .begin_assertion(&mut durable.scope(), NOW + 5)
        .expect("expiring assertion");
    let credential = authenticator.assert(&expired.ceremony);
    durable.reopen();
    assert!(passkeys
        .finish_assertion(
            &mut durable.scope(),
            &expired.assertion_id,
            &credential,
            NOW + 306
        )
        .is_err());
    let listed =
        Passkeys::list_sessions_authorized(&durable.scope(), grant.session_id()).expect("sessions");
    let listed = listed
        .iter()
        .map(IdentityProjector::checked_session)
        .collect::<Result<Vec<_>, _>>()
        .expect("project sessions");
    validate("session.list", &json!({"sessions":listed}));
    assert_eq!(listed[0]["opened_at"], opened["opened_at"]);
    let revoked =
        Passkeys::revoke_session_authorized(&mut durable.scope(), grant.session_id(), NOW + 6)
            .expect("revoke");
    validate(
        "session.revoke",
        &IdentityProjector::checked_revocation(&revoked).expect("revocation"),
    );
    durable.reopen();
    assert!(Passkeys::reserve_refresh(
        &durable.scope(),
        grant.refresh_token().expose(),
        grant.csrf_token().expose(),
        NOW + 7
    )
    .is_err());
}

// This composition uses the real durable authorization index and production refresh
// consumer. It supplies no synthetic provider responses or replacement session state.
struct BrowserSessions {
    store: Mutex<PrincipalStore>,
    index: AuthDiscoveryIndex,
    passkeys: Passkeys,
}
impl HumanApiComponents for BrowserSessions {
    fn authorize(
        &self,
        operation: &Operation,
        credentials: SessionCredentials<'_>,
        trace: &str,
    ) -> Result<PrincipalContext, ApiFailure> {
        let disclosure = AuthorizationDisclosure {
            operation,
            destination: credentials.intended_destination,
            path_parameters: credentials.path_parameters,
            body: credentials.body,
            idempotency_key: credentials.idempotency_key,
            trace,
        };
        let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
        if credentials.refresh {
            authorize_refresh_execution(
                &mut store,
                &self.index,
                credentials.access_token,
                credentials.csrf_token.ok_or_else(ApiFailure::forbidden)?,
                disclosure,
                NOW,
                30,
            )
            .map_err(|_| ApiFailure::unauthenticated())
        } else {
            authorize_execution(
                &mut store,
                &self.passkeys,
                &self.index,
                ExecutionCredentials {
                    access: credentials.access_token,
                    csrf: credentials.csrf_token,
                    step_up: None,
                },
                disclosure,
                NOW,
                30,
            )
            .map_err(|_| ApiFailure::unauthenticated())?
            .into_context()
        }
    }
    fn admit_bearer_assertion(
        &self,
        _: &Operation,
        _: BearerCredentials<'_>,
        _: &str,
    ) -> Result<PrincipalContext, ApiFailure> {
        Err(ApiFailure::unauthenticated())
    }
    fn execute(&self, request: ScopedRequest<'_>) -> Result<BackendResponse, ApiFailure> {
        let context = request
            .principal
            .as_ref()
            .ok_or_else(ApiFailure::unauthenticated)?;
        let destination = if request.operation.name == "session.refresh" {
            "/v1/sessions/refresh"
        } else {
            "/v1/sessions"
        };
        consume_context(
            &self.index,
            context,
            AuthorizationDisclosure {
                operation: request.operation,
                destination,
                path_parameters: &request.path_parameters,
                body: &request.body,
                idempotency_key: request.idempotency_key.as_deref(),
                trace: &request.trace,
            },
            NOW,
        )
        .map_err(|_| ApiFailure::forbidden())?;
        let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
        let mut scope = store
            .principal(&context.principal)
            .map_err(|_| ApiFailure::unavailable())?;
        if scope.tenant() != &context.tenant {
            return Err(ApiFailure::forbidden());
        }
        match request.operation.name.as_str() {
            "session.refresh" => ProductionComponents::refresh_browser_session(
                &self.passkeys,
                &self.index,
                &request,
                &mut scope,
                &context.principal,
                NOW,
            ),
            "session.list" => {
                let sessions = Passkeys::list_sessions_authorized(&scope, &context.session_id)
                    .map_err(|_| ApiFailure::unauthenticated())?;
                let sessions = sessions
                    .iter()
                    .map(IdentityProjector::checked_session)
                    .collect::<Result<Vec<_>, _>>()?;
                Ok(BackendResponse {
                    result: json!({"sessions":sessions}),
                    session: None,
                })
            }
            _ => Err(ApiFailure::not_found()),
        }
    }
    fn stream(
        &self,
        _: ScopedRequest<'_>,
        _: usize,
        _: &mut dyn FnMut(Value) -> Result<(), ApiFailure>,
        _: &dyn Fn() -> bool,
    ) -> Result<(), ApiFailure> {
        Err(ApiFailure::not_found())
    }
    fn readiness(&self, _: &str) -> Result<Readiness, ApiFailure> {
        Err(ApiFailure::not_found())
    }
}

fn http(
    router: Arc<Router<UnixComponents>>,
    method: &str,
    path: &str,
    cookies: &str,
    csrf: &str,
) -> (String, Value) {
    let (mut client, mut server) = UnixStream::pair().expect("HTTP pair");
    client
        .set_read_timeout(Some(Duration::from_secs(10)))
        .expect("deadline");
    let worker = thread::spawn(move || {
        router
            .serve_one(&mut server, "repair-session")
            .expect("serve HTTP")
    });
    let body = if method == "POST" { "{}" } else { "" };
    write!(client,"{method} {path} HTTP/1.1\r\nHost: paxportwallet.com\r\nOrigin: {ORIGIN}\r\nCookie: {cookies}\r\nX-LayerX-CSRF: {csrf}\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{body}",body.len()).expect("request");
    let mut response = String::new();
    client.read_to_string(&mut response).expect("HTTP response");
    worker.join().expect("HTTP thread");
    let (head, body) = response.split_once("\r\n\r\n").expect("HTTP body");
    (
        head.to_owned(),
        serde_json::from_str(body).expect("JSON response"),
    )
}
fn cookie(head: &str, name: &str) -> String {
    head.lines()
        .find_map(|line| {
            line.strip_prefix(&format!("Set-Cookie: {name}="))
                .map(|value| value.split(';').next().expect("cookie").to_owned())
        })
        .expect("returned cookie")
}

#[test]
fn refresh_context_crosses_http_unix_and_production_consumer_after_restart() {
    let mut durable = Durable::new("repair-refresh");
    let passkeys = Passkeys::new(auth_config()).expect("passkeys");
    let mut authenticator = SoftwareAuthenticator::new();
    register(&mut durable.scope(), &passkeys, &mut authenticator);
    let challenge = passkeys
        .begin_assertion(&mut durable.scope(), NOW)
        .expect("assertion");
    let credential = authenticator.assert(&challenge.ceremony);
    let proof = passkeys
        .finish_assertion(
            &mut durable.scope(),
            &challenge.assertion_id,
            &credential,
            NOW,
        )
        .expect("proof");
    let grant = passkeys
        .open_session(
            &mut durable.scope(),
            &proof.assertion_id,
            Device::mint("Browser", "web").expect("device"),
            NOW,
        )
        .expect("grant");
    let index_root = support::directory("repair-index");
    fs::create_dir_all(&index_root).expect("index dir");
    fs::set_permissions(&index_root, fs::Permissions::from_mode(0o700)).expect("index permissions");
    let index = AuthDiscoveryIndex::open(
        &index_root,
        IndexAuthenticationKey::new([0x27; 32]).expect("index key"),
    )
    .expect("index");
    index
        .bind_session(&grant, &support::principal(ACCOUNT_ID))
        .expect("bind session");
    drop(index);
    durable.reopen();
    let socket_root = support::directory("repair-socket");
    fs::create_dir_all(&socket_root).expect("socket dir");
    fs::set_permissions(&socket_root, fs::Permissions::from_mode(0o700))
        .expect("socket permissions");
    let mut refresh = grant.refresh_token().expose().to_owned();
    let mut csrf = grant.csrf_token().expose().to_owned();
    let mut access = grant.access_token().expose().to_owned();
    for restart in 0..2 {
        let index = AuthDiscoveryIndex::open(
            &index_root,
            IndexAuthenticationKey::new([0x27; 32]).expect("key"),
        )
        .expect("reopened index");
        let components = BrowserSessions {
            store: Mutex::new(durable.store.take().expect("store")),
            index,
            passkeys: passkeys.clone(),
        };
        let clock = Arc::new(support::StepClock::new(NOW * 1000));
        let socket = socket_root.join(format!("human-{restart}.sock"));
        let bound = HumanComponentServer::new_with_clock(Arc::new(components), clock.clone())
            .bind(ComponentServerConfig {
                socket_path: socket.clone(),
                allowed_uid: rustix::process::getuid().as_raw(),
                worker_count: 1,
                queue_capacity: 8,
                limits: default_component_limits(),
            })
            .expect("bind actual server");
        let shutdown = bound.shutdown();
        let worker = thread::spawn(move || bound.run().expect("component server"));
        let backend =
            UnixComponents::new_with_clock(&socket, default_component_limits(), clock.clone())
                .expect("Unix components");
        let router = Arc::new(
            Router::new(
                Arc::new(backend),
                PrincipalLimits::new(100, 60, 100).expect("limits"),
                HttpConfig {
                    maximum_header_bytes: 32768,
                    maximum_body_bytes: 1048576,
                    allowed_origin: ORIGIN.into(),
                    service_version: "repair".into(),
                },
                clock,
            )
            .expect("router"),
        );
        let cookies=format!("__Host-layerx_access={access}; __Host-layerx_refresh={refresh}; __Host-layerx_csrf={csrf}");
        let (head, result) = http(
            router.clone(),
            "POST",
            "/v1/sessions/refresh",
            &cookies,
            &csrf,
        );
        assert!(head.starts_with("HTTP/1.1 200 "), "{head}: {result}");
        assert_eq!(result["result"]["session_id"], grant.session_id());
        assert_eq!(result["result"]["opened_at"], stamp(NOW));
        let next_refresh = cookie(&head, "__Host-layerx_refresh");
        let next_csrf = cookie(&head, "__Host-layerx_csrf");
        let next_access = cookie(&head, "__Host-layerx_access");
        assert_ne!(next_refresh, refresh);
        let (replay, body) = http(
            router.clone(),
            "POST",
            "/v1/sessions/refresh",
            &cookies,
            &csrf,
        );
        assert!(
            !replay.starts_with("HTTP/1.1 200 "),
            "replayed refresh accepted: {body}"
        );
        let cookies = format!("__Host-layerx_access={next_access}; __Host-layerx_csrf={next_csrf}");
        let (head, body) = http(router.clone(), "GET", "/v1/sessions", &cookies, &next_csrf);
        assert!(head.starts_with("HTTP/1.1 200 "), "{head}: {body}");
        assert_eq!(
            body["result"]["sessions"][0]["session_id"],
            grant.session_id()
        );
        drop(router);
        shutdown.request();
        worker.join().expect("server thread");
        durable.reopen();
        refresh = next_refresh;
        csrf = next_csrf;
        access = next_access;
    }
    fs::remove_dir_all(index_root).expect("remove index");
    fs::remove_dir_all(socket_root).expect("remove sockets");
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|v| format!("{v:02x}")).collect()
}
fn evidence(scope: &PrincipalScope<'_>, id: &str) -> Value {
    let schema = ApiSchema::v1().expect("schema");
    let response = ProductionComponents::evidence_get(
        scope,
        &ScopedRequest {
            operation: schema.operation("evidence.get").expect("evidence.get"),
            principal: None,
            path_parameters: BTreeMap::from([("evidence_id".into(), id.into())]),
            body: json!({}),
            idempotency_key: None,
            trace: "trc_00112233445566778899aabbccddeeff".into(),
        },
    )
    .expect("real evidence consumer");
    validate("evidence.get", &response.result);
    response.result
}
#[test]
fn balance_material_preserves_optional_rung_through_cache_and_reopen() {
    use layerx_human_service::server::agent_runtime::VerifiedBalance;
    let mut durable = Durable::new("repair-balance");
    for rung in [
        None,
        Some(Rung::Submitted),
        Some(Rung::Receipted),
        Some(Rung::Committed),
    ] {
        // Exercise the real production material codec; cryptographic state-proof
        // authentication remains the responsibility of the daemon balance producer.
        let balance = AccountBalance::from_agent(VerifiedBalance {
            account: [1; 32],
            asset: [2; 32],
            currency: "LXP".into(),
            observed_at: stamp(NOW),
            age_seconds: 0,
            amount: 123,
            verification: rung,
            global_sequence: 10,
            batch_number: 1,
            observed_head_sequence: 10,
            observed_checkpoint: [0; 32],
            canonical_bytes: 123u128.to_be_bytes().to_vec(),
            proof_material: serde_json::to_vec(
                &json!({"achieved_verification_level":rung.map(Rung::name)}),
            )
            .expect("source material"),
        })
        .expect("balance producer");
        let material = balance.evidence_material().expect("production material");
        assert_eq!(&material[..5], b"LXHB2");
        assert_eq!(
            AccountBalance::material_rung(&material).expect("material decoder"),
            rung
        );
        let mut legacy = b"LXHB1".to_vec();
        if let Some(rung) = rung {
            legacy.push(rung.wire_rank());
        }
        legacy.extend_from_slice(&material[if rung.is_some() { 7 } else { 6 }..]);
        assert_eq!(
            AccountBalance::material_rung(&legacy).expect("legacy producer decoder"),
            rung
        );
        for bytes in [material, legacy] {
            let digest: [u8; 32] = Sha256::digest(&bytes).into();
            let id = format!("evd_{}", hex(&digest));
            durable
                .scope()
                .put(
                    Table::Cache,
                    support::row_key(&format!("state-proof-{}", hex(&digest))),
                    NOW,
                    bytes.clone(),
                )
                .expect("cache material");
            let projected = balance.project(digest, 60);
            validate("account.balance", &projected);
            for _ in 0..2 {
                let result = evidence(&durable.scope(), &id);
                assert_eq!(result["verification"], projected["verification"]);
                assert_eq!(
                    STANDARD
                        .decode(result["bytes_base64"].as_str().expect("material"))
                        .expect("decode material"),
                    bytes
                );
                durable.reopen();
            }
        }
        let mut invalid = balance.evidence_material().expect("material");
        invalid[5] = 9;
        assert!(AccountBalance::material_rung(&invalid).is_err());
    }
}

#[test]
fn onboarding_projects_persisted_identity_and_local_progress_after_restart() {
    use layerx_human_service::custody::{EnvelopeKms, Keystore};
    use layerx_human_service::onboarding::{OnboardingJourney, OnboardingStart, RecoveryPolicy};
    use layerx_types::ids::Did;
    use layerx_types::intent::{ApprovalThreshold, RecoveryRoot};
    let mut durable = Durable::new("repair-onboarding");
    let root = support::directory("repair-custody");
    fs::create_dir_all(&root).expect("custody root");
    let secret = root.join("kms-root");
    fs::write(&secret, [0x51u8; 64]).expect("KMS root");
    let keystore = Keystore::open_development(
        root.join("keys"),
        77,
        EnvelopeKms::new("kms://repair", secret).expect("KMS"),
    )
    .expect("keystore");
    let start = OnboardingStart::new(
        [8; 32],
        Did::new(b"did:layerx:mara").expect("did"),
        RecoveryPolicy::new(
            RecoveryRoot::new([9; 32]),
            ApprovalThreshold::new(2).expect("threshold"),
            86400,
        )
        .expect("recovery"),
    )
    .expect("start");
    let mut journey = OnboardingJourney::start(&mut durable.scope(), &start, NOW)
        .expect("start persisted journey");
    let queued =
        IdentityProjector::onboarding(&durable.scope(), &journey.status()).expect("projection");
    validate("onboarding.status", &queued);
    validate(
        "account.create",
        &json!({"account_id":ACCOUNT_ID,"onboarding":queued}),
    );
    assert_eq!(queued["journey_id"], format!("jrn_{}", hex(&[8; 32])));
    assert_ne!(queued["state"], "done");
    let status = journey
        .resume_local(&mut durable.scope(), &keystore, NOW + 1)
        .expect("real custody resume");
    let before =
        IdentityProjector::onboarding(&durable.scope(), &status).expect("local projection");
    validate("onboarding.resume", &before);
    assert_eq!(before["journey_id"], queued["journey_id"]);
    assert_ne!(before["state"], "done");
    assert!(before["stages"]
        .as_array()
        .expect("stages")
        .iter()
        .all(|stage| stage["state"] != "local-complete"));
    for item in before["evidence"].as_array().expect("evidence") {
        assert_eq!(item["verification"], "unverified");
        let result = evidence(
            &durable.scope(),
            item["evidence_id"].as_str().expect("reference"),
        );
        assert_eq!(result["class"], item["class"]);
    }
    durable.reopen();
    let mut reopened = OnboardingJourney::load(&durable.scope())
        .expect("load journey")
        .expect("retained");
    let after = IdentityProjector::onboarding(&durable.scope(), &reopened.status())
        .expect("recovered projection");
    assert_eq!(before, after);
    let status = reopened
        .resume_local(&mut durable.scope(), &keystore, NOW + 2)
        .expect("idempotent custody resume");
    let resumed =
        IdentityProjector::onboarding(&durable.scope(), &status).expect("resume projection");
    validate("onboarding.resume", &resumed);
    assert_eq!(resumed["journey_id"], before["journey_id"]);
    assert_ne!(resumed["state"], "done");
    fs::remove_dir_all(root).expect("remove custody");
}

#[test]
fn activity_dates_pages_and_statement_time_follow_persisted_feed() {
    use layerx_human_service::activity::{EvidenceExport, Feed, PageRequest};
    use layerx_human_service::audit::{AuditChain, AuditEvent, SecurityChangeKind, StepUpEvidence};
    use layerx_human_service::trace::TraceId;
    let mut durable = Durable::new("repair-activity");
    let feed = Feed::new(60).expect("feed");
    let empty = feed
        .page(
            &durable.scope(),
            PageRequest::new(2, activity_filters(&json!({})).expect("filters")),
            NOW,
            0,
        )
        .expect("empty page");
    let empty = activity_page_json(&empty, "LXP").expect("project page");
    validate("activity.query", &empty);
    assert_eq!(empty["filter"], json!({}));
    assert_eq!(empty["groups"], json!([]));
    assert_eq!(empty["next_cursor"], "cur_end");
    for time in [NOW - 20, NOW - 10, NOW, NOW + 10, NOW + 20] {
        let mut scope = durable.scope();
        let mut chain = AuditChain::open(&scope).expect("audit chain");
        chain
            .append(
                &mut scope,
                time,
                &TraceId::mint([0x31; 16]),
                &AuditEvent::SecurityChange {
                    change: SecurityChangeKind::DeviceAdded,
                    step_up: StepUpEvidence::NotRequired,
                },
                &[],
            )
            .expect("real audit producer");
        let entries = chain.entries(&scope).expect("audit entries");
        Feed::record_security(&mut scope, entries.last().expect("audit entry"), time)
            .expect("feed projection");
    }
    let filter_json = json!({"from":stamp(NOW-10),"to":stamp(NOW+10)});
    let filters = activity_filters(&json!({"filter":filter_json})).expect("timestamp filter");
    assert_eq!(activity_filter_json(&filters).expect("echo"), filter_json);
    assert!(activity_filters(&json!({"filter":{"from":"not-a-time"}})).is_err());
    assert!(activity_filters(&json!({"filter":{"from":stamp(NOW+1),"to":stamp(NOW)}})).is_err());
    let first = feed
        .page(
            &durable.scope(),
            PageRequest::new(2, filters.clone()),
            NOW + 21,
            0,
        )
        .expect("first page");
    assert_eq!(first.entries().len(), 2);
    assert_eq!(first.entries()[0].occurred_at(), NOW + 10);
    let page = activity_page_json(&first, "LXP").expect("page");
    validate("activity.query", &page);
    assert_eq!(page["filter"], filter_json);
    let cursor = first.next().expect("continuation").clone();
    durable.reopen();
    let second = feed
        .page(
            &durable.scope(),
            PageRequest::new(2, filters.clone()).after(cursor),
            NOW + 22,
            0,
        )
        .expect("reopened continuation");
    assert_eq!(second.entries().len(), 1);
    assert_eq!(second.entries()[0].occurred_at(), NOW - 10);
    assert!(second.next().is_none());
    validate(
        "activity.query",
        &activity_page_json(&second, "LXP").expect("second projection"),
    );
    let exporter = EvidenceExport::new(feed, 1048576).expect("exporter");
    let statement = exporter
        .statement(&durable.scope(), &filters, NOW + 22, 0)
        .expect("statement");
    assert_eq!(statement.entries(), 3);
    let bytes = statement.content().to_vec();
    let result = cache_activity_export(
        &mut durable.scope(),
        bytes.clone(),
        false,
        Vec::new(),
        NOW + 22,
    )
    .expect("statement cache");
    validate("activity.export.statement", &result);
    assert_eq!(result["created_at"], stamp(NOW + 22));
    let id = result["download_path"]
        .as_str()
        .expect("download")
        .strip_prefix("/v1/evidence/")
        .expect("evidence path")
        .to_owned();
    let (bundle, _) = exporter
        .evidence(
            &durable.scope(),
            &filters,
            &[],
            Vec::new(),
            &[],
            NOW + 22,
            0,
        )
        .expect("security-only bundle");
    let bundle_bytes = bundle.encode().expect("bundle encoding");
    let bundle_result = cache_activity_export(
        &mut durable.scope(),
        bundle_bytes.clone(),
        true,
        Vec::new(),
        NOW + 22,
    )
    .expect("evidence export cache");
    validate("activity.export.evidence", &bundle_result);
    assert_eq!(bundle_result["created_at"], result["created_at"]);
    durable.reopen();
    let download = evidence(&durable.scope(), &id);
    assert_eq!(
        STANDARD
            .decode(download["bytes_base64"].as_str().expect("download bytes"))
            .expect("base64"),
        bytes
    );
    let key = format!("activity-evidence-{}", hex(&Sha256::digest(&bundle_bytes)));
    assert_eq!(
        durable
            .scope()
            .get(Table::Cache, &support::row_key(&key))
            .expect("reopened bundle")
            .bytes(),
        bundle_bytes
    );
    bundle
        .verify(
            Sha256::digest(&bundle_bytes).into(),
            &support::principal(ACCOUNT_ID),
            &[],
        )
        .expect("independent local bundle verification");
}

#[test]
fn node_receipt_material_has_exact_schema_and_independently_verifiable_portable_payload() {
    use layerx_human_service::journeys::{ReceiptKeyEntry, VerifiedLegEvidence};
    use layerx_portable::PortableReceiptV2;
    use layerx_proof::inclusion::SequencerAuthorization;
    use layerx_proof::receipt::{verify_sequencer_signature, AuthorizedBatch, ReceiptKeys};
    use layerx_types::json::{parse, JsonValue};
    fn bytes(value: &JsonValue, path: &str) -> Vec<u8> {
        value.hex_at(path).expect("fixture hex")
    }
    fn hash(value: &JsonValue, path: &str) -> [u8; 32] {
        bytes(value, path).try_into().expect("hash width")
    }
    let document = parse(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )))
    .expect("node-produced receipt fixtures");
    let mut checked = 0;
    for case in document.array_at("cases").expect("cases") {
        if case.str_at("vector").expect("vector") != "V1" {
            continue;
        }
        let signed = bytes(case, "signedReceipt");
        let pins = [SequencerAuthorization::new(
            hash(case, "pinnedKey.keyId"),
            hash(case, "pinnedKey.publicKey"),
            case.u64_at("pinnedKey.firstHeight").expect("first height"),
            case.u64_at("pinnedKey.lastHeight").expect("last height"),
        )];
        let receipt = verify_sequencer_signature(&signed, ReceiptKeys::HeightRanges(&pins))
            .expect("independent node signature verification");
        let protocol = receipt.protocol().expect("protocol receipt");
        assert_eq!(protocol.module_id(), 12);
        assert_eq!(protocol.operation(), 2);
        let height = u64::from_str_radix(
            case.str_at("receiptJson.height")
                .expect("height")
                .strip_prefix("0x")
                .expect("hex height"),
            16,
        )
        .expect("height value");
        let digest = Sha256::digest(&signed).into();
        let rung =
            Rung::from_name(case.str_at("receiptJson.rung").expect("producer rung")).expect("rung");
        assert_eq!(rung, Rung::Receipted);
        let batch = AuthorizedBatch::from_receipt_keys(
            &signed,
            ReceiptKeys::HeightRanges(&pins),
            protocol.asset(),
            protocol.previous_state_root(),
            protocol.resulting_state_root(),
        )
        .expect("authorized batch");
        let evidence = VerifiedLegEvidence {
            action_key: hash(case, "receiptJson.txHash"),
            activity_id: protocol.activity_id(),
            canonical_receipt: signed.clone(),
            receipt_digest: digest,
            authorised_batch: batch,
            tx_hash: hash(case, "receiptJson.txHash"),
            block_height: height,
            rung,
            signed_receipt: signed.clone(),
            receipt_key: ReceiptKeyEntry {
                key_id: hash(case, "pinnedKey.keyId"),
                public_key: hash(case, "pinnedKey.publicKey"),
                first_height: case.u64_at("pinnedKey.firstHeight").expect("first height"),
                last_height: case.u64_at("pinnedKey.lastHeight").expect("last height"),
            },
            inclusion_height: height,
        };
        let id = format!("evd_{}", hex(&digest));
        let material = ProductionComponents::managed_receipt_material(&id, &evidence)
            .expect("production material");
        validate("evidence.get", &material);
        assert_eq!(material["verification"], "receipt-verified");
        assert_eq!(material["rung"], "receipted");
        for extra in ["signed_receipt", "tx_hash", "receipt_key"] {
            assert!(material.get(extra).is_none());
        }
        let portable = STANDARD
            .decode(material["bytes_base64"].as_str().expect("portable bytes"))
            .expect("base64 portable");
        PortableReceiptV2::from_json(&portable)
            .expect("portable payload")
            .verify_v2(&pins)
            .expect("independent portable verification");
        assert!(ProductionComponents::managed_receipt_material(
            &format!("evd_{}", hex(&[0; 32])),
            &evidence
        )
        .is_err());
        checked += 1;
    }
    assert_eq!(checked, 5);
}

#[test]
fn security_key_export_projects_real_custody_mutation_and_refuses_retry_after_reopen() {
    use layerx_human_service::custody::{EnvelopeKms, KeyClass, KeyId, Keystore};
    use layerx_human_service::security::KeyExportCeremony;
    use layerx_human_service::trace::TraceId;
    let mut durable = Durable::new("repair-export-key");
    let root = support::directory("repair-export-custody");
    fs::create_dir_all(&root).expect("custody root");
    let secret = root.join("kms-root");
    fs::write(&secret, [0x61u8; 64]).expect("KMS root");
    let key = KeyId::new("primary").expect("key id");
    let keystore = Keystore::open_development(
        root.join("keys"),
        77,
        EnvelopeKms::new("kms://repair-export", secret.clone()).expect("KMS"),
    )
    .expect("keystore");
    keystore
        .create(
            &support::principal(ACCOUNT_ID),
            &key,
            KeyClass::HumanPrimary,
        )
        .expect("create real primary key");
    let ceremony = KeyExportCeremony::new(key.clone(), 300).expect("key export");
    let challenge = ceremony
        .begin(&keystore, &support::principal(ACCOUNT_ID), NOW)
        .expect("begin key export");
    validate(
        "security.key-export.begin",
        &IdentityProjector::key_export_challenge(&challenge).expect("challenge projection"),
    );
    let passkeys = Passkeys::new(auth_config()).expect("passkeys");
    let mut authenticator = SoftwareAuthenticator::new();
    register(&mut durable.scope(), &passkeys, &mut authenticator);
    let stepup = passkeys
        .begin_step_up_authorized(&mut durable.scope(), challenge.confirms, NOW)
        .expect("key export stepup");
    let credential = authenticator.assert(&stepup.ceremony);
    let proof = passkeys
        .finish_step_up(
            &mut durable.scope(),
            &stepup.challenge_id,
            &credential,
            NOW + 1,
        )
        .expect("verified stepup");
    let exported = ceremony
        .finish(
            &keystore,
            &mut durable.scope(),
            &challenge.export_id,
            &proof,
            &TraceId::mint([0x32; 16]),
            NOW + 2,
        )
        .expect("export primary once");
    let result = IdentityProjector::exported_key(&exported).expect("export projection");
    validate("security.key-export.finish", &result);
    assert_eq!(result["self_custodied_at"], stamp(NOW + 2));
    assert_eq!(result["secret"]["remask_at"], stamp(NOW + 302));
    assert!(keystore
        .self_custodied(&support::principal(ACCOUNT_ID), &key)
        .expect("self custody"));
    drop(keystore);
    durable.reopen();
    let keystore = Keystore::open_development(
        root.join("keys"),
        77,
        EnvelopeKms::new("kms://repair-export", secret).expect("reopened KMS"),
    )
    .expect("reopened keystore");
    assert!(ceremony
        .begin(&keystore, &support::principal(ACCOUNT_ID), NOW + 3)
        .is_err());
    assert!(ceremony
        .finish(
            &keystore,
            &mut durable.scope(),
            &challenge.export_id,
            &proof,
            &TraceId::mint([0x33; 16]),
            NOW + 3
        )
        .is_err());
    fs::remove_dir_all(root).expect("remove custody");
}
