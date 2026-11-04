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
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
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
            "/tmp/proof-sdk-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::DirBuilder::new()
            .mode(0o700)
            .create(&root)
            .expect("private proof directory");
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
        .expect("real payment calldata producer"),
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
        .expect("repository root")
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
        .expect("TLS versions")
        .with_no_client_auth()
        .with_single_cert(
            vec![CertificateDer::from(
                include_bytes!("../src/repair_22_31_227_fixture/server.der").to_vec(),
            )],
            PrivateKeyDer::Pkcs8(PrivatePkcs8KeyDer::from(
                include_bytes!("../src/repair_22_31_227_fixture/server-key.der").to_vec(),
            )),
        )
        .expect("committed TLS fixture");
        Self::open_inner(handler, Some(Arc::new(config)))
    }
    fn open_inner(
        handler: impl Fn(Value) -> Value + Send + 'static,
        tls: Option<Arc<rustls::ServerConfig>>,
    ) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback listener");
        listener
            .set_nonblocking(true)
            .expect("nonblocking listener");
        let endpoint = format!(
            "{}://{}",
            if tls.is_some() { "https" } else { "http" },
            listener.local_addr().expect("fixture address")
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
                    .expect("bounded peer");
                let stream: Box<dyn FixtureIo> = match &tls {
                    Some(config) => Box::new(rustls::StreamOwned::new(
                        rustls::ServerConnection::new(config.clone())
                            .expect("fixture TLS connection"),
                        stream,
                    )),
                    None => Box::new(stream),
                };
                let mut reader = BufReader::new(stream);
                let mut line = String::new();
                reader.read_line(&mut line).expect("request line");
                assert!(line.starts_with("POST "), "{line}");
                let mut length = None;
                loop {
                    line.clear();
                    reader.read_line(&mut line).expect("HTTP header");
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, value)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("content-length") {
                            length = Some(value.trim().parse::<usize>().expect("body length"));
                        }
                    }
                }
                let length = length.expect("length-delimited fixture request");
                assert!(length <= 4 * 1_048_576);
                let mut body = vec![0; length];
                reader.read_exact(&mut body).expect("request body");
                let request = serde_json::from_slice(&body).expect("request JSON");
                let response = serde_json::to_vec(&handler(request)).expect("response JSON");
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.len()).expect("response headers");
                stream.write_all(&response).expect("response body");
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
            if !std::thread::panicking() {
                worker.join().expect("fixture worker");
            } else {
                let _ = worker.join();
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
                .expect("key JSON"),
        )
        .expect("fixture receipt pin");
        let owner = derive_from_seed(&[0x41; 32], 0).expect("fixture owner");
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
            let result = node_response(&mut state.lock().expect("fixture store"), &request);
            json!({"jsonrpc":"2.0", "id":request["id"], "result":result})
        });
        self.endpoint = server.endpoint.clone();
        self.server = Some(server);
    }
    fn rpc(&self) -> RpcClient {
        let rpc = RpcClient::connect_node(&self.endpoint, None)
            .expect("SDK loopback RPC")
            .with_clock(Arc::new(OsClock::new()));
        let state = self.state.lock().expect("fixture state");
        if state["_committed"].is_null() {
            return rpc;
        }
        let record = &state["_committed"];
        let authority = layerx_sdk::rpc_verification::node_commit::NodeCommitAuthority::new(
            "committed-fixture".to_owned(),
            vec![(
                unhex("c614a1c91884c3c13bed586a16d51f7253072f7cc41018ea28dd864e4e548e4d")
                    .try_into()
                    .expect("pinned validator"),
                7_000_000_000,
            )],
            10,
            10,
        )
        .expect("independent validator pin");
        let coverage = layerx_sdk::rpc_verification::node_commit::NodeCommitCoverage {
            covering_height: 10,
            batch_transaction: unhex(field(&record["batch"], "tx")),
            transaction_proof: layerx_sdk::rpc_verification::node_commit::NodeTransactionProof {
                total: record["batch"]["proof"]["total"]
                    .as_u64()
                    .expect("proof total"),
                index: record["batch"]["proof"]["index"]
                    .as_u64()
                    .expect("proof index"),
                aunts: record["batch"]["proof"]["aunts"]
                    .as_array()
                    .expect("proof aunts")
                    .iter()
                    .map(|v| {
                        unhex(v.as_str().expect("aunt"))
                            .try_into()
                            .expect("aunt width")
                    })
                    .collect(),
            },
            activity_ids: vec![fixed_field(&record["receiptJson"], "txHash")],
        };
        rpc.with_node_commit_authority(authority, coverage)
            .expect("real recorded batch inclusion")
    }
    fn restart(&mut self) {
        self.server.take();
        let retained =
            serde_json::to_vec(&*self.state.lock().expect("retained store")).expect("store codec");
        fs::write(self.home.join("node.json"), retained).expect("persist fixture store");
        self.state = Arc::new(Mutex::new(
            serde_json::from_slice(
                &fs::read(self.home.join("node.json")).expect("reopen fixture store"),
            )
            .expect("recovered store codec"),
        ));
        self.open();
    }
}

