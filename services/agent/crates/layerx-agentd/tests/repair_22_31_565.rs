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
    BudgetLimiter::new(vec![LimitConfig {
        id: LimitId([1; 16]),
        name: "migration".to_owned(),
        scope: LimitScope::Tenant([2; 32]),
        ceiling: 1000,
        consumed: 0,
    }])
    .expect("budget limiter")
}

fn key(kind: ObjectKind, id: &[u8]) -> TenantKey {
    TenantKey::new(support::tenant(), kind, id.to_vec()).expect("tenant key")
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
        let store = Arc::new(Mutex::new(Store::open(&root).expect("producer store")));
        let registry = ApprovalRegistry::with_store(Arc::clone(&store));
        let budgets = limiter();
        for id in [1, 2] {
            reserve_and_hold_at(
                &registry,
                &budgets,
                ApprovalContext {
                    tenant: support::tenant(),
                    agent: Did::new(hex::encode([4; 20]).as_bytes()).expect("actor"),
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
            )
            .expect("producer timed hold");
        }
        let mut writer = store.lock().expect("producer lock");
        let first = writer.get(&hold_key(1)).expect("first hold").bytes();
        assert!(first.starts_with(b"LXAPHLD5"));
        let length = u32::from_be_bytes(first[8..12].try_into().expect("legacy length")) as usize;
        let legacy = first[12..12 + length].to_vec();
        assert!(legacy.starts_with(b"LXAPHLD4"));
        let timed = writer
            .get(&hold_key(2))
            .expect("second hold")
            .bytes()
            .to_vec();
        writer
            .put_local(hold_key(1), legacy.clone())
            .expect("legacy store");
        writer
            .put_local(
                key(ObjectKind::PreparedActivity, b"other-preparation"),
                legacy.clone(),
            )
            .expect("unrelated key");
        writer
            .put_local(
                key(ObjectKind::Policy, hold_key(1).object_id()),
                legacy.clone(),
            )
            .expect("unrelated kind");
        drop(writer);
        drop(registry);
        drop(store);
        let path = root.join("store.bin");
        let mut bytes = fs::read(&path).expect("producer store bytes");
        bytes[4..8].copy_from_slice(&version.to_be_bytes());
        fs::write(path, bytes).expect("prior schema fixture");
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
    let report = migrate_forward(&fixture.root).expect("v3 migration");
    assert_eq!(CURRENT_SCHEMA_VERSION, 4);
    assert_eq!(report.original_version, 3);
    assert_eq!(report.final_version, 4);
    assert_eq!(report.steps, vec![MigrationStep { from: 3, to: 4 }]);
    let store = Arc::new(Mutex::new(
        Store::open(&fixture.root).expect("migrated store"),
    ));
    {
        let reader = store.lock().expect("reader lock");
        let migrated = reader.get(&hold_key(1)).expect("migrated legacy").bytes();
        let mut expected = b"LXAPHLD5".to_vec();
        expected.extend_from_slice(&(fixture.legacy.len() as u32).to_be_bytes());
        expected.extend_from_slice(&fixture.legacy);
        expected.extend_from_slice(&0_u64.to_be_bytes());
        expected.extend_from_slice(&EXPIRY.to_be_bytes());
        assert_eq!(migrated, expected);
        assert_eq!(
            reader.get(&hold_key(2)).expect("timed hold").bytes(),
            fixture.timed
        );
        assert_eq!(
            reader
                .get(&key(ObjectKind::PreparedActivity, b"other-preparation"))
                .expect("other key")
                .bytes(),
            fixture.legacy
        );
        assert_eq!(
            reader
                .get(&key(ObjectKind::Policy, hold_key(1).object_id()))
                .expect("other kind")
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
        let ticket = registry
            .ticket([id; 32])
            .expect("decoded ticket")
            .expect("hold");
        assert_eq!(ticket.disclosure.expiry, TimestampSeconds(EXPIRY));
        assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
    }
    drop(registry);
    drop(store);
    let reopened = Arc::new(Mutex::new(Store::open(&fixture.root).expect("reopen")));
    let registry = ApprovalRegistry::with_store(reopened);
    assert_eq!(
        registry.replay_tenant(&support::tenant(), &limiter()),
        Ok(2)
    );
    assert!(expire_at(
        &registry,
        ApprovalSequence(10),
        TimestampSeconds(EXPIRY - 1)
    )
    .expect("before expiry")
    .is_empty());
    let expired =
        expire_at(&registry, ApprovalSequence(10), TimestampSeconds(EXPIRY)).expect("at expiry");
    assert_eq!(expired.len(), 2);
    assert!(expired
        .iter()
        .all(|entry| entry.decision == ApprovalState::Expired));
}

#[test]
fn migrated_unknown_creation_time_has_no_unix_lower_bound_but_keeps_sequence_bounds() {
    let fixture = Fixture::new(3);
    migrate_forward(&fixture.root).expect("migrate");
    let mut store = Store::open(&fixture.root).expect("store");
    store.remove_local(&hold_key(2)).expect("remove timed peer");
    let registry = ApprovalRegistry::with_store(Arc::new(Mutex::new(store)));
    assert_eq!(
        registry.replay_tenant(&support::tenant(), &limiter()),
        Ok(1)
    );
    assert_eq!(
        expire_at(&registry, ApprovalSequence(9), TimestampSeconds(0)),
        Err(ApprovalError::InvalidWindow)
    );
    assert!(
        expire_at(&registry, ApprovalSequence(10), TimestampSeconds(0))
            .expect("unknown creation")
            .is_empty()
    );
    assert_eq!(
        expire_at(&registry, ApprovalSequence(30), TimestampSeconds(0))
            .expect("sequence expiry")
            .len(),
        1
    );
}

#[test]
fn current_v4_store_is_not_rewritten_and_bare_legacy_replay_is_refused() {
    let fixture = Fixture::new(4);
    let path = fixture.root.join("store.bin");
    let before = fs::read(&path).expect("v4 bytes");
    let metadata = fs::metadata(&path).expect("v4 metadata");
    let report = migrate_forward(&fixture.root).expect("already current");
    assert_eq!(report.original_version, 4);
    assert_eq!(report.final_version, 4);
    assert!(report.steps.is_empty());
    assert_eq!(fs::read(&path).expect("after bytes"), before);
    let after = fs::metadata(&path).expect("after metadata");
    assert_eq!(after.ino(), metadata.ino());
    assert_eq!(
        after.modified().expect("after modified"),
        metadata.modified().expect("before modified")
    );
    let store = Arc::new(Mutex::new(
        Store::open(&fixture.root).expect("current store"),
    ));
    let registry = ApprovalRegistry::with_store(store);
    let budgets = limiter();
    assert_eq!(
        registry.replay_tenant(&support::tenant(), &budgets),
        Err(ApprovalError::LegacyTimeUnits)
    );
    assert_eq!(registry.hold_ids(), Ok(Vec::new()));
    assert_eq!(budgets.held_exposure(LimitId([1; 16])), Ok(0));
    assert_eq!(fs::read(path).expect("refused bytes"), before);
}

#[test]
fn malformed_legacy_hold_aborts_migration_without_rewriting_the_store() {
    let fixture = Fixture::new(3);
    let mut store_bytes = fs::read(fixture.root.join("store.bin")).expect("store bytes");
    let hold_key = hold_key(1);
    let mut needle = vec![ObjectKind::PreparedActivity as u8, 1];
    needle.extend_from_slice(&(hold_key.object_id().len() as u32).to_be_bytes());
    needle.extend_from_slice(hold_key.object_id());
    needle.extend_from_slice(&(fixture.legacy.len() as u32).to_be_bytes());
    let offset = store_bytes
        .windows(needle.len())
        .position(|bytes| bytes == needle)
        .expect("legacy hold record")
        + needle.len();
    assert_eq!(&store_bytes[offset..offset + 8], b"LXAPHLD4");
    // The tenant length is bounded by the record; make it impossible to decode.
    store_bytes[offset + 8..offset + 12].copy_from_slice(&u32::MAX.to_be_bytes());
    fs::write(fixture.root.join("store.bin"), &store_bytes).expect("corrupt legacy fixture");
    assert!(matches!(
        migrate_forward(&fixture.root),
        Err(MigrationError::CorruptV1)
    ));
    assert_eq!(
        fs::read(fixture.root.join("store.bin")).expect("unchanged store"),
        store_bytes
    );
}
