use layerx_portable::{
    verify_portable_json, PortableReceiptError, PortableReceiptV2, PortableVerified,
    ReceiptKeyEntry,
};
use layerx_proof::receipt::{ReceiptCheck, VerificationFailure};
use serde::{Deserialize, Serialize};
use sha3::{Digest as _, Keccak256};

#[derive(Deserialize)]
struct Fixture {
    cases: Vec<Vector>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct Vector {
    id: String,
    vector: String,
    raw_transaction: String,
    signed_receipt: String,
    receipt_digest: String,
    receipt_json: ReceiptJson,
    pinned_key: PinnedKey,
}

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
struct ReceiptJson {
    tx_hash: String,
    height: String,
    #[serde(flatten)]
    fields: serde_json::Map<String, serde_json::Value>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct PinnedKey {
    key_id: String,
    public_key: String,
    first_height: u64,
    last_height: u64,
}

fn fixture() -> Fixture {
    serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap_or_else(|error| panic!("shared sequencer fixture parses: {error:?}"))
}

fn vector(id: &str) -> Vector {
    fixture()
        .cases
        .into_iter()
        .find(|case| case.id == id)
        .unwrap_or_else(|| panic!("missing shared vector {id}"))
}

fn hex_bytes(encoded: &str) -> Vec<u8> {
    let digits = encoded.strip_prefix("0x").expect("fixture hex prefix");
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

impl Vector {
    fn key(&self) -> ReceiptKeyEntry {
        ReceiptKeyEntry::new(
            hex_bytes(&self.pinned_key.key_id)
                .try_into()
                .expect("key id"),
            hex_bytes(&self.pinned_key.public_key)
                .try_into()
                .expect("public key"),
            self.pinned_key.first_height,
            self.pinned_key.last_height,
        )
    }

    fn height(&self) -> u64 {
        u64::from_str_radix(
            self.receipt_json
                .height
                .strip_prefix("0x")
                .expect("height prefix"),
            16,
        )
        .expect("fixture height")
    }
}

#[test]
fn a_pax_transfer_vector_exports_with_its_raw_transaction() {
    let case = vector("V1-pax-transfer");
    let signed = hex_bytes(&case.signed_receipt);
    let raw = hex_bytes(&case.raw_transaction);
    let keys = [case.key()];
    let exported =
        PortableReceiptV2::export_with_transaction(&signed, &keys[0], case.height(), &raw)
            .expect("shared PAX transfer exports");
    let json = exported.to_json().expect("portable JSON");
    let wire: serde_json::Value = serde_json::from_slice(&json).expect("portable JSON object");
    assert_eq!(wire["rawTransaction"], case.raw_transaction);
    assert_eq!(wire["signedReceipt"], case.signed_receipt);
    assert_eq!(wire["receiptDigest"], case.receipt_digest);
    assert!(wire["rawTransaction"]
        .as_str()
        .unwrap()
        .bytes()
        .all(|byte| !byte.is_ascii_uppercase()));
    let verified =
        PortableReceiptV2::verify_json(&json, &keys).expect("PAX transfer verifies offline");
    assert_eq!(exported.verify_v2(&keys), Ok(verified.clone()));
    assert_eq!(verified.raw_transaction(), Some(raw.as_slice()));
    assert_eq!(verified.signed_receipt(), signed.as_slice());
    assert_eq!(
        verified.activity_id().as_slice(),
        hex_bytes(&case.receipt_json.tx_hash)
    );
    let transaction_hash: [u8; 32] = Keccak256::digest(&raw).into();
    assert_eq!(verified.activity_id(), transaction_hash);
    assert_eq!(verified.global_sequence(), 1);
    assert_eq!(verified.result_code(), 0);
    assert_eq!(verified.height(), case.height());
    assert_eq!(verified.inclusion_height(), case.height());
    assert_eq!(
        verify_portable_json(&json, &keys),
        Ok(PortableVerified::V2(verified))
    );

    for invalid in [Vec::new(), vec![0; 1_048_577]] {
        assert_eq!(
            PortableReceiptV2::export_with_transaction(&signed, &keys[0], case.height(), &invalid),
            Err(PortableReceiptError::InvalidLength("rawTransaction"))
        );
    }
    for (encoded, error) in [
        ("0xAB", PortableReceiptError::InvalidHex("rawTransaction")),
        ("0x0", PortableReceiptError::InvalidHex("rawTransaction")),
        ("00", PortableReceiptError::InvalidHex("rawTransaction")),
        ("0x", PortableReceiptError::InvalidLength("rawTransaction")),
    ] {
        let mut malformed = wire.clone();
        malformed["rawTransaction"] = encoded.into();
        assert_eq!(
            PortableReceiptV2::from_json(&serde_json::to_vec(&malformed).unwrap()),
            Err(error)
        );
    }
    let mut extended = wire;
    extended["unexpected"] = true.into();
    assert_eq!(
        PortableReceiptV2::from_json(&serde_json::to_vec(&extended).unwrap()),
        Err(PortableReceiptError::JsonShape)
    );
}

#[test]
fn a_raw_transaction_that_does_not_hash_to_the_activity_id_is_refused() {
    let case = vector("V3-activity-id-not-raw-hash");
    let signed = hex_bytes(&case.signed_receipt);
    let raw = hex_bytes(&case.raw_transaction);
    let keys = [case.key()];
    let exported = PortableReceiptV2::export(&signed, &keys[0], case.height())
        .expect("shared mismatch vector has a valid signed receipt");
    let verified = exported.verify_v2(&keys).expect("receipt alone verifies");
    let transaction_hash: [u8; 32] = Keccak256::digest(&raw).into();
    assert_ne!(verified.activity_id(), transaction_hash);
    assert_eq!(verified.raw_transaction(), None);
    assert_eq!(
        PortableReceiptV2::export_with_transaction(&signed, &keys[0], case.height(), &raw),
        Err(PortableReceiptError::RawTransactionMismatch)
    );
    let mut wire: serde_json::Value = serde_json::from_slice(&exported.to_json().unwrap()).unwrap();
    wire["rawTransaction"] = case.raw_transaction.into();
    let json = serde_json::to_vec(&wire).unwrap();
    let parsed = PortableReceiptV2::from_json(&json).expect("mismatch remains valid wire shape");
    assert_eq!(
        parsed.verify_v2(&keys),
        Err(PortableReceiptError::RawTransactionMismatch)
    );
    assert_eq!(
        PortableReceiptV2::verify_json(&json, &keys),
        Err(PortableReceiptError::RawTransactionMismatch)
    );
    assert_eq!(
        verify_portable_json(&json, &keys),
        Err(PortableReceiptError::RawTransactionMismatch)
    );
}

#[test]
fn every_valid_shared_case_exports_from_its_receipt_answer() {
    let mut exported_cases = 0;
    for case in fixture()
        .cases
        .into_iter()
        .filter(|case| matches!(case.vector.as_str(), "V1" | "V2"))
    {
        let key = case.key();
        let unrelated = ReceiptKeyEntry::new([0x99; 32], key.public_key(), 0, u64::MAX);
        let keys = [unrelated, key];
        let raw = hex_bytes(&case.raw_transaction);
        let signed = hex_bytes(&case.signed_receipt);
        let original = serde_json::to_value(&case.receipt_json).expect("receipt answer");
        for rung in ["receipted", "committed"] {
            let mut answer = original.clone();
            answer["rung"] = rung.into();
            let inclusion_height = if rung == "committed" {
                let block_number = key.last_batch_number() + 1;
                answer["blockNumber"] = format!("{block_number:#x}").into();
                answer["blockHash"] = format!("0x{}", "ab".repeat(32)).into();
                block_number
            } else {
                case.height()
            };
            for transaction in [None, Some(raw.as_slice())] {
                let exported =
                    PortableReceiptV2::from_sequencer_receipt(&answer, &keys, transaction)
                        .unwrap_or_else(|error| panic!("{} at {rung}: {error:?}", case.id));
                let json = exported.to_json().expect("exported JSON");
                let wire: serde_json::Value = serde_json::from_slice(&json).unwrap();
                assert_eq!(wire["signedReceipt"], case.signed_receipt);
                assert_eq!(wire["receiptDigest"], case.receipt_digest);
                assert_eq!(wire["receiptKey"]["keyId"], case.pinned_key.key_id);
                assert_eq!(wire["inclusionHeight"], format!("{inclusion_height:#x}"));
                if transaction.is_some() {
                    assert_eq!(wire["rawTransaction"], case.raw_transaction);
                } else {
                    assert!(wire.get("rawTransaction").is_none());
                }
                let verified = PortableReceiptV2::verify_json(&json, &keys)
                    .expect("exported shared case verifies offline");
                assert_eq!(verified.signed_receipt(), signed.as_slice());
                assert_eq!(verified.raw_transaction(), transaction);
                assert_eq!(verified.receipt_key(), &key);
                assert_eq!(verified.height(), case.height());
                assert_eq!(verified.inclusion_height(), inclusion_height);
                assert_eq!(
                    verified.activity_id().as_slice(),
                    hex_bytes(&case.receipt_json.tx_hash)
                );
                assert_eq!(
                    verified.result_code(),
                    if case.vector == "V2" { 1 } else { 0 }
                );
            }
        }
        exported_cases += 1;
    }
    assert_eq!(exported_cases, 5, "all valid shared cases executed");
}

#[test]
fn the_shared_refusal_cases_are_refused_at_export() {
    for (id, error) in [
        (
            "V3-key-not-pinned",
            PortableReceiptError::ReceiptKeyMismatch,
        ),
        (
            "V3-height-outside-key-range",
            PortableReceiptError::Receipt(VerificationFailure {
                check: ReceiptCheck::SequencerSignature,
            }),
        ),
        (
            "V3-activity-id-not-raw-hash",
            PortableReceiptError::RawTransactionMismatch,
        ),
        (
            "V3-non-canonical-bytes",
            PortableReceiptError::Receipt(VerificationFailure {
                check: ReceiptCheck::Decode,
            }),
        ),
    ] {
        let case = vector(id);
        let answer = serde_json::to_value(&case.receipt_json).unwrap();
        let raw = hex_bytes(&case.raw_transaction);
        assert_eq!(
            PortableReceiptV2::from_sequencer_receipt(&answer, &[case.key()], Some(&raw)),
            Err(error),
            "{id}"
        );
    }

    let case = vector("V1-pax-transfer");
    let keys = [case.key()];
    let answer = serde_json::to_value(&case.receipt_json).unwrap();
    let submitted = serde_json::json!({
        "rung": "submitted",
        "txHash": case.receipt_json.tx_hash,
        "nonce": "0x0"
    });
    for transaction in [None, Some(&[][..])] {
        assert_eq!(
            PortableReceiptV2::from_sequencer_receipt(&submitted, &[], transaction),
            Err(PortableReceiptError::NotReceipted)
        );
    }
    assert_eq!(
        PortableReceiptError::NotReceipted.to_string(),
        "transaction has not been receipted"
    );
    assert_eq!(
        PortableReceiptV2::from_sequencer_receipt(&answer, &[], None),
        Err(PortableReceiptError::ReceiptKeyMismatch)
    );

    let mut tampered = answer.clone();
    let mut signed = hex_bytes(&case.signed_receipt);
    *signed.last_mut().unwrap() ^= 1;
    tampered["receipt"] = format!(
        "0x{}",
        signed
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    )
    .into();
    assert_eq!(
        PortableReceiptV2::from_sequencer_receipt(&tampered, &keys, None),
        Err(PortableReceiptError::Receipt(VerificationFailure {
            check: ReceiptCheck::SequencerSignature,
        }))
    );

    let mut committed = answer.clone();
    committed["rung"] = "committed".into();
    assert_eq!(
        PortableReceiptV2::from_sequencer_receipt(&committed, &keys, None),
        Err(PortableReceiptError::JsonShape)
    );
    committed["blockNumber"] = format!("{:#x}", case.height() - 1).into();
    assert_eq!(
        PortableReceiptV2::from_sequencer_receipt(&committed, &keys, None),
        Err(PortableReceiptError::InclusionHeight)
    );
    for (field, value, error) in [
        ("rung", "unknown", PortableReceiptError::JsonShape),
        (
            "receipt",
            "0xAB",
            PortableReceiptError::InvalidHex("receipt"),
        ),
        (
            "keyId",
            "0x00",
            PortableReceiptError::InvalidLength("keyId"),
        ),
        (
            "height",
            "0x00",
            PortableReceiptError::InvalidQuantity("height"),
        ),
    ] {
        let mut malformed = answer.clone();
        malformed[field] = value.into();
        assert_eq!(
            PortableReceiptV2::from_sequencer_receipt(&malformed, &keys, None),
            Err(error),
            "{field}"
        );
    }
    for field in ["rung", "receipt", "keyId", "height"] {
        let mut malformed = answer.clone();
        malformed.as_object_mut().unwrap().remove(field);
        assert_eq!(
            PortableReceiptV2::from_sequencer_receipt(&malformed, &keys, None),
            Err(PortableReceiptError::JsonShape),
            "missing {field}"
        );
    }
}