fn node_response(state: &mut Value, request: &Value) -> Value {
    let params = &request["params"];
    match field(request, "method") {
        "eth_chainId" => json!("0x7d"),
        "eth_gasPrice" => json!("0x1"),
        "eth_maxPriorityFeePerGas" => json!("0x1"),
        "eth_estimateGas" => json!("0x186a0"),
        "eth_getTransactionCount" => {
            assert_eq!(evm_address(&params[0]), evm_address(&state["owner"]));
            json!(format!("0x{:x}", state["nonce"].as_u64().expect("nonce")))
        }
        "eth_getBalance" => {
            assert_eq!(evm_address(&params[0]), evm_address(&state["owner"]));
            json!("0x56bc75e2d63100000")
        }
        "eth_sendRawTransaction" => {
            let raw = unhex(params[0].as_str().expect("raw transaction"));
            let (call, signature) =
                decode_signed(&raw).expect("fixture receives canonical signed transaction");
            let sender = recover_sender(&call, &signature).expect("fixture signature recovery");
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
            assert_eq!(call.nonce, state["nonce"].as_u64().expect("stored nonce"));
            if call.to == call_target(0x101a) {
                let disclosure = layerx_crypto::disclosure::call::bind_call(&call, sender)
                    .expect("fixture payment codec");
                if let Some(Payment::Mint { amount, .. }) = disclosure.payment {
                    state["token"] = json!(amount as u64);
                }
            } else if call.to == call_target(0x101d) && call.data[..4] == [0xbd, 0xe3, 0xff, 0x66] {
                let program = hex(&call.data[4..36]);
                let mut registered = vec![0; 192];
                registered[127] = 5;
                state["registry"][program] = json!(format!("0x{}", hex(&registered)));
            } else {
                assert_eq!(call.to, call_target(0x101d));
            }
            let seq = state["nonce"].as_u64().expect("nonce") + 1;
            state["nonce"] = json!(seq);
            state["transactions"][&hash] = params[0].clone();
            state["receipts"][&hash] = fixture_receipt(&raw, seq);
            json!(hash)
        }
        "eth_getRawTransactionByHash" => {
            state["transactions"][params[0].as_str().expect("hash")].clone()
        }
        "pax_sequencerReceipt" => state["receipts"][params[0].as_str().expect("hash")].clone(),
        "eth_getTransactionReceipt" => {
            let hash = params[0].as_str().expect("hash");
            if state["receipts"][hash].is_null() {
                Value::Null
            } else {
                json!({"transactionHash":hash,"status":"0x1"})
            }
        }
        "eth_getBlockByNumber" if !state["_committed"].is_null() => {
            json!({"number":state["_committed"]["receiptJson"]["blockNumber"],"hash":state["_committed"]["receiptJson"]["blockHash"]})
        }
        "px_getCommit" => {
            assert_eq!(params, &json!(["10"]));
            json!({"height":"10","signedHeader":state["_committed"]["commit"]["signed_header"]})
        }
        "eth_getBlockByNumber" => json!({"number":"0x3e8","hash":format!("0x{}",hex(&[0x33;32]))}),
        "eth_call" => {
            let data = unhex(params[0]["data"].as_str().expect("call data"));
            let selector = &data[..4];
            let mut word = vec![0; 32];
            if selector == layerx_client::sequencer::GET_EVM_ADDR_BY_LAYERX {
                word[12..].copy_from_slice(&unhex(state["owner"].as_str().expect("owner")));
            } else if selector
                == layerx_crypto::disclosure::call::ledger_selector("accountId(bytes32,bytes32)")
            {
                word = [0xa1; 32].to_vec();
            } else if selector
                == layerx_crypto::disclosure::call::ledger_selector("balanceOf(bytes32,bytes32)")
            {
                assert_eq!(&data[4..36], &[0xa1; 32]);
                word[24..].copy_from_slice(
                    &state["token"]
                        .as_u64()
                        .expect("token balance")
                        .to_be_bytes(),
                );
            } else if selector
                == layerx_crypto::disclosure::call::ledger_selector("getProgram(bytes32)")
            {
                return state["registry"][hex(&data[4..36])].clone();
            } else {
                panic!("unexpected fixture eth_call selector: {}", hex(selector));
            }
            json!(format!("0x{}", hex(&word)))
        }
        method => panic!("unexpected fixture RPC method {method}"),
    }
}
fn call_target(value: u16) -> [u8; 20] {
    let mut address = [0; 20];
    address[18..].copy_from_slice(&value.to_be_bytes());
    address
}

fn evm_address(value: &Value) -> layerx_types::intent::EvmAddress {
    let text = value.as_str().expect("EVM address text");
    assert!(text.starts_with("0x"));
    layerx_types::intent::EvmAddress::new(unhex(text).try_into().expect("20-byte EVM address"))
}

