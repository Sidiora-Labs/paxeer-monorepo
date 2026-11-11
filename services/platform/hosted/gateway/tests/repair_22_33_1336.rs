include!("../src/main.rs");

mod repair_22_33_1336 {
    use super::*;
    use layerx_client::sequencer::SequencerEndpoint;
    use layerx_platform_internal::journal::Journal;
    use layerx_platform_internal::producer::{Observation, QueueState};
    use layerx_platform_internal::secret::unhex;
    use serde_json::{json, Value};
    use std::io::Read;
    use std::sync::atomic::AtomicBool;
    use std::thread::JoinHandle;

    struct Gateway {
        address: SocketAddr,
        stop: Arc<AtomicBool>,
        join: Option<JoinHandle<()>>,
    }

    fn config() -> Config {
        let certificate =
            rcgen::generate_simple_self_signed(vec!["localhost".into()]).expect("test certificate");
        let ca = Certificate::from_der(certificate.cert.der()).expect("outbound trust");
        let pin: Value =
            serde_json::from_str(include_str!("fixtures/repair_22_33_1335_committed.json"))
                .expect("recorded producer evidence");
        let key = &pin["pinnedKey"];
        Config {
            wallet_caps: None,
            routes: routes::Registry::configured("repair-22-33-1336", "1").expect("routes"),
            listen: "127.0.0.1:0".parse().expect("listen"),
            listener: Listener::Plain,
            client: Client::without_identity(ca.clone()),
            client_identity: false,
            kernel: None,
            identity: None,
            receipt_keys: Some(
                ReceiptKeyList::from_config(
                    &json!([{
                        "key_id": layerx_platform_internal::base64::encode(&unhex(key["keyId"].as_str().expect("key id").trim_start_matches("0x")).expect("key bytes")),
                        "public_key": layerx_platform_internal::base64::encode(&unhex(key["publicKey"].as_str().expect("public key").trim_start_matches("0x")).expect("public key bytes")),
                        "first_height": key["firstHeight"],
                        "last_height": key["lastHeight"],
                    }])
                    .to_string(),
                )
                .expect("real receipt pin parser"),
            ),
            committed_state: None,
            state_clock: now,
            paxeer: None,
            capabilities: capabilities::Cache::new(Duration::from_secs(1)),
            read_windows: Default::default(),
            indexer: None,
            registration_token: None,
            trusted_proxies: Vec::new(),
            faucet: None,
            store: Arc::new(RedisStore::new(
                RedisEndpoint::parse("rediss://localhost:1").expect("unused store address"),
                ca,
                Zeroizing::new("gateway".into()),
                Zeroizing::new("integration-only".into()),
            )),
            event_producer: false,
            network_id: "repair-22-33-1336".into(),
            wire_version: "1".into(),
            protocol_version: 1,
            protocol_network_id: 7,
            idempotency_seconds: 3600,
        }
    }

    impl Gateway {
        fn start(config: Config) -> Self {
            let listener = TcpListener::bind("127.0.0.1:0").expect("gateway loopback");
            let address = listener.local_addr().expect("gateway address");
            listener.set_nonblocking(true).expect("nonblocking gateway");
            let stop = Arc::new(AtomicBool::new(false));
            let stopped = Arc::clone(&stop);
            let config = Arc::new(config);
            let join = thread::spawn(move || {
                let mut connections = Vec::new();
                while !stopped.load(Ordering::Acquire) {
                    match listener.accept() {
                        Ok((mut stream, peer)) => {
                            let config = Arc::clone(&config);
                            connections.push(thread::spawn(move || {
                                stream
                                    .set_read_timeout(Some(Duration::from_secs(3)))
                                    .expect("read bound");
                                stream
                                    .set_write_timeout(Some(Duration::from_secs(3)))
                                    .expect("write bound");
                                exchange(&config, &mut stream, peer)
                                    .expect("production gateway exchange");
                            }));
                        }
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            thread::sleep(Duration::from_millis(2));
                        }
                        Err(error) => panic!("gateway accept: {error}"),
                    }
                }
                for connection in connections {
                    connection
                        .join()
                        .expect("gateway connection stopped cleanly");
                }
            });
            Self {
                address,
                stop,
                join: Some(join),
            }
        }

