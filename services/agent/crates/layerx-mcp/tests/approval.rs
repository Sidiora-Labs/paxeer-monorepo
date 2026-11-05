#[path = "support/clock.rs"]
mod test_clock;

use layerx_types::clock::Clock as _;
use std::fs;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
    SigningPreimage,
};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::policy::approval::{
    ApprovalContext, ApprovalError as DaemonApprovalError, ApprovalRegistry, ApprovalState,
    ApproverId,
};
use layerx_agentd::session::SessionId;
use layerx_agentd::store::TenantId;
use layerx_mcp::approval::{approve, expire, require, ApprovalError, ApprovalPolicy, Requirement};
use layerx_types::ids::Did;
use sha2::{Digest, Sha256};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

fn directory(label: &str) -> std::path::PathBuf {
    let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-mcp-approval-{label}-{}-{}-{sequence}",
        module_path!().replace("::", "-"),
        std::process::id()
    ))
}

pub fn prepared(amount: u128, request_id: u8) -> Prepared {
    use layerx_agentd::prepare::evm::{self, AbiArgument, PrecompileCall, PrecompileTarget};
    use layerx_crypto::disclosure::call::ledger_selector;
    let mut amount_word = [0; 32];
    amount_word[16..].copy_from_slice(&amount.to_be_bytes());
    let transaction = evm::prepare_bound(
        evm::EvmPreparationState {
            chain_id: 125,
            pending_nonce: u64::from(request_id),
            base_fee_per_gas: 1,
            head_height: 50,
        },
        evm::PreparationDefaults {
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 5,
            maximum_payload_bytes: 65536,
        },
        evm::PrepareRequest {
            from: [4; 20],
            call: PrecompileCall {
                target: PrecompileTarget::Ledger,
                selector: ledger_selector("mint(bytes32,bytes32,uint128)"),
                arguments: vec![
                    AbiArgument::Word([3; 32]),
                    AbiArgument::Word([5; 32]),
                    AbiArgument::Word(amount_word),
                ],
            },
            expected_nonce: Some(u64::from(request_id)),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 65536,
        },
        100_000,
    )
    .expect("real daemon preparation");
    evm::verify_call_binding(&transaction).expect("producer call binding");
    let source = &transaction.disclosure;
    let canonical = transaction.call.encode_unsigned();
    let actor = AgentDid::new(hex(&source.actor)).expect("sender-bound actor");
    let parties: Vec<_> = source
        .counterparties
        .iter()
        .map(|p| AgentDid::new(hex(&p.account)).expect("party"))
        .collect();
    Prepared {
        preparation_ref: PreparationRef::new(format!("preparation-{request_id}"))
            .expect("preparation"),
        unsigned_canonical_bytes: CanonicalBytes::new(canonical.clone()).expect("unsigned call"),
        signing_preimage: SigningPreimage::new(transaction.signing_hash.to_vec())
            .expect("signing hash"),
        disclosure: Disclosure {
            canonical_digest: Sha256::digest(&canonical).into(),
            activity_type: ActivityType(source.activity_type.ordinal()),
            actor,
            authority: AuthorityRef::new(hex(&source.authority)).expect("authority"),
            counterparties: ExplicitSet::allow(parties.clone()),
            amounts: ExplicitSet::allow(
                source
                    .amounts
                    .iter()
                    .map(|a| DisclosedAmount {
                        counterparty: parties[0].clone(),
                        amount: Amount(a.value),
                    })
                    .collect(),
            ),
            asset: Asset::new(hex(&source.asset)).expect("asset"),
            fee_limit: Amount(source.fee_limit),
            expiry: TimestampSeconds(120),
            idempotency_key: IdempotencyRef::new(format!("idempotency-{request_id}"))
                .expect("idempotency"),
        },
        expiry: TimestampSeconds(120),
    }
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}

pub fn context(request_id: u8) -> ApprovalContext {
    ApprovalContext {
        actor_address: [4; 20],
        tenant: TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}")),
        agent: Did::new(hex(&[4; 20]).as_bytes()).unwrap_or_else(|error| panic!("DID: {error:?}")),
        session: SessionId([7; 32]),
        capability: CapabilityId([9; 32]),
        policy_version: "policy-v1".to_owned(),
        request_id: [request_id; 32],
    }
}

