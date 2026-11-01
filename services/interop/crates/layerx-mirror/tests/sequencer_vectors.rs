use std::collections::BTreeSet;
use std::fs;
use std::path::Path;

use layerx_mirror::committed::{CommittedBlock, CommittedHeader};
use layerx_mirror::source::{
    MirrorCanonicalPosition, MirrorObservation, MirrorSourceId, MirrorTargetIdentity,
};
use layerx_mirror::{
    archive_commitment, Archive, MirrorEvidenceLevel, MirrorVerifier, MirrorVerifyError, NodeHead,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptCheck, ReceiptKeys};
use layerx_wire::hash::receipt_digest;
use layerx_wire::receipt::encode_unsigned;
use serde::Deserialize;
use serde_json::Value;
use sha2::{Digest, Sha256};
use sha3::Keccak256;

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct Vectors {
    name: String,
    chain_id: u32,
    cases: Vec<Case>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct Case {
    id: String,
    vector: String,
    receipt_json: Value,
    raw_transaction: String,
    prev_seq_hash: String,
    signed_receipt: String,
    receipt_digest: String,
    pinned_key: PinnedKey,
    expected: Value,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct PinnedKey {
    key_id: String,
    public_key: String,
    first_height: u64,
    last_height: u64,
}

enum Classification {
    Answer,
    Refusal(Option<ReceiptCheck>),
}

fn classify(case: &Case) -> Classification {
    match case.id.as_str() {
        "V1-pax-transfer"
        | "V1-erc20-approve"
        | "V1-layerxledger-pay-exact"
        | "V1-layerxbridge-bridge-out" => {
            assert_eq!(case.vector, "V1", "{}", case.id);
            Classification::Answer
        }
        "V2-layerxledger-revert" => {
            assert_eq!(case.vector, "V2", "{}", case.id);
            Classification::Answer
        }
        "V3-key-not-pinned" | "V3-height-outside-key-range" => {
            assert_eq!(case.vector, "V3", "{}", case.id);
            Classification::Refusal(Some(ReceiptCheck::SequencerSignature))
        }
        "V3-non-canonical-bytes" => {
            assert_eq!(case.vector, "V3", "{}", case.id);
            Classification::Refusal(Some(ReceiptCheck::Decode))
        }
        "V3-activity-id-not-raw-hash" | "V3-seq-hash-mismatch" | "V3-tampered-log" => {
            assert_eq!(case.vector, "V3", "{}", case.id);
            Classification::Refusal(None)
        }
        id => panic!("unclassified shared sequencer-receipt case: {id}"),
    }
}

fn shared_vectors() -> Vectors {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../..")
        .join("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");
    let bytes = fs::read(&path).unwrap_or_else(|error| panic!("{}: {error}", path.display()));
    let vectors: Vectors = serde_json::from_slice(&bytes)
        .unwrap_or_else(|error| panic!("{}: {error}", path.display()));
    assert_eq!(vectors.name, "sequencer-receipt-v1");
    assert_eq!(vectors.chain_id, 125);
    assert_eq!(vectors.cases.len(), 11);
    let mut ids = BTreeSet::new();
    for case in &vectors.cases {
        assert!(ids.insert(&case.id), "duplicate case: {}", case.id);
        let _ = classify(case);
        assert_eq!(
            case.receipt_json["receipt"], case.signed_receipt,
            "{}",
            case.id
        );
    }
    vectors
}

fn hex(text: &str) -> Vec<u8> {
    let digits = text.strip_prefix("0x").expect("fixture hex prefix");
    assert_eq!(digits.len() % 2, 0, "fixture hex length");
    (0..digits.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(&digits[index..index + 2], 16).expect("fixture hex byte"))
        .collect()
}

fn hash(text: &str) -> [u8; 32] {
    hex(text).try_into().expect("fixture 32-byte hash")
}

fn quantity(value: &Value) -> u64 {
    let digits = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .expect("fixture quantity");
    u64::from_str_radix(digits, 16).expect("fixture u64 quantity")
}

fn authorization(case: &Case) -> [SequencerAuthorization; 1] {
    let key = &case.pinned_key;
    assert!(key.first_height <= key.last_height, "{}", case.id);
    [SequencerAuthorization::new(
        hash(&key.key_id),
        hash(&key.public_key),
        key.first_height,
        key.last_height,
    )]
}

fn mirror_verifier(case: &Case, network_id: u32) -> MirrorVerifier {
    let height = quantity(&case.receipt_json["height"]);
    let block = CommittedBlock {
        header: CommittedHeader {
            height,
            block_hash: hash(&case.receipt_digest),
            parent_hash: [0; 32],
            app_hash: [0; 32],
            timestamp: quantity(&case.receipt_json["timestamp"]),
            transactions: vec![hash(
                case.receipt_json["txHash"]
                    .as_str()
                    .expect("transaction hash"),
            )],
        },
        receipts: vec![hex(&case.signed_receipt)],
    };
    let archive = Archive::from_committed(
        &block,
        network_id,
        NodeHead {
            latest_sealed_batch: height,
            latest_committed: None,
        },
    )
    .unwrap_or_else(|error| panic!("{}: archive: {error:?}", case.id));
    let observation = MirrorObservation::from_archive(
        MirrorSourceId::new("shared-sequencer-vectors").expect("source id"),
        MirrorTargetIdentity::Ethereum {
            chain_id: u64::from(network_id),
            genesis_hash: [0; 32],
            contract: [0; 20],
            code_hash: [0; 32],
            publisher: [0; 20],
        },
        archive.bytes(),
        network_id,
        height,
        archive_commitment(archive.bytes()),
        MirrorCanonicalPosition::Ethereum {
            block_number: height,
            block_hash: block.header.block_hash,
            reference_head_number: height,
            reference_head_hash: block.header.block_hash,
        },
    )
    .unwrap_or_else(|error| panic!("{}: observation: {error:?}", case.id));
    MirrorVerifier::admit_committed(archive.bytes(), observation)
        .unwrap_or_else(|error| panic!("{}: mirror admission: {error:?}", case.id))
}

#[test]
fn every_shared_receipted_case_verifies_in_the_mirror() {
    let vectors = shared_vectors();
    let mut checked = 0;
    for case in &vectors.cases {
        if !matches!(classify(case), Classification::Answer) {
            continue;
        }
        assert!(matches!(
            case.expected["rung"].as_str(),
            Some("receipted" | "committed")
        ));
        assert_eq!(case.expected["verifier"], "accepts", "{}", case.id);
        let keys = authorization(case);
        let canonical = hex(&case.signed_receipt);
        let expected = verify_sequencer_signature(&canonical, ReceiptKeys::HeightRanges(&keys))
            .unwrap_or_else(|error| panic!("{}: signature: {error:?}", case.id));
        let verifier = mirror_verifier(case, vectors.chain_id);
        let verified = verifier
            .committed_receipt(0, &keys)
            .unwrap_or_else(|error| panic!("{}: mirror receipt: {error:?}", case.id));
        assert_eq!(verified.value(), &expected, "{}", case.id);
        assert_eq!(
            verified.level(),
            MirrorEvidenceLevel::Committed,
            "{}",
            case.id
        );
        assert_eq!(
            verified.batch_number(),
            quantity(&case.receipt_json["height"]),
            "{}",
            case.id
        );
        let committed = verifier
            .committed()
            .unwrap_or_else(|error| panic!("{}: committed coordinate: {error:?}", case.id));
        assert_eq!(committed.height, verified.batch_number(), "{}", case.id);
        assert_eq!(
            committed.block_hash,
            hash(&case.receipt_digest),
            "{}",
            case.id
        );
        assert!(verified.observation().is_some(), "{}", case.id);
        let unsigned = encode_unsigned(verified.value()).expect("unsigned receipt");
        assert_eq!(
            receipt_digest(&unsigned).expect("receipt digest"),
            hash(&case.receipt_digest),
            "{}",
            case.id
        );
        checked += 1;
    }
    assert_eq!(checked, 5);
}

#[test]
fn every_shared_refusal_case_is_refused_by_the_mirror() {
    let vectors = shared_vectors();
    let mut checked = 0;
    for case in &vectors.cases {
        let Classification::Refusal(expected_check) = classify(case) else {
            continue;
        };
        assert!(case.expected["refusedAt"].is_string(), "{}", case.id);
        let keys = authorization(case);
        let canonical = hex(&case.signed_receipt);
        let signature_result =
            verify_sequencer_signature(&canonical, ReceiptKeys::HeightRanges(&keys));
        let verifier = mirror_verifier(case, vectors.chain_id);
        let mirror_result = verifier.committed_receipt(0, &keys);
        match case.expected["verifier"].as_str() {
            Some("SequencerSignature" | "Decode") => {
                let expected = match case.expected["verifier"].as_str() {
                    Some("SequencerSignature") => ReceiptCheck::SequencerSignature,
                    Some("Decode") => ReceiptCheck::Decode,
                    _ => unreachable!(),
                };
                assert_eq!(expected_check, Some(expected), "{}", case.id);
                assert_eq!(
                    signature_result.map_err(|failure| failure.check),
                    Err(expected),
                    "{}: shared signature refusal",
                    case.id
                );
                assert_eq!(
                    mirror_result,
                    Err(MirrorVerifyError::Receipt(expected)),
                    "{}: mirror receipt refusal",
                    case.id
                );
            }
            Some("accepts") => {
                assert_eq!(expected_check, None, "{}", case.id);
                let signed = signature_result
                    .unwrap_or_else(|error| panic!("{}: signature: {error:?}", case.id));
                let verification = mirror_result
                    .unwrap_or_else(|error| panic!("{}: mirror receipt: {error:?}", case.id));
                assert_eq!(verification.value(), &signed, "{}", case.id);
                let protocol = verification.value().protocol().expect("protocol receipt");
                match case.id.as_str() {
                    "V3-activity-id-not-raw-hash" => {
                        assert_eq!(case.expected["refusedAt"], "R4");
                        let raw_hash: [u8; 32] =
                            Keccak256::digest(hex(&case.raw_transaction)).into();
                        assert_ne!(raw_hash, protocol.activity_id(), "{}", case.id);
                    }
                    "V3-seq-hash-mismatch" => {
                        assert_eq!(case.expected["refusedAt"], "R6");
                        let previous = hash(&case.prev_seq_hash);
                        let mut sequence_hash = Sha256::new();
                        sequence_hash.update(b"PAXEER/v1/sequence\0");
                        sequence_hash.update(previous);
                        sequence_hash.update(protocol.global_sequence().to_be_bytes());
                        sequence_hash.update(protocol.activity_id());
                        let expected: [u8; 32] = sequence_hash.finalize().into();
                        assert!(
                            protocol.resulting_state_root() != expected
                                || protocol.previous_state_root() != previous,
                            "{}: client sequence rule must refuse",
                            case.id
                        );
                    }
                    "V3-tampered-log" => assert_eq!(case.expected["refusedAt"], "R5"),
                    id => panic!("unclassified shared sequencer-receipt client refusal: {id}"),
                }
            }
            expected => panic!(
                "{}: unclassified verifier expectation: {expected:?}",
                case.id
            ),
        }
        checked += 1;
    }
    assert_eq!(checked, 6);
}
