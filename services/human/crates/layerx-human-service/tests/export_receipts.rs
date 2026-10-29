use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::subscription::{Cursor, EventDelivery, EventIdentity, ReceiptReference};
use layerx_agent_api::track::ReceiptRef;
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_human_service::activity::{
    ActivityKind, AgentActivity, EvidenceBundle, EvidenceExport, ExportError, Feed, FilterDraft,
    PendingStatus, ReceiptExport, ReceiptFact, UnverifiedReason, VerifiedStatus, VerifyError,
};
use layerx_human_service::notify::ActivityEntryId;
use layerx_human_test_support as support;
use layerx_intents::vectors::{receipt as receipt_vector, receipt_signing_digest};
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};

fn fixture(rung: Rung) -> (ActivityEntryId, ReceiptFact, Vec<ReceiptKeyEntry>) {
    let (id, tx_hash, height) = match rung {
        Rung::Committed => ("act_receiptexport2", [2; 32], 21_u64),
        Rung::Receipted | Rung::Submitted => ("act_receiptexport1", [1; 32], 20_u64),
    };
    let id = ActivityEntryId::new(id).expect("entry id");
    let key = SigningKey::from_bytes(&[3; 32]);
    let encode = |signature| {
        let mut bytes = receipt_vector(signature).expect("receipt vector");
        let positions = bytes
            .windows(32)
            .enumerate()
            .filter(|(_, bytes)| *bytes == [4; 32])
            .map(|(index, _)| index)
            .collect::<Vec<_>>();
        assert_eq!(positions.len(), 1);
        let mut batch = [0; 32];
        batch[24..].copy_from_slice(&height.to_be_bytes());
        bytes[positions[0]..positions[0] + 32].copy_from_slice(&batch);
        let positions = bytes
            .windows(32)
            .enumerate()
            .filter(|(_, bytes)| *bytes == [1; 32])
            .map(|(index, _)| index)
            .collect::<Vec<_>>();
        assert_eq!(positions.len(), 1);
        bytes[positions[0]..positions[0] + 32].copy_from_slice(&tx_hash);
        bytes
    };
    let digest = receipt_signing_digest(&encode(None)).expect("signing digest");
    let bytes = encode(Some(key.sign(&digest).to_bytes()));
    let fact = EvidenceBundle::receipt_fact(&id, bytes, rung).expect("receipt fact");
    let keys = vec![ReceiptKeyEntry {
        key_id: [11; 32],
        public_key: key.verifying_key().to_bytes(),
        first_height: 10,
        last_height: 30,
    }];
    (id, fact, keys)
}

