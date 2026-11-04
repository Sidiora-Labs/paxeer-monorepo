//! Repair record 22.33.1526. The bounded gate ledger records the qualified revision.
//! admission-controller-unwired -> admission_controller_routes_real_overload_with_reserved_capacity
//! request-deadline-tracker-unwired -> deadline_disconnect_and_reopen_retain_real_unknown_submission
//! shedding-cancellation-hooks-unwired -> shedding_cancels_one_authenticated_client_and_preserves_resolvers
//! quota-unbounded-observations -> quota_churn_and_shed_retention_are_bounded_and_indexed
//! rate-counter-recovery -> independent_daemons_and_reopen_share_atomic_durable_counters
//! rate-window-regression -> interleaved_and_concurrent_delayed_windows_preserve_ceiling
//! Wiring request (Domain 9): services/agent/crates/layerx-agentd/src/agent_rpc_dispatch.rs must own
//! TenantBoundary after OperationPermit authorization and feed transport EOF and request deadlines.
//! Wiring request (Domain 9): services/agent/crates/layerx-agentd/src/human_runtime.rs must route node
//! calls through TenantBoundary and connect its resolver outcomes to SessionControl budget settlement.
//! Wiring request (Domain 9): services/agent/crates/layerx-agentd/src/main.rs and services/agent/crates/
//! layerx-agentd-host/src/lib.rs must configure one shared durable counter root for all tenant instances.
//! Baseline: no baseline gate run; the test target did not exist at the starting revision, and the
//! task's explicit implementation-first order prohibits running verification before implementation.

use std::collections::BTreeSet;
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::net::UnixStream;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Barrier};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::limits::admission::{
    AdmissionConfig, AdmissionError, BackpressureSource, BoundaryAdmission, Priority, QueueBound,
};
use layerx_agentd::limits::deadline::{DisconnectOutcome, RequestDeadline, TrackedWork, WorkOwner};
use layerx_agentd::limits::lifecycle::{
    client_id, CallError, LifecycleError, NodeResponse, NodeWork, TenantBoundary,
};
use layerx_agentd::limits::quota::{
    ClientActivity, ObservationBounds, Quota, QuotaError, Resource, SheddingPolicy, TenantQuota,
};
use layerx_agentd::limits::{
    CounterLedger, LimitConfig, LimitId, LimitScope, RateLimiter, RateRequest, Refusal,
};
use layerx_agentd::outbox::{resolve_unknown, ResolutionObservation, SubmissionState};
use layerx_agentd::session::{open, OpenRequest, SessionId, SessionRegistry};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_agentd::tenant::{
    resolve, Operation, RequestContext, ResolvedPrincipal, Surface, TenantObservability,
};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
use layerx_types::ids::Did;
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};

