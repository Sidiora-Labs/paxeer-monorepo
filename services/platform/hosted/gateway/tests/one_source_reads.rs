use layerx_platform_gateway::evm;
use layerx_platform_gateway::store::{RedisEndpoint, RedisStore};
use layerx_platform_internal::http;
use layerx_platform_internal::tls::{Origin, Upstream};
use native_tls::Certificate;
use rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::{ServerConfig, ServerConnection, StreamOwned};
use serde_json::{json, Value};
use std::fs;
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use zeroize::Zeroizing;

const CHAIN_ID: &str = "0x7d";
const LATEST_BLOCK: &str = "0x10";
const ACCOUNT: &str = "0x102132435465768798a9bacbdcedfe0f1e2d3c4b";

struct RedisProcess {
    child: Child,
    directory: PathBuf,
    endpoint: RedisEndpoint,
    certificate: Certificate,
    port: u16,
}

impl RedisProcess {
    fn start() -> Self {
        let unique = format!(
            "layerx-one-source-redis-{}-{}",
            std::process::id(),
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap_or_default()
                .as_nanos()
        );
        let directory = std::env::temp_dir().join(unique);
        fs::create_dir(&directory)
            .unwrap_or_else(|error| panic!("test Redis directory must be created: {error}"));
        let certificate_pem = directory.join("server.pem");
        let certificate_der = directory.join("server.der");
        let private_key = directory.join("server.key");
        command(
            "openssl",
            &[
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-keyout",
                path(&private_key),
                "-out",
                path(&certificate_pem),
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
                path(&certificate_pem),
                "-outform",
                "DER",
                "-out",
                path(&certificate_der),
            ],
        );
        let port = free_port();
        let acl = directory.join("users.acl");
        fs::write(
            &acl,
            "user default off\nuser gateway on >gateway-secret ~* &* +@all\n",
        )
        .unwrap_or_else(|error| panic!("test Redis ACL must be written: {error}"));
        let config = directory.join("redis.conf");
        fs::write(
            &config,
            format!(
                "bind 127.0.0.1\nport 0\ntls-port {port}\ntls-cert-file {}\ntls-key-file {}\ntls-ca-cert-file {}\ntls-auth-clients no\naclfile {}\nappendonly yes\nappendfsync always\ndir {}\nprotected-mode yes\n",
                path(&certificate_pem),
                path(&private_key),
                path(&certificate_pem),
                path(&acl),
                path(&directory),
            ),
        )
        .unwrap_or_else(|error| panic!("test Redis config must be written: {error}"));
        let endpoint = RedisEndpoint::parse(&format!("rediss://localhost:{port}"))
            .unwrap_or_else(|error| panic!("test Redis endpoint must parse: {error}"));
        let certificate = Certificate::from_der(
            &fs::read(&certificate_der)
                .unwrap_or_else(|error| panic!("test certificate must be read: {error}")),
        )
        .unwrap_or_else(|error| panic!("test certificate must parse: {error}"));
        let child = Command::new("redis-server")
            .arg(&config)
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .unwrap_or_else(|error| panic!("real Redis server must start: {error}"));
        let process = Self {
            child,
            directory,
            endpoint,
            certificate,
            port,
        };
        for _ in 0..100 {
            if TcpStream::connect(("127.0.0.1", port)).is_ok() {
                return process;
            }
            thread::sleep(Duration::from_millis(20));
        }
        panic!("real Redis server did not become reachable")
    }

    fn store(&self) -> RedisStore {
        RedisStore::new(
            self.endpoint.clone(),
            self.certificate.clone(),
            Zeroizing::new("gateway".to_owned()),
            Zeroizing::new("gateway-secret".to_owned()),
        )
    }
}

impl Drop for RedisProcess {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
        let _ = fs::remove_dir_all(&self.directory);
    }
}

struct Gateway {
    child: Child,
    client: Upstream,
}

