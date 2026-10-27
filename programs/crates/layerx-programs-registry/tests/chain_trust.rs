use ed25519_dalek::{Signer, SigningKey};

#[path = "../src"]
mod source {
    pub mod chain_proof;
}
#[path = "../src/chain_proof/trust.rs"]
mod trust;

use chain_proof::{
    ed25519, header, validators, vote, ChainProofError, ChainProofVerifier, Commit,
    VerifiedChainHead,
};
use header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion};
use source::chain_proof;
use validators::{TrustedValidator, ValidatorSet};
use vote::{vote_sign_bytes, BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp};

fn keys(seeds: &[u8]) -> Vec<SigningKey> {
    seeds
        .iter()
        .map(|seed| SigningKey::from_bytes(&[*seed; 32]))
        .collect()
}

fn validator_set(keys: &[SigningKey], powers: &[u64]) -> Result<ValidatorSet, ChainProofError> {
    assert_eq!(keys.len(), powers.len());
    ValidatorSet::new(
        keys.iter()
            .zip(powers)
            .map(|(key, power)| TrustedValidator::new(key.verifying_key().to_bytes(), *power))
            .collect(),
    )
}

fn header(validators: &ValidatorSet, next: &ValidatorSet) -> ChainHeader {
    ChainHeader {
        version: HeaderVersion { block: 11, app: 1 },
        chain_id: "paxeer-trust-test".to_owned(),
        height: 37,
        time: HeaderTime {
            seconds: 1_700_000_000,
            nanos: 123_000_000,
        },
        last_block_id: HeaderBlockId {
            hash: vec![1; 32],
            part_set_total: 1,
            part_set_hash: vec![2; 32],
        },
        last_commit_hash: vec![3; 32],
        data_hash: vec![4; 32],
        validators_hash: validators.hash().to_vec(),
        next_validators_hash: next.hash().to_vec(),
        consensus_hash: vec![6; 32],
        app_hash: vec![7; 32],
        last_results_hash: vec![8; 32],
        evidence_hash: vec![9; 32],
        proposer_address: validators.validators()[0].address.to_vec(),
    }
}

