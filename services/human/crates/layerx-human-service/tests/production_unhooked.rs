#[allow(dead_code)]
#[path = "../../layerx-human-kms/tests/support/attestor_cluster.rs"]
mod cluster;

use std::collections::{BTreeMap, BTreeSet};
use std::env;
use std::ffi::OsString;
use std::fs::{self, DirBuilder};
use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::os::unix::fs::{DirBuilderExt as _, PermissionsExt as _};
use std::os::unix::net::UnixStream;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};
use layerx_agentd::budget::{LimitConfig, LimitId, LimitScope};
use layerx_agentd::human::{HumanListenerConfig, HumanPeer, HumanUnixServer};
use layerx_agentd::human_runtime::{
    HumanAuthorityBoundary, ProductionHumanOperations, RemoteHumanAuthority, SharedAgentOwner,
    UnifiedAgentOwner,
};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::session_keys::SessionKeyRegistry;
use layerx_agentd::store::{Store, TenantId};
use layerx_client::runtime_clock::RuntimeClock;
use layerx_client::sequencer::SequencerEndpoint;
use layerx_human_service::server::production_components::{
    validate_production_configuration, PRODUCTION_OPERATIONS,
};
use layerx_human_service::server::schema::ApiSchema;
use layerx_human_service::server::{ProductionComponents, ProductionComponentsConfig};
use layerx_human_service::store::TenancyMap;
use layerx_types::clock::Deadline;
use layerx_types::payload::{ActivityType, ModuleId};
use native_tls::{Identity, TlsAcceptor};
use serde_json::{json, Value};

static ENVIRONMENT: Mutex<()> = Mutex::new(());
type TestResult = Result<(), Box<dyn std::error::Error>>;

const RETIRED_OPERATIONS: [&str; 11] = [
    "binding.statement",
    "binding.submit",
    "binding.status",
    "binding.rebind.action",
    "binding.rebind",
    "deposit.start",
    "deposit.confirm",
    "withdraw.start",
    "withdraw.claim",
    "exit.start",
    "exit.eligibility",
];

struct Environment {
    root: PathBuf,
    saved: Vec<(OsString, Option<OsString>)>,
    cluster: Option<cluster::Cluster>,
    _events_listener: TcpListener,
}

