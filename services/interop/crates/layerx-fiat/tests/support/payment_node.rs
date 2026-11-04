use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::sync::{Arc, Mutex};
use std::thread::JoinHandle;
use std::time::Duration;

use ed25519_dalek::{Signature, Signer as _, SigningKey};
use layerx_fiat::FiatIntent;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

use super::{array, bytes};

type Result<T> = std::result::Result<T, Box<dyn std::error::Error>>;

// This peer signs a fixture transport envelope, not an Ethereum transaction.
// A chain-125 capture and the service's EVM signer remain explicit wiring requests.
const TRANSACTION_SEED: [u8; 32] = [0x43; 32];
const RECEIPT_SEED: [u8; 32] = [0x59; 32];
const KEY_ID: [u8; 32] = [0x69; 32];
pub const PAYER: [u8; 32] = [0x27; 32];
pub const EXPIRES_AT: u64 = 1_800_000_000;
const HEIGHT: u64 = 1001;
const DOMAIN: &[u8] = b"LayerX/fiat/loopback-payment/v1\0";
const LEDGER: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a,
];
const PAYMENT_EXACT: [u8; 32] = [
    0xc4, 0xda, 0x5d, 0x6f, 0x75, 0x7c, 0x25, 0x1c, 0xd7, 0x09, 0x47, 0xb7, 0x52, 0x91, 0x7e, 0x56,
    0x71, 0xb6, 0x77, 0x51, 0x7e, 0x3d, 0x95, 0x3f, 0x36, 0x2a, 0x1d, 0xf6, 0x64, 0x8e, 0xf5, 0xb1,
];

pub fn calldata_offset() -> usize {
    DOMAIN.len()
}

pub fn receipt_keys() -> [layerx_proof::inclusion::SequencerAuthorization; 1] {
    [layerx_proof::inclusion::SequencerAuthorization::new(
        KEY_ID,
        SigningKey::from_bytes(&RECEIPT_SEED)
            .verifying_key()
            .to_bytes(),
        HEIGHT,
        HEIGHT,
    )]
}

pub fn hex(data: &[u8]) -> String {
    format!(
        "0x{}",
        data.iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    )
}

fn blob(out: &mut Vec<u8>, data: &[u8]) {
    out.extend_from_slice(&u32::try_from(data.len()).unwrap().to_be_bytes());
    out.extend_from_slice(data);
}

fn effect(out: &mut Vec<u8>, ordinal: u16, kind: u8, body: &[u8]) {
    out.extend_from_slice(&12_u16.to_be_bytes());
    out.extend_from_slice(&ordinal.to_be_bytes());
    out.extend_from_slice(&0_u16.to_be_bytes());
    out.push(kind);
    out.push(0);
    blob(out, &[0; 32]);
    blob(out, body);
}

