mod support;

use std::fs;
use std::io::Write;
use std::os::unix::fs::{DirBuilderExt, OpenOptionsExt};
use std::sync::{Arc, Mutex};

pub use layerx_agentd::prepare;
#[path = "approval_ordering/preparation.rs"]
mod preparation;

use layerx_agentd::compat::{
    matrix, verify, verify_published, CompatibilityError, CONTRACT_VERSION, DAEMON_VERSION,
    SDK_VERSION,
};
use layerx_agentd::outbox::{Outbox, SubmissionState};
use layerx_agentd::shutdown::{graceful, DaemonLifecycle, ShutdownError, WriteStage};
use layerx_agentd::store::{
    migrate_forward, MigrationError, MigrationStep, ObjectKind, StorageClass, Store, TenantKey,
    CURRENT_SCHEMA_VERSION,
};

use support::{directory, signed_transaction, tenant};

fn write_store(path: impl AsRef<std::path::Path>, bytes: impl AsRef<[u8]>) -> std::io::Result<()> {
    fs::OpenOptions::new()
        .write(true)
        .create(true)
        .truncate(true)
        .mode(0o600)
        .open(path)?
        .write_all(bytes.as_ref())
}

fn push_bytes(output: &mut Vec<u8>, value: &[u8]) {
    output.extend_from_slice(
        &u32::try_from(value.len())
            .unwrap_or_else(|_| panic!("test value too large"))
            .to_be_bytes(),
    );
    output.extend_from_slice(value);
}

fn prior_store(version: u32, entries: &[(ObjectKind, StorageClass, &[u8], &[u8])]) -> Vec<u8> {
    let mut output = b"LXAS".to_vec();
    output.extend_from_slice(&version.to_be_bytes());
    output.extend_from_slice(
        &u32::try_from(entries.len())
            .unwrap_or_else(|_| panic!("too many test entries"))
            .to_be_bytes(),
    );
    for (kind, class, object_id, value) in entries {
        push_bytes(&mut output, b"tenant-a");
        output.push(*kind as u8);
        if version >= 2 {
            output.push(*class as u8);
        }
        push_bytes(&mut output, object_id);
        push_bytes(&mut output, value);
    }
    output
}

fn key(kind: ObjectKind, object_id: &[u8]) -> TenantKey {
    TenantKey::new(tenant(), kind, object_id.to_vec())
        .unwrap_or_else(|error| panic!("tenant key: {error}"))
}

fn legacy_hold() -> (Vec<u8>, Vec<u8>) {
    use layerx_agent_api::TimestampSeconds;
    use layerx_agentd::budget::{
        BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest,
    };
    use layerx_agentd::capability::CapabilityId;
    use layerx_agentd::policy::approval::{
        reserve_and_hold_at, ApprovalContext, ApprovalRegistry, ApprovalSequence,
    };
    use layerx_agentd::session::SessionId;
    use layerx_types::ids::Did;

    let root = directory("migration-hold-producer");
    let store = Arc::new(Mutex::new(Store::open(&root).expect("producer store")));
    let registry = ApprovalRegistry::with_store(Arc::clone(&store));
    let limiter = BudgetLimiter::new(vec![LimitConfig {
        id: LimitId([1; 16]),
        name: "migration".to_owned(),
        scope: LimitScope::Tenant([2; 32]),
        ceiling: 1000,
        consumed: 0,
    }])
    .expect("limiter");
    reserve_and_hold_at(
        &registry,
        &limiter,
        ApprovalContext {
            tenant: tenant(),
            agent: Did::new(hex::encode([4; 20]).as_bytes()).expect("actor"),
            actor_address: [4; 20],
            session: SessionId([2; 32]),
            capability: CapabilityId([3; 32]),
            policy_version: "migration".to_owned(),
            request_id: [7; 32],
        },
        preparation::prepared(7, 1060),
        &ReservationRequest {
            id: [7; 32],
            amount: 50,
            expiry_sequence: 30,
            current_sequence: 10,
            applicable_limits: vec![LimitId([1; 16])],
        },
        ApprovalSequence(30),
        TimestampSeconds(1000),
    )
    .expect("produced hold");
    let mut id = b"approval-hold-v1:".to_vec();
    id.extend_from_slice(&[7; 32]);
    let bytes = store
        .lock()
        .expect("store lock")
        .get(&key(ObjectKind::PreparedActivity, &id))
        .expect("timed hold")
        .bytes()
        .to_vec();
    assert_eq!(&bytes[..8], b"LXAPHLD5");
    let length = u32::from_be_bytes(bytes[8..12].try_into().expect("length")) as usize;
    let legacy = bytes[12..12 + length].to_vec();
    assert_eq!(&legacy[..8], b"LXAPHLD4");
    drop(registry);
    drop(store);
    fs::remove_dir_all(root).expect("cleanup producer");
    (id, legacy)
}

