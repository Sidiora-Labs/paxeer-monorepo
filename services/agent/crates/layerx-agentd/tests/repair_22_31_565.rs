fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

mod support;

pub use layerx_agentd::prepare;
#[path = "approval_ordering/preparation.rs"]
mod preparation;

use std::fs;
use std::os::unix::fs::MetadataExt;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};

use layerx_agent_api::TimestampSeconds;
use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::policy::approval::{
    expire_at, reserve_and_hold_at, ApprovalContext, ApprovalError, ApprovalRegistry,
    ApprovalSequence, ApprovalState,
};
use layerx_agentd::session::SessionId;
use layerx_agentd::store::{
    migrate_forward, MigrationError, MigrationStep, ObjectKind, Store, TenantKey,
    CURRENT_SCHEMA_VERSION,
};
use layerx_types::ids::Did;

const EXPIRY: u64 = 1060;

fn limiter() -> BudgetLimiter {
    require_ok(
        BudgetLimiter::new(vec![LimitConfig {
            id: LimitId([1; 16]),
            name: "migration".to_owned(),
            scope: LimitScope::Tenant([2; 32]),
            ceiling: 1000,
            consumed: 0,
        }]),
        "budget limiter",
    )
}

fn key(kind: ObjectKind, id: &[u8]) -> TenantKey {
    require_ok(
        TenantKey::new(support::tenant(), kind, id.to_vec()),
        "tenant key",
    )
}

fn hold_key(id: u8) -> TenantKey {
    let mut bytes = b"approval-hold-v1:".to_vec();
    bytes.extend_from_slice(&[id; 32]);
    key(ObjectKind::PreparedActivity, &bytes)
}

struct Fixture {
    root: PathBuf,
    legacy: Vec<u8>,
    timed: Vec<u8>,
}

