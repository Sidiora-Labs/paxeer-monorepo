use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::{SocketAddrV4, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt as _;
use std::path::Path;
use std::process::{Command, Output, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_agentd::agent_rpc;
use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope};
use layerx_agentd::capability::timed::{self, NativeTimedCapabilityV1, TimedCapability};
use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
use layerx_agentd::enrolment::readiness_tls::{
    self as tls,
    pki_types::{pem::PemObject as _, CertificateDer, PrivateKeyDer},
};
use layerx_agentd::enrolment::{
    self, BindingMode, BindingPublisher, DaemonSurface, EnrolmentRequest, PublishedBinding,
    ReadinessDeclaration,
};
use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::prepare::PreparationLifecycle;
use layerx_agentd::session::{SessionId, SessionRegistry};
use layerx_agentd::session_control::{McpInvocationIdentity, SessionControl};
use layerx_agentd::store::{Store, TenantId};
use layerx_mcp::binding::{Binding, OwnerEnvironment};
use layerx_mcp::server::{catalogue, ToolKind};
use layerx_mcp::stdio::DaemonClientSession;
use layerx_sdk::agent_envelope::{AgentEnvelopeTransport, EnvelopeCredential};
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::verify::VerificationLevel;
use serde_json::Value;
use sha2::{Digest as _, Sha256};

const OBSERVED_HEIGHT: u64 = 120;

struct BoundaryIdentity(CoreIdentity);

impl IdentityResolver for BoundaryIdentity {
    fn resolve(&mut self, _did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(self.0.clone()))
    }
}

fn enrol(
    root: &Path,
    endpoint: &str,
) -> (
    PublishedBinding,
    BindingPublisher,
    layerx_agentd::identity::IdentityRecord,
    EnrolmentRequest,
) {
    let mut store =
        Store::open(root.join("store")).unwrap_or_else(|error| panic!("store: {error}"));
    let tenant = TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}"));
    let capability = Capability::new(
        CapabilityId([9; 32]),
        tenant.clone(),
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
    )
    .unwrap_or_else(|error| panic!("capability: {error:?}"));
    capability
        .persist(&mut store)
        .unwrap_or_else(|error| panic!("capability persist: {error:?}"));
    let mut boundary = BoundaryIdentity(CoreIdentity {
        canonical_bytes: b"model-identity".to_vec(),
        head_sequence: 10,
        revocation_sequence: 1,
        verification_level: VerificationLevel::STATE_PROVEN,
        frozen: false,
        authorities: vec![ProtocolAuthority::CapabilityGrant(capability.id.0)],
    });
    let identity = register(
        &mut store,
        tenant,
        Did::new(b"did:layerx:model").unwrap_or_else(|error| panic!("DID: {error:?}")),
        &mut boundary,
    )
    .unwrap_or_else(|error| panic!("identity: {error:?}"));
    let binding_publisher_handle = binding_publisher(root, endpoint);
    let mut sessions = SessionRegistry::default();
    let request = EnrolmentRequest {
        session_id: SessionId([0x0c; 32]),
        capability_id: capability.id,
        permitted_activity_types: BTreeSet::from([7]),
        scopes: catalogue()
            .iter()
            .map(|tool| tool.required_scope.to_owned())
            .collect(),
        expiry_sequence: 300,
        opening_client: "mcp".to_owned(),
        policy_version: "policy-v1".to_owned(),
        core_sequence: 50,
    };
    let published = enrolment::enrol(
        &mut store,
        &mut sessions,
        &identity,
        request.clone(),
        &binding_publisher_handle,
    )
    .unwrap_or_else(|error| panic!("enrol: {error}"));
    drop(store);
    persist_native_grant(root);
    (published, binding_publisher_handle, identity, request)
}

