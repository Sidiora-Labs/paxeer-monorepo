use ed25519_dalek::{Signer as _, SigningKey};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite, PinnedSpec, SpecVersion};
use layerx_interop_gateway::evidence::{
    canonical_receipt_digest, SequencerEvidence, SettlementRung,
};
use layerx_interop_gateway::gateway::TranslationStatus;
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::SequencerLog;
use layerx_ucp::{
    ucp_adapter_descriptor, verify_sequencer_receipt, CheckoutStatus, CheckoutSubmission,
    ExecutedUcpPayment, MerchantProfile, NegotiatedCapabilities, OrderMetadata, PaymentHandler,
    PlatformProfile, UcpAdapter, UcpError, UcpIdempotencyKey, UCP_CHECKOUT_SPEC_SHA256,
};
use serde_json::Value;
use sha2::{Digest as _, Sha256};

const FIXTURE: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
));

fn fixture_case() -> Value {
    let fixture: Value =
        serde_json::from_str(FIXTURE).unwrap_or_else(|error| panic!("sequencer fixture: {error}"));
    fixture["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("missing fixture cases"))
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .unwrap_or_else(|| panic!("missing recorded pay-exact case"))
        .clone()
}

fn hex(value: &Value) -> Vec<u8> {
    let encoded = value
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .unwrap_or_else(|| panic!("missing fixture hex"));
    assert_eq!(encoded.len() % 2, 0);
    (0..encoded.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&encoded[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("invalid fixture hex: {error}"))
        })
        .collect()
}

fn array<const N: usize>(value: &Value) -> [u8; N] {
    hex(value)
        .try_into()
        .unwrap_or_else(|_| panic!("expected {N} fixture bytes"))
}

fn receipt_keys(case: &Value) -> [SequencerAuthorization; 1] {
    let key = &case["pinnedKey"];
    [SequencerAuthorization::new(
        array(&key["keyId"]),
        array(&key["publicKey"]),
        key["firstHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("missing first height")),
        key["lastHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("missing last height")),
    )]
}

fn submission(case: &Value) -> CheckoutSubmission {
    let handler = PaymentHandler::new(
        "layerx-402",
        "2026-04-08",
        "https://layerx.dev/2026-04-08/specification/402",
        "https://layerx.dev/2026-04-08/schemas/402.json",
    )
    .unwrap_or_else(|error| panic!("handler: {error}"));
    let merchant = MerchantProfile::layerx("https://merchant.example/ucp", handler.clone())
        .unwrap_or_else(|error| panic!("merchant: {error}"));
    let platform = PlatformProfile {
        profile_url: "https://buyer.example/profile".to_owned(),
        capabilities: merchant.capabilities().to_vec(),
        payment_handlers: vec![handler.clone()],
    };
    let data = hex(&case["receiptJson"]["logs"][0]["data"]);
    CheckoutSubmission {
        checkout_id: "sequencer-checkout".to_owned(),
        currency: *b"USD",
        total_minor: u128::from_be_bytes(
            data[48..64]
                .try_into()
                .unwrap_or_else(|_| panic!("fixture payment amount")),
        ),
        layerx_asset: data[..32]
            .try_into()
            .unwrap_or_else(|_| panic!("fixture payment asset")),
        layerx_recipient: array(&case["receiptJson"]["logs"][0]["topics"][2]),
        idempotency_key: UcpIdempotencyKey::parse("123e4567-e89b-12d3-a456-426614174000")
            .unwrap_or_else(|error| panic!("idempotency key: {error}")),
        negotiated: NegotiatedCapabilities::negotiate(&platform, &handler)
            .unwrap_or_else(|error| panic!("negotiation: {error}")),
    }
}

fn checkout_payment(case: &Value, submission: &CheckoutSubmission) -> ExecutedUcpPayment {
    let signed = hex(&case["signedReceipt"]);
    let verified = verify_sequencer_signature(&signed, &receipt_keys(case))
        .unwrap_or_else(|error| panic!("recorded signature: {error:?}"));
    let transaction_hash = array(&case["receiptJson"]["txHash"]);
    assert_eq!(
        verified
            .protocol()
            .unwrap_or_else(|| panic!("missing protocol receipt"))
            .activity_id(),
        transaction_hash
    );
    let log = &case["receiptJson"]["logs"][0];
    let mut topics: Vec<[u8; 32]> = log["topics"]
        .as_array()
        .unwrap_or_else(|| panic!("missing log topics"))
        .iter()
        .map(array)
        .collect();
    let recorded_topics = topics.concat();
    topics = vec![
        topics[0],
        submission.idempotency_key.gateway_key(),
        topics[1],
        topics[2],
    ];
    let mut unsigned = signed[..signed.len() - 69].to_vec();
    let offsets: Vec<usize> = unsigned
        .windows(recorded_topics.len())
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == recorded_topics).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "one signed payment log");
    unsigned[offsets[0]..offsets[0] + recorded_topics.len()].copy_from_slice(&topics.concat());
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    digest.update(&unsigned);
    digest.update([0]);
    let signer = SigningKey::from_bytes(&[0x5e; 32]);
    assert_eq!(
        signer.verifying_key().to_bytes(),
        array(&case["pinnedKey"]["publicKey"])
    );
    let signature = signer.sign(&digest.finalize()).to_bytes();
    unsigned.extend_from_slice(&[1, 0, 0, 0, 64]);
    unsigned.extend_from_slice(&signature);
    ExecutedUcpPayment {
        metadata: OrderMetadata {
            order_id: "sequencer-order".to_owned(),
            permalink_url: "https://merchant.example/orders/sequencer-order".to_owned(),
        },
        evidence: SequencerEvidence {
            transaction_hash,
            signed_receipt: unsigned,
            logs: vec![SequencerLog::new(
                array(&log["address"]),
                topics,
                hex(&log["data"]),
                0,
            )],
            rung: SettlementRung::Receipted,
        },
    }
}

