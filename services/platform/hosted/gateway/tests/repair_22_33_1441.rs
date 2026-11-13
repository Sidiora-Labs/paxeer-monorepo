//! Task 22.33.1441: retired-ws-ingress is proven by
//! `repair_22_33_1441_retired_ingress_and_published_transports`.
//! Baseline: not run; the ordered BUILD instructions prohibit pre-implementation gates.
//! Wiring request: services/platform/hosted/gateway/src/main.rs and
//! services/platform/hosted/gateway/route-locations.json (domain 8) retain the
//! prerequisite root transport; sdk/platform (SDK owner) consumes node-root WS
//! subscriptions and the operator-provided hosted origin.

use layerx_platform_gateway::http;
use layerx_platform_gateway::runtime::Server;
use layerx_platform_gateway::store::{KeyRecord, RedisEndpoint, RedisStore};
use layerx_platform_gateway::{gateway_digest, unified_service_catalogue, IssuedKey};
use rustls::pki_types::{PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::{ServerConfig, ServerConnection, StreamOwned};
use serde_json::{json, Value};
use std::ffi::OsString;
use std::fs;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::os::unix::fs::PermissionsExt;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::time::Duration;
use zeroize::Zeroizing;

#[path = "support/repair_22_31_202.rs"]
pub mod support;
#[path = "../src/ws_wire.rs"]
mod ws_wire;
use support::{fixture, json_response, Peer, RedisMemory};

struct Environment(Vec<(OsString, OsString)>);
impl Environment {
    fn clear() -> Self {
        let previous: Vec<_> = std::env::vars_os().filter(|(name, _)| name.to_string_lossy().starts_with("LAYERX_GATEWAY_")).collect();
        for (name, _) in &previous {
            std::env::remove_var(name);
        }
        Self(previous)
    }
}
impl Drop for Environment {
    fn drop(&mut self) {
        for (name, _) in std::env::vars_os().filter(|(name, _)| name.to_string_lossy().starts_with("LAYERX_GATEWAY_")) {
            std::env::remove_var(name);
        }
        for (name, value) in &self.0 {
            std::env::set_var(name, value);
        }
    }
}

fn headers(stream: &mut impl Read) -> String {
    let mut bytes = Vec::new();
    while !bytes.ends_with(b"\r\n\r\n") {
        assert!(bytes.len() < 8192, "bounded upgrade headers");
        let mut byte = [0];
        stream.read_exact(&mut byte).unwrap_or_else(|error| panic!("{}: {error:?}", "upgrade header byte"));
        bytes.push(byte[0]);
    }
    String::from_utf8(bytes).unwrap_or_else(|error| panic!("{}: {error:?}", "upgrade headers UTF-8"))
}

struct NodeSocket(native_tls::TlsStream<TcpStream>);
impl NodeSocket {
    fn open(node: &Peer, ca: &native_tls::Certificate, path: &str) -> Self {
        let tcp = TcpStream::connect(node.address).unwrap_or_else(|error| panic!("{}: {error:?}", "node socket"));
        tcp.set_read_timeout(Some(Duration::from_secs(3))).unwrap_or_else(|error| panic!("{}: {error:?}", "node read deadline"));
        tcp.set_write_timeout(Some(Duration::from_secs(3))).unwrap_or_else(|error| panic!("{}: {error:?}", "node write deadline"));
        let mut stream = native_tls::TlsConnector::builder()
            .add_root_certificate(ca.clone())
            .build()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "node TLS connector"))
            .connect("localhost", tcp)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "node TLS"));
        write!(stream, "GET {path} HTTP/1.1\r\nHost: localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n").unwrap_or_else(|error| panic!("{}: {error:?}", "node upgrade"));
        stream.flush().unwrap_or_else(|error| panic!("{}: {error:?}", "flush upgrade"));
        let answer = headers(&mut stream);
        assert!(answer.starts_with("HTTP/1.1 101 Switching Protocols\r\n"));
        assert!(answer.contains("Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=\r\n"));
        Self(stream)
    }
    fn send(&mut self, opcode: u8, body: &[u8]) {
        let mut mask = [0; 4];
        getrandom::fill(&mut mask).unwrap_or_else(|error| panic!("{}: {error:?}", "client mask randomness"));
        let mut frame = vec![0x80 | opcode];
        if body.len() < 126 {
            frame.push(0x80 | u8::try_from(body.len()).unwrap_or_else(|error| panic!("{}: {error:?}", "short frame")));
        } else {
            frame.push(0x80 | 126);
            frame.extend_from_slice(&u16::try_from(body.len()).unwrap_or_else(|error| panic!("{}: {error:?}", "bounded client frame")).to_be_bytes());
        }
        frame.extend_from_slice(&mask);
        frame.extend(body.iter().enumerate().map(|(i, byte)| byte ^ mask[i % 4]));
        self.0.write_all(&frame).unwrap_or_else(|error| panic!("{}: {error:?}", "masked client frame"));
        self.0.flush().unwrap_or_else(|error| panic!("{}: {error:?}", "flush client frame"));
    }
    fn call(&mut self, id: u64, method: &str, params: &Value) {
        self.send(1, &serde_json::to_vec(&json!({"jsonrpc":"2.0","id":id,"method":method,"params":params})).unwrap_or_else(|error| panic!("{}: {error:?}", "node request JSON")));
    }
    fn frame(&mut self) -> (u8, Vec<u8>) {
        let mut header = [0; 2];
        self.0.read_exact(&mut header).unwrap_or_else(|error| panic!("{}: {error:?}", "node frame header"));
        assert_eq!(header[0] & 0xf0, 0x80, "final uncompressed frame");
        assert_eq!(header[1] & 0x80, 0, "server frames are unmasked");
        let length = match header[1] {
            126 => {
                let mut bytes = [0; 2];
                self.0.read_exact(&mut bytes).unwrap_or_else(|error| panic!("{}: {error:?}", "node frame length"));
                usize::from(u16::from_be_bytes(bytes))
            }
            127 => panic!("fixture messages fit a u16 frame"),
            length => usize::from(length),
        };
        let mut body = vec![0; length];
        self.0.read_exact(&mut body).unwrap_or_else(|error| panic!("{}: {error:?}", "node frame body"));
        (header[0] & 15, body)
    }
    fn json(&mut self) -> Value {
        let (opcode, body) = self.frame();
        assert_eq!(opcode, 1);
        serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{}: {error:?}", "node JSON message"))
    }
}

