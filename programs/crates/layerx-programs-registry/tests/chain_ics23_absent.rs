use layerx_programs::chain_proof::{proto, ChainProofError};
use sha2::{Digest, Sha256};

#[path = "../src/chain_proof/ics23.rs"]
mod ics23;

#[path = "../src/chain_proof/ics23_absent.rs"]
mod ics23_absent;

use ics23::{verify_existence, ExistenceProof, InnerOp, LeafOp, ProofSpec};
use ics23_absent::{verify_non_existence, NonExistenceProof};

struct Tree {
    root: [u8; 32],
    height: u64,
    leaves: Vec<ExistenceProof>,
}

fn varint(mut value: u64, bytes: &mut Vec<u8>) {
    while value >= 128 {
        bytes.push((value as u8 & 0x7f) | 0x80);
        value >>= 7;
    }
    bytes.push(value as u8);
}

fn leaf(key: &[u8]) -> Tree {
    let value = [b"value:".as_slice(), key].concat();
    let op = LeafOp {
        hash: 1,
        prehash_key: 0,
        prehash_value: 1,
        length: 1,
        prefix: vec![0, 2, 2],
    };
    let mut preimage = op.prefix.clone();
    varint(key.len() as u64, &mut preimage);
    preimage.extend_from_slice(key);
    varint(32, &mut preimage);
    preimage.extend_from_slice(&Sha256::digest(&value));
    Tree {
        root: Sha256::digest(&preimage).into(),
        height: 0,
        leaves: vec![ExistenceProof {
            key: key.to_vec(),
            value,
            leaf: op,
            path: Vec::new(),
        }],
    }
}

fn branch(mut left: Tree, mut right: Tree) -> Tree {
    let height = left.height.max(right.height) + 1;
    let mut header = Vec::new();
    varint(height * 2, &mut header);
    varint(
        (left.leaves.len() + right.leaves.len()) as u64 * 2,
        &mut header,
    );
    varint(2, &mut header);
    let mut preimage = header.clone();
    preimage.push(32);
    preimage.extend_from_slice(&left.root);
    preimage.push(32);
    preimage.extend_from_slice(&right.root);
    let mut left_prefix = header.clone();
    left_prefix.push(32);
    let mut left_suffix = vec![32];
    left_suffix.extend_from_slice(&right.root);
    let mut right_prefix = header;
    right_prefix.push(32);
    right_prefix.extend_from_slice(&left.root);
    right_prefix.push(32);
    for proof in &mut left.leaves {
        proof.path.push(InnerOp {
            hash: 1,
            prefix: left_prefix.clone(),
            suffix: left_suffix.clone(),
        });
    }
    for proof in &mut right.leaves {
        proof.path.push(InnerOp {
            hash: 1,
            prefix: right_prefix.clone(),
            suffix: Vec::new(),
        });
    }
    left.leaves.extend(right.leaves);
    Tree {
        root: Sha256::digest(&preimage).into(),
        height,
        leaves: left.leaves,
    }
}

fn tree() -> Tree {
    branch(
        branch(leaf(b"b"), leaf(b"d")),
        branch(leaf(b"h"), branch(leaf(b"j"), leaf(b"n"))),
    )
}

fn absence(
    tree: &Tree,
    key: &[u8],
    left: Option<usize>,
    right: Option<usize>,
) -> NonExistenceProof {
    NonExistenceProof {
        key: key.to_vec(),
        left: left.map(|index| tree.leaves[index].clone()),
        right: right.map(|index| tree.leaves[index].clone()),
    }
}

fn bytes_field(number: u64, value: &[u8], bytes: &mut Vec<u8>) {
    varint(number << 3 | 2, bytes);
    varint(value.len() as u64, bytes);
    bytes.extend_from_slice(value);
}

fn encode_existence(proof: &ExistenceProof) -> Vec<u8> {
    let mut bytes = Vec::new();
    bytes_field(1, &proof.key, &mut bytes);
    bytes_field(2, &proof.value, &mut bytes);
    let mut leaf = vec![8, 1, 24, 1, 32, 1];
    bytes_field(5, &proof.leaf.prefix, &mut leaf);
    bytes_field(3, &leaf, &mut bytes);
    for op in &proof.path {
        let mut inner = vec![8, 1];
        bytes_field(2, &op.prefix, &mut inner);
        if !op.suffix.is_empty() {
            bytes_field(3, &op.suffix, &mut inner);
        }
        bytes_field(4, &inner, &mut bytes);
    }
    bytes
}

fn encode_absence(proof: &NonExistenceProof) -> Vec<u8> {
    let mut bytes = Vec::new();
    if !proof.key.is_empty() {
        bytes_field(1, &proof.key, &mut bytes);
    }
    if let Some(left) = &proof.left {
        bytes_field(2, &encode_existence(left), &mut bytes);
    }
    if let Some(right) = &proof.right {
        bytes_field(3, &encode_existence(right), &mut bytes);
    }
    bytes
}

