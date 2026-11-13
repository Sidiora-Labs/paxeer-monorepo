use std::os::unix::fs::PermissionsExt;
use std::path::Path;
use std::sync::{Arc, Mutex, OnceLock};
use std::time::Duration;

pub mod runtime_clock {
    include!("repair_22_33_44_clock.rs");
}

use layerx_client::runtime_clock::RuntimeClock;
use layerx_types::clock::Clock;

use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
    SigningPreimage,
};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::policy::approval::{
    reserve_and_hold_at, ApprovalContext, ApprovalRegistry, ApprovalSequence,
};
use layerx_agentd::prepare::EvmPrepared;
use layerx_agentd::session::SessionId;
use layerx_agentd::store::{Store, TenantId};
use layerx_types::ids::Did;
use sha2::{Digest, Sha256};

pub fn clock() -> Arc<RuntimeClock> {
    static AUTHORITY: OnceLock<runtime_clock::ClockAuthority> = OnceLock::new();
    AUTHORITY
        .get_or_init(|| {
            let root = layerx_human_test_support::directory("a44");
            std::fs::create_dir_all(&root).or_fail("approval clock root");
            std::fs::set_permissions(&root, std::fs::Permissions::from_mode(0o700))
                .or_fail("private approval clock root");
            runtime_clock::ClockAuthority::start(&root).or_fail("real approval runtime clock")
        })
        .connect()
}

pub fn current_time(clock: &dyn Clock) -> TimestampSeconds {
    TimestampSeconds(
        clock
            .sample(Duration::from_secs(1))
            .or_fail("authenticated approval time")
            .unix_seconds(),
    )
}

pub fn activity_expiry() -> u64 {
    static EXPIRY: OnceLock<u64> = OnceLock::new();
    *EXPIRY.get_or_init(|| {
        current_time(clock().as_ref())
            .get()
            .checked_add(900)
            .or_fail("activity expiry")
    })
}

pub fn hex(bytes: &[u8]) -> String {
    layerx_human_service::hex::lower(bytes)
}

pub fn preparation(evm: &EvmPrepared) -> Prepared {
    layerx_agentd::prepare::verify_call_binding(evm).or_fail("producer disclosure binding");
    let disclosure = &evm.disclosure;
    let bytes = evm.call.encode_unsigned();
    assert_eq!(
        layerx_crypto::disclosure::call::reencode(disclosure).or_fail("call disclosure reencoding"),
        bytes,
    );
    let counterparties: Vec<_> = disclosure
        .counterparties
        .iter()
        .map(|party| AgentDid::new(hex(&party.account)).or_fail("counterparty"))
        .collect();
    let amounts = disclosure
        .amounts
        .iter()
        .map(|amount| DisclosedAmount {
            counterparty: counterparties.last().or_fail("disclosed recipient").clone(),
            amount: Amount(amount.value),
        })
        .collect();
    let expiry = TimestampSeconds(
        disclosure
            .expiry
            .not_after
            .min(disclosure.expiry.payload_expires_at),
    );
    Prepared {
        preparation_ref: PreparationRef::new(hex(&evm.signing_hash))
            .or_fail("preparation reference"),
        disclosure: Disclosure {
            canonical_digest: Sha256::digest(&bytes).into(),
            activity_type: ActivityType(disclosure.activity_type.ordinal()),
            actor: AgentDid::new(hex(&disclosure.actor)).or_fail("sender actor"),
            authority: AuthorityRef::new(hex(&disclosure.authority)).or_fail("authority"),
            counterparties: ExplicitSet::allow(counterparties),
            amounts: ExplicitSet::allow(amounts),
            asset: Asset::new(hex(&disclosure.asset)).or_fail("asset"),
            fee_limit: Amount(disclosure.fee_limit),
            expiry,
            idempotency_key: IdempotencyRef::new(hex(&disclosure.idempotency_key))
                .or_fail("idempotency"),
        },
        unsigned_canonical_bytes: CanonicalBytes::new(bytes).or_fail("unsigned type-2 bytes"),
        signing_preimage: SigningPreimage::new(evm.signing_hash.to_vec()).or_fail("signing hash"),
        expiry,
    }
}

pub fn limiter() -> BudgetLimiter {
    BudgetLimiter::new(vec![LimitConfig {
        id: LimitId([1; 16]),
        name: "approval repair".to_owned(),
        scope: LimitScope::Tenant([1; 32]),
        ceiling: 1_000,
        consumed: 0,
    }])
    .or_fail("budget limiter")
}

pub fn register_and_reopen(
    root: &Path,
    tenant: &TenantId,
    evm: &EvmPrepared,
    marker: u8,
    created: u64,
    expires: u64,
    clock: &dyn Clock,
) -> (
    ApprovalRegistry,
    BudgetLimiter,
    layerx_agentd::approval::ApprovalExpiry,
) {
    let prepared = preparation(evm);
    let store =
        Arc::new(Mutex::new(Store::open(root).unwrap_or_else(|error| {
            panic!("{}: {error:?}", "approval store")
        })));
    let registry = ApprovalRegistry::with_store(store.clone());
    let limits = limiter();
    let ticket = reserve_and_hold_at(
        &registry,
        &limits,
        ApprovalContext {
            actor_address: evm.from,
            tenant: tenant.clone(),
            agent: Did::new(prepared.disclosure.actor.as_str().as_bytes())
                .or_fail("sender identity"),
            session: SessionId([2; 32]),
            capability: CapabilityId([3; 32]),
            policy_version: "approval-repair-v1".to_owned(),
            request_id: [marker; 32],
        },
        prepared.clone(),
        &ReservationRequest {
            id: [marker; 32],
            amount: 1,
            expiry_sequence: expires,
            current_sequence: created,
            applicable_limits: vec![LimitId([1; 16])],
        },
        ApprovalSequence(expires),
        current_time(clock),
    )
    .or_fail("durable reserved approval");
    assert_eq!(ticket.disclosure, prepared.disclosure);
    drop(registry);
    drop(store);
    drop(limits);
    let reopened =
        Arc::new(Mutex::new(Store::open(root).unwrap_or_else(|error| {
            panic!("{}: {error:?}", "reopen approval store")
        })));
    let registry = ApprovalRegistry::with_store(reopened.clone());
    let limits = limiter();
    assert_eq!(
        registry
            .replay_tenant(tenant, &limits)
            .or_fail("replay held approval"),
        1
    );
    let modules = layerx_types::payload::ModuleRegistry::new(&[]).or_fail("EVM registry");
    registry
        .validate_registry(tenant, &modules)
        .or_fail("rebind durable call and sender");
    assert_eq!(
        registry
            .ticket([marker; 32])
            .or_fail("ticket")
            .or_fail("recovered ticket"),
        ticket
    );
    let expiry = layerx_agentd::approval::ApprovalExpiry::from_shared_store(reopened);
    (registry, limits, expiry)
}

use crate::require_value::RequireValue as _;