fn node(tls: Arc<ServerConfig>, sessions: Arc<AtomicUsize>) -> Peer {
    Peer::start(move |tcp| {
        let mut stream = StreamOwned::new(ServerConnection::new(Arc::clone(&tls)).unwrap_or_else(|error| panic!("{}: {error:?}", "node TLS")), tcp);
        let request = http::read_request(&mut stream, 8192).unwrap_or_else(|error| panic!("{}: {error:?}", "node request"));
        assert_eq!(request.path, "/", "the node owns its root transport");
        if request.method == "POST" {
            let call: Value = serde_json::from_slice(&request.body).unwrap_or_else(|error| panic!("{}: {error:?}", "node HTTP JSON"));
            assert_eq!(call["params"], json!([]));
            let result = match call["method"].as_str().unwrap_or_else(|| panic!("{}", "node method")) {
                "eth_chainId" => json!("0x7d"),
                "eth_blockNumber" => json!(format!(
                    "0x{:x}",
                    fixture("node")["settlement"]["header"]["height"]
                        .as_str()
                        .unwrap_or_else(|| panic!("{}", "recorded height"))
                        .parse::<u64>()
                        .unwrap_or_else(|error| panic!("{}: {error:?}", "height"))
                )),
                method => panic!("unrecorded node call {method}"),
            };
            http::write_response(&mut stream, &json_response(200, &json!({"jsonrpc":"2.0","id":call["id"],"result":result}))).unwrap_or_else(|error| panic!("{}: {error:?}", "node HTTP answer"));
        } else {
            assert_eq!(request.method, "GET");
            assert_eq!(request.headers["upgrade"], "websocket");
            assert_eq!(request.headers["sec-websocket-version"], "13");
            let accept = ws_wire::accept(&request.headers["sec-websocket-key"]).unwrap_or_else(|| panic!("{}", "production upgrade codec"));
            write!(stream, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: {accept}\r\n\r\n").unwrap_or_else(|error| panic!("{}: {error:?}", "node upgrade answer"));
            stream.flush().unwrap_or_else(|error| panic!("{}: {error:?}", "node upgrade flush"));
            let session = sessions.fetch_add(1, Ordering::AcqRel) + 1;
            let mut reader = ws_wire::Reader::default();
            let mut active = None;
            loop {
                let (opcode, body) = loop {
                    if let Some(frame) = reader.read(&mut stream).unwrap_or_else(|error| panic!("{}: {error:?}", "production subscription decoder")) {
                        break frame;
                    }
                };
                if opcode == 8 {
                    ws_wire::write(&mut stream, 8, &body).unwrap_or_else(|error| panic!("{}: {error:?}", "node closes"));
                    break;
                }
                assert_eq!(opcode, 1);
                let call: Value = serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{}: {error:?}", "subscription JSON"));
                let result = match call["method"].as_str().unwrap_or_else(|| panic!("{}", "subscription method")) {
                    "eth_subscribe" => {
                        assert_eq!(call["params"], json!(["newPendingTransactions"]));
                        assert!(active.is_none());
                        active = Some(format!("0x{session:x}"));
                        json!(active)
                    }
                    "eth_unsubscribe" => {
                        let matches = active.as_deref().is_some_and(|id| call["params"] == json!([id]));
                        if matches {
                            active = None;
                        }
                        json!(matches)
                    }
                    method => panic!("hosted method reached node WS: {method}"),
                };
                ws_wire::write(&mut stream, 1, &serde_json::to_vec(&json!({"jsonrpc":"2.0","id":call["id"],"result":result})).unwrap_or_else(|error| panic!("{}: {error:?}", "subscription response"))).unwrap_or_else(|error| panic!("{}: {error:?}", "production subscription encoder"));
                if call["method"] == "eth_subscribe" {
                    let event = json!({"jsonrpc":"2.0","method":"eth_subscription","params":{"subscription":active,"result":fixture("node")["settlement"]["receipt"]["txHash"]}});
                    ws_wire::write(&mut stream, 1, &serde_json::to_vec(&event).unwrap_or_else(|error| panic!("{}: {error:?}", "recorded transaction event"))).unwrap_or_else(|error| panic!("{}: {error:?}", "node event"));
                    if session == 1 {
                        ws_wire::write(&mut stream, 8, &1013_u16.to_be_bytes()).unwrap_or_else(|error| panic!("{}: {error:?}", "node disconnect"));
                        break;
                    }
                }
            }
        }
        stream.conn.send_close_notify();
        stream.flush().unwrap_or_else(|error| panic!("{}: {error:?}", "node TLS close"));
    })
}

fn key(seed: u8, principal: &str) -> (KeyRecord, String) {
    let issued = IssuedKey::derive(&[seed; 32], b"retired-ws-ingress");
    let salt = format!("salt-{seed}");
    let record = KeyRecord {
        key_id: issued.id().into(),
        principal_digest: principal.into(),
        secret_digest: gateway_digest(&[b"gateway-key-v1", salt.as_bytes(), issued.secret().as_bytes()]),
        salt,
        signer_public_key: "11".repeat(32),
        scopes: "state:read".into(),
        quota_requests: 100,
        quota_window_seconds: 60,
        epoch: 1,
        disabled: false,
    };
    let credential = format!("LayerX-Key {}:{}", issued.id(), issued.secret());
    (record, credential)
}

#[test]
fn repair_22_33_1441_retired_ingress_and_published_transports() {
    let _environment = Environment::clear();
    let directory = tempfile::tempdir().unwrap_or_else(|error| panic!("{}: {error:?}", "fixture directory"));
    let certificate = rcgen::generate_simple_self_signed(vec!["localhost".into()]).unwrap_or_else(|error| panic!("{}: {error:?}", "fixture certificate"));
    let ca = native_tls::Certificate::from_der(certificate.cert.der()).unwrap_or_else(|error| panic!("{}: {error:?}", "fixture CA"));
    let tls = Arc::new(
        ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
            .with_safe_default_protocol_versions()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "TLS versions"))
            .with_no_client_auth()
            .with_single_cert(vec![certificate.cert.der().clone()], PrivateKeyDer::from(PrivatePkcs8KeyDer::from(certificate.signing_key.serialize_der())))
            .unwrap_or_else(|error| panic!("{}: {error:?}", "fixture TLS")),
    );
    let memory = Arc::new(Mutex::new(RedisMemory::default()));
    let redis = RedisMemory::serve(Arc::clone(&memory), Arc::clone(&tls));
    let sessions = Arc::new(AtomicUsize::new(0));
    let fallback = node(Arc::clone(&tls), Arc::clone(&sessions));
    let node = node(tls, Arc::clone(&sessions));
    let ca_path = directory.path().join("ca.der");
    fs::write(&ca_path, certificate.cert.der()).unwrap_or_else(|error| panic!("{}: {error:?}", "CA file"));
    std::env::set_var("LAYERX_GATEWAY_OUTBOUND_CA_DER", ca_path);
    for (name, value) in [("LISTEN", "127.0.0.1:0"), ("LISTENER", "plain"), ("NETWORK_ID", "retired-ws-ingress"), ("PROTOCOL_NETWORK_ID", "7")] {
        std::env::set_var(format!("LAYERX_GATEWAY_{name}"), value);
    }
    std::env::set_var("LAYERX_GATEWAY_LXP_WIRE_VERSION", layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string());
    std::env::set_var("LAYERX_GATEWAY_REDIS_URL", format!("rediss://localhost:{}", redis.address.port()));
    std::env::set_var("LAYERX_GATEWAY_PAXEER_RPC_URLS", json!([node.url(), fallback.url()]).to_string());
    for (name, value) in [("USERNAME", "gateway"), ("PASSWORD", "gateway-secret")] {
        let path = directory.path().join(name);
        fs::write(&path, value).unwrap_or_else(|error| panic!("{}: {error:?}", "fixture Redis credential"));
        fs::set_permissions(&path, fs::Permissions::from_mode(0o600)).unwrap_or_else(|error| panic!("{}: {error:?}", "protected secret"));
        std::env::set_var(format!("LAYERX_GATEWAY_REDIS_{name}_FILE"), path);
    }
    let store = RedisStore::new(
        RedisEndpoint::parse(&format!("rediss://localhost:{}", redis.address.port())).unwrap_or_else(|error| panic!("{}: {error:?}", "Redis URL")),
        ca.clone(),
        Zeroizing::new("gateway".into()),
        Zeroizing::new("gateway-secret".into()),
    );
    let principal = "42".repeat(32);
    let (old, credential) = key(1, &principal);
    store.issue_key(&old, "issue for transport proof").unwrap_or_else(|error| panic!("{}: {error:?}", "real key issuance"));
    let server = Server::from_environment().unwrap_or_else(|error| panic!("{}: {error:?}", "production gateway configuration"));
    let gateway = Peer::start(move |tcp| {
        server.serve_connection(tcp).unwrap_or_else(|error| panic!("{}: {error:?}", "production gateway transport"));
    });
    let catalogue = assert_retired_gateway_mounts(&gateway, &credential, &sessions);
    assert_published_hosted_rpc(&gateway, catalogue);
    let lookup = |authorization: &str| support::request(gateway.address, "GET", "/internal/v1/principal", &[("Authorization", authorization)], &Value::Null);
    let (status, owner) = lookup(&credential);
    assert_eq!(status, 200);
    assert_eq!(owner["result"]["principal_digest"], principal);
    let node_path = catalogue["node_rpc"]["rpc_path"].as_str().unwrap_or_else(|| panic!("{}", "published node root"));
    let mut first = NodeSocket::open(&node, &ca, node_path);
    first.call(1, "eth_subscribe", &json!(["newPendingTransactions"]));
    let first_id = first.json()["result"].as_str().unwrap_or_else(|| panic!("{}", "first subscription ID")).to_owned();
    let event = first.json();
    assert_eq!(event, json!({"jsonrpc":"2.0","method":"eth_subscription","params":{"subscription":first_id,"result":fixture("node")["settlement"]["receipt"]["txHash"]}}));
    assert_eq!(first.frame(), (8, 1013_u16.to_be_bytes().to_vec()));
    drop(first);
    let mut second = NodeSocket::open(&node, &ca, node_path);
    second.call(2, "eth_unsubscribe", &json!([first_id]));
    assert_eq!(second.json(), json!({"jsonrpc":"2.0","id":2,"result":false}), "disconnected subscriptions are not replayed");
    second.call(3, "eth_subscribe", &json!(["newPendingTransactions"]));
    let second_id = second.json()["result"].as_str().unwrap_or_else(|| panic!("{}", "fresh subscription ID")).to_owned();
    assert_ne!(second_id, first_id);
    let event = second.json();
    assert_eq!(event["params"]["subscription"], second_id);
    assert_eq!(event["params"]["result"], fixture("node")["settlement"]["receipt"]["txHash"]);
    let (replacement, new_credential) = key(2, &principal);
    store.rotate_key(&old, &replacement, "rotate and revoke original credential").unwrap_or_else(|error| panic!("{}: {error:?}", "real atomic credential revocation"));
    let (status, refused) = lookup(&credential);
    assert_eq!(status, 401);
    assert_eq!(refused["error"]["code"], "api_key_required");
    assert_eq!(lookup(&new_credential).0, 200);
    second.call(4, "eth_unsubscribe", &json!([second_id]));
    assert_eq!(second.json(), json!({"jsonrpc":"2.0","id":4,"result":true}));
    second.send(8, &1000_u16.to_be_bytes());
    assert_eq!(second.frame(), (8, 1000_u16.to_be_bytes().to_vec()));
    assert_eq!(sessions.load(Ordering::Acquire), 2);
    drop(second);
    // The real listener proof above also covers the mounts removed by the prerequisite.
    // This regression check covers the dormant combined dispatcher left in this task's file.
    assert!(!include_str!("../src/ws.rs").contains("rpc::dispatch("), "subscription frames cannot dispatch hosted methods");
    drop(gateway);
    drop(store);
    drop(node);
    drop(fallback);
    drop(redis);
}

