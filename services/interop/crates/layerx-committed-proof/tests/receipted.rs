use layerx_committed_proof::{
    header::{BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator},
    verify_committed, verify_header, verify_receipted, ProofError,
};
use sha2::{Digest, Sha256};

const FIXTURE: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");
const COMMITTED_FIXTURE: &str =
    include_str!("../../../../../daemon/committed-fixture/vectors/vector-v4-committed.json");

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
    decode_hex(hex, name)
}

fn decode_hex(hex: &str, name: &str) -> Vec<u8> {
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
            let mut wrong_signing_key = case["receiptJson"].clone();
            wrong_signing_key["signingKey"] = format!("0x{}", "00".repeat(32)).into();
            let wrong_signing_key = serde_json::to_vec(&wrong_signing_key)
                .unwrap_or_else(|error| panic!("{id}: invalid altered JSON view: {error}"));
            assert_eq!(
                verify_receipted(
                    &raw,
                    &receipt,
                    &wrong_signing_key,
                    previous,
                    &[(key_id, key, first, last)]
                ),
                Err(ProofError::ReceiptViewMismatch),
                "{id}: JSON signing key must match the verified pin"
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

#[test]
fn receipted_operation_zero_is_carried_into_signature_check() {
    let fixture: serde_json::Value =
        serde_json::from_str(FIXTURE).unwrap_or_else(|error| panic!("invalid fixture: {error}"));
    let mut checked = 0;
    for case in fixture["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("missing fixture cases"))
        .iter()
        .filter(|case| string(case, "vector") == "V1")
    {
        let id = string(case, "id");
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
        assert_eq!(
            verify_receipted(
                &raw,
                &receipt,
                &view,
                previous,
                &[(key_id, key, first, last)]
            ),
            Ok(hash(case, "receiptDigest")),
            "{id}: original V1 receipt verifies"
        );
        let operation_offset = receipt
            .windows(10)
            .position(|bytes| bytes == [0, 12, 0, 0, 0, 1, 0, 0, 0, 0])
            .unwrap_or_else(|| panic!("{id}: missing module-12 trailer and parameter version"))
            + 10;
        assert_eq!(receipt[operation_offset], 2, "{id}: fixture operation");
        let mut zero_operation = receipt.clone();
        zero_operation[operation_offset] = 0;
        assert_eq!(
            verify_receipted(
                &raw,
                &zero_operation,
                &view,
                previous,
                &[(key_id, key, first, last)]
            ),
            Err(ProofError::UnpinnedReceiptSignature),
            "{id}: operation zero is carried into the signature check"
        );
        checked += 1;
    }
    assert_eq!(checked, 4, "all recorded V1 operations must be checked");
}

fn committed_header(fixture: &serde_json::Value) -> Header {
    let header = &fixture["settlement"]["execution_header"];
    let field = |value: &serde_json::Value, name| decode_hex(string(value, name), name);
    let decimal = |value: &serde_json::Value, name| {
        string(value, name)
            .parse()
            .unwrap_or_else(|error| panic!("invalid header number {name}: {error}"))
    };
    assert_eq!(string(header, "time"), "2026-10-28T20:44:36.174Z");
    Header {
        version: ConsensusVersion {
            block: decimal(&header["version"], "block"),
            app: decimal(&header["version"], "app"),
        },
        chain_id: string(header, "chain_id").to_owned(),
        height: string(header, "height")
            .parse()
            .unwrap_or_else(|error| panic!("invalid header height: {error}")),
        time: Timestamp {
            seconds: 1_793_220_276,
            nanos: 174_000_000,
        },
        last_block_id: BlockId {
            hash: field(&header["last_block_id"], "hash"),
            part_set_header: PartSetHeader {
                total: u32::try_from(number(&header["last_block_id"]["parts"], "total"))
                    .unwrap_or_else(|error| panic!("invalid header part count: {error}")),
                hash: field(&header["last_block_id"]["parts"], "hash"),
            },
        },
        last_commit_hash: field(header, "last_commit_hash"),
        data_hash: field(header, "data_hash"),
        validators_hash: field(header, "validators_hash")
            .try_into()
            .unwrap_or_else(|_| panic!("invalid header validators hash length")),
        next_validators_hash: field(header, "next_validators_hash"),
        consensus_hash: field(header, "consensus_hash"),
        app_hash: field(header, "app_hash"),
        last_results_hash: field(header, "last_results_hash"),
        evidence_hash: field(header, "evidence_hash"),
        proposer_address: field(header, "proposer_address"),
    }
}

#[test]
fn committed_fixture_view_verifies_and_binds_block_fields_and_rung() {
    let fixture: serde_json::Value = serde_json::from_str(COMMITTED_FIXTURE)
        .unwrap_or_else(|error| panic!("invalid committed fixture: {error}"));
    let view = &fixture["settlement"]["receipt"];
    let raw = bytes(&fixture["settlement"], "raw_transaction");
    let receipt = bytes(&fixture["positive"], "receipt");
    assert_eq!(receipt, bytes(view, "receipt"));
    let previous = receipt[54..86]
        .try_into()
        .unwrap_or_else(|_| panic!("invalid receipt predecessor length"));
    let height = u64::from_str_radix(
        string(view, "height")
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("missing committed height prefix")),
        16,
    )
    .unwrap_or_else(|error| panic!("invalid committed height: {error}"));
    let keys = [(
        hash(view, "keyId"),
        hash(view, "signingKey"),
        height,
        height,
    )];
    let validators: Vec<_> = fixture["settlement"]["validators"]
        .as_array()
        .unwrap_or_else(|| panic!("missing committed validators"))
        .iter()
        .map(|validator| {
            assert_eq!(
                string(&validator["pub_key"], "type"),
                "tendermint/PubKeyEd25519"
            );
            assert_eq!(
                string(&validator["pub_key"], "value"),
                "xhShyRiEw8E77VhqFtUfclMHL3zEEBjqKN2GTk5Ujk0="
            );
            Validator {
                pub_key: decode_hex(
                    "c614a1c91884c3c13bed586a16d51f7253072f7cc41018ea28dd864e4e548e4d",
                    "validator public key",
                )
                .try_into()
                .unwrap_or_else(|_| panic!("invalid validator public key length")),
                voting_power: string(validator, "voting_power")
                    .parse()
                    .unwrap_or_else(|error| panic!("invalid validator power: {error}")),
            }
        })
        .collect();
    let header = committed_header(&fixture);
    assert_eq!(u64::try_from(header.height), Ok(height));
    let block_hash = verify_header(&header, &validators)
        .unwrap_or_else(|error| panic!("committed header must verify: {error}"));
    assert_eq!(block_hash, hash(view, "blockHash"));
    assert_eq!(string(view, "blockNumber"), format!("0x{height:x}"));
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    assert_eq!(
        &receipt[receipt.len() - 69..receipt.len() - 64],
        &[1, 0, 0, 0, 64]
    );
    digest.update(&receipt[..receipt.len() - 69]);
    digest.update([0]);
    let digest: [u8; 32] = digest.finalize().into();
    let recorded = serde_json::to_vec(view)
        .unwrap_or_else(|error| panic!("invalid committed JSON view: {error}"));
    assert_eq!(
        verify_committed(&raw, &receipt, &recorded, previous, &keys, block_hash),
        Ok(digest),
        "recorded committed view with inclusive receipt-height pin"
    );

    let mut wrong_hash = view.clone();
    let mut flipped = hash(view, "blockHash");
    flipped[0] ^= 1;
    wrong_hash["blockHash"] = format!(
        "0x{}",
        flipped
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    )
    .into();
    let mut wrong_number = view.clone();
    wrong_number["blockNumber"] = format!("0x{:x}", height + 1).into();
    for (name, altered) in [("blockHash", wrong_hash), ("blockNumber", wrong_number)] {
        let altered = serde_json::to_vec(&altered)
            .unwrap_or_else(|error| panic!("invalid altered committed view: {error}"));
        assert_eq!(
            verify_committed(&raw, &receipt, &altered, previous, &keys, block_hash),
            Err(ProofError::ReceiptViewMismatch),
            "committed view must bind {name}"
        );
    }
    assert_eq!(
        verify_receipted(&raw, &receipt, &recorded, previous, &keys),
        Err(ProofError::ReceiptViewMismatch),
        "receipted verifier must refuse the committed view"
    );
}
