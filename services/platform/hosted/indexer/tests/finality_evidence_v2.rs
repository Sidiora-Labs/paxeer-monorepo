use ed25519_dalek::{Signer as _, SigningKey};
use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    header::{
        validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator,
    },
    proto::{encode_length_delimited, encode_varint, encode_varint_field},
    ProofError,
};
use layerx_indexer::{
    ai_market::{verify_block_commit, BlockCommitProof, FinalityAuthority, VerifiedCommit},
    codec::{hex, unhex},
};
use layerx_programs_ai_market::{
    queries::{SnapshotBinding, BINDING_MAX_BYTES, FINALIZED_RANK},
    types::{
        ChainDomain, Digest32, MarketId, PolicyDigest, Presence, ProgramId, RosterDigest,
        StateDigest, Version,
    },
};
use sha2::{Digest as _, Sha256};

#[test]
fn finality_evidence_v2_matches_vector() {
    // The shared codec vector deliberately distinguishes read facts from commit
    // facts. In particular, none of the four commit fields may come from the read.
    let committed = VerifiedCommit {
        chain_id: "snapshot-binding-v2".to_owned(),
        height: 0x5152_5354_5556_5758,
        block_hash: [0xB2; 32],
        app_hash: [0xC2; 32],
        store_proof_digest: [0xD2; 32],
    };
    let native_state_root = Digest32::new([0xA1; 32]).expect("native root");
    let evidence = committed
        .finality_evidence(native_state_root)
        .expect("v2 evidence");
    assert_eq!(evidence.native_state_root, native_state_root);
    assert_eq!(evidence.block_height, committed.height);
    assert_eq!(evidence.block_hash.bytes(), committed.block_hash);
    assert_eq!(evidence.app_hash.bytes(), committed.app_hash);
    assert_eq!(
        evidence.store_proof_digest.bytes(),
        committed.store_proof_digest
    );
    assert_eq!(evidence.rank, FINALIZED_RANK);
    let binding = SnapshotBinding {
        chain: ChainDomain::new([10; 32]).expect("chain"),
        program: ProgramId::new([11; 32]).expect("program"),
        market: MarketId::new([12; 32]).expect("market"),
        observed_sequence: 0x0102_0304_0506_0708,
        execution_height: 0x1112_1314_1516_1718,
        batch_id: Digest32::new([0xBB; 32]).expect("read block hash"),
        native_state_root: evidence.native_state_root,
        revision: 0x2122_2324_2526_2728,
        state_digest: StateDigest::new([0xD1; 32]).expect("state digest"),
        epoch: Presence::Present(0x3132_3334_3536_3738),
        config: Version::new(0x4142_4344_4546_4748).expect("config"),
        policy: PolicyDigest::new([0xE1; 32]).expect("policy"),
        roster: Presence::Present(RosterDigest::new([0xF1; 32]).expect("roster")),
        block_height: evidence.block_height,
        block_hash: evidence.block_hash,
        app_hash: evidence.app_hash,
        store_proof_digest: evidence.store_proof_digest,
        rank: evidence.rank,
        publication_time_ms: 0x6162_6364_6566_6768,
    };
    let vector = include_str!("../../../../../programs/tests/vectors/snapshot-binding-v2.hex");
    let pinned = unhex(vector.strip_suffix('\n').expect("vector newline")).expect("vector hex");
    let mut encoded = [0; BINDING_MAX_BYTES];
    let n = binding.encode(&mut encoded).expect("encode v2 binding");
    assert_eq!(n, BINDING_MAX_BYTES);
    assert_eq!(vector.len(), 2 * n + 1);
    assert_eq!(&encoded[..n], pinned.as_slice());
    assert_eq!(hex(&encoded[..n]), vector.trim_end());
    binding.require_finalized().expect("rank-4 commit evidence");

    // Exercise the same conversion with an actual verified consensus commit.
    let key = SigningKey::from_bytes(&[7; 32]);
    let validators = vec![Validator {
        pub_key: key.verifying_key().to_bytes(),
        voting_power: 10,
    }];
    let authority = FinalityAuthority {
        chain_id: committed.chain_id.clone(),
        validators,
    };
    let header = Header {
        chain_id: authority.chain_id.clone(),
        height: 0x5152_5354_5556_5758,
        app_hash: committed.app_hash.to_vec(),
        version: ConsensusVersion { block: 11, app: 1 },
        time: Timestamp {
            seconds: 1_000,
            nanos: 123,
        },
        last_block_id: BlockId::default(),
        last_commit_hash: vec![1; 32],
        data_hash: vec![2; 32],
        validators_hash: validators_hash(&authority.validators),
        next_validators_hash: validators_hash(&authority.validators).to_vec(),
        consensus_hash: vec![3; 32],
        last_results_hash: vec![4; 32],
        evidence_hash: vec![5; 32],
        proposer_address: Sha256::digest(key.verifying_key().as_bytes())[..20].to_vec(),
    };
    let mut commit = signed_commit(&header, &key);
    let store_proof = b"snapshot-binding-v2 store proof bytes";
    let verified = verify_block_commit(
        &BlockCommitProof {
            header: &header,
            commit: &commit,
            store_proof,
        },
        &authority,
    )
    .expect("verify signed block commit");
    let root = Digest32::new(verified.app_hash).expect("verified native root");
    let actual = verified
        .finality_evidence(root)
        .expect("verified v2 evidence");
    assert_eq!(actual.block_height, committed.height);
    assert_eq!(actual.block_hash.bytes(), header.hash());
    assert_eq!(actual.app_hash.bytes(), committed.app_hash);
    assert_eq!(actual.native_state_root, actual.app_hash);
    assert_eq!(
        actual.store_proof_digest.bytes(),
        <[u8; 32]>::from(Sha256::digest(store_proof))
    );
    assert_eq!(actual.rank, FINALIZED_RANK);
    let rebound = SnapshotBinding {
        block_height: actual.block_height,
        block_hash: actual.block_hash,
        app_hash: actual.app_hash,
        store_proof_digest: actual.store_proof_digest,
        rank: actual.rank,
        ..binding
    };
    assert_eq!(rebound.snapshot_id(), binding.snapshot_id());
    let m = rebound
        .encode(&mut encoded)
        .expect("encode verified commit binding");
    assert_eq!(m, n);
    assert_ne!(&encoded[..m], pinned.as_slice());

    let signature = commit.signatures[0]
        .signature
        .as_mut()
        .expect("commit signature");
    signature[0] ^= 1;
    assert_eq!(
        verify_block_commit(
            &BlockCommitProof {
                header: &header,
                commit: &commit,
                store_proof
            },
            &authority,
        ),
        Err(ProofError::InvalidCommitSignature),
    );
    for invalid in [
        VerifiedCommit {
            block_hash: [0; 32],
            ..committed.clone()
        },
        VerifiedCommit {
            app_hash: [0; 32],
            ..committed.clone()
        },
        VerifiedCommit {
            store_proof_digest: [0; 32],
            ..committed
        },
    ] {
        assert!(invalid.finality_evidence(native_state_root).is_err());
    }
}

