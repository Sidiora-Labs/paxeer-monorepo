use std::fs::{self, DirBuilder, File, OpenOptions};
use std::io;
use std::os::unix::fs::{symlink, DirBuilderExt, MetadataExt, OpenOptionsExt, PermissionsExt};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Barrier, Mutex};

use layerx_agentd::store::{
    migrate, migrate_forward, MigrationError, ObjectKind, StorageClass, Store, StoreError,
    TenantId, TenantKey, CURRENT_SCHEMA_VERSION,
};

static NEXT: AtomicU64 = AtomicU64::new(0);

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        let root = must(fs::canonicalize(std::env::temp_dir())).join(format!(
            "repair-22-33-1516-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        must(DirBuilder::new().mode(0o700).create(&root));
        Self(root)
    }

    fn child(&self, name: &str) -> PathBuf {
        self.0.join(name)
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        must(fs::remove_dir_all(&self.0));
    }
}

fn tenant() -> TenantId {
    must(TenantId::new("repair-1516"))
}

fn key(kind: ObjectKind, id: &[u8]) -> TenantKey {
    must(TenantKey::new(tenant(), kind, id.to_vec()))
}

fn value(store: &Store, key: &TenantKey) -> Vec<u8> {
    store
        .get(key)
        .unwrap_or_else(|| panic!("missing {key:?}"))
        .bytes()
        .to_vec()
}

fn assert_private(path: &Path, mode: u32) {
    let metadata = must(fs::symlink_metadata(path));
    assert_eq!(metadata.mode() & 0o7777, mode, "{}", path.display());
    assert_eq!(metadata.uid(), rustix::process::geteuid().as_raw());
    if mode == 0o600 {
        assert!(metadata.is_file());
        assert_eq!(metadata.nlink(), 1);
    } else {
        assert!(metadata.is_dir());
    }
}

fn assert_permissive_umask(directory: &Directory) {
    let path = directory.child("umask-control");
    let _control = must(
        OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o666)
            .open(&path),
    );
    assert_eq!(
        must(fs::metadata(path)).mode() & 0o044,
        0o044,
        "this security proof must execute under a permissive umask (000 or 022)"
    );
}

fn produce(root: &Path) -> Vec<u8> {
    let mut store = must(Store::open(root));
    must(store.put_local(
        key(ObjectKind::Session, b"session"),
        b"private-bearer-token".to_vec(),
    ));
    must(store.put_core_cache(
        key(ObjectKind::Receipt, b"receipt"),
        b"exact-core-receipt".to_vec(),
    ));
    drop(store);
    must(fs::read(root.join("store.bin")))
}

fn prior_snapshot(root: &Path, version: u32) -> Vec<u8> {
    // Start with the production producer's snapshot. V2 has the identical record codec;
    // V1 differs only by the authority-class byte introduced by migration.
    let mut bytes = produce(root);
    if version == 1 {
        let mut cursor = 12;
        let count = u32::from_be_bytes(must(bytes[8..12].try_into()));
        for _ in 0..count {
            let length = u32::from_be_bytes(must(bytes[cursor..cursor + 4].try_into()));
            cursor += 4 + must(usize::try_from(length)) + 1;
            bytes.remove(cursor);
            for _ in 0..2 {
                let length = u32::from_be_bytes(must(bytes[cursor..cursor + 4].try_into()));
                cursor += 4 + must(usize::try_from(length));
            }
        }
        assert_eq!(cursor, bytes.len());
    }
    bytes[4..8].copy_from_slice(&version.to_be_bytes());
    must(fs::write(root.join("store.bin"), &bytes));
    assert_private(&root.join("store.bin"), 0o600);
    bytes
}

fn assert_refuses(root: &Path) {
    assert!(matches!(Store::open(root), Err(StoreError::Io(_))));
    assert!(matches!(
        Store::open_read_only(root),
        Err(StoreError::Io(_))
    ));
    assert!(matches!(migrate_forward(root), Err(MigrationError::Io(_))));
}