static NEXT: AtomicU64 = AtomicU64::new(1);
fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}
struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        Self(std::env::temp_dir().join(format!(
            "repair-22-33-1526-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        )))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}
fn tenant(name: &str) -> TenantId {
    must(TenantId::new(name))
}

// The committed observation used by tenant_resolve.rs is a peer fixture; identity,
// session persistence, token minting and tenant resolution below are the real producers.
struct RecordedIdentity;
impl IdentityResolver for RecordedIdentity {
    fn resolve(&mut self, _: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(CoreIdentity {
            canonical_bytes: b"core-identity".to_vec(),
            head_sequence: 10,
            revocation_sequence: 1,
            verification_level: VerificationLevel::STATE_PROVEN,
            frozen: false,
            authorities: vec![ProtocolAuthority::SessionKey([4; 32])],
        }))
    }
}
fn principal(store: &mut Store, name: &str, session: u8) -> ResolvedPrincipal {
    let tenant = tenant(name);
    let agent = must(Did::new(format!("did:layerx:{name}:agent").as_bytes()));
    let identity = must(register(
        store,
        tenant.clone(),
        agent.clone(),
        &mut RecordedIdentity,
    ));
    let mut sessions = SessionRegistry::default();
    let token = must(open(
        store,
        &mut sessions,
        &identity,
        OpenRequest {
            session_id: SessionId([session; 32]),
            token_id: [session.wrapping_add(1); 32],
            tenant,
            agent,
            authority: ProtocolAuthority::SessionKey([4; 32]),
            permitted_activity_types: BTreeSet::from([7]),
            scopes: BTreeSet::from(["read".to_owned(), "write".to_owned()]),
            expiry_sequence: 100,
            expiry_seconds: None,
            opening_client: "tenant-resolve-suite".to_owned(),
            policy_version: "policy-v1".to_owned(),
        },
        10,
    ));
    must(resolve(
        &token,
        &sessions,
        &RequestContext {
            surface: Surface::RustSdk,
            operation: Operation::ReadBalance,
            core_sequence: 20,
            supplied_header_tenant: None,
            supplied_body_tenant: None,
            target_owner: None,
        },
        &mut TenantObservability::default(),
    ))
}
fn policy(maximum: u64) -> SheddingPolicy {
    SheddingPolicy {
        window_ms: 100,
        maximum_requests: maximum,
        maximum_retries: maximum,
        maximum_identical_operations: maximum,
        shed_for_ms: 200,
    }
}
fn quota(maximum: u64) -> Quota {
    must(Quota::new(
        ["alpha", "beta"].map(|name| {
            must(TenantQuota::new(
                tenant(name),
                Resource::ALL.map(|resource| (resource, 100)),
            ))
        }),
        policy(maximum),
    ))
}
fn configs(limit: u64) -> Vec<LimitConfig> {
    ["alpha", "beta"]
        .map(|name| LimitConfig {
            id: must(LimitId::new(format!("{name}-ceiling"))),
            scope: LimitScope::Tenant {
                tenant: name.to_owned(),
            },
            limit,
            window_ms: 1000,
        })
        .to_vec()
}
fn admission() -> BoundaryAdmission {
    must(BoundaryAdmission::with_tenant_bound(
        must(AdmissionConfig::new(
            3,
            1024,
            Priority::ALL.map(|priority| {
                (
                    priority,
                    QueueBound {
                        requests: 16,
                        bytes: 16 * 1024,
                    },
                )
            }),
        )),
        QueueBound {
            requests: 4,
            bytes: 4096,
        },
        2,
    ))
}
fn boundary(
    store: &mut Store,
    counter_root: &Path,
    limit: u64,
    shed_maximum: u64,
    now: u64,
) -> TenantBoundary {
    must(TenantBoundary::open(
        store,
        admission(),
        quota(shed_maximum),
        must(RateLimiter::new(
            configs(limit),
            must(CounterLedger::open(counter_root)),
        )),
        now,
    ))
}
fn deadline() -> RequestDeadline {
    must(RequestDeadline::new(1000, 5000))
}
fn head() -> NodeWork {
    NodeWork::Head {
        priority: Priority::BulkRead,
    }
}
fn raw(nonce: u64) -> Vec<u8> {
    must(
        Eip1559Call {
            chain_id: 125,
            nonce,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
            to: [0x22; 20],
            data: vec![1],
        }
        .sign(&[0xa5; 32]),
    )
}

enum Exchange {
    Reply(&'static str, Value),
    DropAfterRequest(&'static str),
}
fn receive(stream: TcpStream) -> (TcpStream, Value) {
    must(stream.set_read_timeout(Some(Duration::from_secs(3))));
    must(stream.set_write_timeout(Some(Duration::from_secs(3))));
    let mut reader = BufReader::new(stream);
    let mut length = None;
    loop {
        let mut line = String::new();
        assert_ne!(must(reader.read_line(&mut line)), 0);
        if line == "\r\n" {
            break;
        }
        if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
            length = Some(must(value.trim().parse::<usize>()));
        }
    }
    let length = length.unwrap_or_else(|| panic!("content length required"));
    assert!(length <= 64 * 1024);
    let mut body = vec![0; length];
    must(reader.read_exact(&mut body));
    (reader.into_inner(), must(serde_json::from_slice(&body)))
}
fn serve(exchanges: Vec<Exchange>) -> (SequencerEndpoint, JoinHandle<Vec<Value>>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    must(listener.set_nonblocking(true));
    let endpoint = must(SequencerEndpoint::new(
        &format!("http://{}/", must(listener.local_addr())),
        &[],
    ));
    let handle = thread::spawn(move || {
        let mut requests = Vec::new();
        for exchange in exchanges {
            let expires = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < expires, "missing node request");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            let (mut stream, request) = receive(stream);
            let method = match &exchange {
                Exchange::Reply(method, _) | Exchange::DropAfterRequest(method) => method,
            };
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], *method);
            if let Exchange::Reply(_, value) = exchange {
                let response =
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":value}).to_string();
                must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()));
                must(stream.flush());
            }
            requests.push(request);
        }
        requests
    });
    (endpoint, handle)
}
fn eof() -> UnixStream {
    let (server, client) = must(UnixStream::pair());
    drop(client);
    server
}

