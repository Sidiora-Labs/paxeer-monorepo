use layerx_platform_internal::http;
use rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::{ServerConfig, ServerConnection, StreamOwned};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

const RPC_URL: &str = "LAYERX_GATEWAY_CONSENSUS_RPC_URL";

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

/// A real TLS Redis for the gateway's durable store, and the localhost
/// certificate every TLS peer of the test presents.
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
            "layerx-chain-reads-{}-{}",
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
                "chain-reads".to_owned(),
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

struct Consensus {
    port: u16,
    hits: Arc<AtomicUsize>,
    stop: Arc<AtomicBool>,
    join: Option<JoinHandle<()>>,
}

impl Consensus {
    fn start(tls: &Arc<ServerConfig>, method: &'static str, pages: Vec<Value>) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("consensus listener: {error}"));
        let port = listener
            .local_addr()
            .unwrap_or_else(|error| panic!("{error}"))
            .port();
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("{error}"));
        let hits = Arc::new(AtomicUsize::new(0));
        let stop = Arc::new(AtomicBool::new(false));
        let (tls, count, stopped) = (Arc::clone(tls), Arc::clone(&hits), Arc::clone(&stop));
        let join = thread::spawn(move || {
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
                        let request = http::parse_client_request(&mut stream)
                            .unwrap_or_else(|error| panic!("consensus request: {error}"));
                        assert_eq!(request.method, "POST");
                        assert_eq!(request.path, "/consensus");
                        let call: Value = serde_json::from_slice(&request.body)
                            .unwrap_or_else(|error| panic!("{error}"));
                        assert_eq!(call["jsonrpc"], "2.0");
                        assert_eq!(call["method"], method);
                        match method {
                            "status" => assert_eq!(call["params"], json!({})),
                            "eth_chainId" => assert_eq!(call["params"], json!([])),
                            _ => assert_eq!(call["params"]["height"], "5"),
                        }
                        let hit = count.fetch_add(1, Ordering::AcqRel);
                        let page = if method == "validators" {
                            assert_eq!(call["params"]["per_page"], "100");
                            call["params"]["page"]
                                .as_str()
                                .and_then(|page| page.parse::<usize>().ok())
                                .unwrap_or_else(|| panic!("page required"))
                                - 1
                        } else if method == "status" {
                            hit
                        } else {
                            0
                        };
                        let response = http::json(
                            200,
                            &json!({
                                "jsonrpc": "2.0", "id": call["id"], "result": pages[page]
                            }),
                        );
                        http::write_response(&mut stream, &response)
                            .unwrap_or_else(|error| panic!("consensus response: {error}"));
                        stream.conn.send_close_notify();
                        stream.flush().unwrap_or_else(|error| panic!("{error}"));
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(5));
                    }
                    Err(error) => panic!("{error}"),
                }
            }
        });
        Self {
            port,
            hits,
            stop,
            join: Some(join),
        }
    }

    fn url(&self) -> String {
        format!("https://localhost:{}/consensus", self.port)
    }
}

impl Drop for Consensus {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(join) = self.join.take() {
            assert!(join.join().is_ok(), "consensus listener failed");
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
    fn start(fixture: &Fixture, endpoint: Option<&str>) -> Self {
        Self::start_with_chain(fixture, endpoint, None)
    }

