//! Proves: RemoteKmsProvider::encode_evm_request / Keystore -> LXKP mutual TLS and state.aead -> Request::decode / Store::dispatch / Keystore.
//!
//! Covers the frame, EvmWallet, Evm and AuthorizeSend contracts with a real
//! KMS subprocess, a real external secp256k1 signer and the production gateway.
//! The external-signature route deliberately imports a valid signed action
//! before AuthorizeSend: its cached signed result needs no threshold service.
//! All producer processes, keys, TLS identities and files belong to this test.
//! No attestor response, LXKP request or persisted store record is fabricated.
//!
//! Repairs store.rs::Store::evm: exported keys were reported as Refused because
//! their erased seed was checked before SelfCustodied; destroyed keys likewise
//! need NotFound. Also rejects empty JSON payloads at the wire/client boundary.
//! Repairs tests/provider.rs::Host::remote to use its own hosted runtime clock.
//! services/human/crates/layerx-human-service/src/custody/gateway.rs::Operation::from_wire
//! deliberately refuses version 3 EVM operations; the proof preserves that policy.

use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_crypto::disclosure::call::ledger_selector;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
use layerx_human_service::custody::{
    CustodyError, EvmAcknowledgement, EvmAction, EvmExternalSignature, EvmPlanAuthorization,
    EvmTransaction, GatewayLimits, GatewayPolicy, GatewayTls, KeyClass, KeyId, Keystore, KmsError,
    KmsProvider, LxkpGateway, PrincipalKeyBinding, ProviderKeyReference, RemoteKmsProvider,
    GATEWAY_FRAME_LIMIT,
};
use layerx_human_service::frame::{read_frame, write_frame, Limits, MutualTlsConfig};
use layerx_human_service::store::PrincipalId;
use layerx_types::clock::Clock;
use layerx_types::payload::ModuleRegistry;
use rustls::pki_types::{CertificateDer, PrivateKeyDer, ServerName};
use rustls::{ClientConfig, ClientConnection, RootCertStore, StreamOwned};
use serde_json::json;
use sha3::{Digest, Keccak256};
use std::fs;
use std::io::ErrorKind;
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::Duration;
use zeroize::Zeroizing;

#[allow(dead_code)]
#[path = "../src/wire.rs"]
mod wire;

#[allow(dead_code)]
#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod clock_server;

type Result<T = ()> = std::result::Result<T, Box<dyn std::error::Error>>;
const PROVIDER: &str = "proof-kms";
const WAIT: Duration = Duration::from_secs(10);
const SIGNER_SEED: [u8; 32] = [0x11; 32];
const SEAL_SECRET: [u8; 32] = [0x22; 32];
static FIXTURE_COUNTER: AtomicUsize = AtomicUsize::new(0);

fn checked<T, E: std::fmt::Debug>(result: std::result::Result<T, E>) -> Result<T> {
    result.map_err(|error| format!("{error:?}").into())
}

