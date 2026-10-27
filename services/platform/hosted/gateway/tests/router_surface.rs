use layerx_platform_gateway::http::{read_request, IncomingRequest};
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
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

const ROUTER_METHODS: [&str; 17] = layerx_platform_gateway::ROUTER_METHODS;
const ACCOUNT: &str = "0x102132435465768798a9bacbdcedfe0f1e2d3c4b";
const CHAIN_HEIGHT: u64 = 6699;

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
            "layerx-router-surface-{}-{}",
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
    fn start(
        fixture: &Fixture,
        chain: &Listener,
        consensus: &Listener,
        indexer: &Listener,
    ) -> Self {
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
        let child = gateway_command(&environment)
            .stdout(Stdio::null())
            .stderr(Stdio::inherit())
            .spawn()
            .unwrap_or_else(|error| panic!("gateway must start: {error}"));
        let mut gateway = Self { child, port };
        for _ in 0..200 {
            if let Some(status) = gateway.child.try_wait().expect("gateway process status") {
                panic!("gateway refused start-up without the kernel: {status}");
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
            json!({"jsonrpc": "2.0", "id": "router-surface", "method": method, "params": params});
        let (status, body) = self
            .exchange("POST", "/rpc", call.to_string().as_bytes())
            .unwrap_or_else(|error| panic!("RPC exchange: {error}"));
        assert_eq!(status, 200, "{}", String::from_utf8_lossy(&body));
        let answer: Value =
            serde_json::from_slice(&body).unwrap_or_else(|error| panic!("RPC response: {error}"));
        assert_eq!(answer["jsonrpc"], "2.0");
        assert_eq!(answer["id"], "router-surface");
        answer
    }
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn precompile_answers() -> Vec<(String, String)> {
    let vectors: Value = serde_json::from_str(include_str!("fixtures/paxeer-abi-vectors.json"))
        .expect("ABI vectors must be JSON");
    let name = format!("agent:did:layerx:{}:main", "61".repeat(32));
    let account = layerx_types::account::AccountId::parse(&name).expect("main account");
    let account =
        hex(&layerx_wire::hash::account_id_for_protocol(&account, 3).expect("account ID"));
    let mut answers: Vec<_> = vectors["cases"]
        .as_array()
        .expect("ABI cases")
        .iter()
        .filter_map(|case| {
            let result = case["result"].as_str()?;
            let result = if case["name"] == "addr.getUnifiedAccount" {
                result.replace(&"7c".repeat(32), &account)
            } else {
                result.to_owned()
            };
            Some((
                format!("0x{}", case["calldata"].as_str()?),
                format!("0x{result}"),
            ))
        })
        .collect();
    for (calldata, words) in [
        ("0x8162486b".to_owned(), 1),
        ("0xb187bd26".to_owned(), 1),
        ("0x8eec5d70".to_owned(), 1),
        (format!("0xd7778749{}", "00".repeat(32)), 1),
        ("0xbbe2e42d".to_owned(), 9),
    ] {
        answers.push((calldata, format!("0x{}", "00".repeat(words * 32))));
    }
    answers
}

fn chain_answer(answers: &[(String, String)], request: &IncomingRequest) -> http::Response {
    assert_eq!(request.method, "POST");
    assert!(matches!(request.path.as_str(), "/chain" | "/secondary"));
    let call: Value = serde_json::from_slice(&request.body).expect("chain JSON-RPC request");
    let result = match call["method"].as_str().expect("chain method") {
        "eth_chainId" => json!("0x7d"),
        "eth_blockNumber" => json!(format!("0x{CHAIN_HEIGHT:x}")),
        "eth_getBalance" => json!("0x64"),
        "eth_getTransactionCount" => json!("0x7"),
        "eth_getCode" => json!("0x"),
        "pax_sequencerReceipt" => Value::Null,
        "eth_call" => {
            let data = call["params"][0]["data"].as_str().expect("ABI calldata");
            json!(
                answers
                    .iter()
                    .find(|(calldata, _)| calldata == data)
                    .unwrap_or_else(|| panic!("undeclared chain calldata: {data}"))
                    .1
            )
        }
        method => panic!("unexpected chain request: {method}"),
    };
    http::json(
        200,
        &json!({"jsonrpc": "2.0", "id": call["id"], "result": result}),
    )
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

fn sequence_transaction(height: u64) -> Vec<u8> {
    let mut message = protobuf_field(1, b"/paxeer.sequencer.v1.MsgSequenceBatch");
    message.extend(protobuf_field(2, &protobuf_field(1, &height.to_be_bytes())));
    let body = protobuf_field(1, &message);
    let mut transaction = protobuf_field(1, &body);
    transaction.extend(protobuf_field(2, &[]));
    transaction
}

fn transaction_leaf(transaction: &[u8]) -> Vec<u8> {
    Sha256::digest([&[0][..], Sha256::digest(transaction).as_slice()].concat()).to_vec()
}

fn signed_header(height: u64) -> Value {
    json!({
        "header": {"chain_id": "router-surface", "height": height.to_string()},
        "commit": {"height": height.to_string(), "round": 0, "signatures": []}
    })
}

fn validator(height: u64) -> Value {
    json!({
        "address": "AB".repeat(20),
        "pub_key": {"type": "tendermint/PubKeyEd25519",
            "value": layerx_platform_internal::base64::encode(&[0x42; 32])},
        "voting_power": height.to_string(), "proposer_priority": "0"
    })
}

fn consensus_answer(request: &IncomingRequest) -> http::Response {
    assert_eq!(request.method, "POST");
    assert_eq!(request.path, "/consensus");
    let call: Value = serde_json::from_slice(&request.body).expect("consensus JSON-RPC request");
    assert_eq!(call["jsonrpc"], "2.0");
    let height = call["params"]["height"]
        .as_str()
        .expect("requested height")
        .parse::<u64>()
        .expect("decimal height");
    assert!(
        [5, 23].contains(&height),
        "unexpected consensus height: {call}"
    );
    let result = match call["method"].as_str().expect("consensus method") {
        "commit" => {
            assert_eq!(call["params"], json!({"height": height.to_string()}));
            json!({"signed_header": signed_header(height), "canonical": true})
        }
        "validators" => {
            assert_eq!(
                call["params"],
                json!({"height": height.to_string(), "page": "1", "per_page": "100"})
            );
            json!({"block_height": height.to_string(), "validators": [validator(height)], "count": "1", "total": "1"})
        }
        "block" => {
            assert_eq!(call["params"], json!({"height": height.to_string()}));
            let transaction = sequence_transaction(height);
            json!({"block": {
                "header": {"height": height.to_string(), "data_hash": hex(&transaction_leaf(&transaction))},
                "data": {"txs": [layerx_platform_internal::base64::encode(&transaction)]}
            }})
        }
        method => panic!("unexpected consensus request: {method}"),
    };
    http::json(
        200,
        &json!({"jsonrpc": "2.0", "id": call["id"], "result": result}),
    )
}

fn method_params(method: &str, height: u64) -> Value {
    match method {
        "pax_sequencerReceipt" => json!([format!("0x{}", "ab".repeat(32))]),
        "px_getAccount" | "px_getBalances" | "px_resolveAccount" => json!([ACCOUNT]),
        "px_getCapabilities"
        | "px_getNetwork"
        | "px_getReceiptKeys"
        | "px_getRouteCatalogue"
        | "px_listAssets" => json!([]),
        "px_getCommit" | "px_getValidators" | "px_getSequenceBatch" => json!([height.to_string()]),
        "px_getHistory" | "px_getUnifiedHistory" => json!([ACCOUNT, null, 10, "native_transfer"]),
        "px_listHolderRecords" => json!([ACCOUNT, "budget", null, 10]),
        "px_requestFunds" => {
            let key = ed25519_dalek::SigningKey::from_bytes(&[0x42; 32]);
            let key = hex(&key.verifying_key().to_bytes());
            json!([format!("did:layerx:{key}"), key])
        }
        "px_register" => {
            use ed25519_dalek::Signer as _;
            let key = ed25519_dalek::SigningKey::from_bytes(&[0x42; 32]);
            let public_key = key.verifying_key().to_bytes();
            let mut binding = Sha256::new();
            for part in [
                b"layerx-register-binding-v1".as_slice(),
                b"beta",
                &public_key,
            ] {
                binding.update((part.len() as u64).to_be_bytes());
                binding.update(part);
            }
            json!([
                hex(&public_key),
                hex(&key.sign(&binding.finalize()).to_bytes())
            ])
        }
        _ => panic!("router method has no OpenRPC parameter case: {method}"),
    }
}

fn assert_surface_answer(method: &str, entry: &Value, answer: &Value) {
    assert_ne!(answer["error"]["code"], -32601, "{method}: {answer}");
    assert_ne!(
        answer["error"]["code"], -32602,
        "valid OpenRPC params must be accepted by {method}: {answer}"
    );
    assert_ne!(
        answer.get("result").is_some(),
        answer.get("error").is_some(),
        "{method}: {answer}"
    );
    if let Some(error) = answer.get("error") {
        assert!(error["code"].as_i64().is_some(), "{method}: {answer}");
        assert!(
            error["message"]
                .as_str()
                .is_some_and(|message| !message.is_empty()),
            "{method}: {answer}"
        );
        if method == "px_getRouteCatalogue" {
            assert_eq!(
                error,
                &json!({"code": -32002, "message": "Authentication required"})
            );
        } else {
            assert!(
                entry["errors"]
                    .as_array()
                    .expect("declared refusals")
                    .iter()
                    .any(|declared| declared["code"] == error["code"]
                        && declared["message"] == error["message"]),
                "{method} must answer a declared named refusal: {answer}"
            );
        }
    }
}

fn assert_consensus_result(method: &str, height: u64, answer: &Value) {
    assert!(
        answer.get("error").is_none(),
        "{method} at {height}: {answer}"
    );
    let result = &answer["result"];
    assert_eq!(result["height"], height.to_string(), "{method}: {answer}");
    match method {
        "px_getCommit" => assert_eq!(result["signedHeader"], signed_header(height)),
        "px_getValidators" => {
            let validator = validator(height);
            assert_eq!(result["total"], "1");
            assert_eq!(
                result["validators"],
                json!([{
                    "address": validator["address"], "pubKey": validator["pub_key"],
                    "votingPower": validator["voting_power"]
                }])
            );
        }
        "px_getSequenceBatch" => {
            let transaction = sequence_transaction(height);
            assert_eq!(result["tx"], format!("0x{}", hex(&transaction)));
            assert_eq!(
                result["proof"],
                json!({"total": 1, "index": 0,
                "leafHash": format!("0x{}", hex(&transaction_leaf(&transaction))), "aunts": []})
            );
        }
        _ => panic!("not a consensus method: {method}"),
    }
}

#[test]
fn router_answers_every_listed_method() {
    let fixture = Fixture::start();
    let answers = precompile_answers();
    let chain = Listener::start(Arc::clone(&fixture.tls), move |request| {
        chain_answer(&answers, request)
    });
    let consensus_calls = Arc::new(Mutex::new(Vec::new()));
    let calls = Arc::clone(&consensus_calls);
    let consensus = Listener::start(Arc::clone(&fixture.tls), move |request| {
        let call: Value = serde_json::from_slice(&request.body).expect("consensus request JSON");
        calls.lock().expect("consensus calls").push(call);
        consensus_answer(request)
    });
    let indexer_paths = Arc::new(Mutex::new(Vec::new()));
    let paths = Arc::clone(&indexer_paths);
    let store = layerx_indexer::store::Store::open_in_memory().expect("real indexer database");
    let indexer = Listener::start(Arc::clone(&fixture.tls), move |request| {
        let (path, _) = layerx_platform_gateway::http::split_target(&request.path)
            .expect("indexer request target");
        paths.lock().expect("indexer paths").push(path.to_owned());
        layerx_indexer::api::route(&store, &request.method, &request.path)
    });
    let gateway = Gateway::start(&fixture, &chain, &consensus, &indexer);
    let document: Value =
        serde_json::from_str(include_str!("../openrpc.json")).expect("OpenRPC JSON");
    let entries = document["methods"].as_array().expect("OpenRPC methods");
    let mut exercised = Vec::new();
    for method in ROUTER_METHODS {
        let matching: Vec<_> = entries
            .iter()
            .filter(|entry| entry["name"] == method)
            .collect();
        assert_eq!(matching.len(), 1, "{method} needs one OpenRPC entry");
        let entry = matching[0];
        assert_eq!(entry["paramStructure"], "by-position", "{method}");
        let params = method_params(method, 5);
        let declared = entry["params"].as_array().expect("OpenRPC params");
        assert_eq!(
            params.as_array().expect("positional params").len(),
            declared.len(),
            "{method}"
        );
        let answer = gateway.rpc(method, params);
        assert_surface_answer(method, entry, &answer);
        if method == "px_getCapabilities" {
            assert!(answer.get("error").is_none(), "{method}: {answer}");
            for surface in ["exchange", "bridge", "launchpad", "pay", "programs"] {
                assert_eq!(answer["result"][surface], true, "{method}: {answer}");
            }
            assert_eq!(answer["result"]["rpc_height"], CHAIN_HEIGHT.to_string());
        }
        if matches!(
            method,
            "px_getCommit" | "px_getValidators" | "px_getSequenceBatch"
        ) {
            assert_consensus_result(method, 5, &answer);
        }
        exercised.push(method);
    }
    assert_eq!(exercised.as_slice(), ROUTER_METHODS.as_slice());
    for method in ["px_getCommit", "px_getValidators", "px_getSequenceBatch"] {
        let answer = gateway.rpc(method, method_params(method, 23));
        assert_consensus_result(method, 23, &answer);
    }
    let calls = consensus_calls.lock().expect("recorded consensus calls");
    assert_eq!(
        calls.len(),
        6,
        "every consensus read must reach the one listener: {calls:?}"
    );
    for height in [5, 23] {
        for method in ["commit", "validators", "block"] {
            assert_eq!(
                calls
                    .iter()
                    .filter(|call| call["method"] == method
                        && call["params"]["height"] == height.to_string())
                    .count(),
                1,
                "{method} must reach consensus at {height}: {calls:?}"
            );
        }
    }
    let paths = indexer_paths.lock().expect("recorded indexer paths");
    assert!(
        paths
            .iter()
            .any(|path| path == &format!("/v1/history/{ACCOUNT}")),
        "history listener was not read: {paths:?}"
    );
    assert!(
        paths
            .iter()
            .any(|path| path == &format!("/v1/holders/{ACCOUNT}")),
        "holder listener was not read: {paths:?}"
    );
    drop(gateway);
    drop(indexer);
    drop(consensus);
    drop(chain);
}
