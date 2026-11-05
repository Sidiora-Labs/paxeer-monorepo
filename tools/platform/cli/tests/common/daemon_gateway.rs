use std::fs;
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::os::unix::fs::PermissionsExt as _;
use std::path::Path;
use std::process::{Child, Command, Stdio};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};

use layerx_agentd::agent_rpc;
use layerx_agentd::budget::BudgetLimiter;
use layerx_agentd::prepare::PreparationLifecycle;
use layerx_agentd::session::SessionRegistry;
use layerx_agentd::session_control::{McpInvocationIdentity, SessionControl};
use layerx_agentd::store::{Store, TenantId};
use layerx_mcp::binding::{Binding, OwnerEnvironment};
use layerx_mcp::server::catalogue;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

pub struct Gateway {
    process: Child,
    worker: Option<JoinHandle<()>>,
}

fn openssl(root: &Path, args: &[&str]) {
    let output = Command::new("openssl")
        .current_dir(root)
        .args(args)
        .output()
        .expect("OpenSSL");
    assert!(
        output.status.success(),
        "certificate producer: {}",
        String::from_utf8_lossy(&output.stderr)
    );
}

impl Gateway {
    pub fn start(root: &Path, binding_path: &Path) -> Self {
        openssl(
            root,
            &[
                "req",
                "-x509",
                "-newkey",
                "ec",
                "-pkeyopt",
                "ec_paramgen_curve:P-256",
                "-nodes",
                "-keyout",
                "ca.key",
                "-out",
                "ca.pem",
                "-days",
                "1",
                "-subj",
                "/CN=CLI daemon test CA",
                "-addext",
                "basicConstraints=critical,CA:TRUE",
            ],
        );
        openssl(
            root,
            &[
                "req",
                "-new",
                "-newkey",
                "ec",
                "-pkeyopt",
                "ec_paramgen_curve:P-256",
                "-nodes",
                "-keyout",
                "server.key",
                "-out",
                "server.csr",
                "-subj",
                "/CN=localhost",
            ],
        );
        fs::write(root.join("server.ext"), "basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:localhost\n").expect("certificate extensions");
        openssl(
            root,
            &[
                "x509",
                "-req",
                "-in",
                "server.csr",
                "-CA",
                "ca.pem",
                "-CAkey",
                "ca.key",
                "-CAcreateserial",
                "-out",
                "server.pem",
                "-days",
                "1",
                "-extfile",
                "server.ext",
            ],
        );
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
        let mut process = Command::new("python3")
            .args(["-u", "-c", include_str!("daemon_https.py")])
            .arg(root)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::inherit())
            .spawn()
            .expect("TLS listener");
        let mut reader = BufReader::new(process.stdout.take().expect("TLS listener output"));
        let mut port = String::new();
        reader.read_line(&mut port).expect("TLS bound address");
        let port: u16 = port.trim().parse().expect("TLS bound port");
        let mut document: Value = serde_json::from_slice(&fs::read(binding_path).expect("binding"))
            .expect("binding JSON");
        document["agent"]["readiness"] = json!({
            "endpoint": format!("https://localhost:{port}"),
            "gateway_key_file": root.join("gateway-key"), "trust_anchors": root.join("ca.pem"),
        });
        fs::write(
            binding_path,
            serde_json::to_vec_pretty(&document).expect("binding encoding"),
        )
        .expect("publish HTTPS binding");
        let binding = Binding::open(binding_path)
            .unwrap_or_else(|error| panic!("HTTPS binding: {}", error.detail()));
        let store = Store::open(root.join("store")).expect("daemon store");
        let mut sessions = SessionRegistry::default();
        sessions
            .restore_tenant(&store, &TenantId::new("tenant-a").expect("tenant"))
            .expect("daemon session recovery");
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
        let mut writer = process.stdin.take().expect("TLS response input");
        let worker = thread::spawn(move || loop {
            let mut length = String::new();
            if reader.read_line(&mut length).expect("TLS request length") == 0 {
                break;
            }
            let length: usize = length.trim().parse().expect("request length");
            assert!(length <= agent_rpc::MAX_BODY_BYTES);
            let mut bytes = vec![0; length];
            reader.read_exact(&mut bytes).expect("SDK request body");
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
                    writeln!(writer, "{}", response.body.len()).expect("TLS response length");
                    writer
                        .write_all(&response.body)
                        .expect("daemon response bytes");
                    writer.flush().expect("daemon response release");
                    Ok(())
                })
                .unwrap_or_else(|error| panic!("daemon durable release: {error:?}"));
        });
        Self {
            process,
            worker: Some(worker),
        }
    }
}

impl Drop for Gateway {
    fn drop(&mut self) {
        let _ = self.process.kill();
        let _ = self.process.wait();
        if let Some(worker) = self.worker.take() {
            worker.join().expect("daemon worker");
        }
    }
}
