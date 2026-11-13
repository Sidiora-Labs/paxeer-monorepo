use std::fs;
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_platform_internal::tls::{Origin, Upstream};
use layerx_platform_internal::{http, kms, seal::SealKey, secret};
use native_tls::{Certificate, Identity};
use rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::{RootCertStore, ServerConfig, ServerConnection, StreamOwned};
use zeroize::Zeroizing;

const FIXTURE_NETWORK_ID: u64 = 77;
const FIXTURE_CHAIN_ID: &str = "producer-integration";

pub struct Tls {
    pub config: Arc<ServerConfig>,
    pub ca: Certificate,
    pub identity: Identity,
}

impl Tls {
    pub fn new(root: &Path) -> Self {
        let _ = rustls::crypto::ring::default_provider().install_default();
        fs::create_dir_all(root).unwrap_or_else(|error| panic!("{error}"));
        issue_certificates(root);
        let der = fs::read(root.join("cert.der")).unwrap_or_else(|error| panic!("{error}"));
        let ca = Certificate::from_der(&der).unwrap_or_else(|error| panic!("{error}"));
        let config = Self::server_config(root, der);
        let identity = Identity::from_pkcs12(
            &fs::read(root.join("client.p12")).unwrap_or_else(|error| panic!("{error}")),
            "integration-only",
        )
        .unwrap_or_else(|error| panic!("{error}"));
        Self {
            config,
            ca,
            identity,
        }
    }

    fn server_config(root: &Path, der: Vec<u8>) -> Arc<ServerConfig> {
        let mut roots = RootCertStore::empty();
        roots
            .add(CertificateDer::from(der.clone()))
            .unwrap_or_else(|error| panic!("{error}"));
        let verifier = rustls::server::WebPkiClientVerifier::builder(Arc::new(roots))
            .allow_unauthenticated()
            .build()
            .unwrap_or_else(|error| panic!("{error}"));
        let config = ServerConfig::builder()
            .with_client_cert_verifier(verifier)
            .with_single_cert(
                vec![CertificateDer::from(der)],
                PrivateKeyDer::from(PrivatePkcs8KeyDer::from(
                    fs::read(root.join("key.der")).unwrap_or_else(|error| panic!("{error}")),
                )),
            )
            .unwrap_or_else(|error| panic!("{error}"));
        Arc::new(config)
    }

    pub fn upstream(&self, port: u16, token: &str) -> Upstream {
        Upstream::new(
            Origin::parse(&format!("https://localhost:{port}"))
                .unwrap_or_else(|error| panic!("{error}")),
            self.ca.clone(),
            Some(self.identity.clone()),
            Some(Zeroizing::new(token.to_owned())),
        )
    }
}

fn issue_certificates(root: &Path) {
    for args in [
        vec![
            "req",
            "-x509",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-keyout",
            "key.pem",
            "-out",
            "cert.pem",
            "-days",
            "2",
            "-subj",
            "/CN=localhost",
            "-addext",
            "subjectAltName=DNS:localhost",
            "-addext",
            "basicConstraints=critical,CA:TRUE",
        ],
        vec![
            "x509", "-in", "cert.pem", "-outform", "DER", "-out", "cert.der",
        ],
        vec![
            "pkcs8", "-topk8", "-nocrypt", "-in", "key.pem", "-outform", "DER", "-out", "key.der",
        ],
        vec![
            "req",
            "-new",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-keyout",
            "client.key",
            "-out",
            "client.csr",
            "-subj",
            "/CN=producer",
        ],
    ] {
        run(root, &args);
    }
    fs::write(
        root.join("client.ext"),
        "basicConstraints=critical,CA:FALSE\nextendedKeyUsage=clientAuth\nsubjectAltName=URI:urn:layerx:webhooks:role:producer\n",
    )
    .unwrap_or_else(|error| panic!("{error}"));
    run(
        root,
        &[
            "x509",
            "-req",
            "-in",
            "client.csr",
            "-CA",
            "cert.pem",
            "-CAkey",
            "key.pem",
            "-CAcreateserial",
            "-out",
            "client.pem",
            "-days",
            "2",
            "-extfile",
            "client.ext",
        ],
    );
    run(
        root,
        &[
            "pkcs12",
            "-export",
            "-inkey",
            "client.key",
            "-in",
            "client.pem",
            "-out",
            "client.p12",
            "-passout",
            "pass:integration-only",
        ],
    );
}

