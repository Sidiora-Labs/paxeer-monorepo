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

const FIXTURE_CA: &str = r#"-----BEGIN CERTIFICATE-----
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
"#;

const FIXTURE_CERT: &str = r#"-----BEGIN CERTIFICATE-----
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
"#;

const FIXTURE_KEY: &str = r#"-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg9a2umZJLxAShe3Ai
GrazueSKt9VKHVVG97limBRZ2SuhRANCAAQYmyn55dQD0QFi8yG5NR1q8Mg+LQqu
Iramz66I4GpdnANGjPRWT5zF3C+cGTZ6RMWU+j/UaVRQrGYcCLXlteje
-----END PRIVATE KEY-----
"#;

fn transport_files(root: &Path) {
    fs::write(root.join("ca.pem"), FIXTURE_CA).expect("fixture trust anchor");
    let mut secret = [0; 32];
    getrandom::fill(&mut secret).expect("gateway credential entropy");
    let secret = secret
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<String>();
    fs::write(
        root.join("gateway-key"),
        format!("cli-daemon:lxp_live_{secret}"),
    )
    .expect("gateway credential");
    fs::set_permissions(root.join("gateway-key"), fs::Permissions::from_mode(0o600))
        .expect("protected gateway credential");
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
        .expect("bound readiness publication inputs")
    }

    pub fn start(root: &Path, binding_path: &Path) -> Self {
        let document: Value =
            serde_json::from_slice(&fs::read(binding_path).expect("published binding"))
                .expect("binding JSON");
        let endpoint = document["agent"]["readiness"]["endpoint"]
            .as_str()
            .expect("enrolment must publish readiness before gateway serving");
        let port: u16 = endpoint
            .strip_prefix("https://localhost:")
            .expect("loopback HTTPS origin")
            .parse()
            .expect("declared port");
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
        .expect("loopback listener socket");
        net::sockopt::set_socket_reuseaddr(&socket, true).expect("restartable listener");
        net::bind(
            &socket,
            &SocketAddrV4::new(std::net::Ipv4Addr::LOCALHOST, port),
        )
        .expect("bind declared origin");
        net::listen(&socket, 16).expect("listen at declared origin");
        let listener = TcpListener::from(socket);
        let bound = listener.local_addr().expect("bound address").port();
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
        let listener = self.listener.take().expect("one Rust handler per listener");
        let stop = Arc::clone(&self.stop);
        let expected_authorization = format!(
            "LayerX-Key {}",
            fs::read_to_string(root.join("gateway-key"))
                .expect("gateway key")
                .trim()
        );
        let tls = Arc::new(
            tls::ServerConfig::builder_with_provider(Arc::new(
                tls::crypto::ring::default_provider(),
            ))
            .with_safe_default_protocol_versions()
            .expect("TLS protocol versions")
            .with_no_client_auth()
            .with_single_cert(
                CertificateDer::pem_slice_iter(FIXTURE_CERT.as_bytes())
                    .collect::<Result<Vec<_>, _>>()
                    .expect("fixture certificate"),
                PrivateKeyDer::from_pem_slice(FIXTURE_KEY.as_bytes()).expect("fixture key"),
            )
            .expect("TLS server identity"),
        );
        let store = Store::open(root.join("store")).expect("daemon store");
        let mut sessions = SessionRegistry::default();
        sessions
            .restore_tenant(&store, &TenantId::new("tenant-a").expect("tenant"))
            .expect("daemon session recovery");
        let document: Value =
            serde_json::from_slice(&fs::read(binding_path).expect("published binding"))
                .expect("binding JSON");
        assert_eq!(document["agent"]["readiness"]["endpoint"], self.endpoint());
        assert_eq!(
            document["agent"]["readiness"]["gateway_key_file"],
            root.join("gateway-key").to_str().expect("key path")
        );
        assert_eq!(
            document["agent"]["readiness"]["trust_anchors"],
            root.join("ca.pem").to_str().expect("trust path")
        );
        let limit = &document["limit"];
        let budgets = BudgetLimiter::new(vec![layerx_agentd::budget::LimitConfig {
            id: layerx_agentd::budget::LimitId([0x0a; 16]),
            name: limit["name"].as_str().expect("limit name").into(),
            scope: layerx_agentd::budget::LimitScope::Tenant([1; 32]),
            ceiling: 1_000,
            consumed: 0,
        }])
        .expect("daemon limits");
        let control = SessionControl::new(
            Arc::new(Mutex::new(store)),
            sessions,
            Arc::new(PreparationLifecycle::default()),
            Arc::new(budgets),
        );
        let worker = thread::spawn(move || loop {
            let (socket, _) = listener.accept().expect("loopback connection");
            if stop.load(Ordering::Acquire) {
                break;
            }
            socket
                .set_read_timeout(Some(Duration::from_secs(5)))
                .expect("request deadline");
            socket
                .set_write_timeout(Some(Duration::from_secs(5)))
                .expect("response deadline");
            let connection = tls::ServerConnection::new(Arc::clone(&tls)).expect("TLS connection");
            let mut reader = BufReader::new(tls::StreamOwned::new(connection, socket));
            let mut line = String::new();
            reader.read_line(&mut line).expect("HTTP request line");
            assert_eq!(line, "POST /v1/agent/rpc HTTP/1.1\r\n");
            let mut length = None;
            let mut authorization = None;
            let mut header_bytes = line.len();
            loop {
                line.clear();
                assert!(reader.read_line(&mut line).expect("HTTP header") > 0);
                header_bytes += line.len();
                assert!(header_bytes <= 32_768);
                if line == "\r\n" {
                    break;
                }
                let (name, value) = line.split_once(':').expect("header field");
                if name.eq_ignore_ascii_case("content-length") {
                    assert!(length.is_none());
                    length = Some(value.trim().parse::<usize>().expect("request length"));
                } else if name.eq_ignore_ascii_case("authorization") {
                    assert!(authorization.is_none());
                    authorization = Some(value.trim().to_owned());
                }
            }
            let length = length.expect("bounded SDK request");
            assert!(length > 0 && length <= agent_rpc::MAX_BODY_BYTES);
            let mut bytes = vec![0; length];
            reader.read_exact(&mut bytes).expect("SDK request body");
            let writer = reader.get_mut();
            if authorization.as_deref() != Some(expected_authorization.as_str()) {
                write_response(
                    writer,
                    agent_rpc::refusal(
                        401,
                        layerx_sdk::programs::AgentErrorClass::PolicyRefusal,
                        "gateway.authentication",
                    ),
                );
                continue;
            }
            let invocation = agent_rpc::decode_mcp_invocation(&bytes)
                .unwrap_or_else(|_| panic!("SDK envelope must decode"))
                .expect("MCP operation");
            assert!(binding.matches_session(&invocation.credential));
            let tool = if invocation.tool == "mcp.describe" {
                "mcp.describe"
            } else {
                catalogue()
                    .iter()
                    .find(|tool| tool.name == invocation.tool)
                    .expect("catalogued invocation")
                    .name
            };
            let digest = Sha256::new()
                .chain_update(b"LXP/mcp/daemon-invocation/v1\0")
                .chain_update((tool.len() as u64).to_be_bytes())
                .chain_update(tool.as_bytes())
                .chain_update(serde_json::to_vec(&invocation.arguments).expect("arguments"))
                .finalize()
                .into();
            let identity = McpInvocationIdentity {
                tool_name: tool,
                idempotency_key: invocation.idempotency_key,
                request_digest: digest,
            };
            let activities = [1, 5, 6, 7, 10].map(|ordinal| {
                ActivityType::new(ModuleId::Asset, ordinal).expect("native activity")
            });
            let registry =
                ModuleRegistry::new(&[
                    ModuleRegistration::new(ModuleId::Asset, &activities).expect("native module")
                ])
                .expect("native registry");
            let environment = OwnerEnvironment {
                registry,
                core_time_ms: 120,
                head_sequence: super::OBSERVED_SEQUENCE,
                native_effect_profile: true,
                native_send_profile: true,
            };
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
                    write_response(writer, response);
                    Ok(())
                })
                .unwrap_or_else(|error| panic!("daemon durable release: {error:?}"));
        });

        self.worker = Some(worker);
    }
}

fn write_response(
    writer: &mut tls::StreamOwned<tls::ServerConnection, TcpStream>,
    response: agent_rpc::AgentRpcResponse,
) {
    let status = response.status;
    let reason = if status == 200 { "OK" } else { "Unauthorized" };
    write!(writer, "HTTP/1.1 {status} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.body.len()).expect("HTTP response headers");
    writer
        .write_all(&response.body)
        .expect("daemon response bytes");
    writer.conn.send_close_notify();
    writer.flush().expect("daemon response release");
}

impl Drop for Gateway {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = TcpStream::connect((std::net::Ipv4Addr::LOCALHOST, self.port));
            worker.join().expect("daemon handler");
        }
    }
}
