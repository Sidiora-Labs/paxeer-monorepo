use layerx_programs::chain_proof::ChainProofError;
use sha2::{Digest, Sha256};

#[path = "../src/chain_proof/merkle.rs"]
mod merkle;

use merkle::{hash_from_byte_slices, SimpleProof};

fn hash_parts(parts: &[&[u8]]) -> [u8; 32] {
    let mut hash = Sha256::new();
    for part in parts {
        hash.update(part);
    }
    hash.finalize().into()
}

fn node(left: &[u8; 32], right: &[u8; 32]) -> [u8; 32] {
    hash_parts(&[&[0x01], left, right])
}

#[test]
fn chain_merkle_root_matches_split_rule() {
    let items: [&[u8]; 9] = [b"a", b"b", b"c", b"d", b"e", b"f", b"g", b"h", b"i"];
    let leaves = items.map(|item| hash_parts(&[&[0x00], item]));
    let ab = node(&leaves[0], &leaves[1]);
    let cd = node(&leaves[2], &leaves[3]);
    let ef = node(&leaves[4], &leaves[5]);
    let gh = node(&leaves[6], &leaves[7]);
    let abcd = node(&ab, &cd);
    let efgh = node(&ef, &gh);
    let abcdefgh = node(&abcd, &efgh);
    let expected = [
        hash_parts(&[b""]),
        leaves[0],
        ab,
        node(&ab, &leaves[2]),
        abcd,
        node(&abcd, &leaves[4]),
        node(&abcd, &ef),
        node(&abcd, &node(&ef, &leaves[6])),
        abcdefgh,
        node(&abcdefgh, &leaves[8]),
    ];
    for (length, root) in expected.iter().enumerate() {
        assert_eq!(
            hash_from_byte_slices(&items[..length]),
            *root,
            "{length} leaves"
        );
    }
    assert_eq!(hash_from_byte_slices(&[b""]), hash_parts(&[&[0x00]]));
    assert_ne!(hash_from_byte_slices(&[]), hash_from_byte_slices(&[b""]));
    let binary: [&[u8]; 3] = [b"\x00\xff", b"", b"\x01\x80"];
    assert_eq!(
        hash_from_byte_slices(&binary),
        node(
            &node(
                &hash_parts(&[&[0], binary[0]]),
                &hash_parts(&[&[0], binary[1]])
            ),
            &hash_parts(&[&[0], binary[2]]),
        )
    );
}

#[test]
fn chain_merkle_proof_refuses_wrong_index_and_depth() -> Result<(), ChainProofError> {
    let items: [&[u8]; 5] = [b"a", b"b", b"c", b"d", b"e"];
    let leaves = items.map(|item| hash_parts(&[&[0x00], item]));
    let ab = node(&leaves[0], &leaves[1]);
    let cd = node(&leaves[2], &leaves[3]);
    let abcd = node(&ab, &cd);
    let root = node(&abcd, &leaves[4]);
    assert_eq!(hash_from_byte_slices(&items), root);
    let paths = [
        vec![leaves[1], cd, leaves[4]],
        vec![leaves[0], cd, leaves[4]],
        vec![leaves[3], ab, leaves[4]],
        vec![leaves[2], ab, leaves[4]],
        vec![abcd],
    ];
    for (index, aunts) in paths.into_iter().enumerate() {
        let proof = SimpleProof {
            total: 5,
            index: i64::try_from(index).map_err(|_| ChainProofError::Mismatch)?,
            leaf_hash: leaves[index],
            aunts,
        };
        proof.verify(&root, items[index])?;

        for wrong_index in [-1, 5, 6, i64::MAX] {
            let mut invalid = proof.clone();
            invalid.index = wrong_index;
            assert_eq!(
                invalid.verify(&root, items[index]),
                Err(ChainProofError::Mismatch)
            );
        }
        for wrong_total in [i64::MIN, -1, 0] {
            let mut invalid = proof.clone();
            invalid.total = wrong_total;
            assert_eq!(
                invalid.verify(&root, items[index]),
                Err(ChainProofError::Mismatch)
            );
        }
        for removed in 0..proof.aunts.len() {
            let mut invalid = proof.clone();
            invalid.aunts.remove(removed);
            assert_eq!(
                invalid.verify(&root, items[index]),
                Err(ChainProofError::Mismatch)
            );
        }
        let mut invalid = proof.clone();
        invalid.aunts.push(root);
        assert_eq!(
            invalid.verify(&root, items[index]),
            Err(ChainProofError::Mismatch)
        );

        let mut invalid = proof.clone();
        invalid.index = (proof.index + 1) % proof.total;
        assert_eq!(
            invalid.verify(&root, items[index]),
            Err(ChainProofError::Mismatch)
        );
        let mut invalid = proof.clone();
        invalid.leaf_hash[0] ^= 1;
        assert_eq!(
            invalid.verify(&root, items[index]),
            Err(ChainProofError::Mismatch)
        );
        let mut invalid = proof.clone();
        invalid.aunts[0][0] ^= 1;
        assert_eq!(
            invalid.verify(&root, items[index]),
            Err(ChainProofError::Mismatch)
        );
        let mut wrong_root = root;
        wrong_root[0] ^= 1;
        assert_eq!(
            proof.verify(&wrong_root, items[index]),
            Err(ChainProofError::Mismatch)
        );
        assert_eq!(
            proof.verify(&root, b"wrong leaf"),
            Err(ChainProofError::Mismatch)
        );
    }

    let mut singleton = SimpleProof {
        total: 1,
        index: 0,
        leaf_hash: leaves[0],
        aunts: vec![],
    };
    singleton.verify(&leaves[0], items[0])?;
    singleton.aunts.push(leaves[1]);
    assert_eq!(
        singleton.verify(&leaves[0], items[0]),
        Err(ChainProofError::Mismatch)
    );
    singleton.total = 0;
    singleton.aunts.clear();
    assert_eq!(
        singleton.verify(&hash_from_byte_slices(&[]), items[0]),
        Err(ChainProofError::Mismatch)
    );

    let maximal = SimpleProof {
        total: i64::MAX,
        index: i64::MAX - 1,
        leaf_hash: leaves[0],
        aunts: vec![],
    };
    assert_eq!(
        maximal.verify(&root, items[0]),
        Err(ChainProofError::Mismatch)
    );
    Ok(())
}
