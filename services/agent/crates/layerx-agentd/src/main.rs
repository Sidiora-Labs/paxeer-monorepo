#![forbid(unsafe_code)]

mod inference_signal {
    include!("inference_market/signal.rs");
    pub mod deals {
        include!("inference_market/deals.rs");
    }
}

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
    inference_availability: Option<InferenceAvailabilityConfig>,
    listen: String,
    bearer: String,
    node_endpoint: String,
    expected_chain_id: u64,
    node_bearer: String,
    authority_endpoint: String,
    authority_bearer: String,
    authority_ca_der: Vec<u8>,
    // Identity required by the shipped independent program-authority connection.
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
    match std::env::var("LAYERX_AGENT_STATE_EXPORT_SOURCE") {
        Ok(path) => operations.install_state_export_source(
            layerx_agentd::config::StateExportSource::load(std::path::Path::new(&path))?,
        ),
        Err(std::env::VarError::NotPresent) => {}
        Err(std::env::VarError::NotUnicode(_)) => {
            return Err("state export source path is not UTF-8".to_owned())
        }
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

/// Credentials and trust for the hosted AI projection, independent of node RPC.
/// Deliberately does not implement Debug: neither keys nor credential-bearing inputs
/// belong in startup diagnostics.
struct InferenceAvailabilityConfig {
    endpoint: String,
    authorization: ureq::http::HeaderValue,
    roots: Vec<ureq::tls::Certificate<'static>>,
}

impl InferenceAvailabilityConfig {
    fn load(
        inference: bool,
        node_credential: &str,
        authority_credential: &str,
        setting: impl Fn(&str) -> Option<String>,
    ) -> Result<Option<Self>, String> {
        if !inference {
            return Ok(None);
        }
        let required = |name: &str| {
            setting(name)
                .filter(|value| !value.trim().is_empty() && value.trim() == value)
                .ok_or_else(|| format!("{name} is required and must be nonblank"))
        };
        const ENDPOINT: &str = "LAYERX_AGENT_INFERENCE_GATEWAY_ENDPOINT";
        const KEY: &str = "LAYERX_AGENT_INFERENCE_GATEWAY_API_KEY";
        const CA: &str = "LAYERX_AGENT_INFERENCE_GATEWAY_CA_FILE";
        let endpoint = required(ENDPOINT)?;
        let uri = endpoint
            .parse::<ureq::http::Uri>()
            .map_err(|_| format!("{ENDPOINT} must be a canonical HTTPS endpoint"))?;
        if uri.scheme_str() != Some("https")
            || uri.host().is_none_or(str::is_empty)
            || uri.authority().is_none_or(|a| a.as_str().contains('@'))
            || uri.query().is_some()
            || endpoint.contains(['#', '\\'])
            || uri.port_u16() == Some(0)
            || (uri.port().is_some() && uri.port_u16().is_none())
        {
            return Err(format!("{ENDPOINT} must be a canonical HTTPS endpoint"));
        }
        let key = zeroize::Zeroizing::new(required(KEY)?);
        let credential = key.strip_prefix("LayerX-Key ").unwrap_or(&key);
        if credential.is_empty()
            || credential.len() > 4096
            || credential
                .bytes()
                .any(|b| b.is_ascii_whitespace() || b.is_ascii_control())
            || [node_credential, authority_credential].iter().any(|node| {
                credential == *node || credential.strip_prefix("Bearer ") == Some(*node)
            })
        {
            return Err(format!("{KEY} must be a distinct gateway credential"));
        }
        let mut authorization =
            ureq::http::HeaderValue::from_str(&format!("LayerX-Key {credential}"))
                .map_err(|_| format!("{KEY} must be a valid gateway credential"))?;
        authorization.set_sensitive(true);
        let path = required(CA)?;
        let file = fs::File::open(path).map_err(|_| format!("{CA} is unreadable"))?;
        if !file
            .metadata()
            .map_err(|_| format!("{CA} is unreadable"))?
            .is_file()
        {
            return Err(format!("{CA} must contain certificate roots"));
        }
        let mut bytes = Vec::new();
        file.take(1_048_577)
            .read_to_end(&mut bytes)
            .map_err(|_| format!("{CA} is unreadable"))?;
        if bytes.is_empty() || bytes.len() > 1_048_576 {
            return Err(format!("{CA} must contain bounded certificate roots"));
        }
        let roots = if bytes.starts_with(b"-----BEGIN") {
            ureq::tls::parse_pem(&bytes)
                .map(|item| match item {
                    Ok(ureq::tls::PemItem::Certificate(cert)) => Ok(cert),
                    _ => Err(format!("{CA} must contain certificate roots")),
                })
                .collect::<Result<Vec<_>, _>>()?
        } else {
            vec![ureq::tls::Certificate::from_der(&bytes).to_owned()]
        };
        let mut validation = rustls::RootCertStore::empty();
        for root in &roots {
            validation
                .add(rustls::pki_types::CertificateDer::from(root.der().to_vec()))
                .map_err(|_| format!("{CA} must contain valid certificate roots"))?;
        }
        if validation.is_empty() {
            return Err(format!("{CA} must contain certificate roots"));
        }
        Ok(Some(Self {
            endpoint: endpoint.trim_end_matches('/').to_owned(),
            authorization,
            roots,
        }))
    }

    fn client(&self) -> Result<ureq::Agent, String> {
        let endpoint = self
            .endpoint
            .parse::<ureq::http::Uri>()
            .map_err(|_| "inference gateway endpoint is invalid")?;
        let authorization = self.authorization.clone();
        Ok(ureq::Agent::config_builder()
            .timeout_global(Some(REQUEST_DEADLINE))
            .http_status_as_error(false)
            .max_redirects(0)
            .tls_config(
                ureq::tls::TlsConfig::builder()
                    .provider(ureq::tls::TlsProvider::Rustls)
                    .root_certs(ureq::tls::RootCerts::new_with_certs(&self.roots))
                    .build(),
            )
            // read_availability retains the common bounded decoder and all binding
            // checks. Its legacy Bearer header is replaced only on this dedicated
            // gateway client and only for its configured HTTPS origin.
            .middleware(
                move |mut request: ureq::http::Request<ureq::SendBody>,
                      next: ureq::middleware::MiddlewareNext| {
                    if request.uri().scheme() != endpoint.scheme()
                        || request.uri().authority() != endpoint.authority()
                        || !request.uri().path().starts_with(&format!(
                            "{}/v1/ai/markets/",
                            endpoint.path().trim_end_matches('/')
                        ))
                    {
                        return Err(ureq::Error::Tls("inference gateway request origin refused"));
                    }
                    let prefix =
                        format!("{}/v1/ai/markets/", endpoint.path().trim_end_matches('/'));
                    let market = request
                        .uri()
                        .path()
                        .strip_prefix(&format!("{prefix}0x"))
                        .and_then(|path| path.strip_suffix("/snapshot"))
                        .filter(|market| {
                            market.len() == 64
                                && market.bytes().all(|byte| {
                                    byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte)
                                })
                        })
                        .ok_or(ureq::Error::Tls("inference gateway snapshot route refused"))?;
                    // The shared reader uses a prefixed digest; the gateway route
                    // accepts the producer's canonical unprefixed market identifier.
                    let path = format!("{prefix}{market}/snapshot");
                    let mut parts = request.uri().clone().into_parts();
                    parts.path_and_query = Some(path.parse().map_err(|_| {
                        ureq::Error::Tls("inference gateway snapshot route refused")
                    })?);
                    *request.uri_mut() = ureq::http::Uri::from_parts(parts).map_err(|_| {
                        ureq::Error::Tls("inference gateway snapshot route refused")
                    })?;
                    request
                        .headers_mut()
                        .insert(ureq::http::header::AUTHORIZATION, authorization.clone());
                    next.handle(request)
                },
            )
            .build()
            .into())
    }
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
    let inference_availability = InferenceAvailabilityConfig::load(
        inference.is_some(),
        &node_bearer,
        &authority_bearer,
        |name| env::var(name).ok(),
    )?;
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
        inference_availability,
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
    let source = layerx_agentd::read::NativeResultSource::connect(
        &config.node_endpoint,
        Some(&config.authority_ca_der),
    )
    .map_err(|_| "agent native node source configuration failed")?;
    Ok(reader.with_consensus_source(source))
}

struct InferenceSupervision {
    market: layerx_agentd::boot::inference_market::InferenceMarket,
    signal: inference_signal::deals::DealSignalListener,
    deals: InferenceDealRuntime,
    completion: HttpPool,
    reader: LayerxdProgramBalanceReader,
    agent: ureq::Agent,
    next_check: Instant,
}

#[derive(serde::Deserialize)]
#[serde(deny_unknown_fields)]
struct InferenceDealTerms {
    signer: layerx_human_kms::attestor::AttestorSignerConfig,
    assertion_source: PathBuf,
    journal: PathBuf,
    actor_did: String,
    layer_count: u16,
    layer_memory_mib: u32,
    validity_blocks: u64,
    fee_limit: u128,
    gas_limit: u64,
    max_fee_per_gas: u128,
    max_priority_fee_per_gas: u128,
    resources: [u64; 7],
}

struct InferenceDealRuntime {
    engine: Arc<Mutex<inference_signal::deals::DealEngine>>,
    endpoint: SequencerEndpoint,
    verifier: layerx_programs::chain_proof::ChainProofVerifier,
    journal: layerx_client::ai_market::OperationJournal,
    signer: layerx_human_kms::attestor::AttestorSigner,
    terms: InferenceDealTerms,
    program: layerx_client::ai_market::market_program::types::ProgramId,
}