    fn start_with_chain(fixture: &Fixture, endpoint: Option<&str>, chain: Option<&str>) -> Self {
        let port = free_port();
        let mut environment = fixture.environment(port);
        if let Some(endpoint) = endpoint {
            environment.push((RPC_URL.to_owned(), endpoint.to_owned()));
        }
        if let Some(chain) = chain {
            environment.push((
                "LAYERX_GATEWAY_PAXEER_RPC_URLS".to_owned(),
                chain.to_owned(),
            ));
        }
        let child = gateway_command(&environment)
            .stdout(Stdio::null())
            .stderr(Stdio::inherit())
            .spawn()
            .unwrap_or_else(|error| panic!("gateway must start: {error}"));
        let mut gateway = Self { child, port };
        for _ in 0..200 {
            if let Ok(Some(status)) = gateway.child.try_wait() {
                panic!("gateway refused start-up: {status}");
            }
            if gateway
                .exchange("GET", "/livez", b"")
                .is_ok_and(|(status, _)| status == 200)
            {
                return gateway;
            }
            thread::sleep(Duration::from_millis(50));
        }
        panic!("gateway did not become live")
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

    fn rpc(&self, method: &str, params: Value) -> Value {
        let call =
            json!({"jsonrpc": "2.0", "id": "chain-read", "method": method, "params": params});
        let (status, body) = self
            .exchange("POST", "/rpc", call.to_string().as_bytes())
            .unwrap_or_else(|error| panic!("RPC exchange: {error}"));
        assert_eq!(status, 200, "{}", String::from_utf8_lossy(&body));
        let answer: Value =
            serde_json::from_slice(&body).unwrap_or_else(|error| panic!("RPC response: {error}"));
        assert_eq!(answer["jsonrpc"], "2.0");
        assert_eq!(answer["id"], "chain-read");
        answer
    }
}

fn assert_unavailable(answer: &Value) {
    assert_eq!(answer["error"]["code"], -32003, "{answer}");
    assert_eq!(
        answer["error"]["data"]["code"], "consensus_unavailable",
        "{answer}"
    );
    assert!(answer.get("result").is_none(), "{answer}");
}

fn assert_invalid_heights(gateway: &Gateway, method: &str) {
    for params in [
        json!(["0"]),
        json!([""]),
        json!(["-1"]),
        json!(["+5"]),
        json!(["0x5"]),
        json!(["5.0"]),
        json!([" 5"]),
        json!(["18446744073709551616"]),
        json!([5]),
        json!([null]),
        json!([]),
        json!(["5", "6"]),
        json!({"height": "5"}),
        json!(null),
    ] {
        let answer = gateway.rpc(method, params.clone());
        assert_eq!(
            answer["error"]["code"], -32602,
            "{method} {params}: {answer}"
        );
        assert!(answer.get("result").is_none());
    }
}

#[test]
fn readyz_reports_the_consensus_endpoint() {
    let fixture = Fixture::start();
    let chain = Consensus::start(&fixture.tls, "eth_chainId", vec![json!("0x7d")]);
    let secondary = Consensus::start(&fixture.tls, "eth_chainId", vec![json!("0x7d")]);
    let chain_urls = json!([chain.url(), secondary.url()]).to_string();
    let consensus = Consensus::start(
        &fixture.tls,
        "status",
        vec![
            json!({"sync_info": {"catching_up": false}}),
            json!({"sync_info": {"catching_up": true}}),
            json!({"sync_info": {"catching_up": false}}),
            json!({"sync_info": {}}),
            json!({"sync_info": {"catching_up": "false"}}),
        ],
    );
    let gateway = Gateway::start_with_chain(&fixture, Some(&consensus.url()), Some(&chain_urls));
    for (expected_status, expected_state, expected_reason) in [
        (200, "ready", "ready"),
        (503, "unavailable", "unreachable"),
        (200, "ready", "ready"),
        (503, "unavailable", "unreachable"),
        (503, "unavailable", "unreachable"),
    ] {
        let (status, body) = gateway
            .exchange("GET", "/readyz", b"")
            .unwrap_or_else(|error| panic!("readiness exchange: {error}"));
        let document: Value = serde_json::from_slice(&body)
            .unwrap_or_else(|error| panic!("readiness response: {error}"));
        assert_eq!(status, expected_status, "{document}");
        assert_eq!(
            document["backends"]["consensus"],
            json!({
                "state": expected_state, "reason": expected_reason
            }),
            "{document}"
        );
        assert_eq!(
            document["backends"]["paxeer_chain"]["state"], "ready",
            "{document}"
        );
        assert_eq!(
            document["capabilities"]["chain_serving"]["state"], "ready",
            "{document}"
        );
    }
    assert_eq!(consensus.hits.load(Ordering::Acquire), 5);
    drop(gateway);

    let unconfigured = Gateway::start_with_chain(&fixture, None, Some(&chain_urls));
    let (status, body) = unconfigured
        .exchange("GET", "/readyz", b"")
        .unwrap_or_else(|error| panic!("unconfigured readiness exchange: {error}"));
    let document: Value = serde_json::from_slice(&body)
        .unwrap_or_else(|error| panic!("unconfigured readiness response: {error}"));
    assert_eq!(status, 200, "{document}");
    assert_eq!(
        document["backends"]["consensus"],
        json!({
            "state": "unavailable", "reason": "not_configured"
        }),
        "{document}"
    );
    assert_eq!(
        document["backends"]["paxeer_chain"]["state"], "ready",
        "{document}"
    );
    drop(unconfigured);

    let invalid = Gateway::start_with_chain(&fixture, Some(""), Some(&chain_urls));
    let (status, body) = invalid
        .exchange("GET", "/readyz", b"")
        .unwrap_or_else(|error| panic!("invalid readiness exchange: {error}"));
    let document: Value = serde_json::from_slice(&body)
        .unwrap_or_else(|error| panic!("invalid readiness response: {error}"));
    assert_eq!(status, 503, "{document}");
    assert_eq!(
        document["backends"]["consensus"],
        json!({
            "state": "unavailable", "reason": "unreachable"
        }),
        "{document}"
    );
}

#[test]
fn px_get_commit_answers_the_signed_header_of_a_height() {
    let fixture = Fixture::start();
    let signed_header = json!({
        "header": {
            "version": {"block": "11", "app": "0"}, "chain_id": "chain-reads",
            "height": "5", "time": "2026-01-01T00:00:00Z",
            "last_block_id": {"hash": "11".repeat(32), "parts": {"total": 1, "hash": "22".repeat(32)}},
            "last_commit_hash": "33".repeat(32), "data_hash": "44".repeat(32),
            "validators_hash": "55".repeat(32), "next_validators_hash": "55".repeat(32),
            "consensus_hash": "66".repeat(32), "app_hash": "77".repeat(32),
            "last_results_hash": "88".repeat(32), "evidence_hash": "99".repeat(32),
            "proposer_address": "AA".repeat(20)
        },
        "commit": {"height": "5", "round": 0,
            "block_id": {"hash": "BB".repeat(32), "parts": {"total": 1, "hash": "CC".repeat(32)}},
            "signatures": [{"block_id_flag": 2, "validator_address": "AA".repeat(20),
                "timestamp": "2026-01-01T00:00:00Z", "signature": "c2lnbmF0dXJl"}]
        }
    });
    let consensus = Consensus::start(
        &fixture.tls,
        "commit",
        vec![json!({"signed_header": signed_header, "canonical": true})],
    );
    let gateway = Gateway::start(&fixture, Some(&consensus.url()));
    let answer = gateway.rpc("px_getCommit", json!(["5"]));
    assert_eq!(
        answer["result"],
        json!({"height": "5", "signedHeader": signed_header}),
        "{answer}"
    );
    assert!(answer.get("error").is_none());
    assert_invalid_heights(&gateway, "px_getCommit");
    assert_eq!(consensus.hits.load(Ordering::Acquire), 1);
    drop(gateway);
    let unconfigured = Gateway::start(&fixture, None);
    assert_unavailable(&unconfigured.rpc("px_getCommit", json!(["5"])));
    assert_invalid_heights(&unconfigured, "px_getCommit");
}

#[test]
fn px_get_validators_answers_the_set_at_a_height() {
    let fixture = Fixture::start();
    let validators: Vec<Value> = (0_u64..101).map(|index| {
        let public_key = ed25519_dalek::SigningKey::from_bytes(&[(index + 1) as u8; 32])
            .verifying_key().to_bytes();
        let address = layerx_platform_internal::secret::sha256_hex(&public_key)[..40].to_ascii_uppercase();
        json!({
            "address": address,
            "pub_key": {"type": "tendermint/PubKeyEd25519", "value": layerx_platform_internal::base64::encode(&public_key)},
            "voting_power": (1000 - index).to_string(), "proposer_priority": "0"
        })
    }).collect();
    let consensus = Consensus::start(
        &fixture.tls,
        "validators",
        vec![
            json!({"block_height": "5", "validators": validators[..100], "count": "100", "total": "101"}),
            json!({"block_height": "5", "validators": validators[100..], "count": "1", "total": "101"}),
        ],
    );
    let gateway = Gateway::start(&fixture, Some(&consensus.url()));
    let expected: Vec<Value> = validators.iter().map(|validator| json!({
        "address": validator["address"], "pubKey": validator["pub_key"], "votingPower": validator["voting_power"]
    })).collect();
    let answer = gateway.rpc("px_getValidators", json!(["5"]));
    assert_eq!(
        answer["result"],
        json!({"height": "5", "validators": expected, "total": "101"}),
        "{answer}"
    );
    assert!(answer.get("error").is_none());
    assert_invalid_heights(&gateway, "px_getValidators");
    assert_eq!(consensus.hits.load(Ordering::Acquire), 2);
    drop(gateway);
    drop(consensus);
    // Keep the port bound without accepting so no other process can take the unavailable endpoint.
    let unreachable = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{error}"));
    let port = unreachable
        .local_addr()
        .unwrap_or_else(|error| panic!("{error}"))
        .port();
    let gateway = Gateway::start(
        &fixture,
        Some(&format!("https://localhost:{port}/consensus")),
    );
    assert_unavailable(&gateway.rpc("px_getValidators", json!(["5"])));
    assert_unavailable(&gateway.rpc("px_getCommit", json!(["5"])));
    drop(gateway);
    let unconfigured = Gateway::start(&fixture, None);
    assert_unavailable(&unconfigured.rpc("px_getValidators", json!(["5"])));
}

fn protobuf_field(number: u8, bytes: &[u8]) -> Vec<u8> {
    let mut field = vec![(number << 3) | 2];
    let mut length = bytes.len();
    while length >= 128 {
        field.push((length as u8 & 0x7f) | 0x80);
        length >>= 7;
    }
    field.push(length as u8);
    field.extend_from_slice(bytes);
    field
}

fn cosmos_transaction(type_url: &str, memo: &str) -> Vec<u8> {
    let mut any = protobuf_field(1, type_url.as_bytes());
    any.extend(protobuf_field(2, &[8, 5]));
    let mut body = protobuf_field(1, &any);
    body.extend(protobuf_field(2, memo.as_bytes()));
    body.extend_from_slice(&[24, 100]);
    let mut tx = protobuf_field(1, &body);
    tx.extend(protobuf_field(2, &[]));
    tx
}

fn hex_bytes(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn proof_hash(value: &Value) -> Vec<u8> {
    let text = value
        .as_str()
        .unwrap_or_else(|| panic!("hex required: {value}"));
    let text = text
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("0x required: {value}"));
    assert_eq!(text.len(), 64);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(
                std::str::from_utf8(pair).unwrap_or_else(|error| panic!("{error}")),
                16,
            )
            .unwrap_or_else(|error| panic!("{error}"))
        })
        .collect()
}

