fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

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
        let namespace = require_ok(
            layerx_identity_binding::subject_namespace(&transport_tenant, &principal),
            "required test value",
        );
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
            require_ok(
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
                ),
                "required test value",
            )
        } else {
            PrecompileCall {
                target: PrecompileTarget::Ledger,
                selector: ledger_selector("openAccount(bytes32)"),
                arguments: vec![AbiArgument::Word(asset())],
            }
        };
        finish_new_1_repair_22_31_225(root, namespace, peer, call)
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
        let reopened = require_ok(Store::open(&self.root), "required test value");
        let restored = require_ok(BudgetLimiter::new(Vec::new()), "required test value");
        require_ok(
            load_daemon_limits(&reopened, &restored),
            "required test value",
        );
        (reopened, restored)
    }
    fn key(&self, program: bool) -> TenantKey {
        let prefix: &[u8] = if program {
            b"native-program-approval-carrier-v1:"
        } else {
            b"native-effect-approval-carrier-v1:"
        };
        require_ok(
            TenantKey::new(
                self.tenant.clone(),
                ObjectKind::PreparedActivity,
                [prefix, &self.id()].concat(),
            ),
            "required test value",
        )
    }
}

fn finish_new_1_repair_22_31_225(
    root: PathBuf,
    namespace: String,
    peer: HumanPeer,
    call: PrecompileCall,
) -> (Fixture, Store, BudgetLimiter) {
    let prepared = require_ok(
        prepare_bound(
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
        ),
        "required test value",
    );
    assert_eq!(prepared.disclosure.fee_limit, FEE);
    require_ok(verify_call_binding(&prepared), "required test value");
    let tenant = require_ok(TenantId::new(namespace), "required test value");
    let mut store = require_ok(Store::open(&root), "required test value");
    let budgets = require_ok(BudgetLimiter::new(Vec::new()), "required test value");
    let limit = daemon_limit_id([0x21; 32]);
    require_ok(
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
        ),
        "required test value",
    );
    (
        Fixture {
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
    let bytes = require_ok(prepared.call.sign(&secret), "required test value");
    require_ok(
        verify_before_submit(
            &bytes,
            &SigningPreparation {
                transaction: prepared.clone(),
                actor_address: prepared.from,
            },
            &prepared.from,
            &require_ok(ModuleRegistry::new(&[]), "required test value"),
        ),
        "required test value",
    )
}
fn hold_effect(f: &Fixture, store: &mut Store, budgets: &BudgetLimiter) -> Effect {
    let stage = require_ok(
        budgets.stage_program_reservation(&f.reservation(), CoreTimestampMs(1000)),
        "required test value",
    );
    require_ok(
        Effect::hold_call(
            store,
            &f.peer,
            [3; 32],
            1,
            &f.prepared,
            stage,
            1000,
            &keys(),
        ),
        "required test value",
    )
}
fn hold_program(f: &Fixture, store: &mut Store, budgets: &BudgetLimiter) -> Program {
    let stage = require_ok(
        budgets.stage_program_reservation(&f.reservation(), CoreTimestampMs(1000)),
        "required test value",
    );
    require_ok(
        Program::hold_call(
            store,
            &f.peer,
            [3; 32],
            1,
            &f.prepared,
            stage,
            1000,
            &keys(),
        ),
        "required test value",
    )
}

#[test]
fn effect_carrier_legacy_reopens_call_and_terminal_owner_decision() {
    let (f, mut store, budgets) = Fixture::new(false);
    let held = hold_effect(&f, &mut store, &budgets);
    let digest = require_ok(held.call_held_digest(), "required test value");
    let reservation = require_ok(held.call_budget(), "required test value");
    assert_eq!(
        require_ok(held.restore_call(), "required test value"),
        f.prepared
    );
    let (mut store, budgets) = f.restart(store, budgets);
    let restored = require_ok(
        Effect::read_call(&store, &f.peer, f.id()),
        "required test value",
    );
    assert_eq!(
        require_ok(restored.restore_call(), "required test value"),
        f.prepared
    );
    assert_eq!(
        require_ok(restored.call_budget(), "required test value"),
        reservation
    );
    assert_eq!(
        require_ok(restored.call_held_digest(), "required test value"),
        digest
    );
    require_ok(
        restored.restore_call_budget(&budgets),
        "required test value",
    );
    assert_eq!(
        require_ok(budgets.held_exposure(f.limit), "required test value"),
        FEE
    );
    let granted = require_ok(
        Effect::decide_call(
            &mut store,
            &budgets,
            &f.peer,
            f.id(),
            digest,
            [4; 32],
            true,
            6,
            1100,
        ),
        "required test value",
    );
    assert_eq!(
        require_ok(granted.call_state(), "required test value"),
        CallApprovalState::Granted
    );
    complete_effect_carrier_legacy_reopens_call_and_terminal_owner_decision_1_repair_22_31_225(
        &f, digest, store, budgets,
    );
}

fn complete_effect_carrier_legacy_reopens_call_and_terminal_owner_decision_1_repair_22_31_225(
    f: &Fixture,
    digest: [u8; 32],
    store: Store,
    budgets: BudgetLimiter,
) {
    let (mut store, budgets) = f.restart(store, budgets);
    let again = require_ok(
        Effect::read_call(&store, &f.peer, f.id()),
        "required test value",
    );
    require_ok(again.restore_call_budget(&budgets), "required test value");
    assert_eq!(
        require_ok(again.call_state(), "required test value"),
        CallApprovalState::Granted
    );
    assert_eq!(
        require_ok(again.call_held_digest(), "required test value"),
        digest
    );
    assert_eq!(
        require_ok(again.restore_call(), "required test value"),
        f.prepared
    );
    assert_eq!(
        require_ok(
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
            .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
            .call_state(),
            "required test value"
        ),
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
    "another-owner".clone_into(&mut other.principal);
    assert!(Effect::read_call(&store, &other, f.id()).is_err());
    let mut other = f.peer.clone();
    "did:layerx:other"
        .clone_into(&mut require_some(other.subject.as_mut(), "required test value").owner);
    "agent:did:layerx:other:main"
        .clone_into(&mut require_some(other.subject.as_mut(), "required test value").account);
    assert!(Effect::read_call(&store, &other, f.id()).is_err());
}

#[test]
fn effect_carrier_legacy_refuses_altered_bytes_sender_and_version_after_reopen() {
    let (f, mut store, budgets) = Fixture::new(false);
    hold_effect(&f, &mut store, &budgets);
    let key = f.key(false);
    let original = require_some(store.get(&key), "required test value")
        .bytes()
        .to_vec();
    let original_json: serde_json::Value =
        require_ok(serde_json::from_slice(&original), "required test value");
    let mut cases = Vec::new();
    let mut altered = original_json.clone();
    altered["call_bound"]["sender"][0] = serde_json::json!(0);
    altered["actor"][0] = serde_json::json!(0);
    cases.push(altered);
    let mut altered = original_json.clone();
    let mut rebound_sender = SENDER;
    rebound_sender[0] ^= 1;
    let rebound = require_ok(
        layerx_crypto::disclosure::call::bind_call(&f.prepared.call, rebound_sender),
        "required test value",
    );
    altered["call_bound"]["sender"] = serde_json::json!(rebound_sender);
    altered["call_bound"]["disclosure_digest"] =
        serde_json::json!(require_ok(rebound.audit_digest(), "required test value"));
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
        require_ok(
            store.put_local(
                key.clone(),
                require_ok(serde_json::to_vec(&altered), "required test value"),
            ),
            "required test value",
        );
        drop(store);
        store = require_ok(Store::open(&f.root), "required test value");
        assert!(Effect::read_call(&store, &f.peer, f.id()).is_err());
        require_ok(
            store.put_local(key.clone(), original.clone()),
            "required test value",
        );
    }
    assert_eq!(
        require_ok(
            Effect::read_call(&store, &f.peer, f.id())
                .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
                .restore_call(),
            "required test value"
        ),
        f.prepared
    );
}

#[test]
fn effect_carrier_legacy_rejection_and_expiry_preserve_atomic_budget_release() {
    let (f, mut store, budgets) = Fixture::new(false);
    let held = hold_effect(&f, &mut store, &budgets);
    let digest = require_ok(held.call_held_digest(), "required test value");
    require_ok(
        std::fs::create_dir(f.root.join("store.bin.tmp")),
        "required test value",
    );
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
    assert_eq!(
        require_ok(budgets.held_exposure(f.limit), "required test value"),
        FEE
    );
    assert_eq!(
        require_ok(
            Effect::read_call(&store, &f.peer, f.id())
                .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
                .call_state(),
            "required test value"
        ),
        CallApprovalState::Awaiting
    );
    require_ok(
        std::fs::remove_dir(f.root.join("store.bin.tmp")),
        "required test value",
    );
    assert_eq!(
        require_ok(
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
            .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
            .call_state(),
            "required test value"
        ),
        CallApprovalState::Rejected
    );
    complete_effect_carrier_legacy_rejection_and_expiry_preserve_atomic_budget_release_2_repair_22_31_225(&f, store, budgets, digest);
}

fn complete_effect_carrier_legacy_rejection_and_expiry_preserve_atomic_budget_release_2_repair_22_31_225(
    f: &Fixture,
    store: Store,
    budgets: BudgetLimiter,
    digest: [u8; 32],
) {
    assert_eq!(
        require_ok(budgets.held_exposure(f.limit), "required test value"),
        0
    );
    let (mut store, budgets) = f.restart(store, budgets);
    let rejected = require_ok(
        Effect::read_call(&store, &f.peer, f.id()),
        "required test value",
    );
    require_ok(
        rejected.restore_call_budget(&budgets),
        "required test value",
    );
    assert_eq!(
        require_ok(rejected.call_state(), "required test value"),
        CallApprovalState::Rejected
    );
    assert!(!require_ok(
        budgets.has_reservation(f.id()),
        "required test value"
    ));
    assert_eq!(
        require_ok(
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
            .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
            .call_state(),
            "required test value"
        ),
        CallApprovalState::Rejected
    );
    complete_effect_carrier_legacy_rejection_and_expiry_preserve_atomic_budget_release_1_repair_22_31_225(f, store);
}

fn complete_effect_carrier_legacy_rejection_and_expiry_preserve_atomic_budget_release_1_repair_22_31_225(
    f: &Fixture,
    mut store: Store,
) {
    assert!(Effect::queue_call(
        &mut store,
        &mut Outbox::default(),
        &f.peer,
        f.id(),
        &signed(&f.prepared, 1),
        &require_ok(ModuleRegistry::new(&[]), "required test value"),
        7,
        1200
    )
    .is_err());

    let (f, mut store, budgets) = Fixture::new(false);
    let held = hold_effect(&f, &mut store, &budgets);
    let expired = require_ok(
        Effect::decide_call(
            &mut store,
            &budgets,
            &f.peer,
            f.id(),
            require_ok(held.call_held_digest(), "required test value"),
            [7; 32],
            true,
            6,
            DEADLINE,
        ),
        "required test value",
    );
    assert_eq!(
        require_ok(expired.call_state(), "required test value"),
        CallApprovalState::Expired
    );
    assert_eq!(
        require_ok(budgets.held_exposure(f.limit), "required test value"),
        0
    );
    let (store, budgets) = f.restart(store, budgets);
    let recovered = require_ok(
        Effect::read_call(&store, &f.peer, f.id()),
        "required test value",
    );
    require_ok(
        recovered.restore_call_budget(&budgets),
        "required test value",
    );
    assert_eq!(
        require_ok(recovered.call_state(), "required test value"),
        CallApprovalState::Expired
    );
    assert!(!require_ok(
        budgets.has_reservation(f.id()),
        "required test value"
    ));
}

#[test]
fn program_carrier_legacy_survives_restart_approval_submission_and_settlement() {
    let (f, mut store, budgets) = Fixture::new(true);
    let held = hold_program(&f, &mut store, &budgets);
    let digest = require_ok(held.call_held_digest(), "required test value");
    let reservation = require_ok(held.call_budget(), "required test value");
    let registry = require_ok(ModuleRegistry::new(&[]), "required test value");
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
    let restored = require_ok(
        Program::read_call(&store, &f.peer, f.id()),
        "required test value",
    );
    require_ok(
        restored.restore_call_budget(&budgets),
        "required test value",
    );
    assert_eq!(
        require_ok(restored.restore_call(), "required test value"),
        f.prepared
    );
    assert_eq!(
        require_ok(restored.call_budget(), "required test value"),
        reservation
    );
    require_ok(
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
        ),
        "required test value",
    );
    complete_program_carrier_legacy_survives_restart_approval_submission_and_settlement_1_repair_22_31_225(CompleteProgramCarrierLegacySurvivesRestartApprovalSubmissionAndSettlement1Repair2231225Context { f, digest, reservation, registry, verified, hash, store, budgets });
}

