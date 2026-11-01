pub mod support;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_programs::{
    chain_evidence::{programs_state_key, protocol_head::ChainProtocolHeadProof},
    chain_proof::{
        decode::decode_commitment_proof, header::ChainHeader, validators::TrustedValidator,
        vote::vote_sign_bytes,
    },
    ProgramBundleError, ProgramHeadKind, ProgramStateBundle, ProtocolDeploymentVerifier,
    ProtocolEvidenceError, ProtocolHeadProof, ReceiptEvidenceDigest, VerifiedChainHead,
};
use layerx_programs_runtime::{ProgramId, UpgradePolicy};
use sha2::{Digest, Sha256};
use support::{
    chain::ChainFixture, deploy_fixture, program, verifier_for_fixture, ProtocolFixture, AUTHORITY,
    NOW, WASM_V1, WASM_V2,
};

const STALENESS_MS: u64 = 1_000;
const OBSERVED_AT: u64 = NOW - 100;
const VALIDATOR_SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];

fn deployed(block_height: u64, wasm: &[u8]) -> ProtocolFixture {
    deploy_fixture(
        wasm,
        UpgradePolicy::Authority(AUTHORITY),
        block_height,
        OBSERVED_AT,
    )
}

fn verifier(fixture: &ProtocolFixture) -> ProtocolDeploymentVerifier {
    verifier_for_fixture(fixture, STALENESS_MS)
}

fn ordinary_bundle(fixture: &ProtocolFixture) -> ProgramStateBundle {
    ProgramStateBundle {
        commitment: fixture.proof.state.commitment.clone(),
        program: program(),
        program_record_ops: fixture.proof.state.state.ops.clone(),
        interface_ops: fixture.interface_witness.proof.clone(),
        wind_down: Some(fixture.proof.state.wind_down.clone()),
    }
}

fn chain_head(
    verifier: &ProtocolDeploymentVerifier,
    fixture: &ProtocolFixture,
    kind: ProgramHeadKind,
) -> VerifiedChainHead {
    let state = &fixture.proof.state;
    verifier
        .verify_current_chain_head(
            kind,
            &state.commitment,
            &state.transaction,
            &state.transaction_proof,
            NOW,
        )
        .unwrap_or_else(|error| panic!("independent chain head: {error}"))
}

fn ordinary_chain(
    verifier: &ProtocolDeploymentVerifier,
    fixture: &ProtocolFixture,
) -> VerifiedChainHead {
    chain_head(verifier, fixture, ProgramHeadKind::Ordinary)
}

fn other_program() -> ProgramId {
    ProgramId::new([0x32; 32]).unwrap_or_else(|error| panic!("program: {error}"))
}

fn foreign_fixture() -> ProtocolFixture {
    let mut fixture = deployed(70, WASM_V1);
    let state = &fixture.proof.state;
    let commitment = &state.commitment;
    let mut chain = ChainFixture::new(
        "paxeer-other-chain",
        &VALIDATOR_SEEDS,
        vec![
            (
                commitment.key.clone(),
                commitment.value.clone().expect("program record"),
            ),
            (
                programs_state_key(&fixture.interface_witness.key),
                fixture.interface_witness.value.clone(),
            ),
        ],
    )
    .expect("independent foreign chain fixture");
    assert_eq!(chain.header.app_hash, fixture.chain.header.app_hash);
    let chain_id = chain.header.chain_id.clone();
    chain.header = fixture.chain.header.clone();
    chain.header.chain_id = chain_id;
    chain.commit.height = chain.header.height;
    chain.commit.block_id.hash = chain.header.hash().expect("foreign header hash").to_vec();
    for signature in &mut chain.commit.signatures {
        let key = VALIDATOR_SEEDS
            .iter()
            .map(SigningKey::from_bytes)
            .find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == signature.validator_address
            })
            .expect("validator signing key");
        signature.timestamp.seconds = chain.header.time.seconds;
        signature.timestamp.nanos = chain.header.time.nanos;
        let bytes = vote_sign_bytes(
            &chain.header.chain_id,
            chain.commit.height,
            chain.commit.round,
            Some(&chain.commit.block_id),
            signature.timestamp,
        )
        .expect("foreign vote sign bytes");
        signature.signature = key.sign(&bytes).to_bytes().to_vec();
    }
    fixture.proof.state.commitment = decode_commitment_proof(
        &chain
            .encode_key(&commitment.key)
            .expect("foreign commitment encoding"),
    )
    .expect("foreign commitment");
    fixture.chain = chain;
    fixture
}

