//! Repair record: src/repair_22_31_364.kvx maps findings to these real-path tests.
//! Gate revisions and private evidence locations are supplied by hf-build/hf-done.

use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex, OnceLock};
use std::thread::JoinHandle;
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::admin::{
    ActionPlan, AdminError, OperatorCommand, OperatorContext, ProtectedMutation,
    Surface as OperatorSurface,
};
use layerx_agentd::audit::{
    self, Coverage, Decision, Entry, EventClass, EvidenceStore, Log, Query, Redacted,
};
use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope};
use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
use layerx_agentd::enrolment::{
    self, BindingMode, BindingPublisher, DaemonSurface, EnrolmentRequest, WebDeclaration,
};
use layerx_agentd::identity::{
    self, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::outbox::Outbox;
use layerx_agentd::prepare::PreparationLifecycle;
use layerx_agentd::protocol_evidence::RawReceiptEvidence;
use layerx_agentd::receipt::{
    self, ReceiptLookupKey, ReceiptStoreError, NODE_ED25519_SIGNATURE_TAG,
};
use layerx_agentd::session::{SessionCredential, SessionId, SessionRegistry};
use layerx_agentd::session_control::SessionControl;
use layerx_agentd::store::{ObjectKind, StorageClass, Store, TenantId, TenantKey};
use layerx_agentd::tenant::{Config, Operation, RedactionPolicy, Retention, Surface};
use layerx_client::rpc::receipt::{
    lookup_authenticated, AuthenticatedReceipt, Lookup, LookupContext,
};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_proof::merkle::build_proof;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::ids::Did;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::batch_header_digest;
use serde_json::{json, Value};

static NEXT: AtomicU64 = AtomicU64::new(1);

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let root = must(fs::canonicalize(std::env::temp_dir())).join(format!(
            "repair-364-{}-{}",
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
    must(TenantId::new("repair-364"))
}
fn other_tenant() -> TenantId {
    must(TenantId::new("another-tenant"))
}
fn producer() -> &'static Value {
    static PRODUCER: OnceLock<Value> = OnceLock::new();
    PRODUCER.get_or_init(|| {
        must(serde_json::from_str(include_str!(
            "../../layerx-agentd-host/src/repair_22_31_363_receipts.json"
        )))
    })
}
fn unhex(value: &Value) -> Vec<u8> {
    let text = value.as_str().unwrap_or_else(|| panic!("hex string"));
    must(hex::decode(text.strip_prefix("0x").unwrap_or(text)))
}
fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    must(unhex(value).try_into())
}
fn pin(case: &Value) -> ReceiptKeyEntry {
    let key = &case["pinnedKey"];
    ReceiptKeyEntry {
        key_id: fixed(&key["keyId"]),
        public_key: fixed(&key["publicKey"]),
        first_height: key["firstHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("height")),
        last_height: key["lastHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("height")),
    }
}

// The node peer replays committed producer transcripts. All receipt authentication,
// outbox matching, persistence and recovery below run the production components.
struct NodeTranscript {
    url: String,
    stop: Arc<AtomicBool>,
    requests: Arc<Mutex<Vec<Value>>>,
    worker: Option<JoinHandle<()>>,
}
impl NodeTranscript {
    fn open(case: &Value) -> Self {
        let case = case.clone();
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let url = format!("http://{}/", must(listener.local_addr()));
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let requests = Arc::new(Mutex::new(Vec::new()));
        let recorded = Arc::clone(&requests);
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
                let length = length.unwrap_or_else(|| panic!("content length"));
                assert!(length <= 65_536);
                let mut body = vec![0; length];
                must(reader.read_exact(&mut body));
                let request: Value = must(serde_json::from_slice(&body));
                assert_eq!(request["jsonrpc"], "2.0");
                let result = match request["method"]
                    .as_str()
                    .unwrap_or_else(|| panic!("method"))
                {
                    "eth_chainId" => json!("0x7d"),
                    "px_getReceiptKeys" => json!({"keys": [case["pinnedKey"].clone()]}),
                    "pax_sequencerReceipt" => {
                        assert_eq!(request["params"], json!([case["receiptJson"]["txHash"]]));
                        case["receiptJson"].clone()
                    }
                    "eth_getTransactionReceipt" => {
                        assert_eq!(request["params"], json!([case["receiptJson"]["txHash"]]));
                        Value::Null
                    }
                    method => panic!("unexpected node method {method}"),
                };
                let response =
                    json!({"jsonrpc":"2.0", "id": request["id"], "result": result}).to_string();
                must(write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()));
                must(reader.get_mut().flush());
                must(recorded.lock()).push(request);
            }
        });
        Self {
            url,
            stop,
            requests,
            worker: Some(worker),
        }
    }
    fn endpoint(&self) -> SequencerEndpoint {
        must(SequencerEndpoint::new(&self.url, &[]))
    }
    fn authenticated(&self, case: &Value) -> AuthenticatedReceipt {
        let keys = [pin(case)];
        let Lookup::Verified(receipt) = must(lookup_authenticated(
            &self.endpoint(),
            fixed(&case["receiptJson"]["txHash"]),
            LookupContext {
                receipt_keys: &keys,
            },
        )) else {
            panic!("authenticated node receipt")
        };
        receipt
    }
}
impl Drop for NodeTranscript {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            must(worker.join());
        }
    }
}

