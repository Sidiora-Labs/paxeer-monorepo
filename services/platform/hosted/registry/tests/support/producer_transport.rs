use std::fs;
use std::net::{TcpListener, TcpStream};
use std::path::Path;
use std::process::Command;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_platform_internal::tls::{Origin, Upstream};
use native_tls::{Certificate, Identity};
use rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::{RootCertStore, ServerConfig, ServerConnection, StreamOwned};
use zeroize::Zeroizing;

pub struct Tls {
    pub config: Arc<ServerConfig>,
    pub ca: Certificate,
    pub identity: Identity,
}

impl Tls {
    pub fn new(root: &Path) -> Self {
        let _ = rustls::crypto::ring::default_provider().install_default();
        fs::create_dir_all(root).unwrap_or_else(|error| panic!("{error}"));
        issue_certificates(root);
        let der = fs::read(root.join("cert.der")).unwrap_or_else(|error| panic!("{error}"));
        let ca = Certificate::from_der(&der).unwrap_or_else(|error| panic!("{error}"));
        let config = Self::server_config(root, der);
        let identity = Identity::from_pkcs12(
            &fs::read(root.join("client.p12")).unwrap_or_else(|error| panic!("{error}")),
            "integration-only",
        )
        .unwrap_or_else(|error| panic!("{error}"));
        Self {
            config,
            ca,
            identity,
        }
    }

    fn server_config(root: &Path, der: Vec<u8>) -> Arc<ServerConfig> {
        let mut roots = RootCertStore::empty();
        roots
            .add(CertificateDer::from(der.clone()))
            .unwrap_or_else(|error| panic!("{error}"));
        let verifier = rustls::server::WebPkiClientVerifier::builder(Arc::new(roots))
            .allow_unauthenticated()
            .build()
            .unwrap_or_else(|error| panic!("{error}"));
        let config = ServerConfig::builder()
            .with_client_cert_verifier(verifier)
            .with_single_cert(
                vec![CertificateDer::from(der)],
                PrivateKeyDer::from(PrivatePkcs8KeyDer::from(
                    fs::read(root.join("key.der")).unwrap_or_else(|error| panic!("{error}")),
                )),
            )
            .unwrap_or_else(|error| panic!("{error}"));
        Arc::new(config)
    }

    pub fn upstream(&self, port: u16, token: &str) -> Upstream {
        Upstream::new(
            Origin::parse(&format!("https://localhost:{port}"))
                .unwrap_or_else(|error| panic!("{error}")),
            self.ca.clone(),
            Some(self.identity.clone()),
            Some(Zeroizing::new(token.to_owned())),
        )
    }
}

fn issue_certificates(root: &Path) {
    for args in [
        vec![
            "req",
            "-x509",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-keyout",
            "key.pem",
            "-out",
            "cert.pem",
            "-days",
            "2",
            "-subj",
            "/CN=localhost",
            "-addext",
            "subjectAltName=DNS:localhost",
            "-addext",
            "basicConstraints=critical,CA:TRUE",
        ],
        vec![
            "x509", "-in", "cert.pem", "-outform", "DER", "-out", "cert.der",
        ],
        vec![
            "pkcs8", "-topk8", "-nocrypt", "-in", "key.pem", "-outform", "DER", "-out", "key.der",
        ],
        vec![
            "req",
            "-new",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-keyout",
            "client.key",
            "-out",
            "client.csr",
            "-subj",
            "/CN=producer",
        ],
    ] {
        run(root, &args);
    }
    fs::write(
        root.join("client.ext"),
        "basicConstraints=critical,CA:FALSE\nextendedKeyUsage=clientAuth\nsubjectAltName=URI:urn:layerx:webhooks:role:producer\n",
    )
    .unwrap_or_else(|error| panic!("{error}"));
    run(
        root,
        &[
            "x509",
            "-req",
            "-in",
            "client.csr",
            "-CA",
            "cert.pem",
            "-CAkey",
            "key.pem",
            "-CAcreateserial",
            "-out",
            "client.pem",
            "-days",
            "2",
            "-extfile",
            "client.ext",
        ],
    );
    run(
        root,
        &[
            "pkcs12",
            "-export",
            "-inkey",
            "client.key",
            "-in",
            "client.pem",
            "-out",
            "client.p12",
            "-passout",
            "pass:integration-only",
        ],
    );
}

fn run(root: &Path, args: &[&str]) {
    let output = Command::new("openssl")
        .current_dir(root)
        .args(args)
        .output()
        .unwrap_or_else(|error| panic!("{error}"));
    assert!(output.status.success(), "certificate generation failed");
}

pub struct Listener {
    pub port: u16,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl Listener {
    pub fn start(
        tls: Arc<ServerConfig>,
        port: u16,
        handler: impl Fn(&mut StreamOwned<ServerConnection, TcpStream>) + Send + 'static,
    ) -> Self {
        let listener =
            TcpListener::bind(("127.0.0.1", port)).unwrap_or_else(|error| panic!("{error}"));
        let port = listener
            .local_addr()
            .unwrap_or_else(|error| panic!("{error}"))
            .port();
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("{error}"));
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((tcp, _)) => {
                        tcp.set_read_timeout(Some(Duration::from_secs(3)))
                            .unwrap_or_else(|error| panic!("{error}"));
                        tcp.set_write_timeout(Some(Duration::from_secs(3)))
                            .unwrap_or_else(|error| panic!("{error}"));
                        let conn = ServerConnection::new(Arc::clone(&tls))
                            .unwrap_or_else(|error| panic!("{error}"));
                        handler(&mut StreamOwned::new(conn, tcp));
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(5));
                    }
                    Err(error) => panic!("{error}"),
                }
            }
        });
        Self {
            port,
            stop,
            worker: Some(worker),
        }
    }
}

impl Drop for Listener {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            assert!(worker.join().is_ok());
        }
    }
}
