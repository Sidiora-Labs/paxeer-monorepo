include!("../src/main.rs");

#[cfg(test)]
mod repair {
    use super::*;
    use layerx_client::storage_witness::verify_storage_witness;
    use layerx_platform_core::{SeedSigner, TreasurySigner as _};
    use layerx_platform_internal::evm_sender::EvmSender;
    use serde_json::{json, Value};
    use std::collections::VecDeque;
    use std::sync::atomic::AtomicBool;

    const EVM_KEY: &str = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80";
    const ADMIN: &str = "repair-22-31-354-admin";

    struct Directory(PathBuf);
    impl Directory {
        fn new() -> Self {
            let path = env::temp_dir().join(format!(
                "core-repair-{}-{}",
                std::process::id(),
                TRACE.fetch_add(1, Ordering::Relaxed)
            ));
            fs::DirBuilder::new()
                .mode(0o700)
                .create(&path)
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            Self(path)
        }
    }
    impl Drop for Directory {
        fn drop(&mut self) {
            fs::remove_dir_all(&self.0).unwrap_or_else(|error| panic!("required value: {error:?}"));
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
    fn rpc(address: SocketAddr, method: &str, params: &Value) -> Value {
        let document = rpc_document(
            address,
            &json!({"jsonrpc":"2.0", "id": 1, "method":method,"params":params}),
        )
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        assert!(document.get("error").is_none(), "{method}: {document}");
        document["result"].clone()
    }

    fn transcripts() -> Value {
        serde_json::from_str(include_str!("../src/repair_22_31_354_transcripts.json"))
            .unwrap_or_else(|error| panic!("required value: {error:?}"))
    }

    // Captures are emitted by the real JSON-RPC producer before qualification.
    // The gate only replays committed bytes; unexpected requests fail closed.
    struct NativeChain {
        transport: RecordedTransport,
        address: SocketAddr,
    }
    impl NativeChain {
        fn new() -> Self {
            Self::scenario("common")
        }
        fn scenario(name: &str) -> Self {
            let transport = RecordedTransport::captured(Self::captures(name));
            let address = transport.address;
            Self { transport, address }
        }
        fn captures(name: &str) -> BTreeMap<String, Value> {
            transcripts()["scenarios"][name]
                .as_object()
                .unwrap_or_else(|| panic!("required value"))
                .iter()
                .map(|(key, value)| {
                    let offset = key.find('[').unwrap_or_else(|| panic!("required value"));
                    let params: Value = serde_json::from_str(&key[offset..])
                        .unwrap_or_else(|error| panic!("required value: {error:?}"));
                    (format!("{}{params}", &key[..offset]), value.clone())
                })
                .collect()
        }
        fn select(&self, name: &str) {
            *self
                .transport
                .captures
                .lock()
                .unwrap_or_else(|error| panic!("required value: {error:?}")) = Self::captures(name);
        }
        fn funding(&self) -> chain_fund::ChainFunding {
            chain_fund::ChainFunding::from_sender(
                NodeEndpoint {
                    host: self.address.ip().to_string(),
                    port: self.address.port(),
                },
                EvmSender::from_hex_secret(EVM_KEY)
                    .unwrap_or_else(|error| panic!("required value: {error:?}")),
                125,
            )
        }
        fn url(&self) -> String {
            format!("http://{}/", self.address)
        }
    }

    struct RecordedTransport {
        address: SocketAddr,
        requests: Arc<Mutex<Vec<Value>>>,
        captures: Arc<Mutex<BTreeMap<String, Value>>>,
        sequences: Arc<Mutex<BTreeMap<String, VecDeque<Value>>>>,
        stop: Arc<AtomicBool>,
        worker: Option<thread::JoinHandle<()>>,
    }
    impl RecordedTransport {
        fn new(upstream: SocketAddr, captures: BTreeMap<String, Value>) -> Self {
            Self::serve(Some(upstream), captures)
        }
        fn captured(captures: BTreeMap<String, Value>) -> Self {
            Self::serve(None, captures)
        }
        fn serve(upstream: Option<SocketAddr>, captures: BTreeMap<String, Value>) -> Self {
            let captures = Arc::new(Mutex::new(captures));
            let responses = Arc::clone(&captures);
            let sequences = Arc::new(Mutex::new(BTreeMap::<String, VecDeque<Value>>::new()));
            let ordered = Arc::clone(&sequences);
            let listener = TcpListener::bind("127.0.0.1:0")
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            let address = listener
                .local_addr()
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            listener
                .set_nonblocking(true)
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            let stop = Arc::new(AtomicBool::new(false));
            let requests = Arc::new(Mutex::new(Vec::new()));
            let stopping = Arc::clone(&stop);
            let observed = Arc::clone(&requests);
            let worker = thread::spawn(move || {
                while !stopping.load(Ordering::Acquire) {
                    let (mut stream, _) = match listener.accept() {
                        Ok(pair) => pair,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            thread::sleep(Duration::from_millis(5));
                            continue;
                        }
                        Err(error) => panic!("recorded transport: {error}"),
                    };
                    stream
                        .set_read_timeout(Some(Duration::from_secs(10)))
                        .unwrap_or_else(|error| panic!("required value: {error:?}"));
                    let request = layerx_platform_internal::http::parse_client_request(&mut stream)
                        .unwrap_or_else(|error| panic!("required value: {error:?}"));
                    assert_eq!(request.method, "POST");
                    assert_eq!(request.path, "/");
                    let request: Value = serde_json::from_slice(&request.body)
                        .unwrap_or_else(|error| panic!("required value: {error:?}"));
                    observed
                        .lock()
                        .unwrap_or_else(|error| panic!("required value: {error:?}"))
                        .push(request.clone());
                    let key = format!(
                        "{}{}",
                        request["method"]
                            .as_str()
                            .unwrap_or_else(|| panic!("required value")),
                        request["params"]
                    );
                    let result = if let Some(sequence) = ordered
                        .lock()
                        .unwrap_or_else(|error| panic!("required value: {error:?}"))
                        .get_mut(&key)
                    {
                        Some(
                            sequence
                                .pop_front()
                                .unwrap_or_else(|| panic!("{}", "producer transcript exhausted")),
                        )
                    } else {
                        responses
                            .lock()
                            .unwrap_or_else(|error| panic!("required value: {error:?}"))
                            .get(&key)
                            .cloned()
                    };
                    let response = if let Some(result) = result {
                        if key
                            == transcripts()["funding_broadcast"]
                                .as_str()
                                .unwrap_or_else(|| panic!("required value"))
                        {
                            *responses
                                .lock()
                                .unwrap_or_else(|error| panic!("required value: {error:?}")) =
                                NativeChain::captures("funding_after");
                        }
                        json!({"jsonrpc":"2.0","id":request["id"],"result":result})
                    } else if let Some(upstream) = upstream {
                        rpc_document(upstream, &request)
                            .unwrap_or_else(|error| panic!("required value: {error:?}"))
                    } else {
                        panic!("request is absent from the producer transcript: {key}");
                    };
                    let body = response.to_string();
                    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).unwrap_or_else(|error| panic!("required value: {error:?}"));
                }
            });
            Self {
                address,
                requests,
                captures,
                sequences,
                stop,
                worker: Some(worker),
            }
        }
        fn url(&self) -> String {
            format!("http://{}/", self.address)
        }
    }
    impl Drop for RecordedTransport {
        fn drop(&mut self) {
            self.stop.store(true, Ordering::Release);
            self.worker
                .take()
                .unwrap_or_else(|| panic!("required value"))
                .join()
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
        }
    }

