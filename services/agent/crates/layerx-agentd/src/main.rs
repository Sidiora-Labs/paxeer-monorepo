#![forbid(unsafe_code)]

use std::collections::{BTreeMap, BTreeSet};
use std::env;
use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::{mpsc, Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use layerx_agent_api::error::ErrorClass;
use layerx_agentd::agent_rpc::{self, AgentRpcResponse};
use layerx_agentd::agent_rpc_tls::{AgentRpcTls, AgentRpcTlsPaths};
use layerx_agentd::audit::Redacted;
use layerx_agentd::boot::readiness_gate;
use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::enrolment::{
    self, BindingMode, BindingPublisher, DaemonSurface, EnrolmentRequest, ListenerDeclaration,
    WebDeclaration,
};
use layerx_agentd::human::{HumanListenerConfig, HumanPeer, HumanUnixServer};
use layerx_agentd::human_runtime::{
    HumanAuthorityBoundary, ProductionHumanOperations, RemoteHumanAuthority, SharedAgentOwner,
    UnifiedAgentOwner,
};
use layerx_agentd::identity::{self, CoreIdentity, IdentityError, IdentityResolver};
use layerx_agentd::node_rpc::environment::{
    parse_digest, parse_u64, refuse_retired_environment, required, HumanNodeEnvironment, McpHeights,
};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::ops::program::ProgramOperations;
use layerx_agentd::read::{
    LayerxdProgramBalanceReader, NativeReadRoute, ProgramAuthority, ProgramBalanceReadRoute,
    ProgramReadError,
};
use layerx_agentd::session::{SessionId, SessionRegistry};
use layerx_agentd::session_keys::SessionKeyRegistry;
use layerx_agentd::store::{Store, TenantId};
use layerx_client::frame::transport::Limits;
type PxEndpoint = layerx_client::px::PxEndpoint;
type SequencerEndpoint = layerx_client::sequencer::SequencerEndpoint;
use layerx_programs::{
    hex, DeploymentProof, DeploymentRecord, ProgramId, ProgramLifecycle,
    ProtocolDeploymentVerifier, Registry,
};
use layerx_types::ids::Did;

use layerx_agentd::config::agent_mode as human_owner_mode;

mod human_owner {
    use super::serve_http::DeadlineStream;

    // The shared handler is also exercised at a real loopback listener by the repair proof.
    use super::{
        connect_human_authority, human_peers, optional, parse_u64, required, start_human_owner,
        HttpPool, SUPERVISION_INTERVAL,
    };
    use layerx_agentd::human_receipt_frame::owner_health as health_contract;
    use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
    use std::net::TcpListener;
    use std::sync::atomic::{AtomicBool, Ordering};
    use std::sync::{mpsc, Arc};
    use std::thread;
    use std::time::Duration;

    fn node_rpc_config() -> Result<NodeRpcConfig, String> {
        NodeRpcConfig::from_environment().map_err(|error| {
        format!(
            "human node RPC requires LAYERX_AGENT_NODE_RPC_URL and LAYERX_AGENT_NODE_CHAIN_ID: {error}"
        )
    })
    }

    fn dependencies_ready() -> Result<(), String> {
        let deadline = Duration::from_millis(parse_u64("LAYERX_AGENT_HUMAN_DEADLINE_MS")?);
        NodeRpc::connect(&node_rpc_config()?)
            .map_err(|error| format!("human node RPC is unavailable: {error}"))?;
        connect_human_authority(deadline, &human_peers()?)?;
        Ok(())
    }

    fn owner_running(human: &mpsc::Receiver<Result<(), String>>) -> Result<(), String> {
        match human.try_recv() {
            Err(mpsc::TryRecvError::Empty) => Ok(()),
            Ok(Err(error)) => Err(error),
            Ok(Ok(())) | Err(mpsc::TryRecvError::Disconnected) => {
                Err("human listener terminated".to_owned())
            }
        }
    }

    pub(super) fn serve_human_owner() -> Result<(), String> {
        let listen = required("LAYERX_AGENT_PROGRAM_LISTEN")?;
        let bearer = required(health_contract::HEALTH_BEARER_ENV)?;
        health_contract::validate_health_credentials(
            &listen,
            &bearer,
            &required(health_contract::AUTHORITY_BEARER_ENV)?,
        )?;
        if optional("LAYERX_AGENT_MCP_BINDING_ROOT").is_some() {
            return Err(
            "LAYERX_AGENT_MCP_BINDING_ROOT requires the full agent mode, which serves the program endpoint and probe program the binding names"
                .to_owned(),
        );
        }
        human_peers()?;
        NodeRpc::connect(&node_rpc_config()?)
            .map_err(|error| format!("human node RPC is unavailable: {error}"))?;
        let human = start_human_owner(None)?;
        let listener = TcpListener::bind(listen)
            .map_err(|error| format!("human health listener failed: {error}"))?;
        let alive = Arc::new(AtomicBool::new(true));
        let ready = Arc::clone(&alive);
        let pool = HttpPool::start(listener, move |mut stream| {
            let _ = serve_health(&mut stream, &bearer, || ready.load(Ordering::Acquire));
        })?;
        loop {
            if let Err(error) = owner_running(&human).and_then(|()| pool.check()) {
                alive.store(false, Ordering::Release);
                return Err(error);
            }
            thread::sleep(SUPERVISION_INTERVAL);
        }
    }

    fn serve_health<F: Fn() -> bool>(
        stream: &mut DeadlineStream,
        bearer: &str,
        owner_ready: F,
    ) -> Result<(), String> {
        health_contract::serve_health(stream, bearer, owner_ready, dependencies_ready)
    }

    #[cfg(test)]
    mod tests {

        #[test]
        fn health_refuses_terminated_owner_and_preserves_http_boundaries(
        ) -> Result<(), Box<dyn std::error::Error>> {
            use std::io::{Read, Write};
            use std::net::{TcpListener, TcpStream};
            use std::sync::mpsc;
            use std::thread;
            use std::time::Duration;

            let bearer = "h".repeat(32);
            let cases = [
            (format!("GET /healthz HTTP/1.1\r\nAuthorization: Bearer {bearer}\r\n\r\n"), 503, "{\"ready\":false}"),
            ("GET /healthz HTTP/1.1\r\n\r\n".to_owned(), 401, "unauthorized"),
            (format!("GET /v1/programs/anything/balances HTTP/1.1\r\nAuthorization: Bearer {bearer}\r\n\r\n"), 404, "not_found"),
            ("POST /healthz HTTP/1.1\r\n\r\n".to_owned(), 400, "invalid_request"),
            ("x".repeat(crate::HEADER_LIMIT), 431, "headers_too_large"),
        ];
            for (request, status, body) in cases {
                let listener = TcpListener::bind("127.0.0.1:0")?;
                let address = listener.local_addr()?;
                let bearer = bearer.clone();
                let server = thread::spawn(move || -> Result<(), String> {
                    let (stream, _) = listener.accept().map_err(|e| e.to_string())?;
                    stream
                        .set_read_timeout(Some(Duration::from_secs(5)))
                        .map_err(|e| e.to_string())?;
                    let (sender, receiver) = mpsc::channel();
                    drop(sender);
                    let mut stream = super::DeadlineStream {
                        socket: stream,
                        expires: std::time::Instant::now()
                            + super::super::serve_http::REQUEST_DEADLINE,
                    };
                    super::serve_health(&mut stream, &bearer, || {
                        super::owner_running(&receiver).is_ok()
                    })
                });
                let mut client = TcpStream::connect(address)?;
                client.set_read_timeout(Some(Duration::from_secs(5)))?;
                client.write_all(request.as_bytes())?;
                let mut response = String::new();
                client.read_to_string(&mut response)?;
                server.join().map_err(|_| "health server panicked")??;
                assert!(response.starts_with(&format!("HTTP/1.1 {status} ")));
                assert!(response.contains(body));
            }
            Ok(())
        }
    }
}
mod human_peer_config;

#[path = "../../layerx-agentd-host/src/payer.rs"]
mod mcp_payer;

// Share the exact HTTP path with the focused proof target. The implementation
// lives in its declared proof path because this task may not add a source file.
#[path = "../tests/proof_serve_errors.rs"]
mod serve_http;
pub(crate) use self::serve_http::{
    serve_read_routes, HttpPool, HEADER_LIMIT, REQUEST_DEADLINE, SUPERVISION_INTERVAL,
};

struct Config {
    inference: Option<layerx_agentd::config::InferenceConfig>,
    listen: String,
    bearer: String,
    node_endpoint: String,
    expected_chain_id: u64,
    node_bearer: String,
    authority_endpoint: String,
    authority_bearer: String,
    authority_ca_der: Vec<u8>,
    authority_replica_id: [u8; 32],
    chain_trust_root: PathBuf,
    staleness_ms: u64,
    deployment_journal: String,
    probe_program: ProgramId,
    policy_sources: BTreeMap<TenantId, PathBuf>,
    native_policy_sources: BTreeMap<TenantId, PathBuf>,
    native_effect_policy_sources: BTreeMap<TenantId, PathBuf>,
    program_budget_denomination_sources: BTreeMap<TenantId, PathBuf>,
}

fn optional(name: &str) -> Option<String> {
    env::var(name).ok().filter(|value| !value.is_empty())
}

fn absolute_path(name: &str) -> Result<PathBuf, String> {
    let path = PathBuf::from(required(name)?);
    if path.is_absolute() {
        Ok(path)
    } else {
        Err(format!("{name} must be an absolute path"))
    }
}

fn read_ca(name: &str) -> Result<Vec<u8>, String> {
    let bytes = fs::read(required(name)?).map_err(|_| format!("{name} is unreadable"))?;
    if bytes.is_empty() {
        return Err(format!("{name} is empty"));
    }
    Ok(bytes)
}

fn parse_hex<const N: usize>(name: &str) -> Result<[u8; N], String> {
    let value = required(name)?;
    if value.len() != N * 2 {
        return Err(format!("{name} has the wrong width"));
    }
    let mut bytes = [0; N];
    for (index, byte) in bytes.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&value[index * 2..index * 2 + 2], 16)
            .map_err(|_| format!("{name} is not hexadecimal"))?;
    }
    Ok(bytes)
}

fn human_peers() -> Result<BTreeMap<u32, (String, String)>, String> {
    human_peer_config::parse(&required("LAYERX_AGENT_HUMAN_PEERS")?)
        .map_err(|error| error.to_string())
}