#[test]
fn head_kind_byte_and_digest_domains_are_strict_and_distinct() {
    assert_eq!(
        ProgramHeadKind::from_wire_byte(0),
        Some(ProgramHeadKind::Ordinary)
    );
    assert_eq!(
        ProgramHeadKind::from_wire_byte(1),
        Some(ProgramHeadKind::Maintenance)
    );
    for byte in 2..=u8::MAX {
        assert_eq!(ProgramHeadKind::from_wire_byte(byte), None);
    }
    let digest = [0x5a; 32];
    let ordinary = ReceiptEvidenceDigest::OrdinaryUnsignedReceipt(digest);
    let maintenance = ReceiptEvidenceDigest::MaintenanceReceiptSha256(digest);
    assert_ne!(ordinary, maintenance);
    assert_eq!(ordinary.bytes(), maintenance.bytes());
    assert_eq!(ordinary.head_kind(), ProgramHeadKind::Ordinary);
    assert_eq!(maintenance.head_kind(), ProgramHeadKind::Maintenance);
}

#[test]
fn ordinary_bundle_verifies_only_against_its_independent_chain_head() {
    let fixture = deployed(70, WASM_V1);
    let verifier = verifier(&fixture);
    let chain = ordinary_chain(&verifier, &fixture);
    assert_eq!(chain.head_kind(), ProgramHeadKind::Ordinary);
    assert!(matches!(
        chain.receipt_evidence_digest(),
        ReceiptEvidenceDigest::OrdinaryUnsignedReceipt(_)
    ));
    assert_eq!(
        chain.receipt_digest(),
        chain.receipt_evidence_digest().bytes()
    );
    assert_eq!(
        chain.receipt_digest(),
        <[u8; 32]>::from(Sha256::digest(&fixture.proof.state.transaction))
    );
    assert_eq!(
        chain.freshness().observed_sequence,
        u64::try_from(fixture.chain.header.height).expect("positive height")
    );
    assert_eq!(
        chain.freshness().observed_sequence,
        u64::try_from(
            ChainHeader::decode(&fixture.proof.state.commitment.signed_header)
                .expect("committed header")
                .height
        )
        .expect("positive committed height")
    );
    assert_eq!(chain.freshness().observed_at, OBSERVED_AT);
    assert_eq!(chain.state_root().as_slice(), fixture.chain.header.app_hash);
    assert_eq!(
        ChainHeader::decode(&fixture.proof.state.commitment.signed_header)
            .expect("committed header")
            .hash()
            .expect("committed header hash"),
        fixture.chain.header.hash().expect("header hash")
    );

    let encoded = ordinary_bundle(&fixture)
        .canonical_encoding()
        .expect("version two bundle");
    assert_eq!(&encoded[..5], b"LXPB\x02");
    let decoded = ProgramStateBundle::decode(&encoded).expect("decoded version two bundle");
    let verified = verifier
        .verify_current_program_bundle(&decoded, &chain, program(), NOW)
        .expect("bound program bundle");
    let head = verified.program_head();
    assert_eq!(verified.chain_head(), &chain);
    assert_eq!(head.program(), program());
    assert_eq!(head.receipt_digest(), chain.receipt_digest());
    assert_eq!(head.state_root(), chain.state_root());
    assert_eq!(head.programs_root(), chain.programs_root());
    assert_eq!(head.freshness(), chain.freshness());
    assert_eq!(
        Some(head.valid_until_ms()),
        chain.freshness().observed_at.checked_add(STALENESS_MS)
    );
    assert_eq!(verified.interface().state_root, head.state_root());
    assert_eq!(
        verified.interface().interface,
        support::fixture_interface(WASM_V1)
    );

    let foreign = foreign_fixture();
    let foreign_verifier = verifier_for_fixture(&foreign, STALENESS_MS);
    let foreign_chain = ordinary_chain(&foreign_verifier, &foreign);
    let foreign_bytes = ordinary_bundle(&foreign)
        .canonical_encoding()
        .expect("foreign version two bundle");
    assert_eq!(&foreign_bytes[..5], b"LXPB\x02");
    let foreign_bundle =
        ProgramStateBundle::decode(&foreign_bytes).expect("decoded foreign bundle");
    assert_ne!(fixture.chain.header.chain_id, foreign.chain.header.chain_id);
    assert_eq!(
        fixture.chain.verifier.validators,
        foreign.chain.verifier.validators
    );
    assert_eq!(chain.state_root(), foreign_chain.state_root());
    assert_eq!(
        chain.freshness().observed_sequence,
        foreign_chain.freshness().observed_sequence
    );
    assert_eq!(chain.freshness(), foreign_chain.freshness());
    assert_eq!(chain.receipt_digest(), foreign_chain.receipt_digest());
    assert_eq!(
        decoded.program_record_ops,
        foreign_bundle.program_record_ops
    );
    assert_eq!(decoded.interface_ops, foreign_bundle.interface_ops);
    assert_eq!(decoded.wind_down, foreign_bundle.wind_down);
    assert_ne!(
        fixture.chain.header.hash().expect("header hash"),
        foreign.chain.header.hash().expect("foreign header hash")
    );
    assert_eq!(
        foreign_verifier
            .verify_current_program_bundle(&foreign_bundle, &foreign_chain, program(), NOW)
            .expect("foreign bundle against its own chain")
            .chain_head(),
        &foreign_chain
    );
    assert_eq!(
        verifier.verify_current_program_bundle(&decoded, &foreign_chain, program(), NOW),
        Err(ProgramBundleError::ChainHeadMismatch)
    );
    assert_eq!(
        foreign_verifier.verify_current_program_bundle(&foreign_bundle, &chain, program(), NOW),
        Err(ProgramBundleError::ChainHeadMismatch)
    );
    assert_eq!(
        verifier.verify_current_program_bundle(&foreign_bundle, &chain, program(), NOW),
        Err(ProgramBundleError::Evidence(
            ProtocolEvidenceError::StateRoot
        ))
    );
    assert_eq!(
        foreign_verifier.verify_current_program_bundle(&decoded, &foreign_chain, program(), NOW),
        Err(ProgramBundleError::Evidence(
            ProtocolEvidenceError::StateRoot
        ))
    );
}

