mod common;

use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::os::unix::fs::{DirBuilderExt as _, PermissionsExt as _};
use std::path::{Path, PathBuf};
use std::process::{Command, Output, Stdio};
use std::sync::atomic::{AtomicU32, Ordering};
use std::thread;
use std::time::Duration;

use layerx_agentd::capability::timed::{self, NativeTimedCapabilityV1, TimedCapability};
use layerx_types::payload::{ActivityType, ModuleId};

use layerx_agentd::budget::{LimitConfig, LimitId, LimitScope};
use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
use layerx_agentd::enrolment::{
    self, BindingMode, BindingPublisher, DaemonSurface, EnrolmentRequest, PublishedBinding,
};
use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::session::{SessionId, SessionRegistry};
use layerx_agentd::store::{Store, TenantId};
use layerx_mcp::server::{catalogue, ToolKind};
use layerx_types::ids::Did;
use layerx_types::verify::VerificationLevel;
use serde_json::Value;

static SEQUENCE: AtomicU32 = AtomicU32::new(0);
const OBSERVED_SEQUENCE: u64 = 120;

struct BoundaryIdentity(CoreIdentity);

impl IdentityResolver for BoundaryIdentity {
    fn resolve(&mut self, _did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(self.0.clone()))
    }
}

/// Creates one canonical, operator-private root for a single installation journey.
fn isolated(label: &str) -> PathBuf {
    let sequence = SEQUENCE.fetch_add(1, Ordering::Relaxed);
    let base = std::env::temp_dir();
    let canonical = fs::canonicalize(&base).unwrap_or(base);
    let root = canonical.join(format!(
        "layerx-install-daemon-{label}-{}-{sequence}",
        std::process::id()
    ));
    fs::create_dir_all(&root).unwrap_or_else(|error| panic!("root {label}: {error}"));
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700))
        .unwrap_or_else(|error| panic!("root mode {label}: {error}"));
    root
}

/// Enrols the agent daemon exactly as a running daemon does: a real capability, a real identity
/// carrying its grant, a real session opened under that grant, and the binding document the
/// served path opens, written beside the CLI configuration.
fn enrol_published(
    root: &Path,
    endpoint: &str,
    bearer: &str,
    readiness: Option<&str>,
) -> PublishedBinding {
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
    let surface = DaemonSurface::new(endpoint, bearer.to_owned(), [0xcc; 32])
        .unwrap_or_else(|error| panic!("surface: {error}"));
    let surface = if let Some(readiness) = readiness {
        surface.with_readiness(
            layerx_agentd::enrolment::ReadinessDeclaration::new(
                readiness,
                root.join("gateway-key"),
                Some(root.join("ca.pem")),
            )
            .expect("real publisher readiness inputs"),
        )
    } else {
        surface
    };
    let publisher = BindingPublisher::new(
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
    .unwrap_or_else(|error| panic!("publisher: {error}"));
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
    let published = enrolment::enrol(&mut store, &mut sessions, &identity, request, &publisher)
        .unwrap_or_else(|error| panic!("enrol: {error}"));
    assert_eq!(
        fs::metadata(&published.binding)
            .expect("published binding metadata")
            .permissions()
            .mode()
            & 0o777,
        0o600
    );
    published
}

fn enrol(root: &Path, endpoint: &str, bearer: &str, readiness: &str) -> PublishedBinding {
    let published = enrol_published(root, endpoint, bearer, Some(readiness));
    let bytes = fs::read(&published.binding).expect("read enrolled binding");
    let document: Value = serde_json::from_slice(&bytes).expect("enrolled binding JSON");
    assert_eq!(document["core_height"], 50);
    assert!(document.get("core_sequence").is_none());
    assert_eq!(document["agent"]["readiness"]["endpoint"], readiness);
    let mut store = Store::open(root.join("store")).expect("native capability store");
    let tenant = TenantId::new("tenant-a").expect("native tenant");
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
                    .expect("native activity")
                    .into()
            })
            .collect(),
        purpose_commitments: BTreeSet::new(),
        spend_ceilings: BTreeMap::new(),
    };
    store
        .put_local(
            timed::native_record_key(&tenant, &[9; 32]).expect("native key"),
            native.encode().expect("native capability"),
        )
        .expect("persist native grant");
    drop(store);
    let binding = layerx_mcp::binding::Binding::open(&published.binding)
        .unwrap_or_else(|error| panic!("current binding: {}", error.detail()));
    assert_eq!(binding.core_height(), 50);
    published
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