impl Drop for Gateway {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

impl Gateway {
    fn rpc(&self, method: &str, params: &Value) -> Value {
        let request = json!({"jsonrpc": "2.0", "id": 7, "method": method, "params": params});
        let answer = self
            .client
            .post("/rpc", request.to_string().as_bytes())
            .unwrap_or_else(|error| panic!("{method} must be answered: {error:?}"));
        assert_eq!(answer.status, 200, "{method}");
        let document: Value = serde_json::from_slice(&answer.body)
            .unwrap_or_else(|error| panic!("{method} answer must be JSON: {error}"));
        assert_eq!(document["jsonrpc"], "2.0");
        assert_eq!(document["id"], 7);
        document
    }

    fn get(&self, path: &str) -> (u16, Value) {
        let answer = self
            .client
            .get(path)
            .unwrap_or_else(|error| panic!("{path} must be answered: {error:?}"));
        let document = serde_json::from_slice(&answer.body)
            .unwrap_or_else(|error| panic!("{path} answer must be JSON: {error}"));
        (answer.status, document)
    }
}

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
    let listener = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("test port must be allocated: {error}"));
    listener
        .local_addr()
        .unwrap_or_else(|error| panic!("test port must resolve: {error}"))
        .port()
}

fn secret(directory: &Path, name: &str, value: &str) -> String {
    let file = directory.join(name);
    fs::write(&file, value).unwrap_or_else(|error| panic!("{name} must be written: {error}"));
    path(&file).to_owned()
}

fn chain_only_environment(
    redis: &RedisProcess,
    chain_port: u16,
    listen: u16,
) -> Vec<(String, String)> {
    let directory = &redis.directory;
    let unused = free_port();
    let ca = path(&directory.join("server.der")).to_owned();
    vec![
        (
            "LAYERX_GATEWAY_LISTEN".to_owned(),
            format!("127.0.0.1:{listen}"),
        ),
        ("LAYERX_GATEWAY_TLS_CERT_DER".to_owned(), ca.clone()),
        (
            "LAYERX_GATEWAY_TLS_KEY_DER".to_owned(),
            path(&directory.join("server-key.der")).to_owned(),
        ),
        ("LAYERX_GATEWAY_OUTBOUND_CA_DER".to_owned(), ca.clone()),
        (
            "LAYERX_GATEWAY_NETWORK_ID".to_owned(),
            "paxeer-degraded".to_owned(),
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
            "LAYERX_GATEWAY_PAXEER_RPC_URLS".to_owned(),
            json!([
                format!("https://localhost:{chain_port}/rpc"),
                format!("https://localhost:{unused}/rpc")
            ])
            .to_string(),
        ),
        (
            "LAYERX_GATEWAY_REDIS_URL".to_owned(),
            format!("rediss://localhost:{}", redis.port),
        ),
        (
            "LAYERX_GATEWAY_REDIS_USERNAME_FILE".to_owned(),
            secret(directory, "redis-username", "gateway"),
        ),
        (
            "LAYERX_GATEWAY_REDIS_PASSWORD_FILE".to_owned(),
            secret(directory, "redis-password", "gateway-secret"),
        ),
    ]
}

fn gateway_command(environment: &[(String, String)]) -> Command {
    let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-gateway"));
    command
        .env_clear()
        .env("PATH", std::env::var_os("PATH").unwrap_or_default())
        .envs(environment.iter().map(|(name, value)| (name, value)));
    command
}

fn start_gateway(redis: &RedisProcess, chain_port: u16) -> Gateway {
    let listen = free_port();
    let environment = chain_only_environment(redis, chain_port, listen);
    assert!(environment.iter().all(|(name, _)| {
        name != "LAYERX_GATEWAY_COMPONENT_URL"
            && name != "LAYERX_GATEWAY_PUBLIC_CORE_URL"
            && !name.starts_with("LAYERX_EVENTS_")
    }));
    let child = gateway_command(&environment)
        .stdout(Stdio::null())
        .stderr(Stdio::inherit())
        .spawn()
        .unwrap_or_else(|error| panic!("gateway must start: {error}"));
    let mut gateway = Gateway {
        child,
        client: Upstream::new(
            Origin::parse(&format!("https://localhost:{listen}"))
                .unwrap_or_else(|error| panic!("{error}")),
            redis.certificate.clone(),
            None,
            None,
        ),
    };
    for _ in 0..200 {
        if let Ok(Some(status)) = gateway.child.try_wait() {
            panic!("gateway refused start-up without the kernel: {status}");
        }
        if gateway
            .client
            .get("/livez")
            .is_ok_and(|answer| answer.status == 200)
        {
            return gateway;
        }
        thread::sleep(Duration::from_millis(50));
    }
    panic!("gateway did not become live")
}

