#[allow(dead_code)]
#[path = "../src/main.rs"]
mod daemon;

use daemon::HEADER_LIMIT;
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig, NodeRpcError};
use layerx_agentd::read::{
    LayerxdProgramBalanceReader, NativeReadRoute, ProgramAuthority, ProgramBalanceReadRoute,
};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint, SequencerError};
use layerx_programs::{
    chain_proof::{
        trust_store::TrustRoot,
        validators::{TrustedValidator, ValidatorSet},
    },
    hex, ProtocolDeploymentVerifier, Registry,
};
use serde_json::{json, Value};
use std::error::Error;
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::{DirBuilderExt, PermissionsExt};
use std::path::PathBuf;
use std::process::Command;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

type Result<T> = std::result::Result<T, Box<dyn Error>>;
const BEARER: &str = "serve-routes-client-bearer-credential";
static NEXT_AUTHORITY: AtomicUsize = AtomicUsize::new(0);

struct Authority {
    root: PathBuf,
    ca: Vec<u8>,
    key: ReceiptKeyEntry,
    verifier: ProtocolDeploymentVerifier,
}

impl Authority {
    fn new() -> Result<Self> {
        let root = std::env::temp_dir().join(format!(
            "layerx-serve-routes-{}-{}",
            std::process::id(),
            NEXT_AUTHORITY.fetch_add(1, Ordering::Relaxed)
        ));
        fs::DirBuilder::new().mode(0o700).create(&root)?;
        let generated = Command::new("openssl")
            .args([
                "req",
                "-x509",
                "-newkey",
                "ed25519",
                "-nodes",
                "-subj",
                "/CN=serve-routes",
                "-days",
                "1",
                "-outform",
                "DER",
                "-out",
            ])
            .arg(root.join("ca.der"))
            .arg("-keyout")
            .arg(root.join("key.pem"))
            .output()?;
        assert!(
            generated.status.success(),
            "{}",
            String::from_utf8_lossy(&generated.stderr)
        );
        let public = Command::new("openssl")
            .args(["pkey", "-pubout", "-outform", "DER", "-in"])
            .arg(root.join("key.pem"))
            .output()?;
        assert!(public.status.success());
        let public_key: [u8; 32] = public.stdout[public.stdout.len() - 32..].try_into()?;
        let key = ReceiptKeyEntry {
            key_id: [1; 32],
            public_key,
            first_height: 1,
            last_height: 100,
        };
        let trust = TrustRoot {
            chain_id: "paxeer-125".to_owned(),
            height: key.first_height,
            time_ms: 1,
            header_hash: key.key_id,
            validators: ValidatorSet::new(vec![TrustedValidator::new(public_key, 1)])?,
        };
        let path = root.join("chain-trust-root");
        fs::write(&path, trust.canonical_bytes()?)?;
        fs::set_permissions(&path, fs::Permissions::from_mode(0o600))?;
        let persisted = TrustRoot::decode(&fs::read(&path)?)?;
        let verifier = ProtocolDeploymentVerifier::from_chain_trust(
            persisted.chain_id,
            persisted.validators.validators().to_vec(),
            1000,
        )?;
        Ok(Self {
            ca: fs::read(root.join("ca.der"))?,
            root,
            key,
            verifier,
        })
    }

    fn node_tls_config(&self) -> Result<rustls::ServerConfig> {
        fs::write(
            self.root.join("leaf.ext"),
            "basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:localhost\n",
        )?;
        for args in [
            vec!["x509", "-inform", "DER", "-in", "ca.der", "-out", "ca.pem"],
            vec![
                "req",
                "-new",
                "-newkey",
                "ed25519",
                "-nodes",
                "-subj",
                "/CN=localhost",
                "-keyout",
                "leaf-key.pem",
                "-out",
                "leaf.csr",
            ],
            vec![
                "x509",
                "-req",
                "-in",
                "leaf.csr",
                "-CA",
                "ca.pem",
                "-CAkey",
                "key.pem",
                "-set_serial",
                "2",
                "-days",
                "1",
                "-extfile",
                "leaf.ext",
                "-outform",
                "DER",
                "-out",
                "leaf.der",
            ],
            vec![
                "pkcs8",
                "-topk8",
                "-nocrypt",
                "-in",
                "leaf-key.pem",
                "-outform",
                "DER",
                "-out",
                "leaf-key.der",
            ],
        ] {
            let generated = Command::new("openssl")
                .args(args)
                .current_dir(&self.root)
                .output()?;
            assert!(
                generated.status.success(),
                "{}",
                String::from_utf8_lossy(&generated.stderr)
            );
        }
        let certificate =
            rustls::pki_types::CertificateDer::from(fs::read(self.root.join("leaf.der"))?);
        let key =
            rustls::pki_types::PrivatePkcs8KeyDer::from(fs::read(self.root.join("leaf-key.der"))?);
        Ok(rustls::ServerConfig::builder_with_provider(Arc::new(
            rustls::crypto::ring::default_provider(),
        ))
        .with_safe_default_protocol_versions()?
        .with_no_client_auth()
        .with_single_cert(vec![certificate], key.into())?)
    }
}