#[derive(Debug)]
struct InferenceDealSigner<'a>(&'a layerx_human_kms::attestor::AttestorSigner);

impl layerx_crypto::signer::Signer for InferenceDealSigner<'_> {
    fn public_key(&self) -> [u8; 32] {
        self.0.public_key()
    }

    fn sign<'a>(
        &'a self,
        _request: layerx_crypto::signer::SigningRequest<'a>,
    ) -> layerx_crypto::signer::SignFuture<'a> {
        Box::pin(std::future::ready(Err(
            layerx_crypto::signer::SignError::RemoteRefused,
        )))
    }

    fn evm_address(&self) -> Option<[u8; 20]> {
        Some(self.0.evm_address())
    }

    fn sign_transaction<'a>(
        &'a self,
        call: &'a layerx_crypto::evm_transaction::Eip1559Call,
    ) -> layerx_crypto::signer::TransactionSignFuture<'a> {
        Box::pin(async move {
            use layerx_crypto::signer::{AgentSignature, SignError};
            let mut signature = self
                .0
                .sign_evm_transaction(
                    &layerx_human_kms::attestor::unsigned_evm_transaction(call),
                    call.chain_id,
                )
                .map_err(|_| SignError::RemoteRefused)?;
            signature[64] = signature[64]
                .checked_add(27)
                .ok_or(SignError::ReturnedSignatureInvalid)?;
            AgentSignature::new(signature).map_err(|_| SignError::ReturnedSignatureInvalid)
        })
    }
}

fn inference_wait<F: std::future::Future>(future: F) -> F::Output {
    struct WakeThread(thread::Thread);
    impl std::task::Wake for WakeThread {
        fn wake(self: Arc<Self>) {
            self.0.unpark();
        }
        fn wake_by_ref(self: &Arc<Self>) {
            self.0.unpark();
        }
    }
    let waker = std::task::Waker::from(Arc::new(WakeThread(thread::current())));
    let mut context = std::task::Context::from_waker(&waker);
    let mut future = std::pin::pin!(future);
    loop {
        match future.as_mut().poll(&mut context) {
            std::task::Poll::Ready(value) => return value,
            std::task::Poll::Pending => thread::park(),
        }
    }
}

impl InferenceDealRuntime {
    fn open(config: &Config, offer: &inference_signal::RoomOffer) -> Result<Self, String> {
        use inference_signal::deals::{DealConfig, DealEngine};
        let source = required("LAYERX_AGENT_INFERENCE_DEALS")?;
        let bytes = zeroize::Zeroizing::new(
            layerx_agentd::config::read_protected_source(Path::new(&source), 65_536)
                .map_err(|_| "inference deal configuration is invalid")?,
        );
        let terms: InferenceDealTerms = serde_json::from_slice(&bytes)
            .map_err(|_| "inference deal configuration is malformed")?;
        if terms.layer_count == 0
            || terms.layer_memory_mib == 0
            || terms.validity_blocks == 0
            || terms.validity_blocks > 256
            || terms.gas_limit == 0
            || terms.fee_limit == 0
            || terms.max_fee_per_gas == 0
            || terms.max_priority_fee_per_gas > terms.max_fee_per_gas
            || terms.resources.contains(&0)
            || !terms.journal.is_absolute()
        {
            return Err("inference deal model or transaction terms are invalid".to_owned());
        }
        layerx_types::ids::Did::new(terms.actor_did.as_bytes())
            .map_err(|_| "inference deal signer DID is invalid")?;
        let mut signer = layerx_human_kms::attestor::AttestorSigner::from_config(&terms.signer)
            .map_err(|_| "inference deal signer is unavailable")?;
        let assertion = zeroize::Zeroizing::new(
            layerx_agentd::config::read_protected_source(&terms.assertion_source, 8192)
                .map_err(|_| "inference deal signer assertion is unavailable")?,
        );
        let assertion = std::str::from_utf8(&assertion)
            .map_err(|_| "inference deal assertion is malformed")?
            .trim();
        signer
            .set_evm_assertion(assertion)
            .map_err(|_| "inference deal signer assertion is invalid")?;
        let journal = layerx_client::ai_market::OperationJournal::open(&terms.journal)
            .map_err(|error| format!("inference deal journal: {error:?}"))?;
        let engine = DealEngine::open(
            &terms.journal.join("negotiation.json"),
            DealConfig {
                room_id: offer.room_id,
                model_digest: offer.model_digest,
                runtime_digest: offer.runtime_digest,
                program: config.probe_program.bytes(),
                layer_count: terms.layer_count,
                layer_memory_mib: terms.layer_memory_mib,
            },
        )
        .map_err(|error| error.to_string())?;
        let endpoint =
            SequencerEndpoint::new_node(&config.node_endpoint, &[&config.authority_ca_der])
                .map_err(|error| format!("inference deal node: {error:?}"))?;
        if endpoint
            .chain_id()
            .map_err(|error| format!("inference deal chain: {error:?}"))?
            != config.expected_chain_id
            || config.expected_chain_id != 125
        {
            return Err("inference deal node chain differs".to_owned());
        }
        let trust = load_chain_trust_root(&config.chain_trust_root)?;
        let verifier = layerx_programs::chain_proof::ChainProofVerifier {
            chain_id: trust.chain_id,
            validators: trust.validators,
        };
        let program = layerx_client::ai_market::market_program::types::ProgramId::new(
            config.probe_program.bytes(),
        )
        .map_err(|error| error.to_string())?;
        Ok(Self {
            engine: Arc::new(Mutex::new(engine)),
            endpoint,
            verifier,
            journal,
            signer,
            terms,
            program,
        })
    }

    fn snapshot(&self) -> Result<layerx_client::ai_market::FinalizedSnapshot, String> {
        let height = self
            .endpoint
            .block_number()
            .map_err(|error| format!("inference book head: {error:?}"))?;
        layerx_client::ai_market::inference_market::poll_finalized_market(
            &self.endpoint,
            &self.verifier,
            self.program,
            height,
            now_ms()?,
        )
        .map_err(|error| format!("inference committed book: {error:?}"))
    }

    fn submit_ready(&self) -> Result<(), String> {
        let engine = self
            .engine
            .lock()
            .map_err(|_| "inference deal engine unavailable")?;
        let Some(frame) = engine.ready_to_record().cloned() else {
            return inference_signal::deals::resume_submission(
                &engine,
                &self.journal,
                &self.endpoint,
            )
            .map_err(|error| error.to_string());
        };
        let height = self
            .endpoint
            .block_number()
            .map_err(|error| format!("inference deal head: {error:?}"))?;
        let expiry = height
            .checked_add(self.terms.validity_blocks)
            .ok_or("inference deal expiry overflow")?;
        let envelope = engine.envelope(expiry).map_err(|error| error.to_string())?;
        drop(engine);
        let from = self.signer.evm_address();
        let mut actor = [0; 32];
        actor[12..].copy_from_slice(&from);
        if actor != frame.plan.embedding_host {
            return Err("inference signer does not own the room".to_owned());
        }
        let nonce = self
            .endpoint
            .pending_nonce(&from)
            .map_err(|error| format!("inference deal nonce: {error:?}"))?;
        let mut record = inference_wait(layerx_client::ai_market::sign_market_envelope(
            &envelope,
            &layerx_client::ai_market::NativeTerms {
                network_id: 125,
                actor_did: self.terms.actor_did.as_bytes(),
                owner_public_key: self.signer.public_key(),
                account_sequence: nonce,
                idempotency_key: frame.deal_id,
                not_before: 0,
                not_after: expiry,
                fee_limit: self.terms.fee_limit,
                capabilities: &[],
                access_declaration: &[],
                response_capacity: 16_384,
                resources: layerx_types::program_call::Resources(self.terms.resources),
                gas_limit: self.terms.gas_limit,
                max_fee_per_gas: self.terms.max_fee_per_gas,
                max_priority_fee_per_gas: self.terms.max_priority_fee_per_gas,
            },
            &InferenceDealSigner(&self.signer),
        ))
        .map_err(|error| format!("inference deal signature: {error:?}"))?;
        let mut engine = self
            .engine
            .lock()
            .map_err(|_| "inference deal engine unavailable")?;
        inference_signal::deals::record_deal(
            &mut engine,
            &self.journal,
            &self.endpoint,
            &mut record,
        )
        .map_err(|error| error.to_string())?;
        Ok(())
    }
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
    let availability = config
        .inference_availability
        .as_ref()
        .ok_or("inference gateway configuration is required")?;
    let components = layerx_agentd::boot::inference_market::read_availability(
        agent,
        &availability.endpoint,
        "", // Authentication is supplied by the gateway-only client middleware.
        config.expected_chain_id,
        &config.probe_program.bytes(),
        reference,
    )?;
    Ok((reference, components))
}

