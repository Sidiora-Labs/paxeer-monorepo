use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::error::RequestId;
use layerx_agentd::agent_rpc_dispatch::{
    decode_eip1559_prepare_answer, decode_eip1559_submit_answer,
};
use layerx_agentd::human::HumanResponse;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use serde_json::json;

fn call() -> Eip1559Call {
    Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1_000_000_000,
        max_fee_per_gas: 3_000_000_000,
        gas_limit: 100_000,
        to: [0x10; 20],
        data: vec![0xde, 0xad, 0xbe, 0xef],
    }
}

fn preparation(transaction: &Eip1559Call) -> Vec<u8> {
    let mut out = Encoder::new(4096);
    out.bytes(b"preparation-7", 255).expect("reference");
    out.u8(2).expect("transaction type");
    out.u64(transaction.chain_id).expect("chain");
    out.u64(transaction.nonce).expect("nonce");
    out.u128(transaction.max_priority_fee_per_gas).expect("tip");
    out.u128(transaction.max_fee_per_gas).expect("fee cap");
    out.u64(transaction.gas_limit).expect("gas");
    out.fixed(&transaction.to).expect("recipient");
    out.bytes(&transaction.data, 1_048_576).expect("calldata");
    out.u32(0x10005).expect("activity");
    out.bytes(b"did:layerx:agent", 255).expect("actor");
    out.bytes(b"authority", 255).expect("authority");
    out.u64(7).expect("sequence");
    out.u64(10).expect("not before");
    out.u64(20).expect("not after");
    out.u128(300_000_000_000_000).expect("fee limit");
    out.bytes(b"payload", 1_048_576).expect("payload");
    out.fixed(&[0x11; 32]).expect("payload hash");
    out.fixed(&[0x22; 32]).expect("idempotency key");
    out.finish()
}

fn response(bytes: Vec<u8>) -> HumanResponse {
    HumanResponse::new(bytes).expect("bounded owner payload")
}

#[test]
fn prepare_answers_eip1559_fields() {
    let transaction = call();
    let bytes = preparation(&transaction);
    let answer = decode_eip1559_prepare_answer(RequestId(1), response(bytes.clone()))
        .expect("prepare answer");
    assert_eq!(
        answer,
        json!({
            "preparation_ref": "preparation-7",
            "chain_id": "125",
            "nonce": "7",
            "max_priority_fee_per_gas": "1000000000",
            "max_fee_per_gas": "3000000000",
            "gas_limit": "100000",
            "to": hex::encode(transaction.to),
            "data": "deadbeef",
            "signing_hash": hex::encode(transaction.signing_hash()),
            "activity_type": "65541",
            "actor": "did:layerx:agent",
            "authority": "authority",
            "account_sequence": "7",
            "not_before": "10",
            "not_after": "20",
            "fee_limit": "300000000000000",
            "payload": hex::encode(b"payload"),
            "payload_hash": "11".repeat(32),
            "idempotency_key": "22".repeat(32),
        })
    );
    assert!(answer.get("unsigned_canonical_bytes").is_none());
    assert!(answer.get("signing_preimage").is_none());
    let mut wrong_type = bytes.clone();
    wrong_type[4 + b"preparation-7".len()] = 1;
    assert!(decode_eip1559_prepare_answer(RequestId(1), response(wrong_type)).is_err());
    for end in 1..bytes.len() {
        assert!(
            decode_eip1559_prepare_answer(RequestId(1), response(bytes[..end].to_vec())).is_err()
        );
    }
    let mut trailing = bytes;
    trailing.push(0);
    assert!(decode_eip1559_prepare_answer(RequestId(1), response(trailing)).is_err());
    let empty = Eip1559Call {
        nonce: 0,
        data: Vec::new(),
        ..transaction.clone()
    };
    let empty_answer = decode_eip1559_prepare_answer(RequestId(2), response(preparation(&empty)))
        .expect("zero nonce and empty calldata are valid");
    assert_eq!(empty_answer["data"], "");
    assert_eq!(empty_answer["nonce"], "0");
    assert_eq!(
        empty_answer["signing_hash"],
        hex::encode(empty.signing_hash())
    );
    let wide = Eip1559Call {
        chain_id: u64::MAX,
        nonce: u64::MAX,
        max_priority_fee_per_gas: u128::MAX - 1,
        max_fee_per_gas: u128::MAX,
        ..transaction.clone()
    };
    let wide_answer = decode_eip1559_prepare_answer(RequestId(2), response(preparation(&wide)))
        .expect("full-width integers remain exact decimal strings");
    assert_eq!(wide_answer["chain_id"], u64::MAX.to_string());
    assert_eq!(wide_answer["nonce"], u64::MAX.to_string());
    assert_eq!(
        wide_answer["max_priority_fee_per_gas"],
        (u128::MAX - 1).to_string()
    );
    assert_eq!(wide_answer["max_fee_per_gas"], u128::MAX.to_string());
    assert_eq!(
        wide_answer["signing_hash"],
        hex::encode(wide.signing_hash())
    );
    for invalid in [
        Eip1559Call {
            chain_id: 0,
            ..transaction.clone()
        },
        Eip1559Call {
            gas_limit: 0,
            ..transaction.clone()
        },
        Eip1559Call {
            max_priority_fee_per_gas: transaction.max_fee_per_gas + 1,
            ..transaction
        },
    ] {
        assert!(
            decode_eip1559_prepare_answer(RequestId(3), response(preparation(&invalid))).is_err()
        );
    }
}

