//! Task 22.33.102 repair record (audit 22.30.111).
//! Finding 16: finding_16_bound_build_seller_capture,
//! finding_16_buyer_refuses_unbound_construction, and
//! finding_16_service_plane_refuses_conflicting_keys.
//! Finding 15: finding_15_all_transport_decoders_enforce_the_byte_budget,
//! finding_15_ingress_is_bounded_before_parsing, and
//! finding_15_escaped_json_counts_wire_bytes.
//! Baseline: not run because the explicit implementation-first and gate-only
//! order prohibits the contradictory baseline step; these tests are new.
//! No out-of-path wiring is required. The service uses the tested production
//! BoundBuyerPaymentPlane and decode_json_slice ingress boundary.
//! The runner records the verified revision, command, exit and private log.

use std::collections::BTreeMap;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_interop_gateway::{
    adapter::{AdapterId, ConformanceSuite},
    evidence::{SequencerEvidence, SettlementRung},
    gateway::TranslationStatus,
    principal::PrincipalId,
    trace::{TraceId, Traced},
    GatewayCore,
};
use layerx_proof::{
    inclusion::SequencerAuthorization,
    receipt::verify_sequencer_signature,
    sequencer_logs::{verify_logs, SequencerLog},
};
use layerx_wire::{encode::Encoder, limits::PROTOCOL_VERSION};
use layerx_x402::{
    buyer::{BoundBuyerPaymentPlane, Buyer, BuyerPaymentPlane, PaymentBuildRequest, SupportedKind},
    facilitator::{FacilitatorKind, FacilitatorRequest, SupportedResponse, VerifyResponse},
    model::{
        account_identifiers, AtomicAmount, Extension, PaymentPayload, PaymentRequired,
        PaymentRequirements, ResourceInfo, SettlementResponse, X402Error, PAYMENT_REQUIRED_HEADER,
        PAYMENT_RESPONSE_HEADER, PAYMENT_SIGNATURE_HEADER, X402_VERSION,
    },
    seller::{
        ExecutedPayment, LayerXPaymentRequest, PaymentPlane, PlanePaymentOutcome, Seller,
        SellerOutcome,
    },
    transport::{
        decode_facilitator_request, decode_facilitator_settlement, decode_json_slice,
        decode_payment_payload, decode_payment_required, decode_settlement,
        decode_supported_response, decode_verify_response, encode_facilitator_request,
        encode_facilitator_settlement, encode_payment_payload, encode_payment_required,
        encode_settlement, encode_supported_response, encode_verify_response, TransportKind,
        TransportValue,
    },
    x402_adapter_descriptor,
};
use serde::{de::DeserializeOwned, Serialize};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const ACCOUNT: &str = "agent:did:layerx:payee:main";
const ASSET: [u8; 32] = [0xab; 32];
const PAYER: [u8; 32] = [0x71; 32];
const REFERENCE: [u8; 32] = [0x74; 32];
const TRANSACTION: [u8; 32] = [0x61; 32];
const GRANT: [u8; 32] = [0x8a; 32];
const HEIGHT: u64 = 1000;
const LIMIT: usize = 64 * 1024;
const TRANSPORTS: [TransportKind; 3] =
    [TransportKind::Http, TransportKind::Mcp, TransportKind::A2a];

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn word(text: &str) -> [u8; 32] {
    assert_eq!(text.len(), 64);
    std::array::from_fn(|index| u8::from_str_radix(&text[index * 2..index * 2 + 2], 16).unwrap())
}

fn offer(scheme: &str) -> PaymentRequirements {
    let mut profile = json!({"account": ACCOUNT, "currency": "LXP"});
    if scheme != "exact" {
        profile["purposeHash"] = json!("7c".repeat(32));
    }
    PaymentRequirements {
        scheme: scheme.to_owned(),
        network: "eip155:125".to_owned(),
        amount: AtomicAmount::from_u128(1000),
        asset: hex(&ASSET),
        pay_to: hex(&account_identifiers(ACCOUNT).unwrap()[1]),
        max_timeout_seconds: 90,
        extra: Some(json!({"layerx": profile})),
    }
}

