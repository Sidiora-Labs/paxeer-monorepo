//! Task 22.31.223 repair record.
//! `session_tools_unbound`: `session_tools_bind_describe_and_acknowledge`,
//! `session_tools_read_only_filtering`, `session_tools_authenticated_daemon_codec`,
//! `session_approval_result_keeps_major_two_fields`.
//! `shared_payer_methods`: `existing_grants_through_opened_web_route`,
//! `existing_grants_still_require_verified_receipts`.
//! Wiring request: domain 9, `services/agent/crates/layerx-agentd-host/src/payer.rs:`
//! implement `WebPayerBoundary::standing_grant` and `WebPayerBoundary::view` against the authenticated node.
//! Baseline: no prior gate ledger or repair test existed; the ordered implementation-first
//! and wrapper-only gate instructions prohibit the task text's additional baseline run.
//! Qualification revision and private logs are recorded by hf-`build/hf-done`, not invented here.

use layerx_mcp_test_support::option_assertion;
use layerx_mcp_test_support::result_assertion;

use layerx_mcp_test_support::test_clock;

use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::error::RequestId;
use layerx_agent_api::identity::{AgentDid, ExplicitSet};
use layerx_agent_api::subscription::{
    Cursor, DeliveryTarget, SubscriptionCreate, SubscriptionFilter, SubscriptionId,
    SubscriptionScope, SubscriptionTarget,
};
use layerx_agentd::approval::{ApprovalExpiry, ApprovalService};
use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope};
use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
use layerx_agentd::events::subscription::Store as SubscriptionStore;
use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::policy::approval::ApprovalRegistry;
use layerx_agentd::prepare::evm::{PrecompileCall, PrecompileTarget};
use layerx_agentd::prepare::PreparationLifecycle;
use layerx_agentd::session::{open, OpenRequest, SessionCredential, SessionId, SessionRegistry};
use layerx_agentd::session_control::{McpInvocationIdentity, SessionControl};
use layerx_agentd::store::{Store, TenantId};
use layerx_client::sequencer::{CallRequest, SequencerEndpoint, SequencerReceipt};
use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
use layerx_crypto::payments::{Grant, Payment};
use layerx_mcp::binding::{Binding, WebAuthority};
use layerx_mcp::boundary::{BoundaryRefusal, ToolBoundary};
use layerx_mcp::catalogue::{self, SESSION_TOOLS};
use layerx_mcp::server::{DeploymentMode, ReadOnly, Server, ToolDefinition, ToolKind};
use layerx_mcp::stdio::{Bound, Session};
use layerx_mcp::tools::web::{ExactPayment, GrantTerms, WebPayer, WebPayerError};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_sdk::agent_envelope::{encode_envelope, EnvelopeCredential};
use layerx_types::ids::Did;
use layerx_types::payload::ModuleId;
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

static NEXT: AtomicU64 = AtomicU64::new(1);
const HEIGHT: u64 = 50;

fn hex(bytes: &[u8]) -> String {
    layerx_programs::hex::encode(bytes)
}

fn unhex(value: &str) -> Vec<u8> {
    let value = value.strip_prefix("0x").unwrap_or(value);
    (0..value.len())
        .step_by(2)
        .map(|i| result_assertion::required(u8::from_str_radix(&value[i..i + 2], 16), "hex"))
        .collect()
}

fn word(value: &Value) -> [u8; 32] {
    result_assertion::required(
        unhex(option_assertion::required(value.as_str(), "word string")).try_into(),
        "word width",
    )
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let root =
            result_assertion::required(fs::canonicalize(std::env::temp_dir()), "temporary root")
                .join(format!(
                    "repair-22-31-223-{}-{}",
                    std::process::id(),
                    NEXT.fetch_add(1, Ordering::Relaxed)
                ));
        result_assertion::required(fs::create_dir(&root), "directory");
        result_assertion::required(
            fs::set_permissions(&root, fs::Permissions::from_mode(0o700)),
            "directory mode",
        );
        Self(root)
    }
    fn secret(&self, name: &str, contents: &str) -> PathBuf {
        let path = self.0.join(name);
        result_assertion::required(fs::write(&path, contents), "protected file");
        result_assertion::required(
            fs::set_permissions(&path, fs::Permissions::from_mode(0o600)),
            "file mode",
        );
        path
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        result_assertion::required(fs::remove_dir_all(&self.0), "remove state");
    }
}

// Identity provisioning is outside the repaired server; all sessions and capabilities below
// are minted, persisted and restored by the daemon's actual registration/open paths.
struct IdentityLedger(CoreIdentity);
impl IdentityResolver for IdentityLedger {
    fn resolve(&mut self, _: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(self.0.clone()))
    }
}

