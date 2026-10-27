use ed25519_dalek::{Signer as _, SigningKey};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_sdk::production::{verify_receipt, ReceiptKeys, ReceiptVerificationFailure};
use layerx_sdk::ReceiptFailureCode;
use layerx_types::{rung::Rung, verify::VerificationLevel};
use layerx_wire::hash::receipt_digest;
use layerx_wire::receipt::{decode, encode_unsigned};

const HEIGHT: u64 = 42;

struct ReceiptFixture {
    canonical: Vec<u8>,
    key_id: [u8; 32],
    public_key: [u8; 32],
    asset: [u8; 32],
    previous_state_root: [u8; 32],
    resulting_state_root: [u8; 32],
}

fn hex_bytes(value: &str) -> Vec<u8> {
    assert!(value.len().is_multiple_of(2), "odd fixture hex length");
    (0..value.len() / 2)
        .map(|index| {
            u8::from_str_radix(&value[index * 2..index * 2 + 2], 16)
                .unwrap_or_else(|error| panic!("fixture hex: {error}"))
        })
        .collect()
}

fn receipt_fixture() -> ReceiptFixture {
    let fixture: serde_json::Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/receipt-positive-v2.json"
    ))
    .unwrap_or_else(|error| panic!("receipt fixture JSON: {error}"));
    let mut canonical = hex_bytes(
        fixture["canonical_receipt_hex"]
            .as_str()
            .unwrap_or_else(|| panic!("fixture canonical receipt missing")),
    );
    let receipt =
        decode(&canonical).unwrap_or_else(|error| panic!("fixture receipt decode: {error:?}"));
    let protocol = receipt
        .protocol()
        .unwrap_or_else(|| panic!("fixture protocol body missing"));
    let signing_key = SigningKey::from_bytes(&[0x33; 32]);
    let public_key = signing_key.verifying_key().to_bytes();
    assert_eq!(
        public_key.as_slice(),
        hex_bytes(
            fixture["authorized_batch"]["sequencer_public_key_hex"]
                .as_str()
                .unwrap_or_else(|| panic!("fixture sequencer public key missing"))
        )
    );
    let batch_offsets: Vec<_> = canonical
        .windows(32)
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == protocol.batch_id()).then_some(offset))
        .collect();
    assert_eq!(batch_offsets.len(), 1, "fixture batch ID must be unique");
    let offset = batch_offsets[0];
    let mut batch_id = [0; 32];
    batch_id[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    canonical[offset..offset + 32].copy_from_slice(&batch_id);
    let with_height =
        decode(&canonical).unwrap_or_else(|error| panic!("height receipt decode: {error:?}"));
    let unsigned = encode_unsigned(&with_height)
        .unwrap_or_else(|error| panic!("receipt signing preimage: {error:?}"));
    let digest =
        receipt_digest(&unsigned).unwrap_or_else(|error| panic!("receipt digest: {error:?}"));
    let signature_offset = canonical.len() - 64;
    assert_eq!(
        &canonical[signature_offset..],
        protocol
            .sequencer_signature()
            .unwrap_or_else(|| panic!("fixture sequencer signature missing"))
    );
    canonical[signature_offset..].copy_from_slice(&signing_key.sign(&digest).to_bytes());
    ReceiptFixture {
        canonical,
        key_id: protocol.context_hash(),
        public_key,
        asset: protocol.asset(),
        previous_state_root: protocol.previous_state_root(),
        resulting_state_root: protocol.resulting_state_root(),
    }
}

#[test]
fn verify_receipt_accepts_a_receipt_under_a_pinned_key() {
    let fixture = receipt_fixture();
    for (first, last) in [(HEIGHT, HEIGHT), (HEIGHT - 1, HEIGHT), (HEIGHT, HEIGHT + 1)] {
        let keys = [SequencerAuthorization::new(
            fixture.key_id,
            fixture.public_key,
            first,
            last,
        )];
        let verified = verify_receipt(
            &fixture.canonical,
            ReceiptKeys::HeightRanges(&keys),
            fixture.asset,
            fixture.previous_state_root,
            fixture.resulting_state_root,
        )
        .unwrap_or_else(|failure| panic!("pinned receipt refused: {failure:?}"));
        assert_eq!(verified.canonical_bytes(), fixture.canonical);
        assert_eq!(verified.level(), VerificationLevel::SEQUENCER_SIGNED);
        assert_eq!(verified.rung(), Rung::Receipted);
    }
}

#[test]
fn verify_receipt_refuses_a_key_outside_its_height_range() {
    let fixture = receipt_fixture();
    for (first, last) in [(HEIGHT - 2, HEIGHT - 1), (HEIGHT + 1, HEIGHT + 2)] {
        let keys = [SequencerAuthorization::new(
            fixture.key_id,
            fixture.public_key,
            first,
            last,
        )];
        assert_eq!(
            verify_receipt(
                &fixture.canonical,
                ReceiptKeys::HeightRanges(&keys),
                fixture.asset,
                fixture.previous_state_root,
                fixture.resulting_state_root,
            ),
            Err(ReceiptVerificationFailure {
                check: ReceiptFailureCode::SequencerSignature,
            })
        );
    }
}
