use layerx_interop_gateway::adapter::{
    AdapterDescriptor, AdapterId, ConformanceSuite, PinnedSpec, SpecVersion,
};
use layerx_interop_gateway::audit::AuditEventKind;
use layerx_interop_gateway::error::GatewayError;
use layerx_interop_gateway::evidence::{SequencerEvidence, SettlementRung};
use layerx_interop_gateway::gateway::{
    SequencerSettlementError, TranslationKind, TranslationRequest, TranslationStatus,
};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::sequencer_logs::SequencerLog;

const FIXTURE: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
));

fn fixture_case() -> &'static str {
    FIXTURE
        .split_once("\"id\": \"V1-layerxledger-pay-exact\"")
        .and_then(|(_, rest)| rest.split("\"id\":").next())
        .unwrap_or_else(|| panic!("missing recorded pay-exact case"))
}

fn field<'a>(case: &'a str, name: &str) -> &'a str {
    case.split_once(&format!("\"{name}\": \""))
        .and_then(|(_, rest)| rest.split_once('"'))
        .map(|(value, _)| value)
        .unwrap_or_else(|| panic!("missing fixture field {name}"))
}

fn hex(value: &str) -> Vec<u8> {
    let value = value
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("fixture value is not hex"));
    assert_eq!(value.len() % 2, 0);
    (0..value.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&value[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("invalid fixture hex: {error}"))
        })
        .collect()
}

fn array<const N: usize>(value: &str) -> [u8; N] {
    hex(value)
        .try_into()
        .unwrap_or_else(|_| panic!("fixture value must have {N} bytes"))
}

fn recorded_evidence() -> SequencerEvidence {
    let case = fixture_case();
    let topics = case
        .split_once("\"topics\": [")
        .and_then(|(_, rest)| rest.split_once(']'))
        .map(|(topics, _)| topics)
        .unwrap_or_else(|| panic!("missing fixture topics"))
        .split('"')
        .filter(|value| value.starts_with("0x"))
        .map(array)
        .collect();
    let log_index = u64::from_str_radix(
        field(case, "logIndex")
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("invalid log index")),
        16,
    )
    .unwrap_or_else(|error| panic!("invalid log index: {error}"));
    SequencerEvidence {
        transaction_hash: array(field(case, "txHash")),
        signed_receipt: hex(field(case, "signedReceipt")),
        logs: vec![SequencerLog::new(
            array(field(case, "address")),
            topics,
            hex(field(case, "data")),
            log_index,
        )],
        rung: SettlementRung::Receipted,
    }
}

fn receipt_keys() -> [SequencerAuthorization; 1] {
    let case = fixture_case();
    let bound = |name: &str| {
        case.split_once(&format!("\"{name}\": "))
            .and_then(|(_, rest)| rest.split(|c: char| !c.is_ascii_digit()).next())
            .and_then(|value| value.parse().ok())
            .unwrap_or_else(|| panic!("missing fixture bound {name}"))
    };
    [SequencerAuthorization::new(
        array(field(case, "keyId")),
        array(field(case, "publicKey")),
        bound("firstHeight"),
        bound("lastHeight"),
    )]
}

