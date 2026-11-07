//! Task record 22.33.1522, audit 22.30.92, acceptance 22998.1.
//! wire_activity_retired: wire_activity_retired_native_receipts_need_no_envelope.
//! wire_activity_hash: wire_activity_hash_preserves_native_identity_and_accounts.
//! wire_exports: wire_exports_archive_codec_is_explicitly_historical.
//! checkpoint_codec_bounds: checkpoint_codec_bounds_maximum_fields_round_trip,
//! checkpoint_codec_bounds_refuses_the_next_byte_and_field_element.
//! wire_sign_retired: wire_sign_retired_keeps_receipt_signature_verification.
//!
//! Baseline: not executed. The ordered gate instructions prohibit tests before
//! complete implementation and direct verify runs; this target did not exist
//! in the starting tree. No baseline test failures are claimed.
//! Evidence: hf-build/hf-done record the candidate revision, exact command, exit
//! and private log location in /out/run/gates.jsonl; no gate is claimed here.
//! Repair 1: the audit's stated total omitted the four-byte signature count.
//! The full declared layout is exactly 1,053,298 bytes; the codec's derived
//! maximum and all field limits are unchanged by this assertion correction.
//!
//! Boundary: deprecated source aliases remain until the following out-of-scope
//! callers migrate; they expose the same historical implementation, not a native
//! codec. The no-custody wire dev-dependency cannot call the custody-gated EVM
//! producer; its real Go-produced transaction/receipt transcripts are used here.
//! Wiring request (domain 9): layerx-crypto/src/{disclosure,onboarding,send}.rs
//! must move archive imports to historical and current callers to evm_transaction
//! and disclosure::call before removing wire's deprecated source aliases.
//! Wiring request (domain 9): layerx-agentd/src/{ops/program,protocol_evidence,
//! human_runtime,human_runtime/native_receipt,prepare/mod,sign_legacy,approval/native_effect,
//! approval/program_requirement}.rs must use native transaction hashes, signing
//! digests and bound disclosures for live calls, and historical for archives.
//! Wiring request (domain 9): layerx-sdk/src/{program_http,programs/native_activity}.rs
//! must use native transaction identities and call-bound disclosures.
//! Wiring request (domain 9): layerx-wire/tests/{hashing,rejection,structures,
//! wallet_vectors}.rs and archive consumers in layerx-{crypto,agentd}/tests must
//! import historical before deleting the deprecated aliases; retain every test.
//! Paths above are relative to services/agent/crates.

use std::fmt::Debug;

use layerx_crypto::{ed25519, VerifyError};
use layerx_types::account::AccountId;
use layerx_types::json::{decode_hex, parse, JsonValue};
use layerx_types::payload::{ActivityType, ModuleRegistration, ModuleRegistry};
use layerx_types::result::KnownResult;
use layerx_wire::hash::{account_id_for_protocol, receipt_digest, Domain};
use layerx_wire::historical::{activity, hash as archive_hash, sign as archive_sign};
use layerx_wire::limits::MAX_MESSAGE_BYTES;
use layerx_wire::receipt::{
    self, decode_batch_header, decode_checkpoint, encode_batch_header, encode_checkpoint,
    BatchHeader, CheckpointCertificate, MAX_CHECKPOINT_BYTES,
};
use sha2::{Digest as _, Sha256};

const NATIVE_VECTORS: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");
const WALLET_VECTORS: &str = include_str!("support/wallet_vectors/activities.json");
const CHECKPOINT_VECTOR: &str = include_str!("../../../../../tests/vectors/checkpoint/fresh.json");

fn must<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}

fn refused<T: Debug, E>(value: Result<T, E>) -> E {
    match value {
        Err(error) => error,
        Ok(value) => panic!("unexpected acceptance: {value:?}"),
    }
}

fn digest(tag: &[u8], bytes: &[u8]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(tag);
    hasher.update(bytes);
    hasher.finalize().into()
}

fn unprefixed_hex(value: &JsonValue, path: &str) -> Vec<u8> {
    must(decode_hex(&format!("0x{}", must(value.str_at(path)))))
}

fn registry(case: &JsonValue) -> ModuleRegistry {
    let kind = must(ActivityType::from_u32(must(u32::try_from(must(
        case.u64_at("activity_type"),
    )))));
    must(ModuleRegistry::new(&[must(ModuleRegistration::new(
        kind.module(),
        &[kind],
    ))]))
}

fn header() -> BatchHeader {
    let vector = must(parse(CHECKPOINT_VECTOR));
    must(decode_batch_header(&must(vector.hex_at("header.bytes"))))
}

