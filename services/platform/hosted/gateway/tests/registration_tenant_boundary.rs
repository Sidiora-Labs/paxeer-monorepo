use ed25519_dalek::{Signer, SigningKey};
use layerx_platform_gateway::runtime::Server;
use layerx_platform_identity::store::{PendingSignup, Principal};
use native_tls::{Certificate, TlsConnector};
use rustls::pki_types::{PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::ServerConfig;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::ffi::OsString;
use std::fs;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::Path;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::Duration;

#[allow(dead_code)]
mod identity {
    include!("../../identity/src/main.rs");

    pub struct Running {
        shared: Arc<Shared>,
        pub listener: super::Listener,
    }

    impl Running {
        pub fn open(state: &Path, tls: Arc<ServerConfig>) -> Self {
            let limits = Limits::default();
            let store = Store::open(state).expect("production identity store");
            store.probe_writable().expect("identity store readiness");
            let shared = Arc::new(Shared {
                config: Config {
                    listen: "127.0.0.1:0".parse().unwrap(),
                    health: "127.0.0.1:0".parse().unwrap(),
                    tls,
                    state_dir: state.to_owned(),
                    store_key: StoreKey::derive(b"tenant-boundary-test-store-key"),
                    service_tokens: vec![ServiceToken {
                        service: Service::Provisioning,
                        token: Zeroizing::new(super::PROVISIONING_TOKEN.to_owned()),
                    }],
                    default_ttl_seconds: 3600,
                    signup_limits: limits,
                    disposable_domains: Vec::new(),
                    outbox_dir: state.join("signup-outbox"),
                    smtp: None,
                },
                store: Mutex::new(store),
                desk: Mutex::new(Desk::new(limits, Vec::new())),
            });
            let worker = Arc::clone(&shared);
            let listener = super::Listener::start(move |tcp| {
                handle_connection(&worker, tcp).expect("production identity HTTP handler");
            });
            Self { shared, listener }
        }

        pub fn principal(&self, tenant: &str, sub: &str) -> Option<Principal> {
            self.shared
                .store
                .lock()
                .unwrap()
                .principal(tenant, sub)
                .cloned()
        }

        pub fn pending(&self) -> Vec<store::PendingSignup> {
            let store = self.shared.store.lock().unwrap();
            store
                .signup_deliveries()
                .iter()
                .map(|(digest, _)| {
                    store
                        .pending_signup(digest)
                        .expect("durable pending signup")
                        .clone()
                })
                .collect()
        }

        pub fn signup_sizes(&self) -> (usize, usize, usize, usize, usize, usize) {
            self.shared.store.lock().unwrap().signup_state_sizes()
        }
    }

    pub fn reply(stream: &mut impl Read) -> (u16, serde_json::Value) {
        let response = read_http_message(stream, MAX_REQUEST_BYTES).expect("HTTP response");
        assert_eq!(response.headers["content-type"], "application/json");
        let status = response.headers[""]
            .split_whitespace()
            .nth(1)
            .unwrap()
            .parse()
            .unwrap();
        (
            status,
            serde_json::from_slice(&response.body).expect("JSON response"),
        )
    }
}

const PROVISIONING_TOKEN: &str = "tenant-boundary-provisioning-token";

struct Listener {
    address: SocketAddr,
    stopped: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl Listener {
    fn start(handler: impl Fn(TcpStream) + Send + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("production listener");
        let address = listener.local_addr().unwrap();
        let stopped = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&stopped);
        let worker = thread::spawn(move || {
            for connection in listener.incoming() {
                let tcp = connection.expect("accepted connection");
                if stop.load(Ordering::Acquire) {
                    break;
                }
                handler(tcp);
            }
        });
        Self {
            address,
            stopped,
            worker: Some(worker),
        }
    }

    fn connect(&self) -> TcpStream {
        let tcp = TcpStream::connect(self.address).expect("local connection");
        tcp.set_read_timeout(Some(Duration::from_secs(10))).unwrap();
        tcp.set_write_timeout(Some(Duration::from_secs(10)))
            .unwrap();
        tcp
    }
}

impl Drop for Listener {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        TcpStream::connect(self.address).expect("stop listener");
        self.worker
            .take()
            .unwrap()
            .join()
            .expect("listener stopped");
    }
}