fn pending_translation(key: [u8; 32]) -> (GatewayCore, PrincipalId, TraceId) {
    let mut core = GatewayCore::new();
    let trace = TraceId::mint([0x11; 16]);
    let principal = PrincipalId::new("buyer").unwrap_or_else(|error| panic!("principal: {error}"));
    let adapter = AdapterId::new("x402").unwrap_or_else(|error| panic!("adapter: {error}"));
    let version = SpecVersion::parse("2.0.0").unwrap_or_else(|error| panic!("version: {error}"));
    let spec = PinnedSpec::new(adapter.clone(), version, [0x51; 32])
        .unwrap_or_else(|error| panic!("spec: {error}"));
    let suite_id =
        AdapterId::new("x402-v2-vectors").unwrap_or_else(|error| panic!("suite id: {error}"));
    let suite = ConformanceSuite::new(suite_id, 96, [0x52; 32])
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

fn assert_recorded(core: &GatewayCore, principal: &PrincipalId, key: [u8; 32], digest: [u8; 32]) {
    assert_eq!(
        core.translation(principal, key),
        Some(TranslationStatus::ReceiptVerified {
            receipt_digest: digest
        })
    );
    let entries = core
        .principal_audit(principal)
        .unwrap_or_else(|| panic!("missing audit chain"))
        .entries();
    assert_eq!(entries.len(), 2);
    assert_eq!(entries[1].kind(), AuditEventKind::TranslationCompleted);
    assert_eq!(entries[1].subject(), digest);
}

#[test]
fn receipted_evidence_records_the_canonical_receipt_digest() {
    let mut evidence = recorded_evidence();
    let key = evidence.logs[0].topics()[3];
    let digest = array(field(fixture_case(), "receiptDigest"));
    for rung in [
        SettlementRung::Receipted,
        SettlementRung::Committed {
            block_number: 1001,
            block_hash: [0x42; 32],
        },
    ] {
        evidence.rung = rung;
        let (mut core, principal, trace) = pending_translation(key);
        let status = core
            .settle_with_sequencer_evidence(&principal, key, &evidence, &receipt_keys(), &trace, 3)
            .unwrap_or_else(|error| panic!("settle evidence: {error}"));
        assert_eq!(
            status,
            TranslationStatus::ReceiptVerified {
                receipt_digest: digest
            }
        );
        assert_recorded(&core, &principal, key, digest);
    }
}

#[test]
fn submitted_evidence_leaves_the_translation_pending() {
    let mut evidence = recorded_evidence();
    let key = evidence.logs[0].topics()[3];
    evidence.rung = SettlementRung::Submitted;
    evidence.signed_receipt.clear();
    evidence.logs.clear();
    let (mut core, principal, trace) = pending_translation(key);
    let before = format!("{core:?}");
    let error = core
        .settle_with_sequencer_evidence(&principal, key, &evidence, &[], &trace, 3)
        .unwrap_err();
    assert_eq!(
        error.error(),
        &SequencerSettlementError::Gateway(GatewayError::ReceiptPending)
    );
    assert_eq!(error.trace(), &trace);
    assert_eq!(
        core.translation(&principal, key),
        Some(TranslationStatus::Pending)
    );
    assert_eq!(
        format!("{core:?}"),
        before,
        "submitted evidence must write no state or audit entry"
    );

    let absent = PrincipalId::new("absent").unwrap_or_else(|error| panic!("principal: {error}"));
    let error = core
        .settle_with_sequencer_evidence(&absent, key, &evidence, &[], &trace, 4)
        .unwrap_err();
    assert_eq!(
        error.error(),
        &SequencerSettlementError::Gateway(GatewayError::ReceiptPending)
    );
    assert!(core.principal_audit(&absent).is_none());
    assert_eq!(format!("{core:?}"), before);
}

#[test]
fn receipt_and_evidence_paths_agree_on_the_digest() {
    let evidence = recorded_evidence();
    let key = evidence.logs[0].topics()[3];
    let digest = array(field(fixture_case(), "receiptDigest"));
    let (mut receipt_core, principal, trace) = pending_translation(key);
    let (mut evidence_core, _, _) = pending_translation(key);
    let receipt_status = receipt_core
        .settle_with_sequencer_receipt(
            &principal,
            key,
            &evidence.signed_receipt,
            evidence.logs.clone(),
            &receipt_keys(),
            &trace,
            3,
        )
        .unwrap_or_else(|error| panic!("settle receipt: {error}"));
    let evidence_status = evidence_core
        .settle_with_sequencer_evidence(&principal, key, &evidence, &receipt_keys(), &trace, 3)
        .unwrap_or_else(|error| panic!("settle evidence: {error}"));
    assert_eq!(receipt_status, evidence_status);
    assert_recorded(&receipt_core, &principal, key, digest);
    assert_recorded(&evidence_core, &principal, key, digest);
    let before = format!("{receipt_core:?}");
    let replay = receipt_core
        .settle_with_sequencer_evidence(&principal, key, &evidence, &receipt_keys(), &trace, 4)
        .unwrap_or_else(|error| panic!("replay evidence: {error}"));
    assert_eq!(replay, receipt_status);
    assert_eq!(format!("{receipt_core:?}"), before);
}
