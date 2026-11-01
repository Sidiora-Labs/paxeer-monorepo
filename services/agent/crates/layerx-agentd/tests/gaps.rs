use std::collections::BTreeSet;
use std::fs;
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::thread;
use std::time::Duration;

use layerx_agent_api::identity::{AgentDid, CapabilityId, ExplicitSet, TenantId as ApiTenantId};
use layerx_agent_api::subscription::{
    Cursor as ApiCursor, DeliveryTarget, SubscriptionCreate, SubscriptionFilter, SubscriptionId,
    SubscriptionScope, SubscriptionTarget,
};

use layerx_agentd::events::gap::{
    admit, admit_authorized, apply_backfill, apply_backfill_authorized, attempt_backfill, detect,
    detect_authorized, enforce_retention, enforce_retention_authorized, BackfillFailure,
    BackfillReport, BackfillResolution, GapError, Retention, Truncated,
};
use layerx_agentd::events::subscription::{
    Continuity, Store as SubscriptionStore, SubscriptionError,
};
use layerx_agentd::events::{
    backfill, ingest, CoreEvent, DeliveryEngine, DeliveryError, EventAttributes, EventIngestor,
    RetryPolicy,
};
use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityRecord, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::read::{Cursor, HistoryLimits, HistoryReadError};
use layerx_agentd::session::{close, open, OpenRequest, SessionId, SessionRegistry, Token};
use layerx_agentd::store::{Store, TenantId};
use layerx_agentd::tenant::{AuthorizationError, TenantObservability};
use layerx_client::evidence::RootSelector;
use layerx_client::head::Head;
use layerx_client::lni::framing::{read_frame, write_frame};
use layerx_client::lni::schema::{decode_envelope, encode_envelope, Envelope, Version};
use layerx_client::lni::transport::{ConnectionGate, Limits, Uds};
use layerx_client::read::{ReadContext, Requested};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_types::ids::Did;
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};

static NEXT_PATH: AtomicU64 = AtomicU64::new(1);

struct BoundaryIdentity(CoreIdentity);

impl IdentityResolver for BoundaryIdentity {
    fn resolve(&mut self, _did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(self.0.clone()))
    }
}

struct SocketPath(PathBuf);

impl SocketPath {
    fn new(label: &str) -> Self {
        let sequence = NEXT_PATH.fetch_add(1, Ordering::Relaxed);
        Self(std::env::temp_dir().join(format!(
            "layerx-agentd-gap-{label}-{}-{sequence}.sock",
            std::process::id()
        )))
    }
}

impl Drop for SocketPath {
    fn drop(&mut self) {
        let _ = fs::remove_file(&self.0);
    }
}

fn test_directory(name: &str) -> PathBuf {
    let sequence = NEXT_PATH.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-agentd-gap-{name}-{}-{sequence}",
        std::process::id()
    ))
}

fn text<T, E: std::fmt::Debug>(result: Result<T, E>, label: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{label} must be valid: {error:?}"),
    }
}

fn owned_durable(subscriptions: SubscriptionStore) -> Store {
    match subscriptions.into_durable() {
        Ok(durable) => durable,
        Err(_) => panic!("owned subscription store returned unexpected Err(Self)"),
    }
}

fn durable_tenant() -> TenantId {
    text(TenantId::new("tenant-a"), "durable tenant")
}

fn scope() -> SubscriptionScope {
    SubscriptionScope {
        tenant: text(ApiTenantId::new("tenant-a"), "API tenant"),
        agent: text(AgentDid::new("agent-a"), "agent"),
        capability: text(CapabilityId::new("capability-a"), "capability"),
    }
}

fn id() -> SubscriptionId {
    text(SubscriptionId::new("subscription-a"), "subscription")
}

fn target() -> SubscriptionTarget {
    SubscriptionTarget {
        scope: scope(),
        subscription_id: id(),
    }
}

fn filter() -> SubscriptionFilter {
    SubscriptionFilter {
        agents: ExplicitSet::deny_all(),
        accounts: ExplicitSet::deny_all(),
        activity_types: ExplicitSet::deny_all(),
        modules: ExplicitSet::deny_all(),
        assets: ExplicitSet::deny_all(),
        counterparties: ExplicitSet::deny_all(),
        result_classes: ExplicitSet::deny_all(),
    }
}