fn receipt(tx_hash: [u8; 32], signature: Option<[u8; 64]>) -> Vec<u8> {
    let mut out = Encoder::new(4096);
    out.structure_header_version(0x5201, 2)
        .expect("receipt header");
    out.u16(2).expect("protocol");
    out.bytes(&tx_hash, 32).expect("transaction hash");
    out.u64(8).expect("sequence");
    for root in [[2; 32], [3; 32], [8; 32]] {
        out.bytes(&root, 32).expect("root");
    }
    out.i32(0).expect("success");
    out.sequence_length(0, 512).expect("effects");
    out.u128(1).expect("fee charged");
    let mut height = [0; 32];
    height[31] = 4;
    out.bytes(&height, 32).expect("height");
    out.u16(12).expect("sequencer module");
    out.u32(1).expect("module layout");
    out.u32(1).expect("parameters");
    out.u8(1).expect("operation");
    out.bytes(&[5; 32], 32).expect("asset");
    out.u128(0).expect("amount");
    out.bytes(&[6; 32], 32).expect("sender");
    out.u128(100).expect("sender before");
    out.u128(99).expect("sender after");
    out.u64(7).expect("nonce");
    out.bytes(&[7; 32], 32).expect("recipient");
    out.u128(0).expect("recipient before");
    out.u128(0).expect("recipient after");
    out.bytes(&[9; 32], 32).expect("transfers");
    out.bytes(&[10; 32], 32).expect("authorization");
    out.bytes(&[11; 32], 32).expect("receipt key id");
    out.u64(1000).expect("timestamp");
    out.u8(u8::from(signature.is_some()))
        .expect("signature presence");
    if let Some(signature) = signature {
        out.bytes(&signature, 64).expect("signature");
    }
    out.finish()
}

fn submission(raw_transaction: &[u8], signed_receipt: &[u8]) -> Vec<u8> {
    let mut out = Encoder::new(4096);
    out.bytes(b"preparation-7", 255).expect("reference");
    out.bytes(raw_transaction, 1_048_576)
        .expect("signed transaction");
    out.bytes(signed_receipt, 1_048_576)
        .expect("signed receipt");
    out.finish()
}

#[test]
fn submit_answers_receipted_with_tx_hash() {
    let transaction = call();
    let prepared = decode_eip1559_prepare_answer(RequestId(3), response(preparation(&transaction)))
        .expect("prepare before signing");
    let mut secret = [0; 32];
    secret[31] = 1;
    let raw = transaction.sign(&secret).expect("real type-2 signature");
    let (decoded, signature) = decode_signed(&raw).expect("canonical transaction");
    assert_eq!(decoded, transaction);
    assert_eq!(
        hex::encode(recover_sender(&decoded, &signature).expect("sender")),
        "7e5f4552091a69125d5dfcb7b8c2659029395bdf"
    );
    let tx_hash = transaction_hash(&raw);
    let sequencer = SigningKey::from_bytes(&[0x31; 32]);
    let unsigned = receipt(tx_hash, None);
    let digest = receipt_digest(&unsigned).expect("receipt digest");
    let signed_receipt = receipt(tx_hash, Some(sequencer.sign(&digest).to_bytes()));
    let keys = [SequencerAuthorization::new(
        [11; 32],
        sequencer.verifying_key().to_bytes(),
        4,
        4,
    )];
    layerx_proof::receipt::verify_sequencer_signature(&signed_receipt, &keys)
        .expect("real signed sequencer receipt");
    let bytes = submission(&raw, &signed_receipt);
    let answer = decode_eip1559_submit_answer(RequestId(4), response(bytes.clone()))
        .expect("receipted answer");
    assert_eq!(answer["preparation_ref"], prepared["preparation_ref"]);
    assert_eq!(
        prepared["signing_hash"],
        hex::encode(decoded.signing_hash())
    );
    assert_eq!(
        answer,
        json!({
            "preparation_ref": "preparation-7",
            "state": "Receipted",
            "signed_receipt": hex::encode(&signed_receipt),
            "tx_hash": hex::encode(tx_hash),
        })
    );
    for end in 1..bytes.len() {
        assert!(
            decode_eip1559_submit_answer(RequestId(4), response(bytes[..end].to_vec())).is_err()
        );
    }
    let mut trailing = bytes;
    trailing.push(0);
    assert!(decode_eip1559_submit_answer(RequestId(4), response(trailing)).is_err());
    let other_raw = Eip1559Call {
        nonce: 8,
        ..transaction
    }
    .sign(&secret)
    .expect("other transaction");
    assert!(decode_eip1559_submit_answer(
        RequestId(4),
        response(submission(&other_raw, &signed_receipt))
    )
    .is_err());
    assert!(
        decode_eip1559_submit_answer(RequestId(4), response(submission(&raw, &unsigned))).is_err()
    );
    assert!(decode_eip1559_submit_answer(
        RequestId(4),
        response(submission(b"not a transaction", &signed_receipt))
    )
    .is_err());
}
