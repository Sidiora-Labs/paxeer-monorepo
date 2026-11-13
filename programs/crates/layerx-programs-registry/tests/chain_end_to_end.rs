#[allow(dead_code)]
#[path = "support/chain.rs"]
mod chain;

#[path = "support/assertions.rs"]
mod assertions;
use assertions::TestResultExt;

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
    Registry, RegistryError, UpgradePolicy, ValueSchema, ValueType, VerifiedDeploymentEvidence,
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
    fixture.commit.block_id.hash = fixture.header.hash().or_panic("header hash").to_vec();
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
    .or_panic("canonical vote bytes");
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
    let program = ProgramId::new([0x31; 32]).or_panic("program identifier");
    deployment_fixture_at(
        program,
        1,
        UpgradePolicy::Immutable,
        WASM,
        "paxeer-program-end-to-end",
        chain::STATE_HEIGHT + 1,
    )
}

fn interface_for_code(code: &[u8]) -> ProgramInterface {
    ProgramInterface::bind(
        code,
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
    .or_panic("interface bound to executable module")
}

fn deployment_fixture_at(
    program: ProgramId,
    version: u32,
    policy: UpgradePolicy,
    code: &[u8],
    chain_id: &str,
    height: i64,
) -> (
    ChainFixture,
    DeploymentProof,
    ProgramStateBundle,
    ProgramInterface,
) {
    let interface = interface_for_code(code);
    let program_key = state_key(b"program\0", program);
    let interface_key = chain_evidence::programs_state_key(&interface_state_key(program));
    let mut record = vec![0; 33];
    if let UpgradePolicy::Authority(authority) = policy {
        record[0] = 1;
        record[1..33].copy_from_slice(&authority);
    }
    record.extend_from_slice(&interface.code_hash());
    record.extend_from_slice(&interface.abi_version().to_be_bytes());
    record.extend_from_slice(&version.to_be_bytes());
    let mut fixture = ChainFixture::new(
        chain_id,
        &SEEDS,
        vec![
            (program_key.clone(), record.clone()),
            (
                interface_key.clone(),
                interface_state_value(program, version, &interface)
                    .or_panic("interface state value"),
            ),
        ],
    )
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "signed chain fixture with IAVL and multistore proofs"
        )
    });
    fixture.header.time.seconds += height - fixture.header.height;
    fixture.header.height = height;
    let transaction = b"chain program deployment".to_vec();
    let transaction_hash: [u8; 32] = Sha256::digest(&transaction).into();
    let transaction_root = hash_from_byte_slices(&[&transaction_hash]);
    fixture.header.data_hash = transaction_root.to_vec();
    sign(&mut fixture, &SEEDS);
    let commitment = decode_commitment_proof(
        &fixture
            .encode_key(&program_key)
            .or_panic("encoded commitment"),
    )
    .or_panic("decoded commitment");
    let state = ChainProgramStateProof {
        program,
        ops: fixture.prove(&program_key).or_panic("program membership"),
    };
    let wind_down = ChainWindDownProof {
        program,
        value: None,
        ops: fixture
            .prove_absent(&state_key(b"wind-down\0s", program))
            .unwrap_or_else(|error| {
                panic!(
                    "{}: {error:?}",
                    "active program has authenticated absent wind-down status"
                )
            }),
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
            code: code.to_vec(),
            interface: Some(interface.canonical_encoding().to_vec()),
        },
    };
    let bundle = ProgramStateBundle {
        commitment,
        program,
        program_record_ops: state.ops,
        interface_ops: fixture
            .prove(&interface_key)
            .or_panic("interface membership"),
        wind_down: Some(wind_down),
    };
    (fixture, proof, bundle, interface)
}

