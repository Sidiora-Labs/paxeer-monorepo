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
    lint_success(out.bytes(b"preparation-7", 255), "reference");
    lint_success(out.u8(2), "transaction type");
    lint_success(out.u64(transaction.chain_id), "chain");
    lint_success(out.u64(transaction.nonce), "nonce");
    lint_success(out.u128(transaction.max_priority_fee_per_gas), "tip");
    lint_success(out.u128(transaction.max_fee_per_gas), "fee cap");
    lint_success(out.u64(transaction.gas_limit), "gas");
    lint_success(out.fixed(&transaction.to), "recipient");
    lint_success(out.bytes(&transaction.data, 1_048_576), "calldata");
    lint_success(out.u32(0x1_0005), "activity");
    lint_success(out.bytes(b"did:layerx:agent", 255), "actor");
    lint_success(out.bytes(b"authority", 255), "authority");
    lint_success(out.u64(7), "sequence");
    lint_success(out.u64(10), "not before");
    lint_success(out.u64(20), "not after");
    lint_success(out.u128(300_000_000_000_000), "fee limit");
    lint_success(out.bytes(b"payload", 1_048_576), "payload");
    lint_success(out.fixed(&[0x11; 32]), "payload hash");
    lint_success(out.fixed(&[0x22; 32]), "idempotency key");
    out.finish()
}

fn response(bytes: Vec<u8>) -> HumanResponse {
    lint_success(HumanResponse::new(bytes), "bounded owner payload")
}

#[test]
fn prepare_answers_eip1559_fields() {
    let transaction = call();
    let bytes = preparation(&transaction);
    let answer = lint_success(
        decode_eip1559_prepare_answer(RequestId(1), response(bytes.clone())),
        "prepare answer",
    );
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
    let empty_answer = lint_success(
        decode_eip1559_prepare_answer(RequestId(2), response(preparation(&empty))),
        "zero nonce and empty calldata are valid",
    );
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
    let wide_answer = lint_success(
        decode_eip1559_prepare_answer(RequestId(2), response(preparation(&wide))),
        "full-width integers remain exact decimal strings",
    );
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
    lint_success(out.structure_header_version(0x5201, 2), "receipt header");
    lint_success(out.u16(2), "protocol");
    lint_success(out.bytes(&tx_hash, 32), "transaction hash");
    lint_success(out.u64(8), "sequence");
    for root in [[2; 32], [3; 32], [8; 32]] {
        lint_success(out.bytes(&root, 32), "root");
    }
    lint_success(out.i32(0), "success");
    lint_success(out.sequence_length(0, 512), "effects");
    lint_success(out.u128(1), "fee charged");
    let mut height = [0; 32];
    height[31] = 4;
    lint_success(out.bytes(&height, 32), "height");
    lint_success(out.u16(12), "sequencer module");
    lint_success(out.u32(1), "module layout");
    lint_success(out.u32(1), "parameters");
    lint_success(out.u8(1), "operation");
    lint_success(out.bytes(&[5; 32], 32), "asset");
    lint_success(out.u128(0), "amount");
    lint_success(out.bytes(&[6; 32], 32), "sender");
    lint_success(out.u128(100), "sender before");
    lint_success(out.u128(99), "sender after");
    lint_success(out.u64(7), "nonce");
    lint_success(out.bytes(&[7; 32], 32), "recipient");
    lint_success(out.u128(0), "recipient before");
    lint_success(out.u128(0), "recipient after");
    lint_success(out.bytes(&[9; 32], 32), "transfers");
    lint_success(out.bytes(&[10; 32], 32), "authorization");
    lint_success(out.bytes(&[11; 32], 32), "receipt key id");
    lint_success(out.u64(1000), "timestamp");
    lint_success(out.u8(u8::from(signature.is_some())), "signature presence");
    if let Some(signature) = signature {
        lint_success(out.bytes(&signature, 64), "signature");
    }
    out.finish()
}

fn submission(raw_transaction: &[u8], signed_receipt: &[u8]) -> Vec<u8> {
    let mut out = Encoder::new(4096);
    lint_success(out.bytes(b"preparation-7", 255), "reference");
    lint_success(out.bytes(raw_transaction, 1_048_576), "signed transaction");
    lint_success(out.bytes(signed_receipt, 1_048_576), "signed receipt");
    out.finish()
}

#[test]
fn submit_answers_receipted_with_tx_hash() {
    let transaction = call();
    let prepared = lint_success(
        decode_eip1559_prepare_answer(RequestId(3), response(preparation(&transaction))),
        "prepare before signing",
    );
    let mut secret = [0; 32];
    secret[31] = 1;
    let raw = lint_success(transaction.sign(&secret), "real type-2 signature");
    let (decoded, signature) = lint_success(decode_signed(&raw), "canonical transaction");
    assert_eq!(decoded, transaction);
    assert_eq!(
        hex::encode(lint_success(recover_sender(&decoded, &signature), "sender")),
        "7e5f4552091a69125d5dfcb7b8c2659029395bdf"
    );
    let tx_hash = transaction_hash(&raw);
    let sequencer = SigningKey::from_bytes(&[0x31; 32]);
    let unsigned = receipt(tx_hash, None);
    let digest = lint_success(receipt_digest(&unsigned), "receipt digest");
    let signed_receipt = receipt(tx_hash, Some(sequencer.sign(&digest).to_bytes()));
    let keys = [SequencerAuthorization::new(
        [11; 32],
        sequencer.verifying_key().to_bytes(),
        4,
        4,
    )];
    lint_success(
        layerx_proof::receipt::verify_sequencer_signature(&signed_receipt, &keys),
        "real signed sequencer receipt",
    );
    let bytes = submission(&raw, &signed_receipt);
    let answer = lint_success(
        decode_eip1559_submit_answer(RequestId(4), response(bytes.clone())),
        "receipted answer",
    );
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
    let other_raw = lint_success(
        Eip1559Call {
            nonce: 8,
            ..transaction
        }
        .sign(&secret),
        "other transaction",
    );
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

#[track_caller]
fn lint_success<T, E: std::fmt::Debug>(result: Result<T, E>, context: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{context}: {error:?}"),
    }
}
