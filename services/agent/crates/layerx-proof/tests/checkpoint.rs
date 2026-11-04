use std::fs;
use std::path::{Path, PathBuf};

use k256::ecdsa::{Signature, SigningKey};
use layerx_crypto::secp256k1;
use layerx_proof::checkpoint::{
    checkpoint_id, verify_certificate, verify_declared_certificate, Attestation, Certificate,
    Checkpoint, CheckpointError, GuarantorKey, SettlementDomain,
};
use layerx_proof::level::achieved_rung;
use layerx_proof::settlement::{
    declared, declared_domain, maximum_attestation_delay_ms, DeclaredDomain,
    DECLARED_CHECKPOINT_SETTLEMENT,
};
use layerx_types::rung::Rung;
use layerx_types::settlement::{
    DeclaredCheckpointSettlement, SettlementError, CHECKPOINT_SETTLEMENT_PATH,
    CHECKPOINT_SETTLEMENT_SCHEMA,
};
use layerx_types::vectors::checkpoint::{
    load_checkpoint_vectors, CheckpointOutcome, CheckpointRejection, CheckpointVector,
    CHECKPOINT_VECTOR_CASES,
};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::checkpoint_attestation_digest;
use layerx_wire::limits::PROTOCOL_VERSION;

fn header_bytes() -> Vec<u8> {
    let mut encoder = Encoder::new(354);
    assert_eq!(
        encoder.structure_header_version(0x1701, PROTOCOL_VERSION),
        Ok(())
    );
    assert_eq!(encoder.u8(15), Ok(()));
    for field in 1..=15 {
        assert_eq!(encoder.tag(field, 15), Ok(()));
        match field {
            1 => assert_eq!(encoder.u16(PROTOCOL_VERSION), Ok(())),
            2 => assert_eq!(encoder.u32(42), Ok(())),
            3 => assert_eq!(encoder.u64(7), Ok(())),
            4 => assert_eq!(encoder.u64(8), Ok(())),
            5 => assert_eq!(encoder.u64(11), Ok(())),
            6 => assert_eq!(encoder.u64(19), Ok(())),
            7..=13 => assert_eq!(encoder.bytes(&[field; 32], 32), Ok(())),
            14 => assert_eq!(encoder.u64(1_000), Ok(())),
            15 => assert_eq!(encoder.bytes(&[15; 32], 32), Ok(())),
            _ => panic!("unreachable header field"),
        }
    }
    encoder.finish()
}

fn key(value: u8) -> (SigningKey, [u8; 33], [u8; 32]) {
    let mut scalar = [0_u8; 32];
    scalar[31] = value;
    let signing = SigningKey::from_bytes((&scalar).into())
        .unwrap_or_else(|error| panic!("invalid signing key: {error}"));
    let encoded = signing.verifying_key().to_encoded_point(true);
    let public_key: [u8; 33] = encoded
        .as_bytes()
        .try_into()
        .unwrap_or_else(|_| panic!("invalid compressed key width"));
    let mut id = [0_u8; 32];
    id[0] = value;
    (signing, public_key, id)
}

fn repository_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..")
}

fn attestation(
    checkpoint_id: [u8; 32],
    guarantor_id: [u8; 32],
    signing_key: &SigningKey,
) -> Attestation {
    attestation_at(
        checkpoint_id,
        guarantor_id,
        signing_key,
        1_000 + u64::from(guarantor_id[0]),
    )
}

