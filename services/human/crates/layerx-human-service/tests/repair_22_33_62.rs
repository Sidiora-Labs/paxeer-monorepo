//! Repair record 22.33.62 / audit 22.30.107 / acceptance 22998.1.
//! kms-process-clock-not-forwarded: `explicit_supervisor_binding_survives_child_environment_clear`;
//! `startup_refusals_and_chain_snapshot_round_trip_use_the_real_provider`.
//! kms-process-network-disagreement: `startup_refusals_and_chain_snapshot_round_trip_use_the_real_provider`.
//! Baseline: the repair target and prior implementation are absent on this checkout;
//! the earlier attempt stopped in `observed_balances_denom` before verification (gotcha 2974).
//! No unchanged-tree verify ran: the task's ordered implementation/build/hf-done rules prohibit it.
//! Scope excludes spec/GOTCHA edits; this record holds the finding/test mapping.
//! The runner records the verified candidate revision and private gate evidence.
//! Qualification repair: production startup must supply `RemoteKmsProvider`'s
//! authenticated absolute-deadline transport; a refused TCP connection is its
//! exact kms-timeout error, rather than a configuration-stage refusal.
//! Qualification repair: Rustix 1.1.4 converts `SocketAddrAny` via `TryFrom`, not `as_socket`.
//! No out-of-path wiring is required. Peers use committed TLS material, the real
//! supervisor clock server and the real KMS provider/store, entirely in process.

#[allow(dead_code)]
#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod clock_server;
#[path = "../src/bin/kms_gateway/startup.rs"]
mod startup;

use layerx_client::runtime_clock::ClockBinding;
use layerx_human_service::custody::{
    KeyClass, KmsError, KmsProvider, LxkpGateway, PrincipalKeyBinding, RemoteKmsProvider,
    GATEWAY_FRAME_LIMIT,
};
use layerx_human_service::frame::{Limits, MutualTlsConfig};
use layerx_types::clock::Clock;
use rustls::pki_types::{CertificateDer, PrivateKeyDer};
use rustls::RootCertStore;
use std::collections::BTreeMap;
use std::error::Error;
use std::ffi::{OsStr, OsString};
use std::fs;
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{mpsc, Arc};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

type TestResult<T = ()> = Result<T, Box<dyn Error>>;
const PROVIDER: &str = "repair-22-33-62";
const PREFIX: &str = "LAYERX_HUMAN_KMS_GATEWAY";
const CA: &[u8] = include_bytes!("../../layerx-human-kms/tests/fixtures/repair_22_33_27/ca.der");
const SERVER: &[u8] =
    include_bytes!("../../layerx-human-kms/tests/fixtures/repair_22_33_27/server.der");
const SERVER_KEY: &[u8] =
    include_bytes!("../../layerx-human-kms/tests/fixtures/repair_22_33_27/server.pk8");
const CLIENT: &[u8] =
    include_bytes!("../../layerx-human-kms/tests/fixtures/repair_22_33_27/client.der");
const CLIENT_KEY: &[u8] =
    include_bytes!("../../layerx-human-kms/tests/fixtures/repair_22_33_27/client.pk8");