struct Enrolled {
    directory: Directory,
    control: SessionControl,
    credential: SessionCredential,
}
impl Enrolled {
    fn new(scopes: &[&str], agent: &str) -> Self {
        let directory = Directory::new();
        let tenant = result_assertion::required(TenantId::new("tenant-a"), "tenant");
        let mut store = result_assertion::required(Store::open(directory.0.join("store")), "store");
        let capability = result_assertion::required(
            Capability::new(
                CapabilityId([9; 32]),
                tenant.clone(),
                CapabilityDimensions {
                    activity_types: BTreeSet::from([7]),
                    counterparties: BTreeSet::from([[2; 32]]),
                    assets: BTreeSet::from([[3; 32]]),
                    amount_ceiling: 100,
                    rate_ceiling: RateCeiling {
                        maximum_uses: 100,
                        window_sequences: 10,
                    },
                    purposes: BTreeSet::from(["service-payment".to_owned()]),
                    expiry_sequence: 400,
                },
            ),
            "capability",
        );
        result_assertion::required(capability.persist(&mut store), "persist capability");
        let mut ledger = IdentityLedger(CoreIdentity {
            canonical_bytes: agent.as_bytes().to_vec(),
            head_sequence: 10,
            revocation_sequence: 1,
            verification_level: VerificationLevel::STATE_PROVEN,
            frozen: false,
            authorities: vec![ProtocolAuthority::CapabilityGrant(capability.id.0)],
        });
        let identity = result_assertion::required(
            register(
                &mut store,
                tenant.clone(),
                result_assertion::required(Did::new(agent.as_bytes()), "DID"),
                &mut ledger,
            ),
            "register",
        );
        let mut sessions = SessionRegistry::default();
        let credential = result_assertion::required(
            open(
                &mut store,
                &mut sessions,
                &identity,
                OpenRequest {
                    session_id: SessionId([7; 32]),
                    token_id: [8; 32],
                    tenant,
                    agent: identity.did().clone(),
                    authority: ProtocolAuthority::CapabilityGrant(capability.id.0),
                    permitted_activity_types: BTreeSet::from([7]),
                    scopes: scopes.iter().map(|scope| (*scope).to_owned()).collect(),
                    expiry_seconds: None,
                    expiry_sequence: 300,
                    opening_client: "mcp".to_owned(),
                    policy_version: "policy-v1".to_owned(),
                },
                HEIGHT,
            ),
            "open authenticated session",
        )
        .credential();
        let control = SessionControl::new(
            Arc::new(Mutex::new(store)),
            sessions,
            Arc::new(PreparationLifecycle::default()),
            Arc::new(result_assertion::required(
                BudgetLimiter::new(vec![limit()]),
                "limiter",
            )),
        );
        Self {
            directory,
            control,
            credential,
        }
    }
    fn binding_document(&self, endpoint: &str, mode: &str) -> Value {
        json!({
            "mode": mode, "tenant": "tenant-a", "store": self.directory.0.join("store"),
            "audit_root": self.directory.0.join("audit"), "session_id": "07".repeat(32),
            "session_token_file": self.directory.secret("token", &hex(&self.credential.token_id())),
            "session_generation": 1, "capability_id": "09".repeat(32), "core_height": HEIGHT, "deadline_ms": 5000,
            "agent": {"endpoint": endpoint, "bearer_file": self.directory.secret("bearer", &"b".repeat(48)), "probe_program": "cc".repeat(32)},
            "limit": {"id": "0a".repeat(16), "name": "mcp", "scope": "tenant", "scope_id": "01".repeat(32), "ceiling": "1000", "consumed": "0"},
        })
    }
    fn bound(&self, mode: DeploymentMode) -> Bound {
        if mode == DeploymentMode::Full {
            Bound::Full(Box::new(result_assertion::required(
                Server::bind(
                    self.control.clone(),
                    self.credential.clone(),
                    CapabilityId([9; 32]),
                    HEIGHT,
                    self.directory.0.join("audit"),
                ),
                "full binding",
            )))
        } else {
            Bound::ReadOnly(Box::new(result_assertion::required(
                ReadOnly::bind(
                    self.control.clone(),
                    self.credential.clone(),
                    CapabilityId([9; 32]),
                    HEIGHT,
                    self.directory.0.join("audit"),
                ),
                "read-only binding",
            )))
        }
    }
}
fn limit() -> LimitConfig {
    LimitConfig {
        id: LimitId([10; 16]),
        name: "mcp".to_owned(),
        scope: LimitScope::Tenant([1; 32]),
        ceiling: 1000,
        consumed: 0,
    }
}
fn position(cursor: Cursor) -> Value {
    json!({"block_height": cursor.block_height.to_string(), "tx_index": cursor.tx_index.to_string(), "log_index": cursor.log_index.to_string()})
}

struct SubscriptionBoundary {
    store: SubscriptionStore,
    target: SubscriptionTarget,
}
impl SubscriptionBoundary {
    fn open(root: &Path) -> Self {
        let tenant = result_assertion::required(TenantId::new("tenant-a"), "tenant");
        let mut store = result_assertion::required(
            SubscriptionStore::open(
                result_assertion::required(
                    Store::open(root.join("subscriptions")),
                    "durable subscriptions",
                ),
                tenant,
            ),
            "subscription store",
        );
        let scope = SubscriptionScope {
            tenant: result_assertion::required(
                layerx_agent_api::identity::TenantId::new("tenant-a"),
                "tenant",
            ),
            agent: result_assertion::required(AgentDid::new("did:layerx:model"), "agent"),
            capability: result_assertion::required(
                layerx_agent_api::identity::CapabilityId::new("09".repeat(32)),
                "capability",
            ),
        };
        let target = SubscriptionTarget {
            scope: scope.clone(),
            subscription_id: result_assertion::required(
                SubscriptionId::new("subscription-a"),
                "id",
            ),
        };
        result_assertion::required(
            store.create(
                target.subscription_id.clone(),
                SubscriptionCreate {
                    scope,
                    start: Cursor {
                        block_height: 0,
                        tx_index: 0,
                        log_index: 0,
                    },
                    delivery_target: result_assertion::required(
                        DeliveryTarget::new("uds://consumer-a"),
                        "target",
                    ),
                    filter: SubscriptionFilter {
                        agents: ExplicitSet::deny_all(),
                        accounts: ExplicitSet::deny_all(),
                        activity_types: ExplicitSet::deny_all(),
                        modules: ExplicitSet::deny_all(),
                        assets: ExplicitSet::deny_all(),
                        counterparties: ExplicitSet::deny_all(),
                        result_classes: ExplicitSet::deny_all(),
                    },
                },
            ),
            "create subscription",
        );
        result_assertion::required(
            store.mark_delivered(
                &target,
                layerx_agentd::events::subscription::Cursor {
                    block_height: 12,
                    tx_index: 3,
                    log_index: 4,
                },
            ),
            "deliver cursor",
        );
        Self { store, target }
    }
    fn arguments(&self) -> Value {
        json!({"scope": {"tenant": self.target.scope.tenant.as_str(), "agent": self.target.scope.agent.as_str(), "capability": self.target.scope.capability.as_str()},
            "subscription_id": self.target.subscription_id.as_str(), "cursor": {"block_height": "12", "tx_index": "3", "log_index": "4"}})
    }
}
impl ToolBoundary for SubscriptionBoundary {
    fn execute(
        &mut self,
        tool: ToolDefinition,
        arguments: &Value,
    ) -> Result<Value, BoundaryRefusal> {
        let records = match tool.name {
            "subscription.acknowledge" => vec![catalogue::subscription_acknowledge_tool(
                &mut self.store,
                arguments,
            )?],
            "subscription.list" => self.store.list(&self.target.scope),
            _ => return Err(BoundaryRefusal::NotServed(tool.name)),
        };
        Ok(
            json!({"subscriptions": records.iter().map(|record| json!({"subscription_id": record.subscription_id.as_str(), "last_acknowledged": position(record.last_acknowledged)})).collect::<Vec<_>>()}),
        )
    }
    fn observed_sequence(&mut self) -> Result<u64, BoundaryRefusal> {
        Ok(HEIGHT)
    }
}

