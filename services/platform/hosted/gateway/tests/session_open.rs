use ed25519_dalek::{Signer, SigningKey};
use layerx_platform_gateway::runtime::Server;
use native_tls::Certificate;
use rustls::pki_types::{PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::{ServerConfig, ServerConnection, StreamOwned};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::fs;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::Path;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

const TENANT: &str = "beta";
const PROVISIONING_TOKEN: &str = "session-open-provisioning-token";
const IDENTITY_TOKEN: &str = "session-open-gateway-token";

struct IdentityPeer {
    address: SocketAddr,
    stopped: Arc<AtomicBool>,
    calls: Arc<Mutex<Vec<(String, Value)>>>,
    wrong_subject: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl IdentityPeer {
    fn start(tls: Arc<ServerConfig>) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("identity listener: {error}"));
        let address = listener
            .local_addr()
            .unwrap_or_else(|error| panic!("identity address: {error}"));
        let stopped = Arc::new(AtomicBool::new(false));
        let calls = Arc::new(Mutex::new(Vec::new()));
        let wrong_subject = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&stopped);
        let observed = Arc::clone(&calls);
        let mismatch = Arc::clone(&wrong_subject);
        let worker = thread::spawn(move || {
            for accepted in listener.incoming() {
                let tcp = accepted
                    .unwrap_or_else(|error| panic!("identity accepted connection: {error}"));
                if stop.load(Ordering::Acquire) {
                    break;
                }
                let mut stream = StreamOwned::new(
                    ServerConnection::new(Arc::clone(&tls))
                        .unwrap_or_else(|error| panic!("identity TLS: {error}")),
                    tcp,
                );
                let (path, body) = read_request(&mut stream);
                let document: Value = serde_json::from_slice(&body)
                    .unwrap_or_else(|error| panic!("identity JSON: {error}"));
                observed
                    .lock()
                    .unwrap_or_else(|error| panic!("identity calls: {error}"))
                    .push((path.clone(), document.clone()));
                let response = match path.as_str() {
                    "/v1/principals" => {
                        let key = document["allowed_signer_public_keys"][0]
                            .as_str()
                            .unwrap_or_default();
                        let sub = document["sub"].as_str().unwrap_or_default();
                        let sub = if mismatch.load(Ordering::Acquire) {
                            format!("{TENANT}.wrong-subject")
                        } else {
                            sub.to_owned()
                        };
                        json!({"tenant": TENANT, "sub": sub, "allowed_signer_public_keys": [key], "account": null, "audiences": []})
                    }
                    "/v1/sessions" => {
                        json!({"session_id":"f".repeat(32),"tenant":TENANT,"sub":document["sub"],"token":"ses_test_token","csrf_token":"csrf_test_token","expires_at":9999999999_u64})
                    }
                    _ => json!({"error":"unexpected path"}),
                };
                write_response(
                    &mut stream,
                    if path == "/v1/principals" || path == "/v1/sessions" {
                        200
                    } else {
                        404
                    },
                    &response,
                );
            }
        });
        Self {
            address,
            stopped,
            calls,
            wrong_subject,
            worker: Some(worker),
        }
    }

    fn url(&self) -> String {
        format!("https://localhost:{}", self.address.port())
    }

    fn calls(&self) -> Vec<(String, Value)> {
        self.calls
            .lock()
            .unwrap_or_else(|error| panic!("identity calls: {error}"))
            .clone()
    }
}

impl Drop for IdentityPeer {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        if let Some(worker) = self.worker.take() {
            worker
                .join()
                .unwrap_or_else(|error| panic!("identity worker: {error:?}"));
        }
    }
}

