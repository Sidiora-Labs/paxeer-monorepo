use std::fs;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::subscription::{
    Cursor as ApiCursor, EventDelivery, EventIdentity, ReceiptReference,
};
use layerx_agent_api::track::ReceiptRef;
use layerx_agent_api::verify::Level;
use layerx_agent_api::Sequence;
use layerx_human_service::activity::detail::{
    DetailError, EvidenceKind, FinalityReference, ReceiptActual,
};
use layerx_human_service::activity::{
    ActivityEntry, ActivityKind, ActivityStatus, AgentActivity, EntryDetail, Feed, PendingStatus,
    VerifiedStatus,
};
use layerx_human_service::notify::ActivityEntryId;
use layerx_human_test_support as support;
use layerx_proof::receipt::{verify, AuthorizedBatch, VerifiedReceipt};
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use sha2::{Digest as _, Sha256};

#[derive(Clone)]
struct ReceiptFields {
    activity_id: [u8; 32],
    previous_state_root: [u8; 32],
    resulting_state_root: [u8; 32],
    batch_id: [u8; 32],
    asset: [u8; 32],
}

fn push_bytes(output: &mut Vec<u8>, value: &[u8]) {
    let length = u32::try_from(value.len()).unwrap_or_else(|_| panic!("receipt field overflow"));
    output.extend_from_slice(&length.to_be_bytes());
    output.extend_from_slice(value);
}

fn encode_receipt(fields: &ReceiptFields, signature: Option<[u8; 64]>) -> Vec<u8> {
    let mut bytes = Vec::new();
    bytes.extend_from_slice(&layerx_intents::canonical::PROTOCOL_VERSION.to_be_bytes());
    bytes.extend_from_slice(&0x5201_u16.to_be_bytes());
    bytes.extend_from_slice(&layerx_intents::canonical::PROTOCOL_VERSION.to_be_bytes());
    push_bytes(&mut bytes, &fields.activity_id);
    bytes.extend_from_slice(&17_u64.to_be_bytes());
    push_bytes(&mut bytes, &fields.previous_state_root);
    push_bytes(&mut bytes, &fields.resulting_state_root);
    push_bytes(&mut bytes, &[0x81; 32]);
    bytes.extend_from_slice(&0_i32.to_be_bytes());
    bytes.extend_from_slice(&0_u32.to_be_bytes());
    bytes.extend_from_slice(&1_u128.to_be_bytes());
    push_bytes(&mut bytes, &fields.batch_id);
    bytes.extend_from_slice(&1_u16.to_be_bytes());
    bytes.extend_from_slice(&1_u32.to_be_bytes());
    bytes.extend_from_slice(&1_u32.to_be_bytes());
    bytes.push(1);
    push_bytes(&mut bytes, &fields.asset);
    bytes.extend_from_slice(&25_u128.to_be_bytes());
    push_bytes(&mut bytes, &[0x91; 32]);
    bytes.extend_from_slice(&100_u128.to_be_bytes());
    bytes.extend_from_slice(&75_u128.to_be_bytes());
    bytes.extend_from_slice(&1_u64.to_be_bytes());
    push_bytes(&mut bytes, &[0x92; 32]);
    bytes.extend_from_slice(&10_u128.to_be_bytes());
    bytes.extend_from_slice(&35_u128.to_be_bytes());
    push_bytes(&mut bytes, &[0x93; 32]);
    push_bytes(&mut bytes, &[0x94; 32]);
    push_bytes(&mut bytes, &[0x95; 32]);
    bytes.extend_from_slice(&1_000_u64.to_be_bytes());
    bytes.push(u8::from(signature.is_some()));
    if let Some(signature) = signature {
        push_bytes(&mut bytes, &signature);
    }
    bytes
}

fn verified_receipt() -> VerifiedReceipt {
    let fields = ReceiptFields {
        activity_id: [0x11; 32],
        previous_state_root: [0x12; 32],
        resulting_state_root: [0x13; 32],
        batch_id: [0x14; 32],
        asset: [0x15; 32],
    };
    let signing_key = SigningKey::from_bytes(&[0x16; 32]);
    let unsigned = encode_receipt(&fields, None);
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    digest.update(&unsigned);
    let signature = signing_key.sign(&<[u8; 32]>::from(digest.finalize()));
    let canonical = encode_receipt(&fields, Some(signature.to_bytes()));
    let authorised = AuthorizedBatch::new(
        fields.batch_id,
        fields.asset,
        fields.previous_state_root,
        fields.resulting_state_root,
        signing_key.verifying_key().to_bytes(),
    );
    let receipt = verify(&canonical, &authorised)
        .unwrap_or_else(|error| panic!("receipt verification: {error:?}"));
    assert_eq!(receipt.level(), VerificationLevel::SEQUENCER_SIGNED);
    receipt
}