#[test]
fn bundle_commitment_must_equal_the_chain_head_commitment() {
    let fixture = deployed(70, WASM_V1);
    let verifier = verifier(&fixture);
    let chain = ordinary_chain(&verifier, &fixture);
    let mut changed = ordinary_bundle(&fixture);
    changed.commitment.signed_header = foreign_fixture().proof.state.commitment.signed_header;
    assert_eq!(
        verifier.verify_current_program_bundle(&changed, &chain, program(), NOW),
        Err(ProgramBundleError::Evidence(
            ProtocolEvidenceError::StateRoot
        ))
    );
}

#[test]
fn unauthorised_validators_and_wrong_program_are_refused() {
    let fixture = deployed(70, WASM_V1);
    let verifier = verifier(&fixture);
    let chain = ordinary_chain(&verifier, &fixture);
    let bundle = ordinary_bundle(&fixture);
    let untrusted = ProtocolDeploymentVerifier::from_chain_trust(
        &fixture.chain.header.chain_id,
        vec![TrustedValidator::new(
            SigningKey::from_bytes(&[9; 32]).verifying_key().to_bytes(),
            1,
        )],
        STALENESS_MS,
    )
    .expect("different validator trust");
    assert_eq!(
        untrusted.verify_current_program_bundle(&bundle, &chain, program(), NOW),
        Err(ProgramBundleError::Evidence(
            ProtocolEvidenceError::InvalidTrustAnchor
        ))
    );
    assert_eq!(
        verifier.verify_current_program_bundle(&bundle, &chain, other_program(), NOW),
        Err(ProgramBundleError::ProgramMismatch)
    );
}

#[test]
fn cross_associated_or_advanced_heads_are_refused() {
    let pinned = deployed(70, WASM_V1);
    let advanced = deployed(71, WASM_V1);
    let verifier = verifier(&pinned);
    let pinned_chain = ordinary_chain(&verifier, &pinned);
    let advanced_chain = ordinary_chain(&verifier, &advanced);
    assert_ne!(
        pinned_chain.freshness().observed_sequence,
        advanced_chain.freshness().observed_sequence
    );
    for (bundle, chain) in [
        (ordinary_bundle(&advanced), &pinned_chain),
        (ordinary_bundle(&pinned), &advanced_chain),
    ] {
        assert_eq!(
            verifier.verify_current_program_bundle(&bundle, chain, program(), NOW),
            Err(ProgramBundleError::ChainHeadMismatch)
        );
    }
}