#[test]
fn store_private_file_modes_permissive_umask_and_replacements_recover() {
    let directory = Directory::new();
    assert_permissive_umask(&directory);
    let root = directory.child("nested/store");
    let mut writer = must(Store::open(&root));
    assert_private(&directory.child("nested"), 0o700);
    assert_private(&root, 0o700);
    let secret = key(ObjectKind::Session, b"session");
    must(writer.put_local(secret.clone(), b"token-first".to_vec()));
    let snapshot = root.join("store.bin");
    assert_private(&snapshot, 0o600);
    // Hold the old inode open so inode reuse cannot hide whether replacement happened.
    let old = must(File::open(&snapshot));
    must(writer.put_local(secret.clone(), b"token-second".to_vec()));
    assert_ne!(
        must(old.metadata()).ino(),
        must(fs::metadata(&snapshot)).ino()
    );
    assert_private(&snapshot, 0o600);
    let mut reader = must(Store::open_read_only(&root));
    assert_eq!(value(&reader, &secret), b"token-second");
    assert!(reader
        .put_local(secret.clone(), b"forbidden".to_vec())
        .is_err());
    drop(writer);
    let recovered = must(Store::open(&root));
    assert_eq!(value(&recovered, &secret), b"token-second");
    assert_eq!(
        must(fs::read_dir(&root)).count(),
        1,
        "temporary files must be removed"
    );
}

#[test]
fn store_private_file_modes_refuses_unsafe_roots_and_snapshots() {
    let directory = Directory::new();
    let source = directory.child("source");
    let original = produce(&source);
    let root = directory.child("unsafe");
    must(DirBuilder::new().mode(0o700).create(&root));
    let snapshot = root.join("store.bin");
    must(fs::copy(source.join("store.bin"), &snapshot));
    must(fs::set_permissions(
        &root,
        fs::Permissions::from_mode(0o755),
    ));
    assert_refuses(&root);
    assert_eq!(must(fs::read(&snapshot)), original);
    must(fs::set_permissions(
        &root,
        fs::Permissions::from_mode(0o700),
    ));
    for mode in [0o644, 0o660, 0o601] {
        must(fs::set_permissions(
            &snapshot,
            fs::Permissions::from_mode(mode),
        ));
        assert_refuses(&root);
        assert_eq!(must(fs::read(&snapshot)), original);
    }
    must(fs::remove_file(&snapshot));
    must(symlink(source.join("store.bin"), &snapshot));
    assert_refuses(&root);
    must(fs::remove_file(&snapshot));
    must(symlink(directory.child("missing"), &snapshot));
    assert_refuses(&root);
    assert!(!directory.child("missing").exists());
    must(fs::remove_file(&snapshot));
    must(fs::hard_link(source.join("store.bin"), &snapshot));
    assert_refuses(&root);
    must(fs::remove_file(&snapshot));
    must(fs::create_dir(&snapshot));
    assert_refuses(&root);
    let alias = directory.child("alias");
    must(symlink(&source, &alias));
    assert_refuses(&alias);
    assert_refuses(&alias.join("new-store"));
    assert!(!source.join("new-store").exists());
    let ordinary_file = directory.child("ordinary-file");
    must(fs::write(&ordinary_file, b"not a directory"));
    assert_refuses(&ordinary_file);
    assert_eq!(must(fs::read(source.join("store.bin"))), original);
}

#[test]
fn store_private_file_modes_refuses_substituted_temporary_paths_and_rolls_back() {
    let directory = Directory::new();
    let root = directory.child("store");
    let mut store = must(Store::open(&root));
    let secret = key(ObjectKind::Session, b"session");
    must(store.put_local(secret.clone(), b"acknowledged".to_vec()));
    let original = must(fs::read(root.join("store.bin")));
    let sentinel = directory.child("sentinel");
    must(fs::write(&sentinel, b"must not be truncated"));
    let substituted = root.join("store.bin.tmp");
    must(symlink(&sentinel, &substituted));
    assert!(store
        .put_local(secret.clone(), b"rejected".to_vec())
        .is_err());
    assert_eq!(value(&store, &secret), b"acknowledged");
    assert_eq!(must(fs::read(root.join("store.bin"))), original);
    assert_eq!(must(fs::read(&sentinel)), b"must not be truncated");
    assert!(must(fs::symlink_metadata(&substituted))
        .file_type()
        .is_symlink());
    must(fs::remove_file(substituted));
    must(store.put_local(secret.clone(), b"next-acknowledged".to_vec()));
    drop(store);
    let recovered = must(Store::open(&root));
    assert_eq!(value(&recovered, &secret), b"next-acknowledged");
    assert_private(&root.join("store.bin"), 0o600);
}

