use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::TimestampSeconds;
use layerx_agentd::approval::native_program::NativeProgramApprovalCarrier as Program;
use layerx_agentd::approval::{CallApprovalError, CallApprovalState};
use layerx_agentd::budget::{
    create_daemon_limit, daemon_limit_id, load_daemon_limits, BudgetLimiter, CoreTimestampMs,
    DaemonLimitRecord, LimitId, ProgramBudgetCharge, ProgramChargeKind, ProgramReservationRequest,
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
            "repair-22-33-1550-{}-{}",
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

use layerx_agentd::approval::native_program::{CarrierError, StagedNativeRateAdvance};
use layerx_agentd::approval::native_program_presentation::{
    read_owned, ProgramPresentationError, ProgramPresentationOperation, ProgramPresentationState,
};
use layerx_agentd::approval::{
    ApprovalExpiry, ApprovalOutcome, ApprovalService, ApprovalSubmissionQueue, DecisionKey,
    DecisionRequest,
};
use layerx_agentd::budget::{reserve, ReservationRequest};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::policy::approval::{
    hold_at, hold_reserved_timed, ApprovalContext, ApprovalRegistry, ApprovalSequence,
    ApprovalState, ApproverId,
};
use layerx_agentd::protocol_evidence::{
    AuthenticatedTimeWindowUse, EvidenceAuthority, RawActivityReceiptEvidence, RawReceiptEvidence,
    RawTimeWindowBatch,
};
use layerx_agentd::read::{
    self, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator, CommitVote,
    ServedCheckpoint,
};
use layerx_agentd::session::SessionId;
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration};
use std::sync::{Arc, Mutex};

fn api_prepared(f: &Fixture) -> layerx_agent_api::prepare::Prepared {
    use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
    use layerx_agent_api::prepare::{
        CanonicalBytes, Disclosure, IdempotencyRef, PreparationRef, SigningPreimage,
    };
    use layerx_agent_api::{Amount, TimestampSeconds};
    let sender = hex::encode(f.prepared.from);
    layerx_agent_api::prepare::Prepared {
        preparation_ref: PreparationRef::new(hex::encode(f.id())).unwrap(),
        unsigned_canonical_bytes: CanonicalBytes::new(f.prepared.call.encode_unsigned()).unwrap(),
        signing_preimage: SigningPreimage::new(f.prepared.signing_hash.to_vec()).unwrap(),
        disclosure: Disclosure {
            canonical_digest: f.id(),
            activity_type: ActivityType(9),
            actor: AgentDid::new(sender.clone()).unwrap(),
            authority: AuthorityRef::new(sender).unwrap(),
            counterparties: ExplicitSet::deny_all(),
            amounts: ExplicitSet::deny_all(),
            asset: Asset::new("uhpx").unwrap(),
            fee_limit: Amount(FEE),
            expiry: TimestampSeconds(500),
            idempotency_key: IdempotencyRef::new(hex::encode(
                f.prepared.disclosure.idempotency_key,
            ))
            .unwrap(),
        },
        expiry: TimestampSeconds(500),
    }
}

fn approval_context(f: &Fixture) -> ApprovalContext {
    ApprovalContext {
        tenant: f.tenant.clone(),
        agent: Did::new(hex::encode(SENDER).as_bytes()).unwrap(),
        actor_address: SENDER,
        session: SessionId([3; 32]),
        capability: CapabilityId([4; 32]),
        policy_version: "repair-1550".into(),
        request_id: f.id(),
    }
}

fn grant(
    f: &Fixture,
    registry: &ApprovalRegistry,
    budgets: &BudgetLimiter,
    expiry: &ApprovalExpiry,
    queue: &ApprovalSubmissionQueue,
) -> Result<
    layerx_agentd::approval::ApprovalDecision,
    layerx_agentd::approval::ApprovalOperationError,
> {
    ApprovalService::new(registry, budgets, expiry).approve(
        DecisionRequest {
            tenant: &f.tenant,
            approval_id: f.id(),
            idempotency_key: &DecisionKey::new("grant-1550").unwrap(),
            approver: ApproverId::new("human:owner").unwrap(),
            current_sequence: 6,
            current_time: TimestampSeconds(6),
        },
        &api_prepared(f),
        queue,
    )
}