#[test]
fn admission_controller_routes_real_overload_with_reserved_capacity() {
    let directory = Directory::new();
    let mut store = must(Store::open(directory.0.join("store")));
    let alpha = principal(&mut store, "alpha", 1);
    let beta = principal(&mut store, "beta", 2);
    let mut runtime = boundary(&mut store, &directory.0.join("counters"), 100, 100, 1000);
    let signed = raw(1);
    let hash = transaction_hash(&signed);
    let (node, server) = serve(vec![
        Exchange::Reply(
            "eth_sendRawTransaction",
            json!(format!("0x{}", hex::encode(hash))),
        ),
        Exchange::Reply("pax_sequencerReceipt", Value::Null),
        Exchange::Reply("eth_blockNumber", json!("0x11")),
        Exchange::Reply("eth_blockNumber", json!("0x12")),
    ]);
    must(runtime.enqueue(&mut store, &alpha, 1, head(), deadline(), false, 1000));
    let low = must(runtime.start_next(&mut store, 1000)).unwrap_or_else(|| panic!("low call"));
    must(runtime.enqueue(&mut store, &beta, 2, head(), deadline(), false, 1000));
    assert!(matches!(
        runtime.start_next(&mut store, 1000),
        Err(LifecycleError::Admission(AdmissionError::Backpressure {
            source: BackpressureSource::InFlight,
            ..
        }))
    ));
    for id in 3..=6 {
        must(runtime.enqueue(&mut store, &alpha, id, head(), deadline(), false, 1000));
    }
    assert!(matches!(
        runtime.enqueue(&mut store, &alpha, 7, head(), deadline(), false, 1000),
        Err(LifecycleError::Admission(AdmissionError::Backpressure {
            source: BackpressureSource::TenantQueue,
            ..
        }))
    ));
    must(runtime.enqueue(
        &mut store,
        &alpha,
        8,
        NodeWork::Submit {
            signed_bytes: signed.clone(),
        },
        deadline(),
        false,
        1000,
    ));
    must(runtime.enqueue(
        &mut store,
        &beta,
        9,
        NodeWork::Receipt {
            submission_id: hash,
        },
        deadline(),
        false,
        1000,
    ));
    let submission =
        must(runtime.start_next(&mut store, 1000)).unwrap_or_else(|| panic!("submission capacity"));
    let receipt =
        must(runtime.start_next(&mut store, 1000)).unwrap_or_else(|| panic!("receipt capacity"));
    assert_eq!(submission.request_id(), 8);
    assert_eq!(receipt.request_id(), 9);
    let result = submission.execute(&node, 1000);
    assert!(matches!(result, Ok(NodeResponse::Submitted(value)) if value == hash));
    must(runtime.finish_call(&mut store, submission, &result, 1000));
    let result = receipt.execute(&node, 1000);
    assert!(matches!(result, Ok(NodeResponse::Receipt(None))));
    must(runtime.finish_call(&mut store, receipt, &result, 1000));
    let result = low.execute(&node, 1000);
    assert!(matches!(result, Ok(NodeResponse::Head(17))));
    must(runtime.finish_call(&mut store, low, &result, 1000));
    let other =
        must(runtime.start_next(&mut store, 1000)).unwrap_or_else(|| panic!("other tenant"));
    assert_eq!(other.request_id(), 2);
    let result = other.execute(&node, 1000);
    assert!(matches!(result, Ok(NodeResponse::Head(18))));
    must(runtime.finish_call(&mut store, other, &result, 1000));
    let requests = must(server.join());
    assert_eq!(
        requests[0]["params"],
        json!([format!("0x{}", hex::encode(signed))])
    );
    assert_eq!(
        requests[1]["params"],
        json!([format!("0x{}", hex::encode(hash))])
    );
    assert_eq!(
        runtime.lane_utilization(Priority::BulkRead).queued_requests,
        4
    );
    assert_eq!(must(runtime.metrics(1000)).unresolved, 1);
}

