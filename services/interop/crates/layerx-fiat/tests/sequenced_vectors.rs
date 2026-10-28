use ed25519_dalek::{Signature, Signer as _, SigningKey, VerifyingKey};
use layerx_fiat::{
    fiat_adapter_descriptor, EvidenceClass, ExecutedFiatOutcome, ExternalId, FiatAdapter,
    FiatError, FiatIntent, FiatIntentKind, FiatJourneyState, FiatPlane, FiatPlaneResult, FiatRail,
    ProviderEvidence, ProviderVerifier, TokenReference, VerifiedProviderFacts,
};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite, PinnedSpec, SpecVersion};
use layerx_interop_gateway::evidence::{
    canonical_receipt_digest, SequencerEvidence, SettlementRung,
};
use layerx_interop_gateway::gateway::TranslationStatus;
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptKeys};
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const JOURNEYS: &str = include_str!("../../../specs/conformance/fiat/sequenced-journeys.json");
const FIXTURE_PATH: &str = "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json";
const FIXTURE: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
));
const PROVIDER_PUBLIC_KEY: &str =
    "0xc84069fa2216c6edc1d52ca83aa11c267dd5440afa526090ca43ebd0ca86a9cf";
const PROVIDER_DOMAIN: &[u8] = b"LayerX/fiat/sequenced-provider/v1\0";

fn field<'a>(value: &'a Value, name: &str) -> &'a str {
    value[name]
        .as_str()
        .unwrap_or_else(|| panic!("missing string field {name}"))
}

fn number(value: &Value, name: &str) -> u64 {
    value[name]
        .as_u64()
        .unwrap_or_else(|| panic!("missing integer field {name}"))
}

fn hex(value: &str) -> Vec<u8> {
    let value = value
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("hex field must start with 0x"));
    assert!(value.is_ascii());
    assert_eq!(value.len() % 2, 0);
    (0..value.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&value[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("invalid hex field: {error}"))
        })
        .collect()
}

fn array<const N: usize>(value: &str) -> [u8; N] {
    hex(value)
        .try_into()
        .unwrap_or_else(|_| panic!("hex field must contain {N} bytes"))
}

fn rail(value: &str) -> FiatRail {
    match value {
        "card" => FiatRail::Card,
        "bank" => FiatRail::Bank,
        "rtp" => FiatRail::RealTimePayment,
        other => panic!("unknown fiat rail {other}"),
    }
}

fn facts(value: &Value) -> VerifiedProviderFacts {
    assert_eq!(field(value, "class"), "settled");
    assert!(value["hold_until"].is_null());
    VerifiedProviderFacts {
        provider: ExternalId::new(field(value, "provider"))
            .unwrap_or_else(|error| panic!("provider: {error}")),
        settlement: ExternalId::new(field(value, "settlement"))
            .unwrap_or_else(|error| panic!("settlement: {error}")),
        rail: rail(field(value, "rail")),
        class: EvidenceClass::Settled,
        amount: field(value, "amount")
            .parse()
            .unwrap_or_else(|error| panic!("amount: {error}")),
        asset: array(field(value, "asset")),
        destination: array(field(value, "destination")),
        observed_at: number(value, "observed_at"),
        hold_until: None,
    }
}

struct SignedProvider;

impl ProviderVerifier for SignedProvider {
    fn verify(
        &self,
        token: &TokenReference,
        evidence: &ProviderEvidence,
        _trace: &TraceId,
    ) -> Result<VerifiedProviderFacts, FiatError> {
        let envelope: Value =
            serde_json::from_slice(evidence.canonical()).map_err(|_| FiatError::InvalidEvidence)?;
        let payload = &envelope["facts"];
        let canonical = serde_json::to_vec(payload).map_err(|_| FiatError::InvalidEvidence)?;
        let message = [PROVIDER_DOMAIN, &canonical].concat();
        let public = VerifyingKey::from_bytes(&array(PROVIDER_PUBLIC_KEY))
            .map_err(|_| FiatError::InvalidEvidence)?;
        let signature = Signature::from_bytes(&array(field(&envelope, "signature")));
        public
            .verify_strict(&message, &signature)
            .map_err(|_| FiatError::InvalidEvidence)?;
        let token_digest: [u8; 32] = Sha256::digest(token.as_bytes()).into();
        if token_digest != array(field(payload, "token_reference_sha256")) {
            return Err(FiatError::InvalidEvidence);
        }
        Ok(facts(payload))
    }
}