fn registered_gateway(trace: &TraceId) -> GatewayCore {
    let mut gateway = GatewayCore::new();
    let adapter = AdapterId::new("ucp").unwrap_or_else(|error| panic!("adapter: {error}"));
    let version =
        SpecVersion::parse("20260408").unwrap_or_else(|error| panic!("spec version: {error}"));
    let spec = PinnedSpec::new(adapter, version, UCP_CHECKOUT_SPEC_SHA256)
        .unwrap_or_else(|error| panic!("spec: {error}"));
    let suite = ConformanceSuite::new(
        AdapterId::new("ucp-sequencer-vectors").unwrap_or_else(|error| panic!("suite id: {error}")),
        2,
        [0x52; 32],
    )
    .unwrap_or_else(|error| panic!("suite: {error}"));
    let descriptor =
        ucp_adapter_descriptor(spec, suite).unwrap_or_else(|error| panic!("descriptor: {error}"));
    gateway
        .register_adapter(descriptor, trace, 1)
        .unwrap_or_else(|error| panic!("register: {error}"));
    gateway
}

#[test]
fn ucp_order_digest_is_the_recorded_receipt_digest() {
    let case = fixture_case();
    let submission = submission(&case);
    let keys = receipt_keys(&case);
    let signed_receipt = hex(&case["signedReceipt"]);
    let receipt_digest = array(&case["receiptDigest"]);
    assert_eq!(
        canonical_receipt_digest(&signed_receipt),
        Ok(receipt_digest)
    );
    let mut payment = checkout_payment(&case, &submission);
    let principal = PrincipalId::new("buyer").unwrap_or_else(|error| panic!("principal: {error}"));
    let trace = TraceId::mint([0x21; 16]);
    let mut gateway = registered_gateway(&trace);
    let outcome = UcpAdapter::complete_checkout(
        &mut gateway,
        &principal,
        &submission,
        &mut payment,
        &keys,
        &trace,
        2,
    )
    .unwrap_or_else(|error| panic!("recorded checkout: {error}"));
    assert_eq!(outcome.status, CheckoutStatus::Completed);
    let order = outcome.order.unwrap_or_else(|| panic!("missing UCP order"));
    assert_eq!(
        Ok(order.receipt_digest),
        canonical_receipt_digest(&payment.evidence.signed_receipt)
    );
    let stored = outcome
        .stored_order
        .unwrap_or_else(|| panic!("missing stored order"));
    assert_eq!(stored.signed_receipt, payment.evidence.signed_receipt);
    assert_eq!(UcpAdapter::read_order(&stored, &keys), Ok(order));
}

