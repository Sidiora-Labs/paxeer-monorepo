#[path = "support/chain.rs"]
mod chain;
mod support;

#[path = "../src/chain_evidence/verify_deployment.rs"]
mod verify_deployment;

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::{
    chain_evidence::{
        self, deployment::ChainDeploymentProof, program_state::ChainProgramStateProof,
    },
    chain_proof::{
        self,
        header::ChainHeader,
        validators::{TrustedValidator, ValidatorSet},
        vote::{vote_sign_bytes, BlockIdFlag, CommitSig, Timestamp},
        Commit,
    },
    InterfaceEntryPoint, ProgramId, ProgramInterface, ProtocolDeploymentVerifier,
    ProtocolEvidenceError, ReadFreshness, UpgradePolicy, ValueSchema, ValueType,
};
use sha2::{Digest, Sha256};
use verify_deployment::{deployment_at, historical_deployment_at};

const CHAIN_ID: &str = "paxeer-deployment-verifier-test";
const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const MAX_AGE_MS: u64 = 1_000;

fn deployment_fixture() -> (chain::ChainFixture, ChainDeploymentProof, ProgramInterface) {
    let program = ProgramId::new([31; 32]).unwrap();
    let mut key = b"program\0".to_vec();
    key.extend_from_slice(&program.bytes());
    let key = chain_evidence::programs_state_key(&key);
    let mut value = vec![1];
    value.extend_from_slice(&[32; 32]);
    value.extend_from_slice(&Sha256::digest(support::WASM_V1));
    value.extend_from_slice(&1_u16.to_be_bytes());
    value.extend_from_slice(&7_u32.to_be_bytes());
    let fixture = chain::ChainFixture::new(
        CHAIN_ID,
        &SEEDS,
        vec![
            (chain_evidence::programs_state_key(b"a"), vec![1]),
            (key.clone(), value),
            (chain_evidence::programs_state_key(b"z"), vec![2]),
        ],
    )
    .unwrap();
    let interface = ProgramInterface::bind(
        support::WASM_V1,
        1,
        vec![InterfaceEntryPoint {
            name: "call".to_owned(),
            discriminator: [1, 2, 3, 4],
            calldata: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
            response: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
            capabilities: Vec::new(),
            event_topics: Vec::new(),
            failures: Vec::new(),
        }],
    )
    .unwrap();
    let proof = ChainDeploymentProof {
        program_state: ChainProgramStateProof {
            program,
            ops: fixture.prove(&key).unwrap(),
        },
        code: support::WASM_V1.to_vec(),
        interface: Some(interface.canonical_encoding().to_vec()),
    };
    (fixture, proof, interface)
}

fn sign_header(header: &ChainHeader, template: &Commit, seeds: &[[u8; 32]]) -> Commit {
    let mut commit = template.clone();
    commit.height = header.height;
    commit.block_id.hash = header.hash().unwrap().to_vec();
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
    )
    .unwrap();
    commit.signatures = seeds
        .iter()
        .map(|seed| {
            let key = SigningKey::from_bytes(seed);
            CommitSig {
                flag: BlockIdFlag::Commit,
                validator_address: TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .to_vec(),
                timestamp,
                signature: key.sign(&bytes).to_bytes().to_vec(),
            }
        })
        .collect();
    commit
}