struct GatewayPeer {
    address: SocketAddr,
    stopped: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl GatewayPeer {
    fn start(server: Server) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("gateway listener: {error}"));
        let address = listener
            .local_addr()
            .unwrap_or_else(|error| panic!("gateway address: {error}"));
        let stopped = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&stopped);
        let worker = thread::spawn(move || {
            for accepted in listener.incoming() {
                let tcp =
                    accepted.unwrap_or_else(|error| panic!("gateway accepted connection: {error}"));
                if stop.load(Ordering::Acquire) {
                    break;
                }
                server
                    .serve_connection(tcp)
                    .unwrap_or_else(|error| panic!("gateway HTTP: {error}"));
            }
        });
        Self {
            address,
            stopped,
            worker: Some(worker),
        }
    }

    fn call(&self, method: &str, params: Value) -> Value {
        let mut stream = TcpStream::connect(self.address)
            .unwrap_or_else(|error| panic!("gateway connect: {error}"));
        stream
            .set_read_timeout(Some(Duration::from_secs(10)))
            .unwrap_or_else(|error| panic!("gateway read timeout: {error}"));
        let body = json!({"jsonrpc":"2.0","id":1,"method":method,"params":params}).to_string();
        write!(stream, "POST / HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).unwrap_or_else(|error| panic!("gateway request: {error}"));
        stream
            .flush()
            .unwrap_or_else(|error| panic!("gateway flush: {error}"));
        let (_, response_body) = read_http(&mut stream);
        serde_json::from_slice(&response_body)
            .unwrap_or_else(|error| panic!("gateway response: {error}; body={:?}", response_body))
    }
}

impl Drop for GatewayPeer {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        if let Some(worker) = self.worker.take() {
            worker
                .join()
                .unwrap_or_else(|error| panic!("gateway worker: {error:?}"));
        }
    }
}

struct Environment(Vec<(String, Option<String>)>);

impl Environment {
    fn clear() -> Self {
        let previous = std::env::vars()
            .filter(|(key, _)| key.starts_with("LAYERX_GATEWAY_"))
            .map(|(key, value)| (key, Some(value)))
            .collect::<Vec<_>>();
        for (key, _) in &previous {
            std::env::remove_var(key);
        }
        Self(previous)
    }
}

impl Drop for Environment {
    fn drop(&mut self) {
        for (key, _) in std::env::vars().filter(|(key, _)| key.starts_with("LAYERX_GATEWAY_")) {
            std::env::remove_var(key);
        }
        for (key, value) in &self.0 {
            if let Some(value) = value {
                std::env::set_var(key, value);
            }
        }
    }
}

fn write_secret(root: &Path, name: &str, value: &str) {
    let path = root.join(name);
    fs::write(&path, value).unwrap_or_else(|error| panic!("secret: {error}"));
    fs::set_permissions(&path, fs::Permissions::from_mode(0o600))
        .unwrap_or_else(|error| panic!("secret permissions: {error}"));
    std::env::set_var(format!("LAYERX_GATEWAY_{name}_FILE"), path);
}

fn read_request(stream: &mut impl Read) -> (String, Vec<u8>) {
    let mut bytes = Vec::new();
    let mut chunk = [0; 1024];
    while !bytes.windows(4).any(|window| window == b"\r\n\r\n") {
        let count = stream
            .read(&mut chunk)
            .unwrap_or_else(|error| panic!("identity request: {error}"));
        if count == 0 {
            panic!("identity request ended");
        }
        bytes.extend_from_slice(&chunk[..count]);
    }
    let split = bytes
        .windows(4)
        .position(|window| window == b"\r\n\r\n")
        .unwrap_or_else(|| panic!("identity headers"));
    let headers = String::from_utf8_lossy(&bytes[..split]);
    let authorization = headers.lines().find_map(|line| {
        let (name, value) = line.split_once(':')?;
        name.eq_ignore_ascii_case("authorization")
            .then(|| value.trim())
    });
    assert!(
        authorization == Some(format!("Bearer {PROVISIONING_TOKEN}").as_str()),
        "identity requests require the provisioning credential"
    );
    let path = headers
        .lines()
        .next()
        .and_then(|line| line.split_whitespace().nth(1))
        .unwrap_or_default()
        .to_owned();
    let length = headers
        .lines()
        .find_map(|line| line.strip_prefix("Content-Length: "))
        .and_then(|value| value.parse::<usize>().ok())
        .unwrap_or(0);
    let mut body = bytes[split + 4..].to_vec();
    while body.len() < length {
        let count = stream
            .read(&mut chunk)
            .unwrap_or_else(|error| panic!("identity body: {error}"));
        if count == 0 {
            panic!("identity body ended");
        }
        body.extend_from_slice(&chunk[..count]);
    }
    body.truncate(length);
    (path, body)
}

