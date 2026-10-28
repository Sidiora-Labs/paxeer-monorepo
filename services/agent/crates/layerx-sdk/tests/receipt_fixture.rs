use std::fs;
use std::path::Path;

use ed25519_dalek::{Signature, Signer as _, SigningKey};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{canonical_protocol_facts, AuthorizedBatch};
use layerx_sdk::production::{verify_receipt, ReceiptKeys};
use layerx_sdk::ReceiptFailureCode;
use layerx_types::verify::VerificationLevel;
use layerx_wire::hash::receipt_digest;
use layerx_wire::receipt::{decode, encode_unsigned};

fn fixture_json() -> String {
    serde_json::to_string_pretty(&shared_fixture("receipt-positive-v2.json"))
        .unwrap_or_else(|error| panic!("serialize receipt fixture: {error}"))
}

fn shared_fixture(name: &str) -> serde_json::Value {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../sdk/platform/conformance/fixtures")
        .join(name);
    let source = fs::read_to_string(&path)
        .unwrap_or_else(|error| panic!("read {}: {error}", path.display()));
    let mut fixture: serde_json::Value = serde_json::from_str(&source)
        .unwrap_or_else(|error| panic!("decode {}: {error}", path.display()));
    if fixture.get("canonical_receipt_hex").is_some() {
        pin_fixture(&mut fixture);
    }
    fixture
}

const RECEIPT_HEIGHT: u64 = 42;

fn encoded_hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn pin_fixture(fixture: &mut serde_json::Value) {
    let mut canonical = value_hex(fixture, "canonical_receipt_hex");
    let receipt =
        decode(&canonical).unwrap_or_else(|error| panic!("fixture receipt decode: {error:?}"));
    let protocol = receipt
        .protocol()
        .unwrap_or_else(|| panic!("protocol body missing"));
    let signing_key = SigningKey::from_bytes(&[0x33; 32]);
    assert_eq!(
        signing_key.verifying_key().to_bytes(),
        value_hex_32(&fixture["authorized_batch"], "sequencer_public_key_hex")
    );
    let offsets: Vec<_> = canonical
        .windows(32)
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == protocol.batch_id()).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "fixture batch ID must be unique");
    let mut batch_id = [0; 32];
    batch_id[24..].copy_from_slice(&RECEIPT_HEIGHT.to_be_bytes());
    canonical[offsets[0]..offsets[0] + 32].copy_from_slice(&batch_id);
    let receipt =
        decode(&canonical).unwrap_or_else(|error| panic!("height receipt decode: {error:?}"));
    let unsigned = encode_unsigned(&receipt)
        .unwrap_or_else(|error| panic!("receipt signing preimage: {error:?}"));
    let digest =
        receipt_digest(&unsigned).unwrap_or_else(|error| panic!("receipt digest: {error:?}"));
    let signature_offset = canonical.len() - 64;
    assert_eq!(
        &canonical[signature_offset..],
        protocol
            .sequencer_signature()
            .unwrap_or_else(|| panic!("fixture signature missing"))
    );
    canonical[signature_offset..].copy_from_slice(&signing_key.sign(&digest).to_bytes());
    fixture["canonical_receipt_hex"] = encoded_hex(&canonical).into();
    fixture["authorized_batch"]["batch_id_hex"] = encoded_hex(&batch_id).into();
    fixture["expected"]["receipt_digest_hex"] = encoded_hex(&digest).into();
}

