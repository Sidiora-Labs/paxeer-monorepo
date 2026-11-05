use std::collections::BTreeSet;
use std::fmt::Debug;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_agentd::agent_rpc::{self, AgentRpcResponse, Envelope};
use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope};
use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
use layerx_agentd::enrolment::{
    self, readiness_tls as tls, BindingMode, BindingPublisher, DaemonSurface, EnrolmentRequest,
    ReadinessDeclaration,
};
use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::prepare::PreparationLifecycle;
use layerx_agentd::session::{SessionCredential, SessionId, SessionRegistry};
use layerx_agentd::session_control::{McpInvocationIdentity, SessionControl};
use layerx_agentd::store::{Store, TenantId};
use layerx_agentd::tenant::{
    normalize_error, BoundedMetrics, InternalError, ObjectOwner, Operation, Surface,
};
use layerx_mcp::binding::Binding;
use layerx_mcp::server::{Server, ServerError};
use layerx_types::ids::Did;
use layerx_types::verify::VerificationLevel;

use super::{fail, tenant, IsolationFailure, Workspace};

const SEQUENCE: u64 = 20;
const DEADLINE: Duration = Duration::from_secs(5);
const CERTIFICATE: &[u8] =
    include_bytes!("../../../crates/layerx-sdk/src/repair_22_31_227_fixture/server.der");
const PRIVATE_KEY: &[u8] =
    include_bytes!("../../../crates/layerx-sdk/src/repair_22_31_227_fixture/server-key.der");
// SHA-256 of the canonical arguments of mcp.describe, the empty JSON object.
const DESCRIPTION_DIGEST: [u8; 32] = [
    0x44, 0x13, 0x6f, 0xa3, 0x55, 0xb3, 0x67, 0x8a, 0x11, 0x46, 0xad, 0x16, 0xf7, 0xe8, 0x64, 0x9e,
    0x94, 0xfb, 0x4f, 0xc2, 0x1f, 0xe7, 0x7e, 0x83, 0x10, 0xc0, 0x60, 0xf6, 0x1c, 0xaa, 0xff, 0x8a,
];

fn checked<T, E: Debug>(value: Result<T, E>, context: &str) -> Result<T, IsolationFailure> {
    value.map_err(|error| fail(format!("{context}: {error:?}")))
}

// The identity boundary is a committed observation ledger. Registration, capability
// persistence, enrolment, bearer minting and all subsequent authorization are production code.
struct IdentityLedger {
    did: Did,
    grant: CapabilityId,
}

impl IdentityResolver for IdentityLedger {
    fn resolve(&mut self, did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok((did == &self.did).then(|| CoreIdentity {
            canonical_bytes: b"core-identity".to_vec(),
            head_sequence: 10,
            revocation_sequence: 1,
            verification_level: VerificationLevel::STATE_PROVEN,
            frozen: false,
            authorities: vec![ProtocolAuthority::CapabilityGrant(self.grant.0)],
        }))
    }
}

struct Principal {
    tenant: TenantId,
    agent: Did,
    credential: SessionCredential,
    capability: CapabilityId,
    binding: PathBuf,
}

struct Provisioned {
    control: SessionControl,
    alpha: Principal,
    beta: Principal,
    workspace: Workspace,
}

fn limit() -> LimitConfig {
    LimitConfig {
        id: LimitId([1; 16]),
        name: "isolation transport".to_owned(),
        scope: LimitScope::Tenant([2; 32]),
        ceiling: 1_000,
        consumed: 0,
    }
}

