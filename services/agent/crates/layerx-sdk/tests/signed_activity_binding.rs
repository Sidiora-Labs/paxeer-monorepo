//! Decision 2695 / gotcha 3286: bind signed transaction bytes before submitted waits.
//! Uses the existing SDK audit peer's wire producer and OS clock over loopback HTTP.
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::disclosure::call::bind_call;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_crypto::payments::Payment;
use layerx_crypto::signer::LocalSigner;
use layerx_proof::inclusion::{verify_receipt, SequencerAuthorization};
use layerx_proof::merkle::build_proof;
use layerx_sdk::rpc::{Commitment, RpcClient, RpcError};
use layerx_sdk::rpc_verification::node_commit::NodeCommitAuthority;
use layerx_sdk::rpc_verification::{ReceiptPolicy, VerifiedRpcReceipt};
use layerx_sdk::wallet::{PaymentOptions, Wallet};
use layerx_types::clock::{Clock, ClockError, ClockReading};
use layerx_types::rung::Rung;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::batch_header_digest;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::future::Future;
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::TcpListener;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};
use std::task::{Context, Poll, Wake, Waker};
use std::thread::JoinHandle;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

fn hex(bytes: &[u8]) -> String {
    hex::encode(bytes)
}
fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert!(text.len().is_multiple_of(2));
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
fn call_target(value: u16) -> [u8; 20] {
    let mut address = [0; 20];
    address[18..].copy_from_slice(&value.to_be_bytes());
    address
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

fn execution_summary(returned: &[u8], logs: &[Value]) -> Vec<u8> {
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
    summary
}

fn sign_receipt(unsigned: Vec<u8>, key: &SigningKey) -> Vec<u8> {
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

fn encode_receipt_effects(e: &mut Encoder, effects: &[Vec<u8>]) {
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

fn fixture_receipt(raw: &[u8], seq: u64) -> Value {
    use layerx_wire::encode::Encoder;
    let (call, signature) =
        decode_signed(raw).unwrap_or_else(|error| panic!("fixture signed call: {error:?}"));
    let sender = recover_sender(&call, &signature)
        .unwrap_or_else(|error| panic!("fixture signer: {error:?}"));
    let activity = transaction_hash(raw);
    let returned = Vec::<u8>::new();
    let logs = Vec::<Value>::new();
    let summary = execution_summary(&returned, &logs);
    let effects = [summary];
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
    let canonical = sign_receipt(unsigned, &key);
    json!({"rung":"receipted","txHash":format!("0x{}",hex(&activity)),"nonce":format!("0x{:x}",call.nonce),"seq":format!("0x{seq:x}"),"height":"0x3e8","index":"0x1","timestamp":format!("0x{:x}",1_000_000+seq),"status":"0x1","returnData":format!("0x{}",hex(&returned)),"logs":logs,"gasUsed":"0x186a0","cumulativeGasUsed":"0x186a0","effectiveGasPrice":"0x3","paymentDenom":"uhpx","feeDenom":"uhpx","feeAmount":"0x493e0","sponsor":format!("0x{}",hex(&[0;20])),"from":format!("0x{}",hex(&sender)),"to":format!("0x{}",hex(&call.to)),"contractAddress":format!("0x{}",hex(&[0;20])),"vmError":"","keyId":format!("0x{}",hex(&key_id)),"signingKey":format!("0x{}",hex(key.verifying_key().as_bytes())),"receipt":format!("0x{}",hex(&canonical))})
}

fn inclusion(summary: &Value) -> Value {
    inclusion_on_network(summary, 125)
}

fn inclusion_on_network(summary: &Value, network: u32) -> Value {
    let canonical = unhex(
        summary["receipt"]
            .as_str()
            .unwrap_or_else(|| panic!("signed receipt")),
    );
    let decoded = layerx_wire::receipt::decode(&canonical)
        .unwrap_or_else(|error| panic!("receipt producer codec: {error:?}"));
    let receipt = decoded
        .protocol()
        .unwrap_or_else(|| panic!("protocol receipt"));
    let (proof, root) = build_proof(&[b"prior receipt", &canonical, b"next receipt"], 1)
        .unwrap_or_else(|error| panic!("production Merkle constructor: {error:?}"));
    let key = SigningKey::from_bytes(&[0x5e; 32]);
    let key_id: [u8; 32] = Sha256::digest(key.verifying_key().as_bytes()).into();
    let mut encoder = Encoder::new(1024);
    encoder
        .structure_header_version(0x1701, 2)
        .unwrap_or_else(|error| panic!("batch header: {error:?}"));
    encoder
        .u8(15)
        .unwrap_or_else(|error| panic!("field count: {error:?}"));
    for field in 1..=15 {
        encoder
            .tag(field, 15)
            .unwrap_or_else(|error| panic!("batch field: {error:?}"));
        match field {
            1 => encoder
                .u16(2)
                .unwrap_or_else(|error| panic!("protocol: {error:?}")),
            2 => encoder
                .u32(network)
                .unwrap_or_else(|error| panic!("network: {error:?}")),
            3 => encoder
                .u64(1)
                .unwrap_or_else(|error| panic!("epoch: {error:?}")),
            4 => encoder
                .u64(1000)
                .unwrap_or_else(|error| panic!("batch number: {error:?}")),
            5 | 6 => encoder
                .u64(receipt.global_sequence())
                .unwrap_or_else(|error| panic!("sequence: {error:?}")),
            7 => encoder
                .bytes(&receipt.previous_state_root(), 32)
                .unwrap_or_else(|error| panic!("previous root: {error:?}")),
            8 => encoder
                .bytes(&receipt.resulting_state_root(), 32)
                .unwrap_or_else(|error| panic!("resulting root: {error:?}")),
            10 => encoder
                .bytes(&root, 32)
                .unwrap_or_else(|error| panic!("receipt root: {error:?}")),
            14 => encoder
                .u64(receipt.timestamp())
                .unwrap_or_else(|error| panic!("timestamp: {error:?}")),
            15 => encoder
                .bytes(&key_id, 32)
                .unwrap_or_else(|error| panic!("sequencer: {error:?}")),
            _ => encoder
                .bytes(&[1; 32], 32)
                .unwrap_or_else(|error| panic!("commitment: {error:?}")),
        }
    }
    let header = encoder.finish();
    let signature = key
        .sign(
            &batch_header_digest(&header)
                .unwrap_or_else(|error| panic!("header digest: {error:?}")),
        )
        .to_bytes();
    json!({
        "kind":"receipt", "activity_id":hex(&receipt.activity_id()),
        "canonical_value":hex(&canonical),
        "proof":{"leaf_index":proof.leaf_index(),"leaf_count":proof.leaf_count(),
            "siblings":proof.siblings().iter().map(|s| hex(s)).collect::<Vec<_>>()},
        "signed_header":{"canonical_header":hex(&header),"signature":hex(&signature)}
    })
}

struct Node {
    endpoint: String,
    state: Arc<Mutex<Value>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl Node {
    fn start(include_proof: bool) -> Self {
        let owner = derive_from_seed(&[0x41; 32], 0)
            .unwrap_or_else(|error| panic!("wallet owner: {error:?}"));
        let state = Arc::new(Mutex::new(json!({
            "owner":owner.evm_address_text(), "nonce":0, "include_proof":include_proof,
            "transactions":{}, "receipts":{}, "requests":[]
        })));
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("loopback node: {error:?}"));
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("bounded listener: {error:?}"));
        let endpoint = format!(
            "http://{}",
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("node address: {error:?}"))
        );
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let store = state.clone();
        let worker = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("node accept: {error}"),
                };
                let (reader, request) = Self::read_request(stream);
                let result = Self::response(
                    &mut store
                        .lock()
                        .unwrap_or_else(|error| panic!("node store: {error:?}")),
                    &request,
                );
                let body = serde_json::to_vec(
                    &json!({"jsonrpc":"2.0","id":request["id"],"result":result}),
                )
                .unwrap_or_else(|error| panic!("response JSON: {error:?}"));
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len())
                    .unwrap_or_else(|error| panic!("HTTP response: {error:?}"));
                stream
                    .write_all(&body)
                    .unwrap_or_else(|error| panic!("response bytes: {error:?}"));
            }
        });
        Self {
            endpoint,
            state,
            stop,
            worker: Some(worker),
        }
    }

    fn read_request(stream: std::net::TcpStream) -> (BufReader<std::net::TcpStream>, Value) {
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap_or_else(|error| panic!("read bound: {error:?}"));
        stream
            .set_write_timeout(Some(Duration::from_secs(5)))
            .unwrap_or_else(|error| panic!("write bound: {error:?}"));
        let mut reader = BufReader::new(stream);
        let mut line = String::new();
        reader
            .read_line(&mut line)
            .unwrap_or_else(|error| panic!("HTTP request: {error:?}"));
        assert_eq!(line, "POST / HTTP/1.1\r\n");
        let mut length = None;
        let mut header_bytes = line.len();
        loop {
            line.clear();
            reader
                .read_line(&mut line)
                .unwrap_or_else(|error| panic!("HTTP header: {error:?}"));
            header_bytes += line.len();
            assert!(header_bytes <= 8192);
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
        let length = length.unwrap_or_else(|| panic!("length-delimited request"));
        assert!(length <= 1_048_576 + 4096);
        let mut body = vec![0; length];
        reader
            .read_exact(&mut body)
            .unwrap_or_else(|error| panic!("request bytes: {error:?}"));
        let request: Value =
            serde_json::from_slice(&body).unwrap_or_else(|error| panic!("request JSON: {error:?}"));
        (reader, request)
    }

    fn submit_transaction(state: &mut Value, params: &Value) -> Value {
        let raw = unhex(
            params[0]
                .as_str()
                .unwrap_or_else(|| panic!("signed transaction")),
        );
        let (call, signature) = decode_signed(&raw)
            .unwrap_or_else(|error| panic!("production transaction decoder: {error:?}"));
        let sender = recover_sender(&call, &signature)
            .unwrap_or_else(|error| panic!("signature recovery: {error:?}"));
        assert_eq!(
            sender.as_slice(),
            unhex(
                state["owner"]
                    .as_str()
                    .unwrap_or_else(|| panic!("wallet owner"))
            )
        );
        assert_eq!(
            call.nonce,
            state["nonce"]
                .as_u64()
                .unwrap_or_else(|| panic!("stored nonce"))
        );
        assert_eq!(call.chain_id, 125);
        assert_eq!(call.to, call_target(0x101a));
        bind_call(&call, sender)
            .unwrap_or_else(|error| panic!("production disclosure verifier: {error:?}"));
        let hash = format!("0x{}", hex(&transaction_hash(&raw)));
        let seq = call.nonce + 1;
        let mut receipt = fixture_receipt(&raw, seq);
        if state["include_proof"] == true {
            receipt["inclusion"] = inclusion(&receipt);
        }
        state["nonce"] = json!(seq);
        state["transactions"][&hash] = params[0].clone();
        state["receipts"][&hash] = receipt;
        json!(hash)
    }

    fn response(state: &mut Value, request: &Value) -> Value {
        state["requests"]
            .as_array_mut()
            .unwrap_or_else(|| panic!("request journal"))
            .push(request.clone());
        let params = &request["params"];
        match request["method"]
            .as_str()
            .unwrap_or_else(|| panic!("RPC method"))
        {
            "eth_chainId" => json!("0x7d"),
            "eth_gasPrice" | "eth_maxPriorityFeePerGas" => json!("0x1"),
            "eth_estimateGas" => json!("0x186a0"),
            "eth_getTransactionCount" => {
                assert_eq!(
                    params
                        .as_array()
                        .unwrap_or_else(|| panic!("nonce params"))
                        .len(),
                    2
                );
                assert_eq!(
                    unhex(
                        params[0]
                            .as_str()
                            .unwrap_or_else(|| panic!("nonce address"))
                    ),
                    unhex(
                        state["owner"]
                            .as_str()
                            .unwrap_or_else(|| panic!("wallet owner"))
                    )
                );
                assert_eq!(params[1], "pending");
                json!(format!(
                    "0x{:x}",
                    state["nonce"].as_u64().unwrap_or_else(|| panic!("nonce"))
                ))
            }
            "px_getAccount" => json!({"account_id": params[0], "next_sequence":"7"}),
            "eth_sendRawTransaction" => Self::submit_transaction(state, params),
            "pax_sequencerReceipt" => state["receipts"][params[0]
                .as_str()
                .unwrap_or_else(|| panic!("activity hash"))]
            .clone(),
            "eth_getBlockByNumber" => {
                assert_eq!(params, &json!(["0x3e8", false]));
                json!({"number":state["committed"]["receiptJson"]["blockNumber"],
                    "hash":state["committed"]["receiptJson"]["blockHash"],
                    "transactions":state["committed"]["activityIds"]})
            }
            "px_getSequenceBatch" => {
                assert_eq!(params, &json!(["1000"]));
                state["committed"]["batch"].clone()
            }
            "px_getCommit" => {
                assert_eq!(params, &json!(["1000"]));
                json!({"height":"1000","signedHeader":state["committed"]["commit"]["signed_header"]})
            }
            "eth_getRawTransactionByHash" => state["transactions"][params[0]
                .as_str()
                .unwrap_or_else(|| panic!("activity hash"))]
            .clone(),
            method => panic!("unexpected RPC method {method}"),
        }
    }

    fn rpc(&self) -> RpcClient {
        RpcClient::connect_node(&self.endpoint, None)
            .unwrap_or_else(|error| panic!("direct node RPC: {error:?}"))
            .with_clock(Arc::new(OsClock::new()))
    }
    fn submitted(&self, hash: [u8; 32]) -> Vec<u8> {
        let body = serde_json::to_vec(&json!({"jsonrpc":"2.0","id":"raw",
            "method":"eth_getRawTransactionByHash","params":[format!("0x{}",hex(&hash))]}))
        .unwrap_or_else(|error| panic!("raw transaction request: {error:?}"));
        let mut response = ureq::post(&self.endpoint)
            .header("Content-Type", "application/json")
            .config()
            .timeout_global(Some(Duration::from_secs(5)))
            .build()
            .send(body.as_slice())
            .unwrap_or_else(|error| panic!("node raw transaction lookup: {error:?}"));
        let result: Value = serde_json::from_slice(
            &response
                .body_mut()
                .read_to_vec()
                .unwrap_or_else(|error| panic!("raw response: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("raw JSON: {error:?}"));
        assert_eq!(result["id"], "raw");
        unhex(
            result["result"]
                .as_str()
                .unwrap_or_else(|| panic!("submitted bytes")),
        )
    }
}
impl Drop for Node {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let result = worker.join();
            if !std::thread::panicking() {
                result.unwrap_or_else(|error| panic!("node worker: {error:?}"));
            }
        }
    }
}

fn policy() -> ReceiptPolicy {
    let key = SigningKey::from_bytes(&[0x5e; 32])
        .verifying_key()
        .to_bytes();
    ReceiptPolicy {
        protocol_version: 2,
        network_id: 125,
        sequencer: SequencerAuthorization::new(Sha256::digest(key).into(), key, 1, u64::MAX),
        trusted_checkpoint_context_digest: None,
    }
}
fn options() -> PaymentOptions {
    PaymentOptions {
        actor: derive_from_seed(&[0x41; 32], 0)
            .unwrap_or_else(|error| panic!("owner: {error:?}"))
            .did(),
        idempotency_key: [4; 32],
        fee_limit: 1_000_000,
        not_before: 0,
        not_after: u64::MAX,
        commitment: Commitment::Executed,
        wait_timeout: Duration::from_secs(5),
    }
}

// ReceiptCommitEvidence exposes these protocol types from the client's dependency.
use layerx_client::rpc::receipt_commit::SequenceBatch;
use layerx_programs_registry::chain_proof::{
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    merkle::SimpleProof,
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, PartSetHeader, Timestamp},
    ChainProofVerifier,
};

