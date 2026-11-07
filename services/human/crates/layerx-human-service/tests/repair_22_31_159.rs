use std::fmt::Debug;
use std::fs;
use std::os::unix::fs::DirBuilderExt;
use std::time::Duration;

use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
    SigningPreimage,
};
use layerx_agent_api::track::{EvidenceRef, SubmissionRef, SubmissionState, TrackedSubmission};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::approval::{
    ApprovalEventKind, ApprovalEvents, ApprovalExpiry, ApprovalLifecycle, ApprovalService,
    ApprovalSubmissionQueue,
};
use layerx_agentd::audit::{Coverage, Log};
use layerx_agentd::budget::BudgetLimiter;
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::events::EventIngestor;
use layerx_agentd::policy::approval::{
    hold_at, ApprovalContext, ApprovalRegistry, ApprovalSequence, ApprovalState,
};
use layerx_agentd::session::SessionId;
use layerx_agentd::store::{Store as AgentStore, TenantId};
use layerx_human_service::approvals::{
    AgentApprovalRecord, AgentApprovalState, ApprovalBoundary, ApprovalBoundaryError,
    ApprovalEvent, ApprovalNotification, Inbox, InboxError, InboxState, VerifiedBudgetAfter,
};
use layerx_human_service::audit::AuditChain;
use layerx_human_service::notify::{
    ActiveShell, Channel, DeepLinks, DetailLevel, Dispatcher, LandingState, Money,
    NotificationClass, NotifyError, SubjectState, Surface,
};
use layerx_human_service::store::{PrincipalScope, PrincipalStore, Table};
use layerx_human_service::trace::TraceId;
use layerx_human_test_support as support;
use layerx_types::clock::Clock as _;
use layerx_types::ids::Did;
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};

const ACTOR: &str = "0404040404040404040404040404040404040404";

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("real approval path failed: {error:?}"))
}

fn tenant() -> TenantId {
    checked(TenantId::new("tenant-asset-domain"))
}

fn prepared(id: u8, asset: &str, amount: u128) -> Prepared {
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
            [vec![0; 16], amount.to_be_bytes().to_vec()].concat(),
        ]
        .concat(),
    };
    let bytes = call.encode_unsigned();
    let digest = Sha256::digest(&bytes).into();
    let counterparty = checked(AgentDid::new("0505050505050505050505050505050505050505"));
    Prepared {
        preparation_ref: checked(PreparationRef::new(format!("asset-preparation-{id}"))),
        unsigned_canonical_bytes: checked(CanonicalBytes::new(bytes)),
        signing_preimage: checked(SigningPreimage::new(call.signing_hash().to_vec())),
        disclosure: Disclosure {
            canonical_digest: digest,
            activity_type: ActivityType(5),
            actor: checked(AgentDid::new(ACTOR)),
            authority: checked(AuthorityRef::new("session-key")),
            counterparties: ExplicitSet::allow(vec![counterparty.clone()]),
            amounts: ExplicitSet::allow(vec![DisclosedAmount {
                counterparty,
                amount: Amount(amount),
            }]),
            asset: checked(Asset::new(asset)),
            fee_limit: Amount(200_000),
            expiry: TimestampSeconds(100),
            idempotency_key: checked(IdempotencyRef::new(format!("asset-movement-{id}"))),
        },
        expiry: TimestampSeconds(100),
    }
}

/// Adapts the real daemon approval service in-process, as tests/approvals.rs does.
struct AgentdBoundary<'a> {
    clock: support::StepClock,
    service: ApprovalService<'a>,
    queue: &'a ApprovalSubmissionQueue,
}

impl AgentdBoundary<'_> {
    fn decision_time(&self) -> Result<TimestampSeconds, ApprovalBoundaryError> {
        let reading = self
            .clock
            .sample(Duration::from_secs(1))
            .map_err(|_| ApprovalBoundaryError::Unavailable)?;
        if reading.generation == [0; 16] {
            return Err(ApprovalBoundaryError::Unavailable);
        }
        Ok(TimestampSeconds(reading.unix_seconds()))
    }
}