fn provision(
    workspace: &Workspace,
    store: &mut Store,
    sessions: &mut SessionRegistry,
    name: &str,
    id: u8,
    endpoint: &str,
) -> Result<Principal, IsolationFailure> {
    let tenant = tenant(name)?;
    let agent = checked(
        Did::new(format!("did:layerx:{name}-agent").as_bytes()),
        "agent DID",
    )?;
    let capability = checked(
        Capability::new(
            CapabilityId([id + 2; 32]),
            tenant.clone(),
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
        ),
        "bounded capability",
    )?;
    checked(capability.persist(store), "persist capability")?;
    let identity = checked(
        register(
            store,
            tenant.clone(),
            agent.clone(),
            &mut IdentityLedger {
                did: agent.clone(),
                grant: capability.id,
            },
        ),
        "register recorded identity",
    )?;
    let surface = checked(
        DaemonSurface::new("127.0.0.1:1", "b".repeat(48), [3; 32]),
        "daemon surface",
    )?
    .with_readiness(checked(
        ReadinessDeclaration::new(
            endpoint,
            workspace.0.join("gateway-key"),
            Some(workspace.0.join("ca.pem")),
        ),
        "TLS declaration",
    )?);
    let publisher = checked(
        BindingPublisher::new(
            workspace.0.join(name),
            store.root().to_path_buf(),
            workspace.0.join(format!("{name}-audit")),
            surface,
            limit(),
            DEADLINE,
            BindingMode::Full,
        ),
        "binding publisher",
    )?;
    let published = checked(
        enrolment::enrol(
            store,
            sessions,
            &identity,
            EnrolmentRequest {
                session_id: SessionId([id; 32]),
                capability_id: capability.id,
                permitted_activity_types: BTreeSet::from([7]),
                scopes: ["read", "write", "subscribe", "export", "read:balance"]
                    .map(str::to_owned)
                    .into_iter()
                    .collect(),
                expiry_sequence: 300,
                opening_client: "isolation-suite".to_owned(),
                policy_version: "policy-v1".to_owned(),
                core_sequence: SEQUENCE,
            },
            &publisher,
        ),
        "enrol and publish",
    )?;
    let binding = checked(
        Binding::open(&published.binding),
        "published binding consumer",
    )?;
    let credential = checked(binding.daemon_credential(), "published credential consumer")?;
    checked(
        sessions.authenticate(&credential),
        "authenticate produced credential",
    )?;
    Ok(Principal {
        tenant,
        agent,
        credential,
        capability: capability.id,
        binding: published.binding,
    })
}

impl Provisioned {
    fn new(endpoint: &str) -> Result<Self, IsolationFailure> {
        let workspace = Workspace::new();
        let mut store = checked(Store::open(workspace.0.join("store")), "provision store")?;
        let key_path = workspace.0.join("gateway-key");
        checked(
            std::fs::write(&key_path, format!("isolation:lxp_live_{}", "11".repeat(32))),
            "write gateway fixture",
        )?;
        checked(
            std::fs::set_permissions(&key_path, std::fs::Permissions::from_mode(0o600)),
            "protect gateway fixture",
        )?;
        checked(
            std::fs::write(
                workspace.0.join("ca.pem"),
                include_bytes!("../../../crates/layerx-sdk/src/repair_22_31_227_fixture/ca.pem"),
            ),
            "write trust fixture",
        )?;
        let mut sessions = SessionRegistry::default();
        let alpha = provision(&workspace, &mut store, &mut sessions, "alpha", 7, endpoint)?;
        let beta = provision(&workspace, &mut store, &mut sessions, "beta", 8, endpoint)?;
        let control = SessionControl::new(
            Arc::new(Mutex::new(store)),
            sessions,
            Arc::new(PreparationLifecycle::default()),
            Arc::new(checked(
                BudgetLimiter::new(vec![limit()]),
                "budget limiter",
            )?),
        );
        Ok(Self {
            control,
            alpha,
            beta,
            workspace,
        })
    }
}

