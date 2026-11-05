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
                assert_eq!(params, &json!(["0xa", false]));
                json!({"number":state["committed"]["receiptJson"]["blockNumber"],
                    "hash":state["committed"]["receiptJson"]["blockHash"]})
            }
            "px_getCommit" => {
                assert_eq!(params, &json!(["10"]));
                json!({"height":"10","signedHeader":state["committed"]["commit"]["signed_header"]})
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
        .expect("protocol")
        .activity_id();
    let raw = node.submitted(hash);
    assert_eq!(receipt.canonical_activity(), Some(raw.as_slice()));
    assert_eq!(
        transaction_hash(receipt.canonical_activity().expect("bound transaction")),
        hash
    );
    let (call, signature) = decode_signed(&raw).expect("submitted transaction decode");
    assert_eq!(
        recover_sender(&call, &signature).expect("recovered sender"),
        signer.evm_address().expect("signer address")
    );
    assert_eq!(call.to, call_target(0x101a));
    assert_eq!(receipt.rung(), Rung::Receipted);
    assert_eq!(receipt.committed_block(), None);
}

#[test]
fn wallet_send_retains_the_exact_submitted_pax_transaction() {
    let node = Node::start(true);
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0).expect("owner derivation");
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let debit = debit();
    let payload = run(debit.sign(&signer)).expect("real native Send authorization");
    let receipt = run(wallet.send(&payload, &options())).expect("verified wallet Send");
    assert_bound(&node, &receipt, &signer);
    let raw = node.submitted(
        receipt
            .receipt()
            .protocol()
            .expect("protocol")
            .activity_id(),
    );
    let (call, _) = decode_signed(&raw).expect("transaction");
    assert_eq!(
        bind_call(&call, owner.evm_address())
            .expect("disclosure")
            .construct_method()
            .expect("method"),
        Some(ConstructMethod::LedgerSend { payload })
    );
}

#[test]
fn inclusion_material_is_retained_exactly_and_independent_lookup_has_no_transaction() {
    let node = Node::start(true);
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0).expect("owner");
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let receipt = run(wallet.send_debit(&debit(), &options())).expect("verified SendDebit");
    assert_bound(&node, &receipt, &signer);
    let hash = receipt
        .receipt()
        .protocol()
        .expect("protocol")
        .activity_id();
    let material = node.state.lock().expect("node store")["receipts"][format!("0x{}", hex(&hash))]
        ["inclusion"]
        .clone();
    let evidence = receipt
        .batch_evidence()
        .expect("retained inclusion material");
    assert_eq!(
        evidence.canonical_header(),
        unhex(
            material["signed_header"]["canonical_header"]
                .as_str()
                .expect("header")
        )
    );
    assert_eq!(
        evidence.header_signature().as_slice(),
        unhex(
            material["signed_header"]["signature"]
                .as_str()
                .expect("signature")
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
            .expect("siblings")
            .iter()
            .map(|s| s.as_str().expect("sibling").to_owned())
            .collect::<Vec<_>>()
    );
    verify_receipt(
        receipt.canonical_bytes(),
        evidence.proof(),
        evidence.canonical_header(),
        &evidence.header_signature(),
        &policy.sequencer,
    )
    .expect("retained evidence independently verifies");
    let independent = rpc
        .wait_for(hash, Commitment::Executed, &policy, Duration::from_secs(5))
        .expect("independent lookup");
    assert_eq!(independent.canonical_activity(), None);
    assert_eq!(independent.canonical_bytes(), receipt.canonical_bytes());
    assert_eq!(
        independent
            .batch_evidence()
            .expect("independent verified inclusion")
            .proof(),
        evidence.proof()
    );
    node.state.lock().expect("node store")["receipts"][format!("0x{}", hex(&hash))]
        .as_object_mut()
        .expect("summary")
        .remove("inclusion");
    let summary = rpc
        .wait_for(hash, Commitment::Executed, &policy, Duration::from_secs(5))
        .expect("signed summary without inclusion");
    assert_eq!(summary.canonical_activity(), None);
    assert!(summary.batch_evidence().is_none());
    assert_eq!(summary.committed_block(), None);
}

#[test]
fn payment_and_issue_bind_the_submitted_bytes_without_inventing_inclusion() {
    let node = Node::start(false);
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0).expect("owner");
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let options = options();
    let opened = run(wallet.open_account([3; 32], &options)).expect("wallet open account");
    assert_bound(&node, &opened, &signer);
    assert!(opened.batch_evidence().is_none());
    let minted = run(wallet.mint([4; 32], [2; 32], 10, &options)).expect("wallet mint");
    assert_bound(&node, &minted, &signer);
    let burned = run(wallet.burn([4; 32], [2; 32], 1, &options)).expect("wallet burn");
    assert_bound(&node, &burned, &signer);
    let payload = unhex(include_str!("../../layerx-crypto/tests/fixtures/payments/1-7.hex").trim());
    let Payment::IssueGrant(grant) = Payment::decode(
        layerx_types::payload::ModuleId::Asset,
        7,
        &payload,
        options.actor.as_bytes(),
    )
    .expect("canonical signed grant producer fixture") else {
        panic!("expected signed grant");
    };
    let issued = run(wallet.issue(grant, &options)).expect("wallet grant issue");
    assert_bound(&node, &issued, &signer);
    assert!(issued.batch_evidence().is_none());
    let revoked = run(wallet.revoke([7; 32], 1, &options)).expect("wallet revoke");
    assert_bound(&node, &revoked, &signer);
}