fn pin_vector(vector: &mut serde_json::Value) {
    let mut canonical = value_hex(vector, "canonical_receipt_hex");
    let offsets: Vec<_> = canonical
        .windows(32)
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == [0x81; 32]).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "vector batch ID must be unique");
    assert!(canonical.len() >= 69, "vector signature tail missing");
    let signature_offset = canonical.len() - 64;
    let signed = canonical[signature_offset - 5..signature_offset] == [1, 0, 0, 0, 64];
    let mut unsigned = if signed {
        let mut unsigned = canonical[..signature_offset - 5].to_vec();
        unsigned.push(0);
        unsigned
    } else {
        assert_eq!(canonical.last(), Some(&0), "unsigned vector flag missing");
        canonical.clone()
    };
    let signing_key = SigningKey::from_bytes(&[0x33; 32]);
    let signature_valid = signed.then(|| {
        let signature = Signature::from_slice(&canonical[signature_offset..])
            .unwrap_or_else(|error| panic!("vector signature: {error:?}"));
        let digest = receipt_digest(&unsigned)
            .unwrap_or_else(|error| panic!("original vector digest: {error:?}"));
        signing_key
            .verifying_key()
            .verify_strict(&digest, &signature)
            .is_ok()
    });
    let mut batch_id = [0; 32];
    batch_id[24..].copy_from_slice(&RECEIPT_HEIGHT.to_be_bytes());
    canonical[offsets[0]..offsets[0] + 32].copy_from_slice(&batch_id);
    unsigned[offsets[0]..offsets[0] + 32].copy_from_slice(&batch_id);
    if let Some(valid) = signature_valid {
        let digest = receipt_digest(&unsigned)
            .unwrap_or_else(|error| panic!("height vector digest: {error:?}"));
        let mut signature = signing_key.sign(&digest).to_bytes();
        if !valid {
            signature[63] ^= 1;
        }
        canonical[signature_offset..].copy_from_slice(&signature);
    }
    vector["canonical_receipt_hex"] = encoded_hex(&canonical).into();
}

fn fixture_keys(canonical: &[u8], batch: &AuthorizedBatch) -> [SequencerAuthorization; 1] {
    let receipt =
        decode(canonical).unwrap_or_else(|error| panic!("fixture receipt decode: {error:?}"));
    let protocol = receipt
        .protocol()
        .unwrap_or_else(|| panic!("protocol body missing"));
    [SequencerAuthorization::new(
        protocol.context_hash(),
        batch.sequencer_public_key(),
        RECEIPT_HEIGHT,
        RECEIPT_HEIGHT,
    )]
}

fn value_hex(value: &serde_json::Value, field: &str) -> Vec<u8> {
    hex_bytes(
        value[field]
            .as_str()
            .unwrap_or_else(|| panic!("fixture field {field} missing")),
    )
}

fn value_hex_32(value: &serde_json::Value, field: &str) -> [u8; 32] {
    value_hex(value, field)
        .try_into()
        .unwrap_or_else(|bytes: Vec<u8>| panic!("fixture field {field} is {} bytes", bytes.len()))
}

fn value_authorised(fixture: &serde_json::Value) -> AuthorizedBatch {
    let batch = &fixture["authorized_batch"];
    AuthorizedBatch::new(
        value_hex(batch, "batch_id_hex")
            .try_into()
            .unwrap_or_else(|bytes: Vec<u8>| panic!("batch id length: got {} bytes", bytes.len())),
        value_hex(batch, "asset_hex")
            .try_into()
            .unwrap_or_else(|bytes: Vec<u8>| panic!("asset length: got {} bytes", bytes.len())),
        value_hex(batch, "previous_state_root_hex")
            .try_into()
            .unwrap_or_else(|bytes: Vec<u8>| {
                panic!("previous root length: got {} bytes", bytes.len())
            }),
        value_hex(batch, "resulting_state_root_hex")
            .try_into()
            .unwrap_or_else(|bytes: Vec<u8>| {
                panic!("resulting root length: got {} bytes", bytes.len())
            }),
        value_hex(batch, "sequencer_public_key_hex")
            .try_into()
            .unwrap_or_else(|bytes: Vec<u8>| {
                panic!("sequencer key length: got {} bytes", bytes.len())
            }),
    )
}

fn string_field(json: &str, key: &str) -> String {
    let marker = format!("\"{key}\": \"");
    let start = json
        .find(&marker)
        .unwrap_or_else(|| panic!("fixture field {key} missing"))
        + marker.len();
    let rest = &json[start..];
    let end = rest
        .find('"')
        .unwrap_or_else(|| panic!("fixture field {key} unterminated"));
    rest[..end].to_string()
}

fn number_field(json: &str, key: &str) -> u64 {
    let marker = format!("\"{key}\": ");
    let start = json
        .find(&marker)
        .unwrap_or_else(|| panic!("fixture field {key} missing"))
        + marker.len();
    let rest = &json[start..];
    let end = rest
        .find([',', '\n'])
        .unwrap_or_else(|| panic!("fixture field {key} unterminated"));
    rest[..end]
        .trim()
        .parse::<u64>()
        .unwrap_or_else(|error| panic!("fixture field {key}: {error}"))
}