struct Host {
    root: PathBuf,
    address: SocketAddr,
    child: Option<Child>,
    clock: Arc<dyn Clock>,
    clock_host: ClockHost,
    // These reserved ports detect any unintended attempt to use the threshold
    // route. They never accept or return a substitute attestor response.
    unused_threshold: Vec<TcpListener>,
    signer: Zeroizing<[u8; 32]>,
    wallet: [u8; 20],
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
        let root = Path::new("/tmp").join(format!(
            "proof_human_kms_wire-{}-{}",
            std::process::id(),
            FIXTURE_COUNTER.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&root)?;
        fs::set_permissions(&root, fs::Permissions::from_mode(0o700))?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let signer = Zeroizing::new(SIGNER_SEED);
        let signing_key = k256::ecdsa::SigningKey::from_slice(signer.as_ref())?;
        let point = signing_key.verifying_key().to_encoded_point(false);
        let wallet = Keccak256::digest(&point.as_bytes()[1..])[12..].try_into()?;
        let clock_host = ClockHost::start(&root)?;
        let clock: Arc<dyn Clock> = RuntimeClock::connect(clock_host.binding.clone())?;
        let mut host = Self {
            root,
            address,
            child: None,
            clock,
            clock_host,
            unused_threshold: Vec::new(),
            signer,
            wallet,
        };
        host.pki()?;
        let seal = Zeroizing::new(SEAL_SECRET);
        host.write("seal", seal.as_ref())?;
        host.write("chain.json", br#"{"chain_id":125}"#)?;
        let mut nodes = Vec::new();
        for id in ["node-1", "node-2", "node-3"] {
            let node = TcpListener::bind("127.0.0.1:0")?;
            node.set_nonblocking(true)?;
            nodes.push(json!({"id":id, "address":node.local_addr()?}));
            host.unused_threshold.push(node);
        }
        use ring::signature::KeyPair as _;
        let identity = ring::signature::Ed25519KeyPair::from_seed_unchecked(host.signer.as_ref())
            .map_err(|error| format!("identity producer failed: {error:?}"))?;
        let identity_public: [u8; 32] = identity.public_key().as_ref().try_into()?;
        host.write(
            "attestor.json",
            &serde_json::to_vec(&json!({
                "nodes": nodes,
                "trust_anchors": [host.path("ca.der")],
                "client_certificate_chain": [host.path("client.der")],
                "client_private_key_pkcs8": host.path("client-key.der"),
                "key_id": "external-owner", "public_key": identity_public,
                "evm_key_id": "external-wallet", "evm_address": host.wallet,
                "signers": ["node-1", "node-2", "node-3"],
                "network_id": 125, "timeout_seconds": 1
            }))?,
        )?;
        // A genuine signed owner assertion is configuration only on this
        // external-signature route; no remote service consumes it.
        use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};
        use p256::ecdsa::signature::Signer as _;
        let issuer = p256::ecdsa::SigningKey::from_slice(&[3; 32])?;
        let now = host.clock.sample(WAIT)?.unix_seconds();
        let header = URL_SAFE_NO_PAD.encode(br#"{"alg":"ES256","typ":"JWT"}"#);
        let claims = URL_SAFE_NO_PAD.encode(serde_json::to_vec(&json!({
            "sub":"external-owner", "iat":now, "exp":now + 600
        }))?);
        let content = format!("{header}.{claims}");
        let signature: p256::ecdsa::Signature = issuer.sign(content.as_bytes());
        host.write(
            "owner.jwt",
            format!("{content}.{}", URL_SAFE_NO_PAD.encode(signature.to_bytes())).as_bytes(),
        )?;
        drop(listener);
        host.start()?;
        Ok(host)
    }

    fn path(&self, name: &str) -> PathBuf {
        self.root.join(name)
    }

    fn write(&self, name: &str, bytes: &[u8]) -> Result {
        fs::write(self.path(name), bytes)?;
        fs::set_permissions(self.path(name), fs::Permissions::from_mode(0o600))?;
        Ok(())
    }

    fn openssl(&self, args: &[&str]) -> Result {
        let output = Command::new("openssl")
            .args(args)
            .current_dir(&self.root)
            .output()?;
        assert!(
            output.status.success(),
            "TLS producer openssl failed: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        Ok(())
    }

    fn pki(&self) -> Result {
        self.openssl(&[
            "req",
            "-x509",
            "-newkey",
            "ec",
            "-pkeyopt",
            "ec_paramgen_curve:prime256v1",
            "-nodes",
            "-keyout",
            "ca.key",
            "-out",
            "ca.pem",
            "-days",
            "1",
            "-subj",
            "/CN=wire-proof-ca",
            "-addext",
            "basicConstraints=critical,CA:TRUE",
        ])?;
        self.openssl(&["x509", "-in", "ca.pem", "-outform", "DER", "-out", "ca.der"])?;
        for name in ["server", "client", "foreign"] {
            self.openssl(&[
                "req",
                "-newkey",
                "ec",
                "-pkeyopt",
                "ec_paramgen_curve:prime256v1",
                "-nodes",
                "-keyout",
                &format!("{name}.key"),
                "-out",
                &format!("{name}.csr"),
                "-subj",
                &format!("/CN={name}"),
            ])?;
            self.write("extensions", if name == "server" {
                b"basicConstraints=CA:FALSE\nsubjectAltName=DNS:localhost\nextendedKeyUsage=serverAuth\n"
            } else {
                b"basicConstraints=CA:FALSE\nextendedKeyUsage=clientAuth\n"
            })?;
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
        for entry in fs::read_dir(&self.root)? {
            fs::set_permissions(entry?.path(), fs::Permissions::from_mode(0o600))?;
        }
        Ok(())
    }

    fn material(&self, name: &str, secret: bool) -> Result<Zeroizing<Vec<u8>>> {
        // This is the gateway binary's own protected file consumer.
        checked(GatewayTls::read_material(&self.path(name), secret))
    }

    fn tls(&self, identity: &str) -> Result<MutualTlsConfig> {
        let mut roots = RootCertStore::empty();
        roots.add(CertificateDer::from(
            self.material("ca.der", false)?.to_vec(),
        ))?;
        checked(MutualTlsConfig::new(
            roots,
            vec![CertificateDer::from(
                self.material(&format!("{identity}.der"), false)?.to_vec(),
            )],
            PrivateKeyDer::try_from(
                self.material(&format!("{identity}-key.der"), true)?
                    .to_vec(),
            )?,
        ))
    }

    fn remote(&self, address: SocketAddr, identity: &str) -> Result<RemoteKmsProvider> {
        Ok(RemoteKmsProvider::new(
            PROVIDER,
            address,
            "localhost",
            self.tls(identity)?,
            Limits {
                maximum_frame_bytes: GATEWAY_FRAME_LIMIT,
                maximum_connections: 4,
                maximum_streams: 1,
                maximum_queued_bytes: GATEWAY_FRAME_LIMIT,
                deadline: WAIT,
            },
            Arc::clone(&self.clock),
        )?)
    }

    fn start(&mut self) -> Result {
        assert!(self.child.is_none());
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-human-kms"));
        command
            .env_clear()
            .env(
                "LAYERX_RUNTIME_CLOCK_SOCKET",
                &self.clock_host.binding.socket,
            )
            .env(
                "LAYERX_RUNTIME_CLOCK_PID",
                self.clock_host.binding.process.to_string(),
            )
            .env(
                "LAYERX_RUNTIME_CLOCK_UID",
                self.clock_host.binding.user.to_string(),
            );
        for (suffix, name) in [
            ("REGISTRY_FILE", "chain.json"),
            ("CLIENT_CA_DER", "ca.der"),
            ("TLS_CERT_DER", "server.der"),
            ("TLS_KEY_DER", "server-key.der"),
            ("CLIENT_CERT_DER", "client.der"),
            ("SEAL_SECRET_FILE", "seal"),
            ("ATTESTOR_CONFIG_FILE", "attestor.json"),
            ("EVM_ASSERTION_FILE", "owner.jwt"),
        ] {
            command.env(format!("LAYERX_HUMAN_KMS_{suffix}"), self.path(name));
        }
        command
            .env("LAYERX_HUMAN_KMS_LISTEN", self.address.to_string())
            .env("LAYERX_HUMAN_KMS_PROVIDER_REFERENCE", PROVIDER)
            .env("LAYERX_HUMAN_KMS_STATE_DIR", self.path("state"))
            .env("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "10")
            .env_remove("LAYERX_HUMAN_KMS_EVM_CLIENT_CERT_DER")
            .stdout(Stdio::null())
            .stderr(fs::File::create(self.path("kms.log"))?);
        self.child = Some(command.spawn().map_err(|error| {
            format!("services/human/crates/layerx-human-kms/src/main.rs producer missing: {error}")
        })?);
        let started = self.clock.sample(WAIT)?;
        while self.remote(self.address, "client")?.probe().is_err() {
            assert!(
                self.child
                    .as_mut()
                    .ok_or("KMS process missing")?
                    .try_wait()?
                    .is_none(),
                "KMS producer exited: {}",
                fs::read_to_string(self.path("kms.log"))?
            );
            assert!(
                self.clock.sample(WAIT)?.elapsed_since(started)? < WAIT,
                "KMS producer startup timed out: {}",
                fs::read_to_string(self.path("kms.log"))?
            );
            thread::sleep(Duration::from_millis(10));
        }
        Ok(())
    }

    fn stop(&mut self) {
        if let Some(mut child) = self.child.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }

    fn call(&self, address: SocketAddr, frame: &[u8]) -> Result<Vec<u8>> {
        let mut roots = RootCertStore::empty();
        roots.add(CertificateDer::from(
            self.material("ca.der", false)?.to_vec(),
        ))?;
        let config = ClientConfig::builder()
            .with_root_certificates(roots)
            .with_client_auth_cert(
                vec![CertificateDer::from(
                    self.material("client.der", false)?.to_vec(),
                )],
                PrivateKeyDer::try_from(self.material("client-key.der", true)?.to_vec())?,
            )?;
        let tcp = TcpStream::connect(address)?;
        tcp.set_read_timeout(Some(WAIT))?;
        tcp.set_write_timeout(Some(WAIT))?;
        let mut tls = StreamOwned::new(
            ClientConnection::new(Arc::new(config), ServerName::try_from("localhost")?)?,
            tcp,
        );
        checked(write_frame(&mut tls, frame, GATEWAY_FRAME_LIMIT))?;
        checked(read_frame(&mut tls, GATEWAY_FRAME_LIMIT))
    }

    fn gateway(&self) -> Result<GatewayHost> {
        let gateway = LxkpGateway::new(
            Arc::new(self.remote(self.address, "client")?),
            GatewayTls::new(
                &self.material("ca.der", false)?,
                &self.material("server.der", false)?,
                &self.material("server-key.der", true)?,
                &self.material("client.der", false)?,
            )?,
            GatewayPolicy {
                network_id: 125,
                protocol_version: 3,
                registry: checked(ModuleRegistry::new(&[]))?,
                limits: GatewayLimits {
                    maximum_connections: 4,
                    maximum_frame_bytes: GATEWAY_FRAME_LIMIT,
                    deadline: WAIT,
                    maximum_operations: 1000,
                    operation_window: Duration::from_secs(60),
                },
            },
            Arc::clone(&self.clock),
        )?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        listener.set_nonblocking(true)?;
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let thread = thread::spawn(move || -> std::result::Result<(), String> {
            while !stopping.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((stream, _)) => {
                        let _ = gateway.serve_connection(stream);
                    }
                    Err(error) if error.kind() == ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1))
                    }
                    Err(error) => return Err(error.to_string()),
                }
            }
            Ok(())
        });
        Ok(GatewayHost {
            address,
            stop,
            thread: Some(thread),
        })
    }
}

