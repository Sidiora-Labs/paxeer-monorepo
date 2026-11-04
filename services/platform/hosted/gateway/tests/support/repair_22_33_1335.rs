use layerx_client::sequencer::{SequencerEndpoint, SequencerError, SequencerReceipt};
use layerx_platform_gateway::http::{read_request, IncomingRequest};
use layerx_platform_internal::http;
use rustls::{ServerConfig, ServerConnection, StreamOwned};
use serde_json::{json, Value};
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

type TlsStream = StreamOwned<ServerConnection, TcpStream>;

pub fn committed() -> Value {
    serde_json::from_str(include_str!("../fixtures/repair_22_33_1335_committed.json"))
        .expect("recorded node evidence")
}

pub fn unhex(value: &Value) -> Vec<u8> {
    let text = value.as_str().expect("hex string");
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| u8::from_str_radix(std::str::from_utf8(pair).expect("ASCII"), 16).expect("hex"))
        .collect()
}

struct Peer {
    port: u16,
    stop: Arc<AtomicBool>,
    join: Option<JoinHandle<()>>,
}

impl Peer {
    fn start(
        tls: Arc<ServerConfig>,
        handler: impl Fn(&mut TlsStream) -> bool + Send + Sync + 'static,
    ) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback peer");
        let port = listener.local_addr().expect("peer address").port();
        listener.set_nonblocking(true).expect("nonblocking peer");
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let handler = Arc::new(handler);
        let join = thread::spawn(move || {
            let mut connections = Vec::new();
            while !stopped.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((tcp, _)) => {
                        let tls = Arc::clone(&tls);
                        let handler = Arc::clone(&handler);
                        connections.push(thread::spawn(move || {
                            tcp.set_read_timeout(Some(Duration::from_secs(3)))
                                .expect("read bound");
                            tcp.set_write_timeout(Some(Duration::from_secs(3)))
                                .expect("write bound");
                            let connection = ServerConnection::new(tls).expect("TLS connection");
                            let mut stream = StreamOwned::new(connection, tcp);
                            if handler(&mut stream) {
                                stream.conn.send_close_notify();
                                stream.flush().expect("TLS close");
                            }
                        }));
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2))
                    }
                    Err(error) => panic!("peer accept: {error}"),
                }
            }
            for connection in connections {
                connection.join().expect("peer assertions");
            }
        });
        Self {
            port,
            stop,
            join: Some(join),
        }
    }

    fn http(
        tls: Arc<ServerConfig>,
        handler: impl Fn(&IncomingRequest) -> http::Response + Send + Sync + 'static,
    ) -> Self {
        Self::start(tls, move |stream| {
            let request =
                read_request(stream, http::MAX_REQUEST_BYTES).expect("real HTTP request codec");
            http::write_response(stream, &handler(&request)).expect("peer HTTP response");
            true
        })
    }

    fn url(&self) -> String {
        format!("https://localhost:{}", self.port)
    }
}

impl Drop for Peer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(join) = self.join.take() {
            join.join().expect("peer completed");
        }
    }
}

fn resp(reader: &mut impl BufRead) -> Option<Vec<String>> {
    let mut line = String::new();
    match reader.read_line(&mut line) {
        Ok(0) => return None,
        Err(error) if error.kind() == std::io::ErrorKind::UnexpectedEof => return None,
        Ok(_) => {}
        Err(error) => panic!("RESP array: {error}"),
    }
    let count: usize = line
        .strip_prefix('*')
        .expect("RESP array marker")
        .trim()
        .parse()
        .expect("array length");
    let mut parts = Vec::new();
    for _ in 0..count {
        line.clear();
        reader.read_line(&mut line).expect("bulk length");
        let length: usize = line
            .strip_prefix('$')
            .expect("bulk marker")
            .trim()
            .parse()
            .expect("bulk length");
        let mut bytes = vec![0; length + 2];
        reader.read_exact(&mut bytes).expect("bulk data");
        assert_eq!(&bytes[length..], b"\r\n");
        parts.push(String::from_utf8(bytes[..length].to_vec()).expect("UTF-8 command"));
    }
    Some(parts)
}

pub struct Fixture {
    directory: tempfile::TempDir,
    tls: Arc<ServerConfig>,
    store: Peer,
}

