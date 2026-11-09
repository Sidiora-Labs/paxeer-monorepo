//! Contract consumers for audit record 22.30.121. The function-pointer types
//! pin the documented signatures; the shared producer executes real programs,
//! transports encoded proofs, reconstructs admission and consumes both APIs.

#[path = "proof_arbiter_crate.rs"]
mod producer;

use std::collections::BTreeMap;

use layerx_programs_arbiter::{
    ics23::{decode_existence, ExistenceProof},
    AdmissionEvidence, AuthenticatedCatalogue, ChainCommitProof, ReplayError, VerifiedReplay,
    VerifiedReplayAuthority,
};
use layerx_proof::{inclusion::VerifiedBatchHeader, receipt::VerifiedReceipt};
use serde_json::Value;

type AuthorityConstructor = fn(
    &VerifiedReceipt,
    &AdmissionEvidence,
    &VerifiedBatchHeader,
    &ChainCommitProof,
) -> Result<VerifiedReplayAuthority, ReplayError>;
type CodeBlobs = BTreeMap<[u8; 32], Vec<u8>>;
type CodeProofs = BTreeMap<[u8; 32], ExistenceProof>;
type CatalogueConstructor =
    fn(&AdmissionEvidence, &CodeBlobs) -> Result<AuthenticatedCatalogue, ReplayError>;
type ChainCatalogueConstructor = fn(
    &AdmissionEvidence,
    &ChainCommitProof,
    [u8; 32],
    &[u8],
    &CodeProofs,
    &CodeBlobs,
) -> Result<AuthenticatedCatalogue, ReplayError>;
type ReplayConstructor = fn(
    &VerifiedReceipt,
    &AdmissionEvidence,
    &ChainCommitProof,
    [u8; 32],
    &[u8],
    &ExistenceProof,
    &[u8],
    &[u8],
    AuthenticatedCatalogue,
) -> Result<VerifiedReplay, ReplayError>;
type BatchReplayConstructor = fn(
    &VerifiedReceipt,
    &AdmissionEvidence,
    &VerifiedBatchHeader,
    &ChainCommitProof,
    [u8; 32],
    &[u8],
    &ExistenceProof,
    &[u8],
    &[u8],
    AuthenticatedCatalogue,
) -> Result<VerifiedReplay, ReplayError>;

#[test]
fn registry_authority_signature_drives_both_call_envelopes() {
    let _: [AuthorityConstructor; 2] = [
        VerifiedReplayAuthority::derive,
        VerifiedReplayAuthority::derive_program_call,
    ];
    // This producer signs ordinary and replay envelopes, authenticates admission
    // and sequencer proofs, and asserts the derived principal, payer, binding,
    // capabilities and native encoding for both public constructors.
    producer::repair_v5_authority();
}

#[test]
fn registry_replay_signatures_reconstruct_and_verify_adjacent_steps() {
    let _: CatalogueConstructor = AuthenticatedCatalogue::verify;
    let _: ChainCatalogueConstructor = AuthenticatedCatalogue::verify_with_module;
    let _: ReplayConstructor = VerifiedReplay::verify;
    let _: BatchReplayConstructor = VerifiedReplay::verify_with_batch_context;
    // Fresh admission, catalogue, code and metadata proofs cross both replay
    // constructors twice for ABI 2 and 5. Every genuine adjacent executor pair
    // must replay successfully; receipt, payer and boundary tampering must fail.
    producer::repair_executor_payer_replay();
}

fn keeper_vector() -> Value {
    serde_json::from_str(include_str!(
        "../../../tests/vectors/ics23-admission-evidence-app-hash.json"
    ))
    .expect("committed sequencer keeper proof transcript")
}

fn bytes(record: &Value, name: &str) -> Vec<u8> {
    let encoded = record[name].as_str().expect("keeper hex field");
    assert!(encoded.is_ascii());
    assert_eq!(encoded.len() % 2, 0);
    (0..encoded.len())
        .step_by(2)
        .map(|offset| u8::from_str_radix(&encoded[offset..offset + 2], 16).unwrap())
        .collect()
}

fn hash(record: &Value, name: &str) -> [u8; 32] {
    bytes(record, name).try_into().expect("32-byte keeper hash")
}

fn chain_commit(record: &Value) -> ChainCommitProof {
    let proof = decode_existence(&bytes(record, "proof")).expect("transported ICS23 proof");
    assert_eq!(proof.key, bytes(record, "key"));
    assert_eq!(proof.value, bytes(record, "value"));
    ChainCommitProof {
        height: record["height"].as_u64().expect("committed height"),
        app_hash: hash(record, "app_hash"),
        store_root: hash(record, "store_root"),
        multistore_proof: bytes(record, "multistore_proof"),
        receipt_key: bytes(record, "key"),
        receipt_proof: proof,
    }
}

