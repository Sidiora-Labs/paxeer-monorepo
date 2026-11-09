use crate::error::{ApiError, ApiSuccess, ErrorClass, ReasonCode, RequestId, RungStatus};
use crate::recorded_commit::{authenticate, document};
use crate::write_contract::{
    EvidenceRef, ReceiptRef, SubmissionRef, SubmissionState, TrackedSubmission, WaitRequest,
    WaitResult,
};
use crate::TimestampSeconds;
use layerx_types::json::JsonValue;
use layerx_types::result::{KnownResult, ResultCode, Retriability};
use layerx_types::rung::Rung;

#[test]
pub fn exact_retry_codes_cross_the_real_api_consumer() {
    for known in KnownResult::ALL {
        let producer = ResultCode::from(*known);
        for _ in 0..2 {
            let transported = ResultCode::from_wire_bytes(producer.to_wire_bytes());
            let error = ApiError::core_rejection(
                RequestId(51),
                transported,
                ReasonCode::new("core.rejected")
                    .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
            );
            let canonical: Retriability = error.retriability;
            assert_eq!(error.class, ErrorClass::CoreRejection);
            assert_eq!(error.protocol_result_code, Some(producer));
            assert_eq!(canonical, producer.retriability());
            assert_eq!(error.request_id, RequestId(51));
        }
    }
    for raw in [i32::MIN + 51, i32::MAX, -999_999, 999_999] {
        let unknown = ResultCode::from_raw(raw);
        assert!(unknown.known().is_none());
        let error = ApiError::core_rejection(
            RequestId(52),
            ResultCode::from_wire_bytes(unknown.to_wire_bytes()),
            ReasonCode::new("core.unknown")
                .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
        );
        assert_eq!(error.protocol_result_code, Some(unknown));
        assert_eq!(error.retriability, Retriability::Terminal);
    }
    assert!(ReasonCode::new("malformed reason").is_err());
    assert_eq!(
        ApiError::idempotent_replay(RequestId(53)).retriability,
        Retriability::Terminal
    );
}

#[test]
pub fn current_api_status_refuses_impossible_shortfalls() {
    let rungs = [Rung::Submitted, Rung::Receipted, Rung::Committed];
    for requested in rungs {
        for achieved in rungs {
            for _ in 0..2 {
                let transported = Rung::from_name(achieved.name())
                    .unwrap_or_else(|| panic!("current rung must decode"));
                assert_eq!(transported, achieved);
                let status = RungStatus::Below {
                    requested,
                    achieved: transported,
                    reason: ReasonCode::new("verification.below").unwrap_or_else(|error| {
                        panic!("test or reference input rejected: {error:?}")
                    }),
                };
                assert_eq!(status.validate().is_ok(), achieved < requested);
                assert_eq!(RungStatus::Achieved(transported).validate(), Ok(()));
            }
        }
    }
}

#[test]
pub fn committed_block_and_transaction_witness_back_current_tracking() {
    for _ in 0..2 {
        // Actual node output crosses the shared strict JSON/hex codec. Only
        // authenticated header/commit and inclusion witnesses justify the rung.
        let produced = document();
        let verified = authenticate(&produced)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "recorded committed evidence"));
        let receipt_ref = ReceiptRef::new(format!("receipt:{}", hex(&verified.transaction_id)))
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"));
        let submission_ref = SubmissionRef::new(hex(&verified.transaction_id))
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"));
        let request = WaitRequest {
            submission_ref: submission_ref.clone(),
            requested_rung: Rung::Committed,
            deadline: TimestampSeconds(1_000),
        };
        let signed = &verified.signed_receipt;
        assert_eq!(
            signed.as_bytes(),
            produced
                .hex_at("bundle.signedReceipt")
                .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
        );
        let tracking = TrackedSubmission {
            submission_ref,
            state: SubmissionState::Committed {
                receipt_ref: receipt_ref.clone(),
            },
            evidence: vec![
                EvidenceRef {
                    kind: "sequencer_receipt".into(),
                    digest: verified.receipt_digest,
                },
                EvidenceRef {
                    kind: "committed_block".into(),
                    digest: verified.header_digest,
                },
            ],
            rung: Rung::Committed,
            transitions: vec![],
        };
        let observed = ApiSuccess {
            request_id: RequestId(54),
            value: WaitResult {
                submission: tracking,
                actual_rung: Rung::Committed,
                deadline_elapsed: false,
            },
            verification_status: RungStatus::Achieved(Rung::Committed),
        };
        assert_eq!(observed.verification_status.validate(), Ok(()));
        assert_eq!(
            observed.value.submission.submission_ref,
            request.submission_ref
        );
        assert_eq!(
            observed.value.submission.state,
            SubmissionState::Committed { receipt_ref }
        );
        assert_eq!(observed.value.submission.state.name(), "committed");
        assert_eq!(observed.value.actual_rung, request.requested_rung);
        assert_eq!(observed.value.submission.rung, observed.value.actual_rung);
        assert!(!observed.value.deadline_elapsed);
        assert_eq!(
            observed.value.submission.evidence[0].digest,
            produced
                .hex_array_at::<32>("bundle.receiptDigest")
                .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
        );
        assert_eq!(
            observed.value.submission.evidence[1].digest,
            produced
                .hex_array_at::<32>("executionBlockHash")
                .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
        );
        assert_eq!(verified.height, 10);
        assert_eq!(verified.result, KnownResult::Ok.into());
    }
}

fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    let mut encoded = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        write!(&mut encoded, "{byte:02x}").unwrap_or_else(|error| panic!("hex encoding: {error}"));
    }
    encoded
}

fn replace(doc: &mut JsonValue, path: &str, replacement: JsonValue) {
    let (name, rest) = path
        .split_once('.')
        .map_or((path, None), |(name, rest)| (name, Some(rest)));
    let JsonValue::Object(fields) = doc else {
        panic!("object at {path}")
    };
    let (_, value) = fields
        .iter_mut()
        .find(|(key, _)| key == name)
        .unwrap_or_else(|| panic!("recorded field must exist"));
    if let Some(rest) = rest {
        replace(value, rest, replacement);
    } else {
        *value = replacement;
    }
}

#[test]
pub fn committed_evidence_refuses_tampering() {
    for path in [
        "bundle.blockHeader",
        "bundle.commit",
        "executionCommit",
        "bundle.signedReceipt",
        "bundle.receiptDigest",
        "bundle.receiptKey.publicKey",
        "batchTransaction",
        "transaction",
        "bundle.rawTransaction",
    ] {
        let mut recording = document();
        let mut bytes = recording
            .hex_at(path)
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"));
        let last = bytes.len() - 1;
        bytes[last] ^= 1;
        replace(
            &mut recording,
            path,
            JsonValue::String(format!("0x{}", hex(&bytes))),
        );
        assert!(
            authenticate(&recording).is_err(),
            "tampered {path} must not authorize current tracking"
        );
    }
    let mut insufficient = document();
    let commit = insufficient
        .field("insufficientCommit")
        .unwrap_or_else(|| panic!("test or reference input rejected"))
        .clone();
    replace(&mut insufficient, "bundle.commit", commit);
    assert!(
        authenticate(&insufficient).is_err(),
        "two thirds is insufficient"
    );
    let mut witness = document();
    replace(
        &mut witness,
        "transactionProof.index",
        JsonValue::String("0".into()),
    );
    assert!(authenticate(&witness).is_err(), "wrong inclusion position");
    for label in ["checkpoint-finalised", "settlement-anchored", "4", "5"] {
        assert_eq!(Rung::from_name(label), None);
    }
    // A legacy budget proof contains an ABI call result and a numeric label,
    // not a native record with a committed storage witness. The actual current
    // budget codec must refuse it; it is never projected as proven state.
    let legacy = layerx_types::json::parse(include_str!(
        "../../../../../../tests/vectors/budget-proof-v2/export.json"
    ))
    .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"));
    let native = layerx_types::json::decode_hex(&format!(
        "0x{}",
        legacy
            .str_at("budget_state.canonical_core_bytes")
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
    ))
    .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"));
    assert_eq!(
        crate::budget::ProtocolBudgetRecord::decode(&native),
        Err(crate::budget::ReconcileError::ProtocolStateSchemaUnavailable)
    );
}
