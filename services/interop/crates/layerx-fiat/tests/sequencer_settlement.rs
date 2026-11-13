use ed25519_dalek::{Signer as _, SigningKey};
use layerx_fiat::{
    fiat_adapter_descriptor, EvidenceClass, ExternalId, FiatAdapter, FiatError, FiatJourneyState,
    FiatRail, VerifiedProviderFacts,
};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite, PinnedSpec, SpecVersion};
use layerx_interop_gateway::audit::AuditEventKind;
use layerx_interop_gateway::evidence::{
    canonical_receipt_digest, SequencerEvidence, SettlementRung,
};
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
        .or_panic("fixture cases")
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .or_panic("recorded PaymentExact case")
        .clone()
}

fn hex(value: &Value) -> Vec<u8> {
    let value = value
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .or_panic("fixture hex string");
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

fn recorded_payment_topics(case: &Value) -> [[u8; 32]; 3] {
    let transaction = hex(&case["rawTransaction"]);
    let offsets: Vec<_> = transaction
        .windows(4)
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == [0x78, 0xea, 0x11, 0xdb]).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "one recorded payExact calldata selector");
    let start = offsets[0] + 4;
    [5, 0, 1].map(|word| {
        transaction[start + word * 32..start + (word + 1) * 32]
            .try_into()
            .or_panic("recorded payExact calldata word")
    })
}

fn recorded_evidence(case: &Value) -> SequencerEvidence {
    let logs = case["receiptJson"]["logs"]
        .as_array()
        .or_panic("recorded logs")
        .iter()
        .map(|log| {
            let topics = log["topics"]
                .as_array()
                .or_panic("recorded topics")
                .iter()
                .map(array)
                .collect();
            let index = log["logIndex"]
                .as_str()
                .and_then(|index| index.strip_prefix("0x"))
                .or_panic("recorded log index");
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
        pin["firstHeight"].as_u64().or_panic("first height"),
        pin["lastHeight"].as_u64().or_panic("last height"),
    )]
}

fn capture_facts(case: &Value, evidence: &SequencerEvidence) -> VerifiedProviderFacts {
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
        destination: recorded_payment_topics(case)[2],
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

fn signed_payment(
    case: &Value,
    transaction_hash: [u8; 32],
    from: [u8; 32],
    to: [u8; 32],
    reference: [u8; 32],
) -> SequencerEvidence {
    let mut evidence = recorded_evidence(case);
    let original = &evidence.logs[0];
    let mut topics = original.topics().to_vec();
    topics[1] = reference;
    topics[2] = from;
    topics[3] = to;
    let payment = SequencerLog::new(original.address(), topics, original.data().to_vec(), 0);
    let body = |log: &SequencerLog| {
        let mut bytes = log.address().to_vec();
        bytes.push(4);
        for topic in log.topics() {
            bytes.extend_from_slice(topic);
        }
        bytes.extend_from_slice(&Sha256::digest(log.data()));
        bytes
    };
    let original_body = body(original);
    let mut unsigned = evidence.signed_receipt[..evidence.signed_receipt.len() - 69].to_vec();
    for (old, new) in [
        (
            evidence.transaction_hash.to_vec(),
            transaction_hash.to_vec(),
        ),
        (original_body, body(&payment)),
    ] {
        let offsets: Vec<_> = unsigned
            .windows(old.len())
            .enumerate()
            .filter_map(|(offset, bytes)| (bytes == old.as_slice()).then_some(offset))
            .collect();
        assert_eq!(offsets.len(), 1, "one canonical signed field");
        unsigned[offsets[0]..offsets[0] + old.len()].copy_from_slice(&new);
    }
    let signer = SigningKey::from_bytes(&[0x5e; 32]);
    assert_eq!(
        signer.verifying_key().to_bytes(),
        array(&case["pinnedKey"]["publicKey"])
    );
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    digest.update(&unsigned);
    digest.update([0]);
    let signature = signer.sign(&digest.finalize()).to_bytes();
    unsigned.extend_from_slice(&[1, 0, 0, 0, 64]);
    unsigned.extend_from_slice(&signature);
    evidence.transaction_hash = transaction_hash;
    evidence.signed_receipt = unsigned;
    evidence.logs = vec![payment];
    evidence
}

#[test]
fn fiat_reversal_on_the_capture_receipt_is_refused() {
    let case = recorded_case();
    let original = recorded_evidence(&case);
    let facts = capture_facts(&case, &original);
    let payer = recorded_payment_topics(&case)[1];
    let capture_key = FiatAdapter::idempotency_key(&facts);
    let capture = signed_payment(
        &case,
        original.transaction_hash,
        payer,
        facts.destination,
        capture_key,
    );
    let capture_digest =
        canonical_receipt_digest(&capture.signed_receipt).or_panic("expected Ok value");
    let keys = receipt_keys(&case);

    let fixture = RefundFixture {
        case: &case,
        facts: &facts,
        capture: &capture,
        capture_key,
        capture_digest,
        payer,
        keys: &keys,
    };
    for class in [EvidenceClass::Reversed, EvidenceClass::Chargeback] {
        for rung in [
            SettlementRung::Receipted,
            SettlementRung::Committed {
                block_number: 1001,
                block_hash: [0x42; 32],
            },
        ] {
            let mut trial = RefundTrial::new(&fixture, class, rung);
            trial.assert_invalid_refunds();
            trial.assert_missing_capture();
            trial.assert_invalid_captures();
            trial.assert_settled();
        }
    }
}

#[derive(Clone, Copy)]
struct RefundFixture<'a> {
    case: &'a Value,
    facts: &'a VerifiedProviderFacts,
    capture: &'a SequencerEvidence,
    capture_key: [u8; 32],
    capture_digest: [u8; 32],
    payer: [u8; 32],
    keys: &'a [SequencerAuthorization; 1],
}

