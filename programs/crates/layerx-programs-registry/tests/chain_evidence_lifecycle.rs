#[path = "support/chain.rs"]
mod chain;
#[path = "../src/hash.rs"]
mod hash;
#[path = "../src/interface.rs"]
mod interface;
#[path = "../src/chain_evidence/lifecycle.rs"]
mod lifecycle;
#[path = "../src/protocol_evidence.rs"]
mod protocol_evidence;

use chain::ChainFixture;
use interface::{
    verify_interface_read, InterfaceRefusal, InterfaceStateWitness, ProgramInterface,
    VerifiedInterfaceRead,
};
use layerx_programs as account_state;
use layerx_programs::{
    chain_evidence, chain_proof, AccountStateHead, DeploymentRecord, ProgramId, ProgramLifecycle,
    ReadFreshness, StateProof, UpgradePolicy,
};
use lifecycle::ChainLifecycleProof;
use protocol_evidence::{ProtocolEvidenceError, VerifiedDeploymentEvidence, VerifiedProgramHead};

fn key(prefix: &[u8], program: ProgramId) -> Vec<u8> {
    let mut key = prefix.to_vec();
    key.extend_from_slice(&program.bytes());
    chain_evidence::programs_state_key(&key)
}

fn program_record() -> Vec<u8> {
    let mut value = vec![0; 33];
    value.extend_from_slice(&[9; 32]);
    value.extend_from_slice(&1_u16.to_be_bytes());
    value.extend_from_slice(&1_u32.to_be_bytes());
    value
}

fn status_record(program: ProgramId, lifecycle: u8) -> Vec<u8> {
    let mut value = vec![1, lifecycle];
    value.extend_from_slice(&program.bytes());
    value.extend_from_slice(&1_700_000_000_u64.to_be_bytes());
    value.extend_from_slice(&37_u64.to_be_bytes());
    value.extend_from_slice(&0_u32.to_be_bytes());
    value
}

fn fixture(pairs: Vec<(Vec<u8>, Vec<u8>)>) -> ChainFixture {
    ChainFixture::new(
        "paxeer-lifecycle-test",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
        pairs,
    )
    .unwrap()
}