#[test]
fn evidence_bundle_v2_round_trips_receipt_fields() {
    let fixtures = [fixture(Rung::Receipted), fixture(Rung::Committed)];
    let root = support::directory("receipt-export-round-trip");
    let tenancy = support::tenancy(&[("alice", "tenant-a")]);
    let (mut store, _) =
        support::install_and_open(&root, &tenancy, support::retention_uniform(100));
    let principal = support::principal("alice");
    let mut scope = store.principal(&principal).expect("principal scope");
    for (index, (id, fact, _)) in fixtures.iter().enumerate() {
        let reference = fact
            .receipt_hash
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>();
        let delivery = EventDelivery::new(
            EventIdentity::new([index as u8 + 1; 32]),
            fact.canonical_receipt_bytes.clone(),
            Cursor {
                block_height: index as u64 + 1,
                tx_index: 0,
                log_index: 0,
            },
            ReceiptReference::Verified {
                receipt_ref: ReceiptRef::new(reference).expect("receipt reference"),
                rung: fact.rung,
            },
        )
        .expect("delivery");
        let activity = AgentActivity::new(
            id.clone(),
            ActivityKind::Movement,
            None,
            fact.block_height,
            PendingStatus::Processing,
            VerifiedStatus::Done,
        )
        .expect("activity");
        Feed::record_agent_event(&mut scope, &activity, &delivery, 22).expect("project receipt");
    }
    let protocol = fixtures
        .iter()
        .map(|(_, fact, _)| ReceiptExport {
            receipts: vec![fact.clone()],
            inclusions: Vec::new(),
            checkpoints: Vec::new(),
            derived_aggregates: Vec::new(),
        })
        .collect();
    let (bundle, report) = EvidenceExport::new(Feed::new(5).expect("feed"), 64 * 1024)
        .expect("exporter")
        .evidence(
            &scope,
            &Feed::apply_filters(FilterDraft::new()).expect("filters"),
            &fixtures
                .iter()
                .map(|(id, _, _)| id.clone())
                .collect::<Vec<_>>(),
            protocol,
            &fixtures[0].2,
            22,
            2,
        )
        .expect("bundle");
    assert_eq!(report.entries(), 2);
    let bytes = bundle.encode().expect("encode");
    assert_eq!(&bytes[..8], b"LXHEXPV2");
    assert_eq!(bytes[8], 2);
    let decoded = EvidenceBundle::decode(&bytes).expect("decode v2");
    assert_eq!(decoded.encode().expect("re-encode"), bytes);
    assert_eq!(decoded.entries().len(), 2);
    assert_eq!(decoded.principal_binding(), bundle.principal_binding());
    for (id, fact, _) in &fixtures {
        let entry = decoded
            .entries()
            .iter()
            .find(|entry| entry.entry_id() == id)
            .expect("entry");
        assert_eq!(entry.tx_hash(), fact.tx_hash);
        assert_eq!(
            entry.receipt_hash(),
            <[u8; 32]>::from(Sha256::digest(&fact.canonical_receipt_bytes))
        );
        assert_eq!(entry.block_height(), fact.block_height);
        assert_eq!(entry.rung(), fact.rung);
        assert!(decoded
            .protocol_evidence()
            .iter()
            .any(|export| export.receipts == [fact.clone()]));
    }
    assert_eq!(
        decoded
            .verify(
                decoded.digest().expect("digest"),
                &principal,
                &fixtures[0].2
            )
            .expect("verify")
            .verified_receipts(),
        2
    );
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn evidence_bundle_v1_is_refused() {
    let mut legacy = b"LXHEXP02".to_vec();
    legacy.extend_from_slice(&[0x77; 32]);
    legacy.extend_from_slice(&31_337_u64.to_be_bytes());
    legacy.extend_from_slice(&[0x55; 20]);
    legacy.extend_from_slice(&0_u32.to_be_bytes());
    legacy.extend_from_slice(&0_u32.to_be_bytes());
    legacy.push(0);
    assert!(matches!(
        EvidenceBundle::decode(&legacy),
        Err(ExportError::UnsupportedVersion { version: 1 })
    ));
    let mut legacy_version = b"LXHEXPV2".to_vec();
    legacy_version.push(1);
    legacy_version.extend_from_slice(&legacy[8..]);
    assert!(matches!(
        EvidenceBundle::decode(&legacy_version),
        Err(ExportError::UnsupportedVersion { version: 1 })
    ));
    legacy[..8].copy_from_slice(b"LXHEXP01");
    assert!(matches!(
        EvidenceBundle::decode(&legacy),
        Err(ExportError::UnsupportedVersion { version: 1 })
    ));
}

#[test]
fn tampered_receipt_hash_fails_verify_receipt() {
    let (_, mut fact, keys) = fixture(Rung::Receipted);
    assert_eq!(
        EvidenceBundle::verify_receipt(&fact, &keys)
            .expect("valid receipt")
            .verified_receipts(),
        1
    );
    let original = fact.clone();
    fact.receipt_hash[0] ^= 1;
    assert!(matches!(
        EvidenceBundle::verify_receipt(&fact, &keys),
        Err(VerifyError::Unverified(UnverifiedReason::DigestMismatch))
    ));
    fact = original.clone();
    let last = fact.canonical_receipt_bytes.len() - 1;
    fact.canonical_receipt_bytes[last] ^= 1;
    assert!(matches!(
        EvidenceBundle::verify_receipt(&fact, &keys),
        Err(VerifyError::Unverified(UnverifiedReason::DigestMismatch))
    ));
    fact.receipt_hash = Sha256::digest(&fact.canonical_receipt_bytes).into();
    assert!(matches!(
        EvidenceBundle::verify_receipt(&fact, &keys),
        Err(VerifyError::Unverified(UnverifiedReason::AuthorityMismatch))
    ));
    let mut wrong_key = keys.clone();
    wrong_key[0].public_key = SigningKey::from_bytes(&[9; 32]).verifying_key().to_bytes();
    assert!(matches!(
        EvidenceBundle::verify_receipt(&original, &wrong_key),
        Err(VerifyError::Unverified(UnverifiedReason::AuthorityMismatch))
    ));
    let mut wrong_height = original.clone();
    wrong_height.block_height += 1;
    assert!(matches!(
        EvidenceBundle::verify_receipt(&wrong_height, &keys),
        Err(VerifyError::Unverified(UnverifiedReason::Tampered))
    ));
    let mut wrong_tx = original.clone();
    wrong_tx.tx_hash[0] ^= 1;
    assert!(matches!(
        EvidenceBundle::verify_receipt(&wrong_tx, &keys),
        Err(VerifyError::Unverified(UnverifiedReason::Tampered))
    ));
    let mut expired = keys.clone();
    expired[0].last_height = 19;
    assert!(matches!(
        EvidenceBundle::verify_receipt(&original, &expired),
        Err(VerifyError::Unverified(UnverifiedReason::AuthorityMismatch))
    ));
    let duplicate_keys = vec![keys[0], keys[0]];
    assert!(matches!(
        EvidenceBundle::verify_receipt(&original, &duplicate_keys),
        Err(VerifyError::Unverified(UnverifiedReason::AuthorityMismatch))
    ));
    let mut submitted = original.clone();
    submitted.rung = Rung::Submitted;
    assert!(matches!(
        EvidenceBundle::verify_receipt(&submitted, &keys),
        Err(VerifyError::Unverified(UnverifiedReason::Tampered))
    ));
    assert!(matches!(
        EvidenceBundle::verify_receipt(&original, &[]),
        Err(VerifyError::Unavailable(
            ExportError::AuthorityUnavailable { .. }
        ))
    ));
}
