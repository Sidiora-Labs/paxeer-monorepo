use layerx_agent_api::track::{ReceiptRef, SubmissionRef, SubmissionState, TrackedSubmission};
use layerx_human_service::journeys::{
    AgentObservation, ReceiptKeyEntry, ReceiptLookup, ReceiptMaterial,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{
    verify_sequencer_signature, AuthorizedBatch, ReceiptCheck, ReceiptKeys, VerificationFailure,
};
use layerx_types::json::{parse, JsonValue};
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};
use sha3::Keccak256;

fn hex(value: &JsonValue, path: &str) -> Vec<u8> {
    value.hex_at(path).or_fail("fixture hex")
}

fn hash(value: &JsonValue, path: &str) -> [u8; 32] {
    hex(value, path).try_into().or_fail("fixture hash width")
}

fn assert_refused_vector(case: &JsonValue, pins: &[SequencerAuthorization]) {
    let outcome =
        verify_sequencer_signature(&hex(case, "signedReceipt"), ReceiptKeys::HeightRanges(pins));
    match case
        .str_at("expected.verifier")
        .or_fail("expected verifier")
    {
        "SequencerSignature" => assert_eq!(
            outcome,
            Err(VerificationFailure {
                check: ReceiptCheck::SequencerSignature
            })
        ),
        "Decode" => assert_eq!(
            outcome,
            Err(VerificationFailure {
                check: ReceiptCheck::Decode
            })
        ),
        "accepts" => {
            let receipt = outcome.unwrap_or_else(|error| {
                panic!(
                    "{}: {error:?}",
                    "signature remains valid for envelope refusal"
                )
            });
            let protocol = receipt.protocol().or_fail("protocol receipt");
            match case.str_at("expected.refusedAt").or_fail("refusal stage") {
                "R4" => assert_ne!(
                    protocol.activity_id(),
                    <[u8; 32]>::from(Keccak256::digest(hex(case, "rawTransaction")))
                ),
                "R6" => {
                    let mut digest = Sha256::new();
                    digest.update(b"PAXEER/v1/sequence\0");
                    digest.update(hash(case, "prevSeqHash"));
                    digest.update(protocol.global_sequence().to_be_bytes());
                    digest.update(protocol.activity_id());
                    assert_ne!(
                        protocol.resulting_state_root(),
                        <[u8; 32]>::from(digest.finalize())
                    );
                }
                "R5" => {
                    let log_digest: [u8; 32] =
                        Sha256::digest(hex(case, "receiptJson.logs.0.data")).into();
                    let body = protocol.effects()[1].body();
                    assert_ne!(&body[body.len() - 32..], log_digest.as_slice());
                }
                stage => panic!("unexpected envelope refusal stage {stage}"),
            }
        }
        expected => panic!("unexpected verifier outcome {expected}"),
    }
}

fn assert_carried_material(material: &ReceiptMaterial, observation: AgentObservation) {
    let ReceiptMaterial {
        canonical_bytes: carried_bytes,
        authorised_batch: carried_batch,
        verification_level: carried_level,
        tx_hash: carried_tx_hash,
        receipt_hash: carried_receipt_hash,
        block_height: carried_height,
        rung: carried_rung,
        signed_receipt: carried_signed_receipt,
        receipt_key: carried_key,
        inclusion_height: carried_inclusion_height,
    } = observation.receipt.or_fail("journey leg receipt material");
    assert_eq!(carried_bytes, material.canonical_bytes);
    assert_eq!(carried_batch, material.authorised_batch);
    assert_eq!(carried_level, material.verification_level);
    assert_eq!(carried_tx_hash, observation.activity_id);
    assert_eq!(carried_receipt_hash, material.receipt_hash);
    assert_ne!(carried_receipt_hash, carried_tx_hash);
    assert_eq!(carried_height, material.block_height);
    assert_eq!(carried_rung, material.rung);
    assert_eq!(carried_rung, observation.submission.rung);
    assert_eq!(carried_signed_receipt, material.canonical_bytes);
    assert_eq!(carried_key, material.receipt_key);
    assert_eq!(carried_inclusion_height, material.block_height);
}

