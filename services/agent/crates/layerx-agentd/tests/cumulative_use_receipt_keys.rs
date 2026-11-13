fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::protocol_evidence::{
    CumulativeUseError, CumulativeUseWindow, EvidenceAuthority, RawActivityReceiptEvidence,
    RawReceiptEvidence, ReceiptEvidenceError,
};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_crypto::SignatureMessage;
use layerx_proof::merkle::build_proof;
use layerx_proof::receipt::{ReceiptCheck, VerificationFailure};
use layerx_types::activity::{Authority, EnvelopeBuilder, Signature, TimestampBound};
use layerx_types::amount::Amount;
use layerx_types::ids::{Did, IdempotencyKey};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::{batch_header_digest, payload_hash_for, receipt_digest, Domain};
use layerx_wire::historical::activity::{
    decode_signed, encode_signed_envelope, encode_unsigned_envelope,
};
use layerx_wire::historical::hash::activity_id;
use layerx_wire::limits::PROTOCOL_VERSION;
use layerx_wire::receipt::{
    decode, decode_batch_header, encode, encode_batch_header, encode_unsigned,
};

const NETWORK: u32 = 42;
const KEY_ID: [u8; 32] = [0x56; 32];
const AMOUNT: u128 = 25;
const WINDOW: CumulativeUseWindow = CumulativeUseWindow { first: 1, last: 2 };

fn batch_key() -> SigningKey {
    SigningKey::from_bytes(&[0x34; 32])
}

fn receipt_key() -> SigningKey {
    SigningKey::from_bytes(&[0x45; 32])
}

fn authority() -> EvidenceAuthority {
    require_ok(
        EvidenceAuthority::pinned_to_handshake(
            PROTOCOL_VERSION,
            NETWORK,
            batch_key().verifying_key().to_bytes(),
        ),
        "pinned batch authority",
    )
}

fn governed_key() -> ReceiptKeyEntry {
    ReceiptKeyEntry {
        key_id: KEY_ID,
        public_key: receipt_key().verifying_key().to_bytes(),
        first_height: 1,
        last_height: u64::MAX,
    }
}

fn signed_activity(module: ModuleId, actor: &Did) -> (Vec<u8>, [u8; 32]) {
    let signer = SigningKey::from_bytes(&[0x67; 32]);
    let kind = require_ok(ActivityType::new(module, 2), "activity kind");
    let registration = require_ok(
        ModuleRegistration::new(module, &[kind]),
        "module registration",
    );
    let registry = require_ok(ModuleRegistry::new(&[registration]), "module registry");
    let payload = require_ok(
        Payload::new(&registry, kind, b"cumulative-use receipt binding"),
        "registered payload",
    );
    let mut builder = EnvelopeBuilder::new();
    require_ok(builder.protocol_version(PROTOCOL_VERSION), "version");
    require_ok(builder.network_id(NETWORK), "network");
    require_ok(builder.activity_type(kind), "activity type");
    require_ok(builder.actor_did(actor.clone()), "actor");
    require_ok(
        builder.authority(require_ok(
            Authority::owner(&signer.verifying_key().to_bytes()),
            "owner authority",
        )),
        "authority",
    );
    require_ok(builder.account_sequence(1), "account sequence");
    require_ok(
        builder.timestamp_bound(require_ok(
            TimestampBound::new(1, 2_000),
            "timestamp interval",
        )),
        "timestamp",
    );
    require_ok(
        builder.idempotency_key(IdempotencyKey::new([0x78; 32])),
        "idempotency key",
    );
    require_ok(builder.fee_limit(Amount::from_u128(1)), "fee limit");
    require_ok(
        builder.payload_hash(require_ok(payload_hash_for(&payload), "payload hash")),
        "payload commitment",
    );
    require_ok(builder.payload(payload), "payload");
    let envelope = require_ok(builder.build(), "unsigned envelope");
    let unsigned = require_ok(
        encode_unsigned_envelope(&envelope),
        "unsigned wire envelope",
    );
    let digest = require_ok(
        SignatureMessage::new(
            Domain::SignaturePreimage,
            PROTOCOL_VERSION,
            NETWORK,
            &unsigned,
        ),
        "activity signature message",
    )
    .digest();
    let signature = require_ok(
        Signature::new(&signer.sign(&digest).to_bytes()),
        "activity signature",
    );
    let canonical = require_ok(
        encode_signed_envelope(&envelope.attach_signature(signature)),
        "signed wire envelope",
    );
    let activity = require_ok(
        decode_signed(&canonical, &registry),
        "decoded wire activity",
    );
    let id = require_ok(activity_id(&activity), "canonical activity identity");
    (canonical, id)
}