fn signatures(count: u16) -> Vec<([u8; 32], [u8; 64])> {
    let vector = must(parse(CHECKPOINT_VECTOR));
    let attestations = must(vector.array_at("attestations"));
    (1..=count)
        .map(|index| {
            let mut id = [0; 32];
            id[30..].copy_from_slice(&index.to_be_bytes());
            let signature = must(attestations[0].hex_array_at("signature"));
            (id, signature)
        })
        .collect()
}

#[test]
fn wire_activity_retired_native_receipts_need_no_envelope() {
    let vectors = must(parse(NATIVE_VECTORS));
    let cases = must(vectors.array_at("cases"));
    let wallet = must(parse(WALLET_VECTORS));
    let archives = must(wallet.array_at("activities"));
    let archive_registry = registry(&archives[0]);
    for case in &cases[..7] {
        assert_eq!(must(case.str_at("expected.verifier")), "accepts");
        let raw = must(case.hex_at("rawTransaction"));
        assert_eq!(raw[0], 2);
        assert!(activity::decode_signed(&raw, &archive_registry).is_err());
        assert!(activity::decode_unsigned(&raw, &archive_registry).is_err());
        let produced = must(case.hex_at("signedReceipt"));
        let transported = must(receipt::encode(&must(receipt::decode(&produced))));
        let consumed = must(receipt::decode(&transported));
        assert_eq!(transported, produced);
        let fields = consumed
            .protocol()
            .unwrap_or_else(|| panic!("full receipt"));
        assert_eq!(fields.module_id(), 12);
        assert_eq!(fields.protocol_version(), 2);
        assert_eq!(
            fields.activity_id(),
            must(case.hex_array_at::<32>("receiptJson.txHash"))
        );
    }
}

#[test]
fn wire_activity_hash_preserves_native_identity_and_accounts() {
    let vectors = must(parse(NATIVE_VECTORS));
    for case in &must(vectors.array_at("cases"))[..7] {
        let raw = must(case.hex_at("rawTransaction"));
        let receipt = must(receipt::decode(&must(case.hex_at("signedReceipt"))));
        let fields = receipt.protocol().unwrap_or_else(|| panic!("full receipt"));
        let native_hash: [u8; 32] = must(case.hex_array_at("receiptJson.txHash"));
        assert_eq!(fields.activity_id(), native_hash);
        assert_ne!(native_hash, digest(Domain::ActivityId.tag(), &raw));
        let encoded = must(receipt::encode(&receipt));
        assert_eq!(must(receipt::decode(&encoded)), receipt);
    }

    let wallet = must(parse(WALLET_VECTORS));
    for case in must(wallet.array_at("accounts")) {
        let account = must(AccountId::parse(must(case.str_at("name"))));
        assert_eq!(
            must(account_id_for_protocol(&account, 3)).as_slice(),
            unprefixed_hex(case, "account_id")
        );
    }
}

#[test]
fn wire_exports_archive_codec_is_explicitly_historical() {
    let wallet = must(parse(WALLET_VECTORS));
    let cases = must(wallet.array_at("activities"));
    for case in &cases[..5] {
        let signed = unprefixed_hex(case, "signed.hex");
        let unsigned = unprefixed_hex(case, "unsigned.hex");
        let registry = registry(case);
        let archived = must(activity::decode_signed(&signed, &registry));
        assert_eq!(must(activity::encode_signed(&archived)), signed);
        assert_eq!(must(activity::encode_unsigned(&archived)), unsigned);
        let transported = must(archive_hash::canonical_activity(&archived));
        let consumed = must(activity::decode_signed(transported.as_bytes(), &registry));
        assert_eq!(consumed, archived);
        assert_eq!(
            must(archive_hash::activity_id(&consumed)),
            digest(Domain::ActivityId.tag(), &signed)
        );
        assert_eq!(
            must(archive_hash::payload_hash(&consumed)).as_slice(),
            unprefixed_hex(case, "payload_hash")
        );
        assert_eq!(
            must(archive_sign::preimage(&consumed))
                .as_bytes()
                .as_slice(),
            unprefixed_hex(case, "signature_preimage")
        );
        assert_eq!(
            must(activity::encode_unsigned(&must(activity::decode_unsigned(
                &unsigned, &registry
            )))),
            unsigned
        );
    }
}

