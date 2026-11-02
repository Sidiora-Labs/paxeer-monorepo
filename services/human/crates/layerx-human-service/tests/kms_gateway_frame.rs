use std::error::Error;
use std::fs;
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, ExitStatus, Stdio};
use std::sync::Arc;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use layerx_human_service::custody::GATEWAY_FRAME_LIMIT;
use layerx_human_service::frame::{ConnectionGate, FrameTransport, Limits, MutualTlsConfig, Tls};
use layerx_types::clock::{Clock, ClockError, ClockReading};
use rustls::pki_types::{CertificateDer, PrivateKeyDer, ServerName};
use rustls::RootCertStore;

type TestResult<T = ()> = Result<T, Box<dyn Error>>;
const PROVIDER: &str = "gateway-frame-kms";
const BACKEND_PROCESS: &str = "LAYERX_GATEWAY_FRAME_BACKEND_PROCESS";
const STARTUP_LIMIT: Duration = Duration::from_secs(15);

struct Process(Child);

impl Process {
    fn wait(&mut self) -> TestResult<ExitStatus> {
        let started = Instant::now();
        loop {
            if let Some(status) = self.0.try_wait()? {
                return Ok(status);
            }
            if started.elapsed() >= STARTUP_LIMIT {
                return Err("process did not exit before the startup deadline".into());
            }
            std::thread::sleep(Duration::from_millis(20));
        }
    }
}

