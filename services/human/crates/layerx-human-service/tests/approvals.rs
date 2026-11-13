use layerx_agent_api::track::SubmissionState;
use layerx_human_test_support as support;

#[allow(dead_code)]
pub(crate) mod owner {
    include!("support/approval_owner.rs");
}

#[allow(unused_imports)]
use owner::hex;

use layerx_agentd::approval::{
    ApprovalEventKind as AgentEventKind, ApprovalEvents, ApprovalLifecycle,
};
use layerx_agentd::audit::{Coverage, Log};
use layerx_agentd::events::EventIngestor;
use layerx_agentd::session::SessionId;
use layerx_human_service::approvals::{
    AgentApprovalState, AgentDecisionStatus, ApprovalBoundary, ApprovalEvent, ApprovalNotification,
    Inbox, InboxState,
};
use layerx_human_service::audit::AuditChain;
use layerx_human_service::notify::{DetailLevel, Dispatcher};
use layerx_human_service::trace::TraceId;
use layerx_types::ids::Did;
use layerx_types::rung::Rung;
use std::os::unix::fs::DirBuilderExt as _;

#[track_caller]
fn checked<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    match value {
        Ok(value) => value,
        Err(error) => panic!("approval integration: {error:?}"),
    }
}

fn emit(
    fixture: &owner::Fixture,
    ingestor: &mut EventIngestor,
    audit: &mut Log,
    coverage: &mut Coverage,
    kind: AgentEventKind,
    id: u8,
    observed_at: u64,
) -> ApprovalEvent {
    let prepared = &fixture.prepared[&id];
    let lifecycle = ApprovalLifecycle {
        tenant: fixture.tenant.clone(),
        agent: checked(Did::new(prepared.disclosure.actor.as_str().as_bytes())),
        session: SessionId([7; 32]),
        capability: [6; 32],
        policy_version: "approval-actor-v1".to_owned(),
        approval_id: [id; 32],
        canonical_digest: prepared.disclosure.canonical_digest,
        activity_type: prepared.disclosure.activity_type.0,
        asset: prepared.disclosure.asset.as_str().to_owned(),
        kind,
        principal: matches!(kind, AgentEventKind::Granted | AgentEventKind::Rejected)
            .then(|| fixture.peer.principal.clone()),
        observed_at_ms: observed_at,
    };
    let emission = checked(ApprovalEvents::emit(ingestor, audit, coverage, &lifecycle));
    checked(ApprovalEvent::decode_agent_stream(
        &emission.canonical_event_bytes,
        observed_at,
    ))
}

fn assert_listed_holds(
    fixture: &owner::Fixture,
    boundary: &mut layerx_human_service::server::agent_runtime::AgentRuntime,
    sequence: u64,
) -> layerx_human_service::approvals::AgentApprovalRecord {
    let first_page = checked(boundary.approval_list(sequence, None, 2));
    assert_eq!(first_page.approvals.len(), 2);
    assert_eq!(first_page.next_cursor, Some([2; 32]));
    let last_page = checked(boundary.approval_list(sequence, first_page.next_cursor, 2));
    assert_eq!(last_page.approvals.len(), 1);
    assert_eq!(last_page.next_cursor, None);
    for (id, record) in [1, 2, 3]
        .into_iter()
        .zip(first_page.approvals.iter().chain(&last_page.approvals))
    {
        assert_eq!(record.approval_id, [id; 32]);
        assert_eq!(record.held_activity, fixture.prepared[&id].disclosure);
        assert_eq!(
            record.canonical_bytes_digest,
            fixture.prepared[&id].disclosure.canonical_digest
        );
        assert_eq!(record.state, AgentApprovalState::AwaitingApproval);
    }
    let first = checked(boundary.approval_get([1; 32], sequence));
    fixture.assert_budget(&first, fixture.protocol_remaining - 75, sequence);
    let mut changed = first.canonical_bytes_digest;
    changed[0] ^= 1;
    assert!(boundary
        .approval_decide(true, [1; 32], changed, "wrong-digest", sequence)
        .is_err());
    assert!(boundary
        .verified_budget_after(&first, sequence - 1)
        .is_err());
    fixture.assert_reservations(3, 75, 0);

    first
}