fn attestation_at(
    checkpoint_id: [u8; 32],
    guarantor_id: [u8; 32],
    signing_key: &SigningKey,
    attested_at_ms: u64,
) -> Attestation {
    let settlement_contract = [0x55; 20];
    let mut message = [0_u8; 189];
    message[..2].copy_from_slice(&PROTOCOL_VERSION.to_be_bytes());
    message[2..6].copy_from_slice(&42_u32.to_be_bytes());
    message[6..14].copy_from_slice(&31_337_u64.to_be_bytes());
    message[14..34].copy_from_slice(&settlement_contract);
    message[34..42].copy_from_slice(&7_u64.to_be_bytes());
    message[42..74].copy_from_slice(&checkpoint_id);
    message[74..106].copy_from_slice(&checkpoint_id);
    message[106..138].copy_from_slice(&guarantor_id);
    message[138..146].copy_from_slice(&8_u64.to_be_bytes());
    message[146..178].copy_from_slice(&[12; 32]);
    message[178] = 1;
    message[179] = 1;
    message[180] = 0x1f;
    message[181..].copy_from_slice(&attested_at_ms.to_be_bytes());
    let digest = checkpoint_attestation_digest(&message)
        .unwrap_or_else(|error| panic!("attestation hash failed: {error:?}"));
    let (signature, recovery_id): (Signature, _) = signing_key
        .sign_prehash_recoverable(&digest)
        .unwrap_or_else(|error| panic!("attestation signing failed: {error}"));
    let signer = secp256k1::evm_address(
        signing_key
            .verifying_key()
            .to_encoded_point(true)
            .as_bytes(),
    )
    .unwrap_or_else(|error| panic!("attestation signer: {error:?}"));
    Attestation::new(
        PROTOCOL_VERSION,
        42,
        31_337,
        settlement_contract,
        7,
        checkpoint_id,
        checkpoint_id,
        guarantor_id,
        8,
        [12; 32],
        true,
        true,
        0x1f,
        attested_at_ms,
        signer,
        signature.to_bytes().into(),
        27 + u8::from(recovery_id),
    )
}

fn vector_certificate(vector: &CheckpointVector, domain: &DeclaredDomain) -> Certificate {
    let attestations = vector
        .attestations
        .iter()
        .map(|attestation| {
            Attestation::new(
                vector.header.protocol_version,
                vector.header.network_id,
                domain.settlement().paxeer_chain_id(),
                domain.settlement().settlement_contract(),
                vector.header.epoch,
                vector.expected_digest,
                vector.expected_digest,
                attestation.guarantor_id,
                vector.header.batch_number,
                vector.header.data_availability_root,
                attestation.replayed,
                attestation.data_possessed,
                attestation.availability_class_mask,
                attestation.attested_at_ms,
                attestation.signer,
                attestation.signature,
                attestation.signature_v,
            )
        })
        .collect();
    Certificate::new(
        Checkpoint::new(vector.header.bytes.clone(), vector.validity_proof.clone()),
        attestations,
        vector.threshold,
        None,
    )
}

fn fixture_with_settlement(
    settlement_reference: Option<Vec<u8>>,
) -> (Certificate, Vec<GuarantorKey>, [u8; 32]) {
    let checkpoint = Checkpoint::new(header_bytes(), b"PROOF".to_vec());
    let identifier = checkpoint_id(&checkpoint)
        .unwrap_or_else(|error| panic!("checkpoint hash failed: {error:?}"));
    let mut attestations = Vec::new();
    let mut keys = Vec::new();
    for value in 1..=3 {
        let (signing, public, id) = key(value);
        attestations.push(attestation(identifier, id, &signing));
        keys.push(GuarantorKey::new(id, public, true));
    }
    (
        Certificate::new(checkpoint, attestations, 2, settlement_reference),
        keys,
        identifier,
    )
}

fn fixture() -> (Certificate, Vec<GuarantorKey>, [u8; 32]) {
    fixture_with_settlement(None)
}

fn settlement_domain() -> SettlementDomain {
    SettlementDomain::new(31_337, [0x55; 20])
}

