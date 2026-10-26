use layerx_interop_gateway::error::GatewayError;
use layerx_interop_gateway::evidence::{canonical_receipt_digest, SettlementRung};
use layerx_proof::receipt::{ReceiptCheck, VerificationFailure};

const FIXTURE: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
));

fn recorded_hex(case: &str, field: &str) -> Vec<u8> {
    let key = format!("\"{field}\": \"");
    let value = case
        .split_once(&key)
        .and_then(|(_, rest)| rest.split_once('"'))
        .map(|(value, _)| value)
        .unwrap_or_else(|| panic!("missing fixture field {field}"));
    let hex = value
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("fixture field {field} is not hex"));
    assert_eq!(hex.len() % 2, 0, "fixture field {field} has odd length");
    (0..hex.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&hex[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("invalid hex in fixture field {field}: {error}"))
        })
        .collect()
}

#[test]
fn settlement_rung_labels_are_submitted_receipted_and_committed() {
    assert_eq!(SettlementRung::Submitted.label(), "submitted");
    assert_eq!(SettlementRung::Receipted.label(), "receipted");
    assert_eq!(
        SettlementRung::Committed {
            block_number: 1001,
            block_hash: [0x42; 32],
        }
        .label(),
        "committed"
    );
}

#[test]
fn canonical_receipt_digest_matches_the_recorded_pay_exact_vector() {
    let case = FIXTURE
        .split_once("\"id\": \"V1-layerxledger-pay-exact\"")
        .map(|(_, rest)| rest)
        .unwrap_or_else(|| panic!("missing recorded pay-exact case"));
    let case = case
        .split("\"id\":")
        .next()
        .unwrap_or_else(|| panic!("missing recorded pay-exact case body"));
    let signed_receipt = recorded_hex(case, "signedReceipt");
    let expected: [u8; 32] = recorded_hex(case, "receiptDigest")
        .try_into()
        .unwrap_or_else(|_| panic!("recorded receipt digest must be 32 bytes"));
    assert_eq!(canonical_receipt_digest(&signed_receipt), Ok(expected));

    let missing_signature = Err(GatewayError::ReceiptRejected(VerificationFailure {
        check: ReceiptCheck::MissingSignature,
    }));
    assert_eq!(canonical_receipt_digest(&[]), missing_signature);
    let unsigned_end = signed_receipt.len() - 69;
    assert_eq!(
        canonical_receipt_digest(&signed_receipt[..unsigned_end]),
        missing_signature
    );
    let mut invalid_flag = signed_receipt.clone();
    invalid_flag[unsigned_end] = 0;
    assert_eq!(canonical_receipt_digest(&invalid_flag), missing_signature);
    let mut invalid_length = signed_receipt;
    invalid_length[unsigned_end + 4] = 63;
    assert_eq!(canonical_receipt_digest(&invalid_length), missing_signature);
}