#[test]
fn every_supported_prior_schema_migrates_forward_without_changing_core_bytes() {
    let core_receipt = b"exact-core-produced-receipt";
    let (hold_id, legacy) = legacy_hold();
    for version in 1..CURRENT_SCHEMA_VERSION {
        let root = directory(&format!("migrate-v{version}"));
        fs::DirBuilder::new()
            .mode(0o700)
            .create(&root)
            .unwrap_or_else(|error| panic!("create root: {error}"));
        let bytes = prior_store(
            version,
            &[
                (
                    ObjectKind::Policy,
                    StorageClass::LocalOnly,
                    b"policy",
                    b"local-policy-metadata",
                ),
                (
                    ObjectKind::Identity,
                    StorageClass::CoreProducedCache,
                    b"identity",
                    b"exact-core-identity",
                ),
                (
                    ObjectKind::Budget,
                    StorageClass::CoreProducedCache,
                    b"budget",
                    b"exact-core-budget",
                ),
                (
                    ObjectKind::PreparedActivity,
                    StorageClass::LocalOnly,
                    &hold_id,
                    &legacy,
                ),
                (
                    ObjectKind::Receipt,
                    StorageClass::CoreProducedCache,
                    b"receipt",
                    core_receipt,
                ),
            ],
        );
        write_store(root.join("store.bin"), bytes)
            .unwrap_or_else(|error| panic!("write v{version}: {error}"));
        let report =
            migrate_forward(&root).unwrap_or_else(|error| panic!("migrate v{version}: {error}"));
        assert_eq!(report.original_version, version);
        assert_eq!(report.final_version, CURRENT_SCHEMA_VERSION);
        assert_eq!(report.core_produced_records_preserved, 3);
        let expected_steps: Vec<_> = (version..CURRENT_SCHEMA_VERSION)
            .map(|from| MigrationStep { from, to: from + 1 })
            .collect();
        assert_eq!(report.steps, expected_steps);

        let store = Store::open(&root).unwrap_or_else(|error| panic!("open migrated: {error}"));
        let receipt = store
            .get(&key(ObjectKind::Receipt, b"receipt"))
            .unwrap_or_else(|| panic!("migrated receipt absent"));
        assert_eq!(receipt.class(), StorageClass::CoreProducedCache);
        assert_eq!(receipt.bytes(), core_receipt);
        assert_eq!(
            store
                .get(&key(ObjectKind::Identity, b"identity"))
                .expect("identity")
                .bytes(),
            b"exact-core-identity"
        );
        assert_eq!(
            store
                .get(&key(ObjectKind::Budget, b"budget"))
                .expect("budget")
                .bytes(),
            b"exact-core-budget"
        );
        let migrated = store
            .get(&key(ObjectKind::PreparedActivity, &hold_id))
            .expect("migrated hold")
            .bytes();
        let mut expected = b"LXAPHLD5".to_vec();
        push_bytes(&mut expected, &legacy);
        expected.extend_from_slice(&0_u64.to_be_bytes());
        expected.extend_from_slice(&1060_u64.to_be_bytes());
        assert_eq!(migrated, expected);
        drop(store);
        let _ = fs::remove_dir_all(root);
    }
}

#[test]
fn newer_and_corrupt_prior_schemas_are_refused_without_rewriting_the_store() {
    let root = directory("migration-refusal");
    fs::DirBuilder::new()
        .mode(0o700)
        .create(&root)
        .unwrap_or_else(|error| panic!("create root: {error}"));
    let mut newer = b"LXAS".to_vec();
    newer.extend_from_slice(&(CURRENT_SCHEMA_VERSION + 1).to_be_bytes());
    write_store(root.join("store.bin"), &newer)
        .unwrap_or_else(|error| panic!("write newer: {error}"));
    assert!(matches!(
        migrate_forward(&root),
        Err(MigrationError::NewerSchema { .. })
    ));
    assert_eq!(
        fs::read(root.join("store.bin")).unwrap_or_else(|error| panic!("read newer: {error}")),
        newer
    );

    let mut corrupt = prior_store(
        2,
        &[(
            ObjectKind::Receipt,
            StorageClass::CoreProducedCache,
            b"receipt",
            b"core",
        )],
    );
    let class_offset = 8 + 4 + 4 + b"tenant-a".len() + 1;
    corrupt[class_offset] = 9;
    write_store(root.join("store.bin"), corrupt)
        .unwrap_or_else(|error| panic!("write corrupt: {error}"));
    assert!(matches!(
        migrate_forward(&root),
        Err(MigrationError::CorruptV1)
    ));
    let _ = fs::remove_dir_all(root);
}

fn enqueue(outbox: &mut Outbox, store: &mut Store, id: u8) -> [u8; 32] {
    outbox
        .enqueue(store, tenant(), signed_transaction(id))
        .unwrap_or_else(|error| panic!("enqueue {id}: {error:?}"))
}

fn transition(outbox: &mut Outbox, store: &mut Store, hash: [u8; 32], state: SubmissionState) {
    outbox
        .transition(store, hash, state, format!("enter {state:?}"), None)
        .unwrap_or_else(|error| panic!("transition {hash:?}: {error:?}"));
}