struct CompleteProgramCarrierLegacySurvivesRestartApprovalSubmissionAndSettlement1Repair2231225Context
{
    f: Fixture,
    digest: [u8; 32],
    reservation: layerx_agentd::budget::ProgramBudgetReservation,
    registry: ModuleRegistry,
    verified: VerifiedSubmission,
    hash: [u8; 32],
    store: Store,
    budgets: BudgetLimiter,
}

fn complete_program_carrier_legacy_survives_restart_approval_submission_and_settlement_1_repair_22_31_225(
    context: CompleteProgramCarrierLegacySurvivesRestartApprovalSubmissionAndSettlement1Repair2231225Context,
) {
    let CompleteProgramCarrierLegacySurvivesRestartApprovalSubmissionAndSettlement1Repair2231225Context { f, digest, reservation, registry, verified, hash, store, budgets } = context;

    let (mut store, budgets) = f.restart(store, budgets);
    let granted = require_ok(
        Program::read_call(&store, &f.peer, f.id()),
        "required test value",
    );
    require_ok(granted.restore_call_budget(&budgets), "required test value");
    assert_eq!(
        require_ok(granted.call_state(), "required test value"),
        CallApprovalState::Granted
    );
    assert_eq!(
        require_ok(granted.call_held_digest(), "required test value"),
        digest
    );
    let mut outbox = Outbox::default();
    let queued = require_ok(
        Program::queue_call(
            &mut store,
            &mut outbox,
            &f.peer,
            f.id(),
            &verified,
            &registry,
            6,
            1200,
        ),
        "required test value",
    );
    assert_eq!(
        require_ok(queued.call_state(), "required test value"),
        CallApprovalState::Queued
    );
    assert_eq!(
        require_ok(outbox.exact_signed_bytes(hash), "required test value"),
        verified.exact_bytes()
    );
    drop(outbox);
    let (mut store, budgets) = f.restart(store, budgets);
    let queued = require_ok(
        Program::read_call(&store, &f.peer, f.id()),
        "required test value",
    );
    require_ok(queued.restore_call_budget(&budgets), "required test value");
    let mut outbox = Outbox::default();
    let recovered = require_ok(
        Program::recover_call_submission(&mut store, &mut outbox, &f.peer, f.id()),
        "required test value",
    );
    assert_eq!(
        require_ok(recovered.call_state(), "required test value"),
        CallApprovalState::Queued
    );
    finish_program_carrier_legacy_survives_restart_approval_submission_and_settlement_1_repair_22_31_225(FinishProgramCarrierLegacySurvivesRestartApprovalSubmissionAndSettlement1Repair2231225Context { f, digest, reservation, verified, hash, store, budgets, outbox, recovered });
}