fn checked<T, E: std::fmt::Debug>(result: Result<T, E>) -> TestResult<T> {
    result.map_err(|error| format!("{error:?}").into())
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> TestResult<Self> {
        let mut nonce = [0; 8];
        getrandom::fill(&mut nonce).map_err(|error| error.to_string())?;
        // Keep the root short enough for the real supervisor's Unix socket.
        let root = std::env::temp_dir().join(format!("r2262-{:016x}", u64::from_be_bytes(nonce)));
        fs::create_dir(&root)?;
        fs::set_permissions(&root, fs::Permissions::from_mode(0o700))?;
        Ok(Self(root))
    }
    fn write(&self, name: &str, bytes: &[u8]) -> TestResult<PathBuf> {
        let path = self.0.join(name);
        fs::write(&path, bytes)?;
        fs::set_permissions(&path, fs::Permissions::from_mode(0o600))?;
        Ok(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

struct Supervisor {
    binding: ClockBinding,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<std::io::Result<()>>>,
}
impl Supervisor {
    fn start(root: &Path) -> TestResult<Self> {
        let mut server = clock_server::Server::bind(root)?;
        let binding = ClockBinding {
            socket: server.socket().to_owned(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        };
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let worker = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                server.poll()?;
                thread::sleep(Duration::from_millis(1));
            }
            Ok(())
        });
        Ok(Self {
            binding,
            stop,
            worker: Some(worker),
        })
    }
    fn clock(&self) -> TestResult<Arc<dyn Clock>> {
        Ok(startup::runtime_clock(self.binding.clone())?)
    }
}
impl Drop for Supervisor {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            assert!(worker.join().is_ok_and(|result| result.is_ok()));
        }
    }
}

fn forwarded_binding(binding: &ClockBinding) -> TestResult<ClockBinding> {
    // Inspect the exact Command produced for a child; no subprocess is needed.
    let mut command = Command::new("layerx-human-kms-gateway");
    command.env_clear();
    startup::forward_clock_binding(&mut command, binding);
    let values: BTreeMap<_, _> = command
        .get_envs()
        .map(|(name, value)| (name.to_owned(), value.map(OsStr::to_owned)))
        .collect();
    assert_eq!(values.len(), 3);
    let value = |name| -> TestResult<OsString> {
        values
            .get(OsStr::new(name))
            .and_then(Clone::clone)
            .ok_or_else(|| format!("missing child binding {name}").into())
    };
    assert_eq!(
        value("LAYERX_RUNTIME_CLOCK_SOCKET")?,
        binding.socket.as_os_str()
    );
    assert_eq!(
        value("LAYERX_RUNTIME_CLOCK_PID")?,
        binding.process.to_string().as_str()
    );
    assert_eq!(
        value("LAYERX_RUNTIME_CLOCK_UID")?,
        binding.user.to_string().as_str()
    );
    Ok(ClockBinding {
        socket: value("LAYERX_RUNTIME_CLOCK_SOCKET")?.into(),
        process: value("LAYERX_RUNTIME_CLOCK_PID")?
            .to_str()
            .ok_or("PID encoding")?
            .parse()?,
        user: value("LAYERX_RUNTIME_CLOCK_UID")?
            .to_str()
            .ok_or("UID encoding")?
            .parse()?,
    })
}

#[test]
fn explicit_supervisor_binding_survives_child_environment_clear() -> TestResult {
    let root = Directory::new()?;
    let supervisor = Supervisor::start(&root.0)?;
    let clock = startup::runtime_clock(forwarded_binding(&supervisor.binding)?)?;
    let first = clock.sample(Duration::from_secs(1))?;
    let second = clock.sample(Duration::from_secs(1))?;
    assert!(second.follows(first).is_ok());
    assert_ne!(first.generation, [0; 16]);

    let mut wrong_peer = forwarded_binding(&supervisor.binding)?;
    wrong_peer.process = wrong_peer.process.checked_add(1).ok_or("PID overflow")?;
    let error = startup::runtime_clock(wrong_peer)
        .err()
        .ok_or("wrong supervisor accepted")?;
    assert!(error.starts_with("runtime clock unavailable:"), "{error}");
    let mut missing = forwarded_binding(&supervisor.binding)?;
    missing.socket = root.0.join("absent.sock");
    let error = startup::runtime_clock(missing)
        .err()
        .ok_or("absent supervisor accepted")?;
    assert!(error.starts_with("runtime clock unavailable:"), "{error}");
    // A refused binding never substitutes another source of time.
    assert_eq!(
        clock.sample(Duration::from_secs(1))?.generation,
        first.generation
    );
    Ok(())
}

