// Receipt fixtures bind paymentDenom to the signed protocol asset.
#[path = "support/mod.rs"]
mod support;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::budget::{
    reserve, BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest,
};
use layerx_agentd::capability::{consume, Ceiling, CeilingError};
use layerx_agentd::outbox::{
    resolve_unknown, Outbox, ResendObservation, ResolutionObservation, SubmissionState,
};
use layerx_agentd::store::Store;
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use support::{directory, rpc_hex, serve, tenant};

fn unknown_outbox(root: &std::path::Path, id: u8) -> (Store, Outbox, Vec<u8>, [u8; 32]) {
    let exact = support::signed_transaction(id);
    let mut store = Store::open(root).unwrap_or_else(|error| panic!("store: {error}"));
    let mut outbox = Outbox::default();
    let hash = outbox
        .enqueue(&mut store, tenant(), exact.clone())
        .unwrap_or_else(|error| panic!("enqueue: {error:?}"));
    outbox
        .transition(
            &mut store,
            hash,
            SubmissionState::Submitted,
            "transport started",
            None,
        )
        .unwrap_or_else(|error| panic!("submit: {error:?}"));
    outbox
        .transition(
            &mut store,
            hash,
            SubmissionState::Unknown,
            "transport outcome indeterminate",
            None,
        )
        .unwrap_or_else(|error| panic!("unknown: {error:?}"));
    (store, outbox, exact, hash)
}

