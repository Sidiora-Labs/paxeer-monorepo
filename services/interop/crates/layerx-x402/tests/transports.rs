//! x402 v2 transport-binding conformance. The HTTP, MCP and A2A role messages
//! are the first-party suites under `services/interop/specs/conformance/transport-*`,
//! which the gateway deployment pins by digest; every record is a wire document
//! that this harness runs through the production encode and decode paths. The
//! fault-injected settlement cases below sign sequencer receipts and drive the
//! real gateway checks, so they stay in Rust and are not vectors.

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
use layerx_proof::sequencer_logs::SequencerLog;
use layerx_x402::facilitator::{
    Facilitator, FacilitatorKind, FacilitatorPaymentRequest, FacilitatorPlane, FacilitatorRequest,
    FacilitatorSettlementOutcome, PlaneVerifyOutcome, SettlementIdentity, SettlementStep,
    SupportedResponse, VerifyResponse,
};
use layerx_x402::model::{
    account_identifiers, AtomicAmount, PaymentPayload, PaymentRequired, PaymentRequirements,
    SettlementResponse, X402Error, PAYMENT_REQUIRED_HEADER, PAYMENT_RESPONSE_HEADER,
    PAYMENT_SIGNATURE_HEADER, X402_VERSION,
};
use layerx_x402::transport::{
    decode_facilitator_request, decode_facilitator_settlement, decode_payment_payload,
    decode_payment_required, decode_settlement, decode_supported_response, decode_verify_response,
    encode_facilitator_request, encode_facilitator_settlement, encode_payment_payload,
    encode_payment_required, encode_settlement, encode_supported_response, encode_verify_response,
    TransportKind, TransportValue, TRANSPORT_MATRIX,
};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const TRANSPORTS: [TransportKind; 3] =
    [TransportKind::Http, TransportKind::Mcp, TransportKind::A2a];

const HTTP_ROLE_SUITE: &str =
    include_str!("../../../specs/conformance/transport-http/role-messages.json");
const MCP_ROLE_SUITE: &str =
    include_str!("../../../specs/conformance/transport-mcp/role-messages.json");
const A2A_ROLE_SUITE: &str =
    include_str!("../../../specs/conformance/transport-a2a/role-messages.json");

const ROLES: [&str; 7] = [
    "payment-required",
    "payment-payload",
    "settlement-response",
    "facilitator-request",
    "verify-response",
    "facilitator-settlement",
    "supported-response",
];

const MERCHANT_ACCOUNT: &str = "agent:did:layerx:merchant:main";
const CURRENCY: &str = "LXP";

fn account_id(account: &str) -> [u8; 32] {
    account_identifiers(account)
        .unwrap_or_else(|error| panic!("{account} has account identifiers: {error}"))[0]
}

fn pay_to(account: &str) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut text = String::with_capacity(64);
    for byte in account_id(account) {
        text.push(char::from(DIGITS[usize::from(byte >> 4)]));
        text.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    text
}

fn requirements() -> PaymentRequirements {
    PaymentRequirements {
        scheme: "exact".to_owned(),
        network: "eip155:125".to_owned(),
        amount: AtomicAmount::from_u128(25),
        asset: "05".repeat(32),
        pay_to: pay_to(MERCHANT_ACCOUNT),
        max_timeout_seconds: 60,
        extra: Some(json!({
            "layerx": {
                "commitment": "executed",
                "account": MERCHANT_ACCOUNT,
                "currency": CURRENCY
            }
        })),
    }
}

fn payload() -> PaymentPayload {
    PaymentPayload {
        x402_version: X402_VERSION,
        resource: None,
        payload: json!({"transaction": format!("0x{}", hex(&PRIMARY_ACTIVITY))}),
        accepted: requirements(),
        extensions: BTreeMap::new(),
    }
}

fn facilitator_request() -> FacilitatorRequest {
    FacilitatorRequest {
        x402_version: X402_VERSION,
        payment_payload: payload(),
        payment_requirements: requirements(),
    }
}

struct RoleVector {
    name: String,
    role: String,
    accepted: bool,
    literal: TransportValue,
}

