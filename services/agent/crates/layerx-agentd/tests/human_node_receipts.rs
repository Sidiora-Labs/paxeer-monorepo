use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::proof::ProofBundleTarget;
use layerx_agentd::human::HumanOperationError;
use layerx_agentd::human_node::HumanNode;
use layerx_agentd::human_runtime::{lookup_human_receipt_by_outbox_hash, receipt_proof_operation};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::outbox::Outbox;
use layerx_agentd::store::{Store, TenantId};
use layerx_client::rpc::receipt::Lookup;
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_types::rung::Rung;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};

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

fn signed_transaction() -> Vec<u8> {
    must(
        Eip1559Call {
            chain_id: 125,
            nonce: 41,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
            to: [7; 20],
            data: vec![1],
        }
        .sign(&[0xa5; 32]),
    )
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
    to[12..].fill(7);
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
    let mut log = vec![8; 20];
    log.push(1);
    log.extend_from_slice(&[9; 32]);
    log.extend_from_slice(&sha256(&[10, 11]));

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
    must(encoder.u64(41));
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
        "rung": "receipted", "txHash": rpc_hex(&tx_hash), "nonce": "0x29",
        "seq": "0x7", "height": "0x96", "index": "0x1", "timestamp": "0x3e8",
        "status": if success { "0x1" } else { "0x0" }, "returnData": "0xabcd",
        "logs": [{"address": rpc_hex(&[8; 20]), "topics": [rpc_hex(&[9; 32])],
            "data": "0x0a0b", "logIndex": "0x0"}],
        "gasUsed": "0x5208", "cumulativeGasUsed": "0xa410", "effectiveGasPrice": "0x1",
        "feeDenom": "upax", "feeAmount": "0x5208", "sponsor": rpc_hex(&[0; 20]),
        "from": rpc_hex(&sender), "to": rpc_hex(&[7; 20]), "contractAddress": rpc_hex(&[0; 20]),
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

fn lookup_server(
    keys: Value,
    receipt: Value,
    landing: bool,
) -> (
    Box<dyn HumanNode>,
    SequencerEndpoint,
    JoinHandle<Vec<Value>>,
) {
    let mut exchanges = vec![
        ("eth_chainId", json!("0x7d")),
        ("px_getReceiptKeys", keys),
        ("pax_sequencerReceipt", receipt.clone()),
        ("pax_sequencerReceipt", receipt),
    ];
    if landing {
        exchanges.push(("eth_getTransactionReceipt", Value::Null));
    }
    let (url, server) = serve(exchanges);
    let endpoint = must(SequencerEndpoint::new(&url, &[]));
    let node = must(NodeRpc::connect(&NodeRpcConfig {
        url,
        chain_id: 125,
        deadline: Duration::from_secs(2),
    }));
    (Box::new(node), endpoint, server)
}

fn rotated_keys(key: ReceiptKeyEntry) -> Value {
    let mut entries = published_key(key).as_array().unwrap().clone();
    let old = SigningKey::from_bytes(&[42; 32]);
    let current = SigningKey::from_bytes(&[43; 32]);
    entries.insert(
        0,
        json!({
            "keyId": rpc_hex(&[2; 32]), "publicKey": rpc_hex(&old.verifying_key().to_bytes()),
            "firstHeight": "0x0", "lastHeight": "0x63"
        }),
    );
    entries.push(json!({
        "keyId": rpc_hex(&[3; 32]), "publicKey": rpc_hex(&current.verifying_key().to_bytes()),
        "firstHeight": "0xc8", "lastHeight": "0x12c"
    }));
    Value::Array(entries)
}

#[test]
fn human_receipt_lookup_reads_pax_sequencer_receipt_by_outbox_hash() {
    let directory =
        std::env::temp_dir().join(format!("layerx-human-node-receipts-{}", std::process::id(),));
    let tenant = must(TenantId::new("human-node-receipts"));
    let raw = signed_transaction();
    let (transaction, signature) = must(decode_signed(&raw));
    let sender = must(recover_sender(&transaction, &signature));
    let hash = transaction_hash(&raw);
    let mut store = must(Store::open(&directory));
    let mut outbox = Outbox::default();
    let idempotency_key = must(outbox.enqueue(&mut store, tenant.clone(), raw.clone()));
    assert_eq!(idempotency_key, hash);
    assert_eq!(outbox.status(idempotency_key).unwrap().activity_id, hash);
    drop(store);
    let mut store = must(Store::open(&directory));
    let mut outbox = Outbox::default();
    must(outbox.restore(&store, tenant.clone(), idempotency_key));
    assert_eq!(must(outbox.exact_signed_bytes(idempotency_key)), raw);

    for success in [true, false] {
        let (response, canonical, key) = signed_receipt(hash, sender, success);
        for (first, last) in [(100, 199), (HEIGHT, HEIGHT)] {
            let key = ReceiptKeyEntry {
                first_height: first,
                last_height: last,
                ..key
            };
            let (node, endpoint, server) = lookup_server(rotated_keys(key), response.clone(), true);
            let (lookup, keys) = must(lookup_human_receipt_by_outbox_hash(
                node.as_ref(),
                &endpoint,
                &outbox,
                idempotency_key,
                hash,
            ));
            let Lookup::Verified(receipt) = lookup else {
                panic!("authenticated receipt required")
            };
            assert_eq!(receipt.activity_id(), hash);
            assert_eq!(receipt.canonical_bytes(), canonical);
            assert_eq!(receipt.height(), HEIGHT);
            assert_eq!(receipt.execution().success, success);
            assert_eq!(receipt.execution().signing_key, key.public_key);
            assert_eq!(receipt.result_code().raw(), i32::from(!success));
            assert_eq!(receipt.rung(), Rung::Receipted);
            let receipt_tenant = must(TenantId::new(if success {
                "human-receipt-success"
            } else {
                "human-receipt-failure"
            }));
            let served = must(layerx_agentd::receipt::store_verified_if_absent(
                &mut store,
                receipt_tenant.clone(),
                idempotency_key,
                receipt.canonical_bytes(),
                &receipt,
            ));
            assert_eq!(served.canonical_bytes, canonical);
            assert_eq!(served.metadata.activity_id, hash);
            assert_eq!(served.metadata.idempotency_key, idempotency_key);
            assert_eq!(served.metadata.result.code.raw(), i32::from(!success));
            for selector in [
                layerx_agentd::receipt::ReceiptLookupKey::Activity(hash),
                layerx_agentd::receipt::ReceiptLookupKey::Idempotency(idempotency_key),
                layerx_agentd::receipt::ReceiptLookupKey::GlobalSequence(receipt.global_sequence()),
            ] {
                assert_eq!(
                    must(layerx_agentd::receipt::serve(
                        &store,
                        receipt_tenant.clone(),
                        selector
                    )),
                    served
                );
            }
            assert!(layerx_agentd::receipt::store_verified_if_absent(
                &mut store,
                receipt_tenant.clone(),
                [0x88; 32],
                receipt.canonical_bytes(),
                &receipt,
            )
            .is_err());
            assert!(layerx_agentd::receipt::store_verified_if_absent(
                &mut store,
                receipt_tenant,
                idempotency_key,
                b"altered",
                &receipt,
            )
            .is_err());
            let proof = must(receipt_proof_operation(
                ProofBundleTarget::Receipt(hash),
                Rung::Receipted,
                &raw,
                receipt.canonical_bytes(),
                &keys,
                125,
                200,
                [0x55; 32],
            ));
            assert_eq!(proof.value.receipt_key, key);
            assert_eq!(proof.value.raw_transaction.as_bytes(), raw);
            assert_eq!(proof.value.signed_receipt.as_bytes(), canonical);
            assert_eq!(proof.freshness.value_height, HEIGHT);
            assert_eq!(proof.freshness.chain_head, 200);
            assert_eq!(proof.achieved_rung, Rung::Receipted);
            assert!(proof.value.commit.is_none());
            let requests = must(server.join());
            assert_eq!(requests.len(), 5);
            assert_eq!(requests[1]["params"], json!([]));
            for request in &requests[2..] {
                assert_eq!(request["params"], json!([rpc_hex(&hash)]));
            }
        }
    }

    let (response, _, key) = signed_receipt(hash, sender, true);
    for wrong_key in [
        ReceiptKeyEntry {
            last_height: HEIGHT - 1,
            ..key
        },
        ReceiptKeyEntry {
            first_height: HEIGHT + 1,
            ..key
        },
        ReceiptKeyEntry {
            public_key: SigningKey::from_bytes(&[99; 32]).verifying_key().to_bytes(),
            ..key
        },
    ] {
        let (node, endpoint, server) =
            lookup_server(rotated_keys(wrong_key), response.clone(), false);
        assert!(matches!(
            lookup_human_receipt_by_outbox_hash(
                node.as_ref(),
                &endpoint,
                &outbox,
                idempotency_key,
                hash
            ),
            Err(HumanOperationError::Refused)
        ));
        assert_eq!(must(server.join()).len(), 4);
    }

    let mut altered = response.clone();
    altered["seq"] = json!("0x8");
    let (node, endpoint, server) = lookup_server(rotated_keys(key), altered, false);
    assert!(matches!(
        lookup_human_receipt_by_outbox_hash(
            node.as_ref(),
            &endpoint,
            &outbox,
            idempotency_key,
            hash
        ),
        Err(HumanOperationError::Refused)
    ));
    assert_eq!(must(server.join()).len(), 4);

    for (id, expected) in [(idempotency_key, [0x88; 32]), ([0x88; 32], hash)] {
        let (url, server) = serve(vec![("eth_chainId", json!("0x7d"))]);
        let endpoint = must(SequencerEndpoint::new(&url, &[]));
        let node = must(NodeRpc::connect(&NodeRpcConfig {
            url,
            chain_id: 125,
            deadline: Duration::from_secs(2),
        }));
        assert!(matches!(
            lookup_human_receipt_by_outbox_hash(&node, &endpoint, &outbox, id, expected),
            Err(HumanOperationError::Refused)
        ));
        assert_eq!(must(server.join()).len(), 1);
    }

    let (node, endpoint, server) = lookup_server(rotated_keys(key), Value::Null, false);
    let (lookup, _) = must(lookup_human_receipt_by_outbox_hash(
        node.as_ref(),
        &endpoint,
        &outbox,
        idempotency_key,
        hash,
    ));
    assert_eq!(lookup, Lookup::Absent);
    assert_eq!(must(server.join()).len(), 4);
    assert_eq!(must(outbox.exact_signed_bytes(idempotency_key)), raw);
    drop(store);
    must(std::fs::remove_dir_all(directory));
}