fn binding_publisher(root: &Path, endpoint: &str) -> BindingPublisher {
    let surface = DaemonSurface::new("127.0.0.1:9", "d".repeat(48), [0xcc; 32])
        .unwrap_or_else(|error| panic!("surface: {error}"))
        .with_readiness(
            ReadinessDeclaration::new(
                endpoint,
                root.join("gateway-key"),
                Some(root.join("ca.pem")),
            )
            .unwrap_or_else(|error| panic!("{}: {error:?}", "readiness declaration")),
        );
    BindingPublisher::new(
        root.join("mcp"),
        root.join("store"),
        root.join("audit"),
        surface,
        LimitConfig {
            id: LimitId([0x0a; 16]),
            name: "mcp".to_owned(),
            scope: LimitScope::Tenant([1; 32]),
            ceiling: 1_000,
            consumed: 0,
        },
        Duration::from_millis(5_000),
        BindingMode::Full,
    )
    .unwrap_or_else(|error| panic!("publisher: {error}"))
}

fn persist_native_grant(root: &Path) {
    let mut store = Store::open(root.join("store"))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "native capability store"));
    let tenant =
        TenantId::new("tenant-a").unwrap_or_else(|error| panic!("{}: {error:?}", "native tenant"));
    let native = NativeTimedCapabilityV1 {
        record: TimedCapability {
            id: [9; 32],
            parent: None,
            tenant: tenant.clone(),
            agent: "did:layerx:model".into(),
            authority: ProtocolAuthority::CapabilityGrant([9; 32]),
            activity_types: BTreeSet::new(),
            counterparties: BTreeSet::from([[2; 32]]),
            assets: BTreeSet::from([[3; 32]]),
            amount_ceilings: BTreeMap::from([([3; 32], 100)]),
            rate_ceilings: BTreeMap::from([(10, 2)]),
            purposes: BTreeSet::new(),
            expiry_seconds: 400,
            grant_not_after_ms: 400_000,
            created_at_ms: 1,
            created_at_sequence: 1,
            revoked: None,
        },
        activities: [1, 5, 6, 7, 10]
            .into_iter()
            .map(|ordinal| {
                ActivityType::new(ModuleId::Asset, ordinal)
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "native activity"))
                    .into()
            })
            .collect(),
        purpose_commitments: BTreeSet::new(),
        spend_ceilings: BTreeMap::new(),
    };
    store
        .put_local(
            timed::native_record_key(&tenant, &[9; 32])
                .unwrap_or_else(|error| panic!("{}: {error:?}", "native key")),
            native
                .encode()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "native capability")),
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "persist native grant"));
    drop(store);
}

const FIXTURE_CA: &str = r"-----BEGIN CERTIFICATE-----
MIIBlDCCATugAwIBAgIUNP6tUTUFCd7EGaudA/UkPfUpH+wwCgYIKoZIzj0EAwIw
HzEdMBsGA1UEAwwURW5yb2xtZW50IGZpeHR1cmUgQ0EwIBcNMjYxMTA0MDIyMDQw
WhgPMjEyNjEwMTEwMjIwNDBaMB8xHTAbBgNVBAMMFEVucm9sbWVudCBmaXh0dXJl
IENBMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEpCS3VZLS9I9lC5zRj22fJth6
sUErpI7gfy5U8+rlCdI2n74I02y/FKC4GjvFpNzBUqYCCWO7riE+QCS5ce5oL6NT
MFEwHQYDVR0OBBYEFPe/gbinSw+v4J+ybGh79t8iI7RhMB8GA1UdIwQYMBaAFPe/
gbinSw+v4J+ybGh79t8iI7RhMA8GA1UdEwEB/wQFMAMBAf8wCgYIKoZIzj0EAwID
RwAwRAIgCm91etnqdnb0RxhV1K+/JtItK9BAdJdmvd1Py+BlndkCIFq4svi/AO6M
IeS+x/4AIbH1s/GmTtXt0p2/iYhhuUEz
-----END CERTIFICATE-----
";

