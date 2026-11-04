//! Task 22.33.1552 repair record.
//! connect_deadline: connect_deadline_backlog, connect_deadline_stalled_response,
//! connect_deadline_trickled_response.
//! hex_length_validation: hex32_tools_call_rejects_before_daemon,
//! hex64_tools_call_rejects_before_authenticated_codec.
//! approval_page_bound: approval_page_bound_authenticated_transport.
//! listener_monopolisation: listener_monopolisation_active_connection_yields.
//! No proposal requires out-of-scope wiring.
//! Baseline: no prior gate ledger or repair test existed; the ordered implementation-first,
//! wrapper-only gate instructions preclude the conflicting additional baseline test run.
//! Attempt 1: 5d7dbb8964d5e13b338899173cf38d8be1564a22; hf-build passed;
//! hf-done exited 1 (/out/logs/verify-1.log). Six cases passed; the approval fixture
//! failed and the nested write fixture deadlocked, so the stalled test process was stopped.
//! Repair: keep the authenticated approval dispatch audit separate from the calling session;
//! let the write peer consume its signature with the real codec after the session's write gate.
//! Final revision and private evidence are recorded by hf-build/hf-done.

use std::collections::BTreeSet;
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::{MetadataExt, PermissionsExt};
use std::os::unix::net::UnixStream;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, AtomicUsize, Ordering};
use std::sync::{mpsc, Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant};

use ed25519_dalek::{Signature, Signer, SigningKey};
use layerx_agent_api::error::RequestId;
use layerx_agentd::approval::{ApprovalExpiry, ApprovalOperationError, ApprovalService};
use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope};
use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::policy::approval::ApprovalRegistry;
use layerx_agentd::prepare::PreparationLifecycle;
use layerx_agentd::session::{open, OpenRequest, SessionCredential, SessionId, SessionRegistry};
use layerx_agentd::session_control::{McpInvocationIdentity, SessionControl};
use layerx_agentd::store::{Store, TenantId};
use layerx_agentd::tenant::Operation;
use layerx_mcp::binding::Binding;
use layerx_mcp::boundary::{AgentSurface, BoundaryRefusal, ProgramReads, ToolBoundary};
use layerx_mcp::catalogue;
use layerx_mcp::listener::{Listener, ListenerConfig};
use layerx_mcp::server::{DeploymentMode, ReadOnly, Server, ToolDefinition};
use layerx_mcp::stdio::{Bound, Session};
use layerx_sdk::agent_envelope::{encode_envelope, EnvelopeCredential};
use layerx_types::ids::Did;
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};

