include!("../src/main.rs");

#[cfg(test)]
mod repair_22_33_1451 {
    use super::*;
    use layerx_client::rpc::receipt::{Lookup, Rung};
    use layerx_platform_core::{SeedSigner, TreasurySigner as _};
    use layerx_platform_internal::evm_sender::EvmSender;
    use serde_json::{json, Value};
    use std::sync::atomic::AtomicBool;

    const TOKEN: &str = "repair-22-33-1451-local-test-token";
    const EVM_KEY: &str = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80";

    fn transcripts() -> Value {
        serde_json::from_str(include_str!("../src/repair_22_31_354_transcripts.json")).unwrap()
    }

    fn receipt_cases() -> Vec<Value> {
        let fixture: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))
        .unwrap();
        assert_eq!(
            fixture["provenance"]["generator"],
            "rpc/sequencer_receipt_vectors_test.go"
        );
        fixture["cases"].as_array().unwrap().clone()
    }

    // Replay committed producer captures over real HTTP. Missing exchanges are
    // errors; None deliberately drops the connection to exercise availability.
    struct Transport {
        address: SocketAddr,
        captures: Arc<Mutex<BTreeMap<String, Option<Value>>>>,
        requests: Arc<Mutex<Vec<Value>>>,
        stop: Arc<AtomicBool>,
        worker: Option<thread::JoinHandle<()>>,
    }

    impl Transport {
        fn new(scenario: &str) -> Self {
            let captures: BTreeMap<_, _> = transcripts()["scenarios"][scenario]
                .as_object()
                .unwrap()
                .iter()
                .map(|(key, value)| {
                    let offset = key.find('[').unwrap();
                    let params: Value = serde_json::from_str(&key[offset..]).unwrap();
                    (format!("{}{params}", &key[..offset]), Some(value.clone()))
                })
                .collect();
            let captures = Arc::new(Mutex::new(captures));
            let responses = Arc::clone(&captures);
            let requests = Arc::new(Mutex::new(Vec::new()));
            let observed = Arc::clone(&requests);
            let listener = TcpListener::bind("127.0.0.1:0").unwrap();
            let address = listener.local_addr().unwrap();
            listener.set_nonblocking(true).unwrap();
            let stop = Arc::new(AtomicBool::new(false));
            let stopping = Arc::clone(&stop);
            let worker = thread::spawn(move || {
                while !stopping.load(Ordering::Acquire) {
                    let (mut stream, _) = match listener.accept() {
                        Ok(pair) => pair,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            thread::sleep(Duration::from_millis(2));
                            continue;
                        }
                        Err(error) => panic!("recorded transport: {error}"),
                    };
                    stream
                        .set_read_timeout(Some(Duration::from_secs(5)))
                        .unwrap();
                    stream
                        .set_write_timeout(Some(Duration::from_secs(5)))
                        .unwrap();
                    let request =
                        layerx_platform_internal::http::parse_client_request(&mut stream).unwrap();
                    assert_eq!(request.method, "POST");
                    assert_eq!(request.path, "/");
                    let request: Value = serde_json::from_slice(&request.body).unwrap();
                    assert_eq!(request["jsonrpc"], "2.0");
                    observed.lock().unwrap().push(request.clone());
                    let key = format!(
                        "{}{}",
                        request["method"].as_str().unwrap(),
                        request["params"]
                    );
                    let result = responses
                        .lock()
                        .unwrap()
                        .get(&key)
                        .unwrap_or_else(|| panic!("exchange absent from capture: {key}"))
                        .clone();
                    if let Some(result) = result {
                        let body = json!({"jsonrpc":"2.0", "id":request["id"], "result":result})
                            .to_string();
                        write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).unwrap();
                        stream.flush().unwrap();
                    }
                }
            });
            Self {
                address,
                captures,
                requests,
                stop,
                worker: Some(worker),
            }
        }

        fn set(&self, method: &str, params: Value, response: Option<Value>) {
            self.captures
                .lock()
                .unwrap()
                .insert(format!("{method}{params}"), response);
        }

        fn url(&self) -> String {
            format!("http://{}/", self.address)
        }

        fn funding(&self, chain_id: u64) -> chain_fund::ChainFunding {
            chain_fund::ChainFunding::from_sender(
                NodeEndpoint {
                    host: self.address.ip().to_string(),
                    port: self.address.port(),
                },
                EvmSender::from_hex_secret(EVM_KEY).unwrap(),
                chain_id,
            )
        }

        fn case(&self, case: &Value) {
            let hash = case["receiptJson"]["txHash"].clone();
            self.set(
                "px_getReceiptKeys",
                json!([]),
                Some(json!({"keys":[case["pinnedKey"]]})),
            );
            self.set(
                "pax_sequencerReceipt",
                json!([hash]),
                Some(case["receiptJson"].clone()),
            );
            self.set(
                "eth_getTransactionReceipt",
                json!([hash]),
                Some(Value::Null),
            );
        }
    }

    impl Drop for Transport {
        fn drop(&mut self) {
            self.stop.store(true, Ordering::Release);
            self.worker.take().unwrap().join().unwrap();
        }
    }

    struct Core {
        root: PathBuf,
        tls: Arc<ServerConfig>,
        certificate: Vec<u8>,
    }

    impl Core {
        fn new() -> Self {
            static PROVIDER: std::sync::OnceLock<()> = std::sync::OnceLock::new();
            PROVIDER.get_or_init(|| install_provider().unwrap());
            let root = env::temp_dir().join(format!(
                "repair-1451-{}-{}",
                std::process::id(),
                TRACE.fetch_add(1, Ordering::Relaxed)
            ));
            fs::DirBuilder::new().mode(0o700).create(&root).unwrap();
            for name in ["journal", "funding-intents", "funding-receipts"] {
                fs::DirBuilder::new()
                    .mode(0o700)
                    .create(root.join(name))
                    .unwrap();
            }
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

        fn open(&self, transport: &Transport) -> Arc<Config> {
            let treasury = SeedSigner::new(&[9; 32]);
            Arc::new(Config {
                listen: "127.0.0.1:0".parse().unwrap(),
                admin_listen: "127.0.0.1:0".parse().unwrap(),
                tls: Arc::clone(&self.tls),
                admin_tls: Arc::clone(&self.tls),
                sequencer: SequencerEndpoint::new(&transport.url(), &[]).unwrap(),
                network: PxEndpoint::new(&transport.url(), &[]).unwrap(),
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
                receipt_events_token: Some(Zeroizing::new(TOKEN.into())),
                wallet_caps_token: None,
                admin_token: Zeroizing::new(TOKEN.into()),
                treasury_did: did_for_public_key(&treasury.public_key()),
                treasury: Arc::new(treasury),
                treasury_asset: [2; 32],
                funding: chain_fund::FundingSlot::configured(Ok(transport.funding(125))),
                sequencer_id: [4; 32],
                supervisor_socket: self.root.join("supervisor.sock"),
                state_dir: self.root.clone(),
                fee_limit: 1000,
                receipt_deadline: Duration::ZERO,
                admin_lock: Mutex::new(()),
                journal_lock: Mutex::new(()),
            })
        }

        fn exchange(
            &self,
            config: &Arc<Config>,
            request: &Request,
        ) -> (u16, Value, Option<String>) {
            let listener = TcpListener::bind("127.0.0.1:0").unwrap();
            let address = listener.local_addr().unwrap();
            let config = Arc::clone(config);
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
            tcp.set_read_timeout(Some(Duration::from_secs(10))).unwrap();
            tcp.set_write_timeout(Some(Duration::from_secs(10)))
                .unwrap();
            let connector = native_tls::TlsConnector::builder()
                .add_root_certificate(native_tls::Certificate::from_der(&self.certificate).unwrap())
                .build()
                .unwrap();
            let mut stream = connector.connect("localhost", tcp).unwrap();
            write!(
                stream,
                "{} {} HTTP/1.1\r\nHost: localhost\r\nContent-Length: {}\r\nConnection: close\r\n",
                request.method,
                request.path,
                request.body.len()
            )
            .unwrap();
            for (key, value) in &request.headers {
                write!(stream, "{key}: {value}\r\n").unwrap();
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
            let body = serde_json::from_slice(&response.body).unwrap();
            let retry = response.headers.get("retry-after").cloned();
            worker.join().unwrap();
            (status, body, retry)
        }
    }

    impl Drop for Core {
        fn drop(&mut self) {
            fs::remove_dir_all(&self.root).unwrap();
        }
    }

    fn get(path: &str) -> Request {
        Request {
            method: "GET".into(),
            path: path.into(),
            query: None,
            body: Vec::new(),
            headers: BTreeMap::from([("authorization".into(), format!("Bearer {TOKEN}"))]),
        }
    }

    fn hash(case: &Value) -> [u8; 32] {
        fixed_hex(
            "transaction",
            case["receiptJson"]["txHash"]
                .as_str()
                .unwrap()
                .strip_prefix("0x")
                .unwrap(),
        )
        .unwrap()
    }

    fn refusal_code(response: &Response) -> Value {
        serde_json::from_str::<Value>(&response.body).unwrap()["error"]["code"].clone()
    }

    #[test]
    fn funding_wrong_network_refuses_connection_readiness_and_signing() {
        let transport = Transport::new("funding_before");
        for wrong in [0, 1, 126, u64::MAX] {
            transport.set(
                "eth_chainId",
                json!([]),
                Some(json!(format!("0x{wrong:x}"))),
            );
            let slot = chain_fund::FundingSlot::configured(Ok(transport.funding(125)));
            assert!(
                matches!(slot.get(), Err("chain_unavailable")),
                "chain {wrong}"
            );
            assert_eq!(
                transport.funding(wrong).readiness(),
                Err("chain_unavailable")
            );
            let response = transport
                .funding(wrong)
                .sign_native([0x39; 20], 123456, 0, 21000, 1_000_000_000, 3_000_000_000)
                .expect_err("wrong network must not sign");
            assert_eq!(response.status, 503);
            assert_eq!(refusal_code(&response), "treasury_unavailable");
        }
        transport.set("eth_chainId", json!([]), Some(json!("0x7d")));
        let slot = chain_fund::FundingSlot::configured(Ok(transport.funding(125)));
        let connected = slot.get().unwrap();
        assert_eq!(connected.readiness().unwrap()["chain_id"], 125);
        let signed = connected
            .sign_native([0x39; 20], 123456, 0, 21000, 1_000_000_000, 3_000_000_000)
            .unwrap_or_else(|response| panic!("{}", response.body));
        assert_eq!(
            format!("0x{}", hex_encode(&signed.raw)),
            transcripts()["funding_raw"]
        );
        // A cached connection cannot authorize a signature after origin drift.
        transport.set("eth_chainId", json!([]), Some(json!("0x1")));
        assert_eq!(connected.readiness(), Err("chain_unavailable"));
        let response = connected
            .sign_native([0x39; 20], 123456, 1, 21000, 1_000_000_000, 3_000_000_000)
            .expect_err("drifted origin must not sign");
        assert_eq!(response.status, 503);
        assert!(transport.requests.lock().unwrap().iter().all(|request| {
            matches!(
                request["method"].as_str(),
                Some("eth_chainId" | "eth_blockNumber")
            )
        }));
    }

    #[test]
    fn funding_wrong_network_durable_intent_is_bound_on_reopen() {
        let transport = Transport::new("funding_before");
        let core = Core::new();
        let config = core.open(&transport);
        let command = FundingCommand {
            funding_id: "chain-bound".into(),
            did: format!("did:layerx:{}", "ab".repeat(32)),
            public_key: "ab".repeat(32),
            amount: 123456,
        };
        let request = Request {
            method: "POST".into(),
            path: "/admin/v1/testnet/fund".into(),
            query: None,
            headers: BTreeMap::from([("idempotency-key".into(), "chain-bound".into())]),
            body: serde_json::to_vec(&command).unwrap(),
        };
        let funding = config.funding.get().unwrap();
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
        let transfer = chain_fund::NativeTransfer::attest(
            signed.clone(),
            funding.sender_address(),
            config.treasury.as_ref(),
        )
        .unwrap();
        assert_eq!(transfer.chain_id, 125);
        funding_native::persist_prepared(
            &config,
            &request,
            &command,
            "chain-bound",
            transfer.clone(),
        )
        .unwrap_or_else(|response| panic!("{}", response.body));
        let sender = funding.sender_address();
        drop(config);
        let reopened = core.open(&transport);
        let recovered = prepare_funding(&reopened, &request, &command, "chain-bound")
            .unwrap_or_else(|response| panic!("{}", response.body));
        assert_eq!(recovered, transfer);
        let (status, exported, _) = core.exchange(
            &reopened,
            &get("/admin/v1/testnet/funding-intents/chain-bound"),
        );
        assert_eq!(status, 200, "{exported}");
        assert_eq!(exported["result"]["intent"]["transfer"]["chain_id"], 125);
        let mut foreign = signed;
        assert_eq!(foreign.raw[3], 125);
        foreign.raw[3] = 1;
        foreign.transaction_hash = format!(
            "0x{}",
            hex_encode(&layerx_crypto::evm_transaction::transaction_hash(
                &foreign.raw
            ))
        );
        assert!(chain_fund::NativeTransfer::attest(
            foreign.clone(),
            sender,
            reopened.treasury.as_ref()
        )
        .is_err());
        let path = funding_record_path(&reopened, "funding-intents", "chain-bound");
        let mut stored: Value =
            serde_json::from_slice(&funding_read(&path).unwrap().unwrap()).unwrap();
        stored["transfer"]["chain_id"] = json!(1);
        stored["transfer"]["signed"] = serde_json::to_value(&foreign).unwrap();
        let corrupted = serde_json::to_vec(&stored).unwrap();
        assert_eq!(
            funding_store(&path, &corrupted),
            Err("immutable funding record differs".to_owned())
        );
        // Simulate disk corruption outside the immutable store API, then
        // reopen the production reader and require it to reject the record.
        fs::write(&path, corrupted).unwrap();
        drop(reopened);
        let reopened = core.open(&transport);
        let response = prepare_funding(&reopened, &request, &command, "chain-bound")
            .expect_err("foreign durable transfer must not replay");
        assert_eq!(response.status, 503);
        assert_eq!(refusal_code(&response), "journal_unavailable");
        let (status, body, _) = core.exchange(
            &reopened,
            &get("/admin/v1/testnet/funding-intents/chain-bound"),
        );
        assert_eq!(status, 503, "{body}");
        assert_eq!(body["error"]["code"], "journal_unavailable");
    }

    #[test]
    fn absent_receipt_status_preserves_lookup_and_http_outcomes() {
        let transport = Transport::new("receipt");
        let core = Core::new();
        let config = core.open(&transport);
        let case = &receipt_cases()[0];
        let tx_hash = hash(case);
        let params = json!([case["receiptJson"]["txHash"]]);
        let path = format!("/v1/receipts/{}", hex_encode(&tx_hash));
        transport.set("pax_sequencerReceipt", params.clone(), Some(Value::Null));
        assert!(matches!(
            await_authenticated_receipt(&config, tx_hash, Duration::ZERO),
            Ok(Lookup::Absent)
        ));
        assert!(matches!(
            await_receipt(&config, tx_hash, Duration::from_millis(1)),
            Ok(ReceiptRead::Absent)
        ));
        let (status, body, retry) = core.exchange(&config, &get(&path));
        assert_eq!(status, 404, "{body}");
        assert_eq!(body["error"]["code"], "not_found");
        assert_eq!(retry, None);
        let wallet: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/wallet/test/fixtures/sequencer-receipt.json"
        ))
        .unwrap();
        transport.set(
            "pax_sequencerReceipt",
            params.clone(),
            Some(wallet["submitted"].clone()),
        );
        assert!(
            matches!(await_authenticated_receipt(&config, tx_hash, Duration::ZERO), Ok(Lookup::Submitted { tx_hash: answered, nonce: 0 }) if answered == tx_hash)
        );
        assert!(
            matches!(await_receipt(&config, tx_hash, Duration::ZERO), Ok(ReceiptRead::Submitted { tx_hash: answered, nonce: 0 }) if answered == tx_hash)
        );
        let (status, body, _) = core.exchange(&config, &get(&path));
        assert_eq!(status, 202, "{body}");
        assert_eq!(body["result"]["state"], "pending");
        assert_eq!(body["result"]["rung"], "submitted");
        assert_eq!(body["result"]["activity_id"], hex_encode(&tx_hash));
        assert_eq!(body["result"]["nonce"], "0");
        transport.set("pax_sequencerReceipt", params.clone(), None);
        assert!(await_authenticated_receipt(&config, tx_hash, Duration::ZERO).is_err());
        let (status, body, retry) = core.exchange(&config, &get(&path));
        assert_eq!(status, 503, "{body}");
        assert_eq!(body["error"]["code"], "receipt_unavailable");
        assert_eq!(retry.as_deref(), Some("5"));
        transport.set(
            "pax_sequencerReceipt",
            params,
            Some(case["receiptJson"].clone()),
        );
        let (status, body, _) = core.exchange(&config, &get(&path));
        assert_eq!(status, 200, "{body}");
        assert_eq!(
            body["result"]["receipt"],
            case["signedReceipt"]
                .as_str()
                .unwrap()
                .strip_prefix("0x")
                .unwrap()
        );
        transport.set("px_getReceiptKeys", json!([]), None);
        let (status, body, retry) = core.exchange(&config, &get(&path));
        assert_eq!(status, 503, "{body}");
        assert_eq!(body["error"]["code"], "receipt_unavailable");
        assert_eq!(retry.as_deref(), Some("5"));
    }

    #[test]
    fn receipt_event_authentication_accepts_producer_evidence_without_promoting_rung() {
        let transport = Transport::new("receipt");
        let core = Core::new();
        let config = core.open(&transport);
        for case in &receipt_cases()[..6] {
            transport.case(case);
            let tx_hash = hash(case);
            let authenticated =
                await_authenticated_receipt(&config, tx_hash, Duration::ZERO).unwrap();
            let Lookup::Verified(receipt) = authenticated else {
                panic!("producer receipt must verify")
            };
            assert_eq!(receipt.rung(), Rung::Receipted);
            let (status, body, _) = core.exchange(
                &config,
                &get(&format!(
                    "/internal/v1/receipt-events/{}",
                    hex_encode(&tx_hash)
                )),
            );
            assert_eq!(status, 200, "{}: {body}", case["id"]);
            let success = case["receiptJson"]["status"] == "0x1";
            assert_eq!(
                body["result"]["state"],
                if success { "completed" } else { "refused" }
            );
            assert_eq!(body["result"]["result_code"], if success { 0 } else { 1 });
            assert_eq!(body["result"]["rung"], "receipted");
            assert_eq!(body["result"]["activity_id"], hex_encode(&tx_hash));
            assert_eq!(
                body["result"]["global_sequence"],
                receipt.global_sequence().to_string()
            );
            assert_eq!(body["result"]["height"], receipt.height().to_string());
            assert_eq!(
                body["result"]["index"],
                receipt.execution().index.to_string()
            );
            assert_eq!(
                body["result"]["timestamp"],
                receipt.execution().timestamp.to_string()
            );
        }
        // Tamper only the upstream rung and covering-block claims. Those
        // fields cannot promote the original signed evidence to Comet commit.
        let mut committed = receipt_cases()[0].clone();
        committed["receiptJson"]["rung"] = json!("committed");
        let captures = transcripts();
        let landing = captures["scenarios"]["receipt"][format!(
            "eth_getTransactionReceipt{}",
            json!([committed["receiptJson"]["txHash"]])
        )]
        .clone();
        committed["receiptJson"]["blockNumber"] = landing["blockNumber"].clone();
        committed["receiptJson"]["blockHash"] = landing["blockHash"].clone();
        transport.case(&committed);
        let (status, body, _) = core.exchange(
            &config,
            &get(&format!(
                "/internal/v1/receipt-events/{}",
                hex_encode(&hash(&committed))
            )),
        );
        assert_eq!(status, 200, "{body}");
        assert_eq!(body["result"]["rung"], "receipted");
    }

    #[test]
    fn receipt_event_authentication_rejects_unpinned_and_mismatched_evidence() {
        let transport = Transport::new("receipt");
        let core = Core::new();
        let config = core.open(&transport);
        let cases = receipt_cases();
        for name in [
            "V3-key-not-pinned",
            "V3-height-outside-key-range",
            "V3-tampered-log",
            "V3-non-canonical-bytes",
        ] {
            let case = cases.iter().find(|case| case["id"] == name).unwrap();
            transport.case(case);
            let (status, body, retry) = core.exchange(
                &config,
                &get(&format!(
                    "/internal/v1/receipt-events/{}",
                    hex_encode(&hash(case))
                )),
            );
            assert_eq!(status, 503, "{name}: {body}");
            assert_eq!(body["error"]["code"], "receipt_events_unavailable");
            assert_eq!(retry.as_deref(), Some("5"));
            assert!(body.get("result").is_none());
        }
        let base = &cases[0];
        for (field, value) in [
            ("nonce", json!("0x1")),
            ("seq", json!("0x2")),
            ("height", json!("0x3e9")),
            ("index", json!("0x2")),
            ("timestamp", json!("0x1977426f681")),
            ("status", json!("0x0")),
            ("returnData", json!("0x00")),
            ("gasUsed", json!("0x5209")),
            ("from", json!(format!("0x{}", "11".repeat(20)))),
            ("signingKey", json!(format!("0x{}", "11".repeat(32)))),
        ] {
            let mut invalid = base.clone();
            invalid["receiptJson"][field] = value;
            transport.case(&invalid);
            let (status, body, _) = core.exchange(
                &config,
                &get(&format!(
                    "/internal/v1/receipt-events/{}",
                    hex_encode(&hash(base))
                )),
            );
            assert_eq!(status, 503, "{field}: {body}");
            assert_eq!(body["error"]["code"], "receipt_events_unavailable");
            assert!(body.get("result").is_none());
        }
    }

    #[test]
    fn receipt_event_authentication_keeps_authentication_absence_and_submission_distinct() {
        let transport = Transport::new("receipt");
        let core = Core::new();
        let config = core.open(&transport);
        let case = &receipt_cases()[0];
        let path = format!("/internal/v1/receipt-events/{}", hex_encode(&hash(case)));
        let mut request = get(&path);
        request.headers.clear();
        let (status, body, _) = core.exchange(&config, &request);
        assert_eq!(status, 401, "{body}");
        assert!(transport.requests.lock().unwrap().is_empty());
        let params = json!([case["receiptJson"]["txHash"]]);
        transport.set("pax_sequencerReceipt", params.clone(), Some(Value::Null));
        let (status, body, _) = core.exchange(&config, &get(&path));
        assert_eq!(status, 404, "{body}");
        assert_eq!(body["error"]["code"], "not_found");
        let wallet: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/wallet/test/fixtures/sequencer-receipt.json"
        ))
        .unwrap();
        transport.set(
            "pax_sequencerReceipt",
            params.clone(),
            Some(wallet["submitted"].clone()),
        );
        let (status, body, _) = core.exchange(&config, &get(&path));
        assert_eq!(status, 202, "{body}");
        assert_eq!(body["result"]["state"], "pending");
        assert_eq!(body["result"]["rung"], "submitted");
        transport.set("pax_sequencerReceipt", params, None);
        let (status, body, _) = core.exchange(&config, &get(&path));
        assert_eq!(status, 503, "{body}");
        assert_eq!(body["error"]["code"], "receipt_events_unavailable");
    }
}