fn rpc<B: ToolBoundary>(session: &mut Session<B>, method: &str, params: &Value) -> Value {
    let input = json!({"jsonrpc":"2.0", "id":1, "method":method, "params": params});
    let mut source = std::io::Cursor::new(format!("{input}\n"));
    let mut output = Vec::new();
    result_assertion::required(
        session.serve(&mut source, &mut output),
        "MCP line transport",
    );
    result_assertion::required(serde_json::from_slice(&output), "MCP response")
}
fn names(value: &Value) -> BTreeSet<String> {
    option_assertion::required(value.as_array(), "tools")
        .iter()
        .map(|tool| option_assertion::required(tool["name"].as_str(), "tool name").to_owned())
        .collect()
}
fn daemon_call<F>(
    fixture: &Enrolled,
    binding: &Binding,
    tool: &'static str,
    arguments: &Value,
    owner: F,
) -> Value
where
    F: FnMut(
        layerx_agentd::tenant::Operation,
        &SessionCredential,
        &Value,
        [u8; 32],
    ) -> Result<Value, layerx_agentd::agent_rpc::Rejection>,
{
    let sequence = NEXT.fetch_add(1, Ordering::Relaxed);
    let mut key = [1; 32];
    key[..8].copy_from_slice(&sequence.to_be_bytes());
    let credential = result_assertion::required(
        EnvelopeCredential::new("tenant-a", [7; 32], fixture.credential.token_id(), 1),
        "SDK credential",
    );
    let wire = result_assertion::required(
        encode_envelope(
            layerx_agent_api::Operation::McpInvoke,
            RequestId(sequence),
            &json!({"tool":tool,"arguments":arguments}),
            Some(&credential),
            Some(result_assertion::required(
                layerx_agent_api::idempotency::Key::new(key),
                "key",
            )),
        ),
        "SDK envelope producer",
    );
    let bytes = result_assertion::required(serde_json::to_vec(&wire), "encode transport");
    let decoded = option_assertion::required(
        layerx_agentd::agent_rpc::decode_mcp_invocation(&bytes)
            .unwrap_or_else(|_| panic!("daemon envelope consumer")),
        "MCP operation",
    );
    assert_eq!(decoded.tool, tool);
    let result = binding
        .daemon_invocation(
            fixture.control.clone(),
            decoded.credential,
            HEIGHT,
            tool,
            &decoded.arguments,
            McpInvocationIdentity {
                tool_name: tool,
                idempotency_key: decoded.idempotency_key,
                request_digest: Sha256::digest(&bytes).into(),
            },
            None,
            owner,
        )
        .unwrap_or_else(|error| panic!("daemon invocation: {error:?}"));
    result_assertion::required(
        result_assertion::required(result.release_current(HEIGHT, Ok), "authorized release"),
        "tool result",
    )
}

#[test]
fn session_tools_bind_describe_and_acknowledge() {
    for mixed in [false, true] {
        let mut scopes = vec!["subscribe", "approve"];
        if mixed {
            scopes.push("read:balance");
        }
        let fixture = Enrolled::new(&scopes, "did:layerx:model");
        let binding = result_assertion::required(
            Binding::parse(&fixture.binding_document("127.0.0.1:9", "full").to_string()),
            "binding document",
        );
        let description = daemon_call(
            &fixture,
            &binding,
            "mcp.describe",
            &json!({}),
            |_, _, _, _| panic!("describe must use the real server"),
        );
        let boundary = SubscriptionBoundary::open(&fixture.directory.0);
        let target = boundary.target.clone();
        let arguments = boundary.arguments();
        let mut session = Session::new(fixture.bound(DeploymentMode::Full), boundary);
        let listed = rpc(&mut session, "tools/list", &json!({}));
        assert_eq!(
            names(&listed["result"]["tools"]),
            names(&description["tools"])
        );
        for tool in SESSION_TOOLS {
            assert!(
                names(&description["tools"]).contains(tool.name),
                "missing {}",
                tool.name
            );
            assert!(
                option_assertion::required(listed["result"]["tools"].as_array(), "list").contains(
                    &option_assertion::required(catalogue::listing(tool), "description")
                )
            );
        }
        assert_eq!(names(&description["tools"]).contains("balance.get"), mixed);
        for cursor in [
            json!(12),
            json!("12"),
            json!({"block_height":"12","tx_index":"3"}),
            json!({"block_height":"12","tx_index":"3","log_index":"4294967296"}),
        ] {
            let mut invalid = arguments.clone();
            invalid["cursor"] = cursor;
            let response = rpc(
                &mut session,
                "tools/call",
                &json!({"name":"subscription.acknowledge", "arguments":invalid}),
            );
            assert_eq!(response["result"]["isError"], true, "{response}");
        }
        let response = rpc(
            &mut session,
            "tools/call",
            &json!({"name":"subscription.acknowledge", "arguments":arguments}),
        );
        assert_eq!(response["result"]["isError"], false, "{response}");
        assert_eq!(
            response["result"]["structuredContent"]["result"]["subscriptions"][0]
                ["last_acknowledged"],
            arguments["cursor"]
        );
        drop(session);
        let recovered = result_assertion::required(
            SubscriptionStore::open(
                result_assertion::required(
                    Store::open(fixture.directory.0.join("subscriptions")),
                    "reopen durable component",
                ),
                result_assertion::required(TenantId::new("tenant-a"), "tenant"),
            ),
            "restore component",
        );
        assert_eq!(
            position(
                result_assertion::required(recovered.get(&target), "recovered subscription")
                    .last_acknowledged
            ),
            arguments["cursor"]
        );
    }
}

