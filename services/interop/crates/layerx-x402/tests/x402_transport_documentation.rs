//! Task 22.33.107 repair record, audit 22.30.111, finding 40 (doc-drift).
//! Finding 40 is proven by `finding_40_matrix_matches_production_codecs`,
//! `finding_40_identity_survives_transport`, and `finding_40_qualification_stays_pending`.
//! These tests exercise production codecs and identity derivation with committed
//! role fixtures; they do not qualify durable recovery or native economic effects.
//! Baseline: no earlier gate ledger or repair test existed; the controlling
//! implementation-first hf-build/hf-done order forbids `do_5`'s pre-change run.
//! No baseline failure is claimed. Exact verified revision, command, exit and
//! private log are recorded by hf-done and copied into the final task evidence.
//! Wiring request: services/interop/crates/layerx-interop-service (domain 11)
//! must qualify a real service stop/reopen/retry against persisted payment state.
//! Wiring request: modules/layerxledger (domain 2) must supply authoritative native
//! balance/payment observations for that same restart and duplicate-delivery path.

use std::{collections::BTreeSet, fmt::Debug};

use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_x402::{
    facilitator::{
        Facilitator, FacilitatorRequest, SettlementIdentity, SettlementStep, SupportedResponse,
        VerifyResponse,
    },
    model::{
        PaymentPayload, PaymentRequired, SettlementResponse, X402Error, PAYMENT_REQUIRED_HEADER,
        PAYMENT_RESPONSE_HEADER, PAYMENT_SIGNATURE_HEADER,
    },
    transport::{
        decode_facilitator_request, decode_facilitator_settlement, decode_payment_payload,
        decode_payment_required, decode_settlement, decode_supported_response,
        decode_verify_response, encode_facilitator_request, encode_facilitator_settlement,
        encode_payment_payload, encode_payment_required, encode_settlement,
        encode_supported_response, encode_verify_response, TransportKind, TransportValue,
        TRANSPORT_MATRIX,
    },
    Seller,
};
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use serde_json::Value;

const DOCUMENT: &str = include_str!("../COMPATIBILITY.md");
const SUITES: [(TransportKind, &str, &str); 3] = [
    (
        TransportKind::Http,
        "HTTP",
        include_str!("../../../specs/conformance/transport-http/role-messages.json"),
    ),
    (
        TransportKind::Mcp,
        "MCP",
        include_str!("../../../specs/conformance/transport-mcp/role-messages.json"),
    ),
    (
        TransportKind::A2a,
        "A2A",
        include_str!("../../../specs/conformance/transport-a2a/role-messages.json"),
    ),
];

#[derive(Deserialize)]
struct RoleVector {
    name: String,
    role: String,
    accepted: bool,
    representation: String,
    header: Option<String>,
    document: Value,
}

impl RoleVector {
    fn wire(&self) -> TransportValue {
        match self.representation.as_str() {
            "json" => TransportValue::Json(self.document.clone()),
            "http-header" => TransportValue::HttpHeader {
                name: match self.header.as_deref().or_panic("expected Some value") {
                    PAYMENT_REQUIRED_HEADER => PAYMENT_REQUIRED_HEADER,
                    PAYMENT_SIGNATURE_HEADER => PAYMENT_SIGNATURE_HEADER,
                    PAYMENT_RESPONSE_HEADER => PAYMENT_RESPONSE_HEADER,
                    other => panic!("unexpected payment header {other}"),
                },
                value: STANDARD
                    .encode(serde_json::to_vec(&self.document).or_panic("expected Ok value")),
            },
            other => panic!("unexpected representation {other}"),
        }
    }
}