const FIXTURE_CERT: &str = r"-----BEGIN CERTIFICATE-----
MIIBwzCCAWqgAwIBAgIURbkxTKfYXcMVQXtjHhdSOhWQlhQwCgYIKoZIzj0EAwIw
HzEdMBsGA1UEAwwURW5yb2xtZW50IGZpeHR1cmUgQ0EwIBcNMjYxMTA0MDIyMDQw
WhgPMjEyNjEwMTEwMjIwNDBaMBQxEjAQBgNVBAMMCWxvY2FsaG9zdDBZMBMGByqG
SM49AgEGCCqGSM49AwEHA0IABBibKfnl1APRAWLzIbk1HWrwyD4tCq4itqbProjg
al2cA0aM9FZPnMXcL5wZNnpExZT6P9RpVFCsZhwIteW16N6jgYwwgYkwDAYDVR0T
AQH/BAIwADAOBgNVHQ8BAf8EBAMCB4AwEwYDVR0lBAwwCgYIKwYBBQUHAwEwFAYD
VR0RBA0wC4IJbG9jYWxob3N0MB0GA1UdDgQWBBRFsFH0WadiCcq9oBnRyepgWufD
WDAfBgNVHSMEGDAWgBT3v4G4p0sPr+Cfsmxoe/bfIiO0YTAKBggqhkjOPQQDAgNH
ADBEAiBe+llwgYyUukEYLW+o4d0P/uwaMU+refDoqjxaWEN64gIgY/+qpcjI7i0C
KD0S/iQJo+IhkcHr0hFRrr3khSj+MU4=
-----END CERTIFICATE-----
";

const FIXTURE_KEY: &str = r"-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg9a2umZJLxAShe3Ai
GrazueSKt9VKHVVG97limBRZ2SuhRANCAAQYmyn55dQD0QFi8yG5NR1q8Mg+LQqu
Iramz66I4GpdnANGjPRWT5zF3C+cGTZ6RMWU+j/UaVRQrGYcCLXlteje
-----END PRIVATE KEY-----
";

fn transport_files(root: &Path) {
    fs::write(root.join("ca.pem"), FIXTURE_CA)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "fixture trust anchor"));
    let mut secret = [0; 32];
    getrandom::fill(&mut secret)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "gateway credential entropy"));
    let secret = secret.iter().fold(String::new(), |mut output, byte| {
        std::fmt::Write::write_fmt(&mut output, format_args!("{byte:02x}"))
            .unwrap_or_else(|error| panic!("{error:?}"));
        output
    });
    fs::write(
        root.join("gateway-key"),
        format!("cli-daemon:lxp_live_{secret}"),
    )
    .unwrap_or_else(|error| panic!("{}: {error:?}", "gateway credential"));
    fs::set_permissions(root.join("gateway-key"), fs::Permissions::from_mode(0o600))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "protected gateway credential"));
}

struct Gateway {
    listener: Option<TcpListener>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
    port: u16,
}

