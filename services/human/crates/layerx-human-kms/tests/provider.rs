use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_human_kms::export::{ExportAcknowledgement, ExportCeremony, ExportRecipient};
use layerx_human_service::custody::{
    KeyClass, KeyId, Keystore, KmsError, KmsProvider, PrincipalKeyBinding, ProviderKeyReference,
    RemoteKmsProvider,
};
use layerx_human_service::frame::{read_frame, write_frame, Limits, MutualTlsConfig};
use layerx_human_service::store::PrincipalId;
use layerx_types::clock::{Clock, Deadline};
use ring::signature::KeyPair;
use rustls::pki_types::{CertificateDer, PrivateKeyDer, ServerName};
use rustls::{ClientConfig, ClientConnection, RootCertStore, StreamOwned};
use std::error::Error;
use std::fs;
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use attestor_cluster::runtime_clock_server;

struct Runtime {
    stop: Arc<AtomicBool>,
    worker: Option<std::thread::JoinHandle<std::io::Result<()>>>,
}

impl Drop for Runtime {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

type Result<T> = std::result::Result<T, Box<dyn Error>>;
#[allow(dead_code)]
#[path = "support/attestor_cluster.rs"]
mod attestor_cluster;
#[path = "provider/native_setup.rs"]
mod native_setup;
#[path = "support/principal_attestors.rs"]
mod principal_attestors;
const MAX: usize = 2_097_152;
static ATTESTOR_FIXTURE: Mutex<()> = Mutex::new(());
struct Host {
    root: PathBuf,
    address: SocketAddr,
    child: Option<Child>,
    environment: Vec<(&'static str, PathBuf)>,
    _attestors: attestor_cluster::Cluster,
    runtime: Option<Runtime>,
    binding: ClockBinding,
    clock: Arc<RuntimeClock>,
}
impl Drop for Host {
    fn drop(&mut self) {
        self.stop();
        drop(self.runtime.take());
        let _ = fs::remove_dir_all(&self.root);
    }
}
impl Host {
    fn new() -> Result<Self> {
        Self::for_bindings(&[])
    }
    fn for_bindings(bindings: &[u8]) -> Result<Self> {
        let _fixture = ATTESTOR_FIXTURE
            .lock()
            .map_err(|_| "attestor fixture lock poisoned")?;
        let _ = rustls::crypto::ring::default_provider().install_default();
        let mut nonce = [0; 8];
        getrandom::fill(&mut nonce).map_err(|error| std::io::Error::other(error.to_string()))?;
        let root = std::env::temp_dir().join(format!(
            "lxkp-{}-{:x}",
            std::process::id(),
            u64::from_be_bytes(nonce)
        ));
        fs::create_dir(&root)?;
        fs::set_permissions(&root, fs::Permissions::from_mode(0o700))?;
        let mut clock_server = runtime_clock_server::Server::bind(&root)?;
        let binding = ClockBinding {
            socket: clock_server.socket().to_path_buf(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        };
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let runtime = Runtime {
            stop,
            worker: Some(std::thread::spawn(move || {
                while !stopping.load(Ordering::Acquire) {
                    clock_server.poll()?;
                    std::thread::sleep(Duration::from_millis(1));
                }
                Ok(())
            })),
        };
        let clock = RuntimeClock::connect(binding.clone())?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let mut attestors = attestor_cluster::Cluster::start_with_policies(
            &[],
            &serde_json::json!({"version":1,"defaults":{
                "chain_id":125,"kinds":["lx_activity","evm_tx"],
                "selectors":{"0x000000000000000000000000000000000000101a":[format!("0x{}", attestor_cluster::hex(&layerx_crypto::disclosure::call::ledger_selector("openAccount(bytes32)")))]},
                "caps":{"native":{"per_transaction":"1000000000000000000","daily":"10000000000000000000"}},"rate_per_minute":1000
            }}),
            &serde_json::json!({"version":1,"defaults":{"modules":{}}}),
        )?;
        principal_attestors::provision(&mut attestors, &root, "beta-kms", bindings)?;
        let environment = [
            ("REGISTRY_FILE", "registry.json"),
            ("CLIENT_CA_DER", "ca.der"),
            ("TLS_CERT_DER", "server.der"),
            ("TLS_KEY_DER", "server-key.der"),
            ("CLIENT_CERT_DER", "client.der"),
            ("SEAL_SECRET_FILE", "seal"),
            ("ATTESTOR_CONFIG_FILE", "attestor.json"),
        ]
        .into_iter()
        .map(|(suffix, file)| (suffix, root.join(file)))
        .collect();
        let mut host = Self {
            root,
            address,
            child: None,
            environment,
            _attestors: attestors,
            runtime: Some(runtime),
            binding,
            clock,
        };
        host.provision_tls()?;
        let mut seal = [0; 32];
        getrandom::fill(&mut seal).map_err(|error| std::io::Error::other(error.to_string()))?;
        fs::write(host.root.join("seal"), seal)?;
        fs::write(
            host.root.join("registry.json"),
            serde_json::to_vec(&serde_json::json!({"chain_id":125}))?,
        )?;
        for entry in fs::read_dir(&host.root)? {
            let entry = entry?;
            if entry.file_type()?.is_file() {
                fs::set_permissions(entry.path(), fs::Permissions::from_mode(0o600))?;
            }
        }
        drop(listener);
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
        for name in ["server", "client", "foreign"] {
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
            .env("LAYERX_RUNTIME_CLOCK_SOCKET", &self.binding.socket)
            .env("LAYERX_RUNTIME_CLOCK_PID", self.binding.process.to_string())
            .env("LAYERX_RUNTIME_CLOCK_UID", self.binding.user.to_string())
            .env("LAYERX_HUMAN_KMS_LISTEN", self.address.to_string())
            .env("LAYERX_HUMAN_KMS_PROVIDER_REFERENCE", "beta-kms")
            .env("LAYERX_HUMAN_KMS_STATE_DIR", self.root.join("state"))
            .env("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "30")
            .env(
                "LAYERX_HUMAN_KMS_EVM_CLIENT_CERT_DER",
                self.root.join("foreign.der"),
            );
        for (suffix, path) in &self.environment {
            command.env(format!("LAYERX_HUMAN_KMS_{suffix}"), path);
        }
        Ok(command
            .stdout(Stdio::null())
            .stderr(fs::File::create(self.root.join("kms.log"))?)
            .spawn()?)
    }
    fn start(&mut self) -> Result<()> {
        self.child = Some(self.launch()?);
        let clock = Arc::clone(&self.clock);
        let mut deadline = Deadline::start(clock.as_ref(), Duration::from_secs(10))?;
        loop {
            if self.remote("client", "beta-kms")?.probe().is_ok() {
                return Ok(());
            }
            if self
                .child
                .as_mut()
                .ok_or("missing process")?
                .try_wait()?
                .is_some()
            {
                return Err(format!(
                    "KMS exited at startup: {}",
                    fs::read_to_string(self.root.join("kms.log"))?
                )
                .into());
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
    fn remote(&self, name: &str, provider: &str) -> Result<RemoteKmsProvider> {
        let (cert, key) = self.identity(name)?;
        Ok(RemoteKmsProvider::new(
            provider,
            self.address,
            "localhost",
            checked(MutualTlsConfig::new(self.roots()?, cert, key))?,
            Limits {
                maximum_frame_bytes: MAX,
                maximum_connections: 4,
                maximum_streams: 1,
                maximum_queued_bytes: MAX,
                deadline: Duration::from_secs(30),
            },
            self.clock.clone(),
        )?)
    }
    fn connection(&self, name: Option<&str>) -> Result<StreamOwned<ClientConnection, TcpStream>> {
        let builder = ClientConfig::builder().with_root_certificates(self.roots()?);
        let config = if let Some(name) = name {
            let (cert, key) = self.identity(name)?;
            builder.with_client_auth_cert(cert, key)?
        } else {
            builder.with_no_client_auth()
        };
        let tcp = TcpStream::connect(self.address)?;
        tcp.set_read_timeout(Some(Duration::from_secs(30)))?;
        tcp.set_write_timeout(Some(Duration::from_secs(30)))?;
        Ok(StreamOwned::new(
            ClientConnection::new(Arc::new(config), ServerName::try_from("localhost")?)?,
            tcp,
        ))
    }
    fn call(&self, request: &[u8]) -> Result<Vec<u8>> {
        let mut tls = self.connection(Some("client"))?;
        checked(write_frame(&mut tls, request, MAX))?;
        checked(read_frame(&mut tls, MAX))
    }
    fn sender(&self, binding: [u8; 32], handle: &[u8]) -> Result<[u8; 20]> {
        use layerx_human_service::custody::{PrincipalKeyBinding, ProviderKeyReference};
        let remote = self.remote("client", "beta-kms")?;
        let binding = PrincipalKeyBinding::from_digest(binding, 125, KeyClass::HumanPrimary)?;
        let reference = ProviderKeyReference::new(handle.to_vec())?;
        let described = remote.describe_key(&binding, &reference)?;
        let descriptor = remote.wallet_descriptor(&binding, &reference)?;
        assert_eq!(descriptor.native_ed25519, described.public_key());
        assert_ne!(descriptor.evm_secp256k1, [0; 20]);
        Ok(descriptor.evm_secp256k1)
    }
}
fn blob(out: &mut Vec<u8>, value: &[u8]) -> Result<()> {
    out.extend(u32::try_from(value.len())?.to_be_bytes());
    out.extend(value);
    Ok(())
}
fn request(
    op: u8,
    binding: [u8; 32],
    reference: &[u8],
    expected: Option<[u8; 32]>,
) -> Result<Vec<u8>> {
    let mut out = b"LXKP".to_vec();
    out.extend(if expected.is_some() { 2_u16 } else { 1_u16 }.to_be_bytes());
    out.push(op);
    blob(&mut out, b"beta-kms")?;
    if op != 0 {
        out.extend(binding);
        out.extend(125_u32.to_be_bytes());
        out.push(1);
        blob(&mut out, reference)?;
    }
    if let Some(expected) = expected {
        out.extend(expected);
    }
    Ok(out)
}
fn facts(bytes: &[u8]) -> Result<([u8; 32], [u8; 32])> {
    assert_eq!(&bytes[..4], b"LXKP");
    assert_eq!(bytes[7], 0);
    assert_eq!(bytes.len(), 109);
    assert_eq!(&bytes[8..12], &32_u32.to_be_bytes());
    Ok((bytes[12..44].try_into()?, bytes[44..76].try_into()?))
}

#[test]
fn actual_client_lifecycle_and_mutual_tls() -> Result<()> {
    let mut host = Host::new()?;
    let alice = PrincipalId::new("alice")?;
    let bob = PrincipalId::new("bob")?;
    let key = KeyId::new("primary")?;
    let store = Keystore::open_production(
        host.root.join("client-state"),
        125,
        host.remote("client", "beta-kms")?,
    )?;
    let original = store.create(&alice, &key, KeyClass::HumanPrimary)?;
    assert_eq!(store.describe(&alice, &key)?.public_key, original);
    assert!(store.describe(&bob, &key).is_err());
    let bob_public = store.create(&bob, &key, KeyClass::AgentPrimary)?;
    assert_ne!(original, bob_public);
    assert_eq!(store.describe(&bob, &key)?.public_key, bob_public);
    let rotated = store.rotate(&alice, &key)?.public_key;
    assert_ne!(original, rotated);
    let next = store.rotate(&alice, &key)?.public_key;
    assert_ne!(rotated, next);
    host.stop();
    host.start()?;
    assert_eq!(store.describe(&alice, &key)?.public_key, next);
    assert!(host.remote("foreign", "beta-kms")?.probe().is_err());
    assert!(host
        .remote("client", "different-provider")?
        .probe()
        .is_err());
    let mut unauthenticated = host.connection(None)?;
    let probe = request(0, [0; 32], &[], None)?;
    let no_cert = write_frame(&mut unauthenticated, &probe, MAX)
        .and_then(|()| read_frame(&mut unauthenticated, MAX));
    assert!(no_cert.is_err());
    store.destroy(&alice, &key)?;
    assert_eq!(store.describe(&bob, &key)?.public_key, bob_public);
    store.destroy(&bob, &key)?;
    assert!(store.describe(&alice, &key).is_err());
    assert!(store.create(&alice, &key, KeyClass::HumanPrimary).is_err());
    Ok(())
}

#[test]
fn atomic_rotation_lost_response_restart_and_tombstones() -> Result<()> {
    let mut host = Host::new()?;
    let binding = [31; 32];
    let create = request(1, binding, &[], None)?;
    let first = host.call(&create)?;
    assert_eq!(first, host.call(&create)?);
    let (handle, original) = facts(&first)?;
    let rotate = request(3, binding, &handle, Some(original))?;
    {
        let mut tls = host.connection(Some("client"))?;
        checked(write_frame(&mut tls, &rotate, MAX))?;
    }
    let clock = Arc::clone(&host.clock);
    let mut deadline = Deadline::start(clock.as_ref(), Duration::from_secs(5))?;
    loop {
        let (_, observed) = facts(&host.call(&request(2, binding, &handle, None)?)?)?;
        if observed != original {
            break;
        }
        if deadline.remaining(clock.as_ref())?.is_zero() {
            return Err("lost-response rotation was not committed".into());
        }
        std::thread::sleep(Duration::from_millis(20));
    }
    let committed = host.call(&rotate)?;
    let (same, next) = facts(&committed)?;
    assert_eq!(same, handle);
    assert_ne!(original, next);
    host.stop();
    host.start()?;
    assert_eq!(committed, host.call(&rotate)?);
    let second = host.call(&request(3, binding, &handle, Some(next))?)?;
    let (_, latest) = facts(&second)?;
    assert_ne!(latest, next);
    assert_eq!(host.call(&rotate)?[7], 3);
    assert_eq!(host.call(&request(3, binding, &handle, None)?)?[7], 1);
    assert_eq!(host.call(&request(2, [32; 32], &handle, None)?)?[7], 2);
    assert_eq!(host.call(&request(2, binding, &[33; 32], None)?)?[7], 5);
    let mut wrong_class = request(2, binding, &handle, None)?;
    wrong_class[55] = 2;
    assert_eq!(host.call(&wrong_class)?[7], 5);
    let mut wrong_network = request(2, binding, &handle, None)?;
    wrong_network[54] ^= 1;
    assert_eq!(host.call(&wrong_network)?[7], 1);
    let destroy = request(4, binding, &handle, None)?;
    assert_eq!(host.call(&destroy)?[7], 0);
    assert_eq!(host.call(&destroy)?[7], 0);
    host.stop();
    host.start()?;
    assert_eq!(host.call(&destroy)?[7], 0);
    assert_eq!(host.call(&create)?[7], 3);
    assert_eq!(host.call(&request(2, binding, &handle, None)?)?[7], 2);
    host.stop();
    let path = host.root.join("state/state.aead");
    let mut encrypted = fs::read(&path)?;
    encrypted[20] ^= 1;
    fs::write(path, encrypted)?;
    let mut child = host.launch()?;
    let clock = Arc::clone(&host.clock);
    let mut deadline = Deadline::start(clock.as_ref(), Duration::from_secs(5))?;
    loop {
        if let Some(status) = child.try_wait()? {
            assert!(!status.success());
            break;
        }
        if deadline.remaining(clock.as_ref())?.is_zero() {
            child.kill()?;
            child.wait()?;
            return Err("tampered state did not fail closed".into());
        }
        std::thread::sleep(Duration::from_millis(20));
    }
    Ok(())
}

fn registry() -> Result<layerx_types::payload::ModuleRegistry> {
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
    checked(ModuleRegistry::new(&[
        checked(ModuleRegistration::new(
            ModuleId::Asset,
            &[
                checked(ActivityType::new(ModuleId::Asset, 1))?,
                checked(ActivityType::new(ModuleId::Asset, 4))?,
                checked(ActivityType::new(ModuleId::Asset, 5))?,
                checked(ActivityType::new(ModuleId::Asset, 7))?,
            ],
        ))?,
        checked(ModuleRegistration::new(
            ModuleId::Budget,
            &[checked(ActivityType::new(ModuleId::Budget, 1))?],
        ))?,
        checked(ModuleRegistration::new(
            ModuleId::Governance,
            &[
                checked(ActivityType::new(ModuleId::Governance, 1))?,
                checked(ActivityType::new(ModuleId::Governance, 2))?,
                checked(ActivityType::new(ModuleId::Governance, 3))?,
                checked(ActivityType::new(ModuleId::Governance, 5))?,
                checked(ActivityType::new(ModuleId::Governance, 8))?,
            ],
        ))?,
    ]))
}

fn canonical_send(
    network: u32,
    options: &layerx_crypto::send::CallOptions,
) -> Result<layerx_crypto::send::DisclosedCall> {
    use layerx_types::account::AccountId;
    use layerx_types::amount::Amount;
    use layerx_types::ids::{AssetId, IdempotencyKey};
    use layerx_types::intent::{
        AuthorizationSignature, ContextHash, NetworkId, ProtocolVersion, PublicKey,
        SendAuthorization, SendAuthorizationKind, Sequence, TimestampSeconds,
    };
    use ring::signature::KeyPair as _;
    let registry = registry()?;
    let key = checked(ring::signature::Ed25519KeyPair::from_seed_unchecked(
        &[61; 32],
    ))?;
    let public = key.public_key().as_ref().try_into()?;
    let from = AccountId::parse("agent:did:layerx:alice:main")
        .map_err(|error| format!("canonical_send: SendDebit source AccountId: {error:?}"))?;
    let to = AccountId::parse("agent:did:layerx:recipient:main")
        .map_err(|error| format!("canonical_send: SendDebit recipient AccountId: {error:?}"))?;
    let debit = layerx_crypto::send::SendDebit {
        from: checked(layerx_intents::canonical::account_id_for_protocol(&from, 3))?,
        to: checked(layerx_intents::canonical::account_id_for_protocol(&to, 3))?,
        asset: [3; 32],
        amount: 10,
        source_sequence: 7,
        idempotency_key: [4; 32],
        expires_at: 1010,
        context_hash: [5; 32],
        conditions: Vec::new(),
        authorization_kind: SendAuthorizationKind::Owner as u8,
        network_id: network,
        protocol_version: 3,
    };
    let authorization = debit
        .authorization_message()
        .map_err(|error| format!("canonical_send: authorization_message: {error:?}"))?;
    let message = checked(layerx_crypto::SignatureMessage::new(
        layerx_intents::canonical::Domain::SignaturePreimage,
        3,
        network,
        &authorization,
    ))?;
    let signature = key.sign(&message.digest()).as_ref().try_into()?;
    let send = checked(layerx_intents::LxpSend::new(
        from,
        to,
        AssetId::new([3; 32]),
        Amount::from_u128(10),
        Sequence::from_u64(7),
        IdempotencyKey::new([4; 32]),
        TimestampSeconds::from_u64(1010),
        ContextHash::new([5; 32]),
        SendAuthorization::new(
            SendAuthorizationKind::Owner,
            PublicKey::new(public),
            AuthorizationSignature::new(signature),
        ),
        checked(NetworkId::new(network))?,
        checked(ProtocolVersion::new(3))?,
    ))?;
    let compiled = checked(layerx_intents::compile(
        &layerx_intents::Intent::v1(layerx_intents::IntentKind::LxpSend(send)),
        &registry,
    ))?;
    let signed = debit
        .encode_signed(public, signature)
        .map_err(|error| format!("canonical_send: signed SendDebit payload: {error:?}"))?;
    assert_eq!(compiled.payload().as_bytes(), signed);
    let disclosed =
        layerx_crypto::send::ledger_payment_call(&signed, options).map_err(|error| {
            format!("canonical_send: ledger payment DisclosedCall build: {error:?}")
        })?;
    let rebound = layerx_crypto::disclosure::call::bind_call(&disclosed.call, options.from)
        .map_err(|error| format!("canonical_send: bind_call of ledger payment: {error:?}"))?;
    assert_eq!(disclosed.disclosure, rebound);
    Ok(disclosed)
}

fn encoded_disclosure(disclosure: &layerx_crypto::disclosure::Disclosure) -> Result<Vec<u8>> {
    let fee_grant = disclosure
        .authority_grant
        .filter(|grant| grant.fee_budget.is_some());
    let mut out = vec![
        if disclosure.onboarding.is_some() || disclosure.native_operation.is_some() {
            4
        } else if disclosure.session_grant.is_some() {
            3
        } else if fee_grant.is_some() {
            2
        } else {
            1
        },
    ];
    out.extend(disclosure.activity_type.value().to_be_bytes());
    blob(&mut out, &disclosure.actor)?;
    blob(&mut out, &disclosure.authority)?;
    out.extend(u32::try_from(disclosure.counterparties.len())?.to_be_bytes());
    for party in &disclosure.counterparties {
        out.push(match party.role {
            layerx_crypto::disclosure::CounterpartyRole::Payer => 1,
            layerx_crypto::disclosure::CounterpartyRole::Recipient => 2,
        });
        out.extend(party.account);
    }
    out.extend(u32::try_from(disclosure.amounts.len())?.to_be_bytes());
    for amount in &disclosure.amounts {
        out.push(match amount.role {
            layerx_crypto::disclosure::AmountRole::Transfer => 1,
            layerx_crypto::disclosure::AmountRole::SpendingLimit => 2,
            layerx_crypto::disclosure::AmountRole::SupplyCap => 3,
            layerx_crypto::disclosure::AmountRole::PerDrawMaximum => 4,
            layerx_crypto::disclosure::AmountRole::GrantAllowance => 5,
        });
        out.extend(amount.value.to_be_bytes());
    }
    out.extend(disclosure.asset);
    out.extend(disclosure.fee_limit.to_be_bytes());
    out.extend(disclosure.expiry.not_before.to_be_bytes());
    out.extend(disclosure.expiry.not_after.to_be_bytes());
    out.extend(disclosure.expiry.payload_expires_at.to_be_bytes());
    out.extend(disclosure.idempotency_key);
    assert!(disclosure.evm_payout_binding.is_none());
    out.push(0);
    if let Some(grant) = fee_grant {
        blob(&mut out, &checked(grant.encode())?)?;
    }
    if let Some(session) = &disclosure.session_grant {
        blob(&mut out, &session.grant.registration_payload)?;
        out.extend(session.expiry_sequence.to_be_bytes());
        out.extend(session.action_key);
        if let Some(replacement) = session.replacement {
            out.push(1);
            out.extend(replacement.predecessor_grant_id);
            out.extend(replacement.expected_charge_state);
        } else {
            out.push(0);
        }
    }
    if let Some(onboarding) = &disclosure.onboarding {
        blob(&mut out, &checked(onboarding.encode())?)?;
    }
    if let Some(operation) = &disclosure.native_operation {
        blob(&mut out, &checked(operation.encode())?)?;
    }
    Ok(out)
}
fn checked<T, E: std::fmt::Debug>(value: std::result::Result<T, E>) -> Result<T> {
    value.map_err(|error| format!("{error:?}").into())
}
fn verify_evm_signature(
    host: &Host,
    binding: [u8; 32],
    handle: &[u8],
    digest: &[u8; 32],
    bytes: &[u8],
) -> Result<()> {
    assert_eq!(bytes.len(), 65);
    let signature = k256::ecdsa::Signature::from_slice(&bytes[..64])?;
    let recovery =
        k256::ecdsa::RecoveryId::try_from(bytes[64].checked_sub(27).ok_or("invalid recovery id")?)?;
    let key = k256::ecdsa::VerifyingKey::recover_from_prehash(digest, &signature, recovery)?;
    assert_eq!(
        checked(layerx_crypto::secp256k1::evm_address(
            key.to_encoded_point(false).as_bytes()
        ))?,
        host.sender(binding, handle)?
    );
    Ok(())
}

fn authorize_canonical_send(
    host: &Host,
    binding: [u8; 32],
    handle: &[u8],
) -> Result<(layerx_crypto::evm_transaction::Eip1559Call, [u8; 20])> {
    use layerx_crypto::disclosure::call::ledger_selector;
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
    use layerx_human_service::custody::{EvmPlanAuthorization, EvmTransaction};
    let mut wallet_request = request(6, binding, handle, None)?;
    wallet_request[4..6].copy_from_slice(&3_u16.to_be_bytes());
    let wallet_response = host.call(&wallet_request)?;
    assert_eq!(wallet_response[7], 0);
    let wallet: [u8; 20] = wallet_response[8..].try_into()?;
    let recipient = [0x44; 20];
    let denom = "apax";
    let mut amount = [0; 32];
    amount[31] = 10;
    let mut bank = [0; 20];
    bank[18..].copy_from_slice(&[0x10, 0x01]);
    let mut calldata = ledger_selector("send(address,address,string,uint256)").to_vec();
    for address in [wallet, recipient] {
        calldata.extend([0; 12]);
        calldata.extend(address);
    }
    let mut offset = [0; 32];
    offset[31] = 128;
    calldata.extend(offset);
    calldata.extend(amount);
    let mut length = [0; 32];
    length[31] = u8::try_from(denom.len())?;
    calldata.extend(length);
    calldata.extend(denom.as_bytes());
    calldata.extend([0; 28]);
    let now = host.clock.sample(Duration::from_secs(1))?.unix_seconds();
    let authorization = EvmPlanAuthorization {
        plan_id: [61; 32],
        action_key: [62; 32],
        principal: "alice".into(),
        tenant: "tenant".into(),
        binding_digest: binding,
        wallet,
        not_before: now,
        not_after: now + 600,
        transaction: EvmTransaction {
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 100_000,
            to: bank,
            value: [0; 32],
            calldata: calldata.clone(),
        },
    };
    let authorization = serde_json::json!({
        "authorization": authorization,
        "disclosure": {"recipient": recipient, "denom": denom, "amount": amount},
    });
    let mut bytes = request(11, binding, handle, None)?;
    bytes[4..6].copy_from_slice(&3_u16.to_be_bytes());
    blob(&mut bytes, &serde_json::to_vec(&authorization)?)?;
    let response = host.call(&bytes)?;
    assert_eq!(response[7], 0);
    let (call, signature) = decode_signed(&response[8..])?;
    let expected = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: bank,
        data: calldata,
    };
    assert_eq!(call, expected);
    assert_eq!(call.signing_hash(), expected.signing_hash());
    assert_eq!(recover_sender(&call, &signature)?, wallet);
    Ok((call, wallet))
}

fn evm_signing_request(
    host: &Host,
    binding: [u8; 32],
    handle: &[u8],
    canonical: &[u8],
    disclosure: &[u8],
) -> Result<(Vec<u8>, [u8; 32])> {
    use sha3::{Digest, Keccak256};
    let digest: [u8; 32] = Keccak256::digest(canonical).into();
    let mut bytes = request(5, binding, handle, None)?;
    bytes[4..6].copy_from_slice(&7_u16.to_be_bytes());
    bytes.extend(digest);
    bytes.extend(host.sender(binding, handle)?);
    blob(&mut bytes, canonical)?;
    blob(&mut bytes, disclosure)?;
    Ok((bytes, digest))
}

fn construct_signing_call(
    call: &layerx_crypto::evm_transaction::Eip1559Call,
    from: [u8; 20],
) -> Result<(Vec<u8>, layerx_crypto::disclosure::Disclosure)> {
    use sha3::{Digest, Keccak256};
    let disclosure = checked(layerx_crypto::disclosure::call::bind_call(call, from))?;
    let canonical = layerx_human_kms::attestor::unsigned_evm_transaction(call);
    assert_eq!(
        <[u8; 32]>::from(Keccak256::digest(&canonical)),
        call.signing_hash()
    );
    Ok((canonical, disclosure))
}

#[test]
fn canonical_signing_and_disclosure_refusals() -> Result<()> {
    use layerx_crypto::send::{ledger_payment_call, CallOptions};
    use std::io::Write;
    let host = Host::for_bindings(&[41])?;
    let binding = [41; 32];
    let (handle, _) = facts(&host.call(&request(1, binding, &[], None)?)?)?;
    let (call, wallet) = authorize_canonical_send(&host, binding, &handle)?;
    let canonical = layerx_human_kms::attestor::unsigned_evm_transaction(&call);
    let disclosure = encoded_disclosure(&checked(layerx_crypto::disclosure::call::bind_call(
        &call, wallet,
    ))?)?;
    let mut forged_call = call.clone();
    forged_call.data = b"not a ledger payment call".to_vec();
    assert!(matches!(
        ledger_payment_call(
            &forged_call.data,
            &CallOptions {
                from: wallet,
                chain_id: call.chain_id,
                nonce: call.nonce,
                max_priority_fee_per_gas: call.max_priority_fee_per_gas,
                max_fee_per_gas: call.max_fee_per_gas,
                gas_limit: call.gas_limit,
            },
        ),
        Err(layerx_crypto::disclosure::DisclosureError::MalformedPayload)
    ));
    let forged = layerx_human_kms::attestor::unsigned_evm_transaction(&forged_call);
    assert_eq!(
        host.call(&evm_signing_request(&host, binding, &handle, &forged, &disclosure)?.0)?[7],
        1
    );
    let (bytes, digest) = evm_signing_request(&host, binding, &handle, &canonical, &disclosure)?;
    assert_eq!(digest, call.signing_hash());
    let response = host.call(&bytes)?;
    assert_eq!(response[7], 0);
    assert_eq!(response.len(), 73);
    verify_evm_signature(&host, binding, &handle, &digest, &response[8..])?;
    let mut changed_disclosure = disclosure.clone();
    let last = changed_disclosure.len() - 2;
    changed_disclosure[last] ^= 1;
    assert_eq!(
        host.call(
            &evm_signing_request(&host, binding, &handle, &canonical, &changed_disclosure)?.0
        )?[7],
        1
    );
    let mut changed_digest = bytes.clone();
    changed_digest[92] ^= 1;
    assert_eq!(host.call(&changed_digest)?[7], 5);
    let mut foreign_call = call.clone();
    foreign_call.chain_id = 78;
    let foreign = layerx_human_kms::attestor::unsigned_evm_transaction(&foreign_call);
    assert_eq!(
        host.call(&evm_signing_request(&host, binding, &handle, &foreign, &disclosure)?.0)?[7],
        1
    );
    let mut noncanonical = canonical;
    noncanonical.push(0);
    assert_eq!(
        host.call(&evm_signing_request(&host, binding, &handle, &noncanonical, &disclosure)?.0)?[7],
        1
    );
    let mut trailing = bytes;
    trailing.push(0);
    assert!(host.call(&trailing).is_err());
    let mut tls = host.connection(Some("client"))?;
    tls.write_all(&u32::try_from(MAX + 1)?.to_be_bytes())?;
    assert!(read_frame(&mut tls, MAX).is_err());
    Ok(())
}

fn monetary_payloads() -> Result<Vec<layerx_types::payload::Payload>> {
    use layerx_crypto::payments::{asset_id, Payment, Registration};
    use layerx_types::ids::Did;
    use layerx_types::payload::{ActivityType, ModuleId, Payload};
    let actor = checked(Did::new(b"did:layerx:alice"))?;
    let issuer = checked(layerx_intents::canonical::did_id_for_protocol(&actor, 3))?;
    let registration = Payment::Register(Registration {
        asset: asset_id(&issuer, &[21; 32]),
        salt: [21; 32],
        symbol: "KMS".into(),
        name: "KMS asset".into(),
        decimals: 6,
        supply_cap: 1000,
        issuer_kind: 1,
        custody_ref: Vec::new(),
    });
    let mut payloads = vec![checked(Payload::new(
        &registry()?,
        checked(ActivityType::new(ModuleId::Asset, 1))?,
        &checked(registration.encode(actor.as_bytes()))?,
    ))?];
    // Kernel Governance::AuthorityGrant capability and budget fixtures have no construct ABI.
    let grant = include_str!(
        "../../../../agent/crates/layerx-crypto/tests/fixtures/payments/native-1-7.hex"
    )
    .trim()
    .as_bytes()
    .chunks_exact(2)
    .map(|pair| Ok(u8::from_str_radix(std::str::from_utf8(pair)?, 16)?))
    .collect::<Result<Vec<_>>>()?;
    payloads.push(checked(Payload::new(
        &registry()?,
        checked(ActivityType::new(ModuleId::Asset, 7))?,
        &grant,
    ))?);
    Ok(payloads)
}

#[test]
fn monetary_roles_are_bound_by_the_real_provider_before_and_after_restart() -> Result<()> {
    use layerx_crypto::disclosure::AmountRole;
    use layerx_crypto::payments::Payment;
    let mut host = Host::for_bindings(&[71])?;
    let binding = [71; 32];
    let (handle, _) = facts(&host.call(&request(1, binding, &[], None)?)?)?;
    let payloads = monetary_payloads()?;
    for pass in 0..2 {
        if pass == 1 {
            host.stop();
            host.start()?;
        }
        for payload in &payloads {
            let call = native_setup::setup_call(payload.clone(), (7, 1))?;
            let (canonical, disclosure) =
                construct_signing_call(&call, host.sender(binding, &handle)?)?;
            let roles: Vec<_> = disclosure
                .amounts
                .iter()
                .map(|amount| amount.role)
                .collect();
            let expected: &[u8] = match disclosure.payment.as_ref() {
                Some(Payment::Register(_)) => {
                    assert_eq!(roles, [AmountRole::SupplyCap]);
                    assert_eq!(disclosure.amounts[0].value, 1000);
                    &[3]
                }
                Some(Payment::IssueGrant(grant)) => {
                    assert_eq!(
                        roles,
                        [AmountRole::PerDrawMaximum, AmountRole::GrantAllowance]
                    );
                    assert_eq!(disclosure.amounts[0].value, grant.per_draw_maximum);
                    assert_eq!(disclosure.amounts[1].value, grant.allowance);
                    &[4, 5]
                }
                _ => return Err("unsupported monetary fixture".into()),
            };
            let encoded = encoded_disclosure(&disclosure)?;
            let (request, digest) =
                evm_signing_request(&host, binding, &handle, &canonical, &encoded)?;
            let response = host.call(&request)?;
            assert_eq!(response[7], 0);
            assert_eq!(response.len(), 73);
            verify_evm_signature(&host, binding, &handle, &digest, &response[8..])?;
            let start = 1
                + 4
                + 4
                + disclosure.actor.len()
                + 4
                + disclosure.authority.len()
                + 4
                + 33 * disclosure.counterparties.len()
                + 4;
            for (index, code) in expected.iter().enumerate() {
                let offset = start + index * 17;
                assert_eq!(encoded[offset], *code);
                for mutation in [offset, offset + 16] {
                    let mut changed = encoded.clone();
                    changed[mutation] ^= 0x80;
                    assert_eq!(
                        host.call(
                            &evm_signing_request(&host, binding, &handle, &canonical, &changed)?.0
                        )?[7],
                        1
                    );
                }
            }
        }
    }
    Ok(())
}

fn recovery_authorization(
    store: &Keystore,
    principal: &PrincipalId,
    key: &KeyId,
    wallet: [u8; 20],
    now: u64,
) -> Result<layerx_human_service::custody::EvmPlanAuthorization> {
    Ok(layerx_human_service::custody::EvmPlanAuthorization {
        plan_id: [1; 32],
        action_key: [2; 32],
        tenant: "tenant".into(),
        principal: "alice".into(),
        binding_digest: store.evm_binding(principal, key)?.digest(),
        wallet,
        not_before: now,
        not_after: now + 600,
        transaction: layerx_human_service::custody::EvmTransaction {
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 21000,
            to: [3; 20],
            value: [0; 32],
            calldata: vec![],
        },
    })
}

#[test]
fn evm_authorization_nonce_dedup_and_acknowledgement_recovery() -> Result<()> {
    use layerx_human_service::custody::EvmAcknowledgement;
    let mut host = Host::new()?;
    let principal = PrincipalId::new("alice")?;
    let key = KeyId::new("primary")?;
    let store = Keystore::open_production(
        host.root.join("evm-client"),
        125,
        host.remote("client", "beta-kms")?,
    )?;
    store.create(&principal, &key, KeyClass::HumanPrimary)?;
    let wallet = store.evm_wallet(&principal, &key)?;
    assert_ne!(wallet, [0; 20]);
    assert_eq!(wallet, store.evm_wallet(&principal, &key)?);
    let remote = host.remote("client", "beta-kms")?;
    let binding = store.evm_binding(&principal, &key)?;
    let reference = store.evm_provider_reference(&principal, &key)?;
    let wallet_request = remote.encode_evm_request(6, &binding, &reference, &[])?;
    let request_path = host.root.join("wallet.lxkp");
    fs::write(&request_path, &wallet_request)?;
    let wallet_response = host.call(&fs::read(&request_path)?)?;
    assert_eq!(&wallet_response[..7], &wallet_request[..7]);
    assert_eq!(wallet_response[7], 0);
    assert_eq!(&wallet_response[8..], wallet);
    assert_eq!(
        remote.encode_evm_request(7, &binding, &reference, &[]),
        Err(layerx_human_service::custody::KmsError::Refused)
    );
    let now = host.clock.sample(Duration::from_secs(1))?.unix_seconds();
    let authorization = recovery_authorization(&store, &principal, &key, wallet, now)?;
    assert!(store
        .sign_evm_action(&principal, &key, &authorization.action_key)
        .is_err());
    let reserved = store.authorize_evm_plan(&principal, &key, &authorization)?;
    refuse_cross_scope(&store, &principal, &key, &authorization)?;
    assert!(reserved.raw_transaction.is_empty());
    assert_eq!(
        reserved,
        store.authorize_evm_plan(&principal, &key, &authorization)?
    );
    let mut conflict = authorization.clone();
    conflict.action_key = [4; 32];
    assert!(store
        .authorize_evm_plan(&principal, &key, &conflict)
        .is_err());
    conflict = authorization.clone();
    conflict.transaction.value = [5; 32];
    assert!(store
        .authorize_evm_plan(&principal, &key, &conflict)
        .is_err());
    let signed = store.sign_evm_action(&principal, &key, &authorization.action_key)?;
    assert_eq!(signed.raw_transaction[0], 2);
    assert_eq!(
        signed,
        store.sign_evm_action(&principal, &key, &authorization.action_key)?
    );
    host.stop();
    host.start()?;
    assert_eq!(host.call(&fs::read(&request_path)?)?, wallet_response);
    assert_eq!(
        signed,
        store.recover_evm_action(&principal, &key, &authorization.action_key)?
    );
    assert_eq!(
        signed,
        store.sign_evm_action(&principal, &key, &authorization.action_key)?
    );
    assert!(!signed.acknowledged);
    let mut ack = EvmAcknowledgement {
        action_key: authorization.action_key,
        transaction_hash: [0; 32],
    };
    assert!(store
        .acknowledge_evm_action(&principal, &key, &ack)
        .is_err());
    ack.transaction_hash = signed.transaction_hash.ok_or("missing transaction hash")?;
    let acknowledged = store.acknowledge_evm_action(&principal, &key, &ack)?;
    assert!(acknowledged.acknowledged);
    host.stop();
    host.start()?;
    assert_eq!(
        acknowledged,
        store.recover_evm_action(&principal, &key, &authorization.action_key)?
    );
    conflict = authorization.clone();
    conflict.action_key = [6; 32];
    conflict.transaction.nonce = 8;
    conflict.binding_digest = [7; 32];
    assert!(store
        .authorize_evm_plan(&principal, &key, &conflict)
        .is_err());
    conflict.binding_digest = authorization.binding_digest;
    conflict.transaction.chain_id = 0;
    assert!(store
        .authorize_evm_plan(&principal, &key, &conflict)
        .is_err());
    Ok(())
}

#[test]
fn executor_certificate_cannot_authorize_or_manage_keys() -> Result<()> {
    let host = Host::for_bindings(&[81])?;
    let binding = [81; 32];
    let (handle, _) = facts(&host.call(&request(1, binding, &[], None)?)?)?;
    for operation in [0, 1, 2, 4, 7, 11] {
        let mut frame = request(
            operation,
            binding,
            if operation == 1 { &[] } else { &handle },
            None,
        )?;
        if operation >= 7 {
            frame[4..6].copy_from_slice(&3_u16.to_be_bytes());
            blob(&mut frame, b"{}")?;
        }
        let mut stream = host.connection(Some("foreign"))?;
        checked(write_frame(&mut stream, &frame, MAX))?;
        let response = checked(read_frame(&mut stream, MAX))?;
        assert_eq!(response[7], 1);
    }
    let mut frame = request(6, binding, &handle, None)?;
    frame[4..6].copy_from_slice(&3_u16.to_be_bytes());
    let mut stream = host.connection(Some("foreign"))?;
    checked(write_frame(&mut stream, &frame, MAX))?;
    let response = checked(read_frame(&mut stream, MAX))?;
    assert_eq!(response[7], 0);
    assert_eq!(response.len(), 28);
    Ok(())
}

fn export_signing_frame(
    host: &Host,
    binding: [u8; 32],
    handle: &[u8],
) -> Result<(Vec<u8>, [u8; 32])> {
    let payload = monetary_payloads()?
        .into_iter()
        .next()
        .ok_or("missing payload")?;
    let call = native_setup::setup_call(payload, (7, 1))?;
    let (canonical, disclosure) = construct_signing_call(&call, host.sender(binding, handle)?)?;
    evm_signing_request(
        host,
        binding,
        handle,
        &canonical,
        &encoded_disclosure(&disclosure)?,
    )
}

fn assert_export_seed_public(seed: &[u8], public: [u8; 32]) -> Result<()> {
    let derived = checked(ring::signature::Ed25519KeyPair::from_seed_unchecked(seed))?;
    assert_eq!(
        <[u8; 32]>::try_from(ring::signature::KeyPair::public_key(&derived).as_ref())?,
        public
    );
    Ok(())
}

#[test]
fn primary_export_is_admitted_once_committed_before_success_and_survives_restart() -> Result<()> {
    let mut host = Host::for_bindings(&[91, 92, 94])?;
    let client = host.remote("client", "beta-kms")?;
    let executor = host.remote("foreign", "beta-kms")?;
    let human = PrincipalKeyBinding::from_digest([91; 32], 125, KeyClass::HumanPrimary)?;
    let created = client.create_key(&human)?;
    let reference = created.reference().clone();
    let handle = reference.as_bytes().to_vec();
    let public = created.public_key();
    let agent = PrincipalKeyBinding::from_digest([92; 32], 125, KeyClass::AgentPrimary)?;
    let agent_reference = client.create_key(&agent)?.reference().clone();
    let (sign, digest) = export_signing_frame(&host, [91; 32], &handle)?;
    let signed = host.call(&sign)?;
    assert_eq!(signed[7], 0);
    verify_evm_signature(&host, [91; 32], &handle, &digest, &signed[8..])?;

    let recipient = ExportRecipient::new([11; 32])?;
    let ceremony = recipient.ceremony([12; 32])?;
    let export = client.encode_export_request(&human, &reference, &ceremony)?;
    assert_malformed_export_frames(&host, &export)?;
    assert_export_identity_refusals(
        &host,
        &human,
        &reference,
        &agent,
        &agent_reference,
        &ceremony,
        &export,
    )?;
    assert_eq!(
        client.describe_key(&human, &reference)?.public_key(),
        public
    );
    assert_eq!(host.call(&sign)?[7], 0);

    assert_durable_export_blocker(
        &mut host, &client, &human, &reference, &ceremony, public, &sign,
    )?;
    let envelope = client.export_envelope(&human, &reference, &ceremony)?;
    assert_eq!(
        client.export_envelope(&human, &reference, &ceremony)?,
        envelope
    );
    let (seed, acknowledgement) = recipient.open("beta-kms", human.digest(), &handle, &envelope)?;
    let wrong_recipient = ExportRecipient::new([13; 32])?;
    assert!(wrong_recipient
        .open("beta-kms", human.digest(), &handle, &envelope)
        .is_err());
    assert_eq!(
        client.export_envelope(&human, &reference, &wrong_recipient.ceremony(ceremony.id)?),
        Err(KmsError::Conflict)
    );
    assert_eq!(
        client.export_envelope(&human, &reference, &recipient.ceremony([14; 32])?),
        Err(KmsError::Conflict)
    );
    let invalid_ack = ExportAcknowledgement {
        ceremony: ceremony.clone(),
        proof: [0; 32],
    };
    assert_eq!(
        client.acknowledge_export(&human, &reference, &invalid_ack),
        Err(KmsError::Refused)
    );
    client.acknowledge_export(&human, &reference, &acknowledgement)?;
    assert_export_seed_public(seed.as_slice(), public)?;
    for pass in 0..2 {
        if pass == 1 {
            host.stop();
            host.start()?;
        }
        assert_eq!(
            client.export_envelope(&human, &reference, &ceremony).err(),
            Some(KmsError::SelfCustodied)
        );
        assert_eq!(host.call(&sign)?[7], 6);
        assert_eq!(
            client
                .rotate_key_if_current(&human, &reference, public)
                .err(),
            Some(KmsError::SelfCustodied)
        );
        assert_eq!(
            client.evm_operation(6, &human, &reference, &[]).err(),
            Some(KmsError::SelfCustodied)
        );
        assert_eq!(
            executor
                .export_envelope(&human, &reference, &ceremony)
                .err(),
            Some(KmsError::Refused)
        );
        assert_eq!(client.create_key(&human).err(), Some(KmsError::Conflict));
        let described = client.describe_key(&human, &reference)?;
        assert_eq!(described.public_key(), public);
        assert!(described.reference() == &reference);
        assert!(client.describe_key(&agent, &agent_reference)?.reference() == &agent_reference);
    }

    assert_lost_reply_export(&mut host, &recipient, &human)?;
    assert_keystore_export(&mut host, &recipient)?;
    Ok(())
}

#[test]
fn native_send_authorization_is_scoped_and_durable() -> Result<()> {
    use layerx_crypto::disclosure::call::ledger_selector;
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
    use layerx_human_service::custody::{EvmPlanAuthorization, EvmTransaction};
    let mut host = Host::new()?;
    let principal = PrincipalId::new("alice")?;
    let key = KeyId::new("primary")?;
    let store = Keystore::open_production(
        host.root.join("send-client"),
        125,
        host.remote("client", "beta-kms")?,
    )?;
    store.create(&principal, &key, KeyClass::HumanPrimary)?;
    let wallet = store.evm_wallet(&principal, &key)?;
    let recipient = [4; 20];
    let denom = "apax";
    let mut amount = [0; 32];
    amount[31] = 12;
    let mut bank = [0; 20];
    bank[18..].copy_from_slice(&[0x10, 0x01]);
    let mut calldata = ledger_selector("send(address,address,string,uint256)").to_vec();
    for address in [wallet, recipient] {
        calldata.extend([0; 12]);
        calldata.extend(address);
    }
    let mut offset = [0; 32];
    offset[31] = 128;
    calldata.extend(offset);
    calldata.extend(amount);
    let mut length = [0; 32];
    length[31] = u8::try_from(denom.len())?;
    calldata.extend(length);
    calldata.extend(denom.as_bytes());
    calldata.extend([0; 28]);
    let now = host.clock.sample(Duration::from_secs(1))?.unix_seconds();
    let authorization = EvmPlanAuthorization {
        plan_id: [1; 32],
        action_key: [2; 32],
        principal: "alice".into(),
        tenant: "tenant".into(),
        binding_digest: store.evm_binding(&principal, &key)?.digest(),
        wallet,
        not_before: now,
        not_after: now + 600,
        transaction: EvmTransaction {
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 100_000,
            to: bank,
            value: [0; 32],
            calldata: calldata.clone(),
        },
    };
    let authorize = |plan: &EvmPlanAuthorization, amount| {
        store.authorize_send_transaction(&principal, &key, plan, recipient, denom.into(), amount)
    };
    let signed = authorize(&authorization, amount)?;
    let (call, signature) = decode_signed(&signed)?;
    let expected = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: bank,
        data: calldata,
    };
    assert_eq!(call, expected);
    assert_eq!(call.signing_hash(), expected.signing_hash());
    assert_eq!(recover_sender(&call, &signature)?, wallet);
    let recovered = store.recover_evm_action(&principal, &key, &authorization.action_key)?;
    assert_eq!(recovered.authorization.plan_id, authorization.plan_id);
    assert_eq!(
        recovered.authorization.binding_digest,
        authorization.binding_digest
    );
    host.stop();
    host.start()?;
    assert_eq!(signed, authorize(&authorization, amount)?);
    let mut changed_amount = amount;
    changed_amount[31] += 1;
    assert!(authorize(&authorization, changed_amount).is_err());
    let mut changed = authorization.clone();
    changed.action_key = [8; 32];
    changed.transaction.chain_id = 78;
    assert!(authorize(&changed, amount).is_err());
    changed.transaction.chain_id = 125;
    changed.binding_digest = [9; 32];
    assert!(authorize(&changed, amount).is_err());
    Ok(())
}

fn transaction_signature(raw: &[u8]) -> Result<Vec<u8>> {
    let mut signature = vec![0; 65];
    let prefix = *raw.get(1).ok_or("missing RLP list")?;
    let mut offset = if prefix >= 0xf8 {
        2 + usize::from(prefix - 0xf7)
    } else {
        2
    };
    for index in 0..12 {
        let prefix = *raw.get(offset).ok_or("missing RLP field")?;
        offset += 1;
        let value = if prefix < 0x80 {
            std::slice::from_ref(&raw[offset - 1])
        } else if prefix <= 0xb7 {
            let length = usize::from(prefix - 0x80);
            let value = raw.get(offset..offset + length).ok_or("short RLP field")?;
            offset += length;
            value
        } else if prefix == 0xc0 {
            &[]
        } else {
            return Err("unsupported test RLP field".into());
        };
        if index == 9 {
            signature[64] = value.first().copied().unwrap_or(0);
        }
        if index == 10 || index == 11 {
            if value.len() > 32 {
                return Err("large signature field".into());
            }
            let end = if index == 10 { 32 } else { 64 };
            signature[end - value.len()..end].copy_from_slice(value);
        }
    }
    assert_eq!(offset, raw.len());
    Ok(signature)
}

#[test]
fn external_signature_verification_and_executor_journal_recovery() -> Result<()> {
    use layerx_human_service::custody::{
        EvmAction, EvmExternalSignature, EvmPlanAuthorization, EvmTransaction,
    };
    let mut host = Host::new()?;
    let principal = PrincipalId::new("alice")?;
    let key = KeyId::new("primary")?;
    let store = Keystore::open_production(
        host.root.join("external-client"),
        125,
        host.remote("client", "beta-kms")?,
    )?;
    store.create(&principal, &key, KeyClass::HumanPrimary)?;
    let binding = store.evm_binding(&principal, &key)?;
    let handle = store.evm_provider_reference(&principal, &key)?;
    let now = host.clock.sample(Duration::from_secs(1))?.unix_seconds();
    let authorization = EvmPlanAuthorization {
        plan_id: [1; 32],
        action_key: [2; 32],
        tenant: "tenant".into(),
        principal: "alice".into(),
        binding_digest: binding.digest(),
        wallet: store.evm_wallet(&principal, &key)?,
        not_before: now,
        not_after: now + 600,
        transaction: EvmTransaction {
            chain_id: 125,
            nonce: 0,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 21000,
            to: [4; 20],
            value: [0; 32],
            calldata: vec![],
        },
    };
    store.authorize_evm_plan(&principal, &key, &authorization)?;
    let signed = store.sign_evm_action(&principal, &key, &authorization.action_key)?;
    let mut external = EvmExternalSignature {
        action_key: authorization.action_key,
        signature: transaction_signature(&signed.raw_transaction)?,
    };
    let executor = host.remote("foreign", "beta-kms")?;
    let accepted: EvmAction = serde_json::from_slice(&checked(executor.evm_operation(
        12,
        &binding,
        &handle,
        &serde_json::to_vec(&external)?,
    ))?)?;
    assert_eq!(accepted, signed);
    external.signature[64] += 27;
    let accepted: EvmAction = serde_json::from_slice(&checked(executor.evm_operation(
        12,
        &binding,
        &handle,
        &serde_json::to_vec(&external)?,
    ))?)?;
    assert_eq!(accepted, signed);
    host.stop();
    host.start()?;
    assert_eq!(
        signed,
        store.recover_evm_action(&principal, &key, &authorization.action_key)?
    );
    external.signature[0] ^= 1;
    assert!(executor
        .evm_operation(12, &binding, &handle, &serde_json::to_vec(&external)?)
        .is_err());
    external.signature = vec![0; 64];
    assert!(executor
        .evm_operation(12, &binding, &handle, &serde_json::to_vec(&external)?)
        .is_err());
    let foreign = k256::ecdsa::SigningKey::from_bytes((&[21; 32]).into())?;
    let (signature, recovery) = foreign.sign_prehash_recoverable(&[22; 32])?;
    external.signature = signature.to_bytes().to_vec();
    external.signature.push(recovery.to_byte());
    assert!(executor
        .evm_operation(12, &binding, &handle, &serde_json::to_vec(&external)?)
        .is_err());
    external.action_key = [23; 32];
    assert!(executor
        .evm_operation(12, &binding, &handle, &serde_json::to_vec(&external)?)
        .is_err());
    Ok(())
}

fn refuse_cross_scope(
    store: &Keystore,
    principal: &PrincipalId,
    key: &KeyId,
    authorization: &layerx_human_service::custody::EvmPlanAuthorization,
) -> Result<()> {
    use layerx_human_service::custody::{CustodyError, KmsError};
    let mut changed = authorization.clone();
    "another-tenant".clone_into(&mut changed.tenant);
    assert!(matches!(
        store.authorize_evm_plan(principal, key, &changed),
        Err(CustodyError::Kms(KmsError::Conflict))
    ));
    changed = authorization.clone();
    "bob".clone_into(&mut changed.principal);
    assert!(matches!(
        store.authorize_evm_plan(principal, key, &changed),
        Err(CustodyError::Kms(KmsError::Refused))
    ));
    let other = PrincipalId::new("bob")?;
    store.create(&other, key, KeyClass::HumanPrimary)?;
    store.evm_wallet(&other, key)?;
    assert!(store
        .recover_evm_action(&other, key, &authorization.action_key)
        .is_err());
    assert!(store
        .sign_evm_action(&other, key, &authorization.action_key)
        .is_err());
    Ok(())
}

fn mutated_payment_fields(
    disclosure: &layerx_crypto::disclosure::Disclosure,
) -> Vec<layerx_crypto::disclosure::Disclosure> {
    let mutations: &[fn(&mut layerx_crypto::disclosure::Disclosure)] = &[
        |value| value.asset[0] ^= 1,
        |value| value.counterparties[0].account[0] ^= 1,
        |value| value.amounts[0].value += 1,
        |value| value.fee_limit += 1,
        |value| value.expiry.payload_expires_at += 1,
        |value| value.idempotency_key[0] ^= 1,
    ];
    mutations
        .iter()
        .map(|mutate| {
            let mut changed = disclosure.clone();
            mutate(&mut changed);
            changed
        })
        .collect()
}

#[test]
fn owner_fee_grants_bind_every_disclosed_budget_field_across_provider_restart() -> Result<()> {
    use layerx_crypto::authority_grant::AuthorityGrant;
    use layerx_types::ids::Did;
    use layerx_types::payload::{ActivityType, ModuleId, Payload};
    let mut host = Host::for_bindings(&[73])?;
    let binding = [73; 32];
    let (handle, _) = facts(&host.call(&request(1, binding, &[], None)?)?)?;
    let actor = checked(Did::new(b"did:layerx:alice"))?;
    let issuer = checked(layerx_intents::canonical::did_id_for_protocol(&actor, 3))?;
    let mut grant = checked(AuthorityGrant::decode(include_bytes!(
        "../../../../../tests/fixtures/authority/native-fee-grants/period-bound/grant.bin"
    )))?;
    grant.grantor = issuer;
    grant.grantee = issuer;
    let payload = checked(Payload::new(
        &registry()?,
        checked(ActivityType::new(ModuleId::Governance, 8))?,
        &checked(grant.payload())?,
    ))?;
    let grant = checked(AuthorityGrant::from_payload(payload.as_bytes()))?;
    let sender = host.sender(binding, &handle)?;
    let call = canonical_send(
        125,
        &layerx_crypto::send::CallOptions {
            from: sender,
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 0,
            max_fee_per_gas: 1,
            gas_limit: 1,
        },
    )?
    .call;
    let (canonical, disclosure) = construct_signing_call(&call, sender)?;
    // Decision 2333: send(bytes) carries no fee grant; mutate only its bound fields.
    let mut unbound = disclosure.clone();
    unbound.authority_grant = Some(grant);
    for pass in 0..2 {
        if pass == 1 {
            host.stop();
            host.start()?;
        }
        let encoded = encoded_disclosure(&disclosure)?;
        assert_eq!(encoded[0], 1);
        let unbound_encoded = encoded_disclosure(&unbound)?;
        assert_eq!(unbound_encoded[0], 2);
        assert_eq!(
            host.call(
                &evm_signing_request(&host, binding, &handle, &canonical, &unbound_encoded,)?.0
            )?[7],
            1,
            "a fee grant absent from the payment call is refused"
        );
        let (signed_request, digest) =
            evm_signing_request(&host, binding, &handle, &canonical, &encoded)?;
        let response = host.call(&signed_request)?;
        assert_eq!(response[7], 0);
        assert_eq!(response.len(), 73);
        verify_evm_signature(&host, binding, &handle, &digest, &response[8..])?;
        for changed in mutated_payment_fields(&disclosure) {
            assert_eq!(
                host.call(
                    &evm_signing_request(
                        &host,
                        binding,
                        &handle,
                        &canonical,
                        &encoded_disclosure(&changed)?
                    )?
                    .0
                )?[7],
                1
            );
        }
        let mut original_version = unbound_encoded;
        original_version[0] = 1;
        assert_eq!(
            host.call(
                &evm_signing_request(&host, binding, &handle, &canonical, &original_version)?.0
            )?[7],
            1
        );
        let mut truncated = encoded.clone();
        truncated.truncate(truncated.len() - 1);
        assert_eq!(
            host.call(&evm_signing_request(&host, binding, &handle, &canonical, &truncated)?.0)?[7],
            1
        );
    }
    Ok(())
}

#[test]
fn session_registration_and_replacement_disclosures_survive_provider_restart() -> Result<()> {
    use layerx_crypto::disclosure::{DisclosedSessionGrant, DisclosedSessionReplacement};
    use layerx_crypto::session::decode_session_key;
    let mut host = Host::for_bindings(&[74])?;
    let binding = [74; 32];
    let (handle, _) = facts(&host.call(&request(1, binding, &[], None)?)?)?;
    let grantor = checked(layerx_intents::canonical::did_id_for_protocol(
        &checked(layerx_types::ids::Did::new(b"did:layerx:alice"))?,
        3,
    ))?;
    for pass in 0..2 {
        if pass == 1 {
            host.stop();
            host.start()?;
        }
        for version in 1..=4 {
            let issued = session_disclosure_grant(version, grantor)?;
            let expiry_sequence = 9000;
            let action_key = [4; 32];
            let replacement = (version == 4).then_some(DisclosedSessionReplacement {
                predecessor_grant_id: [5; 32],
                expected_charge_state: [6; 32],
            });
            let mut grant = checked(layerx_intents::SessionGrant::new(
                issued.registration_payload.clone(),
                expiry_sequence,
                action_key,
            ))?;
            if let Some(replacement) = replacement {
                grant = checked(grant.replacing(
                    replacement.predecessor_grant_id,
                    replacement.expected_charge_state,
                ))?;
            }
            let _compiled = checked(layerx_intents::compile(
                &layerx_intents::Intent::v3(layerx_intents::IntentKind::SessionGrant(grant)),
                &registry()?,
            ))?;
            let sender = host.sender(binding, &handle)?;
            let call = canonical_send(
                125,
                &layerx_crypto::send::CallOptions {
                    from: sender,
                    chain_id: 125,
                    nonce: 7,
                    max_priority_fee_per_gas: 0,
                    max_fee_per_gas: 1,
                    gas_limit: 1,
                },
            )?
            .call;
            let (canonical, disclosure) = construct_signing_call(&call, sender)?;
            // Decision 2333: send(bytes) carries no session or replacement fields.
            let mut unbound = disclosure.clone();
            unbound.session_grant = Some(DisclosedSessionGrant {
                grant: checked(decode_session_key(&issued.registration_payload))?,
                expiry_sequence,
                action_key,
                replacement,
            });
            let encoded = encoded_disclosure(&disclosure)?;
            assert_eq!(encoded[0], 1);
            let unbound_encoded = encoded_disclosure(&unbound)?;
            assert_eq!(unbound_encoded[0], 3);
            assert_eq!(
                host.call(
                    &evm_signing_request(&host, binding, &handle, &canonical, &unbound_encoded,)?.0
                )?[7],
                1,
                "session metadata absent from the payment call is refused"
            );
            let (request, digest) =
                evm_signing_request(&host, binding, &handle, &canonical, &encoded)?;
            let response = host.call(&request)?;
            assert_eq!(response[7], 0);
            verify_evm_signature(&host, binding, &handle, &digest, &response[8..])?;
            for changed in mutated_payment_fields(&disclosure) {
                assert_eq!(
                    host.call(
                        &evm_signing_request(
                            &host,
                            binding,
                            &handle,
                            &canonical,
                            &encoded_disclosure(&changed)?
                        )?
                        .0
                    )?[7],
                    1
                );
            }
            let mut legacy = unbound_encoded;
            legacy[0] = 1;
            assert_eq!(
                host.call(&evm_signing_request(&host, binding, &handle, &canonical, &legacy)?.0)?
                    [7],
                1
            );
        }
    }
    Ok(())
}

fn session_disclosure_grant(
    version: u8,
    grantor: [u8; 32],
) -> Result<layerx_crypto::session::IssuedSessionKey> {
    use layerx_crypto::authority_grant::NativeFeeBudget;
    use layerx_crypto::local::LocalSigner;
    use layerx_crypto::session::{issue_session_key, SessionKeyRequest, SessionPurpose};
    use layerx_crypto::signer::Signer as _;
    use layerx_types::payload::{ActivityType, ModuleId};
    checked(issue_session_key(&SessionKeyRequest {
        grantor,
        session_public_key: LocalSigner::new([0x67; 32]).public_key(),
        not_before: 1000,
        expires_at: Some(3_601_000),
        revocation_sequence: Some(3),
        permitted_activity_types: if version == 3 {
            vec![]
        } else {
            vec![checked(ActivityType::new(ModuleId::Asset, 5))?]
        },
        fee_budget: matches!(version, 2 | 4).then_some(NativeFeeBudget {
            asset: [3; 32],
            maximum_per_activity: 4,
            maximum_total: 12,
            period_length: 60_000,
            maximum_per_period: 8,
            period_start: 1000,
        }),
        purpose: if version == 3 {
            SessionPurpose::Authentication
        } else {
            SessionPurpose::Activity
        },
    }))
}

#[test]
fn public_wallet_signing_isolated_for_two_principals_after_restart() -> Result<()> {
    let mut host = Host::new()?;
    let client = host.remote("client", "beta-kms")?;
    let store = Keystore::open_production(
        host.root.join("isolated-client"),
        125,
        host.remote("client", "beta-kms")?,
    )?;
    let key = KeyId::new("primary")?;
    let mut rows = Vec::new();
    for name in ["alice", "bob"] {
        let principal = PrincipalId::new(name)?;
        let public = store.create(&principal, &key, KeyClass::HumanPrimary)?;
        let binding = store.evm_binding(&principal, &key)?;
        let reference = store.evm_provider_reference(&principal, &key)?;
        let description = client.describe_key(&binding, &reference)?;
        assert_eq!(description.public_key(), public);
        let wallet = store.evm_wallet(&principal, &key)?;
        assert_eq!(
            client
                .wallet_descriptor(&binding, &reference)?
                .evm_secp256k1,
            wallet
        );
        rows.push((binding, reference, wallet));
    }
    assert_ne!(rows[0].2, rows[1].2);
    assert!(client.wallet_descriptor(&rows[1].0, &rows[0].1).is_err());
    for pass in 0..2 {
        if pass == 1 {
            host.stop();
            host.start()?;
        }
        for (binding, reference, wallet) in &rows {
            assert_eq!(
                client.wallet_descriptor(binding, reference)?.evm_secp256k1,
                *wallet
            );
            let mut to = [0; 20];
            to[18..].copy_from_slice(&[0x10, 0x1a]);
            let mut data =
                layerx_crypto::disclosure::call::ledger_selector("openAccount(bytes32)").to_vec();
            data.extend([3; 32]);
            let call = layerx_crypto::evm_transaction::Eip1559Call {
                chain_id: 125,
                nonce: pass,
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 2,
                gas_limit: 100_000,
                to,
                data,
            };
            let (canonical, disclosure) = construct_signing_call(&call, *wallet)?;
            let (frame, digest) = evm_signing_request(
                &host,
                binding.digest(),
                reference.as_bytes(),
                &canonical,
                &encoded_disclosure(&disclosure)?,
            )?;
            let response = host.call(&frame)?;
            assert_eq!(response[7], 0);
            verify_evm_signature(
                &host,
                binding.digest(),
                reference.as_bytes(),
                &digest,
                &response[8..],
            )?;
        }
    }
    Ok(())
}

fn assert_malformed_export_frames(host: &Host, export: &[u8]) -> Result<()> {
    let mut legacy = export.to_vec();
    legacy[4..6].copy_from_slice(&1_u16.to_be_bytes());
    let mut evm_version = export.to_vec();
    evm_version[4..6].copy_from_slice(&3_u16.to_be_bytes());
    let mut with_evm_payload = export.to_vec();
    blob(&mut with_evm_payload, b"{}")?;
    let truncated = export[..export.len() - 1].to_vec();
    let mut unknown = export.to_vec();
    unknown[6] = 15;
    let mut empty_reference = request(14, [91; 32], &[], None)?;
    empty_reference[4..6].copy_from_slice(&5_u16.to_be_bytes());
    for frame in [
        legacy,
        evm_version,
        with_evm_payload,
        truncated,
        unknown,
        empty_reference,
    ] {
        let mut stream = host.connection(Some("client"))?;
        checked(write_frame(&mut stream, &frame, MAX))?;
        assert!(read_frame(&mut stream, MAX).is_err());
    }
    Ok(())
}

fn assert_export_identity_refusals(
    host: &Host,
    human: &PrincipalKeyBinding,
    reference: &ProviderKeyReference,
    agent: &PrincipalKeyBinding,
    agent_reference: &ProviderKeyReference,
    ceremony: &ExportCeremony,
    export: &[u8],
) -> Result<()> {
    let client = host.remote("client", "beta-kms")?;
    let executor = host.remote("foreign", "beta-kms")?;
    let mut stream = host.connection(Some("foreign"))?;
    checked(write_frame(&mut stream, export, MAX))?;
    assert_eq!(
        checked(read_frame(&mut stream, MAX))?,
        b"LXKP\0\x08\x0e\x01"
    );
    assert_eq!(
        executor.export_envelope(human, reference, ceremony).err(),
        Some(KmsError::Refused)
    );
    assert_eq!(
        host.remote("client", "other-kms")?
            .export_envelope(human, reference, ceremony)
            .err(),
        Some(KmsError::Refused)
    );
    assert_eq!(
        client
            .export_envelope(agent, agent_reference, ceremony)
            .err(),
        Some(KmsError::Refused)
    );
    let misclassed = PrincipalKeyBinding::from_digest([91; 32], 125, KeyClass::AgentPrimary)?;
    assert_eq!(
        client
            .export_envelope(&misclassed, reference, ceremony)
            .err(),
        Some(KmsError::Integrity)
    );
    assert_eq!(
        client
            .export_envelope(human, &ProviderKeyReference::new(vec![33; 32])?, ceremony)
            .err(),
        Some(KmsError::Integrity)
    );
    assert_eq!(
        client
            .export_envelope(
                &PrincipalKeyBinding::from_digest([93; 32], 125, KeyClass::HumanPrimary)?,
                reference,
                ceremony
            )
            .err(),
        Some(KmsError::KeyNotFound)
    );
    Ok(())
}

fn assert_durable_export_blocker(
    host: &mut Host,
    client: &RemoteKmsProvider,
    human: &PrincipalKeyBinding,
    reference: &ProviderKeyReference,
    ceremony: &ExportCeremony,
    public: [u8; 32],
    sign: &[u8],
) -> Result<()> {
    let blocker = host.root.join("state/state.next");
    fs::create_dir(&blocker)?;
    assert_eq!(
        client.export_envelope(human, reference, ceremony).err(),
        Some(KmsError::Unavailable)
    );
    assert_eq!(
        client.describe_key(human, reference).err(),
        Some(KmsError::Unavailable)
    );
    assert_eq!(host.call(sign)?[7], 4);
    host.stop();
    fs::remove_dir(&blocker)?;
    host.start()?;
    assert_eq!(client.describe_key(human, reference)?.public_key(), public);
    assert_eq!(host.call(sign)?[7], 0);

    Ok(())
}

fn assert_lost_reply_export(
    host: &mut Host,
    recipient: &ExportRecipient,
    human: &PrincipalKeyBinding,
) -> Result<()> {
    let client = host.remote("client", "beta-kms")?;
    let lost = PrincipalKeyBinding::from_digest([94; 32], 125, KeyClass::HumanPrimary)?;
    let lost_created = client.create_key(&lost)?;
    let lost_reference = lost_created.reference().clone();
    let lost_ceremony = recipient.ceremony([15; 32])?;
    let lost_export = client.encode_export_request(&lost, &lost_reference, &lost_ceremony)?;
    {
        let mut tls = host.connection(Some("client"))?;
        checked(write_frame(&mut tls, &lost_export, MAX))?;
    }
    let clock = Arc::clone(&host.clock);
    let mut deadline = Deadline::start(clock.as_ref(), Duration::from_secs(5))?;
    loop {
        if client.evm_operation(6, &lost, &lost_reference, &[]).err()
            == Some(KmsError::SelfCustodied)
        {
            break;
        }
        if deadline.remaining(clock.as_ref())?.is_zero() {
            return Err("lost-response export was not committed".into());
        }
        std::thread::sleep(Duration::from_millis(20));
    }
    let lost_envelope = client.export_envelope(&lost, &lost_reference, &lost_ceremony)?;
    host.stop();
    host.start()?;
    assert_eq!(
        client.export_envelope(&lost, &lost_reference, &lost_ceremony)?,
        lost_envelope
    );
    let (recovered, lost_ack) = recipient.open(
        "beta-kms",
        lost.digest(),
        lost_reference.as_bytes(),
        &lost_envelope,
    )?;
    assert_eq!(
        checked(ring::signature::Ed25519KeyPair::from_seed_unchecked(
            &*recovered
        ))?
        .public_key()
        .as_ref(),
        lost_created.public_key()
    );
    assert!(client
        .export_envelope(human, &lost_reference, &lost_ceremony)
        .is_err());
    assert_eq!(
        client.export_envelope(&lost, &lost_reference, &recipient.ceremony([16; 32])?),
        Err(KmsError::Conflict)
    );
    client.acknowledge_export(&lost, &lost_reference, &lost_ack)?;
    host.stop();
    host.start()?;
    assert_eq!(
        client
            .export_envelope(&lost, &lost_reference, &lost_ceremony)
            .err(),
        Some(KmsError::SelfCustodied)
    );
    client.acknowledge_export(&lost, &lost_reference, &lost_ack)?;
    assert_eq!(client.create_key(&lost).err(), Some(KmsError::Conflict));
    assert_eq!(
        client.describe_key(&lost, &lost_reference)?.public_key(),
        lost_created.public_key()
    );

    Ok(())
}

fn assert_keystore_export(host: &mut Host, recipient: &ExportRecipient) -> Result<()> {
    let client = host.remote("client", "beta-kms")?;
    let keystore = Keystore::open_production(
        host.root.join("client-state"),
        125,
        host.remote("client", "beta-kms")?,
    )?;
    let alice = PrincipalId::new("alice")?;
    let bob = PrincipalId::new("bob")?;
    let key = KeyId::new("primary")?;
    let alice_public = keystore.create(&alice, &key, KeyClass::HumanPrimary)?;
    let bob_public = keystore.create(&bob, &key, KeyClass::AgentPrimary)?;
    assert!(keystore.export_primary_once(&bob, &key).is_err());
    let alice_binding = keystore.evm_binding(&alice, &key)?;
    let alice_reference = keystore.evm_provider_reference(&alice, &key)?;
    let alice_ceremony = recipient.ceremony([17; 32])?;
    let exported = client.export_envelope(&alice_binding, &alice_reference, &alice_ceremony)?;
    let (alice_seed, alice_ack) = recipient.open(
        "beta-kms",
        alice_binding.digest(),
        alice_reference.as_bytes(),
        &exported,
    )?;
    assert_eq!(
        checked(ring::signature::Ed25519KeyPair::from_seed_unchecked(
            &*alice_seed
        ))?
        .public_key()
        .as_ref(),
        alice_public
    );
    client.acknowledge_export(&alice_binding, &alice_reference, &alice_ack)?;
    assert!(keystore.export_primary_once(&alice, &key).is_err());
    host.stop();
    host.start()?;
    assert!(keystore.export_primary_once(&alice, &key).is_err());
    assert_eq!(keystore.describe(&alice, &key)?.public_key, alice_public);
    assert_eq!(keystore.describe(&bob, &key)?.public_key, bob_public);
    Ok(())
}
