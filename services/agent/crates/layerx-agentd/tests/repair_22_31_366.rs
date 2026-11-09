//! Repair 22.31.366: real EVM constructors -> signed bytes -> durable Outbox
//! -> subject authorization, including commitment and store restart.

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::human::{HumanPeer, HumanSubject};
use layerx_agentd::human_runtime::{bind_retained_registration, bind_subject_transaction};
use layerx_agentd::outbox::{Outbox, SubmissionState};
use layerx_agentd::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
use layerx_agentd::protocol_evidence::{EvidenceAuthority, RawReceiptEvidence};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::account_binding::{plan_bind, sign_bind_transaction, BindFees, BindPlan};
use layerx_client::sequencer::SequencerBlock;
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::disclosure::call::ledger_selector;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_crypto::onboarding::{OnboardingConsent, SponsoredRegistration};
use layerx_crypto::SignatureMessage;
use layerx_proof::merkle::build_proof;
use layerx_types::activity::{Authority, EnvelopeBuilder, Signature, TimestampBound};
use layerx_types::amount::Amount;
use layerx_types::ids::{Did, IdempotencyKey};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::{
    batch_header_digest, payload_hash_for, program_execution_batch_id, receipt_digest, Domain,
};
use layerx_wire::historical::activity::{encode_signed_envelope, encode_unsigned_envelope};
use layerx_wire::limits::PROTOCOL_VERSION;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

