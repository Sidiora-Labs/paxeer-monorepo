use layerx_fiat::{
    fiat_adapter_descriptor, EvidenceClass, ExternalId, FiatAdapter, FiatError, FiatJourneyState,
    FiatRail, VerifiedProviderFacts,
};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite, PinnedSpec, SpecVersion};
use layerx_interop_gateway::audit::AuditEventKind;
use layerx_interop_gateway::evidence::{SequencerEvidence, SettlementRung};
use layerx_interop_gateway::gateway::{TranslationKind, TranslationRequest, TranslationStatus};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::sequencer_logs::SequencerLog;
use serde_json::Value;
use sha2::{Digest as _, Sha256};

const FIXTURE: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
));

fn recorded_case() -> Value {
    let fixture: Value = serde_json::from_str(FIXTURE)
        .unwrap_or_else(|error| panic!("recorded sequencer fixture: {error}"));
    fixture["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("fixture cases"))
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .unwrap_or_else(|| panic!("recorded PaymentExact case"))
        .clone()
}

fn hex(value: &Value) -> Vec<u8> {
    let value = value
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .unwrap_or_else(|| panic!("fixture hex string"));
    assert_eq!(value.len() % 2, 0);
    (0..value.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&value[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("fixture hex: {error}"))
        })
        .collect()
}

fn array<const N: usize>(value: &Value) -> [u8; N] {
    hex(value)
        .try_into()
        .unwrap_or_else(|_| panic!("fixture field must have {N} bytes"))
}

fn recorded_evidence(case: &Value) -> SequencerEvidence {
    let logs = case["receiptJson"]["logs"]
        .as_array()
        .unwrap_or_else(|| panic!("recorded logs"))
        .iter()
        .map(|log| {
            let topics = log["topics"]
                .as_array()
                .unwrap_or_else(|| panic!("recorded topics"))
                .iter()
                .map(array)
                .collect();
            let index = log["logIndex"]
                .as_str()
                .and_then(|index| index.strip_prefix("0x"))
                .unwrap_or_else(|| panic!("recorded log index"));
            SequencerLog::new(
                array(&log["address"]),
                topics,
                hex(&log["data"]),
                u64::from_str_radix(index, 16).unwrap_or_else(|error| panic!("log index: {error}")),
            )
        })
        .collect();
    SequencerEvidence {
        transaction_hash: array(&case["receiptJson"]["txHash"]),
        signed_receipt: hex(&case["signedReceipt"]),
        logs,
        rung: SettlementRung::Receipted,
    }
}

