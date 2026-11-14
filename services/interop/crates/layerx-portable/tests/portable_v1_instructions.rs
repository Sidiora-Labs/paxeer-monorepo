#[path = "../../../examples/portable_v2.rs"]
mod portable_v2;

use layerx_portable::{PortableReceiptError, PortableReceiptV2, ReceiptKeyEntry};
use layerx_proof::receipt::{ReceiptCheck, VerificationFailure};
use serde_json::Value;

const GUIDE: &str = include_str!("../../../../../docs/interop/PORTABILITY.md");
const EXAMPLE: &str = include_str!("../../../examples/portable_v2.rs");
const FIXTURE: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");

fn cases() -> Vec<Value> {
    serde_json::from_str::<Value>(FIXTURE)
        .unwrap_or_else(|error| panic!("recorded receipt corpus: {error:?}"))["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("recorded cases"))
        .clone()
}

fn case(id: &str) -> Value {
    cases()
        .into_iter()
        .find(|case| case["id"] == id)
        .unwrap_or_else(|| panic!("recorded case"))
}

fn bytes(value: &Value) -> Vec<u8> {
    let digits = value
        .as_str()
        .unwrap_or_else(|| panic!("fixture hex string"))
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("fixture hex prefix"));
    assert_eq!(digits.len() % 2, 0);
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(
                std::str::from_utf8(pair)
                    .unwrap_or_else(|error| panic!("fixture ASCII: {error:?}")),
                16,
            )
            .unwrap_or_else(|error| panic!("fixture hex digits: {error:?}"))
        })
        .collect()
}

fn key(case: &Value) -> ReceiptKeyEntry {
    let key = &case["pinnedKey"];
    ReceiptKeyEntry::new(
        bytes(&key["keyId"])
            .try_into()
            .unwrap_or_else(|error| panic!("32-byte key ID: {error:?}")),
        bytes(&key["publicKey"])
            .try_into()
            .unwrap_or_else(|error| panic!("32-byte public key: {error:?}")),
        key["firstHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("first height")),
        key["lastHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("last height")),
    )
}

fn height(case: &Value) -> u64 {
    u64::from_str_radix(
        case["receiptJson"]["height"]
            .as_str()
            .unwrap_or_else(|| panic!("height quantity"))
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("quantity prefix")),
        16,
    )
    .unwrap_or_else(|error| panic!("signed height: {error:?}"))
}

fn fenced<'a>(section: &'a str, language: &str) -> &'a str {
    section
        .split_once(&format!("```{language}\n"))
        .unwrap_or_else(|| panic!("documented code fence"))
        .1
        .split_once("\n```")
        .unwrap_or_else(|| panic!("closed code fence"))
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
        .unwrap_or_else(|| panic!("v2 example heading"))
        .1;
    let documented = fenced(section, "json");
    let recorded = case("V1-layerxledger-pay-exact");
    let keys = [key(&recorded)];
    let exported = portable_v2::export_receipt(&recorded["receiptJson"], &keys, None)
        .unwrap_or_else(|error| panic!("real recorded RPC answer exports: {error:?}"));
    assert_eq!(
        serde_json::from_str::<Value>(documented)
            .unwrap_or_else(|error| panic!("documented JSON: {error:?}")),
        serde_json::from_slice::<Value>(&exported)
            .unwrap_or_else(|error| panic!("exported JSON: {error:?}"))
    );
    let verified =
        portable_v2::verify_receipt(documented.as_bytes(), &keys).unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "documented consumer verifies the guide's JSON"
            )
        });
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
            let json =
                portable_v2::export_receipt(&answer, &keys, Some(&raw)).unwrap_or_else(|error| {
                    panic!(
                        "{}: {error:?}",
                        "recorded signed receipt exports to transport bytes"
                    )
                });
            let verified = portable_v2::verify_receipt(&json, &keys).unwrap_or_else(|error| {
                panic!(
                    "{}: {error:?}",
                    "offline public consumer verifies transported bytes"
                )
            });
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
                i32::from(recorded["vector"] == "V2"),
                "a valid signature preserves a rejected execution result"
            );
            assert_eq!(verified.inclusion_height(), inclusion_height);
            let transported: Value = serde_json::from_slice(&json)
                .unwrap_or_else(|error| panic!("transport JSON: {error:?}"));
            assert_eq!(transported["verificationLevel"], "receipted");
        }
        exercised += 1;
    }
    assert_eq!(
        exercised, 7,
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
            .unwrap_or_else(|error| panic!("inclusive signed-height endpoint exports: {error:?}"));
        let json = exported
            .to_json()
            .unwrap_or_else(|error| panic!("transport JSON: {error:?}"));
        let verified = portable_v2::verify_receipt(&json, &keys).unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "later inclusion outside key window is allowed"
            )
        });
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
        .unwrap_or_else(|error| panic!("recorded proof exports: {error:?}"));
    assert_eq!(
        portable_v2::verify_receipt(&json, &[]),
        Err(signature_failure()),
        "a carried receipt key does not become a trust anchor"
    );
    let mut carried: Value =
        serde_json::from_slice(&json).unwrap_or_else(|error| panic!("exported JSON: {error:?}"));
    carried["receiptKey"]["lastHeight"] = format!("{:#x}", keys[0].last_batch_number() + 1).into();
    assert_eq!(
        portable_v2::verify_receipt(
            &serde_json::to_vec(&carried).unwrap_or_else(|error| panic!("JSON: {error:?}")),
            &keys
        ),
        Err(PortableReceiptError::ReceiptKeyMismatch),
        "every carried key field must equal the independent pin"
    );
    let mut historical: Value =
        serde_json::from_slice(&json).unwrap_or_else(|error| panic!("exported JSON: {error:?}"));
    historical["format"] = "layerx-receipt-proof-v1".into();
    let historical_json = serde_json::to_vec(&historical)
        .unwrap_or_else(|error| panic!("historical format JSON: {error:?}"));
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
