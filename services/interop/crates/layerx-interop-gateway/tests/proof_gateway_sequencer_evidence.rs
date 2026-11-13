//! Proves: `SequencerEvidence`, `SettlementRung`, `canonical_receipt_digest`, `GatewayError::ReceiptPending` -> public Rust settlement and error-emission bytes -> `GatewayCore::settle_with_sequencer_evidence`, `ExternalState::settlement_error_response`, `redaction::decode`.
//!
//! Receipts and logs are unchanged outputs of the real producer recorded by
//! `rpc/sequencer_receipt_vectors_test.go`, not locally assembled signed messages.
//! The evidence surface has no persistence or reopen operation: each observation
//! calls the real consumers again, including malformed evidence after settlement.
//!
//! Repairs `src/evidence.rs::canonical_receipt_digest` accepting a malformed receipt
//! body with valid signature framing: canonical decoding must precede hashing.
//!
//! The recorded `PaymentExact` reference is checked against the real transaction's
//! calldata, and evidence for that reference cannot settle another translation.

use layerx_interop_gateway::adapter::{
    AdapterDescriptor, AdapterId, ConformanceSuite, PinnedSpec, SpecVersion,
};
use layerx_interop_gateway::audit::AuditEventKind;
use layerx_interop_gateway::error::{error_emission, GatewayError, Retriability};
use layerx_interop_gateway::evidence::{
    canonical_receipt_digest, SequencerEvidence, SettlementRung,
};
use layerx_interop_gateway::gateway::{
    SequencerSettlementError, TranslationKind, TranslationRequest, TranslationStatus,
};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::redaction::{self, FieldValue, ERROR_RESPONSE_SCHEMA};
use layerx_interop_gateway::server::ExternalState;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{
    verify_sequencer_signature, ReceiptCheck, ReceiptKeys, VerificationFailure,
};
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_wire::hash::receipt_digest;
use layerx_wire::receipt::{decode, encode, encode_unsigned};
use serde_json::Value;

const FIXTURE: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
));

fn bytes(value: &Value) -> Vec<u8> {
    let hex = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .or_panic(&format!("expected producer hex bytes: {value}"));
    assert_eq!(hex.len() % 2, 0);
    (0..hex.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&hex[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("producer hex: {error}"))
        })
        .collect()
}

fn array<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value)
        .try_into()
        .unwrap_or_else(|_| panic!("producer field must have {N} bytes"))
}