struct Environment(Vec<(String, Option<OsString>)>);
impl Environment {
    fn install(values: BTreeMap<String, OsString>) -> Self {
        let mut previous = Vec::new();
        for (name, value) in values {
            previous.push((name.clone(), std::env::var_os(&name)));
            std::env::set_var(name, value);
        }
        for name in [
            "LAYERX_HUMAN_KMS_ATTESTOR_CONFIG_FILE",
            "LAYERX_HUMAN_KMS_EVM_CLIENT_CERT_DER",
        ] {
            previous.push((name.into(), std::env::var_os(name)));
            std::env::remove_var(name);
        }
        Self(previous)
    }
}
impl Drop for Environment {
    fn drop(&mut self) {
        for (name, value) in self.0.drain(..) {
            match value {
                Some(value) => std::env::set_var(name, value),
                None => std::env::remove_var(name),
            }
        }
    }
}

fn remote(address: SocketAddr, clock: Arc<dyn Clock>) -> TestResult<RemoteKmsProvider> {
    let mut roots = RootCertStore::empty();
    roots.add(CertificateDer::from(CA.to_vec()))?;
    let tls = checked(MutualTlsConfig::new(
        roots,
        vec![CertificateDer::from(CLIENT.to_vec())],
        PrivateKeyDer::try_from(CLIENT_KEY.to_vec())?,
    ))?;
    Ok(RemoteKmsProvider::new(
        PROVIDER,
        address,
        "localhost",
        tls,
        Limits {
            maximum_frame_bytes: GATEWAY_FRAME_LIMIT,
            maximum_connections: 4,
            maximum_streams: 1,
            maximum_queued_bytes: GATEWAY_FRAME_LIMIT,
            deadline: Duration::from_secs(5),
        },
        clock,
    )?)
}

struct GatewayWorker {
    address: SocketAddr,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl GatewayWorker {
    fn start(listener: TcpListener, gateway: LxkpGateway) -> TestResult<Self> {
        let address = listener.local_addr()?;
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let worker = thread::spawn(move || {
            for stream in listener.incoming() {
                if stopped.load(Ordering::Acquire) {
                    break;
                }
                let stream = stream.or_fail("gateway loopback accept");
                gateway.serve_connection(stream).unwrap_or_else(|error| {
                    panic!("{}: {error:?}", "production gateway connection")
                });
            }
        });
        Ok(Self {
            address,
            stop,
            worker: Some(worker),
        })
    }
}
impl Drop for GatewayWorker {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        if let Some(worker) = self.worker.take() {
            assert!(worker.join().is_ok());
        }
    }
}

fn start_backend(
    root: &Directory,
    supervisor: &Supervisor,
) -> Result<(SocketAddr, RemoteKmsProvider), Box<dyn std::error::Error>> {
    let backend_listener = TcpListener::bind("127.0.0.1:0")?;
    let backend = backend_listener.local_addr()?;
    drop(backend_listener);
    let mut backend_env = BTreeMap::new();
    for (suffix, value) in [
        ("LISTEN", OsString::from(backend.to_string())),
        ("PROVIDER_REFERENCE", OsString::from(PROVIDER)),
        ("STATE_DIR", root.0.join("state").into_os_string()),
        ("DEADLINE_SECONDS", OsString::from("5")),
    ] {
        backend_env.insert(format!("LAYERX_HUMAN_KMS_{suffix}"), value);
    }
    for (suffix, name) in [
        ("REGISTRY_FILE", "chain.json"),
        ("CLIENT_CA_DER", "ca.der"),
        ("TLS_CERT_DER", "server.der"),
        ("TLS_KEY_DER", "server.pk8"),
        ("CLIENT_CERT_DER", "client.der"),
        ("SEAL_SECRET_FILE", "seal"),
    ] {
        backend_env.insert(
            format!("LAYERX_HUMAN_KMS_{suffix}"),
            root.0.join(name).into_os_string(),
        );
    }
    // These are wholly test-owned configuration values, restored after startup;
    // the gate consumes no externally supplied environment or provider process.
    let environment = Environment::install(backend_env);
    let backend_clock = supervisor.clock()?;
    let (sender, exited) = mpsc::channel();
    thread::spawn(move || {
        let _ = sender.send(layerx_human_kms::run_from_environment(&backend_clock));
    });
    let expires = Instant::now() + Duration::from_secs(15);
    loop {
        if let Ok(result) = exited.try_recv() {
            return Err(format!("production KMS exited: {result:?}").into());
        }
        if TcpStream::connect(backend).is_ok() {
            break;
        }
        if Instant::now() >= expires {
            return Err("production KMS startup timed out".into());
        }
        thread::sleep(Duration::from_millis(5));
    }
    let direct = remote(backend, supervisor.clock()?)?;
    direct.probe()?;
    drop(environment);

    Ok((backend, direct))
}