#[test]
fn store_private_file_modes_refuses_root_replacement_and_permission_changes() {
    let directory = Directory::new();
    let root = directory.child("store");
    let original = produce(&root);
    let mut store = must(Store::open(&root));
    let secret = key(ObjectKind::Session, b"session");
    must(fs::set_permissions(
        &root,
        fs::Permissions::from_mode(0o750),
    ));
    assert!(store
        .put_local(secret.clone(), b"rejected".to_vec())
        .is_err());
    assert_eq!(value(&store, &secret), b"private-bearer-token");
    must(fs::set_permissions(
        &root,
        fs::Permissions::from_mode(0o700),
    ));
    let displaced = directory.child("displaced");
    must(fs::rename(&root, &displaced));
    must(DirBuilder::new().mode(0o700).create(&root));
    assert!(store
        .put_local(secret.clone(), b"rejected".to_vec())
        .is_err());
    assert_eq!(value(&store, &secret), b"private-bearer-token");
    assert_eq!(must(fs::read(displaced.join("store.bin"))), original);
    assert!(!root.join("store.bin").exists());
    drop(store);
    let recovered = must(Store::open(&displaced));
    assert_eq!(value(&recovered, &secret), b"private-bearer-token");
}

#[test]
fn migration_private_file_modes_permissive_umask_preserves_produced_bytes_on_reopen() {
    let directory = Directory::new();
    assert_permissive_umask(&directory);
    for version in 1..CURRENT_SCHEMA_VERSION {
        let root = directory.child(&format!("v{version}"));
        prior_snapshot(&root, version);
        let report = must(migrate_forward(&root));
        assert_eq!(report.original_version, version);
        assert_eq!(report.final_version, CURRENT_SCHEMA_VERSION);
        assert_eq!(
            report.steps.len(),
            must(usize::try_from(CURRENT_SCHEMA_VERSION - version))
        );
        assert_eq!(report.core_produced_records_preserved, 1);
        assert_private(&root, 0o700);
        assert_private(&root.join("store.bin"), 0o600);
        let store = must(Store::open(&root));
        assert_eq!(
            value(&store, &key(ObjectKind::Session, b"session")),
            b"private-bearer-token"
        );
        let receipt = store
            .get(&key(ObjectKind::Receipt, b"receipt"))
            .cloned()
            .unwrap_or_else(|| panic!("receipt absent"));
        assert_eq!(receipt.class(), StorageClass::CoreProducedCache);
        assert_eq!(receipt.bytes(), b"exact-core-receipt");
        drop(store);
        let recovered = must(Store::open(&root));
        assert_eq!(
            value(&recovered, &key(ObjectKind::Receipt, b"receipt")),
            b"exact-core-receipt"
        );
        assert_eq!(must(fs::read_dir(&root)).count(), 1);
    }
}

#[test]
fn migration_private_file_modes_refuses_substituted_temporary_paths() {
    let directory = Directory::new();
    for name in ["store.bin.migrating", "store.bin.tmp"] {
        let root = directory.child(name);
        let original = prior_snapshot(&root, 2);
        let sentinel = directory.child(&format!("sentinel-{name}"));
        must(fs::write(&sentinel, b"untouched migration peer"));
        let substituted = root.join(name);
        must(symlink(&sentinel, &substituted));
        assert!(matches!(migrate_forward(&root), Err(MigrationError::Io(_))));
        assert!(matches!(
            Store::open(&root),
            Err(StoreError::Migration(MigrationError::Io(_)))
        ));
        assert_eq!(must(fs::read(root.join("store.bin"))), original);
        assert_eq!(must(fs::read(&sentinel)), b"untouched migration peer");
        assert!(must(fs::symlink_metadata(&substituted))
            .file_type()
            .is_symlink());
        must(fs::remove_file(substituted));
        must(migrate(&root));
        assert_private(&root.join("store.bin"), 0o600);
        let recovered = must(Store::open(&root));
        assert_eq!(
            value(&recovered, &key(ObjectKind::Session, b"session")),
            b"private-bearer-token"
        );
    }
}