const NETWORK: u32 = 42;
const ACTION: [u8; 32] = [0x71; 32];
const ASSET: [u8; 32] = [0x24; 32];
const OWNER_SECRET: [u8; 32] = [0x17; 32];
static NEXT: AtomicU64 = AtomicU64::new(0);

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        Self(std::env::temp_dir().join(format!(
            "repair-22-31-366-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        )))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.0).expect("remove durable test store");
    }
}

fn peer() -> HumanPeer {
    HumanPeer {
        uid: 7,
        principal: "owner-person".into(),
        tenant: layerx_identity_binding::subject_namespace("human-service", "owner-person")
            .unwrap(),
        subject: Some(HumanSubject {
            transport_tenant: "human-service".into(),
            transport_principal: "human-transport".into(),
            owner: "did:layerx:sponsor".into(),
            account: "agent:did:layerx:sponsor:main".into(),
            asset: ASSET,
            registration: None,
        }),
    }
}
fn tenant(peer: &HumanPeer) -> TenantId {
    TenantId::new(peer.tenant.clone()).unwrap()
}
fn address(raw: &[u8]) -> [u8; 20] {
    let (call, signature) = decode_signed(raw).unwrap();
    recover_sender(&call, &signature).unwrap()
}
fn target() -> Did {
    Did::new(
        format!(
            "did:layerx:{}",
            hex::encode(
                SigningKey::from_bytes(&[0x23; 32])
                    .verifying_key()
                    .to_bytes()
            )
        )
        .as_bytes(),
    )
    .unwrap()
}
fn consent() -> Vec<u8> {
    let signer = SigningKey::from_bytes(&[0x23; 32]);
    let consent = OnboardingConsent {
        sponsor: layerx_wire::hash::did_id_for_protocol(
            &Did::new(peer().subject.unwrap().owner.as_bytes()).unwrap(),
            3,
        )
        .unwrap(),
        target: target(),
        target_public_key: signer.verifying_key().to_bytes(),
        native_asset: ASSET,
        action_key: ACTION,
        expires_at: 20_000,
    };
    let kind = ActivityType::new(ModuleId::Governance, 1).unwrap();
    let registry =
        ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Governance, &[kind]).unwrap()])
            .unwrap();
    let payload = Payload::new(&registry, kind, &consent.payload().unwrap()).unwrap();
    let mut builder = EnvelopeBuilder::new();
    builder.protocol_version(3).unwrap();
    builder.network_id(NETWORK).unwrap();
    builder.activity_type(kind).unwrap();
    builder.actor_did(target()).unwrap();
    builder
        .authority(Authority::owner(&signer.verifying_key().to_bytes()).unwrap())
        .unwrap();
    builder.account_sequence(0).unwrap();
    builder
        .timestamp_bound(TimestampBound::new(1_000, consent.expires_at).unwrap())
        .unwrap();
    builder
        .idempotency_key(IdempotencyKey::new(ACTION))
        .unwrap();
    builder.fee_limit(Amount::from_u128(0)).unwrap();
    builder
        .payload_hash(payload_hash_for(&payload).unwrap())
        .unwrap();
    builder.payload(payload).unwrap();
    let envelope = builder.build().unwrap();
    let unsigned = encode_unsigned_envelope(&envelope).unwrap();
    let digest = SignatureMessage::new(Domain::SignaturePreimage, 3, NETWORK, &unsigned)
        .unwrap()
        .digest();
    encode_signed_envelope(
        &envelope.attach_signature(Signature::new(&signer.sign(&digest).to_bytes()).unwrap()),
    )
    .unwrap()
}
fn call(payload: Vec<u8>, action: [u8; 32], nonce: u64) -> Eip1559Call {
    Eip1559Call {
        chain_id: u64::from(NETWORK),
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 100_000,
        to: PrecompileTarget::Auth.address(),
        data: PrecompileCall {
            target: PrecompileTarget::Auth,
            selector: ledger_selector("recordLifecycle(bytes,bytes32)"),
            arguments: vec![AbiArgument::Bytes(payload), AbiArgument::Word(action)],
        }
        .encode()
        .unwrap(),
    }
}
fn registration(nonce: u64) -> Vec<u8> {
    let consent = consent();
    let registration = SponsoredRegistration::from_signed_consent(&consent).unwrap();
    assert_eq!(registration.signed_consent(), consent);
    call(registration.payload().unwrap(), ACTION, nonce)
        .sign(&OWNER_SECRET)
        .unwrap()
}
fn execute(store: &mut Store, outbox: &mut Outbox, hash: [u8; 32], success: bool) {
    outbox
        .transition(
            store,
            hash,
            SubmissionState::Submitted,
            "real exact-byte transmission",
            None,
        )
        .unwrap();
    let authority = EvidenceAuthority::pinned_to_handshake(
        PROTOCOL_VERSION,
        NETWORK,
        SigningKey::from_bytes(&[0x3a; 32])
            .verifying_key()
            .to_bytes(),
    )
    .unwrap();
    let receipt = authority
        .verify_receipt(&raw_receipt(hash, i32::from(!success), 1))
        .unwrap();
    if success {
        outbox.record_execution(store, hash, receipt, 8, 0).unwrap();
    } else {
        outbox.record_failure(store, hash, receipt, 8, 0).unwrap();
    }
}
fn commit(store: &mut Store, outbox: &mut Outbox, hash: [u8; 32], raw: Vec<u8>) {
    let block = SequencerBlock {
        number: 8,
        hash: [0x55; 32],
        parent_hash: [0x44; 32],
        timestamp: 1_000,
        transactions_root: [0x33; 32],
        receipts_root: [0x66; 32],
        transactions: vec![raw],
    };
    assert_eq!(
        outbox
            .reconcile_committed_block(store, hash, &block)
            .unwrap()
            .state,
        SubmissionState::Committed
    );
}
fn bind(store: &Store, peer: &HumanPeer, raw: &[u8]) -> HumanPeer {
    bind_retained_registration(store, peer, &target(), u64::from(NETWORK), address(raw)).unwrap()
}