impl Gateway {
    fn start(_root: &Path, port: u16) -> Self {
        use rustix::net::{self, AddressFamily, SocketFlags, SocketType};
        let socket = net::socket_with(
            AddressFamily::INET,
            SocketType::STREAM,
            SocketFlags::CLOEXEC,
            None,
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "loopback listener socket"));
        net::sockopt::set_socket_reuseaddr(&socket, true)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "restartable listener"));
        net::bind(
            &socket,
            &SocketAddrV4::new(std::net::Ipv4Addr::LOCALHOST, port),
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "bind declared origin"));
        net::listen(&socket, 16)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "listen at declared origin"));
        let listener = TcpListener::from(socket);
        let bound = listener
            .local_addr()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "bound address"))
            .port();
        if port != 0 {
            assert_eq!(bound, port);
        }
        Self {
            listener: Some(listener),
            stop: Arc::new(AtomicBool::new(false)),
            worker: None,
            port: bound,
        }
    }

    fn endpoint(&self) -> String {
        format!("https://localhost:{}", self.port)
    }

    fn serve(&mut self, root: &Path, binding_path: &Path) {
        let binding = Binding::open(binding_path)
            .unwrap_or_else(|error| panic!("published binding: {}", error.detail()));
        let listener = self
            .listener
            .take()
            .unwrap_or_else(|| panic!("{}", "one Rust handler per listener"));
        let stop = Arc::clone(&self.stop);
        let expected_authorization = format!(
            "LayerX-Key {}",
            fs::read_to_string(root.join("gateway-key"))
                .unwrap_or_else(|error| panic!("{}: {error:?}", "gateway key"))
                .trim()
        );
        let tls = Arc::new(
            tls::ServerConfig::builder_with_provider(Arc::new(
                tls::crypto::ring::default_provider(),
            ))
            .with_safe_default_protocol_versions()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "TLS protocol versions"))
            .with_no_client_auth()
            .with_single_cert(
                CertificateDer::pem_slice_iter(FIXTURE_CERT.as_bytes())
                    .collect::<Result<Vec<_>, _>>()
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "fixture certificate")),
                PrivateKeyDer::from_pem_slice(FIXTURE_KEY.as_bytes())
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "fixture key")),
            )
            .unwrap_or_else(|error| panic!("{}: {error:?}", "TLS server identity")),
        );
        let store = Store::open(root.join("store"))
            .unwrap_or_else(|error| panic!("{}: {error:?}", "daemon store"));
        let mut sessions = SessionRegistry::default();
        sessions
            .restore_tenant(
                &store,
                &TenantId::new("tenant-a")
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "tenant")),
            )
            .unwrap_or_else(|error| panic!("{}: {error:?}", "daemon session recovery"));
        let document: Value = serde_json::from_slice(
            &fs::read(binding_path)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "published binding")),
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "binding JSON"));
        let limit = &document["limit"];
        let budgets = BudgetLimiter::new(vec![layerx_agentd::budget::LimitConfig {
            id: layerx_agentd::budget::LimitId([0x0a; 16]),
            name: limit["name"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "limit name"))
                .into(),
            scope: layerx_agentd::budget::LimitScope::Tenant([1; 32]),
            ceiling: 1_000,
            consumed: 0,
        }])
        .unwrap_or_else(|error| panic!("{}: {error:?}", "daemon limits"));
        let control = SessionControl::new(
            Arc::new(Mutex::new(store)),
            sessions,
            Arc::new(PreparationLifecycle::default()),
            Arc::new(budgets),
        );
        let worker = thread::spawn(move || {
            gateway_worker(
                &listener,
                &stop,
                &expected_authorization,
                &tls,
                &binding,
                &control,
            );
        });
        self.worker = Some(worker);
    }
}

fn read_gateway_request(
    socket: TcpStream,
    tls: &Arc<tls::ServerConfig>,
) -> (
    BufReader<tls::StreamOwned<tls::ServerConnection, TcpStream>>,
    Vec<u8>,
    Option<String>,
) {
    socket
        .set_read_timeout(Some(Duration::from_secs(5)))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "request deadline"));
    socket
        .set_write_timeout(Some(Duration::from_secs(5)))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "response deadline"));
    let connection = tls::ServerConnection::new(Arc::clone(tls))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "TLS connection"));
    let mut reader = BufReader::new(tls::StreamOwned::new(connection, socket));
    let mut line = String::new();
    reader
        .read_line(&mut line)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "HTTP request line"));
    assert_eq!(line, "POST /v1/agent/rpc HTTP/1.1\r\n");
    let mut length = None;
    let mut authorization = None;
    let mut header_bytes = line.len();
    loop {
        line.clear();
        assert!(
            reader
                .read_line(&mut line)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "HTTP header"))
                > 0
        );
        header_bytes += line.len();
        assert!(header_bytes <= 32_768);
        if line == "\r\n" {
            break;
        }
        let (name, value) = line
            .split_once(':')
            .unwrap_or_else(|| panic!("{}", "header field"));
        if name.eq_ignore_ascii_case("content-length") {
            assert!(length.is_none());
            length = Some(
                value
                    .trim()
                    .parse::<usize>()
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "request length")),
            );
        } else if name.eq_ignore_ascii_case("authorization") {
            assert!(authorization.is_none());
            authorization = Some(value.trim().to_owned());
        }
    }
    let length = length.unwrap_or_else(|| panic!("{}", "bounded SDK request"));
    assert!(length > 0 && length <= agent_rpc::MAX_BODY_BYTES);
    let mut bytes = vec![0; length];
    reader
        .read_exact(&mut bytes)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "SDK request body"));
    (reader, bytes, authorization)
}

