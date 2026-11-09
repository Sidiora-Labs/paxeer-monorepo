use ed25519_dalek::{Signer as _, SigningKey};
use layerx_programs::{
    account_tree_commitment, state_leaf_commitment, universal_root_commitment, CanonicalAccountLeaf,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::merkle::{build_proof, Proof};
use layerx_proof::state::{verify_nested_account, NestedAccountProof, VerifiedAccountState};
use layerx_wire::encode::Encoder;
use sha2::{Digest as _, Sha256};

use super::ServedCheckpoint;

pub(super) fn committed_account(
    checkpoint: &mut ServedCheckpoint,
    account: [u8; 32],
    asset: [u8; 32],
    balance: u128,
) -> VerifiedAccountState {
    let leaf = CanonicalAccountLeaf {
        account_id: account,
        name: format!(
            "module:programs:value:{}",
            layerx_programs::hex::encode(&account)
        )
        .into_bytes(),
        kind: 13,
        balance,
        asset_id: asset,
        has_asset: true.into(),
        next_sequence: 0,
        created_at_sequence: 2,
        frozen: false.into(),
        has_open_reference: false.into(),
        authority_key: [0; 32],
        has_authority_key: false.into(),
    };
    let value = leaf.primary_value().expect("canonical program account");
    let account_root = state_leaf_commitment(&leaf.primary_key(), &value).expect("account root");
    let universal_root = account_tree_commitment(account_root);
    let root = universal_root_commitment(universal_root);
    let receipt = account_snapshot_receipt(account, asset, root);
    let (receipt_proof, receipt_root) = build_proof(&[&receipt], 0).expect("receipt proof");
    let mut header = Encoder::new(512);
    header
        .structure_header_version(0x1701, 3)
        .expect("header version");
    header.u8(15).expect("header fields");
    header.tag(1, 15).expect("protocol tag");
    header.u16(3).expect("protocol");
    header.tag(2, 15).expect("network tag");
    header.u32(125).expect("network");
    for (tag, number) in [(3, 1), (4, checkpoint.height), (5, 8), (6, 8)] {
        header.tag(tag, 15).expect("coordinate tag");
        header.u64(number).expect("coordinate");
    }
    for (tag, hash) in [
        (7, [5; 32]),
        (8, root),
        (9, [8; 32]),
        (10, receipt_root),
        (11, [1; 32]),
        (12, [2; 32]),
        (13, [3; 32]),
    ] {
        header.tag(tag, 15).expect("root tag");
        header.bytes(&hash, 32).expect("root");
    }
    header.tag(14, 15).expect("timestamp tag");
    header.u64(1_700_000_001_000).expect("timestamp");
    header.tag(15, 15).expect("sequencer tag");
    header.bytes(&[1; 32], 32).expect("sequencer");
    let header_bytes = header.finish();
    let signer = SigningKey::from_bytes(&[8; 32]);
    let header_signature = signer
        .sign(&layerx_wire::hash::batch_header_digest(&header_bytes).expect("header digest"))
        .to_bytes();
    let single = Proof::new(0, 1, vec![]).expect("single-leaf proof");
    let proof = NestedAccountProof {
        account_id: account,
        account_root,
        universal_root,
        resulting_state_root: root,
        account_proof: single.clone(),
        account_tree_proof: single.clone(),
        universal_root_proof: single,
        receipt_bytes: receipt,
        receipt_proof,
        header_bytes,
        header_signature,
    };
    let authorization =
        SequencerAuthorization::new([1; 32], signer.verifying_key().to_bytes(), 1, 100);
    let verified = verify_nested_account(&value, account, Some(asset), &proof, &authorization)
        .expect("signed nested account proof");
    assert_eq!(verified.account().balance(), balance);
    assert!(!verified.account().frozen);
    checkpoint.header.app_hash = root;
    checkpoint.app_hash = root;
    checkpoint.block_hash = checkpoint.header.block_hash();
    checkpoint.commit.block_id.hash = checkpoint.block_hash;
    for index in 0..checkpoint.commit.votes.len() {
        let message = checkpoint.commit.vote_sign_bytes(
            &checkpoint.header.chain_id,
            checkpoint.commit.votes[index].timestamp,
        );
        checkpoint.commit.votes[index].signature = SigningKey::from_bytes(&[index as u8 + 1; 32])
            .sign(&message)
            .to_bytes();
    }
    verified
}

fn number(value: u128) -> [u8; 32] {
    let mut word = [0; 32];
    word[16..].copy_from_slice(&value.to_be_bytes());
    word
}

fn account_snapshot_receipt(account: [u8; 32], asset: [u8; 32], state: [u8; 32]) -> Vec<u8> {
    // Account-state proofs retain a kernel account-open receipt under the
    // signed snapshot header; transaction execution uses module 12 separately.
    let key = SigningKey::from_bytes(&[8; 32]);
    let encode = |signature: Option<[u8; 64]>| {
        let mut e = Encoder::new(4096);
        e.structure_header_version(0x5201, 3)
            .expect("account receipt encoding");
        e.u16(3).expect("account receipt encoding");
        e.bytes(&Sha256::digest(account), 32)
            .expect("account receipt encoding");
        e.u64(8).expect("account receipt encoding");
        for root in [[5; 32], state, [8; 32]] {
            e.bytes(&root, 32).expect("account receipt encoding");
        }
        e.i32(0).expect("account receipt encoding");
        e.sequence_length(1, 512).expect("account receipt encoding");
        e.u16(1).expect("account receipt encoding");
        e.u16(0).expect("account receipt encoding");
        e.u16(4).expect("account receipt encoding");
        e.u8(3).expect("account receipt encoding");
        e.u8(0).expect("account receipt encoding");
        e.bytes(&[0; 32], 32).expect("account receipt encoding");
        e.bytes(&account, 256).expect("account receipt encoding");
        e.u128(0).expect("account receipt encoding");
        e.bytes(&number(8), 32).expect("account receipt encoding");
        e.u16(1).expect("account receipt encoding");
        e.u32(1).expect("account receipt encoding");
        e.u32(0).expect("account receipt encoding");
        e.u8(4).expect("account receipt encoding");
        e.bytes(&asset, 32).expect("account receipt encoding");
        e.u128(0).expect("account receipt encoding");
        e.bytes(&[0; 32], 32).expect("account receipt encoding");
        e.u128(0).expect("account receipt encoding");
        e.u128(0).expect("account receipt encoding");
        e.u64(0).expect("account receipt encoding");
        e.bytes(&account, 32).expect("account receipt encoding");
        e.u128(0).expect("account receipt encoding");
        e.u128(0).expect("account receipt encoding");
        e.bytes(&[0; 32], 32).expect("account receipt encoding");
        e.bytes(&[0; 32], 32).expect("account receipt encoding");
        e.bytes(&[1; 32], 32).expect("account receipt encoding");
        e.u64(1_700_000_001_000).expect("account receipt encoding");
        e.u8(u8::from(signature.is_some()))
            .expect("account receipt encoding");
        if let Some(signature) = signature {
            e.bytes(&signature, 64).expect("account receipt encoding");
        }
        e.finish()
    };
    let signature = key
        .sign(&layerx_wire::hash::receipt_digest(&encode(None)).expect("account receipt encoding"))
        .to_bytes();
    let encoded = encode(Some(signature));
    let decoded =
        layerx_wire::receipt::decode(&encoded).expect("canonical account snapshot receipt");
    assert_eq!(
        layerx_wire::receipt::encode(&decoded).expect("account receipt encoding"),
        encoded
    );
    layerx_proof::receipt::verify_sequencer_signature(&encoded, key.verifying_key().to_bytes())
        .expect("snapshot proof receipt signature");
    encoded
}