fn recorded_evidence(case: &Value, rung: SettlementRung) -> SequencerEvidence {
    let transaction_hash = array(field(&case["receiptJson"], "txHash"));
    if rung == SettlementRung::Submitted {
        return SequencerEvidence {
            transaction_hash,
            signed_receipt: Vec::new(),
            logs: Vec::new(),
            rung,
        };
    }
    let logs = case["receiptJson"]["logs"]
        .as_array()
        .unwrap_or_else(|| panic!("recorded log list"))
        .iter()
        .map(|log| {
            let topics = log["topics"]
                .as_array()
                .unwrap_or_else(|| panic!("recorded topics"))
                .iter()
                .map(|topic| array(topic.as_str().unwrap_or_else(|| panic!("log topic"))))
                .collect();
            let index = field(log, "logIndex")
                .strip_prefix("0x")
                .unwrap_or_else(|| panic!("log index prefix"));
            SequencerLog::new(
                array(field(log, "address")),
                topics,
                hex(field(log, "data")),
                u64::from_str_radix(index, 16).unwrap_or_else(|error| panic!("log index: {error}")),
            )
        })
        .collect();
    SequencerEvidence {
        transaction_hash,
        signed_receipt: hex(field(case, "signedReceipt")),
        logs,
        rung,
    }
}

fn resigned_payment(case: &Value, declared: &Value, reference: [u8; 32]) -> SequencerEvidence {
    let mut evidence = recorded_evidence(case, SettlementRung::Receipted);
    assert_eq!(
        canonical_receipt_digest(&evidence.signed_receipt)
            .unwrap_or_else(|error| panic!("source receipt digest: {error}")),
        array(field(declared, "source_receipt_digest")),
    );
    let original = &evidence.logs[0];
    assert_eq!(original.topics().len(), 4);
    assert_eq!(reference, array(field(declared, "payment_reference")));
    let mut topics = original.topics().to_vec();
    topics[3] = reference;
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
    let offsets: Vec<_> = unsigned
        .windows(original_body.len())
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == original_body.as_slice()).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "one canonical signed payment log");
    unsigned[offsets[0]..offsets[0] + original_body.len()].copy_from_slice(&body(&payment));
    let seed = array(field(declared, "test_signing_seed"));
    assert_eq!(seed, [0x5e; 32]);
    let signer = SigningKey::from_bytes(&seed);
    assert_eq!(
        signer.verifying_key().to_bytes(),
        array(field(&case["pinnedKey"], "publicKey")),
    );
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    digest.update(&unsigned);
    digest.update([0]);
    let signature = signer.sign(&digest.finalize()).to_bytes();
    unsigned.extend_from_slice(&[1, 0, 0, 0, 64]);
    unsigned.extend_from_slice(&signature);
    evidence.signed_receipt = unsigned;
    evidence.logs = vec![payment];
    evidence
}

struct RecordedSequencer<'a> {
    case: &'a Value,
    declared: &'a Value,
    rung: SettlementRung,
    keys: [SequencerAuthorization; 1],
    expected_intent: FiatIntent,
    calls: usize,
}

impl FiatPlane for RecordedSequencer<'_> {
    fn receipt_keys(&self) -> &[SequencerAuthorization] {
        &self.keys
    }

    fn execute(
        &mut self,
        intent: FiatIntent,
        _trace: &TraceId,
    ) -> Result<FiatPlaneResult, FiatError> {
        assert_eq!(
            intent, self.expected_intent,
            "exact provider-derived intent"
        );
        self.calls += 1;
        let evidence = if self.rung == SettlementRung::Receipted
            && self.case["id"] == "V1-layerxledger-pay-exact"
        {
            resigned_payment(self.case, self.declared, intent.idempotency_key)
        } else {
            recorded_evidence(self.case, self.rung)
        };
        if self.rung != SettlementRung::Submitted {
            let receipt = verify_sequencer_signature(
                &evidence.signed_receipt,
                ReceiptKeys::HeightRanges(&self.keys),
            )
            .unwrap_or_else(|error| panic!("recorded sequencer signature: {error:?}"));
            verify_logs(&receipt, evidence.logs.clone())
                .unwrap_or_else(|error| panic!("recorded sequencer logs: {error:?}"));
            let protocol = receipt
                .protocol()
                .unwrap_or_else(|| panic!("recorded protocol receipt"));
            assert_eq!(protocol.activity_id(), evidence.transaction_hash);
            assert_eq!(
                protocol.result_code() == 0,
                self.case["receiptJson"]["status"] == "0x1"
            );
            assert_eq!(
                canonical_receipt_digest(&evidence.signed_receipt)
                    .unwrap_or_else(|error| panic!("receipt digest: {error}")),
                array(field(self.declared, "receipt_digest")),
            );
        }
        Ok(FiatPlaneResult::Executed(ExecutedFiatOutcome { evidence }))
    }
}

