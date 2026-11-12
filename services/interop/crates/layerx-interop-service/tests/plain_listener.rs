#[path = "support/active_inputs.rs"]
pub mod active_inputs;

use layerx_platform_gateway::http;
use serde_json::{json, Value};
use std::fs;
use std::net::{SocketAddr, TcpListener};
use std::os::unix::fs::PermissionsExt as _;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Output, Stdio};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::thread;
use std::time::Duration;

static FIXTURES: AtomicUsize = AtomicUsize::new(0);

struct Fixture {
    dir: PathBuf,
    port: u16,
    sequencer: std::net::SocketAddr,
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.dir);
    }
}

struct Gateway(Child);

impl Drop for Gateway {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

fn run(program: &str, args: &[&str]) -> Output {
    let output = Command::new(program)
        .args(args)
        .output()
        .unwrap_or_else(|error| panic!("{program} failed to start: {error}"));
    assert!(
        output.status.success(),
        "{program} {args:?} failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    output
}

fn path(dir: &Path, name: &str) -> String {
    dir.join(name).to_str().or_panic("utf-8 path").to_owned()
}

fn prepare_certificates(dir: &Path) {
    run(
        "openssl",
        &[
            "req",
            "-x509",
            "-newkey",
            "ec",
            "-pkeyopt",
            "ec_paramgen_curve:prime256v1",
            "-nodes",
            "-days",
            "1",
            "-subj",
            "/CN=localhost",
            "-addext",
            "subjectAltName=DNS:localhost,IP:127.0.0.1",
            "-keyout",
            &path(dir, "key.pem"),
            "-out",
            &path(dir, "cert.pem"),
        ],
    );
    run(
        "openssl",
        &[
            "x509",
            "-in",
            &path(dir, "cert.pem"),
            "-outform",
            "DER",
            "-out",
            &path(dir, "cert.der"),
        ],
    );
    run(
        "openssl",
        &[
            "pkcs8",
            "-topk8",
            "-nocrypt",
            "-in",
            &path(dir, "key.pem"),
            "-outform",
            "DER",
            "-out",
            &path(dir, "key.der"),
        ],
    );
    run(
        "openssl",
        &[
            "pkcs12",
            "-export",
            "-in",
            &path(dir, "cert.pem"),
            "-inkey",
            &path(dir, "key.pem"),
            "-passout",
            "pass:interop-client-identity",
            "-out",
            &path(dir, "client.p12"),
        ],
    );
}

fn spawn_sequencer(dir: &Path, receipt_keys: Value) -> SocketAddr {
    let sequencer = TcpListener::bind("127.0.0.1:0").or_panic("test value");
    sequencer.set_nonblocking(true).or_panic("test value");
    let sequencer_address = sequencer.local_addr().or_panic("test value");
    let lifetime = dir.to_path_buf();
    thread::spawn(move || {
        while lifetime.exists() {
            let (mut stream, _) = match sequencer.accept() {
                Ok(connection) => connection,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    thread::sleep(Duration::from_millis(10));
                    continue;
                }
                Err(error) => panic!("sequencer listener: {error}"),
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .or_panic("test value");
            let request = http::read_request(&mut stream, 64 * 1024).or_panic("test value");
            let rpc: Value = serde_json::from_slice(&request.body).or_panic("test value");
            assert_eq!(request.method, "POST");
            assert_eq!(rpc["method"], "px_getReceiptKeys");
            assert_eq!(rpc["params"], json!([]));
            let keys = json!({"keys": receipt_keys});
            layerx_client::sequencer::decode_receipt_keys(&keys)
                .or_panic("sequencer publishes the canonical receipt-key response");
            http::write_response(
                &mut stream,
                &http::OutgoingResponse {
                    status: 200,
                    content_type: "application/json".to_owned(),
                    headers: Vec::new(),
                    body: serde_json::to_vec(
                        &json!({"jsonrpc": "2.0", "id": rpc["id"], "result": keys}),
                    )
                    .or_panic("test value"),
                    retry_after: None,
                },
            )
            .or_panic("test value");
        }
    });
    sequencer_address
}

fn fixture() -> Fixture {
    let dir = std::env::temp_dir().join(format!(
        "layerx-interop-plain-listener-{}-{}",
        std::process::id(),
        FIXTURES.fetch_add(1, Ordering::Relaxed)
    ));
    let _ = fs::remove_dir_all(&dir);
    fs::create_dir_all(&dir).or_panic("fixture directory");
    prepare_certificates(&dir);
    for (name, contents) in [
        ("identity-password", "interop-client-identity\n"),
        ("redis-username", "interop\n"),
        ("redis-password", "interop-redis-password\n"),
        (
            "registry.json",
            "{\"modules\":[{\"module\":1,\"ordinals\":[1]}]}",
        ),
    ] {
        fs::write(dir.join(name), contents).or_panic("fixture file");
    }
    let public_key: String = hex(&ed25519_dalek::SigningKey::from_bytes(&[7; 32])
        .verifying_key()
        .to_bytes());
    let receipt_keys = json!([{
        "keyId": format!("0x{}", "11".repeat(32)), "publicKey": format!("0x{public_key}"),
        "firstHeight": 1, "lastHeight": 100,
    }]);
    fs::write(
        dir.join("receipt-keys.json"),
        serde_json::to_vec(&receipt_keys).or_panic("test value"),
    )
    .or_panic("test value");
    let mut manifest: Value = serde_json::from_slice(include_bytes!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../deploy/gateway/config.example.json"
    )))
    .or_panic("test value");
    manifest["x402_supported"] = json!({
        "kinds": [{"x402Version": 2, "scheme": "exact", "network": "eip155:125"}],
        "extensions": [], "signers": {"eip155:125": [format!("did:layerx:{public_key}")]}
    });
    let ap2_key = p256::ecdsa::SigningKey::from_bytes((&[9; 32]).into()).or_panic("test value");
    let ap2_public_key: String = hex(ap2_key.verifying_key().to_encoded_point(false).as_bytes());
    for pin in manifest["ap2_keys"].as_array_mut().or_panic("test value") {
        pin["public_key_sec1"] = json!(ap2_public_key);
    }
    manifest["visa_agents"][0]["public_key"] = json!(public_key);
    manifest["fiat_providers"][0]["public_key_ed25519"] = json!(public_key);
    fs::write(
        dir.join("config.json"),
        serde_json::to_vec(&manifest).or_panic("test value"),
    )
    .or_panic("test value");
    for entry in fs::read_dir(&dir).or_panic("test value") {
        fs::set_permissions(
            entry.or_panic("test value").path(),
            fs::Permissions::from_mode(0o600),
        )
        .or_panic("test value");
    }
    fs::set_permissions(&dir, fs::Permissions::from_mode(0o700)).or_panic("test value");
    let sequencer_address = spawn_sequencer(&dir, receipt_keys);
    let port = TcpListener::bind("127.0.0.1:0")
        .or_panic("free port")
        .local_addr()
        .or_panic("local address")
        .port();
    Fixture {
        dir,
        port,
        sequencer: sequencer_address,
    }
}