#[test]
fn acknowledgement_loss_is_resolved_only_by_the_existing_receipt() {
    let root = directory("ack-loss");
    let (mut store, mut outbox, exact, hash) = unknown_outbox(&root, 1);
    let (receipt, key) = sequencer_receipt(&exact, true);
    let (mut endpoint, server) = serve(vec![
        (
            "pax_sequencerReceipt",
            json!([rpc_hex(&hash)]),
            Some(receipt),
        ),
        (
            "eth_getTransactionReceipt",
            json!([rpc_hex(&hash)]),
            Some(Value::Null),
        ),
    ]);

    let result = resolve_unknown(&mut outbox, &mut store, hash, 10_000, &[key], &mut endpoint)
        .unwrap_or_else(|error| panic!("resolve: {error:?}"));
    let node = server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    assert_eq!(result.state, SubmissionState::Executed);
    assert_eq!(result.observation, ResolutionObservation::ExecutedReceipt);
    assert_eq!(result.resend, ResendObservation::NotWarranted);
    assert!(node.transmitted.is_empty());
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn lost_resend_response_keeps_budget_held_while_unreconciled_ceiling_refuses_admission() {
    let root = directory("response-loss");
    let (mut store, mut outbox, exact, hash) = unknown_outbox(&root, 2);
    let limit_id = LimitId([7; 16]);
    let limiter = BudgetLimiter::new(vec![LimitConfig {
        id: limit_id,
        name: "tenant spend".to_owned(),
        scope: LimitScope::Tenant([8; 32]),
        ceiling: 1_000,
        consumed: 0,
    }])
    .unwrap_or_else(|error| panic!("limiter: {error:?}"));
    reserve(
        &limiter,
        &ReservationRequest {
            id: [2; 32],
            amount: 400,
            expiry_sequence: 5,
            current_sequence: 1,
            applicable_limits: vec![limit_id],
        },
    )
    .unwrap_or_else(|error| panic!("budget reserve: {error:?}"));
    let ceiling = Ceiling::new(1_000, support::pinned_evidence_verifier());
    assert_eq!(
        consume(&ceiling, [2; 32], [2; 32], 400, 5, 1),
        Err(CeilingError::Unreconciled)
    );

    let (late_receipt, key) = sequencer_receipt(&exact, true);
    let (transaction, signature) =
        decode_signed(&exact).unwrap_or_else(|error| panic!("transaction: {error:?}"));
    let sender = recover_sender(&transaction, &signature)
        .unwrap_or_else(|error| panic!("sender: {error:?}"));
    let (mut endpoint, server) = serve(vec![
        (
            "pax_sequencerReceipt",
            json!([rpc_hex(&hash)]),
            Some(Value::Null),
        ),
        (
            "eth_getTransactionCount",
            json!([rpc_hex(&sender), "pending"]),
            Some(json!(format!("{:#x}", transaction.nonce))),
        ),
        ("eth_sendRawTransaction", json!([rpc_hex(&exact)]), None),
    ]);
    let first = resolve_unknown(&mut outbox, &mut store, hash, 20_000, &[key], &mut endpoint)
        .unwrap_or_else(|error| panic!("first resolve: {error:?}"));
    let node = server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    assert_eq!(first.state, SubmissionState::Unknown);
    assert_eq!(first.observation, ResolutionObservation::ReceiptMissing);
    assert_eq!(first.resend, ResendObservation::Indeterminate);
    assert_eq!(node.transmitted, vec![(hash, exact)]);
    assert_eq!(limiter.held_reservations(), Ok(1));
    assert_eq!(ceiling.release_expired(50), Ok(0));
    assert_eq!(
        ceiling
            .snapshot()
            .map(|value| (value.held, value.reconciled)),
        Ok((0, false))
    );

    let (mut endpoint, server) = serve(vec![
        (
            "pax_sequencerReceipt",
            json!([rpc_hex(&hash)]),
            Some(late_receipt),
        ),
        (
            "eth_getTransactionReceipt",
            json!([rpc_hex(&hash)]),
            Some(Value::Null),
        ),
    ]);
    let second = resolve_unknown(
        &mut outbox,
        &mut store,
        hash,
        first.age.next_attempt_at_ms,
        &[key],
        &mut endpoint,
    )
    .unwrap_or_else(|error| panic!("second resolve: {error:?}"));
    server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    assert_eq!(second.state, SubmissionState::Executed);
    assert_eq!(second.age.attempt_count, 2);
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn restart_preserves_backoff_age_and_a_receipt_can_appear_minutes_later() {
    let root = directory("restart");
    let (mut store, mut outbox, exact, hash) = unknown_outbox(&root, 3);
    let (receipt, key) = sequencer_receipt(&exact, false);
    let (transaction, signature) =
        decode_signed(&exact).unwrap_or_else(|error| panic!("transaction: {error:?}"));
    let sender = recover_sender(&transaction, &signature)
        .unwrap_or_else(|error| panic!("sender: {error:?}"));
    let (mut endpoint, server) = serve(vec![
        (
            "pax_sequencerReceipt",
            json!([rpc_hex(&hash)]),
            Some(Value::Null),
        ),
        (
            "eth_getTransactionCount",
            json!([rpc_hex(&sender), "pending"]),
            Some(json!(format!("{:#x}", transaction.nonce))),
        ),
        (
            "eth_sendRawTransaction",
            json!([rpc_hex(&exact)]),
            Some(json!(rpc_hex(&hash))),
        ),
    ]);
    let first = resolve_unknown(&mut outbox, &mut store, hash, 30_000, &[key], &mut endpoint)
        .unwrap_or_else(|error| panic!("first resolve: {error:?}"));
    let node = server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    assert_eq!(first.age.attempt_count, 1);
    assert_eq!(node.transmitted, vec![(hash, exact)]);
    drop(outbox);
    drop(store);

    let mut reopened = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
    let mut restored = Outbox::default();
    restored
        .restore(&reopened, tenant(), hash)
        .unwrap_or_else(|error| panic!("restore: {error:?}"));
    let deferred = resolve_unknown(
        &mut restored,
        &mut reopened,
        hash,
        first.age.next_attempt_at_ms - 1,
        &[key],
        &mut endpoint,
    )
    .unwrap_or_else(|error| panic!("deferred: {error:?}"));
    assert_eq!(deferred.observation, ResolutionObservation::Backoff);
    assert_eq!(deferred.age.attempt_count, 1);

    let (mut endpoint, server) = serve(vec![
        (
            "pax_sequencerReceipt",
            json!([rpc_hex(&hash)]),
            Some(receipt),
        ),
        (
            "eth_getTransactionReceipt",
            json!([rpc_hex(&hash)]),
            Some(Value::Null),
        ),
    ]);
    let later = resolve_unknown(
        &mut restored,
        &mut reopened,
        hash,
        210_000,
        &[key],
        &mut endpoint,
    )
    .unwrap_or_else(|error| panic!("later: {error:?}"));
    server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    assert_eq!(later.state, SubmissionState::Failed);
    assert_eq!(later.age.age_ms, 180_000);
    assert_eq!(later.age.attempt_count, 2);
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn lookup_failure_and_unverified_receipt_never_infer_a_terminal_outcome() {
    let root = directory("lookup-loss");
    let (mut store, mut outbox, exact, hash) = unknown_outbox(&root, 4);
    let (mut raw, key) = sequencer_receipt(&exact, true);
    let (mut endpoint, server) = serve(vec![(
        "pax_sequencerReceipt",
        json!([rpc_hex(&hash)]),
        None,
    )]);
    let first = resolve_unknown(&mut outbox, &mut store, hash, 40_000, &[key], &mut endpoint)
        .unwrap_or_else(|error| panic!("lookup loss: {error:?}"));
    assert_eq!(first.state, SubmissionState::Unknown);
    let node = server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    assert_eq!(first.observation, ResolutionObservation::LookupUnavailable);
    assert!(node.transmitted.is_empty());

    let encoded = raw["receipt"]
        .as_str()
        .unwrap_or_else(|| panic!("receipt required"));
    let mut corrupt = hex::decode(encoded.strip_prefix("0x").unwrap_or(encoded))
        .unwrap_or_else(|error| panic!("receipt: {error:?}"));
    corrupt[0] ^= 1;
    raw["receipt"] = json!(rpc_hex(&corrupt));
    let (mut endpoint, server) = serve(vec![(
        "pax_sequencerReceipt",
        json!([rpc_hex(&hash)]),
        Some(raw),
    )]);
    let second = resolve_unknown(
        &mut outbox,
        &mut store,
        hash,
        first.age.next_attempt_at_ms,
        &[key],
        &mut endpoint,
    )
    .unwrap_or_else(|error| panic!("unverified: {error:?}"));
    assert_eq!(second.state, SubmissionState::Unknown);
    assert_eq!(second.observation, ResolutionObservation::UnverifiedReceipt);
    server
        .join()
        .unwrap_or_else(|error| panic!("server: {error:?}"));
    let _ = std::fs::remove_dir_all(root);
}

fn receipt_value<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("receipt fixture: {error:?}"))
}

const HEIGHT: u64 = 150;
const PAYMENT_DENOM: &str = "usid";
const KEY_ID: [u8; 32] = [1; 32];

fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn encoded_receipt(
    tx_hash: [u8; 32],
    sender: [u8; 20],
    nonce: u64,
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
    to[12..].fill(0x22);
    let vm_error = if success { "" } else { "execution reverted" };
    let mut summary = Encoder::new(256);
    receipt_value(summary.u64(21_000));
    receipt_value(summary.u64(42_000));
    receipt_value(summary.u64(1));
    receipt_value(summary.u32(1));
    receipt_value(summary.fixed(&[0; 20]));
    receipt_value(summary.fixed(&[0; 20]));
    receipt_value(summary.fixed(&sha256(vm_error.as_bytes())));
    receipt_value(summary.u32(2));
    receipt_value(summary.u32(1));
    receipt_value(summary.u8(4));
    receipt_value(summary.fixed(b"upax"));
    let mut log = vec![8; 20];
    log.push(1);
    log.extend_from_slice(&[9; 32]);
    log.extend_from_slice(&sha256(&[10, 11]));

    let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
    receipt_value(encoder.structure_header_version(0x5201, PROTOCOL_VERSION));
    receipt_value(encoder.u16(PROTOCOL_VERSION));
    receipt_value(encoder.bytes(&tx_hash, 32));
    receipt_value(encoder.u64(7));
    receipt_value(encoder.bytes(&previous, 32));
    receipt_value(encoder.bytes(&sha256(&sequence), 32));
    receipt_value(encoder.bytes(&sha256(&[0xab, 0xcd]), 32));
    receipt_value(encoder.i32(i32::from(!success)));
    receipt_value(encoder.sequence_length(2, 512));
    for (ordinal, kind, body) in [(0, 1, summary.finish()), (1, 3, log)] {
        receipt_value(encoder.u16(12));
        receipt_value(encoder.u16(ordinal));
        receipt_value(encoder.u16(0));
        receipt_value(encoder.u8(kind));
        receipt_value(encoder.u8(0));
        receipt_value(encoder.bytes(&[0; 32], 32));
        receipt_value(encoder.bytes(&body, 256));
    }
    receipt_value(encoder.u128(21_000));
    receipt_value(encoder.bytes(&height, 32));
    receipt_value(encoder.u16(12));
    receipt_value(encoder.u32(1));
    receipt_value(encoder.u32(0));
    receipt_value(encoder.u8(2));
    receipt_value(encoder.bytes(&sha256(PAYMENT_DENOM.as_bytes()), 32));
    receipt_value(encoder.u128(1));
    receipt_value(encoder.bytes(&from, 32));
    receipt_value(encoder.u128(21_002));
    receipt_value(encoder.u128(if success { 1 } else { 2 }));
    receipt_value(encoder.u64(nonce));
    receipt_value(encoder.bytes(&to, 32));
    receipt_value(encoder.u128(0));
    receipt_value(encoder.u128(u128::from(success)));
    receipt_value(encoder.bytes(&[0; 32], 32));
    receipt_value(encoder.bytes(&[0; 32], 32));
    receipt_value(encoder.bytes(&KEY_ID, 32));
    receipt_value(encoder.u64(1_000));
    receipt_value(encoder.u8(u8::from(signature.is_some())));
    if let Some(signature) = signature {
        receipt_value(encoder.bytes(&signature, 64));
    }
    encoder.finish()
}

fn sequencer_receipt(raw: &[u8], success: bool) -> (Value, ReceiptKeyEntry) {
    let (transaction, signature) = receipt_value(decode_signed(raw));
    let sender = receipt_value(recover_sender(&transaction, &signature));
    let tx_hash = transaction_hash(raw);
    let nonce = transaction.nonce;
    let signer = SigningKey::from_bytes(&[8; 32]);
    let unsigned = encoded_receipt(tx_hash, sender, nonce, success, None);
    let signature = signer
        .sign(&receipt_value(receipt_digest(&unsigned)))
        .to_bytes();
    let bytes = encoded_receipt(tx_hash, sender, nonce, success, Some(signature));
    let decoded = receipt_value(layerx_wire::receipt::decode(&bytes));
    assert_eq!(
        decoded.protocol().unwrap().asset(),
        sha256(PAYMENT_DENOM.as_bytes())
    );
    let key = ReceiptKeyEntry {
        key_id: KEY_ID,
        public_key: signer.verifying_key().to_bytes(),
        first_height: 100,
        last_height: 199,
    };
    let view = json!({
        "rung": "receipted", "txHash": rpc_hex(&tx_hash), "nonce": format!("{nonce:#x}"),
        "seq": "0x7", "height": "0x96", "index": "0x1", "timestamp": "0x3e8",
        "status": if success { "0x1" } else { "0x0" }, "returnData": "0xabcd",
        "logs": [{"address": rpc_hex(&[8; 20]), "topics": [rpc_hex(&[9; 32])],
            "data": "0x0a0b", "logIndex": "0x0"}],
        "gasUsed": "0x5208", "cumulativeGasUsed": "0xa410", "effectiveGasPrice": "0x1",
        "paymentDenom": PAYMENT_DENOM, "feeDenom": "upax", "feeAmount": "0x5208", "sponsor": rpc_hex(&[0; 20]),
        "from": rpc_hex(&sender), "to": rpc_hex(&[0x22; 20]), "contractAddress": rpc_hex(&[0; 20]),
        "vmError": if success { "" } else { "execution reverted" },
        "keyId": rpc_hex(&KEY_ID), "receipt": rpc_hex(&bytes),
        "signingKey": rpc_hex(&key.public_key)
    });
    (view, key)
}

#[test]
fn missing_or_changed_payment_denomination_never_resolves_unknown() {
    for missing in [true, false] {
        let root = directory("payment-denomination");
        let (mut store, mut outbox, exact, hash) = unknown_outbox(&root, 5);
        let (mut receipt, key) = sequencer_receipt(&exact, true);
        if missing {
            receipt.as_object_mut().unwrap().remove("paymentDenom");
        } else {
            receipt["paymentDenom"] = json!("upax");
        }
        let (mut endpoint, server) = serve(vec![(
            "pax_sequencerReceipt",
            json!([rpc_hex(&hash)]),
            Some(receipt),
        )]);
        let result =
            resolve_unknown(&mut outbox, &mut store, hash, 1_000, &[key], &mut endpoint).unwrap();
        let traffic = server.join().unwrap();
        assert_eq!(result.state, SubmissionState::Unknown);
        assert_eq!(result.observation, ResolutionObservation::UnverifiedReceipt);
        assert_eq!(result.resend, ResendObservation::NotWarranted);
        assert_eq!(traffic.lookups, 1);
        assert!(traffic.transmitted.is_empty());
        assert!(outbox.status(hash).unwrap().evidence.is_none());
        drop(outbox);
        drop(store);
        let store = Store::open(&root).unwrap();
        let mut restored = Outbox::default();
        restored.restore(&store, tenant(), hash).unwrap();
        assert_eq!(
            restored.status(hash).unwrap().state,
            SubmissionState::Unknown
        );
        assert!(restored.status(hash).unwrap().evidence.is_none());
        assert_eq!(restored.exact_signed_bytes(hash).unwrap(), exact);
        drop(store);
        std::fs::remove_dir_all(root).unwrap();
    }
}