fn verified_limit() -> Result<LimitConfig, String> {
    let scope_bytes = parse_hex("LAYERX_AGENT_HUMAN_LIMIT_SCOPE_ID")?;
    let scope = match required("LAYERX_AGENT_HUMAN_LIMIT_SCOPE")?.as_str() {
        "tenant" => LimitScope::Tenant(scope_bytes),
        "agent" => LimitScope::Agent(scope_bytes),
        "session" => LimitScope::Session(scope_bytes),
        "capability" => LimitScope::Capability(scope_bytes),
        "counterparty" => LimitScope::Counterparty(scope_bytes),
        _ => return Err("human limit scope is invalid".to_owned()),
    };
    Ok(LimitConfig {
        id: LimitId(parse_hex("LAYERX_AGENT_HUMAN_LIMIT_ID")?),
        name: required("LAYERX_AGENT_HUMAN_LIMIT_NAME")?,
        scope,
        ceiling: required("LAYERX_AGENT_HUMAN_LIMIT_CEILING")?
            .parse()
            .map_err(|_| "human limit ceiling is invalid")?,
        consumed: required("LAYERX_AGENT_HUMAN_LIMIT_CONSUMED")?
            .parse()
            .map_err(|_| "human limit consumed is invalid")?,
    })
}

struct McpEnrolment {
    root: PathBuf,
    audit_root: PathBuf,
    peer_uid: u32,
    did: Did,
    request: EnrolmentRequest,
    limit: LimitConfig,
    deadline: Duration,
    mode: BindingMode,
    listener: ListenerDeclaration,
    web: Option<WebDeclaration>,
}

struct McpBoot {
    surface: DaemonSurface,
    enrolment: McpEnrolment,
}

struct ResolvedIdentity {
    did: Did,
    observation: CoreIdentity,
}

impl IdentityResolver for ResolvedIdentity {
    fn resolve(&mut self, did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        if did == &self.did {
            Ok(Some(self.observation.clone()))
        } else {
            Ok(None)
        }
    }
}

fn mcp_activity_types() -> Result<BTreeSet<u16>, String> {
    let mut values = BTreeSet::new();
    for entry in required("LAYERX_AGENT_MCP_ACTIVITY_TYPES")?.split(',') {
        let value = entry
            .trim()
            .parse()
            .map_err(|_| "LAYERX_AGENT_MCP_ACTIVITY_TYPES lists an invalid activity type")?;
        if !values.insert(value) {
            return Err("LAYERX_AGENT_MCP_ACTIVITY_TYPES repeats an activity type".to_owned());
        }
    }
    Ok(values)
}

fn mcp_scopes() -> Result<BTreeSet<String>, String> {
    let mut values = BTreeSet::new();
    for entry in required("LAYERX_AGENT_MCP_SCOPES")?.split(',') {
        let value = entry.trim();
        if value.is_empty() {
            return Err("LAYERX_AGENT_MCP_SCOPES lists an empty scope".to_owned());
        }
        if !values.insert(value.to_owned()) {
            return Err("LAYERX_AGENT_MCP_SCOPES repeats a scope".to_owned());
        }
    }
    Ok(values)
}

fn mcp_limit() -> Result<LimitConfig, String> {
    let scope_bytes = parse_hex("LAYERX_AGENT_MCP_LIMIT_SCOPE_ID")?;
    let scope = match required("LAYERX_AGENT_MCP_LIMIT_SCOPE")?.as_str() {
        "tenant" => LimitScope::Tenant(scope_bytes),
        "agent" => LimitScope::Agent(scope_bytes),
        "session" => LimitScope::Session(scope_bytes),
        "capability" => LimitScope::Capability(scope_bytes),
        "counterparty" => LimitScope::Counterparty(scope_bytes),
        _ => return Err("LAYERX_AGENT_MCP_LIMIT_SCOPE is invalid".to_owned()),
    };
    Ok(LimitConfig {
        id: LimitId(parse_hex("LAYERX_AGENT_MCP_LIMIT_ID")?),
        name: required("LAYERX_AGENT_MCP_LIMIT_NAME")?,
        scope,
        ceiling: required("LAYERX_AGENT_MCP_LIMIT_CEILING")?
            .parse()
            .map_err(|_| "LAYERX_AGENT_MCP_LIMIT_CEILING is invalid")?,
        consumed: required("LAYERX_AGENT_MCP_LIMIT_CONSUMED")?
            .parse()
            .map_err(|_| "LAYERX_AGENT_MCP_LIMIT_CONSUMED is invalid")?,
    })
}

fn mcp_admitted_uids() -> Result<Vec<u32>, String> {
    let mut values = Vec::new();
    for entry in required("LAYERX_AGENT_MCP_LISTENER_ADMITTED_UIDS")?.split(',') {
        let value = entry
            .trim()
            .parse()
            .map_err(|_| "LAYERX_AGENT_MCP_LISTENER_ADMITTED_UIDS lists an invalid uid")?;
        if values.contains(&value) {
            return Err("LAYERX_AGENT_MCP_LISTENER_ADMITTED_UIDS repeats a uid".to_owned());
        }
        values.push(value);
    }
    Ok(values)
}

/// Reads the protected protocol socket the published binding declares. A binding is never
/// published without one, and the enrolled peer must be one of the admitted peers.
fn mcp_listener(peer_uid: u32) -> Result<ListenerDeclaration, String> {
    let admitted_uids = mcp_admitted_uids()?;
    if !admitted_uids.contains(&peer_uid) {
        return Err(
            "LAYERX_AGENT_MCP_LISTENER_ADMITTED_UIDS does not admit LAYERX_AGENT_MCP_PEER_UID"
                .to_owned(),
        );
    }
    ListenerDeclaration::new(
        absolute_path("LAYERX_AGENT_MCP_LISTENER_SOCKET")?,
        required("LAYERX_AGENT_MCP_LISTENER_OWNER_UID")?
            .parse()
            .map_err(|_| "LAYERX_AGENT_MCP_LISTENER_OWNER_UID is invalid")?,
        required("LAYERX_AGENT_MCP_LISTENER_OWNER_GID")?
            .parse()
            .map_err(|_| "LAYERX_AGENT_MCP_LISTENER_OWNER_GID is invalid")?,
        u32::from_str_radix(&required("LAYERX_AGENT_MCP_LISTENER_MODE")?, 8)
            .map_err(|_| "LAYERX_AGENT_MCP_LISTENER_MODE is not an octal mode")?,
        admitted_uids,
    )
    .map_err(|error| format!("the MCP listener is invalid: {error}"))
}

/// Reads the web sidecar the published binding references, when `LAYERX_AGENT_MCP_WEB_ENDPOINT`
/// enables it; every other web key is then required.
fn mcp_web(config: &Config) -> Result<Option<WebDeclaration>, String> {
    let Some(endpoint) = optional("LAYERX_AGENT_MCP_WEB_ENDPOINT") else {
        return Ok(None);
    };
    let index = usize::try_from(parse_u64("LAYERX_AGENT_MCP_WEB_RECEIPT_KEY_INDEX")?)
        .map_err(|_| "LAYERX_AGENT_MCP_WEB_RECEIPT_KEY_INDEX is out of range")?;
    let client = layerx_client::sequencer::SequencerEndpoint::new(
        &config.node_endpoint,
        &[&config.authority_ca_der],
    )
    .map_err(|error| format!("MCP receipt key endpoint is invalid: {error:?}"))?;
    let keys = NativeReadRoute::new(client, config.expected_chain_id)
        .signed_authority()
        .map_err(|error| format!("MCP receipt keys unavailable: {error:?}"))?;
    if config.expected_chain_id != 125 {
        return Err("the MCP web binding requires chain id 125".to_owned());
    }
    let address = |name: &str| -> Result<[u8; 20], String> {
        let value = required(name)?;
        let text = value
            .strip_prefix("0x")
            .ok_or_else(|| format!("{name} must be an EVM address"))?;
        if text.len() != 40 || !text.bytes().all(|byte| byte.is_ascii_hexdigit()) {
            return Err(format!("{name} must be an EVM address"));
        }
        let mut bytes = [0; 20];
        for (index, byte) in bytes.iter_mut().enumerate() {
            *byte = u8::from_str_radix(&text[index * 2..index * 2 + 2], 16)
                .map_err(|_| format!("{name} must be an EVM address"))?;
        }
        Ok(bytes)
    };
    WebDeclaration::new(
        endpoint,
        required("LAYERX_AGENT_MCP_WEB_NETWORK")?,
        address("LAYERX_AGENT_MCP_WEB_PAYER_ADDRESS")?,
        address("LAYERX_AGENT_MCP_WEB_RECEIVER_ADDRESS")?,
        keys,
        index,
        parse_u64("LAYERX_AGENT_MCP_WEB_TIMEOUT_MS")?,
        required("LAYERX_AGENT_MCP_WEB_PENDING_ATTEMPTS")?
            .parse()
            .map_err(|_| "LAYERX_AGENT_MCP_WEB_PENDING_ATTEMPTS is invalid")?,
        required("LAYERX_AGENT_MCP_WEB_APPROVAL_THRESHOLD")?
            .parse()
            .map_err(|_| "LAYERX_AGENT_MCP_WEB_APPROVAL_THRESHOLD is invalid")?,
    )
    .map(Some)
    .map_err(|error| format!("the MCP web sidecar is invalid: {error}"))
}

/// Reads the model context protocol enrolment this daemon publishes at boot, when one is
/// configured. `LAYERX_AGENT_MCP_BINDING_ROOT` selects the binding directory and every other
/// key is then required.
fn mcp_enrolment(config: &Config) -> Result<Option<McpEnrolment>, String> {
    if optional("LAYERX_AGENT_MCP_BINDING_ROOT").is_none() {
        return Ok(None);
    }
    let mode = match required("LAYERX_AGENT_MCP_MODE")?.as_str() {
        "full" => BindingMode::Full,
        "read-only" => BindingMode::ReadOnly,
        _ => return Err("LAYERX_AGENT_MCP_MODE is invalid".to_owned()),
    };
    let peer_uid = required("LAYERX_AGENT_MCP_PEER_UID")?
        .parse()
        .map_err(|_| "LAYERX_AGENT_MCP_PEER_UID is invalid")?;
    let heights = McpHeights::from_environment()?;
    Ok(Some(McpEnrolment {
        root: absolute_path("LAYERX_AGENT_MCP_BINDING_ROOT")?,
        audit_root: absolute_path("LAYERX_AGENT_MCP_AUDIT_ROOT")?,
        peer_uid,
        did: Did::new(required("LAYERX_AGENT_MCP_AGENT_DID")?.as_bytes())
            .map_err(|_| "LAYERX_AGENT_MCP_AGENT_DID is invalid")?,
        request: EnrolmentRequest {
            session_id: SessionId(parse_digest("LAYERX_AGENT_MCP_SESSION_ID")?),
            capability_id: CapabilityId(parse_digest("LAYERX_AGENT_MCP_CAPABILITY_ID")?),
            permitted_activity_types: mcp_activity_types()?,
            scopes: mcp_scopes()?,
            expiry_sequence: heights.expiry_height,
            opening_client: required("LAYERX_AGENT_MCP_OPENING_CLIENT")?,
            policy_version: required("LAYERX_AGENT_MCP_POLICY_VERSION")?,
            core_sequence: heights.core_height,
        },
        limit: mcp_limit()?,
        deadline: Duration::from_millis(parse_u64("LAYERX_AGENT_MCP_DEADLINE_MS")?),
        mode,
        listener: mcp_listener(peer_uid)?,
        web: mcp_web(config)?,
    }))
}