#[test]
fn deadline_disconnect_and_reopen_retain_real_unknown_submission() {
    let directory = Directory::new();
    let store_root = directory.0.join("store");
    let counters = directory.0.join("counters");
    let mut store = must(Store::open(&store_root));
    let alpha = principal(&mut store, "alpha", 1);
    let mut runtime = boundary(&mut store, &counters, 100, 100, 1000);
    let before = raw(1);
    let before_hash = transaction_hash(&before);
    let token = must(runtime.enqueue(
        &mut store,
        &alpha,
        1,
        NodeWork::Submit {
            signed_bytes: before,
        },
        deadline(),
        false,
        1000,
    ));
    assert_eq!(
        must(runtime.transport_eof(&mut store, &alpha, 1, &mut eof(), 1001)),
        DisconnectOutcome::Cancelled
    );
    assert!(token.is_cancelled());
    assert!(must(runtime.start_next(&mut store, 1001)).is_none());
    assert!(store
        .get(&must(TenantKey::new(
            alpha.tenant.clone(),
            ObjectKind::Outbox,
            before_hash.to_vec()
        )))
        .is_none());

    let signed = raw(2);
    let hash = transaction_hash(&signed);
    let token = must(runtime.enqueue(
        &mut store,
        &alpha,
        2,
        NodeWork::Submit {
            signed_bytes: signed.clone(),
        },
        deadline(),
        false,
        1002,
    ));
    let call = must(runtime.start_next(&mut store, 1002)).unwrap_or_else(|| panic!("broadcast"));
    let (node, server) = serve(vec![Exchange::DropAfterRequest("eth_sendRawTransaction")]);
    let result = call.execute(&node, 1002);
    assert!(matches!(result, Err(CallError::Node(_))));
    let requests = must(server.join());
    assert_eq!(
        requests[0]["params"],
        json!([format!("0x{}", hex::encode(&signed))])
    );
    assert_eq!(
        must(runtime.transport_eof(&mut store, &alpha, 2, &mut eof(), 1003)),
        DisconnectOutcome::ResolutionContinues {
            submission_id: hash
        }
    );
    assert!(!token.is_cancelled());
    must(runtime.finish_call(&mut store, call, &result, 1003));
    assert!(!must(runtime.synchronize_resolution(2)));
    let view = runtime
        .view(2)
        .unwrap_or_else(|| panic!("unknown resolver"));
    assert_eq!(view.owner, WorkOwner::DaemonResolver);
    assert!(matches!(
        view.work,
        TrackedWork::Write {
            reservation_held: true,
            ..
        }
    ));
    must(runtime.expire(&mut store, 6000));
    assert!(!token.is_cancelled());
    drop(runtime);
    drop(store);
    let mut store = must(Store::open(&store_root));
    let mut runtime = boundary(&mut store, &counters, 100, 100, 6000);
    let recovered = runtime
        .recovered_request(&alpha.tenant, hash)
        .unwrap_or_else(|| panic!("restored resolver"));
    let view = runtime
        .view(recovered)
        .unwrap_or_else(|| panic!("recovered work"));
    assert_eq!(view.owner, WorkOwner::DaemonResolver);
    assert!(!view.cancelled);
    assert!(!view.caller_connected);
    assert!(matches!(
        view.work,
        TrackedWork::Write {
            reservation_held: true,
            ..
        }
    ));
    assert_eq!(must(runtime.outbox_mut().exact_signed_bytes(hash)), signed);
    let (mut node, server) = serve(vec![
        Exchange::Reply("pax_sequencerReceipt", Value::Null),
        Exchange::Reply("eth_getTransactionCount", json!("0x2")),
        Exchange::Reply(
            "eth_sendRawTransaction",
            json!(format!("0x{}", hex::encode(hash))),
        ),
    ]);
    let resolution = must(resolve_unknown(
        runtime.outbox_mut(),
        &mut store,
        hash,
        6001,
        &[],
        &mut node,
    ));
    assert_eq!(resolution.state, SubmissionState::Unknown);
    assert_eq!(
        resolution.observation,
        ResolutionObservation::ReceiptMissing
    );
    assert!(!must(runtime.synchronize_resolution(recovered)));
    must(runtime.expire(&mut store, 20_000));
    assert!(matches!(
        runtime
            .view(recovered)
            .unwrap_or_else(|| panic!("hold survives"))
            .work,
        TrackedWork::Write {
            reservation_held: true,
            ..
        }
    ));
    let requests = must(server.join());
    assert_eq!(
        requests[2]["params"],
        json!([format!("0x{}", hex::encode(signed))])
    );
    let (mut node, server) = serve(vec![
        Exchange::Reply("pax_sequencerReceipt", Value::Null),
        Exchange::Reply("eth_getTransactionCount", json!("0x3")),
    ]);
    let resolution = must(resolve_unknown(
        runtime.outbox_mut(),
        &mut store,
        hash,
        20_001,
        &[],
        &mut node,
    ));
    assert_eq!(resolution.state, SubmissionState::Failed);
    assert_eq!(resolution.observation, ResolutionObservation::NonceConsumed);
    assert!(must(runtime.synchronize_resolution(recovered)));
    assert!(runtime.view(recovered).is_none());
    assert_eq!(must(server.join()).len(), 2);
}

