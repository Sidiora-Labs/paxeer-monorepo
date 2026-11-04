//! Audit 22.30.112: exercise the documented receipt and external evidence paths.
//! Finding-to-test mapping and baseline limitations are in repair_22_33_117.kvx.

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
        .expect("fixture hex")
        .strip_prefix("0x")
        .expect("prefix");
    hex.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("ASCII"), 16).expect("hex byte")
        })
        .collect()
}

fn recorded_receipt() -> (Value, ReceiptKeyEntry, Vec<u8>) {
    let fixture: Value = serde_json::from_str(RECEIPTS).expect("recorded producer transcript");
    assert_eq!(
        fixture["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    let case = fixture["cases"]
        .as_array()
        .expect("cases")
        .iter()
        .find(|case| case["id"] == "V1-pax-transfer")
        .expect("recorded PAX transfer");
    let pin = &case["pinnedKey"];
    let key = ReceiptKeyEntry::new(
        bytes(&pin["keyId"]).try_into().expect("key id"),
        bytes(&pin["publicKey"]).try_into().expect("public key"),
        pin["firstHeight"].as_u64().expect("first height"),
        pin["lastHeight"].as_u64().expect("last height"),
    );
    (
        case["receiptJson"].clone(),
        key,
        bytes(&case["rawTransaction"]),
    )
}

fn wire_value(answer: &Value, pin: ReceiptKeyEntry, raw: &[u8]) -> Value {
    let portable = PortableReceiptV2::from_sequencer_receipt(answer, &[pin], Some(raw))
        .expect("production exporter accepts producer transcript");
    serde_json::from_slice(&portable.to_json().expect("production JSON codec"))
        .expect("wire object")
}

#[test]
fn portable_format_versions_real_round_trip_and_windows() {
    assert!(FORMAT.starts_with("# LayerX portable verification formats\n\nThe current receipt format is `layerx-receipt-proof-v2`."));
    assert!(FORMAT.contains("window must cover the **signed height**"));
    let (answer, pin, raw) = recorded_receipt();
    let height = u64::from_str_radix(
        answer["height"].as_str().unwrap().trim_start_matches("0x"),
        16,
    )
    .unwrap();
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
        let PortableVerified::V2(verified) =
            verify_portable_json(&serde_json::to_vec(&wire).unwrap(), &[endpoint_pin])
                .expect("real offline consumer verifies exported JSON");
        assert_eq!(verified.height(), height);
        assert_eq!(verified.inclusion_height(), last + 1);
        assert_eq!(verified.raw_transaction(), Some(raw.as_slice()));
        assert_eq!(verified.activity_id().as_slice(), bytes(&answer["txHash"]));
        assert_eq!(verified.result_code(), 0);
    }
}

#[test]
fn portable_format_versions_current_importer_refusals() {
    assert!(FORMAT.contains("### Current importer refusals"));
    assert!(FORMAT.contains("## Retired v1 migration appendix"));
    let (answer, pin, raw) = recorded_receipt();
    let wire = wire_value(&answer, pin, &raw);
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
        *changed.pointer_mut(pointer).expect("existing field") = replacement;
        assert_eq!(
            verify_portable_json(&serde_json::to_vec(&changed).unwrap(), &[pin]),
            Err(expected),
            "{pointer}"
        );
        let refusal = format!("{expected:?}");
        assert!(
            FORMAT.contains(refusal.split('(').next().unwrap()),
            "documented refusal {expected:?}"
        );
    }
    let encoded = serde_json::to_string(&wire).unwrap();
    let duplicate = format!(
        "{{\"signedReceipt\":{},{}",
        serde_json::to_string(&wire["signedReceipt"]).unwrap(),
        &encoded[1..]
    );
    assert_eq!(
        verify_portable_json(duplicate.as_bytes(), &[pin]),
        Err(PortableReceiptError::JsonShape)
    );
    let mut unknown = wire.clone();
    unknown["receiptKey"]["untrusted"] = true.into();
    assert_eq!(
        verify_portable_json(&serde_json::to_vec(&unknown).unwrap(), &[pin]),
        Err(PortableReceiptError::JsonShape)
    );
    let mut missing = wire.clone();
    missing.as_object_mut().unwrap().remove("receiptDigest");
    assert_eq!(
        verify_portable_json(&serde_json::to_vec(&missing).unwrap(), &[pin]),
        Err(PortableReceiptError::JsonShape)
    );
    let signature_failure = Err(PortableReceiptError::Receipt(VerificationFailure {
        check: ReceiptCheck::SequencerSignature,
    }));
    let encoded = serde_json::to_vec(&wire).unwrap();
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
    *signed.last_mut().unwrap() ^= 1;
    tampered["signedReceipt"] = format!(
        "0x{}",
        signed
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    )
    .into();
    assert_eq!(
        verify_portable_json(&serde_json::to_vec(&tampered).unwrap(), &[pin]),
        signature_failure
    );
    assert_eq!(
        PortableReceiptV2::from_sequencer_receipt(&json!({"rung":"submitted"}), &[pin], None),
        Err(PortableReceiptError::NotReceipted)
    );
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
    .expect("real AP2 presentation envelope")
}