fn payment_log(scheme: &str, reference: [u8; 32]) -> SequencerLog {
    let topic = word(if scheme == "exact" {
        "c4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1"
    } else {
        "13473d103a2d52172016c872eeae0153e254af3c21a1d987492ab737f1a34fe1"
    });
    let mut address = [0; 20];
    address[18..].copy_from_slice(&[0x10, 0x1a]);
    let mut data = ASSET.to_vec();
    data.extend_from_slice(&[0; 16]);
    data.extend_from_slice(&1000_u128.to_be_bytes());
    if scheme != "exact" {
        data.extend_from_slice(&reference);
        // Native PaymentDrawn's last word is contextHash, not purposeHash.
        data.extend_from_slice(&[0x9b; 32]);
    }
    SequencerLog::new(
        address,
        vec![
            topic,
            if scheme == "exact" { reference } else { GRANT },
            PAYER,
            account_identifiers(ACCOUNT).unwrap()[1],
        ],
        data,
        0,
    )
}

fn sequence_hash() -> [u8; 32] {
    let mut hash = Sha256::new();
    hash.update(b"PAXEER/v1/sequence\0");
    hash.update([0x62; 32]);
    hash.update(9_u64.to_be_bytes());
    hash.update(TRANSACTION);
    hash.finalize().into()
}

