#[allow(dead_code)]
#[path = "../../layerx-human-kms/tests/support/attestor_cluster.rs"]
mod cluster;

use std::collections::BTreeSet;
use std::env;
use std::ffi::OsString;
use std::fs::{self, DirBuilder};
use std::io::{Read as _, Write as _};
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::{DirBuilderExt as _, PermissionsExt as _};
use std::os::unix::net::{UnixListener, UnixStream};
use std::os::unix::process::CommandExt as _;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};
use layerx_client::runtime_clock::RuntimeClock;
use layerx_human_service::server::production_components::{
    validate_production_configuration, PRODUCTION_OPERATIONS,
};
use layerx_human_service::server::schema::ApiSchema;
use layerx_human_service::server::{ProductionComponents, ProductionComponentsConfig};
use layerx_human_service::store::TenancyMap;
use layerx_types::clock::Clock as _;
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

fn retired_variable(name: &str) -> bool {
    [
        "LAYERX_HUMAN_PAXEER_",
        "LAYERX_HUMAN_EXIT_",
        "LAYERX_HUMAN_RECIPIENT_",
    ]
    .iter()
    .any(|prefix| name.starts_with(prefix))
        || matches!(
            name,
            "LAYERX_HUMAN_BINDING_STATEMENT_TTL_SECONDS" | "LAYERX_HUMAN_PROTOCOL_VERSION"
        )
}

