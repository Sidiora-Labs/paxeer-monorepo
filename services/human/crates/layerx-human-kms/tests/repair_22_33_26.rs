//! LXKP signing binds the EIP-1559 call, disclosure and sender to one response.

#![allow(dead_code)]

pub use layerx_human_kms::attestor;
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
#[path = "../src/send.rs"]
mod send;
#[allow(dead_code)]
#[path = "../src/store.rs"]
mod store;
#[allow(dead_code)]
#[path = "../src/wire.rs"]
mod wire;
mod server {
    include!(concat!(env!("CARGO_MANIFEST_DIR"), "/src/server.rs"));
    pub fn serve_connection(
        tcp: std::net::TcpStream,
        config: &crate::config::Config,
        store: &std::sync::Mutex<crate::store::Store>,
        clock: &dyn layerx_types::clock::Clock,
    ) -> crate::wire::Result<()> {
        connection_from_accept(tcp, config, store, clock, std::time::Instant::now())
    }
}
#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod clock_server;
mod fixture {
    use crate::{config, server, store, wire};
    use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};
    use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
    use layerx_human_service::custody::{KeyClass, PrincipalKeyBinding, RemoteKmsProvider};
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
    pub const PROVIDER: &str = "repair-22-33-26";
    pub static SERIAL: Mutex<()> = Mutex::new(());
    const CA: &[u8] = include_bytes!("fixtures/repair_22_33_27/ca.der");
    const SERVER: &[u8] = include_bytes!("fixtures/repair_22_33_27/server.der");
    const SERVER_KEY: &[u8] = include_bytes!("fixtures/repair_22_33_27/server.pk8");
    const CLIENT: &[u8] = include_bytes!("fixtures/repair_22_33_27/client.der");
    const CLIENT_KEY: &[u8] = include_bytes!("fixtures/repair_22_33_27/client.pk8");
    const EXECUTOR: &[u8] = include_bytes!("fixtures/repair_22_33_27/executor.der");
    const EXECUTOR_KEY: &[u8] = include_bytes!("fixtures/repair_22_33_27/executor.pk8");

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
    }
    impl Drop for Host {
        fn drop(&mut self) {
            drop(self.kms.take());
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
                let identity =
                    format!("layerx-human-custody/v1\0{principal}\0primary\0human-primary");
                let binding = PrincipalKeyBinding::new(
                    identity.into_bytes(),
                    125,
                    KeyClass::HumanPrimary,
                    PROVIDER,
                )?;
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
                    let worker = Worker::spawn(listener, move |tcp| {
                        // The peer uses the real EIP-1559 signing producer, with fixture-owned
                        // material independent of every KMS primary seed and seal.
                        peer(tcp, &tls, &node_id, &wallet_key, &fixture_secret, &sequence)
                            .unwrap_or_else(|error| {
                                panic!("{}: {error:?}", "signing fixture transport")
                            });
                    })?;
                    nodes.push(json!({"id":node,"address":worker.address}));
                    peers.push(worker);
                }
                let assertion = root.join(format!("owner-{index}.jwt"));
                let now = clock.sample(Duration::from_secs(1))?.unix_seconds();
                let header = URL_SAFE_NO_PAD.encode(br#"{"alg":"EdDSA","typ":"JWT"}"#);
                let claims = URL_SAFE_NO_PAD.encode(serde_json::to_vec(
                    &json!({"sub":principal,"iat":now - 1,"exp":now + 3600}),
                )?);
                let content = format!("{header}.{claims}");
                let issuer = Ed25519KeyPair::from_seed_unchecked(&[7; 32])
                    .map_err(|_| "fixture issuer key")?;
                let token = format!(
                    "{content}.{}",
                    URL_SAFE_NO_PAD.encode(issuer.sign(content.as_bytes()).as_ref())
                );
                fs::write(&assertion, token)?;
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
            };
            host.start()?;
            Ok(host)
        }
        pub fn start(&mut self) -> Result<()> {
            let store = Arc::new(Mutex::new(checked(store::Store::open_with_attestors(
                &self.config,
                Some(&self.root.join("attestors.json")),
            ))?));
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
        pub fn connection(
            &self,
            executor: bool,
        ) -> Result<StreamOwned<ClientConnection, TcpStream>> {
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
    }

    fn peer(
        tcp: TcpStream,
        config: &Arc<ServerConfig>,
        node: &str,
        key_id: &str,
        secret: &[u8; 32],
        sequence: &AtomicU64,
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
        let authorization = header
            .lines()
            .find_map(|line| {
                let (name, value) = line.split_once(':')?;
                name.eq_ignore_ascii_case("authorization")
                    .then_some(value.trim())
            })
            .ok_or("owner assertion missing")?;
        let parts: Vec<_> = authorization
            .strip_prefix("Bearer ")
            .ok_or("bearer missing")?
            .split('.')
            .collect();
        assert_eq!(parts.len(), 3);
        let issuer =
            Ed25519KeyPair::from_seed_unchecked(&[7; 32]).map_err(|_| "fixture issuer key")?;
        ring::signature::UnparsedPublicKey::new(
            &ring::signature::ED25519,
            issuer.public_key().as_ref(),
        )
        .verify(
            format!("{}.{}", parts[0], parts[1]).as_bytes(),
            &URL_SAFE_NO_PAD.decode(parts[2])?,
        )
        .map_err(|_| "owner assertion signature")?;
        let claims: Value = serde_json::from_slice(&URL_SAFE_NO_PAD.decode(parts[1])?)?;
        assert!(matches!(claims["sub"].as_str(), Some("alice" | "bob")));
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
        } else if payload.len() < 256 {
            out.extend([0xf8, u8::try_from(payload.len())?]);
        } else {
            return Err("fixture payload bound".into());
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
}

use fixture::{checked, Host, Result, SERIAL};
use layerx_crypto::disclosure::call::bind_call;
use layerx_crypto::evm_transaction::{recover_sender, Eip1559Call};
use layerx_crypto::signer::AgentSignature;
use layerx_human_kms::signing_contract;
use layerx_human_service::custody::{KmsProvider, ProviderSignRequest};

fn call(nonce: u64) -> Eip1559Call {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x1a]);
    let mut data =
        layerx_crypto::disclosure::call::ledger_selector("openAccount(bytes32)").to_vec();
    data.extend([3; 32]);
    Eip1559Call {
        chain_id: 125,
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to,
        data,
    }
}