#[test]
fn grant_before_persist_failed_commit_is_invisible_and_restart_recovers_exact_release() {
    let (f, store, budgets) = Fixture::new(false);
    let shared = Arc::new(Mutex::new(store));
    let registry = ApprovalRegistry::with_store(shared.clone());
    let expiry = ApprovalExpiry::from_shared_store(shared.clone());
    let queue = ApprovalSubmissionQueue::default();
    let reservation = reserve(
        &budgets,
        &ReservationRequest {
            id: f.id(),
            amount: FEE,
            expiry_sequence: 50,
            current_sequence: 5,
            applicable_limits: vec![f.limit],
        },
    )
    .unwrap();
    hold_reserved_timed(
        &registry,
        approval_context(&f),
        api_prepared(&f),
        ApprovalSequence(5),
        ApprovalSequence(50),
        &reservation,
        TimestampSeconds(5),
    )
    .unwrap();
    ApprovalService::new(&registry, &budgets, &expiry)
        .get(&f.tenant, f.id(), 6, TimestampSeconds(6))
        .unwrap();
    let before = std::fs::read(f.root.join("store.bin")).unwrap();
    std::fs::create_dir(f.root.join("store.bin.tmp")).unwrap();
    assert!(grant(&f, &registry, &budgets, &expiry, &queue).is_err());
    assert!(queue.is_empty());
    assert_eq!(std::fs::read(f.root.join("store.bin")).unwrap(), before);
    assert_eq!(
        ApprovalService::new(&registry, &budgets, &expiry)
            .get(&f.tenant, f.id(), 6, TimestampSeconds(6))
            .unwrap()
            .state,
        ApprovalState::AwaitingApproval
    );
    assert_eq!(budgets.held_exposure(f.limit).unwrap(), FEE);
    std::fs::remove_dir(f.root.join("store.bin.tmp")).unwrap();
    drop(registry);
    drop(expiry);
    drop(queue);
    drop(shared);
    drop(budgets);

    let shared = Arc::new(Mutex::new(Store::open(&f.root).unwrap()));
    let budgets = BudgetLimiter::new(Vec::new()).unwrap();
    load_daemon_limits(&shared.lock().unwrap(), &budgets).unwrap();
    let registry = ApprovalRegistry::with_store(shared.clone());
    assert_eq!(registry.replay_tenant(&f.tenant, &budgets).unwrap(), 1);
    let expiry = ApprovalExpiry::from_shared_store(shared.clone());
    let queue = ApprovalSubmissionQueue::default();
    queue
        .recover_released(
            &registry,
            &f.tenant,
            &budgets,
            &ModuleRegistry::new(&[]).unwrap(),
        )
        .unwrap();
    assert!(queue.is_empty());
    let result = grant(&f, &registry, &budgets, &expiry, &queue).unwrap();
    assert_eq!(result.outcome, ApprovalOutcome::Granted);
    let reference = result.submission_ref.unwrap();
    assert_eq!(queue.prepared(reference), Some(api_prepared(&f)));
    assert_eq!(queue.len(), 1);
    drop(registry);
    drop(expiry);
    drop(queue);
    drop(shared);
    drop(budgets);

    let shared = Arc::new(Mutex::new(Store::open(&f.root).unwrap()));
    let budgets = BudgetLimiter::new(Vec::new()).unwrap();
    load_daemon_limits(&shared.lock().unwrap(), &budgets).unwrap();
    let registry = ApprovalRegistry::with_store(shared.clone());
    registry.replay_tenant(&f.tenant, &budgets).unwrap();
    let expiry = ApprovalExpiry::from_shared_store(shared);
    let queue = ApprovalSubmissionQueue::default();
    queue
        .recover_released(
            &registry,
            &f.tenant,
            &budgets,
            &ModuleRegistry::new(&[]).unwrap(),
        )
        .unwrap();
    assert_eq!(queue.prepared(reference), Some(api_prepared(&f)));
    assert_eq!(budgets.held_exposure(f.limit).unwrap(), FEE);
    assert_eq!(
        grant(&f, &registry, &budgets, &expiry, &queue).unwrap(),
        result
    );
    assert_eq!(queue.len(), 1);
}

