fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};

use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
    SigningPreimage,
};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::approval::{
    ApprovalExpiry, ApprovalOperationError, ApprovalOutcome, ApprovalService,
    ApprovalSubmissionQueue, DecisionKey, DecisionRequest,
};
use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::policy::approval::{
    reserve_and_hold_at, ApprovalContext, ApprovalError, ApprovalRegistry, ApprovalSequence,
    ApprovalState, ApproverId,
};
use layerx_agentd::session::SessionId;
use layerx_agentd::store::{Store, TenantId};
use layerx_types::ids::Did;
use sha2::{Digest as _, Sha256};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        Self(std::env::temp_dir().join(format!(
            "layerx-repair-22-31-495-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        )))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        require_ok(
            std::fs::remove_dir_all(&self.0),
            "remove durable test store",
        );
    }
}

struct Durable {
    registry: ApprovalRegistry,
    limiter: BudgetLimiter,
    expiry: ApprovalExpiry,
    queue: ApprovalSubmissionQueue,
}
impl Durable {
    fn open(directory: &Directory) -> Self {
        let store = Arc::new(Mutex::new(require_ok(
            Store::open(&directory.0),
            "open durable store",
        )));
        Self {
            registry: ApprovalRegistry::with_store(Arc::clone(&store)),
            limiter: limiter(),
            expiry: ApprovalExpiry::from_shared_store(store),
            queue: ApprovalSubmissionQueue::default(),
        }
    }
    fn service(&self) -> ApprovalService<'_> {
        ApprovalService::new(&self.registry, &self.limiter, &self.expiry)
    }
    fn hold(&self, tenant: &TenantId) -> Prepared {
        let prepared = prepared(1);
        let ticket = require_ok(
            reserve_and_hold_at(
                &self.registry,
                &self.limiter,
                context(tenant, 1),
                prepared.clone(),
                &ReservationRequest {
                    id: [1; 32],
                    amount: 50,
                    expiry_sequence: 100,
                    current_sequence: 10,
                    applicable_limits: vec![LimitId([9; 16])],
                },
                ApprovalSequence(100),
                TimestampSeconds(200),
            ),
            "reserve and durably hold with explicit time",
        );
        assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
        assert_eq!(self.limiter.held_reservations(), Ok(1));
        prepared
    }
    fn assert_visible(&self, tenant: &TenantId, prepared: &Prepared) {
        let page = require_ok(
            self.service()
                .list(tenant, None, 10, 11, TimestampSeconds(201)),
            "list awaiting hold with time",
        );
        assert_eq!(page.approvals.len(), 1);
        assert_eq!(page.approvals[0].approval_id, [1; 32]);
        let record = require_ok(
            self.service()
                .get(tenant, [1; 32], 11, TimestampSeconds(201)),
            "get awaiting hold with time",
        );
        assert_eq!(page.approvals[0], record);
        assert_eq!(record.state, ApprovalState::AwaitingApproval);
        assert_eq!(record.held_activity, prepared.disclosure);
        assert_eq!(
            require_some(record.presentation, "durable time facts").created_at_unix_seconds,
            200
        );
    }
}