#[test]
fn shedding_cancels_one_authenticated_client_and_preserves_resolvers() {
    let directory = Directory::new();
    let mut store = must(Store::open(directory.0.join("store")));
    let alpha = principal(&mut store, "alpha", 1);
    let beta = principal(&mut store, "beta", 2);
    let mut runtime = boundary(&mut store, &directory.0.join("counters"), 100, 1, 1000);
    let signed = raw(3);
    let hash = transaction_hash(&signed);
    let (node, server) = serve(vec![
        Exchange::Reply(
            "eth_sendRawTransaction",
            json!(format!("0x{}", hex::encode(hash))),
        ),
        Exchange::Reply("eth_blockNumber", json!("0x22")),
    ]);
    let resolver_token = must(runtime.enqueue(
        &mut store,
        &alpha,
        1,
        NodeWork::Submit {
            signed_bytes: signed,
        },
        deadline(),
        false,
        1000,
    ));
    let submission = must(runtime.start_next(&mut store, 1000)).unwrap_or_else(|| panic!("submit"));
    let result = submission.execute(&node, 1000);
    assert!(result.is_ok());
    must(runtime.finish_call(&mut store, submission, &result, 1000));
    let cancelled = must(runtime.enqueue(&mut store, &alpha, 2, head(), deadline(), false, 1001));
    let read = must(runtime.start_next(&mut store, 1001)).unwrap_or_else(|| panic!("read"));
    assert!(matches!(
        runtime.enqueue(&mut store, &alpha, 3, head(), deadline(), false, 1002),
        Err(LifecycleError::Quota(QuotaError::ClientShed { .. }))
    ));
    assert!(cancelled.is_cancelled());
    assert!(!resolver_token.is_cancelled());
    assert!(matches!(
        read.execute(&node, 1002),
        Err(CallError::Cancelled)
    ));
    // A detached transport still occupies its slot; the second tenant can enqueue immediately.
    must(runtime.enqueue(&mut store, &beta, 4, head(), deadline(), false, 1002));
    must(runtime.finish_call(&mut store, read, &Err(CallError::Cancelled), 1002));
    let other =
        must(runtime.start_next(&mut store, 1002)).unwrap_or_else(|| panic!("other client"));
    let result = other.execute(&node, 1002);
    assert!(matches!(result, Ok(NodeResponse::Head(34))));
    must(runtime.finish_call(&mut store, other, &result, 1002));
    let health = must(runtime.quota_health(&store, &alpha.tenant, 1002));
    assert_eq!(health.actively_shed_clients, vec![client_id(&alpha)]);
    assert!(must(runtime.quota_health(&store, &beta.tenant, 1002))
        .actively_shed_clients
        .is_empty());
    let view = runtime.view(1).unwrap_or_else(|| panic!("daemon resolver"));
    assert_eq!(view.owner, WorkOwner::DaemonResolver);
    assert!(!view.cancelled);
    assert_eq!(must(runtime.metrics(1002)).unresolved, 1);
    assert_eq!(
        must(runtime.rate_utilization(&alpha.tenant, 1002))[0].used,
        2
    );
    assert_eq!(must(server.join()).len(), 2);
}

fn activity(client: &str, now: u64) -> ClientActivity {
    ClientActivity {
        tenant: tenant("alpha"),
        client_id: client.to_owned(),
        operation_digest: [7; 32],
        retry: true,
        observed_at_ms: now,
    }
}
fn bounded_quota() -> Quota {
    must(quota(1).with_observation_bounds(ObservationBounds {
        maximum_clients_per_tenant: 4,
        audit_retention_ms: 300,
    }))
}