impl Drop for Authority {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

#[derive(Clone, Copy)]
enum NodeState {
    Unavailable,
    NoKeys,
    Ready,
    WrongChain,
}

struct NodePeer {
    address: SocketAddr,
    state: Arc<Mutex<NodeState>>,
    requests: Arc<AtomicUsize>,
    stopping: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl NodePeer {
    fn start(key: ReceiptKeyEntry) -> Result<Self> {
        Self::start_transport(key, None)
    }

    fn start_https(authority: &Authority) -> Result<Self> {
        Self::start_transport(authority.key, Some(Arc::new(authority.node_tls_config()?)))
    }

    fn start_transport(
        key: ReceiptKeyEntry,
        tls: Option<Arc<rustls::ServerConfig>>,
    ) -> Result<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        listener.set_nonblocking(true)?;
        let state = Arc::new(Mutex::new(NodeState::Unavailable));
        let requests = Arc::new(AtomicUsize::new(0));
        let stopping = Arc::new(AtomicBool::new(false));
        let current = Arc::clone(&state);
        let captured = Arc::clone(&requests);
        let stopped = Arc::clone(&stopping);
        let worker = thread::spawn(move || {
            'connections: while !stopped.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("node accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(2)))
                    .unwrap();
                stream
                    .set_write_timeout(Some(Duration::from_secs(2)))
                    .unwrap();
                if let Some(config) = &tls {
                    let connection = rustls::ServerConnection::new(Arc::clone(config)).unwrap();
                    let mut stream = rustls::StreamOwned::new(connection, stream);
                    while stream.conn.is_handshaking() {
                        if stream.conn.complete_io(&mut stream.sock).is_err() {
                            continue 'connections;
                        }
                    }
                    Self::answer(stream, &current, &captured, &key);
                } else {
                    Self::answer(stream, &current, &captured, &key);
                }
            }
        });
        Ok(Self {
            address,
            state,
            requests,
            stopping,
            worker: Some(worker),
        })
    }

    fn answer(
        stream: impl Read + Write,
        current: &Mutex<NodeState>,
        captured: &AtomicUsize,
        key: &ReceiptKeyEntry,
    ) {
        let mut reader = BufReader::new(stream);
        let mut length = None;
        loop {
            let mut line = String::new();
            assert_ne!(reader.read_line(&mut line).unwrap(), 0);
            if line == "\r\n" {
                break;
            }
            if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                length = Some(value.trim().parse::<usize>().unwrap());
            }
        }
        let length = length.unwrap();
        assert!(length <= 64 * 1024);
        let mut bytes = vec![0; length];
        reader.read_exact(&mut bytes).unwrap();
        let request: Value = serde_json::from_slice(&bytes).unwrap();
        assert_eq!(request["jsonrpc"], "2.0");
        assert_eq!(request["params"], json!([]));
        captured.fetch_add(1, Ordering::AcqRel);
        let state = *current.lock().unwrap();
        let (status, answer) = if matches!(state, NodeState::Unavailable) {
            (503, json!({"error": "node_unavailable"}))
        } else {
            let result = match request["method"].as_str() {
                Some("eth_chainId") => json!(if matches!(state, NodeState::WrongChain) {
                    "0x7e"
                } else {
                    "0x7d"
                }),
                Some("px_getReceiptKeys") => {
                    if matches!(state, NodeState::NoKeys) {
                        json!([])
                    } else {
                        json!([{
                            "keyId": format!("0x{}", hex::encode(&key.key_id)),
                            "publicKey": format!("0x{}", hex::encode(&key.public_key)),
                            "firstHeight": format!("{:#x}", key.first_height),
                            "lastHeight": format!("{:#x}", key.last_height),
                        }])
                    }
                }
                other => panic!("health or invalid route issued unexpected RPC: {other:?}"),
            };
            (
                200,
                json!({"jsonrpc": "2.0", "id": request["id"], "result": result}),
            )
        };
        let answer = answer.to_string();
        let mut stream = reader.into_inner();
        write!(stream, "HTTP/1.1 {status} Result\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()).unwrap();
        stream.flush().unwrap();
    }

