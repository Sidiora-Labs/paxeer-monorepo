//! Audit 22.30.112: exercise the documented receipt and external evidence paths.
//! Finding-to-test mapping and baseline limitations are in `repair_22_33_117.kvx`.

#[path = "support/ap2_external.rs"]
mod ap2_external;

use layerx_ap2::{Ap2Error, Ap2ExternalMandateVerifier, AP2_MANDATE_PAIR_MEDIA_TYPE};
use layerx_interop_gateway::adapter::{AdapterDescriptor, AdapterId, ConformanceSuite};
use layerx_portable::{
    verify_external_evidence, verify_portable_json, ExternalEvidenceKind, ExternalEvidenceVerifier,
    ExternalPresentation, ExternalVerificationError, PortableReceiptError, PortableReceiptV2,
    PortableVerified, ReceiptKeyEntry,
};
use layerx_proof::receipt::{ReceiptCheck, VerificationFailure};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const FORMAT: &str = include_str!("../FORMAT.md");
const RECEIPTS: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");

fn bytes(value: &Value) -> Vec<u8> {
    let hex = value
        .as_str()
        .unwrap_or_else(|| panic!("fixture hex"))
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("prefix"));
    hex.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(
                std::str::from_utf8(pair).unwrap_or_else(|error| panic!("ASCII: {error:?}")),
                16,
            )
            .unwrap_or_else(|error| panic!("hex byte: {error:?}"))
        })
        .collect()
}

fn recorded_receipt() -> (Value, ReceiptKeyEntry, Vec<u8>) {
    let fixture: Value = serde_json::from_str(RECEIPTS)
        .unwrap_or_else(|error| panic!("recorded producer transcript: {error:?}"));
    assert_eq!(
        fixture["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    let case = fixture["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("cases"))
        .iter()
        .find(|case| case["id"] == "V1-pax-transfer")
        .unwrap_or_else(|| panic!("recorded PAX transfer"));
    let pin = &case["pinnedKey"];
    let key = ReceiptKeyEntry::new(
        bytes(&pin["keyId"])
            .try_into()
            .unwrap_or_else(|error| panic!("key id: {error:?}")),
        bytes(&pin["publicKey"])
            .try_into()
            .unwrap_or_else(|error| panic!("public key: {error:?}")),
        pin["firstHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("first height")),
        pin["lastHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("last height")),
    );
    (
        case["receiptJson"].clone(),
        key,
        bytes(&case["rawTransaction"]),
    )
}

fn wire_value(answer: &Value, pin: ReceiptKeyEntry, raw: &[u8]) -> Value {
    let portable = PortableReceiptV2::from_sequencer_receipt(answer, &[pin], Some(raw))
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "production exporter accepts producer transcript"
            )
        });
    serde_json::from_slice(
        &portable
            .to_json()
            .unwrap_or_else(|error| panic!("production JSON codec: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("wire object: {error:?}"))
}

#[test]
fn portable_format_versions_real_round_trip_and_windows() {
    assert!(FORMAT.starts_with("# LayerX portable verification formats\n\nThe current receipt format is `layerx-receipt-proof-v2`."));
    assert!(FORMAT.contains("window must cover the **signed height**"));
    let (answer, pin, raw) = recorded_receipt();
    let height = u64::from_str_radix(
        answer["height"]
            .as_str()
            .unwrap_or_else(|| panic!("expected Some value"))
            .trim_start_matches("0x"),
        16,
    )
    .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    // Check both inclusive endpoints with real signed bytes; no re-signing.
    for (first, last) in [
        (height, height + 10),
        (height - 10, height),
        (height, height),
    ] {
        let endpoint_pin = ReceiptKeyEntry::new(pin.sequencer_id(), pin.public_key(), first, last);
        let mut committed = answer.clone();
        committed["rung"] = "committed".into();
        committed["blockNumber"] = format!("{:#x}", last + 1).into();
        let wire = wire_value(&committed, endpoint_pin, &raw);
        assert_eq!(wire["format"], "layerx-receipt-proof-v2");
        assert_eq!(wire["verificationLevel"], "receipted");
        assert_eq!(wire["signedReceipt"], answer["receipt"]);
        assert_eq!(wire["receiptKey"]["firstHeight"], format!("{first:#x}"));
        assert_eq!(wire["receiptKey"]["lastHeight"], format!("{last:#x}"));
        let PortableVerified::V2(verified) = verify_portable_json(
            &serde_json::to_vec(&wire)
                .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
            &[endpoint_pin],
        )
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "real offline consumer verifies exported JSON"
            )
        });
        assert_eq!(verified.height(), height);
        assert_eq!(verified.inclusion_height(), last + 1);
        assert_eq!(verified.raw_transaction(), Some(raw.as_slice()));
        assert_eq!(verified.activity_id().as_slice(), bytes(&answer["txHash"]));
        assert_eq!(verified.result_code(), 0);
    }
}

