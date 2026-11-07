#[path = "support/chain.rs"]
mod chain;
mod support;

use chain::{ChainFixture, STATE_HEIGHT};
use layerx_programs::{
    chain_evidence::programs_state_key, chain_proof::decode::ProofOp, interface_state_key,
    interface_state_value, InterfaceEntryPoint, InterfaceRefusal, InterfaceStateWitness, ProgramId,
    ProgramInterface, ReadFreshness, StateProof, ValueSchema, ValueType, VerifiedInterfaceRead,
};

fn interface(module: &[u8]) -> ProgramInterface {
    ProgramInterface::bind(
        module,
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
    .expect("bind actual callable module")
}

fn program_key(program: ProgramId) -> Vec<u8> {
    let mut key = b"program\0".to_vec();
    key.extend_from_slice(&program.bytes());
    programs_state_key(&key)
}

fn program_record(interface: &ProgramInterface, version: u32) -> Vec<u8> {
    let mut record = vec![0; 33];
    record.extend_from_slice(&interface.code_hash());
    record.extend_from_slice(&interface.abi_version().to_be_bytes());
    record.extend_from_slice(&version.to_be_bytes());
    record
}

fn proven(
    program: ProgramId,
    value: Vec<u8>,
    record: Vec<u8>,
) -> (ChainFixture, InterfaceStateWitness, Vec<ProofOp>) {
    let key = interface_state_key(program);
    let chain_key = programs_state_key(&key);
    let record_key = program_key(program);
    let fixture = ChainFixture::new(
        "paxeer-interface-test",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
        vec![
            (chain_key.clone(), value.clone()),
            (record_key.clone(), record),
        ],
    )
    .expect("build actual IAVL and multistore trees and sign the header");
    let mut ops = fixture.prove(&chain_key).expect("interface proof");
    ops.extend(fixture.prove(&record_key).expect("program-record proof"));
    (
        fixture,
        InterfaceStateWitness {
            key,
            value,
            proof: StateProof {
                leaf_index: 0,
                leaf_count: 1,
                siblings: Vec::new(),
            },
        },
        ops,
    )
}

#[test]
fn chain_interface_read_verifies_at_head() {
    let program = ProgramId::new([0x31; 32]).expect("program A");
    let other = ProgramId::new([0x32; 32]).expect("program B");
    let interface = interface(support::WASM_V1);
    let value = interface_state_value(program, 1, &interface).expect("interface value");
    let record = program_record(&interface, 1);
    let (fixture, witness, ops) = proven(program, value.clone(), record.clone());
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .expect("verify the signed chain head");
    let read: VerifiedInterfaceRead =
        VerifiedInterfaceRead::verify_at_head(&head, program, &witness, &ops)
            .expect("interface proven at the verified head");
    assert_eq!(read.interface, interface);
    assert_eq!(read.program, program);
    assert_eq!(read.version, 1);
    assert_eq!(read.state_root, head.app_hash());
    assert_eq!(read.receipt_digest, head.app_hash());
    assert_eq!(
        read.freshness,
        ReadFreshness {
            observed_sequence: STATE_HEIGHT as u64,
            observed_at: head.time_ms() as u64,
        }
    );
    assert_eq!(
        VerifiedInterfaceRead::verify_at_head(&head, other, &witness, &ops),
        Err(InterfaceRefusal::ProgramMismatch)
    );

    let mut changed = witness.clone();
    changed.value[0] ^= 1;
    assert_eq!(
        VerifiedInterfaceRead::verify_at_head(&head, program, &changed, &ops),
        Err(InterfaceRefusal::StateProof)
    );
    for count in 0..ops.len() {
        assert_eq!(
            VerifiedInterfaceRead::verify_at_head(&head, program, &witness, &ops[..count]),
            Err(InterfaceRefusal::StateProof)
        );
    }
    let mut extra = ops.clone();
    extra.push(ops[0].clone());
    assert_eq!(
        VerifiedInterfaceRead::verify_at_head(&head, program, &witness, &extra),
        Err(InterfaceRefusal::StateProof)
    );
    for index in 0..ops.len() {
        let mut changed = ops.clone();
        changed[index].key.push(0);
        assert_eq!(
            VerifiedInterfaceRead::verify_at_head(&head, program, &witness, &changed),
            Err(InterfaceRefusal::StateProof)
        );
    }

    let other_interface = self::interface(support::WASM_V2);
    assert_ne!(other_interface.code_hash(), interface.code_hash());
    let mismatched_code = interface_state_value(program, 1, &other_interface)
        .expect("canonical interface bound to another real module");
    let mismatched_version = interface_state_value(program, 2, &interface).expect("version two");
    let mismatched_program = interface_state_value(other, 1, &interface).expect("other program");
    let mut mismatched_digest = value.clone();
    mismatched_digest[36] ^= 1;
    let mut mismatched_abi = record.clone();
    mismatched_abi[65..67].copy_from_slice(&2_u16.to_be_bytes());
    let mut invalid_policy = record.clone();
    invalid_policy[0] = 1;
    let mut invalid_version = record.clone();
    invalid_version[67..71].fill(0);
    let mut trailing_record = record.clone();
    trailing_record.push(0);
    for (value, record, refusal) in [
        (
            mismatched_code,
            record.clone(),
            InterfaceRefusal::CodeHashMismatch,
        ),
        (
            mismatched_digest,
            record.clone(),
            InterfaceRefusal::CodeHashMismatch,
        ),
        (
            mismatched_version,
            record.clone(),
            InterfaceRefusal::VersionMismatch,
        ),
        (
            mismatched_program,
            record.clone(),
            InterfaceRefusal::ProgramMismatch,
        ),
        (
            value.clone(),
            mismatched_abi,
            InterfaceRefusal::CodeHashMismatch,
        ),
        (value.clone(), invalid_policy, InterfaceRefusal::Invalid),
        (
            value.clone(),
            invalid_version,
            InterfaceRefusal::VersionMismatch,
        ),
        (value.clone(), trailing_record, InterfaceRefusal::Invalid),
    ] {
        let (changed_fixture, changed_witness, changed_ops) = proven(program, value, record);
        let changed_head = changed_fixture
            .verifier
            .verify_head(&changed_fixture.header, &changed_fixture.commit)
            .expect("verify head committing the mismatched state");
        assert_eq!(
            VerifiedInterfaceRead::verify_at_head(
                &changed_head,
                program,
                &changed_witness,
                &changed_ops,
            ),
            Err(refusal)
        );
        assert_eq!(
            VerifiedInterfaceRead::verify_at_head(&changed_head, program, &witness, &ops),
            Err(InterfaceRefusal::StateProof)
        );
        assert_eq!(
            VerifiedInterfaceRead::verify_at_head(&head, program, &changed_witness, &changed_ops),
            Err(InterfaceRefusal::StateProof)
        );
    }
}