fn base64(bytes: &[u8]) -> String {
    const ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut encoded = String::new();
    for chunk in bytes.chunks(3) {
        let word = u32::from(chunk[0]) << 16
            | u32::from(*chunk.get(1).unwrap_or(&0)) << 8
            | u32::from(*chunk.get(2).unwrap_or(&0));
        for index in 0..4 {
            encoded.push(if index > chunk.len() {
                '='
            } else {
                char::from(
                    ALPHABET[usize::try_from((word >> (18 - 6 * index)) & 63).unwrap_or_else(
                        |error| panic!("fixture integer must fit usize: {error:?}"),
                    )],
                )
            });
        }
    }
    encoded
}

struct CommitFixture {
    verifier: ChainProofVerifier,
    block_hash: [u8; 32],
}
impl CommitFixture {
    fn signed_header(
        verifier: &ChainProofVerifier,
        header: &ChainHeader,
        block_hash: [u8; 32],
        validator: &TrustedValidator,
        validator_key: &SigningKey,
    ) -> Value {
        let block_id = BlockId {
            hash: block_hash.to_vec(),
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![0x33; 32],
            },
        };
        let signature = validator_key
            .sign(
                &vote_sign_bytes(
                    &verifier.chain_id,
                    1000,
                    0,
                    Some(&block_id),
                    Timestamp {
                        seconds: 1000,
                        nanos: 0,
                    },
                )
                .unwrap_or_else(|error| panic!("production Comet vote encoder: {error:?}")),
            )
            .to_bytes();
        let id_json = json!({"hash":hex(&block_hash),
            "parts":{"total":1,"hash":hex(&[0x33; 32])}});
        json!({
            "header":{
                "version":{"block":"11","app":"1"}, "chain_id":verifier.chain_id,
                "height":"1000", "time":"1970-01-01T00:16:40Z",
                "last_block_id":{"hash":hex(&header.last_block_id.hash),
                    "parts":{"total":1,"hash":hex(&header.last_block_id.part_set_hash)}},
                "last_commit_hash":hex(&header.last_commit_hash), "data_hash":hex(&header.data_hash),
                "validators_hash":hex(&header.validators_hash),
                "next_validators_hash":hex(&header.next_validators_hash),
                "consensus_hash":hex(&header.consensus_hash), "app_hash":hex(&header.app_hash),
                "last_results_hash":hex(&header.last_results_hash), "evidence_hash":hex(&header.evidence_hash),
                "proposer_address":hex(&header.proposer_address)
            },
            "commit":{"height":"1000","round":0,"block_id":id_json,
                "signatures":[{"block_id_flag":2,"validator_address":hex(&validator.address),
                    "timestamp":"1970-01-01T00:16:40Z","signature":base64(&signature)}]}
        })
    }

    fn header(
        verifier: &ChainProofVerifier,
        validator: &TrustedValidator,
        leaf_hash: [u8; 32],
        resulting_root: [u8; 32],
    ) -> ChainHeader {
        ChainHeader {
            version: HeaderVersion { block: 11, app: 1 },
            chain_id: verifier.chain_id.clone(),
            height: 1000,
            time: HeaderTime {
                seconds: 1000,
                nanos: 0,
            },
            last_block_id: HeaderBlockId {
                hash: vec![0x33; 32],
                part_set_total: 1,
                part_set_hash: vec![0x33; 32],
            },
            last_commit_hash: vec![0x33; 32],
            data_hash: leaf_hash.to_vec(),
            validators_hash: verifier.validators.hash().to_vec(),
            next_validators_hash: verifier.validators.hash().to_vec(),
            consensus_hash: vec![0x33; 32],
            app_hash: resulting_root.to_vec(),
            last_results_hash: vec![0x33; 32],
            evidence_hash: vec![0x33; 32],
            proposer_address: validator.address.to_vec(),
        }
    }

    fn publish(node: &Node, receipt: &VerifiedRpcReceipt) -> Self {
        let protocol = receipt
            .receipt()
            .protocol()
            .unwrap_or_else(|| panic!("protocol receipt"));
        let activity = protocol.activity_id();
        let receipt_key = SigningKey::from_bytes(&[0x5e; 32]);
        let validator_key = SigningKey::from_bytes(&[0x72; 32]);
        let validator = TrustedValidator::new(validator_key.verifying_key().to_bytes(), 10);
        let verifier = ChainProofVerifier {
            chain_id: "submitted-wait".into(),
            validators: ValidatorSet::new(vec![validator.clone()])
                .unwrap_or_else(|error| panic!("pinned validator set: {error:?}")),
        };
        let mut batch = SequenceBatch {
            height: 1000,
            from_sequence: protocol.global_sequence(),
            to_sequence: protocol.global_sequence(),
            previous_root: protocol.previous_state_root(),
            resulting_root: protocol.resulting_state_root(),
            activity_root: Sha256::digest(activity).into(),
            key_id: Sha256::digest(receipt_key.verifying_key().as_bytes()).into(),
            signature: [0; 64],
        };
        batch.signature = receipt_key
            .sign(
                &batch
                    .signing_digest(&verifier.chain_id)
                    .unwrap_or_else(|error| panic!("batch digest: {error:?}")),
            )
            .to_bytes();
        let batch_transaction = batch
            .encode()
            .unwrap_or_else(|error| panic!("production sequence-batch encoder: {error:?}"));
        let leaf_hash: [u8; 32] = Sha256::new()
            .chain_update([0])
            .chain_update(Sha256::digest(&batch_transaction))
            .finalize()
            .into();
        let proof = SimpleProof {
            total: 1,
            index: 0,
            leaf_hash,
            aunts: vec![],
        };
        let header = Self::header(
            &verifier,
            &validator,
            leaf_hash,
            protocol.resulting_state_root(),
        );
        let block_hash = header
            .hash()
            .unwrap_or_else(|error| panic!("production Comet header hash: {error:?}"));
        let signed =
            Self::signed_header(&verifier, &header, block_hash, &validator, &validator_key);
        let mut state = node
            .state
            .lock()
            .unwrap_or_else(|error| panic!("node store: {error:?}"));
        let summary = &mut state["receipts"][format!("0x{}", hex(&activity))];
        summary["rung"] = json!("committed");
        summary["blockNumber"] = json!("0x3e8");
        summary["blockHash"] = json!(format!("0x{}", hex(&block_hash)));
        state["committed"] = json!({"receiptJson":summary.clone(),
            "activityIds":[format!("0x{}", hex(&activity))],
            "commit":{"signed_header":signed},
            "batch":{"height":"1000", "activityIds":[format!("0x{}", hex(&activity))], "tx":format!("0x{}", hex(&batch_transaction)),
                "proof":{"total":proof.total, "index":proof.index,
                    "leafHash":format!("0x{}", hex(&proof.leaf_hash)),
                    "aunts":proof.aunts.iter().map(|aunt| format!("0x{}", hex(aunt))).collect::<Vec<_>>()}}});
        Self {
            verifier,
            block_hash,
        }
    }

    fn authority(&self) -> NodeCommitAuthority {
        NodeCommitAuthority::new(
            self.verifier.chain_id.clone(),
            self.verifier
                .validators
                .validators()
                .iter()
                .map(|validator| (validator.public_key, validator.voting_power))
                .collect(),
            1000,
            1000,
        )
        .unwrap_or_else(|error| panic!("pinned node commit authority: {error:?}"))
    }
}

