use std::collections::BTreeSet;
use std::fs;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};

use layerx_agentd::budget::{
    reserve, BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest,
};
use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::prepare::{
    expire_elapsed, prepare_bound, sweep_retention, verify_call_binding, EvmPreparationDefaults,
    EvmPreparationState, EvmPrepareRequest, EvmPrepared, LifecycleError, LifecycleState,
    PrecompileCall, PreparationLifecycle,
};
use layerx_agentd::session::{open, OpenRequest, SessionId, SessionRef, SessionRegistry, Token};
use layerx_agentd::session_control::{OperationPermit, SessionControl, SessionControlError};
use layerx_agentd::sign::{verify_before_submit, SigningPreparation};
use layerx_agentd::store::{Store, TenantId};
use layerx_agentd::tenant::{AuthorizationError, Operation, Surface};
use layerx_crypto::disclosure::call::bind_call;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::verify::VerificationLevel;
use layerx_wire::encode::Encoder;

const OBSERVED_HEAD_SEQUENCE: u64 = 88;
const NOT_AFTER_MS: u64 = 1_010;

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

struct BoundaryIdentity(CoreIdentity);

impl IdentityResolver for BoundaryIdentity {
    fn resolve(&mut self, _did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(self.0.clone()))
    }
}

fn text<T, E: std::fmt::Debug>(result: Result<T, E>, label: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{label} must be valid: {error:?}"),
    }
}

fn test_directory(name: &str) -> PathBuf {
    let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-agentd-w7-core-time-{name}-{}-{sequence}",
        std::process::id()
    ))
}

fn activity_type() -> ActivityType {
    text(ActivityType::new(ModuleId::Asset, 5), "activity")
}

fn registry() -> ModuleRegistry {
    text(
        ModuleRegistry::new(&[text(
            ModuleRegistration::new(ModuleId::Asset, &[activity_type()]),
            "registration",
        )]),
        "registry",
    )
}

fn send_payload() -> Vec<u8> {
    let mut encoder = Encoder::new(512);
    text(encoder.u16(0x5301), "tag");
    text(encoder.u16(10), "fields");
    text(encoder.fixed(&[0x11; 32]), "from");
    text(encoder.fixed(&[0x22; 32]), "to");
    text(encoder.fixed(&[0x33; 32]), "asset");
    text(encoder.u128(25), "amount");
    text(encoder.u64(5), "sequence");
    text(encoder.fixed(&[4; 32]), "idempotency");
    text(encoder.u64(NOT_AFTER_MS), "expiry");
    text(encoder.fixed(&[0x55; 32]), "context");
    text(encoder.u8(0), "conditions");
    text(encoder.u8(1), "authority kind");
    text(encoder.fixed(&[0x11; 32]), "controller");
    text(encoder.fixed(&[0x66; 32]), "payload key");
    text(encoder.fixed(&[0x77; 64]), "payload signature");
    text(encoder.fixed(&[0x55; 32]), "signed context");
    text(encoder.u32(17), "network");
    text(
        encoder.u16(layerx_wire::limits::PROTOCOL_VERSION),
        "version",
    );
    crate::send_authorization::sign(encoder.finish())
}

const SIGNING_SEED: [u8; 32] = [0xa5; 32];

fn evm_prepared() -> EvmPrepared {
    let key = text(k256::ecdsa::SigningKey::from_slice(&SIGNING_SEED), "key");
    let from = text(
        layerx_crypto::secp256k1::evm_address(
            key.verifying_key().to_encoded_point(false).as_bytes(),
        ),
        "address",
    );
    let prepared = text(
        prepare_bound(
            EvmPreparationState {
                chain_id: 17,
                pending_nonce: 5,
                base_fee_per_gas: 3,
                head_height: OBSERVED_HEAD_SEQUENCE,
            },
            EvmPreparationDefaults {
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 7,
                maximum_payload_bytes: 1_024,
            },
            EvmPrepareRequest {
                from,
                call: PrecompileCall::ledger_transfer(send_payload()),
                expected_nonce: Some(5),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: 1_024,
            },
            100_000,
        ),
        "EIP-1559 prepare",
    );
    text(verify_call_binding(&prepared), "call binding");
    assert_eq!(
        text(bind_call(&prepared.call, from), "disclosure"),
        prepared.disclosure
    );
    assert_eq!(prepared.disclosure.expiry.payload_expires_at, NOT_AFTER_MS);
    assert_eq!(prepared.call.nonce, 5);
    assert_eq!(prepared.observed_head_height, OBSERVED_HEAD_SEQUENCE);
    prepared
}

fn prepared() -> EvmPrepared {
    evm_prepared()
}

