//! End-to-end buyer role tests against real service types. These tests verify
//! offer parsing, payment construction through typed plane paths, extension
//! echoing, receipt capture, and evidence-backed settlement verification.

use std::collections::BTreeMap;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use layerx_interop_gateway::trace::{TraceId, Traced};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_x402::buyer::{
    BuiltPayment, Buyer, BuyerPaymentPlane, PaymentBuildRequest, SupportedKind,
};
use layerx_x402::model::{
    account_identifiers, AtomicAmount, PaymentPayload, PaymentRequired, PaymentRequirements,
    ResourceInfo, SettlementResponse, X402Error, X402_VERSION,
};
use serde_json::{json, Value};

const PAYEE_ACCOUNT: &str = "agent:did:layerx:content-service:main";
const ALTERNATE_ACCOUNT: &str = "agent:did:layerx:content-mirror:main";
const CURRENCY: &str = "LXP";

fn account_id(account: &str) -> [u8; 32] {
    account_identifiers(account)
        .unwrap_or_else(|error| panic!("{account} has account identifiers: {error}"))[0]
}

fn pay_to(account: &str) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut text = String::with_capacity(66);
    text.push_str("0x");
    for byte in account_id(account) {
        text.push(char::from(DIGITS[usize::from(byte >> 4)]));
        text.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    text
}

fn layerx_terms(account: &str) -> Value {
    json!({"layerx": {"commitment": "executed", "account": account, "currency": CURRENCY}})
}

struct TestBuyerPlane {
    payload: Value,
}

impl BuyerPaymentPlane for TestBuyerPlane {
    fn construct(&mut self, _request: PaymentBuildRequest) -> Result<Value, X402Error> {
        Ok(self.payload.clone())
    }
}

fn test_supported() -> Vec<SupportedKind> {
    vec![
        SupportedKind {
            scheme: "exact".to_owned(),
            network: "eip155:125".to_owned(),
        },
        SupportedKind {
            scheme: "402lxp".to_owned(),
            network: "eip155:1".to_owned(),
        },
    ]
}

fn test_requirements() -> PaymentRequirements {
    PaymentRequirements {
        scheme: "exact".to_owned(),
        network: "eip155:125".to_owned(),
        amount: AtomicAmount::from_u128(500),
        asset: "0x".to_owned() + &"12".repeat(32),
        pay_to: pay_to(PAYEE_ACCOUNT),
        max_timeout_seconds: 90,
        extra: Some(layerx_terms(PAYEE_ACCOUNT)),
    }
}

fn test_payment_required() -> PaymentRequired {
    PaymentRequired {
        x402_version: X402_VERSION,
        error: None,
        resource: ResourceInfo {
            url: "https://service.example/content".to_owned(),
            description: Some("Protected content".to_owned()),
            mime_type: Some("application/json".to_owned()),
            service_name: Some("Content Service".to_owned()),
            tags: vec!["content".to_owned()],
            icon_url: None,
        },
        accepts: vec![test_requirements()],
        extensions: BTreeMap::new(),
    }
}

fn encode_payment_required(required: &PaymentRequired) -> String {
    STANDARD.encode(
        serde_json::to_vec(required).unwrap_or_else(|error| panic!("test input: {error:?}")),
    )
}

fn mock_receipt_bytes() -> Vec<u8> {
    receipt_fixture_bytes(&receipt_fixture()["signedReceipt"])
}

