use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::read::{
    checkpoint, validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CheckpointReadError,
    CommitTimestamp, CommitValidator, CommitVote,
};
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};

fn committed_block(
    powers: &[u64],
    signers: &[usize],
) -> (CheckpointHeader, BlockCommit, Vec<CommitValidator>) {
    let keys: Vec<_> = (1_u8..)
        .take(powers.len())
        .map(|seed| SigningKey::from_bytes(&[seed; 32]))
        .collect();
    let validators: Vec<_> = keys
        .iter()
        .zip(powers)
        .map(|(key, power)| {
            let public_key = key.verifying_key().to_bytes();
            CommitValidator {
                address: CommitValidator::address_for_key(&public_key),
                public_key,
                voting_power: *power,
            }
        })
        .collect();
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 1,
        chain_id: "paxeer-test".to_owned(),
        height: 8,
        time: CommitTimestamp {
            seconds: 1_700_000_000,
            nanos: 123,
        },
        last_block_id: Some(BlockId {
            hash: [1; 32],
            part_set_total: 1,
            part_set_hash: [2; 32],
        }),
        last_commit_hash: Some([3; 32]),
        data_hash: Some([4; 32]),
        validators_hash: validator_set_hash(&validators),
        next_validators_hash: validator_set_hash(&validators),
        consensus_hash: [5; 32],
        app_hash: [6; 32],
        last_results_hash: Some([7; 32]),
        evidence_hash: Some([8; 32]),
        proposer_address: validators[0].address,
    };
    let mut commit = BlockCommit {
        height: header.height,
        round: 1,
        block_id: BlockId {
            hash: header.block_hash(),
            part_set_total: 1,
            part_set_hash: [9; 32],
        },
        votes: Vec::new(),
    };
    for index in signers {
        let timestamp = CommitTimestamp {
            seconds: 1_700_000_001,
            nanos: *index as u32,
        };
        let signature = keys[*index]
            .sign(&commit.vote_sign_bytes(&header.chain_id, timestamp))
            .to_bytes();
        commit.votes.push(CommitVote {
            validator_address: validators[*index].address,
            voting_power: validators[*index].voting_power,
            timestamp,
            signature,
        });
    }
    (header, commit, validators)
}

