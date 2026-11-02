use std::fmt::Debug;
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};

use layerx_agentd::admin::{OperatorContext, Surface};
use layerx_agentd::outbox::{
    resolve_unknown, Outbox, ResendObservation, ResolutionObservation, SubmissionState,
    UnknownResolutionError,
};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn tenant() -> TenantId {
    must(TenantId::new("unknown-receipt-lookup"))
}

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
        Self(std::env::temp_dir().join(format!(
            "layerx-unknown-receipt-{}-{sequence}",
            std::process::id()
        )))
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        if self.0.exists() {
            must(fs::remove_dir_all(&self.0));
        }
    }
}

fn protected(path: &Path) -> Vec<u8> {
    let metadata = must(fs::symlink_metadata(path));
    assert!(metadata.is_file());
    assert!(metadata.len() > 0 && metadata.len() <= 1_048_576);
    assert_eq!(metadata.permissions().mode() & 0o077, 0);
    must(fs::read(path))
}

fn fixture() -> (Value, SequencerEndpoint) {
    let path = std::env::var_os("PAXEER_X_UNKNOWN_RECEIPT_LOOKUP_FIXTURE").unwrap_or_else(|| {
        panic!("owned real sequencer fixture required: PAXEER_X_UNKNOWN_RECEIPT_LOOKUP_FIXTURE")
    });
    let fixture: Value = must(serde_json::from_slice(&protected(Path::new(&path))));
    assert_eq!(fixture["schema"], "paxeer-x.unknown-receipt-lookup.v1");
    let url = fixture["rpc_url"]
        .as_str()
        .unwrap_or_else(|| panic!("real sequencer RPC URL required"));
    let ca = fixture["ca_der_file"]
        .as_str()
        .map(|path| protected(Path::new(path)));
    let anchors: Vec<&[u8]> = ca.iter().map(Vec::as_slice).collect();
    let endpoint = must(SequencerEndpoint::new(url, &anchors));
    assert_eq!(must(endpoint.chain_id()), 125);
    (fixture, endpoint)
}

fn raw(fixture: &Value, field: &str) -> Vec<u8> {
    let encoded = fixture[field]
        .as_str()
        .unwrap_or_else(|| panic!("real signed transaction required: {field}"));
    must(hex::decode(encoded.strip_prefix("0x").unwrap_or(encoded)))
}

fn unknown(store: &mut Store, raw: Vec<u8>) -> (Outbox, [u8; 32]) {
    let mut outbox = Outbox::default();
    let hash = must(outbox.enqueue(store, tenant(), raw));
    must(outbox.transition(
        store,
        hash,
        SubmissionState::Submitted,
        "send attempted",
        None,
    ));
    must(outbox.transition(
        store,
        hash,
        SubmissionState::Unknown,
        "transport interrupted",
        None,
    ));
    (outbox, hash)
}

fn state_key(hash: [u8; 32]) -> TenantKey {
    let mut id = b"unknown-resolution:".to_vec();
    id.extend_from_slice(&hash);
    must(TenantKey::new(tenant(), ObjectKind::Configuration, id))
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
        "keyId": rpc_hex(&KEY_ID), "receipt": rpc_hex(&bytes)
    });
    (view, bytes, key)
}

