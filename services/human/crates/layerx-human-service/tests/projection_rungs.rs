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
    PrivilegedHumanServices, ProvisionedAccount, ProvisionedAccounts, Router, UnixComponents,
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

fn assert_server_readiness(router: &Router<UnixComponents>) {
    thread::scope(|threads| {
        let (mut client, mut transport) = UnixStream::pair().or_fail("HTTP stream pair");
        client
            .set_read_timeout(Some(Duration::from_secs(5)))
            .or_fail("read deadline");
        transport
            .set_read_timeout(Some(Duration::from_secs(5)))
            .or_fail("server deadline");
        let request = threads.spawn(move || {
            router
                .serve_one(&mut transport, "projection-test")
                .or_fail("serve readiness");
        });
        client
            .write_all(b"GET /readyz HTTP/1.1\r\nHost: wallet.example\r\n\r\n")
            .or_fail("request readiness");
        let mut raw = String::new();
        client.read_to_string(&mut raw).or_fail("read readiness");
        request.join().or_fail("HTTP worker");
        assert!(raw.starts_with("HTTP/1.1 503"), "{raw}");
        let (_, body) = raw.split_once("\r\n\r\n").or_fail("HTTP body");
        let body: serde_json::Value = serde_json::from_str(body).or_fail("readiness JSON");
        assert_eq!(body["result"]["components"]["human_service"], "ready");
    });
}