fn upgrade_evidence(
    program: ProgramId,
    version: u32,
    code: &[u8],
    chain_id: &str,
    height: i64,
) -> VerifiedDeploymentEvidence {
    let (fixture, proof, bundle, interface) = deployment_fixture_at(
        program,
        version,
        UpgradePolicy::Authority([0x41; 32]),
        code,
        chain_id,
        height,
    );
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        &fixture.verifier.chain_id,
        fixture.verifier.validators.validators().to_vec(),
        MAX_AGE_MS,
    )
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "protocol verifier anchored to chain validators"
        )
    });
    let header = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .or_panic("authenticated deployment header");
    let now = u64::try_from(header.time_ms()).or_panic("header time");
    let head = verifier
        .verify_current_chain_head(
            ProgramHeadKind::Ordinary,
            &proof.state.commitment,
            &proof.state.transaction,
            &proof.state.transaction_proof,
            now,
        )
        .or_panic("authenticated transaction inclusion");
    let encoded = bundle
        .canonical_encoding()
        .or_panic("encode deployment bundle");
    let decoded = ProgramStateBundle::decode(&encoded).or_panic("decode deployment bundle");
    let authenticated_bundle = verifier
        .verify_current_program_bundle(&decoded, &head, program, now)
        .or_panic("authenticate program bundle");
    let evidence = verifier
        .verify_deployment(&proof, now)
        .or_panic("authenticate deployment evidence");
    assert_eq!(
        evidence.program(),
        authenticated_bundle.program_head().program()
    );
    assert_eq!(evidence.version(), version);
    assert_eq!(evidence.code_hash(), interface.code_hash());
    assert_eq!(
        evidence.state_root(),
        authenticated_bundle.program_head().state_root()
    );
    assert_eq!(evidence.record().old_code_hash, None);
    evidence
}

#[test]
fn chain_bundle_appends_a_verified_upgrade() {
    let program = ProgramId::new([0x31; 32]).or_panic("program identifier");
    let chain_id = "paxeer-program-end-to-end";
    let height = chain::STATE_HEIGHT + 1;
    let mut upgraded_code = WASM.to_vec();
    let constant = upgraded_code.len() - 2;
    upgraded_code[constant] = 1;
    let predecessor = upgrade_evidence(program, 1, WASM, chain_id, height);
    let upgrade = upgrade_evidence(program, 2, &upgraded_code, chain_id, height + 1);
    assert_ne!(predecessor.code_hash(), upgrade.code_hash());
    let mut registry = Registry::new();
    registry
        .record_verified_deployment(&predecessor)
        .or_panic("record authenticated version one");
    assert_eq!(registry.latest_version(program), Ok(1));
    assert_eq!(
        registry.record_verified_deployment(&upgrade),
        Err(RegistryError::VersionHistoryMismatch)
    );
    let wrong_hash = upgrade_evidence(program, 1, &upgraded_code, chain_id, height);
    let other_program = upgrade_evidence(
        ProgramId::new([0x32; 32]).or_panic("different program identifier"),
        1,
        WASM,
        chain_id,
        height,
    );
    let later_predecessor = upgrade_evidence(program, 1, WASM, chain_id, height + 2);
    let same_height = upgrade_evidence(program, 1, WASM, chain_id, height + 1);
    let other_chain = upgrade_evidence(program, 1, WASM, "another-chain", height);
    for invalid in [
        wrong_hash,
        other_program,
        later_predecessor,
        same_height,
        other_chain,
    ] {
        assert_eq!(
            registry.record_verified_upgrade(&invalid, &upgrade),
            Err(RegistryError::VersionHistoryMismatch)
        );
        assert_eq!(registry.latest_version(program), Ok(1));
    }
    let skipped_version = upgrade_evidence(program, 3, &upgraded_code, chain_id, height + 1);
    assert_eq!(
        registry.record_verified_upgrade(&predecessor, &skipped_version),
        Err(RegistryError::VersionHistoryMismatch)
    );
    assert_eq!(registry.latest_version(program), Ok(1));
    registry
        .record_verified_upgrade(&predecessor, &upgrade)
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "append authenticated version two using its authenticated predecessor"
            )
        });
    assert_eq!(registry.latest_version(program), Ok(2));
    let resolved = registry
        .resolve_deployment(upgrade.clone())
        .or_panic("resolve upgraded deployment");
    assert_eq!(resolved.code_hash(), upgrade.code_hash());
    assert_eq!(
        registry.record_verified_upgrade(&predecessor, &upgrade),
        Err(RegistryError::VersionHistoryMismatch)
    );
    assert_eq!(registry.latest_version(program), Ok(2));
}