#[test]
fn session_tools_read_only_filtering() {
    for mixed in [false, true] {
        let mut scopes = vec!["subscribe", "approve"];
        if mixed {
            scopes.push("read:balance");
        }
        let fixture = Enrolled::new(&scopes, "did:layerx:model");
        let binding = result_assertion::required(
            Binding::parse(
                &fixture
                    .binding_document("127.0.0.1:9", "read-only")
                    .to_string(),
            ),
            "read-only binding",
        );
        let description = daemon_call(
            &fixture,
            &binding,
            "mcp.describe",
            &json!({}),
            |_, _, _, _| panic!("server describes"),
        );
        let mut session = Session::new(
            fixture.bound(DeploymentMode::ReadOnly),
            SubscriptionBoundary::open(&fixture.directory.0),
        );
        let listed = rpc(&mut session, "tools/list", &json!({}));
        assert_eq!(description["mode"], "read-only");
        assert_eq!(
            names(&description["tools"]),
            names(&listed["result"]["tools"])
        );
        for tool in SESSION_TOOLS {
            assert_eq!(
                names(&description["tools"]).contains(tool.name),
                tool.kind == ToolKind::Read,
                "{}",
                tool.name
            );
        }
        let refused = rpc(
            &mut session,
            "tools/call",
            &json!({"name":"subscription.acknowledge", "arguments":{}}),
        );
        assert_eq!(refused["error"]["code"], -32_602);
        let listed = rpc(
            &mut session,
            "tools/call",
            &json!({"name":"subscription.list", "arguments":{"scope":{"tenant":"tenant-a","agent":"did:layerx:model","capability":"09".repeat(32)}}}),
        );
        assert_eq!(listed["result"]["isError"], false, "{listed}");
        assert_eq!(
            option_assertion::required(
                listed["result"]["structuredContent"]["result"]["subscriptions"].as_array(),
                "actual stored subscription"
            )
            .len(),
            1
        );
    }
}

#[test]
fn session_tools_authenticated_daemon_codec() {
    let fixture = Enrolled::new(&["subscribe", "approve"], "did:layerx:model");
    let binding = result_assertion::required(
        Binding::parse(&fixture.binding_document("127.0.0.1:9", "full").to_string()),
        "binding",
    );
    let mut boundary = SubscriptionBoundary::open(&fixture.directory.0);
    let arguments = boundary.arguments();
    let output = daemon_call(
        &fixture,
        &binding,
        "subscription.acknowledge",
        &arguments,
        |operation, credential, arguments, _| {
            assert_eq!(
                operation,
                layerx_agentd::tenant::Operation::SubscriptionAcknowledge
            );
            assert_eq!(credential, &fixture.credential);
            Ok(result_assertion::required(
                boundary.execute(SESSION_TOOLS[6], arguments),
                "actual store acknowledgement",
            ))
        },
    );
    assert_eq!(
        output["subscriptions"][0]["last_acknowledged"],
        arguments["cursor"]
    );
    let forged = SessionCredential::new(
        result_assertion::required(TenantId::new("tenant-a"), "tenant"),
        SessionId([7; 32]),
        [99; 32],
        1,
    );
    assert!(binding
        .daemon_invocation(
            fixture.control.clone(),
            forged,
            HEIGHT,
            "mcp.describe",
            &json!({}),
            McpInvocationIdentity {
                tool_name: "mcp.describe",
                idempotency_key: [22; 32],
                request_digest: [23; 32],
            },
            None,
            |_, _, _, _| panic!("unauthenticated owner call")
        )
        .is_err());
}

fn recording(name: &str) -> Value {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../services/interop/crates/x-websearch/tests/fixtures")
        .join(name);
    result_assertion::required(
        serde_json::from_slice(&result_assertion::required(
            fs::read(path),
            "producer recording",
        )),
        "recorded JSON",
    )
}

struct Request {
    method: String,
    target: String,
    headers: BTreeMap<String, String>,
    body: Vec<u8>,
}
fn read_request(stream: &mut TcpStream) -> Request {
    result_assertion::required(
        stream.set_read_timeout(Some(Duration::from_secs(5))),
        "read deadline",
    );
    let mut reader = BufReader::new(stream);
    let mut first = String::new();
    result_assertion::required(reader.read_line(&mut first), "request line");
    let parts: Vec<_> = first.split_whitespace().collect();
    let mut headers = BTreeMap::new();
    loop {
        let mut line = String::new();
        result_assertion::required(reader.read_line(&mut line), "header");
        if line == "\r\n" {
            break;
        }
        let (key, value) = option_assertion::required(line.split_once(':'), "HTTP header");
        headers.insert(key.to_ascii_lowercase(), value.trim().to_owned());
    }
    let size = headers.get("content-length").map_or(0, |size| {
        result_assertion::required(size.parse::<usize>(), "body size")
    });
    let mut body = vec![0; size];
    result_assertion::required(reader.read_exact(&mut body), "request body");
    Request {
        method: parts[0].to_owned(),
        target: parts[1].to_owned(),
        headers,
        body,
    }
}
fn respond(stream: &mut TcpStream, status: u16, headers: &[(String, String)], body: &[u8]) {
    result_assertion::required(
        write!(
            stream,
            "HTTP/1.1 {status} Recorded\r\nContent-Length: {}\r\nConnection: close\r\n",
            body.len()
        ),
        "response head",
    );
    for (name, value) in headers {
        result_assertion::required(write!(stream, "{name}: {value}\r\n"), "response header");
    }
    result_assertion::required(stream.write_all(b"\r\n"), "header end");
    result_assertion::required(stream.write_all(body), "response body");
}