fn gateway(fixture: &Fixture, extra: &[(&str, String)]) -> Command {
    let dir = &fixture.dir;
    let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-interop-gateway"));
    command
        .env_clear()
        .env("LAYERX_INTEROP_CONFIG", path(dir, "config.json"))
        .env(
            "LAYERX_INTEROP_LISTEN",
            format!("127.0.0.1:{}", fixture.port),
        )
        .env("LAYERX_INTEROP_OUTBOUND_CA_DER", path(dir, "cert.der"))
        .env(
            "LAYERX_INTEROP_CLIENT_IDENTITY_PKCS12",
            path(dir, "client.p12"),
        )
        .env(
            "LAYERX_INTEROP_CLIENT_IDENTITY_PASSWORD_FILE",
            path(dir, "identity-password"),
        )
        .env("LAYERX_INTEROP_REDIS_URL", "rediss://localhost:1")
        .env(
            "LAYERX_INTEROP_REDIS_USERNAME_FILE",
            path(dir, "redis-username"),
        )
        .env(
            "LAYERX_INTEROP_REDIS_PASSWORD_FILE",
            path(dir, "redis-password"),
        )
        .env(
            "LAYERX_INTEROP_SEQUENCER_URL",
            format!("http://{}", fixture.sequencer),
        )
        .env(
            "LAYERX_INTEROP_RECEIPT_KEYS_FILE",
            path(dir, "receipt-keys.json"),
        )
        .env("LAYERX_INTEROP_CHAIN_ID", "125")
        .env("LAYERX_INTEROP_READINESS_MAX_AGE_MS", "30000")
        .env("LAYERX_INTEROP_NETWORK_ID", "eip155-125")
        .env("LAYERX_INTEROP_WIRE_VERSION", "3")
        .env("LAYERX_INTEROP_PROTOCOL_NETWORK_ID", "402")
        .env(
            "LAYERX_INTEROP_MODULE_REGISTRY_FILE",
            path(dir, "registry.json"),
        )
        .env("LAYERX_INTEROP_TAP_CLOCK_SKEW_SECONDS", "60");
    for (name, value) in extra {
        command.env(name, value);
    }
    active_inputs::assert_command(&command, dir);
    command
}

fn start(fixture: &Fixture, extra: &[(&str, String)]) -> Gateway {
    let child = gateway(fixture, extra)
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .spawn()
        .or_panic("interop gateway starts");
    Gateway(child)
}

