use std::collections::BTreeMap;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use layerx_interop_gateway::trace::{TraceId, Traced};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_wire::hash::receipt_digest;
use layerx_wire::receipt::encode_unsigned;
use layerx_x402::buyer::{BuiltPayment, Buyer, CapturedSettlement, SupportedKind};
use layerx_x402::model::{
    AtomicAmount, PaymentPayload, PaymentRequirements, SettlementResponse, X402Error, X402_VERSION,
};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

type TestResult<T = ()> = Result<T, Box<dyn std::error::Error>>;

fn fixture(id: &str) -> Value {
    let document: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap_or_else(|error| panic!("receipt fixture: {error}"));
    document["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("fixture cases"))
        .iter()
        .find(|case| case["id"] == id)
        .unwrap_or_else(|| panic!("missing fixture {id}"))
        .clone()
}

fn text(value: &Value) -> &str {
    value.as_str().unwrap_or_else(|| panic!("expected text"))
}

fn bytes(value: &str) -> Vec<u8> {
    let digits = value
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("hex prefix"));
    assert_eq!(digits.len() % 2, 0);
    (0..digits.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&digits[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("hex: {error}"))
        })
        .collect()
}

fn word(value: &str) -> [u8; 32] {
    bytes(value)
        .try_into()
        .unwrap_or_else(|_| panic!("expected word"))
}