    fn set(&self, state: NodeState) {
        *self.state.lock().unwrap() = state;
    }

    fn stop(&mut self) {
        self.stopping.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            if worker.join().is_err() && !thread::panicking() {
                panic!("node peer failed");
            }
        }
    }
}

impl Drop for NodePeer {
    fn drop(&mut self) {
        self.stop();
    }
}

struct RouteService {
    address: SocketAddr,
    _pool: daemon::HttpPool,
}

impl RouteService {
    fn start(authority: &Authority, node: &NodePeer, with_native: bool) -> Result<Self> {
        Self::start_at(authority, format!("http://{}/", node.address), with_native)
    }

    fn start_at(authority: &Authority, node_url: String, with_native: bool) -> Result<Self> {
        let node_config = NodeRpcConfig {
            url: node_url,
            chain_id: 125,
            deadline: Duration::from_secs(2),
        };
        let reader = LayerxdProgramBalanceReader::connect(
            &node_config.url,
            "node-read-credential".to_owned(),
            ProgramAuthority {
                endpoint: "https://localhost:1",
                authorization: "authority-read-credential".to_owned(),
                replica_id: [2; 32],
                ca_der: &authority.ca,
            },
            authority.verifier.clone(),
            Registry::new(),
        )
        .map_err(|error| format!("program reader: {error:?}"))?;
        let endpoint = SequencerEndpoint::new(&node_config.url, &[&authority.ca])
            .map_err(|error| format!("node endpoint: {error:?}"))?;
        let native = if with_native {
            Some(NativeReadRoute::new(
                SequencerEndpoint::new(&node_config.url, &[&authority.ca])
                    .map_err(|error| format!("native endpoint: {error:?}"))?,
                125,
            ))
        } else {
            None
        };
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let pool = daemon::serve_read_routes(
            listener,
            BEARER.to_owned(),
            ProgramBalanceReadRoute::new(reader),
            native,
            node_config,
            authority.ca.clone(),
            endpoint,
        )?;
        Ok(Self {
            address,
            _pool: pool,
        })
    }

    fn get(&self, path: &str) -> Result<(u16, Value)> {
        let mut stream = TcpStream::connect(self.address)?;
        stream.set_read_timeout(Some(Duration::from_secs(5)))?;
        stream.set_write_timeout(Some(Duration::from_secs(5)))?;
        write!(stream, "GET {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {BEARER}\r\nConnection: close\r\n\r\n")?;
        stream.flush()?;
        let mut answer = String::new();
        stream.read_to_string(&mut answer)?;
        let (header, body) = answer
            .split_once("\r\n\r\n")
            .ok_or("missing HTTP response body")?;
        let status = header
            .split_ascii_whitespace()
            .nth(1)
            .ok_or("missing HTTP status")?
            .parse()?;
        assert!(header.contains("Content-Type: application/json"));
        assert!(header.contains("Cache-Control: no-store"));
        Ok((status, serde_json::from_str(body)?))
    }
}

