//! Proves: PortableReceiptV2::{export_with_transaction, from_sequencer_receipt} -> PortableReceiptV2::to_json / verify_portable_json -> PortableVerifiedReceiptV2::raw_transaction
//!
//! The input recording is emitted by rpc/sequencer_receipt_vectors_test.go using
//! node/sequencer's real receipt builder and signer. No portable proof is loaded:
//! each exporter builds it here, then is dropped before the public JSON consumer
//! decodes and verifies its bytes. There is no persistent state; two independent
//! passes through every recorded successful and reverted outcome prove repeat use.
//!
//! Repairs services/interop/crates/layerx-portable/src/receipt.rs,
//! PortableReceiptV2::from_sequencer_receipt: selecting the first matching pin
//! discarded duplicate key ids before verification. The exporter must enforce
//! the same complete pinned-key policy as the JSON consumer, including ambiguity.

use layerx_portable::{
    verify_portable_json, PortableReceiptError, PortableReceiptV2, PortableVerified,
    PortableVerifiedReceiptV2, ReceiptKeyEntry, PORTABLE_RECEIPT_FORMAT,
};
use layerx_proof::receipt::{ReceiptCheck, VerificationFailure};
use serde::Deserialize;
use serde_json::Value;
use sha3::{Digest as _, Keccak256};

#[derive(Deserialize)]
struct Recording {
    provenance: Provenance,
    cases: Vec<RecordedReceipt>,
}

#[derive(Deserialize)]
struct Provenance {
    generator: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct RecordedReceipt {
    id: String,
    vector: String,
    raw_transaction: String,
    signed_receipt: String,
    receipt_digest: String,
    receipt_json: Value,
    pinned_key: RecordedKey,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct RecordedKey {
    key_id: String,
    public_key: String,
    first_height: u64,
    last_height: u64,
}

fn recording() -> Vec<RecordedReceipt> {
    let recording: Recording = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .expect("real sequencer recording decodes");
    assert_eq!(
        recording.provenance.generator,
        "rpc/sequencer_receipt_vectors_test.go"
    );
    recording
        .cases
        .into_iter()
        .filter(|case| matches!(case.vector.as_str(), "V1" | "V2"))
        .collect()
}

fn bytes(encoded: &str) -> Vec<u8> {
    let digits = encoded.strip_prefix("0x").expect("recorded hex prefix");
    assert_eq!(digits.len() % 2, 0);
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("ASCII hex"), 16)
                .expect("recorded hex digits")
        })
        .collect()
}

fn quantity(encoded: &Value) -> u64 {
    u64::from_str_radix(
        encoded
            .as_str()
            .expect("recorded quantity")
            .strip_prefix("0x")
            .expect("quantity prefix"),
        16,
    )
    .expect("recorded quantity digits")
}

impl RecordedReceipt {
    fn key(&self) -> ReceiptKeyEntry {
        ReceiptKeyEntry::new(
            bytes(&self.pinned_key.key_id).try_into().expect("key id"),
            bytes(&self.pinned_key.public_key)
                .try_into()
                .expect("public key"),
            self.pinned_key.first_height,
            self.pinned_key.last_height,
        )
    }

    fn export(&self) -> PortableReceiptV2 {
        PortableReceiptV2::export_with_transaction(
            &bytes(&self.signed_receipt),
            &self.key(),
            quantity(&self.receipt_json["height"]),
            &bytes(&self.raw_transaction),
        )
        .expect("real signed outcome exports with its transaction")
    }

    fn assert_observed(&self, observed: &PortableVerifiedReceiptV2, carries_raw: bool) {
        let raw = bytes(&self.raw_transaction);
        assert_eq!(
            observed.raw_transaction(),
            if carries_raw {
                Some(raw.as_slice())
            } else {
                None
            },
            "{} transaction presence and bytes",
            self.id
        );
        assert_eq!(observed.signed_receipt(), bytes(&self.signed_receipt));
        assert_eq!(
            observed.receipt_digest().as_slice(),
            bytes(&self.receipt_digest)
        );
        assert_eq!(observed.receipt_key(), &self.key());
        let activity_id: [u8; 32] = Keccak256::digest(&raw).into();
        assert_eq!(observed.activity_id(), activity_id);
        assert_eq!(
            observed.activity_id().as_slice(),
            bytes(
                self.receipt_json["txHash"]
                    .as_str()
                    .expect("transaction hash")
            )
        );
        assert_eq!(
            observed.global_sequence(),
            quantity(&self.receipt_json["seq"])
        );
        assert_eq!(observed.height(), quantity(&self.receipt_json["height"]));
        assert_eq!(observed.inclusion_height(), observed.height());
        assert_eq!(
            observed.result_code(),
            if self.vector == "V2" { 1 } else { 0 }
        );
    }
}

fn signature_refusal() -> PortableReceiptError {
    PortableReceiptError::Receipt(VerificationFailure {
        check: ReceiptCheck::SequencerSignature,
    })
}