#[test]
fn subject_retained_evm_decode_survives_commitment_and_restart() {
    let directory = Directory::new();
    let peer = peer();
    let raw = registration(7);
    assert_eq!(raw[0], 2);
    let mut store = Store::open(&directory.0).unwrap();
    let mut outbox = Outbox::default();
    let hash = outbox
        .enqueue(&mut store, tenant(&peer), raw.clone())
        .unwrap();
    assert_eq!(hash, transaction_hash(&raw));
    assert_ne!(hash, ACTION, "the action key is not the transaction hash");
    assert!(bind_retained_registration(
        &store,
        &peer,
        &target(),
        u64::from(NETWORK),
        address(&raw)
    )
    .is_err());
    execute(&mut store, &mut outbox, hash, true);
    let bound = bind(&store, &peer, &raw);
    assert_eq!(
        bound.subject.as_ref().unwrap().registration.as_deref(),
        Some(raw.as_slice())
    );
    assert_eq!(bound.tenant, peer.tenant);
    assert_eq!(bound.principal, peer.principal);
    commit(&mut store, &mut outbox, hash, raw.clone());
    assert_eq!(bind(&store, &peer, &raw), bound);
    drop(outbox);
    drop(store);
    let store = Store::open(&directory.0).unwrap();
    assert_eq!(bind(&store, &peer, &raw), bound);
    let mut recovered = Outbox::default();
    recovered.restore(&store, tenant(&peer), hash).unwrap();
    assert_eq!(
        recovered.status(hash).unwrap().state,
        SubmissionState::Committed
    );
    assert_eq!(recovered.exact_signed_bytes(hash).unwrap(), raw);
}

#[test]
fn subject_retained_evm_decode_binds_current_transaction_actor() {
    let directory = Directory::new();
    let peer = peer();
    let raw = registration(7);
    let mut store = Store::open(&directory.0).unwrap();
    let mut outbox = Outbox::default();
    let hash = outbox
        .enqueue(&mut store, tenant(&peer), raw.clone())
        .unwrap();
    execute(&mut store, &mut outbox, hash, true);
    let bound = bind(&store, &peer, &raw);
    let transaction = Eip1559Call {
        chain_id: u64::from(NETWORK),
        nonce: 8,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 100_000,
        to: [0x22; 20],
        data: vec![1, 2, 3],
    }
    .sign(&[0x23; 32])
    .unwrap();
    let actor = address(&transaction);
    assert_eq!(
        bind_subject_transaction(&bound, &transaction, u64::from(NETWORK), actor).unwrap(),
        bound
    );
    assert!(bind_subject_transaction(&bound, &transaction, u64::from(NETWORK) + 1, actor).is_err());
    assert!(
        bind_subject_transaction(&bound, &transaction, u64::from(NETWORK), address(&raw)).is_err()
    );
    assert!(bind_subject_transaction(&bound, &consent(), u64::from(NETWORK), actor).is_err());
    let mut damaged = transaction;
    damaged.pop();
    assert!(bind_subject_transaction(&bound, &damaged, u64::from(NETWORK), actor).is_err());
}

#[test]
fn subject_retained_evm_decode_refuses_owner_target_network_asset_and_tenant_mismatch() {
    let directory = Directory::new();
    let peer = peer();
    let raw = registration(7);
    let mut store = Store::open(&directory.0).unwrap();
    let mut outbox = Outbox::default();
    let hash = outbox
        .enqueue(&mut store, tenant(&peer), raw.clone())
        .unwrap();
    execute(&mut store, &mut outbox, hash, true);
    for (network, owner, did) in [
        (u64::from(NETWORK) + 1, address(&raw), target()),
        (u64::from(NETWORK), [0x99; 20], target()),
        (
            u64::from(NETWORK),
            address(&raw),
            Did::new(b"did:layerx:other-target").unwrap(),
        ),
    ] {
        assert!(bind_retained_registration(&store, &peer, &did, network, owner).is_err());
    }
    for dimension in 0..3 {
        let mut other = peer.clone();
        match dimension {
            0 => other.subject.as_mut().unwrap().owner = "did:layerx:other-owner".into(),
            1 => other.subject.as_mut().unwrap().asset = [0x99; 32],
            _ => other.tenant = "another-tenant".into(),
        }
        assert!(bind_retained_registration(
            &store,
            &other,
            &target(),
            u64::from(NETWORK),
            address(&raw)
        )
        .is_err());
    }
}