fn signed_receipt(module: u16, id: [u8; 32], sequence: u64, signer: &SigningKey) -> Vec<u8> {
    let encode_receipt = |signature: Option<[u8; 64]>| {
        let mut encoder = Encoder::new(4_096);
        require_ok(
            encoder.structure_header_version(0x5201, PROTOCOL_VERSION),
            "receipt header",
        );
        require_ok(encoder.u16(PROTOCOL_VERSION), "receipt version");
        require_ok(encoder.bytes(&id, 32), "activity id");
        require_ok(encoder.u64(sequence), "global sequence");
        for root in [[0x11; 32], [0x22; 32], [0x33; 32]] {
            require_ok(encoder.bytes(&root, 32), "receipt root");
        }
        require_ok(encoder.i32(0), "successful result");
        require_ok(encoder.sequence_length(0, 512), "empty effects");
        require_ok(encoder.u128(1), "fee charged");
        let mut height = [0; 32];
        height[24..].copy_from_slice(&7_u64.to_be_bytes());
        require_ok(encoder.bytes(&height, 32), "receipt height");
        require_ok(encoder.u16(module), "module id");
        require_ok(encoder.u32(1), "module version");
        require_ok(encoder.u32(0), "parameter version");
        require_ok(encoder.u8(2), "operation");
        require_ok(encoder.bytes(&[0x81; 32], 32), "asset");
        require_ok(encoder.u128(AMOUNT), "amount");
        require_ok(encoder.bytes(&[0x82; 32], 32), "debit account");
        require_ok(encoder.u128(100), "debit before");
        require_ok(encoder.u128(74), "debit after amount and fee");
        require_ok(encoder.u64(1), "debit sequence");
        require_ok(encoder.bytes(&[0x83; 32], 32), "credit account");
        require_ok(encoder.u128(0), "credit before");
        require_ok(encoder.u128(AMOUNT), "credit after");
        require_ok(encoder.bytes(&[0; 32], 32), "transfer set root");
        require_ok(encoder.bytes(&[0x84; 32], 32), "authorization hash");
        require_ok(encoder.bytes(&KEY_ID, 32), "receipt key context");
        require_ok(encoder.u64(1_000), "timestamp");
        require_ok(
            encoder.u8(u8::from(signature.is_some())),
            "signature presence",
        );
        if let Some(signature) = signature {
            require_ok(encoder.bytes(&signature, 64), "receipt signature");
        }
        encoder.finish()
    };
    let unsigned = encode_receipt(None);
    let decoded = require_ok(decode(&unsigned), "unsigned receipt decodes");
    assert_eq!(
        require_ok(encode_unsigned(&decoded), "canonical unsigned receipt"),
        unsigned
    );
    let signature = signer
        .sign(&require_ok(receipt_digest(&unsigned), "receipt digest"))
        .to_bytes();
    let canonical = encode_receipt(Some(signature));
    let decoded = require_ok(decode(&canonical), "signed receipt decodes");
    assert_eq!(
        require_ok(encode(&decoded), "canonical signed receipt"),
        canonical
    );
    canonical
}

