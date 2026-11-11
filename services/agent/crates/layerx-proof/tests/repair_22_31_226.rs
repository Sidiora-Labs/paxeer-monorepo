//! Task 22.31.226: guarantor-evidence-reaches-committed is covered by
//! `guarantor_only_success_cannot_yield_committed` and `native_evidence_stops_at_receipted`.
//! Task 22.33.1557 migrates retired API calls to the retained certificate-bucket
//! refusal while preserving recorded signature checks and receipt/inclusion coverage.

#[path = "support/historical_checkpoint.rs"]
pub(crate) mod historical;

use std::collections::BTreeSet;

use layerx_crypto::secp256k1;
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
use layerx_types::json::parse;
use layerx_types::rung::Rung;
use layerx_types::vectors::checkpoint::{checkpoint_vector_path, CheckpointOutcome};
use layerx_types::verify::VerificationLevel;
use layerx_wire::hash::{batch_header_digest, checkpoint_attestation_digest, checkpoint_id};
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
    let root = historical::repository_root();
    let path = checkpoint_vector_path(&root, "fresh");
    let vector = historical::fresh();
    assert_eq!(vector.outcome, CheckpointOutcome::Accept);
    let configuration = historical::configuration();
    let domain = configuration.domain(&vector.settlement_domain);
    assert_eq!(domain.network_id, vector.header.network_id);
    assert_eq!(
        must(checkpoint_id(&vector.header.bytes, &vector.validity_proof)),
        vector.expected_digest
    );
    let mut signers = BTreeSet::new();
    for signed in &vector.attestations {
        assert!(signers.insert(signed.guarantor_id));
        let key = domain
            .guarantor_set
            .iter()
            .find(|key| key.id == signed.guarantor_id)
            .unwrap_or_else(|| panic!("published guarantor must be bonded"));
        let mut statement = Vec::new();
        statement.extend_from_slice(&vector.header.protocol_version.to_be_bytes());
        statement.extend_from_slice(&vector.header.network_id.to_be_bytes());
        statement.extend_from_slice(&domain.paxeer_chain_id.to_be_bytes());
        statement.extend_from_slice(&domain.settlement_contract);
        statement.extend_from_slice(&vector.header.epoch.to_be_bytes());
        statement.extend_from_slice(&vector.expected_digest);
        statement.extend_from_slice(&vector.expected_digest);
        statement.extend_from_slice(&signed.guarantor_id);
        statement.extend_from_slice(&vector.header.batch_number.to_be_bytes());
        statement.extend_from_slice(&vector.header.data_availability_root);
        statement.extend_from_slice(&[
            u8::from(signed.replayed),
            u8::from(signed.data_possessed),
            signed.availability_class_mask,
        ]);
        statement.extend_from_slice(&signed.attested_at_ms.to_be_bytes());
        assert_eq!(statement, signed.message);
        assert_eq!(
            must(checkpoint_attestation_digest(&signed.message)),
            signed.digest
        );
        assert_eq!(must(secp256k1::evm_address(&key.public_key)), signed.signer);
        assert_eq!(key.signer, signed.signer);
        must(secp256k1::verify_recoverable_digest(
            &key.public_key,
            &signed.signature,
            signed.signature_v,
            &signed.digest,
        ));
        assert!(signed.replayed && signed.data_possessed);
        assert_eq!(signed.availability_class_mask, 0x1f);
        assert!(signed.attested_at_ms >= vector.header.timestamp_ms);
        assert!(
            signed.attested_at_ms - vector.header.timestamp_ms
                <= configuration.maximum_attestation_delay_ms
        );
    }
    assert_eq!(signers.len(), 3);
    assert_eq!(vector.threshold, configuration.certificate_threshold);
    assert!(signers.len() >= vector.threshold);

    for reference in [None, Some(b"registered-checkpoint".as_slice())] {
        historical::assert_refused(
            historical::certificate_record(&vector, reference),
            vector.expected_digest,
        );
        let authorization = SequencerAuthorization::new(
            vector.header.sequencer_id,
            KEY,
            vector.header.batch_number,
            vector.header.batch_number,
        );
        for attestation in &vector.attestations {
            assert_eq!(
                verify_header(&vector.header.bytes, &attestation.signature, &authorization),
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
