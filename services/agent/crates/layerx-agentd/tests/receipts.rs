use std::sync::atomic::{AtomicU64, Ordering};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::receipt::{
    classify, serve, serve_evidence, store, store_verified_if_absent, ReceiptLookupKey,
    ReceiptStoreError, TransactionReceiptEvidence, TransactionReceiptMetadata,
};
use layerx_agentd::store::{Store, TenantId};
use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::result::{KnownResult, ResultCode, Retriability};
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use sha2::{Digest, Sha256};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

#[derive(Clone)]
struct Fields {
    activity_id: [u8; 32],
    global_sequence: u64,
    previous_state_root: [u8; 32],
    resulting_state_root: [u8; 32],
    batch_id: [u8; 32],
    asset: [u8; 32],
    result_code: i32,
}

fn fields(result_code: i32) -> Fields {
    Fields {
        activity_id: [1; 32],
        global_sequence: 9,
        previous_state_root: [2; 32],
        resulting_state_root: [3; 32],
        batch_id: [4; 32],
        asset: [5; 32],
        result_code,
    }
}

fn encode_fields(fields: &Fields, signature: Option<[u8; 64]>) -> Vec<u8> {
    let successful = fields.result_code == 0;
    let mut encoder = Encoder::new(4096);
    assert_eq!(
        encoder.structure_header_version(0x5201, layerx_wire::limits::PROTOCOL_VERSION),
        Ok(())
    );
    assert_eq!(encoder.u16(layerx_wire::limits::PROTOCOL_VERSION), Ok(()));
    assert_eq!(encoder.bytes(&fields.activity_id, 32), Ok(()));
    assert_eq!(encoder.u64(fields.global_sequence), Ok(()));
    assert_eq!(encoder.bytes(&fields.previous_state_root, 32), Ok(()));
    assert_eq!(encoder.bytes(&fields.resulting_state_root, 32), Ok(()));
    assert_eq!(encoder.bytes(&[8; 32], 32), Ok(()));
    assert_eq!(encoder.i32(fields.result_code), Ok(()));
    assert_eq!(encoder.sequence_length(0, 512), Ok(()));
    assert_eq!(encoder.u128(1), Ok(()));
    assert_eq!(encoder.bytes(&fields.batch_id, 32), Ok(()));
    assert_eq!(encoder.u16(1), Ok(()));
    assert_eq!(encoder.u32(1), Ok(()));
    assert_eq!(encoder.u32(1), Ok(()));
    assert_eq!(encoder.u8(1), Ok(()));
    assert_eq!(encoder.bytes(&fields.asset, 32), Ok(()));
    assert_eq!(encoder.u128(25), Ok(()));
    assert_eq!(encoder.bytes(&[6; 32], 32), Ok(()));
    assert_eq!(encoder.u128(100), Ok(()));
    assert_eq!(encoder.u128(if successful { 75 } else { 100 }), Ok(()));
    assert_eq!(encoder.u64(1), Ok(()));
    assert_eq!(encoder.bytes(&[7; 32], 32), Ok(()));
    assert_eq!(encoder.u128(10), Ok(()));
    assert_eq!(encoder.u128(if successful { 35 } else { 10 }), Ok(()));
    assert_eq!(encoder.bytes(&[9; 32], 32), Ok(()));
    assert_eq!(encoder.bytes(&[10; 32], 32), Ok(()));
    assert_eq!(encoder.bytes(&[11; 32], 32), Ok(()));
    assert_eq!(encoder.u64(1_000), Ok(()));
    assert_eq!(encoder.u8(u8::from(signature.is_some())), Ok(()));
    if let Some(value) = signature {
        assert_eq!(encoder.bytes(&value, 64), Ok(()));
    }
    encoder.finish()
}

fn sign(fields: &Fields, signing_key: &SigningKey) -> Vec<u8> {
    let unsigned = encode_fields(fields, None);
    let digest = receipt_digest(&unsigned)
        .unwrap_or_else(|error| panic!("receipt hashing failed: {error:?}"));
    encode_fields(fields, Some(signing_key.sign(&digest).to_bytes()))
}