fn assert_pending_inbox(
    inbox: &Inbox,
    scope: &layerx_human_service::store::PrincipalScope<'_>,
    first: &layerx_human_service::approvals::AgentApprovalRecord,
    sequence: u64,
) {
    let pending = checked(inbox.snapshot(sequence));
    assert_eq!(pending.awaiting_count(), 1);
    let item = pending.items()[0];
    assert_eq!(item.amount(), 25);
    assert_eq!(item.agent(), first.held_activity.actor.as_str());
    assert_eq!(
        item.counterparty(),
        first.held_activity.counterparties.values()[0].as_str()
    );
    assert_eq!(item.asset(), first.held_activity.asset.as_str());
    assert_eq!(item.fee_limit(), first.held_activity.fee_limit.0);
    assert_eq!(item.activity_expiry(), first.held_activity.expiry.get());
    assert_eq!(item.disclosure_digest(), first.canonical_bytes_digest);
    assert_eq!(item.budget_after().remaining, 2);
    assert_eq!(item.budget_after().rung, Rung::Committed);
    assert_eq!(item.remaining(sequence + 2), 18);
    let deliveries = checked(Dispatcher::deliveries(scope));
    assert_eq!(deliveries.len(), 3);
    assert!(deliveries
        .iter()
        .all(|delivery| delivery.deep_link().contains(item.approval_id().as_str())));
    assert!(deliveries
        .iter()
        .all(|delivery| delivery.payload().contains("agt_") && delivery.payload().contains("25")));

    for delivery in &deliveries {
        let notification = checked(ApprovalNotification::for_delivery(scope, delivery));
        assert_eq!(notification.amount(), 25);
        assert_eq!(notification.asset(), item.asset());
    }
}

struct ApprovalTerminal<'a, 'scope> {
    fixture: &'a owner::Fixture,
    boundary: &'a mut layerx_human_service::server::agent_runtime::AgentRuntime,
    inbox: &'a mut Inbox,
    ingestor: &'a mut EventIngestor,
    agent_audit: &'a mut Log,
    coverage: &'a mut Coverage,
    scope: &'a mut layerx_human_service::store::PrincipalScope<'scope>,
    audit: &'a mut AuditChain,
    trace: &'a TraceId,
}

fn assert_terminal_lifecycle(
    context: ApprovalTerminal<'_, '_>,
    sequence: u64,
    reference: [u8; 32],
) {
    let ApprovalTerminal {
        fixture,
        boundary,
        inbox,
        ingestor,
        agent_audit,
        coverage,
        scope,
        audit,
        trace,
    } = context;
    for (offset, id, kind) in [
        (1, 2, AgentEventKind::Created),
        (2, 3, AgentEventKind::Created),
        (3, 2, AgentEventKind::Rejected),
        (4, 3, AgentEventKind::Expired),
    ] {
        let head = sequence + offset + 1;
        fixture.advance(head);
        if kind == AgentEventKind::Rejected {
            let decision = checked(boundary.approval_decide(
                false,
                [id; 32],
                fixture.prepared[&id].disclosure.canonical_digest,
                &format!("reject-{id}"),
                head,
            ));
            assert_eq!(decision.status, AgentDecisionStatus::Rejected);
        }
        let record = checked(boundary.approval_get([id; 32], head));
        eprintln!(
            "approval event {kind:?}: id={id} sequence={head} served_state={:?} expires_at_sequence={}",
            record.state, record.expires_at_sequence,
        );
        assert_eq!(
            record.state,
            match kind {
                AgentEventKind::Created => AgentApprovalState::AwaitingApproval,
                AgentEventKind::Rejected => AgentApprovalState::Rejected,
                AgentEventKind::Expired => AgentApprovalState::Expired,
                AgentEventKind::Granted => unreachable!(),
            }
        );
        checked(inbox.consume(
            &[emit(
                fixture,
                &mut *ingestor,
                &mut *agent_audit,
                &mut *coverage,
                kind,
                id,
                11 + offset,
            )],
            boundary,
            &mut *scope,
            &mut *audit,
            trace,
        ));
        let snapshot = checked(inbox.snapshot(head));
        assert_eq!(
            snapshot.awaiting_count(),
            match offset {
                1 | 3 => 1,
                2 => 2,
                4 => 0,
                _ => unreachable!(),
            }
        );
    }
    fixture.assert_reservations(1, 25, 1);
    fixture.assert_release(1, reference);
    let resolved = checked(inbox.snapshot(sequence + 5));
    assert_eq!(resolved.awaiting_count(), 0);
    assert!(resolved
        .items()
        .iter()
        .any(|item| matches!(item.state(), InboxState::Rejected)
            && item.state().nothing_moved() == Some("Nothing moved.")));
    assert!(resolved
        .items()
        .iter()
        .any(|item| matches!(item.state(), InboxState::Expired)
            && !item.state().can_approve()
            && item.state().nothing_moved() == Some("Nothing moved.")));
    assert_eq!(
        checked(Dispatcher::deliveries(scope)).len(),
        9,
        "each created hold notified once"
    );
    assert!(
        inbox.snapshot(sequence + 9).is_err(),
        "stale counts must not be served"
    );
    fixture.advance(sequence + 5);
    let terminal = checked(boundary.approval_get([1; 32], sequence + 5));
    fixture.assert_budget(&terminal, 52, sequence + 5);
}