fn payment_case() -> Value {
    let document: Value =
        serde_json::from_str(FIXTURE).unwrap_or_else(|error| panic!("producer recording: {error}"));
    assert_eq!(
        document["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    document["cases"]
        .as_array()
        .or_panic("missing producer cases")
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .or_panic("missing real payExact output")
        .clone()
}

fn evidence(case: &Value) -> SequencerEvidence {
    let response = &case["receiptJson"];
    assert_eq!(response["rung"], "receipted");
    assert_eq!(bytes(&response["receipt"]), bytes(&case["signedReceipt"]));
    let logs = response["logs"]
        .as_array()
        .or_panic("producer logs missing")
        .iter()
        .map(|log| {
            let topics = log["topics"]
                .as_array()
                .or_panic("producer topics missing")
                .iter()
                .map(array)
                .collect();
            let index = log["logIndex"]
                .as_str()
                .and_then(|text| text.strip_prefix("0x"))
                .and_then(|text| u64::from_str_radix(text, 16).ok())
                .or_panic("producer log index missing");
            SequencerLog::new(array(&log["address"]), topics, bytes(&log["data"]), index)
        })
        .collect();
    SequencerEvidence {
        transaction_hash: array(&response["txHash"]),
        signed_receipt: bytes(&response["receipt"]),
        logs,
        rung: SettlementRung::Receipted,
    }
}

fn keys(case: &Value) -> [SequencerAuthorization; 1] {
    let pin = &case["pinnedKey"];
    [SequencerAuthorization::new(
        array(&pin["keyId"]),
        array(&pin["publicKey"]),
        pin["firstHeight"].as_u64().or_panic("first height"),
        pin["lastHeight"].as_u64().or_panic("last height"),
    )]
}

fn pending(key: [u8; 32]) -> (GatewayCore, PrincipalId, TraceId) {
    let mut core = GatewayCore::new();
    let trace = TraceId::mint([0x11; 16]);
    let principal = PrincipalId::new("evidence-proof-buyer")
        .unwrap_or_else(|error| panic!("principal: {error}"));
    let adapter = AdapterId::new("x402").unwrap_or_else(|error| panic!("adapter: {error}"));
    let version = SpecVersion::parse("2.0.0").unwrap_or_else(|error| panic!("version: {error}"));
    let spec = PinnedSpec::new(adapter.clone(), version, [0x51; 32])
        .unwrap_or_else(|error| panic!("spec: {error}"));
    let suite_id =
        AdapterId::new("sequencer-proof").unwrap_or_else(|error| panic!("suite id: {error}"));
    let suite = ConformanceSuite::new(suite_id, 1, [0x52; 32])
        .unwrap_or_else(|error| panic!("suite: {error}"));
    core.register_adapter(
        AdapterDescriptor::new(adapter.clone(), spec, suite),
        &trace,
        1,
    )
    .unwrap_or_else(|error| panic!("register: {error}"));
    let request = TranslationRequest::new(adapter, TranslationKind::StateChanging, key, [0x42; 32])
        .unwrap_or_else(|error| panic!("request: {error}"));
    assert_eq!(
        core.begin_translation(&principal, &request, &trace, 2)
            .unwrap_or_else(|error| panic!("begin: {error}")),
        TranslationStatus::Pending
    );
    (core, principal, trace)
}

fn rejected(check: ReceiptCheck) -> GatewayError {
    GatewayError::ReceiptRejected(VerificationFailure { check })
}

// The declared graph symbol Vec names this test of the public owned evidence vectors.
#[test]
#[allow(non_snake_case)]
fn Vec() {
    let case = payment_case();
    let original = evidence(&case);
    let expected_digest = array(&case["receiptDigest"]);
    let keys = keys(&case);
    for rung in [
        SettlementRung::Receipted,
        SettlementRung::Committed {
            block_number: 1001,
            block_hash: [0x42; 32],
        },
    ] {
        let mut evidence = original.clone();
        evidence.rung = rung;
        assert_eq!(
            rung.label(),
            if matches!(rung, SettlementRung::Receipted) {
                "receipted"
            } else {
                "committed"
            }
        );
        // The gateway consumes the indexed paymentReference from the signed log.
        let key = evidence.logs[0].topics()[1];
        let (mut core, principal, trace) = pending(key);
        for now in [3, 4] {
            let receipt = verify_sequencer_signature(
                &evidence.signed_receipt,
                ReceiptKeys::HeightRanges(&keys),
            )
            .or_panic("real signature consumer");
            assert_eq!(encode(&receipt).or_panic("encode"), evidence.signed_receipt);
            assert_eq!(
                receipt
                    .protocol()
                    .or_panic("protocol receipt")
                    .activity_id(),
                evidence.transaction_hash
            );
            let verified_logs =
                verify_logs(&receipt, evidence.logs.clone()).or_panic("real log consumer");
            assert_eq!(verified_logs.logs(), evidence.logs);
            assert_eq!(verified_logs.summary().log_count(), 1);
            let unsigned = encode_unsigned(&receipt).or_panic("canonical unsigned producer");
            assert_eq!(
                receipt_digest(&unsigned).or_panic("wire digest"),
                expected_digest
            );
            assert_eq!(
                canonical_receipt_digest(&evidence.signed_receipt),
                Ok(expected_digest)
            );
            assert_eq!(core.settle_with_sequencer_evidence(
                &principal, key, &evidence, &keys, &trace, now,
            ).unwrap_or_else(|error| panic!("real settlement consumer: {error}")),
                TranslationStatus::ReceiptVerified { receipt_digest: expected_digest });
            assert_eq!(
                core.translation(&principal, key),
                Some(TranslationStatus::ReceiptVerified {
                    receipt_digest: expected_digest
                })
            );
            assert_completed_audit(&core, &principal, expected_digest);
        }
        let settled = format!("{core:?}");
        let mut malformed = evidence.clone();
        malformed.signed_receipt.truncate(10);
        let error = core
            .settle_with_sequencer_evidence(&principal, key, &malformed, &keys, &trace, 5)
            .err()
            .or_panic("expected Err value");
        assert_eq!(
            error.error(),
            &SequencerSettlementError::Gateway(rejected(ReceiptCheck::Decode))
        );
        assert_eq!(error.trace(), &trace);
        let mut tampered = evidence;
        let signature_byte = tampered
            .signed_receipt
            .last_mut()
            .or_panic("missing producer signature");
        *signature_byte ^= 1;
        let error = core
            .settle_with_sequencer_evidence(&principal, key, &tampered, &keys, &trace, 6)
            .err()
            .or_panic("expected Err value");
        assert_eq!(
            error.error(),
            &SequencerSettlementError::Gateway(rejected(ReceiptCheck::SequencerSignature))
        );
        assert_eq!(
            format!("{core:?}"),
            settled,
            "refusals must preserve settled state"
        );
    }
}

fn assert_completed_audit(core: &GatewayCore, principal: &PrincipalId, digest: [u8; 32]) {
    let audit = core
        .principal_audit(principal)
        .or_panic("missing real audit");
    assert_eq!(
        audit.entries().len(),
        2,
        "re-observation must not duplicate completion"
    );
    assert_eq!(
        audit.entries()[1].kind(),
        AuditEventKind::TranslationCompleted
    );
    assert_eq!(audit.entries()[1].subject(), digest);
}

#[test]
fn submitted_result_reaches_the_error_response_consumer_twice_without_writes() {
    let case = payment_case();
    let mut evidence = evidence(&case);
    let key = evidence.logs[0].topics()[1];
    let (mut core, principal, trace) = pending(key);
    evidence.rung = SettlementRung::Submitted;
    evidence.signed_receipt.clear();
    evidence.logs.clear();
    assert_eq!(evidence.rung.label(), "submitted");
    let before = format!("{core:?}");
    for now in [3, 4] {
        let error = core
            .settle_with_sequencer_evidence(&principal, key, &evidence, &[], &trace, now)
            .err()
            .or_panic("expected Err value");
        assert_eq!(error.trace(), &trace);
        let SequencerSettlementError::Gateway(pending) = *error.error() else {
            panic!("submitted evidence did not return a gateway error");
        };
        assert_eq!(pending, GatewayError::ReceiptPending);
        assert_eq!(pending.code(), "receipt-pending");
        assert_eq!(pending.retriability(), Retriability::Retriable);
        assert_eq!(
            ExternalState::settlement_error_response(pending),
            Ok((202, ExternalState::Submitted))
        );
        let mut wire = error_emission(&trace.wrap(pending))
            .unwrap_or_else(|error| panic!("real error producer: {error}"));
        let emission =
            redaction::decode(&wire).unwrap_or_else(|error| panic!("real error consumer: {error}"));
        assert_eq!(emission.schema().name, ERROR_RESPONSE_SCHEMA);
        let [FieldValue::Label(error_code), FieldValue::Label(retriability), FieldValue::Trace(observed)] =
            emission.values()
        else {
            panic!("error response shape changed");
        };
        assert_eq!(error_code.as_str(), "receipt-pending");
        assert_eq!(retriability.as_str(), "retriable");
        assert_eq!(observed, &trace);
        wire.push(0);
        assert!(
            redaction::decode(&wire).is_err(),
            "consumer must refuse malformed emissions"
        );
        assert_eq!(format!("{core:?}"), before);
        assert_eq!(
            core.translation(&principal, key),
            Some(TranslationStatus::Pending)
        );
    }
    let receipted = self::evidence(&case);
    assert_eq!(
        core.settle_with_sequencer_evidence(&principal, key, &receipted, &keys(&case), &trace, 5,)
            .unwrap_or_else(|error| panic!("receipt after submitted: {error}")),
        TranslationStatus::ReceiptVerified {
            receipt_digest: array(&case["receiptDigest"])
        }
    );
    assert_eq!(
        ExternalState::settlement_error_response(rejected(ReceiptCheck::Decode)),
        Err(rejected(ReceiptCheck::Decode))
    );
}

#[test]
fn canonical_digest_refuses_malformed_bodies_even_with_a_valid_signature_trailer() {
    let case = payment_case();
    let evidence = evidence(&case);
    for _ in 0..2 {
        assert_eq!(
            canonical_receipt_digest(&evidence.signed_receipt),
            Ok(array(&case["receiptDigest"]))
        );
        let mut malformed = evidence.signed_receipt.clone();
        malformed[0] = 0xff;
        assert!(decode(&malformed).is_err());
        assert_eq!(
            canonical_receipt_digest(&malformed),
            Err(rejected(ReceiptCheck::Decode))
        );
        let trailer_only = &evidence.signed_receipt[evidence.signed_receipt.len() - 69..];
        assert_eq!(
            canonical_receipt_digest(trailer_only),
            Err(rejected(ReceiptCheck::Decode))
        );
        let decoded = decode(&evidence.signed_receipt).or_panic("real producer receipt");
        let unsigned = encode_unsigned(&decoded).or_panic("real unsigned producer");
        assert_eq!(
            canonical_receipt_digest(&unsigned),
            Err(rejected(ReceiptCheck::MissingSignature))
        );
    }
}

#[test]
fn producer_reference_is_indexed_and_a_different_translation_is_refused() {
    let case = payment_case();
    let evidence = evidence(&case);
    let raw = bytes(&case["rawTransaction"]);
    let selectors: Vec<usize> = raw
        .windows(4)
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == [0x78, 0xea, 0x11, 0xdb]).then_some(offset))
        .collect();
    assert_eq!(selectors.len(), 1, "one real payExact call");
    let calldata = &raw[selectors[0] + 4..];
    let actual_reference: [u8; 32] = calldata[5 * 32..6 * 32]
        .try_into()
        .unwrap_or_else(|_| panic!("real paymentReference word"));
    assert_eq!(actual_reference, evidence.logs[0].topics()[1]);
    let from: [u8; 32] = calldata[..32]
        .try_into()
        .unwrap_or_else(|_| panic!("real from word"));
    let to: [u8; 32] = calldata[32..64]
        .try_into()
        .unwrap_or_else(|_| panic!("real to word"));
    assert_eq!(evidence.logs[0].topics()[2..], [from, to]);
    let mut different_reference = actual_reference;
    different_reference[0] ^= 1;
    assert_ne!(different_reference, evidence.logs[0].topics()[1]);
    let (mut core, principal, trace) = pending(different_reference);
    let before = format!("{core:?}");
    for now in [3, 4] {
        let error = core
            .settle_with_sequencer_evidence(
                &principal,
                different_reference,
                &evidence,
                &keys(&case),
                &trace,
                now,
            )
            .err()
            .or_panic("expected Err value");
        assert_eq!(error.error(), &SequencerSettlementError::PaymentUnmatched);
        assert_eq!(format!("{core:?}"), before);
    }
}