struct Environment(Vec<(OsString, OsString)>);

impl Environment {
    fn clear() -> Self {
        let previous: Vec<_> = std::env::vars_os()
            .filter(|(key, _)| key.to_string_lossy().starts_with("LAYERX_GATEWAY_"))
            .collect();
        for (key, _) in &previous {
            std::env::remove_var(key);
        }
        Self(previous)
    }
}

impl Drop for Environment {
    fn drop(&mut self) {
        for (key, _) in std::env::vars_os()
            .filter(|(key, _)| key.to_string_lossy().starts_with("LAYERX_GATEWAY_"))
        {
            std::env::remove_var(key);
        }
        for (key, value) in &self.0 {
            std::env::set_var(key, value);
        }
    }
}

fn secret(root: &Path, name: &str, value: &str) {
    let path = root.join(name);
    fs::write(&path, value).unwrap();
    fs::set_permissions(&path, fs::Permissions::from_mode(0o600)).unwrap();
    std::env::set_var(format!("LAYERX_GATEWAY_{name}_FILE"), path);
}

fn post(
    stream: &mut (impl Read + Write),
    path: &str,
    token: Option<&str>,
    body: Value,
) -> (u16, Value) {
    let authorization = token.map_or_else(String::new, |token| {
        format!("Authorization: Bearer {token}\r\n")
    });
    let body = body.to_string();
    write!(stream, "POST {path} HTTP/1.1\r\nHost: localhost\r\n{authorization}Content-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).unwrap();
    stream.flush().unwrap();
    identity::reply(stream)
}

fn digest(domain: &[u8], tenant: &str, key: &[u8]) -> [u8; 32] {
    let mut hash = Sha256::new();
    for part in [domain, tenant.as_bytes(), key] {
        hash.update((part.len() as u64).to_be_bytes());
        hash.update(part);
    }
    hash.finalize().into()
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn subject(tenant: &str, key: &SigningKey) -> String {
    format!(
        "{tenant}.{}",
        &hex(&digest(
            b"layerx-register-subject-v1",
            tenant,
            &key.verifying_key().to_bytes()
        ))[..32]
    )
}

fn register(gateway: &Listener, tenant: &str, key: &SigningKey, email: Option<&str>) -> Value {
    let signer = key.verifying_key().to_bytes();
    let signature = key.sign(&digest(b"layerx-register-binding-v1", tenant, &signer));
    let mut params = vec![json!(hex(&signer)), json!(hex(&signature.to_bytes()))];
    if let Some(email) = email {
        params.push(json!(email));
    }
    let (status, reply) = post(
        &mut gateway.connect(),
        "/",
        None,
        json!({
            "jsonrpc":"2.0", "id":1, "method":"px_register", "params":params,
        }),
    );
    assert_eq!(status, 200, "{reply}");
    assert_eq!(reply["jsonrpc"], "2.0");
    assert_eq!(reply["id"], 1);
    reply
}

#[test]
fn registration_tenant_subject_boundary() {
    let _environment = Environment::clear();
    let root = tempfile::tempdir().unwrap();
    let certificate = rcgen::generate_simple_self_signed(vec!["localhost".into()]).unwrap();
    let tls = Arc::new(
        ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
            .with_safe_default_protocol_versions()
            .unwrap()
            .with_no_client_auth()
            .with_single_cert(
                vec![certificate.cert.der().clone()],
                PrivateKeyDer::from(PrivatePkcs8KeyDer::from(
                    certificate.signing_key.serialize_der(),
                )),
            )
            .unwrap(),
    );
    let ca = root.path().join("ca.der");
    fs::write(&ca, certificate.cert.der()).unwrap();
    let connector = TlsConnector::builder()
        .add_root_certificate(Certificate::from_der(certificate.cert.der()).unwrap())
        .build()
        .unwrap();
    let state = root.path().join("identity");
    let identity = identity::Running::open(&state, Arc::clone(&tls));
    for (name, value) in [
        ("LISTEN", "127.0.0.1:0"),
        ("LISTENER", "plain"),
        ("NETWORK_ID", "tenant-boundary"),
        ("PROTOCOL_NETWORK_ID", "7"),
        ("REDIS_URL", "rediss://localhost:1"),
    ] {
        std::env::set_var(format!("LAYERX_GATEWAY_{name}"), value);
    }
    std::env::set_var(
        "LAYERX_GATEWAY_LXP_WIRE_VERSION",
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string(),
    );
    std::env::set_var("LAYERX_GATEWAY_OUTBOUND_CA_DER", ca);
    std::env::set_var(
        "LAYERX_GATEWAY_IDENTITY_URL",
        format!("https://localhost:{}", identity.listener.address.port()),
    );
    secret(root.path(), "REDIS_USERNAME", "gateway");
    secret(root.path(), "REDIS_PASSWORD", "boundary-test-password");
    secret(root.path(), "IDENTITY_TOKEN", "boundary-test-gateway-token");
    secret(
        root.path(),
        "IDENTITY_PROVISIONING_TOKEN",
        PROVISIONING_TOKEN,
    );

    let boundary = "a".repeat(95);
    let too_long = ["a".repeat(96), "a".repeat(128)];
    assert!(
        Server::from_environment().is_err(),
        "missing tenant must fail configuration"
    );
    for tenant in [
        "",
        "Upper",
        "colon:tenant",
        "white space",
        "slash/tenant",
        "é",
    ]
    .into_iter()
    .chain(too_long.iter().map(String::as_str))
    {
        std::env::set_var("LAYERX_GATEWAY_TENANT", tenant);
        assert!(
            matches!(Server::from_environment(), Err(error) if error == "invalid gateway registration tenant"),
            "invalid tenant: {tenant:?}"
        );
    }

    std::env::set_var("LAYERX_GATEWAY_TENANT", &boundary);
    let server = Server::from_environment().expect("95-byte tenant configuration");
    let gateway = Listener::start(move |tcp| {
        server
            .serve_connection(tcp)
            .expect("production gateway HTTP handler")
    });
    let direct_key = SigningKey::from_bytes(&[7; 32]);
    let signup_key = SigningKey::from_bytes(&[8; 32]);
    let mut accepted_principals: Vec<Principal> = Vec::new();
    let mut accepted_signups: Vec<PendingSignup> = Vec::new();
    for (index, tenant) in [boundary.as_str(), "a", "alpha-1_beta.example"]
        .into_iter()
        .enumerate()
    {
        std::env::set_var("LAYERX_GATEWAY_TENANT", tenant);
        let configured = Server::from_environment().expect("valid shorter tenant configuration");
        drop(configured);
        let direct_sub = subject(tenant, &direct_key);
        let signup_sub = subject(tenant, &signup_key);
        assert_eq!(direct_sub.len(), tenant.len() + 33);
        assert_eq!(signup_sub.len(), tenant.len() + 33);
        if index == 0 {
            assert_eq!(direct_sub.len(), 128);
            assert_eq!(signup_sub.len(), 128);
        }
        let expected = Principal {
            tenant: tenant.into(),
            sub: direct_sub.clone(),
            allowed_signer_public_keys: vec![hex(&direct_key.verifying_key().to_bytes())],
            account: None,
            audiences: Vec::new(),
        };
        let reply = register(&gateway, tenant, &direct_key, None);
        assert_eq!(
            reply["result"],
            serde_json::to_value(&expected).unwrap(),
            "{reply}"
        );
        assert_eq!(
            identity.principal(tenant, &direct_sub),
            Some(expected.clone())
        );
        accepted_principals.push(expected);

        let email = format!("boundary{index}@example.com");
        let reply = register(&gateway, tenant, &signup_key, Some(&email));
        assert_eq!(reply["result"]["pending"], true, "{reply}");
        assert_eq!(reply["result"]["tenant"], tenant);
        assert_eq!(reply["result"]["sub"], signup_sub);
        assert!(reply["result"].get("token").is_none());
        assert!(identity.principal(tenant, &signup_sub).is_none());
        let pending = identity
            .pending()
            .into_iter()
            .find(|pending| pending.sub == signup_sub)
            .expect("HTTP signup persisted");
        assert_eq!(pending.tenant, tenant);
        assert_eq!(
            pending.signer_public_key,
            hex(&signup_key.verifying_key().to_bytes())
        );
        assert_eq!(pending.email_digest, hex(&Sha256::digest(email.as_bytes())));
        assert_eq!(json!(pending.expires_at), reply["result"]["expires_at"]);
        accepted_signups.push(pending);
    }

    let sizes = identity.signup_sizes();
    let journal = fs::read(state.join("journal.log")).unwrap();
    for tenant in &too_long {
        std::env::set_var("LAYERX_GATEWAY_TENANT", tenant);
        assert!(
            matches!(Server::from_environment(), Err(error) if error == "invalid gateway registration tenant")
        );
        for (key, email) in [
            (&direct_key, None),
            (&signup_key, Some("refused@example.com")),
        ] {
            let reply = register(&gateway, tenant, key, email);
            assert_eq!(reply["error"]["code"], -32001, "{reply}");
            assert_eq!(
                reply["error"]["data"]["code"],
                "registration_tenant_not_configured"
            );
            assert!(reply.get("result").is_none());
            assert!(identity.principal(tenant, &subject(tenant, key)).is_none());
        }
        for path in ["/v1/principals", "/v1/signup"] {
            let key = if path == "/v1/principals" {
                &direct_key
            } else {
                &signup_key
            };
            let sub = subject(tenant, key);
            assert_eq!(sub.len(), tenant.len() + 33);
            assert!(sub.len() > 128);
            let signer = hex(&key.verifying_key().to_bytes());
            let body = if path == "/v1/principals" {
                json!({"tenant":tenant,"sub":sub,"allowed_signer_public_keys":[signer]})
            } else {
                json!({"tenant":tenant,"sub":sub,"signer_public_key":signer,"email":"refused@example.com","client_ip":"127.0.0.1"})
            };
            let mut stream = connector
                .connect("localhost", identity.listener.connect())
                .expect("real identity TLS connection");
            let (status, reply) = post(&mut stream, path, Some(PROVISIONING_TOKEN), body);
            assert_eq!(status, 400, "{path}: {reply}");
            assert_eq!(reply["error"]["code"], "invalid_argument");
            assert!(identity.principal(tenant, &sub).is_none());
        }
        assert_eq!(
            identity.signup_sizes(),
            sizes,
            "refusals cannot create pending signups or admissions"
        );
        assert_eq!(
            fs::read(state.join("journal.log")).unwrap(),
            journal,
            "refusals cannot write identity state"
        );
    }
    drop(gateway);
    drop(identity);

    let reopened = identity::Running::open(&state, tls);
    for principal in accepted_principals {
        assert_eq!(
            reopened.principal(&principal.tenant, &principal.sub),
            Some(principal.clone()),
            "accepted principal survives reopen"
        );
    }
    let pending = reopened.pending();
    assert_eq!(pending.len(), accepted_signups.len());
    for signup in accepted_signups {
        assert!(
            pending.contains(&signup),
            "accepted pending signup survives reopen"
        );
        assert!(reopened.principal(&signup.tenant, &signup.sub).is_none());
    }
    for tenant in &too_long {
        assert!(reopened
            .principal(tenant, &subject(tenant, &direct_key))
            .is_none());
        assert!(!pending.iter().any(|signup| &signup.tenant == tenant));
    }
}
