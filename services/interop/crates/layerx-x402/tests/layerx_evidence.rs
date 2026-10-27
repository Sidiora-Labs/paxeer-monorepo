use std::collections::BTreeMap;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_interop_gateway::trace::{TraceId, Traced};
use layerx_proof::merkle::leaf_hash;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_wire::encode::Encoder;
use layerx_wire::limits::PROTOCOL_VERSION;
use layerx_x402::buyer::{BuiltPayment, Buyer, CapturedSettlement};
use layerx_x402::model::{
    account_identifiers, AtomicAmount, PaymentPayload, PaymentRequirements, SettlementResponse,
    X402Error, X402_VERSION,
};
use serde::Serialize;
use serde_json::{json, Value};

fn encode_header(value: &impl Serialize) -> Result<String, X402Error> {
    serde_json::to_vec(value)
        .map(|bytes| STANDARD.encode(bytes))
        .map_err(|_| X402Error::Encode)
}

fn capture(
    payment: &BuiltPayment,
    response: &SettlementResponse,
    batch: &AuthorizedBatch,
) -> Result<CapturedSettlement, X402Error> {
    Buyer::capture_settlement(
        &encode_header(response)?,
        payment,
        batch,
        &TraceId::mint([0xab; 16]),
    )
    .map_err(Traced::into_error)
}