fn suite(transport: TransportKind) -> (&'static str, &'static str, &'static str) {
    match transport {
        TransportKind::Http => (
            HTTP_ROLE_SUITE,
            "http",
            "services/interop/specs/conformance/transport-http/role-messages.json",
        ),
        TransportKind::Mcp => (
            MCP_ROLE_SUITE,
            "mcp",
            "services/interop/specs/conformance/transport-mcp/role-messages.json",
        ),
        TransportKind::A2a => (
            A2A_ROLE_SUITE,
            "a2a",
            "services/interop/specs/conformance/transport-a2a/role-messages.json",
        ),
    }
}

fn payment_header(declared: &str, source: &str) -> &'static str {
    match declared {
        PAYMENT_REQUIRED_HEADER => PAYMENT_REQUIRED_HEADER,
        PAYMENT_SIGNATURE_HEADER => PAYMENT_SIGNATURE_HEADER,
        PAYMENT_RESPONSE_HEADER => PAYMENT_RESPONSE_HEADER,
        other => panic!("{source}: {other} is not an x402 v2 payment header"),
    }
}

fn role_vectors(transport: TransportKind) -> Vec<RoleVector> {
    let (text, name, source) = suite(transport);
    let records: Vec<serde_json::Value> =
        serde_json::from_str(text).unwrap_or_else(|error| panic!("{source}: {error}"));
    assert!(
        !records.is_empty(),
        "{source}: a suite that carries no vector is not a conformance suite"
    );
    records
        .into_iter()
        .map(|record| {
            let vector = record["name"]
                .as_str()
                .unwrap_or_else(|| panic!("{source}: every vector is named"))
                .to_owned();
            assert_eq!(
                record["transport"].as_str(),
                Some(name),
                "{source}: {vector} declares another transport binding"
            );
            let role = record["role"]
                .as_str()
                .unwrap_or_else(|| panic!("{source}: {vector} names no role message"))
                .to_owned();
            let accepted = record["accepted"].as_bool().unwrap_or_else(|| {
                panic!("{source}: {vector} declares whether the transport accepts it")
            });
            let document = record
                .get("document")
                .cloned()
                .unwrap_or_else(|| panic!("{source}: {vector} carries no wire document"));
            let literal = match record["representation"].as_str() {
                Some("http-header") => TransportValue::HttpHeader {
                    name: payment_header(
                        record["header"]
                            .as_str()
                            .unwrap_or_else(|| panic!("{source}: {vector} names no header")),
                        source,
                    ),
                    value: STANDARD.encode(
                        serde_json::to_vec(&document)
                            .unwrap_or_else(|error| panic!("{source}: {vector}: {error}")),
                    ),
                },
                Some("json") => TransportValue::Json(document),
                other => panic!("{source}: {vector} declares the representation {other:?}"),
            };
            RoleVector {
                name: vector,
                role,
                accepted,
                literal,
            }
        })
        .collect()
}

macro_rules! round_trip {
    ($model:ty, $encode:path, $decode:path, $transport:expr, $vector:expr, $source:expr) => {{
        let name = &$vector.name;
        if !$vector.accepted {
            assert!(
                $decode($transport, &$vector.literal).is_err(),
                "{}: {name}: this representation must be refused on this transport",
                $source
            );
        } else {
            let document = match &$vector.literal {
                TransportValue::Json(value) => value.clone(),
                TransportValue::HttpHeader { value, .. } => serde_json::from_slice(
                    &STANDARD
                        .decode(value.as_bytes())
                        .unwrap_or_else(|error| panic!("{}: {name}: {error}", $source)),
                )
                .unwrap_or_else(|error| panic!("{}: {name}: {error}", $source)),
            };
            let parsed: $model = serde_json::from_value(document)
                .unwrap_or_else(|error| panic!("{}: {name}: {error}", $source));
            let encoded = $encode($transport, &parsed)
                .unwrap_or_else(|error| panic!("{}: {name}: encode: {error}", $source));
            match (&encoded, &$vector.literal) {
                (
                    TransportValue::HttpHeader { name: produced, .. },
                    TransportValue::HttpHeader {
                        name: declared,
                        value: _,
                    },
                ) => assert_eq!(produced, declared, "{}: {name}: header name", $source),
                (TransportValue::Json(_), TransportValue::Json(_)) => assert_eq!(
                    &encoded, &$vector.literal,
                    "{}: {name}: JSON representation",
                    $source
                ),
                _ => panic!(
                    "{}: {name}: the transport encoding is not the declared representation",
                    $source
                ),
            }
            assert_eq!(
                $decode($transport, &encoded),
                Ok(parsed.clone()),
                "{}: {name}: produced representation",
                $source
            );
            assert_eq!(
                $decode($transport, &$vector.literal),
                Ok(parsed),
                "{}: {name}: declared representation",
                $source
            );
        }
    }};
}