#[test]
fn proof_portable_receipt_producers_reach_the_consumer_twice() {
    let cases = recording();
    assert_eq!(cases.len(), 7, "all real successful and reverted outcomes");
    for _ in 0..2 {
        for case in &cases {
            let raw = bytes(&case.raw_transaction);
            let key = case.key();
            let keys = [
                ReceiptKeyEntry::new([0x99; 32], key.public_key(), 0, u64::MAX),
                key,
            ];
            let direct_json = case.export().to_json().expect("producer JSON transport");
            for transaction in [Some(raw.as_slice()), None] {
                let rpc_json = PortableReceiptV2::from_sequencer_receipt(
                    &case.receipt_json,
                    &keys,
                    transaction,
                )
                .expect("RPC answer exports under independently pinned keys")
                .to_json()
                .expect("RPC producer JSON transport");
                if transaction.is_some() {
                    assert_eq!(rpc_json, direct_json, "both producers emit the same proof");
                }
                // Both temporary producer objects have been dropped. Only wire
                // bytes and independently supplied pins reach the consumer.
                for (json, carries_raw) in
                    [(&direct_json, true), (&rpc_json, transaction.is_some())]
                {
                    let PortableVerified::V2(observed) = verify_portable_json(json, &keys)
                        .expect("public format dispatch decodes and verifies the producer output");
                    case.assert_observed(&observed, carries_raw);
                }
            }
        }
    }
}

#[test]
fn proof_portable_receipt_consumer_refuses_corrupted_transport_and_wrong_pins() {
    let case = recording().remove(0);
    let key = case.key();
    let json = case.export().to_json().expect("producer JSON");
    let original: Value = serde_json::from_slice(&json).expect("producer wire object");
    assert_eq!(original["format"], PORTABLE_RECEIPT_FORMAT);
    for _ in 0..2 {
        // A valid observation cannot cache trust for the next decode.
        let PortableVerified::V2(observed) =
            verify_portable_json(&json, &[key]).expect("original verifies");
        case.assert_observed(&observed, true);
        let mut malformed = original.clone();
        malformed["rawTransaction"] = "0xAB".into();
        assert_eq!(
            verify_portable_json(
                &serde_json::to_vec(&malformed).expect("malformed wire"),
                &[key]
            ),
            Err(PortableReceiptError::InvalidHex("rawTransaction"))
        );
        malformed["rawTransaction"] = "0x00".into();
        assert_eq!(
            verify_portable_json(
                &serde_json::to_vec(&malformed).expect("substituted wire"),
                &[key]
            ),
            Err(PortableReceiptError::RawTransactionMismatch)
        );
        assert_eq!(
            verify_portable_json(&json[..json.len() - 1], &[key]),
            Err(PortableReceiptError::JsonShape)
        );
        let mut wrong_public_key = key.public_key();
        wrong_public_key[0] ^= 1;
        let wrong_signer = ReceiptKeyEntry::new(
            key.sequencer_id(),
            wrong_public_key,
            key.first_batch_number(),
            key.last_batch_number(),
        );
        assert_eq!(
            verify_portable_json(&json, &[wrong_signer]),
            Err(signature_refusal())
        );
        assert_eq!(verify_portable_json(&json, &[]), Err(signature_refusal()));
    }
}

#[test]
fn proof_portable_receipt_export_preserves_the_full_pinned_key_policy() {
    let case = recording().remove(0);
    let key = case.key();
    let raw = bytes(&case.raw_transaction);
    let json = case
        .export()
        .to_json()
        .expect("unambiguous producer output");
    let mut wrong_public_key = key.public_key();
    wrong_public_key[0] ^= 1;
    let conflicting = ReceiptKeyEntry::new(
        key.sequencer_id(),
        wrong_public_key,
        key.first_batch_number(),
        key.last_batch_number(),
    );
    for pins in [[key, key], [key, conflicting], [conflicting, key]] {
        for transaction in [Some(raw.as_slice()), None] {
            assert_eq!(
                PortableReceiptV2::from_sequencer_receipt(&case.receipt_json, &pins, transaction),
                Err(signature_refusal()),
                "export must not discard ambiguous pins before verification"
            );
        }
        assert_eq!(verify_portable_json(&json, &pins), Err(signature_refusal()));
    }
    let mut wrong_raw = raw.clone();
    wrong_raw[0] ^= 1;
    assert_eq!(
        PortableReceiptV2::export_with_transaction(
            &bytes(&case.signed_receipt),
            &key,
            quantity(&case.receipt_json["height"]),
            &wrong_raw
        ),
        Err(PortableReceiptError::RawTransactionMismatch)
    );
    let submitted = serde_json::json!({"rung": "submitted", "txHash": case.receipt_json["txHash"], "nonce": "0x0"});
    assert_eq!(
        PortableReceiptV2::from_sequencer_receipt(&submitted, &[key], Some(&raw)),
        Err(PortableReceiptError::NotReceipted)
    );
}
