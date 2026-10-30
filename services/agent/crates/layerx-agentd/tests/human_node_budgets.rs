#[path = "support/send_authorization.rs"]
mod send_authorization;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::budget::{
    create_protocol_budget, BudgetCreateIdentity, BudgetKind, BudgetRequest, ProtocolBudgetState,
};
use layerx_agentd::human_node::HumanNode;
use layerx_agentd::human_runtime::NodeBudgetPipeline;
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::prepare::{restore_canonical, EvmPrepared};
use layerx_agentd::protocol_evidence::EvidenceAuthority;
use layerx_agentd::sign::{verify_before_submit, SigningPreparation};
use layerx_agentd::store::{Store, TenantId};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_types::activity::{Authority, EnvelopeBuilder, TimestampBound};
use layerx_types::amount::Amount;
use layerx_types::ids::{Did, IdempotencyKey};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};
#[track_caller]
fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{error:?}"),
    }
}

fn serve(exchanges: Vec<(&'static str, Value)>) -> (String, JoinHandle<Vec<Value>>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    must(listener.set_nonblocking(true));
    let handle = thread::spawn(move || {
        let mut requests = Vec::new();
        for (method, result) in exchanges {
            let expires = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < expires, "missing {method} request");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            must(stream.set_read_timeout(Some(Duration::from_secs(2))));
            must(stream.set_write_timeout(Some(Duration::from_secs(2))));
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(must(value.trim().parse::<usize>()));
                }
            }
            let length = length.unwrap_or_else(|| panic!("content length required"));
            assert!(length <= 64 * 1024);
            let mut body = vec![0; length];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            let answer =
                json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
            must(stream.flush());
            requests.push(request);
        }
        requests
    });
    (url, handle)
}

fn rpc_hex(bytes: &[u8]) -> String {
    format!("0x{}", hex::encode(bytes))
}

const HEIGHT: u64 = 150;
const KEY_ID: [u8; 32] = [1; 32];

fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn encoded_receipt(
    tx_hash: [u8; 32],
    sender: [u8; 20],
    success: bool,
    signature: Option<[u8; 64]>,
) -> Vec<u8> {
    let previous = [4; 32];
    let mut sequence = b"PAXEER/v1/sequence\0".to_vec();
    sequence.extend_from_slice(&previous);
    sequence.extend_from_slice(&7_u64.to_be_bytes());
    sequence.extend_from_slice(&tx_hash);
    let mut height = [0; 32];
    height[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    let mut to = [0; 32];
    to[30..].copy_from_slice(&[0x10, 0x1b]);
    let vm_error = if success { "" } else { "execution reverted" };
    let mut summary = Encoder::new(256);
    must(summary.u64(21_000));
    must(summary.u64(42_000));
    must(summary.u64(1));
    must(summary.u32(1));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&sha256(vm_error.as_bytes())));
    must(summary.u32(2));
    must(summary.u32(1));
    must(summary.u8(4));
    must(summary.fixed(b"upax"));
    let mut log = vec![0; 20];
    log[18..].copy_from_slice(&[0x10, 0x1b]);
    log.push(2);
    log.extend_from_slice(&transaction_hash(b"BudgetCreated(bytes32,bytes32,uint128)"));
    log.extend_from_slice(&identity().budget_id);
    log.extend_from_slice(&sha256(&creation_log_data()));

    let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
    must(encoder.structure_header_version(0x5201, PROTOCOL_VERSION));
    must(encoder.u16(PROTOCOL_VERSION));
    must(encoder.bytes(&tx_hash, 32));
    must(encoder.u64(7));
    must(encoder.bytes(&previous, 32));
    must(encoder.bytes(&sha256(&sequence), 32));
    must(encoder.bytes(&sha256(&[0xab, 0xcd]), 32));
    must(encoder.i32(i32::from(!success)));
    must(encoder.sequence_length(2, 512));
    for (ordinal, kind, body) in [(0, 1, summary.finish()), (1, 3, log)] {
        must(encoder.u16(12));
        must(encoder.u16(ordinal));
        must(encoder.u16(0));
        must(encoder.u8(kind));
        must(encoder.u8(0));
        must(encoder.bytes(&[0; 32], 32));
        must(encoder.bytes(&body, 256));
    }
    must(encoder.u128(21_000));
    must(encoder.bytes(&height, 32));
    must(encoder.u16(12));
    must(encoder.u32(1));
    must(encoder.u32(0));
    must(encoder.u8(2));
    must(encoder.bytes(&sha256(b"upax"), 32));
    must(encoder.u128(1));
    must(encoder.bytes(&from, 32));
    must(encoder.u128(21_002));
    must(encoder.u128(if success { 1 } else { 2 }));
    must(encoder.u64(7));
    must(encoder.bytes(&to, 32));
    must(encoder.u128(0));
    must(encoder.u128(u128::from(success)));
    must(encoder.bytes(&[0; 32], 32));
    must(encoder.bytes(&[0; 32], 32));
    must(encoder.bytes(&KEY_ID, 32));
    must(encoder.u64(1_000));
    must(encoder.u8(u8::from(signature.is_some())));
    if let Some(signature) = signature {
        must(encoder.bytes(&signature, 64));
    }
    encoder.finish()
}

