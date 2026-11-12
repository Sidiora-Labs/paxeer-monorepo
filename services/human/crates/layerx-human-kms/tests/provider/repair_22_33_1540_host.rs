// In-process production KMS host with the committed 22.33.27 loopback peers.
use crate::{config, server, store, wire};
use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};
use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
use layerx_human_service::custody::{
    KeyClass, PrincipalKeyBinding, ProviderKeyReference, RemoteKmsProvider,
};
use layerx_human_service::frame::{Limits, MutualTlsConfig};
use layerx_types::clock::Clock;
use ring::signature::{Ed25519KeyPair, KeyPair};
use rustls::pki_types::{CertificateDer, PrivateKeyDer, ServerName};
use rustls::{
    ClientConfig, ClientConnection, RootCertStore, ServerConfig, ServerConnection, StreamOwned,
};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::fs;
use std::future::Future;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::task::{Context, Poll, Wake, Waker};
use std::thread::{self, JoinHandle};
use std::time::Duration;
use zeroize::Zeroizing;

pub type Result<T = ()> = std::result::Result<T, Box<dyn std::error::Error>>;
pub const PROVIDER: &str = "repair-22-33-1540";
pub static SERIAL: Mutex<()> = Mutex::new(());
const CA: &[u8] = include_bytes!("../fixtures/repair_22_33_27/ca.der");
const SERVER: &[u8] = include_bytes!("../fixtures/repair_22_33_27/server.der");
const SERVER_KEY: &[u8] = include_bytes!("../fixtures/repair_22_33_27/server.pk8");
const CLIENT: &[u8] = include_bytes!("../fixtures/repair_22_33_27/client.der");
const CLIENT_KEY: &[u8] = include_bytes!("../fixtures/repair_22_33_27/client.pk8");
const EXECUTOR: &[u8] = include_bytes!("../fixtures/repair_22_33_27/executor.der");
const EXECUTOR_KEY: &[u8] = include_bytes!("../fixtures/repair_22_33_27/executor.pk8");

pub fn checked<T, E: std::fmt::Debug>(value: std::result::Result<T, E>) -> Result<T> {
    value.map_err(|error| format!("{error:?}").into())
}
fn roots() -> Result<RootCertStore> {
    let mut roots = RootCertStore::empty();
    roots.add(CertificateDer::from(CA.to_vec()))?;
    Ok(roots)
}
fn tls_server() -> Result<Arc<ServerConfig>> {
    let verifier = rustls::server::WebPkiClientVerifier::builder(Arc::new(roots()?)).build()?;
    Ok(Arc::new(
        ServerConfig::builder()
            .with_client_cert_verifier(verifier)
            .with_single_cert(
                vec![CertificateDer::from(SERVER.to_vec())],
                PrivateKeyDer::try_from(SERVER_KEY.to_vec())?,
            )?,
    ))
}
fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    let mut encoded = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        write!(&mut encoded, "{byte:02x}").unwrap_or_else(|error| panic!("{error:?}"));
    }
    encoded
}
fn unhex(value: &str) -> Result<Vec<u8>> {
    let value = value.strip_prefix("0x").unwrap_or(value);
    if !value.len().is_multiple_of(2) {
        return Err("odd hex".into());
    }
    value
        .as_bytes()
        .chunks_exact(2)
        .map(|v| Ok(u8::from_str_radix(std::str::from_utf8(v)?, 16)?))
        .collect()
}
fn blob(out: &mut Vec<u8>, value: &[u8]) -> Result<()> {
    out.extend(u32::try_from(value.len())?.to_be_bytes());
    out.extend(value);
    Ok(())
}

