//! Decision 2539 / gotcha 3026: signed wallet bytes and verified inclusion evidence.
//! Uses the existing SDK audit peer's wire producer and OS clock over loopback HTTP.
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_crypto::payments::Payment;
use layerx_crypto::send::{send_context_hash, SendDebit};
use layerx_crypto::signer::{LocalSigner, Signer};
use layerx_proof::inclusion::{verify_receipt, SequencerAuthorization};
use layerx_proof::merkle::build_proof;
use layerx_sdk::rpc::{Commitment, RpcClient, RpcError};
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
pub(crate) fn run<F: Future>(future: F) -> F::Output {
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

pub(crate) struct Node {
    endpoint: String,
    pub(crate) state: Arc<Mutex<Value>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl Node {
    pub(crate) fn start(include_proof: bool) -> Self {
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
                let response = {
                    let mut state = store
                        .lock()
                        .unwrap_or_else(|error| panic!("node store: {error:?}"));
                    let result = Self::response(&mut state, &request);
                    if request["method"] == "px_getSequenceBatch" && !state["batch_error"].is_null()
                    {
                        json!({"jsonrpc":"2.0","id":request["id"],"error":state["batch_error"]})
                    } else {
                        json!({"jsonrpc":"2.0","id":request["id"],"result":result})
                    }
                };
                let body = serde_json::to_vec(&response)
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
                let height = u64::from_str_radix(
                    params[0]
                        .as_str()
                        .unwrap_or_else(|| panic!("block height"))
                        .strip_prefix("0x")
                        .unwrap_or_else(|| panic!("hex block height")),
                    16,
                )
                .unwrap_or_else(|error| panic!("height: {error:?}"));
                assert_eq!(params, &json!([format!("0x{height:x}"), false]));
                let record = if state["committed_by_height"].is_object() {
                    &state["committed_by_height"][height.to_string()]
                } else {
                    &state["committed"]
                };
                assert_eq!(params[0], record["receiptJson"]["blockNumber"]);
                json!({"number":record["receiptJson"]["blockNumber"],
                    "hash":record["receiptJson"]["blockHash"],
                    "transactions":record["activityIds"]})
            }
            "px_getCommit" | "px_getSequenceBatch" => {
                let height = params[0]
                    .as_str()
                    .unwrap_or_else(|| panic!("decimal height"));
                assert_eq!(
                    height
                        .parse::<u64>()
                        .unwrap_or_else(|error| panic!("height: {error:?}"))
                        .to_string(),
                    height
                );
                assert_eq!(
                    params.as_array().unwrap_or_else(|| panic!("params")).len(),
                    1
                );
                let record = if state["committed_by_height"].is_object() {
                    &state["committed_by_height"][height]
                } else {
                    &state["committed"]
                };
                if request["method"] == "px_getCommit" {
                    json!({"height":height,"signedHeader":record["commit"]["signed_header"]})
                } else {
                    record["batch"].clone()
                }
            }
            "eth_getRawTransactionByHash" => state["transactions"][params[0]
                .as_str()
                .unwrap_or_else(|| panic!("activity hash"))]
            .clone(),
            method => panic!("unexpected RPC method {method}"),
        }
    }

    pub(crate) fn rpc(&self) -> RpcClient {
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

pub(crate) fn policy() -> ReceiptPolicy {
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
pub(crate) fn options() -> PaymentOptions {
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
fn debit() -> SendDebit {
    SendDebit {
        from: [1; 32],
        to: [2; 32],
        asset: [3; 32],
        amount: 23,
        source_sequence: 7,
        idempotency_key: [4; 32],
        expires_at: u64::MAX,
        context_hash: send_context_hash(&[1; 32], &[2; 32], &[3; 32], 23, &[4; 32]),
        conditions: vec![],
        authorization_kind: 1,
        network_id: 125,
        protocol_version: 2,
    }
}
fn assert_bound(node: &Node, receipt: &VerifiedRpcReceipt, signer: &LocalSigner) {
    let hash = receipt
        .receipt()
        .protocol()
        .unwrap_or_else(|| panic!("protocol"))
        .activity_id();
    let raw = node.submitted(hash);
    assert_eq!(receipt.canonical_activity(), Some(raw.as_slice()));
    assert_eq!(
        transaction_hash(
            receipt
                .canonical_activity()
                .unwrap_or_else(|| panic!("bound transaction"))
        ),
        hash
    );
    let (call, signature) = decode_signed(&raw)
        .unwrap_or_else(|error| panic!("submitted transaction decode: {error:?}"));
    assert_eq!(
        recover_sender(&call, &signature)
            .unwrap_or_else(|error| panic!("recovered sender: {error:?}")),
        signer
            .evm_address()
            .unwrap_or_else(|| panic!("signer address"))
    );
    assert_eq!(call.to, call_target(0x101a));
    assert_eq!(receipt.rung(), Rung::Receipted);
    assert_eq!(receipt.committed_block(), None);
}

#[test]
fn wallet_send_retains_the_exact_submitted_pax_transaction() {
    let node = Node::start(true);
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0)
        .unwrap_or_else(|error| panic!("owner derivation: {error:?}"));
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let debit = debit();
    let payload = run(debit.sign(&signer))
        .unwrap_or_else(|error| panic!("real native Send authorization: {error:?}"));
    let receipt = run(wallet.send(&payload, &options()))
        .unwrap_or_else(|error| panic!("verified wallet Send: {error:?}"));
    assert_bound(&node, &receipt, &signer);
    let raw = node.submitted(
        receipt
            .receipt()
            .protocol()
            .unwrap_or_else(|| panic!("protocol"))
            .activity_id(),
    );
    let (call, _) = decode_signed(&raw).unwrap_or_else(|error| panic!("transaction: {error:?}"));
    assert_eq!(
        bind_call(&call, owner.evm_address())
            .unwrap_or_else(|error| panic!("disclosure: {error:?}"))
            .construct_method()
            .unwrap_or_else(|error| panic!("method: {error:?}")),
        Some(ConstructMethod::LedgerSend { payload })
    );
}

#[test]
fn inclusion_material_is_retained_exactly_and_independent_lookup_has_no_transaction() {
    let node = Node::start(true);
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0).unwrap_or_else(|error| panic!("owner: {error:?}"));
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let receipt = run(wallet.send_debit(&debit(), &options()))
        .unwrap_or_else(|error| panic!("verified SendDebit: {error:?}"));
    assert_bound(&node, &receipt, &signer);
    let hash = receipt
        .receipt()
        .protocol()
        .unwrap_or_else(|| panic!("protocol"))
        .activity_id();
    let material = node
        .state
        .lock()
        .unwrap_or_else(|error| panic!("node store: {error:?}"))["receipts"]
        [format!("0x{}", hex(&hash))]["inclusion"]
        .clone();
    let evidence = receipt
        .batch_evidence()
        .unwrap_or_else(|| panic!("retained inclusion material"));
    assert_eq!(
        evidence.canonical_header(),
        unhex(
            material["signed_header"]["canonical_header"]
                .as_str()
                .unwrap_or_else(|| panic!("header"))
        )
    );
    assert_eq!(
        evidence.header_signature().as_slice(),
        unhex(
            material["signed_header"]["signature"]
                .as_str()
                .unwrap_or_else(|| panic!("signature"))
        )
    );
    assert_eq!(evidence.proof().leaf_index(), 1);
    assert_eq!(evidence.proof().leaf_count(), 3);
    assert_eq!(
        evidence
            .proof()
            .siblings()
            .iter()
            .map(|s| hex(s))
            .collect::<Vec<_>>(),
        material["proof"]["siblings"]
            .as_array()
            .unwrap_or_else(|| panic!("siblings"))
            .iter()
            .map(|s| s.as_str().unwrap_or_else(|| panic!("sibling")).to_owned())
            .collect::<Vec<_>>()
    );
    verify_receipt(
        receipt.canonical_bytes(),
        evidence.proof(),
        evidence.canonical_header(),
        &evidence.header_signature(),
        &policy.sequencer,
    )
    .unwrap_or_else(|error| panic!("retained evidence independently verifies: {error:?}"));
    let independent = rpc
        .wait_for(hash, Commitment::Executed, &policy, Duration::from_secs(5))
        .unwrap_or_else(|error| panic!("independent lookup: {error:?}"));
    assert_eq!(independent.canonical_activity(), None);
    assert_eq!(independent.canonical_bytes(), receipt.canonical_bytes());
    assert_eq!(
        independent
            .batch_evidence()
            .unwrap_or_else(|| panic!("independent verified inclusion"))
            .proof(),
        evidence.proof()
    );
    node.state
        .lock()
        .unwrap_or_else(|error| panic!("node store: {error:?}"))["receipts"]
        [format!("0x{}", hex(&hash))]
    .as_object_mut()
    .unwrap_or_else(|| panic!("summary"))
    .remove("inclusion");
    let summary = rpc
        .wait_for(hash, Commitment::Executed, &policy, Duration::from_secs(5))
        .unwrap_or_else(|error| panic!("signed summary without inclusion: {error:?}"));
    assert_eq!(summary.canonical_activity(), None);
    assert!(summary.batch_evidence().is_none());
    assert_eq!(summary.committed_block(), None);
}

#[test]
fn payment_and_issue_bind_the_submitted_bytes_without_inventing_inclusion() {
    let node = Node::start(false);
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0).unwrap_or_else(|error| panic!("owner: {error:?}"));
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let options = options();
    let opened = run(wallet.open_account([3; 32], &options))
        .unwrap_or_else(|error| panic!("wallet open account: {error:?}"));
    assert_bound(&node, &opened, &signer);
    assert!(opened.batch_evidence().is_none());
    let minted = run(wallet.mint([4; 32], [2; 32], 10, &options))
        .unwrap_or_else(|error| panic!("wallet mint: {error:?}"));
    assert_bound(&node, &minted, &signer);
    let burned = run(wallet.burn([4; 32], [2; 32], 1, &options))
        .unwrap_or_else(|error| panic!("wallet burn: {error:?}"));
    assert_bound(&node, &burned, &signer);
    let payload = unhex(include_str!("../../layerx-crypto/tests/fixtures/payments/1-7.hex").trim());
    let Payment::IssueGrant(grant) = Payment::decode(
        layerx_types::payload::ModuleId::Asset,
        7,
        &payload,
        options.actor.as_bytes(),
    )
    .unwrap_or_else(|error| panic!("canonical signed grant producer fixture: {error:?}")) else {
        panic!("expected signed grant");
    };
    let issued = run(wallet.issue(grant, &options))
        .unwrap_or_else(|error| panic!("wallet grant issue: {error:?}"));
    assert_bound(&node, &issued, &signer);
    assert!(issued.batch_evidence().is_none());
    let revoked = run(wallet.revoke([7; 32], 1, &options))
        .unwrap_or_else(|error| panic!("wallet revoke: {error:?}"));
    assert_bound(&node, &revoked, &signer);
}

#[test]
fn invalid_inclusion_material_is_refused_without_dropping_to_summary_verification() {
    let node = Node::start(true);
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0).unwrap_or_else(|error| panic!("owner: {error:?}"));
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let receipt = run(wallet.open_account([3; 32], &options()))
        .unwrap_or_else(|error| panic!("verified wallet payment: {error:?}"));
    let hash = receipt
        .receipt()
        .protocol()
        .unwrap_or_else(|| panic!("protocol"))
        .activity_id();
    let id = format!("0x{}", hex(&hash));
    let original = node
        .state
        .lock()
        .unwrap_or_else(|error| panic!("node store: {error:?}"))["receipts"][&id]
        .clone();
    let mut corruptions = Vec::new();
    for (path, replacement) in [
        ("/inclusion/signed_header/signature", json!("00".repeat(64))),
        ("/inclusion/proof/siblings/0", json!("00".repeat(32))),
        ("/inclusion/activity_id", json!("00".repeat(32))),
        ("/inclusion/canonical_value", json!("00")),
        ("/inclusion/proof/leaf_index", json!(0)),
    ] {
        let mut changed = original.clone();
        *changed
            .pointer_mut(path)
            .unwrap_or_else(|| panic!("inclusion field")) = replacement;
        corruptions.push(changed);
    }
    let mut changed = original.clone();
    changed["inclusion"] = inclusion_on_network(&original, 126);
    corruptions.push(changed);
    let mut changed = original.clone();
    changed["inclusion"] = Value::Null;
    corruptions.push(changed);
    for changed in corruptions {
        node.state
            .lock()
            .unwrap_or_else(|error| panic!("node store: {error:?}"))["receipts"][&id] = changed;
        assert!(matches!(
            rpc.wait_for(hash, Commitment::Executed, &policy, Duration::from_secs(5)),
            Err(RpcError::Verification | RpcError::InvalidResponse)
        ));
    }
}