fn run(root: &Path, args: &[&str]) {
    let output = Command::new("openssl")
        .current_dir(root)
        .args(args)
        .output()
        .unwrap_or_else(|error| panic!("{error}"));
    assert!(output.status.success(), "certificate generation failed");
}

pub struct Listener {
    pub port: u16,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl Listener {
    pub fn start(
        tls: Arc<ServerConfig>,
        port: u16,
        handler: impl Fn(&mut StreamOwned<ServerConnection, TcpStream>) + Send + 'static,
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
        let worker = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((tcp, _)) => {
                        tcp.set_read_timeout(Some(Duration::from_secs(3)))
                            .unwrap_or_else(|error| panic!("{error}"));
                        tcp.set_write_timeout(Some(Duration::from_secs(3)))
                            .unwrap_or_else(|error| panic!("{error}"));
                        let conn = ServerConnection::new(Arc::clone(&tls))
                            .unwrap_or_else(|error| panic!("{error}"));
                        handler(&mut StreamOwned::new(conn, tcp));
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
            worker: Some(worker),
        }
    }
}

impl Drop for Listener {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            assert!(worker.join().is_ok());
        }
    }
}

pub struct Webhooks {
    redis: std::process::Child,
    redis_log: PathBuf,
    child: Option<std::process::Child>,
    child_log: PathBuf,
    command: Command,
    _readiness: Listener,
    _kms: Listener,
    pub port: u16,
}

impl Webhooks {
    pub fn start(root: &Path, sources: &[(&str, u16)]) -> Self {
        let executable = std::env::var_os("LAYERX_TEST_WEBHOOKS_BIN")
            .unwrap_or_else(|| panic!("LAYERX_TEST_WEBHOOKS_BIN required"));
        let port = free_port();
        let redis_port = free_port();
        let (redis, redis_log) = start_redis(root, redis_port);
        let child_log = root.join("webhooks.log");
        let child_output = process_log(&child_log);
        let mut command = Command::new(executable);
        command
            .env_clear()
            .stdout(
                child_output
                    .try_clone()
                    .unwrap_or_else(|error| panic!("{error}")),
            )
            .stderr(child_output);
        for (name, value) in [
            ("ROLE", "ingress".to_owned()),
            ("LISTEN", format!("127.0.0.1:{port}")),
            ("REDIS_URL", format!("rediss://localhost:{redis_port}")),
            ("INSTANCE_ID", "producer-integration".to_owned()),
            ("LXP_WIRE_VERSION", "3".to_owned()),
            ("NETWORK_ID", FIXTURE_NETWORK_ID.to_string()),
        ] {
            command.env(format!("LAYERX_WEBHOOKS_{name}"), value);
        }
        for (name, file) in [
            ("TLS_CERT_DER", "cert.der"),
            ("TLS_KEY_DER", "key.der"),
            ("INTERNAL_CA_DER", "cert.der"),
            ("INGRESS_CLIENT_CA_DER", "cert.der"),
            ("PUBLIC_CA_DER", "cert.der"),
            ("CLIENT_IDENTITY_PKCS12", "client.p12"),
        ] {
            command.env(format!("LAYERX_WEBHOOKS_{name}"), root.join(file));
        }
        for (name, value) in [
            ("CLIENT_IDENTITY_PASSWORD", "integration-only".to_owned()),
            ("REDIS_USERNAME", "producer".to_owned()),
            ("REDIS_PASSWORD", "integration-only".to_owned()),
            ("CURSOR_KEY", "11".repeat(32)),
            ("KMS_TOKEN", "kms-integration".to_owned()),
            ("NODE_TOKEN", "node-integration".to_owned()),
            ("SOURCE_TRIGGER_TOKEN", "notification-token".to_owned()),
            ("OPERATOR_TOKEN", "operator-integration".to_owned()),
        ] {
            fs::write(root.join(name), value).unwrap_or_else(|error| panic!("{error}"));
            command.env(format!("LAYERX_WEBHOOKS_{name}_FILE"), root.join(name));
        }
        configure_chain_policy(root, &mut command);
        let kms = kms_listener(root);
        command.env(
            "LAYERX_WEBHOOKS_KMS_URL",
            format!("https://localhost:{}", kms.port),
        );
        let readiness = readiness_listener(root);
        let readiness_url = format!("https://localhost:{}", readiness.port);
        command.env("LAYERX_WEBHOOKS_NODE_URL", &readiness_url);
        command.env("LAYERX_WEBHOOKS_COMET_URL", &readiness_url);
        fs::write(root.join("consumer-token"), "consumer-token")
            .unwrap_or_else(|error| panic!("{error}"));
        for kind in ["JOURNEY", "APPROVAL", "PROGRAM", "PAYMENT"] {
            let source_port = sources
                .iter()
                .find(|(name, _)| *name == kind)
                .map_or(sources[0].1, |(_, port)| *port);
            command.env(
                format!("LAYERX_WEBHOOKS_{kind}_SOURCE_URL"),
                format!("https://localhost:{source_port}"),
            );
            command.env(
                format!("LAYERX_WEBHOOKS_{kind}_SOURCE_TOKEN_FILE"),
                root.join("consumer-token"),
            );
        }
        let mut result = Self {
            redis,
            redis_log,
            child: None,
            child_log,
            command,
            _readiness: readiness,
            _kms: kms,
            port,
        };
        wait_for_listener(&mut result.redis, redis_port, "redis", &result.redis_log);
        result.restart();
        result
    }

