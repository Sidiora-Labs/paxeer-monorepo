include!("../src/main.rs");

use chain_readiness_tests::{base64, must, ReceiptHarness};
use layerx_platform_authority::assertion::Must;
use serde_json::{json as document, Value};
use std::io::{BufRead as _, BufReader};
use std::os::unix::fs::PermissionsExt as _;
use std::process::{Child, Command, Stdio};
use std::sync::Mutex;

fn corpus() -> Value {
    must(serde_json::from_str(include_str!(
        "fixtures/committed-node.json"
    )))
}

struct RecordedNode {
    address: SocketAddr,
    responses: Arc<Mutex<Vec<Value>>>,
    calls: Arc<Mutex<Vec<Value>>>,
    stopped: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}

impl RecordedNode {
    fn start() -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let address = must(listener.local_addr());
        let responses = Arc::new(Mutex::new(corpus()["responses"].as_array().must().clone()));
        let calls = Arc::new(Mutex::new(Vec::new()));
        let stopped = Arc::new(AtomicBool::new(false));
        let replies = responses.clone();
        let seen = calls.clone();
        let stop = stopped.clone();
        let worker = thread::spawn(move || {
            while !stop.load(Ordering::Acquire) {
                let (tcp, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("recorded node transport: {error}"),
                };
                must(tcp.set_read_timeout(Some(IO_TIMEOUT)));
                let mut reader = BufReader::new(tcp);
                let mut line = String::new();
                must(reader.read_line(&mut line));
                assert!(line.starts_with("POST /evm HTTP/1."));
                let mut length = None;
                loop {
                    line.clear();
                    assert_ne!(must(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(must(value.trim().parse::<usize>()));
                    }
                }
                let mut bytes = vec![0; length.must()];
                must(reader.read_exact(&mut bytes));
                let request: Value = must(serde_json::from_slice(&bytes));
                let result = {
                    let replies = must(replies.lock());
                    replies
                        .iter()
                        .find(|row| {
                            row["method"] == request["method"] && row["params"] == request["params"]
                        })
                        .map(|row| row["result"].clone())
                        .unwrap_or_else(|| {
                            assert_eq!(
                                request["method"], "pax_sequencerReceipt",
                                "unrecorded request: {request}"
                            );
                            Value::Null
                        })
                };
                must(seen.lock()).push(request.clone());
                let answer =
                    document!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
                let mut tcp = reader.into_inner();
                must(write!(
                    tcp,
                    "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",
                    answer.len()
                ));
            }
        });
        Self {
            address,
            responses,
            calls,
            stopped,
            worker: Some(worker),
        }
    }

    fn change(&self, method: &str, change: impl Fn(&mut Value)) {
        for row in must(self.responses.lock())
            .iter_mut()
            .filter(|row| row["method"] == method)
        {
            change(&mut row["result"]);
        }
    }
}

impl Drop for RecordedNode {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        self.worker.take().must().join().must();
    }
}

struct Authority {
    child: Child,
    address: SocketAddr,
}

fn startup(harness: &ReceiptHarness, node: &RecordedNode) -> Command {
    let token = harness.directory.join("service-token");
    must(fs::write(&token, "merged-authority-service-token"));
    must(fs::set_permissions(
        &token,
        fs::Permissions::from_mode(0o600),
    ));
    let pin = corpus()["pinned_key"].clone();
    let keys = document!([{"keyId":base64(&chain_readiness_tests::hex_bytes(&pin["keyId"])),
        "publicKey":base64(&chain_readiness_tests::hex_bytes(&pin["publicKey"])),
        "firstHeight":pin["firstHeight"],"lastHeight":pin["lastHeight"]}]);
    let keys_file = harness.directory.join("node-keys.json");
    must(fs::write(&keys_file, keys.to_string()));
    must(fs::set_permissions(
        &keys_file,
        fs::Permissions::from_mode(0o600),
    ));
    let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-receipt-authority"));
    command
        .env_clear()
        .envs(
            env::vars_os()
                .filter(|(name, _)| !name.to_string_lossy().starts_with("LAYERX_AUTHORITY_")),
        )
        .env(
            "LAYERX_AUTHORITY_TLS_CERT_DER",
            harness.directory.join("tls-cert.der"),
        )
        .env(
            "LAYERX_AUTHORITY_TLS_KEY_DER",
            harness.directory.join("tls-key.der"),
        )
        .env("LAYERX_AUTHORITY_TOKEN_FILES", token)
        .env(
            "LAYERX_AUTHORITY_CHAIN_RPC_URL",
            format!("http://{}/evm", node.address),
        )
        .env("LAYERX_AUTHORITY_RECEIPT_KEYS_FILE", keys_file)
        .env("LAYERX_AUTHORITY_NETWORK_ID", "merged-authority-test")
        .env("LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID", "125")
        .env(
            "LAYERX_AUTHORITY_SEQUENCER_ID",
            pin["keyId"].as_str().must().trim_start_matches("0x"),
        )
        .env(
            "LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY",
            pin["publicKey"].as_str().must().trim_start_matches("0x"),
        );
    command
}

