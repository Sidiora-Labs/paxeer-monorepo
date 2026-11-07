//! Repair 22.33.1557. Finding/test mappings and out-of-scope wiring requests
//! are recorded in src/repair_22_33_1557.kvx. Qualification evidence is supplied
//! by the runner's gate ledger, including the exact candidate revision.

// Preserve the existing real inclusion producer/consumer assertions for all
// three retained constructors, including state evidence, in this focused gate.
#[path = "repair_22_31_226.rs"]
mod retained_certificate_retirement;
#[path = "checkpoint.rs"]
mod retained_checkpoint_consumers;
#[path = "export_complete.rs"]
mod retained_complete_export;
#[path = "inclusion.rs"]
mod retained_inclusion;
#[path = "offline_export.rs"]
mod retained_offline_export;

#[path = "support/historical_checkpoint.rs"]
mod historical;

use layerx_proof::evidence::Evidence;
use layerx_proof::export::{
    verify, verify_complete, CompleteExportError, CompleteVerificationReport,
    ExportVerificationError, IndependentOfflineTrust, OfflineExport, ReceiptFact,
    VerificationReport,
};
use layerx_proof::export_codec::{
    ArtifactDecodeError, CompleteOfflineArtifact, ExportCodecError, FactRefError, FactSelector,
    HeaderRecord, InclusionRecord, InclusionRecordKind, ProofRecord, ReceiptRecord,
};
use layerx_proof::inclusion::{verify_activity, verify_receipt, SequencerAuthorization};
use layerx_proof::level::{achieved, achieved_rung};
use layerx_proof::merkle::{build_proof, Proof};
use layerx_proof::receipt::{verify_outcome, AuthorizedBatch, ReceiptCheck};
use layerx_proof::signed_authority::SignedAuthorityHistory;
use layerx_types::json::parse;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use layerx_wire::handover::decode_genesis_trust;
use layerx_wire::hash::{batch_header_digest, receipt_digest};
use layerx_wire::receipt::{decode, decode_batch_header, decode_merkle_proof, encode_unsigned};

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
const GENESIS: &[u8] = include_bytes!("fixtures/signed-authority/genesis.bin");
const HEADERS: &[u8] = include_bytes!("fixtures/signed-authority/headers.bin");

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("repair 22.33.1557: {error:?}"))
}

fn receipt_record() -> ReceiptRecord {
    let receipt = must(decode(RECEIPT));
    let protocol = receipt.protocol().expect("recorded native withdrawal");
    ReceiptRecord {
        reference: FactSelector::Receipt {
            activity_id: protocol.activity_id(),
        },
        canonical_receipt: RECEIPT.to_vec(),
        authorised_batch: AuthorizedBatch::new(
            protocol.batch_id(),
            protocol.asset(),
            protocol.previous_state_root(),
            protocol.resulting_state_root(),
            KEY,
        ),
        expected_receipt_digest: must(receipt_digest(&must(encode_unsigned(&receipt)))),
    }
}

fn artifact() -> CompleteOfflineArtifact {
    let record = receipt_record();
    let header = must(decode_batch_header(HEADER));
    let captured = must(decode_merkle_proof(include_bytes!(
        "../../../../../tests/fixtures/asset/bound-native-withdrawal/receipt.proof"
    )));
    CompleteOfflineArtifact {
        facts: vec![record.reference],
        receipts: vec![record.clone()],
        proofs: vec![ProofRecord::Inclusion(InclusionRecord {
            kind: InclusionRecordKind::Receipt,
            reference: record.reference,
            canonical_leaf: RECEIPT.to_vec(),
            proof: must(Proof::new(
                captured.leaf_index(),
                captured.leaf_count(),
                captured.siblings().to_vec(),
            )),
            header_digest: must(batch_header_digest(HEADER)),
        })],
        headers: vec![HeaderRecord {
            canonical_header: HEADER.to_vec(),
            signature: SIGNATURE,
            sequencer_id: header.sequencer_id(),
            public_key: KEY,
            first_batch: header.batch_number(),
            last_batch: header.batch_number(),
        }],
    }
}