fn sign_vote(
    header: &ChainHeader,
    commit: &Commit,
    key: &SigningKey,
    flag: BlockIdFlag,
) -> Result<CommitSig, ChainProofError> {
    let timestamp = Timestamp {
        seconds: header.time.seconds,
        nanos: header.time.nanos,
    };
    let bytes = vote_sign_bytes(
        &header.chain_id,
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
        .map(|key| sign_vote(header, &commit, key, BlockIdFlag::Commit))
        .collect::<Result<_, _>>()?;
    Ok(commit)
}

fn trusted_head(
    validators: ValidatorSet,
    next: &ValidatorSet,
    keys: &[SigningKey],
) -> Result<(ChainProofVerifier, ChainHeader, VerifiedChainHead), ChainProofError> {
    let header = header(&validators, next);
    let verifier = ChainProofVerifier {
        chain_id: header.chain_id.clone(),
        validators,
    };
    let trusted = verifier.verify_head(&header, &signed_commit(&header, keys)?)?;
    Ok((verifier, header, trusted))
}

fn assert_error(result: Result<ChainProofVerifier, ChainProofError>, expected: ChainProofError) {
    match result {
        Err(error) => assert_eq!(error, expected),
        Ok(_) => panic!("accepted an invalid trust transition"),
    }
}

#[test]
fn chain_trust_advances_adjacent_and_one_third_skips() -> Result<(), ChainProofError> {
    let old_keys = keys(&[1, 2, 3]);
    let next_keys = keys(&[4, 5, 6]);
    let next_set = validator_set(&next_keys, &[1, 1, 1])?;
    let (verifier, mut adjacent, trusted) =
        trusted_head(validator_set(&old_keys, &[4, 3, 2])?, &next_set, &old_keys)?;
    adjacent.height += 1;
    adjacent.time.seconds += 1;
    adjacent.validators_hash = next_set.hash().to_vec();
    let commit = signed_commit(&adjacent, &next_keys)?;
    let advanced = verifier.advance(&trusted, &adjacent, &commit, next_set.clone())?;
    assert_eq!(advanced.validators, next_set);
    assert_eq!(advanced.chain_id, verifier.chain_id);
    let next_head = advanced.verify_head(&adjacent, &commit)?;
    assert_eq!(next_head.height(), trusted.height() + 1);
    assert_eq!(next_head.time_ms(), trusted.time_ms() + 1_000);

    let mut following = adjacent.clone();
    following.height += 1;
    following.time.seconds += 1;
    let following_commit = signed_commit(&following, &next_keys)?;
    let following_verifier =
        advanced.advance(&next_head, &following, &following_commit, next_set.clone())?;
    assert_eq!(following_verifier.validators, next_set);
    assert_eq!(
        following_verifier
            .verify_head(&following, &following_commit)?
            .height(),
        trusted.height() + 2
    );

    let mut wrong_adjacent = adjacent.clone();
    wrong_adjacent.validators_hash = verifier.validators.hash().to_vec();
    assert_error(
        verifier.advance(
            &trusted,
            &wrong_adjacent,
            &signed_commit(&wrong_adjacent, &old_keys)?,
            verifier.validators.clone(),
        ),
        ChainProofError::Mismatch,
    );
    assert_error(
        verifier.advance(&trusted, &adjacent, &commit, verifier.validators.clone()),
        ChainProofError::Mismatch,
    );
    assert_error(
        verifier.advance(
            &trusted,
            &adjacent,
            &signed_commit(&adjacent, &next_keys[..2])?,
            next_set.clone(),
        ),
        ChainProofError::InsufficientPower,
    );

    for (seeds, expected) in [
        ([1, 4, 5], None),
        ([2, 4, 5], Some(ChainProofError::InsufficientPower)),
        ([3, 4, 5], Some(ChainProofError::InsufficientPower)),
        ([4, 5, 6], Some(ChainProofError::InsufficientPower)),
    ] {
        let skip_keys = keys(&seeds);
        let skip_set = validator_set(&skip_keys, &[1, 4, 4])?;
        let mut skipped = adjacent.clone();
        skipped.height += 20;
        skipped.validators_hash = skip_set.hash().to_vec();
        let skip_commit = signed_commit(&skipped, &skip_keys)?;
        match expected {
            Some(error) => assert_error(
                verifier.advance(&trusted, &skipped, &skip_commit, skip_set.clone()),
                error,
            ),
            None => {
                let advanced =
                    verifier.advance(&trusted, &skipped, &skip_commit, skip_set.clone())?;
                assert_eq!(advanced.validators, skip_set);
                assert_eq!(
                    advanced.verify_head(&skipped, &skip_commit)?.height(),
                    skipped.height
                );

                let mut forged = skip_commit.clone();
                forged.signatures[0].signature[0] ^= 1;
                assert_error(
                    verifier.advance(&trusted, &skipped, &forged, skip_set.clone()),
                    ChainProofError::Mismatch,
                );
                let mut duplicate = skip_commit.clone();
                duplicate.signatures.push(skip_commit.signatures[0].clone());
                assert_error(
                    verifier.advance(&trusted, &skipped, &duplicate, skip_set.clone()),
                    ChainProofError::Mismatch,
                );
                let mut nil = skip_commit.clone();
                nil.signatures[0] = sign_vote(&skipped, &nil, &skip_keys[0], BlockIdFlag::Nil)?;
                assert_error(
                    verifier.advance(&trusted, &skipped, &nil, skip_set.clone()),
                    ChainProofError::InsufficientPower,
                );
                let mut absent = skip_commit.clone();
                absent.signatures[0] = CommitSig {
                    flag: BlockIdFlag::Absent,
                    validator_address: Vec::new(),
                    timestamp: Timestamp::ZERO,
                    signature: Vec::new(),
                };
                assert_error(
                    verifier.advance(&trusted, &skipped, &absent, skip_set.clone()),
                    ChainProofError::InsufficientPower,
                );
                assert_error(
                    verifier.advance(
                        &trusted,
                        &skipped,
                        &signed_commit(&skipped, &skip_keys[..1])?,
                        skip_set.clone(),
                    ),
                    ChainProofError::InsufficientPower,
                );
                let mut wrong_block = skip_commit.clone();
                wrong_block.block_id.hash[0] ^= 1;
                wrong_block.signatures = skip_keys
                    .iter()
                    .map(|key| sign_vote(&skipped, &wrong_block, key, BlockIdFlag::Commit))
                    .collect::<Result<_, _>>()?;
                assert_error(
                    verifier.advance(&trusted, &skipped, &wrong_block, skip_set.clone()),
                    ChainProofError::Mismatch,
                );
            }
        }
    }

    let boundary_keys = keys(&[2, 4, 5]);
    let boundary_set = validator_set(&boundary_keys, &[1, 1, 1])?;
    let mut skipped = adjacent.clone();
    skipped.height += 10;
    skipped.validators_hash = boundary_set.hash().to_vec();
    let mut duplicate = signed_commit(&skipped, &boundary_keys)?;
    duplicate.signatures.push(duplicate.signatures[0].clone());
    assert_error(
        verifier.advance(&trusted, &skipped, &duplicate, boundary_set),
        ChainProofError::Mismatch,
    );
    assert_eq!(verifier.validators, validator_set(&old_keys, &[4, 3, 2])?);
    Ok(())
}

#[test]
fn chain_trust_refuses_backwards_and_expired_heads() -> Result<(), ChainProofError> {
    let keys = keys(&[1, 2, 3]);
    let validators = validator_set(&keys, &[1, 1, 1])?;
    let (verifier, original, trusted) = trusted_head(validators.clone(), &validators, &keys)?;
    for height in [original.height - 1, original.height] {
        let mut candidate = original.clone();
        candidate.height = height;
        candidate.time.seconds += 1;
        assert_error(
            verifier.advance(
                &trusted,
                &candidate,
                &signed_commit(&candidate, &keys)?,
                validators.clone(),
            ),
            ChainProofError::Expired,
        );
    }
    for height_delta in [1, 10] {
        for elapsed_ms in [-1_000_i64, -1, 0, 1, 1_209_600_000, 1_209_600_001] {
            let mut candidate = original.clone();
            candidate.height += height_delta;
            let time_ms = trusted.time_ms() + elapsed_ms;
            candidate.time = HeaderTime {
                seconds: time_ms.div_euclid(1_000),
                nanos: (time_ms.rem_euclid(1_000) * 1_000_000) as i32,
            };
            let commit = signed_commit(&candidate, &keys)?;
            let result = verifier.advance(&trusted, &candidate, &commit, validators.clone());
            if elapsed_ms > 0 && elapsed_ms <= 1_209_600_000 {
                let advanced = result?;
                assert_eq!(
                    advanced.verify_head(&candidate, &commit)?.time_ms(),
                    time_ms
                );
                assert_eq!(advanced.validators, validators);
            } else {
                assert_error(result, ChainProofError::Expired);
            }
        }
    }

    let mut other_chain = original.clone();
    other_chain.chain_id.push_str("-other");
    other_chain.height += 1;
    other_chain.time.seconds += 1;
    assert_error(
        verifier.advance(
            &trusted,
            &other_chain,
            &signed_commit(&other_chain, &keys)?,
            validators.clone(),
        ),
        ChainProofError::Mismatch,
    );
    let other_verifier = ChainProofVerifier {
        chain_id: other_chain.chain_id.clone(),
        validators: validators.clone(),
    };
    assert_error(
        other_verifier.advance(
            &trusted,
            &other_chain,
            &signed_commit(&other_chain, &keys)?,
            validators.clone(),
        ),
        ChainProofError::Mismatch,
    );

    for seconds in [i64::MAX, i64::MIN] {
        let mut candidate = original.clone();
        candidate.height += 1;
        candidate.time.seconds = seconds;
        assert_error(
            verifier.advance(
                &trusted,
                &candidate,
                &signed_commit(&original, &keys)?,
                validators.clone(),
            ),
            ChainProofError::Expired,
        );
    }
    Ok(())
}