#[test]
fn amount_at_threshold_proceeds_and_one_unit_over_is_held() {
    let registry = ApprovalRegistry::default();
    let at = require(
        &registry,
        ApprovalPolicy {
            amount_threshold: 100,
        },
        context(1),
        prepared(100, 1),
        50,
        &test_clock::TestClock::new(50),
    )
    .unwrap_or_else(|error| panic!("threshold: {error:?}"));
    assert!(matches!(
        at,
        Requirement::NotRequired {
            disclosed_amount: 100,
            ..
        }
    ));
    let above = require(
        &registry,
        ApprovalPolicy {
            amount_threshold: 100,
        },
        context(2),
        prepared(101, 2),
        50,
        &test_clock::TestClock::new(50),
    )
    .unwrap_or_else(|error| panic!("above threshold: {error:?}"));
    assert!(matches!(above, Requirement::Required(_)));
}

#[test]
fn approver_sees_the_disclosure_and_cannot_approve_an_altered_request() {
    let registry = ApprovalRegistry::default();
    let Requirement::Required(ticket) = require(
        &registry,
        ApprovalPolicy {
            amount_threshold: 100,
        },
        context(3),
        prepared(101, 3),
        50,
        &test_clock::TestClock::new(50),
    )
    .unwrap_or_else(|error| panic!("hold: {error:?}")) else {
        panic!("approval was not required");
    };
    let mut altered = ticket.disclosure.clone();
    altered.fee_limit = Amount(1);
    let approver =
        ApproverId::new("human:alice").unwrap_or_else(|error| panic!("approver: {error:?}"));
    assert_eq!(
        approve(
            &registry,
            ticket.hold_id,
            approver,
            &altered,
            60,
            &test_clock::TestClock::new(60)
        ),
        Err(ApprovalError::DisclosureChanged)
    );
    let audit = approve(
        &registry,
        ticket.hold_id,
        ApproverId::new("human:alice").unwrap_or_else(|error| panic!("approver: {error:?}")),
        &ticket.disclosure,
        60,
        &test_clock::TestClock::new(60),
    )
    .unwrap_or_else(|error| panic!("approval: {error:?}"));
    assert_eq!(audit.decision, ApprovalState::Approved);
    assert_eq!(
        audit
            .approver
            .as_ref()
            .unwrap_or_else(|| panic!("approver absent"))
            .as_str(),
        "human:alice"
    );
    assert_eq!(audit.disclosure_digest, ticket.disclosure_digest);
}

#[test]
fn expiry_at_the_declared_sequence_is_terminal_and_never_auto_approves() {
    let root = directory("expiry");
    fs::create_dir_all(&root).unwrap_or_else(|error| panic!("directory: {error}"));
    let registry = ApprovalRegistry::default();
    let Requirement::Required(ticket) = require(
        &registry,
        ApprovalPolicy {
            amount_threshold: 1,
        },
        context(4),
        prepared(2, 4),
        50,
        &test_clock::TestClock::new(50),
    )
    .unwrap_or_else(|error| panic!("hold: {error:?}")) else {
        panic!("approval was not required");
    };
    assert!(expire(&registry, 119, &test_clock::TestClock::new(119))
        .unwrap_or_else(|error| panic!("early expiry: {error:?}"))
        .is_empty());
    let expired = expire(&registry, 120, &test_clock::TestClock::new(120))
        .unwrap_or_else(|error| panic!("expiry: {error:?}"));
    assert_eq!(expired.len(), 1);
    assert_eq!(expired[0].decision, ApprovalState::Expired);
    assert!(expired[0].approver.is_none());
    assert_eq!(
        approve(
            &registry,
            ticket.hold_id,
            ApproverId::new("human:late").unwrap_or_else(|error| panic!("approver: {error:?}")),
            &ticket.disclosure,
            120,
            &test_clock::TestClock::new(120),
        ),
        Err(ApprovalError::Daemon(DaemonApprovalError::AlreadyDecided(
            ApprovalState::Expired
        )))
    );
    let _ = fs::remove_dir_all(root);
}

pub fn assert_major_two_approval_facts(
    facts: &serde_json::Value,
    approval_id: &str,
    tx_hash: &str,
    rung: &str,
) {
    assert_eq!(facts["approval_id"], approval_id);
    assert_eq!(facts["tx_hash"], tx_hash);
    assert_eq!(facts["rung"], rung);
    assert!(facts.get("activity_id").is_none());
    assert!(facts.get("resulting_activity_id").is_none());
    assert!(facts.get("verification_level").is_none());
    assert!(facts.get("level").is_none());
}

