//! Archived fixture data for testing the retained decoder's certificate refusal.

use std::path::PathBuf;

use layerx_proof::export_codec::{
    ArtifactDecodeError, CompleteOfflineArtifact, ExportCodecError, FactSelector,
};
use layerx_types::settlement::{DeclaredCheckpointSettlement, CHECKPOINT_SETTLEMENT_PATH};
use layerx_types::vectors::checkpoint::{
    checkpoint_vector_path, load_checkpoint_vector, CheckpointVector,
};

pub fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("historical checkpoint fixture: {error:?}"))
}

pub fn repository_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../..")
}

// Select archival configuration explicitly; it is not BFT proof trust.
pub fn configuration() -> DeclaredCheckpointSettlement {
    must(DeclaredCheckpointSettlement::parse(&must(
        std::fs::read_to_string(repository_root().join(CHECKPOINT_SETTLEMENT_PATH)),
    )))
}

pub fn fresh() -> CheckpointVector {
    must(load_checkpoint_vector(
        &checkpoint_vector_path(&repository_root(), "fresh"),
        "fresh",
    ))
}

fn lp(out: &mut Vec<u8>, bytes: &[u8]) {
    out.extend_from_slice(&must(u32::try_from(bytes.len())).to_be_bytes());
    out.extend_from_slice(bytes);
}

// Transport the recorded header, proof and signed statements in the frozen LXCP layout.
pub fn certificate_record(vector: &CheckpointVector, reference: Option<&[u8]>) -> Vec<u8> {
    let mut certificate = Vec::new();
    lp(&mut certificate, &vector.header.bytes);
    lp(&mut certificate, &vector.validity_proof);
    certificate.push(must(u8::try_from(vector.threshold)));
    certificate.push(must(u8::try_from(vector.attestations.len())));
    for item in &vector.attestations {
        certificate.extend_from_slice(&item.message);
        certificate.extend_from_slice(&item.signer);
        certificate.extend_from_slice(&item.signature);
        certificate.push(item.signature_v);
    }
    certificate.push(u8::from(reference.is_some()));
    if let Some(reference) = reference {
        lp(&mut certificate, reference);
    }
    let hex: String = vector
        .expected_digest
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect();
    let mut record = b"LXCP".to_vec();
    record.push(1);
    lp(
        &mut record,
        format!("checkpoint:{hex}:{}", vector.header.batch_number).as_bytes(),
    );
    lp(&mut record, &certificate);
    record.extend_from_slice(&vector.header.batch_number.to_be_bytes());
    let configuration = configuration();
    let domain = must(configuration.domain(&vector.settlement_domain));
    record.extend_from_slice(&must(u16::try_from(domain.guarantor_set().len())).to_be_bytes());
    for key in domain.guarantor_set() {
        record.extend_from_slice(&key.guarantor_id());
        record.extend_from_slice(&key.public_key());
        record.push(1);
    }
    record.extend_from_slice(&vector.expected_digest);
    record.push(u8::from(reference.is_some()));
    if let Some(reference) = reference {
        lp(&mut record, reference);
    }
    record.extend_from_slice(&0_u16.to_be_bytes());
    record
}

pub fn assert_refused(record: Vec<u8>, activity_id: [u8; 32]) {
    assert_eq!(
        CompleteOfflineArtifact::decode(
            &[FactSelector::Receipt { activity_id }.canonical_text()],
            &[],
            &[],
            &[record],
            &[],
        ),
        Err(ArtifactDecodeError::Record {
            bucket: "certificates",
            index: 0,
            error: ExportCodecError::Tag,
        }),
    );
}

pub fn assert_deprecated(source: &str, declaration: &str) {
    let (prefix, _) = source
        .split_once(declaration)
        .expect("retained public declaration");
    let (_, attribute) = prefix
        .rsplit_once("#[deprecated(")
        .expect("deprecation attribute");
    assert!(attribute.contains("historical settlement configuration cannot establish BFT commitment; checkpoint::verify_declared_certificate returns CheckpointError::RetiredCertificateVerifier; use receipt or inclusion verification"));
    assert!(attribute.trim_end().ends_with(")]"));
    assert!(
        !attribute.contains("pub "),
        "deprecation must apply to {declaration}"
    );
}
