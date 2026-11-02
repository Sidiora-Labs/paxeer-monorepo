use std::collections::{BTreeMap, BTreeSet};
use std::fmt::{Display, Formatter, Write as _};
use std::future::Future;
use std::path::{Path, PathBuf};
use std::pin::pin;
use std::sync::atomic::{AtomicU64, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::task::{Context, Poll, Wake, Waker};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::budget::{
    release, reserve, BudgetLimiter, LimitConfig as BudgetLimitConfig, LimitId as BudgetLimitId,
    LimitScope as BudgetLimitScope, ReleaseKind, ReservationRequest,
};
use layerx_agentd::idempotency::{
    EconomicResult, Outcome as IdempotencyOutcome, RetentionPolicy, Store as IdempotencyStore,
};
use layerx_agentd::limits::deadline::{RequestDeadline, RequestTracker, TrackedWork, WriteStage};
use layerx_agentd::limits::quota::{
    ClientActivity, QuotaError, Resource, ResourceWrite, SheddingPolicy, TenantQuota,
};
use layerx_agentd::limits::{
    cancel, shed, CounterLedger, LimitConfig, LimitId, LimitScope, Quota, RateLimiter, RateRequest,
    Refusal,
};
use layerx_agentd::outbox::{Outbox, OutboxError, SubmissionState};
use layerx_agentd::prepare::{
    prepare_bound, restore_canonical, EvmPreparationDefaults, EvmPreparationState,
    EvmPrepareRequest, PrecompileCall,
};
use layerx_agentd::protocol_evidence::{
    EvidenceAuthority, RawReceiptEvidence, VerifiedReceiptEvidence,
};
use layerx_agentd::sign::{
    attach_external_signature, verify_before_submit, SigningPreparation, VerifiedSubmission,
};
use layerx_agentd::store::{ObjectKind, Store, TenantId};
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::evm_transaction::{decode_signed, transaction_hash};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::send::{encode_send_envelope, send_context_hash, EnvelopeOptions, SendDebit};
use layerx_crypto::signer::Signer;
use layerx_types::payload::ModuleRegistry;

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);
const INTENT: [u8; 32] = [0x17; 32];

type AttemptRecord = ([u8; 32], Vec<u8>, bool);
type AttemptHistory = Mutex<Vec<AttemptRecord>>;

struct DuplicateConvergence {
    attempts: usize,
    unique_receipts: usize,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SuiteReport {
    pub attempts: usize,
    pub economic_effects: usize,
    pub unique_receipts: usize,
    pub final_state: SubmissionState,
    pub transition_history: Vec<String>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SuiteFailure {
    message: String,
    submission_history: Vec<String>,
}

impl Display for SuiteFailure {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        writeln!(formatter, "{}", self.message)?;
        writeln!(formatter, "full submission history:")?;
        for entry in &self.submission_history {
            writeln!(formatter, "  {entry}")?;
        }
        Ok(())
    }
}

struct NoopWake;

impl Wake for NoopWake {
    fn wake(self: Arc<Self>) {}
}

struct Workspace {
    root: PathBuf,
}

impl Workspace {
    fn new() -> Self {
        let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
        Self {
            root: std::env::temp_dir().join(format!(
                "layerx-limits-exactly-once-{}-{sequence}",
                std::process::id()
            )),
        }
    }

