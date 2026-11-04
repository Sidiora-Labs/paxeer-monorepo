use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::approval::native_effect::NativeEffectApprovalCarrier as Effect;
use layerx_agentd::approval::native_program::NativeProgramApprovalCarrier as Program;
use layerx_agentd::approval::{CallApprovalError, CallApprovalState};
use layerx_agentd::budget::{
    create_daemon_limit, daemon_limit_id, daemon_limits, load_daemon_limits, BudgetLimiter,
    CoreTimestampMs, DaemonLimitRecord, LimitId, ProgramBudgetCharge, ProgramChargeKind,
    ProgramReservationRequest,
};
use layerx_agentd::human::{HumanPeer, HumanSubject};
use layerx_agentd::ops::program::ProgramOperations;
use layerx_agentd::outbox::Outbox;
use layerx_agentd::prepare::{
    prepare_bound, verify_call_binding, AbiArgument, EvmPreparationDefaults, EvmPreparationState,
    EvmPrepareRequest, EvmPrepared, PrecompileCall, PrecompileTarget,
};
use layerx_agentd::sign::{verify_before_submit, SigningPreparation, VerifiedSubmission};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::disclosure::call::ledger_selector;
use layerx_crypto::evm_transaction::transaction_hash;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_types::intent::ProgramId;
use layerx_types::payload::ModuleRegistry;
use layerx_types::program_call::{NativeProgramCall, Resources};
use layerx_wire::encode::Encoder;
use sha2::{Digest, Sha256};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

static NEXT: AtomicU64 = AtomicU64::new(1);
const SENDER: [u8; 20] = [
    0x7e, 0x5f, 0x45, 0x52, 0x09, 0x1a, 0x69, 0x12, 0x5d, 0x5d, 0xfc, 0xb7, 0xb8, 0xc2, 0x65, 0x90,
    0x29, 0x39, 0x5b, 0xdf,
];
const FEE: u128 = 150_000;
const DEADLINE: u64 = 20_000;