#[test]
fn every_role_round_trips_on_every_transport() {
    for transport in TRANSPORTS {
        let (_, _, source) = suite(transport);
        let vectors = role_vectors(transport);
        for role in ROLES {
            assert!(
                vectors
                    .iter()
                    .any(|vector| vector.role == role && vector.accepted),
                "{source}: {role} is not covered"
            );
        }
        for vector in &vectors {
            match vector.role.as_str() {
                "payment-required" => round_trip!(
                    PaymentRequired,
                    encode_payment_required,
                    decode_payment_required,
                    transport,
                    vector,
                    source
                ),
                "payment-payload" => round_trip!(
                    PaymentPayload,
                    encode_payment_payload,
                    decode_payment_payload,
                    transport,
                    vector,
                    source
                ),
                "settlement-response" => round_trip!(
                    SettlementResponse,
                    encode_settlement,
                    decode_settlement,
                    transport,
                    vector,
                    source
                ),
                "facilitator-request" => round_trip!(
                    FacilitatorRequest,
                    encode_facilitator_request,
                    decode_facilitator_request,
                    transport,
                    vector,
                    source
                ),
                "verify-response" => round_trip!(
                    VerifyResponse,
                    encode_verify_response,
                    decode_verify_response,
                    transport,
                    vector,
                    source
                ),
                "facilitator-settlement" => round_trip!(
                    SettlementResponse,
                    encode_facilitator_settlement,
                    decode_facilitator_settlement,
                    transport,
                    vector,
                    source
                ),
                "supported-response" => round_trip!(
                    SupportedResponse,
                    encode_supported_response,
                    decode_supported_response,
                    transport,
                    vector,
                    source
                ),
                other => panic!("{source}: {other} is not an x402 v2 role message"),
            }
        }
    }
    assert!(TRANSPORT_MATRIX
        .iter()
        .all(|row| row.buyer && row.seller && row.facilitator));
}

#[test]
fn settlement_identity_is_transport_independent_and_step_separated() {
    let principal =
        PrincipalId::new("merchant-a").unwrap_or_else(|error| panic!("principal: {error:?}"));
    let request = facilitator_request();
    let stable = [9; 32];
    let baseline = SettlementIdentity::derive(&principal, &request, stable, SettlementStep::Single)
        .unwrap_or_else(|error| panic!("identity: {error}"));
    for transport in TRANSPORTS {
        let encoded = encode_facilitator_request(transport, &request)
            .unwrap_or_else(|error| panic!("encode: {error}"));
        let decoded = decode_facilitator_request(transport, &encoded)
            .unwrap_or_else(|error| panic!("decode: {error}"));
        assert_eq!(
            SettlementIdentity::derive(&principal, &decoded, stable, SettlementStep::Single),
            Ok(baseline)
        );
    }
    let deposit =
        SettlementIdentity::derive(&principal, &request, stable, SettlementStep::EscrowDeposit)
            .unwrap_or_else(|error| panic!("deposit identity: {error}"));
    let charge =
        SettlementIdentity::derive(&principal, &request, stable, SettlementStep::EscrowCharge)
            .unwrap_or_else(|error| panic!("charge identity: {error}"));
    assert_ne!(baseline.idempotency_key, deposit.idempotency_key);
    assert_ne!(deposit.idempotency_key, charge.idempotency_key);
    assert_ne!(deposit.request_digest, charge.request_digest);
}

// --- Fault-injected settlement: exactly-once economic effect -----------------
//
// The facilitator's only successful settlement path terminates in a
// gateway-verified canonical LayerX receipt. These tests drive that real path
// with a signed receipt whose economic facts match the transport-independent
// `requirements()` above (asset `05..`, the merchant account's own identifier
// as recipient, amount 25) and inject
// faults - delayed confirmation, a crashed downstream, duplicate delivery and a
// swapped receipt - to prove the economic effect lands exactly once regardless
// of how many times settlement is retried or redelivered.

const SEQUENCER_SEED: [u8; 32] = [3; 32];
const PRIMARY_ACTIVITY: [u8; 32] = [1; 32];
const ALTERNATE_ACTIVITY: [u8; 32] = [12; 32];