    fn path(&self, name: &str) -> PathBuf {
        self.root.join(name)
    }
}

impl Drop for Workspace {
    fn drop(&mut self) {
        let _cleanup = std::fs::remove_dir_all(&self.root);
    }
}

/// Runs the full limits, restart, duplicate, and receipt convergence scenario.
///
/// # Errors
///
/// Returns [`SuiteFailure`] when the scenario cannot be driven to convergence: a store,
/// outbox, budget, rate-limit, quota, deadline, or idempotency call returns its own error; a
/// duplicate worker thread panics or poisons an attempt-history mutex; the final outbox status
/// is missing; or one of the invariants the suite exists to prove does not hold — the
/// indeterminate first transport settling, a same-window retry escaping rate refusal, quota
/// creation past the ceiling being accepted, a refusal releasing the unresolved reservation, a
/// durable shedding decision disappearing across restart, concurrent duplicates producing more
/// than one economic effect or more than one authoritative receipt, a post-restart resubmission
/// re-executing, or the reservation and attempt accounting not converging. The failure carries
/// the message together with the full recorded submission history.
pub fn agent_limits_exactly_once_suite() -> Result<SuiteReport, SuiteFailure> {
    let workspace = Workspace::new();
    let tenant = tenant("tenant-a")?;
    let verified = verified_submission()?;
    let exact_signed_bytes = verified.exact_bytes().to_vec();
    let tx_hash = transaction_hash(&exact_signed_bytes);
    let mut history = vec![format!(
        "prepared and signature-verified intent {} bytes={}",
        hex_id(INTENT),
        exact_signed_bytes.len()
    )];

    let outbox_root = workspace.path("outbox");
    let mut durable = step(Store::open(&outbox_root), "open outbox store", &history)?;
    let mut outbox = Outbox::default();
    let queued_hash = step(
        outbox.enqueue(&mut durable, tenant.clone(), verified.into_exact_bytes()),
        "durably enqueue verified submission",
        &history,
    )?;
    ensure(
        queued_hash == tx_hash,
        "outbox transaction hash changed",
        &history,
    )?;
    history.extend(status_history(&outbox, tx_hash));

    let (budget, budget_id) = configure_reservation(&history)?;
    let mut tracker = RequestTracker::default();
    track_write_to_transmission(&mut tracker, &history)?;
    record_indeterminate_transmission(
        &mut outbox,
        &mut durable,
        tx_hash,
        &mut tracker,
        &mut history,
    )?;

    let idempotency_root = workspace.path("idempotency");
    let retention = step(
        RetentionPolicy::new(1_000, 500),
        "configure idempotency retention",
        &history,
    )?;
    let limiter = rate_limiter(&history)?;
    let (idempotency, initial_attempts) = refuse_duplicate_in_same_window(
        &idempotency_root,
        &tenant,
        retention,
        &limiter,
        &outbox,
        &exact_signed_bytes,
        &history,
    )?;

    let quota_root = workspace.path("quota");
    let mut quota_store = step(Store::open(&quota_root), "open quota store", &history)?;
    let mut quota = quota(&tenant, &history)?;
    exhaust_quota_and_shed(&mut quota, &mut quota_store, &tenant, &history)?;
    hold_reservation_through_refusals(&budget, &tracker, &history)?;

    drop(idempotency);
    drop(outbox);
    drop(durable);
    drop(quota_store);
    history.push("simulated daemon process loss after refusal and shedding".to_owned());

    let (mut durable, mut outbox, idempotency) = restore_after_process_loss(
        &workspace, &tenant, retention, &quota, &limiter, tx_hash, &history,
    )?;
    let activity_id = outbox
        .status(tx_hash)
        .map(|status| status.activity_id)
        .ok_or_else(|| failure("authoritative outbox activity is missing", &history))?;
    let receipt = authoritative_receipt(activity_id)
        .map_err(|error| failure(format!("receipt verification: {error}"), &history))?;
    let receipt_ref = receipt.receipt_ref();

    let economic_effects = Arc::new(AtomicUsize::new(0));
    let convergence = converge_concurrent_duplicates(
        &idempotency,
        &exact_signed_bytes,
        &economic_effects,
        &initial_attempts,
        receipt_ref,
        &history,
    )?;

    settle_from_authoritative_receipt(&mut outbox, &mut durable, tx_hash, receipt, &mut history)?;

    drop(idempotency);
    refuse_post_restart_duplicate(
        &idempotency_root,
        &tenant,
        retention,
        &exact_signed_bytes,
        &economic_effects,
        &history,
    )?;
    converge_reservation_accounting(&budget, budget_id, &mut tracker, &history)?;

    let final_state = outbox
        .status(tx_hash)
        .map(|status| status.state)
        .ok_or_else(|| failure("final outbox status is missing", &history))?;
    ensure(
        convergence.attempts == 2 && final_state == SubmissionState::Executed,
        "attempt count or final outbox state did not converge",
        &history,
    )?;
    Ok(SuiteReport {
        attempts: convergence.attempts,
        economic_effects: economic_effects.load(Ordering::SeqCst),
        unique_receipts: convergence.unique_receipts,
        final_state,
        transition_history: history,
    })
}

fn configure_reservation(
    history: &[String],
) -> Result<(BudgetLimiter, BudgetLimitId), SuiteFailure> {
    let budget_id = BudgetLimitId([1; 16]);
    let budget = step(
        BudgetLimiter::new(vec![BudgetLimitConfig {
            id: budget_id,
            name: "exactly-once tenant budget".to_owned(),
            scope: BudgetLimitScope::Tenant([2; 32]),
            ceiling: 1_000,
            consumed: 0,
        }]),
        "configure reservation accounting",
        history,
    )?;
    step(
        reserve(
            &budget,
            &ReservationRequest {
                id: INTENT,
                amount: 100,
                expiry_sequence: 100,
                current_sequence: 1,
                applicable_limits: vec![budget_id],
            },
        ),
        "reserve spend before transmission",
        history,
    )?;
    Ok((budget, budget_id))
}

fn track_write_to_transmission(
    tracker: &mut RequestTracker,
    history: &[String],
) -> Result<(), SuiteFailure> {
    step(
        tracker.begin_write(
            1,
            INTENT,
            step(
                RequestDeadline::new(1_000, 2_000),
                "construct request deadline",
                history,
            )?,
        ),
        "track write request",
        history,
    )?;
    for stage in [
        WriteStage::Signing,
        WriteStage::DurableQueued,
        WriteStage::Transmitting,
    ] {
        step(
            tracker.advance_write(1, stage, 1_100),
            "advance write ownership",
            history,
        )?;
    }
    Ok(())
}

fn record_indeterminate_transmission(
    outbox: &mut Outbox,
    durable: &mut Store,
    tx_hash: [u8; 32],
    tracker: &mut RequestTracker,
    history: &mut Vec<String>,
) -> Result<(), SuiteFailure> {
    step(
        outbox.transition(
            durable,
            tx_hash,
            SubmissionState::Submitted,
            "first exact-byte transmission began",
            None,
        ),
        "record submitted state",
        history,
    )?;
    step(
        outbox.transition(
            durable,
            tx_hash,
            SubmissionState::Unknown,
            "transport response was indeterminate",
            None,
        ),
        "record unknown state",
        history,
    )?;
    step(
        cancel(tracker, 1, 1_200),
        "transfer disconnected submission to resolver",
        history,
    )?;
    history.extend(status_history(outbox, tx_hash));
    Ok(())
}

fn refuse_duplicate_in_same_window(
    idempotency_root: &Path,
    tenant: &TenantId,
    retention: RetentionPolicy,
    limiter: &RateLimiter,
    outbox: &Outbox,
    exact_signed_bytes: &[u8],
    history: &[String],
) -> Result<(IdempotencyStore, Arc<AttemptHistory>), SuiteFailure> {
    let idempotency = step(
        IdempotencyStore::open(idempotency_root, tenant.clone(), retention),
        "open idempotency store",
        history,
    )?;
    let initial_attempts: Arc<AttemptHistory> = Arc::new(Mutex::new(Vec::new()));
    let attempts_for_first = Arc::clone(&initial_attempts);

    step(
        limiter.admit(&rate_request(100)),
        "admit initial transmission",
        history,
    )?;
    let first_result = idempotency.execute(INTENT, exact_signed_bytes, 1, |attempt| {
        record_attempt(&attempts_for_first, &attempt);
        Err("transport outcome indeterminate".to_owned())
    });
    ensure(
        first_result.is_err(),
        "the indeterminate first transport unexpectedly settled",
        history,
    )?;
    ensure(
        matches!(
            limiter.admit(&rate_request(101)),
            Err(Refusal::Exceeded { .. })
        ),
        "same-window retry was not rate-refused",
        history,
    )?;
    ensure(
        outbox
            .status(transaction_hash(exact_signed_bytes))
            .map(|status| status.state)
            == Some(SubmissionState::Unknown),
        "rate refusal changed the unknown outbox state",
        history,
    )?;
    Ok((idempotency, initial_attempts))
}

fn exhaust_quota_and_shed(
    quota: &mut Quota,
    quota_store: &mut Store,
    tenant: &TenantId,
    history: &[String],
) -> Result<(), SuiteFailure> {
    step(
        quota.create_resource(
            quota_store,
            tenant,
            "storm-client",
            ResourceWrite {
                resource: Resource::Subscription,
                object_id: b"subscription-1".to_vec(),
                bytes: b"durable-subscription".to_vec(),
            },
            100,
        ),
        "consume subscription quota",
        history,
    )?;
    ensure(
        matches!(
            quota.create_resource(
                quota_store,
                tenant,
                "storm-client",
                ResourceWrite {
                    resource: Resource::Subscription,
                    object_id: b"subscription-2".to_vec(),
                    bytes: b"must-not-persist".to_vec(),
                },
                101,
            ),
            Err(QuotaError::Exhausted { .. })
        ),
        "creation past the subscription quota was accepted",
        history,
    )?;
    for retry in 0..3_u8 {
        step(
            shed(
                quota,
                quota_store,
                ClientActivity {
                    tenant: tenant.clone(),
                    client_id: "storm-client".to_owned(),
                    operation_digest: [retry; 32],
                    retry: true,
                    observed_at_ms: 200 + u64::from(retry),
                },
            ),
            "record retry-storm observation",
            history,
        )?;
    }
    Ok(())
}

fn hold_reservation_through_refusals(
    budget: &BudgetLimiter,
    tracker: &RequestTracker,
    history: &[String],
) -> Result<(), SuiteFailure> {
    ensure(
        step(
            budget.held_reservations(),
            "read held reservations after limit refusals",
            history,
        )? == 1,
        "rate, quota, or shedding released an unresolved reservation",
        history,
    )?;
    ensure(
        !step(
            release(budget, INTENT, ReleaseKind::Unknown, 50),
            "apply unknown reservation outcome",
            history,
        )?,
        "unknown outcome released the reservation",
        history,
    )?;
    ensure(
        matches!(
            tracker.view(1).map(|view| view.work),
            Some(TrackedWork::Write {
                stage: WriteStage::UnknownResolving,
                reservation_held: true,
                ..
            })
        ),
        "deadline cancellation orphaned the submission or released its reservation",
        history,
    )?;
    Ok(())
}

fn restore_after_process_loss(
    workspace: &Workspace,
    tenant: &TenantId,
    retention: RetentionPolicy,
    quota: &Quota,
    limiter: &RateLimiter,
    tx_hash: [u8; 32],
    history: &[String],
) -> Result<(Store, Outbox, Arc<IdempotencyStore>), SuiteFailure> {
    let durable = step(
        Store::open(workspace.path("outbox")),
        "reopen outbox store",
        history,
    )?;
    let mut outbox = Outbox::default();
    step(
        outbox.restore(&durable, tenant.clone(), tx_hash),
        "restore unknown outbox",
        history,
    )?;
    let idempotency = Arc::new(step(
        IdempotencyStore::open(workspace.path("idempotency"), tenant.clone(), retention),
        "reopen idempotency store",
        history,
    )?);
    step(
        idempotency.restore(&[INTENT]),
        "restore pending idempotency record",
        history,
    )?;
    let quota_store = step(
        Store::open(workspace.path("quota")),
        "reopen quota decisions",
        history,
    )?;
    let quota_health = step(
        quota.health(&quota_store, tenant, 300),
        "restore quota health",
        history,
    )?;
    ensure(
        quota_health.actively_shed_clients == vec!["storm-client"],
        "durable shedding decision disappeared across restart",
        history,
    )?;
    step(
        limiter.admit(&rate_request(1_000)),
        "admit retry in the next rate window",
        history,
    )?;
    Ok((durable, outbox, idempotency))
}

fn converge_concurrent_duplicates(
    idempotency: &Arc<IdempotencyStore>,
    exact_signed_bytes: &[u8],
    economic_effects: &Arc<AtomicUsize>,
    initial_attempts: &Arc<AttemptHistory>,
    receipt_ref: [u8; 32],
    history: &[String],
) -> Result<DuplicateConvergence, SuiteFailure> {
    let successful_attempts: Arc<AttemptHistory> = Arc::new(Mutex::new(Vec::new()));
    let mut workers = Vec::new();
    for _ in 0..16 {
        let store = Arc::clone(idempotency);
        let bytes = exact_signed_bytes.to_vec();
        let effects = Arc::clone(economic_effects);
        let attempts = Arc::clone(&successful_attempts);
        workers.push(std::thread::spawn(move || {
            store.execute(INTENT, &bytes, 2, |attempt| {
                record_attempt(&attempts, &attempt);
                effects.fetch_add(1, Ordering::SeqCst);
                Ok(EconomicResult {
                    response_bytes: b"executed".to_vec(),
                    receipt_ref: Some(receipt_ref),
                })
            })
        }));
    }
    let mut outcomes = Vec::new();
    for worker in workers {
        let joined = worker
            .join()
            .map_err(|_| failure("duplicate worker panicked", history))?;
        outcomes.push(step(joined, "execute concurrent duplicate", history)?);
    }

    let attempts = initial_attempts
        .lock()
        .map_err(|_| failure("initial attempt history was poisoned", history))?
        .len()
        + successful_attempts
            .lock()
            .map_err(|_| failure("successful attempt history was poisoned", history))?
            .len();
    let successful = successful_attempts
        .lock()
        .map_err(|_| failure("successful attempt history was poisoned", history))?
        .clone();
    ensure(
        successful.len() == 1
            && successful[0].0 == INTENT
            && successful[0].1.as_slice() == exact_signed_bytes
            && successful[0].2,
        "concurrent retry changed bytes/key or executed more than once",
        history,
    )?;
    ensure(
        economic_effects.load(Ordering::SeqCst) == 1,
        "concurrent duplicates produced multiple economic effects",
        history,
    )?;

    let receipts = outcomes
        .iter()
        .filter_map(|outcome| match outcome {
            IdempotencyOutcome::First(result) | IdempotencyOutcome::RepeatedOriginal(result) => {
                result.receipt_ref
            }
        })
        .collect::<BTreeSet<_>>();
    ensure(
        receipts == BTreeSet::from([receipt_ref]),
        "duplicates did not converge on one authoritative receipt",
        history,
    )?;
    Ok(DuplicateConvergence {
        attempts,
        unique_receipts: receipts.len(),
    })
}

fn settle_from_authoritative_receipt(
    outbox: &mut Outbox,
    durable: &mut Store,
    tx_hash: [u8; 32],
    evidence: VerifiedReceiptEvidence,
    history: &mut Vec<String>,
) -> Result<(), SuiteFailure> {
    step(
        outbox.record_execution(durable, tx_hash, evidence, 7, 0),
        "settle outbox from authoritative receipt",
        history,
    )?;
    history.extend(status_history(outbox, tx_hash));
    Ok(())
}

fn refuse_post_restart_duplicate(
    idempotency_root: &Path,
    tenant: &TenantId,
    retention: RetentionPolicy,
    exact_signed_bytes: &[u8],
    economic_effects: &Arc<AtomicUsize>,
    history: &[String],
) -> Result<(), SuiteFailure> {
    let idempotency = step(
        IdempotencyStore::open(idempotency_root, tenant.clone(), retention),
        "reopen settled idempotency store",
        history,
    )?;
    step(
        idempotency.restore(&[INTENT]),
        "restore settled idempotency record",
        history,
    )?;
    let effects = Arc::clone(economic_effects);
    let post_restart = step(
        idempotency.execute(INTENT, exact_signed_bytes, 3, move |_| {
            effects.fetch_add(1, Ordering::SeqCst);
            Ok(EconomicResult {
                response_bytes: b"duplicate-effect".to_vec(),
                receipt_ref: Some([0xff; 32]),
            })
        }),
        "submit duplicate after settled restart",
        history,
    )?;
    ensure(
        matches!(post_restart, IdempotencyOutcome::RepeatedOriginal(_))
            && economic_effects.load(Ordering::SeqCst) == 1,
        "post-restart resubmission produced a duplicate effect",
        history,
    )?;
    Ok(())
}

fn converge_reservation_accounting(
    budget: &BudgetLimiter,
    budget_id: BudgetLimitId,
    tracker: &mut RequestTracker,
    history: &[String],
) -> Result<(), SuiteFailure> {
    step(
        tracker.resolved_by_receipt(1),
        "release resolver ownership after receipt",
        history,
    )?;
    ensure(
        step(
            release(budget, INTENT, ReleaseKind::Executed, 60),
            "consume reservation after verified receipt",
            history,
        )?,
        "verified receipt did not consume the reservation",
        history,
    )?;
    ensure(
        step(
            budget.held_reservations(),
            "read final reservations",
            history,
        )? == 0
            && step(
                budget.consumed(budget_id),
                "read final consumed budget",
                history,
            )? == 100,
        "reservation accounting did not converge after the receipt",
        history,
    )?;
    Ok(())
}

#[test]
fn limits_restarts_and_duplicates_converge_on_one_economic_effect() {
    match agent_limits_exactly_once_suite() {
        Ok(report) => {
            assert_eq!(report.economic_effects, 1);
            assert_eq!(report.unique_receipts, 1);
            assert_eq!(report.final_state, SubmissionState::Executed);
            assert_eq!(report.attempts, 2);
        }
        Err(error) => panic!("{error}"),
    }
}

#[test]
fn limits_identical_raw_transaction_keeps_one_hash() {
    let workspace = Workspace::new();
    let tenant = tenant("tenant-a").unwrap_or_else(|error| panic!("{error}"));
    let raw = verified_submission()
        .unwrap_or_else(|error| panic!("{error}"))
        .into_exact_bytes();
    let tx_hash = transaction_hash(&raw);
    let root = workspace.path("outbox");
    let mut durable = Store::open(&root).expect("open outbox");
    let mut outbox = Outbox::default();
    assert_eq!(
        outbox
            .enqueue(&mut durable, tenant.clone(), raw.clone())
            .expect("enqueue signed raw transaction"),
        tx_hash
    );
    outbox
        .transition(
            &mut durable,
            tx_hash,
            SubmissionState::Submitted,
            "first raw submission",
            None,
        )
        .expect("record transmission");
    outbox
        .transition(
            &mut durable,
            tx_hash,
            SubmissionState::Unknown,
            "response lost before restart",
            None,
        )
        .expect("record indeterminate response");
    let original_status = outbox.status(tx_hash).expect("queued hash").clone();
    drop(outbox);
    drop(durable);

    let mut durable = Store::open(&root).expect("reopen outbox after restart");
    let mut outbox = Outbox::default();
    assert!(matches!(
        outbox.enqueue(&mut durable, tenant.clone(), raw.clone()),
        Err(OutboxError::Duplicate)
    ));
    outbox
        .restore(&durable, tenant.clone(), tx_hash)
        .expect("restore original raw transaction");
    assert_eq!(
        outbox.exact_signed_bytes(tx_hash).expect("retained bytes"),
        raw
    );
    assert!(matches!(
        outbox.enqueue(&mut durable, tenant.clone(), raw.clone()),
        Err(OutboxError::Duplicate)
    ));
    assert_eq!(outbox.status(tx_hash), Some(&original_status));
    assert_eq!(original_status.submission_id, tx_hash);
    assert_eq!(original_status.activity_id, tx_hash);
    for kind in [ObjectKind::Outbox, ObjectKind::PreparedActivity] {
        assert_eq!(
            durable.list_object_ids(&tenant, kind),
            vec![tx_hash.to_vec()]
        );
    }
    drop(outbox);
    drop(durable);

    let durable = Store::open(&root).expect("reopen after duplicate submission");
    let mut outbox = Outbox::default();
    outbox
        .restore(&durable, tenant.clone(), tx_hash)
        .expect("restore unique hash");
    assert_eq!(
        durable.list_object_ids(&tenant, ObjectKind::Outbox),
        vec![tx_hash.to_vec()]
    );
    assert_eq!(outbox.status(tx_hash), Some(&original_status));
    assert_eq!(
        outbox
            .exact_signed_bytes(tx_hash)
            .expect("unchanged raw bytes"),
        raw
    );
}

fn rate_limiter(history: &[String]) -> Result<RateLimiter, SuiteFailure> {
    RateLimiter::new(
        vec![LimitConfig {
            id: LimitId::new("tenant-submit")
                .map_err(|error| failure(format!("rate id: {error:?}"), history))?,
            scope: LimitScope::Tenant {
                tenant: "tenant-a".to_owned(),
            },
            limit: 1,
            window_ms: 1_000,
        }],
        CounterLedger::shared(),
    )
    .map_err(|error| failure(format!("rate configuration: {error:?}"), history))
}

fn rate_request(observed_at_ms: u64) -> RateRequest {
    RateRequest {
        tenant: "tenant-a".to_owned(),
        agent: "agent-a".to_owned(),
        session: "session-a".to_owned(),
        capability: "submit".to_owned(),
        operation_class: "write".to_owned(),
        logical_time_ms: observed_at_ms,
        cost: 1,
    }
}

fn quota(tenant: &TenantId, history: &[String]) -> Result<Quota, SuiteFailure> {
    let limits = Resource::ALL
        .into_iter()
        .map(|resource| (resource, 1))
        .collect::<BTreeMap<_, _>>();
    let tenant_quota = TenantQuota::new(tenant.clone(), limits)
        .map_err(|error| failure(format!("tenant quota: {error:?}"), history))?;
    Quota::new(
        [tenant_quota],
        SheddingPolicy {
            window_ms: 1_000,
            maximum_requests: 10,
            maximum_retries: 2,
            maximum_identical_operations: 10,
            shed_for_ms: 5_000,
        },
    )
    .map_err(|error| failure(format!("quota configuration: {error:?}"), history))
}

fn record_attempt(
    attempts: &AttemptHistory,
    attempt: &layerx_agentd::idempotency::ProtocolAttempt<'_>,
) {
    if let Ok(mut attempts) = attempts.lock() {
        attempts.push((
            attempt.idempotency_key,
            attempt.exact_request_bytes.to_vec(),
            attempt.retry,
        ));
    }
}

fn status_history(outbox: &Outbox, submission_id: [u8; 32]) -> Vec<String> {
    let Some(status) = outbox.status(submission_id) else {
        return vec!["outbox status missing".to_owned()];
    };
    let mut history = vec![format!("current state: {:?}", status.state)];
    history.extend(status.transitions.iter().map(|transition| {
        format!(
            "{:?} -> {:?}: {} receipt={:?}",
            transition.from, transition.to, transition.cause, transition.receipt
        )
    }));
    history
}

fn tenant(value: &str) -> Result<TenantId, SuiteFailure> {
    TenantId::new(value).map_err(|error| failure(format!("tenant: {error}"), &[]))
}

fn step<T, E: std::fmt::Debug>(
    result: Result<T, E>,
    label: &str,
    history: &[String],
) -> Result<T, SuiteFailure> {
    result.map_err(|error| failure(format!("{label}: {error:?}"), history))
}

fn ensure(
    condition: bool,
    message: impl Into<String>,
    history: &[String],
) -> Result<(), SuiteFailure> {
    if condition {
        Ok(())
    } else {
        Err(failure(message, history))
    }
}

fn failure(message: impl Into<String>, history: &[String]) -> SuiteFailure {
    SuiteFailure {
        message: message.into(),
        submission_history: history.to_vec(),
    }
}

fn hex_id(identifier: [u8; 32]) -> String {
    identifier.iter().fold(String::new(), |mut rendered, byte| {
        let _ = write!(rendered, "{byte:02x}");
        rendered
    })
}

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(NoopWake));
    let mut context = Context::from_waker(&waker);
    match future.as_mut().poll(&mut context) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("local signer unexpectedly blocked"),
    }
}

