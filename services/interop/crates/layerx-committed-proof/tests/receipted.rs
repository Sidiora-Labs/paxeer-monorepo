use layerx_committed_proof::{verify_receipted, ProofError};

const FIXTURE: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");

fn string<'a>(case: &'a serde_json::Value, name: &str) -> &'a str {
    case[name]
        .as_str()
        .unwrap_or_else(|| panic!("invalid fixture string {name}"))
}

fn number(case: &serde_json::Value, name: &str) -> u64 {
    case[name]
        .as_u64()
        .unwrap_or_else(|| panic!("invalid fixture number {name}"))
}

fn bytes(case: &serde_json::Value, name: &str) -> Vec<u8> {
    let hex = string(case, name)
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("missing fixture hex prefix {name}"));
    assert_eq!(hex.len() % 2, 0, "odd fixture hex length {name}");
    (0..hex.len())
        .step_by(2)
        .map(|offset| {
            u8::from_str_radix(&hex[offset..offset + 2], 16)
                .unwrap_or_else(|error| panic!("invalid fixture bytes {name}: {error}"))
        })
        .collect()
}

fn hash(case: &serde_json::Value, name: &str) -> [u8; 32] {
    bytes(case, name)
        .try_into()
        .unwrap_or_else(|_| panic!("invalid fixture hash length {name}"))
}

