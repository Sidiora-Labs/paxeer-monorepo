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
use layerx_wire::activity::{decode_signed, encode_signed_envelope, encode_unsigned_envelope};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::{
    activity_id, batch_header_digest, payload_hash_for, receipt_digest, Domain,
};
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
    EvidenceAuthority::pinned_to_handshake(
        PROTOCOL_VERSION,
        NETWORK,
        batch_key().verifying_key().to_bytes(),
    )
    .expect("pinned batch authority")
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
    let kind = ActivityType::new(module, 2).expect("activity kind");
    let registration = ModuleRegistration::new(module, &[kind]).expect("module registration");
    let registry = ModuleRegistry::new(&[registration]).expect("module registry");
    let payload = Payload::new(&registry, kind, b"cumulative-use receipt binding")
        .expect("registered payload");
    let mut builder = EnvelopeBuilder::new();
    builder.protocol_version(PROTOCOL_VERSION).expect("version");
    builder.network_id(NETWORK).expect("network");
    builder.activity_type(kind).expect("activity type");
    builder.actor_did(actor.clone()).expect("actor");
    builder
        .authority(Authority::owner(&signer.verifying_key().to_bytes()).expect("owner authority"))
        .expect("authority");
    builder.account_sequence(1).expect("account sequence");
    builder
        .timestamp_bound(TimestampBound::new(1, 2_000).expect("timestamp interval"))
        .expect("timestamp");
    builder
        .idempotency_key(IdempotencyKey::new([0x78; 32]))
        .expect("idempotency key");
    builder.fee_limit(Amount::from_u128(1)).expect("fee limit");
    builder
        .payload_hash(payload_hash_for(&payload).expect("payload hash"))
        .expect("payload commitment");
    builder.payload(payload).expect("payload");
    let envelope = builder.build().expect("unsigned envelope");
    let unsigned = encode_unsigned_envelope(&envelope).expect("unsigned wire envelope");
    let digest = SignatureMessage::new(
        Domain::SignaturePreimage,
        PROTOCOL_VERSION,
        NETWORK,
        &unsigned,
    )
    .expect("activity signature message")
    .digest();
    let signature = Signature::new(&signer.sign(&digest).to_bytes()).expect("activity signature");
    let canonical = encode_signed_envelope(&envelope.attach_signature(signature))
        .expect("signed wire envelope");
    let activity = decode_signed(&canonical, &registry).expect("decoded wire activity");
    let id = activity_id(&activity).expect("canonical activity identity");
    (canonical, id)
}

fn signed_receipt(module: u16, id: [u8; 32], sequence: u64, signer: &SigningKey) -> Vec<u8> {
    let encode_receipt = |signature: Option<[u8; 64]>| {
        let mut encoder = Encoder::new(4_096);
        encoder
            .structure_header_version(0x5201, PROTOCOL_VERSION)
            .expect("receipt header");
        encoder.u16(PROTOCOL_VERSION).expect("receipt version");
        encoder.bytes(&id, 32).expect("activity id");
        encoder.u64(sequence).expect("global sequence");
        for root in [[0x11; 32], [0x22; 32], [0x33; 32]] {
            encoder.bytes(&root, 32).expect("receipt root");
        }
        encoder.i32(0).expect("successful result");
        encoder.sequence_length(0, 512).expect("empty effects");
        encoder.u128(1).expect("fee charged");
        let mut height = [0; 32];
        height[24..].copy_from_slice(&7_u64.to_be_bytes());
        encoder.bytes(&height, 32).expect("receipt height");
        encoder.u16(module).expect("module id");
        encoder.u32(1).expect("module version");
        encoder.u32(0).expect("parameter version");
        encoder.u8(2).expect("operation");
        encoder.bytes(&[0x81; 32], 32).expect("asset");
        encoder.u128(AMOUNT).expect("amount");
        encoder.bytes(&[0x82; 32], 32).expect("debit account");
        encoder.u128(100).expect("debit before");
        encoder.u128(74).expect("debit after amount and fee");
        encoder.u64(1).expect("debit sequence");
        encoder.bytes(&[0x83; 32], 32).expect("credit account");
        encoder.u128(0).expect("credit before");
        encoder.u128(AMOUNT).expect("credit after");
        encoder.bytes(&[0; 32], 32).expect("transfer set root");
        encoder.bytes(&[0x84; 32], 32).expect("authorization hash");
        encoder.bytes(&KEY_ID, 32).expect("receipt key context");
        encoder.u64(1_000).expect("timestamp");
        encoder
            .u8(u8::from(signature.is_some()))
            .expect("signature presence");
        if let Some(signature) = signature {
            encoder.bytes(&signature, 64).expect("receipt signature");
        }
        encoder.finish()
    };
    let unsigned = encode_receipt(None);
    let decoded = decode(&unsigned).expect("unsigned receipt decodes");
    assert_eq!(
        encode_unsigned(&decoded).expect("canonical unsigned receipt"),
        unsigned
    );
    let signature = signer
        .sign(&receipt_digest(&unsigned).expect("receipt digest"))
        .to_bytes();
    let canonical = encode_receipt(Some(signature));
    let decoded = decode(&canonical).expect("signed receipt decodes");
    assert_eq!(
        encode(&decoded).expect("canonical signed receipt"),
        canonical
    );
    canonical
}