fn verified_submission() -> Result<VerifiedSubmission, SuiteFailure> {
    let (prepared, registry, signer) = send_payload()?;
    let signature = step(
        ready(signer.sign_transaction(&prepared.transaction.call)),
        "sign EIP-1559 transfer",
        &[],
    )?;
    ensure(
        signature.as_bytes().len() == 65,
        "EVM signature width changed",
        &[],
    )?;
    let signed_bytes = step(
        attach_external_signature(&prepared, *signature.as_bytes()),
        "attach EVM signature",
        &[],
    )?;
    let verified = step(
        verify_before_submit(&signed_bytes, &prepared, &prepared.actor_address, &registry),
        "verify EVM signature",
        &[],
    )?;
    let (call, signature) = step(
        decode_signed(verified.exact_bytes()),
        "decode raw transaction",
        &[],
    )?;
    ensure(
        verified.exact_bytes().first() == Some(&0x02)
            && call == prepared.transaction.call
            && signature.as_bytes().len() == 65
            && verified.activity_id() == transaction_hash(verified.exact_bytes())
            && verified.idempotency_key() == INTENT,
        "verified submission is not the exact prepared EIP-1559 transfer",
        &[],
    )?;
    Ok(verified)
}

fn send_payload() -> Result<(SigningPreparation, ModuleRegistry, LocalSigner), SuiteFailure> {
    let account = step(
        derive_from_seed(&[0xa5; 32], 0),
        "derive signing account",
        &[],
    )?;
    let signer = LocalSigner::from_derived_account(&account);
    let debit = SendDebit {
        from: [0x11; 32],
        to: [0x22; 32],
        asset: [0x33; 32],
        amount: 25,
        source_sequence: 5,
        idempotency_key: INTENT,
        expires_at: 1_010,
        context_hash: send_context_hash(&[0x11; 32], &[0x22; 32], &[0x33; 32], 25, &INTENT),
        conditions: Vec::new(),
        authorization_kind: 1,
        network_id: 17,
        protocol_version: 2,
    };
    let payload = step(
        ready(debit.sign(&signer)),
        "sign native debit authorization",
        &[],
    )?;
    let disclosed = step(
        encode_send_envelope(
            &payload,
            &EnvelopeOptions {
                actor: "did:layerx:limits-exactly-once",
                public_key: signer.public_key(),
                protocol_version: 2,
                network_id: 17,
                identity_sequence: 5,
                idempotency_key: INTENT,
                fee_limit: 7,
                not_before: 995,
                not_after: 1_010,
            },
        ),
        "disclose transfer authorization",
        &[],
    )?;
    let authorization = step(
        restore_canonical(
            &disclosed.canonical,
            88,
            &disclosed.registry,
            Some(disclosed.envelope.authority()),
        ),
        "bind transfer authorization",
        &[],
    )?;
    let transaction = step(
        prepare_bound(
            EvmPreparationState {
                chain_id: 17,
                pending_nonce: 5,
                base_fee_per_gas: 3,
                head_height: 88,
            },
            EvmPreparationDefaults {
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 7,
                maximum_payload_bytes: 1_024,
            },
            EvmPrepareRequest {
                from: account.evm_address(),
                call: PrecompileCall::ledger_transfer(payload),
                expected_nonce: Some(5),
                max_priority_fee_per_gas: Some(1),
                max_fee_per_gas: Some(7),
                declared_payload_limit: 1_024,
            },
            100_000,
        ),
        "prepare EIP-1559 transfer",
        &[],
    )?;
    Ok((
        SigningPreparation {
            transaction,
            actor_address: account.evm_address(),
            authorization,
        },
        disclosed.registry,
        signer,
    ))
}