fn assert_authenticated_receipt(original: &SequencerEvidence, keys: &[SequencerAuthorization]) {
    let receipt =
        verify_sequencer_signature(&original.signed_receipt, ReceiptKeys::HeightRanges(keys))
            .or_panic("real signature consumer");
    assert_eq!(
        receipt
            .protocol()
            .or_panic("protocol receipt")
            .activity_id(),
        original.transaction_hash
    );
    let verified = verify_logs(&receipt, original.logs.clone()).or_panic("real log consumer");
    assert_eq!(verified.logs(), original.logs);
}

#[test]
fn changed_transaction_hash_is_refused_without_state_or_audit_writes() {
    let case = payment_case();
    let original = evidence(&case);
    let keys = keys(&case);
    assert_authenticated_receipt(&original, &keys);
    let key = original.logs[0].topics()[1];
    let expected_digest = array(&case["receiptDigest"]);

    for rung in [
        SettlementRung::Receipted,
        SettlementRung::Committed {
            block_number: 1001,
            block_hash: [0x42; 32],
        },
    ] {
        let mut genuine = original.clone();
        genuine.rung = rung;
        let mut changed = genuine.clone();
        changed.transaction_hash[0] ^= 1;
        assert_ne!(changed.transaction_hash, genuine.transaction_hash);
        assert_eq!(changed.signed_receipt, genuine.signed_receipt);
        assert_eq!(changed.logs, genuine.logs);
        assert_eq!(changed.rung, genuine.rung);

        let (mut core, principal, trace) = pending(key);
        let absent = PrincipalId::new("absent-evidence-proof-buyer")
            .unwrap_or_else(|error| panic!("principal: {error}"));
        let before = format!("{core:?}");
        let audit_before = core
            .principal_audit(&principal)
            .or_panic("missing pending audit")
            .export();
        for now in [3, 4] {
            let error = core
                .settle_with_sequencer_evidence(&principal, key, &changed, &keys, &trace, now)
                .err()
                .unwrap_or_else(|| {
                    panic!(
                        "{}",
                        "transport transaction hash must match the authenticated activity id"
                    )
                });
            assert_eq!(
                error.error(),
                &SequencerSettlementError::Gateway(rejected(ReceiptCheck::ActivityId))
            );
            assert_eq!(error.trace(), &trace);
            assert_eq!(
                core.translation(&principal, key),
                Some(TranslationStatus::Pending)
            );
            let audit = core
                .principal_audit(&principal)
                .or_panic("missing audit after refusal");
            assert_eq!(audit.export(), audit_before);
            assert!(audit
                .entries()
                .iter()
                .all(|entry| entry.kind() != AuditEventKind::TranslationCompleted));
            assert_eq!(format!("{core:?}"), before);

            let error = core
                .settle_with_sequencer_evidence(&absent, key, &changed, &keys, &trace, now)
                .err()
                .unwrap_or_else(|| {
                    panic!(
                        "{}",
                        "identity mismatch must be refused before creating principal state"
                    )
                });
            assert_eq!(
                error.error(),
                &SequencerSettlementError::Gateway(rejected(ReceiptCheck::ActivityId))
            );
            assert_eq!(error.trace(), &trace);
            assert!(core.principal_audit(&absent).is_none());
            assert_eq!(core.translation(&absent, key), None);
            assert_eq!(format!("{core:?}"), before);
        }

        assert_eq!(
            core.settle_with_sequencer_evidence(&principal, key, &genuine, &keys, &trace, 5)
                .unwrap_or_else(|error| panic!("genuine evidence after refusal: {error}")),
            TranslationStatus::ReceiptVerified {
                receipt_digest: expected_digest
            }
        );
        let audit = core
            .principal_audit(&principal)
            .or_panic("missing completion audit");
        assert_eq!(audit.entries().len(), audit_before.len() + 1);
        let completion = &audit.entries()[audit_before.len()];
        assert_eq!(completion.kind(), AuditEventKind::TranslationCompleted);
        assert_eq!(completion.subject(), expected_digest);
        assert_eq!(completion.trace(), &trace);
    }
}

trait OrPanic {
    type Output;

    fn or_panic(self, context: &str) -> Self::Output;
}

impl<T, E: std::fmt::Debug> OrPanic for std::result::Result<T, E> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}

impl<T> OrPanic for std::option::Option<T> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|| panic!("{context}"))
    }
}
