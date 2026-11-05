use std::fmt::Debug;
use std::fs;
use std::os::unix::fs::DirBuilderExt;

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
    ApprovalEvent, ApprovalNotification, Inbox, VerifiedBudgetAfter,
};
use layerx_human_service::audit::AuditChain;
use layerx_human_service::notify::{Channel, Delivery, DetailLevel, Dispatcher, NotificationClass};
use layerx_human_service::store::{PrincipalScope, PrincipalStore};
use layerx_human_service::trace::TraceId;
use layerx_human_test_support as support;
use layerx_types::ids::Did;
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};

const ACTOR: &str = "0404040404040404040404040404040404040404";

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("real approval path failed: {error:?}"))
}

fn tenant() -> TenantId {
    checked(TenantId::new("tenant-notification-binding"))
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
    service: ApprovalService<'a>,
    queue: &'a ApprovalSubmissionQueue,
}

impl ApprovalBoundary for AgentdBoundary<'_> {
    fn approval(
        &mut self,
        approval_id: [u8; 32],
        at_sequence: u64,
    ) -> Result<AgentApprovalRecord, ApprovalBoundaryError> {
        let record = self
            .service
            .get(
                &tenant(),
                approval_id,
                at_sequence,
                TimestampSeconds(at_sequence),
            )
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

fn assert_payload(
    scope: &PrincipalScope<'_>,
    deliveries: &[Delivery],
    detail: DetailLevel,
    amount: u128,
    asset: &str,
) {
    assert_eq!(deliveries.len(), 3);
    for channel in Channel::ALL {
        assert_eq!(
            deliveries.iter().filter(|d| d.channel() == channel).count(),
            1
        );
    }
    for delivery in deliveries {
        assert_eq!(delivery.class(), NotificationClass::ApprovalWaiting);
        let binding = checked(ApprovalNotification::for_delivery(scope, delivery));
        assert_eq!(binding.amount(), amount);
        assert_eq!(binding.asset(), asset);
        let full = detail == DetailLevel::Full;
        if delivery.channel() == Channel::Email {
            if full {
                assert!(delivery
                    .payload()
                    .contains(&format!("\r\nAmount: {amount}\r\n")));
                assert!(delivery
                    .payload()
                    .contains(&format!("\r\nAsset: {asset}\r\n")));
                assert!(delivery.payload().contains("\r\nAgent: agt_"));
            } else {
                assert!(!delivery.payload().contains("Amount:"));
                assert!(!delivery.payload().contains("Asset:"));
                assert!(!delivery.payload().contains("Agent:"));
                assert!(!delivery.payload().contains(&amount.to_string()));
                assert!(!delivery.payload().contains(asset));
            }
        } else {
            let payload: serde_json::Value = checked(serde_json::from_str(delivery.payload()));
            assert_eq!(payload["deep_link"], delivery.deep_link());
            assert!(payload.get("money").is_none());
            assert!(payload.get("currency").is_none());
            if full {
                assert_eq!(payload["amount"], amount.to_string());
                assert_eq!(payload["asset"], asset);
                assert!(payload["agent_id"]
                    .as_str()
                    .is_some_and(|id| id.starts_with("agt_")));
            } else {
                assert!(payload.get("amount").is_none());
                assert!(payload.get("asset").is_none());
                assert!(payload.get("agent_id").is_none());
            }
        }
    }
}

fn round_trip(asset: &str, amount: u128) {
    let root = support::directory("repair-22-31-523");
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
    let mut boundary = AgentdBoundary {
        service: ApprovalService::new(&registry, &limiter, &expiry),
        queue: &queue,
    };
    let mut ingestor = checked(EventIngestor::open(
        checked(AgentStore::open(&stream_root)),
        tenant(),
        16,
        10,
    ));
    let mut agent_audit = checked(Log::open(&stream_root, &tenant()));
    let human_root = root.join("human");
    let map = support::tenancy(&[("alice", "tenant-notification-binding")]);
    let retention = support::retention_uniform(1_000);
    let (mut store, tenancy_digest) = support::install_and_open(&human_root, &map, retention);
    let trace = TraceId::mint([7; 16]);
    let mut scope = checked(store.principal(&support::principal("alice")));
    let mut audit = checked(AuditChain::open(&scope));
    let mut inbox = checked(Inbox::new(10, 3));
    let mut events = Vec::new();
    let mut groups = Vec::new();
    for (id, detail) in [
        DetailLevel::Full,
        DetailLevel::Summary,
        DetailLevel::Minimal,
    ]
    .into_iter()
    .enumerate()
    {
        let id = u8::try_from(id).expect("three detail levels");
        let sequence = 10 + u64::from(id);
        let held = prepared(id, asset, amount);
        let context = ApprovalContext {
            tenant: tenant(),
            agent: checked(Did::new(ACTOR.as_bytes())),
            actor_address: [4; 20],
            session: SessionId([2; 32]),
            capability: CapabilityId([3; 32]),
            policy_version: "notification-binding-policy-v1".to_owned(),
            request_id: [id; 32],
        };
        let ticket = checked(hold_at(
            &registry,
            context.clone(),
            held,
            ApprovalSequence(sequence),
            ApprovalSequence(50),
            TimestampSeconds(sequence),
        ));
        let record = checked(boundary.service.get(
            &tenant(),
            ticket.hold_id,
            sequence,
            TimestampSeconds(sequence),
        ));
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
                observed_at_ms: sequence * 1_000,
            },
        ));
        let event = checked(ApprovalEvent::decode_agent_stream(
            &emission.canonical_event_bytes,
            sequence,
        ));
        let previous = checked(Dispatcher::deliveries(&scope));
        let mut preferences = checked(Dispatcher::preferences(&scope));
        preferences.set_detail(detail);
        checked(Dispatcher::update_preferences(
            &mut scope,
            sequence,
            &preferences,
        ));
        checked(inbox.consume(&[event], &mut boundary, &mut scope, &mut audit, &trace));
        assert_eq!(
            checked(inbox.snapshot(sequence)).awaiting_count(),
            usize::from(id) + 1
        );
        let deliveries = checked(Dispatcher::deliveries(&scope));
        assert_eq!(deliveries.len(), previous.len() + 3);
        let added: Vec<_> = deliveries
            .into_iter()
            .filter(|d| !previous.contains(d))
            .collect();
        assert_payload(&scope, &added, detail, amount, asset);
        for delivery in &added {
            assert_eq!(
                checked(ApprovalNotification::for_delivery(&scope, delivery)).disclosure_digest(),
                record.canonical_bytes_digest
            );
        }
        groups.push((detail, added));
        events.push((sequence, emission.canonical_event_bytes));
    }
    let before = checked(Dispatcher::deliveries(&scope));
    let audit_before = checked(audit.entries(&scope)).len();
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
    let mut store = checked(PrincipalStore::open(&human_root, retention, tenancy_digest));
    let mut scope = checked(store.principal(&support::principal("alice")));
    assert_eq!(checked(Dispatcher::deliveries(&scope)), before);
    // Changing preferences cannot rebuild a persisted full-detail batch on replay.
    let mut preferences = checked(Dispatcher::preferences(&scope));
    preferences.set_detail(DetailLevel::Summary);
    checked(Dispatcher::update_preferences(&mut scope, 13, &preferences));
    let mut audit = checked(AuditChain::open(&scope));
    let mut inbox = checked(Inbox::new(10, 3));
    for (sequence, canonical) in events {
        let retained = checked(ingestor.retained_event(sequence));
        assert_eq!(retained.canonical_bytes, canonical);
        let replay = checked(ApprovalEvent::decode_agent_stream(
            &retained.canonical_bytes,
            sequence,
        ));
        checked(inbox.consume(&[replay], &mut boundary, &mut scope, &mut audit, &trace));
    }
    assert_eq!(checked(Dispatcher::deliveries(&scope)), before);
    assert_eq!(checked(audit.entries(&scope)).len(), audit_before);
    assert_eq!(checked(inbox.snapshot(12)).awaiting_count(), 3);
    for (detail, deliveries) in groups {
        assert_payload(&scope, &deliveries, detail, amount, asset);
    }
    drop(scope);
    drop(store);
    drop(ingestor);
    drop(boundary);
    drop(expiry);
    checked(fs::remove_dir_all(root));
}

#[test]
fn full_detail_approval_carries_maximum_base_units_and_bank_asset_after_replay() {
    round_trip("ibc/AbCd0123456789", u128::MAX);
}

#[test]
fn full_detail_approval_carries_native_asset_without_currency_metadata() {
    round_trip(&hex([0xab; 32]), (1_u128 << 100) + 25);
}

#[test]
fn full_detail_approval_escapes_stable_asset_id_in_json() {
    round_trip("bank/\"quoted\"\\asset-λ", u128::MAX - 1);
}
