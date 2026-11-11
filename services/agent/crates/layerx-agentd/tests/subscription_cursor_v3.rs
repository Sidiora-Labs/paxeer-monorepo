use std::fs;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agent_api::identity::{AgentDid, CapabilityId, ExplicitSet, TenantId as ApiTenantId};
use layerx_agent_api::subscription::{
    Cursor as ApiCursor, CursorAcknowledgement, DeliveryTarget, SubscriptionCreate,
    SubscriptionFilter, SubscriptionId, SubscriptionScope, SubscriptionTarget,
};
use layerx_agentd::events::subscription::{Cursor, Store as SubscriptionStore, SubscriptionError};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

fn directory(name: &str) -> PathBuf {
    std::env::temp_dir().join(format!(
        "subscription-cursor-v3-{name}-{}-{}",
        std::process::id(),
        NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
    ))
}

fn scope() -> SubscriptionScope {
    SubscriptionScope {
        tenant: ApiTenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error:?}")),
        agent: AgentDid::new("agent-a").unwrap_or_else(|error| panic!("agent: {error:?}")),
        capability: CapabilityId::new("capability-a")
            .unwrap_or_else(|error| panic!("capability: {error:?}")),
    }
}

#[test]
fn subscription_cursor_triple_survives_restart() {
    assert_cursor_restart(
        Cursor {
            block_height: 12,
            tx_index: 0,
            log_index: 0,
        },
        Cursor {
            block_height: 12,
            tx_index: 1,
            log_index: 0,
        },
        Cursor {
            block_height: 12,
            tx_index: 1,
            log_index: 1,
        },
    );
}

#[test]
fn anchored_local_counters_use_the_same_lxs3_record() {
    assert_cursor_restart(
        Cursor {
            block_height: 0,
            tx_index: 0,
            log_index: 0,
        },
        Cursor {
            block_height: 0,
            // 2985: attempt-1 (0, 0, MAX) -> (0, MAX, MAX - 1).
            tx_index: u32::MAX,
            log_index: u32::MAX - 1,
        },
        Cursor {
            block_height: 0,
            // 2985: attempt-1 (0, 1, 0) -> (0, MAX, MAX); no counter carry.
            tx_index: u32::MAX,
            log_index: u32::MAX,
        },
    );
}

fn assert_cursor_restart(start: Cursor, acknowledged: Cursor, pending: Cursor) {
    let root = directory("restart");
    let tenant = TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error:?}"));
    let scope = scope();
    let id = SubscriptionId::new("subscription-a")
        .unwrap_or_else(|error| panic!("subscription id: {error:?}"));
    let target = SubscriptionTarget {
        scope: scope.clone(),
        subscription_id: id.clone(),
    };
    let mut subscriptions = SubscriptionStore::open(
        Store::open(&root).unwrap_or_else(|error| panic!("store: {error:?}")),
        tenant.clone(),
    )
    .unwrap_or_else(|error| panic!("subscriptions: {error:?}"));
    subscriptions
        .create(
            id.clone(),
            SubscriptionCreate {
                scope: scope.clone(),
                filter: SubscriptionFilter {
                    agents: ExplicitSet::allow(Vec::new()),
                    accounts: ExplicitSet::allow(Vec::new()),
                    activity_types: ExplicitSet::allow(Vec::new()),
                    modules: ExplicitSet::allow(Vec::new()),
                    assets: ExplicitSet::allow(Vec::new()),
                    counterparties: ExplicitSet::allow(Vec::new()),
                    result_classes: ExplicitSet::allow(Vec::new()),
                },
                start: ApiCursor::from(start),
                delivery_target: DeliveryTarget::new("uds://consumer-a")
                    .unwrap_or_else(|error| panic!("delivery target: {error:?}")),
            },
        )
        .unwrap_or_else(|error| panic!("create: {error:?}"));
    subscriptions
        .mark_delivered(&target, acknowledged)
        .unwrap_or_else(|error| panic!("deliver acknowledged position: {error:?}"));
    subscriptions
        .mark_delivered(&target, pending)
        .unwrap_or_else(|error| panic!("deliver pending position: {error:?}"));
    subscriptions
        .acknowledge(&CursorAcknowledgement {
            scope: scope.clone(),
            subscription_id: id.clone(),
            cursor: acknowledged.into(),
        })
        .unwrap_or_else(|error| panic!("acknowledge: {error:?}"));
    drop(subscriptions);
    let mut reopened = SubscriptionStore::open(
        Store::open(&root).unwrap_or_else(|error| panic!("reopen store: {error:?}")),
        tenant.clone(),
    )
    .unwrap_or_else(|error| panic!("reopen subscriptions: {error:?}"));
    assert_eq!(
        reopened
            .resume_cursor(&target)
            .unwrap_or_else(|error| panic!("resume: {error:?}")),
        acknowledged
    );
    assert_eq!(
        Cursor::from(
            reopened
                .get(&target)
                .unwrap_or_else(|error| panic!("record: {error:?}"))
                .start
        ),
        start
    );
    let key = TenantKey::new(
        tenant,
        ObjectKind::Subscription,
        id.as_str().as_bytes().to_vec(),
    )
    .unwrap_or_else(|error| panic!("subscription key: {error:?}"));
    assert!(reopened
        .durable()
        .unwrap_or_else(|error| panic!("durable: {error:?}"))
        .get(&key)
        .unwrap_or_else(|| panic!("persisted record"))
        .bytes()
        .starts_with(b"LXS3"));
    reopened
        .acknowledge(&CursorAcknowledgement {
            scope,
            subscription_id: id,
            cursor: pending.into(),
        })
        .unwrap_or_else(|error| panic!("pending delivery survived restart: {error:?}"));
    assert_eq!(
        reopened
            .resume_cursor(&target)
            .unwrap_or_else(|error| panic!("resume pending: {error:?}")),
        pending
    );
    drop(reopened);
    fs::remove_dir_all(root).unwrap_or_else(|error| panic!("cleanup: {error:?}"));
}

