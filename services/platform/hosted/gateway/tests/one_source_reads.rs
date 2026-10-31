include!("../src/main.rs");

use layerx_platform_gateway::evm;
use layerx_platform_internal::http as internal_http;
use layerx_platform_internal::tls::{Origin, Upstream};
use serde_json::{json, Value};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::AtomicBool;
use std::sync::Mutex;

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
    start_gateway_with_indexer(redis, chain_port, None)
}

fn start_gateway_with_indexer(
    redis: &RedisProcess,
    chain_port: u16,
    indexer_port: Option<u16>,
) -> Gateway {
    start_gateway_with_consensus(redis, chain_port, indexer_port, None)
}

fn start_gateway_with_consensus(
    redis: &RedisProcess,
    chain_port: u16,
    indexer_port: Option<u16>,
    consensus_url: Option<&str>,
) -> Gateway {
    let listen = free_port();
    let mut environment = chain_only_environment(redis, chain_port, listen);
    if let Some(url) = consensus_url {
        environment.push((
            "LAYERX_GATEWAY_CONSENSUS_RPC_URL".to_owned(),
            url.to_owned(),
        ));
    }
    if let Some(port) = indexer_port {
        environment.push((
            "LAYERX_GATEWAY_INDEXER_URL".to_owned(),
            format!("http://127.0.0.1:{port}"),
        ));
    }
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
                        if let Ok(request) = internal_http::parse_client_request(&mut stream) {
                            let answer = chain_answer(&state, &request);
                            let _ = internal_http::write_response(&mut stream, &answer);
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
    asset_reads: bool,
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
            asset_reads: false,
        }
    }

    fn take_calls(&mut self) -> Vec<Value> {
        std::mem::take(&mut self.calls)
    }
}

