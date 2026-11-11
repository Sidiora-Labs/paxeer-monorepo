use super::agentd;
use agentd::human::{HumanOperationError, HumanPeer, HumanSubject};
use agentd::human_runtime::{bind_retained_registration, bind_subject_transaction};
use agentd::outbox::{Outbox, SubmissionState, SubmissionStatus};
use agentd::protocol_evidence::{EvidenceAuthority, RawReceiptEvidence};
use agentd::store::{Store, TenantId};
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::account_binding::{plan_bind, sign_bind_transaction, BindFees, BindPlan};
use layerx_client::sequencer::SequencerBlock;
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_proof::merkle::build_proof;
use layerx_types::ids::Did;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::{batch_header_digest, program_execution_batch_id, receipt_digest};
use layerx_wire::limits::PROTOCOL_VERSION;
use std::fmt::Debug;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

const NETWORK: u32 = 125;
static NEXT: AtomicU64 = AtomicU64::new(0);

#[test]
fn subject_registration_test_gap_resolves_reads_and_writes_across_commitment_and_restart() {
    prove_registration_resolution(|_, alice, bob, _| (alice.clone(), bob.clone()));
}

fn must<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("managed subject registration: {error:?}"))
}

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        Self(std::env::temp_dir().join(format!(
            "repair-22-33-1514-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        )))
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        must(std::fs::remove_dir_all(&self.0));
    }
}

fn peer(principal: &str) -> HumanPeer {
    HumanPeer {
        uid: 7,
        principal: principal.to_owned(),
        tenant: must(layerx_identity_binding::subject_namespace(
            "human-service",
            principal,
        )),
        subject: Some(HumanSubject {
            transport_tenant: "human-service".into(),
            transport_principal: "human-transport".into(),
            owner: format!("did:layerx:{principal}"),
            account: format!("agent:did:layerx:{principal}:main"),
            asset: [0x24; 32],
            registration: None,
        }),
    }
}

fn tenant(peer: &HumanPeer) -> TenantId {
    must(TenantId::new(peer.tenant.clone()))
}

fn assert_resolution(
    store: &Store,
    alice: &HumanPeer,
    bob: &HumanPeer,
    did: &Did,
    owner_address: [u8; 20],
    raw: &[u8],
    activity: &[u8],
) {
    let bound = must(bind_retained_registration(
        store,
        alice,
        did,
        u64::from(NETWORK),
        owner_address,
    ));
    let mut expected = alice.clone();
    expected
        .subject
        .as_mut()
        .unwrap_or_else(|| panic!("subject"))
        .registration = Some(raw.to_vec());
    assert_eq!(
        bound, expected,
        "read resolution retains exact producer bytes and scope"
    );
    assert_eq!(
        must(bind_subject_transaction(
            &bound,
            activity,
            u64::from(NETWORK),
            owner_address
        )),
        bound,
        "write resolution preserves the read's registration and owner scope"
    );
    assert!(matches!(
        bind_retained_registration(store, bob, did, u64::from(NETWORK), owner_address),
        Err(HumanOperationError::Refused)
    ));
    let mut other_owner = alice.clone();
    other_owner
        .subject
        .as_mut()
        .unwrap_or_else(|| panic!("subject"))
        .owner = bob
        .subject
        .as_ref()
        .unwrap_or_else(|| panic!("subject"))
        .owner
        .clone();
    // Even access to the same tenant's retained bytes cannot authorize another owner address.
    let other = must(derive_from_seed(&[0x77; 64], 0));
    assert!(matches!(
        bind_retained_registration(
            store,
            &other_owner,
            did,
            u64::from(NETWORK),
            other.evm_address()
        ),
        Err(HumanOperationError::Refused)
    ));
    assert!(matches!(
        bind_subject_transaction(&bound, activity, u64::from(NETWORK), other.evm_address()),
        Err(HumanOperationError::Refused)
    ));
    let (call, _) = must(decode_signed(activity));
    let other_activity = must(call.sign(other.evm_secret().unwrap_or_else(|| panic!("EVM key"))));
    assert!(matches!(
        bind_subject_transaction(&bound, &other_activity, u64::from(NETWORK), owner_address),
        Err(HumanOperationError::Refused)
    ));
}

pub(super) fn prove_registration_resolution(
    mut bindings: impl FnMut(&mut Store, &HumanPeer, &HumanPeer, bool) -> (HumanPeer, HumanPeer),
) {
    let directory = Directory::new();
    let account = must(derive_from_seed(&[0x33; 64], 0));
    let BindPlan::Bind(call) = must(plan_bind(&account, u64::from(NETWORK), None, 0)) else {
        panic!("new account must produce a registration");
    };
    must(call.signed_binding().verify());
    let did = must(Did::new(call.signed_binding().did().as_bytes()));
    let raw = must(sign_bind_transaction(
        &account,
        u64::from(NETWORK),
        &call,
        BindFees {
            evm_nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
        },
    ));
    let (decoded, signature) = must(decode_signed(&raw));
    assert_eq!(decoded.to, call.to());
    assert_eq!(decoded.data, call.data());
    assert_eq!(
        must(recover_sender(&decoded, &signature)),
        account.evm_address()
    );
    let activity = must(
        Eip1559Call {
            chain_id: u64::from(NETWORK),
            nonce: 8,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
            to: [0x22; 20],
            data: vec![1, 2, 3],
        }
        .sign(account.evm_secret().unwrap_or_else(|| panic!("EVM key"))),
    );
    let alice = peer("alice");
    let bob = peer("bob");
    let mut store = must(Store::open(&directory.0));
    let (alice, bob) = bindings(&mut store, &alice, &bob, false);
    let mut outbox = Outbox::default();
    let hash = must(outbox.enqueue(&mut store, tenant(&alice), raw.clone()));
    assert_eq!(hash, transaction_hash(&raw));
    assert_eq!(must(outbox.bytes_for_transmission(hash)), raw);
    assert_pending_refuses(
        &mut store,
        &mut outbox,
        hash,
        &alice,
        &did,
        account.evm_address(),
    );
    let authority = must(EvidenceAuthority::pinned_to_handshake(
        PROTOCOL_VERSION,
        NETWORK,
        SigningKey::from_bytes(&[0x3a; 32])
            .verifying_key()
            .to_bytes(),
    ));
    let receipt = must(authority.verify_receipt(&raw_receipt(hash, 0, 1)));
    let executed = must(outbox.record_execution(&mut store, hash, receipt, 8, 0));
    assert_eq!(executed.state, SubmissionState::Executed);
    assert_resolution(
        &store,
        &alice,
        &bob,
        &did,
        account.evm_address(),
        &raw,
        &activity,
    );
    drop(outbox);
    drop(store);

    assert_restart_and_reconcile(
        &directory,
        &mut bindings,
        &RegistrationResolution {
            alice: &alice,
            bob: &bob,
            did: &did,
            owner_address: account.evm_address(),
            raw: &raw,
            activity: &activity,
            hash,
            executed: &executed,
        },
    );
}

fn assert_pending_refuses(
    store: &mut Store,
    outbox: &mut Outbox,
    hash: [u8; 32],
    alice: &HumanPeer,
    did: &Did,
    owner_address: [u8; 20],
) {
    for state in [SubmissionState::Queued, SubmissionState::Submitted] {
        if state == SubmissionState::Submitted {
            must(outbox.transition(store, hash, state, "exact registration transmitted", None));
        }
        assert_eq!(
            outbox
                .status(hash)
                .unwrap_or_else(|| panic!("outbox status"))
                .state,
            state
        );
        assert!(matches!(
            bind_retained_registration(store, alice, did, u64::from(NETWORK), owner_address),
            Err(HumanOperationError::Refused)
        ));
    }
}

#[derive(Clone, Copy)]
struct RegistrationResolution<'a> {
    alice: &'a HumanPeer,
    bob: &'a HumanPeer,
    did: &'a Did,
    owner_address: [u8; 20],
    raw: &'a [u8],
    activity: &'a [u8],
    hash: [u8; 32],
    executed: &'a SubmissionStatus,
}

