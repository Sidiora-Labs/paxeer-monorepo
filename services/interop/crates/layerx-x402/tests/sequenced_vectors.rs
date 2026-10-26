use layerx_x402::model::{SettlementResponse, X402Error};
use serde::Deserialize;
use serde_json::Value;

const SEQUENCED_SETTLEMENT_SUITE: &str =
    include_str!("../../../specs/conformance/x402/sequenced-settlement.json");
const RECORDED_RECEIPTS: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct Vector {
    name: String,
    valid: bool,
    expected: Expected,
    document: SettlementResponse,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct Expected {
    verdict: Verdict,
    field: String,
}

#[derive(Clone, Copy, Debug, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
enum Verdict {
    Accept,
    Pending,
    Refuse,
}

fn sequenced_settlement_vectors() -> Vec<Vector> {
    let vectors: Vec<Vector> = serde_json::from_str(SEQUENCED_SETTLEMENT_SUITE)
        .expect("sequenced settlement vectors decode as PAYMENT-RESPONSE bodies");
    assert_eq!(
        vectors
            .iter()
            .map(|vector| vector.name.as_str())
            .collect::<Vec<_>>(),
        [
            "receipted_settlement",
            "submitted_settlement",
            "committed_without_proof"
        ],
        "the suite must exercise each declared case exactly once"
    );
    vectors
}

#[test]
fn sequenced_settlement_vectors_carry_the_recorded_receipt() {
    let fixture: Value = serde_json::from_str(RECORDED_RECEIPTS)
        .expect("recorded sequencer receipt fixture decodes");
    let recorded = fixture["cases"]
        .as_array()
        .expect("receipt fixture contains cases")
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .expect("receipt fixture contains the recorded payExact case");
    let tx_hash = recorded["receiptJson"]["txHash"]
        .as_str()
        .expect("recorded payExact transaction hash");
    let signed_receipt = recorded["signedReceipt"]
        .as_str()
        .expect("recorded payExact signed receipt");
    let receipt_digest = recorded["receiptDigest"]
        .as_str()
        .expect("recorded payExact receipt digest");
    assert!(!tx_hash.is_empty());
    assert!(!signed_receipt.is_empty());
    assert!(!receipt_digest.is_empty());

    for vector in sequenced_settlement_vectors() {
        let layerx = vector
            .document
            .extensions
            .get("layerx")
            .and_then(Value::as_object)
            .expect("every vector carries a LayerX receipt evidence object");
        assert_eq!(layerx["receiptDigest"], receipt_digest, "{}", vector.name);
        match vector.name.as_str() {
            "receipted_settlement" | "submitted_settlement" => {
                assert_eq!(vector.document.transaction, tx_hash, "{}", vector.name);
                assert_eq!(layerx["signedReceipt"], signed_receipt, "{}", vector.name);
                assert_eq!(layerx.len(), 2, "{}", vector.name);
            }
            "committed_without_proof" => {
                assert!(vector.document.transaction.is_empty(), "{}", vector.name);
                assert_eq!(layerx.len(), 1, "{}", vector.name);
                assert!(!layerx.contains_key("signedReceipt"), "{}", vector.name);
            }
            name => panic!("unexpected sequenced settlement case: {name}"),
        }
        assert_eq!(vector.document.network, "eip155:125", "{}", vector.name);
        assert_eq!(
            vector.document.network,
            format!("eip155:{}", fixture["chainId"]),
            "{}",
            vector.name
        );
    }
}

#[test]
fn sequenced_settlement_vectors_validate_on_the_wire() {
    for vector in sequenced_settlement_vectors() {
        let (verdict, field) = match vector.name.as_str() {
            "receipted_settlement" => (Verdict::Accept, "/success"),
            "submitted_settlement" => (Verdict::Pending, "/errorReason"),
            "committed_without_proof" => (Verdict::Refuse, "/transaction"),
            name => panic!("unexpected sequenced settlement case: {name}"),
        };
        assert_eq!(vector.expected.verdict, verdict, "{}", vector.name);
        assert_eq!(vector.expected.field, field, "{}", vector.name);
        assert_eq!(vector.valid, verdict != Verdict::Refuse, "{}", vector.name);
        let document = serde_json::to_value(&vector.document)
            .expect("PAYMENT-RESPONSE serializes as a wire document");
        let expected_validation = match verdict {
            Verdict::Accept => {
                assert!(vector.document.success, "{}", vector.name);
                assert_eq!(vector.document.error_reason, None, "{}", vector.name);
                assert_eq!(document.pointer(field), Some(&Value::Bool(true)));
                Ok(())
            }
            Verdict::Pending => {
                assert!(!vector.document.success, "{}", vector.name);
                assert_eq!(
                    document.pointer(field),
                    Some(&Value::from("settlement_pending")),
                    "{}",
                    vector.name
                );
                Ok(())
            }
            Verdict::Refuse => {
                assert!(vector.document.success, "{}", vector.name);
                assert_eq!(vector.document.error_reason, None, "{}", vector.name);
                assert_eq!(
                    document.pointer(field),
                    Some(&Value::from("")),
                    "{}",
                    vector.name
                );
                Err(X402Error::EvidenceMissing)
            }
        };
        assert_eq!(
            vector.document.validate_wire(),
            expected_validation,
            "{}: {:?} decided on {}",
            vector.name,
            vector.expected.verdict,
            vector.expected.field
        );
    }
}