impl Environment {
    fn new() -> Result<Self, Box<dyn std::error::Error>> {
        let mut entropy = [0; 16];
        getrandom::fill(&mut entropy).map_err(|error| error.to_string())?;
        let root = fs::canonicalize(env::temp_dir())?.join(format!(
            "production-unhooked-{:032x}",
            u128::from_be_bytes(entropy)
        ));
        DirBuilder::new().mode(0o700).create(&root)?;
        let events_listener = TcpListener::bind("127.0.0.1:0")?;
        let events_url = format!("https://localhost:{}", events_listener.local_addr()?.port());
        let mut environment = Self {
            root,
            saved: Vec::new(),
            cluster: None,
            _events_listener: events_listener,
        };
        for (name, value) in env::vars_os() {
            if name.to_string_lossy().starts_with("LAYERX_HUMAN_") {
                environment.saved.push((name.clone(), Some(value)));
                env::remove_var(name);
            }
        }
        for (variable, directory) in [
            ("LAYERX_HUMAN_STORE_ROOT", "store"),
            ("LAYERX_HUMAN_CUSTODY_ROOT", "custody"),
            ("LAYERX_HUMAN_AUTH_INDEX_ROOT", "auth-index"),
        ] {
            let path = environment.root.join(directory);
            DirBuilder::new().mode(0o700).create(&path)?;
            environment.set(variable, path.into_os_string());
        }
        let tenancy = TenancyMap::new([])?;
        let digest = tenancy.install(&environment.root.join("store"))?;
        let activity = ActivityType::new(ModuleId::Asset, 5)
            .map_err(|error| format!("asset send activity: {error:?}"))?;
        let cluster = cluster::Cluster::start_with_kernel_policy(
            &[activity.value().to_string()],
            &json!({"version": 1, "defaults": {
                "modules": {"asset": [activity.ordinal()]},
                "caps": {"native": {"per_operation": "1000"}}
            }}),
        )?;
        for (name, value) in loader_environment(&environment.root, &cluster, digest.bytes())? {
            environment.set(name, value);
        }
        let mut events_secret = [0; 32];
        getrandom::fill(&mut events_secret).map_err(|error| error.to_string())?;
        write_private(
            &environment.root.join("events-token"),
            URL_SAFE_NO_PAD.encode(events_secret).as_bytes(),
        )?;
        getrandom::fill(&mut events_secret).map_err(|error| error.to_string())?;
        write_private(
            &environment.root.join("events-identity-password"),
            URL_SAFE_NO_PAD.encode(events_secret).as_bytes(),
        )?;
        cluster::openssl(
            &environment.root,
            &[
                "pkcs12",
                "-export",
                "-in",
                &cluster.pki.path("gateway.pem").display().to_string(),
                "-inkey",
                &cluster.pki.path("gateway.key").display().to_string(),
                "-passout",
                "file:events-identity-password",
                "-out",
                "events-client.p12",
            ],
        )?;
        fs::set_permissions(
            environment.root.join("events-client.p12"),
            fs::Permissions::from_mode(0o600),
        )?;
        for prefix in [
            "LAYERX_EVENTS_JOURNEY",
            "LAYERX_EVENTS_APPROVAL",
            "LAYERX_EVENTS_WEBHOOKS",
        ] {
            environment.set(&format!("{prefix}_UPSTREAM_URL"), events_url.clone());
            for (suffix, file) in [
                ("CA_DER", "attestor-ca.der"),
                ("TOKEN_FILE", "events-token"),
                ("CLIENT_IDENTITY_PKCS12", "events-client.p12"),
                ("CLIENT_IDENTITY_PASSWORD_FILE", "events-identity-password"),
            ] {
                environment.set(
                    &format!("{prefix}_UPSTREAM_{suffix}"),
                    environment.root.join(file).into_os_string(),
                );
            }
        }
        environment.cluster = Some(cluster);
        write_private(
            &environment.root.join("purpose-catalog.json"),
            &serde_json::to_vec(&purpose_catalog())?,
        )?;
        Ok(environment)
    }

    fn set(&mut self, name: &str, value: impl Into<OsString>) {
        let name = OsString::from(name);
        if !self.saved.iter().any(|(saved, _)| saved == &name) {
            self.saved.push((name.clone(), env::var_os(&name)));
        }
        env::set_var(name, value.into());
    }

    fn write_agent_policy(&self, network: u32) -> Result<PathBuf, Box<dyn std::error::Error>> {
        let policy = self.root.join("agent-native-policy.json");
        let native_policy = serde_json::to_string(&json!({
            "schema": "layerx.native-program-policy.v1",
            "version": "production-unhooked-v1",
            "network_id": network,
            "step_limit": 1,
            "rules": [{
                "id": "no-program-writes", "effect": "Deny",
                "activity": {"module": ModuleId::Programs as u16, "ordinal": 1},
                "scope": {"kind": "Operation"}, "required_approval": false,
                "purpose_commitment": null
            }]
        }))?;
        write_private(
            &policy,
            &serde_json::to_vec(&json!({
                "version": "layerx.native-program-policy-source.v1",
                "tenant": "tenant-a",
                "policy": native_policy
            }))?,
        )?;
        Ok(policy)
    }