// Hosts the production authority; the client still authenticates its Unix peer.
struct ClockHost {
    binding: ClockBinding,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<std::io::Result<()>>>,
}
impl ClockHost {
    fn start(root: &Path) -> Result<Self> {
        let mut server = clock_server::Server::bind(root)?;
        let binding = ClockBinding {
            socket: server.socket().to_owned(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        };
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            while !stopping.load(Ordering::Acquire) {
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
}
impl Drop for ClockHost {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            assert!(
                worker.join().is_ok_and(|result| result.is_ok()),
                "services/platform/hosted/runtime-clock/src/server.rs producer failed"
            );
        }
    }
}

struct GatewayHost {
    address: SocketAddr,
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<std::result::Result<(), String>>>,
}
impl Drop for GatewayHost {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(thread) = self.thread.take() {
            assert!(
                thread.join().is_ok_and(|result| result.is_ok()),
                "real gateway worker failed"
            );
        }
    }
}

fn bank_call(wallet: [u8; 20], recipient: [u8; 20], amount: [u8; 32]) -> Eip1559Call {
    let mut data = ledger_selector("send(address,address,string,uint256)").to_vec();
    for address in [wallet, recipient] {
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
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x01]);
    Eip1559Call {
        chain_id: 125,
        nonce: 0,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to,
        data,
    }
}