        fn exchange(&self, method: &str, path: &str, body: &[u8], headers: &str) -> (u16, Vec<u8>) {
            let mut stream = TcpStream::connect(self.address).expect("gateway connection");
            stream
                .set_read_timeout(Some(Duration::from_secs(3)))
                .expect("response bound");
            let connection = if headers.contains("Upgrade: websocket") {
                "Upgrade"
            } else {
                "close"
            };
            write!(stream, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: {connection}\r\n{headers}\r\n", body.len()).expect("request headers");
            stream.write_all(body).expect("request body");
            stream
                .shutdown(std::net::Shutdown::Write)
                .expect("finish request");
            let mut response = Vec::new();
            while !response.ends_with(b"\r\n\r\n") {
                let mut byte = [0];
                stream
                    .read_exact(&mut byte)
                    .expect("gateway response header");
                response.push(byte[0]);
                assert!(response.len() <= 16_384, "bounded response headers");
            }
            let head = std::str::from_utf8(&response).expect("HTTP headers");
            let status = head
                .split_whitespace()
                .nth(1)
                .expect("HTTP status")
                .parse()
                .expect("status code");
            let length = head
                .lines()
                .filter_map(|line| line.split_once(':'))
                .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
                .expect("response content length")
                .1
                .trim()
                .parse::<usize>()
                .expect("valid response length");
            assert!(length <= MAX_REQUEST, "bounded response body");
            let mut body = vec![0; length];
            stream.read_exact(&mut body).expect("gateway response body");
            (status, body)
        }

        fn rpc(&self, method: &str, params: Value) -> Value {
            let call = json!({"jsonrpc":"2.0", "id":9_007_199_254_740_993_u64, "method":method, "params":params});
            let (status, body) = self.exchange(
                "POST",
                "/",
                &serde_json::to_vec(&call).expect("request JSON"),
                "",
            );
            assert_eq!(status, 200);
            let answer: Value = serde_json::from_slice(&body).expect("RPC JSON");
            assert_eq!(answer["id"], call["id"]);
            assert_eq!(answer["jsonrpc"], "2.0");
            answer
        }

        fn retired(&self) {
            for path in ["/rpc", "/rpc/schema", "/rpc/ws", "/rpc/evm/ws"] {
                for method in ["GET", "POST"] {
                    let (status, body) = self.exchange(method, path, b"{}", "");
                    assert_eq!(status, 404, "{method} {path}");
                    let refusal: Value = serde_json::from_slice(&body).expect("refusal JSON");
                    assert_eq!(refusal["error"]["code"], "not_found");
                }
            }
            for path in ["/rpc/ws", "/rpc/evm/ws"] {
                let (status, body) = self.exchange(
                    "GET",
                    path,
                    b"",
                    "Upgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n",
                );
                assert_eq!(status, 404, "retired upgrade {path}");
                let refusal: Value = serde_json::from_slice(&body).expect("upgrade refusal JSON");
                assert_eq!(refusal["error"]["code"], "not_found");
            }
        }

        fn stop(mut self) {
            self.stop.store(true, Ordering::Release);
            self.join
                .take()
                .expect("running gateway")
                .join()
                .expect("gateway stopped cleanly");
        }
    }

    impl Drop for Gateway {
        fn drop(&mut self) {
            self.stop.store(true, Ordering::Release);
            if let Some(join) = self.join.take() {
                let _ = join.join();
            }
        }
    }

