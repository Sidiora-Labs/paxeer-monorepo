use std::collections::BTreeMap;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_interop_gateway::adapter::{
    AdapterDescriptor, AdapterId, ConformanceSuite, PinnedSpec, SpecVersion,
};
use layerx_interop_gateway::evidence::{
    canonical_receipt_digest, SequencerEvidence, SettlementRung,
};
use layerx_interop_gateway::gateway::TranslationStatus;
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_x402::facilitator::{
    Facilitator, FacilitatorKind, FacilitatorRequest, SequencerFacilitatorPlane,
    SettlementIdentity, SettlementStep, SupportedResponse,
};
use layerx_x402::model::{
    account_identifiers, AtomicAmount, PaymentPayload, PaymentRequirements, X402_VERSION,
};
use serde_json::{json, Value};

const FIXTURE: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
));

fn recorded_case() -> Value {
    let fixture: Value =
        serde_json::from_str(FIXTURE).unwrap_or_else(|error| panic!("recorded fixture: {error}"));
    fixture["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("fixture cases"))
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .unwrap_or_else(|| panic!("recorded PaymentExact case"))
        .clone()
}

fn text(value: &Value) -> &str {
    value.as_str().unwrap_or_else(|| panic!("fixture string"))
}

fn bytes(value: &str) -> Vec<u8> {
    let digits = value
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("hex prefix"));
    assert_eq!(digits.len() % 2, 0);
    (0..digits.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&digits[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("fixture hex: {error}"))
        })
        .collect()
}

fn array<const N: usize>(value: &str) -> [u8; N] {
    bytes(value)
        .try_into()
        .unwrap_or_else(|_| panic!("expected {N} bytes"))
}

fn recorded_evidence(case: &Value) -> SequencerEvidence {
    SequencerEvidence {
        transaction_hash: array(text(&case["receiptJson"]["txHash"])),
        signed_receipt: bytes(text(&case["signedReceipt"])),
        logs: case["receiptJson"]["logs"]
            .as_array()
            .unwrap_or_else(|| panic!("logs"))
            .iter()
            .map(|log| {
                SequencerLog::new(
                    array(text(&log["address"])),
                    log["topics"]
                        .as_array()
                        .unwrap_or_else(|| panic!("topics"))
                        .iter()
                        .map(|topic| array(text(topic)))
                        .collect(),
                    bytes(text(&log["data"])),
                    u64::from_str_radix(text(&log["logIndex"]).trim_start_matches("0x"), 16)
                        .unwrap_or_else(|error| panic!("log index: {error}")),
                )
            })
            .collect(),
        rung: SettlementRung::Receipted,
    }
}