fn write_response(stream: &mut impl Write, status: u16, body: &Value) {
    let body = body.to_string();
    write!(stream, "HTTP/1.1 {status} OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).unwrap_or_else(|error| panic!("identity response: {error}"));
    stream
        .flush()
        .unwrap_or_else(|error| panic!("identity response flush: {error}"));
}

fn read_http(stream: &mut impl Read) -> (u16, Vec<u8>) {
    let mut bytes = Vec::new();
    stream
        .read_to_end(&mut bytes)
        .unwrap_or_else(|error| panic!("gateway response: {error}"));
    let split = bytes
        .windows(4)
        .position(|window| window == b"\r\n\r\n")
        .unwrap_or_else(|| panic!("gateway response headers"));
    let status = String::from_utf8_lossy(&bytes[..split])
        .split_whitespace()
        .nth(1)
        .and_then(|value| value.parse().ok())
        .unwrap_or_else(|| panic!("gateway status"));
    (status, bytes[split + 4..].to_vec())
}

fn digest(domain: &[u8], tenant: &str, key: &[u8], time: Option<u64>) -> [u8; 32] {
    let mut hash = Sha256::new();
    let decimal = time.map(|time| time.to_string());
    let mut parts = vec![domain, tenant.as_bytes(), key];
    if let Some(decimal) = decimal.as_ref() {
        parts.push(decimal.as_bytes());
    }
    for part in parts {
        hash.update((part.len() as u64).to_be_bytes());
        hash.update(part);
    }
    hash.finalize().into()
}

fn subject(key: &SigningKey) -> String {
    format!(
        "{TENANT}.{}",
        &layerx_platform_gateway::encoding::hex_encode(&digest(
            b"layerx-register-subject-v1",
            TENANT,
            &key.verifying_key().to_bytes(),
            None
        ))[..32]
    )
}

fn session_params(key: &SigningKey, time: u64) -> Value {
    let public_key = key.verifying_key().to_bytes();
    json!([
        layerx_platform_gateway::encoding::hex_encode(&public_key),
        time.to_string(),
        layerx_platform_gateway::encoding::hex_encode(
            &key.sign(&digest(
                b"layerx-session-open-v1",
                TENANT,
                &public_key,
                Some(time)
            ))
            .to_bytes()
        )
    ])
}

fn tls_fixture() -> (Arc<ServerConfig>, Certificate) {
    let certificate = rcgen::generate_simple_self_signed(vec!["localhost".into()])
        .unwrap_or_else(|error| panic!("certificate: {error}"));
    let tls =
        ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
            .with_safe_default_protocol_versions()
            .unwrap_or_else(|error| panic!("TLS versions: {error}"))
            .with_no_client_auth()
            .with_single_cert(
                vec![certificate.cert.der().clone()],
                PrivateKeyDer::from(PrivatePkcs8KeyDer::from(
                    certificate.signing_key.serialize_der(),
                )),
            )
            .unwrap_or_else(|error| panic!("TLS certificate: {error}"));
    (
        Arc::new(tls),
        Certificate::from_der(certificate.cert.der()).unwrap_or_else(|error| panic!("CA: {error}")),
    )
}

fn configure(root: &Path, identity: &IdentityPeer, ca_path: &Path) {
    for (name, value) in [
        ("LISTENER", "plain"),
        ("NETWORK_ID", "session-open"),
        ("PROTOCOL_NETWORK_ID", "7"),
        ("REDIS_URL", "rediss://localhost:1"),
        ("TENANT", TENANT),
    ] {
        std::env::set_var(format!("LAYERX_GATEWAY_{name}"), value);
    }
    std::env::set_var(
        "LAYERX_GATEWAY_LXP_WIRE_VERSION",
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string(),
    );
    std::env::set_var("LAYERX_GATEWAY_LISTEN", "127.0.0.1:0");
    std::env::set_var("LAYERX_GATEWAY_OUTBOUND_CA_DER", ca_path);
    std::env::set_var("LAYERX_GATEWAY_IDENTITY_URL", identity.url());
    write_secret(root, "REDIS_USERNAME", "gateway");
    write_secret(root, "REDIS_PASSWORD", "session-open-password");
    write_secret(root, "IDENTITY_TOKEN", IDENTITY_TOKEN);
    write_secret(root, "IDENTITY_PROVISIONING_TOKEN", PROVISIONING_TOKEN);
}

#[test]
fn open_session_requires_a_fresh_registered_key_proof() {
    let _environment = Environment::clear();
    let root = tempfile::tempdir().unwrap_or_else(|error| panic!("temporary directory: {error}"));
    let (tls, ca) = tls_fixture();
    let ca_path = root.path().join("identity-ca.der");
    fs::write(
        &ca_path,
        ca.to_der()
            .unwrap_or_else(|error| panic!("CA bytes: {error}")),
    )
    .unwrap_or_else(|error| panic!("CA file: {error}"));
    let identity = IdentityPeer::start(tls);
    configure(root.path(), &identity, &ca_path);
    let server =
        Server::from_environment().unwrap_or_else(|error| panic!("gateway configuration: {error}"));
    let gateway = GatewayPeer::start(server);
    let key = SigningKey::from_bytes(&[7; 32]);
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_else(|error| panic!("clock: {error}"))
        .as_secs();

    let accepted = gateway.call("px_openSession", session_params(&key, now));
    assert_eq!(
        accepted["result"],
        json!({"tenant":TENANT,"sub":subject(&key),"token":"ses_test_token","csrf_token":"csrf_test_token","expires_at":9999999999_u64})
    );
    let calls = identity.calls();
    assert_eq!(
        calls
            .iter()
            .map(|(path, _)| path.as_str())
            .collect::<Vec<_>>(),
        vec!["/v1/principals", "/v1/sessions"]
    );
    assert_eq!(calls[0].1["tenant"], TENANT);
    assert_eq!(calls[0].1["sub"], subject(&key));
    assert_eq!(
        calls[0].1["allowed_signer_public_keys"][0],
        layerx_platform_gateway::encoding::hex_encode(&key.verifying_key().to_bytes())
    );
    assert_eq!(calls[1].1, json!({"tenant":TENANT,"sub":subject(&key)}));

    let replay = gateway.call("px_openSession", session_params(&key, now));
    assert_eq!(replay["error"]["data"]["code"], "session_proof_replay");
    assert_eq!(identity.calls().len(), 2);

    let registration_signature = key.sign(&digest(
        b"layerx-register-binding-v1",
        TENANT,
        &key.verifying_key().to_bytes(),
        None,
    ));
    let registration = json!([
        layerx_platform_gateway::encoding::hex_encode(&key.verifying_key().to_bytes()),
        now.to_string(),
        layerx_platform_gateway::encoding::hex_encode(&registration_signature.to_bytes())
    ]);
    assert_eq!(
        gateway.call("px_openSession", registration)["error"]["data"]["code"],
        "session_proof_invalid"
    );

    let stale = gateway.call(
        "px_openSession",
        session_params(&key, now.saturating_sub(121)),
    );
    assert_eq!(stale["error"]["data"]["code"], "session_time_out_of_range");

    identity.wrong_subject.store(true, Ordering::Release);
    let other_key = SigningKey::from_bytes(&[8; 32]);
    let mismatch = gateway.call("px_openSession", session_params(&other_key, now));
    assert_eq!(
        mismatch["error"]["data"]["code"],
        "registration_principal_mismatch"
    );
    assert_eq!(
        identity
            .calls()
            .iter()
            .filter(|(path, _)| path == "/v1/sessions")
            .count(),
        1
    );

    drop(gateway);
    std::env::remove_var("LAYERX_GATEWAY_IDENTITY_PROVISIONING_TOKEN_FILE");
    let without_token = GatewayPeer::start(
        Server::from_environment()
            .unwrap_or_else(|error| panic!("gateway without provisioning token: {error}")),
    );
    let refused = without_token.call(
        "px_openSession",
        session_params(&SigningKey::from_bytes(&[9; 32]), now),
    );
    assert_eq!(
        refused["error"]["data"]["code"],
        "registration_not_configured"
    );
    drop(without_token);
}