fn receipt_fixture() -> Value {
    let fixture: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap_or_else(|error| panic!("receipt fixture: {error}"));
    fixture["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("fixture cases"))
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .unwrap_or_else(|| panic!("recorded settlement"))
        .clone()
}

fn receipt_fixture_bytes(value: &Value) -> Vec<u8> {
    let digits = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .unwrap_or_else(|| panic!("fixture hexadecimal bytes"));
    assert_eq!(digits.len() % 2, 0);
    (0..digits.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(&digits[index..index + 2], 16).unwrap())
        .collect()
}

fn mock_authorized_batch() -> AuthorizedBatch {
    AuthorizedBatch::new([1; 32], [0x12; 32], [2; 32], [3; 32], [4; 32])
}

fn pinned_receipt_keys() -> [SequencerAuthorization; 1] {
    let fixture: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap_or_else(|error| panic!("receipt fixture: {error}"));
    let key = &fixture["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("fixture cases"))
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .unwrap_or_else(|| panic!("recorded settlement"))["pinnedKey"];
    let field = |name: &str| {
        key[name]
            .as_str()
            .and_then(|value| value.strip_prefix("0x"))
            .unwrap_or_else(|| panic!("pinned key {name}"))
    };
    [SequencerAuthorization::from_config(
        field("keyId"),
        field("publicKey"),
        &key["firstHeight"].to_string(),
        &key["lastHeight"].to_string(),
    )
    .unwrap_or_else(|error| panic!("receipt authorization: {error}"))]
}

fn payment_log_settlement(
    scheme: &str,
    logged_idempotency_key: [u8; 32],
) -> (
    BuiltPayment,
    SettlementResponse,
    [SequencerAuthorization; 1],
) {
    let mut accepted = test_requirements();
    accepted.scheme = scheme.to_owned();
    if scheme == "metered" {
        accepted.extra.as_mut().unwrap()["layerx"]["purposeHash"] = json!(CHALLENGE_PURPOSE);
    }
    signed_payment_log_settlement(accepted, logged_idempotency_key)
}

fn signed_payment_log_settlement(
    accepted: PaymentRequirements,
    logged_idempotency_key: [u8; 32],
) -> (
    BuiltPayment,
    SettlementResponse,
    [SequencerAuthorization; 1],
) {
    use ed25519_dalek::{Signer as _, SigningKey};
    use sha2::{Digest as _, Sha256};

    let scheme = accepted.scheme.clone();
    let fixture = receipt_fixture();
    let recorded_log = &fixture["receiptJson"]["logs"][0];
    let address = receipt_fixture_bytes(&recorded_log["address"]);
    let mut recorded_body = address.clone();
    recorded_body.push(4);
    for topic in recorded_log["topics"].as_array().unwrap() {
        recorded_body.extend_from_slice(&receipt_fixture_bytes(topic));
    }
    recorded_body.extend_from_slice(&Sha256::digest(receipt_fixture_bytes(
        &recorded_log["data"],
    )));

    let grant = [0x8a; 32];
    let payer = [0x6a; 32];
    let idempotency_key = [0x5c; 32];
    let mut data = receipt_fixture_bytes(&json!(accepted.asset));
    data.extend_from_slice(&[0; 16]);
    data.extend_from_slice(&accepted.amount.value().to_be_bytes());
    let topic = if scheme == "metered" {
        data.extend_from_slice(&logged_idempotency_key);
        data.extend_from_slice(&[0x9b; 32]);
        "13473d103a2d52172016c872eeae0153e254af3c21a1d987492ab737f1a34fe1"
    } else {
        "c4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1"
    };
    let topics = json!([
        format!("0x{topic}"),
        format!(
            "0x{}",
            hex32(&if scheme == "metered" {
                grant
            } else {
                idempotency_key
            })
        ),
        format!("0x{}", hex32(&payer)),
        pay_to(PAYEE_ACCOUNT),
    ]);
    let mut body = address;
    body.push(4);
    for topic in topics.as_array().unwrap() {
        body.extend_from_slice(&receipt_fixture_bytes(topic));
    }
    body.extend_from_slice(&Sha256::digest(&data));
    assert_eq!(body.len(), recorded_body.len());
    let mut receipt = mock_receipt_bytes();
    let positions: Vec<_> = receipt
        .windows(recorded_body.len())
        .enumerate()
        .filter_map(|(index, bytes)| (bytes == recorded_body).then_some(index))
        .collect();
    assert_eq!(positions.len(), 1);
    receipt[positions[0]..positions[0] + body.len()].copy_from_slice(&body);
    let digest = layerx_interop_gateway::evidence::canonical_receipt_digest(&receipt).unwrap();
    let signer = SigningKey::from_bytes(&[0x72; 32]);
    let signature_start = receipt.len() - 64;
    receipt[signature_start..].copy_from_slice(&signer.sign(&digest).to_bytes());
    let [pinned] = pinned_receipt_keys();
    let keys = [SequencerAuthorization::new(
        pinned.sequencer_id(),
        signer.verifying_key().to_bytes(),
        pinned.first_batch_number(),
        pinned.last_batch_number(),
    )];
    let payment = BuiltPayment {
        header: String::new(),
        payload: PaymentPayload {
            x402_version: X402_VERSION,
            resource: None,
            payload: json!({"grantId": hex32(&grant), "idempotencyKey": hex32(&idempotency_key)}),
            accepted,
            extensions: BTreeMap::new(),
        },
        idempotency_key,
    };
    let mut evidence = json!({
        "receipt": STANDARD.encode(&receipt),
        "receiptDigest": hex32(&digest),
        "verificationLevel": "receipted",
        "logs": [{
            "address": recorded_log["address"],
            "topics": topics,
            "data": format!("0x{}", data.iter().map(|byte| format!("{byte:02x}")).collect::<String>()),
            "logIndex": "0x0",
        }],
    });
    if scheme == "metered" {
        evidence["purposeHash"] = json!(CHALLENGE_PURPOSE);
    }
    let response = SettlementResponse {
        success: true,
        error_reason: None,
        payer: Some(hex32(&payer)),
        transaction: fixture["receiptJson"]["txHash"]
            .as_str()
            .unwrap()
            .to_owned(),
        network: payment.payload.accepted.network.clone(),
        amount: Some(payment.payload.accepted.amount),
        extensions: BTreeMap::from([("layerx".to_owned(), evidence)]),
    };
    (payment, response, keys)
}

#[test]
fn buyer_captures_a_metered_settlement_from_a_payment_drawn_log() {
    for scheme in ["metered", "exact"] {
        let (mut payment, response, keys) = payment_log_settlement(scheme, [0x5c; 32]);
        for grant_id in ["8a".repeat(32), format!("0x{}", "8A".repeat(32))] {
            payment.payload.payload["grantId"] = json!(grant_id);
            let captured = Buyer::capture_settlement(
                &STANDARD.encode(serde_json::to_vec(&response).unwrap()),
                &payment,
                &keys,
                &TraceId::mint([0xab; 16]),
            )
            .unwrap_or_else(|error| panic!("{scheme} settlement: {error:?}"));
            assert_eq!(captured.response, response);
            assert_eq!(captured.rung, "receipted");
            assert_eq!(
                captured.canonical_receipt,
                STANDARD
                    .decode(response.extensions["layerx"]["receipt"].as_str().unwrap())
                    .unwrap()
            );
            assert_eq!(
                hex32(&captured.receipt_digest),
                response.extensions["layerx"]["receiptDigest"]
            );
        }
    }
}

#[test]
fn buyer_refuses_a_metered_settlement_whose_idempotency_word_differs() {
    let (payment, response, keys) = payment_log_settlement("metered", [0x5d; 32]);
    let header = STANDARD.encode(serde_json::to_vec(&response).unwrap());
    let trace = TraceId::mint([0xab; 16]);
    assert_eq!(
        Buyer::capture_settlement(&header, &payment, &keys, &trace).map_err(Traced::into_error),
        Err(X402Error::EvidenceMismatch),
    );
    let mut matching_payment = payment;
    matching_payment.idempotency_key = [0x5d; 32];
    matching_payment.payload.payload["idempotencyKey"] = json!("5d".repeat(32));
    let captured = Buyer::capture_settlement(&header, &matching_payment, &keys, &trace)
        .unwrap_or_else(|error| panic!("matching idempotency word: {error:?}"));
    assert_eq!(captured.response, response);
}

#[test]
fn buyer_validates_supported_kinds_on_construction() {
    let valid = test_supported();
    assert!(Buyer::new(valid).is_ok());

    assert!(Buyer::new(vec![]).is_err());

    let duplicate = vec![
        SupportedKind {
            scheme: "exact".to_owned(),
            network: "eip155:125".to_owned(),
        },
        SupportedKind {
            scheme: "exact".to_owned(),
            network: "eip155:125".to_owned(),
        },
    ];
    assert!(Buyer::new(duplicate).is_err());

    let too_many = (0..100)
        .map(|i| SupportedKind {
            scheme: format!("scheme{i}"),
            network: "eip155:125".to_owned(),
        })
        .collect();
    assert!(Buyer::new(too_many).is_err());
}

#[test]
fn buyer_refuses_unsupported_offer() {
    let supported = test_supported();
    let buyer = Buyer::new(supported).unwrap_or_else(|error| panic!("valid supported: {error:?}"));

    let mut required = test_payment_required();
    required.accepts[0].scheme = "unsupported".to_owned();

    let encoded = encode_payment_required(&required);
    let mut plane = TestBuyerPlane {
        payload: json!({"scheme": "exact"}),
    };
    let trace = TraceId::mint([0xab; 16]);

    let result = buyer.build_payment(&encoded, [1; 32], &mut plane, &trace);

    assert!(result.is_err());
}

#[test]
fn buyer_selects_first_supported_offer_in_seller_order() {
    let supported = test_supported();
    let buyer = Buyer::new(supported).unwrap_or_else(|error| panic!("valid supported: {error:?}"));

    let mut required = test_payment_required();
    let unsupported = PaymentRequirements {
        scheme: "unsupported".to_owned(),
        network: "eip155:125".to_owned(),
        amount: AtomicAmount::from_u128(100),
        asset: "0x".to_owned() + &"aa".repeat(32),
        pay_to: pay_to(ALTERNATE_ACCOUNT),
        max_timeout_seconds: 60,
        extra: Some(layerx_terms(ALTERNATE_ACCOUNT)),
    };
    let exact_supported = test_requirements();

    required.accepts = vec![unsupported, exact_supported.clone()];

    let encoded = encode_payment_required(&required);
    let mut plane = TestBuyerPlane {
        payload: json!({"authorization": "test"}),
    };
    let trace = TraceId::mint([0xab; 16]);

    let payment = buyer
        .build_payment(&encoded, [1; 32], &mut plane, &trace)
        .unwrap_or_else(|error| panic!("payment built: {error:?}"));

    assert_eq!(payment.payload.accepted.scheme, "exact");
    assert_eq!(payment.payload.accepted.network, "eip155:125");
}

#[test]
fn buyer_echoes_required_extensions_byte_for_value() {
    let supported = test_supported();
    let buyer = Buyer::new(supported).unwrap_or_else(|error| panic!("valid supported: {error:?}"));

    let mut required = test_payment_required();
    required.extensions.insert(
        "custom".to_owned(),
        layerx_x402::model::Extension {
            info: json!({"key": "value", "number": 42}),
            schema: json!({"type": "object"}),
        },
    );

    let encoded = encode_payment_required(&required);
    let mut plane = TestBuyerPlane {
        payload: json!({"authorization": "test"}),
    };
    let trace = TraceId::mint([0xab; 16]);

    let payment = buyer
        .build_payment(&encoded, [1; 32], &mut plane, &trace)
        .unwrap_or_else(|error| panic!("payment built: {error:?}"));

    assert_eq!(payment.payload.extensions, required.extensions);
}

#[test]
fn buyer_refuses_zero_idempotency_key() {
    let supported = test_supported();
    let buyer = Buyer::new(supported).unwrap_or_else(|error| panic!("valid supported: {error:?}"));

    let required = test_payment_required();
    let encoded = encode_payment_required(&required);
    let mut plane = TestBuyerPlane {
        payload: json!({"authorization": "test"}),
    };
    let trace = TraceId::mint([0xab; 16]);

    let result = buyer.build_payment(&encoded, [0; 32], &mut plane, &trace);

    assert!(result.is_err());
}

#[test]
fn buyer_validates_payment_required_header_before_parsing() {
    let supported = test_supported();
    let buyer = Buyer::new(supported).unwrap_or_else(|error| panic!("valid supported: {error:?}"));

    let invalid_header = "not-valid-base64!";
    let mut plane = TestBuyerPlane {
        payload: json!({"authorization": "test"}),
    };
    let trace = TraceId::mint([0xab; 16]);

    let result = buyer.build_payment(invalid_header, [1; 32], &mut plane, &trace);

    assert!(result.is_err());
}

#[test]
fn buyer_refuses_wrong_x402_version() {
    let supported = test_supported();
    let buyer = Buyer::new(supported).unwrap_or_else(|error| panic!("valid supported: {error:?}"));

    let mut required = test_payment_required();
    required.x402_version = 1;

    let encoded = encode_payment_required(&required);
    let mut plane = TestBuyerPlane {
        payload: json!({"authorization": "test"}),
    };
    let trace = TraceId::mint([0xab; 16]);

    let result = buyer.build_payment(&encoded, [1; 32], &mut plane, &trace);

    assert!(result.is_err());
}

#[test]
fn buyer_includes_resource_info_in_built_payment() {
    let supported = test_supported();
    let buyer = Buyer::new(supported).unwrap_or_else(|error| panic!("valid supported: {error:?}"));

    let required = test_payment_required();
    let encoded = encode_payment_required(&required);
    let mut plane = TestBuyerPlane {
        payload: json!({"authorization": "test"}),
    };
    let trace = TraceId::mint([0xab; 16]);

    let payment = buyer
        .build_payment(&encoded, [1; 32], &mut plane, &trace)
        .unwrap_or_else(|error| panic!("payment built: {error:?}"));

    assert!(payment.payload.resource.is_some());
    assert_eq!(
        payment
            .payload
            .resource
            .unwrap_or_else(|| panic!("payment resource missing"))
            .url,
        required.resource.url
    );
}

#[test]
fn buyer_payment_header_is_base64_encoded_json() {
    let supported = test_supported();
    let buyer = Buyer::new(supported).unwrap_or_else(|error| panic!("valid supported: {error:?}"));

    let required = test_payment_required();
    let encoded = encode_payment_required(&required);
    let mut plane = TestBuyerPlane {
        payload: json!({"authorization": "test"}),
    };
    let trace = TraceId::mint([0xab; 16]);

    let payment = buyer
        .build_payment(&encoded, [1; 32], &mut plane, &trace)
        .unwrap_or_else(|error| panic!("payment built: {error:?}"));

    let decoded = STANDARD
        .decode(payment.header.as_bytes())
        .unwrap_or_else(|error| panic!("valid base64: {error:?}"));
    let parsed: PaymentPayload = serde_json::from_slice(&decoded)
        .unwrap_or_else(|error| panic!("valid payment payload: {error:?}"));

    assert_eq!(parsed.x402_version, X402_VERSION);
    assert_eq!(parsed.accepted, test_requirements());
}

#[test]
fn buyer_plane_request_contains_all_requirements() {
    struct CaptureBuyerPlane {
        captured: Option<PaymentBuildRequest>,
    }

    impl BuyerPaymentPlane for CaptureBuyerPlane {
        fn construct(&mut self, request: PaymentBuildRequest) -> Result<Value, X402Error> {
            self.captured = Some(request);
            Ok(json!({"test": "payload"}))
        }
    }

    let supported = test_supported();
    let buyer = Buyer::new(supported).unwrap_or_else(|error| panic!("valid supported: {error:?}"));

    let required = test_payment_required();
    let encoded = encode_payment_required(&required);
    let mut plane = CaptureBuyerPlane { captured: None };
    let trace = TraceId::mint([0xab; 16]);

    let _payment = buyer
        .build_payment(&encoded, [5; 32], &mut plane, &trace)
        .unwrap_or_else(|error| panic!("payment built: {error:?}"));

    let captured = plane.captured.unwrap_or_else(|| panic!("plane was called"));
    assert_eq!(captured.requirements, test_requirements());
    assert_eq!(captured.idempotency_key, [5; 32]);
}

#[test]
fn buyer_refuses_non_object_scheme_payload() {
    let supported = test_supported();
    let buyer = Buyer::new(supported).unwrap_or_else(|error| panic!("valid supported: {error:?}"));

    let required = test_payment_required();
    let encoded = encode_payment_required(&required);
    let mut plane = TestBuyerPlane {
        payload: json!("not-an-object"),
    };
    let trace = TraceId::mint([0xab; 16]);

    let result = buyer.build_payment(&encoded, [1; 32], &mut plane, &trace);

    assert!(result.is_err());
}

#[test]
fn buyer_built_payment_preserves_idempotency_key() {
    let supported = test_supported();
    let buyer = Buyer::new(supported).unwrap_or_else(|error| panic!("valid supported: {error:?}"));

    let required = test_payment_required();
    let encoded = encode_payment_required(&required);
    let mut plane = TestBuyerPlane {
        payload: json!({"authorization": "test"}),
    };
    let trace = TraceId::mint([0xab; 16]);

    let key = [7; 32];
    let payment = buyer
        .build_payment(&encoded, key, &mut plane, &trace)
        .unwrap_or_else(|error| panic!("payment built: {error:?}"));

    assert_eq!(payment.idempotency_key, key);
}

#[test]
fn buyer_capture_refuses_failed_settlement_as_success() {
    let payment = BuiltPayment {
        header: "test".to_owned(),
        payload: PaymentPayload {
            x402_version: X402_VERSION,
            resource: None,
            payload: json!({"test": "data"}),
            accepted: test_requirements(),
            extensions: BTreeMap::new(),
        },
        idempotency_key: [1; 32],
    };

    let failed = SettlementResponse {
        success: false,
        error_reason: Some("payment_refused".to_owned()),
        payer: None,
        transaction: format!("0x{}", "61".repeat(32)),
        network: "eip155:125".to_owned(),
        amount: None,
        extensions: BTreeMap::new(),
    };

    let encoded = STANDARD.encode(
        serde_json::to_vec(&failed).unwrap_or_else(|error| panic!("test input: {error:?}")),
    );
    let _batch = mock_authorized_batch();
    let trace = TraceId::mint([0xab; 16]);

    let result = Buyer::capture_settlement(&encoded, &payment, &pinned_receipt_keys(), &trace);

    assert!(result.is_err());
}

#[test]
fn buyer_capture_refuses_missing_layerx_evidence() {
    let payment = BuiltPayment {
        header: "test".to_owned(),
        payload: PaymentPayload {
            x402_version: X402_VERSION,
            resource: None,
            payload: json!({"test": "data"}),
            accepted: test_requirements(),
            extensions: BTreeMap::new(),
        },
        idempotency_key: [1; 32],
    };

    let no_evidence = SettlementResponse {
        success: true,
        error_reason: None,
        payer: Some("0x".to_owned() + &"ab".repeat(32)),
        transaction: format!("0x{}", "61".repeat(32)),
        network: "eip155:125".to_owned(),
        amount: Some(AtomicAmount::from_u128(500)),
        extensions: BTreeMap::new(),
    };

    let encoded = STANDARD.encode(
        serde_json::to_vec(&no_evidence).unwrap_or_else(|error| panic!("test input: {error:?}")),
    );
    let _batch = mock_authorized_batch();
    let trace = TraceId::mint([0xab; 16]);

    let result = Buyer::capture_settlement(&encoded, &payment, &pinned_receipt_keys(), &trace);

    assert!(result.is_err());
}

#[test]
fn buyer_capture_refuses_wrong_verification_level() {
    let payment = BuiltPayment {
        header: "test".to_owned(),
        payload: PaymentPayload {
            x402_version: X402_VERSION,
            resource: None,
            payload: json!({"test": "data"}),
            accepted: test_requirements(),
            extensions: BTreeMap::new(),
        },
        idempotency_key: [1; 32],
    };

    let mut extensions = BTreeMap::new();
    extensions.insert(
        "layerx".to_owned(),
        json!({
            "receipt": STANDARD.encode(mock_receipt_bytes()),
            "receiptDigest": "ab".repeat(32),
            "verificationLevel": "unverified"
        }),
    );

    let wrong_level = SettlementResponse {
        success: true,
        error_reason: None,
        payer: Some("0x".to_owned() + &"ab".repeat(32)),
        transaction: format!("0x{}", "61".repeat(32)),
        network: "eip155:125".to_owned(),
        amount: Some(AtomicAmount::from_u128(500)),
        extensions,
    };

    let encoded = STANDARD.encode(
        serde_json::to_vec(&wrong_level).unwrap_or_else(|error| panic!("test input: {error:?}")),
    );
    let _batch = mock_authorized_batch();
    let trace = TraceId::mint([0xab; 16]);

    let result = Buyer::capture_settlement(&encoded, &payment, &pinned_receipt_keys(), &trace);

    assert!(result.is_err());
}

#[test]
fn buyer_capture_refuses_malformed_receipt() {
    let payment = BuiltPayment {
        header: "test".to_owned(),
        payload: PaymentPayload {
            x402_version: X402_VERSION,
            resource: None,
            payload: json!({"test": "data"}),
            accepted: test_requirements(),
            extensions: BTreeMap::new(),
        },
        idempotency_key: [1; 32],
    };

    let mut extensions = BTreeMap::new();
    extensions.insert(
        "layerx".to_owned(),
        json!({
            "receipt": "not-valid-base64!",
            "receiptDigest": "ab".repeat(32),
            "verificationLevel": "sequencer-signed"
        }),
    );

    let bad_receipt = SettlementResponse {
        success: true,
        error_reason: None,
        payer: Some("0x".to_owned() + &"ab".repeat(32)),
        transaction: format!("0x{}", "61".repeat(32)),
        network: "eip155:125".to_owned(),
        amount: Some(AtomicAmount::from_u128(500)),
        extensions,
    };

    let encoded = STANDARD.encode(
        serde_json::to_vec(&bad_receipt).unwrap_or_else(|error| panic!("test input: {error:?}")),
    );
    let _batch = mock_authorized_batch();
    let trace = TraceId::mint([0xab; 16]);

    let result = Buyer::capture_settlement(&encoded, &payment, &pinned_receipt_keys(), &trace);

    assert!(result.is_err());
}

#[test]
fn supported_kind_equality_matches_both_scheme_and_network() {
    let kind1 = SupportedKind {
        scheme: "exact".to_owned(),
        network: "eip155:125".to_owned(),
    };
    let kind2 = SupportedKind {
        scheme: "exact".to_owned(),
        network: "eip155:125".to_owned(),
    };
    let kind3 = SupportedKind {
        scheme: "402lxp".to_owned(),
        network: "eip155:125".to_owned(),
    };

    assert_eq!(kind1, kind2);
    assert_ne!(kind1, kind3);
}

#[test]
fn buyer_validates_payment_payload_after_construction() {
    let supported = test_supported();
    let buyer = Buyer::new(supported).unwrap_or_else(|error| panic!("valid supported: {error:?}"));

    let required = test_payment_required();
    let encoded = encode_payment_required(&required);
    let mut plane = TestBuyerPlane {
        payload: json!({"valid": "object"}),
    };
    let trace = TraceId::mint([0xab; 16]);

    let payment = buyer
        .build_payment(&encoded, [1; 32], &mut plane, &trace)
        .unwrap_or_else(|error| panic!("payment built: {error:?}"));

    assert!(payment.payload.validate().is_ok());
}

#[test]
fn buyer_refuses_a_layerx_offer_that_carries_no_quote_terms() {
    let buyer =
        Buyer::new(test_supported()).unwrap_or_else(|error| panic!("valid supported: {error:?}"));
    let mut required = test_payment_required();
    required.accepts[0].extra = None;

    let encoded = encode_payment_required(&required);
    let mut plane = TestBuyerPlane {
        payload: json!({"authorization": "test"}),
    };
    let trace = TraceId::mint([0xab; 16]);

    let refusal = buyer
        .build_payment(&encoded, [1; 32], &mut plane, &trace)
        .err()
        .map(Traced::into_error);

    assert_eq!(refusal, Some(X402Error::ProfileMissing));
}

#[test]
fn buyer_refuses_a_layerx_offer_paying_another_accounts_identifier() {
    let buyer =
        Buyer::new(test_supported()).unwrap_or_else(|error| panic!("valid supported: {error:?}"));
    let mut required = test_payment_required();
    required.accepts[0].pay_to = pay_to(ALTERNATE_ACCOUNT);

    let encoded = encode_payment_required(&required);
    let mut plane = TestBuyerPlane {
        payload: json!({"authorization": "test"}),
    };
    let trace = TraceId::mint([0xab; 16]);

    let refusal = buyer
        .build_payment(&encoded, [1; 32], &mut plane, &trace)
        .err()
        .map(Traced::into_error);

    assert_eq!(refusal, Some(X402Error::ProfileMismatch));
}

const SETTLED_ASSET: [u8; 32] = [0xab; 32];
const SETTLED_PAYER: [u8; 32] = [0x6a; 32];
const CHALLENGE_PURPOSE: &str = "7c3e6a0d5b1f48e29c0a6d3b7e1f5a2c9d8b4e6f0a1c3d5e7f9b2a4c6e8d0f13";
const OTHER_PURPOSE: &str = "0d8e6c4a2b9f7e5d3c1a0f6e4b8d9c2a5f1e7b3d0a6c5e4b2d9f8a1c3e7b6d5f";

fn hex32(bytes: &[u8; 32]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut text = String::with_capacity(64);
    for byte in bytes {
        text.push(char::from(DIGITS[usize::from(byte >> 4)]));
        text.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    text
}

fn settled_offer(scheme: &str, purpose: Option<&str>) -> PaymentRequirements {
    let mut layerx = json!({
        "commitment": "executed",
        "account": PAYEE_ACCOUNT,
        "currency": CURRENCY,
        "payer": hex32(&SETTLED_PAYER),
    });
    if let (Some(purpose), Some(terms)) = (purpose, layerx.as_object_mut()) {
        terms.insert("purposeHash".to_owned(), json!(purpose));
    }
    PaymentRequirements {
        scheme: scheme.to_owned(),
        amount: AtomicAmount::from_u128(1000),
        asset: "0x".to_owned() + &hex32(&SETTLED_ASSET),
        extra: Some(json!({ "layerx": layerx })),
        ..test_requirements()
    }
}

fn capture(
    offer: PaymentRequirements,
    repeated: Option<&str>,
) -> Result<layerx_x402::buyer::CapturedSettlement, X402Error> {
    let (payment, mut response, keys) = signed_payment_log_settlement(offer, [0x5c; 32]);
    let layerx = response
        .extensions
        .get_mut("layerx")
        .unwrap()
        .as_object_mut()
        .unwrap();
    layerx.remove("purposeHash");
    if let Some(purpose) = repeated {
        layerx.insert("purposeHash".to_owned(), json!(purpose));
    }
    Buyer::capture_settlement(
        &STANDARD.encode(serde_json::to_vec(&response).unwrap()),
        &payment,
        &keys,
        &TraceId::mint([0xab; 16]),
    )
    .map_err(Traced::into_error)
}

#[test]
fn buyer_capture_accepts_a_grant_settlement_repeating_the_challenge_purpose() {
    let (_, response, _) = signed_payment_log_settlement(
        settled_offer("metered", Some(CHALLENGE_PURPOSE)),
        [0x5c; 32],
    );
    let receipt = STANDARD
        .decode(response.extensions["layerx"]["receipt"].as_str().unwrap())
        .unwrap();
    let captured = capture(
        settled_offer("metered", Some(CHALLENGE_PURPOSE)),
        Some(CHALLENGE_PURPOSE),
    )
    .unwrap_or_else(|error| panic!("grant settlement: {error:?}"));
    assert_eq!(captured.canonical_receipt, receipt);
    assert_eq!(
        captured.response.extensions["layerx"]["purposeHash"],
        CHALLENGE_PURPOSE
    );
    assert_eq!(
        captured.response.transaction,
        receipt_fixture()["receiptJson"]["txHash"]
    );
}

#[test]
fn buyer_capture_refuses_a_grant_settlement_that_omits_the_purpose() {
    assert_eq!(
        capture(settled_offer("metered", Some(CHALLENGE_PURPOSE)), None),
        Err(X402Error::EvidenceMismatch)
    );
    assert_eq!(
        capture(settled_offer("subscription", Some(CHALLENGE_PURPOSE)), None),
        Err(X402Error::EvidenceMismatch)
    );
}

#[test]
fn buyer_capture_refuses_a_grant_settlement_repeating_another_purpose() {
    for repeated in [OTHER_PURPOSE, &CHALLENGE_PURPOSE.to_ascii_uppercase()] {
        assert_eq!(
            capture(
                settled_offer("metered", Some(CHALLENGE_PURPOSE)),
                Some(repeated)
            ),
            Err(X402Error::EvidenceMismatch),
            "{repeated}"
        );
    }
    assert_eq!(
        capture(settled_offer("metered", None), Some(CHALLENGE_PURPOSE)),
        Err(X402Error::EvidenceMismatch)
    );
}

#[test]
fn buyer_capture_refuses_an_exact_settlement_carrying_a_purpose() {
    capture(settled_offer("exact", None), None)
        .unwrap_or_else(|error| panic!("exact settlement: {error:?}"));
    assert_eq!(
        capture(settled_offer("exact", None), Some(CHALLENGE_PURPOSE)),
        Err(X402Error::EvidenceMismatch)
    );
}
