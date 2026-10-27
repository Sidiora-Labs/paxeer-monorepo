use layerx_crypto::evm_transaction::transaction_hash;
use layerx_platform_core::{hex_decode, hex_encode};
use native_tls::{Certificate, TlsConnector};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::fs;
use std::io::Write;
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::os::unix::net::UnixStream;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

static NEXT: AtomicU64 = AtomicU64::new(0);
const RECIPIENT: [u8; 20] = [0x39; 20];
const PUBLIC_KEY: &str = "abababababababababababababababababababababababababababababababab";
const TOKEN: &str = "chain-funding-integration-test-admin-token";

#[derive(Default)]
struct ChainState {
    requests: Vec<Value>,
    submissions: Vec<Vec<u8>>,
    mined: bool,
    unbound: bool,
    lose_response: bool,
    journal: PathBuf,
}

struct Chain {
    address: std::net::SocketAddr,
    state: Arc<Mutex<ChainState>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl Chain {
    fn start(journal: PathBuf) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        listener.set_nonblocking(true).unwrap();
        let state = Arc::new(Mutex::new(ChainState {
            journal,
            ..ChainState::default()
        }));
        let stop = Arc::new(AtomicBool::new(false));
        let shared = Arc::clone(&state);
        let stopping = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            while !stopping.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(5));
                        continue;
                    }
                    Err(error) => panic!("chain listener: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                let request =
                    layerx_platform_internal::http::parse_client_request(&mut stream).unwrap();
                assert_eq!(request.method, "POST");
                let request: Value = serde_json::from_slice(&request.body).unwrap();
                let mut state = shared.lock().unwrap();
                state.requests.push(request.clone());
                let result = match request["method"].as_str().unwrap() {
                    "eth_chainId" => json!("0x7d"),
                    "eth_call" => {
                        assert_eq!(
                            request["params"][0]["to"],
                            "0x0000000000000000000000000000000000001004"
                        );
                        assert_eq!(
                            request["params"][0]["data"],
                            format!("0xc413bad3{PUBLIC_KEY}")
                        );
                        assert_eq!(request["params"][1], "latest");
                        let recipient = if state.unbound { [0; 20] } else { RECIPIENT };
                        json!(format!("0x{}{}", "00".repeat(12), hex_encode(&recipient)))
                    }
                    "eth_getTransactionCount" => {
                        assert_eq!(
                            request["params"][0],
                            "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf"
                        );
                        assert_eq!(request["params"][1], "pending");
                        let count = state
                            .requests
                            .iter()
                            .filter(|request| request["method"] == "eth_getTransactionCount")
                            .count();
                        json!(format!("0x{:x}", 6 + count))
                    }
                    "eth_gasPrice" => json!("0xa"),
                    "eth_maxPriorityFeePerGas" => json!("0x2"),
                    "eth_estimateGas" => {
                        assert_eq!(
                            request["params"][0]["to"],
                            format!("0x{}", hex_encode(&RECIPIENT))
                        );
                        assert_eq!(request["params"][0]["value"], "0x1e240");
                        assert_eq!(request["params"][0]["data"], "0x");
                        json!("0x5208")
                    }
                    "eth_sendRawTransaction" => {
                        let raw = hex_decode(
                            request["params"][0]
                                .as_str()
                                .unwrap()
                                .strip_prefix("0x")
                                .unwrap(),
                        )
                        .unwrap();
                        let records = fs::read_to_string(&state.journal)
                            .expect("signed transaction must be journaled before broadcast");
                        let record: Value =
                            serde_json::from_str(records.lines().last().unwrap()).unwrap();
                        let signed: Value =
                            serde_json::from_str(record["body"].as_str().unwrap()).unwrap();
                        assert_eq!(signed["raw"], json!(raw));
                        assert_eq!(
                            signed["transaction_hash"],
                            format!("0x{}", hex_encode(&transaction_hash(&raw)))
                        );
                        state.submissions.push(raw.clone());
                        if state.lose_response {
                            state.lose_response = false;
                            continue;
                        }
                        json!(format!("0x{}", hex_encode(&transaction_hash(&raw))))
                    }
                    "eth_getTransactionReceipt" => {
                        let raw = state
                            .submissions
                            .last()
                            .expect("receipt queried after submission");
                        let hash = format!("0x{}", hex_encode(&transaction_hash(raw)));
                        assert_eq!(request["params"][0], hash);
                        if state.mined {
                            json!({"transactionHash": hash, "status": "0x1"})
                        } else {
                            Value::Null
                        }
                    }
                    method => panic!("unexpected RPC {method}"),
                };
                let body =
                    json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).unwrap();
            }
        });
        Self {
            address,
            state,
            stop,
            worker: Some(worker),
        }
    }
}