    fn start_agent(&mut self) -> Result<AgentServer, Box<dyn std::error::Error>> {
        let node = LocalNode::start(&self.root)?;
        let network: u32 = env::var("LAYERX_HUMAN_NETWORK_ID")?.parse()?;
        let endpoint = SequencerEndpoint::new(&node.endpoint, &[])
            .map_err(|error| format!("node endpoint: {error:?}"))?;
        let chain_id = endpoint
            .chain_id()
            .map_err(|error| format!("node chain: {error:?}"))?;
        assert_eq!(chain_id, u64::from(network), "local node network mismatch");
        layerx_agentd::human_node::verify_direct_node_readiness_for_chain(
            &endpoint,
            chain_id,
            &node.consensus_chain,
        )
        .map_err(|error| format!("committed node readiness: {error:?}"))?;
        let authority = Authority::start(&self.root)?;
        let uid = rustix::process::getuid().as_raw();
        let gid = rustix::process::getgid().as_raw();
        let actor = env::var("LAYERX_HUMAN_AGENT_ACTOR")?;
        let peers = BTreeMap::from([(uid, (actor.clone(), "tenant-a".to_owned()))]);
        let deadline = Duration::from_secs(5);
        let remote = RemoteHumanAuthority::connect(
            &authority.endpoint,
            authority.bearer.clone(),
            deadline,
            1_048_576,
            &fs::read(self.root.join("authority-ca.der"))?,
        )
        .map_err(|error| format!("authority connection: {error:?}"))?;
        remote
            .registry(&HumanPeer {
                uid,
                principal: actor,
                tenant: "tenant-a".to_owned(),
                subject: None,
            })
            .map_err(|error| format!("authority readiness: {error:?}"))?;
        let store = Arc::new(Mutex::new(Store::open(self.root.join("agent-store"))?));
        let chain = NodeRpc::connect(&NodeRpcConfig {
            url: node.endpoint.clone(),
            chain_id,
            deadline,
        })
        .map_err(|error| format!("node connection: {error:?}"))?;
        let mut operations = ProductionHumanOperations::new(
            remote,
            endpoint,
            Box::new(chain),
            Arc::clone(&store),
            &peers,
            1_048_576,
            300,
            RuntimeClock::from_environment()?,
        )
        .map_err(|error| format!("agent operations: {error:?}"))?;
        operations
            .attach_native_policies(&BTreeMap::from([(
                TenantId::new("tenant-a")?,
                self.write_agent_policy(network)?,
            )]))
            .map_err(|error| format!("native policy: {error:?}"))?;
        let sessions = self.root.join("agent-sessions");
        DirBuilder::new().mode(0o700).create(&sessions)?;
        let mut secret = [0; 32];
        getrandom::fill(&mut secret).map_err(|error| error.to_string())?;
        let keys = SessionKeyRegistry::open(sessions, secret.to_vec(), network, uid)
            .map_err(|error| format!("session keys: {error:?}"))?;
        let owner = UnifiedAgentOwner::new(
            operations,
            store,
            &peers,
            vec![LimitConfig {
                id: LimitId([1; 16]),
                name: "production-components".to_owned(),
                scope: LimitScope::Tenant([2; 32]),
                ceiling: 1000,
                consumed: 0,
            }],
            keys,
        )
        .map_err(|error| format!("agent owner: {error:?}"))?;
        let socket_directory = self.root.join("agent-socket");
        DirBuilder::new().mode(0o700).create(&socket_directory)?;
        let socket = socket_directory.join("agent.sock");
        let server = HumanUnixServer::bind(
            HumanListenerConfig {
                endpoint: socket.clone(),
                owner_uid: uid,
                owner_gid: gid,
                mode: 0o600,
                maximum_frame_bytes: 1_048_576,
                deadline,
                maximum_connections: 4,
                maximum_streams: 4,
                maximum_queued_bytes: 1_048_576,
                peers,
            },
            SharedAgentOwner::new(owner),
        )
        .map_err(|error| format!("agent listener: {error:?}"))?;
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            server
                .serve_until(&stopped)
                .map_err(|error| format!("agent listener: {error:?}"))
        });
        let agent = AgentServer {
            stop,
            worker: Some(worker),
            _authority: authority,
            _node: node,
        };
        assert!(!agent.worker.as_ref().or_fail("agent worker").is_finished());
        drop(UnixStream::connect(socket)?);
        Ok(agent)
    }
}

struct LocalNode {
    script: PathBuf,
    directory: PathBuf,
    home: PathBuf,
    endpoint: String,
    consensus_chain: String,
}

impl LocalNode {
    fn command(&self) -> Command {
        let mut command = Command::new("bash");
        command.arg(&self.script).env("TMPDIR", &self.directory);
        command
    }

