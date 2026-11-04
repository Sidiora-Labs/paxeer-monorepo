use std::collections::{BTreeMap, BTreeSet};
use std::fmt::Debug;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};

use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
    SigningPreimage,
};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest};
use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
use layerx_agentd::identity::ProtocolAuthority;
use layerx_agentd::policy::approval::{
    decide_at, expire_at, hold_at, reserve_and_hold_at, ApprovalChoice, ApprovalContext,
    ApprovalError, ApprovalRegistry, ApprovalReservationError, ApprovalSequence, ApprovalState,
    ApproverId,
};
use layerx_agentd::policy::{
    activate_source, dry_run, dry_run_request_id, dry_run_with_context, evaluate,
    evaluate_admission_and_record, load_tenant_registries, load_tenant_registries_with_store,
    DecisionReason, EvaluationInput, Explanation, Outcome, PolicyHistoryError, PolicyRegistry,
    PolicyRequest, PolicyRetentionQuotas, PolicySourceError, PolicyValidationError,
    VerifiedPolicyContext,
};
use layerx_agentd::prepare::evm::{
    prepare_bound, AbiArgument, EvmPreparationState, PrecompileCall, PrecompileTarget,
    PreparationDefaults, PrepareRequest,
};
use layerx_agentd::session::{OpenRequest, SessionId, SessionRecord};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_types::ids::Did;
use layerx_types::payload::ModuleRegistry;
use sha2::{Digest as _, Sha256};

const HEIGHT: u64 = 42_000_000;
const NOW: u64 = 1_793_798_930;
const FIRST: &[u8] = b"version=first\nsteps=2\nrule=restrict,deny\n";
const SECOND: &[u8] = b"version=second\nsteps=2\nrule=review,permit\n";
const THIRD: &[u8] = b"version=third\nsteps=2\nrule=stop,deny\n";
static NEXT_ROOT: AtomicU64 = AtomicU64::new(0);

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("operation refused: {error:?}"))
}

struct Root(PathBuf);

impl Root {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-1507-{}-{}",
            std::process::id(),
            NEXT_ROOT.fetch_add(1, Ordering::Relaxed)
        ));
        must(std::fs::create_dir(&path));
        Self(path)
    }

    fn store(&self) -> Arc<Mutex<Store>> {
        Arc::new(Mutex::new(must(Store::open(self.0.join("store")))))
    }
}