fn round_trip<T: DeserializeOwned + Serialize + Debug + PartialEq>(
    transport: TransportKind,
    vector: &RoleVector,
    encode: fn(TransportKind, &T) -> Result<TransportValue, X402Error>,
    decode: fn(TransportKind, &TransportValue) -> Result<T, X402Error>,
) {
    let wire = vector.wire();
    if let Some(expected) = profile_refusal(&vector.name) {
        let model: T =
            serde_json::from_value(vector.document.clone()).or_panic("expected Ok value");
        assert_eq!(encode(transport, &model), Err(expected));
        assert_eq!(decode(transport, &wire), Err(expected));
    } else if vector.accepted {
        let model: T =
            serde_json::from_value(vector.document.clone()).or_panic("expected Ok value");
        let produced = encode(transport, &model).or_panic("expected Ok value");
        match (&produced, &wire) {
            (
                TransportValue::HttpHeader { name, value },
                TransportValue::HttpHeader { name: expected, .. },
            ) => {
                assert_eq!(name, expected);
                let document: Value =
                    serde_json::from_slice(&STANDARD.decode(value).or_panic("expected Ok value"))
                        .or_panic("expected Ok value");
                assert_eq!(document, vector.document);
            }
            (TransportValue::Json(_), TransportValue::Json(_)) => assert_eq!(produced, wire),
            _ => panic!("{} representation mismatch", vector.role),
        }
        assert_eq!(
            decode(transport, &produced).or_panic("expected Ok value"),
            model
        );
        assert_eq!(
            decode(transport, &wire).or_panic("expected Ok value"),
            model
        );
    } else {
        assert!(
            decode(transport, &wire).is_err(),
            "{} must be refused",
            vector.role
        );
    }
}

fn assert_role_transport(transport: TransportKind, vector: &RoleVector) {
    match vector.role.as_str() {
        "payment-required" => {
            round_trip::<PaymentRequired>(
                transport,
                vector,
                encode_payment_required,
                decode_payment_required,
            );
            if vector.accepted && profile_refusal(&vector.name).is_none() {
                let seller = Seller::new(
                    serde_json::from_value(vector.document.clone()).or_panic("expected Ok value"),
                )
                .or_panic("expected Ok value");
                let signal = seller.payment_required().or_panic("expected Ok value");
                assert_eq!(signal.status, 402);
                let produced = if transport == TransportKind::Http {
                    TransportValue::HttpHeader {
                        name: PAYMENT_REQUIRED_HEADER,
                        value: signal.header,
                    }
                } else {
                    encode_payment_required(transport, &signal.body).or_panic("expected Ok value")
                };
                assert_eq!(
                    decode_payment_required(transport, &produced).or_panic("expected Ok value"),
                    signal.body
                );
            }
        }
        "payment-payload" => round_trip::<PaymentPayload>(
            transport,
            vector,
            encode_payment_payload,
            decode_payment_payload,
        ),
        "settlement-response" => round_trip::<SettlementResponse>(
            transport,
            vector,
            encode_settlement,
            decode_settlement,
        ),
        "facilitator-request" => round_trip::<FacilitatorRequest>(
            transport,
            vector,
            encode_facilitator_request,
            decode_facilitator_request,
        ),
        "verify-response" => round_trip::<VerifyResponse>(
            transport,
            vector,
            encode_verify_response,
            decode_verify_response,
        ),
        "facilitator-settlement" => round_trip::<SettlementResponse>(
            transport,
            vector,
            encode_facilitator_settlement,
            decode_facilitator_settlement,
        ),
        "supported-response" => {
            round_trip::<SupportedResponse>(
                transport,
                vector,
                encode_supported_response,
                decode_supported_response,
            );
            let facilitator = Facilitator::new(
                serde_json::from_value(vector.document.clone()).or_panic("expected Ok value"),
            )
            .or_panic("expected Ok value");
            let produced = encode_supported_response(transport, &facilitator.supported())
                .or_panic("expected Ok value");
            assert_eq!(
                decode_supported_response(transport, &produced).or_panic("expected Ok value"),
                facilitator.supported()
            );
        }
        other => panic!("uncovered role {other}"),
    }
}