fn transport(artifact: &CompleteOfflineArtifact) -> CompleteOfflineArtifact {
    let bytes = must(artifact.encode());
    assert!(bytes.certificates.is_empty());
    let decoded = must(CompleteOfflineArtifact::decode(
        &bytes.facts,
        &bytes.receipts,
        &bytes.proofs,
        &bytes.certificates,
        &bytes.headers,
    ));
    assert_eq!(must(decoded.encode()), bytes);
    decoded
}

fn assert_rung(evidence: &Evidence, expected: Rung, level: VerificationLevel) {
    assert_eq!(evidence.rung(), expected);
    assert_eq!(achieved_rung(evidence), expected);
    assert_eq!(evidence.level(), level);
    assert_eq!(achieved(evidence), level);
    assert!(evidence.rung() <= Rung::Receipted);
    assert_ne!(evidence.rung(), Rung::Committed);
}

#[test]
fn checkpoint_evidence_constructor_retains_native_receipt_and_inclusion() {
    let artifact = transport(&artifact());
    let record = &artifact.receipts[0];
    let verified = must(verify_outcome(
        &record.canonical_receipt,
        &record.authorised_batch,
    ));
    assert_eq!(verified.canonical_bytes(), RECEIPT);
    assert_eq!(
        verified.evidence().receipt_digest(),
        Some(record.expected_receipt_digest)
    );
    assert_rung(
        verified.evidence(),
        Rung::Submitted,
        VerificationLevel::SEQUENCER_SIGNED,
    );
    let header = &artifact.headers[0];
    let authorization = SequencerAuthorization::new(
        header.sequencer_id,
        header.public_key,
        header.first_batch,
        header.last_batch,
    );
    let ProofRecord::Inclusion(inclusion) = &artifact.proofs[0] else {
        panic!("native receipt inclusion");
    };
    let included = must(verify_receipt(
        &record.canonical_receipt,
        &inclusion.proof,
        &header.canonical_header,
        &header.signature,
        &authorization,
    ));
    assert_rung(
        included.evidence(),
        Rung::Receipted,
        VerificationLevel::BATCH_INCLUDED,
    );
    assert_eq!(
        included.evidence().header_digest(),
        Some(inclusion.header_digest)
    );
    assert_eq!(
        included.evidence().proof_root(),
        Some(included.header().header().receipt_merkle_root())
    );
    let (proof, root) = must(build_proof(&[ACTIVITY], 0));
    let activity = must(verify_activity(
        ACTIVITY,
        &proof,
        &header.canonical_header,
        &header.signature,
        &authorization,
    ));
    assert_eq!(activity.evidence().proof_root(), Some(root));
    assert_rung(
        activity.evidence(),
        Rung::Receipted,
        VerificationLevel::BATCH_INCLUDED,
    );
}

#[test]
fn legacy_level_mints_commitment_pinned_receipts_stop_at_receipted() {
    let document = must(parse(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )));
    let case = &must(document.array_at("cases"))[0];
    assert_eq!(must(case.str_at("id")), "V1-pax-transfer");
    assert_eq!(must(case.str_at("expected.verifier")), "accepts");
    let bytes = must(case.hex_at("signedReceipt"));
    let receipt = must(decode(&bytes));
    let protocol = receipt.protocol().expect("recorded sequencer receipt");
    let pin = SequencerAuthorization::new(
        must(case.hex_array_at("pinnedKey.keyId")),
        must(case.hex_array_at("pinnedKey.publicKey")),
        must(case.u64_at("pinnedKey.firstHeight")),
        must(case.u64_at("pinnedKey.lastHeight")),
    );
    let pins = [pin];
    let batch = must(AuthorizedBatch::from_receipt_keys(
        &bytes,
        &pins,
        protocol.asset(),
        protocol.previous_state_root(),
        protocol.resulting_state_root(),
    ));
    let verified = must(verify_outcome(&bytes, &batch));
    assert_eq!(verified.canonical_bytes(), bytes);
    assert_rung(
        verified.evidence(),
        Rung::Receipted,
        VerificationLevel::SEQUENCER_SIGNED,
    );
    let empty: [SequencerAuthorization; 0] = [];
    assert_eq!(
        AuthorizedBatch::from_receipt_keys(
            &bytes,
            &empty,
            protocol.asset(),
            protocol.previous_state_root(),
            protocol.resulting_state_root(),
        )
        .expect_err("absent receipt-key pins")
        .check,
        ReceiptCheck::SequencerSignature,
    );
}

