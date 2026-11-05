//! Historical certificates are refused at the retained codec boundary.
//! Recorded signatures and freshness cases remain checked as archival data.

#[path = "support/historical_checkpoint.rs"]
mod historical;

use historical::{assert_refused, certificate_record, configuration, fresh, must, repository_root};
use k256::ecdsa::{Signature, SigningKey};
use layerx_crypto::secp256k1;
use layerx_proof::level::achieved_rung;
use layerx_types::rung::Rung;
use layerx_types::settlement::{
    DeclaredCheckpointSettlement, SettlementError, CHECKPOINT_SETTLEMENT_PATH,
};
use layerx_types::vectors::checkpoint::{
    load_checkpoint_vectors, CheckpointOutcome, CheckpointRejection, CheckpointVector,
    CHECKPOINT_VECTOR_CASES,
};
use layerx_wire::hash::{checkpoint_attestation_digest, checkpoint_id};
use layerx_wire::receipt::{decode_batch_header, encode_batch_header};

fn assert_recorded_signatures(vector: &CheckpointVector) {
    let configuration = configuration();
    let domain = must(configuration.domain(&vector.settlement_domain));
    for captured in &vector.attestations {
        let key = domain
            .guarantor_set()
            .iter()
            .find(|key| key.guarantor_id() == captured.guarantor_id)
            .expect("recorded guarantor membership");
        assert_eq!(
            must(checkpoint_attestation_digest(&captured.message)),
            captured.digest
        );
        assert_eq!(
            must(secp256k1::evm_address(&key.public_key())),
            captured.signer
        );
        assert_eq!(
            &captured.message[..2],
            &vector.header.protocol_version.to_be_bytes()
        );
        assert_eq!(
            &captured.message[2..6],
            &vector.header.network_id.to_be_bytes()
        );
        assert_eq!(
            &captured.message[6..14],
            &domain.paxeer_chain_id().to_be_bytes()
        );
        assert_eq!(&captured.message[14..34], &domain.settlement_contract());
        assert_eq!(&captured.message[42..74], &vector.expected_digest);
        assert_eq!(&captured.message[74..106], &vector.expected_digest);
        assert_eq!(&captured.message[106..138], &captured.guarantor_id);
        assert_eq!(
            &captured.message[181..],
            &captured.attested_at_ms.to_be_bytes()
        );
        must(secp256k1::verify_recoverable_digest(
            &key.public_key(),
            &captured.signature,
            captured.signature_v,
            &captured.digest,
        ));
    }
}

#[test]
fn transported_registration_never_invents_paxeer_chain_finality() {
    let vector = fresh();
    let header = must(decode_batch_header(&vector.header.bytes));
    assert_eq!(must(encode_batch_header(&header)), vector.header.bytes);
    assert_eq!(header.protocol_version(), vector.header.protocol_version);
    assert_eq!(header.network_id(), vector.header.network_id);
    assert_eq!(header.batch_number(), vector.header.batch_number);
    assert_eq!(
        (header.first_sequence(), header.last_sequence()),
        (vector.header.first_sequence, vector.header.last_sequence)
    );
    assert_eq!(
        header.resulting_state_root(),
        vector.header.resulting_state_root
    );
    assert_eq!(
        header.activity_merkle_root(),
        vector.header.activity_merkle_root
    );
    assert_eq!(
        header.receipt_merkle_root(),
        vector.header.receipt_merkle_root
    );
    assert_eq!(header.event_merkle_root(), vector.header.event_merkle_root);
    assert_eq!(
        header.data_availability_root(),
        vector.header.data_availability_root
    );
    assert_eq!(header.oracle_root(), vector.header.oracle_root);
    assert_eq!(
        must(checkpoint_id(&vector.header.bytes, &vector.validity_proof)),
        vector.expected_digest
    );
    assert_eq!(vector.attestations.len(), 3);
    assert_eq!(vector.threshold, 2);
    assert_recorded_signatures(&vector);
    for reference in [
        None,
        Some(b"paxeer-registered-1".as_slice()),
        Some(b"different-reference".as_slice()),
    ] {
        assert_refused(
            certificate_record(&vector, reference),
            vector.expected_digest,
        );
    }
}

#[test]
fn rejects_threshold_duplicate_membership_and_signature_failures() {
    let vector = fresh();
    let mut duplicate = vector.clone();
    duplicate.attestations = vec![vector.attestations[0].clone(); 2];
    assert_eq!(
        duplicate.attestations[0].guarantor_id,
        duplicate.attestations[1].guarantor_id
    );
    let mut outsider = vector.clone();
    outsider.attestations[0].guarantor_id = [0xff; 32];
    outsider.attestations[0].message[106..138].fill(0xff);
    let configuration = configuration();
    let domain = must(configuration.domain(&vector.settlement_domain));
    assert!(!domain
        .guarantor_set()
        .iter()
        .any(|key| key.guarantor_id() == outsider.attestations[0].guarantor_id));
    let mut insufficient = vector.clone();
    insufficient.attestations.truncate(1);
    assert!(insufficient.attestations.len() < insufficient.threshold);
    let mut bad_signature = vector.clone();
    bad_signature.attestations[0].signature = [1; 64];
    let key = domain
        .guarantor_set()
        .iter()
        .find(|key| key.guarantor_id() == vector.attestations[0].guarantor_id)
        .expect("recorded member");
    assert!(secp256k1::verify_recoverable_digest(
        &key.public_key(),
        &bad_signature.attestations[0].signature,
        vector.attestations[0].signature_v,
        &vector.attestations[0].digest
    )
    .is_err());
    for candidate in [duplicate, outsider, insufficient, bad_signature] {
        assert_refused(
            certificate_record(&candidate, None),
            candidate.expected_digest,
        );
    }
}