#[test]
fn proof_human_kms_wire() -> Result {
    let mut host = Host::new()?;
    fs::set_permissions(
        host.path("client-key.der"),
        fs::Permissions::from_mode(0o640),
    )?;
    assert!(GatewayTls::read_material(&host.path("client-key.der"), true).is_err());
    fs::set_permissions(
        host.path("client-key.der"),
        fs::Permissions::from_mode(0o600),
    )?;
    std::os::unix::fs::symlink(host.path("ca.der"), host.path("linked.der"))?;
    assert!(GatewayTls::read_material(&host.path("linked.der"), false).is_err());
    host.write("empty.der", &[])?;
    assert!(GatewayTls::read_material(&host.path("empty.der"), false).is_err());
    host.write("oversized.der", &vec![1; 65_537])?;
    assert!(GatewayTls::read_material(&host.path("oversized.der"), false).is_err());
    let principal = PrincipalId::new("alice")?;
    let key = KeyId::new("primary")?;
    let gateway = host.gateway()?;
    let relay = host.remote(gateway.address, "client")?;
    relay.probe()?;
    assert!(host.remote(gateway.address, "foreign")?.probe().is_err());
    assert!(host.remote(host.address, "foreign")?.probe().is_err());
    let store = Keystore::open_production(
        host.path("client-state"),
        125,
        host.remote(host.address, "client")?,
    )?;
    let public = store.create(&principal, &key, KeyClass::HumanPrimary)?;
    let (wallet, binding, reference) = store.public_wallet_identity(&principal, &key)?;
    assert_eq!(wallet, host.wallet);
    let direct = host.remote(host.address, "client")?;
    let created = direct.describe_key(&binding, &reference)?;
    assert_eq!(created.public_key(), public);
    assert_eq!(relay.describe_key(&binding, &reference)?, created);
    assert_eq!(direct.create_key(&binding)?, created);
    let other = PrincipalKeyBinding::new(b"bob".to_vec(), 125, KeyClass::HumanPrimary, PROVIDER)?;
    assert_eq!(
        relay.describe_key(&other, &reference),
        Err(KmsError::KeyNotFound)
    );
    assert_eq!(
        direct.describe_key(&binding, &ProviderKeyReference::new(vec![9; 32])?),
        Err(KmsError::Integrity)
    );

    let recipient = [4; 20];
    let mut amount = [0; 32];
    amount[31] = 12;
    let call = bank_call(wallet, recipient, amount);
    let now = host.clock.sample(WAIT)?.unix_seconds();
    let plan = EvmPlanAuthorization {
        plan_id: [1; 32],
        action_key: [2; 32],
        tenant: "tenant".into(),
        principal: "alice".into(),
        binding_digest: binding.digest(),
        wallet,
        not_before: now,
        not_after: now + 600,
        transaction: EvmTransaction {
            chain_id: call.chain_id,
            nonce: call.nonce,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: call.gas_limit,
            to: call.to,
            value: [0; 32],
            calldata: call.data.clone(),
        },
    };
    let authorized = store.authorize_evm_plan(&principal, &key, &plan)?;
    assert_eq!(authorized.authorization, plan);
    assert!(authorized.raw_transaction.is_empty());
    let mut conflicting = plan.clone();
    conflicting.tenant = "other-tenant".into();
    assert!(matches!(
        store.authorize_evm_plan(&principal, &key, &conflicting),
        Err(CustodyError::Kms(KmsError::Conflict))
    ));

    let expected_raw = call.sign(&host.signer)?;
    let (_, signature) = decode_signed(&expected_raw)?;
    let external = EvmExternalSignature {
        action_key: plan.action_key,
        signature: signature.as_bytes().to_vec(),
    };
    let mut foreign = external.clone();
    let foreign_key = k256::ecdsa::SigningKey::from_slice(&[7; 32])?;
    let (signature, recovery) = foreign_key.sign_prehash_recoverable(&call.signing_hash())?;
    foreign.signature = signature.to_bytes().to_vec();
    foreign.signature.push(recovery.to_byte());
    assert_eq!(
        direct.evm_operation(12, &binding, &reference, &serde_json::to_vec(&foreign)?),
        Err(KmsError::Refused)
    );
    let signed: EvmAction = serde_json::from_slice(&direct.evm_operation(
        12,
        &binding,
        &reference,
        &serde_json::to_vec(&external)?,
    )?)?;
    assert_eq!(signed.raw_transaction, expected_raw);
    let raw = store.authorize_send_transaction(
        &principal,
        &key,
        &plan,
        recipient,
        "apax".into(),
        amount,
    )?;
    assert_eq!(raw, expected_raw);
    let (observed, signature) = decode_signed(&raw)?;
    assert_eq!(observed, call);
    assert_eq!(recover_sender(&observed, &signature)?, wallet);
    let mut wrong_amount = amount;
    wrong_amount[31] += 1;
    assert!(matches!(
        store.authorize_send_transaction(
            &principal,
            &key,
            &plan,
            recipient,
            "apax".into(),
            wrong_amount
        ),
        Err(CustodyError::Kms(KmsError::Refused))
    ));

    // Each inventory contract uses bytes emitted by the production encoder.
    // Persist and reread them before invoking the real wire decoder, KMS TLS
    // server, and gateway consumer. No valid frame is hand assembled.
    let send_payload = serde_json::to_vec(&json!({"authorization":plan,"disclosure":{
        "recipient":recipient,"denom":"apax","amount":amount }}))?;
    for (operation, payload, kind) in [
        (6, Vec::new(), wire::Operation::EvmWallet),
        (7, serde_json::to_vec(&plan)?, wire::Operation::Evm),
        (
            8,
            serde_json::to_vec(&plan.action_key)?,
            wire::Operation::Evm,
        ),
        (
            9,
            serde_json::to_vec(&EvmAcknowledgement {
                action_key: plan.action_key,
                transaction_hash: signed.transaction_hash.ok_or("missing signed hash")?,
            })?,
            wire::Operation::Evm,
        ),
        (
            10,
            serde_json::to_vec(&plan.action_key)?,
            wire::Operation::Evm,
        ),
        (11, send_payload, wire::Operation::AuthorizeSend),
        (12, serde_json::to_vec(&external)?, wire::Operation::Evm),
    ] {
        let frame = direct.encode_evm_request(operation, &binding, &reference, &payload)?;
        host.write("request.lxkp", &frame)?;
        if operation == 10 {
            host.write("recover.lxkp", &frame)?;
        }
        let transported = fs::read(host.path("request.lxkp"))?;
        let decoded = checked(wire::Request::decode(&transported))?;
        assert_eq!(decoded.version, 3);
        assert_eq!(decoded.operation, operation);
        assert_eq!(decoded.kind, kind);
        assert_eq!(decoded.binding, binding.digest());
        assert_eq!(decoded.reference, reference.as_bytes());
        assert_eq!(decoded.evm, payload);
        assert_eq!(&transported[..4], b"LXKP");
        let response = host.call(host.address, &transported)?;
        assert_eq!(&response[..7], &transported[..7]);
        assert_eq!(response[7], 0);
        match operation {
            6 => assert_eq!(&response[8..], wallet),
            11 => assert_eq!(&response[8..], expected_raw),
            _ => {
                let action: EvmAction = serde_json::from_slice(&response[8..])?;
                assert_eq!(action.authorization, plan);
                assert_eq!(action.raw_transaction, expected_raw);
            }
        }
        assert_eq!(
            relay.evm_operation(operation, &binding, &reference, &payload),
            Err(KmsError::Refused)
        );
        let mut trailing = transported.clone();
        trailing.push(0);
        assert!(matches!(
            wire::Request::decode(&trailing),
            Err(wire::Error::Refused)
        ));
        assert!(host.call(host.address, &trailing).is_err());
        assert!(matches!(
            wire::Request::decode(&transported[..transported.len() - 1]),
            Err(wire::Error::Refused)
        ));
        if operation != 6 {
            let mut empty = transported[..transported.len() - payload.len() - 4].to_vec();
            empty.extend(0_u32.to_be_bytes());
            assert!(matches!(
                wire::Request::decode(&empty),
                Err(wire::Error::Refused)
            ));
            assert!(host.call(host.address, &empty).is_err());
        }
    }
    assert_eq!(wire::MAX_FRAME, GATEWAY_FRAME_LIMIT);
    assert!(matches!(
        wire::Request::decode(&vec![0; wire::MAX_FRAME + 1]),
        Err(wire::Error::Refused)
    ));
    assert_eq!(
        direct.encode_evm_request(7, &binding, &reference, &[]),
        Err(KmsError::Refused)
    );
    assert_eq!(
        direct.encode_evm_request(6, &binding, &reference, b"{}"),
        Err(KmsError::Refused)
    );
    assert_eq!(
        direct.encode_evm_request(255, &binding, &reference, b"{}"),
        Err(KmsError::Refused)
    );
    let overhead = direct
        .encode_evm_request(7, &binding, &reference, b"{}")?
        .len()
        - 2;
    let mut bounded_payload = vec![b' '; wire::MAX_FRAME - overhead];
    bounded_payload[..2].copy_from_slice(b"{}");
    let maximum = direct.encode_evm_request(7, &binding, &reference, &bounded_payload)?;
    assert_eq!(maximum.len(), wire::MAX_FRAME);
    assert_eq!(
        checked(wire::Request::decode(&maximum))?.evm,
        bounded_payload
    );
    assert_eq!(
        direct.evm_operation(7, &binding, &reference, &bounded_payload),
        Err(KmsError::Refused)
    );
    bounded_payload.push(b' ');
    assert_eq!(
        direct.encode_evm_request(7, &binding, &reference, &bounded_payload),
        Err(KmsError::InvalidConfiguration)
    );
    assert_eq!(
        direct.evm_operation(7, &binding, &reference, b"{"),
        Err(KmsError::Refused)
    );
    assert_eq!(
        direct.evm_operation(10, &binding, &reference, &serde_json::to_vec(&[8_u8; 32])?),
        Err(KmsError::KeyNotFound)
    );

    let persisted = fs::read(host.path("state/state.aead"))?;
    assert!(persisted.len() > 28);
    drop(store);
    host.stop();
    assert_eq!(fs::read(host.path("state/state.aead"))?, persisted);
    host.start()?;
    let store = Keystore::open_production(
        host.path("client-state"),
        125,
        host.remote(host.address, "client")?,
    )?;
    assert_eq!(store.describe(&principal, &key)?.public_key, public);
    assert_eq!(store.evm_wallet(&principal, &key)?, wallet);
    let recovered = store.recover_evm_action(&principal, &key, &plan.action_key)?;
    assert_eq!(recovered.authorization, plan);
    assert_eq!(recovered.raw_transaction, expected_raw);
    assert!(recovered.acknowledged);
    let recovery_response = host.call(host.address, &fs::read(host.path("recover.lxkp"))?)?;
    assert_eq!(recovery_response[7], 0);
    assert_eq!(
        serde_json::from_slice::<EvmAction>(&recovery_response[8..])?,
        recovered
    );
    assert_eq!(
        store.authorize_evm_plan(&principal, &key, &plan)?,
        recovered
    );
    assert_eq!(
        store.sign_evm_action(&principal, &key, &plan.action_key)?,
        recovered
    );
    assert_eq!(
        store.authorize_send_transaction(
            &principal,
            &key,
            &plan,
            recipient,
            "apax".into(),
            amount
        )?,
        expected_raw
    );
    assert_eq!(direct.create_key(&binding)?, created);
    assert!(store
        .create(&principal, &key, KeyClass::HumanPrimary)
        .is_err());
    assert_eq!(relay.describe_key(&binding, &reference)?, created);

    // Handover must keep its specific refusal across the encrypted-file restart.
    drop(direct.export_primary_key(&binding, &reference)?);
    assert_eq!(direct.evm_operation(11,&binding,&reference,&serde_json::to_vec(&json!({
        "authorization":plan,"disclosure":{"recipient":recipient,"denom":"apax","amount":amount}
    }))?),Err(KmsError::SelfCustodied));
    for operation in [6, 7, 8, 9, 10, 12] {
        let payload = if operation == 6 {
            Vec::new()
        } else {
            serde_json::to_vec(&plan.action_key)?
        };
        assert_eq!(
            direct.evm_operation(operation, &binding, &reference, &payload),
            Err(KmsError::SelfCustodied)
        );
    }
    host.stop();
    host.start()?;
    assert_eq!(
        direct.evm_operation(6, &binding, &reference, &[]),
        Err(KmsError::SelfCustodied)
    );
    assert_eq!(
        direct.export_primary_key(&binding, &reference).err(),
        Some(KmsError::SelfCustodied)
    );
    assert_eq!(direct.create_key(&binding), Err(KmsError::Conflict));
    let agent = PrincipalKeyBinding::new(b"agent".to_vec(), 125, KeyClass::AgentPrimary, PROVIDER)?;
    let disposable = direct.create_key(&agent)?;
    direct.destroy_key(&agent, disposable.reference())?;
    assert_eq!(
        direct.evm_operation(6, &agent, disposable.reference(), &[]),
        Err(KmsError::KeyNotFound)
    );
    for node in &host.unused_threshold {
        assert_eq!(
            node.accept().err().map(|error| error.kind()),
            Some(ErrorKind::WouldBlock),
            "external signature replay unexpectedly contacted a threshold service"
        );
    }
    Ok(())
}
