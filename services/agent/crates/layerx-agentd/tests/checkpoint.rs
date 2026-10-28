use ed25519_dalek::{Signer as _, SigningKey as EdSigningKey};
use layerx_agentd::read::{
    checkpoint, proof_bundle, validator_set_hash, BlockCommit, BlockId, CheckpointHeader,
    CheckpointReadError, CommitTimestamp, CommitValidator, CommitVote, ProofBundleKind,
    ProofBundleRequest,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::merkle::{build_proof, Proof};
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::batch_header_digest;

fn header_bytes(state_root: [u8; 32], activity_root: [u8; 32], sequencer_id: [u8; 32]) -> Vec<u8> {
    let mut encoder = Encoder::new(354);
    assert_eq!(
        encoder.structure_header_version(0x1701, layerx_wire::limits::PROTOCOL_VERSION),
        Ok(())
    );
    assert_eq!(encoder.u8(15), Ok(()));
    for field in 1..=15 {
        assert_eq!(encoder.tag(field, 15), Ok(()));
        match field {
            1 => assert_eq!(encoder.u16(layerx_wire::limits::PROTOCOL_VERSION), Ok(())),
            2 => assert_eq!(encoder.u32(42), Ok(())),
            3 => assert_eq!(encoder.u64(7), Ok(())),
            4 => assert_eq!(encoder.u64(8), Ok(())),
            5 => assert_eq!(encoder.u64(11), Ok(())),
            6 => assert_eq!(encoder.u64(19), Ok(())),
            7 => assert_eq!(encoder.bytes(&[7; 32], 32), Ok(())),
            8 => assert_eq!(encoder.bytes(&state_root, 32), Ok(())),
            9 => assert_eq!(encoder.bytes(&activity_root, 32), Ok(())),
            10 => assert_eq!(encoder.bytes(&[10; 32], 32), Ok(())),
            11 => assert_eq!(encoder.bytes(&[11; 32], 32), Ok(())),
            12 => assert_eq!(encoder.bytes(&[12; 32], 32), Ok(())),
            13 => assert_eq!(encoder.bytes(&[13; 32], 32), Ok(())),
            14 => assert_eq!(encoder.u64(1_000), Ok(())),
            15 => assert_eq!(encoder.bytes(&sequencer_id, 32), Ok(())),
            _ => panic!("unreachable header field"),
        }
    }
    encoder.finish()
}

fn committed_block() -> (CheckpointHeader, BlockCommit, Vec<CommitValidator>) {
    let key = EdSigningKey::from_bytes(&[17; 32]);
    let public_key = key.verifying_key().to_bytes();
    let validators = vec![CommitValidator {
        address: CommitValidator::address_for_key(&public_key),
        public_key,
        voting_power: 10,
    }];
    let timestamp = CommitTimestamp {
        seconds: 1_700_000_000,
        nanos: 0,
    };
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 0,
        chain_id: "paxeer-test".to_owned(),
        height: 8,
        time: timestamp,
        last_block_id: None,
        last_commit_hash: None,
        data_hash: None,
        validators_hash: validator_set_hash(&validators),
        next_validators_hash: validator_set_hash(&validators),
        consensus_hash: [1; 32],
        app_hash: [2; 32],
        last_results_hash: None,
        evidence_hash: None,
        proposer_address: validators[0].address,
    };
    let mut commit = BlockCommit {
        height: 8,
        round: 0,
        block_id: BlockId {
            hash: header.block_hash(),
            part_set_total: 1,
            part_set_hash: [3; 32],
        },
        votes: Vec::new(),
    };
    commit.votes.push(CommitVote {
        validator_address: validators[0].address,
        voting_power: 10,
        timestamp,
        signature: key
            .sign(&commit.vote_sign_bytes(&header.chain_id, timestamp))
            .to_bytes(),
    });
    (header, commit, validators)
}

struct InclusionFixture {
    state: Vec<u8>,
    state_proof: Proof,
    state_root: [u8; 32],
    header: Vec<u8>,
    header_signature: [u8; 64],
    authorization: SequencerAuthorization,
}

fn inclusion_fixture() -> InclusionFixture {
    let state = b"state-leaf".to_vec();
    let activity = b"activity-leaf".to_vec();
    let (state_proof, state_root) = build_proof(&[state.as_slice()], 0)
        .unwrap_or_else(|error| panic!("state proof: {error:?}"));
    let (_, activity_root) = build_proof(&[activity.as_slice()], 0)
        .unwrap_or_else(|error| panic!("activity proof: {error:?}"));
    let key = EdSigningKey::from_bytes(&[7; 32]);
    let sequencer_id = key.verifying_key().to_bytes();
    let header = header_bytes(state_root, activity_root, sequencer_id);
    let digest =
        batch_header_digest(&header).unwrap_or_else(|error| panic!("header digest: {error:?}"));
    InclusionFixture {
        state,
        state_proof,
        state_root,
        header,
        header_signature: key.sign(&digest).to_bytes(),
        authorization: SequencerAuthorization::new(sequencer_id, sequencer_id, 8, 8),
    }
}

#[test]
fn verified_checkpoint_derives_block_commit_without_anchor_escalation() {
    let (header, commit, validators) = committed_block();
    let served = checkpoint(8, "paxeer-test", &header, &commit, &validators)
        .unwrap_or_else(|error| panic!("checkpoint: {error:?}"));
    assert_eq!(served.height, 8);
    assert_eq!(served.block_hash, header.block_hash());
    assert_eq!(served.app_hash, header.app_hash);
    assert_eq!(served.commit, commit);
    assert_eq!(served.rung, Rung::Committed);

    let fixture = inclusion_fixture();
    let result = proof_bundle(&ProofBundleRequest {
        kind: ProofBundleKind::State,
        canonical_leaf_bytes: &fixture.state,
        proof: &fixture.state_proof,
        named_root: fixture.state_root,
        header_bytes: &fixture.header,
        header_signature: fixture.header_signature,
        authorization: &fixture.authorization,
    });
    let Err(CheckpointReadError::UnavailableCapability(error)) = result else {
        panic!("old state proofs cannot produce receipt-proof-v2");
    };
    assert_eq!(
        error.class,
        layerx_agent_api::error::ErrorClass::UnavailableCapability
    );
    assert_eq!(
        error.reason.as_str(),
        "unavailable_capability.read.proof_bundle"
    );
}

#[test]
fn missing_subthreshold_duplicate_and_chain_mismatch_are_refused() {
    let (header, commit, validators) = committed_block();
    let mut missing = commit.clone();
    missing.votes.clear();
    assert_eq!(
        checkpoint(8, "paxeer-test", &header, &missing, &validators),
        Err(CheckpointReadError::Commit)
    );
    let mut duplicate = commit.clone();
    duplicate.votes.push(duplicate.votes[0]);
    assert_eq!(
        checkpoint(8, "paxeer-test", &header, &duplicate, &validators),
        Err(CheckpointReadError::Commit)
    );
    assert_eq!(
        checkpoint(8, "other-chain", &header, &commit, &validators),
        Err(CheckpointReadError::Commit)
    );
    assert_eq!(
        checkpoint(7, "paxeer-test", &header, &commit, &validators),
        Err(CheckpointReadError::Commit)
    );
    let mut untrusted = validators.clone();
    untrusted[0].voting_power = 11;
    assert_eq!(
        checkpoint(8, "paxeer-test", &header, &commit, &untrusted),
        Err(CheckpointReadError::Commit)
    );
}