impl Server {
    fn start() -> Self {
        let root = support::directory("projection-rungs");
        fs::create_dir_all(&root).or_fail("create private server directory");
        fs::set_permissions(&root, fs::Permissions::from_mode(0o700))
            .or_fail("protect server directory");
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
            .or_fail("clock generation")
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
        .or_fail("passkeys");
        let accounts = ProvisionedAccounts::new([ProvisionedAccount::new(
            "act_projection",
            "projection@example.test",
            "Projection",
            "Primary passkey",
        )
        .or_fail("account")])
        .or_fail("accounts");
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
        .or_fail("privileged components");
        let socket = root.join("human.sock");
        let bound = HumanComponentServer::new_with_clock(Arc::new(components), clock.clone())
            .bind(ComponentServerConfig {
                socket_path: socket.clone(),
                allowed_uid: rustix::process::getuid().as_raw(),
                worker_count: 1,
                queue_capacity: 1,
                limits: default_component_limits(),
            })
            .or_fail("bind in-process component server");
        let shutdown = bound.shutdown();
        let worker = thread::spawn(move || {
            bound.run().or_fail("run component server");
        });
        let router = Router::new(
            Arc::new(
                UnixComponents::new_with_clock(&socket, default_component_limits(), clock.clone())
                    .or_fail("components"),
            ),
            PrincipalLimits::new(100, 60, 100).or_fail("principal limits"),
            HttpConfig {
                maximum_header_bytes: 32_768,
                maximum_body_bytes: 1_048_576,
                allowed_origin: "https://wallet.example".to_owned(),
                service_version: "projection-rungs-test".to_owned(),
            },
            clock,
        )
        .or_fail("HTTP router");
        assert_server_readiness(&router);
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
            worker.join().or_fail("component worker");
        }
        fs::remove_dir_all(&self.root).or_fail("remove private fixture");
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
        .or_fail("activity scope");
    for (index, rung, expected) in [
        (1, Rung::Receipted, "receipt-verified"),
        (2, Rung::Committed, "committed"),
    ] {
        let entry_id = ActivityEntryId::new(format!("act_projection{index}")).or_fail("entry id");
        let activity = AgentActivity::new(
            entry_id.clone(),
            ActivityKind::AgentAction,
            Some("did:layerx:projection".to_owned()),
            index,
            PendingStatus::Processing,
            VerifiedStatus::Done,
        )
        .or_fail("activity");
        let index_byte = u8::try_from(index).unwrap_or_else(|_| panic!("event index fits u8"));
        let delivery = EventDelivery::new(
            EventIdentity::new([index_byte; 32]),
            vec![index_byte],
            Cursor {
                block_height: index,
                tx_index: 0,
                log_index: 0,
            },
            ReceiptReference::Verified {
                receipt_ref: ReceiptRef::new(format!("receipt-{index}"))
                    .or_fail("receipt reference"),
                rung,
            },
        )
        .or_fail("delivery");
        Feed::record_agent_event(&mut scope, &activity, &delivery, index)
            .or_fail("record activity");
        let entry = Feed::entry(&scope, &entry_id)
            .or_fail("read activity")
            .or_fail("stored entry");
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
mod route_auth {
    include!("support/repair_22_33_44_auth.rs");
}
pub mod route_clock {
    include!("support/repair_22_33_44_clock.rs");
}

fn record_projection_rung(
    fixture: &mut recorded_bank::StoreFixture,
    rung: Rung,
) -> (layerx_human_service::notify::JourneyId, ActivityEntryId) {
    use layerx_agent_api::subscription::ReceiptReference;
    use layerx_human_service::journeys::JourneyEngine;
    fixture.submitted();
    let id = layerx_human_service::notify::JourneyId::new("jrn_repairreceipt").or_fail("id");
    let entry_id = if rung == Rung::Submitted {
        let mut scope = fixture
            .store()
            .principal(&support::principal("act_alice"))
            .or_fail("scope");
        let raw = recorded_bank::unhex(
            recorded_bank::transcript()["records"][0]["raw"]
                .as_str()
                .or_fail("recorded call"),
        );
        let hash = layerx_crypto::evm_transaction::transaction_hash(&raw);
        let entry_id = ActivityEntryId::new("act_recordedsubmission").or_fail("entry id");
        let activity = AgentActivity::new(
            entry_id.clone(),
            ActivityKind::Movement,
            None,
            100,
            PendingStatus::Processing,
            VerifiedStatus::Done,
        )
        .or_fail("activity");
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
        .or_fail("submission event");
        Feed::record_agent_event(&mut scope, &activity, &event, 100).or_fail("production feed");
        entry_id
    } else {
        let node = recorded_bank::RpcPeer::receipts();
        let mut engine = fixture.load().or_fail("load").or_fail("journey");
        let proof = (rung == Rung::Committed).then(recorded_bank::commit_proof);
        let mut scope = fixture
            .store()
            .principal(&support::principal("act_alice"))
            .or_fail("scope");
        let status = engine
            .refresh_bank_receipt_with_commit(
                &mut scope,
                &node.endpoint,
                &recorded_bank::keys(),
                proof.as_ref(),
                0,
                101,
            )
            .or_fail("real receipt/consensus boundary");
        assert_eq!(
            engine
                .verified_leg_evidence(0)
                .or_fail("evidence")
                .or_fail("verified")
                .rung,
            rung
        );
        let progress = JourneyEngine::stream_events(&scope, &id)
            .or_fail("production outbox")
            .pop()
            .or_fail("progress");
        let entry = Feed::record_journey(
            &mut scope,
            ActivityKind::Movement,
            &status,
            &progress,
            None,
            None,
            102,
        )
        .or_fail("project actual journey progress");
        assert_eq!(entry.receipts()[0].rung(), rung);
        entry.entry_id().clone()
    };
    (id, entry_id)
}

struct ProjectionRoutes {
    authority: route_clock::ClockAuthority,
    router: Router<UnixComponents>,
    grant: layerx_human_service::auth::SessionGrant,
    shutdown: ComponentShutdown,
    worker: JoinHandle<()>,
    schema: layerx_human_service::server::schema::ApiSchema,
}

fn start_projection_routes(fixture: &mut recorded_bank::StoreFixture) -> ProjectionRoutes {
    use layerx_human_service::server::{schema::ApiSchema, PersistedReadServices};
    use serde_json::json;
    use sha2::{Digest as _, Sha256};
    let clock_root = fixture.root.join("c");
    fs::create_dir(&clock_root).or_fail("clock directory");
    fs::set_permissions(&clock_root, fs::Permissions::from_mode(0o700)).or_fail("private clock");
    let authority = route_clock::ClockAuthority::start(&clock_root).or_fail("real runtime clock");
    let clock = authority.connect();
    let now = clock
        .sample(Duration::from_secs(1))
        .or_fail("time")
        .unix_seconds();
    let passkeys = Passkeys::new(route_auth::auth_config()).or_fail("passkeys");
    let grant = route_auth::register_session(
        &passkeys,
        &mut fixture
            .store()
            .principal(&support::principal("act_alice"))
            .or_fail("scope"),
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
    .or_fail("real asset schema codec");
    let accounts = ProvisionedAccounts::new([ProvisionedAccount::new(
        "act_alice",
        "mara@example.com",
        "Mara",
        "Phone passkey",
    )
    .or_fail("account")])
    .or_fail("accounts");
    let reads = PersistedReadServices::new(
        IdentityServices::new(accounts),
        recorded_bank::keys(),
        assets,
    )
    .or_fail("production read composition");
    let components = PrivilegedHumanComponents::new(
        fixture.store.take().or_fail("exclusive durable owner"),
        passkeys,
        ReceiptReadServices(reads),
        AuthorizationGrantPolicy {
            lifetime_seconds: 30,
            maximum_outstanding: 32,
        },
        authority.connect(),
    )
    .or_fail("authenticated components");
    let socket = fixture.root.join("routes.sock");
    let bound = HumanComponentServer::new_with_clock(Arc::new(components), authority.connect())
        .bind(ComponentServerConfig {
            socket_path: socket.clone(),
            allowed_uid: rustix::process::getuid().as_raw(),
            worker_count: 1,
            queue_capacity: 1,
            limits: default_component_limits(),
        })
        .or_fail("real component transport");
    let shutdown = bound.shutdown();
    let worker = thread::spawn(move || {
        bound.run().or_fail("serve component");
    });
    let router = Router::new(
        Arc::new(
            UnixComponents::new_with_clock(
                &socket,
                default_component_limits(),
                authority.connect(),
            )
            .or_fail("client"),
        ),
        PrincipalLimits::new(100, 60, 100).or_fail("limits"),
        HttpConfig {
            maximum_header_bytes: 32_768,
            maximum_body_bytes: 1_048_576,
            allowed_origin: "https://paxportwallet.com".to_owned(),
            service_version: "projection-repair".to_owned(),
        },
        clock,
    )
    .or_fail("real HTTP router");
    let schema = ApiSchema::v1().or_fail("schema client");
    ProjectionRoutes {
        authority,
        router,
        grant,
        shutdown,
        worker,
        schema,
    }
}

fn assert_portable_receipt(
    material: &serde_json::Value,
    original: &layerx_human_service::journeys::VerifiedLegEvidence,
) {
    use base64::{engine::general_purpose::STANDARD, Engine as _};
    use layerx_portable::PortableReceiptV2;
    use serde_json::Value;
    let bytes = STANDARD
        .decode(material["bytes_base64"].as_str().or_fail("portable bytes"))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "base64 receipt proof"));
    let portable_json: Value = serde_json::from_slice(&bytes).or_fail("portable JSON");
    assert_eq!(portable_json["verificationLevel"], "receipted");
    let portable = PortableReceiptV2::from_json(&bytes)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "portable receipt"));
    let keys = recorded_bank::keys()
        .iter()
        .map(|key| {
            layerx_proof::inclusion::SequencerAuthorization::new(
                key.key_id,
                key.public_key,
                key.first_height,
                key.last_height,
            )
        })
        .collect::<Vec<_>>();
    let verified = portable
        .verify_v2(&keys)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "independent pinned-key verification"));
    assert_eq!(verified.signed_receipt(), original.signed_receipt);
    assert_eq!(verified.signed_receipt(), original.canonical_receipt);
    assert_eq!(verified.activity_id(), original.activity_id);
    assert_eq!(verified.inclusion_height(), original.inclusion_height);
    assert_eq!(verified.receipt_key(), &keys[0]);
    assert_eq!(
        portable
            .to_json()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "portable encoding")),
        bytes
    );
}