#[test]
fn grant_before_persist_fallback_failure_keeps_claim_retryable() {
    let (f, store, budgets) = Fixture::new(false);
    drop(store);
    let registry = ApprovalRegistry::default();
    hold_at(
        &registry,
        approval_context(&f),
        api_prepared(&f),
        ApprovalSequence(5),
        ApprovalSequence(50),
        TimestampSeconds(5),
    )
    .unwrap();
    let expiry = ApprovalExpiry::open(&f.root).unwrap();
    let queue = ApprovalSubmissionQueue::default();
    ApprovalService::new(&registry, &budgets, &expiry)
        .get(&f.tenant, f.id(), 6, TimestampSeconds(6))
        .unwrap();
    std::fs::create_dir(f.root.join("store.bin.tmp")).unwrap();
    assert!(grant(&f, &registry, &budgets, &expiry, &queue).is_err());
    assert!(queue.is_empty());
    assert_eq!(
        ApprovalService::new(&registry, &budgets, &expiry)
            .get(&f.tenant, f.id(), 6, TimestampSeconds(6))
            .unwrap()
            .state,
        ApprovalState::AwaitingApproval
    );
    std::fs::remove_dir(f.root.join("store.bin.tmp")).unwrap();
    let result = grant(&f, &registry, &budgets, &expiry, &queue).unwrap();
    assert_eq!(result.outcome, ApprovalOutcome::Granted);
    assert_eq!(
        queue.prepared(result.submission_ref.unwrap()),
        Some(api_prepared(&f))
    );
}

