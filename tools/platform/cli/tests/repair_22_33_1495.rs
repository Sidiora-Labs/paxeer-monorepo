//! Task 22.33.1495: production consumers over bounded, committed protocol peers.
#![allow(dead_code)]
use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::{SocketAddrV4, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt as _;
use std::os::unix::net::UnixListener;
use std::path::Path;
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
use layerx_mcp::server::catalogue;
use layerx_mcp::stdio::DaemonClientSession;
use layerx_platform_cli::{config, credential, encoding, http, rpc};
use layerx_sdk::agent_envelope::{AgentEnvelopeTransport, EnvelopeCredential};
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};

mod output {
    pub type CommandOutput = layerx_platform_cli::output::CommandOutput;
}

#[path = "../src/faucet.rs"]
mod faucet;
mod toolset {
    include!("../src/toolset.rs");
    pub fn runtime(endpoint: &str) -> Result<Runtime, String> {
        Ok(Runtime {
            client: crate::http::Client::new(endpoint, None)?,
            network_id: 125,
            environment: "test".into(),
            endpoint: endpoint.into(),
        })
    }
}

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
            .unwrap_or_else(panic_error);
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
    submission: Option<Arc<Mutex<SubmissionOwner>>>,
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
            submission: None,
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
        let submission = self.submission.clone();
        let worker = thread::spawn(move || {
            gateway_worker(
                &listener,
                &stop,
                &expected_authorization,
                &tls,
                &binding,
                &control,
                submission.as_ref(),
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
    submission: Option<&Arc<Mutex<SubmissionOwner>>>,
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
                |operation, credential, arguments, _| {
                    let owner = submission
                        .unwrap_or_else(|| panic!("{}", "real submission owner configured"));
                    let mut owner = owner
                        .lock()
                        .unwrap_or_else(|error| panic!("{}: {error:?}", "submission owner"));
                    assert_eq!(credential.tenant().as_str(), "tenant-a");
                    assert_eq!(operation, layerx_agentd::tenant::Operation::Submit);
                    let SubmissionOwner {
                        store,
                        outbox,
                        node,
                    } = &mut *owner;
                    layerx_agentd::agent_rpc_dispatch::dispatch_raw_submit(
                        arguments
                            .as_object()
                            .unwrap_or_else(|| panic!("{}", "submit arguments")),
                        invocation.request_id,
                        |raw| {
                            layerx_agentd::human_runtime::submit_raw_write(
                                outbox,
                                store,
                                TenantId::new("tenant-a")
                                    .unwrap_or_else(|error| panic!("{}: {error:?}", "tenant")),
                                raw,
                                node,
                            )
                        },
                    )
                    .map(|(value, _)| value)
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
            let result = worker.join();
            if !thread::panicking() {
                result.unwrap_or_else(|error| panic!("{}: {error:?}", "daemon handler"));
            }
        }
    }
}

// A Secret Service wire peer. The native zbus/keyring backend and its DH/AES codec
// run unchanged. The peer retains opaque encrypted secret tuples in memory.
#[derive(Clone, Debug, PartialEq)]
enum BusValue {
    Byte(u8),
    Uint(u32),
    Bool(bool),
    Text(String),
    Path(String),
    Signature(String),
    Bytes(Vec<u8>),
    Array(String, Vec<BusValue>),
    Tuple(Vec<BusValue>),
    Variant(Box<BusValue>),
}
impl BusValue {
    fn text(&self) -> &str {
        match self {
            Self::Text(text) | Self::Path(text) | Self::Signature(text) => text,
            _ => panic!("expected text: {self:?}"),
        }
    }
    fn values(&self) -> &[Self] {
        match self {
            Self::Array(_, values) | Self::Tuple(values) => values,
            _ => panic!("expected array: {self:?}"),
        }
    }
    fn inner(&self) -> &Self {
        match self {
            Self::Variant(value) => value,
            _ => panic!("expected variant: {self:?}"),
        }
    }
    fn signature(&self) -> String {
        match self {
            Self::Byte(_) => "y".into(),
            Self::Uint(_) => "u".into(),
            Self::Bool(_) => "b".into(),
            Self::Text(_) => "s".into(),
            Self::Path(_) => "o".into(),
            Self::Signature(_) => "g".into(),
            Self::Bytes(_) => "ay".into(),
            Self::Array(kind, _) => format!("a{kind}"),
            Self::Tuple(values) => format!(
                "({})",
                values.iter().map(Self::signature).collect::<String>()
            ),
            Self::Variant(_) => "v".into(),
        }
    }
    fn encode(&self, out: &mut Vec<u8>) {
        match self {
            Self::Byte(value) => out.push(*value),
            Self::Uint(value) => {
                pad(out, 4);
                out.extend_from_slice(&value.to_le_bytes());
            }
            Self::Bool(value) => Self::Uint(u32::from(*value)).encode(out),
            Self::Text(text) | Self::Path(text) => {
                pad(out, 4);
                out.extend_from_slice(
                    &u32::try_from(text.len())
                        .unwrap_or_else(panic_error)
                        .to_le_bytes(),
                );
                out.extend_from_slice(text.as_bytes());
                out.push(0);
            }
            Self::Signature(text) => {
                out.push(u8::try_from(text.len()).unwrap_or_else(panic_error));
                out.extend_from_slice(text.as_bytes());
                out.push(0);
            }
            Self::Bytes(bytes) => {
                pad(out, 4);
                out.extend_from_slice(
                    &u32::try_from(bytes.len())
                        .unwrap_or_else(panic_error)
                        .to_le_bytes(),
                );
                out.extend_from_slice(bytes);
            }
            Self::Tuple(values) => {
                pad(out, 8);
                for value in values {
                    value.encode(out);
                }
            }
            Self::Variant(value) => {
                Self::Signature(value.signature()).encode(out);
                value.encode(out);
            }
            Self::Array(kind, values) => {
                pad(out, 4);
                let size = out.len();
                out.extend_from_slice(&[0; 4]);
                pad(out, alignment(kind.as_bytes()[0]));
                let start = out.len();
                for value in values {
                    value.encode(out);
                }
                let length = u32::try_from(out.len() - start).unwrap_or_else(panic_error);
                out[size..size + 4].copy_from_slice(&length.to_le_bytes());
            }
        }
    }
}
fn pad(out: &mut Vec<u8>, align: usize) {
    while !out.len().is_multiple_of(align) {
        out.push(0);
    }
}
fn aligned(offset: usize, align: usize) -> usize {
    offset.div_ceil(align) * align
}
fn alignment(kind: u8) -> usize {
    match kind {
        b'y' | b'g' | b'v' => 1,
        b'(' | b'{' => 8,
        _ => 4,
    }
}
fn type_end(sig: &[u8], pos: usize) -> usize {
    match sig[pos] {
        b'a' => type_end(sig, pos + 1),
        b'(' | b'{' => {
            let close = if sig[pos] == b'(' { b')' } else { b'}' };
            let mut end = pos + 1;
            while sig[end] != close {
                end = type_end(sig, end);
            }
            end + 1
        }
        _ => pos + 1,
    }
}
fn parse_value(sig: &[u8], pos: &mut usize, data: &[u8], offset: &mut usize) -> BusValue {
    let kind = sig[*pos];
    *pos += 1;
    *offset = aligned(*offset, alignment(kind));
    match kind {
        b'y' => {
            let value = data[*offset];
            *offset += 1;
            BusValue::Byte(value)
        }
        b'u' | b'b' => {
            let value = u32::from_le_bytes(
                data[*offset..*offset + 4]
                    .try_into()
                    .unwrap_or_else(panic_error),
            );
            *offset += 4;
            if kind == b'u' {
                BusValue::Uint(value)
            } else {
                BusValue::Bool(value != 0)
            }
        }
        b's' | b'o' | b'g' => {
            let length = if kind == b'g' {
                let length = usize::from(data[*offset]);
                *offset += 1;
                length
            } else {
                let length = u32::from_le_bytes(
                    data[*offset..*offset + 4]
                        .try_into()
                        .unwrap_or_else(panic_error),
                ) as usize;
                *offset += 4;
                length
            };
            let text = String::from_utf8(data[*offset..*offset + length].to_vec())
                .unwrap_or_else(panic_error);
            *offset += length + 1;
            match kind {
                b's' => BusValue::Text(text),
                b'o' => BusValue::Path(text),
                _ => BusValue::Signature(text),
            }
        }
        b'v' => {
            let text = parse_value(b"g", &mut 0, data, offset);
            BusValue::Variant(Box::new(parse_value(
                text.text().as_bytes(),
                &mut 0,
                data,
                offset,
            )))
        }
        b'(' | b'{' => {
            let end = if kind == b'(' { b')' } else { b'}' };
            let mut values = Vec::new();
            while sig[*pos] != end {
                values.push(parse_value(sig, pos, data, offset));
            }
            *pos += 1;
            BusValue::Tuple(values)
        }
        b'a' => {
            let length = u32::from_le_bytes(
                data[*offset..*offset + 4]
                    .try_into()
                    .unwrap_or_else(panic_error),
            ) as usize;
            *offset += 4;
            let start = *pos;
            let end = type_end(sig, start);
            *pos = end;
            *offset = aligned(*offset, alignment(sig[start]));
            let limit = *offset + length;
            if sig[start] == b'y' {
                let bytes = data[*offset..limit].to_vec();
                *offset = limit;
                return BusValue::Bytes(bytes);
            }
            let mut values = Vec::new();
            while *offset < limit {
                values.push(parse_value(sig, &mut { start }, data, offset));
            }
            assert_eq!(*offset, limit);
            BusValue::Array(
                std::str::from_utf8(&sig[start..end])
                    .unwrap_or_else(panic_error)
                    .to_owned(),
                values,
            )
        }
        _ => panic!("unhandled D-Bus signature {}", String::from_utf8_lossy(sig)),
    }
}
fn bus_text(value: &str) -> BusValue {
    BusValue::Text(value.into())
}
fn bus_path(value: &str) -> BusValue {
    BusValue::Path(value.into())
}
fn variant(value: BusValue) -> BusValue {
    BusValue::Variant(Box::new(value))
}
fn dict(rows: Vec<(&str, BusValue)>) -> BusValue {
    BusValue::Array(
        "{sv}".into(),
        rows.into_iter()
            .map(|(key, value)| BusValue::Tuple(vec![bus_text(key), variant(value)]))
            .collect(),
    )
}
fn lookup<'a>(value: &'a BusValue, key: &str) -> &'a BusValue {
    value
        .values()
        .iter()
        .map(BusValue::values)
        .find(|row| row[0].text() == key)
        .unwrap_or_else(panic_missing)[1]
        .inner()
}
fn reply_bus(
    stream: &mut std::os::unix::net::UnixStream,
    reply: u32,
    serial: u32,
    values: Vec<BusValue>,
) {
    let signature = values.iter().map(BusValue::signature).collect::<String>();
    let mut headers = Vec::new();
    for (code, value) in [
        (5, BusValue::Uint(reply)),
        (6, bus_text(":1.1")),
        (7, bus_text(":1.0")),
        (8, BusValue::Signature(signature)),
    ] {
        BusValue::Tuple(vec![BusValue::Byte(code), variant(value)]).encode(&mut headers);
    }
    let mut body = Vec::new();
    for value in values {
        value.encode(&mut body);
    }
    let mut out = vec![b'l', 2, 0, 1];
    out.extend_from_slice(
        &u32::try_from(body.len())
            .unwrap_or_else(panic_error)
            .to_le_bytes(),
    );
    out.extend_from_slice(&serial.to_le_bytes());
    out.extend_from_slice(
        &u32::try_from(headers.len())
            .unwrap_or_else(panic_error)
            .to_le_bytes(),
    );
    out.extend(headers);
    pad(&mut out, 8);
    out.extend(body);
    stream.write_all(&out).unwrap_or_else(panic_error);
}
struct SecretBus {
    _directory: tempfile::TempDir,
    path: std::path::PathBuf,
    stop: Arc<AtomicBool>,
    thread: Option<std::thread::JoinHandle<()>>,
}
impl SecretBus {
    fn start() -> Self {
        let directory = tempfile::tempdir().unwrap_or_else(panic_error);
        let path = directory.path().join("bus");
        let listener = UnixListener::bind(&path).unwrap_or_else(panic_error);
        listener.set_nonblocking(true).unwrap_or_else(panic_error);
        let stop = Arc::new(AtomicBool::new(false));
        let closing = stop.clone();
        let thread = std::thread::spawn(move || secret_bus_worker(&listener, &closing));
        Self {
            _directory: directory,
            path,
            stop,
            thread: Some(thread),
        }
    }
}
fn authenticate_secret_bus(
    listener: &std::os::unix::net::UnixListener,
    closing: &AtomicBool,
) -> Option<std::os::unix::net::UnixStream> {
    let mut stream = loop {
        if closing.load(Ordering::Acquire) {
            return None;
        }
        match listener.accept() {
            Ok((stream, _)) => break stream,
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                std::thread::sleep(Duration::from_millis(1));
            }
            Err(error) => panic!("{error}"),
        }
    };
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .unwrap_or_else(panic_error);
    let mut byte = [0];
    stream.read_exact(&mut byte).unwrap_or_else(panic_error);
    assert_eq!(byte, [0]);
    loop {
        let mut line = Vec::new();
        while !line.ends_with(b"\r\n") {
            stream.read_exact(&mut byte).unwrap_or_else(panic_error);
            line.push(byte[0]);
        }
        let line = std::str::from_utf8(&line).unwrap_or_else(panic_error);
        if line.starts_with("AUTH ") {
            stream
                .write_all(b"OK 0123456789abcdef0123456789abcdef\r\n")
                .unwrap_or_else(panic_error);
        } else if line.starts_with("NEGOTIATE_UNIX_FD") {
            stream
                .write_all(b"ERROR Unix FD passing is not supported\r\n")
                .unwrap_or_else(panic_error);
        } else if line == "BEGIN\r\n" {
            break;
        } else {
            panic!("unexpected SASL: {line}");
        }
    }
    stream
        .set_read_timeout(Some(Duration::from_millis(100)))
        .unwrap_or_else(panic_error);
    Some(stream)
}