#[test]
fn invalid_inclusion_material_is_refused_without_dropping_to_summary_verification() {
    let node = Node::start(true);
    let rpc = node.rpc();
    let owner = derive_from_seed(&[0x41; 32], 0).expect("owner");
    let signer = LocalSigner::from_derived_account(&owner);
    let policy = policy();
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let receipt = run(wallet.open_account([3; 32], &options())).expect("verified wallet payment");
    let hash = receipt
        .receipt()
        .protocol()
        .expect("protocol")
        .activity_id();
    let id = format!("0x{}", hex(&hash));
    let original = node.state.lock().expect("node store")["receipts"][&id].clone();
    let mut corruptions = Vec::new();
    for (path, replacement) in [
        ("/inclusion/signed_header/signature", json!("00".repeat(64))),
        ("/inclusion/proof/siblings/0", json!("00".repeat(32))),
        ("/inclusion/activity_id", json!("00".repeat(32))),
        ("/inclusion/canonical_value", json!("00")),
        ("/inclusion/proof/leaf_index", json!(0)),
    ] {
        let mut changed = original.clone();
        *changed.pointer_mut(path).expect("inclusion field") = replacement;
        corruptions.push(changed);
    }
    let mut changed = original.clone();
    changed["inclusion"] = inclusion_on_network(&original, 126);
    corruptions.push(changed);
    let mut changed = original.clone();
    changed["inclusion"] = Value::Null;
    corruptions.push(changed);
    for changed in corruptions {
        node.state.lock().expect("node store")["receipts"][&id] = changed;
        assert!(matches!(
            rpc.wait_for(hash, Commitment::Executed, &policy, Duration::from_secs(5)),
            Err(RpcError::Verification | RpcError::InvalidResponse)
        ));
    }
}

#[test]
fn committed_lookup_keeps_its_verified_block_and_does_not_invent_batch_evidence() {
    use layerx_sdk::rpc_verification::node_commit::{
        NodeCommitAuthority, NodeCommitCoverage, NodeTransactionProof,
    };
    let node = Node::start(false);
    let record: Value = serde_json::from_str(include_str!(
        "../src/repair_22_31_227_fixture/committed.json"
    ))
    .expect("existing real node and Comet transcript");
    let hash_text = record["receiptJson"]["txHash"].as_str().expect("hash");
    let hash: [u8; 32] = unhex(hash_text).try_into().expect("hash width");
    {
        let mut state = node.state.lock().expect("node store");
        state["receipts"][hash_text] = record["receiptJson"].clone();
        state["committed"] = record.clone();
    }
    let authority = NodeCommitAuthority::new(
        "committed-fixture".to_owned(),
        vec![(
            unhex("c614a1c91884c3c13bed586a16d51f7253072f7cc41018ea28dd864e4e548e4d")
                .try_into()
                .expect("independently pinned validator"),
            7_000_000_000,
        )],
        10,
        10,
    )
    .expect("pinned Comet authority");
    let coverage = NodeCommitCoverage {
        covering_height: 10,
        batch_transaction: unhex(record["batch"]["tx"].as_str().expect("batch transaction")),
        transaction_proof: NodeTransactionProof {
            total: record["batch"]["proof"]["total"]
                .as_u64()
                .expect("leaf count"),
            index: record["batch"]["proof"]["index"]
                .as_u64()
                .expect("leaf index"),
            aunts: record["batch"]["proof"]["aunts"]
                .as_array()
                .expect("aunts")
                .iter()
                .map(|v| {
                    unhex(v.as_str().expect("aunt"))
                        .try_into()
                        .expect("aunt width")
                })
                .collect(),
        },
        activity_ids: vec![hash],
    };
    let rpc = node
        .rpc()
        .with_node_commit_authority(authority, coverage)
        .expect("committed coverage");
    let policy = ReceiptPolicy {
        protocol_version: 2,
        network_id: 125,
        sequencer: SequencerAuthorization::new(
            unhex(record["pinnedKey"]["keyId"].as_str().expect("key id"))
                .try_into()
                .expect("key id width"),
            unhex(
                record["pinnedKey"]["publicKey"]
                    .as_str()
                    .expect("receipt key"),
            )
            .try_into()
            .expect("key width"),
            10,
            10,
        ),
        trusted_checkpoint_context_digest: None,
    };
    let receipt = rpc
        .wait_for(hash, Commitment::Committed, &policy, Duration::from_secs(5))
        .expect("verified committed receipt");
    let block_hash = unhex(
        record["receiptJson"]["blockHash"]
            .as_str()
            .expect("block hash"),
    )
    .try_into()
    .expect("block hash width");
    assert_eq!(receipt.committed_block(), Some((10, block_hash)));
    assert_eq!(receipt.rung(), Rung::Committed);
    assert_eq!(receipt.canonical_activity(), None);
    assert!(receipt.batch_evidence().is_none());
    assert_eq!(
        receipt.canonical_bytes(),
        unhex(
            record["receiptJson"]["receipt"]
                .as_str()
                .expect("canonical receipt")
        )
    );
}
