//! Attestor signing preserves audit order, deadlines and durable replay bindings.

#[allow(dead_code)]
mod attestor {
    pub(super) fn decode_peer_evm_wire(raw: &[u8]) -> Result<Eip1559Call, AttestorError> {
        let invalid = || AttestorError::Configuration("fixture attestor EIP-1559 envelope");
        if raw.first() != Some(&2) {
            return Err(invalid());
        }
        let (fields, length, list) = evm_rlp_item(&raw[1..]).ok_or_else(invalid)?;
        if !list || length != raw.len() - 1 || !fields.ends_with(&[0x80, 0x80, 0x80]) {
            return Err(invalid());
        }
        let mut unsigned = vec![2];
        unsigned.extend(evm_rlp_list(&fields[..fields.len() - 3]));
        let call = decode_unsigned_evm_transaction(&unsigned)?;
        let (digest, encoded) = evm_transaction(&unsigned, call.chain_id)?;
        if encoded != raw || digest != call.signing_hash() {
            return Err(invalid());
        }
        Ok(call)
    }
    include!(concat!(env!("CARGO_MANIFEST_DIR"), "/src/attestor.rs"));

    // The attestor consumes twelve fields, including empty parity/r/s; the
    // signing preimage contains nine. Require the exact production wire form.
}
#[allow(dead_code)]
#[path = "../src/config.rs"]
mod config;
#[allow(dead_code)]
#[path = "../src/evm.rs"]
mod evm;
#[allow(dead_code)]
#[path = "../src/evm_types.rs"]
mod evm_types;
#[allow(dead_code)]
#[path = "../src/export.rs"]
mod export;
#[allow(dead_code)]
#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod runtime_clock_server;
#[allow(dead_code)]
#[path = "../src/send.rs"]
mod send;
#[allow(dead_code)]
#[path = "../src/server.rs"]
mod server;
#[allow(dead_code)]
#[path = "../src/store.rs"]
mod store;
#[allow(dead_code)]
#[path = "../src/wire.rs"]
mod wire;

use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};
use layerx_client::frame::framing::{read_frame, write_frame};
use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
use layerx_human_service::custody::{
    KeyClass, KmsProvider, PrincipalKeyBinding, ProviderKeyDescription, RemoteKmsProvider,
};
use layerx_human_service::frame::{Limits, MutualTlsConfig};
use layerx_types::clock::{Clock, ClockReading, Deadline};
use ring::signature::{Ed25519KeyPair, KeyPair};
use rustls::pki_types::{CertificateDer, PrivateKeyDer, ServerName};
use rustls::{
    ClientConfig, ClientConnection, RootCertStore, ServerConfig, ServerConnection, StreamOwned,
};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use sha3::Keccak256;
use std::collections::BTreeMap;
use std::error::Error;
use std::fs;
use std::future::Future;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Condvar, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;
use zeroize::Zeroizing;

type Result<T = ()> = std::result::Result<T, Box<dyn Error>>;
const CA: &[u8] = include_bytes!("../src/fixtures/repair_22_33_25/ca.der");
const CERT: &[u8] = include_bytes!("../src/fixtures/repair_22_33_25/peer.der");
// Public test-only TLS key; it has no production authority.
const TLS_KEY: &[u8] = include_bytes!("../src/fixtures/repair_22_33_25/peer.pk8");
const SIGNERS: [&str; 3] = ["node-1", "node-2", "node-3"];
const PROVIDER: &str = "repair-22-33-25";
const MAX_FRAME: usize = 2_097_152;