fn stable_identity() -> [u8; 32] {
    [9; 32]
}

fn trace() -> TraceId {
    TraceId::mint([0xa5; 16])
}

fn supported() -> SupportedResponse {
    SupportedResponse {
        kinds: vec![FacilitatorKind {
            x402_version: X402_VERSION,
            scheme: "exact".to_owned(),
            network: "eip155:125".to_owned(),
            extra: None,
        }],
        extensions: Vec::new(),
        signers: BTreeMap::from([(
            "eip155:*".to_owned(),
            vec!["did:layerx:facilitator".to_owned()],
        )]),
    }
}

fn facilitator() -> Facilitator {
    Facilitator::new(supported()).unwrap_or_else(|error| panic!("facilitator: {error}"))
}

fn register_x402(gateway: &mut GatewayCore, trace: &TraceId) {
    let adapter = AdapterId::new("x402").unwrap_or_else(|error| panic!("adapter id: {error}"));
    let version = SpecVersion::parse("2").unwrap_or_else(|error| panic!("version: {error}"));
    let spec = PinnedSpec::new(adapter.clone(), version, [0x7d; 32])
        .unwrap_or_else(|error| panic!("pinned spec: {error}"));
    let suite_id = AdapterId::new("x402-v2").unwrap_or_else(|error| panic!("suite id: {error}"));
    let conformance = ConformanceSuite::new(suite_id, 20, [0xc0; 32])
        .unwrap_or_else(|error| panic!("conformance suite: {error}"));
    let descriptor = AdapterDescriptor::new(adapter, spec, conformance);
    gateway
        .register_adapter(descriptor, trace, 0)
        .unwrap_or_else(|error| panic!("register x402 adapter: {error}"));
}

fn fixture_bytes(value: &Value) -> Vec<u8> {
    let digits = value
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .unwrap_or_else(|| panic!("fixture hex string"));
    assert_eq!(digits.len() % 2, 0);
    (0..digits.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&digits[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("fixture hex: {error}"))
        })
        .collect()
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

fn sequencer_evidence(
    signing_key: &SigningKey,
    transaction_hash: [u8; 32],
    idempotency_key: [u8; 32],
) -> SequencerEvidence {
    // Sign the sequencer layout with the requested payment facts and key;
    // the facilitator and gateway run their real signature and log checks.
    let fixture: Value = serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )))
    .unwrap_or_else(|error| panic!("fixture: {error}"));
    let case = fixture["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("fixture cases"))
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .unwrap_or_else(|| panic!("PaymentExact fixture"));
    let mut receipt = fixture_bytes(&case["signedReceipt"]);
    let log = &case["receiptJson"]["logs"][0];
    let mut topics: Vec<[u8; 32]> = log["topics"]
        .as_array()
        .unwrap_or_else(|| panic!("topics"))
        .iter()
        .map(|topic| {
            fixture_bytes(topic)
                .try_into()
                .unwrap_or_else(|_| panic!("topic"))
        })
        .collect();
    let payee = account_id(MERCHANT_ACCOUNT);
    replace(&mut receipt, &topics[2], &payee);
    topics[2] = payee;
    replace(&mut receipt, &topics[3], &idempotency_key);
    topics[3] = idempotency_key;
    let mut data = vec![5; 32];
    data.extend_from_slice(&[0; 16]);
    data.extend_from_slice(&25_u128.to_be_bytes());
    replace(
        &mut receipt,
        &Sha256::digest(fixture_bytes(&log["data"])),
        &Sha256::digest(&data),
    );
    replace(
        &mut receipt,
        &fixture_bytes(&case["receiptJson"]["txHash"]),
        &transaction_hash,
    );
    let digest = canonical_receipt_digest(&receipt)
        .unwrap_or_else(|error| panic!("receipt digest: {error}"));
    let signature_start = receipt.len() - 64;
    receipt[signature_start..].copy_from_slice(&signing_key.sign(&digest).to_bytes());
    SequencerEvidence {
        transaction_hash,
        signed_receipt: receipt,
        logs: vec![SequencerLog::new(
            fixture_bytes(&log["address"])
                .try_into()
                .unwrap_or_else(|_| panic!("log address")),
            topics,
            data,
            0,
        )],
        rung: SettlementRung::Receipted,
    }
}

