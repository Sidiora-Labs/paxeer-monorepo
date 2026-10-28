use std::fmt::Write as _;
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::thread::JoinHandle;
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::ai_market::{AiClientError, OperationJournal, OperationRecord, OperationState};
use layerx_client::rpc::receipt::{LookupContext, ReceiptError};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::evm_transaction::{self, transaction_hash, Eip1559Call};
use layerx_types::payload::ModuleRegistry;
use layerx_types::rung::Rung;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const HEIGHT: u64 = 150;
const KEY_ID: [u8; 32] = [1; 32];
static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}

struct Scratch(PathBuf);

impl Scratch {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "ai-market-rpc-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ));
        must(fs::create_dir(&path));
        Self(path)
    }
}

impl Drop for Scratch {
    fn drop(&mut self) {
        must(fs::remove_dir_all(&self.0));
    }
}

fn registry() -> ModuleRegistry {
    must(ModuleRegistry::new(&[]))
}

fn signed_transaction(nonce: u64) -> Vec<u8> {
    must(
        Eip1559Call {
            chain_id: 125,
            nonce,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 1,
            gas_limit: 90_000,
            to: [7; 20],
            data: vec![0, 0x12, 0xab, 0xff],
        }
        .sign(&[0x46; 32]),
    )
}

fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().fold(String::from("0x"), |mut text, byte| {
        must(write!(text, "{byte:02x}"));
        text
    })
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
        "rung": "receipted", "txHash": hex(&tx_hash), "nonce": "0x29",
        "seq": "0x7", "height": "0x96", "index": "0x1", "timestamp": "0x3e8",
        "status": if success { "0x1" } else { "0x0" }, "returnData": "0xabcd",
        "logs": [{"address": hex(&[8; 20]), "topics": [hex(&[9; 32])],
            "data": "0x0a0b", "logIndex": "0x0"}],
        "gasUsed": "0x5208", "cumulativeGasUsed": "0xa410", "effectiveGasPrice": "0x1",
        "feeDenom": "upax", "feeAmount": "0x5208", "sponsor": hex(&[0; 20]),
        "from": hex(&sender), "to": hex(&[7; 20]), "contractAddress": hex(&[0; 20]),
        "vmError": if success { "" } else { "execution reverted" },
        "keyId": hex(&KEY_ID), "receipt": hex(&bytes)
    });
    (view, bytes, key)
}

type Exchange = (&'static str, Value, Option<Value>);

fn serve(
    exchanges: Vec<Exchange>,
    path: PathBuf,
) -> (SequencerEndpoint, JoinHandle<Vec<OperationRecord>>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    must(listener.set_nonblocking(true));
    let url = format!("http://{}/", must(listener.local_addr()));
    let handle = std::thread::spawn(move || {
        let mut observed = Vec::new();
        for (index, (method, params, response)) in exchanges.into_iter().enumerate() {
            let deadline = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(
                            Instant::now() < deadline,
                            "missing RPC request {index}: {method}"
                        );
                        std::thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            must(stream.set_read_timeout(Some(Duration::from_secs(5))));
            must(stream.set_write_timeout(Some(Duration::from_secs(5))));
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
            let mut body = vec![0; length.unwrap_or_else(|| panic!("missing content length"))];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], params);
            assert_eq!(request["id"], json!(index + 1));
            observed.push(must(OperationRecord::decode(
                &must(fs::read(&path)),
                &registry(),
            )));
            if let Some(result) = response {
                let answer =
                    json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
                let mut stream = reader.into_inner();
                must(write!(stream,
                    "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()
                ));
                must(stream.flush());
            }
        }
        assert!(
            matches!(listener.accept(), Err(error) if error.kind() == std::io::ErrorKind::WouldBlock),
            "unexpected extra RPC request"
        );
        observed
    });
    (must(SequencerEndpoint::new(&url, &[])), handle)
}

fn receipt_exchange(tx_hash: [u8; 32], view: Value) -> Exchange {
    ("pax_sequencerReceipt", json!([hex(&tx_hash)]), Some(view))
}

fn landing_exchange(tx_hash: [u8; 32]) -> Exchange {
    (
        "eth_getTransactionReceipt",
        json!([hex(&tx_hash)]),
        Some(Value::Null),
    )
}