fn u128_field(json: &str, key: &str) -> u128 {
    string_field(json, key)
        .parse::<u128>()
        .unwrap_or_else(|error| panic!("fixture field {key}: {error}"))
}

fn hex_bytes(value: &str) -> Vec<u8> {
    assert!(value.len().is_multiple_of(2), "odd hex length");
    (0..value.len() / 2)
        .map(|index| {
            u8::from_str_radix(&value[2 * index..2 * index + 2], 16)
                .unwrap_or_else(|error| panic!("fixture hex: {error}"))
        })
        .collect()
}

fn hex_32(json: &str, key: &str) -> [u8; 32] {
    hex_bytes(&string_field(json, key))
        .as_slice()
        .try_into()
        .unwrap_or_else(|error| panic!("fixture field {key} is not 32 bytes: {error}"))
}

fn authorised_batch(json: &str) -> AuthorizedBatch {
    AuthorizedBatch::new(
        hex_32(json, "batch_id_hex"),
        hex_32(json, "asset_hex"),
        hex_32(json, "previous_state_root_hex"),
        hex_32(json, "resulting_state_root_hex"),
        hex_32(json, "sequencer_public_key_hex"),
    )
}

#[test]
fn core_fixture_receipt_verifies_positively() {
    let json = fixture_json();
    let canonical = hex_bytes(&string_field(&json, "canonical_receipt_hex"));
    let batch = authorised_batch(&json);
    let keys = fixture_keys(&canonical, &batch);
    let verified = verify_receipt(
        &canonical,
        ReceiptKeys::HeightRanges(&keys),
        batch.asset(),
        batch.previous_state_root(),
        batch.resulting_state_root(),
    )
    .unwrap_or_else(|failure| panic!("canonical core receipt refused: {failure:?}"));
    assert_eq!(verified.level(), VerificationLevel::SEQUENCER_SIGNED);
    assert_eq!(verified.canonical_bytes(), canonical.as_slice());
    assert_eq!(
        verified.evidence().receipt_digest(),
        Some(hex_32(&json, "receipt_digest_hex"))
    );
    let protocol = verified
        .receipt()
        .protocol()
        .unwrap_or_else(|| panic!("verified receipt lost its protocol body"));
    assert_eq!(i64::from(protocol.result_code()), 0);
    assert_eq!(
        u64::from(protocol.protocol_version()),
        number_field(&json, "protocol_version")
    );
    assert_eq!(
        u64::from(protocol.operation()),
        number_field(&json, "operation")
    );
    assert_eq!(
        u64::from(protocol.module_id()),
        number_field(&json, "module_id")
    );
    assert_eq!(
        protocol.global_sequence(),
        number_field(&json, "global_sequence")
    );
    assert_eq!(protocol.timestamp(), number_field(&json, "timestamp_ms"));
    assert_eq!(protocol.amount(), u128_field(&json, "amount"));
    assert_eq!(protocol.fee_charged(), u128_field(&json, "fee_charged"));
    assert_eq!(
        protocol.debit_balance_before(),
        u128_field(&json, "from_balance_before")
    );
    assert_eq!(
        protocol.debit_balance_after(),
        u128_field(&json, "from_balance_after")
    );
    assert_eq!(
        protocol.credit_balance_before(),
        u128_field(&json, "to_balance_before")
    );
    assert_eq!(
        protocol.credit_balance_after(),
        u128_field(&json, "to_balance_after")
    );
    assert_eq!(protocol.activity_id(), hex_32(&json, "activity_id_hex"));
    assert_eq!(protocol.from(), hex_32(&json, "from_hex"));
    assert_eq!(protocol.to(), hex_32(&json, "to_hex"));
    assert_eq!(protocol.batch_id(), hex_32(&json, "batch_id_hex"));
    assert_eq!(protocol.asset(), hex_32(&json, "asset_hex"));
    assert_eq!(
        protocol.previous_state_root(),
        hex_32(&json, "previous_state_root_hex")
    );
    assert_eq!(
        protocol.resulting_state_root(),
        hex_32(&json, "resulting_state_root_hex")
    );
    let facts = canonical_protocol_facts(&canonical)
        .unwrap_or_else(|failure| panic!("canonical facts refused: {failure:?}"));
    assert_eq!(facts.result_code(), 0);
    assert_eq!(facts.amount(), u128_field(&json, "amount"));
    assert_eq!(facts.fee_charged(), u128_field(&json, "fee_charged"));
    assert_eq!(facts.asset(), hex_32(&json, "asset_hex"));
}