struct Worker {
    stop: Arc<AtomicBool>,
    join: Option<JoinHandle<()>>,
    address: SocketAddr,
}
impl Worker {
    fn spawn(
        listener: TcpListener,
        action: impl Fn(TcpStream) + Send + Sync + 'static,
    ) -> Result<Self> {
        let address = listener.local_addr()?;
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let action = Arc::new(action);
        let join = thread::spawn(move || {
            for incoming in listener.incoming() {
                if stopped.load(Ordering::Acquire) {
                    break;
                }
                if let Ok(tcp) = incoming {
                    let action = action.clone();
                    // Each worker finishes its connection before the KMS store is reopened.
                    action(tcp);
                }
            }
        });
        Ok(Self {
            stop,
            join: Some(join),
            address,
        })
    }
}
impl Drop for Worker {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        if let Some(join) = self.join.take() {
            let _ = join.join();
        }
    }
}
struct ClockHost {
    stop: Arc<AtomicBool>,
    join: Option<JoinHandle<()>>,
    binding: ClockBinding,
}
impl ClockHost {
    fn new(root: &std::path::Path) -> Result<Self> {
        let mut server = crate::clock_server::Server::bind(root)?;
        let binding = ClockBinding {
            socket: server.socket().to_owned(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        };
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let join = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                server
                    .poll()
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "clock server"));
                thread::sleep(Duration::from_millis(1));
            }
        });
        Ok(Self {
            stop,
            join: Some(join),
            binding,
        })
    }
}
impl Drop for ClockHost {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(join) = self.join.take() {
            let _ = join.join();
        }
    }
}