impl Fixture {
    fn new(version: u32) -> Self {
        let root = support::directory("repair-22-31-565");
        let store = Arc::new(Mutex::new(require_ok(Store::open(&root), "producer store")));
        let registry = ApprovalRegistry::with_store(Arc::clone(&store));
        let budgets = limiter();
        for id in [1, 2] {
            require_ok(
                reserve_and_hold_at(
                    &registry,
                    &budgets,
                    ApprovalContext {
                        tenant: support::tenant(),
                        agent: require_ok(Did::new(hex::encode([4; 20]).as_bytes()), "actor"),
                        actor_address: [4; 20],
                        session: SessionId([2; 32]),
                        capability: CapabilityId([3; 32]),
                        policy_version: "migration".to_owned(),
                        request_id: [id; 32],
                    },
                    preparation::prepared(id, EXPIRY),
                    &ReservationRequest {
                        id: [id; 32],
                        amount: 50,
                        expiry_sequence: 30,
                        current_sequence: 10,
                        applicable_limits: vec![LimitId([1; 16])],
                    },
                    ApprovalSequence(30),
                    TimestampSeconds(1000),
                ),
                "producer timed hold",
            );
        }
        let mut writer = require_ok(store.lock(), "producer lock");
        let first = require_some(writer.get(&hold_key(1)), "first hold").bytes();
        assert!(first.starts_with(b"LXAPHLD5"));
        let length =
            u32::from_be_bytes(require_ok(first[8..12].try_into(), "legacy length")) as usize;
        let legacy = first[12..12 + length].to_vec();
        assert!(legacy.starts_with(b"LXAPHLD4"));
        let timed = require_some(writer.get(&hold_key(2)), "second hold")
            .bytes()
            .to_vec();
        require_ok(
            writer.put_local(hold_key(1), legacy.clone()),
            "legacy store",
        );
        require_ok(
            writer.put_local(
                key(ObjectKind::PreparedActivity, b"other-preparation"),
                legacy.clone(),
            ),
            "unrelated key",
        );
        require_ok(
            writer.put_local(
                key(ObjectKind::Policy, hold_key(1).object_id()),
                legacy.clone(),
            ),
            "unrelated kind",
        );
        drop(writer);
        drop(registry);
        drop(store);
        let path = root.join("store.bin");
        let mut bytes = require_ok(fs::read(&path), "producer store bytes");
        bytes[4..8].copy_from_slice(&version.to_be_bytes());
        require_ok(fs::write(path, bytes), "prior schema fixture");
        Self {
            root,
            legacy,
            timed,
        }
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

#[test]
fn mixed_v3_holds_migrate_decode_and_replay_with_the_prepared_expiry() {
    let fixture = Fixture::new(3);
    let report = require_ok(migrate_forward(&fixture.root), "v3 migration");
    assert_eq!(CURRENT_SCHEMA_VERSION, 4);
    assert_eq!(report.original_version, 3);
    assert_eq!(report.final_version, 4);
    assert_eq!(report.steps, vec![MigrationStep { from: 3, to: 4 }]);
    let store = Arc::new(Mutex::new(require_ok(
        Store::open(&fixture.root),
        "migrated store",
    )));
    {
        let reader = require_ok(store.lock(), "reader lock");
        let migrated = require_some(reader.get(&hold_key(1)), "migrated legacy").bytes();
        let mut expected = b"LXAPHLD5".to_vec();
        expected.extend_from_slice(
            &(require_ok(u32::try_from(fixture.legacy.len()), "test value fits u32")).to_be_bytes(),
        );
        expected.extend_from_slice(&fixture.legacy);
        expected.extend_from_slice(&0_u64.to_be_bytes());
        expected.extend_from_slice(&EXPIRY.to_be_bytes());
        assert_eq!(migrated, expected);
        assert_eq!(
            require_some(reader.get(&hold_key(2)), "timed hold").bytes(),
            fixture.timed
        );
        assert_eq!(
            require_some(
                reader.get(&key(ObjectKind::PreparedActivity, b"other-preparation")),
                "other key"
            )
            .bytes(),
            fixture.legacy
        );
        assert_eq!(
            require_some(
                reader.get(&key(ObjectKind::Policy, hold_key(1).object_id())),
                "other kind"
            )
            .bytes(),
            fixture.legacy
        );
    }
    let registry = ApprovalRegistry::with_store(Arc::clone(&store));
    let budgets = limiter();
    assert_eq!(registry.replay_tenant(&support::tenant(), &budgets), Ok(2));
    assert_eq!(registry.hold_ids(), Ok(vec![[1; 32], [2; 32]]));
    assert_eq!(budgets.held_exposure(LimitId([1; 16])), Ok(100));
    for id in [1, 2] {
        let ticket = require_some(
            registry
                .ticket([id; 32])
                .unwrap_or_else(|error| panic!("{}: {error:?}", "decoded ticket")),
            "hold",
        );
        assert_eq!(ticket.disclosure.expiry, TimestampSeconds(EXPIRY));
        assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
    }
    drop(registry);
    drop(store);
    let reopened = Arc::new(Mutex::new(require_ok(Store::open(&fixture.root), "reopen")));
    let registry = ApprovalRegistry::with_store(reopened);
    assert_eq!(
        registry.replay_tenant(&support::tenant(), &limiter()),
        Ok(2)
    );
    assert!(require_ok(
        expire_at(
            &registry,
            ApprovalSequence(10),
            TimestampSeconds(EXPIRY - 1)
        ),
        "before expiry"
    )
    .is_empty());
    let expired = require_ok(
        expire_at(&registry, ApprovalSequence(10), TimestampSeconds(EXPIRY)),
        "at expiry",
    );
    assert_eq!(expired.len(), 2);
    assert!(expired
        .iter()
        .all(|entry| entry.decision == ApprovalState::Expired));
}

#[test]
fn migrated_unknown_creation_time_has_no_unix_lower_bound_but_keeps_sequence_bounds() {
    let fixture = Fixture::new(3);
    require_ok(migrate_forward(&fixture.root), "migrate");
    let mut store = require_ok(Store::open(&fixture.root), "store");
    require_ok(store.remove_local(&hold_key(2)), "remove timed peer");
    let registry = ApprovalRegistry::with_store(Arc::new(Mutex::new(store)));
    assert_eq!(
        registry.replay_tenant(&support::tenant(), &limiter()),
        Ok(1)
    );
    assert_eq!(
        expire_at(&registry, ApprovalSequence(9), TimestampSeconds(0)),
        Err(ApprovalError::InvalidWindow)
    );
    assert!(require_ok(
        expire_at(&registry, ApprovalSequence(10), TimestampSeconds(0)),
        "unknown creation"
    )
    .is_empty());
    assert_eq!(
        require_ok(
            expire_at(&registry, ApprovalSequence(30), TimestampSeconds(0)),
            "sequence expiry"
        )
        .len(),
        1
    );
}

#[test]
fn current_v4_store_is_not_rewritten_and_bare_legacy_replay_is_refused() {
    let fixture = Fixture::new(4);
    let path = fixture.root.join("store.bin");
    let before = require_ok(fs::read(&path), "v4 bytes");
    let metadata = require_ok(fs::metadata(&path), "v4 metadata");
    let report = require_ok(migrate_forward(&fixture.root), "already current");
    assert_eq!(report.original_version, 4);
    assert_eq!(report.final_version, 4);
    assert!(report.steps.is_empty());
    assert_eq!(require_ok(fs::read(&path), "after bytes"), before);
    let after = require_ok(fs::metadata(&path), "after metadata");
    assert_eq!(after.ino(), metadata.ino());
    assert_eq!(
        require_ok(after.modified(), "after modified"),
        require_ok(metadata.modified(), "before modified")
    );
    let store = Arc::new(Mutex::new(require_ok(
        Store::open(&fixture.root),
        "current store",
    )));
    let registry = ApprovalRegistry::with_store(store);
    let budgets = limiter();
    assert_eq!(
        registry.replay_tenant(&support::tenant(), &budgets),
        Err(ApprovalError::LegacyTimeUnits)
    );
    assert_eq!(registry.hold_ids(), Ok(Vec::new()));
    assert_eq!(budgets.held_exposure(LimitId([1; 16])), Ok(0));
    assert_eq!(require_ok(fs::read(path), "refused bytes"), before);
}

#[test]
fn malformed_legacy_hold_aborts_migration_without_rewriting_the_store() {
    let fixture = Fixture::new(3);
    let mut store_bytes = require_ok(fs::read(fixture.root.join("store.bin")), "store bytes");
    let hold_key = hold_key(1);
    let mut needle = vec![ObjectKind::PreparedActivity as u8, 1];
    needle.extend_from_slice(
        &(require_ok(
            u32::try_from(hold_key.object_id().len()),
            "test value fits u32",
        ))
        .to_be_bytes(),
    );
    needle.extend_from_slice(hold_key.object_id());
    needle.extend_from_slice(
        &(require_ok(u32::try_from(fixture.legacy.len()), "test value fits u32")).to_be_bytes(),
    );
    let offset = require_some(
        store_bytes
            .windows(needle.len())
            .position(|bytes| bytes == needle),
        "legacy hold record",
    ) + needle.len();
    assert_eq!(&store_bytes[offset..offset + 8], b"LXAPHLD4");
    // The tenant length is bounded by the record; make it impossible to decode.
    store_bytes[offset + 8..offset + 12].copy_from_slice(&u32::MAX.to_be_bytes());
    require_ok(
        fs::write(fixture.root.join("store.bin"), &store_bytes),
        "corrupt legacy fixture",
    );
    assert!(matches!(
        migrate_forward(&fixture.root),
        Err(MigrationError::CorruptV1)
    ));
    assert_eq!(
        require_ok(fs::read(fixture.root.join("store.bin")), "unchanged store"),
        store_bytes
    );
}
