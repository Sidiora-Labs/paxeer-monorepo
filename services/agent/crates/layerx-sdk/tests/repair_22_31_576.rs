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
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}
fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert!(text.len().is_multiple_of(2));
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("hex text"), 16).expect("hex byte")
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

fn fixture_receipt(raw: &[u8], seq: u64) -> Value {
    use layerx_wire::encode::Encoder;
    let (call, signature) = decode_signed(raw).expect("fixture signed call");
    let sender = recover_sender(&call, &signature).expect("fixture signer");
    let activity = transaction_hash(raw);
    let returned = Vec::<u8>::new();
    let logs = Vec::<Value>::new();
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

fn inclusion(summary: &Value) -> Value {
    inclusion_on_network(summary, 125)
}

fn inclusion_on_network(summary: &Value, network: u32) -> Value {
    let canonical = unhex(summary["receipt"].as_str().expect("signed receipt"));
    let decoded = layerx_wire::receipt::decode(&canonical).expect("receipt producer codec");
    let receipt = decoded.protocol().expect("protocol receipt");
    let (proof, root) = build_proof(&[b"prior receipt", &canonical, b"next receipt"], 1)
        .expect("production Merkle constructor");
    let key = SigningKey::from_bytes(&[0x5e; 32]);
    let key_id: [u8; 32] = Sha256::digest(key.verifying_key().as_bytes()).into();
    let mut encoder = Encoder::new(1024);
    encoder
        .structure_header_version(0x1701, 2)
        .expect("batch header");
    encoder.u8(15).expect("field count");
    for field in 1..=15 {
        encoder.tag(field, 15).expect("batch field");
        match field {
            1 => encoder.u16(2).expect("protocol"),
            2 => encoder.u32(network).expect("network"),
            3 => encoder.u64(1).expect("epoch"),
            4 => encoder.u64(1000).expect("batch number"),
            5 | 6 => encoder.u64(receipt.global_sequence()).expect("sequence"),
            7 => encoder
                .bytes(&receipt.previous_state_root(), 32)
                .expect("previous root"),
            8 => encoder
                .bytes(&receipt.resulting_state_root(), 32)
                .expect("resulting root"),
            10 => encoder.bytes(&root, 32).expect("receipt root"),
            14 => encoder.u64(receipt.timestamp()).expect("timestamp"),
            15 => encoder.bytes(&key_id, 32).expect("sequencer"),
            _ => encoder.bytes(&[1; 32], 32).expect("commitment"),
        }
    }
    let header = encoder.finish();
    let signature = key
        .sign(&batch_header_digest(&header).expect("header digest"))
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
        let owner = derive_from_seed(&[0x41; 32], 0).expect("wallet owner");
        let state = Arc::new(Mutex::new(json!({
            "owner":owner.evm_address_text(), "nonce":0, "include_proof":include_proof,
            "transactions":{}, "receipts":{}, "requests":[]
        })));
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback node");
        listener.set_nonblocking(true).expect("bounded listener");
        let endpoint = format!("http://{}", listener.local_addr().expect("node address"));
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
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("read bound");
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .expect("write bound");
                let mut reader = BufReader::new(stream);
                let mut line = String::new();
                reader.read_line(&mut line).expect("HTTP request");
                assert_eq!(line, "POST / HTTP/1.1\r\n");
                let mut length = None;
                let mut header_bytes = line.len();
                loop {
                    line.clear();
                    reader.read_line(&mut line).expect("HTTP header");
                    header_bytes += line.len();
                    assert!(header_bytes <= 8192);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, value)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("content-length") {
                            length = Some(value.trim().parse::<usize>().expect("body length"));
                        }
                    }
                }
                let length = length.expect("length-delimited request");
                assert!(length <= 1_048_576 + 4096);
                let mut body = vec![0; length];
                reader.read_exact(&mut body).expect("request bytes");
                let request: Value = serde_json::from_slice(&body).expect("request JSON");
                let result = Self::response(&mut store.lock().expect("node store"), &request);
                let body = serde_json::to_vec(
                    &json!({"jsonrpc":"2.0","id":request["id"],"result":result}),
                )
                .expect("response JSON");
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len())
                    .expect("HTTP response");
                stream.write_all(&body).expect("response bytes");
            }
        });
        Self {
            endpoint,
            state,
            stop,
            worker: Some(worker),
        }
    }

    fn response(state: &mut Value, request: &Value) -> Value {
        state["requests"]
            .as_array_mut()
            .expect("request journal")
            .push(request.clone());
        let params = &request["params"];
        match request["method"].as_str().expect("RPC method") {
            "eth_chainId" => json!("0x7d"),
            "eth_gasPrice" | "eth_maxPriorityFeePerGas" => json!("0x1"),
            "eth_estimateGas" => json!("0x186a0"),
            "eth_getTransactionCount" => {
                assert_eq!(params.as_array().expect("nonce params").len(), 2);
                assert_eq!(
                    unhex(params[0].as_str().expect("nonce address")),
                    unhex(state["owner"].as_str().expect("wallet owner"))
                );
                assert_eq!(params[1], "pending");
                json!(format!("0x{:x}", state["nonce"].as_u64().expect("nonce")))
            }
            "px_getAccount" => json!({"account_id": params[0], "next_sequence":"7"}),
            "eth_sendRawTransaction" => {
                let raw = unhex(params[0].as_str().expect("signed transaction"));
                let (call, signature) =
                    decode_signed(&raw).expect("production transaction decoder");
                let sender = recover_sender(&call, &signature).expect("signature recovery");
                assert_eq!(
                    sender.as_slice(),
                    unhex(state["owner"].as_str().expect("wallet owner"))
                );
                assert_eq!(call.nonce, state["nonce"].as_u64().expect("stored nonce"));
                assert_eq!(call.chain_id, 125);
                assert_eq!(call.to, call_target(0x101a));
                bind_call(&call, sender).expect("production disclosure verifier");
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
            "pax_sequencerReceipt" => {
                state["receipts"][params[0].as_str().expect("activity hash")].clone()
            }
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
            "eth_getRawTransactionByHash" => {
                state["transactions"][params[0].as_str().expect("activity hash")].clone()
            }
            method => panic!("unexpected RPC method {method}"),
        }
    }

    fn rpc(&self) -> RpcClient {
        RpcClient::connect_node(&self.endpoint, None)
            .expect("direct node RPC")
            .with_clock(Arc::new(OsClock::new()))
    }
    fn submitted(&self, hash: [u8; 32]) -> Vec<u8> {
        let body = serde_json::to_vec(&json!({"jsonrpc":"2.0","id":"raw",
            "method":"eth_getRawTransactionByHash","params":[format!("0x{}",hex(&hash))]}))
        .expect("raw transaction request");
        let mut response = ureq::post(&self.endpoint)
            .header("Content-Type", "application/json")
            .config()
            .timeout_global(Some(Duration::from_secs(5)))
            .build()
            .send(body.as_slice())
            .expect("node raw transaction lookup");
        let result: Value =
            serde_json::from_slice(&response.body_mut().read_to_vec().expect("raw response"))
                .expect("raw JSON");
        assert_eq!(result["id"], "raw");
        unhex(result["result"].as_str().expect("submitted bytes"))
    }
}
impl Drop for Node {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let result = worker.join();
            if !std::thread::panicking() {
                result.expect("node worker");
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
        actor: derive_from_seed(&[0x41; 32], 0).expect("owner").did(),
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
                char::from(ALPHABET[((word >> (18 - 6 * index)) & 63) as usize])
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
    fn publish(node: &Node, receipt: &VerifiedRpcReceipt) -> Self {
        let protocol = receipt.receipt().protocol().expect("protocol receipt");
        let activity = protocol.activity_id();
        let receipt_key = SigningKey::from_bytes(&[0x5e; 32]);
        let validator_key = SigningKey::from_bytes(&[0x72; 32]);
        let validator = TrustedValidator::new(validator_key.verifying_key().to_bytes(), 10);
        let verifier = ChainProofVerifier {
            chain_id: "submitted-wait".into(),
            validators: ValidatorSet::new(vec![validator.clone()]).expect("pinned validator set"),
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
                    .expect("batch digest"),
            )
            .to_bytes();
        let batch_transaction = batch.encode().expect("production sequence-batch encoder");
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
        let header = ChainHeader {
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
            app_hash: protocol.resulting_state_root().to_vec(),
            last_results_hash: vec![0x33; 32],
            evidence_hash: vec![0x33; 32],
            proposer_address: validator.address.to_vec(),
        };
        let block_hash = header.hash().expect("production Comet header hash");
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
                .expect("production Comet vote encoder"),
            )
            .to_bytes();
        let id_json = json!({"hash":hex(&block_hash),
            "parts":{"total":1,"hash":hex(&[0x33; 32])}});
        let signed = json!({
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
        });
        let mut state = node.state.lock().expect("node store");
        let summary = &mut state["receipts"][format!("0x{}", hex(&activity))];
        summary["rung"] = json!("committed");
        summary["blockNumber"] = json!("0x3e8");
        summary["blockHash"] = json!(format!("0x{}", hex(&block_hash)));
        state["committed"] = json!({"receiptJson":summary.clone(),
            "activityIds":[format!("0x{}", hex(&activity))],
            "commit":{"signed_header":signed},
            "batch":{"height":"1000", "tx":format!("0x{}", hex(&batch_transaction)),
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
        .expect("pinned node commit authority")
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
        .expect("canonical payment payload");
    let prepared = wallet
        .prepare_payload(module, ordinal, &payload, &options)
        .expect("prepared payment");
    run(wallet.execute(prepared, &options)).expect("submitted signed payment")
}

fn assert_activity(receipt: &VerifiedRpcReceipt, activity: [u8; 32], canonical: &[u8]) {
    assert_eq!(
        receipt
            .receipt()
            .protocol()
            .expect("protocol receipt")
            .activity_id(),
        activity
    );
    assert_eq!(receipt.canonical_activity(), Some(canonical));
    assert_eq!(transaction_hash(canonical), activity);
}

fn assert_inclusion(receipt: &VerifiedRpcReceipt, policy: &ReceiptPolicy) {
    let evidence = receipt
        .batch_evidence()
        .expect("verified batch evidence retained");
    verify_receipt(
        receipt.canonical_bytes(),
        evidence.proof(),
        evidence.canonical_header(),
        &evidence.header_signature(),
        &policy.sequencer,
    )
    .expect("retained inclusion re-verifies");
}

#[test]
fn bind_canonical_activity() {
    let node = Node::start(true);
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0).expect("owner");
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = wallet(&rpc, &signer, &policy);
    let activity = submit_payment(&wallet);
    let raw = node.submitted(activity);
    assert_eq!(transaction_hash(&raw), activity);
    let executed = wallet
        .wait_for_signed(&raw, Commitment::Executed, Duration::from_secs(5))
        .expect("signed payment waited to Executed");
    assert_eq!(executed.commitment(), Commitment::Executed);
    assert_eq!(executed.rung(), Rung::Receipted);
    assert_eq!(executed.committed_block(), None);
    assert_activity(&executed, activity, &raw);
    assert_inclusion(&executed, &policy);

    let commit = CommitFixture::publish(&node, &executed);
    let committed_rpc = node
        .rpc()
        .with_node_commit_authority(commit.authority())
        .expect("node commit trust");
    let committed_wallet = self::wallet(&committed_rpc, &signer, &policy);
    let batched = committed_wallet
        .wait_for_submitted(&executed, Commitment::Batched, Duration::from_secs(5))
        .expect("Batched submitted receipt");
    assert_eq!(batched.commitment(), Commitment::Batched);
    assert_eq!(batched.rung(), Rung::Committed);
    assert_eq!(batched.committed_block(), Some((1000, commit.block_hash)));
    assert_activity(&batched, activity, &raw);
    assert_inclusion(&batched, &policy);
    let original = executed.batch_evidence().expect("Executed batch evidence");
    let retained = batched.batch_evidence().expect("Batched batch evidence");
    assert_eq!(retained.canonical_header(), original.canonical_header());
    assert_eq!(retained.header_signature(), original.header_signature());
    assert_eq!(retained.proof(), original.proof());
    assert_activity(&executed, activity, &raw);
    assert_eq!(executed.commitment(), Commitment::Executed);
    assert_eq!(
        node.state.lock().expect("node store")["requests"]
            .as_array()
            .expect("requests")
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
    let owner = derive_from_seed(&[0x41; 32], 0).expect("owner");
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
    assert!(node.state.lock().expect("node store")["requests"]
        .as_array()
        .expect("requests")
        .is_empty());
}

#[test]
fn hash_only_wait_has_no_canonical_activity() {
    let node = Node::start(true);
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0).expect("owner");
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = wallet(&rpc, &signer, &policy);
    let activity = submit_payment(&wallet);
    let independent = wallet
        .wait_for(activity, Commitment::Executed, Duration::from_secs(5))
        .expect("hash-only wait");
    assert_eq!(independent.commitment(), Commitment::Executed);
    assert_eq!(
        independent
            .receipt()
            .protocol()
            .expect("protocol")
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
        .expect("signed wait for the same activity");
    assert_activity(&bound, activity, &raw);
    assert_eq!(bound.canonical_bytes(), independent.canonical_bytes());
    assert_eq!(independent.canonical_activity(), None);
}