struct ExchangeServer {
    address: String,
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
    node_calls: Arc<Mutex<Vec<Value>>>,
    paid: Arc<AtomicU64>,
}
impl ExchangeServer {
    fn start(offered: bool, corrupt_receipt: bool) -> Self {
        let listener =
            result_assertion::required(TcpListener::bind("127.0.0.1:0"), "recording listener");
        let address = result_assertion::required(listener.local_addr(), "address").to_string();
        let endpoint = format!("http://{address}");
        let stop = Arc::new(AtomicBool::new(false));
        let stop_worker = Arc::clone(&stop);
        let node_calls = Arc::new(Mutex::new(Vec::new()));
        let calls_worker = Arc::clone(&node_calls);
        let paid = Arc::new(AtomicU64::new(0));
        let paid_worker = Arc::clone(&paid);
        let thread = thread::spawn(move || {
            let exchange = recording("client-exchange.json");
            let gateway = recording("gateway/metered.json");
            let mut challenge =
                exchange["exchange"][0]["response"]["headers"]["PAYMENT-REQUIRED"].clone();
            challenge["resource"]["url"] = json!(format!("{endpoint}/search?q=paxeer"));
            let signature = &exchange["exchange"][1]["request"]["headers"]["PAYMENT-SIGNATURE"];
            let grant_id = signature["payload"]["grantId"].clone();
            if offered {
                challenge["accepts"][0]["extra"]["layerx"]["grantId"] = grant_id.clone();
            }
            loop {
                let (mut stream, _) =
                    result_assertion::required(listener.accept(), "accept exchange");
                if stop_worker.load(Ordering::SeqCst) {
                    break;
                }
                let request = read_request(&mut stream);
                match (request.method.as_str(), request.target.as_str()) {
                    ("GET", path) if path.starts_with("/v1/programs/") => {
                        assert_eq!(
                            request.headers.get("authorization"),
                            Some(&format!("Bearer {}", "b".repeat(48)))
                        );
                        let body = json!({"program":"cc".repeat(32), "lifecycle":"active", "accounts":[],
                            "freshness":{"observed_sequence":HEIGHT,"observed_at":1,"receipt_digest":"33".repeat(32),"state_root":"44".repeat(32),"valid_through":400}});
                        respond(&mut stream, 200, &[], body.to_string().as_bytes());
                    }
                    ("POST", "/") => {
                        let query: Value = result_assertion::required(
                            serde_json::from_slice(&request.body),
                            "node query",
                        );

                        answer_node_query(&mut stream, &query, &gateway, &calls_worker);
                    }
                    ("GET", "/search?q=paxeer") => answer_paid_search(
                        &mut stream,
                        &request,
                        &challenge,
                        signature,
                        &exchange,
                        &paid_worker,
                        corrupt_receipt,
                    ),
                    _ => panic!("unrecorded exchange: {} {}", request.method, request.target),
                }
            }
        });
        Self {
            address,
            stop,
            thread: Some(thread),
            node_calls,
            paid,
        }
    }
    fn endpoint(&self) -> String {
        format!("http://{}", self.address)
    }
}
impl Drop for ExchangeServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        let _ = TcpStream::connect(&self.address);
        result_assertion::required(
            option_assertion::required(self.thread.take(), "recording worker").join(),
            "recording assertions",
        );
    }
}

#[derive(Default)]
struct PayerTraffic {
    standing: u64,
    account_views: u64,
    grant_views: u64,
    issued: u64,
    submitted: u64,
}

// A node-backed payer: grant discovery uses actual signed ledger grant records,
// reads use the real SequencerEndpoint transport, and submission signs a real type-2
// transaction and verifies its sequencer receipt. Existing-grant cases must not submit.
struct NodePayer {
    node: SequencerEndpoint,
    grants: Vec<Grant>,
    traffic: Arc<Mutex<PayerTraffic>>,
    key: SigningKey,
    authorization: SequencerAuthorization,
}
impl NodePayer {
    fn new(server: &ExchangeServer, traffic: Arc<Mutex<PayerTraffic>>) -> Self {
        let buyer = recording("gateway/buyer.json");
        let gateway = recording("gateway/metered.json");
        let grants =
            option_assertion::required(gateway["ledger"]["grants"].as_array(), "ledger grants")
                .iter()
                .filter(|record| {
                    ["PAX", "SID", "USDC", "USDL"].contains(&option_assertion::required(
                        record["name"].as_str(),
                        "grant name",
                    ))
                })
                .map(|record| {
                    match result_assertion::required(
                        Payment::decode(
                            ModuleId::Asset,
                            7,
                            &unhex(option_assertion::required(
                                record["canonical"].as_str(),
                                "grant bytes",
                            )),
                            b"",
                        ),
                        "real signed grant codec",
                    ) {
                        Payment::IssueGrant(grant) => grant,
                        _ => panic!("ledger grant producer"),
                    }
                })
                .collect();
        Self {
            node: result_assertion::required(
                SequencerEndpoint::new(&server.endpoint(), &[]),
                "node transport",
            ),
            grants,
            traffic,
            key: SigningKey::from_bytes(&[0x33; 32]),
            authorization: SequencerAuthorization::new(
                word(&buyer["sequencerId"]),
                word(&buyer["sequencerPublicKey"]),
                option_assertion::required(buyer["firstBatchNumber"].as_u64(), "first height"),
                option_assertion::required(buyer["lastBatchNumber"].as_u64(), "last height"),
            ),
        }
    }
}
impl WebPayer for NodePayer {
    fn standing_grant(&mut self, terms: &GrantTerms) -> Result<Option<[u8; 32]>, WebPayerError> {
        self.traffic
            .lock()
            .map_err(|_| WebPayerError::Unavailable)?
            .standing += 1;
        Ok(self
            .grants
            .iter()
            .find(|grant| {
                grant.from == terms.payer_account
                    && grant.recipient == terms.recipient
                    && grant.asset == terms.asset
                    && grant.purpose_hash == terms.purpose_hash
                    && grant.per_draw_maximum >= terms.amount
                    && grant.allowance >= terms.amount
            })
            .map(|grant| grant.id))
    }
    fn view(&mut self, call: &PrecompileCall) -> Result<Vec<u8>, WebPayerError> {
        assert_eq!(call.target, PrecompileTarget::Ledger);
        {
            let mut traffic = self
                .traffic
                .lock()
                .map_err(|_| WebPayerError::Unavailable)?;
            match call.selector {
                [0x3f, 0x05, 0x0c, 0xbc] => traffic.account_views += 1,
                [0x42, 0x3f, 0xe4, 0xb2] => traffic.grant_views += 1,
                selector => panic!("unexpected ledger view selector: {selector:?}"),
            }
        }
        let data = call.encode().map_err(|_| WebPayerError::Refused)?;
        self.node
            .call_pending(&call.target.address(), &data)
            .map_err(|_| WebPayerError::Unavailable)
    }
    fn sign_grant(&mut self, terms: &GrantTerms) -> Result<Grant, WebPayerError> {
        self.traffic
            .lock()
            .map_err(|_| WebPayerError::Unavailable)?
            .issued += 1;
        let mut grant = terms.unsigned_grant(self.key.verifying_key().to_bytes());
        grant.id = grant.signing_digest().map_err(|_| WebPayerError::Refused)?;
        grant.signature = self.key.sign(&grant.id).to_bytes();
        Ok(grant)
    }
    fn submit(&mut self, call: &PrecompileCall) -> Result<ExactPayment, WebPayerError> {
        self.traffic
            .lock()
            .map_err(|_| WebPayerError::Unavailable)?
            .submitted += 1;
        let secret = [3; 32];
        let signer =
            k256::ecdsa::SigningKey::from_slice(&secret).map_err(|_| WebPayerError::Refused)?;
        let address = layerx_crypto::secp256k1::evm_address(
            signer.verifying_key().to_encoded_point(false).as_bytes(),
        )
        .map_err(|_| WebPayerError::Refused)?;
        let data = call.encode().map_err(|_| WebPayerError::Refused)?;
        let request = CallRequest {
            from: Some(address),
            to: Some(call.target.address()),
            value: 0,
            data: data.clone(),
        };
        let tx = Eip1559Call {
            chain_id: self
                .node
                .chain_id()
                .map_err(|_| WebPayerError::Unavailable)?,
            nonce: self
                .node
                .pending_nonce(&address)
                .map_err(|_| WebPayerError::Unavailable)?,
            max_priority_fee_per_gas: 0,
            max_fee_per_gas: 1,
            gas_limit: self
                .node
                .estimate_gas(&request)
                .map_err(|_| WebPayerError::Unavailable)?,
            to: call.target.address(),
            data,
        };
        let raw = tx.sign(&secret).map_err(|_| WebPayerError::Refused)?;
        let hash = self
            .node
            .send_raw_transaction(&raw)
            .map_err(|_| WebPayerError::Unavailable)?;
        if hash != transaction_hash(&raw) {
            return Err(WebPayerError::Refused);
        }
        let Some(
            SequencerReceipt::Receipted(execution)
            | SequencerReceipt::Committed {
                receipt: execution, ..
            },
        ) = self
            .node
            .sequencer_receipt(&hash)
            .map_err(|_| WebPayerError::Unavailable)?
        else {
            return Err(WebPayerError::Unavailable);
        };
        if !execution.success || execution.from != address || execution.to != call.target.address()
        {
            return Err(WebPayerError::Refused);
        }
        let decoded = verify_sequencer_signature(&execution.receipt, &[self.authorization])
            .map_err(|_| WebPayerError::Refused)?;
        let logs = execution
            .logs
            .iter()
            .map(|log| {
                SequencerLog::new(
                    log.address,
                    log.topics.clone(),
                    log.data.clone(),
                    log.log_index,
                )
            })
            .collect();
        let logs = verify_logs(&decoded, logs).map_err(|_| WebPayerError::Refused)?;
        Ok(ExactPayment {
            tx_hash: hash,
            receipt: execution.receipt,
            logs: logs.into_logs(),
        })
    }
}