#[test]
fn transported_registration_never_invents_paxeer_chain_finality() {
    for reference in [None, Some(b"paxeer-registered-1".to_vec())] {
        let (certificate, keys, identifier) = fixture_with_settlement(reference);
        let header =
            layerx_wire::receipt::decode_batch_header(certificate.checkpoint().header_bytes())
                .unwrap_or_else(|error| panic!("canonical header: {error:?}"));
        assert_eq!(header.protocol_version(), PROTOCOL_VERSION);
        assert_eq!(header.network_id(), 42);
        assert_eq!(header.batch_number(), 8);
        assert_eq!((header.first_sequence(), header.last_sequence()), (11, 19));
        assert_eq!(header.resulting_state_root(), [8; 32]);
        assert_eq!(header.activity_merkle_root(), [9; 32]);
        assert_eq!(header.receipt_merkle_root(), [10; 32]);
        assert_eq!(header.event_merkle_root(), [11; 32]);
        assert_eq!(header.data_availability_root(), [12; 32]);
        assert_eq!(header.oracle_root(), [13; 32]);
        assert_eq!(certificate.attestations().len(), 3);
        assert_eq!(certificate.threshold(), 2);
        for (attestation, key) in certificate.attestations().iter().zip(&keys) {
            let digest = checkpoint_attestation_digest(&attestation.canonical_statement())
                .unwrap_or_else(|error| panic!("attestation digest: {error:?}"));
            assert_eq!(attestation.guarantor_id(), key.guarantor_id());
            assert!(key.bonded());
            assert_eq!(
                secp256k1::verify_recoverable_digest(
                    &key.public_key(),
                    &attestation.signature(),
                    attestation.signature_v(),
                    &digest,
                ),
                Ok(())
            );
        }
        for registered in [
            certificate.settlement_reference(),
            None,
            Some(b"different-reference".as_slice()),
        ] {
            assert_eq!(
                verify_certificate(
                    &certificate,
                    &keys,
                    &identifier,
                    settlement_domain(),
                    registered
                ),
                Err(CheckpointError::RetiredCertificateVerifier)
            );
        }
        assert_eq!(
            verify_declared_certificate(
                &certificate,
                "vectors",
                &identifier,
                certificate.settlement_reference()
            ),
            Err(CheckpointError::RetiredCertificateVerifier)
        );
    }
}

#[test]
fn rejects_threshold_duplicate_membership_and_signature_failures() {
    let (_, keys, identifier) = fixture();
    let (signing, _, first_id) = key(1);
    let duplicate = attestation(identifier, first_id, &signing);
    let duplicate_certificate = Certificate::new(
        Checkpoint::new(header_bytes(), b"PROOF".to_vec()),
        vec![duplicate.clone(), duplicate],
        2,
        None,
    );
    assert_eq!(
        verify_certificate(
            &duplicate_certificate,
            &keys,
            &identifier,
            settlement_domain(),
            None
        ),
        Err(CheckpointError::RetiredCertificateVerifier)
    );

    let (outsider_signing, _, outsider_id) = key(9);
    let outsider = Certificate::new(
        Checkpoint::new(header_bytes(), b"PROOF".to_vec()),
        vec![attestation(identifier, outsider_id, &outsider_signing)],
        1,
        None,
    );
    assert_eq!(
        verify_certificate(&outsider, &keys, &identifier, settlement_domain(), None),
        Err(CheckpointError::RetiredCertificateVerifier)
    );

    let one_signature = Certificate::new(
        Checkpoint::new(header_bytes(), b"PROOF".to_vec()),
        vec![attestation(identifier, first_id, &signing)],
        2,
        None,
    );
    assert_eq!(
        verify_certificate(
            &one_signature,
            &keys,
            &identifier,
            settlement_domain(),
            None
        ),
        Err(CheckpointError::RetiredCertificateVerifier)
    );

    let bad_signature = Certificate::new(
        Checkpoint::new(header_bytes(), b"PROOF".to_vec()),
        vec![Attestation::new(
            PROTOCOL_VERSION,
            42,
            31_337,
            [0x55; 20],
            7,
            identifier,
            identifier,
            first_id,
            8,
            [12; 32],
            true,
            true,
            0x1f,
            1_001,
            [1; 20],
            [1; 64],
            27,
        )],
        1,
        None,
    );
    assert_eq!(
        verify_certificate(
            &bad_signature,
            &keys,
            &identifier,
            settlement_domain(),
            None
        ),
        Err(CheckpointError::RetiredCertificateVerifier)
    );
}