impl Fixture {
    pub fn start() -> Self {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let directory = tempfile::tempdir().expect("fixture directory");
        let certificate = rcgen::generate_simple_self_signed(vec!["localhost".into()])
            .expect("local certificate");
        fs::write(directory.path().join("ca.der"), certificate.cert.der()).expect("CA file");
        let tls = Arc::new(
            ServerConfig::builder()
                .with_no_client_auth()
                .with_single_cert(
                    vec![certificate.cert.der().clone()],
                    rustls::pki_types::PrivatePkcs8KeyDer::from(
                        certificate.signing_key.serialize_der(),
                    )
                    .into(),
                )
                .expect("TLS peer configuration"),
        );
        // This dependency peer exercises the production Redis client's AUTH/PING
        // transport. It has no application storage or receipt production role.
        let store = Peer::start(Arc::clone(&tls), |stream| {
            let mut reader = BufReader::new(stream);
            assert_eq!(
                resp(&mut reader),
                Some(vec![
                    "AUTH".into(),
                    "gateway".into(),
                    "integration-only".into()
                ])
            );
            reader
                .get_mut()
                .write_all(b"+OK\r\n")
                .expect("AUTH response");
            reader.get_mut().flush().expect("AUTH flush");
            while let Some(command) = resp(&mut reader) {
                assert_eq!(command, ["PING"]);
                reader
                    .get_mut()
                    .write_all(b"+PONG\r\n")
                    .expect("PING response");
                reader.get_mut().flush().expect("PING flush");
            }
            false
        });
        let fixture = Self {
            directory,
            tls,
            store,
        };
        let key = &committed()["pinnedKey"];
        let key_document = json!([{
            "key_id": layerx_platform_internal::base64::encode(&unhex(&key["keyId"])),
            "public_key": layerx_platform_internal::base64::encode(&unhex(&key["publicKey"])),
            "first_height":key["firstHeight"], "last_height":key["lastHeight"]
        }]);
        fixture.file("keys.json", key_document.to_string());
        fixture
    }

    fn file(&self, name: &str, bytes: impl AsRef<[u8]>) -> String {
        let path = self.directory.path().join(name);
        fs::write(&path, bytes).expect("fixture file");
        path.to_str().expect("UTF-8 path").to_owned()
    }

    pub fn node(&self, failing: bool, trace: Arc<Mutex<Vec<Value>>>) -> Node {
        let failed = Arc::new(AtomicBool::new(failing));
        let failure = Arc::clone(&failed);
        let peer = Peer::http(Arc::clone(&self.tls), move |request| {
            assert_eq!(request.method, "POST");
            assert_eq!(request.path, "/");
            let call: Value = serde_json::from_slice(&request.body).expect("node JSON");
            trace.lock().expect("node trace").push(call.clone());
            if failure.load(Ordering::Acquire) {
                return http::refusal(502, "bad_gateway", None);
            }
            let result = match call["method"].as_str().expect("RPC method") {
                "eth_chainId" => json!("0x7d"),
                "eth_blockNumber" => json!("0xa"),
                "eth_getTransactionCount" => json!("0x20000000000001"),
                "pax_sequencerReceipt" => {
                    let recorded = committed();
                    assert_eq!(call["params"][0], recorded["receipt"]["txHash"]);
                    recorded["receipt"].clone()
                }
                method => panic!("unrecorded node method: {method}"),
            };
            http::json(
                200,
                &json!({"jsonrpc":"2.0", "id":call["id"], "result":result}),
            )
        });
        Node { peer, failed }
    }

    pub fn consensus(&self) -> Node {
        let failed = Arc::new(AtomicBool::new(false));
        let failure = Arc::clone(&failed);
        let peer = Peer::http(Arc::clone(&self.tls), move |request| {
            assert_eq!(request.method, "POST");
            assert_eq!(request.path, "/");
            let call: Value = serde_json::from_slice(&request.body).expect("consensus JSON");
            if failure.load(Ordering::Acquire) {
                return http::refusal(503, "unavailable", None);
            }
            let recorded = committed();
            let result = match call["method"].as_str().expect("consensus method") {
                "status" => json!({"sync_info":{"catching_up":false}}),
                method @ ("commit" | "validators" | "block") => {
                    assert_eq!(call["params"]["height"], "10");
                    recorded[method].clone()
                }
                method => panic!("unrecorded consensus method: {method}"),
            };
            http::json(
                200,
                &json!({"jsonrpc":"2.0", "id":call["id"], "result":result}),
            )
        });
        Node { peer, failed }
    }