#[test]
fn subject_retained_evm_decode_refuses_changed_action_consent_and_contract() {
    let payload = SponsoredRegistration::from_signed_consent(&consent())
        .unwrap()
        .payload()
        .unwrap();
    for variant in 0..4 {
        let directory = Directory::new();
        let peer = peer();
        let mut transaction = call(payload.clone(), ACTION, 7);
        match variant {
            0 => transaction = call(payload.clone(), [0x88; 32], 7),
            1 => {
                let mut changed = payload.clone();
                *changed.last_mut().unwrap() ^= 1;
                transaction = call(changed, ACTION, 7);
            }
            2 => transaction.to = PrecompileTarget::Pay.address(),
            _ => transaction.data[0] ^= 1,
        }
        let raw = transaction.sign(&OWNER_SECRET).unwrap();
        let mut store = Store::open(&directory.0).unwrap();
        let mut outbox = Outbox::default();
        let hash = outbox
            .enqueue(&mut store, tenant(&peer), raw.clone())
            .unwrap();
        execute(&mut store, &mut outbox, hash, true);
        assert!(
            bind_retained_registration(&store, &peer, &target(), u64::from(NETWORK), address(&raw))
                .is_err(),
            "variant {variant}"
        );
    }
}

#[test]
fn subject_retained_evm_decode_refuses_failed_ambiguous_and_corrupt_records() {
    let directory = Directory::new();
    let peer = peer();
    let raw = registration(7);
    let mut store = Store::open(&directory.0).unwrap();
    let mut outbox = Outbox::default();
    let hash = outbox
        .enqueue(&mut store, tenant(&peer), raw.clone())
        .unwrap();
    execute(&mut store, &mut outbox, hash, false);
    assert!(bind_retained_registration(
        &store,
        &peer,
        &target(),
        u64::from(NETWORK),
        address(&raw)
    )
    .is_err());
    for nonce in [8, 9] {
        let raw = registration(nonce);
        let hash = outbox.enqueue(&mut store, tenant(&peer), raw).unwrap();
        execute(&mut store, &mut outbox, hash, true);
        if nonce == 8 {
            assert!(bind_retained_registration(
                &store,
                &peer,
                &target(),
                u64::from(NETWORK),
                address(&registration(8))
            )
            .is_ok());
        }
    }
    assert!(bind_retained_registration(
        &store,
        &peer,
        &target(),
        u64::from(NETWORK),
        address(&raw)
    )
    .is_err());
    let key = TenantKey::new(tenant(&peer), ObjectKind::PreparedActivity, hash.to_vec()).unwrap();
    store.put_local(key, registration(10)).unwrap();
    assert!(bind_retained_registration(
        &store,
        &peer,
        &target(),
        u64::from(NETWORK),
        address(&raw)
    )
    .is_err());
}

