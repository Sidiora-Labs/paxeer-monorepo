use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::node_rpc::NodeRpcConfig;
use layerx_agentd::read::{NativeReadError, NativeReadRoute};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_programs::hex;
use layerx_types::rung::Rung;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const HEIGHT: u64 = 150;
const KEY_ID: [u8; 32] = [1; 32];

type Exchange = (&'static str, Value, Value);

fn rpc_hex(bytes: &[u8]) -> String {
    format!("0x{}", hex::encode(bytes))
}

fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn encoded_receipt(signature: Option<[u8; 64]>) -> Vec<u8> {
    let previous = [4; 32];
    let mut sequence = b"PAXEER/v1/sequence\0".to_vec();
    sequence.extend_from_slice(&previous);
    sequence.extend_from_slice(&7_u64.to_be_bytes());
    sequence.extend_from_slice(&receipt_hash());
    let mut height = [0; 32];
    height[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    let mut from = [0; 32];
    from[12..].fill(6);
    let mut to = [0; 32];
    to[12..].fill(7);
    let mut summary = Encoder::new(256);
    must(summary.u64(21_000));
    must(summary.u64(42_000));
    must(summary.u64(1));
    must(summary.u32(1));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&sha256(b"")));
    must(summary.u32(2));
    must(summary.u32(1));
    must(summary.u8(4));
    must(summary.fixed(b"upax"));
    let mut log = vec![8; 20];
    log.push(1);
    log.extend_from_slice(&[9; 32]);
    log.extend_from_slice(&sha256(&[10, 11]));

    let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
    must(encoder.structure_header_version(0x5201, PROTOCOL_VERSION));
    must(encoder.u16(PROTOCOL_VERSION));
    must(encoder.bytes(&receipt_hash(), 32));
    must(encoder.u64(7));
    must(encoder.bytes(&previous, 32));
    must(encoder.bytes(&sha256(&sequence), 32));
    must(encoder.bytes(&sha256(&[0xab, 0xcd]), 32));
    must(encoder.i32(0));
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
    must(encoder.u128(1));
    must(encoder.u64(41));
    must(encoder.bytes(&to, 32));
    must(encoder.u128(0));
    must(encoder.u128(1));
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

fn signed_receipt() -> (Value, Vec<u8>, ReceiptKeyEntry) {
    let signer = SigningKey::from_bytes(&[8; 32]);
    let digest = must(receipt_digest(&encoded_receipt(None)));
    let bytes = encoded_receipt(Some(signer.sign(&digest).to_bytes()));
    let key = ReceiptKeyEntry {
        key_id: KEY_ID,
        public_key: signer.verifying_key().to_bytes(),
        first_height: 100,
        last_height: 199,
    };
    let view = json!({
        "rung": "receipted", "txHash": rpc_hex(&receipt_hash()), "nonce": "0x29",
        "seq": "0x7", "height": "0x96", "index": "0x1", "timestamp": "0x3e8",
        "status": "0x1", "returnData": "0xabcd",
        "logs": [{"address": rpc_hex(&[8; 20]), "topics": [rpc_hex(&[9; 32])],
            "data": "0x0a0b", "logIndex": "0x0"}],
        "gasUsed": "0x5208", "cumulativeGasUsed": "0xa410", "effectiveGasPrice": "0x1",
        "feeDenom": "upax", "feeAmount": "0x5208", "sponsor": rpc_hex(&[0; 20]),
        "from": rpc_hex(&[6; 20]), "to": rpc_hex(&[7; 20]), "contractAddress": rpc_hex(&[0; 20]),
        "vmError": "", "keyId": rpc_hex(&KEY_ID), "receipt": rpc_hex(&bytes)
    });
    (view, bytes, key)
}

fn published_keys() -> Value {
    let (_, _, key) = signed_receipt();
    json!([
        {"keyId": rpc_hex(&[2; 32]),
            "publicKey": rpc_hex(SigningKey::from_bytes(&[9; 32]).verifying_key().as_bytes()),
            "firstHeight": "0x64", "lastHeight": "0xc7"},
        {"keyId": rpc_hex(&key.key_id), "publicKey": rpc_hex(&key.public_key),
            "firstHeight": "0x64", "lastHeight": "0xc7"}
    ])
}

fn receipt_exchanges() -> Vec<Exchange> {
    let (receipt, _, _) = signed_receipt();
    let hash = json!([rpc_hex(&receipt_hash())]);
    vec![
        ("px_getReceiptKeys", json!([]), published_keys()),
        ("pax_sequencerReceipt", hash.clone(), receipt.clone()),
        ("eth_getTransactionReceipt", hash.clone(), Value::Null),
        ("eth_chainId", json!([]), json!("0x7d")),
        ("px_getReceiptKeys", json!([]), published_keys()),
        ("pax_sequencerReceipt", hash.clone(), receipt),
        ("eth_getTransactionReceipt", hash, Value::Null),
        ("eth_chainId", json!([]), json!("0x7d")),
        ("px_getReceiptKeys", json!([]), published_keys()),
    ]
}

fn serve(exchanges: Vec<Exchange>) -> (NodeRpcConfig, JoinHandle<()>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let config = NodeRpcConfig {
        url: format!("http://{}/", must(listener.local_addr())),
        chain_id: 125,
        deadline: Duration::from_secs(2),
    };
    must(listener.set_nonblocking(true));
    let server = thread::spawn(move || {
        for (method, params, result) in exchanges {
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
            let length = length.expect("content length required");
            assert!(length <= 64 * 1024);
            let mut body = vec![0; length];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], params);
            let answer =
                json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
            must(stream.flush());
        }
    });
    (config, server)
}

