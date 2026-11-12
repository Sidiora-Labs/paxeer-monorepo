//! Tests for verifying external mandates and receipts through adapters.
//!
//! These tests prove that external protocol evidence is bound to exact
//! presentation bytes, pinned specifications, and conformance suite digests.

#[path = "support/ap2_external.rs"]
mod ap2_external;

use layerx_interop_gateway::adapter::{
    AdapterDescriptor, AdapterId, ConformanceSuite, PinnedSpec, SpecVersion,
};
use layerx_portable::{
    verify_external_evidence, ExternalEvidenceKind, ExternalEvidenceVerifier, ExternalPresentation,
    ExternalPresentationError, ExternalVerificationError,
};

struct MockMandateVerifier {
    descriptor: AdapterDescriptor,
}

impl MockMandateVerifier {
    fn new() -> Self {
        let adapter_id = AdapterId::new("test-adapter")
            .unwrap_or_else(|error| panic!("valid adapter id: {error:?}"));
        let protocol_id = AdapterId::new("test-protocol")
            .unwrap_or_else(|error| panic!("valid protocol id: {error:?}"));
        let spec_version = SpecVersion::parse("1.0.0")
            .unwrap_or_else(|error| panic!("valid spec version: {error:?}"));
        let spec_document_digest = [1u8; 32];
        let spec = PinnedSpec::new(protocol_id, spec_version, spec_document_digest)
            .unwrap_or_else(|error| panic!("valid pinned spec: {error:?}"));
        let suite_id = AdapterId::new("test-suite-v1")
            .unwrap_or_else(|error| panic!("valid suite id: {error:?}"));
        let conformance = ConformanceSuite::new(suite_id, 10, [2u8; 32])
            .unwrap_or_else(|error| panic!("valid conformance suite: {error:?}"));
        let descriptor = AdapterDescriptor::new(adapter_id, spec, conformance);
        Self { descriptor }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct VerifiedMandate {
    payload_size: usize,
}

#[derive(Clone, Debug, Eq, PartialEq)]
enum MockVerificationError {
    InvalidSignature,
    ExpiredTimestamp,
}

impl std::fmt::Display for MockVerificationError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::InvalidSignature => formatter.write_str("invalid signature"),
            Self::ExpiredTimestamp => formatter.write_str("expired timestamp"),
        }
    }
}

impl std::error::Error for MockVerificationError {}

impl ExternalEvidenceVerifier<()> for MockMandateVerifier {
    type Verified = VerifiedMandate;
    type Error = MockVerificationError;

    fn descriptor(&self) -> &AdapterDescriptor {
        &self.descriptor
    }

    fn evidence_kind(&self) -> ExternalEvidenceKind {
        ExternalEvidenceKind::Mandate
    }

    fn media_type(&self) -> &'static str {
        "application/test-mandate+json"
    }

    fn verify(&self, payload: &[u8], _context: &()) -> Result<Self::Verified, Self::Error> {
        if payload.len() < 10 {
            return Err(MockVerificationError::InvalidSignature);
        }
        if payload[0] == 0xFF {
            return Err(MockVerificationError::ExpiredTimestamp);
        }
        Ok(VerifiedMandate {
            payload_size: payload.len(),
        })
    }
}

#[test]
fn verify_external_mandate_with_matching_presentation() {
    let evidence_checker = MockMandateVerifier::new();
    let payload = b"valid-mandate-payload-data";
    let presentation = ExternalPresentation::new(
        "test-adapter",
        "test-protocol",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "application/test-mandate+json",
        payload,
    )
    .unwrap_or_else(|error| panic!("valid presentation: {error:?}"));

    let result = verify_external_evidence(&evidence_checker, &presentation, &());
    assert!(
        result.is_ok(),
        "Valid mandate must verify successfully: {result:?}"
    );

    let verified = result.unwrap_or_else(|error| panic!("verification succeeds: {error:?}"));
    assert_eq!(verified.adapter(), "test-adapter");
    assert_eq!(verified.protocol(), "test-protocol");
    assert_eq!(verified.spec_version(), "1.0.0");
    assert_eq!(verified.kind(), ExternalEvidenceKind::Mandate);
    assert_eq!(verified.media_type(), "application/test-mandate+json");
    assert_eq!(verified.conformance_suite(), "test-suite-v1");
    assert_eq!(verified.conformance_vector_count(), 10);
    assert_eq!(verified.verified().payload_size, payload.len());
}