struct Environment {
    root: PathBuf,
    saved: Vec<(OsString, Option<OsString>)>,
    cluster: Option<cluster::Cluster>,
    _movement_listener: UnixListener,
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
        let movement_directory = root.join("movement-socket");
        DirBuilder::new().mode(0o700).create(&movement_directory)?;
        let movement_socket = movement_directory.join("movement.sock");
        let movement_listener = UnixListener::bind(&movement_socket)?;
        fs::set_permissions(&movement_socket, fs::Permissions::from_mode(0o600))?;
        let events_listener = TcpListener::bind("127.0.0.1:0")?;
        let events_url = format!("https://127.0.0.1:{}", events_listener.local_addr()?.port());
        let mut environment = Self {
            root,
            saved: Vec::new(),
            cluster: None,
            _movement_listener: movement_listener,
            _events_listener: events_listener,
        };
        for (url, ca) in [
            (
                "LAYERX_EVENTS_JOURNEY_UPSTREAM_URL",
                "LAYERX_EVENTS_JOURNEY_UPSTREAM_CA_DER",
            ),
            (
                "LAYERX_EVENTS_APPROVAL_UPSTREAM_URL",
                "LAYERX_EVENTS_APPROVAL_UPSTREAM_CA_DER",
            ),
            (
                "LAYERX_EVENTS_WEBHOOKS_UPSTREAM_URL",
                "LAYERX_EVENTS_WEBHOOKS_UPSTREAM_CA_DER",
            ),
        ] {
            environment.set(url, events_url.clone());
            environment.set(
                ca,
                environment.root.join("authority-ca.der").into_os_string(),
            );
        }
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
            assert!(!retired_variable(name));
            environment.set(name, value);
        }
        environment.cluster = Some(cluster);
        write_private(
            &environment.root.join("purpose-catalog.json"),
            &serde_json::to_vec(&purpose_catalog())?,
        )?;
        assert!(env::vars_os().all(|(name, _)| !retired_variable(&name.to_string_lossy())));
        Ok(environment)
    }

    fn set(&mut self, name: &str, value: impl Into<OsString>) {
        let name = OsString::from(name);
        if !self.saved.iter().any(|(saved, _)| saved == &name) {
            self.saved.push((name.clone(), env::var_os(&name)));
        }
        env::set_var(name, value.into());
    }

    fn start_agent(&self) -> Result<Process, Box<dyn std::error::Error>> {
        let binary = env::var_os("LAYERX_TEST_AGENTD_BIN")
            .ok_or("LAYERX_TEST_AGENTD_BIN must name the built, real layerx-agentd")?;
        let native = env::var_os("LAYERX_TEST_NATIVE_BIN_DIR").ok_or(
            "do_16 precondition: LAYERX_TEST_NATIVE_BIN_DIR must provide the real node binaries",
        )?;
        if !rustix::process::getuid().is_root() {
            return Err(
                "do_16 precondition: the real node fixture requires root for a distinct daemon uid"
                    .into(),
            );
        }
        let rpc_port: u16 = env::var("LAYERX_TEST_PAXEER_RPC_PORT")
            .map_err(|error| format!("LAYERX_TEST_PAXEER_RPC_PORT: {error}"))?
            .parse()?;
        let chain_id: u64 = env::var("LAYERX_TEST_PAXEER_CHAIN_ID")
            .map_err(|error| format!("LAYERX_TEST_PAXEER_CHAIN_ID: {error}"))?
            .parse()?;
        let network = env::var("LAYERX_HUMAN_NETWORK_ID")?;
        let node = start_node(&self.root, Path::new(&native), network.parse()?)?;
        let authority = Authority::start(&self.root)?;
        let uid = rustix::process::getuid().as_raw().to_string();
        let gid = rustix::process::getgid().as_raw().to_string();
        let actor = env::var("LAYERX_HUMAN_AGENT_ACTOR")?;
        let sessions = self.root.join("agent-sessions");
        DirBuilder::new().mode(0o700).create(&sessions)?;
        let mut secret = [0; 32];
        getrandom::fill(&mut secret).map_err(|error| error.to_string())?;
        let operator_secret = self.root.join("agent-operator-secret");
        write_private(&operator_secret, &secret)?;
        let health_bearer = URL_SAFE_NO_PAD.encode(secret);
        let policy = self.root.join("agent-native-policy.json");
        let native_policy = serde_json::to_string(&json!({
            "schema": "layerx.native-program-policy.v1",
            "version": "production-unhooked-v1",
            "network_id": network.parse::<u32>()?,
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
        let stderr = self.root.join("agent.stderr");
        let socket_directory = self.root.join("agent-socket");
        DirBuilder::new().mode(0o700).create(&socket_directory)?;
        let socket = socket_directory.join("agent.sock");
        let mut process = Process::new(
            Command::new(binary)
                .env("LAYERX_AGENT_MODE", "human-owner")
                .env(
                    "LAYERX_AGENT_NODE_RPC_URL",
                    format!("http://127.0.0.1:{rpc_port}"),
                )
                .env("LAYERX_AGENT_NODE_CHAIN_ID", chain_id.to_string())
                .env(
                    "LAYERX_AGENT_HUMAN_NODE_LNI",
                    self.root.join("node-run/layerxd.sock"),
                )
                .env("LAYERX_AGENT_HUMAN_AUTHORITY_ENDPOINT", &authority.endpoint)
                .env("LAYERX_AGENT_HUMAN_AUTHORITY_BEARER", &authority.bearer)
                .env(
                    "LAYERX_AGENT_HUMAN_AUTHORITY_CA_DER",
                    self.root.join("authority-ca.der"),
                )
                .env("LAYERX_AGENT_PROGRAM_LISTEN", "127.0.0.1:0")
                .env("LAYERX_AGENT_PROGRAM_BEARER_TOKEN", health_bearer)
                .env(
                    "LAYERX_AGENT_HUMAN_PEERS",
                    format!("uid={uid};tenant=tenant-a;principal={actor}"),
                )
                .env(
                    "LAYERX_NATIVE_POLICY_SOURCES",
                    format!("tenant-a:{}", policy.display()),
                )
                .env("LAYERX_AGENT_HUMAN_STORE", self.root.join("agent-store"))
                .env("LAYERX_AGENT_HUMAN_SOCKET", &socket)
                .env("LAYERX_AGENT_HUMAN_SOCKET_UID", uid)
                .env("LAYERX_AGENT_HUMAN_SOCKET_GID", gid)
                .env("LAYERX_AGENT_HUMAN_SOCKET_MODE", "600")
                .env("LAYERX_AGENT_HUMAN_SESSION_KEY_ROOT", sessions)
                .env(
                    "LAYERX_AGENT_HUMAN_SESSION_OPERATOR_SECRET_FILE",
                    operator_secret,
                )
                .env("LAYERX_AGENT_HUMAN_NETWORK_ID", network)
                .env(
                    "LAYERX_AGENT_HUMAN_PROTOCOL_VERSION",
                    layerx_wire::limits::PROTOCOL_VERSION.to_string(),
                )
                .env("LAYERX_AGENT_HUMAN_MAX_FRAME_BYTES", "1048576")
                .env("LAYERX_AGENT_HUMAN_MAX_CONNECTIONS", "4")
                .env("LAYERX_AGENT_HUMAN_MAX_STREAMS", "4")
                .env("LAYERX_AGENT_HUMAN_MAX_QUEUED_BYTES", "1048576")
                .env("LAYERX_AGENT_HUMAN_DEADLINE_MS", "5000")
                .env("LAYERX_AGENT_HUMAN_AUTHORITY_MAX_BYTES", "1048576")
                .env("LAYERX_AGENT_HUMAN_MAX_PAYLOAD_BYTES", "1048576")
                .env("LAYERX_AGENT_HUMAN_TIMESTAMP_SPAN", "300")
                .env("LAYERX_AGENT_HUMAN_RECONNECT_ATTEMPTS", "1")
                .env("LAYERX_AGENT_HUMAN_RECONNECT_BASE_MS", "10")
                .env("LAYERX_AGENT_HUMAN_RECONNECT_MAX_MS", "10")
                .env("LAYERX_AGENT_HUMAN_RECONNECT_JITTER_PERCENT", "0")
                .env("LAYERX_AGENT_HUMAN_LIMIT_ID", cluster::hex(&[1; 16]))
                .env("LAYERX_AGENT_HUMAN_LIMIT_NAME", "production-unhooked")
                .env("LAYERX_AGENT_HUMAN_LIMIT_SCOPE", "tenant")
                .env("LAYERX_AGENT_HUMAN_LIMIT_SCOPE_ID", cluster::hex(&[2; 32]))
                .env("LAYERX_AGENT_HUMAN_LIMIT_CEILING", "1000")
                .env("LAYERX_AGENT_HUMAN_LIMIT_CONSUMED", "0")
                .stdin(Stdio::null())
                .stdout(Stdio::null())
                .stderr(fs::File::create(&stderr)?)
                .spawn()?,
        );
        process.1.push(node);
        process.2 = Some(authority);
        let deadline = Instant::now() + Duration::from_secs(15);
        loop {
            if let Some(status) = process.0.try_wait()? {
                return Err(format!(
                    "real agent daemon exited {status}: {}",
                    fs::read_to_string(&stderr)?
                )
                .into());
            }
            if let Ok(connection) = UnixStream::connect(&socket) {
                assert!(process.0.try_wait()?.is_none());
                drop(connection);
                return Ok(process);
            }
            if Instant::now() >= deadline {
                return Err(format!(
                    "real agent daemon did not bind: {}",
                    fs::read_to_string(&stderr)?
                )
                .into());
            }
            std::thread::sleep(Duration::from_millis(20));
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
    fn start(root: &Path) -> Result<Self, Box<dyn std::error::Error>> {
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
                    .expect("authority read timeout");
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .expect("authority write timeout");
                let mut stream = acceptor.accept(stream).expect("authority TLS");
                let mut request = Vec::new();
                while !request.ends_with(b"\r\n\r\n") {
                    assert!(request.len() < 16_384, "authority request bound");
                    let mut byte = [0];
                    stream
                        .read_exact(&mut byte)
                        .expect("authority HTTP request");
                    request.push(byte[0]);
                }
                let request = String::from_utf8(request).expect("authority HTTP UTF-8");
                assert!(request.lines().any(|line| line
                    .eq_ignore_ascii_case(&format!("authorization: Bearer {expected_bearer}"))));
                let route = request.lines().next().expect("request line");
                let expected = format!("/v1/agent/registry?tenant=tenant-a&principal={principal}");
                assert_eq!(
                    route,
                    format!("GET {expected} HTTP/1.1"),
                    "unexpected authority route"
                );
                let activity = ActivityType::new(ModuleId::Asset, 5).expect("asset send");
                let body = json!({"modules": [{"module_id": ModuleId::Asset as u16, "activity_types": [activity.value()]}]}).to_string();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).expect("authority response");
                stream.flush().expect("authority response flush");
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
        let result = self.worker.take().expect("authority worker").join();
        if !thread::panicking() {
            result.expect("authority worker panicked");
        }
    }
}

#[allow(dead_code)]
#[path = "../../../../../tests/support/lxgb_metadata.rs"]
mod lxgb_metadata;

fn start_node(
    root: &Path,
    native: &Path,
    network: u32,
) -> Result<Process, Box<dyn std::error::Error>> {
    let chain_id: u64 = env::var("LAYERX_TEST_PAXEER_CHAIN_ID")?.parse()?;
    let rpc_port: u16 = env::var("LAYERX_TEST_PAXEER_RPC_PORT")?.parse()?;
    assert!(chain_id != 0 && rpc_port != 0);
    let genesis = root.join("genesis");
    let node = root.join("node");
    let run = root.join("node-run");
    let replica = root.join("replica");
    for directory in [&genesis, &node, &replica] {
        DirBuilder::new().mode(0o700).create(directory)?;
    }
    DirBuilder::new().mode(0o750).create(&run)?;
    DirBuilder::new()
        .mode(0o700)
        .create(node.join("checkpoints"))?;
    DirBuilder::new().mode(0o700).create(node.join("logs"))?;
    fs::set_permissions(root, fs::Permissions::from_mode(0o711))?;
    cluster::openssl(
        &genesis,
        &[
            "genpkey",
            "-algorithm",
            "EC",
            "-pkeyopt",
            "ec_paramgen_curve:secp256k1",
            "-out",
            "guarantor.key",
        ],
    )?;
    cluster::openssl(
        &genesis,
        &[
            "ec",
            "-in",
            "guarantor.key",
            "-pubout",
            "-conv_form",
            "compressed",
            "-outform",
            "DER",
            "-out",
            "guarantor.der",
        ],
    )?;
    let public = fs::read(genesis.join("guarantor.der"))?;
    let guarantor = &public[public.len() - 33..];
    let clock = RuntimeClock::from_environment()?;
    let now = clock.sample(Duration::from_secs(1))?.unix_milliseconds;
    let protocol = layerx_wire::limits::PROTOCOL_VERSION;
    let asset = random32()?;
    let seed = random32()?;
    let key = ed25519_dalek::SigningKey::from_bytes(&seed)
        .verifying_key()
        .to_bytes();
    let mut request = b"LXGB".to_vec();
    request.push(2);
    request.extend_from_slice(&protocol.to_be_bytes());
    request.extend_from_slice(&network.to_be_bytes());
    request.extend_from_slice(&now.to_be_bytes());
    request.extend_from_slice(&1_u16.to_be_bytes());
    request.extend_from_slice(&7_u16.to_be_bytes());
    let mut parameter = [0; 32];
    parameter[..17].copy_from_slice(b"parameter-version");
    request.extend_from_slice(&parameter);
    let mut version = [0; 32];
    version[31] = 1;
    request.extend_from_slice(&version);
    request.extend_from_slice(&1_u16.to_be_bytes());
    let mut guarantor_id = [0; 32];
    guarantor_id[0] = 1;
    request.extend_from_slice(&guarantor_id);
    request.extend_from_slice(guarantor);
    request.extend_from_slice(&[0; 16]);
    request.extend_from_slice(&asset);
    request.extend_from_slice(&1_u32.to_be_bytes());
    for value in [1_u64, 1, 1, 1, 1, 8, 8, 64, 8] {
        request.extend_from_slice(&value.to_be_bytes());
    }
    request.extend_from_slice(&1_u64.to_be_bytes());
    request.push(1);
    request.extend_from_slice(&1_u32.to_be_bytes());
    for value in [1_u64, 1, 2, 4, 1, 1, 100, 100, 1, 1, 10, 1, 1000] {
        request.extend_from_slice(&value.to_be_bytes());
    }
    lxgb_metadata::append(&mut request, &asset, &key, &random32()?);
    write_private(&genesis.join("request.lxgb"), &request)?;
    write_private(&genesis.join("signer.key"), &random32()?)?;
    let artifacts = genesis.join("artifacts");
    let output = Command::new(native.join("layerx-genesis-build"))
        .arg(genesis.join("request.lxgb"))
        .arg(genesis.join("signer.key"))
        .arg(&artifacts)
        .output()?;
    if !output.status.success() {
        return Err(format!(
            "real genesis builder failed {}: {}",
            output.status,
            String::from_utf8_lossy(&output.stderr)
        )
        .into());
    }
    let registration_request = fs::read(artifacts.join("paxeer-registration-request.lxrr"))?;
    assert_eq!(registration_request.len(), 73);
    assert_eq!(&registration_request[..4], b"LXRR");
    let mut registration = b"LXGR".to_vec();
    registration.push(1);
    registration.extend_from_slice(&network.to_be_bytes());
    registration.extend_from_slice(&0_u64.to_be_bytes());
    registration.extend_from_slice(&registration_request[41..73]);
    registration.extend_from_slice(&registration_request[41..73]);
    registration.push(1);
    write_private(&node.join("registration.lxgr"), &registration)?;
    let actor = env::var("LAYERX_HUMAN_AGENT_ACTOR")?;
    write_private(
        &node.join("identities.txt"),
        format!(
            "{}:{}:1\n",
            cluster::hex(actor.as_bytes()),
            cluster::hex(&key)
        )
        .as_bytes(),
    )?;
    for (directory, role) in [(&node, "sequencer"), (&replica, "replica")] {
        write_private(&directory.join("config.txt"), format!("config_version=2\nrole={role}\nnetwork_id={network}\nstart_sequence=0\nverify_workers=0\nserial_execution=true\n").as_bytes())?;
    }
    for directory in [&genesis, &node, &run, &replica] {
        chown_node(directory)?;
    }
    let ports = [
        TcpListener::bind("127.0.0.1:0")?,
        TcpListener::bind("127.0.0.1:0")?,
    ];
    let replica_port = ports[0].local_addr()?.port();
    let program_port = ports[1].local_addr()?.port();
    let sequencer_id = cluster::hex(&random32()?);
    let replica_id = cluster::hex(&random32()?);
    let replica_token = cluster::hex(&random32()?);
    let layerxd = native.join("layerxd");
    let mut replica_command = node_command(
        &layerxd,
        "--authority-replica",
        &replica.join("config.txt"),
        &root.join("replica.stderr"),
    )?;
    replica_command
        .env("LAYERX_AUTHORITY_REPLICA_LOG", replica.join("replica.log"))
        .env("LAYERX_AUTHORITY_REPLICA_ID", &replica_id)
        .env("LAYERX_AUTHORITY_SEQUENCER_ID", &sequencer_id)
        .env("LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY", cluster::hex(&key))
        .env("LAYERX_AUTHORITY_FIRST_BATCH", "1")
        .env("LAYERX_AUTHORITY_LAST_BATCH", u64::MAX.to_string())
        .env("LAYERX_AUTHORITY_BEARER_TOKEN", &replica_token)
        .env("LAYERX_AUTHORITY_ADDRESS", "127.0.0.1")
        .env("LAYERX_AUTHORITY_PORT", replica_port.to_string());
    drop(ports);
    let mut replica_process = Process::new(replica_command.spawn()?);
    wait_for_process(
        &mut replica_process,
        &root.join("replica.stderr"),
        "",
        || TcpStream::connect(("127.0.0.1", replica_port)).is_ok(),
    )?;
    let mut command = node_command(
        &layerxd,
        "--serve",
        &node.join("config.txt"),
        &root.join("node.stderr"),
    )?;
    command
        .env("LAYERX_NODE_PAXEER_CHAIN_ID", chain_id.to_string())
        .env("LAYERX_NODE_PAXEER_RPC_ADDRESS", "127.0.0.1")
        .env("LAYERX_NODE_PAXEER_RPC_PORT", rpc_port.to_string())
        .env(
            "LAYERX_NODE_SETTLEMENT_CONTRACT",
            "0x0000000000000000000000000000000000001014",
        )
        .env(
            "LAYERX_NODE_CHECKPOINT_REGISTRY",
            "0x0000000000000000000000000000000000001014",
        )
        .env("LAYERX_NODE_CHECKPOINT_DIRECTORY", node.join("checkpoints"))
        .env(
            "LAYERX_NODE_SNAPSHOT",
            artifacts.join("00000000000000000000.lxs"),
        )
        .env(
            "LAYERX_NODE_GENESIS_MANIFEST",
            artifacts.join("genesis.manifest"),
        )
        .env(
            "LAYERX_NODE_GENESIS_REGISTRATION",
            node.join("registration.lxgr"),
        )
        .env("LAYERX_NODE_IDENTITIES", node.join("identities.txt"))
        .env("LAYERX_NODE_HISTORY_DATABASE", node.join("history.db"))
        .env(
            "LAYERX_NODE_HISTORY_MIGRATIONS",
            Path::new(env!("CARGO_MANIFEST_DIR"))
                .join("../../../../migrations/0007_history_index.sql"),
        )
        .env("LAYERX_NODE_SEQUENCER_ID", sequencer_id)
        .env("LAYERX_NODE_SEQUENCER_PUBLIC_KEY", cluster::hex(&key))
        .env("LAYERX_NODE_SEQUENCER_PRIVATE_KEY", cluster::hex(&seed))
        .env("LAYERX_NODE_FIRST_BATCH", "1")
        .env("LAYERX_NODE_LAST_BATCH", u64::MAX.to_string())
        .env("LAYERX_NODE_AUTHORITY_REPLICA_ADDRESS", "127.0.0.1")
        .env(
            "LAYERX_NODE_AUTHORITY_REPLICA_PORT",
            replica_port.to_string(),
        )
        .env("LAYERX_NODE_AUTHORITY_REPLICA_ID", replica_id)
        .env("LAYERX_NODE_AUTHORITY_REPLICA_BEARER_TOKEN", replica_token)
        .env(
            "LAYERX_NODE_PROGRAM_BEARER_TOKEN",
            cluster::hex(&random32()?),
        )
        .env("LAYERX_NODE_PROGRAM_ADDRESS", "127.0.0.1")
        .env("LAYERX_NODE_PROGRAM_PORT", program_port.to_string())
        .env("LAYERX_NODE_LNI_SOCKET", run.join("layerxd.sock"))
        .env("LAYERX_NODE_LNI_ALLOWED_UID", "0")
        .env("LAYERX_NODE_LNI_ALLOWED_GID", "0")
        .env("LAYERX_NODE_LNI_FRAME_BYTES", "1212416")
        .env("LAYERX_NODE_LNI_DEADLINE_MS", "10000");
    for (variable, file) in [
        ("LAYERX_NODE_PROGRAM_FEED_LOG", "feed.log"),
        ("LAYERX_NODE_CANONICAL_LOG", "canonical.log"),
        ("LAYERX_NODE_RECEIPT_AUTHORITY_LOG", "receipt-authority.log"),
        ("LAYERX_NODE_BATCH_LOG", "batch.log"),
        ("LAYERX_NODE_EVIDENCE_LOG", "evidence.log"),
    ] {
        command.env(variable, node.join("logs").join(file));
    }
    let finality_diagnostics = finality_rpc_diagnostics(rpc_port);
    let mut process = Process::new(command.spawn()?);
    process.1.push(replica_process);
    wait_for_process(
        &mut process,
        &root.join("node.stderr"),
        &finality_diagnostics,
        || run.join("layerxd.sock").exists(),
    )?;
    Ok(process)
}

fn finality_rpc_diagnostics(port: u16) -> String {
    use sha3::{Digest as _, Keccak256};

    let mut diagnostics = String::new();
    for signature in ["eth_chainId", "threshold()", "latestFinalized()"] {
        let (method, params) = if signature == "eth_chainId" {
            ("eth_chainId", json!([]))
        } else {
            let selector = Keccak256::digest(signature.as_bytes());
            (
                "eth_call",
                json!([{
                    "to": "0x0000000000000000000000000000000000001014",
                    "data": format!("0x{}", cluster::hex(&selector[..4])),
                }, "latest"]),
            )
        };
        let body =
            json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).to_string();
        let mut response = Vec::new();
        let result = (|| -> std::io::Result<()> {
            let mut stream = TcpStream::connect_timeout(
                &std::net::SocketAddr::from(([127, 0, 0, 1], port)),
                Duration::from_secs(5),
            )?;
            stream.set_read_timeout(Some(Duration::from_secs(5)))?;
            stream.set_write_timeout(Some(Duration::from_secs(5)))?;
            write!(stream, "POST / HTTP/1.1\r\nHost: 127.0.0.1:{port}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len())?;
            stream.read_to_end(&mut response)?;
            Ok(())
        })();
        diagnostics.push_str(&format!(
            "\n{signature} raw HTTP response:\n{}",
            String::from_utf8_lossy(&response)
        ));
        if let Err(error) = result {
            diagnostics.push_str(&format!("\nRPC diagnostic transport error: {error}"));
        }
    }
    diagnostics
}

fn random32() -> Result<[u8; 32], Box<dyn std::error::Error>> {
    let mut bytes = [0; 32];
    getrandom::fill(&mut bytes).map_err(|error| error.to_string())?;
    Ok(bytes)
}

fn chown_node(path: &Path) -> std::io::Result<()> {
    std::os::unix::fs::chown(path, Some(65_534), Some(0))?;
    if path.is_dir() {
        for entry in fs::read_dir(path)? {
            chown_node(&entry?.path())?;
        }
    }
    Ok(())
}

fn node_command(
    binary: &Path,
    mode: &str,
    config: &Path,
    stderr: &Path,
) -> Result<Command, Box<dyn std::error::Error>> {
    let mut command = Command::new(binary);
    command
        .arg(mode)
        .arg(config)
        .env_clear()
        .env("PATH", "/usr/bin:/bin")
        .uid(65_534)
        .gid(0)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(fs::File::create(stderr)?);
    Ok(command)
}

fn wait_for_process(
    process: &mut Process,
    stderr: &Path,
    diagnostics: &str,
    ready: impl Fn() -> bool,
) -> TestResult {
    let deadline = Instant::now() + Duration::from_secs(30);
    loop {
        if let Some(status) = process.0.try_wait()? {
            return Err(format!(
                "real node exited {status}: {}{diagnostics}",
                fs::read_to_string(stderr)?
            )
            .into());
        }
        if ready() {
            return Ok(());
        }
        if Instant::now() >= deadline {
            return Err(format!(
                "real node startup timed out: {}",
                fs::read_to_string(stderr)?
            )
            .into());
        }
        thread::sleep(Duration::from_millis(20));
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
    let path = |name: &str| root.join(name).display().to_string();
    let secret = |byte: u8| URL_SAFE_NO_PAD.encode([byte; 32]);
    let nodes = cluster
        .nodes
        .iter()
        .map(|(id, address)| format!("{id}={address}"))
        .collect::<Vec<_>>()
        .join(",");
    Ok(vec![
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
        (
            "LAYERX_HUMAN_MOVEMENT_SOCKET",
            path("movement-socket/movement.sock"),
        ),
        (
            "LAYERX_HUMAN_MOVEMENT_PEER_UID",
            rustix::process::getuid().as_raw().to_string(),
        ),
        (
            "LAYERX_HUMAN_MOVEMENT_PEER_GID",
            rustix::process::getgid().as_raw().to_string(),
        ),
        (
            "LAYERX_HUMAN_MOVEMENT_MAX_FRAME_BYTES",
            "1048576".to_owned(),
        ),
        ("LAYERX_HUMAN_MOVEMENT_DEADLINE_SECONDS", "5".to_owned()),
        ("LAYERX_HUMAN_ATTESTOR_NODES", nodes),
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
    ])
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

struct Process(Child, Vec<Process>, Option<Authority>);

impl Process {
    fn new(child: Child) -> Self {
        Self(child, Vec::new(), None)
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
    let environment = Environment::new()?;
    let configuration = ProductionComponentsConfig::from_environment()?;
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
    let deadline = Instant::now() + Duration::from_secs(15);
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
        if Instant::now() >= deadline {
            return Err(
                format!("components did not bind: {}", fs::read_to_string(&stderr)?).into(),
            );
        }
        std::thread::sleep(Duration::from_millis(20));
    }
    Ok(())
}