#[test]
fn version_7_producer_transport_consumer_and_restart() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "fixture lock poisoned")?;
    let mut host = Host::new()?;
    let binding = host.bindings[0].clone();
    let description = host.remote(false)?.create_key(&binding)?;
    let wallet = host
        .remote(false)?
        .wallet_descriptor(&binding, description.reference())?;
    assert_eq!(signing_contract::VERSION, 7);
    assert_eq!(signing_contract::OPERATION, 5);
    assert_eq!(signing_contract::SENDER_BYTES, 20);
    assert_eq!(signing_contract::SIGNATURE_BYTES, 65);
    for nonce in 0..2 {
        let remote = host.remote(false)?;
        assert_eq!(
            remote.describe_key(&binding, description.reference())?,
            description
        );
        assert_eq!(
            remote.wallet_descriptor(&binding, description.reference())?,
            wallet
        );
        let call = call(nonce);
        let disclosure = checked(bind_call(&call, wallet.evm_secp256k1))?;
        let request = ProviderSignRequest::new(
            &call,
            &disclosure,
            wallet.evm_secp256k1,
            description.public_key(),
        );
        let frame = remote.encode_signing_request(&binding, description.reference(), request)?;
        let decoded = checked(wire::Request::decode(&frame))?;
        assert_eq!(
            (decoded.version, decoded.operation),
            (signing_contract::VERSION, signing_contract::OPERATION)
        );
        assert_eq!(decoded.kind, wire::Operation::Sign);
        assert_eq!(decoded.provider, fixture::PROVIDER);
        assert_eq!(decoded.binding, binding.digest());
        assert_eq!(decoded.network, 125);
        assert_eq!(decoded.class, 1);
        assert_eq!(decoded.reference, description.reference().as_bytes());
        assert_eq!(decoded.from, wallet.evm_secp256k1);
        assert_eq!(decoded.digest, call.signing_hash());
        assert_eq!(decoded.canonical, attestor::unsigned_evm_transaction(&call));
        assert_eq!(decoded.disclosure, checked(wire::disclosure(&disclosure))?);
        let response = host.exchange(&frame, false)?;
        assert_eq!(&response[..8], b"LXKP\0\x07\x05\0");
        assert_eq!(response.len(), 8 + signing_contract::SIGNATURE_BYTES);
        let signature: [u8; 65] = response[8..].try_into()?;
        assert!(matches!(signature[64], 27 | 28));
        assert_eq!(
            recover_sender(&call, &checked(AgentSignature::new(signature))?)?,
            wallet.evm_secp256k1
        );
        // The actual client also consumes and verifies the KMS result.
        assert_eq!(
            fixture::run(remote.sign(&binding, description.reference(), request))?,
            signature
        );
        if nonce == 0 {
            host.restart()?;
        }
    }
    Ok(())
}