fn entry(module: u16, signer: &SigningKey) -> (Did, RawActivityReceiptEvidence) {
    let actor = Did::new(b"did:layerx:receipt-key-owner").expect("actor DID");
    let activity_module = if module > 11 {
        ModuleId::Asset
    } else {
        ModuleId::from_u16(module).expect("kernel module")
    };
    let (activity, id) = signed_activity(activity_module, &actor);
    let receipt = signed_receipt(module, id, 1, signer);
    let tail = signed_receipt(1, [0xfe; 32], 2, &batch_key());
    let (proof, receipt_root) = build_proof(&[&receipt, &tail], 0).expect("two-leaf receipt tree");
    let (_, activity_root) = build_proof(&[&activity], 0).expect("activity tree");
    let key = batch_key();
    let mut header = Encoder::new(354);
    header
        .structure_header_version(0x1701, PROTOCOL_VERSION)
        .expect("batch header");
    header.u8(15).expect("batch fields");
    header.tag(1, 15).expect("version tag");
    header.u16(PROTOCOL_VERSION).expect("batch version");
    header.tag(2, 15).expect("network tag");
    header.u32(NETWORK).expect("batch network");
    for (tag, value) in [(3, 1), (4, 7), (5, 1), (6, 2)] {
        header.tag(tag, 15).expect("integer tag");
        header.u64(value).expect("batch integer");
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
        header.tag(tag, 15).expect("commitment tag");
        header.bytes(&value, 32).expect("batch commitment");
    }
    header.tag(14, 15).expect("timestamp tag");
    header.u64(1_000).expect("batch timestamp");
    header.tag(15, 15).expect("sequencer tag");
    header
        .bytes(&key.verifying_key().to_bytes(), 32)
        .expect("batch sequencer");
    let header = header.finish();
    let decoded = decode_batch_header(&header).expect("batch header decodes");
    assert_eq!(
        encode_batch_header(&decoded).expect("canonical batch header"),
        header
    );
    let signature = key
        .sign(&batch_header_digest(&header).expect("batch digest"))
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
    let authority = authority()
        .with_receipt_keys(vec![key])
        .expect("governed receipt key");
    let (actor, entry) = entry(12, &receipt_key());
    let decoded = decode(entry.receipt().canonical_receipt()).expect("node receipt");
    let protocol = decoded.protocol().expect("protocol receipt");
    assert_eq!(protocol.module_id(), 12);
    assert_eq!(protocol.operation(), 2);
    assert_eq!(protocol.context_hash(), KEY_ID);
    let facts = authority
        .authenticate_cumulative_use(&actor, WINDOW, &[entry])
        .expect("governed node receipt authenticates the cumulative-use window");
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
    let authority = authority()
        .with_receipt_keys(vec![governed_key()])
        .expect("governed receipt key");
    let (actor, entry) = entry(12, &receipt_key());
    let other_actor = Did::new(b"did:layerx:different-activity").expect("different actor DID");
    let (other_activity, other_id) = signed_activity(ModuleId::Asset, &other_actor);
    let receipt = entry.receipt().clone();
    assert_ne!(
        other_id,
        decode(receipt.canonical_receipt())
            .expect("node receipt")
            .protocol()
            .expect("protocol receipt")
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
            authority()
                .with_receipt_keys(vec![governed_key()])
                .expect("independent governed receipt key"),
        ] {
            let facts = authority
                .authenticate_cumulative_use(&actor, WINDOW, std::slice::from_ref(&entry))
                .expect("historical kernel receipt remains authenticated by its batch key");
            assert_eq!(facts.count(), 1);
            assert_eq!(facts.amount(), AMOUNT);
        }
    }
}