fn owner_environment() -> OwnerEnvironment {
    let activities = [1, 5, 6, 7, 10].map(|ordinal| {
        ActivityType::new(ModuleId::Asset, ordinal)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "native activity"))
    });
    let registry = ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Asset, &activities)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "native module"))])
    .unwrap_or_else(|error| panic!("{}: {error:?}", "native registry"));
    OwnerEnvironment {
        registry,
        core_time_ms: 120,
        head_sequence: OBSERVED_HEIGHT,
        native_effect_profile: true,
        native_send_profile: true,
    }
}

fn invocation_digest(tool: &str, arguments: &Value) -> [u8; 32] {
    Sha256::new()
        .chain_update(b"LXP/mcp/daemon-invocation/v1\0")
        .chain_update((tool.len() as u64).to_be_bytes())
        .chain_update(tool.as_bytes())
        .chain_update(
            serde_json::to_vec(arguments)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "arguments")),
        )
        .finalize()
        .into()
}

fn gateway_worker(
    listener: &TcpListener,
    stop: &AtomicBool,
    expected_authorization: &str,
    tls: &Arc<tls::ServerConfig>,
    binding: &Binding,
    control: &SessionControl,
) {
    loop {
        let (socket, _) = listener
            .accept()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "loopback connection"));
        if stop.load(Ordering::Acquire) {
            break;
        }
        let (mut reader, bytes, authorization) = read_gateway_request(socket, tls);
        let writer = reader.get_mut();
        if authorization.as_deref() != Some(expected_authorization) {
            write_response(
                writer,
                &agent_rpc::refusal(
                    401,
                    layerx_sdk::programs::AgentErrorClass::PolicyRefusal,
                    "gateway.authentication",
                ),
            );
            continue;
        }
        let invocation = agent_rpc::decode_mcp_invocation(&bytes)
            .unwrap_or_else(|_| panic!("SDK envelope must decode"))
            .unwrap_or_else(|| panic!("{}", "MCP operation"));
        assert!(binding.matches_session(&invocation.credential));
        let tool = if invocation.tool == "mcp.describe" {
            "mcp.describe"
        } else {
            catalogue()
                .iter()
                .find(|tool| tool.name == invocation.tool)
                .unwrap_or_else(|| panic!("{}", "catalogued invocation"))
                .name
        };
        let digest = invocation_digest(tool, &invocation.arguments);
        let identity = McpInvocationIdentity {
            tool_name: tool,
            idempotency_key: invocation.idempotency_key,
            request_digest: digest,
        };
        let environment = owner_environment();
        let result = binding
            .daemon_invocation(
                control.clone(),
                invocation.credential,
                OBSERVED_HEIGHT,
                tool,
                &invocation.arguments,
                identity,
                Some(environment),
                |_, _, _, _| {
                    unreachable!(
                        "catalogue and balance reads must use the daemon's own read boundary"
                    )
                },
            )
            .unwrap_or_else(|error| panic!("real daemon authorization: {error:?}"));
        result
            .release_current(OBSERVED_HEIGHT, |value| {
                let value = value.unwrap_or_else(|error| panic!("real daemon read: {error:?}"));
                let response = agent_rpc::mcp_result_response(invocation.request_id, value);
                assert_eq!(response.status, 200);
                write_response(writer, &response);
                Ok(())
            })
            .unwrap_or_else(|error| panic!("daemon durable release: {error:?}"));
    }
}