fn gateway_settings(
    root: &Directory,
    backend: SocketAddr,
    listen: SocketAddr,
) -> TestResult<BTreeMap<String, String>> {
    let mut values = BTreeMap::new();
    for (suffix, value) in [
        ("PROVIDER_KIND", "remote-lxkp".to_owned()),
        ("LISTEN", listen.to_string()),
        ("PROVIDER_REFERENCE", PROVIDER.to_owned()),
        ("PROVIDER_ENDPOINT", backend.to_string()),
        ("PROVIDER_SERVER_NAME", "localhost".to_owned()),
        ("PROVIDER_MAX_FRAME_BYTES", GATEWAY_FRAME_LIMIT.to_string()),
        ("PROVIDER_MAX_CONNECTIONS", "4".to_owned()),
        ("PROVIDER_MAX_STREAMS", "1".to_owned()),
        ("PROVIDER_MAX_QUEUED_BYTES", GATEWAY_FRAME_LIMIT.to_string()),
        ("PROVIDER_DEADLINE_SECONDS", "5".to_owned()),
        ("MAX_CONNECTIONS", "8".to_owned()),
        ("MAX_FRAME_BYTES", GATEWAY_FRAME_LIMIT.to_string()),
        ("DEADLINE_SECONDS", "5".to_owned()),
        ("RATE_MAXIMUM", "1000".to_owned()),
        ("RATE_WINDOW_SECONDS", "60".to_owned()),
    ] {
        values.insert(format!("{PREFIX}_{suffix}"), value);
    }
    for (suffix, name) in [
        ("REGISTRY_FILE", "chain.json"),
        ("CLIENT_CA_DER", "ca.der"),
        ("TLS_CERT_DER", "server.der"),
        ("TLS_KEY_DER", "server.pk8"),
        ("CLIENT_CERT_DER", "client.der"),
        ("PROVIDER_ROOT_CERTIFICATE_DER", "ca.der"),
        ("PROVIDER_CLIENT_CERTIFICATE_DER", "client.der"),
        ("PROVIDER_CLIENT_PRIVATE_KEY_DER", "client.pk8"),
    ] {
        values.insert(
            format!("{PREFIX}_{suffix}"),
            root.0
                .join(name)
                .to_str()
                .ok_or("path encoding")?
                .to_owned(),
        );
    }

    Ok(values)
}