#[test]
fn rejects_checkpoint_identifier_and_settlement_domain_failures() {
    let vector = fresh();
    let mut wrong_id = vector.clone();
    wrong_id.expected_digest = [99; 32];
    assert_ne!(
        must(checkpoint_id(
            &wrong_id.header.bytes,
            &wrong_id.validity_proof
        )),
        wrong_id.expected_digest
    );
    assert_refused(
        certificate_record(&wrong_id, None),
        wrong_id.expected_digest,
    );
    for (chain, contract) in [(31_338_u64, [0x55; 20]), (31_337, [0x56; 20]), (0, [0; 20])] {
        let mut candidate = vector.clone();
        for item in &mut candidate.attestations {
            item.message[6..14].copy_from_slice(&chain.to_be_bytes());
            item.message[14..34].copy_from_slice(&contract);
        }
        assert_refused(
            certificate_record(&candidate, None),
            candidate.expected_digest,
        );
    }
}

#[test]
fn embedded_settlement_is_deprecated_and_archival_input_is_explicit() {
    let on_disk = must(std::fs::read_to_string(
        repository_root().join(CHECKPOINT_SETTLEMENT_PATH),
    ));
    let parsed = must(DeclaredCheckpointSettlement::parse(&on_disk));
    assert_eq!(configuration(), parsed);
    assert_eq!(
        parsed.finality_policy().maximum_attestation_delay_ms(),
        parsed.finality_policy().maximum_attestation_delay_seconds() * 1_000
    );
    assert_eq!(
        parsed
            .domain("undeclared")
            .map(|domain| domain.name().to_owned()),
        Err(SettlementError::UnknownDomain("undeclared".to_owned()))
    );
    let vectors = must(parsed.domain("vectors"));
    assert_eq!(vectors.network_id(), 42);
    assert_eq!(vectors.paxeer_chain_id(), 31_337);
    assert_eq!(vectors.guarantor_set().len(), 3);
    assert_eq!(parsed.finality_policy().certificate_threshold(), 2);
    historical::assert_deprecated(
        include_str!("../src/settlement.rs"),
        "pub const DECLARED_CHECKPOINT_SETTLEMENT:",
    );
    let vector = fresh();
    assert_refused(certificate_record(&vector, None), vector.expected_digest);
}

#[test]
fn checkpoint_vectors_apply_the_declared_freshness_window() {
    let vectors = must(load_checkpoint_vectors(&repository_root()));
    assert_eq!(vectors.len(), CHECKPOINT_VECTOR_CASES.len());
    let maximum_delay_ms = configuration()
        .finality_policy()
        .maximum_attestation_delay_ms();
    for vector in &vectors {
        assert_eq!(
            must(checkpoint_id(&vector.header.bytes, &vector.validity_proof)),
            vector.expected_digest
        );
        assert_recorded_signatures(vector);
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
                assert!(first.attested_at_ms < vector.header.timestamp_ms);
            }
            CheckpointOutcome::Reject(CheckpointRejection::Expired) => {
                assert!(first.attested_at_ms > vector.header.timestamp_ms + maximum_delay_ms);
            }
        }
        assert_refused(certificate_record(vector, None), vector.expected_digest);
    }
}

#[test]
fn freshness_window_is_header_relative_and_closed_on_both_ends() {
    let vector = fresh();
    let maximum_delay_ms = configuration()
        .finality_policy()
        .maximum_attestation_delay_ms();
    let start = vector.header.timestamp_ms;
    let mut scalar = [0; 32];
    scalar[31] = 1;
    let signing = must(SigningKey::from_bytes((&scalar).into()));
    let public = signing.verifying_key().to_encoded_point(true);
    for time in [
        start - 1,
        start,
        start + maximum_delay_ms,
        start + maximum_delay_ms + 1,
    ] {
        let mut candidate = vector.clone();
        candidate.threshold = 1;
        candidate.attestations.truncate(1);
        let attestation = &mut candidate.attestations[0];
        attestation.attested_at_ms = time;
        attestation.message[181..].copy_from_slice(&time.to_be_bytes());
        attestation.digest = must(checkpoint_attestation_digest(&attestation.message));
        let (signature, recovery): (Signature, _) =
            must(signing.sign_prehash_recoverable(&attestation.digest));
        attestation.signature = signature.to_bytes().into();
        attestation.signature_v = 27 + u8::from(recovery);
        attestation.signer = must(secp256k1::evm_address(public.as_bytes()));
        assert_eq!(attestation.attested_at_ms, time);
        must(secp256k1::verify_recoverable_digest(
            public.as_bytes(),
            &attestation.signature,
            attestation.signature_v,
            &attestation.digest,
        ));
        assert_refused(
            certificate_record(&candidate, None),
            candidate.expected_digest,
        );
    }
}

#[test]
fn declared_domain_verification_rejects_foreign_contracts_and_weak_thresholds() {
    let vector = fresh();
    for contract in [[0x55; 20], [0xff; 20]] {
        for threshold in [0, 1, 2, 4] {
            let mut candidate = vector.clone();
            candidate.threshold = threshold;
            for item in &mut candidate.attestations {
                item.message[14..34].copy_from_slice(&contract);
            }
            assert_eq!(candidate.threshold, threshold);
            assert_refused(
                certificate_record(&candidate, None),
                candidate.expected_digest,
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