#[test]
fn complete_checkout_stores_the_transaction_hash() {
    let case = fixture_case();
    let submission = submission(&case);
    let keys = receipt_keys(&case);
    let principal = PrincipalId::new("buyer").unwrap_or_else(|error| panic!("principal: {error}"));
    let trace = TraceId::mint([0x19; 16]);
    for rung in [
        SettlementRung::Receipted,
        SettlementRung::Committed {
            block_number: 1001,
            block_hash: [0x42; 32],
        },
    ] {
        let mut gateway = registered_gateway(&trace);
        let mut payment = checkout_payment(&case, &submission);
        payment.evidence.rung = rung;
        let receipt_digest =
            verify_sequencer_receipt(&submission, &payment.evidence.signed_receipt, &keys)
                .unwrap_or_else(|error| panic!("checkout receipt: {error}"));
        let outcome = UcpAdapter::complete_checkout(
            &mut gateway,
            &principal,
            &submission,
            &mut payment,
            &keys,
            &trace,
            2,
        )
        .unwrap_or_else(|error| panic!("complete checkout: {error}"));
        assert_eq!(outcome.status, CheckoutStatus::Completed);
        let mut stored = outcome
            .stored_order
            .unwrap_or_else(|| panic!("missing stored order"));
        let order = outcome.order.unwrap_or_else(|| panic!("missing UCP order"));
        let transaction_hash = array(&case["receiptJson"]["txHash"]);
        assert_eq!(stored.transaction_hash, transaction_hash);
        assert_eq!(order.transaction_hash, transaction_hash);
        assert_eq!(stored.signed_receipt, payment.evidence.signed_receipt);
        assert_eq!(stored.logs, payment.evidence.logs);
        assert_eq!(stored.rung, rung);
        assert_eq!(order.receipt_digest, receipt_digest);
        assert_eq!(UcpAdapter::read_order(&stored, &keys), Ok(order.clone()));
        let replay = UcpAdapter::complete_checkout(
            &mut gateway,
            &principal,
            &submission,
            &mut payment,
            &keys,
            &trace,
            3,
        )
        .unwrap_or_else(|error| panic!("replay checkout: {error}"));
        assert_eq!(replay.status, CheckoutStatus::Completed);
        assert_eq!(replay.order, Some(order));
        assert_eq!(replay.stored_order.as_ref(), Some(&stored));
        assert_eq!(
            gateway.translation(&principal, submission.idempotency_key.gateway_key()),
            Some(TranslationStatus::ReceiptVerified { receipt_digest })
        );
        assert_eq!(
            UcpAdapter::read_order(&stored, &[]),
            Err(UcpError::ReceiptMismatch)
        );
        stored.transaction_hash[0] ^= 1;
        assert_eq!(
            UcpAdapter::read_order(&stored, &keys),
            Err(UcpError::ReceiptMismatch)
        );
        stored.transaction_hash = transaction_hash;
        stored.rung = SettlementRung::Submitted;
        assert_eq!(
            UcpAdapter::read_order(&stored, &keys),
            Err(UcpError::ReceiptRequired)
        );
    }
}

#[test]
fn read_order_refuses_an_unpinned_receipt() {
    let case = fixture_case();
    let submission = submission(&case);
    let principal = PrincipalId::new("buyer").unwrap_or_else(|error| panic!("principal: {error}"));
    let trace = TraceId::mint([0x22; 16]);
    for rung in [
        SettlementRung::Receipted,
        SettlementRung::Committed {
            block_number: 1001,
            block_hash: [0x42; 32],
        },
    ] {
        let mut keys = receipt_keys(&case).to_vec();
        let mut gateway = registered_gateway(&trace);
        let mut payment = checkout_payment(&case, &submission);
        payment.evidence.rung = rung;
        let outcome = UcpAdapter::complete_checkout(
            &mut gateway,
            &principal,
            &submission,
            &mut payment,
            &keys,
            &trace,
            2,
        )
        .unwrap_or_else(|error| panic!("complete checkout: {error}"));
        assert_eq!(outcome.status, CheckoutStatus::Completed);
        let stored = outcome
            .stored_order
            .unwrap_or_else(|| panic!("missing stored order"));
        let order = outcome.order.unwrap_or_else(|| panic!("missing UCP order"));
        assert_eq!(UcpAdapter::read_order(&stored, &keys), Ok(order.clone()));
        keys.clear();
        assert_eq!(
            UcpAdapter::read_order(&stored, &keys),
            Err(UcpError::ReceiptMismatch)
        );
        assert_eq!(
            UcpAdapter::read_order(&stored, &receipt_keys(&case)),
            Ok(order)
        );
    }
}

