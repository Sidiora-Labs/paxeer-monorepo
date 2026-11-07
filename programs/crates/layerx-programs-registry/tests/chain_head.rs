use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_proof::{
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp},
    ChainProofError, ChainProofVerifier, Commit,
};

fn fixture(
    powers: &[u64],
) -> Result<(ChainProofVerifier, ChainHeader, Vec<SigningKey>), ChainProofError> {
    let keys: Vec<_> = powers
        .iter()
        .zip(1_u8..)
        .map(|(_, seed)| SigningKey::from_bytes(&[seed; 32]))
        .collect();
    let validators = ValidatorSet::new(
        keys.iter()
            .zip(powers)
            .map(|(key, power)| TrustedValidator::new(key.verifying_key().to_bytes(), *power))
            .collect(),
    )?;
    let header = ChainHeader {
        version: HeaderVersion { block: 11, app: 1 },
        chain_id: "paxeer-head-test".to_owned(),
        height: 37,
        time: HeaderTime {
            seconds: 1_700_000_000,
            nanos: 123_456_789,
        },
        last_block_id: HeaderBlockId {
            hash: vec![1; 32],
            part_set_total: 1,
            part_set_hash: vec![2; 32],
        },
        last_commit_hash: vec![3; 32],
        data_hash: vec![4; 32],
        validators_hash: validators.hash().to_vec(),
        next_validators_hash: vec![5; 32],
        consensus_hash: vec![6; 32],
        app_hash: vec![7; 32],
        last_results_hash: vec![8; 32],
        evidence_hash: vec![9; 32],
        proposer_address: validators.validators()[0].address.to_vec(),
    };
    let verifier = ChainProofVerifier {
        chain_id: header.chain_id.clone(),
        validators,
    };
    Ok((verifier, header, keys))
}

fn sign_vote(
    chain_id: &str,
    commit: &Commit,
    key: &SigningKey,
    flag: BlockIdFlag,
) -> Result<CommitSig, ChainProofError> {
    let timestamp = Timestamp {
        seconds: 1_700_000_001,
        nanos: 987_654_321,
    };
    let bytes = vote_sign_bytes(
        chain_id,
        commit.height,
        commit.round,
        (flag == BlockIdFlag::Commit).then_some(&commit.block_id),
        timestamp,
    )?;
    Ok(CommitSig {
        flag,
        validator_address: TrustedValidator::new(key.verifying_key().to_bytes(), 0)
            .address
            .to_vec(),
        timestamp,
        signature: key.sign(&bytes).to_bytes().to_vec(),
    })
}

fn signed_commit(header: &ChainHeader, keys: &[SigningKey]) -> Result<Commit, ChainProofError> {
    let mut commit = Commit {
        height: header.height,
        round: 2,
        block_id: BlockId {
            hash: header.hash()?.to_vec(),
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![10; 32],
            },
        },
        signatures: Vec::new(),
    };
    commit.signatures = keys
        .iter()
        .map(|key| sign_vote(&header.chain_id, &commit, key, BlockIdFlag::Commit))
        .collect::<Result<_, _>>()?;
    Ok(commit)
}

#[test]
fn chain_head_verifies_with_two_thirds_power() -> Result<(), ChainProofError> {
    let (verifier, header, keys) = fixture(&[1, 1, 1, 1])?;
    let mut commit = signed_commit(&header, &keys[..3])?;
    let head = verifier.verify_head(&header, &commit)?;
    assert_eq!(head.chain_id(), header.chain_id);
    assert_eq!(head.height(), header.height);
    assert_eq!(head.time_ms(), 1_700_000_000_123);
    assert_eq!(head.app_hash().as_slice(), header.app_hash);
    assert_eq!(head.data_hash().as_slice(), header.data_hash);
    assert_eq!(
        head.last_results_hash().as_slice(),
        header.last_results_hash
    );
    assert_eq!(
        head.next_validators_hash().as_slice(),
        header.next_validators_hash
    );

    commit.signatures.push(sign_vote(
        &header.chain_id,
        &commit,
        &keys[3],
        BlockIdFlag::Nil,
    )?);
    assert_eq!(verifier.verify_head(&header, &commit)?, head);
    commit.signatures[3] = CommitSig {
        flag: BlockIdFlag::Absent,
        validator_address: Vec::new(),
        timestamp: Timestamp::ZERO,
        signature: Vec::new(),
    };
    assert_eq!(verifier.verify_head(&header, &commit)?, head);

    let (weighted, header, keys) = fixture(&[7, 1, 1, 1])?;
    assert_eq!(
        weighted
            .verify_head(&header, &signed_commit(&header, &keys[..1])?)?
            .height(),
        header.height
    );
    Ok(())
}