fn wallet<'a>(
    rpc: &'a RpcClient,
    signer: &'a LocalSigner,
    policy: &'a ReceiptPolicy,
) -> Wallet<'a> {
    Wallet {
        rpc,
        signer,
        policy,
        native_asset: [3; 32],
    }
}

fn submit_payment(wallet: &Wallet<'_>) -> [u8; 32] {
    let options = options();
    let payment = Payment::OpenAccount { asset: [3; 32] };
    let (module, ordinal) = payment.activity_type();
    let payload = payment
        .encode(options.actor.as_bytes())
        .unwrap_or_else(|error| panic!("canonical payment payload: {error:?}"));
    let prepared = wallet
        .prepare_payload(module, ordinal, &payload, &options)
        .unwrap_or_else(|error| panic!("prepared payment: {error:?}"));
    run(wallet.execute(prepared, &options))
        .unwrap_or_else(|error| panic!("submitted signed payment: {error:?}"))
}

fn assert_activity(receipt: &VerifiedRpcReceipt, activity: [u8; 32], canonical: &[u8]) {
    assert_eq!(
        receipt
            .receipt()
            .protocol()
            .unwrap_or_else(|| panic!("protocol receipt"))
            .activity_id(),
        activity
    );
    assert_eq!(receipt.canonical_activity(), Some(canonical));
    assert_eq!(transaction_hash(canonical), activity);
}