    #[test]
    fn receipt_key_router_path() {
        let gateway = Gateway::start(config());
        gateway.retired();
        let consumer = SequencerEndpoint::new(&format!("http://{}", gateway.address), &[])
            .expect("real SDK consumer");
        let keys = consumer.receipt_keys().expect("root receipt-key read");
        let recorded: Value =
            serde_json::from_str(include_str!("fixtures/repair_22_33_1335_committed.json"))
                .expect("recorded key");
        assert_eq!(keys.len(), 1);
        assert_eq!(
            keys[0].key_id.as_slice(),
            unhex(
                recorded["pinnedKey"]["keyId"]
                    .as_str()
                    .expect("id")
                    .trim_start_matches("0x")
            )
            .expect("key bytes")
        );
        assert_eq!(
            keys[0].public_key.as_slice(),
            unhex(
                recorded["pinnedKey"]["publicKey"]
                    .as_str()
                    .expect("public key")
                    .trim_start_matches("0x")
            )
            .expect("public key bytes")
        );
        drop(consumer);
        for params in [json!(["5"]), json!({}), Value::Null] {
            let answer = gateway.rpc("px_getReceiptKeys", params);
            assert_eq!(answer["error"]["code"], -32602);
            assert!(answer.get("result").is_none());
        }
        gateway.stop();
        let mut unpinned = config();
        unpinned.receipt_keys = None;
        let gateway = Gateway::start(unpinned);
        let answer = gateway.rpc("px_getReceiptKeys", json!([]));
        assert_eq!(answer["error"]["code"], -32003);
        assert_eq!(answer["error"]["data"]["code"], "receipt_keys_unavailable");
        assert!(answer.get("result").is_none());
        gateway.stop();
    }

    #[test]
    fn surface_router_path() {
        let config = config();
        for path in ["/rpc", "/rpc/schema", "/rpc/ws", "/rpc/evm/ws"] {
            for method in ["GET", "POST"] {
                let request = IncomingRequest {
                    method: method.into(),
                    path: path.into(),
                    headers: Default::default(),
                    body: Vec::new(),
                };
                assert_eq!(
                    rpc::route(&config, &request).status,
                    404,
                    "direct producer {method} {path}"
                );
            }
        }
        let gateway = Gateway::start(config);
        gateway.retired();
        let schema: Value =
            serde_json::from_slice(include_bytes!("../openrpc.json")).expect("distributed OpenRPC");
        for method in layerx_platform_gateway::ROUTER_METHODS {
            assert_eq!(
                schema["methods"]
                    .as_array()
                    .expect("methods")
                    .iter()
                    .filter(|entry| entry["name"] == method)
                    .count(),
                1,
                "{method}"
            );
            let answer = gateway.rpc(method, json!([]));
            assert_ne!(answer["error"]["code"], -32601, "{method}: {answer}");
        }
        assert_eq!(
            gateway.rpc("px_getReceiptKeys", json!([]))["result"]["keys"]
                .as_array()
                .expect("published keys")
                .len(),
            1
        );
        for method in [
            "lx_getNodeInfo",
            "lx_register",
            "lx_subscribe",
            "lx_unsubscribe",
            "lx_unknown",
        ] {
            let answer = gateway.rpc(method, json!([]));
            assert_eq!(answer["error"]["code"], -32601);
            assert_eq!(answer["error"]["message"], "Method not found");
            assert!(answer.get("result").is_none());
        }
        gateway.stop();
    }

    #[track_caller]
    fn reopen_queue(directory: &std::path::Path) -> (Journal, QueueState) {
        let mut recovered = QueueState::default();
        let deadline = std::time::Instant::now() + Duration::from_secs(3);
        let journal = loop {
            match Journal::open(directory, |state: QueueState| recovered = state) {
                Ok(journal) => break journal,
                Err(error)
                    if error.starts_with(layerx_platform_internal::journal::LOCK_REFUSAL)
                        && std::time::Instant::now() < deadline =>
                {
                    thread::sleep(Duration::from_millis(2));
                }
                Err(error) => panic!("reopen production journal: {error}"),
            }
        };
        recovered.validate().expect("recovered producer state");
        (journal, recovered)
    }