fn existing_grant_exchange(offered: bool, corrupt_receipt: bool) {
    let server = ExchangeServer::start(offered, corrupt_receipt);
    let buyer = recording("gateway/buyer.json");
    let fixture = Enrolled::new(
        &["write:web:search"],
        option_assertion::required(buyer["payerDid"].as_str(), "recorded payer"),
    );
    let mut document = fixture.binding_document(&server.address, "full");
    drop(fixture.control);
    document["web"] = json!({
        "endpoint":server.endpoint(), "payer_address":"0xabababababababababababababababababababab",
        "receiver_address":recording("gateway/metered.json")["ledger"]["receiverAddress"], "network":"eip155:125",
        "receipt_key_index":[{"keyId":buyer["sequencerId"], "publicKey":buyer["sequencerPublicKey"], "firstHeight":buyer["firstBatchNumber"], "lastHeight":buyer["lastBatchNumber"]}],
        "sequencer_id":buyer["sequencerId"], "first_batch_number":buyer["firstBatchNumber"], "last_batch_number":buyer["lastBatchNumber"],
        "timeout_ms":5000, "pending_attempts":1, "approval_threshold":"1000000000000000000",
    });
    let traffic = Arc::new(Mutex::new(PayerTraffic::default()));
    let payer = NodePayer::new(&server, Arc::clone(&traffic));
    let mut binding =
        result_assertion::required(Binding::parse(&document.to_string()), "real web binding");
    binding.attach_web(WebAuthority::new(
        Arc::new(test_clock::TestClock::new(50)),
        Arc::new(ApprovalRegistry::default()),
        Box::new(payer),
    ));
    let mut session = result_assertion::required(binding.open_session(), "open daemon web route");
    let exchange = recording("client-exchange.json");
    let signature = &exchange["exchange"][1]["request"]["headers"]["PAYMENT-SIGNATURE"];
    let answer = rpc(
        &mut session,
        "tools/call",
        &json!({"name":"web.search", "arguments":{
            "query":"paxeer", "currency":"SID", "scheme":"metered", "idempotency_key":signature["payload"]["idempotencyKey"],
        }}),
    );
    assert_eq!(answer["result"]["isError"], corrupt_receipt, "{answer}");
    if corrupt_receipt {
        assert!(
            option_assertion::required(
                answer["result"]["structuredContent"]["refusal"].as_str(),
                "refusal"
            )
            .contains("receipt"),
            "{answer}"
        );
    } else {
        let result = &answer["result"]["structuredContent"]["result"];
        assert_eq!(
            result["settlement"]["verificationLevel"],
            "sequencer-signed"
        );
        assert_eq!(
            result["settlement"]["transaction"],
            exchange["exchange"][1]["response"]["headers"]["PAYMENT-RESPONSE"]["transaction"]
        );
        assert_eq!(result["untrusted"], true);
    }
    let traffic = result_assertion::required(traffic.lock(), "payer traffic");
    assert_eq!(
        traffic.standing,
        u64::from(!offered),
        "host standing-grant method must survive the shared adapter"
    );
    assert_eq!(
        traffic.account_views, 1,
        "payer account must be resolved through WebPayer::view"
    );
    assert_eq!(
        traffic.grant_views, 1,
        "existing grant must be checked through WebPayer::view"
    );
    assert_eq!(
        traffic.issued, 0,
        "existing grants must not prepare issueGrant"
    );
    assert_eq!(
        traffic.submitted, 0,
        "existing grants must not submit issueGrant"
    );
    assert_eq!(server.paid.load(Ordering::SeqCst), 1);
    let node = result_assertion::required(server.node_calls.lock(), "actual node traffic");
    assert_eq!(node.len(), 1);
    assert_eq!(node[0]["method"], "eth_call");
    assert_eq!(
        node[0]["params"][0]["to"],
        format!("0x{}", hex(&PrecompileTarget::Ledger.address()))
    );
    assert_eq!(
        node[0]["params"][0]["data"],
        format!(
            "0x423fe4b2{}",
            option_assertion::required(signature["payload"]["grantId"].as_str(), "grant ID")
        )
    );
}