fn assert_inclusion(receipt: &VerifiedRpcReceipt, policy: &ReceiptPolicy) {
    let evidence = receipt
        .batch_evidence()
        .unwrap_or_else(|| panic!("verified batch evidence retained"));
    verify_receipt(
        receipt.canonical_bytes(),
        evidence.proof(),
        evidence.canonical_header(),
        &evidence.header_signature(),
        &policy.sequencer,
    )
    .unwrap_or_else(|error| panic!("retained inclusion re-verifies: {error:?}"));
}

#[test]
fn bind_canonical_activity() {
    let node = Node::start(true);
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0).unwrap_or_else(|error| panic!("owner: {error:?}"));
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = wallet(&rpc, &signer, &policy);
    let activity = submit_payment(&wallet);
    let raw = node.submitted(activity);
    assert_eq!(transaction_hash(&raw), activity);
    let executed = wallet
        .wait_for_signed(&raw, Commitment::Executed, Duration::from_secs(5))
        .unwrap_or_else(|error| panic!("signed payment waited to Executed: {error:?}"));
    assert_eq!(executed.commitment(), Commitment::Executed);
    assert_eq!(executed.rung(), Rung::Receipted);
    assert_eq!(executed.committed_block(), None);
    assert_activity(&executed, activity, &raw);
    assert_inclusion(&executed, &policy);

    let commit = CommitFixture::publish(&node, &executed);
    let committed_rpc = node
        .rpc()
        .with_node_commit_authority(commit.authority())
        .unwrap_or_else(|error| panic!("node commit trust: {error:?}"));
    let committed_wallet = self::wallet(&committed_rpc, &signer, &policy);
    let batched = committed_wallet
        .wait_for_submitted(&executed, Commitment::Batched, Duration::from_secs(5))
        .unwrap_or_else(|error| panic!("Batched submitted receipt: {error:?}"));
    assert_eq!(batched.commitment(), Commitment::Batched);
    assert_eq!(batched.rung(), Rung::Committed);
    assert_eq!(batched.committed_block(), Some((1000, commit.block_hash)));
    assert_activity(&batched, activity, &raw);
    assert_inclusion(&batched, &policy);
    let original = executed
        .batch_evidence()
        .unwrap_or_else(|| panic!("Executed batch evidence"));
    let retained = batched
        .batch_evidence()
        .unwrap_or_else(|| panic!("Batched batch evidence"));
    assert_eq!(retained.canonical_header(), original.canonical_header());
    assert_eq!(retained.header_signature(), original.header_signature());
    assert_eq!(retained.proof(), original.proof());
    assert_activity(&executed, activity, &raw);
    assert_eq!(executed.commitment(), Commitment::Executed);
    assert_eq!(
        node.state
            .lock()
            .unwrap_or_else(|error| panic!("node store: {error:?}"))["requests"]
            .as_array()
            .unwrap_or_else(|| panic!("requests"))
            .iter()
            .filter(|request| request["method"] == "eth_sendRawTransaction")
            .count(),
        1
    );
}

