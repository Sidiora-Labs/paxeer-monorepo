#![allow(dead_code)]
use super::assertions;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::disclosure::Disclosure;
use layerx_crypto::signer::BindMessage;
use layerx_crypto::signer::SigningRequest;
use layerx_crypto::SignatureMessage;
use layerx_wire::hash::Domain;
use openssl::asn1::Asn1Time;
use openssl::bn::BigNum;
use openssl::hash::MessageDigest;
use openssl::pkey::{PKey, Private};
use openssl::rsa::Rsa;
use openssl::x509::extension::{
    AuthorityKeyIdentifier, BasicConstraints, ExtendedKeyUsage, KeyUsage, SubjectAlternativeName,
    SubjectKeyIdentifier,
};
use openssl::x509::{X509NameBuilder, X509};
use std::future::Future;
use std::path::{Path, PathBuf};
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};
use std::time::{SystemTime, UNIX_EPOCH};

struct NoopWake;
impl Wake for NoopWake {
    fn wake(self: Arc<Self>) {}
}
/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(NoopWake));
    match future.as_mut().poll(&mut Context::from_waker(&waker)) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("synchronous signer unexpectedly pending"),
    }
}
/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn binding() -> BindMessage {
    let bytes = super::paxeer_binding::Binding::new(125, [0xa1; 20], 7).message();
    assertions::require(BindMessage::decode(&bytes), "real retained bind producer")
}
/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn activity_request<'a>(canonical: &'a [u8], disclosure: &'a Disclosure) -> SigningRequest<'a> {
    let message = assertions::require(
        SignatureMessage::new(
            Domain::SignaturePreimage,
            super::PROTOCOL_VERSION,
            super::NETWORK_ID,
            canonical,
        ),
        "activity signature preimage domain",
    );
    assertions::require(
        SigningRequest::new(message, disclosure),
        "validated call disclosure",
    )
}
pub struct Directory(pub PathBuf);
impl Directory {
    /// # Panics
    /// Panics if the retained protocol fixture, local peer or test environment is invalid.
    pub fn new(label: &str) -> Self {
        let sequence =
            assertions::require(SystemTime::now().duration_since(UNIX_EPOCH), "clock").as_nanos();
        let directory = Self(
            std::env::temp_dir().join(format!("lx-{label}-{}-{sequence}", std::process::id())),
        );
        assertions::require(std::fs::create_dir(&directory.0), "fixture directory");
        directory
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

pub struct Certificates {
    directory: Directory,
}
impl Certificates {
    /// # Panics
    /// Panics if the retained protocol fixture, local peer or test environment is invalid.
    pub fn create() -> Self {
        let directory = Directory::new("repair-mtls");
        let ca_key = assertions::require(
            PKey::from_rsa(assertions::require(Rsa::generate(2048), "CA RSA key")),
            "CA key",
        );
        let ca = certificate(&ca_key, None, 1);
        assertions::require(
            std::fs::write(
                directory.0.join("ca.pem"),
                assertions::require(ca.to_pem(), "CA PEM"),
            ),
            "CA fixture",
        );
        for (name, serial) in [("server", 2), ("client", 3)] {
            let key = assertions::require(
                PKey::from_rsa(assertions::require(Rsa::generate(2048), "peer RSA key")),
                "peer key",
            );
            let peer = certificate(&key, Some((&ca, &ca_key)), serial);
            assertions::require(
                std::fs::write(
                    directory.0.join(format!("{name}.pem")),
                    assertions::require(peer.to_pem(), "peer PEM"),
                ),
                "peer fixture",
            );
            assertions::require(
                std::fs::write(
                    directory.0.join(format!("{name}.key")),
                    assertions::require(key.private_key_to_pem_pkcs8(), "key PEM"),
                ),
                "key fixture",
            );
        }
        Self { directory }
    }
    pub fn path(&self, name: &str) -> PathBuf {
        self.directory.0.join(name)
    }
}
fn certificate(key: &PKey<Private>, issuer: Option<(&X509, &PKey<Private>)>, serial: u32) -> X509 {
    let mut name = assertions::require(X509NameBuilder::new(), "name builder");
    assertions::require(
        name.append_entry_by_text(
            "CN",
            if issuer.is_none() {
                "LayerX Repair CA"
            } else {
                "localhost"
            },
        ),
        "name",
    );
    let name = name.build();
    let mut cert = assertions::require(X509::builder(), "certificate builder");
    assertions::require(cert.set_version(2), "version");
    let serial_number = assertions::require(
        assertions::require(BigNum::from_u32(serial), "serial").to_asn1_integer(),
        "ASN1 serial",
    );
    assertions::require(cert.set_serial_number(&serial_number), "serial number");
    assertions::require(cert.set_subject_name(&name), "subject");
    assertions::require(
        cert.set_issuer_name(issuer.map_or(name.as_ref(), |(ca, _)| ca.subject_name())),
        "issuer",
    );
    assertions::require(cert.set_pubkey(key), "public key");
    assertions::require(
        {
            let start = assertions::require(Asn1Time::days_from_now(0), "start");
            cert.set_not_before(&start)
        },
        "not before",
    );
    assertions::require(
        {
            let end = assertions::require(Asn1Time::days_from_now(1), "end");
            cert.set_not_after(&end)
        },
        "not after",
    );
    if issuer.is_none() {
        assertions::require(
            cert.append_extension(assertions::require(
                BasicConstraints::new().critical().ca().build(),
                "CA constraint",
            )),
            "CA extension",
        );
    } else {
        assertions::require(
            cert.append_extension(assertions::require(
                ExtendedKeyUsage::new().server_auth().client_auth().build(),
                "usage",
            )),
            "usage extension",
        );
        let san = assertions::require(
            SubjectAlternativeName::new()
                .dns("localhost")
                .build(&cert.x509v3_context(issuer.map(|(ca, _)| ca.as_ref()), None)),
            "SAN",
        );
        assertions::require(cert.append_extension(san), "SAN extension");
    }
    let identifier = assertions::require(
        SubjectKeyIdentifier::new()
            .build(&cert.x509v3_context(issuer.map(|(ca, _)| ca.as_ref()), None)),
        "subject key identifier",
    );
    assertions::require(cert.append_extension(identifier), "subject key extension");
    certificate_key_usage(&mut cert, issuer);
    assertions::require(
        cert.sign(issuer.map_or(key, |(_, key)| key), MessageDigest::sha256()),
        "certificate signature",
    );
    cert.build()
}

#[cfg(unix)]
pub struct AgentPeer {
    directory: Directory,
    stop: Arc<std::sync::atomic::AtomicBool>,
    handle: Option<std::thread::JoinHandle<()>>,
    pub attempts: Arc<std::sync::atomic::AtomicUsize>,
    pub public_key: [u8; 32],
}
#[cfg(unix)]
impl AgentPeer {
    /// # Panics
    /// Panics if the retained protocol fixture, local peer or test environment is invalid.
    pub fn open(persisted: &[u8]) -> Self {
        use layerx_crypto::keystore::Keystore;
        use std::os::unix::net::UnixListener;
        use std::sync::atomic::{AtomicBool, AtomicUsize};
        let store =
            assertions::require(Keystore::from_bytes(persisted), "reopen encrypted keystore");
        let key = assertions::require(
            store.open_with(b"fixture secret", b"repair-key", 17, |seed| {
                SigningKey::from_bytes(seed)
            }),
            "recover external key",
        );
        let public_key = key.verifying_key().to_bytes();
        let directory = Directory::new("repair-agent");
        let listener = assertions::require(
            UnixListener::bind(directory.0.join("agent.sock")),
            "SSH peer socket",
        );
        assertions::require(listener.set_nonblocking(true), "bounded accept");
        let stop = Arc::new(AtomicBool::new(false));
        let shutdown = Arc::clone(&stop);
        let attempts = Arc::new(AtomicUsize::new(0));
        let observed = Arc::clone(&attempts);
        let handle =
            std::thread::spawn(move || serve_agent_peer(&listener, &key, &shutdown, &observed));
        Self {
            directory,
            stop,
            handle: Some(handle),
            attempts,
            public_key,
        }
    }
    pub fn socket(&self) -> &Path {
        self.directory.0.as_path()
    }
    pub fn socket_path(&self) -> PathBuf {
        self.directory.0.join("agent.sock")
    }
}
#[cfg(unix)]
impl Drop for AgentPeer {
    fn drop(&mut self) {
        self.stop.store(true, std::sync::atomic::Ordering::SeqCst);
        assertions::require(
            assertions::require(self.handle.take(), "running peer").join(),
            "SSH peer stopped",
        );
    }
}

fn certificate_key_usage(
    cert: &mut openssl::x509::X509Builder,
    issuer: Option<(&X509, &PKey<Private>)>,
) {
    if let Some((ca, _)) = issuer {
        let authority = assertions::require(
            AuthorityKeyIdentifier::new()
                .keyid(true)
                .build(&cert.x509v3_context(Some(ca.as_ref()), None)),
            "authority key identifier",
        );
        assertions::require(cert.append_extension(authority), "authority key extension");
        assertions::require(
            cert.append_extension(assertions::require(
                KeyUsage::new()
                    .digital_signature()
                    .key_encipherment()
                    .build(),
                "peer key usage",
            )),
            "peer key usage extension",
        );
    } else {
        assertions::require(
            cert.append_extension(assertions::require(
                KeyUsage::new()
                    .critical()
                    .key_cert_sign()
                    .crl_sign()
                    .build(),
                "CA key usage",
            )),
            "CA key usage extension",
        );
    }
}

#[cfg(unix)]
fn serve_agent_peer(
    listener: &std::os::unix::net::UnixListener,
    key: &SigningKey,
    shutdown: &std::sync::atomic::AtomicBool,
    observed: &std::sync::atomic::AtomicUsize,
) {
    use std::io::{Read, Write};
    use std::sync::atomic::Ordering;
    use std::time::Duration;

    let public_key = key.verifying_key().to_bytes();
    while !shutdown.load(Ordering::SeqCst) {
        let (mut stream, _) = match listener.accept() {
            Ok(peer) => peer,
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                std::thread::sleep(Duration::from_millis(5));
                continue;
            }
            Err(error) => panic!("accept: {error}"),
        };
        assertions::require(
            stream.set_read_timeout(Some(Duration::from_secs(2))),
            "read deadline",
        );
        assertions::require(
            stream.set_write_timeout(Some(Duration::from_secs(2))),
            "write deadline",
        );
        observed.fetch_add(1, Ordering::SeqCst);
        let mut size = [0; 4];
        assertions::require(stream.read_exact(&mut size), "request size");
        let size = {
            let Ok(length) = usize::try_from(u32::from_be_bytes(size)) else {
                panic!("SSH size exceeds usize");
            };
            length
        };
        assert!((1..=4096).contains(&size));
        let mut body = vec![0; size];
        assertions::require(stream.read_exact(&mut body), "request");
        assert_eq!(body[0], 13);
        let mut rest = &body[1..];
        let mut key_blob = agent_string(&mut rest);
        assert_eq!(agent_string(&mut key_blob), b"ssh-ed25519");
        let requested_key = agent_string(&mut key_blob);
        assert!(key_blob.is_empty());
        let message = agent_string(&mut rest);
        assert_eq!(rest, [0; 4]);
        let binding = assertions::require(
            BindMessage::decode(message),
            "peer accepts retained bind only",
        );
        let response = if requested_key == public_key {
            let signature = key.sign(binding.as_bytes());
            let mut blob = Vec::new();
            push_agent_string(&mut blob, b"ssh-ed25519");
            push_agent_string(&mut blob, &signature.to_bytes());
            let mut out = vec![14];
            push_agent_string(&mut out, &blob);
            out
        } else {
            vec![5]
        };
        assertions::require(
            stream.write_all(
                &({
                    let Ok(length) = u32::try_from(response.len()) else {
                        panic!("SSH response exceeds u32");
                    };
                    length
                })
                .to_be_bytes(),
            ),
            "response size",
        );
        assertions::require(stream.write_all(&response), "response");
    }
}

#[cfg(unix)]
fn agent_string<'a>(body: &mut &'a [u8]) -> &'a [u8] {
    let size = {
        let Ok(length) = usize::try_from(u32::from_be_bytes(assertions::require(
            body[..4].try_into(),
            "string size",
        ))) else {
            panic!("SSH size exceeds usize");
        };
        length
    };
    let value = &body[4..4 + size];
    *body = &body[4 + size..];
    value
}

#[cfg(unix)]
fn push_agent_string(out: &mut Vec<u8>, value: &[u8]) {
    out.extend_from_slice(
        &({
            let Ok(length) = u32::try_from(value.len()) else {
                panic!("SSH string exceeds u32");
            };
            length
        })
        .to_be_bytes(),
    );
    out.extend_from_slice(value);
}