fn mcp_boot(config: &Config, enrolment: McpEnrolment) -> Result<McpBoot, String> {
    let surface = DaemonSurface::new(
        &config.listen,
        config.bearer.clone(),
        config.probe_program.bytes(),
    )
    .map_err(|error| format!("the MCP daemon surface is invalid: {error}"))?;
    Ok(McpBoot { surface, enrolment })
}

/// Publishes the binding document one model context protocol session reads.
///
/// The agent identity is resolved through the configured human authority, registered against
/// the daemon store, and enrolled into a capability-grant session whose token reaches disk only
/// inside the operator-protected files beside the document. A daemon that already opened the
/// configured session keeps the document it published rather than opening a second session.
fn publish_mcp_binding(
    authority: &mut RemoteHumanAuthority,
    peers: &BTreeMap<u32, (String, String)>,
    shared_store: &Arc<Mutex<Store>>,
    store_path: &Path,
    boot: McpBoot,
) -> Result<(EnrolmentRequest, Did, HumanPeer), String> {
    let McpBoot {
        surface,
        enrolment: configured,
    } = boot;
    let (principal, tenant) = peers
        .get(&configured.peer_uid)
        .ok_or("LAYERX_AGENT_MCP_PEER_UID names no configured human peer")?;
    let peer = HumanPeer {
        subject: None,
        uid: configured.peer_uid,
        principal: principal.clone(),
        tenant: tenant.clone(),
    };
    let tenant_id = TenantId::new(tenant.clone())
        .map_err(|error| format!("the MCP peer tenant is invalid: {error:?}"))?;
    let observation = authority
        .core_identity(&peer, &configured.did)
        .map_err(|error| format!("the MCP agent identity is unverified: {error:?}"))?;
    let capability_id = configured.request.capability_id;
    let verified = if layerx_agentd::human_runtime::has_completed_capability_install(
        shared_store,
        &tenant_id,
        capability_id,
    )
    .map_err(|error| format!("the MCP capability install record is unreadable: {error:?}"))?
    {
        Some(
            authority
                .verified_enrolment_expiry(shared_store, &peer, &configured.did, capability_id)
                .map_err(|error| {
                    format!("the MCP capability grant expiry is unverified: {error:?}")
                })?,
        )
    } else {
        None
    };
    let publisher = BindingPublisher::new(
        configured.root,
        store_path.to_path_buf(),
        configured.audit_root,
        surface,
        configured.limit,
        configured.deadline,
        configured.mode,
    )
    .map_err(|error| format!("the MCP binding publisher is invalid: {error}"))?
    .with_listener(configured.listener);
    let publisher = match configured.web {
        Some(web) => publisher.with_web(web),
        None => publisher,
    };
    let mut resolver = ResolvedIdentity {
        did: configured.did.clone(),
        observation,
    };
    let mut store = shared_store
        .lock()
        .map_err(|_| "the agent store is unavailable".to_owned())?;
    let identity = identity::register(&mut store, tenant_id.clone(), configured.did, &mut resolver)
        .map_err(|error| format!("the MCP agent identity is unusable: {error:?}"))?;
    let mut sessions = SessionRegistry::default();
    sessions
        .restore_tenant(&store, &tenant_id)
        .map_err(|error| format!("the agent sessions are unrestorable: {error:?}"))?;
    let request = configured.request;
    let republish = (request.clone(), identity.did().clone(), peer);
    let session_id = request.session_id;
    if let Some(existing) = sessions.get(&tenant_id, session_id) {
        let advertised = existing.request.expiry_seconds.is_some();
        publisher
            .validate_existing(&sessions, &identity, &request)
            .map_err(|error| format!("the published MCP binding is unusable: {error}"))?;
        if let (Some(expiry), false) = (verified, advertised) {
            enrolment::republish_with_verified_expiry(
                &mut store,
                &mut sessions,
                &tenant_id,
                identity.did(),
                enrolment::VerifiedExpiryEnrolment::new(request, expiry),
            )
            .map_err(|error| format!("MCP expiry republication failed: {error}"))?;
        }
        return Ok(republish);
    }
    match verified {
        Some(expiry) => enrolment::enrol_with_verified_expiry(
            &mut store,
            &mut sessions,
            &identity,
            enrolment::VerifiedExpiryEnrolment::new(request, expiry),
            &publisher,
        ),
        None => enrolment::enrol(&mut store, &mut sessions, &identity, request, &publisher),
    }
    .map_err(|error| format!("MCP enrolment failed: {error}"))?;
    Ok(republish)
}

fn human_frame_limits(deadline: Duration) -> Result<Limits, String> {
    layerx_agentd::human::human_frame_limits(deadline, required)
}

fn connect_human_node() -> Result<SequencerEndpoint, String> {
    let environment = HumanNodeEnvironment::from_environment()?;
    let config = environment.rpc;
    let certificates = if config.url.starts_with("https://") {
        let loaded = rustls_native_certs::load_native_certs();
        if loaded.certs.is_empty() || !loaded.errors.is_empty() {
            return Err("human node RPC trust roots are unavailable".to_owned());
        }
        loaded.certs
    } else {
        Vec::new()
    };
    let roots: Vec<&[u8]> = certificates.iter().map(AsRef::as_ref).collect();
    let endpoint = SequencerEndpoint::new(&config.url, &roots)
        .map_err(|error| format!("human sequencer endpoint is invalid: {error:?}"))?;
    if endpoint
        .chain_id()
        .map_err(|error| format!("human sequencer chain id is unavailable: {error:?}"))?
        != config.chain_id
    {
        return Err("human sequencer chain id mismatch".to_owned());
    }
    layerx_agentd::human_node::verify_direct_node_readiness(&endpoint, config.chain_id)
        .map_err(|error| format!("human node evidence refused: {error:?}"))?;
    Ok(endpoint)
}

fn configured_registry_source(
) -> Result<Option<layerx_agentd::registry_source::RegistrySourceProvider>, String> {
    let path = match env::var_os("LAYERX_AGENT_REGISTRY_SOURCE_CONFIG") {
        None => return Ok(None),
        Some(path) => PathBuf::from(path),
    };
    if !path.is_absolute() {
        return Err(
            "LAYERX_AGENT_REGISTRY_SOURCE_CONFIG must be an absolute protected file".to_owned(),
        );
    }
    layerx_agentd::registry_source::RegistrySourceProvider::from_protected_config(&path)
        .map(Some)
        .map_err(|error| format!("registry source provider configuration refused: {error:?}"))
}

fn connect_human_authority(
    deadline: Duration,
    peers: &BTreeMap<u32, (String, String)>,
) -> Result<RemoteHumanAuthority, String> {
    let authority = RemoteHumanAuthority::connect(
        &required("LAYERX_AGENT_HUMAN_AUTHORITY_ENDPOINT")?,
        required("LAYERX_AGENT_HUMAN_AUTHORITY_BEARER")?,
        deadline,
        required("LAYERX_AGENT_HUMAN_AUTHORITY_MAX_BYTES")?
            .parse()
            .map_err(|_| "human authority bound is invalid")?,
        &read_ca("LAYERX_AGENT_HUMAN_AUTHORITY_CA_DER")?,
    )
    .map_err(|error| format!("human authority is invalid: {error:?}"))?;
    for (uid, (principal, tenant)) in peers {
        authority
            .registry(&HumanPeer {
                subject: None,
                uid: *uid,
                principal: principal.clone(),
                tenant: tenant.clone(),
            })
            .map_err(|error| format!("human authority readiness failed: {error:?}"))?;
    }
    Ok(authority)
}

fn node_rpc(endpoint: &str, ca_der: &[u8]) -> Result<ureq::Agent, String> {
    layerx_client::sequencer::SequencerEndpoint::new(endpoint, &[ca_der])
        .map_err(|error| format!("node endpoint is invalid: {error:?}"))?;
    Ok(ureq::Agent::config_builder()
        .timeout_global(Some(Duration::from_secs(30)))
        .max_redirects(0)
        .tls_config(
            ureq::tls::TlsConfig::builder()
                .provider(ureq::tls::TlsProvider::Rustls)
                .root_certs(ureq::tls::RootCerts::new_with_certs(&[
                    ureq::tls::Certificate::from_der(ca_der).to_owned(),
                ]))
                .build(),
        )
        .build()
        .into())
}