    pub fn gateway(&self, nodes: &[&Node], consensus: Option<&Node>, kernel: bool) -> Gateway {
        let listener = TcpListener::bind("127.0.0.1:0").expect("gateway port");
        let port = listener.local_addr().expect("gateway address").port();
        drop(listener);
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-gateway"));
        command
            .env_clear()
            .stdout(Stdio::null())
            .stderr(Stdio::inherit())
            .env("LAYERX_GATEWAY_LISTEN", format!("127.0.0.1:{port}"))
            .env("LAYERX_GATEWAY_LISTENER", "plain")
            .env(
                "LAYERX_GATEWAY_OUTBOUND_CA_DER",
                self.directory.path().join("ca.der"),
            )
            .env("LAYERX_GATEWAY_NETWORK_ID", "repair-22-33-1335")
            .env(
                "LAYERX_GATEWAY_LXP_WIRE_VERSION",
                layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string(),
            )
            .env("LAYERX_GATEWAY_PROTOCOL_NETWORK_ID", "7")
            .env(
                "LAYERX_GATEWAY_REDIS_URL",
                format!("rediss://localhost:{}", self.store.port),
            )
            .env(
                "LAYERX_GATEWAY_REDIS_USERNAME_FILE",
                self.file("username", "gateway"),
            )
            .env(
                "LAYERX_GATEWAY_REDIS_PASSWORD_FILE",
                self.file("password", "integration-only"),
            );
        if !nodes.is_empty() {
            command.env(
                "LAYERX_GATEWAY_PAXEER_RPC_URLS",
                json!(nodes.iter().map(|node| node.peer.url()).collect::<Vec<_>>()).to_string(),
            );
        }
        if let Some(node) = consensus {
            command.env("LAYERX_GATEWAY_CONSENSUS_RPC_URL", node.peer.url());
        }
        command.env(
            "LAYERX_GATEWAY_RECEIPT_KEYS_FILE",
            self.directory.path().join("keys.json"),
        );
        if kernel {
            let retired = self.store.url();
            for name in ["COMPONENT", "AUTHORITY", "IDENTITY", "PROGRAM_REGISTRY"] {
                command
                    .env(format!("LAYERX_GATEWAY_{name}_URL"), &retired)
                    .env(
                        format!("LAYERX_GATEWAY_{name}_TOKEN_FILE"),
                        self.file(&format!("{name}-token"), "integration-only"),
                    );
            }
            command.env("LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12", self.file("identity.p12", include_bytes!(concat!(env!("CARGO_MANIFEST_DIR"), "/../../../../tools/bringup/tests/fixtures/repair_m792/renewed/identity.p12"))))
                .env("LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE", self.file("identity-password", include_bytes!(concat!(env!("CARGO_MANIFEST_DIR"), "/../../../../tools/bringup/tests/fixtures/repair_m792/renewed/password"))))
                .env("LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE", self.file("provisioning-key", "5a".repeat(32)))
                .env("LAYERX_GATEWAY_MODULE_REGISTRY_FILE", self.file("modules.json", json!({
                    "schema_version":2, "assets":[{"asset":"02".repeat(32),"currency":"USD","decimals":6,"symbol":"$"}],
                    "modules":[{"module":9,"ordinals":[1,2,7]}]
                }).to_string()));
        }
        let child = command.spawn().expect("Cargo-built production gateway");
        let mut gateway = Gateway { child, port };
        for _ in 0..200 {
            assert!(
                gateway.child.try_wait().expect("gateway status").is_none(),
                "gateway refused startup"
            );
            if gateway
                .exchange("GET", "/livez", b"")
                .is_ok_and(|(status, _)| status == 200)
            {
                return gateway;
            }
            thread::sleep(Duration::from_millis(10));
        }
        panic!("gateway startup deadline");
    }
}

pub struct Node {
    peer: Peer,
    pub failed: Arc<AtomicBool>,
}

pub struct Gateway {
    child: Child,
    port: u16,
}

impl Gateway {
    pub fn consumer(&self) -> SequencerEndpoint {
        SequencerEndpoint::new(&format!("http://127.0.0.1:{}/", self.port), &[])
            .expect("real Rust consumer")
    }

