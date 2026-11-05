//! Findings and baseline constraints are recorded in src/repair_22_33_1513.kvx.
use std::collections::BTreeSet;
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::{symlink, PermissionsExt};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_agentd::audit::{self, Decision, Log};
use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope};
use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
use layerx_agentd::enrolment::{
    self, BindingMode, BindingPublisher, DaemonSurface, EnrolmentRequest, PublicationStage,
    BINDING_FILE, DAEMON_BEARER_FILE, SESSION_TOKEN_FILE,
};
use layerx_agentd::identity::{
    self, CoreIdentity, IdentityError, IdentityRecord, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::prepare::PreparationLifecycle;
use layerx_agentd::receipt::{self, AuthenticatedTransactionReceipt, ReceiptStoreError};
use layerx_agentd::session::{self, SessionId, SessionRegistry, Token};
use layerx_agentd::session_control::SessionControl;
use layerx_agentd::store::{ObjectKind, Store, TenantId};
use layerx_agentd::tenant::{
    self, AuthorizationOutcome, Operation, RequestContext, Surface, TenantObservability,
};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_types::ids::Did;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};

static NEXT: AtomicU64 = AtomicU64::new(0);
const CAPABILITY: CapabilityId = CapabilityId([0x51; 32]);
const SESSION: SessionId = SessionId([0x52; 32]);

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}
struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let root = must(fs::canonicalize(std::env::temp_dir())).join(format!(
            "repair-1513-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        must(fs::create_dir(&root));
        must(fs::set_permissions(
            &root,
            fs::Permissions::from_mode(0o700),
        ));
        Self(root)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        must(fs::remove_dir_all(&self.0));
    }
}
fn tenant() -> TenantId {
    must(TenantId::new("repair1513"))
}

struct RecordedIdentity(CoreIdentity);
impl IdentityResolver for RecordedIdentity {
    fn resolve(&mut self, _did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(self.0.clone()))
    }
}
fn identity(store: &mut Store, owner: TenantId) -> IdentityRecord {
    let mut peer = RecordedIdentity(CoreIdentity {
        canonical_bytes: b"recorded-identity-1513".to_vec(),
        head_sequence: 10,
        revocation_sequence: 1,
        verification_level: VerificationLevel::STATE_PROVEN,
        frozen: false,
        authorities: vec![ProtocolAuthority::CapabilityGrant(CAPABILITY.0)],
    });
    must(identity::register(
        store,
        owner,
        must(Did::new(b"did:layerx:repair1513:agent")),
        &mut peer,
    ))
}
fn request() -> EnrolmentRequest {
    EnrolmentRequest {
        session_id: SESSION,
        capability_id: CAPABILITY,
        permitted_activity_types: BTreeSet::from([7]),
        scopes: BTreeSet::from(["read:balance".to_owned()]),
        expiry_sequence: 300,
        opening_client: "mcp".to_owned(),
        policy_version: "policy-v1".to_owned(),
        core_sequence: 50,
    }
}
fn setup(root: &Path) -> (Store, IdentityRecord, BindingPublisher) {
    let mut store = must(Store::open(root.join("store")));
    let record = identity(&mut store, tenant());
    must(
        must(Capability::new(
            CAPABILITY,
            tenant(),
            CapabilityDimensions {
                activity_types: BTreeSet::from([7]),
                counterparties: BTreeSet::from([[2; 32]]),
                assets: BTreeSet::from([[3; 32]]),
                amount_ceiling: 100,
                rate_ceiling: RateCeiling {
                    maximum_uses: 2,
                    window_sequences: 10,
                },
                purposes: BTreeSet::from(["service-payment".to_owned()]),
                expiry_sequence: 400,
            },
        ))
        .persist(&mut store),
    );
    let publisher = must(BindingPublisher::new(
        root.join("mcp"),
        root.join("store"),
        root.join("audit"),
        must(DaemonSurface::new(
            "127.0.0.1:9",
            "d".repeat(48),
            [0xcc; 32],
        )),
        LimitConfig {
            id: LimitId([0x0a; 16]),
            name: "mcp".to_owned(),
            scope: LimitScope::Tenant([1; 32]),
            ceiling: 1000,
            consumed: 0,
        },
        Duration::from_secs(5),
        BindingMode::Full,
    ));
    (store, record, publisher)
}
fn stop_publication(
    store: &mut Store,
    sessions: &mut SessionRegistry,
    identity: &IdentityRecord,
    publisher: &BindingPublisher,
    boundary: PublicationStage,
) {
    let mut reached = false;
    let stopped = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        must(enrolment::enrol_observed(
            store,
            sessions,
            identity,
            request(),
            publisher,
            |stage| {
                if stage == boundary {
                    reached = true;
                    panic!("power loss at {stage:?}");
                }
            },
        ));
    }));
    assert!(
        reached,
        "publication must reach the requested durable boundary"
    );
    assert!(stopped.is_err());
}
fn reopen(root: &Path) -> (Store, SessionRegistry) {
    let store = must(Store::open(root.join("store")));
    let mut sessions = SessionRegistry::default();
    must(sessions.restore_tenant(&store, &tenant()));
    (store, sessions)
}
fn token(sessions: &SessionRegistry) -> Token {
    let record = sessions.get(&tenant(), SESSION).unwrap();
    must(sessions.authenticate_bearer(&tenant(), SESSION, record.request.token_id))
}