fn signed_commit(header: &Header, key: &SigningKey) -> Commit {
    let mut commit = Commit {
        height: u64::try_from(header.height).expect("positive block height"),
        round: 0,
        block_id: BlockId {
            hash: header.hash().to_vec(),
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![0xA5; 32],
            },
        },
        signatures: Vec::new(),
    };
    let mut vote = Vec::new();
    encode_varint_field(1, 2, &mut vote);
    vote.push(0x11);
    vote.extend_from_slice(&commit.height.to_le_bytes());
    let mut parts = Vec::new();
    encode_varint_field(
        1,
        u64::from(commit.block_id.part_set_header.total),
        &mut parts,
    );
    encode_length_delimited(2, &commit.block_id.part_set_header.hash, &mut parts);
    let mut block = Vec::new();
    encode_length_delimited(1, &commit.block_id.hash, &mut block);
    encode_length_delimited(2, &parts, &mut block);
    encode_length_delimited(4, &block, &mut vote);
    let mut time = Vec::new();
    encode_varint_field(
        1,
        u64::try_from(header.time.seconds).expect("positive timestamp"),
        &mut time,
    );
    encode_varint_field(2, u64::from(header.time.nanos), &mut time);
    encode_length_delimited(5, &time, &mut vote);
    encode_length_delimited(6, header.chain_id.as_bytes(), &mut vote);
    let mut sign_bytes = Vec::new();
    encode_varint(
        u64::try_from(vote.len()).expect("vote length"),
        &mut sign_bytes,
    );
    sign_bytes.extend_from_slice(&vote);
    commit.signatures.push(CommitSig {
        block_id_flag: BlockIdFlag::Commit,
        validator_address: Sha256::digest(key.verifying_key().as_bytes())[..20].to_vec(),
        timestamp: header.time,
        signature: Some(key.sign(&sign_bytes).to_bytes()),
    });
    commit
}