#[test]
fn rejects_checkpoint_identifier_and_settlement_domain_failures() {
    let (certificate, keys, identifier) = fixture();
    assert_eq!(
        verify_certificate(&certificate, &keys, &[99; 32], settlement_domain(), None),
        Err(CheckpointError::RetiredCertificateVerifier)
    );
    assert_eq!(
        verify_certificate(
            &certificate,
            &keys,
            &identifier,
            SettlementDomain::new(31_338, [0x55; 20]),
            None,
        ),
        Err(CheckpointError::RetiredCertificateVerifier)
    );
    assert_eq!(
        verify_certificate(
            &certificate,
            &keys,
            &identifier,
            SettlementDomain::new(31_337, [0x56; 20]),
            None,
        ),
        Err(CheckpointError::RetiredCertificateVerifier)
    );
    assert_eq!(
        verify_certificate(
            &certificate,
            &keys,
            &identifier,
            SettlementDomain::new(0, [0; 20]),
            None,
        ),
        Err(CheckpointError::RetiredCertificateVerifier)
    );
}

#[test]
fn embedded_settlement_is_the_declared_document() {
    let on_disk = fs::read_to_string(repository_root().join(CHECKPOINT_SETTLEMENT_PATH))
        .unwrap_or_else(|error| panic!("declared settlement unreadable: {error}"));
    assert_eq!(on_disk, DECLARED_CHECKPOINT_SETTLEMENT);
    let parsed = DeclaredCheckpointSettlement::parse(&on_disk).unwrap_or_else(|error| {
        panic!("{CHECKPOINT_SETTLEMENT_PATH} must satisfy {CHECKPOINT_SETTLEMENT_SCHEMA}: {error}")
    });
    let embedded =
        declared().unwrap_or_else(|error| panic!("embedded settlement invalid: {error}"));
    assert_eq!(*embedded, parsed);
    assert_eq!(
        maximum_attestation_delay_ms(),
        Ok(parsed.finality_policy().maximum_attestation_delay_seconds() * 1_000)
    );
    assert_eq!(
        declared_domain("undeclared").map(|domain| domain.name().to_owned()),
        Err(SettlementError::UnknownDomain("undeclared".to_owned()))
    );
    let vectors = declared_domain("vectors")
        .unwrap_or_else(|error| panic!("vectors domain undeclared: {error}"));
    assert_eq!(vectors.network_id(), 42);
    assert_eq!(vectors.settlement().paxeer_chain_id(), 31_337);
    assert_eq!(vectors.guarantor_set().len(), 3);
    assert_eq!(
        vectors.certificate_threshold(),
        parsed.finality_policy().certificate_threshold()
    );
}

#[test]
fn checkpoint_vectors_apply_the_declared_freshness_window() {
    let vectors = load_checkpoint_vectors(&repository_root())
        .unwrap_or_else(|error| panic!("checkpoint vectors failed to load: {error:?}"));
    assert_eq!(vectors.len(), CHECKPOINT_VECTOR_CASES.len());
    let maximum_delay_ms = maximum_attestation_delay_ms()
        .unwrap_or_else(|error| panic!("declared delay unavailable: {error}"));
    for vector in &vectors {
        let domain = declared_domain(&vector.settlement_domain)
            .unwrap_or_else(|error| panic!("{}: {error}", vector.case_name));
        let certificate = vector_certificate(vector, &domain);
        assert_eq!(
            checkpoint_id(certificate.checkpoint()),
            Ok(vector.expected_digest)
        );
        for (attestation, captured) in certificate.attestations().iter().zip(&vector.attestations) {
            assert_eq!(attestation.canonical_statement(), captured.message);
            let key = domain
                .guarantor_set()
                .iter()
                .find(|key| key.guarantor_id() == captured.guarantor_id)
                .unwrap_or_else(|| panic!("recorded guarantor membership"));
            assert_eq!(
                checkpoint_attestation_digest(&captured.message),
                Ok(captured.digest)
            );
            assert_eq!(
                secp256k1::verify_recoverable_digest(
                    &key.public_key(),
                    &captured.signature,
                    captured.signature_v,
                    &captured.digest,
                ),
                Ok(())
            );
        }
        let first = &vector.attestations[0];
        match vector.outcome {
            CheckpointOutcome::Accept => {
                assert!(vector.attestations.len() >= vector.threshold);
                for attestation in &vector.attestations {
                    assert!((vector.header.timestamp_ms
                        ..=vector.header.timestamp_ms + maximum_delay_ms)
                        .contains(&attestation.attested_at_ms));
                }
            }
            CheckpointOutcome::Reject(CheckpointRejection::NotYetValid) => {
                assert!(first.attested_at_ms < vector.header.timestamp_ms)
            }
            CheckpointOutcome::Reject(CheckpointRejection::Expired) => {
                assert!(first.attested_at_ms > vector.header.timestamp_ms + maximum_delay_ms)
            }
        }
        assert_eq!(
            verify_declared_certificate(
                &certificate,
                &vector.settlement_domain,
                &vector.expected_digest,
                None
            ),
            Err(CheckpointError::RetiredCertificateVerifier),
            "{}",
            vector.case_name
        );
        assert_eq!(
            verify_certificate(
                &certificate,
                domain.guarantor_set(),
                &vector.expected_digest,
                domain.settlement(),
                None
            ),
            Err(CheckpointError::RetiredCertificateVerifier),
            "{}",
            vector.case_name
        );
    }
}