#[test]
fn retired_activity_signing_is_explicitly_refused() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "fixture lock poisoned")?;
    let host = Host::new()?;
    let remote = host.remote(false)?;
    let binding = &host.bindings[0];
    let description = remote.create_key(binding)?;
    let wallet = remote.wallet_descriptor(binding, description.reference())?;
    let call = call(0);
    let disclosure = checked(bind_call(&call, wallet.evm_secp256k1))?;
    let request = ProviderSignRequest::new(
        &call,
        &disclosure,
        wallet.evm_secp256k1,
        description.public_key(),
    );
    let frame = remote.encode_signing_request(binding, description.reference(), request)?;
    for version in [0_u16, 1, 2, 3, 4, 5, 6, 8, u16::MAX] {
        let mut retired = frame.clone();
        retired[4..6].copy_from_slice(&version.to_be_bytes());
        assert!(matches!(
            wire::Request::decode(&retired),
            Err(wire::Error::Refused)
        ));
        let mut expected = retired[..7].to_vec();
        expected.push(1);
        assert_eq!(host.exchange(&retired, false)?, expected);
        if version == 1 {
            // Refusal is at the operation boundary, even before the old payload
            // can be interpreted as a current sender/transaction/disclosure.
            let decoded = checked(wire::Request::decode(&frame))?;
            let sender_at =
                7 + 4 + decoded.provider.len() + 32 + 4 + 1 + 4 + decoded.reference.len() + 32;
            retired.drain(sender_at..sender_at + signing_contract::SENDER_BYTES);
            assert_eq!(host.exchange(&retired, false)?, expected);
            assert_eq!(host.exchange(&retired[..7], false)?, expected);
        }
    }
    // Refused legacy requests have not consumed current signing authority.
    let response = host.exchange(&frame, false)?;
    assert_eq!(&response[..8], b"LXKP\0\x07\x05\0");
    assert_eq!(response.len(), 73);
    assert_eq!(
        fixture::run(remote.sign(binding, description.reference(), request))?.as_slice(),
        &response[8..]
    );
    Ok(())
}

#[test]
fn version_7_payload_bounds_are_enforced() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "fixture lock poisoned")?;
    let host = Host::new()?;
    let remote = host.remote(false)?;
    let binding = &host.bindings[0];
    let description = remote.create_key(binding)?;
    let wallet = remote.wallet_descriptor(binding, description.reference())?;
    let call = call(0);
    let disclosure = checked(bind_call(&call, wallet.evm_secp256k1))?;
    let request = ProviderSignRequest::new(
        &call,
        &disclosure,
        wallet.evm_secp256k1,
        description.public_key(),
    );
    let frame = remote.encode_signing_request(binding, description.reference(), request)?;
    let decoded = checked(wire::Request::decode(&frame))?;
    let sender_at = 7 + 4 + decoded.provider.len() + 32 + 4 + 1 + 4 + decoded.reference.len() + 32;
    let canonical_at = sender_at + signing_contract::SENDER_BYTES;
    let disclosure_at = canonical_at + 4 + decoded.canonical.len();
    let mut zero_sender = frame.clone();
    zero_sender[sender_at..canonical_at].fill(0);
    let mut trailing = frame.clone();
    trailing.push(0);
    let mut oversized = frame.clone();
    oversized[canonical_at..canonical_at + 4].copy_from_slice(&u32::MAX.to_be_bytes());
    let mut empty_disclosure = frame[..disclosure_at].to_vec();
    empty_disclosure.extend(0_u32.to_be_bytes());
    for invalid in [
        zero_sender,
        trailing,
        oversized,
        empty_disclosure,
        frame[..frame.len() - 1].to_vec(),
    ] {
        assert!(matches!(
            wire::Request::decode(&invalid),
            Err(wire::Error::Refused)
        ));
        assert_eq!(host.exchange(&invalid, false)?, b"LXKP\0\x07\x05\x01");
    }
    assert_eq!(host.exchange(&frame, false)?.len(), 73);
    Ok(())
}