    fn start(directory: &Path) -> Result<Self, Box<dyn std::error::Error>> {
        let binary = env::var_os("PAXD").ok_or("PAXD must name the built node")?;
        assert!(Path::new(&binary).is_file(), "built node exists");
        let repository = Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .nth(4)
            .ok_or("repository root")?;
        let script = repository.join("tools/qualification/localnode/v6120-node.sh");
        let settings = directory.join("lx-localnode/localnode.env");
        let mut node = Self {
            script,
            directory: directory.to_owned(),
            home: PathBuf::new(),
            endpoint: String::new(),
            consensus_chain: String::new(),
        };
        let output = node
            .command()
            .env("PAXD", binary)
            .args(["--sequencer", "--env-out"])
            .arg(&settings)
            .stdin(Stdio::null())
            .output()?;
        assert!(
            output.status.success(),
            "local node startup: {}: {}",
            output.status,
            String::from_utf8_lossy(&output.stderr)
        );
        let document = zeroize::Zeroizing::new(fs::read_to_string(settings)?);
        let field = |name: &str| -> Result<String, Box<dyn std::error::Error>> {
            let value = document
                .lines()
                .filter_map(|line| line.split_once('='))
                .find_map(|(key, value)| (key == name).then_some(value))
                .ok_or_else(|| format!("node setting {name} required"))?;
            assert!(
                !value.is_empty()
                    && value
                        .bytes()
                        .all(|byte| byte.is_ascii_alphanumeric() || b"/._:-".contains(&byte)),
                "node setting encoding"
            );
            Ok(value.to_owned())
        };
        node.home = field("LOCAL_NODE_HOME")?.into();
        assert!(node.home.starts_with(directory.join("lx-localnode")));
        node.endpoint = field("LOCAL_NODE_SEQUENCER_RPC")?;
        let genesis: Value =
            serde_json::from_slice(&fs::read(node.home.join("config/genesis.json"))?)?;
        node.consensus_chain = genesis["chain_id"]
            .as_str()
            .ok_or("node consensus chain")?
            .to_owned();
        Ok(node)
    }
}

impl Drop for LocalNode {
    fn drop(&mut self) {
        if !self.home.as_os_str().is_empty() {
            let status = self
                .command()
                .arg("--stop")
                .arg(&self.home)
                .output()
                .or_fail("stop local node")
                .status;
            if !thread::panicking() {
                assert!(status.success(), "local node shutdown: {status}");
            }
        }
    }
}

struct AgentServer {
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<Result<(), String>>>,
    _authority: Authority,
    _node: LocalNode,
}

impl Drop for AgentServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let result = self.worker.take().or_fail("agent worker").join();
        if !thread::panicking() {
            result
                .or_fail("agent worker panicked")
                .or_fail("agent listener stopped");
        }
    }
}