fn recorded_entry(level: Level) -> (ActivityEntry, ReceiptActual) {
    let root = support::directory("activity-detail-rungs");
    let tenancy = support::tenancy(&[("alice", "tenant-a")]);
    let (mut store, _) =
        support::install_and_open(&root, &tenancy, support::retention_uniform(100));
    let mut scope = store
        .principal(&support::principal("alice"))
        .unwrap_or_else(|error| panic!("scope: {error}"));
    let receipt = verified_receipt();
    let actual = ReceiptActual::from_verified(&receipt)
        .unwrap_or_else(|error| panic!("receipt actual: {error}"));
    let delivery = EventDelivery::new(
        EventIdentity::new([1; 32]),
        receipt.canonical_bytes().to_vec(),
        ApiCursor(Sequence(1)),
        ReceiptReference::Verified {
            receipt_ref: ReceiptRef::new(actual.reference())
                .unwrap_or_else(|error| panic!("receipt reference: {error:?}")),
            verification_level: level,
        },
    )
    .unwrap_or_else(|error| panic!("delivery: {error:?}"));
    let activity = AgentActivity::new(
        ActivityEntryId::new("act_detailrung").unwrap_or_else(|error| panic!("entry id: {error}")),
        ActivityKind::Movement,
        None,
        1,
        PendingStatus::Processing,
        VerifiedStatus::Done,
    )
    .unwrap_or_else(|error| panic!("activity: {error}"));
    let entry = Feed::record_agent_event(&mut scope, &activity, &delivery, 2)
        .unwrap_or_else(|error| panic!("feed entry: {error}"));
    fs::remove_dir_all(root).unwrap_or_else(|error| panic!("cleanup: {error}"));
    (entry, actual)
}

#[test]
fn receipted_entry_detail_is_not_final() {
    for level in [
        Level::SequencerSigned,
        Level::BatchIncluded,
        Level::StateProven,
    ] {
        let (entry, actual) = recorded_entry(level);
        assert_eq!(entry.rung(), Rung::Receipted);
        let reference = FinalityReference::new([0x77; 32], Rung::Receipted)
            .unwrap_or_else(|error| panic!("pending finality: {error}"));
        assert_eq!(reference.rung(), Rung::Receipted);
        assert!(!reference.is_final());
        for finality in [None, Some(reference)] {
            let detail = EntryDetail::assemble(&entry, vec![actual.clone()], finality)
                .unwrap_or_else(|error| panic!("receipted detail: {error}"));
            assert_eq!(detail.rung(), Rung::Receipted);
            assert!(!detail.is_final());
            assert_eq!(detail.status(), ActivityStatus::Done);
            assert_eq!(detail.finality(), finality);
            assert_eq!(detail.evidence()[0].level(), level);
            if finality.is_some() {
                assert_eq!(detail.evidence().len(), 2);
                assert_eq!(detail.evidence()[1].kind(), EvidenceKind::Checkpoint);
                assert_eq!(detail.evidence()[1].level(), Level::SequencerSigned);
                assert!(!detail
                    .finality()
                    .unwrap_or_else(|| panic!("missing finality"))
                    .is_final());
            } else {
                assert_eq!(detail.evidence().len(), 1);
            }
        }
        let committed = FinalityReference::new([0x77; 32], Rung::Committed)
            .unwrap_or_else(|error| panic!("committed reference: {error}"));
        assert_eq!(
            EntryDetail::assemble(&entry, vec![actual], Some(committed)),
            Err(DetailError::UnjustifiedFinalityEvidence)
        );
        assert_eq!(
            FinalityReference::new([0; 32], Rung::Receipted),
            Err(DetailError::InvalidFinalityEvidence)
        );
    }
}

#[test]
fn committed_entry_detail_is_final() {
    for level in [Level::CheckpointFinalised, Level::SettlementAnchored] {
        let (entry, actual) = recorded_entry(level);
        assert_eq!(entry.rung(), Rung::Committed);
        let reference = FinalityReference::new([0x77; 32], Rung::Committed)
            .unwrap_or_else(|error| panic!("committed finality: {error}"));
        assert_eq!(reference.rung(), Rung::Committed);
        assert!(reference.is_final());
        let detail = EntryDetail::assemble(&entry, vec![actual.clone()], Some(reference))
            .unwrap_or_else(|error| panic!("committed detail: {error}"));
        assert_eq!(detail.rung(), Rung::Committed);
        assert!(detail.is_final());
        assert_eq!(detail.status(), ActivityStatus::DoneFinalised);
        assert_eq!(detail.finality(), Some(reference));
        assert!(detail
            .finality()
            .unwrap_or_else(|| panic!("missing finality"))
            .is_final());
        assert_eq!(detail.evidence().len(), 2);
        assert_eq!(detail.evidence()[0].level(), level);
        assert_eq!(detail.evidence()[1].kind(), EvidenceKind::Checkpoint);
        assert_eq!(detail.evidence()[1].level(), Level::CheckpointFinalised);
        assert_eq!(
            EntryDetail::assemble(&entry, vec![actual.clone()], None),
            Err(DetailError::MissingFinalityEvidence)
        );
        let pending = FinalityReference::new([0x77; 32], Rung::Receipted)
            .unwrap_or_else(|error| panic!("pending reference: {error}"));
        assert_eq!(
            EntryDetail::assemble(&entry, vec![actual], Some(pending)),
            Err(DetailError::UnjustifiedFinalityEvidence)
        );
        assert_eq!(
            FinalityReference::new([0; 32], Rung::Committed),
            Err(DetailError::InvalidFinalityEvidence)
        );
    }
}
