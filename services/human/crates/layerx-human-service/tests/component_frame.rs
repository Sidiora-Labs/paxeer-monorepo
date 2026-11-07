use std::fmt::Debug;
use std::fs::{self, DirBuilder};
use std::io;
use std::os::unix::fs::DirBuilderExt as _;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_human_service::auth::{AuthConfig, Passkeys, RateLimit};
use layerx_human_service::frame::Limits;
use layerx_human_service::server::{
    default_component_limits, AuthorizationGrantPolicy, ComponentServerConfig, ComponentState,
    HumanApiComponents, HumanComponentServer, IdentityServices, PrivilegedHumanComponents,
    ProvisionedAccount, ProvisionedAccounts, Readiness, UnixComponents,
};
use layerx_human_test_support::{install_and_open, retention_uniform, tenancy};

#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod clock_server;

const ACCOUNT_ID: &str = "act_00112233445566778899aabbccddeeff";
const TRACE: &str = "trc_00112233445566778899aabbccddeeff";

fn required<T, E: Debug>(result: Result<T, E>, label: &str) -> T {
    result.unwrap_or_else(|error| panic!("{label}: {error:?}"))
}

struct TestDirectory(PathBuf);

impl TestDirectory {
    fn new() -> io::Result<Self> {
        let mut entropy = [0_u8; 16];
        getrandom::fill(&mut entropy).map_err(|error| io::Error::other(error.to_string()))?;
        let path = std::env::temp_dir().join(format!("hcf-{:032x}", u128::from_be_bytes(entropy)));
        DirBuilder::new().mode(0o700).create(&path)?;
        Ok(Self(path))
    }
}

impl Drop for TestDirectory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

struct RunningClock {
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<io::Result<()>>>,
}

impl RunningClock {
    fn start(root: &Path) -> io::Result<(Self, ClockBinding)> {
        let mut server = clock_server::Server::bind(root)?;
        let binding = ClockBinding {
            socket: server.socket().to_path_buf(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        };
        let shutdown = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&shutdown);
        let worker = thread::spawn(move || {
            while !stop.load(Ordering::Acquire) {
                server.poll()?;
                thread::sleep(Duration::from_millis(1));
            }
            Ok(())
        });
        Ok((
            Self {
                shutdown,
                worker: Some(worker),
            },
            binding,
        ))
    }
}

impl Drop for RunningClock {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

#[test]
fn unix_components_reach_the_component_server_over_the_human_frame_module() {
    let directory = required(TestDirectory::new(), "private test directory");
    let (_clock_server, binding) = required(RunningClock::start(&directory.0), "runtime clock");
    let clock = required(RuntimeClock::connect(binding), "clock client");
    let map = tenancy(&[(ACCOUNT_ID, "component-frame")]);
    let (store, _) = install_and_open(&directory.0.join("store"), &map, retention_uniform(86_400));
    let passkeys = required(
        Passkeys::new(AuthConfig {
            rp_id: "wallet.example".to_owned(),
            rp_name: "LayerX".to_owned(),
            origin: "https://wallet.example".to_owned(),
            ceremony_ttl_secs: 300,
            assertion_ttl_secs: 60,
            session_ttl_secs: 300,
            refresh_ttl_secs: 3_600,
            step_up_ttl_secs: 60,
            rate_limit: RateLimit {
                attempts: 100,
                window_secs: 60,
            },
        }),
        "passkeys",
    );
    let account = required(
        ProvisionedAccount::new(ACCOUNT_ID, "owner@wallet.example", "Owner", "Passkey"),
        "provisioned account",
    );
    let accounts = required(ProvisionedAccounts::new([account]), "account directory");
    let components = required(
        PrivilegedHumanComponents::new(
            store,
            passkeys,
            IdentityServices::new(accounts),
            AuthorizationGrantPolicy {
                lifetime_seconds: 30,
                maximum_outstanding: 32,
            },
            clock,
        ),
        "identity components",
    );
    let socket = directory.0.join("component.sock");
    let limits: Limits = default_component_limits();
    let bound = required(
        HumanComponentServer::new(Arc::new(components)).bind(ComponentServerConfig {
            socket_path: socket.clone(),
            allowed_uid: rustix::process::getuid().as_raw(),
            worker_count: 1,
            queue_capacity: 1,
            limits,
        }),
        "bind component server",
    );
    let client = required(UnixComponents::new(&socket, limits), "Unix components");
    let shutdown = bound.shutdown();
    let worker = thread::spawn(move || bound.run());
    let readiness = client.readiness(TRACE);
    shutdown.request();
    required(
        required(worker.join(), "component worker"),
        "component server",
    );

    let readiness = required(readiness, "read readiness over the human frame module");
    assert_eq!(
        readiness,
        Readiness {
            human_service: ComponentState::Ready,
            custody: ComponentState::Degraded,
            agent: ComponentState::Unavailable,
            core: ComponentState::Unavailable,
            paxeer: ComponentState::Unavailable,
        }
    );
    assert!(!readiness.ready());
    assert!(!socket.exists());
}