    pub fn exchange(
        &self,
        method: &str,
        path: &str,
        body: &[u8],
    ) -> std::io::Result<(u16, Vec<u8>)> {
        let mut stream = TcpStream::connect(("127.0.0.1", self.port))?;
        stream.set_read_timeout(Some(Duration::from_secs(10)))?;
        write!(stream, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len())?;
        stream.write_all(body)?;
        let mut answer = Vec::new();
        stream.read_to_end(&mut answer)?;
        let end = answer
            .windows(4)
            .position(|part| part == b"\r\n\r\n")
            .expect("HTTP headers");
        let status = std::str::from_utf8(&answer[..end])
            .expect("HTTP header text")
            .split_whitespace()
            .nth(1)
            .expect("HTTP status")
            .parse()
            .expect("status number");
        Ok((status, answer[end + 4..].to_vec()))
    }

    pub fn rpc(&self, method: &str, params: Value) -> Value {
        let call = json!({"jsonrpc":"2.0", "id":9_007_199_254_740_993_u64,"method":method,"params":params});
        let (status, body) = self
            .exchange("POST", "/", call.to_string().as_bytes())
            .expect("root RPC");
        assert_eq!(status, 200);
        let answer: Value = serde_json::from_slice(&body).expect("RPC JSON");
        assert_eq!(answer["id"], call["id"]);
        answer
    }

    pub fn retired(&self) {
        let call = br#"{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}"#;
        for path in ["/rpc", "/rpc/", "/rpc?query=1", "/rpc/schema", "/rpc/ws"] {
            for method in ["POST", "GET"] {
                let (status, body) = self
                    .exchange(method, path, call)
                    .expect("retired route response");
                assert_eq!(status, 404, "{method} {path}");
                let value: Value = serde_json::from_slice(&body).expect("refusal JSON");
                assert!(value.get("result").is_none());
            }
        }
    }

    pub fn readiness(&self, path: &str) -> Value {
        let (status, body) = self.exchange("GET", path, b"").expect("readiness HTTP");
        let mut value: Value = serde_json::from_slice(&body).expect("readiness document");
        assert_eq!(status, 200, "{value}");
        assert_eq!(value["status"], "ready");
        assert_eq!(
            value["capabilities"],
            json!({
                "chain_serving":{"state":"ready","reason":"ready","chain_id":125},
                "layerx_activity":{"state":"not_configured","reason":"not_configured"}
            })
        );
        assert_eq!(value["components"], json!({"durable_store":"ready"}));
        assert_eq!(
            value["backends"],
            json!({
                "durable_store":{"state":"ready","reason":"ready"},
                "paxeer_chain":{"state":"ready","reason":"ready"},
                "consensus":{"state":"ready","reason":"ready"}
            })
        );
        let observed = value["observed_at_ms"].as_u64().expect("observation");
        assert_eq!(
            value["valid_until_ms"].as_u64().expect("expiry") - observed,
            30_000
        );
        for key in ["observed_at_ms", "valid_until_ms"] {
            value.as_object_mut().expect("document").remove(key);
        }
        value
    }
}

impl Drop for Gateway {
    fn drop(&mut self) {
        self.child.kill().expect("stop gateway");
        self.child.wait().expect("reap gateway");
    }
}