#[test]
fn quota_churn_and_shed_retention_are_bounded_and_indexed() {
    let directory = Directory::new();
    let mut store = must(Store::open(&directory.0));
    let owner = tenant("alpha");
    let mut quota = bounded_quota();
    must(quota.restore(&mut store, 1000));
    for id in 0..4 {
        assert!(
            must(quota.observe_activity(&mut store, activity(&format!("initial-{id}"), 1000)))
                .is_none()
        );
    }
    assert!(matches!(
        quota.observe_activity(&mut store, activity("excess", 1000)),
        Err(QuotaError::ObservationCapacity { limit: 4, .. })
    ));
    assert_eq!(quota.active_windows(&owner), 4);
    // New identities cannot evict a still-live observation and reset its counters.
    let decision = must(quota.observe_activity(&mut store, activity("initial-0", 1001)))
        .unwrap_or_else(|| panic!("tracked client must shed"));
    assert_eq!(decision.requests_in_window, 2);
    assert!(matches!(
        quota.admit_work(&store, &owner, "initial-0", Priority::BulkRead, 1001),
        Err(QuotaError::ClientShed {
            retry_after_ms: 200,
            ..
        })
    ));
    assert!(quota
        .admit_work(&store, &owner, "initial-0", Priority::Submission, 1001)
        .is_ok());
    assert!(quota
        .admit_work(
            &store,
            &owner,
            "initial-0",
            Priority::ReceiptResolution,
            1001
        )
        .is_ok());
    let mut last_time = 0;
    for period in 0..100 {
        let now = 2000 + period * 1000;
        last_time = now;
        for id in 0..4 {
            let client = format!("churn-{period}-{id}");
            assert!(must(quota.observe_activity(&mut store, activity(&client, now))).is_none());
            let decision = must(quota.observe_activity(&mut store, activity(&client, now)))
                .unwrap_or_else(|| panic!("shed churn"));
            let key = must(TenantKey::new(
                owner.clone(),
                ObjectKind::Configuration,
                decision.durable_record,
            ));
            assert!(store.get(&key).is_some());
            assert!(matches!(
                quota.admit_work(&store, &owner, &client, Priority::BulkRead, now),
                Err(QuotaError::ClientShed {
                    retry_after_ms: 200,
                    ..
                })
            ));
        }
        assert_eq!(quota.active_windows(&owner), 4);
        assert_eq!(must(quota.retained_decisions(&store, &owner)), 4);
        assert_eq!(
            store
                .list_object_ids(&owner, ObjectKind::Configuration)
                .len(),
            5
        );
        assert_eq!(
            must(quota.health(&store, &owner, now))
                .actively_shed_clients
                .len(),
            4
        );
        assert!(must(fs::metadata(directory.0.join("store.bin"))).len() < 8000);
    }
    // Repeated shed periods of the same client overwrite the indexed latest record.
    let mut stable_key = None;
    for period in 0..100 {
        let now = last_time + 1000 + period * 201;
        assert!(must(quota.observe_activity(&mut store, activity("stable", now))).is_none());
        let decision = must(quota.observe_activity(&mut store, activity("stable", now)))
            .unwrap_or_else(|| panic!("repeated shed"));
        if let Some(key) = &stable_key {
            assert_eq!(&decision.durable_record, key);
        }
        stable_key = Some(decision.durable_record);
        assert_eq!(must(quota.retained_decisions(&store, &owner)), 1);
        assert_eq!(
            store
                .list_object_ids(&owner, ObjectKind::Configuration)
                .len(),
            2
        );
        last_time = now;
    }
    drop(quota);
    drop(store);
    let mut store = must(Store::open(&directory.0));
    let mut quota = bounded_quota();
    must(quota.restore(&mut store, last_time));
    assert!(matches!(
        quota.admit_work(&store, &owner, "stable", Priority::BulkRead, last_time),
        Err(QuotaError::ClientShed {
            retry_after_ms: 200,
            ..
        })
    ));
    must(quota.restore(&mut store, last_time + 501));
    assert_eq!(must(quota.retained_decisions(&store, &owner)), 0);
    assert_eq!(
        store
            .list_object_ids(&owner, ObjectKind::Configuration)
            .len(),
        1
    );
    assert!(matches!(
        quota.observe_activity(&mut store, activity("backwards", last_time)),
        Err(QuotaError::TimeRegressed)
    ));
}

