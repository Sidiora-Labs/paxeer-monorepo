include!("../../src/main.rs");

use layerx_platform_core::SeedSigner;
use layerx_platform_internal::evm_sender::EvmSender;
use serde_json::{json, Value};
use std::sync::atomic::AtomicBool;

const EVM_KEY: &str = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80";
const ADMIN: &str = "repair-22-33-1455-admin";
const RECIPIENT: &str = "0x3939393939393939393939393939393939393939";
const SENDER: &str = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266";
const KEY: &str = "native-recovery";

fn transcripts() -> Value {
    serde_json::from_str(include_str!("../../src/repair_22_31_354_transcripts.json")).unwrap()
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let path = env::temp_dir().join(format!(
            "core-repair-{}-{}",
            std::process::id(),
            TRACE.fetch_add(1, Ordering::Relaxed)
        ));
        fs::DirBuilder::new().mode(0o700).create(&path).unwrap();
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}

fn rpc_document(address: SocketAddr, request: &Value) -> Result<Value, String> {
    let body = serde_json::to_vec(request).map_err(|error| error.to_string())?;
    let mut stream = TcpStream::connect_timeout(&address, Duration::from_secs(2))
        .map_err(|error| error.to_string())?;
    stream
        .set_read_timeout(Some(Duration::from_secs(10)))
        .map_err(|error| error.to_string())?;
    stream
        .set_write_timeout(Some(Duration::from_secs(10)))
        .map_err(|error| error.to_string())?;
    write!(stream, "POST / HTTP/1.1\r\nHost: {address}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).map_err(|error| error.to_string())?;
    stream.write_all(&body).map_err(|error| error.to_string())?;
    let response =
        layerx_platform_internal::http::read_http_message(&mut stream, MAX_REQUEST_BYTES)?;
    serde_json::from_slice(&response.body).map_err(|error| error.to_string())
}
fn rpc(address: SocketAddr, method: &str, params: Value) -> Value {
    let document = rpc_document(
        address,
        &json!({"jsonrpc":"2.0", "id": 1, "method":method,"params":params}),
    )
    .unwrap();
    assert!(document.get("error").is_none(), "{method}: {document}");
    document["result"].clone()
}

struct CoreFixture {
    root: Directory,
    tls: Arc<ServerConfig>,
    certificate: Vec<u8>,
}
impl CoreFixture {
    fn new() -> Self {
        static PROVIDER: std::sync::OnceLock<()> = std::sync::OnceLock::new();
        PROVIDER.get_or_init(|| install_provider().unwrap());
        let root = Directory::new();
        let recorded = transcripts();
        let certificate = hex_decode(recorded["tls"]["certificate"].as_str().unwrap()).unwrap();
        let key = hex_decode(recorded["tls"]["key"].as_str().unwrap()).unwrap();
        let tls = Arc::new(
            ServerConfig::builder()
                .with_no_client_auth()
                .with_single_cert(
                    vec![CertificateDer::from(certificate.clone())],
                    PrivateKeyDer::from(PrivatePkcs8KeyDer::from(key)),
                )
                .unwrap(),
        );
        Self {
            root,
            tls,
            certificate,
        }
    }
    fn config(
        &self,
        name: &str,
        sequencer_url: &str,
        network_url: &str,
        funding: chain_fund::FundingSlot,
    ) -> Arc<Config> {
        let state_dir = self.root.0.join(name);
        fs::DirBuilder::new()
            .mode(0o700)
            .create(&state_dir)
            .unwrap();
        for directory in ["journal", "funding-intents", "funding-receipts"] {
            fs::DirBuilder::new()
                .mode(0o700)
                .create(state_dir.join(directory))
                .unwrap();
        }
        self.reopen(state_dir, sequencer_url, network_url, funding)
    }
    fn reopen(
        &self,
        state_dir: PathBuf,
        sequencer_url: &str,
        network_url: &str,
        funding: chain_fund::FundingSlot,
    ) -> Arc<Config> {
        let treasury = SeedSigner::new(&[9; 32]);
        Arc::new(Config {
            listen: "127.0.0.1:0".parse().unwrap(),
            admin_listen: "127.0.0.1:0".parse().unwrap(),
            tls: Arc::clone(&self.tls),
            admin_tls: Arc::clone(&self.tls),
            sequencer: SequencerEndpoint::new(sequencer_url, &[]).unwrap(),
            network: PxEndpoint::new(network_url, &[]).unwrap(),
            network_id: 125,
            node: NodeEndpoint {
                host: String::new(),
                port: 0,
            },
            node_token: Zeroizing::new(String::new()),
            replica: NodeEndpoint {
                host: String::new(),
                port: 0,
            },
            replica_token: Zeroizing::new(String::new()),
            receipt_events_token: None,
            wallet_caps_token: None,
            admin_token: Zeroizing::new(ADMIN.into()),
            treasury_did: did_for_public_key(&treasury.public_key()),
            treasury: Arc::new(treasury),
            treasury_asset: [2; 32],
            funding,
            sequencer_id: [4; 32],
            supervisor_socket: state_dir.join("supervisor.sock"),
            state_dir,
            fee_limit: 1000,
            receipt_deadline: Duration::from_secs(5),
            admin_lock: Mutex::new(()),
            journal_lock: Mutex::new(()),
        })
    }
    fn exchange(&self, config: Arc<Config>, request: &Request) -> (u16, Value) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let plane = if request.path.starts_with("/admin/") {
            Plane::Admin
        } else {
            Plane::Core
        };
        let worker = thread::spawn(move || {
            let (tcp, _) = listener.accept().unwrap();
            handle_connection(&config, plane, tcp).unwrap();
        });
        let tcp = TcpStream::connect(address).unwrap();
        tcp.set_read_timeout(Some(Duration::from_secs(15))).unwrap();
        tcp.set_write_timeout(Some(Duration::from_secs(15)))
            .unwrap();
        let connector = native_tls::TlsConnector::builder()
            .add_root_certificate(native_tls::Certificate::from_der(&self.certificate).unwrap())
            .build()
            .unwrap();
        let mut stream = connector.connect("localhost", tcp).unwrap();
        write!(stream, "{} {} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {ADMIN}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n", request.method, request.path, request.body.len()).unwrap();
        if let Some(key) = request.headers.get("idempotency-key") {
            write!(stream, "Idempotency-Key: {key}\r\n").unwrap();
        }
        stream.write_all(b"\r\n").unwrap();
        stream.write_all(&request.body).unwrap();
        stream.flush().unwrap();
        let response =
            layerx_platform_internal::http::read_http_message(&mut stream, MAX_REQUEST_BYTES)
                .unwrap();
        let status = response.headers[""]
            .split_whitespace()
            .nth(1)
            .unwrap()
            .parse()
            .unwrap();
        let document = serde_json::from_slice(&response.body).unwrap();
        worker.join().unwrap();
        (status, document)
    }
}
fn request(method: &str, path: &str, body: Vec<u8>, key: Option<&str>) -> Request {
    let mut headers = BTreeMap::from([("content-type".into(), "application/json".into())]);
    if let Some(key) = key {
        headers.insert("idempotency-key".into(), key.into());
    }
    Request {
        method: method.into(),
        path: path.into(),
        query: None,
        headers,
        body,
    }
}
fn get(path: &str) -> Request {
    request("GET", path, Vec::new(), None)
}
fn unavailable_funding() -> chain_fund::FundingSlot {
    chain_fund::FundingSlot::configured(Err("sender_unavailable"))
}

