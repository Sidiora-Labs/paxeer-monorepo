//! Fiat provider-callback conformance harness. The cases are the first-party
//! suite under `services/interop/specs/conformance/fiat`, which the gateway deployment
//! pins by identifier, vector count and digest: `token-references.json` holds
//! the exact token bytes the card-data boundary must accept or refuse, and
//! `journeys.json` holds one provider-evidence journey per record. Every
//! record runs through the production `TokenReference`, `VerifiedProviderFacts`
//! and `FiatAdapter` types. Settlement also exercises the recorded sequencer
//! receipt and its pinned signing key; journey-specific receipts are signed
//! from each vector's key seed.

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_fiat::{
    fiat_adapter_descriptor, EvidenceClass, ExternalId, FiatAdapter, FiatError, FiatIntent,
    FiatJourneyState, FiatPlane, FiatPlaneResult, FiatRail, PlaneFiatOutcome, ProviderEvidence,
    ProviderVerifier, TokenReference, VerifiedProviderFacts,
};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite, PinnedSpec, SpecVersion};
use layerx_interop_gateway::evidence::{
    canonical_receipt_digest, SequencerEvidence, SettlementRung,
};
use layerx_interop_gateway::gateway::{TranslationKind, TranslationRequest, TranslationStatus};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::{interop_gateway_core, GatewayCore};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::sequencer_logs::SequencerLog;
use sha2::{Digest as _, Sha256};

type ExecutedFiatOutcome = layerx_fiat::ExecutedFiatOutcome;

const PERIOD_START: u64 = 1_700_000_000;
const WINDOW_START: u64 = 200;
const SUITE: &str = "layerx-fiat-conformance-v1";
const TOKENS_SOURCE: &str = "services/interop/specs/conformance/fiat/token-references.json";
const JOURNEYS_SOURCE: &str = "services/interop/specs/conformance/fiat/journeys.json";

const TOKEN_REFERENCES: &str =
    include_str!("../../../specs/conformance/fiat/token-references.json");
const JOURNEYS: &str = include_str!("../../../specs/conformance/fiat/journeys.json");
const SEQUENCER_RECEIPTS_SOURCE: &str =
    "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json";
const SEQUENCER_RECEIPTS: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");

/// Every vector file of the suite by its path relative to the suite directory,
/// in the sorted order the deployment renderer walks.
const SUITE_FILES: [(&str, &str); 2] = [
    ("journeys.json", JOURNEYS),
    ("token-references.json", TOKEN_REFERENCES),
];

struct ReceiptFields {
    activity_id: [u8; 32],
    sequence: u64,
    previous_state_root: [u8; 32],
    resulting_state_root: [u8; 32],
    batch_id: [u8; 32],
    idempotency_key: [u8; 32],
    asset: [u8; 32],
    amount: u128,
    from: [u8; 32],
    to: [u8; 32],
}

struct ReceiptMaterial {
    evidence: SequencerEvidence,
    receipt_keys: [SequencerAuthorization; 1],
}