#[test]
fn lxs2_subscription_record_requires_cursor_reset() {
    let root = directory("lxs2");
    let tenant = TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error:?}"));
    let id = SubscriptionId::new("legacy-subscription")
        .unwrap_or_else(|error| panic!("subscription id: {error:?}"));
    let scope = scope();
    let key = TenantKey::new(
        tenant.clone(),
        ObjectKind::Subscription,
        id.as_str().as_bytes().to_vec(),
    )
    .unwrap_or_else(|error| panic!("subscription key: {error:?}"));
    let mut bytes = b"LXS2".to_vec();
    for value in [
        id.as_str(),
        scope.tenant.as_str(),
        scope.agent.as_str(),
        scope.capability.as_str(),
    ] {
        bytes.extend_from_slice(
            &u32::try_from(value.len())
                .unwrap_or_else(|error| panic!("string length: {error:?}"))
                .to_be_bytes(),
        );
        bytes.extend_from_slice(value.as_bytes());
    }
    bytes.push(0);
    let delivery_target = "uds://consumer-a";
    bytes.extend_from_slice(
        &u32::try_from(delivery_target.len())
            .unwrap_or_else(|error| panic!("target length: {error:?}"))
            .to_be_bytes(),
    );
    bytes.extend_from_slice(delivery_target.as_bytes());
    bytes.extend_from_slice(&10_u64.to_be_bytes());
    bytes.extend_from_slice(&12_u64.to_be_bytes());
    bytes.extend_from_slice(&0_u32.to_be_bytes());
    bytes.extend_from_slice(&[0, 0, 0]);
    for _ in 0..7 {
        bytes.extend_from_slice(&0_u32.to_be_bytes());
    }
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error:?}"));
    durable
        .put_local(key.clone(), bytes.clone())
        .unwrap_or_else(|error| panic!("persist LXS2 record: {error:?}"));
    drop(durable);
    let error = SubscriptionStore::open(
        Store::open(&root).unwrap_or_else(|error| panic!("reopen store: {error:?}")),
        tenant.clone(),
    )
    .err()
    .unwrap_or_else(|| panic!("scalar cursor requires reset"));
    // 2986: preserve the bytes while asserting the producer's named upgrade refusal.
    assert!(
        matches!(&error, SubscriptionError::UpgradeRequired { tenant: owner, subscription_id }
        if owner == tenant.as_str() && subscription_id == id.as_str())
    );
    assert_eq!(error.to_string(), format!("subscription.cursor_reset_required: tenant={} subscription={}; use Store::recover_legacy with an explicit replay boundary and retained backup", tenant.as_str(), id.as_str()));
    let durable =
        Store::open(&root).unwrap_or_else(|error| panic!("inspect stored bytes: {error:?}"));
    assert_eq!(
        durable
            .get(&key)
            .unwrap_or_else(|| panic!("legacy record"))
            .bytes(),
        bytes
    );
    drop(durable);
    fs::remove_dir_all(root).unwrap_or_else(|error| panic!("cleanup: {error:?}"));
}