#[test]
fn finding_40_matrix_matches_production_codecs() {
    assert_eq!(TRANSPORT_MATRIX.len(), SUITES.len());
    for (row, (transport, label, suite)) in TRANSPORT_MATRIX.iter().zip(SUITES) {
        assert_eq!(row.transport, transport);
        assert!(row.buyer && row.seller && row.facilitator);
        assert!(DOCUMENT.contains(&format!("| {label} | yes | yes | yes |")));
        let vectors: Vec<RoleVector> = serde_json::from_str(suite).or_panic("expected Ok value");
        let mut roles = BTreeSet::new();
        let mut refused = BTreeSet::new();
        for vector in &vectors {
            if vector.accepted {
                roles.insert(vector.role.as_str());
            } else {
                refused.insert(vector.name.as_str());
            }
            assert_role_transport(transport, vector);
        }
        assert_eq!(
            roles,
            BTreeSet::from([
                "payment-required",
                "payment-payload",
                "settlement-response",
                "facilitator-request",
                "verify-response",
                "facilitator-settlement",
                "supported-response"
            ])
        );
        let expected_refusals = match transport {
            TransportKind::Http => {
                BTreeSet::from(["json_payment_required_is_not_an_http_representation"])
            }
            TransportKind::Mcp => {
                BTreeSet::from(["http_header_payment_required_is_not_a_mcp_representation"])
            }
            TransportKind::A2a => BTreeSet::from([
                "http_header_payment_required_is_not_a_a2a_representation",
                "profile_free_payment_required",
                "profile_free_payment_payload",
                "profile_free_facilitator_request",
            ]),
        };
        assert_eq!(refused, expected_refusals);
    }
}

#[test]
fn finding_40_identity_survives_transport() {
    let principal = PrincipalId::new("finding-40-merchant").or_panic("expected Ok value");
    let mut identities = Vec::new();
    for (transport, _, suite) in SUITES {
        let vectors: Vec<RoleVector> = serde_json::from_str(suite).or_panic("expected Ok value");
        let vector = vectors
            .iter()
            .find(|v| v.role == "facilitator-request" && v.accepted)
            .or_panic("expected Some value");
        let request: FacilitatorRequest =
            serde_json::from_value(vector.document.clone()).or_panic("expected Ok value");
        let produced =
            encode_facilitator_request(transport, &request).or_panic("expected Ok value");
        let received =
            decode_facilitator_request(transport, &produced).or_panic("expected Ok value");
        let identity =
            SettlementIdentity::derive(&principal, &received, [9; 32], SettlementStep::Single)
                .or_panic("expected Ok value");
        let deposit = SettlementIdentity::derive(
            &principal,
            &received,
            [9; 32],
            SettlementStep::EscrowDeposit,
        )
        .or_panic("expected Ok value");
        let charge = SettlementIdentity::derive(
            &principal,
            &received,
            [9; 32],
            SettlementStep::EscrowCharge,
        )
        .or_panic("expected Ok value");
        assert_ne!(identity.idempotency_key, deposit.idempotency_key);
        assert_ne!(deposit.idempotency_key, charge.idempotency_key);
        assert_ne!(deposit.request_digest, charge.request_digest);
        identities.push(identity);
    }
    assert_eq!(identities[0], identities[1]);
    assert_eq!(identities[1], identities[2]);
}

#[test]
fn finding_40_qualification_stays_pending() {
    for claim in [
        "Durable recovery across service restart",
        "Exactly-once economic effect",
        "Upstream reference conformance",
        "Independently operated facilitator interoperability",
    ] {
        let rows: Vec<_> = DOCUMENT
            .lines()
            .filter(|line| line.starts_with(&format!("| {claim} |")))
            .collect();
        assert_eq!(rows.len(), 1, "one qualification status for {claim}");
        assert_eq!(
            rows[0]
                .split('|')
                .nth(2)
                .or_panic("expected Some value")
                .trim(),
            "pending"
        );
    }
    let text = DOCUMENT.split_whitespace().collect::<Vec<_>>().join(" ");
    for boundary in [
        "HTTP row covers header and JSON-body codecs",
        "MCP and A2A rows cover JSON message bindings",
        "do not start HTTP, MCP or A2A services",
        "retain the same `GatewayCore` across retries",
        "does not terminate a process, reopen persistent state or observe a funded ledger across restart",
        "not an independently measured chain balance",
        "Promote durable recovery and economic-effect claims only with real service restart and native ledger evidence from the same payment path",
        "`0x101a` on chain 125",
    ] {
        assert!(text.contains(boundary), "missing qualification boundary: {boundary}");
    }
}

fn profile_refusal(name: &str) -> Option<X402Error> {
    match name {
        "eip155_125_payment_required"
        | "eip155_125_payment_payload"
        | "eip155_125_facilitator_request" => Some(X402Error::ProfileMissing),
        _ => None,
    }
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
