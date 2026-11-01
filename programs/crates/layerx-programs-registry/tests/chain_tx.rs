use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_proof::{
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp},
    ChainProofError, ChainProofVerifier, Commit, VerifiedChainHead,
};
use sha2::{Digest, Sha256};

#[path = "../src/chain_proof/merkle.rs"]
mod merkle;
#[path = "../src/chain_proof/tx.rs"]
mod tx;

use merkle::{hash_from_byte_slices, SimpleProof};
use tx::verify_tx;

fn verified_head(data_hash: [u8; 32], height: i64) -> Result<VerifiedChainHead, ChainProofError> {
    let key = SigningKey::from_bytes(&[17; 32]);
    let validator = TrustedValidator::new(key.verifying_key().to_bytes(), 10);
    let address = validator.address;
    let validators = ValidatorSet::new(vec![validator])?;
    let header = ChainHeader {
        version: HeaderVersion { block: 11, app: 1 },
        chain_id: "paxeer-tx-test".to_owned(),
        height,
        time: HeaderTime {
            seconds: 1_700_000_000,
            nanos: 0,
        },
        last_block_id: HeaderBlockId {
            hash: vec![1; 32],
            part_set_total: 1,
            part_set_hash: vec![2; 32],
        },
        last_commit_hash: vec![3; 32],
        data_hash: data_hash.to_vec(),
        validators_hash: validators.hash().to_vec(),
        next_validators_hash: validators.hash().to_vec(),
        consensus_hash: vec![6; 32],
        app_hash: vec![7; 32],
        last_results_hash: vec![8; 32],
        evidence_hash: vec![9; 32],
        proposer_address: address.to_vec(),
    };
    let mut commit = Commit {
        height,
        round: 0,
        block_id: BlockId {
            hash: header.hash()?.to_vec(),
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![10; 32],
            },
        },
        signatures: Vec::new(),
    };
    let timestamp = Timestamp {
        seconds: 1_700_000_001,
        nanos: 0,
    };
    let bytes = vote_sign_bytes(
        &header.chain_id,
        height,
        commit.round,
        Some(&commit.block_id),
        timestamp,
    )?;
    commit.signatures.push(CommitSig {
        flag: BlockIdFlag::Commit,
        validator_address: address.to_vec(),
        timestamp,
        signature: key.sign(&bytes).to_bytes().to_vec(),
    });
    ChainProofVerifier {
        chain_id: header.chain_id.clone(),
        validators,
    }
    .verify_head(&header, &commit)
}

fn inner_hash(left: &[u8; 32], right: &[u8; 32]) -> [u8; 32] {
    let mut hash = Sha256::new();
    hash.update([1]);
    hash.update(left);
    hash.update(right);
    hash.finalize().into()
}

fn three_tx_block(txs: [&[u8]; 3]) -> ([u8; 32], [SimpleProof; 3]) {
    let hashes: [[u8; 32]; 3] = txs.map(|tx| Sha256::digest(tx).into());
    let leaves = hashes.map(|hash| {
        let mut leaf = Sha256::new();
        leaf.update([0]);
        leaf.update(hash);
        leaf.finalize().into()
    });
    let left = inner_hash(&leaves[0], &leaves[1]);
    let root = inner_hash(&left, &leaves[2]);
    assert_eq!(
        root,
        hash_from_byte_slices(&hashes.each_ref().map(|hash| hash.as_slice()))
    );
    (
        root,
        [
            SimpleProof {
                total: 3,
                index: 0,
                leaf_hash: leaves[0],
                aunts: vec![leaves[1], leaves[2]],
            },
            SimpleProof {
                total: 3,
                index: 1,
                leaf_hash: leaves[1],
                aunts: vec![leaves[0], leaves[2]],
            },
            SimpleProof {
                total: 3,
                index: 2,
                leaf_hash: leaves[2],
                aunts: vec![left],
            },
        ],
    )
}

#[test]
fn chain_tx_inclusion_verifies_against_data_hash() -> Result<(), ChainProofError> {
    let txs: [&[u8]; 3] = [b"transfer:alice:bob:7", b"", b"\x00\xff\x80\x01"];
    let (root, proofs) = three_tx_block(txs);
    let head = verified_head(root, 37)?;
    assert_eq!(head.data_hash(), root);
    for (tx, proof) in txs.into_iter().zip(proofs) {
        let expected: [u8; 32] = Sha256::digest(tx).into();
        assert_eq!(verify_tx(&head, tx, &proof), Ok(expected));
        assert_eq!(
            verify_tx(&head, b"changed transaction", &proof),
            Err(ChainProofError::Mismatch)
        );
        for index in [i64::MIN, -1, proof.total, proof.total + 1, i64::MAX] {
            let mut invalid = proof.clone();
            invalid.index = index;
            assert_eq!(
                verify_tx(&head, tx, &invalid),
                Err(ChainProofError::Mismatch)
            );
        }
        for total in [i64::MIN, -1, 0] {
            let mut invalid = proof.clone();
            invalid.total = total;
            assert_eq!(
                verify_tx(&head, tx, &invalid),
                Err(ChainProofError::Mismatch)
            );
        }
        let mut invalid = proof.clone();
        invalid.leaf_hash[0] ^= 1;
        assert_eq!(
            verify_tx(&head, tx, &invalid),
            Err(ChainProofError::Mismatch)
        );
        let mut invalid = proof.clone();
        invalid.aunts.pop();
        assert_eq!(
            verify_tx(&head, tx, &invalid),
            Err(ChainProofError::Mismatch)
        );
        let mut invalid = proof;
        invalid.aunts.push(root);
        assert_eq!(
            verify_tx(&head, tx, &invalid),
            Err(ChainProofError::Mismatch)
        );
    }
    Ok(())
}

#[test]
fn chain_tx_inclusion_refuses_other_block() -> Result<(), ChainProofError> {
    let txs: [&[u8]; 3] = [b"tx-a", b"tx-b", b"tx-c"];
    let (root, proofs) = three_tx_block(txs);
    let (other_root, other_proofs) = three_tx_block([txs[0], b"other-tx", txs[2]]);
    let head = verified_head(root, 37)?;
    let other_head = verified_head(other_root, 38)?;
    assert_ne!(head.data_hash(), other_head.data_hash());
    assert_ne!(head.height(), other_head.height());
    for (tx, proof) in txs.into_iter().zip(proofs) {
        let expected: [u8; 32] = Sha256::digest(tx).into();
        assert_eq!(verify_tx(&head, tx, &proof), Ok(expected));
        assert_eq!(
            verify_tx(&other_head, tx, &proof),
            Err(ChainProofError::Mismatch)
        );
    }
    let expected: [u8; 32] = Sha256::digest(txs[0]).into();
    assert_eq!(
        verify_tx(&other_head, txs[0], &other_proofs[0]),
        Ok(expected)
    );
    assert_eq!(
        verify_tx(&head, txs[0], &other_proofs[0]),
        Err(ChainProofError::Mismatch)
    );
    Ok(())
}