fn answer_secret_bus(
    method: &str,
    object: &str,
    args: &[BusValue],
    items: &mut std::collections::BTreeMap<String, (BusValue, BusValue)>,
    item_id: &mut u64,
) -> Vec<BusValue> {
    match method {
        "Hello" => vec![bus_text(":1.1")],
        "GetNameOwner" => vec![bus_text(":1.0")],
        "NameHasOwner" => vec![BusValue::Bool(true)],
        "AddMatch" | "RemoveMatch" | "Close" => vec![],
        "OpenSession" => {
            assert_eq!(args[0].text(), "dh-ietf1024-sha256-aes128-cbc-pkcs7");
            vec![
                variant(BusValue::Bytes(vec![2])),
                bus_path("/org/freedesktop/secrets/session/s1"),
            ]
        }
        "ReadAlias" => vec![bus_path("/org/freedesktop/secrets/collection/default")],
        "SearchItems" => {
            let found = items
                .iter()
                .filter(|(_, (attrs, _))| {
                    args[0]
                        .values()
                        .iter()
                        .all(|query| attrs.values().contains(query))
                })
                .map(|(path, _)| bus_path(path))
                .collect();
            vec![
                BusValue::Array("o".into(), found),
                BusValue::Array("o".into(), vec![]),
            ]
        }
        "Get" => {
            assert_eq!(args[1].text(), "Locked");
            vec![variant(BusValue::Bool(false))]
        }
        "GetAll" => vec![dict(vec![("Locked", BusValue::Bool(false))])],
        "CreateItem" => {
            let attrs = lookup(&args[0], "org.freedesktop.Secret.Item.Attributes").clone();
            let path = items
                .iter()
                .find(|(_, (existing, _))| existing == &attrs)
                .map_or_else(
                    || {
                        let path = format!("/org/freedesktop/secrets/item/i{item_id}");
                        *item_id += 1;
                        path
                    },
                    |(path, _)| path.clone(),
                );
            items.insert(path.clone(), (attrs, args[1].clone()));
            vec![bus_path(&path), bus_path("/")]
        }
        "GetSecret" => vec![items
            .get(object)
            .unwrap_or_else(|| panic!("{}", "existing Secret Service item"))
            .1
            .clone()],
        "SetSecret" => {
            items.get_mut(object).unwrap_or_else(panic_missing).1 = args[0].clone();
            vec![]
        }
        "Delete" => {
            assert!(items.remove(object).is_some());
            vec![bus_path("/")]
        }
        method => {
            panic!("unhandled Secret Service operation {method}({args:?}) on {object}")
        }
    }
}