// Encode a new receipt from the call the peer received. No source receipt is
// loaded, patched, or re-signed; production decoding/signature/log checks below
// validate this peer's protocol output before it reaches the adapter.
fn produce(raw: &[u8]) -> Result<Value> {
    let prefix = DOMAIN.len() + 196;
    if raw.len() != prefix + 64 || !raw.starts_with(DOMAIN) {
        return Err("invalid signed fixture payment envelope".into());
    }
    let signer = SigningKey::from_bytes(&TRANSACTION_SEED);
    signer
        .verifying_key()
        .verify_strict(&raw[..prefix], &Signature::from_slice(&raw[prefix..])?)?;
    let call = &raw[DOMAIN.len()..prefix];
    if call[..4] != [0x78, 0xea, 0x11, 0xdb]
        || call[100..116] != [0; 16]
        || call[132..156] != [0; 24]
    {
        return Err("invalid payExact calldata".into());
    }
    let hash: [u8; 32] = Sha256::digest(raw).into();
    let from = &call[4..36];
    let to = &call[36..68];
    let asset = &call[68..100];
    let amount = &call[116..132];
    let reference = &call[164..196];
    let mut data = asset.to_vec();
    data.extend_from_slice(&[0; 16]);
    data.extend_from_slice(amount);
    let topics = [PAYMENT_EXACT.as_slice(), reference, from, to];
    let mut log_body = LEDGER.to_vec();
    log_body.push(4);
    for topic in topics {
        log_body.extend_from_slice(topic);
    }
    log_body.extend_from_slice(&Sha256::digest(&data));

    let mut summary = Vec::new();
    for gas in [21_000_u64, 21_000, 1] {
        summary.extend_from_slice(&gas.to_be_bytes());
    }
    summary.extend_from_slice(&0_u32.to_be_bytes());
    summary.extend_from_slice(&[0; 40]);
    summary.extend_from_slice(&Sha256::digest([]));
    summary.extend_from_slice(&0_u32.to_be_bytes());
    summary.extend_from_slice(&1_u32.to_be_bytes());
    summary.push(4);
    summary.extend_from_slice(b"upax");

    let mut receipt = Vec::new();
    receipt.extend_from_slice(&[0, 2, 0x52, 1, 0, 2]);
    blob(&mut receipt, &hash);
    receipt.extend_from_slice(&1_u64.to_be_bytes());
    for root in [[0x11; 32], [0x22; 32], hash] {
        blob(&mut receipt, &root);
    }
    receipt.extend_from_slice(&0_i32.to_be_bytes());
    receipt.extend_from_slice(&2_u32.to_be_bytes());
    effect(&mut receipt, 0, 1, &summary);
    effect(&mut receipt, 1, 3, &log_body);
    receipt.extend_from_slice(&21_000_u128.to_be_bytes());
    let mut batch = [0; 32];
    batch[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    blob(&mut receipt, &batch);
    receipt.extend_from_slice(&12_u16.to_be_bytes());
    receipt.extend_from_slice(&1_u32.to_be_bytes());
    receipt.extend_from_slice(&1_u32.to_be_bytes());
    receipt.push(0);
    blob(&mut receipt, &[0; 32]);
    receipt.extend_from_slice(&0_u128.to_be_bytes());
    blob(&mut receipt, &[0; 32]);
    for _ in 0..2 {
        receipt.extend_from_slice(&0_u128.to_be_bytes());
    }
    receipt.extend_from_slice(&0_u64.to_be_bytes());
    blob(&mut receipt, &[0; 32]);
    for _ in 0..2 {
        receipt.extend_from_slice(&0_u128.to_be_bytes());
    }
    blob(&mut receipt, &[0; 32]);
    blob(&mut receipt, &[0; 32]);
    blob(&mut receipt, &KEY_ID);
    receipt.extend_from_slice(&10_u64.to_be_bytes());
    let mut unsigned = receipt.clone();
    unsigned.push(0);
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    digest.update(unsigned);
    let digest: [u8; 32] = digest.finalize().into();
    let key = SigningKey::from_bytes(&RECEIPT_SEED);
    receipt.push(1);
    blob(&mut receipt, &key.sign(&digest).to_bytes());
    Ok(json!({
        "id": "fiat-loopback-pay-exact",
        "provenance": {"format": "signed-fixture-payment-v1", "producer": "FiatIntent::payment_calldata"},
        "rawTransaction": hex(raw),
        "signedReceipt": hex(&receipt),
        "receiptDigest": hex(&digest),
        "pinnedKey": {"keyId": hex(&KEY_ID), "publicKey": hex(&key.verifying_key().to_bytes()), "firstHeight": HEIGHT, "lastHeight": HEIGHT},
        "receiptJson": {"txHash": hex(&hash), "receipt": hex(&receipt), "status": "0x1", "rung": "receipted", "logs": [{"address": hex(&LEDGER), "topics": topics.map(hex), "data": hex(&data), "logIndex": "0x0"}]}
    }))
}

#[derive(Default)]
struct State {
    case: Option<Value>,
    submissions: usize,
    ready: bool,
}

pub struct PaymentNode {
    address: SocketAddr,
    state: Arc<Mutex<State>>,
    thread: Option<JoinHandle<()>>,
}

fn read_body(stream: &mut TcpStream) -> Result<Vec<u8>> {
    stream.set_read_timeout(Some(Duration::from_secs(5)))?;
    let mut data = Vec::new();
    let mut buffer = [0; 2048];
    loop {
        let n = stream.read(&mut buffer)?;
        if n == 0 {
            return Err("truncated HTTP message".into());
        }
        data.extend_from_slice(&buffer[..n]);
        if let Some(end) = data.windows(4).position(|v| v == b"\r\n\r\n") {
            let header = std::str::from_utf8(&data[..end])?;
            let length: usize = header
                .lines()
                .find_map(|line| {
                    let (key, value) = line.split_once(':')?;
                    key.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse().unwrap())
                })
                .ok_or("missing HTTP content length")?;
            if data.len() >= end + 4 + length {
                return Ok(data[end + 4..end + 4 + length].to_vec());
            }
        }
        if data.len() > 128 * 1024 {
            return Err("oversized HTTP message".into());
        }
    }
}