fn subscriptions(root: &PathBuf, start: u64) -> SubscriptionStore {
    let durable = match Store::open(root) {
        Ok(value) => value,
        Err(error) => panic!("store open failed: {error}"),
    };
    let mut subscriptions = match SubscriptionStore::open(durable, durable_tenant()) {
        Ok(value) => value,
        Err(error) => panic!("subscription store open failed: {error}"),
    };
    let create = SubscriptionCreate {
        scope: scope(),
        filter: filter(),
        start: ApiCursor {
            block_height: start,
            tx_index: 0,
            log_index: 0,
        },
        delivery_target: text(DeliveryTarget::new("uds://consumer-a"), "target"),
    };
    if let Err(error) = subscriptions.create(id(), create) {
        panic!("subscription create failed: {error}");
    }
    subscriptions
}

fn session_identity(store: &mut Store) -> IdentityRecord {
    let mut boundary = BoundaryIdentity(CoreIdentity {
        canonical_bytes: b"gap-session-identity".to_vec(),
        head_sequence: 10,
        revocation_sequence: 1,
        verification_level: VerificationLevel::STATE_PROVEN,
        frozen: false,
        authorities: vec![ProtocolAuthority::SessionKey([4; 32])],
    });
    text(
        register(
            store,
            durable_tenant(),
            text(Did::new(b"agent-a"), "agent DID"),
            &mut boundary,
        ),
        "session identity",
    )
}

fn authorized_subscriptions(
    root: &Path,
    start: u64,
) -> (SubscriptionStore, Store, SessionRegistry, Token) {
    let mut session_store = text(Store::open(root.join("sessions")), "session store");
    let identity = session_identity(&mut session_store);
    let mut sessions = SessionRegistry::default();
    let token = text(
        open(
            &mut session_store,
            &mut sessions,
            &identity,
            OpenRequest {
                session_id: SessionId([1; 32]),
                token_id: [2; 32],
                tenant: durable_tenant(),
                agent: text(Did::new(b"agent-a"), "agent DID"),
                authority: ProtocolAuthority::SessionKey([4; 32]),
                permitted_activity_types: BTreeSet::from([9]),
                scopes: BTreeSet::from(["subscribe".to_owned()]),
                expiry_sequence: 100,
                expiry_seconds: None,
                opening_client: "gap-suite".to_owned(),
                policy_version: "policy-v1".to_owned(),
            },
            10,
        ),
        "session",
    );
    let durable = text(Store::open(root.join("events")), "event store");
    let mut subscriptions = text(
        SubscriptionStore::open(durable, durable_tenant()),
        "subscription store",
    );
    text(
        subscriptions.create_authorized(
            &sessions,
            &token,
            &mut TenantObservability::default(),
            10,
            id(),
            SubscriptionCreate {
                scope: scope(),
                filter: filter(),
                start: ApiCursor {
                    block_height: start,
                    tx_index: 0,
                    log_index: 0,
                },
                delivery_target: text(DeliveryTarget::new("uds://consumer-a"), "target"),
            },
        ),
        "authorized subscription",
    );
    (subscriptions, session_store, sessions, token)
}

fn limits() -> Limits {
    Limits {
        maximum_frame_bytes: 1024 * 1024,
        maximum_connections: 1,
        maximum_streams: 2,
        maximum_queued_bytes: 1024 * 1024,
        deadline: Duration::from_secs(2),
    }
}

pub fn history_limits() -> HistoryLimits {
    HistoryLimits {
        maximum_items: 2,
        maximum_response_bytes: 1_048_576,
    }
}

pub fn context() -> ReadContext {
    ReadContext {
        interface_version: Version::V1_0,
        correlation_id: 77,
        expected_protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
        expected_network_id: 77,
        requested: Requested::new(VerificationLevel::UNVERIFIED),
        head: Head {
            chain_sequence: 20,
            sealed_batch: 2,
            finalised_checkpoint: [0x71; 32],
        },
        sequencer_authorization: SequencerAuthorization::new([1; 32], [2; 32], 0, u64::MAX),
        handshake_sequencer_key: [2; 32],
        root_selector: RootSelector::Latest,
    }
}

