use std::fs;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Barrier};
use std::thread;

use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, Disclosure, IdempotencyRef, PreparationRef, Prepared, SigningPreimage,
};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::approval::{
    ApprovalExpiry, ApprovalOutcome, ApprovalService, ApprovalSubmissionQueue, DecisionKey,
    DecisionRequest,
};
use layerx_agentd::budget::{
    reserve, BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest,
};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::policy::approval::{
    hold_at, ApprovalContext, ApprovalError, ApprovalRegistry, ApprovalSequence, ApproverId,
};
use layerx_agentd::session::SessionId;
use layerx_agentd::store::TenantId;
use layerx_types::ids::Did;
use sha2::{Digest as _, Sha256};

struct Directory(PathBuf);

impl Directory {
    fn new(label: &str) -> Self {
        let root = std::env::temp_dir().join(format!(
            "layerx-approval-semantics-{label}-{}",
            std::process::id()
        ));
        let _ = fs::remove_dir_all(&root);
        Self(root)
    }

    fn path(&self) -> &Path {
        &self.0
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn tenant() -> TenantId {
    TenantId::new("tenant-approval-semantics").unwrap_or_else(|error| panic!("tenant: {error}"))
}

fn prepared(id: u8, expiry: u64) -> Prepared {
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
    let bytes = call.encode_unsigned();
    let digest: [u8; 32] = Sha256::digest(&bytes).into();
    let actor = AgentDid::new("0404040404040404040404040404040404040404")
        .unwrap_or_else(|error| panic!("actor: {error:?}"));
    Prepared {
        preparation_ref: PreparationRef::new(format!("prepared-{id}"))
            .unwrap_or_else(|error| panic!("preparation: {error:?}")),
        unsigned_canonical_bytes: CanonicalBytes::new(bytes)
            .unwrap_or_else(|error| panic!("bytes: {error:?}")),
        signing_preimage: SigningPreimage::new(call.signing_hash().to_vec())
            .unwrap_or_else(|error| panic!("preimage: {error:?}")),
        disclosure: Disclosure {
            canonical_digest: digest,
            activity_type: ActivityType(7),
            actor,
            authority: AuthorityRef::new("session-key")
                .unwrap_or_else(|error| panic!("authority: {error:?}")),
            counterparties: ExplicitSet::deny_all(),
            amounts: ExplicitSet::deny_all(),
            asset: Asset::new("LXP").unwrap_or_else(|error| panic!("asset: {error:?}")),
            fee_limit: Amount(2),
            expiry: TimestampSeconds(expiry),
            idempotency_key: IdempotencyRef::new(format!("activity-{id}"))
                .unwrap_or_else(|error| panic!("activity key: {error:?}")),
        },
        expiry: TimestampSeconds(expiry),
    }
}

fn context(id: u8) -> ApprovalContext {
    ApprovalContext {
        actor_address: [4; 20],
        tenant: tenant(),
        agent: Did::new(b"0404040404040404040404040404040404040404")
            .unwrap_or_else(|error| panic!("agent: {error:?}")),
        session: SessionId([2; 32]),
        capability: CapabilityId([3; 32]),
        policy_version: "policy-v3".to_owned(),
        request_id: [id; 32],
    }
}

fn limiter() -> BudgetLimiter {
    BudgetLimiter::new(vec![LimitConfig {
        id: LimitId([4; 16]),
        name: "approval-semantics".to_owned(),
        scope: LimitScope::Tenant([5; 32]),
        ceiling: 1_000,
        consumed: 0,
    }])
    .unwrap_or_else(|error| panic!("limiter: {error:?}"))
}

fn reserve_hold(limiter: &BudgetLimiter, id: u8, expiry: u64) {
    reserve(
        limiter,
        &ReservationRequest {
            id: [id; 32],
            amount: 10,
            expiry_sequence: expiry,
            current_sequence: 10,
            applicable_limits: vec![LimitId([4; 16])],
        },
    )
    .unwrap_or_else(|error| panic!("reserve: {error:?}"));
}

fn key(value: &str) -> DecisionKey {
    DecisionKey::new(value).unwrap_or_else(|error| panic!("decision key: {error:?}"))
}

fn approver(value: &str) -> ApproverId {
    ApproverId::new(value).unwrap_or_else(|error| panic!("approver: {error:?}"))
}

#[test]
fn concurrent_conflicting_decisions_have_one_durable_winner() {
    let directory = Directory::new("concurrency");
    let registry = Arc::new(ApprovalRegistry::default());
    let limiter = Arc::new(limiter());
    let expiry = Arc::new(
        ApprovalExpiry::open(directory.path()).unwrap_or_else(|error| panic!("expiry: {error:?}")),
    );
    let queue = Arc::new(ApprovalSubmissionQueue::default());
    let held = prepared(10, 40);
    reserve_hold(&limiter, 10, 40);
    hold_at(
        &registry,
        context(10),
        held.clone(),
        ApprovalSequence(10),
        ApprovalSequence(40),
        TimestampSeconds(10),
    )
    .unwrap_or_else(|error| panic!("hold: {error:?}"));
    let barrier = Arc::new(Barrier::new(2));

    let approve_worker = {
        let registry = Arc::clone(&registry);
        let limiter = Arc::clone(&limiter);
        let expiry = Arc::clone(&expiry);
        let queue = Arc::clone(&queue);
        let barrier = Arc::clone(&barrier);
        let held = held.clone();
        thread::spawn(move || {
            let service = ApprovalService::new(&registry, &limiter, &expiry);
            barrier.wait();
            service.approve(
                DecisionRequest {
                    tenant: &tenant(),
                    approval_id: [10; 32],
                    idempotency_key: &key("approve-10"),
                    approver: approver("human:approve"),
                    current_sequence: 11,
                    current_time: TimestampSeconds(11),
                },
                &held,
                &queue,
            )
        })
    };
    let reject_worker = {
        let registry = Arc::clone(&registry);
        let limiter = Arc::clone(&limiter);
        let expiry = Arc::clone(&expiry);
        let barrier = Arc::clone(&barrier);
        thread::spawn(move || {
            let service = ApprovalService::new(&registry, &limiter, &expiry);
            barrier.wait();
            service.reject(DecisionRequest {
                tenant: &tenant(),
                approval_id: [10; 32],
                idempotency_key: &key("reject-10"),
                approver: approver("human:reject"),
                current_sequence: 11,
                current_time: TimestampSeconds(11),
            })
        })
    };

    let decisions = [
        approve_worker
            .join()
            .unwrap_or_else(|_| panic!("approve thread panicked"))
            .unwrap_or_else(|error| panic!("approve: {error:?}")),
        reject_worker
            .join()
            .unwrap_or_else(|_| panic!("reject thread panicked"))
            .unwrap_or_else(|error| panic!("reject: {error:?}")),
    ];
    let winner = decisions
        .iter()
        .find(|decision| decision.outcome != ApprovalOutcome::Conflict)
        .unwrap_or_else(|| panic!("winning decision missing"));
    let loser = decisions
        .iter()
        .find(|decision| decision.outcome == ApprovalOutcome::Conflict)
        .unwrap_or_else(|| panic!("conflicting decision missing"));
    assert!(matches!(
        winner.outcome,
        ApprovalOutcome::Granted | ApprovalOutcome::Rejected
    ));
    assert_eq!(loser.winning_outcome, Some(winner.outcome));
    assert_eq!(
        usize::from(winner.outcome == ApprovalOutcome::Granted),
        queue.len()
    );
}

#[test]
fn decision_record_survives_process_restart_and_replays_original_outcome() {
    let directory = Directory::new("decision-restart");
    let tenant = tenant();
    let held = prepared(11, 40);
    let first_reference = {
        let registry = ApprovalRegistry::default();
        let limiter = limiter();
        let expiry = ApprovalExpiry::open(directory.path())
            .unwrap_or_else(|error| panic!("first open: {error:?}"));
        hold_at(
            &registry,
            context(11),
            held.clone(),
            ApprovalSequence(10),
            ApprovalSequence(40),
            TimestampSeconds(10),
        )
        .unwrap_or_else(|error| panic!("first hold: {error:?}"));
        let service = ApprovalService::new(&registry, &limiter, &expiry);
        service
            .approve(
                DecisionRequest {
                    tenant: &tenant,
                    approval_id: [11; 32],
                    idempotency_key: &key("approve-11"),
                    approver: approver("human:first"),
                    current_sequence: 11,
                    current_time: TimestampSeconds(11),
                },
                &held,
                &ApprovalSubmissionQueue::default(),
            )
            .unwrap_or_else(|error| panic!("first approve: {error:?}"))
            .submission_ref
    };

    let registry = ApprovalRegistry::default();
    let limiter = limiter();
    let expiry =
        ApprovalExpiry::open(directory.path()).unwrap_or_else(|error| panic!("reopen: {error:?}"));
    hold_at(
        &registry,
        context(11),
        held.clone(),
        ApprovalSequence(10),
        ApprovalSequence(40),
        TimestampSeconds(10),
    )
    .unwrap_or_else(|error| panic!("restored hold: {error:?}"));
    let queue = ApprovalSubmissionQueue::default();
    let service = ApprovalService::new(&registry, &limiter, &expiry);
    let replay = service
        .approve(
            DecisionRequest {
                tenant: &tenant,
                approval_id: [11; 32],
                idempotency_key: &key("approve-11"),
                approver: approver("human:retry"),
                current_sequence: 12,
                current_time: TimestampSeconds(11),
            },
            &held,
            &queue,
        )
        .unwrap_or_else(|error| panic!("replay: {error:?}"));
    assert_eq!(replay.outcome, ApprovalOutcome::Granted);
    assert_eq!(replay.submission_ref, first_reference);
    assert!(queue.is_empty(), "idempotent replay must not submit twice");
}

#[test]
fn downtime_expiry_is_recovered_once_and_releases_the_reservation() {
    let directory = Directory::new("expiry-restart");
    let tenant = tenant();
    {
        let registry = ApprovalRegistry::default();
        let limiter = limiter();
        let expiry = ApprovalExpiry::open(directory.path())
            .unwrap_or_else(|error| panic!("first open: {error:?}"));
        hold_at(
            &registry,
            context(12),
            prepared(12, 20),
            ApprovalSequence(10),
            ApprovalSequence(20),
            TimestampSeconds(10),
        )
        .unwrap_or_else(|error| panic!("hold: {error:?}"));
        let service = ApprovalService::new(&registry, &limiter, &expiry);
        service
            .get(&tenant, [12; 32], 11, TimestampSeconds(11))
            .unwrap_or_else(|error| panic!("persist pending hold: {error:?}"));
    }

    let recovered_limiter = limiter();
    reserve_hold(&recovered_limiter, 12, 20);
    let recovered =
        ApprovalExpiry::open(directory.path()).unwrap_or_else(|error| panic!("reopen: {error:?}"));
    let decision = recovered
        .recover(&tenant, [12; 32], 20, 20, &recovered_limiter)
        .unwrap_or_else(|error| panic!("recover: {error:?}"));
    assert_eq!(decision.outcome, ApprovalOutcome::Expired);
    assert_eq!(recovered_limiter.held_reservations(), Ok(0));
    let repeated = recovered
        .recover(&tenant, [12; 32], 20, 21, &recovered_limiter)
        .unwrap_or_else(|error| panic!("repeat recovery: {error:?}"));
    assert_eq!(repeated.outcome, ApprovalOutcome::Expired);
}

#[test]
fn hold_expiry_cannot_outlive_the_prepared_activity() {
    let registry = ApprovalRegistry::default();
    assert_eq!(
        hold_at(
            &registry,
            context(13),
            prepared(13, 30),
            ApprovalSequence(10),
            ApprovalSequence(31),
            TimestampSeconds(30),
        ),
        Err(ApprovalError::InvalidWindow)
    );
}