fn digest(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn tenant(value: &str) -> TenantId {
    TenantId::new(value).unwrap_or_else(|error| panic!("tenant: {error}"))
}

fn prepared(id: u8) -> Prepared {
    let call = layerx_crypto::evm_transaction::Eip1559Call {
        chain_id: 125,
        nonce: u64::from(id),
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: [3; 20],
        data: [
            layerx_crypto::disclosure::call::ledger_selector("transfer(address,uint256)").to_vec(),
            [vec![0; 12], vec![5; 20]].concat(),
            [vec![0; 31], vec![50]].concat(),
        ]
        .concat(),
    };
    let canonical = call.encode_unsigned();
    let actor = AgentDid::new("0404040404040404040404040404040404040404")
        .unwrap_or_else(|error| panic!("actor: {error:?}"));
    Prepared {
        preparation_ref: PreparationRef::new(format!("prepared-{id}"))
            .unwrap_or_else(|error| panic!("preparation: {error:?}")),
        unsigned_canonical_bytes: CanonicalBytes::new(canonical.clone())
            .unwrap_or_else(|error| panic!("canonical: {error:?}")),
        signing_preimage: SigningPreimage::new(call.signing_hash().to_vec())
            .unwrap_or_else(|error| panic!("preimage: {error:?}")),
        disclosure: Disclosure {
            canonical_digest: digest(&canonical),
            activity_type: ActivityType(7),
            actor: actor.clone(),
            authority: AuthorityRef::new("session-key-1")
                .unwrap_or_else(|error| panic!("authority: {error:?}")),
            counterparties: ExplicitSet::allow(vec![AgentDid::new("did:layerx:merchant")
                .unwrap_or_else(|error| panic!("counterparty: {error:?}"))]),
            amounts: ExplicitSet::allow(vec![DisclosedAmount {
                counterparty: AgentDid::new("did:layerx:merchant")
                    .unwrap_or_else(|error| panic!("counterparty: {error:?}")),
                amount: Amount(50),
            }]),
            asset: Asset::new("LXP").unwrap_or_else(|error| panic!("asset: {error:?}")),
            fee_limit: Amount(2),
            expiry: TimestampSeconds(500),
            idempotency_key: IdempotencyRef::new(format!("idempotency-{id}"))
                .unwrap_or_else(|error| panic!("idempotency: {error:?}")),
        },
        expiry: TimestampSeconds(500),
    }
}

fn context(tenant_id: &TenantId, id: u8) -> ApprovalContext {
    ApprovalContext {
        actor_address: [4; 20],
        tenant: tenant_id.clone(),
        agent: Did::new(b"0404040404040404040404040404040404040404")
            .unwrap_or_else(|error| panic!("agent: {error:?}")),
        session: SessionId([2; 32]),
        capability: CapabilityId([3; 32]),
        policy_version: "policy-v2".to_owned(),
        request_id: [id; 32],
    }
}

fn limiter() -> BudgetLimiter {
    BudgetLimiter::new(vec![LimitConfig {
        id: LimitId([9; 16]),
        name: "approval-limit".to_owned(),
        scope: LimitScope::Tenant([1; 32]),
        ceiling: 1_000,
        consumed: 0,
    }])
    .unwrap_or_else(|error| panic!("limiter: {error:?}"))
}

fn approver() -> ApproverId {
    ApproverId::new("human:operator").unwrap_or_else(|error| panic!("approver: {error:?}"))
}

fn decision_key(value: &str) -> DecisionKey {
    DecisionKey::new(value).unwrap_or_else(|error| panic!("decision key: {error:?}"))
}

fn request<'a>(tenant: &'a TenantId, key: &'a DecisionKey, time: u64) -> DecisionRequest<'a> {
    DecisionRequest {
        tenant,
        approval_id: [1; 32],
        idempotency_key: key,
        approver: approver(),
        current_sequence: 11,
        current_time: TimestampSeconds(time),
    }
}

#[test]
fn durable_awaiting_hold_lists_gets_and_approves_with_explicit_time() {
    let directory = Directory::new();
    let durable = Durable::open(&directory);
    let tenant = tenant("timed-approval");
    let prepared = durable.hold(&tenant);
    durable.assert_visible(&tenant, &prepared);
    let key = decision_key("approve-timed");
    let decision = require_ok(
        durable
            .service()
            .approve(request(&tenant, &key, 201), &prepared, &durable.queue),
        "approve awaiting hold with time",
    );
    assert_eq!(decision.outcome, ApprovalOutcome::Granted);
    let reference = require_some(decision.submission_ref, "released submission");
    assert_eq!(durable.queue.prepared(reference), Some(prepared.clone()));
    assert_eq!(durable.queue.len(), 1);
    assert_eq!(
        require_some(
            durable
                .registry
                .audit_entry([1; 32])
                .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value")),
            "required test value"
        )
        .decision,
        ApprovalState::Approved
    );
    drop(durable);
    let reopened = Durable::open(&directory);
    assert_eq!(
        reopened.registry.replay_tenant(&tenant, &reopened.limiter),
        Ok(0)
    );
    assert_eq!(
        require_ok(
            reopened
                .service()
                .approve(request(&tenant, &key, 202), &prepared, &reopened.queue),
            "durable decision survives reopen"
        ),
        decision
    );
}

#[test]
fn durable_awaiting_hold_rejects_with_explicit_time() {
    let directory = Directory::new();
    let durable = Durable::open(&directory);
    let tenant = tenant("timed-rejection");
    let prepared = durable.hold(&tenant);
    durable.assert_visible(&tenant, &prepared);
    let key = decision_key("reject-timed");
    let decision = require_ok(
        durable.service().reject(request(&tenant, &key, 201)),
        "reject awaiting hold with time",
    );
    assert_eq!(decision.outcome, ApprovalOutcome::Rejected);
    assert_eq!(decision.submission_ref, None);
    assert!(durable.queue.is_empty());
    assert_eq!(durable.limiter.held_reservations(), Ok(0));
    assert_eq!(
        require_some(
            durable
                .registry
                .audit_entry([1; 32])
                .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value")),
            "required test value"
        )
        .decision,
        ApprovalState::Rejected
    );
    drop(durable);
    let reopened = Durable::open(&directory);
    assert_eq!(
        reopened.registry.replay_tenant(&tenant, &reopened.limiter),
        Ok(0)
    );
    assert_eq!(
        require_ok(
            reopened
                .expiry
                .recover(&tenant, [1; 32], 100, 12, &reopened.limiter),
            "recover persisted rejection"
        ),
        decision
    );
}

#[test]
fn reopened_awaiting_hold_decides_with_explicit_time() {
    let directory = Directory::new();
    let tenant = tenant("timed-recovery");
    let prepared = {
        let durable = Durable::open(&directory);
        durable.hold(&tenant)
    };
    let recovered = Durable::open(&directory);
    assert_eq!(
        recovered
            .registry
            .replay_tenant(&tenant, &recovered.limiter),
        Ok(1)
    );
    assert_eq!(recovered.limiter.held_reservations(), Ok(1));
    recovered.assert_visible(&tenant, &prepared);
    let key = decision_key("recovered-approve");
    let decision = require_ok(
        recovered
            .service()
            .approve(request(&tenant, &key, 201), &prepared, &recovered.queue),
        "approve recovered awaiting hold with time",
    );
    assert_eq!(decision.outcome, ApprovalOutcome::Granted);
    assert_eq!(
        recovered
            .queue
            .prepared(require_some(decision.submission_ref, "required test value")),
        Some(prepared)
    );
    assert_eq!(
        require_some(
            recovered
                .registry
                .audit_entry([1; 32])
                .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value")),
            "required test value"
        )
        .decision,
        ApprovalState::Approved
    );
}

#[test]
fn time_before_reservation_is_refused_without_deciding() {
    let directory = Directory::new();
    let durable = Durable::open(&directory);
    let tenant = tenant("timed-backwards");
    let prepared = durable.hold(&tenant);
    let expected = ApprovalOperationError::Registry(ApprovalError::InvalidWindow);
    assert_eq!(
        durable
            .service()
            .list(&tenant, None, 10, 11, TimestampSeconds(199)),
        Err(expected.clone())
    );
    assert_eq!(
        durable
            .service()
            .get(&tenant, [1; 32], 11, TimestampSeconds(199)),
        Err(expected.clone())
    );
    let key = decision_key("backwards");
    assert_eq!(
        durable
            .service()
            .approve(request(&tenant, &key, 199), &prepared, &durable.queue),
        Err(expected.clone())
    );
    assert_eq!(
        durable.service().reject(request(&tenant, &key, 199)),
        Err(expected)
    );
    assert!(durable.queue.is_empty());
    assert_eq!(durable.limiter.held_reservations(), Ok(1));
    assert_eq!(durable.registry.audit_entry([1; 32]), Ok(None));
    durable.assert_visible(&tenant, &prepared);
    assert_eq!(
        require_ok(
            durable.service().reject(request(&tenant, &key, 201)),
            "required test value"
        )
        .outcome,
        ApprovalOutcome::Rejected
    );
}