fn registered_gateway(vector_count: usize, trace: &TraceId) -> GatewayCore {
    let mut gateway = GatewayCore::new();
    let digest: [u8; 32] = Sha256::digest(JOURNEYS.as_bytes()).into();
    let adapter = AdapterId::new("fiat").unwrap_or_else(|error| panic!("adapter: {error}"));
    let version = SpecVersion::parse("1.0.0").unwrap_or_else(|error| panic!("version: {error}"));
    let spec = PinnedSpec::new(adapter, version, Sha256::digest(FIXTURE.as_bytes()).into())
        .unwrap_or_else(|error| panic!("spec: {error}"));
    let suite = ConformanceSuite::new(
        AdapterId::new("fiat-sequenced-journeys-v1")
            .unwrap_or_else(|error| panic!("suite identifier: {error}")),
        u64::try_from(vector_count).unwrap_or_else(|error| panic!("vector count: {error}")),
        digest,
    )
    .unwrap_or_else(|error| panic!("suite: {error}"));
    gateway
        .register_adapter(
            fiat_adapter_descriptor(spec, suite)
                .unwrap_or_else(|error| panic!("descriptor: {error}")),
            trace,
            10,
        )
        .unwrap_or_else(|error| panic!("register adapter: {error}"));
    gateway
}

