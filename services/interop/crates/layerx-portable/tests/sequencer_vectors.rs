use layerx_portable::{
    verify_portable_json, PortableReceiptError, PortableReceiptV2, PortableVerified,
    ReceiptKeyEntry,
};
use serde::Deserialize;
use sha3::{Digest as _, Keccak256};

#[derive(Deserialize)]
struct Fixture {
    cases: Vec<Vector>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct Vector {
    id: String,
    raw_transaction: String,
    signed_receipt: String,
    receipt_digest: String,
    receipt_json: ReceiptJson,
    pinned_key: PinnedKey,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ReceiptJson {
    tx_hash: String,
    height: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct PinnedKey {
    key_id: String,
    public_key: String,
    first_height: u64,
    last_height: u64,
}

fn vector(id: &str) -> Vector {
    let fixture: Fixture = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap_or_else(|error| panic!("shared sequencer fixture parses: {error:?}"));
    fixture
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
        verify_portable_json(&json, None, &keys),
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
        verify_portable_json(&json, None, &keys),
        Err(PortableReceiptError::RawTransactionMismatch)
    );
}