fn export_trust_source(
    receipt_keys: Vec<layerx_client::sequencer::ReceiptKeyEntry>,
    ca_der: &[u8],
) -> Result<layerx_agentd::export::ExportTrustSource, String> {
    use base64::Engine as _;
    let endpoint = required("LAYERX_AGENT_NODE_CONSENSUS_ENDPOINT")?;
    let client = node_rpc(&endpoint, ca_der)?;
    let mut validators = Vec::new();
    let mut height = None;
    for page in 1..=100_u64 {
        let url = format!(
            "{}/validators?page={page}&per_page=100{}",
            endpoint.trim_end_matches('/'),
            height
                .map(|height| format!("&height={height}"))
                .unwrap_or_default()
        );
        let answer: serde_json::Value = client
            .get(&url)
            .call()
            .map_err(|error| format!("node validators unavailable: {error}"))?
            .body_mut()
            .with_config()
            .limit(1_048_576)
            .read_json()
            .map_err(|error| format!("node validators malformed: {error}"))?;
        let result = answer
            .get("result")
            .ok_or("node validator result missing")?;
        let number = |field: &str| {
            result
                .get(field)
                .and_then(serde_json::Value::as_str)
                .and_then(|value| value.parse::<u64>().ok())
                .ok_or("node validator number malformed")
        };
        let block_height = number("block_height")?;
        if height.is_some_and(|height| height != block_height) {
            return Err("node validator height changed".to_owned());
        }
        height = Some(block_height);
        let total =
            usize::try_from(number("total")?).map_err(|_| "node validator count overflow")?;
        let entries = result
            .get("validators")
            .and_then(serde_json::Value::as_array)
            .ok_or("node validators missing")?;
        if entries.is_empty() || total == 0 || total > 10_000 {
            return Err("node validator count invalid".to_owned());
        }
        for entry in entries {
            if entry
                .pointer("/pub_key/type")
                .and_then(serde_json::Value::as_str)
                != Some("tendermint/PubKeyEd25519")
            {
                return Err("node validator key type unsupported".to_owned());
            }
            let public_key: [u8; 32] = base64::engine::general_purpose::STANDARD
                .decode(
                    entry
                        .pointer("/pub_key/value")
                        .and_then(serde_json::Value::as_str)
                        .ok_or("node validator key missing")?,
                )
                .map_err(|_| "node validator key malformed")?
                .try_into()
                .map_err(|_| "node validator key width")?;
            let address_text = entry
                .get("address")
                .and_then(serde_json::Value::as_str)
                .ok_or("node validator address missing")?;
            let mut address = [0; 20];
            if address_text.len() != 40
                || !address_text.bytes().all(|byte| byte.is_ascii_hexdigit())
            {
                return Err("node validator address width".to_owned());
            }
            for (index, byte) in address.iter_mut().enumerate() {
                *byte = u8::from_str_radix(&address_text[index * 2..index * 2 + 2], 16)
                    .map_err(|_| "node validator address malformed")?;
            }
            let voting_power = entry
                .get("voting_power")
                .and_then(serde_json::Value::as_str)
                .and_then(|value| value.parse().ok())
                .ok_or("node validator power malformed")?;
            validators.push(layerx_agentd::read::CommitValidator {
                address,
                public_key,
                voting_power,
            });
        }
        if validators.len() == total {
            return layerx_agentd::export::ExportTrustSource::new(receipt_keys, validators)
                .map_err(|error| format!("export trust is invalid: {error:?}"));
        }
        if validators.len() > total {
            return Err("node validator count disagrees".to_owned());
        }
    }
    Err("node validator pagination exhausted".to_owned())
}

type OwnerStatus = mpsc::Receiver<Result<(), String>>;

fn start_human_owner(mcp: Option<McpBoot>) -> Result<OwnerStatus, String> {
    let runtime_clock = layerx_client::runtime_clock::RuntimeClock::from_environment()
        .map_err(|error| format!("runtime clock unavailable: {error}"))?;
    let tenants = human_policy_tenants(&human_peers()?)?;
    let native_policy_sources = native_policy_sources(&tenants)?;
    let native_effect_policy_sources = native_effect_policy_sources(&tenants)?;
    let denomination_sources = program_budget_denomination_sources(&tenants)?;
    start_shared_owner(
        mcp,
        None,
        None,
        None,
        &native_policy_sources,
        &native_effect_policy_sources,
        &denomination_sources,
        runtime_clock,
    )
    .map(|(receiver, _, _, _)| receiver)
}

fn start_shared_owner(
    mcp: Option<McpBoot>,
    programs: Option<ProgramOperations>,

    export_trust: Option<layerx_agentd::export::ExportTrustSource>,

    policy_sources: Option<&BTreeMap<TenantId, PathBuf>>,
    native_policy_sources: &BTreeMap<TenantId, PathBuf>,
    native_effect_policy_sources: &BTreeMap<TenantId, PathBuf>,
    program_budget_denomination_sources: &BTreeMap<TenantId, PathBuf>,

    clock: Arc<dyn layerx_types::clock::Clock>,
) -> Result<
    (
        OwnerStatus,
        SharedAgentOwner<RemoteHumanAuthority>,
        mpsc::SyncSender<Result<(), String>>,
        Arc<Mutex<Store>>,
    ),
    String,
> {
    let peers = human_peers()?;
    let tenants = human_policy_tenants(&peers)?;
    if !native_policy_sources.keys().eq(tenants.iter()) {
        return Err(
            "LAYERX_NATIVE_POLICY_SOURCES must cover exactly the configured Human tenants"
                .to_owned(),
        );
    }
    if program_budget_denomination_sources
        .keys()
        .any(|tenant| !tenants.contains(tenant))
    {
        return Err(
            "Program budget denomination sources name an unconfigured Human tenant".to_owned(),
        );
    }
    let deadline = Duration::from_millis(parse_u64("LAYERX_AGENT_HUMAN_DEADLINE_MS")?);
    let human_limits = human_frame_limits(deadline)?;
    let store_path = PathBuf::from(required("LAYERX_AGENT_HUMAN_STORE")?);
    let socket_path = PathBuf::from(required("LAYERX_AGENT_HUMAN_SOCKET")?);
    let session_key_root = PathBuf::from(required("LAYERX_AGENT_HUMAN_SESSION_KEY_ROOT")?);
    let session_secret_path =
        PathBuf::from(required("LAYERX_AGENT_HUMAN_SESSION_OPERATOR_SECRET_FILE")?);
    if !store_path.is_absolute()
        || !socket_path.is_absolute()
        || !session_key_root.is_absolute()
        || !session_secret_path.is_absolute()
    {
        return Err("human daemon paths must be absolute".to_owned());
    }
    let node = connect_human_node()?;
    let mut authority = connect_human_authority(deadline, &peers)?;
    let shared_store =
        Arc::new(Mutex::new(Store::open(&store_path).map_err(|error| {
            format!("human store is unavailable: {error}")
        })?));
    let mut mcp_republish = None;
    if let Some(boot) = mcp {
        mcp_republish = Some(publish_mcp_binding(
            &mut authority,
            &peers,
            &shared_store,
            &store_path,
            boot,
        )?);
    }
    let mut operations = ProductionHumanOperations::new(
        authority,
        node,
        Box::new({
            let config = NodeRpcConfig::from_environment().map_err(|error| {
                format!(
                    "human node RPC requires LAYERX_AGENT_NODE_RPC_URL and LAYERX_AGENT_NODE_CHAIN_ID: {error}"
                )
            })?;
            NodeRpc::connect(&config)
                .map_err(|error| format!("human node RPC is unavailable: {error}"))?
        }),
        Arc::clone(&shared_store),
        &peers,
        required("LAYERX_AGENT_HUMAN_MAX_PAYLOAD_BYTES")?
            .parse()
            .map_err(|_| "human payload bound is invalid")?,
        parse_u64("LAYERX_AGENT_HUMAN_TIMESTAMP_SPAN")?,
        clock,
    )
    .map_err(|error| format!("human operations are invalid: {error:?}"))?;
    if let Some(source) = export_trust {
        operations.install_export_trust(source, deadline);
    }

    if let Some(sources) = policy_sources {
        operations
            .attach_policies(sources)
            .map_err(|error| format!("tenant policies are invalid: {error:?}"))?;
    }

    operations
        .attach_native_policies(native_policy_sources)
        .map_err(|error| format!("native tenant policies are invalid: {error}"))?;
    operations
        .attach_native_effect_policies(native_effect_policy_sources)
        .map_err(|error| format!("native effect policy source invalid: {error}"))?;

    operations
        .attach_program_budget_denominations(program_budget_denomination_sources)
        .map_err(|error| format!("Program budget denominations are invalid: {error}"))?;

    let socket_uid = required("LAYERX_AGENT_HUMAN_SOCKET_UID")?
        .parse()
        .map_err(|_| "human socket uid is invalid")?;
    let operator_secret = layerx_agentd::config::read_protected_source(&session_secret_path, 4096)
        .map_err(|error| format!("human session operator secret is unavailable: {error:?}"))?;
    let session_keys = SessionKeyRegistry::open(
        session_key_root,
        operator_secret,
        required("LAYERX_AGENT_HUMAN_NETWORK_ID")?
            .parse()
            .map_err(|_| "human network id is invalid")?,
        socket_uid,
    )
    .map_err(|error| format!("human session key registry is invalid: {error:?}"))?;
    let mut unified = UnifiedAgentOwner::new(
        operations,
        Arc::clone(&shared_store),
        &peers,
        vec![verified_limit()?],
        session_keys,
    )
    .map_err(|error| format!("human owner is invalid: {error:?}"))?;
    unified.programs = programs;
    if let Some(provider) = configured_registry_source()? {
        unified
            .attach_registry_source(provider)
            .map_err(|error| format!("registry source provider attachment refused: {error:?}"))?;
    }
    unified.mcp_enrolment = mcp_republish;
    let owner = SharedAgentOwner::new(unified);
    let server = HumanUnixServer::bind(
        HumanListenerConfig {
            endpoint: socket_path,
            owner_uid: socket_uid,
            owner_gid: required("LAYERX_AGENT_HUMAN_SOCKET_GID")?
                .parse()
                .map_err(|_| "human socket gid is invalid")?,
            mode: u32::from_str_radix(&required("LAYERX_AGENT_HUMAN_SOCKET_MODE")?, 8)
                .map_err(|_| "human socket mode is invalid")?,
            maximum_frame_bytes: human_limits.maximum_frame_bytes,
            maximum_connections: human_limits.maximum_connections,
            maximum_streams: human_limits.maximum_streams,
            maximum_queued_bytes: human_limits.maximum_queued_bytes,
            deadline,
            peers,
        },
        owner.clone(),
    )
    .map_err(|error| format!("human listener is invalid: {error:?}"))?;
    let (sender, receiver) = mpsc::sync_channel(2);
    let status = sender.clone();
    thread::Builder::new()
        .name("layerx-agent-human".to_owned())
        .spawn(move || {
            let _ = status.send(
                server
                    .serve()
                    .map_err(|error| format!("human listener stopped: {error:?}")),
            );
        })
        .map_err(|error| format!("human listener thread failed: {error}"))?;
    Ok((receiver, owner, sender, shared_store))
}

