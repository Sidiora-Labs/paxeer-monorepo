#[path = "support/chain.rs"]
mod chain;
#[path = "../src/hash.rs"]
mod hash;
#[path = "../src/interface.rs"]
mod interface;
#[path = "../src/protocol_evidence.rs"]
mod protocol_evidence;
mod support;
#[path = "../src/chain_evidence/verify_bundle.rs"]
mod verify_bundle;

use chain::{ChainFixture, STATE_HEIGHT};
use interface::{
    interface_state_key, interface_state_value, verify_interface_read, InterfaceEntryPoint,
    InterfaceRefusal, InterfaceStateWitness, ProgramInterface, ValueSchema, ValueType,
    VerifiedInterfaceRead,
};
use layerx_programs as account_state;
use layerx_programs::{
    chain_evidence, chain_proof, AccountStateHead, DeploymentRecord, ProgramId, ProgramLifecycle,
    ReadFreshness, StateProof, UpgradePolicy,
};
use protocol_evidence::{
    ProgramBundleError, ProtocolEvidenceError, VerifiedDeploymentEvidence, VerifiedProgramHead,
};
use verify_bundle::{program_bundle_at, ChainWindDownProof, ProgramStateBundle};

fn program_key(program: ProgramId) -> Vec<u8> {
    let mut key = b"program\0".to_vec();
    key.extend_from_slice(&program.bytes());
    chain_evidence::programs_state_key(&key)
}

fn status_key(program: ProgramId) -> Vec<u8> {
    let mut key = b"wind-down\0s".to_vec();
    key.extend_from_slice(&program.bytes());
    chain_evidence::programs_state_key(&key)
}

fn bundle_fixture(
    status: Option<[u8; 54]>,
) -> (ChainFixture, ProgramStateBundle, ProgramInterface) {
    let program = ProgramId::new([0x31; 32]).expect("program");
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
    .expect("interface bound to the executable module");
    let program_key = program_key(program);
    let interface_key = chain_evidence::programs_state_key(&interface_state_key(program));
    let mut record = vec![0; 33];
    record.extend_from_slice(&interface.code_hash());
    record.extend_from_slice(&interface.abi_version().to_be_bytes());
    record.extend_from_slice(&7_u32.to_be_bytes());
    let mut pairs = vec![
        (program_key.clone(), record),
        (
            interface_key.clone(),
            interface_state_value(program, 7, &interface).expect("interface value"),
        ),
    ];
    if let Some(status) = status {
        pairs.push((status_key(program), status.to_vec()));
    }
    let fixture = ChainFixture::new(
        "paxeer-bundle-test",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
        pairs,
    )
    .expect("signed header and real IAVL and multistore proofs");
    let bundle = ProgramStateBundle {
        commitment: chain_proof::decode::decode_commitment_proof(
            &fixture.encode_key(&program_key).expect("commitment bytes"),
        )
        .expect("commitment"),
        program,
        program_record_ops: fixture.prove(&program_key).expect("program proof"),
        interface_ops: fixture.prove(&interface_key).expect("interface proof"),
        wind_down: Some(ChainWindDownProof {
            program,
            value: status.map(|value| value.to_vec()),
            ops: if status.is_some() {
                fixture
                    .prove(&status_key(program))
                    .expect("status membership")
            } else {
                fixture
                    .prove_absent(&status_key(program))
                    .expect("status absence")
            },
        }),
    };
    (fixture, bundle, interface)
}

