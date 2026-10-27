use std::sync::atomic::{AtomicU64, Ordering};

use k256::ecdsa::SigningKey;
use layerx_agentd::receipt::{
    serve_evidence, store_verified_if_absent, ReceiptStoreError, TransactionReceiptEvidence,
    TransactionReceiptMetadata,
};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_proof::merkle::{encode_proof, Proof};
use layerx_types::rung::Rung;
use sha2::{Digest, Sha256};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

fn directory() -> std::path::PathBuf {
    std::env::temp_dir().join(format!(
        "layerx-receipt-v2-{}-{}",
        std::process::id(),
        NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
    ))
}

fn tenant(name: &str) -> TenantId {
    TenantId::new(name).unwrap_or_else(|error| panic!("tenant: {error}"))
}

fn evidence_key(tenant: TenantId, hash: [u8; 32]) -> TenantKey {
    TenantKey::new(
        tenant,
        ObjectKind::Configuration,
        [b"receipt-evidence:".as_slice(), &hash].concat(),
    )
    .unwrap_or_else(|error| panic!("key: {error}"))
}

fn record(hash: [u8; 32], status: bool, rung: Rung) -> TransactionReceiptEvidence {
    let mut bloom = [0; 256];
    for (value, byte) in (0_u8..=255).zip(bloom.iter_mut()) {
        *byte = value;
    }
    let canonical_receipt = [
        &[
            0xf9,
            0x01,
            0x08,
            if status { 1 } else { 0x80 },
            0x82,
            0x52,
            0x08,
            0xb9,
            0x01,
            0x00,
        ][..],
        &bloom,
        &[0xc0],
    ]
    .concat();
    let key =
        SigningKey::from_slice(&[7; 32]).unwrap_or_else(|error| panic!("signing key: {error}"));
    let digest: [u8; 32] = Sha256::digest(&canonical_receipt).into();
    let (signature, recovery) = key
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
            status,
            gas_used: 21_000,
            logs_bloom: bloom,
            receipt_signature,
            rung,
        },
        canonical_receipt,
    }
}