#[test]
fn enrolment_publication_recovery_reopens_every_boundary_with_one_generation() {
    for boundary in [
        PublicationStage::SessionRecorded,
        PublicationStage::TokenPublished,
        PublicationStage::BearerPublished,
        PublicationStage::BindingPublished,
        PublicationStage::DirectorySynced,
    ] {
        let directory = Directory::new();
        let (mut store, identity, publisher) = setup(&directory.0);
        let mut sessions = SessionRegistry::default();
        stop_publication(&mut store, &mut sessions, &identity, &publisher, boundary);
        let generation = token(&sessions).generation();
        let token_id = token(&sessions).token_id();
        drop(sessions);
        drop(store);
        let (store, sessions) = reopen(&directory.0);
        must(publisher.validate_existing(&sessions, &identity, &request()));
        assert_eq!(sessions.open_count(), 1);
        assert_eq!(token(&sessions).generation(), generation);
        assert_eq!(token(&sessions).token_id(), token_id);
        let document: Value = must(serde_json::from_slice(&must(fs::read(
            publisher.binding_path(),
        ))));
        assert_eq!(document["session_generation"], generation);
        assert_eq!(document["session_id"], hex::encode(SESSION.0));
        assert_eq!(
            must(fs::read_to_string(
                directory.0.join("mcp").join(SESSION_TOKEN_FILE)
            )),
            format!("{}\n", hex::encode(token_id))
        );
        assert_eq!(
            must(fs::read_to_string(
                directory.0.join("mcp").join(DAEMON_BEARER_FILE)
            )),
            format!("{}\n", "d".repeat(48))
        );
        let mut original = Vec::new();
        for name in [BINDING_FILE, SESSION_TOKEN_FILE, DAEMON_BEARER_FILE] {
            let path = directory.0.join("mcp").join(name);
            assert_eq!(
                must(fs::metadata(&path)).permissions().mode() & 0o777,
                0o600
            );
            original.push((path.clone(), must(fs::read(path))));
        }
        drop(sessions);
        drop(store);
        let (_store, sessions) = reopen(&directory.0);
        must(publisher.validate_existing(&sessions, &identity, &request()));
        for (path, bytes) in original {
            assert_eq!(must(fs::read(path)), bytes);
        }
    }
}

