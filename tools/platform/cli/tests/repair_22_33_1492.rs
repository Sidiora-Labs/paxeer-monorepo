//! Task 22.33.1492 repair record (spec edits are outside the authorized scope).
//! `a2a_task_recovery`: `a2a_restart_recovers_original_hash`; `a2a_capacity_retains_unresolved_tasks`.
//! `test_retired_endpoint_a2a_embedded`: `a2a_root_listener_authentication_and_retired_refusal`.
//! `retired_endpoint_chains`: `chains_root_and_creation_recovery`.
//! `chain_submission_recovery`: `chains_root_and_creation_recovery`.
//! `retired_endpoint_faucet`: `faucet_explicit_service_route_preserves_identity_authorization`.
//! `retired_endpoint_main`: `main_accepts_root_node_and_rejects_retired_path`.
//! `mcp_serve_binding_wiring`: `mcp_unchanged_enrollment_stdio_https_authorized_call_after_restart`.
//! `program_lifecycle_evidence`: `lifecycle_signed_producer_receipts_distinguish_pending_success_and_revert`.
//! Prior attempt: 60 passed, three failed: `a2a_root_listener_authentication_and_retired_refusal`,
//! `lifecycle_signed_producer_receipts_distinguish_pending_success_and_revert`, and
//! `a2a::submission_outcome_tests::a2a_response_loss_keeps_unknown_outcome` (gotcha 3179).
//! Wiring request: tools/platform/cli/src/rpc.rs and `rpc_subscription.rs`, CLI transport domain 8:
//! migrate other consumers of the retired public routes in their owning task.
//! Wiring request: tools/platform/cli/src/toolset.rs, CLI domain 8: share the corrected root-node
//! and explicit faucet routes with non-A2A consumers; this task repairs the A2A execution owner.
//! Wiring request: `tools/platform/cli/tests/program_evm_lifecycle.rs`, CLI qualification domain 8:
//! replace its bare txHash/rung success contract with authenticated producer evidence.
//! Wiring request: tools/platform/cli/src/install/a2a.rs, CLI installation domain 8: persist
//! the optional explicit faucet endpoint and receipt policy in installed runtime arguments.
//! Wiring request: spec/layerx-merge/spec.kvx, workflow owner: copy this mapping and gate evidence
//! into the task record; the worker is explicitly forbidden to edit spec files.
//! The MCP peer below is the committed enrollment/TLS peer from `repair_22_31_213.rs`.

#[path = "../src/a2a.rs"]
#[allow(dead_code)]
mod a2a;
#[path = "../src/config.rs"]
#[allow(dead_code)]
mod config;
#[path = "../src/credential.rs"]
#[allow(dead_code)]
mod credential;
#[path = "../src/encoding.rs"]
mod encoding;
#[path = "../src/faucet.rs"]
#[allow(dead_code)]
mod faucet;
#[path = "../src/file_store.rs"]
mod file_store;
#[path = "../src/http.rs"]
#[allow(dead_code)]
mod http;
#[path = "../src/install/mod.rs"]
#[allow(dead_code)]
mod install;
#[path = "../src/mcp.rs"]
#[allow(dead_code)]
mod mcp;
#[path = "../src/output.rs"]
#[allow(dead_code)]
mod output;
#[path = "../src/programs.rs"]
#[allow(dead_code)]
mod programs;
#[path = "../src/toolset.rs"]
#[allow(dead_code)]
mod toolset;
use ed25519_dalek::Signer as _;
use serde_json::json;