#[test]
fn chain_ics23_absence_verifies_between_neighbours() -> Result<(), ChainProofError> {
    let spec = ProofSpec::iavl();
    let tree = tree();
    for proof in &tree.leaves {
        verify_existence(proof, &spec, &tree.root, &proof.key, &proof.value)?;
    }
    for (key, left, right) in [
        (b"".as_slice(), None, Some(0)),
        (b"a", None, Some(0)),
        (b"c", Some(0), Some(1)),
        (b"e", Some(1), Some(2)),
        (b"i", Some(2), Some(3)),
        (b"k", Some(3), Some(4)),
        (b"o", Some(4), None),
    ] {
        let proof = absence(&tree, key, left, right);
        assert_eq!(verify_non_existence(&proof, &spec, &tree.root, key), Ok(()));
        let decoded = NonExistenceProof::decode(&encode_absence(&proof))?;
        assert_eq!(
            verify_non_existence(&decoded, &spec, &tree.root, key),
            Ok(())
        );
    }

    let singleton = leaf(b"b");
    for (key, left, right) in [(b"a".as_slice(), None, Some(0)), (b"c", Some(0), None)] {
        let proof = absence(&singleton, key, left, right);
        assert_eq!(
            verify_non_existence(&proof, &spec, &singleton.root, key),
            Ok(())
        );
    }

    let binary = branch(leaf(&[0x7f, 0xff]), leaf(&[0x80, 0x01]));
    let proof = absence(&binary, &[0x80], Some(0), Some(1));
    assert_eq!(
        verify_non_existence(&proof, &spec, &binary.root, &[0x80]),
        Ok(())
    );
    Ok(())
}

#[test]
fn chain_ics23_absence_refuses_gap_and_order() -> Result<(), ChainProofError> {
    let spec = ProofSpec::iavl();
    let tree = tree();
    for (key, left, right) in [
        (b"e".as_slice(), Some(0), Some(2)),
        (b"d", Some(0), Some(2)),
        (b"j", Some(1), Some(4)),
        (b"i", Some(1), Some(3)),
        (b"e", Some(0), Some(4)),
        (b"e", Some(2), Some(1)),
        (b"a", Some(1), Some(2)),
        (b"z", Some(1), Some(2)),
        (b"d", Some(1), Some(2)),
        (b"h", Some(1), Some(2)),
        (b"e", Some(1), Some(1)),
        (b"c", Some(0), None),
        (b"o", Some(1), None),
        (b"o", Some(2), None),
        (b"o", Some(3), None),
        (b"a", None, Some(1)),
        (b"i", None, Some(3)),
        (b"e", None, None),
    ] {
        let proof = absence(&tree, key, left, right);
        assert_eq!(
            verify_non_existence(&proof, &spec, &tree.root, key),
            Err(ChainProofError::Mismatch)
        );
    }

    let valid = absence(&tree, b"e", Some(1), Some(2));
    assert_eq!(
        verify_non_existence(&valid, &spec, &tree.root, b"f"),
        Err(ChainProofError::Mismatch)
    );
    let mut wrong_root = tree.root;
    wrong_root[0] ^= 1;
    assert_eq!(
        verify_non_existence(&valid, &spec, &wrong_root, b"e"),
        Err(ChainProofError::Mismatch)
    );

    for left in [true, false] {
        let mut proof = valid.clone();
        let neighbour = if left {
            &mut proof.left
        } else {
            &mut proof.right
        };
        if let Some(neighbour) = neighbour {
            neighbour.value[0] ^= 1;
        }
        assert!(verify_non_existence(&proof, &spec, &tree.root, b"e").is_err());
    }
    let foreign = leaf(b"h");
    let mut proof = valid.clone();
    proof.right = Some(foreign.leaves[0].clone());
    assert!(verify_non_existence(&proof, &spec, &tree.root, b"e").is_err());

    for malformed in [
        vec![10, 1, b'e', 10, 1, b'e'],
        vec![18, 0, 18, 0],
        vec![26, 0, 26, 0],
        vec![8, 1],
        vec![34, 0],
        vec![10, 2, b'e'],
        vec![10, 0x81, 0, b'e'],
        vec![26, 0, 10, 1, b'e'],
        vec![18, 1, 0x80],
        vec![0; (1 << 20) + 1],
    ] {
        assert!(NonExistenceProof::decode(&malformed).is_err());
    }
    let decoded = NonExistenceProof::decode(&[])?;
    assert_eq!(
        verify_non_existence(&decoded, &spec, &tree.root, b""),
        Err(ChainProofError::Mismatch)
    );
    Ok(())
}
