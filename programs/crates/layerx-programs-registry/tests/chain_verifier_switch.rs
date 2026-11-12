#[allow(dead_code)]
#[path = "support/chain.rs"]
mod chain;

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::{
    chain_evidence::{
        self, deployment::ChainDeploymentProof, lifecycle::ChainLifecycleProof,
        program_state::ChainProgramStateProof, protocol_head::ChainProtocolHeadProof,
        wind_down::ChainWindDownProof,
    },
    chain_proof::{
        decode::decode_commitment_proof,
        merkle::{hash_from_byte_slices, SimpleProof},
        validators::TrustedValidator,
        vote::vote_sign_bytes,
    },
    interface_state_key, interface_state_value, DeploymentProof, InterfaceEntryPoint,
    ProgramHeadKind, ProgramId, ProgramInterface, ProgramLifecycle, ProgramLifecycleProof,
    ProgramStateBundle, ProgramStateProof, ProtocolDeploymentVerifier, ProtocolEvidenceError,
    ProtocolHeadProof, ReceiptEvidenceDigest, ValueSchema, ValueType,
};
use sha2::{Digest, Sha256};

pub const WASM_V1: &[u8] = &[
    0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127, 3, 3, 2, 0,
    1, 5, 3, 1, 0, 1, 7, 34, 3, 4, b'c', b'a', b'l', b'l', 0, 0, 14, b'l', b'a', b'y', b'e', b'r',
    b'x', b'_', b'r', b'e', b's', b'e', b'r', b'v', b'e', 0, 1, 6, b'm', b'e', b'm', b'o', b'r',
    b'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
];

const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const NOW: u64 = 1_700_000_000_123;
const PERSISTED_TRUST: &str = "LAYERX_CHAIN_TRUST_ONLY_TEST_ROOT";

fn key(prefix: &[u8], program: ProgramId) -> Vec<u8> {
    let mut key = prefix.to_vec();
    key.extend_from_slice(&program.bytes());
    chain_evidence::programs_state_key(&key)
}

fn fixture_interface() -> ProgramInterface {
    ProgramInterface::bind(
        WASM_V1,
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
    .unwrap_or_else(|error| panic!("{error:?}"))
}

fn prepare_signed_transaction(fixture: &mut chain::ChainFixture) -> (Vec<u8>, [u8; 32]) {
    let transaction = b"chain deployment transaction".to_vec();
    let tx_hash: [u8; 32] = Sha256::digest(&transaction).into();
    let tx_root = hash_from_byte_slices(&[&tx_hash]);
    fixture.header.data_hash = tx_root.to_vec();
    fixture.commit.block_id.hash = fixture
        .header
        .hash()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .to_vec();
    let keys: Vec<_> = SEEDS.iter().map(SigningKey::from_bytes).collect();
    for signature in &mut fixture.commit.signatures {
        let signer = keys
            .iter()
            .find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == signature.validator_address
            })
            .unwrap_or_else(|| panic!("expected value"));
        let bytes = vote_sign_bytes(
            &fixture.header.chain_id,
            fixture.commit.height,
            fixture.commit.round,
            Some(&fixture.commit.block_id),
            signature.timestamp,
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        signature.signature = signer.sign(&bytes).to_bytes().to_vec();
    }
    (transaction, tx_root)
}

