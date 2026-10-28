use std::fs;
use std::io::{Read as _, Write as _};
use std::os::unix::fs::PermissionsExt as _;
use std::os::unix::net::UnixStream;
use std::path::PathBuf;
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use layerx_agent_api::subscription::{Cursor, EventDelivery, EventIdentity, ReceiptReference};
use layerx_agent_api::track::ReceiptRef;
use layerx_human_service::activity::{
    ActivityKind, AgentActivity, Feed, PendingStatus, VerifiedStatus,
};
use layerx_human_service::auth::{AuthConfig, Passkeys, RateLimit};
use layerx_human_service::notify::ActivityEntryId;
use layerx_human_service::server::{
    default_component_limits, AuthorizationGrantPolicy, ComponentServerConfig, ComponentShutdown,
    HttpConfig, HumanComponentServer, IdentityServices, PrincipalLimits, PrivilegedHumanComponents,
    ProvisionedAccount, ProvisionedAccounts, Router, UnixComponents,
};
use layerx_human_service::store::PrincipalStore;
use layerx_human_test_support as support;
use layerx_types::clock::{Clock, ClockError, ClockReading};
use layerx_types::rung::Rung;

struct SystemClock {
    origin: Instant,
    generation: [u8; 16],
}

impl Clock for SystemClock {
    fn sample(&self, budget: Duration) -> Result<ClockReading, ClockError> {
        if budget.is_zero() {
            return Err(ClockError::Unavailable);
        }
        Ok(ClockReading {
            generation: self.generation,
            unix_milliseconds: SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map_err(|_| ClockError::Regression)?
                .as_millis()
                .try_into()
                .map_err(|_| ClockError::Overflow)?,
            monotonic_nanoseconds: self
                .origin
                .elapsed()
                .as_nanos()
                .try_into()
                .map_err(|_| ClockError::Overflow)?,
        })
    }
}

struct Server {
    root: PathBuf,
    shutdown: ComponentShutdown,
    worker: Option<JoinHandle<()>>,
    _router: Router<UnixComponents>,
    activity: PrincipalStore,
}

impl Server {
    fn start() -> Self {
        let root = support::directory("projection-rungs");
        fs::create_dir_all(&root).expect("create private server directory");
        fs::set_permissions(&root, fs::Permissions::from_mode(0o700))
            .expect("protect server directory");
        let tenancy = support::tenancy(&[("act_projection", "tenant-projection")]);
        let (store, _) = support::install_and_open(
            &root.join("server"),
            &tenancy,
            support::retention_uniform(86_400),
        );
        let (activity, _) = support::install_and_open(
            &root.join("activity"),
            &tenancy,
            support::retention_uniform(86_400),
        );
        let mut generation = [0; 16];
        getrandom::fill(&mut generation).expect("clock generation");
        assert_ne!(generation, [0; 16]);
        let clock = Arc::new(SystemClock {
            origin: Instant::now(),
            generation,
        });
        let passkeys = Passkeys::new(AuthConfig {
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
        })
        .expect("passkeys");
        let accounts = ProvisionedAccounts::new([ProvisionedAccount::new(
            "act_projection",
            "projection@example.test",
            "Projection",
            "Primary passkey",
        )
        .expect("account")])
        .expect("accounts");
        let components = PrivilegedHumanComponents::new(
            store,
            passkeys,
            IdentityServices::new(accounts),
            AuthorizationGrantPolicy {
                lifetime_seconds: 30,
                maximum_outstanding: 32,
            },
            clock.clone(),
        )
        .expect("privileged components");
        let socket = root.join("human.sock");
        let bound = HumanComponentServer::new(Arc::new(components))
            .bind(ComponentServerConfig {
                socket_path: socket.clone(),
                allowed_uid: rustix::process::getuid().as_raw(),
                worker_count: 1,
                queue_capacity: 1,
                limits: default_component_limits(),
            })
            .expect("bind in-process component server");
        let shutdown = bound.shutdown();
        let worker = thread::spawn(move || bound.run().expect("run component server"));
        let router = Router::new(
            Arc::new(UnixComponents::new(&socket, default_component_limits()).expect("components")),
            PrincipalLimits::new(100, 60, 100).expect("principal limits"),
            HttpConfig {
                maximum_header_bytes: 32_768,
                maximum_body_bytes: 1_048_576,
                allowed_origin: "https://wallet.example".to_owned(),
                service_version: "projection-rungs-test".to_owned(),
            },
            clock,
        )
        .expect("HTTP router");
        thread::scope(|threads| {
            let (mut client, mut transport) = UnixStream::pair().expect("HTTP stream pair");
            client
                .set_read_timeout(Some(Duration::from_secs(5)))
                .expect("read deadline");
            transport
                .set_read_timeout(Some(Duration::from_secs(5)))
                .expect("server deadline");
            let router = &router;
            let request = threads.spawn(move || {
                router
                    .serve_one(&mut transport, "projection-test")
                    .expect("serve readiness");
            });
            client
                .write_all(b"GET /readyz HTTP/1.1\r\nHost: wallet.example\r\n\r\n")
                .expect("request readiness");
            let mut raw = String::new();
            client.read_to_string(&mut raw).expect("read readiness");
            request.join().expect("HTTP worker");
            assert!(raw.starts_with("HTTP/1.1 503"), "{raw}");
            let (_, body) = raw.split_once("\r\n\r\n").expect("HTTP body");
            let body: serde_json::Value = serde_json::from_str(body).expect("readiness JSON");
            assert_eq!(body["result"]["components"]["human_service"], "ready");
        });
        Self {
            root,
            shutdown,
            worker: Some(worker),
            _router: router,
            activity,
        }
    }
}

