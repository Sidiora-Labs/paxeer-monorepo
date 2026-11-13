use support::chain;
#[allow(dead_code)]
#[path = "support/mod.rs"]
mod support;

#[allow(dead_code)]
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
    ProgramInterface, ProtocolDeploymentVerifier, ProtocolEvidenceError, ReadFreshness,
    UpgradePolicy,
};
use sha2::{Digest, Sha256};
use verify_deployment::{deployment_at, historical_deployment_at};

const CHAIN_ID: &str = "paxeer-deployment-verifier-test";
const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const MAX_AGE_MS: u64 = 1_000;

fn deployment_fixture(
    published: bool,
) -> (chain::ChainFixture, ChainDeploymentProof, ProgramInterface) {
    let program = repair_22_33_221::program();
    let key = repair_22_33_221::program_key();
    let interface = repair_22_33_221::descriptor(1);
    let record = repair_22_33_221::runtime_records()[0].clone();
    let interface_key =
        chain_evidence::programs_state_key(&layerx_programs::interface_state_key(program));
    let mut rows = vec![(key.clone(), repair_22_33_221::state_value(&record))];
    if published {
        rows.push((
            interface_key.clone(),
            layerx_programs::interface_state_value(program, 1, &interface)
                .unwrap_or_else(|error| panic!("{error:?}")),
        ));
    }
    let fixture = chain::ChainFixture::new(CHAIN_ID, &SEEDS, rows)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let witness = layerx_programs::chain_proof::decode::decode_commitment_proof(
        &fixture
            .encode_key(&interface_key)
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let proof = ChainDeploymentProof::from_state_proofs(
        ChainProgramStateProof {
            program,
            ops: fixture
                .prove(&key)
                .unwrap_or_else(|error| panic!("{error:?}")),
        },
        record.module,
        &witness,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    (fixture, proof, interface)
}

fn sign_header(header: &ChainHeader, template: &Commit, seeds: &[[u8; 32]]) -> Commit {
    let mut commit = template.clone();
    commit.height = header.height;
    commit.block_id.hash = header
        .hash()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .to_vec();
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
    .unwrap_or_else(|error| panic!("{error:?}"));
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

fn verify_deployment_variants(
    verifier: &ProtocolDeploymentVerifier,
    trust: &chain_proof::ChainProofVerifier,
    proof: &ChainDeploymentProof,
    old: &chain_proof::VerifiedChainHead,
    old_time: u64,
    interface: &ProgramInterface,
) {
    let expected_hash: [u8; 32] = Sha256::digest(support::WASM_V1).into();
    for published in [false, true] {
        let (case_fixture, proof, _) = deployment_fixture(published);
        let old = trust
            .verify_head(&case_fixture.header, &case_fixture.commit)
            .unwrap_or_else(|error| panic!("{error:?}"));
        let old_time = u64::from_le_bytes((old.time_ms()).to_le_bytes());
        let evidence = deployment_at(verifier, &proof, &old, old_time)
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(evidence.program(), proof.program_state.program);
        assert_eq!(evidence.version(), 1);
        assert_eq!(evidence.code_hash(), expected_hash);
        assert_eq!(evidence.abi_version(), 1);
        assert_eq!(
            evidence.upgrade_policy(),
            UpgradePolicy::Authority([0x51; 32])
        );
        assert_eq!(evidence.module(), support::WASM_V1);
        assert_eq!(evidence.state_height(), chain::STATE_HEIGHT);
        assert_eq!(evidence.state_root(), old.app_hash());
        assert_eq!(evidence.interface_present(), published);
        assert_eq!(
            evidence.interface(),
            if published { Some(interface) } else { None }
        );
        assert_eq!(
            evidence.freshness(),
            ReadFreshness {
                observed_sequence: u64::try_from(old.height())
                    .unwrap_or_else(|error| panic!("{error:?}")),
                observed_at: old_time,
            }
        );
    }
    assert!(deployment_at(verifier, proof, old, old_time + MAX_AGE_MS).is_ok());
    assert_eq!(
        deployment_at(verifier, proof, old, old_time + MAX_AGE_MS + 1),
        Err(ProtocolEvidenceError::Stale)
    );
    assert!(deployment_at(verifier, proof, old, old_time - 1_000).is_ok());
    assert_eq!(
        deployment_at(verifier, proof, old, old_time - 1_001),
        Err(ProtocolEvidenceError::Stale)
    );
}

type TrustHistoryStep = (ChainHeader, Commit, ValidatorSet);
const ROTATED_SEEDS: [[u8; 32]; 4] = [[11; 32], [12; 32], [13; 32], [14; 32]];

fn rotated_deployment_chain(
    fixture: &chain::ChainFixture,
    trust: &chain_proof::ChainProofVerifier,
) -> (
    chain_proof::ChainProofVerifier,
    chain_proof::VerifiedChainHead,
    Vec<TrustHistoryStep>,
) {
    let new_seeds = ROTATED_SEEDS;
    let mut new_validators = new_seeds
        .iter()
        .map(|seed| {
            let key = SigningKey::from_bytes(seed);
            TrustedValidator::new(key.verifying_key().to_bytes(), 1)
        })
        .collect::<Vec<_>>();
    new_validators.sort_by_key(|validator| validator.address);
    let new_validators =
        ValidatorSet::new(new_validators).unwrap_or_else(|error| panic!("{error:?}"));
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
        .unwrap_or_else(|error| panic!("{error:?}"));
    let trust_chain = vec![
        (
            fixture.header.clone(),
            fixture.commit.clone(),
            trust.validators.clone(),
        ),
        (intermediate, intermediate_commit, trust.validators.clone()),
        (current_header, current_commit, new_validators),
    ];
    (current_verifier, current, trust_chain)
}

fn verify_historical_deployment_rejections(
    verifier: &ProtocolDeploymentVerifier,
    proof: &ChainDeploymentProof,
    old: &chain_proof::VerifiedChainHead,
    current: &chain_proof::VerifiedChainHead,
    trust_chain: &[TrustHistoryStep],
    current_verifier: &chain_proof::ChainProofVerifier,
    times: (u64, u64),
) {
    for incomplete in [&trust_chain[..0], &trust_chain[..1], &trust_chain[..2]] {
        assert_eq!(
            historical_deployment_at(verifier, proof, old, current, incomplete, times.0),
            Err(ProtocolEvidenceError::HistoricalTrustAnchor)
        );
    }
    let mut tampered = trust_chain.to_vec();
    tampered[1].1.signatures[0].signature[0] ^= 1;
    assert_eq!(
        historical_deployment_at(verifier, proof, old, current, &tampered, times.0),
        Err(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
    let skipped_rotation = vec![trust_chain[0].clone(), trust_chain[2].clone()];
    assert_eq!(
        historical_deployment_at(verifier, proof, old, current, &skipped_rotation, times.0),
        Err(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
    let mut wrong_anchor = trust_chain.to_vec();
    wrong_anchor[0].2.clone_from(&current_verifier.validators);
    assert_eq!(
        historical_deployment_at(verifier, proof, old, current, &wrong_anchor, times.0),
        Err(ProtocolEvidenceError::InvalidTrustAnchor)
    );
    let mut fork_header = trust_chain[2].0.clone();
    fork_header.app_hash = vec![99; 32];
    let fork_commit = sign_header(&fork_header, &trust_chain[2].1, &ROTATED_SEEDS);
    let fork = current_verifier
        .verify_head(&fork_header, &fork_commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        historical_deployment_at(verifier, proof, old, &fork, trust_chain, times.0),
        Err(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
    assert_eq!(
        historical_deployment_at(
            verifier,
            proof,
            old,
            current,
            trust_chain,
            times.0 + MAX_AGE_MS + 1
        ),
        Err(ProtocolEvidenceError::Stale)
    );
    let other_verifier = ProtocolDeploymentVerifier::from_chain_trust(
        "other-chain",
        trust_chain[0].2.validators().to_vec(),
        MAX_AGE_MS,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        deployment_at(&other_verifier, proof, old, times.1),
        Err(ProtocolEvidenceError::ProtocolDomain)
    );
    assert_eq!(
        historical_deployment_at(&other_verifier, proof, old, current, trust_chain, times.0),
        Err(ProtocolEvidenceError::ProtocolDomain)
    );
}

#[test]
fn chain_deployment_current_and_historical_verify() {
    let (mut fixture, proof, interface) = deployment_fixture(true);
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        CHAIN_ID,
        fixture.verifier.validators.validators().to_vec(),
        MAX_AGE_MS,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let trust = verifier
        .chain_trust()
        .unwrap_or_else(|| panic!("expected value"));
    let old = trust
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let old_time = u64::try_from(old.time_ms()).unwrap_or_else(|error| panic!("{error:?}"));
    verify_deployment_variants(&verifier, trust, &proof, &old, old_time, &interface);

    let (current_verifier, current, trust_chain) = rotated_deployment_chain(&fixture, trust);
    let now = u64::try_from(current.time_ms()).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        deployment_at(&verifier, &proof, &old, now),
        Err(ProtocolEvidenceError::Stale)
    );
    let historical = historical_deployment_at(&verifier, &proof, &old, &current, &trust_chain, now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        historical,
        deployment_at(&verifier, &proof, &old, old_time)
            .unwrap_or_else(|error| panic!("{error:?}"))
    );
    // Refresh the witness framing for the actual current head. The unchanged
    // state root alone cannot authorize borrowing an old interface witness.
    assert_eq!(
        deployment_at(&verifier, &proof, &current, now),
        Err(ProtocolEvidenceError::StateProof)
    );
    fixture.header = trust_chain[2].0.clone();
    fixture.commit = trust_chain[2].1.clone();
    fixture.verifier.validators = trust_chain[2].2.clone();
    let mut current_proof = proof.clone();
    let key = chain_evidence::programs_state_key(&layerx_programs::interface_state_key(
        repair_22_33_221::program(),
    ));
    current_proof.interface = Some(
        fixture
            .encode_key(&key)
            .unwrap_or_else(|error| panic!("{error:?}")),
    );
    let current_evidence = deployment_at(&verifier, &current_proof, &current, now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(current_evidence.state_height(), current.height() - 1);
    assert_eq!(current_evidence.state_root(), current.app_hash());
    assert_eq!(current_evidence.module(), historical.module());
    assert_eq!(current_evidence.freshness().observed_at, now);

    verify_historical_deployment_rejections(
        &verifier,
        &proof,
        &old,
        &current,
        &trust_chain,
        &current_verifier,
        (now, old_time),
    );
}

#[test]
fn chain_deployment_refuses_code_hash_mismatch() {
    let (fixture, mut proof, _) = deployment_fixture(true);
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        CHAIN_ID,
        fixture.verifier.validators.validators().to_vec(),
        MAX_AGE_MS,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let head = verifier
        .chain_trust()
        .unwrap_or_else(|| panic!("expected value"))
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let now = u64::try_from(head.time_ms()).unwrap_or_else(|error| panic!("{error:?}"));
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

#[allow(dead_code)]
#[path = "support/repair_22_33_221.rs"]
mod repair_22_33_221;

#[test]
fn node_interface_presence_absence_current_historical() {
    repair_22_33_221::verifier_interface_coverage();
}