#[test]
fn independent_daemons_and_reopen_share_atomic_durable_counters() {
    let directory = Directory::new();
    let counters = directory.0.join("counters");
    // Independently opened authorities and real daemon owners, not clones of one Arc ledger.
    let mut first_store = must(Store::open(directory.0.join("first")));
    let mut second_store = must(Store::open(directory.0.join("second")));
    let first_principal = principal(&mut first_store, "alpha", 1);
    let second_principal = principal(&mut second_store, "alpha", 2);
    let first = boundary(&mut first_store, &counters, 8, 100, 1000);
    let second = boundary(&mut second_store, &counters, 8, 100, 1000);
    let barrier = Arc::new(Barrier::new(2));
    let worker = |mut runtime: TenantBoundary,
                  mut store: Store,
                  principal: ResolvedPrincipal,
                  barrier: Arc<Barrier>| {
        thread::spawn(move || {
            barrier.wait();
            let mut accepted = Vec::new();
            let mut refused = 0;
            for id in 1..=8 {
                match runtime.enqueue(&mut store, &principal, id, head(), deadline(), false, 1000) {
                    Ok(_) => {
                        let call = must(runtime.start_next(&mut store, 1000))
                            .unwrap_or_else(|| panic!("dispatch shared counter admission"));
                        let (node, server) =
                            serve(vec![Exchange::Reply("eth_blockNumber", json!("0x2a"))]);
                        let result = call.execute(&node, 1000);
                        assert!(matches!(result, Ok(NodeResponse::Head(42))));
                        must(runtime.finish_call(&mut store, call, &result, 1000));
                        assert_eq!(must(server.join()).len(), 1);
                        accepted.push(id);
                    }
                    Err(LifecycleError::Rate(Refusal::Exceeded {
                        limit,
                        window,
                        remaining,
                        retry_after_ms,
                    })) => {
                        assert_eq!(limit.limit, 8);
                        assert_eq!(window.start_ms, 1000);
                        assert_eq!(remaining, 0);
                        assert_eq!(retry_after_ms, 1000);
                        refused += 1;
                    }
                    Err(error) => panic!("unexpected concurrent admission: {error:?}"),
                }
            }
            assert_eq!(
                runtime.lane_utilization(Priority::BulkRead).queued_requests,
                0
            );
            assert_eq!(must(runtime.metrics(1000)).in_flight, 0);
            (
                accepted.len(),
                refused,
                must(runtime.rate_utilization(&principal.tenant, 1000))[0].clone(),
            )
        })
    };
    let left = worker(first, first_store, first_principal, Arc::clone(&barrier));
    let right = worker(second, second_store, second_principal, barrier);
    let left = must(left.join());
    let right = must(right.join());
    assert_eq!(left.0 + right.0, 8);
    assert_eq!(left.1 + right.1, 8);
    // Both daemon owners have stopped. Reopen the real store and authority within the live window.
    let mut store = must(Store::open(directory.0.join("first")));
    let mut runtime = boundary(&mut store, &counters, 8, 100, 1000);
    let reopened = must(runtime.rate_utilization(&tenant("alpha"), 1000));
    assert_eq!(reopened[0].used, 8);
    assert_eq!(reopened[0].remaining, 0);
    assert_eq!(reopened[0].ledger_revision, 8);
    assert!(left.2.ledger_revision <= 8 && right.2.ledger_revision <= 8);
    let authenticated = principal(&mut store, "alpha", 3);
    assert!(matches!(
        runtime.enqueue(
            &mut store,
            &authenticated,
            100,
            head(),
            deadline(),
            false,
            1000
        ),
        Err(LifecycleError::Rate(Refusal::Exceeded { remaining: 0, .. }))
    ));
    must(runtime.enqueue(
        &mut store,
        &authenticated,
        101,
        head(),
        deadline(),
        false,
        2000,
    ));
    let usage = must(runtime.rate_utilization(&authenticated.tenant, 2000));
    assert_eq!(usage[0].used, 1);
    assert_eq!(usage[0].ledger_revision, 9);
}

fn rate_request(now: u64) -> RateRequest {
    RateRequest {
        tenant: "alpha".to_owned(),
        agent: "agent".to_owned(),
        session: "session".to_owned(),
        capability: "authenticated".to_owned(),
        operation_class: "read".to_owned(),
        logical_time_ms: now,
        cost: 1,
    }
}
fn limiter(root: &Path, limit: u64) -> RateLimiter {
    must(RateLimiter::new(
        configs(limit),
        must(CounterLedger::open(root)),
    ))
}