fn write_response(
    writer: &mut tls::StreamOwned<tls::ServerConnection, TcpStream>,
    response: &agent_rpc::AgentRpcResponse,
) {
    let status = response.status;
    let reason = if status == 200 { "OK" } else { "Unauthorized" };
    write!(writer, "HTTP/1.1 {status} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.body.len()).unwrap_or_else(|error| panic!("{}: {error:?}", "HTTP response headers"));
    writer
        .write_all(&response.body)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "daemon response bytes"));
    writer.conn.send_close_notify();
    writer
        .flush()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "daemon response release"));
}

impl Drop for Gateway {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = TcpStream::connect((std::net::Ipv4Addr::LOCALHOST, self.port));
            worker
                .join()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "daemon handler"));
        }
    }
}

fn layerx(root: &Path) -> Command {
    let mut command = Command::new(env!("CARGO_BIN_EXE_layerx"));
    command
        .env("LAYERX_CONFIG", root.join("config.json"))
        .env("LAYERX_INSTALL_ROOT", root)
        .env_remove("LAYERX_CREDENTIAL_STORE")
        .env_remove("LAYERX_GATEWAY_KEY_ID");
    command
}

fn installed(root: &Path, read_only: bool) -> Value {
    let mut command = layerx(root);
    command.args(["--json", "install", "mcp", "--host", "layerx"]);
    if read_only {
        command.arg("--read-only");
    }
    let output = command
        .output()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "install process"));
    let value = success(&output);
    assert_eq!(value["ok"], true);
    assert_eq!(value["kind"], "install.mcp");
    assert_eq!(value["data"]["authorization"], "agent-daemon");
    value["data"].clone()
}

fn success(output: &Output) -> Value {
    assert!(
        output.status.success(),
        "stderr: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    serde_json::from_slice(&output.stdout)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "CLI JSON"))
}

fn served(root: &Path, installation: &Value) {
    let registration: Value = serde_json::from_slice(
        &fs::read(root.join("mcp.json"))
            .unwrap_or_else(|error| panic!("{}: {error:?}", "registration")),
    )
    .unwrap_or_else(|error| panic!("{}: {error:?}", "registration JSON"));
    let entry = &registration["mcpServers"]["layerx"];
    assert_eq!(entry["args"], installation["server"]["args"]);
    assert_eq!(entry["command"], installation["server"]["command"]);
    let mut command = layerx(root);
    command.args(
        entry["args"]
            .as_array()
            .unwrap_or_else(|| panic!("{}", "launch arguments"))
            .iter()
            .map(|arg| {
                arg.as_str()
                    .unwrap_or_else(|| panic!("{}", "text argument"))
            }),
    );
    let mut child = command
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "installed MCP server"));
    {
        let mut input = child
            .stdin
            .take()
            .unwrap_or_else(|| panic!("{}", "stdio input"));
        writeln!(
            input,
            "{{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\"}}"
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "initialize"));
        writeln!(
            input,
            "{{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}}"
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "initialized"));
        writeln!(
            input,
            "{{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\"}}"
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "tools/list"));
    }
    let output = child
        .wait_with_output()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "MCP exit"));
    assert!(
        output.status.success(),
        "MCP stderr: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    let responses: Vec<Value> = String::from_utf8(output.stdout)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "stdio UTF8"))
        .lines()
        .map(|line| {
            serde_json::from_str(line)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "MCP response"))
        })
        .collect();
    assert_eq!(responses.len(), 2);
    assert_eq!(
        responses[0]["result"]["_meta"]["layerx/binding"],
        "agent-daemon"
    );
    assert_eq!(
        responses[0]["result"]["_meta"]["layerx/deployment_mode"],
        installation["deployment_mode"]
    );
    let names = |tools: &Value| {
        tools
            .as_array()
            .unwrap_or_else(|| panic!("{}", "tool catalogue"))
            .iter()
            .map(|tool| {
                tool["name"]
                    .as_str()
                    .unwrap_or_else(|| panic!("{}", "tool name"))
                    .to_owned()
            })
            .collect::<BTreeSet<_>>()
    };
    let expected: BTreeSet<_> = catalogue()
        .iter()
        .filter(|tool| installation["deployment_mode"] == "full" || tool.kind == ToolKind::Read)
        .map(|tool| tool.name.to_owned())
        .collect();
    assert_eq!(names(&responses[1]["result"]["tools"]), expected);
    assert_eq!(names(&installation["tools"]), expected);
}