#[test]
fn chain_bundle_v2_round_trips_and_verifies() {
    let mut status = [0; 54];
    status[0] = 1;
    status[1] = 2;
    status[2..34].fill(0x31);
    status[34..42].copy_from_slice(&1_800_000_000_u64.to_be_bytes());
    status[42..50].copy_from_slice(&72_u64.to_be_bytes());
    for status in [None, Some(status)] {
        let (fixture, bundle, interface) = bundle_fixture(status);
        for with_wind_down in [true, false] {
            let mut bundle = bundle.clone();
            if !with_wind_down {
                bundle.wind_down = None;
            }
            let bytes = bundle.canonical_encoding().expect("encode version two");
            assert_eq!(&bytes[..5], b"LXPB\x02");
            let decoded = ProgramStateBundle::decode(&bytes).expect("decode version two");
            assert_eq!(decoded, bundle);
            assert_eq!(decoded.canonical_encoding().expect("re-encode"), bytes);
            let verified = program_bundle_at(&fixture.verifier, &decoded, bundle.program)
                .expect("bundle verified at one commitment");
            let head = fixture
                .verifier
                .verify_head(&fixture.header, &fixture.commit)
                .expect("independently verify chain head");
            assert_eq!(verified.chain_head(), &head);
            assert_eq!(verified.program_head().program(), bundle.program);
            assert_eq!(verified.program_head().version(), 7);
            assert_eq!(verified.program_head().code_hash(), interface.code_hash());
            assert_eq!(
                verified.program_head().abi_version(),
                interface.abi_version()
            );
            assert_eq!(
                verified.program_head().upgrade_policy(),
                UpgradePolicy::Immutable
            );
            assert_eq!(verified.program_head().state_root(), head.app_hash());
            assert_eq!(verified.program_head().state_height(), STATE_HEIGHT);
            assert_eq!(verified.program_head().state_height(), head.height() - 1);
            assert_eq!(verified.interface().interface, interface);
            assert_eq!(verified.interface().program, bundle.program);
            assert_eq!(verified.interface().version, 7);
            assert_eq!(verified.interface().state_root, head.app_hash());
            assert_eq!(
                verified.interface().freshness.observed_sequence,
                STATE_HEIGHT as u64
            );
            assert_eq!(verified.wind_down(), with_wind_down.then_some(status));
        }
    }

    let (fixture, bundle, _) = bundle_fixture(None);
    let (_, other, _) = bundle_fixture(Some(status));
    for changed in [
        ProgramStateBundle {
            program_record_ops: other.program_record_ops.clone(),
            ..bundle.clone()
        },
        ProgramStateBundle {
            interface_ops: other.interface_ops.clone(),
            ..bundle.clone()
        },
        ProgramStateBundle {
            wind_down: other.wind_down.clone(),
            ..bundle.clone()
        },
        ProgramStateBundle {
            commitment: other.commitment.clone(),
            ..bundle.clone()
        },
    ] {
        assert!(
            program_bundle_at(&fixture.verifier, &changed, bundle.program).is_err(),
            "proofs from different commitments must never be combined"
        );
    }
    assert_eq!(
        program_bundle_at(
            &fixture.verifier,
            &bundle,
            ProgramId::new([0x32; 32]).unwrap()
        ),
        Err(ProgramBundleError::ProgramMismatch)
    );
    let mut forged = bundle.clone();
    let last = forged.commitment.commit.len() - 1;
    forged.commitment.commit[last] ^= 1;
    assert!(program_bundle_at(&fixture.verifier, &forged, bundle.program).is_err());
    let (foreign, _, _) = bundle_fixture(None);
    let foreign_trust = chain_proof::ChainProofVerifier {
        chain_id: "another-chain".to_owned(),
        validators: foreign.verifier.validators,
    };
    assert!(program_bundle_at(&foreign_trust, &bundle, bundle.program).is_err());
    let mut changed = bundle.clone();
    changed.commitment.value.as_mut().unwrap()[33] ^= 1;
    assert!(program_bundle_at(&fixture.verifier, &changed, bundle.program).is_err());
}

#[test]
fn chain_bundle_refuses_version_1_and_trailing_bytes() {
    let (_, bundle, _) = bundle_fixture(None);
    let canonical = bundle.canonical_encoding().expect("version two bytes");
    assert!(ProgramStateBundle::decode(&canonical).is_ok());
    let refusal = Err(ProgramBundleError::Evidence(
        ProtocolEvidenceError::Encoding,
    ));
    let mut version_one = canonical.clone();
    version_one[4] = 1;
    assert_eq!(ProgramStateBundle::decode(&version_one), refusal);
    let mut trailing = canonical.clone();
    trailing.push(0);
    assert_eq!(ProgramStateBundle::decode(&trailing), refusal);
    assert_eq!(
        ProgramStateBundle::decode(b"LayerX/programs/state-proof/v1\0"),
        refusal
    );
    for length in 0..canonical.len() {
        assert_eq!(
            ProgramStateBundle::decode(&canonical[..length]),
            refusal,
            "truncated bundle at byte {length}"
        );
    }
    let mut oversized = canonical.clone();
    oversized[5..9].copy_from_slice(&u32::MAX.to_be_bytes());
    assert_eq!(ProgramStateBundle::decode(&oversized), refusal);
    let mut without_wind_down = bundle;
    without_wind_down.wind_down = None;
    let option_offset = without_wind_down
        .canonical_encoding()
        .expect("option location")
        .len()
        - 1;
    let mut bad_option = canonical.clone();
    bad_option[option_offset] = 2;
    assert_eq!(ProgramStateBundle::decode(&bad_option), refusal);
    let mut bad_option = without_wind_down
        .canonical_encoding()
        .expect("optional status omitted");
    *bad_option.last_mut().unwrap() = 2;
    assert_eq!(ProgramStateBundle::decode(&bad_option), refusal);
}