fn receipt_keys(case: &Value) -> [SequencerAuthorization; 1] {
    let pin = &case["pinnedKey"];
    [SequencerAuthorization::new(
        array(&pin["keyId"]),
        array(&pin["publicKey"]),
        pin["firstHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("first height")),
        pin["lastHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("last height")),
    )]
}

fn capture_facts(evidence: &SequencerEvidence) -> VerifiedProviderFacts {
    let log = &evidence.logs[0];
    let amount = log.data()[48..64]
        .try_into()
        .unwrap_or_else(|_| panic!("recorded payment amount"));
    VerifiedProviderFacts {
        provider: ExternalId::new("recorded-card-provider")
            .unwrap_or_else(|error| panic!("provider: {error}")),
        settlement: ExternalId::new("recorded-card-capture")
            .unwrap_or_else(|error| panic!("capture: {error}")),
        rail: FiatRail::Card,
        class: EvidenceClass::Settled,
        amount: u128::from_be_bytes(amount),
        asset: log.data()[..32]
            .try_into()
            .unwrap_or_else(|_| panic!("recorded payment asset")),
        destination: log.topics()[2],
        observed_at: 10,
        hold_until: None,
    }
}

fn pending_capture(key: [u8; 32]) -> (GatewayCore, PrincipalId, TraceId) {
    let mut gateway = GatewayCore::new();
    let trace = TraceId::mint([0x18; 16]);
    let principal =
        PrincipalId::new("fiat-card-capture").unwrap_or_else(|error| panic!("principal: {error}"));
    let adapter = AdapterId::new("fiat").unwrap_or_else(|error| panic!("adapter: {error}"));
    let version =
        SpecVersion::parse("1.0.0").unwrap_or_else(|error| panic!("spec version: {error}"));
    let digest: [u8; 32] = Sha256::digest(FIXTURE.as_bytes()).into();
    let spec = PinnedSpec::new(adapter.clone(), version, digest)
        .unwrap_or_else(|error| panic!("pinned spec: {error}"));
    let suite = ConformanceSuite::new(
        AdapterId::new("sequencer-receipt-v1").unwrap_or_else(|error| panic!("suite id: {error}")),
        1,
        digest,
    )
    .unwrap_or_else(|error| panic!("conformance suite: {error}"));
    gateway
        .register_adapter(
            fiat_adapter_descriptor(spec, suite)
                .unwrap_or_else(|error| panic!("fiat descriptor: {error}")),
            &trace,
            10,
        )
        .unwrap_or_else(|error| panic!("register fiat: {error}"));
    let request = TranslationRequest::new(adapter, TranslationKind::StateChanging, key, digest)
        .unwrap_or_else(|error| panic!("capture request: {error}"));
    assert_eq!(
        gateway
            .begin_translation(&principal, &request, &trace, 11)
            .unwrap_or_else(|error| panic!("reserve capture: {error}")),
        TranslationStatus::Pending,
    );
    (gateway, principal, trace)
}

#[test]
fn fiat_card_capture_settles_receipted_evidence() {
    let case = recorded_case();
    let mut evidence = recorded_evidence(&case);
    let facts = capture_facts(&evidence);
    assert_eq!(facts.rail, FiatRail::Card);
    assert_eq!(facts.amount, 1_000_000);
    let key = evidence.logs[0].topics()[3];
    let digest = array(&case["receiptDigest"]);
    let keys = receipt_keys(&case);
    for rung in [
        SettlementRung::Receipted,
        SettlementRung::Committed {
            block_number: 1001,
            block_hash: [0x42; 32],
        },
    ] {
        evidence.rung = rung;
        let (mut gateway, principal, trace) = pending_capture(key);
        let state = FiatAdapter::settle_evidence(
            &mut gateway,
            &principal,
            key,
            &facts,
            &evidence,
            &keys,
            &trace,
            12,
        )
        .unwrap_or_else(|error| panic!("settle card capture: {error}"));
        assert_eq!(
            state,
            FiatJourneyState::Credited {
                receipt_digest: digest
            }
        );
        assert_eq!(
            gateway.translation(&principal, key),
            Some(TranslationStatus::ReceiptVerified {
                receipt_digest: digest
            }),
        );
        let entries = gateway
            .principal_audit(&principal)
            .unwrap_or_else(|| panic!("capture audit"))
            .entries();
        assert_eq!(entries.len(), 2);
        assert_eq!(entries[1].kind(), AuditEventKind::TranslationCompleted);
        assert_eq!(entries[1].subject(), digest);
        let before_replay = format!("{gateway:?}");
        assert_eq!(
            FiatAdapter::settle_evidence(
                &mut gateway,
                &principal,
                key,
                &facts,
                &evidence,
                &keys,
                &trace,
                13,
            )
            .unwrap_or_else(|error| panic!("replay capture: {error}")),
            state,
        );
        assert_eq!(format!("{gateway:?}"), before_replay);
    }

    for mismatch in 0..4 {
        let (mut gateway, principal, trace) = pending_capture(key);
        let mut facts = facts.clone();
        let mut evidence = evidence.clone();
        match mismatch {
            0 => facts.amount += 1,
            1 => facts.asset[0] ^= 1,
            2 => facts.destination[0] ^= 1,
            3 => evidence.transaction_hash[0] ^= 1,
            _ => unreachable!(),
        }
        let before = format!("{gateway:?}");
        let error = FiatAdapter::settle_evidence(
            &mut gateway,
            &principal,
            key,
            &facts,
            &evidence,
            &keys,
            &trace,
            12,
        )
        .unwrap_err();
        assert_eq!(error.error(), &FiatError::ReceiptMismatch);
        assert_eq!(error.trace(), &trace);
        assert_eq!(format!("{gateway:?}"), before);
    }
}

#[test]
fn fiat_submitted_rung_keeps_the_journey_waiting() {
    let case = recorded_case();
    let mut evidence = recorded_evidence(&case);
    let mut facts = capture_facts(&evidence);
    let key = evidence.logs[0].topics()[3];
    evidence.rung = SettlementRung::Submitted;
    evidence.signed_receipt.clear();
    evidence.logs.clear();
    for (class, waiting) in [
        (EvidenceClass::Settled, FiatJourneyState::CreditPending),
        (
            EvidenceClass::Reversed,
            FiatJourneyState::ReversalPending { hold_until: None },
        ),
        (
            EvidenceClass::Chargeback,
            FiatJourneyState::ChargebackPending { hold_until: None },
        ),
    ] {
        facts.class = class;
        let (mut gateway, principal, trace) = pending_capture(key);
        let before = format!("{gateway:?}");
        for now in [12, 13] {
            let state = FiatAdapter::settle_evidence(
                &mut gateway,
                &principal,
                key,
                &facts,
                &evidence,
                &[],
                &trace,
                now,
            )
            .unwrap_or_else(|error| panic!("submitted fiat evidence: {error}"));
            assert_eq!(state, waiting);
            assert_eq!(
                gateway.translation(&principal, key),
                Some(TranslationStatus::Pending)
            );
            assert_eq!(format!("{gateway:?}"), before);
        }
    }
}