fn audit_config() -> Config {
    Config {
        tenant: tenant(),
        policy_version: "policy-v1".to_owned(),
        redaction: RedactionPolicy::Standard,
        retention: Retention {
            events: 100,
            audit: 100,
            receipts: 100,
        },
        verification_default: VerificationLevel::STATE_PROVEN,
        approval_required_for: BTreeSet::new(),
    }
}

#[test]
fn operator_audit_payload_mixed_events_export_and_review_after_restart() {
    let directory = Directory::new();
    let config = audit_config();
    let ordinary = Entry {
        class: EventClass::Authentication,
        observed_at_ms: 10,
        tenant: tenant(),
        agent: must(Did::new(b"did:layerx:ordinary")),
        session: None,
        capability: None,
        policy_version: "policy-v1".to_owned(),
        request_id: [1; 32],
        idempotency_key: None,
        decision: Decision::Allowed,
        reason: Redacted::boot_diagnostic("session authenticated"),
        resulting_activity_id: None,
        verification_level: VerificationLevel::UNVERIFIED,
        protocol_authority: None,
        submitted_bytes: None,
        receipt_id: None,
        tx_hash: None,
        nonce: None,
        rung: None,
    };
    let mut log = must(Log::open(&directory.0, &tenant()));
    must(audit::record(
        &mut log,
        &mut Coverage::default(),
        &ordinary,
        || (),
    ));
    let path = log.path().to_owned();
    drop(log);
    let context = must(OperatorContext::new(
        "operator-with-a-real-request",
        [2; 32],
    ));
    let mut operator = must(OperatorSurface::open(&directory.0, &tenant()));
    assert_eq!(operator.audit_path(), path);
    assert_eq!(
        must(operator.dispatch(&context, OperatorCommand::InspectUnknown([3; 32]))),
        ActionPlan::InspectOnly
    );
    assert!(matches!(
        operator.dispatch(
            &context,
            OperatorCommand::AttemptProtectedMutation {
                target: [4; 32],
                mutation: ProtectedMutation::ReplaceReceipt,
            }
        ),
        Err(AdminError::ProtectedMutation(
            ProtectedMutation::ReplaceReceipt
        ))
    ));
    assert_eq!(operator.audit_entries(), 3);
    drop(operator);
    let mut log = must(Log::open(&directory.0, &tenant()));
    must(audit::record(
        &mut log,
        &mut Coverage::default(),
        &ordinary,
        || (),
    ));
    drop(log);
    let reopened = must(OperatorSurface::open(&directory.0, &tenant()));
    assert_eq!(reopened.audit_entries(), 4);
    let entries = must(audit::read_entries(reopened.audit_path()));
    assert_eq!(entries[0], ordinary);
    assert_eq!(entries[3], ordinary);
    for (index, class, decision, target) in [
        (
            1,
            EventClass::AdministrativeAction,
            Decision::Requested,
            [3; 32],
        ),
        (2, EventClass::MutationAttempt, Decision::Refused, [4; 32]),
    ] {
        let entry = &entries[index];
        assert_eq!(entry.class, class);
        assert_eq!(entry.decision, decision);
        assert_eq!(entry.tenant, tenant());
        assert_eq!(entry.request_id, context.request_id);
        assert_eq!(entry.policy_version, "operator-attempt-v1");
        let reason: Value = must(serde_json::from_str(entry.reason.as_str()));
        assert_eq!(reason["version"], 1);
        assert_eq!(reason["operator_id"], context.operator_id);
        assert_eq!(reason["target"], hex::encode(target));
        assert!(entry.protocol_authority.is_none());
    }
    let exported = must(audit::export(
        &path,
        &config,
        Query {
            tenant: tenant(),
            agent: None,
            from_observed_at_ms: None,
            through_observed_at_ms: None,
        },
        &EvidenceStore::new(tenant()),
    ));
    let reviewed = must(audit::review(&exported));
    assert_eq!(reviewed.exported_entries, 4);
    assert_eq!(
        exported
            .entries
            .iter()
            .map(|item| item.entry.clone())
            .collect::<Vec<_>>(),
        entries
    );
    assert_eq!(reviewed.failed_records, 0);
}