fn assert_restart_and_reconcile(
    directory: &Directory,
    bindings: &mut impl FnMut(&mut Store, &HumanPeer, &HumanPeer, bool) -> (HumanPeer, HumanPeer),
    fixture: &RegistrationResolution<'_>,
) {
    let RegistrationResolution {
        alice,
        bob,
        did,
        owner_address,
        raw,
        activity,
        hash,
        executed,
    } = *fixture;
    let mut store = must(Store::open(&directory.0));
    let (alice, bob) = bindings(&mut store, alice, bob, true);
    let mut outbox = Outbox::default();
    must(outbox.restore(&store, tenant(&alice), hash));
    assert_eq!(outbox.status(hash), Some(executed));
    assert_eq!(must(outbox.exact_signed_bytes(hash)), raw);
    assert_resolution(&store, &alice, &bob, did, owner_address, raw, activity);
    let block = SequencerBlock {
        number: 8,
        hash: [0x55; 32],
        parent_hash: [0x44; 32],
        timestamp: 1_000,
        transactions_root: [0x33; 32],
        receipts_root: [0x66; 32],
        transactions: vec![raw.to_vec()],
    };
    let committed = must(outbox.reconcile_committed_block(&mut store, hash, &block));
    assert_eq!(committed.state, SubmissionState::Committed);
    assert_resolution(&store, &alice, &bob, did, owner_address, raw, activity);
    drop(outbox);
    drop(store);

    let mut store = must(Store::open(&directory.0));
    let (alice, bob) = bindings(&mut store, &alice, &bob, true);
    let mut outbox = Outbox::default();
    must(outbox.restore(&store, tenant(&alice), hash));
    assert_eq!(outbox.status(hash), Some(&committed));
    assert_eq!(must(outbox.exact_signed_bytes(hash)), raw);
    assert_resolution(&store, &alice, &bob, did, owner_address, raw, activity);
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