impl Authority {
    fn start(mut command: Command) -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let address = must(listener.local_addr());
        drop(listener);
        let log = must(fs::File::create(
            std::env::temp_dir().join(format!("authority-start-{}.log", address.port())),
        ));
        let child = must(
            command
                .env("LAYERX_AUTHORITY_LISTEN", address.to_string())
                .stdout(Stdio::null())
                .stderr(log)
                .spawn(),
        );
        let mut authority = Self { child, address };
        let deadline = Instant::now() + Duration::from_secs(5);
        while TcpStream::connect(address).is_err() {
            assert!(
                must(authority.child.try_wait()).is_none(),
                "authority startup refused"
            );
            assert!(Instant::now() < deadline, "authority startup deadline");
            thread::sleep(Duration::from_millis(10));
        }
        authority
    }

    fn get(&self, path: &str) -> (u16, Value, String) {
        let mut roots = RootCertStore::empty();
        must(roots.add(CertificateDer::from(
            include_bytes!("../src/human/fixtures/ca.der").to_vec(),
        )));
        let client = rustls::ClientConfig::builder()
            .with_root_certificates(roots)
            .with_no_client_auth();
        let connection = must(rustls::ClientConnection::new(
            Arc::new(client),
            must(rustls::pki_types::ServerName::try_from("localhost")),
        ));
        let tcp = must(TcpStream::connect(self.address));
        must(tcp.set_read_timeout(Some(Duration::from_secs(12))));
        let mut stream = StreamOwned::new(connection, tcp);
        must(write!(stream,"GET {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer merged-authority-service-token\r\nConnection: close\r\n\r\n"));
        let mut answer = Vec::new();
        must(stream.read_to_end(&mut answer));
        let end = answer
            .windows(4)
            .position(|bytes| bytes == b"\r\n\r\n")
            .must();
        let header = must(std::str::from_utf8(&answer[..end])).to_owned();
        let status = must(header.split_whitespace().nth(1).must().parse());
        let body = &answer[end + 4..];
        let document = if body.is_empty() {
            document!({"error":{"status":status,"code":"empty_response"}})
        } else {
            must(serde_json::from_slice(body))
        };
        (status, document, header)
    }
}
impl Drop for Authority {
    fn drop(&mut self) {
        must(self.child.kill());
        must(self.child.wait());
    }
}

fn harness() -> ReceiptHarness {
    let vectors = chain_readiness_tests::receipt_vectors();
    let case = chain_readiness_tests::vector(&vectors, "V1-pax-transfer");
    ReceiptHarness::start(std::slice::from_ref(&case), &case)
}

fn activity_path(prefix: &str) -> String {
    let rows = corpus();
    let receipt = rows["responses"]
        .as_array()
        .must()
        .iter()
        .find(|row| row["method"] == "pax_sequencerReceipt")
        .must();
    format!(
        "{prefix}{}",
        receipt["result"]["txHash"]
            .as_str()
            .must()
            .trim_start_matches("0x")
    )
}