fn assert_evidence_material(
    router: &Router<UnixComponents>,
    grant: &layerx_human_service::auth::SessionGrant,
    schema: &layerx_human_service::server::schema::ApiSchema,
    evidence: &serde_json::Value,
    rung: Rung,
    original_evidence: Option<&layerx_human_service::journeys::VerifiedLegEvidence>,
) -> String {
    use serde_json::Value;
    let id = evidence["evidence_id"]
        .as_str()
        .or_fail("receipt evidence id");
    let path = format!("/v1/evidence/{id}");
    let raw = route_request(router, &path, grant.access_token().expose());
    assert!(raw.starts_with("HTTP/1.1 200"), "{rung:?}: {raw}");
    let body: Value = serde_json::from_str(raw.split_once("\r\n\r\n").or_fail("evidence body").1)
        .or_fail("evidence JSON");
    let material = &body["result"];
    schema
        .encode_response(
            schema
                .operation("evidence.get")
                .or_fail("evidence operation"),
            material,
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "schema-valid production evidence"));
    assert_eq!(material["evidence_id"], evidence["evidence_id"]);
    assert_eq!(material["class"], evidence["class"]);
    assert_eq!(material["class"], "layerx-receipt");
    assert_eq!(material["verification"], evidence["verification"]);
    assert_eq!(material["rung"], rung.name());
    assert_eq!(
        material["content_type"],
        "application/vnd.layerx.receipt-proof-v2+json"
    );
    let original = original_evidence.or_fail("producer evidence");
    assert_eq!(
        id,
        format!(
            "evd_{}",
            recorded_bank::hex(&original.receipt_digest).trim_start_matches("0x")
        )
    );
    assert_eq!(material["inclusion_height"], original.inclusion_height);
    assert_portable_receipt(material, original);
    let unauthenticated = route_request(router, &path, "invalid-session");
    assert!(
        unauthenticated.starts_with("HTTP/1.1 401"),
        "{unauthenticated}"
    );
    assert!(!unauthenticated.contains("bytes_base64"));
    id.to_owned()
}