fn authorised(fields: &Fields, signing_key: &SigningKey) -> AuthorizedBatch {
    AuthorizedBatch::new(
        fields.batch_id,
        fields.asset,
        fields.previous_state_root,
        fields.resulting_state_root,
        signing_key.verifying_key().to_bytes(),
    )
}

fn directory(label: &str) -> std::path::PathBuf {
    let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-receipts-{label}-{}-{sequence}",
        std::process::id()
    ))
}

fn tenant() -> TenantId {
    TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}"))
}

fn transaction_receipt(hash: [u8; 32]) -> TransactionReceiptEvidence {
    let canonical_receipt = [
        &[0xf9, 0x01, 0x08, 0x01, 0x82, 0x52, 0x08, 0xb9, 0x01, 0x00][..],
        &[0; 256],
        &[0xc0],
    ]
    .concat();
    let signing_key = k256::ecdsa::SigningKey::from_slice(&[7; 32])
        .unwrap_or_else(|error| panic!("signing key: {error}"));
    let digest: [u8; 32] = Sha256::digest(&canonical_receipt).into();
    let (signature, recovery) = signing_key
        .sign_prehash_recoverable(&digest)
        .unwrap_or_else(|error| panic!("receipt signature: {error}"));
    let mut receipt_signature = [0; 65];
    receipt_signature[..64].copy_from_slice(&signature.to_bytes());
    receipt_signature[64] = recovery.to_byte() + 27;
    TransactionReceiptEvidence {
        metadata: TransactionReceiptMetadata {
            transaction_hash: hash,
            block_height: 12,
            tx_index: 1,
            status: true,
            gas_used: 21_000,
            logs_bloom: [0; 256],
            receipt_signature,
            rung: Rung::Receipted,
        },
        canonical_receipt,
    }
}