#[test]
fn cross_associated_or_missing_interface_is_refused() {
    let fixture = deployed(70, WASM_V1);
    let unrelated = deployed(72, WASM_V2);
    let verifier = verifier(&fixture);
    let chain = ordinary_chain(&verifier, &fixture);
    let mut foreign = ordinary_bundle(&fixture);
    foreign.interface_ops = unrelated.interface_witness.proof;
    assert!(matches!(
        verifier.verify_current_program_bundle(&foreign, &chain, program(), NOW),
        Err(ProgramBundleError::Interface(_))
    ));
    let mut missing = ordinary_bundle(&fixture);
    missing.interface_ops = missing.program_record_ops.clone();
    assert_eq!(
        verifier.verify_current_program_bundle(&missing, &chain, program(), NOW),
        Err(ProgramBundleError::ProgramMismatch)
    );
    missing.interface_ops.clear();
    assert_eq!(
        verifier.verify_current_program_bundle(&missing, &chain, program(), NOW),
        Err(ProgramBundleError::Evidence(
            ProtocolEvidenceError::Encoding
        ))
    );
}

#[test]
fn altered_head_or_state_material_is_refused() {
    let fixture = deployed(70, WASM_V1);
    let other = deployed(71, WASM_V2);
    let verifier = verifier(&fixture);
    let chain = ordinary_chain(&verifier, &fixture);
    let verify = |bundle: &ProgramStateBundle| {
        verifier.verify_current_program_bundle(bundle, &chain, program(), NOW)
    };

    let mut header = ordinary_bundle(&fixture);
    let last = header.commitment.signed_header.len() - 1;
    header.commitment.signed_header[last] ^= 1;
    assert!(matches!(
        verify(&header),
        Err(ProgramBundleError::Evidence(_))
    ));
    let mut signature = ordinary_bundle(&fixture);
    let last = signature.commitment.commit.len() - 1;
    signature.commitment.commit[last] ^= 1;
    assert!(matches!(
        verify(&signature),
        Err(ProgramBundleError::Evidence(_))
    ));
    let mut value = ordinary_bundle(&fixture);
    value.commitment.value.as_mut().expect("program record")[0] ^= 1;
    assert_eq!(
        verify(&value),
        Err(ProgramBundleError::Evidence(
            ProtocolEvidenceError::StateProof
        ))
    );
    let mut root = ordinary_bundle(&fixture);
    root.commitment.proof_ops[1] = other.proof.state.commitment.proof_ops[1].clone();
    assert_eq!(
        verify(&root),
        Err(ProgramBundleError::Evidence(
            ProtocolEvidenceError::StateProof
        ))
    );
    let mut record = ordinary_bundle(&fixture);
    record.program_record_ops = other.proof.state.state.ops;
    assert!(matches!(
        verify(&record),
        Err(ProgramBundleError::Evidence(_))
    ));
    let mut record_proof = ordinary_bundle(&fixture);
    record_proof.program_record_ops[0].data[0] ^= 1;
    assert!(matches!(
        verify(&record_proof),
        Err(ProgramBundleError::Evidence(_))
    ));
    let mut no_status = ordinary_bundle(&fixture);
    no_status.wind_down = None;
    assert_eq!(
        verify(&no_status),
        Err(ProgramBundleError::Evidence(
            ProtocolEvidenceError::LifecycleProof
        ))
    );
    let mut no_neighbours = ordinary_bundle(&fixture);
    no_neighbours
        .wind_down
        .as_mut()
        .expect("wind-down proof")
        .ops
        .clear();
    assert!(matches!(
        verify(&no_neighbours),
        Err(ProgramBundleError::Evidence(_))
    ));
    let mut forged_neighbour = ordinary_bundle(&fixture);
    forged_neighbour
        .wind_down
        .as_mut()
        .expect("wind-down proof")
        .ops = fixture.interface_witness.proof;
    assert!(matches!(
        verify(&forged_neighbour),
        Err(ProgramBundleError::Evidence(_))
    ));
}

#[test]
fn stale_chain_head_and_stale_bundle_are_refused() {
    let fixture = deployed(70, WASM_V1);
    let verifier = verifier(&fixture);
    let state = &fixture.proof.state;
    for now in [OBSERVED_AT - 1_001, OBSERVED_AT + STALENESS_MS + 1] {
        assert_eq!(
            verifier.verify_current_chain_head(
                ProgramHeadKind::Ordinary,
                &state.commitment,
                &state.transaction,
                &state.transaction_proof,
                now
            ),
            Err(ProtocolEvidenceError::Stale)
        );
    }
    let chain = ordinary_chain(&verifier, &fixture);
    assert_eq!(
        verifier.verify_current_program_bundle(
            &ordinary_bundle(&fixture),
            &chain,
            program(),
            OBSERVED_AT + STALENESS_MS + 1
        ),
        Err(ProgramBundleError::Evidence(ProtocolEvidenceError::Stale))
    );
}

