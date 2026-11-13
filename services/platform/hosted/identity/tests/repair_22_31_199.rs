use layerx_platform_identity::store::{PendingSignup, StarterCredit, Store, StoredSession};
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

const DAY: u64 = 86_400;

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = Path::new(env!("CARGO_TARGET_TMPDIR")).join(format!(
            "repair-22-31-199-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).unwrap_or_else(|error| panic!("directory: {error}"));
        Self(path)
    }

    fn open(&self) -> Store {
        Store::open(&self.0).unwrap_or_else(|error| panic!("open: {error}"))
    }

    fn read(&self, name: &str) -> Vec<u8> {
        fs::read(self.0.join(name)).unwrap_or_else(|error| panic!("read {name}: {error}"))
    }

    fn write(&self, name: &str, bytes: &[u8]) {
        fs::write(self.0.join(name), bytes).unwrap_or_else(|error| panic!("write {name}: {error}"));
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap_or_else(|error| panic!("cleanup: {error}"));
    }
}

fn signup(id: &str, day: u64) -> PendingSignup {
    PendingSignup {
        token_digest: format!("token-{id}"),
        email_digest: format!("email-{id}"),
        tenant: "beta".to_owned(),
        sub: format!("did:key:{id}"),
        signer_public_key: "ab".repeat(32),
        created_at: day * DAY + 10,
        expires_at: (day + 1) * DAY + 10,
    }
}

fn put_signup(store: &mut Store, signup: &PendingSignup) {
    store
        .put_signup(signup.clone())
        .unwrap_or_else(|error| panic!("signup: {error}"));
}

fn verify(store: &mut Store, signup: &PendingSignup) {
    let principal = store
        .verify_signup(&signup.token_digest, signup.created_at + 1)
        .unwrap_or_else(|error| panic!("verify: {error}"));
    assert_eq!(principal.sub, signup.sub);
    assert_eq!(principal.tenant, signup.tenant);
}

fn assert_verified(store: &Store, signup: &PendingSignup) {
    assert_eq!(store.pending_signup(&signup.token_digest), None);
    assert_eq!(
        store.verified_email(&signup.sub),
        Some(signup.email_digest.as_str())
    );
    assert!(store.email_in_use(&signup.email_digest));
    let principal = store
        .principal(&signup.tenant, &signup.sub)
        .unwrap_or_else(|| panic!("verified principal missing"));
    assert_eq!(
        principal.allowed_signer_public_keys,
        vec![signup.signer_public_key.clone()]
    );
}

#[test]
fn identity_compaction_replay_verify_after_snapshotted_signup() {
    let directory = Directory::new();
    let signup = signup("snapshotted", 3);
    let mut store = directory.open();
    put_signup(&mut store, &signup);
    drop(store);

    let mut store = directory.open();
    assert_eq!(store.pending_signup(&signup.token_digest), Some(&signup));
    assert_eq!(store.signups_on_day(3), 1);
    assert!(directory.read("journal.log").is_empty());
    verify(&mut store, &signup);
    drop(store);
    let verify_journal = directory.read("journal.log");
    assert_eq!(verify_journal.split(|byte| *byte == b'\n').count() - 1, 1);

    let store = directory.open();
    assert_verified(&store, &signup);
    drop(store);
    for _ in 0..3 {
        directory.write("journal.log", &verify_journal);
        let mut store = directory.open();
        assert_verified(&store, &signup);
        assert_eq!(store.signups_on_day(3), 1);
        assert!(store
            .verify_signup(&signup.token_digest, 3 * DAY + 20)
            .is_err());
        assert!(directory.read("journal.log").is_empty());
        drop(store);
    }

    let next = signup_for_next_day();
    let mut store = directory.open();
    put_signup(&mut store, &next);
    verify(&mut store, &next);
    drop(store);
    let store = directory.open();
    assert_verified(&store, &signup);
    assert_verified(&store, &next);
    assert_eq!(store.signups_on_day(3), 1);
    assert_eq!(store.signups_on_day(4), 1);
}

fn signup_for_next_day() -> PendingSignup {
    signup("next-day", 4)
}