fn fixture() -> (chain::ChainFixture, DeploymentProof, ProgramStateBundle) {
    let program = ProgramId::new([0x31; 32]).unwrap_or_else(|error| panic!("{error:?}"));
    let interface = fixture_interface();
    let mut record = vec![0; 33];
    record.extend_from_slice(&interface.code_hash());
    record.extend_from_slice(&1_u16.to_be_bytes());
    record.extend_from_slice(&7_u32.to_be_bytes());
    let program_key = key(b"program\0", program);
    let interface_key = chain_evidence::programs_state_key(&interface_state_key(program));
    let mut fixture = chain::ChainFixture::new(
        "paxeer-public-verifier-switch",
        &SEEDS,
        vec![
            (program_key.clone(), record.clone()),
            (
                interface_key.clone(),
                interface_state_value(program, 7, &interface)
                    .unwrap_or_else(|error| panic!("{error:?}")),
            ),
        ],
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let (transaction, tx_root) = prepare_signed_transaction(&mut fixture);
    let commitment = decode_commitment_proof(
        &fixture
            .encode_key(&program_key)
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let state = ChainProgramStateProof {
        program,
        ops: fixture
            .prove(&program_key)
            .unwrap_or_else(|error| panic!("{error:?}")),
    };
    let wind_down = ChainWindDownProof {
        program,
        value: None,
        ops: fixture
            .prove_absent(&key(b"wind-down\0s", program))
            .unwrap_or_else(|error| panic!("{error:?}")),
    };
    let lifecycle = ProgramLifecycleProof(ChainLifecycleProof {
        program,
        record: Some(record),
        record_ops: state.ops.clone(),
        status: None,
        status_ops: wind_down.ops.clone(),
    });
    let deployment = DeploymentProof {
        state: ProgramStateProof {
            commitment: commitment.clone(),
            state: state.clone(),
            lifecycle,
            wind_down: wind_down.clone(),
            transaction,
            transaction_proof: SimpleProof {
                total: 1,
                index: 0,
                leaf_hash: tx_root,
                aunts: Vec::new(),
            },
        },
        deployment: ChainDeploymentProof {
            program_state: state.clone(),
            code: WASM_V1.to_vec(),
            interface: Some(interface.canonical_encoding().to_vec()),
        },
    };
    let bundle = ProgramStateBundle {
        commitment,
        program,
        program_record_ops: state.ops,
        interface_ops: fixture
            .prove(&interface_key)
            .unwrap_or_else(|error| panic!("{error:?}")),
        wind_down: Some(wind_down),
    };
    (fixture, deployment, bundle)
}

fn verify_protocol_head_methods(
    fixture: &chain::ChainFixture,
    proof: &DeploymentProof,
    verifier: &ProtocolDeploymentVerifier,
    root: [u8; 32],
    tx_hash: [u8; 32],
    expected_freshness: layerx_programs::ReadFreshness,
) {
    let commitment = &proof.state.commitment;
    let protocol = ProtocolHeadProof {
        commitment,
        chain: ChainProtocolHeadProof {
            ops: &commitment.proof_ops,
            kernel_key: &commitment.key[3..],
            value: commitment
                .value
                .as_deref()
                .unwrap_or_else(|| panic!("expected value")),
            store_name: &commitment.store_name,
            verifier: &fixture.verifier,
            header: &fixture.header,
            commit: &fixture.commit,
        },
        transaction: &proof.state.transaction,
        transaction_proof: &proof.state.transaction_proof,
    };
    let head = verifier
        .verify_current_protocol_head(&protocol, NOW)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(head.state_root(), root);
    assert_eq!(head.receipt_digest(), tx_hash);
    assert_eq!(head.activity_id(), tx_hash);
    assert_eq!(head.freshness(), expected_freshness);
    assert_eq!(
        verifier
            .verify_current_protocol_head_proof(&protocol, NOW)
            .unwrap_or_else(|error| panic!("{error:?}")),
        head
    );
    assert_eq!(
        verifier
            .verify_historical_protocol_head(&protocol, &fixture.header, &fixture.commit, &[])
            .unwrap_or_else(|error| panic!("{error:?}")),
        head
    );
    assert_eq!(
        verifier
            .verify_historical_protocol_head_proof(&protocol, &fixture.header, &fixture.commit, &[])
            .unwrap_or_else(|error| panic!("{error:?}")),
        head
    );
}

fn verify_program_bundle_methods(
    verifier: &ProtocolDeploymentVerifier,
    proof: &DeploymentProof,
    bundle: &ProgramStateBundle,
    root: [u8; 32],
    tx_hash: [u8; 32],
) {
    let commitment = &proof.state.commitment;
    let encoded_bundle = bundle
        .canonical_encoding()
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(&encoded_bundle[..5], b"LXPB\x02");
    let decoded_bundle =
        ProgramStateBundle::decode(&encoded_bundle).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(&decoded_bundle, bundle);
    for kind in [ProgramHeadKind::Ordinary, ProgramHeadKind::Maintenance] {
        let chain = verifier
            .verify_current_chain_head(
                kind,
                commitment,
                &proof.state.transaction,
                &proof.state.transaction_proof,
                NOW,
            )
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(chain.state_root(), root);
        assert_eq!(chain.receipt_digest(), tx_hash);
        assert_eq!(chain.head_kind(), kind);
        assert_eq!(chain.receipt_evidence_digest().bytes(), tx_hash);
        let authenticated_bundle = verifier
            .verify_current_program_bundle(&decoded_bundle, &chain, bundle.program, NOW)
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(authenticated_bundle.program_head().state_root(), root);
        assert_eq!(authenticated_bundle.program_head().programs_root(), root);
        assert_eq!(authenticated_bundle.chain_head().state_root(), root);
        assert_eq!(authenticated_bundle.interface().state_root, root);
    }
}

fn verify_public_evidence_rejections(
    verifier: &ProtocolDeploymentVerifier,
    proof: &DeploymentProof,
    bundle: &ProgramStateBundle,
    tx_hash: [u8; 32],
) {
    assert_eq!(ProgramHeadKind::Ordinary.wire_byte(), 0);
    assert_eq!(ProgramHeadKind::Maintenance.wire_byte(), 1);
    assert_eq!(
        ProgramHeadKind::from_wire_byte(0),
        Some(ProgramHeadKind::Ordinary)
    );
    assert_eq!(
        ProgramHeadKind::from_wire_byte(1),
        Some(ProgramHeadKind::Maintenance)
    );
    assert_eq!(ProgramHeadKind::from_wire_byte(2), None);
    assert_eq!(
        ReceiptEvidenceDigest::OrdinaryUnsignedReceipt(tx_hash).bytes(),
        tx_hash
    );
    assert_eq!(
        ReceiptEvidenceDigest::MaintenanceReceiptSha256(tx_hash).bytes(),
        tx_hash
    );
    let mut tampered = proof.clone();
    tampered.state.transaction[0] ^= 1;
    assert!(verifier.verify_deployment(&tampered, NOW).is_err());
    assert!(verifier
        .verify_current_program(&tampered.state, bundle.program, NOW)
        .is_err());
    assert!(verifier.verify_deployment(proof, NOW + 5_001).is_err());
    let mut tampered = proof.clone();
    tampered.deployment.code[0] ^= 1;
    assert!(verifier.verify_deployment(&tampered, NOW).is_err());
    let commitment = &proof.state.commitment;
    let mut forged = bundle.clone();
    forged.commitment.commit[0] ^= 1;
    let chain = verifier
        .verify_current_chain_head(
            ProgramHeadKind::Ordinary,
            commitment,
            &proof.state.transaction,
            &proof.state.transaction_proof,
            NOW,
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(verifier
        .verify_current_program_bundle(&forged, &chain, bundle.program, NOW)
        .is_err());
}

#[test]
fn verifier_public_methods_accept_chain_evidence() {
    let (fixture, proof, bundle) = fixture();
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        &fixture.verifier.chain_id,
        fixture.verifier.validators.validators().to_vec(),
        5_000,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let verified_header = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let root = verified_header.app_hash();
    let tx_hash: [u8; 32] = Sha256::digest(&proof.state.transaction).into();
    let bytes = proof
        .canonical_encoding()
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(&bytes[..5], b"LXDP\x02");
    assert_eq!(
        DeploymentProof::decode(&bytes).unwrap_or_else(|error| panic!("{error:?}")),
        proof
    );
    assert_eq!(
        proof
            .claimed_receipt_digest()
            .unwrap_or_else(|error| panic!("{error:?}")),
        tx_hash
    );
    let deployment = verifier
        .verify_deployment(&proof, NOW)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(deployment.state_root(), root);
    assert_eq!(deployment.programs_root(), root);
    assert_eq!(deployment.receipt_digest(), tx_hash);
    assert_eq!(deployment.activity_id(), tx_hash);
    assert_eq!(
        layerx_programs::chain_proof::header::ChainHeader::decode(
            &deployment.proof().state.commitment.signed_header
        )
        .unwrap_or_else(|error| panic!("{error:?}"))
        .hash()
        .unwrap_or_else(|error| panic!("{error:?}")),
        verified_header.header_hash()
    );
    assert_eq!(
        deployment.freshness().observed_sequence,
        u64::from_le_bytes((verified_header.height()).to_le_bytes())
    );
    assert_eq!(deployment.freshness().observed_at, NOW);
    assert_eq!(deployment.module(), WASM_V1);
    assert!(deployment.interface_present());
    assert_eq!(deployment.version(), 7);
    assert_eq!(deployment.lifecycle(), ProgramLifecycle::Active);
    let history = [(
        fixture.header.clone(),
        fixture.commit.clone(),
        fixture.verifier.validators.clone(),
    )];
    assert_eq!(
        verifier
            .verify_historical_deployment(&proof, &verified_header, &history, NOW)
            .unwrap_or_else(|error| panic!("{error:?}")),
        deployment
    );
    let program = verifier
        .verify_current_program(&proof.state, bundle.program, NOW)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(program.state_root(), root);
    assert_eq!(program.programs_root(), root);
    assert_eq!(program.receipt_digest(), tx_hash);
    assert_eq!(program.valid_until_ms(), NOW + 5_000);
    verify_protocol_head_methods(
        &fixture,
        &proof,
        &verifier,
        root,
        tx_hash,
        deployment.freshness(),
    );
    verify_program_bundle_methods(&verifier, &proof, &bundle, root, tx_hash);
    verify_public_evidence_rejections(&verifier, &proof, &bundle, tx_hash);
}

#[test]
fn verifier_public_methods_refuse_kernel_batch_evidence() {
    let (_, proof, bundle) = fixture();
    // The old decoder admitted this domain followed by kernel activity and batch material.
    let mut kernel = b"LayerX/programs/deployment-proof/v1\0".to_vec();
    for material in [
        include_bytes!("fixtures/maintained-head/receipt").as_slice(),
        include_bytes!("fixtures/maintained-head/header").as_slice(),
        include_bytes!("fixtures/maintained-head/header.signature").as_slice(),
    ] {
        kernel.extend_from_slice(
            &u32::try_from(material.len())
                .unwrap_or_else(|error| panic!("{error:?}"))
                .to_be_bytes(),
        );
        kernel.extend_from_slice(material);
    }
    assert_eq!(
        DeploymentProof::decode(&kernel),
        Err(ProtocolEvidenceError::Encoding)
    );
    let mut old_version = proof
        .canonical_encoding()
        .unwrap_or_else(|error| panic!("{error:?}"));
    old_version[4] = 1;
    assert_eq!(
        DeploymentProof::decode(&old_version),
        Err(ProtocolEvidenceError::Encoding)
    );
    let mut trailing = proof
        .canonical_encoding()
        .unwrap_or_else(|error| panic!("{error:?}"));
    trailing.push(0);
    assert_eq!(
        DeploymentProof::decode(&trailing),
        Err(ProtocolEvidenceError::Encoding)
    );
    let mut old_bundle = bundle
        .canonical_encoding()
        .unwrap_or_else(|error| panic!("{error:?}"));
    old_bundle[4] = 1;
    assert!(ProgramStateBundle::decode(&old_bundle).is_err());
    assert!(ProgramStateBundle::decode(b"LayerX/programs/state-proof/v1\0").is_err());
    let mut kernel_header = proof.clone();
    kernel_header.state.commitment.signed_header =
        include_bytes!("fixtures/maintained-head/header").to_vec();
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        "paxeer-public-verifier-switch",
        fixture().0.verifier.validators.validators().to_vec(),
        5_000,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(verifier.verify_deployment(&kernel_header, NOW).is_err());
}

fn verify_trust_in_empty_directory() {
    use layerx_programs::chain_proof::trust_store::TrustRoot;
    use std::fs;
    use std::process::Command;
    use std::time::{SystemTime, UNIX_EPOCH};
    let (fixture, _, _) = fixture();
    let header = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let trust = TrustRoot::from_head(&header, fixture.verifier.validators.clone())
        .unwrap_or_else(|error| panic!("{error:?}"));
    let directory = std::env::temp_dir().join(format!(
        "layerx-chain-trust-only-{}-{}",
        std::process::id(),
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .as_nanos()
    ));
    fs::create_dir(&directory).unwrap_or_else(|error| panic!("{error:?}"));
    let trust_path = directory.join("chain-trust-root");
    fs::write(
        &trust_path,
        trust
            .canonical_bytes()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let persisted_bytes = fs::read(&trust_path).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        TrustRoot::decode(&persisted_bytes).unwrap_or_else(|error| panic!("{error:?}")),
        trust
    );
    fs::remove_file(&trust_path).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        fs::read_dir(&directory)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .count(),
        0
    );
    let output = Command::new(std::env::current_exe().unwrap_or_else(|error| panic!("{error:?}")))
        .args([
            "--exact",
            "verifier_needs_no_trust_history_file",
            "--nocapture",
        ])
        .current_dir(&directory)
        .env(
            PERSISTED_TRUST,
            layerx_programs::hex::encode(&persisted_bytes),
        )
        .output()
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(
        output.status.success(),
        "empty-directory verification failed: {}\n{}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    assert_eq!(
        fs::read_dir(&directory)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .count(),
        0
    );
    fs::remove_dir(directory).unwrap_or_else(|error| panic!("{error:?}"));
}

#[test]
fn verifier_needs_no_trust_history_file() {
    use layerx_programs::chain_proof::trust_store::TrustRoot;
    use std::fs;
    let persisted = match std::env::var(PERSISTED_TRUST) {
        Ok(encoded) => TrustRoot::decode(
            &layerx_programs::hex::decode(&encoded).unwrap_or_else(|error| panic!("{error:?}")),
        )
        .unwrap_or_else(|error| panic!("{error:?}")),
        Err(std::env::VarError::NotPresent) => {
            verify_trust_in_empty_directory();
            return;
        }
        Err(error) => panic!("invalid persisted trust environment: {error}"),
    };
    let (fixture, proof, _) = fixture();
    let header = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let directory = std::env::current_dir().unwrap_or_else(|error| panic!("{error:?}"));
    let history_path = directory.join("sequencer-trust-history");
    assert_eq!(
        fs::read_dir(&directory)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .count(),
        0
    );
    assert_eq!(
        fs::symlink_metadata(&history_path)
            .err()
            .unwrap_or_else(|| panic!("expected an error"))
            .kind(),
        std::io::ErrorKind::NotFound
    );
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        persisted.chain_id,
        persisted.validators.validators().to_vec(),
        5_000,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let deployment = verifier
        .verify_deployment(&proof, NOW)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(deployment.program(), proof.deployment.program_state.program);
    assert_eq!(deployment.module(), WASM_V1);
    assert_eq!(
        deployment.code_hash(),
        <[u8; 32]>::from(Sha256::digest(WASM_V1))
    );
    assert_eq!(deployment.state_root(), header.app_hash());
    assert_eq!(
        deployment.receipt_digest(),
        <[u8; 32]>::from(Sha256::digest(&proof.state.transaction))
    );
    assert_eq!(
        deployment.freshness().observed_sequence,
        u64::from_le_bytes((header.height()).to_le_bytes())
    );
    assert_eq!(deployment.freshness().observed_at, NOW);
    assert_eq!(deployment.lifecycle(), ProgramLifecycle::Active);
    assert_eq!(deployment.version(), 7);
    let mut tampered = proof.clone();
    tampered.state.transaction[0] ^= 1;
    assert!(verifier.verify_deployment(&tampered, NOW).is_err());
    assert_eq!(
        verifier.verify_deployment(&proof, NOW + 5_001),
        Err(ProtocolEvidenceError::Stale)
    );
    assert_eq!(
        fs::symlink_metadata(&history_path)
            .err()
            .unwrap_or_else(|| panic!("expected an error"))
            .kind(),
        std::io::ErrorKind::NotFound
    );
    assert_eq!(
        fs::read_dir(&directory)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .count(),
        0
    );
}
