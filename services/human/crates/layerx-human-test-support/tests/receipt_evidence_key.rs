use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::protocol_evidence::{ReceiptEvidenceError, VerifierPolicyError};
use layerx_human_test_support::{
    committed_execution_batch_id, evidence_verifier, raw_receipt_evidence, RawReceiptEvidence,
};
use layerx_intents::canonical::{decode_batch_header, decode_receipt};
use layerx_intents::vectors::{receipt, receipt_signing_digest};
use layerx_proof::inclusion::InclusionError;
use layerx_types::verify::VerificationLevel;

const SEQUENCE: u64 = 7;
const PREVIOUS_STATE_ROOT: [u8; 32] = [2; 32];
const RESULTING_STATE_ROOT: [u8; 32] = [3; 32];

fn signed_receipt(signer: &SigningKey) -> Vec<u8> {
    let encode = |signature| {
        let mut canonical = receipt(signature).expect("canonical receipt vector");
        let sequence_at = 2 + 2 + 2 + 4 + 32;
        assert_eq!(
            &canonical[sequence_at..sequence_at + 8],
            &9_u64.to_be_bytes()
        );
        canonical[sequence_at..sequence_at + 8].copy_from_slice(&SEQUENCE.to_be_bytes());
        let batch_positions: Vec<_> = canonical
            .windows(32)
            .enumerate()
            .filter_map(|(position, bytes)| (bytes == [4; 32]).then_some(position))
            .collect();
        assert_eq!(batch_positions.len(), 1);
        let batch_at = batch_positions[0];
        canonical[batch_at..batch_at + 32].copy_from_slice(&committed_execution_batch_id(
            PREVIOUS_STATE_ROOT,
            [8; 32],
            SEQUENCE,
        ));
        canonical
    };
    let unsigned = encode(None);
    let digest = receipt_signing_digest(&unsigned).expect("canonical receipt signing digest");
    encode(Some(signer.sign(&digest).to_bytes()))
}

fn evidence(signer: &SigningKey) -> RawReceiptEvidence {
    raw_receipt_evidence(
        signed_receipt(signer),
        signer.verifying_key().to_bytes(),
        PREVIOUS_STATE_ROOT,
        RESULTING_STATE_ROOT,
        SEQUENCE,
        signer,
    )
}

#[test]
fn raw_receipt_evidence_verifies_under_pinned_key() {
    let signer = SigningKey::from_bytes(&[0x84; 32]);
    let raw = evidence(&signer);
    let verifier = evidence_verifier(&signer);
    let verified = verifier
        .verify_receipt(&raw)
        .expect("receipt evidence signed by the pinned key");

    assert_eq!(verified.canonical_receipt(), raw.canonical_receipt());
    assert_eq!(verified.global_sequence(), SEQUENCE);
    assert_eq!(verified.activity_id(), [1; 32]);
    assert_eq!(verified.result_code(), 0);
    assert_eq!(verified.amount(), 25);
    assert_eq!(verified.level(), VerificationLevel::BATCH_INCLUDED);

    let header = decode_batch_header(raw.canonical_header()).expect("canonical batch header");
    assert_eq!(header.sequencer_id(), signer.verifying_key().to_bytes());
    assert_eq!(header.previous_state_root(), PREVIOUS_STATE_ROOT);
    assert_eq!(header.resulting_state_root(), RESULTING_STATE_ROOT);
    let decoded = decode_receipt(raw.canonical_receipt()).expect("canonical receipt");
    let protocol = decoded.protocol().expect("protocol receipt");
    assert_eq!(protocol.previous_state_root(), header.previous_state_root());
    assert_eq!(
        protocol.resulting_state_root(),
        header.resulting_state_root()
    );
}

#[test]
fn raw_receipt_evidence_rejects_foreign_key() {
    let signer = SigningKey::from_bytes(&[0x84; 32]);
    let foreign = SigningKey::from_bytes(&[0x85; 32]);
    assert_ne!(signer.verifying_key(), foreign.verifying_key());
    let raw = evidence(&foreign);
    let verifier = evidence_verifier(&signer);
    assert_eq!(
        verifier.verify_receipt(&raw),
        Err(ReceiptEvidenceError::Policy(
            VerifierPolicyError::UnknownSequencer
        ))
    );

    let pinned_raw = evidence(&signer);
    let digest =
        layerx_intents::vectors::batch_header_signing_digest(pinned_raw.canonical_header())
            .expect("canonical header digest");
    let foreign_signature = RawReceiptEvidence::new(
        pinned_raw.canonical_receipt().to_vec(),
        pinned_raw.proof().clone(),
        pinned_raw.canonical_header().to_vec(),
        foreign.sign(&digest).to_bytes(),
    );
    assert_eq!(
        verifier.verify_receipt(&foreign_signature),
        Err(ReceiptEvidenceError::Inclusion(
            InclusionError::HeaderSignature
        ))
    );

    assert!(std::panic::catch_unwind(|| raw_receipt_evidence(
        signed_receipt(&foreign),
        signer.verifying_key().to_bytes(),
        PREVIOUS_STATE_ROOT,
        RESULTING_STATE_ROOT,
        SEQUENCE,
        &foreign,
    ))
    .is_err());
}
