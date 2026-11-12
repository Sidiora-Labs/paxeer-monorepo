#[path = "support/assertions.rs"]
mod assertions;
use assertions::TestResultExt;

use support::chain;
#[allow(dead_code)]
#[path = "support/mod.rs"]
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
    .or_panic("bind actual callable module")
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
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "build actual IAVL and multistore trees and sign the header"
        )
    });
    let mut ops = fixture.prove(&chain_key).or_panic("interface proof");
    ops.extend(fixture.prove(&record_key).or_panic("program-record proof"));
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

fn verify_mismatched_interface_states(
    program: ProgramId,
    interface: &ProgramInterface,
    value: &[u8],
    record: &[u8],
    witness: &InterfaceStateWitness,
    ops: &[ProofOp],
    head: &layerx_programs::chain_proof::VerifiedChainHead,
) {
    let other = ProgramId::new([0x32; 32]).or_panic("program B");
    let other_interface = self::interface(support::WASM_V2);
    assert_ne!(other_interface.code_hash(), interface.code_hash());
    let mismatched_code =
        interface_state_value(program, 1, &other_interface).unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "canonical interface bound to another real module"
            )
        });
    let mismatched_version = interface_state_value(program, 2, interface).or_panic("version two");
    let mismatched_program = interface_state_value(other, 1, interface).or_panic("other program");
    let mut mismatched_digest = value.to_vec();
    mismatched_digest[36] ^= 1;
    let mut mismatched_abi = record.to_vec();
    mismatched_abi[65..67].copy_from_slice(&2_u16.to_be_bytes());
    let mut invalid_policy = record.to_vec();
    invalid_policy[0] = 1;
    let mut invalid_version = record.to_vec();
    invalid_version[67..71].fill(0);
    let mut trailing_record = record.to_vec();
    trailing_record.push(0);
    for (value, record, refusal) in [
        (
            mismatched_code,
            record.to_vec(),
            InterfaceRefusal::CodeHashMismatch,
        ),
        (
            mismatched_digest,
            record.to_vec(),
            InterfaceRefusal::CodeHashMismatch,
        ),
        (
            mismatched_version,
            record.to_vec(),
            InterfaceRefusal::VersionMismatch,
        ),
        (
            mismatched_program,
            record.to_vec(),
            InterfaceRefusal::ProgramMismatch,
        ),
        (
            value.to_vec(),
            mismatched_abi,
            InterfaceRefusal::CodeHashMismatch,
        ),
        (value.to_vec(), invalid_policy, InterfaceRefusal::Invalid),
        (
            value.to_vec(),
            invalid_version,
            InterfaceRefusal::VersionMismatch,
        ),
        (value.to_vec(), trailing_record, InterfaceRefusal::Invalid),
    ] {
        let (changed_fixture, changed_witness, changed_ops) = proven(program, value, record);
        let changed_head = changed_fixture
            .verifier
            .verify_head(&changed_fixture.header, &changed_fixture.commit)
            .unwrap_or_else(|error| {
                panic!(
                    "{}: {error:?}",
                    "verify head committing the mismatched state"
                )
            });
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
            VerifiedInterfaceRead::verify_at_head(&changed_head, program, witness, ops),
            Err(InterfaceRefusal::StateProof)
        );
        assert_eq!(
            VerifiedInterfaceRead::verify_at_head(head, program, &changed_witness, &changed_ops),
            Err(InterfaceRefusal::StateProof)
        );
    }
}

#[test]
fn chain_interface_read_verifies_at_head() {
    let program = ProgramId::new([0x31; 32]).or_panic("program A");
    let other = ProgramId::new([0x32; 32]).or_panic("program B");
    let interface = interface(support::WASM_V1);
    let value = interface_state_value(program, 1, &interface).or_panic("interface value");
    let record = program_record(&interface, 1);
    let (fixture, witness, ops) = proven(program, value.clone(), record.clone());
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .or_panic("verify the signed chain head");
    let read: VerifiedInterfaceRead =
        VerifiedInterfaceRead::verify_at_head(&head, program, &witness, &ops)
            .or_panic("interface proven at the verified head");
    assert_eq!(read.interface, interface);
    assert_eq!(read.program, program);
    assert_eq!(read.version, 1);
    assert_eq!(read.state_root, head.app_hash());
    assert_eq!(read.receipt_digest, head.app_hash());
    assert_eq!(
        read.freshness,
        ReadFreshness {
            observed_sequence: STATE_HEIGHT as u64,
            observed_at: u64::from_le_bytes((head.time_ms()).to_le_bytes()),
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

    verify_mismatched_interface_states(program, &interface, &value, &record, &witness, &ops, &head);
}