#[derive(Clone, Copy)]
struct ProjectionRouteChecks<'a> {
    router: &'a Router<UnixComponents>,
    grant: &'a layerx_human_service::auth::SessionGrant,
    schema: &'a layerx_human_service::server::schema::ApiSchema,
    journey_id: &'a layerx_human_service::notify::JourneyId,
    entry_id: &'a ActivityEntryId,
    rung: Rung,
    expected: &'a str,
    original_evidence: Option<&'a layerx_human_service::journeys::VerifiedLegEvidence>,
}

fn assert_projection_routes(checks: ProjectionRouteChecks<'_>) -> Option<String> {
    use serde_json::Value;
    let ProjectionRouteChecks {
        router,
        grant,
        schema,
        journey_id,
        entry_id,
        rung,
        expected,
        original_evidence,
    } = checks;
    let id = journey_id;
    let mut receipt_id = None;
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
            let raw = route_request(router, &path, token);
            assert!(
                raw.starts_with(&format!("HTTP/1.1 {status}")),
                "{operation} at {rung:?}: {raw}"
            );
            if status != 200 {
                continue;
            }
            let body: Value =
                serde_json::from_str(raw.split_once("\r\n\r\n").or_fail("body").1).or_fail("JSON");
            let result = &body["result"];
            schema
                .encode_response(schema.operation(operation).or_fail("operation"), result)
                .unwrap_or_else(|error| {
                    panic!("{}: {error:?}", "schema consumer accepts production JSON")
                });
            assert_eq!(
                result[if operation == "journey.get" {
                    "journey_id"
                } else {
                    "entry_id"
                }],
                expected_id
            );
            if rung == Rung::Submitted {
                assert_eq!(result["evidence"].as_array().or_fail("evidence").len(), 0);
                assert_ne!(result["state"], "done");
            } else {
                assert!(!result["evidence"].as_array().or_fail("evidence").is_empty());
                for evidence in result["evidence"].as_array().or_fail("references") {
                    assert_eq!(evidence["verification"], expected);
                    if operation == "journey.get" {
                        receipt_id = Some(assert_evidence_material(
                            router,
                            grant,
                            schema,
                            evidence,
                            rung,
                            original_evidence,
                        ));
                    }
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
    receipt_id
}

struct EvidenceCorruption<'a, 'operation> {
    fixture: &'a mut recorded_bank::StoreFixture,
    key_source: &'a RecordedReceiptKeys,
    original_evidence: &'a Option<layerx_human_service::journeys::VerifiedLegEvidence>,
    row_key: &'a layerx_human_service::store::RowKey,
    original_record: &'a serde_json::Value,
    request: &'a layerx_human_service::server::ScopedRequest<'operation>,
    rung: Rung,
}

fn assert_evidence_corruptions(context: EvidenceCorruption<'_, '_>) {
    use layerx_human_service::server::ProductionComponents;
    use layerx_human_service::store::Table;
    use serde_json::json;
    use sha2::{Digest as _, Sha256};
    let EvidenceCorruption {
        fixture,
        key_source,
        original_evidence,
        row_key,
        original_record,
        request,
        rung,
    } = context;
    for corruption in [
        "signature",
        "digest",
        "receipt-key",
        "key-window",
        "unsupported-rung",
        "missing-commit",
        "altered-commit",
    ] {
        if rung != Rung::Committed && matches!(corruption, "missing-commit" | "altered-commit") {
            continue;
        }
        let mut record = (*original_record).clone();
        let leg = &mut record["legs"][0];
        match corruption {
            "signature" => {
                let mut receipt: Vec<u8> =
                    serde_json::from_value(leg["receipt"].clone()).or_fail("signed bytes");
                *receipt.last_mut().or_fail("signature byte") ^= 1;
                leg["receipt"] = json!(receipt);
                leg["rung_evidence"]["signed_receipt"] = json!(receipt);
                leg["receipt_digest"] = json!(<[u8; 32]>::from(Sha256::digest(&receipt)));
            }
            "digest" => {
                let digest = leg["receipt_digest"][0].as_u64().or_fail("digest byte");
                leg["receipt_digest"][0] = json!(digest ^ 1);
            }
            "receipt-key" => {
                key_source.keys.lock().or_fail("keys")["keys"][0]["publicKey"] =
                    json!(recorded_bank::hex(
                        &ed25519_dalek::SigningKey::from_bytes(&[0x72; 32])
                            .verifying_key()
                            .to_bytes(),
                    ));
            }
            "key-window" => {
                let height = original_evidence
                    .as_ref()
                    .or_fail("evidence")
                    .inclusion_height;
                let mut keys = key_source.keys.lock().or_fail("keys");
                keys["keys"][0]["firstHeight"] = json!(height + 1);
                keys["keys"][0]["lastHeight"] = json!(height + 2);
            }
            "unsupported-rung" => leg["rung_evidence"]["rung"] = json!(3),
            "missing-commit" => {
                leg.as_object_mut().or_fail("leg").remove("bank_commit");
            }
            "altered-commit" => {
                let byte = leg["bank_commit"]["batch_transaction"][0]
                    .as_u64()
                    .or_fail("retained batch byte");
                leg["bank_commit"]["batch_transaction"][0] = json!(byte ^ 1);
            }
            _ => unreachable!(),
        }
        fixture
            .store()
            .principal(&support::principal("act_alice"))
            .or_fail("scope")
            .put(
                Table::Journeys,
                (*row_key).clone(),
                102,
                serde_json::to_vec(&record).or_fail("record bytes"),
            )
            .or_fail("persist invalid evidence");
        fixture.restart();
        let scope = fixture
            .store()
            .principal(&support::principal("act_alice"))
            .or_fail("reopened scope");
        let error = ProductionComponents::evidence_get(&scope, request)
            .err()
            .unwrap_or_else(|| panic!("invalid reopened receipt must be refused"));
        assert_eq!(error.status, 503, "{rung:?} {corruption}: {error:?}");
        *key_source.keys.lock().or_fail("keys") = RecordedReceiptKeys::document();
    }
}

#[test]
fn authenticated_routes_preserve_reopened_producer_rungs() {
    use layerx_human_service::server::ScopedRequest;
    use layerx_human_service::store::Table;
    use serde_json::{json, Value};

    let key_source = RecordedReceiptKeys::start();
    let _environment = ReceiptKeyEnvironment::install(&key_source.url);
    for (rung, expected) in [
        (Rung::Submitted, "unverified"),
        (Rung::Receipted, "receipt-verified"),
        (Rung::Committed, "committed"),
    ] {
        let mut fixture = recorded_bank::StoreFixture::new("p44");
        let (id, entry_id) = record_projection_rung(&mut fixture, rung);
        fixture.restart();
        let entry = Feed::entry(
            &fixture
                .store()
                .principal(&support::principal("act_alice"))
                .or_fail("reopen scope"),
            &entry_id,
        )
        .or_fail("reopen feed")
        .or_fail("persisted entry");
        assert_eq!(entry.rung(), rung);
        let original_evidence = fixture
            .load()
            .or_fail("reopened journey")
            .or_fail("journey")
            .verified_leg_evidence(0)
            .or_fail("reopened receipt evidence");

        let ProjectionRoutes {
            authority,
            router,
            grant,
            shutdown,
            worker,
            schema,
        } = start_projection_routes(&mut fixture);
        let receipt_id = assert_projection_routes(ProjectionRouteChecks {
            router: &router,
            grant: &grant,
            schema: &schema,
            journey_id: &id,
            entry_id: &entry_id,
            rung,
            expected,
            original_evidence: original_evidence.as_ref(),
        });
        shutdown.request();
        worker.join().or_fail("worker");
        drop(router);
        if let Some(receipt_id) = receipt_id {
            fixture.restart();
            let (row_key, original_record) = {
                let scope = fixture
                    .store()
                    .principal(&support::principal("act_alice"))
                    .or_fail("scope");
                let key = scope
                    .keys(Table::Journeys)
                    .into_iter()
                    .find(|key| key.as_str().starts_with("journey-"))
                    .or_fail("journey row");
                let record: Value =
                    serde_json::from_slice(scope.get(Table::Journeys, &key).or_fail("row").bytes())
                        .or_fail("producer record");
                (key, record)
            };
            let request = ScopedRequest {
                operation: schema
                    .operation("evidence.get")
                    .or_fail("evidence operation"),
                principal: None,
                path_parameters: [("evidence_id".to_owned(), receipt_id)]
                    .into_iter()
                    .collect(),
                body: json!({}),
                idempotency_key: None,
                trace: "reopened-refusal".to_owned(),
            };
            assert_evidence_corruptions(EvidenceCorruption {
                fixture: &mut fixture,
                key_source: &key_source,
                original_evidence: &original_evidence,
                row_key: &row_key,
                original_record: &original_record,
                request: &request,
                rung,
            });
        }
        drop(authority);
    }
    assert!(
        key_source
            .requests
            .load(std::sync::atomic::Ordering::Acquire)
            >= 4
    );
}

fn route_request(router: &Router<UnixComponents>, path: &str, token: &str) -> String {
    thread::scope(|threads| {
        let (mut client, mut transport) = UnixStream::pair().or_fail("HTTP pair");
        client
            .set_read_timeout(Some(Duration::from_secs(5)))
            .or_fail("read bound");
        transport
            .set_read_timeout(Some(Duration::from_secs(5)))
            .or_fail("server bound");
        let worker = threads.spawn(move || {
            router
                .serve_one(&mut transport, "projection-route")
                .or_fail("route");
        });
        write!(client, "GET {path} HTTP/1.1\r\nHost: paxportwallet.com\r\nCookie: __Host-layerx_access={token}\r\nContent-Length: 0\r\n\r\n").or_fail("request");
        let mut response = String::new();
        client.read_to_string(&mut response).or_fail("response");
        worker.join().or_fail("HTTP worker");
        response
    })
}

struct ReceiptReadServices(layerx_human_service::server::PersistedReadServices);

impl PrivilegedHumanServices for ReceiptReadServices {
    fn execute_public(
        &mut self,
        store: &mut PrincipalStore,
        passkeys: &Passkeys,
        request: layerx_human_service::server::ComponentOperationRequest<'_>,
        now: u64,
    ) -> Result<
        layerx_human_service::server::BackendResponse,
        layerx_human_service::server::ApiFailure,
    > {
        self.0.execute_public(store, passkeys, request, now)
    }

    fn execute_authorized(
        &mut self,
        scope: &mut layerx_human_service::store::PrincipalScope<'_>,
        passkeys: &Passkeys,
        session: layerx_human_service::server::AuthorizedSession<'_>,
        request: layerx_human_service::server::ComponentOperationRequest<'_>,
        now: u64,
    ) -> Result<
        layerx_human_service::server::BackendResponse,
        layerx_human_service::server::ApiFailure,
    > {
        if request.operation.name == "evidence.get" {
            layerx_human_service::server::ProductionComponents::evidence_get(
                scope,
                &layerx_human_service::server::ScopedRequest {
                    operation: request.operation,
                    principal: None,
                    path_parameters: request.path_parameters,
                    body: request.body,
                    idempotency_key: request.idempotency_key,
                    trace: request.trace,
                },
            )
        } else {
            self.0
                .execute_authorized(scope, passkeys, session, request, now)
        }
    }

    fn readiness(
        &mut self,
        store: &mut PrincipalStore,
        now: u64,
    ) -> Result<layerx_human_service::server::Readiness, layerx_human_service::server::ApiFailure>
    {
        self.0.readiness(store, now)
    }
}

struct ReceiptKeyEnvironment(Vec<(&'static str, Option<std::ffi::OsString>)>);

impl ReceiptKeyEnvironment {
    fn install(url: &str) -> Self {
        let mut previous = Vec::new();
        for (name, value) in [
            ("LAYERX_AGENT_NODE_RPC_URL", url),
            ("LAYERX_AGENT_NODE_CHAIN_ID", "125"),
        ] {
            previous.push((name, std::env::var_os(name)));
            std::env::set_var(name, value);
        }
        Self(previous)
    }
}

impl Drop for ReceiptKeyEnvironment {
    fn drop(&mut self) {
        for (name, value) in self.0.drain(..) {
            match value {
                Some(value) => std::env::set_var(name, value),
                None => std::env::remove_var(name),
            }
        }
    }
}

struct RecordedReceiptKeys {
    url: String,
    keys: Arc<std::sync::Mutex<serde_json::Value>>,
    requests: Arc<std::sync::atomic::AtomicUsize>,
    stop: Arc<std::sync::atomic::AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl RecordedReceiptKeys {
    fn document() -> serde_json::Value {
        serde_json::json!({"keys": [recorded_bank::transcript()["key"]]})
    }

    fn start() -> Self {
        use serde_json::{json, Value};
        use std::io::{BufRead as _, BufReader};
        use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};

        let listener = std::net::TcpListener::bind("127.0.0.1:0").or_fail("recorded keys RPC");
        listener.set_nonblocking(true).or_fail("nonblocking RPC");
        let url = format!("http://{}/", listener.local_addr().or_fail("RPC address"));
        let keys = Arc::new(std::sync::Mutex::new(Self::document()));
        let stop = Arc::new(AtomicBool::new(false));
        let requests = Arc::new(AtomicUsize::new(0));
        let source = keys.clone();
        let shutdown = stop.clone();
        let served = requests.clone();
        let worker = thread::spawn(move || {
            while !shutdown.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("recorded RPC accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .or_fail("RPC deadline");
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(reader.read_line(&mut line).or_fail("RPC header"), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(value.trim().parse::<usize>().or_fail("RPC length"));
                    }
                }
                let length = length.or_fail("RPC content length");
                assert!(length <= 65_536);
                let mut body = vec![0; length];
                reader.read_exact(&mut body).or_fail("RPC request");
                let request: Value = serde_json::from_slice(&body).or_fail("RPC JSON");
                assert_eq!(request["params"], json!([]));
                let result = match request["method"].as_str().or_fail("RPC method") {
                    "eth_chainId" => json!("0x7d"),
                    "px_getReceiptKeys" => {
                        served.fetch_add(1, Ordering::AcqRel);
                        source.lock().or_fail("recorded keys").clone()
                    }
                    method => panic!("unrecorded key-source method {method}"),
                };
                let body =
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
                write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).or_fail("recorded RPC response");
                reader.get_mut().flush().or_fail("flush RPC");
            }
        });
        Self {
            url,
            keys,
            requests,
            stop,
            worker: Some(worker),
        }
    }
}

impl Drop for RecordedReceiptKeys {
    fn drop(&mut self) {
        self.stop.store(true, std::sync::atomic::Ordering::Release);
        self.worker
            .take()
            .or_fail("RPC worker")
            .join()
            .or_fail("recorded RPC worker");
    }
}

use self::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