fn livez(gateway: &mut Gateway, curl: &[String]) -> String {
    for _ in 0..200 {
        if let Some(status) = gateway.0.try_wait().or_panic("child status") {
            let output = gateway
                .0
                .stderr
                .take()
                .map_or_else(String::new, |mut stderr| {
                    let mut text = String::new();
                    let _ = std::io::Read::read_to_string(&mut stderr, &mut text);
                    text
                });
            panic!("interop gateway exited with {status}: {output}");
        }
        let output = Command::new("curl")
            .args(curl)
            .output()
            .or_panic("curl runs");
        let text = String::from_utf8_lossy(&output.stdout).into_owned();
        if output.status.success() && text.ends_with("200") {
            return text;
        }
        thread::sleep(Duration::from_millis(50));
    }
    panic!("interop gateway never answered /livez");
}

fn listener_tools_available() -> bool {
    for tool in ["openssl", "curl"] {
        match Command::new(tool).arg("--version").output() {
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                eprintln!("SKIP listener test: missing prerequisite {tool}");
                return false;
            }
            Err(error) => panic!("{tool} prerequisite check failed: {error}"),
            Ok(_) => {}
        }
    }
    true
}

#[test]
fn plain_listener_answers_livez_over_http() {
    if !listener_tools_available() {
        return;
    }
    let fixture = fixture();
    let mut gateway = start(&fixture, &[("LAYERX_INTEROP_LISTENER", "plain".to_owned())]);
    let body = livez(
        &mut gateway,
        &[
            "-s".to_owned(),
            "-o".to_owned(),
            "-".to_owned(),
            "-w".to_owned(),
            "%{http_code}".to_owned(),
            format!("http://127.0.0.1:{}/livez", fixture.port),
        ],
    );
    assert!(body.contains("\"status\":\"live\""), "{body}");
    assert!(body.contains("layerx-interop-gateway"), "{body}");
}

#[test]
fn default_listener_answers_livez_over_tls() {
    if !listener_tools_available() {
        return;
    }
    let fixture = fixture();
    let dir = &fixture.dir;
    let mut gateway = start(
        &fixture,
        &[
            ("LAYERX_INTEROP_TLS_CERT_DER", path(dir, "cert.der")),
            ("LAYERX_INTEROP_TLS_KEY_DER", path(dir, "key.der")),
        ],
    );
    let body = livez(
        &mut gateway,
        &[
            "-s".to_owned(),
            "--cacert".to_owned(),
            path(dir, "cert.pem"),
            "--resolve".to_owned(),
            format!("localhost:{}:127.0.0.1", fixture.port),
            "-o".to_owned(),
            "-".to_owned(),
            "-w".to_owned(),
            "%{http_code}".to_owned(),
            format!("https://localhost:{}/livez", fixture.port),
        ],
    );
    assert!(body.contains("\"status\":\"live\""), "{body}");
    let plain = Command::new("curl")
        .args([
            "-s",
            "-o",
            "/dev/null",
            "-w",
            "%{http_code}",
            &format!("http://127.0.0.1:{}/livez", fixture.port),
        ])
        .output()
        .or_panic("curl runs");
    assert_ne!(String::from_utf8_lossy(&plain.stdout), "200");
}

fn refused(fixture: &Fixture, extra: &[(&str, String)]) -> String {
    let output = gateway(fixture, extra)
        .output()
        .or_panic("interop gateway runs");
    assert!(!output.status.success(), "gateway accepted {extra:?}");
    String::from_utf8_lossy(&output.stderr).into_owned()
}

#[test]
fn plain_listener_refuses_tls_material_and_unknown_modes() {
    if !listener_tools_available() {
        return;
    }
    let fixture = fixture();
    let dir = &fixture.dir;
    let plain = || ("LAYERX_INTEROP_LISTENER", "plain".to_owned());
    let stderr = refused(
        &fixture,
        &[
            plain(),
            ("LAYERX_INTEROP_TLS_CERT_DER", path(dir, "cert.der")),
        ],
    );
    assert!(
        stderr.contains("LAYERX_INTEROP_TLS_CERT_DER is set with LAYERX_INTEROP_LISTENER plain"),
        "{stderr}"
    );
    let stderr = refused(
        &fixture,
        &[
            plain(),
            ("LAYERX_INTEROP_TLS_KEY_DER", path(dir, "key.der")),
        ],
    );
    assert!(
        stderr.contains("LAYERX_INTEROP_TLS_KEY_DER is set with LAYERX_INTEROP_LISTENER plain"),
        "{stderr}"
    );
    let stderr = refused(&fixture, &[("LAYERX_INTEROP_LISTENER", "http".to_owned())]);
    assert!(
        stderr.contains("LAYERX_INTEROP_LISTENER must be tls or plain"),
        "{stderr}"
    );
}

trait OrPanic {
    type Output;

    fn or_panic(self, context: &str) -> Self::Output;
}

impl<T, E: std::fmt::Debug> OrPanic for std::result::Result<T, E> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}

impl<T> OrPanic for std::option::Option<T> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|| panic!("{context}"))
    }
}

fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    let mut encoded = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        write!(&mut encoded, "{byte:02x}")
            .unwrap_or_else(|error| panic!("hex encoding: {error:?}"));
    }
    encoded
}