fn inference_enrol_host(
    config: &Config,
    inference: &layerx_agentd::config::InferenceConfig,
) -> Result<(), String> {
    use layerx_agentd::finality::inference_enrol::{
        read_enrolment_file, EnrolmentRelay, MeasurementConfig, PriceQuote, SignedManifest,
    };
    use layerx_client::ai_market::{market_envelope, OperationJournal, OperationRecord};
    PriceQuote::require_program_interface().map_err(|error| error.to_string())?;
    let path = PathBuf::from(required("LAYERX_AGENT_INFERENCE_ENROLMENT")?);
    let document = read_enrolment_file(&path).map_err(|error| error.to_string())?;
    let bytes = |name: &str| -> Result<Vec<u8>, String> {
        let text = document[name]
            .as_str()
            .ok_or_else(|| format!("inference enrolment {name} is required"))?;
        layerx_programs::hex::decode(text.strip_prefix("0x").unwrap_or(text))
            .map_err(|_| format!("inference enrolment {name} is malformed"))
    };
    let record = |name: &str| -> Result<OperationRecord, String> {
        let raw = bytes(name)?;
        let expiry = document["expiry"]
            .as_u64()
            .ok_or("inference enrolment expiry is required")?;
        let record = OperationRecord::from_signed_transaction(raw, [0; 32], expiry)
            .map_err(|error| format!("inference enrolment {name}: {error:?}"))?;
        let envelope = market_envelope(&record)
            .map_err(|error| format!("inference enrolment {name}: {error:?}"))?;
        let env = layerx_client::ai_market::market_program::codec::decode_envelope(&envelope)
            .map_err(|error| error.to_string())?;
        if env.envelope.program.bytes() != config.probe_program.bytes()
            || env.envelope.chain.bytes()
                != layerx_client::ai_market::market_program::merged_chain_domain(
                    config.expected_chain_id,
                )
        {
            return Err(
                "inference enrolment differs from the configured chain or program".to_owned(),
            );
        }
        let intent = env.request_digest().map_err(|error| error.to_string())?;
        OperationRecord::from_signed_transaction(
            record.signed_bytes().to_vec(),
            intent.bytes(),
            expiry,
        )
        .map_err(|error| format!("inference enrolment {name}: {error:?}"))
    };
    let legacy = bytes("manifest_v1")?;
    if legacy.get(138..170) != Some(inference.runtime_digest.as_slice())
        || legacy.get(174..206) != Some(inference.model_digest.as_slice())
    {
        return Err(
            "inference enrolment manifest differs from configured model or runtime".to_owned(),
        );
    }
    let mut manifest = SignedManifest {
        manifest: bytes("manifest_v2")?,
        proposal: record("enroll")?,
        acceptance: record("accept")?,
        publication: record("publish")?,
    };
    let mut quote = PriceQuote {
        lock_bond: record("lock_bond")?,
        post_bid: record("post_bid")?,
    };
    let region: [u8; 2] = required("LAYERX_AGENT_INFERENCE_REGION")?
        .as_bytes()
        .try_into()
        .map_err(|_| "inference region requires two ASCII letters")?;
    let attestation_family = required("LAYERX_AGENT_INFERENCE_ATTESTATION_FAMILY")?
        .parse::<u8>()
        .map_err(|_| "inference attestation family must be a byte")?;
    let probe = required("LAYERX_AGENT_INFERENCE_BANDWIDTH_PROBE")?;
    let journal = OperationJournal::open(Path::new(&required(
        "LAYERX_AGENT_INFERENCE_ENROLMENT_JOURNAL",
    )?))
    .map_err(|error| format!("inference enrolment journal: {error:?}"))?;
    let endpoint = layerx_client::sequencer::SequencerEndpoint::new(
        &config.node_endpoint,
        &[&config.authority_ca_der],
    )
    .map_err(|error| format!("inference enrolment endpoint: {error:?}"))?;
    let keys = endpoint
        .receipt_keys()
        .map_err(|error| format!("inference receipt keys: {error:?}"))?;
    let relay = EnrolmentRelay {
        endpoint: &endpoint,
        journal: &journal,
        receipt_keys: &keys,
        timeout: REQUEST_DEADLINE,
    };
    relay
        .enrol_host(
            &MeasurementConfig {
                manifest_v1: &legacy,
                region_code: region,
                attestation_family,
                bandwidth_probe: &probe,
            },
            &mut manifest,
            &mut quote,
        )
        .map_err(|error| error.to_string())?;
    Ok(())
}