fn checked<T, E: std::fmt::Debug>(result: std::result::Result<T, E>) -> Result<T> {
    result.map_err(|error| format!("{error:?}").into())
}
fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    let mut encoded = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        write!(&mut encoded, "{byte:02x}").unwrap_or_else(|error| panic!("{error:?}"));
    }
    encoded
}
fn unhex(value: &str) -> Vec<u8> {
    value
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(
                std::str::from_utf8(pair).unwrap_or_else(|error| panic!("{error:?}")),
                16,
            )
            .unwrap_or_else(|error| panic!("{error:?}"))
        })
        .collect()
}
struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let mut nonce = [0; 8];
        getrandom::fill(&mut nonce).unwrap_or_else(|error| panic!("{error:?}"));
        let path =
            std::env::temp_dir().join(format!("kms-repair-{}-{}", std::process::id(), hex(&nonce)));
        fs::create_dir(&path).unwrap_or_else(|error| panic!("{error:?}"));
        fs::set_permissions(&path, fs::Permissions::from_mode(0o700))
            .unwrap_or_else(|error| panic!("{error:?}"));
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}
fn private(path: &Path, bytes: &[u8]) {
    fs::write(path, bytes).unwrap_or_else(|error| panic!("{error:?}"));
    fs::set_permissions(path, fs::Permissions::from_mode(0o600))
        .unwrap_or_else(|error| panic!("{error:?}"));
}
fn roots() -> RootCertStore {
    let mut roots = RootCertStore::empty();
    roots
        .add(CertificateDer::from(CA.to_vec()))
        .unwrap_or_else(|error| panic!("{error:?}"));
    roots
}
fn server_tls() -> Arc<ServerConfig> {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let verifier = rustls::server::WebPkiClientVerifier::builder(Arc::new(roots()))
        .build()
        .unwrap_or_else(|error| panic!("{error:?}"));
    Arc::new(
        ServerConfig::builder()
            .with_client_cert_verifier(verifier)
            .with_single_cert(
                vec![CertificateDer::from(CERT.to_vec())],
                PrivateKeyDer::try_from(TLS_KEY.to_vec())
                    .unwrap_or_else(|error| panic!("{error:?}")),
            )
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
}
fn client_tls() -> Arc<ClientConfig> {
    let _ = rustls::crypto::ring::default_provider().install_default();
    Arc::new(
        ClientConfig::builder()
            .with_root_certificates(roots())
            .with_client_auth_cert(
                vec![CertificateDer::from(CERT.to_vec())],
                PrivateKeyDer::try_from(TLS_KEY.to_vec())
                    .unwrap_or_else(|error| panic!("{error:?}")),
            )
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
}
fn tls(address: SocketAddr) -> StreamOwned<ClientConnection, TcpStream> {
    let tcp = TcpStream::connect(address).unwrap_or_else(|error| panic!("{error:?}"));
    tcp.set_read_timeout(Some(Duration::from_secs(10)))
        .unwrap_or_else(|error| panic!("{error:?}"));
    tcp.set_write_timeout(Some(Duration::from_secs(10)))
        .unwrap_or_else(|error| panic!("{error:?}"));
    StreamOwned::new(
        ClientConnection::new(client_tls(), ServerName::IpAddress(address.ip().into()))
            .unwrap_or_else(|error| panic!("{error:?}")),
        tcp,
    )
}
struct ClockSource {
    source: Arc<RuntimeClock>,
    observations: Mutex<()>,
}
impl Clock for ClockSource {
    fn sample(
        &self,
        budget: Duration,
    ) -> std::result::Result<layerx_types::clock::ClockReading, layerx_types::clock::ClockError>
    {
        let _observation = self
            .observations
            .lock()
            .map_err(|_| layerx_types::clock::ClockError::Unavailable)?;
        self.source.sample(budget)
    }
}
struct Runtime {
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
    clock: Arc<dyn Clock>,
    _root: Directory,
}
impl Runtime {
    fn new() -> Self {
        let root = Directory::new();
        let mut server =
            runtime_clock_server::Server::bind(&root.0).unwrap_or_else(|error| panic!("{error:?}"));
        let binding = ClockBinding {
            socket: server.socket().to_path_buf(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        };
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = stop.clone();
        let worker = thread::spawn(move || {
            while !stopping.load(Ordering::Acquire) {
                server.poll().unwrap_or_else(|error| panic!("{error:?}"));
                thread::sleep(Duration::from_millis(1));
            }
        });
        let source = RuntimeClock::connect(binding).unwrap_or_else(|error| panic!("{error:?}"));
        let clock = Arc::new(ClockSource {
            source,
            observations: Mutex::new(()),
        });
        Self {
            stop,
            worker: Some(worker),
            clock,
            _root: root,
        }
    }
    fn reading(&self) -> ClockReading {
        self.clock
            .sample(Duration::from_secs(1))
            .unwrap_or_else(|error| panic!("{error:?}"))
    }
    fn elapsed_since(&self, start: ClockReading) -> Duration {
        self.reading()
            .elapsed_since(start)
            .unwrap_or_else(|error| panic!("{error:?}"))
    }
    fn now(&self) -> u64 {
        self.reading().unix_seconds()
    }
}
impl Drop for Runtime {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            worker.join().unwrap_or_else(|error| panic!("{error:?}"));
        }
    }
}

#[derive(Default)]
struct Gate {
    state: Mutex<(bool, usize)>,
    changed: Condvar,
}
impl Gate {
    fn hold(&self) {
        let mut state = self
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        *state = (true, 0);
    }
    fn after_signature(&self) {
        let mut state = self
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.1 += 1;
        self.changed.notify_all();
        while state.0 {
            state = self
                .changed
                .wait(state)
                .unwrap_or_else(|error| panic!("{error:?}"));
        }
    }
    fn entered(&self, count: usize) {
        let state = self
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        let (state, timeout) = self
            .changed
            .wait_timeout_while(state, Duration::from_secs(10), |state| state.1 < count)
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert!(
            !timeout.timed_out() && state.1 >= count,
            "attestor did not reach response barrier"
        );
    }
    fn release(&self) {
        self.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .0 = false;
        self.changed.notify_all();
    }
}
#[derive(Clone)]
struct PeerContext {
    id: String,
    clock: Arc<dyn Clock>,
    gate: Arc<Gate>,
    lose: Arc<AtomicBool>,
    signs: Arc<AtomicUsize>,
    // The native peer reproduces the attestor native-v2 durable key/session contract.
    journal: PathBuf,
    sessions: Arc<Mutex<BTreeMap<String, (String, Value)>>>,
    sequence: Arc<AtomicUsize>,
}
struct Peers {
    runtime: Runtime,
    root: Directory,
    nodes: Vec<(String, SocketAddr)>,
    stop: Arc<AtomicBool>,
    workers: Mutex<Vec<JoinHandle<()>>>,
    gate: Arc<Gate>,
    lose: Arc<AtomicBool>,
    signs: Arc<AtomicUsize>,
}
fn native_key() -> Ed25519KeyPair {
    Ed25519KeyPair::from_seed_unchecked(&[7; 32]).unwrap_or_else(|error| panic!("{error:?}"))
}
fn evm_key(owner: &str) -> k256::ecdsa::SigningKey {
    k256::ecdsa::SigningKey::from_bytes((&[if owner == "bob" { 10 } else { 9 }; 32]).into())
        .unwrap_or_else(|error| panic!("{error:?}"))
}
fn address(owner: &str) -> [u8; 20] {
    let point = evm_key(owner).verifying_key().to_encoded_point(false);
    Keccak256::digest(&point.as_bytes()[1..])[12..]
        .try_into()
        .unwrap_or_else(|error| panic!("{error:?}"))
}
fn token(subject: &str, issued: u64, expires: u64, tag: u64) -> String {
    let header = URL_SAFE_NO_PAD.encode(br#"{"alg":"EdDSA","typ":"JWT"}"#);
    let claims = URL_SAFE_NO_PAD.encode(
        serde_json::to_vec(&json!({"sub":subject,"iat":issued,"exp":expires,"jti":tag}))
            .unwrap_or_else(|error| panic!("{error:?}")),
    );
    let message = format!("{header}.{claims}");
    let signature = native_key().sign(message.as_bytes());
    format!("{message}.{}", URL_SAFE_NO_PAD.encode(signature.as_ref()))
}
fn authenticate(token: &str, clock: &dyn Clock) -> String {
    let parts: Vec<_> = token.split('.').collect();
    assert_eq!(parts.len(), 3);
    ring::signature::UnparsedPublicKey::new(
        &ring::signature::ED25519,
        native_key().public_key().as_ref(),
    )
    .verify(
        format!("{}.{}", parts[0], parts[1]).as_bytes(),
        &URL_SAFE_NO_PAD
            .decode(parts[2])
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let claims: Value = serde_json::from_slice(
        &URL_SAFE_NO_PAD
            .decode(parts[1])
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let now = clock
        .sample(Duration::from_secs(1))
        .unwrap_or_else(|error| panic!("{error:?}"))
        .unix_seconds();
    assert!(
        claims["iat"]
            .as_u64()
            .unwrap_or_else(|| panic!("expected value"))
            <= now
            && claims["exp"]
                .as_u64()
                .unwrap_or_else(|| panic!("expected value"))
                > now
    );
    claims["sub"]
        .as_str()
        .unwrap_or_else(|| panic!("expected value"))
        .to_owned()
}
impl Peers {
    fn new() -> Arc<Self> {
        let runtime = Runtime::new();
        let root = Directory::new();
        let tls = server_tls();
        private(&root.0.join("ca.der"), CA);
        private(&root.0.join("peer.der"), CERT);
        private(&root.0.join("peer.pk8"), TLS_KEY);
        let stop = Arc::new(AtomicBool::new(false));
        let gate = Arc::new(Gate::default());
        let lose = Arc::new(AtomicBool::new(false));
        let signs = Arc::new(AtomicUsize::new(0));
        let mut nodes = Vec::new();
        let mut workers = Vec::new();
        for id in SIGNERS {
            let listener =
                TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{error:?}"));
            nodes.push((
                id.into(),
                listener
                    .local_addr()
                    .unwrap_or_else(|error| panic!("{error:?}")),
            ));
            listener
                .set_nonblocking(true)
                .unwrap_or_else(|error| panic!("{error:?}"));
            let context = PeerContext {
                id: id.into(),
                clock: runtime.clock.clone(),
                gate: gate.clone(),
                lose: lose.clone(),
                signs: signs.clone(),
                journal: root.0.join(format!("{id}.json")),
                sessions: Arc::new(Mutex::new(BTreeMap::new())),
                sequence: Arc::new(AtomicUsize::new(0)),
            };
            let stopping = stop.clone();
            let config = tls.clone();
            workers.push(thread::spawn(move || {
                let mut requests = Vec::new();
                while !stopping.load(Ordering::Acquire) {
                    match listener.accept() {
                        Ok((tcp, _)) => {
                            let context = context.clone();
                            let config = config.clone();
                            requests.push(thread::spawn(move || {
                                let _ = peer_request(tcp, config, &context);
                            }));
                        }
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            thread::sleep(Duration::from_millis(1));
                        }
                        Err(error) => panic!("{error}"),
                    }
                }
                for request in requests {
                    request.join().unwrap_or_else(|error| panic!("{error:?}"));
                }
            }));
        }
        Arc::new(Self {
            runtime,
            root,
            nodes,
            stop,
            workers: Mutex::new(workers),
            gate,
            lose,
            signs,
        })
    }
    fn signer_config(&self, owner: &str) -> attestor::AttestorSignerConfig {
        attestor::AttestorSignerConfig {
            nodes: self
                .nodes
                .iter()
                .map(|(id, address)| attestor::AttestorNodeConfig {
                    id: id.clone(),
                    address: *address,
                })
                .collect(),
            trust_anchors: vec![self.root.0.join("ca.der")],
            client_certificate_chain: vec![self.root.0.join("peer.der")],
            client_private_key_pkcs8: self.root.0.join("peer.pk8"),
            key_id: "native-owner".into(),
            public_key: native_key()
                .public_key()
                .as_ref()
                .try_into()
                .unwrap_or_else(|error| panic!("{error:?}")),
            evm_key_id: format!("evm-{owner}"),
            evm_address: address(owner),
            signers: SIGNERS.iter().map(|id| (*id).to_string()).collect(),
            network_id: 125,
            timeout_seconds: 10,
        }
    }
    fn signer(&self) -> attestor::AttestorSigner {
        let mut signer = attestor::AttestorSigner::from_config(&self.signer_config("alice"))
            .unwrap_or_else(|error| panic!("{error:?}"));
        let now = self.runtime.now();
        signer
            .set_evm_assertion(&token("alice", now - 1, now + 3600, 1))
            .unwrap_or_else(|error| panic!("{error:?}"));
        signer
    }
}
impl Drop for Peers {
    fn drop(&mut self) {
        self.gate.release();
        self.stop.store(true, Ordering::Release);
        for worker in self
            .workers
            .get_mut()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .drain(..)
        {
            worker.join().unwrap_or_else(|error| panic!("{error:?}"));
        }
    }
}
fn sign_native_peer(
    peer: &PeerContext,
    request: &serde_json::Value,
    body: &[u8],
    status: &mut u16,
) -> serde_json::Value {
    let session = request["session_id"]
        .as_str()
        .unwrap_or_else(|| panic!("expected value"))
        .to_owned();
    let request_hash = hex(&Sha256::digest(body));
    let mut sessions = peer
        .sessions
        .lock()
        .unwrap_or_else(|error| panic!("{error:?}"));
    // Reopen the peer's committed journal at each request, as on restart.
    if peer.journal.exists() {
        *sessions = serde_json::from_slice(
            &fs::read(&peer.journal).unwrap_or_else(|error| panic!("{error:?}")),
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
    }
    if let Some((bound, response)) = sessions.get(&session) {
        if bound == &request_hash {
            response.clone()
        } else {
            *status = 400;
            json!({"error":{"category":"session","code":"session.bad_request","message":"session binds different request"}})
        }
    } else {
        let canonical = unhex(
            request["purpose"]
                .as_str()
                .unwrap_or_else(|| panic!("expected value")),
        );
        let decoded = layerx_agent_api::identity::NativePreparationPurposeV1::from_canonical_bytes(
            &canonical,
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        assert!(
            decoded.expires_at_ms
                > peer
                    .clock
                    .sample(Duration::from_secs(1))
                    .unwrap_or_else(|error| panic!("{error:?}"))
                    .unix_milliseconds
        );
        let digest = Sha256::digest(&canonical);
        let signature = native_key().sign(&digest);
        let response = json!({"profile":2,"node_id":peer.id,"key_id":request["key_id"],"operation":request["operation"],"signed_bytes":hex(&digest),"signature":hex(signature.as_ref()),"audit_sequence":peer.sequence.fetch_add(1, Ordering::AcqRel) + 1});
        sessions.insert(session, (request_hash, response.clone()));
        private(
            &peer.journal,
            &serde_json::to_vec(&*sessions).unwrap_or_else(|error| panic!("{error:?}")),
        );
        response
    }
}

fn peer_request(
    tcp: TcpStream,
    config: Arc<ServerConfig>,
    peer: &PeerContext,
) -> std::io::Result<()> {
    tcp.set_read_timeout(Some(Duration::from_secs(10)))?;
    tcp.set_write_timeout(Some(Duration::from_secs(10)))?;
    let mut tls = StreamOwned::new(
        ServerConnection::new(config).unwrap_or_else(|error| panic!("{error:?}")),
        tcp,
    );
    let (header, body, request) = read_peer_request(&mut tls)?;
    let path = header
        .split_whitespace()
        .nth(1)
        .unwrap_or_else(|| panic!("expected value"));
    let mut status = 200;
    let value = if path == "/health" {
        json!({"node_id":peer.id,"ready":true,"audit_sequence":peer.sequence.load(Ordering::Acquire) + 1,"reachable_peers":3})
    } else if path == "/v1/keys/generate" {
        json!({"node_id":peer.id,"key_id":request["key_id"],"curve":"ed25519","public_key":hex(native_key().public_key().as_ref()),"audit_sequence":peer.sequence.fetch_add(1, Ordering::AcqRel) + 1})
    } else {
        let bearer = header
            .lines()
            .find_map(|line| {
                line.strip_prefix("authorization: Bearer ")
                    .or_else(|| line.strip_prefix("Authorization: Bearer "))
            })
            .unwrap_or_else(|| panic!("expected bearer authorization"));
        let owner = authenticate(bearer, peer.clock.as_ref());
        if path == "/v1/keys/public-wallet" {
            assert_eq!(request["owner"], owner);
            assert_eq!(
                request["public_key"],
                hex(native_key().public_key().as_ref())
            );
            json!({"node_id":peer.id,"key_id":request["key_id"],"public_key":request["public_key"],"owner":owner,"wallet_key_id":format!("evm-{owner}"),"wallet_public_key":hex(evm_key(&owner).verifying_key().to_encoded_point(false).as_bytes()),"wallet_epoch":1,"address":format!("0x{}",hex(&address(&owner))),"audit_sequence":peer.sequence.fetch_add(1, Ordering::AcqRel) + 1})
        } else if path == "/v2/sign/native" {
            sign_native_peer(peer, &request, &body, &mut status)
        } else {
            assert_eq!(path, "/v1/sign");
            assert_eq!(request["key_id"], format!("evm-{owner}"));
            assert_eq!(request["kind"], "evm_tx");
            let raw = unhex(
                request["transaction"]
                    .as_str()
                    .unwrap_or_else(|| panic!("expected value")),
            );
            let call =
                attestor::decode_peer_evm_wire(&raw).unwrap_or_else(|error| panic!("{error:?}"));
            let digest = call.signing_hash();
            let (signature, recovery) = evm_key(&owner)
                .sign_prehash_recoverable(&digest)
                .unwrap_or_else(|error| panic!("{error:?}"));
            let mut signed = signature.to_bytes().to_vec();
            signed.push(recovery.to_byte());
            let value = json!({"node_id":peer.id,"key_id":request["key_id"],"kind":"evm_tx","signed_bytes":hex(&digest),"signature":hex(&signed),"recovery_id":recovery.to_byte(),"audit_sequence":peer.sequence.fetch_add(1, Ordering::AcqRel) + 1});
            peer.signs.fetch_add(1, Ordering::AcqRel);
            peer.gate.after_signature();
            if peer.lose.swap(false, Ordering::AcqRel) {
                return Ok(());
            }
            value
        }
    };
    let bytes = serde_json::to_vec(&value).unwrap_or_else(|error| panic!("{error:?}"));
    write!(tls, "HTTP/1.1 {status} fixture\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", bytes.len())?;
    tls.write_all(&bytes)?;
    tls.flush()
}

struct Host {
    peers: Arc<Peers>,
    root: Directory,
    config: Arc<config::Config>,
    store: Option<Arc<Mutex<store::Store>>>,
    address: SocketAddr,
    active: Arc<AtomicUsize>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
    manifest: PathBuf,
}
impl Host {
    fn new(peers: Arc<Peers>, deadline: Duration) -> Self {
        let root = Directory::new();
        let manifest = root.0.join("attestors.json");
        let now = peers.runtime.now();
        let mut bindings = Vec::new();
        for (index, owner) in ["alice", "bob"].iter().enumerate() {
            let path = root.0.join(format!("{owner}.jwt"));
            private(&path, token(owner, now - 1, now + 3600, 1).as_bytes());
            bindings.push(json!({"binding":binding(index).digest(),"tenant":"tenant","principal":owner,"config":peers.signer_config(owner),"assertion_file":path}));
        }
        private(
            &manifest,
            &serde_json::to_vec(&json!({"bindings":bindings}))
                .unwrap_or_else(|error| panic!("{error:?}")),
        );
        let config = Arc::new(config::Config {
            listen: "127.0.0.1:0"
                .parse()
                .unwrap_or_else(|error| panic!("{error:?}")),
            state: root.0.join("state"),
            provider: PROVIDER.into(),
            chain: config::ChainSnapshot { chain_id: 125 },
            seal: Zeroizing::new(vec![33; 32]),
            tls: server_tls(),
            client_pin: Sha256::digest(CERT).into(),
            attestor_evm_client_certificate: None,
            deadline,
        });
        let mut host = Self {
            peers,
            root,
            config,
            store: None,
            address: "127.0.0.1:0"
                .parse()
                .unwrap_or_else(|error| panic!("{error:?}")),
            active: Arc::new(AtomicUsize::new(0)),
            stop: Arc::new(AtomicBool::new(false)),
            worker: None,
            manifest,
        };
        host.start();
        host
    }
    fn start(&mut self) {
        let store = Arc::new(Mutex::new(
            store::Store::open_with_attestors(&self.config, Some(&self.manifest))
                .unwrap_or_else(|error| panic!("{error:?}")),
        ));
        let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{error:?}"));
        self.address = listener
            .local_addr()
            .unwrap_or_else(|error| panic!("{error:?}"));
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("{error:?}"));
        self.stop.store(false, Ordering::Release);
        let stopping = self.stop.clone();
        let config = self.config.clone();
        let active = self.active.clone();
        let clock = self.peers.runtime.clock.clone();
        let shared = store.clone();
        self.worker = Some(thread::spawn(move || {
            let mut workers = Vec::new();
            while !stopping.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((tcp, _)) => match server::spawn_connection(
                        tcp,
                        config.clone(),
                        shared.clone(),
                        clock.clone(),
                        active.clone(),
                    ) {
                        Ok(worker) => workers.push(worker),
                        Err(wire::Error::Capacity) => {}
                        Err(error) => panic!("{error:?}"),
                    },
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("{error}"),
                }
            }
            for worker in workers {
                worker.join().unwrap_or_else(|error| panic!("{error:?}"));
            }
        }));
        self.store = Some(store);
    }
    fn stop(&mut self) {
        self.peers.gate.release();
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            worker.join().unwrap_or_else(|error| panic!("{error:?}"));
        }
        self.store.take();
        assert_eq!(self.active.load(Ordering::Acquire), 0);
    }
    fn restart(&mut self) {
        self.stop();
        self.start();
    }
    fn remote(&self) -> RemoteKmsProvider {
        remote(self.address, self.peers.runtime.clock.clone())
    }
    fn create(&self, index: usize) -> ProviderKeyDescription {
        let remote = self.remote();
        let description = remote
            .create_key(&binding(index))
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(
            remote
                .evm_operation(6, &binding(index), description.reference(), &[])
                .unwrap_or_else(|error| panic!("{error:?}")),
            address(if index == 0 { "alice" } else { "bob" })
        );
        description
    }
    fn exchange(
        &self,
        operation: u8,
        index: usize,
        description: &ProviderKeyDescription,
        payload: &[u8],
    ) -> Vec<u8> {
        exchange_produced(
            self.address,
            self.peers.runtime.clock.clone(),
            operation,
            index,
            description.reference(),
            payload,
        )
    }

    fn action(
        &self,
        operation: u8,
        index: usize,
        description: &ProviderKeyDescription,
        payload: &[u8],
    ) -> evm_types::EvmAction {
        let response = self.exchange(operation, index, description, payload);
        assert_eq!(response[7], 0, "{}", hex(&response));
        serde_json::from_slice(&response[8..]).unwrap_or_else(|error| panic!("{error:?}"))
    }
}
impl Drop for Host {
    fn drop(&mut self) {
        self.stop();
    }
}
fn binding(index: usize) -> PrincipalKeyBinding {
    PrincipalKeyBinding::new(
        format!(
            "layerx-human-custody/v1\0tenant/{}\0primary\0human-primary",
            if index == 0 { "alice" } else { "bob" }
        )
        .into_bytes(),
        125,
        KeyClass::HumanPrimary,
        PROVIDER,
    )
    .unwrap_or_else(|error| panic!("{error:?}"))
}
fn remote(address: SocketAddr, clock: Arc<dyn Clock>) -> RemoteKmsProvider {
    let mutual = MutualTlsConfig::new(
        roots(),
        vec![CertificateDer::from(CERT.to_vec())],
        PrivateKeyDer::try_from(TLS_KEY.to_vec()).unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    RemoteKmsProvider::new(
        PROVIDER,
        address,
        "127.0.0.1",
        mutual,
        Limits {
            maximum_frame_bytes: MAX_FRAME,
            maximum_connections: 16,
            maximum_streams: 1,
            maximum_queued_bytes: MAX_FRAME,
            deadline: Duration::from_secs(10),
        },
        clock,
    )
    .unwrap_or_else(|error| panic!("{error:?}"))
}
// Use the production Human encoder, including the actual LXKP version and payload framing.
fn exchange_produced(
    address: SocketAddr,
    clock: Arc<dyn Clock>,
    operation: u8,
    index: usize,
    reference: &layerx_human_service::custody::ProviderKeyReference,
    payload: &[u8],
) -> Vec<u8> {
    let request = remote(address, clock)
        .encode_evm_request(operation, &binding(index), reference, payload)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let mut stream = tls(address);
    write_frame(&mut stream, &request, MAX_FRAME).unwrap_or_else(|error| panic!("{error:?}"));
    read_frame(&mut stream, MAX_FRAME).unwrap_or_else(|error| panic!("{error:?}"))
}
fn call(nonce: u64) -> Eip1559Call {
    Eip1559Call {
        chain_id: 125,
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: [4; 20],
        data: Vec::new(),
    }
}
fn plan(peers: &Peers, index: usize, nonce: u64, id: u8) -> evm_types::EvmPlanAuthorization {
    let now = peers.runtime.now();
    evm_types::EvmPlanAuthorization {
        plan_id: [1; 32],
        action_key: [id; 32],
        tenant: "tenant".into(),
        principal: if index == 0 {
            "alice".into()
        } else {
            "bob".into()
        },
        binding_digest: binding(index).digest(),
        wallet: address(if index == 0 { "alice" } else { "bob" }),
        not_before: now - 1,
        not_after: now + 600,
        transaction: evm_types::EvmTransaction {
            chain_id: 125,
            nonce,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 100_000,
            to: [4; 20],
            value: [0; 32],
            calldata: Vec::new(),
        },
    }
}
fn send_plan(peers: &Peers, nonce: u64, id: u8) -> send::SendAuthorization {
    let mut authorization = plan(peers, 0, nonce, id);
    let recipient = [0x44; 20];
    let mut amount = [0; 32];
    amount[31] = 42;
    let mut bank = [0; 20];
    bank[18..].copy_from_slice(&[0x10, 0x01]);
    authorization.transaction.to = bank;
    let mut data =
        layerx_crypto::disclosure::call::ledger_selector("send(address,address,string,uint256)")
            .to_vec();
    for address in [authorization.wallet, recipient] {
        data.extend([0; 12]);
        data.extend(address);
    }
    let mut offset = [0; 32];
    offset[31] = 128;
    data.extend(offset);
    data.extend(amount);
    let mut length = [0; 32];
    length[31] = 4;
    data.extend(length);
    data.extend(b"apax");
    data.extend([0; 28]);
    authorization.transaction.calldata = data;
    send::SendAuthorization {
        authorization,
        disclosure: send::SendDisclosure {
            recipient,
            denom: "apax".into(),
            amount,
        },
    }
}
fn encoded<T: serde::Serialize>(value: &T) -> Vec<u8> {
    serde_json::to_vec(value).unwrap_or_else(|error| panic!("{error:?}"))
}
#[track_caller]
fn await_count(value: &AtomicUsize, count: usize, clock: &dyn Clock) {
    let mut deadline =
        Deadline::start(clock, Duration::from_secs(5)).unwrap_or_else(|error| panic!("{error:?}"));
    while value.load(Ordering::Acquire) != count {
        assert!(
            !deadline
                .remaining(clock)
                .unwrap_or_else(|error| panic!("{error:?}"))
                .is_zero(),
            "count did not become {count}; active={}",
            value.load(Ordering::Acquire)
        );
        thread::sleep(Duration::from_millis(1));
    }
}
fn ready<F: Future>(future: F) -> F::Output {
    let mut future = std::pin::pin!(future);
    let mut context = std::task::Context::from_waker(std::task::Waker::noop());
    match future.as_mut().poll(&mut context) {
        std::task::Poll::Ready(result) => result,
        std::task::Poll::Pending => panic!("synchronous signer unexpectedly yielded"),
    }
}

#[test]
fn concurrent_sessions_delay_and_exact_signatures() -> Result {
    let peers = Peers::new();
    let signer = Arc::new(peers.signer());
    peers.gate.hold();
    let first = call(1);
    let second = call(2);
    let first_raw = attestor::unsigned_evm_transaction(&first);
    let second_raw = attestor::unsigned_evm_transaction(&second);
    let signer_a = signer.clone();
    let worker_a = thread::spawn(move || signer_a.sign_evm_transaction(&first_raw, 125));
    peers.gate.entered(3);
    let signer_b = signer.clone();
    let worker_b = thread::spawn(move || signer_b.sign_evm_transaction(&second_raw, 125));
    thread::sleep(Duration::from_millis(100));
    assert_eq!(
        peers.signs.load(Ordering::Acquire),
        3,
        "second session must wait until first response is accepted"
    );
    peers.gate.release();
    let a = worker_a
        .join()
        .unwrap_or_else(|error| panic!("{error:?}"))?;
    let b = worker_b
        .join()
        .unwrap_or_else(|error| panic!("{error:?}"))?;
    for (call, returned) in [(&first, a), (&second, b)] {
        let (expected, recovery) =
            evm_key("alice").sign_prehash_recoverable(&call.signing_hash())?;
        assert_eq!(&returned[..64], expected.to_bytes().as_slice());
        assert_eq!(returned[64], recovery.to_byte());
    }
    assert_eq!(
        signer.audit_sequences(),
        SIGNERS.iter().map(|id| ((*id).to_string(), 2)).collect()
    );
    Ok(())
}

#[test]
fn native_sessions_journal_replay_conflict_and_restart() -> Result {
    use layerx_agent_api::identity::{
        AgentDid, CapabilityId, NativePreparationPurposeV1, SessionId, TenantId,
    };
    let peers = Peers::new();
    let journal = peers.root.0.join("native-replay.json");
    let now = peers.runtime.now();
    let mut purpose = NativePreparationPurposeV1 {
        tenant: checked(TenantId::new("tenant"))?,
        agent_did: checked(AgentDid::new("did:layerx:alice"))?,
        session_id: checked(SessionId::new(hex(&[1; 32])))?,
        generation: 1,
        expires_at_ms: (now + 600) * 1000,
        capability_id: checked(CapabilityId::new(hex(&[2; 32])))?,
        preparation_id: [3; 32],
        canonical_digest: [4; 32],
        commitment: [5; 32],
    };
    let assertion = token("alice", now - 1, now + 3600, 1);
    let signer = peers
        .signer()
        .with_native_replay_journal(journal.clone(), peers.runtime.clock.clone())?;
    let first = signer.sign_native_preparation_purpose(&purpose, "native-0", &assertion)?;
    for index in 1..520 {
        purpose.preparation_id[..8].copy_from_slice(&u64::try_from(index)?.to_be_bytes());
        let signature = signer.sign_native_preparation_purpose(
            &purpose,
            &format!("native-{index}"),
            &assertion,
        )?;
        assert_eq!(
            signature.digest(),
            &<[u8; 32]>::from(Sha256::digest(checked(purpose.canonical_bytes())?))
        );
    }
    let high = signer.audit_sequences();
    assert_eq!(high.values().copied().collect::<Vec<_>>(), vec![520; 3]);
    let mut original = purpose.clone();
    original.preparation_id = [3; 32];
    assert_eq!(
        signer.sign_native_preparation_purpose(&original, "native-0", &assertion)?,
        first
    );
    assert_eq!(signer.audit_sequences(), high);
    assert!(signer
        .sign_native_preparation_purpose(&purpose, "native-0", &assertion)
        .is_err());
    drop(signer);
    let recovered = peers
        .signer()
        .with_native_replay_journal(journal.clone(), peers.runtime.clock.clone())?;
    assert_eq!(recovered.audit_sequences(), high);
    assert_eq!(
        recovered.sign_native_preparation_purpose(&original, "native-0", &assertion)?,
        first
    );
    assert!(recovered
        .sign_native_preparation_purpose(&purpose, "native-0", &assertion)
        .is_err());
    assert!(fs::metadata(&journal)?.len() < 8 * 1024 * 1024);
    let mut expiring = purpose.clone();
    let expires = peers.runtime.now() + 1;
    expiring.expires_at_ms = expires * 1000;
    recovered.sign_native_preparation_purpose(&expiring, "expiring", &assertion)?;
    while peers.runtime.now() <= expires {
        thread::sleep(Duration::from_millis(20));
    }
    assert!(matches!(
        recovered.sign_native_preparation_purpose(&expiring, "expiring", &assertion),
        Err(attestor::AttestorError::Refused { code, .. }) if code == "assertion.expired"
    ));
    drop(recovered);
    let recovered = peers
        .signer()
        .with_native_replay_journal(journal.clone(), peers.runtime.clock.clone())?;
    let retained: Value = serde_json::from_slice(&fs::read(&journal)?)?;
    assert!(
        retained["records"].get("expiring").is_none(),
        "expired authority is compacted on reopen"
    );
    assert_eq!(
        recovered.sign_native_preparation_purpose(&original, "native-0", &assertion)?,
        first
    );
    Ok(())
}

#[test]
fn fragmented_handshake_and_frame_release_all_permits() -> Result {
    let peers = Peers::new();
    let host = Host::new(peers.clone(), Duration::from_millis(350));
    let mut streams = Vec::new();
    let mut client = ClientConnection::new(
        client_tls(),
        ServerName::IpAddress(host.address.ip().into()),
    )?;
    let mut hello = Vec::new();
    client.write_tls(&mut hello)?;
    assert!(hello.len() > 20);
    for _ in 0..16 {
        let mut tcp = TcpStream::connect(host.address)?;
        tcp.write_all(&hello[..1])?;
        streams.push(tcp);
    }
    await_count(&host.active, 16, peers.runtime.clock.as_ref());
    let started = peers.runtime.reading();
    for byte in &hello[1..31] {
        thread::sleep(Duration::from_millis(25));
        for stream in &mut streams {
            let _ = stream.write_all(&[*byte]);
        }
        if host.active.load(Ordering::Acquire) == 0 {
            break;
        }
    }
    assert_eq!(
        host.active.load(Ordering::Acquire),
        0,
        "continuous handshake fragments must not retain permits"
    );
    assert!(peers.runtime.elapsed_since(started) < Duration::from_millis(700));
    assert!(
        host.remote().probe().is_ok(),
        "permits must admit a new authenticated request"
    );
    let start = std::sync::Barrier::new(16);
    let mut frames = thread::scope(|scope| {
        let workers: Vec<_> = (0..16)
            .map(|_| {
                scope.spawn(|| -> std::io::Result<_> {
                    start.wait();
                    let mut stream = tls(host.address);
                    while stream.conn.is_handshaking() {
                        stream.conn.complete_io(&mut stream.sock)?;
                    }
                    stream.write_all(&100_u32.to_be_bytes())?;
                    stream.flush()?;
                    Ok(stream)
                })
            })
            .collect();
        workers
            .into_iter()
            .map(|worker| worker.join().unwrap_or_else(|error| panic!("{error:?}")))
            .collect::<std::io::Result<Vec<_>>>()
    })?;
    await_count(&host.active, 16, peers.runtime.clock.as_ref());
    let started = peers.runtime.reading();
    for _ in 0..30 {
        thread::sleep(Duration::from_millis(25));
        for stream in &mut frames {
            let _ = stream.write_all(&[0x41]);
            let _ = stream.flush();
        }
        if host.active.load(Ordering::Acquire) == 0 {
            break;
        }
    }
    assert_eq!(
        host.active.load(Ordering::Acquire),
        0,
        "continuous frame fragments must not retain permits"
    );
    assert!(peers.runtime.elapsed_since(started) < Duration::from_millis(700));
    assert!(host.remote().probe().is_ok());
    Ok(())
}

#[test]
fn fragmented_peer_close_and_completed_request_return_permits() -> Result {
    let peers = Peers::new();
    let host = Host::new(peers.clone(), Duration::from_secs(10));
    let mut streams = Vec::new();
    for _ in 0..16 {
        let mut stream = TcpStream::connect(host.address)?;
        stream.write_all(&[0x16])?;
        streams.push(stream);
    }
    await_count(&host.active, 16, peers.runtime.clock.as_ref());
    let started = peers.runtime.reading();
    drop(streams);
    await_count(&host.active, 0, peers.runtime.clock.as_ref());
    assert!(peers.runtime.elapsed_since(started) < Duration::from_secs(2));

    let mut streams = Vec::new();
    for _ in 0..16 {
        let mut stream = tls(host.address);
        while stream.conn.is_handshaking() {
            stream.conn.complete_io(&mut stream.sock)?;
        }
        stream.write_all(&100_u32.to_be_bytes())?;
        stream.write_all(&[0x41])?;
        stream.flush()?;
        streams.push(stream);
    }
    await_count(&host.active, 16, peers.runtime.clock.as_ref());
    let started = peers.runtime.reading();
    drop(streams);
    await_count(&host.active, 0, peers.runtime.clock.as_ref());
    assert!(peers.runtime.elapsed_since(started) < Duration::from_secs(2));
    for _ in 0..16 {
        host.remote().probe()?;
        await_count(&host.active, 0, peers.runtime.clock.as_ref());
    }
    Ok(())
}

#[test]
fn store_admission_has_the_same_deadline() {
    let peers = Peers::new();
    let host = Host::new(peers.clone(), Duration::from_millis(250));
    let shared = host
        .store
        .as_ref()
        .unwrap_or_else(|| panic!("expected value"))
        .clone();
    let guard = shared.lock().unwrap_or_else(|error| panic!("{error:?}"));
    let address = host.address;
    let clock = peers.runtime.clock.clone();
    let started = peers.runtime.reading();
    let worker = thread::spawn(move || remote(address, clock).probe());
    await_count(&host.active, 1, peers.runtime.clock.as_ref());
    await_count(&host.active, 0, peers.runtime.clock.as_ref());
    assert!(peers.runtime.elapsed_since(started) < Duration::from_secs(2));
    assert!(worker
        .join()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .is_err());
    drop(guard);
    assert!(host.remote().probe().is_ok());
}

#[test]
fn blocked_attestor_expires_queued_authority_and_late_result() {
    let peers = Peers::new();
    let mut host = Host::new(peers.clone(), Duration::from_secs(10));
    let description = host.create(0);
    let mut first = plan(&peers, 0, 0, 1);
    first.not_after = peers.runtime.now() + 1;
    host.action(7, 0, &description, &encoded(&first));
    peers.gate.hold();
    let address = host.address;
    let reference = description.reference().clone();
    let clock = peers.runtime.clock.clone();
    let worker = thread::spawn(move || {
        exchange_produced(address, clock, 8, 0, &reference, &encoded(&[1_u8; 32]))
    });
    peers.gate.entered(3);
    let mut queued = send_plan(&peers, 1, 2);
    queued.authorization.not_after = first.not_after;
    let address = host.address;
    let reference = description.reference().clone();
    let payload = encoded(&queued);
    let clock = peers.runtime.clock.clone();
    let queued_worker =
        thread::spawn(move || exchange_produced(address, clock, 11, 0, &reference, &payload));
    await_count(&host.active, 2, peers.runtime.clock.as_ref());
    while peers.runtime.now() <= first.not_after {
        thread::sleep(Duration::from_millis(20));
    }
    peers.gate.release();
    let late = worker.join().unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(late[7], wire::Error::Expired as u8);
    let queued_result = queued_worker
        .join()
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(queued_result[7], wire::Error::Refused as u8);
    assert_eq!(
        peers.signs.load(Ordering::Acquire),
        3,
        "expired queued authority must never reach the attestor"
    );
    host.restart();
    let recovered = host.action(10, 0, &description, &encoded(&first.action_key));
    assert!(recovered.raw_transaction.is_empty());
    assert_eq!(recovered.authorization, first);
    let replacement = send_plan(&peers, 1, 3);
    let result = host.exchange(11, 0, &description, &encoded(&replacement));
    assert_eq!(result[7], 0, "expired queue must not reserve a fresh nonce");
}

#[test]
fn protected_assertion_rotation_expiry_and_restart() -> Result {
    let peers = Peers::new();
    let mut host = Host::new(peers.clone(), Duration::from_secs(10));
    let description = host.create(0);
    let now = peers.runtime.now();
    let expired = token("alice", now - 3601, now + 600, 2);
    let claims = layerx_human_kms::attestor::assertion_claims(&expired)?;
    assert_eq!(claims.subject, "alice");
    assert!(matches!(
        claims.check(now),
        Err(layerx_human_kms::attestor::AttestorError::Refused { code, .. })
            if code == "assertion.expired"
    ));
    private(&host.root.0.join("alice.jwt"), expired.as_bytes());
    let authorization = plan(&peers, 0, 0, 1);
    host.action(7, 0, &description, &encoded(&authorization));
    assert_eq!(
        host.exchange(8, 0, &description, &encoded(&authorization.action_key))[7],
        wire::Error::Expired as u8
    );
    assert_eq!(peers.signs.load(Ordering::Acquire), 0);
    private(
        &host.root.0.join("rotated.jwt"),
        token("bob", now - 1, now + 600, 3).as_bytes(),
    );
    fs::rename(
        host.root.0.join("rotated.jwt"),
        host.root.0.join("alice.jwt"),
    )?;
    assert_eq!(
        host.exchange(8, 0, &description, &encoded(&authorization.action_key))[7],
        wire::Error::Integrity as u8
    );
    assert_eq!(peers.signs.load(Ordering::Acquire), 0);
    private(
        &host.root.0.join("rotated.jwt"),
        token("alice", now - 1, now + 600, 4).as_bytes(),
    );
    fs::rename(
        host.root.0.join("rotated.jwt"),
        host.root.0.join("alice.jwt"),
    )?;
    let signed = host.action(8, 0, &description, &encoded(&authorization.action_key));
    let (call, signature) = decode_signed(&signed.raw_transaction)?;
    assert_eq!(recover_sender(&call, &signature)?, address("alice"));
    host.restart();
    assert_eq!(
        host.action(10, 0, &description, &encoded(&authorization.action_key)),
        signed
    );
    let next = plan(&peers, 0, 1, 2);
    host.action(7, 0, &description, &encoded(&next));
    assert!(!host
        .action(8, 0, &description, &encoded(&next.action_key))
        .raw_transaction
        .is_empty());
    host.stop();
    private(
        &host.root.0.join("alice.jwt"),
        token("bob", now - 1, now + 600, 5).as_bytes(),
    );
    assert!(
        store::Store::open_with_attestors(&host.config, Some(&host.manifest)).is_err(),
        "restart must retain the original assertion subject binding"
    );
    Ok(())
}

#[test]
fn send_response_loss_reservation_and_restart() -> Result {
    let peers = Peers::new();
    let mut host = Host::new(peers.clone(), Duration::from_secs(10));
    let description = host.create(0);
    let original = send_plan(&peers, 0, 1);
    peers.lose.store(true, Ordering::Release);
    let response = host.exchange(11, 0, &description, &encoded(&original));
    assert_ne!(response[7], 0);
    assert_eq!(peers.signs.load(Ordering::Acquire), 3);
    host.restart();
    let reserved = host.action(
        10,
        0,
        &description,
        &encoded(&original.authorization.action_key),
    );
    assert_eq!(reserved.authorization, original.authorization);
    assert!(reserved.raw_transaction.is_empty());
    let changed = send_plan(&peers, 0, 2);
    assert_eq!(
        host.exchange(11, 0, &description, &encoded(&changed))[7],
        wire::Error::Conflict as u8
    );
    let mut changed_disclosure = original.clone();
    changed_disclosure.disclosure.amount[31] += 1;
    assert_eq!(
        host.exchange(11, 0, &description, &encoded(&changed_disclosure))[7],
        wire::Error::Refused as u8
    );
    assert_eq!(peers.signs.load(Ordering::Acquire), 3);
    let response = host.exchange(11, 0, &description, &encoded(&original));
    assert_eq!(response[7], 0);
    let raw = response[8..].to_vec();
    let (call, signature) = decode_signed(&raw)?;
    assert_eq!(recover_sender(&call, &signature)?, address("alice"));
    assert_eq!(call.nonce, 0);
    host.restart();
    assert_eq!(
        host.exchange(11, 0, &description, &encoded(&original))[8..],
        raw
    );
    assert_eq!(peers.signs.load(Ordering::Acquire), 6);
    Ok(())
}

fn assign_long_denom(request: &mut send::SendAuthorization) {
    request.disclosure.denom = "x".repeat(16_384);
    let tx = &mut request.authorization.transaction;
    let mut data =
        layerx_crypto::disclosure::call::ledger_selector("send(address,address,string,uint256)")
            .to_vec();
    for address in [request.authorization.wallet, request.disclosure.recipient] {
        data.extend([0; 12]);
        data.extend(address);
    }
    let mut offset = [0; 32];
    offset[31] = 128;
    data.extend(offset);
    data.extend(request.disclosure.amount);
    let mut length = [0; 32];
    length[24..].copy_from_slice(&(request.disclosure.denom.len() as u64).to_be_bytes());
    data.extend(length);
    data.extend(request.disclosure.denom.as_bytes());
    tx.calldata = data;
}

#[test]
fn wallet_capacity_refusal_archive_and_restart() -> Result {
    let peers = Peers::new();
    let mut host = Host::new(peers.clone(), Duration::from_secs(10));
    let alice = host.create(0);
    let bob = host.create(1);
    // Actual JSON byte growth reaches the per-wallet admission budget before the old 4096 action count.
    let mut accepted = Vec::new();
    let mut refused = None;
    for nonce in 0..20_u64 {
        let mut request = send_plan(&peers, nonce, u8::try_from(nonce)? + 1);
        // A long, canonical denom is produced by the real ABI encoder below; no padding is hidden from disclosure.
        assign_long_denom(&mut request);
        let response = host.exchange(11, 0, &alice, &encoded(&request));
        if response[7] == wire::Error::Capacity as u8 {
            refused = Some(request);
            break;
        }
        assert_eq!(response[7], 0);
        accepted.push((request, response[8..].to_vec()));
    }
    assert!(!accepted.is_empty());
    let refused = refused.ok_or("capacity was not exercised")?;
    let bob_plan = send_plan(&peers, 0, 99);
    let mut bob_plan = bob_plan;
    bob_plan.authorization = plan(&peers, 1, 0, 99);
    host.action(7, 1, &bob, &encoded(&bob_plan.authorization));
    assert!(!host
        .action(8, 1, &bob, &encoded(&bob_plan.authorization.action_key))
        .raw_transaction
        .is_empty());
    assert!(
        host.remote().probe().is_ok(),
        "capacity must not poison store health"
    );
    let (first, raw) = accepted.first().unwrap_or_else(|| panic!("expected value"));
    let action = host.action(10, 0, &alice, &encoded(&first.authorization.action_key));
    assert_eq!(&action.raw_transaction, raw);
    let acknowledged = host.action(
        9,
        0,
        &alice,
        &encoded(&evm_types::EvmAcknowledgement {
            action_key: first.authorization.action_key,
            transaction_hash: action
                .transaction_hash
                .unwrap_or_else(|| panic!("expected value")),
        }),
    );
    assert!(acknowledged.acknowledged);
    assert!(host
        .config
        .state
        .join(format!(
            "action-{}-{}.aead",
            hex(&binding(0).digest()),
            hex(&first.authorization.action_key)
        ))
        .exists());
    host.restart();
    assert_eq!(
        host.action(10, 0, &alice, &encoded(&first.authorization.action_key)),
        acknowledged
    );
    assert_eq!(host.exchange(11, 0, &alice, &encoded(first))[8..], *raw);
    let changed = send_plan(&peers, first.authorization.transaction.nonce, 100);
    assert_eq!(
        host.exchange(11, 0, &alice, &encoded(&changed))[7],
        wire::Error::Conflict as u8
    );
    assert!(host.remote().probe().is_ok());
    let recovered = host.action(
        10,
        0,
        &alice,
        &encoded(
            &accepted
                .last()
                .unwrap_or_else(|| panic!("expected value"))
                .0
                .authorization
                .action_key,
        ),
    );
    assert_eq!(
        recovered.raw_transaction,
        accepted
            .last()
            .unwrap_or_else(|| panic!("expected value"))
            .1
    );
    // The refused candidate did not consume its action or nonce, even across a restart.
    let small = send_plan(&peers, refused.authorization.transaction.nonce, 101);
    let result = host.exchange(11, 0, &alice, &encoded(&small));
    assert_eq!(result[7], 0);
    Ok(())
}

#[test]
fn human_consumer_calls_current_sign_activity_contract() -> Result {
    use layerx_human_service::custody::{
        CustodySigner, KeyId, Keystore, Operation, SignAuthorization, SignRequest, SigningLimits,
    };
    use layerx_human_service::server::production_components::{AttestorCustodyConfig, AttestorKms};
    use layerx_human_service::store::{
        AgentTenantId, PrincipalId, PrincipalStore, RetentionPeriod, RetentionPolicy, TenancyMap,
    };
    use layerx_human_service::trace::TraceId;
    let peers = Peers::new();
    let root = Directory::new();
    let config = AttestorCustodyConfig::new(
        peers.nodes.clone(),
        SIGNERS.iter().map(|id| (*id).to_string()).collect(),
        CA.to_vec(),
        CERT.to_vec(),
        TLS_KEY.to_vec(),
        Duration::from_secs(10),
    )?;
    let provider = AttestorKms::connect(config)?;
    let now = peers.runtime.now();
    let assertion = token("alice", now - 1, now + 600, 1);
    provider.admit_assertion("alice", &assertion)?;
    let principal = PrincipalId::new("alice")?;
    let key = KeyId::new("primary")?;
    let keystore = Keystore::open_production(root.0.join("keys"), 125, provider.clone())?;
    let public = provider.create_owned_key(
        &keystore,
        &principal,
        &key,
        "alice",
        "0x1111111111111111111111111111111111111111",
    )?;
    assert_eq!(public, native_key().public_key().as_ref());
    let tenancy = TenancyMap::new([(principal.clone(), AgentTenantId::new("tenant")?)])?;
    let store_root = root.0.join("human-store");
    let digest = tenancy.install(&store_root)?;
    let period = RetentionPeriod::new(86400);
    let store = PrincipalStore::open(
        &store_root,
        RetentionPolicy {
            journeys: period,
            notifications: period,
            audit: period,
            telemetry: period,
            cache: period,
        },
        digest,
    )?;
    let registry = checked(layerx_types::payload::ModuleRegistry::new(&[]))?;
    let custody = CustodySigner::new(keystore, store, registry, SigningLimits::new(32, 60)?);
    let disclosed = disclosed_send_call(public, now)?;
    let bytes = attestor::unsigned_evm_transaction(&disclosed.call);
    let trace = TraceId::mint([1; 16]);
    let grant = ready(custody.sign(SignRequest::new(
        &principal,
        &key,
        &trace,
        SignAuthorization::new(Operation::ProtocolMutation, None),
        &bytes,
        &disclosed.disclosure,
        now,
    )))?;
    let signature = checked(layerx_crypto::signer::AgentSignature::new(
        *grant.signature(),
    ))?;
    assert_eq!(
        recover_sender(&disclosed.call, &signature)?,
        address("alice")
    );
    assert_eq!(grant.signer_public_key(), public);
    assert_eq!(peers.signs.load(Ordering::Acquire), 3);
    // Compile and execute the exact exported named contract independently too.
    let signer = peers.signer();
    let approval = attestor::Approval {
        principal: "alice",
        session_id: "named-sign-activity",
        expires_at: now + 300,
    };
    let signature: attestor::AttestorCallSignature = signer.sign_activity(
        &disclosed.call,
        &disclosed.disclosure,
        address("alice"),
        &approval,
        &assertion,
    )?;
    assert_eq!(signature.digest(), &disclosed.call.signing_hash());
    assert_eq!(signature.signature().len(), 65);
    assert_eq!(signature.audit().len(), 3);
    Ok(())
}

fn read_peer_request(
    tls: &mut StreamOwned<ServerConnection, TcpStream>,
) -> std::io::Result<(String, Vec<u8>, Value)> {
    let mut header = Vec::new();
    while !header.ends_with(b"\r\n\r\n") {
        let mut byte = [0];
        tls.read_exact(&mut byte)?;
        header.extend(byte);
        assert!(header.len() < 70_000);
    }
    let header = String::from_utf8(header).unwrap_or_else(|error| panic!("{error:?}"));
    let length = header
        .lines()
        .find_map(|line| {
            line.to_ascii_lowercase()
                .strip_prefix("content-length:")
                .map(|value| {
                    value
                        .trim()
                        .parse::<usize>()
                        .unwrap_or_else(|error| panic!("{error:?}"))
                })
        })
        .unwrap_or(0);
    let mut body = vec![0; length];
    tls.read_exact(&mut body)?;
    let request: Value = if body.is_empty() {
        json!({})
    } else {
        serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{error:?}"))
    };
    Ok((header, body, request))
}

fn disclosed_send_call(public: [u8; 32], now: u64) -> Result<layerx_crypto::send::DisclosedCall> {
    use layerx_crypto::send::{ledger_payment_call, CallOptions, SendDebit};
    let actor = format!("did:layerx:{}", hex(&public));
    let recipient = format!("did:layerx:{}", hex(&[0x52; 32]));
    let account = |did: &str| -> Result<[u8; 32]> {
        let account = checked(layerx_types::account::AccountId::parse(&format!(
            "agent:{did}:main"
        )))?;
        checked(layerx_intents::canonical::account_id_for_protocol(
            &account, 3,
        ))
    };
    let debit = SendDebit {
        from: account(&actor)?,
        to: account(&recipient)?,
        asset: [0; 32],
        amount: 42,
        source_sequence: 1,
        idempotency_key: [4; 32],
        expires_at: now + 300,
        context_hash: [5; 32],
        conditions: Vec::new(),
        authorization_kind: 1,
        network_id: 125,
        protocol_version: 3,
    };
    let message = checked(debit.authorization_message())?;
    let signature = native_key().sign(&attestor::preimage(&message));
    let payload = checked(debit.encode_signed(public, signature.as_ref().try_into()?))?;
    checked(ledger_payment_call(
        &payload,
        &CallOptions {
            from: address("alice"),
            chain_id: 125,
            nonce: 0,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 100_000,
        },
    ))
}
