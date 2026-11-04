#[path = "../examples/portable_v2.rs"]
mod portable_v2;

use layerx_portable::{PortableReceiptError, PortableReceiptV2, ReceiptKeyEntry};
use layerx_proof::receipt::{ReceiptCheck, VerificationFailure};
use serde_json::Value;

const GUIDE: &str = include_str!("../PORTABILITY.md");
const EXAMPLE: &str = include_str!("../examples/portable_v2.rs");
const FIXTURE: &str =
    include_str!("../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");

fn cases() -> Vec<Value> {
    serde_json::from_str::<Value>(FIXTURE).expect("recorded receipt corpus")["cases"]
        .as_array()
        .expect("recorded cases")
        .clone()
}

fn case(id: &str) -> Value {
    cases()
        .into_iter()
        .find(|case| case["id"] == id)
        .expect("recorded case")
}

fn bytes(value: &Value) -> Vec<u8> {
    let digits = value
        .as_str()
        .expect("fixture hex string")
        .strip_prefix("0x")
        .expect("fixture hex prefix");
    assert_eq!(digits.len() % 2, 0);
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("fixture ASCII"), 16)
                .expect("fixture hex digits")
        })
        .collect()
}

fn key(case: &Value) -> ReceiptKeyEntry {
    let key = &case["pinnedKey"];
    ReceiptKeyEntry::new(
        bytes(&key["keyId"]).try_into().expect("32-byte key ID"),
        bytes(&key["publicKey"])
            .try_into()
            .expect("32-byte public key"),
        key["firstHeight"].as_u64().expect("first height"),
        key["lastHeight"].as_u64().expect("last height"),
    )
}

fn height(case: &Value) -> u64 {
    u64::from_str_radix(
        case["receiptJson"]["height"]
            .as_str()
            .expect("height quantity")
            .strip_prefix("0x")
            .expect("quantity prefix"),
        16,
    )
    .expect("signed height")
}

fn fenced<'a>(section: &'a str, language: &str) -> &'a str {
    section
        .split_once(&format!("```{language}\n"))
        .expect("documented code fence")
        .1
        .split_once("\n```")
        .expect("closed code fence")
        .0
}

fn signature_failure() -> PortableReceiptError {
    PortableReceiptError::Receipt(VerificationFailure {
        check: ReceiptCheck::SequencerSignature,
    })
}

#[test]
fn portable_v1_instructions_example_matches_compiled_consumer() {
    assert_eq!(fenced(GUIDE, "rust"), EXAMPLE.trim_end());
    assert!(GUIDE.contains("## Historical v1 (unsupported archival context)"));
    let section = GUIDE
        .split_once("### Example (v2)\n")
        .expect("v2 example heading")
        .1;
    let documented = fenced(section, "json");
    let recorded = case("V1-layerxledger-pay-exact");
    let keys = [key(&recorded)];
    let exported = portable_v2::export_receipt(&recorded["receiptJson"], &keys, None)
        .expect("real recorded RPC answer exports");
    assert_eq!(
        serde_json::from_str::<Value>(documented).expect("documented JSON"),
        serde_json::from_slice::<Value>(&exported).expect("exported JSON")
    );
    let verified = portable_v2::verify_receipt(documented.as_bytes(), &keys)
        .expect("documented consumer verifies the guide's JSON");
    assert_eq!(verified.signed_receipt(), bytes(&recorded["signedReceipt"]));
    assert_eq!(verified.receipt_key(), &keys[0]);
    assert_eq!(verified.height(), height(&recorded));
    assert_eq!(verified.inclusion_height(), height(&recorded));
    assert_eq!(verified.result_code(), 0);
}