static NEXT: AtomicU64 = AtomicU64::new(1);
const HEIGHT: u64 = 50;
const DEADLINE: Duration = Duration::from_millis(150);
const SCHEDULING_MARGIN: Duration = Duration::from_millis(100);

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}
fn unhex(value: &Value) -> Vec<u8> {
    let text = value.as_str().expect("hex string");
    (0..text.len())
        .step_by(2)
        .map(|offset| u8::from_str_radix(&text[offset..offset + 2], 16).expect("hex byte"))
        .collect()
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let root = fs::canonicalize(std::env::temp_dir())
            .expect("temporary root")
            .join(format!(
                "repair-22-33-1552-{}-{}",
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
            "audit_root": self.directory.0.join("owner-audit"), "session_id": "07".repeat(32),
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
                    self.directory.0.join("audit"),
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
                    self.directory.0.join("audit"),
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
fn rpc<B: ToolBoundary>(session: &mut Session<B>, tool: &str, arguments: Value) -> Value {
    let input = json!({"jsonrpc":"2.0", "id":1, "method":"tools/call",
        "params":{"name":tool,"arguments":arguments}});
    let mut source = std::io::Cursor::new(format!("{input}\n"));
    let mut output = Vec::new();
    session
        .serve(&mut source, &mut output)
        .expect("MCP line transport");
    serde_json::from_slice(&output).expect("MCP response")
}

fn surface(endpoint: &str) -> AgentSurface {
    AgentSurface::new(endpoint, "b".repeat(48), &"cc".repeat(32), DEADLINE)
        .expect("loopback surface")
}

fn assert_unavailable_by_deadline(surface: &AgentSurface) {
    let started = Instant::now();
    let result = surface.native_read(&format!("/v1/reads/receipt/{}", "11".repeat(32)));
    let elapsed = started.elapsed();
    assert!(
        matches!(result, Err(BoundaryRefusal::Unavailable(_))),
        "{result:?}"
    );
    assert!(
        elapsed < DEADLINE + SCHEDULING_MARGIN,
        "deadline {DEADLINE:?}, elapsed {elapsed:?}"
    );
}

#[test]
fn connect_deadline_backlog() {
    let listener = TcpListener::bind("127.0.0.1:0").expect("backlog listener");
    rustix::net::listen(&listener, 0).expect("one queued connection");
    let address = listener.local_addr().expect("address");
    let queued = TcpStream::connect_timeout(&address, DEADLINE).expect("fill backlog");
    let saturated = TcpStream::connect_timeout(&address, Duration::from_millis(40));
    assert!(
        matches!(saturated, Err(ref error) if error.kind() == std::io::ErrorKind::TimedOut),
        "backlog must be saturated: {saturated:?}"
    );
    assert_unavailable_by_deadline(&surface(&address.to_string()));
    drop(queued);
}

fn read_http(stream: &mut TcpStream) -> String {
    stream
        .set_read_timeout(Some(Duration::from_secs(2)))
        .expect("fixture deadline");
    let mut bytes = Vec::new();
    let mut byte = [0];
    while !bytes.ends_with(b"\r\n\r\n") {
        stream.read_exact(&mut byte).expect("HTTP request");
        bytes.push(byte[0]);
        assert!(bytes.len() < 8192, "bounded request");
    }
    let text = String::from_utf8(bytes).expect("HTTP text");
    assert!(text.contains(&format!("Authorization: Bearer {}\r\n", "b".repeat(48))));
    text
}

fn delayed_response(trickle: bool) {
    let listener = TcpListener::bind("127.0.0.1:0").expect("response listener");
    let endpoint = listener.local_addr().expect("address").to_string();
    let (stop, stopped) = mpsc::channel();
    let peer = thread::spawn(move || {
        let (mut stream, _) = listener.accept().expect("accepted request");
        read_http(&mut stream);
        if trickle {
            stream
                .write_all(b"HTTP/1.1 200 OK\r\n\r\n{")
                .expect("response prefix");
            for _ in 0..40 {
                match stopped.recv_timeout(Duration::from_millis(20)) {
                    Ok(()) | Err(mpsc::RecvTimeoutError::Disconnected) => return,
                    Err(mpsc::RecvTimeoutError::Timeout) => (),
                }
                if stream.write_all(b" ").is_err() {
                    return;
                }
            }
        } else {
            let _ = stopped.recv_timeout(Duration::from_secs(2));
        }
    });
    assert_unavailable_by_deadline(&surface(&endpoint));
    stop.send(()).expect("stop fixture");
    peer.join().expect("response fixture");
}

#[test]
fn connect_deadline_stalled_response() {
    delayed_response(false);
}

#[test]
fn connect_deadline_trickled_response() {
    delayed_response(true);
}

#[test]
fn connect_deadline_requires_parsed_loopback() {
    for endpoint in [
        "127.0.0.1:bad",
        "127.0.0.1:1/extra",
        "localhost:123",
        "192.0.2.1:123",
    ] {
        assert!(matches!(
            AgentSurface::new(endpoint, "b".repeat(48), &"cc".repeat(32), DEADLINE),
            Err(BoundaryRefusal::Malformed(_))
        ));
    }
}

#[test]
fn hex32_tools_call_rejects_before_daemon() {
    let fixture = Enrolled::new(&["read:receipt", "read:balance"], "did:layerx:model");
    let listener = TcpListener::bind("127.0.0.1:0").expect("native reads listener");
    let endpoint = listener.local_addr().expect("address").to_string();
    let paths = Arc::new(Mutex::new(Vec::new()));
    let observed = Arc::clone(&paths);
    // Two successful calls each obtain freshness and then read their requested evidence.
    let peer = thread::spawn(move || {
        for _ in 0..4 {
            let (mut stream, _) = listener.accept().expect("read connection");
            let request = read_http(&mut stream);
            let path = request
                .split_ascii_whitespace()
                .nth(1)
                .expect("request path")
                .to_owned();
            observed.lock().expect("paths").push(path.clone());
            let body = if path.starts_with("/v1/programs/") {
                json!({"program":"cc".repeat(32), "lifecycle":"Active", "accounts":[],
                    "freshness":{"observed_sequence":HEIGHT}})
            } else {
                assert_eq!(path, format!("/v1/reads/receipt/{}", "11".repeat(32)));
                json!({"activity_id":"11".repeat(32), "evidence":"receipt recording"})
            };
            let body = body.to_string();
            write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                body.len()
            )
            .expect("native response");
        }
    });
    let mut session = Session::new(
        fixture.bound(DeploymentMode::Full),
        ProgramReads::new(surface(&endpoint)),
    );
    for (tool, field) in [("receipt.get", "activity_id"), ("balance.get", "program")] {
        for text in [
            "11".to_owned(),
            "11".repeat(31),
            "11".repeat(33),
            "gg".repeat(32),
        ] {
            let response = rpc(&mut session, tool, json!({(field):text}));
            assert_eq!(response["result"]["isError"], true, "{response}");
            assert_eq!(
                response["result"]["structuredContent"]["stage"],
                "arguments"
            );
            assert!(
                paths.lock().expect("paths").is_empty(),
                "malformed request reached daemon"
            );
        }
    }
    let receipt = rpc(
        &mut session,
        "receipt.get",
        json!({"activity_id":"11".repeat(32)}),
    );
    assert_eq!(receipt["result"]["isError"], false, "{receipt}");
    assert_eq!(
        receipt["result"]["structuredContent"]["result"]["activity_id"],
        "11".repeat(32)
    );
    let balance = rpc(
        &mut session,
        "balance.get",
        json!({"program":"cc".repeat(32)}),
    );
    assert_eq!(balance["result"]["isError"], false, "{balance}");
    peer.join().expect("native reads producer");
    assert_eq!(paths.lock().expect("paths").len(), 4);
}

// This fixture connects the production MCP session to the SDK envelope producer, daemon
// decoder, authenticated binding and actual approval store or signature codec consumer.
struct OwnerTransport<'a> {
    fixture: &'a Enrolled,
    binding: Binding,
    service: Option<ApprovalService<'a>>,
    calls: Arc<AtomicUsize>,
}
impl ToolBoundary for OwnerTransport<'_> {
    fn observed_sequence(&mut self) -> Result<u64, BoundaryRefusal> {
        Ok(HEIGHT)
    }
    fn execute(
        &mut self,
        tool: ToolDefinition,
        arguments: &Value,
    ) -> Result<Value, BoundaryRefusal> {
        let sequence = NEXT.fetch_add(1, Ordering::Relaxed);
        let mut key = [1; 32];
        key[..8].copy_from_slice(&sequence.to_be_bytes());
        let credential =
            EnvelopeCredential::new("tenant-a", [7; 32], self.fixture.credential.token_id(), 1)
                .expect("SDK credential");
        let wire = encode_envelope(
            layerx_agent_api::Operation::McpInvoke,
            RequestId(sequence),
            &json!({"tool":tool.name,"arguments":arguments}),
            Some(&credential),
            Some(layerx_agent_api::idempotency::Key::new(key).expect("key")),
        )
        .expect("SDK envelope producer");
        let bytes = serde_json::to_vec(&wire).expect("wire bytes");
        let listener = TcpListener::bind("127.0.0.1:0").expect("authenticated owner listener");
        let address = listener.local_addr().expect("owner address");
        thread::scope(|scope| {
            let server = scope.spawn(|| {
                let (mut stream, _) = listener.accept().expect("owner accepted transport");
                stream
                    .set_read_timeout(Some(Duration::from_secs(2)))
                    .expect("owner read bound");
                stream
                    .set_write_timeout(Some(Duration::from_secs(2)))
                    .expect("owner write bound");
                let mut body = Vec::new();
                (&mut stream)
                    .take(1_048_577)
                    .read_to_end(&mut body)
                    .expect("owner envelope bytes");
                let value = self.consume(tool, &body).expect("real owner consumer");
                serde_json::to_writer(&mut stream, &value).expect("owner response");
            });
            let mut stream = TcpStream::connect_timeout(&address, Duration::from_secs(2))
                .expect("owner transport");
            stream
                .set_read_timeout(Some(Duration::from_secs(2)))
                .expect("response bound");
            stream
                .set_write_timeout(Some(Duration::from_secs(2)))
                .expect("request bound");
            stream.write_all(&bytes).expect("SDK envelope transport");
            stream
                .shutdown(std::net::Shutdown::Write)
                .expect("complete request");
            let mut response = Vec::new();
            stream
                .read_to_end(&mut response)
                .expect("owner result transport");
            server.join().expect("owner server");
            Ok(serde_json::from_slice(&response).expect("transport result consumer"))
        })
    }
}
impl OwnerTransport<'_> {
    fn consume(&self, tool: ToolDefinition, bytes: &[u8]) -> Result<Value, BoundaryRefusal> {
        self.calls.fetch_add(1, Ordering::SeqCst);
        let decoded = layerx_agentd::agent_rpc::decode_mcp_invocation(bytes)
            .unwrap_or_else(|_| panic!("production daemon decoder"))
            .expect("MCP operation");
        assert_eq!(decoded.credential.tenant().as_str(), "tenant-a");
        assert_eq!(
            decoded.credential.token_id(),
            self.fixture.credential.token_id()
        );
        if tool.name == "activity.submit" {
            let key: [u8; 32] = unhex(&decoded.arguments["signer_public_key"])
                .try_into()
                .expect("public key width");
            let signature = Signature::from_slice(&unhex(&decoded.arguments["signature"]))
                .expect("real signature codec consumes exactly 64 bytes");
            ed25519_dalek::VerifyingKey::from_bytes(&key)
                .expect("public key")
                .verify_strict(b"catalogue transport signature", &signature)
                .expect("verified signature");
            return Ok(json!({"verified":true}));
        }
        let result = self.binding.daemon_invocation(
            self.fixture.control.clone(), decoded.credential, HEIGHT, &decoded.tool,
            &decoded.arguments,
            McpInvocationIdentity { tool_name:tool.name, idempotency_key:decoded.idempotency_key,
                request_digest:Sha256::digest(bytes).into() },
            None,
            |operation, credential, request, _| {
                assert_eq!(credential.tenant().as_str(), "tenant-a");
                assert_eq!(credential.token_id(), self.fixture.credential.token_id());
                match operation {
                    Operation::ApprovalList => {
                        let service = self.service.as_ref().expect("real approval service");
                        let limit = request["limit"].as_str().expect("limit").parse().expect("page limit");
                        let page = service.list(credential.tenant(), None, limit, HEIGHT)
                            .expect("owning service accepts schema maximum");
                        Ok(json!({"approvals": page.approvals.iter().map(|record| json!({"approval_id":hex(&record.approval_id)})).collect::<Vec<_>>(),
                            "next_cursor": page.next_cursor.map(|id| hex(&id))}))
                    }
                    _ => panic!("unexpected owner operation {operation:?}"),
                }
            })
            .unwrap_or_else(|error| panic!("authenticated daemon invocation: {error:?}"));
        result
            .release_current(HEIGHT, |value| Ok(value))
            .expect("authorized result release")
    }
}

