fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use std::os::unix::fs::DirBuilderExt;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Barrier, Mutex};

use layerx_agent_api::prepare::Prepared;
use layerx_agent_api::TimestampSeconds;
use layerx_agentd::approval::{
    ApprovalExpiry, ApprovalOutcome, ApprovalService, ApprovalSubmissionQueue, DecisionKey,
    DecisionRequest,
};
use layerx_agentd::budget::{
    reserve, BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest,
};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::policy::approval::{
    decide_at, hold_reserved_timed, ApprovalChoice, ApprovalContext, ApprovalError,
    ApprovalRegistry, ApprovalSequence, ApprovalState, ApproverId,
};
use layerx_agentd::prepare;
use layerx_agentd::session::SessionId;
use layerx_agentd::store::{Store, TenantId};
use layerx_crypto::disclosure::call::{bind_call, validate_call, validate_canonical};
use layerx_crypto::evm_transaction::decode_unsigned;
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use sha2::{Digest as _, Sha256};

#[path = "approval_ordering/preparation.rs"]
mod preparation;

static NEXT_ROOT: AtomicU64 = AtomicU64::new(0);

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-1539-{}-{}",
            std::process::id(),
            NEXT_ROOT.fetch_add(1, Ordering::Relaxed)
        ));
        require_ok(
            std::fs::DirBuilder::new().mode(0o700).create(&path),
            "private test directory",
        );
        Self(path)
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn tenant() -> TenantId {
    require_ok(TenantId::new("tenant-approval-semantics"), "tenant")
}

fn context(id: u8) -> ApprovalContext {
    ApprovalContext {
        actor_address: [4; 20],
        tenant: tenant(),
        agent: require_ok(Did::new(hex::encode([4; 20]).as_bytes()), "agent"),
        session: SessionId([2; 32]),
        capability: CapabilityId([3; 32]),
        policy_version: "policy-v3".to_owned(),
        request_id: [id; 32],
    }
}

fn modules() -> ModuleRegistry {
    require_ok(
        ModuleRegistry::new(&[require_ok(
            ModuleRegistration::new(
                ModuleId::Asset,
                &[require_ok(
                    ActivityType::new(ModuleId::Asset, 5),
                    "ledger send activity",
                )],
            ),
            "ledger registration",
        )]),
        "module registry",
    )
}

fn limiter() -> BudgetLimiter {
    require_ok(
        BudgetLimiter::new(vec![LimitConfig {
            id: LimitId([4; 16]),
            name: "approval-semantics".to_owned(),
            scope: LimitScope::Tenant([5; 32]),
            ceiling: 1_000,
            consumed: 0,
        }]),
        "limiter",
    )
}

fn hold(registry: &ApprovalRegistry, limiter: &BudgetLimiter, id: u8, prepared: Prepared) {
    let reservation = require_ok(
        reserve(
            limiter,
            &ReservationRequest {
                id: [id; 32],
                amount: 50,
                expiry_sequence: 40,
                current_sequence: 10,
                applicable_limits: vec![LimitId([4; 16])],
            },
        ),
        "reservation",
    );
    require_ok(
        hold_reserved_timed(
            registry,
            context(id),
            prepared,
            ApprovalSequence(10),
            ApprovalSequence(40),
            &reservation,
            TimestampSeconds(10),
        ),
        "producer-bound hold",
    );
}

fn request<'a>(tenant: &'a TenantId, id: u8, key: &'a DecisionKey) -> DecisionRequest<'a> {
    DecisionRequest {
        tenant,
        approval_id: [id; 32],
        idempotency_key: key,
        approver: require_ok(ApproverId::new("human:review"), "approver"),
        current_sequence: 11,
        current_time: TimestampSeconds(11),
    }
}