#[test]
fn operation_journal_submit_sends_the_exact_raw_bytes_once() {
    let scratch = Scratch::new();
    let journal = must(OperationJournal::open(&scratch.0));
    let raw = signed_transaction(41);
    let mut record = must(OperationRecord::from_signed_transaction(
        raw.clone(),
        [0x81; 32],
        200,
    ));
    let id = transaction_hash(&raw);
    assert_eq!(record.activity_id(), id);
    assert_eq!(record.rung(), None);
    must(journal.record_signed(&record));
    assert_eq!(must(journal.load(&id, &registry())), record);
    let mut corrupted = must(record.encode());
    let last = corrupted.len() - 1;
    corrupted[last] ^= 1;
    assert!(OperationRecord::decode(&corrupted, &registry()).is_err());

    let (endpoint, server) = serve(
        vec![(
            "eth_sendRawTransaction",
            json!([hex(&raw)]),
            Some(json!(hex(&id))),
        )],
        journal.record_path(&id),
    );
    assert_eq!(
        must(journal.submit(&mut record, &endpoint)),
        OperationState::Pending
    );
    assert_eq!(record.rung(), Some(Rung::Submitted));
    assert_eq!(record.attempt(), 1);
    assert_eq!(record.signed_bytes(), raw);
    assert_eq!(must(journal.load(&id, &registry())), record);
    assert_eq!(
        journal.submit(&mut record, &endpoint),
        Err(AiClientError::InvalidTransition {
            from: OperationState::Pending
        })
    );
    assert_eq!(
        journal.resend_exact(&mut record, &endpoint),
        Err(AiClientError::InvalidTransition {
            from: OperationState::Pending
        })
    );
    let observed = must(server.join());
    assert_eq!(observed.len(), 1);
    assert_eq!(observed[0].state(), OperationState::Submitting);
    assert_eq!(observed[0].attempt(), 1);
    assert_eq!(observed[0].signed_bytes(), raw);
    // An interrupted submission recovers durably and is resent only by explicit request.
    must(fs::write(
        journal.record_path(&id),
        must(observed[0].encode()),
    ));
    record = must(journal.load(&id, &registry()));
    assert_eq!(record.state(), OperationState::Unknown);
    assert_eq!(must(journal.load(&id, &registry())), record);
    let (endpoint, server) = serve(
        vec![
            ("eth_sendRawTransaction", json!([hex(&raw)]), None),
            (
                "eth_sendRawTransaction",
                json!([hex(&raw)]),
                Some(json!(hex(&id))),
            ),
        ],
        journal.record_path(&id),
    );
    assert_eq!(
        must(journal.resend_exact(&mut record, &endpoint)),
        OperationState::Unknown
    );
    assert_eq!(record.attempt(), 2);
    assert_eq!(must(journal.load(&id, &registry())), record);
    assert_eq!(
        must(journal.resend_exact(&mut record, &endpoint)),
        OperationState::Pending
    );
    assert_eq!(record.attempt(), 3);
    assert_eq!(record.signed_bytes(), raw);
    assert_eq!(must(journal.load(&id, &registry())), record);
    let observed = must(server.join());
    assert_eq!(observed.len(), 2);
    for (index, stored) in observed.iter().enumerate() {
        assert_eq!(stored.state(), OperationState::Submitting);
        assert_eq!(stored.attempt(), must(u32::try_from(index)) + 2);
        assert_eq!(stored.signed_bytes(), raw);
    }
    assert!(OperationRecord::from_signed_transaction(vec![2, 0], [0; 32], 200).is_err());
}

#[test]
fn operation_journal_resolve_records_the_receipted_rung() {
    for success in [true, false] {
        let scratch = Scratch::new();
        let journal = must(OperationJournal::open(&scratch.0));
        let raw = signed_transaction(41);
        let id = transaction_hash(&raw);
        let (call, signature) = must(evm_transaction::decode_signed(&raw));
        let sender = must(evm_transaction::recover_sender(&call, &signature));
        let mut record = must(OperationRecord::from_signed_transaction(
            raw.clone(),
            [0x82; 32],
            200,
        ));
        must(journal.record_signed(&record));
        let (view, _, key) = signed_receipt(id, sender, success);
        let keys = [key];
        let context = LookupContext {
            receipt_keys: &keys,
        };
        let (endpoint, server) = serve(
            vec![
                (
                    "eth_sendRawTransaction",
                    json!([hex(&raw)]),
                    if success { Some(json!(hex(&id))) } else { None },
                ),
                receipt_exchange(id, Value::Null),
                receipt_exchange(
                    id,
                    json!({"rung": "submitted", "txHash": hex(&id), "nonce": "0x29"}),
                ),
                receipt_exchange(id, view.clone()),
                receipt_exchange(id, view),
                landing_exchange(id),
            ],
            journal.record_path(&id),
        );
        let initial = if success {
            OperationState::Pending
        } else {
            OperationState::Unknown
        };
        assert_eq!(must(journal.submit(&mut record, &endpoint)), initial);
        let unresolved = record.clone();
        for _ in 0..2 {
            assert_eq!(
                must(journal.resolve(&mut record, &endpoint, context)),
                initial
            );
            assert_eq!(record, unresolved);
            assert_eq!(must(journal.load(&id, &registry())), unresolved);
        }
        assert!(matches!(
            journal.resolve(&mut record, &endpoint, LookupContext { receipt_keys: &[] }),
            Err(AiClientError::Receipt(ReceiptError::Verification(_)))
        ));
        assert_eq!(record, unresolved);
        assert_eq!(must(journal.load(&id, &registry())), unresolved);
        let expected = if success {
            OperationState::Executed
        } else {
            OperationState::Failed
        };
        assert_eq!(
            must(journal.resolve(&mut record, &endpoint, context)),
            expected
        );
        assert_eq!(record.rung(), Some(Rung::Receipted));
        assert_eq!(record.result_code(), Some(i32::from(!success)));
        assert_eq!(record.global_sequence(), Some(7));
        assert_eq!(record.signed_bytes(), raw);
        assert_eq!(record.attempt(), 1);
        assert_eq!(record.checkpoint(), None);
        assert_eq!(must(journal.load(&id, &registry())), record);
        let observed = must(server.join());
        assert_eq!(observed.len(), 6);
        assert_eq!(observed[0].state(), OperationState::Submitting);
        for stored in &observed[1..] {
            assert_eq!(stored, &unresolved);
        }
    }
}