#[test]
fn hex64_tools_call_rejects_before_authenticated_codec() {
    let fixture = Enrolled::new(&["write:submit"], "did:layerx:model");
    let calls = Arc::new(AtomicUsize::new(0));
    let boundary = OwnerTransport {
        fixture: &fixture,
        binding: Binding::parse(&fixture.binding_document("127.0.0.1:9", "full").to_string())
            .expect("binding"),
        service: None,
        calls: Arc::clone(&calls),
    };
    let mut session = Session::new(fixture.bound(DeploymentMode::Full), boundary);
    let signing_key = SigningKey::from_bytes(&[27; 32]);
    let signature = signing_key.sign(b"catalogue transport signature");
    let mut arguments = json!({"preparation_ref":"codec-preparation",
        "signature":hex(&signature.to_bytes()), "signer_public_key":hex(signing_key.verifying_key().as_bytes())});
    for text in [
        "11".to_owned(),
        "11".repeat(63),
        "11".repeat(65),
        "gg".repeat(64),
    ] {
        let mut invalid = arguments.clone();
        invalid["signature"] = json!(text);
        let response = rpc(&mut session, "activity.submit", invalid);
        assert_eq!(response["result"]["isError"], true, "{response}");
        assert_eq!(
            response["result"]["structuredContent"]["stage"],
            "arguments"
        );
        assert_eq!(
            calls.load(Ordering::SeqCst),
            0,
            "malformed signature reached daemon"
        );
    }
    let response = rpc(&mut session, "activity.submit", arguments.take());
    assert_eq!(response["result"]["isError"], false, "{response}");
    assert_eq!(
        response["result"]["structuredContent"]["result"]["verified"],
        true
    );
    assert_eq!(calls.load(Ordering::SeqCst), 1);
    // Variable byte strings retain their own bounds, independently of fixed identifiers.
    for count in [1, 31, 32, 64, 65] {
        catalogue::validate(
            "activity.disclose",
            &json!({"canonical_bytes":"ab".repeat(count)}),
        )
        .expect("variable byte string");
    }
}

