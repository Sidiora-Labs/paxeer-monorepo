#[path = "support/chain.rs"]
mod chain;

use chain::ChainFixture;
use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::{
    chain_evidence::{
        self, deployment::ChainDeploymentProof, lifecycle::ChainLifecycleProof,
        program_state::ChainProgramStateProof, wind_down::ChainWindDownProof,
    },
    chain_proof::{
        decode::decode_commitment_proof,
        merkle::{hash_from_byte_slices, SimpleProof},
        validators::{TrustedValidator, ValidatorSet},
        vote::{vote_sign_bytes, BlockIdFlag, CommitSig, Timestamp},
    },
    interface_state_key, interface_state_value, DeploymentProof, InterfaceEntryPoint,
    ProgramHeadKind, ProgramId, ProgramInterface, ProgramLifecycle, ProgramLifecycleProof,
    ProgramStateBundle, ProgramStateProof, ProtocolDeploymentVerifier, ProtocolEvidenceError,
    Registry, RegistryError, UpgradePolicy, ValueSchema, ValueType,
};
use sha2::{Digest, Sha256};

const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const ROTATED_SEEDS: [[u8; 32]; 4] = [[11; 32], [12; 32], [13; 32], [14; 32]];
const MAX_AGE_MS: u64 = 5_000;
const WASM: &[u8] = &[
    0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127, 3, 3, 2, 0,
    1, 5, 3, 1, 0, 1, 7, 34, 3, 4, b'c', b'a', b'l', b'l', 0, 0, 14, b'l', b'a', b'y', b'e', b'r',
    b'x', b'_', b'r', b'e', b's', b'e', b'r', b'v', b'e', 0, 1, 6, b'm', b'e', b'm', b'o', b'r',
    b'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
];

fn state_key(prefix: &[u8], program: ProgramId) -> Vec<u8> {
    let mut key = prefix.to_vec();
    key.extend_from_slice(&program.bytes());
    chain_evidence::programs_state_key(&key)
}

fn sign(fixture: &mut ChainFixture, seeds: &[[u8; 32]]) {
    fixture.commit.height = fixture.header.height;
    fixture.commit.block_id.hash = fixture.header.hash().expect("header hash").to_vec();
    let timestamp = Timestamp {
        seconds: fixture.header.time.seconds,
        nanos: fixture.header.time.nanos,
    };
    let bytes = vote_sign_bytes(
        &fixture.header.chain_id,
        fixture.commit.height,
        fixture.commit.round,
        Some(&fixture.commit.block_id),
        timestamp,
    )
    .expect("canonical vote bytes");
    let mut keys = seeds.iter().map(SigningKey::from_bytes).collect::<Vec<_>>();
    keys.sort_by_key(|key| TrustedValidator::new(key.verifying_key().to_bytes(), 1).address);
    fixture.commit.signatures = keys
        .iter()
        .map(|key| CommitSig {
            flag: BlockIdFlag::Commit,
            validator_address: TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                .address
                .to_vec(),
            timestamp,
            signature: key.sign(&bytes).to_bytes().to_vec(),
        })
        .collect();
}

