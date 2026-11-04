#[path = "middleware.rs"]
mod middleware;

use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite, PinnedSpec, SpecVersion};
use layerx_interop_gateway::evidence::{canonical_receipt_digest, SettlementRung};
use layerx_interop_gateway::gateway::{TranslationKind, TranslationRequest, TranslationStatus};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_visa_tap::{
    visa_tap_adapter_descriptor, MerchantOperationResult, TapError, VISA_TAP_SPEC_SHA256,
};
use middleware::{fixture_fixed, recorded_case, recorded_payment, recorded_payment_topics};
use sha2::{Digest as _, Sha256};

#[test]
fn visa_intent_path_producer_receipt_agrees_with_both_consumers() {
    let case = recorded_case();
    let (evidence, keys, digest) = recorded_payment();
    let receipt = verify_sequencer_signature(&evidence.signed_receipt, &keys[..])
        .expect("the producer's original receipt verifies without re-signing");
    assert_eq!(
        receipt
            .protocol()
            .expect("native protocol facts")
            .activity_id(),
        fixture_fixed(&case["receiptJson"]["txHash"])
    );
    assert_eq!(
        receipt
            .protocol()
            .expect("native protocol facts")
            .activity_id(),
        evidence.transaction_hash
    );
    assert_eq!(
        canonical_receipt_digest(&evidence.signed_receipt).expect("canonical producer digest"),
        digest
    );
    let logs = verify_logs(&receipt, evidence.logs.clone()).expect("signed producer logs");
    assert_eq!(logs.logs().len(), 1);
    let payment = &logs.logs()[0];
    let [reference, from, to] = recorded_payment_topics(&case);
    assert_eq!(&payment.topics()[1..], &[reference, from, to]);
    let asset = payment.data()[..32].try_into().expect("payment asset");
    let amount = u128::from_be_bytes(payment.data()[48..64].try_into().expect("payment amount"));
    let expected = MerchantOperationResult::ReceiptVerified {
        receipt_digest: digest,
        from,
        to,
        asset,
        amount,
    };
    assert_eq!(
        MerchantOperationResult::from_receipt(&evidence, &keys, reference),
        Ok(expected.clone())
    );

    let principal = PrincipalId::new("merchant-shop-alpha").expect("merchant scope");
    let other_principal = PrincipalId::new("merchant-shop-beta").expect("other merchant scope");
    let trace = TraceId::mint([0x16; 16]);
    let adapter = AdapterId::new("visa-tap").expect("adapter id");
    let document = PinnedSpec::new(
        adapter.clone(),
        SpecVersion::parse("1").expect("pinned version"),
        VISA_TAP_SPEC_SHA256,
    )
    .expect("Visa specification pin");
    let suite_bytes = include_bytes!("repair_22_33_116.rs");
    let suite = ConformanceSuite::new(
        AdapterId::new("visa-intent-path").expect("suite id"),
        3,
        Sha256::digest(suite_bytes).into(),
    )
    .expect("recorded suite declaration");
    let mut gateway = GatewayCore::new();
    gateway
        .register_adapter(
            visa_tap_adapter_descriptor(document, suite).expect("production adapter descriptor"),
            &trace,
            1,
        )
        .expect("register the Visa adapter");
    let request = TranslationRequest::new(
        adapter,
        TranslationKind::StateChanging,
        reference,
        Sha256::digest(
            case["rawTransaction"]
                .as_str()
                .expect("producer transaction"),
        )
        .into(),
    )
    .expect("production translation request");
    assert_eq!(
        gateway
            .begin_translation(&principal, &request, &trace, 2)
            .expect("begin the production translation"),
        TranslationStatus::Pending
    );
    assert_eq!(gateway.translation(&other_principal, reference), None);
    let settled = TranslationStatus::ReceiptVerified {
        receipt_digest: digest,
    };
    assert_eq!(
        gateway
            .settle_with_sequencer_evidence(&principal, reference, &evidence, &keys, &trace, 3)
            .expect("settle the producer receipt"),
        settled
    );
    assert_eq!(gateway.translation(&principal, reference), Some(settled));
    assert_eq!(
        gateway
            .begin_translation(&principal, &request, &trace, 4)
            .expect("retry the same translation"),
        settled,
        "an identical retry retains its verified outcome"
    );
    assert_eq!(
        gateway
            .settle_with_sequencer_evidence(&principal, reference, &evidence, &keys, &trace, 5)
            .expect("repeat the same settlement"),
        settled
    );
    assert_eq!(
        MerchantOperationResult::from_receipt(&evidence, &keys, reference),
        Ok(expected),
        "both production consumers retain the same receipt-bound commerce result"
    );
}

#[test]
fn visa_intent_path_refuses_substituted_receipt_identity_and_payment_fields() {
    let (evidence, keys, _) = recorded_payment();
    let reference = evidence.logs[0].topics()[1];
    assert!(MerchantOperationResult::from_receipt(&evidence, &keys, reference).is_ok());
    for field in 0..8 {
        let mut changed = evidence.clone();
        match field {
            0 => changed.transaction_hash[0] ^= 1,
            1 => {
                *changed
                    .signed_receipt
                    .last_mut()
                    .expect("receipt signature") ^= 1
            }
            2..=6 => {
                let log = &changed.logs[0];
                let mut topics = log.topics().to_vec();
                let mut data = log.data().to_vec();
                match field {
                    2 => topics[1][0] ^= 1,
                    3 => topics[2][0] ^= 1,
                    4 => topics[3][0] ^= 1,
                    5 => data[0] ^= 1,
                    6 => data[63] ^= 1,
                    _ => unreachable!(),
                }
                changed.logs[0] = SequencerLog::new(log.address(), topics, data, log.log_index());
            }
            7 => changed.rung = SettlementRung::Submitted,
            _ => unreachable!(),
        }
        assert_eq!(
            MerchantOperationResult::from_receipt(&changed, &keys, reference),
            Err(TapError::ReceiptMismatch),
            "substituted field {field} cannot produce commerce success"
        );
    }
    let mut wrong_reference = reference;
    wrong_reference[0] ^= 1;
    assert_eq!(
        MerchantOperationResult::from_receipt(&evidence, &keys, wrong_reference),
        Err(TapError::ReceiptMismatch)
    );
    assert_eq!(
        MerchantOperationResult::from_receipt(&evidence, &[], reference),
        Err(TapError::ReceiptMismatch)
    );
}

#[test]
fn visa_intent_path_reopened_recorded_evidence_retains_the_same_commerce_outcome() {
    let before = {
        let (evidence, keys, _) = recorded_payment();
        let reference = evidence.logs[0].topics()[1];
        MerchantOperationResult::from_receipt(&evidence, &keys, reference)
            .expect("producer receipt before releasing every decoded object")
    };
    let (reopened, keys, _) = recorded_payment();
    let reference = reopened.logs[0].topics()[1];
    assert_eq!(
        MerchantOperationResult::from_receipt(&reopened, &keys, reference),
        Ok(before)
    );
}