#[test]
fn approval_page_bound_authenticated_transport() {
    let fixture = Enrolled::new(&["approve"], "did:layerx:model");
    let registry = ApprovalRegistry::default();
    let limiter = BudgetLimiter::new(Vec::new()).expect("approval limiter");
    let expiry = ApprovalExpiry::open(fixture.directory.0.join("approval-decisions"))
        .expect("approval store");
    let service = ApprovalService::new(&registry, &limiter, &expiry);
    assert!(matches!(
        service.list(
            &TenantId::new("tenant-a").expect("tenant"),
            None,
            101,
            HEIGHT
        ),
        Err(ApprovalOperationError::InvalidPageLimit)
    ));
    let calls = Arc::new(AtomicUsize::new(0));
    let boundary = OwnerTransport {
        fixture: &fixture,
        binding: Binding::parse(&fixture.binding_document("127.0.0.1:9", "full").to_string())
            .expect("binding"),
        service: Some(service),
        calls: Arc::clone(&calls),
    };
    let mut session = Session::new(fixture.bound(DeploymentMode::Full), boundary);
    let schema = catalogue::input_schema("approval.list").expect("approval schema");
    assert_eq!(
        schema["oneOf"][0]["properties"]["limit"]["layerx/maximum"],
        100
    );
    let response = rpc(
        &mut session,
        "approval.list",
        json!({"current_sequence":HEIGHT.to_string(),"limit":"100"}),
    );
    assert_eq!(response["result"]["isError"], false, "{response}");
    assert_eq!(
        response["result"]["structuredContent"]["result"]["approvals"],
        json!([])
    );
    assert_eq!(calls.load(Ordering::SeqCst), 1);
    let response = rpc(
        &mut session,
        "approval.list",
        json!({"current_sequence":HEIGHT.to_string(),"limit":"101"}),
    );
    assert_eq!(response["result"]["isError"], true, "{response}");
    assert_eq!(
        response["result"]["structuredContent"]["stage"],
        "arguments"
    );
    assert_eq!(calls.load(Ordering::SeqCst), 1, "limit 101 reached daemon");
}