type Exchange = (&'static str, Value, Value);

fn serve(exchanges: Vec<Exchange>) -> (SequencerEndpoint, JoinHandle<()>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    must(listener.set_nonblocking(true));
    let server = thread::spawn(move || {
        for (method, params, result) in exchanges {
            let deadline = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing {method} request");
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
            assert_eq!(request["params"], params);
            let answer =
                json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
            must(stream.flush());
        }
    });
    (must(SequencerEndpoint::new(&url, &[])), server)
}

fn published_key(key: ReceiptKeyEntry) -> Value {
    json!([{
        "keyId": rpc_hex(&key.key_id),
        "publicKey": rpc_hex(&key.public_key),
        "firstHeight": format!("{:#x}", key.first_height),
        "lastHeight": format!("{:#x}", key.last_height),
    }])
}

#[test]
fn unknown_submission_resolves_to_executed_from_a_signed_receipt() {
    let raw = signed_transaction();
    let (transaction, signature) = must(decode_signed(&raw));
    let sender = must(recover_sender(&transaction, &signature));
    let hash = transaction_hash(&raw);
    let (receipt, _, key) = signed_receipt(hash, sender, true);
    let hash_params = json!([rpc_hex(&hash)]);
    let (endpoint, server) = serve(vec![
        ("px_getReceiptKeys", json!([]), published_key(key)),
        ("pax_sequencerReceipt", hash_params.clone(), receipt.clone()),
        (
            "eth_getTransactionReceipt",
            hash_params.clone(),
            Value::Null,
        ),
        ("pax_sequencerReceipt", hash_params.clone(), receipt.clone()),
        ("pax_sequencerReceipt", hash_params.clone(), receipt),
        ("eth_getTransactionReceipt", hash_params, Value::Null),
    ]);
    assert_receipted_resolution(raw, endpoint);
    must(server.join());
}

#[test]
fn unknown_submission_with_consumed_nonce_fails() {
    let raw = signed_transaction();
    let (transaction, signature) = must(decode_signed(&raw));
    let sender = must(recover_sender(&transaction, &signature));
    let hash = transaction_hash(&raw);
    let (_, _, key) = signed_receipt(hash, sender, true);
    let hash_params = json!([rpc_hex(&hash)]);
    let nonce_params = json!([rpc_hex(&sender), "pending"]);
    let pending_nonce = json!(format!("{:#x}", transaction.nonce + 1));
    let (endpoint, server) = serve(vec![
        ("pax_sequencerReceipt", hash_params.clone(), Value::Null),
        (
            "eth_getTransactionCount",
            nonce_params.clone(),
            pending_nonce.clone(),
        ),
        ("px_getReceiptKeys", json!([]), published_key(key)),
        ("pax_sequencerReceipt", hash_params, Value::Null),
        ("eth_getTransactionCount", nonce_params, pending_nonce),
    ]);
    assert_consumed_nonce_resolution(raw, endpoint);
    must(server.join());
}

#[test]
fn real_unknown_submission_resolves_against_an_owned_sequencer() {
    let (fixture, endpoint) = fixture();
    assert_receipted_resolution(raw(&fixture, "receipted_raw_transaction"), endpoint);
    let (fixture, endpoint) = self::fixture();
    assert_consumed_nonce_resolution(raw(&fixture, "consumed_nonce_raw_transaction"), endpoint);
}

fn assert_receipted_resolution(raw: Vec<u8>, mut endpoint: SequencerEndpoint) {
    let (transaction, _) = must(decode_signed(&raw));
    let directory = Directory::new();
    let mut store = must(Store::open(&directory.0));
    let (mut outbox, hash) = unknown(&mut store, raw.clone());
    assert_eq!(hash, transaction_hash(&raw));
    let keys = must(endpoint.receipt_keys());
    assert!(!keys.is_empty(), "published receipt keys required");
    let Lookup::Verified(receipt) = must(lookup_authenticated(
        &endpoint,
        hash,
        LookupContext {
            receipt_keys: &keys,
        },
    )) else {
        panic!("fixture transaction must have a signed sequencer receipt");
    };
    assert!(receipt.execution().success);

    let mut surface = must(Surface::open(&directory.0, &tenant()));
    let context = must(OperatorContext::new("receipt-operator", [1; 32]));
    let inspection = must(surface.inspect_unknown(&context, &outbox, hash));
    assert_eq!(inspection.transaction_hash, hash);
    assert_eq!(inspection.nonce, transaction.nonce);
    assert_eq!(inspection.state, SubmissionState::Unknown);

    let unverified = must(surface.resolve_unknown(
        &context,
        &mut outbox,
        &mut store,
        hash,
        1_000,
        &[],
        &mut endpoint,
    ));
    assert_eq!(unverified.state, SubmissionState::Unknown);
    assert_eq!(
        unverified.observation,
        ResolutionObservation::UnverifiedReceipt
    );
    assert_eq!(unverified.resend, ResendObservation::NotWarranted);
    assert!(outbox
        .status(hash)
        .unwrap_or_else(|| panic!("unknown status missing"))
        .evidence
        .is_none());
    let saved = store
        .get(&state_key(hash))
        .unwrap_or_else(|| panic!("retry state missing"))
        .bytes();
    assert_eq!(&saved[..4], b"LXU2");
    assert_eq!(&saved[4..36], &hash);
    assert_eq!(saved.len(), 56);
    let before_backoff = must(resolve_unknown(
        &mut outbox,
        &mut store,
        hash,
        1_000,
        &keys,
        &mut endpoint,
    ));
    assert_eq!(before_backoff.observation, ResolutionObservation::Backoff);
    assert_eq!(before_backoff.age.attempt_count, 1);

    drop(store);
    let mut store = must(Store::open(&directory.0));
    let mut outbox = Outbox::default();
    must(outbox.restore(&store, tenant(), hash));
    let resolved = must(surface.resolve_unknown(
        &context,
        &mut outbox,
        &mut store,
        hash,
        unverified.age.next_attempt_at_ms,
        &keys,
        &mut endpoint,
    ));
    assert_eq!(resolved.state, SubmissionState::Executed);
    assert_eq!(resolved.observation, ResolutionObservation::ExecutedReceipt);
    assert_eq!(resolved.resend, ResendObservation::NotWarranted);
    assert_eq!(resolved.age.attempt_count, 2);
    assert!(store.get(&state_key(hash)).is_none());
    let evidence = outbox
        .status(hash)
        .and_then(|status| status.evidence)
        .unwrap_or_else(|| panic!("verified execution evidence missing"));
    assert_eq!(evidence.block_height(), receipt.height());
    assert_eq!(evidence.transaction_index(), receipt.execution().index);
    let receipt_ref: [u8; 32] = Sha256::digest(receipt.canonical_bytes()).into();
    assert_eq!(evidence.receipt_ref(), receipt_ref);
    let receipt_key = must(TenantKey::new(
        tenant(),
        ObjectKind::Receipt,
        receipt_ref.to_vec(),
    ));
    assert_eq!(
        store
            .get(&receipt_key)
            .unwrap_or_else(|| panic!("signed receipt not retained"))
            .bytes(),
        receipt.canonical_bytes()
    );
    assert_eq!(must(outbox.exact_signed_bytes(hash)), raw);
    let mut restored = Outbox::default();
    must(restored.restore(&store, tenant(), hash));
    assert_eq!(restored.status(hash), outbox.status(hash));
}

fn assert_consumed_nonce_resolution(raw: Vec<u8>, mut endpoint: SequencerEndpoint) {
    let (transaction, signature) = must(decode_signed(&raw));
    let sender = must(recover_sender(&transaction, &signature));
    let hash = transaction_hash(&raw);
    assert_eq!(must(endpoint.sequencer_receipt(&hash)), None);
    assert!(must(endpoint.pending_nonce(&sender)) > transaction.nonce);
    let directory = Directory::new();
    let mut store = must(Store::open(&directory.0));
    let (mut outbox, stored_hash) = unknown(&mut store, raw.clone());
    assert_eq!(stored_hash, hash);
    let keys = must(endpoint.receipt_keys());
    let result = must(resolve_unknown(
        &mut outbox,
        &mut store,
        hash,
        1_000,
        &keys,
        &mut endpoint,
    ));
    assert_eq!(result.state, SubmissionState::Failed);
    assert_eq!(result.observation, ResolutionObservation::NonceConsumed);
    assert_eq!(result.resend, ResendObservation::NotWarranted);
    let status = outbox
        .status(hash)
        .unwrap_or_else(|| panic!("failed status missing"));
    assert_eq!(status.state, SubmissionState::Failed);
    assert!(status.evidence.is_none());
    assert_eq!(
        status
            .transitions
            .last()
            .map(|transition| transition.cause.as_str()),
        Some("submission.nonce_consumed")
    );
    assert!(store.get(&state_key(hash)).is_none());
    assert_eq!(must(outbox.exact_signed_bytes(hash)), raw);
    drop(store);
    let store = must(Store::open(&directory.0));
    let mut restored = Outbox::default();
    must(restored.restore(&store, tenant(), hash));
    assert_eq!(restored.status(hash), outbox.status(hash));
}

#[test]
fn legacy_lxur_resolution_state_is_refused() {
    let raw = must(
        Eip1559Call {
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
            to: [0x22; 20],
            data: vec![1],
        }
        .sign(&[0xa5; 32]),
    );
    let directory = Directory::new();
    let mut store = must(Store::open(&directory.0));
    let (outbox, hash) = unknown(&mut store, raw);
    let original = outbox.status(hash).cloned();
    let mut legacy = b"LXUR".to_vec();
    legacy.extend_from_slice(&1_000_u64.to_be_bytes());
    legacy.extend_from_slice(&1_u32.to_be_bytes());
    legacy.extend_from_slice(&2_000_u64.to_be_bytes());
    must(store.put_local(state_key(hash), legacy.clone()));
    drop(store);
    let mut store = must(Store::open(&directory.0));
    let mut outbox = Outbox::default();
    must(outbox.restore(&store, tenant(), hash));
    // No node request is needed to refuse an incompatible durable state.
    let mut endpoint = must(SequencerEndpoint::new("http://127.0.0.1:1", &[]));
    let error = resolve_unknown(&mut outbox, &mut store, hash, 1_500, &[], &mut endpoint)
        .err()
        .unwrap_or_else(|| panic!("legacy resolution state was accepted"));
    assert!(matches!(error, UnknownResolutionError::StateVersion));
    assert_eq!(error.reason(), "unknown.state_version");
    assert_eq!(outbox.status(hash), original.as_ref());
    assert_eq!(
        store
            .get(&state_key(hash))
            .unwrap_or_else(|| panic!("legacy state was rewritten"))
            .bytes(),
        legacy
    );
}
