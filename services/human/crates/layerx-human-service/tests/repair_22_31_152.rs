//! Repair record 22.31.152, audit 22.30.103, acceptance 22998.1.
//! funding-authorization-refused: funding_authorization_round_trips_and_recovers.
//! export-key-loss: export_delivery_recovers_lost_reply_and_failed_marker.
//! remote-native-signature-gap: remote_native_admission_refuses_before_discovery; attestor_native_consent_still_signs.
//! Baseline not run: the ordered implementation/build/hf-done instructions prohibit a pre-implementation verification.
//! Wiring request (Human domain 10): services/human/crates/layerx-human-service/src/server/agent_creation_native.rs must replace authorize_send and its 64-byte journal with CustodySigner::authorize_send_transaction and persist SignedBankTransfer raw bytes, hash and full 65-byte signature before submission; the legacy refusal stays until migration.
//! Wiring request (Human domain 10): services/human/crates/layerx-human-service/src/security.rs must bind ExportCeremony to authenticated owner/recipient, return export_primary_envelope on retries despite audit failure, and call acknowledge_primary_export only after recipient decryption and durable retention.
//! Wiring request (Human KMS domain 10): services/human/crates/layerx-human-kms/src/store.rs must defer managed-signing retirement to acknowledgement if signing during pending encrypted delivery is required; its existing v8 durable delivery survives retirement before acknowledgement.
//! Wiring request (Human domain 10): services/human/crates/layerx-human-service/src/server/production_components.rs must propagate kms-unsupported-native-consent instead of replacing the startup refusal with a generic message; operation discovery must stay behind successful startup.
//! Gotcha: NativeSendPurposeV1 is already explicitly retired by layerx-agent-api; the retained working attestor proof uses the valid preparation-purpose producer, without changing that external interface.
//! Scope forbids spec/GOTCHA edits; finding/test mapping, baseline limitation and wiring are retained here. Gate evidence is retained by hf-build/hf-done at the candidate revision.

#[allow(dead_code)]
mod cluster {
    include!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../layerx-human-kms/tests/support/attestor_cluster.rs"
    ));

    pub fn allow_preparation(
        cluster: &mut Cluster,
        key: &str,
        public: &[u8],
        owner: &str,
    ) -> Result<()> {
        let path = cluster.root.join("native-consent.json");
        fs::write(
            &path,
            serde_json::to_vec(&json!({
                "version": 2, "rules": [{
                    "key_id": key, "owner": owner, "tenant": owner,
                    "agent_did": "did:layerx:alice", "owner_public_key": hex(public),
                    "operations": ["preparation-purpose"], "capabilities": [hex(&[0x22;32])],
                    "maximum_validity_ms": 600000, "rate_per_minute": 1000
                }]
            }))?,
        )?;
        for (_, child) in &mut cluster.children {
            child.kill()?;
            child.wait()?;
        }
        cluster.children.clear();
        for (id, command) in &mut cluster.commands {
            cluster.children.push((
                id.clone(),
                command.env("ATTESTOR_NATIVE_POLICY_FILE", &path).spawn()?,
            ));
        }
        cluster.wait_ready()
    }
}

use std::fs;
use std::future::Future;
use std::net::{SocketAddr, TcpListener};
use std::os::unix::fs::PermissionsExt as _;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::{Arc, Mutex};
use std::task::{Context, Poll, Wake, Waker};
use std::time::{Duration, Instant};

use cluster::{checked, hex, scratch, Pki, Result};
use layerx_agent_api::identity::{
    AgentDid, CapabilityId, NativePreparationPurposeV1, SessionId, TenantId,
};
use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_human_kms::export::ExportRecipient;
use layerx_human_service::custody::{
    CustodyError, CustodySigner, EnvelopeKms, EvmPlanAuthorization, EvmTransaction, KeyClass,
    KeyEntropy, KeyId, Keystore, KmsError, KmsProvider, NativeConsent, NativeConsentRequest,
    Operation, RemoteKmsProvider, SignAuthorization, SigningLimits,
};
use layerx_human_service::frame::{ConnectionGate, FrameTransport, Limits, MutualTlsConfig, Tls};
use layerx_human_service::server::production_components::{AttestorCustodyConfig, AttestorKms};
use layerx_human_service::store::PrincipalStore;
use layerx_human_service::trace::TraceId;
use layerx_human_test_support::{install_and_open, principal, retention_uniform, tenancy};
use layerx_types::clock::Clock;
use layerx_types::payload::ModuleRegistry;
use rustls::pki_types::{CertificateDer, PrivateKeyDer};
use rustls::RootCertStore;
use sha2::{Digest as _, Sha256};