fn install(root: &Path, arguments: &[&str]) -> Output {
    layerx(root)
        .args(["--json", "install", "mcp"])
        .args(arguments)
        .output()
        .unwrap_or_else(|error| panic!("real layerx executable should start: {error}"))
}

fn success(output: &Output) -> Value {
    assert!(
        output.status.success(),
        "stderr: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    let value = common::envelope(output);
    assert_eq!(value["ok"], true);
    assert!(value["kind"].is_string());
    assert!(value["message"].is_string());
    assert!(value.get("data").is_some());
    value
}

/// Answers the daemon's verified balance read for any authorized loopback caller.
fn agent_daemon(bearer: String) -> String {
    let listener =
        TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("agent listener: {error}"));
    let endpoint = listener
        .local_addr()
        .unwrap_or_else(|error| panic!("agent address: {error}"))
        .to_string();
    thread::spawn(move || loop {
        let Ok((mut stream, _)) = listener.accept() else {
            return;
        };
        let mut request = Vec::new();
        let mut chunk = [0_u8; 1_024];
        while !request.windows(4).any(|window| window == b"\r\n\r\n") {
            match stream.read(&mut chunk) {
                Ok(0) | Err(_) => break,
                Ok(count) => request.extend_from_slice(&chunk[..count]),
            }
        }
        let text = String::from_utf8_lossy(&request).into_owned();
        let authorized = text
            .lines()
            .any(|header| header.strip_prefix("Authorization: Bearer ") == Some(bearer.as_str()));
        let path = text
            .lines()
            .next()
            .and_then(|line| line.split_ascii_whitespace().nth(1))
            .unwrap_or_default()
            .to_owned();
        let (status, body) = if authorized {
            match path
                .strip_prefix("/v1/programs/")
                .and_then(|value| value.strip_suffix("/balances"))
            {
                Some(program) => (200, balances(program)),
                None => (404, "{\"error\":\"not_found\"}".to_owned()),
            }
        } else {
            (401, "{\"error\":\"unauthorized\"}".to_owned())
        };
        let reason = if status < 300 { "OK" } else { "Refused" };
        let header = format!(
            "HTTP/1.1 {status} {reason}\r\nContent-Type: application/json\r\nCache-Control: no-store\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
            body.len()
        );
        let _ = stream
            .write_all(header.as_bytes())
            .and_then(|()| stream.write_all(body.as_bytes()))
            .and_then(|()| stream.flush());
    });
    endpoint
}

fn balances(program: &str) -> String {
    format!(
        "{{\"program\":\"{program}\",\"lifecycle\":\"active\",\"accounts\":[{{\"account\":\"{}\",\"asset\":\"{}\",\"amount\":\"7\",\"frozen\":false}}],\"freshness\":{{\"observed_sequence\":{OBSERVED_SEQUENCE},\"observed_at\":1,\"receipt_digest\":\"{}\",\"state_root\":\"{}\",\"valid_through\":400}}}}",
        "11".repeat(32),
        "22".repeat(32),
        "33".repeat(32),
        "44".repeat(32)
    )
}

fn strings(value: Option<&Value>) -> Vec<String> {
    value
        .and_then(Value::as_array)
        .unwrap_or_else(|| panic!("string array absent"))
        .iter()
        .map(|item| {
            item.as_str()
                .unwrap_or_else(|| panic!("string expected"))
                .to_owned()
        })
        .collect()
}