struct FinishProgramCarrierLegacySurvivesRestartApprovalSubmissionAndSettlement1Repair2231225Context
{
    f: Fixture,
    digest: [u8; 32],
    reservation: layerx_agentd::budget::ProgramBudgetReservation,
    verified: VerifiedSubmission,
    hash: [u8; 32],
    store: Store,
    budgets: BudgetLimiter,
    outbox: Outbox,
    recovered: Program,
}

fn finish_program_carrier_legacy_survives_restart_approval_submission_and_settlement_1_repair_22_31_225(
    context: FinishProgramCarrierLegacySurvivesRestartApprovalSubmissionAndSettlement1Repair2231225Context,
) {
    let FinishProgramCarrierLegacySurvivesRestartApprovalSubmissionAndSettlement1Repair2231225Context { f, digest, reservation, verified, hash, mut store, budgets, outbox, recovered } = context;

    assert_eq!(
        require_ok(recovered.restore_call(), "required test value"),
        f.prepared
    );
    assert_eq!(
        require_ok(recovered.call_budget(), "required test value"),
        reservation
    );
    assert_eq!(
        require_ok(outbox.bytes_for_transmission(hash), "required test value"),
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
    assert_eq!(
        require_ok(budgets.held_exposure(f.limit), "required test value"),
        FEE
    );
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
    require_ok(
        std::fs::create_dir(f.root.join("store.bin.tmp")),
        "required test value",
    );
    assert!(matches!(
        Program::settle_call(&mut store, &budgets, &f.peer, f.id(), &receipt),
        Err(CallApprovalError::Store)
    ));
    assert_eq!(
        require_ok(budgets.held_exposure(f.limit), "required test value"),
        FEE
    );
    assert_eq!(
        require_ok(budgets.consumed(f.limit), "required test value"),
        0
    );
    assert_eq!(
        require_ok(daemon_limits(&store, &f.tenant), "required test value")[0].consumed,
        0
    );
    assert_eq!(
        require_ok(
            Program::read_call(&store, &f.peer, f.id())
                .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
                .call_state(),
            "required test value"
        ),
        CallApprovalState::Queued
    );
    require_ok(
        std::fs::remove_dir(f.root.join("store.bin.tmp")),
        "required test value",
    );
    complete_finish_program_carrier_legacy_survives_restart_approval_submission_and_settlement_1_repair_22_31_225_1_repair_22_31_225(CompleteFinishProgramCarrierLegacySurvivesRestartApprovalSubmissionAndSettlement1Repair22312251Repair2231225Context { f, digest, reservation, store, budgets, outbox, receipt });
}

struct CompleteFinishProgramCarrierLegacySurvivesRestartApprovalSubmissionAndSettlement1Repair22312251Repair2231225Context
{
    f: Fixture,
    digest: [u8; 32],
    reservation: layerx_agentd::budget::ProgramBudgetReservation,
    store: Store,
    budgets: BudgetLimiter,
    outbox: Outbox,
    receipt: Vec<u8>,
}

fn complete_finish_program_carrier_legacy_survives_restart_approval_submission_and_settlement_1_repair_22_31_225_1_repair_22_31_225(
    context: CompleteFinishProgramCarrierLegacySurvivesRestartApprovalSubmissionAndSettlement1Repair22312251Repair2231225Context,
) {
    let CompleteFinishProgramCarrierLegacySurvivesRestartApprovalSubmissionAndSettlement1Repair22312251Repair2231225Context { f, digest, reservation, mut store, budgets, outbox, receipt } = context;

    let settled = require_ok(
        Program::settle_call(&mut store, &budgets, &f.peer, f.id(), &receipt),
        "required test value",
    );
    assert_eq!(
        require_ok(settled.call_state(), "required test value"),
        CallApprovalState::Settled
    );
    assert_eq!(
        require_ok(budgets.held_exposure(f.limit), "required test value"),
        0
    );
    assert_eq!(
        require_ok(budgets.consumed(f.limit), "required test value"),
        FEE
    );
    assert_eq!(
        require_ok(daemon_limits(&store, &f.tenant), "required test value")[0].consumed,
        FEE
    );
    drop(outbox);
    let (mut store, budgets) = f.restart(store, budgets);
    let settled = require_ok(
        Program::read_call(&store, &f.peer, f.id()),
        "required test value",
    );
    require_ok(settled.restore_call_budget(&budgets), "required test value");
    assert_eq!(
        require_ok(settled.call_state(), "required test value"),
        CallApprovalState::Settled
    );
    assert_eq!(
        require_ok(settled.restore_call(), "required test value"),
        f.prepared
    );
    assert_eq!(
        require_ok(settled.call_held_digest(), "required test value"),
        digest
    );
    assert_eq!(
        require_ok(settled.call_budget(), "required test value"),
        reservation
    );
    assert_eq!(
        require_ok(budgets.held_exposure(f.limit), "required test value"),
        0
    );
    assert_eq!(
        require_ok(budgets.consumed(f.limit), "required test value"),
        FEE
    );
    require_ok(
        Program::settle_call(&mut store, &budgets, &f.peer, f.id(), &receipt),
        "required test value",
    );
    assert_eq!(
        require_ok(budgets.consumed(f.limit), "required test value"),
        FEE
    );
}

#[test]
fn program_carrier_legacy_refuses_changed_signed_call_and_recovered_sender() {
    let (f, mut store, budgets) = Fixture::new(true);
    let held = hold_program(&f, &mut store, &budgets);
    require_ok(
        Program::decide_call(
            &mut store,
            &budgets,
            &f.peer,
            f.id(),
            require_ok(held.call_held_digest(), "required test value"),
            [9; 32],
            true,
            6,
            1100,
        ),
        "required test value",
    );
    let mut altered = f.prepared.clone();
    altered.call.nonce += 1;
    altered.signing_hash = altered.call.signing_hash();
    altered.disclosure = require_ok(
        layerx_crypto::disclosure::call::bind_call(&altered.call, altered.from),
        "required test value",
    );
    altered.disclosure_digest =
        require_ok(altered.disclosure.signing_digest(), "required test value");
    let registry = require_ok(ModuleRegistry::new(&[]), "required test value");
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
    let (call, signature) = require_ok(
        layerx_crypto::evm_transaction::decode_signed(&require_ok(
            f.prepared.call.sign(&secret),
            "required test value",
        )),
        "required test value",
    );
    altered = f.prepared.clone();
    altered.from = require_ok(
        layerx_crypto::evm_transaction::recover_sender(&call, &signature),
        "required test value",
    );
    altered.disclosure = require_ok(
        layerx_crypto::disclosure::call::bind_call(&altered.call, altered.from),
        "required test value",
    );
    altered.disclosure_digest =
        require_ok(altered.disclosure.signing_digest(), "required test value");
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
        require_ok(
            Program::read_call(&store, &f.peer, f.id())
                .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
                .call_state(),
            "required test value"
        ),
        CallApprovalState::Granted
    );
    assert_eq!(
        require_ok(budgets.held_exposure(f.limit), "required test value"),
        FEE
    );
    assert!(store
        .list_object_ids(&f.tenant, ObjectKind::Outbox)
        .is_empty());
}