fn chain_answer(
    state: &Mutex<ChainState>,
    request: &internal_http::Request,
) -> internal_http::Response {
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
            if state.asset_reads {
                return asset_chain_answer(&call);
            }
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
    internal_http::json(200, &result.map_or_else(
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

fn abi_word(value: u128) -> Vec<u8> {
    let mut word = vec![0; 16];
    word.extend_from_slice(&value.to_be_bytes());
    word
}

fn abi_bytes(value: &[u8]) -> Vec<u8> {
    let mut bytes = abi_word(value.len() as u128);
    bytes.extend_from_slice(value);
    bytes.resize(32 + value.len().div_ceil(32) * 32, 0);
    bytes
}

fn ledger_asset_answer() -> Vec<u8> {
    let mut answer = vec![0x11; 32];
    answer.extend(abi_word(384));
    answer.extend(abi_word(448));
    answer.extend(abi_word(18));
    answer.extend(abi_word(u128::MAX));
    answer.extend([0x61; 32]);
    answer.extend([0x42; 32]);
    answer.extend(abi_word(1));
    answer.extend(abi_word(2));
    answer.extend(abi_word(512));
    answer.extend(abi_word(0));
    answer.extend(abi_word(u128::MAX - 1));
    answer.extend(abi_bytes(b"LED"));
    answer.extend(abi_bytes(b"Ledger Asset"));
    answer.extend(abi_bytes(&[0xff, 0x00, 0x81]));
    answer
}

fn asset_chain_answer(call: &Value) -> internal_http::Response {
    let to = call["params"][0]["to"].as_str().unwrap();
    let data = call["params"][0]["data"].as_str().unwrap();
    assert_ne!(to, evm::address_hex(&evm::CUSTODY_PRECOMPILE));
    assert_eq!(call["params"][1], LATEST_BLOCK);
    let answer = if to == evm::address_hex(&evm::LEDGER_PRECOMPILE) {
        let calldata = |byte| format!("0x2cc3ce80{}", hex(&[byte; 32]));
        if data == calldata(0x11) {
            Some(ledger_asset_answer())
        } else if data == calldata(0x22) {
            None
        } else if data == calldata(0x33) {
            Some(vec![0; 32])
        } else if data == calldata(0x44) {
            Some(ledger_asset_answer())
        } else {
            panic!("unexpected ledger read: {call}");
        }
    } else {
        assert_eq!(to, evm::address_hex(&evm::BANK_PRECOMPILE));
        // Independent ABI calldata for a single dynamic string argument.
        let calldata = |selector: &str, denom: &[u8]| {
            format!(
                "0x{selector}{}{}",
                hex(&abi_word(32)),
                hex(&abi_bytes(denom))
            )
        };
        if data == calldata("5b43bc99", b"ubroken") {
            None
        } else if data == calldata("5b43bc99", b"ubank") {
            Some([abi_word(32), abi_bytes(b"Bank Asset")].concat())
        } else if data == calldata("41bb0559", b"ubank") {
            Some([abi_word(32), abi_bytes(b"BANK")].concat())
        } else if data == calldata("3b2b3204", b"ubank") {
            Some(abi_word(6))
        } else if data == calldata("6923a1fd", b"ubank") {
            Some(vec![0xff; 32])
        } else {
            panic!("unexpected bank read: {call}");
        }
    };
    internal_http::json(
        200,
        &match answer {
            Some(bytes) => {
                json!({"jsonrpc":"2.0", "id":call["id"], "result":format!("0x{}", hex(&bytes))})
            }
            None => {
                json!({"jsonrpc":"2.0", "id":call["id"], "error":{"code":-32000, "message":"execution reverted"}})
            }
        },
    )
}

#[test]
fn px_list_assets_joins_indexer_assets_with_chain_metadata() {
    use layerx_indexer::store::{AssetRow, Store};

    let _ = rustls::crypto::ring::default_provider().install_default();
    let abi: Value = serde_json::from_str(include_str!(
        "../../../../../precompiles/layerxledger/abi.json"
    ))
    .unwrap();
    let get_asset = abi
        .as_array()
        .unwrap()
        .iter()
        .find(|entry| entry["name"] == "getAsset")
        .unwrap();
    let outputs: Vec<_> = get_asset["outputs"]
        .as_array()
        .unwrap()
        .iter()
        .map(|output| output["type"].as_str().unwrap())
        .collect();
    assert_eq!(
        outputs,
        [
            "bytes32", "string", "string", "uint8", "uint128", "bytes32", "bytes32", "uint8",
            "uint8", "bytes", "bool", "uint128"
        ]
    );
    assert_eq!(
        evm::SELECTOR_GET_ASSET,
        layerx_indexer::abi::keccak(b"getAsset(bytes32)")[..4]
    );
    for (signature, selector) in [
        ("name(string)", evm::SELECTOR_BANK_NAME),
        ("symbol(string)", evm::SELECTOR_BANK_SYMBOL),
        ("decimals(string)", evm::SELECTOR_BANK_DECIMALS),
        ("supply(string)", evm::SELECTOR_BANK_SUPPLY),
    ] {
        assert_eq!(
            selector,
            layerx_indexer::abi::keccak(signature.as_bytes())[..4]
        );
    }
    let bytes = ledger_asset_answer();
    let decoded = evm::decode_ledger_asset(&bytes).unwrap();
    assert_eq!(decoded.asset_id, [0x11; 32]);
    assert_eq!(decoded.name, "Ledger Asset");
    assert_eq!(decoded.symbol, "LED");
    assert_eq!(decoded.decimals, 18);
    assert_eq!(decoded.supply_cap, u128::MAX);
    assert_eq!(decoded.total_units, u128::MAX - 1);
    assert_eq!(decoded.custody_reference, [0xff, 0x00, 0x81]);
    for (offset, value) in [
        (3 * 32, 1),
        (4 * 32, 1),
        (7 * 32, 1),
        (8 * 32, 1),
        (10 * 32 + 31, 2),
        (11 * 32, 1),
    ] {
        let mut malformed = bytes.clone();
        malformed[offset] = value;
        assert_eq!(
            evm::decode_ledger_asset(&malformed),
            Err(evm::AbiError::Width)
        );
    }
    let mut malformed = bytes.clone();
    malformed[63] = 32;
    malformed[62] = 0;
    assert_eq!(
        evm::decode_ledger_asset(&malformed),
        Err(evm::AbiError::Offset)
    );
    assert!(evm::decode_ledger_asset(&bytes[..383]).is_err());
    assert!(evm::decode_ledger_asset(&bytes[..bytes.len() - 1]).is_err());

    let store = Store::open_in_memory().unwrap();
    let mut rows = Vec::new();
    // The first sorted asset is registered beyond the first indexer page.
    for number in (0..501).rev() {
        rows.push(AssetRow {
            asset: format!("z{number:04}"),
            chain: "paxeer".to_owned(),
            kind: "erc20".to_owned(),
            address: None,
            denom: None,
            metadata: json!({}),
        });
    }
    let mut expected = Vec::new();
    for byte in [0x44, 0x33, 0x22, 0x11] {
        let asset_id = hex(&[byte; 32]);
        rows.push(AssetRow { asset:asset_id.clone(), chain:"paxeer".to_owned(),
            kind:"layerx".to_owned(), address:None, denom:None,
            metadata:json!({"name":"stale indexer name", "symbol":"STALE", "decimals":2,"supply":"7"}) });
        expected.push(if byte == 0x11 {
            json!({"asset_id":asset_id,"denom":null,"kind":"layerx","name":"Ledger Asset","symbol":"LED","decimals":18,"supply":(u128::MAX - 1).to_string()})
        } else {
            json!({"asset_id":asset_id,"denom":null,"kind":"layerx","name":null,"symbol":null,"decimals":null,"supply":null})
        });
    }
    for (denom, kind) in [("ubank", "bank"), ("ubroken", "token_factory")] {
        let asset_id = format!("denom:{denom}");
        rows.push(AssetRow {
            asset: asset_id.clone(),
            chain: "paxeer".to_owned(),
            kind: kind.to_owned(),
            address: None,
            denom: Some(denom.to_owned()),
            metadata: json!({}),
        });
        expected.push(if denom == "ubank" {
            json!({"asset_id":asset_id,"denom":denom,"kind":kind,"name":"Bank Asset","symbol":"BANK","decimals":6,"supply":"115792089237316195423570985008687907853269984665640564039457584007913129639935"})
        } else {
            json!({"asset_id":asset_id,"denom":denom,"kind":kind,"name":null,"symbol":null,"decimals":null,"supply":null})
        });
    }
    for number in 0..10 {
        expected.push(json!({"asset_id":format!("z{number:04}"),"denom":null,"kind":"erc20","name":null,"symbol":null,"decimals":null,"supply":null}));
    }
    expected.sort_by(|a, b| a["asset_id"].as_str().cmp(&b["asset_id"].as_str()));
    store.register_assets(&rows).unwrap();
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let indexer_address = listener.local_addr().unwrap();
    let store = Arc::new(store);
    thread::spawn(move || layerx_indexer::api::serve(&listener, &store, None));
    let redis = RedisProcess::start();
    assert!(redis.store().ready());
    let state = Arc::new(Mutex::new(ChainState::new()));
    state.lock().unwrap().asset_reads = true;
    let chain = ChainListener::start(tls(&redis), Arc::clone(&state));
    let gateway = start_gateway_with_indexer(&redis, chain.port, Some(indexer_address.port()));
    let answer = gateway.rpc("px_listAssets", &json!([]));
    assert!(answer.get("error").is_none(), "{answer}");
    assert_eq!(
        answer["result"],
        json!({"height":16,"assets":expected,"joined_limit":16})
    );
    let calls = state.lock().unwrap().take_calls();
    assert_eq!(
        calls
            .iter()
            .filter(|call| call["method"] == "eth_blockNumber")
            .count(),
        1
    );
    let asset_calls: Vec<_> = calls
        .iter()
        .filter(|call| call["method"] == "eth_call")
        .collect();
    assert_eq!(asset_calls.len(), 9);
    for call in asset_calls {
        assert_eq!(call["params"][1], LATEST_BLOCK);
        assert!(
            matches!(
                call["params"][0]["to"].as_str(),
                Some(
                    "0x0000000000000000000000000000000000001001"
                        | "0x000000000000000000000000000000000000101a"
                )
            ),
            "{call}"
        );
    }
    let invalid = gateway.rpc("px_listAssets", &json!(["unexpected"]));
    assert_eq!(invalid["error"]["code"], -32602);
    assert!(state.lock().unwrap().take_calls().is_empty());
}

#[test]
fn px_get_network_reports_the_consensus_head() {
    let block_hash = "A1".repeat(32);
    let app_hash = "B2".repeat(32);
    for (height, catching_up) in [("34742403", false), ("9007199254740993", true)] {
        let status = json!({
            "sync_info": {
                "latest_block_height": height,
                "latest_block_hash": block_hash,
                "latest_app_hash": app_hash,
                "catching_up": catching_up,
                "earliest_block_height": "1"
            }
        });
        let head = chain_reads::ConsensusHead::from_status(&status)
            .expect("the consensus status sync_info must decode");
        let result = paxeer::network_result("paxeer-degraded", CHAIN_ID, LATEST_BLOCK, Some(head));
        assert_eq!(
            result,
            json!({
                "network_id": "paxeer-degraded",
                "paxeer": {"chain_id": CHAIN_ID, "latest_block": LATEST_BLOCK},
                "consensus": {
                    "latest_height": height,
                    "latest_block_hash": block_hash,
                    "latest_app_hash": app_hash,
                    "catching_up": catching_up
                }
            })
        );
        for member in ["anchor", "kernel", "layerx", "reason"] {
            assert!(result.get(member).is_none(), "{result}");
        }
        for (field, invalid) in [
            ("latest_block_height", json!(16)),
            ("latest_block_height", json!("-1")),
            ("latest_block_height", json!("18446744073709551616")),
            ("latest_block_hash", json!("not-a-hash")),
            ("latest_app_hash", json!(null)),
            ("catching_up", json!("false")),
        ] {
            let mut malformed = status.clone();
            malformed["sync_info"][field] = invalid;
            assert!(
                chain_reads::ConsensusHead::from_status(&malformed).is_none(),
                "{malformed}"
            );
        }
        for field in [
            "latest_block_height",
            "latest_block_hash",
            "latest_app_hash",
            "catching_up",
        ] {
            let mut incomplete = status.clone();
            incomplete["sync_info"]
                .as_object_mut()
                .unwrap()
                .remove(field);
            assert!(
                chain_reads::ConsensusHead::from_status(&incomplete).is_none(),
                "{incomplete}"
            );
        }
    }

    let _ = rustls::crypto::ring::default_provider().install_default();
    let redis = RedisProcess::start();
    assert!(redis.store().ready());
    let state = Arc::new(Mutex::new(ChainState::new()));
    let chain = ChainListener::start(tls(&redis), Arc::clone(&state));
    let unavailable_listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let unavailable_port = unavailable_listener.local_addr().unwrap().port();
    drop(unavailable_listener);
    let unavailable_url = format!("https://localhost:{unavailable_port}/consensus");
    for consensus_url in [None, Some(unavailable_url.as_str())] {
        let gateway = start_gateway_with_consensus(&redis, chain.port, None, consensus_url);
        state.lock().unwrap().take_calls();
        let answer = gateway.rpc("px_getNetwork", &json!([]));
        assert!(answer.get("error").is_none(), "{answer}");
        assert_eq!(
            answer["result"],
            json!({
                "network_id": "paxeer-degraded",
                "paxeer": {"chain_id": CHAIN_ID, "latest_block": LATEST_BLOCK},
                "consensus": null,
                "reason": "consensus_unavailable"
            })
        );
        let calls = state.lock().unwrap().take_calls();
        let mut exchanges = calls.chunks_exact(2);
        let methods: Vec<_> = exchanges
            .by_ref()
            .map(|exchange| {
                assert_eq!(exchange[0], json!({
                    "jsonrpc": "2.0", "id": "route-network", "method": "eth_chainId", "params": []
                }));
                assert_eq!(exchange[1]["id"], "px-read");
                assert_eq!(exchange[1]["params"], json!([]));
                exchange[1]["method"].as_str().unwrap()
            })
            .collect();
        assert!(exchanges.remainder().is_empty());
        assert_eq!(methods, ["eth_chainId", "eth_blockNumber"]);
        let invalid = gateway.rpc("px_getNetwork", &json!(["unexpected"]));
        assert_eq!(invalid["error"]["code"], -32602);
        assert!(state.lock().unwrap().take_calls().is_empty());
    }

    let document: Value = serde_json::from_str(include_str!("../openrpc.json")).unwrap();
    let method = document["methods"]
        .as_array()
        .unwrap()
        .iter()
        .find(|method| method["name"] == "px_getNetwork")
        .unwrap();
    let schema = &method["result"]["schema"];
    assert_eq!(schema["additionalProperties"], false);
    assert_eq!(
        schema["required"],
        json!(["network_id", "paxeer", "consensus"])
    );
    assert_eq!(
        schema["properties"]["consensus"]["required"],
        json!([
            "latest_height",
            "latest_block_hash",
            "latest_app_hash",
            "catching_up"
        ])
    );
    for member in ["anchor", "kernel", "layerx"] {
        assert!(schema["properties"].get(member).is_none());
    }
}
