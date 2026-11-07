use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::proof::ProofBundleTarget;
use layerx_agent_api::read::{HistoryCursor, HistorySelector};
use layerx_agentd::human_runtime::{
    balance_operation_value, history_operation_value, proof_bundle_operation_value,
    receipt_proof_operation,
};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
use layerx_types::{amount::Amount, rung::Rung};
use layerx_wire::{
    encode::Encoder,
    hash::receipt_digest,
    limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION},
};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

fn assert_no_sequences(value: &Value) {
    match value {
        Value::Object(fields) => {
            for (name, value) in fields {
                assert!(
                    !name.contains("sequence"),
                    "retired sequence coordinate {name}"
                );
                assert_no_sequences(value);
            }
        }
        Value::Array(items) => {
            for value in items {
                assert_no_sequences(value);
            }
        }
        _ => {}
    }
}

#[test]
fn read_balance_operation_returns_height_freshness() {
    let amount = Amount::from_u128(12345678901234567890);
    let value = balance_operation_value([1; 32], [2; 32], amount, &amount.to_be_bytes(), 150, 151);
    assert_eq!(value["value"]["amount"], "12345678901234567890");
    assert_eq!(
        value["value"]["canonical_state"],
        hex::encode(amount.to_be_bytes())
    );
    assert_eq!(value["value"]["height"], "151");
    assert_eq!(
        value["freshness"],
        json!({"chain_head": "150", "value_height": "151", "relative_to": "pending"})
    );
    assert_eq!(value["achieved_verification_level"], Value::Null);
    assert_no_sequences(&value);
    let historical =
        balance_operation_value([1; 32], [2; 32], amount, &amount.to_be_bytes(), 150, 140);
    assert_eq!(historical["freshness"]["relative_to"], "140");
    assert_eq!(historical["freshness"]["chain_head"], "150");
    assert_eq!(historical["freshness"]["value_height"], "140");
}