use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::{SocketAddrV4, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt as _;
use std::path::Path;
use std::process::Command;
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
    let surface = DaemonSurface::new(
        &fs::read_to_string(root.join("read-address"))
            .unwrap_or_else(|error| panic!("{}: {error:?}", "read peer address")),
        "d".repeat(48),
        [0xcc; 32],
    )
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

struct Peer {
    endpoint: String,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl Peer {
    fn start(mut handle: impl FnMut(&str, &str, Value) -> Option<Value> + Send + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("{}: {error:?}", "loopback peer"));
        listener.set_nonblocking(true).unwrap_or_else(panic_error);
        let endpoint = format!(
            "http://{}",
            listener.local_addr().unwrap_or_else(panic_error)
        );
        let stop = Arc::new(AtomicBool::new(false));
        let closing = stop.clone();
        let worker = thread::spawn(move || {
            while !closing.load(Ordering::Acquire) {
                let (mut socket, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("peer accept: {error}"),
                };
                socket
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap_or_else(panic_error);
                socket
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap_or_else(panic_error);
                let mut reader = BufReader::new(&mut socket);
                let mut header = String::new();
                reader.read_line(&mut header).unwrap_or_else(panic_error);
                let request_line = header.clone();
                let mut length = 0;
                let mut authorization = String::new();
                loop {
                    header.clear();
                    assert!(reader.read_line(&mut header).unwrap_or_else(panic_error) > 0);
                    if header == "\r\n" {
                        break;
                    }
                    let (name, value) = header
                        .split_once(':')
                        .unwrap_or_else(|| panic!("{}", "HTTP field"));
                    if name.eq_ignore_ascii_case("content-length") {
                        length = value.trim().parse::<usize>().unwrap_or_else(panic_error);
                    } else if name.eq_ignore_ascii_case("authorization") {
                        value.trim().clone_into(&mut authorization);
                    }
                }
                assert!(length < 2_000_000);
                let mut body = vec![0; length];
                reader.read_exact(&mut body).unwrap_or_else(panic_error);
                let request = if body.is_empty() {
                    Value::Null
                } else {
                    serde_json::from_slice(&body).unwrap_or_else(panic_error)
                };
                if let Some(response) = handle(request_line.trim(), &authorization, request) {
                    let bytes = serde_json::to_vec(&response).unwrap_or_else(panic_error);
                    write!(socket,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",bytes.len()).unwrap_or_else(panic_error);
                    socket.write_all(&bytes).unwrap_or_else(panic_error);
                }
            }
        });
        Self {
            endpoint,
            stop,
            worker: Some(worker),
        }
    }
}
impl Drop for Peer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .unwrap_or_else(panic_missing)
            .join()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "peer worker"));
    }
}
fn envelope_for(request: &Value, result: &Value) -> Value {
    json!({"jsonrpc":"2.0","id":request["id"],"result":result})
}
fn hex(bytes: &[u8]) -> String {
    format!("0x{}", encoding::hex_encode(bytes))
}
fn bytes(text: &str) -> Vec<u8> {
    encoding::hex_decode("fixture", text.strip_prefix("0x").unwrap_or(text))
        .unwrap_or_else(panic_error)
}
fn vectors() -> Value {
    let document: Value = serde_json::from_str(include_str!(
        "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap_or_else(panic_error);
    assert_eq!(
        document["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    document
}
fn fixture_case() -> Value {
    vectors()["cases"]
        .as_array()
        .unwrap_or_else(panic_missing)
        .iter()
        .find(|case| case["id"] == "V1-layerxprograms-call")
        .unwrap_or_else(panic_missing)
        .clone()
}
fn policy_for(case: &Value) -> layerx_sdk::rpc_verification::ReceiptPolicy {
    let pin = &case["pinnedKey"];
    layerx_sdk::rpc_verification::ReceiptPolicy {
        protocol_version: 2,
        network_id: 125,
        sequencer: layerx_proof::inclusion::SequencerAuthorization::new(
            bytes(pin["keyId"].as_str().unwrap_or_else(panic_missing))
                .try_into()
                .unwrap_or_else(panic_error),
            bytes(pin["publicKey"].as_str().unwrap_or_else(panic_missing))
                .try_into()
                .unwrap_or_else(panic_error),
            pin["firstHeight"].as_u64().unwrap_or_else(panic_missing),
            pin["lastHeight"].as_u64().unwrap_or_else(panic_missing),
        ),
        trusted_checkpoint_context_digest: None,
    }
}
fn task_message(raw: &str) -> Value {
    json!({"message":{"kind":"message","role":"user","messageId":"payment","parts":[{"kind":"data","data":{"skill":"activity.submit","arguments":{"raw_transaction":raw}}}]}})
}
fn dispatch(
    runtime: &a2a::Runtime,
    ledger: &Mutex<a2a::TaskLedger>,
    method: &str,
    params: &Value,
) -> Value {
    a2a::dispatch(
        runtime,
        &toolset::surface(layerx_mcp::server::DeploymentMode::Full).unwrap_or_else(panic_error),
        ledger,
        &serde_json::to_vec(&json!({"jsonrpc":"2.0","id":1,"method":method,"params":params}))
            .unwrap_or_else(panic_error),
    )
}

#[test]
fn a2a_restart_recovers_original_hash() {
    let root = tempfile::tempdir().unwrap_or_else(panic_error);
    let path = root.path().join("tasks.json");
    let case = fixture_case();
    let expected_hash = case["receiptJson"]["txHash"].clone();
    let raw = case["rawTransaction"]
        .as_str()
        .unwrap_or_else(panic_missing);
    let on_disk = path.clone();
    let transaction = raw.to_owned();
    let expected = expected_hash.clone();
    let node = Peer::start(move |line, auth, request| {
        assert_eq!(line, "POST / HTTP/1.1");
        assert!(auth.is_empty());
        assert_eq!(request["method"], "eth_sendRawTransaction");
        assert_eq!(request["params"][0], transaction);
        let durable: BTreeMap<String, Value> =
            serde_json::from_slice(&fs::read(&on_disk).unwrap_or_else(panic_error))
                .unwrap_or_else(panic_error);
        assert_eq!(durable.len(), 1);
        let task = durable.values().next().unwrap_or_else(panic_missing);
        assert_eq!(task["txHash"], expected);
        assert_eq!(task["status"]["state"], "unknown");
        None // Accepted by the peer, then the response is lost.
    });
    let runtime = a2a::Runtime::connect(&node.endpoint, 125, None).unwrap_or_else(panic_error);
    let ledger = Mutex::new(a2a::TaskLedger::open(&path).unwrap_or_else(panic_error));
    let sent = dispatch(&runtime, &ledger, "message/send", &task_message(raw));
    assert_eq!(sent["result"]["status"]["state"], "unknown");
    assert_eq!(sent["result"]["txHash"], expected_hash);
    let id = sent["result"]["id"]
        .as_str()
        .unwrap_or_else(panic_missing)
        .to_owned();
    drop(ledger);
    drop(runtime);
    drop(node);
    let receipt = case["receiptJson"].clone();
    let expected = expected_hash.clone();
    let node = Peer::start(move |line, _, request| {
        assert_eq!(line, "POST / HTTP/1.1");
        assert_eq!(request["method"], "pax_sequencerReceipt");
        assert_eq!(request["params"], json!([expected]));
        Some(envelope_for(&request, &receipt))
    });
    let runtime = a2a::Runtime::connect(&node.endpoint, 125, None)
        .unwrap_or_else(panic_error)
        .with_policy(policy_for(&case))
        .unwrap_or_else(panic_error);
    let mut recovered = a2a::TaskLedger::open(&path).unwrap_or_else(panic_error);
    assert_eq!(recovered[&id]["status"]["state"], "unknown");
    recovered.reconcile(&runtime).unwrap_or_else(panic_error);
    assert_eq!(recovered[&id]["status"]["state"], "completed");
    assert_eq!(recovered[&id]["reconciliation"], case["receiptJson"]);
    drop(recovered);
    let reopened = Mutex::new(a2a::TaskLedger::open(&path).unwrap_or_else(panic_error));
    let fetched = dispatch(&runtime, &reopened, "tasks/get", &json!({"id":id}));
    assert_eq!(fetched["result"]["txHash"], expected_hash);
    assert_eq!(fetched["result"]["status"]["state"], "completed");
    assert!(
        a2a::TaskLedger::open(&path).is_err(),
        "two services must not own one journal"
    );
}

#[test]
fn a2a_capacity_retains_unresolved_tasks() {
    let root = tempfile::tempdir().unwrap_or_else(panic_error);
    let path = root.path().join("tasks.json");
    let accepted = Arc::new(std::sync::atomic::AtomicUsize::new(0));
    let count = accepted.clone();
    let node = Peer::start(move |line, _, request| {
        assert_eq!(line, "POST / HTTP/1.1");
        assert_eq!(request["method"], "eth_sendRawTransaction");
        count.fetch_add(1, Ordering::SeqCst);
        None
    });
    let runtime = a2a::Runtime::connect(&node.endpoint, 125, None).unwrap_or_else(panic_error);
    let ledger = Mutex::new(a2a::TaskLedger::open(&path).unwrap_or_else(panic_error));
    let case = fixture_case();
    let mut ids = Vec::new();
    for _ in 0..128 {
        let sent = dispatch(
            &runtime,
            &ledger,
            "message/send",
            &task_message(
                case["rawTransaction"]
                    .as_str()
                    .unwrap_or_else(panic_missing),
            ),
        );
        assert_eq!(sent["result"]["status"]["state"], "unknown");
        ids.push(
            sent["result"]["id"]
                .as_str()
                .unwrap_or_else(panic_missing)
                .to_owned(),
        );
    }
    drop(ledger);
    let ledger = Mutex::new(a2a::TaskLedger::open(&path).unwrap_or_else(panic_error));
    let refused = dispatch(
        &runtime,
        &ledger,
        "message/send",
        &task_message(
            case["rawTransaction"]
                .as_str()
                .unwrap_or_else(panic_missing),
        ),
    );
    assert_eq!(refused["error"]["code"], -32005);
    assert_eq!(accepted.load(Ordering::SeqCst), 128);
    for id in ids {
        let task = dispatch(&runtime, &ledger, "tasks/get", &json!({"id":id}));
        assert_eq!(task["result"]["txHash"], case["receiptJson"]["txHash"]);
        assert_eq!(task["result"]["status"]["state"], "unknown");
    }
    assert_eq!(ledger.lock().unwrap_or_else(panic_error).len(), 128);
}

#[test]
fn a2a_root_listener_authentication_and_retired_refusal() {
    let node_key = format!("node1:lxp_live_{}", "a".repeat(64));
    let expected = format!("LayerX-Key {node_key}");
    let case = fixture_case();
    let receipt = case["receiptJson"].clone();
    let node = Peer::start(move |line, auth, request| {
        assert_eq!(line, "POST / HTTP/1.1");
        assert_eq!(auth, expected);
        assert_eq!(request["method"], "pax_sequencerReceipt");
        Some(envelope_for(&request, &receipt))
    });
    let runtime = Arc::new(
        a2a::Runtime::connect(&node.endpoint, 125, Some(zeroize::Zeroizing::new(node_key)))
            .unwrap_or_else(panic_error),
    );
    let root = tempfile::tempdir().unwrap_or_else(panic_error);
    let ledger = Arc::new(Mutex::new(
        a2a::TaskLedger::open(&root.path().join("tasks.json")).unwrap_or_else(panic_error),
    ));
    let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(panic_error);
    let endpoint = format!(
        "http://{}",
        listener.local_addr().unwrap_or_else(panic_error)
    );
    let worker = thread::spawn(move || {
        let tools =
            toolset::surface(layerx_mcp::server::DeploymentMode::Full).unwrap_or_else(panic_error);
        for _ in 0..4 {
            let (mut stream, _) = listener.accept().unwrap_or_else(panic_error);
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap_or_else(panic_error);
            a2a::serve_connection(
                &runtime,
                &tools,
                &json!({"name":"LayerX"}),
                &ledger,
                "local-secret",
                &mut stream,
            )
            .unwrap_or_else(panic_error);
        }
    });
    let request = json!({"jsonrpc":"2.0","id":1,"method":"message/send","params":{"message":{"parts":[{"kind":"data","data":{"skill":"receipt.get","arguments":{"activity_id":case["receiptJson"]["txHash"]}}}]}}});
    let (status, refusal) = post(&endpoint, &request, false);
    assert_eq!(status, 401, "{refusal}");
    assert_eq!(refusal["error"], "valid bearer authorization is required");
    let mut withheld =
        TcpStream::connect(endpoint.trim_start_matches("http://")).unwrap_or_else(panic_error);
    withheld
        .set_read_timeout(Some(Duration::from_secs(5)))
        .unwrap_or_else(panic_error);
    withheld.write_all(b"POST / HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n").unwrap_or_else(panic_error);
    let mut refusal = String::new();
    withheld
        .read_to_string(&mut refusal)
        .unwrap_or_else(panic_error);
    assert!(refusal.starts_with("HTTP/1.1 401 Unauthorized\r\n"));
    let refusal: Value = serde_json::from_str(
        refusal
            .split_once("\r\n\r\n")
            .unwrap_or_else(panic_missing)
            .1,
    )
    .unwrap_or_else(panic_error);
    assert_eq!(refusal["error"], "valid bearer authorization is required");
    let (status, response) = post(&endpoint, &request, true);
    assert_eq!(status, 200, "{response}");
    assert_eq!(
        response["result"]["artifacts"][0]["parts"][0]["data"]["result"],
        case["receiptJson"]
    );
    let retired = json!({"jsonrpc":"2.0","id":2,"method":"message/send","params":{"message":{"parts":[{"kind":"data","data":{"skill":"lx_submitActivity","arguments":{}}}]}}});
    assert_eq!(post(&endpoint, &retired, true).1["error"]["code"], -32601);
    assert!(layerx_platform_cli::rpc::request("lx_getReceipt", &json!([])).is_err());
    worker.join().unwrap_or_else(panic_error);
    assert!(a2a::Runtime::connect("http://public.example", 125, None).is_err());
}

fn post(endpoint: &str, body: &Value, authorized: bool) -> (u16, Value) {
    let agent: ureq::Agent = ureq::Agent::config_builder()
        .http_status_as_error(false)
        .timeout_global(Some(Duration::from_secs(5)))
        .build()
        .into();
    let mut request = agent
        .post(format!("{endpoint}/"))
        .header("Content-Type", "application/json");
    if authorized {
        request = request.header("Authorization", "Bearer local-secret");
    }
    let mut response = request
        .send(
            serde_json::to_vec(body)
                .unwrap_or_else(panic_error)
                .as_slice(),
        )
        .unwrap_or_else(panic_error);
    let status = response.status().as_u16();
    let value: Value = response.body_mut().read_json().unwrap_or_else(panic_error);
    (status, value)
}

fn exercise(binding: &Path, program: &str, read_only: bool) {
    let input = format!(
        "{}\n{}\n{}\n{}\n",
        json!({"jsonrpc":"2.0","id":1,"method":"initialize"}),
        json!({"jsonrpc":"2.0","method":"notifications/initialized"}),
        json!({"jsonrpc":"2.0","id":2,"method":"tools/list"}),
        json!({"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"balance.get","arguments":{"program":program}}})
    );
    let mut output = Vec::new();
    mcp::serve_io(
        binding,
        read_only,
        &mut std::io::Cursor::new(input),
        &mut output,
    )
    .unwrap_or_else(panic_error);
    let replies: Vec<Value> = String::from_utf8(output)
        .unwrap_or_else(panic_error)
        .lines()
        .map(|line| serde_json::from_str(line).unwrap_or_else(panic_error))
        .collect();
    assert_eq!(replies.len(), 3);
    assert_eq!(
        replies[0]["result"]["_meta"]["layerx/binding"],
        "agent-daemon"
    );
    let names: BTreeSet<_> = replies[1]["result"]["tools"]
        .as_array()
        .unwrap_or_else(panic_missing)
        .iter()
        .map(|tool| tool["name"].as_str().unwrap_or_else(panic_missing))
        .collect();
    let expected: BTreeSet<_> = catalogue()
        .iter()
        .filter(|tool| !read_only || tool.kind == ToolKind::Read)
        .map(|tool| tool.name)
        .collect();
    assert_eq!(names, expected);
    assert_eq!(replies[2]["result"]["isError"], false, "{}", replies[2]);
    let result = &replies[2]["result"]["structuredContent"]["result"];
    assert_eq!(result["program"], program);
    assert_eq!(result["balances"][0]["amount"], "7");
    assert_eq!(result["freshness"]["observed_sequence"], 120);
}

fn chain_record() -> layerx_client::chains::ChainRecord {
    layerx_client::chains::ChainRecord {
        id: 1,
        name: "recovered-chain".into(),
        operator: bytes("7e5f4552091a69125d5dfcb7b8c2659029395bdf")
            .try_into()
            .unwrap_or_else(panic_error),
        status: 1,
        chain_id: layerx_client::chains::virtual_chain_id(1),
        gas_allowance: 500_000,
        byte_allowance: 65_536,
        fee_share_bps: 100,
        bond: [0; 32],
        base_fee_per_gas: [0; 32],
        created_height: 10,
    }
}
fn creation_peer(saved: std::path::PathBuf) -> Peer {
    use layerx_client::chains::{ChainCall, ChainEvent, ChainsParams, CHAINS_PRECOMPILE};
    Peer::start(move |line, _, request| {
        assert_eq!(line, "POST / HTTP/1.1");
        let result = match request["method"].as_str().unwrap_or_else(panic_missing) {
            "eth_call" => {
                let data = bytes(
                    request["params"][0]["data"]
                        .as_str()
                        .unwrap_or_else(panic_missing),
                );
                assert_eq!(
                    layerx_client::chains::decode_call(&data).unwrap_or_else(panic_error),
                    ChainCall::Params
                );
                hex(&ChainsParams {
                    authority: [1; 20],
                    min_bond: [0; 32],
                    creation_fee: [0; 32],
                    min_gas_allowance: 1,
                    max_gas_allowance: 5_000_000,
                    total_gas_allowance_left: 5_000_000,
                    max_chains: 100,
                }
                .encode())
                .into()
            }
            "eth_chainId" => json!("0x7d"),
            "eth_getTransactionCount" => json!("0x0"),
            "eth_gasPrice" => json!("0x2"),
            "eth_maxPriorityFeePerGas" => json!("0x1"),
            "eth_estimateGas" => json!("0x186a0"),
            "eth_sendRawTransaction" => {
                let raw = bytes(request["params"][0].as_str().unwrap_or_else(panic_missing));
                let hash = hex(&layerx_crypto::evm_transaction::transaction_hash(&raw));
                let event = ChainEvent::Created {
                    id: 1,
                    name: chain_record().name,
                    operator: chain_record().operator,
                    chain_id: chain_record().chain_id,
                };
                let (topics, data) = event.encode();
                let receipt = json!({"transactionHash":hash,"status":"0x1","logs":[{"address":hex(&CHAINS_PRECOMPILE),"topics":topics.iter().map(|v|hex(v)).collect::<Vec<_>>(),"data":hex(&data)}]});
                fs::write(
                    &saved,
                    json!({"hash":hash,"raw":request["params"][0],"receipt":receipt}).to_string(),
                )
                .unwrap_or_else(panic_error);
                return None;
            }
            method => panic!("unexpected creation method {method}"),
        };
        Some(envelope_for(&request, &result))
    })
}

#[test]
fn chains_root_and_creation_recovery() {
    use layerx_client::chains::ChainCall;
    let root = tempfile::tempdir().unwrap_or_else(panic_error);
    let key = root.path().join("evm.json");
    fs::write(&key,json!({"evm_address":hex(&chain_record().operator),"evm_private_key":format!("{}01","00".repeat(31))}).to_string()).unwrap_or_else(panic_error);
    fs::set_permissions(&key, fs::Permissions::from_mode(0o600)).unwrap_or_else(panic_error);
    let durable = root.path().join("node.json");
    let node = creation_peer(durable.clone());
    let client =
        layerx_platform_cli::http::Client::new(&node.endpoint, None).unwrap_or_else(panic_error);
    let pending = layerx_platform_cli::chains::create(
        &client,
        &node.endpoint,
        "recovered-chain",
        layerx_platform_cli::commands::ChainTier::Starter,
        100,
        &key,
    )
    .unwrap_or_else(panic_error);
    assert_eq!(pending["state"], "unknown");
    let recorded: Value = serde_json::from_slice(&fs::read(&durable).unwrap_or_else(panic_error))
        .unwrap_or_else(panic_error);
    assert_eq!(pending["transactionHash"], recorded["hash"]);
    assert_eq!(pending["operator"], hex(&chain_record().operator));
    drop(client);
    drop(node);
    let reopened: Value = serde_json::from_slice(&fs::read(&durable).unwrap_or_else(panic_error))
        .unwrap_or_else(panic_error);
    let producer = reopened.clone();
    let node = Peer::start(move |line, _, request| {
        assert_eq!(line, "POST / HTTP/1.1");
        let result = match request["method"].as_str().unwrap_or_else(panic_missing) {
            "eth_getTransactionReceipt" => {
                assert_eq!(request["params"][0], producer["hash"]);
                producer["receipt"].clone()
            }
            "eth_call" => {
                assert_eq!(
                    layerx_client::chains::decode_call(&bytes(
                        request["params"][0]["data"]
                            .as_str()
                            .unwrap_or_else(panic_missing)
                    ))
                    .unwrap_or_else(panic_error),
                    ChainCall::GetChain(1)
                );
                json!(hex(&chain_record().encode()))
            }
            method => panic!("recovery must not resubmit: {method}"),
        };
        Some(envelope_for(&request, &result))
    });
    let client =
        layerx_platform_cli::http::Client::new(&node.endpoint, None).unwrap_or_else(panic_error);
    let recovered = layerx_platform_cli::chains::reconcile_creation(
        &client,
        &node.endpoint,
        reopened["hash"].as_str().unwrap_or_else(panic_missing),
        "recovered-chain",
        &hex(&chain_record().operator),
    )
    .unwrap_or_else(panic_error);
    assert_eq!(recovered["state"], "completed");
    assert_eq!(recovered["chain"]["id"], 1);
    assert_eq!(recovered["chain"]["operator"], pending["operator"]);
    assert_eq!(recovered["transactionHash"], pending["transactionHash"]);
}

#[test]
fn faucet_explicit_service_route_preserves_identity_authorization() {
    let peer = Peer::start(|line, auth, request| {
        assert_eq!(line, "POST /services/faucet HTTP/1.1");
        assert_eq!(auth, "Bearer identity-session");
        assert_eq!(
            request,
            faucet::request("did:layerx:fixture", &"ab".repeat(32))
        );
        Some(envelope_for(
            &request,
            &json!({"funded":true,"funding_id":"ef".repeat(32),"transaction_id":"cd".repeat(32),"amount":"1000000","network":"mainnet-beta"}),
        ))
    });
    let response = faucet::claim(
        &format!("{}/services/faucet", peer.endpoint),
        zeroize::Zeroizing::new("identity-session".into()),
        "did:layerx:fixture",
        &"ab".repeat(32),
    )
    .unwrap_or_else(panic_error);
    assert_eq!(
        faucet::decode(&response).unwrap_or_else(panic_error)["txHash"],
        "cd".repeat(32)
    );
    for endpoint in [
        "http://public.example/services/faucet",
        "https://user:password@public.example/services/faucet",
        "https://public.example/services/faucet?token=1",
    ] {
        assert!(faucet::claim(
            endpoint,
            zeroize::Zeroizing::new("identity-session".into()),
            "did:layerx:fixture",
            &"ab".repeat(32)
        )
        .is_err());
    }
}

#[test]
fn main_accepts_root_node_and_rejects_retired_path() {
    let seen = Arc::new(std::sync::atomic::AtomicUsize::new(0));
    let count = seen.clone();
    let node = Peer::start(move |line, authorization, request| {
        assert_eq!(line, "POST / HTTP/1.1");
        assert!(authorization.is_empty());
        assert_eq!(request["method"], "eth_call");
        count.fetch_add(1, Ordering::SeqCst);
        Some(envelope_for(
            &request,
            &json!(hex(&chain_record().encode())),
        ))
    });
    let root = tempfile::tempdir().unwrap_or_else(panic_error);
    let output = Command::new(env!("CARGO_BIN_EXE_layerx"))
        .env("LAYERX_CONFIG", root.path().join("config.json"))
        .args(["--json", "--rpc", &node.endpoint, "chains", "show", "1"])
        .output()
        .unwrap_or_else(panic_error);
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let result: Value = serde_json::from_slice(&output.stdout).unwrap_or_else(panic_error);
    assert_eq!(result["data"]["id"], 1);
    assert_eq!(
        result["data"]["rpcUrl"],
        format!("{}/chain/1", node.endpoint)
    );
    let output = Command::new(env!("CARGO_BIN_EXE_layerx"))
        .env("LAYERX_CONFIG", root.path().join("config.json"))
        .args([
            "--json",
            "--rpc",
            &format!("{}/rpc", node.endpoint),
            "chains",
            "show",
            "1",
        ])
        .output()
        .unwrap_or_else(panic_error);
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("root"));
    assert_eq!(seen.load(Ordering::SeqCst), 1);
    for endpoint in [
        "http://public.example",
        "https://public.example/rpc",
        "https://public.example/rpc/ws",
        "https://public.example/?token=1",
        "https://user:password@public.example",
    ] {
        assert!(layerx_platform_cli::chains::node_endpoint(endpoint).is_err());
    }
    assert_eq!(
        layerx_platform_cli::chains::node_endpoint("https://public.example/")
            .unwrap_or_else(panic_error),
        "https://public.example"
    );
}

#[test]
fn mcp_unchanged_enrollment_stdio_https_authorized_call_after_restart() {
    let directory = tempfile::tempdir().unwrap_or_else(panic_error);
    let root = fs::canonicalize(directory.path()).unwrap_or_else(panic_error);
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700)).unwrap_or_else(panic_error);
    transport_files(&root);
    let program = "cc".repeat(32);
    let expected_program = program.clone();
    let read_peer = Peer::start(move |line, authorization, request| {
        assert!(request.is_null());
        assert_eq!(
            line,
            format!("GET /v1/programs/{expected_program}/balances HTTP/1.1")
        );
        assert_eq!(authorization, format!("Bearer {}", "d".repeat(48)));
        // Committed daemon read transcript, consumed through AgentSurface and ProgramReads.
        Some(
            json!({"program":expected_program,"lifecycle":"active","accounts":[{"account":"11".repeat(32),"asset":"22".repeat(32),"amount":"7","frozen":false}],
            "freshness":{"observed_sequence":120,"observed_at":1,"receipt_digest":"33".repeat(32),"state_root":"44".repeat(32),"valid_through":400}}),
        )
    });
    fs::write(
        root.join("read-address"),
        read_peer
            .endpoint
            .strip_prefix("http://")
            .unwrap_or_else(panic_missing),
    )
    .unwrap_or_else(panic_error);
    let mut gateway = Gateway::start(&root, 0);
    let endpoint = gateway.endpoint();
    let port = gateway.port;
    let (published, binding_publisher_handle, identity, request) = enrol(&root, &endpoint);
    let original = fs::read(&published.binding).unwrap_or_else(panic_error);
    let document: Value = serde_json::from_slice(&original).unwrap_or_else(panic_error);
    assert!(document.get("core_sequence").is_none());
    assert_eq!(document["core_height"], 50);
    gateway.serve(&root, &published.binding);
    exercise(&published.binding, &program, false);
    let binding = Binding::open(&published.binding).unwrap_or_else(panic_error);
    let session = binding.daemon_credential().unwrap_or_else(panic_error);
    let credential = EnvelopeCredential::new(
        session.tenant().as_str(),
        session.session_id().0,
        session.token_id(),
        session.generation(),
    )
    .unwrap_or_else(panic_error);
    let transport = AgentEnvelopeTransport::connect(&endpoint, None, Some(&root.join("ca.pem")))
        .unwrap_or_else(panic_error);
    let Err(refusal) = DaemonClientSession::connect(transport, credential, binding.mode()) else {
        panic!("gateway credential must be required");
    };
    assert_eq!(refusal["reason"], "gateway.authentication");
    drop(gateway);
    drop(binding_publisher_handle);
    let binding_publisher_handle = binding_publisher(&root, &endpoint);
    let store = Store::open(root.join("store")).unwrap_or_else(panic_error);
    let mut sessions = SessionRegistry::default();
    sessions
        .restore_tenant(&store, identity.tenant())
        .unwrap_or_else(panic_error);
    binding_publisher_handle
        .validate_existing(&sessions, &identity, &request)
        .unwrap_or_else(panic_error);
    drop(store);
    let mut gateway = Gateway::start(&root, port);
    gateway.serve(&root, &published.binding);
    exercise(&published.binding, &program, true);
    assert_eq!(
        fs::read(&published.binding).unwrap_or_else(panic_error),
        original
    );
}

// The node peer signs the actual lifecycle transaction with the production wire codec,
// including the authenticated execution summary and lifecycle log. No CLI verifier is replaced.
fn lifecycle_effects(returned: &[u8], vm_error: &str, logs: &[Value]) -> Vec<Vec<u8>> {
    let mut summary = Vec::new();
    for number in [100_000_u64, 100_000, 3] {
        summary.extend_from_slice(&number.to_be_bytes());
    }
    summary.extend_from_slice(&1_u32.to_be_bytes());
    summary.extend_from_slice(&[0; 40]);
    summary.extend_from_slice(&Sha256::digest(vm_error.as_bytes()));
    summary.extend_from_slice(
        &u32::try_from(returned.len())
            .unwrap_or_else(panic_error)
            .to_be_bytes(),
    );
    summary.extend_from_slice(
        &u32::try_from(logs.len())
            .unwrap_or_else(panic_error)
            .to_be_bytes(),
    );
    summary.push(4);
    summary.extend_from_slice(b"uhpx");
    let mut effects = vec![summary];
    for log in logs {
        let mut body = bytes(log["address"].as_str().unwrap_or_else(panic_missing));
        let topics = log["topics"].as_array().unwrap_or_else(panic_missing);
        body.push(u8::try_from(topics.len()).unwrap_or_else(panic_error));
        for topic in topics {
            body.extend_from_slice(&bytes(topic.as_str().unwrap_or_else(panic_missing)));
        }
        body.extend_from_slice(&Sha256::digest(bytes(
            log["data"].as_str().unwrap_or_else(panic_missing),
        )));
        effects.push(body);
    }
    effects
}

fn unsigned_lifecycle_receipt(
    call: &layerx_crypto::evm_transaction::Eip1559Call,
    sender: [u8; 20],
    activity: [u8; 32],
    returned: &[u8],
    failed: bool,
    effects: &[Vec<u8>],
    key_id: [u8; 32],
) -> (Vec<u8>, u64) {
    use layerx_wire::encode::Encoder;
    let mut height = [0; 32];
    height[24..].copy_from_slice(&1000_u64.to_be_bytes());
    let sequence = call.nonce + 1;
    let previous = [0; 32];
    let resulting: [u8; 32] = Sha256::new()
        .chain_update(b"PAXEER/v1/sequence\0")
        .chain_update(previous)
        .chain_update(sequence.to_be_bytes())
        .chain_update(activity)
        .finalize()
        .into();
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&call.to);
    let mut encoder = Encoder::new(1_048_576);
    encoder
        .structure_header_version(0x5201, 2)
        .unwrap_or_else(panic_error);
    encoder.u16(2).unwrap_or_else(panic_error);
    encoder.bytes(&activity, 32).unwrap_or_else(panic_error);
    encoder.u64(sequence).unwrap_or_else(panic_error);
    for value in [previous, resulting, Sha256::digest(returned).into()] {
        encoder.bytes(&value, 32).unwrap_or_else(panic_error);
    }
    encoder.i32(i32::from(failed)).unwrap_or_else(panic_error);
    encoder
        .sequence_length(effects.len(), 512)
        .unwrap_or_else(panic_error);
    for (ordinal, body) in effects.iter().enumerate() {
        encoder.u16(12).unwrap_or_else(panic_error);
        encoder
            .u16(u16::try_from(ordinal).unwrap_or_else(panic_error))
            .unwrap_or_else(panic_error);
        encoder.u16(0).unwrap_or_else(panic_error);
        encoder
            .u8(if ordinal == 0 { 1 } else { 3 })
            .unwrap_or_else(panic_error);
        encoder.u8(0).unwrap_or_else(panic_error);
        encoder.bytes(&[0; 32], 32).unwrap_or_else(panic_error);
        encoder.bytes(body, 256).unwrap_or_else(panic_error);
    }
    encoder.u128(300_000).unwrap_or_else(panic_error);
    encoder.bytes(&height, 32).unwrap_or_else(panic_error);
    encoder.u16(12).unwrap_or_else(panic_error);
    encoder.u32(1).unwrap_or_else(panic_error);
    encoder.u32(0).unwrap_or_else(panic_error);
    encoder.u8(2).unwrap_or_else(panic_error);
    encoder
        .bytes(&Sha256::digest(b"uhpx"), 32)
        .unwrap_or_else(panic_error);
    encoder.u128(0).unwrap_or_else(panic_error);
    encoder.bytes(&from, 32).unwrap_or_else(panic_error);
    encoder.u128(1_000_000).unwrap_or_else(panic_error);
    encoder.u128(700_000).unwrap_or_else(panic_error);
    encoder.u64(call.nonce).unwrap_or_else(panic_error);
    encoder.bytes(&to, 32).unwrap_or_else(panic_error);
    encoder.u128(0).unwrap_or_else(panic_error);
    encoder.u128(0).unwrap_or_else(panic_error);
    for value in [[0; 32], [0; 32], key_id] {
        encoder.bytes(&value, 32).unwrap_or_else(panic_error);
    }
    encoder
        .u64(1_000_000 + sequence)
        .unwrap_or_else(panic_error);
    encoder.u8(0).unwrap_or_else(panic_error);
    (encoder.finish(), sequence)
}

fn produce_lifecycle_receipt(raw: &[u8], failed: bool) -> Value {
    let (call, signature) =
        layerx_crypto::evm_transaction::decode_signed(raw).unwrap_or_else(panic_error);
    let sender = layerx_crypto::evm_transaction::recover_sender(&call, &signature)
        .unwrap_or_else(panic_error);
    let activity = layerx_crypto::evm_transaction::transaction_hash(raw);
    let returned = if failed {
        vec![0xde, 0xad, 0xbe, 0xef]
    } else {
        Vec::new()
    };
    let vm_error = if failed { "execution reverted" } else { "" };
    let topic = programs::lifecycle_event_topic(&call.data).unwrap_or_else(panic_error);
    let logs = if failed {
        vec![]
    } else {
        vec![
            json!({"address":hex(&call.to),"topics":[format!("0x{topic}"),hex(&call.data[4..36])],"data":"0x","logIndex":"0x0"}),
        ]
    };
    let effects = lifecycle_effects(&returned, vm_error, &logs);
    let key = ed25519_dalek::SigningKey::from_bytes(&[0x5e; 32]);
    let key_id: [u8; 32] = Sha256::digest(key.verifying_key().as_bytes()).into();
    let (unsigned, sequence) =
        unsigned_lifecycle_receipt(&call, sender, activity, &returned, failed, &effects, key_id);
    let decoded = layerx_wire::receipt::decode(&unsigned).unwrap_or_else(panic_error);
    assert_eq!(
        layerx_wire::receipt::encode_unsigned(&decoded).unwrap_or_else(panic_error),
        unsigned
    );
    let digest = layerx_wire::hash::receipt_digest(&unsigned).unwrap_or_else(panic_error);
    let signature = key.sign(&digest).to_bytes();
    let mut canonical = unsigned;
    canonical.pop();
    canonical.push(1);
    canonical.extend_from_slice(&64_u32.to_be_bytes());
    canonical.extend_from_slice(&signature);
    assert_eq!(
        layerx_wire::receipt::encode(
            &layerx_wire::receipt::decode(&canonical).unwrap_or_else(panic_error)
        )
        .unwrap_or_else(panic_error),
        canonical
    );
    json!({"rung":"receipted","txHash":hex(&activity),"nonce":format!("0x{:x}",call.nonce),"seq":format!("0x{sequence:x}"),"height":"0x3e8","index":"0x1","timestamp":format!("0x{:x}",1_000_000+sequence),
        "status":if failed {"0x0"} else {"0x1"},"returnData":hex(&returned),"logs":logs,"gasUsed":"0x186a0","cumulativeGasUsed":"0x186a0","effectiveGasPrice":"0x3","paymentDenom":"uhpx","feeDenom":"uhpx","feeAmount":"0x493e0","sponsor":hex(&[0;20]),"from":hex(&sender),"to":hex(&call.to),"contractAddress":hex(&[0;20]),"vmError":vm_error,"keyId":hex(&key_id),"signingKey":hex(key.verifying_key().as_bytes()),"receipt":hex(&canonical)})
}

#[test]
fn lifecycle_signed_producer_receipts_distinguish_pending_success_and_revert() {
    let root = tempfile::tempdir().unwrap_or_else(panic_error);
    let policy_path = root.path().join("policy.json");
    let key = ed25519_dalek::SigningKey::from_bytes(&[0x5e; 32]);
    let key_id: [u8; 32] = Sha256::digest(key.verifying_key().as_bytes()).into();
    fs::write(&policy_path,json!({"protocol_version":2,"network_id":125,"sequencer_id":encoding::hex_encode(&key_id),"sequencer_key":encoding::hex_encode(key.verifying_key().as_bytes()),"first_batch":1000,"last_batch":1999}).to_string()).unwrap_or_else(panic_error);
    let policy = programs::read_lifecycle_policy(&policy_path).unwrap_or_else(panic_error);
    for (ordinal,fixture) in [(1,include_str!("../../../../sdk/platform/conformance/fixtures/native-program-deploy-v4.json")),(2,include_str!("../../../../sdk/platform/conformance/fixtures/native-program-upgrade-v4.json")),(7,include_str!("../../../../sdk/platform/conformance/fixtures/native-program-wind-down-tombstone-v4.json"))] {
        let fixture: Value = serde_json::from_str(fixture).unwrap_or_else(panic_error);
        let data = programs::lifecycle_calldata(ordinal,&bytes(fixture["payload_hex"].as_str().unwrap_or_else(panic_missing)),"1000").unwrap_or_else(panic_error);
        assert_eq!(hex(&data),fixture["evm"]["calldata"], "lifecycle ordinal {ordinal}");
        if ordinal == 2 {
            let repriced = programs::lifecycle_calldata(ordinal,&bytes(fixture["payload_hex"].as_str().unwrap_or_else(panic_missing)),"2000").unwrap_or_else(panic_error);
            let mut expected = data.clone();
            expected[4 + 8 * 32 + 16..4 + 9 * 32].copy_from_slice(&2000_u128.to_be_bytes());
            assert_eq!(repriced, expected, "upgrade fee must occupy its ABI word");
        }
        for outcome in ["completed","failed","pending"] {
            let failed = outcome == "failed";
            let pending = outcome == "pending";
            let raw = layerx_crypto::evm_transaction::Eip1559Call {chain_id:125,nonce:u64::from(failed),max_priority_fee_per_gas:1,max_fee_per_gas:3,gas_limit:2_000_000,to:bytes("000000000000000000000000000000000000101d").try_into().unwrap_or_else(panic_error),data:data.clone()}.sign(&[1;32]).unwrap_or_else(panic_error);
            let hash = hex(&layerx_crypto::evm_transaction::transaction_hash(&raw));
            let stored = Arc::new(Mutex::new(Value::Null));
            let node_store = stored.clone();
            let expected_raw = raw.clone();
            let node = Peer::start(move |line,_,request| {
                assert_eq!(line,"POST / HTTP/1.1");
                let result = match request["method"].as_str().unwrap_or_else(panic_missing) {
                    "eth_sendRawTransaction" => {
                        let transaction = bytes(request["params"][0].as_str().unwrap_or_else(panic_missing));
                        assert_eq!(transaction,expected_raw);
                        let receipt = produce_lifecycle_receipt(&transaction,failed);
                        let hash = receipt["txHash"].clone();
                        *node_store.lock().unwrap_or_else(panic_error) = receipt;
                        hash
                    }
                    "pax_sequencerReceipt" => {
                        let receipt = node_store.lock().unwrap_or_else(panic_error).clone();
                        assert_eq!(request["params"][0],receipt["txHash"]);
                        if pending { json!({"txHash":receipt["txHash"],"rung":"submitted","nonce":receipt["nonce"]}) } else { receipt }
                    }
                    method => panic!("unexpected lifecycle RPC {method}"),
                };
                Some(envelope_for(&request,&result))
            });
            let client = http::Client::new(&node.endpoint,None).unwrap_or_else(panic_error);
            let result = programs::submit_lifecycle_transaction_verified(&client,&raw,&policy).unwrap_or_else(panic_error);
            assert_eq!(result["state"],outcome);
            if pending {
                assert_eq!(result["rung"],"submitted");
                assert!(result.get("result_code").is_none());
                assert!(result.get("sequencer_receipt").is_none());
                continue;
            }
            assert_eq!(result["result_code"],i32::from(failed));
            assert_eq!(result["txHash"],hash);
            assert_eq!(result["vm_error"],if failed {"execution reverted"} else {""});
            assert_eq!(result["return_data"],if failed {"0xdeadbeef"} else {"0x"});
            let receipt = stored.lock().unwrap_or_else(panic_error).clone();
            assert_eq!(result["sequencer_receipt"],receipt);
            let wrong_policy = layerx_sdk::rpc_verification::ReceiptPolicy {
                protocol_version:2,network_id:125,
                sequencer:layerx_proof::inclusion::SequencerAuthorization::new(key_id,[1;32],1000,1999),
                trusted_checkpoint_context_digest:None,
            };
            assert!(programs::lifecycle_receipt_result_verified(&hash,&receipt,&wrong_policy).is_err());
            for (field,value) in [("status",json!(if failed {"0x1"} else {"0x0"})),("returnData",json!("0xff")),("gasUsed",json!("0x1")),("receipt",json!("0x00")),("signingKey",json!(hex(&[1;32])))] {
                let mut corrupted = receipt.clone(); corrupted[field]=value;
                assert!(programs::lifecycle_receipt_result_verified(&hash,&corrupted,&policy).is_err(),"must authenticate {field}");
            }
            assert!(programs::lifecycle_receipt_result_verified(&hash,&json!({"txHash":hash,"rung":"committed"}),&policy).is_err());
            assert_eq!(programs::lifecycle_receipt_result_verified(&hash,&json!({"txHash":hash,"rung":"submitted"}),&policy).unwrap_or_else(panic_error)["state"],"pending");
            assert_eq!(programs::lifecycle_receipt_result_verified(&hash,&Value::Null,&policy).unwrap_or_else(panic_error)["state"],"pending");
        }
    }
}

fn panic_error<T, E: std::fmt::Debug>(error: E) -> T {
    panic!("{error:?}")
}

fn panic_missing<T>() -> T {
    panic!("expected value")
}
