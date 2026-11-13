use std::error::Error;
use std::fs;
use std::io;
use std::os::unix::fs::{symlink, PermissionsExt as _};
use std::path::Path;
use std::process::Command;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_human_identity_provider::{Policy, Server, State};
use layerx_human_service::audit::{
    verify_export, AuditChain, AuditError, AuditEvent, AuthMethod, ChainHead, Decision,
};
use layerx_human_service::store::{
    AgentTenantId, EvidenceRef, PrincipalId, PrincipalStore, PrincipalTenancyAuthority, StoreError,
    Table, TenancyDigest,
};
use layerx_human_service::trace::TraceId;
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, row_key, tenancy, StepClock,
};
use layerx_identity_binding::{Client, Config};

type Result<T = ()> = std::result::Result<T, Box<dyn Error>>;

// A concurrent spawn can inherit another fixture's flock until the child execs.
static FIXTURE_LIFETIMES: Mutex<()> = Mutex::new(());

fn assert_contended(result: std::result::Result<PrincipalStore, StoreError>) {
    match result {
        Err(StoreError::Io(error)) if error.kind() == io::ErrorKind::WouldBlock => {}
        unexpected => panic!("competing owner was not explicitly refused: {unexpected:?}"),
    }
}

fn open(root: &Path, digest: TenancyDigest) -> Result<PrincipalStore> {
    Ok(PrincipalStore::open(
        root,
        retention_uniform(1_000),
        digest,
    )?)
}

fn write_audited_row(
    store: &mut PrincipalStore,
    id: &PrincipalId,
    key: &str,
    stamp: u8,
) -> Result<ChainHead> {
    let mut scope = store.principal(id)?;
    let mut chain = AuditChain::open(&scope)?;
    let key = row_key(key);
    Ok(scope.transaction::<_, AuditError>(|staged| {
        staged.put(Table::Journeys, key.clone(), u64::from(stamp), vec![stamp])?;
        chain.append(
            staged,
            u64::from(stamp),
            &TraceId::mint([stamp; 16]),
            &AuditEvent::Authentication {
                method: AuthMethod::Passkey,
                outcome: Decision::Granted,
            },
            &[EvidenceRef::new(Table::Journeys, key)],
        )
    })?)
}

fn assert_recovered(
    store: &mut PrincipalStore,
    id: &PrincipalId,
    head: Option<ChainHead>,
) -> Result {
    let scope = store.principal(id)?;
    assert_eq!(
        scope.keys(Table::Journeys),
        vec![row_key("x"), row_key("y")]
    );
    for (key, stamp) in [("x", 1), ("y", 2)] {
        let row = scope
            .get(Table::Journeys, &row_key(key))
            .ok_or("acknowledged row lost")?;
        assert_eq!(row.bytes(), &[stamp]);
        assert_eq!(row.written_at(), u64::from(stamp));
    }
    let chain = match head {
        Some(head) => AuditChain::open_anchored(&scope, head)?,
        None => AuditChain::open(&scope)?,
    };
    let entries = chain.entries(&scope)?;
    assert_eq!(entries.len(), 2);
    assert_eq!(entries[0].sequence(), 0);
    assert_eq!(entries[1].sequence(), 1);
    assert_eq!(entries[1].prev_link(), entries[0].link());
    for (entry, key) in entries.iter().zip(["x", "y"]) {
        assert_eq!(entry.evidence().len(), 1);
        assert_eq!(entry.evidence()[0].key(), &row_key(key));
    }
    let report = verify_export(&chain.export(&scope)?)?;
    assert_eq!(report.entries(), 2);
    assert_eq!(report.evidence_rows(), 2);
    assert_eq!(report.head(), chain.head());
    Ok(())
}

#[test]
fn store_independent_writer_loss_recovery() -> Result {
    let _fixture = FIXTURE_LIFETIMES.lock().or_fail("fixture lock poisoned");
    let root = directory("repair-22-31-158-handles");
    let map = tenancy(&[("alice", "tenant-a")]);
    let (mut first, digest) = install_and_open(&root, &map, retention_uniform(1_000));
    assert_contended(PrincipalStore::open(
        &root,
        retention_uniform(1_000),
        digest,
    ));
    let alias = root.with_extension("alias");
    symlink(&root, &alias)?;
    assert_contended(PrincipalStore::open(
        &alias,
        retention_uniform(1_000),
        digest,
    ));
    let first_head = write_audited_row(&mut first, &principal("alice"), "x", 1)?;
    drop(first);
    let mut second = open(&root, digest)?;
    {
        let scope = second.principal(&principal("alice"))?;
        assert_eq!(
            AuditChain::open_anchored(&scope, first_head)?.head(),
            first_head
        );
    }
    let final_head = write_audited_row(&mut second, &principal("alice"), "y", 2)?;
    drop(second);
    let mut recovered = open(&root, digest)?;
    assert_recovered(&mut recovered, &principal("alice"), Some(final_head))?;
    drop(recovered);
    fs::remove_file(alias)?;
    fs::remove_dir_all(root)?;
    Ok(())
}

