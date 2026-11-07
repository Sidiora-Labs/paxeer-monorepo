use std::fs;
use std::io::{Read as _, Write as _};
use std::os::unix::fs::PermissionsExt as _;
use std::os::unix::net::UnixStream;
use std::path::PathBuf;
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::Duration;

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
use layerx_types::clock::Clock;
use layerx_types::rung::Rung;

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
        let clock = Arc::new(support::StepClock::new(1_700_000_000_000));
        let generation = clock
            .sample(Duration::from_secs(1))
            .expect("clock generation")
            .generation;
        assert_ne!(generation, [0; 16]);
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
        let bound = HumanComponentServer::new_with_clock(Arc::new(components), clock.clone())
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
            Arc::new(
                UnixComponents::new_with_clock(&socket, default_component_limits(), clock.clone())
                    .expect("components"),
            ),
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
        (Rung::Committed, "committed"),
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
        (2, Rung::Committed, "committed"),
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

#[path = "support/repair_22_33_44_bank.rs"]
mod recorded_bank;
#[path = "support/repair_22_33_44_auth.rs"]
mod route_auth;
#[path = "support/repair_22_33_44_clock.rs"]
mod route_clock;

#[test]
fn authenticated_routes_preserve_reopened_producer_rungs() {
    use layerx_agent_api::subscription::ReceiptReference;
    use layerx_human_service::journeys::JourneyEngine;
    use layerx_human_service::server::{schema::ApiSchema, PersistedReadServices};
    use serde_json::{json, Value};
    use sha2::{Digest as _, Sha256};

    for (rung, expected) in [
        (Rung::Submitted, "unverified"),
        (Rung::Receipted, "receipt-verified"),
        (Rung::Committed, "committed"),
    ] {
        let mut fixture = recorded_bank::StoreFixture::new("p44");
        fixture.submitted();
        let id = layerx_human_service::notify::JourneyId::new("jrn_repairreceipt").expect("id");
        let entry_id = if rung == Rung::Submitted {
            let mut scope = fixture
                .store()
                .principal(&support::principal("act_alice"))
                .expect("scope");
            let raw = recorded_bank::unhex(
                recorded_bank::transcript()["records"][0]["raw"]
                    .as_str()
                    .expect("recorded call"),
            );
            let hash = layerx_crypto::evm_transaction::transaction_hash(&raw);
            let entry_id = ActivityEntryId::new("act_recordedsubmission").expect("entry id");
            let activity = AgentActivity::new(
                entry_id.clone(),
                ActivityKind::Movement,
                None,
                100,
                PendingStatus::Processing,
                VerifiedStatus::Done,
            )
            .expect("activity");
            let event = EventDelivery::new(
                EventIdentity::new(hash),
                raw,
                Cursor {
                    block_height: 1,
                    tx_index: 0,
                    log_index: 0,
                },
                ReceiptReference::None,
            )
            .expect("submission event");
            Feed::record_agent_event(&mut scope, &activity, &event, 100).expect("production feed");
            entry_id
        } else {
            let node = recorded_bank::RpcPeer::receipts();
            let mut engine = fixture.load().expect("load").expect("journey");
            let proof = (rung == Rung::Committed).then(recorded_bank::commit_proof);
            let mut scope = fixture
                .store()
                .principal(&support::principal("act_alice"))
                .expect("scope");
            let status = engine
                .refresh_bank_receipt_with_commit(
                    &mut scope,
                    &node.endpoint,
                    &recorded_bank::keys(),
                    proof.as_ref(),
                    0,
                    101,
                )
                .expect("real receipt/consensus boundary");
            assert_eq!(
                engine
                    .verified_leg_evidence(0)
                    .expect("evidence")
                    .expect("verified")
                    .rung,
                rung
            );
            let progress = JourneyEngine::stream_events(&scope, &id)
                .expect("production outbox")
                .pop()
                .expect("progress");
            let entry = Feed::record_journey(
                &mut scope,
                ActivityKind::Movement,
                &status,
                &progress,
                None,
                None,
                102,
            )
            .expect("project actual journey progress");
            assert_eq!(entry.receipts()[0].rung(), rung);
            entry.entry_id().clone()
        };
        fixture.restart();
        let entry = Feed::entry(
            &fixture
                .store()
                .principal(&support::principal("act_alice"))
                .expect("reopen scope"),
            &entry_id,
        )
        .expect("reopen feed")
        .expect("persisted entry");
        assert_eq!(entry.rung(), rung);

        let clock_root = fixture.root.join("c");
        fs::create_dir(&clock_root).expect("clock directory");
        fs::set_permissions(&clock_root, fs::Permissions::from_mode(0o700)).expect("private clock");
        let authority =
            route_clock::ClockAuthority::start(&clock_root).expect("real runtime clock");
        let clock = authority.connect();
        let now = clock
            .sample(Duration::from_secs(1))
            .expect("time")
            .unix_seconds();
        let passkeys = Passkeys::new(route_auth::auth_config()).expect("passkeys");
        let grant = route_auth::register_session(
            &passkeys,
            &mut fixture
                .store()
                .principal(&support::principal("act_alice"))
                .expect("scope"),
            &mut route_auth::SoftwareAuthenticator::new(),
            now - 10,
        );
        // Reopen both the evidence and session owner before crossing the router.
        fixture.restart();
        let assets = layerx_network_gateway::decode_assets(
            &json!({"height":"9", "joined_limit":"16", "assets":[{
                "asset_id":recorded_bank::hex(&<[u8;32]>::from(Sha256::digest(b"uhpx"))),
                "kind":"bank", "symbol":"HPX", "decimals":"18"
            }]}),
        )
        .expect("real asset schema codec");
        let accounts = ProvisionedAccounts::new([ProvisionedAccount::new(
            "act_alice",
            "mara@example.com",
            "Mara",
            "Phone passkey",
        )
        .expect("account")])
        .expect("accounts");
        let reads = PersistedReadServices::new(
            IdentityServices::new(accounts),
            recorded_bank::keys(),
            assets,
        )
        .expect("production read composition");
        let components = PrivilegedHumanComponents::new(
            fixture.store.take().expect("exclusive durable owner"),
            passkeys,
            reads,
            AuthorizationGrantPolicy {
                lifetime_seconds: 30,
                maximum_outstanding: 32,
            },
            authority.connect(),
        )
        .expect("authenticated components");
        let socket = fixture.root.join("routes.sock");
        let bound = HumanComponentServer::new_with_clock(Arc::new(components), authority.connect())
            .bind(ComponentServerConfig {
                socket_path: socket.clone(),
                allowed_uid: rustix::process::getuid().as_raw(),
                worker_count: 1,
                queue_capacity: 1,
                limits: default_component_limits(),
            })
            .expect("real component transport");
        let shutdown = bound.shutdown();
        let worker = thread::spawn(move || bound.run().expect("serve component"));
        let router = Router::new(
            Arc::new(
                UnixComponents::new_with_clock(
                    &socket,
                    default_component_limits(),
                    authority.connect(),
                )
                .expect("client"),
            ),
            PrincipalLimits::new(100, 60, 100).expect("limits"),
            HttpConfig {
                maximum_header_bytes: 32_768,
                maximum_body_bytes: 1_048_576,
                allowed_origin: "https://paxportwallet.com".to_owned(),
                service_version: "projection-repair".to_owned(),
            },
            clock,
        )
        .expect("real HTTP router");
        let schema = ApiSchema::v1().expect("schema client");
        for (operation, path, expected_id) in [
            (
                "journey.get",
                format!("/v1/journeys/{}", id.as_str()),
                id.as_str(),
            ),
            (
                "activity.entry",
                format!("/v1/activity/{}", entry_id.as_str()),
                entry_id.as_str(),
            ),
        ] {
            for (token, status) in [
                (grant.access_token().expose(), 200),
                ("invalid-session", 401),
            ] {
                let raw = route_request(&router, &path, token);
                assert!(
                    raw.starts_with(&format!("HTTP/1.1 {status}")),
                    "{operation} at {rung:?}: {raw}"
                );
                if status != 200 {
                    continue;
                }
                let body: Value = serde_json::from_str(raw.split_once("\r\n\r\n").expect("body").1)
                    .expect("JSON");
                let result = &body["result"];
                schema
                    .encode_response(schema.operation(operation).expect("operation"), result)
                    .expect("schema consumer accepts production JSON");
                assert_eq!(
                    result[if operation == "journey.get" {
                        "journey_id"
                    } else {
                        "entry_id"
                    }],
                    expected_id
                );
                if rung == Rung::Submitted {
                    assert_eq!(result["evidence"].as_array().expect("evidence").len(), 0);
                    assert_ne!(result["state"], "done");
                } else {
                    assert!(!result["evidence"].as_array().expect("evidence").is_empty());
                    for evidence in result["evidence"].as_array().expect("references") {
                        assert_eq!(evidence["verification"], expected);
                    }
                    if operation == "activity.entry" {
                        assert_eq!(
                            result["state"],
                            if rung == Rung::Committed {
                                "done-finalised"
                            } else {
                                "done"
                            }
                        );
                    }
                }
                for forbidden in ["settlement-anchored", "paxeer-finalised"] {
                    assert!(!raw.contains(forbidden));
                }
            }
        }
        shutdown.request();
        worker.join().expect("worker");
        drop(router);
        drop(authority);
    }
}

fn route_request(router: &Router<UnixComponents>, path: &str, token: &str) -> String {
    thread::scope(|threads| {
        let (mut client, mut transport) = UnixStream::pair().expect("HTTP pair");
        client
            .set_read_timeout(Some(Duration::from_secs(5)))
            .expect("read bound");
        transport
            .set_read_timeout(Some(Duration::from_secs(5)))
            .expect("server bound");
        let worker = threads.spawn(move || {
            router
                .serve_one(&mut transport, "projection-route")
                .expect("route")
        });
        write!(client, "GET {path} HTTP/1.1\r\nHost: paxportwallet.com\r\nCookie: __Host-layerx_access={token}\r\nContent-Length: 0\r\n\r\n").expect("request");
        let mut response = String::new();
        client.read_to_string(&mut response).expect("response");
        worker.join().expect("HTTP worker");
        response
    })
}