struct Authority {
    endpoint: String,
    bearer: String,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl Authority {
    fn create_certificates(root: &Path) -> Result<(), Box<dyn std::error::Error>> {
        cluster::openssl(
            root,
            &[
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-subj",
                "/CN=authority-ca",
                "-days",
                "1",
                "-keyout",
                "authority-ca.key",
                "-out",
                "authority-ca.pem",
                "-addext",
                "basicConstraints=critical,CA:TRUE",
            ],
        )?;
        cluster::openssl(
            root,
            &[
                "req",
                "-new",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-subj",
                "/CN=localhost",
                "-keyout",
                "authority.key",
                "-out",
                "authority.csr",
            ],
        )?;
        write_private(&root.join("authority.ext"), b"basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:localhost\n")?;
        cluster::openssl(
            root,
            &[
                "x509",
                "-req",
                "-in",
                "authority.csr",
                "-CA",
                "authority-ca.pem",
                "-CAkey",
                "authority-ca.key",
                "-set_serial",
                "2",
                "-days",
                "1",
                "-sha256",
                "-extfile",
                "authority.ext",
                "-out",
                "authority.pem",
            ],
        )?;
        cluster::openssl(
            root,
            &[
                "x509",
                "-in",
                "authority-ca.pem",
                "-outform",
                "DER",
                "-out",
                "authority-ca.der",
            ],
        )?;
        Ok(())
    }

    fn start(root: &Path) -> Result<Self, Box<dyn std::error::Error>> {
        Self::create_certificates(root)?;
        let acceptor = TlsAcceptor::new(Identity::from_pkcs8(
            &fs::read(root.join("authority.pem"))?,
            &fs::read(root.join("authority.key"))?,
        )?)?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let endpoint = format!("https://localhost:{}", listener.local_addr()?.port());
        listener.set_nonblocking(true)?;
        let mut secret = [0; 32];
        getrandom::fill(&mut secret).map_err(|error| error.to_string())?;
        let bearer = URL_SAFE_NO_PAD.encode(secret);
        let expected_bearer = bearer.clone();
        let principal = env::var("LAYERX_HUMAN_AGENT_ACTOR")?.replace(':', "%3A");
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(10));
                        continue;
                    }
                    Err(error) => panic!("authority accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .or_fail("authority read timeout");
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .or_fail("authority write timeout");
                let mut stream = acceptor.accept(stream).or_fail("authority TLS");
                let mut request = Vec::new();
                while !request.ends_with(b"\r\n\r\n") {
                    assert!(request.len() < 16_384, "authority request bound");
                    let mut byte = [0];
                    stream
                        .read_exact(&mut byte)
                        .or_fail("authority HTTP request");
                    request.push(byte[0]);
                }
                let request = String::from_utf8(request).or_fail("authority HTTP UTF-8");
                assert!(request.lines().any(|line| line
                    .eq_ignore_ascii_case(&format!("authorization: Bearer {expected_bearer}"))));
                let route = request.lines().next().or_fail("request line");
                let expected = format!("/v1/agent/registry?tenant=tenant-a&principal={principal}");
                assert_eq!(
                    route,
                    format!("GET {expected} HTTP/1.1"),
                    "unexpected authority route"
                );
                let activity = ActivityType::new(ModuleId::Asset, 5).or_fail("asset send");
                let body = json!({"modules": [{"module_id": ModuleId::Asset as u16, "activity_types": [activity.value()]}]}).to_string();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).or_fail("authority response");
                stream.flush().or_fail("authority response flush");
            }
        });
        Ok(Self {
            endpoint,
            bearer,
            stop,
            worker: Some(worker),
        })
    }
}

impl Drop for Authority {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let result = self.worker.take().or_fail("authority worker").join();
        if !thread::panicking() {
            result.or_fail("authority worker panicked");
        }
    }
}

fn write_private(path: &Path, bytes: &[u8]) -> std::io::Result<()> {
    fs::write(path, bytes)?;
    fs::set_permissions(path, fs::Permissions::from_mode(0o600))
}

fn purpose_catalog() -> Value {
    let counterparty = [1_u8; 32];
    let asset = [2_u8; 32];
    json!({"version": "production-unhooked-v1", "presets": [{
        "id": "bounded-send", "call_selectors": ["0xa9059cbb"],
        "counterparties": [counterparty], "assets": [asset],
        "amount_ceiling": 1000, "rate_maximum_uses": 10,
        "rate_window_sequences": 100, "purposes": ["send"],
        "expiry_sequence": 1000, "session_scopes": ["send"],
        "session_lifetime_seconds": 300, "budget_asset": asset,
        "budget_period_seconds": 60, "budget_expiry_seconds": 600,
        "initial_funding": 100
    }]})
}