    pub fn stop(&mut self) {
        if let Some(mut child) = self.child.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }

    pub fn restart(&mut self) {
        assert!(self.child.is_none());
        self.child = Some(
            self.command
                .spawn()
                .unwrap_or_else(|error| panic!("{error}")),
        );
        wait_for_listener(
            self.child.as_mut().unwrap_or_else(|| panic!("child")),
            self.port,
            "webhook",
            &self.child_log,
        );
    }
}

fn configure_chain_policy(root: &Path, command: &mut Command) {
    let chain_policy = root.join("chain-policy.json");
    fs::write(
        &chain_policy,
        serde_json::to_vec(&serde_json::json!({
            "chain_id": FIXTURE_CHAIN_ID,
            "validators": [{
                "public_key": ed25519_dalek::SigningKey::from_bytes(&[11; 32])
                    .verifying_key().to_bytes(),
                "voting_power": 1,
            }],
        }))
        .unwrap_or_else(|error| panic!("{error}")),
    )
    .unwrap_or_else(|error| panic!("{error}"));
    command.env("LAYERX_WEBHOOKS_CHAIN_POLICY_FILE", chain_policy);
}

fn start_redis(root: &Path, redis_port: u16) -> (std::process::Child, PathBuf) {
    fs::write(
        root.join("redis.acl"),
        "user default off\nuser producer on >integration-only ~* &* +@all\n",
    )
    .unwrap_or_else(|error| panic!("{error}"));
    fs::write(root.join("redis.conf"), format!("bind 127.0.0.1\nport 0\ntls-port {redis_port}\ntls-cert-file {}\ntls-key-file {}\ntls-ca-cert-file {}\ntls-auth-clients no\naclfile {}\nappendonly yes\nappendfsync always\ndir {}\n", root.join("cert.pem").display(), root.join("key.pem").display(), root.join("cert.pem").display(), root.join("redis.acl").display(), root.display())).unwrap_or_else(|error| panic!("{error}"));
    let redis_log = root.join("redis.log");
    let redis_output = process_log(&redis_log);
    let redis = Command::new("redis-server")
        .arg(root.join("redis.conf"))
        .stdout(
            redis_output
                .try_clone()
                .unwrap_or_else(|error| panic!("{error}")),
        )
        .stderr(redis_output)
        .spawn()
        .unwrap_or_else(|error| panic!("{error}"));
    (redis, redis_log)
}