fn assert_installed_catalogue(root: &Path, binding_path: &Path, read_only: bool) {
    let mut command = layerx(root);
    command
        .args(["mcp", "serve", "--daemon-binding"])
        .arg(binding_path);
    if read_only {
        command.arg("--read-only");
    }
    let mut server = command
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap_or_else(|error| panic!("installed server should start: {error}"));
    let program = "cc".repeat(32);
    {
        let mut stdin = server
            .stdin
            .take()
            .unwrap_or_else(|| panic!("server stdin absent"));
        let mut requests = vec![
            "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\"}".to_owned(),
            "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\"}".to_owned(),
            "{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}".to_owned(),
            format!(
                "{{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{{\"name\":\"balance.get\",\"arguments\":{{\"program\":\"{program}\"}}}}}}"
            ),
        ];
        if read_only {
            requests.push(r#"{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"activity.submit","arguments":{}}}"#.to_owned());
        }
        for request in requests {
            writeln!(stdin, "{request}").unwrap_or_else(|error| panic!("request: {error}"));
        }
    }
    let output = server
        .wait_with_output()
        .unwrap_or_else(|error| panic!("server exit: {error}"));
    assert!(
        output.status.success(),
        "stderr: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    let responses = String::from_utf8_lossy(&output.stdout)
        .lines()
        .map(|line| {
            serde_json::from_str::<Value>(line).unwrap_or_else(|error| panic!("response: {error}"))
        })
        .collect::<Vec<Value>>();
    assert_eq!(responses.len(), if read_only { 4 } else { 3 });
    if read_only {
        assert_eq!(
            responses[3].pointer("/error/code").and_then(Value::as_i64),
            Some(-32602)
        );
        assert!(responses[3]
            .pointer("/error/message")
            .and_then(Value::as_str)
            .expect("read-only refusal")
            .contains("read-only"));
    }
    assert_eq!(
        responses[0]
            .pointer("/result/_meta/layerx~1binding")
            .and_then(Value::as_str),
        Some("agent-daemon")
    );
    assert_eq!(
        responses[0]
            .pointer("/result/_meta/layerx~1deployment_mode")
            .and_then(Value::as_str),
        Some(if read_only { "read-only" } else { "full" })
    );
    assert_eq!(
        responses[1]
            .pointer("/result/tools")
            .and_then(Value::as_array)
            .map(Vec::len),
        Some(
            catalogue()
                .iter()
                .filter(|tool| !read_only || tool.kind == ToolKind::Read)
                .count()
        )
    );
    let served = responses[1]
        .pointer("/result/tools")
        .and_then(Value::as_array)
        .expect("served tools");
    let expected = catalogue()
        .iter()
        .filter(|tool| !read_only || tool.kind == ToolKind::Read)
        .map(|tool| tool.name)
        .collect::<BTreeSet<_>>();
    let names = served
        .iter()
        .map(|tool| tool["name"].as_str().expect("tool name"))
        .collect::<BTreeSet<_>>();
    assert_eq!(names, expected);
    assert_eq!(
        responses[0]
            .pointer("/result/_meta/layerx~1mutations_reachable")
            .and_then(Value::as_bool),
        Some(!read_only)
    );
    assert_eq!(
        responses[2]
            .pointer("/result/isError")
            .and_then(Value::as_bool),
        Some(false)
    );
    assert_eq!(
        responses[2]
            .pointer("/result/structuredContent/result/program")
            .and_then(Value::as_str),
        Some(program.as_str())
    );
    assert_eq!(
        responses[2]
            .pointer("/result/structuredContent/result/freshness/observed_sequence")
            .and_then(Value::as_u64),
        Some(OBSERVED_SEQUENCE)
    );
}

#[test]
fn a_fresh_install_serves_the_catalogue_through_the_daemon_without_a_gateway_key() {
    let root = isolated("journey");
    let bearer = "d".repeat(48);
    let endpoint = agent_daemon(bearer.clone());
    daemon_gateway::transport_files(&root);
    let mut gateway = daemon_gateway::Gateway::start(&root, 0);
    let readiness = gateway.endpoint();
    let published = enrol(&root, &endpoint, &bearer, &readiness);
    let original_binding = fs::read(&published.binding).expect("unchanged publisher bytes");
    gateway.serve(&root, &published.binding);
    let binding_path = root.join("mcp").join("binding.json");
    assert_eq!(published.binding, binding_path);

    let first = success(&install(&root, &["--host", "layerx"]));
    assert_eq!(first.pointer("/ok").and_then(Value::as_bool), Some(true));
    assert_eq!(
        first.pointer("/kind").and_then(Value::as_str),
        Some("install.mcp")
    );
    let data = first
        .pointer("/data")
        .unwrap_or_else(|| panic!("data absent"));
    assert_eq!(
        data.pointer("/authorization").and_then(Value::as_str),
        Some("agent-daemon")
    );
    assert_eq!(
        data.pointer("/deployment_mode").and_then(Value::as_str),
        Some("full")
    );
    assert_eq!(
        data.pointer("/daemon_binding/path").and_then(Value::as_str),
        binding_path.to_str()
    );
    assert_eq!(
        data.pointer("/daemon_binding/tenant")
            .and_then(Value::as_str),
        Some("tenant-a")
    );
    assert_eq!(
        data.pointer("/daemon_binding/agent_endpoint")
            .and_then(Value::as_str),
        Some(endpoint.as_str())
    );
    assert_eq!(
        data.pointer("/daemon_binding/session_generation")
            .and_then(Value::as_u64),
        Some(published.session_generation)
    );
    assert_eq!(
        strings(data.pointer("/server/args")),
        vec![
            "mcp".to_owned(),
            "serve".to_owned(),
            "--daemon-binding".to_owned(),
            binding_path.display().to_string(),
        ]
    );
    assert_eq!(
        data.pointer("/server/env")
            .and_then(Value::as_object)
            .map(serde_json::Map::len),
        Some(0)
    );
    assert!(data.get("credentials").is_none());
    assert!(data.get("environment").is_none());
    assert!(data.get("account_binding").is_none());
    assert_eq!(
        data.pointer("/tools")
            .and_then(Value::as_array)
            .map(Vec::len),
        Some(catalogue().len())
    );
    assert_eq!(
        data.pointer("/changed").and_then(Value::as_bool),
        Some(true)
    );
    let rendered = serde_json::to_string(&first).unwrap_or_else(|error| panic!("render: {error}"));
    assert!(!rendered.contains(&bearer));
    assert!(!rendered.contains("LAYERX_GATEWAY_KEY_ID"));

    let registration = fs::read_to_string(root.join("mcp.json"))
        .unwrap_or_else(|error| panic!("registration file: {error}"));
    assert!(!registration.contains(&bearer));
    let registered: Value =
        serde_json::from_str(&registration).unwrap_or_else(|error| panic!("mcp.json: {error}"));
    assert_eq!(
        strings(registered.pointer("/mcpServers/layerx/args")),
        strings(data.pointer("/server/args"))
    );
    assert_eq!(
        registered
            .pointer("/mcpServers/layerx/env")
            .and_then(Value::as_object)
            .map(serde_json::Map::len),
        Some(0)
    );

    let second = success(&install(&root, &["--host", "layerx"]));
    assert_eq!(
        second.pointer("/data/changed").and_then(Value::as_bool),
        Some(false)
    );

    assert_installed_catalogue(&root, &binding_path, false);
    let port = gateway.port();
    drop(gateway);
    let store = Store::open(root.join("store")).expect("reopen daemon state");
    let mut sessions = SessionRegistry::default();
    let tenant = TenantId::new("tenant-a").expect("tenant");
    sessions
        .restore_tenant(&store, &tenant)
        .expect("restore enrolled authority");
    assert_eq!(
        sessions
            .get(&tenant, published.session_id)
            .expect("enrolled session survived")
            .generation,
        published.session_generation
    );
    drop(store);
    assert_eq!(
        fs::read(&binding_path).expect("reopen same document"),
        original_binding
    );
    let mut restarted = daemon_gateway::Gateway::start(&root, port);
    restarted.serve(&root, &binding_path);
    let reopened = success(&install(&root, &["--host", "layerx"]));
    assert_eq!(reopened["data"]["changed"], false);
    assert_installed_catalogue(&root, &binding_path, false);
    assert_eq!(
        fs::read(&binding_path).expect("served document unchanged"),
        original_binding
    );
    drop(restarted);
    let _ = fs::remove_dir_all(root);
}

#[test]
fn an_explicit_binding_path_narrows_the_installed_surface_to_read_only() {
    let root = isolated("read-only");
    let bearer = "e".repeat(48);
    let endpoint = agent_daemon(bearer.clone());
    daemon_gateway::transport_files(&root);
    let mut gateway = daemon_gateway::Gateway::start(&root, 0);
    let readiness = gateway.endpoint();
    let published = enrol(&root, &endpoint, &bearer, &readiness);
    let original_binding = fs::read(&published.binding).expect("unchanged publisher bytes");
    gateway.serve(&root, &published.binding);
    let relocated = root.join("elsewhere");
    fs::DirBuilder::new()
        .mode(0o700)
        .create(&relocated)
        .unwrap_or_else(|error| panic!("relocated: {error}"));
    let binding_path = relocated.join("binding.json");
    fs::copy(&published.binding, &binding_path).unwrap_or_else(|error| panic!("copy: {error}"));
    assert_eq!(
        fs::metadata(&binding_path)
            .expect("relocated binding metadata")
            .permissions()
            .mode()
            & 0o777,
        0o600
    );
    let installed = success(&install(
        &root,
        &[
            "--host",
            "layerx",
            "--read-only",
            "--daemon-binding",
            binding_path
                .to_str()
                .unwrap_or_else(|| panic!("binding path is UTF-8")),
        ],
    ));
    let data = installed
        .pointer("/data")
        .unwrap_or_else(|| panic!("data absent"));
    assert_eq!(
        data.pointer("/deployment_mode").and_then(Value::as_str),
        Some("read-only")
    );
    assert_eq!(
        data.pointer("/daemon_binding/declared_mode")
            .and_then(Value::as_str),
        Some("full")
    );
    assert_eq!(
        data.pointer("/daemon_binding/path").and_then(Value::as_str),
        binding_path.to_str()
    );
    let arguments = strings(data.pointer("/server/args"));
    assert_eq!(arguments.last().map(String::as_str), Some("--read-only"));
    assert_eq!(arguments.get(3).map(String::as_str), binding_path.to_str());
    let tools = data
        .pointer("/tools")
        .and_then(Value::as_array)
        .unwrap_or_else(|| panic!("tools absent"));
    assert_eq!(
        tools.len(),
        catalogue()
            .iter()
            .filter(|tool| tool.kind == ToolKind::Read)
            .count()
    );
    assert_installed_catalogue(&root, &binding_path, true);
    assert_eq!(
        fs::read(&binding_path).expect("relocated publisher document unchanged"),
        original_binding
    );
    drop(gateway);
    let _ = fs::remove_dir_all(root);
}

#[test]
fn installation_refuses_before_any_registration_when_the_daemon_has_not_enrolled() {
    let root = isolated("unenrolled");
    let output = install(&root, &["--host", "layerx"]);
    assert!(!output.status.success());
    let stderr = String::from_utf8_lossy(&output.stderr).into_owned();
    let expected = root.join("mcp").join("binding.json").display().to_string();
    assert!(stderr.contains(&expected), "stderr: {stderr}");
    assert!(
        stderr.contains("agent-daemon enrolment"),
        "stderr: {stderr}"
    );
    assert!(!root.join("mcp.json").exists());
    let _ = fs::remove_dir_all(root);
}

#[test]
fn a_retired_core_sequence_binding_is_refused_before_installation_or_serving() {
    let root = isolated("retired-binding");
    let published = enrol_published(&root, "127.0.0.1:1", &"f".repeat(48), None);
    let mut retired: Value =
        serde_json::from_slice(&fs::read(&published.binding).expect("published binding"))
            .expect("binding JSON");
    let fields = retired.as_object_mut().expect("binding object");
    let height = fields
        .remove("core_height")
        .expect("current publisher height");
    fields.insert("core_sequence".into(), height);
    fs::write(
        &published.binding,
        serde_json::to_vec(&retired).expect("explicit retired fixture"),
    )
    .expect("write negative fixture");
    let output = install(&root, &["--host", "layerx"]);
    assert!(!output.status.success());
    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(
        stderr.contains("mcp.binding_retired_key"),
        "stderr: {stderr}"
    );
    assert!(!root.join("mcp.json").exists());
    let served = layerx(&root)
        .args(["--json", "mcp", "serve", "--daemon-binding"])
        .arg(&published.binding)
        .stdin(Stdio::null())
        .output()
        .expect("start MCP server with retired binding");
    assert!(!served.status.success());
    assert!(served.stdout.is_empty());
    let stderr = String::from_utf8_lossy(&served.stderr);
    assert!(
        stderr.contains("mcp.binding_retired_key"),
        "stderr: {stderr}"
    );
    assert!(!root.join("mcp.json").exists());
    fs::remove_dir_all(root).expect("remove retired binding profile");
}

mod daemon_gateway {

    use layerx_agentd::enrolment::readiness_tls::{
        self as tls,
        pki_types::{pem::PemObject as _, CertificateDer, PrivateKeyDer},
    };
    use layerx_agentd::{
        agent_rpc,
        budget::BudgetLimiter,
        prepare::PreparationLifecycle,
        session::SessionRegistry,
        session_control::{McpInvocationIdentity, SessionControl},
        store::{Store, TenantId},
    };
    use layerx_mcp::{
        binding::{Binding, OwnerEnvironment},
        server::catalogue,
    };
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
    use serde_json::Value;
    use sha2::{Digest as _, Sha256};
    use std::fs;
    use std::io::{BufRead as _, BufReader, Read as _, Write as _};
    use std::net::{SocketAddrV4, TcpListener, TcpStream};
    use std::os::unix::fs::PermissionsExt as _;
    use std::path::Path;
    use std::sync::atomic::{AtomicBool, Ordering};
    use std::sync::{Arc, Mutex};
    use std::thread::{self, JoinHandle};
    use std::time::Duration;
    const OBSERVED_HEIGHT: u64 = super::OBSERVED_SEQUENCE;
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

    pub(super) fn transport_files(root: &Path) {
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

    pub(super) struct Gateway {
        listener: Option<TcpListener>,
        stop: Arc<AtomicBool>,
        worker: Option<JoinHandle<()>>,
        port: u16,
    }

    impl Gateway {
        pub(super) fn port(&self) -> u16 {
            self.port
        }

        pub(super) fn start(_root: &Path, port: u16) -> Self {
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

        pub(super) fn endpoint(&self) -> String {
            format!("https://localhost:{}", self.port)
        }

        pub(super) fn serve(&mut self, root: &Path, binding_path: &Path) {
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
                let connection =
                    tls::ServerConnection::new(Arc::clone(&tls)).expect("TLS connection");
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
                    ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Asset, &activities)
                        .expect("native module")])
                    .expect("native registry");
                let environment = OwnerEnvironment {
                    registry,
                    core_time_ms: 120,
                    head_sequence: OBSERVED_HEIGHT,
                    native_effect_profile: true,
                    native_send_profile: true,
                };
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
                        let value =
                            value.unwrap_or_else(|error| panic!("real daemon read: {error:?}"));
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
}