fn loader_primary_environment(
    root: &Path,
    tenancy_digest: [u8; 32],
    nodes: String,
) -> Vec<(&'static str, String)> {
    let path = |name: &str| root.join(name).display().to_string();
    let secret = |byte: u8| URL_SAFE_NO_PAD.encode([byte; 32]);
    vec![
        ("LAYERX_HUMAN_STORE_ROOT", path("store")),
        ("LAYERX_HUMAN_CUSTODY_ROOT", path("custody")),
        ("LAYERX_HUMAN_AUTH_INDEX_ROOT", path("auth-index")),
        (
            "LAYERX_HUMAN_TENANCY_DIGEST",
            URL_SAFE_NO_PAD.encode(tenancy_digest),
        ),
        ("LAYERX_HUMAN_AUTH_INDEX_KEY", secret(0x61)),
        ("LAYERX_HUMAN_STREAM_CURSOR_KEY", secret(0x62)),
        ("LAYERX_HUMAN_RP_ID", "paxportwallet.com".to_owned()),
        ("LAYERX_HUMAN_RP_NAME", "Paxport".to_owned()),
        (
            "LAYERX_HUMAN_ORIGIN",
            "https://paxportwallet.com".to_owned(),
        ),
        ("LAYERX_HUMAN_CEREMONY_TTL_SECONDS", "300".to_owned()),
        ("LAYERX_HUMAN_ASSERTION_TTL_SECONDS", "300".to_owned()),
        ("LAYERX_HUMAN_SESSION_TTL_SECONDS", "900".to_owned()),
        ("LAYERX_HUMAN_REFRESH_TTL_SECONDS", "86400".to_owned()),
        ("LAYERX_HUMAN_STEP_UP_TTL_SECONDS", "300".to_owned()),
        ("LAYERX_HUMAN_AUTH_RATE_ATTEMPTS", "10".to_owned()),
        ("LAYERX_HUMAN_AUTH_RATE_WINDOW_SECONDS", "60".to_owned()),
        (
            "LAYERX_HUMAN_RETENTION_JOURNEYS_SECONDS",
            "86400".to_owned(),
        ),
        (
            "LAYERX_HUMAN_RETENTION_NOTIFICATIONS_SECONDS",
            "86400".to_owned(),
        ),
        ("LAYERX_HUMAN_RETENTION_AUDIT_SECONDS", "86400".to_owned()),
        (
            "LAYERX_HUMAN_RETENTION_TELEMETRY_SECONDS",
            "86400".to_owned(),
        ),
        ("LAYERX_HUMAN_RETENTION_CACHE_SECONDS", "86400".to_owned()),
        ("LAYERX_HUMAN_CAPABILITY_TTL_SECONDS", "30".to_owned()),
        ("LAYERX_HUMAN_AGENT_SOCKET", path("agent-socket/agent.sock")),
        ("LAYERX_HUMAN_AGENT_MAX_FRAME_BYTES", "1048576".to_owned()),
        ("LAYERX_HUMAN_AGENT_MAX_CONNECTIONS", "4".to_owned()),
        ("LAYERX_HUMAN_AGENT_MAX_STREAMS", "4".to_owned()),
        ("LAYERX_HUMAN_AGENT_MAX_QUEUED_BYTES", "1048576".to_owned()),
        ("LAYERX_HUMAN_AGENT_DEADLINE_SECONDS", "5".to_owned()),
        ("LAYERX_HUMAN_SECURITY_SOCKET", path("security.sock")),
        ("LAYERX_HUMAN_SECURITY_DEADLINE_SECONDS", "5".to_owned()),
        (
            "LAYERX_HUMAN_SECURITY_MAX_FRAME_BYTES",
            "1048576".to_owned(),
        ),
        (
            "LAYERX_HUMAN_IDENTITY_BINDING_SOCKET",
            path("identity-binding.sock"),
        ),
        (
            "LAYERX_HUMAN_IDENTITY_BINDING_TENANT",
            "tenant-a".to_owned(),
        ),
        (
            "LAYERX_HUMAN_IDENTITY_BINDING_PEER_UID",
            rustix::process::getuid().as_raw().to_string(),
        ),
        (
            "LAYERX_HUMAN_IDENTITY_BINDING_PEER_GID",
            rustix::process::getgid().as_raw().to_string(),
        ),
        (
            "LAYERX_HUMAN_IDENTITY_BINDING_DEADLINE_SECONDS",
            "10".to_owned(),
        ),
        ("LAYERX_HUMAN_IDENTITY_SOCKET", path("identity.sock")),
        ("LAYERX_HUMAN_IDENTITY_DEADLINE_SECONDS", "5".to_owned()),
        (
            "LAYERX_HUMAN_IDENTITY_MAX_FRAME_BYTES",
            "1048576".to_owned(),
        ),
        (
            "LAYERX_HUMAN_IDENTITY_PEER_UID",
            rustix::process::getuid().as_raw().to_string(),
        ),
        (
            "LAYERX_HUMAN_IDENTITY_PEER_GID",
            rustix::process::getgid().as_raw().to_string(),
        ),
        ("LAYERX_HUMAN_ATTESTOR_NODES", nodes),
    ]
}

