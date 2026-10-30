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

fn key(prefix: &[u8], program: ProgramId) -> Vec<u8> {
    let mut key = prefix.to_vec();
    key.extend_from_slice(&program.bytes());
    chain_evidence::programs_state_key(&key)
}

fn fixture() -> (chain::ChainFixture, DeploymentProof, ProgramStateBundle) {
    let program = ProgramId::new([0x31; 32]).unwrap();
    let interface = ProgramInterface::bind(
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
    .unwrap();
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
                interface_state_value(program, 7, &interface).unwrap(),
            ),
        ],
    )
    .unwrap();
    let transaction = b"chain deployment transaction".to_vec();
    let tx_hash: [u8; 32] = Sha256::digest(&transaction).into();
    let tx_root = hash_from_byte_slices(&[&tx_hash]);
    fixture.header.data_hash = tx_root.to_vec();
    fixture.commit.block_id.hash = fixture.header.hash().unwrap().to_vec();
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
            .unwrap();
        let bytes = vote_sign_bytes(
            &fixture.header.chain_id,
            fixture.commit.height,
            fixture.commit.round,
            Some(&fixture.commit.block_id),
            signature.timestamp,
        )
        .unwrap();
        signature.signature = signer.sign(&bytes).to_bytes().to_vec();
    }
    let commitment = decode_commitment_proof(&fixture.encode_key(&program_key).unwrap()).unwrap();
    let state = ChainProgramStateProof {
        program,
        ops: fixture.prove(&program_key).unwrap(),
    };
    let wind_down = ChainWindDownProof {
        program,
        value: None,
        ops: fixture
            .prove_absent(&key(b"wind-down\0s", program))
            .unwrap(),
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
        interface_ops: fixture.prove(&interface_key).unwrap(),
        wind_down: Some(wind_down),
    };
    (fixture, deployment, bundle)
}

#[test]
fn verifier_public_methods_accept_chain_evidence() {
    let (fixture, proof, bundle) = fixture();
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        &fixture.verifier.chain_id,
        fixture.verifier.validators.validators().to_vec(),
        5_000,
    )
    .unwrap();
    let verified_header = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap();
    let root = verified_header.app_hash();
    let tx_hash: [u8; 32] = Sha256::digest(&proof.state.transaction).into();
    let bytes = proof.canonical_encoding().unwrap();
    assert_eq!(&bytes[..5], b"LXDP\x02");
    assert_eq!(DeploymentProof::decode(&bytes).unwrap(), proof);
    assert_eq!(proof.claimed_receipt_digest().unwrap(), tx_hash);
    let deployment = verifier.verify_deployment(&proof, NOW).unwrap();
    assert_eq!(deployment.state_root(), root);
    assert_eq!(deployment.programs_root(), root);
    assert_eq!(deployment.receipt_digest(), tx_hash);
    assert_eq!(deployment.activity_id(), tx_hash);
    assert_eq!(
        deployment.batch_header_digest(),
        verified_header.header_hash()
    );
    assert_eq!(
        deployment.freshness().observed_sequence,
        verified_header.height() as u64
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
            .unwrap(),
        deployment
    );
    let program = verifier
        .verify_current_program(&proof.state, bundle.program, NOW)
        .unwrap();
    assert_eq!(program.state_root(), root);
    assert_eq!(program.programs_root(), root);
    assert_eq!(program.receipt_digest(), tx_hash);
    assert_eq!(program.valid_until_ms(), NOW + 5_000);
    let commitment = &proof.state.commitment;
    let protocol = ProtocolHeadProof {
        commitment,
        chain: ChainProtocolHeadProof {
            ops: &commitment.proof_ops,
            kernel_key: &commitment.key[3..],
            value: commitment.value.as_deref().unwrap(),
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
        .unwrap();
    assert_eq!(head.state_root(), root);
    assert_eq!(head.receipt_digest(), tx_hash);
    assert_eq!(head.activity_id(), tx_hash);
    assert_eq!(head.freshness(), deployment.freshness());
    assert_eq!(
        verifier
            .verify_current_protocol_head_proof(&protocol, NOW)
            .unwrap(),
        head
    );
    assert_eq!(
        verifier
            .verify_historical_protocol_head(&protocol, &fixture.header, &fixture.commit, &[])
            .unwrap(),
        head
    );
    assert_eq!(
        verifier
            .verify_historical_protocol_head_proof(&protocol, &fixture.header, &fixture.commit, &[])
            .unwrap(),
        head
    );
    let encoded_bundle = bundle.canonical_encoding().unwrap();
    assert_eq!(&encoded_bundle[..5], b"LXPB\x02");
    let decoded_bundle = ProgramStateBundle::decode(&encoded_bundle).unwrap();
    assert_eq!(decoded_bundle, bundle);
    for kind in [ProgramHeadKind::Ordinary, ProgramHeadKind::Maintenance] {
        let chain = verifier
            .verify_current_chain_head(
                kind,
                commitment,
                &proof.state.transaction,
                &proof.state.transaction_proof,
                NOW,
            )
            .unwrap();
        assert_eq!(chain.state_root(), root);
        assert_eq!(chain.receipt_digest(), tx_hash);
        assert_eq!(chain.head_kind(), kind);
        assert_eq!(chain.receipt_evidence_digest().bytes(), tx_hash);
        let verified = verifier
            .verify_current_program_bundle(&decoded_bundle, &chain, bundle.program, NOW)
            .unwrap();
        assert_eq!(verified.program_head().state_root(), root);
        assert_eq!(verified.program_head().programs_root(), root);
        assert_eq!(verified.chain_head().state_root(), root);
        assert_eq!(verified.interface().state_root, root);
    }
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
    assert!(verifier.verify_deployment(&proof, NOW + 5_001).is_err());
    let mut tampered = proof.clone();
    tampered.deployment.code[0] ^= 1;
    assert!(verifier.verify_deployment(&tampered, NOW).is_err());
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
        .unwrap();
    assert!(verifier
        .verify_current_program_bundle(&forged, &chain, bundle.program, NOW)
        .is_err());
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
        kernel.extend_from_slice(&(material.len() as u32).to_be_bytes());
        kernel.extend_from_slice(material);
    }
    assert_eq!(
        DeploymentProof::decode(&kernel),
        Err(ProtocolEvidenceError::Encoding)
    );
    let mut old_version = proof.canonical_encoding().unwrap();
    old_version[4] = 1;
    assert_eq!(
        DeploymentProof::decode(&old_version),
        Err(ProtocolEvidenceError::Encoding)
    );
    let mut trailing = proof.canonical_encoding().unwrap();
    trailing.push(0);
    assert_eq!(
        DeploymentProof::decode(&trailing),
        Err(ProtocolEvidenceError::Encoding)
    );
    let mut old_bundle = bundle.canonical_encoding().unwrap();
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
    .unwrap();
    assert!(verifier.verify_deployment(&kernel_header, NOW).is_err());
}