#[test]
fn enrolment_publication_recovery_preserves_fail_closed_secret_checks() {
    for mutation in [
        "contents",
        "permissions",
        "symlink",
        "generation",
        "closed",
        "request",
    ] {
        let directory = Directory::new();
        let (mut store, identity, publisher) = setup(&directory.0);
        let mut sessions = SessionRegistry::default();
        stop_publication(
            &mut store,
            &mut sessions,
            &identity,
            &publisher,
            PublicationStage::BearerPublished,
        );
        let root = directory.0.join("mcp");
        let bearer = root.join(DAEMON_BEARER_FILE);
        must(fs::remove_file(root.join(SESSION_TOKEN_FILE)));
        match mutation {
            "contents" => must(fs::write(&bearer, b"forged\n")),
            "permissions" => must(fs::set_permissions(
                &bearer,
                fs::Permissions::from_mode(0o644),
            )),
            "symlink" => {
                let target = directory.0.join("substituted");
                must(fs::rename(&bearer, &target));
                must(symlink(target, &bearer));
            }
            "generation" => {
                must(publisher.validate_existing(&sessions, &identity, &request()));
                let mut doc: Value = must(serde_json::from_slice(&must(fs::read(
                    publisher.binding_path(),
                ))));
                doc["session_generation"] = json!(token(&sessions).generation() + 1);
                must(fs::write(
                    publisher.binding_path(),
                    must(serde_json::to_vec(&doc)),
                ));
                must(fs::remove_file(root.join(SESSION_TOKEN_FILE)));
            }
            "closed" => must(session::close(
                &mut store,
                &mut sessions,
                &tenant(),
                SESSION,
            )),
            "request" => {}
            _ => unreachable!(),
        }
        drop(sessions);
        drop(store);
        let (_store, sessions) = reopen(&directory.0);
        let mut requested = request();
        if mutation == "request" {
            requested.policy_version = "forged-policy".to_owned();
        }
        assert!(
            publisher
                .validate_existing(&sessions, &identity, &requested)
                .is_err(),
            "{mutation}"
        );
        assert!(
            !root.join(SESSION_TOKEN_FILE).exists(),
            "no file repaired before all survivors and authority are validated"
        );
    }
}

// Loopback peer replays committed node producer transcripts; the real node client
// performs signature, metadata and commit verification before the agent stores them.
struct NodeTranscript {
    endpoint: SequencerEndpoint,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl NodeTranscript {
    fn open(case: &Value) -> Self {
        let case = case.clone();
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let endpoint = must(SequencerEndpoint::new(
            &format!("http://{}/", must(listener.local_addr())),
            &[],
        ));
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let worker = std::thread::spawn(move || {
            while !stopping.load(Ordering::Acquire) {
                let stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("{error}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(5))));
                must(stream.set_write_timeout(Some(Duration::from_secs(5))));
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
                let length = length.unwrap();
                assert!(length <= 65_536);
                let mut body = vec![0; length];
                must(reader.read_exact(&mut body));
                let request: Value = must(serde_json::from_slice(&body));
                let result = match request["method"].as_str().unwrap() {
                    "eth_chainId" => json!("0x7d"),
                    "pax_sequencerReceipt" => {
                        assert_eq!(request["params"], json!([case["receiptJson"]["txHash"]]));
                        case["receiptJson"].clone()
                    }
                    "eth_getTransactionReceipt" => case["landing"].clone(),
                    "px_getCommit" => {
                        assert_eq!(
                            request["params"],
                            json!([case["height"].as_u64().unwrap().to_string()])
                        );
                        case["material"].clone()
                    }
                    "px_getValidators" => case["validators"].clone(),
                    "eth_getBlockByNumber" => case["block"].clone(),
                    method => panic!("unexpected method {method}"),
                };
                let response =
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
                must(write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()));
                must(reader.get_mut().flush());
            }
        });
        Self {
            endpoint,
            stop,
            worker: Some(worker),
        }
    }
    fn lookup(
        &self,
        case: &Value,
        keys: &[ReceiptKeyEntry],
    ) -> Result<Lookup, layerx_client::rpc::receipt::ReceiptError> {
        lookup_authenticated(
            &self.endpoint,
            fixed(&case["receiptJson"]["txHash"]),
            LookupContext { receipt_keys: keys },
        )
    }
}
impl Drop for NodeTranscript {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        must(self.worker.take().unwrap().join());
    }
}
fn producer() -> Value {
    must(serde_json::from_str(include_str!(
        "../../layerx-agentd-host/src/repair_22_31_363_receipts.json"
    )))
}
fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    must(
        must(hex::decode(
            value.as_str().unwrap().trim_start_matches("0x"),
        ))
        .try_into(),
    )
}
fn pin(case: &Value) -> ReceiptKeyEntry {
    let key = &case["pinnedKey"];
    ReceiptKeyEntry {
        key_id: fixed(&key["keyId"]),
        public_key: fixed(&key["publicKey"]),
        first_height: key["firstHeight"].as_u64().unwrap(),
        last_height: key["lastHeight"].as_u64().unwrap(),
    }
}