#[test]
fn fiat_sequenced_journey_vectors_replay() {
    let vectors: Vec<Value> = serde_json::from_str(JOURNEYS)
        .unwrap_or_else(|error| panic!("sequenced journey vectors: {error}"));
    let fixture: Value = serde_json::from_str(FIXTURE)
        .unwrap_or_else(|error| panic!("sequencer receipt fixture: {error}"));
    assert_eq!(
        vectors.len(),
        5,
        "three settled rails, submitted and reverted"
    );
    let mut coverage = [0_usize; 5];
    let mut failures = Vec::new();
    for vector in &vectors {
        let name = field(vector, "name");
        let declared = &vector["sequencer"];
        assert_eq!(field(declared, "fixture"), FIXTURE_PATH, "{name}");
        let case = fixture["cases"]
            .as_array()
            .unwrap_or_else(|| panic!("fixture cases"))
            .iter()
            .find(|case| field(case, "id") == field(declared, "case"))
            .unwrap_or_else(|| panic!("{name}: recorded case not found"));
        let rung = match field(declared, "rung") {
            "submitted" => SettlementRung::Submitted,
            "receipted" => SettlementRung::Receipted,
            other => panic!("{name}: unsupported rung {other}"),
        };
        assert_eq!(declared["transaction_hash"], case["receiptJson"]["txHash"]);
        if rung == SettlementRung::Receipted {
            if case["id"] == "V1-layerxledger-pay-exact" {
                assert_eq!(declared["source_receipt_digest"], case["receiptDigest"]);
                assert_ne!(declared["receipt_digest"], case["receiptDigest"]);
            } else {
                assert_eq!(declared["receipt_digest"], case["receiptDigest"]);
            }
        } else {
            assert!(declared["receipt_digest"].is_null());
        }
        assert_eq!(field(vector, "provider_public_key"), PROVIDER_PUBLIC_KEY);
        let facts = facts(&vector["provider_facts"]);
        let token = TokenReference::new(field(vector, "token").as_bytes().to_vec())
            .unwrap_or_else(|error| panic!("{name}: token: {error}"));
        let evidence = ProviderEvidence::new(
            serde_json::to_vec(&json!({
                "facts": vector["provider_facts"],
                "signature": vector["provider_signature"],
            }))
            .unwrap_or_else(|error| panic!("{name}: evidence JSON: {error}")),
        )
        .unwrap_or_else(|error| panic!("{name}: provider evidence: {error}"));
        let trace = TraceId::mint(array(&format!("0x{}", field(vector, "trace"))));
        let principal = PrincipalId::new(field(vector, "principal"))
            .unwrap_or_else(|error| panic!("{name}: principal: {error}"));
        let key = FiatAdapter::idempotency_key(&facts);
        let pin = &case["pinnedKey"];
        let mut plane = RecordedSequencer {
            case,
            declared,
            rung,
            keys: [SequencerAuthorization::new(
                array(field(pin, "keyId")),
                array(field(pin, "publicKey")),
                number(pin, "firstHeight"),
                number(pin, "lastHeight"),
            )],
            expected_intent: FiatIntent {
                kind: FiatIntentKind::Credit,
                provider: facts.provider.clone(),
                settlement: facts.settlement.clone(),
                amount: facts.amount,
                asset: facts.asset,
                destination: facts.destination,
                evidence_digest: evidence.digest(),
                idempotency_key: key,
            },
            calls: 0,
        };
        let expected = &vector["expect"];
        let (expected_outcome, expected_translation, coverage_index) =
            if let Some(refusal) = expected["refusal"].as_str() {
                assert_eq!(refusal, "ReceiptMismatch", "{name}");
                assert_eq!(field(case, "id"), "V2-layerxledger-revert");
                assert_eq!(case["receiptJson"]["status"], "0x0");
                assert!(case["receiptJson"]["logs"]
                    .as_array()
                    .is_some_and(Vec::is_empty));
                (
                    Err(FiatError::ReceiptMismatch),
                    TranslationStatus::Pending,
                    4,
                )
            } else {
                assert_eq!(field(case, "id"), "V1-layerxledger-pay-exact");
                match field(expected, "state") {
                    "credit-pending" => {
                        assert_eq!(rung, SettlementRung::Submitted);
                        (
                            Ok(FiatJourneyState::CreditPending),
                            TranslationStatus::Pending,
                            3,
                        )
                    }
                    "credited" => {
                        assert_eq!(rung, SettlementRung::Receipted);
                        assert_eq!(expected["receipt_digest"], declared["receipt_digest"]);
                        let receipt_digest = array(field(expected, "receipt_digest"));
                        let index = match facts.rail {
                            FiatRail::Card => 0,
                            FiatRail::Bank => 1,
                            FiatRail::RealTimePayment => 2,
                        };
                        (
                            Ok(FiatJourneyState::Credited { receipt_digest }),
                            TranslationStatus::ReceiptVerified { receipt_digest },
                            index,
                        )
                    }
                    other => panic!("{name}: unsupported expected state {other}"),
                }
            };
        assert_eq!(
            field(expected, "translation"),
            if expected_translation == TranslationStatus::Pending {
                "pending"
            } else {
                "receipt-verified"
            },
        );
        let mut gateway = registered_gateway(vectors.len(), &trace);
        for now in [number(vector, "now"), number(vector, "replay_at")] {
            let outcome = FiatAdapter::apply(
                &mut gateway,
                &principal,
                &token,
                &evidence,
                &SignedProvider,
                &mut plane,
                &trace,
                now,
            )
            .map_err(|error| {
                assert_eq!(error.trace(), &trace, "{name}: refusal trace");
                *error.error()
            });
            let translation = gateway.translation(&principal, key);
            if outcome != expected_outcome || translation != Some(expected_translation) {
                failures.push(format!(
                    "{name} at {now}: expected {expected_outcome:?}/{expected_translation:?}, got {outcome:?}/{translation:?}; derived reference {key:02x?}, recorded reference {}",
                    case["receiptJson"]["logs"][0]["topics"][3],
                ));
            }
        }
        let cached = matches!(
            gateway.translation(&principal, key),
            Some(TranslationStatus::ReceiptVerified { .. })
        );
        assert_eq!(
            plane.calls,
            if cached { 1 } else { 2 },
            "{name}: receipt replay calls"
        );
        coverage[coverage_index] += 1;
    }
    assert_eq!(coverage, [1; 5], "every sequenced journey must execute");
    assert!(
        failures.is_empty(),
        "sequenced journey failures:\n{}",
        failures.join("\n")
    );
}
