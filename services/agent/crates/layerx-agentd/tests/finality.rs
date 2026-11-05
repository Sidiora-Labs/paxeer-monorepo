#[path = "finality_rungs.rs"]
mod rungs;

use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agentd::finality::{
    augment, augment_verified, wait_for_rung, FinalityError, ReceiptEvidence, SubmissionProgress,
    VerificationProgress, VerifiedBlock, VerifiedReceipt,
};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_types::rung::Rung;
use rungs::fixtures::*;

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

fn tenant() -> TenantId {
    must(TenantId::new("tenant-a"))
}

fn directory(label: &str) -> std::path::PathBuf {
    let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-finality-{label}-{}-{sequence}",
        std::process::id()
    ))
}

fn record_key() -> TenantKey {
    let mut identifier = b"finality:".to_vec();
    identifier.extend_from_slice(&IDEMPOTENCY_KEY);
    must(TenantKey::new(
        tenant(),
        ObjectKind::Configuration,
        identifier,
    ))
}

#[test]
fn authenticated_commit_raises_only_committed_without_altering_receipt() {
    let root = directory("committed");
    let (bytes, key) = receipt();
    let receipt = must(VerifiedReceipt::verify(TX_HASH, &bytes, &[key]));
    let (commit, validators) = commit(&[1, 1, 1, 1], &[0, 1, 2]);
    let verified = must(VerifiedBlock::verify(
        &block(),
        &commit,
        CHAIN_ID,
        &validators,
    ));
    let mut durable = must(Store::open(&root));
    let record = must(augment_verified(
        &mut durable,
        tenant(),
        IDEMPOTENCY_KEY,
        &receipt,
        Some(&verified),
    ));
    assert_eq!(record.rung, Rung::Committed);
    assert_eq!(record.receipt_bytes, bytes);
    assert_eq!(record.tx_hash, TX_HASH);
    assert_eq!(record.receipt_key, key);
    assert_eq!(record.height, 7);
    assert!(record.block.is_some());
    let encoded = durable
        .get(&record_key())
        .unwrap_or_else(|| panic!("missing finality record"))
        .bytes()
        .to_vec();
    assert_eq!(&encoded[..4], b"LXF2");
    drop(durable);
    let mut reopened = must(Store::open(&root));
    assert_eq!(
        reopened
            .get(&record_key())
            .unwrap_or_else(|| panic!("lost record"))
            .bytes(),
        encoded
    );
    let repeated = must(augment_verified(
        &mut reopened,
        tenant(),
        IDEMPOTENCY_KEY,
        &receipt,
        Some(&verified),
    ));
    assert_eq!(repeated, record);
    assert!(matches!(
        augment_verified(&mut reopened, tenant(), IDEMPOTENCY_KEY, &receipt, None),
        Err(FinalityError::Corrupt)
    ));
    let mut conflicting_block = block();
    conflicting_block.hash[0] ^= 1;
    let mut conflicting_commit = commit.clone();
    conflicting_commit.block_hash = conflicting_block.hash;
    for (index, signature) in conflicting_commit.signatures.iter_mut().enumerate() {
        use ed25519_dalek::Signer as _;
        let signer = ed25519_dalek::SigningKey::from_bytes(
            &[u8::try_from(index + 50).unwrap_or_else(|_| panic!("key index")); 32],
        );
        signature.signature = signer
            .sign(&must({
                let mut vote = commit.clone();
                vote.block_hash = conflicting_block.hash;
                vote.vote_sign_bytes(
                    CHAIN_ID,
                    signature.timestamp_seconds,
                    signature.timestamp_nanos,
                )
            }))
            .to_bytes();
    }
    let conflicting = must(VerifiedBlock::verify(
        &conflicting_block,
        &conflicting_commit,
        CHAIN_ID,
        &validators,
    ));
    assert!(matches!(
        augment_verified(
            &mut reopened,
            tenant(),
            IDEMPOTENCY_KEY,
            &receipt,
            Some(&conflicting)
        ),
        Err(FinalityError::Corrupt)
    ));
    assert_eq!(
        reopened
            .get(&record_key())
            .unwrap_or_else(|| panic!("lost record"))
            .bytes(),
        encoded
    );
    assert_eq!(receipt.canonical_bytes(), bytes);
    drop(reopened);
    must(std::fs::remove_dir_all(root));
}