#[test]
fn committed_lookup_keeps_its_verified_block_and_does_not_invent_batch_evidence() {
    use layerx_sdk::rpc_verification::node_commit::NodeCommitAuthority;
    let node = Node::start(false);
    let record: Value = serde_json::from_str(include_str!(
        "../src/repair_22_31_227_fixture/committed.json"
    ))
    .unwrap_or_else(|error| panic!("existing real node and Comet transcript: {error:?}"));
    let hash_text = record["receiptJson"]["txHash"]
        .as_str()
        .unwrap_or_else(|| panic!("hash"));
    let hash: [u8; 32] = unhex(hash_text)
        .try_into()
        .unwrap_or_else(|error| panic!("hash width: {error:?}"));
    {
        let mut state = node
            .state
            .lock()
            .unwrap_or_else(|error| panic!("node store: {error:?}"));
        state["receipts"][hash_text] = record["receiptJson"].clone();
        state["committed"] = record.clone();
        state["committed"]["activityIds"] = json!([hash_text]);
        state["committed"]["batch"]["activityIds"] = json!([hash_text]);
    }
    let authority = NodeCommitAuthority::new(
        "committed-fixture".to_owned(),
        vec![(
            unhex("c614a1c91884c3c13bed586a16d51f7253072f7cc41018ea28dd864e4e548e4d")
                .try_into()
                .unwrap_or_else(|error| panic!("independently pinned validator: {error:?}")),
            7_000_000_000,
        )],
        10,
        10,
    )
    .unwrap_or_else(|error| panic!("pinned Comet authority: {error:?}"));
    let rpc = node
        .rpc()
        .with_node_commit_authority(authority)
        .unwrap_or_else(|error| panic!("committed coverage: {error:?}"));
    let policy = ReceiptPolicy {
        protocol_version: 2,
        network_id: 125,
        sequencer: SequencerAuthorization::new(
            unhex(
                record["pinnedKey"]["keyId"]
                    .as_str()
                    .unwrap_or_else(|| panic!("key id")),
            )
            .try_into()
            .unwrap_or_else(|error| panic!("key id width: {error:?}")),
            unhex(
                record["pinnedKey"]["publicKey"]
                    .as_str()
                    .unwrap_or_else(|| panic!("receipt key")),
            )
            .try_into()
            .unwrap_or_else(|error| panic!("key width: {error:?}")),
            10,
            10,
        ),
        trusted_checkpoint_context_digest: None,
    };
    let receipt = rpc
        .wait_for(hash, Commitment::Committed, &policy, Duration::from_secs(5))
        .unwrap_or_else(|error| panic!("verified committed receipt: {error:?}"));
    let block_hash = unhex(
        record["receiptJson"]["blockHash"]
            .as_str()
            .unwrap_or_else(|| panic!("block hash")),
    )
    .try_into()
    .unwrap_or_else(|error| panic!("block hash width: {error:?}"));
    assert_eq!(receipt.committed_block(), Some((10, block_hash)));
    assert_eq!(receipt.rung(), Rung::Committed);
    assert_eq!(receipt.canonical_activity(), None);
    assert!(receipt.batch_evidence().is_none());
    assert_eq!(
        receipt.canonical_bytes(),
        unhex(
            record["receiptJson"]["receipt"]
                .as_str()
                .unwrap_or_else(|| panic!("canonical receipt"))
        )
    );
}