// Committed identity records are a peer of enrolment, not a replacement for the publisher.
struct IdentityRecords(BTreeMap<Did, CoreIdentity>);
impl IdentityResolver for IdentityRecords {
    fn resolve(&mut self, did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(self.0.get(did).cloned())
    }
}
fn limit() -> LimitConfig {
    LimitConfig {
        id: LimitId([1; 16]),
        name: "mcp".to_owned(),
        scope: LimitScope::Tenant([1; 32]),
        ceiling: 100,
        consumed: 0,
    }
}
fn enrolment_records(store: &mut Store) -> (identity::IdentityRecord, EnrolmentRequest) {
    let capability_id = CapabilityId([9; 32]);
    must(
        must(Capability::new(
            capability_id,
            tenant(),
            CapabilityDimensions {
                activity_types: BTreeSet::from([7]),
                counterparties: BTreeSet::from([[2; 32]]),
                assets: BTreeSet::from([[3; 32]]),
                amount_ceiling: 100,
                rate_ceiling: RateCeiling {
                    maximum_uses: 10,
                    window_sequences: 10,
                },
                purposes: BTreeSet::from(["service-payment".to_owned()]),
                expiry_sequence: 400,
            },
        ))
        .persist(store),
    );
    let did = must(Did::new(b"did:layerx:web-agent"));
    let mut identities = IdentityRecords(BTreeMap::from([(
        did.clone(),
        CoreIdentity {
            canonical_bytes: b"recorded-web-identity".to_vec(),
            head_sequence: 10,
            revocation_sequence: 1,
            verification_level: VerificationLevel::STATE_PROVEN,
            frozen: false,
            authorities: vec![ProtocolAuthority::CapabilityGrant(capability_id.0)],
        },
    )]));
    let identity = must(identity::register(store, tenant(), did, &mut identities));
    let request = EnrolmentRequest {
        session_id: SessionId([7; 32]),
        capability_id,
        permitted_activity_types: BTreeSet::from([7]),
        scopes: BTreeSet::from(["read:balance".to_owned(), "web.search".to_owned()]),
        expiry_sequence: 300,
        opening_client: "mcp".to_owned(),
        policy_version: "policy-v1".to_owned(),
        core_sequence: 50,
    };
    (identity, request)
}