#[test]
fn portable_v1_instructions_recorded_producer_transport_consumer() {
    let mut exercised = 0;
    for recorded in cases()
        .into_iter()
        .filter(|case| matches!(case["vector"].as_str(), Some("V1" | "V2")))
    {
        let keys = [key(&recorded)];
        let raw = bytes(&recorded["rawTransaction"]);
        for rung in ["receipted", "committed"] {
            let mut answer = recorded["receiptJson"].clone();
            answer["rung"] = rung.into();
            let inclusion_height = if rung == "committed" {
                let later = keys[0].last_batch_number() + 1;
                answer["blockNumber"] = format!("{later:#x}").into();
                later
            } else {
                height(&recorded)
            };
            let json = portable_v2::export_receipt(&answer, &keys, Some(&raw))
                .expect("recorded signed receipt exports to transport bytes");
            let verified = portable_v2::verify_receipt(&json, &keys)
                .expect("offline public consumer verifies transported bytes");
            assert_eq!(verified.signed_receipt(), bytes(&recorded["signedReceipt"]));
            assert_eq!(verified.raw_transaction(), Some(raw.as_slice()));
            assert_eq!(
                verified.activity_id().as_slice(),
                bytes(&recorded["receiptJson"]["txHash"])
            );
            assert_eq!(
                verified.receipt_digest().as_slice(),
                bytes(&recorded["receiptDigest"])
            );
            assert_eq!(verified.receipt_key(), &keys[0]);
            assert_eq!(verified.height(), height(&recorded));
            assert_eq!(
                verified.result_code(),
                if recorded["vector"] == "V2" { 1 } else { 0 },
                "a valid signature preserves a rejected execution result"
            );
            assert_eq!(verified.inclusion_height(), inclusion_height);
            let transported: Value = serde_json::from_slice(&json).expect("transport JSON");
            assert_eq!(transported["verificationLevel"], "receipted");
        }
        exercised += 1;
    }
    assert_eq!(
        exercised, 6,
        "all recorded successful and reverted receipts"
    );
}

#[test]
fn portable_v1_instructions_height_ranged_keys_use_signed_height() {
    let recorded = case("V1-pax-transfer");
    let pinned = key(&recorded);
    let signed_height = height(&recorded);
    let signed = bytes(&recorded["signedReceipt"]);
    let inclusion_height = pinned.last_batch_number() + 1;
    for (first, last) in [
        (signed_height, signed_height + 1),
        (signed_height - 1, signed_height),
    ] {
        let keys = [ReceiptKeyEntry::new(
            pinned.sequencer_id(),
            pinned.public_key(),
            first,
            last,
        )];
        let exported = PortableReceiptV2::export(&signed, &keys[0], inclusion_height)
            .expect("inclusive signed-height endpoint exports");
        let json = exported.to_json().expect("transport JSON");
        let verified = portable_v2::verify_receipt(&json, &keys)
            .expect("later inclusion outside key window is allowed");
        assert_eq!(verified.height(), signed_height);
        assert_eq!(verified.inclusion_height(), inclusion_height);
        assert_eq!(verified.receipt_key(), &keys[0]);
        for (first, last) in [
            (signed_height + 1, inclusion_height),
            (signed_height - 2, signed_height - 1),
        ] {
            let outside = [ReceiptKeyEntry::new(
                pinned.sequencer_id(),
                pinned.public_key(),
                first,
                last,
            )];
            assert_eq!(
                portable_v2::verify_receipt(&json, &outside),
                Err(signature_failure()),
                "a matching key with a window excluding the signed height is refused"
            );
        }
        assert_eq!(
            portable_v2::verify_receipt(&json, &[keys[0], keys[0]]),
            Err(signature_failure()),
            "ambiguous pinned key IDs are refused"
        );
    }
}

#[test]
fn portable_v1_instructions_proof_cannot_supply_trust_or_enable_v1() {
    let recorded = case("V1-layerxledger-pay-exact");
    let keys = [key(&recorded)];
    let json = portable_v2::export_receipt(&recorded["receiptJson"], &keys, None)
        .expect("recorded proof exports");
    assert_eq!(
        portable_v2::verify_receipt(&json, &[]),
        Err(signature_failure()),
        "a carried receipt key does not become a trust anchor"
    );
    let mut carried: Value = serde_json::from_slice(&json).expect("exported JSON");
    carried["receiptKey"]["lastHeight"] = format!("{:#x}", keys[0].last_batch_number() + 1).into();
    assert_eq!(
        portable_v2::verify_receipt(&serde_json::to_vec(&carried).expect("JSON"), &keys),
        Err(PortableReceiptError::ReceiptKeyMismatch),
        "every carried key field must equal the independent pin"
    );
    let mut historical: Value = serde_json::from_slice(&json).expect("exported JSON");
    historical["format"] = "layerx-receipt-proof-v1".into();
    let historical_json = serde_json::to_vec(&historical).expect("historical format JSON");
    assert_eq!(
        portable_v2::verify_receipt(&historical_json, &keys),
        Err(PortableReceiptError::UnsupportedFormat)
    );
    assert_eq!(
        PortableReceiptV2::from_json(&historical_json),
        Err(PortableReceiptError::UnsupportedFormat)
    );
    let submitted = serde_json::json!({"rung": "submitted"});
    assert_eq!(
        portable_v2::export_receipt(&submitted, &keys, None),
        Err(PortableReceiptError::NotReceipted)
    );
}