#[test]
fn interleaved_and_concurrent_delayed_windows_preserve_ceiling() {
    let directory = Directory::new();
    let root = directory.0.join("interleaved");
    let rate = limiter(&root, 2);
    assert_eq!(must(rate.admit(&rate_request(1000))).ledger_revision, 1); // A
    assert_eq!(must(rate.admit(&rate_request(2000))).ledger_revision, 2); // B
    match rate.admit(&rate_request(1000)) {
        // delayed A
        Err(Refusal::StaleWindow {
            limit,
            window,
            remaining,
            retry_after_ms,
        }) => {
            assert_eq!(limit.id.as_str(), "alpha-ceiling");
            assert_eq!(window.start_ms, 2000);
            assert_eq!(window.end_ms, 3000);
            assert_eq!(remaining, 1);
            assert_eq!(retry_after_ms, 2000);
        }
        other => panic!("stale A must refuse with current B metadata: {other:?}"),
    }
    assert_eq!(must(rate.admit(&rate_request(2000))).ledger_revision, 3); // B
    assert!(matches!(
        rate.admit(&rate_request(2000)),
        Err(Refusal::Exceeded {
            remaining: 0,
            retry_after_ms: 1000,
            ..
        })
    ));
    drop(rate);
    let rate = limiter(&root, 2);
    assert!(matches!(
        rate.admit(&rate_request(1000)),
        Err(Refusal::StaleWindow { remaining: 0, .. })
    ));
    assert_eq!(must(rate.utilization("alpha", 2000))[0].ledger_revision, 3);
    drop(rate);

    let concurrent = directory.0.join("concurrent");
    let rate = limiter(&concurrent, 12);
    must(rate.admit(&rate_request(1000)));
    must(rate.admit(&rate_request(2000)));
    drop(rate);
    let barrier = Arc::new(Barrier::new(24));
    let workers = (0..24)
        .map(|index| {
            let barrier = Arc::clone(&barrier);
            let rate = limiter(&concurrent, 12);
            thread::spawn(move || {
                barrier.wait();
                let now = if index % 2 == 0 { 1000 } else { 2000 };
                match rate.admit(&rate_request(now)) {
                    Ok(admission) => {
                        assert_eq!(now, 2000);
                        Some(admission.ledger_revision)
                    }
                    Err(Refusal::StaleWindow {
                        window,
                        remaining,
                        retry_after_ms,
                        ..
                    }) => {
                        assert_eq!(now, 1000);
                        assert_eq!(window.start_ms, 2000);
                        assert!(remaining <= 11);
                        assert_eq!(retry_after_ms, 2000);
                        None
                    }
                    Err(Refusal::Exceeded {
                        window,
                        remaining,
                        retry_after_ms,
                        ..
                    }) => {
                        assert_eq!(now, 2000);
                        assert_eq!(window.start_ms, 2000);
                        assert_eq!(remaining, 0);
                        assert_eq!(retry_after_ms, 1000);
                        None
                    }
                    other => panic!("delayed concurrent counter: {other:?}"),
                }
            })
        })
        .collect::<Vec<_>>();
    let revisions = workers
        .into_iter()
        .filter_map(|worker| must(worker.join()))
        .collect::<BTreeSet<_>>();
    assert_eq!(revisions, (3..=13).collect::<BTreeSet<_>>());
    let rate = limiter(&concurrent, 12);
    let usage = must(rate.utilization("alpha", 2000));
    assert_eq!(usage[0].used, 12);
    assert_eq!(usage[0].remaining, 0);
    assert_eq!(usage[0].ledger_revision, 13);
    // A smaller matching dimension refuses without charging any other dimension or revision.
    let layered_root = directory.0.join("layered");
    let mut layered = configs(10);
    layered.push(LimitConfig {
        id: must(LimitId::new("session-ceiling")),
        scope: LimitScope::Session {
            tenant: "alpha".to_owned(),
            session: "session".to_owned(),
        },
        limit: 1,
        window_ms: 1000,
    });
    let layered = must(RateLimiter::new(
        layered,
        must(CounterLedger::open(&layered_root)),
    ));
    must(layered.admit(&rate_request(1000)));
    assert!(matches!(
        layered.admit(&rate_request(1000)),
        Err(Refusal::Exceeded { remaining: 0, .. })
    ));
    for usage in must(layered.utilization("alpha", 1000)) {
        assert_eq!(usage.used, 1);
        assert_eq!(usage.ledger_revision, 1);
    }
}
