use layerx_types::clock::Clock as _;
#[path = "clock.rs"]
mod test_clock;

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
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn unhex(value: &str) -> Vec<u8> {
    let value = value.strip_prefix("0x").unwrap_or(value);
    (0..value.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&value[i..i + 2], 16).expect("hex"))
        .collect()
}

fn word(value: &Value) -> [u8; 32] {
    unhex(value.as_str().expect("word string"))
        .try_into()
        .expect("word width")
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let root = fs::canonicalize(std::env::temp_dir())
            .expect("temporary root")
            .join(format!(
                "repair-22-33-1553-{}-{}-{}",
                module_path!().replace("::", "-"),
                std::process::id(),
                NEXT.fetch_add(1, Ordering::Relaxed)
            ));
        fs::create_dir(&root).expect("directory");
        fs::set_permissions(&root, fs::Permissions::from_mode(0o700)).expect("directory mode");
        Self(root)
    }
    fn secret(&self, name: &str, contents: &str) -> PathBuf {
        let path = self.0.join(name);
        fs::write(&path, contents).expect("protected file");
        fs::set_permissions(&path, fs::Permissions::from_mode(0o600)).expect("file mode");
        path
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove state");
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
        let tenant = TenantId::new("tenant-a").expect("tenant");
        let mut store = Store::open(directory.0.join("store")).expect("store");
        let capability = Capability::new(
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
        )
        .expect("capability");
        capability.persist(&mut store).expect("persist capability");
        let mut ledger = IdentityLedger(CoreIdentity {
            canonical_bytes: agent.as_bytes().to_vec(),
            head_sequence: 10,
            revocation_sequence: 1,
            verification_level: VerificationLevel::STATE_PROVEN,
            frozen: false,
            authorities: vec![ProtocolAuthority::CapabilityGrant(capability.id.0)],
        });
        let identity = register(
            &mut store,
            tenant.clone(),
            Did::new(agent.as_bytes()).expect("DID"),
            &mut ledger,
        )
        .expect("register");
        let mut sessions = SessionRegistry::default();
        let credential = open(
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
        )
        .expect("open authenticated session")
        .credential();
        let control = SessionControl::new(
            Arc::new(Mutex::new(store)),
            sessions,
            Arc::new(PreparationLifecycle::default()),
            Arc::new(BudgetLimiter::new(vec![limit()]).expect("limiter")),
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
            "audit_root": self.directory.0.join("daemon-audit"), "session_id": "07".repeat(32),
            "session_token_file": self.directory.secret("token", &hex(&self.credential.token_id())),
            "session_generation": 1, "capability_id": "09".repeat(32), "core_height": HEIGHT, "deadline_ms": 5000,
            "agent": {"endpoint": endpoint, "bearer_file": self.directory.secret("bearer", &"b".repeat(48)), "probe_program": "cc".repeat(32)},
            "limit": {"id": "0a".repeat(16), "name": "mcp", "scope": "tenant", "scope_id": "01".repeat(32), "ceiling": "1000", "consumed": "0"},
        })
    }
    fn bound(&self, mode: DeploymentMode) -> Bound {
        if mode == DeploymentMode::Full {
            Bound::Full(Box::new(
                Server::bind(
                    self.control.clone(),
                    self.credential.clone(),
                    CapabilityId([9; 32]),
                    HEIGHT,
                    self.directory.0.join("mcp-audit"),
                )
                .expect("full binding"),
            ))
        } else {
            Bound::ReadOnly(Box::new(
                ReadOnly::bind(
                    self.control.clone(),
                    self.credential.clone(),
                    CapabilityId([9; 32]),
                    HEIGHT,
                    self.directory.0.join("mcp-audit"),
                )
                .expect("read-only binding"),
            ))
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
        let tenant = TenantId::new("tenant-a").expect("tenant");
        let mut store = SubscriptionStore::open(
            Store::open(root.join("subscriptions")).expect("durable subscriptions"),
            tenant,
        )
        .expect("subscription store");
        let scope = SubscriptionScope {
            tenant: layerx_agent_api::identity::TenantId::new("tenant-a").expect("tenant"),
            agent: AgentDid::new("did:layerx:model").expect("agent"),
            capability: layerx_agent_api::identity::CapabilityId::new("09".repeat(32))
                .expect("capability"),
        };
        let target = SubscriptionTarget {
            scope: scope.clone(),
            subscription_id: SubscriptionId::new("subscription-a").expect("id"),
        };
        store
            .create(
                target.subscription_id.clone(),
                SubscriptionCreate {
                    scope,
                    start: Cursor {
                        block_height: 0,
                        tx_index: 0,
                        log_index: 0,
                    },
                    delivery_target: DeliveryTarget::new("uds://consumer-a").expect("target"),
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
            )
            .expect("create subscription");
        store
            .mark_delivered(
                &target,
                layerx_agentd::events::subscription::Cursor {
                    block_height: 12,
                    tx_index: 3,
                    log_index: 4,
                },
            )
            .expect("deliver cursor");
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

fn rpc<B: ToolBoundary>(session: &mut Session<B>, method: &str, params: Value) -> Value {
    let input = json!({"jsonrpc":"2.0", "id":1, "method":method, "params": params});
    let mut source = std::io::Cursor::new(format!("{input}\n"));
    let mut output = Vec::new();
    session
        .serve(&mut source, &mut output)
        .expect("MCP line transport");
    serde_json::from_slice(&output).expect("MCP response")
}
fn names(value: &Value) -> BTreeSet<String> {
    value
        .as_array()
        .expect("tools")
        .iter()
        .map(|tool| tool["name"].as_str().expect("tool name").to_owned())
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
    let credential = EnvelopeCredential::new("tenant-a", [7; 32], fixture.credential.token_id(), 1)
        .expect("SDK credential");
    let wire = encode_envelope(
        layerx_agent_api::Operation::McpInvoke,
        RequestId(sequence),
        &json!({"tool":tool,"arguments":arguments}),
        Some(&credential),
        Some(layerx_agent_api::idempotency::Key::new(key).expect("key")),
    )
    .expect("SDK envelope producer");
    let bytes = serde_json::to_vec(&wire).expect("encode transport");
    let decoded = layerx_agentd::agent_rpc::decode_mcp_invocation(&bytes)
        .unwrap_or_else(|_| panic!("daemon envelope consumer"))
        .expect("MCP operation");
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
    result
        .release_current(HEIGHT, |value| Ok(value))
        .expect("authorized release")
        .expect("tool result")
}

pub fn session_tools_bind_describe_and_acknowledge() {
    for mixed in [false, true] {
        let mut scopes = vec!["subscribe", "approve"];
        if mixed {
            scopes.push("read:balance");
        }
        let fixture = Enrolled::new(&scopes, "did:layerx:model");
        let binding = Binding::parse(&fixture.binding_document("127.0.0.1:9", "full").to_string())
            .expect("binding document");
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
        let listed = rpc(&mut session, "tools/list", json!({}));
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
            assert!(listed["result"]["tools"]
                .as_array()
                .expect("list")
                .contains(&catalogue::listing(tool).expect("description")));
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
                json!({"name":"subscription.acknowledge", "arguments":invalid}),
            );
            assert_eq!(response["result"]["isError"], true, "{response}");
        }
        let response = rpc(
            &mut session,
            "tools/call",
            json!({"name":"subscription.acknowledge", "arguments":arguments}),
        );
        assert_eq!(response["result"]["isError"], false, "{response}");
        assert_eq!(
            response["result"]["structuredContent"]["result"]["subscriptions"][0]
                ["last_acknowledged"],
            arguments["cursor"]
        );
        drop(session);
        let recovered = SubscriptionStore::open(
            Store::open(fixture.directory.0.join("subscriptions"))
                .expect("reopen durable component"),
            TenantId::new("tenant-a").expect("tenant"),
        )
        .expect("restore component");
        assert_eq!(
            position(
                recovered
                    .get(&target)
                    .expect("recovered subscription")
                    .last_acknowledged
            ),
            arguments["cursor"]
        );
    }
}

pub fn session_tools_read_only_filtering() {
    for mixed in [false, true] {
        let mut scopes = vec!["subscribe", "approve"];
        if mixed {
            scopes.push("read:balance");
        }
        let fixture = Enrolled::new(&scopes, "did:layerx:model");
        let binding = Binding::parse(
            &fixture
                .binding_document("127.0.0.1:9", "read-only")
                .to_string(),
        )
        .expect("read-only binding");
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
        let listed = rpc(&mut session, "tools/list", json!({}));
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
        for tool in SESSION_TOOLS
            .iter()
            .filter(|tool| tool.kind == ToolKind::Write)
        {
            let refused = rpc(
                &mut session,
                "tools/call",
                json!({"name":tool.name,"arguments":{}}),
            );
            assert_eq!(refused["error"]["code"], -32602, "{}: {refused}", tool.name);
        }
        let listed = rpc(
            &mut session,
            "tools/call",
            json!({"name":"subscription.list", "arguments":{"scope":{"tenant":"tenant-a","agent":"did:layerx:model","capability":"09".repeat(32)}}}),
        );
        assert_eq!(listed["result"]["isError"], false, "{listed}");
        assert_eq!(
            listed["result"]["structuredContent"]["result"]["subscriptions"]
                .as_array()
                .expect("actual stored subscription")
                .len(),
            1
        );
    }
}

pub fn session_tools_authenticated_daemon_codec() {
    let fixture = Enrolled::new(&["subscribe", "approve"], "did:layerx:model");
    let binding = Binding::parse(&fixture.binding_document("127.0.0.1:9", "full").to_string())
        .expect("binding");
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
            Ok(boundary
                .execute(SESSION_TOOLS[6], arguments)
                .expect("actual store acknowledgement"))
        },
    );
    assert_eq!(
        output["subscriptions"][0]["last_acknowledged"],
        arguments["cursor"]
    );
    let forged = SessionCredential::new(
        TenantId::new("tenant-a").expect("tenant"),
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

#[path = "../approval.rs"]
mod approval_fixture;

struct SessionOwners {
    subscriptions: SubscriptionBoundary,
    approvals: ApprovalRegistry,
    limiter: BudgetLimiter,
    decisions: ApprovalExpiry,
    queue: layerx_agentd::approval::ApprovalSubmissionQueue,
}
impl SessionOwners {
    fn open(root: &Path) -> Self {
        let store_path = root.join("approval-holds");
        let store = Arc::new(Mutex::new(Store::open(&store_path).expect("hold store")));
        let approvals = ApprovalRegistry::with_store(Arc::clone(&store));
        let limiter = approval_fixture::limiter();
        for id in [19, 20] {
            approval_fixture::reserved_hold(&approvals, &limiter, id);
        }
        drop(approvals);
        drop(limiter);
        drop(store);
        let reopened = Arc::new(Mutex::new(Store::open(&store_path).expect("reopen holds")));
        let approvals = ApprovalRegistry::with_store(Arc::clone(&reopened));
        let limiter = approval_fixture::limiter();
        assert_eq!(
            approvals.replay_tenant(&approval_fixture::context(19).tenant, &limiter),
            Ok(2)
        );
        approvals
            .validate_registry(
                &approval_fixture::context(19).tenant,
                &layerx_types::payload::ModuleRegistry::new(&[]).expect("modules"),
            )
            .expect("recovered call and sender");
        Self {
            subscriptions: SubscriptionBoundary::open(root),
            approvals,
            limiter,
            decisions: ApprovalExpiry::from_shared_store(reopened),
            queue: Default::default(),
        }
    }
}
impl ToolBoundary for SessionOwners {
    fn execute(
        &mut self,
        tool: ToolDefinition,
        arguments: &Value,
    ) -> Result<Value, BoundaryRefusal> {
        use layerx_agentd::approval::{ApprovalOutcome, DecisionKey, DecisionRequest};
        use layerx_agentd::policy::approval::ApproverId;
        match tool.name {
            "subscription.create" => {
                assert_eq!(arguments["scope"], self.subscriptions.arguments()["scope"]);
                assert_eq!(
                    arguments["filter"],
                    json!({"agents":[],"accounts":[],"activity_types":[],"modules":[],"assets":[],"counterparties":[],"result_classes":[]})
                );
                let cursor = &arguments["start"];
                let request = SubscriptionCreate {
                    scope: self.subscriptions.target.scope.clone(),
                    start: Cursor {
                        block_height: cursor["block_height"]
                            .as_str()
                            .expect("height")
                            .parse()
                            .expect("height integer"),
                        tx_index: cursor["tx_index"]
                            .as_str()
                            .expect("transaction")
                            .parse()
                            .expect("transaction integer"),
                        log_index: cursor["log_index"]
                            .as_str()
                            .expect("log")
                            .parse()
                            .expect("log integer"),
                    },
                    delivery_target: DeliveryTarget::new(
                        arguments["delivery_target"].as_str().expect("delivery"),
                    )
                    .expect("target"),
                    filter: SubscriptionFilter {
                        agents: ExplicitSet::deny_all(),
                        accounts: ExplicitSet::deny_all(),
                        activity_types: ExplicitSet::deny_all(),
                        modules: ExplicitSet::deny_all(),
                        assets: ExplicitSet::deny_all(),
                        counterparties: ExplicitSet::deny_all(),
                        result_classes: ExplicitSet::deny_all(),
                    },
                };
                let record = self
                    .subscriptions
                    .store
                    .create(
                        SubscriptionId::new("created-on-transport").expect("id"),
                        request,
                    )
                    .expect("real subscription creation");
                Ok(subscription_result(&record))
            }
            "subscription.list" | "subscription.acknowledge" => {
                self.subscriptions.execute(tool, arguments)
            }
            "subscription.health" => Ok(subscription_result(
                &self
                    .subscriptions
                    .store
                    .get(&self.subscriptions.target)
                    .expect("subscription health"),
            )),
            "subscription.pause" => Ok(subscription_result(
                &self
                    .subscriptions
                    .store
                    .pause(&self.subscriptions.target)
                    .expect("pause"),
            )),
            "subscription.resume" => Ok(subscription_result(
                &self
                    .subscriptions
                    .store
                    .resume(&self.subscriptions.target)
                    .expect("resume"),
            )),
            "subscription.delete" => {
                self.subscriptions
                    .store
                    .delete(&self.subscriptions.target)
                    .expect("delete");
                assert!(self
                    .subscriptions
                    .store
                    .get(&self.subscriptions.target)
                    .is_err());
                Ok(json!({"deleted":true}))
            }
            "approval.list" | "approval.get" | "approval.approve" | "approval.reject" => {
                let tenant = approval_fixture::context(19).tenant;
                let service = ApprovalService::new(&self.approvals, &self.limiter, &self.decisions);
                match tool.name {
                    "approval.list" => {
                        let page = service
                            .list(
                                &tenant,
                                None,
                                100,
                                HEIGHT,
                                layerx_agent_api::TimestampSeconds(
                                    test_clock::TestClock::new(HEIGHT).now(),
                                ),
                            )
                            .expect("real approval list");
                        Ok(
                            json!({"approval_ids":page.approvals.iter().map(|r| hex(&r.approval_id)).collect::<Vec<_>>()}),
                        )
                    }
                    "approval.get" => {
                        catalogue::approval_get_tool(&service, &tenant, &BTreeMap::new(), arguments)
                    }
                    name => {
                        let id = if name == "approval.approve" { 19 } else { 20 };
                        assert_eq!(arguments["approval_id"], hex(&[id; 32]));
                        let ticket = self
                            .approvals
                            .ticket([id; 32])
                            .expect("ticket")
                            .expect("held call");
                        assert_eq!(arguments["held_digest"], hex(&ticket.disclosure_digest));
                        let key = DecisionKey::new(format!("transport-{id}")).expect("key");
                        let request = DecisionRequest {
                            tenant: &tenant,
                            approval_id: [id; 32],
                            idempotency_key: &key,
                            approver: ApproverId::new("human:reviewer").expect("approver"),
                            current_sequence: HEIGHT,
                            current_time: layerx_agent_api::TimestampSeconds(
                                test_clock::TestClock::new(HEIGHT)
                                    .sample(std::time::Duration::from_secs(1))
                                    .expect("clock")
                                    .unix_seconds(),
                            ),
                        };
                        let decision = if id == 19 {
                            service
                                .approve(request, &approval_fixture::prepared(101, id), &self.queue)
                                .expect("real approval")
                        } else {
                            service.reject(request).expect("real rejection")
                        };
                        assert_eq!(
                            decision.outcome,
                            if id == 19 {
                                ApprovalOutcome::Granted
                            } else {
                                ApprovalOutcome::Rejected
                            }
                        );
                        if id == 19 {
                            assert_eq!(
                                self.queue
                                    .prepared(decision.submission_ref.expect("release")),
                                Some(approval_fixture::prepared(101, id))
                            );
                        }
                        Ok(
                            json!({"approval_id":hex(&[id;32]), "state":format!("{:?}",decision.outcome)}),
                        )
                    }
                }
            }
            _ => Err(BoundaryRefusal::NotServed(tool.name)),
        }
    }
    fn observed_sequence(&mut self) -> Result<u64, BoundaryRefusal> {
        Ok(HEIGHT)
    }
}

pub fn all_session_tools_through_daemon_and_mcp() {
    let fixture = Enrolled::new(&["subscribe", "approve"], "did:layerx:model");
    let binding = Binding::parse(&fixture.binding_document("127.0.0.1:9", "full").to_string())
        .expect("binding");
    let description = daemon_call(
        &fixture,
        &binding,
        "mcp.describe",
        &json!({}),
        |_, _, _, _| panic!("description is daemon-owned"),
    );
    let first_root = fixture.directory.0.join("daemon");
    let second_root = fixture.directory.0.join("mcp");
    fs::create_dir(&first_root).expect("daemon owners");
    fs::create_dir(&second_root).expect("MCP owners");
    let mut daemon = SessionOwners::open(&first_root);
    let mcp = SessionOwners::open(&second_root);
    let arguments = daemon.subscriptions.arguments();
    let target = json!({"scope":arguments["scope"],"subscription_id":arguments["subscription_id"]});
    let filter = json!({"agents":[],"accounts":[],"activity_types":[],"modules":[],"assets":[],"counterparties":[],"result_classes":[]});
    let create = json!({"scope":arguments["scope"],"filter":filter,"start":{"block_height":"0","tx_index":"0","log_index":"0"},"delivery_target":"uds://consumer-a"});
    let mut session = Session::new(fixture.bound(DeploymentMode::Full), mcp);
    let listed = rpc(&mut session, "tools/list", json!({}));
    assert_eq!(
        names(&description["tools"]),
        names(&listed["result"]["tools"])
    );
    let mut exercised = BTreeSet::new();
    for (name, args) in [
        ("subscription.create", create),
        ("subscription.list", json!({"scope":arguments["scope"]})),
        ("subscription.health", target.clone()),
        ("subscription.acknowledge", arguments.clone()),
        ("subscription.pause", target.clone()),
        ("subscription.resume", target.clone()),
        ("subscription.delete", target),
        (
            "approval.list",
            json!({"current_sequence":"50","limit":"100"}),
        ),
        (
            "approval.get",
            json!({"approval_id":hex(&[19;32]),"current_sequence":"50", "current_time":test_clock::TestClock::new(50).now().to_string()}),
        ),
        (
            "approval.approve",
            json!({"approval_id":hex(&[19;32]),"current_sequence":"50","held_digest":hex(&daemon.approvals.ticket([19;32]).expect("ticket").expect("hold").disclosure_digest)}),
        ),
        (
            "approval.reject",
            json!({"approval_id":hex(&[20;32]),"current_sequence":"50","held_digest":hex(&daemon.approvals.ticket([20;32]).expect("ticket").expect("hold").disclosure_digest)}),
        ),
    ] {
        let tool = *SESSION_TOOLS
            .iter()
            .find(|tool| tool.name == name)
            .expect("declared tool");
        assert!(names(&description["tools"]).contains(name));
        let expected = daemon_call(&fixture, &binding, name, &args, |_, credential, args, _| {
            assert_eq!(credential, &fixture.credential);
            Ok(daemon.execute(tool, args).expect("real daemon owner"))
        });
        let response = rpc(
            &mut session,
            "tools/call",
            json!({"name":name,"arguments":args}),
        );
        assert_eq!(response["result"]["isError"], false, "{name}: {response}");
        assert_eq!(
            response["result"]["structuredContent"]["result"], expected,
            "{name}"
        );
        if name == "subscription.acknowledge" {
            assert_eq!(
                expected["subscriptions"][0]["last_acknowledged"],
                arguments["cursor"]
            );
        }
        if name == "approval.get" {
            assert_eq!(expected["approval_id"], hex(&[19; 32]));
            assert_eq!(expected["state"], "AwaitingApproval");
            for field in ["tx_hash", "rung"] {
                assert!(expected.as_object().expect("facts").contains_key(field));
                assert!(expected[field].is_null());
            }
            for field in [
                "activity_id",
                "resulting_activity_id",
                "verification_level",
                "level",
            ] {
                assert!(expected.get(field).is_none());
            }
        }
        exercised.insert(name);
    }
    assert_eq!(
        exercised,
        SESSION_TOOLS.iter().map(|tool| tool.name).collect()
    );
    drop(session);
    for root in ["daemon-audit", "mcp-audit"] {
        let audit = layerx_agentd::audit::Log::open(
            fixture.directory.0.join(root),
            &TenantId::new("tenant-a").expect("tenant"),
        )
        .expect("reopen and verify transport audit chain");
        assert_eq!(
            audit.entries(),
            2 * SESSION_TOOLS.len() as u64,
            "{root} must retain an attempt and completion for every session tool"
        );
    }
}

fn subscription_result(record: &layerx_agent_api::subscription::SubscriptionRecord) -> Value {
    json!({"subscription_id":record.subscription_id.as_str(),"last_acknowledged":position(record.last_acknowledged),"paused":record.paused})
}

pub fn approval_result_over_transport(
    service: &ApprovalService<'_>,
    tenant: &TenantId,
    submissions: &BTreeMap<[u8; 32], layerx_agent_api::track::TrackedSubmission>,
    arguments: &Value,
    expected: &Value,
) {
    struct ApprovalOwner<'a, 'b> {
        service: &'a ApprovalService<'b>,
        tenant: &'a TenantId,
        submissions: &'a BTreeMap<[u8; 32], layerx_agent_api::track::TrackedSubmission>,
    }
    impl ToolBoundary for ApprovalOwner<'_, '_> {
        fn execute(
            &mut self,
            tool: ToolDefinition,
            args: &Value,
        ) -> Result<Value, BoundaryRefusal> {
            assert_eq!(tool.name, "approval.get");
            catalogue::approval_get_tool(self.service, self.tenant, self.submissions, args)
        }
        fn observed_sequence(&mut self) -> Result<u64, BoundaryRefusal> {
            Ok(HEIGHT)
        }
    }
    let fixture = Enrolled::new(
        &["approve"],
        expected["actor"].as_str().expect("held actor"),
    );
    let binding = Binding::parse(&fixture.binding_document("127.0.0.1:9", "full").to_string())
        .expect("approval binding");
    let mut owner = ApprovalOwner {
        service,
        tenant,
        submissions,
    };
    let daemon = daemon_call(
        &fixture,
        &binding,
        "approval.get",
        arguments,
        |operation, credential, args, _| {
            assert_eq!(operation, layerx_agentd::tenant::Operation::ApprovalGet);
            assert_eq!(credential, &fixture.credential);
            Ok(owner
                .execute(SESSION_TOOLS[8], args)
                .expect("approval owner"))
        },
    );
    assert_eq!(&daemon, expected);
    let mut session = Session::new(fixture.bound(DeploymentMode::Full), owner);
    let described = daemon_call(
        &fixture,
        &binding,
        "mcp.describe",
        &json!({}),
        |_, _, _, _| panic!("daemon description"),
    );
    let listed = rpc(&mut session, "tools/list", json!({}));
    assert_eq!(
        names(&described["tools"]),
        names(&listed["result"]["tools"])
    );
    assert_eq!(
        names(&listed["result"]["tools"]),
        SESSION_TOOLS
            .iter()
            .filter(|t| t.required_scope == "approve")
            .map(|t| t.name.to_owned())
            .collect()
    );
    let response = rpc(
        &mut session,
        "tools/call",
        json!({"name":"approval.get","arguments":arguments}),
    );
    assert_eq!(response["result"]["isError"], false, "{response}");
    assert_eq!(&response["result"]["structuredContent"]["result"], expected);
    for field in [
        "activity_id",
        "resulting_activity_id",
        "verification_level",
        "level",
    ] {
        assert!(expected.get(field).is_none());
    }
}
