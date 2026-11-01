use ed25519_dalek::SigningKey;
use layerx_agentd::budget::ProtocolBudgetState;
use layerx_agentd::outbox::{
    legal_transition, recover, Outbox, OutboxError, RecoveryInputs, SubmissionState,
};
use layerx_agentd::store::{ObjectKind, Store, TenantKey};
use layerx_client::sequencer::SequencerBlock;
use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};

mod support;

fn raw(nonce: u64, data: u8, secret: u8) -> Vec<u8> {
    Eip1559Call {
        chain_id: 125,
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 100_000,
        to: [0x22; 20],
        data: vec![data],
    }
    .sign(&[secret; 32])
    .unwrap_or_else(|error| panic!("sign: {error:?}"))
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
fn outbox_keys_records_by_transaction_hash() {
    let root = support::directory("outbox-hash");
    let mut store =
        Store::open(&root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let mut outbox = Outbox::default();
    let bytes = raw(4, 1, 0xa5);
    let hash = transaction_hash(&bytes);
    assert_eq!(
        outbox
            .enqueue(&mut store, support::tenant(), bytes.clone())
            .unwrap_or_else(|error| panic!("operation failed: {error:?}")),
        hash
    );
    assert_eq!(
        outbox
            .status(hash)
            .unwrap_or_else(|| panic!("expected value missing"))
            .submission_id,
        hash
    );
    assert_eq!(
        outbox
            .bytes_for_transmission(hash)
            .unwrap_or_else(|error| panic!("operation failed: {error:?}")),
        bytes
    );
    for kind in [ObjectKind::Outbox, ObjectKind::PreparedActivity] {
        let key = TenantKey::new(support::tenant(), kind, hash.to_vec())
            .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
        assert!(store.get(&key).is_some());
    }
    assert!(matches!(
        outbox.enqueue(&mut store, support::tenant(), bytes.clone()),
        Err(OutboxError::Duplicate)
    ));
    assert_eq!(
        outbox
            .status(hash)
            .unwrap_or_else(|| panic!("expected value missing"))
            .transitions
            .len(),
        2
    );
    drop(store);
    let mut store =
        Store::open(&root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let mut restored = Outbox::default();
    assert!(matches!(
        restored.enqueue(&mut store, support::tenant(), bytes.clone()),
        Err(OutboxError::Duplicate)
    ));
    restored
        .restore(&store, support::tenant(), hash)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    assert_eq!(
        restored
            .exact_signed_bytes(hash)
            .unwrap_or_else(|error| panic!("operation failed: {error:?}")),
        bytes
    );
    let different = raw(4, 2, 0xa5);
    let different_hash = transaction_hash(&different);
    assert_ne!(hash, different_hash);
    assert_eq!(
        restored
            .enqueue(&mut store, support::tenant(), different)
            .unwrap_or_else(|error| panic!("operation failed: {error:?}")),
        different_hash
    );
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
}

#[test]
fn committed_is_reached_only_from_executed() {
    use SubmissionState::{
        Committed, Executed, Expired, Failed, Prepared, Queued, Signed, Submitted, Unknown,
    };
    let states = [
        Prepared, Signed, Queued, Submitted, Unknown, Executed, Committed, Failed, Expired,
    ];
    for state in states {
        assert_eq!(legal_transition(state, Committed), state == Executed);
        assert!(!legal_transition(Committed, state));
    }
    let root = support::directory("outbox-committed");
    let mut store =
        Store::open(&root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let mut outbox = Outbox::default();
    let bytes = raw(4, 1, 0xa5);
    let hash = outbox
        .enqueue(&mut store, support::tenant(), bytes.clone())
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    outbox
        .transition(&mut store, hash, Submitted, "sent", None)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let acknowledged = outbox
        .transition(
            &mut store,
            hash,
            Submitted,
            "core admission acknowledged",
            None,
        )
        .unwrap_or_else(|error| panic!("record admission acknowledgement: {error:?}"));
    assert_eq!(acknowledged.state, Submitted);
    assert_eq!(
        acknowledged
            .transitions
            .last()
            .map(|transition| transition.cause.as_str()),
        Some("core admission acknowledged")
    );
    assert!(matches!(
        outbox.transition(&mut store, hash, Committed, "too early", None),
        Err(OutboxError::InvalidTransition { .. })
    ));
    let failed_receipt = layerx_agentd::protocol_evidence::EvidenceAuthority::pinned_to_handshake(
        layerx_wire::limits::PROTOCOL_VERSION,
        42,
        SigningKey::from_bytes(&[0x3a; 32])
            .verifying_key()
            .to_bytes(),
    )
    .unwrap_or_else(|error| panic!("pinned evidence authority: {error:?}"))
    .verify_receipt(&support::raw_receipt(hash, 1, 1))
    .unwrap_or_else(|error| panic!("verify failure receipt: {error:?}"));
    assert!(matches!(
        outbox.record_execution(&mut store, hash, failed_receipt.clone(), 8, 1),
        Err(OutboxError::ReceiptFailed)
    ));
    assert!(matches!(
        outbox.transition(
            &mut store,
            hash,
            Executed,
            "failed receipt",
            Some(failed_receipt)
        ),
        Err(OutboxError::ReceiptFailed)
    ));
    let verified = layerx_agentd::protocol_evidence::EvidenceAuthority::pinned_to_handshake(
        layerx_wire::limits::PROTOCOL_VERSION,
        42,
        SigningKey::from_bytes(&[0x3a; 32])
            .verifying_key()
            .to_bytes(),
    )
    .unwrap_or_else(|error| panic!("pinned evidence authority: {error:?}"))
    .verify_receipt(&support::raw_receipt(hash, 0, 1))
    .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let reference = verified.receipt_ref();
    let executed = outbox
        .record_execution(&mut store, hash, verified, 8, 1)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    assert_eq!(executed.state, Executed);
    assert!(!executed.state.terminal());
    let evidence = executed
        .evidence
        .unwrap_or_else(|| panic!("expected value missing"));
    assert_eq!(evidence.receipt_ref(), reference);
    assert_eq!(evidence.block_height(), 8);
    assert_eq!(evidence.transaction_index(), 1);
    assert!(matches!(
        outbox.transition(&mut store, hash, Committed, "uncommitted", None),
        Err(OutboxError::BlockNotCommitted)
    ));
    assert!(matches!(
        outbox.reconcile_committed_block(
            &mut store,
            hash,
            &block(9, vec![raw(1, 1, 0xa5), bytes.clone()])
        ),
        Err(OutboxError::ReceiptMismatch)
    ));
    assert!(matches!(
        outbox.reconcile_committed_block(
            &mut store,
            hash,
            &block(8, vec![bytes.clone(), raw(1, 1, 0xa5)])
        ),
        Err(OutboxError::ReceiptMismatch)
    ));
    assert_eq!(outbox.status(hash), Some(&executed));
    let committed = outbox
        .reconcile_committed_block(&mut store, hash, &block(8, vec![raw(1, 1, 0xa5), bytes]))
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    assert_eq!(committed.state, Committed);
    assert!(committed.state.terminal());
    assert_eq!(committed.evidence, Some(evidence));
    assert_eq!(
        outbox
            .reconcile_committed_block(&mut store, hash, &block(9, vec![raw(4, 2, 0xa5)]))
            .unwrap_or_else(|error| panic!("reconcile terminal record: {error:?}")),
        committed
    );
    for state in states {
        assert!(matches!(
            outbox.transition(&mut store, hash, state, "terminal", None),
            Err(OutboxError::InvalidTransition { .. })
        ));
    }
    drop(store);
    let mut store =
        Store::open(&root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let mut restored = Outbox::default();
    restored
        .restore(&store, support::tenant(), hash)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    assert_eq!(restored.status(hash), Some(&committed));
    assert!(matches!(
        restored.bytes_for_transmission(hash),
        Err(OutboxError::NotQueued)
    ));
    let mut budget_record = vec![0; 278];
    budget_record[1] = 1;
    budget_record[2..34].copy_from_slice(&[1; 32]);
    budget_record[162..178].copy_from_slice(&500_u128.to_be_bytes());
    budget_record[178..194].copy_from_slice(&500_u128.to_be_bytes());
    budget_record[242..250].copy_from_slice(&1_000_u64.to_be_bytes());
    let recovered = recover(
        &mut store,
        &support::tenant(),
        &RecoveryInputs {
            verifier: layerx_agentd::protocol_evidence::EvidenceAuthority::pinned_to_handshake(
                layerx_wire::limits::PROTOCOL_VERSION,
                42,
                SigningKey::from_bytes(&[0x3a; 32])
                    .verifying_key()
                    .to_bytes(),
            )
            .unwrap_or_else(|error| panic!("pinned evidence authority: {error:?}")),
            unknown_budget_ids: &[],
            budget_receipts: &[],
            protocol_budget: ProtocolBudgetState {
                evidence: support::raw_state_leaf(budget_record, 99),
            },
            ceiling_maximum: 500,
            ceiling_receipts: &[],
            unknown_ceiling_reservations: &[],
            current_sequence: 99,
        },
    )
    .unwrap_or_else(|error| panic!("recover: {error:?}"));
    assert_eq!(recovered.outbox.status(hash), Some(&committed));
    assert!(recovered.queued_for_transmission.is_empty());
    assert!(recovered.queued_without_authority.is_empty());
    assert!(recovered.awaiting_receipt_resolution.is_empty());
    assert!(matches!(
        recovered.outbox.bytes_for_transmission(hash),
        Err(OutboxError::NotQueued)
    ));
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
}

#[test]
fn consumed_nonce_fails_with_nonce_consumed() {
    let root = support::directory("outbox-consumed-nonce");
    let mut store =
        Store::open(&root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let mut outbox = Outbox::default();
    let bytes = raw(4, 1, 0xa5);
    let hash = outbox
        .enqueue(&mut store, support::tenant(), bytes.clone())
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    outbox
        .transition(&mut store, hash, SubmissionState::Submitted, "sent", None)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let unrelated = block(8, vec![raw(4, 2, 0xa6), raw(5, 2, 0xa5), bytes]);
    assert_eq!(
        outbox
            .reconcile_committed_block(&mut store, hash, &unrelated)
            .unwrap_or_else(|error| panic!("operation failed: {error:?}"))
            .state,
        SubmissionState::Submitted
    );
    let replacement = raw(4, 2, 0xa5);
    assert_ne!(transaction_hash(&replacement), hash);
    let failed = outbox
        .reconcile_committed_block(&mut store, hash, &block(9, vec![replacement]))
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    assert_eq!(failed.state, SubmissionState::Failed);
    assert_eq!(
        failed
            .transitions
            .last()
            .unwrap_or_else(|| panic!("expected value missing"))
            .cause,
        "submission.nonce_consumed"
    );
    assert!(failed.evidence.is_none());
    assert_eq!(
        outbox
            .reconcile_committed_block(&mut store, hash, &block(10, vec![raw(4, 3, 0xa5)]))
            .unwrap_or_else(|error| panic!("reconcile terminal record: {error:?}")),
        failed
    );
    assert!(matches!(
        outbox.bytes_for_transmission(hash),
        Err(OutboxError::NotQueued)
    ));
    drop(store);
    let store = Store::open(&root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    let mut restored = Outbox::default();
    restored
        .restore(&store, support::tenant(), hash)
        .unwrap_or_else(|error| panic!("operation failed: {error:?}"));
    assert_eq!(restored.status(hash), Some(&failed));
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("operation failed: {error:?}"));
}
