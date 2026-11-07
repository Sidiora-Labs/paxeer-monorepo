use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::protocol_evidence::{
    AuthenticatedCoreTime, CumulativeUseError, CumulativeUseWindow, EvidenceAuthority,
    RawActivityReceiptEvidence, RawReceiptEvidence, RawTimeWindowBatch, TimeWindowUseError,
};
use layerx_agentd::read::{
    self, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator, CommitVote,
    ServedCheckpoint,
};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_crypto::evm_transaction::{self, Eip1559Call};
use layerx_crypto::SignatureMessage;
use layerx_proof::merkle::build_proof;
use layerx_types::activity::{Authority, EnvelopeBuilder, Signature, TimestampBound};
use layerx_types::amount::Amount;
use layerx_types::ids::{Did, IdempotencyKey};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload};
use layerx_wire::activity::{decode_signed, encode_signed_envelope, encode_unsigned_envelope};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::{
    activity_id, batch_header_digest, payload_hash_for, receipt_digest, Domain,
};
use layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION as PROTOCOL_VERSION;
use layerx_wire::receipt::{
    decode, decode_batch_header, encode, encode_batch_header, encode_unsigned, BatchHeader,
};

const NETWORK: u32 = 125;
const KEY_ID: [u8; 32] = [0x56; 32];
const AMOUNT: u128 = 25;
const WINDOW: CumulativeUseWindow = CumulativeUseWindow { first: 1, last: 2 };

fn batch_key() -> SigningKey {
    SigningKey::from_bytes(&[0x34; 32])
}

fn receipt_key() -> SigningKey {
    SigningKey::from_bytes(&[0x45; 32])
}

fn actor() -> Did {
    Did::new(b"did:layerx:evm-cumulative-owner").expect("actor DID")
}

fn signed_activity() -> (Vec<u8>, [u8; 20]) {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&0x101a_u16.to_be_bytes());
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 1,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to,
        data: vec![0x01, 0x02, 0x03, 0x04],
    };
    let canonical = call
        .sign(&[0x67; 32])
        .expect("signed zero-value EIP-1559 call");
    let (decoded, signature) = evm_transaction::decode_signed(&canonical).expect("canonical call");
    assert_eq!(decoded, call);
    let sender = evm_transaction::recover_sender(&decoded, &signature).expect("recovered sender");
    let expected = k256::ecdsa::SigningKey::from_bytes((&[0x67; 32]).into()).expect("fixed key");
    assert_eq!(
        sender,
        layerx_crypto::secp256k1::evm_address(
            expected.verifying_key().to_encoded_point(false).as_bytes(),
        )
        .expect("fixed key account")
    );
    (canonical, sender)
}