fn assert_field_refusals(wire: &serde_json::Value, pin: ReceiptKeyEntry) {
    for (pointer, replacement, expected) in [
        (
            "/format",
            json!("layerx-receipt-proof-v1"),
            PortableReceiptError::UnsupportedFormat,
        ),
        (
            "/verificationLevel",
            json!("committed"),
            PortableReceiptError::UnsupportedVerificationLevel,
        ),
        (
            "/signedReceipt",
            json!("c2lnbmVk"),
            PortableReceiptError::InvalidHex("signedReceipt"),
        ),
        (
            "/signedReceipt",
            json!("0xAB"),
            PortableReceiptError::InvalidHex("signedReceipt"),
        ),
        (
            "/signedReceipt",
            json!("0x0"),
            PortableReceiptError::InvalidHex("signedReceipt"),
        ),
        (
            "/receiptDigest",
            json!("0x00"),
            PortableReceiptError::InvalidLength("receiptDigest"),
        ),
        (
            "/receiptKey/firstHeight",
            json!("0x00"),
            PortableReceiptError::InvalidQuantity("firstHeight"),
        ),
        (
            "/receiptKey/publicKey",
            json!(format!("0x{}", "01".repeat(32))),
            PortableReceiptError::ReceiptKeyMismatch,
        ),
        (
            "/receiptDigest",
            json!(format!("0x{}", "00".repeat(32))),
            PortableReceiptError::ReceiptDigestMismatch,
        ),
        (
            "/rawTransaction",
            json!("0x00"),
            PortableReceiptError::RawTransactionMismatch,
        ),
        (
            "/inclusionHeight",
            json!("0x0"),
            PortableReceiptError::InclusionHeight,
        ),
    ] {
        let mut changed = wire.clone();
        *changed
            .pointer_mut(pointer)
            .unwrap_or_else(|| panic!("existing field")) = replacement;
        assert_eq!(
            verify_portable_json(
                &serde_json::to_vec(&changed)
                    .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
                &[pin]
            ),
            Err(expected),
            "{pointer}"
        );
        let refusal = format!("{expected:?}");
        assert!(
            FORMAT.contains(
                refusal
                    .split('(')
                    .next()
                    .unwrap_or_else(|| panic!("expected Some value"))
            ),
            "documented refusal {expected:?}"
        );
    }
}