#[test]
fn raw_receipt_cannot_escalate_to_committed_and_wait_reports_observed_rung() {
    let root = directory("pending");
    let (bytes, key) = receipt();
    let receipt = must(VerifiedReceipt::verify(TX_HASH, &bytes, &[key]));
    let mut durable = must(Store::open(&root));
    let record = must(augment(
        &mut durable,
        tenant(),
        IDEMPOTENCY_KEY,
        &ReceiptEvidence {
            tx_hash: TX_HASH,
            canonical_bytes: &bytes,
            published_keys: &[key],
        },
        None,
        CHAIN_ID,
        &[],
    ));
    assert_eq!(record.rung, Rung::Receipted);
    assert!(record.block.is_none());
    assert_eq!(record.receipt_bytes, bytes);
    let mut progress = SubmissionProgress::accepted(IDEMPOTENCY_KEY, TX_HASH);
    must(progress.observe_receipt(&bytes, &[key]));
    let waited = must(wait_for_rung(
        &mut progress,
        IDEMPOTENCY_KEY,
        Rung::Committed,
        10_000,
        70_000,
        5_000,
    ));
    assert!(waited.deadline_elapsed);
    assert_eq!(waited.observed_at_ms, 70_000);
    assert_eq!(waited.reached, Rung::Receipted);
    assert!(matches!(
        wait_for_rung(&mut progress, IDEMPOTENCY_KEY, Rung::Committed, 2, 1, 1),
        Err(FinalityError::InvalidDeadline)
    ));
    assert!(matches!(
        wait_for_rung(&mut progress, IDEMPOTENCY_KEY, Rung::Committed, 1, 2, 0),
        Err(FinalityError::InvalidDeadline)
    ));
    assert!(matches!(
        wait_for_rung(
            &mut progress,
            IDEMPOTENCY_KEY,
            Rung::Committed,
            u64::MAX - 1,
            u64::MAX,
            5,
        ),
        Err(FinalityError::Arithmetic)
    ));
    drop(durable);
    must(std::fs::remove_dir_all(root));
}

#[test]
fn ineligible_validator_snapshot_records_no_unearned_committed_rung() {
    let root = directory("threshold");
    let (bytes, key) = receipt();
    let receipt = must(VerifiedReceipt::verify(TX_HASH, &bytes, &[key]));
    let mut durable = must(Store::open(&root));
    let before = must(augment_verified(
        &mut durable,
        tenant(),
        IDEMPOTENCY_KEY,
        &receipt,
        None,
    ));
    let (commit, mut validators) = commit(&[1, 1, 1, 1], &[0, 1, 2]);
    validators[1].public_key = ed25519_dalek::SigningKey::from_bytes(&[99; 32])
        .verifying_key()
        .to_bytes();
    assert!(matches!(
        augment(
            &mut durable,
            tenant(),
            IDEMPOTENCY_KEY,
            &ReceiptEvidence {
                tx_hash: TX_HASH,
                canonical_bytes: &bytes,
                published_keys: &[key]
            },
            Some((&block(), &commit)),
            CHAIN_ID,
            &validators
        ),
        Err(FinalityError::InvalidCommit)
    ));
    let after = must(augment_verified(
        &mut durable,
        tenant(),
        IDEMPOTENCY_KEY,
        &receipt,
        None,
    ));
    assert_eq!(after, before);
    assert_eq!(after.rung, Rung::Receipted);
    assert_eq!(after.receipt_bytes, bytes);
    drop(durable);
    must(std::fs::remove_dir_all(root));
}

#[test]
fn mismatched_block_reference_is_rejected_without_rung_change() {
    let root = directory("block-reference");
    let (bytes, key) = receipt();
    let receipt = must(VerifiedReceipt::verify(TX_HASH, &bytes, &[key]));
    let (mut commit, validators) = commit(&[1, 1, 1, 1], &[0, 1, 2]);
    let mut wrong = block();
    wrong.hash[0] ^= 1;
    assert!(matches!(
        VerifiedBlock::verify(&wrong, &commit, CHAIN_ID, &validators),
        Err(FinalityError::InvalidCommit)
    ));
    let mut durable = must(Store::open(&root));
    let record = must(augment_verified(
        &mut durable,
        tenant(),
        IDEMPOTENCY_KEY,
        &receipt,
        None,
    ));
    commit.height = 6;
    assert!(matches!(
        augment(
            &mut durable,
            tenant(),
            IDEMPOTENCY_KEY,
            &ReceiptEvidence {
                tx_hash: TX_HASH,
                canonical_bytes: &bytes,
                published_keys: &[key]
            },
            Some((&block(), &commit)),
            CHAIN_ID,
            &validators
        ),
        Err(FinalityError::InvalidCommit)
    ));
    assert_eq!(record.rung, Rung::Receipted);
    assert_eq!(record.receipt_bytes, bytes);
    let mut progress = SubmissionProgress::accepted(IDEMPOTENCY_KEY, TX_HASH);
    assert!(matches!(
        progress.rung_at([0; 32], 0),
        Err(FinalityError::ProgressUnavailable)
    ));
    assert_eq!(must(progress.rung_at(IDEMPOTENCY_KEY, 0)), Rung::Submitted);
    drop(durable);
    must(std::fs::remove_dir_all(root));
}