/// One scripted plane behaviour per settlement attempt. Anything past the end
/// of the script defaults to a confirmed primary settlement, modelling a
/// downstream that recovers and then keeps confirming under retry.
#[derive(Clone, Copy)]
enum Planned {
    /// Settlement accepted but not yet confirmed on chain.
    Pending,
    /// The settlement authority crashed or timed out mid-call.
    Transient,
    /// Confirmed with the canonical primary receipt.
    Execute,
    /// Confirmed, but with a *different* receipt for the same identity.
    ExecuteAlternate,
}

struct FaultInjectingPlane {
    signing_key: SigningKey,
    receipt_keys: Vec<SequencerAuthorization>,
    script: Vec<Planned>,
    cursor: usize,
    settle_calls: usize,
}

impl FaultInjectingPlane {
    fn new(script: Vec<Planned>) -> Self {
        Self {
            signing_key: SigningKey::from_bytes(&SEQUENCER_SEED),
            receipt_keys: vec![SequencerAuthorization::new(
                [0x6b; 32],
                SigningKey::from_bytes(&SEQUENCER_SEED)
                    .verifying_key()
                    .to_bytes(),
                1000,
                1999,
            )],
            script,
            cursor: 0,
            settle_calls: 0,
        }
    }
}

impl FacilitatorPlane for FaultInjectingPlane {
    fn receipt_keys(&self) -> &[SequencerAuthorization] {
        &self.receipt_keys
    }

    fn verify(
        &mut self,
        _request: &FacilitatorPaymentRequest,
        _trace: &TraceId,
    ) -> Result<PlaneVerifyOutcome, X402Error> {
        Ok(PlaneVerifyOutcome::Valid {
            payer: None,
            extra: None,
        })
    }

    fn settle(
        &mut self,
        request: FacilitatorPaymentRequest,
        _trace: &TraceId,
    ) -> Result<FacilitatorSettlementOutcome, X402Error> {
        self.settle_calls += 1;
        let planned = self
            .script
            .get(self.cursor)
            .copied()
            .unwrap_or(Planned::Execute);
        self.cursor += 1;
        match planned {
            Planned::Pending => Ok(FacilitatorSettlementOutcome::Pending {
                transaction: format!("0x{}", hex(&PRIMARY_ACTIVITY)),
            }),
            Planned::Transient => Err(X402Error::Decode),
            Planned::Execute => Ok(FacilitatorSettlementOutcome::Executed(sequencer_evidence(
                &self.signing_key,
                PRIMARY_ACTIVITY,
                request.idempotency_key,
            ))),
            Planned::ExecuteAlternate => {
                Ok(FacilitatorSettlementOutcome::Executed(sequencer_evidence(
                    &self.signing_key,
                    ALTERNATE_ACTIVITY,
                    request.idempotency_key,
                )))
            }
        }
    }
}

fn principal() -> PrincipalId {
    PrincipalId::new("merchant-a").unwrap_or_else(|error| panic!("principal: {error:?}"))
}

fn settled_digest(gateway: &GatewayCore, principal: &PrincipalId, key: [u8; 32]) -> [u8; 32] {
    match gateway.translation(principal, key) {
        Some(TranslationStatus::ReceiptVerified { receipt_digest }) => receipt_digest,
        other => panic!("expected a receipt-verified settlement, found {other:?}"),
    }
}

#[test]
fn confirmed_settlement_records_exactly_one_receipt_verified_effect() {
    let facilitator = facilitator();
    let mut gateway = GatewayCore::new();
    let trace = trace();
    register_x402(&mut gateway, &trace);
    let principal = principal();
    let request = facilitator_request();
    let stable = stable_identity();
    let step = SettlementStep::Single;
    let identity = SettlementIdentity::derive(&principal, &request, stable, step)
        .unwrap_or_else(|error| panic!("identity: {error}"));

    let mut plane = FaultInjectingPlane::new(vec![Planned::Execute]);
    let response = facilitator
        .settle(
            &mut gateway,
            &principal,
            &request,
            stable,
            step,
            &mut plane,
            &trace,
            0,
        )
        .unwrap_or_else(|error| panic!("settle: {error}"));

    assert!(response.success);
    assert_eq!(
        response.extensions["layerx"]["verificationLevel"],
        "receipted"
    );
    assert_eq!(response.amount, Some(AtomicAmount::from_u128(25)));
    let digest = settled_digest(&gateway, &principal, identity.idempotency_key);
    assert_eq!(
        response.transaction,
        format!("0x{}", hex(&PRIMARY_ACTIVITY))
    );
    assert_eq!(response.extensions["layerx"]["receiptDigest"], hex(&digest));
    assert_eq!(plane.settle_calls, 1);
}