fn signed_bytes() -> Vec<u8> {
    let transaction = evm_prepared();
    let actor_address = transaction.from;
    let prepared = SigningPreparation {
        transaction,
        actor_address,
    };
    let signed = text(
        prepared.transaction.call.sign(&SIGNING_SEED),
        "sign ledger call",
    );
    let verified = text(
        verify_before_submit(&signed, &prepared, &actor_address, &registry()),
        "verify submission",
    );
    assert_eq!(verified.exact_bytes(), signed);
    let (call, signature) = text(decode_signed(&signed), "decode signed call");
    assert_eq!(
        text(recover_sender(&call, &signature), "sender"),
        actor_address
    );
    signed
}

fn signed_hash() -> [u8; 32] {
    transaction_hash(&signed_bytes())
}

fn limiter() -> BudgetLimiter {
    text(
        BudgetLimiter::new(vec![LimitConfig {
            id: LimitId([1; 16]),
            name: "tenant-limit".to_owned(),
            scope: LimitScope::Tenant([1; 32]),
            ceiling: 1_000,
            consumed: 0,
        }]),
        "limiter",
    )
}

fn tenant() -> TenantId {
    text(TenantId::new("tenant-a"), "tenant")
}

fn opened_session(store: &mut Store, sessions: &mut SessionRegistry) -> Token {
    let mut boundary = BoundaryIdentity(CoreIdentity {
        canonical_bytes: b"w7-core-time-identity".to_vec(),
        head_sequence: 10,
        revocation_sequence: 1,
        verification_level: VerificationLevel::STATE_PROVEN,
        frozen: false,
        authorities: vec![ProtocolAuthority::SessionKey([4; 32])],
    });
    let identity = text(
        register(
            store,
            tenant(),
            text(Did::new(b"agent-a"), "agent DID"),
            &mut boundary,
        ),
        "identity",
    );
    text(
        open(
            store,
            sessions,
            &identity,
            OpenRequest {
                session_id: SessionId([1; 32]),
                token_id: [2; 32],
                tenant: tenant(),
                agent: text(Did::new(b"agent-a"), "agent DID"),
                authority: ProtocolAuthority::SessionKey([4; 32]),
                permitted_activity_types: BTreeSet::from([5]),
                scopes: BTreeSet::from(["prepare".to_owned(), "write".to_owned()]),
                expiry_sequence: 1_000,
                expiry_seconds: None,
                opening_client: "w7-core-time-suite".to_owned(),
                policy_version: "policy-v1".to_owned(),
            },
            10,
        ),
        "session",
    )
}

struct Harness {
    root: PathBuf,
    control: SessionControl,
    lifecycle: Arc<PreparationLifecycle>,
    token: Token,
}

impl Harness {
    fn new(name: &str) -> Self {
        let root = test_directory(name);
        let mut store = text(Store::open(&root), "session store");
        let mut sessions = SessionRegistry::default();
        let token = opened_session(&mut store, &mut sessions);
        let lifecycle = Arc::new(PreparationLifecycle::default());
        let control = SessionControl::new(
            Arc::new(Mutex::new(store)),
            sessions,
            Arc::clone(&lifecycle),
            Arc::new(limiter()),
        );
        Self {
            root,
            control,
            lifecycle,
            token,
        }
    }

    fn permit(&self, operation: Operation) -> OperationPermit {
        text(
            self.control.authorize(
                &self.token.credential(),
                operation,
                Surface::Contract,
                OBSERVED_HEAD_SEQUENCE,
                None,
            ),
            "permit",
        )
    }

    fn register(&self, preparation_id: [u8; 32]) {
        text(
            self.permit(Operation::Prepare).register_preparation(
                &self.control,
                preparation_id,
                &prepared(),
                Vec::new(),
            ),
            "register",
        );
    }
}