#[test]
fn reject_adapter_id_mismatch() {
    let evidence_checker = MockMandateVerifier::new();
    let payload = b"valid-mandate-payload-data";
    let presentation = ExternalPresentation::new(
        "wrong-adapter",
        "test-protocol",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "application/test-mandate+json",
        payload,
    )
    .unwrap_or_else(|error| panic!("valid presentation: {error:?}"));

    let result = verify_external_evidence(&evidence_checker, &presentation, &());
    match result {
        Err(ExternalVerificationError::DescriptorMismatch) => {}
        other => panic!("Must reject adapter mismatch, got {other:?}"),
    }
}

#[test]
fn reject_protocol_id_mismatch() {
    let evidence_checker = MockMandateVerifier::new();
    let payload = b"valid-mandate-payload-data";
    let presentation = ExternalPresentation::new(
        "test-adapter",
        "wrong-protocol",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "application/test-mandate+json",
        payload,
    )
    .unwrap_or_else(|error| panic!("valid presentation: {error:?}"));

    let result = verify_external_evidence(&evidence_checker, &presentation, &());
    match result {
        Err(ExternalVerificationError::DescriptorMismatch) => {}
        other => panic!("Must reject protocol mismatch, got {other:?}"),
    }
}

#[test]
fn reject_spec_version_mismatch() {
    let evidence_checker = MockMandateVerifier::new();
    let payload = b"valid-mandate-payload-data";
    let presentation = ExternalPresentation::new(
        "test-adapter",
        "test-protocol",
        "2.0.0",
        ExternalEvidenceKind::Mandate,
        "application/test-mandate+json",
        payload,
    )
    .unwrap_or_else(|error| panic!("valid presentation: {error:?}"));

    let result = verify_external_evidence(&evidence_checker, &presentation, &());
    match result {
        Err(ExternalVerificationError::DescriptorMismatch) => {}
        other => panic!("Must reject version mismatch, got {other:?}"),
    }
}

#[test]
fn reject_evidence_kind_mismatch() {
    let evidence_checker = MockMandateVerifier::new();
    let payload = b"valid-mandate-payload-data";
    let presentation = ExternalPresentation::new(
        "test-adapter",
        "test-protocol",
        "1.0.0",
        ExternalEvidenceKind::Receipt,
        "application/test-mandate+json",
        payload,
    )
    .unwrap_or_else(|error| panic!("valid presentation: {error:?}"));

    let result = verify_external_evidence(&evidence_checker, &presentation, &());
    match result {
        Err(ExternalVerificationError::EvidenceKindMismatch) => {}
        other => panic!("Must reject evidence kind mismatch, got {other:?}"),
    }
}

#[test]
fn reject_media_type_mismatch() {
    let evidence_checker = MockMandateVerifier::new();
    let payload = b"valid-mandate-payload-data";
    let presentation = ExternalPresentation::new(
        "test-adapter",
        "test-protocol",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "application/wrong-media-type+json",
        payload,
    )
    .unwrap_or_else(|error| panic!("valid presentation: {error:?}"));

    let result = verify_external_evidence(&evidence_checker, &presentation, &());
    match result {
        Err(ExternalVerificationError::MediaTypeMismatch) => {}
        other => panic!("Must reject media type mismatch, got {other:?}"),
    }
}

#[test]
fn preserve_adapter_verification_error() {
    let evidence_checker = MockMandateVerifier::new();
    let payload = b"short";
    let presentation = ExternalPresentation::new(
        "test-adapter",
        "test-protocol",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "application/test-mandate+json",
        payload,
    )
    .unwrap_or_else(|error| panic!("valid presentation: {error:?}"));

    let result = verify_external_evidence(&evidence_checker, &presentation, &());
    match result {
        Err(ExternalVerificationError::Adapter(MockVerificationError::InvalidSignature)) => {}
        other => panic!("Must preserve adapter error, got {other:?}"),
    }
}

