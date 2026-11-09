//! Archived fixture data for testing the retained decoder's certificate refusal.

use std::path::PathBuf;

use layerx_proof::export_codec::{
    ArtifactDecodeError, CompleteOfflineArtifact, ExportCodecError, FactSelector,
};
use layerx_types::vectors::checkpoint::{
    checkpoint_vector_path, load_checkpoint_vector, CheckpointVector,
};
use serde_json::Value;

pub fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("historical checkpoint fixture: {error:?}"))
}

pub fn repository_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../..")
}

pub struct Configuration {
    document: Value,
    pub certificate_threshold: usize,
    pub maximum_attestation_delay_ms: u64,
}

pub struct Domain {
    pub paxeer_chain_id: u64,
    pub network_id: u32,
    pub settlement_contract: [u8; 20],
    pub guarantor_set: Vec<Guarantor>,
}

pub struct Guarantor {
    pub guarantor_id: [u8; 32],
    pub signer: [u8; 20],
    pub public_key: [u8; 33],
}

fn number(value: &Value) -> u64 {
    value
        .as_u64()
        .expect("historical configuration unsigned integer")
}

fn hex<const N: usize>(value: &Value) -> [u8; N] {
    let text = value
        .as_str()
        .expect("historical configuration hex string")
        .strip_prefix("0x")
        .expect("historical configuration hex prefix");
    assert_eq!(text.len(), N * 2);
    let mut bytes = [0; N];
    for (byte, pair) in bytes.iter_mut().zip(text.as_bytes().chunks_exact(2)) {
        *byte = must(u8::from_str_radix(must(std::str::from_utf8(pair)), 16));
    }
    bytes
}

impl Configuration {
    pub fn domain(&self, name: &str) -> Domain {
        let domain = self.document["settlement_domains"]
            .get(name)
            .expect("historical configuration settlement domain");
        Domain {
            paxeer_chain_id: number(&domain["paxeer_chain_id"]),
            network_id: must(u32::try_from(number(&domain["network_id"]))),
            settlement_contract: hex(&domain["settlement_contract"]),
            guarantor_set: domain["guarantor_set"]
                .as_array()
                .expect("historical configuration guarantor set")
                .iter()
                .map(|key| Guarantor {
                    guarantor_id: hex(&key["guarantor_id"]),
                    signer: hex(&key["signer"]),
                    public_key: hex(&key["public_key"]),
                })
                .collect(),
        }
    }
}

// Read the archival producer contracts/config/checkpoint-settlement.json, not BFT proof trust.
pub fn configuration() -> Configuration {
    let document: Value = must(serde_json::from_str(&must(std::fs::read_to_string(
        repository_root().join("contracts/config/checkpoint-settlement.json"),
    ))));
    let policy = &document["finality_policy"];
    let certificate_threshold = must(usize::try_from(number(&policy["certificate_threshold"])));
    assert!(certificate_threshold > 0);
    let maximum_attestation_delay_ms = number(&policy["maximum_attestation_delay_seconds"])
        .checked_mul(1_000)
        .expect("historical configuration attestation delay fits milliseconds");
    assert!(maximum_attestation_delay_ms > 0);
    Configuration {
        document,
        certificate_threshold,
        maximum_attestation_delay_ms,
    }
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
    let domain = configuration.domain(&vector.settlement_domain);
    record.extend_from_slice(&must(u16::try_from(domain.guarantor_set.len())).to_be_bytes());
    for key in &domain.guarantor_set {
        record.extend_from_slice(&key.guarantor_id);
        record.extend_from_slice(&key.public_key);
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