fn kms_listener(root: &Path) -> Listener {
    let certificate = fs::read(root.join("cert.der"))
        .unwrap_or_else(|error| panic!("read KMS certificate: {error}"));
    let tls = Tls::server_config(root, certificate);
    let token = Zeroizing::new(
        fs::read_to_string(root.join("KMS_TOKEN"))
            .unwrap_or_else(|error| panic!("read KMS bearer: {error}")),
    );
    let seal_secret = Zeroizing::new(
        secret::random_hex(32).unwrap_or_else(|error| panic!("KMS seal entropy: {error}")),
    );
    let store = kms::KeyStore::open(
        &root.join("kms"),
        SealKey::derive(kms::SEAL_LABEL, seal_secret.as_bytes()),
    )
    .unwrap_or_else(|error| panic!("open fixture KMS: {error}"));
    let service = kms::Service::new(store, token, true);
    Listener::start(tls, 0, move |stream| {
        if let Ok(mut request) = http::parse_client_request(stream) {
            request.peer_verified = stream
                .conn
                .peer_certificates()
                .is_some_and(|certs| !certs.is_empty());
            let _ = http::write_response(stream, &service.route(&request));
        }
    })
}

fn readiness_listener(root: &Path) -> Listener {
    let certificate = fs::read(root.join("cert.der"))
        .unwrap_or_else(|error| panic!("read readiness certificate: {error}"));
    let tls = Tls::server_config(root, certificate);
    let token = Zeroizing::new(
        fs::read_to_string(root.join("NODE_TOKEN"))
            .unwrap_or_else(|error| panic!("read readiness bearer: {error}")),
    );
    Listener::start(tls, 0, move |stream| {
        let Ok(request) = http::parse_client_request(stream) else {
            return;
        };
        let response = if !request.bearer_matches(&token) {
            http::refusal(401, "unauthorized", None)
        } else if request.method != "POST" || request.path != "/" {
            http::refusal(404, "not_found", None)
        } else {
            let reply = match serde_json::from_slice::<serde_json::Value>(&request.body) {
                Ok(rpc) => {
                    let id = rpc.get("id").cloned().unwrap_or(serde_json::Value::Null);
                    if rpc["jsonrpc"] != "2.0" || !rpc["method"].is_string() {
                        serde_json::json!({"jsonrpc": "2.0", "id": id,
                            "error": {"code": -32600, "message": "Invalid Request"}})
                    } else {
                        match rpc["method"].as_str() {
                            Some("eth_chainId") => serde_json::json!({
                                "jsonrpc": "2.0", "id": id,
                                "result": format!("{FIXTURE_NETWORK_ID:#x}"),
                            }),
                            Some("status") => serde_json::json!({
                                "jsonrpc": "2.0", "id": id,
                                "result": {
                                    "node_info": {"network": FIXTURE_CHAIN_ID},
                                    "sync_info": {"catching_up": false},
                                },
                            }),
                            _ => serde_json::json!({"jsonrpc": "2.0", "id": id,
                                "error": {"code": -32601, "message": "Method not found"}}),
                        }
                    }
                }
                Err(_) => serde_json::json!({"jsonrpc": "2.0", "id": null,
                    "error": {"code": -32700, "message": "Parse error"}}),
            };
            http::json(200, &reply)
        };
        let _ = http::write_response(stream, &response);
    })
}

fn process_log(path: &Path) -> fs::File {
    fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(path)
        .unwrap_or_else(|error| panic!("open process log {}: {error}", path.display()))
}

fn wait_for_listener(child: &mut std::process::Child, port: u16, name: &str, log: &Path) {
    for _ in 0..100 {
        if let Some(status) = child
            .try_wait()
            .unwrap_or_else(|error| panic!("{name} status: {error}; {}", log_tail(log)))
        {
            panic!(
                "{name} exited before listening ({status}); {}",
                log_tail(log)
            );
        }
        if TcpStream::connect(("127.0.0.1", port)).is_ok() {
            return;
        }
        std::thread::sleep(Duration::from_millis(50));
    }
    panic!("{name} did not listen; {}", log_tail(log));
}

fn log_tail(path: &Path) -> String {
    let output = fs::read_to_string(path)
        .unwrap_or_else(|error| panic!("read process log {}: {error}", path.display()));
    let lines = output.lines().rev().take(20).collect::<Vec<_>>();
    format!(
        "{}:\n{}",
        path.display(),
        lines.into_iter().rev().collect::<Vec<_>>().join("\n")
    )
}

fn free_port() -> u16 {
    TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("{error}"))
        .local_addr()
        .unwrap_or_else(|error| panic!("{error}"))
        .port()
}

impl Drop for Webhooks {
    fn drop(&mut self) {
        self.stop();
        let _ = self.redis.kill();
        let _ = self.redis.wait();
    }
}
