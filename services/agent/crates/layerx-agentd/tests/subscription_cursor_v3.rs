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
        tenant: ApiTenantId::new("tenant-a").expect("tenant"),
        agent: AgentDid::new("agent-a").expect("agent"),
        capability: CapabilityId::new("capability-a").expect("capability"),
    }
}

#[test]
fn subscription_cursor_triple_survives_restart() {
    let root = directory("restart");
    let tenant = TenantId::new("tenant-a").expect("tenant");
    let scope = scope();
    let id = SubscriptionId::new("subscription-a").expect("subscription id");
    let target = SubscriptionTarget {
        scope: scope.clone(),
        subscription_id: id.clone(),
    };
    let mut subscriptions =
        SubscriptionStore::open(Store::open(&root).expect("store"), tenant.clone())
            .expect("subscriptions");
    let start = Cursor {
        block_height: 12,
        tx_index: 0,
        log_index: 0,
    };
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
                delivery_target: DeliveryTarget::new("uds://consumer-a").expect("delivery target"),
            },
        )
        .expect("create");
    let acknowledged = Cursor {
        block_height: 12,
        tx_index: 1,
        log_index: 0,
    };
    let pending = Cursor {
        block_height: 12,
        tx_index: 1,
        log_index: 1,
    };
    subscriptions
        .mark_delivered(&target, acknowledged)
        .expect("deliver acknowledged position");
    subscriptions
        .mark_delivered(&target, pending)
        .expect("deliver pending position");
    subscriptions
        .acknowledge(&CursorAcknowledgement {
            scope: scope.clone(),
            subscription_id: id.clone(),
            cursor: acknowledged.into(),
        })
        .expect("acknowledge");
    drop(subscriptions);
    let mut reopened =
        SubscriptionStore::open(Store::open(&root).expect("reopen store"), tenant.clone())
            .expect("reopen subscriptions");
    assert_eq!(
        reopened.resume_cursor(&target).expect("resume"),
        acknowledged
    );
    assert_eq!(
        Cursor::from(reopened.get(&target).expect("record").start),
        start
    );
    let key = TenantKey::new(
        tenant,
        ObjectKind::Subscription,
        id.as_str().as_bytes().to_vec(),
    )
    .expect("subscription key");
    assert!(reopened
        .durable()
        .expect("durable")
        .get(&key)
        .expect("persisted record")
        .bytes()
        .starts_with(b"LXS3"));
    reopened
        .acknowledge(&CursorAcknowledgement {
            scope,
            subscription_id: id,
            cursor: pending.into(),
        })
        .expect("pending delivery survived restart");
    assert_eq!(
        reopened.resume_cursor(&target).expect("resume pending"),
        pending
    );
    drop(reopened);
    fs::remove_dir_all(root).expect("cleanup");
}

#[test]
fn lxs2_subscription_record_requires_cursor_reset() {
    let root = directory("lxs2");
    let tenant = TenantId::new("tenant-a").expect("tenant");
    let id = SubscriptionId::new("legacy-subscription").expect("subscription id");
    let scope = scope();
    let key = TenantKey::new(
        tenant.clone(),
        ObjectKind::Subscription,
        id.as_str().as_bytes().to_vec(),
    )
    .expect("subscription key");
    let mut bytes = b"LXS2".to_vec();
    for value in [
        id.as_str(),
        scope.tenant.as_str(),
        scope.agent.as_str(),
        scope.capability.as_str(),
    ] {
        bytes.extend_from_slice(
            &u32::try_from(value.len())
                .expect("string length")
                .to_be_bytes(),
        );
        bytes.extend_from_slice(value.as_bytes());
    }
    bytes.push(0);
    let delivery_target = "uds://consumer-a";
    bytes.extend_from_slice(
        &u32::try_from(delivery_target.len())
            .expect("target length")
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
    let mut durable = Store::open(&root).expect("store");
    durable
        .put_local(key.clone(), bytes.clone())
        .expect("persist LXS2 record");
    drop(durable);
    let error = SubscriptionStore::open(Store::open(&root).expect("reopen store"), tenant)
        .expect_err("scalar cursor requires reset");
    assert!(matches!(error, SubscriptionError::CursorResetRequired));
    assert_eq!(error.to_string(), "subscription.cursor_reset_required");
    let durable = Store::open(&root).expect("inspect stored bytes");
    assert_eq!(durable.get(&key).expect("legacy record").bytes(), bytes);
    drop(durable);
    fs::remove_dir_all(root).expect("cleanup");
}