fn transaction_leaf(tx: &[u8]) -> Vec<u8> {
    Sha256::digest([&[0][..], Sha256::digest(tx).as_slice()].concat()).to_vec()
}

fn block_response(transactions: &[Vec<u8>]) -> Value {
    let mut level: Vec<_> = transactions.iter().map(|tx| transaction_leaf(tx)).collect();
    if level.is_empty() {
        level.push(Sha256::digest([]).to_vec());
    }
    while level.len() > 1 {
        level = level
            .chunks(2)
            .map(|pair| {
                if pair.len() == 1 {
                    pair[0].clone()
                } else {
                    Sha256::digest([&[1][..], &pair[0], &pair[1]].concat()).to_vec()
                }
            })
            .collect();
    }
    let txs: Vec<_> = transactions
        .iter()
        .map(|tx| layerx_platform_internal::base64::encode(tx))
        .collect();
    json!({"block_id": {"hash": "AA".repeat(32)}, "block": {
        "header": {"height": "5", "data_hash": hex_bytes(&level[0]).to_ascii_uppercase()},
        "data": {"txs": txs}
    }})
}

#[test]
fn px_get_sequence_batch_proves_tx_zero_under_the_data_hash() {
    let fixture = Fixture::start();
    let batch = cosmos_transaction(
        "/paxeer.sequencer.v1.MsgSequenceBatch",
        &"batch memo".repeat(20),
    );
    for total in [1_usize, 2, 3, 5, 8, 9] {
        let mut transactions = vec![batch.clone()];
        transactions.extend(
            (1..total).map(|index| {
                cosmos_transaction("/cosmos.bank.v1beta1.MsgSend", &index.to_string())
            }),
        );
        let block = block_response(&transactions);
        let consensus = Consensus::start(&fixture.tls, "block", vec![block.clone()]);
        let gateway = Gateway::start(&fixture, Some(&consensus.url()));
        let answer = gateway.rpc("px_getSequenceBatch", json!(["5"]));
        assert!(answer.get("error").is_none(), "{answer}");
        let result = &answer["result"];
        assert_eq!(result["height"], "5");
        assert_eq!(result["tx"], format!("0x{}", hex_bytes(&batch)));
        let proof = &result["proof"];
        assert_eq!(proof["total"], total);
        assert_eq!(proof["index"], 0);
        let mut root = proof_hash(&proof["leafHash"]);
        assert_eq!(root, transaction_leaf(&batch));
        let aunts = proof["aunts"]
            .as_array()
            .unwrap_or_else(|| panic!("{answer}"));
        let depth = if total == 1 {
            0
        } else {
            (total - 1).ilog2() as usize + 1
        };
        assert_eq!(aunts.len(), depth);
        for aunt in aunts {
            root = Sha256::digest([&[1][..], &root, &proof_hash(aunt)].concat()).to_vec();
        }
        assert_eq!(
            hex_bytes(&root).to_ascii_uppercase(),
            block["block"]["header"]["data_hash"]
        );
        assert_invalid_heights(&gateway, "px_getSequenceBatch");
        assert_eq!(consensus.hits.load(Ordering::Acquire), 1);
    }

    let valid = block_response(&[
        batch,
        cosmos_transaction("/cosmos.bank.v1beta1.MsgSend", "second"),
    ]);
    let mut corruptions = Vec::new();
    let mut wrong_root = valid.clone();
    wrong_root["block"]["header"]["data_hash"] = json!("00".repeat(32));
    corruptions.push(wrong_root);
    let mut wrong_height = valid.clone();
    wrong_height["block"]["header"]["height"] = json!("6");
    corruptions.push(wrong_height);
    let mut bad_hash = valid.clone();
    bad_hash["block"]["header"]["data_hash"] = json!("not-a-hash");
    corruptions.push(bad_hash);
    let mut bad_base64 = valid.clone();
    bad_base64["block"]["data"]["txs"][0] = json!("!!!!");
    corruptions.push(bad_base64);
    for index in [0, 1] {
        let mut changed_tx = valid.clone();
        changed_tx["block"]["data"]["txs"][index] =
            json!(layerx_platform_internal::base64::encode(
                &cosmos_transaction("/paxeer.sequencer.v1.MsgSequenceBatch", "changed")
            ));
        corruptions.push(changed_tx);
    }
    for block in corruptions {
        let consensus = Consensus::start(&fixture.tls, "block", vec![block]);
        let gateway = Gateway::start(&fixture, Some(&consensus.url()));
        let answer = gateway.rpc("px_getSequenceBatch", json!(["5"]));
        assert_eq!(answer["error"]["code"], -32003, "{answer}");
        assert_eq!(
            answer["error"]["data"]["code"], "invalid_consensus_response",
            "{answer}"
        );
        assert!(answer.get("result").is_none(), "{answer}");
        assert_eq!(consensus.hits.load(Ordering::Acquire), 1);
    }
    let gateway = Gateway::start(&fixture, None);
    assert_unavailable(&gateway.rpc("px_getSequenceBatch", json!(["5"])));
}