#[test]
fn program_presentation_legacy_matches_reopened_approved_and_submitted_call() {
    let (f, mut store, budgets) = Fixture::new(true);
    let held = hold_program(&f, &mut store, &budgets);
    let digest = held.call_held_digest().unwrap();
    let modules = ModuleRegistry::new(&[]).unwrap();
    let view = read_owned(&store, &f.peer, f.id(), &modules).unwrap();
    assert_eq!(view.state(), ProgramPresentationState::Awaiting);
    assert_eq!(view.held_digest(), digest);
    assert_eq!(view.actor(), SENDER);
    assert_eq!(view.call(), Some(&f.prepared.call));
    assert_eq!(
        view.canonical_unsigned_bytes(),
        f.prepared.call.encode_unsigned()
    );
    assert_eq!(
        view.reservation().charges[0].amount,
        u128::from(f.prepared.call.gas_limit) * f.prepared.call.max_fee_per_gas
    );
    assert_eq!(view.fee_asset(), Some(asset()));
    assert_eq!(view.activity_expires_at_unix_milliseconds(), DEADLINE);
    let disclosed = match &f.prepared.disclosure.native_operation {
        Some(layerx_crypto::disclosure::DisclosedNativeOperation::ProgramCall(value)) => {
            (**value).clone()
        }
        _ => panic!("program producer did not produce a program call"),
    };
    assert_eq!(
        view.operation(),
        &ProgramPresentationOperation::Call(disclosed)
    );
    let immutable = view.immutable_carrier_bytes().to_vec();
    let (mut store, budgets) = f.restart(store, budgets);
    Program::read_call(&store, &f.peer, f.id())
        .unwrap()
        .restore_call_budget(&budgets)
        .unwrap();
    Program::decide_call(
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
    let verified = signed(&f.prepared, 1);
    let mut outbox = Outbox::default();
    Program::queue_call(
        &mut store,
        &mut outbox,
        &f.peer,
        f.id(),
        &verified,
        &modules,
        7,
        1200,
    )
    .unwrap();
    let view = read_owned(&store, &f.peer, f.id(), &modules).unwrap();
    assert_eq!(view.state(), ProgramPresentationState::Granted);
    assert_eq!(view.immutable_carrier_bytes(), immutable);
    assert_eq!(view.held_digest(), digest);
    let actual = outbox.exact_signed_bytes(verified.activity_id()).unwrap();
    let (call, signature) = layerx_crypto::evm_transaction::decode_signed(actual).unwrap();
    assert_eq!(view.call(), Some(&call));
    assert_eq!(view.canonical_unsigned_bytes(), call.encode_unsigned());
    assert_eq!(
        view.actor(),
        layerx_crypto::evm_transaction::recover_sender(&call, &signature).unwrap()
    );
    let (mut store, _) = f.restart(store, budgets);
    let mut outbox = Outbox::default();
    Program::recover_call_submission(&mut store, &mut outbox, &f.peer, f.id()).unwrap();
    assert_eq!(
        read_owned(&store, &f.peer, f.id(), &modules)
            .unwrap()
            .immutable_carrier_bytes(),
        immutable
    );
    assert_eq!(
        outbox.exact_signed_bytes(verified.activity_id()).unwrap(),
        verified.exact_bytes()
    );
}

#[test]
fn program_presentation_legacy_refuses_fee_allocation_from_another_sender() {
    let (f, mut store, budgets) = Fixture::new(true);
    let mut request = f.reservation();
    request.charges[0].source = [9; 32];
    let stage = budgets
        .stage_program_reservation(&request, CoreTimestampMs(1000))
        .unwrap();
    Program::hold_call(
        &mut store,
        &f.peer,
        [3; 32],
        1,
        &f.prepared,
        stage,
        1000,
        &keys(),
    )
    .unwrap();
    assert!(matches!(
        read_owned(&store, &f.peer, f.id(), &ModuleRegistry::new(&[]).unwrap()),
        Err(ProgramPresentationError::Binding)
    ));
}

#[test]
fn program_requirement_legacy_replay_refuses_changed_call_sender_and_decision() {
    let (f, mut store, budgets) = Fixture::new(true);
    let held = hold_program(&f, &mut store, &budgets);
    let digest = held.call_held_digest().unwrap();
    Program::authorize_call_requirement(&store, &f.peer, f.id(), &f.prepared).unwrap();
    let (mut store, budgets) = f.restart(store, budgets);
    Program::read_call(&store, &f.peer, f.id())
        .unwrap()
        .restore_call_budget(&budgets)
        .unwrap();
    Program::decide_call(
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
    let (mut store, _) = f.restart(store, budgets);
    Program::authorize_call_requirement(&store, &f.peer, f.id(), &f.prepared).unwrap();
    for change in 0..3 {
        let mut altered = f.prepared.clone();
        match change {
            0 => altered.call.nonce += 1,
            1 => altered.from = [8; 20],
            _ => altered.call.max_fee_per_gas += 1,
        }
        altered.disclosure =
            layerx_crypto::disclosure::call::bind_call(&altered.call, altered.from).unwrap();
        altered.disclosure_digest = altered.disclosure.signing_digest().unwrap();
        altered.signing_hash = altered.call.signing_hash();
        verify_call_binding(&altered).unwrap();
        assert_eq!(
            Program::authorize_call_requirement(&store, &f.peer, f.id(), &altered),
            Err(CallApprovalError::Binding)
        );
    }
    let key = TenantKey::new(
        f.tenant.clone(),
        ObjectKind::Configuration,
        [b"program-required-call-v2:".as_slice(), f.id().as_slice()].concat(),
    )
    .unwrap();
    let original = store.get(&key).unwrap().bytes().to_vec();
    let mut record: serde_json::Value = serde_json::from_slice(&original).unwrap();
    record["decision"]["grant"] = serde_json::json!(false);
    store
        .put_local(key, serde_json::to_vec(&record).unwrap())
        .unwrap();
    drop(store);
    let store = Store::open(&f.root).unwrap();
    assert!(matches!(
        Program::read_call(&store, &f.peer, f.id()),
        Err(CallApprovalError::Binding)
    ));
    assert!(Program::authorize_call_requirement(&store, &f.peer, f.id(), &f.prepared).is_err());
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

fn rate_evidence() -> Vec<AuthenticatedTimeWindowUse> {
    let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../tests/fixtures/custody/daemon-light-credit-receipt");
    let read = |name: &str| std::fs::read(root.join(name)).unwrap();
    let header_bytes = read("header");
    let header = layerx_wire::receipt::decode_batch_header(&header_bytes).unwrap();
    let signature = read("header.signature").try_into().unwrap();
    let authority = EvidenceAuthority::pinned_to_handshake(
        header.protocol_version(),
        header.network_id(),
        read("sequencer.public").try_into().unwrap(),
    )
    .unwrap();
    let checkpoint = committed_checkpoint(
        header.last_sequence(),
        header.timestamp_ms(),
        header.resulting_state_root(),
        &header.network_id().to_string(),
    );
    let authority = authority
        .pin_committed_validators(
            checkpoint.header.chain_id.clone(),
            checkpoint.validators.clone(),
        )
        .unwrap();
    let observed = authority.authenticate_committed_time(&checkpoint).unwrap();
    let kind = ActivityType::new(ModuleId::Bridge, 1).unwrap();
    let registration = ModuleRegistration::new(ModuleId::Bridge, &[kind]).unwrap();
    let registry = ModuleRegistry::new(&[registration]).unwrap();
    let activity = read("activity");
    layerx_wire::activity::decode_signed(&activity, &registry).unwrap();
    let receipt = RawReceiptEvidence::new(
        read("credit.receipt"),
        layerx_proof::merkle::decode_proof(&read("receipt.proof")).unwrap(),
        header_bytes.clone(),
        signature,
    );
    let maintenance = RawReceiptEvidence::new(
        read("maintenance.receipt"),
        layerx_proof::merkle::decode_proof(&read("maintenance.proof")).unwrap(),
        header_bytes,
        signature,
    );
    let batch = RawTimeWindowBatch::new(
        vec![RawActivityReceiptEvidence::from_signed_inclusion(
            activity, receipt,
        )],
        maintenance,
    );
    [1, 10]
        .into_iter()
        .map(|seconds| {
            authority
                .authenticate_cumulative_time_use(
                    &Did::new(&SENDER).unwrap(),
                    seconds,
                    &observed,
                    std::slice::from_ref(&batch),
                )
                .unwrap()
        })
        .collect()
}

fn rate_fixture(
    f: &Fixture,
    index: u64,
    admitted: u64,
    committed: Option<u64>,
    actor: &[u8],
) -> (TenantKey, Vec<u8>) {
    let mut call = f.prepared.call.clone();
    call.nonce = index;
    let canonical = call.encode_unsigned();
    let id: [u8; 32] = Sha256::digest(&canonical).into();
    let mut secret = [0; 32];
    secret[31] = 1;
    let activity = transaction_hash(&call.sign(&secret).unwrap());
    let record = serde_json::json!({
        "version": 1, "tenant": f.tenant.as_str(), "actor": actor, "preparation": id,
        "canonical_bytes": canonical, "chain": vec![[9u8; 32]], "observed_head_sequence": 0,
        "admitted_at_ms": admitted, "activity_id": committed.map(|_| activity),
        "committed": committed.map(|timestamp| (1u64, timestamp)),
    });
    // Field order is the persisted codec's order, not serde_json::Value's sorted map order.
    #[derive(serde::Serialize)]
    struct Snapshot {
        version: u8,
        tenant: String,
        actor: Vec<u8>,
        preparation: [u8; 32],
        canonical_bytes: Vec<u8>,
        chain: Vec<[u8; 32]>,
        observed_head_sequence: u64,
        admitted_at_ms: u64,
        activity_id: Option<[u8; 32]>,
        committed: Option<(u64, u64)>,
    }
    let snapshot = Snapshot {
        version: 1,
        tenant: f.tenant.as_str().into(),
        actor: actor.to_vec(),
        preparation: id,
        canonical_bytes: call.encode_unsigned(),
        chain: vec![[9; 32]],
        observed_head_sequence: 0,
        admitted_at_ms: admitted,
        activity_id: committed.map(|_| activity),
        committed: committed.map(|timestamp| (1, timestamp)),
    };
    let bytes = serde_json::to_vec(&snapshot).unwrap();
    assert_eq!(
        serde_json::from_slice::<serde_json::Value>(&bytes).unwrap(),
        record
    );
    (
        TenantKey::new(
            f.tenant.clone(),
            ObjectKind::PreparedActivity,
            [b"native-program-rate-v1:".as_slice(), id.as_slice()].concat(),
        )
        .unwrap(),
        bytes,
    )
}

fn active_rate_count(store: &Store, tenant: &TenantId) -> usize {
    store
        .list_object_ids(tenant, ObjectKind::PreparedActivity)
        .iter()
        .filter(|key| key.starts_with(b"native-program-rate-v1:"))
        .count()
}
fn archived_rate_count(store: &Store, tenant: &TenantId) -> usize {
    store
        .list_object_ids(tenant, ObjectKind::PreparedActivity)
        .iter()
        .filter(|key| key.starts_with(b"native-program-rate-committed-v1:"))
        .count()
}

#[test]
fn rate_lifetime_cap_more_than_4096_completed_uses_preserve_pending_and_replay_on_reopen() {
    let (f, mut store, budgets) = Fixture::new(true);
    let evidence = rate_evidence();
    let now = evidence[0].observed_core_ms();
    let old = now.checked_sub(11_000).unwrap();
    let records = (0..4093)
        .map(|index| rate_fixture(&f, index, old - 1, Some(old), &SENDER))
        .chain([
            rate_fixture(&f, 4093, old - 1, None, &SENDER),
            rate_fixture(&f, 4094, old - 1, Some(now - 5_000), &SENDER),
            rate_fixture(&f, 4095, old - 1, Some(old), &[7; 20]),
        ])
        .collect::<Vec<_>>();
    let preserved = records[4093..].to_vec();
    store.commit_local_records(records).unwrap();
    assert_eq!(active_rate_count(&store, &f.tenant), 4096);
    let staged = StagedNativeRateAdvance::stage(&store, &f.tenant, &evidence).unwrap();
    assert_eq!(staged.remaining(), 3);
    let admission = rate_fixture(&f, 4096, old - 1, Some(old), &SENDER);
    let before = std::fs::read(f.root.join("store.bin")).unwrap();
    std::fs::create_dir(f.root.join("store.bin.tmp")).unwrap();
    assert_eq!(
        staged.commit_joined(&mut store, Vec::new(), vec![admission.clone()]),
        Err(CarrierError::Corrupt)
    );
    assert_eq!(active_rate_count(&store, &f.tenant), 4096);
    assert_eq!(archived_rate_count(&store, &f.tenant), 0);
    assert!(store.get(&admission.0).is_none());
    assert_eq!(std::fs::read(f.root.join("store.bin")).unwrap(), before);
    std::fs::remove_dir(f.root.join("store.bin.tmp")).unwrap();
    let (mut store, _) = f.restart(store, budgets);
    assert_eq!(active_rate_count(&store, &f.tenant), 4096);
    let inserts = (4096..4112)
        .map(|index| rate_fixture(&f, index, old - 1, Some(old), &SENDER))
        .collect();
    StagedNativeRateAdvance::stage(&store, &f.tenant, &evidence)
        .unwrap()
        .commit_joined(&mut store, Vec::new(), inserts)
        .unwrap();
    assert_eq!(active_rate_count(&store, &f.tenant), 19);
    assert_eq!(archived_rate_count(&store, &f.tenant), 4093);
    let staged = StagedNativeRateAdvance::stage(&store, &f.tenant, &evidence).unwrap();
    assert_eq!(staged.remaining(), 3);
    staged
        .commit_joined(&mut store, Vec::new(), Vec::new())
        .unwrap();
    drop(store);
    let store = Store::open(&f.root).unwrap();
    assert_eq!(active_rate_count(&store, &f.tenant), 3);
    assert_eq!(archived_rate_count(&store, &f.tenant), 4109);
    for (key, bytes) in preserved {
        assert_eq!(store.get(&key).unwrap().bytes(), bytes);
    }
    let (old_key, old_bytes) = rate_fixture(&f, 0, old - 1, Some(old), &SENDER);
    let archive = TenantKey::new(
        f.tenant.clone(),
        ObjectKind::PreparedActivity,
        [
            b"native-program-rate-committed-v1:".as_slice(),
            &old_key.object_id()[b"native-program-rate-v1:".len()..],
        ]
        .concat(),
    )
    .unwrap();
    assert_eq!(store.get(&archive).unwrap().bytes(), old_bytes);
    assert!(StagedNativeRateAdvance::stage(&store, &f.tenant, &[]).is_err());
}

#[test]
fn program_requirement_legacy_failed_decision_keeps_recovered_requirement_awaiting() {
    let (f, mut store, budgets) = Fixture::new(true);
    let held = hold_program(&f, &mut store, &budgets);
    let digest = held.call_held_digest().unwrap();
    let before = std::fs::read(f.root.join("store.bin")).unwrap();
    std::fs::create_dir(f.root.join("store.bin.tmp")).unwrap();
    assert!(matches!(
        Program::decide_call(
            &mut store,
            &budgets,
            &f.peer,
            f.id(),
            digest,
            [4; 32],
            true,
            6,
            1100
        ),
        Err(CallApprovalError::Store)
    ));
    assert_eq!(std::fs::read(f.root.join("store.bin")).unwrap(), before);
    std::fs::remove_dir(f.root.join("store.bin.tmp")).unwrap();
    let (store, _) = f.restart(store, budgets);
    Program::authorize_call_requirement(&store, &f.peer, f.id(), &f.prepared).unwrap();
    assert_eq!(
        Program::read_call(&store, &f.peer, f.id())
            .unwrap()
            .call_state()
            .unwrap(),
        CallApprovalState::Awaiting
    );
}