fn loader_secondary_environment(root: &Path) -> Vec<(&'static str, String)> {
    let path = |name: &str| root.join(name).display().to_string();
    let secret = |byte: u8| URL_SAFE_NO_PAD.encode([byte; 32]);
    vec![
        ("LAYERX_HUMAN_ATTESTOR_SIGNERS", cluster::SIGNERS.join(",")),
        (
            "LAYERX_HUMAN_ATTESTOR_ROOT_CERTIFICATE_DER",
            path("attestor-ca.der"),
        ),
        (
            "LAYERX_HUMAN_ATTESTOR_CLIENT_CERTIFICATE_DER",
            path("attestor-client.der"),
        ),
        (
            "LAYERX_HUMAN_ATTESTOR_CLIENT_PRIVATE_KEY_DER",
            path("attestor-client-key.der"),
        ),
        ("LAYERX_HUMAN_ATTESTOR_DEADLINE_SECONDS", "120".to_owned()),
        ("LAYERX_HUMAN_NETWORK_ID", "125".to_owned()),
        ("LAYERX_HUMAN_SIGNING_RATE_MAXIMUM", "1000".to_owned()),
        (
            "LAYERX_HUMAN_SIGNING_RATE_WINDOW_SECONDS",
            "10000".to_owned(),
        ),
        (
            "LAYERX_HUMAN_AGENT_ACTOR",
            format!("did:layerx:{}", cluster::hex(&[0x77; 32])),
        ),
        (
            "LAYERX_HUMAN_AGENT_AUTHORITY",
            "custody-human-primary".to_owned(),
        ),
        (
            "LAYERX_HUMAN_AGENT_TIMESTAMP_SPAN_SECONDS",
            "300".to_owned(),
        ),
        ("LAYERX_HUMAN_AGENT_FEE_LIMIT", "1000000".to_owned()),
        (
            "LAYERX_HUMAN_ONBOARDING_SPONSOR_PRINCIPAL",
            "onboarding-sponsor".to_owned(),
        ),
        ("LAYERX_HUMAN_ONBOARDING_INITIAL_FUNDING", "1000".to_owned()),
        ("LAYERX_HUMAN_EVM_GAS_LIMIT", "300000".to_owned()),
        ("LAYERX_HUMAN_EVM_MAX_FEE_PER_GAS", "1000000000".to_owned()),
        (
            "LAYERX_HUMAN_EVM_MAX_PRIORITY_FEE_PER_GAS",
            "1000000000".to_owned(),
        ),
        (
            "LAYERX_HUMAN_AGENT_PURPOSE_CATALOG",
            path("purpose-catalog.json"),
        ),
        (
            "LAYERX_HUMAN_AGENT_OWNER_ACCOUNT",
            "0x00000000000000000000000000000000000000a1".to_owned(),
        ),
        ("LAYERX_HUMAN_AGENT_RECOVERY_ROOT", secret(0x63)),
        ("LAYERX_HUMAN_AGENT_RECOVERY_THRESHOLD", "1".to_owned()),
        ("LAYERX_HUMAN_ACTIVITY_FRESHNESS_SECONDS", "300".to_owned()),
        (
            "LAYERX_HUMAN_ACTIVITY_EXPORT_MAXIMUM_BYTES",
            "1048576".to_owned(),
        ),
        (
            "LAYERX_HUMAN_CONTINUATION_UNKNOWN_DEADLINE_SECONDS",
            "300".to_owned(),
        ),
    ]
}

fn loader_environment(
    root: &Path,
    cluster: &cluster::Cluster,
    tenancy_digest: [u8; 32],
) -> Result<Vec<(&'static str, String)>, Box<dyn std::error::Error>> {
    write_private(
        &root.join("attestor-ca.der"),
        &cluster.pki.certificate_der("node-ca")?,
    )?;
    write_private(
        &root.join("attestor-client.der"),
        &cluster.pki.certificate_der("gateway")?,
    )?;
    write_private(
        &root.join("attestor-client-key.der"),
        &cluster.pki.key_der("gateway")?,
    )?;
    let nodes = cluster
        .nodes
        .iter()
        .map(|(id, address)| format!("{id}={address}"))
        .collect::<Vec<_>>()
        .join(",");
    let mut entries = loader_primary_environment(root, tenancy_digest, nodes);
    entries.extend(loader_secondary_environment(root));
    Ok(entries)
}