    struct CoreFixture {
        root: Directory,
        tls: Arc<ServerConfig>,
        certificate: Vec<u8>,
    }
    impl CoreFixture {
        fn new() -> Self {
            static PROVIDER: std::sync::OnceLock<()> = std::sync::OnceLock::new();
            PROVIDER.get_or_init(|| {
                install_provider().unwrap_or_else(|error| panic!("required value: {error:?}"));
            });
            let root = Directory::new();
            let recorded = transcripts();
            let certificate = hex_decode(
                recorded["tls"]["certificate"]
                    .as_str()
                    .unwrap_or_else(|| panic!("required value")),
            )
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
            let key = hex_decode(
                recorded["tls"]["key"]
                    .as_str()
                    .unwrap_or_else(|| panic!("required value")),
            )
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
            let tls = Arc::new(
                ServerConfig::builder()
                    .with_no_client_auth()
                    .with_single_cert(
                        vec![CertificateDer::from(certificate.clone())],
                        PrivateKeyDer::from(PrivatePkcs8KeyDer::from(key)),
                    )
                    .unwrap_or_else(|error| panic!("required value: {error:?}")),
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
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            for directory in ["journal", "funding-intents", "funding-receipts"] {
                fs::DirBuilder::new()
                    .mode(0o700)
                    .create(state_dir.join(directory))
                    .unwrap_or_else(|error| panic!("required value: {error:?}"));
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
                listen: "127.0.0.1:0"
                    .parse()
                    .unwrap_or_else(|error| panic!("required value: {error:?}")),
                admin_listen: "127.0.0.1:0"
                    .parse()
                    .unwrap_or_else(|error| panic!("required value: {error:?}")),
                tls: Arc::clone(&self.tls),
                admin_tls: Arc::clone(&self.tls),
                sequencer: SequencerEndpoint::new(sequencer_url, &[])
                    .unwrap_or_else(|error| panic!("required value: {error:?}")),
                network: PxEndpoint::new(network_url, &[])
                    .unwrap_or_else(|error| panic!("required value: {error:?}")),
                network_id: 125,
                _node: NodeEndpoint {
                    host: String::new(),
                    port: 0,
                },
                _node_token: Zeroizing::new(String::new()),
                _replica: NodeEndpoint {
                    host: String::new(),
                    port: 0,
                },
                _replica_token: Zeroizing::new(String::new()),
                receipt_events_token: None,
                _wallet_caps_token: None,
                admin_token: Zeroizing::new(ADMIN.into()),
                treasury_did: did_for_public_key(&treasury.public_key()),
                treasury: Arc::new(treasury),
                treasury_asset: [2; 32],
                funding,
                sequencer_id: [4; 32],
                supervisor_socket: state_dir.join("supervisor.sock"),
                state_dir,
                _fee_limit: 1000,
                receipt_deadline: Duration::from_secs(5),
                admin_lock: Mutex::new(()),
                journal_lock: Mutex::new(()),
            })
        }
        fn exchange(&self, config: Arc<Config>, request: &Request) -> (u16, Value) {
            let listener = TcpListener::bind("127.0.0.1:0")
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            let address = listener
                .local_addr()
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            let plane = if request.path.starts_with("/admin/") {
                Plane::Admin
            } else {
                Plane::Core
            };
            let worker = thread::spawn(move || {
                let (tcp, _) = listener
                    .accept()
                    .unwrap_or_else(|error| panic!("required value: {error:?}"));
                handle_connection(&config, plane, tcp)
                    .unwrap_or_else(|error| panic!("required value: {error:?}"));
            });
            let tcp = TcpStream::connect(address)
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            tcp.set_read_timeout(Some(Duration::from_secs(15)))
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            tcp.set_write_timeout(Some(Duration::from_secs(15)))
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            let connector = native_tls::TlsConnector::builder()
                .add_root_certificate(
                    native_tls::Certificate::from_der(&self.certificate)
                        .unwrap_or_else(|error| panic!("required value: {error:?}")),
                )
                .build()
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            let mut stream = connector
                .connect("localhost", tcp)
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            write!(stream, "{} {} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {ADMIN}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n", request.method, request.path, request.body.len()).unwrap_or_else(|error| panic!("required value: {error:?}"));
            if let Some(key) = request.headers.get("idempotency-key") {
                write!(stream, "Idempotency-Key: {key}\r\n")
                    .unwrap_or_else(|error| panic!("required value: {error:?}"));
            }
            stream
                .write_all(b"\r\n")
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            stream
                .write_all(&request.body)
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            stream
                .flush()
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            let response =
                layerx_platform_internal::http::read_http_message(&mut stream, MAX_REQUEST_BYTES)
                    .unwrap_or_else(|error| panic!("required value: {error:?}"));
            let status = response.headers[""]
                .split_whitespace()
                .nth(1)
                .unwrap_or_else(|| panic!("required value"))
                .parse()
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            let document = serde_json::from_slice(&response.body)
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
            worker
                .join()
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
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

    #[test]
    fn core_replica_readiness_uses_live_chain_and_writable_journal() {
        let chain = NativeChain::new();
        let fixture = CoreFixture::new();
        let config = fixture.config("ready", &chain.url(), &chain.url(), unavailable_funding());
        let (status, body) = fixture.exchange(Arc::clone(&config), &get("/readyz"));
        assert_eq!(status, 200, "{body}");
        assert_eq!(body["ready"], true);
        assert_eq!(body["funding"]["ready"], false);
        assert_eq!(body["funding"]["reason"], "sender_unavailable");
        assert_eq!(body["paxeer"]["chain_id"], 125);
        assert!(config.state_dir.join("journal/ready").is_file());
        fs::remove_file(config.state_dir.join("journal/ready"))
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
        fs::create_dir(config.state_dir.join("journal/ready"))
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let (status, body) = fixture.exchange(config, &get("/readyz"));
        assert_eq!(status, 503);
        assert_eq!(body["error"]["code"], "journal_unavailable");
    }

    #[test]
    fn core_retired_relays_use_native_state_and_explicit_retirement() {
        let chain = NativeChain::new();
        let fixture = CoreFixture::new();
        let config = fixture.config("state", &chain.url(), &chain.url(), unavailable_funding());
        let block = rpc(
            chain.address,
            "eth_getBlockByNumber",
            &json!(["0x1", false]),
        );
        for path in ["/v1/state", "/v1/protocol/account-state/head"] {
            let (status, body) = fixture.exchange(Arc::clone(&config), &get(path));
            assert_eq!(status, 200, "{body}");
            assert_eq!(body["result"]["height"], "1");
            assert_eq!(
                body["result"]["state_root"],
                block["stateRoot"]
                    .as_str()
                    .unwrap_or_else(|| panic!("required value"))
                    .strip_prefix("0x")
                    .unwrap_or_else(|| panic!("required value"))
            );
        }
        for path in [
            format!("/v1/batches/{}/receipt-authority", "12".repeat(32)),
            "/v1/programs/account-state/changes".into(),
            format!("/v1/programs/{}/account-state", "12".repeat(32)),
            format!("/v1/receipts/{}/account-state", "12".repeat(32)),
        ] {
            let (status, body) = fixture.exchange(Arc::clone(&config), &get(&path));
            assert_eq!(status, 410, "{path}: {body}");
            assert_eq!(body["error"]["code"], "retired_http_contract");
        }
    }

    #[test]
    fn custody_retired_precompiles_use_native_reads_and_refuse_absent_metadata() {
        let chain = NativeChain::new();
        let transport = RecordedTransport::new(chain.address, BTreeMap::new());
        let sequencer = SequencerEndpoint::new(&transport.url(), &[])
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let (block, root) = layerx_platform_core::native_committed_state(&sequencer, 1)
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
        assert_eq!(block.number, 1);
        assert_ne!(root, [0; 32]);
        assert_eq!(
            layerx_platform_core::custody_latest_finalized(&sequencer, 1)
                .unwrap_or_else(|error| panic!("required value: {error:?}")),
            1
        );
        assert_eq!(
            layerx_platform_core::custody_checkpoint_state_root(&sequencer, 1, 1)
                .unwrap_or_else(|error| panic!("required value: {error:?}")),
            Some(root)
        );
        assert!(layerx_platform_core::read_native_asset_profile(
            &sequencer, 125, 1, [2; 32], "uhpx"
        )
        .is_err());
        let requests = transport
            .requests
            .lock()
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let calls: Vec<_> = requests
            .iter()
            .filter(|request| request["method"] == "eth_call")
            .collect();
        assert!(!calls.is_empty());
        assert_eq!(
            calls[0]["params"][0]["to"],
            "0x0000000000000000000000000000000000001001"
        );
        assert!(calls.iter().all(|request| ![
            "0x0000000000000000000000000000000000001013",
            "0x0000000000000000000000000000000000001014"
        ]
        .contains(
            &request["params"][0]["to"]
                .as_str()
                .unwrap_or_else(|| panic!("required value"))
        )));
    }

    fn receipt_capture() -> Value {
        let vector: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        assert_eq!(
            vector["provenance"]["generator"],
            "rpc/sequencer_receipt_vectors_test.go"
        );
        vector["cases"][0].clone()
    }
    fn receipt_transport(chain: &NativeChain, capture: &Value) -> RecordedTransport {
        RecordedTransport::new(
            chain.address,
            BTreeMap::from([
                (
                    format!(
                        "pax_sequencerReceipt{}",
                        json!([capture["receiptJson"]["txHash"]])
                    ),
                    capture["receiptJson"].clone(),
                ),
                (
                    "px_getReceiptKeys[]".into(),
                    json!({"keys":[capture["pinnedKey"].clone()]}),
                ),
            ]),
        )
    }
    fn land_captured_transaction(chain: &NativeChain, capture: &Value) {
        chain.select("receipt_before");
        assert_eq!(rpc(chain.address, "eth_blockNumber", &json!([])), "0x3e7");
        assert_eq!(capture["rawTransaction"], transcripts()["receipt_raw"]);
        chain.select("receipt");
        assert_eq!(rpc(chain.address, "eth_blockNumber", &json!([])), "0x3e8");
        let landing = rpc(
            chain.address,
            "eth_getTransactionReceipt",
            &json!([capture["receiptJson"]["txHash"]]),
        );
        assert_eq!(landing["blockNumber"], "0x3e8");
    }

    fn assert_misplaced_receipt_refused(
        fixture: &CoreFixture,
        config: Arc<Config>,
        chain: &NativeChain,
        transport: &RecordedTransport,
        capture: &Value,
        tx: &str,
    ) {
        let mut misplaced = rpc(
            chain.address,
            "eth_getTransactionReceipt",
            &json!([capture["receiptJson"]["txHash"]]),
        );
        misplaced["transactionIndex"] = json!("0x1");
        transport
            .captures
            .lock()
            .unwrap_or_else(|error| panic!("required value: {error:?}"))
            .insert(
                format!(
                    "eth_getTransactionReceipt{}",
                    json!([capture["receiptJson"]["txHash"]])
                ),
                misplaced,
            );
        let (status, body) = fixture.exchange(config, &get(&format!("/v1/receipts/{tx}")));
        assert_eq!(status, 503, "{body}");
        assert_eq!(body["error"]["code"], "receipt_unavailable");
    }

    fn gateway_account_record() -> Value {
        let gateway: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/wallet/test/fixtures/endpoint/available.json"
        ))
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        gateway
            .as_array()
            .unwrap_or_else(|| panic!("required value"))
            .iter()
            .find(|record| record["request"]["method"] == "px_getAccount")
            .unwrap_or_else(|| panic!("required value"))
            .clone()
    }