fn secret_bus_worker(listener: &UnixListener, closing: &AtomicBool) {
    let Some(mut stream) = authenticate_secret_bus(listener, closing) else {
        return;
    };
    // path -> (attributes, encrypted Secret tuple)
    let mut items = std::collections::BTreeMap::<String, (BusValue, BusValue)>::new();
    let mut serial = 1;
    let mut item_id = 1;
    loop {
        let mut header = [0; 16];
        let mut got = 0;
        while got < header.len() {
            match stream.read(&mut header[got..]) {
                Ok(0) => return,
                Ok(count) => got += count,
                Err(error)
                    if matches!(
                        error.kind(),
                        std::io::ErrorKind::WouldBlock | std::io::ErrorKind::TimedOut
                    ) =>
                {
                    if closing.load(Ordering::Acquire) {
                        return;
                    }
                }
                Err(error) => panic!("bus header: {error}"),
            }
        }
        assert_eq!(header[0], b'l');
        let size = u32::from_le_bytes(header[4..8].try_into().unwrap_or_else(panic_error)) as usize;
        let reply = u32::from_le_bytes(header[8..12].try_into().unwrap_or_else(panic_error));
        let fields =
            u32::from_le_bytes(header[12..16].try_into().unwrap_or_else(panic_error)) as usize;
        assert!(size < 1_048_576 && fields < 8192);
        let mut data = vec![0; aligned(16 + fields, 8) + size];
        data[..16].copy_from_slice(&header);
        stream
            .read_exact(&mut data[16..])
            .unwrap_or_else(panic_error);
        let mut cursor = 16;
        let mut method = String::new();
        let mut object = String::new();
        let mut signature = String::new();
        while cursor < 16 + fields {
            let field = parse_value(b"(yv)", &mut 0, &data, &mut cursor);
            let values = field.values();
            match values[0] {
                BusValue::Byte(1) => object = values[1].inner().text().into(),
                BusValue::Byte(3) => method = values[1].inner().text().into(),
                BusValue::Byte(8) => signature = values[1].inner().text().into(),
                _ => (),
            }
        }
        if header[1] != 1 {
            continue;
        }
        let mut args = Vec::new();
        let mut pos = 0;
        cursor = aligned(16 + fields, 8);
        while pos < signature.len() {
            args.push(parse_value(
                signature.as_bytes(),
                &mut pos,
                &data,
                &mut cursor,
            ));
        }
        let result = answer_secret_bus(&method, &object, &args, &mut items, &mut item_id);
        reply_bus(&mut stream, reply, serial, result);
        serial += 1;
    }
}

impl Drop for SecretBus {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(thread) = self.thread.take() {
            let result = thread.join();
            if !std::thread::panicking() {
                result.unwrap_or_else(panic_error);
            }
        }
    }
}

struct SubmissionOwner {
    store: Store,
    outbox: layerx_agentd::outbox::Outbox,
    node: layerx_agentd::node_rpc::NodeRpc,
}

impl SubmissionOwner {
    fn open(root: &Path, url: &str, recover: Option<[u8; 32]>) -> Self {
        let store = Store::open(root)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "submission owner store"));
        let mut outbox = layerx_agentd::outbox::Outbox::default();
        if let Some(hash) = recover {
            outbox
                .restore(
                    &store,
                    TenantId::new("tenant-a").unwrap_or_else(panic_error),
                    hash,
                )
                .unwrap_or_else(|error| panic!("{}: {error:?}", "recover actual submission owner"));
        }
        let node =
            layerx_agentd::node_rpc::NodeRpc::connect(&layerx_agentd::node_rpc::NodeRpcConfig {
                url: url.into(),
                chain_id: 125,
                deadline: Duration::from_secs(5),
            })
            .unwrap_or_else(|error| panic!("{}: {error:?}", "root node connection"));
        Self {
            store,
            outbox,
            node,
        }
    }
}

#[derive(Clone, Debug)]
struct WireRequest {
    path: String,
    authorization: Option<String>,
    body: Value,
}

struct HttpFixture {
    url: String,
    requests: Arc<Mutex<Vec<WireRequest>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
fn read_fixture_request(stream: TcpStream) -> (BufReader<TcpStream>, WireRequest) {
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .unwrap_or_else(panic_error);
    stream
        .set_write_timeout(Some(Duration::from_secs(5)))
        .unwrap_or_else(panic_error);
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    reader.read_line(&mut line).unwrap_or_else(panic_error);
    let fields: Vec<_> = line.split_ascii_whitespace().collect();
    assert_eq!(fields.len(), 3);
    assert_eq!(fields[0], "POST");
    let path = fields[1].to_owned();
    let mut length = None;
    let mut authorization = None;
    let mut size = line.len();
    loop {
        line.clear();
        assert!(reader.read_line(&mut line).unwrap_or_else(panic_error) > 0);
        size += line.len();
        assert!(size <= 32_768);
        if line == "\r\n" {
            break;
        }
        let (key, value) = line.split_once(':').unwrap_or_else(panic_missing);
        if key.eq_ignore_ascii_case("content-length") {
            assert!(length.is_none());
            length = Some(value.trim().parse::<usize>().unwrap_or_else(panic_error));
        } else if key.eq_ignore_ascii_case("authorization") {
            assert!(authorization.is_none());
            authorization = Some(value.trim().to_owned());
        }
    }
    let length = length
        .filter(|n| *n <= 1_048_576)
        .unwrap_or_else(|| panic!("{}", "bounded request"));
    let mut bytes = vec![0; length];
    reader.read_exact(&mut bytes).unwrap_or_else(panic_error);
    let request = WireRequest {
        path,
        authorization,
        body: serde_json::from_slice(&bytes).unwrap_or_else(panic_error),
    };
    (reader, request)
}

impl HttpFixture {
    fn start(
        mut answer: impl FnMut(&WireRequest) -> Option<(u16, Value)> + Send + 'static,
    ) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(panic_error);
        listener.set_nonblocking(true).unwrap_or_else(panic_error);
        let url = format!(
            "http://{}",
            listener.local_addr().unwrap_or_else(panic_error)
        );
        let stop = Arc::new(AtomicBool::new(false));
        let closing = stop.clone();
        let requests = Arc::new(Mutex::new(Vec::new()));
        let observed = requests.clone();
        let worker = thread::spawn(move || {
            while !closing.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(value) => value,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("loopback listener: {error}"),
                };
                let (mut reader, request) = read_fixture_request(stream);
                observed
                    .lock()
                    .unwrap_or_else(panic_error)
                    .push(request.clone());
                if let Some((status, body)) = answer(&request) {
                    let body = serde_json::to_vec(&body).unwrap_or_else(panic_error);
                    let writer = reader.get_mut();
                    write!(writer,"HTTP/1.1 {status} Fixture\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",body.len()).unwrap_or_else(panic_error);
                    writer.write_all(&body).unwrap_or_else(panic_error);
                    writer.flush().unwrap_or_else(panic_error);
                }
            }
        });
        Self {
            url,
            requests,
            stop,
            worker: Some(worker),
        }
    }
    fn requests(&self) -> Vec<WireRequest> {
        self.requests.lock().unwrap_or_else(panic_error).clone()
    }
}
impl Drop for HttpFixture {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let result = worker.join();
            if !thread::panicking() {
                result.unwrap_or_else(|error| panic!("{}: {error:?}", "bounded HTTP peer"));
            }
        }
    }
}