#[test]
fn freshness_window_is_header_relative_and_closed_on_both_ends() {
    let maximum_delay_ms = maximum_attestation_delay_ms()
        .unwrap_or_else(|error| panic!("declared delay unavailable: {error}"));
    let (certificate, keys, identifier) = fixture();
    let (signing, _, first_id) = key(1);
    // The historical time boundaries remain recorded; no point in that window
    // can turn a guarantor signature into a committed block proof.
    for time in [
        999,
        1_000,
        1_000 + maximum_delay_ms,
        1_001 + maximum_delay_ms,
    ] {
        let certificate = Certificate::new(
            certificate.checkpoint().clone(),
            vec![attestation_at(identifier, first_id, &signing, time)],
            1,
            None,
        );
        assert_eq!(certificate.attestations()[0].attested_at_ms(), time);
        assert_eq!(
            verify_certificate(&certificate, &keys, &identifier, settlement_domain(), None),
            Err(CheckpointError::RetiredCertificateVerifier)
        );
    }
}

#[test]
fn declared_domain_verification_rejects_foreign_contracts_and_weak_thresholds() {
    let (certificate, _, identifier) = fixture();
    for domain in ["vectors", "undeclared"] {
        for threshold in [0, 1, 2, 4] {
            let candidate = Certificate::new(
                certificate.checkpoint().clone(),
                certificate.attestations().to_vec(),
                threshold,
                None,
            );
            assert_eq!(candidate.threshold(), threshold);
            assert_eq!(
                verify_declared_certificate(&candidate, domain, &identifier, None),
                Err(CheckpointError::RetiredCertificateVerifier)
            );
        }
    }
}

#[test]
fn retained_receipt_inclusion_achieves_receipted_without_committed_authority() {
    use layerx_proof::inclusion::{verify_receipt, SequencerAuthorization};
    use layerx_proof::merkle::Proof;
    use layerx_wire::receipt::{decode_batch_header, decode_merkle_proof};
    let receipt =
        include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/receipt");
    let header =
        include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/header");
    let signature = include_bytes!(
        "../../../../../tests/fixtures/asset/bound-native-withdrawal/header.signature"
    );
    let key = *include_bytes!(
        "../../../../../tests/fixtures/asset/bound-native-withdrawal/sequencer.public"
    );
    let header_fields = decode_batch_header(header).unwrap_or_else(|error| panic!("{error:?}"));
    let captured = decode_merkle_proof(include_bytes!(
        "../../../../../tests/fixtures/asset/bound-native-withdrawal/receipt.proof"
    ))
    .unwrap_or_else(|error| panic!("{error:?}"));
    let proof = Proof::new(
        captured.leaf_index(),
        captured.leaf_count(),
        captured.siblings().to_vec(),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let authorization = SequencerAuthorization::new(
        header_fields.sequencer_id(),
        key,
        header_fields.batch_number(),
        header_fields.batch_number(),
    );
    let verified = verify_receipt(receipt, &proof, header, signature, &authorization)
        .unwrap_or_else(|error| panic!("retained receipt proof: {error:?}"));
    assert_eq!(achieved_rung(verified.evidence()), Rung::Receipted);
    assert_ne!(achieved_rung(verified.evidence()), Rung::Committed);
    assert_eq!(
        verified.evidence().proof_root(),
        Some(header_fields.receipt_merkle_root())
    );
}