#[test]
fn identity_compaction_replay_signup_counts_once_at_rollover_boundaries() {
    let directory = Directory::new();
    let first = signup("first", 7);
    let second = signup("second", 7);
    let mut store = directory.open();
    put_signup(&mut store, &first);
    drop(store);
    let journal = directory.read("journal.log");
    let old_snapshot = directory.read("snapshot.json");

    drop(directory.open());
    let replayed_snapshot = directory.read("snapshot.json");
    directory.write("snapshot.json", &old_snapshot);
    directory.write("snapshot.json.tmp", &replayed_snapshot);
    directory.write("journal.log", &journal);
    let store = directory.open();
    assert_eq!(store.pending_signup(&first.token_digest), Some(&first));
    assert_eq!(store.signups_on_day(7), 1);
    drop(store);

    directory.write("journal.log", &journal);
    let mut store = directory.open();
    assert_eq!(store.pending_signup(&first.token_digest), Some(&first));
    assert_eq!(store.signups_on_day(7), 1);
    put_signup(&mut store, &second);
    verify(&mut store, &first);
    verify(&mut store, &second);
    drop(store);
    let store = directory.open();
    assert_verified(&store, &first);
    assert_verified(&store, &second);
    assert_eq!(store.signups_on_day(7), 2);
    assert_eq!(store.signups_on_day(6), 0);
    assert_eq!(store.signups_on_day(8), 0);
}

#[test]
fn identity_compaction_replay_preserves_acknowledged_credit_and_session_state() {
    let directory = Directory::new();
    let signup = signup("all-records", 8);
    let mut store = directory.open();
    put_signup(&mut store, &signup);
    verify(&mut store, &signup);
    let mut credit = StarterCredit {
        sub: signup.sub.clone(),
        grant_id: "grant".to_owned(),
        amount: "1000000".to_owned(),
        granted_at: 8 * DAY + 12,
        settled_at: None,
    };
    store
        .put_starter_credit(credit.clone())
        .unwrap_or_else(|error| panic!("credit: {error}"));
    store
        .reissue_starter_credit(&signup.sub, "2000000")
        .unwrap_or_else(|error| panic!("reissue: {error}"));
    store
        .settle_starter_credit(&signup.sub, 8 * DAY + 14)
        .unwrap_or_else(|error| panic!("settle: {error}"));
    credit.amount = "2000000".to_owned();
    credit.settled_at = Some(8 * DAY + 14);
    let mut session = StoredSession {
        session_id: "session".to_owned(),
        tenant: signup.tenant.clone(),
        principal: signup.sub.clone(),
        token_digest: "11".repeat(32),
        csrf_digest: "22".repeat(32),
        csrf_sealed: "33".repeat(48),
        issued_at: 8 * DAY + 12,
        expires_at: 9 * DAY,
        revoked_at: None,
    };
    store
        .put_session(session.clone(), session.issued_at)
        .unwrap_or_else(|error| panic!("session: {error}"));
    assert_eq!(
        store
            .revoke_session(&session.session_id, 8 * DAY + 15)
            .unwrap_or_else(|error| panic!("revoke: {error}")),
        Some(8 * DAY + 15)
    );
    session.revoked_at = Some(8 * DAY + 15);
    store
        .bind_publication_key("key".to_owned(), signup.sub.clone(), false)
        .unwrap_or_else(|error| panic!("bind: {error}"));
    store
        .bind_publication_key("key".to_owned(), signup.sub.clone(), true)
        .unwrap_or_else(|error| panic!("revoke key: {error}"));
    drop(store);
    let journal = directory.read("journal.log");
    drop(directory.open());
    directory.write("journal.log", &journal);
    let store = directory.open();
    assert_verified(&store, &signup);
    assert_eq!(store.signups_on_day(8), 1);
    assert_eq!(store.starter_credit(&signup.sub), Some(&credit));
    assert_eq!(store.session(&session.session_id), Some(&session));
    assert_eq!(store.publication_principal("key"), None);
    drop(store);
    let store = directory.open();
    assert_eq!(store.starter_credit(&signup.sub), Some(&credit));
    assert_eq!(store.session(&session.session_id), Some(&session));
    assert_eq!(store.signups_on_day(8), 1);
}

