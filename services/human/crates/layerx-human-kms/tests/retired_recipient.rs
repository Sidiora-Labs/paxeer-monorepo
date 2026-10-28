#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod runtime_clock_server;

use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_types::clock::{Clock, Deadline};
use rustls::pki_types::{CertificateDer, PrivateKeyDer, ServerName};
use rustls::{ClientConfig, ClientConnection, RootCertStore, StreamOwned};
use std::error::Error;
use std::fs;
use std::io::{ErrorKind, Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::{DirBuilderExt, PermissionsExt};
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::JoinHandle;
use std::time::Duration;

type Result<T> = std::result::Result<T, Box<dyn Error>>;
const PROVIDER: &[u8] = b"retired-recipient-kms";

struct ClockAuthority {
    socket: PathBuf,
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<std::io::Result<()>>>,
}

impl ClockAuthority {
    fn start() -> Result<Self> {
        let mut server = runtime_clock_server::Server::bind(&std::env::temp_dir())?;
        let socket = server.socket().to_owned();
        let shutdown = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&shutdown);
        let worker = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                server.poll()?;
                std::thread::sleep(Duration::from_millis(1));
            }
            Ok(())
        });
        Ok(Self {
            socket,
            shutdown,
            worker: Some(worker),
        })
    }
}

impl Drop for ClockAuthority {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

struct Host {
    root: PathBuf,
    address: SocketAddr,
    child: Option<Child>,
    clock: ClockAuthority,
}

impl Drop for Host {
    fn drop(&mut self) {
        self.stop();
        let _ = fs::remove_dir_all(&self.root);
    }
}

impl Host {
    fn new() -> Result<Self> {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let mut nonce = [0; 16];
        getrandom::fill(&mut nonce).map_err(|error| std::io::Error::other(error.to_string()))?;
        let root = std::env::temp_dir().join(format!(
            "lxkp-retired-recipient-{}-{:x}",
            std::process::id(),
            u128::from_be_bytes(nonce)
        ));
        fs::DirBuilder::new().mode(0o700).create(&root)?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        drop(listener);
        let mut host = Self {
            root,
            address,
            child: None,
            clock: ClockAuthority::start()?,
        };
        host.provision_tls()?;
        let mut seal = [0; 32];
        getrandom::fill(&mut seal).map_err(|error| std::io::Error::other(error.to_string()))?;
        fs::write(host.root.join("seal"), seal)?;
        fs::write(host.root.join("snapshot.json"), br#"{"chain_id":125}"#)?;
        for entry in fs::read_dir(&host.root)? {
            fs::set_permissions(entry?.path(), fs::Permissions::from_mode(0o600))?;
        }
        host.start()?;
        Ok(host)
    }

    fn provision_tls(&self) -> Result<()> {
        self.openssl(&[
            "req",
            "-x509",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-keyout",
            "ca.key",
            "-out",
            "ca.pem",
            "-days",
            "1",
            "-subj",
            "/CN=LXKP test CA",
            "-addext",
            "basicConstraints=critical,CA:TRUE",
        ])?;
        for name in ["server", "client"] {
            self.openssl(&[
                "req",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-keyout",
                &format!("{name}.key"),
                "-out",
                &format!("{name}.csr"),
                "-subj",
                &format!("/CN={name}"),
            ])?;
            fs::write(
                self.root.join("extensions"),
                if name == "server" {
                    "subjectAltName=DNS:localhost\nextendedKeyUsage=serverAuth\n"
                } else {
                    "extendedKeyUsage=clientAuth\n"
                },
            )?;
            self.openssl(&[
                "x509",
                "-req",
                "-in",
                &format!("{name}.csr"),
                "-CA",
                "ca.pem",
                "-CAkey",
                "ca.key",
                "-CAcreateserial",
                "-out",
                &format!("{name}.pem"),
                "-days",
                "1",
                "-extfile",
                "extensions",
            ])?;
            self.openssl(&[
                "x509",
                "-in",
                &format!("{name}.pem"),
                "-outform",
                "DER",
                "-out",
                &format!("{name}.der"),
            ])?;
            self.openssl(&[
                "pkcs8",
                "-topk8",
                "-nocrypt",
                "-in",
                &format!("{name}.key"),
                "-outform",
                "DER",
                "-out",
                &format!("{name}-key.der"),
            ])?;
        }
        self.openssl(&["x509", "-in", "ca.pem", "-outform", "DER", "-out", "ca.der"])?;
        Ok(())
    }
    fn openssl(&self, arguments: &[&str]) -> Result<()> {
        let result = Command::new("openssl")
            .args(arguments)
            .current_dir(&self.root)
            .output()?;
        if !result.status.success() {
            return Err(format!(
                "openssl failed: {}",
                String::from_utf8_lossy(&result.stderr)
            )
            .into());
        }
        Ok(())
    }
    fn launch(&self) -> Result<Child> {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-human-kms"));
        command
            .env("LAYERX_HUMAN_KMS_LISTEN", self.address.to_string())
            .env(
                "LAYERX_HUMAN_KMS_PROVIDER_REFERENCE",
                "retired-recipient-kms",
            )
            .env("LAYERX_HUMAN_KMS_STATE_DIR", self.root.join("state"))
            .env("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "2")
            .env_remove("LAYERX_HUMAN_KMS_EVM_CLIENT_CERT_DER")
            .env_remove("LAYERX_HUMAN_KMS_ATTESTOR_CONFIG_FILE");
        for (suffix, file) in [
            ("REGISTRY_FILE", "snapshot.json"),
            ("CLIENT_CA_DER", "ca.der"),
            ("TLS_CERT_DER", "server.der"),
            ("TLS_KEY_DER", "server-key.der"),
            ("CLIENT_CERT_DER", "client.der"),
            ("SEAL_SECRET_FILE", "seal"),
        ] {
            command.env(format!("LAYERX_HUMAN_KMS_{suffix}"), self.root.join(file));
        }
        command
            .env("LAYERX_RUNTIME_CLOCK_SOCKET", &self.clock.socket)
            .env("LAYERX_RUNTIME_CLOCK_PID", std::process::id().to_string())
            .env(
                "LAYERX_RUNTIME_CLOCK_UID",
                rustix::process::geteuid().as_raw().to_string(),
            );
        Ok(command
            .stdout(Stdio::null())
            .stderr(Stdio::inherit())
            .spawn()?)
    }
    fn start(&mut self) -> Result<()> {
        self.child = Some(self.launch()?);
        let clock = RuntimeClock::connect(ClockBinding {
            socket: self.clock.socket.clone(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        })?;
        let mut deadline = Deadline::start(clock.as_ref(), Duration::from_secs(10))?;
        loop {
            if TcpStream::connect(self.address).is_ok() {
                return Ok(());
            }
            if self
                .child
                .as_mut()
                .ok_or("missing process")?
                .try_wait()?
                .is_some()
            {
                return Err("KMS exited at startup".into());
            }
            if deadline.remaining(clock.as_ref())?.is_zero() {
                return Err("KMS startup deadline".into());
            }
            std::thread::sleep(Duration::from_millis(20));
        }
    }

    fn stop(&mut self) {
        if let Some(mut child) = self.child.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }
    fn roots(&self) -> Result<RootCertStore> {
        let mut roots = RootCertStore::empty();
        roots.add(CertificateDer::from(fs::read(self.root.join("ca.der"))?))?;
        Ok(roots)
    }
    fn identity(
        &self,
        name: &str,
    ) -> Result<(Vec<CertificateDer<'static>>, PrivateKeyDer<'static>)> {
        Ok((
            vec![CertificateDer::from(fs::read(
                self.root.join(format!("{name}.der")),
            )?)],
            PrivateKeyDer::try_from(fs::read(self.root.join(format!("{name}-key.der")))?)?,
        ))
    }
    fn connection(&self) -> Result<StreamOwned<ClientConnection, TcpStream>> {
        let builder = ClientConfig::builder().with_root_certificates(self.roots()?);
        let (cert, key) = self.identity("client")?;
        let config = builder.with_client_auth_cert(cert, key)?;
        let tcp = TcpStream::connect(self.address)?;
        tcp.set_read_timeout(Some(Duration::from_secs(3)))?;
        tcp.set_write_timeout(Some(Duration::from_secs(3)))?;
        Ok(StreamOwned::new(
            ClientConnection::new(Arc::new(config), ServerName::try_from("localhost")?)?,
            tcp,
        ))
    }

    fn probe(&self) -> Result<()> {
        let mut tls = self.connection()?;
        let mut request = b"LXKP\0\x01\0".to_vec();
        blob(&mut request, PROVIDER)?;
        write_request(&mut tls, &request)?;
        let mut response = [0; 12];
        tls.read_exact(&mut response)?;
        assert_eq!(&response, b"\0\0\0\x08LXKP\0\x01\0\0");
        Ok(())
    }
}

fn blob(out: &mut Vec<u8>, value: &[u8]) -> Result<()> {
    out.extend(u32::try_from(value.len())?.to_be_bytes());
    out.extend(value);
    Ok(())
}

fn write_request(tls: &mut impl Write, request: &[u8]) -> Result<()> {
    tls.write_all(&u32::try_from(request.len())?.to_be_bytes())?;
    tls.write_all(request)?;
    tls.flush()?;
    Ok(())
}

#[test]
fn lxkp_authorize_recipient_is_an_unknown_operation() -> Result<()> {
    let host = Host::new()?;
    host.probe()?;
    let state = fs::read(host.root.join("state/state.aead"))?;
    for operation in [255, 13] {
        let mut request = b"LXKP".to_vec();
        request.extend(4_u16.to_be_bytes());
        request.push(operation);
        blob(&mut request, PROVIDER)?;
        request.extend([1; 32]);
        request.extend(125_u32.to_be_bytes());
        request.push(1);
        blob(&mut request, &[2; 32])?;
        blob(&mut request, b"retired-recipient-authorization")?;
        let mut tls = host.connection()?;
        write_request(&mut tls, &request)?;
        let mut response = [0; 1];
        match tls.read(&mut response) {
            Ok(0) => {}
            Err(error) if error.kind() == ErrorKind::UnexpectedEof => {}
            other => panic!(
                "unknown LXKP operation {operation} must close without a response: {other:?}"
            ),
        }
        assert_eq!(fs::read(host.root.join("state/state.aead"))?, state);
    }
    host.probe()?;
    Ok(())
}