#[test]
fn web_binding_schema_publishes_addresses_keys_and_recovers_session() {
    let directory = Directory::new();
    let case = &producer()["success"];
    let node = NodeTranscript::open(case);
    let mut native = layerx_agentd::read::NativeReadRoute::new(node.endpoint(), 125);
    let keys = must(native.signed_authority());
    assert_eq!(keys, vec![pin(case)]);
    let raw = unhex(&case["rawTransaction"]);
    let (transaction, signature) = must(decode_signed(&raw));
    let payer = must(recover_sender(&transaction, &signature));
    let receiver = fixed(&case["receiptJson"]["to"]);
    let web = must(WebDeclaration::new(
        "http://127.0.0.1:4000".to_owned(),
        "eip155:125".to_owned(),
        payer,
        receiver,
        keys.clone(),
        0,
        5_000,
        3,
        20,
    ));
    let publisher = must(BindingPublisher::new(
        directory.0.join("binding"),
        directory.0.join("store"),
        directory.0.join("audit"),
        must(DaemonSurface::new(
            "127.0.0.1:4001",
            "b".repeat(48),
            [8; 32],
        )),
        limit(),
        Duration::from_secs(5),
        BindingMode::Full,
    ))
    .with_web(web);
    let mut store = must(Store::open(directory.0.join("store")));
    let (identity, request) = enrolment_records(&mut store);
    let mut sessions = SessionRegistry::default();
    let published = must(enrolment::enrol(
        &mut store,
        &mut sessions,
        &identity,
        request.clone(),
        &publisher,
    ));
    let original = must(fs::read(&published.binding));
    let document: enrolment::binding_shape::BindingDocument =
        must(serde_json::from_slice(&original));
    let web = document
        .web
        .unwrap_or_else(|| panic!("published web binding"));
    assert_eq!(
        web.as_object()
            .unwrap_or_else(|| panic!("web object"))
            .keys()
            .map(String::as_str)
            .collect::<BTreeSet<_>>(),
        BTreeSet::from([
            "endpoint",
            "payer_address",
            "receiver_address",
            "network",
            "receipt_key_index",
            "sequencer_id",
            "first_batch_number",
            "last_batch_number",
            "timeout_ms",
            "pending_attempts",
            "approval_threshold"
        ])
    );
    assert_eq!(web["network"], "eip155:125");
    assert_eq!(web["payer_address"], format!("0x{}", hex::encode(payer)));
    assert_eq!(
        web["receiver_address"],
        format!("0x{}", hex::encode(receiver))
    );
    assert_eq!(
        web["receipt_key_index"],
        json!([{
            "keyId": hex::encode(keys[0].key_id), "publicKey": hex::encode(keys[0].public_key),
            "firstHeight": keys[0].first_height, "lastHeight": keys[0].last_height,
        }])
    );
    assert_eq!(web["sequencer_id"], hex::encode(keys[0].key_id));
    assert_eq!(web["first_batch_number"], keys[0].first_height);
    assert_eq!(web["last_batch_number"], keys[0].last_height);
    assert_eq!(web["timeout_ms"], 5_000);
    assert_eq!(web["pending_attempts"], 3);
    assert_eq!(web["approval_threshold"], "20");
    drop(sessions);
    drop(store);
    let store = must(Store::open(directory.0.join("store")));
    let mut sessions = SessionRegistry::default();
    must(sessions.restore_tenant(&store, &tenant()));
    must(publisher.validate_existing(&sessions, &identity, &request));
    assert_eq!(must(fs::read(&published.binding)), original);
    let token = must(fs::read_to_string(&published.session_token_file));
    let credential = SessionCredential::new(
        tenant(),
        published.session_id,
        must(must(hex::decode(token.trim())).try_into()),
        published.session_generation,
    );
    let control = SessionControl::new(
        Arc::new(Mutex::new(store)),
        sessions,
        Arc::new(PreparationLifecycle::default()),
        Arc::new(must(BudgetLimiter::new(vec![limit()]))),
    );
    let permit =
        must(control.authorize(&credential, Operation::ReadBalance, Surface::Mcp, 51, None));
    must(permit.boundary(&control));
    let requests = must(node.requests.lock());
    assert!(requests
        .iter()
        .any(|request| request["method"] == "px_getReceiptKeys"));
}