#[test]
fn subject_retained_evm_decode_native_bind_uses_the_real_account_constructor() {
    let directory = Directory::new();
    let account = derive_from_seed(&[0x33; 64], 0).unwrap();
    let BindPlan::Bind(call) = plan_bind(&account, u64::from(NETWORK), None, 0).unwrap() else {
        panic!("unbound pair");
    };
    call.signed_binding().verify().unwrap();
    let did = Did::new(call.signed_binding().did().as_bytes()).unwrap();
    let raw = sign_bind_transaction(
        &account,
        u64::from(NETWORK),
        &call,
        BindFees {
            evm_nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
        },
    )
    .unwrap();
    let peer = peer();
    let mut store = Store::open(&directory.0).unwrap();
    let mut outbox = Outbox::default();
    let hash = outbox
        .enqueue(&mut store, tenant(&peer), raw.clone())
        .unwrap();
    execute(&mut store, &mut outbox, hash, true);
    commit(&mut store, &mut outbox, hash, raw.clone());
    drop(outbox);
    drop(store);
    let store = Store::open(&directory.0).unwrap();
    let bound = bind_retained_registration(
        &store,
        &peer,
        &did,
        u64::from(NETWORK),
        account.evm_address(),
    )
    .unwrap();
    assert_eq!(bound.subject.unwrap().registration, Some(raw.clone()));
    assert!(
        bind_retained_registration(&store, &peer, &did, u64::from(NETWORK), [0x99; 20]).is_err()
    );
    let (mut transaction, _) = decode_signed(&raw).unwrap();
    transaction.data[67] = 0x20;
    let malformed = transaction.sign(account.evm_secret().unwrap()).unwrap();
    drop(store);
    let mut store = Store::open(&directory.0).unwrap();
    let mut outbox = Outbox::default();
    let hash = outbox
        .enqueue(&mut store, tenant(&peer), malformed)
        .unwrap();
    execute(&mut store, &mut outbox, hash, true);
    assert!(bind_retained_registration(
        &store,
        &peer,
        &did,
        u64::from(NETWORK),
        account.evm_address()
    )
    .is_err());
}

fn raw_receipt(activity_id: [u8; 32], result_code: i32, amount: u128) -> RawReceiptEvidence {
    raw_receipt_at(activity_id, result_code, amount, 9)
}

fn raw_receipt_at(
    activity_id: [u8; 32],
    result_code: i32,
    amount: u128,
    global_sequence: u64,
) -> RawReceiptEvidence {
    raw_receipt_for_execution_batch(activity_id, result_code, amount, global_sequence, 7)
}