#[test]
fn verified_bytes_survive_all_three_indexes_and_restart_unchanged() {
    let root = directory("indexes");
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let fields = fields(0);
    let exact = sign(&fields, &signing_key);
    let idem = [0x44; 32];
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let metadata = store(
        &mut durable,
        tenant(),
        idem,
        &exact,
        &authorised(&fields, &signing_key),
    )
    .unwrap_or_else(|error| panic!("record: {error:?}"));
    assert_eq!(
        metadata.verification_level,
        VerificationLevel::SEQUENCER_SIGNED
    );
    assert_eq!(metadata.result.code.raw(), 0);
    drop(durable);

    let reopened = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
    for lookup in [
        ReceiptLookupKey::Activity(fields.activity_id),
        ReceiptLookupKey::Idempotency(idem),
        ReceiptLookupKey::GlobalSequence(fields.global_sequence),
    ] {
        let served =
            serve(&reopened, tenant(), lookup).unwrap_or_else(|error| panic!("serve: {error:?}"));
        assert_eq!(served.canonical_bytes, exact);
        assert_eq!(served.metadata, metadata);
    }
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn first_verified_ingress_is_stored_before_it_is_served() {
    let root = directory("first-ingress");
    let idem = [0x45; 32];
    let record = transaction_receipt(idem);
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    assert_eq!(
        serve_evidence(&durable, tenant(), idem).unwrap_or_else(|error| panic!("{error:?}")),
        None
    );
    let served = store_verified_if_absent(
        &mut durable,
        tenant(),
        idem,
        &record.canonical_receipt,
        &record,
    )
    .unwrap_or_else(|error| panic!("first ingress: {error:?}"));
    assert_eq!(served, record);
    assert_eq!(served.metadata.rung, Rung::Receipted);
    assert_eq!(
        serve_evidence(&durable, tenant(), idem).unwrap_or_else(|error| panic!("{error:?}")),
        Some(record)
    );
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn repeated_verified_ingress_refuses_conflicts_without_replacing_the_first_receipt() {
    let root = directory("ingress-conflict");
    let idem = [0x46; 32];
    let original = transaction_receipt(idem);
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    store_verified_if_absent(
        &mut durable,
        tenant(),
        idem,
        &original.canonical_receipt,
        &original,
    )
    .unwrap_or_else(|error| panic!("first ingress: {error:?}"));

    let mut conflicting = original.clone();
    conflicting.metadata.gas_used += 1;
    assert!(matches!(
        store_verified_if_absent(
            &mut durable,
            tenant(),
            idem,
            &conflicting.canonical_receipt,
            &conflicting,
        ),
        Err(ReceiptStoreError::Store(_))
    ));
    let served = serve_evidence(&durable, tenant(), idem)
        .unwrap_or_else(|error| panic!("serve original: {error:?}"));
    assert_eq!(served, Some(original));
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn rejected_receipt_preserves_exact_code_and_canonical_taxonomy() {
    let root = directory("rejection");
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let fields = fields(KnownResult::BadSignature.raw());
    let exact = sign(&fields, &signing_key);
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let metadata = store(
        &mut durable,
        tenant(),
        [0x55; 32],
        &exact,
        &authorised(&fields, &signing_key),
    )
    .unwrap_or_else(|error| panic!("record rejection: {error:?}"));
    assert_eq!(metadata.result.code.raw(), KnownResult::BadSignature.raw());
    assert_eq!(metadata.result.canonical, Some(KnownResult::BadSignature));
    assert_eq!(metadata.result.retriability, Retriability::Terminal);
    assert!(!metadata.result.retry_permitted);
    let served = serve(
        &durable,
        tenant(),
        ReceiptLookupKey::Idempotency([0x55; 32]),
    )
    .unwrap_or_else(|error| panic!("serve rejection: {error:?}"));
    assert_eq!(served.canonical_bytes, exact);
    assert_eq!(served.metadata.result.code.raw(), -201);
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn every_terminal_protocol_code_class_is_never_retried() {
    for known in KnownResult::ALL {
        let result = classify(ResultCode::from(*known));
        assert_eq!(result.code.raw(), known.raw());
        assert_eq!(result.canonical, Some(*known));
        if known.retriability() == Retriability::Terminal {
            assert!(!result.retry_permitted, "terminal code {known:?} retried");
        } else {
            assert!(result.retry_permitted, "retriable code {known:?} blocked");
        }
    }
    let future_unknown = classify(ResultCode::from_raw(-7_777));
    assert_eq!(future_unknown.code.raw(), -7_777);
    assert_eq!(future_unknown.canonical, None);
    assert!(!future_unknown.retry_permitted);
}

#[test]
fn unverifiable_or_conflicting_receipts_are_never_recorded() {
    let root = directory("invalid");
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let fields = fields(0);
    let exact = sign(&fields, &signing_key);
    let mut altered = exact.clone();
    let last = altered.len() - 1;
    altered[last] ^= 1;
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    assert!(matches!(
        store(
            &mut durable,
            tenant(),
            [0x66; 32],
            &altered,
            &authorised(&fields, &signing_key),
        ),
        Err(ReceiptStoreError::Verification(_))
    ));
    assert!(matches!(
        serve(
            &durable,
            tenant(),
            ReceiptLookupKey::Idempotency([0x66; 32])
        ),
        Err(ReceiptStoreError::Missing)
    ));

    store(
        &mut durable,
        tenant(),
        [0x66; 32],
        &exact,
        &authorised(&fields, &signing_key),
    )
    .unwrap_or_else(|error| panic!("first record: {error:?}"));
    let mut other = fields.clone();
    other.activity_id = [9; 32];
    let other_exact = sign(&other, &signing_key);
    assert!(matches!(
        store(
            &mut durable,
            tenant(),
            [0x66; 32],
            &other_exact,
            &authorised(&other, &signing_key),
        ),
        Err(ReceiptStoreError::Store(_))
    ));
    let _ = std::fs::remove_dir_all(root);
}