#[test]
fn web_binding_schema_refuses_retired_network_and_invalid_governance() {
    let key = pin(&producer()["success"]);
    for (network, payer, receiver, keys, selection, timeout, attempts) in [
        ("layerx:125", [1; 20], [2; 20], vec![key], 0, 1, 1),
        ("eip155:1", [1; 20], [2; 20], vec![key], 0, 1, 1),
        ("eip155:125", [0; 20], [2; 20], vec![key], 0, 1, 1),
        ("eip155:125", [1; 20], [0; 20], vec![key], 0, 1, 1),
        ("eip155:125", [1; 20], [2; 20], vec![key, key], 0, 1, 1),
        ("eip155:125", [1; 20], [2; 20], vec![], 0, 1, 1),
        ("eip155:125", [1; 20], [2; 20], vec![key], 1, 1, 1),
        (
            "eip155:125",
            [1; 20],
            [2; 20],
            vec![ReceiptKeyEntry {
                first_height: key.last_height + 1,
                ..key
            }],
            0,
            1,
            1,
        ),
        ("eip155:125", [1; 20], [2; 20], vec![key], 0, 0, 1),
        ("eip155:125", [1; 20], [2; 20], vec![key], 0, 60_001, 1),
        ("eip155:125", [1; 20], [2; 20], vec![key], 0, 1, 0),
        ("eip155:125", [1; 20], [2; 20], vec![key], 0, 1, 11),
    ] {
        assert!(WebDeclaration::new(
            "http://127.0.0.1:4000".to_owned(),
            network.to_owned(),
            payer,
            receiver,
            keys,
            selection,
            timeout,
            attempts,
            20
        )
        .is_err());
    }
}

fn raw_evidence(receipt: &AuthenticatedReceipt) -> RawReceiptEvidence {
    let canonical = receipt.canonical_bytes();
    let (proof, root) = must(build_proof(&[canonical], 0));
    let signer = SigningKey::from_bytes(&[31; 32]);
    let key = signer.verifying_key().to_bytes();
    let mut header = Encoder::new(4096);
    must(header.structure_header_version(0x1701, layerx_wire::limits::PROTOCOL_VERSION));
    must(header.u8(15));
    for field in 1..=15 {
        must(header.tag(field, 15));
        match field {
            1 => must(header.u16(layerx_wire::limits::PROTOCOL_VERSION)),
            2 => must(header.u32(125)),
            3 => must(header.u64(1)),
            4 => must(header.u64(receipt.height())),
            5 | 6 => must(header.u64(receipt.global_sequence())),
            8 => must(header.bytes(&root, 32)),
            14 => must(header.u64(receipt.execution().timestamp)),
            15 => must(header.bytes(&key, 32)),
            _ => must(header.bytes(&[1; 32], 32)),
        }
    }
    let header = header.finish();
    let signature = signer.sign(&must(batch_header_digest(&header))).to_bytes();
    RawReceiptEvidence::new(canonical.to_vec(), proof, header, signature)
}

fn store_legacy(store: &mut Store, authenticated: &AuthenticatedReceipt, case: &Value) {
    let protocol = authenticated
        .receipt()
        .protocol()
        .unwrap_or_else(|| panic!("protocol"));
    let authority = AuthorizedBatch::new(
        protocol.batch_id(),
        protocol.asset(),
        protocol.previous_state_root(),
        protocol.resulting_state_root(),
        pin(case).public_key,
    );
    must(receipt::store(
        store,
        tenant(),
        authenticated.activity_id(),
        authenticated.canonical_bytes(),
        &authority,
    ));
}