const PROVIDER: &str = "repair-22-31-152";
const CHILD: &str = "REPAIR_22_31_152_KMS_PROCESS";
const MAX: usize = 2_097_152;
static SERIAL: Mutex<()> = Mutex::new(());

struct Root(PathBuf);
impl Drop for Root {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}
struct ClockEnvironment(Vec<(&'static str, Option<std::ffi::OsString>)>);
impl ClockEnvironment {
    fn install(socket: &std::path::Path) -> Self {
        let entries = [
            ("LAYERX_RUNTIME_CLOCK_SOCKET", socket.as_os_str().to_owned()),
            (
                "LAYERX_RUNTIME_CLOCK_PID",
                std::process::id().to_string().into(),
            ),
            (
                "LAYERX_RUNTIME_CLOCK_UID",
                rustix::process::geteuid().as_raw().to_string().into(),
            ),
        ];
        let old = entries
            .iter()
            .map(|(name, _)| (*name, std::env::var_os(name)))
            .collect();
        for (name, value) in entries {
            std::env::set_var(name, value);
        }
        Self(old)
    }
}
impl Drop for ClockEnvironment {
    fn drop(&mut self) {
        for (name, value) in &self.0 {
            match value {
                Some(value) => std::env::set_var(name, value),
                None => std::env::remove_var(name),
            }
        }
    }
}

fn envelope(root: &Root) -> Result<Keystore> {
    Ok(Keystore::open_development(
        root.0.join("custody"),
        125,
        EnvelopeKms::new(PROVIDER, root.0.join("seal"))?,
    )?)
}
fn service(keystore: Keystore, store: PrincipalStore) -> Result<CustodySigner> {
    Ok(CustodySigner::new(
        keystore,
        store,
        checked(ModuleRegistry::new(&[]))?,
        SigningLimits::new(32, 60)?,
    ))
}

#[test]
fn funding_authorization_round_trips_and_recovers() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "serialization poisoned")?;
    let clock = cluster::StepClock::new()?;
    let authority = cluster::ClockAuthority::start(clock.clone())?;
    let _environment = ClockEnvironment::install(&authority.socket);
    let root = Root(scratch("funding-22-31-152")?);
    fs::write(root.0.join("seal"), [0x42; 64])?;
    let keystore = envelope(&root)?;
    let principal = principal("alice");
    let key = KeyId::new("primary")?;
    keystore.generate(
        &principal,
        &key,
        KeyClass::HumanPrimary,
        KeyEntropy::new([0x43; 32], [0x44; 16], [0x45; 24])?,
    )?;
    let wallet = keystore.evm_wallet(&principal, &key)?;
    let recipient = [0x55; 20];
    let mut amount = [0; 32];
    amount[31] = 42;
    let mut bank = [0; 20];
    bank[18..].copy_from_slice(&0x1001_u16.to_be_bytes());
    let now = clock.now()?;
    let transaction = EvmTransaction {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 100000,
        to: bank,
        value: [0; 32],
        calldata: Vec::new(),
    }
    .with_bank_transfer(wallet, recipient, "alxp", amount)?;
    let authorization = EvmPlanAuthorization {
        plan_id: [1; 32],
        action_key: [2; 32],
        tenant: "tenant-a".into(),
        principal: "alice".into(),
        binding_digest: keystore.evm_binding(&principal, &key)?.digest(),
        wallet,
        not_before: now - 1,
        not_after: now + 300,
        transaction,
    };
    let (store, digest) = install_and_open(
        &root.0.join("store"),
        &tenancy(&[("alice", "tenant-a")]),
        retention_uniform(1000),
    );
    let signer = service(keystore, store)?;
    let signed = signer.authorize_send_transaction(
        &principal,
        &key,
        &authorization,
        recipient,
        "alxp".into(),
        amount,
    )?;
    let (call, signature) = checked(decode_signed(signed.raw_transaction()))?;
    assert_eq!(signature.as_bytes(), signed.signature());
    assert!(matches!(signed.signature()[64], 27 | 28));
    assert_eq!(checked(recover_sender(&call, &signature))?, wallet);
    assert_eq!(signed.sender(), wallet);
    assert_eq!(call.nonce, 7);
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.data, authorization.transaction.calldata);
    assert_eq!(
        transaction_hash(signed.raw_transaction()),
        signed.transaction_hash()
    );
    assert_eq!(
        checked(checked(bind_call(&call, wallet))?.construct_method())?,
        Some(ConstructMethod::BankSend {
            from: wallet,
            to: recipient,
            denom: "alxp".into(),
            amount
        })
    );
    assert!(signer
        .authorize_send_transaction(
            &principal,
            &key,
            &authorization,
            [0x56; 20],
            "alxp".into(),
            amount
        )
        .is_err());
    let mut wrong = authorization.clone();
    wrong.wallet = [0x57; 20];
    assert!(signer
        .authorize_send_transaction(&principal, &key, &wrong, recipient, "alxp".into(), amount)
        .is_err());
    let mut wrong = authorization.clone();
    wrong.transaction.nonce += 1;
    wrong.transaction.calldata[16] ^= 1;
    assert!(signer
        .authorize_send_transaction(&principal, &key, &wrong, recipient, "alxp".into(), amount)
        .is_err());
    let mut conflict = authorization.clone();
    conflict.action_key = [3; 32];
    assert!(matches!(
        signer.authorize_send_transaction(
            &principal,
            &key,
            &conflict,
            recipient,
            "alxp".into(),
            amount
        ),
        Err(CustodyError::Kms(KmsError::Conflict))
    ));
    drop(signer);
    let keystore = envelope(&root)?;
    let store = PrincipalStore::open(root.0.join("store"), retention_uniform(1000), digest)?;
    let signer = service(keystore, store)?;
    let recovered = signer.authorize_send_transaction(
        &principal,
        &key,
        &authorization,
        recipient,
        "alxp".into(),
        amount,
    )?;
    assert_eq!(recovered, signed);
    Ok(())
}