fn assert_valid_vector(case: &JsonValue, canonical_bytes: &[u8], pins: &[SequencerAuthorization]) {
    let verified = verify_sequencer_signature(canonical_bytes, ReceiptKeys::HeightRanges(pins))
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "receipt signature against independently pinned key"
            )
        });
    assert_eq!(
        layerx_intents::canonical::receipt_bytes(&verified).or_fail("canonical receipt"),
        canonical_bytes
    );
    let protocol = verified.protocol().or_fail("protocol receipt");
    let tx_hash: [u8; 32] = Keccak256::digest(hex(case, "rawTransaction")).into();
    assert_eq!(tx_hash, hash(case, "receiptJson.txHash"));
    assert_eq!(tx_hash, protocol.activity_id());
    let height = case.str_at("receiptJson.height").or_fail("receipt height");
    let block_height = u64::from_str_radix(height.strip_prefix("0x").or_fail("hex height"), 16)
        .or_fail("block height");
    assert_eq!(&protocol.batch_id()[24..], &block_height.to_be_bytes());
    let receipt_hash: [u8; 32] = Sha256::digest(canonical_bytes).into();
    let rung = Rung::from_name(case.str_at("receiptJson.rung").or_fail("receipt rung"))
        .or_fail("known rung");
    assert_eq!(rung, Rung::Receipted);
    let authorised_batch = AuthorizedBatch::from_receipt_keys(
        canonical_bytes,
        ReceiptKeys::HeightRanges(pins),
        protocol.asset(),
        protocol.previous_state_root(),
        protocol.resulting_state_root(),
    )
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "batch authority against independently pinned receipt key"
        )
    });
    let verification_level = layerx_types::verify::VerificationLevel::SEQUENCER_SIGNED;
    let receipt_key = ReceiptKeyEntry {
        key_id: hash(case, "pinnedKey.keyId"),
        public_key: hash(case, "pinnedKey.publicKey"),
        first_height: case.u64_at("pinnedKey.firstHeight").or_fail("first height"),
        last_height: case.u64_at("pinnedKey.lastHeight").or_fail("last height"),
    };
    for rung in [rung, Rung::Committed] {
        let material = ReceiptMaterial {
            canonical_bytes: canonical_bytes.to_vec(),
            signed_receipt: canonical_bytes.to_vec(),
            receipt_key,
            inclusion_height: block_height,
            authorised_batch,
            verification_level,
            tx_hash,
            receipt_hash,
            block_height,
            rung,
        };
        let lookup = ReceiptLookup::Found(material.clone());
        let ReceiptLookup::Found(found) = lookup else {
            panic!("receipt material lost during lookup");
        };
        assert_eq!(found, material);
        let observation = AgentObservation {
            submission: TrackedSubmission {
                submission_ref: SubmissionRef::new("receipt-material-submission")
                    .or_fail("submission reference"),
                state: SubmissionState::Executed {
                    receipt_ref: ReceiptRef::new("receipt-material-receipt")
                        .or_fail("receipt reference"),
                },
                evidence: Vec::new(),
                rung,
                transitions: Vec::new(),
            },
            activity_id: tx_hash,
            receipt: Some(found),
        };
        assert_carried_material(&material, observation);
    }
}

#[test]
fn journey_leg_receipt_material_carries_rung_and_receipt_hash() {
    let document = parse(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )))
    .or_fail("sequencer receipt vectors");
    let cases = document.array_at("cases").or_fail("receipt cases");
    assert!(!cases.is_empty());
    let mut observed = (0, 0);
    for case in cases {
        let canonical_bytes = hex(case, "signedReceipt");
        let pins = [SequencerAuthorization::new(
            hash(case, "pinnedKey.keyId"),
            hash(case, "pinnedKey.publicKey"),
            case.u64_at("pinnedKey.firstHeight").or_fail("first height"),
            case.u64_at("pinnedKey.lastHeight").or_fail("last height"),
        )];
        match case.str_at("vector").or_fail("fixture vector") {
            "V3" => {
                assert_refused_vector(case, &pins);
                observed.1 += 1;
                continue;
            }
            "V1" | "V2" => observed.0 += 1,
            vector => panic!("unexpected fixture vector {vector}"),
        }
        assert_valid_vector(case, &canonical_bytes, &pins);
    }
    assert_eq!(observed, (5, 6));
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