#[test]
fn healthz_trusts_the_configured_node_ca_over_https() -> Result<()> {
    let authority = Authority::new()?;
    let other_authority = Authority::new()?;
    let node = NodePeer::start_https(&authority)?;
    node.set(NodeState::Ready);
    let url = format!("https://localhost:{}/", node.address.port());
    let node_config = NodeRpcConfig {
        url: url.clone(),
        chain_id: 125,
        deadline: Duration::from_secs(2),
    };
    assert_eq!(
        NodeRpc::connect_with_ca(&node_config, &[]).unwrap_err(),
        NodeRpcError::Sequencer(SequencerError::Tls)
    );
    let service = RouteService::start_at(&authority, url.clone(), false)?;
    let (status, body) = service.get("/healthz")?;
    assert_eq!(status, 200);
    assert_eq!(body["ready"], true);
    assert_eq!(body["transport_ready"], true);
    assert_eq!(node.requests.load(Ordering::Acquire), 2);

    let wrong_ca = RouteService::start_at(&other_authority, url, false)?;
    let wrong_host =
        RouteService::start_at(&authority, format!("https://{}/", node.address), false)?;
    for refused in [&wrong_ca, &wrong_host] {
        let (status, body) = refused.get("/healthz")?;
        assert_eq!(status, 503);
        assert_eq!(body["ready"], false);
        assert_eq!(body["transport_ready"], false);
    }
    let (status, body) = wrong_ca.get(&format!("/v1/programs/{}/balances", "11".repeat(32)))?;
    assert_eq!(status, 503);
    assert_eq!(body, json!({"error": "program_state_unavailable"}));
    assert_eq!(node.requests.load(Ordering::Acquire), 2);

    node.set(NodeState::WrongChain);
    let (status, body) = service.get("/healthz")?;
    assert_eq!(status, 503);
    assert_eq!(body["ready"], false);
    assert_eq!(body["transport_ready"], false);
    node.set(NodeState::Ready);
    let (status, body) = service.get("/healthz")?;
    assert_eq!(status, 200);
    assert_eq!(body["ready"], true);
    assert_eq!(body["transport_ready"], true);
    Ok(())
}

#[test]
fn healthz_reports_not_ready_until_the_node_answers() -> Result<()> {
    let authority = Authority::new()?;
    let mut node = NodePeer::start(authority.key)?;
    let service = RouteService::start(&authority, &node, false)?;
    let (status, body) = service.get("/healthz")?;
    assert_eq!(status, 503);
    assert_eq!(body["ready"], false);
    thread::sleep(Duration::from_millis(300));
    node.set(NodeState::Ready);
    let (status, body) = service.get("/healthz")?;
    assert_eq!(status, 200);
    assert_eq!(body["ready"], true);
    for (state, status, transport_ready, ready) in [
        (NodeState::Unavailable, 503, false, false),
        (NodeState::NoKeys, 503, true, false),
        (NodeState::Ready, 200, true, true),
        (NodeState::WrongChain, 503, false, false),
        (NodeState::Ready, 200, true, true),
        (NodeState::NoKeys, 503, true, false),
    ] {
        node.set(state);
        let (actual_status, body) = service.get("/healthz")?;
        assert_eq!(actual_status, status);
        assert_eq!(
            body,
            json!({
                "ready": ready, "transport_ready": transport_ready,
                "verified_reads_ready": ready, "tenant_writes": "unknown",
            })
        );
    }
    assert_eq!(node.requests.load(Ordering::Acquire), 13);
    node.stop();
    let (status, body) = service.get("/healthz")?;
    assert_eq!(status, 503);
    assert_eq!(body["ready"], false);
    assert_eq!(body["transport_ready"], false);
    Ok(())
}

#[test]
fn unknown_program_balance_route_is_invalid_program() -> Result<()> {
    let authority = Authority::new()?;
    let node = NodePeer::start(authority.key)?;
    let service = RouteService::start(&authority, &node, true)?;
    for malformed in [
        "unknown",
        "zz",
        "",
        "00",
        &"00".repeat(32),
        &"gg".repeat(32),
        &"11".repeat(33),
    ] {
        let (status, body) = service.get(&format!("/v1/programs/{malformed}/balances"))?;
        assert_eq!(status, 400, "malformed program {malformed}");
        assert_eq!(body, json!({"error": "invalid_program"}));
    }
    assert_eq!(node.requests.load(Ordering::Acquire), 0);
    let (status, body) = service.get("/v1/reads/unknown/1")?;
    assert_eq!(status, 400);
    assert_eq!(body, json!({"error": "invalid_read"}));
    let (status, body) = service.get(&format!("/v1/programs/{}/balances", "11".repeat(32)))?;
    assert_eq!(status, 503);
    assert_eq!(body, json!({"error": "program_state_unavailable"}));
    Ok(())
}