#[test]
fn node_alone_starts_ready_and_serves_committed_receipts() {
    let harness = harness();
    let node = RecordedNode::start();
    let authority = Authority::start(startup(&harness, &node));
    let (status, ready, headers) = authority.get("/readyz");
    assert_eq!(status, 200);
    assert_eq!(
        ready,
        document!({"ready":true,"network_id":"merged-authority-test","protocol_network_id":125,"wire_version":PROTOCOL_VERSION.to_string()})
    );
    assert!(headers.contains("X-LayerX-Authority-Replica: not_configured\r\n"));
    assert!(headers.contains("X-LayerX-Authority-Chain: ready\r\n"));
    let corpus = corpus();
    let receipt = &corpus["responses"]
        .as_array()
        .must()
        .iter()
        .find(|row| row["method"] == "pax_sequencerReceipt")
        .must()["result"];
    let bytes = chain_readiness_tests::hex_bytes(&receipt["receipt"]);
    let decoded = must(layerx_wire::receipt::decode(&bytes));
    let protocol = decoded.protocol().must();
    for path in [
        activity_path("/v1/authorized-batches/by-activity/"),
        activity_path("/v1/authorized-batches/wait-by-activity/"),
        format!("{}/authority", activity_path("/internal/v1/activities/")),
    ] {
        let (status, answer, _) = authority.get(&path);
        assert_eq!(status, 200, "{answer}");
        assert_eq!(
            answer,
            document!({"activity_id":hex::encode(&protocol.activity_id()),"receipt":hex::encode(&bytes),
            "batch_id":hex::encode(&protocol.batch_id()),"asset":hex::encode(&protocol.asset()),
            "previous_state_root":hex::encode(&protocol.previous_state_root()),"resulting_state_root":hex::encode(&protocol.resulting_state_root()),
            "sequencer_public_key":corpus["pinned_key"]["publicKey"].as_str().must().trim_start_matches("0x"),
            "network_id":"merged-authority-test","protocol_network_id":125,"wire_version":PROTOCOL_VERSION.to_string()})
        );
        assert!(answer.get("batch_evidence").is_none());
    }
    let endpoint = must(SequencerEndpoint::new(
        &format!("http://{}/evm", node.address),
        &[],
    ));
    let block = must(endpoint.block_by_number(10)).must();
    assert_eq!(
        block.transactions,
        vec![chain_readiness_tests::hex_bytes(&corpus["raw_transaction"])]
    );
    for path in [
        committed_head_route::HEAD_PATH,
        "/v1/protocol/account-state/head?height=10",
    ] {
        let (status, head, _) = authority.get(path);
        assert_eq!(status, 200, "{head}");
        assert_eq!(head["height"], "10");
        assert_eq!(head["block_hash"], hex::encode(&block.hash));
        assert_eq!(head["commitment"], "native_app_hash");
        assert!(head.get("authority_replica_id").is_none());
        assert!(head.get("chain_evidence").is_none());
    }
    assert_eq!(
        authority
            .get("/v1/protocol/account-state/head?height=010")
            .0,
        400
    );
    assert_eq!(
        authority.get("/v1/protocol/account-state/head?height=11").0,
        503
    );
    let (status, retired, _) = authority.get(&format!(
        "/v1/batches/{}/receipt-authority?receipt_digest={}",
        hex::encode(&[1; 32]),
        hex::encode(&[2; 32])
    ));
    assert_eq!(status, 410);
    assert_eq!(retired["error"]["code"], "retired_http_contract");
    assert!(must(node.calls.lock())
        .iter()
        .any(|call| call["method"] == "px_getCommit"));
}

#[test]
fn partial_origin_group_names_every_missing_setting() {
    let harness = harness();
    let node = RecordedNode::start();
    for present in REPLICA_SETTINGS {
        let output = must(startup(&harness, &node).env(present, "present").output());
        assert!(!output.status.success());
        let message = must(String::from_utf8(output.stderr));
        assert!(message.contains("batch evidence origin group"), "{message}");
        for missing in REPLICA_SETTINGS.into_iter().filter(|name| *name != present) {
            assert!(message.contains(missing), "{message}");
        }
    }
}

#[test]
fn node_receipts_refuse_signature_height_and_landing_substitution() {
    let harness = harness();
    for fault in [
        "signature",
        "key_height",
        "height_zero",
        "index",
        "transaction",
        "block_hash",
        "status",
        "commit",
        "app_hash",
        "uncommitted",
    ] {
        let node = RecordedNode::start();
        let command = startup(&harness, &node);
        match fault {
            "signature" => node.change("pax_sequencerReceipt", |receipt| {
                let mut bytes = chain_readiness_tests::hex_bytes(&receipt["receipt"]);
                let last = bytes.len() - 1;
                bytes[last] ^= 1;
                receipt["receipt"] = document!(format!("0x{}", hex::encode(&bytes)));
            }),
            "key_height" => {
                let path = harness.directory.join("node-keys.json");
                let mut keys: Value = must(serde_json::from_slice(&must(fs::read(&path))));
                keys[0]["firstHeight"] = document!(11);
                keys[0]["lastHeight"] = document!(12);
                must(fs::write(path, keys.to_string()));
            }
            "height_zero" => node.change("pax_sequencerReceipt", |receipt| {
                receipt["height"] = document!("0x0")
            }),
            "index" => node.change("eth_getTransactionReceipt", |receipt| {
                receipt["transactionIndex"] = document!("0x1")
            }),
            "transaction" => node.change("eth_getBlockByNumber", |block| {
                block["transactions"][0]["value"] = document!("0x1")
            }),
            "block_hash" => node.change("pax_sequencerReceipt", |receipt| {
                receipt["blockHash"] = document!(format!("0x{}", hex::encode(&[9; 32])))
            }),
            "status" => node.change("eth_getTransactionReceipt", |receipt| {
                receipt["status"] = document!("0x0")
            }),
            "commit" => node.change("px_getCommit", |commit| {
                commit["signedHeader"]["commit"]["block_id"]["hash"] =
                    document!(hex::encode(&[9; 32]))
            }),
            "app_hash" => node.change("px_getCommit", |commit| {
                commit["signedHeader"]["header"]["app_hash"] = document!(hex::encode(&[9; 32]))
            }),
            "uncommitted" => node.change("pax_sequencerReceipt", |receipt| {
                receipt["rung"] = document!("receipted");
                receipt.as_object_mut().must().remove("blockNumber");
                receipt.as_object_mut().must().remove("blockHash");
            }),
            _ => unreachable!(),
        }
        let authority = Authority::start(command);
        let (status, answer, _) =
            authority.get(&activity_path("/v1/authorized-batches/by-activity/"));
        assert_eq!(
            status,
            if ["signature", "key_height", "height_zero", "uncommitted"].contains(&fault) {
                503
            } else {
                502
            },
            "{fault}: {answer}"
        );
        assert!(answer.get("receipt").is_none(), "{fault}: {answer}");
    }
}