#[test]
fn core_fixture_receipt_byte_flip_fails() {
    let json = fixture_json();
    let mut mutated = hex_bytes(&string_field(&json, "canonical_receipt_hex"));
    let batch = authorised_batch(&json);
    let keys = fixture_keys(&mutated, &batch);
    let last = mutated.len() - 1;
    mutated[last] ^= 0x01;
    let Err(failure) = verify_receipt(
        &mutated,
        ReceiptKeys::HeightRanges(&keys),
        batch.asset(),
        batch.previous_state_root(),
        batch.resulting_state_root(),
    ) else {
        panic!("mutated receipt verified; a flipped signature byte must fail")
    };
    assert_eq!(failure.check, ReceiptFailureCode::SequencerSignature);
}

#[test]
fn core_programs_fixture_preserves_the_optional_outcome() {
    let fixture = shared_fixture("receipt-programs-positive-v2.json");
    let canonical = value_hex(&fixture, "canonical_receipt_hex");
    let batch = value_authorised(&fixture);
    let keys = fixture_keys(&canonical, &batch);
    let verified = verify_receipt(
        &canonical,
        ReceiptKeys::HeightRanges(&keys),
        batch.asset(),
        batch.previous_state_root(),
        batch.resulting_state_root(),
    )
    .unwrap_or_else(|failure| panic!("Programs receipt refused: {failure:?}"));
    let receipt = verified
        .receipt()
        .protocol()
        .unwrap_or_else(|| panic!("protocol receipt missing"));
    let outcome = receipt
        .program_outcome()
        .unwrap_or_else(|| panic!("Programs outcome missing"));
    assert_eq!(outcome.encoding_version(), 3);
    assert_eq!(outcome.runtime_version(), 1);
    assert_eq!(outcome.abi_version(), 1);
    assert_eq!(outcome.occupancy_byte_batches(), 2);
    assert_eq!(outcome.occupancy_fee_units(), 7);
    assert_eq!(
        outcome.occupancy_asset_id(),
        value_hex_32(
            &fixture["expected"],
            "program_outcome_occupancy_asset_id_hex"
        )
    );
    assert_eq!(
        outcome.occupancy_evidence_digest(),
        value_hex_32(
            &fixture["expected"],
            "program_outcome_occupancy_evidence_digest_hex"
        )
    );
    assert_eq!(
        outcome.occupancy_transfer_root(),
        value_hex_32(
            &fixture["expected"],
            "program_outcome_occupancy_transfer_root_hex"
        )
    );
    assert_eq!(outcome.fee_units(), 16);
    assert_eq!(outcome.call_graph_root(), [0x11; 32]);
    assert_eq!(outcome.terminal_payload_root(), [0x22; 32]);
}

fn assert_refusal_taxonomy(name: &str) {
    let mut fixture = shared_fixture(name);
    let authorised = value_authorised(&fixture);
    let positive = shared_fixture("receipt-programs-positive-v2.json");
    let keys = fixture_keys(&value_hex(&positive, "canonical_receipt_hex"), &authorised);
    for vector in fixture["vectors"]
        .as_array_mut()
        .unwrap_or_else(|| panic!("refusal vectors missing"))
    {
        pin_vector(vector);
        let canonical = value_hex(vector, "canonical_receipt_hex");
        let Err(failure) = verify_receipt(
            &canonical,
            ReceiptKeys::HeightRanges(&keys),
            authorised.asset(),
            authorised.previous_state_root(),
            authorised.resulting_state_root(),
        ) else {
            panic!("non-canonical receipt verified: {}", vector["name"]);
        };
        assert_eq!(
            failure.check.as_str(),
            vector["expected_check"]
                .as_str()
                .unwrap_or_else(|| panic!("expected check missing")),
            "{}",
            vector["name"]
        );
    }
}

#[test]
fn core_refusal_vectors_expose_the_shared_taxonomy() {
    assert_refusal_taxonomy("receipt-refusals-v2.json");
}

#[test]
fn programs_refusal_vectors_expose_the_shared_taxonomy() {
    assert_refusal_taxonomy("receipt-programs-refusals-v2.json");
}