struct RefundTrial<'a> {
    fixture: &'a RefundFixture<'a>,
    gateway: GatewayCore,
    principal: PrincipalId,
    trace: TraceId,
    refund_facts: VerifiedProviderFacts,
    refund_key: [u8; 32],
    refund: SequencerEvidence,
    before: String,
    class: EvidenceClass,
    rung: SettlementRung,
}

impl<'a> RefundTrial<'a> {
    fn new(fixture: &'a RefundFixture<'a>, class: EvidenceClass, rung: SettlementRung) -> Self {
        let RefundFixture {
            case,
            facts,
            capture,
            capture_key,
            capture_digest,
            payer,
            keys,
        } = *fixture;
        let (mut gateway, principal, trace) = pending_capture(capture_key);
        assert_eq!(
            FiatAdapter::settle_evidence(
                &mut gateway,
                &principal,
                capture_key,
                facts,
                capture,
                keys,
                &trace,
                12,
            )
            .or_panic("expected Ok value"),
            FiatJourneyState::Credited {
                receipt_digest: capture_digest
            }
        );
        let mut refund_facts = (*facts).clone();
        refund_facts.class = class;
        let refund_key = FiatAdapter::idempotency_key(&refund_facts);
        let mut refund = signed_payment(case, [0x74; 32], facts.destination, payer, refund_key);
        refund.rung = rung;
        let request = TranslationRequest::new(
            AdapterId::new("fiat").or_panic("expected Ok value"),
            TranslationKind::StateChanging,
            refund_key,
            [0x29; 32],
        )
        .or_panic("expected Ok value");
        assert_eq!(
            gateway
                .begin_translation(&principal, &request, &trace, 13)
                .or_panic("expected Ok value"),
            TranslationStatus::Pending
        );
        let before = format!("{gateway:?}");
        Self {
            fixture,
            gateway,
            principal,
            trace,
            refund_facts,
            refund_key,
            refund,
            before,
            class,
            rung,
        }
    }

