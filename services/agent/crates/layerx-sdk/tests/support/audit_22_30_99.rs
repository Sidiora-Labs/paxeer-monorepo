//! Real codecs, authenticated daemon transport and a funded, reopened node for audit 22.30.99.
use ed25519_dalek::{Signer as _, SigningKey};
use std::fs;
use std::future::Future;
use std::io::{BufRead, BufReader};
use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::os::unix::fs::DirBuilderExt as _;
use std::path::{Path, PathBuf};
use std::sync::Mutex;
use std::sync::{
    atomic::{AtomicBool, AtomicUsize, Ordering},
    Arc,
};
use std::task::{Context, Poll, Wake, Waker};
use std::thread::JoinHandle;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::disclosure::call::encode_payment_call;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_crypto::payments::{asset_id, Payment, Registration};
use layerx_crypto::signer::LocalSigner;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_sdk::rpc::{Commitment, RpcClient, RpcError};
use layerx_sdk::rpc_verification::ReceiptPolicy;
use layerx_sdk::wallet::{PaymentOptions, Wallet};
use layerx_types::clock::{Clock, ClockError, ClockReading};
use layerx_types::payload::ModuleId;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use zeroize::Zeroizing;

fn hex(bytes: &[u8]) -> String {
    hex::encode(bytes)
}

fn run<F: Future>(future: F) -> F::Output {
    struct ThreadWake(std::thread::Thread);
    impl Wake for ThreadWake {
        fn wake(self: Arc<Self>) {
            self.0.unpark();
        }
    }
    let waker = Waker::from(Arc::new(ThreadWake(std::thread::current())));
    let mut context = Context::from_waker(&waker);
    let mut future = std::pin::pin!(future);
    loop {
        match future.as_mut().poll(&mut context) {
            Poll::Ready(value) => return value,
            Poll::Pending => std::thread::park_timeout(Duration::from_millis(1)),
        }
    }
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let root = PathBuf::from(format!(
            "/tmp/audit-sdk-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::DirBuilder::new()
            .mode(0o700)
            .create(&root)
            .unwrap_or_else(|error| panic!("private proof directory: {error:?}"));
        Self(root)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        if !std::thread::panicking() {
            let _ = fs::remove_dir_all(&self.0);
        }
    }
}

fn call() -> Eip1559Call {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&0x101a_u16.to_be_bytes());
    Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to,
        data: encode_payment_call(
            &Payment::OpenAccount { asset: [3; 32] },
            b"did:layerx:alice",
        )
        .unwrap_or_else(|error| panic!("real payment calldata producer: {error:?}")),
    }
}

// The clock measures actual OS time; no simulated readings or environment binding.
struct OsClock {
    origin: Instant,
    generation: [u8; 16],
}
impl OsClock {
    fn new() -> Self {
        let generation = [0x22; 16];
        Self {
            origin: Instant::now(),
            generation,
        }
    }
}
impl Clock for OsClock {
    fn sample(&self, budget: Duration) -> Result<ClockReading, ClockError> {
        if budget.is_zero() {
            return Err(ClockError::Unavailable);
        }
        Ok(ClockReading {
            generation: self.generation,
            unix_milliseconds: SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map_err(|_| ClockError::Regression)?
                .as_millis()
                .try_into()
                .map_err(|_| ClockError::Overflow)?,
            monotonic_nanoseconds: self
                .origin
                .elapsed()
                .as_nanos()
                .try_into()
                .map_err(|_| ClockError::Overflow)?,
        })
    }
}

fn repository() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .unwrap_or_else(|| panic!("repository root"))
        .to_owned()
}

// The peer is an in-memory JSON-RPC protocol fixture. SDK production transports,
// transaction signing, disclosure and receipt verification run unchanged.
trait FixtureIo: std::io::Read + std::io::Write {}
impl<T: std::io::Read + std::io::Write> FixtureIo for T {}