fn hex(bytes: &[u8; 32]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut output = String::with_capacity(64);
    for byte in bytes {
        output.push(char::from(DIGITS[usize::from(byte >> 4)]));
        output.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    output
}

const ACCOUNT: &str = "agent:did:layerx:payee:main";
const ASSET: [u8; 32] = [0x11; 32];
const PAYER: [u8; 32] = [0x22; 32];
const TRANSACTION: [u8; 32] = [0x61; 32];

fn receipt(recipient: &[u8; 32], signature: Option<[u8; 64]>) -> Result<Vec<u8>, X402Error> {
    let invalid = |_| X402Error::InvalidPayload;
    let mut encoder = Encoder::new(4096);
    encoder
        .structure_header_version(0x5201, PROTOCOL_VERSION)
        .map_err(invalid)?;
    encoder.u16(PROTOCOL_VERSION).map_err(invalid)?;
    encoder.bytes(&TRANSACTION, 32).map_err(invalid)?;
    encoder.u64(9).map_err(invalid)?;
    encoder.bytes(&[0x62; 32], 32).map_err(invalid)?;
    encoder.bytes(&[0x63; 32], 32).map_err(invalid)?;
    encoder.bytes(&[0x64; 32], 32).map_err(invalid)?;
    encoder.i32(0).map_err(invalid)?;
    encoder.sequence_length(0, 512).map_err(invalid)?;
    encoder.u128(1).map_err(invalid)?;
    encoder.bytes(&[0x65; 32], 32).map_err(invalid)?;
    encoder.u16(1).map_err(invalid)?;
    encoder.u32(1).map_err(invalid)?;
    encoder.u32(1).map_err(invalid)?;
    encoder.u8(6).map_err(invalid)?;
    encoder.bytes(&ASSET, 32).map_err(invalid)?;
    encoder.u128(1000).map_err(invalid)?;
    encoder.bytes(&PAYER, 32).map_err(invalid)?;
    encoder.u128(5000).map_err(invalid)?;
    encoder.u128(4000).map_err(invalid)?;
    encoder.u64(1).map_err(invalid)?;
    encoder.bytes(recipient, 32).map_err(invalid)?;
    encoder.u128(0).map_err(invalid)?;
    encoder.u128(1000).map_err(invalid)?;
    encoder.bytes(&[0x66; 32], 32).map_err(invalid)?;
    encoder.bytes(&[0x67; 32], 32).map_err(invalid)?;
    encoder.bytes(&[0x68; 32], 32).map_err(invalid)?;
    encoder.u64(1000).map_err(invalid)?;
    encoder.u8(u8::from(signature.is_some())).map_err(invalid)?;
    if let Some(signature) = signature {
        encoder.bytes(&signature, 64).map_err(invalid)?;
    }
    Ok(encoder.finish())
}

fn settlement() -> Result<(BuiltPayment, SettlementResponse, AuthorizedBatch), X402Error> {
    let recipient = account_identifiers(ACCOUNT)?[0];
    let key = SigningKey::from_bytes(&[0x51; 32]);
    let unsigned = receipt(&recipient, None)?;
    let signature_digest =
        layerx_wire::hash::receipt_digest(&unsigned).map_err(|_| X402Error::EvidenceMismatch)?;
    let canonical = receipt(&recipient, Some(key.sign(&signature_digest).to_bytes()))?;
    let digest = leaf_hash(&canonical).map_err(|_| X402Error::EvidenceMismatch)?;
    let mut payment = BuiltPayment {
        header: String::new(),
        payload: PaymentPayload {
            x402_version: X402_VERSION,
            resource: None,
            accepted: PaymentRequirements {
                scheme: "exact".to_owned(),
                network: "eip155:125".to_owned(),
                amount: AtomicAmount::from_u128(1000),
                asset: format!("0x{}", hex(&ASSET)),
                pay_to: format!("0x{}", hex(&recipient)),
                max_timeout_seconds: 90,
                extra: Some(json!({"layerx": {"account": ACCOUNT, "currency": "LXP"}})),
            },
            payload: json!({}),
            extensions: BTreeMap::new(),
        },
        idempotency_key: [0x5c; 32],
    };
    payment.header = encode_header(&payment.payload)?;
    let response = SettlementResponse {
        success: true,
        error_reason: None,
        payer: Some(hex(&PAYER)),
        transaction: format!("0x{}", hex(&TRANSACTION)),
        network: payment.payload.accepted.network.clone(),
        amount: Some(payment.payload.accepted.amount),
        extensions: BTreeMap::from([(
            "layerx".to_owned(),
            json!({
                "receipt": STANDARD.encode(&canonical),
                "receiptDigest": hex(&digest),
                "verificationLevel": "receipted",
            }),
        )]),
    };
    let batch = AuthorizedBatch::new(
        [0x65; 32],
        ASSET,
        [0x62; 32],
        [0x63; 32],
        key.verifying_key().to_bytes(),
    );
    Ok((payment, response, batch))
}

#[test]
fn committed_evidence_without_block_fields_is_refused() -> Result<(), X402Error> {
    let (payment, mut response, batch) = settlement()?;
    let receipted = capture(&payment, &response, &batch)?;
    response
        .extensions
        .get_mut("layerx")
        .ok_or(X402Error::EvidenceMissing)?["verificationLevel"] = json!("committed");
    assert_eq!(
        capture(&payment, &response, &batch),
        Err(X402Error::EvidenceMismatch)
    );

    response
        .extensions
        .get_mut("layerx")
        .ok_or(X402Error::EvidenceMissing)?["blockNumber"] = json!("0x9");
    assert_eq!(
        capture(&payment, &response, &batch),
        Err(X402Error::EvidenceMismatch)
    );
    response
        .extensions
        .get_mut("layerx")
        .ok_or(X402Error::EvidenceMissing)?["blockHash"] = json!(format!("0x{}", "ab".repeat(32)));
    let committed = response.clone();
    let captured = capture(&payment, &committed, &batch)?;
    assert_eq!(captured.response, committed);
    assert_eq!(captured.canonical_receipt, receipted.canonical_receipt);
    assert_eq!(captured.receipt_digest, receipted.receipt_digest);

    for number in ["0x0", "0x1", "0xa", "0xA", "0x100", "0xffffffffffffffff"] {
        response = committed.clone();
        response
            .extensions
            .get_mut("layerx")
            .ok_or(X402Error::EvidenceMissing)?["blockNumber"] = json!(number);
        assert_eq!(capture(&payment, &response, &batch)?.response, response);
    }
    for field in ["blockNumber", "blockHash"] {
        response = committed.clone();
        response
            .extensions
            .get_mut("layerx")
            .and_then(Value::as_object_mut)
            .ok_or(X402Error::EvidenceMissing)?
            .remove(field);
        assert_eq!(
            capture(&payment, &response, &batch),
            Err(X402Error::EvidenceMismatch)
        );
        for value in [Value::Null, json!(9), json!(true)] {
            response = committed.clone();
            response
                .extensions
                .get_mut("layerx")
                .ok_or(X402Error::EvidenceMissing)?[field] = value;
            assert_eq!(
                capture(&payment, &response, &batch),
                Err(X402Error::EvidenceMismatch)
            );
        }
    }
    for number in [
        "",
        "0x",
        "9",
        "0X9",
        "0x00",
        "0x09",
        "0xg",
        "0x-1",
        "0x+1",
        "0x9 ",
        "0x10000000000000000",
    ] {
        response = committed.clone();
        response
            .extensions
            .get_mut("layerx")
            .ok_or(X402Error::EvidenceMissing)?["blockNumber"] = json!(number);
        assert_eq!(
            capture(&payment, &response, &batch),
            Err(X402Error::EvidenceMismatch),
            "{number}"
        );
    }
    for hash in [
        String::new(),
        "0x".to_owned(),
        "ab".repeat(32),
        format!("0X{}", "ab".repeat(32)),
        format!("0x{}", "ab".repeat(31)),
        format!("0x{}", "ab".repeat(33)),
        format!("0x{}", "AB".repeat(32)),
        format!("0x{}", "gg".repeat(32)),
    ] {
        response = committed.clone();
        response
            .extensions
            .get_mut("layerx")
            .ok_or(X402Error::EvidenceMissing)?["blockHash"] = json!(hash);
        assert_eq!(
            capture(&payment, &response, &batch),
            Err(X402Error::EvidenceMismatch),
            "{hash}"
        );
    }
    Ok(())
}

#[test]
fn receipted_evidence_with_a_block_hash_is_refused() -> Result<(), X402Error> {
    let (payment, response, batch) = settlement()?;
    assert_eq!(capture(&payment, &response, &batch)?.response, response);
    for fields in [
        json!({"blockHash": format!("0x{}", "ab".repeat(32))}),
        json!({"blockNumber": "0x9"}),
        json!({"blockNumber": "0x9", "blockHash": format!("0x{}", "ab".repeat(32))}),
        json!({"blockHash": null}),
        json!({"blockNumber": null}),
    ] {
        let mut changed = response.clone();
        changed
            .extensions
            .get_mut("layerx")
            .and_then(Value::as_object_mut)
            .ok_or(X402Error::EvidenceMissing)?
            .extend(fields.as_object().ok_or(X402Error::InvalidPayload)?.clone());
        assert_eq!(
            capture(&payment, &changed, &batch),
            Err(X402Error::EvidenceMismatch)
        );
    }
    Ok(())
}