struct Fixture {
    root: PathBuf,
    peer: HumanPeer,
    prepared: EvmPrepared,
    tenant: TenantId,
    limit: LimitId,
}
impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.root);
    }
}
impl Fixture {
    fn new(program: bool) -> (Self, Store, BudgetLimiter) {
        let root = std::env::temp_dir().join(format!(
            "repair-22-31-225-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        let principal = "native-owner".to_owned();
        let transport_tenant = "approval-recovery".to_owned();
        let namespace =
            layerx_identity_binding::subject_namespace(&transport_tenant, &principal).unwrap();
        let peer = HumanPeer {
            uid: 1000,
            principal,
            tenant: namespace.clone(),
            subject: Some(HumanSubject {
                transport_tenant,
                transport_principal: "unix-owner".to_owned(),
                owner: "did:layerx:owner".to_owned(),
                account: "agent:did:layerx:owner:main".to_owned(),
                asset: asset(),
                registration: None,
            }),
        };
        let call = if program {
            ProgramOperations::<SequencerEndpoint>::call_precompile(
                NativeProgramCall {
                    program_id: ProgramId::new([0x31; 32]),
                    guest_abi: 1,
                    entrypoint: b"layerx_call",
                    calldata: &[1, 2, 3],
                    capabilities: &[0, 0],
                    access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
                    response_capacity: 64,
                    resources: Resources([1_000_000, 16_777_216, 100, 200, 4, 1024, 4096]),
                },
                1000,
            )
            .unwrap()
        } else {
            PrecompileCall {
                target: PrecompileTarget::Ledger,
                selector: ledger_selector("openAccount(bytes32)"),
                arguments: vec![AbiArgument::Word(asset())],
            }
        };
        let prepared = prepare_bound(
            EvmPreparationState {
                chain_id: 125,
                pending_nonce: 7,
                base_fee_per_gas: 1,
                head_height: 5,
            },
            EvmPreparationDefaults {
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 3,
                maximum_payload_bytes: 4096,
            },
            EvmPrepareRequest {
                from: SENDER,
                call,
                expected_nonce: Some(7),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: 4096,
            },
            50_000,
        )
        .unwrap();
        assert_eq!(prepared.disclosure.fee_limit, FEE);
        verify_call_binding(&prepared).unwrap();
        let tenant = TenantId::new(namespace).unwrap();
        let mut store = Store::open(&root).unwrap();
        let budgets = BudgetLimiter::new(Vec::new()).unwrap();
        let limit = daemon_limit_id([0x21; 32]);
        create_daemon_limit(
            &mut store,
            &budgets,
            DaemonLimitRecord {
                tenant: tenant.clone(),
                budget_id: [0x21; 32],
                limit_id: limit,
                agent_digest: [0x22; 32],
                asset: asset(),
                ceiling: 1_000_000,
                consumed: 0,
                expiry_ms: 100_000,
                revoked: false,
                mutation_key: [0x23; 32],
                body_digest: [0x24; 32],
                revoke_key: [0; 32],
            },
            CoreTimestampMs(1000),
        )
        .unwrap();
        (
            Self {
                root,
                peer,
                prepared,
                tenant,
                limit,
            },
            store,
            budgets,
        )
    }
    fn id(&self) -> [u8; 32] {
        Sha256::digest(self.prepared.call.encode_unsigned()).into()
    }
    fn reservation(&self) -> ProgramReservationRequest {
        let mut source = [0; 32];
        source[12..].copy_from_slice(&SENDER);
        ProgramReservationRequest {
            id: self.id(),
            charges: vec![ProgramBudgetCharge {
                kind: ProgramChargeKind::Fee,
                source,
                asset: asset(),
                amount: FEE,
                applicable_limits: vec![self.limit],
            }],
            expiry_sequence: 50,
            current_sequence: 5,
            core_deadline: Some(CoreTimestampMs(DEADLINE)),
        }
    }
    fn restart(&self, store: Store, budgets: BudgetLimiter) -> (Store, BudgetLimiter) {
        drop(store);
        drop(budgets);
        let reopened = Store::open(&self.root).unwrap();
        let restored = BudgetLimiter::new(Vec::new()).unwrap();
        load_daemon_limits(&reopened, &restored).unwrap();
        (reopened, restored)
    }
    fn key(&self, program: bool) -> TenantKey {
        let prefix: &[u8] = if program {
            b"native-program-approval-carrier-v1:"
        } else {
            b"native-effect-approval-carrier-v1:"
        };
        TenantKey::new(
            self.tenant.clone(),
            ObjectKind::PreparedActivity,
            [prefix, &self.id()].concat(),
        )
        .unwrap()
    }
}
fn asset() -> [u8; 32] {
    Sha256::digest(b"uhpx").into()
}
fn keys() -> Vec<SequencerAuthorization> {
    vec![SequencerAuthorization::new(
        [0x11; 32],
        SigningKey::from_bytes(&[0x31; 32])
            .verifying_key()
            .to_bytes(),
        6,
        99,
    )]
}
fn signed(prepared: &EvmPrepared, key_byte: u8) -> VerifiedSubmission {
    let mut secret = [0; 32];
    secret[31] = key_byte;
    let bytes = prepared.call.sign(&secret).unwrap();
    verify_before_submit(
        &bytes,
        &SigningPreparation {
            transaction: prepared.clone(),
            actor_address: prepared.from,
        },
        &prepared.from,
        &ModuleRegistry::new(&[]).unwrap(),
    )
    .unwrap()
}
fn hold_effect(f: &Fixture, store: &mut Store, budgets: &BudgetLimiter) -> Effect {
    let stage = budgets
        .stage_program_reservation(&f.reservation(), CoreTimestampMs(1000))
        .unwrap();
    Effect::hold_call(
        store,
        &f.peer,
        [3; 32],
        1,
        &f.prepared,
        stage,
        1000,
        &keys(),
    )
    .unwrap()
}
fn hold_program(f: &Fixture, store: &mut Store, budgets: &BudgetLimiter) -> Program {
    let stage = budgets
        .stage_program_reservation(&f.reservation(), CoreTimestampMs(1000))
        .unwrap();
    Program::hold_call(
        store,
        &f.peer,
        [3; 32],
        1,
        &f.prepared,
        stage,
        1000,
        &keys(),
    )
    .unwrap()
}

#[test]
fn effect_carrier_legacy_reopens_call_and_terminal_owner_decision() {
    let (f, mut store, budgets) = Fixture::new(false);
    let held = hold_effect(&f, &mut store, &budgets);
    let digest = held.call_held_digest().unwrap();
    let reservation = held.call_budget().unwrap();
    assert_eq!(held.restore_call().unwrap(), f.prepared);
    let (mut store, budgets) = f.restart(store, budgets);
    let restored = Effect::read_call(&store, &f.peer, f.id()).unwrap();
    assert_eq!(restored.restore_call().unwrap(), f.prepared);
    assert_eq!(restored.call_budget().unwrap(), reservation);
    assert_eq!(restored.call_held_digest().unwrap(), digest);
    restored.restore_call_budget(&budgets).unwrap();
    assert_eq!(budgets.held_exposure(f.limit).unwrap(), FEE);
    let granted = Effect::decide_call(
        &mut store,
        &budgets,
        &f.peer,
        f.id(),
        digest,
        [4; 32],
        true,
        6,
        1100,
    )
    .unwrap();
    assert_eq!(granted.call_state().unwrap(), CallApprovalState::Granted);
    let (mut store, budgets) = f.restart(store, budgets);
    let again = Effect::read_call(&store, &f.peer, f.id()).unwrap();
    again.restore_call_budget(&budgets).unwrap();
    assert_eq!(again.call_state().unwrap(), CallApprovalState::Granted);
    assert_eq!(again.call_held_digest().unwrap(), digest);
    assert_eq!(again.restore_call().unwrap(), f.prepared);
    assert_eq!(
        Effect::decide_call(
            &mut store,
            &budgets,
            &f.peer,
            f.id(),
            digest,
            [4; 32],
            true,
            7,
            1200
        )
        .unwrap()
        .call_state()
        .unwrap(),
        CallApprovalState::Granted
    );
    assert!(Effect::decide_call(
        &mut store,
        &budgets,
        &f.peer,
        f.id(),
        digest,
        [5; 32],
        false,
        7,
        1200
    )
    .is_err());
    let mut other = f.peer.clone();
    other.principal = "another-owner".to_owned();
    assert!(Effect::read_call(&store, &other, f.id()).is_err());
    let mut other = f.peer.clone();
    other.subject.as_mut().unwrap().owner = "did:layerx:other".to_owned();
    other.subject.as_mut().unwrap().account = "agent:did:layerx:other:main".to_owned();
    assert!(Effect::read_call(&store, &other, f.id()).is_err());
}

#[test]
fn effect_carrier_legacy_refuses_altered_bytes_sender_and_version_after_reopen() {
    let (f, mut store, budgets) = Fixture::new(false);
    hold_effect(&f, &mut store, &budgets);
    let key = f.key(false);
    let original = store.get(&key).unwrap().bytes().to_vec();
    let original_json: serde_json::Value = serde_json::from_slice(&original).unwrap();
    let mut cases = Vec::new();
    let mut altered = original_json.clone();
    altered["call_bound"]["sender"][0] = serde_json::json!(0);
    altered["actor"][0] = serde_json::json!(0);
    cases.push(altered);
    let mut altered = original_json.clone();
    let mut rebound_sender = SENDER;
    rebound_sender[0] ^= 1;
    let rebound =
        layerx_crypto::disclosure::call::bind_call(&f.prepared.call, rebound_sender).unwrap();
    altered["call_bound"]["sender"] = serde_json::json!(rebound_sender);
    altered["call_bound"]["disclosure_digest"] = serde_json::json!(rebound.audit_digest().unwrap());
    altered["disclosure_digest"] = altered["call_bound"]["disclosure_digest"].clone();
    altered["actor"] = serde_json::json!(rebound_sender);
    altered["authority"]["bytes"] = serde_json::json!(rebound_sender);
    cases.push(altered);
    let mut altered = original_json.clone();
    let mut changed_call = f.prepared.call.clone();
    changed_call.nonce += 1;
    altered["call_bound"]["canonical_bytes"] = serde_json::json!(changed_call.encode_unsigned());
    altered["canonical_bytes"] = altered["call_bound"]["canonical_bytes"].clone();
    cases.push(altered);
    let mut altered = original_json.clone();
    altered["call_bound"]["version"] = serde_json::json!(0);
    cases.push(altered);
    let mut altered = original_json.clone();
    altered["version"] = serde_json::json!(1);
    cases.push(altered);
    for altered in cases {
        store
            .put_local(key.clone(), serde_json::to_vec(&altered).unwrap())
            .unwrap();
        drop(store);
        store = Store::open(&f.root).unwrap();
        assert!(Effect::read_call(&store, &f.peer, f.id()).is_err());
        store.put_local(key.clone(), original.clone()).unwrap();
    }
    assert_eq!(
        Effect::read_call(&store, &f.peer, f.id())
            .unwrap()
            .restore_call()
            .unwrap(),
        f.prepared
    );
}

#[test]
fn effect_carrier_legacy_rejection_and_expiry_preserve_atomic_budget_release() {
    let (f, mut store, budgets) = Fixture::new(false);
    let held = hold_effect(&f, &mut store, &budgets);
    let digest = held.call_held_digest().unwrap();
    std::fs::create_dir(f.root.join("store.bin.tmp")).unwrap();
    assert!(matches!(
        Effect::decide_call(
            &mut store,
            &budgets,
            &f.peer,
            f.id(),
            digest,
            [6; 32],
            false,
            6,
            1100
        ),
        Err(CallApprovalError::Store)
    ));
    assert_eq!(budgets.held_exposure(f.limit).unwrap(), FEE);
    assert_eq!(
        Effect::read_call(&store, &f.peer, f.id())
            .unwrap()
            .call_state()
            .unwrap(),
        CallApprovalState::Awaiting
    );
    std::fs::remove_dir(f.root.join("store.bin.tmp")).unwrap();
    assert_eq!(
        Effect::decide_call(
            &mut store,
            &budgets,
            &f.peer,
            f.id(),
            digest,
            [6; 32],
            false,
            6,
            1100
        )
        .unwrap()
        .call_state()
        .unwrap(),
        CallApprovalState::Rejected
    );
    assert_eq!(budgets.held_exposure(f.limit).unwrap(), 0);
    let (mut store, budgets) = f.restart(store, budgets);
    let rejected = Effect::read_call(&store, &f.peer, f.id()).unwrap();
    rejected.restore_call_budget(&budgets).unwrap();
    assert_eq!(rejected.call_state().unwrap(), CallApprovalState::Rejected);
    assert!(!budgets.has_reservation(f.id()).unwrap());
    assert_eq!(
        Effect::decide_call(
            &mut store,
            &budgets,
            &f.peer,
            f.id(),
            digest,
            [6; 32],
            false,
            7,
            1200
        )
        .unwrap()
        .call_state()
        .unwrap(),
        CallApprovalState::Rejected
    );
    assert!(Effect::queue_call(
        &mut store,
        &mut Outbox::default(),
        &f.peer,
        f.id(),
        &signed(&f.prepared, 1),
        &ModuleRegistry::new(&[]).unwrap(),
        7,
        1200
    )
    .is_err());

    let (f, mut store, budgets) = Fixture::new(false);
    let held = hold_effect(&f, &mut store, &budgets);
    let expired = Effect::decide_call(
        &mut store,
        &budgets,
        &f.peer,
        f.id(),
        held.call_held_digest().unwrap(),
        [7; 32],
        true,
        6,
        DEADLINE,
    )
    .unwrap();
    assert_eq!(expired.call_state().unwrap(), CallApprovalState::Expired);
    assert_eq!(budgets.held_exposure(f.limit).unwrap(), 0);
    let (store, budgets) = f.restart(store, budgets);
    let recovered = Effect::read_call(&store, &f.peer, f.id()).unwrap();
    recovered.restore_call_budget(&budgets).unwrap();
    assert_eq!(recovered.call_state().unwrap(), CallApprovalState::Expired);
    assert!(!budgets.has_reservation(f.id()).unwrap());
}

#[test]
fn program_carrier_legacy_survives_restart_approval_submission_and_settlement() {
    let (f, mut store, budgets) = Fixture::new(true);
    let held = hold_program(&f, &mut store, &budgets);
    let digest = held.call_held_digest().unwrap();
    let reservation = held.call_budget().unwrap();
    let registry = ModuleRegistry::new(&[]).unwrap();
    let verified = signed(&f.prepared, 1);
    let hash = transaction_hash(verified.exact_bytes());
    assert!(Program::queue_call(
        &mut store,
        &mut Outbox::default(),
        &f.peer,
        f.id(),
        &verified,
        &registry,
        6,
        1100
    )
    .is_err());
    let (mut store, budgets) = f.restart(store, budgets);
    let restored = Program::read_call(&store, &f.peer, f.id()).unwrap();
    restored.restore_call_budget(&budgets).unwrap();
    assert_eq!(restored.restore_call().unwrap(), f.prepared);
    assert_eq!(restored.call_budget().unwrap(), reservation);
    Program::decide_call(
        &mut store,
        &budgets,
        &f.peer,
        f.id(),
        digest,
        [8; 32],
        true,
        6,
        1100,
    )
    .unwrap();
    let (mut store, budgets) = f.restart(store, budgets);
    let granted = Program::read_call(&store, &f.peer, f.id()).unwrap();
    granted.restore_call_budget(&budgets).unwrap();
    assert_eq!(granted.call_state().unwrap(), CallApprovalState::Granted);
    assert_eq!(granted.call_held_digest().unwrap(), digest);
    let mut outbox = Outbox::default();
    let queued = Program::queue_call(
        &mut store,
        &mut outbox,
        &f.peer,
        f.id(),
        &verified,
        &registry,
        6,
        1200,
    )
    .unwrap();
    assert_eq!(queued.call_state().unwrap(), CallApprovalState::Queued);
    assert_eq!(
        outbox.exact_signed_bytes(hash).unwrap(),
        verified.exact_bytes()
    );
    drop(outbox);
    let (mut store, budgets) = f.restart(store, budgets);
    let queued = Program::read_call(&store, &f.peer, f.id()).unwrap();
    queued.restore_call_budget(&budgets).unwrap();
    let mut outbox = Outbox::default();
    let recovered =
        Program::recover_call_submission(&mut store, &mut outbox, &f.peer, f.id()).unwrap();
    assert_eq!(recovered.call_state().unwrap(), CallApprovalState::Queued);
    assert_eq!(recovered.restore_call().unwrap(), f.prepared);
    assert_eq!(recovered.call_budget().unwrap(), reservation);
    assert_eq!(
        outbox.bytes_for_transmission(hash).unwrap(),
        verified.exact_bytes()
    );
    let receipt = signed_receipt(&f.prepared, hash, FEE, &SigningKey::from_bytes(&[0x31; 32]));
    let mut wrong = receipt.clone();
    let end = wrong.len() - 1;
    wrong[end] ^= 1;
    assert!(matches!(
        Program::settle_call(&mut store, &budgets, &f.peer, f.id(), &wrong),
        Err(CallApprovalError::Receipt)
    ));
    assert_eq!(budgets.held_exposure(f.limit).unwrap(), FEE);
    let wrong_hash = signed_receipt(
        &f.prepared,
        [0x99; 32],
        FEE,
        &SigningKey::from_bytes(&[0x31; 32]),
    );
    assert!(Program::settle_call(&mut store, &budgets, &f.peer, f.id(), &wrong_hash).is_err());
    let lower_fee = signed_receipt(
        &f.prepared,
        hash,
        FEE - 1,
        &SigningKey::from_bytes(&[0x31; 32]),
    );
    assert!(matches!(
        Program::settle_call(&mut store, &budgets, &f.peer, f.id(), &lower_fee),
        Err(CallApprovalError::DebitProofRequired)
    ));
    std::fs::create_dir(f.root.join("store.bin.tmp")).unwrap();
    assert!(matches!(
        Program::settle_call(&mut store, &budgets, &f.peer, f.id(), &receipt),
        Err(CallApprovalError::Store)
    ));
    assert_eq!(budgets.held_exposure(f.limit).unwrap(), FEE);
    assert_eq!(budgets.consumed(f.limit).unwrap(), 0);
    assert_eq!(daemon_limits(&store, &f.tenant).unwrap()[0].consumed, 0);
    assert_eq!(
        Program::read_call(&store, &f.peer, f.id())
            .unwrap()
            .call_state()
            .unwrap(),
        CallApprovalState::Queued
    );
    std::fs::remove_dir(f.root.join("store.bin.tmp")).unwrap();
    let settled = Program::settle_call(&mut store, &budgets, &f.peer, f.id(), &receipt).unwrap();
    assert_eq!(settled.call_state().unwrap(), CallApprovalState::Settled);
    assert_eq!(budgets.held_exposure(f.limit).unwrap(), 0);
    assert_eq!(budgets.consumed(f.limit).unwrap(), FEE);
    assert_eq!(daemon_limits(&store, &f.tenant).unwrap()[0].consumed, FEE);
    drop(outbox);
    let (mut store, budgets) = f.restart(store, budgets);
    let settled = Program::read_call(&store, &f.peer, f.id()).unwrap();
    settled.restore_call_budget(&budgets).unwrap();
    assert_eq!(settled.call_state().unwrap(), CallApprovalState::Settled);
    assert_eq!(settled.restore_call().unwrap(), f.prepared);
    assert_eq!(settled.call_held_digest().unwrap(), digest);
    assert_eq!(settled.call_budget().unwrap(), reservation);
    assert_eq!(budgets.held_exposure(f.limit).unwrap(), 0);
    assert_eq!(budgets.consumed(f.limit).unwrap(), FEE);
    Program::settle_call(&mut store, &budgets, &f.peer, f.id(), &receipt).unwrap();
    assert_eq!(budgets.consumed(f.limit).unwrap(), FEE);
}

#[test]
fn program_carrier_legacy_refuses_changed_signed_call_and_recovered_sender() {
    let (f, mut store, budgets) = Fixture::new(true);
    let held = hold_program(&f, &mut store, &budgets);
    Program::decide_call(
        &mut store,
        &budgets,
        &f.peer,
        f.id(),
        held.call_held_digest().unwrap(),
        [9; 32],
        true,
        6,
        1100,
    )
    .unwrap();
    let mut altered = f.prepared.clone();
    altered.call.nonce += 1;
    altered.signing_hash = altered.call.signing_hash();
    altered.disclosure =
        layerx_crypto::disclosure::call::bind_call(&altered.call, altered.from).unwrap();
    altered.disclosure_digest = altered.disclosure.signing_digest().unwrap();
    let registry = ModuleRegistry::new(&[]).unwrap();
    assert!(Program::queue_call(
        &mut store,
        &mut Outbox::default(),
        &f.peer,
        f.id(),
        &signed(&altered, 1),
        &registry,
        6,
        1200
    )
    .is_err());
    let mut secret = [0; 32];
    secret[31] = 2;
    let (call, signature) =
        layerx_crypto::evm_transaction::decode_signed(&f.prepared.call.sign(&secret).unwrap())
            .unwrap();
    altered = f.prepared.clone();
    altered.from = layerx_crypto::evm_transaction::recover_sender(&call, &signature).unwrap();
    altered.disclosure =
        layerx_crypto::disclosure::call::bind_call(&altered.call, altered.from).unwrap();
    altered.disclosure_digest = altered.disclosure.signing_digest().unwrap();
    assert!(Program::queue_call(
        &mut store,
        &mut Outbox::default(),
        &f.peer,
        f.id(),
        &signed(&altered, 2),
        &registry,
        6,
        1200
    )
    .is_err());
    assert_eq!(
        Program::read_call(&store, &f.peer, f.id())
            .unwrap()
            .call_state()
            .unwrap(),
        CallApprovalState::Granted
    );
    assert_eq!(budgets.held_exposure(f.limit).unwrap(), FEE);
    assert!(store
        .list_object_ids(&f.tenant, ObjectKind::Outbox)
        .is_empty());
}

#[test]
fn program_carrier_legacy_recovers_signed_intent_after_real_outbox_refusal() {
    let (f, mut store, budgets) = Fixture::new(true);
    let held = hold_program(&f, &mut store, &budgets);
    Program::decide_call(
        &mut store,
        &budgets,
        &f.peer,
        f.id(),
        held.call_held_digest().unwrap(),
        [10; 32],
        true,
        6,
        1100,
    )
    .unwrap();
    let verified = signed(&f.prepared, 1);
    let hash = transaction_hash(verified.exact_bytes());
    let mut another_store = Store::open(f.root.join("another-outbox")).unwrap();
    let mut already_queued = Outbox::default();
    already_queued
        .enqueue(
            &mut another_store,
            f.tenant.clone(),
            verified.exact_bytes().to_vec(),
        )
        .unwrap();
    assert!(matches!(
        Program::queue_call(
            &mut store,
            &mut already_queued,
            &f.peer,
            f.id(),
            &verified,
            &ModuleRegistry::new(&[]).unwrap(),
            6,
            1200
        ),
        Err(CallApprovalError::Store)
    ));
    assert_eq!(
        Program::read_call(&store, &f.peer, f.id())
            .unwrap()
            .call_state()
            .unwrap(),
        CallApprovalState::Signed
    );
    assert!(store
        .list_object_ids(&f.tenant, ObjectKind::Outbox)
        .is_empty());
    assert_eq!(budgets.held_exposure(f.limit).unwrap(), FEE);
    drop(already_queued);
    drop(another_store);
    let (mut store, budgets) = f.restart(store, budgets);
    let signed = Program::read_call(&store, &f.peer, f.id()).unwrap();
    signed.restore_call_budget(&budgets).unwrap();
    let mut outbox = Outbox::default();
    let recovered =
        Program::recover_call_submission(&mut store, &mut outbox, &f.peer, f.id()).unwrap();
    assert_eq!(recovered.call_state().unwrap(), CallApprovalState::Queued);
    assert_eq!(recovered.restore_call().unwrap(), f.prepared);
    assert_eq!(
        outbox.bytes_for_transmission(hash).unwrap(),
        verified.exact_bytes()
    );
    assert_eq!(
        store.list_object_ids(&f.tenant, ObjectKind::Outbox),
        vec![hash.to_vec()]
    );
    Program::recover_call_submission(&mut store, &mut outbox, &f.peer, f.id()).unwrap();
    assert_eq!(
        store.list_object_ids(&f.tenant, ObjectKind::Outbox),
        vec![hash.to_vec()]
    );
}

#[test]
fn effect_carrier_legacy_failed_hold_does_not_publish_budget_or_carrier() {
    let (f, mut store, budgets) = Fixture::new(false);
    std::fs::create_dir(f.root.join("store.bin.tmp")).unwrap();
    let stage = budgets
        .stage_program_reservation(&f.reservation(), CoreTimestampMs(1000))
        .unwrap();
    assert!(matches!(
        Effect::hold_call(
            &mut store,
            &f.peer,
            [3; 32],
            1,
            &f.prepared,
            stage,
            1000,
            &keys()
        ),
        Err(CallApprovalError::Store)
    ));
    assert!(!budgets.has_reservation(f.id()).unwrap());
    assert!(store.get(&f.key(false)).is_none());
    std::fs::remove_dir(f.root.join("store.bin.tmp")).unwrap();
    let held = hold_effect(&f, &mut store, &budgets);
    assert_eq!(held.call_state().unwrap(), CallApprovalState::Awaiting);
    assert_eq!(budgets.held_exposure(f.limit).unwrap(), FEE);
}

fn signed_receipt(
    prepared: &EvmPrepared,
    hash: [u8; 32],
    fee: u128,
    signer: &SigningKey,
) -> Vec<u8> {
    let unsigned = receipt(prepared, hash, fee, None);
    let signature = signer
        .sign(&layerx_wire::hash::receipt_digest(&unsigned).unwrap())
        .to_bytes();
    let signed = receipt(prepared, hash, fee, Some(signature));
    let decoded = layerx_wire::receipt::decode(&signed).unwrap();
    assert_eq!(layerx_wire::receipt::encode(&decoded).unwrap(), signed);
    signed
}

fn receipt(
    prepared: &EvmPrepared,
    hash: [u8; 32],
    fee: u128,
    signature: Option<[u8; 64]>,
) -> Vec<u8> {
    // The production wire encoder and receipt signer produce the sequencer's
    // type-2 EVM layout; the carrier consumes it through signature verification.
    let mut summary = Encoder::new(256);
    summary.u64(50_000).unwrap();
    summary.u64(50_000).unwrap();
    summary.u64(3).unwrap();
    summary.u32(0).unwrap();
    summary.fixed(&[0; 20]).unwrap();
    summary.fixed(&[0; 20]).unwrap();
    summary
        .fixed(&<[u8; 32]>::from(Sha256::digest(b"")))
        .unwrap();
    summary.u32(0).unwrap();
    summary.u32(0).unwrap();
    summary.u8(4).unwrap();
    summary.fixed(b"uhpx").unwrap();
    let mut out = Encoder::new(4096);
    out.structure_header_version(0x5201, 2).unwrap();
    out.u16(2).unwrap();
    out.bytes(&hash, 32).unwrap();
    out.u64(7).unwrap();
    let previous = [2; 32];
    let next: [u8; 32] = Sha256::new()
        .chain_update(b"PAXEER/v1/sequence\0")
        .chain_update(previous)
        .chain_update(7_u64.to_be_bytes())
        .chain_update(hash)
        .finalize()
        .into();
    out.bytes(&previous, 32).unwrap();
    out.bytes(&next, 32).unwrap();
    out.bytes(&<[u8; 32]>::from(Sha256::digest(b"")), 32)
        .unwrap();
    out.i32(0).unwrap();
    out.sequence_length(1, 512).unwrap();
    out.u16(12).unwrap();
    out.u16(0).unwrap();
    out.u16(0).unwrap();
    out.u8(1).unwrap();
    out.u8(0).unwrap();
    out.bytes(&[0; 32], 32).unwrap();
    out.bytes(&summary.finish(), 256).unwrap();
    out.u128(fee).unwrap();
    let mut height = [0; 32];
    height[31] = 6;
    out.bytes(&height, 32).unwrap();
    out.u16(12).unwrap();
    out.u32(1).unwrap();
    out.u32(0).unwrap();
    out.u8(2).unwrap();
    out.bytes(&asset(), 32).unwrap();
    out.u128(0).unwrap();
    let mut from = [0; 32];
    from[12..].copy_from_slice(&prepared.from);
    out.bytes(&from, 32).unwrap();
    out.u128(1_000_000).unwrap();
    out.u128(1_000_000 - fee).unwrap();
    out.u64(prepared.call.nonce).unwrap();
    let mut to = [0; 32];
    to[12..].copy_from_slice(&prepared.call.to);
    out.bytes(&to, 32).unwrap();
    out.u128(0).unwrap();
    out.u128(0).unwrap();
    out.bytes(&[0; 32], 32).unwrap();
    out.bytes(&[0; 32], 32).unwrap();
    out.bytes(&[0x11; 32], 32).unwrap();
    out.u64(1300).unwrap();
    out.u8(u8::from(signature.is_some())).unwrap();
    if let Some(signature) = signature {
        out.bytes(&signature, 64).unwrap();
    }
    out.finish()
}