#[test]
fn wire_sign_retired_keeps_receipt_signature_verification() {
    let vectors = must(parse(NATIVE_VECTORS));
    for case in &must(vectors.array_at("cases"))[..7] {
        let produced = must(case.hex_at("signedReceipt"));
        let transported = must(receipt::encode(&must(receipt::decode(&produced))));
        let consumed = must(receipt::decode(&transported));
        let unsigned = must(receipt::encode_unsigned(&consumed));
        let fields = consumed
            .protocol()
            .unwrap_or_else(|| panic!("full receipt"));
        let signature = fields
            .sequencer_signature()
            .unwrap_or_else(|| panic!("sequencer signature"));
        let public_key = must(case.hex_array_at::<32>("pinnedKey.publicKey"));
        let receipt_hash = must(receipt_digest(&unsigned));
        assert_eq!(receipt_hash, must(case.hex_array_at::<32>("receiptDigest")));
        assert_eq!(
            ed25519::verify_digest(&public_key, &signature, &receipt_hash),
            Ok(())
        );
        assert_eq!(
            ed25519::verify_digest(
                &public_key,
                &signature,
                &digest(Domain::SignaturePreimage.tag(), &unsigned)
            ),
            Err(VerifyError::BadSignature)
        );
        let mut changed = unsigned;
        changed[10] ^= 1;
        assert_eq!(
            ed25519::verify_digest(&public_key, &signature, &must(receipt_digest(&changed))),
            Err(VerifyError::BadSignature)
        );
    }
}

#[test]
fn checkpoint_codec_bounds_maximum_fields_round_trip() {
    assert_eq!(MAX_CHECKPOINT_BYTES, 1_053_298);
    let produced = must(CheckpointCertificate::new(
        header(),
        vec![0x5a; MAX_MESSAGE_BYTES],
        signatures(32),
        32,
        vec![0xa5; 1024],
    ));
    let transported = must(encode_checkpoint(&produced));
    assert_eq!(transported.len(), MAX_CHECKPOINT_BYTES);
    let consumed = must(decode_checkpoint(&transported));
    assert_eq!(consumed, produced);
    assert_eq!(consumed.validity_proof(), vec![0x5a; MAX_MESSAGE_BYTES]);
    assert_eq!(consumed.guarantor_signatures(), signatures(32));
    assert_eq!(consumed.threshold(), 32);
    assert_eq!(consumed.settlement_reference(), vec![0xa5; 1024]);
    assert_eq!(must(encode_batch_header(consumed.header())).len(), 354);
    assert_eq!(must(encode_checkpoint(&consumed)), transported);
}

#[test]
fn checkpoint_codec_bounds_refuses_the_next_byte_and_field_element() {
    let certificate = must(CheckpointCertificate::new(
        header(),
        vec![0x5a; MAX_MESSAGE_BYTES],
        signatures(32),
        32,
        vec![0xa5; 1024],
    ));
    let bytes = must(encode_checkpoint(&certificate));
    let mut beyond = bytes.clone();
    beyond.push(0);
    let error = refused(decode_checkpoint(&beyond));
    assert_eq!(error.result, KnownResult::LengthLimit.into());
    assert_eq!(error.offset, 0);

    for (offset, length) in [
        (354, u32::try_from(MAX_MESSAGE_BYTES + 1)),
        (354 + 4 + MAX_MESSAGE_BYTES, Ok(33)),
        (bytes.len() - 1024 - 4, Ok(1025)),
    ] {
        let mut changed = bytes.clone();
        changed[offset..offset + 4].copy_from_slice(&must(length).to_be_bytes());
        let error = refused(decode_checkpoint(&changed));
        assert_eq!(error.result, KnownResult::LengthLimit.into());
        assert_eq!(error.offset, offset);
    }

    for (proof, signatures, reference) in [
        (MAX_MESSAGE_BYTES + 1, 32, 1024),
        (MAX_MESSAGE_BYTES, 33, 1024),
        (MAX_MESSAGE_BYTES, 32, 1025),
    ] {
        let error = refused(CheckpointCertificate::new(
            header(),
            vec![0x5a; proof],
            self::signatures(signatures),
            32,
            vec![0xa5; reference],
        ));
        assert_eq!(error.result, KnownResult::LengthLimit.into());
    }

    for threshold in [0, 33] {
        assert_eq!(
            refused(CheckpointCertificate::new(
                header(),
                vec![],
                signatures(32),
                threshold,
                vec![]
            ))
            .result,
            KnownResult::AttestationThreshold.into()
        );
    }
    let mut duplicate = signatures(2);
    duplicate[1].0 = duplicate[0].0;
    assert_eq!(
        refused(CheckpointCertificate::new(
            header(),
            vec![],
            duplicate,
            1,
            vec![]
        ))
        .result,
        KnownResult::UnsortedSequence.into()
    );
    let small = must(CheckpointCertificate::new(
        header(),
        vec![],
        signatures(1),
        1,
        vec![],
    ));
    let mut trailing = must(encode_checkpoint(&small));
    trailing.push(0);
    assert_eq!(
        refused(decode_checkpoint(&trailing)).result,
        KnownResult::TrailingBytes.into()
    );
}
