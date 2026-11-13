use std::fs;
use std::io;
use std::net::{Shutdown, SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_human_service::custody::{
    GatewayLimits, GatewayPolicy, GatewayTls, KmsProvider, LxkpGateway, RemoteKmsProvider,
    GATEWAY_FRAME_LIMIT,
};
use layerx_human_service::frame::{ConnectionGate, FrameTransport, Limits, MutualTlsConfig, Tls};
use layerx_human_test_support::StepClock;
use layerx_types::clock::{Clock, Deadline};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use rustls::pki_types::{CertificateDer, PrivateKeyDer};
use rustls::RootCertStore;
use zeroize::Zeroizing;

type Result<T = ()> = std::result::Result<T, Box<dyn std::error::Error>>;
const TEST: &str = "custody_gateway_relays_an_lxkp_probe_over_the_human_frame_module";
const CHILD: &str = "HUMAN_CUSTODY_FRAME_KMS_CHILD";
const PROVIDER: &str = "custody-frame-provider";
const DEADLINE: Duration = Duration::from_secs(5);

struct Host {
    root: PathBuf,
    child: Option<Child>,
    clock: Arc<dyn Clock>,
}

impl Drop for Host {
    fn drop(&mut self) {
        if let Some(child) = self.child.as_mut() {
            let _ = child.kill();
            let _ = child.wait();
        }
        let _ = fs::remove_dir_all(&self.root);
    }
}

impl Host {
    fn new() -> Result<Self> {
        let mut random = [0; 16];
        getrandom::fill(&mut random).map_err(|error| io::Error::other(error.to_string()))?;
        let host = Self {
            root: std::env::temp_dir().join(format!(
                "custody-frame-{:032x}",
                u128::from_be_bytes(random)
            )),
            child: None,
            clock: Arc::new(StepClock::new(1_700_000_000_000)),
        };
        fs::create_dir(&host.root)?;
        fs::set_permissions(&host.root, fs::Permissions::from_mode(0o700))?;
        host.openssl(&[
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
            "/CN=Custody frame CA",
            "-addext",
            "basicConstraints=critical,CA:TRUE",
        ])?;
        host.openssl(&["x509", "-in", "ca.pem", "-outform", "DER", "-out", "ca.der"])?;
        host.generate_identities()?;
        let mut seal = Zeroizing::new([0; 32]);
        getrandom::fill(&mut *seal).map_err(|error| io::Error::other(error.to_string()))?;
        fs::write(host.path("seal"), *seal)?;
        fs::write(
            host.path("registry.json"),
            serde_json::to_vec(&serde_json::json!({"chain_id": 125}))?,
        )?;
        for entry in fs::read_dir(&host.root)? {
            fs::set_permissions(entry?.path(), fs::Permissions::from_mode(0o600))?;
        }
        Ok(host)
    }

    fn generate_identities(&self) -> Result<()> {
        for (identity, extension) in [
            (
                "server",
                "subjectAltName=DNS:localhost\nextendedKeyUsage=serverAuth\n",
            ),
            ("client", "extendedKeyUsage=clientAuth\n"),
        ] {
            self.openssl(&[
                "req",
                "-newkey",
                "ec",
                "-pkeyopt",
                "ec_paramgen_curve:P-256",
                "-nodes",
                "-keyout",
                &format!("{identity}.key"),
                "-out",
                &format!("{identity}.csr"),
                "-subj",
                &format!("/CN={identity}"),
            ])?;
            fs::write(self.path("extensions"), extension)?;
            self.openssl(&[
                "x509",
                "-req",
                "-in",
                &format!("{identity}.csr"),
                "-CA",
                "ca.pem",
                "-CAkey",
                "ca.key",
                "-CAcreateserial",
                "-out",
                &format!("{identity}.pem"),
                "-days",
                "1",
                "-extfile",
                "extensions",
            ])?;
            self.openssl(&[
                "x509",
                "-in",
                &format!("{identity}.pem"),
                "-outform",
                "DER",
                "-out",
                &format!("{identity}.der"),
            ])?;
            self.openssl(&[
                "pkcs8",
                "-topk8",
                "-nocrypt",
                "-in",
                &format!("{identity}.key"),
                "-outform",
                "DER",
                "-out",
                &format!("{identity}-key.der"),
            ])?;
        }
        Ok(())
    }

    fn path(&self, file: &str) -> PathBuf {
        self.root.join(file)
    }

    fn openssl(&self, arguments: &[&str]) -> Result {
        let output = Command::new("openssl")
            .args(arguments)
            .current_dir(&self.root)
            .output()?;
        if !output.status.success() {
            return Err(
                io::Error::other(String::from_utf8_lossy(&output.stderr).into_owned()).into(),
            );
        }
        Ok(())
    }

    fn tls(&self) -> Result<MutualTlsConfig> {
        let mut roots = RootCertStore::empty();
        roots.add(CertificateDer::from(fs::read(self.path("ca.der"))?))?;
        MutualTlsConfig::new(
            roots,
            vec![CertificateDer::from(fs::read(self.path("client.der"))?)],
            PrivateKeyDer::try_from(fs::read(self.path("client-key.der"))?)?,
        )
        .map_err(|error| io::Error::other(format!("TLS configuration: {error:?}")).into())
    }

    fn provider(&self, endpoint: SocketAddr) -> Result<RemoteKmsProvider> {
        Ok(RemoteKmsProvider::new(
            PROVIDER,
            endpoint,
            "localhost",
            self.tls()?,
            limits(),
            Arc::clone(&self.clock),
        )?)
    }

    fn start(&mut self, address: SocketAddr) -> Result {
        let mut command = Command::new(std::env::current_exe()?);
        command
            .args(["--exact", TEST, "--nocapture"])
            .env(CHILD, "1")
            .env("LAYERX_HUMAN_KMS_LISTEN", address.to_string())
            .env("LAYERX_HUMAN_KMS_PROVIDER_REFERENCE", PROVIDER)
            .env("LAYERX_HUMAN_KMS_STATE_DIR", self.path("state"))
            .env("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "5");
        for (suffix, file) in [
            ("REGISTRY_FILE", "registry.json"),
            ("CLIENT_CA_DER", "ca.der"),
            ("TLS_CERT_DER", "server.der"),
            ("TLS_KEY_DER", "server-key.der"),
            ("CLIENT_CERT_DER", "client.der"),
            ("SEAL_SECRET_FILE", "seal"),
        ] {
            command.env(format!("LAYERX_HUMAN_KMS_{suffix}"), self.path(file));
        }
        command.env_remove("LAYERX_HUMAN_KMS_EVM_CLIENT_CERT_DER");
        let log = fs::File::create(self.path("kms.log"))?;
        self.child = Some(
            command
                .stdout(Stdio::from(log.try_clone()?))
                .stderr(Stdio::from(log))
                .spawn()?,
        );
        let mut expires = Deadline::start(self.clock.as_ref(), Duration::from_secs(15))?;
        let provider = self.provider(address)?;
        loop {
            if provider.probe().is_ok() {
                return Ok(());
            }
            if self
                .child
                .as_mut()
                .ok_or("KMS process missing")?
                .try_wait()?
                .is_some()
                || expires.remaining(self.clock.as_ref())?.is_zero()
            {
                return Err(format!(
                    "KMS startup failed: {}",
                    fs::read_to_string(self.path("kms.log"))?
                )
                .into());
            }
            thread::sleep(Duration::from_millis(20));
        }
    }
}

fn registry() -> Result<ModuleRegistry> {
    let activity =
        ActivityType::new(ModuleId::Asset, 1).map_err(|error| format!("activity: {error:?}"))?;
    let module = ModuleRegistration::new(ModuleId::Asset, &[activity])
        .map_err(|error| format!("module: {error:?}"))?;
    ModuleRegistry::new(&[module]).map_err(|error| format!("registry: {error:?}").into())
}

fn limits() -> Limits {
    Limits {
        maximum_frame_bytes: GATEWAY_FRAME_LIMIT,
        maximum_connections: 4,
        maximum_streams: 1,
        maximum_queued_bytes: GATEWAY_FRAME_LIMIT,
        deadline: DEADLINE,
    }
}

fn relay(mut tcp: TcpStream, mut unix: UnixStream) -> io::Result<(u64, u64)> {
    tcp.set_read_timeout(Some(DEADLINE))?;
    tcp.set_write_timeout(Some(DEADLINE))?;
    unix.set_read_timeout(Some(DEADLINE))?;
    unix.set_write_timeout(Some(DEADLINE))?;
    let mut tcp_reader = tcp.try_clone()?;
    let mut unix_writer = unix.try_clone()?;
    let outgoing = thread::spawn(move || -> io::Result<u64> {
        let bytes = io::copy(&mut tcp_reader, &mut unix_writer)?;
        unix_writer.shutdown(Shutdown::Write)?;
        Ok(bytes)
    });
    let incoming = io::copy(&mut unix, &mut tcp)?;
    tcp.shutdown(Shutdown::Write)?;
    let outgoing = outgoing
        .join()
        .map_err(|_| io::Error::other("Unix relay panicked"))??;
    Ok((outgoing, incoming))
}

type TunnelJoin = JoinHandle<io::Result<Vec<(u64, u64)>>>;

fn unix_tunnel(
    listener: TcpListener,
    socket: &Path,
    backend: SocketAddr,
) -> Result<[TunnelJoin; 2]> {
    let unix_listener = UnixListener::bind(socket)?;
    let socket = socket.to_path_buf();
    let front = thread::spawn(move || {
        (0..2)
            .map(|_| {
                let (tcp, _) = listener.accept()?;
                relay(tcp, UnixStream::connect(&socket)?)
            })
            .collect()
    });
    let back = thread::spawn(move || {
        (0..2)
            .map(|_| {
                let (unix, _) = unix_listener.accept()?;
                relay(TcpStream::connect_timeout(&backend, DEADLINE)?, unix)
            })
            .collect()
    });
    Ok([front, back])
}

#[test]
fn custody_gateway_relays_an_lxkp_probe_over_the_human_frame_module() -> Result {
    if std::env::var_os(CHILD).is_some() {
        let clock: Arc<dyn Clock> = Arc::new(StepClock::new(1_700_000_000_000));
        return layerx_human_kms::run_from_environment(&clock).map_err(Into::into);
    }
    rustls::crypto::ring::default_provider()
        .install_default()
        .map_err(|_| "TLS provider already installed")?;
    let mut host = Host::new()?;
    let backend_listener = TcpListener::bind("127.0.0.1:0")?;
    let backend = backend_listener.local_addr()?;
    let tunnel_listener = TcpListener::bind("127.0.0.1:0")?;
    let tunnel = tunnel_listener.local_addr()?;
    let gateway_listener = TcpListener::bind("127.0.0.1:0")?;
    let address = gateway_listener.local_addr()?;
    drop(backend_listener);
    host.start(backend)?;
    let workers = unix_tunnel(tunnel_listener, &host.path("kms.sock"), backend)?;
    let gateway = Arc::new(LxkpGateway::new(
        Arc::new(host.provider(tunnel)?),
        GatewayTls::new(
            &fs::read(host.path("ca.der"))?,
            &fs::read(host.path("server.der"))?,
            &Zeroizing::new(fs::read(host.path("server-key.der"))?),
            &fs::read(host.path("client.der"))?,
        )?,
        GatewayPolicy {
            network_id: 42,
            protocol_version: 3,
            registry: registry()?,
            limits: GatewayLimits {
                maximum_connections: 4,
                maximum_frame_bytes: GATEWAY_FRAME_LIMIT,
                deadline: DEADLINE,
                maximum_operations: 2,
                operation_window: Duration::from_secs(60),
            },
        },
        Arc::clone(&host.clock),
    )?);
    gateway_listener.set_nonblocking(true)?;
    let gate = ConnectionGate::new(1);
    let mut client = Tls::connect(
        address,
        "localhost".try_into()?,
        &host.tls()?,
        &gate,
        limits(),
    )
    .map_err(|error| format!("connect gateway: {error:?}"))?;
    let server = thread::spawn(move || gateway.serve(&gateway_listener));
    let mut request = b"LXKP\x00\x01\x00".to_vec();
    request.extend_from_slice(&u32::try_from(PROVIDER.len())?.to_be_bytes());
    request.extend_from_slice(PROVIDER.as_bytes());
    assert_eq!(client.send(&request), Ok(()));
    assert_eq!(client.receive(), Ok(b"LXKP\x00\x01\x00\x00".to_vec()));
    drop(client);
    assert_eq!(gate.active(), 0);
    for worker in workers {
        let exchanges = worker.join().map_err(|_| "Unix tunnel panicked")??;
        assert_eq!(exchanges.len(), 2);
        assert!(exchanges
            .iter()
            .all(|(sent, received)| *sent > 0 && *received > 0));
    }
    let stopped = server.join().map_err(|_| "gateway panicked")?;
    assert!(
        matches!(stopped, Err(layerx_human_service::custody::CustodyError::Io(error)) if error.kind() == io::ErrorKind::WouldBlock)
    );
    Ok(())
}