fn hash(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn receipt_bytes(tx: [u8; 32], sender: [u8; 20], signature: Option<[u8; 64]>) -> Vec<u8> {
    let previous = [4; 32];
    let mut sequence = b"PAXEER/v1/sequence\0".to_vec();
    sequence.extend_from_slice(&previous);
    sequence.extend_from_slice(&7u64.to_be_bytes());
    sequence.extend_from_slice(&tx);
    let mut height = [0; 32];
    height[24..].copy_from_slice(&151u64.to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    let mut to = [0; 32];
    to[12..].fill(7);
    let mut summary = Encoder::new(256);
    summary.u64(21000).unwrap();
    summary.u64(42000).unwrap();
    summary.u64(1).unwrap();
    summary.u32(1).unwrap();
    summary.fixed(&[0; 20]).unwrap();
    summary.fixed(&[0; 20]).unwrap();
    summary.fixed(&hash(b"")).unwrap();
    summary.u32(2).unwrap();
    summary.u32(1).unwrap();
    summary.u8(4).unwrap();
    summary.fixed(b"upax").unwrap();
    let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
    encoder
        .structure_header_version(0x5201, PROTOCOL_VERSION)
        .unwrap();
    encoder.u16(PROTOCOL_VERSION).unwrap();
    encoder.bytes(&tx, 32).unwrap();
    encoder.u64(7).unwrap();
    encoder.bytes(&previous, 32).unwrap();
    encoder.bytes(&hash(&sequence), 32).unwrap();
    encoder.bytes(&hash(&[0xab, 0xcd]), 32).unwrap();
    encoder.i32(0).unwrap();
    encoder.sequence_length(1, 512).unwrap();
    encoder.u16(12).unwrap();
    encoder.u16(0).unwrap();
    encoder.u16(0).unwrap();
    encoder.u8(1).unwrap();
    encoder.u8(0).unwrap();
    encoder.bytes(&[0; 32], 32).unwrap();
    encoder.bytes(&summary.finish(), 256).unwrap();
    encoder.u128(21000).unwrap();
    encoder.bytes(&height, 32).unwrap();
    encoder.u16(12).unwrap();
    encoder.u32(1).unwrap();
    encoder.u32(0).unwrap();
    encoder.u8(2).unwrap();
    encoder.bytes(&hash(b"upax"), 32).unwrap();
    encoder.u128(1).unwrap();
    encoder.bytes(&from, 32).unwrap();
    encoder.u128(21002).unwrap();
    encoder.u128(1).unwrap();
    encoder.u64(41).unwrap();
    encoder.bytes(&to, 32).unwrap();
    encoder.u128(0).unwrap();
    encoder.u128(1).unwrap();
    encoder.bytes(&[0; 32], 32).unwrap();
    encoder.bytes(&[0; 32], 32).unwrap();
    encoder.bytes(&[1; 32], 32).unwrap();
    encoder.u64(1000).unwrap();
    encoder.u8(u8::from(signature.is_some())).unwrap();
    if let Some(signature) = signature {
        encoder.bytes(&signature, 64).unwrap();
    }
    encoder.finish()
}

#[test]
fn read_proof_bundle_operation_returns_receipt_proof_v2() {
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 41,
        max_priority_fee_per_gas: 0,
        max_fee_per_gas: 1,
        gas_limit: 21000,
        to: [7; 20],
        data: vec![],
    };
    let raw = call.sign(&[5; 32]).unwrap();
    let (_, signature) = layerx_crypto::evm_transaction::decode_signed(&raw).unwrap();
    let sender = layerx_crypto::evm_transaction::recover_sender(&call, &signature).unwrap();
    let tx = transaction_hash(&raw);
    let signer = SigningKey::from_bytes(&[8; 32]);
    let digest = receipt_digest(&receipt_bytes(tx, sender, None)).unwrap();
    let signed = receipt_bytes(tx, sender, Some(signer.sign(&digest).to_bytes()));
    let keys = [ReceiptKeyEntry {
        key_id: [1; 32],
        public_key: signer.verifying_key().to_bytes(),
        first_height: 100,
        last_height: 199,
    }];
    let response = receipt_proof_operation(
        ProofBundleTarget::Receipt(tx),
        Rung::Receipted,
        &raw,
        &signed,
        &keys,
        125,
        150,
        [9; 32],
    )
    .unwrap();
    assert_eq!(response.achieved_rung, Rung::Receipted);
    let value = proof_bundle_operation_value(&response).unwrap();
    assert_eq!(value["value"]["format"], "layerx-receipt-proof-v2");
    assert_eq!(value["value"]["verificationLevel"], "receipted");
    assert_eq!(value["achieved_verification_level"], "receipted");
    assert_eq!(
        value["value"]["rawTransaction"],
        format!("0x{}", hex::encode(&raw))
    );
    assert_eq!(
        value["value"]["signedReceipt"],
        format!("0x{}", hex::encode(&signed))
    );
    assert_eq!(
        value["value"]["receiptDigest"],
        format!("0x{}", hex::encode(digest))
    );
    let decoded = layerx_wire::receipt::decode(&signed).unwrap();
    let unsigned = layerx_wire::receipt::encode_unsigned(&decoded).unwrap();
    assert_eq!(unsigned, receipt_bytes(tx, sender, None));
    assert_eq!(
        response.value.receipt_digest,
        receipt_digest(&unsigned).unwrap()
    );
    layerx_crypto::ed25519::verify_digest(
        &keys[0].public_key,
        &decoded.protocol().unwrap().sequencer_signature().unwrap(),
        &response.value.receipt_digest,
    )
    .unwrap();
    assert_eq!(value["freshness"]["chain_head"], "150");
    assert_eq!(value["freshness"]["relative_to"], "pending");
    assert_eq!(value["freshness"]["value_height"], "151");
    assert!(value["value"].get("commit").is_none());
    assert_no_sequences(&value);
    let mut corrupt = signed.clone();
    let last = corrupt.len() - 1;
    corrupt[last] ^= 1;
    assert!(receipt_proof_operation(
        ProofBundleTarget::Receipt(tx),
        Rung::Receipted,
        &raw,
        &corrupt,
        &keys,
        125,
        150,
        [9; 32]
    )
    .is_err());
    assert!(receipt_proof_operation(
        ProofBundleTarget::Receipt([3; 32]),
        Rung::Receipted,
        &raw,
        &signed,
        &keys,
        125,
        150,
        [9; 32]
    )
    .is_err());
    assert!(receipt_proof_operation(
        ProofBundleTarget::Receipt(tx),
        Rung::Committed,
        &raw,
        &signed,
        &keys,
        125,
        150,
        [9; 32]
    )
    .is_err());
    assert!(receipt_proof_operation(
        ProofBundleTarget::Receipt(tx),
        Rung::Receipted,
        &raw,
        &signed,
        &keys,
        126,
        150,
        [9; 32]
    )
    .is_err());
}

#[test]
fn read_history_operation_returns_block_position_cursor() {
    let selector = HistorySelector {
        from_height: 140,
        to_height: 150,
        cursor: Some(HistoryCursor {
            block_height: 140,
            tx_index: 0,
            log_index: 0,
        }),
        page_limit: 2,
    };
    let result = json!({"height":"150", "items":[
        {"height":"141","txIndex":"2","logIndex":"0","kind":"Receipt","canonical":"abcd","proof":null},
        {"height":"141","txIndex":"2","logIndex":"1","kind":"Event","canonical":"1234","proof":"aa"}],
        "cursor":["141","2","1"]});
    let value = history_operation_value(&selector, &result, 155).unwrap();
    assert_eq!(value["value"]["items"], result["items"]);
    assert_eq!(value["value"]["next_cursor"], json!(["141", "2", "1"]));
    assert_eq!(value["value"]["cursor"], value["value"]["next_cursor"]);
    assert_eq!(value["freshness"]["chain_head"], "155");
    assert_eq!(value["freshness"]["value_height"], "150");
    assert!(value["achieved_verification_level"].is_null());
    assert_no_sequences(&value);
    for cursor in [
        json!(["0141", "2", "1"]),
        json!(["141", "2"]),
        json!(["141", "2", "18446744073709551616"]),
        json!(["142", "0", "0"]),
    ] {
        let mut malformed = result.clone();
        malformed["cursor"] = cursor;
        assert!(history_operation_value(&selector, &malformed, 155).is_err());
    }
    let mut unordered = result.clone();
    unordered["items"].as_array_mut().unwrap().reverse();
    assert!(history_operation_value(&selector, &unordered, 155).is_err());
    let mut limited = selector.clone();
    limited.page_limit = 1;
    assert!(history_operation_value(&limited, &result, 155).is_err());
}
