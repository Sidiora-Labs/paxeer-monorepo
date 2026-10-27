//! Offline portable receipt verification against independently pinned receipt keys.
//! The Python bridge implements FORMAT.md without importing any LayerX SDK.

use layerx_portable::{
    PortableReceiptError, PortableReceiptV2, ReceiptKeyEntry, PORTABLE_RECEIPT_FORMAT_V2,
};
use serde_json::{json, Value};

const SEQUENCER_VECTORS: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");

pub struct IndependentVerifier {
    name: &'static str,
}

impl IndependentVerifier {
    #[must_use]
    pub const fn new(name: &'static str) -> Self {
        Self { name }
    }

    /// # Errors
    /// Refuses invalid proofs, signatures, digests, transactions or unpinned keys.
    pub fn verify_vector_against_pinned_keys(
        &self,
        vector_json: &[u8],
        pinned_keys: &[ReceiptKeyEntry],
    ) -> Result<VerificationOutcome, PortableReceiptError> {
        let verified = PortableReceiptV2::verify_json(vector_json, pinned_keys)?;
        Ok(VerificationOutcome {
            verifier_name: self.name,
            receipt_digest: verified.receipt_digest(),
            activity_id: verified.activity_id(),
            result_code: verified.result_code(),
            height: verified.height(),
        })
    }

    pub fn verify_all_golden_vectors(&self) -> Result<Vec<VerificationOutcome>, String> {
        positive_vectors()?
            .iter()
            .map(|case| {
                let keys = pinned_keys(case)?;
                self.verify_vector_against_pinned_keys(&export_vector(case, &keys)?, &keys)
                    .map_err(|error| error.to_string())
            })
            .collect()
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerificationOutcome {
    pub verifier_name: &'static str,
    pub receipt_digest: [u8; 32],
    pub activity_id: [u8; 32],
    pub result_code: i32,
    pub height: u64,
}

fn positive_vectors() -> Result<Vec<Value>, String> {
    let fixture: Value = serde_json::from_str(SEQUENCER_VECTORS).map_err(|e| e.to_string())?;
    Ok(fixture["cases"]
        .as_array()
        .ok_or("sequencer cases")?
        .iter()
        .filter(|case| case["expected"]["refusedAt"].is_null())
        .cloned()
        .collect())
}

fn decode_hex(value: &Value) -> Result<Vec<u8>, String> {
    let digits = value
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .ok_or("hex string")?;
    if digits.len() % 2 != 0 {
        return Err("odd hex length".into());
    }
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let text = std::str::from_utf8(pair).map_err(|e| e.to_string())?;
            u8::from_str_radix(text, 16).map_err(|e| e.to_string())
        })
        .collect()
}

fn pinned_keys(case: &Value) -> Result<Vec<ReceiptKeyEntry>, String> {
    let key = &case["pinnedKey"];
    let fixed = |field| {
        decode_hex(&key[field])?
            .try_into()
            .map_err(|_| "key must be 32 bytes".to_owned())
    };
    Ok(vec![ReceiptKeyEntry::new(
        fixed("keyId")?,
        fixed("publicKey")?,
        key["firstHeight"].as_u64().ok_or("first height")?,
        key["lastHeight"].as_u64().ok_or("last height")?,
    )])
}

fn export_vector(case: &Value, keys: &[ReceiptKeyEntry]) -> Result<Vec<u8>, String> {
    let raw = decode_hex(&case["rawTransaction"])?;
    PortableReceiptV2::from_sequencer_receipt(&case["receiptJson"], keys, Some(&raw))
        .and_then(|portable| portable.to_json())
        .map_err(|error| error.to_string())
}

fn flip_byte(value: &Value, index: usize) -> Result<Value, String> {
    let mut bytes = decode_hex(value)?;
    bytes[index] ^= 1;
    let mut encoded = String::from("0x");
    for byte in bytes {
        use std::fmt::Write as _;
        write!(encoded, "{byte:02x}").map_err(|e| e.to_string())?;
    }
    Ok(json!(encoded))
}

#[test]
fn independent_verifier_processes_all_vectors() -> Result<(), String> {
    let results = IndependentVerifier::new("all-vectors").verify_all_golden_vectors()?;
    assert_eq!(results.len(), 5);
    assert_eq!(
        results
            .iter()
            .filter(|result| result.result_code == 0)
            .count(),
        4
    );
    assert!(results.iter().any(|result| result.result_code != 0));
    Ok(())
}