fn deployment_fixture() -> (
    ChainFixture,
    DeploymentProof,
    ProgramStateBundle,
    ProgramInterface,
) {
    let program = ProgramId::new([0x31; 32]).expect("program identifier");
    let interface = ProgramInterface::bind(
        WASM,
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
    .expect("interface bound to executable module");
    let program_key = state_key(b"program\0", program);
    let interface_key = chain_evidence::programs_state_key(&interface_state_key(program));
    let mut record = vec![0; 33];
    record.extend_from_slice(&interface.code_hash());
    record.extend_from_slice(&interface.abi_version().to_be_bytes());
    record.extend_from_slice(&1_u32.to_be_bytes());
    let mut fixture = ChainFixture::new(
        "paxeer-program-end-to-end",
        &SEEDS,
        vec![
            (program_key.clone(), record.clone()),
            (
                interface_key.clone(),
                interface_state_value(program, 1, &interface).expect("interface state value"),
            ),
        ],
    )
    .expect("signed chain fixture with IAVL and multistore proofs");
    let transaction = b"chain program deployment".to_vec();
    let transaction_hash: [u8; 32] = Sha256::digest(&transaction).into();
    let transaction_root = hash_from_byte_slices(&[&transaction_hash]);
    fixture.header.data_hash = transaction_root.to_vec();
    sign(&mut fixture, &SEEDS);
    let commitment = decode_commitment_proof(
        &fixture
            .encode_key(&program_key)
            .expect("encoded commitment"),
    )
    .expect("decoded commitment");
    let state = ChainProgramStateProof {
        program,
        ops: fixture.prove(&program_key).expect("program membership"),
    };
    let wind_down = ChainWindDownProof {
        program,
        value: None,
        ops: fixture
            .prove_absent(&state_key(b"wind-down\0s", program))
            .expect("active program has authenticated absent wind-down status"),
    };
    let proof = DeploymentProof {
        state: ProgramStateProof {
            commitment: commitment.clone(),
            state: state.clone(),
            lifecycle: ProgramLifecycleProof(ChainLifecycleProof {
                program,
                record: Some(record),
                record_ops: state.ops.clone(),
                status: None,
                status_ops: wind_down.ops.clone(),
            }),
            wind_down: wind_down.clone(),
            transaction,
            transaction_proof: SimpleProof {
                total: 1,
                index: 0,
                leaf_hash: transaction_root,
                aunts: Vec::new(),
            },
        },
        deployment: ChainDeploymentProof {
            program_state: state.clone(),
            code: WASM.to_vec(),
            interface: Some(interface.canonical_encoding().to_vec()),
        },
    };
    let bundle = ProgramStateBundle {
        commitment,
        program,
        program_record_ops: state.ops,
        interface_ops: fixture.prove(&interface_key).expect("interface membership"),
        wind_down: Some(wind_down),
    };
    (fixture, proof, bundle, interface)
}

#[test]
fn chain_bundle_records_deployment_and_survives_rotation() {
    let (mut fixture, proof, bundle, interface) = deployment_fixture();
    let program = bundle.program;
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        &fixture.verifier.chain_id,
        fixture.verifier.validators.validators().to_vec(),
        MAX_AGE_MS,
    )
    .expect("protocol verifier anchored to original chain validators");
    let original = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .expect("original signed header");
    let now = u64::try_from(original.time_ms()).expect("original time");
    let chain = verifier
        .verify_current_chain_head(
            ProgramHeadKind::Ordinary,
            &proof.state.commitment,
            &proof.state.transaction,
            &proof.state.transaction_proof,
            now,
        )
        .expect("transaction included under authenticated chain head");
    let encoded = bundle.canonical_encoding().expect("version two bundle");
    assert_eq!(&encoded[..5], b"LXPB\x02");
    let decoded = ProgramStateBundle::decode(&encoded).expect("decode version two bundle");
    assert_eq!(decoded, bundle);
    let verified = verifier
        .verify_current_program_bundle(&decoded, &chain, program, now)
        .expect("program, interface and wind-down proven at the same chain head");
    assert_eq!(verified.chain_head(), &chain);
    assert_eq!(verified.program_head().program(), program);
    assert_eq!(verified.program_head().version(), 1);
    assert_eq!(verified.program_head().code_hash(), interface.code_hash());
    assert_eq!(
        verified.program_head().abi_version(),
        interface.abi_version()
    );
    assert_eq!(
        verified.program_head().lifecycle(),
        ProgramLifecycle::Active
    );
    assert_eq!(verified.program_head().policy(), UpgradePolicy::Immutable);
    assert_eq!(verified.program_head().state_root(), original.app_hash());
    assert_eq!(verified.interface().interface, interface);
    assert_eq!(verified.interface().program, program);
    assert_eq!(verified.interface().version, 1);
    assert_eq!(verified.interface().state_root, original.app_hash());
    assert_eq!(
        decoded
            .wind_down
            .as_ref()
            .expect("wind-down proof")
            .verify(&original),
        Ok(None)
    );

    let deployment = verifier
        .verify_deployment(&proof, now)
        .expect("deployment evidence from the same authenticated state and transaction");
    assert_eq!(deployment.program(), verified.program_head().program());
    assert_eq!(deployment.version(), verified.program_head().version());
    assert_eq!(deployment.code_hash(), verified.program_head().code_hash());
    assert_eq!(
        deployment.state_root(),
        verified.program_head().state_root()
    );
    assert_eq!(deployment.receipt_digest(), chain.receipt_digest());
    assert_eq!(
        deployment.interface(),
        Some(&verified.interface().interface)
    );
    assert_eq!(deployment.module(), WASM);
    let mut registry = Registry::new();
    assert_eq!(
        registry.resolve_deployment(deployment.clone()),
        Err(RegistryError::UnknownProgram)
    );
    registry
        .record_verified_deployment(&deployment)
        .expect("record chain-verified initial deployment");
    let resolved = registry
        .resolve_deployment(deployment.clone())
        .expect("resolve the registered program code hash");
    assert_eq!(resolved.code_hash(), interface.code_hash());
    assert_eq!(registry.latest_version(program), Ok(1));

    let mut rotated = ROTATED_SEEDS
        .iter()
        .map(|seed| {
            let key = SigningKey::from_bytes(seed);
            TrustedValidator::new(key.verifying_key().to_bytes(), 1)
        })
        .collect::<Vec<_>>();
    rotated.sort_by_key(|validator| validator.address);
    let rotated = ValidatorSet::new(rotated).expect("rotated validator set");
    assert!(rotated.validators().iter().all(|validator| !fixture
        .verifier
        .validators
        .validators()
        .contains(validator)));
    let mut history = vec![(
        fixture.header.clone(),
        fixture.commit.clone(),
        fixture.verifier.validators.clone(),
    )];
    for offset in 1..=20 {
        fixture.header.last_block_id.hash = fixture.commit.block_id.hash.clone();
        fixture.header.last_block_id.part_set_total = fixture.commit.block_id.part_set_header.total;
        fixture.header.last_block_id.part_set_hash =
            fixture.commit.block_id.part_set_header.hash.clone();
        fixture.header.height += 1;
        fixture.header.time.seconds += 1;
        if offset == 10 {
            fixture.header.next_validators_hash = rotated.hash().to_vec();
        }
        if offset == 11 {
            fixture.verifier.validators = rotated.clone();
            fixture.header.validators_hash = rotated.hash().to_vec();
            fixture.header.proposer_address = rotated.validators()[0].address.to_vec();
        }
        sign(
            &mut fixture,
            if offset <= 10 { &SEEDS } else { &ROTATED_SEEDS },
        );
        history.push((
            fixture.header.clone(),
            fixture.commit.clone(),
            fixture.verifier.validators.clone(),
        ));
    }
    assert_eq!(history.len(), 21);
    assert_eq!(
        history
            .windows(2)
            .filter(|pair| pair[0].2 != pair[1].2)
            .count(),
        1
    );
    let current = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .expect("current head signed by rotated validators");
    assert_eq!(current.height(), original.height() + 20);
    assert_eq!(current.time_ms(), original.time_ms() + 20_000);
    let later = u64::try_from(current.time_ms()).expect("current time");
    assert_eq!(
        verifier.verify_deployment(&proof, later),
        Err(ProtocolEvidenceError::Stale)
    );
    let historical = verifier
        .verify_historical_deployment(&proof, &current, &history, later)
        .expect("original deployment survives twenty heights and authenticated rotation");
    assert_eq!(historical, deployment);
    assert_eq!(
        registry
            .resolve_deployment(historical)
            .expect("resolve historical deployment")
            .code_hash(),
        resolved.code_hash()
    );
    let mut missing_rotation = history.clone();
    missing_rotation.remove(10);
    assert_eq!(
        verifier.verify_historical_deployment(&proof, &current, &missing_rotation, later),
        Err(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
    let mut forged = history;
    forged[10].1.signatures[0].signature[0] ^= 1;
    assert_eq!(
        verifier.verify_historical_deployment(&proof, &current, &forged, later),
        Err(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
}
