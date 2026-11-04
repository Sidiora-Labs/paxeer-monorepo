#![allow(dead_code)]
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::send::{encode_send_envelope, DisclosedEnvelope, EnvelopeOptions};
use layerx_crypto::signer::BindMessage;
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
pub fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(NoopWake));
    match future.as_mut().poll(&mut Context::from_waker(&waker)) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("synchronous signer unexpectedly pending"),
    }
}
pub fn binding() -> BindMessage {
    let bytes = super::paxeer_binding::Binding::new(125, [0xa1; 20], 7).message();
    BindMessage::decode(&bytes).expect("real retained bind producer")
}
pub fn old_envelope(public_key: [u8; 32]) -> DisclosedEnvelope {
    let (payload, _) = super::kernel_payloads::send(123);
    encode_send_envelope(
        &payload,
        &EnvelopeOptions {
            actor: "did:layerx:crypto-repair",
            public_key,
            protocol_version: super::PROTOCOL_VERSION,
            network_id: super::NETWORK_ID,
            identity_sequence: 7,
            idempotency_key: super::IDEMPOTENCY_KEY,
            fee_limit: 20,
            not_before: 10,
            not_after: 100,
        },
    )
    .expect("legacy producer for refusal coverage")
}
pub struct Directory(pub PathBuf);
impl Directory {
    pub fn new(label: &str) -> Self {
        let sequence = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("clock")
            .as_nanos();
        let directory = Self(
            std::env::temp_dir().join(format!("lx-{label}-{}-{sequence}", std::process::id())),
        );
        std::fs::create_dir(&directory.0).expect("fixture directory");
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
    pub fn create() -> Self {
        let directory = Directory::new("repair-mtls");
        let ca_key = PKey::from_rsa(Rsa::generate(2048).expect("CA RSA key")).expect("CA key");
        let ca = certificate(&ca_key, None, 1);
        std::fs::write(directory.0.join("ca.pem"), ca.to_pem().expect("CA PEM"))
            .expect("CA fixture");
        for (name, serial) in [("server", 2), ("client", 3)] {
            let key = PKey::from_rsa(Rsa::generate(2048).expect("peer RSA key")).expect("peer key");
            let peer = certificate(&key, Some((&ca, &ca_key)), serial);
            std::fs::write(
                directory.0.join(format!("{name}.pem")),
                peer.to_pem().expect("peer PEM"),
            )
            .expect("peer fixture");
            std::fs::write(
                directory.0.join(format!("{name}.key")),
                key.private_key_to_pem_pkcs8().expect("key PEM"),
            )
            .expect("key fixture");
        }
        Self { directory }
    }
    pub fn path(&self, name: &str) -> PathBuf {
        self.directory.0.join(name)
    }
}
fn certificate(key: &PKey<Private>, issuer: Option<(&X509, &PKey<Private>)>, serial: u32) -> X509 {
    let mut name = X509NameBuilder::new().expect("name builder");
    name.append_entry_by_text(
        "CN",
        if issuer.is_none() {
            "LayerX Repair CA"
        } else {
            "localhost"
        },
    )
    .expect("name");
    let name = name.build();
    let mut cert = X509::builder().expect("certificate builder");
    cert.set_version(2).expect("version");
    let serial_number = BigNum::from_u32(serial)
        .expect("serial")
        .to_asn1_integer()
        .expect("ASN1 serial");
    cert.set_serial_number(&serial_number)
        .expect("serial number");
    cert.set_subject_name(&name).expect("subject");
    cert.set_issuer_name(issuer.map_or(name.as_ref(), |(ca, _)| ca.subject_name()))
        .expect("issuer");
    cert.set_pubkey(key).expect("public key");
    cert.set_not_before(&Asn1Time::days_from_now(0).expect("start"))
        .expect("not before");
    cert.set_not_after(&Asn1Time::days_from_now(1).expect("end"))
        .expect("not after");
    if issuer.is_none() {
        cert.append_extension(
            BasicConstraints::new()
                .critical()
                .ca()
                .build()
                .expect("CA constraint"),
        )
        .expect("CA extension");
    } else {
        cert.append_extension(
            ExtendedKeyUsage::new()
                .server_auth()
                .client_auth()
                .build()
                .expect("usage"),
        )
        .expect("usage extension");
        let san = SubjectAlternativeName::new()
            .dns("localhost")
            .build(&cert.x509v3_context(issuer.map(|(ca, _)| ca.as_ref()), None))
            .expect("SAN");
        cert.append_extension(san).expect("SAN extension");
    }
    let identifier = SubjectKeyIdentifier::new()
        .build(&cert.x509v3_context(issuer.map(|(ca, _)| ca.as_ref()), None))
        .expect("subject key identifier");
    cert.append_extension(identifier)
        .expect("subject key extension");
    if let Some((ca, _)) = issuer {
        let authority = AuthorityKeyIdentifier::new()
            .keyid(true)
            .build(&cert.x509v3_context(Some(ca.as_ref()), None))
            .expect("authority key identifier");
        cert.append_extension(authority)
            .expect("authority key extension");
        cert.append_extension(
            KeyUsage::new()
                .digital_signature()
                .key_encipherment()
                .build()
                .expect("peer key usage"),
        )
        .expect("peer key usage extension");
    } else {
        cert.append_extension(
            KeyUsage::new()
                .critical()
                .key_cert_sign()
                .crl_sign()
                .build()
                .expect("CA key usage"),
        )
        .expect("CA key usage extension");
    }
    cert.sign(issuer.map_or(key, |(_, key)| key), MessageDigest::sha256())
        .expect("certificate signature");
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
    pub fn open(persisted: &[u8]) -> Self {
        use layerx_crypto::keystore::Keystore;
        use std::io::{Read, Write};
        use std::os::unix::net::UnixListener;
        use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
        use std::time::Duration;
        let store = Keystore::from_bytes(persisted).expect("reopen encrypted keystore");
        let key = store
            .open_with(b"fixture secret", b"repair-key", 17, |seed| {
                SigningKey::from_bytes(seed)
            })
            .expect("recover external key");
        let public_key = key.verifying_key().to_bytes();
        let directory = Directory::new("repair-agent");
        let listener = UnixListener::bind(directory.0.join("agent.sock")).expect("SSH peer socket");
        listener.set_nonblocking(true).expect("bounded accept");
        let stop = Arc::new(AtomicBool::new(false));
        let shutdown = Arc::clone(&stop);
        let attempts = Arc::new(AtomicUsize::new(0));
        let observed = Arc::clone(&attempts);
        let handle = std::thread::spawn(move || {
            fn string<'a>(body: &mut &'a [u8]) -> &'a [u8] {
                let size = u32::from_be_bytes(body[..4].try_into().expect("string size")) as usize;
                let value = &body[4..4 + size];
                *body = &body[4 + size..];
                value
            }
            fn push(out: &mut Vec<u8>, value: &[u8]) {
                out.extend_from_slice(&(value.len() as u32).to_be_bytes());
                out.extend_from_slice(value);
            }
            while !shutdown.load(Ordering::SeqCst) {
                let (mut stream, _) = match listener.accept() {
                    Ok(peer) => peer,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(5));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(2)))
                    .expect("read deadline");
                stream
                    .set_write_timeout(Some(Duration::from_secs(2)))
                    .expect("write deadline");
                observed.fetch_add(1, Ordering::SeqCst);
                let mut size = [0; 4];
                stream.read_exact(&mut size).expect("request size");
                let size = u32::from_be_bytes(size) as usize;
                assert!((1..=4096).contains(&size));
                let mut body = vec![0; size];
                stream.read_exact(&mut body).expect("request");
                assert_eq!(body[0], 13);
                let mut rest = &body[1..];
                let mut key_blob = string(&mut rest);
                assert_eq!(string(&mut key_blob), b"ssh-ed25519");
                let requested_key = string(&mut key_blob);
                assert!(key_blob.is_empty());
                let message = string(&mut rest);
                assert_eq!(rest, [0; 4]);
                let binding =
                    BindMessage::decode(message).expect("peer accepts retained bind only");
                let response = if requested_key == public_key {
                    let signature = key.sign(binding.as_bytes());
                    let mut blob = Vec::new();
                    push(&mut blob, b"ssh-ed25519");
                    push(&mut blob, &signature.to_bytes());
                    let mut out = vec![14];
                    push(&mut out, &blob);
                    out
                } else {
                    vec![5]
                };
                stream
                    .write_all(&(response.len() as u32).to_be_bytes())
                    .expect("response size");
                stream.write_all(&response).expect("response");
            }
        });
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
        self.handle
            .take()
            .expect("running peer")
            .join()
            .expect("SSH peer stopped");
    }
}