#[test]
fn finding_27_ledger_producer_fields_survive_codec_and_approval_hold() {
    let produced = preparation::produced(14, 40);
    let api = preparation::prepared(14, 40);
    let bytes = api.unsigned_canonical_bytes.as_bytes();
    let received = require_ok(decode_unsigned(bytes), "real transaction codec");
    let disclosure = require_ok(bind_call(&received, [4; 20]), "consumer disclosure");
    assert_eq!(received, produced.call);
    assert_eq!(disclosure, produced.disclosure);
    assert_eq!(received.to, prepare::PrecompileTarget::Ledger.address());
    assert_eq!(api.signing_preimage.as_bytes(), received.signing_hash());
    assert_eq!(
        api.disclosure.canonical_digest,
        <[u8; 32]>::from(Sha256::digest(bytes))
    );
    assert_eq!(
        api.disclosure.activity_type.0,
        disclosure.activity_type.ordinal()
    );
    assert_eq!(
        api.disclosure.actor.as_str(),
        hex::encode(&disclosure.actor)
    );
    assert_eq!(
        api.disclosure.authority.as_str(),
        hex::encode(&disclosure.authority)
    );
    assert_eq!(api.disclosure.asset.as_str(), hex::encode(disclosure.asset));
    assert_eq!(
        api.disclosure.counterparties.values().len(),
        disclosure.counterparties.len()
    );
    for (actual, expected) in api
        .disclosure
        .counterparties
        .values()
        .iter()
        .zip(&disclosure.counterparties)
    {
        assert_eq!(actual.as_str(), hex::encode(expected.account));
    }
    assert_eq!(api.disclosure.amounts.values().len(), 1);
    assert_eq!(
        api.disclosure.amounts.values()[0].counterparty.as_str(),
        hex::encode([5; 32])
    );
    assert_eq!(
        api.disclosure.amounts.values()[0].amount.0,
        disclosure.amounts[0].value
    );
    assert_eq!(
        api.disclosure.fee_limit.0,
        u128::from(received.gas_limit) * received.max_fee_per_gas
    );
    assert_eq!(
        api.disclosure.expiry.get(),
        disclosure.expiry.payload_expires_at
    );
    assert_eq!(api.expiry, api.disclosure.expiry);
    assert_eq!(
        api.disclosure.idempotency_key.as_str(),
        hex::encode(disclosure.idempotency_key)
    );
    let registry = ApprovalRegistry::default();
    let limiter = limiter();
    hold(&registry, &limiter, 14, api);
    require_ok(
        registry.validate_registry(&tenant(), &modules()),
        "real approval consumer",
    );
    assert_eq!(
        require_some(
            registry
                .ticket([14; 32])
                .unwrap_or_else(|error| panic!("{}: {error:?}", "ticket")),
            "held ticket"
        )
        .state,
        ApprovalState::AwaitingApproval
    );
}

#[test]
fn finding_27_changed_call_and_disclosure_refusals() {
    let produced = preparation::produced(16, 40);
    let canonical = produced.call.encode_unsigned();
    for field in 0..4 {
        let mut changed = produced.clone();
        match field {
            0 => changed.call.nonce += 1,
            1 => changed.call.to[0] ^= 1,
            2 => changed.call.max_fee_per_gas += 1,
            3 => changed.call.data[72] ^= 1,
            _ => unreachable!(),
        }
        assert!(prepare::verify_call_binding(&changed).is_err());
        assert!(validate_canonical(&produced.disclosure, &changed.call.encode_unsigned()).is_err());
    }
    for field in 0..7 {
        let mut changed = produced.clone();
        match field {
            0 => changed.disclosure.counterparties[1].account[0] ^= 1,
            1 => changed.disclosure.amounts[0].value += 1,
            2 => changed.disclosure.asset[0] ^= 1,
            3 => changed.disclosure.fee_limit += 1,
            4 => changed.disclosure.expiry.payload_expires_at += 1,
            5 => changed.disclosure.idempotency_key[0] ^= 1,
            6 => changed.disclosure.authority[0] ^= 1,
            _ => unreachable!(),
        }
        assert!(prepare::verify_call_binding(&changed).is_err());
        assert!(validate_call(&changed.call, &changed.disclosure, changed.from).is_err());
        assert!(validate_canonical(&changed.disclosure, &canonical).is_err());
    }
}

#[test]
fn finding_27_durable_hold_reopens_with_the_exact_producer_binding() {
    let directory = Directory::new();
    let prepared = preparation::prepared(17, 40);
    {
        let store = Arc::new(Mutex::new(require_ok(Store::open(&directory.0), "store")));
        let registry = ApprovalRegistry::with_store(store);
        hold(&registry, &limiter(), 17, prepared.clone());
    }
    let store = Arc::new(Mutex::new(require_ok(
        Store::open(&directory.0),
        "reopen store",
    )));
    let registry = ApprovalRegistry::with_store(Arc::clone(&store));
    let limiter = limiter();
    assert_eq!(registry.replay_tenant(&tenant(), &limiter), Ok(1));
    assert_eq!(limiter.held_reservations(), Ok(1));
    require_ok(
        registry.validate_registry(&tenant(), &modules()),
        "recovered call binding",
    );
    let mut changed_digest = prepared.disclosure.canonical_digest;
    changed_digest[0] ^= 1;
    assert_eq!(
        decide_at(
            &registry,
            [17; 32],
            require_ok(ApproverId::new("human:review"), "approver"),
            ApprovalChoice::Approve,
            changed_digest,
            ApprovalSequence(11),
            TimestampSeconds(11)
        ),
        Err(ApprovalError::DisclosureChanged)
    );
    let audit = require_ok(
        decide_at(
            &registry,
            [17; 32],
            require_ok(ApproverId::new("human:review"), "approver"),
            ApprovalChoice::Approve,
            prepared.disclosure.canonical_digest,
            ApprovalSequence(11),
            TimestampSeconds(11),
        ),
        "recovered decision",
    );
    assert_eq!(
        audit.disclosure_digest,
        prepared.disclosure.canonical_digest
    );
    assert_eq!(
        require_some(
            registry
                .ticket([17; 32])
                .unwrap_or_else(|error| panic!("{}: {error:?}", "ticket")),
            "held"
        )
        .state,
        ApprovalState::Approved
    );
    drop(registry);
    drop(store);
    let reopened = ApprovalRegistry::with_store(Arc::new(Mutex::new(require_ok(
        Store::open(&directory.0),
        "reopen after decision",
    ))));
    assert_eq!(reopened.replay_tenant(&tenant(), &self::limiter()), Ok(0));
}