#[test]
fn store_independent_writer_loss_process_recovery() -> Result {
    const CHILD_ROOT: &str = "REPAIR_22_31_158_CHILD_ROOT";
    const CHILD_MODE: &str = "REPAIR_22_31_158_CHILD_MODE";
    let _fixture = FIXTURE_LIFETIMES.lock().or_fail("fixture lock poisoned");
    let map = tenancy(&[("alice", "tenant-a")]);
    let digest = map.digest()?;
    if let Some(root) = std::env::var_os(CHILD_ROOT) {
        let mode = std::env::var(CHILD_MODE)?;
        let attempt = PrincipalStore::open(Path::new(&root), retention_uniform(1_000), digest);
        if mode == "contended" {
            assert_contended(attempt);
            return Ok(());
        }
        assert_eq!(mode, "write-and-exit");
        let mut store = attempt?;
        let head = write_audited_row(&mut store, &principal("alice"), "y", 2)?;
        assert_eq!(head.length(), 2);
        assert_recovered(&mut store, &principal("alice"), Some(head))?;
        // Exit with the ownership descriptor still live, as after a process stop.
        std::process::exit(0);
    }
    let root = directory("repair-22-31-158-process");
    let (mut first, _) = install_and_open(&root, &map, retention_uniform(1_000));
    let child = |mode: &str| -> Result {
        let status = Command::new(std::env::current_exe()?)
            .args([
                "--exact",
                "store_independent_writer_loss_process_recovery",
                "--nocapture",
            ])
            .env(CHILD_ROOT, &root)
            .env(CHILD_MODE, mode)
            .status()?;
        assert!(status.success(), "store subprocess failed: {status}");
        Ok(())
    };
    child("contended")?;
    write_audited_row(&mut first, &principal("alice"), "x", 1)?;
    drop(first);
    child("write-and-exit")?;
    let mut recovered = open(&root, digest)?;
    assert_recovered(&mut recovered, &principal("alice"), None)?;
    drop(recovered);
    fs::remove_dir_all(root)?;
    Ok(())
}

#[derive(Debug)]
struct IdentityTenancy(Client);

impl PrincipalTenancyAuthority for IdentityTenancy {
    fn tenant_for(&self, id: &PrincipalId) -> std::result::Result<AgentTenantId, StoreError> {
        let binding = self.0.lookup(id.as_str())?;
        if binding.principal() != id.as_str() {
            return Err(StoreError::InvalidPrincipal);
        }
        AgentTenantId::new(binding.agent_tenant())
    }
}

struct RunningIdentity {
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<io::Result<()>>>,
}

impl Drop for RunningIdentity {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            assert!(worker.join().or_fail("identity worker panicked").is_ok());
        }
    }
}

#[test]
fn store_independent_writer_loss_all_open_modes() -> Result {
    let _fixture = FIXTURE_LIFETIMES.lock().or_fail("fixture lock poisoned");
    let root = directory("repair-22-31-158-provider");
    fs::create_dir_all(&root)?;
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700))?;
    let root = fs::canonicalize(root)?;
    let mut identity = State::open(
        &root.join("identity"),
        Policy {
            root: [0x43; 32],
            threshold: 1,
            delay_seconds: 86_400,
        },
    )?;
    let mut provisioned = Vec::new();
    identity.provision_owner(
        serde_json::to_vec(&serde_json::json!({
            "email": "writer@example.test", "display_name": "Writer",
            "idempotency_key": "repair-22-31-158", "now": 1,
        }))?
        .as_slice(),
        &mut provisioned,
    )?;
    let document: serde_json::Value = serde_json::from_slice(&provisioned)?;
    let id = PrincipalId::new(document["principal"].as_str().ok_or("principal missing")?)?;
    let uid = rustix::process::geteuid().as_raw();
    let clock = Arc::new(StepClock::new(1_000_000));
    let server = Server::bind(
        &root.join("identity.sock"),
        identity,
        uid,
        Duration::from_secs(5),
        clock.clone(),
    )?
    .with_binding_reader(&root.join("binding.sock"), "human-provider", &[uid])?;
    let shutdown = Arc::new(AtomicBool::new(false));
    let stop = Arc::clone(&shutdown);
    let running = RunningIdentity {
        shutdown,
        worker: Some(thread::spawn(move || server.run(&stop))),
    };
    let client = Client::new(
        Config {
            socket: root.join("binding.sock"),
            tenant: "human-provider".into(),
            peer_uid: uid,
            peer_gid: rustix::process::getegid().as_raw(),
            deadline: Duration::from_secs(5),
        },
        clock,
    )?;
    let tenant = client.lookup(id.as_str())?;
    let map = tenancy(&[(id.as_str(), tenant.agent_tenant())]);
    let store_root = root.join("store");
    let (mut first, digest) = install_and_open(&store_root, &map, retention_uniform(1_000));
    let authority = Arc::new(IdentityTenancy(client));
    assert_contended(PrincipalStore::open_with_authority(
        &store_root,
        retention_uniform(1_000),
        digest,
        authority.clone(),
    ));
    let first_head = write_audited_row(&mut first, &id, "x", 1)?;
    drop(first);
    let mut second = PrincipalStore::open_with_authority(
        &store_root,
        retention_uniform(1_000),
        digest,
        authority.clone(),
    )?;
    assert_contended(PrincipalStore::open(
        &store_root,
        retention_uniform(1_000),
        digest,
    ));
    assert_contended(PrincipalStore::open_with_authority(
        &store_root,
        retention_uniform(1_000),
        digest,
        authority.clone(),
    ));
    {
        let scope = second.principal(&id)?;
        assert_eq!(
            AuditChain::open_anchored(&scope, first_head)?.head(),
            first_head
        );
    }
    let final_head = write_audited_row(&mut second, &id, "y", 2)?;
    drop(second);
    let mut recovered = PrincipalStore::open_with_authority(
        &store_root,
        retention_uniform(1_000),
        digest,
        authority,
    )?;
    assert_recovered(&mut recovered, &id, Some(final_head))?;
    drop(recovered);
    drop(running);
    fs::remove_dir_all(root)?;
    Ok(())
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