/// Starts the dedicated mutually authenticated agent RPC listener when it is
/// configured. A configured listener without its complete TLS material refuses boot.
fn start_agent_rpc(
    owner: SharedAgentOwner<RemoteHumanAuthority>,
    shared_store: Arc<Mutex<Store>>,
) -> Result<Option<HttpPool>, String> {
    let Some(listen) = optional("LAYERX_AGENTD_RPC_LISTEN") else {
        return Ok(None);
    };
    let owner = owner
        .with_idempotency(
            absolute_path("LAYERX_AGENTD_RPC_IDEMPOTENCY_ROOT")?,
            parse_u64("LAYERX_AGENTD_RPC_IDEMPOTENCY_DAEMON_SEQUENCES")?,
            parse_u64("LAYERX_AGENTD_RPC_IDEMPOTENCY_PROTOCOL_SEQUENCES")?,
        )
        .map_err(|error| format!("agent rpc idempotency is invalid: {error:?}"))?;
    let tls = AgentRpcTls::from_paths(&AgentRpcTlsPaths {
        cert: absolute_path("LAYERX_AGENTD_RPC_TLS_CERT")?,
        key: absolute_path("LAYERX_AGENTD_RPC_TLS_KEY")?,
        client_ca: absolute_path("LAYERX_AGENTD_RPC_TLS_CLIENT_CA")?,
        peer: required("LAYERX_AGENTD_RPC_PEER")?,
    })
    .map_err(|error| format!("agent rpc tls is invalid: {error:?}"))?;
    let network = required("LAYERX_NODE_NETWORK_NAME")?;
    if network.is_empty()
        || network.len() > 64
        || !network
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_'))
    {
        return Err("agent rpc network name is invalid".to_owned());
    }

    let mut mcp_bindings = optional("LAYERX_AGENTD_MCP_BINDINGS")
        .map(|value| {
            value
                .split(',')
                .map(|path| {
                    layerx_mcp::binding::Binding::open(Path::new(path))
                        .map_err(|_| "daemon MCP binding is invalid".to_owned())
                })
                .collect::<Result<Vec<_>, String>>()
        })
        .transpose()?
        .unwrap_or_default();
    attach_mcp_payers(&owner, &shared_store, &mut mcp_bindings)?;
    let listener = TcpListener::bind(&listen)
        .map_err(|error| format!("agent rpc listener failed: {error}"))?;
    HttpPool::start(listener, move |stream| {
        let Ok(mut stream) = tls.accept(stream) else {
            return;
        };
        let response = agent_rpc_exchange(&mut stream, &network, &owner, &mcp_bindings);
        if matches!(&response, RpcExchangeResponse::Mcp { .. })
            && stream.sock.socket.set_nonblocking(true).is_err()
        {
            return;
        }
        if response.write(&mut stream, &owner).is_err() {
            return;
        }
        stream.conn.send_close_notify();
        let _ = stream.flush();
    })
    .map(Some)
}

#[derive(serde::Deserialize)]
#[serde(deny_unknown_fields)]
struct McpPayerConfig {
    tenant: String,
    session_id: String,
    payer_did: String,
    signer: layerx_human_kms::attestor::AttestorSignerConfig,
    assertion_source: PathBuf,
}

fn mcp_config_digest(value: &str) -> Result<[u8; 32], String> {
    if value.len() != 64
        || !value
            .bytes()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
    {
        return Err("MCP payer digest is invalid".to_owned());
    }
    let mut output = [0_u8; 32];
    for (index, pair) in value.as_bytes().chunks_exact(2).enumerate() {
        output[index] = u8::from_str_radix(
            std::str::from_utf8(pair).map_err(|_| "MCP payer digest is invalid")?,
            16,
        )
        .map_err(|_| "MCP payer digest is invalid")?;
    }
    if output == [0; 32] {
        return Err("MCP payer digest is invalid".to_owned());
    }
    Ok(output)
}

fn attach_mcp_payers(
    owner: &SharedAgentOwner<RemoteHumanAuthority>,
    shared_store: &Arc<Mutex<Store>>,
    bindings: &mut [layerx_mcp::binding::Binding],
) -> Result<(), String> {
    let Some(source) = optional("LAYERX_AGENTD_MCP_PAYERS") else {
        return Ok(());
    };
    let bytes = zeroize::Zeroizing::new(
        layerx_agentd::config::read_protected_source(Path::new(&source), 1_048_576)
            .map_err(|_| "MCP payer source is invalid")?,
    );
    let configs: Vec<McpPayerConfig> =
        serde_json::from_slice(&bytes).map_err(|_| "MCP payer source is invalid")?;
    if configs.is_empty() || configs.len() > 64 {
        return Err("MCP payer configuration count is invalid".to_owned());
    }
    let limit = mcp_limit()?;
    let limit_id = limit.id;
    let limiter =
        Arc::new(BudgetLimiter::new(vec![limit]).map_err(|_| "MCP payer budget limit is invalid")?);
    let mut attached = BTreeSet::new();
    for config in configs {
        let session_id = mcp_config_digest(&config.session_id)?;
        let coordinate = (config.tenant.clone(), session_id);
        if !attached.insert(coordinate) {
            return Err("MCP payer binding is duplicated".to_owned());
        }
        let binding = bindings
            .iter_mut()
            .find(|binding| {
                binding.daemon_credential().is_ok_and(|credential| {
                    credential.tenant().as_str() == config.tenant
                        && credential.session_id().0 == session_id
                })
            })
            .ok_or("MCP payer has no configured binding")?;
        let credential = binding
            .daemon_credential()
            .map_err(|_| "MCP payer credential is unavailable")?;
        let assertion = zeroize::Zeroizing::new(
            layerx_agentd::config::read_protected_source(&config.assertion_source, 8192)
                .map_err(|_| "MCP payer assertion source is invalid")?,
        );
        let assertion = std::str::from_utf8(&assertion)
            .map_err(|_| "MCP payer assertion is invalid")?
            .trim();
        if assertion.is_empty() || !assertion.bytes().all(|byte| byte.is_ascii_graphic()) {
            return Err("MCP payer assertion is invalid".to_owned());
        }
        let mut signer = layerx_human_kms::attestor::AttestorSigner::from_config(&config.signer)
            .map_err(|_| "MCP payer attestor is invalid")?;
        signer
            .set_evm_assertion(assertion)
            .map_err(|_| "MCP payer EVM assertion is invalid")?;
        let endpoint = required("LAYERX_AGENT_NODE_ENDPOINT")?;
        let ca_der = read_ca("LAYERX_AGENT_AUTHORITY_CA_DER")?;
        let node = NodeRpc::connect_with_ca(
            &NodeRpcConfig {
                url: endpoint.clone(),
                chain_id: 125,
                deadline: Duration::from_secs(30),
            },
            &ca_der,
        )
        .map_err(|_| "MCP payer node connection is unavailable")?;
        let receipt_keys = node
            .receipt_keys()
            .map_err(|_| "MCP payer receipt keys are unavailable")?;
        let payer = mcp_payer::ProductionWebPayer::from_owner(
            owner.clone(),
            credential,
            config.payer_did,
            Arc::new(signer),
            &endpoint,
            &[&ca_der],
            receipt_keys,
        )
        .map_err(|_| "MCP payer owner authority is unavailable")?
        .with_approval_resources(
            Arc::clone(shared_store),
            Arc::clone(&limiter),
            vec![limit_id],
        );
        let approvals = owner
            .lock()
            .map_err(|_| "MCP approval owner is unavailable")?
            .approvals
            .clone();
        binding.attach_web(layerx_mcp::binding::WebAuthority::new(
            owner
                .mcp_clock()
                .map_err(|_| "MCP approval clock is unavailable")?,
            approvals,
            Box::new(payer.into_web_payer()),
        ));
    }
    Ok(())
}

enum RpcExchangeResponse {
    Ordinary(AgentRpcResponse),
    Mcp {
        id: layerx_agent_api::error::RequestId,
        result: layerx_mcp::server::DaemonToolResult,
    },
}

impl From<AgentRpcResponse> for RpcExchangeResponse {
    fn from(value: AgentRpcResponse) -> Self {
        Self::Ordinary(value)
    }
}

fn mcp_authority_response(
    id: layerx_agent_api::error::RequestId,
    error: &layerx_mcp::server::ServerError,
) -> AgentRpcResponse {
    use layerx_mcp::server::ServerError;
    let (class, reason) = match error {
        ServerError::DurableInvocation(
            layerx_agentd::session_control::McpInvocationError::Control(error),
        ) => {
            return agent_rpc::mcp_control_refusal(id, error);
        }
        ServerError::DurableInvocation(
            layerx_agentd::session_control::McpInvocationError::Conflict,
        ) => (ErrorClass::IdempotencyConflict, "mcp.invocation_conflict"),
        ServerError::DurableInvocation(_) => {
            (ErrorClass::InternalFault, "mcp.invocation_unavailable")
        }
        ServerError::InvocationUnknown => (ErrorClass::TransportFailure, "outcome.unknown"),
        ServerError::RevokedSession | ServerError::ClosedSession => {
            (ErrorClass::PolicyRefusal, "session.revoked")
        }
        ServerError::ExpiredAuthority => (ErrorClass::PolicyRefusal, "session.expired"),
        ServerError::ToolAbsent | ServerError::NoScope => {
            (ErrorClass::PolicyRefusal, "session.scope_denied")
        }
        ServerError::TenantMismatch | ServerError::CapabilityMismatch => {
            (ErrorClass::PolicyRefusal, "session.not_authorized")
        }
        _ => (ErrorClass::InternalFault, "unavailable"),
    };
    agent_rpc::mcp_result_refusal(id, class, reason)
}

impl RpcExchangeResponse {
    fn write<S: Write>(
        self,
        stream: &mut S,
        owner: &SharedAgentOwner<RemoteHumanAuthority>,
    ) -> Result<(), String> {
        match self {
            Self::Ordinary(response) => write_rpc_response(stream, &response),
            Self::Mcp { id, result } => {
                let sequence = owner
                    .current_core_sequence()
                    .map_err(|_| "MCP current head is unavailable".to_owned())?;
                let mut attempted = false;
                let outcome = result.release_current(sequence, |value| {
                    use layerx_mcp::boundary::BoundaryRefusal;
                    let response = match value {
                        Ok(value) => agent_rpc::mcp_result_response(id, value),
                        Err(BoundaryRefusal::Owner(mut error)) => {
                            error.request_id = id;
                            agent_rpc::mcp_api_refusal(&error)
                        }
                        Err(BoundaryRefusal::Unavailable(_)) => agent_rpc::mcp_result_refusal(
                            id,
                            ErrorClass::TransportFailure,
                            "outcome.unknown",
                        ),
                        Err(BoundaryRefusal::Unauthorized) => agent_rpc::mcp_result_refusal(
                            id,
                            ErrorClass::PolicyRefusal,
                            "session.not_authorized",
                        ),
                        Err(BoundaryRefusal::Held { .. }) => agent_rpc::mcp_result_refusal(
                            id,
                            ErrorClass::PolicyRefusal,
                            "mcp.awaiting_approval",
                        ),
                        Err(BoundaryRefusal::NotServed(_) | BoundaryRefusal::UnsupportedRead) => {
                            agent_rpc::mcp_result_refusal(
                                id,
                                ErrorClass::UnavailableCapability,
                                "mcp.tool_not_served",
                            )
                        }
                        Err(BoundaryRefusal::Malformed(_)) => agent_rpc::mcp_result_refusal(
                            id,
                            ErrorClass::ProtocolIncompatibility,
                            "mcp.malformed",
                        ),
                    };
                    attempted = true;
                    write_rpc_response(stream, &response).map_err(|_| {
                        layerx_agentd::session_control::SessionControlError::Unavailable
                    })
                });
                match outcome {
                    Ok(()) => Ok(()),
                    Err(error) if !attempted => {
                        write_rpc_response(stream, &mcp_authority_response(id, &error))
                    }
                    Err(_) => Err("MCP response release outcome is unknown".to_owned()),
                }
            }
        }
    }
}