impl Drop for Harness {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

#[test]
fn expiry_predicate_admits_equality_and_refuses_one_millisecond_past_not_after() {
    let lifecycle = PreparationLifecycle::default();
    text(
        lifecycle.register([1; 32], &prepared(), Vec::new()),
        "register",
    );
    assert_eq!(lifecycle.check_unexpired([1; 32], NOT_AFTER_MS - 1), Ok(()));
    assert_eq!(lifecycle.check_unexpired([1; 32], NOT_AFTER_MS), Ok(()));
    assert_eq!(
        lifecycle.check_unexpired([1; 32], NOT_AFTER_MS + 1),
        Err(LifecycleError::PreparationExpired)
    );
    assert_eq!(lifecycle.state([1; 32]), Ok(LifecycleState::Prepared));
    assert_eq!(
        lifecycle.check_unexpired([9; 32], NOT_AFTER_MS),
        Err(LifecycleError::NotFound)
    );

    text(
        lifecycle.transition([1; 32], LifecycleState::Signing, OBSERVED_HEAD_SEQUENCE),
        "signing",
    );
    text(
        lifecycle.retain_signed_bytes([1; 32], signed_bytes(), signed_hash()),
        "signed",
    );
    assert_eq!(lifecycle.admit_submission([1; 32], NOT_AFTER_MS), Ok(()));
    assert_eq!(
        lifecycle.admit_submission([1; 32], NOT_AFTER_MS + 1),
        Err(LifecycleError::PreparationExpired)
    );
}

#[test]
fn admit_signing_requires_the_sign_operation_and_the_exact_bound_preparation() {
    let harness = Harness::new("sign-operation");
    harness.register([2; 32]);

    assert!(matches!(
        harness
            .permit(Operation::Prepare)
            .admit_signing(&harness.control, [2; 32], NOT_AFTER_MS),
        Err(SessionControlError::Authorization(
            AuthorizationError::ScopeDenied
        ))
    ));
    assert!(matches!(
        harness
            .permit(Operation::Submit)
            .admit_signing(&harness.control, [2; 32], NOT_AFTER_MS),
        Err(SessionControlError::Authorization(
            AuthorizationError::ScopeDenied
        ))
    ));
    assert_eq!(
        harness.lifecycle.check_unexpired([2; 32], NOT_AFTER_MS),
        Err(LifecycleError::AuthorizationRequired)
    );

    let sign = harness.permit(Operation::Sign);
    assert!(matches!(
        sign.admit_signing(&harness.control, [2; 32], NOT_AFTER_MS),
        Ok(())
    ));
    assert!(matches!(
        sign.admit_signing(&harness.control, [3; 32], NOT_AFTER_MS),
        Err(SessionControlError::Lifecycle(LifecycleError::NotFound))
    ));
    assert_eq!(
        harness.lifecycle.state([2; 32]),
        Ok(LifecycleState::Prepared)
    );
    assert_eq!(harness.lifecycle.has_signed_bytes([2; 32]), Ok(false));
}

#[test]
fn expired_preparation_is_refused_before_any_signing_mutation() {
    let harness = Harness::new("sign-expired");
    harness.register([4; 32]);
    let sign = harness.permit(Operation::Sign);
    text(
        sign.transition_preparation(
            &harness.control,
            [4; 32],
            LifecycleState::Signing,
            OBSERVED_HEAD_SEQUENCE,
        ),
        "signing",
    );

    assert!(matches!(
        sign.admit_signing(&harness.control, [4; 32], NOT_AFTER_MS + 1),
        Err(SessionControlError::Lifecycle(
            LifecycleError::PreparationExpired
        ))
    ));
    assert_eq!(
        harness.lifecycle.state([4; 32]),
        Ok(LifecycleState::Signing)
    );
    assert_eq!(harness.lifecycle.has_signed_bytes([4; 32]), Ok(false));

    assert!(matches!(
        sign.admit_signing(&harness.control, [4; 32], NOT_AFTER_MS),
        Ok(())
    ));
    text(
        sign.retain_signed_bytes(&harness.control, [4; 32], signed_bytes(), signed_hash()),
        "retain",
    );
    assert_eq!(harness.lifecycle.state([4; 32]), Ok(LifecycleState::Signed));
    assert_eq!(harness.lifecycle.has_signed_bytes([4; 32]), Ok(true));
}

#[test]
fn external_submission_checks_time_before_mutation_and_keeps_sequences_as_sequences() {
    let harness = Harness::new("external-submit");
    harness.register([5; 32]);
    let submit = harness.permit(Operation::Submit);

    assert!(matches!(
        submit.submit_with_external_signature(
            &harness.control,
            [5; 32],
            signed_bytes(),
            signed_hash(),
            OBSERVED_HEAD_SEQUENCE,
            NOT_AFTER_MS + 1,
        ),
        Err(SessionControlError::Lifecycle(
            LifecycleError::PreparationExpired
        ))
    ));
    assert_eq!(
        harness.lifecycle.state([5; 32]),
        Ok(LifecycleState::Prepared)
    );
    assert_eq!(harness.lifecycle.has_signed_bytes([5; 32]), Ok(false));

    assert!(matches!(
        submit.submit_with_external_signature(
            &harness.control,
            [5; 32],
            signed_bytes(),
            signed_hash(),
            OBSERVED_HEAD_SEQUENCE,
            NOT_AFTER_MS,
        ),
        Ok(())
    ));
    assert_eq!(harness.lifecycle.state([5; 32]), Ok(LifecycleState::Signed));
    assert_eq!(harness.lifecycle.has_signed_bytes([5; 32]), Ok(true));

    text(
        submit.transition_preparation(
            &harness.control,
            [5; 32],
            LifecycleState::Submitted,
            OBSERVED_HEAD_SEQUENCE + 1,
        ),
        "submitted",
    );
    text(
        harness.permit(Operation::Track).transition_preparation(
            &harness.control,
            [5; 32],
            LifecycleState::Failed,
            OBSERVED_HEAD_SEQUENCE + 2,
        ),
        "failed",
    );

    let early = text(
        sweep_retention(&harness.lifecycle, OBSERVED_HEAD_SEQUENCE + 6, 5),
        "early sweep",
    );
    assert_eq!(early.discarded_terminal_signed_bytes, 0);
    assert_eq!(harness.lifecycle.has_signed_bytes([5; 32]), Ok(true));
    let due = text(
        sweep_retention(&harness.lifecycle, OBSERVED_HEAD_SEQUENCE + 7, 5),
        "due sweep",
    );
    assert_eq!(due.discarded_terminal_signed_bytes, 1);
    assert_eq!(harness.lifecycle.has_signed_bytes([5; 32]), Ok(false));
}

#[test]
fn expiry_sweep_decides_on_milliseconds_and_records_only_sequences() {
    const CURRENT_SEQUENCE: u64 = 5_000;
    let limiter = limiter();
    text(
        reserve(
            &limiter,
            &ReservationRequest {
                id: [6; 32],
                amount: 100,
                expiry_sequence: 3_000,
                current_sequence: 2_000,
                applicable_limits: vec![LimitId([1; 16])],
            },
        ),
        "reserve",
    );
    let lifecycle = PreparationLifecycle::default();
    text(
        lifecycle.register([6; 32], &prepared(), vec![[6; 32]]),
        "register",
    );
    text(
        lifecycle.transition([6; 32], LifecycleState::Signing, CURRENT_SEQUENCE - 10),
        "signing",
    );
    text(
        lifecycle.retain_signed_bytes([6; 32], signed_bytes(), signed_hash()),
        "signed",
    );

    let unexpired = text(
        expire_elapsed(&lifecycle, &limiter, NOT_AFTER_MS, CURRENT_SEQUENCE),
        "equality sweep",
    );
    assert!(unexpired.expired_preparations.is_empty());
    assert!(unexpired.released_reservations.is_empty());
    assert_eq!(lifecycle.state([6; 32]), Ok(LifecycleState::Signed));
    assert_eq!(limiter.held_reservations(), Ok(1));

    let expired = text(
        expire_elapsed(&lifecycle, &limiter, NOT_AFTER_MS + 1, CURRENT_SEQUENCE),
        "elapsed sweep",
    );
    assert_eq!(expired.expired_preparations, vec![[6; 32]]);
    assert_eq!(expired.released_reservations, vec![[6; 32]]);
    assert_eq!(limiter.held_reservations(), Ok(0));
    assert_eq!(lifecycle.state([6; 32]), Ok(LifecycleState::Expired));

    let early = text(
        sweep_retention(&lifecycle, CURRENT_SEQUENCE + 4, 5),
        "early sweep",
    );
    assert_eq!(early.discarded_terminal_signed_bytes, 0);
    assert_eq!(lifecycle.has_signed_bytes([6; 32]), Ok(true));
    let due = text(
        sweep_retention(&lifecycle, CURRENT_SEQUENCE + 5, 5),
        "due sweep",
    );
    assert_eq!(due.discarded_terminal_signed_bytes, 1);
    assert_eq!(lifecycle.has_signed_bytes([6; 32]), Ok(false));
}

#[test]
fn retained_evm_call_reopens_with_exact_expiry_bytes_and_reservations() {
    const GENERATION: u64 = 1;
    const CURRENT_SEQUENCE: u64 = 5_000;
    let root = test_directory("retained-evm-call");
    let owner = SessionRef::new(tenant(), SessionId([7; 32]));
    let id = [7; 32];
    let (limiter, prepared, raw) = retain_evm_call(&root, &owner, GENERATION, id, CURRENT_SEQUENCE);
    let mut store = text(Store::open(&root), "reopen call store");
    let lifecycle = PreparationLifecycle::default();
    assert_restored_evm_call(&store, &lifecycle, &owner, GENERATION, id, &prepared, &raw);
    let key = text(
        layerx_agentd::prepare::DurablePreparation::store_key(&owner.tenant, id),
        "call key",
    );
    let before = store
        .get(&key)
        .unwrap_or_else(|| panic!("test value missing"))
        .bytes()
        .to_vec();
    assert_eq!(
        lifecycle.admit_call_submission(&mut store, &owner, GENERATION, id, NOT_AFTER_MS + 1),
        Err(LifecycleError::PreparationExpired)
    );
    assert_eq!(
        store
            .get(&key)
            .unwrap_or_else(|| panic!("test value missing"))
            .bytes(),
        before
    );
    assert_eq!(lifecycle.state(id), Ok(LifecycleState::Signed));
    assert_eq!(lifecycle.has_signed_bytes(id), Ok(true));
    let equality = text(
        lifecycle.expire_calls(
            &mut store,
            &owner.tenant,
            &limiter,
            NOT_AFTER_MS,
            CURRENT_SEQUENCE,
        ),
        "call equality sweep",
    );
    assert!(equality.expired_preparations.is_empty());
    assert!(equality.released_reservations.is_empty());
    assert_eq!(limiter.held_reservations(), Ok(1));
    assert_eq!(
        text(
            lifecycle.admit_call_submission(&mut store, &owner, GENERATION, id, NOT_AFTER_MS),
            "call equality submission"
        ),
        raw
    );
    text(
        lifecycle.transition_call(
            &mut store,
            &owner,
            GENERATION,
            id,
            LifecycleState::Failed,
            CURRENT_SEQUENCE,
        ),
        "call failure",
    );
    let early = text(
        lifecycle.sweep_call_retention(&mut store, &owner.tenant, CURRENT_SEQUENCE + 4, 5),
        "call early retention",
    );
    assert_eq!(early.discarded_terminal_signed_bytes, 0);
    assert_eq!(lifecycle.has_signed_bytes(id), Ok(true));
    let due = text(
        lifecycle.sweep_call_retention(&mut store, &owner.tenant, CURRENT_SEQUENCE + 5, 5),
        "call due retention",
    );
    assert_eq!(due.discarded_terminal_signed_bytes, 1);
    assert_eq!(due.evicted_terminal_preparations, 1);
    assert_eq!(lifecycle.state(id), Err(LifecycleError::NotFound));
    drop(store);
    text(fs::remove_dir_all(root), "remove call store");
}

fn retain_evm_call(
    root: &std::path::Path,
    owner: &SessionRef,
    generation: u64,
    id: [u8; 32],
    current_sequence: u64,
) -> (BudgetLimiter, EvmPrepared, Vec<u8>) {
    let mut store = text(Store::open(root), "call store");
    let lifecycle = PreparationLifecycle::default();
    let limiter = limiter();
    let hold = text(
        reserve(
            &limiter,
            &ReservationRequest {
                id,
                amount: 100,
                expiry_sequence: current_sequence + 1_000,
                current_sequence: OBSERVED_HEAD_SEQUENCE,
                applicable_limits: vec![LimitId([1; 16])],
            },
        ),
        "call reservation",
    );
    let prepared = evm_prepared();
    text(
        lifecycle.register_call(
            &mut store,
            owner,
            generation,
            id,
            &prepared,
            hold.durable.into_iter().map(|hold| (hold, None)).collect(),
        ),
        "register call",
    );
    text(
        lifecycle.transition_call(
            &mut store,
            owner,
            generation,
            id,
            LifecycleState::Signing,
            OBSERVED_HEAD_SEQUENCE,
        ),
        "signing call",
    );
    let raw = signed_bytes();
    assert_eq!(
        text(
            lifecycle.retain_signed_call(&mut store, owner, generation, id, raw.clone(),),
            "retain call"
        ),
        transaction_hash(&raw)
    );
    drop(lifecycle);
    drop(store);
    (limiter, prepared, raw)
}

fn assert_restored_evm_call(
    store: &Store,
    lifecycle: &PreparationLifecycle,
    owner: &SessionRef,
    generation: u64,
    id: [u8; 32],
    prepared: &EvmPrepared,
    raw: &[u8],
) {
    assert_eq!(
        text(
            lifecycle.restore_calls(store, &owner.tenant),
            "restore calls"
        ),
        1
    );
    assert_eq!(
        text(
            lifecycle.prepared_call(store, owner, generation, id),
            "restore preparation"
        ),
        *prepared
    );
    assert_eq!(
        text(
            lifecycle.recovered_signed_call(store, owner, generation, id),
            "restore bytes"
        ),
        raw
    );
    assert_eq!(
        lifecycle.prepared_call(store, owner, generation + 1, id),
        Err(LifecycleError::AuthorizationMismatch)
    );
}
