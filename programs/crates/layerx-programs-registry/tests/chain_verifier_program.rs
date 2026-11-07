#[path = "support/chain.rs"]
mod chain;
#[path = "../src/chain_evidence/verify_program.rs"]
mod verify_program;

use chain::{ChainFixture, STATE_HEIGHT};
use layerx_programs::chain_evidence::{
    lifecycle::ChainLifecycleProof, program_state::ChainProgramStateProof,
    wind_down::ChainWindDownProof,
};
use layerx_programs::{
    chain_evidence, chain_proof, ProgramId, ProgramLifecycle, ProtocolEvidenceError, ReadFreshness,
    UpgradePolicy,
};
use verify_program::current_program_at;

fn key(prefix: &[u8], program: ProgramId) -> Vec<u8> {
    let mut key = prefix.to_vec();
    key.extend_from_slice(&program.bytes());
    chain_evidence::programs_state_key(&key)
}

fn record(policy: UpgradePolicy, code_hash: [u8; 32], version: u32) -> Vec<u8> {
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

fn status(program: ProgramId, lifecycle: u8) -> Vec<u8> {
    let mut value = vec![1, lifecycle];
    value.extend_from_slice(&program.bytes());
    value.extend_from_slice(&1_700_000_000_u64.to_be_bytes());
    value.extend_from_slice(&37_u64.to_be_bytes());
    value.extend_from_slice(&0_u32.to_be_bytes());
    value
}

fn proofs(
    fixture: &ChainFixture,
    program: ProgramId,
    record: Vec<u8>,
    status: Option<Vec<u8>>,
) -> (
    ChainProgramStateProof,
    ChainLifecycleProof,
    ChainWindDownProof,
) {
    let record_ops = fixture.prove(&key(b"program\0", program)).unwrap();
    let status_key = key(b"wind-down\0s", program);
    let status_ops = if status.is_some() {
        fixture.prove(&status_key).unwrap()
    } else {
        fixture.prove_absent(&status_key).unwrap()
    };
    (
        ChainProgramStateProof {
            program,
            ops: record_ops.clone(),
        },
        ChainLifecycleProof {
            program,
            record: Some(record),
            record_ops,
            status: status.clone(),
            status_ops: status_ops.clone(),
        },
        ChainWindDownProof {
            program,
            value: status,
            ops: status_ops,
        },
    )
}

#[test]
fn chain_program_head_reads_record_lifecycle_and_wind_down() {
    let active = ProgramId::new([0x11; 32]).unwrap();
    let deprecated = ProgramId::new([0x22; 32]).unwrap();
    let tombstoned = ProgramId::new([0x33; 32]).unwrap();
    let active_record = record(UpgradePolicy::Immutable, [0x41; 32], 7);
    let policy = UpgradePolicy::Authority([0x51; 32]);
    let deprecated_record = record(policy, [0x42; 32], 8);
    let deprecated_status = status(deprecated, 2);
    let tombstoned_status = status(tombstoned, 3);
    let fixture = ChainFixture::new(
        "paxeer-current-program",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
        vec![
            (key(b"program\0", active), active_record.clone()),
            (key(b"program\0", deprecated), deprecated_record.clone()),
            (key(b"program\0", tombstoned), active_record.clone()),
            (key(b"wind-down\0s", deprecated), deprecated_status.clone()),
            (key(b"wind-down\0s", tombstoned), tombstoned_status.clone()),
        ],
    )
    .unwrap();
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap();
    let time_ms = 1_700_000_000_123;
    let max_age_ms = 5_000;
    let active_proofs = proofs(&fixture, active, active_record.clone(), None);
    let deprecated_proofs = proofs(
        &fixture,
        deprecated,
        deprecated_record,
        Some(deprecated_status.clone()),
    );
    let tombstoned_proofs = proofs(
        &fixture,
        tombstoned,
        active_record.clone(),
        Some(tombstoned_status),
    );
    let verify = |head: &chain_proof::VerifiedChainHead,
                  proofs: &(
        ChainProgramStateProof,
        ChainLifecycleProof,
        ChainWindDownProof,
    ),
                  program,
                  now,
                  max_age| {
        current_program_at(head, &proofs.0, &proofs.1, &proofs.2, program, now, max_age)
    };

    for (program, proofs, version, code_hash, policy, lifecycle, status) in [
        (
            active,
            &active_proofs,
            7,
            [0x41; 32],
            UpgradePolicy::Immutable,
            ProgramLifecycle::Active,
            None,
        ),
        (
            deprecated,
            &deprecated_proofs,
            8,
            [0x42; 32],
            policy,
            ProgramLifecycle::Deprecated,
            Some(deprecated_status.try_into().unwrap()),
        ),
    ] {
        for now in [time_ms - 1_000, time_ms, time_ms + max_age_ms] {
            let verified = verify(&head, proofs, program, now, max_age_ms).unwrap();
            assert_eq!(verified.program(), program);
            assert_eq!(verified.version(), version);
            assert_eq!(verified.code_hash(), code_hash);
            assert_eq!(verified.abi_version(), 1);
            assert_eq!(verified.policy(), policy);
            assert_eq!(verified.lifecycle(), lifecycle);
            assert_eq!(verified.wind_down(), status);
            assert_eq!(verified.valid_until_ms(), time_ms + max_age_ms);
            assert_eq!(verified.state_root(), head.app_hash());
            assert_eq!(
                verified.freshness(),
                ReadFreshness {
                    observed_sequence: (STATE_HEIGHT + 1) as u64,
                    observed_at: time_ms,
                }
            );
        }
    }
    assert_eq!(
        verify(&head, &tombstoned_proofs, tombstoned, time_ms, max_age_ms),
        Err(ProtocolEvidenceError::LifecycleProof)
    );
    assert_eq!(
        verify(&head, &active_proofs, deprecated, time_ms, max_age_ms),
        Err(ProtocolEvidenceError::ProgramRecord)
    );
    for (now, max_age) in [
        (time_ms + max_age_ms + 1, max_age_ms),
        (time_ms - 1_001, max_age_ms),
        (time_ms, u64::MAX),
    ] {
        assert_eq!(
            verify(&head, &active_proofs, active, now, max_age),
            Err(ProtocolEvidenceError::Stale)
        );
    }
    assert_eq!(
        verify(&head, &active_proofs, active, time_ms, 0)
            .unwrap()
            .valid_until_ms(),
        time_ms
    );

    let mut wrong_lifecycle = active_proofs.clone();
    wrong_lifecycle.1 = deprecated_proofs.1.clone();
    let mut wrong_status = active_proofs.clone();
    wrong_status.2 = deprecated_proofs.2.clone();
    for invalid in [wrong_lifecycle, wrong_status] {
        assert_eq!(
            verify(&head, &invalid, active, time_ms, max_age_ms),
            Err(ProtocolEvidenceError::LifecycleProof)
        );
    }

    let mut damaged_record = active_proofs.clone();
    damaged_record.0.ops.clear();
    let mut damaged_lifecycle = active_proofs.clone();
    damaged_lifecycle.1.record_ops.clear();
    let mut damaged_status = active_proofs.clone();
    damaged_status.2.ops.clear();
    let mut hidden_status = deprecated_proofs.clone();
    hidden_status.2.value = None;
    for (invalid, program) in [
        (damaged_record, active),
        (damaged_lifecycle, active),
        (damaged_status, active),
        (hidden_status, deprecated),
    ] {
        assert_eq!(
            verify(&head, &invalid, program, time_ms, max_age_ms),
            Err(ProtocolEvidenceError::StateProof)
        );
    }

    let other = ChainFixture::new(
        "paxeer-current-program",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
        vec![
            (key(b"program\0", active), active_record.clone()),
            (key(b"wind-down\0s", active), status(active, 2)),
        ],
    )
    .unwrap();
    let other_head = other
        .verifier
        .verify_head(&other.header, &other.commit)
        .unwrap();
    assert_ne!(other_head.app_hash(), head.app_hash());
    assert_eq!(
        verify(&other_head, &active_proofs, active, time_ms, max_age_ms),
        Err(ProtocolEvidenceError::StateProof)
    );
    let other_proofs = proofs(&other, active, active_record, Some(status(active, 2)));
    for mixed in [
        (
            other_proofs.0.clone(),
            active_proofs.1.clone(),
            active_proofs.2.clone(),
        ),
        (
            active_proofs.0.clone(),
            other_proofs.1.clone(),
            active_proofs.2.clone(),
        ),
        (
            active_proofs.0.clone(),
            active_proofs.1.clone(),
            other_proofs.2.clone(),
        ),
    ] {
        assert_eq!(
            verify(&head, &mixed, active, time_ms, max_age_ms),
            Err(ProtocolEvidenceError::StateProof)
        );
    }
}