#[test]
fn registry_authority_signature_binds_trusted_app_hash_and_receipt_height() {
    let vector = keeper_vector();
    let entries = vector["receipt_entries"].as_array().unwrap();
    assert!(!entries.is_empty());
    for entry in entries {
        // Reopen the consumer from the committed transport on every pass.
        for _ in 0..2 {
            let commit = chain_commit(entry);
            let receipt_bytes = bytes(entry, "receipt");
            assert_eq!(commit.verify_receipt_bytes(&receipt_bytes), Ok(()));
            let keys = commit.receipt_keys().unwrap();
            assert_eq!(keys.len(), 2);
            let receipt =
                layerx_proof::receipt::verify_sequencer_signature(&receipt_bytes, keys.as_slice())
                    .unwrap();
            let protocol = receipt.protocol().unwrap();
            assert_eq!(&protocol.batch_id()[..24], &[0; 24]);
            let receipt_height = u64::from_be_bytes(protocol.batch_id()[24..].try_into().unwrap());
            assert_eq!(receipt_height, commit.height);
            let matching: Vec<_> = keys
                .iter()
                .filter(|key| key.sequencer_id() == protocol.context_hash())
                .collect();
            assert_eq!(matching.len(), 1);
            assert!(matching[0].first_batch_number() <= receipt_height);
            assert!(receipt_height <= matching[0].last_batch_number());

            let mut earlier = commit.clone();
            earlier.height = receipt_height - 1;
            assert_eq!(
                earlier.verify_receipt_bytes(&receipt_bytes),
                Err(ReplayError::Receipt)
            );
            let mut later = commit.clone();
            later.height += 1;
            assert_eq!(later.verify_receipt_bytes(&receipt_bytes), Ok(()));

            let mut wrong_app = commit.clone();
            wrong_app.app_hash[0] ^= 1;
            assert!(matches!(
                wrong_app.receipt_keys(),
                Err(ReplayError::Receipt)
            ));
            let mut wrong_table = commit.clone();
            wrong_table.receipt_proof.value[0] ^= 1;
            assert_eq!(
                wrong_table.verify_receipt_bytes(&receipt_bytes),
                Err(ReplayError::Receipt)
            );
        }
    }
}

#[test]
fn registry_replay_signatures_verify_admission_from_transported_chain_proofs() {
    let vector = keeper_vector();
    let entries = vector["entries"].as_array().unwrap();
    assert_eq!(entries.len(), 2);
    for entry in entries {
        for _ in 0..2 {
            let commit = chain_commit(&entry["chain_commit"]);
            let records = entry["records"].as_array().unwrap();
            let proofs: BTreeMap<_, _> = records
                .iter()
                .map(|record| {
                    assert_eq!(hash(record, "app_hash"), commit.app_hash);
                    let proof = decode_existence(&bytes(record, "proof")).unwrap();
                    assert_eq!(proof.key, bytes(record, "key"));
                    assert_eq!(proof.value, bytes(record, "value"));
                    (proof.key.clone(), proof)
                })
                .collect();
            assert_eq!(proofs.len(), records.len());
            let host_root = hash(&records[0], "store_root");
            let host_multistore = bytes(&records[0], "multistore_proof");
            let activity_id = hash(entry, "activity_id");
            let verify = |app_hash, store_root, proofs: &BTreeMap<_, _>| {
                AdmissionEvidence::verify_under_app_hash(
                    app_hash,
                    store_root,
                    &host_multistore,
                    &activity_id,
                    proofs,
                )
            };
            let admission = verify(commit.app_hash, host_root, &proofs).unwrap();
            assert_eq!(
                admission.network_id() as u64,
                entry["network_id"].as_u64().unwrap()
            );
            assert_eq!(admission.activity_id(), activity_id);
            assert_eq!(
                admission.execution_sequence(),
                entry["sequence"].as_u64().unwrap()
            );
            assert_eq!(admission.state_root(), hash(entry, "state_root"));
            assert_eq!(admission.receipt_digest(), hash(entry, "receipt_digest"));
            assert_eq!(
                layerx_wire::historical::activity::encode_signed(admission.activity()).unwrap(),
                bytes(entry, "signed_activity")
            );
            assert_eq!(
                admission.legacy().legacy().canonical_bytes(),
                bytes(entry, "native")
            );

            let mut wrong_app = commit.app_hash;
            wrong_app[0] ^= 1;
            assert!(matches!(
                verify(wrong_app, host_root, &proofs),
                Err(ReplayError::Admission)
            ));
            let mut wrong_host = host_root;
            wrong_host[0] ^= 1;
            assert!(matches!(
                verify(commit.app_hash, wrong_host, &proofs),
                Err(ReplayError::Admission)
            ));
            let mut changed = proofs.clone();
            // Mutate an authenticated governance record, leaving the real
            // producer's proof/root intact. An unverified admission is refused.
            let key = changed
                .keys()
                .find(|key| key.starts_with(&[1, 0, 7]))
                .unwrap()
                .clone();
            changed.get_mut(&key).unwrap().value[0] ^= 1;
            assert!(matches!(
                verify(commit.app_hash, host_root, &changed),
                Err(ReplayError::Admission)
            ));
        }
    }
}