#[test]
fn shutdown_records_pre_submission_work_and_verifies_every_outbox_stage() {
    let root = directory("shutdown-stages");
    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let mut outbox = Outbox::default();
    let mut ids = std::collections::BTreeMap::from([(1, [1; 32]), (2, [2; 32])]);
    for id in 3..=6 {
        ids.insert(id, enqueue(&mut outbox, &mut store, id));
    }
    transition(&mut outbox, &mut store, ids[&4], SubmissionState::Submitted);
    transition(&mut outbox, &mut store, ids[&5], SubmissionState::Submitted);
    transition(&mut outbox, &mut store, ids[&5], SubmissionState::Submitted);
    transition(&mut outbox, &mut store, ids[&6], SubmissionState::Submitted);
    transition(&mut outbox, &mut store, ids[&6], SubmissionState::Unknown);

    let mut lifecycle = DaemonLifecycle::running();
    for (id, stage) in [
        (1, WriteStage::Preparing),
        (2, WriteStage::Signing),
        (3, WriteStage::Queued),
        (4, WriteStage::Submitted),
        (5, WriteStage::Acknowledged),
        (6, WriteStage::Unknown),
    ] {
        lifecycle
            .begin_stage(ids[&id], stage, format!("durable-{stage:?}").into_bytes())
            .unwrap_or_else(|error| panic!("begin stage {stage:?}: {error:?}"));
    }
    lifecycle
        .append_audit(b"shutdown requested".to_vec())
        .unwrap_or_else(|error| panic!("append audit: {error:?}"));
    let report = graceful(&mut store, tenant(), &outbox, &mut lifecycle)
        .unwrap_or_else(|error| panic!("graceful shutdown: {error:?}"));
    assert_eq!(report.in_flight_recorded, 6);
    assert_eq!(report.pre_submission_recorded, 2);
    assert_eq!(report.outbox_submissions_verified, 4);
    assert_eq!(report.audit_entries_flushed, 1);
    assert!(!report.accepting_work);
    assert!(!lifecycle.accepting_work());

    drop(store);
    let store = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
    for (id, expected) in [
        (3, SubmissionState::Queued),
        (4, SubmissionState::Submitted),
        (5, SubmissionState::Submitted),
        (6, SubmissionState::Unknown),
    ] {
        let mut restored = Outbox::default();
        restored
            .restore(&store, tenant(), ids[&id])
            .unwrap_or_else(|error| panic!("restore {id}: {error:?}"));
        assert_eq!(
            restored
                .status(ids[&id])
                .unwrap_or_else(|| panic!("status {id} absent"))
                .state,
            expected
        );
    }
    assert!(store
        .list_object_ids(&tenant(), ObjectKind::Configuration)
        .iter()
        .any(|id| id == b"shutdown-complete"));
    let _ = fs::remove_dir_all(root);
}

#[test]
fn shutdown_refuses_an_inflight_stage_that_disagrees_with_the_durable_outbox() {
    let root = directory("shutdown-mismatch");
    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let mut outbox = Outbox::default();
    let hash = enqueue(&mut outbox, &mut store, 3);
    let mut lifecycle = DaemonLifecycle::running();
    lifecycle
        .begin_stage(hash, WriteStage::Submitted, b"submitted".to_vec())
        .unwrap_or_else(|error| panic!("begin stage: {error:?}"));
    assert!(matches!(
        graceful(&mut store, tenant(), &outbox, &mut lifecycle),
        Err(ShutdownError::OutboxStageMismatch {
            expected: SubmissionState::Submitted,
            actual: SubmissionState::Queued,
            ..
        })
    ));
    assert!(!store
        .list_object_ids(&tenant(), ObjectKind::Configuration)
        .iter()
        .any(|id| id == b"shutdown-complete"));
    let _ = fs::remove_dir_all(root);
}

#[test]
fn published_compatibility_matrix_accepts_only_the_supported_complete_tuple() {
    assert_eq!(matrix().len(), 1);
    assert_eq!(verify_published(), Ok(()));
    let node_client = format!(
        "paxd/v{}.0.0/linux-amd64/go1.25",
        matrix()[0].node_interface_major
    );
    let unsupported_node_client = format!(
        "paxd/v{}.0.0/linux-amd64/go1.25",
        matrix()[0].node_interface_major + 1
    );
    assert!(verify(DAEMON_VERSION, &node_client, CONTRACT_VERSION, SDK_VERSION).is_ok());
    assert_eq!(
        verify(
            DAEMON_VERSION,
            &unsupported_node_client,
            CONTRACT_VERSION,
            SDK_VERSION,
        ),
        Err(CompatibilityError::UnsupportedNodeInterface)
    );
    assert_eq!(
        verify(
            DAEMON_VERSION,
            &node_client,
            CONTRACT_VERSION + 1,
            SDK_VERSION
        ),
        Err(CompatibilityError::UnsupportedContract)
    );
    assert_eq!(
        verify(DAEMON_VERSION, &node_client, CONTRACT_VERSION, "9.0.0"),
        Err(CompatibilityError::UnsupportedSdk)
    );
}