fn assert_shape_and_signature_refusals(wire: serde_json::Value, pin: ReceiptKeyEntry) {
    let encoded =
        serde_json::to_string(&wire).unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    let duplicate = format!(
        "{{\"signedReceipt\":{},{}",
        serde_json::to_string(&wire["signedReceipt"])
            .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
        &encoded[1..]
    );
    assert_eq!(
        verify_portable_json(duplicate.as_bytes(), &[pin]),
        Err(PortableReceiptError::JsonShape)
    );
    let mut unknown = wire.clone();
    unknown["receiptKey"]["untrusted"] = true.into();
    assert_eq!(
        verify_portable_json(
            &serde_json::to_vec(&unknown)
                .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
            &[pin]
        ),
        Err(PortableReceiptError::JsonShape)
    );
    let mut missing = wire.clone();
    missing
        .as_object_mut()
        .unwrap_or_else(|| panic!("expected Some value"))
        .remove("receiptDigest");
    assert_eq!(
        verify_portable_json(
            &serde_json::to_vec(&missing)
                .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
            &[pin]
        ),
        Err(PortableReceiptError::JsonShape)
    );
    let signature_failure = Err(PortableReceiptError::Receipt(VerificationFailure {
        check: ReceiptCheck::SequencerSignature,
    }));
    let encoded =
        serde_json::to_vec(&wire).unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    for pins in [
        vec![],
        vec![pin, pin],
        vec![ReceiptKeyEntry::new(
            pin.sequencer_id(),
            pin.public_key(),
            1001,
            1999,
        )],
    ] {
        assert_eq!(verify_portable_json(&encoded, &pins), signature_failure);
    }
    let mut tampered = wire;
    let mut signed = bytes(&tampered["signedReceipt"]);
    *signed
        .last_mut()
        .unwrap_or_else(|| panic!("expected Some value")) ^= 1;
    let mut signed_hex = String::from("0x");
    for byte in &signed {
        use std::fmt::Write as _;
        write!(&mut signed_hex, "{byte:02x}")
            .unwrap_or_else(|error| panic!("hex rendering: {error:?}"));
    }
    tampered["signedReceipt"] = signed_hex.into();
    assert_eq!(
        verify_portable_json(
            &serde_json::to_vec(&tampered)
                .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
            &[pin]
        ),
        signature_failure
    );
    assert_eq!(
        PortableReceiptV2::from_sequencer_receipt(&json!({"rung":"submitted"}), &[pin], None),
        Err(PortableReceiptError::NotReceipted)
    );
}

#[test]
fn portable_format_versions_current_importer_refusals() {
    assert!(FORMAT.contains("### Current importer refusals"));
    assert!(FORMAT.contains("## Retired v1 migration appendix"));
    let (answer, pin, raw) = recorded_receipt();
    let wire = wire_value(&answer, pin, &raw);
    assert_field_refusals(&wire, pin);
    assert_shape_and_signature_refusals(wire, pin);
}

fn presentation(payload: &[u8]) -> ExternalPresentation<'_> {
    ExternalPresentation::new(
        "ap2",
        "ap2",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        AP2_MANDATE_PAIR_MEDIA_TYPE,
        payload,
    )
    .unwrap_or_else(|error| panic!("real AP2 presentation envelope: {error:?}"))
}

// Independent encoding of the FORMAT.md digest formula, including every descriptor field.
fn documented_digest(payload: &[u8], descriptor: &AdapterDescriptor) -> [u8; 32] {
    fn part(hash: &mut Sha256, bytes: &[u8]) {
        hash.update(
            u64::try_from(bytes.len())
                .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"))
                .to_be_bytes(),
        );
        hash.update(bytes);
    }
    let mut hash = Sha256::new();
    hash.update(b"LayerX/interop/external-evidence/v1\0");
    hash.update([1]);
    part(&mut hash, descriptor.id().as_str().as_bytes());
    part(&mut hash, descriptor.spec().protocol().as_str().as_bytes());
    part(&mut hash, descriptor.spec().version().as_str().as_bytes());
    hash.update(descriptor.spec().document_digest());
    part(
        &mut hash,
        descriptor.conformance().suite().as_str().as_bytes(),
    );
    hash.update(descriptor.conformance().vector_count().to_be_bytes());
    hash.update(descriptor.conformance().suite_digest());
    part(&mut hash, AP2_MANDATE_PAIR_MEDIA_TYPE.as_bytes());
    part(&mut hash, payload);
    hash.finalize().into()
}