fn verify_program_bundle(
    verifier: &ProtocolDeploymentVerifier,
    proof: &DeploymentProof,
    bundle: &ProgramStateBundle,
    interface: &ProgramInterface,
    original: &layerx_programs::chain_proof::VerifiedChainHead,
    now: u64,
) -> (
    layerx_programs::VerifiedChainHead,
    layerx_programs::VerifiedProgramBundle,
) {
    let program = bundle.program;
    let chain = verifier
        .verify_current_chain_head(
            ProgramHeadKind::Ordinary,
            &proof.state.commitment,
            &proof.state.transaction,
            &proof.state.transaction_proof,
            now,
        )
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "transaction included under authenticated chain head"
            )
        });
    let encoded = bundle.canonical_encoding().or_panic("version two bundle");
    assert_eq!(&encoded[..5], b"LXPB\x02");
    let decoded = ProgramStateBundle::decode(&encoded).or_panic("decode version two bundle");
    assert_eq!(&decoded, bundle);
    let authenticated_bundle = verifier
        .verify_current_program_bundle(&decoded, &chain, program, now)
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "program, interface and wind-down proven at the same chain head"
            )
        });
    assert_eq!(authenticated_bundle.chain_head(), &chain);
    assert_eq!(authenticated_bundle.program_head().program(), program);
    assert_eq!(authenticated_bundle.program_head().version(), 1);
    assert_eq!(
        authenticated_bundle.program_head().code_hash(),
        interface.code_hash()
    );
    assert_eq!(
        authenticated_bundle.program_head().abi_version(),
        interface.abi_version()
    );
    assert_eq!(
        authenticated_bundle.program_head().lifecycle(),
        ProgramLifecycle::Active
    );
    assert_eq!(
        authenticated_bundle.program_head().policy(),
        UpgradePolicy::Immutable
    );
    assert_eq!(
        authenticated_bundle.program_head().state_root(),
        original.app_hash()
    );
    assert_eq!(&authenticated_bundle.interface().interface, interface);
    assert_eq!(authenticated_bundle.interface().program, program);
    assert_eq!(authenticated_bundle.interface().version, 1);
    assert_eq!(
        authenticated_bundle.interface().state_root,
        original.app_hash()
    );
    assert_eq!(
        decoded
            .wind_down
            .as_ref()
            .unwrap_or_else(|| panic!("{}", "wind-down proof"))
            .verify(original),
        Ok(None)
    );

    (chain, authenticated_bundle)
}

fn verify_registry_deployment(
    verifier: &ProtocolDeploymentVerifier,
    proof: &DeploymentProof,
    interface: &ProgramInterface,
    chain: &layerx_programs::VerifiedChainHead,
    authenticated_bundle: &layerx_programs::VerifiedProgramBundle,
    now: u64,
) -> (
    Registry,
    VerifiedDeploymentEvidence,
    VerifiedDeploymentEvidence,
) {
    let program = authenticated_bundle.program_head().program();
    let deployment = verifier
        .verify_deployment(proof, now)
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "deployment evidence from the same authenticated state and transaction"
            )
        });
    assert_eq!(
        deployment.program(),
        authenticated_bundle.program_head().program()
    );
    assert_eq!(
        deployment.version(),
        authenticated_bundle.program_head().version()
    );
    assert_eq!(
        deployment.code_hash(),
        authenticated_bundle.program_head().code_hash()
    );
    assert_eq!(
        deployment.state_root(),
        authenticated_bundle.program_head().state_root()
    );
    assert_eq!(deployment.receipt_digest(), chain.receipt_digest());
    assert_eq!(
        deployment.interface(),
        Some(&authenticated_bundle.interface().interface)
    );
    assert_eq!(deployment.module(), WASM);
    let mut registry = Registry::new();
    assert_eq!(
        registry.resolve_deployment(deployment.clone()),
        Err(RegistryError::UnknownProgram)
    );
    registry
        .record_verified_deployment(&deployment)
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "record chain-authenticated_bundle initial deployment"
            )
        });
    let resolved = registry
        .resolve_deployment(deployment.clone())
        .unwrap_or_else(|error| {
            panic!("{}: {error:?}", "resolve the registered program code hash")
        });
    assert_eq!(resolved.code_hash(), interface.code_hash());
    assert_eq!(registry.latest_version(program), Ok(1));

    (registry, deployment, resolved)
}