#[test]
fn full_origin_group_preserves_replica_readiness_and_evidence_contract() {
    let harness = harness();
    let node = RecordedNode::start();
    let token = harness.directory.join("origin-token");
    must(fs::write(&token, "origin-test-token"));
    let mut command = startup(&harness, &node);
    command
        .env(
            "LAYERX_AUTHORITY_CHAIN_RPC_URL",
            &harness.config.chain_rpc_url,
        )
        .env(
            "LAYERX_AUTHORITY_RECEIPT_KEYS_FILE",
            harness.directory.join("receipt-keys.json"),
        )
        .env(
            "LAYERX_AUTHORITY_SEQUENCER_ID",
            hex::encode(
                &harness
                    .config
                    .replica
                    .as_ref()
                    .must()
                    .authorization
                    .sequencer_id(),
            ),
        )
        .env(
            "LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY",
            hex::encode(&harness.config.sequencer_public_key),
        );
    command
        .env(
            "LAYERX_AUTHORITY_REPLICA_URL",
            format!("http://{}", harness.config.replica.as_ref().must().address),
        )
        .env("LAYERX_AUTHORITY_REPLICA_ID", hex::encode(&[1; 32]))
        .env("LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE", token)
        .env("LAYERX_AUTHORITY_FIRST_BATCH", "1")
        .env("LAYERX_AUTHORITY_LAST_BATCH", "100");
    let authority = Authority::start(command);
    let (status, ready, headers) = authority.get("/readyz");
    assert_eq!(status, 200, "{ready}");
    assert!(headers.contains("X-LayerX-Authority-Replica: ready\r\n"));
    let vectors = chain_readiness_tests::receipt_vectors();
    let original = chain_readiness_tests::vector(&vectors, "V1-pax-transfer");
    let path = format!(
        "/v1/authorized-batches/by-activity/{}",
        original["receiptJson"]["txHash"]
            .as_str()
            .must()
            .trim_start_matches("0x")
    );
    let (status, answer, _) = authority.get(&path);
    assert_eq!(status, 503);
    assert_eq!(answer["error"]["code"], "replica_evidence_unavailable");
    assert_eq!(
        authority
            .get(&format!(
                "/v1/batches/{}/receipt-authority?receipt_digest={}",
                hex::encode(&[1; 32]),
                hex::encode(&[2; 32])
            ))
            .0,
        404
    );
}

#[test]
fn node_waits_for_commit_and_keeps_the_activity_deadline() {
    let harness = harness();
    let node = RecordedNode::start();
    let original = must(node.responses.lock()).clone();
    node.change("pax_sequencerReceipt", |receipt| {
        receipt["rung"] = document!("receipted");
        receipt.as_object_mut().must().remove("blockNumber");
        receipt.as_object_mut().must().remove("blockHash");
    });
    let authority = Authority::start(startup(&harness, &node));
    let replies = node.responses.clone();
    let worker = thread::spawn(move || {
        thread::sleep(Duration::from_millis(150));
        *must(replies.lock()) = original;
    });
    let start = Instant::now();
    let (status, answer, _) =
        authority.get(&activity_path("/v1/authorized-batches/wait-by-activity/"));
    assert_eq!(status, 200, "{answer}");
    assert!(start.elapsed() >= Duration::from_millis(100));
    worker.join().must();
    let start = Instant::now();
    let (status, answer, _) = authority.get(&format!(
        "/v1/authorized-batches/wait-by-activity/{}",
        hex::encode(&[0x42; 32])
    ));
    assert_eq!(status, 503, "{answer}");
    assert_eq!(answer["error"]["code"], "receipt_source_unavailable");
    assert!(start.elapsed() >= Duration::from_millis(7900));
    assert!(start.elapsed() < Duration::from_secs(9));
}