#[test]
fn receipt_legacy_evidence_roundtrip_reopens_and_explicitly_migrates() {
    let directory = Directory::new();
    // The original node transfer transcript charges fees in the payment asset,
    // so its signed balance changes also satisfy the legacy outcome verifier.
    let original: Value = must(serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )));
    let case = original["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("committed node cases"))
        .iter()
        .find(|case| case["id"] == "V1-pax-transfer")
        .unwrap_or_else(|| panic!("committed transfer receipt"));
    let node = NodeTranscript::open(case);
    let authenticated = node.authenticated(case);
    let hash = authenticated.activity_id();
    let mut store = must(Store::open(&directory.0));
    store_legacy(&mut store, &authenticated, case);
    let raw = raw_evidence(&authenticated);
    let persisted = must(receipt::persist_evidence(&mut store, tenant(), hash, &raw));
    assert_eq!(
        must(receipt::persist_evidence(&mut store, tenant(), hash, &raw)),
        persisted
    );
    drop(store);
    let mut store = must(Store::open(&directory.0));
    let inventory = must(receipt::evidence_inventory(&store, &tenant()));
    assert_eq!(inventory.with_evidence, vec![persisted.clone()]);
    assert!(inventory.without_evidence.is_empty());
    let refusal = receipt::serve_evidence(&store, tenant(), hash).unwrap_err();
    assert_eq!(refusal.reason(), "receipt.evidence_version");
    let before = must(fs::read(directory.0.join("store.bin")));
    assert!(receipt::store_verified_if_absent(
        &mut store,
        tenant(),
        hash,
        authenticated.canonical_bytes(),
        &authenticated
    )
    .is_err());
    assert_eq!(must(fs::read(directory.0.join("store.bin"))), before);
    let served = must(receipt::migrate_authenticated_evidence(
        &mut store,
        tenant(),
        hash,
        &authenticated,
    ));
    assert_eq!(served.canonical_bytes, authenticated.canonical_bytes());
    let v2 = must(receipt::serve_evidence(&store, tenant(), hash))
        .unwrap_or_else(|| panic!("migrated evidence"));
    assert_eq!(v2.canonical_receipt, authenticated.canonical_bytes());
    assert_eq!(v2.metadata.transaction_hash, hash);
    assert_eq!(
        must(receipt::evidence_inventory(&store, &tenant())).with_evidence,
        vec![persisted.clone()]
    );
    drop(store);
    let store = must(Store::open(&directory.0));
    assert_eq!(
        must(receipt::serve_evidence(&store, tenant(), hash)),
        Some(v2)
    );
    assert_eq!(
        must(receipt::evidence_inventory(&store, &tenant())).with_evidence,
        vec![persisted]
    );
    assert_eq!(
        must(receipt::serve_evidence(&store, other_tenant(), hash)),
        None
    );
}

