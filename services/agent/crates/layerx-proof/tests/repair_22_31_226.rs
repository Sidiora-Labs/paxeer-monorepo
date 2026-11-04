//! Task 22.31.226: guarantor-evidence-reaches-committed is covered by
//! guarantor_only_success_cannot_yield_committed and native_evidence_stops_at_receipted.
//! Baseline verification was not run: the task's gate order permits verification only
//! through hf-done after the complete implementation and hf-build.
//! Wiring request (domain 9): migrate services/agent/crates/layerx-proof/tests/checkpoint.rs
//! from the retired certificate verifiers to refusal and receipted-evidence assertions.
//! Wiring request (domain 9): remove the ThresholdReport cache consumer in
//! services/agent/crates/layerx-agentd/src/cache.rs; use receipt/inclusion evidence.
//! Wiring request (domain 9): reconcile the retired certificate-verifier, ThresholdReport
//! and checkpoint-evidence rows in spec/layerx-merge/interfaces.kvx.
//! Continuation: the prior build failed because checkpoint.rs still imports both
//! verifier names; the retained interfaces now explicitly refuse every certificate.
//! ThresholdReport is uninhabited and has no Evidence path; the legacy checkpoint
//! success assertions still require migration by domain 9 outside this task's scope.

use std::collections::BTreeSet;
use std::path::Path;

use layerx_crypto::secp256k1;
use layerx_proof::checkpoint::{
    checkpoint_id, Attestation, Certificate, Checkpoint, CheckpointError,
};
use layerx_proof::evidence::Evidence;
use layerx_proof::export_codec::{
    ArtifactDecodeError, CompleteOfflineArtifact, ExportCodecError, FactSelector, HeaderRecord,
    InclusionRecord, InclusionRecordKind, ProofRecord,
};
use layerx_proof::inclusion::{
    verify_activity, verify_header, verify_receipt, InclusionError, SequencerAuthorization,
};
use layerx_proof::level::{achieved, achieved_rung};
use layerx_proof::merkle::{build_proof, Proof};
use layerx_proof::receipt::{verify_outcome, AuthorizedBatch};
use layerx_proof::settlement::{declared_domain, maximum_attestation_delay_ms};
use layerx_types::json::parse;
use layerx_types::rung::Rung;
use layerx_types::vectors::checkpoint::{
    checkpoint_vector_path, load_checkpoint_vector, CheckpointOutcome,
};
use layerx_types::verify::VerificationLevel;
use layerx_wire::hash::{batch_header_digest, checkpoint_attestation_digest};
use layerx_wire::receipt::{decode, decode_batch_header, decode_merkle_proof};

const RECEIPT: &[u8] =
    include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/receipt");
const ACTIVITY: &[u8] =
    include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/activity.lxa");
const HEADER: &[u8] =
    include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/header");
const SIGNATURE: [u8; 64] =
    *include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/header.signature");
const KEY: [u8; 32] =
    *include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/sequencer.public");
const NO_RECORDS: [Vec<u8>; 0] = [];

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("repair 22.31.226 fixture: {error:?}"))
}

fn assert_rung(evidence: &Evidence, expected: Rung) {
    assert_eq!(evidence.rung(), expected);
    assert_eq!(achieved_rung(evidence), expected);
    assert_eq!(achieved(evidence), evidence.level());
    assert!(evidence.rung() <= Rung::Receipted);
    assert!(evidence.level() <= VerificationLevel::STATE_PROVEN);
    assert_ne!(evidence.level().rung(), Some(Rung::Committed));
}