#[test]
fn program_carrier_legacy_recovers_signed_intent_after_real_outbox_refusal() {
    let (f, mut store, budgets) = Fixture::new(true);
    let held = hold_program(&f, &mut store, &budgets);
    require_ok(
        Program::decide_call(
            &mut store,
            &budgets,
            &f.peer,
            f.id(),
            require_ok(held.call_held_digest(), "required test value"),
            [10; 32],
            true,
            6,
            1100,
        ),
        "required test value",
    );
    let verified = signed(&f.prepared, 1);
    let hash = transaction_hash(verified.exact_bytes());
    let mut another_store = require_ok(
        Store::open(f.root.join("another-outbox")),
        "required test value",
    );
    let mut already_queued = Outbox::default();
    require_ok(
        already_queued.enqueue(
            &mut another_store,
            f.tenant.clone(),
            verified.exact_bytes().to_vec(),
        ),
        "required test value",
    );
    assert!(matches!(
        Program::queue_call(
            &mut store,
            &mut already_queued,
            &f.peer,
            f.id(),
            &verified,
            &require_ok(ModuleRegistry::new(&[]), "required test value"),
            6,
            1200
        ),
        Err(CallApprovalError::Store)
    ));
    assert_eq!(
        require_ok(
            Program::read_call(&store, &f.peer, f.id())
                .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
                .call_state(),
            "required test value"
        ),
        CallApprovalState::Signed
    );
    assert!(store
        .list_object_ids(&f.tenant, ObjectKind::Outbox)
        .is_empty());
    assert_eq!(
        require_ok(budgets.held_exposure(f.limit), "required test value"),
        FEE
    );
    drop(already_queued);
    drop(another_store);
    let (mut store, budgets) = f.restart(store, budgets);
    let signed = require_ok(
        Program::read_call(&store, &f.peer, f.id()),
        "required test value",
    );
    require_ok(signed.restore_call_budget(&budgets), "required test value");
    let mut outbox = Outbox::default();
    let recovered = require_ok(
        Program::recover_call_submission(&mut store, &mut outbox, &f.peer, f.id()),
        "required test value",
    );
    assert_eq!(
        require_ok(recovered.call_state(), "required test value"),
        CallApprovalState::Queued
    );
    assert_eq!(
        require_ok(recovered.restore_call(), "required test value"),
        f.prepared
    );
    assert_eq!(
        require_ok(outbox.bytes_for_transmission(hash), "required test value"),
        verified.exact_bytes()
    );
    assert_eq!(
        store.list_object_ids(&f.tenant, ObjectKind::Outbox),
        vec![hash.to_vec()]
    );
    require_ok(
        Program::recover_call_submission(&mut store, &mut outbox, &f.peer, f.id()),
        "required test value",
    );
    assert_eq!(
        store.list_object_ids(&f.tenant, ObjectKind::Outbox),
        vec![hash.to_vec()]
    );
}