fn entry(module: u16, signer: &SigningKey) -> (Did, RawActivityReceiptEvidence) {
    let actor = require_ok(Did::new(b"did:layerx:receipt-key-owner"), "actor DID");
    let activity_module = if module > 11 {
        ModuleId::Asset
    } else {
        require_ok(ModuleId::from_u16(module), "kernel module")
    };
    let (activity, id) = signed_activity(activity_module, &actor);
    let receipt = signed_receipt(module, id, 1, signer);
    let tail = signed_receipt(1, [0xfe; 32], 2, &batch_key());
    let (proof, receipt_root) =
        require_ok(build_proof(&[&receipt, &tail], 0), "two-leaf receipt tree");
    let (_, activity_root) = require_ok(build_proof(&[&activity], 0), "activity tree");
    let key = batch_key();
    let mut header = Encoder::new(354);
    require_ok(
        header.structure_header_version(0x1701, PROTOCOL_VERSION),
        "batch header",
    );
    require_ok(header.u8(15), "batch fields");
    require_ok(header.tag(1, 15), "version tag");
    require_ok(header.u16(PROTOCOL_VERSION), "batch version");
    require_ok(header.tag(2, 15), "network tag");
    require_ok(header.u32(NETWORK), "batch network");
    for (tag, value) in [(3, 1), (4, 7), (5, 1), (6, 2)] {
        require_ok(header.tag(tag, 15), "integer tag");
        require_ok(header.u64(value), "batch integer");
    }
    for (tag, value) in [
        (7, [0x11; 32]),
        (8, [0x22; 32]),
        (9, activity_root),
        (10, receipt_root),
        (11, [0x41; 32]),
        (12, [0x42; 32]),
        (13, [0x43; 32]),
    ] {
        require_ok(header.tag(tag, 15), "commitment tag");
        require_ok(header.bytes(&value, 32), "batch commitment");
    }
    require_ok(header.tag(14, 15), "timestamp tag");
    require_ok(header.u64(1_000), "batch timestamp");
    require_ok(header.tag(15, 15), "sequencer tag");
    require_ok(
        header.bytes(&key.verifying_key().to_bytes(), 32),
        "batch sequencer",
    );
    let header = header.finish();
    let decoded = require_ok(decode_batch_header(&header), "batch header decodes");
    assert_eq!(
        require_ok(encode_batch_header(&decoded), "canonical batch header"),
        header
    );
    let signature = key
        .sign(&require_ok(batch_header_digest(&header), "batch digest"))
        .to_bytes();
    (
        actor,
        RawActivityReceiptEvidence::from_signed_inclusion(
            activity,
            RawReceiptEvidence::new(receipt, proof, header, signature),
        ),
    )
}

#[test]
fn governed_receipt_key_authenticates_node_cumulative_use() {
    let key = governed_key();
    assert_ne!(key.public_key, batch_key().verifying_key().to_bytes());
    let authority = require_ok(
        authority().with_receipt_keys(vec![key]),
        "governed receipt key",
    );
    let (actor, entry) = entry(12, &receipt_key());
    let decoded = require_ok(decode(entry.receipt().canonical_receipt()), "node receipt");
    let protocol = require_some(decoded.protocol(), "protocol receipt");
    assert_eq!(protocol.module_id(), 12);
    assert_eq!(protocol.operation(), 2);
    assert_eq!(protocol.context_hash(), KEY_ID);
    let facts = require_ok(
        authority.authenticate_cumulative_use(&actor, WINDOW, &[entry]),
        "governed node receipt authenticates the cumulative-use window",
    );
    assert_eq!(facts.count(), 1);
    assert_eq!(facts.amount(), AMOUNT);
}

#[test]
fn node_cumulative_use_without_governed_keys_refuses_sequencer_signature() {
    let (actor, entry) = entry(12, &receipt_key());
    assert!(matches!(
        authority().authenticate_cumulative_use(&actor, WINDOW, &[entry]),
        Err(CumulativeUseError::Receipt(ReceiptEvidenceError::Receipt(
            VerificationFailure {
                check: ReceiptCheck::SequencerSignature,
            }
        )))
    ));
}

#[test]
fn node_cumulative_use_refuses_activity_receipt_mismatch() {
    let authority = require_ok(
        authority().with_receipt_keys(vec![governed_key()]),
        "governed receipt key",
    );
    let (actor, entry) = entry(12, &receipt_key());
    let other_actor = require_ok(
        Did::new(b"did:layerx:different-activity"),
        "different actor DID",
    );
    let (other_activity, other_id) = signed_activity(ModuleId::Asset, &other_actor);
    let receipt = entry.receipt().clone();
    assert_ne!(
        other_id,
        require_some(
            decode(receipt.canonical_receipt())
                .unwrap_or_else(|error| panic!("{}: {error:?}", "node receipt"))
                .protocol(),
            "protocol receipt"
        )
        .activity_id()
    );
    let mismatched = RawActivityReceiptEvidence::from_signed_inclusion(other_activity, receipt);
    assert!(matches!(
        authority.authenticate_cumulative_use(&actor, WINDOW, &[mismatched]),
        Err(CumulativeUseError::ActivityReceiptMismatch)
    ));
}

#[test]
fn kernel_cumulative_use_keeps_the_batch_authorization_key() {
    for module in [1, 11] {
        let (actor, entry) = entry(module, &batch_key());
        for authority in [
            authority(),
            require_ok(
                authority().with_receipt_keys(vec![governed_key()]),
                "independent governed receipt key",
            ),
        ] {
            let facts = require_ok(
                authority.authenticate_cumulative_use(&actor, WINDOW, std::slice::from_ref(&entry)),
                "historical kernel receipt remains authenticated by its batch key",
            );
            assert_eq!(facts.count(), 1);
            assert_eq!(facts.amount(), AMOUNT);
        }
    }
}