#[test]
fn receipted_fixture_cases_verify_or_refuse() {
    let mut accepted = 0;
    let mut refused = 0;
    let mut failures = Vec::new();
    let fixture: serde_json::Value =
        serde_json::from_str(FIXTURE).unwrap_or_else(|error| panic!("invalid fixture: {error}"));
    for case in fixture["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("missing fixture cases"))
    {
        let id = string(case, "id");
        let vector = string(case, "vector");
        if vector != "V1" && vector != "V3" {
            continue;
        }
        let raw = bytes(case, "rawTransaction");
        let receipt = bytes(case, "signedReceipt");
        let view = serde_json::to_vec(&case["receiptJson"])
            .unwrap_or_else(|error| panic!("{id}: invalid JSON view: {error}"));
        let previous = hash(case, "prevSeqHash");
        let pin = &case["pinnedKey"];
        let key_id = hash(pin, "keyId");
        let key = hash(pin, "publicKey");
        let first = number(pin, "firstHeight");
        let last = number(pin, "lastHeight");
        let result = verify_receipted(
            &raw,
            &receipt,
            &view,
            previous,
            &[(key_id, key, first, last)],
        );
        if vector == "V1" {
            accepted += 1;
            let expected = Ok(hash(case, "receiptDigest"));
            if result != expected {
                failures.push(format!("{id}: expected {expected:?}, got {result:?}"));
            }
            let height = u64::from_str_radix(
                string(&case["receiptJson"], "height")
                    .strip_prefix("0x")
                    .unwrap_or_else(|| panic!("{id}: missing height prefix")),
                16,
            )
            .unwrap_or_else(|error| panic!("{id}: invalid height: {error}"));
            assert_eq!(
                verify_receipted(
                    &raw,
                    &receipt,
                    &view,
                    previous,
                    &[(key_id, key, height, height)]
                ),
                expected,
                "{id}: inclusive height bounds"
            );
            for (first, last) in [(0, height - 1), (height + 1, u64::MAX)] {
                assert_eq!(
                    verify_receipted(
                        &raw,
                        &receipt,
                        &view,
                        previous,
                        &[(key_id, key, first, last)]
                    ),
                    Err(ProofError::UnpinnedReceiptSignature),
                    "{id}: pin excludes receipt height"
                );
            }
            let mut wrong_key = key;
            wrong_key[0] ^= 1;
            assert_eq!(
                verify_receipted(
                    &raw,
                    &receipt,
                    &view,
                    previous,
                    &[(key_id, wrong_key, first, last), (key_id, key, first, last)],
                ),
                expected,
                "{id}: matching standby pin"
            );
            let mut wrong_id = key_id;
            wrong_id[0] ^= 1;
            assert_eq!(
                verify_receipted(
                    &raw,
                    &receipt,
                    &view,
                    previous,
                    &[(wrong_id, key, first, last)]
                ),
                Err(ProofError::UnpinnedReceiptSignature),
                "{id}: public key alone does not pin a key id"
            );
            for name in case["receiptJson"]
                .as_object()
                .unwrap_or_else(|| panic!("{id}: view must be an object"))
                .keys()
            {
                let mut altered = case["receiptJson"].clone();
                altered[name] = serde_json::Value::Null;
                let altered = serde_json::to_vec(&altered)
                    .unwrap_or_else(|error| panic!("{id}: cannot serialize altered view: {error}"));
                assert_eq!(
                    verify_receipted(
                        &raw,
                        &receipt,
                        &altered,
                        previous,
                        &[(key_id, key, first, last)]
                    ),
                    Err(ProofError::ReceiptViewMismatch),
                    "{id}: JSON field {name} must be bound"
                );
            }
            let mut wrong_previous = previous;
            wrong_previous[0] ^= 1;
            assert_eq!(
                verify_receipted(
                    &raw,
                    &receipt,
                    &view,
                    wrong_previous,
                    &[(key_id, key, first, last)]
                ),
                Err(ProofError::ReceiptSequenceMismatch),
                "{id}: predecessor mismatch"
            );
            let mut wrong_raw = raw.clone();
            wrong_raw[0] ^= 1;
            assert_eq!(
                verify_receipted(
                    &wrong_raw,
                    &receipt,
                    &view,
                    previous,
                    &[(key_id, key, first, last)]
                ),
                Err(ProofError::ReceiptActivityMismatch),
                "{id}: raw transaction mismatch"
            );
            let mut trailing = receipt.clone();
            trailing.push(0);
            assert_eq!(
                verify_receipted(
                    &raw,
                    &trailing,
                    &view,
                    previous,
                    &[(key_id, key, first, last)]
                ),
                Err(ProofError::InvalidReceipt),
                "{id}: trailing bytes"
            );
            assert_eq!(
                verify_receipted(&raw, &receipt, &view, previous, &[]),
                Err(ProofError::UnpinnedReceiptSignature),
                "{id}: empty pins"
            );
            let mut tampered = receipt.clone();
            let last_byte = tampered.len() - 1;
            tampered[last_byte] ^= 1;
            assert_eq!(
                verify_receipted(
                    &raw,
                    &tampered,
                    &view,
                    previous,
                    &[(key_id, key, first, last)]
                ),
                Err(ProofError::UnpinnedReceiptSignature),
                "{id}: tampered signature"
            );
            for length in 0..receipt.len() {
                assert!(
                    verify_receipted(
                        &raw,
                        &receipt[..length],
                        &view,
                        previous,
                        &[(key_id, key, first, last)]
                    )
                    .is_err(),
                    "{id}: truncated receipt at {length}"
                );
            }
        } else {
            refused += 1;
            let expected_error = match string(&case["expected"], "refusedAt") {
                "R2" => ProofError::InvalidReceipt,
                "R3" => ProofError::UnpinnedReceiptSignature,
                "R4" => ProofError::ReceiptActivityMismatch,
                "R5" => ProofError::ReceiptViewMismatch,
                "R6" => ProofError::ReceiptSequenceMismatch,
                rule => panic!("{id}: unknown refusal {rule}"),
            };
            assert_eq!(result, Err(expected_error), "{id}: recorded refusal rule");
            if result.is_ok() {
                failures.push(format!(
                    "{id}: must refuse at {}, got {result:?}",
                    string(&case["expected"], "refusedAt")
                ));
            }
        }
    }
    assert_eq!(accepted, 4, "all recorded V1 cases must execute");
    assert_eq!(refused, 6, "all recorded V3 cases must execute");
    assert!(failures.is_empty(), "{}", failures.join("\n"));
}