#[test]
fn registry_export_signatures_stale_current_interfaces_verify_real_records() {
    let legacy: fn(&OfflineExport) -> Result<VerificationReport, ExportVerificationError> = verify;
    let complete: fn(
        &CompleteOfflineArtifact,
        &IndependentOfflineTrust,
        Rung,
    ) -> Result<CompleteVerificationReport, CompleteExportError> = verify_complete;
    let decoded = transport(&artifact());
    let receipt = &decoded.receipts[0];
    let export = OfflineExport {
        receipts: vec![ReceiptFact {
            statement: receipt.reference.canonical_text(),
            canonical_receipt_bytes: receipt.canonical_receipt.clone(),
            authorised_batch: receipt.authorised_batch.clone(),
            expected_receipt_digest: receipt.expected_receipt_digest,
        }],
        inclusions: Vec::new(),
        derived_aggregates: Vec::new(),
    };
    let report = must(legacy(&export));
    assert_eq!(report.verified_receipts, 1);
    assert_eq!(
        report.receipt_digests,
        vec![receipt.expected_receipt_digest]
    );

    // This independent native genesis/history is deliberately a different
    // producer from the withdrawal artifact; export-supplied keys cannot replace it.
    let genesis = must(decode_genesis_trust(GENESIS));
    let mut history = must(SignedAuthorityHistory::from_genesis(
        genesis.network_id,
        genesis.canonical_state_root,
        genesis.initial_sequencer_key,
        genesis.governance_witness,
    ));
    must(history.advance(&HEADERS[..354], &must(HEADERS[354..418].try_into()), None));
    let trust = must(IndependentOfflineTrust::new(genesis.registry, history));
    for requested in [Rung::Submitted, Rung::Receipted, Rung::Committed] {
        assert_eq!(
            complete(&decoded, &trust, requested),
            Err(CompleteExportError::Header { fact: 0 }),
        );
    }
}

#[test]
fn registry_removed_codec_symbols_certificate_bucket_is_always_refused() {
    let artifact = transport(&artifact());
    let encoded = must(artifact.encode());
    let published = include_bytes!("../../../../../tests/vectors/checkpoint/fresh.json");
    for certificate in [published.as_slice(), RECEIPT, &[]] {
        assert_eq!(
            CompleteOfflineArtifact::decode(
                &encoded.facts,
                &encoded.receipts,
                &encoded.proofs,
                &[certificate.to_vec()],
                &encoded.headers,
            ),
            Err(ArtifactDecodeError::Record {
                bucket: "certificates",
                index: 0,
                error: ExportCodecError::Tag,
            }),
        );
    }
    let id = artifact.facts[0].activity_id();
    let hex: String = id.iter().map(|byte| format!("{byte:02x}")).collect();
    assert_eq!(
        FactSelector::parse(&format!("checkpoint:{hex}:1")),
        Err(FactRefError::UnknownKind),
    );
}

