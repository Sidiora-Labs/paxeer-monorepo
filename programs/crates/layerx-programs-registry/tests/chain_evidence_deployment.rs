#[path = "support/chain.rs"]
mod chain;
mod support;

use chain::{ChainFixture, STATE_HEIGHT};
use layerx_programs::chain_evidence::{
    deployment::ChainDeploymentProof, program_state::ChainProgramStateProof,
};
use layerx_programs::{
    chain_evidence, InterfaceEntryPoint, ProgramId, ProgramInterface, ProtocolEvidenceError,
    ReadFreshness, UpgradePolicy, ValueSchema, ValueType,
};
use sha2::{Digest, Sha256};

fn interface(code: &[u8], abi_version: u16) -> ProgramInterface {
    ProgramInterface::bind(
        code,
        abi_version,
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
    .unwrap()
}

fn deployment_fixture(
    code: &[u8],
    policy: UpgradePolicy,
    version: u32,
) -> (ChainFixture, ChainDeploymentProof) {
    let program = ProgramId::new([31; 32]).unwrap();
    let mut key = b"program\0".to_vec();
    key.extend_from_slice(&program.bytes());
    let key = chain_evidence::programs_state_key(&key);
    let mut value = Vec::with_capacity(71);
    match policy {
        UpgradePolicy::Immutable => {
            value.push(0);
            value.extend_from_slice(&[0; 32]);
        }
        UpgradePolicy::Authority(authority) => {
            value.push(1);
            value.extend_from_slice(&authority);
        }
    }
    value.extend_from_slice(&Sha256::digest(code));
    value.extend_from_slice(&1_u16.to_be_bytes());
    value.extend_from_slice(&version.to_be_bytes());
    assert_eq!(value.len(), 71);
    let fixture = ChainFixture::new(
        "paxeer-deployment-test",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
        vec![
            (chain_evidence::programs_state_key(b"a"), vec![1]),
            (key.clone(), value),
            (chain_evidence::programs_state_key(b"z"), vec![2]),
        ],
    )
    .unwrap();
    let proof = ChainDeploymentProof {
        program_state: ChainProgramStateProof {
            program,
            ops: fixture.prove(&key).unwrap(),
        },
        code: code.to_vec(),
        interface: None,
    };
    (fixture, proof)
}

#[test]
fn chain_deployment_binds_code_hash_at_head() {
    for (policy, version) in [
        (UpgradePolicy::Immutable, 1),
        (UpgradePolicy::Authority([32; 32]), 7),
    ] {
        let (fixture, mut proof) = deployment_fixture(support::WASM_V1, policy, version);
        let head = fixture
            .verifier
            .verify_head(&fixture.header, &fixture.commit)
            .unwrap();
        let bound_interface = interface(support::WASM_V1, 1);
        for supplied_interface in [None, Some(bound_interface.canonical_encoding().to_vec())] {
            proof.interface = supplied_interface;
            let evidence = proof.verify(&head).unwrap();
            assert_eq!(evidence.program(), proof.program_state.program);
            assert_eq!(evidence.version(), version);
            assert_eq!(evidence.upgrade_policy(), policy);
            assert_eq!(evidence.abi_version(), 1);
            let expected_hash: [u8; 32] = Sha256::digest(&proof.code).into();
            assert_eq!(evidence.code_hash(), expected_hash);
            assert_eq!(evidence.module(), proof.code);
            assert_eq!(evidence.state_height(), head.height() - 1);
            assert_eq!(evidence.state_height(), STATE_HEIGHT);
            assert_eq!(evidence.state_root(), head.app_hash());
            assert_eq!(
                evidence.freshness(),
                ReadFreshness {
                    observed_sequence: u64::try_from(head.height()).unwrap(),
                    observed_at: u64::try_from(head.time_ms()).unwrap(),
                }
            );
            assert_eq!(evidence.interface_present(), proof.interface.is_some());
            assert_eq!(
                evidence
                    .interface()
                    .map(ProgramInterface::canonical_encoding),
                proof.interface.as_deref()
            );
        }

        let (other, _) = deployment_fixture(support::WASM_V2, policy, version);
        let other_head = other
            .verifier
            .verify_head(&other.header, &other.commit)
            .unwrap();
        assert_ne!(head.app_hash(), other_head.app_hash());
        assert_eq!(
            proof.verify(&other_head),
            Err(ProtocolEvidenceError::StateProof)
        );
        let mut broken = proof.clone();
        broken.program_state.ops[1].data.pop();
        assert_eq!(broken.verify(&head), Err(ProtocolEvidenceError::StateProof));
    }
}

#[test]
fn chain_deployment_refuses_substituted_code() {
    let (fixture, proof) = deployment_fixture(support::WASM_V1, UpgradePolicy::Immutable, 1);
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap();
    assert!(proof.verify(&head).is_ok());
    for code in [support::WASM_V2.to_vec(), Vec::new()] {
        assert_ne!(Sha256::digest(&code), Sha256::digest(&proof.code));
        let substituted = ChainDeploymentProof {
            code,
            ..proof.clone()
        };
        assert_eq!(
            substituted.verify(&head),
            Err(ProtocolEvidenceError::DeploymentMismatch)
        );
    }

    let mut changed_code = proof.clone();
    let last_byte = changed_code.code.len() - 1;
    changed_code.code[last_byte] ^= 1;
    assert_eq!(
        changed_code.verify(&head),
        Err(ProtocolEvidenceError::DeploymentMismatch)
    );

    let mut malformed = interface(support::WASM_V1, 1).canonical_encoding().to_vec();
    malformed.push(0);
    for bytes in [
        interface(support::WASM_V2, 1).canonical_encoding().to_vec(),
        interface(support::WASM_V1, 2).canonical_encoding().to_vec(),
        malformed,
        Vec::new(),
    ] {
        let substituted = ChainDeploymentProof {
            interface: Some(bytes),
            ..proof.clone()
        };
        assert_eq!(
            substituted.verify(&head),
            Err(ProtocolEvidenceError::DeploymentMismatch)
        );
    }
    let mut other_program = proof;
    other_program.program_state.program = ProgramId::new([33; 32]).unwrap();
    assert_eq!(
        other_program.verify(&head),
        Err(ProtocolEvidenceError::ProgramRecord)
    );
}
