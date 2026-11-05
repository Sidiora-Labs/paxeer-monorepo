use std::error::Error;
use std::fs;
use std::io;
use std::os::unix::fs::{symlink, PermissionsExt as _};
use std::path::{Path, PathBuf};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_human_identity_provider::{Policy, Server, State};
use layerx_human_service::store::crash::{DurabilityStep, StorageCrashHarness};
use layerx_human_service::store::{
    AgentTenantId, PrincipalId, PrincipalStore, PrincipalTenancyAuthority, StoreError, Table,
    TenancyDigest,
};
use layerx_human_test_support::{
    directory, principal, retention_uniform, row_key, tenancy, StepClock,
};
use layerx_identity_binding::{Client, Config};

type Result<T = ()> = std::result::Result<T, Box<dyn Error>>;

#[derive(Debug)]
struct IdentityTenancy(Client);

impl PrincipalTenancyAuthority for IdentityTenancy {
    fn tenant_for(
        &self,
        principal: &PrincipalId,
    ) -> std::result::Result<AgentTenantId, StoreError> {
        let binding = self.0.lookup(principal.as_str())?;
        if binding.principal() != principal.as_str() {
            return Err(StoreError::InvalidPrincipal);
        }
        AgentTenantId::new(binding.agent_tenant())
    }
}

struct IdentityServer {
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<io::Result<()>>>,
}

impl Drop for IdentityServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            assert!(
                matches!(worker.join(), Ok(Ok(()))),
                "identity server failed"
            );
        }
    }
}

struct Fixture {
    base: PathBuf,
    root: PathBuf,
    ids: [PrincipalId; 2],
    digest: TenancyDigest,
    authority: Option<Arc<dyn PrincipalTenancyAuthority>>,
    store: Option<PrincipalStore>,
    server: Option<IdentityServer>,
}

impl Fixture {
    fn new(provider: bool) -> Result<Self> {
        let base = directory("repair-22-33-58");
        fs::create_dir_all(&base)?;
        fs::set_permissions(&base, fs::Permissions::from_mode(0o700))?;
        let base = fs::canonicalize(base)?;
        let root = base.join("store");
        let mut ids = [principal("alice"), principal("bob")];
        let mut server = None;
        let mut authority: Option<Arc<dyn PrincipalTenancyAuthority>> = None;
        if provider {
            let mut state = State::open(
                &base.join("identity"),
                Policy {
                    root: [0x58; 32],
                    threshold: 1,
                    delay_seconds: 86_400,
                },
            )?;
            for (index, id) in ids.iter_mut().enumerate() {
                let mut output = Vec::new();
                state.provision_owner(
                    serde_json::to_vec(&serde_json::json!({
                        "email": format!("owner-{index}@example.test"),
                        "display_name": format!("Owner {index}"),
                        "idempotency_key": format!("repair-22-33-58-{index}"),
                        "now": 1,
                    }))?
                    .as_slice(),
                    &mut output,
                )?;
                let document: serde_json::Value = serde_json::from_slice(&output)?;
                *id = PrincipalId::new(document["principal"].as_str().ok_or("principal missing")?)?;
            }
            let uid = rustix::process::geteuid().as_raw();
            let clock = Arc::new(StepClock::new(1_000_000));
            let listener = Server::bind(
                &base.join("identity.sock"),
                state,
                uid,
                Duration::from_secs(5),
                clock.clone(),
            )?
            .with_binding_reader(
                &base.join("binding.sock"),
                "human-provider",
                &[uid],
            )?;
            let stop = Arc::new(AtomicBool::new(false));
            let shutdown = stop.clone();
            server = Some(IdentityServer {
                stop,
                worker: Some(thread::spawn(move || listener.run(&shutdown))),
            });
            authority = Some(Arc::new(IdentityTenancy(Client::new(
                Config {
                    socket: base.join("binding.sock"),
                    tenant: "human-provider".into(),
                    peer_uid: uid,
                    peer_gid: rustix::process::getegid().as_raw(),
                    deadline: Duration::from_secs(5),
                },
                clock,
            )?)));
        }
        // Provider principals are discovered through the real binding service.
        let map = if provider {
            tenancy(&[])
        } else {
            tenancy(&[("alice", "tenant-a"), ("bob", "tenant-b")])
        };
        let digest = map.install(&root)?;
        let mut fixture = Self {
            base,
            root,
            ids,
            digest,
            authority,
            store: None,
            server,
        };
        fixture.reopen()?;
        Ok(fixture)
    }

    fn open_at(&self, root: &Path) -> std::result::Result<PrincipalStore, StoreError> {
        match &self.authority {
            Some(authority) => PrincipalStore::open_with_authority(
                root,
                retention_uniform(1_000),
                self.digest,
                authority.clone(),
            ),
            None => PrincipalStore::open(root, retention_uniform(1_000), self.digest),
        }
    }

    fn reopen(&mut self) -> Result {
        self.store = None;
        self.store = Some(self.open_at(&self.root)?);
        Ok(())
    }

    fn store(&mut self) -> &mut PrincipalStore {
        self.store
            .as_mut()
            .unwrap_or_else(|| panic!("store is stopped"))
    }