impl Drop for Chain {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let result = worker.join();
            if !thread::panicking() {
                result.unwrap();
            }
        }
    }
}

struct Harness {
    root: PathBuf,
    signer: Child,
    core: Option<Child>,
    admin: u16,
    listen: u16,
    chain: Option<Chain>,
    chain_address: std::net::SocketAddr,
}

fn private_file(path: &Path, bytes: &[u8]) {
    fs::write(path, bytes).unwrap();
    fs::set_permissions(path, fs::Permissions::from_mode(0o600)).unwrap();
}

fn openssl(root: &Path, arguments: &[&str]) {
    let output = Command::new("openssl")
        .current_dir(root)
        .args(arguments)
        .output()
        .unwrap();
    assert!(
        output.status.success(),
        "openssl: {}",
        String::from_utf8_lossy(&output.stderr)
    );
}

fn free_port() -> u16 {
    TcpListener::bind("127.0.0.1:0")
        .unwrap()
        .local_addr()
        .unwrap()
        .port()
}

impl Harness {
    fn start(key: &str, lose_response: bool) -> Self {
        Self::with_chain(key, lose_response, None)
    }

    fn with_chain(
        key: &str,
        lose_response: bool,
        chain_address: Option<std::net::SocketAddr>,
    ) -> Self {
        let root = std::env::temp_dir().join(format!(
            "chain-fund-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&root).unwrap();
        fs::set_permissions(&root, fs::Permissions::from_mode(0o700)).unwrap();
        fs::create_dir(root.join("state")).unwrap();
        fs::set_permissions(root.join("state"), fs::Permissions::from_mode(0o700)).unwrap();
        private_file(&root.join("treasury.key"), &[9; 32]);
        private_file(&root.join("sender.key"), format!("{:064x}", 1).as_bytes());
        private_file(&root.join("token"), TOKEN.as_bytes());
        openssl(
            &root,
            &[
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-keyout",
                "tls.key",
                "-out",
                "tls.pem",
                "-days",
                "1",
                "-subj",
                "/CN=localhost",
                "-addext",
                "subjectAltName=DNS:localhost",
            ],
        );
        openssl(
            &root,
            &[
                "x509", "-in", "tls.pem", "-outform", "DER", "-out", "tls.der",
            ],
        );
        openssl(
            &root,
            &[
                "pkcs8",
                "-topk8",
                "-nocrypt",
                "-in",
                "tls.key",
                "-outform",
                "DER",
                "-out",
                "tls-key.der",
            ],
        );
        let repository = Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .nth(4)
            .unwrap();
        let uid = fs::read_to_string("/proc/self/status")
            .unwrap()
            .lines()
            .find_map(|line| {
                line.strip_prefix("Uid:")
                    .map(|line| line.split_whitespace().nth(1).unwrap().to_owned())
            })
            .unwrap();
        let signer = Command::new("python3")
            .arg(repository.join("services/platform/hosted/node/signer/signer.py"))
            .arg("--socket")
            .arg(root.join("signer.sock"))
            .arg("--allowed-uid")
            .arg(uid)
            .arg("--provider")
            .arg("file")
            .arg("--key-file")
            .arg(root.join("treasury.key"))
            .stdout(Stdio::null())
            .stderr(fs::File::create(root.join("signer.log")).unwrap())
            .spawn()
            .unwrap();
        let deadline = Instant::now() + Duration::from_secs(10);
        while UnixStream::connect(root.join("signer.sock")).is_err() {
            assert!(
                Instant::now() < deadline,
                "signer startup: {}",
                fs::read_to_string(root.join("signer.log")).unwrap()
            );
            thread::sleep(Duration::from_millis(20));
        }
        let mut digest = Sha256::new();
        digest.update(b"fund-canonical\0");
        digest.update(key.as_bytes());
        let chain = if chain_address.is_none() {
            let chain = Chain::start(
                root.join("state/journal")
                    .join(format!("{}.json", hex_encode(&digest.finalize()))),
            );
            chain.state.lock().unwrap().lose_response = lose_response;
            Some(chain)
        } else {
            None
        };
        let chain_address = chain_address.unwrap_or_else(|| chain.as_ref().unwrap().address);
        let mut harness = Self {
            root,
            signer,
            core: None,
            admin: free_port(),
            listen: free_port(),
            chain,
            chain_address,
        };
        harness.restart(true, true);
        harness
    }

    fn chain(&self) -> &Chain {
        self.chain.as_ref().unwrap()
    }

    fn restart(&mut self, endpoint: bool, sender: bool) {
        if let Some(mut core) = self.core.take() {
            core.kill().unwrap();
            core.wait().unwrap();
        }
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-core-boundary"));
        command
            .env_remove("LAYERX_CORE_CLIENT_CA_DER")
            .env_remove("LAYERX_CORE_RECEIPT_EVENTS_TOKEN_FILE")
            .env_remove("LAYERX_CORE_WALLET_CAPS_TOKEN_FILE")
            .env_remove("LAYERX_CORE_FUNDING_SENDER_KMS_KEY_ID")
            .env_remove("LAYERX_CORE_CHAIN_RPC_URL")
            .env_remove("LAYERX_CORE_FUNDING_SENDER_KEY_FILE")
            .env("LAYERX_CORE_NETWORK_ID", "125")
            .env("LAYERX_CORE_LISTEN", format!("127.0.0.1:{}", self.listen))
            .env(
                "LAYERX_CORE_ADMIN_LISTEN",
                format!("127.0.0.1:{}", self.admin),
            )
            .env(
                "LAYERX_CORE_TREASURY_SIGNER_SOCKET",
                self.root.join("signer.sock"),
            )
            .env("LAYERX_CORE_TREASURY_ASSET", "02".repeat(32))
            .env("LAYERX_CORE_SEQUENCER_ID", "03".repeat(32))
            .env("LAYERX_CORE_STATE_DIR", self.root.join("state"))
            .env("LAYERX_CORE_NODE_URL", "http://127.0.0.1:1")
            .env("LAYERX_CORE_REPLICA_URL", "http://127.0.0.1:1")
            .env("LAYERX_CORE_LNI_SOCKET", self.root.join("absent-lni.sock"))
            .env(
                "LAYERX_CORE_SUPERVISOR_SOCKET",
                self.root.join("absent-supervisor.sock"),
            )
            .env(
                "LAYERX_CORE_NODE_BEARER_TOKEN_FILE",
                self.root.join("token"),
            )
            .env(
                "LAYERX_CORE_REPLICA_BEARER_TOKEN_FILE",
                self.root.join("token"),
            )
            .env("LAYERX_CORE_ADMIN_TOKEN_FILE", self.root.join("token"))
            .env("LAYERX_CORE_TLS_CERT_DER", self.root.join("tls.der"))
            .env("LAYERX_CORE_TLS_KEY_DER", self.root.join("tls-key.der"))
            .env("LAYERX_CORE_ADMIN_TLS_CERT_DER", self.root.join("tls.der"))
            .env(
                "LAYERX_CORE_ADMIN_TLS_KEY_DER",
                self.root.join("tls-key.der"),
            );
        if endpoint {
            command.env(
                "LAYERX_CORE_CHAIN_RPC_URL",
                format!("http://{}", self.chain_address),
            );
        }
        if sender {
            command.env(
                "LAYERX_CORE_FUNDING_SENDER_KEY_FILE",
                self.root.join("sender.key"),
            );
        }
        self.core = Some(
            command
                .stdout(Stdio::null())
                .stderr(fs::File::create(self.root.join("core.log")).unwrap())
                .spawn()
                .unwrap(),
        );
        let deadline = Instant::now() + Duration::from_secs(10);
        while TcpStream::connect(("127.0.0.1", self.admin)).is_err() {
            assert!(
                self.core.as_mut().unwrap().try_wait().unwrap().is_none(),
                "core exited: {}",
                fs::read_to_string(self.root.join("core.log")).unwrap()
            );
            assert!(
                Instant::now() < deadline,
                "core startup: {}",
                fs::read_to_string(self.root.join("core.log")).unwrap()
            );
            thread::sleep(Duration::from_millis(20));
        }
    }

    fn fund(&self, key: &str, amount: u64) -> (u16, Value) {
        let body = json!({"funding_id": "funding-8109", "did": format!("did:layerx:{PUBLIC_KEY}"), "public_key": PUBLIC_KEY, "amount": amount}).to_string();
        self.request(&format!("POST /admin/v1/testnet/fund HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {TOKEN}\r\nContent-Type: application/json\r\nIdempotency-Key: {key}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()))
    }

    fn readiness(&self) -> (u16, Value) {
        self.request_on(
            self.listen,
            "GET /readyz HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n",
        )
    }

    fn request(&self, request: &str) -> (u16, Value) {
        self.request_on(self.admin, request)
    }

    fn request_on(&self, port: u16, request: &str) -> (u16, Value) {
        let connector = TlsConnector::builder()
            .add_root_certificate(
                Certificate::from_der(&fs::read(self.root.join("tls.der")).unwrap()).unwrap(),
            )
            .build()
            .unwrap();
        let tcp = TcpStream::connect(("127.0.0.1", port)).unwrap();
        tcp.set_read_timeout(Some(Duration::from_secs(10))).unwrap();
        let mut stream = connector.connect("localhost", tcp).unwrap();
        stream.write_all(request.as_bytes()).unwrap();
        let response =
            layerx_platform_internal::http::read_http_message(&mut stream, 64 * 1024).unwrap();
        let status = response.headers[""]
            .split_whitespace()
            .nth(1)
            .unwrap()
            .parse()
            .unwrap();
        (status, serde_json::from_slice(&response.body).unwrap())
    }
}

impl Drop for Harness {
    fn drop(&mut self) {
        if let Some(mut core) = self.core.take() {
            let _ = core.kill();
            let _ = core.wait();
        }
        let _ = self.signer.kill();
        let _ = self.signer.wait();
        let _ = fs::remove_dir_all(&self.root);
    }
}

fn take_rlp<'a>(input: &mut &'a [u8]) -> (&'a [u8], &'a [u8]) {
    let original = *input;
    let prefix = original[0];
    let (offset, size) = match prefix {
        0..=0x7f => (0, 1),
        0x80..=0xb7 => (1, usize::from(prefix - 0x80)),
        0xc0..=0xf7 => (1, usize::from(prefix - 0xc0)),
        _ => {
            let width = usize::from(if prefix < 0xc0 {
                prefix - 0xb7
            } else {
                prefix - 0xf7
            });
            let size = original[1..=width]
                .iter()
                .fold(0_usize, |size, byte| size * 256 + usize::from(*byte));
            (1 + width, size)
        }
    };
    *input = &original[offset + size..];
    (&original[offset..offset + size], &original[..offset + size])
}

fn assert_transfer(raw: &[u8]) {
    assert_eq!(raw[0], 2);
    let mut envelope = &raw[1..];
    let (mut payload, _) = take_rlp(&mut envelope);
    assert!(envelope.is_empty());
    let mut fields = Vec::new();
    let mut encoded = Vec::new();
    while !payload.is_empty() {
        let (field, original) = take_rlp(&mut payload);
        fields.push(field);
        encoded.push(original);
    }
    assert_eq!(fields.len(), 12);
    let number = |bytes: &[u8]| {
        bytes
            .iter()
            .fold(0_u128, |value, byte| value * 256 + u128::from(*byte))
    };
    assert_eq!(number(fields[0]), 125);
    assert_eq!(number(fields[1]), 7);
    assert_eq!(number(fields[2]), 2);
    assert_eq!(number(fields[3]), 22);
    assert_eq!(number(fields[4]), 21_000);
    assert_eq!(fields[5], RECIPIENT);
    assert_eq!(number(fields[6]), 123_456);
    assert!(fields[7].is_empty());
    assert_eq!(encoded[8], &[0xc0]);
    let unsigned = encoded[..9].concat();
    assert!(unsigned.len() < 56);
    let mut preimage = vec![2, 0xc0 + u8::try_from(unsigned.len()).unwrap()];
    preimage.extend(unsigned);
    let mut signature = [0; 64];
    signature[32 - fields[10].len()..32].copy_from_slice(fields[10]);
    signature[64 - fields[11].len()..].copy_from_slice(fields[11]);
    let public =
        hex_decode("0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798").unwrap();
    layerx_crypto::secp256k1::verify_recoverable_digest(
        &public,
        &signature,
        27 + u8::try_from(number(fields[9])).unwrap(),
        &transaction_hash(&preimage),
    )
    .unwrap();
}

#[test]
fn testnet_fund_sends_one_bank_transfer_to_the_bound_address() {
    let mut harness = Harness::start("one-transfer", false);
    let (status, pending) = harness.fund("one-transfer", 123_456);
    assert_eq!(status, 202);
    assert_eq!(pending["funding_id"], "funding-8109");
    assert_eq!(pending["state"], "pending");
    {
        let state = harness.chain().state.lock().unwrap();
        assert_eq!(state.submissions.len(), 1);
        assert_transfer(&state.submissions[0]);
        assert_eq!(
            pending["transaction_id"],
            format!("0x{}", hex_encode(&transaction_hash(&state.submissions[0])))
        );
    }
    assert_eq!(harness.fund("one-transfer", 1).0, 409);
    assert_eq!(harness.fund("invalid-amount", 0).0, 400);
    harness.chain().state.lock().unwrap().mined = true;
    let (status, funded) = harness.fund("one-transfer", 123_456);
    assert_eq!(status, 200);
    assert_eq!(funded["state"], "funded");
    assert_eq!(funded["transaction_id"], pending["transaction_id"]);
    assert_eq!(funded["funding_id"], "funding-8109");
    assert_eq!(harness.fund("one-transfer", 123_456), (200, funded));
    harness.chain().state.lock().unwrap().unbound = true;
    let (status, refusal) = harness.fund("unbound", 123_456);
    assert_eq!(status, 409);
    assert_eq!(refusal["error"]["code"], "identity_binding_unavailable");
    {
        let state = harness.chain().state.lock().unwrap();
        assert_eq!(state.submissions.len(), 2);
        assert_eq!(state.submissions[0], state.submissions[1]);
    }
    harness.restart(false, true);
    let (status, unavailable) = harness.fund("missing-endpoint", 123_456);
    assert_eq!(status, 503);
    assert_eq!(unavailable["error"]["code"], "treasury_unavailable");
    harness.restart(true, false);
    let (status, unavailable) = harness.fund("missing-sender", 123_456);
    assert_eq!(status, 503);
    assert_eq!(unavailable["error"]["code"], "treasury_unavailable");
}

#[test]
fn testnet_fund_replays_the_journaled_transaction() {
    let mut harness = Harness::start("replay-transfer", true);
    let (status, refusal) = harness.fund("replay-transfer", 123_456);
    assert_eq!(status, 503);
    assert_eq!(refusal["error"]["code"], "treasury_unavailable");
    let original = harness.chain().state.lock().unwrap().submissions[0].clone();
    assert_transfer(&original);
    private_file(
        &harness.root.join("sender.key"),
        format!("{:064x}", 2).as_bytes(),
    );
    harness.restart(true, true);
    let (status, pending) = harness.fund("replay-transfer", 123_456);
    assert_eq!(status, 202);
    assert_eq!(pending["state"], "pending");
    assert_eq!(
        pending["transaction_id"],
        format!("0x{}", hex_encode(&transaction_hash(&original)))
    );
    harness.chain().state.lock().unwrap().mined = true;
    let (status, funded) = harness.fund("replay-transfer", 123_456);
    assert_eq!(status, 200);
    assert_eq!(funded["state"], "funded");
    assert_eq!(funded["transaction_id"], pending["transaction_id"]);
    assert_eq!(harness.fund("replay-transfer", 123_457).0, 409);
    let state = harness.chain().state.lock().unwrap();
    assert_eq!(state.submissions.len(), 3);
    for raw in &state.submissions {
        assert_eq!(raw, &original);
    }
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
                .filter(|request| request["method"] == method)
                .count(),
            1,
            "{method} must not repeat preparation"
        );
    }
    assert_eq!(
        state
            .requests
            .iter()
            .filter(|request| request["method"] == "eth_chainId")
            .count(),
        2
    );
}

struct ReadinessChain {
    child: Child,
    address: std::net::SocketAddr,
}

impl ReadinessChain {
    fn start() -> Self {
        let port = free_port();
        let child = Command::new(std::env::var_os("ANVIL_BIN").unwrap_or_else(|| "anvil".into()))
            .args([
                "--host",
                "127.0.0.1",
                "--port",
                &port.to_string(),
                "--chain-id",
                "125",
                "--no-mining",
                "--silent",
            ])
            .stdout(Stdio::null())
            .spawn()
            .expect("Anvil must be installed to run the real funding chain readiness test");
        let mut chain = Self {
            child,
            address: ([127, 0, 0, 1], port).into(),
        };
        let deadline = Instant::now() + Duration::from_secs(10);
        while TcpStream::connect(chain.address).is_err() {
            assert!(
                chain.child.try_wait().unwrap().is_none(),
                "Anvil exited before listening"
            );
            assert!(Instant::now() < deadline, "Anvil did not start");
            thread::sleep(Duration::from_millis(20));
        }
        assert_eq!(chain.rpc("eth_chainId"), "0x7d");
        chain
    }

