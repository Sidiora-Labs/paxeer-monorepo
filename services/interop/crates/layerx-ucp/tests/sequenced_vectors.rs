use ed25519_dalek::SigningKey;
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite, PinnedSpec, SpecVersion};
use layerx_interop_gateway::evidence::{SequencerEvidence, SettlementRung};
use layerx_interop_gateway::gateway::TranslationStatus;
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_ucp::{
    ucp_adapter_descriptor, CheckoutOutcome, CheckoutStatus, CheckoutSubmission,
    ExecutedUcpPayment, MerchantCurrencyBinding, MerchantProfile, NegotiatedCapabilities,
    OrderMetadata, PaymentHandler, PlatformProfile, StoredOrder, UcpAdapter, UcpError,
    UcpIdempotencyKey, UcpOrder, UcpServiceConfig, UCP_CHECKOUT_SPEC_SHA256,
};
use serde_json::Value;
use sha2::{Digest as _, Sha256};

const VECTORS: &str = include_str!("fixtures/sequenced-order.json");
const RECORDED: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/ucp-checkout-receipt-v1.json");
const NEGATIVE_RECORDED: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");

fn string<'a>(value: &'a Value, key: &str) -> &'a str {
    value[key]
        .as_str()
        .or_panic(&format!("missing string {key}"))
}

fn hex(value: &Value) -> Vec<u8> {
    let encoded = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .or_panic("missing hex value");
    assert_eq!(encoded.len() % 2, 0);
    (0..encoded.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&encoded[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("invalid hex: {error}"))
        })
        .collect()
}

fn array<const N: usize>(value: &Value) -> [u8; N] {
    hex(value)
        .try_into()
        .unwrap_or_else(|_| panic!("expected {N} bytes"))
}

fn logs(value: &Value) -> Vec<SequencerLog> {
    value
        .as_array()
        .or_panic("missing log array")
        .iter()
        .map(|log| {
            SequencerLog::new(
                array(&log["address"]),
                log["topics"]
                    .as_array()
                    .or_panic("missing topics")
                    .iter()
                    .map(array)
                    .collect(),
                hex(&log["data"]),
                u64::from_str_radix(
                    string(log, "logIndex")
                        .strip_prefix("0x")
                        .or_panic("missing hex log index"),
                    16,
                )
                .unwrap_or_else(|error| panic!("invalid log index: {error}")),
            )
        })
        .collect()
}

fn rung(value: &str) -> SettlementRung {
    match value {
        "submitted" => SettlementRung::Submitted,
        "receipted" => SettlementRung::Receipted,
        other => panic!("unsupported rung: {other}"),
    }
}

fn currency(value: &Value) -> [u8; 3] {
    string(value, "currency")
        .as_bytes()
        .try_into()
        .unwrap_or_else(|_| panic!("expected three currency bytes"))
}

fn amount(value: &Value) -> u128 {
    string(value, "total_minor")
        .parse()
        .unwrap_or_else(|error| panic!("invalid minor amount: {error}"))
}

fn submission(value: &Value) -> CheckoutSubmission {
    assert_eq!(string(value, "status"), "ready_for_complete");
    let declaration = &value["payment_handler"];
    let handler = PaymentHandler::new(
        string(declaration, "id"),
        string(declaration, "version"),
        string(declaration, "spec"),
        string(declaration, "schema"),
    )
    .unwrap_or_else(|error| panic!("payment handler: {error}"));
    let merchant = MerchantProfile::layerx(string(value, "rest_endpoint"), handler.clone())
        .unwrap_or_else(|error| panic!("merchant: {error}"));
    let platform = PlatformProfile {
        profile_url: string(value, "platform_profile_url").to_owned(),
        capabilities: merchant.capabilities().to_vec(),
        payment_handlers: vec![handler.clone()],
    };
    CheckoutSubmission {
        checkout_id: string(value, "id").to_owned(),
        currency: currency(value),
        total_minor: amount(value),
        layerx_asset: array(&value["layerx_asset"]),
        layerx_recipient: array(&value["layerx_recipient"]),
        idempotency_key: UcpIdempotencyKey::parse(string(value, "idempotency_key"))
            .unwrap_or_else(|error| panic!("idempotency key: {error}")),
        negotiated: NegotiatedCapabilities::negotiate(&platform, &handler)
            .unwrap_or_else(|error| panic!("negotiation: {error}")),
    }
}

fn metadata(value: &Value) -> OrderMetadata {
    OrderMetadata {
        order_id: string(value, "order_id").to_owned(),
        permalink_url: string(value, "permalink_url").to_owned(),
    }
}