fn ping(
    writer: &mut UnixStream,
    reader: &mut BufReader<UnixStream>,
    id: u64,
) -> std::io::Result<Value> {
    writeln!(
        writer,
        "{}",
        json!({"jsonrpc":"2.0", "id":id, "method":"ping"})
    )?;
    writer.flush()?;
    let mut response = String::new();
    if reader.read_line(&mut response)? == 0 {
        return Err(std::io::Error::from(std::io::ErrorKind::UnexpectedEof));
    }
    Ok(serde_json::from_str(&response).expect("ping response"))
}

#[test]
fn listener_monopolisation_active_connection_yields() {
    let fixture = Enrolled::new(&["read:receipt"], "did:layerx:model");
    let endpoint = fixture.directory.0.join("mcp.sock");
    let metadata = fs::metadata(&fixture.directory.0).expect("directory metadata");
    let lifetime = Duration::from_millis(250);
    let listener = Listener::bind(ListenerConfig {
        endpoint: endpoint.clone(),
        owner_uid: metadata.uid(),
        owner_gid: metadata.gid(),
        mode: 0o600,
        admitted_uids: vec![metadata.uid()],
        deadline: lifetime,
    })
    .expect("admitted listener");
    let mut session = Session::new(
        fixture.bound(DeploymentMode::Full),
        ProgramReads::new(surface("127.0.0.1:9")),
    );
    thread::spawn(move || {
        let _ = listener.serve(&mut session);
    });
    let mut first = UnixStream::connect(&endpoint).expect("first client");
    first
        .set_read_timeout(Some(Duration::from_secs(2)))
        .expect("client bound");
    let mut reader = BufReader::new(first.try_clone().expect("reader"));
    assert_eq!(
        ping(&mut first, &mut reader, 1).expect("first admitted ping")["id"],
        1
    );
    let (finished, hold_open) = mpsc::channel();
    let (closed, observed_close) = mpsc::channel();
    let active = thread::spawn(move || {
        let mut count = 1;
        for id in 2..100 {
            thread::sleep(Duration::from_millis(15));
            if ping(&mut first, &mut reader, id).is_err() {
                closed.send(count).expect("connection expiry observation");
                // The peer retains its connection until the second client has been served.
                hold_open
                    .recv_timeout(Duration::from_secs(2))
                    .expect("second client served");
                return;
            }
            count += 1;
        }
        panic!("active connection monopolised listener");
    });
    let mut second = UnixStream::connect(&endpoint).expect("second admitted client");
    second
        .set_read_timeout(Some(lifetime + SCHEDULING_MARGIN))
        .expect("service bound");
    let mut second_reader = BufReader::new(second.try_clone().expect("second reader"));
    let started = Instant::now();
    let response =
        ping(&mut second, &mut second_reader, 999).expect("second client must receive service");
    assert_eq!(response["id"], 999);
    assert!(started.elapsed() < lifetime + SCHEDULING_MARGIN);
    assert!(
        observed_close
            .recv_timeout(SCHEDULING_MARGIN)
            .expect("first connection expired")
            >= 3
    );
    finished.send(()).expect("release first client socket");
    active.join().expect("active client");
}