pub fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("native JSON-RPC read: {error:?}"))
}

pub fn endpoint(config: &NodeRpcConfig) -> SequencerEndpoint {
    let certificates = if config.url.starts_with("https://") {
        let roots = rustls_native_certs::load_native_certs();
        assert!(
            roots.errors.is_empty(),
            "native TLS roots could not be loaded"
        );
        assert!(!roots.certs.is_empty(), "native TLS roots are required");
        roots.certs
    } else {
        Vec::new()
    };
    let anchors: Vec<&[u8]> = certificates.iter().map(AsRef::as_ref).collect();
    must(SequencerEndpoint::new(&config.url, &anchors))
}

pub fn live_route() -> (NativeReadRoute, NodeRpcConfig) {
    let block = json!({"number": "0x96", "hash": rpc_hex(&[5; 32]),
        "parentHash": rpc_hex(&[4; 32]), "timestamp": "0x3e8",
        "transactionsRoot": rpc_hex(&[6; 32]), "receiptsRoot": rpc_hex(&[7; 32]),
        "transactions": []});
    let mut exchanges = receipt_exchanges();
    exchanges.extend([
        ("eth_blockNumber", json!([]), json!("0x96")),
        ("eth_chainId", json!([]), json!("0x7d")),
        ("eth_blockNumber", json!([]), json!("0x96")),
        ("eth_chainId", json!([]), json!("0x7d")),
        ("eth_blockNumber", json!([]), json!("0x96")),
        ("eth_getBlockByNumber", json!(["0x96", true]), block.clone()),
        ("eth_getBlockByNumber", json!(["0x96", true]), block),
        ("eth_chainId", json!([]), json!("0x7d")),
        ("px_getHistory", json!([rpc_hex(&receipt_hash()), null, 2]),
            json!({"items": [], "next_cursor": null, "indexedHeight": HEIGHT, "chainHeight": HEIGHT})),
    ]);
    let (config, _server) = serve(exchanges);
    let route = NativeReadRoute::new(endpoint(&config), config.chain_id);
    (route, config)
}

pub fn receipt_hash() -> [u8; 32] {
    [3; 32]
}

pub fn assert_receipt(route: &mut NativeReadRoute, config: &NodeRpcConfig) {
    let hash = receipt_hash();
    let client = endpoint(config);
    let keys: Vec<ReceiptKeyEntry> = must(client.receipt_keys());
    let Lookup::Verified(expected) = must(lookup_authenticated(
        &client,
        hash,
        LookupContext {
            receipt_keys: &keys,
        },
    )) else {
        panic!("the local transaction must carry a signed sequencer receipt");
    };
    assert_eq!(
        expected.rung(),
        Rung::Receipted,
        "the acceptance transaction must still be at the receipted rung"
    );
    assert_eq!(expected.canonical_bytes(), signed_receipt().1);
    let value = must(route.read(&format!("/v1/reads/receipt/{}", hex::encode(&hash))));
    assert_eq!(value["receipt"], hex::encode(expected.canonical_bytes()));
    assert_eq!(value["transaction_hash"], hex::encode(&hash));
    assert_eq!(value["rung"], "receipted");
    let index = usize::try_from(
        value["key_index"]
            .as_u64()
            .expect("receipt signing key index"),
    )
    .expect("representable key index");
    assert_eq!(index, 1, "select the signing key from the published set");
    let key = keys.get(index).expect("index in px_getReceiptKeys");
    assert_eq!(key.key_id, expected.execution().key_id);
    assert!(key.first_height <= expected.height());
    assert!(expected.height() <= key.last_height);
    assert_eq!(
        value["receipt_key"]["public_key"],
        hex::encode(&key.public_key)
    );
    assert_eq!(value["height"], expected.height());
    assert_eq!(must(route.signed_authority()), keys);
}

#[test]
fn native_receipt_route_serves_a_signed_receipt() {
    let (config, server) = serve(receipt_exchanges());
    let mut route = NativeReadRoute::new(endpoint(&config), config.chain_id);
    assert_receipt(&mut route, &config);
    must(server.join());
}

#[test]
fn native_route_rejects_an_unknown_kind() {
    let (config, server) = serve(vec![
        ("eth_chainId", json!([]), json!("0x7d")),
        ("px_getReceiptKeys", json!([]), published_keys()),
    ]);
    let mut route = NativeReadRoute::new(endpoint(&config), config.chain_id);
    for path in [
        "/v1/reads/batch/1",
        "/v1/reads/unknown/1",
        "/v1/reads/receipt",
        "/reads/receipt/1",
        "/v1/reads/receipt/not-a-hash",
    ] {
        assert!(
            matches!(route.read(path), Err(NativeReadError::InvalidRequest)),
            "malformed or unknown native route must refuse: {path}"
        );
    }
    assert_eq!(must(route.signed_authority()).len(), 2);
    must(server.join());
}