#[test]
fn px_get_sequence_batch_refuses_a_block_without_a_batch() {
    let fixture = Fixture::start();
    let batch_url = "/paxeer.sequencer.v1.MsgSequenceBatch";
    let batch = cosmos_transaction(batch_url, "batch");
    for transactions in [
        vec![
            cosmos_transaction("/cosmos.bank.v1beta1.MsgSend", "send"),
            batch.clone(),
        ],
        vec![cosmos_transaction(
            "/cosmos.bank.v1beta1.MsgSend",
            batch_url,
        )],
        vec![cosmos_transaction(
            "/other.sequencer.v1.MsgSequenceBatch",
            "",
        )],
        vec![cosmos_transaction(
            "/paxeer.sequencer.v1.MsgSequenceBatchResponse",
            "",
        )],
        vec![cosmos_transaction(
            "/paxeer.sequencer.v1.MsgSequenceBatchExtra",
            "",
        )],
        Vec::new(),
    ] {
        let block = block_response(&transactions);
        let consensus = Consensus::start(&fixture.tls, "block", vec![block]);
        let gateway = Gateway::start(&fixture, Some(&consensus.url()));
        let answer = gateway.rpc("px_getSequenceBatch", json!(["5"]));
        assert_eq!(answer["error"]["code"], -32602, "{answer}");
        assert_eq!(
            answer["error"]["data"]["code"], "no_sequence_batch",
            "{answer}"
        );
        assert!(answer.get("result").is_none(), "{answer}");
        assert_eq!(consensus.hits.load(Ordering::Acquire), 1);
    }
}
