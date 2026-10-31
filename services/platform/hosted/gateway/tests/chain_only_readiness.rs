use layerx_platform_gateway::http::{read_request, IncomingRequest};
use layerx_platform_internal::http;
use rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::{ServerConfig, ServerConnection, StreamOwned};
use serde_json::{json, Value};
use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

fn command(program: &str, arguments: &[&str]) {
    let output = Command::new(program)
        .args(arguments)
        .output()
        .unwrap_or_else(|error| panic!("{program} must run: {error}"));
    assert!(
        output.status.success(),
        "{program} failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
}

fn path(value: &Path) -> &str {
    value
        .to_str()
        .unwrap_or_else(|| panic!("test path must be UTF-8"))
}

fn free_port() -> u16 {
    TcpListener::bind("127.0.0.1:0")
        .and_then(|listener| listener.local_addr())
        .unwrap_or_else(|error| panic!("test port must be allocated: {error}"))
        .port()
}

struct Fixture {
    redis: Child,
    directory: PathBuf,
    redis_port: u16,
    tls: Arc<ServerConfig>,
}

impl Fixture {
    fn start() -> Self {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let directory = std::env::temp_dir().join(format!(
            "layerx-chain-only-readiness-{}-{}",
            std::process::id(),
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap_or_default()
                .as_nanos()
        ));
        fs::create_dir(&directory)
            .unwrap_or_else(|error| panic!("test directory must be created: {error}"));
        let pem = directory.join("server.pem");
        let der = directory.join("server.der");
        let key = directory.join("server.key");
        let key_der = directory.join("server-key.der");
        command(
            "openssl",
            &[
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-keyout",
                path(&key),
                "-out",
                path(&pem),
                "-days",
                "1",
                "-subj",
                "/CN=localhost",
                "-addext",
                "subjectAltName=DNS:localhost",
            ],
        );
        command(
            "openssl",
            &[
                "x509",
                "-in",
                path(&pem),
                "-outform",
                "DER",
                "-out",
                path(&der),
            ],
        );
        command(
            "openssl",
            &[
                "pkcs8",
                "-topk8",
                "-nocrypt",
                "-in",
                path(&key),
                "-outform",
                "DER",
                "-out",
                path(&key_der),
            ],
        );
        let redis_port = free_port();
        fs::write(
            directory.join("users.acl"),
            "user default off\nuser gateway on >gateway-secret ~* &* +@all\n",
        )
        .unwrap_or_else(|error| panic!("test Redis ACL must be written: {error}"));
        let config = directory.join("redis.conf");
        fs::write(
            &config,
            format!(
                "bind 127.0.0.1\nport 0\ntls-port {redis_port}\ntls-cert-file {}\ntls-key-file {}\ntls-ca-cert-file {}\ntls-auth-clients no\naclfile {}\nappendonly yes\nappendfsync always\ndir {}\nprotected-mode yes\n",
                path(&pem),
                path(&key),
                path(&pem),
                path(&directory.join("users.acl")),
                path(&directory),
            ),
        )
        .unwrap_or_else(|error| panic!("test Redis config must be written: {error}"));
        let redis = Command::new("redis-server")
            .arg(&config)
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .unwrap_or_else(|error| panic!("real Redis server must start: {error}"));
        let certificate = CertificateDer::from(
            fs::read(&der).unwrap_or_else(|error| panic!("certificate must be read: {error}")),
        );
        let private_key = PrivateKeyDer::from(PrivatePkcs8KeyDer::from(
            fs::read(&key_der).unwrap_or_else(|error| panic!("key must be read: {error}")),
        ));
        let tls = Arc::new(
            ServerConfig::builder()
                .with_no_client_auth()
                .with_single_cert(vec![certificate], private_key)
                .unwrap_or_else(|error| panic!("listener TLS must build: {error}")),
        );
        let fixture = Self {
            redis,
            directory,
            redis_port,
            tls,
        };
        for _ in 0..100 {
            if TcpStream::connect(("127.0.0.1", redis_port)).is_ok() {
                return fixture;
            }
            thread::sleep(Duration::from_millis(20));
        }
        panic!("real Redis server did not become reachable")
    }

    fn secret(&self, name: &str, value: &str) -> String {
        let file = self.directory.join(name);
        fs::write(&file, value).unwrap_or_else(|error| panic!("{name} must be written: {error}"));
        path(&file).to_owned()
    }

    fn environment(&self, listen: u16) -> Vec<(String, String)> {
        let ca = path(&self.directory.join("server.der")).to_owned();
        vec![
            (
                "LAYERX_GATEWAY_LISTEN".to_owned(),
                format!("127.0.0.1:{listen}"),
            ),
            ("LAYERX_GATEWAY_LISTENER".to_owned(), "plain".to_owned()),
            ("LAYERX_GATEWAY_OUTBOUND_CA_DER".to_owned(), ca),
            (
                "LAYERX_GATEWAY_NETWORK_ID".to_owned(),
                "router-surface".to_owned(),
            ),
            (
                "LAYERX_GATEWAY_LXP_WIRE_VERSION".to_owned(),
                layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string(),
            ),
            (
                "LAYERX_GATEWAY_PROTOCOL_NETWORK_ID".to_owned(),
                "7".to_owned(),
            ),
            (
                "LAYERX_GATEWAY_REDIS_URL".to_owned(),
                format!("rediss://localhost:{}", self.redis_port),
            ),
            (
                "LAYERX_GATEWAY_REDIS_USERNAME_FILE".to_owned(),
                self.secret("redis-username", "gateway"),
            ),
            (
                "LAYERX_GATEWAY_REDIS_PASSWORD_FILE".to_owned(),
                self.secret("redis-password", "gateway-secret"),
            ),
        ]
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = self.redis.kill();
        let _ = self.redis.wait();
        let _ = fs::remove_dir_all(&self.directory);
    }
}

fn gateway_command(environment: &[(String, String)]) -> Command {
    let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-gateway"));
    command
        .env_clear()
        .env("PATH", std::env::var_os("PATH").unwrap_or_default())
        .envs(environment.iter().map(|(name, value)| (name, value)));
    command
}

struct Listener {
    port: u16,
    stop: Arc<AtomicBool>,
    join: Option<JoinHandle<()>>,
}
impl Listener {
    fn start(
        tls: Arc<ServerConfig>,
        handler: impl Fn(&IncomingRequest) -> http::Response + Send + 'static,
    ) -> Self {
        Self::start_on(tls, 0, handler)
    }
    fn start_on(
        tls: Arc<ServerConfig>,
        port: u16,
        handler: impl Fn(&IncomingRequest) -> http::Response + Send + 'static,
    ) -> Self {
        let listener =
            TcpListener::bind(("127.0.0.1", port)).unwrap_or_else(|error| panic!("{error}"));
        let port = listener
            .local_addr()
            .unwrap_or_else(|error| panic!("{error}"))
            .port();
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("{error}"));
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let join = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((tcp, _)) => {
                        tcp.set_read_timeout(Some(Duration::from_secs(3)))
                            .unwrap_or_else(|error| panic!("{error}"));
                        tcp.set_write_timeout(Some(Duration::from_secs(3)))
                            .unwrap_or_else(|error| panic!("{error}"));
                        let connection = ServerConnection::new(Arc::clone(&tls))
                            .unwrap_or_else(|error| panic!("{error}"));
                        let mut stream = StreamOwned::new(connection, tcp);
                        let request = read_request(&mut stream, http::MAX_REQUEST_BYTES)
                            .unwrap_or_else(|error| panic!("listener request: {error}"));
                        {
                            http::write_response(&mut stream, &handler(&request))
                                .unwrap_or_else(|error| panic!("listener response: {error}"));
                            stream.conn.send_close_notify();
                            stream
                                .flush()
                                .unwrap_or_else(|error| panic!("TLS flush: {error}"));
                        }
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(5));
                    }
                    Err(error) => panic!("{error}"),
                }
            }
        });
        Self {
            port,
            stop,
            join: Some(join),
        }
    }
}
impl Drop for Listener {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(join) = self.join.take() {
            assert!(join.join().is_ok());
        }
    }
}