#[test]
fn complete_checkout_on_a_submitted_rung_stays_open() {
    let case = fixture_case();
    let submission = submission(&case);
    let mut payment = checkout_payment(&case, &submission);
    payment.evidence.rung = SettlementRung::Submitted;
    payment.evidence.signed_receipt.clear();
    payment.evidence.logs.clear();
    let principal = PrincipalId::new("buyer").unwrap_or_else(|error| panic!("principal: {error}"));
    let trace = TraceId::mint([0x20; 16]);
    let mut gateway = registered_gateway(&trace);
    for now in [2, 3] {
        let outcome = UcpAdapter::complete_checkout(
            &mut gateway,
            &principal,
            &submission,
            &mut payment,
            &[],
            &trace,
            now,
        )
        .unwrap_or_else(|error| panic!("submitted checkout: {error}"));
        assert_eq!(outcome.status, CheckoutStatus::CompleteInProgress);
        assert!(outcome.order.is_none());
        assert!(outcome.stored_order.is_none());
        assert_eq!(
            gateway.translation(&principal, submission.idempotency_key.gateway_key()),
            Some(TranslationStatus::Pending)
        );
        assert_eq!(
            gateway
                .principal_audit(&principal)
                .unwrap_or_else(|| panic!("missing principal audit"))
                .entries()
                .len(),
            1
        );
    }
}

#[test]
fn payment_exact_native_topics_preserve_reference_and_identity_checks() {
    let case = fixture_case();
    let submission = submission(&case);
    let keys = receipt_keys(&case);
    let original = checkout_payment(&case, &submission);
    let original_log = &original.evidence.logs[0];
    let original_topics = original_log.topics();
    assert_ne!(original_topics[1], original_topics[2]);
    assert_ne!(original_topics[1], original_topics[3]);
    assert_ne!(original_topics[2], original_topics[3]);
    for permutation in [
        [1, 2, 3],
        [1, 3, 2],
        [2, 1, 3],
        [2, 3, 1],
        [3, 1, 2],
        [3, 2, 1],
    ] {
        let topics = vec![
            original_topics[0],
            original_topics[permutation[0]],
            original_topics[permutation[1]],
            original_topics[permutation[2]],
        ];
        let payment = with_signed_log(&original, topics, original_log.data().to_vec());
        let result = verify_sequencer_receipt(&submission, &payment.evidence.signed_receipt, &keys);
        if permutation == [1, 2, 3] {
            assert!(result.is_ok());
        } else {
            assert_eq!(result, Err(UcpError::ReceiptMismatch), "{permutation:?}");
        }
    }
    for mutation in [
        "zero-payer",
        "self-payment",
        "reference",
        "recipient",
        "asset",
        "amount",
    ] {
        let mut topics = original_topics.to_vec();
        let mut data = original_log.data().to_vec();
        match mutation {
            "zero-payer" => topics[2] = [0; 32],
            "self-payment" => topics[2] = topics[3],
            "reference" => topics[1][0] ^= 1,
            "recipient" => topics[3][0] ^= 1,
            "asset" => data[0] ^= 1,
            "amount" => data[63] ^= 1,
            _ => unreachable!(),
        }
        let payment = with_signed_log(&original, topics, data);
        assert_eq!(
            verify_sequencer_receipt(&submission, &payment.evidence.signed_receipt, &keys),
            Err(UcpError::ReceiptMismatch),
            "{mutation}",
        );
    }
}

fn with_signed_log(
    original: &ExecutedUcpPayment,
    topics: Vec<[u8; 32]>,
    data: Vec<u8>,
) -> ExecutedUcpPayment {
    let log = &original.evidence.logs[0];
    let before = [log.topics().concat(), Sha256::digest(log.data()).to_vec()].concat();
    let after = [topics.concat(), Sha256::digest(&data).to_vec()].concat();
    let receipt = &original.evidence.signed_receipt;
    let mut unsigned = receipt[..receipt.len() - 69].to_vec();
    let offsets: Vec<usize> = unsigned
        .windows(before.len())
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == before).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "one signed payment log");
    unsigned[offsets[0]..offsets[0] + before.len()].copy_from_slice(&after);
    let digest = Sha256::digest([b"LXP/v1/receipt\0".as_slice(), &unsigned, &[0]].concat());
    let signature = SigningKey::from_bytes(&[0x5e; 32]).sign(&digest);
    unsigned.extend_from_slice(&[1, 0, 0, 0, 64]);
    unsigned.extend_from_slice(&signature.to_bytes());
    let verified = verify_sequencer_signature(&unsigned, &receipt_keys(&fixture_case()))
        .unwrap_or_else(|error| panic!("mutated signature: {error:?}"));
    let logs = vec![SequencerLog::new(
        log.address(),
        topics,
        data,
        log.log_index(),
    )];
    layerx_proof::sequencer_logs::verify_logs(&verified, logs.clone())
        .unwrap_or_else(|error| panic!("mutated log commitment: {error:?}"));
    ExecutedUcpPayment {
        metadata: original.metadata.clone(),
        evidence: SequencerEvidence {
            signed_receipt: unsigned,
            logs,
            ..original.evidence.clone()
        },
    }
}