#[test]
fn evidence_digest_changes_with_payload() {
    let evidence_checker = MockMandateVerifier::new();
    let payload1 = b"mandate-payload-one";
    let payload2 = b"mandate-payload-two";

    let presentation1 = ExternalPresentation::new(
        "test-adapter",
        "test-protocol",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "application/test-mandate+json",
        payload1,
    )
    .unwrap_or_else(|error| panic!("valid presentation: {error:?}"));

    let presentation2 = ExternalPresentation::new(
        "test-adapter",
        "test-protocol",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "application/test-mandate+json",
        payload2,
    )
    .unwrap_or_else(|error| panic!("valid presentation: {error:?}"));

    let verified1 = verify_external_evidence(&evidence_checker, &presentation1, &())
        .unwrap_or_else(|error| panic!("verification 1 succeeds: {error:?}"));
    let verified2 = verify_external_evidence(&evidence_checker, &presentation2, &())
        .unwrap_or_else(|error| panic!("verification 2 succeeds: {error:?}"));

    assert_ne!(
        verified1.evidence_digest(),
        verified2.evidence_digest(),
        "Evidence digest must change with payload"
    );
}

#[test]
fn reject_invalid_adapter_id() {
    let payload = b"valid-mandate-payload-data";
    let result = ExternalPresentation::new(
        "INVALID ID",
        "test-protocol",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "application/test-mandate+json",
        payload,
    );
    match result {
        Err(ExternalPresentationError::InvalidAdapter) => {}
        other => panic!("Must reject invalid adapter id, got {other:?}"),
    }
}

#[test]
fn reject_invalid_protocol_id() {
    let payload = b"valid-mandate-payload-data";
    let result = ExternalPresentation::new(
        "test-adapter",
        "",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "application/test-mandate+json",
        payload,
    );
    match result {
        Err(ExternalPresentationError::InvalidProtocol) => {}
        other => panic!("Must reject invalid protocol id, got {other:?}"),
    }
}

#[test]
fn reject_unpinned_version() {
    let payload = b"valid-mandate-payload-data";
    let result = ExternalPresentation::new(
        "test-adapter",
        "test-protocol",
        "1.x",
        ExternalEvidenceKind::Mandate,
        "application/test-mandate+json",
        payload,
    );
    match result {
        Err(ExternalPresentationError::UnpinnedVersion) => {}
        other => panic!("Must reject unpinned version, got {other:?}"),
    }
}

#[test]
fn reject_empty_media_type() {
    let payload = b"valid-mandate-payload-data";
    let result = ExternalPresentation::new(
        "test-adapter",
        "test-protocol",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "",
        payload,
    );
    match result {
        Err(ExternalPresentationError::InvalidMediaType) => {}
        other => panic!("Must reject empty media type, got {other:?}"),
    }
}

#[test]
fn reject_media_type_with_control_characters() {
    let payload = b"valid-mandate-payload-data";
    let result = ExternalPresentation::new(
        "test-adapter",
        "test-protocol",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "application/test\x00+json",
        payload,
    );
    match result {
        Err(ExternalPresentationError::InvalidMediaType) => {}
        other => panic!("Must reject media type with control chars, got {other:?}"),
    }
}

#[test]
fn reject_empty_payload() {
    let result = ExternalPresentation::new(
        "test-adapter",
        "test-protocol",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "application/test-mandate+json",
        b"",
    );
    match result {
        Err(ExternalPresentationError::PayloadBounds) => {}
        other => panic!("Must reject empty payload, got {other:?}"),
    }
}

#[test]
fn reject_oversized_payload() {
    let oversized_payload = vec![0u8; 2_097_153];
    let result = ExternalPresentation::new(
        "test-adapter",
        "test-protocol",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "application/test-mandate+json",
        &oversized_payload,
    );
    match result {
        Err(ExternalPresentationError::PayloadBounds) => {}
        other => panic!("Must reject oversized payload, got {other:?}"),
    }
}

