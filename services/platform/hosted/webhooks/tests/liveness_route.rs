use serde_json::Value;
use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::thread::{self, JoinHandle};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

const BINARY: &str = env!("CARGO_BIN_EXE_layerx-webhooks");

struct Material {
    directory: PathBuf,
}

impl Drop for Material {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.directory);
    }
}

struct Service {
    child: Child,
}

impl Drop for Service {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

struct Redis {
    child: Child,
}

impl Drop for Redis {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

struct Kms {
    address: String,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl Drop for Kms {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let _ = TcpStream::connect(&self.address);
        if let Some(worker) = self.worker.take() {
            worker
                .join()
                .unwrap_or_else(|_| panic!("KMS server must stop"));
        }
    }
}

fn path(value: &Path) -> String {
    value
        .to_str()
        .unwrap_or_else(|| panic!("test path must be UTF-8"))
        .to_owned()
}

fn run(program: &str, arguments: &[&str]) {
    let output = Command::new(program)
        .args(arguments)
        .output()
        .unwrap_or_else(|error| panic!("{program} must run: {error}"));
    assert!(
        output.status.success(),
        "{program} {arguments:?} failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
}

fn free_port() -> u16 {
    TcpListener::bind("127.0.0.1:0")
        .and_then(|listener| listener.local_addr())
        .map_or_else(
            |error| panic!("test port must be allocated: {error}"),
            |address| address.port(),
        )
}

fn material(label: &str) -> Material {
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_or(0, |duration| duration.as_nanos());
    let directory = std::env::temp_dir().join(format!(
        "layerx-webhooks-liveness-{label}-{}-{nanos}",
        std::process::id()
    ));
    fs::create_dir_all(&directory)
        .unwrap_or_else(|error| panic!("test directory must be created: {error}"));
    let key = path(&directory.join("server-key.pem"));
    let certificate = path(&directory.join("server.pem"));
    run(
        "openssl",
        &[
            "req",
            "-x509",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-keyout",
            &key,
            "-out",
            &certificate,
            "-days",
            "1",
            "-subj",
            "/CN=localhost",
            "-addext",
            "subjectAltName=DNS:localhost,IP:127.0.0.1",
        ],
    );
    run(
        "openssl",
        &[
            "x509",
            "-in",
            &certificate,
            "-outform",
            "DER",
            "-out",
            &path(&directory.join("server.der")),
        ],
    );
    run(
        "openssl",
        &[
            "pkcs8",
            "-topk8",
            "-nocrypt",
            "-in",
            &key,
            "-outform",
            "DER",
            "-out",
            &path(&directory.join("server-key.der")),
        ],
    );
    run(
        "openssl",
        &[
            "pkcs12",
            "-export",
            "-inkey",
            &key,
            "-in",
            &certificate,
            "-out",
            &path(&directory.join("client.p12")),
            "-passout",
            "pass:integration-only",
        ],
    );
    Material { directory }
}

fn secret(directory: &Path, name: &str, value: &str) -> String {
    let file = directory.join(name);
    fs::write(&file, value).unwrap_or_else(|error| panic!("{name} must be written: {error}"));
    path(&file)
}

fn environment(material: &Material, listen: u16, redis: u16, kms: u16) -> Vec<(String, String)> {
    let directory = &material.directory;
    let mut environment = vec![
        (
            "LAYERX_WEBHOOKS_LISTEN".to_owned(),
            format!("127.0.0.1:{listen}"),
        ),
        (
            "LAYERX_WEBHOOKS_TLS_CERT_DER".to_owned(),
            path(&directory.join("server.der")),
        ),
        (
            "LAYERX_WEBHOOKS_TLS_KEY_DER".to_owned(),
            path(&directory.join("server-key.der")),
        ),
        (
            "LAYERX_WEBHOOKS_INTERNAL_CA_DER".to_owned(),
            path(&directory.join("server.der")),
        ),
        (
            "LAYERX_WEBHOOKS_PUBLIC_CA_DER".to_owned(),
            path(&directory.join("server.der")),
        ),
        (
            "LAYERX_WEBHOOKS_CLIENT_IDENTITY_PKCS12".to_owned(),
            path(&directory.join("client.p12")),
        ),
        (
            "LAYERX_WEBHOOKS_CLIENT_IDENTITY_PASSWORD_FILE".to_owned(),
            secret(directory, "identity-password", "integration-only"),
        ),
        (
            "LAYERX_WEBHOOKS_REDIS_URL".to_owned(),
            format!("rediss://localhost:{redis}"),
        ),
        (
            "LAYERX_WEBHOOKS_REDIS_USERNAME_FILE".to_owned(),
            secret(directory, "redis-username", "webhooks"),
        ),
        (
            "LAYERX_WEBHOOKS_REDIS_PASSWORD_FILE".to_owned(),
            secret(directory, "redis-password", "webhooks-secret"),
        ),
        (
            "LAYERX_WEBHOOKS_CURSOR_KEY_FILE".to_owned(),
            secret(directory, "cursor-key", &"11".repeat(32)),
        ),
        (
            "LAYERX_WEBHOOKS_KMS_URL".to_owned(),
            format!("https://localhost:{kms}"),
        ),
        (
            "LAYERX_WEBHOOKS_KMS_TOKEN_FILE".to_owned(),
            secret(directory, "kms-token", "kms-token"),
        ),
        (
            "LAYERX_WEBHOOKS_INSTANCE_ID".to_owned(),
            "webhooks-liveness".to_owned(),
        ),
        (
            "LAYERX_WEBHOOKS_IDENTITY_URL".to_owned(),
            format!("https://localhost:{kms}"),
        ),
        (
            "LAYERX_WEBHOOKS_IDENTITY_TOKEN_FILE".to_owned(),
            secret(directory, "identity-token", "identity-token"),
        ),
        ("LAYERX_WEBHOOKS_ROLE".to_owned(), "public".to_owned()),
        ("LAYERX_WEBHOOKS_LISTENER".to_owned(), "plain".to_owned()),
    ];
    for stem in ["JOURNEY", "PAYMENT", "APPROVAL", "PROGRAM"] {
        environment.push((
            format!("LAYERX_WEBHOOKS_{stem}_SOURCE_URL"),
            format!("https://localhost:{kms}"),
        ));
        environment.push((
            format!("LAYERX_WEBHOOKS_{stem}_SOURCE_TOKEN_FILE"),
            secret(directory, &format!("{stem}-token"), "source-token"),
        ));
    }
    environment
}

fn start(environment: &[(String, String)]) -> Service {
    let mut command = Command::new(BINARY);
    command
        .env_clear()
        .envs(environment.iter().map(|(name, value)| (name, value)))
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::inherit());
    Service {
        child: command
            .spawn()
            .unwrap_or_else(|error| panic!("layerx-webhooks must start: {error}")),
    }
}

fn start_redis(material: &Material, port: u16) -> Redis {
    let directory = &material.directory;
    let config = format!(
        "bind 127.0.0.1\nport 0\ntls-port {port}\ntls-cert-file {}\ntls-key-file {}\ntls-ca-cert-file {}\ntls-auth-clients no\nuser default off\nuser webhooks on >webhooks-secret ~webhooks:* +@all\nsave \"\"\nappendonly no\ndir {}\n",
        path(&directory.join("server.pem")),
        path(&directory.join("server-key.pem")),
        path(&directory.join("server.pem")),
        path(directory),
    );
    let config_path = directory.join("redis.conf");
    fs::write(&config_path, config).unwrap_or_else(|error| panic!("redis config: {error}"));
    let mut child = Command::new("redis-server")
        .arg(config_path)
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .unwrap_or_else(|error| panic!("redis-server must start: {error}"));
    for _ in 0..100 {
        if let Ok(Some(status)) = child.try_wait() {
            panic!("redis-server exited before listening: {status}");
        }
        if TcpStream::connect(("127.0.0.1", port)).is_ok() {
            return Redis { child };
        }
        thread::sleep(Duration::from_millis(10));
    }
    panic!("redis-server never listened on {port}");
}

fn start_kms(material: &Material) -> Kms {
    let listener = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("KMS listener must bind: {error}"));
    listener
        .set_nonblocking(true)
        .unwrap_or_else(|error| panic!("KMS listener must be nonblocking: {error}"));
    let address = format!("127.0.0.1:{}", listener.local_addr().unwrap().port());
    let identity = native_tls::Identity::from_pkcs8(
        &fs::read(material.directory.join("server.pem")).unwrap(),
        &fs::read(material.directory.join("server-key.pem")).unwrap(),
    )
    .unwrap_or_else(|error| panic!("KMS identity: {error}"));
    let acceptor = native_tls::TlsAcceptor::new(identity)
        .unwrap_or_else(|error| panic!("KMS TLS acceptor: {error}"));
    let stop = Arc::new(AtomicBool::new(false));
    let stopped = Arc::clone(&stop);
    let worker = thread::spawn(move || {
        while !stopped.load(Ordering::Acquire) {
            let Ok((socket, _)) = listener.accept() else {
                thread::sleep(Duration::from_millis(2));
                continue;
            };
            let Ok(mut stream) = acceptor.accept(socket) else {
                continue;
            };
            let mut request = Vec::new();
            let mut chunk = [0_u8; 1024];
            while !request.windows(4).any(|window| window == b"\r\n\r\n") {
                let Ok(count) = stream.read(&mut chunk) else {
                    break;
                };
                if count == 0 {
                    break;
                }
                request.extend_from_slice(&chunk[..count]);
                if request.len() > 16 * 1024 {
                    break;
                }
            }
            let body = br#"{"ready":true,"ed25519_non_exportable":true}"#;
            let response = format!(
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                body.len()
            );
            let _ = stream.write_all(response.as_bytes());
            let _ = stream.write_all(body);
            let _ = stream.flush();
        }
    });
    Kms {
        address,
        stop,
        worker: Some(worker),
    }
}

fn request(port: u16, method: &str, path: &str) -> Option<(u16, Value)> {
    let output = Command::new("curl")
        .args([
            "-sS",
            "--max-time",
            "5",
            "-X",
            method,
            "-w",
            "\n%{http_code}",
            &format!("http://127.0.0.1:{port}{path}"),
        ])
        .output()
        .unwrap_or_else(|error| panic!("curl must run: {error}"));
    if !output.status.success() {
        return None;
    }
    let answer = String::from_utf8(output.stdout).unwrap();
    let (body, status) = answer.rsplit_once('\n').unwrap();
    Some((status.parse().unwrap(), serde_json::from_str(body).unwrap()))
}

fn wait_for_request(service: &mut Service, port: u16, method: &str, path: &str) -> (u16, Value) {
    for _ in 0..100 {
        if let Ok(Some(status)) = service.child.try_wait() {
            panic!("layerx-webhooks exited before answering: {status}");
        }
        if let Some(response) = request(port, method, path) {
            return response;
        }
        thread::sleep(Duration::from_millis(100));
    }
    panic!("layerx-webhooks never answered {method} {path}");
}

fn assert_live(document: &Value, role: &str) {
    let object = document.as_object().unwrap();
    assert_eq!(object.len(), 2, "liveness must not expose readiness fields");
    assert_eq!(document["live"], true);
    assert_eq!(document["role"], role);
    assert!(!object.contains_key("ready"));
}

#[test]
fn liveness_route_is_independent_of_unready_dependencies() {
    let material = material("unready");
    let port = free_port();
    let health_port = free_port();
    let mut environment = environment(&material, port, free_port(), free_port());
    environment.retain(|(name, _)| {
        !matches!(
            name.as_str(),
            "LAYERX_WEBHOOKS_TLS_CERT_DER" | "LAYERX_WEBHOOKS_TLS_KEY_DER"
        )
    });
    environment.push((
        "LAYERX_WEBHOOKS_HEALTH_LISTEN".to_owned(),
        format!("127.0.0.1:{health_port}"),
    ));
    environment.push((
        "LAYERX_WEBHOOKS_NETWORK_ID".to_owned(),
        "paxeer-liveness".to_owned(),
    ));
    let mut service = start(&environment);
    let (status, document) = wait_for_request(&mut service, port, "GET", "/livez");
    assert_eq!(status, 200);
    assert_live(&document, "public");
    let (status, document) = wait_for_request(&mut service, health_port, "GET", "/livez");
    assert_eq!(status, 200);
    assert_live(&document, "public");
    let (status, document) = wait_for_request(&mut service, port, "GET", "/healthz");
    assert_eq!(status, 503);
    assert_eq!(document["ready"], false);
    assert_eq!(document["version"], env!("CARGO_PKG_VERSION"));
    assert_eq!(document["network_id"], "paxeer-liveness");
    let (status, document) = wait_for_request(&mut service, port, "POST", "/livez");
    assert_eq!(
        status, 404,
        "POST /livez must be an unknown route: {document}"
    );
    let (status, document) = wait_for_request(&mut service, health_port, "POST", "/livez");
    assert_eq!(
        status, 404,
        "POST /livez must be an unknown route: {document}"
    );
}

#[test]
fn liveness_route_preserves_ready_health() {
    let material = material("ready");
    let redis_port = free_port();
    let redis = start_redis(&material, redis_port);
    let kms = start_kms(&material);
    let port = free_port();
    let mut environment = environment(
        &material,
        port,
        redis_port,
        kms.address.rsplit_once(':').unwrap().1.parse().unwrap(),
    );
    environment.retain(|(name, _)| {
        !matches!(
            name.as_str(),
            "LAYERX_WEBHOOKS_TLS_CERT_DER" | "LAYERX_WEBHOOKS_TLS_KEY_DER"
        )
    });
    for network in [None, Some("paxeer-liveness")] {
        environment.retain(|(name, _)| name != "LAYERX_WEBHOOKS_NETWORK_ID");
        if let Some(network) = network {
            environment.push(("LAYERX_WEBHOOKS_NETWORK_ID".to_owned(), network.to_owned()));
        }
        let mut service = start(&environment);
        let (status, document) = wait_for_request(&mut service, port, "GET", "/healthz");
        assert_eq!(status, 200, "ready process must answer health: {document}");
        assert_eq!(document["ready"], true);
        assert_eq!(document["version"], env!("CARGO_PKG_VERSION"));
        match network {
            Some(network) => assert_eq!(document["network_id"], network),
            None => assert!(document.get("network_id").is_none(), "{document}"),
        }
        let (status, document) = wait_for_request(&mut service, port, "GET", "/livez");
        assert_eq!(status, 200);
        assert_live(&document, "public");
        drop(service);
    }
    drop(kms);
    drop(redis);
}