struct Gateway {
    child: Child,
    port: u16,
}

impl Drop for Gateway {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

impl Gateway {
    fn start(environment: &[(String, String)], port: u16) -> Self {
        let child = gateway_command(environment)
            .stdout(Stdio::null())
            .stderr(Stdio::inherit())
            .spawn()
            .expect("gateway must start");
        let mut gateway = Self { child, port };
        for _ in 0..200 {
            if let Some(status) = gateway.child.try_wait().expect("gateway process status") {
                panic!("gateway refused startup: {status}");
            }
            if gateway
                .exchange("GET", "/livez", b"")
                .is_ok_and(|(status, _)| status == 200)
            {
                return gateway;
            }
            thread::sleep(Duration::from_millis(50));
        }
        panic!("gateway did not become live");
    }

    fn readiness(&self, path: &str) -> (u16, Value) {
        let (status, body) = self.exchange("GET", path, b"").expect("readiness exchange");
        (
            status,
            serde_json::from_slice(&body).expect("readiness JSON"),
        )
    }
    fn exchange(&self, method: &str, path: &str, body: &[u8]) -> Result<(u16, Vec<u8>), String> {
        let mut stream =
            TcpStream::connect(("127.0.0.1", self.port)).map_err(|error| error.to_string())?;
        stream
            .set_read_timeout(Some(Duration::from_secs(30)))
            .map_err(|error| error.to_string())?;
        let mut request = format!(
            "{method} {path} HTTP/1.1\r\nHost: localhost\r\nAccept: application/json\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
            body.len()
        )
        .into_bytes();
        request.extend_from_slice(body);
        stream
            .write_all(&request)
            .map_err(|error| error.to_string())?;
        let mut answer = Vec::new();
        stream
            .read_to_end(&mut answer)
            .map_err(|error| error.to_string())?;
        let header_end = answer
            .windows(4)
            .position(|window| window == b"\r\n\r\n")
            .ok_or_else(|| "answer has no header terminator".to_owned())?;
        let head = String::from_utf8_lossy(&answer[..header_end]).into_owned();
        let status = head
            .split_whitespace()
            .nth(1)
            .and_then(|value| value.parse::<u16>().ok())
            .ok_or_else(|| format!("answer status is invalid: {head}"))?;
        Ok((status, answer[header_end + 4..].to_vec()))
    }
}

#[test]
fn chain_only_readiness_omits_kernel_backends() {
    let fixture = Fixture::start();
    let chain = Listener::start(Arc::clone(&fixture.tls), |request| {
        assert_eq!(request.method, "POST");
        let call: Value = serde_json::from_slice(&request.body).expect("chain request JSON");
        assert_eq!(call["method"], "eth_chainId");
        http::json(
            200,
            &json!({"jsonrpc": "2.0", "id": call["id"], "result": "0x7d"}),
        )
    });
    let consensus = Listener::start(Arc::clone(&fixture.tls), |request| {
        assert_eq!(request.method, "POST");
        let call: Value = serde_json::from_slice(&request.body).expect("consensus request JSON");
        assert_eq!(call["method"], "status");
        http::json(
            200,
            &json!({"jsonrpc": "2.0", "id": call["id"], "result": {"sync_info": {"catching_up": false}}}),
        )
    });
    let store = layerx_indexer::store::Store::open_in_memory().expect("real indexer database");
    let indexer = Listener::start(Arc::clone(&fixture.tls), move |request| {
        layerx_indexer::api::route(&store, &request.method, &request.path)
    });
    let port = free_port();
    let mut environment = fixture.environment(port);
    environment.extend([
        (
            "LAYERX_GATEWAY_PAXEER_RPC_URLS".to_owned(),
            json!([
                format!("https://localhost:{}/chain", chain.port),
                format!("https://localhost:{}/secondary", chain.port)
            ])
            .to_string(),
        ),
        (
            "LAYERX_GATEWAY_CONSENSUS_RPC_URL".to_owned(),
            format!("https://localhost:{}/consensus", consensus.port),
        ),
        (
            "LAYERX_GATEWAY_INDEXER_URL".to_owned(),
            format!("https://localhost:{}", indexer.port),
        ),
    ]);
    assert!(environment.iter().all(|(name, _)| matches!(
        name.as_str(),
        "LAYERX_GATEWAY_LISTEN"
            | "LAYERX_GATEWAY_LISTENER"
            | "LAYERX_GATEWAY_OUTBOUND_CA_DER"
            | "LAYERX_GATEWAY_NETWORK_ID"
            | "LAYERX_GATEWAY_LXP_WIRE_VERSION"
            | "LAYERX_GATEWAY_PROTOCOL_NETWORK_ID"
            | "LAYERX_GATEWAY_REDIS_URL"
            | "LAYERX_GATEWAY_REDIS_USERNAME_FILE"
            | "LAYERX_GATEWAY_REDIS_PASSWORD_FILE"
            | "LAYERX_GATEWAY_PAXEER_RPC_URLS"
            | "LAYERX_GATEWAY_CONSENSUS_RPC_URL"
            | "LAYERX_GATEWAY_INDEXER_URL"
    )));
    let gateway = Gateway::start(&environment, port);
    let chain_only = gateway.readiness("/readyz");
    let core_only = gateway.readiness("/readyz/core");
    drop(gateway);

    command(
        "openssl",
        &[
            "pkcs12",
            "-export",
            "-inkey",
            path(&fixture.directory.join("server.key")),
            "-in",
            path(&fixture.directory.join("server.pem")),
            "-out",
            path(&fixture.directory.join("client.p12")),
            "-passout",
            "pass:integration-only",
        ],
    );
    let port = free_port();
    environment.retain(|(name, _)| name != "LAYERX_GATEWAY_LISTEN");
    environment.push((
        "LAYERX_GATEWAY_LISTEN".to_owned(),
        format!("127.0.0.1:{port}"),
    ));
    for name in ["COMPONENT", "AUTHORITY", "IDENTITY", "PROGRAM_REGISTRY"] {
        environment.push((
            format!("LAYERX_GATEWAY_{name}_URL"),
            format!("https://localhost:{}", indexer.port),
        ));
        environment.push((
            format!("LAYERX_GATEWAY_{name}_TOKEN_FILE"),
            fixture.secret(&format!("{name}-token"), "integration-token"),
        ));
    }
    environment.extend([
        ("LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12".to_owned(), path(&fixture.directory.join("client.p12")).to_owned()),
        ("LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE".to_owned(), fixture.secret("identity-password", "integration-only")),
        ("LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE".to_owned(), fixture.secret("provisioning-key", &"5a".repeat(32))),
        ("LAYERX_GATEWAY_MODULE_REGISTRY_FILE".to_owned(), fixture.secret("modules.json", &json!({
            "schema_version": 2,
            "assets": [{"asset": "02".repeat(32), "currency": "USD", "decimals": 6, "symbol": "$"}],
            "modules": [{"module": 9, "ordinals": [1, 2, 7]}]
        }).to_string())),
    ]);
    let gateway = Gateway::start(&environment, port);
    let (configured_status, configured) = gateway.readiness("/readyz");
    let (configured_core_status, configured_core) = gateway.readiness("/readyz/core");
    let kernel_backends = [
        "core_agent_boundary",
        "public_core",
        "independent_receipt_authority",
        "identity",
        "program_registry",
    ];
    for backend in kernel_backends {
        assert!(
            configured["backends"].get(backend).is_some(),
            "{backend}: {configured}"
        );
        assert!(
            configured["components"].get(backend).is_some(),
            "{backend}: {configured}"
        );
        assert!(
            configured_core["backends"].get(backend).is_some(),
            "{backend}: {configured_core}"
        );
    }
    assert_eq!(configured_status, 503, "{configured}");
    assert_eq!(configured_core_status, 503, "{configured_core}");
    assert_eq!(configured["status"], "degraded");
    assert_eq!(configured["scope"], "product");
    assert_eq!(configured_core["scope"], "core");
    assert_eq!(
        configured["backends"]["event_producer"],
        json!({"state": "unavailable", "reason": "not_configured"})
    );
    assert_eq!(
        configured["components"]["principal_state_boundary"],
        "unavailable"
    );
    assert_eq!(
        configured["backends"]["public_core"]["reason"],
        "not_configured"
    );
    for (status, document) in [chain_only, core_only] {
        assert!(document["backends"].get("event_producer").is_none());
        assert!(document["backends"]
            .as_object()
            .expect("backend object")
            .values()
            .all(|backend| backend["state"] == "ready"));
        for backend in kernel_backends {
            assert!(
                document["backends"].get(backend).is_none(),
                "{backend}: {document}"
            );
            assert!(
                document["components"].get(backend).is_none(),
                "{backend}: {document}"
            );
        }
        assert!(
            document["components"]
                .get("principal_state_boundary")
                .is_none(),
            "{document}"
        );
        assert_eq!(
            document["capabilities"]["layerx_activity"]["reason"],
            "not_configured"
        );
        assert_eq!(document["capabilities"]["chain_serving"]["state"], "ready");
        assert_eq!(status, 200, "{document}");
        assert_eq!(document["status"], "ready", "{document}");
    }
}
