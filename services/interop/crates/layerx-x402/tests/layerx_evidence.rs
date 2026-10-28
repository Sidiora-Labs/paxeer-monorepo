use std::collections::BTreeMap;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_interop_gateway::trace::{TraceId, Traced};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_x402::buyer::{BuiltPayment, Buyer, CapturedSettlement};
use layerx_x402::model::{
    AtomicAmount, PaymentPayload, PaymentRequirements, SettlementResponse, X402Error, X402_VERSION,
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
    receipt_keys: &[SequencerAuthorization],
) -> Result<CapturedSettlement, X402Error> {
    Buyer::capture_settlement(
        &encode_header(response)?,
        payment,
        receipt_keys,
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

fn unhex(value: &str) -> Result<Vec<u8>, X402Error> {
    let digits = value
        .strip_prefix("0x")
        .ok_or(X402Error::EvidenceMismatch)?;
    if digits.len() % 2 != 0 || !digits.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err(X402Error::EvidenceMismatch);
    }
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let text = std::str::from_utf8(pair).map_err(|_| X402Error::EvidenceMismatch)?;
            u8::from_str_radix(text, 16).map_err(|_| X402Error::EvidenceMismatch)
        })
        .collect()
}

fn settlement() -> Result<
    (
        BuiltPayment,
        SettlementResponse,
        Vec<SequencerAuthorization>,
    ),
    X402Error,
> {
    let document: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .map_err(|_| X402Error::EvidenceMismatch)?;
    let case = document["cases"]
        .as_array()
        .and_then(|cases| {
            cases
                .iter()
                .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        })
        .ok_or(X402Error::EvidenceMissing)?;
    let text = |value: &Value| {
        value
            .as_str()
            .map(str::to_owned)
            .ok_or(X402Error::EvidenceMismatch)
    };
    let word = |value: &Value| -> Result<[u8; 32], X402Error> {
        unhex(&text(value)?)?
            .try_into()
            .map_err(|_| X402Error::EvidenceMismatch)
    };
    let log = &case["receiptJson"]["logs"][0];
    let mut topics = log["topics"]
        .as_array()
        .ok_or(X402Error::EvidenceMismatch)?
        .iter()
        .map(word)
        .collect::<Result<Vec<_>, _>>()?;
    let recorded_topics = topics.concat();
    topics[1..].rotate_right(1);
    let mut receipt = unhex(&text(&case["signedReceipt"])?)?;
    let positions: Vec<_> = receipt
        .windows(recorded_topics.len())
        .enumerate()
        .filter_map(|(index, value)| (value == recorded_topics).then_some(index))
        .collect();
    assert_eq!(positions.len(), 1);
    receipt[positions[0]..positions[0] + recorded_topics.len()].copy_from_slice(&topics.concat());
    let digest = layerx_interop_gateway::evidence::canonical_receipt_digest(&receipt)
        .map_err(|_| X402Error::EvidenceMismatch)?;
    let signer = SigningKey::from_bytes(&[0x72; 32]);
    let signature_start = receipt.len() - 64;
    receipt[signature_start..].copy_from_slice(&signer.sign(&digest).to_bytes());
    let mut logs = case["receiptJson"]["logs"].clone();
    logs[0]["topics"] = json!(topics
        .iter()
        .map(|topic| format!("0x{}", hex(topic)))
        .collect::<Vec<_>>());
    let data = unhex(&text(&log["data"])?)?;
    let amount = AtomicAmount::from_u128(1_000_000);
    let payment = BuiltPayment {
        header: String::new(),
        payload: PaymentPayload {
            x402_version: X402_VERSION,
            resource: None,
            accepted: PaymentRequirements {
                scheme: "exact".to_owned(),
                network: "eip155:125".to_owned(),
                amount,
                asset: format!(
                    "0x{}",
                    hex(&data[..32]
                        .try_into()
                        .map_err(|_| X402Error::EvidenceMismatch)?)
                ),
                pay_to: text(&log["topics"][2])?,
                max_timeout_seconds: 90,
                extra: Some(
                    json!({"layerx": {"account": "agent:did:layerx:payee:main", "currency": "LXP"}}),
                ),
            },
            payload: json!({}),
            extensions: BTreeMap::new(),
        },
        idempotency_key: word(&log["topics"][3])?,
    };
    let response = SettlementResponse {
        success: true,
        error_reason: None,
        payer: Some(hex(&word(&log["topics"][1])?)),
        transaction: text(&case["receiptJson"]["txHash"])?,
        network: "eip155:125".to_owned(),
        amount: Some(amount),
        extensions: BTreeMap::from([(
            "layerx".to_owned(),
            json!({
                "receipt": STANDARD.encode(&receipt),
                "receiptDigest": hex(&digest),
                "verificationLevel": "receipted", "logs": logs,
            }),
        )]),
    };
    let key = &case["pinnedKey"];
    let keys = vec![SequencerAuthorization::new(
        word(&key["keyId"])?,
        signer.verifying_key().to_bytes(),
        key["firstHeight"]
            .as_u64()
            .ok_or(X402Error::EvidenceMismatch)?,
        key["lastHeight"]
            .as_u64()
            .ok_or(X402Error::EvidenceMismatch)?,
    )];
    Ok((payment, response, keys))
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