fn receipt_keys(case: &Value) -> Vec<SequencerAuthorization> {
    let key = &case["pinnedKey"];
    vec![SequencerAuthorization::new(
        array(text(&key["keyId"])),
        array(text(&key["publicKey"])),
        key["firstHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("first height")),
        key["lastHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("last height")),
    )]
}

fn request(transaction: &str) -> FacilitatorRequest {
    let account = "agent:did:layerx:merchant:main";
    let pay_to = account_identifiers(account).unwrap_or_else(|error| panic!("merchant: {error}"))
        [0]
    .iter()
    .map(|byte| format!("{byte:02x}"))
    .collect::<String>();
    let requirements = PaymentRequirements {
        scheme: "exact".to_owned(),
        network: "eip155:125".to_owned(),
        amount: AtomicAmount::from_u128(1_000_000),
        asset: "0x4e366c1f82bbcb0cfc80b6168346024ca11db454d4b5e93dc7cac57bc5292f28".to_owned(),
        pay_to,
        max_timeout_seconds: 60,
        extra: Some(json!({"layerx": {"account": account, "currency": "LXP"}})),
    };
    FacilitatorRequest {
        x402_version: X402_VERSION,
        payment_payload: PaymentPayload {
            x402_version: X402_VERSION,
            resource: None,
            payload: json!({"transaction": transaction}),
            accepted: requirements.clone(),
            extensions: BTreeMap::new(),
        },
        payment_requirements: requirements,
    }
}

fn context() -> (Facilitator, GatewayCore, PrincipalId, TraceId) {
    let facilitator = Facilitator::new(SupportedResponse {
        kinds: vec![FacilitatorKind {
            x402_version: X402_VERSION,
            scheme: "exact".to_owned(),
            network: "eip155:125".to_owned(),
            extra: None,
        }],
        extensions: Vec::new(),
        signers: BTreeMap::from([("eip155:*".to_owned(), vec!["sequencer".to_owned()])]),
    })
    .unwrap_or_else(|error| panic!("facilitator: {error}"));
    let mut gateway = GatewayCore::new();
    let principal = PrincipalId::new("buyer").unwrap_or_else(|error| panic!("principal: {error}"));
    let trace = TraceId::mint([0x11; 16]);
    let adapter = AdapterId::new("x402").unwrap_or_else(|error| panic!("adapter: {error}"));
    let version = SpecVersion::parse("2.0.0").unwrap_or_else(|error| panic!("version: {error}"));
    let spec = PinnedSpec::new(adapter.clone(), version, [0x51; 32])
        .unwrap_or_else(|error| panic!("spec: {error}"));
    let suite = ConformanceSuite::new(
        AdapterId::new("x402-v2-vectors").unwrap_or_else(|error| panic!("suite id: {error}")),
        96,
        [0x52; 32],
    )
    .unwrap_or_else(|error| panic!("suite: {error}"));
    gateway
        .register_adapter(AdapterDescriptor::new(adapter, spec, suite), &trace, 0)
        .unwrap_or_else(|error| panic!("register: {error}"));
    (facilitator, gateway, principal, trace)
}

fn replace(receipt: &mut [u8], old: &[u8], new: &[u8]) {
    assert_eq!(old.len(), new.len());
    let positions: Vec<_> = receipt
        .windows(old.len())
        .enumerate()
        .filter_map(|(index, value)| (value == old).then_some(index))
        .collect();
    assert_eq!(positions.len(), 1);
    receipt[positions[0]..positions[0] + old.len()].copy_from_slice(new);
}

#[test]
fn facilitator_settles_receipted_evidence() {
    let case = recorded_case();
    let transaction = text(&case["receiptJson"]["txHash"]);
    let request = request(transaction);
    let mut evidence = recorded_evidence(&case);
    let recorded_keys = receipt_keys(&case);
    let recorded_receipt =
        verify_sequencer_signature(&evidence.signed_receipt, recorded_keys.as_slice())
            .unwrap_or_else(|error| panic!("recorded signature: {error:?}"));
    assert!(verify_logs(&recorded_receipt, evidence.logs.clone()).is_ok());
    assert_eq!(
        canonical_receipt_digest(&evidence.signed_receipt),
        Ok(array(text(&case["receiptDigest"])))
    );

    let (facilitator, mut gateway, principal, trace) = context();
    let stable = [0x42; 32];
    let identity = SettlementIdentity::derive(&principal, &request, stable, SettlementStep::Single)
        .unwrap_or_else(|error| panic!("identity: {error}"));

    // Preserve the recorded txHash, amount and asset. Sign the payment for
    // this request's account profile and derived settlement identity, which
    // differ from the standalone conformance vector's payee and payment key.
    let log = &evidence.logs[0];
    let mut topics = log.topics().to_vec();
    let payee = array(&format!("0x{}", request.payment_requirements.pay_to));
    let recorded_topics = topics.concat();
    topics = vec![topics[0], identity.idempotency_key, topics[1], payee];
    replace(
        &mut evidence.signed_receipt,
        &recorded_topics,
        &topics.concat(),
    );
    evidence.logs = vec![SequencerLog::new(
        log.address(),
        topics,
        log.data().to_vec(),
        log.log_index(),
    )];
    let signer = SigningKey::from_bytes(&[0x72; 32]);
    let receipt_digest = canonical_receipt_digest(&evidence.signed_receipt)
        .unwrap_or_else(|error| panic!("receipt digest: {error}"));
    let signature_start = evidence.signed_receipt.len() - 64;
    evidence.signed_receipt[signature_start..]
        .copy_from_slice(&signer.sign(&receipt_digest).to_bytes());
    let keys = vec![SequencerAuthorization::new(
        array(text(&case["pinnedKey"]["keyId"])),
        signer.verifying_key().to_bytes(),
        1000,
        1999,
    )];
    let mut plane = SequencerFacilitatorPlane::new(evidence.clone(), keys.clone());
    let response = facilitator
        .settle(
            &mut gateway,
            &principal,
            &request,
            stable,
            SettlementStep::Single,
            &mut plane,
            &trace,
            1,
        )
        .unwrap_or_else(|error| panic!("recorded transaction settlement: {error}"));
    assert!(response.success);
    assert_eq!(response.transaction, transaction);
    assert_eq!(
        response.payer.as_deref(),
        Some(text(&case["receiptJson"]["logs"][0]["topics"][1]).trim_start_matches("0x"))
    );
    assert_eq!(response.amount, Some(request.payment_requirements.amount));
    assert_eq!(
        response.extensions["layerx"]["verificationLevel"],
        "receipted"
    );
    assert_eq!(
        response.extensions["layerx"]["receipt"],
        STANDARD.encode(&evidence.signed_receipt)
    );
    assert_eq!(
        array::<32>(&format!(
            "0x{}",
            text(&response.extensions["layerx"]["receiptDigest"])
        )),
        receipt_digest
    );
    assert_eq!(
        gateway.translation(&principal, identity.idempotency_key),
        Some(TranslationStatus::ReceiptVerified { receipt_digest })
    );
    assert_eq!(response.validate_wire(), Ok(()));

    // Untrusted pins, tampering and an unrelated payment key must never
    // produce a successful settlement or advance the gateway record.
    let mut corrupt = evidence.clone();
    corrupt.signed_receipt[signature_start] ^= 1;
    let mut legacy = evidence.clone();
    let log = &legacy.logs[0];
    let native_topics = log.topics().to_vec();
    let mut topics = native_topics.clone();
    topics[1..].rotate_left(1);
    replace(
        &mut legacy.signed_receipt,
        &native_topics.concat(),
        &topics.concat(),
    );
    legacy.logs = vec![SequencerLog::new(
        log.address(),
        topics,
        log.data().to_vec(),
        log.log_index(),
    )];
    let digest = canonical_receipt_digest(&legacy.signed_receipt)
        .unwrap_or_else(|error| panic!("legacy receipt digest: {error}"));
    legacy.signed_receipt[signature_start..].copy_from_slice(&signer.sign(&digest).to_bytes());
    let verified = verify_sequencer_signature(&legacy.signed_receipt, keys.as_slice())
        .unwrap_or_else(|error| panic!("legacy signature: {error:?}"));
    assert!(verify_logs(&verified, legacy.logs.clone()).is_ok());
    for (evidence, keys) in [
        (evidence.clone(), recorded_keys),
        (corrupt, keys.clone()),
        (legacy, keys.clone()),
        (recorded_evidence(&case), receipt_keys(&case)),
    ] {
        let (facilitator, mut gateway, principal, trace) = context();
        let mut plane = SequencerFacilitatorPlane::new(evidence, keys);
        assert!(facilitator
            .settle(
                &mut gateway,
                &principal,
                &request,
                stable,
                SettlementStep::Single,
                &mut plane,
                &trace,
                1
            )
            .is_err());
        assert_eq!(
            gateway.translation(&principal, identity.idempotency_key),
            Some(TranslationStatus::Pending)
        );
    }
}

#[test]
fn facilitator_pending_reports_the_relayed_hash() {
    let case = recorded_case();
    let transaction = text(&case["receiptJson"]["txHash"]);
    let request = request(transaction);
    let mut evidence = recorded_evidence(&case);
    evidence.rung = SettlementRung::Submitted;
    evidence.signed_receipt.clear();
    evidence.logs.clear();
    let mut plane = SequencerFacilitatorPlane::new(evidence, Vec::new());
    let (facilitator, mut gateway, principal, trace) = context();
    let identity =
        SettlementIdentity::derive(&principal, &request, [0x42; 32], SettlementStep::Single)
            .unwrap_or_else(|error| panic!("identity: {error}"));
    let response = facilitator
        .settle(
            &mut gateway,
            &principal,
            &request,
            [0x42; 32],
            SettlementStep::Single,
            &mut plane,
            &trace,
            1,
        )
        .unwrap_or_else(|error| panic!("pending settlement: {error}"));
    assert!(!response.success);
    assert_eq!(response.error_reason.as_deref(), Some("settlement_pending"));
    assert_eq!(response.transaction, transaction);
    assert_ne!(array::<32>(&response.transaction), identity.request_digest);
    assert_ne!(array::<32>(&response.transaction), identity.idempotency_key);
    assert_eq!(
        response.extensions["layerx"]["verificationLevel"],
        "submitted"
    );
    assert_eq!(
        gateway.translation(&principal, identity.idempotency_key),
        Some(TranslationStatus::Pending)
    );
    assert!(response.extensions["layerx"].get("receipt").is_none());
    assert_eq!(response.validate_wire(), Ok(()));

    let verified = facilitator
        .verify(&mut gateway, &principal, &request, &mut plane, &trace, 2)
        .unwrap_or_else(|error| panic!("payload verification: {error}"));
    assert!(verified.is_valid);
    assert!(verified.invalid_reason.is_none());
    assert_eq!(
        gateway.translation(&principal, identity.idempotency_key),
        Some(TranslationStatus::Pending)
    );

    for transaction in [
        "lxp:abc",
        "0xabc",
        "",
        "0x0000000000000000000000000000000000000000000000000000000000000000",
        "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
    ] {
        let request = self::request(transaction);
        let response = facilitator
            .verify(&mut gateway, &principal, &request, &mut plane, &trace, 2)
            .unwrap_or_else(|error| panic!("transaction form refusal: {error}"));
        assert!(!response.is_valid);
        assert_eq!(response.invalid_reason.as_deref(), Some("transaction_form"));
    }
}