#[test]
fn portable_digest_claim_content_and_descriptor_binding() {
    assert!(FORMAT.contains("content and descriptor binding"));
    assert!(FORMAT.contains("Context and typed output are not hashed."));
    let keys = ap2_external::MandateKeys::pinned();
    let mandate_checker = Ap2ExternalMandateVerifier::new(&keys, ap2_external::suite())
        .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    let fixture = ap2_external::MandateFixture::direct(false);
    let payload = fixture.payload();
    let verified = verify_external_evidence(
        &mandate_checker,
        &presentation(&payload),
        &ap2_external::context(),
    )
    .unwrap_or_else(|error| panic!("production AP2 verification: {error:?}"));
    assert_eq!(verified.verified().transaction_id(), fixture.transaction_id);
    assert_eq!(
        verified.verified().checkout_id(),
        fixture.expected["checkout_id"]
            .as_str()
            .unwrap_or_else(|| panic!("expected Some value"))
    );
    assert_eq!(verified.verified().amount().minor_units(), 10_000);
    assert_eq!(
        verified.evidence_digest(),
        documented_digest(&payload, mandate_checker.descriptor())
    );

    let mut spaced = payload.clone();
    spaced.push(b'\n');
    let whitespace = verify_external_evidence(
        &mandate_checker,
        &presentation(&spaced),
        &ap2_external::context(),
    )
    .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    assert_eq!(
        whitespace.verified().transaction_id(),
        verified.verified().transaction_id()
    );
    assert_eq!(
        whitespace.verified().checkout_receipt_reference(),
        verified.verified().checkout_receipt_reference()
    );
    assert_eq!(
        whitespace.verified().payment_receipt_reference(),
        verified.verified().payment_receipt_reference()
    );
    assert_ne!(whitespace.evidence_digest(), verified.evidence_digest());
    let suite = ConformanceSuite::new(
        AdapterId::new("retained-audit-vector")
            .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
        1,
        Sha256::digest(&payload).into(),
    )
    .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    let other_verifier = Ap2ExternalMandateVerifier::new(&keys, suite)
        .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    let other = verify_external_evidence(
        &other_verifier,
        &presentation(&payload),
        &ap2_external::context(),
    )
    .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    assert_eq!(
        other.verified().transaction_id(),
        verified.verified().transaction_id()
    );
    assert_eq!(
        other.verified().checkout_receipt_reference(),
        verified.verified().checkout_receipt_reference()
    );
    assert_eq!(
        other.verified().payment_receipt_reference(),
        verified.verified().payment_receipt_reference()
    );
    assert_ne!(other.evidence_digest(), verified.evidence_digest());
    assert_eq!(
        other.evidence_digest(),
        documented_digest(&payload, other_verifier.descriptor())
    );

    let mut later = ap2_external::context();
    later.now += 1;
    let contextual = verify_external_evidence(&mandate_checker, &presentation(&payload), &later)
        .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    assert_ne!(
        contextual.verified().execution_at(),
        verified.verified().execution_at()
    );
    assert_eq!(contextual.evidence_digest(), verified.evidence_digest());
}

#[test]
fn portable_digest_claim_requires_actual_verification() {
    for claim in [
        "separately authenticated verifier attestation",
        "Retain audit evidence in either case",
        "contains no authenticated verifier",
        "non-repudiation",
    ] {
        assert!(
            FORMAT.contains(claim),
            "document trust requirement: {claim}"
        );
    }
    let keys = ap2_external::MandateKeys::pinned();
    let mandate_checker = Ap2ExternalMandateVerifier::new(&keys, ap2_external::suite())
        .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    let mut fixture = ap2_external::MandateFixture::direct(false);
    fixture.payment = ap2_external::corrupt_signature(&fixture.payment);
    let invalid = fixture.payload();
    // Anyone can hash invalid evidence. Only the production verifier can admit it.
    let digest = documented_digest(&invalid, mandate_checker.descriptor());
    assert_eq!(digest.len(), 32);
    assert!(matches!(
        verify_external_evidence(
            &mandate_checker,
            &presentation(&invalid),
            &ap2_external::context()
        ),
        Err(ExternalVerificationError::Adapter(
            Ap2Error::InvalidSignature
        ))
    ));

    let valid = ap2_external::MandateFixture::direct(false).payload();
    assert_ne!(
        digest,
        documented_digest(&valid, mandate_checker.descriptor())
    );
    let mut untrusted = ap2_external::MandateKeys::pinned();
    untrusted.pins.clear();
    let untrusted_verifier = Ap2ExternalMandateVerifier::new(&untrusted, ap2_external::suite())
        .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    assert_eq!(
        documented_digest(&valid, mandate_checker.descriptor()),
        documented_digest(&valid, untrusted_verifier.descriptor())
    );
    assert!(verify_external_evidence(
        &mandate_checker,
        &presentation(&valid),
        &ap2_external::context()
    )
    .is_ok());
    assert!(matches!(
        verify_external_evidence(
            &untrusted_verifier,
            &presentation(&valid),
            &ap2_external::context()
        ),
        Err(ExternalVerificationError::Adapter(Ap2Error::KeyResolution))
    ));
}