    fn path(&self, index: usize) -> PathBuf {
        self.root.join("principals").join(self.ids[index].as_str())
    }

    fn put(&mut self, index: usize, value: &[u8]) -> Result {
        let id = self.ids[index].clone();
        self.store()
            .principal(&id)?
            .put(Table::Journeys, row_key("secret"), 42, value.to_vec())?;
        Ok(())
    }

    fn assert_row(&mut self, index: usize, value: &[u8]) -> Result {
        let id = self.ids[index].clone();
        let scope = self.store().principal(&id)?;
        let row = scope
            .get(Table::Journeys, &row_key("secret"))
            .ok_or("acknowledged row missing")?;
        assert_eq!(row.bytes(), value);
        assert_eq!(row.written_at(), 42);
        Ok(())
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        self.store = None;
        self.authority = None;
        self.server = None;
        assert!(
            fs::remove_dir_all(&self.base).is_ok(),
            "fixture cleanup failed"
        );
    }
}

#[test]
fn store_new_directory_durability_first_principal_restart() -> Result {
    for provider in [false, true] {
        let mut fixture = Fixture::new(provider)?;
        assert!(!fixture.root.join("principals").exists());
        let harness = StorageCrashHarness::arm(None);
        fixture.put(0, b"acknowledged first row")?;
        let steps = harness.steps();
        drop(harness);
        for name in ["principals", fixture.ids[0].as_str()] {
            let created = steps
                .iter()
                .position(|step| step == &DurabilityStep::DirectoryCreated(name.into()))
                .ok_or("new directory was not recorded")?;
            assert_eq!(
                steps.get(created + 1),
                Some(&DurabilityStep::ParentSynced(name.into()))
            );
        }
        assert_eq!(
            &steps[steps.len() - 3..],
            &[
                DurabilityStep::TemporarySynced,
                DurabilityStep::Published,
                DurabilityStep::DirectorySynced,
            ]
        );
        fixture.reopen()?;
        fixture.assert_row(0, b"acknowledged first row")?;
        let id = fixture.ids[1].clone();
        assert!(fixture
            .store()
            .principal(&id)?
            .get(Table::Journeys, &row_key("secret"))
            .is_none());

        // Stop and reopen the real store after every first-write boundary.
        for boundary in 1..=steps.len() {
            let mut crashed = Fixture::new(provider)?;
            let harness = StorageCrashHarness::arm(Some(boundary));
            assert!(crashed.put(0, b"unacknowledged first row").is_err());
            assert_eq!(harness.steps().len(), boundary);
            drop(harness);
            crashed.reopen()?;
            let id = crashed.ids[0].clone();
            {
                let scope = crashed.store().principal(&id)?;
                if let Some(row) = scope.get(Table::Journeys, &row_key("secret")) {
                    assert_eq!(row.bytes(), b"unacknowledged first row");
                    assert_eq!(row.written_at(), 42);
                }
            }
            crashed.put(0, b"acknowledged retry")?;
            crashed.reopen()?;
            crashed.assert_row(0, b"acknowledged retry")?;
        }
    }
    Ok(())
}

#[test]
fn store_new_directory_durability_atomic_replacement_restart() -> Result {
    for provider in [false, true] {
        for boundary in 1..=3 {
            let mut fixture = Fixture::new(provider)?;
            fixture.put(0, b"previous acknowledged row")?;
            let harness = StorageCrashHarness::arm(Some(boundary));
            assert!(fixture.put(0, b"replacement").is_err());
            assert_eq!(harness.steps().len(), boundary);
            drop(harness);
            fixture.reopen()?;
            fixture.assert_row(
                0,
                if boundary == 1 {
                    b"previous acknowledged row"
                } else {
                    b"replacement"
                },
            )?;
            fixture.put(0, b"final acknowledged row")?;
            fixture.reopen()?;
            fixture.assert_row(0, b"final acknowledged row")?;
        }
    }
    Ok(())
}

#[test]
fn store_principal_symlink_aliases_static_and_provider() -> Result {
    for provider in [false, true] {
        for outside in [false, true] {
            let mut fixture = Fixture::new(provider)?;
            fixture.put(1, b"bob private rows")?;
            fixture.put(0, b"alice private rows")?;
            let alice = fixture.path(0);
            let bob = fixture.path(1);
            let target = if outside {
                let external = fixture.base.join("outside");
                fs::rename(&bob, &external)?;
                external
            } else {
                bob.clone()
            };
            let original = fs::read(target.join("store.bin"))?;
            let saved = fixture.base.join("alice-saved");
            fs::rename(&alice, &saved)?;
            symlink(&target, &alice)?;
            let id = fixture.ids[0].clone();
            assert!(fixture.store().principal(&id).is_err());
            assert!(fixture.store().probe().is_err());
            fixture.store = None;
            assert!(fixture.open_at(&fixture.root).is_err());
            assert_eq!(fs::read(target.join("store.bin"))?, original);
            fs::remove_file(&alice)?;
            fs::rename(&saved, &alice)?;
            if outside {
                fs::rename(&target, &bob)?;
            }
            fixture.reopen()?;
            fixture.assert_row(0, b"alice private rows")?;
            fixture.assert_row(1, b"bob private rows")?;
        }
    }
    Ok(())
}