#[test]
fn receipt_v2_unverified_ingress_authenticates_producer_transport_and_restart() {
    for outcome in ["success", "reverted"] {
        let case = producer()[outcome].clone();
        let peer = NodeTranscript::open(&case);
        let Lookup::Verified(verified) = must(peer.lookup(&case, &[pin(&case)])) else {
            panic!("verified receipt")
        };
        let capability = must(AuthenticatedTransactionReceipt::from_authenticated(
            &verified,
        ));
        let hash = verified.activity_id();
        let exact = verified.canonical_bytes();
        let directory = Directory::new();
        let mut store = must(Store::open(&directory.0));
        let expected = capability.evidence().clone();
        assert_eq!(expected.metadata.status, outcome == "success");
        assert_eq!(expected.metadata.gas_used, verified.execution().gas_used);
        assert_eq!(expected.metadata.block_height, verified.height());
        assert_eq!(expected.metadata.tx_index, verified.execution().index);
        assert_eq!(
            &expected.metadata.receipt_signature[..64],
            &verified
                .receipt()
                .protocol()
                .unwrap()
                .sequencer_signature()
                .unwrap()
        );
        assert_eq!(
            must(receipt::store_verified_if_absent(
                &mut store,
                tenant(),
                hash,
                exact,
                &capability
            )),
            expected
        );
        assert_eq!(
            must(receipt::store_verified_if_absent(
                &mut store,
                tenant(),
                hash,
                exact,
                &capability
            )),
            expected
        );
        assert!(receipt::store_verified_if_absent(
            &mut store,
            tenant(),
            [0xfe; 32],
            exact,
            &capability
        )
        .is_err());
        let mut changed_bytes = exact.to_vec();
        changed_bytes[0] ^= 1;
        assert!(receipt::store_verified_if_absent(
            &mut store,
            tenant(),
            hash,
            &changed_bytes,
            &capability
        )
        .is_err());
        drop(store);
        drop(peer);
        let store = must(Store::open(&directory.0));
        assert_eq!(
            must(receipt::serve_evidence(&store, tenant(), hash)),
            Some(expected)
        );
    }
}

#[test]
fn receipt_v2_unverified_ingress_rejects_raw_metadata_and_claimed_commit_before_indexes() {
    let case = producer()["success"].clone();
    let peer = NodeTranscript::open(&case);
    let Lookup::Verified(verified) = must(peer.lookup(&case, &[pin(&case)])) else {
        panic!("verified receipt")
    };
    let capability = must(AuthenticatedTransactionReceipt::from_authenticated(
        &verified,
    ));
    let directory = Directory::new();
    let mut store = must(Store::open(&directory.0));
    let before = fs::read(directory.0.join("store.bin")).ok();
    for field in [
        "unchanged",
        "height",
        "index",
        "status",
        "gas",
        "bloom",
        "signature",
        "committed",
        "hash",
        "bytes",
    ] {
        let mut raw = capability.evidence().clone();
        match field {
            "unchanged" => {}
            "height" => raw.metadata.block_height += 1,
            "index" => raw.metadata.tx_index += 1,
            "status" => raw.metadata.status = !raw.metadata.status,
            "gas" => raw.metadata.gas_used += 1,
            "bloom" => raw.metadata.logs_bloom[0] ^= 1,
            "signature" => raw.metadata.receipt_signature[0] ^= 1,
            "committed" => raw.metadata.rung = Rung::Committed,
            "hash" => raw.metadata.transaction_hash[0] ^= 1,
            "bytes" => raw.canonical_receipt[0] ^= 1,
            _ => unreachable!(),
        }
        assert!(
            matches!(
                receipt::store_verified_if_absent(
                    &mut store,
                    tenant(),
                    raw.metadata.transaction_hash,
                    &raw.canonical_receipt,
                    &raw
                ),
                Err(ReceiptStoreError::UnauthenticatedIngress)
            ),
            "{field}"
        );
        assert!(store
            .list_object_ids(&tenant(), ObjectKind::Receipt)
            .is_empty());
        assert!(store
            .list_object_ids(&tenant(), ObjectKind::Configuration)
            .is_empty());
        assert_eq!(fs::read(directory.0.join("store.bin")).ok(), before);
    }
    drop(store);
    let store = must(Store::open(&directory.0));
    assert_eq!(
        must(receipt::serve_evidence(
            &store,
            tenant(),
            verified.activity_id()
        )),
        None
    );
}