pub struct Host {
    pub root: PathBuf,
    pub clock: Arc<dyn Clock>,
    pub bindings: Vec<PrincipalKeyBinding>,
    config: Arc<config::Config>,
    address: SocketAddr,
    kms: Option<Worker>,
    peers: Vec<Worker>,
    clock_service: ClockHost,
    previous_attestors: Option<std::ffi::OsString>,
}
impl Drop for Host {
    fn drop(&mut self) {
        drop(self.kms.take());
        match self.previous_attestors.take() {
            Some(value) => std::env::set_var("LAYERX_HUMAN_KMS_ATTESTOR_CONFIG_FILE", value),
            None => std::env::remove_var("LAYERX_HUMAN_KMS_ATTESTOR_CONFIG_FILE"),
        }
        self.peers.clear();
        let _ = fs::remove_dir_all(&self.root);
    }
}
impl Host {
    pub fn new() -> Result<Self> {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let mut unique = [0; 16];
        getrandom::fill(&mut unique)?;
        let root = std::env::temp_dir().join(format!("kms-repair-{}", hex(&unique)));
        fs::create_dir(&root)?;
        fs::set_permissions(&root, fs::Permissions::from_mode(0o700))?;
        let clock_service = ClockHost::new(&root)?;
        let clock: Arc<dyn Clock> = RuntimeClock::connect(clock_service.binding.clone())?;
        write_tls_material(&root)?;
        let mut bindings = Vec::new();
        let mut rows = Vec::new();
        let mut peers = Vec::new();
        for (index, principal) in ["alice", "bob"].into_iter().enumerate() {
            let identity = format!("layerx-human-custody/v1\0{principal}\0primary\0human-primary");
            let binding = PrincipalKeyBinding::new(
                identity.into_bytes(),
                125,
                KeyClass::HumanPrimary,
                PROVIDER,
            )?;
            let now = clock.sample(Duration::from_secs(1))?.unix_seconds();
            let owner_token = owner_assertion(principal, now)?;
            let fixture_secret = [u8::try_from(index + 21)?; 32];
            let fixture_call = crate::call(0);
            let raw = fixture_call.sign(&fixture_secret)?;
            let (_, signature) = decode_signed(&raw)?;
            let address = recover_sender(&fixture_call, &signature)?;
            let key_id = format!("fixture-wallet-{index}");
            let mut nodes = Vec::new();
            for number in 1..=3 {
                let node = format!("node-{number}");
                let listener = TcpListener::bind("127.0.0.1:0")?;
                let tls = tls_server()?;
                let sequence = Arc::new(AtomicU64::new(0));
                let node_id = node.clone();
                let wallet_key = key_id.clone();
                let assertion = owner_token.clone();
                let worker = Worker::spawn(listener, move |tcp| {
                    // The peer uses the real EIP-1559 signing producer, with fixture-owned
                    // material independent of every KMS primary seed and seal.
                    peer(
                        tcp,
                        &tls,
                        &node_id,
                        &wallet_key,
                        &fixture_secret,
                        &sequence,
                        &assertion,
                    )
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "signing fixture transport"));
                })?;
                nodes.push(json!({"id":node,"address":worker.address}));
                peers.push(worker);
            }
            let assertion = root.join(format!("owner-{index}.jwt"));
            fs::write(&assertion, &owner_token)?;
            fs::set_permissions(&assertion, fs::Permissions::from_mode(0o600))?;
            rows.push(json!({"binding":binding.digest(),"tenant":"tenant","principal":principal,"assertion_file":assertion,
                "config":{"nodes":nodes,"trust_anchors":[root.join("ca.der")],"client_certificate_chain":[root.join("client.der")],
                "client_private_key_pkcs8":root.join("client.pk8"),"key_id":"fixture-native","public_key":([3u8;32].to_vec()),
                "evm_key_id":key_id,"evm_address":address,"signers":["node-1","node-2","node-3"],"network_id":125,"timeout_seconds":5}}));
            bindings.push(binding);
        }
        let attestors = root.join("attestors.json");
        fs::write(&attestors, serde_json::to_vec(&json!({"bindings":rows}))?)?;
        fs::set_permissions(&attestors, fs::Permissions::from_mode(0o600))?;
        let previous_attestors = std::env::var_os("LAYERX_HUMAN_KMS_ATTESTOR_CONFIG_FILE");
        std::env::set_var("LAYERX_HUMAN_KMS_ATTESTOR_CONFIG_FILE", &attestors);
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        drop(listener);
        let mut seal = vec![0; 32];
        getrandom::fill(&mut seal)?;
        let config = Arc::new(config::Config {
            listen: address,
            state: root.join("state"),
            provider: PROVIDER.into(),
            chain: config::ChainSnapshot { chain_id: 125 },
            seal: Zeroizing::new(seal),
            tls: tls_server()?,
            client_pin: Sha256::digest(CLIENT).into(),
            attestor_evm_client_certificate: Some(CertificateDer::from(EXECUTOR.to_vec())),
            deadline: Duration::from_secs(3),
        });
        let mut host = Self {
            root,
            clock,
            bindings,
            config,
            address,
            kms: None,
            peers,
            clock_service,
            previous_attestors,
        };
        host.start()?;
        Ok(host)
    }
    pub fn start(&mut self) -> Result<()> {
        let store = Arc::new(Mutex::new(checked(store::Store::open(&self.config))?));
        let config = self.config.clone();
        let clock = self.clock.clone();
        let listener = TcpListener::bind(self.address)?;
        self.kms = Some(Worker::spawn(listener, move |tcp| {
            // Production connection authenticates mTLS, decodes and validates the
            // frame, dispatches the durable store and serializes the response.
            let _ = server::serve_connection(tcp, &config, &store, clock.as_ref());
        })?);
        Ok(())
    }
    pub fn stop(&mut self) -> Result<()> {
        self.kms.as_ref().ok_or("KMS worker is not running")?;
        drop(self.kms.take());
        Ok(())
    }
    pub fn restart(&mut self) -> Result<()> {
        self.stop()?;
        self.start()
    }
    pub fn remote(&self, executor: bool) -> Result<RemoteKmsProvider> {
        let (cert, key) = if executor {
            (EXECUTOR, EXECUTOR_KEY)
        } else {
            (CLIENT, CLIENT_KEY)
        };
        let tls = checked(MutualTlsConfig::new(
            roots()?,
            vec![CertificateDer::from(cert.to_vec())],
            PrivateKeyDer::try_from(key.to_vec())?,
        ))?;
        Ok(RemoteKmsProvider::new(
            PROVIDER,
            self.address,
            "127.0.0.1",
            tls,
            Limits {
                maximum_frame_bytes: wire::MAX_FRAME,
                maximum_connections: 4,
                maximum_streams: 1,
                maximum_queued_bytes: wire::MAX_FRAME,
                deadline: Duration::from_secs(5),
            },
            self.clock.clone(),
        )?)
    }
    pub fn connection(&self, executor: bool) -> Result<StreamOwned<ClientConnection, TcpStream>> {
        let (cert, key) = if executor {
            (EXECUTOR, EXECUTOR_KEY)
        } else {
            (CLIENT, CLIENT_KEY)
        };
        let config = ClientConfig::builder()
            .with_root_certificates(roots()?)
            .with_client_auth_cert(
                vec![CertificateDer::from(cert.to_vec())],
                PrivateKeyDer::try_from(key.to_vec())?,
            )?;
        let tcp = TcpStream::connect(self.address)?;
        tcp.set_read_timeout(Some(Duration::from_secs(5)))?;
        tcp.set_write_timeout(Some(Duration::from_secs(5)))?;
        Ok(StreamOwned::new(
            ClientConnection::new(Arc::new(config), ServerName::try_from("127.0.0.1")?)?,
            tcp,
        ))
    }
    pub fn exchange(&self, frame: &[u8], executor: bool) -> Result<Vec<u8>> {
        let mut tls = self.connection(executor)?;
        checked(layerx_human_service::frame::write_frame(
            &mut tls,
            frame,
            wire::MAX_FRAME,
        ))?;
        checked(layerx_human_service::frame::read_frame(
            &mut tls,
            wire::MAX_FRAME,
        ))
    }
    pub fn sign_frame(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        from: [u8; 20],
        canonical: &[u8],
        disclosure: &[u8],
    ) -> Result<Vec<u8>> {
        use sha3::Keccak256;
        let mut bytes = self
            .remote(false)?
            .encode_evm_request(6, binding, reference, &[])?;
        bytes[4..6].copy_from_slice(&7_u16.to_be_bytes());
        bytes[6] = 5;
        bytes.extend(Keccak256::digest(canonical));
        bytes.extend(from);
        blob(&mut bytes, canonical)?;
        blob(&mut bytes, disclosure)?;
        Ok(bytes)
    }
}