// Canonical node-protocol peer fixture emitted through the real wire encoder. It
// signs the exact execution fields consumed by the production SDK verifier.
fn fixture_receipt(raw: &[u8], seq: u64) -> Value {
    use layerx_wire::encode::Encoder;
    let (call, signature) = decode_signed(raw).expect("fixture signed call");
    let sender = recover_sender(&call, &signature).expect("fixture signer");
    let activity = transaction_hash(raw);
    let mut returned = Vec::new();
    let mut logs = Vec::<Value>::new();
    if call.to == call_target(0x101d) && call.data[..4] == [0x54, 0xbd, 0x2b, 0x48] {
        let (native, _) = layerx_crypto::disclosure::call::decode_program_call(&call.data)
            .expect("production Programs ABI decoder");
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
    let mut summary = Vec::new();
    for n in [100_000_u64, 100_000, 3] {
        summary.extend_from_slice(&n.to_be_bytes());
    }
    summary.extend_from_slice(&1_u32.to_be_bytes());
    summary.extend_from_slice(&[0; 40]);
    summary.extend_from_slice(&Sha256::digest([]));
    summary.extend_from_slice(&(returned.len() as u32).to_be_bytes());
    summary.extend_from_slice(&(logs.len() as u32).to_be_bytes());
    summary.push(4);
    summary.extend_from_slice(b"uhpx");
    let mut effects = vec![summary];
    for log in &logs {
        let mut body = unhex(field(log, "address"));
        let topics = log["topics"].as_array().expect("fixture topics");
        body.push(topics.len() as u8);
        for topic in topics {
            body.extend_from_slice(&unhex(topic.as_str().expect("topic")));
        }
        body.extend_from_slice(&Sha256::digest(unhex(field(log, "data"))));
        effects.push(body);
    }
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
        .expect("receipt header");
    e.u16(2).expect("protocol");
    e.bytes(&activity, 32).expect("activity");
    e.u64(seq).expect("sequence");
    for value in [previous, resulting, Sha256::digest(&returned).into()] {
        e.bytes(&value, 32).expect("receipt root");
    }
    e.i32(0).expect("result");
    e.sequence_length(effects.len(), 512).expect("effect count");
    for (ordinal, body) in effects.iter().enumerate() {
        e.u16(12).expect("summary module");
        e.u16(ordinal as u16).expect("ordinal");
        e.u16(0).expect("event type");
        e.u8(if ordinal == 0 { 1 } else { 3 }).expect("effect kind");
        e.u8(0).expect("nonmonetary");
        e.bytes(&[0; 32], 32).expect("transfer root");
        e.bytes(body, 256).expect("effect body");
    }
    e.u128(300_000).expect("fee");
    e.bytes(&height, 32).expect("height");
    e.u16(12).expect("module");
    e.u32(1).expect("layout");
    e.u32(0).expect("parameters");
    e.u8(2).expect("call operation");
    e.bytes(&Sha256::digest(b"uhpx"), 32).expect("asset");
    e.u128(0).expect("amount");
    e.bytes(&from, 32).expect("from");
    e.u128(1_000_000).expect("before");
    e.u128(700_000).expect("after");
    e.u64(call.nonce).expect("nonce");
    e.bytes(&to, 32).expect("to");
    e.u128(0).expect("to before");
    e.u128(0).expect("to after");
    for value in [[0; 32], [0; 32], key_id] {
        e.bytes(&value, 32).expect("authorization");
    }
    e.u64(1_000_000 + seq).expect("timestamp");
    e.u8(0).expect("unsigned receipt");
    let unsigned = e.finish();
    let decoded = layerx_wire::receipt::decode(&unsigned).expect("real receipt codec");
    assert_eq!(
        layerx_wire::receipt::encode_unsigned(&decoded).expect("canonical unsigned receipt"),
        unsigned
    );
    let digest =
        layerx_wire::hash::receipt_digest(&unsigned).expect("domain-separated receipt digest");
    let signature = key.sign(&digest).to_bytes();
    let mut canonical = unsigned;
    canonical.pop();
    canonical.push(1);
    canonical.extend_from_slice(&64_u32.to_be_bytes());
    canonical.extend_from_slice(&signature);
    let decoded = layerx_wire::receipt::decode(&canonical).expect("real signed receipt codec");
    assert_eq!(
        layerx_wire::receipt::encode(&decoded).expect("canonical signed receipt"),
        canonical
    );
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
    let endpoint = url::Url::parse(endpoint).expect("owned node endpoint");
    let request = serde_json::to_vec(&json!({
        "jsonrpc": "2.0", "id": "receipt-diagnostic",
        "method": "eth_getTransactionReceipt", "params": [hash]
    }))
    .expect("receipt diagnostic request");
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
                    eprintln!("eth_getTransactionReceipt diagnostic body failed: {error}")
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
        .send(serde_json::to_vec(&json!({"jsonrpc":"2.0","id":1,"method":"eth_call","params":[{"to":format!("0x{}", hex(to)),"data":format!("0x{}",hex(data))},"latest"]})).expect("setup read request").as_slice()).expect("real precompile read");
    let document: Value =
        serde_json::from_slice(&response.body_mut().read_to_vec().expect("precompile reply"))
            .expect("precompile JSON");
    assert!(
        document.get("error").is_none(),
        "precompile refused: {document}"
    );
    unhex(document["result"].as_str().expect("precompile hex"))
}

fn quantity_latest(node: &LocalNode, method: &str, address: &str) -> u128 {
    let request = serde_json::to_vec(&json!({
        "jsonrpc": "2.0", "id": "latest-quantity",
        "method": method, "params": [address, "latest"]
    }))
    .expect("latest quantity request");
    let mut response = ureq::post(&node.endpoint)
        .header("Content-Type", "application/json")
        .send(request.as_slice())
        .expect("real latest quantity transport");
    let document: Value = serde_json::from_slice(
        &response
            .body_mut()
            .read_to_vec()
            .expect("latest quantity reply"),
    )
    .expect("latest quantity JSON");
    assert_eq!(document["jsonrpc"], "2.0");
    assert_eq!(document["id"], "latest-quantity");
    assert!(
        document.get("error").is_none(),
        "{method} at latest refused: {document}"
    );
    let quantity = document["result"]
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .expect("latest hex quantity");
    u128::from_str_radix(quantity, 16).expect("latest quantity fits u128")
}

fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("hex text"), 16).expect("hex byte")
        })
        .collect()
}
fn registration(actor: &str, salt: [u8; 32]) -> Registration {
    let did = layerx_types::ids::Did::new(actor.as_bytes()).expect("real DID");
    let issuer = layerx_wire::hash::did_id_for_protocol(&did, 3).expect("real issuer id");
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
    let state = node.state.lock().expect("fixture state");
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
        &fs::read(node.home.join("receipt-public.json")).expect("node receipt key output"),
    )
    .expect("node key JSON");
    let key: [u8; 32] = unhex(public["public_key"].as_str().expect("node-generated key"))
        .try_into()
        .expect("key width");
    ReceiptPolicy {
        protocol_version: 2,
        network_id: 125,
        sequencer: SequencerAuthorization::new(Sha256::digest(key).into(), key, 1, u64::MAX),
        trusted_checkpoint_context_digest: None,
    }
}
fn funded_owner(node: &LocalNode) -> layerx_crypto::account_derivation::DerivedAccount {
    let owner = derive_from_seed(&[0x41; 32], 0).expect("fixture owner derivation");
    assert_eq!(
        node.state.lock().expect("funded state")["owner"],
        owner.evm_address_text()
    );
    assert_eq!(
        quantity_latest(node, "eth_getBalance", &owner.evm_address_text()),
        100_000_000_000_000_000_000
    );
    owner
}