#[test]
fn receipt_v2_unverified_ingress_refuses_wire_forgery_and_unpinned_keys() {
    let original = producer()["success"].clone();
    for field in [
        "height",
        "index",
        "status",
        "gasUsed",
        "logs",
        "signature",
        "key",
        "key-range",
        "committed",
    ] {
        let directory = Directory::new();
        let mut store = must(Store::open(&directory.0));
        let mut case = original.clone();
        let mut key = pin(&case);
        match field {
            "height" => case["receiptJson"]["height"] = json!("0x3ea"),
            "index" => case["receiptJson"]["index"] = json!("0x2"),
            "status" => case["receiptJson"]["status"] = json!("0x0"),
            "gasUsed" => case["receiptJson"]["gasUsed"] = json!("0x1"),
            "logs" => {
                case["receiptJson"]["logs"][0]["address"] = json!(format!("0x{}", "ff".repeat(20)))
            }
            "signature" => {
                let mut bytes = must(hex::decode(
                    case["receiptJson"]["receipt"]
                        .as_str()
                        .unwrap()
                        .trim_start_matches("0x"),
                ));
                let last = bytes.len() - 1;
                bytes[last] ^= 1;
                case["receiptJson"]["receipt"] = json!(format!("0x{}", hex::encode(bytes)));
            }
            "key" => key.public_key[0] ^= 1,
            "key-range" => key.first_height = key.last_height,
            "committed" => {
                case["receiptJson"]["rung"] = json!("committed");
                case["receiptJson"]["blockNumber"] = json!("0x3ea");
                case["receiptJson"]["blockHash"] = json!(format!("0x{}", "aa".repeat(32)));
            }
            _ => unreachable!(),
        }
        let peer = NodeTranscript::open(&case);
        match peer.lookup(&case, &[key]) {
            Err(_) => assert_ne!(
                field, "committed",
                "a claimed commit should reach the capability's proof check"
            ),
            Ok(Lookup::Verified(receipt)) => {
                assert_eq!(field, "committed");
                assert!(matches!(
                    AuthenticatedTransactionReceipt::from_authenticated(&receipt),
                    Err(ReceiptStoreError::UnauthenticatedIngress)
                ));
                assert!(receipt::store_verified_if_absent(
                    &mut store,
                    tenant(),
                    receipt.activity_id(),
                    receipt.canonical_bytes(),
                    &receipt
                )
                .is_err());
            }
            other => panic!("forged evidence accepted: {field}: {other:?}"),
        }
        assert!(store
            .list_object_ids(&tenant(), ObjectKind::Receipt)
            .is_empty());
        assert!(store
            .list_object_ids(&tenant(), ObjectKind::Configuration)
            .is_empty());
    }
}

fn authorization() -> RequestContext {
    RequestContext {
        surface: Surface::Mcp,
        operation: Operation::ReadBalance,
        core_sequence: 60,
        supplied_header_tenant: None,
        supplied_body_tenant: None,
        target_owner: None,
    }
}
fn live_token(root: &Path) -> (Store, SessionRegistry, Token) {
    let (mut store, identity, publisher) = setup(root);
    let mut sessions = SessionRegistry::default();
    must(enrolment::enrol(
        &mut store,
        &mut sessions,
        &identity,
        request(),
        &publisher,
    ));
    let token = token(&sessions);
    (store, sessions, token)
}