#[allow(clippy::too_many_lines)]
fn raw_receipt_for_execution_batch(
    activity_id: [u8; 32],
    result_code: i32,
    amount: u128,
    global_sequence: u64,
    execution_batch_number: u64,
) -> RawReceiptEvidence {
    let key = SigningKey::from_bytes(&[0x3a; 32]);
    let previous_state_root = [0x21; 32];
    let resulting_state_root = [0x22; 32];
    let batch_id = program_execution_batch_id(
        previous_state_root,
        [0x25; 32],
        global_sequence,
        global_sequence,
        execution_batch_number,
    )
    .unwrap_or_else(|error| panic!("execution batch id: {error:?}"));
    let asset = [0x24; 32];
    let encode = |signature: Option<[u8; 64]>| {
        let mut encoder = Encoder::new(4096);
        assert_eq!(
            encoder.structure_header_version(0x5201, layerx_wire::limits::PROTOCOL_VERSION,),
            Ok(())
        );
        assert_eq!(encoder.u16(layerx_wire::limits::PROTOCOL_VERSION), Ok(()));
        assert_eq!(encoder.bytes(&activity_id, 32), Ok(()));
        assert_eq!(encoder.u64(global_sequence), Ok(()));
        assert_eq!(encoder.bytes(&previous_state_root, 32), Ok(()));
        assert_eq!(encoder.bytes(&resulting_state_root, 32), Ok(()));
        assert_eq!(encoder.bytes(&[0x25; 32], 32), Ok(()));
        assert_eq!(encoder.i32(result_code), Ok(()));
        assert_eq!(encoder.sequence_length(0, 512), Ok(()));
        assert_eq!(encoder.u128(1), Ok(()));
        assert_eq!(encoder.bytes(&batch_id, 32), Ok(()));
        assert_eq!(encoder.u16(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u8(1), Ok(()));
        assert_eq!(encoder.bytes(&asset, 32), Ok(()));
        assert_eq!(encoder.u128(amount), Ok(()));
        assert_eq!(encoder.bytes(&[0x26; 32], 32), Ok(()));
        assert_eq!(encoder.u128(1_000 + amount), Ok(()));
        assert_eq!(
            encoder.u128(if result_code == 0 {
                1_000
            } else {
                1_000 + amount
            }),
            Ok(())
        );
        assert_eq!(encoder.u64(1), Ok(()));
        assert_eq!(encoder.bytes(&[0x27; 32], 32), Ok(()));
        assert_eq!(encoder.u128(10), Ok(()));
        assert_eq!(
            encoder.u128(if result_code == 0 { 10 + amount } else { 10 }),
            Ok(())
        );
        for value in [[0x28; 32], [0x29; 32], [0x2a; 32]] {
            assert_eq!(encoder.bytes(&value, 32), Ok(()));
        }
        assert_eq!(encoder.u64(1_000), Ok(()));
        assert_eq!(encoder.u8(u8::from(signature.is_some())), Ok(()));
        if let Some(signature) = signature {
            assert_eq!(encoder.bytes(&signature, 64), Ok(()));
        }
        encoder.finish()
    };
    let unsigned = encode(None);
    let digest =
        receipt_digest(&unsigned).unwrap_or_else(|error| panic!("receipt digest: {error:?}"));
    let canonical_receipt = encode(Some(key.sign(&digest).to_bytes()));
    let leaves = [canonical_receipt.as_slice()];
    let (proof, receipt_root) =
        build_proof(&leaves, 0).unwrap_or_else(|error| panic!("receipt proof: {error:?}"));
    let sequencer_id = key.verifying_key().to_bytes();
    let mut encoder = Encoder::new(354);
    assert_eq!(
        encoder.structure_header_version(0x1701, layerx_wire::limits::PROTOCOL_VERSION),
        Ok(())
    );
    assert_eq!(encoder.u8(15), Ok(()));
    let fields: [(u8, Vec<u8>); 15] = [
        (
            1,
            layerx_wire::limits::PROTOCOL_VERSION.to_be_bytes().to_vec(),
        ),
        (2, NETWORK.to_be_bytes().to_vec()),
        (3, 2_u64.to_be_bytes().to_vec()),
        (4, 7_u64.to_be_bytes().to_vec()),
        (5, global_sequence.to_be_bytes().to_vec()),
        (6, global_sequence.to_be_bytes().to_vec()),
        (7, previous_state_root.to_vec()),
        (8, resulting_state_root.to_vec()),
        (9, [0x25; 32].to_vec()),
        (10, receipt_root.to_vec()),
        (11, [0x34; 32].to_vec()),
        (12, [0x35; 32].to_vec()),
        (13, [0x36; 32].to_vec()),
        (14, 1_000_u64.to_be_bytes().to_vec()),
        (15, sequencer_id.to_vec()),
    ];
    for (field, value) in fields {
        assert_eq!(encoder.tag(field, 15), Ok(()));
        match field {
            1 => assert_eq!(
                encoder.u16(u16::from_be_bytes([value[0], value[1]])),
                Ok(())
            ),
            2 => assert_eq!(
                encoder.u32(u32::from_be_bytes(
                    value
                        .as_slice()
                        .try_into()
                        .unwrap_or_else(|_| panic!("u32 field"))
                )),
                Ok(())
            ),
            3..=6 | 14 => assert_eq!(
                encoder.u64(u64::from_be_bytes(
                    value
                        .as_slice()
                        .try_into()
                        .unwrap_or_else(|_| panic!("u64 field"))
                )),
                Ok(())
            ),
            _ => assert_eq!(encoder.bytes(&value, 32), Ok(())),
        }
    }
    let header = encoder.finish();
    let header_digest =
        batch_header_digest(&header).unwrap_or_else(|error| panic!("header digest: {error:?}"));
    RawReceiptEvidence::new(
        canonical_receipt,
        proof,
        header,
        key.sign(&header_digest).to_bytes(),
    )
}