impl ApprovalBoundary for AgentdBoundary<'_> {
    fn approval(
        &mut self,
        approval_id: [u8; 32],
        at_sequence: u64,
    ) -> Result<AgentApprovalRecord, ApprovalBoundaryError> {
        let record = self
            .service
            .get(&tenant(), approval_id, at_sequence, self.decision_time()?)
            .map_err(|_| ApprovalBoundaryError::Unavailable)?;
        let state = match record.state {
            ApprovalState::AwaitingApproval => AgentApprovalState::AwaitingApproval,
            ApprovalState::Approved => AgentApprovalState::Approved {
                submission_ref: record
                    .submission_ref
                    .ok_or(ApprovalBoundaryError::Corrupt)?,
            },
            ApprovalState::Rejected => AgentApprovalState::Rejected,
            ApprovalState::Expired => AgentApprovalState::Expired,
            ApprovalState::Defective => AgentApprovalState::Defective,
        };
        Ok(AgentApprovalRecord {
            approval_id: record.approval_id,
            held_activity: record.held_activity,
            canonical_bytes_digest: record.canonical_bytes_digest,
            hold_reason_code: record.hold_reason.code.to_owned(),
            hold_reason: record.hold_reason.message.to_owned(),
            created_at_sequence: record.created_at_sequence,
            expires_at_sequence: record.expires_at_sequence,
            state,
        })
    }

    fn verified_budget_after(
        &mut self,
        hold: &AgentApprovalRecord,
        at_sequence: u64,
    ) -> Result<VerifiedBudgetAfter, ApprovalBoundaryError> {
        let account = [0x41; 32];
        let asset: [u8; 32] = Sha256::digest(hold.held_activity.asset.as_str().as_bytes()).into();
        let mut canonical = account.to_vec();
        canonical.extend_from_slice(&asset);
        canonical.extend_from_slice(&975_u128.to_be_bytes());
        let authority =
            support::evidence_verifier(&ed25519_dalek::SigningKey::from_bytes(&[0x84; 32]));
        let state = authority
            .verify_state(&support::raw_state_leaf(canonical, at_sequence))
            .map_err(|_| ApprovalBoundaryError::VerificationFailed)?;
        let canonical: [u8; 80] = state
            .canonical_state()
            .try_into()
            .map_err(|_| ApprovalBoundaryError::Corrupt)?;
        if state.level() != layerx_types::verify::VerificationLevel::STATE_PROVEN
            || canonical[..32] != account
            || canonical[32..64] != asset
        {
            return Err(ApprovalBoundaryError::VerificationFailed);
        }
        Ok(VerifiedBudgetAfter {
            remaining: u128::from_be_bytes(
                canonical[64..]
                    .try_into()
                    .map_err(|_| ApprovalBoundaryError::Corrupt)?,
            ),
            rung: Rung::Committed,
            evidence_digest: Sha256::digest(canonical).into(),
            observed_at_sequence: state.observed_head_sequence(),
        })
    }

    fn track_released(
        &mut self,
        submission_ref: [u8; 32],
    ) -> Result<TrackedSubmission, ApprovalBoundaryError> {
        if self.queue.prepared(submission_ref).is_none() {
            return Err(ApprovalBoundaryError::Corrupt);
        }
        Ok(TrackedSubmission {
            submission_ref: SubmissionRef::new(hex(submission_ref))
                .map_err(|_| ApprovalBoundaryError::Corrupt)?,
            state: SubmissionState::Queued,
            evidence: vec![EvidenceRef {
                kind: "approval-release".to_owned(),
                digest: submission_ref,
            }],
            rung: Rung::Receipted,
            transitions: Vec::new(),
        })
    }
}