#[test]
fn tenant_observability_growth_bounds_sustained_allowed_and_refused_authorizations() {
    let directory = Directory::new();
    let (_store, sessions, token) = live_token(&directory.0);
    let mut observability = TenantObservability::default();
    let allowed = authorization();
    let mut refused = authorization();
    refused.supplied_body_tenant = Some(must(TenantId::new("foreign")));
    for i in 0..20_000 {
        if i % 2 == 0 {
            must(tenant::resolve(
                &token,
                &sessions,
                &allowed,
                &mut observability,
            ));
        } else {
            assert!(tenant::resolve(&token, &sessions, &refused, &mut observability).is_err());
        }
        assert!(observability.audit().len() <= TenantObservability::TENANT_RETAINED_LIMIT);
        assert!(observability.traces().len() <= TenantObservability::TENANT_RETAINED_LIMIT);
    }
    assert_eq!(
        observability.audit().len(),
        TenantObservability::TENANT_RETAINED_LIMIT
    );
    assert_eq!(
        observability.overflowed(),
        20_000 - TenantObservability::TENANT_RETAINED_LIMIT as u64
    );
    assert_eq!(observability.metrics().values().sum::<u64>(), 20_000);
    assert_eq!(observability.metrics().len(), 2);
    assert!(observability
        .audit()
        .iter()
        .any(|entry| entry.outcome == AuthorizationOutcome::Allowed));
    assert!(observability
        .audit()
        .iter()
        .any(|entry| entry.outcome != AuthorizationOutcome::Allowed));
    must(observability.flush_to(&directory.0));
    assert!(observability.audit().is_empty());
    assert!(observability.traces().is_empty());
    let count = TenantObservability::TENANT_RETAINED_LIMIT as u64;
    assert_eq!(observability.delivered(), (count, count));
    drop(observability);
    let audit = must(Log::open(
        directory.0.join("tenant-authorization"),
        &tenant(),
    ));
    let entries = must(audit::read_entries(audit.path()));
    assert_eq!(entries.len() as u64, count);
    assert_eq!(
        entries
            .iter()
            .filter(|entry| entry.decision == Decision::Allowed)
            .count(),
        entries.len() / 2
    );
    assert_eq!(
        entries
            .iter()
            .filter(|entry| entry.decision == Decision::Refused)
            .count(),
        entries.len() / 2
    );
    let traces = must(Log::open(directory.0.join("tenant-traces"), &tenant()));
    assert_eq!(must(audit::verify_chain(traces.path())).entries, count);
}

#[test]
fn tenant_observability_growth_accounts_for_sink_failure_and_recovers_delivery() {
    let directory = Directory::new();
    let (_store, sessions, token) = live_token(&directory.0);
    let sinks = directory.0.join("sinks");
    must(fs::create_dir(&sinks));
    // Audit delivery succeeds while trace delivery is unavailable. Retry must
    // dequeue each sink separately, without replaying already delivered audits.
    let blocked = sinks.join("tenant-traces");
    must(fs::write(&blocked, b"unavailable"));
    let mut observability = TenantObservability::with_durable_sinks(sinks.clone());
    for _ in 0..300 {
        must(tenant::resolve(
            &token,
            &sessions,
            &authorization(),
            &mut observability,
        ));
    }
    let count = TenantObservability::TENANT_RETAINED_LIMIT as u64;
    assert_eq!(observability.audit().len(), 0);
    assert_eq!(observability.traces().len() as u64, count);
    assert_eq!(observability.delivered(), (count, 0));
    assert_eq!(observability.overflowed(), 300 - count);
    assert!(observability.delivery_failures() >= 300);
    must(fs::remove_file(blocked));
    must(tenant::resolve(
        &token,
        &sessions,
        &authorization(),
        &mut observability,
    ));
    assert_eq!(observability.delivered(), (count + 1, count + 1));
    assert!(observability.audit().is_empty());
    assert!(observability.traces().is_empty());
    drop(observability);
    assert_eq!(
        must(Log::open(sinks.join("tenant-authorization"), &tenant())).entries(),
        count + 1
    );
    assert_eq!(
        must(Log::open(sinks.join("tenant-traces"), &tenant())).entries(),
        count + 1
    );
}