fn assert_retired_gateway_mounts(gateway: &Peer, credential: &str, sessions: &AtomicUsize) -> &'static Value {
    let catalogue = unified_service_catalogue();
    assert_eq!(catalogue["node_rpc"]["rpc_path"], "/");
    assert_eq!(catalogue["hosted_gateway"]["rpc_path"], "/");
    assert_eq!(catalogue["hosted_gateway"]["location_source"], "operator-provided");
    let upgrade = [("Upgrade", "websocket"), ("Connection", "Upgrade"), ("Sec-WebSocket-Version", "13"), ("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ=="), ("Authorization", credential)];
    for mount in ["/rpc/ws", "/rpc/evm/ws"] {
        for path in [mount.to_owned(), format!("{mount}/"), format!("{mount}?token=legacy")] {
            let mut tcp = TcpStream::connect(gateway.address).unwrap_or_else(|error| panic!("{}: {error:?}", "retired upgrade connection"));
            tcp.set_read_timeout(Some(Duration::from_secs(3))).unwrap_or_else(|error| panic!("{}: {error:?}", "retired upgrade deadline"));
            write!(tcp, "GET {path} HTTP/1.1\r\nHost: localhost\r\n").unwrap_or_else(|error| panic!("{}: {error:?}", "retired request line"));
            for (name, value) in upgrade {
                write!(tcp, "{name}: {value}\r\n").unwrap_or_else(|error| panic!("{}: {error:?}", "upgrade header"));
            }
            tcp.write_all(b"\r\n").unwrap_or_else(|error| panic!("{}: {error:?}", "end upgrade headers"));
            assert!(headers(&mut tcp).starts_with("HTTP/1.1 404 "), "retired WS upgrade {path}");
            assert_eq!(support::request(gateway.address, "POST", &path, &[], &json!({"jsonrpc":"2.0","id":1,"method":"px_getNetwork","params":[]})).0, 404);
        }
    }
    assert_eq!(sessions.load(Ordering::Acquire), 0, "retired mounts never connect a node");
    catalogue
}

fn assert_published_hosted_rpc(gateway: &Peer, catalogue: &Value) {
    let path = catalogue["hosted_gateway"]["rpc_path"].as_str().unwrap_or_else(|| panic!("{}", "published hosted path"));
    let (status, hosted) = support::request(gateway.address, "POST", path, &[], &json!({"jsonrpc":"2.0","id":"hosted","method":"px_getNetwork","params":[]}));
    assert_eq!(status, 200);
    assert_eq!(hosted["id"], "hosted");
    assert_eq!(hosted["result"]["paxeer"]["chain_id"], format!("0x{:x}", catalogue["node_rpc"]["chain_id"].as_u64().unwrap_or_else(|| panic!("{}", "published chain ID"))));
    assert_eq!(
        hosted["result"]["paxeer"]["latest_block"],
        format!(
            "0x{:x}",
            fixture("node")["settlement"]["header"]["height"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "recorded height"))
                .parse::<u64>()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "height"))
        )
    );
}