pub fn log(height: u64, tx: u32, index: u32) -> Value {
    json!({"blockNumber": format!("0x{height:x}"), "transactionIndex": format!("0x{tx:x}"),
        "logIndex": format!("0x{index:x}"), "blockHash": format!("0x{}", "ab".repeat(32)),
        "address": format!("0x{}", "12".repeat(20)), "topics": [format!("0x{}", "34".repeat(32))],
        "data": "0x00ff", "removed": false})
}

pub fn attributes() -> EventAttributes {
    EventAttributes {
        agent: "agent-a".to_owned(),
        account: "account-a".to_owned(),
        activity_type: 9,
        module: "asset".to_owned(),
        asset: "LXR".to_owned(),
        counterparty: "counterparty-a".to_owned(),
        result_code: 0,
    }
}

pub fn row(height: u64, tx: u32, index: u32) -> Value {
    json!({"height": height.to_string(), "txIndex": tx.to_string(), "logIndex": index.to_string(),
        "kind":"Event", "canonical": hex::encode(serde_json::to_vec(&log(height, tx, index)).unwrap()), "proof": null})
}

fn receive_history(stream: &mut UnixStream, cursor: Value, limit: u16) {
    stream
        .set_read_timeout(Some(Duration::from_secs(3)))
        .unwrap();
    let frame = read_frame(stream, 1_048_576).unwrap();
    let envelope = decode_envelope(&frame).unwrap();
    assert_eq!(envelope.message_tag, 9);
    let request: Value = serde_json::from_slice(envelope.canonical_payload).unwrap();
    assert_eq!(
        request,
        json!({"jsonrpc":"2.0", "id":77, "method":"px_getHistory", "params":[{
        "from_height":"11", "to_height":"12", "cursor":cursor, "limit":limit.to_string()}]})
    );
}

fn response(stream: &mut UnixStream, result: Value) {
    let bytes = serde_json::to_vec(&result).unwrap();
    let encoded = encode_envelope(Envelope {
        version: Version::V1_0,
        message_tag: 11,
        correlation_id: 77,
        canonical_payload: &bytes,
        proof_material: &[],
    })
    .unwrap();
    write_frame(stream, &encoded, 1_048_576).unwrap();
}

pub struct HistoryServer {
    socket: SocketPath,
    server: thread::JoinHandle<()>,
}

impl HistoryServer {
    pub fn new(pages: Vec<(Value, u16, Value)>) -> Self {
        let socket = SocketPath::new("history");
        let listener = UnixListener::bind(&socket.0).unwrap();
        let server = thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            for (cursor, limit, result) in pages {
                receive_history(&mut stream, cursor, limit);
                response(&mut stream, result);
            }
        });
        Self { socket, server }
    }
    pub fn connect(&self) -> Uds {
        Uds::connect(&self.socket.0, &ConnectionGate::new(1), limits()).unwrap()
    }
    pub fn finish(self) {
        self.server.join().unwrap();
    }
}

pub fn page(items: Vec<Value>, cursor: Value) -> Value {
    json!({"jsonrpc":"2.0", "id":77, "result":{"height":"13", "items":items, "cursor":cursor}})
}

fn recovered_event(sequence: u64, bytes: Vec<u8>) -> CoreEvent {
    let mut event = CoreEvent::from_log_bytes(bytes, attributes()).unwrap();
    event.global_sequence = sequence;
    event
}