fn tls(redis: &RedisProcess) -> Arc<ServerConfig> {
    command(
        "openssl",
        &[
            "pkcs8",
            "-topk8",
            "-nocrypt",
            "-in",
            path(&redis.directory.join("server.key")),
            "-outform",
            "DER",
            "-out",
            path(&redis.directory.join("server-key.der")),
        ],
    );
    let certificate = CertificateDer::from(fs::read(redis.directory.join("server.der")).unwrap());
    let key = PrivateKeyDer::from(PrivatePkcs8KeyDer::from(
        fs::read(redis.directory.join("server-key.der")).unwrap(),
    ));
    Arc::new(
        ServerConfig::builder()
            .with_no_client_auth()
            .with_single_cert(vec![certificate], key)
            .unwrap(),
    )
}

struct ChainListener {
    port: u16,
    stop: Arc<AtomicBool>,
    join: Option<thread::JoinHandle<()>>,
}

impl ChainListener {
    fn start(tls: Arc<ServerConfig>, state: Arc<Mutex<ChainState>>) -> Self {
        let listener = TcpListener::bind(("127.0.0.1", 0)).unwrap();
        let port = listener.local_addr().unwrap().port();
        listener.set_nonblocking(true).unwrap();
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let join = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((tcp, _)) => {
                        tcp.set_read_timeout(Some(Duration::from_secs(3))).unwrap();
                        tcp.set_write_timeout(Some(Duration::from_secs(3))).unwrap();
                        let connection = ServerConnection::new(Arc::clone(&tls)).unwrap();
                        let mut stream = StreamOwned::new(connection, tcp);
                        if let Ok(request) = http::parse_client_request(&mut stream) {
                            let answer = chain_answer(&state, &request);
                            let _ = http::write_response(&mut stream, &answer);
                        }
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(5));
                    }
                    Err(error) => panic!("chain listener: {error}"),
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

impl Drop for ChainListener {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        assert!(self.join.take().unwrap().join().is_ok());
    }
}

fn vector(name: &str) -> Value {
    let vectors: Value =
        serde_json::from_str(include_str!("fixtures/paxeer-abi-vectors.json")).unwrap();
    vectors["cases"]
        .as_array()
        .unwrap()
        .iter()
        .find(|case| case["name"] == name)
        .unwrap()
        .clone()
}