#[test]
fn duplicate_delivery_of_the_same_settlement_does_not_double_charge() {
    let facilitator = facilitator();
    let mut gateway = GatewayCore::new();
    let trace = trace();
    register_x402(&mut gateway, &trace);
    let principal = principal();
    let request = facilitator_request();
    let stable = stable_identity();
    let step = SettlementStep::Single;
    let identity = SettlementIdentity::derive(&principal, &request, stable, step)
        .unwrap_or_else(|error| panic!("identity: {error}"));

    // Every transport delivers the same economic request under one identity, so
    // three redeliveries drive three confirmed plane calls.
    let mut plane =
        FaultInjectingPlane::new(vec![Planned::Execute, Planned::Execute, Planned::Execute]);

    let mut transaction = None;
    for delivery in 0..3u64 {
        let response = facilitator
            .settle(
                &mut gateway,
                &principal,
                &request,
                stable,
                step,
                &mut plane,
                &trace,
                delivery,
            )
            .unwrap_or_else(|error| panic!("settle {delivery}: {error}"));
        assert!(response.success);
        match &transaction {
            None => transaction = Some(response.transaction.clone()),
            Some(first) => assert_eq!(&response.transaction, first),
        }
    }

    assert_eq!(plane.settle_calls, 3);
    let digest = settled_digest(&gateway, &principal, identity.idempotency_key);
    let expected = sequencer_evidence(
        &plane.signing_key,
        PRIMARY_ACTIVITY,
        identity.idempotency_key,
    );
    assert_eq!(
        canonical_receipt_digest(&expected.signed_receipt),
        Ok(digest)
    );
    assert_eq!(
        transaction,
        Some(format!("0x{}", hex(&PRIMARY_ACTIVITY))),
        "every redelivery resolves to one transaction hash"
    );
}

#[test]
fn transport_independent_identity_settles_once_across_http_mcp_and_a2a() {
    let facilitator = facilitator();
    let mut gateway = GatewayCore::new();
    let trace = trace();
    register_x402(&mut gateway, &trace);
    let principal = principal();
    let request = facilitator_request();
    let stable = stable_identity();
    let step = SettlementStep::Single;
    let identity = SettlementIdentity::derive(&principal, &request, stable, step)
        .unwrap_or_else(|error| panic!("identity: {error}"));

    let mut plane = FaultInjectingPlane::new(vec![Planned::Execute; 3]);
    let mut settled = None;
    for transport in TRANSPORTS {
        // The same request arrives over a different transport each time; the
        // wire encoding round-trips but the settlement identity is unchanged.
        let encoded = encode_facilitator_request(transport, &request)
            .unwrap_or_else(|error| panic!("encode {transport:?}: {error}"));
        let decoded = decode_facilitator_request(transport, &encoded)
            .unwrap_or_else(|error| panic!("decode {transport:?}: {error}"));
        let response = facilitator
            .settle(
                &mut gateway,
                &principal,
                &decoded,
                stable,
                step,
                &mut plane,
                &trace,
                0,
            )
            .unwrap_or_else(|error| panic!("settle {transport:?}: {error}"));
        assert!(response.success);
        match &settled {
            None => settled = Some(response.transaction.clone()),
            Some(first) => assert_eq!(&response.transaction, first),
        }
    }

    let digest = settled_digest(&gateway, &principal, identity.idempotency_key);
    let expected = sequencer_evidence(
        &plane.signing_key,
        PRIMARY_ACTIVITY,
        identity.idempotency_key,
    );
    assert_eq!(
        canonical_receipt_digest(&expected.signed_receipt),
        Ok(digest)
    );
    assert_eq!(settled, Some(format!("0x{}", hex(&PRIMARY_ACTIVITY))));
    assert_eq!(plane.settle_calls, 3);
}