#[test]
fn independent_verifier_no_layerx_infrastructure_required() -> Result<(), String> {
    let verifier = IndependentVerifier::new("standalone-verifier");
    let case = &positive_vectors()?[0];
    let keys = pinned_keys(case)?;
    let outcome = verifier
        .verify_vector_against_pinned_keys(&export_vector(case, &keys)?, &keys)
        .map_err(|error| error.to_string())?;
    assert_eq!(
        outcome.receipt_digest.to_vec(),
        decode_hex(&case["receiptDigest"])?
    );
    Ok(())
}

#[test]
fn portable_format_constant_is_stable() {
    assert_eq!(PORTABLE_RECEIPT_FORMAT_V2, "layerx-receipt-proof-v2");
}

#[test]
fn independent_implementation_can_enumerate_vectors() -> Result<(), String> {
    let vectors = positive_vectors()?;
    assert_eq!(vectors.len(), 5);
    for case in vectors {
        let bytes = export_vector(&case, &pinned_keys(&case)?)?;
        let document: Value = serde_json::from_slice(&bytes).map_err(|e| e.to_string())?;
        assert_eq!(document["format"], PORTABLE_RECEIPT_FORMAT_V2);
        assert_eq!(document["signedReceipt"], case["signedReceipt"]);
        assert_eq!(document["rawTransaction"], case["rawTransaction"]);
    }
    Ok(())
}

#[test]
fn independent_verifier_accepts_a_v2_proof_and_refuses_substitutions() -> Result<(), String> {
    let verifier = IndependentVerifier::new("independent-v2-verifier");
    let vectors = positive_vectors()?;
    assert_eq!(vectors.len(), 5);
    for case in vectors {
        let keys = pinned_keys(&case)?;
        let height = u64::from_str_radix(
            case["receiptJson"]["height"]
                .as_str()
                .and_then(|value| value.strip_prefix("0x"))
                .ok_or("fixture height")?,
            16,
        )
        .map_err(|error| error.to_string())?;
        let bytes = export_vector(&case, &keys)?;
        let document: Value = serde_json::from_slice(&bytes).map_err(|e| e.to_string())?;
        let outcome = verifier
            .verify_vector_against_pinned_keys(&bytes, &keys)
            .map_err(|error| error.to_string())?;
        assert_eq!(
            outcome.receipt_digest.to_vec(),
            decode_hex(&case["receiptDigest"])?
        );
        assert_eq!(
            outcome.activity_id.to_vec(),
            decode_hex(&case["receiptJson"]["txHash"])?
        );
        assert_eq!(
            outcome.result_code == 0,
            case["receiptJson"]["status"] == "0x1"
        );
        assert_eq!(outcome.height, height);
        assert_eq!(document["signedReceipt"], case["signedReceipt"]);
        assert_eq!(document["rawTransaction"], case["rawTransaction"]);

        for field in ["signedReceipt", "receiptDigest", "rawTransaction"] {
            let length = decode_hex(&document[field])?.len();
            for index in [0, length / 2, length - 1] {
                let mut altered = document.clone();
                altered[field] = flip_byte(&document[field], index)?;
                assert!(
                    verifier
                        .verify_vector_against_pinned_keys(altered.to_string().as_bytes(), &keys)
                        .is_err(),
                    "{}: accepted {field} byte {index}",
                    case["id"]
                );
            }
        }
        for field in ["keyId", "publicKey"] {
            let mut altered = document.clone();
            altered["receiptKey"][field] = flip_byte(&document["receiptKey"][field], 0)?;
            assert!(verifier
                .verify_vector_against_pinned_keys(altered.to_string().as_bytes(), &keys)
                .is_err());
        }
        let key = keys[0];
        for (field, endpoint) in [
            ("firstHeight", key.first_batch_number()),
            ("lastHeight", key.last_batch_number()),
        ] {
            let mut altered = document.clone();
            altered["receiptKey"][field] = json!(format!("{:#x}", endpoint ^ 1));
            assert!(verifier
                .verify_vector_against_pinned_keys(altered.to_string().as_bytes(), &keys)
                .is_err());
        }
        let mut later_inclusion = document.clone();
        later_inclusion["inclusionHeight"] = json!(format!("{:#x}", key.last_batch_number() + 1));
        assert_eq!(
            verifier
                .verify_vector_against_pinned_keys(later_inclusion.to_string().as_bytes(), &keys)
                .map_err(|error| error.to_string())?,
            outcome,
            "the key window covers signed height, regardless of later inclusion"
        );
        for (first, last) in [
            (height, height),
            (height + 1, key.last_batch_number() + 1),
            (0, height - 1),
        ] {
            let narrowed_key =
                ReceiptKeyEntry::new(key.sequencer_id(), key.public_key(), first, last);
            let mut narrowed = later_inclusion.clone();
            narrowed["receiptKey"]["firstHeight"] = json!(format!("{first:#x}"));
            narrowed["receiptKey"]["lastHeight"] = json!(format!("{last:#x}"));
            let result = verifier.verify_vector_against_pinned_keys(
                narrowed.to_string().as_bytes(),
                &[narrowed_key],
            );
            if first == height && last == height {
                assert_eq!(result.map_err(|error| error.to_string())?, outcome);
            } else {
                assert!(
                    result.is_err(),
                    "the carried key must cover the signed height"
                );
            }
        }
        let mut wrong_public_key = key.public_key();
        wrong_public_key[0] ^= 1;
        for untrusted in [
            vec![],
            vec![ReceiptKeyEntry::new(
                key.sequencer_id(),
                wrong_public_key,
                key.first_batch_number(),
                key.last_batch_number(),
            )],
            vec![ReceiptKeyEntry::new(
                key.sequencer_id(),
                key.public_key(),
                height + 1,
                key.last_batch_number(),
            )],
            vec![ReceiptKeyEntry::new(
                key.sequencer_id(),
                key.public_key(),
                0,
                height - 1,
            )],
            vec![key, key],
        ] {
            assert!(verifier
                .verify_vector_against_pinned_keys(&bytes, &untrusted)
                .is_err());
        }
        let mut altered = document.clone();
        altered["inclusionHeight"] = json!(format!("{:#x}", height - 1));
        assert!(verifier
            .verify_vector_against_pinned_keys(altered.to_string().as_bytes(), &keys)
            .is_err());
    }
    Ok(())
}