#[test]
fn verified_external_evidence_binds_all_inputs() {
    let evidence_checker = MockMandateVerifier::new();
    let payload = b"mandate-payload-complete-binding-test";
    let presentation = ExternalPresentation::new(
        "test-adapter",
        "test-protocol",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        "application/test-mandate+json",
        payload,
    )
    .unwrap_or_else(|error| panic!("valid presentation: {error:?}"));

    let verified = verify_external_evidence(&evidence_checker, &presentation, &())
        .unwrap_or_else(|error| panic!("verification succeeds: {error:?}"));

    assert_eq!(verified.adapter(), presentation.adapter());
    assert_eq!(verified.protocol(), presentation.protocol());
    assert_eq!(verified.spec_version(), presentation.spec_version());
    assert_eq!(verified.kind(), presentation.kind());
    assert_eq!(verified.media_type(), presentation.media_type());
    assert_eq!(
        verified.spec_document_digest(),
        evidence_checker.descriptor().spec().document_digest()
    );
    assert_eq!(
        verified.conformance_suite(),
        evidence_checker.descriptor().conformance().suite().as_str()
    );
    assert_eq!(
        verified.conformance_vector_count(),
        evidence_checker.descriptor().conformance().vector_count()
    );
    assert_eq!(
        verified.conformance_suite_digest(),
        evidence_checker.descriptor().conformance().suite_digest()
    );
}

fn ap2_presentation(payload: &[u8]) -> ExternalPresentation<'_> {
    ExternalPresentation::new(
        "ap2",
        "ap2",
        "1.0.0",
        ExternalEvidenceKind::Mandate,
        layerx_ap2::AP2_MANDATE_PAIR_MEDIA_TYPE,
        payload,
    )
    .unwrap_or_else(|error| panic!("AP2 presentation: {error:?}"))
}

#[test]
fn portable_external_path() {
    use layerx_ap2::{Ap2ExternalMandateVerifier, MandateMode, AP2_SPEC_SHA256};

    let fixture = ap2_external::MandateFixture::direct(false);
    let keys = ap2_external::MandateKeys::pinned();
    let suite = ap2_external::suite();
    let mandate_checker = Ap2ExternalMandateVerifier::new(&keys, suite.clone())
        .unwrap_or_else(|error| panic!("production AP2 verifier: {error:?}"));
    let payload = fixture.payload();
    let presentation = ap2_presentation(&payload);
    let verified =
        verify_external_evidence(&mandate_checker, &presentation, &ap2_external::context())
            .unwrap_or_else(|error| panic!("signed AP2 pair verifies: {error:?}"));
    assert_eq!(verified.adapter(), "ap2");
    assert_eq!(verified.protocol(), "ap2");
    assert_eq!(verified.spec_version(), "1.0.0");
    assert_eq!(verified.kind(), ExternalEvidenceKind::Mandate);
    assert_eq!(
        verified.media_type(),
        layerx_ap2::AP2_MANDATE_PAIR_MEDIA_TYPE
    );
    assert_eq!(verified.spec_document_digest(), AP2_SPEC_SHA256);
    assert_eq!(verified.conformance_suite(), suite.suite().as_str());
    assert_eq!(verified.conformance_vector_count(), suite.vector_count());
    assert_eq!(verified.conformance_suite_digest(), suite.suite_digest());
    let mandates = verified.verified();
    assert_eq!(mandates.mode(), MandateMode::Direct);
    assert_eq!(
        mandates.checkout_id(),
        fixture.expected["checkout_id"]
            .as_str()
            .unwrap_or_else(|| panic!("checkout id"))
    );
    assert_eq!(mandates.transaction_id(), fixture.transaction_id);
    assert_eq!(
        mandates.payee().id(),
        fixture.expected["payee_id"]
            .as_str()
            .unwrap_or_else(|| panic!("payee id"))
    );
    assert_eq!(mandates.amount().minor_units(), 10_000);
    assert_eq!(mandates.amount().currency(), "USD");
    assert_eq!(mandates.execution_at(), ap2_external::NOW);
    assert_eq!(mandates.payment_instrument().id(), "card-001");

    let mut spaced_payload = payload.clone();
    spaced_payload.push(b'\n');
    let spaced = verify_external_evidence(
        &mandate_checker,
        &ap2_presentation(&spaced_payload),
        &ap2_external::context(),
    )
    .unwrap_or_else(|error| panic!("same signed tokens with JSON whitespace: {error:?}"));
    assert_eq!(
        spaced.verified().transaction_id(),
        mandates.transaction_id()
    );
    assert_eq!(
        spaced.verified().checkout_receipt_reference(),
        mandates.checkout_receipt_reference()
    );
    assert_eq!(
        spaced.verified().payment_receipt_reference(),
        mandates.payment_receipt_reference()
    );
    assert_ne!(
        spaced.evidence_digest(),
        verified.evidence_digest(),
        "binding includes exact transported bytes"
    );
}

