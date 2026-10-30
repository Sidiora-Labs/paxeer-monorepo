#[path = "support/chain.rs"]
mod chain;
#[path = "../src/chain_evidence/wind_down.rs"]
mod wind_down;

use chain::ChainFixture;
use layerx_programs::{chain_evidence, chain_proof, ProgramId, ProtocolEvidenceError};
use wind_down::ChainWindDownProof;

fn status_key(program: ProgramId) -> Vec<u8> {
    let mut key = b"wind-down\0s".to_vec();
    key.extend_from_slice(&program.bytes());
    chain_evidence::programs_state_key(&key)
}

fn status_record(program: ProgramId, lifecycle: u8) -> [u8; 54] {
    let mut value = [0; 54];
    value[0] = 1;
    value[1] = lifecycle;
    value[2..34].copy_from_slice(&program.bytes());
    value[34..42].copy_from_slice(&1_800_000_000_u64.to_be_bytes());
    value[42..50].copy_from_slice(&72_u64.to_be_bytes());
    value
}

#[test]
fn chain_wind_down_reads_status_or_absence() -> Result<(), String> {
    let program = |byte| ProgramId::new([byte; 32]).map_err(|error| format!("{error:?}"));
    let a = program(0x11)?;
    let b = program(0x22)?;
    let c = program(0x33)?;
    let status_a = status_record(a, 2);
    let status_c = status_record(c, 3);
    let mut raw_key = b"wind-down\0s".to_vec();
    raw_key.extend_from_slice(&a.bytes());
    let fixture = ChainFixture::new(
        "paxeer-wind-down-test",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
        vec![
            (status_key(a), status_a.to_vec()),
            (status_key(c), status_c.to_vec()),
            (raw_key.clone(), status_a.to_vec()),
        ],
    )
    .map_err(|error| format!("fixture: {error:?}"))?;
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .map_err(|error| format!("head: {error:?}"))?;

    for (program, status) in [(a, status_a), (c, status_c)] {
        let proof = ChainWindDownProof {
            program,
            value: Some(status.to_vec()),
            ops: fixture
                .prove(&status_key(program))
                .map_err(|error| format!("membership: {error:?}"))?,
        };
        assert_eq!(proof.verify(&head), Ok(Some(status)));
    }

    let present = ChainWindDownProof {
        program: a,
        value: Some(status_a.to_vec()),
        ops: fixture
            .prove(&status_key(a))
            .map_err(|error| format!("membership: {error:?}"))?,
    };
    for program in [program(1)?, b, program(0x44)?] {
        let absent = ChainWindDownProof {
            program,
            value: None,
            ops: fixture
                .prove_absent(&status_key(program))
                .map_err(|error| format!("absence: {error:?}"))?,
        };
        assert_eq!(absent.verify(&head), Ok(None));
        let claimed_present = ChainWindDownProof {
            value: Some(status_record(program, 2).to_vec()),
            ..absent
        };
        assert_eq!(
            claimed_present.verify(&head),
            Err(ProtocolEvidenceError::StateProof)
        );
    }

    let mut altered = status_a;
    altered[34] ^= 1;
    for invalid in [
        ChainWindDownProof {
            value: Some(altered.to_vec()),
            ..present.clone()
        },
        ChainWindDownProof {
            program: b,
            ..present.clone()
        },
        ChainWindDownProof {
            value: None,
            ..present.clone()
        },
        ChainWindDownProof {
            ops: Vec::new(),
            ..present.clone()
        },
        ChainWindDownProof {
            ops: fixture
                .prove(&raw_key)
                .map_err(|error| format!("unwrapped key: {error:?}"))?,
            ..present.clone()
        },
    ] {
        assert_eq!(
            invalid.verify(&head),
            Err(ProtocolEvidenceError::StateProof)
        );
    }

    let mut wrong_store = present.clone();
    wrong_store.ops[1].key = b"bank".to_vec();
    assert_eq!(
        wrong_store.verify(&head),
        Err(ProtocolEvidenceError::StateProof)
    );
    let mut damaged_ops = present.clone();
    damaged_ops.ops[0].data[0] ^= 1;
    assert_eq!(
        damaged_ops.verify(&head),
        Err(ProtocolEvidenceError::StateProof)
    );

    let other = ChainFixture::new(
        "paxeer-wind-down-test",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
        vec![(status_key(a), altered.to_vec())],
    )
    .map_err(|error| format!("other fixture: {error:?}"))?;
    let other_head = other
        .verifier
        .verify_head(&other.header, &other.commit)
        .map_err(|error| format!("other head: {error:?}"))?;
    assert_ne!(head.app_hash(), other_head.app_hash());
    assert_eq!(
        present.verify(&other_head),
        Err(ProtocolEvidenceError::StateProof)
    );

    for length in [1, 53, 55] {
        let value = vec![1; length];
        let malformed = ChainFixture::new(
            "paxeer-wind-down-test",
            &[[1; 32], [2; 32], [3; 32], [4; 32]],
            vec![(status_key(a), value.clone())],
        )
        .map_err(|error| format!("malformed fixture: {error:?}"))?;
        let malformed_head = malformed
            .verifier
            .verify_head(&malformed.header, &malformed.commit)
            .map_err(|error| format!("malformed head: {error:?}"))?;
        let proof = ChainWindDownProof {
            program: a,
            value: Some(value),
            ops: malformed
                .prove(&status_key(a))
                .map_err(|error| format!("malformed membership: {error:?}"))?,
        };
        assert_eq!(
            proof.verify(&malformed_head),
            Err(ProtocolEvidenceError::Encoding)
        );
    }
    let empty = ChainWindDownProof {
        value: Some(Vec::new()),
        ..present
    };
    assert_eq!(empty.verify(&head), Err(ProtocolEvidenceError::Encoding));
    Ok(())
}