fn signed_receipt(
    tx_hash: [u8; 32],
    sender: [u8; 20],
    success: bool,
) -> (Value, Vec<u8>, ReceiptKeyEntry) {
    let signer = SigningKey::from_bytes(&[8; 32]);
    let unsigned = encoded_receipt(tx_hash, sender, success, None);
    let signature = signer.sign(&must(receipt_digest(&unsigned))).to_bytes();
    let bytes = encoded_receipt(tx_hash, sender, success, Some(signature));
    let key = ReceiptKeyEntry {
        key_id: KEY_ID,
        public_key: signer.verifying_key().to_bytes(),
        first_height: 100,
        last_height: 199,
    };
    let view = json!({
        "rung": "committed", "blockNumber": "0x96", "blockHash": rpc_hex(&[0x11; 32]), "txHash": rpc_hex(&tx_hash), "nonce": "0x7",
        "seq": "0x7", "height": "0x96", "index": "0x1", "timestamp": "0x3e8",
        "status": if success { "0x1" } else { "0x0" }, "returnData": "0xabcd",
        "logs": [{"address": rpc_hex(&layerx_agentd::prepare::evm::PrecompileTarget::Pay.address()),
            "topics": [rpc_hex(&transaction_hash(b"BudgetCreated(bytes32,bytes32,uint128)")), rpc_hex(&identity().budget_id)],
            "data": rpc_hex(&creation_log_data()), "logIndex": "0x0"}],
        "gasUsed": "0x5208", "cumulativeGasUsed": "0xa410", "effectiveGasPrice": "0x1",
        "feeDenom": "upax", "feeAmount": "0x5208", "sponsor": rpc_hex(&[0; 20]),
        "from": rpc_hex(&sender), "to": rpc_hex(&layerx_agentd::prepare::evm::PrecompileTarget::Pay.address()), "contractAddress": rpc_hex(&[0; 20]),
        "vmError": if success { "" } else { "execution reverted" },
        "keyId": rpc_hex(&KEY_ID), "receipt": rpc_hex(&bytes),
        "signingKey": rpc_hex(&signer.verifying_key().to_bytes())
    });
    (view, bytes, key)
}

fn published_key(key: ReceiptKeyEntry) -> Value {
    json!([{
        "keyId": rpc_hex(&key.key_id),
        "publicKey": rpc_hex(&key.public_key),
        "firstHeight": format!("{:#x}", key.first_height),
        "lastHeight": format!("{:#x}", key.last_height),
    }])
}

pub fn request() -> BudgetRequest {
    BudgetRequest {
        tenant: TenantId::new("budget-owner").expect("tenant"),
        request_id: [9; 32],
        kind: BudgetKind::ProtocolBudget,
        asset: [3; 32],
        ceiling: 5_000,
        expiry_ms: 90_000,
        core_time_ms: 1_000,
        canonical_activity: Vec::new(),
        verified_submission: None,
    }
}

pub fn identity() -> BudgetCreateIdentity {
    BudgetCreateIdentity {
        encoding_version: 1,
        budget_id: [1; 32],
        sender: [0; 20],
        budget_account: [2; 32],
        asset: [3; 32],
        purpose: [4; 32],
        per_period_limit: 5_000,
        carry_cap: 0,
        initial_amount: 5_000,
        period_length_ms: 60_000,
        period_start_ms: 1_000,
        expiry_ms: 90_000,
        revocation_sequence: 7,
        rollover: 1,
        source_account: [0; 32],
        source_sequence: 0,
    }
}

pub fn signed_call(request: &BudgetRequest, identity: &BudgetCreateIdentity) -> Vec<u8> {
    let prepared = request
        .create_protocol_budget(identity)
        .expect("budget calldata");
    Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 3,
        gas_limit: 100_000,
        to: prepared.target.address(),
        data: prepared.encode().expect("ABI encoding"),
    }
    .sign(&[0xa5; 32])
    .expect("real secp256k1 signature")
}

pub fn budget_result() -> Vec<u8> {
    let mut result = vec![0; 20 * 32];
    for (index, value) in [
        (0, 32_u128),
        (5, 5_000),
        (6, 5_000),
        (10, 60_000),
        (11, 1_000),
        (12, 90_000),
        (13, 7),
        (14, 1),
        (17, 18 * 32),
    ] {
        result[index * 32 + 16..(index + 1) * 32].copy_from_slice(&value.to_be_bytes());
    }
    for (index, value) in [(1, 0x11), (2, 2), (3, 3), (4, 4)] {
        result[index * 32..(index + 1) * 32].fill(value);
    }
    result
}