#[test]
fn receipt_v2_producer_not_wired_live_observation_serves_after_restart() {
    for name in ["success", "reverted"] {
        let directory = Directory::new();
        let case = &producer()[name];
        let node = NodeTranscript::open(case);
        let chain = must(NodeRpc::connect(&NodeRpcConfig {
            url: node.url.clone(),
            chain_id: 125,
            deadline: Duration::from_secs(5),
        }));
        let raw = unhex(&case["rawTransaction"]);
        let hash = transaction_hash(&raw);
        let mut store = must(Store::open(&directory.0));
        let mut outbox = Outbox::default();
        assert_eq!(must(outbox.enqueue(&mut store, tenant(), raw)), hash);
        let (lookup, keys) = must(
            layerx_agentd::human_runtime::lookup_human_receipt_by_outbox_hash(
                &chain,
                &node.endpoint(),
                &outbox,
                hash,
                hash,
            ),
        );
        assert_eq!(keys, vec![pin(case)]);
        let Lookup::Verified(authenticated) = lookup else {
            panic!("live authenticated receipt")
        };
        let served = must(receipt::store_verified_if_absent(
            &mut store,
            tenant(),
            hash,
            authenticated.canonical_bytes(),
            &authenticated,
        ));
        let evidence = must(receipt::serve_evidence(&store, tenant(), hash))
            .unwrap_or_else(|| panic!("live ingress omitted v2 evidence"));
        assert_eq!(evidence.canonical_receipt, authenticated.canonical_bytes());
        assert_eq!(evidence.metadata.transaction_hash, hash);
        assert_eq!(
            evidence.metadata.block_height,
            authenticated.execution().height
        );
        assert_eq!(evidence.metadata.tx_index, authenticated.execution().index);
        assert_eq!(evidence.metadata.status, name == "success");
        assert_eq!(
            evidence.metadata.gas_used,
            authenticated.execution().gas_used
        );
        assert_eq!(evidence.metadata.rung, Rung::Receipted);
        assert_eq!(
            evidence.metadata.receipt_signature[..64],
            authenticated
                .receipt()
                .protocol()
                .unwrap_or_else(|| panic!("protocol"))
                .sequencer_signature()
                .unwrap_or_else(|| panic!("signature"))
        );
        assert_eq!(
            evidence.metadata.receipt_signature[64],
            NODE_ED25519_SIGNATURE_TAG
        );
        assert_eq!(
            must(receipt::store_verified_if_absent(
                &mut store,
                tenant(),
                hash,
                authenticated.canonical_bytes(),
                &authenticated
            )),
            served
        );
        for selector in [
            ReceiptLookupKey::Activity(hash),
            ReceiptLookupKey::Idempotency(hash),
            ReceiptLookupKey::GlobalSequence(authenticated.global_sequence()),
        ] {
            assert_eq!(must(receipt::serve(&store, tenant(), selector)), served);
        }
        let transaction_key = must(TenantKey::new(
            tenant(),
            ObjectKind::Receipt,
            [b"receipt:transaction:".as_slice(), &hash].concat(),
        ));
        let stored = store
            .get(&transaction_key)
            .unwrap_or_else(|| panic!("transaction index"));
        assert_eq!(stored.class(), StorageClass::CoreProducedCache);
        assert_eq!(stored.bytes(), authenticated.canonical_bytes());
        let mut altered = authenticated.canonical_bytes().to_vec();
        altered[0] ^= 1;
        assert!(receipt::store_verified_if_absent(
            &mut store,
            tenant(),
            hash,
            &altered,
            &authenticated
        )
        .is_err());
        assert_eq!(
            must(receipt::serve_evidence(&store, tenant(), hash)),
            Some(evidence.clone())
        );
        drop(outbox);
        drop(store);
        let store = must(Store::open(&directory.0));
        assert_eq!(
            must(receipt::serve_evidence(&store, tenant(), hash)),
            Some(evidence)
        );
        assert_eq!(
            must(receipt::serve(
                &store,
                tenant(),
                ReceiptLookupKey::Idempotency(hash)
            )),
            served
        );
        assert_eq!(
            must(receipt::serve_evidence(&store, other_tenant(), hash)),
            None
        );
        assert!(
            must(node.requests.lock())
                .iter()
                .filter(|request| request["method"] == "pax_sequencerReceipt")
                .count()
                >= 2
        );
    }
}

#[test]
fn receipt_v2_authenticated_write_rolls_back_all_indexes_on_io_failure() {
    let directory = Directory::new();
    let case = &producer()["success"];
    let node = NodeTranscript::open(case);
    let authenticated = node.authenticated(case);
    let hash = authenticated.activity_id();
    let mut store = must(Store::open(&directory.0));
    must(fs::create_dir(directory.0.join("store.bin.tmp")));
    let error = receipt::store_verified_if_absent(
        &mut store,
        tenant(),
        hash,
        authenticated.canonical_bytes(),
        &authenticated,
    )
    .unwrap_err();
    assert!(matches!(error, ReceiptStoreError::Store(_)));
    assert!(store
        .list_object_ids(&tenant(), ObjectKind::Receipt)
        .is_empty());
    assert!(store
        .list_object_ids(&tenant(), ObjectKind::Configuration)
        .is_empty());
    assert!(!directory.0.join("store.bin").exists());
    assert_eq!(must(receipt::serve_evidence(&store, tenant(), hash)), None);
    must(fs::remove_dir(directory.0.join("store.bin.tmp")));
    must(receipt::store_verified_if_absent(
        &mut store,
        tenant(),
        hash,
        authenticated.canonical_bytes(),
        &authenticated,
    ));
    drop(store);
    let store = must(Store::open(&directory.0));
    assert_eq!(
        must(receipt::serve_evidence(&store, tenant(), hash))
            .unwrap_or_else(|| panic!("evidence"))
            .canonical_receipt,
        authenticated.canonical_bytes()
    );
}