#[derive(Default)]
struct ChainState {
    requests: Vec<Value>,
    submissions: Vec<Vec<u8>>,
    lose_response: bool,
    committed: bool,
    state_dir: Option<PathBuf>,
}

struct NativeChain {
    address: SocketAddr,
    state: Arc<Mutex<ChainState>>,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}

impl NativeChain {
    fn new(lose_response: bool) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        listener.set_nonblocking(true).unwrap();
        let state = Arc::new(Mutex::new(ChainState {
            lose_response,
            ..ChainState::default()
        }));
        let stop = Arc::new(AtomicBool::new(false));
        let observed = Arc::clone(&state);
        let stopping = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            let captured = transcripts();
            let preparation: Value =
                serde_json::from_str(include_str!("../fixtures/native_funding_preparation.json"))
                    .unwrap();
            let expected_raw = hex_decode(
                captured["funding_raw"]
                    .as_str()
                    .unwrap()
                    .strip_prefix("0x")
                    .unwrap(),
            )
            .unwrap();
            while !stopping.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(5));
                        continue;
                    }
                    Err(error) => panic!("native chain fixture: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                let incoming =
                    layerx_platform_internal::http::parse_client_request(&mut stream).unwrap();
                assert_eq!(incoming.method, "POST");
                assert_eq!(incoming.path, "/");
                let request: Value = serde_json::from_slice(&incoming.body).unwrap();
                assert_eq!(request["jsonrpc"], "2.0");
                let method = request["method"].as_str().unwrap();
                let key = format!("{method}{}", request["params"]);
                let mut state = observed.lock().unwrap();
                state.requests.push(request.clone());
                let result = if method == "eth_sendRawTransaction" {
                    assert_eq!(key, captured["funding_broadcast"].as_str().unwrap());
                    let raw = hex_decode(
                        request["params"][0]
                            .as_str()
                            .unwrap()
                            .strip_prefix("0x")
                            .unwrap(),
                    )
                    .unwrap();
                    assert_eq!(raw, expected_raw);
                    let root = state
                        .state_dir
                        .as_ref()
                        .expect("core state attached before broadcast");
                    let mut digest = Sha256::new();
                    digest.update(b"fund-canonical\0");
                    digest.update(KEY.as_bytes());
                    let basename = format!("{}.json", hex_encode(&digest.finalize()));
                    let records = fs::read_to_string(root.join("journal").join(&basename)).unwrap();
                    let record: Value =
                        serde_json::from_str(records.lines().last().unwrap()).unwrap();
                    let transfer: Value =
                        serde_json::from_str(record["body"].as_str().unwrap()).unwrap();
                    assert_eq!(transfer["signed"]["raw"], json!(raw));
                    let intent: Value = serde_json::from_slice(
                        &fs::read(root.join("funding-intents").join(&basename)).unwrap(),
                    )
                    .unwrap();
                    assert_eq!(intent["transfer"], transfer);
                    assert_eq!(intent["command"]["amount"], 123456);
                    assert_eq!(
                        transfer["signed"]["transaction_hash"],
                        format!(
                            "0x{}",
                            hex_encode(&layerx_crypto::evm_transaction::transaction_hash(&raw))
                        )
                    );
                    state.submissions.push(raw);
                    if state.lose_response {
                        state.lose_response = false;
                        continue;
                    }
                    captured["scenarios"]["funding_after"][&key].clone()
                } else if let Some(record) = preparation["responses"]
                    .as_array()
                    .unwrap()
                    .iter()
                    .find(|record| {
                        record["method"] == request["method"]
                            && record["params"] == request["params"]
                    })
                {
                    record["result"].clone()
                } else if key == format!("eth_getBalance{}", json!([RECIPIENT, "latest"]))
                    && !state.committed
                {
                    json!("0x0")
                } else {
                    let scenario = if state.committed {
                        "funding_after"
                    } else {
                        "funding_before"
                    };
                    captured["scenarios"][scenario]
                        .as_object()
                        .unwrap()
                        .get(&key)
                        .unwrap_or_else(|| {
                            panic!("request absent from native chain transcript: {key}")
                        })
                        .clone()
                };
                let body =
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).unwrap();
            }
        });
        Self {
            address,
            state,
            stop,
            worker: Some(worker),
        }
    }

    fn url(&self) -> String {
        format!("http://{}/", self.address)
    }

    fn funding(&self) -> chain_fund::FundingSlot {
        chain_fund::FundingSlot::configured(Ok(chain_fund::ChainFunding::from_sender(
            NodeEndpoint {
                host: self.address.ip().to_string(),
                port: self.address.port(),
            },
            EvmSender::from_hex_secret(EVM_KEY).unwrap(),
            125,
        )))
    }

    fn attach(&self, config: &Config) {
        self.state.lock().unwrap().state_dir = Some(config.state_dir.clone());
    }

    fn assert_replays(&self, count: usize) {
        let state = self.state.lock().unwrap();
        assert_eq!(state.submissions.len(), count);
        let raw = hex_decode(
            transcripts()["funding_raw"]
                .as_str()
                .unwrap()
                .strip_prefix("0x")
                .unwrap(),
        )
        .unwrap();
        assert!(state
            .submissions
            .iter()
            .all(|submission| submission == &raw));
        for method in [
            "eth_call",
            "eth_getTransactionCount",
            "eth_gasPrice",
            "eth_maxPriorityFeePerGas",
            "eth_estimateGas",
        ] {
            assert_eq!(
                state
                    .requests
                    .iter()
                    .filter(|request| request["method"] == method
                        && (method != "eth_getTransactionCount"
                            || request["params"][1] == "pending"))
                    .count(),
                1,
                "{method} must prepare only once"
            );
        }
    }
}