fn expected(value: &Value) -> Result<CheckoutOutcome, UcpError> {
    if !value["refusal"].is_null() {
        assert_eq!(string(value, "refusal"), "ReceiptMismatch");
        assert!(value["status"].is_null());
        assert!(value["order"].is_null());
        assert!(value["stored_order"].is_null());
        return Err(UcpError::ReceiptMismatch);
    }
    let status = match string(value, "status") {
        "completed" => CheckoutStatus::Completed,
        "complete_in_progress" => CheckoutStatus::CompleteInProgress,
        other => panic!("unsupported expected status: {other}"),
    };
    let order = (!value["order"].is_null()).then(|| {
        let order = &value["order"];
        UcpOrder {
            id: string(order, "id").to_owned(),
            checkout_id: string(order, "checkout_id").to_owned(),
            permalink_url: string(order, "permalink_url").to_owned(),
            currency: currency(order),
            total_minor: amount(order),
            receipt_digest: array(&order["receipt_digest"]),
            transaction_hash: array(&order["transaction_hash"]),
        }
    });
    let stored_order = (!value["stored_order"].is_null()).then(|| {
        let stored = &value["stored_order"];
        StoredOrder {
            submission: submission(&stored["checkout"]),
            metadata: metadata(&stored["metadata"]),
            transaction_hash: array(&stored["transaction_hash"]),
            signed_receipt: hex(&stored["signed_receipt"]),
            logs: logs(&stored["logs"]),
            rung: rung(string(stored, "rung")),
        }
    });
    assert_eq!(order.is_some(), status == CheckoutStatus::Completed);
    assert_eq!(stored_order.is_some(), status == CheckoutStatus::Completed);
    Ok(CheckoutOutcome {
        status,
        order,
        stored_order,
    })
}

fn gateway(trace: &TraceId, count: u64) -> GatewayCore {
    let spec = PinnedSpec::new(
        AdapterId::new("ucp").unwrap_or_else(|error| panic!("adapter id: {error}")),
        SpecVersion::parse("20260408").unwrap_or_else(|error| panic!("version: {error}")),
        UCP_CHECKOUT_SPEC_SHA256,
    )
    .unwrap_or_else(|error| panic!("spec: {error}"));
    let suite = ConformanceSuite::new(
        AdapterId::new("ucp-sequenced-order-vectors")
            .unwrap_or_else(|error| panic!("suite id: {error}")),
        count,
        Sha256::digest(VECTORS.as_bytes()).into(),
    )
    .unwrap_or_else(|error| panic!("suite: {error}"));
    let descriptor =
        ucp_adapter_descriptor(spec, suite).unwrap_or_else(|error| panic!("descriptor: {error}"));
    let mut gateway = GatewayCore::new();
    gateway
        .register_adapter(descriptor, trace, 1)
        .unwrap_or_else(|error| panic!("register adapter: {error}"));
    gateway
}

fn payment_exact_words(case: &Value) -> [[u8; 32]; 3] {
    let transaction = hex(&case["rawTransaction"]);
    let offsets: Vec<usize> = transaction
        .windows(4)
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == [0x78, 0xea, 0x11, 0xdb]).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "one recorded payExact selector");
    let calldata = &transaction[offsets[0] + 4..];
    [5, 0, 1].map(|word| {
        calldata[word * 32..(word + 1) * 32]
            .try_into()
            .unwrap_or_else(|_| panic!("recorded payExact word {word}"))
    })
}

fn assert_payment_derivation(vector: &Value, source: &Value, submission: &CheckoutSubmission) {
    let name = string(vector, "name");
    let [reference, from, to] = payment_exact_words(source);
    let signer = SigningKey::from_bytes(&array(&vector["testReceiptKeySeed"]));
    assert_eq!(
        signer.verifying_key().to_bytes(),
        array(&vector["pinnedKey"]["publicKey"]),
        "{name}: published test receipt key"
    );
    assert_eq!(
        reference,
        submission.idempotency_key.gateway_key(),
        "{name}: native transaction must contain the checkout gateway key"
    );
    assert_eq!(
        vector["rawTransaction"], source["rawTransaction"],
        "{name}: unchanged producer transaction"
    );
    assert_eq!(
        vector["evidence"]["logs"], source["receiptJson"]["logs"],
        "{name}: unchanged producer logs"
    );
    assert_eq!(
        vector["signedReceipt"], source["signedReceipt"],
        "{name}: unchanged producer receipt and signature"
    );
    let topics = &source["receiptJson"]["logs"][0]["topics"];
    assert_eq!(
        array::<32>(&topics[1]),
        reference,
        "{name}: producer reference"
    );
    assert_eq!(array::<32>(&topics[2]), from, "{name}: producer payer");
    assert_eq!(array::<32>(&topics[3]), to, "{name}: producer recipient");
}

