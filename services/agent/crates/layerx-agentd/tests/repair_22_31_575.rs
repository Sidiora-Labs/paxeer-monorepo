fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

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
use layerx_wire::encode::Encoder;
use layerx_wire::hash::{batch_header_digest, payload_hash_for, receipt_digest, Domain};
use layerx_wire::historical::activity::{
    decode_signed, encode_signed_envelope, encode_unsigned_envelope,
};
use layerx_wire::historical::hash::activity_id;
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
    require_ok(Did::new(b"did:layerx:evm-cumulative-owner"), "actor DID")
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
    let canonical = require_ok(call.sign(&[0x67; 32]), "signed zero-value EIP-1559 call");
    let (decoded, signature) =
        require_ok(evm_transaction::decode_signed(&canonical), "canonical call");
    assert_eq!(decoded, call);
    let sender = require_ok(
        evm_transaction::recover_sender(&decoded, &signature),
        "recovered sender",
    );
    let expected = require_ok(
        k256::ecdsa::SigningKey::from_bytes((&[0x67; 32]).into()),
        "fixed key",
    );
    assert_eq!(
        sender,
        require_ok(
            layerx_crypto::secp256k1::evm_address(
                expected.verifying_key().to_encoded_point(false).as_bytes(),
            ),
            "fixed key account"
        )
    );
    (canonical, sender)
}

fn signed_envelope(module: ModuleId, actor: &Did) -> (Vec<u8>, [u8; 32]) {
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

fn fixture(name: &str) -> Vec<u8> {
    let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../tests/fixtures/custody/daemon-light-credit-receipt");
    require_ok(
        std::fs::read(root.join(name)),
        "real maintained-batch fixture",
    )
}

fn template_header() -> BatchHeader {
    require_ok(decode_batch_header(&fixture("header")), "fixture header")
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
        require_ok(
            encoder.structure_header_version(0x5201, PROTOCOL_VERSION),
            "receipt header",
        );
        require_ok(encoder.u16(PROTOCOL_VERSION), "receipt version");
        require_ok(encoder.bytes(&id, 32), "activity id");
        require_ok(encoder.u64(1), "global sequence");
        let template = template_header();
        for root in [
            template.previous_state_root(),
            template.resulting_state_root(),
            activity_root,
        ] {
            require_ok(encoder.bytes(&root, 32), "receipt root");
        }
        require_ok(encoder.i32(result_code), "successful result");
        require_ok(encoder.sequence_length(0, 512), "empty effects");
        require_ok(encoder.u128(1), "fee charged");
        let batch_id = require_ok(
            layerx_wire::hash::program_execution_batch_id(
                template.previous_state_root(),
                activity_root,
                1,
                1,
                1,
            ),
            "maintained execution batch id",
        );
        let batch_id = if module > 11 {
            let mut height = [0; 32];
            height[24..].copy_from_slice(&receipt_height.to_be_bytes());
            height
        } else {
            batch_id
        };
        require_ok(encoder.bytes(&batch_id, 32), "execution batch id");
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
        require_ok(encoder.u64(template.timestamp_ms()), "timestamp");
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
    let signer = if module > 11 {
        receipt_key()
    } else {
        batch_key()
    };
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
    let (_, activity_root) = require_ok(build_proof(&[&activity], 0), "activity tree");
    let receipt = signed_receipt(module, id, activity_root, result_code, receipt_height);
    let maintenance = fixture("maintenance.receipt");
    let (proof, receipt_root) =
        require_ok(build_proof(&[&receipt, &maintenance], 0), "receipt tree");
    let (maintenance_proof, same_root) = require_ok(
        build_proof(&[&receipt, &maintenance], 1),
        "maintenance proof",
    );
    assert_eq!(same_root, receipt_root);
    let key = batch_key();
    let template = template_header();
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
    for (tag, value) in [(3, 1), (4, 1), (5, 1), (6, 2)] {
        require_ok(header.tag(tag, 15), "integer tag");
        require_ok(header.u64(value), "batch integer");
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
        require_ok(header.tag(tag, 15), "commitment tag");
        require_ok(header.bytes(&value, 32), "batch commitment");
    }
    require_ok(header.tag(14, 15), "timestamp tag");
    require_ok(header.u64(template.timestamp_ms()), "batch timestamp");
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
            seconds: require_ok(i64::try_from(milliseconds / 1000), "test value fits i64"),
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
    require_ok(
        read::checkpoint(height, chain, &header, &commit, &validators),
        "required test value",
    )
}

fn authority() -> (EvidenceAuthority, AuthenticatedCoreTime) {
    let header = template_header();
    let checkpoint = committed_checkpoint(
        2,
        header.timestamp_ms(),
        header.resulting_state_root(),
        &NETWORK.to_string(),
    );
    let authority = require_ok(
        EvidenceAuthority::pinned_to_handshake(
            PROTOCOL_VERSION,
            NETWORK,
            batch_key().verifying_key().to_bytes(),
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "batch authority"))
        .with_receipt_keys(vec![ReceiptKeyEntry {
            key_id: KEY_ID,
            public_key: receipt_key().verifying_key().to_bytes(),
            first_height: 1,
            last_height: u64::MAX,
        }])
        .unwrap_or_else(|error| panic!("{}: {error:?}", "governed receipt key"))
        .pin_committed_validators(
            checkpoint.header.chain_id.clone(),
            checkpoint.validators.clone(),
        ),
        "committed validators",
    );
    let observed = require_ok(
        authority.authenticate_committed_time(&checkpoint),
        "committed observation",
    );
    (authority, observed)
}