fn agent_rpc_exchange<S: Read>(
    stream: &mut S,
    network: &str,
    owner: &SharedAgentOwner<RemoteHumanAuthority>,
    mcp_bindings: &[layerx_mcp::binding::Binding],
) -> RpcExchangeResponse {
    let malformed = || {
        agent_rpc::refusal(
            400,
            ErrorClass::ProtocolIncompatibility,
            "envelope.malformed",
        )
    };
    let mut bytes = vec![0_u8; HEADER_LIMIT];
    let mut length = 0_usize;
    let head_end = loop {
        if let Some(end) = bytes[..length]
            .windows(4)
            .position(|value| value == b"\r\n\r\n")
        {
            break end + 4;
        }
        if length == bytes.len() {
            return agent_rpc::refusal(
                431,
                ErrorClass::ProtocolIncompatibility,
                "envelope.malformed",
            )
            .into();
        }
        match stream.read(&mut bytes[length..]) {
            Ok(0) | Err(_) => return malformed().into(),
            Ok(count) => length += count,
        }
    };
    let Ok(head) = std::str::from_utf8(&bytes[..head_end - 4]) else {
        return malformed().into();
    };
    let mut lines = head.split("\r\n");
    let mut parts = lines.next().unwrap_or_default().split(' ');
    let (method, path, version) = (parts.next(), parts.next(), parts.next());
    if version != Some("HTTP/1.1") || parts.next().is_some() {
        return malformed().into();
    }
    let health = path == Some("/healthz");
    if path != Some("/rpc") && !health {
        return agent_rpc::refusal(
            404,
            ErrorClass::ProtocolIncompatibility,
            "envelope.malformed",
        )
        .into();
    }
    if method != Some(if health { "GET" } else { "POST" }) {
        return agent_rpc::refusal(
            405,
            ErrorClass::ProtocolIncompatibility,
            "envelope.malformed",
        )
        .into();
    }
    let mut content_length = None;
    for line in lines {
        let Some((name, value)) = line.split_once(':') else {
            return malformed().into();
        };
        let value = value.trim_matches(|c| c == ' ' || c == '\t');
        if name.eq_ignore_ascii_case("transfer-encoding") {
            return malformed().into();
        }
        if [
            "authorization",
            "layerx-key",
            "layerx-tenant",
            "layerx-agent",
        ]
        .iter()
        .any(|principal| name.eq_ignore_ascii_case(principal))
        {
            return agent_rpc::refusal(403, ErrorClass::PolicyRefusal, "envelope.header_principal")
                .into();
        }
        if name.eq_ignore_ascii_case("content-length") {
            let canonical = !value.is_empty()
                && value.bytes().all(|byte| byte.is_ascii_digit())
                && (value == "0" || !value.starts_with('0'));
            let parsed = value.parse::<usize>().ok().filter(|_| canonical);
            if content_length.is_some() || parsed.is_none() {
                return malformed().into();
            }
            content_length = parsed;
        }
    }
    if health {
        if content_length.unwrap_or(0) != 0 || length != head_end {
            return malformed().into();
        }
        return agent_rpc_health(owner, network).into();
    }
    let Some(content_length) = content_length else {
        return malformed().into();
    };
    if content_length > agent_rpc::MAX_BODY_BYTES {
        return agent_rpc::refusal(
            413,
            ErrorClass::ProtocolIncompatibility,
            "envelope.oversized",
        )
        .into();
    }
    let mut body = bytes[head_end..length].to_vec();
    if body.len() > content_length {
        return malformed().into();
    }
    let missing = content_length - body.len();
    body.resize(content_length, 0);
    if stream
        .read_exact(&mut body[content_length - missing..])
        .is_err()
    {
        return malformed().into();
    }
    let invocation = match agent_rpc::decode_mcp_invocation(&body) {
        Ok(None) => return agent_rpc::handle_rpc(owner, &body).into(),
        Err(response) => return response.into(),
        Ok(Some(invocation)) => invocation,
    };
    let id = invocation.request_id;
    let Some(binding) = mcp_bindings
        .iter()
        .find(|binding| binding.matches_session(&invocation.credential))
    else {
        return agent_rpc::mcp_result_refusal(
            id,
            ErrorClass::UnavailableCapability,
            "mcp.owner_unavailable",
        )
        .into();
    };
    let control = match owner.lock() {
        Ok(owner) => owner.session_control.clone(),
        Err(_) => {
            return agent_rpc::mcp_result_refusal(id, ErrorClass::InternalFault, "unavailable")
                .into()
        }
    };
    let sequence = match owner.current_core_sequence() {
        Ok(sequence) => sequence,
        Err(_) => {
            return agent_rpc::mcp_result_refusal(id, ErrorClass::InternalFault, "unavailable")
                .into()
        }
    };
    use sha2::Digest as _;
    let encoded = match serde_json::to_vec(&invocation.arguments) {
        Ok(encoded) => encoded,
        Err(_) => {
            return agent_rpc::mcp_result_refusal(
                id,
                ErrorClass::ProtocolIncompatibility,
                "mcp.malformed",
            )
            .into()
        }
    };
    let request_digest = sha2::Sha256::new()
        .chain_update(b"LXP/mcp/daemon-invocation/v1\0")
        .chain_update((invocation.tool.len() as u64).to_be_bytes())
        .chain_update(invocation.tool.as_bytes())
        .chain_update(encoded)
        .finalize()
        .into();
    let tool_name = if invocation.tool == "mcp.describe" {
        "mcp.describe"
    } else {
        let Some(tool) = layerx_mcp::server::catalogue()
            .iter()
            .chain(layerx_mcp::catalogue::WEB_TOOLS.iter())
            .chain(layerx_mcp::catalogue::SESSION_TOOLS.iter())
            .find(|tool| tool.name == invocation.tool)
        else {
            return agent_rpc::mcp_result_refusal(id, ErrorClass::PolicyRefusal, "mcp.tool_absent")
                .into();
        };
        tool.name
    };
    let identity = layerx_agentd::session_control::McpInvocationIdentity {
        tool_name,
        idempotency_key: invocation.idempotency_key,
        request_digest,
    };
    let environment = match agent_rpc::mcp_owner_environment(owner, id, &invocation.credential) {
        Ok(environment) => Some(layerx_mcp::binding::OwnerEnvironment {
            registry: environment.registry,
            core_time_ms: environment.core_time_ms,
            head_sequence: environment.head_sequence,
            native_effect_profile: environment.native_effect_profile,
            native_send_profile: environment.native_send_profile,
        }),
        Err(_) => None,
    };
    match binding.daemon_invocation(
        control,
        invocation.credential,
        sequence,
        &invocation.tool,
        &invocation.arguments,
        identity,
        environment,
        |operation, credential, arguments, key| {
            agent_rpc::dispatch_mcp_owner(owner, id, credential, operation, arguments, key)
        },
    ) {
        Ok(result) => RpcExchangeResponse::Mcp { id, result },
        Err(layerx_mcp::binding::DaemonInvocationError::Authority(error)) => {
            mcp_authority_response(id, &error).into()
        }
        Err(layerx_mcp::binding::DaemonInvocationError::Binding(_)) => {
            agent_rpc::mcp_result_refusal(
                id,
                ErrorClass::UnavailableCapability,
                "mcp.owner_unavailable",
            )
            .into()
        }
    }
}

/// Readiness and negotiated node identity for the gateway's binding health check.
fn agent_rpc_health(
    owner: &SharedAgentOwner<RemoteHumanAuthority>,
    network: &str,
) -> AgentRpcResponse {
    let health = owner.lock().and_then(|guard| guard.rpc_health());
    match health {
        Ok((ready, _, protocol_version)) => AgentRpcResponse {
            status: 200,
            body: format!(
                "{{\"ready\":{ready},\"network_id\":\"{network}\",\"wire_version\":\"{protocol_version}\",\"transport_ready\":true,\"verified_reads_ready\":null,\"tenant_writes\":\"unknown\"}}"
            )
            .into_bytes(),
        },
        Err(_) => agent_rpc::refusal(503, ErrorClass::InternalFault, "owner.unavailable"),
    }
}

fn write_rpc_response<S: Write>(stream: &mut S, response: &AgentRpcResponse) -> Result<(), String> {
    let reason = if response.status < 300 {
        "OK"
    } else {
        "Refused"
    };
    let header = format!(
        "HTTP/1.1 {} {reason}\r\nContent-Type: application/json\r\nCache-Control: no-store\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        response.status,
        response.body.len()
    );
    stream
        .write_all(header.as_bytes())
        .and_then(|()| stream.write_all(&response.body))
        .map_err(|error| format!("agent rpc response failed: {error}"))
}

fn human_policy_tenants(
    peers: &BTreeMap<u32, (String, String)>,
) -> Result<BTreeSet<TenantId>, String> {
    peers
        .values()
        .map(|(_, tenant)| TenantId::new(tenant.clone()))
        .collect::<Result<BTreeSet<_>, _>>()
        .map_err(|error| format!("human policy tenant is invalid: {error:?}"))
}

fn native_policy_sources(
    tenants: &BTreeSet<TenantId>,
) -> Result<BTreeMap<TenantId, PathBuf>, String> {
    layerx_agentd::config::parse_policy_sources(&required("LAYERX_NATIVE_POLICY_SOURCES")?, tenants)
        .map_err(|error| format!("LAYERX_NATIVE_POLICY_SOURCES is invalid: {error}"))
}