#[test]
fn skipped_sequence_is_backfilled_over_the_real_stream_before_unblocking() {
    let root = test_directory("recovered");
    let mut subscriptions = subscriptions(&root, 11);
    let gap = match detect(&mut subscriptions, &target(), 11, 13) {
        Ok(Some(value)) => value,
        other => panic!("gap was not detected: {other:?}"),
    };
    assert!(
        matches!(admit(&subscriptions, &target()), Err(GapError::Blocked(value)) if value == gap)
    );

    let server = HistoryServer::new(vec![(
        Value::Null,
        2,
        page(vec![row(11, 0, 0), row(12, 0, 0)], Value::Null),
    )]);
    let mut transport = server.connect();
    let report = attempt_backfill(&mut transport, gap, context(), history_limits(), 2);
    server.finish();
    let recovered = match &report {
        BackfillReport::Recovered { events, .. } => events.clone(),
        other @ BackfillReport::Incomplete { .. } => {
            panic!("backfill did not recover: {other:?}")
        }
    };

    let durable = owned_durable(subscriptions);
    let mut ingestor = match EventIngestor::open(durable, durable_tenant(), 2, 11) {
        Ok(value) => value,
        Err(error) => panic!("ingestor open failed: {error}"),
    };
    for recovered in recovered {
        if let Err(error) = ingest(
            &mut ingestor,
            recovered_event(recovered.global_sequence, recovered.canonical_bytes),
        ) {
            panic!("recovered event persistence failed: {error}");
        }
    }
    ingestor
        .new_head(
            13,
            &mut transport,
            history_limits(),
            context(),
            2,
            attributes(),
        )
        .unwrap();
    let mut subscriptions = match SubscriptionStore::open(ingestor.into_store(), durable_tenant()) {
        Ok(value) => value,
        Err(error) => panic!("subscription reopen failed: {error}"),
    };
    assert!(matches!(
        apply_backfill(&mut subscriptions, &target(), gap, &report),
        Ok(BackfillResolution::Restored)
    ));
    assert!(matches!(
        subscriptions.continuity(&target()),
        Ok(Continuity::Healthy)
    ));
    assert!(admit(&subscriptions, &target()).is_ok());

    drop(subscriptions);
    let durable = match Store::open(&root) {
        Ok(value) => value,
        Err(error) => panic!("store restart failed: {error}"),
    };
    let restarted = match SubscriptionStore::open(durable, durable_tenant()) {
        Ok(value) => value,
        Err(error) => panic!("subscription restart failed: {error}"),
    };
    assert!(matches!(
        restarted.continuity(&target()),
        Ok(Continuity::Healthy)
    ));
    let _ = fs::remove_dir_all(root);
}

#[test]
fn unrecoverable_core_gap_remains_durably_blocked() {
    let root = test_directory("unrecoverable");
    let mut subscriptions = subscriptions(&root, 11);
    let gap = match detect(&mut subscriptions, &target(), 11, 13) {
        Ok(Some(value)) => value,
        other => panic!("gap was not detected: {other:?}"),
    };
    let server = HistoryServer::new(vec![(
        Value::Null,
        2,
        json!({"jsonrpc":"2.0", "id":77, "error":{"code":-32001,"message":"history pruned"}}),
    )]);
    let mut transport = server.connect();
    let report = attempt_backfill(&mut transport, gap, context(), history_limits(), 2);
    server.finish();
    assert!(matches!(
        &report,
        BackfillReport::Incomplete {
            failure: BackfillFailure::History(HistoryReadError::Rpc(_)),
            ..
        }
    ));
    assert!(matches!(
        apply_backfill(&mut subscriptions, &target(), gap, &report),
        Ok(BackfillResolution::StillBlocked {
            recovered_through: None,
        })
    ));
    assert!(matches!(
        subscriptions.continuity(&target()),
        Ok(Continuity::GapBlocked {
            backfill_attempted: true,
            recovered_through: None,
            ..
        })
    ));

    let retry = RetryPolicy {
        base_delay_ms: 10,
        maximum_delay_ms: 20,
        jitter_percent: 0,
        maximum_attempts: 2,
    };
    let mut delivery = match DeliveryEngine::open(subscriptions, target(), 13, 2, retry) {
        Ok(value) => value,
        Err(error) => panic!("delivery engine open failed: {error}"),
    };
    assert!(matches!(
        backfill(&mut delivery),
        Err(DeliveryError::ContinuityBlocked)
    ));
    let _ = fs::remove_dir_all(root);
}

#[test]
fn retention_expiry_marks_outage_truncated_and_survives_restart() {
    let root = test_directory("truncated");
    let mut subscriptions = subscriptions(&root, 10);
    let _ = detect(&mut subscriptions, &target(), 10, 15);
    let notice = match enforce_retention(
        &mut subscriptions,
        &target(),
        31,
        15,
        Retention {
            maximum_undelivered_sequences: 10,
        },
    ) {
        Ok(Some(value)) => value,
        other => panic!("retention did not truncate: {other:?}"),
    };
    assert_eq!(
        notice,
        Truncated {
            requested_from: 10,
            oldest_available: 21,
            lost_through: 20,
        }
    );
    assert!(matches!(
        admit(&subscriptions, &target()),
        Err(GapError::Truncated(value)) if value == notice
    ));
    drop(subscriptions);

    let durable = match Store::open(&root) {
        Ok(value) => value,
        Err(error) => panic!("store restart failed: {error}"),
    };
    let restarted = match SubscriptionStore::open(durable, durable_tenant()) {
        Ok(value) => value,
        Err(error) => panic!("subscription restart failed: {error}"),
    };
    assert!(matches!(
        restarted.continuity(&target()),
        Ok(Continuity::Truncated {
            requested_from: 10,
            oldest_available: 21,
            lost_through: 20,
        })
    ));
    let _ = fs::remove_dir_all(root);
}