fn approve_first_hold(
    fixture: &owner::Fixture,
    boundary: &mut layerx_human_service::server::agent_runtime::AgentRuntime,
    first: &layerx_human_service::approvals::AgentApprovalRecord,
    sequence: u64,
) -> [u8; 32] {
    let granted = checked(boundary.approval_decide(
        true,
        [1; 32],
        first.canonical_bytes_digest,
        "approve-one",
        sequence,
    ));
    let reference = match granted.status {
        AgentDecisionStatus::Approved {
            submission_ref: Some(reference),
        } => reference,
        status => panic!("approval did not release the held call: {status:?}"),
    };
    fixture.assert_release(1, reference);
    fixture.assert_reservations(3, 75, 1);
    reference
}

#[test]
fn real_agent_holds_drive_live_inbox_counts_notifications_and_honest_lifecycle() {
    let fixture = owner::Fixture::new(&[1, 2, 3]);
    let sequence = fixture.sequence;
    let mut boundary = fixture.client();
    fixture.assert_reservations(3, 75, 0);
    fixture.assert_foreign_refusals(1, sequence);
    let first = assert_listed_holds(&fixture, &mut boundary, sequence);
    let stream_root = fixture.root().join("stream");
    checked(std::fs::DirBuilder::new().mode(0o700).create(&stream_root));
    let stream_store = owner::open_store(&stream_root);
    let mut ingestor = checked(EventIngestor::open(
        stream_store,
        fixture.tenant.clone(),
        16,
        sequence,
    ));
    let mut agent_audit = checked(Log::open(&stream_root, &fixture.tenant));
    let mut coverage = Coverage::default();
    let map = support::tenancy(&[("alice", fixture.peer.tenant.as_str())]);
    let (mut store, _) = support::install_and_open(
        &fixture.root().join("human"),
        &map,
        support::retention_uniform(1000),
    );
    let mut scope = checked(store.principal(&support::principal("alice")));
    let mut audit = checked(AuditChain::open(&scope));
    let trace = TraceId::mint([7; 16]);
    let mut preferences = checked(Dispatcher::preferences(&scope));
    preferences.set_detail(DetailLevel::Full);
    checked(Dispatcher::update_preferences(&mut scope, 9, &preferences));
    let mut inbox = checked(Inbox::new(sequence, 3));
    checked(inbox.consume(
        &[emit(
            &fixture,
            &mut ingestor,
            &mut agent_audit,
            &mut coverage,
            AgentEventKind::Created,
            1,
            10,
        )],
        &mut boundary,
        &mut scope,
        &mut audit,
        &trace,
    ));
    assert_pending_inbox(&inbox, &scope, &first, sequence);
    let reference = approve_first_hold(&fixture, &mut boundary, &first, sequence);
    fixture.advance(sequence + 1);
    let tracking = boundary.track_released(reference);
    eprintln!(
        "approval Granted hydration: track_released({}) = {tracking:?}; next call: Inbox::consume(Granted, sequence={})",
        hex::encode(reference),
        sequence + 1,
    );
    let tracking = checked(tracking);
    assert_eq!(tracking.state, SubmissionState::Queued);
    assert_eq!(tracking.submission_ref.as_str(), hex::encode(reference));
    checked(inbox.consume(
        &[emit(
            &fixture,
            &mut ingestor,
            &mut agent_audit,
            &mut coverage,
            AgentEventKind::Granted,
            1,
            11,
        )],
        &mut boundary,
        &mut scope,
        &mut audit,
        &trace,
    ));
    let approved = checked(inbox.snapshot(sequence));
    assert_eq!(approved.awaiting_count(), 0);
    assert_eq!(
        approved.items()[0].state(),
        &InboxState::Approved { tracking }
    );

    assert_terminal_lifecycle(
        ApprovalTerminal {
            fixture: &fixture,
            boundary: &mut boundary,
            inbox: &mut inbox,
            ingestor: &mut ingestor,
            agent_audit: &mut agent_audit,
            coverage: &mut coverage,
            scope: &mut scope,
            audit: &mut audit,
            trace: &trace,
        },
        sequence,
        reference,
    );
}

pub mod require_value {
    include!("support/require_value.rs");
}