fn vector() -> Value {
    let fixture: Value = serde_json::from_str(include_str!(
        "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap_or_else(panic_error);
    fixture["cases"]
        .as_array()
        .unwrap_or_else(panic_missing)
        .iter()
        .find(|row| row["id"] == "V1-layerxledger-pay-exact")
        .unwrap_or_else(panic_missing)
        .clone()
}
fn raw_vector() -> Vec<u8> {
    let fixture = vector();
    let bytes = encoding::hex_decode(
        "recorded node transaction",
        fixture["rawTransaction"]
            .as_str()
            .unwrap_or_else(panic_missing)
            .strip_prefix("0x")
            .unwrap_or_else(panic_missing),
    )
    .unwrap_or_else(panic_error);
    let (call, signature) =
        layerx_crypto::evm_transaction::decode_signed(&bytes).unwrap_or_else(panic_error);
    let secret = encoding::fixed_hex::<32>(
        "conformance signing key",
        "8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63",
    )
    .unwrap_or_else(panic_error);
    let signed = call.sign(&secret).unwrap_or_else(panic_error);
    assert_eq!(
        signed, bytes,
        "production signer must reproduce the actual node vector"
    );
    let sender = layerx_crypto::evm_transaction::recover_sender(&call, &signature)
        .unwrap_or_else(panic_error);
    assert_eq!(
        format!("0x{}", encoding::hex_encode(&sender)),
        fixture["receiptJson"]["from"]
    );
    assert_eq!(
        format!(
            "0x{}",
            encoding::hex_encode(&layerx_crypto::evm_transaction::transaction_hash(&signed))
        ),
        fixture["receiptJson"]["txHash"]
    );
    signed
}
fn node_fixture(lose_submission_reply: bool) -> HttpFixture {
    let fixture = vector();
    let raw = raw_vector();
    let mut sends = 0;
    HttpFixture::start(move |request| {
        if request.path != "/" {
            return Some((404, json!({"error":"node RPC is served at root"})));
        }
        let method = request.body["method"]
            .as_str()
            .unwrap_or_else(panic_missing);
        let result = match method {
            "eth_chainId" => json!("0x7d"),
            "eth_sendRawTransaction" => {
                assert_eq!(
                    request.body["params"],
                    json!([format!("0x{}", encoding::hex_encode(&raw))])
                );
                sends += 1;
                assert_eq!(sends, 1, "one physical submission only");
                if lose_submission_reply {
                    return None;
                }
                fixture["receiptJson"]["txHash"].clone()
            }
            "pax_sequencerReceipt" => {
                assert_eq!(
                    request.body["params"],
                    json!([fixture["receiptJson"]["txHash"]])
                );
                fixture["receiptJson"].clone()
            }
            "px_getReceiptKeys" => json!({"keys":[fixture["pinnedKey"]]}),
            "eth_getTransactionReceipt" => {
                assert_eq!(
                    request.body["params"],
                    json!([fixture["receiptJson"]["txHash"]])
                );
                Value::Null
            }
            other => panic!("unexpected node method {other}"),
        };
        Some((
            200,
            json!({"jsonrpc":"2.0","id":request.body["id"],"result":result}),
        ))
    })
}

#[test]
fn test_retired_endpoint_chains() {
    use layerx_client::chains::{encode_call, ChainCall, CHAINS_PRECOMPILE};
    let node = HttpFixture::start(|request| {
        if request.path != "/" {
            return Some((404, json!({"error":"root node route required"})));
        }
        assert_eq!(request.body["method"], "eth_call");
        assert_eq!(
            request.body["params"],
            json!([{"to":format!("0x{}",encoding::hex_encode(&CHAINS_PRECOMPILE)),"data":format!("0x{}",encoding::hex_encode(&encode_call(&ChainCall::ChainCount)))},"pending"])
        );
        Some((
            200,
            json!({"jsonrpc":"2.0","id":request.body["id"],"result":format!("0x{}","00".repeat(32))}),
        ))
    });
    let client = http::Client::new(&node.url, None).unwrap_or_else(panic_error);
    let result = layerx_platform_cli::chains::list(&client, &node.url);
    assert_eq!(result, Ok(json!({"chains":[]})), "wiring request: tools/platform/cli/src/chains.rs (domain 8) must send Ethereum requests to the node root");
    assert_eq!(node.requests().len(), 1);
    assert_eq!(node.requests()[0].path, "/");
}

#[test]
fn test_retired_endpoint_lx_refusal() {
    let node = HttpFixture::start(|request| {
        assert_eq!(request.path, "/");
        assert_eq!(request.body["method"], "eth_chainId");
        Some((
            200,
            json!({"jsonrpc":"2.0","id":request.body["id"],"result":"0x7d"}),
        ))
    });
    let client = rpc::RpcClient::new(&node.url,None).unwrap_or_else(|error| panic!("{}: {error:?}", "wiring request: tools/platform/cli/src/rpc.rs (domain 8) must admit the root node endpoint"));
    assert_eq!(
        client
            .call("eth_chainId", &json!([]))
            .unwrap_or_else(panic_error),
        json!("0x7d")
    );
    let before = node.requests().len();
    for method in [
        "lx_sendActivity",
        "lx_getSequence",
        "lx_requestFunds",
        "lx_futureMethod",
    ] {
        assert_eq!(
            client
                .call(method, &json!([]))
                .err()
                .unwrap_or_else(|| panic!("expected error")),
            format!("rpc_method_unavailable: {method} is switched off")
        );
    }
    assert_eq!(
        node.requests().len(),
        before,
        "retired methods never leave the client"
    );
}

#[test]
fn test_retired_endpoint_faucet() {
    let node = node_fixture(false);
    let endpoint = layerx_client::sequencer::SequencerEndpoint::new(&node.url, &[])
        .unwrap_or_else(panic_error);
    assert_eq!(endpoint.chain_id().unwrap_or_else(panic_error), 125);
    let gateway = HttpFixture::start(|request| {
        assert_eq!(request.path, "/rpc", "explicit hosted service route");
        if request.authorization.as_deref() != Some("Bearer fixture-identity-session") {
            return Some((401, json!({"error":"identity_session_required"})));
        }
        assert_eq!(
            request.body,
            faucet::request("did:layerx:alice", &"42".repeat(32))
        );
        Some((
            200,
            json!({"jsonrpc":"2.0","id":"faucet","result":{"funded":true,"funding_id":"ef".repeat(32),"transaction_id":"cd".repeat(32),"amount":"1000000","network":"layerx-testnet"}}),
        ))
    });
    let claim = faucet::request("did:layerx:alice", &"42".repeat(32));
    assert!(http::Client::new(&gateway.url, None)
        .unwrap_or_else(panic_error)
        .post("/rpc", &claim, None)
        .err()
        .unwrap_or_else(|| panic!("expected error"))
        .contains("HTTP 401"));
    let authorized = http::Client::new(
        &gateway.url,
        Some(zeroize::Zeroizing::new("fixture-identity-session".into())),
    )
    .unwrap_or_else(panic_error);
    let response = authorized
        .post("/rpc", &claim, None)
        .unwrap_or_else(panic_error);
    let decoded = faucet::decode(&response).unwrap_or_else(panic_error);
    assert_eq!(decoded["funded"], true);
    assert_eq!(decoded["txHash"], "cd".repeat(32));
    assert!(rpc::request("lx_requestFunds", &json!([])).is_err());
    assert_eq!(gateway.requests().len(), 2);
    assert_eq!(
        node.requests().len(),
        1,
        "hosted claim never reaches the node"
    );
}

#[test]
fn test_retired_endpoint_mcp_submit() {
    let node = node_fixture(false);
    let raw = raw_vector();
    let runtime = toolset::runtime(&node.url).unwrap_or_else(panic_error);
    let tools =
        toolset::surface(layerx_mcp::server::DeploymentMode::Full).unwrap_or_else(panic_error);
    let submit = *tools
        .iter()
        .find(|tool| tool.name == "activity.submit")
        .unwrap_or_else(panic_missing);
    let result = toolset::invoke(
        &runtime,
        submit,
        &json!({"raw_transaction":format!("0x{}",encoding::hex_encode(&raw))}),
    )
    .unwrap_or_else(panic_error);
    assert_eq!(
        result,
        json!({"txHash":vector()["receiptJson"]["txHash"],"rung":"submitted"})
    );
    let receipt = *tools
        .iter()
        .find(|tool| tool.name == "receipt.get")
        .unwrap_or_else(panic_missing);
    let observed = toolset::invoke(&runtime, receipt, &json!({"activity_id":result["txHash"]}))
        .unwrap_or_else(panic_error);
    assert_eq!(observed, vector()["receiptJson"]);
    assert_eq!(
        node.requests()
            .iter()
            .map(|r| r.path.as_str())
            .collect::<Vec<_>>(),
        vec!["/", "/"]
    );
    assert!(rpc::request("lx_sendActivity", &json!([])).is_err());
}

fn stdio(binding_path: &Path, messages: &[Value]) -> Vec<Value> {
    let binding = Binding::open(binding_path)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "unchanged enrollment consumer"));
    let mut client = binding
        .open_daemon_client()
        .unwrap_or_else(|error| panic!("authenticated client: {}", error.detail()));
    let mut bytes = Vec::new();
    for message in messages {
        serde_json::to_writer(&mut bytes, message).unwrap_or_else(panic_error);
        bytes.push(b'\n');
    }
    let mut output = Vec::new();
    client
        .serve(&mut std::io::Cursor::new(bytes), &mut output)
        .unwrap_or_else(panic_error);
    String::from_utf8(output)
        .unwrap_or_else(panic_error)
        .lines()
        .map(|line| serde_json::from_str(line).unwrap_or_else(panic_error))
        .collect()
}
fn initialize() -> Value {
    json!({"jsonrpc":"2.0","id":1,"method":"initialize"})
}
fn submit_message(raw: &[u8]) -> Value {
    json!({"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"activity.submit","arguments":{"raw_transaction":format!("0x{}",encoding::hex_encode(raw))},"_meta":{"layerx/idempotency_key":"17".repeat(32)}}})
}

#[test]
fn mcp_binding_test_rewrite() {
    let directory = tempfile::tempdir().unwrap_or_else(panic_error);
    let root = directory.path();
    transport_files(root);
    let mut gateway = Gateway::start(root, 0);
    let port = gateway.port;
    let endpoint = gateway.endpoint();
    let (published, binding_publisher_handle, identity, request) = enrol(root, &endpoint);
    let bytes = fs::read(&published.binding).unwrap_or_else(panic_error);
    let document: Value = serde_json::from_slice(&bytes).unwrap_or_else(panic_error);
    assert_eq!(document["core_height"], 50);
    assert!(document.get("core_sequence").is_none());
    assert_eq!(document["agent"]["readiness"]["endpoint"], endpoint);
    gateway.serve(root, &published.binding);
    gateway_authentication_required(root, &endpoint, &published.binding);
    let responses = stdio(
        &published.binding,
        &[
            initialize(),
            json!({"jsonrpc":"2.0","id":2,"method":"tools/list"}),
        ],
    );
    assert_eq!(
        responses[0]["result"]["_meta"]["layerx/binding"],
        "agent-daemon"
    );
    let first = responses[1]["result"]["tools"].clone();
    assert!(!first.as_array().unwrap_or_else(panic_missing).is_empty());
    drop(gateway);
    drop(binding_publisher_handle);
    let store = Store::open(root.join("store")).unwrap_or_else(panic_error);
    let mut sessions = SessionRegistry::default();
    sessions
        .restore_tenant(&store, identity.tenant())
        .unwrap_or_else(panic_error);
    let recovered_publisher = binding_publisher(root, &endpoint);
    recovered_publisher
        .validate_existing(&sessions, &identity, &request)
        .unwrap_or_else(panic_error);
    assert_eq!(
        sessions
            .get(identity.tenant(), published.session_id)
            .unwrap_or_else(panic_missing)
            .generation,
        published.session_generation
    );
    drop(store);
    let mut recovered = Gateway::start(root, port);
    recovered.serve(root, &published.binding);
    let responses = stdio(
        &published.binding,
        &[
            initialize(),
            json!({"jsonrpc":"2.0","id":2,"method":"tools/list"}),
        ],
    );
    assert_eq!(responses[1]["result"]["tools"], first);
    assert_eq!(
        fs::read(&published.binding).unwrap_or_else(panic_error),
        bytes,
        "publication never rewritten before or after restart"
    );
}

#[test]
fn mcp_submit_path_test() {
    served_submission_journey(None);
}

fn verify_recorded_submission(
    recovered_owner: &mut SubmissionOwner,
    node: &HttpFixture,
    hash: [u8; 32],
    lost: bool,
) {
    let mut sequencer = layerx_client::sequencer::SequencerEndpoint::new(&node.url, &[])
        .unwrap_or_else(panic_error);
    let keys = sequencer.receipt_keys().unwrap_or_else(panic_error);
    let looked = layerx_client::rpc::receipt::lookup_authenticated(
        &sequencer,
        hash,
        layerx_client::rpc::receipt::LookupContext {
            receipt_keys: &keys,
        },
    )
    .unwrap_or_else(panic_error);
    let layerx_client::rpc::receipt::Lookup::Verified(receipt) = looked else {
        panic!("actual recorded node receipt required")
    };
    assert_eq!(receipt.activity_id(), hash);
    assert_eq!(
        receipt.canonical_bytes(),
        encoding::hex_decode(
            "canonical receipt",
            vector()["receiptJson"]["receipt"]
                .as_str()
                .unwrap_or_else(panic_missing)
                .trim_start_matches("0x")
        )
        .unwrap_or_else(panic_error)
    );
    if lost {
        let resolution = layerx_agentd::outbox::resolve_unknown(
            &mut recovered_owner.outbox,
            &mut recovered_owner.store,
            hash,
            1000,
            &keys,
            &mut sequencer,
        )
        .unwrap_or_else(panic_error);
        assert_eq!(
            resolution.state,
            layerx_agentd::outbox::SubmissionState::Executed
        );
        assert_eq!(
            resolution.resend,
            layerx_agentd::outbox::ResendObservation::NotWarranted
        );
        assert_eq!(
            resolution.observation,
            layerx_agentd::outbox::ResolutionObservation::ExecutedReceipt
        );
    }
}

/// Exercises submission recovery through the served MCP route.
///
/// # Panics
/// Panics when transport or receipt assertions do not hold.
pub fn served_submission_journey(binary: Option<&str>) {
    for lost in [false, true] {
        let directory = tempfile::tempdir().unwrap_or_else(panic_error);
        let root = directory.path();
        transport_files(root);
        let node = node_fixture(lost);
        let raw = raw_vector();
        let hash = layerx_crypto::evm_transaction::transaction_hash(&raw);
        let mut gateway = Gateway::start(root, 0);
        let endpoint = gateway.endpoint();
        let port = gateway.port;
        let (published, binding_publisher_handle, _, _) = enrol(root, &endpoint);
        let binding_bytes = fs::read(&published.binding).unwrap_or_else(panic_error);
        let owner = Arc::new(Mutex::new(SubmissionOwner::open(
            &root.join("submission-owner"),
            &node.url,
            None,
        )));
        gateway.submission = Some(owner.clone());
        gateway.serve(root, &published.binding);
        let messages = [initialize(), submit_message(&raw)];
        let responses = served_messages(binary, root, &published.binding, &messages);
        assert_eq!(responses.len(), 2);
        assert_eq!(
            responses[1]["result"]["isError"], false,
            "served raw submission must reach the production owner: {}",
            responses[1]
        );
        let submitted = &responses[1]["result"]["structuredContent"]["result"];
        assert_eq!(
            submitted,
            &json!({"tx_hash":encoding::hex_encode(&hash),"submission_ref":encoding::hex_encode(&hash),"state":if lost {"Unknown"} else {"Submitted"},"rung":"submitted","evidence":[],"transitions":[]})
        );
        let expected = if lost {
            layerx_agentd::outbox::SubmissionState::Unknown
        } else {
            layerx_agentd::outbox::SubmissionState::Submitted
        };
        assert_eq!(
            owner
                .lock()
                .unwrap_or_else(panic_error)
                .outbox
                .status(hash)
                .unwrap_or_else(panic_missing)
                .state,
            expected
        );
        drop(gateway);
        drop(binding_publisher_handle);
        drop(owner);
        let mut recovered_owner =
            SubmissionOwner::open(&root.join("submission-owner"), &node.url, Some(hash));
        assert_eq!(
            recovered_owner
                .outbox
                .status(hash)
                .unwrap_or_else(panic_missing)
                .state,
            expected
        );
        assert_eq!(
            recovered_owner
                .outbox
                .exact_signed_bytes(hash)
                .unwrap_or_else(panic_error),
            raw
        );
        verify_recorded_submission(&mut recovered_owner, &node, hash, lost);
        let recovered_owner = Arc::new(Mutex::new(recovered_owner));
        let mut recovered_gateway = Gateway::start(root, port);
        recovered_gateway.submission = Some(recovered_owner);
        recovered_gateway.serve(root, &published.binding);
        let repeated = served_messages(binary, root, &published.binding, &messages);
        assert_eq!(
            repeated[1], responses[1],
            "durable invocation replays the retained outcome"
        );
        assert_eq!(
            node.requests()
                .iter()
                .filter(|request| request.body["method"] == "eth_sendRawTransaction")
                .count(),
            1
        );
        assert_eq!(
            fs::read(&published.binding).unwrap_or_else(panic_error),
            binding_bytes
        );
    }
}

struct EnvironmentRestore(Vec<(&'static str, Option<std::ffi::OsString>)>);
impl EnvironmentRestore {
    fn capture(names: &[&'static str]) -> Self {
        Self(
            names
                .iter()
                .map(|name| (*name, std::env::var_os(name)))
                .collect(),
        )
    }
}
impl Drop for EnvironmentRestore {
    fn drop(&mut self) {
        for (name, value) in &self.0 {
            match value {
                Some(value) => std::env::set_var(name, value),
                None => std::env::remove_var(name),
            }
        }
    }
}
#[test]
fn credential_profile_collision_gap() {
    let bus = SecretBus::start();
    let _environment = EnvironmentRestore::capture(&[
        "DBUS_SESSION_BUS_ADDRESS",
        "LAYERX_CREDENTIAL_STORE",
        "LAYERX_CONFIG",
    ]);
    std::env::set_var(
        "DBUS_SESSION_BUS_ADDRESS",
        format!("unix:path={}", bus.path.display()),
    );
    std::env::set_var("LAYERX_CREDENTIAL_STORE", "os");
    let root = tempfile::tempdir().unwrap_or_else(panic_error);
    let a = root.path().join("a.json");
    let b = root.path().join("b.json");
    let failed = root.path().join("cannot-save.json");
    assert!(
        keyring::Entry::store_status().is_ok(),
        "native OS credential backend"
    );
    let fixture =
        keyring_core::Entry::new("dev.layerx.cli", "key:same").unwrap_or_else(panic_error);
    fixture
        .set_password(&"22".repeat(32))
        .unwrap_or_else(panic_error);
    std::env::set_var("LAYERX_CONFIG", &a);
    let mut ca = config::Configuration::default();
    let ma = credential::create_key(&mut ca, "same", None).unwrap_or_else(panic_error);
    let seed_a = credential::key_seed("same").unwrap_or_else(panic_error);
    std::env::set_var("LAYERX_CONFIG", &b);
    let mut cb = config::Configuration::default();
    let mb = credential::create_key(&mut cb, "same", None).unwrap_or_else(panic_error);
    let seed_b = credential::key_seed("same").unwrap_or_else(panic_error);
    assert_ne!(ma.public_key, mb.public_key);
    assert_ne!(*seed_a, *seed_b);
    assert_eq!(
        credential::store_label(),
        "operating-system-credential-store"
    );
    fs::create_dir(&failed).unwrap_or_else(panic_error);
    std::env::set_var("LAYERX_CONFIG", &failed);
    let mut cannot_save = config::Configuration::default();
    assert!(credential::create_key(&mut cannot_save, "same", None).is_err());
    assert!(cannot_save.keys.is_empty());
    let rolled_back = keyring_core::Entry::new(
        "dev.layerx.cli",
        &credential::key_entry_identity(&failed, "same").unwrap_or_else(panic_error),
    )
    .unwrap_or_else(panic_error);
    assert!(matches!(
        rolled_back.get_password(),
        Err(keyring_core::Error::NoEntry)
    ));
    for (path, metadata, seed) in [(&a, &ma, &seed_a), (&b, &mb, &seed_b)] {
        std::env::set_var("LAYERX_CONFIG", path);
        assert_eq!(
            config::Configuration::load()
                .unwrap_or_else(panic_error)
                .keys["same"]
                .public_key,
            metadata.public_key
        );
        assert_eq!(
            *credential::key_seed("same").unwrap_or_else(panic_error),
            **seed
        );
    }
    std::env::set_var("LAYERX_CONFIG", &a);
    credential::delete_key(&mut ca, "same").unwrap_or_else(panic_error);
    assert!(credential::key_seed("same").is_err());
    std::env::set_var("LAYERX_CONFIG", &b);
    assert_eq!(
        *credential::key_seed("same").unwrap_or_else(panic_error),
        *seed_b
    );
    assert_eq!(
        config::Configuration::load()
            .unwrap_or_else(panic_error)
            .keys["same"]
            .public_key,
        mb.public_key
    );
    assert_eq!(
        fixture.get_password().unwrap_or_else(panic_error),
        "22".repeat(32)
    );
    credential::delete_key(&mut cb, "same").unwrap_or_else(panic_error);
    assert_eq!(
        fixture.get_password().unwrap_or_else(panic_error),
        "22".repeat(32)
    );
    fixture.delete_credential().unwrap_or_else(panic_error);
}

fn assert_submit_invocation(raw: &[u8], hash: &str) {
    let expected_raw = raw.to_vec();
    let expected_hash = hash.to_owned();
    let node = HttpFixture::start(move |request| {
        assert_eq!(request.path, "/");
        assert_eq!(request.body["method"], "eth_sendRawTransaction");
        assert_eq!(
            request.body["params"],
            json!([format!("0x{}", encoding::hex_encode(&expected_raw))])
        );
        Some((
            200,
            json!({"jsonrpc":"2.0","id":request.body["id"],"result":expected_hash}),
        ))
    });
    let tools =
        toolset::surface(layerx_mcp::server::DeploymentMode::Full).unwrap_or_else(panic_error);
    let submit = *tools
        .iter()
        .find(|tool| tool.name == "activity.submit")
        .unwrap_or_else(panic_missing);
    let runtime = toolset::runtime(&node.url).unwrap_or_else(panic_error);
    assert_eq!(
        toolset::invoke_task(
            &runtime,
            submit,
            &json!({"raw_transaction":format!("0x{}",encoding::hex_encode(raw))})
        )
        .unwrap_or_else(panic_error),
        json!({"txHash":hash,"rung":"submitted"})
    );
    assert!(toolset::invoke_task(
        &runtime,
        submit,
        &json!({"destination":"02".repeat(32),"amount":"1","account_sequence":"7"})
    )
    .is_err());
    assert_eq!(node.requests().len(), 1);
}

fn assert_a2a_signed_submission() {
    use ed25519_dalek::Signer as _;
    use layerx_crypto::payments::{send_context_hash, CallOptions, SendDebit};
    let signing = ed25519_dalek::SigningKey::from_bytes(&[6; 32]);
    let debit = SendDebit {
        from: [1; 32],
        to: [2; 32],
        asset: [3; 32],
        amount: 1,
        source_sequence: 7,
        idempotency_key: [4; 32],
        expires_at: 2000,
        context_hash: send_context_hash(&[1; 32], &[2; 32], &[3; 32], 1, &[4; 32]),
        conditions: vec![],
        authorization_kind: 1,
        network_id: 125,
        protocol_version: 3,
    };
    let digest = Sha256::new()
        .chain_update(b"LXP/v1/signature-preimage\0")
        .chain_update(debit.authorization_message().unwrap_or_else(panic_error))
        .finalize();
    let payload = debit
        .encode_signed(
            signing.verifying_key().to_bytes(),
            signing.sign(&digest).to_bytes(),
        )
        .unwrap_or_else(panic_error);
    let evm_secret = [1; 32];
    let key = k256::ecdsa::SigningKey::from_slice(&evm_secret).unwrap_or_else(panic_error);
    let public = key.verifying_key().to_encoded_point(false);
    let from = layerx_crypto::secp256k1::evm_address(public.as_bytes()).unwrap_or_else(panic_error);
    let options = CallOptions {
        from,
        chain_id: 125,
        nonce: 41,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
    };
    let disclosed = layerx_crypto::payments::ledger_payment_call(&payload, &options)
        .unwrap_or_else(panic_error);
    assert_eq!(
        disclosed
            .disclosure
            .payload_sequence()
            .unwrap_or_else(panic_error),
        Some(7)
    );
    let confirmation =
        layerx_platform_cli::wallet_signing::PreparedTransaction::new(disclosed.call.clone(), from)
            .unwrap_or_else(panic_error)
            .confirmation();
    assert_eq!(confirmation["nonce"], "41");
    assert_ne!(debit.source_sequence, options.nonce);
    let raw = disclosed.call.sign(&evm_secret).unwrap_or_else(panic_error);
    let hash = format!(
        "0x{}",
        encoding::hex_encode(&layerx_crypto::evm_transaction::transaction_hash(&raw))
    );
    assert_submit_invocation(&raw, &hash);
}

fn reconciliation_program() -> String {
    let journey = include_str!("install-journey.sh");
    let helpers = journey
        .split("<<'A2A_RECEIPT'\n")
        .nth(1)
        .unwrap_or_else(panic_missing)
        .split("def rpc(method, params):")
        .next()
        .unwrap_or_else(panic_missing);
    format!(
        "{helpers}\n{}",
        r"
value = json.load(sys.stdin)
if value.get('batch_fixture'):
    import base64
    case = value['batch_fixture']
    proof = case['sequence_batch_proof']
    batch = {'height': str(case['sequence_batch']['Height']),
             'tx': base64.b64decode(proof['data']).hex(),
             'proof': {'total': int(proof['proof']['total']), 'index': int(proof['proof']['index']),
                       'leafHash': base64.b64decode(proof['proof']['leaf_hash']).hex(),
                       'aunts': [base64.b64decode(a).hex() for a in proof['proof']['aunts']]}}
    wire, positions, previous, resulting, tx_root = batch_evidence(
        batch, 10, case['execution_header']['data_hash'])
    digest = batch_digest(wire, case['execution_header']['chain_id']).hex()
    print(json.dumps({'digest': digest, 'signature': wire[-64:].hex()}))
else:
    authority = sequence_authority(value['height'], value['first'], value['last'],
        hex_bytes(value['previous']), hex_bytes(value['resulting']), hex_bytes(value['tx_root']),
        value['activities'], value['payment']['txHash'], value['denomination'])
    if not value.get('sequence_only'):
        canonical = reconcile_transaction(value['payment'], value['served'], value['actual'],
                                          value['key'], value['denomination'])
        require(int(value['actual']['seq'], 16) == int.from_bytes(canonical[42:50], 'big')
                == authority['sequence'], 'receipt sequence differs from independent batch position')
        reconcile_ledger(value['payment']['disclosure'], value['logs'])
    print(json.dumps(authority))
"
    )
}

fn run_reconciliation(program: &str, value: &Value) -> std::process::Output {
    use std::process::{Command, Stdio};
    let mut child = Command::new("python3")
        .args(["-c", program])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap_or_else(panic_error);
    child
        .stdin
        .take()
        .unwrap_or_else(panic_missing)
        .write_all(&serde_json::to_vec(value).unwrap_or_else(panic_error))
        .unwrap_or_else(panic_error);
    child.wait_with_output().unwrap_or_else(panic_error)
}

fn verify_receipt(
    receipt_path: &Path,
    evidence: &Value,
    case: &Value,
    facts: &Value,
) -> std::process::Output {
    fs::write(
        receipt_path,
        case["signedReceipt"]
            .as_str()
            .unwrap_or_else(panic_missing)
            .trim_start_matches("0x"),
    )
    .unwrap_or_else(panic_error);
    std::process::Command::new(env!("CARGO_BIN_EXE_layerx"))
        .args(["--json", "receipt", "verify", "--receipt"])
        .arg(receipt_path)
        .args([
            "--batch-id",
            facts["batch_id"].as_str().unwrap_or_else(panic_missing),
            "--asset",
            facts["asset"].as_str().unwrap_or_else(panic_missing),
            "--previous-state-root",
            facts["previous_state_root"]
                .as_str()
                .unwrap_or_else(panic_missing),
            "--resulting-state-root",
            facts["resulting_state_root"]
                .as_str()
                .unwrap_or_else(panic_missing),
            "--sequencer-public-key",
            evidence["key"].as_str().unwrap_or_else(panic_missing),
        ])
        .output()
        .unwrap_or_else(panic_error)
}

fn journey_evidence() -> (Vec<Value>, Value, String, Vec<String>, Value) {
    let corpus: Value = serde_json::from_str(include_str!(
        "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap_or_else(panic_error);
    let cases = corpus["cases"]
        .as_array()
        .unwrap_or_else(panic_missing)
        .clone();
    let ledger_case = vector();
    let raw = raw_vector();
    let (call, signature) =
        layerx_crypto::evm_transaction::decode_signed(&raw).unwrap_or_else(panic_error);
    let sender = layerx_crypto::evm_transaction::recover_sender(&call, &signature)
        .unwrap_or_else(panic_error);
    let transaction = format!(
        "0x{}",
        encoding::hex_encode(&layerx_crypto::evm_transaction::transaction_hash(&raw))
    );
    // Replay the producer's accepted transaction order, independently of receipt bytes.
    let activities: Vec<String> = cases[..4]
        .iter()
        .map(|case| {
            let raw = encoding::hex_decode(
                "producer transaction",
                case["rawTransaction"]
                    .as_str()
                    .unwrap_or_else(panic_missing)
                    .trim_start_matches("0x"),
            )
            .unwrap_or_else(panic_error);
            format!(
                "0x{}",
                encoding::hex_encode(&layerx_crypto::evm_transaction::transaction_hash(&raw))
            )
        })
        .collect();
    let mut chain = [0u8; 32];
    let mut predecessor = chain;
    for (offset, activity) in activities.iter().enumerate() {
        if offset == 2 {
            predecessor = chain;
        }
        chain = Sha256::new()
            .chain_update(b"PAXEER/v1/sequence\0")
            .chain_update(chain)
            .chain_update(((offset + 1) as u64).to_be_bytes())
            .chain_update(
                encoding::hex_decode("activity", &activity[2..]).unwrap_or_else(panic_error),
            )
            .finalize()
            .into();
    }
    let block_ids: Vec<u8> = activities[2..4]
        .iter()
        .flat_map(|id| encoding::hex_decode("activity", &id[2..]).unwrap_or_else(panic_error))
        .collect();
    // The producer's seqVecLedgerPayment and accept schedule declare these fields.
    let data = &call.data;
    assert_eq!(&data[..4], &[0x78, 0xea, 0x11, 0xdb]);
    let disclosure = json!({
        "from": format!("0x{}", encoding::hex_encode(&sender)),
        "to": format!("0x{}", encoding::hex_encode(&call.to)),
        "source": encoding::hex_encode(&data[4..36]),
        "destination": encoding::hex_encode(&data[36..68]),
        "asset": encoding::hex_encode(&data[68..100]),
        "amount": u128::from_be_bytes(data[116..132].try_into().unwrap_or_else(panic_error)).to_string(),
        "evm_nonce": call.nonce.to_string()
    });
    let evidence = json!({
        "payment": {"txHash": transaction, "disclosure": disclosure},
        "served": ledger_case["receiptJson"],
        "actual": ledger_case["receiptJson"],
        "logs": ledger_case["receiptJson"]["logs"],
        "key": ledger_case["pinnedKey"]["publicKey"].as_str().unwrap_or_else(panic_missing).trim_start_matches("0x"),
        "denomination": "uhpx",
        "height": 1001, "first": 3, "last": 4,
        "previous": encoding::hex_encode(&predecessor),
        "resulting": encoding::hex_encode(&chain),
        "tx_root": encoding::hex_encode(&Sha256::digest(&block_ids)),
        "activities": activities[2..4],
    });
    assert_ne!(
        disclosure["asset"].as_str().unwrap_or_else(panic_missing),
        encoding::hex_encode(&Sha256::digest(b"uhpx")),
        "ledger payload asset must remain distinct from the native payment denomination"
    );
    (cases, ledger_case, transaction, activities, evidence)
}

fn assert_reconciliation_mismatches(
    program: &str,
    receipt_path: &Path,
    evidence: &Value,
    authority: &Value,
    ledger_case: &Value,
) {
    let reconcile = |value: &Value| run_reconciliation(program, value);
    let verify = |case: &Value, facts: &Value| verify_receipt(receipt_path, evidence, case, facts);
    for (field, check) in [
        ("batch_id", "BatchId"),
        ("asset", "Asset"),
        ("previous_state_root", "PreviousStateRoot"),
        ("resulting_state_root", "ResultingStateRoot"),
    ] {
        let mut wrong = authority.clone();
        wrong[field] = json!("ff".repeat(32));
        let refused = verify(ledger_case, &wrong);
        assert!(!refused.status.success(), "{field} mismatch accepted");
        assert!(
            String::from_utf8_lossy(&refused.stderr).contains(check),
            "{}",
            String::from_utf8_lossy(&refused.stderr)
        );
    }
    for field in ["asset", "source", "destination", "amount"] {
        let mut wrong = evidence.clone();
        wrong["payment"]["disclosure"][field] = if field == "amount" {
            json!("999")
        } else {
            json!("ff".repeat(32))
        };
        let refused = reconcile(&wrong);
        assert!(
            !refused.status.success(),
            "ledger {field} mismatch accepted"
        );
        assert!(String::from_utf8_lossy(&refused.stderr).contains("ledger payment differs"));
    }
    for field in ["previous", "resulting", "tx_root"] {
        let mut wrong = evidence.clone();
        wrong[field] = json!("ff".repeat(32));
        assert!(
            !reconcile(&wrong).status.success(),
            "inconsistent batch {field} accepted"
        );
    }
    for field in [
        "txHash",
        "from",
        "nonce",
        "signingKey",
        "paymentDenom",
        "status",
    ] {
        let mut wrong = evidence.clone();
        wrong["actual"][field] = match field {
            "nonce" | "status" => json!("0x0"),
            "paymentDenom" => json!("another-denomination"),
            _ => json!(format!("0x{}", "ff".repeat(32))),
        };
        assert!(
            !reconcile(&wrong).status.success(),
            "receipt {field} mismatch accepted"
        );
    }
}

fn assert_alternate_transition(
    program: &str,
    receipt_path: &Path,
    evidence: &Value,
    authority: &Value,
    cases: &[Value],
    activities: &[String],
) {
    let reconcile = |value: &Value| run_reconciliation(program, value);
    let verify = |case: &Value, facts: &Value| verify_receipt(receipt_path, evidence, case, facts);
    let mut absent_ledger = evidence.clone();
    absent_ledger["logs"] = json!([]);
    assert!(!reconcile(&absent_ledger).status.success());
    let mut absent = evidence.clone();
    absent["activities"] = json!([]);
    assert!(!reconcile(&absent).status.success());
    let mut other = evidence.clone();
    other["sequence_only"] = json!(true);
    other["payment"]["txHash"] = json!(activities[3]);
    let other_output = reconcile(&other);
    assert!(other_output.status.success());
    let other_authority: Value =
        serde_json::from_slice(&other_output.stdout).unwrap_or_else(panic_error);
    assert!(
        verify(&cases[3], &other_authority).status.success(),
        "other transition has a genuine valid signed receipt"
    );
    assert!(
        !verify(&cases[3], authority).status.success(),
        "valid receipt for a different transition accepted"
    );
    let mut substituted = evidence.clone();
    substituted["actual"] = cases[3]["receiptJson"].clone();
    substituted["served"] = cases[3]["receiptJson"].clone();
    assert!(!reconcile(&substituted).status.success());
    let tampered = cases
        .iter()
        .find(|case| case["id"] == "V3-tampered-log")
        .unwrap_or_else(panic_missing);
    assert!(!verify(tampered, authority).status.success());
}

fn assert_batch_signature(program: &str) {
    let reconcile = |value: &Value| run_reconciliation(program, value);
    let committed: Value = serde_json::from_str(include_str!(
        "../../../../sdk/platform/conformance/fixtures/vector-v7-named-payee.json"
    ))
    .unwrap_or_else(panic_error);
    let batch = reconcile(&json!({"batch_fixture": committed["settlement"]}));
    assert!(
        batch.status.success(),
        "{}",
        String::from_utf8_lossy(&batch.stderr)
    );
    let batch: Value = serde_json::from_slice(&batch.stdout).unwrap_or_else(panic_error);
    let batch_key: [u8; 32] = committed["authorization"]["PublicKey"]
        .as_array()
        .unwrap_or_else(panic_missing)
        .iter()
        .map(|byte| {
            u8::try_from(byte.as_u64().unwrap_or_else(panic_missing)).unwrap_or_else(panic_error)
        })
        .collect::<Vec<_>>()
        .try_into()
        .unwrap_or_else(panic_error);
    let batch_signature = encoding::fixed_hex::<64>(
        "recorded batch signature",
        batch["signature"].as_str().unwrap_or_else(panic_missing),
    )
    .unwrap_or_else(panic_error);
    ed25519_dalek::VerifyingKey::from_bytes(&batch_key)
        .unwrap_or_else(panic_error)
        .verify_strict(
            &encoding::hex_decode(
                "batch digest",
                batch["digest"].as_str().unwrap_or_else(panic_missing),
            )
            .unwrap_or_else(panic_error),
            &ed25519_dalek::Signature::from_bytes(&batch_signature),
        )
        .unwrap_or_else(panic_error);
    let mut wrong_batch = committed["settlement"].clone();
    wrong_batch["sequence_batch"]["Height"] = json!(11);
    assert!(!reconcile(&json!({"batch_fixture": wrong_batch}))
        .status
        .success());
}

#[test]
fn install_journey_a2a_contract() {
    assert_a2a_signed_submission();
    let (cases, ledger_case, transaction, activities, evidence) = journey_evidence();
    let program = reconciliation_program();
    let reconcile = |value: &Value| run_reconciliation(&program, value);
    let matching = reconcile(&evidence);
    assert!(
        matching.status.success(),
        "{}",
        String::from_utf8_lossy(&matching.stderr)
    );
    let authority: Value = serde_json::from_slice(&matching.stdout).unwrap_or_else(panic_error);
    let root = tempfile::tempdir().unwrap_or_else(panic_error);
    let receipt_path = root.path().join("receipt.hex");
    let verify =
        |case: &Value, facts: &Value| verify_receipt(&receipt_path, &evidence, case, facts);
    let accepted = verify(&ledger_case, &authority);
    assert!(
        accepted.status.success(),
        "{}",
        String::from_utf8_lossy(&accepted.stderr)
    );
    let accepted: Value = serde_json::from_slice(&accepted.stdout).unwrap_or_else(panic_error);
    assert_eq!(accepted["data"]["verified"], true);
    assert_eq!(accepted["data"]["result_code"], 0);
    assert_eq!(accepted["data"]["activity_id"], &transaction[2..]);
    assert_reconciliation_mismatches(&program, &receipt_path, &evidence, &authority, &ledger_case);
    assert_alternate_transition(
        &program,
        &receipt_path,
        &evidence,
        &authority,
        &cases,
        &activities,
    );
    assert_batch_signature(&program);
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

fn served_messages(
    binary: Option<&str>,
    root: &Path,
    binding: &Path,
    messages: &[Value],
) -> Vec<Value> {
    use std::process::{Command, Stdio};
    let Some(binary) = binary else {
        return stdio(binding, messages);
    };
    let mut child = Command::new(binary)
        .args(["mcp", "serve", "--daemon-binding"])
        .arg(binding)
        .env("LAYERX_CONFIG", root.join("config.json"))
        .env("LAYERX_INSTALL_ROOT", root)
        .env_remove("LAYERX_CREDENTIAL_STORE")
        .env_remove("LAYERX_GATEWAY_KEY_ID")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "real CLI MCP stdio"));
    let mut stdin = child
        .stdin
        .take()
        .unwrap_or_else(|| panic!("{}", "MCP stdin"));
    for message in messages {
        serde_json::to_writer(&mut stdin, message).unwrap_or_else(panic_error);
        writeln!(stdin).unwrap_or_else(panic_error);
    }
    drop(stdin);
    let output = child
        .wait_with_output()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "CLI MCP exit"));
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    String::from_utf8(output.stdout)
        .unwrap_or_else(panic_error)
        .lines()
        .map(|line| {
            serde_json::from_str(line)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "CLI protocol JSON"))
        })
        .collect()
}

fn panic_error<T, E: std::fmt::Debug>(error: E) -> T {
    panic!("{error:?}")
}

fn panic_missing<T>() -> T {
    panic!("expected value")
}
