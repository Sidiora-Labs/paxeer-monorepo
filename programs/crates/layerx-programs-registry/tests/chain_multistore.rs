use layerx_programs::chain_proof::{proto, ChainProofError};
use sha2::{Digest, Sha256};

#[path = "../src/chain_proof/multistore.rs"]
mod multistore;

use multistore::verify_store_root;

const LEAF_OP: &[u8] = &[0x08, 0x01, 0x18, 0x01, 0x20, 0x01, 0x2a, 0x01, 0x00];

fn varint(mut value: usize) -> Vec<u8> {
    let mut bytes = Vec::new();
    while value >= 128 {
        bytes.push((value as u8 & 127) | 128);
        value >>= 7;
    }
    bytes.push(value as u8);
    bytes
}

fn bytes_field(number: u8, value: &[u8], output: &mut Vec<u8>) {
    output.push((number << 3) | 2);
    output.extend(varint(value.len()));
    output.extend_from_slice(value);
}

fn leaf_hash(name: &str, store_root: [u8; 32]) -> [u8; 32] {
    let value_hash = Sha256::digest(store_root);
    let mut bytes = vec![0];
    bytes.extend(varint(name.len()));
    bytes.extend_from_slice(name.as_bytes());
    bytes.extend(varint(value_hash.len()));
    bytes.extend_from_slice(&value_hash);
    Sha256::digest(bytes).into()
}

fn inner_hash(left: [u8; 32], right: [u8; 32]) -> [u8; 32] {
    let mut bytes = vec![1];
    bytes.extend_from_slice(&left);
    bytes.extend_from_slice(&right);
    Sha256::digest(bytes).into()
}

fn inner_op(sibling: [u8; 32], sibling_on_left: bool) -> Vec<u8> {
    let mut bytes = vec![0x08, 0x01];
    let mut prefix = vec![1];
    if sibling_on_left {
        prefix.extend_from_slice(&sibling);
    }
    bytes_field(2, &prefix, &mut bytes);
    if !sibling_on_left {
        bytes_field(3, &sibling, &mut bytes);
    }
    bytes
}

fn commitment(name: &str, root: [u8; 32], leaf: &[u8], path: &[Vec<u8>]) -> Vec<u8> {
    let mut existence = Vec::new();
    bytes_field(1, name.as_bytes(), &mut existence);
    bytes_field(2, &root, &mut existence);
    bytes_field(3, leaf, &mut existence);
    for inner in path {
        bytes_field(4, inner, &mut existence);
    }
    let mut proof = Vec::new();
    bytes_field(1, &existence, &mut proof);
    proof
}

fn stores() -> [(&'static str, [u8; 32]); 4] {
    [
        ("auth", [0x11; 32]),
        ("bank", [0x22; 32]),
        ("lxphost", [0x33; 32]),
        ("staking", [0x44; 32]),
    ]
}

fn four_store_proof(index: usize) -> (Vec<u8>, [u8; 32]) {
    let stores = stores();
    let leaves = stores.map(|(name, root)| leaf_hash(name, root));
    let branches = [
        inner_hash(leaves[0], leaves[1]),
        inner_hash(leaves[2], leaves[3]),
    ];
    let path = [
        inner_op(leaves[index ^ 1], index % 2 == 1),
        inner_op(branches[(index / 2) ^ 1], index >= 2),
    ];
    (
        commitment(stores[index].0, stores[index].1, LEAF_OP, &path),
        inner_hash(branches[0], branches[1]),
    )
}

#[test]
fn chain_multistore_binds_store_root_to_app_hash() {
    for (index, (name, root)) in stores().into_iter().enumerate() {
        let (proof, app_hash) = four_store_proof(index);
        assert_eq!(verify_store_root(name, root, &proof, app_hash), Ok(()));

        let mut wrong_root = root;
        wrong_root[17] ^= 1;
        assert_eq!(
            verify_store_root(name, wrong_root, &proof, app_hash),
            Err(ChainProofError::Mismatch)
        );

        let mut wrong_app_hash = app_hash;
        wrong_app_hash[31] ^= 1;
        assert_eq!(
            verify_store_root(name, root, &proof, wrong_app_hash),
            Err(ChainProofError::Mismatch)
        );

        let leaves = stores().map(|(name, root)| leaf_hash(name, root));
        let branches = [
            inner_hash(leaves[0], leaves[1]),
            inner_hash(leaves[2], leaves[3]),
        ];
        let mut path = [
            inner_op(leaves[index ^ 1], index % 2 == 1),
            inner_op(branches[(index / 2) ^ 1], index >= 2),
        ];
        path[0][4] = 0;
        let invalid = commitment(name, root, LEAF_OP, &path);
        assert_eq!(
            verify_store_root(name, root, &invalid, app_hash),
            Err(ChainProofError::Mismatch)
        );
    }

    let name = "store".repeat(32);
    let root = [0x55; 32];
    let proof = commitment(&name, root, LEAF_OP, &[]);
    assert_eq!(
        verify_store_root(&name, root, &proof, leaf_hash(&name, root)),
        Ok(())
    );

    for leaf in [
        &[0x08, 0x00, 0x18, 0x01, 0x20, 0x01, 0x2a, 0x01, 0x00][..],
        &[
            0x08, 0x01, 0x10, 0x01, 0x18, 0x01, 0x20, 0x01, 0x2a, 0x01, 0x00,
        ],
        &[0x08, 0x01, 0x18, 0x00, 0x20, 0x01, 0x2a, 0x01, 0x00],
        &[0x08, 0x01, 0x18, 0x01, 0x20, 0x00, 0x2a, 0x01, 0x00],
        &[0x08, 0x01, 0x18, 0x01, 0x20, 0x01, 0x2a, 0x01, 0x01],
    ] {
        let invalid = commitment(&name, root, leaf, &[]);
        assert_eq!(
            verify_store_root(&name, root, &invalid, leaf_hash(&name, root)),
            Err(ChainProofError::Mismatch)
        );
    }

    let (proof, app_hash) = four_store_proof(2);
    for length in 0..proof.len() {
        assert!(verify_store_root("lxphost", [0x33; 32], &proof[..length], app_hash).is_err());
    }
    let mut trailing = proof;
    trailing.extend_from_slice(&[0x12, 0x00]);
    assert!(verify_store_root("lxphost", [0x33; 32], &trailing, app_hash).is_err());
}

#[test]
fn chain_multistore_refuses_wrong_store_name() {
    for (index, (name, root)) in stores().into_iter().enumerate() {
        let (proof, app_hash) = four_store_proof(index);
        assert_eq!(verify_store_root(name, root, &proof, app_hash), Ok(()));
        for wrong_name in ["another-store", "", "Lxphost", "lxphost\0"] {
            assert_eq!(
                verify_store_root(wrong_name, root, &proof, app_hash),
                Err(ChainProofError::Mismatch)
            );
        }
        let other = stores()[(index + 1) % 4].0;
        assert_eq!(
            verify_store_root(other, root, &proof, app_hash),
            Err(ChainProofError::Mismatch)
        );
        let renamed = commitment(other, root, LEAF_OP, &[]);
        assert_eq!(
            verify_store_root(other, root, &renamed, app_hash),
            Err(ChainProofError::Mismatch)
        );
    }
}