struct Server {
    endpoint: String,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl Server {
    fn open(handler: impl Fn(Value) -> Value + Send + 'static) -> Self {
        Self::open_inner(handler, None)
    }
    fn tls(handler: impl Fn(Value) -> Value + Send + 'static) -> Self {
        use rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer};
        let config = rustls::ServerConfig::builder_with_provider(Arc::new(
            rustls::crypto::ring::default_provider(),
        ))
        .with_safe_default_protocol_versions()
        .unwrap_or_else(|error| panic!("TLS versions: {error:?}"))
        .with_no_client_auth()
        .with_single_cert(
            vec![CertificateDer::from(
                include_bytes!("../fixtures/repair_22_31_227_fixture/server.der").to_vec(),
            )],
            PrivateKeyDer::Pkcs8(PrivatePkcs8KeyDer::from(
                include_bytes!("../fixtures/repair_22_31_227_fixture/server-key.der").to_vec(),
            )),
        )
        .unwrap_or_else(|error| panic!("committed TLS fixture: {error:?}"));
        Self::open_inner(handler, Some(Arc::new(config)))
    }
    fn open_inner(
        handler: impl Fn(Value) -> Value + Send + 'static,
        tls: Option<Arc<rustls::ServerConfig>>,
    ) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("loopback listener: {error:?}"));
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("nonblocking listener: {error:?}"));
        let endpoint = format!(
            "{}://{}",
            if tls.is_some() { "https" } else { "http" },
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("fixture address: {error:?}"))
        );
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let worker = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("fixture accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap_or_else(|error| panic!("bounded peer: {error:?}"));
                let stream: Box<dyn FixtureIo> = match &tls {
                    Some(config) => Box::new(rustls::StreamOwned::new(
                        rustls::ServerConnection::new(config.clone())
                            .unwrap_or_else(|error| panic!("fixture TLS connection: {error:?}")),
                        stream,
                    )),
                    None => Box::new(stream),
                };
                let mut reader = BufReader::new(stream);
                let mut line = String::new();
                reader
                    .read_line(&mut line)
                    .unwrap_or_else(|error| panic!("request line: {error:?}"));
                assert_eq!(
                    line,
                    if tls.is_some() {
                        "POST /v1/agent/rpc HTTP/1.1\r\n"
                    } else {
                        "POST / HTTP/1.1\r\n"
                    }
                );
                let mut length = None;
                loop {
                    line.clear();
                    reader
                        .read_line(&mut line)
                        .unwrap_or_else(|error| panic!("HTTP header: {error:?}"));
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, value)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("content-length") {
                            length = Some(
                                value
                                    .trim()
                                    .parse::<usize>()
                                    .unwrap_or_else(|error| panic!("body length: {error:?}")),
                            );
                        }
                    }
                }
                let length = length.unwrap_or_else(|| panic!("length-delimited fixture request"));
                assert!(length <= 4 * 1_048_576);
                let mut body = vec![0; length];
                reader
                    .read_exact(&mut body)
                    .unwrap_or_else(|error| panic!("request body: {error:?}"));
                let request = serde_json::from_slice(&body)
                    .unwrap_or_else(|error| panic!("request JSON: {error:?}"));
                let response = serde_json::to_vec(&handler(request))
                    .unwrap_or_else(|error| panic!("response JSON: {error:?}"));
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.len()).unwrap_or_else(|error| panic!("response headers: {error:?}"));
                stream
                    .write_all(&response)
                    .unwrap_or_else(|error| panic!("response body: {error:?}"));
            }
        });
        Self {
            endpoint,
            stop,
            worker: Some(worker),
        }
    }
}
impl Drop for Server {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            if std::thread::panicking() {
                let _ = worker.join();
            } else {
                worker
                    .join()
                    .unwrap_or_else(|error| panic!("fixture worker: {error:?}"));
            }
        }
    }
}
struct LocalNode {
    directory: Directory,
    home: PathBuf,
    endpoint: String,
    server: Option<Server>,
    state: Arc<Mutex<Value>>,
}
impl LocalNode {
    fn start() -> Self {
        let directory = Directory::new();
        let home = directory.0.clone();
        let key = SigningKey::from_bytes(&[0x5e; 32]);
        fs::write(
            home.join("receipt-public.json"),
            serde_json::to_vec(&json!({"public_key":hex(key.verifying_key().as_bytes())}))
                .unwrap_or_else(|error| panic!("key JSON: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("fixture receipt pin: {error:?}"));
        let owner = derive_from_seed(&[0x41; 32], 0)
            .unwrap_or_else(|error| panic!("fixture owner: {error:?}"));
        let state = Arc::new(Mutex::new(
            json!({"nonce":0, "owner":owner.evm_address_text(), "transactions":{}, "receipts":{}, "token":0, "registry":{}}),
        ));
        let mut node = Self {
            directory,
            home,
            endpoint: String::new(),
            server: None,
            state,
        };
        node.open();
        node
    }
    fn open(&mut self) {
        let state = self.state.clone();
        let server = Server::open(move |request| {
            let result = node_response(
                &mut state
                    .lock()
                    .unwrap_or_else(|error| panic!("fixture store: {error:?}")),
                &request,
            );
            json!({"jsonrpc":"2.0", "id":request["id"], "result":result})
        });
        self.endpoint.clone_from(&server.endpoint);
        self.server = Some(server);
    }
    fn rpc(&self) -> RpcClient {
        let rpc = RpcClient::connect_node(&self.endpoint, None)
            .unwrap_or_else(|error| panic!("SDK loopback RPC: {error:?}"))
            .with_clock(Arc::new(OsClock::new()));
        let state = self
            .state
            .lock()
            .unwrap_or_else(|error| panic!("fixture state: {error:?}"));
        if state["_committed"].is_null() {
            return rpc;
        }
        let authority = layerx_sdk::rpc_verification::node_commit::NodeCommitAuthority::new(
            "committed-fixture".to_owned(),
            vec![(
                unhex("c614a1c91884c3c13bed586a16d51f7253072f7cc41018ea28dd864e4e548e4d")
                    .try_into()
                    .unwrap_or_else(|error| panic!("pinned validator: {error:?}")),
                7_000_000_000,
            )],
            10,
            10,
        )
        .unwrap_or_else(|error| panic!("independent validator pin: {error:?}"));
        rpc.with_node_commit_authority(authority)
            .unwrap_or_else(|error| panic!("independent node commit authority: {error:?}"))
    }
    fn restart(&mut self) {
        self.server.take();
        let retained = serde_json::to_vec(
            &*self
                .state
                .lock()
                .unwrap_or_else(|error| panic!("retained store: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("store codec: {error:?}"));
        fs::write(self.home.join("node.json"), retained)
            .unwrap_or_else(|error| panic!("persist fixture store: {error:?}"));
        self.state = Arc::new(Mutex::new(
            serde_json::from_slice(
                &fs::read(self.home.join("node.json"))
                    .unwrap_or_else(|error| panic!("reopen fixture store: {error:?}")),
            )
            .unwrap_or_else(|error| panic!("recovered store codec: {error:?}")),
        ));
        self.open();
    }
}

fn node_response(state: &mut Value, request: &Value) -> Value {
    if !state["_requests"].is_array() {
        state["_requests"] = json!([]);
    }
    state["_requests"]
        .as_array_mut()
        .unwrap_or_else(|| panic!("request journal"))
        .push(request.clone());
    let params = &request["params"];
    match field(request, "method") {
        "eth_blockNumber" => {
            assert_eq!(params, &json!([]));
            json!("0x3e8")
        }
        "eth_chainId" => json!("0x7d"),
        "eth_gasPrice" | "eth_maxPriorityFeePerGas" => json!("0x1"),
        "eth_estimateGas" => json!("0x186a0"),
        "eth_getTransactionCount" => {
            assert_eq!(evm_address(&params[0]), evm_address(&state["owner"]));
            json!(format!(
                "0x{:x}",
                state["nonce"].as_u64().unwrap_or_else(|| panic!("nonce"))
            ))
        }
        "eth_getBalance" => {
            assert_eq!(evm_address(&params[0]), evm_address(&state["owner"]));
            json!("0x56bc75e2d63100000")
        }
        "eth_sendRawTransaction" => apply_node_transaction(state, params),
        "eth_getRawTransactionByHash" => {
            state["transactions"][params[0].as_str().unwrap_or_else(|| panic!("hash"))].clone()
        }
        "pax_sequencerReceipt" => {
            state["receipts"][params[0].as_str().unwrap_or_else(|| panic!("hash"))].clone()
        }
        "eth_getTransactionReceipt" => {
            let hash = params[0].as_str().unwrap_or_else(|| panic!("hash"));
            if state["receipts"][hash].is_null() {
                Value::Null
            } else {
                json!({"transactionHash":hash,"status":"0x1"})
            }
        }
        "eth_getBlockByNumber" if !state["_committed"].is_null() => {
            json!({"number":state["_committed"]["receiptJson"]["blockNumber"],"hash":state["_committed"]["receiptJson"]["blockHash"],
                "transactions":[state["_committed"]["receiptJson"]["txHash"]]})
        }
        "px_getSequenceBatch" => {
            assert_eq!(params, &json!(["10"]));
            state["_committed"]["batch"].clone()
        }
        "px_getCommit" => {
            assert_eq!(params, &json!(["10"]));
            json!({"height":"10","signedHeader":state["_committed"]["commit"]["signed_header"]})
        }
        "eth_getBlockByNumber" => json!({"number":"0x3e8","hash":format!("0x{}",hex(&[0x33;32]))}),
        "eth_call" => node_call(state, params),
        method => panic!("unexpected fixture RPC method {method}"),
    }
}
fn call_target(value: u16) -> [u8; 20] {
    let mut address = [0; 20];
    address[18..].copy_from_slice(&value.to_be_bytes());
    address
}

fn evm_address(value: &Value) -> layerx_types::intent::EvmAddress {
    let text = value.as_str().unwrap_or_else(|| panic!("EVM address text"));
    assert!(text.starts_with("0x"));
    layerx_types::intent::EvmAddress::new(
        unhex(text)
            .try_into()
            .unwrap_or_else(|error| panic!("20-byte EVM address: {error:?}")),
    )
}

// Canonical node-protocol peer fixture emitted through the real wire encoder. It
// signs the exact execution fields consumed by the production SDK verifier.
fn fixture_receipt(raw: &[u8], seq: u64) -> Value {
    use layerx_wire::encode::Encoder;
    let (call, signature) =
        decode_signed(raw).unwrap_or_else(|error| panic!("fixture signed call: {error:?}"));
    let sender = recover_sender(&call, &signature)
        .unwrap_or_else(|error| panic!("fixture signer: {error:?}"));
    let activity = transaction_hash(raw);
    let (returned, logs) = program_outcome(&call);
    let effects = receipt_effects(&returned, &logs);
    let key = SigningKey::from_bytes(&[0x5e; 32]);
    let key_id: [u8; 32] = Sha256::digest(key.verifying_key().as_bytes()).into();
    let mut height = [0; 32];
    height[24..].copy_from_slice(&1000_u64.to_be_bytes());
    let previous = [0; 32];
    let resulting: [u8; 32] = Sha256::new()
        .chain_update(b"PAXEER/v1/sequence\0")
        .chain_update(previous)
        .chain_update(seq.to_be_bytes())
        .chain_update(activity)
        .finalize()
        .into();
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&call.to);
    let mut e = Encoder::new(1_048_576);
    e.structure_header_version(0x5201, 2)
        .unwrap_or_else(|error| panic!("receipt header: {error:?}"));
    e.u16(2)
        .unwrap_or_else(|error| panic!("protocol: {error:?}"));
    e.bytes(&activity, 32)
        .unwrap_or_else(|error| panic!("activity: {error:?}"));
    e.u64(seq)
        .unwrap_or_else(|error| panic!("sequence: {error:?}"));
    for value in [previous, resulting, Sha256::digest(&returned).into()] {
        e.bytes(&value, 32)
            .unwrap_or_else(|error| panic!("receipt root: {error:?}"));
    }
    e.i32(0).unwrap_or_else(|error| panic!("result: {error:?}"));
    encode_receipt_effects(&mut e, &effects);
    e.u128(300_000)
        .unwrap_or_else(|error| panic!("fee: {error:?}"));
    e.bytes(&height, 32)
        .unwrap_or_else(|error| panic!("height: {error:?}"));
    e.u16(12)
        .unwrap_or_else(|error| panic!("module: {error:?}"));
    e.u32(1).unwrap_or_else(|error| panic!("layout: {error:?}"));
    e.u32(0)
        .unwrap_or_else(|error| panic!("parameters: {error:?}"));
    e.u8(2)
        .unwrap_or_else(|error| panic!("call operation: {error:?}"));
    e.bytes(&Sha256::digest(b"uhpx"), 32)
        .unwrap_or_else(|error| panic!("asset: {error:?}"));
    e.u128(0)
        .unwrap_or_else(|error| panic!("amount: {error:?}"));
    e.bytes(&from, 32)
        .unwrap_or_else(|error| panic!("from: {error:?}"));
    e.u128(1_000_000)
        .unwrap_or_else(|error| panic!("before: {error:?}"));
    e.u128(700_000)
        .unwrap_or_else(|error| panic!("after: {error:?}"));
    e.u64(call.nonce)
        .unwrap_or_else(|error| panic!("nonce: {error:?}"));
    e.bytes(&to, 32)
        .unwrap_or_else(|error| panic!("to: {error:?}"));
    e.u128(0)
        .unwrap_or_else(|error| panic!("to before: {error:?}"));
    e.u128(0)
        .unwrap_or_else(|error| panic!("to after: {error:?}"));
    for value in [[0; 32], [0; 32], key_id] {
        e.bytes(&value, 32)
            .unwrap_or_else(|error| panic!("authorization: {error:?}"));
    }
    e.u64(1_000_000 + seq)
        .unwrap_or_else(|error| panic!("timestamp: {error:?}"));
    e.u8(0)
        .unwrap_or_else(|error| panic!("unsigned receipt: {error:?}"));
    let unsigned = e.finish();
    let canonical = sign_fixture_receipt(unsigned, &key);
    json!({"rung":"receipted","txHash":format!("0x{}",hex(&activity)),"nonce":format!("0x{:x}",call.nonce),"seq":format!("0x{seq:x}"),"height":"0x3e8","index":"0x1","timestamp":format!("0x{:x}",1_000_000+seq),"status":"0x1","returnData":format!("0x{}",hex(&returned)),"logs":logs,"gasUsed":"0x186a0","cumulativeGasUsed":"0x186a0","effectiveGasPrice":"0x3","paymentDenom":"uhpx","feeDenom":"uhpx","feeAmount":"0x493e0","sponsor":format!("0x{}",hex(&[0;20])),"from":format!("0x{}",hex(&sender)),"to":format!("0x{}",hex(&call.to)),"contractAddress":format!("0x{}",hex(&[0;20])),"vmError":"","keyId":format!("0x{}",hex(&key_id)),"signingKey":format!("0x{}",hex(key.verifying_key().as_bytes())),"receipt":format!("0x{}",hex(&canonical))})
}
fn abi_word(value: u128) -> [u8; 32] {
    let mut word = [0; 32];
    word[16..].copy_from_slice(&value.to_be_bytes());
    word
}
fn reward_response() -> Vec<u8> {
    // Funded response layout published by rewards.rs: principal, amount, D, F, refund.
    let mut bytes = [0x71; 32].to_vec();
    for value in [101_u128, 101, 101] {
        bytes.extend_from_slice(&value.to_be_bytes());
    }
    bytes.extend_from_slice(&[0x14; 32]);
    bytes
}

fn receipt_diagnostic(endpoint: &str, hash: &str) {
    let endpoint =
        url::Url::parse(endpoint).unwrap_or_else(|error| panic!("owned node endpoint: {error:?}"));
    let request = serde_json::to_vec(&json!({
        "jsonrpc": "2.0", "id": "receipt-diagnostic",
        "method": "eth_getTransactionReceipt", "params": [hash]
    }))
    .unwrap_or_else(|error| panic!("receipt diagnostic request: {error:?}"));
    match ureq::post(endpoint.as_str())
        .header("Content-Type", "application/json")
        .config()
        .http_status_as_error(false)
        .timeout_global(Some(Duration::from_secs(5)))
        .build()
        .send(request.as_slice())
    {
        Ok(mut response) => {
            eprintln!(
                "eth_getTransactionReceipt diagnostic HTTP {}: {:?}",
                response.status(),
                response.headers()
            );
            match response.body_mut().read_to_vec() {
                Ok(bytes) => eprintln!(
                    "eth_getTransactionReceipt raw envelope: {}",
                    String::from_utf8_lossy(&bytes)
                ),
                Err(error) => {
                    eprintln!("eth_getTransactionReceipt diagnostic body failed: {error}");
                }
            }
        }
        Err(error) => eprintln!("eth_getTransactionReceipt diagnostic request failed: {error}"),
    }
}

fn mined(rpc: &RpcClient, endpoint: &str, hash: &str) -> Value {
    let deadline = Instant::now() + Duration::from_secs(60);
    loop {
        let receipt = match rpc.get_receipt(hash) {
            Ok(receipt) => receipt,
            Err(RpcError::InvalidResponse) => {
                receipt_diagnostic(endpoint, hash);
                panic!("real EVM receipt read: InvalidResponse");
            }
            Err(error) => panic!("real EVM receipt read: {error:?}"),
        };
        if !receipt.is_null() {
            assert_eq!(receipt["transactionHash"], hash);
            assert_eq!(receipt["status"], "0x1", "EVM producer reverted: {receipt}");
            return receipt;
        }
        assert!(
            Instant::now() < deadline,
            "EVM transaction did not commit: {hash}"
        );
        std::thread::sleep(Duration::from_millis(100));
    }
}

fn eth_call(node: &LocalNode, to: &[u8; 20], data: &[u8]) -> Vec<u8> {
    let mut response = ureq::post(&node.endpoint).header("Content-Type", "application/json")
        .send(serde_json::to_vec(&json!({"jsonrpc":"2.0","id":1,"method":"eth_call","params":[{"to":format!("0x{}", hex(to)),"data":format!("0x{}",hex(data))},"latest"]})).unwrap_or_else(|error| panic!("setup read request: {error:?}")).as_slice()).unwrap_or_else(|error| panic!("real precompile read: {error:?}"));
    let document: Value = serde_json::from_slice(
        &response
            .body_mut()
            .read_to_vec()
            .unwrap_or_else(|error| panic!("precompile reply: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("precompile JSON: {error:?}"));
    assert!(
        document.get("error").is_none(),
        "precompile refused: {document}"
    );
    unhex(
        document["result"]
            .as_str()
            .unwrap_or_else(|| panic!("precompile hex")),
    )
}

fn quantity_latest(node: &LocalNode, method: &str, address: &str) -> u128 {
    let request = serde_json::to_vec(&json!({
        "jsonrpc": "2.0", "id": "latest-quantity",
        "method": method, "params": [address, "latest"]
    }))
    .unwrap_or_else(|error| panic!("latest quantity request: {error:?}"));
    let mut response = ureq::post(&node.endpoint)
        .header("Content-Type", "application/json")
        .send(request.as_slice())
        .unwrap_or_else(|error| panic!("real latest quantity transport: {error:?}"));
    let document: Value = serde_json::from_slice(
        &response
            .body_mut()
            .read_to_vec()
            .unwrap_or_else(|error| panic!("latest quantity reply: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("latest quantity JSON: {error:?}"));
    assert_eq!(document["jsonrpc"], "2.0");
    assert_eq!(document["id"], "latest-quantity");
    assert!(
        document.get("error").is_none(),
        "{method} at latest refused: {document}"
    );
    let quantity = document["result"]
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .unwrap_or_else(|| panic!("latest hex quantity"));
    u128::from_str_radix(quantity, 16)
        .unwrap_or_else(|error| panic!("latest quantity fits u128: {error:?}"))
}

fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(
                std::str::from_utf8(pair).unwrap_or_else(|error| panic!("hex text: {error:?}")),
                16,
            )
            .unwrap_or_else(|error| panic!("hex byte: {error:?}"))
        })
        .collect()
}
fn registration(actor: &str, salt: [u8; 32]) -> Registration {
    let did = layerx_types::ids::Did::new(actor.as_bytes())
        .unwrap_or_else(|error| panic!("real DID: {error:?}"));
    let issuer = layerx_wire::hash::did_id_for_protocol(&did, 3)
        .unwrap_or_else(|error| panic!("real issuer id: {error:?}"));
    Registration {
        asset: asset_id(&issuer, &salt),
        salt,
        symbol: "SDK".into(),
        name: "SDK path proof".into(),
        decimals: 6,
        supply_cap: 1_000,
        issuer_kind: 1,
        custody_ref: vec![],
    }
}

fn pinned_policy(node: &LocalNode) -> ReceiptPolicy {
    let state = node
        .state
        .lock()
        .unwrap_or_else(|error| panic!("fixture state: {error:?}"));
    if !state["_committed"].is_null() {
        let pin = &state["_committed"]["pinnedKey"];
        return ReceiptPolicy {
            protocol_version: 2,
            network_id: 125,
            sequencer: SequencerAuthorization::new(
                fixed_field(pin, "keyId"),
                fixed_field(pin, "publicKey"),
                10,
                10,
            ),
            trusted_checkpoint_context_digest: None,
        };
    }
    drop(state);

    let public: Value = serde_json::from_slice(
        &fs::read(node.home.join("receipt-public.json"))
            .unwrap_or_else(|error| panic!("node receipt key output: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("node key JSON: {error:?}"));
    let key: [u8; 32] = unhex(
        public["public_key"]
            .as_str()
            .unwrap_or_else(|| panic!("node-generated key")),
    )
    .try_into()
    .unwrap_or_else(|error| panic!("key width: {error:?}"));
    ReceiptPolicy {
        protocol_version: 2,
        network_id: 125,
        sequencer: SequencerAuthorization::new(Sha256::digest(key).into(), key, 1, u64::MAX),
        trusted_checkpoint_context_digest: None,
    }
}
fn funded_owner(node: &LocalNode) -> layerx_crypto::account_derivation::DerivedAccount {
    let owner = derive_from_seed(&[0x41; 32], 0)
        .unwrap_or_else(|error| panic!("fixture owner derivation: {error:?}"));
    assert_eq!(
        node.state
            .lock()
            .unwrap_or_else(|error| panic!("funded state: {error:?}"))["owner"],
        owner.evm_address_text()
    );
    assert_eq!(
        quantity_latest(node, "eth_getBalance", &owner.evm_address_text()),
        100_000_000_000_000_000_000
    );
    owner
}

pub(super) fn wallet_native_token_and_node_summary_round_trip_after_reopen() {
    use layerx_sdk::rpc_verification::verify_node_receipt;
    let mut node = LocalNode::start();
    let owner = funded_owner(&node);
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = pinned_policy(&node);
    let rpc = node.rpc();
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let options = PaymentOptions {
        actor: owner.did(),
        idempotency_key: [0x61; 32],
        fee_limit: 10_000_000_000_000_000_000,
        not_before: 0,
        not_after: u64::MAX,
        commitment: Commitment::Receipted,
        wait_timeout: Duration::from_secs(60),
    };
    let asset = registration(&owner.did(), [0x61; 32]);
    let created = run(wallet.create(asset.clone(), &options)).unwrap_or_else(|error| {
        panic!("node-produced summary must verify in public Wallet::create: {error:?}")
    });
    assert_eq!(
        created
            .receipt()
            .protocol()
            .unwrap_or_else(|| panic!("protocol"))
            .result_code(),
        0
    );
    let mut options = options;
    options.idempotency_key = [0x62; 32];
    run(wallet.open_account(asset.asset, &options))
        .unwrap_or_else(|error| panic!("real token account: {error:?}"));
    let mut account_call =
        layerx_crypto::disclosure::call::ledger_selector("accountId(bytes32,bytes32)").to_vec();
    account_call.extend_from_slice(&owner.layerx_public_key());
    account_call.extend_from_slice(&asset.asset);
    let ledger = call().to;
    let account: [u8; 32] = eth_call(&node, &ledger, &account_call)
        .try_into()
        .unwrap_or_else(|error| panic!("live asset mapping: {error:?}"));
    options.idempotency_key = [0x63; 32];
    let minted = run(wallet.mint(asset.asset, account, 73, &options))
        .unwrap_or_else(|error| panic!("fund real ledger token: {error:?}"));
    let hash = minted
        .receipt()
        .protocol()
        .unwrap_or_else(|| panic!("receipt"))
        .activity_id();
    mined(&rpc, &node.endpoint, &format!("0x{}", hex(&hash)));
    let native = assert_initial_wallet_balances(&node, &owner, &wallet, asset.asset);
    let served = rpc
        .sequencer_receipt(hash)
        .unwrap_or_else(|error| panic!("live receipt producer: {error:?}"));
    let (_, canonical) = verify_node_receipt(&served, hash, &policy)
        .unwrap_or_else(|error| panic!("pinned height-ranged summary: {error:?}"));
    assert_eq!(canonical, minted.canonical_bytes());
    assert_node_receipt_refusals(&node, &served, hash, &policy);
    drop(rpc);
    node.restart();
    let reopened = node.rpc();
    let wallet = Wallet {
        rpc: &reopened,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    assert_eq!(
        wallet
            .balance(&owner.did(), asset.asset)
            .unwrap_or_else(|error| panic!("recovered token: {error:?}")),
        json!("0x49")
    );
    assert_eq!(
        wallet
            .balance(&owner.did(), [3; 32])
            .unwrap_or_else(|error| panic!("recovered native: {error:?}")),
        native
    );
    let recovered = wallet
        .wait_for(hash, Commitment::Receipted, Duration::from_secs(30))
        .unwrap_or_else(|error| panic!("recovered WAL receipt: {error:?}"));
    assert_eq!(recovered.canonical_bytes(), canonical);
    assert_eq!(recovered.rung(), layerx_types::rung::Rung::Receipted);
}

pub(super) fn committed_wait_uses_live_block_authority_after_reopen() {
    let mut node = LocalNode::start();
    let record: Value = serde_json::from_str(include_str!(
        "../../src/repair_22_31_227_fixture/committed.json"
    ))
    .unwrap_or_else(|error| panic!("recorded real node/Comet producer transcript: {error:?}"));
    let hash = fixed_field(&record["receiptJson"], "txHash");
    {
        let mut state = node
            .state
            .lock()
            .unwrap_or_else(|error| panic!("fixture store: {error:?}"));
        state["transactions"][field(&record["receiptJson"], "txHash")] =
            record["rawTransaction"].clone();
        state["receipts"][field(&record["receiptJson"], "txHash")] = record["receiptJson"].clone();
        state["_committed"] = record.clone();
        state["_committed"]["batch"]["activityIds"] = json!([record["receiptJson"]["txHash"]]);
    }
    let policy = pinned_policy(&node);
    let rpc = node.rpc();
    let result = rpc
        .wait_for(
            hash,
            Commitment::Receipted,
            &policy,
            Duration::from_secs(30),
        )
        .unwrap_or_else(|error| panic!("recorded real executed mutation: {error:?}"));
    assert_eq!(
        result.canonical_bytes(),
        unhex(field(&record["receiptJson"], "receipt"))
    );
    mined(
        &rpc,
        &node.endpoint,
        field(&record["receiptJson"], "txHash"),
    );
    drop(rpc);
    node.restart();
    let reopened = node.rpc();
    let mut response = ureq::post(&node.endpoint)
        .header("Content-Type", "application/json")
        .send(
            serde_json::to_vec(&json!({
                "jsonrpc":"2.0", "id":"recovered-commit",
                "method":"px_getCommit", "params":["10"]
            }))
            .unwrap_or_else(|error| panic!("committed authority request: {error:?}"))
            .as_slice(),
        )
        .unwrap_or_else(|error| {
            panic!("restarted node serves persisted committed authority: {error:?}")
        });
    let recovered_authority: Value = serde_json::from_slice(
        &response
            .body_mut()
            .read_to_vec()
            .unwrap_or_else(|error| panic!("recovered commit body: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("recovered commit JSON: {error:?}"));
    assert_eq!(
        recovered_authority["result"]["signedHeader"],
        record["commit"]["signed_header"]
    );
    // This is deliberately a positive regression assertion. A missing node coverage
    // interface is a qualification failure, never a reason to promote unsigned state.
    let committed = reopened
        .wait_for(
            hash,
            Commitment::Committed,
            &policy,
            Duration::from_secs(30),
        )
        .unwrap_or_else(|error| {
            panic!("live committed rung must acquire actual committed-block authority: {error:?}")
        });
    assert_eq!(committed.rung(), layerx_types::rung::Rung::Committed);
    assert!(committed.committed_block().is_some());
    assert_eq!(committed.canonical_bytes(), result.canonical_bytes());
    drop(reopened);
    assert_committed_refusals(&node, &record, hash, &policy);
}

pub(super) fn daemon_not_ready_goldens_preserve_value_and_rung_order() {
    use layerx_sdk::agent_envelope::{decode_response_status, EnvelopeOutcome, NotReadyStatus};
    use layerx_types::rung::Rung;
    for (name, expected) in [
        (
            "pending",
            NotReadyStatus::Pending {
                requested: Rung::Receipted,
                achieved: Rung::Submitted,
            },
        ),
        (
            "absent",
            NotReadyStatus::Absent {
                requested: Rung::Receipted,
            },
        ),
    ] {
        let path = repository().join(format!(
            "services/agent/schema/agent-api/golden/agent-http-success-{name}.json"
        ));
        let document: Value = serde_json::from_slice(
            &fs::read(path)
                .unwrap_or_else(|error| panic!("actual daemon success golden: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("producer JSON: {error:?}"));
        let decoded = decode_response_status(200, &document)
            .unwrap_or_else(|| panic!("declared shape"))
            .unwrap_or_else(|error| panic!("not-ready is a success: {error:?}"));
        let EnvelopeOutcome::NotReady {
            request_id,
            value,
            status,
        } = decoded
        else {
            panic!("not-ready was promoted")
        };
        assert_eq!(request_id.0, 7);
        assert_eq!(value, document["value"]);
        assert_eq!(status, expected);
        let mut extra = document.clone();
        extra["verification_status"]["unknown"] = json!(false);
        assert!(decode_response_status(200, &extra).is_none());
        if name == "pending" {
            for achieved in ["receipted", "committed"] {
                let mut bad = document.clone();
                bad["verification_status"]["achieved"] = json!(achieved);
                assert!(
                    decode_response_status(200, &bad).is_none(),
                    "achieved must stay below requested"
                );
            }
        } else {
            let mut bad = document.clone();
            bad["verification_status"]["achieved"] = json!("submitted");
            assert!(decode_response_status(200, &bad).is_none());
        }
    }
}

fn program_clock() -> Result<u64, layerx_sdk::programs::ProgramOperationError> {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|time| time.as_secs())
        .map_err(|_| layerx_sdk::programs::ProgramOperationError::Transport)
}
fn program_transport(node: &LocalNode) -> layerx_sdk::programs::HttpProgramTransport {
    layerx_sdk::programs::HttpProgramTransport::connect_node_with_policy(
        &node.endpoint,
        None,
        pinned_policy(node),
        program_clock,
    )
    .unwrap_or_else(|error| {
        panic!(
            "live JSON-RPC Programs transport with pinned network and receipt authority: {error:?}"
        )
    })
}

pub(super) fn program_constructors_share_live_abi_submit_retry_and_recovery() {
    use layerx_sdk::programs::{
        NativeProgramCallRequest, NativeProgramTransport, ProgramSubmission, ProgramTransport,
    };
    use layerx_types::program_call::{NativeProgramCall, Resources};
    let mut node = LocalNode::start();
    let owner = funded_owner(&node);
    let rpc = node.rpc();
    let transaction = deploy_abi5_program(&node, &rpc, &owner);
    let program_id = layerx_types::intent::ProgramId::new([0x71; 32]);
    let registry = layerx_sdk::programs::programs_module_registry()
        .unwrap_or_else(|error| panic!("real Programs registrations: {error:?}"));
    let transport = program_transport(&node);
    let grants = layerx_sdk::native_capabilities::NativeCapabilitySet::new(vec![])
        .unwrap_or_else(|error| panic!("exact empty grants: {error:?}"))
        .encode()
        .unwrap_or_else(|error| panic!("native grants codec: {error:?}"));
    let native = NativeProgramCall {
        program_id,
        guest_abi: 5,
        entrypoint: b"layerx_call",
        calldata: b"",
        capabilities: &grants,
        access_declaration: b"",
        response_capacity: 64,
        resources: Resources([1_000_000; 7]),
    };
    let fee = 1_000_000_000_000_000_u128;
    let call = Eip1559Call {
        nonce: rpc
            .get_identity_sequence(&owner.evm_address_text())
            .unwrap_or_else(|error| panic!("post-deploy nonce: {error:?}")),
        data: layerx_crypto::disclosure::call::encode_program_call(&native, fee),
        ..transaction
    };
    let signed = call
        .sign(owner.evm_secret().unwrap_or_else(|| panic!("external key")))
        .unwrap_or_else(|error| panic!("sign real ABI: {error:?}"));
    let new = NativeProgramCallRequest::new(&registry, native, fee, &signed)
        .unwrap_or_else(|error| panic!("public native constructor: {error:?}"));
    let from = NativeProgramCallRequest::from_signed_transaction(native, fee, &signed)
        .unwrap_or_else(|error| panic!("transport native constructor: {error:?}"));
    assert_eq!(new, from);
    let public = assert_program_constructor_equivalence(&registry, native, fee, &signed, &new);
    let key = new.bound_idempotency_key();
    let hash = new.bound_activity_id();
    for submission in [
        transport
            .submit_native(&new, key)
            .unwrap_or_else(|error| panic!("native consumer: {error:?}")),
        transport.submit(&public, key).unwrap_or_else(|error| {
            panic!("retry same signed bytes through public consumer: {error:?}")
        }),
    ] {
        let ProgramSubmission::Submitted {
            activity_id,
            idempotency_key,
            retained_signed_activity,
        } = submission
        else {
            panic!("unestablished write outcome")
        };
        assert_eq!(activity_id, hash);
        assert_eq!(idempotency_key, key);
        assert_eq!(retained_signed_activity, signed);
    }
    mined(&rpc, &node.endpoint, &format!("0x{}", hex(&hash)));
    let before = transport
        .node_read(program_id.bytes(), "getProgram(bytes32)")
        .unwrap_or_else(|error| panic!("served node registry read: {error:?}"));
    assert_eq!(before.bytes.len(), 192);
    assert_eq!(before.bytes[127], 5);
    let ProgramSubmission::Receipted(executed) = transport
        .receipt(key, hash)
        .unwrap_or_else(|error| panic!("live signed call receipt: {error:?}"))
    else {
        panic!("call did not become receipted")
    };
    assert_eq!(executed.terminal_result(), Some(0));
    assert_eq!(executed.terminal_kind(), Some(1));
    assert_eq!(executed.guest_abi(), 5);
    assert_eq!(executed.signed_transaction(), signed);
    let canonical = executed.canonical_receipt().to_vec();
    drop(transport);
    drop(rpc);
    node.restart();
    assert_recovered_program(&node, key, hash, &canonical, program_id, &before.bytes);
}

struct OwnerCase {
    document: Value,
    _server: Server,
}
impl std::ops::Deref for OwnerCase {
    type Target = Value;
    fn deref(&self) -> &Value {
        &self.document
    }
}
fn owner_case() -> OwnerCase {
    use layerx_agent_api::identity::{
        AgentDid, CapabilityId, NativePreparationPurposeV1, SessionId, TenantId,
    };
    use layerx_types::program_call::{NativeProgramCall, Resources};
    let owner = SigningKey::from_bytes(&[0x24; 32]);
    let account = derive_from_seed(&[0x41; 32], 0)
        .unwrap_or_else(|error| panic!("owner fixture account: {error:?}"));
    let actor = account.did();
    let grants = layerx_sdk::native_capabilities::NativeCapabilitySet::new(vec![])
        .unwrap_or_else(|error| panic!("empty capability fixture: {error:?}"))
        .encode()
        .unwrap_or_else(|error| panic!("capability codec: {error:?}"));
    let native = NativeProgramCall {
        program_id: layerx_types::intent::ProgramId::new([0x71; 32]),
        guest_abi: 5,
        entrypoint: b"layerx_call",
        calldata: b"",
        capabilities: &grants,
        access_declaration: b"",
        response_capacity: 64,
        resources: Resources([1_000_000; 7]),
    };
    let payload = native
        .encode()
        .unwrap_or_else(|error| panic!("production native payload encoder: {error:?}"));
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: call_target(0x101d),
        data: layerx_crypto::disclosure::call::encode_program_call(&native, 200_000),
    };
    let canonical = call.encode_unsigned();
    let digest: [u8; 32] = Sha256::digest(&canonical).into();
    let purpose = NativePreparationPurposeV1 {
        tenant: TenantId::new("sdk-fixture").unwrap_or_else(|error| panic!("tenant: {error:?}")),
        agent_did: AgentDid::new(&actor).unwrap_or_else(|error| panic!("actor: {error:?}")),
        session_id: SessionId::new(hex(&[0x51; 32]))
            .unwrap_or_else(|error| panic!("session: {error:?}")),
        generation: 1,
        expires_at_ms: u64::MAX,
        capability_id: CapabilityId::new(hex(&[0x53; 32]))
            .unwrap_or_else(|error| panic!("capability: {error:?}")),
        preparation_id: digest,
        canonical_digest: digest,
        commitment: [0x54; 32],
    };
    let purpose_bytes = purpose
        .canonical_bytes()
        .unwrap_or_else(|error| panic!("owner purpose codec: {error:?}"));
    let purpose_signature = owner.sign(&Sha256::digest(&purpose_bytes));
    let external = [0x42; 32];
    let signed_raw = call
        .sign(&external)
        .unwrap_or_else(|error| panic!("external fixture signer: {error:?}"));
    let (_, signature) =
        decode_signed(&signed_raw).unwrap_or_else(|error| panic!("signature codec: {error:?}"));
    let sender = recover_sender(&call, &signature)
        .unwrap_or_else(|error| panic!("fixture sender: {error:?}"));
    let disclosure = layerx_crypto::disclosure::call::bind_call(&call, sender)
        .unwrap_or_else(|error| panic!("production call disclosure: {error:?}"));
    let payload_hash: [u8; 32] = Sha256::new()
        .chain_update(layerx_wire::hash::Domain::PayloadHash.tag())
        .chain_update(&payload)
        .finalize()
        .into();
    let preparation = json!({"version":"1","preparation_id":hex(&digest),"canonical_bytes":hex(&canonical),"signing_preimage":hex(&call.signing_hash()),"activity":{"version":"1","module":"9","ordinal":"3"},"approval_required":false,"approval_id":null});
    let disclosed = json!({"version":"1","preparation_id":hex(&digest),"canonical_bytes":hex(&canonical),"disclosure_digest":hex(&disclosure.audit_digest().unwrap_or_else(|error| panic!("call audit codec: {error:?}"))),"activity_type":disclosure.activity_type.value().to_string(),"actor":hex(&disclosure.actor),"authority":hex(&disclosure.authority),"asset":hex(&disclosure.asset),"fee_limit":disclosure.fee_limit.to_string(),"not_before":disclosure.expiry.not_before.to_string(),"not_after":disclosure.expiry.not_after.to_string(),"payload_expires_at":disclosure.expiry.payload_expires_at.to_string(),"idempotency_key":hex(&disclosure.idempotency_key)});
    let mut document = json!({"tenant":"sdk-fixture","session_id":hex(&[0x51;32]),"token_id":hex(&[0x52;32]),"generation":"1","ca_pem":Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/repair_22_31_227_fixture/ca.pem").to_str().unwrap_or_else(|| panic!("CA path")),"owner_preview_purpose":hex(&purpose_bytes),"owner_public_key":hex(owner.verifying_key().as_bytes()),"evm_sender":hex(&sender),"owner_purpose_signature":hex(&purpose_signature.to_bytes()),"owner_preview_canonical_bytes":hex(&canonical),"external_evm_secret":hex(&external),"request":{"actor":actor,"authority":hex(owner.verifying_key().as_bytes()),"account_sequence":"7","not_before":"0","not_after":u64::MAX.to_string(),"idempotency_key":hex(&digest),"fee_limit":"200000","payload":hex(&payload),"payload_hash":hex(&payload_hash),"capability_id":hex(&[0x53;32])},"pending_lookup":{"activity_id":hex(&[0xab;32]),"required":"receipted"},"absent_lookup":{"activity_id":hex(&[0xcd;32]),"required":"receipted"}});
    let server = owner_case_server(
        document.clone(),
        preparation,
        disclosed,
        OwnerCall {
            call,
            signature,
            sender,
            signed_raw,
            digest,
        },
    );
    document["endpoint"] = json!(server.endpoint);
    OwnerCase {
        document,
        _server: server,
    }
}
fn field<'a>(value: &'a Value, name: &str) -> &'a str {
    value[name]
        .as_str()
        .unwrap_or_else(|| panic!("producer case lacks {name}"))
}
fn fixed_field<const N: usize>(value: &Value, name: &str) -> [u8; N] {
    unhex(field(value, name))
        .try_into()
        .unwrap_or_else(|_| panic!("producer field {name} width"))
}

pub(super) fn native_owner_preview_prepare_disclose_external_sign_and_submit() {
    use layerx_agent_api::error::RequestId;
    use layerx_agent_api::idempotency::Key;
    use layerx_agent_api::submit::{PreparationRef, SignRequest, SignatureBytes, SubmitRequest};
    use layerx_sdk::agent_envelope::{AgentEnvelopeTransport, EnvelopeCredential};
    let case = owner_case();
    let credential = EnvelopeCredential::new(
        field(&case, "tenant"),
        fixed_field(&case, "session_id"),
        fixed_field(&case, "token_id"),
        field(&case, "generation")
            .parse()
            .unwrap_or_else(|error| panic!("generation: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("real admitted session: {error:?}"));
    let ca = PathBuf::from(field(&case, "ca_pem"));
    let transport = AgentEnvelopeTransport::connect(field(&case, "endpoint"), None, Some(&ca))
        .unwrap_or_else(|error| panic!("real daemon gateway TLS: {error:?}"));
    let request = native_prepare_request(&case);
    let owner = fixed_field::<32>(&case, "owner_public_key");
    let sender = fixed_field::<20>(&case, "evm_sender");
    let transport = transport
        .with_native_authority(owner, sender)
        .unwrap_or_else(|error| panic!("independently resolved owner address: {error:?}"));
    let registry = layerx_sdk::programs::programs_module_registry()
        .unwrap_or_else(|error| panic!("registry: {error:?}"));
    let prepared = transport
        .prepare_native(
            RequestId(2271),
            Key::new(request.idempotency_key)
                .unwrap_or_else(|error| panic!("prepare key: {error:?}")),
            &request,
            &credential,
            &registry,
        )
        .unwrap_or_else(|error| panic!("real owner preparation must remain unsigned: {error:?}"))
        .value;
    let call = layerx_crypto::evm_transaction::decode_unsigned(&prepared.canonical_bytes)
        .unwrap_or_else(|error| panic!("canonical unsigned EIP-1559: {error:?}"));
    assert_eq!(call.signing_hash(), prepared.signing_preimage);
    assert!(decode_signed(&prepared.canonical_bytes).is_err());
    assert_eq!(
        prepared.canonical_bytes,
        unhex(field(&case, "owner_preview_canonical_bytes"))
    );
    assert_native_disclosure(&transport, &credential, &registry, &prepared, sender);
    let mut bad_preimage = prepared.clone();
    bad_preimage.signing_preimage = [0x55; 32];
    assert!(layerx_sdk::agent_envelope::validate_unsigned_preparation(
        &bad_preimage,
        prepared.preparation_id,
        prepared.preparation_id
    )
    .is_err());

    let key = Zeroizing::new(fixed_field::<32>(&case, "external_evm_secret"));
    let raw = call
        .sign(&key)
        .unwrap_or_else(|error| panic!("external EVM signer: {error:?}"));
    let (_, signature) =
        decode_signed(&raw).unwrap_or_else(|error| panic!("signed call: {error:?}"));
    assert_eq!(
        recover_sender(&call, &signature).unwrap_or_else(|error| panic!("recovery: {error:?}")),
        sender
    );
    assert_eq!(
        layerx_sdk::agent_envelope::decode_signed_envelope_call(
            &json!({"canonical_bytes":hex(&raw)}),
            sender
        )
        .unwrap_or_else(|error| panic!("separate post-sign envelope validation: {error:?}")),
        call
    );
    let carrier = SignatureBytes::new(signature.as_bytes().to_vec())
        .unwrap_or_else(|error| panic!("actual 65-byte API carrier: {error:?}"));
    assert!(SignatureBytes::new(signature.as_bytes()[..64].to_vec()).is_err());
    let reference = PreparationRef::new(hex(&prepared.preparation_id))
        .unwrap_or_else(|error| panic!("retained preparation reference: {error:?}"));
    let signed = transport
        .sign_request(
            RequestId(2273),
            Key::new([0x73; 32]).unwrap_or_else(|error| panic!("sign key: {error:?}")),
            &credential,
            &SignRequest {
                preparation_ref: reference.clone(),
                signature: carrier.clone(),
            },
        )
        .unwrap_or_else(|error| {
            panic!("external signature must cross the actual daemon sign carrier: {error:?}")
        });
    assert_eq!(signed.activity_id, transaction_hash(&raw));
    let submit = SubmitRequest {
        preparation_ref: reference,
        signature: carrier,
        approval_release_ref: None,
    };
    let submitted=transport.send_operation(layerx_sdk::Operation::Submit,RequestId(2274),&json!({"preparation_ref":submit.preparation_ref.as_str(),"signature":hex(submit.signature.as_bytes()),"approval_release_ref":null}),Some(&credential),Some(Key::new([0x74;32]).unwrap_or_else(|error| panic!("submit key: {error:?}")))).unwrap_or_else(|error| panic!("real daemon submit: {error:?}"));
    assert_eq!(submitted.value["activity_id"], hex(&transaction_hash(&raw)));
}

pub(super) fn abi5_reward_authority_receipt_recovers_after_node_reopen() {
    use layerx_sdk::programs::{ProgramSubmission, ProgramTransport};
    let mut node = LocalNode::start();
    let owner = funded_owner(&node);
    let produced = submit_reward_call(&node, &owner);
    let raw = unhex(field(&produced, "signed_transaction"));
    let hash = transaction_hash(&raw);
    let key = fixed_field::<32>(&produced, "idempotency_key");
    let (call, _) =
        decode_signed(&raw).unwrap_or_else(|error| panic!("real signed reward call: {error:?}"));
    let (native, _) = layerx_crypto::disclosure::call::decode_program_call(&call.data)
        .unwrap_or_else(|error| panic!("real ABI5 reward ABI: {error:?}"));
    assert_eq!(native.guest_abi, 5);
    let transport = program_transport(&node);
    let ProgramSubmission::Receipted(reward) = transport
        .receipt(key, hash)
        .unwrap_or_else(|error| panic!("real reward node receipt: {error:?}"))
    else {
        panic!("reward did not execute")
    };
    assert_eq!(reward.terminal_result(), Some(0));
    assert_eq!(reward.terminal_kind(), Some(1));
    assert_eq!(reward.response(), unhex(field(&produced, "response")));
    let canonical = reward.canonical_receipt().to_vec();
    drop(transport);
    node.restart();
    let transport = program_transport(&node);
    let ProgramSubmission::Receipted(reward) = transport
        .receipt(key, hash)
        .unwrap_or_else(|error| panic!("recovered reward authority: {error:?}"))
    else {
        panic!("reopened reward unresolved")
    };
    assert_eq!(reward.terminal_result(), Some(0));
    assert_eq!(reward.canonical_receipt(), canonical);
    let served = node
        .rpc()
        .sequencer_receipt(hash)
        .unwrap_or_else(|error| panic!("real recovered reward receipt: {error:?}"));
    for field in ["receipt", "returnData", "logs", "txHash"] {
        let mut altered = served.clone();
        altered[field] = if field == "logs" {
            json!([])
        } else {
            json!("0x00")
        };
        assert!(
            layerx_sdk::programs::verify_node_program_execution(
                &altered,
                &raw,
                &pinned_policy(&node)
            )
            .is_err(),
            "must bind reward {field}"
        );
    }
    let restricted = layerx_types::program_call::NativeProgramCall {
        response_capacity: 111,
        ..native.native()
    };
    let oversized_call = Eip1559Call {
        data: layerx_crypto::disclosure::call::encode_program_call(&restricted, 200_000),
        ..call.clone()
    };
    let oversized_raw = oversized_call
        .sign(owner.evm_secret().unwrap_or_else(|| panic!("owner key")))
        .unwrap_or_else(|error| panic!("bounded call signer: {error:?}"));
    let oversized = fixture_receipt(&oversized_raw, 7);
    assert!(
        layerx_sdk::programs::verify_node_program_execution(
            &oversized,
            &oversized_raw,
            &pinned_policy(&node)
        )
        .is_err(),
        "a valid authority signature cannot override response capacity"
    );
    let foreign_call = Eip1559Call {
        chain_id: 126,
        ..call
    };
    let foreign_raw = foreign_call
        .sign(owner.evm_secret().unwrap_or_else(|| panic!("owner key")))
        .unwrap_or_else(|error| panic!("foreign call signer: {error:?}"));
    assert!(
        layerx_sdk::programs::verify_node_program_execution(
            &fixture_receipt(&foreign_raw, 8),
            &foreign_raw,
            &pinned_policy(&node)
        )
        .is_err(),
        "ABI5 does not relax the pinned chain"
    );
}

pub(super) fn daemon_not_ready_lookup_crosses_authenticated_transport() {
    use layerx_agent_api::error::RequestId;
    use layerx_sdk::agent_envelope::{
        AgentEnvelopeTransport, EnvelopeCredential, EnvelopeOutcome, NotReadyStatus,
    };
    let case = owner_case();
    let ca = PathBuf::from(field(&case, "ca_pem"));
    let transport = AgentEnvelopeTransport::connect(field(&case, "endpoint"), None, Some(&ca))
        .unwrap_or_else(|error| panic!("real gateway TLS: {error:?}"));
    let credential = EnvelopeCredential::new(
        field(&case, "tenant"),
        fixed_field(&case, "session_id"),
        fixed_field(&case, "token_id"),
        field(&case, "generation")
            .parse()
            .unwrap_or_else(|error| panic!("generation: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("real admitted session: {error:?}"));
    for (index, name) in ["pending_lookup", "absent_lookup"].iter().enumerate() {
        let response = transport
            .send_operation_status(
                layerx_sdk::Operation::Wait,
                RequestId(2280 + index as u64),
                &case[*name],
                Some(&credential),
                None,
            )
            .unwrap_or_else(|error| panic!("real daemon receipt lookup: {error:?}"));
        let EnvelopeOutcome::NotReady { value, status, .. } = response else {
            panic!("lookup was promoted")
        };
        assert_eq!(value["found"], false);
        match (index, status) {
            (
                0,
                NotReadyStatus::Pending {
                    requested,
                    achieved,
                },
            ) => assert!(achieved < requested),
            (1, NotReadyStatus::Absent { requested }) => {
                assert_eq!(requested, layerx_types::rung::Rung::Receipted);
            }
            _ => panic!("not-ready state changed across transport"),
        }
    }
}

pub(super) fn program_abi_constructor_retains_c_producer_transaction() {
    use layerx_sdk::programs::NativeProgramCallRequest;
    use layerx_types::program_call::NativeProgramCall;
    let fixture: Value = serde_json::from_slice(
        &fs::read(
            repository().join("sdk/platform/conformance/fixtures/native-program-call-v4.json"),
        )
        .unwrap_or_else(|error| panic!("actual C producer fixture: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("fixture JSON: {error:?}"));
    let payload = unhex(field(&fixture, "payload_hex"));
    let native = NativeProgramCall::decode(&payload)
        .unwrap_or_else(|error| panic!("real native payload codec: {error:?}"));
    let data = unhex(field(&fixture["evm"], "calldata"));
    let (decoded, fee) = layerx_crypto::disclosure::call::decode_program_call(&data)
        .unwrap_or_else(|error| panic!("node ABI codec: {error:?}"));
    assert_eq!(decoded.program_id, native.program_id);
    assert_eq!(decoded.resources, native.resources);
    assert_eq!(
        layerx_crypto::disclosure::call::encode_program_call(&native, fee),
        data
    );
    let transaction = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 2_000_000,
        to: fixed_field(&fixture["evm"], "to"),
        data,
    };
    let raw = transaction
        .sign(&[0x42; 32])
        .unwrap_or_else(|error| panic!("external EVM signing producer: {error:?}"));
    let registry = layerx_sdk::programs::programs_module_registry()
        .unwrap_or_else(|error| panic!("real module registrations: {error:?}"));
    let public = NativeProgramCallRequest::new(&registry, native, fee, &raw)
        .unwrap_or_else(|error| panic!("public ABI constructor: {error:?}"));
    let consumer = NativeProgramCallRequest::from_signed_transaction(native, fee, &raw)
        .unwrap_or_else(|error| panic!("transport validation constructor: {error:?}"));
    assert_eq!(public, consumer);
    assert_eq!(public.signed_activity(), raw);
    assert_eq!(public.bound_activity_id(), transaction_hash(&raw));
    let expected: [u8; 32] = Sha256::new()
        .chain_update(b"layerxprograms/idempotency/v1")
        .chain_update(transaction_hash(&raw))
        .chain_update(0_u32.to_be_bytes())
        .chain_update(&transaction.data[..4])
        .finalize()
        .into();
    assert_eq!(public.bound_idempotency_key(), expected);
    assert_ne!(expected, transaction_hash(&raw));
    assert!(NativeProgramCallRequest::new(&registry, native, fee + 1, &raw).is_err());
    let mut retired = transaction.clone();
    retired.data.clone_from(&payload);
    assert!(NativeProgramCallRequest::new(
        &registry,
        native,
        fee,
        &retired
            .sign(&[0x42; 32])
            .unwrap_or_else(|error| panic!("signed retired payload: {error:?}"))
    )
    .is_err());
    let (_, signature) =
        decode_signed(&raw).unwrap_or_else(|error| panic!("real recoverable signature: {error:?}"));
    let carrier = layerx_agent_api::submit::SignatureBytes::new(signature.as_bytes().to_vec())
        .unwrap_or_else(|error| panic!("65-byte carrier: {error:?}"));
    assert_eq!(carrier.as_bytes().len(), 65);
    assert!(
        layerx_agent_api::submit::SignatureBytes::new(carrier.as_bytes()[..64].to_vec()).is_err()
    );
    assert_eq!(
        layerx_crypto::evm_transaction::assemble(&transaction, &signature),
        raw
    );
}

#[path = "owner_result.rs"]
pub(super) mod owner_result;

// Compile the production binary action unchanged into this test target so the
// focused gate needs neither a child process nor ambient clock credentials.
mod binary {
    include!("../../src/bin/layerx-wallet-rpc.rs");

    pub(super) fn invoke(input: &Value) -> Result<Value, ()> {
        let encoded = serde_json::to_vec(input).map_err(|_| ())?;
        let decoded = serde_json::from_slice(&encoded).map_err(|_| ())?;
        execute(&decoded).map_err(|_| ())
    }
}

pub(super) fn authenticated_registration_and_session_reach_exact_owner_results() {
    use layerx_agent_api::error::RequestId;
    use layerx_agent_api::idempotency::Key;
    use layerx_sdk::agent_envelope::{AgentEnvelopeTransport, EnvelopeCredential, EnvelopeError};
    use layerx_sdk::Operation;

    let BootstrapRequests {
        registration,
        session,
        registered,
        opened,
    } = bootstrap_requests();
    let tenant = "sdk-fixture";
    let requests = [registration.clone(), session.clone()];
    let values = [registered.clone(), opened.clone()];
    let seen = Arc::new(AtomicUsize::new(0));
    let counted = seen.clone();
    let server = Server::tls(move |envelope| {
        let index = match field(&envelope, "operation") {
            "agent.register" => 0,
            "session.open" => 1,
            other => panic!("unexpected owner operation {other}"),
        };
        assert_eq!(envelope["version"], 1);
        assert_eq!(
            envelope["credential"],
            json!({"tenant":"sdk-fixture", "session_id":hex(&[0x51;32]),"token_id":hex(&[0x52;32]),"generation":"1"})
        );
        assert_eq!(envelope["request"], requests[index]);
        assert_eq!(
            envelope["idempotency_key"],
            hex(&[0x61
                + u8::try_from(index)
                    .unwrap_or_else(|error| panic!("fixture integer must fit u8: {error:?}"));
                32])
        );
        counted.fetch_add(1, Ordering::SeqCst);
        json!({"request_id":envelope["request_id"],"value":values[index],
            "verification_status":{"state":"achieved","rung":"submitted"}})
    });
    let ca = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("tests/fixtures/repair_22_31_227_fixture/ca.pem");
    let transport = AgentEnvelopeTransport::connect(&server.endpoint, None, Some(&ca))
        .unwrap_or_else(|error| panic!("real authenticated envelope transport: {error:?}"));
    let credential = EnvelopeCredential::new(tenant, [0x51; 32], [0x52; 32], 1)
        .unwrap_or_else(|error| panic!("provisioned session: {error:?}"));
    for (index, (operation, request, expected_value)) in [
        (Operation::AgentRegister, registration, registered),
        (Operation::SessionOpen, session, opened),
    ]
    .into_iter()
    .enumerate()
    {
        let id = RequestId(15_660 + index as u64);
        let key = Key::new(
            [0x61 + u8::try_from(index)
                .unwrap_or_else(|error| panic!("fixture integer must fit u8: {error:?}"));
                32],
        )
        .unwrap_or_else(|error| panic!("mutation key: {error:?}"));
        assert_eq!(
            transport.send_operation(operation, id, &request, None, Some(key)),
            Err(EnvelopeError::CredentialPresence { operation })
        );
        let outcome =
            transport.send_operation(operation, id, &request, Some(&credential), Some(key));
        let expected = json!({"result":"success","value":expected_value,
            "verification_status":{"state":"achieved","rung":"submitted"}});
        owner_result::assert_exact_owner_result(&outcome, id, &expected)
            .unwrap_or_else(|error| panic!("exact provisioned owner result: {error:?}"));
        let mut wrong = expected.clone();
        wrong["value"]["value"]["nonce"] = json!("99");
        assert!(owner_result::assert_exact_owner_result(&outcome, id, &wrong).is_err());
        assert!(
            owner_result::assert_exact_owner_result(&outcome, RequestId(id.0 + 1), &expected)
                .is_err()
        );
    }
    assert_eq!(
        seen.load(Ordering::SeqCst),
        2,
        "anonymous calls must stop before transport"
    );
}

pub(super) fn root_rpc_request_and_method_response() {
    let expected_address = derive_from_seed(&[0x41; 32], 0)
        .unwrap_or_else(|error| panic!("owner: {error:?}"))
        .evm_address_text();
    let address = expected_address.clone();
    let count = Arc::new(AtomicUsize::new(0));
    let counted = count.clone();
    let server = Server::open(move |request| {
        assert_eq!(request["jsonrpc"], "2.0");
        assert_eq!(request["method"], "eth_getBalance");
        assert_eq!(request["params"], json!([expected_address, "pending"]));
        counted.fetch_add(1, Ordering::SeqCst);
        json!({"jsonrpc":"2.0","id":request["id"],"result":"0x2a"})
    });
    let rpc = RpcClient::connect_node(&server.endpoint, None)
        .unwrap_or_else(|error| panic!("explicit live node transport: {error:?}"));
    assert_eq!(
        rpc.get_balance(&address)
            .unwrap_or_else(|error| panic!("actual eth_getBalance response: {error:?}")),
        json!("0x2a")
    );
    assert_eq!(count.load(Ordering::SeqCst), 1);
}

fn bridge_configuration(node: &LocalNode) -> Value {
    let policy = pinned_policy(node);
    json!({"endpoint":node.endpoint,"rpc_route":"node","protocol_version":"2","network_id":"125",
        "native_asset":hex(&[3;32]),"sequencer_id":hex(&policy.sequencer.sequencer_id()),
        "sequencer_key":hex(&policy.sequencer.public_key()),"first_batch":"1","last_batch":u64::MAX.to_string()})
}

pub(super) fn bridge_balances_and_summary_receipt_survive_reopen() {
    let mut node = LocalNode::start();
    let owner = funded_owner(&node);
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = pinned_policy(&node);
    let rpc = node.rpc();
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let options = PaymentOptions {
        actor: owner.did(),
        idempotency_key: [0x64; 32],
        fee_limit: 1_000_000,
        not_before: 0,
        not_after: u64::MAX,
        commitment: Commitment::Receipted,
        wait_timeout: Duration::from_secs(2),
    };
    let token = [4; 32];
    let account = layerx_sdk::rpc::wallet_account(&owner.did(), token, [3; 32])
        .unwrap_or_else(|error| panic!("token account: {error:?}"));
    let minted = run(wallet.mint(token, account, 73, &options))
        .unwrap_or_else(|error| panic!("real SDK mint and verified node summary: {error:?}"));
    let hash = minted
        .receipt()
        .protocol()
        .unwrap_or_else(|| panic!("protocol receipt"))
        .activity_id();
    let canonical = minted.canonical_bytes().to_vec();
    assert_eq!(
        minted
            .receipt()
            .protocol()
            .unwrap_or_else(|| panic!("protocol"))
            .protocol_version(),
        2
    );
    drop(rpc);
    node.restart();
    let config = bridge_configuration(&node);
    for (asset, expected) in [([3; 32], 101_u128), (token, 73)] {
        let request = json!({"action":"balance","configuration":config,"did":owner.did(),"asset":hex(&asset)});
        let actual = binary::invoke(&request).unwrap_or_else(|error| {
            panic!("production binary balance action over root HTTP: {error:?}")
        });
        assert_eq!(
            actual,
            json!({"account_id":hex(&layerx_sdk::rpc::wallet_account(&owner.did(),asset,[3;32]).unwrap_or_else(|error| panic!("account: {error:?}"))),"asset_id":hex(&asset),"balance":expected.to_string()})
        );
    }
    let wait = json!({"action":"wait","configuration":config,"txHash":format!("0x{}",hex(&hash)),"rung":"receipted","timeout_ms":"1000"});
    let receipt = binary::invoke(&wait).unwrap_or_else(|error| {
        panic!("production binary verifies reopened signed summary: {error:?}")
    });
    assert_eq!(
        receipt,
        json!({"txHash":format!("0x{}",hex(&hash)),"rung":"receipted","receipt":hex(&canonical),"result_code":0,"state":"executed"})
    );
    let mut wrong = wait.clone();
    wrong["configuration"]["protocol_version"] = json!("3");
    assert!(
        binary::invoke(&wrong).is_err(),
        "protocol mismatch must remain a refusal"
    );
    wrong = wait.clone();
    wrong["configuration"]["sequencer_key"] = json!(hex(SigningKey::from_bytes(&[0x62; 32])
        .verifying_key()
        .as_bytes()));
    assert!(
        binary::invoke(&wrong).is_err(),
        "signature mismatch must remain a refusal"
    );
    wrong = wait.clone();
    wrong["configuration"]["first_batch"] = json!("1001");
    assert!(
        binary::invoke(&wrong).is_err(),
        "receipt outside its key range must refuse"
    );
    let mut invalid_asset =
        json!({"action":"balance","configuration":config,"did":owner.did(),"asset":"04"});
    assert!(binary::invoke(&invalid_asset).is_err());
    invalid_asset["asset"] = json!(hex(&token));
    invalid_asset["did"] = json!("bad::did");
    assert!(binary::invoke(&invalid_asset).is_err());
    let stored = node
        .state
        .lock()
        .unwrap_or_else(|error| panic!("fixture store: {error:?}"));
    assert_eq!(stored["nonce"], 1);
    assert_eq!(stored["token"], 73);
    drop(stored);
    node.restart();
    let mut recovered_wait = wait;
    recovered_wait["configuration"] = bridge_configuration(&node);
    assert_eq!(binary::invoke(&recovered_wait), Ok(receipt));
}

pub(super) fn root_wallet_request_and_method_response() {
    let node = LocalNode::start();
    let owner = funded_owner(&node);
    let rpc = node.rpc();
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = pinned_policy(&node);
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    assert_eq!(
        wallet
            .balance(&owner.did(), [4; 32])
            .unwrap_or_else(|error| panic!("actual token balance: {error:?}")),
        json!("0x0")
    );
    let state = node
        .state
        .lock()
        .unwrap_or_else(|error| panic!("node request journal: {error:?}"));
    let requests = state["_requests"]
        .as_array()
        .unwrap_or_else(|| panic!("requests"));
    assert_eq!(
        requests.len(),
        3,
        "one funding read, account mapping and token balance"
    );
    assert_eq!(requests[0]["method"], "eth_getBalance");
    let mut mapping =
        layerx_crypto::disclosure::call::ledger_selector("accountId(bytes32,bytes32)").to_vec();
    mapping.extend_from_slice(&owner.layerx_public_key());
    mapping.extend_from_slice(&[4; 32]);
    let mut balance =
        layerx_crypto::disclosure::call::ledger_selector("balanceOf(bytes32,bytes32)").to_vec();
    balance.extend_from_slice(
        &layerx_sdk::rpc::wallet_account(&owner.did(), [4; 32], [3; 32])
            .unwrap_or_else(|error| panic!("account: {error:?}")),
    );
    balance.extend_from_slice(&[4; 32]);
    for (request, data) in requests[1..].iter().zip([mapping, balance]) {
        assert_eq!(request["method"], "eth_call");
        assert_eq!(
            request["params"],
            json!([{"to":"0x000000000000000000000000000000000000101a","data":format!("0x{}",hex(&data))},"pending"])
        );
    }
}

pub(super) fn root_binary_request_and_method_response() {
    let count = Arc::new(AtomicUsize::new(0));
    let counted = count.clone();
    let did = derive_from_seed(&[0x41; 32], 0)
        .unwrap_or_else(|error| panic!("owner: {error:?}"))
        .did();
    let account = layerx_sdk::rpc::wallet_account(&did, [4; 32], [3; 32])
        .unwrap_or_else(|error| panic!("actual account codec: {error:?}"));
    let server = Server::open(move |request| {
        let result = match field(&request, "method") {
            "eth_blockNumber" => {
                assert_eq!(request["params"], json!([]));
                json!("0x2a")
            }
            "eth_call" => {
                let mut data =
                    layerx_crypto::disclosure::call::ledger_selector("balanceOf(bytes32,bytes32)")
                        .to_vec();
                data.extend_from_slice(&account);
                data.extend_from_slice(&[4; 32]);
                assert_eq!(
                    request["params"],
                    json!([{"to":"0x000000000000000000000000000000000000101a","data":format!("0x{}",hex(&data))},"0x2a"])
                );
                json!(format!("0x{}", hex(&abi_word(73))))
            }
            other => panic!("unexpected live method {other}"),
        };
        counted.fetch_add(1, Ordering::SeqCst);
        json!({"jsonrpc":"2.0","id":request["id"],"result":result})
    });
    let config = json!({"endpoint":server.endpoint,"rpc_route":"node","protocol_version":"2","network_id":"125","native_asset":hex(&[3;32]),"sequencer_id":hex(&[1;32]),"sequencer_key":hex(&[2;32]),"first_batch":"1","last_batch":"100"});
    let input = json!({"action":"balance","configuration":config,"did":did,"asset":hex(&[4;32])});
    assert_eq!(
        binary::invoke(&input),
        Ok(json!({"account_id":hex(&account),"asset_id":hex(&[4;32]),"balance":"73"}))
    );
    assert_eq!(count.load(Ordering::SeqCst), 2);
}

// This is an adversarial HTTP peer, not a replacement for any SDK component.
fn raw_peer(response: Vec<u8>, delay: Duration) -> (String, JoinHandle<()>) {
    let listener = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("framing boundary: {error:?}"));
    listener
        .set_nonblocking(true)
        .unwrap_or_else(|error| panic!("nonblocking boundary: {error:?}"));
    let endpoint = format!(
        "http://{}",
        listener
            .local_addr()
            .unwrap_or_else(|error| panic!("address: {error:?}"))
    );
    let worker = std::thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(3);
        let mut socket = loop {
            match listener.accept() {
                Ok((socket, _)) => break socket,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(Instant::now() < deadline, "missing adversarial request");
                    std::thread::sleep(Duration::from_millis(1));
                }
                Err(error) => panic!("accept: {error}"),
            }
        };
        socket
            .set_read_timeout(Some(Duration::from_secs(2)))
            .unwrap_or_else(|error| panic!("read timeout: {error:?}"));
        socket
            .set_write_timeout(Some(Duration::from_secs(2)))
            .unwrap_or_else(|error| panic!("write timeout: {error:?}"));
        let mut reader = BufReader::new(&mut socket);
        let mut line = String::new();
        reader
            .read_line(&mut line)
            .unwrap_or_else(|error| panic!("request line: {error:?}"));
        assert_eq!(line, "POST / HTTP/1.1\r\n");
        let mut length = None;
        let mut total = line.len();
        loop {
            line.clear();
            assert_ne!(
                reader
                    .read_line(&mut line)
                    .unwrap_or_else(|error| panic!("header: {error:?}")),
                0
            );
            total += line.len();
            assert!(total <= 8192);
            if line == "\r\n" {
                break;
            }
            if let Some((name, value)) = line.split_once(':') {
                if name.eq_ignore_ascii_case("content-length") {
                    length = Some(
                        value
                            .trim()
                            .parse::<usize>()
                            .unwrap_or_else(|error| panic!("length: {error:?}")),
                    );
                }
            }
        }
        let length = length.unwrap_or_else(|| panic!("content length"));
        assert!(length <= 8192);
        let mut body = vec![0; length];
        reader
            .read_exact(&mut body)
            .unwrap_or_else(|error| panic!("request body: {error:?}"));
        let request: Value =
            serde_json::from_slice(&body).unwrap_or_else(|error| panic!("real request: {error:?}"));
        assert_eq!(request["jsonrpc"], "2.0");
        std::thread::sleep(delay);
        // A deadline test deliberately closes its client before this peer replies.
        if delay.is_zero() {
            socket
                .write_all(&response)
                .unwrap_or_else(|error| panic!("malformed response: {error:?}"));
        }
    });
    (endpoint, worker)
}

pub(super) fn root_routes_preserve_endpoint_framing_and_timeout_refusals() {
    for endpoint in [
        "",
        "http://user:pass@127.0.0.1",
        "ftp://127.0.0.1",
        "http://example.invalid",
    ] {
        assert!(
            RpcClient::connect_node(endpoint, None).is_err(),
            "malformed endpoint {endpoint}"
        );
    }
    for body in [
        b"{".as_slice(),
        b"{\"jsonrpc\":\"2.0\",\"id\":\"foreign\",\"result\":\"0x1\"}",
        b"{\"jsonrpc\":\"2.0\",\"id\":1}",
    ] {
        let response=[format!("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",body.len()).as_bytes(),body].concat();
        let (endpoint, peer) = raw_peer(response, Duration::ZERO);
        let rpc = RpcClient::connect_node(&endpoint, None)
            .unwrap_or_else(|error| panic!("root transport: {error:?}"));
        assert!(matches!(
            rpc.get_balance("0x1111111111111111111111111111111111111111"),
            Err(RpcError::InvalidResponse)
        ));
        peer.join()
            .unwrap_or_else(|error| panic!("framing peer: {error:?}"));
    }
    let (endpoint, peer) = raw_peer(
        b"HTTP/1.1 200 OK\r\nContent-Length: 100\r\nConnection: close\r\n\r\n{}".to_vec(),
        Duration::ZERO,
    );
    let rpc = RpcClient::connect_node(&endpoint, None)
        .unwrap_or_else(|error| panic!("root transport: {error:?}"));
    assert!(
        rpc.get_balance("0x1111111111111111111111111111111111111111")
            .is_err(),
        "truncated body must refuse"
    );
    peer.join()
        .unwrap_or_else(|error| panic!("truncated peer: {error:?}"));
    for consumer in ["rpc", "wallet", "binary"] {
        let (endpoint, peer) = raw_peer(vec![], Duration::from_millis(150));
        let rpc = RpcClient::connect_node(&endpoint, None)
            .unwrap_or_else(|error| panic!("root timeout transport: {error:?}"))
            .with_clock(Arc::new(OsClock::new()));
        let policy = ReceiptPolicy {
            protocol_version: 2,
            network_id: 125,
            sequencer: SequencerAuthorization::new([1; 32], [2; 32], 1, 100),
            trusted_checkpoint_context_digest: None,
        };
        let began = Instant::now();
        if consumer == "binary" {
            let config = json!({"endpoint":endpoint,"rpc_route":"node","protocol_version":"2","network_id":"125","native_asset":hex(&[3;32]),"sequencer_id":hex(&[1;32]),"sequencer_key":hex(&[2;32]),"first_batch":"1","last_batch":"100"});
            assert!(binary::invoke(&json!({"action":"wait","configuration":config,"txHash":format!("0x{}",hex(&[9;32])),"rung":"receipted","timeout_ms":"50"})).is_err());
        } else {
            let signer = LocalSigner::new([0x42; 32]);
            let wallet = Wallet {
                rpc: &rpc,
                signer: &signer,
                policy: &policy,
                native_asset: [3; 32],
            };
            let outcome = if consumer == "rpc" {
                rpc.wait_for(
                    [9; 32],
                    Commitment::Receipted,
                    &policy,
                    Duration::from_millis(50),
                )
            } else {
                wallet.wait_for([9; 32], Commitment::Receipted, Duration::from_millis(50))
            };
            assert!(
                matches!(outcome,Err(RpcError::Pending {activity_id}) if activity_id==[9;32]),
                "{consumer} stalled request must remain pending"
            );
        }
        assert!(
            began.elapsed() < Duration::from_secs(1),
            "{consumer} deadline must bound transport"
        );
        peer.join()
            .unwrap_or_else(|error| panic!("timeout peer: {error:?}"));
    }
}

fn apply_node_transaction(state: &mut Value, params: &Value) -> Value {
    let raw = unhex(
        params[0]
            .as_str()
            .unwrap_or_else(|| panic!("raw transaction")),
    );
    let (call, signature) = decode_signed(&raw)
        .unwrap_or_else(|error| panic!("fixture receives canonical signed transaction: {error:?}"));
    let sender = recover_sender(&call, &signature)
        .unwrap_or_else(|error| panic!("fixture signature recovery: {error:?}"));
    assert_eq!(
        layerx_types::intent::EvmAddress::new(sender),
        evm_address(&state["owner"])
    );
    assert_eq!(call.chain_id, 125);
    let hash = format!("0x{}", hex(&transaction_hash(&raw)));
    if let Some(retained) = state["transactions"].get(&hash) {
        assert_eq!(retained, &params[0]);
        return json!(hash);
    }
    assert_eq!(
        call.nonce,
        state["nonce"]
            .as_u64()
            .unwrap_or_else(|| panic!("stored nonce"))
    );
    if call.to == call_target(0x101a) {
        let disclosure = layerx_crypto::disclosure::call::bind_call(&call, sender)
            .unwrap_or_else(|error| panic!("fixture payment codec: {error:?}"));
        if let Some(Payment::Mint { amount, .. }) = disclosure.payment {
            state["token"] = json!(u64::try_from(amount)
                .unwrap_or_else(|error| panic!("fixture integer must fit u64: {error:?}")));
        }
    } else if call.to == call_target(0x101d) && call.data[..4] == [0xbd, 0xe3, 0xff, 0x66] {
        let program = hex(&call.data[4..36]);
        let mut registered = vec![0; 192];
        registered[127] = 5;
        state["registry"][program] = json!(format!("0x{}", hex(&registered)));
    } else {
        assert_eq!(call.to, call_target(0x101d));
    }
    let seq = state["nonce"].as_u64().unwrap_or_else(|| panic!("nonce")) + 1;
    state["nonce"] = json!(seq);
    state["transactions"][&hash] = params[0].clone();
    state["receipts"][&hash] = fixture_receipt(&raw, seq);
    json!(hash)
}

fn program_outcome(call: &Eip1559Call) -> (Vec<u8>, Vec<Value>) {
    let mut returned = Vec::new();
    let mut logs = Vec::<Value>::new();
    if call.to == call_target(0x101d) && call.data[..4] == [0x54, 0xbd, 0x2b, 0x48] {
        let (native, _) = layerx_crypto::disclosure::call::decode_program_call(&call.data)
            .unwrap_or_else(|error| panic!("production Programs ABI decoder: {error:?}"));
        let response = if native.calldata == b"reward-fixture" {
            reward_response()
        } else {
            vec![]
        };
        for value in [0, 1, 96, response.len() as u128] {
            returned.extend_from_slice(&abi_word(value));
        }
        returned.extend_from_slice(&response);
        returned.resize(returned.len().next_multiple_of(32), 0);
        let mut outcome = Vec::new();
        for value in [
            [0x31; 32],
            [0x32; 32],
            abi_word(0),
            abi_word(0),
            abi_word(1),
            abi_word(u128::from(native.guest_abi)),
            abi_word(1),
            abi_word(1),
            abi_word(0),
            [0; 32],
            [0x34; 32],
            [0x35; 32],
            [0x36; 32],
        ] {
            outcome.extend_from_slice(&value);
        }
        let topic = transaction_hash(b"ProgramCallOutcome(bytes32,bytes32,bytes32,uint64,uint8,uint16,uint16,uint32,uint32,int32,bytes32,bytes32,bytes32,bytes32)");
        logs.push(json!({"address":format!("0x{}",hex(&call.to)),"topics":[format!("0x{}",hex(&topic)),format!("0x{}",hex(&native.program_id.bytes()))],"data":format!("0x{}",hex(&outcome)),"logIndex":"0x0"}));
    }
    if call.to == call_target(0x101d) && call.data[..4] == [0xbd, 0xe3, 0xff, 0x66] {
        logs.push(json!({"address":format!("0x{}",hex(&call.to)),"topics":["0x2765c1020c93cab47e389edaa5b5c68834bdd76f7a5d384adb460fc86e688709",format!("0x{}",hex(&call.data[4..36]))],"data":"0x","logIndex":"0x0"}));
    }
    (returned, logs)
}

fn encode_receipt_effects(e: &mut layerx_wire::encode::Encoder, effects: &[Vec<u8>]) {
    e.sequence_length(effects.len(), 512)
        .unwrap_or_else(|error| panic!("effect count: {error:?}"));
    for (ordinal, body) in effects.iter().enumerate() {
        e.u16(12)
            .unwrap_or_else(|error| panic!("summary module: {error:?}"));
        e.u16(
            u16::try_from(ordinal)
                .unwrap_or_else(|error| panic!("fixture integer must fit u16: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("ordinal: {error:?}"));
        e.u16(0)
            .unwrap_or_else(|error| panic!("event type: {error:?}"));
        e.u8(if ordinal == 0 { 1 } else { 3 })
            .unwrap_or_else(|error| panic!("effect kind: {error:?}"));
        e.u8(0)
            .unwrap_or_else(|error| panic!("nonmonetary: {error:?}"));
        e.bytes(&[0; 32], 32)
            .unwrap_or_else(|error| panic!("transfer root: {error:?}"));
        e.bytes(body, 256)
            .unwrap_or_else(|error| panic!("effect body: {error:?}"));
    }
}

fn sign_fixture_receipt(unsigned: Vec<u8>, key: &SigningKey) -> Vec<u8> {
    let decoded = layerx_wire::receipt::decode(&unsigned)
        .unwrap_or_else(|error| panic!("real receipt codec: {error:?}"));
    assert_eq!(
        layerx_wire::receipt::encode_unsigned(&decoded)
            .unwrap_or_else(|error| panic!("canonical unsigned receipt: {error:?}")),
        unsigned
    );
    let digest = layerx_wire::hash::receipt_digest(&unsigned)
        .unwrap_or_else(|error| panic!("domain-separated receipt digest: {error:?}"));
    let signature = key.sign(&digest).to_bytes();
    let mut canonical = unsigned;
    canonical.pop();
    canonical.push(1);
    canonical.extend_from_slice(&64_u32.to_be_bytes());
    canonical.extend_from_slice(&signature);
    let decoded = layerx_wire::receipt::decode(&canonical)
        .unwrap_or_else(|error| panic!("real signed receipt codec: {error:?}"));
    assert_eq!(
        layerx_wire::receipt::encode(&decoded)
            .unwrap_or_else(|error| panic!("canonical signed receipt: {error:?}")),
        canonical
    );
    canonical
}

fn assert_node_receipt_refusals(
    node: &LocalNode,
    served: &Value,
    hash: [u8; 32],
    policy: &ReceiptPolicy,
) {
    use layerx_sdk::rpc_verification::verify_node_receipt;
    for field in [
        "paymentDenom",
        "feeDenom",
        "returnData",
        "from",
        "keyId",
        "receipt",
    ] {
        let mut altered = served.clone();
        altered[field] = if field == "paymentDenom" || field == "feeDenom" {
            json!("foreign")
        } else {
            json!("0x00")
        };
        assert!(
            verify_node_receipt(&altered, hash, policy).is_err(),
            "must bind {field}"
        );
    }
    let wrong = ReceiptPolicy {
        sequencer: SequencerAuthorization::new([0x55; 32], [0x66; 32], 1, u64::MAX),
        ..pinned_policy(node)
    };
    assert!(verify_node_receipt(served, hash, &wrong).is_err());
    let outside = ReceiptPolicy {
        sequencer: SequencerAuthorization::new(
            policy.sequencer.sequencer_id(),
            policy.sequencer.public_key(),
            u64::MAX,
            u64::MAX,
        ),
        ..pinned_policy(node)
    };
    assert!(verify_node_receipt(served, hash, &outside).is_err());
}

fn assert_committed_refusals(
    node: &LocalNode,
    record: &Value,
    hash: [u8; 32],
    policy: &ReceiptPolicy,
) {
    for (section, field_name, replacement) in [
        ("header", "data_hash", json!("00".repeat(32))),
        ("header", "chain_id", json!("foreign-chain")),
        ("commit", "height", json!("11")),
    ] {
        node.state
            .lock()
            .unwrap_or_else(|error| panic!("fixture store: {error:?}"))["_committed"]["commit"]
            ["signed_header"][section][field_name] = replacement;
        assert!(
            node.rpc()
                .wait_for(hash, Commitment::Committed, policy, Duration::from_secs(1))
                .is_err(),
            "must refuse altered committed {section}/{field_name}"
        );
        node.state
            .lock()
            .unwrap_or_else(|error| panic!("fixture store: {error:?}"))["_committed"] =
            record.clone();
    }

    let unpinned = RpcClient::connect_node(&node.endpoint, None)
        .unwrap_or_else(|error| panic!("unpinned node transport: {error:?}"))
        .with_clock(Arc::new(OsClock::new()));
    assert!(matches!(
        unpinned.wait_for(hash, Commitment::Committed, policy, Duration::from_secs(1)),
        Err(RpcError::MissingFinalityTrust)
    ));
    {
        let mut state = node
            .state
            .lock()
            .unwrap_or_else(|error| panic!("fixture store: {error:?}"));
        state["_committed"]["batch"]["proof"]["aunts"][0] = json!(format!("0x{}", hex(&[0; 32])));
    }
    assert!(
        node.rpc()
            .wait_for(hash, Commitment::Committed, policy, Duration::from_secs(1))
            .is_err(),
        "commit signature cannot replace batch inclusion"
    );
    node.state
        .lock()
        .unwrap_or_else(|error| panic!("fixture store: {error:?}"))["_committed"] = record.clone();
    {
        let mut state = node
            .state
            .lock()
            .unwrap_or_else(|error| panic!("fixture store: {error:?}"));
        state["_committed"]["commit"]["signed_header"]["commit"]["signatures"][0]["signature"] =
            json!("A".repeat(88));
    }
    assert!(
        node.rpc()
            .wait_for(hash, Commitment::Committed, policy, Duration::from_secs(1))
            .is_err(),
        "the committed label cannot replace quorum signatures"
    );
}

struct OwnerCall {
    call: Eip1559Call,
    signature: layerx_crypto::signer::AgentSignature,
    sender: [u8; 20],
    signed_raw: Vec<u8>,
    digest: [u8; 32],
}
fn owner_case_server(
    document: Value,
    preparation: Value,
    disclosed: Value,
    signed: OwnerCall,
) -> Server {
    let OwnerCall {
        call,
        signature,
        sender,
        signed_raw,
        digest,
    } = signed;
    let fixture = document;
    let pending: Value = serde_json::from_str(include_str!(
        "../../../../../../services/agent/schema/agent-api/golden/agent-http-success-pending.json"
    ))
    .unwrap_or_else(|error| panic!("daemon pending golden: {error:?}"));
    let absent: Value = serde_json::from_str(include_str!(
        "../../../../../../services/agent/schema/agent-api/golden/agent-http-success-absent.json"
    ))
    .unwrap_or_else(|error| panic!("daemon absent golden: {error:?}"));
    Server::tls(move |envelope| {
        assert_eq!(envelope["credential"]["tenant"], fixture["tenant"]);
        for name in ["session_id", "token_id", "generation"] {
            assert_eq!(envelope["credential"][name], fixture[name]);
        }
        let request = &envelope["request"];
        let value = match field(&envelope, "operation") {
            "prepare" if request["variant"] == "native_v1" => {
                for name in [
                    "actor",
                    "authority",
                    "account_sequence",
                    "not_before",
                    "not_after",
                    "idempotency_key",
                    "fee_limit",
                    "payload",
                    "payload_hash",
                    "capability_id",
                ] {
                    assert_eq!(request[name], fixture["request"][name]);
                }
                preparation.clone()
            }
            "prepare" => {
                assert_eq!(
                    request["canonical_bytes"],
                    fixture["owner_preview_canonical_bytes"]
                );
                disclosed.clone()
            }
            "sign" | "submit" => {
                use layerx_crypto::evm_transaction::assemble;
                use layerx_crypto::signer::AgentSignature;
                let sig = layerx_agent_api::submit::SignatureBytes::new(unhex(field(
                    request,
                    "signature",
                )))
                .unwrap_or_else(|error| panic!("production 65-byte API carrier: {error:?}"));
                assert_eq!(sig.as_bytes(), signature.as_bytes());
                let sig = AgentSignature::new(
                    sig.as_bytes()
                        .try_into()
                        .unwrap_or_else(|error| panic!("signature width: {error:?}")),
                )
                .unwrap_or_else(|error| panic!("signature codec: {error:?}"));
                assert_eq!(
                    recover_sender(&call, &sig)
                        .unwrap_or_else(|error| panic!("external signer recovery: {error:?}")),
                    sender
                );
                let raw = assemble(&call, &sig);
                assert_eq!(raw, signed_raw);
                assert_eq!(request["preparation_ref"], hex(&digest));
                json!({"activity_id":hex(&transaction_hash(&raw)),"submission":{"submission_ref":hex(&digest),"state":"Signed","rung":"submitted","evidence":[],"transitions":[]},"receipt":null})
            }
            "wait" => {
                let mut response = if request == &fixture["pending_lookup"] {
                    pending.clone()
                } else {
                    assert_eq!(request, &fixture["absent_lookup"]);
                    absent.clone()
                };
                response["request_id"] = envelope["request_id"].clone();
                return response;
            }
            other => panic!("unexpected owner fixture operation {other}"),
        };
        json!({"request_id":envelope["request_id"],"value":value,"verification_status":{"state":"achieved","rung":"submitted"}})
    })
}

fn native_prepare_request(case: &Value) -> layerx_agent_api::identity::NativePrepareRequestV1 {
    use layerx_agent_api::identity::{
        AgentDid, CapabilityId, NativeActivity, NativePreparationPurposeV1, NativePrepareRequestV1,
        SignedNativePreparationPurposeV1,
    };
    let input = &case["request"];
    let purpose = NativePreparationPurposeV1::from_canonical_bytes(&unhex(field(
        case,
        "owner_preview_purpose",
    )))
    .unwrap_or_else(|error| panic!("actual owner canonical preview purpose: {error:?}"));
    let owner = fixed_field::<32>(case, "owner_public_key");
    NativePrepareRequestV1 {
        activity: NativeActivity::from(
            layerx_types::payload::ActivityType::new(layerx_types::payload::ModuleId::Programs, 3)
                .unwrap_or_else(|error| panic!("Programs call: {error:?}")),
        ),
        actor: AgentDid::new(field(input, "actor"))
            .unwrap_or_else(|error| panic!("actor: {error:?}")),
        authority: field(input, "authority").to_owned(),
        account_sequence: field(input, "account_sequence")
            .parse()
            .unwrap_or_else(|error| panic!("nonce: {error:?}")),
        not_before: field(input, "not_before")
            .parse()
            .unwrap_or_else(|error| panic!("not before: {error:?}")),
        not_after: field(input, "not_after")
            .parse()
            .unwrap_or_else(|error| panic!("not after: {error:?}")),
        idempotency_key: fixed_field(input, "idempotency_key"),
        fee_limit: field(input, "fee_limit")
            .parse()
            .unwrap_or_else(|error| panic!("fee: {error:?}")),
        payload: unhex(field(input, "payload")),
        payload_hash: fixed_field(input, "payload_hash"),
        capability_id: CapabilityId::new(field(input, "capability_id"))
            .unwrap_or_else(|error| panic!("capability: {error:?}")),
        purpose: SignedNativePreparationPurposeV1 {
            purpose,
            owner_public_key: owner,
            signature: fixed_field(case, "owner_purpose_signature"),
        },
        local_grant: None,
    }
}

fn submit_reward_call(
    node: &LocalNode,
    owner: &layerx_crypto::account_derivation::DerivedAccount,
) -> Value {
    use layerx_sdk::programs::{
        NativeProgramCallRequest, NativeProgramTransport, ProgramSubmission,
    };
    use layerx_types::program_call::{NativeProgramCall, Resources};
    let grants = layerx_sdk::native_capabilities::NativeCapabilitySet::new(vec![])
        .unwrap_or_else(|error| panic!("reward capabilities: {error:?}"))
        .encode()
        .unwrap_or_else(|error| panic!("capability codec: {error:?}"));
    let native = NativeProgramCall {
        program_id: layerx_types::intent::ProgramId::new([0x71; 32]),
        guest_abi: 5,
        entrypoint: b"layerx_call",
        calldata: b"reward-fixture",
        capabilities: &grants,
        access_declaration: b"",
        response_capacity: 112,
        resources: Resources([1_000_000; 7]),
    };
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 0,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: call_target(0x101d),
        data: layerx_crypto::disclosure::call::encode_program_call(&native, 200_000),
    };
    let raw = call
        .sign(owner.evm_secret().unwrap_or_else(|| panic!("owner key")))
        .unwrap_or_else(|error| panic!("reward signer: {error:?}"));
    let registry = layerx_sdk::programs::programs_module_registry()
        .unwrap_or_else(|error| panic!("registry: {error:?}"));
    let request = NativeProgramCallRequest::new(&registry, native, 200_000, &raw)
        .unwrap_or_else(|error| panic!("reward ABI5 constructor: {error:?}"));
    let key = request.bound_idempotency_key();
    let submission = program_transport(node)
        .submit_native(&request, key)
        .unwrap_or_else(|error| panic!("reward transaction transport: {error:?}"));
    assert!(matches!(submission, ProgramSubmission::Submitted { .. }));
    json!({"signed_transaction":hex(&raw),"idempotency_key":hex(&key),"response":hex(&reward_response())})
}

fn deploy_abi5_program(
    node: &LocalNode,
    rpc: &RpcClient,
    owner: &layerx_crypto::account_derivation::DerivedAccount,
) -> Eip1559Call {
    use layerx_sdk::programs::{
        NativeProgramLifecycleRequest, ProgramLifecycleSubmission, ProgramTransport,
    };
    use layerx_types::intent::ProgramId;
    use layerx_types::program_lifecycle::{NativeProgramDeploy, ProgramUpgradePolicy};
    let registry = layerx_sdk::programs::programs_module_registry()
        .unwrap_or_else(|error| panic!("real Programs registrations: {error:?}"));
    let fixture: Value = serde_json::from_slice(
        &fs::read(
            repository().join("sdk/platform/conformance/fixtures/native-program-deploy-v4.json"),
        )
        .unwrap_or_else(|error| panic!("C native producer fixture: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("native deploy JSON: {error:?}"));
    let payload = unhex(
        fixture["payload_hex"]
            .as_str()
            .unwrap_or_else(|| panic!("native deploy payload")),
    );
    let original = NativeProgramDeploy::decode(&payload)
        .unwrap_or_else(|error| panic!("real lifecycle codec: {error:?}"));
    let deploy = NativeProgramDeploy {
        program_id: ProgramId::new([0x71; 32]),
        guest_abi: 5,
        policy: ProgramUpgradePolicy::Immutable,
        interface: None,
        ..original
    };
    let payload = deploy
        .encode()
        .unwrap_or_else(|error| panic!("ABI5 deploy producer: {error:?}"));
    let (chain_id, tip, maximum) = rpc
        .eip1559_parameters()
        .unwrap_or_else(|error| panic!("live fees: {error:?}"));
    let mut to = [0; 20];
    to[18..].copy_from_slice(&0x101d_u16.to_be_bytes());
    let transaction = Eip1559Call {
        chain_id,
        nonce: rpc
            .get_identity_sequence(&owner.evm_address_text())
            .unwrap_or_else(|error| panic!("pending nonce: {error:?}")),
        max_priority_fee_per_gas: tip,
        max_fee_per_gas: maximum,
        gas_limit: 10_000_000,
        to,
        data: layerx_sdk::program_lifecycle::lifecycle_calldata(1, &payload)
            .unwrap_or_else(|error| panic!("actual deployment ABI: {error:?}")),
    };
    let raw = transaction
        .sign(
            owner
                .evm_secret()
                .unwrap_or_else(|| panic!("external owner key")),
        )
        .unwrap_or_else(|error| panic!("external deploy signing: {error:?}"));
    let request = NativeProgramLifecycleRequest::deploy(&registry, deploy, &raw)
        .unwrap_or_else(|error| panic!("public deploy constructor: {error:?}"));
    let transport = program_transport(node);
    let submitted = transport
        .deploy(&request, request.bound_idempotency_key())
        .unwrap_or_else(|error| panic!("live deploy write: {error:?}"));
    let ProgramLifecycleSubmission::Submitted {
        activity_id,
        retained_signed_activity,
        ..
    } = submitted
    else {
        panic!("deploy outcome was not submitted")
    };
    assert_eq!(retained_signed_activity, raw);
    assert_eq!(activity_id, transaction_hash(&raw));
    mined(rpc, &node.endpoint, &format!("0x{}", hex(&activity_id)));
    let receipt = transport
        .lifecycle_receipt(&request)
        .unwrap_or_else(|error| panic!("signed node lifecycle receipt: {error:?}"));
    assert_eq!(
        receipt
            .protocol()
            .unwrap_or_else(|| panic!("protocol"))
            .result_code(),
        0
    );
    transaction
}

fn assert_program_constructor_equivalence(
    registry: &layerx_types::payload::ModuleRegistry,
    native: layerx_types::program_call::NativeProgramCall<'_>,
    fee: u128,
    signed: &[u8],
    new: &layerx_sdk::programs::NativeProgramCallRequest,
) -> layerx_sdk::programs::ProgramCallRequest {
    use layerx_sdk::programs::ProgramCallRequest;
    use layerx_types::intent::{CallBudget, Calldata, ProgramCall, RequestedCapabilities};
    let program_id = native.program_id;
    let generic = ProgramCall::new(
        program_id,
        Calldata::new(b"").unwrap_or_else(|error| panic!("typed calldata: {error:?}")),
        CallBudget::new(
            native.resources.0[0],
            layerx_types::amount::Amount::from_u128(fee),
        )
        .unwrap_or_else(|error| panic!("typed fee semantics: {error:?}")),
        RequestedCapabilities::none(),
    );
    let public = ProgramCallRequest::new(registry, generic, signed).unwrap_or_else(|error| {
        panic!("public generic constructor agrees with actual ABI: {error:?}")
    });
    assert_eq!(public.bound_activity_id(), new.bound_activity_id());
    assert_eq!(public.bound_idempotency_key(), new.bound_idempotency_key());
    assert_ne!(public.bound_idempotency_key(), transaction_hash(signed));
    public
}

fn assert_native_disclosure(
    transport: &layerx_sdk::agent_envelope::AgentEnvelopeTransport,
    credential: &layerx_sdk::agent_envelope::EnvelopeCredential,
    registry: &layerx_types::payload::ModuleRegistry,
    prepared: &layerx_agent_api::identity::NativePrepareResultV1,
    sender: [u8; 20],
) {
    use layerx_agent_api::error::RequestId;
    use layerx_agent_api::idempotency::Key;
    use layerx_agent_api::identity::NativeDisclosureRequestV1;
    use layerx_sdk::agent_envelope::validate_call_disclosure;
    let disclosure_request = NativeDisclosureRequestV1 {
        canonical_bytes: prepared.canonical_bytes.clone(),
    };
    let disclosed = transport
        .disclose_native_preparation(
            RequestId(2272),
            Key::new([0x72; 32]).unwrap_or_else(|error| panic!("disclosure key: {error:?}")),
            &disclosure_request,
            credential,
            registry,
        )
        .unwrap_or_else(|error| panic!("actual daemon disclosure: {error:?}"))
        .value;
    let disclosure = validate_call_disclosure(&disclosure_request, &disclosed, sender)
        .unwrap_or_else(|error| panic!("migrated bind_call adapter: {error:?}"));
    assert_eq!(
        disclosure
            .audit_digest()
            .unwrap_or_else(|error| panic!("actual call audit: {error:?}")),
        disclosed.disclosure_digest
    );
    for coordinate in ["actor", "authority", "fee", "purpose", "digest"] {
        let mut altered = disclosed.clone();
        match coordinate {
            "actor" => altered.actor.push(0),
            "authority" => altered.authority.push(0),
            "fee" => altered.fee_limit += 1,
            "purpose" => altered.idempotency_key = [0x55; 32],
            "digest" => altered.disclosure_digest = [0x55; 32],
            _ => unreachable!(),
        }
        assert!(
            validate_call_disclosure(&disclosure_request, &altered, sender).is_err(),
            "must bind disclosure {coordinate}"
        );
    }
}

struct BootstrapRequests {
    registration: Value,
    session: Value,
    registered: Value,
    opened: Value,
}
fn bootstrap_requests() -> BootstrapRequests {
    use layerx_client::account_binding::{plan_bind, BindPlan};
    use layerx_crypto::session::{issue_session_key, SessionKeyRequest, SessionPurpose};
    let account = derive_from_seed(&[0x41; 32], 0)
        .unwrap_or_else(|error| panic!("provisioned owner: {error:?}"));
    let BindPlan::Bind(binding) = plan_bind(&account, 125, None, 0)
        .unwrap_or_else(|error| panic!("consent producer: {error:?}"))
    else {
        panic!("fresh owner must need binding");
    };
    binding
        .signed_binding()
        .verify()
        .unwrap_or_else(|error| panic!("real owner consent: {error:?}"));
    let did = layerx_types::ids::Did::new(account.did().as_bytes())
        .unwrap_or_else(|error| panic!("owner DID: {error:?}"));
    let grantor = layerx_wire::hash::did_id_for_protocol(&did, 3)
        .unwrap_or_else(|error| panic!("protocol grantor: {error:?}"));
    let mut session_public_key = [0; 32];
    session_public_key[12..].copy_from_slice(
        &derive_from_seed(&[0x42; 32], 0)
            .unwrap_or_else(|error| panic!("session account: {error:?}"))
            .evm_address(),
    );
    let issued = issue_session_key(&SessionKeyRequest {
        grantor,
        session_public_key,
        not_before: 1,
        expires_at: Some(10_000),
        permitted_activity_types: vec![layerx_types::payload::ActivityType::new(
            ModuleId::Asset,
            4,
        )
        .unwrap_or_else(|error| panic!("open account: {error:?}"))],
        revocation_sequence: Some(1),
        fee_budget: None,
        purpose: SessionPurpose::Activity,
    })
    .unwrap_or_else(|error| panic!("real session grant producer: {error:?}"));
    assert_eq!(
        layerx_crypto::session::decode_session_key(&issued.registration_payload)
            .unwrap_or_else(|error| panic!("owner grant consumer: {error:?}")),
        issued
    );

    let tenant = "sdk-fixture";
    let authority = hex(&account.layerx_public_key());
    let context = json!({"tenant":tenant,"agent_did":account.did(),"authority_ref":authority,
        "permitted_activity_types":["65540"],"expiry":"10000","client":"sdk","policy_version":"v1"});
    let registration = json!({"tenant":tenant,"agent_did":account.did(),"authority_ref":authority,
        "client":"sdk","policy_version":"v1","owner_address":hex(&account.evm_address()),
        "binding_consent":hex(&binding.signed_binding().signature()),"fee_limit":"1000000"});
    let session = json!({"context":context,"grant_id":hex(&issued.grant_id),
        "registration_payload":hex(&issued.registration_payload),"expiry_sequence":"10000",
        "purpose":hex(&[0x51;32]),"fee_limit":"1000000"});
    let register_call = Eip1559Call {
        chain_id: 125,
        nonce: 0,
        max_priority_fee_per_gas: 0,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: binding.to(),
        data: binding.data().to_vec(),
    };
    // Committed peer contract: grantSession(bytes,uint64,bytes32), as served by
    // agent_rpc_dispatch::session_open. The grant bytes came from issue_session_key.
    let mut session_data =
        layerx_crypto::disclosure::call::ledger_selector("grantSession(bytes,uint64,bytes32)")
            .to_vec();
    session_data.extend_from_slice(&abi_word(96));
    session_data.extend_from_slice(&abi_word(10_000));
    session_data.extend_from_slice(&[0x51; 32]);
    session_data.extend_from_slice(&abi_word(issued.registration_payload.len() as u128));
    session_data.extend_from_slice(&issued.registration_payload);
    session_data.resize(4 + (session_data.len() - 4).div_ceil(32) * 32, 0);
    let session_call = Eip1559Call {
        nonce: 1,
        to: call_target(0x101c),
        data: session_data,
        ..register_call.clone()
    };
    let response = |call: &Eip1559Call| {
        json!({"authority":{"tenant":tenant,"agent_did":account.did(),"authority_ref":authority,
            "protocol_authority":format!("01{authority}")},
            "value":{"chain_id":"125","nonce":call.nonce.to_string(),"max_priority_fee_per_gas":"0",
            "max_fee_per_gas":"2","gas_limit":"100000","to":hex(&call.to),"value":"0","data":hex(&call.data),
            "signing_hash":hex(&call.signing_hash()),"observed_head_height":"1000"}})
    };
    let registered = response(&register_call);
    let opened = response(&session_call);
    assert_ne!(registered, opened);
    BootstrapRequests {
        registration,
        session,
        registered,
        opened,
    }
}

fn node_call(state: &Value, params: &Value) -> Value {
    let data = unhex(
        params[0]["data"]
            .as_str()
            .unwrap_or_else(|| panic!("call data")),
    );
    let selector = &data[..4];
    let mut word = vec![0; 32];
    if selector == layerx_client::sequencer::GET_EVM_ADDR_BY_LAYERX {
        word[12..].copy_from_slice(&unhex(
            state["owner"].as_str().unwrap_or_else(|| panic!("owner")),
        ));
    } else if selector
        == layerx_crypto::disclosure::call::ledger_selector("accountId(bytes32,bytes32)")
    {
        let owner = derive_from_seed(&[0x41; 32], 0)
            .unwrap_or_else(|error| panic!("fixture owner: {error:?}"));
        assert_eq!(&data[4..36], &owner.layerx_public_key());
        let asset = data[36..68]
            .try_into()
            .unwrap_or_else(|error| panic!("asset width: {error:?}"));
        word = layerx_sdk::rpc::wallet_account(&owner.did(), asset, [3; 32])
            .unwrap_or_else(|error| panic!("node account mapping: {error:?}"))
            .to_vec();
    } else if selector
        == layerx_crypto::disclosure::call::ledger_selector("balanceOf(bytes32,bytes32)")
    {
        let owner = derive_from_seed(&[0x41; 32], 0)
            .unwrap_or_else(|error| panic!("fixture owner: {error:?}"));
        let asset: [u8; 32] = data[36..68]
            .try_into()
            .unwrap_or_else(|error| panic!("asset width: {error:?}"));
        assert_eq!(
            &data[4..36],
            &layerx_sdk::rpc::wallet_account(&owner.did(), asset, [3; 32])
                .unwrap_or_else(|error| panic!("ledger account: {error:?}"))
        );
        let amount = if asset == [3; 32] {
            101
        } else {
            state["token"]
                .as_u64()
                .unwrap_or_else(|| panic!("token balance"))
        };
        word[24..].copy_from_slice(&amount.to_be_bytes());
    } else if selector == layerx_crypto::disclosure::call::ledger_selector("getProgram(bytes32)") {
        return state["registry"][hex(&data[4..36])].clone();
    } else {
        panic!("unexpected fixture eth_call selector: {}", hex(selector));
    }
    json!(format!("0x{}", hex(&word)))
}

fn receipt_effects(returned: &[u8], logs: &[Value]) -> Vec<Vec<u8>> {
    let mut summary = Vec::new();
    for n in [100_000_u64, 100_000, 3] {
        summary.extend_from_slice(&n.to_be_bytes());
    }
    summary.extend_from_slice(&1_u32.to_be_bytes());
    summary.extend_from_slice(&[0; 40]);
    summary.extend_from_slice(&Sha256::digest([]));
    summary.extend_from_slice(
        &(u32::try_from(returned.len())
            .unwrap_or_else(|error| panic!("fixture integer must fit u32: {error:?}")))
        .to_be_bytes(),
    );
    summary.extend_from_slice(
        &(u32::try_from(logs.len())
            .unwrap_or_else(|error| panic!("fixture integer must fit u32: {error:?}")))
        .to_be_bytes(),
    );
    summary.push(4);
    summary.extend_from_slice(b"uhpx");
    let mut effects = vec![summary];
    for log in logs {
        let mut body = unhex(field(log, "address"));
        let topics = log["topics"]
            .as_array()
            .unwrap_or_else(|| panic!("fixture topics"));
        body.push(
            u8::try_from(topics.len())
                .unwrap_or_else(|error| panic!("fixture integer must fit u8: {error:?}")),
        );
        for topic in topics {
            body.extend_from_slice(&unhex(topic.as_str().unwrap_or_else(|| panic!("topic"))));
        }
        body.extend_from_slice(&Sha256::digest(unhex(field(log, "data"))));
        effects.push(body);
    }
    effects
}

fn assert_initial_wallet_balances(
    node: &LocalNode,
    owner: &layerx_crypto::account_derivation::DerivedAccount,
    wallet: &Wallet<'_>,
    asset: [u8; 32],
) -> Value {
    assert_eq!(
        wallet
            .balance(&owner.did(), asset)
            .unwrap_or_else(|error| panic!("public token balance: {error:?}")),
        json!("0x49")
    );
    assert_eq!(
        wallet
            .rpc
            .wallet([3; 32])
            .balance(&owner.did(), asset)
            .unwrap_or_else(|error| panic!("RPC token balance: {error:?}")),
        json!("0x49")
    );
    let native = wallet
        .balance(&owner.did(), [3; 32])
        .unwrap_or_else(|error| panic!("mapped native balance: {error:?}"));
    assert_eq!(
        native,
        json!(format!(
            "0x{:x}",
            quantity_latest(node, "eth_getBalance", &owner.evm_address_text())
        ))
    );
    native
}

fn assert_recovered_program(
    node: &LocalNode,
    key: [u8; 32],
    hash: [u8; 32],
    canonical: &[u8],
    program_id: layerx_types::intent::ProgramId,
    before: &[u8],
) {
    use layerx_sdk::programs::{ProgramSubmission, ProgramTransport};
    let recovered = program_transport(node);
    let ProgramSubmission::Receipted(executed) = recovered
        .receipt(key, hash)
        .unwrap_or_else(|error| panic!("reopened node and fresh client recovery: {error:?}"))
    else {
        panic!("recovered call lost execution")
    };
    assert_eq!(executed.transaction_hash(), hash);
    assert_eq!(executed.idempotency_key(), key);
    assert_eq!(executed.terminal_result(), Some(0));
    assert_eq!(executed.canonical_receipt(), canonical);
    assert_eq!(
        recovered
            .node_read(program_id.bytes(), "getProgram(bytes32)")
            .unwrap_or_else(|error| panic!("reopened registry: {error:?}"))
            .bytes,
        before
    );
    assert!(
        recovered.receipt([0x55; 32], hash).is_err(),
        "foreign idempotency cannot recover this transaction"
    );
}
