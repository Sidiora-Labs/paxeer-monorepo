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

trait AssertFixture<T> {
    fn assert_fixture(self, context: &str) -> T;
}

impl<T, E: std::fmt::Debug> AssertFixture<T> for Result<T, E> {
    fn assert_fixture(self, context: &str) -> T {
        match self {
            Ok(value) => value,
            Err(error) => panic!("{context}: {error:?}"),
        }
    }
}

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
    let value = leaf
        .primary_value()
        .assert_fixture("canonical program account");
    let account_root =
        state_leaf_commitment(&leaf.primary_key(), &value).assert_fixture("account root");
    let universal_root = account_tree_commitment(account_root);
    let root = universal_root_commitment(universal_root);
    let receipt = account_snapshot_receipt(account, asset, root);
    let (receipt_proof, receipt_root) = build_proof(&[&receipt], 0).assert_fixture("receipt proof");
    let mut header = Encoder::new(512);
    header
        .structure_header_version(0x1701, 3)
        .assert_fixture("header version");
    header.u8(15).assert_fixture("header fields");
    header.tag(1, 15).assert_fixture("protocol tag");
    header.u16(3).assert_fixture("protocol");
    header.tag(2, 15).assert_fixture("network tag");
    header.u32(125).assert_fixture("network");
    for (tag, number) in [(3, 1), (4, checkpoint.height), (5, 8), (6, 8)] {
        header.tag(tag, 15).assert_fixture("coordinate tag");
        header.u64(number).assert_fixture("coordinate");
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
        header.tag(tag, 15).assert_fixture("root tag");
        header.bytes(&hash, 32).assert_fixture("root");
    }
    header.tag(14, 15).assert_fixture("timestamp tag");
    header.u64(1_700_000_001_000).assert_fixture("timestamp");
    header.tag(15, 15).assert_fixture("sequencer tag");
    header.bytes(&[1; 32], 32).assert_fixture("sequencer");
    let header_bytes = header.finish();
    let signer = SigningKey::from_bytes(&[8; 32]);
    let header_signature = signer
        .sign(
            &layerx_wire::hash::batch_header_digest(&header_bytes).assert_fixture("header digest"),
        )
        .to_bytes();
    let single = Proof::new(0, 1, vec![]).assert_fixture("single-leaf proof");
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
        .assert_fixture("signed nested account proof");
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
        let signer_index = u8::try_from(index).assert_fixture("fixture vote index fits a byte") + 1;
        checkpoint.commit.votes[index].signature = SigningKey::from_bytes(&[signer_index; 32])
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
            .assert_fixture("account receipt encoding");
        e.u16(3).assert_fixture("account receipt encoding");
        e.bytes(&Sha256::digest(account), 32)
            .assert_fixture("account receipt encoding");
        e.u64(8).assert_fixture("account receipt encoding");
        for root in [[5; 32], state, [8; 32]] {
            e.bytes(&root, 32)
                .assert_fixture("account receipt encoding");
        }
        e.i32(0).assert_fixture("account receipt encoding");
        e.sequence_length(1, 512)
            .assert_fixture("account receipt encoding");
        e.u16(1).assert_fixture("account receipt encoding");
        e.u16(0).assert_fixture("account receipt encoding");
        e.u16(4).assert_fixture("account receipt encoding");
        e.u8(3).assert_fixture("account receipt encoding");
        e.u8(0).assert_fixture("account receipt encoding");
        e.bytes(&[0; 32], 32)
            .assert_fixture("account receipt encoding");
        e.bytes(&account, 256)
            .assert_fixture("account receipt encoding");
        e.u128(0).assert_fixture("account receipt encoding");
        e.bytes(&number(8), 32)
            .assert_fixture("account receipt encoding");
        e.u16(1).assert_fixture("account receipt encoding");
        e.u32(1).assert_fixture("account receipt encoding");
        e.u32(0).assert_fixture("account receipt encoding");
        e.u8(4).assert_fixture("account receipt encoding");
        e.bytes(&asset, 32)
            .assert_fixture("account receipt encoding");
        e.u128(0).assert_fixture("account receipt encoding");
        e.bytes(&[0; 32], 32)
            .assert_fixture("account receipt encoding");
        e.u128(0).assert_fixture("account receipt encoding");
        e.u128(0).assert_fixture("account receipt encoding");
        e.u64(0).assert_fixture("account receipt encoding");
        e.bytes(&account, 32)
            .assert_fixture("account receipt encoding");
        e.u128(0).assert_fixture("account receipt encoding");
        e.u128(0).assert_fixture("account receipt encoding");
        e.bytes(&[0; 32], 32)
            .assert_fixture("account receipt encoding");
        e.bytes(&[0; 32], 32)
            .assert_fixture("account receipt encoding");
        e.bytes(&[1; 32], 32)
            .assert_fixture("account receipt encoding");
        e.u64(1_700_000_001_000)
            .assert_fixture("account receipt encoding");
        e.u8(u8::from(signature.is_some()))
            .assert_fixture("account receipt encoding");
        if let Some(signature) = signature {
            e.bytes(&signature, 64)
                .assert_fixture("account receipt encoding");
        }
        e.finish()
    };
    let signature = key
        .sign(
            &layerx_wire::hash::receipt_digest(&encode(None))
                .assert_fixture("account receipt encoding"),
        )
        .to_bytes();
    let encoded = encode(Some(signature));
    let decoded =
        layerx_wire::receipt::decode(&encoded).assert_fixture("canonical account snapshot receipt");
    assert_eq!(
        layerx_wire::receipt::encode(&decoded).assert_fixture("account receipt encoding"),
        encoded
    );
    layerx_proof::receipt::verify_sequencer_signature(&encoded, key.verifying_key().to_bytes())
        .assert_fixture("snapshot proof receipt signature");
    encoded
}