#[test]
fn chain_lifecycle_reads_proven_record_state() {
    let active = ProgramId::new([0x11; 32]).unwrap();
    let deprecated = ProgramId::new([0x22; 32]).unwrap();
    let tombstoned = ProgramId::new([0x33; 32]).unwrap();
    let missing = ProgramId::new([0x44; 32]).unwrap();
    let record = program_record();
    let fixture = fixture(vec![
        (key(b"program\0", active), record.clone()),
        (key(b"program\0", deprecated), record.clone()),
        (key(b"program\0", tombstoned), record.clone()),
        (
            key(b"wind-down\0s", deprecated),
            status_record(deprecated, 2),
        ),
        (
            key(b"wind-down\0s", tombstoned),
            status_record(tombstoned, 3),
        ),
    ]);
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap();
    let proof_for = |program, status| ChainLifecycleProof {
        program,
        record: Some(record.clone()),
        record_ops: fixture.prove(&key(b"program\0", program)).unwrap(),
        status,
        status_ops: if program == active {
            fixture
                .prove_absent(&key(b"wind-down\0s", program))
                .unwrap()
        } else {
            fixture.prove(&key(b"wind-down\0s", program)).unwrap()
        },
    };
    let active_proof = proof_for(active, None);
    let deprecated_proof = proof_for(deprecated, Some(status_record(deprecated, 2)));
    let tombstoned_proof = proof_for(tombstoned, Some(status_record(tombstoned, 3)));
    assert_eq!(active_proof.verify(&head), Ok(ProgramLifecycle::Active));
    assert_eq!(
        deprecated_proof.verify(&head),
        Ok(ProgramLifecycle::Deprecated)
    );
    assert_eq!(
        tombstoned_proof.verify(&head),
        Ok(ProgramLifecycle::Tombstoned)
    );

    let absent = ChainLifecycleProof {
        program: missing,
        record: None,
        record_ops: fixture.prove_absent(&key(b"program\0", missing)).unwrap(),
        status: None,
        status_ops: fixture
            .prove_absent(&key(b"wind-down\0s", missing))
            .unwrap(),
    };
    assert_eq!(
        absent.verify(&head),
        Err(ProtocolEvidenceError::LifecycleProof)
    );
    let fabricated = ChainLifecycleProof {
        record: Some(record.clone()),
        ..absent
    };
    assert_eq!(
        fabricated.verify(&head),
        Err(ProtocolEvidenceError::StateProof)
    );

    let mut changed_status = deprecated_proof.clone();
    changed_status.status = Some(status_record(deprecated, 3));
    let mut changed_record = deprecated_proof.clone();
    changed_record.record.as_mut().unwrap()[33] ^= 1;
    let mut wrong_program = deprecated_proof.clone();
    wrong_program.program = active;
    let mut missing_record_ops = deprecated_proof.clone();
    missing_record_ops.record_ops.clear();
    let mut missing_status_ops = deprecated_proof.clone();
    missing_status_ops.status_ops.clear();
    let mut hidden_status = deprecated_proof.clone();
    hidden_status.status = None;
    let mut wrong_store = deprecated_proof.clone();
    wrong_store.status_ops[1].key = b"bank".to_vec();
    let mut damaged_ops = deprecated_proof.clone();
    damaged_ops.record_ops[0].data[0] ^= 1;
    for invalid in [
        changed_status,
        changed_record,
        wrong_program,
        missing_record_ops,
        missing_status_ops,
        hidden_status,
        wrong_store,
        damaged_ops,
    ] {
        assert_eq!(
            invalid.verify(&head),
            Err(ProtocolEvidenceError::StateProof)
        );
    }

    let other = self::fixture(vec![
        (key(b"program\0", deprecated), record.clone()),
        (
            key(b"wind-down\0s", deprecated),
            status_record(deprecated, 3),
        ),
    ]);
    let other_head = other
        .verifier
        .verify_head(&other.header, &other.commit)
        .unwrap();
    assert_ne!(head.app_hash(), other_head.app_hash());
    assert_eq!(
        deprecated_proof.verify(&other_head),
        Err(ProtocolEvidenceError::StateProof)
    );
    let mixed_heads = ChainLifecycleProof {
        status: Some(status_record(deprecated, 3)),
        status_ops: other.prove(&key(b"wind-down\0s", deprecated)).unwrap(),
        ..deprecated_proof.clone()
    };
    assert_eq!(
        mixed_heads.verify(&head),
        Err(ProtocolEvidenceError::StateProof)
    );
    assert_eq!(
        mixed_heads.verify(&other_head),
        Err(ProtocolEvidenceError::StateProof)
    );

    let canonical = status_record(deprecated, 2);
    let mut invalid_statuses = Vec::new();
    for length in [1, 53, 55] {
        let mut value = canonical.clone();
        value.resize(length, 0);
        invalid_statuses.push(value);
    }
    for (index, byte) in [(0, 0), (1, 0), (1, 1), (1, 4), (2, 0)] {
        let mut value = canonical.clone();
        value[index] = byte;
        invalid_statuses.push(value);
    }
    for range in [34..42, 42..50] {
        let mut value = canonical.clone();
        value[range].fill(0);
        invalid_statuses.push(value);
    }
    for value in invalid_statuses {
        let invalid = self::fixture(vec![
            (key(b"program\0", deprecated), record.clone()),
            (key(b"wind-down\0s", deprecated), value.clone()),
        ]);
        let invalid_head = invalid
            .verifier
            .verify_head(&invalid.header, &invalid.commit)
            .unwrap();
        let proof = ChainLifecycleProof {
            program: deprecated,
            record: Some(record.clone()),
            record_ops: invalid.prove(&key(b"program\0", deprecated)).unwrap(),
            status: Some(value),
            status_ops: invalid.prove(&key(b"wind-down\0s", deprecated)).unwrap(),
        };
        assert_eq!(
            proof.verify(&invalid_head),
            Err(ProtocolEvidenceError::LifecycleProof)
        );
    }

    let mut malformed_record = record;
    malformed_record[0] = 2;
    let malformed = self::fixture(vec![(key(b"program\0", active), malformed_record.clone())]);
    let malformed_head = malformed
        .verifier
        .verify_head(&malformed.header, &malformed.commit)
        .unwrap();
    let malformed_proof = ChainLifecycleProof {
        record: Some(malformed_record),
        record_ops: malformed.prove(&key(b"program\0", active)).unwrap(),
        status_ops: malformed
            .prove_absent(&key(b"wind-down\0s", active))
            .unwrap(),
        ..active_proof
    };
    assert_eq!(
        malformed_proof.verify(&malformed_head),
        Err(ProtocolEvidenceError::ProgramRecord)
    );
}