#[test]
fn recover_sender() {
    let (activity, sender) = signed_activity();
    let id = evm_transaction::transaction_hash(&activity);
    let batch = batch(activity, id, 12, 0);
    let (authority, observed) = authority();
    let protocol = require_ok(
        decode(batch.entries[0].receipt().canonical_receipt()),
        "node receipt",
    );
    let protocol = require_some(protocol.protocol(), "protocol receipt");
    assert_eq!(protocol.module_id(), 12);
    assert_eq!(protocol.operation(), 2);
    assert_eq!(protocol.activity_id(), id);
    assert_eq!(protocol.context_hash(), KEY_ID);
    assert_ne!(receipt_key().verifying_key(), batch_key().verifying_key());
    let mut other = sender;
    other[0] ^= 1;
    for (account, expected_count, expected_amount) in [(sender, 1, AMOUNT), (other, 0, 0)] {
        let facts = require_ok(
            authority.authenticate_account_cumulative_use(
                &actor(),
                account,
                WINDOW,
                &batch.entries,
            ),
            "account-bound sequence use",
        );
        assert_eq!(facts.count(), expected_count);
        assert_eq!(facts.amount(), expected_amount);
        let facts = require_ok(
            authority.authenticate_account_cumulative_time_use(
                &actor(),
                account,
                1,
                &observed,
                std::slice::from_ref(&batch.time),
            ),
            "account-bound time use",
        );
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
    let facts = require_ok(
        authority.authenticate_account_cumulative_use(&actor(), sender, WINDOW, &batch.entries),
        "authenticated failed outcome",
    );
    assert_eq!(facts.count(), 0);
    assert_eq!(facts.amount(), 0);
    let facts = require_ok(
        authority.authenticate_account_cumulative_time_use(
            &actor(),
            sender,
            1,
            &observed,
            &[batch.time],
        ),
        "authenticated attempted use",
    );
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
            require_ok(ModuleId::from_u16(module), "kernel module")
        };
        let (canonical, id) = signed_envelope(activity_module, &actor());
        let batch = batch(canonical, id, module, 0);
        for (did, expected) in [
            (actor(), 1),
            (require_ok(Did::new(b"did:layerx:other"), "other DID"), 0),
        ] {
            let facts = require_ok(
                authority.authenticate_account_cumulative_use(
                    &did,
                    account,
                    WINDOW,
                    &batch.entries,
                ),
                "DID envelope in account-aware consumer",
            );
            assert_eq!(facts.count(), expected);
            assert_eq!(facts.amount(), if expected == 1 { AMOUNT } else { 0 });
            let facts = require_ok(
                authority.authenticate_account_cumulative_time_use(
                    &did,
                    account,
                    1,
                    &observed,
                    std::slice::from_ref(&batch.time),
                ),
                "DID envelope in time consumer",
            );
            assert_eq!(facts.count(), expected);
            assert_eq!(facts.successful_count(), expected);
        }
        assert_eq!(
            require_ok(
                authority.authenticate_cumulative_use(&actor(), WINDOW, &batch.entries),
                "existing sequence consumer"
            )
            .count(),
            1
        );
        assert_eq!(
            require_ok(
                authority.authenticate_cumulative_time_use(&actor(), 1, &observed, &[batch.time],),
                "existing time consumer"
            )
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