    fn rpc(&self, method: &str) -> Value {
        let body = json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": []}).to_string();
        let mut stream = TcpStream::connect(self.address).unwrap();
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap();
        write!(stream, "POST / HTTP/1.1\r\nHost: {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", self.address, body.len()).unwrap();
        let response =
            layerx_platform_internal::http::read_http_message(&mut stream, 64 * 1024).unwrap();
        assert_eq!(response.headers[""].split_whitespace().nth(1), Some("200"));
        let response: Value = serde_json::from_slice(&response.body).unwrap();
        assert_eq!(response["jsonrpc"], "2.0");
        assert_eq!(response["id"], 1);
        assert!(response.get("error").is_none(), "{response}");
        response.get("result").unwrap().clone()
    }

    fn stop(&mut self) {
        self.child.kill().unwrap();
        assert!(self.child.wait().unwrap().code().is_none());
    }
}

impl Drop for ReadinessChain {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

#[test]
fn readyz_reports_the_funding_chain_and_sender() {
    let mut chain = ReadinessChain::start();
    let mut harness = Harness::with_chain("readiness", false, Some(chain.address));
    let (status, mut document) = harness.readiness();
    assert_eq!(status, 503);
    assert_eq!(document["error"]["code"], "node_unavailable");
    assert_eq!(
        document["funding"],
        json!({
            "ready": true,
            "chain_id": 125,
            "latest_block": 0,
            "sender": "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf",
        })
    );
    document.as_object_mut().unwrap().remove("funding");
    let kernel = document;

    chain.rpc("evm_mine");
    assert_eq!(chain.rpc("eth_blockNumber"), "0x1");
    let (status, mut document) = harness.readiness();
    assert_eq!(status, 503);
    assert_eq!(document["funding"]["ready"], true);
    assert_eq!(document["funding"]["latest_block"], 1);
    document.as_object_mut().unwrap().remove("funding");
    assert_eq!(document, kernel);

    for valid_key in [false, true] {
        if valid_key {
            private_file(&harness.root.join("sender.key"), b"invalid-key");
        }
        harness.restart(true, valid_key);
        let (status, mut document) = harness.readiness();
        assert_eq!(status, 503);
        assert_eq!(
            document["funding"],
            json!({"ready": false, "reason": "sender_unavailable"})
        );
        document.as_object_mut().unwrap().remove("funding");
        assert_eq!(document, kernel);
    }
    private_file(
        &harness.root.join("sender.key"),
        format!("{:064x}", 1).as_bytes(),
    );
    harness.restart(false, true);
    let (status, mut document) = harness.readiness();
    assert_eq!(status, 503);
    assert_eq!(
        document["funding"],
        json!({"ready": false, "reason": "chain_unavailable"})
    );
    document.as_object_mut().unwrap().remove("funding");
    assert_eq!(document, kernel);

    harness.restart(true, true);
    assert_eq!(harness.readiness().1["funding"]["ready"], true);
    chain.stop();
    let (status, mut document) = harness.readiness();
    assert_eq!(status, 503);
    assert_eq!(
        document["funding"],
        json!({"ready": false, "reason": "chain_unavailable"})
    );
    document.as_object_mut().unwrap().remove("funding");
    assert_eq!(document, kernel);
    harness.restart(true, true);
    assert_eq!(
        harness.readiness().1["funding"],
        json!({"ready": false, "reason": "chain_unavailable"})
    );
}