// Independent encoding of the FORMAT.md digest formula, including every descriptor field.
fn documented_digest(payload: &[u8], descriptor: &AdapterDescriptor) -> [u8; 32] {
    fn part(hash: &mut Sha256, bytes: &[u8]) {
        hash.update(u64::try_from(bytes.len()).unwrap().to_be_bytes());
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
    let verifier = Ap2ExternalMandateVerifier::new(&keys, ap2_external::suite()).unwrap();
    let fixture = ap2_external::MandateFixture::direct(false);
    let payload = fixture.payload();
    let verified =
        verify_external_evidence(&verifier, &presentation(&payload), &ap2_external::context())
            .expect("production AP2 verification");
    assert_eq!(verified.verified().transaction_id(), fixture.transaction_id);
    assert_eq!(
        verified.verified().checkout_id(),
        fixture.expected["checkout_id"].as_str().unwrap()
    );
    assert_eq!(verified.verified().amount().minor_units(), 10_000);
    assert_eq!(
        verified.evidence_digest(),
        documented_digest(&payload, verifier.descriptor())
    );

    let mut spaced = payload.clone();
    spaced.push(b'\n');
    let whitespace =
        verify_external_evidence(&verifier, &presentation(&spaced), &ap2_external::context())
            .unwrap();
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
        AdapterId::new("retained-audit-vector").unwrap(),
        1,
        Sha256::digest(&payload).into(),
    )
    .unwrap();
    let other_verifier = Ap2ExternalMandateVerifier::new(&keys, suite).unwrap();
    let other = verify_external_evidence(
        &other_verifier,
        &presentation(&payload),
        &ap2_external::context(),
    )
    .unwrap();
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
    let contextual = verify_external_evidence(&verifier, &presentation(&payload), &later).unwrap();
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
    let verifier = Ap2ExternalMandateVerifier::new(&keys, ap2_external::suite()).unwrap();
    let mut fixture = ap2_external::MandateFixture::direct(false);
    fixture.payment = ap2_external::corrupt_signature(&fixture.payment);
    let invalid = fixture.payload();
    // Anyone can hash invalid evidence. Only the production verifier can admit it.
    let digest = documented_digest(&invalid, verifier.descriptor());
    assert_eq!(digest.len(), 32);
    assert!(matches!(
        verify_external_evidence(&verifier, &presentation(&invalid), &ap2_external::context()),
        Err(ExternalVerificationError::Adapter(
            Ap2Error::InvalidSignature
        ))
    ));

    let valid = ap2_external::MandateFixture::direct(false).payload();
    assert_ne!(digest, documented_digest(&valid, verifier.descriptor()));
    let mut untrusted = ap2_external::MandateKeys::pinned();
    untrusted.pins.clear();
    let untrusted_verifier =
        Ap2ExternalMandateVerifier::new(&untrusted, ap2_external::suite()).unwrap();
    assert_eq!(
        documented_digest(&valid, verifier.descriptor()),
        documented_digest(&valid, untrusted_verifier.descriptor())
    );
    assert!(
        verify_external_evidence(&verifier, &presentation(&valid), &ap2_external::context())
            .is_ok()
    );
    assert!(matches!(
        verify_external_evidence(
            &untrusted_verifier,
            &presentation(&valid),
            &ap2_external::context()
        ),
        Err(ExternalVerificationError::Adapter(Ap2Error::KeyResolution))
    ));
}