#[test]
fn ucp_sequenced_order_vectors_replay() {
    let vectors: Vec<Value> = serde_json::from_str(VECTORS)
        .unwrap_or_else(|error| panic!("sequenced order vectors: {error}"));
    let recorded: Value =
        serde_json::from_str(RECORDED).unwrap_or_else(|error| panic!("recorded receipts: {error}"));
    let negative_recorded: Value = serde_json::from_str(NEGATIVE_RECORDED)
        .unwrap_or_else(|error| panic!("negative recorded receipts: {error}"));
    assert_eq!(
        vectors
            .iter()
            .map(|v| string(v, "name"))
            .collect::<Vec<_>>(),
        ["receipted_order", "submitted_order", "tampered_log"]
    );
    let trace = TraceId::mint([0x14; 16]);
    let principal = PrincipalId::new("sequenced-vector-buyer")
        .unwrap_or_else(|error| panic!("principal: {error}"));
    let mut failures = Vec::new();
    for vector in &vectors {
        let name = string(vector, "name");
        let recording = if name == "tampered_log" {
            &negative_recorded
        } else {
            &recorded
        };
        let source = recording["cases"]
            .as_array()
            .or_panic("missing recorded cases")
            .iter()
            .find(|case| case["id"] == vector["source_case"])
            .or_panic(&format!("{name}: missing recorded source"));
        for field in ["rawTransaction", "pinnedKey"] {
            assert_eq!(vector[field], source[field], "{name}: recorded {field}");
        }
        let submission = submission(&vector["checkout"]);
        let config = UcpServiceConfig::new(
            MerchantCurrencyBinding::new(submission.currency, submission.layerx_asset, 2, 6)
                .or_panic("expected Ok value"),
        );
        assert_source_recording(name, vector, source, &submission);
        let (mut payment, keys) = recorded_payment(name, vector, source);
        let expected = expected(&vector["expected"]);
        let mut gateway = gateway(&trace, vectors.len() as u64);
        for now in [2, 3] {
            let actual = UcpAdapter::complete_checkout(
                &mut gateway,
                &principal,
                &submission,
                &mut config.bind_plane(&mut payment),
                &keys,
                &trace,
                now,
            )
            .map_err(layerx_interop_gateway::trace::Traced::into_error);
            if actual != expected {
                failures.push(format!(
                    "{name} at {now}: expected {:?}, got {:?}; payment reference {:?}, UCP reference {:?}",
                    expected.as_ref().map(|outcome| outcome.status),
                    actual.as_ref().map(|outcome| outcome.status),
                    payment.evidence.logs.first().and_then(|log| log.topics().get(1)),
                    submission.idempotency_key.gateway_key()
                ));
            }
            if let Ok(outcome) = &actual {
                if let Some(stored) = &outcome.stored_order {
                    assert_eq!(
                        UcpAdapter::read_order_with_config(stored, &keys, &config).ok(),
                        outcome.order
                    );
                    assert_eq!(
                        gateway.translation(&principal, submission.idempotency_key.gateway_key()),
                        Some(TranslationStatus::ReceiptVerified {
                            receipt_digest: outcome
                                .order
                                .as_ref()
                                .or_panic("missing order")
                                .receipt_digest
                        })
                    );
                } else {
                    assert!(
                        outcome.order.is_none(),
                        "{name}: open checkout has no order"
                    );
                    assert_eq!(
                        gateway.translation(&principal, submission.idempotency_key.gateway_key()),
                        Some(TranslationStatus::Pending)
                    );
                }
            } else {
                assert_eq!(
                    gateway.translation(&principal, submission.idempotency_key.gateway_key()),
                    Some(TranslationStatus::Pending),
                    "{name}: refused evidence must not settle the checkout"
                );
            }
        }
    }
    assert!(failures.is_empty(), "{}", failures.join("\n"));
}

fn assert_source_recording(
    name: &str,
    vector: &Value,
    source: &Value,
    submission: &CheckoutSubmission,
) {
    if name == "tampered_log" {
        assert_eq!(vector["signedReceipt"], source["signedReceipt"]);
        assert_eq!(vector["evidence"]["logs"], source["receiptJson"]["logs"]);
    } else {
        assert_payment_derivation(vector, source, submission);
    }
    assert_eq!(
        vector["evidence"]["transaction_hash"],
        source["receiptJson"]["txHash"]
    );
}

fn recorded_payment(
    name: &str,
    vector: &Value,
    source: &Value,
) -> (ExecutedUcpPayment, [SequencerAuthorization; 1]) {
    let key = &vector["pinnedKey"];
    let keys = [SequencerAuthorization::new(
        array(&key["keyId"]),
        array(&key["publicKey"]),
        key["firstHeight"].as_u64().or_panic("first height"),
        key["lastHeight"].as_u64().or_panic("last height"),
    )];
    let payment = ExecutedUcpPayment {
        metadata: metadata(&vector["metadata"]),
        evidence: SequencerEvidence {
            transaction_hash: array(&vector["evidence"]["transaction_hash"]),
            signed_receipt: hex(&vector["signedReceipt"]),
            logs: logs(&vector["evidence"]["logs"]),
            rung: rung(string(&vector["evidence"], "rung")),
        },
    };
    let verified = verify_sequencer_signature(&payment.evidence.signed_receipt, &keys)
        .or_panic(&format!("{name}: recorded signature"));
    assert_eq!(
        verified
            .protocol()
            .or_panic("protocol receipt")
            .activity_id(),
        payment.evidence.transaction_hash
    );
    let verified_logs = verify_logs(&verified, payment.evidence.logs.clone());
    if name == "tampered_log" {
        assert_eq!(string(source, "vector"), "V3");
        assert!(
            verified_logs.is_err(),
            "{name}: tampering must fail log verification"
        );
    } else {
        assert!(verified_logs.is_ok(), "{name}: recorded logs must verify");
    }
    (payment, keys)
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