impl Drop for Server {
    fn drop(&mut self) {
        self.shutdown.request();
        if let Some(worker) = self.worker.take() {
            worker.join().expect("component worker");
        }
        fs::remove_dir_all(&self.root).expect("remove private fixture");
    }
}

#[test]
fn projected_journey_names_rung_vocabulary() {
    let _server = Server::start();
    for (rung, expected) in [
        (Rung::Submitted, "unverified"),
        (Rung::Receipted, "receipt-verified"),
        (Rung::Committed, "checkpoint-finalised"),
    ] {
        assert_eq!(
            HumanComponentServer::verification_level_name(rung),
            expected
        );
    }
}

#[test]
fn projection_never_emits_settlement_anchored() {
    let mut server = Server::start();
    let principal = support::principal("act_projection");
    let mut scope = server
        .activity
        .principal(&principal)
        .expect("activity scope");
    for (index, rung, expected) in [
        (1, Rung::Receipted, "receipt-verified"),
        (2, Rung::Committed, "checkpoint-finalised"),
    ] {
        let entry_id = ActivityEntryId::new(format!("act_projection{index}")).expect("entry id");
        let activity = AgentActivity::new(
            entry_id.clone(),
            ActivityKind::AgentAction,
            Some("did:layerx:projection".to_owned()),
            index,
            PendingStatus::Processing,
            VerifiedStatus::Done,
        )
        .expect("activity");
        let delivery = EventDelivery::new(
            EventIdentity::new([index as u8; 32]),
            vec![index as u8],
            Cursor {
                block_height: index,
                tx_index: 0,
                log_index: 0,
            },
            ReceiptReference::Verified {
                receipt_ref: ReceiptRef::new(format!("receipt-{index}"))
                    .expect("receipt reference"),
                rung,
            },
        )
        .expect("delivery");
        Feed::record_agent_event(&mut scope, &activity, &delivery, index).expect("record activity");
        let entry = Feed::entry(&scope, &entry_id)
            .expect("read activity")
            .expect("stored entry");
        assert_eq!(entry.receipts().len(), 1);
        assert_eq!(entry.receipts()[0].rung(), rung);
        let projected = HumanComponentServer::verification_level_name(entry.receipts()[0].rung());
        assert_eq!(projected, expected);
        assert_ne!(projected, "settlement-anchored");
        assert_ne!(projected, "paxeer-finalised");
    }
    for rung in [Rung::Submitted, Rung::Receipted, Rung::Committed] {
        let projected = HumanComponentServer::verification_level_name(rung);
        assert_ne!(projected, "settlement-anchored");
        assert_ne!(projected, "paxeer-finalised");
    }
}