#[test]
fn existing_grants_through_opened_web_route() {
    existing_grant_exchange(false, false);
    existing_grant_exchange(true, false);
}

#[test]
fn existing_grants_still_require_verified_receipts() {
    existing_grant_exchange(false, true);
    existing_grant_exchange(true, true);
}

struct ApprovalBoundary<'a> {
    service: ApprovalService<'a>,
    tenant: TenantId,
}
impl ToolBoundary for ApprovalBoundary<'_> {
    fn execute(
        &mut self,
        tool: ToolDefinition,
        arguments: &Value,
    ) -> Result<Value, BoundaryRefusal> {
        if tool.name != "approval.get" {
            return Err(BoundaryRefusal::NotServed(tool.name));
        }
        catalogue::approval_get_tool(&self.service, &self.tenant, &BTreeMap::new(), arguments)
    }
    fn observed_sequence(&mut self) -> Result<u64, BoundaryRefusal> {
        Ok(HEIGHT)
    }
}

#[test]
fn session_approval_result_keeps_major_two_fields() {
    use layerx_agent_api::TimestampSeconds;
    use layerx_agentd::policy::approval::{hold_at, ApprovalContext, ApprovalSequence};

    let call = Eip1559Call {
        chain_id: 125,
        nonce: 5,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: [3; 20],
        data: [
            layerx_crypto::disclosure::call::ledger_selector("transfer(address,uint256)").to_vec(),
            [vec![0; 12], vec![5; 20]].concat(),
            [vec![0; 31], vec![50]].concat(),
        ]
        .concat(),
    };
    let source = result_assertion::required(
        layerx_crypto::disclosure::call::bind_call(&call, [4; 20]),
        "real call-bound producer",
    );
    let bytes = call.encode_unsigned();
    assert_eq!(
        result_assertion::required(
            layerx_crypto::disclosure::call::reencode(&source),
            "canonical call codec"
        ),
        bytes
    );
    let fixture = Enrolled::new(&["approve"], &hex(&source.actor));
    let prepared = approval_prepared(&call, &source, &bytes);
    let registry = ApprovalRegistry::default();
    let clock = layerx_types::test_support::DeterministicClock::new(HEIGHT);
    let ticket = result_assertion::required(
        hold_at(
            &registry,
            ApprovalContext {
                tenant: fixture.credential.tenant().clone(),
                agent: result_assertion::required(Did::new(hex(&source.actor).as_bytes()), "agent"),
                actor_address: [4; 20],
                session: SessionId([7; 32]),
                capability: CapabilityId([9; 32]),
                policy_version: "policy-v1".to_owned(),
                request_id: [31; 32],
            },
            prepared,
            ApprovalSequence(HEIGHT),
            ApprovalSequence(200),
            TimestampSeconds(clock.now()),
        ),
        "hold the real call",
    );
    let expiry = result_assertion::required(
        ApprovalExpiry::open(fixture.directory.0.join("approval-decisions")),
        "durable approval service",
    );
    let limiter = result_assertion::required(BudgetLimiter::new(Vec::new()), "limiter");
    let mut boundary = ApprovalBoundary {
        service: ApprovalService::new(&registry, &limiter, &expiry),
        tenant: fixture.credential.tenant().clone(),
    };
    let binding = result_assertion::required(
        Binding::parse(&fixture.binding_document("127.0.0.1:9", "full").to_string()),
        "approval-only binding",
    );
    let arguments = json!({
        "approval_id":hex(&ticket.hold_id),
        "current_sequence":HEIGHT.to_string(),
        "current_time":clock.now().to_string()
    });
    let daemon = daemon_call(
        &fixture,
        &binding,
        "approval.get",
        &arguments,
        |operation, _, arguments, _| {
            assert_eq!(operation, layerx_agentd::tenant::Operation::ApprovalGet);
            Ok(result_assertion::required(
                boundary.execute(SESSION_TOOLS[8], arguments),
                "real approval owner",
            ))
        },
    );
    let mut session = Session::new(fixture.bound(DeploymentMode::Full), boundary);
    let response = rpc(
        &mut session,
        "tools/call",
        &json!({"name":"approval.get","arguments":arguments}),
    );
    assert_approval_response(&response, &daemon, &ticket.hold_id, &bytes);
}

fn answer_node_query(
    stream: &mut TcpStream,
    query: &Value,
    gateway: &Value,
    calls: &Mutex<Vec<Value>>,
) {
    if query["method"] == "eth_call"
        && query["params"][0]["data"]
            .as_str()
            .is_some_and(|data| data.starts_with("0x3f050cbc"))
    {
        assert_eq!(query["method"], "eth_call");
        let data = option_assertion::required(query["params"][0]["data"].as_str(), "ABI data");
        let prefix = format!("0x3f050cbc{}{}", "0".repeat(24), "ab".repeat(20));
        let asset =
            option_assertion::required(data.strip_prefix(&prefix), "accountId selector and owner");
        let grant = option_assertion::required(
            option_assertion::required(gateway["ledger"]["grants"].as_array(), "ledger grants")
                .iter()
                .find(|grant| {
                    option_assertion::required(grant["canonical"].as_str(), "canonical grant")
                        .get(192..256)
                        == Some(asset)
                }),
            "captured account for asset",
        );
        let canonical = option_assertion::required(grant["canonical"].as_str(), "canonical bytes");
        let body = json!({"jsonrpc":"2.0", "id":query["id"], "result":format!("0x{}", &canonical[64..128])});
        respond(stream, 200, &[], body.to_string().as_bytes());
        return;
    }
    result_assertion::required(calls.lock(), "node traffic").push(query.clone());
    let captured = option_assertion::required(
        gateway["endpoints"]["gateway"]["*"].as_array(),
        "recorded node traffic",
    )
    .iter()
    .find(|captured| captured["method"] == query["method"] && captured["params"] == query["params"])
    .unwrap_or_else(|| panic!("unrecorded node request: {query}"));
    let body = json!({"jsonrpc":"2.0","id":query["id"],"result":captured["result"]});
    respond(stream, 200, &[], body.to_string().as_bytes());
}

