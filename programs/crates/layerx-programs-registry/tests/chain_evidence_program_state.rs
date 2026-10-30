#[path = "support/chain.rs"]
mod chain;
#[path = "../src/hash.rs"]
mod hash;
#[path = "../src/interface.rs"]
mod interface;
#[path = "../src/chain_evidence/program_state.rs"]
mod program_state;
#[path = "../src/protocol_evidence.rs"]
mod protocol_evidence;

use chain::{ChainFixture, STATE_HEIGHT};
use chain_evidence::programs_state_key;
use interface::{
    verify_interface_read, InterfaceRefusal, InterfaceStateWitness, ProgramInterface,
    VerifiedInterfaceRead,
};
use layerx_programs as account_state;
use layerx_programs::{
    chain_evidence, chain_proof, AccountStateHead, DeploymentRecord, ProgramId, ProgramLifecycle,
    ReadFreshness, StateProof, UpgradePolicy,
};
use program_state::ChainProgramStateProof;
use protocol_evidence::{ProtocolEvidenceError, VerifiedDeploymentEvidence, VerifiedProgramHead};

fn program_key(program: ProgramId) -> Vec<u8> {
    let mut key = b"program\0".to_vec();
    key.extend_from_slice(&program.bytes());
    programs_state_key(&key)
}

fn program_record(policy: UpgradePolicy, code_hash: [u8; 32], version: u32) -> Vec<u8> {
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
    value.extend_from_slice(&code_hash);
    value.extend_from_slice(&1_u16.to_be_bytes());
    value.extend_from_slice(&version.to_be_bytes());
    value
}

#[test]
fn chain_program_record_verifies_at_head() {
    let program = ProgramId::new([11; 32]).unwrap();
    let key = program_key(program);
    assert_eq!(&key[..3], &[0x01, 0x00, 0x09]);
    assert_eq!(&key[3..11], b"program\0");
    assert_eq!(&key[11..], &program.bytes());

    for policy in [UpgradePolicy::Immutable, UpgradePolicy::Authority([12; 32])] {
        let value = program_record(policy, [13; 32], 7);
        assert_eq!(value.len(), 71);
        let fixture = ChainFixture::new(
            "paxeer-program-state",
            &[[1; 32], [2; 32], [3; 32], [4; 32]],
            vec![
                (programs_state_key(b"a"), vec![1]),
                (key.clone(), value),
                (programs_state_key(b"z"), vec![2]),
            ],
        )
        .unwrap();
        let head = fixture
            .verifier
            .verify_head(&fixture.header, &fixture.commit)
            .unwrap();
        let proof = ChainProgramStateProof {
            program,
            ops: fixture.prove(&key).unwrap(),
        };
        let record = proof.verify(&head).unwrap();
        assert_eq!(record.program(), program);
        assert_eq!(record.code_hash(), [13; 32]);
        assert_eq!(record.abi_version(), 1);
        assert_eq!(record.version(), 7);
        assert_eq!(record.upgrade_policy(), policy);
        assert_eq!(record.state_height(), STATE_HEIGHT);
        assert_eq!(record.state_height(), head.height() - 1);
        assert_eq!(record.state_root(), head.app_hash());

        let other = ChainFixture::new(
            "paxeer-program-state",
            &[[1; 32]],
            vec![(key.clone(), program_record(policy, [14; 32], 8))],
        )
        .unwrap();
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
        broken.ops[1].data.pop();
        assert_eq!(broken.verify(&head), Err(ProtocolEvidenceError::StateProof));
        let mut wrong_store = proof.clone();
        wrong_store.ops[1].key = b"bank".to_vec();
        assert_eq!(
            wrong_store.verify(&head),
            Err(ProtocolEvidenceError::StateProof)
        );
        let mut empty = proof;
        empty.ops.clear();
        assert_eq!(empty.verify(&head), Err(ProtocolEvidenceError::StateProof));
    }
}

#[test]
fn chain_program_record_refuses_other_program() {
    let program = ProgramId::new([21; 32]).unwrap();
    let other_program = ProgramId::new([22; 32]).unwrap();
    let key = program_key(program);
    let canonical = program_record(UpgradePolicy::Authority([23; 32]), [24; 32], 3);
    let fixture = ChainFixture::new(
        "paxeer-program-refusal",
        &[[1; 32]],
        vec![(key.clone(), canonical.clone())],
    )
    .unwrap();
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap();
    let proof = ChainProgramStateProof {
        program,
        ops: fixture.prove(&key).unwrap(),
    };
    assert!(proof.verify(&head).is_ok());
    let other_proof = ChainProgramStateProof {
        program: other_program,
        ops: proof.ops.clone(),
    };
    assert_eq!(
        other_proof.verify(&head),
        Err(ProtocolEvidenceError::ProgramRecord)
    );

    for length in [1, 33, 65, 67, 70, 72, 142] {
        let mut value = canonical.clone();
        value.resize(length, 0);
        let fixture = ChainFixture::new(
            "paxeer-program-refusal",
            &[[1; 32]],
            vec![(key.clone(), value)],
        )
        .unwrap();
        let head = fixture
            .verifier
            .verify_head(&fixture.header, &fixture.commit)
            .unwrap();
        let proof = ChainProgramStateProof {
            program,
            ops: fixture.prove(&key).unwrap(),
        };
        assert_eq!(
            proof.verify(&head),
            Err(ProtocolEvidenceError::ProgramRecord)
        );
    }

    let mut wrong_module = key;
    wrong_module[2] = 8;
    let fixture = ChainFixture::new(
        "paxeer-program-refusal",
        &[[1; 32]],
        vec![(wrong_module.clone(), canonical)],
    )
    .unwrap();
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap();
    let proof = ChainProgramStateProof {
        program,
        ops: fixture.prove(&wrong_module).unwrap(),
    };
    assert_eq!(proof.verify(&head), Err(ProtocolEvidenceError::StateProof));
}