fn recorded_receipt() -> (ReceiptMaterial, [u8; 32]) {
    let source = SEQUENCER_RECEIPTS_SOURCE;
    let fixture: serde_json::Value = serde_json::from_str(SEQUENCER_RECEIPTS)
        .unwrap_or_else(|error| panic!("{source}: {error}"));
    let case = fixture["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("{source}: receipt cases"))
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .unwrap_or_else(|| panic!("{source}: recorded PaymentExact receipt"));
    let receipt = &case["receiptJson"];
    assert_eq!(field(receipt, "rung", source), "receipted");
    let logs = receipt["logs"]
        .as_array()
        .unwrap_or_else(|| panic!("{source}: receipt logs"))
        .iter()
        .map(|log| {
            let topics = log["topics"]
                .as_array()
                .unwrap_or_else(|| panic!("{source}: log topics"))
                .iter()
                .map(|topic| {
                    let topic = topic
                        .as_str()
                        .and_then(|value| value.strip_prefix("0x"))
                        .unwrap_or_else(|| panic!("{source}: log topic hex"));
                    hex_bytes::<32>(topic, source)
                })
                .collect();
            SequencerLog::new(
                recorded_array(log, "address"),
                topics,
                recorded_bytes(log, "data"),
                u64::from_str_radix(
                    field(log, "logIndex", source)
                        .strip_prefix("0x")
                        .unwrap_or_else(|| panic!("{source}: log index hex")),
                    16,
                )
                .unwrap_or_else(|error| panic!("{source}: log index: {error}")),
            )
        })
        .collect();
    let pin = &case["pinnedKey"];
    (
        ReceiptMaterial {
            evidence: SequencerEvidence {
                transaction_hash: recorded_array(receipt, "txHash"),
                signed_receipt: recorded_bytes(case, "signedReceipt"),
                logs,
                rung: SettlementRung::Receipted,
            },
            receipt_keys: [SequencerAuthorization::new(
                recorded_array(pin, "keyId"),
                recorded_array(pin, "publicKey"),
                number(pin, "firstHeight", source),
                number(pin, "lastHeight", source),
            )],
        },
        recorded_array(case, "receiptDigest"),
    )
}

fn recorded_bytes(record: &serde_json::Value, name: &str) -> Vec<u8> {
    let source = SEQUENCER_RECEIPTS_SOURCE;
    let value = field(record, name, source)
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("{source}: {name} hex prefix"));
    assert_eq!(value.len() % 2, 0, "{source}: {name} whole bytes");
    (0..value.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&value[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("{source}: {name}: {error}"))
        })
        .collect()
}

fn recorded_array<const N: usize>(record: &serde_json::Value, name: &str) -> [u8; N] {
    recorded_bytes(record, name)
        .try_into()
        .unwrap_or_else(|_| panic!("{SEQUENCER_RECEIPTS_SOURCE}: {name} must have {N} bytes"))
}

fn signed_receipt(
    seed: [u8; 32],
    sequence: u64,
    idempotency_key: [u8; 32],
    asset: [u8; 32],
    amount: u128,
    from: [u8; 32],
    to: [u8; 32],
) -> ReceiptMaterial {
    let activity_id: [u8; 32] = Sha256::digest(
        [
            b"fiat-adapter-activity/v1".as_slice(),
            &sequence.to_be_bytes(),
            &idempotency_key,
        ]
        .concat(),
    )
    .into();
    let mut batch_id = [0; 32];
    batch_id[24..].copy_from_slice(&sequence.to_be_bytes());
    let fields = ReceiptFields {
        activity_id,
        sequence,
        previous_state_root: Sha256::digest([b"before".as_slice(), &activity_id].concat()).into(),
        resulting_state_root: Sha256::digest([b"after".as_slice(), &activity_id].concat()).into(),
        batch_id,
        idempotency_key,
        asset,
        amount,
        from,
        to,
    };
    let signer = SigningKey::from_bytes(&seed);
    let mut address = [0; 20];
    address[18..].copy_from_slice(&[0x10, 0x1a]);
    let mut data = fields.asset.to_vec();
    data.extend_from_slice(&[0; 16]);
    data.extend_from_slice(&fields.amount.to_be_bytes());
    let log = SequencerLog::new(
        address,
        vec![
            hex_bytes::<32>(
                "c4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1",
                JOURNEYS_SOURCE,
            ),
            fields.from,
            fields.to,
            fields.idempotency_key,
        ],
        data,
        0,
    );
    let unsigned = encode_receipt(&fields, &log, None);
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    digest.update(&unsigned);
    let signature = signer.sign(&<[u8; 32]>::from(digest.finalize()));
    ReceiptMaterial {
        evidence: SequencerEvidence {
            transaction_hash: activity_id,
            signed_receipt: encode_receipt(&fields, &log, Some(signature.to_bytes())),
            logs: vec![log],
            rung: SettlementRung::Receipted,
        },
        receipt_keys: [SequencerAuthorization::new(
            [0x93; 32],
            signer.verifying_key().to_bytes(),
            sequence,
            sequence,
        )],
    }
}