#[test]
fn tenant_observability_growth_caps_tenant_cardinality_and_total_queues() {
    let directory = Directory::new();
    let mut store = must(Store::open(&directory.0));
    let mut sessions = SessionRegistry::default();
    let mut observability = TenantObservability::default();
    let mut retained = 0_u64;
    for i in 0..(TenantObservability::TENANT_LIMIT + 10) {
        let owner = must(TenantId::new(format!("tenant-{i}")));
        let record = identity(&mut store, owner.clone());
        let token = must(session::open(
            &mut store,
            &mut sessions,
            &record,
            session::OpenRequest {
                session_id: SESSION,
                token_id: [0x67; 32],
                tenant: owner,
                agent: record.did().clone(),
                authority: ProtocolAuthority::CapabilityGrant(CAPABILITY.0),
                permitted_activity_types: BTreeSet::from([7]),
                scopes: request().scopes,
                expiry_sequence: 300,
                expiry_seconds: None,
                opening_client: "mcp".to_owned(),
                policy_version: "policy-v1".to_owned(),
            },
            50,
        ));
        for _ in 0..TenantObservability::TENANT_RETAINED_LIMIT {
            must(tenant::resolve(
                &token,
                &sessions,
                &authorization(),
                &mut observability,
            ));
        }
        retained += TenantObservability::TENANT_RETAINED_LIMIT as u64;
        assert!(observability.audit().len() <= TenantObservability::RETAINED_LIMIT);
        assert!(observability.traces().len() <= TenantObservability::RETAINED_LIMIT);
        assert!(observability.metrics().len() <= TenantObservability::TENANT_LIMIT);
    }
    assert_eq!(
        observability.audit().len(),
        TenantObservability::RETAINED_LIMIT
    );
    assert_eq!(
        observability.metrics().len(),
        TenantObservability::TENANT_LIMIT
    );
    assert_eq!(
        observability.overflowed(),
        retained - TenantObservability::RETAINED_LIMIT as u64
    );
}

#[test]
fn tenant_observability_growth_session_control_wires_durable_sinks() {
    let directory = Directory::new();
    let (store, sessions, token) = live_token(&directory.0);
    let control = SessionControl::new(
        Arc::new(Mutex::new(store)),
        sessions,
        Arc::new(PreparationLifecycle::default()),
        Arc::new(must(BudgetLimiter::new(vec![]))),
    );
    for _ in 0..8 {
        must(control.authorize(
            &token.credential(),
            Operation::ReadBalance,
            Surface::Mcp,
            60,
            None,
        ));
    }
    let stats = must(control.observability_stats());
    assert_eq!(stats.queued_audits, 0);
    assert_eq!(stats.queued_traces, 0);
    assert_eq!(stats.audits_delivered, 8);
    assert_eq!(stats.traces_delivered, 8);
    assert_eq!(stats.overflowed, 0);
    assert_eq!(stats.delivery_failures, 0);
    drop(control);
    let root = directory.0.join("store");
    let log = must(Log::open(root.join("tenant-authorization"), &tenant()));
    assert_eq!(must(audit::read_entries(log.path())).len(), 8);
    assert_eq!(
        must(Log::open(root.join("tenant-traces"), &tenant())).entries(),
        8
    );
}