#[test]
fn chain_head_refuses_forged_and_underpowered_commits() -> Result<(), ChainProofError> {
    let (verifier, header, keys) = fixture(&[1, 1, 1, 1])?;
    let valid = signed_commit(&header, &keys[..3])?;
    let underpowered = signed_commit(&header, &keys[..2])?;
    assert_eq!(
        verifier.verify_head(&header, &underpowered),
        Err(ChainProofError::InsufficientPower)
    );

    let mut unknown = valid.clone();
    unknown.signatures.push(sign_vote(
        &header.chain_id,
        &unknown,
        &SigningKey::from_bytes(&[99; 32]),
        BlockIdFlag::Commit,
    )?);
    assert_eq!(
        verifier.verify_head(&header, &unknown),
        Err(ChainProofError::Mismatch)
    );
    let mut forged = valid.clone();
    forged.signatures[0] = sign_vote(&header.chain_id, &forged, &keys[3], BlockIdFlag::Commit)?;
    forged.signatures[0].validator_address = valid.signatures[0].validator_address.clone();
    assert_eq!(
        verifier.verify_head(&header, &forged),
        Err(ChainProofError::Mismatch)
    );
    let mut duplicate = valid.clone();
    duplicate.signatures.push(valid.signatures[0].clone());
    assert_eq!(
        verifier.verify_head(&header, &duplicate),
        Err(ChainProofError::Mismatch)
    );

    let mut wrong_block = valid.clone();
    wrong_block.block_id.hash[0] ^= 1;
    wrong_block.signatures = keys[..3]
        .iter()
        .map(|key| sign_vote(&header.chain_id, &wrong_block, key, BlockIdFlag::Commit))
        .collect::<Result<_, _>>()?;
    assert_eq!(
        verifier.verify_head(&header, &wrong_block),
        Err(ChainProofError::Mismatch)
    );
    let mut wrong_height = valid.clone();
    wrong_height.height += 1;
    assert_eq!(
        verifier.verify_head(&header, &wrong_height),
        Err(ChainProofError::Mismatch)
    );
    let mut wrong_validators = header.clone();
    wrong_validators.validators_hash[0] ^= 1;
    assert_eq!(
        verifier.verify_head(&wrong_validators, &valid),
        Err(ChainProofError::Mismatch)
    );
    let mut wrong_chain = header.clone();
    wrong_chain.chain_id.push_str("-other");
    let other_chain_commit = signed_commit(&wrong_chain, &keys[..3])?;
    assert_eq!(
        verifier.verify_head(&wrong_chain, &other_chain_commit),
        Err(ChainProofError::Mismatch)
    );

    let mut corrupted = valid.clone();
    corrupted.signatures[0].signature[0] ^= 1;
    assert_eq!(
        verifier.verify_head(&header, &corrupted),
        Err(ChainProofError::Mismatch)
    );
    let mut short = valid.clone();
    short.signatures[0].signature.pop();
    assert_eq!(
        verifier.verify_head(&header, &short),
        Err(ChainProofError::Mismatch)
    );
    let mut wrong_round = valid.clone();
    wrong_round.round += 1;
    assert_eq!(
        verifier.verify_head(&header, &wrong_round),
        Err(ChainProofError::Mismatch)
    );
    let mut wrong_time = valid.clone();
    wrong_time.signatures[0].timestamp.nanos += 1;
    assert_eq!(
        verifier.verify_head(&header, &wrong_time),
        Err(ChainProofError::Mismatch)
    );
    let mut wrong_parts = valid.clone();
    wrong_parts.block_id.part_set_header.hash[0] ^= 1;
    assert_eq!(
        verifier.verify_head(&header, &wrong_parts),
        Err(ChainProofError::Mismatch)
    );

    let mut nil = underpowered.clone();
    nil.signatures.push(sign_vote(
        &header.chain_id,
        &nil,
        &keys[2],
        BlockIdFlag::Nil,
    )?);
    assert_eq!(
        verifier.verify_head(&header, &nil),
        Err(ChainProofError::InsufficientPower)
    );
    nil.signatures[2].flag = BlockIdFlag::Commit;
    assert_eq!(
        verifier.verify_head(&header, &nil),
        Err(ChainProofError::Mismatch)
    );
    let mut relabeled_nil = valid.clone();
    relabeled_nil.signatures[0].flag = BlockIdFlag::Nil;
    assert_eq!(
        verifier.verify_head(&header, &relabeled_nil),
        Err(ChainProofError::Mismatch)
    );
    let mut malformed_absent = valid.clone();
    malformed_absent.signatures[0].flag = BlockIdFlag::Absent;
    assert_eq!(
        verifier.verify_head(&header, &malformed_absent),
        Err(ChainProofError::Mismatch)
    );

    let (boundary, header, keys) = fixture(&[1, 1, 1])?;
    assert_eq!(
        boundary.verify_head(&header, &signed_commit(&header, &keys[..2])?),
        Err(ChainProofError::InsufficientPower)
    );
    let (weighted, header, keys) = fixture(&[7, 1, 1, 1])?;
    assert_eq!(
        weighted.verify_head(&header, &signed_commit(&header, &keys[1..])?),
        Err(ChainProofError::InsufficientPower)
    );
    Ok(())
}