    fn assert_invalid_refunds(&mut self) {
        let case = self.fixture.case;
        let facts = self.fixture.facts;
        let capture = self.fixture.capture;
        let capture_key = self.fixture.capture_key;
        let capture_digest = self.fixture.capture_digest;
        let payer = self.fixture.payer;
        let keys = self.fixture.keys;
        let gateway = &mut self.gateway;
        let principal = &self.principal;
        let trace = &self.trace;
        let refund_facts = &self.refund_facts;
        let refund_key = self.refund_key;
        let before = self.before.clone();
        let rung = self.rung;
        let mut same_transaction = signed_payment(
            case,
            capture.transaction_hash,
            facts.destination,
            payer,
            refund_key,
        );
        same_transaction.rung = rung;
        assert_ne!(
            canonical_receipt_digest(&same_transaction.signed_receipt)
                .or_panic("expected Ok value"),
            capture_digest
        );
        let wrong_payer =
            signed_payment(case, [0x74; 32], facts.destination, [0x37; 32], refund_key);
        let wrong_sender = signed_payment(case, [0x74; 32], [0x37; 32], payer, refund_key);
        let wrong_reference =
            signed_payment(case, [0x74; 32], facts.destination, payer, capture_key);
        for invalid in [
            capture,
            &same_transaction,
            &wrong_payer,
            &wrong_sender,
            &wrong_reference,
        ] {
            let error = FiatAdapter::settle_refund_evidence(
                gateway,
                principal,
                refund_key,
                refund_facts,
                invalid,
                capture,
                keys,
                trace,
                14,
            )
            .err()
            .or_panic("expected Err value");
            assert_eq!(error.error(), &FiatError::ReceiptMismatch);
            assert_eq!(error.trace(), trace);
            assert_eq!(format!("{gateway:?}"), before);
            assert_eq!(
                gateway.translation(principal, refund_key),
                Some(TranslationStatus::Pending)
            );
        }
    }

    fn assert_missing_capture(&mut self) {
        let capture = self.fixture.capture;
        let keys = self.fixture.keys;
        let gateway = &mut self.gateway;
        let principal = &self.principal;
        let trace = &self.trace;
        let refund_facts = &self.refund_facts;
        let refund_key = self.refund_key;
        let refund = &self.refund;
        let before = self.before.clone();
        let missing = FiatAdapter::settle_evidence(
            gateway,
            principal,
            refund_key,
            refund_facts,
            refund,
            keys,
            trace,
            14,
        )
        .err()
        .or_panic("expected Err value");
        assert_eq!(missing.error(), &FiatError::ReceiptMismatch);
        assert_eq!(format!("{gateway:?}"), before);

        let (mut uncaptured_gateway, uncaptured_principal, uncaptured_trace) =
            pending_capture(refund_key);
        let uncaptured_before = format!("{uncaptured_gateway:?}");
        let error = FiatAdapter::settle_refund_evidence(
            &mut uncaptured_gateway,
            &uncaptured_principal,
            refund_key,
            refund_facts,
            refund,
            capture,
            keys,
            &uncaptured_trace,
            14,
        )
        .err()
        .or_panic("expected Err value");
        assert_eq!(error.error(), &FiatError::ReceiptMismatch);
        assert_eq!(format!("{uncaptured_gateway:?}"), uncaptured_before);
    }

    fn assert_invalid_captures(&mut self) {
        let case = self.fixture.case;
        let facts = self.fixture.facts;
        let capture = self.fixture.capture;
        let capture_key = self.fixture.capture_key;
        let payer = self.fixture.payer;
        let keys = self.fixture.keys;
        let gateway = &mut self.gateway;
        let principal = &self.principal;
        let trace = &self.trace;
        let refund_facts = &self.refund_facts;
        let refund_key = self.refund_key;
        let refund = &self.refund;
        let before = self.before.clone();
        let mut bad_signature = capture.clone();
        *bad_signature
            .signed_receipt
            .last_mut()
            .or_panic("expected Some value") ^= 1;
        let mut bad_hash = capture.clone();
        bad_hash.transaction_hash[0] ^= 1;
        let mut bad_logs = capture.clone();
        bad_logs.logs = refund.logs.clone();
        let mut submitted = capture.clone();
        submitted.rung = SettlementRung::Submitted;
        let other_capture = signed_payment(case, [0x49; 32], payer, facts.destination, capture_key);
        for invalid_capture in [
            &bad_signature,
            &bad_hash,
            &bad_logs,
            &submitted,
            &other_capture,
        ] {
            let error = FiatAdapter::settle_refund_evidence(
                gateway,
                principal,
                refund_key,
                refund_facts,
                refund,
                invalid_capture,
                keys,
                trace,
                14,
            )
            .err()
            .or_panic("expected Err value");
            assert_eq!(error.error(), &FiatError::ReceiptMismatch);
            assert_eq!(format!("{gateway:?}"), before);
        }
    }