pub fn limiter() -> layerx_agentd::budget::BudgetLimiter {
    use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope};
    BudgetLimiter::new(vec![LimitConfig {
        id: LimitId([1; 16]),
        name: "approval".into(),
        scope: LimitScope::Tenant([1; 32]),
        ceiling: 1000,
        consumed: 0,
    }])
    .expect("reservation limiter")
}

pub fn reserved_hold(
    registry: &ApprovalRegistry,
    limiter: &layerx_agentd::budget::BudgetLimiter,
    request_id: u8,
) -> layerx_agentd::policy::approval::ApprovalTicket {
    use layerx_agentd::budget::{LimitId, ReservationRequest};
    layerx_agentd::policy::approval::reserve_and_hold_at(
        registry,
        limiter,
        context(request_id),
        prepared(101, request_id),
        &ReservationRequest {
            id: [request_id; 32],
            amount: 101,
            expiry_sequence: 120,
            current_sequence: 50,
            applicable_limits: vec![LimitId([1; 16])],
        },
        layerx_agentd::policy::approval::ApprovalSequence(120),
        layerx_agent_api::TimestampSeconds(
            test_clock::TestClock::new(50)
                .sample(std::time::Duration::from_secs(1))
                .expect("clock")
                .unix_seconds(),
        ),
    )
    .expect("durable reserved call hold")
}

#[test]
fn durable_call_reopens_with_reservation_and_refuses_changed_bytes_and_sender() {
    use layerx_agentd::store::{ObjectKind, Store, TenantKey};
    use layerx_types::payload::ModuleRegistry;
    use std::sync::{Arc, Mutex};
    for corruption in [None, Some("bytes"), Some("sender")] {
        let root = directory("replay");
        let store = Arc::new(Mutex::new(Store::open(&root).expect("durable store")));
        let registry = ApprovalRegistry::with_store(Arc::clone(&store));
        let original = limiter();
        let ticket = reserved_hold(&registry, &original, 19);
        registry
            .validate_registry(
                &context(19).tenant,
                &ModuleRegistry::new(&[]).expect("modules"),
            )
            .expect("valid held call");
        assert_eq!(
            original
                .held_exposure(layerx_agentd::budget::LimitId([1; 16]))
                .expect("reserved exposure"),
            101
        );
        if let Some(field) = corruption {
            let mut durable = store.lock().expect("store");
            let id = durable
                .list_object_ids(&context(19).tenant, ObjectKind::PreparedActivity)
                .into_iter()
                .find(|id| id.starts_with(b"approval-hold-v1:"))
                .expect("hold key");
            let key =
                TenantKey::new(context(19).tenant, ObjectKind::PreparedActivity, id).expect("key");
            let mut bytes = durable.get(&key).expect("record").bytes().to_vec();
            let needle = if field == "bytes" {
                prepared(101, 19)
                    .unsigned_canonical_bytes
                    .as_bytes()
                    .to_vec()
            } else {
                vec![4; 20]
            };
            let offset = bytes
                .windows(needle.len())
                .position(|w| w == needle)
                .expect("retained field");
            bytes[offset] ^= 1;
            durable.put_local(key, bytes).expect("persist corruption");
        }
        drop(registry);
        drop(original);
        drop(store);
        let reopened = Arc::new(Mutex::new(
            Store::open(&root).expect("reopen stopped store"),
        ));
        let registry = ApprovalRegistry::with_store(reopened);
        let recovered = limiter();
        let replay = registry.replay_tenant(&context(19).tenant, &recovered);
        if corruption.is_some() {
            assert!(
                replay.is_err()
                    || registry
                        .validate_registry(
                            &context(19).tenant,
                            &ModuleRegistry::new(&[]).expect("modules")
                        )
                        .is_err(),
                "changed retained {corruption:?} accepted"
            );
        } else {
            assert_eq!(replay, Ok(1));
            registry
                .validate_registry(
                    &context(19).tenant,
                    &ModuleRegistry::new(&[]).expect("modules"),
                )
                .expect("rebind recovered sender and call");
            assert_eq!(
                registry.ticket(ticket.hold_id).expect("ticket"),
                Some(ticket)
            );
            assert_eq!(
                recovered
                    .held_exposure(layerx_agentd::budget::LimitId([1; 16]))
                    .expect("recovered reservation"),
                101
            );
        }
        drop(registry);
        fs::remove_dir_all(root).expect("remove replay state");
    }
}