#[test]
fn receipt_evidence_round_trips_by_transaction_hash() {
    let root = directory();
    let owner = tenant("receipt-owner");
    let other = tenant("another-owner");
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    for (hash, status, rung) in [
        ([0x42; 32], true, Rung::Receipted),
        ([0x43; 32], false, Rung::Committed),
    ] {
        let expected = record(hash, status, rung);
        assert_eq!(
            serve_evidence(&durable, owner.clone(), hash)
                .unwrap_or_else(|error| panic!("{error:?}")),
            None
        );
        let mut submitted = expected.clone();
        submitted.metadata.rung = Rung::Submitted;
        assert!(matches!(
            store_verified_if_absent(
                &mut durable,
                owner.clone(),
                hash,
                &submitted.canonical_receipt,
                &submitted
            ),
            Err(ReceiptStoreError::BelowReceipted)
        ));
        assert!(durable.get(&evidence_key(owner.clone(), hash)).is_none());
        for _ in 0..2 {
            let stored = store_verified_if_absent(
                &mut durable,
                owner.clone(),
                hash,
                &expected.canonical_receipt,
                &expected,
            )
            .unwrap_or_else(|error| panic!("store receipt: {error:?}"));
            assert_eq!(stored, expected);
        }
        if rung == Rung::Committed {
            let mut weaker = expected.clone();
            weaker.metadata.rung = Rung::Receipted;
            assert_eq!(
                store_verified_if_absent(
                    &mut durable,
                    owner.clone(),
                    hash,
                    &weaker.canonical_receipt,
                    &weaker
                )
                .unwrap_or_else(|error| panic!("{error:?}")),
                expected
            );
        }
        let key = evidence_key(owner.clone(), hash);
        let encoded = durable
            .get(&key)
            .unwrap_or_else(|| panic!("missing stored evidence"))
            .bytes()
            .to_vec();
        assert_eq!(&encoded[..5], b"LXRE\x02");
        assert_eq!(&encoded[5..10], b"LXRM\x02");
        assert_eq!(
            serve_evidence(&durable, owner.clone(), hash)
                .unwrap_or_else(|error| panic!("{error:?}")),
            Some(expected.clone())
        );
        assert_eq!(
            serve_evidence(&durable, other.clone(), hash)
                .unwrap_or_else(|error| panic!("{error:?}")),
            None
        );
        assert_eq!(
            serve_evidence(&durable, owner.clone(), [0x99; 32])
                .unwrap_or_else(|error| panic!("{error:?}")),
            None
        );

        let mut conflicting = expected.clone();
        conflicting.metadata.gas_used += 1;
        assert!(matches!(
            store_verified_if_absent(
                &mut durable,
                owner.clone(),
                hash,
                &conflicting.canonical_receipt,
                &conflicting
            ),
            Err(ReceiptStoreError::Store(_))
        ));
        assert!(matches!(
            store_verified_if_absent(
                &mut durable,
                owner.clone(),
                [0x99; 32],
                &expected.canonical_receipt,
                &expected
            ),
            Err(ReceiptStoreError::Corrupt)
        ));
        assert!(matches!(
            store_verified_if_absent(
                &mut durable,
                owner.clone(),
                hash,
                b"altered receipt",
                &expected
            ),
            Err(ReceiptStoreError::Corrupt)
        ));
        assert!(matches!(
            store_verified_if_absent(
                &mut durable,
                owner.clone(),
                hash,
                &submitted.canonical_receipt,
                &submitted
            ),
            Err(ReceiptStoreError::BelowReceipted)
        ));
        assert_eq!(
            durable
                .get(&key)
                .unwrap_or_else(|| panic!("missing stored evidence"))
                .bytes(),
            encoded
        );
        drop(durable);
        durable = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
        let served = serve_evidence(&durable, owner.clone(), hash)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .unwrap_or_else(|| panic!("missing served evidence"));
        assert_eq!(served, expected);
        assert_eq!(served.canonical_receipt, expected.canonical_receipt);
        assert_eq!(
            durable
                .get(&key)
                .unwrap_or_else(|| panic!("missing stored evidence"))
                .bytes(),
            encoded
        );

        for malformed in [
            encoded[..4].to_vec(),
            encoded[..10].to_vec(),
            encoded[..encoded.len() - 1].to_vec(),
            [encoded.as_slice(), &[0]].concat(),
        ] {
            durable
                .put_local(key.clone(), malformed)
                .unwrap_or_else(|error| panic!("{error:?}"));
            assert!(matches!(
                serve_evidence(&durable, owner.clone(), hash),
                Err(ReceiptStoreError::Corrupt)
            ));
        }
        for (offset, value) in [(10, 0), (58, 2), (388, 3)] {
            let mut malformed = encoded.clone();
            malformed[offset] = value;
            durable
                .put_local(key.clone(), malformed)
                .unwrap_or_else(|error| panic!("{error:?}"));
            assert!(matches!(
                serve_evidence(&durable, owner.clone(), hash),
                Err(ReceiptStoreError::Corrupt)
            ));
        }
        let mut submitted_bytes = encoded.clone();
        submitted_bytes[388] = Rung::Submitted.wire_rank();
        durable
            .put_local(key.clone(), submitted_bytes)
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert!(matches!(
            serve_evidence(&durable, owner.clone(), hash),
            Err(ReceiptStoreError::BelowReceipted)
        ));
        durable
            .put_local(key, encoded)
            .unwrap_or_else(|error| panic!("{error:?}"));
    }
    let receipted = record([0x42; 32], true, Rung::Receipted);
    let mut committed = receipted.clone();
    committed.metadata.rung = Rung::Committed;
    assert_eq!(
        store_verified_if_absent(
            &mut durable,
            owner.clone(),
            [0x42; 32],
            &committed.canonical_receipt,
            &committed,
        )
        .unwrap_or_else(|error| panic!("{error:?}")),
        committed
    );
    assert_eq!(
        store_verified_if_absent(
            &mut durable,
            owner.clone(),
            [0x42; 32],
            &receipted.canonical_receipt,
            &receipted,
        )
        .unwrap_or_else(|error| panic!("{error:?}")),
        committed
    );
    assert_eq!(
        serve_evidence(&durable, owner, [0x42; 32]).unwrap_or_else(|error| panic!("{error:?}")),
        Some(committed)
    );
    drop(durable);
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("{error:?}"));
}

#[test]
fn version_one_receipt_evidence_is_refused() {
    let root = directory();
    let owner = tenant("receipt-owner");
    let hash = [0x55; 32];
    let current = record(hash, true, Rung::Receipted);
    let mut legacy = b"LXRE\x01".to_vec();
    legacy.extend_from_slice(&hash);
    legacy.extend_from_slice(&[0x66; 32]);
    legacy.extend_from_slice(&9_u64.to_be_bytes());
    let proof = encode_proof(&Proof::new(0, 1, vec![]).unwrap_or_else(|error| panic!("{error:?}")));
    for section in [
        current.canonical_receipt.as_slice(),
        proof.as_slice(),
        b"retained signed header",
    ] {
        legacy.extend_from_slice(
            &u32::try_from(section.len())
                .unwrap_or_else(|error| panic!("{error:?}"))
                .to_be_bytes(),
        );
        legacy.extend_from_slice(section);
    }
    legacy.extend_from_slice(&[0x77; 64]);
    let key = evidence_key(owner.clone(), hash);
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("{error:?}"));
    durable
        .put_local(key.clone(), legacy.clone())
        .unwrap_or_else(|error| panic!("{error:?}"));
    drop(durable);
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("{error:?}"));
    let error = serve_evidence(&durable, owner.clone(), hash).unwrap_err();
    assert!(matches!(error, ReceiptStoreError::EvidenceVersion));
    assert_eq!(error.reason(), "receipt.evidence_version");
    let error = store_verified_if_absent(
        &mut durable,
        owner.clone(),
        hash,
        &current.canonical_receipt,
        &current,
    )
    .unwrap_err();
    assert_eq!(error.reason(), "receipt.evidence_version");
    assert_eq!(
        durable
            .get(&key)
            .unwrap_or_else(|| panic!("missing stored evidence"))
            .bytes(),
        legacy
    );
    assert!(durable
        .list_object_ids(&owner, ObjectKind::Receipt)
        .is_empty());
    drop(durable);
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("{error:?}"));
}
