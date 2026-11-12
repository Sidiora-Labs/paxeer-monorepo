use ed25519_dalek::{Signature, VerifyingKey};
use layerx_fiat::{
    fiat_adapter_descriptor, EvidenceClass, ExecutedFiatOutcome, ExternalId, FiatAdapter,
    FiatError, FiatIntent, FiatIntentKind, FiatJourneyState, FiatPlane, FiatPlaneResult, FiatRail,
    ProviderEvidence, ProviderVerifier, TokenReference, VerifiedProviderFacts,
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
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use serde_json::{json, Value};

#[path = "payment_node.rs"]
mod payment_node;
use payment_node::{PaymentNode, EXPIRES_AT, PAYER};
use sha2::{Digest as _, Sha256};

type TestResult = Result<(), Box<dyn std::error::Error>>;

const JOURNEYS: &str = include_str!("../../../../specs/conformance/fiat/sequenced-journeys.json");

fn bytes(value: &Value) -> Vec<u8> {
    let text = value
        .as_str()
        .unwrap_or_else(|| panic!("expected Some value"))
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("expected Some value"));
    assert!(text.is_ascii());
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|offset| {
            u8::from_str_radix(&text[offset..offset + 2], 16)
                .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"))
        })
        .collect()
}

fn array<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value)
        .try_into()
        .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"))
}

fn journeys() -> Vec<Value> {
    serde_json::from_str(JOURNEYS).unwrap_or_else(|error| panic!("expected Ok value: {error:?}"))
}

fn facts(journey: &Value) -> VerifiedProviderFacts {
    authenticate_provider(journey).unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    let facts = &journey["provider_facts"];
    assert_eq!(facts["class"], "settled");
    VerifiedProviderFacts {
        provider: ExternalId::new(
            facts["provider"]
                .as_str()
                .unwrap_or_else(|| panic!("expected Some value")),
        )
        .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
        settlement: ExternalId::new(
            facts["settlement"]
                .as_str()
                .unwrap_or_else(|| panic!("expected Some value")),
        )
        .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
        rail: match facts["rail"]
            .as_str()
            .unwrap_or_else(|| panic!("expected Some value"))
        {
            "card" => FiatRail::Card,
            "bank" => FiatRail::Bank,
            "rtp" => FiatRail::RealTimePayment,
            other => panic!("unsupported recorded rail {other}"),
        },
        class: EvidenceClass::Settled,
        amount: facts["amount"]
            .as_str()
            .unwrap_or_else(|| panic!("expected Some value"))
            .parse()
            .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
        asset: array(&facts["asset"]),
        destination: array(&facts["destination"]),
        observed_at: facts["observed_at"]
            .as_u64()
            .unwrap_or_else(|| panic!("expected Some value")),
        hold_until: None,
    }
}

fn authenticate_provider(journey: &Value) -> TestResult {
    let token = TokenReference::new(
        journey["token"]
            .as_str()
            .unwrap_or_else(|| panic!("expected Some value"))
            .as_bytes()
            .to_vec(),
    )?;
    let payload = serde_json::to_vec(&journey["provider_facts"])?;
    let evidence = ProviderEvidence::new(payload)?;
    let public = VerifyingKey::from_bytes(&array(&journey["provider_public_key"]))?;
    let signature = Signature::from_bytes(&array(&journey["provider_signature"]));
    let message = [
        b"LayerX/fiat/sequenced-provider/v1\0".as_slice(),
        evidence.canonical(),
    ]
    .concat();
    public.verify_strict(&message, &signature)?;
    let token_digest: [u8; 32] = Sha256::digest(token.as_bytes()).into();
    if token_digest != array(&journey["provider_facts"]["token_reference_sha256"]) {
        return Err("provider recording does not authenticate this token".into());
    }
    Ok(())
}

#[test]
pub fn provider_recordings_authenticate() -> TestResult {
    let recordings = journeys();
    assert_eq!(recordings.len(), 5);
    for journey in recordings {
        authenticate_provider(&journey)?;

        let mut changed_token = journey.clone();
        changed_token["token"] = Value::from("tok_different_provider_recording");
        assert!(authenticate_provider(&changed_token).is_err());

        let mut changed_facts = journey.clone();
        changed_facts["provider_facts"]["amount"] = Value::from("1");
        assert!(authenticate_provider(&changed_facts).is_err());

        let mut changed_signature = journey.clone();
        changed_signature["provider_signature"] = Value::from(format!("0x{}", "00".repeat(64)));
        assert!(authenticate_provider(&changed_signature).is_err());
    }
    Ok(())
}

