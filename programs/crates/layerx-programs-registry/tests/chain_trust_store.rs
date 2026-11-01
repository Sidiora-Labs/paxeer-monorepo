use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_proof::{
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    trust_store::TrustRoot,
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp},
    ChainProofError, ChainProofVerifier, Commit,
};

fn fixture() -> Result<(ChainProofVerifier, ChainHeader, Vec<SigningKey>), ChainProofError> {
    let keys: Vec<_> = (1..=3)
        .map(|seed| SigningKey::from_bytes(&[seed; 32]))
        .collect();
    let mut validators: Vec<_> = keys
        .iter()
        .map(|key| TrustedValidator::new(key.verifying_key().to_bytes(), 10))
        .collect();
    validators.sort_unstable_by_key(|validator| validator.address);
    let validators = ValidatorSet::new(validators)?;
    let header = ChainHeader {
        version: HeaderVersion { block: 11, app: 1 },
        chain_id: "paxeer-trust-store".to_owned(),
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
        next_validators_hash: validators.hash().to_vec(),
        consensus_hash: vec![6; 32],
        app_hash: vec![7; 32],
        last_results_hash: vec![8; 32],
        evidence_hash: vec![9; 32],
        proposer_address: validators.validators()[0].address.to_vec(),
    };
    Ok((
        ChainProofVerifier {
            chain_id: header.chain_id.clone(),
            validators,
        },
        header,
        keys,
    ))
}

fn signed_commit(header: &ChainHeader, keys: &[SigningKey]) -> Result<Commit, ChainProofError> {
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
    commit.signatures = keys
        .iter()
        .map(|key| CommitSig {
            flag: BlockIdFlag::Commit,
            validator_address: TrustedValidator::new(key.verifying_key().to_bytes(), 0)
                .address
                .to_vec(),
            timestamp,
            signature: key.sign(&bytes).to_bytes().to_vec(),
        })
        .collect();
    Ok(commit)
}

#[test]
fn chain_trust_root_round_trips() -> Result<(), ChainProofError> {
    let (verifier, mut header, keys) = fixture()?;
    let trusted = verifier.verify_head(&header, &signed_commit(&header, &keys)?)?;
    let root = TrustRoot::from_head(&trusted, verifier.validators.clone())?;
    assert_eq!(root.chain_id, trusted.chain_id());
    assert_eq!(root.height, 37);
    assert_eq!(root.time_ms, 1_700_000_000_123);
    assert_eq!(root.header_hash, header.hash()?);
    assert_eq!(root.validators.validators().len(), 3);
    let bytes = root.canonical_bytes()?;
    assert_eq!(&bytes[..5], b"LXTR\x01");
    let decoded = TrustRoot::decode(&bytes)?;
    assert_eq!(decoded, root);
    assert_eq!(decoded.canonical_bytes()?, bytes);

    let mut reordered = root.clone();
    let mut validators = root.validators.validators().to_vec();
    validators.reverse();
    reordered.validators = ValidatorSet::new(validators)?;
    assert_eq!(reordered.canonical_bytes()?, bytes);

    let rebuilt = ChainProofVerifier {
        chain_id: decoded.chain_id,
        validators: decoded.validators,
    };
    header.last_block_id.hash = root.header_hash.to_vec();
    header.height += 1;
    header.time.seconds += 1;
    let commit = signed_commit(&header, &keys)?;
    let next = rebuilt.verify_head(&header, &commit)?;
    assert_eq!(next.height(), trusted.height() + 1);
    assert_eq!(next.time_ms(), trusted.time_ms() + 1_000);
    assert_eq!(rebuilt.validators, verifier.validators);
    let mut invalid = commit;
    invalid.signatures[0].signature[0] ^= 1;
    assert_eq!(
        rebuilt.verify_head(&header, &invalid),
        Err(ChainProofError::Mismatch)
    );
    Ok(())
}