fn send_type() -> ActivityType {
    ActivityType::new(ModuleId::Asset, 5).unwrap_or_else(|error| panic!("activity: {error:?}"))
}

fn registry() -> ModuleRegistry {
    ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Asset, &[send_type()])
        .unwrap_or_else(|error| panic!("registration: {error:?}"))])
    .unwrap_or_else(|error| panic!("registry: {error:?}"))
}

fn send_payload() -> Vec<u8> {
    let mut encoder = Encoder::new(512);
    for result in [encoder.u16(0x5301), encoder.u16(10)] {
        result.unwrap_or_else(|error| panic!("header: {error:?}"));
    }
    encoder
        .fixed(&[0x11; 32])
        .unwrap_or_else(|error| panic!("from: {error:?}"));
    encoder
        .fixed(&[0x22; 32])
        .unwrap_or_else(|error| panic!("to: {error:?}"));
    encoder
        .fixed(&[0x33; 32])
        .unwrap_or_else(|error| panic!("asset: {error:?}"));
    encoder
        .u128(25)
        .unwrap_or_else(|error| panic!("amount: {error:?}"));
    encoder
        .u64(5)
        .unwrap_or_else(|error| panic!("sequence: {error:?}"));
    encoder
        .fixed(&[4; 32])
        .unwrap_or_else(|error| panic!("idempotency: {error:?}"));
    encoder
        .u64(1_010)
        .unwrap_or_else(|error| panic!("expiry: {error:?}"));
    encoder
        .fixed(&[0x55; 32])
        .unwrap_or_else(|error| panic!("context: {error:?}"));
    encoder
        .u8(0)
        .unwrap_or_else(|error| panic!("conditions: {error:?}"));
    encoder
        .u8(1)
        .unwrap_or_else(|error| panic!("authority kind: {error:?}"));
    encoder
        .fixed(&[0x11; 32])
        .unwrap_or_else(|error| panic!("controller: {error:?}"));
    encoder
        .fixed(&[0x66; 32])
        .unwrap_or_else(|error| panic!("payload key: {error:?}"));
    encoder
        .fixed(&[0x77; 64])
        .unwrap_or_else(|error| panic!("payload signature: {error:?}"));
    encoder
        .fixed(&[0x55; 32])
        .unwrap_or_else(|error| panic!("signed context: {error:?}"));
    encoder
        .u32(17)
        .unwrap_or_else(|error| panic!("network: {error:?}"));
    encoder
        .u16(layerx_wire::limits::PROTOCOL_VERSION)
        .unwrap_or_else(|error| panic!("version: {error:?}"));
    send_authorization::sign(encoder.finish())
}

fn prepared(authority: Authority) -> SigningPreparation {
    let payload = layerx_types::payload::Payload::new(&registry(), send_type(), &send_payload())
        .unwrap_or_else(|error| panic!("payload: {error:?}"));
    let payload_hash = layerx_wire::hash::payload_hash_for(&payload)
        .unwrap_or_else(|error| panic!("payload hash: {error:?}"));
    let mut builder = EnvelopeBuilder::new();
    builder
        .protocol_version(layerx_wire::limits::PROTOCOL_VERSION)
        .and_then(|builder| builder.network_id(17))
        .and_then(|builder| builder.activity_type(send_type()))
        .and_then(|builder| builder.actor_did(Did::new(b"did:layerx:prepare-agent").unwrap()))
        .and_then(|builder| builder.authority(authority.clone()))
        .and_then(|builder| builder.account_sequence(5))
        .and_then(|builder| builder.timestamp_bound(TimestampBound::new(995, 1_010).unwrap()))
        .and_then(|builder| builder.idempotency_key(IdempotencyKey::new([4; 32])))
        .and_then(|builder| builder.fee_limit(Amount::from_u128(7)))
        .and_then(|builder| builder.payload_hash(payload_hash))
        .and_then(|builder| builder.payload(payload))
        .unwrap_or_else(|error| panic!("envelope: {error:?}"));
    let envelope = builder
        .build()
        .unwrap_or_else(|error| panic!("envelope: {error:?}"));
    let canonical = layerx_wire::activity::encode_unsigned_envelope(&envelope)
        .unwrap_or_else(|error| panic!("encode: {error:?}"));
    let authorization = restore_canonical(&canonical, 88, &registry(), Some(&authority))
        .unwrap_or_else(|error| panic!("restore: {error:?}"));
    let call = Eip1559Call {
        chain_id: 17,
        nonce: 5,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 7,
        gas_limit: 100_000,
        to: [0x1a; 20],
        data: send_payload(),
    };
    SigningPreparation {
        transaction: EvmPrepared {
            signing_hash: call.signing_hash(),
            call: call.clone(),
            observed_head_height: 88,
        },
        actor_address: {
            let raw = call.sign(&[0xa5; 32]).unwrap();
            let (c, sig) = decode_signed(&raw).unwrap();
            recover_sender(&c, &sig).unwrap()
        },
        authorization,
    }
}