#[test]
pub fn fiat_payment_call_carries_provider_reference() -> TestResult {
    let mut references = std::collections::BTreeSet::new();
    for journey in journeys()
        .iter()
        .filter(|journey| journey["expect"]["state"] == "credited")
    {
        let facts = facts(journey);
        let provider = ProviderEvidence::new(serde_json::to_vec(&journey["provider_facts"])?)?;
        let reference = FiatAdapter::idempotency_key(&facts);
        let mut intent = FiatIntent {
            kind: FiatIntentKind::Credit,
            provider: facts.provider.clone(),
            settlement: facts.settlement.clone(),
            amount: facts.amount,
            asset: facts.asset,
            destination: facts.destination,
            evidence_digest: provider.digest(),
            idempotency_key: reference,
        };
        let payer = PAYER;
        let expires_at = EXPIRES_AT;
        let encoded = intent.payment_calldata(payer, expires_at);
        assert_eq!(&encoded[..4], &[0x78, 0xea, 0x11, 0xdb]);
        assert_eq!(&encoded[4..36], &payer);
        assert_eq!(&encoded[36..68], &facts.destination);
        assert_eq!(&encoded[68..100], &facts.asset);
        assert_eq!(&encoded[100..116], &[0; 16]);
        assert_eq!(&encoded[116..132], &facts.amount.to_be_bytes());
        assert_eq!(&encoded[132..156], &[0; 24]);
        assert_eq!(&encoded[156..164], &expires_at.to_be_bytes());
        assert_eq!(&encoded[164..], reference);
        assert_eq!(reference, array(&journey["sequencer"]["payment_reference"]));
        assert_eq!(intent.payment_calldata(payer, expires_at), encoded);
        references.insert(reference);

        for kind in [FiatIntentKind::Reverse, FiatIntentKind::Chargeback] {
            intent.kind = kind;
            let refund = intent.payment_calldata(payer, expires_at);
            assert_eq!(&refund[4..36], facts.destination);
            assert_eq!(&refund[36..68], payer);
            assert_eq!(&refund[68..], &encoded[68..]);
        }
    }
    assert_eq!(references.len(), 3);
    Ok(())
}

fn evidence(case: &Value) -> SequencerEvidence {
    SequencerEvidence {
        transaction_hash: array(&case["receiptJson"]["txHash"]),
        signed_receipt: bytes(&case["signedReceipt"]),
        logs: case["receiptJson"]["logs"]
            .as_array()
            .unwrap_or_else(|| panic!("expected Some value"))
            .iter()
            .map(|log| {
                SequencerLog::new(
                    array(&log["address"]),
                    log["topics"]
                        .as_array()
                        .unwrap_or_else(|| panic!("expected Some value"))
                        .iter()
                        .map(array)
                        .collect(),
                    bytes(&log["data"]),
                    u64::from_str_radix(
                        log["logIndex"]
                            .as_str()
                            .unwrap_or_else(|| panic!("expected Some value"))
                            .strip_prefix("0x")
                            .unwrap_or_else(|| panic!("expected Some value")),
                        16,
                    )
                    .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
                )
            })
            .collect(),
        rung: SettlementRung::Receipted,
    }
}