fn encode_receipt(
    fields: &ReceiptFields,
    log: &SequencerLog,
    signature: Option<[u8; 64]>,
) -> Vec<u8> {
    let from_before = 50_000_u128;
    let to_before = 10_000_u128;
    let mut bytes = Vec::new();
    push_u16(&mut bytes, 2);
    push_u16(&mut bytes, 0x5201);
    push_u16(&mut bytes, 2);
    push_bytes(&mut bytes, &fields.activity_id);
    push_u64(&mut bytes, fields.sequence);
    push_bytes(&mut bytes, &fields.previous_state_root);
    push_bytes(&mut bytes, &fields.resulting_state_root);
    push_bytes(&mut bytes, &[0x81; 32]);
    bytes.extend_from_slice(&0_i32.to_be_bytes());
    bytes.extend_from_slice(&2_u32.to_be_bytes());
    let mut summary = Vec::new();
    for gas in [52_000_u64, 52_000, 1_000_000_000] {
        push_u64(&mut summary, gas);
    }
    summary.extend_from_slice(&0_u32.to_be_bytes());
    summary.extend_from_slice(&[0; 40]);
    summary.extend_from_slice(&Sha256::digest(b""));
    summary.extend_from_slice(&0_u32.to_be_bytes());
    summary.extend_from_slice(&1_u32.to_be_bytes());
    summary.push(4);
    summary.extend_from_slice(b"uhpx");
    let mut log_body = log.address().to_vec();
    log_body.push(4);
    for topic in log.topics() {
        log_body.extend_from_slice(topic);
    }
    log_body.extend_from_slice(&Sha256::digest(log.data()));
    for (ordinal, kind, body) in [(0, 1, summary), (1, 3, log_body)] {
        push_u16(&mut bytes, 12);
        push_u16(&mut bytes, ordinal);
        push_u16(&mut bytes, 0);
        bytes.push(kind);
        bytes.push(0);
        push_bytes(&mut bytes, &[0; 32]);
        push_bytes(&mut bytes, &body);
    }
    bytes.extend_from_slice(&1_u128.to_be_bytes());
    push_bytes(&mut bytes, &fields.batch_id);
    push_u16(&mut bytes, 12);
    bytes.extend_from_slice(&1_u32.to_be_bytes());
    bytes.extend_from_slice(&1_u32.to_be_bytes());
    bytes.push(1);
    push_bytes(&mut bytes, &fields.asset);
    bytes.extend_from_slice(&fields.amount.to_be_bytes());
    push_bytes(&mut bytes, &fields.from);
    bytes.extend_from_slice(&from_before.to_be_bytes());
    bytes.extend_from_slice(&(from_before - fields.amount).to_be_bytes());
    push_u64(&mut bytes, fields.sequence - WINDOW_START + 1);
    push_bytes(&mut bytes, &fields.to);
    bytes.extend_from_slice(&to_before.to_be_bytes());
    bytes.extend_from_slice(&(to_before + fields.amount).to_be_bytes());
    push_bytes(&mut bytes, &[0x91; 32]);
    push_bytes(&mut bytes, &[0x92; 32]);
    push_bytes(&mut bytes, &[0x93; 32]);
    push_u64(&mut bytes, PERIOD_START + fields.sequence);
    bytes.push(u8::from(signature.is_some()));
    if let Some(signature) = signature {
        push_bytes(&mut bytes, &signature);
    }
    bytes
}

fn push_u16(bytes: &mut Vec<u8>, value: u16) {
    bytes.extend_from_slice(&value.to_be_bytes());
}

fn push_u64(bytes: &mut Vec<u8>, value: u64) {
    bytes.extend_from_slice(&value.to_be_bytes());
}

fn push_bytes(output: &mut Vec<u8>, value: &[u8]) {
    let length = u32::try_from(value.len()).unwrap_or_else(|_| panic!("field overflow"));
    output.extend_from_slice(&length.to_be_bytes());
    output.extend_from_slice(value);
}

fn principal(name: &str) -> PrincipalId {
    PrincipalId::new(name).unwrap_or_else(|error| panic!("principal {name}: {error}"))
}

fn adapter_id() -> AdapterId {
    AdapterId::new("fiat").unwrap_or_else(|error| panic!("adapter: {error}"))
}

/// The suite digest over each vector file's path and bytes in sorted path
/// order: the exact rule `services/interop/deploy/gateway/render.py` applies to the same
/// files, so the declared suite is the suite this harness runs.
fn suite_digest() -> [u8; 32] {
    let mut hasher = Sha256::new();
    for (path, content) in SUITE_FILES {
        hasher.update(path.as_bytes());
        hasher.update([0]);
        hasher.update(content.as_bytes());
        hasher.update([0]);
    }
    hasher.finalize().into()
}