#[test]
fn startup_refusals_and_chain_snapshot_round_trip_use_the_real_provider() -> TestResult {
    let root = Directory::new()?;
    let supervisor = Supervisor::start(&root.0)?;
    for (name, bytes) in [
        ("ca.der", CA),
        ("server.der", SERVER),
        ("server.pk8", SERVER_KEY),
        ("client.der", CLIENT),
        ("client.pk8", CLIENT_KEY),
        ("chain.json", br#"{"chain_id":125}"#.as_slice()),
        ("wrong-chain.json", br#"{"chain_id":77}"#.as_slice()),
    ] {
        root.write(name, bytes)?;
    }
    let mut seal = [0; 32];
    getrandom::fill(&mut seal).map_err(|error| error.to_string())?;
    root.write("seal", &seal)?;

    let (backend, direct) = start_backend(&root, &supervisor)?;
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let listen = listener.local_addr()?;
    let values = gateway_settings(&root, backend, listen)?;

    let gateway_clock = startup::runtime_clock(forwarded_binding(&supervisor.binding)?)?;
    for (suffix, value, expected) in [
        ("PROVIDER_KIND", "development".to_owned(), "LAYERX_HUMAN_KMS_GATEWAY_PROVIDER_KIND must name the non-exportable remote KMS provider remote-lxkp; development envelope providers never back the gateway"),
        ("PROVIDER_KIND", "envelope".to_owned(), "LAYERX_HUMAN_KMS_GATEWAY_PROVIDER_KIND must name the non-exportable remote KMS provider remote-lxkp; development envelope providers never back the gateway"),
        ("PROVIDER_KIND", String::new(), "LAYERX_HUMAN_KMS_GATEWAY_PROVIDER_KIND is required"),
        ("PROVIDER_ENDPOINT", listen.to_string(), "the gateway cannot be its own KMS provider"),
        ("MAX_CONNECTIONS", "0".to_owned(), "the gateway was refused: invalid-limits"),
        ("REGISTRY_FILE", root.0.join("wrong-chain.json").to_str().ok_or("path encoding")?.to_owned(), "LAYERX_HUMAN_KMS_GATEWAY_REGISTRY_FILE chain_id must be 125"),
    ] {
        let mut refused = values.clone();
        refused.insert(format!("{PREFIX}_{suffix}"), value);
        let error = startup::prepare(gateway_clock.clone(), &|name| refused.get(name).cloned())
            .err().ok_or("invalid startup accepted")?;
        assert_eq!(error, expected);
    }
    // Hold a bound, non-listening socket so the absent provider cannot be replaced
    // by a racing listener on a released port.
    let absent = rustix::net::socket(
        rustix::net::AddressFamily::INET,
        rustix::net::SocketType::STREAM,
        None,
    )?;
    rustix::net::bind(&absent, &"127.0.0.1:0".parse::<std::net::SocketAddrV4>()?)?;
    let absent_address = SocketAddr::try_from(rustix::net::getsockname(&absent)?)?;
    let mut refused = values.clone();
    refused.insert(
        format!("{PREFIX}_PROVIDER_ENDPOINT"),
        absent_address.to_string(),
    );
    let error = startup::prepare(gateway_clock.clone(), &|name| refused.get(name).cloned())
        .err()
        .ok_or("absent provider accepted")?;
    assert_eq!(error, "the gateway was refused: kms-timeout");

    let snapshot = startup::snapshot(&root.0.join("chain.json"))?;
    assert_eq!(snapshot.chain_id, 125);
    let (configured_listen, gateway) =
        startup::prepare(gateway_clock, &|name| values.get(name).cloned())?;
    assert_eq!(configured_listen, listen);
    let _worker = GatewayWorker::start(listener, gateway)?;
    let client = remote(listen, supervisor.clock()?)?;
    client.probe()?;
    let binding = startup::probe_binding(&snapshot, PROVIDER)?;
    let expected_binding =
        PrincipalKeyBinding::new(b"carol".to_vec(), 125, KeyClass::HumanPrimary, PROVIDER)?;
    assert_eq!(binding.digest(), expected_binding.digest());
    let wrong_network =
        PrincipalKeyBinding::new(b"carol".to_vec(), 77, KeyClass::HumanPrimary, PROVIDER)?;
    assert_eq!(client.create_key(&wrong_network), Err(KmsError::Refused));
    // The real producer emits each request, both TLS hops transport it, and
    // the actual KMS store creates and describes the key consumed by the client.
    let created = client.create_key(&binding)?;
    assert_eq!(created.binding_digest(), binding.digest());
    assert_eq!(client.describe_key(&binding, created.reference())?, created);
    assert_eq!(direct.describe_key(&binding, created.reference())?, created);
    assert_eq!(
        client.describe_key(&wrong_network, created.reference()),
        Err(KmsError::Refused)
    );
    assert!(root.0.join("state/state.aead").is_file());
    Ok(())
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