fn approval_prepared(
    call: &Eip1559Call,
    source: &layerx_crypto::disclosure::Disclosure,
    bytes: &[u8],
) -> layerx_agent_api::prepare::Prepared {
    use layerx_agent_api::identity::{ActivityType, Asset, AuthorityRef};
    use layerx_agent_api::prepare::{
        CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
        SigningPreimage,
    };
    use layerx_agent_api::{Amount, TimestampSeconds};
    let actor = result_assertion::required(AgentDid::new(hex(&source.actor)), "actor");
    let counterparties: Vec<_> = source
        .counterparties
        .iter()
        .map(|party| result_assertion::required(AgentDid::new(hex(&party.account)), "counterparty"))
        .collect();
    Prepared {
        preparation_ref: result_assertion::required(
            PreparationRef::new("call-approval"),
            "preparation reference",
        ),
        unsigned_canonical_bytes: result_assertion::required(
            CanonicalBytes::new(bytes.to_vec()),
            "canonical bytes",
        ),
        signing_preimage: result_assertion::required(
            SigningPreimage::new(call.signing_hash().to_vec()),
            "signing preimage",
        ),
        disclosure: Disclosure {
            canonical_digest: Sha256::digest(bytes).into(),
            activity_type: ActivityType(source.activity_type.ordinal()),
            actor,
            authority: result_assertion::required(
                AuthorityRef::new(hex(&source.authority)),
                "authority",
            ),
            counterparties: ExplicitSet::allow(counterparties.clone()),
            amounts: ExplicitSet::allow(
                source
                    .amounts
                    .iter()
                    .map(|amount| DisclosedAmount {
                        counterparty: counterparties[0].clone(),
                        amount: Amount(amount.value),
                    })
                    .collect(),
            ),
            asset: result_assertion::required(Asset::new(hex(&source.asset)), "asset"),
            fee_limit: Amount(source.fee_limit),
            expiry: TimestampSeconds(source.expiry.not_after),
            idempotency_key: result_assertion::required(
                IdempotencyRef::new(hex(&source.idempotency_key)),
                "idempotency reference",
            ),
        },
        expiry: TimestampSeconds(source.expiry.not_after),
    }
}

fn answer_paid_search(
    stream: &mut TcpStream,
    request: &Request,
    challenge: &Value,
    signature: &Value,
    exchange: &Value,
    paid: &AtomicU64,
    corrupt_receipt: bool,
) {
    assert_eq!(
        request
            .headers
            .get("layerx-payer-address")
            .map(String::as_str),
        Some("0xabababababababababababababababababababab")
    );
    if let Some(header) = request.headers.get("payment-signature") {
        let received: Value = result_assertion::required(
            serde_json::from_slice(&result_assertion::required(
                STANDARD.decode(header),
                "payment header",
            )),
            "payment JSON",
        );
        assert_eq!(received["x402Version"], signature["x402Version"]);
        assert_eq!(received["payload"], signature["payload"]);
        assert_eq!(received["accepted"], challenge["accepts"][0]);
        paid.fetch_add(1, Ordering::SeqCst);
        let response = &exchange["exchange"][1]["response"];
        let mut settlement = response["headers"]["PAYMENT-RESPONSE"].clone();
        if corrupt_receipt {
            let receipt = option_assertion::required(
                settlement["extensions"]["layerx"]["receipt"].as_str(),
                "receipt",
            );
            let mut bytes = result_assertion::required(STANDARD.decode(receipt), "signed receipt");
            let last = option_assertion::required(bytes.last_mut(), "signature byte");
            *last ^= 1;
            settlement["extensions"]["layerx"]["receipt"] = json!(STANDARD.encode(bytes));
        }
        let headers = vec![(
            "PAYMENT-RESPONSE".to_owned(),
            STANDARD.encode(result_assertion::required(
                serde_json::to_vec(&settlement),
                "settlement encode",
            )),
        )];
        respond(
            stream,
            200,
            &headers,
            &result_assertion::required(serde_json::to_vec(&response["body"]), "recorded resource"),
        );
    } else {
        let headers = vec![(
            "PAYMENT-REQUIRED".to_owned(),
            STANDARD.encode(result_assertion::required(
                serde_json::to_vec(&challenge),
                "challenge encode",
            )),
        )];
        respond(
            stream,
            402,
            &headers,
            &result_assertion::required(
                serde_json::to_vec(&json!({"paymentRequired":challenge})),
                "challenge body",
            ),
        );
    }
}

fn assert_approval_response(response: &Value, daemon: &Value, hold_id: &[u8; 32], bytes: &[u8]) {
    assert_eq!(response["result"]["isError"], false, "{response}");
    let result = &response["result"]["structuredContent"]["result"];
    assert_eq!(result, daemon);
    assert_eq!(result["approval_id"], hex(hold_id));
    assert_eq!(result["state"], "AwaitingApproval");
    assert!(
        option_assertion::required(result.as_object(), "approval facts").contains_key("tx_hash")
    );
    assert!(option_assertion::required(result.as_object(), "approval facts").contains_key("rung"));
    assert!(result["tx_hash"].is_null());
    assert!(result["rung"].is_null());
    assert!(result.get("activity_id").is_none());
    assert_eq!(
        result["canonical_bytes_digest"],
        hex(&Sha256::digest(bytes))
    );
}