fn suite_vector_count() -> u64 {
    SUITE_FILES
        .iter()
        .map(|(path, content)| records(content, path).len() as u64)
        .sum()
}

fn conformance() -> ConformanceSuite {
    ConformanceSuite::new(
        AdapterId::new(SUITE).unwrap_or_else(|error| panic!("conformance identifier: {error}")),
        suite_vector_count(),
        suite_digest(),
    )
    .unwrap_or_else(|error| panic!("conformance: {error}"))
}

fn registered_gateway(now: u64) -> GatewayCore {
    let mut gateway = interop_gateway_core();
    let version = SpecVersion::parse("1.0.0").unwrap_or_else(|error| panic!("version: {error}"));
    let spec = PinnedSpec::new(adapter_id(), version, [0xa1; 32])
        .unwrap_or_else(|error| panic!("spec: {error}"));
    let descriptor = fiat_adapter_descriptor(spec, conformance())
        .unwrap_or_else(|error| panic!("descriptor: {error}"));
    gateway
        .register_adapter(descriptor, &TraceId::mint([1; 16]), now)
        .unwrap_or_else(|error| panic!("register: {error}"));
    gateway
}

#[derive(Clone)]
struct SandboxVerifier {
    facts: VerifiedProviderFacts,
    fault: Option<FiatError>,
}

impl ProviderVerifier for SandboxVerifier {
    fn verify(
        &self,
        _token: &TokenReference,
        _evidence: &ProviderEvidence,
        _trace: &TraceId,
    ) -> Result<VerifiedProviderFacts, FiatError> {
        if let Some(fault) = self.fault {
            return Err(fault);
        }
        Ok(self.facts.clone())
    }
}

struct SandboxPlane {
    intent_outcome: Result<FiatPlaneResult, FiatError>,
    receipt_keys: Vec<SequencerAuthorization>,
}

impl FiatPlane for SandboxPlane {
    fn receipt_keys(&self) -> &[SequencerAuthorization] {
        &self.receipt_keys
    }

    fn execute(
        &mut self,
        _intent: FiatIntent,
        _trace: &TraceId,
    ) -> Result<FiatPlaneResult, FiatError> {
        match &self.intent_outcome {
            Ok(FiatPlaneResult::Open(outcome)) => Ok(FiatPlaneResult::Open(*outcome)),
            Ok(FiatPlaneResult::Executed(executed)) => {
                Ok(FiatPlaneResult::Executed(ExecutedFiatOutcome {
                    evidence: executed.evidence.clone(),
                }))
            }
            Err(error) => Err(*error),
        }
    }
}

fn records(suite: &str, source: &str) -> Vec<serde_json::Value> {
    let records: Vec<serde_json::Value> =
        serde_json::from_str(suite).unwrap_or_else(|error| panic!("{source}: {error}"));
    assert!(
        !records.is_empty(),
        "{source}: a suite that carries no vector is not a conformance suite"
    );
    records
}

fn field<'a>(record: &'a serde_json::Value, name: &str, source: &str) -> &'a str {
    record[name]
        .as_str()
        .unwrap_or_else(|| panic!("{source}: every vector declares {name}"))
}

fn number(record: &serde_json::Value, name: &str, source: &str) -> u64 {
    record[name]
        .as_u64()
        .unwrap_or_else(|| panic!("{source}: every vector declares {name}"))
}

fn amount(record: &serde_json::Value, name: &str, source: &str) -> u128 {
    u128::from(number(record, name, source))
}

fn hex_bytes<const N: usize>(value: &str, source: &str) -> [u8; N] {
    assert_eq!(value.len(), N * 2, "{source}: {value} is not {N} bytes");
    let mut bytes = [0_u8; N];
    for (index, byte) in bytes.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&value[index * 2..index * 2 + 2], 16)
            .unwrap_or_else(|error| panic!("{source}: {value}: {error}"));
    }
    bytes
}