pub fn proof_success_and_recovery() {
    use layerx_client::rpc::commit::{acquire_committed_header, decode_validators};
    use layerx_client::rpc::receipt_commit::{
        verify_receipt_coverage, ReceiptCommitEvidence, SequenceBatch,
    };
    use layerx_programs::chain_proof::{merkle::SimpleProof, ChainProofVerifier};
    use layerx_proof::inclusion::SequencerAuthorization;
    use layerx_proof::receipt::{verify_sequencer_signature, ReceiptKeys};

    let fixture = Fixture::start();
    let trace = Arc::new(Mutex::new(Vec::new()));
    let first = fixture.node(false, Arc::clone(&trace));
    let second = fixture.node(false, Arc::clone(&trace));
    let consensus = fixture.consensus();
    let recorded = committed();
    let hash: [u8; 32] = unhex(&recorded["receipt"]["txHash"])
        .try_into()
        .expect("transaction hash");
    let mut previous = None;
    for _ in 0..2 {
        let gateway = fixture.gateway(&[&first, &second], Some(&consensus), false);
        gateway.retired();
        let consumer = gateway.consumer();
        let keys = consumer
            .receipt_keys()
            .expect("producer keys through root endpoint");
        let receipt = consumer
            .sequencer_receipt(&hash)
            .expect("real consumer receipt")
            .expect("recorded transaction");
        let SequencerReceipt::Receipted(execution) = receipt else {
            panic!("signed execution receipt");
        };
        let pins: Vec<_> = keys
            .iter()
            .map(|key| {
                SequencerAuthorization::new(
                    key.key_id,
                    key.public_key,
                    key.first_height,
                    key.last_height,
                )
            })
            .collect();
        let verified =
            verify_sequencer_signature(&execution.receipt, ReceiptKeys::HeightRanges(&pins))
                .expect("genuine node signature");
        assert_eq!(execution.receipt, unhex(&recorded["receipt"]["receipt"]));
        let logs = execution
            .logs
            .iter()
            .map(|log| {
                layerx_proof::sequencer_logs::SequencerLog::new(
                    log.address,
                    log.topics.clone(),
                    log.data.clone(),
                    log.log_index,
                )
            })
            .collect();
        let checked = layerx_proof::sequencer_logs::verify_logs(&verified, logs)
            .expect("logs bound to signed receipt effects");
        assert_eq!(checked.summary().log_count() as usize, execution.logs.len());
        let validators =
            decode_validators(&consumer.validator_set(10).expect("relayed validators"))
                .expect("real validator consumer");
        let verifier = ChainProofVerifier {
            chain_id: "committed-fixture".into(),
            validators,
        };
        let head = acquire_committed_header(&consumer, 10, &verifier)
            .expect("authenticate real recorded Comet commit");
        let mut changed = consumer
            .commit_material(10)
            .expect("producer commit material")["signedHeader"]
            .clone();
        changed["commit"]["signatures"][0]["signature"] =
            json!(layerx_platform_internal::base64::encode(&[0; 64]));
        assert!(layerx_client::rpc::commit::verify_commit_material(&changed, &verifier).is_err());
        let batch_answer = gateway.rpc("px_getSequenceBatch", json!(["10"]));
        let batch = &batch_answer["result"];
        let tx = unhex(&batch["tx"]);
        let proof = SimpleProof {
            total: batch["proof"]["total"].as_u64().expect("total") as i64,
            index: batch["proof"]["index"].as_u64().expect("index") as i64,
            leaf_hash: unhex(&batch["proof"]["leafHash"])
                .try_into()
                .expect("leaf hash"),
            aunts: batch["proof"]["aunts"]
                .as_array()
                .expect("aunts")
                .iter()
                .map(|value| unhex(value).try_into().expect("aunt hash"))
                .collect(),
        };
        let sequence = SequenceBatch::decode(&tx).expect("real sequence batch codec");
        assert_eq!(sequence.height, 10);
        verify_receipt_coverage(
            verified.protocol().expect("protocol receipt"),
            10,
            &head,
            &ReceiptCommitEvidence {
                verifier: &verifier,
                covering_height: 10,
                batch_transaction: &tx,
                transaction_proof: &proof,
                activity_ids: &[hash],
            },
            &keys,
        )
        .expect("receipt covered by authenticated committed batch");
        let snapshot = (execution.receipt, head.header_hash(), batch.clone());
        if let Some(previous) = &previous {
            assert_eq!(
                &snapshot, previous,
                "reopened gateway must recover the same evidence"
            );
        }
        previous = Some(snapshot);
        let mut corrupted = unhex(&recorded["receipt"]["receipt"]);
        *corrupted.last_mut().expect("signature tail") ^= 1;
        assert!(verify_sequencer_signature(&corrupted, ReceiptKeys::HeightRanges(&pins)).is_err());
        for (method, params) in [
            ("px_getCommit", json!(["0"])),
            ("px_getValidators", json!([10])),
            ("px_getSequenceBatch", json!(["0xa"])),
            ("pax_sequencerReceipt", json!(["0x01"])),
        ] {
            assert_eq!(gateway.rpc(method, params)["error"]["code"], -32602);
        }
        let funds = gateway.rpc("px_requestFunds", json!([]));
        assert_eq!(funds["error"]["data"]["code"], "kernel_unavailable");
        consensus.failed.store(true, Ordering::Release);
        assert!(
            matches!(consumer.commit_material(10), Err(SequencerError::Rpc(error)) if error.code == -32003)
        );
        consensus.failed.store(false, Ordering::Release);
        drop(gateway);
    }
}
