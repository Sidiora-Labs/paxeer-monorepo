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

/// # Panics
/// Panics when a required operation fails.
#[must_use]
pub fn committed() -> Value {
    serde_json::from_str(include_str!("../fixtures/repair_22_33_1335_committed.json")).unwrap_or_else(|error| panic!("{}: {error:?}", "recorded node evidence"))
}

/// # Panics
/// Panics when a required operation fails.
#[must_use]
pub fn unhex(value: &Value) -> Vec<u8> {
    let text = value.as_str().unwrap_or_else(|| panic!("{}", "hex string"));
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| u8::from_str_radix(std::str::from_utf8(pair).unwrap_or_else(|error| panic!("{}: {error:?}", "ASCII")), 16).unwrap_or_else(|error| panic!("{}: {error:?}", "hex")))
        .collect()
}

struct Peer {
    port: u16,
    stop: Arc<AtomicBool>,
    join: Option<JoinHandle<()>>,
}

impl Peer {
    fn start(tls: Arc<ServerConfig>, handler: impl Fn(&mut TlsStream) -> bool + Send + Sync + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{}: {error:?}", "loopback peer"));
        let port = listener.local_addr().unwrap_or_else(|error| panic!("{}: {error:?}", "peer address")).port();
        listener.set_nonblocking(true).unwrap_or_else(|error| panic!("{}: {error:?}", "nonblocking peer"));
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
                            tcp.set_read_timeout(Some(Duration::from_secs(3))).unwrap_or_else(|error| panic!("{}: {error:?}", "read bound"));
                            tcp.set_write_timeout(Some(Duration::from_secs(3))).unwrap_or_else(|error| panic!("{}: {error:?}", "write bound"));
                            let connection = ServerConnection::new(tls).unwrap_or_else(|error| panic!("{}: {error:?}", "TLS connection"));
                            let mut stream = StreamOwned::new(connection, tcp);
                            if handler(&mut stream) {
                                stream.conn.send_close_notify();
                                stream.flush().unwrap_or_else(|error| panic!("{}: {error:?}", "TLS close"));
                            }
                        }));
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("peer accept: {error}"),
                }
            }
            for connection in connections {
                connection.join().unwrap_or_else(|error| panic!("{}: {error:?}", "peer assertions"));
            }
        });
        Self { port, stop, join: Some(join) }
    }

    fn http(tls: Arc<ServerConfig>, handler: impl Fn(&IncomingRequest) -> http::Response + Send + Sync + 'static) -> Self {
        Self::start(tls, move |stream| {
            let request = read_request(stream, http::MAX_REQUEST_BYTES).unwrap_or_else(|error| panic!("{}: {error:?}", "real HTTP request codec"));
            http::write_response(stream, &handler(&request)).unwrap_or_else(|error| panic!("{}: {error:?}", "peer HTTP response"));
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
            join.join().unwrap_or_else(|error| panic!("{}: {error:?}", "peer completed"));
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
    let count: usize = line.strip_prefix('*').unwrap_or_else(|| panic!("{}", "RESP array marker")).trim().parse().unwrap_or_else(|error| panic!("{}: {error:?}", "array length"));
    let mut parts = Vec::new();
    for _ in 0..count {
        line.clear();
        reader.read_line(&mut line).unwrap_or_else(|error| panic!("{}: {error:?}", "bulk length"));
        let length: usize = line.strip_prefix('$').unwrap_or_else(|| panic!("{}", "bulk marker")).trim().parse().unwrap_or_else(|error| panic!("{}: {error:?}", "bulk length"));
        let mut bytes = vec![0; length + 2];
        reader.read_exact(&mut bytes).unwrap_or_else(|error| panic!("{}: {error:?}", "bulk data"));
        assert_eq!(&bytes[length..], b"\r\n");
        parts.push(String::from_utf8(bytes[..length].to_vec()).unwrap_or_else(|error| panic!("{}: {error:?}", "UTF-8 command")));
    }
    Some(parts)
}

pub struct Fixture {
    directory: tempfile::TempDir,
    tls: Arc<ServerConfig>,
    store: Peer,
}

impl Fixture {
    /// # Panics
    /// Panics when a required operation fails.
    #[must_use]
    pub fn start() -> Self {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let directory = tempfile::tempdir().unwrap_or_else(|error| panic!("{}: {error:?}", "fixture directory"));
        let certificate = rcgen::generate_simple_self_signed(vec!["localhost".into()]).unwrap_or_else(|error| panic!("{}: {error:?}", "local certificate"));
        fs::write(directory.path().join("ca.der"), certificate.cert.der()).unwrap_or_else(|error| panic!("{}: {error:?}", "CA file"));
        let tls = Arc::new(
            ServerConfig::builder()
                .with_no_client_auth()
                .with_single_cert(vec![certificate.cert.der().clone()], rustls::pki_types::PrivatePkcs8KeyDer::from(certificate.signing_key.serialize_der()).into())
                .unwrap_or_else(|error| panic!("{}: {error:?}", "TLS peer configuration")),
        );
        // This dependency peer exercises the production Redis client's AUTH/PING
        // transport. It has no application storage or receipt production role.
        let store = Peer::start(Arc::clone(&tls), |stream| {
            let mut reader = BufReader::new(stream);
            assert_eq!(resp(&mut reader), Some(vec!["AUTH".into(), "gateway".into(), "integration-only".into()]));
            reader.get_mut().write_all(b"+OK\r\n").unwrap_or_else(|error| panic!("{}: {error:?}", "AUTH response"));
            reader.get_mut().flush().unwrap_or_else(|error| panic!("{}: {error:?}", "AUTH flush"));
            while let Some(command) = resp(&mut reader) {
                assert_eq!(command, ["PING"]);
                reader.get_mut().write_all(b"+PONG\r\n").unwrap_or_else(|error| panic!("{}: {error:?}", "PING response"));
                reader.get_mut().flush().unwrap_or_else(|error| panic!("{}: {error:?}", "PING flush"));
            }
            false
        });
        let fixture = Self { directory, tls, store };
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
        fs::write(&path, bytes).unwrap_or_else(|error| panic!("{}: {error:?}", "fixture file"));
        path.to_str().unwrap_or_else(|| panic!("{}", "UTF-8 path")).to_owned()
    }

    /// # Panics
    /// Panics when a required operation fails.
    pub fn node(&self, failing: bool, trace: Arc<Mutex<Vec<Value>>>) -> Node {
        let failed = Arc::new(AtomicBool::new(failing));
        let failure = Arc::clone(&failed);
        let peer = Peer::http(Arc::clone(&self.tls), move |request| {
            assert_eq!(request.method, "POST");
            assert_eq!(request.path, "/");
            let call: Value = serde_json::from_slice(&request.body).unwrap_or_else(|error| panic!("{}: {error:?}", "node JSON"));
            trace.lock().unwrap_or_else(|error| panic!("{}: {error:?}", "node trace")).push(call.clone());
            if failure.load(Ordering::Acquire) {
                return http::refusal(502, "bad_gateway", None);
            }
            let result = match call["method"].as_str().unwrap_or_else(|| panic!("{}", "RPC method")) {
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
            http::json(200, &json!({"jsonrpc":"2.0", "id":call["id"], "result":result}))
        });
        Node { peer, failed }
    }

    /// # Panics
    /// Panics when a required operation fails.
    #[must_use]
    pub fn consensus(&self) -> Node {
        let failed = Arc::new(AtomicBool::new(false));
        let failure = Arc::clone(&failed);
        let peer = Peer::http(Arc::clone(&self.tls), move |request| {
            assert_eq!(request.method, "POST");
            assert_eq!(request.path, "/");
            let call: Value = serde_json::from_slice(&request.body).unwrap_or_else(|error| panic!("{}: {error:?}", "consensus JSON"));
            if failure.load(Ordering::Acquire) {
                return http::refusal(503, "unavailable", None);
            }
            let recorded = committed();
            let result = match call["method"].as_str().unwrap_or_else(|| panic!("{}", "consensus method")) {
                "status" => json!({"sync_info":{"catching_up":false}}),
                method @ ("commit" | "validators" | "block") => {
                    assert_eq!(call["params"]["height"], "10");
                    recorded[method].clone()
                }
                method => panic!("unrecorded consensus method: {method}"),
            };
            http::json(200, &json!({"jsonrpc":"2.0", "id":call["id"], "result":result}))
        });
        Node { peer, failed }
    }

    /// # Panics
    /// Panics when a required operation fails.
    #[must_use]
    pub fn gateway(&self, nodes: &[&Node], consensus: Option<&Node>, kernel: bool) -> Gateway {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{}: {error:?}", "gateway port"));
        let port = listener.local_addr().unwrap_or_else(|error| panic!("{}: {error:?}", "gateway address")).port();
        drop(listener);
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-gateway"));
        command
            .env_clear()
            .stdout(Stdio::null())
            .stderr(Stdio::inherit())
            .env("LAYERX_GATEWAY_LISTEN", format!("127.0.0.1:{port}"))
            .env("LAYERX_GATEWAY_LISTENER", "plain")
            .env("LAYERX_GATEWAY_OUTBOUND_CA_DER", self.directory.path().join("ca.der"))
            .env("LAYERX_GATEWAY_NETWORK_ID", "repair-22-33-1335")
            .env("LAYERX_GATEWAY_LXP_WIRE_VERSION", layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string())
            .env("LAYERX_GATEWAY_PROTOCOL_NETWORK_ID", "7")
            .env("LAYERX_GATEWAY_REDIS_URL", format!("rediss://localhost:{}", self.store.port))
            .env("LAYERX_GATEWAY_REDIS_USERNAME_FILE", self.file("username", "gateway"))
            .env("LAYERX_GATEWAY_REDIS_PASSWORD_FILE", self.file("password", "integration-only"));
        if !nodes.is_empty() {
            command.env("LAYERX_GATEWAY_PAXEER_RPC_URLS", json!(nodes.iter().map(|node| node.peer.url()).collect::<Vec<_>>()).to_string());
        }
        if let Some(node) = consensus {
            command.env("LAYERX_GATEWAY_CONSENSUS_RPC_URL", node.peer.url());
        }
        command.env("LAYERX_GATEWAY_RECEIPT_KEYS_FILE", self.directory.path().join("keys.json"));
        if kernel {
            let retired = self.store.url();
            for name in ["COMPONENT", "AUTHORITY", "IDENTITY", "PROGRAM_REGISTRY"] {
                command.env(format!("LAYERX_GATEWAY_{name}_URL"), &retired).env(format!("LAYERX_GATEWAY_{name}_TOKEN_FILE"), self.file(&format!("{name}-token"), "integration-only"));
            }
            command
                .env(
                    "LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12",
                    self.file("identity.p12", include_bytes!(concat!(env!("CARGO_MANIFEST_DIR"), "/../../../../tools/bringup/tests/fixtures/repair_m792/renewed/identity.p12"))),
                )
                .env(
                    "LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE",
                    self.file("identity-password", include_bytes!(concat!(env!("CARGO_MANIFEST_DIR"), "/../../../../tools/bringup/tests/fixtures/repair_m792/renewed/password"))),
                )
                .env("LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE", self.file("provisioning-key", "5a".repeat(32)))
                .env(
                    "LAYERX_GATEWAY_MODULE_REGISTRY_FILE",
                    self.file(
                        "modules.json",
                        json!({
                            "schema_version":2, "assets":[{"asset":"02".repeat(32),"currency":"USD","decimals":6,"symbol":"$"}],
                            "modules":[{"module":9,"ordinals":[1,2,7]}]
                        })
                        .to_string(),
                    ),
                );
        }
        let child = command.spawn().unwrap_or_else(|error| panic!("{}: {error:?}", "Cargo-built production gateway"));
        let mut gateway = Gateway { child, port };
        for _ in 0..200 {
            assert!(gateway.child.try_wait().unwrap_or_else(|error| panic!("{}: {error:?}", "gateway status")).is_none(), "gateway refused startup");
            if gateway.exchange("GET", "/livez", b"").is_ok_and(|(status, _)| status == 200) {
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
    /// # Panics
    /// Panics when a required operation fails.
    #[must_use]
    pub fn consumer(&self) -> SequencerEndpoint {
        SequencerEndpoint::new(&format!("http://127.0.0.1:{}/", self.port), &[]).unwrap_or_else(|error| panic!("{}: {error:?}", "real Rust consumer"))
    }

    /// # Panics
    /// Panics when a required operation fails.
    /// # Errors
    /// Returns an error when the gateway connection or response read fails.
    pub fn exchange(&self, method: &str, path: &str, body: &[u8]) -> std::io::Result<(u16, Vec<u8>)> {
        let mut stream = TcpStream::connect(("127.0.0.1", self.port))?;
        stream.set_read_timeout(Some(Duration::from_secs(10)))?;
        write!(stream, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len())?;
        stream.write_all(body)?;
        let mut answer = Vec::new();
        stream.read_to_end(&mut answer)?;
        let end = answer.windows(4).position(|part| part == b"\r\n\r\n").unwrap_or_else(|| panic!("{}", "HTTP headers"));
        let status = std::str::from_utf8(&answer[..end])
            .unwrap_or_else(|error| panic!("{}: {error:?}", "HTTP header text"))
            .split_whitespace()
            .nth(1)
            .unwrap_or_else(|| panic!("{}", "HTTP status"))
            .parse()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "status number"));
        Ok((status, answer[end + 4..].to_vec()))
    }

    /// # Panics
    /// Panics when a required operation fails.
    #[must_use]
    pub fn rpc(&self, method: &str, params: Value) -> Value {
        let params = serde_json::to_value(params).unwrap_or_else(|error| panic!("RPC parameters: {error}"));
        let call = json!({"jsonrpc":"2.0", "id":9_007_199_254_740_993_u64,"method":method,"params":params});
        let (status, body) = self.exchange("POST", "/", call.to_string().as_bytes()).unwrap_or_else(|error| panic!("{}: {error:?}", "root RPC"));
        assert_eq!(status, 200);
        let answer: Value = serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{}: {error:?}", "RPC JSON"));
        assert_eq!(answer["id"], call["id"]);
        answer
    }

    /// # Panics
    /// Panics when a required operation fails.
    pub fn retired(&self) {
        let call = br#"{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}"#;
        for path in ["/rpc", "/rpc/", "/rpc?query=1", "/rpc/schema", "/rpc/ws"] {
            for method in ["POST", "GET"] {
                let (status, body) = self.exchange(method, path, call).unwrap_or_else(|error| panic!("{}: {error:?}", "retired route response"));
                assert_eq!(status, 404, "{method} {path}");
                let value: Value = serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{}: {error:?}", "refusal JSON"));
                assert!(value.get("result").is_none());
            }
        }
    }

    /// # Panics
    /// Panics when a required operation fails.
    #[must_use]
    pub fn readiness(&self, path: &str) -> Value {
        let (status, body) = self.exchange("GET", path, b"").unwrap_or_else(|error| panic!("{}: {error:?}", "readiness HTTP"));
        let mut value: Value = serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{}: {error:?}", "readiness document"));
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
        let observed = value["observed_at_ms"].as_u64().unwrap_or_else(|| panic!("{}", "observation"));
        assert_eq!(value["valid_until_ms"].as_u64().unwrap_or_else(|| panic!("{}", "expiry")) - observed, 30_000);
        for key in ["observed_at_ms", "valid_until_ms"] {
            value.as_object_mut().unwrap_or_else(|| panic!("{}", "document")).remove(key);
        }
        value
    }
}

impl Drop for Gateway {
    fn drop(&mut self) {
        self.child.kill().unwrap_or_else(|error| panic!("{}: {error:?}", "stop gateway"));
        self.child.wait().unwrap_or_else(|error| panic!("{}: {error:?}", "reap gateway"));
    }
}

/// # Panics
/// Panics when a required operation fails.
pub fn proof_success_and_recovery() {
    use layerx_client::rpc::commit::{acquire_committed_header, decode_validators};
    use layerx_client::rpc::receipt_commit::{verify_receipt_coverage, ReceiptCommitEvidence, SequenceBatch};
    use layerx_programs::chain_proof::{merkle::SimpleProof, ChainProofVerifier};
    use layerx_proof::inclusion::SequencerAuthorization;
    use layerx_proof::receipt::{verify_sequencer_signature, ReceiptKeys};

    let fixture = Fixture::start();
    let trace = Arc::new(Mutex::new(Vec::new()));
    let first = fixture.node(false, Arc::clone(&trace));
    let second = fixture.node(false, Arc::clone(&trace));
    let consensus = fixture.consensus();
    let recorded = committed();
    let hash: [u8; 32] = unhex(&recorded["receipt"]["txHash"]).try_into().unwrap_or_else(|error| panic!("{}: {error:?}", "transaction hash"));
    let mut previous = None;
    for _ in 0..2 {
        let gateway = fixture.gateway(&[&first, &second], Some(&consensus), false);
        gateway.retired();
        let consumer = gateway.consumer();
        let keys = consumer.receipt_keys().unwrap_or_else(|error| panic!("{}: {error:?}", "producer keys through root endpoint"));
        let receipt = consumer.sequencer_receipt(&hash).unwrap_or_else(|error| panic!("{}: {error:?}", "real consumer receipt")).unwrap_or_else(|| panic!("{}", "recorded transaction"));
        let SequencerReceipt::Receipted(execution) = receipt else {
            panic!("signed execution receipt");
        };
        let pins: Vec<_> = keys.iter().map(|key| SequencerAuthorization::new(key.key_id, key.public_key, key.first_height, key.last_height)).collect();
        let verified = verify_sequencer_signature(&execution.receipt, ReceiptKeys::HeightRanges(&pins)).unwrap_or_else(|error| panic!("{}: {error:?}", "genuine node signature"));
        assert_eq!(execution.receipt, unhex(&recorded["receipt"]["receipt"]));
        let logs = execution.logs.iter().map(|log| layerx_proof::sequencer_logs::SequencerLog::new(log.address, log.topics.clone(), log.data.clone(), log.log_index)).collect();
        let checked = layerx_proof::sequencer_logs::verify_logs(&verified, logs).unwrap_or_else(|error| panic!("{}: {error:?}", "logs bound to signed receipt effects"));
        assert_eq!(checked.summary().log_count() as usize, execution.logs.len());
        let validators = decode_validators(&consumer.validator_set(10).unwrap_or_else(|error| panic!("{}: {error:?}", "relayed validators"))).unwrap_or_else(|error| panic!("{}: {error:?}", "real validator consumer"));
        let proof_verifier = ChainProofVerifier { chain_id: "committed-fixture".into(), validators };
        let head = acquire_committed_header(&consumer, 10, &proof_verifier).unwrap_or_else(|error| panic!("{}: {error:?}", "authenticate real recorded Comet commit"));
        let mut changed = consumer.commit_material(10).unwrap_or_else(|error| panic!("{}: {error:?}", "producer commit material"))["signedHeader"].clone();
        changed["commit"]["signatures"][0]["signature"] = json!(layerx_platform_internal::base64::encode(&[0; 64]));
        assert!(layerx_client::rpc::commit::verify_commit_material(&changed, &proof_verifier).is_err());
        let batch_answer = gateway.rpc("px_getSequenceBatch", json!(["10"]));
        let batch = &batch_answer["result"];
        let tx = unhex(&batch["tx"]);
        let proof = SimpleProof {
            total: batch["proof"]["total"].as_u64().unwrap_or_else(|| panic!("{}", "total")).try_into().unwrap_or_else(|_| panic!("proof index range")),
            index: batch["proof"]["index"].as_u64().unwrap_or_else(|| panic!("{}", "index")).try_into().unwrap_or_else(|_| panic!("proof index range")),
            leaf_hash: unhex(&batch["proof"]["leafHash"]).try_into().unwrap_or_else(|error| panic!("{}: {error:?}", "leaf hash")),
            aunts: batch["proof"]["aunts"]
                .as_array()
                .unwrap_or_else(|| panic!("{}", "aunts"))
                .iter()
                .map(|value| unhex(value).try_into().unwrap_or_else(|error| panic!("{}: {error:?}", "aunt hash")))
                .collect(),
        };
        let sequence = SequenceBatch::decode(&tx).unwrap_or_else(|error| panic!("{}: {error:?}", "real sequence batch codec"));
        assert_eq!(sequence.height, 10);
        verify_receipt_coverage(
            verified.protocol().unwrap_or_else(|| panic!("{}", "protocol receipt")),
            10,
            &head,
            &ReceiptCommitEvidence {
                verifier: &proof_verifier,
                covering_height: 10,
                batch_transaction: &tx,
                transaction_proof: &proof,
                activity_ids: &[hash],
            },
            &keys,
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "receipt covered by authenticated committed batch"));
        let snapshot = (execution.receipt, head.header_hash(), batch.clone());
        if let Some(previous) = &previous {
            assert_eq!(&snapshot, previous, "reopened gateway must recover the same evidence");
        }
        previous = Some(snapshot);
        let mut corrupted = unhex(&recorded["receipt"]["receipt"]);
        *corrupted.last_mut().unwrap_or_else(|| panic!("{}", "signature tail")) ^= 1;
        assert!(verify_sequencer_signature(&corrupted, ReceiptKeys::HeightRanges(&pins)).is_err());
        for (method, params) in [("px_getCommit", json!(["0"])), ("px_getValidators", json!([10])), ("px_getSequenceBatch", json!(["0xa"])), ("pax_sequencerReceipt", json!(["0x01"]))] {
            assert_eq!(gateway.rpc(method, params)["error"]["code"], -32602);
        }
        let funds = gateway.rpc("px_requestFunds", json!([]));
        assert_eq!(funds["error"]["data"]["code"], "kernel_unavailable");
        consensus.failed.store(true, Ordering::Release);
        assert!(matches!(consumer.commit_material(10), Err(SequencerError::Rpc(error)) if error.code == -32003));
        consensus.failed.store(false, Ordering::Release);
        drop(gateway);
    }
}