fn authoritative_receipt(activity_id: [u8; 32]) -> Result<VerifiedReceiptEvidence, SuiteFailure> {
    let key = SigningKey::from_bytes(&[0x3a; 32]);
    let protocol_version = layerx_wire::limits::PROTOCOL_VERSION;
    let previous_state_root = [0x21; 32];
    let resulting_state_root = [0x22; 32];
    let global_sequence = 9_u64;
    let batch_number = 7_u64;
    let batch_id = step(
        layerx_wire::hash::program_execution_batch_id(
            previous_state_root,
            [0x25; 32],
            global_sequence,
            global_sequence,
            batch_number,
        ),
        "derive receipt execution batch",
        &[],
    )?;
    let encode = |signature: Option<[u8; 64]>| {
        let mut encoder = layerx_wire::encode::Encoder::new(4096);
        assert_eq!(
            encoder.structure_header_version(0x5201, protocol_version),
            Ok(())
        );
        assert_eq!(encoder.u16(protocol_version), Ok(()));
        assert_eq!(encoder.bytes(&activity_id, 32), Ok(()));
        assert_eq!(encoder.u64(global_sequence), Ok(()));
        assert_eq!(encoder.bytes(&previous_state_root, 32), Ok(()));
        assert_eq!(encoder.bytes(&resulting_state_root, 32), Ok(()));
        assert_eq!(encoder.bytes(&[0x25; 32], 32), Ok(()));
        assert_eq!(encoder.i32(0), Ok(()));
        assert_eq!(encoder.sequence_length(0, 512), Ok(()));
        assert_eq!(encoder.u128(1), Ok(()));
        assert_eq!(encoder.bytes(&batch_id, 32), Ok(()));
        assert_eq!(encoder.u16(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u8(1), Ok(()));
        assert_eq!(encoder.bytes(&[0x24; 32], 32), Ok(()));
        assert_eq!(encoder.u128(25), Ok(()));
        assert_eq!(encoder.bytes(&[0x26; 32], 32), Ok(()));
        assert_eq!(encoder.u128(1_025), Ok(()));
        assert_eq!(encoder.u128(1_000), Ok(()));
        assert_eq!(encoder.u64(1), Ok(()));
        assert_eq!(encoder.bytes(&[0x27; 32], 32), Ok(()));
        assert_eq!(encoder.u128(10), Ok(()));
        assert_eq!(encoder.u128(35), Ok(()));
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
    let digest = step(
        layerx_wire::hash::receipt_digest(&encode(None)),
        "hash unsigned receipt",
        &[],
    )?;
    let canonical_receipt = encode(Some(key.sign(&digest).to_bytes()));
    let (proof, receipt_root) = step(
        layerx_proof::merkle::build_proof(&[canonical_receipt.as_slice()], 0),
        "prove receipt inclusion",
        &[],
    )?;
    let sequencer_id = key.verifying_key().to_bytes();
    let mut encoder = layerx_wire::encode::Encoder::new(354);
    assert_eq!(
        encoder.structure_header_version(0x1701, protocol_version),
        Ok(())
    );
    assert_eq!(encoder.u8(15), Ok(()));
    assert_eq!(encoder.tag(1, 15), Ok(()));
    assert_eq!(encoder.u16(protocol_version), Ok(()));
    assert_eq!(encoder.tag(2, 15), Ok(()));
    assert_eq!(encoder.u32(17), Ok(()));
    for (field, value) in [
        (3, 2),
        (4, batch_number),
        (5, global_sequence),
        (6, global_sequence),
    ] {
        assert_eq!(encoder.tag(field, 15), Ok(()));
        assert_eq!(encoder.u64(value), Ok(()));
    }
    for (field, value) in [
        (7, previous_state_root),
        (8, resulting_state_root),
        (9, [0x25; 32]),
        (10, receipt_root),
        (11, [0x34; 32]),
        (12, [0x35; 32]),
        (13, [0x36; 32]),
    ] {
        assert_eq!(encoder.tag(field, 15), Ok(()));
        assert_eq!(encoder.bytes(&value, 32), Ok(()));
    }
    assert_eq!(encoder.tag(14, 15), Ok(()));
    assert_eq!(encoder.u64(1_000), Ok(()));
    assert_eq!(encoder.tag(15, 15), Ok(()));
    assert_eq!(encoder.bytes(&sequencer_id, 32), Ok(()));
    let header = encoder.finish();
    let header_digest = step(
        layerx_wire::hash::batch_header_digest(&header),
        "hash receipt batch header",
        &[],
    )?;
    let raw = RawReceiptEvidence::new(
        canonical_receipt,
        proof,
        header,
        key.sign(&header_digest).to_bytes(),
    );
    let authority = step(
        EvidenceAuthority::pinned_to_handshake(protocol_version, 17, sequencer_id),
        "pin receipt signing authority",
        &[],
    )?;
    step(authority.verify_receipt(&raw), "verify signed receipt", &[])
}