#[test]
fn checkpoint_is_the_committed_block_at_the_height() {
    // Match consensus/types/block_test.go::TestHeaderHash independently of commit signing.
    let hash = |bytes: &[u8]| -> [u8; 32] { Sha256::digest(bytes).into() };
    let mut proposer_address = [0; 20];
    proposer_address.copy_from_slice(&hash(b"proposer_address")[..20]);
    let header_vector = CheckpointHeader {
        version_block: 1,
        version_app: 2,
        chain_id: "chainId".to_owned(),
        height: 3,
        time: CommitTimestamp {
            seconds: 1_570_983_284,
            nanos: 0,
        },
        last_block_id: Some(BlockId {
            hash: [0; 32],
            part_set_total: 6,
            part_set_hash: [0; 32],
        }),
        last_commit_hash: Some(hash(b"last_commit_hash")),
        data_hash: Some(hash(b"data_hash")),
        validators_hash: hash(b"validators_hash"),
        next_validators_hash: hash(b"next_validators_hash"),
        consensus_hash: hash(b"consensus_hash"),
        app_hash: hash(b"app_hash"),
        last_results_hash: Some(hash(b"last_results_hash")),
        evidence_hash: Some(hash(b"evidence_hash")),
        proposer_address,
    };
    assert_eq!(
        hex::encode(header_vector.block_hash()),
        "f740121f553b5418c3efbd343c2dbfe9e007bb67b0d020a0741374bab65242a4"
    );

    for (powers, signers) in [
        (vec![4, 3, 2], vec![0, 1]),
        (vec![7, 1, 1], vec![0]),
        (vec![1, 1, 1], vec![0, 1, 2]),
    ] {
        let (header, commit, validators) = committed_block(&powers, &signers);
        let served = checkpoint(8, "paxeer-test", &header, &commit, &validators)
            .unwrap_or_else(|error| panic!("valid block commit: {error:?}"));
        assert_eq!(served.height, 8);
        assert_eq!(served.block_hash, header.block_hash());
        assert_eq!(served.app_hash, [6; 32]);
        assert_eq!(served.commit, commit);
        assert_eq!(served.rung, Rung::Committed);
        let signed: u64 = served
            .commit
            .votes
            .iter()
            .map(|vote| vote.voting_power)
            .sum();
        let total: u64 = validators
            .iter()
            .map(|validator| validator.voting_power)
            .sum();
        assert!(signed * 3 > total * 2);

        assert_eq!(
            checkpoint(9, "paxeer-test", &header, &commit, &validators),
            Err(CheckpointReadError::Commit)
        );
        assert_eq!(
            checkpoint(8, "another-chain", &header, &commit, &validators),
            Err(CheckpointReadError::Commit)
        );
        let mut tampered_header = header.clone();
        tampered_header.app_hash[0] ^= 1;
        assert_eq!(
            checkpoint(8, "paxeer-test", &tampered_header, &commit, &validators),
            Err(CheckpointReadError::Commit)
        );
        let mut tampered_commit = commit.clone();
        tampered_commit.votes[0].signature[0] ^= 1;
        assert_eq!(
            checkpoint(8, "paxeer-test", &header, &tampered_commit, &validators),
            Err(CheckpointReadError::Commit)
        );
        let mut tampered_commit = commit.clone();
        tampered_commit.round += 1;
        assert_eq!(
            checkpoint(8, "paxeer-test", &header, &tampered_commit, &validators),
            Err(CheckpointReadError::Commit)
        );
        let mut tampered_commit = commit.clone();
        tampered_commit.block_id.part_set_hash[0] ^= 1;
        assert_eq!(
            checkpoint(8, "paxeer-test", &header, &tampered_commit, &validators),
            Err(CheckpointReadError::Commit)
        );
        let mut tampered_commit = commit.clone();
        tampered_commit.votes[0].timestamp.nanos += 1;
        assert_eq!(
            checkpoint(8, "paxeer-test", &header, &tampered_commit, &validators),
            Err(CheckpointReadError::Commit)
        );
    }

    // CanonicalVote is length-delimited protobuf, with sfixed64 height/round.
    let vector = BlockCommit {
        height: 1,
        round: 1,
        block_id: BlockId {
            hash: [0x11; 32],
            part_set_total: 1,
            part_set_hash: [0x22; 32],
        },
        votes: Vec::new(),
    };
    let expected = hex::decode(concat!(
        "68080211010000000000000019010000000000000022480a20",
        "1111111111111111111111111111111111111111111111111111111111111111",
        "122408011220",
        "2222222222222222222222222222222222222222222222222222222222222222",
        "2a020801320470617878"
    ))
    .unwrap_or_else(|error| panic!("canonical vector: {error}"));
    assert_eq!(
        vector.vote_sign_bytes(
            "paxx",
            CommitTimestamp {
                seconds: 1,
                nanos: 0
            }
        ),
        expected
    );
}

#[test]
fn checkpoint_with_insufficient_commit_power_is_refused() {
    for (powers, signers) in [
        (vec![4, 3, 2], vec![0, 2]),
        (vec![4, 3, 2], vec![0]),
        (vec![1, 1, 1], vec![0, 1]),
        (vec![1, 1, 1], vec![0]),
        (vec![1, 1, 1], vec![]),
        (vec![7, 1, 1], vec![1, 2]),
    ] {
        let (header, commit, validators) = committed_block(&powers, &signers);
        let signed: u64 = commit.votes.iter().map(|vote| vote.voting_power).sum();
        let total: u64 = validators
            .iter()
            .map(|validator| validator.voting_power)
            .sum();
        assert!(signed * 3 <= total * 2);
        assert_eq!(
            checkpoint(8, "paxeer-test", &header, &commit, &validators),
            Err(CheckpointReadError::Commit)
        );
    }
    let (header, mut commit, validators) = committed_block(&[4, 3, 2], &[0]);
    commit.votes.push(commit.votes[0]);
    assert_eq!(
        checkpoint(8, "paxeer-test", &header, &commit, &validators),
        Err(CheckpointReadError::Commit)
    );
    commit.votes.pop();
    commit.votes[0].voting_power = 9;
    assert_eq!(
        checkpoint(8, "paxeer-test", &header, &commit, &validators),
        Err(CheckpointReadError::Commit)
    );
    let (header, mut commit, validators) = committed_block(&[4, 3, 2], &[0, 1, 2]);
    commit.votes[0].validator_address = [0xff; 20];
    assert_eq!(
        checkpoint(8, "paxeer-test", &header, &commit, &validators),
        Err(CheckpointReadError::Commit)
    );
}