#[test]
fn guarantor_only_success_cannot_yield_committed() {
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
    let path = checkpoint_vector_path(&root, "fresh");
    let vector = must(load_checkpoint_vector(&path, "fresh"));
    assert_eq!(vector.outcome, CheckpointOutcome::Accept);
    let domain = must(declared_domain(&vector.settlement_domain));
    let checkpoint = Checkpoint::new(vector.header.bytes.clone(), vector.validity_proof.clone());
    assert_eq!(must(checkpoint_id(&checkpoint)), vector.expected_digest);
    let mut signers = BTreeSet::new();
    let mut attestations = Vec::new();
    for signed in &vector.attestations {
        assert!(signers.insert(signed.guarantor_id));
        let key = domain
            .guarantor_set()
            .iter()
            .find(|key| key.guarantor_id() == signed.guarantor_id)
            .unwrap_or_else(|| panic!("published guarantor must be bonded"));
        assert!(key.bonded());
        let attestation = Attestation::new(
            vector.header.protocol_version,
            vector.header.network_id,
            domain.settlement().paxeer_chain_id(),
            domain.settlement().settlement_contract(),
            vector.header.epoch,
            vector.expected_digest,
            vector.expected_digest,
            signed.guarantor_id,
            vector.header.batch_number,
            vector.header.data_availability_root,
            signed.replayed,
            signed.data_possessed,
            signed.availability_class_mask,
            signed.attested_at_ms,
            signed.signer,
            signed.signature,
            signed.signature_v,
        );
        assert_eq!(attestation.canonical_statement(), signed.message);
        assert_eq!(
            must(checkpoint_attestation_digest(&signed.message)),
            signed.digest
        );
        assert_eq!(
            must(secp256k1::evm_address(&key.public_key())),
            signed.signer
        );
        must(secp256k1::verify_recoverable_digest(
            &key.public_key(),
            &signed.signature,
            signed.signature_v,
            &signed.digest,
        ));
        assert!(signed.replayed && signed.data_possessed);
        assert_eq!(signed.availability_class_mask, 0x1f);
        assert!(signed.attested_at_ms >= vector.header.timestamp_ms);
        assert!(
            signed.attested_at_ms - vector.header.timestamp_ms
                <= must(maximum_attestation_delay_ms())
        );
        attestations.push(attestation);
    }
    assert_eq!(signers.len(), 3);
    assert_eq!(vector.threshold, domain.certificate_threshold());
    assert!(signers.len() >= vector.threshold);

    for reference in [None, Some(b"registered-checkpoint".to_vec())] {
        let certificate = Certificate::new(
            checkpoint.clone(),
            attestations.clone(),
            vector.threshold,
            reference,
        );
        // Drive the actual legacy consumers with the authenticated producer's
        // complete certificate, both with and without a matching registration.
        assert_eq!(
            layerx_proof::checkpoint::verify_certificate(
                &certificate,
                domain.guarantor_set(),
                &vector.expected_digest,
                domain.settlement(),
                certificate.settlement_reference(),
            ),
            Err(CheckpointError::RetiredCertificateVerifier),
        );
        assert_eq!(
            layerx_proof::checkpoint::verify_declared_certificate(
                &certificate,
                &vector.settlement_domain,
                &vector.expected_digest,
                certificate.settlement_reference(),
            ),
            Err(CheckpointError::RetiredCertificateVerifier),
        );
        let authorization = SequencerAuthorization::new(
            vector.header.sequencer_id,
            KEY,
            vector.header.batch_number,
            vector.header.batch_number,
        );
        for attestation in certificate.attestations() {
            assert_eq!(
                verify_header(
                    certificate.checkpoint().header_bytes(),
                    &attestation.signature(),
                    &authorization,
                ),
                Err(InclusionError::HeaderSignature),
            );
        }
    }

    // The published, genuinely signed guarantor fixture is refused at ingress,
    // before any evidence or write rung can be produced from its certificate.
    let published_certificate = must(std::fs::read(path));
    assert_eq!(
        CompleteOfflineArtifact::decode(
            &[FactSelector::Receipt {
                activity_id: vector.expected_digest
            }
            .canonical_text()],
            &NO_RECORDS,
            &NO_RECORDS,
            &[published_certificate],
            &NO_RECORDS,
        ),
        Err(ArtifactDecodeError::Record {
            bucket: "certificates",
            index: 0,
            error: ExportCodecError::Tag,
        }),
    );
}