#[test]
fn finding_27_grant_rejection_concurrency_and_durable_winner() {
    let directory = Directory::new();
    let registry = ApprovalRegistry::default();
    let limiter = limiter();
    let expiry = require_ok(ApprovalExpiry::open(&directory.0), "expiry");
    let queue = ApprovalSubmissionQueue::default();
    let prepared = preparation::prepared(18, 40);
    hold(&registry, &limiter, 18, prepared.clone());
    let owner = tenant();
    let grant_key = require_ok(DecisionKey::new("grant-18"), "grant key");
    let reject_key = require_ok(DecisionKey::new("reject-18"), "reject key");
    let start = Barrier::new(2);
    let (grant, rejection) = std::thread::scope(|scope| {
        let grant = scope.spawn(|| {
            start.wait();
            ApprovalService::new(&registry, &limiter, &expiry).approve(
                request(&owner, 18, &grant_key),
                &prepared,
                &queue,
            )
        });
        let rejection = scope.spawn(|| {
            start.wait();
            ApprovalService::new(&registry, &limiter, &expiry).reject(request(
                &owner,
                18,
                &reject_key,
            ))
        });
        (
            require_ok(
                grant
                    .join()
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "grant thread")),
                "grant decision",
            ),
            require_ok(
                rejection
                    .join()
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "reject thread")),
                "rejection decision",
            ),
        )
    });
    let winner = match (grant.outcome, rejection.outcome) {
        (ApprovalOutcome::Granted, ApprovalOutcome::Conflict) => {
            assert_eq!(rejection.winning_outcome, Some(ApprovalOutcome::Granted));
            assert_eq!(queue.len(), 1);
            assert_eq!(
                queue.prepared(require_some(grant.submission_ref, "submission")),
                Some(prepared)
            );
            grant
        }
        (ApprovalOutcome::Conflict, ApprovalOutcome::Rejected) => {
            assert_eq!(grant.winning_outcome, Some(ApprovalOutcome::Rejected));
            assert!(queue.is_empty());
            rejection
        }
        outcomes => panic!("exactly one durable winner required: {outcomes:?}"),
    };
    drop(expiry);
    let reopened = require_ok(ApprovalExpiry::open(&directory.0), "reopen expiry");
    let durable = require_ok(
        reopened.recover(&owner, [18; 32], 40, 12, &limiter),
        "recovered decision",
    );
    assert_eq!(durable.outcome, winner.outcome);
    assert_eq!(durable.submission_ref, winner.submission_ref);
}

#[test]
fn finding_27_api_changed_call_and_disclosure_never_queue() {
    for id in [19, 20] {
        let directory = Directory::new();
        let registry = ApprovalRegistry::default();
        let limiter = limiter();
        let expiry = require_ok(ApprovalExpiry::open(&directory.0), "expiry");
        let queue = ApprovalSubmissionQueue::default();
        let prepared = preparation::prepared(id, 40);
        hold(&registry, &limiter, id, prepared.clone());
        let mut changed = prepared;
        if id == 19 {
            let other_call = preparation::produced(id + 1, 40);
            changed.unsigned_canonical_bytes = require_ok(
                layerx_agent_api::prepare::CanonicalBytes::new(other_call.call.encode_unsigned()),
                "changed call",
            );
        } else {
            changed.disclosure.fee_limit.0 += 1;
        }
        let decision = require_ok(
            ApprovalService::new(&registry, &limiter, &expiry).approve(
                request(
                    &tenant(),
                    id,
                    &require_ok(DecisionKey::new(format!("changed-{id}")), "decision key"),
                ),
                &changed,
                &queue,
            ),
            "changed preparation decision",
        );
        assert_eq!(decision.outcome, ApprovalOutcome::Defective);
        assert_eq!(decision.submission_ref, None);
        assert!(queue.is_empty());
        drop(expiry);
        let reopened = require_ok(ApprovalExpiry::open(&directory.0), "reopen refusal");
        assert_eq!(
            require_ok(
                reopened.recover(&tenant(), [id; 32], 40, 12, &limiter),
                "durable refusal"
            )
            .outcome,
            ApprovalOutcome::Defective
        );
    }
}