#[test]
fn wallet_native_token_and_node_summary_round_trip_after_reopen() {
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
    let created = run(wallet.create(asset.clone(), &options))
        .expect("node-produced summary must verify in public Wallet::create");
    assert_eq!(
        created
            .receipt()
            .protocol()
            .expect("protocol")
            .result_code(),
        0
    );
    let mut options = options;
    options.idempotency_key = [0x62; 32];
    run(wallet.open_account(asset.asset, &options)).expect("real token account");
    let mut account_call =
        layerx_crypto::disclosure::call::ledger_selector("accountId(bytes32,bytes32)").to_vec();
    account_call.extend_from_slice(&owner.layerx_public_key());
    account_call.extend_from_slice(&asset.asset);
    let ledger = call().to;
    let account: [u8; 32] = eth_call(&node, &ledger, &account_call)
        .try_into()
        .expect("live asset mapping");
    options.idempotency_key = [0x63; 32];
    let minted =
        run(wallet.mint(asset.asset, account, 73, &options)).expect("fund real ledger token");
    let hash = minted.receipt().protocol().expect("receipt").activity_id();
    mined(&rpc, &node.endpoint, &format!("0x{}", hex(&hash)));
    assert_eq!(
        wallet
            .balance(&owner.did(), asset.asset)
            .expect("public token balance"),
        json!("0x49")
    );
    assert_eq!(
        rpc.wallet([3; 32])
            .balance(&owner.did(), asset.asset)
            .expect("RPC token balance"),
        json!("0x49")
    );
    let native = wallet
        .balance(&owner.did(), [3; 32])
        .expect("mapped native balance");
    assert_eq!(
        native,
        json!(format!(
            "0x{:x}",
            quantity_latest(&node, "eth_getBalance", &owner.evm_address_text())
        ))
    );
    let served = rpc.sequencer_receipt(hash).expect("live receipt producer");
    let (_, canonical) =
        verify_node_receipt(&served, hash, &policy).expect("pinned height-ranged summary");
    assert_eq!(canonical, minted.canonical_bytes());
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
            verify_node_receipt(&altered, hash, &policy).is_err(),
            "must bind {field}"
        );
    }
    let wrong = ReceiptPolicy {
        sequencer: SequencerAuthorization::new([0x55; 32], [0x66; 32], 1, u64::MAX),
        ..pinned_policy(&node)
    };
    assert!(verify_node_receipt(&served, hash, &wrong).is_err());
    let outside = ReceiptPolicy {
        sequencer: SequencerAuthorization::new(
            policy.sequencer.sequencer_id(),
            policy.sequencer.public_key(),
            u64::MAX,
            u64::MAX,
        ),
        ..pinned_policy(&node)
    };
    assert!(verify_node_receipt(&served, hash, &outside).is_err());
    drop(wallet);
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
            .expect("recovered token"),
        json!("0x49")
    );
    assert_eq!(
        wallet
            .balance(&owner.did(), [3; 32])
            .expect("recovered native"),
        native
    );
    let recovered = wallet
        .wait_for(hash, Commitment::Receipted, Duration::from_secs(30))
        .expect("recovered WAL receipt");
    assert_eq!(recovered.canonical_bytes(), canonical);
    assert_eq!(recovered.rung(), layerx_types::rung::Rung::Receipted);
}

#[test]
fn committed_wait_uses_live_block_authority_after_reopen() {
    let mut node = LocalNode::start();
    let record: Value = serde_json::from_str(include_str!(
        "../src/repair_22_31_227_fixture/committed.json"
    ))
    .expect("recorded real node/Comet producer transcript");
    let hash = fixed_field(&record["receiptJson"], "txHash");
    {
        let mut state = node.state.lock().expect("fixture store");
        state["transactions"][field(&record["receiptJson"], "txHash")] =
            record["rawTransaction"].clone();
        state["receipts"][field(&record["receiptJson"], "txHash")] = record["receiptJson"].clone();
        state["_committed"] = record.clone();
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
        .expect("recorded real executed mutation");
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
            .expect("committed authority request")
            .as_slice(),
        )
        .expect("restarted node serves persisted committed authority");
    let recovered_authority: Value = serde_json::from_slice(
        &response
            .body_mut()
            .read_to_vec()
            .expect("recovered commit body"),
    )
    .expect("recovered commit JSON");
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
        .expect("live committed rung must acquire actual committed-block authority");
    assert_eq!(committed.rung(), layerx_types::rung::Rung::Committed);
    assert!(committed.committed_block().is_some());
    assert_eq!(committed.canonical_bytes(), result.canonical_bytes());
    drop(reopened);
    for (section, field_name, replacement) in [
        ("header", "data_hash", json!("00".repeat(32))),
        ("header", "chain_id", json!("foreign-chain")),
        ("commit", "height", json!("11")),
    ] {
        node.state.lock().expect("fixture store")["_committed"]["commit"]["signed_header"]
            [section][field_name] = replacement;
        assert!(
            node.rpc()
                .wait_for(hash, Commitment::Committed, &policy, Duration::from_secs(1))
                .is_err(),
            "must refuse altered committed {section}/{field_name}"
        );
        node.state.lock().expect("fixture store")["_committed"] = record.clone();
    }

    let unpinned = RpcClient::connect_node(&node.endpoint, None)
        .expect("unpinned node transport")
        .with_clock(Arc::new(OsClock::new()));
    assert!(matches!(
        unpinned.wait_for(hash, Commitment::Committed, &policy, Duration::from_secs(1)),
        Err(RpcError::MissingFinalityTrust)
    ));
    {
        let mut state = node.state.lock().expect("fixture store");
        state["_committed"]["batch"]["proof"]["aunts"][0] = json!(format!("0x{}", hex(&[0; 32])));
    }
    assert!(
        node.rpc()
            .wait_for(hash, Commitment::Committed, &policy, Duration::from_secs(1))
            .is_err(),
        "commit signature cannot replace batch inclusion"
    );
    node.state.lock().expect("fixture store")["_committed"] = record.clone();
    {
        let mut state = node.state.lock().expect("fixture store");
        state["_committed"]["commit"]["signed_header"]["commit"]["signatures"][0]["signature"] =
            json!("A".repeat(88));
    }
    assert!(
        node.rpc()
            .wait_for(hash, Commitment::Committed, &policy, Duration::from_secs(1))
            .is_err(),
        "the committed label cannot replace quorum signatures"
    );
}