fn signed_envelope(module: ModuleId, actor: &Did) -> (Vec<u8>, [u8; 32]) {
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

fn fixture(name: &str) -> Vec<u8> {
    let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../tests/fixtures/custody/daemon-light-credit-receipt");
    std::fs::read(root.join(name)).expect("real maintained-batch fixture")
}

fn template_header() -> BatchHeader {
    decode_batch_header(&fixture("header")).expect("fixture header")
}

fn signed_receipt(
    module: u16,
    id: [u8; 32],
    activity_root: [u8; 32],
    result_code: i32,
    receipt_height: u64,
) -> Vec<u8> {
    let encode_receipt = |signature: Option<[u8; 64]>| {
        let mut encoder = Encoder::new(4_096);
        encoder
            .structure_header_version(0x5201, PROTOCOL_VERSION)
            .expect("receipt header");
        encoder.u16(PROTOCOL_VERSION).expect("receipt version");
        encoder.bytes(&id, 32).expect("activity id");
        encoder.u64(1).expect("global sequence");
        let template = template_header();
        for root in [
            template.previous_state_root(),
            template.resulting_state_root(),
            activity_root,
        ] {
            encoder.bytes(&root, 32).expect("receipt root");
        }
        encoder.i32(result_code).expect("successful result");
        encoder.sequence_length(0, 512).expect("empty effects");
        encoder.u128(1).expect("fee charged");
        let batch_id = layerx_wire::hash::program_execution_batch_id(
            template.previous_state_root(),
            activity_root,
            1,
            1,
            1,
        )
        .expect("maintained execution batch id");
        let batch_id = if module > 11 {
            let mut height = [0; 32];
            height[24..].copy_from_slice(&receipt_height.to_be_bytes());
            height
        } else {
            batch_id
        };
        encoder.bytes(&batch_id, 32).expect("execution batch id");
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
        encoder.u64(template.timestamp_ms()).expect("timestamp");
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
    let signer = if module > 11 {
        receipt_key()
    } else {
        batch_key()
    };
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

struct BatchEvidence {
    entries: Vec<RawActivityReceiptEvidence>,
    time: RawTimeWindowBatch,
}

fn batch(activity: Vec<u8>, id: [u8; 32], module: u16, result_code: i32) -> BatchEvidence {
    batch_at_height(activity, id, module, result_code, 1)
}

fn batch_at_height(
    activity: Vec<u8>,
    id: [u8; 32],
    module: u16,
    result_code: i32,
    receipt_height: u64,
) -> BatchEvidence {
    let (_, activity_root) = build_proof(&[&activity], 0).expect("activity tree");
    let receipt = signed_receipt(module, id, activity_root, result_code, receipt_height);
    let maintenance = fixture("maintenance.receipt");
    let (proof, receipt_root) = build_proof(&[&receipt, &maintenance], 0).expect("receipt tree");
    let (maintenance_proof, same_root) =
        build_proof(&[&receipt, &maintenance], 1).expect("maintenance proof");
    assert_eq!(same_root, receipt_root);
    let key = batch_key();
    let template = template_header();
    let mut header = Encoder::new(354);
    header
        .structure_header_version(0x1701, PROTOCOL_VERSION)
        .expect("batch header");
    header.u8(15).expect("batch fields");
    header.tag(1, 15).expect("version tag");
    header.u16(PROTOCOL_VERSION).expect("batch version");
    header.tag(2, 15).expect("network tag");
    header.u32(NETWORK).expect("batch network");
    for (tag, value) in [(3, 1), (4, 1), (5, 1), (6, 2)] {
        header.tag(tag, 15).expect("integer tag");
        header.u64(value).expect("batch integer");
    }
    for (tag, value) in [
        (7, template.previous_state_root()),
        (8, template.resulting_state_root()),
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
    header
        .u64(template.timestamp_ms())
        .expect("batch timestamp");
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
    let entries = vec![RawActivityReceiptEvidence::from_signed_inclusion(
        activity,
        RawReceiptEvidence::new(receipt, proof, header.clone(), signature),
    )];
    BatchEvidence {
        time: RawTimeWindowBatch::new(
            entries.clone(),
            RawReceiptEvidence::new(maintenance, maintenance_proof, header, signature),
        ),
        entries,
    }
}

fn committed_checkpoint(
    height: u64,
    milliseconds: u64,
    state: [u8; 32],
    chain: &str,
) -> ServedCheckpoint {
    let keys = (1..=4)
        .map(|n| SigningKey::from_bytes(&[n; 32]))
        .collect::<Vec<_>>();
    let validators = keys
        .iter()
        .map(|key| {
            let public_key = key.verifying_key().to_bytes();
            CommitValidator {
                address: CommitValidator::address_for_key(&public_key),
                public_key,
                voting_power: 1,
            }
        })
        .collect::<Vec<_>>();
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 1,
        chain_id: chain.into(),
        height,
        time: CommitTimestamp {
            seconds: (milliseconds / 1000) as i64,
            nanos: (milliseconds % 1000) as u32 * 1_000_000,
        },
        last_block_id: Some(BlockId {
            hash: [1; 32],
            part_set_total: 1,
            part_set_hash: [2; 32],
        }),
        last_commit_hash: Some([3; 32]),
        data_hash: Some([4; 32]),
        validators_hash: read::validator_set_hash(&validators),
        next_validators_hash: read::validator_set_hash(&validators),
        consensus_hash: [5; 32],
        app_hash: state,
        last_results_hash: Some([6; 32]),
        evidence_hash: Some([7; 32]),
        proposer_address: validators[0].address,
    };
    let mut commit = BlockCommit {
        height,
        round: 0,
        block_id: BlockId {
            hash: header.block_hash(),
            part_set_total: 1,
            part_set_hash: [8; 32],
        },
        votes: Vec::new(),
    };
    for (key, validator) in keys.iter().zip(&validators) {
        commit.votes.push(CommitVote {
            validator_address: validator.address,
            voting_power: 1,
            timestamp: header.time,
            signature: key
                .sign(&commit.vote_sign_bytes(chain, header.time))
                .to_bytes(),
        });
    }
    read::checkpoint(height, chain, &header, &commit, &validators).unwrap()
}

fn authority() -> (EvidenceAuthority, AuthenticatedCoreTime) {
    let header = template_header();
    let checkpoint = committed_checkpoint(
        2,
        header.timestamp_ms(),
        header.resulting_state_root(),
        &NETWORK.to_string(),
    );
    let authority = EvidenceAuthority::pinned_to_handshake(
        PROTOCOL_VERSION,
        NETWORK,
        batch_key().verifying_key().to_bytes(),
    )
    .expect("batch authority")
    .with_receipt_keys(vec![ReceiptKeyEntry {
        key_id: KEY_ID,
        public_key: receipt_key().verifying_key().to_bytes(),
        first_height: 1,
        last_height: u64::MAX,
    }])
    .expect("governed receipt key")
    .pin_committed_validators(
        checkpoint.header.chain_id.clone(),
        checkpoint.validators.clone(),
    )
    .expect("committed validators");
    let observed = authority
        .authenticate_committed_time(&checkpoint)
        .expect("committed observation");
    (authority, observed)
}

#[test]
fn recover_sender() {
    let (activity, sender) = signed_activity();
    let id = evm_transaction::transaction_hash(&activity);
    let batch = batch(activity, id, 12, 0);
    let (authority, observed) = authority();
    let protocol = decode(batch.entries[0].receipt().canonical_receipt()).expect("node receipt");
    let protocol = protocol.protocol().expect("protocol receipt");
    assert_eq!(protocol.module_id(), 12);
    assert_eq!(protocol.operation(), 2);
    assert_eq!(protocol.activity_id(), id);
    assert_eq!(protocol.context_hash(), KEY_ID);
    assert_ne!(receipt_key().verifying_key(), batch_key().verifying_key());
    let mut other = sender;
    other[0] ^= 1;
    for (account, expected_count, expected_amount) in [(sender, 1, AMOUNT), (other, 0, 0)] {
        let facts = authority
            .authenticate_account_cumulative_use(&actor(), account, WINDOW, &batch.entries)
            .expect("account-bound sequence use");
        assert_eq!(facts.count(), expected_count);
        assert_eq!(facts.amount(), expected_amount);
        let facts = authority
            .authenticate_account_cumulative_time_use(
                &actor(),
                account,
                1,
                &observed,
                std::slice::from_ref(&batch.time),
            )
            .expect("account-bound time use");
        assert_eq!(facts.count(), expected_count);
        assert_eq!(facts.successful_count(), expected_count);
        assert_eq!(facts.actor(), &actor());
        if expected_count == 1 {
            assert_eq!(facts.activity_ids(), &[id]);
            assert_eq!(facts.receipt_amounts().len(), 1);
            assert_eq!(facts.receipt_amounts()[0].amount(), AMOUNT);
            assert_eq!(facts.receipt_amounts()[0].asset(), [0x81; 32]);
            assert_eq!(facts.receipt_amounts()[0].source_account(), [0x82; 32]);
        } else {
            assert!(facts.activity_ids().is_empty());
            assert!(facts.receipt_amounts().is_empty());
        }
    }
}

#[test]
fn transaction_hash() {
    let (activity, sender) = signed_activity();
    let mut wrong_id = evm_transaction::transaction_hash(&activity);
    wrong_id[0] ^= 1;
    let batch = batch(activity, wrong_id, 12, 0);
    let (authority, observed) = authority();
    assert!(matches!(
        authority.authenticate_account_cumulative_use(&actor(), sender, WINDOW, &batch.entries,),
        Err(CumulativeUseError::ActivityReceiptMismatch)
    ));
    assert!(matches!(
        authority.authenticate_account_cumulative_time_use(
            &actor(),
            sender,
            1,
            &observed,
            &[batch.time],
        ),
        Err(TimeWindowUseError::Identity)
    ));
}

#[test]
fn did_only_consumers_require_an_account_binding() {
    let (activity, _) = signed_activity();
    let batch = batch(
        activity.clone(),
        evm_transaction::transaction_hash(&activity),
        12,
        0,
    );
    let (authority, observed) = authority();
    assert!(matches!(
        authority.authenticate_cumulative_use(&actor(), WINDOW, &batch.entries,),
        Err(CumulativeUseError::AccountBinding)
    ));
    assert!(matches!(
        authority.authenticate_cumulative_time_use(&actor(), 1, &observed, &[batch.time],),
        Err(TimeWindowUseError::AccountBinding)
    ));
}

#[test]
fn failed_receipts_keep_each_consumers_counting_rules() {
    let (activity, sender) = signed_activity();
    let batch = batch(
        activity.clone(),
        evm_transaction::transaction_hash(&activity),
        12,
        1,
    );
    let (authority, observed) = authority();
    let facts = authority
        .authenticate_account_cumulative_use(&actor(), sender, WINDOW, &batch.entries)
        .expect("authenticated failed outcome");
    assert_eq!(facts.count(), 0);
    assert_eq!(facts.amount(), 0);
    let facts = authority
        .authenticate_account_cumulative_time_use(&actor(), sender, 1, &observed, &[batch.time])
        .expect("authenticated attempted use");
    assert_eq!(facts.count(), 1);
    assert_eq!(facts.successful_count(), 0);
    assert!(facts.receipt_amounts().is_empty());
}

#[test]
fn malformed_evm_and_kernel_evm_activities_are_refused() {
    let (activity, sender) = signed_activity();
    let (authority, observed) = authority();
    let mut trailing = activity.clone();
    trailing.push(0);
    for (canonical, module) in [(vec![2], 12), (trailing, 12), (activity, 1)] {
        let id = evm_transaction::transaction_hash(&canonical);
        let batch = batch(canonical, id, module, 0);
        assert!(matches!(
            authority
                .authenticate_account_cumulative_use(&actor(), sender, WINDOW, &batch.entries,),
            Err(CumulativeUseError::Activity)
        ));
        assert!(matches!(
            authority.authenticate_account_cumulative_time_use(
                &actor(),
                sender,
                1,
                &observed,
                &[batch.time],
            ),
            Err(TimeWindowUseError::Activity)
        ));
    }
}

#[test]
fn account_consumers_retain_envelope_did_binding() {
    let (authority, observed) = authority();
    let (_, account) = signed_activity();
    for module in [1, 11, 12] {
        let activity_module = if module > 11 {
            ModuleId::Asset
        } else {
            ModuleId::from_u16(module).expect("kernel module")
        };
        let (canonical, id) = signed_envelope(activity_module, &actor());
        let batch = batch(canonical, id, module, 0);
        for (did, expected) in [
            (actor(), 1),
            (Did::new(b"did:layerx:other").expect("other DID"), 0),
        ] {
            let facts = authority
                .authenticate_account_cumulative_use(&did, account, WINDOW, &batch.entries)
                .expect("DID envelope in account-aware consumer");
            assert_eq!(facts.count(), expected);
            assert_eq!(facts.amount(), if expected == 1 { AMOUNT } else { 0 });
            let facts = authority
                .authenticate_account_cumulative_time_use(
                    &did,
                    account,
                    1,
                    &observed,
                    std::slice::from_ref(&batch.time),
                )
                .expect("DID envelope in time consumer");
            assert_eq!(facts.count(), expected);
            assert_eq!(facts.successful_count(), expected);
        }
        assert_eq!(
            authority
                .authenticate_cumulative_use(&actor(), WINDOW, &batch.entries)
                .expect("existing sequence consumer")
                .count(),
            1
        );
        assert_eq!(
            authority
                .authenticate_cumulative_time_use(&actor(), 1, &observed, &[batch.time],)
                .expect("existing time consumer")
                .count(),
            1
        );
    }
}

#[test]
fn time_window_refuses_a_node_receipt_from_another_height() {
    let (activity, sender) = signed_activity();
    let id = evm_transaction::transaction_hash(&activity);
    let batch = batch_at_height(activity, id, 12, 0, 2);
    let (authority, observed) = authority();
    assert!(matches!(
        authority.authenticate_account_cumulative_time_use(
            &actor(),
            sender,
            1,
            &observed,
            &[batch.time],
        ),
        Err(TimeWindowUseError::Identity)
    ));
}