struct KmsHost {
    root: Root,
    address: SocketAddr,
    pki: Pki,
    clock: Arc<cluster::StepClock>,
    child: Option<Child>,
}
impl Drop for KmsHost {
    fn drop(&mut self) {
        self.stop();
    }
}
impl KmsHost {
    fn new() -> Result<Self> {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let root = Root(scratch("export-22-31-152")?);
        let mut pki = Pki::new(&root.0);
        pki.authority("ca")?;
        pki.leaf("ca", "server", true)?;
        pki.leaf("ca", "client", false)?;
        for name in ["ca", "server", "client"] {
            pki.certificate_der(name)?;
        }
        for name in ["server", "client"] {
            pki.key_der(name)?;
        }
        fs::write(root.0.join("snapshot.json"), br#"{"chain_id":125}"#)?;
        fs::write(root.0.join("seal"), [0x42; 32])?;
        for entry in fs::read_dir(&root.0)? {
            fs::set_permissions(entry?.path(), fs::Permissions::from_mode(0o600))?;
        }
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let clock = cluster::StepClock::new()?;
        let mut host = Self {
            root,
            address,
            pki,
            clock,
            child: None,
        };
        drop(listener);
        host.start()?;
        Ok(host)
    }
    fn start(&mut self) -> Result<()> {
        let mut command = Command::new(std::env::current_exe()?);
        command
            .args(["--exact", "kms_producer_process", "--nocapture"])
            .env(CHILD, "1")
            .env("LAYERX_HUMAN_KMS_LISTEN", self.address.to_string())
            .env("LAYERX_HUMAN_KMS_PROVIDER_REFERENCE", PROVIDER)
            .env("LAYERX_HUMAN_KMS_STATE_DIR", self.root.0.join("state"))
            .env("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "10")
            .env_remove("LAYERX_HUMAN_KMS_ATTESTOR_CONFIG_FILE")
            .env_remove("LAYERX_HUMAN_KMS_EVM_CLIENT_CERT_DER");
        for (suffix, path) in [
            ("REGISTRY_FILE", self.root.0.join("snapshot.json")),
            ("SEAL_SECRET_FILE", self.root.0.join("seal")),
            ("CLIENT_CA_DER", self.pki.path("ca.der")),
            ("TLS_CERT_DER", self.pki.path("server.der")),
            ("TLS_KEY_DER", self.pki.path("server.pk8")),
            ("CLIENT_CERT_DER", self.pki.path("client.der")),
        ] {
            command.env(format!("LAYERX_HUMAN_KMS_{suffix}"), path);
        }
        let log = fs::File::create(self.root.0.join("kms.log"))?;
        self.child = Some(
            command
                .stdin(Stdio::null())
                .stdout(log.try_clone()?)
                .stderr(log)
                .spawn()?,
        );
        let start = Instant::now();
        loop {
            if self.remote()?.probe().is_ok() {
                return Ok(());
            }
            if self
                .child
                .as_mut()
                .ok_or("missing child")?
                .try_wait()?
                .is_some()
                || start.elapsed() > Duration::from_secs(15)
            {
                return Err(format!(
                    "real KMS startup failed: {}",
                    fs::read_to_string(self.root.0.join("kms.log"))?
                )
                .into());
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
    fn tls(&self) -> Result<MutualTlsConfig> {
        let mut roots = RootCertStore::empty();
        roots.add(CertificateDer::from(fs::read(self.pki.path("ca.der"))?))?;
        checked(MutualTlsConfig::new(
            roots,
            vec![CertificateDer::from(fs::read(self.pki.path("client.der"))?)],
            PrivateKeyDer::try_from(fs::read(self.pki.path("client.pk8"))?)?,
        ))
    }
    fn limits(&self) -> Limits {
        Limits {
            maximum_frame_bytes: MAX,
            maximum_connections: 4,
            maximum_streams: 1,
            maximum_queued_bytes: MAX,
            deadline: Duration::from_secs(10),
        }
    }
    fn remote(&self) -> Result<RemoteKmsProvider> {
        Ok(RemoteKmsProvider::new(
            PROVIDER,
            self.address,
            "127.0.0.1",
            self.tls()?,
            self.limits(),
            self.clock.clone(),
        )?)
    }
    fn custody(&self) -> Result<Keystore> {
        Ok(Keystore::open(
            self.root.0.join("custody"),
            125,
            self.remote()?,
        )?)
    }
}

#[test]
fn kms_producer_process() -> Result<()> {
    if std::env::var_os(CHILD).is_none() {
        return Ok(());
    }
    let clock: Arc<dyn Clock> = cluster::StepClock::new()?;
    layerx_human_kms::run_from_environment(&clock).map_err(Into::into)
}

#[test]
fn export_delivery_recovers_lost_reply_and_failed_marker() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "serialization poisoned")?;
    let mut host = KmsHost::new()?;
    let principal = principal("alice");
    let key = KeyId::new("primary")?;
    let custody = host.custody()?;
    let public = custody.create(&principal, &key, KeyClass::HumanPrimary)?;
    let binding = custody.evm_binding(&principal, &key)?;
    let reference = custody.evm_provider_reference(&principal, &key)?;
    assert!(custody.export_primary_once(&principal, &key).is_err());
    assert_eq!(custody.describe(&principal, &key)?.public_key, public);
    let recipient = ExportRecipient::new([0x61; 32])?;
    let ceremony = recipient.ceremony([0x62; 32])?;
    host.stop();
    assert!(custody
        .export_primary_envelope(&principal, &key, &ceremony)
        .is_err());
    let path = host
        .root
        .0
        .join("custody/exports/alice/primary/delivery.json");
    let pending: serde_json::Value = serde_json::from_slice(&fs::read(&path)?)?;
    assert_eq!(pending["ceremony"], serde_json::to_value(&ceremony)?);
    assert!(pending["envelope"].is_null());
    host.start()?;
    // Send the actual producer frame, then close before reading its reply.
    let remote = host.remote()?;
    let frame = remote.encode_export_request(&binding, &reference, &ceremony)?;
    let gate = ConnectionGate::new(1);
    let mut transport = checked(Tls::connect(
        host.address,
        "127.0.0.1".try_into()?,
        &host.tls()?,
        &gate,
        host.limits(),
    ))?;
    checked(transport.send(&frame))?;
    drop(transport);
    let produced = remote.export_envelope(&binding, &reference, &ceremony)?;
    drop(custody);
    host.stop();
    host.start()?;
    let custody = host.custody()?;
    let recovered = custody.export_primary_envelope(&principal, &key, &ceremony)?;
    assert_eq!(recovered, produced);
    assert_eq!(
        custody.export_primary_envelope(&principal, &key, &ceremony)?,
        produced
    );
    assert!(matches!(
        custody.export_primary_envelope(&principal, &key, &recipient.ceremony([0x63; 32])?),
        Err(CustodyError::Kms(KmsError::Conflict))
    ));
    let foreign = ExportRecipient::new([0x64; 32])?;
    assert!(foreign
        .open(PROVIDER, binding.digest(), reference.as_bytes(), &recovered)
        .is_err());
    let (secret, acknowledgement) =
        recipient.open(PROVIDER, binding.digest(), reference.as_bytes(), &recovered)?;
    let actual = ed25519_dalek::SigningKey::from_bytes(&secret)
        .verifying_key()
        .to_bytes();
    assert_eq!(actual, public);
    assert!(!fs::read(&path)?
        .windows(32)
        .any(|bytes| bytes == secret.as_slice()));
    let mut bad = acknowledgement.clone();
    bad.proof[0] ^= 1;
    assert!(custody
        .acknowledge_primary_export(&principal, &key, &bad)
        .is_err());
    let temporary = host.root.0.join("custody/principals/alice/primary.key.tmp");
    fs::create_dir(&temporary)?;
    assert!(custody
        .acknowledge_primary_export(&principal, &key, &acknowledgement)
        .is_err());
    assert!(!custody.self_custodied(&principal, &key)?);
    // The provider already acknowledged: recovery must now use the local encrypted copy.
    assert_eq!(
        remote.export_envelope(&binding, &reference, &ceremony),
        Err(KmsError::SelfCustodied)
    );
    fs::remove_dir(&temporary)?;
    drop(custody);
    host.stop();
    host.start()?;
    let custody = host.custody()?;
    let retained = custody.export_primary_envelope(&principal, &key, &ceremony)?;
    assert_eq!(retained, produced);
    assert_eq!(
        *recipient
            .open(PROVIDER, binding.digest(), reference.as_bytes(), &retained)?
            .0,
        *secret
    );
    custody.acknowledge_primary_export(&principal, &key, &acknowledgement)?;
    custody.acknowledge_primary_export(&principal, &key, &acknowledgement)?;
    assert!(custody.self_custodied(&principal, &key)?);
    assert!(matches!(
        custody.rotate(&principal, &key),
        Err(CustodyError::SelfCustodied)
    ));
    assert!(matches!(
        custody.export_primary_envelope(&principal, &key, &ceremony),
        Err(CustodyError::SelfCustodied)
    ));
    drop(custody);
    host.stop();
    host.start()?;
    assert!(host.custody()?.self_custodied(&principal, &key)?);
    Ok(())
}

#[test]
fn remote_native_admission_refuses_before_discovery() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "serialization poisoned")?;
    let host = KmsHost::new()?;
    host.remote()?.probe()?;
    let result =
        Keystore::open_production(host.root.0.join("refused-service"), 125, host.remote()?);
    let error =
        result.expect_err("a remote-only provider must not expose native access operations");
    assert_eq!(error.refusal_code(), "kms-unsupported-native-consent");
    assert!(matches!(
        error,
        CustodyError::Kms(KmsError::UnsupportedNativeConsent)
    ));
    assert!(!host.root.0.join("refused-service").exists());
    Ok(())
}

struct Ready;
impl Wake for Ready {
    fn wake(self: Arc<Self>) {}
}
fn ready<F: Future>(future: F) -> Result<F::Output> {
    let mut future = std::pin::pin!(future);
    let waker = Waker::from(Arc::new(Ready));
    match future.as_mut().poll(&mut Context::from_waker(&waker)) {
        Poll::Ready(value) => Ok(value),
        Poll::Pending => Err("synchronous real provider unexpectedly suspended".into()),
    }
}

#[test]
fn attestor_native_consent_still_signs() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "serialization poisoned")?;
    let mut cluster = cluster::Cluster::start(&[])?;
    let _environment = ClockEnvironment::install(&cluster.clock_authority.socket);
    let root = Root(scratch("attestor-consent-22-31-152")?);
    let principal = principal("alice");
    let key = KeyId::new("primary")?;
    let provider = AttestorKms::connect(AttestorCustodyConfig::new(
        cluster.nodes.clone(),
        cluster::SIGNERS
            .iter()
            .map(|value| (*value).into())
            .collect(),
        cluster.pki.certificate_der("node-ca")?,
        cluster.pki.certificate_der("gateway")?,
        cluster.pki.key_der("gateway")?,
        Duration::from_secs(60),
    )?)?;
    let keystore = Keystore::open_production(root.0.join("custody"), 125, provider.clone())?;
    let binding = keystore.binding(&principal, &key, KeyClass::HumanPrimary)?;
    let provider_key = format!("lx-{}", hex(&binding.digest()[..16]));
    cluster.approve_generate(&provider_key, "ed25519", "alice")?;
    let public = provider.create_owned_key(
        &keystore,
        &principal,
        &key,
        "alice",
        "0x1111111111111111111111111111111111111111",
    )?;
    cluster.approve_sign(&provider_key, &public)?;
    cluster::allow_preparation(&mut cluster, &provider_key, &public, "alice")?;
    provider.admit_assertion("alice", &cluster.tokens.mint("alice")?)?;
    let now = cluster.clock.now()?;
    let purpose = checked(
        NativePreparationPurposeV1 {
            tenant: checked(TenantId::new("alice"))?,
            agent_did: checked(AgentDid::new("did:layerx:alice"))?,
            session_id: checked(SessionId::new(hex(&[0x11; 32])))?,
            generation: 1,
            expires_at_ms: now * 1000 + 300000,
            capability_id: checked(CapabilityId::new(hex(&[0x22; 32])))?,
            preparation_id: [0x33; 32],
            canonical_digest: [0x44; 32],
            commitment: [0x55; 32],
        }
        .validate(),
    )?;
    let (store, _) = install_and_open(
        &root.0.join("store"),
        &tenancy(&[("alice", "tenant-a")]),
        retention_uniform(1000),
    );
    let shared = Arc::new(Mutex::new(store));
    let signer = CustodySigner::new_shared(
        keystore,
        shared.clone(),
        checked(ModuleRegistry::new(&[]))?,
        SigningLimits::new(32, 60)?,
    );
    let mut store = shared.lock().map_err(|_| "store poisoned")?;
    let mut scope = store.principal(&principal)?;
    let signed = ready(signer.sign_native_consent_in_scope(
        &mut scope,
        NativeConsentRequest::new(
            &principal,
            &key,
            NativeConsent::PreparationPurpose(&purpose),
            SignAuthorization::new(Operation::ProtocolMutation, None),
            now,
            TraceId::mint([0x66; 16]),
        ),
    ))??;
    let expected: [u8; 32] = Sha256::digest(checked(purpose.canonical_bytes())?).into();
    checked(layerx_crypto::ed25519::verify_digest(
        &public,
        signed.signature(),
        &expected,
    ))?;
    assert_eq!(signed.signer_public_key(), public);
    assert!(!scope.audit_keys().is_empty());
    Ok(())
}
