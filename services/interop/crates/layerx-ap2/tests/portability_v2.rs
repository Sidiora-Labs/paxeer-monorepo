use layerx_portable::{PortableReceiptV2, ReceiptKeyEntry, PORTABLE_RECEIPT_FORMAT};
use serde_json::Value;

const GUIDE: &str = include_str!("../../../PORTABILITY.md");
const FIXTURE: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");

fn hex_bytes(value: &Value) -> Vec<u8> {
    hex::decode(
        value
            .as_str()
            .expect("fixture hex string")
            .strip_prefix("0x")
            .expect("fixture hex prefix"),
    )
    .expect("fixture hex bytes")
}

fn hex32(value: &Value) -> [u8; 32] {
    hex_bytes(value).try_into().expect("32-byte fixture field")
}

#[test]
fn documented_v2_example_parses_and_verifies() {
    portability_v2_example_parses();
}

#[test]
fn portability_v2_example_parses() {
    let section = GUIDE
        .split_once("### Example (v2)\n")
        .expect("v2 example heading")
        .1
        .split("\n### ")
        .next()
        .expect("v2 example section");
    let json = section
        .split_once("```json\n")
        .expect("v2 JSON fence")
        .1
        .split_once("\n```")
        .expect("closed v2 JSON fence")
        .0;
    let portable =
        PortableReceiptV2::from_json(json.as_bytes()).expect("documented v2 example must parse");
    assert_eq!(portable.format(), PORTABLE_RECEIPT_FORMAT);
    assert_eq!(portable.format(), "layerx-receipt-proof-v2");

    let fixture: Value = serde_json::from_str(FIXTURE).expect("conformance fixture JSON");
    let recorded = fixture["cases"]
        .as_array()
        .expect("fixture cases")
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .expect("recorded payExact case");
    let key = &recorded["pinnedKey"];
    let pinned = ReceiptKeyEntry::new(
        hex32(&key["keyId"]),
        hex32(&key["publicKey"]),
        key["firstHeight"].as_u64().expect("first signing height"),
        key["lastHeight"].as_u64().expect("last signing height"),
    );
    let height = u64::from_str_radix(
        recorded["receiptJson"]["height"]
            .as_str()
            .expect("signed height")
            .strip_prefix("0x")
            .expect("height quantity prefix"),
        16,
    )
    .expect("signed height quantity");
    let signed = hex_bytes(&recorded["signedReceipt"]);
    let exported =
        PortableReceiptV2::export(&signed, &pinned, height).expect("recorded receipt must export");
    let exported_json: Value =
        serde_json::from_slice(&exported.to_json().expect("export JSON")).expect("export object");
    let documented_json: Value = serde_json::from_str(json).expect("documented JSON object");
    assert_eq!(
        documented_json, exported_json,
        "every export value is recorded"
    );
    assert_eq!(documented_json["verificationLevel"], "receipted");

    let verified = portable
        .verify_v2(std::slice::from_ref(&pinned))
        .expect("documented receipt must verify against the fixture's pinned key");
    assert_eq!(verified.signed_receipt(), signed);
    assert_eq!(verified.receipt_digest(), hex32(&recorded["receiptDigest"]));
    assert_eq!(verified.receipt_key(), &pinned);
    assert_eq!(
        verified.activity_id(),
        hex32(&recorded["receiptJson"]["txHash"])
    );
    assert_eq!(verified.height(), height);
    assert_eq!(verified.inclusion_height(), height);
    assert_eq!(verified.result_code(), 0);
}