#[test]
fn receipt_v2_unverified_ingress_requires_real_committed_block_and_sequence_coverage() {
    use layerx_client::rpc::commit::decode_validators;
    use layerx_client::rpc::receipt::lookup_authenticated_with_commit;
    use layerx_client::rpc::receipt_commit::ReceiptCommitEvidence;
    use layerx_programs::chain_proof::{merkle::SimpleProof, ChainProofVerifier};
    let recording: Value = must(serde_json::from_str(include_str!(
        "../../layerx-client/tests/fixtures/repair_22_33_1546_node.json"
    )));
    let mut case = recording["receipt"].clone();
    case["receiptJson"] = case["view"].clone();
    case["pinnedKey"] = case["key"].clone();
    let trust = ChainProofVerifier {
        chain_id: case["material"]["signedHeader"]["header"]["chain_id"]
            .as_str()
            .unwrap()
            .to_owned(),
        validators: must(decode_validators(&case["validators"])),
    };
    let batch = must(hex::decode(
        case["batch"].as_str().unwrap().trim_start_matches("0x"),
    ));
    let ids: Vec<[u8; 32]> = case["activityIds"]
        .as_array()
        .unwrap()
        .iter()
        .map(fixed)
        .collect();
    let proof = SimpleProof {
        total: must(case["proof"]["total"].as_str().unwrap().parse()),
        index: must(case["proof"]["index"].as_str().unwrap().parse()),
        leaf_hash: must(
            must(openssl::base64::decode_block(
                case["proof"]["leaf_hash"].as_str().unwrap(),
            ))
            .try_into(),
        ),
        aunts: case["proof"]["aunts"]
            .as_array()
            .unwrap()
            .iter()
            .map(|aunt| {
                must(must(openssl::base64::decode_block(aunt.as_str().unwrap())).try_into())
            })
            .collect(),
    };
    let evidence = ReceiptCommitEvidence {
        verifier: &trust,
        covering_height: case["height"].as_u64().unwrap(),
        batch_transaction: &batch,
        transaction_proof: &proof,
        activity_ids: &ids,
    };
    let keys = [pin(&case)];
    let hash = fixed(&case["receiptJson"]["txHash"]);
    let directory = Directory::new();
    let mut store = must(Store::open(&directory.0));
    let peer = NodeTranscript::open(&case);
    let Lookup::Verified(receipted) = must(peer.lookup(&case, &keys)) else {
        panic!("verified receipt")
    };
    assert_eq!(receipted.rung(), Rung::Receipted);
    let initial = must(AuthenticatedTransactionReceipt::from_authenticated(
        &receipted,
    ));
    must(receipt::store_verified_if_absent(
        &mut store,
        tenant(),
        hash,
        receipted.canonical_bytes(),
        &initial,
    ));
    let Lookup::Verified(committed) = must(lookup_authenticated_with_commit(
        &peer.endpoint,
        hash,
        LookupContext {
            receipt_keys: &keys,
        },
        &evidence,
    )) else {
        panic!("verified committed receipt")
    };
    assert_eq!(committed.rung(), Rung::Committed);
    assert!(committed.committed_header().is_some());
    let authenticated = must(AuthenticatedTransactionReceipt::from_authenticated(
        &committed,
    ));
    let expected = must(receipt::store_verified_if_absent(
        &mut store,
        tenant(),
        hash,
        committed.canonical_bytes(),
        &authenticated,
    ));
    assert_eq!(expected.metadata.rung, Rung::Committed);
    assert_eq!(
        must(receipt::store_verified_if_absent(
            &mut store,
            tenant(),
            hash,
            receipted.canonical_bytes(),
            &initial
        )),
        expected
    );
    for damage in ["commit-signature", "block-hash", "activity-coverage"] {
        let mut altered = case.clone();
        let mut activity_ids = ids.clone();
        match damage {
            "commit-signature" => {
                altered["material"]["signedHeader"]["commit"]["signatures"][0]["signature"] =
                    json!(openssl::base64::encode_block(&[0; 64]))
            }
            "block-hash" => altered["block"]["hash"] = json!(format!("0x{}", "ff".repeat(32))),
            "activity-coverage" => activity_ids[0][0] ^= 1,
            _ => unreachable!(),
        }
        let peer = NodeTranscript::open(&altered);
        let altered_evidence = ReceiptCommitEvidence {
            verifier: &trust,
            covering_height: evidence.covering_height,
            batch_transaction: &batch,
            transaction_proof: &proof,
            activity_ids: &activity_ids,
        };
        let before = must(fs::read(directory.0.join("store.bin")));
        assert!(
            lookup_authenticated_with_commit(
                &peer.endpoint,
                hash,
                LookupContext {
                    receipt_keys: &keys
                },
                &altered_evidence
            )
            .is_err(),
            "{damage}"
        );
        assert_eq!(must(fs::read(directory.0.join("store.bin"))), before);
    }
    drop(store);
    let store = must(Store::open(&directory.0));
    assert_eq!(
        must(receipt::serve_evidence(&store, tenant(), hash)),
        Some(expected)
    );
}
