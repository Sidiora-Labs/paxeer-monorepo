use std::fs;
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::{SocketAddrV4, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt as _;
use std::path::Path;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_agentd::agent_rpc;
use layerx_agentd::budget::BudgetLimiter;
use layerx_agentd::enrolment::readiness_tls::{
    self as tls,
    pki_types::{pem::PemObject as _, CertificateDer, PrivateKeyDer},
};
use layerx_agentd::enrolment::ReadinessDeclaration;
use layerx_agentd::prepare::PreparationLifecycle;
use layerx_agentd::session::SessionRegistry;
use layerx_agentd::session_control::{McpInvocationIdentity, SessionControl};
use layerx_agentd::store::{Store, TenantId};
use layerx_mcp::binding::{Binding, OwnerEnvironment};
use layerx_mcp::server::catalogue;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use serde_json::Value;
use sha2::{Digest as _, Sha256};

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

pub struct Gateway {
    listener: Option<TcpListener>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
    port: u16,
}

impl Gateway {
    pub fn prepare(root: &Path) -> Self {
        transport_files(root);
        Self::bind(0)
    }

    pub fn readiness(&self, root: &Path) -> ReadinessDeclaration {
        ReadinessDeclaration::new(
            &self.endpoint(),
            root.join("gateway-key"),
            Some(root.join("ca.pem")),
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "bound readiness publication inputs"))
    }

    pub fn start(root: &Path, binding_path: &Path) -> Self {
        let document: Value = serde_json::from_slice(
            &fs::read(binding_path)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "published binding")),
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "binding JSON"));
        let endpoint = document["agent"]["readiness"]["endpoint"]
            .as_str()
            .unwrap_or_else(|| {
                panic!(
                    "{}",
                    "enrolment must publish readiness before gateway serving"
                )
            });
        let port: u16 = endpoint
            .strip_prefix("https://localhost:")
            .unwrap_or_else(|| panic!("{}", "loopback HTTPS origin"))
            .parse()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "declared port"));
        assert_ne!(port, 0);
        let mut gateway = Self::bind(port);
        gateway.serve(root, binding_path);
        gateway
    }

    fn bind(port: u16) -> Self {
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

    pub fn endpoint(&self) -> String {
        format!("https://localhost:{}", self.port)
    }

    pub fn serve(&mut self, root: &Path, binding_path: &Path) {
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
        assert_eq!(document["agent"]["readiness"]["endpoint"], self.endpoint());
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
                .unwrap_or_else(|| panic!("{}", "trust path"))
        );
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
        head_sequence: super::OBSERVED_SEQUENCE,
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
                super::OBSERVED_SEQUENCE,
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
            .release_current(super::OBSERVED_SEQUENCE, |value| {
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