fn hex(value: &[u8]) -> String {
    value.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn recorded(id: &str) -> TestResult<(Buyer, BuiltPayment, SettlementResponse, Value)> {
    let case = fixture(id);
    let exact = fixture("V1-layerxledger-pay-exact");
    let log = &exact["receiptJson"]["logs"][0];
    let data = bytes(text(&log["data"]));
    let amount = AtomicAmount::from_u128(1_000_000);
    let key = &case["pinnedKey"];
    let authorization = SequencerAuthorization::new(
        word(text(&key["keyId"])),
        word(text(&key["publicKey"])),
        key["firstHeight"].as_u64().ok_or("first height")?,
        key["lastHeight"].as_u64().ok_or("last height")?,
    );
    let buyer = Buyer::with_receipt_keys(
        vec![SupportedKind {
            scheme: "exact".to_owned(),
            network: "eip155:125".to_owned(),
        }],
        vec![authorization],
    )?;
    let payment = BuiltPayment {
        header: String::new(),
        payload: PaymentPayload {
            x402_version: X402_VERSION,
            resource: None,
            accepted: PaymentRequirements {
                scheme: "exact".to_owned(),
                network: "eip155:125".to_owned(),
                amount,
                asset: format!("0x{}", hex(&data[..32])),
                pay_to: text(&log["topics"][2]).to_owned(),
                max_timeout_seconds: 90,
                extra: Some(
                    json!({"layerx": {"account": "agent:did:layerx:payee:main", "currency": "LXP"}}),
                ),
            },
            payload: json!({}),
            extensions: BTreeMap::new(),
        },
        idempotency_key: word(text(&log["topics"][3])),
    };
    let response = SettlementResponse {
        success: true,
        error_reason: None,
        payer: Some(text(&log["topics"][1]).trim_start_matches("0x").to_owned()),
        transaction: text(&case["receiptJson"]["txHash"]).to_owned(),
        network: "eip155:125".to_owned(),
        amount: Some(amount),
        extensions: BTreeMap::from([(
            "layerx".to_owned(),
            json!({
                "receipt": STANDARD.encode(bytes(text(&case["signedReceipt"]))),
                "receiptDigest": text(&case["receiptDigest"]).trim_start_matches("0x"),
                "verificationLevel": "receipted",
                "logs": case["receiptJson"]["logs"],
            }),
        )]),
    };
    Ok((buyer, payment, response, case))
}

fn capture(
    buyer: &Buyer,
    payment: &BuiltPayment,
    response: &SettlementResponse,
) -> Result<CapturedSettlement, X402Error> {
    let header = STANDARD.encode(serde_json::to_vec(response).map_err(|_| X402Error::Encode)?);
    Buyer::capture_settlement(
        &header,
        payment,
        buyer.receipt_keys(),
        &TraceId::mint([0xab; 16]),
    )
    .map_err(Traced::into_error)
}

#[test]
fn buyer_captures_the_recorded_receipted_settlement() -> TestResult {
    let (buyer, payment, response, case) = recorded("V1-layerxledger-pay-exact")?;
    let captured = capture(&buyer, &payment, &response)?;
    assert_eq!(
        captured.transaction_hash,
        word(text(&case["receiptJson"]["txHash"]))
    );
    assert_eq!(captured.response, response);
    assert_eq!(captured.rung, "receipted");
    assert_eq!(
        captured.canonical_receipt,
        bytes(text(&case["signedReceipt"]))
    );
    assert_eq!(captured.receipt_digest, word(text(&case["receiptDigest"])));
    let decoded = verify_sequencer_signature(&captured.canonical_receipt, buyer.receipt_keys())
        .map_err(|error| format!("signature: {error:?}"))?;
    let canonical_digest =
        receipt_digest(&encode_unsigned(&decoded).map_err(|error| format!("encode: {error:?}"))?)
            .map_err(|error| format!("digest: {error:?}"))?;
    assert_eq!(captured.receipt_digest, canonical_digest);

    let mut wrong_reference = payment.clone();
    wrong_reference.idempotency_key[0] ^= 1;
    assert_eq!(
        capture(&buyer, &wrong_reference, &response),
        Err(X402Error::EvidenceMismatch)
    );
    let mut wrong_amount = payment.clone();
    wrong_amount.payload.accepted.amount = AtomicAmount::from_u128(999_999);
    assert_eq!(
        capture(&buyer, &wrong_amount, &response),
        Err(X402Error::EvidenceMismatch)
    );
    for field in ["asset", "payTo"] {
        let mut wrong_payment = payment.clone();
        let value = format!("0x{}", "ff".repeat(32));
        if field == "asset" {
            wrong_payment.payload.accepted.asset = value;
        } else {
            wrong_payment.payload.accepted.pay_to = value;
        }
        assert_eq!(
            capture(&buyer, &wrong_payment, &response),
            Err(X402Error::EvidenceMismatch)
        );
    }
    let unpinned = Buyer::new(vec![SupportedKind {
        scheme: "exact".to_owned(),
        network: "eip155:125".to_owned(),
    }])?;
    assert_eq!(
        capture(&unpinned, &payment, &response),
        Err(X402Error::EvidenceMismatch)
    );
    Ok(())
}

#[test]
fn buyer_refuses_a_tampered_log() -> TestResult {
    let (buyer, payment, response, case) = recorded("V3-tampered-log")?;
    let signed = bytes(text(&case["signedReceipt"]));
    let receipt = verify_sequencer_signature(&signed, buyer.receipt_keys())
        .map_err(|error| format!("signature: {error:?}"))?;
    let log = &case["receiptJson"]["logs"][0];
    let served = SequencerLog::new(
        bytes(text(&log["address"]))
            .try_into()
            .map_err(|_| "address")?,
        log["topics"]
            .as_array()
            .ok_or("topics")?
            .iter()
            .map(|topic| word(text(topic)))
            .collect(),
        bytes(text(&log["data"])),
        0,
    );
    assert!(verify_logs(&receipt, vec![served]).is_err());
    assert_eq!(
        capture(&buyer, &payment, &response),
        Err(X402Error::EvidenceMismatch)
    );

    let (buyer, payment, mut response, _) = recorded("V1-layerxledger-pay-exact")?;
    capture(&buyer, &payment, &response)?;
    let original = text(&response.extensions["layerx"]["logs"][0]["data"]).to_owned();
    let mut tampered = bytes(&original);
    *tampered.last_mut().ok_or("log data")? ^= 1;
    response.extensions.get_mut("layerx").ok_or("layerx")?["logs"][0]["data"] =
        json!(format!("0x{}", hex(&tampered)));
    assert_eq!(
        capture(&buyer, &payment, &response),
        Err(X402Error::EvidenceMismatch)
    );
    Ok(())
}

#[test]
fn buyer_refuses_a_digest_over_the_signed_bytes() -> TestResult {
    let (buyer, payment, mut response, case) = recorded("V1-layerxledger-pay-exact")?;
    capture(&buyer, &payment, &response)?;
    let signed = bytes(text(&case["signedReceipt"]));
    let signed_digest: [u8; 32] = Sha256::digest(&signed).into();
    assert_ne!(signed_digest, word(text(&case["receiptDigest"])));
    response.extensions.get_mut("layerx").ok_or("layerx")?["receiptDigest"] =
        json!(hex(&signed_digest));
    assert_eq!(
        capture(&buyer, &payment, &response),
        Err(X402Error::EvidenceMismatch)
    );
    let domain_signed_digest =
        receipt_digest(&signed).map_err(|error| format!("digest: {error:?}"))?;
    assert_ne!(domain_signed_digest, word(text(&case["receiptDigest"])));
    response.extensions.get_mut("layerx").ok_or("layerx")?["receiptDigest"] =
        json!(hex(&domain_signed_digest));
    assert_eq!(
        capture(&buyer, &payment, &response),
        Err(X402Error::EvidenceMismatch)
    );
    Ok(())
}