fn owner_assertion(principal: &str, now: u64) -> Result<String> {
    let key = Ed25519KeyPair::from_seed_unchecked(&[0x44; 32])
        .map_err(|_| "fixture owner assertion key")?;
    let header = URL_SAFE_NO_PAD.encode(br#"{"alg":"EdDSA","typ":"JWT"}"#);
    let claims = URL_SAFE_NO_PAD.encode(serde_json::to_vec(&json!({
        "sub": format!("fixture-owner-{principal}"), "iat": now, "exp": now + 600,
    }))?);
    let message = format!("{header}.{claims}");
    let signature = key.sign(message.as_bytes());
    ring::signature::UnparsedPublicKey::new(&ring::signature::ED25519, key.public_key().as_ref())
        .verify(message.as_bytes(), signature.as_ref())
        .map_err(|_| "fixture owner assertion signature")?;
    let assertion = format!("{message}.{}", URL_SAFE_NO_PAD.encode(signature.as_ref()));
    let claims = crate::attestor::assertion_claims(&assertion)?;
    assert_eq!(claims.subject, format!("fixture-owner-{principal}"));
    claims.check(now)?;
    Ok(assertion)
}

fn peer(
    tcp: TcpStream,
    config: &Arc<ServerConfig>,
    node: &str,
    key_id: &str,
    secret: &[u8; 32],
    sequence: &AtomicU64,
    assertion: &str,
) -> Result<()> {
    tcp.set_read_timeout(Some(Duration::from_secs(5)))?;
    tcp.set_write_timeout(Some(Duration::from_secs(5)))?;
    let mut tls = StreamOwned::new(ServerConnection::new(config.clone())?, tcp);
    let mut header = Vec::new();
    let mut byte = [0];
    while !header.ends_with(b"\r\n\r\n") {
        tls.read_exact(&mut byte)?;
        header.extend(byte);
        if header.len() > 65536 {
            return Err("header bound".into());
        }
    }
    let header = std::str::from_utf8(&header)?;
    assert!(header.starts_with("POST /v1/sign HTTP/1.1\r\n"));
    let authorization = header.lines().find_map(|line| {
        let (name, value) = line.split_once(':')?;
        name.eq_ignore_ascii_case("authorization")
            .then_some(value.trim())
    });
    assert_eq!(
        authorization,
        Some(format!("Bearer {assertion}").as_str()),
        "each signing peer requires the exact signed owner assertion"
    );
    let length = header
        .lines()
        .find_map(|line| {
            line.to_ascii_lowercase()
                .strip_prefix("content-length:")
                .map(str::trim)
                .map(str::to_owned)
        })
        .ok_or("body length missing")?
        .parse::<usize>()?;
    if length > wire::MAX_FRAME {
        return Err("body bound".into());
    }
    let mut body = vec![0; length];
    tls.read_exact(&mut body)?;
    let request: Value = serde_json::from_slice(&body)?;
    assert_eq!(request["key_id"], key_id);
    assert_eq!(request["kind"], "evm_tx");
    assert_eq!(request["signers"], json!(["node-1", "node-2", "node-3"]));
    let transaction = unhex(
        request["transaction"]
            .as_str()
            .ok_or("transaction missing")?,
    )?;
    // The real attestor client sends a type-2 envelope with three empty signature fields.
    let unsigned = strip_signature(&transaction)?;
    let call = crate::attestor::decode_unsigned_evm_transaction(&unsigned)?;
    let raw = call.sign(secret)?;
    let (_, signature) = decode_signed(&raw)?;
    let compact = signature.as_bytes();
    let mut compact = *compact;
    compact[64] = match compact[64] {
        27 | 28 => compact[64] - 27,
        value => value,
    };
    let response = json!({"node_id":node,"key_id":key_id,"kind":"evm_tx","signed_bytes":hex(&call.signing_hash()),"signature":hex(&compact),"recovery_id":compact[64],"audit_sequence":sequence.fetch_add(1, Ordering::AcqRel)+1}).to_string();
    write!(tls, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", response.len(), response)?;
    tls.flush()?;
    Ok(())
}
fn strip_signature(transaction: &[u8]) -> Result<Vec<u8>> {
    let prefix = *transaction.get(1).ok_or("RLP missing")?;
    let start = if prefix <= 0xf7 {
        2
    } else {
        2 + usize::from(prefix - 0xf7)
    };
    if !transaction.ends_with(&[0x80, 0x80, 0x80]) {
        return Err("expected empty signatures".into());
    }
    let payload = &transaction[start..transaction.len() - 3];
    let mut out = vec![2];
    if payload.len() < 56 {
        out.push(0xc0 + u8::try_from(payload.len())?);
    } else {
        let length = payload.len().to_be_bytes();
        let first = length
            .iter()
            .position(|byte| *byte != 0)
            .ok_or("empty RLP length")?;
        let length = &length[first..];
        out.push(0xf7 + u8::try_from(length.len())?);
        out.extend(length);
    }
    out.extend(payload);
    Ok(out)
}
struct ThreadWake(thread::Thread);
impl Wake for ThreadWake {
    fn wake(self: Arc<Self>) {
        self.0.unpark();
    }
}
pub fn run<F: Future>(future: F) -> F::Output {
    let waker = Waker::from(Arc::new(ThreadWake(thread::current())));
    let mut context = Context::from_waker(&waker);
    let mut future = std::pin::pin!(future);
    loop {
        match future.as_mut().poll(&mut context) {
            Poll::Ready(value) => return value,
            Poll::Pending => thread::park(),
        }
    }
}

fn write_tls_material(root: &Path) -> Result<()> {
    for (name, bytes) in [
        ("ca.der", CA),
        ("client.der", CLIENT),
        ("client.pk8", CLIENT_KEY),
    ] {
        let path = root.join(name);
        fs::write(&path, bytes)?;
        fs::set_permissions(path, fs::Permissions::from_mode(0o600))?;
    }
    Ok(())
}
