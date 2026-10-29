use layerx_agentd::outbox::{Outbox, OutboxError, SubmissionState};
use layerx_agentd::protocol_evidence::VerifiedReceiptEvidence;
use layerx_agentd::store::{ObjectKind, Store, TenantKey};
use layerx_client::sequencer::SequencerBlock;
use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};

mod support;

fn raw(nonce: u64, data: u8) -> Vec<u8> {
    Eip1559Call {
        chain_id: 125,
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 100_000,
        to: [0x22; 20],
        data: vec![data],
    }
    .sign(&[0xa5; 32])
    .unwrap_or_else(|error| panic!("sign: {error:?}"))
}

fn receipt(hash: [u8; 32]) -> VerifiedReceiptEvidence {
    support::evidence_verifier()
        .verify_receipt(&support::raw_receipt(hash, 0, 1))
        .unwrap_or_else(|error| panic!("verify receipt: {error:?}"))
}

fn block(number: u64, transactions: Vec<Vec<u8>>) -> SequencerBlock {
    SequencerBlock {
        number,
        hash: [0x55; 32],
        parent_hash: [0x44; 32],
        timestamp: 1_000,
        transactions_root: [0x33; 32],
        receipts_root: [0x66; 32],
        transactions,
    }
}

#[test]
fn outbox_is_durable_before_transmission_and_unknown_is_a_real_state() {
    let root = support::directory("outbox-durable");
    let bytes = raw(4, 1);
    let hash = transaction_hash(&bytes);
    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let mut outbox = Outbox::default();
    assert_eq!(
        outbox
            .enqueue(&mut store, support::tenant(), bytes.clone())
            .unwrap_or_else(|error| panic!("operation failed: {error:?}")),
        hash
    );
    assert_eq!(
        outbox
            .bytes_for_transmission(hash)
            .unwrap_or_else(|error| panic!("operation failed: {error:?}")),
        bytes
    );
    drop(outbox);
    drop(store);

    let mut store =
        Store::open(&root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let mut restored = Outbox::default();
    restored
        .restore(&store, support::tenant(), hash)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    assert_eq!(
        restored
            .bytes_for_transmission(hash)
            .unwrap_or_else(|error| panic!("operation failed: {error:?}")),
        bytes
    );
    restored
        .transition(&mut store, hash, SubmissionState::Submitted, "sent", None)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    restored
        .transition(
            &mut store,
            hash,
            SubmissionState::Unknown,
            "transport uncertain",
            None,
        )
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let status = restored
        .status(hash)
        .unwrap_or_else(|| panic!("expected value missing"));
    assert_eq!(status.state, SubmissionState::Unknown);
    assert_eq!(status.transitions.len(), 4);
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
}

#[test]
fn every_legal_terminal_path_is_recorded_and_illegal_transitions_fail() {
    let root = support::directory("outbox-terminal");
    let mut store =
        Store::open(&root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let mut outbox = Outbox::default();
    let mut hashes = Vec::new();
    for nonce in 1..=6 {
        hashes.push(
            outbox
                .enqueue(&mut store, support::tenant(), raw(nonce, 1))
                .unwrap_or_else(|error| panic!("operation failed: {error:?}")),
        );
    }
    outbox
        .transition(
            &mut store,
            hashes[0],
            SubmissionState::Submitted,
            "sent",
            None,
        )
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    outbox
        .transition(
            &mut store,
            hashes[0],
            SubmissionState::Submitted,
            "core admission acknowledged",
            None,
        )
        .unwrap_or_else(|error| panic!("record admission acknowledgement: {error:?}"));
    outbox
        .transition(
            &mut store,
            hashes[0],
            SubmissionState::Unknown,
            "uncertain",
            None,
        )
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let execution_receipt = receipt(hashes[0]);
    assert_eq!(
        execution_receipt.level(),
        layerx_types::verify::VerificationLevel::BATCH_INCLUDED
    );
    outbox
        .record_execution(&mut store, hashes[0], execution_receipt, 8, 0)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    assert!(!outbox
        .status(hashes[0])
        .unwrap_or_else(|| panic!("expected value missing"))
        .state
        .terminal());
    outbox
        .reconcile_committed_block(&mut store, hashes[0], &block(8, vec![raw(1, 1)]))
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    outbox
        .transition(
            &mut store,
            hashes[1],
            SubmissionState::Expired,
            "expired",
            None,
        )
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    outbox
        .transition(
            &mut store,
            hashes[2],
            SubmissionState::Failed,
            "submission.nonce_consumed",
            None,
        )
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    outbox
        .transition(
            &mut store,
            hashes[3],
            SubmissionState::Submitted,
            "sent",
            None,
        )
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    outbox
        .transition(
            &mut store,
            hashes[3],
            SubmissionState::Unknown,
            "uncertain",
            None,
        )
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let failure = support::evidence_verifier()
        .verify_receipt(&support::raw_receipt(hashes[3], 1, 1))
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    assert_eq!(
        failure.level(),
        layerx_types::verify::VerificationLevel::BATCH_INCLUDED
    );
    outbox
        .record_failure(&mut store, hashes[3], failure, 8, 1)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    outbox
        .transition(
            &mut store,
            hashes[4],
            SubmissionState::Submitted,
            "sent",
            None,
        )
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    outbox
        .record_execution(&mut store, hashes[4], receipt(hashes[4]), 9, 0)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    outbox
        .reconcile_committed_block(&mut store, hashes[4], &block(9, vec![raw(5, 1)]))
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    outbox
        .transition(
            &mut store,
            hashes[5],
            SubmissionState::Submitted,
            "sent",
            None,
        )
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let failure = support::evidence_verifier()
        .verify_receipt(&support::raw_receipt(hashes[5], 1, 1))
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    outbox
        .record_failure(&mut store, hashes[5], failure, 9, 1)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    for hash in hashes.iter().copied() {
        assert!(outbox
            .status(hash)
            .unwrap_or_else(|| panic!("expected value missing"))
            .state
            .terminal());
    }
    assert!(matches!(
        outbox.transition(
            &mut store,
            hashes[0],
            SubmissionState::Unknown,
            "illegal",
            None
        ),
        Err(OutboxError::InvalidTransition { .. })
    ));
    assert!(matches!(
        outbox.transition(
            &mut store,
            hashes[1],
            SubmissionState::Submitted,
            "illegal",
            None
        ),
        Err(OutboxError::InvalidTransition { .. })
    ));
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
}

#[test]
fn executed_is_impossible_without_a_verified_receipt_reference() {
    let root = support::directory("outbox-receipt");
    let mut store =
        Store::open(&root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let mut outbox = Outbox::default();
    let hash = outbox
        .enqueue(&mut store, support::tenant(), raw(4, 1))
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    outbox
        .transition(&mut store, hash, SubmissionState::Submitted, "sent", None)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    assert!(matches!(
        outbox.transition(
            &mut store,
            hash,
            SubmissionState::Executed,
            "missing receipt",
            None
        ),
        Err(OutboxError::SuccessWithoutVerifiedReceipt)
    ));
    assert!(matches!(
        outbox.record_execution(&mut store, hash, receipt([8; 32]), 8, 0),
        Err(OutboxError::ReceiptMismatch)
    ));
    let raw = support::raw_receipt([8; 32], 0, 1);
    let mut corrupt = raw.canonical_receipt().to_vec();
    corrupt[0] ^= 1;
    assert!(support::evidence_verifier()
        .verify_receipt(&support::corrupt_raw_receipt(&raw, corrupt))
        .is_err());
    assert_eq!(
        outbox
            .status(hash)
            .unwrap_or_else(|| panic!("expected value missing"))
            .state,
        SubmissionState::Submitted
    );
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
}

#[test]
fn legacy_boolean_receipt_records_fail_closed() {
    let root = support::directory("outbox-legacy");
    let mut store =
        Store::open(&root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let mut outbox = Outbox::default();
    let hash = outbox
        .enqueue(&mut store, support::tenant(), raw(4, 1))
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let key = TenantKey::new(support::tenant(), ObjectKind::Outbox, hash.to_vec())
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let mut legacy = store
        .get(&key)
        .unwrap_or_else(|| panic!("expected value missing"))
        .bytes()
        .to_vec();
    legacy[4] = 1;
    store
        .put_local(key, legacy)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    assert!(matches!(
        Outbox::default().restore(&store, support::tenant(), hash),
        Err(OutboxError::Corrupt)
    ));
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
}