fn token_bytes(record: &serde_json::Value, source: &str) -> Vec<u8> {
    match (record.get("token"), record.get("token_hex")) {
        (Some(token), None) => token
            .as_str()
            .unwrap_or_else(|| panic!("{source}: token is a string"))
            .as_bytes()
            .to_vec(),
        (None, Some(hex)) => {
            let hex = hex
                .as_str()
                .unwrap_or_else(|| panic!("{source}: token_hex is a string"));
            assert!(hex.len() % 2 == 0, "{source}: token_hex is whole bytes");
            (0..hex.len() / 2)
                .map(|index| {
                    u8::from_str_radix(&hex[index * 2..index * 2 + 2], 16)
                        .unwrap_or_else(|error| panic!("{source}: {error}"))
                })
                .collect()
        }
        _ => panic!("{source}: every vector carries exactly one token representation"),
    }
}

fn fiat_error(name: &str, source: &str) -> FiatError {
    match name {
        "InvalidIdentifier" => FiatError::InvalidIdentifier,
        "CardDataRefused" => FiatError::CardDataRefused,
        "InvalidEvidence" => FiatError::InvalidEvidence,
        "HoldRequired" => FiatError::HoldRequired,
        "UnsupportedTransition" => FiatError::UnsupportedTransition,
        "ReceiptRequired" => FiatError::ReceiptRequired,
        "ReceiptMismatch" => FiatError::ReceiptMismatch,
        other => panic!("{source}: {other} is not a fiat refusal"),
    }
}

fn rail(name: &str, source: &str) -> FiatRail {
    match name {
        "card" => FiatRail::Card,
        "bank" => FiatRail::Bank,
        "real-time-payment" => FiatRail::RealTimePayment,
        other => panic!("{source}: {other} is not a certified provider rail"),
    }
}

fn evidence_class(name: &str, source: &str) -> EvidenceClass {
    match name {
        "authorised" => EvidenceClass::Authorised,
        "clearing" => EvidenceClass::Clearing,
        "settled" => EvidenceClass::Settled,
        "reversed" => EvidenceClass::Reversed,
        "chargeback" => EvidenceClass::Chargeback,
        other => panic!("{source}: {other} is not a provider evidence class"),
    }
}

fn external(value: &str, source: &str) -> ExternalId {
    ExternalId::new(value).unwrap_or_else(|error| panic!("{source}: {value}: {error:?}"))
}

fn verifier(facts: &serde_json::Value, source: &str) -> SandboxVerifier {
    let verified = VerifiedProviderFacts {
        provider: external(field(facts, "provider", source), source),
        settlement: external(field(facts, "settlement", source), source),
        rail: rail(field(facts, "rail", source), source),
        class: evidence_class(field(facts, "evidence_class", source), source),
        amount: amount(facts, "amount", source),
        asset: hex_bytes::<32>(field(facts, "asset", source), source),
        destination: hex_bytes::<32>(field(facts, "destination", source), source),
        observed_at: number(facts, "observed_at", source),
        hold_until: facts["hold_until"].as_u64(),
    };
    SandboxVerifier {
        facts: verified,
        fault: facts["fault"].as_str().map(|name| fiat_error(name, source)),
    }
}