fn keys(case: &Value) -> [SequencerAuthorization; 1] {
    let pin = &case["pinnedKey"];
    [SequencerAuthorization::new(
        array(&pin["keyId"]),
        array(&pin["publicKey"]),
        pin["firstHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("expected Some value")),
        pin["lastHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("expected Some value")),
    )]
}

fn authenticate(case: &Value, evidence: &SequencerEvidence) -> TestResult {
    assert_eq!(evidence.signed_receipt, bytes(&case["signedReceipt"]));
    assert_eq!(
        evidence.signed_receipt,
        bytes(&case["receiptJson"]["receipt"])
    );
    let receipt = verify_sequencer_signature(&evidence.signed_receipt, &keys(case))
        .map_err(|error| format!("producer signature: {error:?}"))?;
    assert_eq!(
        receipt
            .protocol()
            .unwrap_or_else(|| panic!("expected Some value"))
            .activity_id(),
        evidence.transaction_hash
    );
    verify_logs(&receipt, evidence.logs.clone())
        .map_err(|error| format!("producer logs: {error:?}"))?;
    assert_eq!(
        canonical_receipt_digest(&evidence.signed_receipt)?,
        array(&case["receiptDigest"])
    );
    Ok(())
}

fn reserve(facts: &VerifiedProviderFacts) -> (GatewayCore, PrincipalId, TraceId) {
    let principal = PrincipalId::new("fiat-producer-path")
        .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    let trace = TraceId::mint([0x33; 16]);
    let mut gateway = GatewayCore::new();
    let adapter =
        AdapterId::new("fiat").unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    let digest: [u8; 32] = Sha256::digest(JOURNEYS.as_bytes()).into();
    let spec = PinnedSpec::new(
        adapter.clone(),
        SpecVersion::parse("1.0.0").unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
        digest,
    )
    .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    let suite = ConformanceSuite::new(
        AdapterId::new("fiat-producer-path")
            .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
        5,
        digest,
    )
    .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    gateway
        .register_adapter(
            fiat_adapter_descriptor(spec, suite)
                .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
            &trace,
            10,
        )
        .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    let request = TranslationRequest::new(
        adapter,
        TranslationKind::StateChanging,
        FiatAdapter::idempotency_key(facts),
        digest,
    )
    .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    assert_eq!(
        gateway
            .begin_translation(&principal, &request, &trace, 11)
            .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
        TranslationStatus::Pending
    );
    (gateway, principal, trace)
}

struct SignedProvider<'a>(&'a Value);

impl ProviderVerifier for SignedProvider<'_> {
    fn verify(
        &self,
        token: &TokenReference,
        evidence: &ProviderEvidence,
        _: &TraceId,
    ) -> Result<VerifiedProviderFacts, FiatError> {
        if token.as_bytes()
            != self.0["token"]
                .as_str()
                .unwrap_or_else(|| panic!("expected Some value"))
                .as_bytes()
            || evidence.canonical()
                != serde_json::to_vec(&self.0["provider_facts"])
                    .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"))
            || authenticate_provider(self.0).is_err()
        {
            return Err(FiatError::InvalidEvidence);
        }
        Ok(facts(self.0))
    }
}

struct NodePlane<'a> {
    node: &'a PaymentNode,
    hash: Option<[u8; 32]>,
    keys: [SequencerAuthorization; 1],
}

impl<'a> NodePlane<'a> {
    fn new(node: &'a PaymentNode) -> Self {
        Self {
            node,
            hash: None,
            keys: payment_node::receipt_keys(),
        }
    }
}

impl FiatPlane for NodePlane<'_> {
    fn receipt_keys(&self) -> &[SequencerAuthorization] {
        &self.keys
    }
    fn execute(&mut self, intent: FiatIntent, _: &TraceId) -> Result<FiatPlaneResult, FiatError> {
        let hash = if let Some(hash) = self.hash {
            hash
        } else {
            let hash = self
                .node
                .submit(&intent)
                .map_err(|_| FiatError::ReceiptRequired)?;
            self.hash = Some(hash);
            hash
        };
        let case = self
            .node
            .rpc("pax_sequencerReceipt", &json!([payment_node::hex(&hash)]))
            .map_err(|_| FiatError::ReceiptRequired)?;
        let evidence = if case.is_null() {
            SequencerEvidence {
                transaction_hash: hash,
                signed_receipt: Vec::new(),
                logs: Vec::new(),
                rung: SettlementRung::Submitted,
            }
        } else {
            assert_eq!(
                case,
                self.node.transcript(),
                "consumer receives the producer transcript unchanged"
            );
            let evidence = evidence(&case);
            authenticate(&case, &evidence).map_err(|_| FiatError::ReceiptMismatch)?;
            evidence
        };
        Ok(FiatPlaneResult::Executed(ExecutedFiatOutcome { evidence }))
    }
}