fn native_effect_policy_sources(
    tenants: &BTreeSet<TenantId>,
) -> Result<BTreeMap<TenantId, PathBuf>, String> {
    const SETTING: &str = "LAYERX_NATIVE_EFFECT_POLICY_SOURCES";
    let value = match env::var(SETTING) {
        Ok(value) => value,
        Err(env::VarError::NotPresent) => return Ok(BTreeMap::new()),
        Err(env::VarError::NotUnicode(_)) => return Err(format!("{SETTING} is not UTF-8")),
    };
    let mut declared = BTreeSet::new();
    for declaration in value.split(',') {
        let (tenant, _) = declaration
            .split_once(':')
            .ok_or_else(|| format!("{SETTING} requires tenant:absolute-path entries"))?;
        let tenant = TenantId::new(tenant.trim().to_owned())
            .map_err(|_| format!("{SETTING} contains an invalid tenant"))?;
        if !tenants.contains(&tenant) || !declared.insert(tenant) {
            return Err(format!("{SETTING} contains an unknown or duplicate tenant"));
        }
    }
    layerx_agentd::config::parse_policy_sources(&value, &declared)
        .map_err(|error| format!("{SETTING} is invalid: {error}"))
}

fn program_budget_denomination_sources(
    tenants: &BTreeSet<TenantId>,
) -> Result<BTreeMap<TenantId, PathBuf>, String> {
    const SETTING: &str = "LAYERX_PROGRAM_BUDGET_DENOMINATION_SOURCES";
    let value = match env::var(SETTING) {
        Ok(value) => value,
        Err(env::VarError::NotPresent) => return Ok(BTreeMap::new()),
        Err(env::VarError::NotUnicode(_)) => return Err(format!("{SETTING} is not UTF-8")),
    };
    let mut opted_in = BTreeSet::new();
    for declaration in value.split(',') {
        let (tenant, _) = declaration
            .split_once(':')
            .ok_or_else(|| format!("{SETTING} requires tenant:absolute-path entries"))?;
        let tenant = TenantId::new(tenant.trim().to_owned())
            .map_err(|_| format!("{SETTING} contains an invalid tenant"))?;
        if !tenants.contains(&tenant) || !opted_in.insert(tenant) {
            return Err(format!("{SETTING} contains an unknown or duplicate tenant"));
        }
    }
    layerx_agentd::config::parse_policy_sources(&value, &opted_in)
        .map_err(|error| format!("{SETTING} is invalid: {error}"))
}

fn config() -> Result<Config, String> {
    let inference = match human_owner_mode::Mode::from_environment()? {
        human_owner_mode::Mode::InferenceMarket => {
            Some(layerx_agentd::config::InferenceConfig::from_environment()?)
        }
        _ => None,
    };
    let listen = required("LAYERX_AGENT_PROGRAM_LISTEN")?;
    let bearer = required("LAYERX_AGENT_PROGRAM_BEARER_TOKEN")?;
    let node_bearer = required("LAYERX_AGENT_NODE_BEARER_TOKEN")?;
    let authority_bearer = required("LAYERX_AGENT_AUTHORITY_BEARER_TOKEN")?;
    if !listen.starts_with("127.0.0.1:")
        || bearer.len() < 32
        || node_bearer.len() < 32
        || authority_bearer.len() < 32
        || bearer == node_bearer
        || bearer == authority_bearer
    {
        return Err(
            "agent program reads require loopback and distinct bounded credentials".to_owned(),
        );
    }
    let staleness_ms = parse_u64("LAYERX_AGENT_PROGRAM_MAX_STALENESS_MS")?;
    if staleness_ms == 0 {
        return Err("agent staleness bound is non-canonical".to_owned());
    }
    let tenants = human_policy_tenants(&human_peers()?)?;
    let policy_sources =
        layerx_agentd::config::parse_policy_sources(&required("LAYERX_POLICY_SOURCES")?, &tenants)
            .map_err(|error| format!("human policy sources are invalid: {error}"))?;
    let native_policy_sources = native_policy_sources(&tenants)?;
    let native_effect_policy_sources = native_effect_policy_sources(&tenants)?;
    let program_budget_denomination_sources = program_budget_denomination_sources(&tenants)?;
    Ok(Config {
        inference,
        listen,
        policy_sources,
        native_policy_sources,
        native_effect_policy_sources,
        program_budget_denomination_sources,
        bearer,
        node_endpoint: required("LAYERX_AGENT_NODE_ENDPOINT")?,
        expected_chain_id: parse_u64("LAYERX_EXPECTED_CHAIN_ID")?,
        node_bearer,
        authority_endpoint: required("LAYERX_AGENT_AUTHORITY_ENDPOINT")?,
        authority_bearer,
        authority_ca_der: read_ca("LAYERX_AGENT_AUTHORITY_CA_DER")?,
        authority_replica_id: parse_digest("LAYERX_AGENT_AUTHORITY_REPLICA_ID")?,
        chain_trust_root: absolute_path("LAYERX_AGENT_CHAIN_TRUST_ROOT")?,
        staleness_ms,
        deployment_journal: required("LAYERX_AGENT_DEPLOYMENT_JOURNAL")?,
        probe_program: ProgramId::new(parse_digest("LAYERX_AGENT_PROGRAM_PROBE_ID")?)
            .map_err(|error| format!("LAYERX_AGENT_PROGRAM_PROBE_ID is invalid: {error}"))?,
    })
}

fn load_registry(root: &Path, reader: &LayerxdProgramBalanceReader) -> Result<Registry, String> {
    let mut paths = fs::read_dir(root)
        .map_err(|error| format!("deployment journal is unavailable: {error}"))?
        .map(|entry| entry.map(|value| value.path()))
        .collect::<Result<Vec<_>, _>>()
        .map_err(|error| format!("deployment journal is unreadable: {error}"))?;
    paths.retain(|path| path.extension().is_some_and(|value| value == "admission"));
    paths.sort();
    let mut registry = Registry::new();
    for path in paths {
        let bytes = fs::read(&path)
            .map_err(|error| format!("{} is unreadable: {error}", path.display()))?;
        let proof = DeploymentProof::decode(&bytes)
            .map_err(|error| format!("{} is corrupt: {error}", path.display()))?;
        let evidence = reader
            .verify_journal_admission(&proof, now_ms()?)
            .map_err(|error| format!("{} is unverified: {error:?}", path.display()))?;
        let expected = hex::encode(&evidence.receipt_digest());
        if path.file_stem().and_then(|value| value.to_str()) != Some(expected.as_str()) {
            return Err(format!(
                "{} is filed under the wrong receipt",
                path.display()
            ));
        }
        let record_path = root.join(format!("{expected}.deployment"));
        let record = DeploymentRecord::decode(
            &fs::read(&record_path)
                .map_err(|error| format!("{} is unreadable: {error}", record_path.display()))?,
        )
        .map_err(|error| format!("{} is corrupt: {error}", record_path.display()))?;
        record
            .validate()
            .map_err(|error| format!("{} is inadmissible: {error}", record_path.display()))?;
        if &record != evidence.record() {
            return Err(format!(
                "{} disagrees with protocol evidence",
                record_path.display()
            ));
        }
        registry
            .record_verified_deployment(&evidence)
            .map_err(|error| format!("verified deployment replay failed: {error}"))?;
    }
    if registry.program_ids().is_empty() {
        return Err("deployment journal contains no verified admissions".to_owned());
    }
    Ok(registry)
}

fn now_ms() -> Result<u64, String> {
    let value = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|_| "system time precedes the Unix epoch".to_owned())?;
    u64::try_from(value.as_millis()).map_err(|_| "system time is out of range".to_owned())
}

fn load_chain_trust_root(
    path: &Path,
) -> Result<layerx_programs::chain_proof::trust_store::TrustRoot, String> {
    use std::os::unix::fs::{MetadataExt as _, PermissionsExt as _};

    const SETTING: &str = "LAYERX_AGENT_CHAIN_TRUST_ROOT";
    const MAX_BYTES: u64 = 1_048_576;
    let unavailable = || format!("{SETTING} must be a protected regular file");
    let link = fs::symlink_metadata(path).map_err(|_| unavailable())?;
    if !link.is_file() || link.permissions().mode() & 0o077 != 0 {
        return Err(unavailable());
    }
    let file = fs::File::open(path).map_err(|_| unavailable())?;
    let metadata = file.metadata().map_err(|_| unavailable())?;
    if !metadata.is_file()
        || metadata.permissions().mode() & 0o077 != 0
        || link.dev() != metadata.dev()
        || link.ino() != metadata.ino()
        || metadata.len() == 0
        || metadata.len() > MAX_BYTES
    {
        return Err(unavailable());
    }
    let mut bytes = Vec::new();
    file.take(MAX_BYTES + 1)
        .read_to_end(&mut bytes)
        .map_err(|_| unavailable())?;
    if bytes.len() as u64 != metadata.len() {
        return Err(unavailable());
    }
    layerx_programs::chain_proof::trust_store::TrustRoot::decode(&bytes)
        .map_err(|error| format!("{SETTING} is invalid: {error:?}"))
}

struct ProgramAuthorityBoot {
    verifier: ProtocolDeploymentVerifier,
    registry: Registry,
    receipt_keys: Option<Vec<layerx_client::sequencer::ReceiptKeyEntry>>,
}

fn program_authority_boot(
    config: &Config,
    native: &mut Option<NativeReadRoute>,
) -> Result<ProgramAuthorityBoot, String> {
    let trust = load_chain_trust_root(&config.chain_trust_root)?;
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        trust.chain_id,
        trust.validators.validators().to_vec(),
        config.staleness_ms,
    )
    .map_err(|error| format!("agent deployment verifier is invalid: {error}"))?;
    let receipt_keys = native
        .as_mut()
        .map(NativeReadRoute::signed_authority)
        .transpose()
        .map_err(|error| format!("program receipt keys unavailable: {error:?}"))?;
    let journal = program_reader(config, verifier.clone(), Registry::new())?;
    let registry = load_registry(Path::new(&config.deployment_journal), &journal)?;
    Ok(ProgramAuthorityBoot {
        verifier,
        registry,
        receipt_keys,
    })
}

/// Connects one authenticated protocol reader from the configured node and authority,
/// bound to the protected deployment verifier.
fn program_reader(
    config: &Config,
    verifier: ProtocolDeploymentVerifier,
    registry: Registry,
) -> Result<LayerxdProgramBalanceReader, String> {
    let reader = LayerxdProgramBalanceReader::connect(
        &config.node_endpoint,
        config.node_bearer.clone(),
        ProgramAuthority {
            endpoint: &config.authority_endpoint,
            authorization: config.authority_bearer.clone(),
            replica_id: config.authority_replica_id,
            ca_der: &config.authority_ca_der,
        },
        verifier,
        registry,
    )
    .map_err(|error: ProgramReadError| {
        format!("agent program reader configuration failed: {error:?}")
    })?;
    Ok(reader)
}

struct InferenceSupervision {
    market: layerx_agentd::boot::inference_market::InferenceMarket,
    signal: HttpPool,
    completion: HttpPool,
    reader: LayerxdProgramBalanceReader,
    agent: ureq::Agent,
    next_check: Instant,
}