fn envelope(
    principal: &Principal,
    operation: Operation,
    body_tenant: &str,
) -> Result<Envelope, IsolationFailure> {
    let credential = &principal.credential;
    let key = if operation.mutating() {
        format!("\"{}\"", "42".repeat(32))
    } else {
        "null".to_owned()
    };
    let wire = format!(
        r#"{{"version":1,"request_id":"1","operation":"{}","idempotency_key":{},"credential":{{"tenant":"{}","session_id":"{}","token_id":"{}","generation":"{}"}},"request":{{"tenant":"{}","agent":"{}"}}}}"#,
        operation.name(),
        key,
        credential.tenant().as_str(),
        hex(&credential.session_id().0),
        hex(&credential.token_id()),
        credential.generation(),
        body_tenant,
        std::str::from_utf8(principal.agent.as_bytes()).map_err(|error| fail(error.to_string()))?,
    );
    checked(
        agent_rpc::decode(wire.as_bytes()),
        "production envelope decoder",
    )
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

/// Exercises the actual envelope admission and tenant-scoped object consumers. SDK
/// execution beyond this owner boundary is tracked explicitly in the repair record.
pub fn agent_owner_route_suite() -> Result<usize, IsolationFailure> {
    let fixture = Provisioned::new("https://localhost/")?;
    let owner = {
        let registry = fixture.control.registry();
        let registry = checked(registry.read(), "owner registry")?;
        let record = registry
            .get(&fixture.beta.tenant, fixture.beta.credential.session_id())
            .ok_or_else(|| fail("foreign session was not provisioned"))?;
        ObjectOwner {
            tenant: record.request.tenant.clone(),
            agent: Some(record.request.agent.clone()),
        }
    };
    for surface in Surface::ALL {
        let operation = match surface {
            Surface::Subscription => Operation::SubscriptionHealth,
            Surface::Export => Operation::ExportOffline,
            _ => Operation::CapabilityList,
        };
        let own = envelope(&fixture.alpha, operation, "alpha")?;
        let permit = checked(
            agent_rpc::authorize(&fixture.control, &own, surface, SEQUENCE, None),
            "own admission",
        )?;
        if permit.principal().tenant != fixture.alpha.tenant
            || permit.principal().agent != fixture.alpha.agent
        {
            return Err(fail(format!("{surface:?} admitted the wrong principal")));
        }
        let shared = fixture.control.store();
        let store = checked(shared.lock(), "object consumer store")?;
        let owned = checked(
            Capability::restore(
                &store,
                permit.principal().tenant.clone(),
                fixture.alpha.capability,
            ),
            "owned lookup",
        )?;
        if owned.as_ref().map(|capability| &capability.tenant) != Some(&fixture.alpha.tenant) {
            return Err(fail(format!(
                "{surface:?} did not consume the provisioned own object"
            )));
        }
        if checked(
            Capability::restore(&store, fixture.beta.tenant.clone(), fixture.beta.capability),
            "foreign positive control",
        )?
        .is_none()
        {
            return Err(fail("foreign object was not provisioned"));
        }
        let foreign = checked(
            Capability::restore(
                &store,
                permit.principal().tenant.clone(),
                fixture.beta.capability,
            ),
            "foreign lookup",
        )?;
        let absent = checked(
            Capability::restore(
                &store,
                permit.principal().tenant.clone(),
                CapabilityId([99; 32]),
            ),
            "absent lookup",
        )?;
        if foreign.is_some() || absent.is_some() {
            return Err(fail(format!(
                "{surface:?} foreign/absent lookup escaped its partition"
            )));
        }
        // Normalize the errors of real absent tenant-qualified lookups, and compare all
        // public observables, including the fixed-work timing shape.
        let mut foreign_metrics = BoundedMetrics::default();
        let mut absent_metrics = BoundedMetrics::default();
        let foreign_error =
            InternalError::not_authorized("beta-secret", b"existing-foreign-object".to_vec());
        let absent_error = InternalError::missing(b"missing-object".to_vec());
        let foreign_response = normalize_error(
            &foreign_error,
            &fixture.alpha.tenant,
            surface,
            &mut foreign_metrics,
        );
        let absent_response = normalize_error(
            &absent_error,
            &fixture.alpha.tenant,
            surface,
            &mut absent_metrics,
        );
        if foreign_response != absent_response
            || foreign_metrics.counters() != absent_metrics.counters()
            || foreign_metrics.traces() != absent_metrics.traces()
        {
            return Err(fail(format!(
                "{surface:?} object existence leaked through observables"
            )));
        }
        let rendered = format!(
            "{foreign_response:?} {:?} {:?}",
            foreign_metrics.counters(),
            foreign_metrics.traces()
        );
        if rendered.contains("beta-secret") || rendered.contains("existing-foreign-object") {
            return Err(fail(format!("{surface:?} leaked foreign context")));
        }
        drop(store);
        drop(permit);
        let denied = agent_rpc::authorize(
            &fixture.control,
            &own,
            surface,
            SEQUENCE,
            Some(owner.clone()),
        )
        .err()
        .ok_or_else(|| fail(format!("{surface:?} admitted the stored foreign owner")))?;
        let mut spoofed = envelope(&fixture.alpha, operation, "alpha")?;
        spoofed.credential = Some(SessionCredential::new(
            fixture.beta.tenant.clone(),
            fixture.alpha.credential.session_id(),
            fixture.alpha.credential.token_id(),
            fixture.alpha.credential.generation(),
        ));
        let foreign_credential =
            agent_rpc::authorize(&fixture.control, &spoofed, surface, SEQUENCE, None)
                .err()
                .ok_or_else(|| fail("cross-tenant session credential admitted"))?;
        spoofed.credential = Some(SessionCredential::new(
            tenant("absent")?,
            fixture.alpha.credential.session_id(),
            fixture.alpha.credential.token_id(),
            fixture.alpha.credential.generation(),
        ));
        let absent_credential =
            agent_rpc::authorize(&fixture.control, &spoofed, surface, SEQUENCE, None)
                .err()
                .ok_or_else(|| fail("absent session credential admitted"))?;
        if agent_rpc::http_status(&foreign_credential) != agent_rpc::http_status(&absent_credential)
            || agent_rpc::error_body(&foreign_credential)
                != agent_rpc::error_body(&absent_credential)
            || denied.reason != "session.not_authorized"
        {
            return Err(fail(format!("{surface:?} credential existence leaked")));
        }
        let spoofed = envelope(&fixture.alpha, operation, "beta")?;
        if agent_rpc::authorize(&fixture.control, &spoofed, surface, SEQUENCE, None).is_ok() {
            return Err(fail(format!("{surface:?} trusted the body tenant")));
        }
        let denied_text = String::from_utf8(agent_rpc::error_body(&denied))
            .map_err(|error| fail(error.to_string()))?;
        if denied_text.contains("beta")
            || denied_text.contains(&hex(&fixture.alpha.credential.token_id()))
        {
            return Err(fail(format!(
                "{surface:?} refusal leaked foreign or bearer context"
            )));
        }
    }
    // The MCP owner itself, rather than a helper, loads capabilities in the session's
    // partition. A valid foreign capability and an absent capability are indistinguishable.
    let bind = |capability| {
        Server::bind(
            fixture.control.clone(),
            fixture.alpha.credential.clone(),
            capability,
            SEQUENCE,
            fixture.workspace.0.join("mcp-object-audit"),
        )
    };
    if !matches!(
        bind(fixture.beta.capability),
        Err(ServerError::MissingCapability)
    ) || !matches!(
        bind(CapabilityId([99; 32])),
        Err(ServerError::MissingCapability)
    ) {
        return Err(fail("MCP owner disclosed foreign capability existence"));
    }
    checked(
        bind(fixture.alpha.capability),
        "MCP owned capability positive control",
    )?;
    Ok(Surface::ALL.len())
}

struct Loopback {
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<Result<(), IsolationFailure>>>,
    requests: Arc<AtomicUsize>,
}

impl Loopback {
    fn start(listener: TcpListener, fixture: &Provisioned) -> Result<Self, IsolationFailure> {
        checked(listener.set_nonblocking(true), "nonblocking TLS listener")?;
        let config = checked(
            tls::ServerConfig::builder_with_provider(Arc::new(
                tls::crypto::ring::default_provider(),
            ))
            .with_safe_default_protocol_versions(),
            "TLS protocol versions",
        )?
        .with_no_client_auth()
        .with_single_cert(
            vec![tls::pki_types::CertificateDer::from(CERTIFICATE.to_vec())],
            tls::pki_types::PrivateKeyDer::Pkcs8(tls::pki_types::PrivatePkcs8KeyDer::from(
                PRIVATE_KEY.to_vec(),
            )),
        );
        let config = Arc::new(checked(config, "recorded TLS identity")?);
        let stop = Arc::new(AtomicBool::new(false));
        let signal = Arc::clone(&stop);
        let requests = Arc::new(AtomicUsize::new(0));
        let received = Arc::clone(&requests);
        let control = fixture.control.clone();
        let audit = fixture.workspace.0.join("transport-audit");
        let capability = fixture.alpha.capability;
        let worker = thread::spawn(move || {
            while !signal.load(Ordering::Acquire) {
                let (socket, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => return Err(fail(format!("TLS accept: {error}"))),
                };
                checked(socket.set_read_timeout(Some(DEADLINE)), "TLS read deadline")?;
                checked(
                    socket.set_write_timeout(Some(DEADLINE)),
                    "TLS write deadline",
                )?;
                let connection = checked(
                    tls::ServerConnection::new(Arc::clone(&config)),
                    "TLS server",
                )?;
                let mut stream = tls::StreamOwned::new(connection, socket);
                let body = read_request(&mut stream)?;
                let response = describe(&control, capability, &audit, &body)?;
                checked(write!(stream, "HTTP/1.1 {} Response\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.status, response.body.len()), "HTTP response head")?;
                checked(stream.write_all(&response.body), "HTTP response body")?;
                checked(stream.flush(), "HTTP response flush")?;
                stream.conn.send_close_notify();
                checked(stream.flush(), "TLS close notify")?;
                received.fetch_add(1, Ordering::Release);
            }
            Ok(())
        });
        Ok(Self {
            stop,
            worker: Some(worker),
            requests,
        })
    }

    fn finish(mut self) -> Result<usize, IsolationFailure> {
        self.stop.store(true, Ordering::Release);
        let worker = self
            .worker
            .take()
            .ok_or_else(|| fail("TLS worker already stopped"))?;
        checked(worker.join(), "TLS worker panic")??;
        Ok(self.requests.load(Ordering::Acquire))
    }
}

impl Drop for Loopback {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _joined = worker.join();
        }
    }
}

fn read_request(stream: &mut impl Read) -> Result<Vec<u8>, IsolationFailure> {
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    checked(reader.read_line(&mut line), "HTTP route")?;
    if line != "POST /v1/agent/rpc HTTP/1.1\r\n" {
        return Err(fail("Rust SDK did not use its declared Agent route"));
    }
    let mut length = None;
    let mut authenticated = false;
    let mut sdk = false;
    loop {
        line.clear();
        if checked(reader.read_line(&mut line), "HTTP header")? == 0 {
            return Err(fail("truncated HTTP request"));
        }
        if line == "\r\n" {
            break;
        }
        if let Some((name, value)) = line.split_once(':') {
            if name.eq_ignore_ascii_case("content-length") {
                length = Some(checked(value.trim().parse::<usize>(), "HTTP length")?);
            }
            if name.eq_ignore_ascii_case("authorization") {
                authenticated =
                    value.trim() == format!("LayerX-Key isolation:lxp_live_{}", "11".repeat(32));
            }
            if name.eq_ignore_ascii_case("user-agent") {
                sdk = value.trim().starts_with("layerx-rust/");
            }
        }
    }
    if !authenticated || !sdk {
        return Err(fail(
            "SDK gateway authentication or client provenance missing",
        ));
    }
    let length = length
        .filter(|length| *length <= agent_rpc::MAX_BODY_BYTES)
        .ok_or_else(|| fail("missing or oversized HTTP body"))?;
    let mut body = vec![0; length];
    checked(reader.read_exact(&mut body), "HTTP body")?;
    Ok(body)
}

fn describe(
    control: &SessionControl,
    capability: CapabilityId,
    audit: &Path,
    body: &[u8],
) -> Result<AgentRpcResponse, IsolationFailure> {
    let incoming = agent_rpc::decode_mcp_invocation(body)
        .map_err(|response| {
            fail(format!(
                "production MCP decoder rejected HTTP {}: {}",
                response.status,
                String::from_utf8_lossy(&response.body)
            ))
        })?
        .ok_or_else(|| fail("SDK did not produce mcp.invoke"))?;
    if incoming.tool != "mcp.describe"
        || incoming
            .arguments
            .as_object()
            .is_none_or(|fields| !fields.is_empty())
    {
        return Err(fail("unexpected description request"));
    }
    // mcp.describe is authorized by the first advertised read tool, as in Server.
    let mut admission = checked(agent_rpc::decode(body), "production admission decoder")?;
    admission.operation = Operation::ReadBalance;
    admission.request.clear();
    if let Err(rejection) = agent_rpc::authorize(control, &admission, Surface::Mcp, SEQUENCE, None)
    {
        return Ok(agent_rpc::mcp_result_refusal(
            incoming.request_id,
            rejection.class,
            rejection.reason,
        ));
    }
    let server = checked(
        Server::bind(
            control.clone(),
            incoming.credential,
            capability,
            SEQUENCE,
            audit,
        ),
        "production MCP bind",
    )?;
    let produced = checked(
        server.describe_for_release(
            SEQUENCE,
            McpInvocationIdentity {
                tool_name: "mcp.describe",
                idempotency_key: incoming.idempotency_key,
                request_digest: DESCRIPTION_DIGEST,
            },
        ),
        "production MCP description",
    )?;
    checked(
        produced.release_current(SEQUENCE, |value| {
            let value = value
                .map_err(|_| layerx_agentd::session_control::SessionControlError::Unavailable)?;
            Ok(agent_rpc::mcp_result_response(incoming.request_id, value))
        }),
        "production MCP result release",
    )
}

/// Runs the real MCP stdio consumer through its Rust SDK, TLS, production codec,
/// persisted capability/session owner, durable invocation and response encoder.
pub fn agent_mcp_transport_suite() -> Result<usize, IsolationFailure> {
    let listener = checked(TcpListener::bind("127.0.0.1:0"), "TLS loopback bind")?;
    let endpoint = format!(
        "https://localhost:{}/",
        checked(listener.local_addr(), "TLS address")?.port()
    );
    let fixture = Provisioned::new(&endpoint)?;
    let server = Loopback::start(listener, &fixture)?;
    let binding = checked(
        Binding::open(&fixture.alpha.binding),
        "MCP transport binding",
    )?;
    let mut client = checked(binding.open_daemon_client(), "real Rust SDK MCP connect")?;
    let listed = client
        .handle(r#"{"jsonrpc":"2.0","id":1,"method":"tools/list"}"#)
        .ok_or_else(|| fail("missing tools/list response"))?;
    if listed.get("error").is_some()
        || !listed["result"]["tools"].as_array().is_some_and(|tools| {
            tools
                .iter()
                .any(|tool| tool["name"].as_str() == Some("balance.get"))
        })
    {
        return Err(fail(
            "MCP consumer did not receive the real owner's tool description",
        ));
    }
    let rendered = listed.to_string();
    if rendered.contains("beta") || rendered.contains(&hex(&fixture.alpha.credential.token_id())) {
        return Err(fail("MCP description leaked another tenant or a bearer"));
    }
    let document = checked(
        layerx_mcp::boundary::decode_native_read_result(&checked(
            std::fs::read_to_string(&fixture.alpha.binding),
            "published binding bytes",
        )?),
        "published binding JSON",
    )?;
    let refused_binding = |name: &str| -> Result<_, IsolationFailure> {
        let mut document = document.clone();
        document["tenant"] = name.into();
        let binding = checked(
            Binding::parse(&document.to_string()),
            "spoofed transport coordinate",
        )?;
        binding
            .open_daemon_client()
            .err()
            .ok_or_else(|| fail("SDK admitted a cross-tenant/absent credential"))
    };
    let foreign = refused_binding(fixture.beta.tenant.as_str())?;
    let absent = refused_binding("absent")?;
    if foreign != absent {
        return Err(fail(
            "MCP SDK disclosed foreign versus absent session existence",
        ));
    }
    checked(
        fixture.control.close(
            &fixture.alpha.tenant,
            fixture.alpha.credential.session_id(),
            SEQUENCE,
        ),
        "close real session",
    )?;
    let refused = client
        .handle(r#"{"jsonrpc":"2.0","id":2,"method":"tools/list"}"#)
        .ok_or_else(|| fail("missing revoked-client response"))?;
    if refused.get("error").is_none() || refused.get("result").is_some() {
        return Err(fail("MCP SDK released a description after session closure"));
    }
    let beta = checked(
        Server::bind(
            fixture.control.clone(),
            fixture.beta.credential.clone(),
            fixture.beta.capability,
            SEQUENCE,
            fixture.workspace.0.join("beta-live-audit"),
        ),
        "other tenant remained authorized",
    )?;
    if beta.binding().tenant() != &fixture.beta.tenant {
        return Err(fail("closing alpha changed beta authority"));
    }
    drop(beta);
    drop(client);
    let requests = server.finish()?;
    if requests != 5 {
        return Err(fail(format!(
            "expected five genuine SDK exchanges, observed {requests}"
        )));
    }
    Ok(requests)
}