fn inference_boot(
    config: &Config,
    inference: layerx_agentd::config::InferenceConfig,
    reader: LayerxdProgramBalanceReader,
) -> Result<InferenceSupervision, String> {
    use layerx_agentd::boot::inference_market;
    let agent = config
        .inference_availability
        .as_ref()
        .ok_or("inference gateway configuration is required")?
        .client()?;
    let (reference, components) = inference_observation(config, &reader, &agent)?;
    let mut gate = layerx_agentd::boot::InferenceBootGate::default();
    gate.ready(reference, layerx_types::rung::Rung::Committed, &components)
        .map_err(|error| format!("inference boot refused: {error:?}"))?;
    inference_enrol_host(config, &inference)?;
    let market = inference_market::serve_inference_market(inference, gate)?;
    let offer: inference_signal::RoomOffer = serde_json::from_slice(&market.room_profile)
        .map_err(|error| format!("inference room offer is invalid: {error}"))?;
    if offer.model_digest != market.config.model_digest
        || offer.runtime_digest != market.config.runtime_digest
    {
        return Err("inference room offer disagrees with configured digests".to_owned());
    }
    let signal_authority = inference_signal::SignalAuthority::new(
        agent.clone(),
        config.node_endpoint.clone(),
        config.node_bearer.clone(),
        config.expected_chain_id,
        config.probe_program.bytes(),
        Arc::clone(&market.gate),
    )?;
    let deals = InferenceDealRuntime::open(config, &offer)?;
    let snapshot = deals.snapshot()?;
    deals
        .engine
        .lock()
        .map_err(|_| "inference deal engine unavailable")?
        .poll(snapshot, now_ms()?)
        .map_err(|error| error.to_string())?;
    let signal = inference_signal::deals::DealSignalListener::start(
        market
            .signal_listener
            .try_clone()
            .map_err(|error| error.to_string())?,
        offer,
        signal_authority,
        Arc::clone(&deals.engine),
        now_ms,
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
        deals,
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
                let observed = layerx_agentd::boot::inference_market::observe_availability(
                    &inference.market.gate,
                    observation,
                );
                let result = observed.and_then(|()| {
                    let snapshot = inference.deals.snapshot()?;
                    inference
                        .signal
                        .tick(snapshot, now_ms()?)
                        .map_err(|error| error.to_string())?;
                    inference.deals.submit_ready()
                });
                if let Err(error) = result {
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

#[cfg(test)]
#[path = "../../../../../programs/crates/layerx-programs-registry/tests/support/chain.rs"]
mod inference_availability_chain;

#[cfg(test)]
mod inference_availability_tests {
    use super::inference_availability_chain as chain;
    use super::*;
    use layerx_platform_gateway::{
        ai_views, gateway_digest, http,
        store::{KeyRecord, RedisEndpoint, RedisStore},
    };
    use serde_json::{json, Value};
    use std::net::{SocketAddr, TcpStream};
    use std::process::{Child, Command, Stdio};
    use std::sync::atomic::{AtomicBool, Ordering};
    use zeroize::Zeroizing;
    const NODE_KEY: &str = "node-only-credential-01234567890123456789";
    const AUTHORITY_KEY: &str = "authority-only-credential-0123456789012345";
    const ENDPOINT: &str = "LAYERX_AGENT_INFERENCE_GATEWAY_ENDPOINT";
    const KEY: &str = "LAYERX_AGENT_INFERENCE_GATEWAY_API_KEY";
    const CA: &str = "LAYERX_AGENT_INFERENCE_GATEWAY_CA_FILE";

    struct CertificateFixture {
        certificate: rcgen::CertifiedKey<rcgen::KeyPair>,
        tls: Arc<rustls::ServerConfig>,
    }
    impl CertificateFixture {
        fn new() -> Self {
            let certificate = rcgen::generate_simple_self_signed(vec!["localhost".to_owned()])
                .expect("certificate");
            let tls = Arc::new(
                rustls::ServerConfig::builder_with_provider(Arc::new(
                    rustls::crypto::ring::default_provider(),
                ))
                .with_safe_default_protocol_versions()
                .expect("TLS versions")
                .with_no_client_auth()
                .with_single_cert(
                    vec![certificate.cert.der().clone()],
                    rustls::pki_types::PrivatePkcs8KeyDer::from(
                        certificate.signing_key.serialize_der(),
                    )
                    .into(),
                )
                .expect("TLS certificate"),
            );
            Self { certificate, tls }
        }
        fn write(&self, root: &Path, prefix: &str) -> PathBuf {
            fs::write(
                root.join(format!("{prefix}.pem")),
                self.certificate.cert.pem(),
            )
            .expect("certificate PEM");
            fs::write(
                root.join(format!("{prefix}.key")),
                self.certificate.signing_key.serialize_pem(),
            )
            .expect("key PEM");
            let path = root.join(format!("{prefix}.der"));
            fs::write(&path, self.certificate.cert.der()).expect("certificate DER");
            path
        }
    }

    struct RedisProcess(Child);
    impl Drop for RedisProcess {
        fn drop(&mut self) {
            let _ = self.0.kill();
            let _ = self.0.wait();
        }
    }
    fn durable_keys(
        root: &Path,
        certificate: &CertificateFixture,
    ) -> (RedisProcess, Arc<RedisStore>) {
        let socket = TcpListener::bind("127.0.0.1:0").expect("Redis port reservation");
        let port = socket.local_addr().expect("Redis port").port();
        certificate.write(root, "redis");
        let acl = root.join("redis.acl");
        fs::write(
            &acl,
            "user default off\nuser gateway on >integration-secret ~* &* +@all\n",
        )
        .expect("Redis ACL");
        let config = root.join("redis.conf");
        fs::write(&config, format!("bind 127.0.0.1\nport 0\ntls-port {port}\ntls-cert-file {}\ntls-key-file {}\ntls-ca-cert-file {}\ntls-auth-clients no\naclfile {}\nappendonly yes\nappendfsync always\ndir {}\n", root.join("redis.pem").display(), root.join("redis.key").display(), root.join("redis.pem").display(), acl.display(), root.display())).expect("Redis configuration");
        drop(socket);
        let process = RedisProcess(
            Command::new("redis-server")
                .arg(config)
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .spawn()
                .expect("real durable Redis"),
        );
        let store = Arc::new(RedisStore::new(
            RedisEndpoint::parse(&format!("rediss://localhost:{port}")).expect("Redis endpoint"),
            native_tls::Certificate::from_der(certificate.certificate.cert.der())
                .expect("Redis root"),
            Zeroizing::new("gateway".into()),
            Zeroizing::new("integration-secret".into()),
        ));
        let until = Instant::now() + Duration::from_secs(10);
        while store.key("readiness").is_err() {
            assert!(Instant::now() < until, "real Redis did not become ready");
            thread::sleep(Duration::from_millis(20));
        }
        (process, store)
    }
    fn issue(store: &RedisStore, id: &str) -> (KeyRecord, String) {
        let secret = format!("lxp_live_{}", "42".repeat(32));
        let salt = "12".repeat(32);
        let record = KeyRecord {
            key_id: id.into(),
            principal_digest: "34".repeat(32),
            secret_digest: gateway_digest(&[b"gateway-key-v1", salt.as_bytes(), secret.as_bytes()]),
            salt,
            signer_public_key: "56".repeat(32),
            scopes: "read".into(),
            quota_requests: 1000,
            quota_window_seconds: 60,
            epoch: 1,
            disabled: false,
        };
        store
            .issue_key(&record, &format!("issue-{id}"))
            .expect("durable gateway key issuance");
        (record, format!("{id}:{secret}"))
    }

    struct HttpsPeer {
        address: SocketAddr,
        stopping: Arc<AtomicBool>,
        worker: Option<thread::JoinHandle<()>>,
    }
    impl HttpsPeer {
        fn start(
            tls: Arc<rustls::ServerConfig>,
            handler: impl Fn(http::IncomingRequest) -> Option<http::OutgoingResponse> + Send + 'static,
        ) -> Self {
            let listener = TcpListener::bind("127.0.0.1:0").expect("HTTPS listener");
            let address = listener.local_addr().expect("HTTPS address");
            listener
                .set_nonblocking(true)
                .expect("nonblocking HTTPS listener");
            let stopping = Arc::new(AtomicBool::new(false));
            let stop = stopping.clone();
            let worker = thread::spawn(move || {
                while !stop.load(Ordering::Acquire) {
                    let (tcp, _) = match listener.accept() {
                        Ok(stream) => stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            thread::sleep(Duration::from_millis(2));
                            continue;
                        }
                        Err(error) => panic!("HTTPS accept: {error}"),
                    };
                    tcp.set_read_timeout(Some(Duration::from_secs(3)))
                        .expect("HTTPS read bound");
                    tcp.set_write_timeout(Some(Duration::from_secs(3)))
                        .expect("HTTPS write bound");
                    let connection =
                        rustls::ServerConnection::new(tls.clone()).expect("TLS server connection");
                    let mut stream = rustls::StreamOwned::new(connection, tcp);
                    if let Ok(request) = http::read_request(&mut stream, 65_536) {
                        if let Some(response) = handler(request) {
                            let _ = http::write_response(&mut stream, &response);
                        }
                    }
                }
            });
            Self {
                address,
                stopping,
                worker: Some(worker),
            }
        }
        fn url(&self) -> String {
            format!("https://localhost:{}", self.address.port())
        }
    }
    impl Drop for HttpsPeer {
        fn drop(&mut self) {
            self.stopping.store(true, Ordering::Release);
            if let Some(worker) = self.worker.take() {
                if !thread::panicking() {
                    worker.join().expect("HTTPS peer stopped");
                }
            }
        }
    }
    fn response(value: Value) -> http::OutgoingResponse {
        http::OutgoingResponse {
            status: 200,
            content_type: "application/json".into(),
            headers: Vec::new(),
            body: serde_json::to_vec(&value).expect("wire response"),
            retry_after: None,
        }
    }
    fn availability(
        values: &BTreeMap<String, String>,
    ) -> Result<Option<InferenceAvailabilityConfig>, String> {
        InferenceAvailabilityConfig::load(true, NODE_KEY, AUTHORITY_KEY, |key| {
            values.get(key).cloned()
        })
    }
    fn fixture_config(
        values: &BTreeMap<String, String>,
        node: &HttpsPeer,
        node_ca: Vec<u8>,
        profile: PathBuf,
    ) -> Config {
        Config {
            inference: Some(layerx_agentd::config::InferenceConfig {
                signal_listen: "127.0.0.1:0".parse().expect("signal address"),
                serve_listen: "127.0.0.1:0".parse().expect("completion address"),
                model_digest: [1; 32],
                runtime_digest: [2; 32],
                room_profile: profile,
            }),
            inference_availability: availability(values).expect("gateway configuration"),
            listen: "127.0.0.1:0".into(),
            bearer: "program-only-credential-0123456789012345".into(),
            node_endpoint: node.url(),
            expected_chain_id: 125,
            node_bearer: NODE_KEY.into(),
            authority_endpoint: format!("{}/authority", node.url()),
            authority_bearer: AUTHORITY_KEY.into(),
            authority_ca_der: node_ca,
            authority_replica_id: [1; 32],
            chain_trust_root: PathBuf::new(),
            staleness_ms: 60_000,
            deployment_journal: String::new(),
            probe_program: ProgramId::new([11; 32]).expect("program"),
            policy_sources: BTreeMap::new(),
            native_policy_sources: BTreeMap::new(),
            native_effect_policy_sources: BTreeMap::new(),
            program_budget_denomination_sources: BTreeMap::new(),
        }
    }
    fn assert_health(supervision: &InferenceSupervision, healthy: bool) {
        for listener in [
            &supervision.market.signal_listener,
            &supervision.market.completion_listener,
        ] {
            let mut stream = TcpStream::connect(listener.local_addr().expect("health address"))
                .expect("health connection");
            stream
                .set_read_timeout(Some(Duration::from_secs(3)))
                .expect("health deadline");
            stream
                .write_all(b"GET /healthz HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
                .expect("health request");
            let mut wire = String::new();
            stream.read_to_string(&mut wire).expect("health response");
            assert!(wire.starts_with(if healthy {
                "HTTP/1.1 200 "
            } else {
                "HTTP/1.1 503 "
            }));
            let body: Value =
                serde_json::from_str(wire.split_once("\r\n\r\n").expect("health body").1)
                    .expect("health JSON");
            assert_eq!(body["ready"], healthy);
        }
    }

    #[test]
    fn inference_availability_uses_gateway_credentials() {
        use layerx_agentd::boot::inference_market::{observe_availability, read_availability};
        let directory = tempfile::tempdir().expect("fixture directory");
        let node_certificate = CertificateFixture::new();
        let gateway_certificate = CertificateFixture::new();
        let gateway_ca = gateway_certificate.write(directory.path(), "gateway");
        let (_redis, keys) = durable_keys(directory.path(), &gateway_certificate);
        let (record, credential) = issue(&keys, "inference-first");
        let database = directory.path().join("ai.sqlite");
        let (market, committed) = ai_producer::ingest(&database, 1000, 4, true);
        let unavailable_database = directory.path().join("ai-unavailable.sqlite");
        ai_producer::ingest(&unavailable_database, 1000, 4, false);
        let room_enabled = Arc::new(AtomicBool::new(true));
        let node_room_enabled = room_enabled.clone();
        let (_, unavailable_status, _) = ai_producer::consensus(1000, false);
        let unavailable_block_hash = layerx_programs::hex::decode_digest(
            unavailable_status["sync_info"]["latest_block_hash"]
                .as_str()
                .expect("unavailable producer block hash"),
        )
        .expect("unavailable block hash");
        let (_, status, _) = ai_producer::consensus(1000, true);
        let block_hash = layerx_programs::hex::decode_digest(
            status["sync_info"]["latest_block_hash"]
                .as_str()
                .expect("producer block hash"),
        )
        .expect("block hash");
        let paths = Arc::new(Mutex::new(Vec::new()));
        let recorded = paths.clone();
        let node = HttpsPeer::start(node_certificate.tls.clone(), move |request| {
            assert!(
                !request.path.contains("/snapshot"),
                "snapshots belong to the gateway"
            );
            assert!(!request
                .headers
                .values()
                .any(|v| v.contains("lxp_live_") || v.starts_with("LayerX-Key ")));
            recorded
                .lock()
                .expect("node requests")
                .push(request.path.clone());
            let call: Value = serde_json::from_slice(&request.body).expect("node RPC request");
            Some(response(
                json!({"jsonrpc":"2.0","id":call["id"],"result":ai_producer::node_result(call["method"].as_str().expect("RPC method"),
                if node_room_enabled.load(Ordering::Acquire) { block_hash } else { unavailable_block_hash },
                node_room_enabled.load(Ordering::Acquire))}),
            ))
        });
        let fault = Arc::new(Mutex::new(None::<String>));
        let gateway_requests = Arc::new(Mutex::new(0_usize));
        let start_gateway = || {
            let views = ai_views::AiViews::open(&database, 1, [0x5a; 32])
                .expect("real durable projection reader");
            let unavailable_views = ai_views::AiViews::open(&unavailable_database, 1, [0x5a; 32])
                .expect("real unavailable projection reader");
            let room_enabled = room_enabled.clone();
            let snapshot_path = format!("/v1/ai/markets/{market}/snapshot");
            let keys = keys.clone();
            let fault = fault.clone();
            let count = gateway_requests.clone();
            HttpsPeer::start(gateway_certificate.tls.clone(), move |request| {
                assert_eq!(request.path, snapshot_path);
                assert!(
                    !request
                        .headers
                        .values()
                        .any(|v| v.contains(NODE_KEY) || v.contains(AUTHORITY_KEY)),
                    "node credentials must stay off the gateway"
                );
                *count.lock().expect("gateway request count") += 1;
                let fault = fault.lock().expect("gateway fault").clone();
                if fault.as_deref() == Some("disconnect") {
                    return None;
                }
                let mut answer = ai_views::respond(
                    if room_enabled.load(Ordering::Acquire) {
                        &views
                    } else {
                        &unavailable_views
                    },
                    &keys,
                    &request.method,
                    &request.path,
                    request.headers.get("authorization").map(String::as_str),
                    Some(1000),
                    5_000_000,
                )
                .expect("real gateway AI route");
                // Negative cases corrupt one field of the real authenticated producer
                // response. Successful responses are always the untouched gateway bytes.
                if answer.status == 200 {
                    if let Some(field) = fault {
                        let mut value: Value =
                            serde_json::from_slice(&answer.body).expect("producer response");
                        value["result"]["binding"][&field] = json!("invalid-binding");
                        answer.body = serde_json::to_vec(&value).expect("corrupted response");
                    }
                }
                Some(answer)
            })
        };
        let mut gateway = start_gateway();
        let mut values = BTreeMap::from([
            (ENDPOINT.into(), gateway.url()),
            (KEY.into(), credential.clone()),
            (CA.into(), gateway_ca.to_string_lossy().into_owned()),
        ]);
        assert!(
            InferenceAvailabilityConfig::load(false, NODE_KEY, AUTHORITY_KEY, |_| panic!(
                "gateway settings must only load in inference mode"
            ))
            .expect("other modes")
            .is_none()
        );
        for name in [ENDPOINT, KEY, CA] {
            let mut missing = values.clone();
            missing.remove(name);
            let error = availability(&missing)
                .err()
                .expect("missing setting must refuse");
            assert!(error.contains(name));
            assert!(!error.contains(&credential));
        }
        for endpoint in [
            "http://localhost",
            "https://secret@localhost",
            "https://localhost/?secret",
            "https://localhost/#secret",
            "https://localhost:0",
        ] {
            let mut invalid = values.clone();
            invalid.insert(ENDPOINT.into(), endpoint.into());
            let error = availability(&invalid)
                .err()
                .expect("invalid HTTPS origin must refuse");
            assert!(!error.contains("secret"));
        }
        for key in [" ", NODE_KEY, AUTHORITY_KEY, "secret\r\nInjected: header"] {
            let mut invalid = values.clone();
            invalid.insert(KEY.into(), key.into());
            let error = availability(&invalid)
                .err()
                .expect("invalid key must refuse");
            assert!(error.starts_with(KEY));
            assert!(
                !error.contains(NODE_KEY)
                    && !error.contains(AUTHORITY_KEY)
                    && !error.contains("Injected")
            );
        }
        for root in [
            directory.path().join("missing.der"),
            directory.path().to_owned(),
        ] {
            let mut invalid = values.clone();
            invalid.insert(CA.into(), root.to_string_lossy().into_owned());
            assert!(availability(&invalid).is_err());
        }
        let invalid_ca = directory.path().join("invalid.der");
        fs::write(&invalid_ca, b"invalid root").expect("invalid certificate fixture");
        let mut invalid = values.clone();
        invalid.insert(CA.into(), invalid_ca.to_string_lossy().into_owned());
        assert!(availability(&invalid).is_err());
        let profile = directory.path().join("room-profile");
        fs::write(&profile, b"room profile").expect("room profile");
        let mut config = fixture_config(
            &values,
            &node,
            node_certificate.certificate.cert.der().to_vec(),
            profile,
        );
        let reader = || {
            program_reader(&config, ai_producer::verifier(), Registry::new())
                .expect("production node reader")
        };
        let node_reader = reader();
        let gate = Arc::new(Mutex::new(layerx_agentd::boot::InferenceBootGate::default()));
        let refuse = |settings: &BTreeMap<String, String>| {
            let availability = availability(settings)
                .expect("valid configuration")
                .expect("inference configuration");
            let client = availability.client().expect("gateway client");
            let reference = layerx_agentd::degraded::Reference {
                head_sequence: 1000,
                checkpoint: if room_enabled.load(Ordering::Acquire) {
                    block_hash
                } else {
                    unavailable_block_hash
                },
            };
            read_availability(
                &client,
                &availability.endpoint,
                "",
                125,
                &[11; 32],
                reference,
            )
            .expect_err("availability must be refused")
        };
        let mut wrong_key = values.clone();
        wrong_key.insert(KEY.into(), "unknown:lxp_live_invalid".into());
        assert_eq!(refuse(&wrong_key), "inference availability read refused");
        keys.revoke_key(
            &record.key_id,
            &record.principal_digest,
            "revoke-inference-first",
        )
        .expect("durable revocation");
        assert_eq!(refuse(&values), "inference availability read refused");
        let (_, replacement) = issue(&keys, "inference-replacement");
        values.insert(KEY.into(), replacement);
        let node_ca_file = node_certificate.write(directory.path(), "node");
        let mut wrong_roots = values.clone();
        wrong_roots.insert(CA.into(), node_ca_file.to_string_lossy().into_owned());
        assert_eq!(refuse(&wrong_roots), "inference availability read failed");
        for field in [
            "chain",
            "program",
            "market",
            "block_height",
            "block_hash",
            "rank",
        ] {
            *fault.lock().expect("fault") = Some(field.into());
            assert_eq!(
                refuse(&values),
                "inference availability is not bound to the current committed program"
            );
        }
        *fault.lock().expect("fault") = None;
        room_enabled.store(false, Ordering::Release);
        assert!(
            refuse(&values).contains("F11Unavailable"),
            "an unavailable F11 must close availability"
        );
        *fault.lock().expect("fault") = Some("disconnect".into());
        assert_eq!(refuse(&values), "inference availability read failed");
        *fault.lock().expect("fault") = None;
        config.inference_availability =
            availability(&values).expect("replacement gateway configuration");
        let agent = config
            .inference_availability
            .as_ref()
            .expect("configuration")
            .client()
            .expect("client");
        let observation = inference_observation(&config, &node_reader, &agent);
        assert!(
            observation
                .as_ref()
                .is_err_and(|error| error.contains("F11Unavailable")),
            "the production observation must reach the F11 refusal: {observation:?}"
        );
        assert!(inference_boot(
            &config,
            config.inference.clone().expect("inference mode"),
            program_reader(&config, ai_producer::verifier(), Registry::new()).expect("reader"),
        )
        .err()
        .expect("unavailable F11 must refuse boot")
        .contains("F11Unavailable"));
        assert!(observe_availability(&gate, observation).is_err());
        assert_ne!(
            gate.lock().expect("gate").status().mode,
            layerx_agentd::degraded::Mode::Healthy
        );
        assert!(!paths.lock().expect("node requests").is_empty());
        assert!(*gateway_requests.lock().expect("gateway requests") > 0);
        // Reopen the real gateway reader and replay the producer capture, preserving
        // its durable snapshot identity across recovery.
        drop(gateway);
        room_enabled.store(true, Ordering::Release);
        let (_, recovered) = ai_producer::ingest(&database, 1000, 4, true);
        assert_eq!(committed, recovered);
        gateway = start_gateway();
        values.insert(ENDPOINT.into(), gateway.url());
        config.inference_availability =
            availability(&values).expect("reopened gateway configuration");
        let availability = config
            .inference_availability
            .as_ref()
            .expect("gateway configuration");
        let client = availability.client().expect("gateway client");
        let mut response = client
            .get(&format!(
                "{}/v1/ai/markets/0x{market}/snapshot",
                gateway.url()
            ))
            .call()
            .expect("authenticated real gateway");
        assert_eq!(response.status().as_u16(), 200);
        let produced: Value = response
            .body_mut()
            .read_json()
            .expect("indexer-produced snapshot");
        assert_eq!(produced["result"]["committed"], true);
        assert_eq!(
            produced["result"]["snapshot_id"],
            format!("0x{}", committed["id"].as_str().expect("snapshot id"))
        );
        // This case's own capture funds TaskPolicyV1::minimum_epoch_funding through
        // rewards::apply(FUND), producing F06, then dispatches OpenRoom with a
        // RoomProfile::minimum_bond above zero, producing the decodable placement
        // book that feature_availability projects as available F11.
        assert_eq!(
            produced["result"]["components"]["F11"], "available",
            "the real producer must expose available F11 before inference startup can succeed"
        );
        let supervision = inference_boot(
            &config,
            config.inference.clone().expect("inference mode"),
            program_reader(&config, ai_producer::verifier(), Registry::new()).expect("reader"),
        )
        .expect("healthy inference boot with the real gateway");
        assert_health(&supervision, true);
        *fault.lock().expect("fault") = Some("disconnect".into());
        assert!(observe_availability(
            &supervision.market.gate,
            inference_observation(&config, &supervision.reader, &supervision.agent)
        )
        .is_err());
        assert_health(&supervision, false);
        *fault.lock().expect("fault") = None;
        observe_availability(
            &supervision.market.gate,
            inference_observation(&config, &supervision.reader, &supervision.agent),
        )
        .expect("supervision recovery through real gateway");
        assert_health(&supervision, true);
    }

    mod ai_producer {
        use ed25519_dalek::{Signer as _, SigningKey};
        use layerx_committed_proof::{
            commit::{BlockIdFlag, Commit, CommitSig},
            header::{
                validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp,
                Validator,
            },
            proto::{encode_length_delimited, encode_varint, encode_varint_field},
            ProofError,
        };
        use layerx_indexer::ai_market::ViewError;
        use layerx_indexer::codec::hex;
        use layerx_programs_ai_market::{
            codec::{
                self, decode_envelope, derive_market, derive_worker, encode_envelope, Envelope,
            },
            dispatch,
            errors::ApplicationError,
            policy::{PolicyCommitments, RoomProfile, TaskPolicyV1, TASK_POLICY_BYTES},
            queries::{read_state_chunk, CaptureFacts, QueryError, ReadProof, StateCapture},
            registry::{derive_rewards_account, F01_SECTION_CAP},
            registry_ops::{self, Outcome},
            rewards::{self, FundRequest, FUNDING_POLICY_VERSION, FUND_PAYLOAD_BYTES},
            rooms::{OpenRoomRequest, OPEN_ROOM_REQUEST_BYTES},
            state::{self, Section, SharedState},
            types::{
                AccountId, AssetId, Authentication, ChainDomain, Digest32, MarketId,
                MetadataDigest, Presence, PrincipalId, ProgramId, PublicKey32, RequestId,
                RosterDigest, RubricDigest, StateDigest,
            },
            workers::{WorkerCurrent, WorkerState, WorkerTable, WORKER_TABLE_MAX_BYTES},
            MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
        };
        use serde_json::{json, Value};
        use sha2::{Digest as _, Sha256};
        use std::fs;

        fn chain_bytes() -> [u8; 32] {
            let mut digest = Sha256::new();
            digest.update(b"PAXAI/chain-domain/v1");
            digest.update(125_u64.to_be_bytes());
            digest.finalize().into()
        }
        const PROGRAM: [u8; 32] = [11; 32];
        const OWNER: [u8; 32] = [12; 32];
        const ASSET: [u8; 32] = [13; 32];
        const REFUND: [u8; 32] = [14; 32];
        const CHUNK_RESPONSE_MAX: usize = 8_244;
        const CHAIN_ID: &str = "ai-market-finality";
        fn clock() -> time::OffsetDateTime {
            static CLOCK: std::sync::OnceLock<time::OffsetDateTime> = std::sync::OnceLock::new();
            *CLOCK.get_or_init(|| {
                time::OffsetDateTime::now_utc()
                    .replace_nanosecond(0)
                    .expect("whole-second fixture clock")
            })
        }
        fn stamp(nanos: u32) -> String {
            clock()
                .replace_nanosecond(nanos)
                .expect("fixture nanoseconds")
                .format(&time::format_description::well_known::Rfc3339)
                .expect("fixture timestamp")
        }
        const PUBLISHED_AT_MS: u64 = 2_000;
        const NOW_MS: u64 = 5_000_000;
        const SECRET: [u8; 32] = [0x5A; 32];

        #[derive(Debug)]
        struct Failure(String);
        type Checked<T = ()> = Result<T, Failure>;
        macro_rules! convert { ($($error:ty),+ $(,)?) => {$ (
    impl From<$error> for Failure { fn from(error:$error)->Self {Self(format!("{error:?}"))} }
)+}; }
        convert!(
            ApplicationError,
            QueryError,
            ProofError,
            ViewError,
            layerx_wire::WireError,
            std::io::Error,
            &'static str
        );
        fn chain() -> Checked<ChainDomain> {
            Ok(ChainDomain::new(chain_bytes())?)
        }
        fn program() -> Checked<ProgramId> {
            Ok(ProgramId::new(PROGRAM)?)
        }
        fn market() -> Checked<MarketId> {
            Ok(derive_market(chain()?, program()?)?)
        }
        fn market_hex() -> Checked<String> {
            Ok(hex(market()?.as_bytes()))
        }
        fn principal(bytes: [u8; 32]) -> Checked<PrincipalId> {
            Ok(PrincipalId::new(bytes)?)
        }
        fn digest(byte: u8) -> Checked<Digest32> {
            Ok(Digest32::new([byte; 32])?)
        }

        fn policy() -> Checked<TaskPolicyV1> {
            Ok(TaskPolicyV1::bounded_default(
                1,
                1,
                PolicyCommitments {
                    model_artifact: digest(1)?,
                    dataset_artifact: [2; 32],
                    benchmark_suite: digest(3)?,
                    rubric: RubricDigest::new([4; 32])?,
                    task_schema: digest(5)?,
                    result_schema: digest(6)?,
                    service_terms: digest(7)?,
                },
                100,
                1,
            )?)
        }

        fn encode_shared(shared: &SharedState<'_>) -> Checked<Vec<u8>> {
            let mut out = vec![0; shared.encoded_len()?];
            let mut scratch = vec![0; Section::Control.payload_cap()];
            let n = state::encode_shared_state(shared, &mut out, &mut scratch)?;
            out.truncate(n);
            Ok(out)
        }

        /// Real F01 CREATE through `registry_ops::apply`.
        fn created_state() -> Checked<Vec<u8>> {
            let mut encoded_policy = vec![0; TASK_POLICY_BYTES];
            policy()?.encode(&mut encoded_policy)?;
            let rewards = derive_rewards_account(program()?, AssetId::new(ASSET)?)?;
            let mut payload = OWNER.to_vec();
            payload.extend_from_slice(&ASSET);
            payload.extend_from_slice(rewards.as_bytes());
            payload.extend_from_slice(&REFUND);
            payload.push(0);
            payload.extend_from_slice(&encoded_policy);
            payload.extend_from_slice(&[16; 32]);
            let envelope = Envelope {
                operation: dispatch::CREATE,
                chain: chain()?,
                program: program()?,
                market: market()?,
                actor: principal(OWNER)?,
                epoch: 0,
                config: 1,
                roster: Presence::Absent,
                sequence: 1,
                expiry: 1_000_000,
                request: RequestId::new([1; 32])?,
                payload: &payload,
                authentication: Authentication::Native,
            };
            let mut encoded = vec![0; 16_384];
            let n = encode_envelope(&envelope, &mut encoded)?;
            let validated = decode_envelope(encoded.get(..n).ok_or("envelope")?)?;
            let ctx = registry_ops::CallContext {
                chain: chain()?,
                program: program()?,
                principal: principal(OWNER)?,
                height: 1000,
            };
            let mut section = vec![0; F01_SECTION_CAP];
            let mut event = vec![0; MAX_EVENT_BYTES];
            match registry_ops::apply(&ctx, None, &validated, &mut section, &mut event)? {
                Outcome::Applied { state, .. } => encode_shared(&state),
                Outcome::AlreadyApplied(_) => Err(Failure("fresh create retried".to_owned())),
            }
        }

        fn worker(slot: u8, nonce: u8, state: WorkerState) -> Checked<WorkerCurrent> {
            Ok(WorkerCurrent {
                worker: derive_worker(market()?, principal([20; 32])?, [nonce; 32])?,
                owner: principal([20; 32])?,
                delegate: PublicKey32([slot + 50; 32]),
                metadata: MetadataDigest::new([30; 32])?,
                generation: 2,
                key_version: 2,
                metadata_revision: 4,
                valid_from: 200,
                expiry: 400,
                revocation_sequence: 0,
                effective_epoch: 1,
                last_sequence: 0,
                last_request_id: [0; 32],
                last_request_digest: [0; 32],
                last_result_digest: [0; 32],
                state,
                slot,
                last_metadata_height: 200,
            })
        }

        fn funded_state() -> Checked<Vec<u8>> {
            let current = created_state()?;
            let mut payload = vec![0; FUND_PAYLOAD_BYTES];
            FundRequest {
                amount: policy()?.minimum_epoch_funding,
                refund_recipient: AccountId::new(REFUND)?,
                policy_version: FUNDING_POLICY_VERSION,
                consent: true,
            }
            .encode(&mut payload)?;
            let envelope = Envelope {
                operation: dispatch::FUND,
                chain: chain()?,
                program: program()?,
                market: market()?,
                actor: principal(OWNER)?,
                epoch: 0,
                config: 1,
                roster: Presence::Absent,
                sequence: 2,
                expiry: 1_000_000,
                request: RequestId::new([2; 32])?,
                payload: &payload,
                authentication: Authentication::Native,
            };
            let mut encoded = vec![0; 16_384];
            let n = encode_envelope(&envelope, &mut encoded)?;
            let validated = decode_envelope(encoded.get(..n).ok_or("fund envelope")?)?;
            let ctx = registry_ops::CallContext {
                chain: chain()?,
                program: program()?,
                principal: principal(OWNER)?,
                height: 1000,
            };
            let mut next = vec![0; MAX_STATE_BYTES];
            let mut scratch = vec![0; rewards::SCRATCH_BYTES];
            let mut event = vec![0; MAX_EVENT_BYTES];
            let rewards::Outcome::Applied { state_len, .. } = rewards::apply(
                &ctx,
                &validated,
                &current,
                &mut next,
                &mut scratch,
                &mut event,
            )?
            else {
                return Err(Failure("fresh funding retried".to_owned()));
            };
            next.truncate(state_len);
            assert_eq!(
                layerx_programs_ai_market::queries::feature_availability(&next)?[5],
                layerx_programs_ai_market::queries::Availability::Available,
                "Fund must produce the real F06 reward state"
            );
            Ok(next)
        }

        fn room_state() -> Checked<Vec<u8>> {
            let current = funded_state()?;
            let shared = state::decode_shared_state(&current)?;
            let header =
                registry_ops::PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?
                    .header;
            let admission = layerx_programs_ai_market::admission::AdmissionTable::decode(
                layerx_programs_ai_market::reputation_transition::split_joint(
                    shared.section(Section::ReputationAdmission)?,
                )?
                .1,
            )?;
            let roster = RosterDigest::new(
                layerx_programs_ai_market::roster::roster_digest(&admission, 0)?.bytes(),
            )?;
            let request = OpenRoomRequest {
                room_id: digest(40)?,
                model_digest: policy()?.commitments.model_artifact,
                runtime_digest: digest(41)?,
                profile: RoomProfile {
                    minimum_bond: 100,
                    canary_ratio_bps: 250,
                    lap_size_tokens: 256,
                    challenge_window_blocks: 120,
                    slash_fraction_bps: 2500,
                    host_share_bps: 1000,
                    protocol_share_bps: 500,
                    settlement_account: AccountId::new(REFUND)?,
                    max_generated_tokens: 4096,
                    queue_depth: 32,
                    liveness_timeout_ms: 30_000,
                    acknowledgement_window_ms: 5_000,
                    canary_relative_tolerance_bps: 100,
                },
            };
            let mut payload = vec![0; OPEN_ROOM_REQUEST_BYTES];
            let n = request.encode(&mut payload)?;
            payload.truncate(n);
            let envelope = Envelope {
                operation: dispatch::OpenRoom,
                chain: chain()?,
                program: program()?,
                market: market()?,
                actor: principal(OWNER)?,
                epoch: 0,
                config: 1,
                roster: Presence::Present(roster),
                sequence: 3,
                expiry: 1_000_000,
                request: RequestId::new([3; 32])?,
                payload: &payload,
                authentication: Authentication::Native,
            };
            let mut encoded = vec![0; 16_384];
            let n = encode_envelope(&envelope, &mut encoded)?;
            let mut next = vec![0; MAX_STATE_BYTES];
            let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
            let mut event = vec![0; MAX_EVENT_BYTES];
            let mut result = vec![0; MAX_RESULT_BYTES];
            let routed = dispatch::route(
                &registry_ops::CallContext {
                    chain: chain()?,
                    program: program()?,
                    principal: principal(OWNER)?,
                    height: header.origin_height,
                },
                &encoded[..n],
                Some(&current),
                dispatch::Buffers {
                    next: &mut next,
                    scratch: &mut scratch,
                    event: &mut event,
                    result: &mut result,
                },
            )?;
            let dispatch::Routed::Applied { state_len, .. } = routed else {
                return Err(Failure(format!("fresh OpenRoom did not apply: {routed:?}")));
            };
            next.truncate(state_len);
            assert_eq!(
                layerx_programs_ai_market::queries::feature_availability(&next)?[10],
                layerx_programs_ai_market::queries::Availability::Available,
                "OpenRoom must produce the real F11 placement book",
            );
            Ok(next)
        }

        /// Funded market plus an F02 worker table; even slots available, odd slots retired. Seat
        /// `slot` is held by the worker derived from nonce `slot + nonce_base`.
        fn market_with_workers(count: u8, nonce_base: u8, room_enabled: bool) -> Checked<Vec<u8>> {
            let base = if room_enabled {
                room_state()?
            } else {
                funded_state()?
            };
            let shared = state::decode_shared_state(&base)?;
            let mut table = WorkerTable::new();
            for slot in 0..count {
                let status = if slot % 2 == 0 {
                    WorkerState::Available
                } else {
                    WorkerState::Retired
                };
                table.insert(&worker(slot, slot + nonce_base, status)?)?;
            }
            let mut section = vec![0; WORKER_TABLE_MAX_BYTES];
            let n = table.encode(&mut section)?;
            let identity = section.get(..n).ok_or("worker table")?;
            encode_shared(&shared.replace_section(Section::IdentityRoster, identity)?)
        }

        fn chunk_payload(
            revision: u64,
            pinned: Option<StateDigest>,
            offset: u32,
            requested: u16,
        ) -> Vec<u8> {
            let mut payload = revision.to_be_bytes().to_vec();
            payload.extend_from_slice(&pinned.map_or([0; 32], StateDigest::bytes));
            payload.extend_from_slice(&offset.to_be_bytes());
            payload.extend_from_slice(&requested.to_be_bytes());
            payload
        }

        fn read_all(state_bytes: &[u8]) -> Checked<Vec<Vec<u8>>> {
            let mut out = vec![0; CHUNK_RESPONSE_MAX];
            let n = read_state_chunk(state_bytes, &chunk_payload(0, None, 0, 8192), &mut out)?;
            let first = codec::decode_chunk_response(&out[..n])?;
            let (revision, pinned, total) = (first.revision, first.digest, first.total_bytes);
            let mut bodies = vec![out[..n].to_vec()];
            for offset in (8192..total).step_by(8192) {
                let payload = chunk_payload(revision, Some(pinned), offset, 8192);
                let n = read_state_chunk(state_bytes, &payload, &mut out)?;
                bodies.push(out[..n].to_vec());
            }
            Ok(bodies)
        }

        fn validators() -> Vec<Validator> {
            let mut validators: Vec<_> = (1..=4)
                .map(|value| Validator {
                    pub_key: SigningKey::from_bytes(&[value; 32])
                        .verifying_key()
                        .to_bytes(),
                    voting_power: 1,
                })
                .collect();
            validators.sort_by_key(|v| Sha256::digest(v.pub_key)[..20].to_vec());
            validators
        }

        fn signed(header: &Header, signers: &[(u8, bool)]) -> Commit {
            let mut commit = Commit {
                height: header.height as u64,
                round: 0,
                block_id: BlockId {
                    hash: header.hash().to_vec(),
                    part_set_header: PartSetHeader {
                        total: 1,
                        hash: vec![0xA5; 32],
                    },
                },
                signatures: Vec::new(),
            };
            let mut seeds: Vec<u8> = (1..=4).collect();
            seeds.sort_by_key(|v| {
                Sha256::digest(SigningKey::from_bytes(&[*v; 32]).verifying_key().as_bytes())[..20]
                    .to_vec()
            });
            for value in seeds {
                let Some((_, tamper)) = signers.iter().find(|(signer, _)| *signer == value) else {
                    commit.signatures.push(CommitSig::absent());
                    continue;
                };
                let key = SigningKey::from_bytes(&[value; 32]);
                let timestamp = Timestamp {
                    seconds: clock().unix_timestamp(),
                    nanos: 123,
                };
                let mut vote = Vec::new();
                encode_varint_field(1, 2, &mut vote);
                vote.push(0x11);
                vote.extend_from_slice(&commit.height.to_le_bytes());
                let mut parts = Vec::new();
                encode_varint_field(
                    1,
                    u64::from(commit.block_id.part_set_header.total),
                    &mut parts,
                );
                encode_length_delimited(2, &commit.block_id.part_set_header.hash, &mut parts);
                let mut block = Vec::new();
                encode_length_delimited(1, &commit.block_id.hash, &mut block);
                encode_length_delimited(2, &parts, &mut block);
                encode_length_delimited(4, &block, &mut vote);
                let mut time = Vec::new();
                encode_varint_field(1, timestamp.seconds as u64, &mut time);
                encode_varint_field(2, u64::from(timestamp.nanos), &mut time);
                encode_length_delimited(5, &time, &mut vote);
                encode_length_delimited(6, header.chain_id.as_bytes(), &mut vote);
                let mut sign_bytes = Vec::new();
                encode_varint(vote.len() as u64, &mut sign_bytes);
                sign_bytes.extend_from_slice(&vote);
                let mut signature = key.sign(&sign_bytes).to_bytes();
                if *tamper {
                    signature[40] ^= 1;
                }
                commit.signatures.push(CommitSig {
                    block_id_flag: BlockIdFlag::Commit,
                    validator_address: Sha256::digest(key.verifying_key().as_bytes())[..20]
                        .to_vec(),
                    timestamp,
                    signature: Some(signature),
                });
            }
            commit
        }

        fn header(height: u64, root: Digest32) -> Header {
            Header {
                chain_id: CHAIN_ID.to_owned(),
                height: height as i64,
                app_hash: root.bytes().to_vec(),
                version: ConsensusVersion { block: 11, app: 1 },
                time: Timestamp {
                    seconds: clock().unix_timestamp(),
                    nanos: 0,
                },
                last_block_id: BlockId::default(),
                last_commit_hash: vec![1; 32],
                data_hash: layerx_programs::chain_proof::merkle::hash_from_byte_slices(&[
                    &Sha256::digest(&batch(height)),
                ])
                .to_vec(),
                validators_hash: validators_hash(&validators()),
                next_validators_hash: validators_hash(&validators()).to_vec(),
                consensus_hash: vec![3; 32],
                last_results_hash: vec![4; 32],
                evidence_hash: vec![5; 32],
                proposer_address: Sha256::digest(validators()[0].pub_key)[..20].to_vec(),
            }
        }

        struct Material {
            header: Header,
            commit: Commit,
            proof: Vec<u8>,
        }
        // Canonical native fixture: the namespace head/manifest layout emitted by lxphost,
        // with ICS23 leaf operations encoded by the production protobuf codec.
        fn namespace(
            state: &[u8],
            program: [u8; 32],
            cell: &[u8],
        ) -> (super::chain::ChainFixture, Vec<u8>, Vec<u8>) {
            let mut manifest = vec![0, 1];
            manifest.extend(1_u32.to_be_bytes());
            manifest.extend((cell.len() as u16).to_be_bytes());
            manifest.extend(cell);
            manifest.extend(Sha256::digest(state));
            manifest.extend((state.len() as u32).to_be_bytes());
            let mut head = vec![0, 1];
            head.extend(1_u32.to_be_bytes());
            head.extend(Sha256::digest(&manifest));
            let mut key = b"\x01\x00\x09progstor".to_vec();
            key.extend(program);
            key.push(1);
            let fixture = super::chain::ChainFixture::new(
                CHAIN_ID,
                &[[1; 32], [2; 32], [3; 32], [4; 32]],
                vec![(key.clone(), head), (vec![2], 2_u64.to_be_bytes().to_vec())],
            )
            .expect("real IAVL namespace and host marker");
            (fixture, key, manifest)
        }

        fn native_membership(state: &[u8], program: [u8; 32], cell: &[u8]) -> (Vec<u8>, [u8; 32]) {
            let (fixture, key, manifest) = namespace(state, program, cell);
            let ops = fixture.prove(&key).expect("namespace proof");
            let encoded = &ops[0].data;
            assert_eq!(encoded[0], 10);
            let mut offset = 1;
            while encoded[offset] & 128 != 0 {
                offset += 1;
            }
            offset += 1;
            let proof = &encoded[offset..];
            (serde_json::to_vec(&json!({"membership":{"proof":proof,"multistore":ops[1].data},"manifest":manifest})).expect("native namespace proof"), fixture.header.app_hash.as_slice().try_into().expect("root"))
        }

        pub(super) fn batch(height: u64) -> Vec<u8> {
            let mut bytes = b"PAXEER/v1/sequence-batch\0".to_vec();
            bytes.extend(height.to_be_bytes());
            bytes.extend(1_u64.to_be_bytes());
            bytes.extend(2_u64.to_be_bytes());
            bytes.resize(b"PAXEER/v1/sequence-batch\0".len() + 216, 0);
            bytes
        }

        pub(super) fn node_result(method: &str, block_hash: [u8; 32], room_enabled: bool) -> Value {
            let state = market_with_workers(4, 0, room_enabled).expect("market producer");
            let (fixture, _, _) =
                namespace(&state, PROGRAM, layerx_programs_ai_market::SHARED_STATE_KEY);
            let (_, _, committed) = consensus(1000, room_enabled);
            match method {
                "eth_chainId" => json!("0x7d"),
                "eth_blockNumber" => json!("0x3e8"),
                "px_getReceiptKeys" => {
                    json!({"keys":[{"keyId":format!("0x{}", "01".repeat(32)),"publicKey":format!("0x{}", hex(&validators()[0].pub_key)),"firstHeight":1,"lastHeight":2000}]})
                }
                "eth_getBlockByNumber" => {
                    json!({"number":"0x3e8","hash":format!("0x{}",hex(&block_hash)),"parentHash":format!("0x{}","01".repeat(32)),"timestamp":"0x3e8","transactionsRoot":format!("0x{}","02".repeat(32)),"receiptsRoot":format!("0x{}","03".repeat(32)),"transactions":[]})
                }
                "commit" => committed,
                "validators" => {
                    json!({"block_height":"1000","count":"4","total":"4","validators":validators().iter().map(|v|json!({"address":hex(&Sha256::digest(v.pub_key)[..20]),"pub_key":{"type":"tendermint/PubKeyEd25519","value":layerx_platform_internal::base64::encode(&v.pub_key)},"voting_power":"1"})).collect::<Vec<_>>()})
                }
                "block" => {
                    json!({"block":{"header":{"height":"1000"},"data":{"txs":[layerx_platform_internal::base64::encode(&batch(1000))]}}})
                }
                "abci_query" => {
                    json!({"response":{"code":0,"height":"999","key":"Ag==","value":layerx_platform_internal::base64::encode(&2_u64.to_be_bytes()),"proof_ops":{"ops":fixture.prove(&[2]).expect("host marker proof").iter().map(|op|json!({"type":op.op_type,"key":layerx_platform_internal::base64::encode(&op.key),"data":layerx_platform_internal::base64::encode(&op.data)})).collect::<Vec<_>>()}}})
                }
                _ => panic!("unexpected node method {method}"),
            }
        }

        pub(super) fn verifier() -> layerx_programs::ProtocolDeploymentVerifier {
            layerx_programs::ProtocolDeploymentVerifier::from_chain_trust(
                CHAIN_ID,
                validators()
                    .iter()
                    .map(|v| {
                        let voting_power = u64::try_from(v.voting_power)
                            .expect("validator voting power must not be negative");
                        layerx_programs::chain_proof::validators::TrustedValidator::new(
                            v.pub_key,
                            voting_power,
                        )
                    })
                    .collect(),
                60_000,
            )
            .expect("fixture chain trust")
        }

        fn certify_at(state: &[u8], height: u64) -> Checked<Material> {
            let program = PROGRAM;
            let cell = layerx_programs_ai_market::SHARED_STATE_KEY;
            let (proof, root) = native_membership(state, program, cell);
            let header = header(height, Digest32::new(root)?);
            let commit = signed(&header, &[(1, false), (2, false), (3, false), (4, false)]);
            Ok(Material {
                header,
                commit,
                proof,
            })
        }

        fn capture_state(
            state: &[u8],
            material: &Material,
        ) -> Checked<(
            ReadProof,
            Vec<Vec<u8>>,
            layerx_programs_ai_market::queries::CaptureFacts,
        )> {
            let proof = ReadProof {
                chain: chain()?,
                program: program()?,
                native_state_root: Digest32::new(
                    material
                        .header
                        .app_hash
                        .as_slice()
                        .try_into()
                        .expect("root"),
                )?,
                observed_sequence: material.header.height as u64,
                execution_height: material.header.height as u64,
                block_hash: Digest32::new(material.header.hash())?,
            };
            let chunks = read_all(state)?;
            let mut bytes = vec![0; MAX_STATE_BYTES];
            let mut capture = StateCapture::new(&mut bytes);
            for chunk in &chunks {
                capture.accept(&proof, chunk)?;
            }
            let (assembled, facts) = capture.finish()?;
            assert_eq!(assembled, state);
            Ok((proof, chunks, facts))
        }

        pub(super) fn consensus(height: u64, room_enabled: bool) -> (Value, Value, Value) {
            let state = market_with_workers(4, 0, room_enabled).expect("real market state");
            let material = certify_at(&state, height).expect("signed consensus material");
            let h = &material.header;
            let c = &material.commit;
            let block = |b: &BlockId| json!({"hash":hex(&b.hash),"parts":{"total":b.part_set_header.total,"hash":hex(&b.part_set_header.hash)}});
            let header = json!({"version":{"block":h.version.block.to_string(),"app":h.version.app.to_string()},
            "chain_id":h.chain_id,"height":h.height.to_string(),"time":stamp(0),
            "last_block_id":block(&h.last_block_id),"last_commit_hash":hex(&h.last_commit_hash),"data_hash":hex(&h.data_hash),
            "validators_hash":hex(&h.validators_hash),"next_validators_hash":hex(&h.next_validators_hash),
            "consensus_hash":hex(&h.consensus_hash),"app_hash":hex(&h.app_hash),"last_results_hash":hex(&h.last_results_hash),
            "evidence_hash":hex(&h.evidence_hash),"proposer_address":hex(&h.proposer_address)});
            let commit = json!({"height":c.height.to_string(),"round":c.round,"block_id":block(&c.block_id),
            "signatures":c.signatures.iter().map(|v|json!({"block_id_flag":2,"validator_address":hex(&v.validator_address),
                "timestamp":stamp(123),"signature":v.signature.as_ref().map(|v|layerx_platform_internal::base64::encode(v))})).collect::<Vec<_>>()});
            let status = json!({"sync_info":{"latest_block_height":h.height.to_string(),"latest_block_hash":hex(&h.hash()),"latest_app_hash":hex(&h.app_hash),"catching_up":false}});
            let authority = json!({"chain_id":CHAIN_ID,"validators":validators().iter().map(|v|json!({"public_key":hex(&v.pub_key),"voting_power":v.voting_power})).collect::<Vec<_>>()});
            (
                authority,
                status,
                json!({"signed_header":{"header":header,"commit":commit}}),
            )
        }

        pub(super) fn ingest(
            database: &std::path::Path,
            height: u64,
            count: u8,
            room_enabled: bool,
        ) -> (String, Value) {
            let state = market_with_workers(count, 0, room_enabled).expect("real market producer");
            let material = certify_at(&state, height).expect("native membership and signed commit");
            let (_, chunks, _) =
                capture_state(&state, &material).expect("real whole-root chunk codec");
            let config = serde_json::json!({"chain":chain_bytes(),"program":PROGRAM,"consensus_chain_id":CHAIN_ID,
            "validators": validators().iter().map(|v|serde_json::json!({"pub_key":v.pub_key,"voting_power":v.voting_power})).collect::<Vec<_>>(),
            "source":"http://127.0.0.1:1","ca_der":null,"allow_remote_plaintext":false});
            let path = database.with_extension("config.json");
            fs::write(&path, serde_json::to_vec(&config).expect("runtime config"))
                .expect("runtime config file");
            let (mut runtime, _) = layerx_indexer::ai_runtime::RuntimeConfig::read(&path)
                .expect("runtime config")
                .open(database)
                .expect("real projection runtime");
            let h = &material.header;
            let c = &material.commit;
            // The transport carries the real read_state_chunk output and signed
            // native membership evidence; AiRuntime authenticates the entire capture.
            let proof = layerx_indexer::ai_market::BlockCommitProof {
                header: h,
                commit: c,
                store_proof: &material.proof,
            };
            let payload = layerx_indexer::ai_runtime::encode_committed_read(
                chain().expect("chain"),
                program().expect("program"),
                height,
                &proof,
                &chunks,
            )
            .expect("production committed transport encoder");
            let record = runtime
                .ingest(&payload, 1_000_000)
                .expect("committed market ingestion");
            assert!(record.projection.finalized());
            let market = market_hex().expect("market identifier");
            (
                market,
                serde_json::json!({"height":height,"id":hex(record.snapshot_id.as_bytes())}),
            )
        }
    }
}