fn inference_node_config(config: &Config) -> NodeRpcConfig {
    NodeRpcConfig {
        url: config.node_endpoint.clone(),
        chain_id: config.expected_chain_id,
        deadline: REQUEST_DEADLINE,
    }
}

fn inference_observation(
    config: &Config,
    reader: &LayerxdProgramBalanceReader,
    agent: &ureq::Agent,
) -> Result<(layerx_agentd::degraded::Reference, serde_json::Value), String> {
    let node = NodeRpc::connect_with_ca(&inference_node_config(config), &config.authority_ca_der)
        .map_err(|error| format!("inference node RPC readiness failed: {error}"))?;
    readiness_gate(&node)
        .map_err(|error| format!("inference node RPC readiness failed: {error:?}"))?;
    let height = node.block_number().map_err(|error| error.to_string())?;
    reader
        .read_chain_head_at(height, now_ms()?)
        .map_err(|error| format!("inference program authority failed: {error:?}"))?;
    let block = node
        .block_by_number(height)
        .map_err(|error| error.to_string())?
        .ok_or("inference node head is unavailable")?;
    let reference = layerx_agentd::degraded::Reference {
        head_sequence: height,
        checkpoint: block.hash,
    };
    let components = layerx_agentd::boot::inference_market::read_availability(
        agent,
        &config.node_endpoint,
        &config.node_bearer,
        config.expected_chain_id,
        &config.probe_program.bytes(),
        reference,
    )?;
    Ok((reference, components))
}

fn inference_boot(
    config: &Config,
    inference: layerx_agentd::config::InferenceConfig,
    reader: LayerxdProgramBalanceReader,
) -> Result<InferenceSupervision, String> {
    use layerx_agentd::boot::inference_market;
    let roots = [ureq::tls::Certificate::from_der(&config.authority_ca_der).to_owned()];
    let agent: ureq::Agent = ureq::Agent::config_builder()
        .timeout_global(Some(REQUEST_DEADLINE))
        .http_status_as_error(false)
        .max_redirects(0)
        .tls_config(
            ureq::tls::TlsConfig::builder()
                .provider(ureq::tls::TlsProvider::Rustls)
                .root_certs(ureq::tls::RootCerts::new_with_certs(&roots))
                .build(),
        )
        .build()
        .into();
    let (reference, components) = inference_observation(config, &reader, &agent)?;
    let mut gate = layerx_agentd::boot::InferenceBootGate::default();
    gate.ready(reference, layerx_types::rung::Rung::Committed, &components)
        .map_err(|error| format!("inference boot refused: {error:?}"))?;
    let market = inference_market::serve_inference_market(inference, gate)?;
    let signal_gate = Arc::clone(&market.gate);
    let signal = HttpPool::start(
        market
            .signal_listener
            .try_clone()
            .map_err(|error| error.to_string())?,
        move |mut stream| {
            let _ = inference_market::serve_health(&mut stream, &signal_gate);
        },
    )?;
    let completion_gate = Arc::clone(&market.gate);
    let completion = HttpPool::start(
        market
            .completion_listener
            .try_clone()
            .map_err(|error| error.to_string())?,
        move |mut stream| {
            let _ = inference_market::serve_health(&mut stream, &completion_gate);
        },
    )?;
    Ok(InferenceSupervision {
        market,
        signal,
        completion,
        reader,
        agent,
        next_check: Instant::now(),
    })
}

fn serve(config: Config) -> Result<(), String> {
    if config.inference.is_some() {
        let node =
            NodeRpc::connect_with_ca(&inference_node_config(&config), &config.authority_ca_der)
                .map_err(|error| format!("inference node RPC readiness failed: {error}"))?;
        readiness_gate(&node)
            .map_err(|error| format!("inference node RPC readiness failed: {error:?}"))?;
    }
    let mcp = mcp_enrolment(&config)?
        .map(|enrolment| mcp_boot(&config, enrolment))
        .transpose()?;
    let runtime_clock = layerx_client::runtime_clock::RuntimeClock::from_environment()
        .map_err(|error| format!("runtime clock unavailable: {error}"))?;

    let mut native = mcp
        .as_ref()
        .map(|_| {
            let client = layerx_client::sequencer::SequencerEndpoint::new(
                &config.node_endpoint,
                &[&config.authority_ca_der],
            )
            .map_err(|error| format!("native node endpoint is invalid: {error:?}"))?;
            Ok::<_, String>(NativeReadRoute::new(client, config.expected_chain_id))
        })
        .transpose()?;
    let ProgramAuthorityBoot {
        verifier,
        registry,
        receipt_keys,
    } = program_authority_boot(&config, &mut native)?;
    let mut inference = config
        .inference
        .clone()
        .map(|inference| {
            inference_boot(
                &config,
                inference,
                program_reader(&config, verifier.clone(), registry.clone())?,
            )
        })
        .transpose()?;
    let route =
        ProgramBalanceReadRoute::new(program_reader(&config, verifier.clone(), registry.clone())?);
    let programs = ProgramOperations::new(program_reader(&config, verifier, registry)?);
    let export_trust = receipt_keys
        .map(|keys| export_trust_source(keys, &config.authority_ca_der))
        .transpose()?;
    let (human, owner, status, shared_store) = start_shared_owner(
        mcp,
        Some(programs),
        export_trust,
        Some(&config.policy_sources),
        &config.native_policy_sources,
        &config.native_effect_policy_sources,
        &config.program_budget_denomination_sources,
        runtime_clock,
    )?;
    let rpc = start_agent_rpc(owner, shared_store)?;
    drop(status);
    let node_config = NodeRpcConfig {
        url: config.node_endpoint.clone(),
        chain_id: config.expected_chain_id,
        deadline: REQUEST_DEADLINE,
    };
    let endpoint = layerx_client::sequencer::SequencerEndpoint::new(
        &node_config.url,
        &[&config.authority_ca_der],
    )
    .map_err(|error| format!("program node endpoint is invalid: {error:?}"))?;
    let program = if inference.is_none() {
        let listener = TcpListener::bind(&config.listen)
            .map_err(|error| format!("agent program listener failed: {error}"))?;
        let pool = serve_read_routes(
            listener,
            config.bearer.clone(),
            route,
            native,
            node_config,
            config.authority_ca_der.clone(),
            endpoint,
        )?;
        Some(pool)
    } else {
        None
    };
    loop {
        match human.try_recv() {
            Ok(result) => return result,
            Err(mpsc::TryRecvError::Disconnected) => {
                return Err("human listener terminated without status".to_owned())
            }
            Err(mpsc::TryRecvError::Empty) => {}
        }
        if let Some(program) = &program {
            program.check()?;
        }
        if let Some(inference) = &mut inference {
            inference.signal.check()?;
            inference.completion.check()?;
            if Instant::now() >= inference.next_check {
                let observation =
                    inference_observation(&config, &inference.reader, &inference.agent);
                if let Err(error) = layerx_agentd::boot::inference_market::observe_availability(
                    &inference.market.gate,
                    observation,
                ) {
                    eprintln!("layerx-agentd: {}", Redacted::boot_diagnostic(&error));
                }
                inference.next_check = Instant::now() + Duration::from_secs(1);
            }
        }
        if let Some(rpc) = &rpc {
            rpc.check()?;
        }
        thread::sleep(SUPERVISION_INTERVAL);
    }
}

fn mcp_evidence_mode() -> Option<Result<(), String>> {
    let arguments: Vec<_> = env::args_os().collect();
    if arguments
        .get(1)
        .is_none_or(|value| value != "--mcp-evidence")
    {
        return None;
    }
    Some((|| {
        if !(arguments.len() == 5 || arguments.len() == 6) {
            return Err("invalid MCP evidence arguments".to_owned());
        }
        let root = PathBuf::from(&arguments[2]);
        if !root.is_absolute()
            || fs::canonicalize(&root).map_err(|_| "MCP evidence store is unavailable")? != root
        {
            return Err("MCP evidence store path is invalid".to_owned());
        }
        let tenant = arguments[3]
            .to_str()
            .ok_or("MCP evidence tenant is invalid")?;
        let tenant =
            TenantId::new(tenant.to_owned()).map_err(|_| "MCP evidence tenant is invalid")?;
        let encoded = arguments[4].to_str().ok_or("MCP evidence key is invalid")?;
        if encoded.len() != 64
            || !encoded
                .bytes()
                .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
        {
            return Err("MCP evidence key is invalid".to_owned());
        }
        let mut key = [0_u8; 32];
        for (index, pair) in encoded.as_bytes().chunks_exact(2).enumerate() {
            key[index] = u8::from_str_radix(
                std::str::from_utf8(pair).map_err(|_| "MCP evidence key is invalid")?,
                16,
            )
            .map_err(|_| "MCP evidence key is invalid")?;
        }
        if key == [0; 32] {
            return Err("MCP evidence key is invalid".to_owned());
        }
        let store =
            Store::open_read_only(&root).map_err(|_| "MCP evidence store is unavailable")?;
        let tool = arguments
            .get(5)
            .map(|value| value.to_str().ok_or("MCP evidence tool is invalid"))
            .transpose()?;
        if tool
            .is_some_and(|tool| tool.is_empty() || tool.len() > 128 || tool.as_bytes().contains(&0))
        {
            return Err("MCP evidence tool is invalid".to_owned());
        }
        let evidence =
            layerx_agentd::session_control::mcp_store_evidence_for_tool(&store, &tenant, key, tool)
                .map_err(|_| "MCP evidence record is unavailable")?;
        println!(
            "{}",
            serde_json::to_string(&evidence).map_err(|_| "MCP evidence encoding failed")?
        );
        Ok(())
    })())
}

fn main() {
    if let Err(error) = layerx_agentd::human::refuse_retired_human_inputs(
        env::vars_os().map(|(name, _)| name.to_string_lossy().into_owned()),
    )
    .and_then(|()| refuse_retired_environment())
    {
        eprintln!("layerx-agentd: {}", Redacted::boot_diagnostic(&error));
        std::process::exit(2);
    }
    if let Some(result) = mcp_evidence_mode() {
        if result.is_err() {
            eprintln!("layerx-agentd: MCP evidence refused");
            std::process::exit(2);
        }
        return;
    }
    if let Err(error) = human_owner_mode::run(
        || config().and_then(serve),
        human_owner::serve_human_owner,
        || config().and_then(serve),
    ) {
        eprintln!("layerx-agentd: {}", Redacted::boot_diagnostic(&error));
        std::process::exit(2);
    }
}