#[test]
fn chain_trust_root_reload_preserves_unequal_power_validator_hash() -> Result<(), ChainProofError> {
    let (mut verifier, mut header, keys) = fixture()?;
    let mut validators = verifier.validators.validators().to_vec();
    validators.reverse();
    for (validator, power) in validators.iter_mut().zip([30, 20, 10]) {
        validator.voting_power = power;
    }
    let mut address_sorted = validators.clone();
    address_sorted.sort_unstable_by_key(|validator| validator.address);
    assert_ne!(validators, address_sorted);
    verifier.validators = ValidatorSet::new(validators.clone())?;
    let validators_hash = verifier.validators.hash();
    header.validators_hash = validators_hash.to_vec();
    header.next_validators_hash = validators_hash.to_vec();
    header.proposer_address = validators[0].address.to_vec();

    let commit = signed_commit(&header, &keys)?;
    let trusted = verifier.verify_head(&header, &commit)?;
    let root = TrustRoot::from_head(&trusted, verifier.validators.clone())?;
    let bytes = root.canonical_bytes()?;
    let mut address_sorted_root = root.clone();
    address_sorted_root.validators = ValidatorSet::new(address_sorted)?;
    assert_eq!(address_sorted_root.canonical_bytes()?, bytes);
    let decoded = TrustRoot::decode(&bytes)?;
    assert_eq!(decoded.validators.validators(), validators.as_slice());
    assert_eq!(decoded.validators.hash(), validators_hash);
    assert_eq!(decoded, root);
    assert_eq!(decoded.canonical_bytes()?, bytes);

    let rebuilt = ChainProofVerifier {
        chain_id: decoded.chain_id,
        validators: decoded.validators,
    };
    assert_eq!(rebuilt.verify_head(&header, &commit)?, trusted);
    header.last_block_id.hash = root.header_hash.to_vec();
    header.height += 1;
    header.time.seconds += 1;
    let next_commit = signed_commit(&header, &keys)?;
    let next = rebuilt.verify_head(&header, &next_commit)?;
    assert_eq!(next, verifier.verify_head(&header, &next_commit)?);
    assert_eq!(next.height(), trusted.height() + 1);
    assert_eq!(next.time_ms(), trusted.time_ms() + 1_000);
    Ok(())
}

#[test]
fn chain_trust_root_refuses_unsorted_and_trailing_bytes() -> Result<(), ChainProofError> {
    let (verifier, header, keys) = fixture()?;
    let head = verifier.verify_head(&header, &signed_commit(&header, &keys)?)?;
    let root = TrustRoot::from_head(&head, verifier.validators)?;
    let canonical = root.canonical_bytes()?;
    let start = 5 + 4 + root.chain_id.len() + 4 + 8 + 4 + 8 + 4 + 32 + 4;
    let width = 4 + 20 + 4 + 32 + 4 + 8;
    assert_eq!(canonical.len(), start + 3 * width);
    let mut unsorted = canonical.clone();
    for index in 0..width {
        unsorted.swap(start + index, start + width + index);
    }
    assert_eq!(
        TrustRoot::decode(&unsorted),
        Err(ChainProofError::NonCanonical)
    );
    let mut duplicate = canonical.clone();
    duplicate.copy_within(start..start + width, start + width);
    assert_eq!(
        TrustRoot::decode(&duplicate),
        Err(ChainProofError::NonCanonical)
    );
    let mut trailing = canonical.clone();
    trailing.push(0);
    assert_eq!(
        TrustRoot::decode(&trailing),
        Err(ChainProofError::NonCanonical)
    );
    let mut version = canonical.clone();
    version[4] = 2;
    assert_eq!(
        TrustRoot::decode(&version),
        Err(ChainProofError::NonCanonical)
    );
    for length in 0..canonical.len() {
        assert!(TrustRoot::decode(&canonical[..length]).is_err());
    }
    let mut invalid_address = canonical.clone();
    invalid_address[start + 4] ^= 1;
    assert_eq!(
        TrustRoot::decode(&invalid_address),
        Err(ChainProofError::NonCanonical)
    );
    let mut oversized_field = canonical.clone();
    oversized_field[5..9].copy_from_slice(&u32::MAX.to_be_bytes());
    assert_eq!(
        TrustRoot::decode(&oversized_field),
        Err(ChainProofError::NonCanonical)
    );
    Ok(())
}