fn encoded_receipt(logs: &[SequencerLog], signature: Option<[u8; 64]>) -> Vec<u8> {
    let mut summary = vec![0; 8 * 3 + 4 + 20 * 2];
    summary.extend_from_slice(&Sha256::digest(b""));
    summary.extend_from_slice(&0_u32.to_be_bytes());
    summary.extend_from_slice(&u32::try_from(logs.len()).unwrap().to_be_bytes());
    summary.push(4);
    summary.extend_from_slice(b"uhpx");
    let mut encoder = Encoder::new(16384);
    encoder
        .structure_header_version(0x5201, PROTOCOL_VERSION)
        .unwrap();
    encoder.u16(PROTOCOL_VERSION).unwrap();
    encoder.bytes(&TRANSACTION, 32).unwrap();
    encoder.u64(9).unwrap();
    for value in [[0x62; 32], sequence_hash(), [0x64; 32]] {
        encoder.bytes(&value, 32).unwrap();
    }
    encoder.i32(0).unwrap();
    let mut effects = vec![(1, summary)];
    for log in logs {
        let mut body = log.address().to_vec();
        body.push(u8::try_from(log.topics().len()).unwrap());
        for topic in log.topics() {
            body.extend_from_slice(topic);
        }
        body.extend_from_slice(&Sha256::digest(log.data()));
        effects.push((3, body));
    }
    encoder.sequence_length(effects.len(), 4096).unwrap();
    for (ordinal, (kind, body)) in effects.into_iter().enumerate() {
        encoder.u16(12).unwrap();
        encoder.u16(u16::try_from(ordinal).unwrap()).unwrap();
        encoder.u16(0).unwrap();
        encoder.u8(kind).unwrap();
        encoder.u8(0).unwrap();
        encoder.bytes(&[0; 32], 32).unwrap();
        encoder.bytes(&body, 256).unwrap();
    }
    let mut height = [0; 32];
    height[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    encoder.u128(1).unwrap();
    encoder.bytes(&height, 32).unwrap();
    encoder.u16(12).unwrap();
    encoder.u32(1).unwrap();
    encoder.u32(0).unwrap();
    encoder.u8(2).unwrap();
    encoder.bytes(&ASSET, 32).unwrap();
    encoder.u128(1000).unwrap();
    encoder.bytes(&PAYER, 32).unwrap();
    encoder.u128(5000).unwrap();
    encoder.u128(4000).unwrap();
    encoder.u64(1).unwrap();
    encoder
        .bytes(&account_identifiers(ACCOUNT).unwrap()[1], 32)
        .unwrap();
    encoder.u128(0).unwrap();
    encoder.u128(1000).unwrap();
    for value in [0x66, 0x67, 0x68] {
        encoder.bytes(&[value; 32], 32).unwrap();
    }
    encoder.u64(HEIGHT).unwrap();
    encoder.u8(u8::from(signature.is_some())).unwrap();
    if let Some(signature) = signature {
        encoder.bytes(&signature, 64).unwrap();
    }
    encoder.finish()
}

fn receipt_keys() -> Vec<SequencerAuthorization> {
    vec![SequencerAuthorization::new(
        [0x68; 32],
        SigningKey::from_bytes(&[0x72; 32])
            .verifying_key()
            .to_bytes(),
        HEIGHT,
        HEIGHT,
    )]
}

fn evidence(scheme: &str, reference: [u8; 32]) -> SequencerEvidence {
    let logs = vec![payment_log(scheme, reference)];
    let unsigned = encoded_receipt(&logs, None);
    let digest = layerx_wire::hash::receipt_digest(&unsigned).unwrap();
    let signature = SigningKey::from_bytes(&[0x72; 32]).sign(&digest).to_bytes();
    let signed_receipt = encoded_receipt(&logs, Some(signature));
    let decoded = layerx_wire::receipt::decode(&signed_receipt).unwrap();
    assert_eq!(
        layerx_wire::receipt::encode(&decoded).unwrap(),
        signed_receipt
    );
    let keys = receipt_keys();
    let receipt = verify_sequencer_signature(&signed_receipt, keys.as_slice()).unwrap();
    assert_eq!(verify_logs(&receipt, logs.clone()).unwrap().logs(), logs);
    SequencerEvidence {
        transaction_hash: TRANSACTION,
        signed_receipt,
        logs,
        rung: SettlementRung::Receipted,
    }
}

fn gateway() -> (GatewayCore, PrincipalId, TraceId) {
    let trace = TraceId::mint([0x73; 16]);
    let principal = PrincipalId::new("repair-buyer").unwrap();
    let suite = ConformanceSuite::new(AdapterId::new("x402-v2").unwrap(), 1, [0x75; 32]).unwrap();
    let mut gateway = GatewayCore::new();
    gateway
        .register_adapter(x402_adapter_descriptor(suite).unwrap(), &trace, 0)
        .unwrap();
    (gateway, principal, trace)
}

fn required(scheme: &str) -> PaymentRequired {
    PaymentRequired {
        x402_version: X402_VERSION,
        error: None,
        resource: ResourceInfo {
            url: "https://merchant.example/item".to_owned(),
            description: None,
            mime_type: None,
            service_name: None,
            tags: vec![],
            icon_url: None,
        },
        accepts: vec![offer(scheme)],
        extensions: BTreeMap::from([(
            "merchant".to_owned(),
            Extension {
                info: json!({"order": 27}),
                schema: json!({"type": "object"}),
            },
        )]),
    }
}

fn buyer(scheme: &str) -> Buyer {
    Buyer::with_receipt_keys(
        vec![SupportedKind {
            scheme: scheme.to_owned(),
            network: "eip155:125".to_owned(),
        }],
        receipt_keys(),
    )
    .unwrap()
}

fn scheme_payload() -> Value {
    json!({"transaction": format!("0x{}", hex(&TRANSACTION)), "grantId": hex(&GRANT)})
}

// A recorded receipt peer to the production seller. Every submitted term and
// identity must match the transcript before its real signed evidence is returned.
struct ReceiptTranscript {
    scheme: String,
    submissions: Vec<LayerXPaymentRequest>,
}

impl PaymentPlane for ReceiptTranscript {
    fn execute(
        &mut self,
        request: LayerXPaymentRequest,
        _: &TraceId,
    ) -> Result<PlanePaymentOutcome, X402Error> {
        let accepted = offer(&self.scheme);
        assert_eq!(request.scheme, self.scheme);
        assert_eq!(request.network, accepted.network);
        assert_eq!(request.amount, accepted.amount);
        assert_eq!(request.asset, accepted.asset);
        assert_eq!(request.pay_to, accepted.pay_to);
        assert_eq!(request.idempotency_key, REFERENCE);
        assert_eq!(request.scheme_payload["idempotencyKey"], hex(&REFERENCE));
        self.submissions.push(request);
        Ok(PlanePaymentOutcome::Executed(ExecutedPayment {
            evidence: evidence(&self.scheme, REFERENCE),
            committed: None,
        }))
    }
}

#[test]
fn finding_16_bound_build_seller_capture() {
    for scheme in ["exact", "metered", "subscription"] {
        for supplied in [
            None,
            Some(hex(&REFERENCE)),
            Some(format!("0x{}", hex(&REFERENCE).to_uppercase())),
        ] {
            let required = required(scheme);
            let seller = Seller::new(required.clone())
                .unwrap()
                .with_receipt_keys(receipt_keys());
            let signal = seller.payment_required().unwrap();
            let mut source = scheme_payload();
            if let Some(key) = supplied {
                source["idempotencyKey"] = json!(key);
            }
            let buyer = buyer(scheme);
            let mut construction = BoundBuyerPaymentPlane::new(source);
            let (mut gateway, principal, trace) = gateway();
            let built = buyer
                .build_payment(&signal.header, REFERENCE, &mut construction, &trace)
                .unwrap();
            assert_eq!(built.idempotency_key, REFERENCE);
            assert_eq!(built.payload.payload["idempotencyKey"], hex(&REFERENCE));
            assert_eq!(built.payload.extensions, required.extensions);
            let wire = TransportValue::HttpHeader {
                name: PAYMENT_SIGNATURE_HEADER,
                value: built.header.clone(),
            };
            assert_eq!(
                decode_payment_payload(TransportKind::Http, &wire).unwrap(),
                built.payload
            );
            let mut transcript = ReceiptTranscript {
                scheme: scheme.to_owned(),
                submissions: vec![],
            };
            let outcome = seller
                .settle(
                    &mut gateway,
                    &principal,
                    &built.header,
                    &mut transcript,
                    &trace,
                    1,
                )
                .unwrap();
            let SellerOutcome::Settled {
                header,
                response,
                receipt_digest,
            } = outcome
            else {
                panic!("the recorded receipted payment must settle");
            };
            assert_eq!(transcript.submissions.len(), 1);
            assert_eq!(
                transcript.submissions[0].idempotency_key,
                built.idempotency_key
            );
            assert_eq!(
                gateway.translation(&principal, REFERENCE),
                Some(TranslationStatus::ReceiptVerified { receipt_digest })
            );
            let captured = buyer.capture(&header, &built, &trace).unwrap();
            assert_eq!(captured.transaction_hash, TRANSACTION);
            assert_eq!(captured.receipt_digest, receipt_digest);
            assert_eq!(captured.response, response);
            assert_eq!(captured.rung, "receipted");
            assert_eq!(
                captured.canonical_receipt,
                evidence(scheme, REFERENCE).signed_receipt
            );
        }
    }
}

// These committed construction transcripts exercise the buyer's untrusted plane
// output boundary, independently of the service's key-binding implementation.
struct ConstructionTranscript(Value);
impl BuyerPaymentPlane for ConstructionTranscript {
    fn construct(&mut self, request: PaymentBuildRequest) -> Result<Value, X402Error> {
        assert_eq!(request.idempotency_key, REFERENCE);
        assert_eq!(request.requirements, offer(&request.requirements.scheme));
        Ok(self.0.take())
    }
}

#[test]
fn finding_16_buyer_refuses_unbound_construction() {
    for scheme in ["exact", "metered", "subscription"] {
        let signal = Seller::new(required(scheme))
            .unwrap()
            .payment_required()
            .unwrap();
        let buyer = buyer(scheme);
        for payload in [
            json!({}),
            json!({"idempotencyKey": "75".repeat(32)}),
            json!({"idempotencyKey": "00".repeat(32)}),
            json!({"idempotencyKey": "not-hex"}),
            json!({"idempotencyKey": 74}),
            json!({"idempotencyKey": null}),
            json!([]),
        ] {
            let result = buyer.build_payment(
                &signal.header,
                REFERENCE,
                &mut ConstructionTranscript(payload),
                &TraceId::mint([1; 16]),
            );
            assert_eq!(
                result.map_err(Traced::into_error),
                Err(X402Error::InvalidPayload)
            );
        }
    }
}

#[test]
fn finding_16_service_plane_refuses_conflicting_keys() {
    let buyer = buyer("exact");
    let signal = Seller::new(required("exact"))
        .unwrap()
        .payment_required()
        .unwrap();
    for payload in [
        json!({"idempotencyKey": "75".repeat(32)}),
        json!({"idempotencyKey": "00".repeat(32)}),
        json!({"idempotencyKey": "74"}),
        json!({"idempotencyKey": false}),
        json!({"idempotencyKey": null}),
        json!(null),
        json!([]),
    ] {
        let result = buyer.build_payment(
            &signal.header,
            REFERENCE,
            &mut BoundBuyerPaymentPlane::new(payload),
            &TraceId::mint([1; 16]),
        );
        assert_eq!(
            result.map_err(Traced::into_error),
            Err(X402Error::InvalidPayload)
        );
    }
    let result = buyer.build_payment(
        &signal.header,
        [0; 32],
        &mut BoundBuyerPaymentPlane::new(scheme_payload()),
        &TraceId::mint([1; 16]),
    );
    assert_eq!(
        result.map_err(Traced::into_error),
        Err(X402Error::InvalidPayload)
    );
}

fn payment() -> PaymentPayload {
    PaymentPayload {
        x402_version: X402_VERSION,
        resource: None,
        accepted: offer("exact"),
        payload: json!({"idempotencyKey": hex(&REFERENCE), "padding": ""}),
        extensions: BTreeMap::new(),
    }
}

fn settlement() -> SettlementResponse {
    SettlementResponse {
        success: true,
        error_reason: None,
        payer: Some(hex(&PAYER)),
        transaction: format!("0x{}", hex(&TRANSACTION)),
        network: "eip155:125".to_owned(),
        amount: Some(AtomicAmount::from_u128(1000)),
        extensions: BTreeMap::from([("padding".to_owned(), json!(""))]),
    }
}

type Encode<T> = fn(TransportKind, &T) -> Result<TransportValue, X402Error>;
type Decode<T> = fn(TransportKind, &TransportValue) -> Result<T, X402Error>;

fn assert_boundary<T>(
    template: T,
    padding: &str,
    header: Option<&'static str>,
    encode: Encode<T>,
    decode: Decode<T>,
) where
    T: Serialize + DeserializeOwned + PartialEq + std::fmt::Debug,
{
    let template = serde_json::to_value(template).unwrap();
    let overhead = serde_json::to_vec(&template).unwrap().len();
    for length in [LIMIT - 1, LIMIT, LIMIT + 1] {
        let mut document = template.clone();
        *document.pointer_mut(padding).unwrap() = json!("x".repeat(length - overhead));
        let bytes = serde_json::to_vec(&document).unwrap();
        assert_eq!(bytes.len(), length);
        let model: T = serde_json::from_value(document.clone()).unwrap();
        for transport in TRANSPORTS {
            if length <= LIMIT {
                let wire = encode(transport, &model).unwrap();
                assert_eq!(decode(transport, &wire).unwrap(), model);
                if header.is_none() || transport != TransportKind::Http {
                    let ingress = TransportValue::from_json_slice(&bytes).unwrap();
                    assert_eq!(decode(transport, &ingress).unwrap(), model);
                }
            } else {
                assert_eq!(encode(transport, &model), Err(X402Error::Bounds));
                let wire = match (transport, header) {
                    (TransportKind::Http, Some(name)) => TransportValue::HttpHeader {
                        name,
                        value: STANDARD.encode(&bytes),
                    },
                    _ => TransportValue::Json(document.clone()),
                };
                assert_eq!(decode(transport, &wire), Err(X402Error::Bounds));
                assert_eq!(
                    TransportValue::from_json_slice(&bytes),
                    Err(X402Error::Bounds)
                );
            }
        }
    }
}

#[test]
fn finding_15_all_transport_decoders_enforce_the_byte_budget() {
    let mut required = required("exact");
    required.extensions.insert(
        "padding".to_owned(),
        Extension {
            info: json!(""),
            schema: json!({"type": "string"}),
        },
    );
    assert_boundary(
        required,
        "/extensions/padding/info",
        Some(PAYMENT_REQUIRED_HEADER),
        encode_payment_required,
        decode_payment_required,
    );
    assert_boundary(
        payment(),
        "/payload/padding",
        Some(PAYMENT_SIGNATURE_HEADER),
        encode_payment_payload,
        decode_payment_payload,
    );
    assert_boundary(
        settlement(),
        "/extensions/padding",
        Some(PAYMENT_RESPONSE_HEADER),
        encode_settlement,
        decode_settlement,
    );
    assert_boundary(
        FacilitatorRequest {
            x402_version: X402_VERSION,
            payment_payload: payment(),
            payment_requirements: offer("exact"),
        },
        "/paymentPayload/payload/padding",
        None,
        encode_facilitator_request,
        decode_facilitator_request,
    );
    assert_boundary(
        settlement(),
        "/extensions/padding",
        None,
        encode_facilitator_settlement,
        decode_facilitator_settlement,
    );
    assert_boundary(
        VerifyResponse {
            is_valid: true,
            invalid_reason: None,
            payer: Some(hex(&PAYER)),
            extra: Some(json!({"padding": ""})),
        },
        "/extra/padding",
        None,
        encode_verify_response,
        decode_verify_response,
    );
    assert_boundary(
        SupportedResponse {
            kinds: vec![FacilitatorKind {
                x402_version: X402_VERSION,
                scheme: "exact".to_owned(),
                network: "eip155:125".to_owned(),
                extra: Some(json!({"padding": ""})),
            }],
            extensions: vec![],
            signers: BTreeMap::new(),
        },
        "/kinds/0/extra/padding",
        None,
        encode_supported_response,
        decode_supported_response,
    );
}

#[test]
fn finding_15_ingress_is_bounded_before_parsing() {
    for transport in TRANSPORTS {
        let wire = encode_payment_payload(transport, &payment()).unwrap();
        assert_eq!(decode_payment_payload(transport, &wire).unwrap(), payment());
    }
    // Even a syntactically invalid oversized body is refused by the size check.
    assert_eq!(
        TransportValue::from_json_slice(&vec![0xff; LIMIT + 1]),
        Err(X402Error::Bounds)
    );
    assert_eq!(
        TransportValue::from_json_slice(b"{invalid"),
        Err(X402Error::Decode)
    );
    let mut bytes = serde_json::to_vec(&payment()).unwrap();
    bytes.resize(LIMIT, b' ');
    let ingress = TransportValue::from_json_slice(&bytes).unwrap();
    for transport in [TransportKind::Mcp, TransportKind::A2a] {
        assert_eq!(
            decode_payment_payload(transport, &ingress).unwrap(),
            payment()
        );
    }
    bytes.push(b' ');
    assert_eq!(
        TransportValue::from_json_slice(&bytes),
        Err(X402Error::Bounds)
    );
    assert_eq!(
        decode_json_slice::<PaymentPayload>(&bytes),
        Err(X402Error::Bounds)
    );
    let duplicate = br#"{"x402Version":2,"x402Version":2}"#;
    assert_eq!(
        decode_json_slice::<PaymentPayload>(duplicate),
        Err(X402Error::Decode)
    );
}

#[test]
fn finding_15_escaped_json_counts_wire_bytes() {
    let mut payload = payment();
    payload.payload["padding"] = json!("\n".repeat(LIMIT / 2));
    let bytes = serde_json::to_vec(&payload).unwrap();
    assert!(bytes.len() > LIMIT);
    for transport in TRANSPORTS {
        let wire = match transport {
            TransportKind::Http => TransportValue::HttpHeader {
                name: PAYMENT_SIGNATURE_HEADER,
                value: STANDARD.encode(&bytes),
            },
            _ => TransportValue::Json(serde_json::to_value(&payload).unwrap()),
        };
        assert_eq!(
            decode_payment_payload(transport, &wire),
            Err(X402Error::Bounds)
        );
    }
    assert_eq!(
        TransportValue::from_json_slice(&bytes),
        Err(X402Error::Bounds)
    );
    payload.payload["padding"] = json!("界".repeat(LIMIT / 3));
    assert!(serde_json::to_vec(&payload).unwrap().len() > LIMIT);
    assert_eq!(
        decode_payment_payload(
            TransportKind::Mcp,
            &TransportValue::Json(serde_json::to_value(payload).unwrap())
        ),
        Err(X402Error::Bounds)
    );
}