    #[test]
    fn receipt_anchor_dependency_public_receipt_binds_native_landing_block() {
        let chain = NativeChain::new();
        let capture = receipt_capture();
        land_captured_transaction(&chain, &capture);
        let transport = receipt_transport(&chain, &capture);
        let joined = gateway_account_record();
        transport
            .captures
            .lock()
            .unwrap_or_else(|error| panic!("required value: {error:?}"))
            .insert(
                format!("px_getAccount{}", joined["request"]["params"]),
                joined["response"]["result"].clone(),
            );
        let fixture = CoreFixture::new();
        let config = fixture.config(
            "receipt",
            &transport.url(),
            &transport.url(),
            unavailable_funding(),
        );
        let tx = capture["receiptJson"]["txHash"]
            .as_str()
            .unwrap_or_else(|| panic!("required value"))
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("required value"));
        let landing = rpc(
            chain.address,
            "eth_getBlockByNumber",
            &json!(["0x3e8", false]),
        );
        chain.select("receipt_advanced");
        let (status, body) =
            fixture.exchange(Arc::clone(&config), &get(&format!("/v1/receipts/{tx}")));
        assert_eq!(status, 200, "{body}");
        assert_eq!(
            body["result"]["receipt"],
            capture["signedReceipt"]
                .as_str()
                .unwrap_or_else(|| panic!("required value"))
                .strip_prefix("0x")
                .unwrap_or_else(|| panic!("required value"))
        );
        assert_eq!(
            body["result"]["state_root"],
            landing["stateRoot"]
                .as_str()
                .unwrap_or_else(|| panic!("required value"))
                .strip_prefix("0x")
                .unwrap_or_else(|| panic!("required value"))
        );
        assert_eq!(
            body["result"]["block_hash"],
            landing["hash"]
                .as_str()
                .unwrap_or_else(|| panic!("required value"))
                .strip_prefix("0x")
                .unwrap_or_else(|| panic!("required value"))
        );
        assert!(transport
            .requests
            .lock()
            .unwrap_or_else(|error| panic!("required value: {error:?}"))
            .iter()
            .all(|request| request["method"] != "eth_call"));
        for kind in ["receipt", "activity"] {
            let (status, body) = fixture.exchange(
                Arc::clone(&config),
                &get(&format!("/v1/proofs/{kind}/{tx}")),
            );
            assert_eq!(status, 503, "{body}");
            assert_eq!(
                body["error"]["code"],
                "native_receipt_inclusion_unavailable"
            );
        }
        let selector = joined["request"]["params"][0]
            .as_str()
            .unwrap_or_else(|| panic!("required value"));
        let (status, body) = fixture.exchange(
            Arc::clone(&config),
            &get(&format!("/v1/proofs/account/{tx}/{selector}")),
        );
        assert_eq!(status, 503, "{body}");
        assert_eq!(
            body["error"]["code"],
            "native_account_inclusion_unavailable"
        );
        assert!(transport
            .requests
            .lock()
            .unwrap_or_else(|error| panic!("required value: {error:?}"))
            .iter()
            .filter(|request| request["method"] == "eth_getBlockByNumber")
            .all(|request| request["params"][0] == "0x3e8"));
        assert_misplaced_receipt_refused(&fixture, config, &chain, &transport, &capture, tx);
    }

    fn assert_import_replay_and_tamper(
        fixture: &CoreFixture,
        target: Arc<Config>,
        chain: &NativeChain,
        fund_request: &Request,
        body: &Value,
        exported: &Value,
        transfer: &chain_fund::NativeTransfer,
    ) {
        let key = fund_request.headers.get("idempotency-key").map_or_else(
            || panic!("funding idempotency key required"),
            String::as_str,
        );
        let target_path = target.state_dir.clone();
        drop(target);
        let target = fixture.reopen(
            target_path,
            &chain.url(),
            &chain.url(),
            unavailable_funding(),
        );
        let (status, replay) = fixture.exchange(Arc::clone(&target), fund_request);
        assert_eq!(status, 200, "{replay}");
        assert_eq!(&replay, body);
        retain_funding_receipts(&target).unwrap_or_else(|response| panic!("{}", response.body));
        let mut damaged = exported["result"]["recovery"].clone();
        damaged["intent"]["transfer"]["signed"]["raw"][10] = json!(0);
        let (status, _) = fixture.exchange(
            Arc::clone(&target),
            &request(
                "POST",
                "/admin/v1/testnet/funding-intents",
                serde_json::to_vec(&damaged)
                    .unwrap_or_else(|error| panic!("required value: {error:?}")),
                Some(key),
            ),
        );
        assert_eq!(status, 400);
        let mut changed_request = request(
            "POST",
            "/admin/v1/testnet/fund",
            fund_request.body.clone(),
            Some(key),
        );
        changed_request.body.push(b' ');
        let (status, conflict) = fixture.exchange(target, &changed_request);
        assert_eq!(status, 409, "{conflict}");
        assert_eq!(
            rpc(
                chain.address,
                "eth_getTransactionCount",
                &json!([format!("0x{}", hex_encode(&transfer.sender)), "latest"])
            ),
            "0x1"
        );
    }

    fn native_recovery_request() -> (FundingCommand, Request) {
        let command = FundingCommand {
            funding_id: "native-recovery".into(),
            did: format!("did:layerx:{}", "ab".repeat(32)),
            public_key: "ab".repeat(32),
            amount: 123_456,
        };
        let fund_request = request(
            "POST",
            "/admin/v1/testnet/fund",
            serde_json::to_vec(&command)
                .unwrap_or_else(|error| panic!("required value: {error:?}")),
            Some("native-recovery"),
        );
        (command, fund_request)
    }

    #[test]
    fn funding_recovery_schema_live_archive_import_and_restart_replay() {
        let chain = NativeChain::scenario("funding_before");
        let fixture = CoreFixture::new();
        let source = fixture.config(
            "fund-source",
            &chain.url(),
            &chain.url(),
            chain_fund::FundingSlot::configured(Ok(chain.funding())),
        );
        let (command, fund_request) = native_recovery_request();
        let key = "native-recovery";
        let funding = source
            .funding
            .get()
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let signed = funding
            .sign_native(
                [0x39; 20],
                command.amount,
                0,
                21000,
                1_000_000_000,
                3_000_000_000,
            )
            .unwrap_or_else(|response| panic!("{}", response.body));
        assert_eq!(
            format!("0x{}", hex_encode(&signed.raw)),
            transcripts()["funding_raw"]
        );
        let transfer = chain_fund::NativeTransfer::attest(
            signed,
            funding.sender_address(),
            source.treasury.as_ref(),
        )
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        funding_native::persist_prepared(&source, &fund_request, &command, key, transfer.clone())
            .unwrap_or_else(|response| panic!("{}", response.body));
        // The prepare response is lost. Reopening the component consumes the
        // real durable stage; no RPC preparation or new signature is needed.
        let state_dir = source.state_dir.clone();
        drop(source);
        let source = fixture.reopen(
            state_dir,
            &chain.url(),
            &chain.url(),
            chain_fund::FundingSlot::configured(Ok(chain.funding())),
        );
        let recovered = prepare_funding(&source, &fund_request, &command, key)
            .unwrap_or_else(|response| panic!("{}", response.body));
        assert_eq!(recovered, transfer);
        let (status, body) = fixture.exchange(Arc::clone(&source), &fund_request);
        assert_eq!(status, 200, "{body}");
        assert_eq!(body["state"], "funded");
        assert_eq!(
            rpc(
                chain.address,
                "eth_getBalance",
                &json!([format!("0x{}", "39".repeat(20)), "latest"])
            ),
            "0x1e240"
        );
        retain_funding_receipts(&source).unwrap_or_else(|response| panic!("{}", response.body));
        let (status, exported) = fixture.exchange(
            Arc::clone(&source),
            &get(&format!("/admin/v1/testnet/funding-intents/{key}")),
        );
        assert_eq!(status, 200, "{exported}");
        assert_eq!(
            exported["result"]["recovery"]["intent"]["transfer"]["signed"]["raw"],
            json!(transfer.signed.raw)
        );
        assert!(exported["result"]["recovery"]["archive"].is_object());
        let target = fixture.config(
            "fund-target",
            &chain.url(),
            &chain.url(),
            unavailable_funding(),
        );
        let import = request(
            "POST",
            "/admin/v1/testnet/funding-intents",
            serde_json::to_vec(&exported["result"]["recovery"])
                .unwrap_or_else(|error| panic!("required value: {error:?}")),
            Some(key),
        );
        let (status, imported) = fixture.exchange(Arc::clone(&target), &import);
        assert_eq!(status, 200, "{imported}");
        assert_import_replay_and_tamper(
            &fixture,
            target,
            &chain,
            &fund_request,
            &body,
            &exported,
            &transfer,
        );
    }

    fn legacy_binding_transport(chain: &NativeChain) -> (Value, RecordedTransport) {
        let vectors: Value = serde_json::from_str(include_str!(
            "../../gateway/tests/fixtures/paxeer-abi-vectors.json"
        ))
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let binding = vectors["cases"]
            .as_array()
            .unwrap_or_else(|| panic!("required value"))
            .iter()
            .find(|case| case["name"] == "addr.getEvmAddrByLayerX")
            .unwrap_or_else(|| panic!("required value"));
        let params = json!([{"to": vectors["precompiles"]["addr"], "data": format!("0x{}", binding["calldata"].as_str().unwrap_or_else(|| panic!("required value")))}, "latest"]);
        let transport = RecordedTransport::new(
            chain.address,
            BTreeMap::from([(
                format!("eth_call{params}"),
                json!(format!(
                    "0x{}",
                    binding["result"]
                        .as_str()
                        .unwrap_or_else(|| panic!("required value"))
                )),
            )]),
        );
        ((*binding).clone(), transport)
    }

    fn assert_recipient_mismatch_refused(
        source: &Arc<Config>,
        command: &FundingCommand,
        fund_request: &Request,
        legacy: JournalEntry,
    ) {
        let mut wrong = legacy;
        wrong.body = serde_json::to_string(
            &source
                .funding
                .get()
                .unwrap_or_else(|error| panic!("required value: {error:?}"))
                .sign_native(
                    [0x39; 20],
                    command.amount,
                    0,
                    21000,
                    1_000_000_000,
                    3_000_000_000,
                )
                .unwrap_or_else(|response| panic!("{}", response.body)),
        )
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        journal_write(
            &journal_path(source, "fund-canonical", "wrong-recipient"),
            &wrong,
        )
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let error = prepare_funding(source, fund_request, command, "wrong-recipient")
            .err()
            .unwrap_or_else(|| panic!("{}", "mismatched recipient must be refused"));
        assert_eq!(error.status, 503);
        assert!(!funding_record_path(source, "funding-intents", "wrong-recipient").exists());
    }

    fn configured_legacy_funding(transport: &RecordedTransport) -> chain_fund::FundingSlot {
        chain_fund::FundingSlot::configured(Ok(chain_fund::ChainFunding::from_sender(
            NodeEndpoint {
                host: transport.address.ip().to_string(),
                port: transport.address.port(),
            },
            EvmSender::from_hex_secret(EVM_KEY)
                .unwrap_or_else(|error| panic!("required value: {error:?}")),
            125,
        )))
    }

    #[test]
    fn funding_recovery_schema_migrates_legacy_native_stage_and_refuses_recipient_mismatch() {
        let chain = NativeChain::new();
        let (binding, transport) = legacy_binding_transport(&chain);
        let fixture = CoreFixture::new();
        let source = fixture.config(
            "legacy-native-stage",
            &chain.url(),
            &chain.url(),
            configured_legacy_funding(&transport),
        );
        let command = FundingCommand {
            funding_id: "legacy-native".into(),
            did: format!(
                "did:layerx:{}",
                binding["in"]["didPublicKey"]
                    .as_str()
                    .unwrap_or_else(|| panic!("required value"))
            ),
            public_key: binding["in"]["didPublicKey"]
                .as_str()
                .unwrap_or_else(|| panic!("required value"))
                .into(),
            amount: 123_456,
        };
        let fund_request = request(
            "POST",
            "/admin/v1/testnet/fund",
            serde_json::to_vec(&command)
                .unwrap_or_else(|error| panic!("required value: {error:?}")),
            Some("legacy-native"),
        );
        let recipient = fixed_hex::<20>(
            "recipient",
            binding["out"]["evmAddr"]
                .as_str()
                .unwrap_or_else(|| panic!("required value"))
                .strip_prefix("0x")
                .unwrap_or_else(|| panic!("required value")),
        )
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let signed = source
            .funding
            .get()
            .unwrap_or_else(|error| panic!("required value: {error:?}"))
            .sign_native(
                recipient,
                command.amount,
                0,
                21000,
                1_000_000_000,
                3_000_000_000,
            )
            .unwrap_or_else(|response| panic!("{}", response.body));
        let digest = hex_encode(&Sha256::digest(
            serde_json::to_vec(&command)
                .unwrap_or_else(|error| panic!("required value: {error:?}")),
        ));
        let legacy = JournalEntry {
            request_digest: digest,
            status: 202,
            body: serde_json::to_string(&signed)
                .unwrap_or_else(|error| panic!("required value: {error:?}")),
            retry_after: Some(5),
        };
        journal_write(
            &journal_path(&source, "fund-canonical", "legacy-native"),
            &legacy,
        )
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let state_dir = source.state_dir.clone();
        drop(source);
        let source = fixture.reopen(
            state_dir,
            &chain.url(),
            &chain.url(),
            configured_legacy_funding(&transport),
        );
        let migrated = prepare_funding(&source, &fund_request, &command, "legacy-native")
            .unwrap_or_else(|response| panic!("{}", response.body));
        assert_eq!(migrated.signed, signed);
        assert_eq!(migrated.recipient, recipient);
        let persisted = journal_read(&journal_path(&source, "fund-canonical", "legacy-native"))
            .unwrap_or_else(|error| panic!("required value: {error:?}"))
            .unwrap_or_else(|| panic!("required value"));
        assert_eq!(
            serde_json::from_str::<chain_fund::NativeTransfer>(&persisted.body)
                .unwrap_or_else(|error| panic!("required value: {error:?}")),
            migrated
        );
        let (status, exported) = fixture.exchange(
            Arc::clone(&source),
            &get("/admin/v1/testnet/funding-intents/legacy-native"),
        );
        assert_eq!(status, 200, "{exported}");
        assert_eq!(
            exported["result"]["recovery"]["intent"]["transfer"]["version"],
            2
        );
        assert_recipient_mismatch_refused(&source, &command, &fund_request, legacy);
    }

    #[test]
    fn account_balance_shape_consumes_recorded_gateway_through_public_route() {
        let chain = NativeChain::scenario("balance");
        let records: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/wallet/test/fixtures/endpoint/available.json"
        ))
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let record = records
            .as_array()
            .unwrap_or_else(|| panic!("required value"))
            .iter()
            .find(|record| record["request"]["method"] == "px_getAccount")
            .unwrap_or_else(|| panic!("required value"));
        let selector = record["request"]["params"][0]
            .as_str()
            .unwrap_or_else(|| panic!("required value"));
        let transport = RecordedTransport::new(
            chain.address,
            BTreeMap::from([(
                format!("px_getAccount{}", record["request"]["params"]),
                record["response"]["result"].clone(),
            )]),
        );
        let fixture = CoreFixture::new();
        let config = fixture.config(
            "balance",
            &chain.url(),
            &transport.url(),
            unavailable_funding(),
        );
        let (status, body) =
            fixture.exchange(config, &get(&format!("/v1/accounts/{selector}/balance")));
        assert_eq!(status, 200, "{body}");
        assert_eq!(body["result"]["balance"], "1000000000000000000");
        assert_eq!(body["result"]["balance_unit"], "wei");
        assert_eq!(body["result"]["balance_decimals"], 18);
        assert_eq!(body["result"]["next_sequence"], "3");
        assert_eq!(body["result"]["height"], "2");
        assert_eq!(
            transport
                .requests
                .lock()
                .unwrap_or_else(|error| panic!("required value: {error:?}"))
                .len(),
            1
        );
    }

    #[test]
    fn proof_height_and_content_verifies_native_key_and_rejects_wrong_account_or_root() {
        let chain = NativeChain::new();
        let vector: Value = serde_json::from_str(include_str!(
            "../../../../../programs/tests/vectors/ics23-evm-storage-app-hash.json"
        ))
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        assert_eq!(vector["source"]["generator"], "modules/sequencer/keeper/ics23_evm_storage_vector_test.go::TestEvmStorageAppHashVector");
        let entry = &vector["entries"][0];
        let address = fixed_hex::<20>(
            "address",
            entry["address"]
                .as_str()
                .unwrap_or_else(|| panic!("required value")),
        )
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let slot = fixed_hex::<32>(
            "slot",
            entry["slot"]
                .as_str()
                .unwrap_or_else(|| panic!("required value")),
        )
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let root = fixed_hex::<32>(
            "app hash",
            entry["app_hash"]
                .as_str()
                .unwrap_or_else(|| panic!("required value")),
        )
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let height = entry["height"]
            .as_u64()
            .unwrap_or_else(|| panic!("required value"));
        let params = json!([
            format!("0x{}", hex_encode(&address)),
            [format!("0x{}", hex_encode(&slot))],
            format!("0x{height:x}")
        ]);
        let transport = RecordedTransport::new(
            chain.address,
            BTreeMap::from([(format!("eth_getProof{params}"), entry["response"].clone())]),
        );
        let endpoint = SequencerEndpoint::new(&transport.url(), &[])
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
        let witness = endpoint
            .storage_witness(&address, &[slot], height)
            .unwrap_or_else(|error| panic!("required value: {error:?}"))
            .remove(0);
        assert_eq!(verify_storage_witness(&witness, root), Ok(()));
        let mut wrong_account = witness.clone();
        wrong_account.address[0] ^= 1;
        assert!(verify_storage_witness(&wrong_account, root).is_err());
        let mut wrong_root = root;
        wrong_root[0] ^= 1;
        assert!(verify_storage_witness(&witness, wrong_root).is_err());
        let mut wrong_value = witness;
        wrong_value.value[0] ^= 1;
        assert!(verify_storage_witness(&wrong_value, root).is_err());
        let (original, original_root) = layerx_platform_core::native_committed_state(&endpoint, 1)
            .unwrap_or_else(|error| panic!("required value: {error:?}"));
        chain.select("advanced");
        let (historical, historical_root) =
            layerx_platform_core::native_committed_state(&endpoint, 1)
                .unwrap_or_else(|error| panic!("required value: {error:?}"));
        assert_eq!(historical, original);
        assert_eq!(historical_root, original_root);
        assert_eq!(historical.number, 1);
        assert!(
            endpoint
                .block_number()
                .unwrap_or_else(|error| panic!("required value: {error:?}"))
                > historical.number
        );
        let header = rpc(
            chain.address,
            "eth_getBlockByNumber",
            &json!(["0x1", false]),
        );
        let other_root = transcripts()["scenarios"]["balance"]
            ["eth_getBlockByNumber[\"0x2\",false]"]["stateRoot"]
            .clone();
        assert_ne!(header["stateRoot"], other_root);
        let mut changed_header = header.clone();
        changed_header["stateRoot"] = other_root;
        transport
            .sequences
            .lock()
            .unwrap_or_else(|error| panic!("required value: {error:?}"))
            .insert(
                "eth_getBlockByNumber[\"0x1\",false]".into(),
                VecDeque::from([header, changed_header]),
            );
        assert!(layerx_platform_core::native_committed_state(&endpoint, 1).is_err());
        assert_eq!(
            layerx_client::rpc::evidence::proof_bundle(&endpoint, address, &[slot], height),
            Err(layerx_client::rpc::evidence::EvidenceError::MissingCommittedHeader)
        );
    }
}
