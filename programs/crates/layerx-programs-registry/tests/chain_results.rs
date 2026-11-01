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
#[path = "../src/chain_proof/results.rs"]
mod results;

use merkle::{hash_from_byte_slices, SimpleProof};
use results::{verify_tx_result, TxResult};

fn hash_parts(parts: &[&[u8]]) -> [u8; 32] {
    let mut hash = Sha256::new();
    for part in parts {
        hash.update(part);
    }
    hash.finalize().into()
}

fn verified_next_head(results_hash: [u8; 32]) -> Result<VerifiedChainHead, ChainProofError> {
    let key = SigningKey::from_bytes(&[1; 32]);
    let validator = TrustedValidator::new(key.verifying_key().to_bytes(), 1);
    let address = validator.address;
    let validators = ValidatorSet::new(vec![validator])?;
    let header = ChainHeader {
        version: HeaderVersion { block: 11, app: 1 },
        chain_id: "paxeer-results-test".to_owned(),
        height: 38,
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
        data_hash: vec![4; 32],
        validators_hash: validators.hash().to_vec(),
        next_validators_hash: validators.hash().to_vec(),
        consensus_hash: vec![6; 32],
        app_hash: vec![7; 32],
        last_results_hash: results_hash.to_vec(),
        evidence_hash: vec![9; 32],
        proposer_address: address.to_vec(),
    };
    let mut commit = Commit {
        height: header.height,
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
        seconds: header.time.seconds,
        nanos: header.time.nanos,
    };
    let bytes = vote_sign_bytes(
        &header.chain_id,
        commit.height,
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

fn fixture() -> Result<(VerifiedChainHead, [TxResult; 2], [SimpleProof; 2]), ChainProofError> {
    let results = [
        TxResult::default(),
        TxResult {
            code: 5,
            data: vec![0, 0xff, 0x80],
            gas_wanted: 300,
            gas_used: 150,
        },
    ];
    let encoded: [&[u8]; 2] = [
        b"",
        &[
            0x08, 5, 0x12, 3, 0, 0xff, 0x80, 0x28, 0xac, 2, 0x30, 0x96, 1,
        ],
    ];
    let leaves = encoded.map(|leaf| hash_parts(&[&[0], leaf]));
    let root = hash_from_byte_slices(&encoded);
    assert_eq!(root, hash_parts(&[&[1], &leaves[0], &leaves[1]]));
    let proofs = [
        SimpleProof {
            total: 2,
            index: 0,
            leaf_hash: leaves[0],
            aunts: vec![leaves[1]],
        },
        SimpleProof {
            total: 2,
            index: 1,
            leaf_hash: leaves[1],
            aunts: vec![leaves[0]],
        },
    ];
    Ok((verified_next_head(root)?, results, proofs))
}

#[test]
fn chain_tx_result_verifies_against_next_results_hash() -> Result<(), ChainProofError> {
    let (next, results, proofs) = fixture()?;
    let result_height = 37;
    assert_eq!(next.height(), result_height + 1);
    assert_ne!(next.data_hash(), next.last_results_hash());
    for (result, proof) in results.iter().zip(&proofs) {
        assert_eq!(verify_tx_result(&next, result, proof), Ok(()));
    }

    let extremes = TxResult {
        code: u32::MAX,
        data: vec![0xab; 128],
        gas_wanted: -1,
        gas_used: i64::MIN,
    };
    let mut encoded = vec![0x08, 0xff, 0xff, 0xff, 0xff, 0x0f, 0x12, 0x80, 1];
    encoded.extend_from_slice(&[0xab; 128]);
    encoded.extend_from_slice(&[
        0x28, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 1, 0x30, 0x80, 0x80, 0x80,
        0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 1,
    ]);
    let root = hash_parts(&[&[0], &encoded]);
    let proof = SimpleProof {
        total: 1,
        index: 0,
        leaf_hash: root,
        aunts: Vec::new(),
    };
    assert_eq!(
        verify_tx_result(&verified_next_head(root)?, &extremes, &proof),
        Ok(())
    );
    Ok(())
}

#[test]
fn chain_tx_result_refuses_changed_code() -> Result<(), ChainProofError> {
    let (next, results, proofs) = fixture()?;
    let result = &results[1];
    let proof = &proofs[1];
    assert_eq!(verify_tx_result(&next, result, proof), Ok(()));

    let mut changed_code = result.clone();
    changed_code.code = 0;
    let mut changed_gas_used = result.clone();
    changed_gas_used.gas_used += 1;
    let mut changed_gas_wanted = result.clone();
    changed_gas_wanted.gas_wanted += 1;
    let mut changed_data = result.clone();
    changed_data.data[0] ^= 1;
    for changed in [
        changed_code,
        changed_gas_used,
        changed_gas_wanted,
        changed_data,
    ] {
        assert_eq!(
            verify_tx_result(&next, &changed, proof),
            Err(ChainProofError::Mismatch)
        );
    }

    for index in [-1, proof.total, proof.total + 1, i64::MAX] {
        let mut invalid = proof.clone();
        invalid.index = index;
        assert_eq!(
            verify_tx_result(&next, result, &invalid),
            Err(ChainProofError::Mismatch)
        );
    }
    let mut invalid = proof.clone();
    invalid.total = 0;
    assert_eq!(
        verify_tx_result(&next, result, &invalid),
        Err(ChainProofError::Mismatch)
    );

    let other = verified_next_head(next.data_hash())?;
    assert_eq!(
        verify_tx_result(&other, result, proof),
        Err(ChainProofError::Mismatch)
    );
    Ok(())
}