#[test]
fn store_independent_writer_loss_exclusive_opener_includes_migration_and_recovery() {
    let directory = Directory::new();
    let root = directory.child("store");
    let original = prior_snapshot(&root, 2);
    // This is the same kernel lock an independent process takes, with a separately opened fd.
    let independent_writer = must(File::open(&root));
    must(independent_writer.try_lock());
    assert!(
        matches!(Store::open(&root), Err(StoreError::Io(error)) if error.kind() == io::ErrorKind::WouldBlock)
    );
    assert!(
        matches!(migrate_forward(&root), Err(MigrationError::Io(error)) if error.kind() == io::ErrorKind::WouldBlock)
    );
    assert_eq!(must(fs::read(root.join("store.bin"))), original);
    drop(independent_writer);
    let mut writer = must(Store::open(&root));
    must(writer.record_submission(
        tenant(),
        b"first".to_vec(),
        b"signed-first".to_vec(),
        b"queued-first".to_vec(),
    ));
    assert!(
        matches!(Store::open(&root), Err(StoreError::Io(error)) if error.kind() == io::ErrorKind::WouldBlock)
    );
    assert!(
        matches!(migrate(&root), Err(MigrationError::Io(error)) if error.kind() == io::ErrorKind::WouldBlock)
    );
    let reader = must(Store::open_read_only(&root));
    assert_eq!(
        value(&reader, &key(ObjectKind::Outbox, b"first")),
        b"queued-first"
    );
    drop(writer);
    let mut next = must(Store::open(&root));
    must(next.record_submission(
        tenant(),
        b"second".to_vec(),
        b"signed-second".to_vec(),
        b"queued-second".to_vec(),
    ));
    drop(next);
    let recovered = must(Store::open(&root));
    for (id, signed, queued) in [
        (
            b"first".as_slice(),
            b"signed-first".as_slice(),
            b"queued-first".as_slice(),
        ),
        (
            b"second".as_slice(),
            b"signed-second".as_slice(),
            b"queued-second".as_slice(),
        ),
    ] {
        assert_eq!(
            value(&recovered, &key(ObjectKind::PreparedActivity, id)),
            signed
        );
        assert_eq!(value(&recovered, &key(ObjectKind::Outbox, id)), queued);
        assert_eq!(
            value(&recovered, &key(ObjectKind::Idempotency, id)),
            b"pending"
        );
    }
}

#[test]
fn store_independent_writer_loss_simultaneous_openers_accept_exactly_one_writer() {
    let directory = Directory::new();
    let root = directory.child("store");
    produce(&root);
    let barrier = Arc::new(Barrier::new(2));
    let results = std::thread::scope(|scope| {
        let workers: Vec<_> = (0_u8..2)
            .map(|id| {
                let barrier = Arc::clone(&barrier);
                let root = &root;
                scope.spawn(move || {
                    barrier.wait();
                    let opened = Store::open(root);
                    barrier.wait(); // Keep the winner alive until both opens have returned.
                    match opened {
                        Ok(mut store) => {
                            must(store.put_local(key(ObjectKind::Policy, &[id]), vec![id]));
                            Some(id)
                        }
                        Err(StoreError::Io(error)) => {
                            assert_eq!(error.kind(), io::ErrorKind::WouldBlock);
                            None
                        }
                        Err(error) => panic!("unexpected open refusal: {error}"),
                    }
                })
            })
            .collect();
        workers
            .into_iter()
            .map(|worker| must(worker.join()))
            .collect::<Vec<_>>()
    });
    let accepted: Vec<_> = results.into_iter().flatten().collect();
    assert_eq!(accepted.len(), 1);
    let recovered = must(Store::open(&root));
    assert_eq!(
        value(&recovered, &key(ObjectKind::Policy, &[accepted[0]])),
        vec![accepted[0]]
    );
    assert_eq!(
        recovered
            .list_object_ids(&tenant(), ObjectKind::Policy)
            .len(),
        1
    );
    assert_eq!(
        value(&recovered, &key(ObjectKind::Session, b"session")),
        b"private-bearer-token"
    );
}

#[test]
fn store_independent_writer_loss_simultaneous_shared_updates_all_survive_restart() {
    let directory = Directory::new();
    let root = directory.child("store");
    let store = Arc::new(Mutex::new(must(Store::open(&root))));
    let barrier = Arc::new(Barrier::new(8));
    std::thread::scope(|scope| {
        let workers: Vec<_> = (0_u8..8)
            .map(|id| {
                let store = Arc::clone(&store);
                let barrier = Arc::clone(&barrier);
                scope.spawn(move || {
                    barrier.wait();
                    let mut writer = must(store.lock());
                    must(writer.record_submission(tenant(), vec![id], vec![id, 1], vec![id, 2]));
                })
            })
            .collect();
        for worker in workers {
            must(worker.join());
        }
    });
    assert!(
        matches!(Store::open(&root), Err(StoreError::Io(error)) if error.kind() == io::ErrorKind::WouldBlock)
    );
    drop(store);
    let recovered = must(Store::open(&root));
    for id in 0_u8..8 {
        assert_eq!(
            value(&recovered, &key(ObjectKind::PreparedActivity, &[id])),
            vec![id, 1]
        );
        assert_eq!(
            value(&recovered, &key(ObjectKind::Outbox, &[id])),
            vec![id, 2]
        );
        assert_eq!(
            value(&recovered, &key(ObjectKind::Idempotency, &[id])),
            b"pending"
        );
    }
    for kind in [
        ObjectKind::PreparedActivity,
        ObjectKind::Outbox,
        ObjectKind::Idempotency,
    ] {
        assert_eq!(recovered.list_object_ids(&tenant(), kind).len(), 8);
    }
}