#[test]
fn chain_deployment_current_and_historical_verify() {
    let (fixture, mut proof, interface) = deployment_fixture();
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        CHAIN_ID,
        fixture.verifier.validators.validators().to_vec(),
        MAX_AGE_MS,
    )
    .unwrap();
    let trust = verifier.chain_trust().unwrap();
    let old = trust.verify_head(&fixture.header, &fixture.commit).unwrap();
    let old_time = u64::try_from(old.time_ms()).unwrap();
    let expected_hash: [u8; 32] = Sha256::digest(support::WASM_V1).into();
    for supplied_interface in [None, Some(interface.canonical_encoding().to_vec())] {
        proof.interface = supplied_interface;
        let evidence = deployment_at(&verifier, &proof, &old, old_time).unwrap();
        assert_eq!(evidence.program(), proof.program_state.program);
        assert_eq!(evidence.version(), 7);
        assert_eq!(evidence.code_hash(), expected_hash);
        assert_eq!(evidence.abi_version(), 1);
        assert_eq!(
            evidence.upgrade_policy(),
            UpgradePolicy::Authority([32; 32])
        );
        assert_eq!(evidence.module(), support::WASM_V1);
        assert_eq!(evidence.state_height(), chain::STATE_HEIGHT);
        assert_eq!(evidence.state_root(), old.app_hash());
        assert_eq!(evidence.interface_present(), proof.interface.is_some());
        assert_eq!(
            evidence.interface(),
            proof.interface.as_ref().map(|_| &interface)
        );
        assert_eq!(
            evidence.freshness(),
            ReadFreshness {
                observed_sequence: u64::try_from(old.height()).unwrap(),
                observed_at: old_time,
            }
        );
    }
    assert!(deployment_at(&verifier, &proof, &old, old_time + MAX_AGE_MS).is_ok());
    assert_eq!(
        deployment_at(&verifier, &proof, &old, old_time + MAX_AGE_MS + 1),
        Err(ProtocolEvidenceError::Stale)
    );
    assert!(deployment_at(&verifier, &proof, &old, old_time - 1_000).is_ok());
    assert_eq!(
        deployment_at(&verifier, &proof, &old, old_time - 1_001),
        Err(ProtocolEvidenceError::Stale)
    );

    let new_seeds = [[11; 32], [12; 32], [13; 32], [14; 32]];
    let mut new_validators = new_seeds
        .iter()
        .map(|seed| {
            let key = SigningKey::from_bytes(seed);
            TrustedValidator::new(key.verifying_key().to_bytes(), 1)
        })
        .collect::<Vec<_>>();
    new_validators.sort_by_key(|validator| validator.address);
    let new_validators = ValidatorSet::new(new_validators).unwrap();
    let mut intermediate = fixture.header.clone();
    intermediate.height += 1;
    intermediate.time.seconds += 10;
    intermediate.next_validators_hash = new_validators.hash().to_vec();
    let intermediate_commit = sign_header(&intermediate, &fixture.commit, &SEEDS);
    let mut current_header = intermediate.clone();
    current_header.height += 1;
    current_header.time.seconds += 10;
    current_header.validators_hash = new_validators.hash().to_vec();
    let current_commit = sign_header(&current_header, &fixture.commit, &new_seeds);
    let current_verifier = chain_proof::ChainProofVerifier {
        chain_id: CHAIN_ID.to_owned(),
        validators: new_validators.clone(),
    };
    let current = current_verifier
        .verify_head(&current_header, &current_commit)
        .unwrap();
    let now = u64::try_from(current.time_ms()).unwrap();
    let trust_chain = vec![
        (
            fixture.header.clone(),
            fixture.commit.clone(),
            trust.validators.clone(),
        ),
        (intermediate, intermediate_commit, trust.validators.clone()),
        (current_header, current_commit, new_validators),
    ];
    assert_eq!(
        deployment_at(&verifier, &proof, &old, now),
        Err(ProtocolEvidenceError::Stale)
    );
    let historical =
        historical_deployment_at(&verifier, &proof, &old, &current, &trust_chain, now).unwrap();
    assert_eq!(
        historical,
        deployment_at(&verifier, &proof, &old, old_time).unwrap()
    );
    let current_evidence = deployment_at(&verifier, &proof, &current, now).unwrap();
    assert_eq!(current_evidence.state_height(), current.height() - 1);
    assert_eq!(current_evidence.state_root(), current.app_hash());
    assert_eq!(current_evidence.module(), historical.module());
    assert_eq!(current_evidence.freshness().observed_at, now);

    for incomplete in [&trust_chain[..0], &trust_chain[..1], &trust_chain[..2]] {
        assert_eq!(
            historical_deployment_at(&verifier, &proof, &old, &current, incomplete, now),
            Err(ProtocolEvidenceError::HistoricalTrustAnchor)
        );
    }
    let mut tampered = trust_chain.clone();
    tampered[1].1.signatures[0].signature[0] ^= 1;
    assert_eq!(
        historical_deployment_at(&verifier, &proof, &old, &current, &tampered, now),
        Err(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
    let skipped_rotation = vec![trust_chain[0].clone(), trust_chain[2].clone()];
    assert_eq!(
        historical_deployment_at(&verifier, &proof, &old, &current, &skipped_rotation, now),
        Err(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
    let mut wrong_anchor = trust_chain.clone();
    wrong_anchor[0].2 = current_verifier.validators.clone();
    assert_eq!(
        historical_deployment_at(&verifier, &proof, &old, &current, &wrong_anchor, now),
        Err(ProtocolEvidenceError::InvalidTrustAnchor)
    );
    let mut fork_header = trust_chain[2].0.clone();
    fork_header.app_hash = vec![99; 32];
    let fork_commit = sign_header(&fork_header, &fixture.commit, &new_seeds);
    let fork = current_verifier
        .verify_head(&fork_header, &fork_commit)
        .unwrap();
    assert_eq!(
        historical_deployment_at(&verifier, &proof, &old, &fork, &trust_chain, now),
        Err(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
    assert_eq!(
        historical_deployment_at(
            &verifier,
            &proof,
            &old,
            &current,
            &trust_chain,
            now + MAX_AGE_MS + 1
        ),
        Err(ProtocolEvidenceError::Stale)
    );
    let other_verifier = ProtocolDeploymentVerifier::from_chain_trust(
        "other-chain",
        trust.validators.validators().to_vec(),
        MAX_AGE_MS,
    )
    .unwrap();
    assert_eq!(
        deployment_at(&other_verifier, &proof, &old, old_time),
        Err(ProtocolEvidenceError::ProtocolDomain)
    );
    assert_eq!(
        historical_deployment_at(&other_verifier, &proof, &old, &current, &trust_chain, now),
        Err(ProtocolEvidenceError::ProtocolDomain)
    );
}

#[test]
fn chain_deployment_refuses_code_hash_mismatch() {
    let (fixture, mut proof, _) = deployment_fixture();
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        CHAIN_ID,
        fixture.verifier.validators.validators().to_vec(),
        MAX_AGE_MS,
    )
    .unwrap();
    let head = verifier
        .chain_trust()
        .unwrap()
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap();
    let now = u64::try_from(head.time_ms()).unwrap();
    assert!(deployment_at(&verifier, &proof, &head, now).is_ok());
    let trust_chain = [(
        fixture.header.clone(),
        fixture.commit.clone(),
        fixture.verifier.validators.clone(),
    )];
    for interface in [proof.interface.clone(), None] {
        proof.interface = interface;
        proof.code = support::WASM_V2.to_vec();
        assert_eq!(
            deployment_at(&verifier, &proof, &head, now),
            Err(ProtocolEvidenceError::DeploymentMismatch)
        );
        assert_eq!(
            historical_deployment_at(&verifier, &proof, &head, &head, &trust_chain, now),
            Err(ProtocolEvidenceError::DeploymentMismatch)
        );
    }
}