    fn assert_settled(&mut self) {
        let capture = self.fixture.capture;
        let capture_key = self.fixture.capture_key;
        let capture_digest = self.fixture.capture_digest;
        let keys = self.fixture.keys;
        let gateway = &mut self.gateway;
        let principal = &self.principal;
        let trace = &self.trace;
        let refund_facts = &self.refund_facts;
        let refund_key = self.refund_key;
        let refund = &self.refund;
        let class = self.class;
        let digest = canonical_receipt_digest(&refund.signed_receipt).or_panic("expected Ok value");
        let expected = match class {
            EvidenceClass::Reversed => FiatJourneyState::Reversed {
                receipt_digest: digest,
            },
            EvidenceClass::Chargeback => FiatJourneyState::ChargedBack {
                receipt_digest: digest,
            },
            _ => unreachable!(),
        };
        assert_eq!(
            FiatAdapter::settle_refund_evidence(
                gateway,
                principal,
                refund_key,
                refund_facts,
                refund,
                capture,
                keys,
                trace,
                15,
            )
            .or_panic("expected Ok value"),
            expected
        );
        assert_eq!(
            gateway.translation(principal, refund_key),
            Some(TranslationStatus::ReceiptVerified {
                receipt_digest: digest
            })
        );
        assert_eq!(
            gateway.translation(principal, capture_key),
            Some(TranslationStatus::ReceiptVerified {
                receipt_digest: capture_digest
            })
        );
        let settled = format!("{gateway:?}");
        assert_eq!(
            FiatAdapter::settle_refund_evidence(
                gateway,
                principal,
                refund_key,
                refund_facts,
                refund,
                capture,
                keys,
                trace,
                16,
            )
            .or_panic("expected Ok value"),
            expected
        );
        assert_eq!(format!("{gateway:?}"), settled);
    }
}

fn assert_capture_audit(gateway: &GatewayCore, principal: &PrincipalId, digest: [u8; 32]) {
    let entries = gateway
        .principal_audit(principal)
        .or_panic("capture audit")
        .entries();
    assert_eq!(entries.len(), 2);
    assert_eq!(entries[1].kind(), AuditEventKind::TranslationCompleted);
    assert_eq!(entries[1].subject(), digest);
}

#[test]
fn fiat_card_capture_settles_receipted_evidence() {
    let case = recorded_case();
    let recorded = recorded_evidence(&case);
    let facts = capture_facts(&case, &recorded);
    assert_eq!(facts.rail, FiatRail::Card);
    assert_eq!(facts.amount, 1_000_000);
    let [key, from, to] = recorded_payment_topics(&case);
    let mut evidence = signed_payment(&case, recorded.transaction_hash, from, to, key);
    assert_eq!(&evidence.logs[0].topics()[1..], &[key, from, to]);
    let digest = canonical_receipt_digest(&evidence.signed_receipt).or_panic("expected Ok value");
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
        assert_capture_audit(&gateway, &principal, digest);
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

    for mismatch in 0..5 {
        let (mut gateway, principal, trace) = pending_capture(key);
        let mut facts = facts.clone();
        let mut evidence = evidence.clone();
        match mismatch {
            0 => facts.amount += 1,
            1 => facts.asset[0] ^= 1,
            2 => facts.destination[0] ^= 1,
            3 => evidence.transaction_hash[0] ^= 1,
            4 => {
                let mut wrong_reference = key;
                wrong_reference[0] ^= 1;
                evidence =
                    signed_payment(&case, recorded.transaction_hash, from, to, wrong_reference);
            }
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
        .err()
        .or_panic("expected Err value");
        assert_eq!(error.error(), &FiatError::ReceiptMismatch);
        assert_eq!(error.trace(), &trace);
        assert_eq!(format!("{gateway:?}"), before);
    }
}

#[test]
fn fiat_submitted_rung_keeps_the_journey_waiting() {
    let case = recorded_case();
    let mut evidence = recorded_evidence(&case);
    let mut facts = capture_facts(&case, &evidence);
    let key = recorded_payment_topics(&case)[0];
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

#[path = "support/producer_path.rs"]
mod producer_path;

#[test]
fn fiat_settlement_path_uses_unchanged_producer_evidence() -> Result<(), Box<dyn std::error::Error>>
{
    producer_path::fiat_settlement_path()
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
