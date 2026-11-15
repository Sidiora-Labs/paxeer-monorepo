//! Task 22.33.67, audit 22.30.108: evidence-identity is proven by
//! `evidence_identity_rejects_receipted_transaction_hash_mutation` and
//! `evidence_identity_rejects_committed_transaction_hash_mutation`.
//! The committed RPC producer transcript crosses JSON decoding, the public
//! evidence boundary, real signature/log verification and the gateway store.
//! Baseline verification was not run: the bounded gate order requires complete
//! implementation before hf-build/hf-done. No pre-change failures are claimed.
//! The proposal requires no out-of-scope wiring.

use layerx_interop_gateway::adapter::{
    AdapterDescriptor, AdapterId, ConformanceSuite, PinnedSpec, SpecVersion,
};
use layerx_interop_gateway::audit::AuditEventKind;
use layerx_interop_gateway::error::{error_emission, GatewayError};
use layerx_interop_gateway::evidence::{
    canonical_receipt_digest, SequencerEvidence, SettlementRung,
};
use layerx_interop_gateway::gateway::{
    SequencerSettlementError, TranslationKind, TranslationRequest, TranslationStatus,
};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::redaction::{self, FieldValue};
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{
    verify_sequencer_signature, ReceiptCheck, ReceiptKeys, VerificationFailure,
};
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_wire::receipt::encode;
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

fn recorded_payment() -> (SequencerEvidence, [SequencerAuthorization; 1], [u8; 32]) {
    let document: Value =
        serde_json::from_str(FIXTURE).unwrap_or_else(|error| panic!("producer recording: {error}"));
    assert_eq!(
        document["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    let case = document["cases"]
        .as_array()
        .or_panic("missing producer cases")
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .or_panic("missing real payExact output");
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
    let evidence = SequencerEvidence {
        transaction_hash: array(&response["txHash"]),
        signed_receipt: bytes(&response["receipt"]),
        logs,
        rung: SettlementRung::Receipted,
    };
    let pin = &case["pinnedKey"];
    let keys = [SequencerAuthorization::new(
        array(&pin["keyId"]),
        array(&pin["publicKey"]),
        pin["firstHeight"].as_u64().or_panic("first height"),
        pin["lastHeight"].as_u64().or_panic("last height"),
    )];
    (evidence, keys, array(&case["receiptDigest"]))
}

fn pending(key: [u8; 32]) -> (GatewayCore, PrincipalId, TraceId) {
    let mut core = GatewayCore::new();
    let trace = TraceId::mint([0x67; 16]);
    let principal = PrincipalId::new("evidence-identity-buyer")
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

fn assert_identity_binding(rung: SettlementRung) {
    let (mut original, keys, digest) = recorded_payment();
    original.rung = rung;
    assert_recorded_identity(&original, &keys, digest);

    let key = original.logs[0].topics()[1];
    let (mut core, principal, trace) = pending(key);
    let before = format!("{core:?}");
    let mut changed_hash = original.transaction_hash;
    changed_hash[0] ^= 1;
    for transaction_hash in [changed_hash, [0; 32]] {
        let mut mutated = original.clone();
        mutated.transaction_hash = transaction_hash;
        assert_ne!(mutated.transaction_hash, original.transaction_hash);
        assert_eq!(mutated.signed_receipt, original.signed_receipt);
        assert_eq!(mutated.logs, original.logs);
        assert_eq!(mutated.rung, original.rung);
        let error = core
            .settle_with_sequencer_evidence(&principal, key, &mutated, &keys, &trace, 3)
            .err()
            .or_panic("expected Err value");
        let rejection = GatewayError::ReceiptRejected(VerificationFailure {
            check: ReceiptCheck::ActivityId,
        });
        assert_eq!(error.error(), &SequencerSettlementError::Gateway(rejection));
        assert_eq!(error.trace(), &trace);
        assert_eq!(
            core.translation(&principal, key),
            Some(TranslationStatus::Pending)
        );
        assert_eq!(
            format!("{core:?}"),
            before,
            "rejection must write no state or audit"
        );

        let wire = error_emission(&error.trace().wrap(rejection))
            .unwrap_or_else(|error| panic!("error emission: {error}"));
        let response =
            redaction::decode(&wire).unwrap_or_else(|error| panic!("error response: {error}"));
        let [FieldValue::Label(error_code), FieldValue::Label(retriability), FieldValue::Trace(observed)] =
            response.values()
        else {
            panic!("error response shape changed");
        };
        assert_eq!(error_code.as_str(), "receipt-rejected");
        assert_eq!(retriability.as_str(), "retriable");
        assert_eq!(observed, &trace);
    }

    let expected = TranslationStatus::ReceiptVerified {
        receipt_digest: digest,
    };
    assert_eq!(
        core.settle_with_sequencer_evidence(&principal, key, &original, &keys, &trace, 4)
            .unwrap_or_else(|error| panic!("genuine evidence: {error}")),
        expected
    );
    assert_eq!(core.translation(&principal, key), Some(expected));
    let entries = core
        .principal_audit(&principal)
        .or_panic("missing audit")
        .entries();
    assert_eq!(entries.len(), 2);
    assert_eq!(entries[1].kind(), AuditEventKind::TranslationCompleted);
    assert_eq!(entries[1].subject(), digest);
    let settled = format!("{core:?}");
    assert_eq!(
        core.settle_with_sequencer_receipt(
            &principal,
            key,
            &original.signed_receipt,
            original.logs.clone(),
            &keys,
            &trace,
            5,
        )
        .unwrap_or_else(|error| panic!("receipt replay: {error}")),
        expected
    );
    assert_eq!(format!("{core:?}"), settled);

    let mut mutated = original;
    mutated.transaction_hash = changed_hash;
    let error = core
        .settle_with_sequencer_evidence(&principal, key, &mutated, &keys, &trace, 6)
        .err()
        .or_panic("expected Err value");
    assert_eq!(
        error.error(),
        &SequencerSettlementError::Gateway(GatewayError::ReceiptRejected(VerificationFailure {
            check: ReceiptCheck::ActivityId
        }))
    );
    assert_eq!(error.trace(), &trace);
    assert_eq!(
        format!("{core:?}"),
        settled,
        "settled replays must still check identity"
    );
}

fn assert_recorded_identity(
    original: &SequencerEvidence,
    keys: &[SequencerAuthorization],
    digest: [u8; 32],
) {
    let receipt =
        verify_sequencer_signature(&original.signed_receipt, ReceiptKeys::HeightRanges(keys))
            .or_panic("producer signature");
    assert_eq!(
        encode(&receipt).or_panic("canonical encoding"),
        original.signed_receipt
    );
    assert_eq!(
        receipt
            .protocol()
            .or_panic("protocol receipt")
            .activity_id(),
        original.transaction_hash
    );
    let verified = verify_logs(&receipt, original.logs.clone()).or_panic("producer logs");
    assert_eq!(verified.logs(), original.logs);
    assert_eq!(verified.logs().len(), 1);
    assert_eq!(
        canonical_receipt_digest(&original.signed_receipt),
        Ok(digest)
    );
}

#[test]
fn evidence_identity_rejects_receipted_transaction_hash_mutation() {
    assert_identity_binding(SettlementRung::Receipted);
}

#[test]
fn evidence_identity_rejects_committed_transaction_hash_mutation() {
    assert_identity_binding(SettlementRung::Committed {
        block_number: 1001,
        block_hash: [0x42; 32],
    });
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