impl PaymentNode {
    pub fn start() -> Result<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let state = Arc::new(Mutex::new(State::default()));
        let shared = state.clone();
        let thread = std::thread::spawn(move || {
            for connection in listener.incoming() {
                let mut stream = connection.unwrap();
                let request: Value =
                    serde_json::from_slice(&read_body(&mut stream).unwrap()).unwrap();
                let mut state = shared.lock().unwrap();
                let result = match request["method"].as_str().unwrap() {
                    "fixture_stop" => break,
                    "fixture_submitPayment" => {
                        let raw = bytes(&request["params"][0]);
                        match produce(&raw) {
                            Ok(case) => {
                                state.submissions += 1;
                                assert!(
                                    state.case.is_none(),
                                    "the adapter must not duplicate payment submission"
                                );
                                let hash = case["receiptJson"]["txHash"].clone();
                                state.case = Some(case);
                                hash
                            }
                            Err(_) => Value::Null,
                        }
                    }
                    "pax_sequencerReceipt" => {
                        if state.ready {
                            state
                                .case
                                .as_ref()
                                .filter(|case| {
                                    case["receiptJson"]["txHash"] == request["params"][0]
                                })
                                .cloned()
                                .unwrap_or(Value::Null)
                        } else {
                            Value::Null
                        }
                    }
                    other => panic!("unexpected RPC method {other}"),
                };
                let body = serde_json::to_vec(
                    &json!({"jsonrpc": "2.0", "id": request["id"], "result": result}),
                )
                .unwrap();
                write!(
                    stream,
                    "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                    body.len()
                )
                .unwrap();
                stream.write_all(&body).unwrap();
            }
        });
        Ok(Self {
            address,
            state,
            thread: Some(thread),
        })
    }

    pub fn rpc(&self, method: &str, params: Value) -> Result<Value> {
        let body = serde_json::to_vec(
            &json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}),
        )?;
        let mut stream = TcpStream::connect(self.address)?;
        stream.set_write_timeout(Some(Duration::from_secs(5)))?;
        write!(
            stream,
            "POST / HTTP/1.1\r\nHost: {}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
            self.address,
            body.len()
        )?;
        stream.write_all(&body)?;
        let response: Value = serde_json::from_slice(&read_body(&mut stream)?)?;
        assert_eq!(response["id"], 1);
        Ok(response["result"].clone())
    }

    pub fn submit(&self, intent: &FiatIntent) -> Result<[u8; 32]> {
        let call = intent.payment_calldata(PAYER, EXPIRES_AT);
        let mut raw = DOMAIN.to_vec();
        raw.extend_from_slice(&call);
        let signature = SigningKey::from_bytes(&TRANSACTION_SEED).sign(&raw);
        raw.extend_from_slice(&signature.to_bytes());
        let expected: [u8; 32] = Sha256::digest(&raw).into();
        let result = self.rpc("fixture_submitPayment", json!([hex(&raw)]))?;
        assert_eq!(array::<32>(&result), expected);
        Ok(expected)
    }

    pub fn ready(&self) {
        self.state.lock().unwrap().ready = true;
    }
    pub fn submissions(&self) -> usize {
        self.state.lock().unwrap().submissions
    }
    pub fn transcript(&self) -> Value {
        self.state.lock().unwrap().case.clone().unwrap()
    }
    pub fn calldata(case: &Value) -> Vec<u8> {
        bytes(&case["rawTransaction"])[DOMAIN.len()..DOMAIN.len() + 196].to_vec()
    }
}

impl Drop for PaymentNode {
    fn drop(&mut self) {
        // Wake the listener even when a test assertion unwinds.
        if let Ok(mut stream) = TcpStream::connect(self.address) {
            let body = br#"{"method":"fixture_stop"}"#;
            let _ = write!(
                stream,
                "POST / HTTP/1.1\r\nContent-Length: {}\r\n\r\n",
                body.len()
            );
            let _ = stream.write_all(body);
        }
        if let Some(thread) = self.thread.take() {
            let _ = thread.join();
        }
    }
}