    #[test]
    fn tap_recovery_proof_gap_outbox_consumer() {
        use layerx_platform_internal::{events::Fact, secret};
        let document: Value =
            serde_json::from_str(include_str!("fixtures/maintained-authority.json"))
                .expect("recorded receipt");
        let receipt_bytes = secret::unhex(document["receipt_hex"].as_str().expect("receipt bytes"))
            .expect("receipt hex");
        let decoded = layerx_wire::receipt::decode(&receipt_bytes).expect("real receipt consumer");
        let receipt = decoded.protocol().expect("protocol receipt");
        let resource = secret::hex(&receipt.activity_id());
        let observation = Observation {
            kind: "payment".into(),
            id: String::new(),
            principal: None,
            principal_digest: Some(secret::sha256_hex(b"payment-principal")),
            resource: resource.clone(),
            sequence: 0,
            source_sequence: receipt.global_sequence(),
            occurred_at: receipt.timestamp(),
            facts: vec![Fact {
                name: "result_code".into(),
                value: receipt.result_code().to_string(),
            }],
            activity_id: Some(resource),
            amount: Some(receipt.amount().to_string()),
            asset: Some(secret::hex(&receipt.asset())),
        };
        let directory = tempfile::tempdir().expect("persisted outbox directory");
        let (mut journal, mut queue) = reopen_queue(directory.path());
        let original = queue
            .enqueue("tap-payment", observation.clone())
            .expect("real producer enqueue");
        let bytes = original.encode().expect("producer transport bytes");
        journal.append(&queue).expect("persist pending event");
        drop(queue);
        drop(journal);
        let (mut journal, mut queue) = reopen_queue(directory.path());
        assert_eq!(
            queue.enqueue("tap-payment", observation.clone()),
            Ok(original.clone())
        );
        let mut changed = observation.clone();
        changed.amount = Some("2".into());
        assert_ne!(changed.amount, observation.amount);
        assert!(queue.enqueue("tap-payment", changed).is_err());
        let pending = queue.pending().expect("recovered pending event");
        assert_eq!(
            pending.observation.encode().expect("recovered bytes"),
            bytes
        );
        assert!(!pending.observed);
        assert!(queue.acknowledge(&original.id, false).is_err());
        queue
            .acknowledge(&original.id, true)
            .expect("source acknowledgement");
        journal
            .append(&queue)
            .expect("persist source acknowledgement");
        drop(queue);
        drop(journal);
        let (mut journal, mut queue) = reopen_queue(directory.path());
        assert!(queue.pending().expect("pending webhook event").observed);
        queue
            .acknowledge(&original.id, false)
            .expect("webhook acknowledgement");
        journal
            .append(&queue)
            .expect("persist full acknowledgement");
        drop(queue);
        drop(journal);
        let (_journal, mut queue) = reopen_queue(directory.path());
        assert!(queue.pending().is_none());
        assert_eq!(queue.enqueue("tap-payment", observation), Ok(original));
        assert!(
            queue.pending().is_none(),
            "retry cannot duplicate acknowledged event"
        );
    }

    #[test]
    fn unified_paths_retired_upgrades() {
        let gateway = Gateway::start(config());
        gateway.retired();
        let schema: Value = serde_json::from_slice(include_bytes!("../openrpc.json"))
            .expect("maintained OpenRPC distribution");
        let names: Vec<_> = schema["methods"]
            .as_array()
            .expect("methods")
            .iter()
            .map(|entry| entry["name"].as_str().expect("method name"))
            .collect();
        assert!(names.contains(&"px_register"));
        assert!(names.contains(&"px_getHistory"));
        assert!(names.iter().all(|name| !name.starts_with("lx_")));
        let (status, _) = gateway.exchange("GET", "/", b"", "Upgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n");
        // The maintained subscription handler must be registered before it can
        // authenticate a client; a missing route is not an authentication refusal.
        assert_eq!(
            status, 401,
            "maintained root subscription transport must reach authentication"
        );
        gateway.stop();
    }
}