#[test]
fn daemon_not_ready_goldens_preserve_value_and_rung_order() {
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
        let document: Value =
            serde_json::from_slice(&fs::read(path).expect("actual daemon success golden"))
                .expect("producer JSON");
        let decoded = decode_response_status(200, &document)
            .expect("declared shape")
            .expect("not-ready is a success");
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
    .expect("live JSON-RPC Programs transport with pinned network and receipt authority")
}

#[test]
fn program_constructors_share_live_abi_submit_retry_and_recovery() {
    use layerx_sdk::programs::{
        NativeProgramCallRequest, NativeProgramLifecycleRequest, NativeProgramTransport,
        ProgramCallRequest, ProgramLifecycleSubmission, ProgramSubmission, ProgramTransport,
    };
    use layerx_types::intent::{
        CallBudget, Calldata, ProgramCall, ProgramId, RequestedCapabilities,
    };
    use layerx_types::program_call::{NativeProgramCall, Resources};
    use layerx_types::program_lifecycle::{NativeProgramDeploy, ProgramUpgradePolicy};
    let mut node = LocalNode::start();
    let owner = funded_owner(&node);
    let rpc = node.rpc();
    let registry =
        layerx_sdk::programs::programs_module_registry().expect("real Programs registrations");
    let fixture: Value = serde_json::from_slice(
        &fs::read(
            repository().join("sdk/platform/conformance/fixtures/native-program-deploy-v4.json"),
        )
        .expect("C native producer fixture"),
    )
    .expect("native deploy JSON");
    let payload = unhex(
        fixture["payload_hex"]
            .as_str()
            .expect("native deploy payload"),
    );
    let original = NativeProgramDeploy::decode(&payload).expect("real lifecycle codec");
    let deploy = NativeProgramDeploy {
        program_id: ProgramId::new([0x71; 32]),
        guest_abi: 5,
        policy: ProgramUpgradePolicy::Immutable,
        interface: None,
        ..original
    };
    let payload = deploy.encode().expect("ABI5 deploy producer");
    let (chain_id, tip, maximum) = rpc.eip1559_parameters().expect("live fees");
    let mut to = [0; 20];
    to[18..].copy_from_slice(&0x101d_u16.to_be_bytes());
    let transaction = Eip1559Call {
        chain_id,
        nonce: rpc
            .get_identity_sequence(&owner.evm_address_text())
            .expect("pending nonce"),
        max_priority_fee_per_gas: tip,
        max_fee_per_gas: maximum,
        gas_limit: 10_000_000,
        to,
        data: layerx_sdk::program_lifecycle::lifecycle_calldata(1, &payload)
            .expect("actual deployment ABI"),
    };
    let raw = transaction
        .sign(owner.evm_secret().expect("external owner key"))
        .expect("external deploy signing");
    let request = NativeProgramLifecycleRequest::deploy(&registry, deploy, &raw)
        .expect("public deploy constructor");
    let transport = program_transport(&node);
    let submitted = transport
        .deploy(&request, request.bound_idempotency_key())
        .expect("live deploy write");
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
    mined(&rpc, &node.endpoint, &format!("0x{}", hex(&activity_id)));
    let receipt = transport
        .lifecycle_receipt(&request)
        .expect("signed node lifecycle receipt");
    assert_eq!(receipt.protocol().expect("protocol").result_code(), 0);
    let grants = layerx_sdk::native_capabilities::NativeCapabilitySet::new(vec![])
        .expect("exact empty grants")
        .encode()
        .expect("native grants codec");
    let native = NativeProgramCall {
        program_id: deploy.program_id,
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
            .expect("post-deploy nonce"),
        data: layerx_crypto::disclosure::call::encode_program_call(&native, fee),
        ..transaction
    };
    let signed = call
        .sign(owner.evm_secret().expect("external key"))
        .expect("sign real ABI");
    let new = NativeProgramCallRequest::new(&registry, native, fee, &signed)
        .expect("public native constructor");
    let from = NativeProgramCallRequest::from_signed_transaction(native, fee, &signed)
        .expect("transport native constructor");
    assert_eq!(new, from);
    let generic = ProgramCall::new(
        deploy.program_id,
        Calldata::new(b"").expect("typed calldata"),
        CallBudget::new(
            native.resources.0[0],
            layerx_types::amount::Amount::from_u128(fee),
        )
        .expect("typed fee semantics"),
        RequestedCapabilities::none(),
    );
    let public = ProgramCallRequest::new(&registry, generic, &signed)
        .expect("public generic constructor agrees with actual ABI");
    assert_eq!(public.bound_activity_id(), new.bound_activity_id());
    assert_eq!(public.bound_idempotency_key(), new.bound_idempotency_key());
    assert_ne!(public.bound_idempotency_key(), transaction_hash(&signed));
    let key = new.bound_idempotency_key();
    let hash = new.bound_activity_id();
    for submission in [
        transport.submit_native(&new, key).expect("native consumer"),
        transport
            .submit(&public, key)
            .expect("retry same signed bytes through public consumer"),
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
        .node_read(deploy.program_id.bytes(), "getProgram(bytes32)")
        .expect("served node registry read");
    assert_eq!(before.bytes.len(), 192);
    assert_eq!(before.bytes[127], 5);
    let ProgramSubmission::Receipted(executed) = transport
        .receipt(key, hash)
        .expect("live signed call receipt")
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
    let recovered = program_transport(&node);
    let ProgramSubmission::Receipted(executed) = recovered
        .receipt(key, hash)
        .expect("reopened node and fresh client recovery")
    else {
        panic!("recovered call lost execution")
    };
    assert_eq!(executed.transaction_hash(), hash);
    assert_eq!(executed.idempotency_key(), key);
    assert_eq!(executed.terminal_result(), Some(0));
    assert_eq!(executed.canonical_receipt(), canonical);
    assert_eq!(
        recovered
            .node_read(deploy.program_id.bytes(), "getProgram(bytes32)")
            .expect("reopened registry")
            .bytes,
        before.bytes
    );
    assert!(
        recovered.receipt([0x55; 32], hash).is_err(),
        "foreign idempotency cannot recover this transaction"
    );
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
    let account = derive_from_seed(&[0x41; 32], 0).expect("owner fixture account");
    let actor = account.did();
    let grants = layerx_sdk::native_capabilities::NativeCapabilitySet::new(vec![])
        .expect("empty capability fixture")
        .encode()
        .expect("capability codec");
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
    let payload = native.encode().expect("production native payload encoder");
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
        tenant: TenantId::new("sdk-fixture").expect("tenant"),
        agent_did: AgentDid::new(&actor).expect("actor"),
        session_id: SessionId::new(hex(&[0x51; 32])).expect("session"),
        generation: 1,
        expires_at_ms: u64::MAX,
        capability_id: CapabilityId::new(hex(&[0x53; 32])).expect("capability"),
        preparation_id: digest,
        canonical_digest: digest,
        commitment: [0x54; 32],
    };
    let purpose_bytes = purpose.canonical_bytes().expect("owner purpose codec");
    let purpose_signature = owner.sign(&Sha256::digest(&purpose_bytes));
    let external = [0x42; 32];
    let signed_raw = call.sign(&external).expect("external fixture signer");
    let (_, signature) = decode_signed(&signed_raw).expect("signature codec");
    let sender = recover_sender(&call, &signature).expect("fixture sender");
    let disclosure = layerx_crypto::disclosure::call::bind_call(&call, sender)
        .expect("production call disclosure");
    let payload_hash: [u8; 32] = Sha256::new()
        .chain_update(layerx_wire::hash::Domain::PayloadHash.tag())
        .chain_update(&payload)
        .finalize()
        .into();
    let preparation = json!({"version":"1","preparation_id":hex(&digest),"canonical_bytes":hex(&canonical),"signing_preimage":hex(&call.signing_hash()),"activity":{"version":"1","module":"9","ordinal":"3"},"approval_required":false,"approval_id":null});
    let disclosed = json!({"version":"1","preparation_id":hex(&digest),"canonical_bytes":hex(&canonical),"disclosure_digest":hex(&disclosure.audit_digest().expect("call audit codec")),"activity_type":disclosure.activity_type.value().to_string(),"actor":hex(&disclosure.actor),"authority":hex(&disclosure.authority),"asset":hex(&disclosure.asset),"fee_limit":disclosure.fee_limit.to_string(),"not_before":disclosure.expiry.not_before.to_string(),"not_after":disclosure.expiry.not_after.to_string(),"payload_expires_at":disclosure.expiry.payload_expires_at.to_string(),"idempotency_key":hex(&disclosure.idempotency_key)});
    let mut document = json!({"tenant":"sdk-fixture","session_id":hex(&[0x51;32]),"token_id":hex(&[0x52;32]),"generation":"1","ca_pem":Path::new(env!("CARGO_MANIFEST_DIR")).join("src/repair_22_31_227_fixture/ca.pem").to_str().expect("CA path"),"owner_preview_purpose":hex(&purpose_bytes),"owner_public_key":hex(owner.verifying_key().as_bytes()),"evm_sender":hex(&sender),"owner_purpose_signature":hex(&purpose_signature.to_bytes()),"owner_preview_canonical_bytes":hex(&canonical),"external_evm_secret":hex(&external),"request":{"actor":actor,"authority":hex(owner.verifying_key().as_bytes()),"account_sequence":"7","not_before":"0","not_after":u64::MAX.to_string(),"idempotency_key":hex(&digest),"fee_limit":"200000","payload":hex(&payload),"payload_hash":hex(&payload_hash),"capability_id":hex(&[0x53;32])},"pending_lookup":{"activity_id":hex(&[0xab;32]),"required":"receipted"},"absent_lookup":{"activity_id":hex(&[0xcd;32]),"required":"receipted"}});
    let fixture = document.clone();
    let pending: Value = serde_json::from_str(include_str!(
        "../../../../../services/agent/schema/agent-api/golden/agent-http-success-pending.json"
    ))
    .expect("daemon pending golden");
    let absent: Value = serde_json::from_str(include_str!(
        "../../../../../services/agent/schema/agent-api/golden/agent-http-success-absent.json"
    ))
    .expect("daemon absent golden");
    let server = Server::tls(move |envelope| {
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
                .expect("production 65-byte API carrier");
                assert_eq!(sig.as_bytes(), signature.as_bytes());
                let sig = AgentSignature::new(sig.as_bytes().try_into().expect("signature width"))
                    .expect("signature codec");
                assert_eq!(
                    recover_sender(&call, &sig).expect("external signer recovery"),
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
    });
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

#[test]
fn native_owner_preview_prepare_disclose_external_sign_and_submit() {
    use layerx_agent_api::error::RequestId;
    use layerx_agent_api::idempotency::Key;
    use layerx_agent_api::identity::{
        AgentDid, CapabilityId, NativeActivity, NativeDisclosureRequestV1,
        NativePreparationPurposeV1, NativePrepareRequestV1, SignedNativePreparationPurposeV1,
    };
    use layerx_agent_api::submit::{PreparationRef, SignRequest, SignatureBytes, SubmitRequest};
    use layerx_sdk::agent_envelope::{
        validate_call_disclosure, AgentEnvelopeTransport, EnvelopeCredential,
    };
    let case = owner_case();
    let credential = EnvelopeCredential::new(
        field(&case, "tenant"),
        fixed_field(&case, "session_id"),
        fixed_field(&case, "token_id"),
        field(&case, "generation").parse().expect("generation"),
    )
    .expect("real admitted session");
    let ca = PathBuf::from(field(&case, "ca_pem"));
    let transport = AgentEnvelopeTransport::connect(field(&case, "endpoint"), None, Some(&ca))
        .expect("real daemon gateway TLS");
    let input = &case["request"];
    let purpose = NativePreparationPurposeV1::from_canonical_bytes(&unhex(field(
        &case,
        "owner_preview_purpose",
    )))
    .expect("actual owner canonical preview purpose");
    let owner = fixed_field::<32>(&case, "owner_public_key");
    let sender = fixed_field::<20>(&case, "evm_sender");
    let transport = transport
        .with_native_authority(owner, sender)
        .expect("independently resolved owner address");
    let request = NativePrepareRequestV1 {
        activity: NativeActivity::from(
            layerx_types::payload::ActivityType::new(layerx_types::payload::ModuleId::Programs, 3)
                .expect("Programs call"),
        ),
        actor: AgentDid::new(field(input, "actor")).expect("actor"),
        authority: field(input, "authority").to_owned(),
        account_sequence: field(input, "account_sequence").parse().expect("nonce"),
        not_before: field(input, "not_before").parse().expect("not before"),
        not_after: field(input, "not_after").parse().expect("not after"),
        idempotency_key: fixed_field(input, "idempotency_key"),
        fee_limit: field(input, "fee_limit").parse().expect("fee"),
        payload: unhex(field(input, "payload")),
        payload_hash: fixed_field(input, "payload_hash"),
        capability_id: CapabilityId::new(field(input, "capability_id")).expect("capability"),
        purpose: SignedNativePreparationPurposeV1 {
            purpose,
            owner_public_key: owner,
            signature: fixed_field(&case, "owner_purpose_signature"),
        },
        local_grant: None,
    };
    let registry = layerx_sdk::programs::programs_module_registry().expect("registry");
    let prepared = transport
        .prepare_native(
            RequestId(2271),
            Key::new(request.idempotency_key).expect("prepare key"),
            &request,
            &credential,
            &registry,
        )
        .expect("real owner preparation must remain unsigned")
        .value;
    let call = layerx_crypto::evm_transaction::decode_unsigned(&prepared.canonical_bytes)
        .expect("canonical unsigned EIP-1559");
    assert_eq!(call.signing_hash(), prepared.signing_preimage);
    assert!(decode_signed(&prepared.canonical_bytes).is_err());
    assert_eq!(
        prepared.canonical_bytes,
        unhex(field(&case, "owner_preview_canonical_bytes"))
    );
    let disclosure_request = NativeDisclosureRequestV1 {
        canonical_bytes: prepared.canonical_bytes.clone(),
    };
    let disclosed = transport
        .disclose_native_preparation(
            RequestId(2272),
            Key::new([0x72; 32]).expect("disclosure key"),
            &disclosure_request,
            &credential,
            &registry,
        )
        .expect("actual daemon disclosure")
        .value;
    let disclosure = validate_call_disclosure(&disclosure_request, &disclosed, sender)
        .expect("migrated bind_call adapter");
    assert_eq!(
        disclosure.audit_digest().expect("actual call audit"),
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
    let mut bad_preimage = prepared.clone();
    bad_preimage.signing_preimage = [0x55; 32];
    assert!(layerx_sdk::agent_envelope::validate_unsigned_preparation(
        &bad_preimage,
        prepared.preparation_id,
        prepared.preparation_id
    )
    .is_err());

    let key = Zeroizing::new(fixed_field::<32>(&case, "external_evm_secret"));
    let raw = call.sign(&key).expect("external EVM signer");
    let (_, signature) = decode_signed(&raw).expect("signed call");
    assert_eq!(recover_sender(&call, &signature).expect("recovery"), sender);
    let carrier =
        SignatureBytes::new(signature.as_bytes().to_vec()).expect("actual 65-byte API carrier");
    assert!(SignatureBytes::new(signature.as_bytes()[..64].to_vec()).is_err());
    let reference =
        PreparationRef::new(hex(&prepared.preparation_id)).expect("retained preparation reference");
    let signed = transport
        .sign_request(
            RequestId(2273),
            Key::new([0x73; 32]).expect("sign key"),
            &credential,
            &SignRequest {
                preparation_ref: reference.clone(),
                signature: carrier.clone(),
            },
        )
        .expect("external signature must cross the actual daemon sign carrier");
    assert_eq!(signed.activity_id, transaction_hash(&raw));
    let submit = SubmitRequest {
        preparation_ref: reference,
        signature: carrier,
        approval_release_ref: None,
    };
    let submitted=transport.send_operation(layerx_sdk::Operation::Submit,RequestId(2274),&json!({"preparation_ref":submit.preparation_ref.as_str(),"signature":hex(submit.signature.as_bytes()),"approval_release_ref":null}),Some(&credential),Some(Key::new([0x74;32]).expect("submit key"))).expect("real daemon submit");
    assert_eq!(submitted.value["activity_id"], hex(&transaction_hash(&raw)));
}

#[test]
fn abi5_reward_authority_receipt_recovers_after_node_reopen() {
    use layerx_sdk::programs::{
        NativeProgramCallRequest, NativeProgramTransport, ProgramSubmission, ProgramTransport,
    };
    use layerx_types::program_call::{NativeProgramCall, Resources};
    let mut node = LocalNode::start();
    let owner = funded_owner(&node);
    let grants = layerx_sdk::native_capabilities::NativeCapabilitySet::new(vec![])
        .expect("reward capabilities")
        .encode()
        .expect("capability codec");
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
        .sign(owner.evm_secret().expect("owner key"))
        .expect("reward signer");
    let registry = layerx_sdk::programs::programs_module_registry().expect("registry");
    let request = NativeProgramCallRequest::new(&registry, native, 200_000, &raw)
        .expect("reward ABI5 constructor");
    let key = request.bound_idempotency_key();
    let submission = program_transport(&node)
        .submit_native(&request, key)
        .expect("reward transaction transport");
    assert!(matches!(submission, ProgramSubmission::Submitted { .. }));
    let produced = json!({"signed_transaction":hex(&raw),"idempotency_key":hex(&key),"response":hex(&reward_response())});
    let raw = unhex(field(&produced, "signed_transaction"));
    let hash = transaction_hash(&raw);
    let key = fixed_field::<32>(&produced, "idempotency_key");
    let (call, _) = decode_signed(&raw).expect("real signed reward call");
    let (native, _) = layerx_crypto::disclosure::call::decode_program_call(&call.data)
        .expect("real ABI5 reward ABI");
    assert_eq!(native.guest_abi, 5);
    let transport = program_transport(&node);
    let ProgramSubmission::Receipted(reward) = transport
        .receipt(key, hash)
        .expect("real reward node receipt")
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
        .expect("recovered reward authority")
    else {
        panic!("reopened reward unresolved")
    };
    assert_eq!(reward.terminal_result(), Some(0));
    assert_eq!(reward.canonical_receipt(), canonical);
    let served = node
        .rpc()
        .sequencer_receipt(hash)
        .expect("real recovered reward receipt");
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
        .sign(owner.evm_secret().expect("owner key"))
        .expect("bounded call signer");
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
        .sign(owner.evm_secret().expect("owner key"))
        .expect("foreign call signer");
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

#[test]
fn daemon_not_ready_lookup_crosses_authenticated_transport() {
    use layerx_agent_api::error::RequestId;
    use layerx_sdk::agent_envelope::{
        AgentEnvelopeTransport, EnvelopeCredential, EnvelopeOutcome, NotReadyStatus,
    };
    let case = owner_case();
    let ca = PathBuf::from(field(&case, "ca_pem"));
    let transport = AgentEnvelopeTransport::connect(field(&case, "endpoint"), None, Some(&ca))
        .expect("real gateway TLS");
    let credential = EnvelopeCredential::new(
        field(&case, "tenant"),
        fixed_field(&case, "session_id"),
        fixed_field(&case, "token_id"),
        field(&case, "generation").parse().expect("generation"),
    )
    .expect("real admitted session");
    for (index, name) in ["pending_lookup", "absent_lookup"].iter().enumerate() {
        let response = transport
            .send_operation_status(
                layerx_sdk::Operation::Wait,
                RequestId(2280 + index as u64),
                &case[*name],
                Some(&credential),
                None,
            )
            .expect("real daemon receipt lookup");
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
                assert_eq!(requested, layerx_types::rung::Rung::Receipted)
            }
            _ => panic!("not-ready state changed across transport"),
        }
    }
}

#[test]
fn program_abi_constructor_retains_c_producer_transaction() {
    use layerx_sdk::programs::NativeProgramCallRequest;
    use layerx_types::program_call::NativeProgramCall;
    let fixture: Value = serde_json::from_slice(
        &fs::read(
            repository().join("sdk/platform/conformance/fixtures/native-program-call-v4.json"),
        )
        .expect("actual C producer fixture"),
    )
    .expect("fixture JSON");
    let payload = unhex(field(&fixture, "payload_hex"));
    let native = NativeProgramCall::decode(&payload).expect("real native payload codec");
    let data = unhex(field(&fixture["evm"], "calldata"));
    let (decoded, fee) =
        layerx_crypto::disclosure::call::decode_program_call(&data).expect("node ABI codec");
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
        .expect("external EVM signing producer");
    let registry =
        layerx_sdk::programs::programs_module_registry().expect("real module registrations");
    let public = NativeProgramCallRequest::new(&registry, native, fee, &raw)
        .expect("public ABI constructor");
    let consumer = NativeProgramCallRequest::from_signed_transaction(native, fee, &raw)
        .expect("transport validation constructor");
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
    retired.data = payload.clone();
    assert!(NativeProgramCallRequest::new(
        &registry,
        native,
        fee,
        &retired.sign(&[0x42; 32]).expect("signed retired payload")
    )
    .is_err());
    let (_, signature) = decode_signed(&raw).expect("real recoverable signature");
    let carrier = layerx_agent_api::submit::SignatureBytes::new(signature.as_bytes().to_vec())
        .expect("65-byte carrier");
    assert_eq!(carrier.as_bytes().len(), 65);
    assert!(
        layerx_agent_api::submit::SignatureBytes::new(carrier.as_bytes()[..64].to_vec()).is_err()
    );
    assert_eq!(
        layerx_crypto::evm_transaction::assemble(&transaction, &signature),
        raw
    );
}