/// Runs one journey vector through the production adapter and asserts its
/// declared outcome. Returns the rail it exercised and the journey state a
/// non-refused vector reached.
fn run_journey(record: &serde_json::Value) -> (FiatRail, Option<FiatJourneyState>) {
    let source = JOURNEYS_SOURCE;
    let name = field(record, "name", source);
    let verifier = verifier(&record["provider_facts"], source);
    let token = TokenReference::new(token_bytes(record, source))
        .unwrap_or_else(|error| panic!("{name}: token: {error}"));
    let evidence = ProviderEvidence::new(field(record, "evidence", source).as_bytes().to_vec())
        .unwrap_or_else(|error| panic!("{name}: evidence: {error}"));
    let trace = TraceId::mint(hex_bytes::<16>(field(record, "trace", source), source));
    let now = number(record, "now", source);
    let plane_record = &record["plane"];
    let declared_plane = field(plane_record, "outcome", source);
    let material = if declared_plane == "executed" {
        let receipt = &plane_record["receipt"];
        Some(signed_receipt(
            hex_bytes::<32>(field(receipt, "signing_key_seed", source), source),
            number(receipt, "sequence", source),
            FiatAdapter::idempotency_key(&verifier.facts),
            verifier.facts.asset,
            amount(receipt, "amount", source),
            hex_bytes::<32>(field(receipt, "from", source), source),
            hex_bytes::<32>(field(receipt, "to", source), source),
        ))
    } else {
        None
    };
    let intent_outcome = match (declared_plane, material.as_ref()) {
        ("pending", _) => Ok(FiatPlaneResult::Open(PlaneFiatOutcome::Pending)),
        ("refused", _) => Ok(FiatPlaneResult::Open(PlaneFiatOutcome::Refused)),
        ("executed", Some(material)) => Ok(FiatPlaneResult::Executed(ExecutedFiatOutcome {
            evidence: material.evidence.clone(),
        })),
        (other, _) => panic!("{source}: {other} is not a plane outcome"),
    };
    let mut plane = SandboxPlane {
        intent_outcome,
        receipt_keys: material
            .as_ref()
            .map(|material| material.receipt_keys.to_vec())
            .unwrap_or_default(),
    };
    let mut gateway = registered_gateway(now);
    let principal = principal(field(record, "principal", source));
    let outcome = FiatAdapter::apply(
        &mut gateway,
        &principal,
        &token,
        &evidence,
        &verifier,
        &mut plane,
        &trace,
        now,
    );
    let expect = &record["expect"];
    if let Some(refusal) = expect["refusal"].as_str() {
        assert_eq!(
            outcome.map_err(|traced| *traced.error()),
            Err(fiat_error(refusal, source)),
            "{name}"
        );
        return (verifier.facts.rail, None);
    }
    let state = outcome.unwrap_or_else(|error| panic!("{name}: {error}"));
    let digest = || {
        let material = material
            .as_ref()
            .unwrap_or_else(|| panic!("{name}: a completed journey carries a receipt"));
        canonical_receipt_digest(&material.evidence.signed_receipt)
            .unwrap_or_else(|error| panic!("{name}: receipt digest: {error:?}"))
    };
    let expected = match field(expect, "state", source) {
        "authorised-hold" => FiatJourneyState::AuthorisedHold {
            until: number(expect, "until", source),
        },
        "clearing-hold" => FiatJourneyState::ClearingHold {
            until: number(expect, "until", source),
        },
        "credit-pending" => FiatJourneyState::CreditPending,
        "credited" => FiatJourneyState::Credited {
            receipt_digest: digest(),
        },
        "reversal-pending" => FiatJourneyState::ReversalPending {
            hold_until: expect["hold_until"].as_u64(),
        },
        "reversed" => FiatJourneyState::Reversed {
            receipt_digest: digest(),
        },
        "chargeback-pending" => FiatJourneyState::ChargebackPending {
            hold_until: expect["hold_until"].as_u64(),
        },
        "charged-back" => FiatJourneyState::ChargedBack {
            receipt_digest: digest(),
        },
        "refused" => FiatJourneyState::Refused,
        other => panic!("{source}: {other} is not a fiat journey state"),
    };
    assert_eq!(state, expected, "{name}");
    if let Some(replay_at) = record["replay_at"].as_u64() {
        let replay = FiatAdapter::apply(
            &mut gateway,
            &principal,
            &token,
            &evidence,
            &verifier,
            &mut plane,
            &trace,
            replay_at,
        )
        .unwrap_or_else(|error| panic!("{name}: replay: {error}"));
        assert_eq!(
            replay, state,
            "{name}: replayed settlement must return the same outcome"
        );
    }
    (verifier.facts.rail, Some(state))
}