#[test]
fn native_maintenance_head_is_typed_tagged_and_equal_to_the_protocol_api() {
    let fixture = deployed(70, WASM_V1);
    let verifier = verifier(&fixture);
    let state = &fixture.proof.state;
    let module_prefix = programs_state_key(&[]);
    let kernel_key = state
        .commitment
        .key
        .strip_prefix(module_prefix.as_slice())
        .expect("programs module prefix");
    let proof = ProtocolHeadProof {
        commitment: &state.commitment,
        chain: ChainProtocolHeadProof {
            ops: &state.commitment.proof_ops,
            kernel_key,
            value: state.commitment.value.as_ref().expect("program record"),
            store_name: "layerx",
            verifier: &fixture.chain.verifier,
            header: &fixture.chain.header,
            commit: &fixture.chain.commit,
        },
        transaction: &state.transaction,
        transaction_proof: &state.transaction_proof,
    };
    let protocol = verifier
        .verify_current_protocol_head(&proof, NOW)
        .expect("protocol head");
    let chain = chain_head(&verifier, &fixture, ProgramHeadKind::Maintenance);
    assert_eq!(chain.head_kind(), ProgramHeadKind::Maintenance);
    let sha: [u8; 32] = Sha256::digest(&state.transaction).into();
    assert_eq!(
        chain.receipt_evidence_digest(),
        ReceiptEvidenceDigest::MaintenanceReceiptSha256(sha)
    );
    assert_eq!(chain.receipt_digest(), protocol.receipt_digest());
    assert_eq!(chain.state_root(), protocol.state_root());
    assert_eq!(
        fixture.chain.header.hash().expect("header hash"),
        ChainHeader::decode(&proof.commitment.signed_header)
            .expect("protocol committed header")
            .hash()
            .expect("protocol header hash")
    );
    assert_eq!(chain.freshness(), protocol.freshness());
    assert_eq!(
        chain.freshness().observed_sequence,
        u64::try_from(fixture.chain.header.height).expect("height")
    );
}

#[test]
fn maintenance_head_refuses_wrong_transaction_signature_and_staleness() {
    let fixture = deployed(70, WASM_V1);
    let verifier = verifier(&fixture);
    let state = &fixture.proof.state;
    let head = |commitment: &_, transaction: &[u8], transaction_proof: &_, now| {
        verifier.verify_current_chain_head(
            ProgramHeadKind::Maintenance,
            commitment,
            transaction,
            transaction_proof,
            now,
        )
    };
    let mut wrong_leaf = state.transaction_proof.clone();
    wrong_leaf.index = 1;
    assert!(head(&state.commitment, &state.transaction, &wrong_leaf, NOW).is_err());
    let mut signature = state.commitment.clone();
    let last = signature.commit.len() - 1;
    signature.commit[last] ^= 1;
    assert!(head(
        &signature,
        &state.transaction,
        &state.transaction_proof,
        NOW
    )
    .is_err());
    let mut transaction = state.transaction.clone();
    let last = transaction.len() - 1;
    transaction[last] ^= 1;
    assert!(head(
        &state.commitment,
        &transaction,
        &state.transaction_proof,
        NOW
    )
    .is_err());
    assert_eq!(
        head(
            &state.commitment,
            &state.transaction,
            &state.transaction_proof,
            OBSERVED_AT + STALENESS_MS + 1
        ),
        Err(ProtocolEvidenceError::Stale)
    );
}

#[test]
fn ordinary_bundle_is_refused_against_an_unrelated_maintenance_chain_head() {
    let fixture = deployed(70, WASM_V1);
    let maintenance = deployed(71, WASM_V1);
    let verifier = verifier(&fixture);
    let chain = chain_head(&verifier, &maintenance, ProgramHeadKind::Maintenance);
    assert_eq!(chain.head_kind(), ProgramHeadKind::Maintenance);
    assert_eq!(
        verifier.verify_current_program_bundle(&ordinary_bundle(&fixture), &chain, program(), NOW),
        Err(ProgramBundleError::ChainHeadMismatch)
    );
}