#[test]
fn empty_signed_bytes_are_refused_before_rpc() {
    let node = Node::start(true);
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0).unwrap_or_else(|error| panic!("owner: {error:?}"));
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = wallet(&rpc, &signer, &policy);
    for commitment in [
        Commitment::Executed,
        Commitment::Batched,
        Commitment::Finalised,
    ] {
        assert!(matches!(
            wallet.wait_for_signed(&[], commitment, Duration::from_secs(5)),
            Err(RpcError::InvalidRequest)
        ));
    }
    assert!(node
        .state
        .lock()
        .unwrap_or_else(|error| panic!("node store: {error:?}"))["requests"]
        .as_array()
        .unwrap_or_else(|| panic!("requests"))
        .is_empty());
}

#[test]
fn hash_only_wait_has_no_canonical_activity() {
    let node = Node::start(true);
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0).unwrap_or_else(|error| panic!("owner: {error:?}"));
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = wallet(&rpc, &signer, &policy);
    let activity = submit_payment(&wallet);
    let independent = wallet
        .wait_for(activity, Commitment::Executed, Duration::from_secs(5))
        .unwrap_or_else(|error| panic!("hash-only wait: {error:?}"));
    assert_eq!(independent.commitment(), Commitment::Executed);
    assert_eq!(
        independent
            .receipt()
            .protocol()
            .unwrap_or_else(|| panic!("protocol"))
            .activity_id(),
        activity
    );
    assert_eq!(independent.canonical_activity(), None);
    assert!(matches!(
        wallet.wait_for_submitted(&independent, Commitment::Batched, Duration::from_secs(5)),
        Err(RpcError::InvalidRequest)
    ));
    let raw = node.submitted(activity);
    let bound = wallet
        .wait_for_signed(&raw, Commitment::Executed, Duration::from_secs(5))
        .unwrap_or_else(|error| panic!("signed wait for the same activity: {error:?}"));
    assert_activity(&bound, activity, &raw);
    assert_eq!(bound.canonical_bytes(), independent.canonical_bytes());
    assert_eq!(independent.canonical_activity(), None);
}