fn gateway_authentication_required(root: &Path, endpoint: &str, binding_path: &Path) {
    let binding = Binding::open(binding_path)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "unchanged binding"));
    let session = binding
        .daemon_credential()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "real enrolled credential"));
    let credential = EnvelopeCredential::new(
        session.tenant().as_str(),
        session.session_id().0,
        session.token_id(),
        session.generation(),
    )
    .unwrap_or_else(|error| panic!("{}: {error:?}", "session envelope credential"));
    let transport = AgentEnvelopeTransport::connect(endpoint, None, Some(&root.join("ca.pem")))
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "TLS transport without a gateway credential"
            )
        });
    let Err(refusal) = DaemonClientSession::connect(transport, credential, binding.mode()) else {
        panic!("a session token must not replace gateway authentication");
    };
    assert_eq!(refusal["reason"], "gateway.authentication");
    assert_eq!(refusal["state"], "refused");
}

fn assert_enrolment_shape(
    root: &Path,
    endpoint: &str,
    published: &PublishedBinding,
    request: &EnrolmentRequest,
    original: &[u8],
) -> (Value, enrolment::binding_shape::BindingDocument) {
    let document: Value = serde_json::from_slice(original)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "genuine JSON"));
    let shape: enrolment::binding_shape::BindingDocument = serde_json::from_slice(original)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "shared persisted shape"));
    assert_eq!(
        serde_json::to_value(&shape)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "shared shape encoding")),
        document
    );
    assert_eq!(document["core_height"], request.core_sequence);
    assert!(document.get("core_sequence").is_none());
    assert_eq!(document["agent"]["readiness"]["endpoint"], endpoint);
    assert_eq!(
        document["agent"]["readiness"]["gateway_key_file"],
        root.join("gateway-key")
            .to_str()
            .unwrap_or_else(|| panic!("{}", "key path"))
    );
    assert_eq!(
        document["agent"]["readiness"]["trust_anchors"],
        root.join("ca.pem")
            .to_str()
            .unwrap_or_else(|| panic!("{}", "CA path"))
    );
    assert_eq!(
        Binding::open(&published.binding)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "consumer schema"))
            .core_height(),
        50
    );
    (document, shape)
}