fn creation_log_data() -> Vec<u8> {
    [
        identity().asset.as_slice(),
        &[0; 16],
        &identity().initial_amount.to_be_bytes(),
    ]
    .concat()
}
fn block() -> Value {
    json!({"number": "0x96", "hash": rpc_hex(&[0x11; 32]), "parentHash": rpc_hex(&[0x22; 32]),
        "timestamp": "0x3e8", "transactionsRoot": rpc_hex(&[0x33; 32]), "receiptsRoot": rpc_hex(&[0x44; 32]), "transactions": []})
}
#[test]
fn budget_pipeline_submits_raw_transaction_and_reads_get_budget() {
    let mut request = request();
    let raw = signed_call(&request, &identity());
    assert_eq!(raw[0], 2);
    let (call, signature) = must(decode_signed(&raw));
    let sender = must(recover_sender(&call, &signature));
    let authority = must(Authority::owner(&[0xa5; 32]));
    let mut preparation = prepared(authority);
    preparation.transaction = EvmPrepared {
        signing_hash: call.signing_hash(),
        call,
        observed_head_height: HEIGHT,
    };
    preparation.actor_address = sender;
    let submission = must(verify_before_submit(
        &raw,
        &preparation,
        &sender,
        &registry(),
    ));
    request.request_id = submission.idempotency_key();
    request.canonical_activity = raw.clone();
    request.verified_submission = Some(submission);
    let (receipt, _, key) = signed_receipt(transaction_hash(&raw), sender, true);
    let result = budget_result();
    let (url, server) = serve(vec![
        ("eth_chainId", json!("0x7d")),
        (
            "eth_sendRawTransaction",
            json!(rpc_hex(&transaction_hash(&raw))),
        ),
        ("pax_sequencerReceipt", Value::Null),
        (
            "pax_sequencerReceipt",
            json!({"rung": "submitted", "txHash": rpc_hex(&transaction_hash(&raw)), "nonce": "0x7"}),
        ),
        ("pax_sequencerReceipt", receipt),
        ("px_getReceiptKeys", published_key(key)),
        ("eth_blockNumber", json!("0x96")),
        ("eth_getBlockByNumber", block()),
        ("eth_getBlockByNumber", block()),
        ("eth_call", json!(rpc_hex(&result))),
    ]);
    let node: Box<dyn HumanNode> = Box::new(must(NodeRpc::connect(&NodeRpcConfig {
        url,
        chain_id: 125,
        deadline: Duration::from_secs(2),
    })));
    let mut pipeline =
        NodeBudgetPipeline::new(node.as_ref(), ProtocolBudgetState::decode_get_budget);
    let verifier = must(EvidenceAuthority::pinned_to_handshake(
        PROTOCOL_VERSION,
        125,
        key.public_key,
    ));
    let directory = std::env::temp_dir().join(format!(
        "human-node-budgets-{}-{}",
        std::process::id(),
        must(std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH)).as_nanos()
    ));
    let mut store = must(Store::open(&directory));
    let budget = must(create_protocol_budget(
        &mut store,
        &request,
        &registry(),
        &verifier,
        &mut pipeline,
    ));
    assert_eq!(budget.object_id(), identity().budget_id);
    assert_eq!(budget.observed_head_sequence(), HEIGHT);
    assert_eq!(budget.record().asset_id, request.asset);
    assert_eq!(budget.record().budget_account, identity().budget_account);
    assert_eq!(budget.record().per_period_limit, request.ceiling);
    assert_eq!(budget.record().expiry, request.expiry_ms);
    drop(store);
    must(std::fs::remove_dir_all(directory));
    let requests = must(server.join());
    assert_eq!(requests.len(), 10);
    assert_eq!(requests[1]["params"], json!([rpc_hex(&raw)]));
    for index in 2..=4 {
        assert_eq!(
            requests[index]["params"],
            json!([rpc_hex(&transaction_hash(&raw))])
        );
    }
    let mut calldata = transaction_hash(b"getBudget(bytes32)")[..4].to_vec();
    calldata.extend_from_slice(&identity().budget_id);
    assert_eq!(
        requests[9]["params"],
        json!([{"to": rpc_hex(&layerx_agentd::prepare::evm::PrecompileTarget::Pay.address()), "data": rpc_hex(&calldata)}, "0x96"])
    );
}