fn main_account() -> String {
    let name = format!("agent:did:layerx:{}:main", "61".repeat(32));
    let account = layerx_types::account::AccountId::parse(&name).unwrap();
    hex(&layerx_wire::hash::account_id_for_protocol(&account, 3).unwrap())
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn account_document() -> Value {
    json!({
        "evm_address": ACCOUNT,
        "pax_address": "pax1exampleaccount",
        "layerx_did": format!("did:layerx:{}", "61".repeat(32)),
        "layerx_account": main_account(),
        "bound": true
    })
}

struct ChainState {
    calls: Vec<Value>,
    bank_answer: String,
    bank_refused: bool,
}

impl ChainState {
    fn new() -> Self {
        Self {
            calls: Vec::new(),
            bank_answer: format!(
                "0x{}",
                vector("bank.all_balances")["result"].as_str().unwrap()
            ),
            bank_refused: false,
        }
    }

    fn take_calls(&mut self) -> Vec<Value> {
        std::mem::take(&mut self.calls)
    }
}

fn chain_answer(state: &Mutex<ChainState>, request: &http::Request) -> http::Response {
    assert_eq!(request.path, "/rpc");
    let call: Value = serde_json::from_slice(&request.body).unwrap();
    let mut state = state.lock().unwrap();
    state.calls.push(call.clone());
    let result = match call["method"].as_str().unwrap() {
        "eth_chainId" => Some(json!(CHAIN_ID)),
        "eth_blockNumber" => {
            let count = state
                .calls
                .iter()
                .filter(|call| call["method"] == "eth_blockNumber")
                .count();
            Some(json!(format!("0x{:x}", 0x0f + count)))
        }
        "eth_getBalance" => Some(json!("0x123456789abcdef")),
        "eth_getTransactionCount" => Some(json!("0x2a")),
        "eth_call" => {
            let data = call["params"][0]["data"].as_str().unwrap();
            let to = call["params"][0]["to"].as_str().unwrap();
            let unified = vector("addr.getUnifiedAccount");
            let reverse = vector("addr.getEvmAddrByLayerX");
            let bank = vector("bank.all_balances");
            if to == evm::address_hex(&evm::ADDR_PRECOMPILE)
                && data == format!("0x{}", unified["calldata"].as_str().unwrap())
            {
                Some(json!(format!(
                    "0x{}",
                    unified["result"]
                        .as_str()
                        .unwrap()
                        .replace(&"7c".repeat(32), &main_account())
                )))
            } else if to == evm::address_hex(&evm::ADDR_PRECOMPILE)
                && data == format!("0x{}", reverse["calldata"].as_str().unwrap())
            {
                Some(json!(format!("0x{}", reverse["result"].as_str().unwrap())))
            } else if to == evm::address_hex(&evm::ADDR_PRECOMPILE)
                && data
                    == format!(
                        "0x{}",
                        hex(&evm::calldata_word(
                            evm::SELECTOR_GET_EVM_ADDR_BY_LAYERX,
                            &[0x62; 32]
                        ))
                    )
            {
                Some(json!(format!("0x{}", "00".repeat(32))))
            } else if to == evm::address_hex(&evm::BANK_PRECOMPILE)
                && data == format!("0x{}", bank["calldata"].as_str().unwrap())
            {
                if state.bank_refused {
                    None
                } else {
                    Some(json!(state.bank_answer))
                }
            } else {
                panic!("unexpected chain call: {call}");
            }
        }
        _ => panic!("unexpected chain request: {call}"),
    };
    http::json(200, &result.map_or_else(
        || json!({"jsonrpc":"2.0", "id":call["id"], "error":{"code":-32000,"message":"execution reverted"}}),
        |result| json!({"jsonrpc":"2.0", "id":call["id"], "result":result}),
    ))
}

fn assert_snapshot(calls: &[Value], bank_calls: usize, quantity_reads: usize) {
    assert_eq!(
        calls
            .iter()
            .filter(|call| call["method"] == "eth_blockNumber")
            .count(),
        1,
        "{calls:?}"
    );
    let reads: Vec<_> = calls
        .iter()
        .filter(|call| call["method"] != "eth_chainId")
        .collect();
    assert_eq!(reads.first().unwrap()["method"], "eth_blockNumber");
    let mut bank_count = 0;
    let mut quantity_count = 0;
    for call in reads.into_iter().skip(1) {
        assert_eq!(call["params"][1], LATEST_BLOCK, "{call}");
        if call["method"] == "eth_call" {
            if call["params"][0]["to"] == evm::address_hex(&evm::BANK_PRECOMPILE) {
                bank_count += 1;
                assert_eq!(
                    call["params"][0]["data"],
                    format!(
                        "0x{}",
                        vector("bank.all_balances")["calldata"].as_str().unwrap()
                    )
                );
            } else {
                assert_eq!(
                    call["params"][0]["to"],
                    evm::address_hex(&evm::ADDR_PRECOMPILE)
                );
            }
        } else {
            assert!(
                matches!(
                    call["method"].as_str(),
                    Some("eth_getBalance" | "eth_getTransactionCount")
                ),
                "{call}"
            );
            assert_eq!(call["params"][0], ACCOUNT);
            quantity_count += 1;
        }
    }
    assert_eq!(bank_count, bank_calls);
    assert_eq!(quantity_count, quantity_reads);
}

#[test]
fn px_get_balances_reads_bank_all_balances_at_one_height() {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let redis = RedisProcess::start();
    assert!(redis.store().ready());
    let state = Arc::new(Mutex::new(ChainState::new()));
    let chain = ChainListener::start(tls(&redis), Arc::clone(&state));
    let gateway = start_gateway(&redis, chain.port);
    let expected = json!({"account":account_document(), "height":16,
        "balances":vector("bank.all_balances")["out"]["response"]});
    for selector in [
        ACCOUNT.to_owned(),
        format!("did:layerx:{}", "61".repeat(32)),
    ] {
        let answer = gateway.rpc("px_getBalances", &json!([selector]));
        assert_eq!(answer["result"], expected, "{answer}");
        assert!(answer.get("error").is_none(), "{answer}");
        assert_snapshot(&state.lock().unwrap().take_calls(), 1, 0);
    }
    state.lock().unwrap().bank_answer = format!("0x{:064x}{:064x}", 32, 0);
    let empty = gateway.rpc("px_getBalances", &json!([ACCOUNT]));
    assert_eq!(
        empty["result"],
        json!({"account":account_document(), "height":16, "balances":[]})
    );
    assert_snapshot(&state.lock().unwrap().take_calls(), 1, 0);

    state.lock().unwrap().bank_answer = "0x00".to_owned();
    let malformed = gateway.rpc("px_getBalances", &json!([ACCOUNT]));
    assert!(malformed.get("result").is_none(), "{malformed}");
    assert_eq!(
        malformed["error"]["data"]["code"],
        "invalid_paxeer_response"
    );
    assert_snapshot(&state.lock().unwrap().take_calls(), 1, 0);

    state.lock().unwrap().bank_refused = true;
    let refused = gateway.rpc("px_getBalances", &json!([ACCOUNT]));
    assert!(refused.get("result").is_none(), "{refused}");
    assert_eq!(refused["error"]["data"]["code"], "paxeer_call_refused");
    assert_snapshot(&state.lock().unwrap().take_calls(), 1, 0);
}

#[test]
fn px_get_account_reads_no_kernel_backend() {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let redis = RedisProcess::start();
    assert!(redis.store().ready());
    let state = Arc::new(Mutex::new(ChainState::new()));
    let chain = ChainListener::start(tls(&redis), Arc::clone(&state));
    let gateway = start_gateway(&redis, chain.port);
    for selector in [
        ACCOUNT.to_owned(),
        format!("did:layerx:{}", "61".repeat(32)),
    ] {
        let answer = gateway.rpc("px_getAccount", &json!([selector]));
        assert_eq!(
            answer["result"],
            json!({
                "account":account_document(),
                "paxeer":{"address":ACCOUNT,"balance":"0x123456789abcdef","nonce":"0x2a","height":16}
            }),
            "{answer}"
        );
        assert!(answer["result"].get("layerx").is_none(), "{answer}");
        assert_snapshot(&state.lock().unwrap().take_calls(), 0, 2);
    }
    let unbound = gateway.rpc(
        "px_getAccount",
        &json!([format!("did:layerx:{}", "62".repeat(32))]),
    );
    assert!(unbound.get("error").is_none(), "{unbound}");
    assert_eq!(unbound["result"]["paxeer"], Value::Null, "{unbound}");
    assert_eq!(unbound["result"]["account"]["evm_address"], Value::Null);
    assert_eq!(unbound["result"]["account"]["bound"], false);
    assert_eq!(
        unbound["result"]["account"]["layerx_did"],
        format!("did:layerx:{}", "62".repeat(32))
    );
    assert!(unbound["result"].get("layerx").is_none(), "{unbound}");
    let calls = state.lock().unwrap().take_calls();
    assert_snapshot(&calls, 0, 0);
    assert_eq!(
        calls
            .iter()
            .filter(|call| call["method"] == "eth_call")
            .count(),
        1
    );
    let (_, readiness) = gateway.get("/readyz");
    assert_eq!(
        readiness["backends"]["public_core"]["reason"], "not_configured",
        "{readiness}"
    );
}