#[test]
fn mcp_enrollment_shape_installs_and_serves_unchanged_before_and_after_restart() {
    let directory = tempfile::Builder::new()
        .prefix("mcp-enrolment-")
        .tempdir()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "private test directory"));
    let root = fs::canonicalize(directory.path())
        .unwrap_or_else(|error| panic!("{}: {error:?}", "canonical directory"));
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "private directory"));
    transport_files(&root);
    let mut gateway = Gateway::start(&root, 0);
    let endpoint = gateway.endpoint();
    let port = gateway.port;
    let (published, binding_publisher_handle, identity, request) = enrol(&root, &endpoint);
    let original = fs::read(&published.binding)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "genuine enrollment document"));
    let token = fs::read(&published.session_token_file)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "session token"));
    let bearer = fs::read(&published.daemon_bearer_file)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "daemon bearer"));
    let (document, shape) =
        assert_enrolment_shape(&root, &endpoint, &published, &request, &original);
    gateway.serve(&root, &published.binding);
    gateway_authentication_required(&root, &endpoint, &published.binding);
    let first = installed(&root, false);
    assert_eq!(first["changed"], true);
    assert_eq!(
        first["daemon_binding"]["session_generation"],
        published.session_generation
    );
    assert_eq!(first["daemon_binding"]["core_height"], 50);
    assert_eq!(
        first["daemon_binding"]["readiness"],
        document["agent"]["readiness"]
    );
    served(&root, &first);
    drop(gateway);
    // Reopen both durable session authority and its network listener at the declared origin.
    drop(binding_publisher_handle);
    let binding_publisher_handle = binding_publisher(&root, &endpoint);
    let mut gateway = Gateway::start(&root, port);
    assert_eq!(gateway.endpoint(), endpoint);
    let store = Store::open(root.join("store"))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "reopen durable store"));
    let mut sessions = SessionRegistry::default();
    sessions
        .restore_tenant(&store, identity.tenant())
        .unwrap_or_else(|error| panic!("{}: {error:?}", "recover enrolled session"));
    binding_publisher_handle
        .validate_existing(&sessions, &identity, &request)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "validate recovered publisher state"));
    assert_eq!(
        sessions
            .get(identity.tenant(), published.session_id)
            .unwrap_or_else(|| panic!("{}", "recovered session"))
            .generation,
        published.session_generation
    );
    drop(store);
    gateway.serve(&root, &published.binding);
    gateway_authentication_required(&root, &endpoint, &published.binding);
    let reopened = installed(&root, false);
    assert_eq!(reopened["changed"], false);
    assert_eq!(reopened["tools"], first["tools"]);
    assert_eq!(reopened["daemon_binding"], first["daemon_binding"]);
    let recovered_shape: enrolment::binding_shape::BindingDocument = serde_json::from_slice(
        &fs::read(&published.binding)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "recovered shape")),
    )
    .unwrap_or_else(|error| panic!("{}: {error:?}", "shared recovered shape"));
    assert_eq!(recovered_shape, shape);
    served(&root, &reopened);
    let narrowed = installed(&root, true);
    assert_eq!(narrowed["deployment_mode"], "read-only");
    served(&root, &narrowed);
    assert_eq!(
        fs::read(&published.binding)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "reopened binding")),
        original
    );
    assert_eq!(
        fs::read(&published.session_token_file)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "reopened token")),
        token
    );
    assert_eq!(
        fs::read(&published.daemon_bearer_file)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "reopened bearer")),
        bearer
    );
    let registration = fs::read(root.join("mcp.json"))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "registration bytes"));
    drop(gateway);
    assert_install_unavailable(&root, &registration);
}

fn assert_install_unavailable(root: &Path, registration: &[u8]) {
    let unavailable = layerx(root)
        .args(["--json", "install", "mcp", "--host", "layerx"])
        .output()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "unavailable install"));
    assert!(!unavailable.status.success());
    assert_eq!(
        fs::read(root.join("mcp.json"))
            .unwrap_or_else(|error| panic!("{}: {error:?}", "unchanged registration")),
        registration
    );
}

#[test]
fn mcp_enrollment_shape_readiness_requires_https_and_absolute_protected_file_references() {
    let root = std::env::temp_dir();
    for endpoint in [
        "http://localhost:443",
        "https://user:password@localhost",
        "https://localhost/?query=1",
        "https://localhost/#fragment",
        "https://localhost/path",
        "https://localhost/\n",
    ] {
        assert!(
            ReadinessDeclaration::new(endpoint, root.join("gateway-key"), None).is_err(),
            "refuse {endpoint:?}"
        );
    }
    assert!(ReadinessDeclaration::new("https://localhost", "relative-key".into(), None).is_err());
    assert!(ReadinessDeclaration::new(
        "https://localhost",
        root.join("gateway-key"),
        Some("relative-ca".into())
    )
    .is_err());
    assert!(ReadinessDeclaration::new("https://localhost", root.join("gateway-key"), None).is_ok());
}