#[test]
fn python_verifier_accepts_the_same_v2_proof() -> Result<(), String> {
    let vectors = positive_vectors()?;
    assert_eq!(vectors.len(), 5);
    for case in vectors {
        verify_python_export(&export_vector(&case, &pinned_keys(&case)?)?, &case)?;
    }
    Ok(())
}

fn verify_python_export(bytes: &[u8], fixture: &Value) -> Result<(), String> {
    let output = run_python_verifier(bytes)?;
    assert!(
        output.status.success(),
        "Python receipt verification failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    let independent: Value =
        serde_json::from_slice(&output.stdout).map_err(|error| error.to_string())?;
    assert_eq!(independent["receipt_digest"], fixture["receiptDigest"]);
    assert_eq!(independent["activity_id"], fixture["receiptJson"]["txHash"]);
    assert_eq!(independent["mutations_refused"], 14);
    assert_eq!(independent["key_lists_refused"], 5);

    let mut altered: Value = serde_json::from_slice(bytes).map_err(|error| error.to_string())?;
    altered["rawTransaction"] = flip_byte(&altered["rawTransaction"], 0)?;
    let refused = run_python_verifier(altered.to_string().as_bytes())?;
    assert!(
        !refused.status.success(),
        "Python accepted a substituted raw transaction"
    );
    assert!(
        String::from_utf8_lossy(&refused.stderr)
            .contains("raw transaction does not match activity id"),
        "Python failed without checking the transaction binding: {}",
        String::from_utf8_lossy(&refused.stderr)
    );
    Ok(())
}

fn run_python_verifier(bytes: &[u8]) -> Result<std::process::Output, String> {
    use std::io::Write as _;
    use std::path::Path;
    use std::process::{Command, Stdio};
    let directory = Path::new(env!("CARGO_MANIFEST_DIR"));
    let mut python =
        Command::new(std::env::var_os("LAYERX_TEST_PYTHON").unwrap_or_else(|| "python3".into()))
            .arg(directory.join("tests/python_portable_verifier.py"))
            .arg(
                directory.join(
                    "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json",
                ),
            )
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .map_err(|error| format!("independent Python verifier: {error}"))?;
    python
        .stdin
        .take()
        .ok_or("independent verifier stdin")?
        .write_all(bytes)
        .map_err(|error| format!("portable verifier input: {error}"))?;
    python.wait_with_output().map_err(|error| error.to_string())
}