#[test]
fn native_evidence_stops_at_receipted() {
    let header = must(decode_batch_header(HEADER));
    let authorization = SequencerAuthorization::new(
        header.sequencer_id(),
        KEY,
        header.batch_number(),
        header.batch_number(),
    );
    let receipt = must(decode(RECEIPT));
    let protocol = receipt
        .protocol()
        .unwrap_or_else(|| panic!("native protocol receipt"));
    let batch = AuthorizedBatch::new(
        protocol.batch_id(),
        protocol.asset(),
        protocol.previous_state_root(),
        protocol.resulting_state_root(),
        KEY,
    );
    let verified = must(verify_outcome(RECEIPT, &batch));
    assert_eq!(verified.canonical_bytes(), RECEIPT);
    assert_rung(verified.evidence(), Rung::Submitted);

    let captured = must(decode_merkle_proof(include_bytes!(
        "../../../../../tests/fixtures/asset/bound-native-withdrawal/receipt.proof"
    )));
    let proof = must(Proof::new(
        captured.leaf_index(),
        captured.leaf_count(),
        captured.siblings().to_vec(),
    ));
    let header_record = HeaderRecord {
        canonical_header: HEADER.to_vec(),
        signature: SIGNATURE,
        sequencer_id: header.sequencer_id(),
        public_key: KEY,
        first_batch: header.batch_number(),
        last_batch: header.batch_number(),
    };
    let proof_record = ProofRecord::Inclusion(InclusionRecord {
        kind: InclusionRecordKind::Receipt,
        reference: FactSelector::Receipt {
            activity_id: protocol.activity_id(),
        },
        canonical_leaf: RECEIPT.to_vec(),
        proof,
        header_digest: must(batch_header_digest(HEADER)),
    });
    let transported_header = must(HeaderRecord::decode(&must(header_record.encode())));
    let transported_proof = must(ProofRecord::decode(&must(proof_record.encode())));
    let ProofRecord::Inclusion(included) = transported_proof else {
        panic!("receipt inclusion record required");
    };
    let inclusion = must(verify_receipt(
        &included.canonical_leaf,
        &included.proof,
        &transported_header.canonical_header,
        &transported_header.signature,
        &authorization,
    ));
    assert_eq!(
        inclusion.evidence().proof_root(),
        Some(header.receipt_merkle_root())
    );
    assert_rung(inclusion.evidence(), Rung::Receipted);

    let (activity_proof, root) = must(build_proof(&[ACTIVITY], 0));
    assert_eq!(root, header.activity_merkle_root());
    let activity = must(verify_activity(
        ACTIVITY,
        &activity_proof,
        HEADER,
        &SIGNATURE,
        &authorization,
    ));
    assert_rung(activity.evidence(), Rung::Receipted);
}

#[test]
fn pinned_native_sequencer_receipt_cannot_yield_committed() {
    let document = must(parse(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )));
    let case = &must(document.array_at("cases"))[0];
    assert_eq!(must(case.str_at("id")), "V1-pax-transfer");
    assert_eq!(must(case.str_at("expected.verifier")), "accepts");
    let bytes = must(case.hex_at("signedReceipt"));
    let receipt = must(decode(&bytes));
    let protocol = receipt
        .protocol()
        .unwrap_or_else(|| panic!("sequencer protocol receipt"));
    let pins = [SequencerAuthorization::new(
        must(case.hex_array_at("pinnedKey.keyId")),
        must(case.hex_array_at("pinnedKey.publicKey")),
        must(case.u64_at("pinnedKey.firstHeight")),
        must(case.u64_at("pinnedKey.lastHeight")),
    )];
    let batch = must(AuthorizedBatch::from_receipt_keys(
        &bytes,
        &pins,
        protocol.asset(),
        protocol.previous_state_root(),
        protocol.resulting_state_root(),
    ));
    let verified = must(verify_outcome(&bytes, &batch));
    assert_eq!(verified.canonical_bytes(), bytes);
    assert_rung(verified.evidence(), Rung::Receipted);
}