impl Drop for Root {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn tenant() -> TenantId {
    must(TenantId::new("repair-policy-tenant"))
}

// The peer fixture uses the production call builder and disclosure binder. The API carrier
// below transports those produced bytes and signing hash into the real approval registry.
fn prepared(id: u8, expiry: u64) -> Prepared {
    let evm = must(prepare_bound(
        EvmPreparationState {
            chain_id: 125,
            pending_nonce: u64::from(id),
            base_fee_per_gas: 1,
            head_height: HEIGHT,
        },
        PreparationDefaults {
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 3,
            maximum_payload_bytes: 4_096,
        },
        PrepareRequest {
            from: [4; 20],
            call: PrecompileCall {
                target: PrecompileTarget::Ledger,
                selector: layerx_crypto::disclosure::call::ledger_selector(
                    "mint(bytes32,bytes32,uint128)",
                ),
                arguments: vec![
                    AbiArgument::Word([9; 32]),
                    AbiArgument::Word([8; 32]),
                    AbiArgument::Word({
                        let mut amount = [0; 32];
                        amount[31] = 7;
                        amount
                    }),
                ],
            },
            expected_nonce: Some(u64::from(id)),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 4_096,
        },
        100_000,
    ));
    let bytes = evm.call.encode_unsigned();
    let counterparty = must(AgentDid::new(hex::encode([8; 32])));
    Prepared {
        preparation_ref: must(PreparationRef::new(format!("prepared-{id}"))),
        signing_preimage: must(SigningPreimage::new(evm.signing_hash.to_vec())),
        disclosure: Disclosure {
            canonical_digest: Sha256::digest(&bytes).into(),
            activity_type: ActivityType(7),
            actor: must(AgentDid::new(hex::encode(evm.disclosure.actor))),
            authority: must(AuthorityRef::new("session-key")),
            counterparties: ExplicitSet::allow(vec![counterparty.clone()]),
            amounts: ExplicitSet::allow(vec![DisclosedAmount {
                counterparty,
                amount: Amount(7),
            }]),
            asset: must(Asset::new(hex::encode(evm.disclosure.asset))),
            fee_limit: Amount(evm.disclosure.fee_limit),
            expiry: TimestampSeconds(expiry),
            idempotency_key: must(IdempotencyRef::new(hex::encode([id; 32]))),
        },
        unsigned_canonical_bytes: must(CanonicalBytes::new(bytes)),
        expiry: TimestampSeconds(expiry),
    }
}

fn approval_context(id: u8) -> ApprovalContext {
    ApprovalContext {
        tenant: tenant(),
        agent: must(Did::new(hex::encode([4; 20]).as_bytes())),
        actor_address: [4; 20],
        session: SessionId([2; 32]),
        capability: CapabilityId([3; 32]),
        policy_version: "first".to_owned(),
        request_id: [id; 32],
    }
}

fn approver() -> ApproverId {
    must(ApproverId::new("operator"))
}

fn limiter() -> BudgetLimiter {
    must(BudgetLimiter::new(vec![LimitConfig {
        id: LimitId([1; 16]),
        name: "tenant-budget".to_owned(),
        scope: LimitScope::Tenant([2; 32]),
        ceiling: 1_000,
        consumed: 0,
    }]))
}

#[test]
fn approval_time_units_check_unix_expiry_without_a_sequence_change() {
    let registry = ApprovalRegistry::default();
    let ticket = must(hold_at(
        &registry,
        approval_context(1),
        prepared(1, NOW + 60),
        ApprovalSequence(HEIGHT),
        ApprovalSequence(HEIGHT + 20),
        TimestampSeconds(NOW),
    ));
    assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
    assert_eq!(ticket.expires_at_sequence, HEIGHT + 20);
    assert!(must(expire_at(
        &registry,
        ApprovalSequence(HEIGHT),
        TimestampSeconds(NOW + 59)
    ))
    .is_empty());
    assert_eq!(
        decide_at(
            &registry,
            ticket.hold_id,
            approver(),
            ApprovalChoice::Approve,
            ticket.disclosure_digest,
            ApprovalSequence(HEIGHT),
            TimestampSeconds(NOW + 60),
        ),
        Err(ApprovalError::AlreadyDecided(ApprovalState::Expired))
    );
    let audit = must(registry.audit_entry(ticket.hold_id)).expect("expired audit");
    assert_eq!(audit.decision, ApprovalState::Expired);
    assert_eq!(audit.resulting_activity_id, None);
    assert_eq!(audit.approver, None);

    let second = must(hold_at(
        &registry,
        approval_context(2),
        prepared(2, NOW + 60),
        ApprovalSequence(HEIGHT),
        ApprovalSequence(HEIGHT + 20),
        TimestampSeconds(NOW),
    ));
    let expired = must(expire_at(
        &registry,
        ApprovalSequence(HEIGHT),
        TimestampSeconds(NOW + 60),
    ));
    assert_eq!(expired.len(), 1);
    assert_eq!(expired[0].request_id, second.hold_id);
    assert_eq!(expired[0].decision, ApprovalState::Expired);
}

#[test]
fn approval_time_units_keep_sequence_and_timestamp_bounds_independent() {
    let registry = ApprovalRegistry::default();
    // A sequence greater than Unix time is legal: only the sequence window bounds it.
    let high_sequence = 2_000_000_000;
    let ticket = must(hold_at(
        &registry,
        approval_context(3),
        prepared(3, NOW + 60),
        ApprovalSequence(high_sequence),
        ApprovalSequence(high_sequence + 20),
        TimestampSeconds(NOW),
    ));
    assert_eq!(
        decide_at(
            &registry,
            ticket.hold_id,
            approver(),
            ApprovalChoice::Approve,
            [0; 32],
            ApprovalSequence(high_sequence),
            TimestampSeconds(NOW),
        ),
        Err(ApprovalError::DisclosureChanged)
    );
    let approved = must(decide_at(
        &registry,
        ticket.hold_id,
        approver(),
        ApprovalChoice::Approve,
        ticket.disclosure_digest,
        ApprovalSequence(high_sequence),
        TimestampSeconds(NOW + 59),
    ));
    assert_eq!(approved.decision, ApprovalState::Approved);
    assert_eq!(approved.disclosure_digest, ticket.disclosure_digest);
    assert_eq!(
        decide_at(
            &registry,
            ticket.hold_id,
            approver(),
            ApprovalChoice::Approve,
            ticket.disclosure_digest,
            ApprovalSequence(high_sequence),
            TimestampSeconds(NOW + 59),
        ),
        Err(ApprovalError::AlreadyDecided(ApprovalState::Approved))
    );
    let ticket = must(hold_at(
        &registry,
        approval_context(4),
        prepared(4, NOW + 60),
        ApprovalSequence(HEIGHT),
        ApprovalSequence(HEIGHT + 20),
        TimestampSeconds(NOW),
    ));
    assert_eq!(
        decide_at(
            &registry,
            ticket.hold_id,
            approver(),
            ApprovalChoice::Approve,
            ticket.disclosure_digest,
            ApprovalSequence(HEIGHT + 20),
            TimestampSeconds(NOW),
        ),
        Err(ApprovalError::AlreadyDecided(ApprovalState::Expired))
    );
    assert_eq!(
        hold_at(
            &registry,
            approval_context(5),
            prepared(5, NOW),
            ApprovalSequence(HEIGHT),
            ApprovalSequence(HEIGHT + 20),
            TimestampSeconds(NOW),
        ),
        Err(ApprovalError::InvalidWindow)
    );
    let mut mismatch = prepared(5, NOW + 60);
    mismatch.disclosure.expiry = TimestampSeconds(NOW + 61);
    assert_eq!(
        hold_at(
            &registry,
            approval_context(5),
            mismatch,
            ApprovalSequence(HEIGHT),
            ApprovalSequence(HEIGHT + 20),
            TimestampSeconds(NOW),
        ),
        Err(ApprovalError::InvalidWindow)
    );
}

#[test]
#[allow(deprecated)]
fn approval_time_units_retire_ambiguous_entries_and_recover_timed_holds() {
    use layerx_agentd::policy::approval::{decide, hold, hold_reserved, reserve_and_hold};

    let root = Root::new();
    let budgets = limiter();
    let request = ReservationRequest {
        id: [6; 32],
        amount: 7,
        expiry_sequence: HEIGHT + 20,
        current_sequence: HEIGHT,
        applicable_limits: vec![LimitId([1; 16])],
    };
    {
        let registry = ApprovalRegistry::with_store(root.store());
        assert_eq!(
            hold(
                &registry,
                approval_context(6),
                prepared(6, NOW + 60),
                HEIGHT,
                HEIGHT + 20
            ),
            Err(ApprovalError::ClockRequired)
        );
        assert_eq!(
            reserve_and_hold(
                &registry,
                &budgets,
                approval_context(6),
                prepared(6, NOW + 60),
                &request,
                HEIGHT + 20
            ),
            Err(ApprovalReservationError::Approval(
                ApprovalError::ClockRequired
            ))
        );
        assert_eq!(budgets.held_reservations(), Ok(0));
        assert_eq!(
            reserve_and_hold_at(
                &registry,
                &budgets,
                approval_context(6),
                prepared(6, NOW),
                &request,
                ApprovalSequence(HEIGHT + 20),
                TimestampSeconds(NOW)
            ),
            Err(ApprovalReservationError::Approval(
                ApprovalError::InvalidWindow
            ))
        );
        assert_eq!(budgets.held_reservations(), Ok(0));
        let ticket = must(reserve_and_hold_at(
            &registry,
            &budgets,
            approval_context(6),
            prepared(6, NOW + 60),
            &request,
            ApprovalSequence(HEIGHT + 20),
            TimestampSeconds(NOW),
        ));
        assert_eq!(budgets.held_exposure(LimitId([1; 16])), Ok(7));
        assert_eq!(
            decide(
                &registry,
                ticket.hold_id,
                approver(),
                ApprovalChoice::Approve,
                ticket.disclosure_digest,
                HEIGHT
            ),
            Err(ApprovalError::ClockRequired)
        );
        // A borrowed legacy reservation is also refused without consuming or changing it.
        let other = limiter();
        let reservation = must(layerx_agentd::budget::reserve(&other, &request));
        assert_eq!(
            hold_reserved(
                &registry,
                approval_context(7),
                prepared(7, NOW + 60),
                HEIGHT,
                HEIGHT + 20,
                &reservation
            ),
            Err(ApprovalError::ClockRequired)
        );
        assert_eq!(other.held_exposure(LimitId([1; 16])), Ok(7));
    }
    let registry = ApprovalRegistry::with_store(root.store());
    let recovered_budgets = limiter();
    assert_eq!(registry.replay_tenant(&tenant(), &recovered_budgets), Ok(1));
    let modules = must(ModuleRegistry::new(&[]));
    assert_eq!(registry.validate_registry(&tenant(), &modules), Ok(()));
    assert_eq!(recovered_budgets.held_exposure(LimitId([1; 16])), Ok(7));
    let ticket = must(registry.ticket([6; 32])).expect("recovered hold");
    assert_eq!(ticket.disclosure.expiry, TimestampSeconds(NOW + 60));
    assert_eq!(
        decide_at(
            &registry,
            ticket.hold_id,
            approver(),
            ApprovalChoice::Approve,
            ticket.disclosure_digest,
            ApprovalSequence(HEIGHT),
            TimestampSeconds(NOW + 60)
        ),
        Err(ApprovalError::AlreadyDecided(ApprovalState::Expired))
    );
    assert_eq!(
        must(registry.audit_entry(ticket.hold_id))
            .expect("expired audit")
            .resulting_activity_id,
        None
    );
}

fn session() -> SessionRecord {
    SessionRecord {
        request: OpenRequest {
            session_id: SessionId([2; 32]),
            token_id: [1; 32],
            tenant: tenant(),
            agent: must(Did::new(b"did:layerx:policy-repair")),
            authority: ProtocolAuthority::SessionKey([1; 32]),
            permitted_activity_types: BTreeSet::from([7]),
            scopes: BTreeSet::from(["prepare".to_owned()]),
            expiry_seconds: None,
            expiry_sequence: HEIGHT + 100,
            opening_client: "policy-repair".to_owned(),
            policy_version: "first".to_owned(),
        },
        open: true,
        sequence: 0,
        budget_reserved: 0,
        subscription_cursor: 0,
        generation: 1,
        retired_token_ids: BTreeSet::new(),
    }
}

fn capability() -> Capability {
    must(Capability::new(
        CapabilityId([3; 32]),
        tenant(),
        CapabilityDimensions {
            activity_types: BTreeSet::from([7]),
            counterparties: BTreeSet::from([[8; 32]]),
            assets: BTreeSet::from([[9; 32]]),
            amount_ceiling: 500,
            rate_ceiling: RateCeiling {
                maximum_uses: 10,
                window_sequences: 100,
            },
            purposes: BTreeSet::from(["research".to_owned()]),
            expiry_sequence: HEIGHT + 100,
        },
    ))
}

fn request() -> PolicyRequest {
    PolicyRequest {
        activity_type: 7,
        counterparty: [8; 32],
        asset: [9; 32],
        amount: 7,
        purpose: "research".to_owned(),
        core_sequence: HEIGHT,
    }
}

fn decision(registry: &mut PolicyRegistry, id: [u8; 32]) -> layerx_agentd::policy::DryRunResult {
    dry_run_with_context(
        registry,
        id,
        &request(),
        &session(),
        &capability(),
        VerifiedPolicyContext::Unavailable,
    )
}

#[test]
fn policy_history_durability_reconstructs_original_decision_after_activation_and_reopen() {
    let root = Root::new();
    let source_path = root.0.join("tenant.policy");
    must(std::fs::write(&source_path, FIRST));
    let sources = BTreeMap::from([(tenant(), source_path.clone())]);
    let id;
    let original;
    {
        let mut registries = must(load_tenant_registries(&sources));
        let registry = registries.get_mut(&tenant()).expect("tenant registry");
        let snapshot = registry.begin_request();
        id = dry_run_request_id(
            &tenant(),
            SessionId([2; 32]),
            CapabilityId([3; 32]),
            snapshot.generation(),
            &request(),
        );
        let result = decision(registry, id);
        assert_eq!(result.audit_error, None);
        assert_eq!(result.decision.outcome, Outcome::Deny);
        assert_eq!(result.decision.reason, DecisionReason::InvalidContext);
        assert_eq!(
            must(Explanation::from_machine_bytes(
                &result.explanation.machine_bytes()
            )),
            result.explanation
        );
        original = result.decision;
        let admitted = must(evaluate_admission_and_record(
            registry,
            [17; 32],
            &(&request()).into(),
            &session(),
            (&capability()).into(),
            VerifiedPolicyContext::Unavailable,
        ));
        assert_eq!(admitted, original);
        let activation = must(activate_source(registry, SECOND));
        assert_eq!(activation.generation, 2);
        assert_eq!(snapshot.source(), Some(FIRST));
        assert_eq!(snapshot.generation(), 1);
        must(registry.record_decision(id, original.clone()));
        let after_activation = decision(registry, [10; 32]);
        assert_eq!(after_activation.audit_error, None);
        assert_eq!(after_activation.decision.policy_version, "second");
        assert_eq!(
            registry.record_decision(id, after_activation.decision),
            Err(PolicyHistoryError::DecisionConflict)
        );
        must(std::fs::write(&source_path, SECOND));
    }
    {
        let mut registries = must(load_tenant_registries(&sources));
        let registry = registries.get_mut(&tenant()).expect("reopened tenant");
        assert_eq!(registry.active_version(), "second");
        assert_eq!(registry.begin_request().generation(), 2);
        assert_eq!(
            registry
                .audit_entry([17; 32])
                .expect("durable admission decision")
                .decision,
            original
        );
        let (entry, recovered) = registry
            .reconstruct_decision(id)
            .expect("retained decision and source");
        assert_eq!(entry.decision, original);
        assert_eq!(entry.tenant.as_ref(), Some(&tenant()));
        assert_eq!(entry.generation, 1);
        assert_eq!(entry.source_digest, Some(Sha256::digest(FIRST).into()));
        assert_eq!(recovered.generation(), 1);
        assert_eq!(recovered.source(), Some(FIRST));
        let (request, session, capability) = (request(), session(), capability());
        let input = EvaluationInput::without_protocol_budget(&request, &session, &capability);
        assert_eq!(evaluate(recovered.policy(), &input), original);
        let replay = dry_run(registry, id, recovered.policy(), &input);
        assert_eq!(replay.audit_error, None);
        assert_eq!(replay.decision, original);
        assert_eq!(must(activate_source(registry, THIRD)).generation, 3);
        must(std::fs::write(&source_path, THIRD));
    }
    let registries = must(load_tenant_registries(&sources));
    let registry = &registries[&tenant()];
    assert_eq!(registry.begin_request().generation(), 3);
    assert_eq!(
        registry
            .audit_entry(id)
            .expect("original decision")
            .decision,
        original
    );
    assert_eq!(
        registry
            .retained_snapshot("second")
            .expect("second snapshot")
            .generation(),
        2
    );
}

#[test]
fn policy_history_quotas_refuse_new_publication_without_losing_references() {
    let root = Root::new();
    let quotas = PolicyRetentionQuotas {
        versions: 2,
        decisions: 1,
        bytes: 32_768,
    };
    let mut registry = must(PolicyRegistry::open(root.store(), tenant(), FIRST, quotas));
    let first = decision(&mut registry, [11; 32]);
    assert_eq!(first.audit_error, None);
    assert_eq!(
        decision(&mut registry, [12; 32]).audit_error,
        Some(PolicyHistoryError::DecisionQuota)
    );
    assert!(registry.audit_entry([12; 32]).is_none());
    must(registry.record_decision([11; 32], first.decision.clone()));
    assert_eq!(must(activate_source(&mut registry, SECOND)).generation, 2);
    assert_eq!(
        activate_source(&mut registry, THIRD),
        Err(PolicySourceError::Validation(
            PolicyValidationError::History(PolicyHistoryError::VersionQuota)
        ))
    );
    assert_eq!(registry.active_version(), "second");
    assert_eq!(registry.begin_request().generation(), 2);
    assert!(registry.retained("third").is_none());
    drop(registry);
    let registry = must(PolicyRegistry::open(root.store(), tenant(), SECOND, quotas));
    assert_eq!(
        registry
            .audit_entry([11; 32])
            .expect("preserved decision")
            .decision,
        first.decision
    );
    assert_eq!(
        registry
            .reconstruct_decision([11; 32])
            .expect("source retained")
            .1
            .source(),
        Some(FIRST)
    );

    let tiny = Root::new();
    assert!(matches!(
        PolicyRegistry::open(
            tiny.store(),
            tenant(),
            FIRST,
            PolicyRetentionQuotas {
                versions: 2,
                decisions: 1,
                bytes: 1
            }
        ),
        Err(PolicyHistoryError::ByteQuota)
    ));
    assert!(must(tiny.store().lock())
        .list_object_ids(&tenant(), ObjectKind::Policy)
        .is_empty());
}

#[test]
fn policy_history_failed_persistence_and_stale_owners_never_publish() {
    let root = Root::new();
    let store = root.store();
    let mut registry = must(PolicyRegistry::open(
        Arc::clone(&store),
        tenant(),
        FIRST,
        PolicyRetentionQuotas::default(),
    ));
    let mut stale = registry.clone();
    let original = decision(&mut registry, [13; 32]);
    assert_eq!(original.audit_error, None);
    assert_eq!(
        decision(&mut stale, [14; 32]).audit_error,
        Some(PolicyHistoryError::StaleRegistry)
    );
    assert!(stale.audit_entry([14; 32]).is_none());
    let blocked_temp = root.0.join("store/store.bin.tmp");
    must(std::fs::create_dir(&blocked_temp));
    assert_eq!(
        activate_source(&mut registry, SECOND),
        Err(PolicySourceError::Validation(
            PolicyValidationError::History(PolicyHistoryError::Unavailable)
        ))
    );
    assert_eq!(registry.active_version(), "first");
    assert_eq!(registry.begin_request().generation(), 1);
    assert!(registry.retained("second").is_none());
    let failed = decision(&mut registry, [15; 32]);
    assert_eq!(failed.audit_error, Some(PolicyHistoryError::Unavailable));
    assert_eq!(failed.decision.outcome, Outcome::Deny);
    assert_eq!(failed.decision.reason, DecisionReason::EvaluationFailure);
    assert!(registry.audit_entry([15; 32]).is_none());
    must(std::fs::remove_dir(&blocked_temp));
    drop(stale);
    drop(registry);
    drop(store);
    let registry = must(PolicyRegistry::open(
        root.store(),
        tenant(),
        FIRST,
        PolicyRetentionQuotas::default(),
    ));
    assert_eq!(registry.begin_request().generation(), 1);
    assert_eq!(
        registry
            .audit_entry([13; 32])
            .expect("original audit")
            .decision,
        original.decision
    );
    assert!(registry.audit_entry([14; 32]).is_none());
    assert!(registry.audit_entry([15; 32]).is_none());
}

#[test]
fn policy_history_shared_store_loader_preserves_tenant_binding_and_refuses_corruption() {
    let root = Root::new();
    let source_path = root.0.join("configured.policy");
    must(std::fs::write(&source_path, FIRST));
    let sources = BTreeMap::from([(tenant(), source_path.clone())]);
    let store = root.store();
    {
        let mut registries = must(load_tenant_registries_with_store(
            &sources,
            Arc::clone(&store),
            PolicyRetentionQuotas::default(),
        ));
        assert_eq!(
            decision(registries.get_mut(&tenant()).expect("tenant"), [16; 32]).audit_error,
            None
        );
    }
    must(std::fs::write(&source_path, SECOND));
    {
        let registries = must(load_tenant_registries_with_store(
            &sources,
            Arc::clone(&store),
            PolicyRetentionQuotas::default(),
        ));
        assert_eq!(registries[&tenant()].begin_request().generation(), 2);
        assert_eq!(
            registries[&tenant()]
                .audit_entry([16; 32])
                .expect("original")
                .generation,
            1
        );
    }
    let key = must(TenantKey::new(
        tenant(),
        ObjectKind::Policy,
        b"policy-history-v1".to_vec(),
    ));
    let bytes = must(store.lock())
        .get(&key)
        .expect("producer history")
        .bytes()
        .to_vec();
    let other_tenant = must(TenantId::new("another-tenant"));
    let other_key = must(TenantKey::new(
        other_tenant.clone(),
        ObjectKind::Policy,
        b"policy-history-v1".to_vec(),
    ));
    must(must(store.lock()).put_local(other_key, bytes.clone()));
    assert!(matches!(
        PolicyRegistry::open(
            Arc::clone(&store),
            other_tenant,
            SECOND,
            PolicyRetentionQuotas::default()
        ),
        Err(PolicyHistoryError::CorruptRecord)
    ));
    let mut record: serde_json::Value = must(serde_json::from_slice(&bytes));
    let original_byte = record["versions"][0]["digest"][0]
        .as_u64()
        .expect("digest byte");
    record["versions"][0]["digest"][0] = serde_json::json!((original_byte + 1) % 256);
    must(must(store.lock()).put_local(key, must(serde_json::to_vec(&record))));
    assert!(matches!(
        PolicyRegistry::open(store, tenant(), SECOND, PolicyRetentionQuotas::default()),
        Err(PolicyHistoryError::CorruptRecord)
    ));
}