#[test]
fn retired_settlement_items_are_deprecated_and_checkpoint_api_is_retained() {
    let lib = include_str!("../src/lib.rs");
    assert!(lib.contains("pub mod checkpoint;"));
    historical::assert_deprecated(lib, "pub mod settlement;");
    let settlement = include_str!("../src/settlement.rs");
    for declaration in [
        "pub const DECLARED_CHECKPOINT_SETTLEMENT:",
        "pub struct DeclaredDomain {",
        "pub fn declared()",
        "pub fn maximum_attestation_delay_ms()",
        "pub fn declared_domain(",
    ] {
        historical::assert_deprecated(settlement, declaration);
    }
    let evidence = include_str!("../src/evidence.rs");
    for retired in ["checkpoint_id", "settlement_reference", "fn checkpoint("] {
        assert!(!evidence.contains(retired));
    }
    assert!(!include_str!("../src/level.rs").contains("checkpoint_finalised"));
    assert_eq!(VerificationLevel::COMMITTED.rung(), Some(Rung::Committed));
    // Historical configuration cannot raise the retained receipt verifier's rung.
    let decoded = transport(&artifact());
    let record = &decoded.receipts[0];
    let verified = must(verify_outcome(
        &record.canonical_receipt,
        &record.authorised_batch,
    ));
    assert_rung(
        verified.evidence(),
        Rung::Submitted,
        VerificationLevel::SEQUENCER_SIGNED,
    );
}

#[test]
fn retained_checkpoint_api_refuses_historical_certificates() {
    use layerx_proof::checkpoint::{
        checkpoint_id, verify_certificate, verify_declared_certificate, Attestation, Certificate,
        Checkpoint, CheckpointError, GuarantorKey, SettlementDomain, ThresholdReport,
    };
    use layerx_types::vectors::checkpoint::load_checkpoint_vectors;

    // Keep the exact result type consumed by layerx-agentd::cache available.
    let verify: fn(
        &Certificate,
        &str,
        &[u8; 32],
        Option<&[u8]>,
    ) -> Result<ThresholdReport, CheckpointError> = verify_declared_certificate;
    let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
    let configuration = must(
        layerx_types::settlement::DeclaredCheckpointSettlement::parse(include_str!(
            "../../../../../contracts/config/checkpoint-settlement.json"
        )),
    );
    for vector in must(load_checkpoint_vectors(&root)) {
        let domain = must(configuration.domain(&vector.settlement_domain));
        let checkpoint =
            Checkpoint::new(vector.header.bytes.clone(), vector.validity_proof.clone());
        assert_eq!(must(checkpoint_id(&checkpoint)), vector.expected_digest);
        let keys: Vec<_> = domain
            .guarantor_set()
            .iter()
            .map(|key| GuarantorKey::new(key.guarantor_id(), key.public_key(), true))
            .collect();
        let attestations: Vec<_> = vector
            .attestations
            .iter()
            .map(|signed| {
                let attestation = Attestation::new(
                    vector.header.protocol_version,
                    vector.header.network_id,
                    domain.paxeer_chain_id(),
                    domain.settlement_contract(),
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
                attestation
            })
            .collect();
        for reference in [None, Some(b"registered-checkpoint".to_vec())] {
            let certificate = Certificate::new(
                checkpoint.clone(),
                attestations.clone(),
                vector.threshold,
                reference,
            );
            assert_eq!(
                verify_certificate(
                    &certificate,
                    &keys,
                    &vector.expected_digest,
                    SettlementDomain::new(domain.paxeer_chain_id(), domain.settlement_contract()),
                    certificate.settlement_reference(),
                ),
                Err(CheckpointError::RetiredCertificateVerifier)
            );
            for name in [vector.settlement_domain.as_str(), "undeclared"] {
                assert_eq!(
                    verify(
                        &certificate,
                        name,
                        &vector.expected_digest,
                        certificate.settlement_reference(),
                    ),
                    Err(CheckpointError::RetiredCertificateVerifier)
                );
            }
        }
    }
}