impl Drop for NativeChain {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let result = self.worker.take().unwrap().join();
        if !thread::panicking() {
            result.unwrap();
        }
    }
}

fn fund_request() -> Request {
    request(
        "POST",
        "/admin/v1/testnet/fund",
        serde_json::to_vec(&FundingCommand {
            funding_id: "native-recovery".into(),
            did: format!("did:layerx:{}", "ab".repeat(32)),
            public_key: "ab".repeat(32),
            amount: 123456,
        })
        .unwrap(),
        Some(KEY),
    )
}

fn export(fixture: &CoreFixture, config: &Arc<Config>) -> Value {
    let (status, exported) = fixture.exchange(
        Arc::clone(config),
        &get(&format!("/admin/v1/testnet/funding-intents/{KEY}")),
    );
    assert_eq!(status, 200, "{exported}");
    assert_eq!(
        exported["result"]["canonical_hex"],
        transcripts()["funding_raw"]
            .as_str()
            .unwrap()
            .strip_prefix("0x")
            .unwrap()
    );
    exported["result"]["recovery"].clone()
}

fn import(fixture: &CoreFixture, config: &Arc<Config>, recovery: &Value) {
    let (status, imported) = fixture.exchange(
        Arc::clone(config),
        &request(
            "POST",
            "/admin/v1/testnet/funding-intents",
            serde_json::to_vec(recovery).unwrap(),
            Some(KEY),
        ),
    );
    assert_eq!(status, 200, "{imported}");
    assert_eq!(imported["result"]["intent"], recovery["intent"]);
}