type TrustHistoryStep = (
    layerx_programs::chain_proof::header::ChainHeader,
    layerx_programs::chain_proof::Commit,
    ValidatorSet,
);

fn rotate_history(
    fixture: &mut ChainFixture,
    original: &layerx_programs::chain_proof::VerifiedChainHead,
) -> (
    layerx_programs::chain_proof::VerifiedChainHead,
    Vec<TrustHistoryStep>,
) {
    let mut rotated = ROTATED_SEEDS
        .iter()
        .map(|seed| {
            let key = SigningKey::from_bytes(seed);
            TrustedValidator::new(key.verifying_key().to_bytes(), 1)
        })
        .collect::<Vec<_>>();
    rotated.sort_by_key(|validator| validator.address);
    let rotated = ValidatorSet::new(rotated).or_panic("rotated validator set");
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
        sign(fixture, if offset <= 10 { &SEEDS } else { &ROTATED_SEEDS });
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
        .unwrap_or_else(|error| {
            panic!("{}: {error:?}", "current head signed by rotated validators")
        });
    assert_eq!(current.height(), original.height() + 20);
    assert_eq!(current.time_ms(), original.time_ms() + 20_000);
    (current, history)
}

fn verify_rotation(
    verifier: &ProtocolDeploymentVerifier,
    proof: &DeploymentProof,
    current: &layerx_programs::chain_proof::VerifiedChainHead,
    history: &[TrustHistoryStep],
    registry: &Registry,
    deployment: &VerifiedDeploymentEvidence,
    resolved: &VerifiedDeploymentEvidence,
) {
    let later = u64::try_from(current.time_ms()).or_panic("current time");
    assert_eq!(
        verifier.verify_deployment(proof, later),
        Err(ProtocolEvidenceError::Stale)
    );
    let historical = verifier
        .verify_historical_deployment(proof, current, history, later)
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "original deployment survives twenty heights and authenticated rotation"
            )
        });
    assert_eq!(&historical, deployment);
    assert_eq!(
        registry
            .resolve_deployment(historical)
            .or_panic("resolve historical deployment")
            .code_hash(),
        resolved.code_hash()
    );
    let mut missing_rotation = history.to_vec();
    missing_rotation.remove(10);
    assert_eq!(
        verifier.verify_historical_deployment(proof, current, &missing_rotation, later),
        Err(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
    let mut forged = history.to_vec();
    forged[10].1.signatures[0].signature[0] ^= 1;
    assert_eq!(
        verifier.verify_historical_deployment(proof, current, &forged, later),
        Err(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
}

#[test]
fn chain_bundle_records_deployment_and_survives_rotation() {
    let (mut fixture, proof, bundle, interface) = deployment_fixture();
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        &fixture.verifier.chain_id,
        fixture.verifier.validators.validators().to_vec(),
        MAX_AGE_MS,
    )
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "protocol verifier anchored to original chain validators"
        )
    });
    let original = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .or_panic("original signed header");
    let now = u64::try_from(original.time_ms()).or_panic("original time");
    let (chain, authenticated_bundle) =
        verify_program_bundle(&verifier, &proof, &bundle, &interface, &original, now);
    let (registry, deployment, resolved) = verify_registry_deployment(
        &verifier,
        &proof,
        &interface,
        &chain,
        &authenticated_bundle,
        now,
    );
    let (current, history) = rotate_history(&mut fixture, &original);
    verify_rotation(
        &verifier,
        &proof,
        &current,
        &history,
        &registry,
        &deployment,
        &resolved,
    );
}

#[allow(dead_code)]
#[path = "support/repair_22_33_221.rs"]
mod repair_22_33_221;

#[test]
fn node_upgrade_survives_hosted_envelope_restart() {
    repair_22_33_221::restart_recovery(true);
}