impl Drop for Environment {
    fn drop(&mut self) {
        drop(self.cluster.take());
        for (name, value) in self.saved.iter().rev() {
            match value {
                Some(value) => env::set_var(name, value),
                None => env::remove_var(name),
            }
        }
        let _ = fs::remove_dir_all(&self.root);
    }
}

struct Process(Child);

impl Process {
    fn new(child: Child) -> Self {
        Self(child)
    }
}

impl Drop for Process {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

#[test]
fn production_components_open_without_paxeer_or_exit_environment() -> TestResult {
    let _guard = ENVIRONMENT
        .lock()
        .map_err(|_| "environment mutex poisoned")?;
    let mut environment = Environment::new()?;
    let configuration = ProductionComponentsConfig::from_environment()?;
    validate_production_configuration(&configuration)?;
    for name in RETIRED_OPERATIONS {
        assert!(
            !PRODUCTION_OPERATIONS.contains(&name),
            "retired dispatch {name}"
        );
    }
    let _agent = environment.start_agent()?;
    let clock = RuntimeClock::from_environment()?;
    let components = ProductionComponents::open(configuration, clock)?;
    assert!(environment.root.join("store").is_dir());
    assert!(environment.root.join("custody").is_dir());
    drop(components);
    Ok(())
}

#[test]
fn production_configuration_validates_without_retired_operations() -> TestResult {
    let _guard = ENVIRONMENT
        .lock()
        .map_err(|_| "environment mutex poisoned")?;
    let _environment = Environment::new()?;
    let schema = ApiSchema::v1()?;
    for name in RETIRED_OPERATIONS {
        assert!(
            !PRODUCTION_OPERATIONS.contains(&name),
            "retired dispatch {name}"
        );
        assert!(schema
            .operations()
            .iter()
            .any(|operation| operation.name == name && operation.retired()));
    }
    let active = schema
        .operations()
        .iter()
        .filter(|operation| !operation.retired())
        .map(|operation| operation.name.as_str())
        .collect::<BTreeSet<_>>();
    assert_eq!(active, PRODUCTION_OPERATIONS.iter().copied().collect());
    let configuration = ProductionComponentsConfig::from_environment()?;
    validate_production_configuration(&configuration)?;
    Ok(())
}

#[test]
fn components_binary_starts_without_recipient_environment() -> TestResult {
    let _guard = ENVIRONMENT
        .lock()
        .map_err(|_| "environment mutex poisoned")?;
    let mut environment = Environment::new()?;
    let socket = environment.root.join("components.sock");
    environment.set(
        "LAYERX_HUMAN_COMPONENT_ALLOWED_UID",
        rustix::process::getuid().as_raw().to_string(),
    );
    environment.set(
        "LAYERX_HUMAN_COMPONENT_SOCKET",
        socket.clone().into_os_string(),
    );
    environment.set("LAYERX_HUMAN_COMPONENT_WORKERS", "1");
    environment.set("LAYERX_HUMAN_COMPONENT_QUEUE_CAPACITY", "4");
    environment.set("LAYERX_HUMAN_MAINTENANCE_INTERVAL_SECONDS", "60");
    environment.set("LAYERX_HUMAN_MAINTENANCE_MAXIMUM_ITEMS", "16");
    let _agent = environment.start_agent()?;
    let stderr = environment.root.join("components.stderr");
    let mut process = Process::new(
        Command::new(env!("CARGO_BIN_EXE_layerx-human-components"))
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(fs::File::create(&stderr)?)
            .spawn()?,
    );
    let clock = RuntimeClock::from_environment()?;
    let mut deadline = Deadline::start(clock.as_ref(), Duration::from_secs(15))?;
    loop {
        if let Some(status) = process.0.try_wait()? {
            return Err(format!(
                "components exited {status}: {}",
                fs::read_to_string(&stderr)?
            )
            .into());
        }
        if let Ok(connection) = UnixStream::connect(&socket) {
            assert!(process.0.try_wait()?.is_none());
            drop(connection);
            break;
        }
        if deadline.remaining(clock.as_ref())?.is_zero() {
            return Err(
                format!("components did not bind: {}", fs::read_to_string(&stderr)?).into(),
            );
        }
        std::thread::sleep(Duration::from_millis(20));
    }
    Ok(())
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