fn hex(bytes: [u8; 32]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn assert_notification_path(
    scope: &PrincipalScope<'_>,
    inbox: &Inbox,
    amount: u128,
    asset: &str,
    digest: [u8; 32],
) {
    let snapshot = checked(inbox.snapshot(10));
    assert_eq!(snapshot.awaiting_count(), 1);
    assert_eq!(snapshot.observed_sequence(), 10);
    assert_eq!(snapshot.items().len(), 1);
    let item = snapshot.items()[0];
    assert_eq!(item.amount(), amount);
    assert_eq!(item.asset(), asset);
    assert_eq!(item.disclosure_digest(), digest);
    assert_eq!(item.state(), &InboxState::AwaitingApproval);
    assert_eq!(inbox.next_sequence(), 11);
    let deliveries = checked(Dispatcher::deliveries(scope));
    assert_eq!(deliveries.len(), 3);
    for channel in [Channel::Push, Channel::Email, Channel::InApp] {
        assert_eq!(
            deliveries
                .iter()
                .filter(|delivery| delivery.channel() == channel)
                .count(),
            1
        );
    }
    for delivery in deliveries {
        assert_eq!(delivery.class(), NotificationClass::ApprovalWaiting);
        let value = checked(ApprovalNotification::for_delivery(scope, &delivery));
        assert_eq!(value.approval_id(), item.approval_id());
        assert_eq!(value.amount(), amount);
        assert_eq!(value.asset(), asset);
        assert_eq!(value.disclosure_digest(), digest);
        for shell in [ActiveShell::Desktop, ActiveShell::Mobile] {
            let landing = checked(DeepLinks::resolve(
                &delivery,
                shell,
                SubjectState::Approval {
                    approval_id: item.approval_id(),
                    state: item.state(),
                },
            ));
            assert_eq!(landing.surface(), Surface::Approval);
            assert_eq!(landing.state(), LandingState::Actionable);
            assert_eq!(landing.path(), delivery.deep_link());
        }
        if channel_is_json(delivery.channel()) {
            let payload: serde_json::Value = checked(serde_json::from_str(delivery.payload()));
            assert_eq!(payload["deep_link"], delivery.deep_link());
            assert!(
                payload.get("money").is_none(),
                "asset IDs are not display currencies"
            );
        } else {
            if delivery.payload().contains("Amount:") {
                assert!(delivery
                    .payload()
                    .contains(&format!("\r\nAmount: {amount}\r\nAsset: {asset}\r\n")));
            }
            assert!(!delivery.payload().contains(&format!("{amount} {asset}")));
        }
    }
}

fn channel_is_json(channel: Channel) -> bool {
    matches!(channel, Channel::Push | Channel::InApp)
}

fn round_trip(asset: &str, amount: u128) {
    let root = support::directory("repair-22-31-159");
    let stream_root = root.join("stream");
    checked(
        fs::DirBuilder::new()
            .recursive(true)
            .mode(0o700)
            .create(&stream_root),
    );
    let registry = ApprovalRegistry::default();
    let limiter = checked(BudgetLimiter::new(Vec::new()));
    let expiry = checked(ApprovalExpiry::open(root.join("approval")));
    let queue = ApprovalSubmissionQueue::default();
    let held = prepared(1, asset, amount);
    let context = ApprovalContext {
        tenant: tenant(),
        agent: checked(Did::new(ACTOR.as_bytes())),
        actor_address: [4; 20],
        session: SessionId([2; 32]),
        capability: CapabilityId([3; 32]),
        policy_version: "asset-policy-v1".to_owned(),
        request_id: [1; 32],
    };
    let mut boundary = AgentdBoundary {
        clock: support::StepClock::new(10_000),
        service: ApprovalService::new(&registry, &limiter, &expiry),
        queue: &queue,
    };
    let ticket = checked(hold_at(
        &registry,
        context.clone(),
        held.clone(),
        ApprovalSequence(10),
        ApprovalSequence(50),
        checked(boundary.decision_time()),
    ));
    let record = checked(boundary.service.get(
        &tenant(),
        ticket.hold_id,
        10,
        checked(boundary.decision_time()),
    ));
    assert_eq!(record.held_activity.asset.as_str(), asset);
    assert_eq!(record.held_activity.amounts.values()[0].amount.0, amount);

    let mut ingestor = checked(EventIngestor::open(
        checked(AgentStore::open(&stream_root)),
        tenant(),
        16,
        10,
    ));
    let mut agent_audit = checked(Log::open(&stream_root, &tenant()));
    let emission = checked(ApprovalEvents::emit(
        &mut ingestor,
        &mut agent_audit,
        &mut Coverage::default(),
        &ApprovalLifecycle {
            tenant: context.tenant,
            agent: context.agent,
            session: context.session,
            capability: context.capability.0,
            policy_version: context.policy_version,
            approval_id: record.approval_id,
            canonical_digest: record.canonical_bytes_digest,
            activity_type: record.held_activity.activity_type.0,
            asset: record.held_activity.asset.as_str().to_owned(),
            kind: ApprovalEventKind::Created,
            principal: None,
            observed_at_ms: 10_000,
        },
    ));
    let event = checked(ApprovalEvent::decode_agent_stream(
        &emission.canonical_event_bytes,
        10,
    ));
    let human_root = root.join("human");
    let map = support::tenancy(&[("alice", "tenant-asset-domain"), ("bob", "tenant-other")]);
    let retention = support::retention_uniform(1_000);
    let (mut store, tenancy_digest) = support::install_and_open(&human_root, &map, retention);
    let trace = TraceId::mint([7; 16]);
    let mut scope = checked(store.principal(&support::principal("alice")));
    let mut audit = checked(AuditChain::open(&scope));
    let mut preferences = checked(Dispatcher::preferences(&scope));
    preferences.set_detail(DetailLevel::Full);
    checked(Dispatcher::update_preferences(&mut scope, 9, &preferences));
    let mut inbox = checked(Inbox::new(10, 3));
    checked(inbox.consume(&[event], &mut boundary, &mut scope, &mut audit, &trace));
    assert_notification_path(&scope, &inbox, amount, asset, record.canonical_bytes_digest);
    let before = checked(Dispatcher::deliveries(&scope));
    let audit_before = checked(audit.entries(&scope)).len();
    let notification_rows_before = scope.keys(Table::Notifications).len();
    drop(scope);
    drop(store);
    drop(inbox);
    drop(audit);
    drop(ingestor);
    drop(agent_audit);

    let ingestor = checked(EventIngestor::open(
        checked(AgentStore::open(&stream_root)),
        tenant(),
        16,
        10,
    ));
    let retained = checked(ingestor.retained_event(10));
    assert_eq!(retained.canonical_bytes, emission.canonical_event_bytes);
    assert_eq!(retained.attributes.asset, asset);
    let replay = checked(ApprovalEvent::decode_agent_stream(
        &retained.canonical_bytes,
        10,
    ));
    let mut store = checked(PrincipalStore::open(&human_root, retention, tenancy_digest));
    let mut scope = checked(store.principal(&support::principal("alice")));
    assert_eq!(checked(Dispatcher::deliveries(&scope)), before);
    for delivery in &before {
        let value = checked(ApprovalNotification::for_delivery(&scope, delivery));
        assert_eq!(value.amount(), amount);
        assert_eq!(value.asset(), asset);
    }
    let mut audit = checked(AuditChain::open(&scope));
    let mut inbox = checked(Inbox::new(10, 3));
    checked(inbox.consume(&[replay], &mut boundary, &mut scope, &mut audit, &trace));
    assert_notification_path(&scope, &inbox, amount, asset, record.canonical_bytes_digest);
    assert_eq!(checked(Dispatcher::deliveries(&scope)), before);
    assert_eq!(checked(audit.entries(&scope)).len(), audit_before);
    assert_eq!(
        scope.keys(Table::Notifications).len(),
        notification_rows_before
    );
    assert!(matches!(
        inbox.consume(&[replay], &mut boundary, &mut scope, &mut audit, &trace),
        Err(InboxError::EventOrder {
            expected: 11,
            actual: 10
        })
    ));
    drop(scope);
    let bob = checked(store.principal(&support::principal("bob")));
    for delivery in &before {
        assert!(matches!(
            ApprovalNotification::for_delivery(&bob, delivery),
            Err(NotifyError::NotificationNotFound)
        ));
    }
    drop(bob);
    drop(store);
    drop(ingestor);
    drop(boundary);
    drop(expiry);
    checked(fs::remove_dir_all(root));
}

#[test]
fn approval_asset_currency_domain_bank_denoms_route_and_recover() {
    for asset in ["upax", "ibc/AbCd0123456789", "bank/\"quoted\"\\asset\nλ"] {
        assert!(Money::new(u128::MAX, asset).is_err());
        round_trip(asset, u128::MAX);
    }
}

#[test]
fn approval_asset_currency_domain_native_ids_route_and_recover() {
    let asset = hex([0xab; 32]);
    assert!(Money::new(25, &asset).is_err());
    round_trip(&asset, (1_u128 << 100) + 25);
}

#[test]
fn approval_asset_currency_domain_does_not_infer_display_metadata() {
    assert!(Money::new(25, "LXP").is_ok());
    round_trip("LXP", 25);
}