#[test]
fn fiat_adapter_suite_settles_on_the_recorded_receipt() {
    let (material, digest) = recorded_receipt();
    let executed = ExecutedFiatOutcome {
        evidence: material.evidence,
    };
    assert_eq!(
        canonical_receipt_digest(&executed.evidence.signed_receipt)
            .unwrap_or_else(|error| panic!("recorded receipt digest: {error:?}")),
        digest,
    );
    assert_eq!(executed.evidence.logs.len(), 1);
    let key = executed.evidence.logs[0].topics()[3];
    let facts = VerifiedProviderFacts {
        provider: external("recorded-card-provider", SEQUENCER_RECEIPTS_SOURCE),
        settlement: external("recorded-card-capture", SEQUENCER_RECEIPTS_SOURCE),
        rail: FiatRail::Card,
        class: EvidenceClass::Settled,
        amount: 1_000_000,
        asset: hex_bytes::<32>(
            "4e366c1f82bbcb0cfc80b6168346024ca11db454d4b5e93dc7cac57bc5292f28",
            SEQUENCER_RECEIPTS_SOURCE,
        ),
        destination: hex_bytes::<32>(
            "84ddb3e2af095e5f24fbe8a6d3e161eb9ebae5d2292861f3287f14e0db317985",
            SEQUENCER_RECEIPTS_SOURCE,
        ),
        observed_at: 10,
        hold_until: None,
    };
    let principal = principal("recorded-card-capture");
    let trace = TraceId::mint([0x18; 16]);
    let mut gateway = registered_gateway(10);
    let request = TranslationRequest::new(
        adapter_id(),
        TranslationKind::StateChanging,
        key,
        ProviderEvidence::new(b"recorded-card-provider-evidence".to_vec())
            .unwrap_or_else(|error| panic!("provider evidence: {error}"))
            .digest(),
    )
    .unwrap_or_else(|error| panic!("capture request: {error}"));
    assert_eq!(
        gateway
            .begin_translation(&principal, &request, &trace, 11)
            .unwrap_or_else(|error| panic!("reserve capture: {error}")),
        TranslationStatus::Pending,
    );
    let expected = FiatJourneyState::Credited {
        receipt_digest: digest,
    };
    for now in [12, 13] {
        assert_eq!(
            FiatAdapter::settle_evidence(
                &mut gateway,
                &principal,
                key,
                &facts,
                &executed.evidence,
                &material.receipt_keys,
                &trace,
                now,
            )
            .unwrap_or_else(|error| panic!("settle recorded card capture: {error}")),
            expected,
        );
        assert_eq!(
            gateway.translation(&principal, key),
            Some(TranslationStatus::ReceiptVerified {
                receipt_digest: digest,
            }),
        );
    }
}

#[test]
fn the_declared_suite_is_the_suite_this_harness_runs() {
    let suite = conformance();
    assert_eq!(suite.suite().as_str(), SUITE);
    assert_eq!(suite.vector_count(), suite_vector_count());
    assert_ne!(
        suite.suite_digest(),
        [0; 32],
        "an unpinned suite is not a suite"
    );
    assert_eq!(
        suite_vector_count(),
        u64::try_from(
            records(TOKEN_REFERENCES, TOKENS_SOURCE).len()
                + records(JOURNEYS, JOURNEYS_SOURCE).len()
        )
        .unwrap_or_else(|error| panic!("vector count: {error}"))
    );
}

#[test]
fn card_data_never_enters_layerx_components() {
    let mut refusals = 0_usize;
    let mut accepted = 0_usize;
    for record in records(TOKEN_REFERENCES, TOKENS_SOURCE) {
        let name = field(&record, "name", TOKENS_SOURCE);
        let bytes = token_bytes(&record, TOKENS_SOURCE);
        let parsed = TokenReference::new(bytes.clone());
        let valid = record["valid"]
            .as_bool()
            .unwrap_or_else(|| panic!("{TOKENS_SOURCE}: {name} declares whether it is accepted"));
        if valid {
            let token = parsed.unwrap_or_else(|error| panic!("{name}: {error}"));
            assert_eq!(token.as_bytes(), bytes.as_slice(), "{name}: exact bytes");
            let debug = format!("{token:?}");
            assert!(
                debug.contains("[REDACTED]"),
                "{name}: token debug must redact contents"
            );
            if let Some(probe) = record["redaction_probe"].as_str() {
                assert!(
                    !debug.contains(probe),
                    "{name}: token debug must not leak the actual token bytes"
                );
            }
            accepted += 1;
        } else {
            assert_eq!(
                parsed.err(),
                Some(fiat_error(
                    field(&record, "refusal", TOKENS_SOURCE),
                    TOKENS_SOURCE
                )),
                "{name}"
            );
            refusals += 1;
        }
    }
    assert!(accepted > 0, "{TOKENS_SOURCE}: no accepted token vector");
    assert!(
        refusals >= 6,
        "{TOKENS_SOURCE}: the card-data boundary needs its refusal cases"
    );
}