#[test]
fn store_principal_parent_replacement_static_and_provider() -> Result {
    for provider in [false, true] {
        for replace_root in [false, true] {
            let mut fixture = Fixture::new(provider)?;
            fixture.put(0, b"alice acknowledged")?;
            fixture.put(1, b"bob acknowledged")?;
            let id = fixture.ids[0].clone();
            let bob_before = fs::read(fixture.path(1).join("store.bin"))?;
            let parent = if replace_root {
                fixture.root.clone()
            } else {
                fixture.root.join("principals")
            };
            let saved = fixture.base.join("saved-parent");
            let replacement = fixture.base.join("replacement");
            fs::create_dir(&replacement)?;
            // A hostile parent points at a genuine store tree containing another principal.
            if replace_root {
                fs::create_dir(replacement.join("principals"))?;
            }
            let foreign = if replace_root {
                replacement.join("principals")
            } else {
                replacement.clone()
            };
            fs::create_dir(foreign.join(id.as_str()))?;
            fs::write(foreign.join(id.as_str()).join("store.bin"), &bob_before)?;
            let root = fixture.root.clone();
            let mut scope = fixture.store().principal(&id)?;
            fs::rename(&parent, &saved)?;
            symlink(&replacement, &parent)?;
            assert!(scope
                .put(
                    Table::Journeys,
                    row_key("secret"),
                    43,
                    b"must not reach bob".to_vec()
                )
                .is_err());
            assert_eq!(
                scope
                    .get(Table::Journeys, &row_key("secret"))
                    .ok_or("cached row lost")?
                    .bytes(),
                b"alice acknowledged"
            );
            drop(scope);
            assert!(fixture.store().principal(&id).is_err());
            assert!(fixture.store().probe().is_err());
            assert_eq!(
                fs::read(foreign.join(id.as_str()).join("store.bin"))?,
                bob_before
            );
            fixture.store = None;
            assert!(fixture.open_at(&fixture.root).is_err());
            fs::remove_file(&parent)?;
            fs::rename(&saved, &parent)?;
            assert_eq!(fixture.root, root);
            fixture.reopen()?;
            fixture.assert_row(0, b"alice acknowledged")?;
            fixture.assert_row(1, b"bob acknowledged")?;
        }
    }
    Ok(())
}

#[test]
fn store_principal_file_links_and_ownership_static_and_provider() -> Result {
    for provider in [false, true] {
        for hard_link in [false, true] {
            for name in ["store.bin", "store.bin.tmp"] {
                let mut fixture = Fixture::new(provider)?;
                fixture.put(0, b"alice acknowledged")?;
                fixture.put(1, b"bob private rows")?;
                let victim = fixture.path(1).join("store.bin");
                let original = fs::read(&victim)?;
                let attack = fixture.path(0).join(name);
                if attack.exists() {
                    fs::remove_file(&attack)?;
                }
                if hard_link {
                    fs::hard_link(&victim, &attack)?;
                } else {
                    symlink(&victim, &attack)?;
                }
                if name == "store.bin" {
                    let id = fixture.ids[0].clone();
                    assert!(fixture.store().principal(&id).is_err());
                } else {
                    assert!(fixture.put(0, b"must not overwrite victim").is_err());
                }
                assert_eq!(fs::read(&victim)?, original);
            }
        }
        let mut fixture = Fixture::new(provider)?;
        fixture.put(0, b"alice acknowledged")?;
        let id = fixture.ids[0].clone();
        let path = fixture.path(0);
        fs::set_permissions(&path, fs::Permissions::from_mode(0o777))?;
        assert!(fixture.store().principal(&id).is_err());
        fs::set_permissions(&path, fs::Permissions::from_mode(0o700))?;
        let path = path.join("store.bin");
        fs::set_permissions(&path, fs::Permissions::from_mode(0o666))?;
        assert!(fixture.store().principal(&id).is_err());
    }
    Ok(())
}

#[test]
fn store_principal_provider_binding_links_are_refused() -> Result {
    for name in ["provider-binding", "provider-binding.pending"] {
        for hard_link in [false, true] {
            let mut fixture = Fixture::new(true)?;
            fixture.put(0, b"alice acknowledged")?;
            fixture.put(1, b"bob private rows")?;
            let victim = fixture.path(1).join("provider-binding");
            let original = fs::read(&victim)?;
            let binding = fixture.path(0).join("provider-binding");
            fs::remove_file(&binding)?;
            let attack = fixture.path(0).join(name);
            if hard_link {
                fs::hard_link(&victim, &attack)?;
            } else {
                symlink(&victim, &attack)?;
            }
            let id = fixture.ids[0].clone();
            assert!(fixture.store().principal(&id).is_err());
            fixture.store = None;
            assert!(fixture.open_at(&fixture.root).is_err());
            assert_eq!(fs::read(&victim)?, original);
        }
    }
    Ok(())
}
