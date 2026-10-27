use ed25519_dalek::{Signer as _, SigningKey};
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
    ExecutedUcpPayment, MerchantProfile, NegotiatedCapabilities, OrderMetadata, PaymentHandler,
    PlatformProfile, StoredOrder, UcpAdapter, UcpError, UcpIdempotencyKey, UcpOrder,
    UCP_CHECKOUT_SPEC_SHA256,
};
use serde_json::Value;
use sha2::{Digest as _, Sha256};

const VECTORS: &str = include_str!("../../../specs/conformance/ucp/sequenced-order.json");
const RECORDED: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");

fn string<'a>(value: &'a Value, key: &str) -> &'a str {
    value[key]
        .as_str()
        .unwrap_or_else(|| panic!("missing string {key}"))
}

fn hex(value: &Value) -> Vec<u8> {
    let encoded = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .unwrap_or_else(|| panic!("missing hex value"));
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
        .unwrap_or_else(|| panic!("missing log array"))
        .iter()
        .map(|log| {
            SequencerLog::new(
                array(&log["address"]),
                log["topics"]
                    .as_array()
                    .unwrap_or_else(|| panic!("missing topics"))
                    .iter()
                    .map(array)
                    .collect(),
                hex(&log["data"]),
                u64::from_str_radix(
                    string(log, "logIndex")
                        .strip_prefix("0x")
                        .unwrap_or_else(|| panic!("missing hex log index")),
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

fn assert_payment_derivation(vector: &Value, source: &Value, submission: &CheckoutSubmission) {
    let name = string(vector, "name");
    let mut expected_logs = source["receiptJson"]["logs"].clone();
    let reference: [u8; 32] = array(&expected_logs[0]["topics"][3]);
    let gateway_key = submission.idempotency_key.gateway_key();
    assert_ne!(
        reference, gateway_key,
        "{name}: replace the recorded reference"
    );
    expected_logs[0]["topics"][3] = Value::String(format!(
        "0x{}",
        gateway_key
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    ));
    assert_eq!(
        vector["evidence"]["logs"], expected_logs,
        "{name}: derived logs"
    );

    let recorded = hex(&source["signedReceipt"]);
    let mut unsigned = recorded[..recorded.len() - 69].to_vec();
    let offsets: Vec<usize> = unsigned
        .windows(32)
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == reference).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "{name}: one signed payment reference");
    unsigned[offsets[0]..offsets[0] + 32].copy_from_slice(&gateway_key);
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    digest.update(&unsigned);
    digest.update([0]);
    let signer = SigningKey::from_bytes(&array(&vector["testReceiptKeySeed"]));
    assert_eq!(
        signer.verifying_key().to_bytes(),
        array(&vector["pinnedKey"]["publicKey"]),
        "{name}: published test receipt key"
    );
    let signature = signer.sign(&digest.finalize()).to_bytes();
    unsigned.extend_from_slice(&[1, 0, 0, 0, 64]);
    unsigned.extend_from_slice(&signature);
    assert_eq!(
        hex(&vector["signedReceipt"]),
        unsigned,
        "{name}: re-signed receipt"
    );
}

#[test]
fn ucp_sequenced_order_vectors_replay() {
    let vectors: Vec<Value> = serde_json::from_str(VECTORS)
        .unwrap_or_else(|error| panic!("sequenced order vectors: {error}"));
    let recorded: Value =
        serde_json::from_str(RECORDED).unwrap_or_else(|error| panic!("recorded receipts: {error}"));
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
        let source = recorded["cases"]
            .as_array()
            .unwrap_or_else(|| panic!("missing recorded cases"))
            .iter()
            .find(|case| case["id"] == vector["source_case"])
            .unwrap_or_else(|| panic!("{name}: missing recorded source"));
        for field in ["rawTransaction", "pinnedKey"] {
            assert_eq!(vector[field], source[field], "{name}: recorded {field}");
        }
        let submission = submission(&vector["checkout"]);
        if name == "tampered_log" {
            assert_eq!(vector["signedReceipt"], source["signedReceipt"]);
            assert_eq!(vector["evidence"]["logs"], source["receiptJson"]["logs"]);
        } else {
            assert_payment_derivation(vector, source, &submission);
        }
        assert_eq!(
            vector["evidence"]["transaction_hash"],
            source["receiptJson"]["txHash"]
        );
        let key = &vector["pinnedKey"];
        let keys = [SequencerAuthorization::new(
            array(&key["keyId"]),
            array(&key["publicKey"]),
            key["firstHeight"]
                .as_u64()
                .unwrap_or_else(|| panic!("first height")),
            key["lastHeight"]
                .as_u64()
                .unwrap_or_else(|| panic!("last height")),
        )];
        let mut payment = ExecutedUcpPayment {
            metadata: metadata(&vector["metadata"]),
            evidence: SequencerEvidence {
                transaction_hash: array(&vector["evidence"]["transaction_hash"]),
                signed_receipt: hex(&vector["signedReceipt"]),
                logs: logs(&vector["evidence"]["logs"]),
                rung: rung(string(&vector["evidence"], "rung")),
            },
        };
        let verified = verify_sequencer_signature(&payment.evidence.signed_receipt, &keys)
            .unwrap_or_else(|error| panic!("{name}: recorded signature: {error:?}"));
        assert_eq!(
            verified
                .protocol()
                .unwrap_or_else(|| panic!("protocol receipt"))
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
        let expected = expected(&vector["expected"]);
        let mut gateway = gateway(&trace, vectors.len() as u64);
        for now in [2, 3] {
            let actual = UcpAdapter::complete_checkout(
                &mut gateway,
                &principal,
                &submission,
                &mut payment,
                &keys,
                &trace,
                now,
            )
            .map_err(|error| error.into_error());
            if actual != expected {
                failures.push(format!(
                    "{name} at {now}: expected {:?}, got {:?}; payment reference {:?}, UCP reference {:?}",
                    expected.as_ref().map(|outcome| outcome.status),
                    actual.as_ref().map(|outcome| outcome.status),
                    payment.evidence.logs.first().and_then(|log| log.topics().get(3)),
                    submission.idempotency_key.gateway_key()
                ));
            }
            if let Ok(outcome) = &actual {
                if let Some(stored) = &outcome.stored_order {
                    assert_eq!(UcpAdapter::read_order(stored, &keys).ok(), outcome.order);
                    assert_eq!(
                        gateway.translation(&principal, submission.idempotency_key.gateway_key()),
                        Some(TranslationStatus::ReceiptVerified {
                            receipt_digest: outcome
                                .order
                                .as_ref()
                                .unwrap_or_else(|| panic!("missing order"))
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