#[test]
fn bound_gap_runtime_requires_the_common_resolver_and_refuses_after_revocation() {
    let root = test_directory("authorized");
    let (mut subscriptions, mut session_store, mut sessions, token) =
        authorized_subscriptions(&root, 11);
    let mut observability = TenantObservability::default();

    assert!(matches!(
        detect(&mut subscriptions, &target(), 11, 13),
        Err(GapError::Subscription(
            SubscriptionError::AuthorizationRequired
        ))
    ));
    let gap = match detect_authorized(
        &mut subscriptions,
        layerx_agentd::events::gap::GapAuthorization {
            sessions: &sessions,
            token: &token,
            observability: &mut observability,
            core_sequence: 11,
        },
        &target(),
        11,
        13,
    ) {
        Ok(Some(value)) => value,
        other => panic!("authorized gap was not detected: {other:?}"),
    };
    assert!(matches!(
        admit(&subscriptions, &target()),
        Err(GapError::Subscription(
            SubscriptionError::AuthorizationRequired
        ))
    ));
    assert!(matches!(
        admit_authorized(
            &subscriptions,
            &sessions,
            &token,
            &mut observability,
            11,
            &target(),
        ),
        Err(GapError::Blocked(value)) if value == gap
    ));
    let report = BackfillReport::Incomplete {
        gap,
        recovered: Vec::new(),
        failure: BackfillFailure::Incomplete,
    };
    assert!(matches!(
        apply_backfill(&mut subscriptions, &target(), gap, &report),
        Err(GapError::Subscription(
            SubscriptionError::AuthorizationRequired
        ))
    ));
    assert!(matches!(
        apply_backfill_authorized(
            &mut subscriptions,
            layerx_agentd::events::gap::GapAuthorization {
                sessions: &sessions,
                token: &token,
                observability: &mut observability,
                core_sequence: 11
            },
            &target(),
            gap,
            &report,
        ),
        Ok(BackfillResolution::StillBlocked {
            recovered_through: None,
        })
    ));

    text(
        close(
            &mut session_store,
            &mut sessions,
            &durable_tenant(),
            SessionId([1; 32]),
        ),
        "session close",
    );
    assert!(matches!(
        admit_authorized(
            &subscriptions,
            &sessions,
            &token,
            &mut observability,
            11,
            &target(),
        ),
        Err(GapError::Subscription(SubscriptionError::Authorization(
            AuthorizationError::Revoked
        )))
    ));
    assert_eq!(observability.audit().len(), 4);

    assert_bound_retention(&root);
    let _ = fs::remove_dir_all(root);
}

fn assert_bound_retention(root: &Path) {
    let retention_root = root.join("retention");
    let (mut retention, _store, sessions, token) = authorized_subscriptions(&retention_root, 10);
    let mut retention_observability = TenantObservability::default();
    assert!(matches!(
        enforce_retention(
            &mut retention,
            &target(),
            31,
            15,
            Retention {
                maximum_undelivered_sequences: 10,
            },
        ),
        Err(GapError::Subscription(
            SubscriptionError::AuthorizationRequired
        ))
    ));
    assert!(matches!(
        enforce_retention_authorized(
            &mut retention,
            layerx_agentd::events::gap::GapAuthorization {
                sessions: &sessions,
                token: &token,
                observability: &mut retention_observability,
                core_sequence: 11
            },
            &target(),
            31,
            15,
            Retention {
                maximum_undelivered_sequences: 10,
            },
        ),
        Ok(Some(Truncated {
            requested_from: 10,
            oldest_available: 21,
            lost_through: 20,
        }))
    ));
    assert_eq!(retention_observability.audit().len(), 1);
}