fn assert_committed(chain: &NativeChain, recovery: &Value) {
    let receipt = &recovery["archive"]["receipt"];
    let expected = transcripts();
    let receipt_key = format!(
        "eth_getTransactionReceipt{}",
        json!([recovery["intent"]["transfer"]["signed"]["transaction_hash"]])
    );
    assert_eq!(
        receipt,
        &expected["scenarios"]["funding_after"][&receipt_key]
    );
    assert_eq!(receipt["status"], "0x1");
    assert_eq!(receipt["blockNumber"], "0x2");
    assert_ne!(receipt["blockHash"], format!("0x{}", "00".repeat(32)));
    assert_eq!(receipt["from"], SENDER);
    assert_eq!(receipt["to"], RECIPIENT);
    assert_eq!(recovery["intent"]["transfer"]["amount"], 123456);
    assert_eq!(
        rpc(
            chain.address,
            "eth_getBalance",
            json!([RECIPIENT, "latest"])
        ),
        "0x1e240"
    );
    assert_eq!(
        rpc(
            chain.address,
            "eth_getTransactionCount",
            json!([SENDER, "latest"])
        ),
        "0x1"
    );
}

fn archive_import_reopen(
    fixture: &CoreFixture,
    chain: &NativeChain,
    recovery: &Value,
    funded: &Value,
) {
    let target = fixture.config(
        "archive-import",
        &chain.url(),
        &chain.url(),
        unavailable_funding(),
    );
    import(fixture, &target, recovery);
    let root = target.state_dir.clone();
    drop(target);
    let target = fixture.reopen(root, &chain.url(), &chain.url(), unavailable_funding());
    let before = chain.state.lock().unwrap().requests.len();
    assert_eq!(
        fixture.exchange(Arc::clone(&target), &fund_request()),
        (200, funded.clone())
    );
    assert_eq!(export(fixture, &target), *recovery);
    let mut changed = fund_request();
    changed.body.push(b' ');
    let (status, refusal) = fixture.exchange(Arc::clone(&target), &changed);
    assert_eq!(status, 409, "{refusal}");
    assert_eq!(refusal["error"]["code"], "idempotency_conflict");
    assert_eq!(
        chain.state.lock().unwrap().requests.len(),
        before,
        "archived replay must work without a chain or sender"
    );
}

pub fn native_funding_lost_response_reopen_archive_import() {
    let chain = NativeChain::new(true);
    let fixture = CoreFixture::new();
    let source = fixture.config("lost-response", &chain.url(), &chain.url(), chain.funding());
    chain.attach(&source);
    assert_eq!(
        rpc(
            chain.address,
            "eth_getBalance",
            json!([RECIPIENT, "latest"])
        ),
        "0x0"
    );
    let (status, refusal) = fixture.exchange(Arc::clone(&source), &fund_request());
    assert_eq!(status, 503, "{refusal}");
    assert_eq!(refusal["error"]["code"], "treasury_unavailable");
    let prepared = export(&fixture, &source);
    assert!(prepared["archive"].is_null());
    chain.assert_replays(1);
    let root = source.state_dir.clone();
    drop(source);
    let recovered = fixture.reopen(root, &chain.url(), &chain.url(), chain.funding());
    let (status, pending) = fixture.exchange(Arc::clone(&recovered), &fund_request());
    assert_eq!(status, 202, "{pending}");
    assert_eq!(pending["state"], "pending");
    assert_eq!(export(&fixture, &recovered), prepared);
    chain.state.lock().unwrap().committed = true;
    let (status, funded) = fixture.exchange(Arc::clone(&recovered), &fund_request());
    assert_eq!(status, 200, "{funded}");
    assert_eq!(funded["state"], "funded");
    assert_eq!(funded["funding_id"], "native-recovery");
    assert_eq!(funded["transaction_id"], pending["transaction_id"]);
    let archived = export(&fixture, &recovered);
    assert_eq!(archived["intent"], prepared["intent"]);
    assert_committed(&chain, &archived);
    archive_import_reopen(&fixture, &chain, &archived, &funded);
    chain.assert_replays(3);
}