#[test]
fn every_provider_journey_reaches_its_declared_state() {
    let mut states = Vec::new();
    let mut refusals = Vec::new();
    for record in records(JOURNEYS, JOURNEYS_SOURCE) {
        let (_, state) = run_journey(&record);
        if state.is_some() {
            states.push(field(&record["expect"], "state", JOURNEYS_SOURCE).to_owned());
        } else {
            refusals.push(fiat_error(
                field(&record["expect"], "refusal", JOURNEYS_SOURCE),
                JOURNEYS_SOURCE,
            ));
        }
    }
    for state in [
        "authorised-hold",
        "clearing-hold",
        "credit-pending",
        "credited",
        "reversal-pending",
        "reversed",
        "chargeback-pending",
        "charged-back",
        "refused",
    ] {
        assert!(
            states.iter().any(|declared| declared == state),
            "{JOURNEYS_SOURCE}: {state} is not covered by a journey vector"
        );
    }
    for error in [
        FiatError::HoldRequired,
        FiatError::ReceiptMismatch,
        FiatError::InvalidEvidence,
        FiatError::CardDataRefused,
    ] {
        assert!(
            refusals.contains(&error),
            "{JOURNEYS_SOURCE}: {error} is not covered by a refusal vector"
        );
    }
}

#[test]
fn evidence_classes_model_rail_specific_settlement_stages() {
    assert_ne!(EvidenceClass::Authorised, EvidenceClass::Clearing);
    assert_ne!(EvidenceClass::Clearing, EvidenceClass::Settled);
    assert_ne!(EvidenceClass::Settled, EvidenceClass::Reversed);
    assert_ne!(EvidenceClass::Reversed, EvidenceClass::Chargeback);
    let mut classes = Vec::new();
    for record in records(JOURNEYS, JOURNEYS_SOURCE) {
        let verifier = verifier(&record["provider_facts"], JOURNEYS_SOURCE);
        if verifier.fault.is_none() {
            let facts = verifier
                .verify(
                    &TokenReference::new(token_bytes(&record, JOURNEYS_SOURCE))
                        .unwrap_or_else(|error| panic!("token: {error}")),
                    &ProviderEvidence::new(
                        field(&record, "evidence", JOURNEYS_SOURCE)
                            .as_bytes()
                            .to_vec(),
                    )
                    .unwrap_or_else(|error| panic!("evidence: {error}")),
                    &TraceId::mint(hex_bytes::<16>(
                        field(&record, "trace", JOURNEYS_SOURCE),
                        JOURNEYS_SOURCE,
                    )),
                )
                .unwrap_or_else(|error| panic!("verifier: {error}"));
            assert_eq!(facts.rail, verifier.facts.rail);
            assert_eq!(facts.class, verifier.facts.class);
            classes.push(facts.class);
        }
    }
    for class in [
        EvidenceClass::Authorised,
        EvidenceClass::Clearing,
        EvidenceClass::Settled,
        EvidenceClass::Reversed,
        EvidenceClass::Chargeback,
    ] {
        assert!(
            classes.contains(&class),
            "{JOURNEYS_SOURCE}: {class:?} is not covered by a journey vector"
        );
    }
}

#[test]
fn adapter_interfaces_are_rail_agnostic_and_provider_edge_only() {
    let mut credited = Vec::new();
    for record in records(JOURNEYS, JOURNEYS_SOURCE) {
        if record["expect"]["state"].as_str() != Some("credited") {
            continue;
        }
        let (rail, state) = run_journey(&record);
        assert!(
            matches!(state, Some(FiatJourneyState::Credited { .. })),
            "{}: {rail:?} rail must credit",
            field(&record, "name", JOURNEYS_SOURCE)
        );
        credited.push(rail);
    }
    for rail in [FiatRail::Card, FiatRail::Bank, FiatRail::RealTimePayment] {
        assert!(
            credited.contains(&rail),
            "{JOURNEYS_SOURCE}: {rail:?} does not credit through the adapter"
        );
    }
}