#[test]
fn portable_external_path_rejects_mandate_signature_tampering() {
    let keys = ap2_external::MandateKeys::pinned();
    let mandate_checker = layerx_ap2::Ap2ExternalMandateVerifier::new(&keys, ap2_external::suite())
        .unwrap_or_else(|error| panic!("production AP2 verifier: {error:?}"));
    for tamper_checkout in [true, false] {
        let mut fixture = ap2_external::MandateFixture::direct(false);
        let token = if tamper_checkout {
            &mut fixture.checkout
        } else {
            &mut fixture.payment
        };
        *token = ap2_external::corrupt_signature(token);
        let payload = fixture.payload();
        assert!(
            matches!(
                verify_external_evidence(
                    &mandate_checker,
                    &ap2_presentation(&payload),
                    &ap2_external::context()
                ),
                Err(ExternalVerificationError::Adapter(
                    layerx_ap2::Ap2Error::InvalidSignature
                ))
            ),
            "tampered {} signature must be refused",
            if tamper_checkout {
                "checkout"
            } else {
                "payment"
            }
        );
    }
}

#[test]
fn portable_external_path_rejects_merchant_signature_tampering() {
    let keys = ap2_external::MandateKeys::pinned();
    let mandate_checker = layerx_ap2::Ap2ExternalMandateVerifier::new(&keys, ap2_external::suite())
        .unwrap_or_else(|error| panic!("production AP2 verifier: {error:?}"));
    // Both issuer signatures and checkout hashes bind the altered merchant JWS,
    // so only production verification of the nested merchant signature refuses it.
    let payload = ap2_external::MandateFixture::direct(true).payload();
    assert!(matches!(
        verify_external_evidence(
            &mandate_checker,
            &ap2_presentation(&payload),
            &ap2_external::context()
        ),
        Err(ExternalVerificationError::Adapter(
            layerx_ap2::Ap2Error::InvalidSignature
        ))
    ));
}

#[test]
fn portable_external_path_requires_purpose_scoped_trust() {
    for missing_purpose in [
        layerx_ap2::KeyUse::CheckoutMandateIssuer,
        layerx_ap2::KeyUse::PaymentMandateIssuer,
        layerx_ap2::KeyUse::MerchantCheckout,
    ] {
        let mut keys = ap2_external::MandateKeys::pinned();
        keys.pins
            .retain(|(purpose, _, _)| *purpose != missing_purpose);
        let mandate_checker =
            layerx_ap2::Ap2ExternalMandateVerifier::new(&keys, ap2_external::suite())
                .unwrap_or_else(|error| panic!("production AP2 verifier: {error:?}"));
        let payload = ap2_external::MandateFixture::direct(false).payload();
        assert!(
            matches!(
                verify_external_evidence(
                    &mandate_checker,
                    &ap2_presentation(&payload),
                    &ap2_external::context()
                ),
                Err(ExternalVerificationError::Adapter(
                    layerx_ap2::Ap2Error::KeyResolution
                ))
            ),
            "missing {missing_purpose:?} pin must be refused"
        );
    }
}