#[test]
fn effect_carrier_legacy_failed_hold_does_not_publish_budget_or_carrier() {
    let (f, mut store, budgets) = Fixture::new(false);
    require_ok(
        std::fs::create_dir(f.root.join("store.bin.tmp")),
        "required test value",
    );
    let stage = require_ok(
        budgets.stage_program_reservation(&f.reservation(), CoreTimestampMs(1000)),
        "required test value",
    );
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
    assert!(!require_ok(
        budgets.has_reservation(f.id()),
        "required test value"
    ));
    assert!(store.get(&f.key(false)).is_none());
    require_ok(
        std::fs::remove_dir(f.root.join("store.bin.tmp")),
        "required test value",
    );
    let held = hold_effect(&f, &mut store, &budgets);
    assert_eq!(
        require_ok(held.call_state(), "required test value"),
        CallApprovalState::Awaiting
    );
    assert_eq!(
        require_ok(budgets.held_exposure(f.limit), "required test value"),
        FEE
    );
}

fn signed_receipt(
    prepared: &EvmPrepared,
    hash: [u8; 32],
    fee: u128,
    signer: &SigningKey,
) -> Vec<u8> {
    let unsigned = receipt(prepared, hash, fee, None);
    let signature = signer
        .sign(&require_ok(
            layerx_wire::hash::receipt_digest(&unsigned),
            "required test value",
        ))
        .to_bytes();
    let receipt_encoding = receipt(prepared, hash, fee, Some(signature));
    let decoded = require_ok(
        layerx_wire::receipt::decode(&receipt_encoding),
        "required test value",
    );
    assert_eq!(
        require_ok(
            layerx_wire::receipt::encode(&decoded),
            "required test value"
        ),
        receipt_encoding
    );
    receipt_encoding
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
    require_ok(summary.u64(50_000), "required test value");
    require_ok(summary.u64(50_000), "required test value");
    require_ok(summary.u64(3), "required test value");
    require_ok(summary.u32(0), "required test value");
    require_ok(summary.fixed(&[0; 20]), "required test value");
    require_ok(summary.fixed(&[0; 20]), "required test value");
    require_ok(
        summary.fixed(&<[u8; 32]>::from(Sha256::digest(b""))),
        "required test value",
    );
    require_ok(summary.u32(0), "required test value");
    require_ok(summary.u32(0), "required test value");
    require_ok(summary.u8(4), "required test value");
    require_ok(summary.fixed(b"uhpx"), "required test value");
    let mut out = Encoder::new(4096);
    require_ok(
        out.structure_header_version(0x5201, 2),
        "required test value",
    );
    require_ok(out.u16(2), "required test value");
    require_ok(out.bytes(&hash, 32), "required test value");
    require_ok(out.u64(7), "required test value");
    let previous = [2; 32];
    let next: [u8; 32] = Sha256::new()
        .chain_update(b"PAXEER/v1/sequence\0")
        .chain_update(previous)
        .chain_update(7_u64.to_be_bytes())
        .chain_update(hash)
        .finalize()
        .into();
    require_ok(out.bytes(&previous, 32), "required test value");
    require_ok(out.bytes(&next, 32), "required test value");
    require_ok(
        out.bytes(&<[u8; 32]>::from(Sha256::digest(b"")), 32),
        "required test value",
    );
    require_ok(out.i32(0), "required test value");
    require_ok(out.sequence_length(1, 512), "required test value");
    require_ok(out.u16(12), "required test value");
    require_ok(out.u16(0), "required test value");
    require_ok(out.u16(0), "required test value");
    require_ok(out.u8(1), "required test value");
    require_ok(out.u8(0), "required test value");
    require_ok(out.bytes(&[0; 32], 32), "required test value");
    require_ok(out.bytes(&summary.finish(), 256), "required test value");
    require_ok(out.u128(fee), "required test value");
    let mut height = [0; 32];
    height[31] = 6;
    require_ok(out.bytes(&height, 32), "required test value");
    require_ok(out.u16(12), "required test value");
    require_ok(out.u32(1), "required test value");
    require_ok(out.u32(0), "required test value");
    require_ok(out.u8(2), "required test value");
    require_ok(out.bytes(&asset(), 32), "required test value");
    require_ok(out.u128(0), "required test value");
    let mut from = [0; 32];
    from[12..].copy_from_slice(&prepared.from);
    require_ok(out.bytes(&from, 32), "required test value");
    require_ok(out.u128(1_000_000), "required test value");
    require_ok(out.u128(1_000_000 - fee), "required test value");
    require_ok(out.u64(prepared.call.nonce), "required test value");
    let mut to = [0; 32];
    to[12..].copy_from_slice(&prepared.call.to);
    require_ok(out.bytes(&to, 32), "required test value");
    require_ok(out.u128(0), "required test value");
    require_ok(out.u128(0), "required test value");
    require_ok(out.bytes(&[0; 32], 32), "required test value");
    require_ok(out.bytes(&[0; 32], 32), "required test value");
    require_ok(out.bytes(&[0x11; 32], 32), "required test value");
    require_ok(out.u64(1300), "required test value");
    require_ok(out.u8(u8::from(signature.is_some())), "required test value");
    if let Some(signature) = signature {
        require_ok(out.bytes(&signature, 64), "required test value");
    }
    out.finish()
}