#[test]
fn identity_compaction_replay_discards_torn_stale_tail() {
    let directory = Directory::new();
    let signup = signup("torn", 9);
    let mut store = directory.open();
    put_signup(&mut store, &signup);
    drop(store);
    let mut store = directory.open();
    verify(&mut store, &signup);
    drop(store);
    let journal = directory.read("journal.log");
    drop(directory.open());
    let mut interrupted = journal.clone();
    interrupted.extend_from_slice(&journal[..journal.len() / 2]);
    directory.write("journal.log", &interrupted);
    let store = directory.open();
    assert_verified(&store, &signup);
    assert_eq!(store.signups_on_day(9), 1);
}

#[test]
fn identity_compaction_replay_refuses_future_generation_and_unknown_verification() {
    let source = Directory::new();
    let signup = signup("unknown", 10);
    let mut store = source.open();
    put_signup(&mut store, &signup);
    drop(store);
    let old_snapshot = source.read("snapshot.json");
    let mut store = source.open();
    verify(&mut store, &signup);
    drop(store);
    let journal = source.read("journal.log");
    let current_snapshot = source.read("snapshot.json");
    source.write("snapshot.json", &old_snapshot);
    assert!(
        matches!(Store::open(&source.0), Err(error) if error == "journal generation is ahead of the snapshot")
    );
    assert_eq!(source.read("journal.log"), journal);
    source.write("snapshot.json", &current_snapshot);
    assert_verified(&source.open(), &signup);

    let target = Directory::new();
    drop(target.open());
    drop(target.open());
    target.write("journal.log", &journal);
    assert!(
        matches!(Store::open(&target.0), Err(error) if error == "verification references no pending signup")
    );
}

fn legacy_rollover(snapshotted_signup: bool) {
    let directory = Directory::new();
    let signup = signup("legacy", 11);
    let mut store = directory.open();
    put_signup(&mut store, &signup);
    if snapshotted_signup {
        drop(store);
        store = directory.open();
        assert_eq!(store.pending_signup(&signup.token_digest), Some(&signup));
    }
    verify(&mut store, &signup);
    drop(store);

    let mut snapshot: serde_json::Value = serde_json::from_slice(&directory.read("snapshot.json"))
        .unwrap_or_else(|error| panic!("decode snapshot: {error}"));
    snapshot
        .as_object_mut()
        .unwrap_or_else(|| panic!("snapshot object"))
        .remove("generation");
    let legacy_snapshot = serde_json::to_vec(&snapshot)
        .unwrap_or_else(|error| panic!("encode legacy snapshot: {error}"));
    let mut legacy_journal = Vec::new();
    for line in directory
        .read("journal.log")
        .split(|byte| *byte == b'\n')
        .filter(|line| !line.is_empty())
    {
        let envelope: serde_json::Value =
            serde_json::from_slice(line).unwrap_or_else(|error| panic!("decode journal: {error}"));
        let record = envelope
            .get("record")
            .unwrap_or_else(|| panic!("producer record missing"));
        serde_json::to_writer(&mut legacy_journal, record)
            .unwrap_or_else(|error| panic!("encode legacy record: {error}"));
        legacy_journal.push(b'\n');
    }
    directory.write("snapshot.json", &legacy_snapshot);
    directory.write("journal.log", &legacy_journal);

    fs::create_dir(directory.0.join("snapshot.json.tmp"))
        .unwrap_or_else(|error| panic!("block snapshot publication: {error}"));
    assert!(matches!(Store::open(&directory.0), Err(error) if error.starts_with("snapshot:")));
    assert_eq!(directory.read("snapshot.json"), legacy_snapshot);
    let migrated_journal = directory.read("journal.log");
    assert_ne!(migrated_journal, legacy_journal);
    fs::remove_dir(directory.0.join("snapshot.json.tmp"))
        .unwrap_or_else(|error| panic!("unblock snapshot publication: {error}"));

    let store = directory.open();
    assert_verified(&store, &signup);
    assert_eq!(store.signups_on_day(11), 1);
    drop(store);
    directory.write("journal.log", &migrated_journal);
    let store = directory.open();
    assert_verified(&store, &signup);
    assert_eq!(store.signups_on_day(11), 1);
    drop(store);
    let store = directory.open();
    assert_verified(&store, &signup);
    assert_eq!(store.signups_on_day(11), 1);
}

#[test]
fn identity_compaction_replay_legacy_verify_after_snapshotted_signup() {
    legacy_rollover(true);
}

#[test]
fn identity_compaction_replay_legacy_signup_count_and_migration_restart() {
    legacy_rollover(false);
}