impl Drop for Process {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

struct Fixture(PathBuf);

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

impl Fixture {
    fn new() -> TestResult<Self> {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let mut unique = [0; 16];
        getrandom::fill(&mut unique).map_err(|error| error.to_string())?;
        let root = std::env::temp_dir().join(format!(
            "kms-gateway-frame-{:032x}",
            u128::from_be_bytes(unique)
        ));
        fs::create_dir(&root)?;
        fs::set_permissions(&root, fs::Permissions::from_mode(0o700))?;
        let fixture = Self(root);
        fixture.openssl(&[
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
            "/CN=Gateway frame CA",
            "-addext",
            "basicConstraints=critical,CA:TRUE",
        ])?;
        fixture.openssl(&["x509", "-in", "ca.pem", "-outform", "DER", "-out", "ca.der"])?;
        for identity in ["backend", "gateway", "relay", "service"] {
            fixture.openssl(&[
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
            fixture.write(
                "extensions",
                if matches!(identity, "backend" | "gateway") {
                    b"subjectAltName=DNS:localhost\nextendedKeyUsage=serverAuth\n"
                } else {
                    b"extendedKeyUsage=clientAuth\n"
                },
            )?;
            fixture.openssl(&[
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
            fixture.openssl(&[
                "x509",
                "-in",
                &format!("{identity}.pem"),
                "-outform",
                "DER",
                "-out",
                &format!("{identity}.der"),
            ])?;
            fixture.openssl(&[
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
        let mut seal = [0; 32];
        getrandom::fill(&mut seal).map_err(|error| error.to_string())?;
        fixture.write("seal", &seal)?;
        fixture.write("backend.json", br#"{"chain_id":125}"#)?;
        for entry in fs::read_dir(&fixture.0)? {
            fs::set_permissions(entry?.path(), fs::Permissions::from_mode(0o600))?;
        }
        Ok(fixture)
    }

    fn write(&self, file: &str, bytes: &[u8]) -> TestResult {
        fs::write(self.0.join(file), bytes)?;
        fs::set_permissions(self.0.join(file), fs::Permissions::from_mode(0o600))?;
        Ok(())
    }

    fn openssl(&self, args: &[&str]) -> TestResult {
        let output = Command::new("openssl")
            .args(args)
            .current_dir(&self.0)
            .output()?;
        if !output.status.success() {
            return Err(format!("openssl: {}", String::from_utf8_lossy(&output.stderr)).into());
        }
        Ok(())
    }

    fn gateway(&self, address: SocketAddr, backend: SocketAddr) -> TestResult<Process> {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-human-kms-gateway"));
        command.env_clear();
        for (suffix, value) in [
            ("PROVIDER_KIND", "remote-lxkp".to_owned()),
            ("LISTEN", address.to_string()),
            ("PROVIDER_REFERENCE", PROVIDER.to_owned()),
            ("PROVIDER_ENDPOINT", backend.to_string()),
            ("PROVIDER_SERVER_NAME", "localhost".to_owned()),
            ("PROVIDER_MAX_FRAME_BYTES", GATEWAY_FRAME_LIMIT.to_string()),
            ("PROVIDER_MAX_CONNECTIONS", "4".to_owned()),
            ("PROVIDER_MAX_STREAMS", "1".to_owned()),
            ("PROVIDER_MAX_QUEUED_BYTES", GATEWAY_FRAME_LIMIT.to_string()),
            ("PROVIDER_DEADLINE_SECONDS", "2".to_owned()),
            ("MAX_CONNECTIONS", "8".to_owned()),
            ("MAX_FRAME_BYTES", GATEWAY_FRAME_LIMIT.to_string()),
            ("DEADLINE_SECONDS", "2".to_owned()),
            ("RATE_MAXIMUM", "100".to_owned()),
            ("RATE_WINDOW_SECONDS", "60".to_owned()),
        ] {
            command.env(format!("LAYERX_HUMAN_KMS_GATEWAY_{suffix}"), value);
        }
        for (suffix, file) in [
            ("REGISTRY_FILE", "snapshot.json"),
            ("CLIENT_CA_DER", "ca.der"),
            ("TLS_CERT_DER", "gateway.der"),
            ("TLS_KEY_DER", "gateway-key.der"),
            ("CLIENT_CERT_DER", "service.der"),
            ("PROVIDER_ROOT_CERTIFICATE_DER", "ca.der"),
            ("PROVIDER_CLIENT_CERTIFICATE_DER", "relay.der"),
            ("PROVIDER_CLIENT_PRIVATE_KEY_DER", "relay-key.der"),
        ] {
            command.env(
                format!("LAYERX_HUMAN_KMS_GATEWAY_{suffix}"),
                self.0.join(file),
            );
        }
        self.spawn(command, "gateway.log")
    }

    fn spawn(&self, mut command: Command, log: &str) -> TestResult<Process> {
        Ok(Process(
            command
                .stdout(Stdio::null())
                .stderr(fs::File::create(self.0.join(log))?)
                .spawn()?,
        ))
    }

    fn backend(&self, address: SocketAddr) -> TestResult<Process> {
        let mut command = Command::new(std::env::current_exe()?);
        command
            .args([
                "--exact",
                "kms_gateway_starts_with_a_chain_snapshot_document",
                "--nocapture",
            ])
            .env_clear()
            .env(BACKEND_PROCESS, "1")
            .env("LAYERX_HUMAN_KMS_LISTEN", address.to_string())
            .env("LAYERX_HUMAN_KMS_PROVIDER_REFERENCE", PROVIDER)
            .env("LAYERX_HUMAN_KMS_STATE_DIR", self.0.join("state"))
            .env("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "2");
        for (suffix, file) in [
            ("REGISTRY_FILE", "backend.json"),
            ("CLIENT_CA_DER", "ca.der"),
            ("TLS_CERT_DER", "backend.der"),
            ("TLS_KEY_DER", "backend-key.der"),
            ("CLIENT_CERT_DER", "relay.der"),
            ("SEAL_SECRET_FILE", "seal"),
        ] {
            command.env(format!("LAYERX_HUMAN_KMS_{suffix}"), self.0.join(file));
        }
        self.spawn(command, "backend.log")
    }

    fn probe(&self, address: SocketAddr, identity: &str) -> TestResult {
        let mut roots = RootCertStore::empty();
        roots.add(CertificateDer::from(fs::read(self.0.join("ca.der"))?))?;
        let tls = MutualTlsConfig::new(
            roots,
            vec![CertificateDer::from(fs::read(
                self.0.join(format!("{identity}.der")),
            )?)],
            PrivateKeyDer::try_from(fs::read(self.0.join(format!("{identity}-key.der")))?)?,
        )
        .map_err(|error| format!("TLS configuration: {error:?}"))?;
        let limits = Limits {
            maximum_frame_bytes: GATEWAY_FRAME_LIMIT,
            maximum_connections: 1,
            maximum_streams: 1,
            maximum_queued_bytes: GATEWAY_FRAME_LIMIT,
            deadline: Duration::from_secs(2),
        };
        let mut connection = Tls::connect(
            address,
            ServerName::try_from("localhost")?,
            &tls,
            &ConnectionGate::new(1),
            limits,
        )
        .map_err(|error| format!("connect: {error:?}"))?;
        let mut request = b"LXKP\0\x01\0".to_vec();
        request.extend(u32::try_from(PROVIDER.len())?.to_be_bytes());
        request.extend(PROVIDER.as_bytes());
        connection
            .send(&request)
            .map_err(|error| format!("send: {error:?}"))?;
        let response = connection
            .receive()
            .map_err(|error| format!("receive: {error:?}"))?;
        assert_eq!(response, b"LXKP\0\x01\0\0");
        Ok(())
    }
}

struct OsClock {
    start: Instant,
    generation: [u8; 16],
}

impl Clock for OsClock {
    fn sample(&self, _transport_budget: Duration) -> Result<ClockReading, ClockError> {
        Ok(ClockReading {
            generation: self.generation,
            unix_milliseconds: u64::try_from(
                SystemTime::now()
                    .duration_since(UNIX_EPOCH)
                    .map_err(|_| ClockError::Invalid)?
                    .as_millis(),
            )
            .map_err(|_| ClockError::Overflow)?,
            monotonic_nanoseconds: u64::try_from(self.start.elapsed().as_nanos())
                .map_err(|_| ClockError::Overflow)?,
        })
    }
}

fn free_address() -> TestResult<SocketAddr> {
    Ok(TcpListener::bind("127.0.0.1:0")?.local_addr()?)
}

fn wait_for_socket(process: &mut Process, address: SocketAddr, log: &Path) -> TestResult {
    let started = Instant::now();
    loop {
        if let Some(status) = process.0.try_wait()? {
            return Err(format!("startup exited {status}: {}", fs::read_to_string(log)?).into());
        }
        if TcpStream::connect_timeout(&address, Duration::from_millis(100)).is_ok() {
            return Ok(());
        }
        if started.elapsed() >= STARTUP_LIMIT {
            return Err(format!("startup deadline: {}", fs::read_to_string(log)?).into());
        }
        std::thread::sleep(Duration::from_millis(20));
    }
}

#[test]
fn kms_gateway_refuses_a_registry_document_with_modules() -> TestResult {
    let fixture = Fixture::new()?;
    for document in [
        br#"{"network_id":125,"protocol_version":3,"modules":[]}"#.as_slice(),
        br#"{"chain_id":125,"modules":[]}"#.as_slice(),
        br#"{"chain_id":125,"modules":[{"module_id":1,"activity_types":[65537]}]}"#.as_slice(),
        br#"{"chain_id":125,"modules":null}"#.as_slice(),
        br#"{"chain_id":124}"#.as_slice(),
        br#"{"chain_id":126}"#.as_slice(),
    ] {
        fixture.write("snapshot.json", document)?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let backend = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let backend_address = backend.local_addr()?;
        drop(listener);
        let mut gateway = fixture.gateway(address, backend_address)?;
        assert!(!gateway.wait()?.success(), "accepted {document:?}");
        let stderr = fs::read_to_string(fixture.0.join("gateway.log"))?;
        assert!(
            stderr.contains("layerx-human-kms-gateway refused startup"),
            "{stderr}"
        );
        assert!(
            stderr.contains("LAYERX_HUMAN_KMS_GATEWAY_REGISTRY_FILE"),
            "{stderr}"
        );
        assert!(TcpStream::connect(address).is_err());
    }
    Ok(())
}

#[test]
fn kms_gateway_starts_with_a_chain_snapshot_document() -> TestResult {
    if std::env::var_os(BACKEND_PROCESS).is_some() {
        let mut generation = [0; 16];
        getrandom::fill(&mut generation).map_err(|error| error.to_string())?;
        let clock: Arc<dyn Clock> = Arc::new(OsClock {
            start: Instant::now(),
            generation,
        });
        return layerx_human_kms::run_from_environment(&clock).map_err(Into::into);
    }
    let fixture = Fixture::new()?;
    fixture.write("snapshot.json", br#"{"chain_id":125}"#)?;
    let backend_address = free_address()?;
    let mut backend = fixture.backend(backend_address)?;
    wait_for_socket(
        &mut backend,
        backend_address,
        &fixture.0.join("backend.log"),
    )?;
    fixture.probe(backend_address, "relay")?;
    let address = free_address()?;
    let mut gateway = fixture.gateway(address, backend_address)?;
    wait_for_socket(&mut gateway, address, &fixture.0.join("gateway.log"))?;
    fixture.probe(address, "service")?;
    assert!(gateway.0.try_wait()?.is_none());
    Ok(())
}