#[test]
fn settlement_recovers_after_a_crash_without_a_second_economic_effect() {
    let facilitator = facilitator();
    let mut gateway = GatewayCore::new();
    let trace = trace();
    register_x402(&mut gateway, &trace);
    let principal = principal();
    let request = facilitator_request();
    let stable = stable_identity();
    let step = SettlementStep::Single;
    let identity = SettlementIdentity::derive(&principal, &request, stable, step)
        .unwrap_or_else(|error| panic!("identity: {error}"));

    // Attempt 1: delayed confirmation, then attempt 2 crashes, then attempt 3
    // confirms, then attempt 4 is a post-success duplicate.
    let mut plane = FaultInjectingPlane::new(vec![
        Planned::Pending,
        Planned::Transient,
        Planned::Execute,
        Planned::Execute,
    ]);

    // Delayed confirmation: no economic effect yet.
    let pending = facilitator
        .settle(
            &mut gateway,
            &principal,
            &request,
            stable,
            step,
            &mut plane,
            &trace,
            0,
        )
        .unwrap_or_else(|error| panic!("pending settle: {error}"));
    assert!(!pending.success);
    assert_eq!(pending.error_reason.as_deref(), Some("settlement_pending"));
    assert_eq!(pending.transaction, format!("0x{}", hex(&PRIMARY_ACTIVITY)));
    assert_eq!(
        pending.extensions["layerx"]["verificationLevel"],
        "submitted"
    );
    assert_eq!(
        gateway.translation(&principal, identity.idempotency_key),
        Some(TranslationStatus::Pending)
    );

    // Crash mid-settlement: still no economic effect, translation stays open.
    let crashed = facilitator.settle(
        &mut gateway,
        &principal,
        &request,
        stable,
        step,
        &mut plane,
        &trace,
        1,
    );
    assert!(crashed.is_err());
    assert_eq!(
        gateway.translation(&principal, identity.idempotency_key),
        Some(TranslationStatus::Pending)
    );

    // Recovery: the retry confirms exactly one receipt-verified effect.
    let confirmed = facilitator
        .settle(
            &mut gateway,
            &principal,
            &request,
            stable,
            step,
            &mut plane,
            &trace,
            2,
        )
        .unwrap_or_else(|error| panic!("recovery settle: {error}"));
    assert!(confirmed.success);
    let digest = settled_digest(&gateway, &principal, identity.idempotency_key);

    // Post-success duplicate: idempotent, no new effect, same digest.
    let duplicate = facilitator
        .settle(
            &mut gateway,
            &principal,
            &request,
            stable,
            step,
            &mut plane,
            &trace,
            3,
        )
        .unwrap_or_else(|error| panic!("duplicate settle: {error}"));
    assert!(duplicate.success);
    assert_eq!(confirmed.transaction, duplicate.transaction);
    assert_eq!(
        settled_digest(&gateway, &principal, identity.idempotency_key),
        digest,
        "the recorded economic effect is unchanged by the duplicate"
    );
    assert_eq!(plane.settle_calls, 4);
}

#[test]
fn a_swapped_receipt_for_a_settled_identity_is_refused() {
    let facilitator = facilitator();
    let mut gateway = GatewayCore::new();
    let trace = trace();
    register_x402(&mut gateway, &trace);
    let principal = principal();
    let request = facilitator_request();
    let stable = stable_identity();
    let step = SettlementStep::Single;
    let identity = SettlementIdentity::derive(&principal, &request, stable, step)
        .unwrap_or_else(|error| panic!("identity: {error}"));

    // First confirm the primary receipt, then a fault redelivers a *different*
    // receipt (same asset, recipient and amount, different activity) under the
    // already-settled identity.
    let mut plane = FaultInjectingPlane::new(vec![Planned::Execute, Planned::ExecuteAlternate]);

    let confirmed = facilitator
        .settle(
            &mut gateway,
            &principal,
            &request,
            stable,
            step,
            &mut plane,
            &trace,
            0,
        )
        .unwrap_or_else(|error| panic!("primary settle: {error}"));
    assert!(confirmed.success);
    let digest = settled_digest(&gateway, &principal, identity.idempotency_key);

    let swapped = facilitator.settle(
        &mut gateway,
        &principal,
        &request,
        stable,
        step,
        &mut plane,
        &trace,
        1,
    );
    assert!(
        swapped.is_err(),
        "a conflicting receipt under a settled identity must be refused"
    );
    assert_eq!(
        settled_digest(&gateway, &principal, identity.idempotency_key),
        digest,
        "the original economic effect is preserved unchanged"
    );
}

fn hex(bytes: &[u8; 32]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut output = String::with_capacity(64);
    for byte in bytes {
        output.push(char::from(DIGITS[usize::from(byte >> 4)]));
        output.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    output
}