fn intent(journey: &Value) -> FiatIntent {
    let facts = facts(journey);
    let provider = ProviderEvidence::new(
        serde_json::to_vec(&journey["provider_facts"])
            .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    FiatIntent {
        kind: FiatIntentKind::Credit,
        provider: facts.provider.clone(),
        settlement: facts.settlement.clone(),
        amount: facts.amount,
        asset: facts.asset,
        destination: facts.destination,
        evidence_digest: provider.digest(),
        idempotency_key: FiatAdapter::idempotency_key(&facts),
    }
}

pub fn produced_case(journey: &Value) -> Result<Value, Box<dyn std::error::Error>> {
    let node = PaymentNode::start()?;
    let hash = node.submit(&intent(journey))?;
    assert!(node
        .rpc("pax_sequencerReceipt", &json!([payment_node::hex(&hash)]))?
        .is_null());
    let transcript = node.transcript();
    for offset in [
        payment_node::calldata_offset() + 164,
        bytes(&transcript["rawTransaction"]).len() - 1,
    ] {
        let mut changed = bytes(&transcript["rawTransaction"]);
        changed[offset] ^= 1;
        assert!(
            node.rpc(
                "fixture_submitPayment",
                &json!([payment_node::hex(&changed)])
            )?
            .is_null(),
            "a mutated call or signature is not producer input"
        );
        assert_eq!(node.submissions(), 1);
        assert_eq!(node.transcript(), transcript);
    }
    node.ready();
    assert!(node
        .rpc(
            "pax_sequencerReceipt",
            &json!([payment_node::hex(&[0; 32])])
        )?
        .is_null());
    let case = node.rpc("pax_sequencerReceipt", &json!([payment_node::hex(&hash)]))?;
    assert_eq!(case, node.transcript());
    assert_eq!(node.submissions(), 1);
    authenticate(&case, &evidence(&case))?;
    Ok(case)
}

fn apply(
    journey: &Value,
    gateway: &mut GatewayCore,
    principal: &PrincipalId,
    plane: &mut NodePlane<'_>,
    trace: &TraceId,
    now: u64,
) -> Result<FiatJourneyState, Box<dyn std::error::Error>> {
    let token = TokenReference::new(
        journey["token"]
            .as_str()
            .unwrap_or_else(|| panic!("expected Some value"))
            .as_bytes()
            .to_vec(),
    )?;
    let provider = ProviderEvidence::new(serde_json::to_vec(&journey["provider_facts"])?)?;
    Ok(FiatAdapter::apply(
        gateway,
        principal,
        &token,
        &provider,
        &SignedProvider(journey),
        plane,
        trace,
        now,
    )?)
}

// Register only: apply must reserve the request using its real provider digest.
fn gateway(journey: &Value) -> (GatewayCore, PrincipalId, TraceId) {
    let (reserved, principal, trace) = reserve(&facts(journey));
    let mut gateway = GatewayCore::new();
    let adapter = reserved
        .adapter(
            &AdapterId::new("fiat").unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
        )
        .unwrap_or_else(|| panic!("expected Some value"))
        .clone();
    gateway
        .register_adapter(adapter, &trace, 10)
        .unwrap_or_else(|error| panic!("expected Ok value: {error:?}"));
    (gateway, principal, trace)
}

fn completed_once(gateway: &GatewayCore, principal: &PrincipalId) {
    assert_eq!(
        gateway
            .principal_audit(principal)
            .unwrap_or_else(|| panic!("expected Some value"))
            .entries()
            .iter()
            .filter(|entry| entry.kind() == AuditEventKind::TranslationCompleted)
            .count(),
        1
    );
}

#[test]
pub fn fiat_unit_path() -> TestResult {
    let journey = journeys()
        .into_iter()
        .find(|v| v["name"] == "card_receipted_credit")
        .unwrap_or_else(|| panic!("expected Some value"));
    let node = PaymentNode::start()?;
    node.ready();
    let mut plane = NodePlane::new(&node);
    let (mut gateway, principal, trace) = gateway(&journey);
    let state = apply(&journey, &mut gateway, &principal, &mut plane, &trace, 12)?;
    let case = node.transcript();
    let evidence = evidence(&case);
    authenticate(&case, &evidence)?;
    let digest = canonical_receipt_digest(&evidence.signed_receipt)?;
    assert_eq!(
        state,
        FiatJourneyState::Credited {
            receipt_digest: digest
        }
    );
    let mut reopened_plane = NodePlane::new(&node);
    assert_eq!(
        apply(
            &journey,
            &mut gateway,
            &principal,
            &mut reopened_plane,
            &trace,
            13
        )?,
        state
    );
    assert_eq!(
        gateway.translation(&principal, FiatAdapter::idempotency_key(&facts(&journey))),
        Some(TranslationStatus::ReceiptVerified {
            receipt_digest: digest
        })
    );
    assert_eq!(node.submissions(), 1);
    assert_eq!(node.transcript(), case);
    completed_once(&gateway, &principal);
    Ok(())
}

#[test]
pub fn fiat_resigned_vectors() -> TestResult {
    let journeys = journeys();
    assert_eq!(journeys.len(), 5);
    let mut checked = 0;
    for journey in &journeys {
        authenticate_provider(journey)?;
        if journey["expect"]["state"] != "credited" {
            continue;
        }
        let case = produced_case(journey)?;
        let evidence = evidence(&case);
        authenticate(&case, &evidence)?;
        let call = PaymentNode::calldata(&case);
        let facts = facts(journey);
        let reference = FiatAdapter::idempotency_key(&facts);
        assert_eq!(reference, array(&journey["sequencer"]["payment_reference"]));
        assert_eq!(
            &call[164..196],
            reference,
            "producer call carries the adapter reference before signing"
        );
        assert_ne!(
            case["receiptJson"]["txHash"], journey["sequencer"]["transaction_hash"],
            "a different transaction must have its own activity identity"
        );
        let log = &evidence.logs[0];
        assert_eq!(
            &log.topics()[1..],
            &[
                reference,
                call[4..36]
                    .try_into()
                    .unwrap_or_else(|error| panic!("expected Ok value: {error:?}")),
                facts.destination
            ]
        );
        assert_eq!(&call[36..68], facts.destination);
        assert_eq!(&call[68..100], facts.asset);
        assert_eq!(&log.data()[..32], facts.asset);
        assert_eq!(&log.data()[32..48], &[0; 16]);
        assert_eq!(&log.data()[48..64], facts.amount.to_be_bytes());
        assert_eq!(
            canonical_receipt_digest(&evidence.signed_receipt)?,
            array(&case["receiptDigest"])
        );
        let (mut gateway, principal, trace) = reserve(&facts);
        assert_eq!(
            FiatAdapter::settle_evidence(
                &mut gateway,
                &principal,
                reference,
                &facts,
                &evidence,
                &keys(&case),
                &trace,
                12
            )?,
            FiatJourneyState::Credited {
                receipt_digest: array(&case["receiptDigest"])
            }
        );
        checked += 1;
    }
    assert_eq!(
        checked, 3,
        "all settled rails use original producer evidence"
    );
    Ok(())
}

#[test]
pub fn fiat_settlement_path() -> TestResult {
    let mut checked = 0;
    for journey in journeys()
        .iter()
        .filter(|v| v["expect"]["state"] == "credited")
    {
        let facts = facts(journey);
        let reference = FiatAdapter::idempotency_key(&facts);
        let (mut gateway, principal, trace) = gateway(journey);
        let node = PaymentNode::start()?;
        let mut plane = NodePlane::new(&node);
        let mut pending_snapshot = None;
        for now in [12, 13] {
            assert_eq!(
                apply(journey, &mut gateway, &principal, &mut plane, &trace, now)?,
                FiatJourneyState::CreditPending
            );
            assert_eq!(
                gateway.translation(&principal, reference),
                Some(TranslationStatus::Pending)
            );
            assert_eq!(node.submissions(), 1);
            let snapshot = format!("{gateway:?}");
            if let Some(before) = &pending_snapshot {
                assert_eq!(&snapshot, before);
            }
            pending_snapshot = Some(snapshot);
        }
        let hash = plane.hash;
        let mut plane = NodePlane::new(&node);
        plane.hash = hash;
        assert_eq!(
            apply(journey, &mut gateway, &principal, &mut plane, &trace, 14)?,
            FiatJourneyState::CreditPending
        );
        node.ready();
        let case = node.transcript();
        let evidence = evidence(&case);
        authenticate(&case, &evidence)?;
        let digest = canonical_receipt_digest(&evidence.signed_receipt)?;
        for now in [15, 16] {
            assert_eq!(
                apply(journey, &mut gateway, &principal, &mut plane, &trace, now)?,
                FiatJourneyState::Credited {
                    receipt_digest: digest
                }
            );
            assert_eq!(
                gateway.translation(&principal, reference),
                Some(TranslationStatus::ReceiptVerified {
                    receipt_digest: digest
                })
            );
            assert_eq!(node.transcript(), case);
            assert_eq!(node.submissions(), 1);
        }
        completed_once(&gateway, &principal);
        let (mut pending_gateway, principal, trace) = reserve(&facts);
        for changed in 0..3 {
            let mut invalid = evidence.clone();
            match changed {
                0 => invalid.transaction_hash[0] ^= 1,
                1 => {
                    *invalid
                        .signed_receipt
                        .last_mut()
                        .unwrap_or_else(|| panic!("expected Some value")) ^= 1;
                }
                _ => {
                    let log = &invalid.logs[0];
                    let mut topics = log.topics().to_vec();
                    topics[1][0] ^= 1;
                    invalid.logs[0] =
                        SequencerLog::new(log.address(), topics, log.data().to_vec(), 0);
                }
            }
            assert!(FiatAdapter::settle_evidence(
                &mut pending_gateway,
                &principal,
                reference,
                &facts,
                &invalid,
                &keys(&case),
                &trace,
                17
            )
            .is_err());
            assert_eq!(
                pending_gateway.translation(&principal, reference),
                Some(TranslationStatus::Pending)
            );
        }
        checked += 1;
    }
    assert_eq!(checked, 3);
    Ok(())
}