pub fn native_funding_intent_producer_import_replay_and_refusals() {
    let chain = NativeChain::new(false);
    let fixture = CoreFixture::new();
    let source = fixture.config(
        "intent-producer",
        &chain.url(),
        &chain.url(),
        chain.funding(),
    );
    chain.attach(&source);
    let (status, pending) = fixture.exchange(Arc::clone(&source), &fund_request());
    assert_eq!(status, 202, "{pending}");
    assert_eq!(pending["state"], "pending");
    let produced = export(&fixture, &source);
    assert_eq!(produced["version"], 2);
    assert_eq!(produced["intent"]["version"], 2);
    assert!(produced["archive"].is_null());
    let source_path = source.state_dir.clone();
    drop(source);
    let source = fixture.reopen(source_path, &chain.url(), &chain.url(), chain.funding());
    assert_eq!(export(&fixture, &source), produced);
    drop(source);
    let target = fixture.config("intent-import", &chain.url(), &chain.url(), chain.funding());
    let target_path = target.state_dir.clone();
    drop(target);
    let target = fixture.reopen(
        target_path.clone(),
        &chain.url(),
        &chain.url(),
        chain.funding(),
    );
    for field in ["hash", "raw", "amount", "request", "authorization"] {
        let mut damaged = produced.clone();
        match field {
            "hash" => {
                damaged["intent"]["transfer"]["signed"]["transaction_hash"] =
                    json!(format!("0x{}", "00".repeat(32)))
            }
            "raw" => damaged["intent"]["transfer"]["signed"]["raw"][10] = json!(0),
            "amount" => damaged["intent"]["command"]["amount"] = json!(123457),
            "request" => damaged["intent"]["request_digest"] = json!("00".repeat(32)),
            "authorization" => {
                damaged["intent"]["authorization"][0] =
                    json!(damaged["intent"]["authorization"][0].as_u64().unwrap() ^ 1)
            }
            _ => unreachable!(),
        }
        let (status, refusal) = fixture.exchange(
            Arc::clone(&target),
            &request(
                "POST",
                "/admin/v1/testnet/funding-intents",
                serde_json::to_vec(&damaged).unwrap(),
                Some(KEY),
            ),
        );
        assert_eq!(status, 400, "{field}: {refusal}");
        assert_eq!(refusal["error"]["code"], "invalid_funding_intent");
        assert_eq!(
            fs::read_dir(target.state_dir.join("funding-intents"))
                .unwrap()
                .count(),
            0
        );
    }
    import(&fixture, &target, &produced);
    drop(target);
    let target = fixture.reopen(target_path, &chain.url(), &chain.url(), chain.funding());
    chain.attach(&target);
    assert_eq!(export(&fixture, &target), produced);
    chain.state.lock().unwrap().committed = true;
    let (status, funded) = fixture.exchange(Arc::clone(&target), &fund_request());
    assert_eq!(status, 200, "{funded}");
    assert_eq!(funded["state"], "funded");
    assert_eq!(funded["transaction_id"], pending["transaction_id"]);
    let archived = export(&fixture, &target);
    assert_eq!(archived["intent"], produced["intent"]);
    assert_committed(&chain, &archived);
    let mut damaged = archived.clone();
    damaged["archive"]["receipt"]["to"] = json!(SENDER);
    let (status, refusal) = fixture.exchange(
        Arc::clone(&target),
        &request(
            "POST",
            "/admin/v1/testnet/funding-intents",
            serde_json::to_vec(&damaged).unwrap(),
            Some(KEY),
        ),
    );
    assert_eq!(status, 400, "{refusal}");
    assert_eq!(refusal["error"]["code"], "invalid_funding_archive");
    archive_import_reopen(&fixture, &chain, &archived, &funded);
    chain.assert_replays(2);
}
