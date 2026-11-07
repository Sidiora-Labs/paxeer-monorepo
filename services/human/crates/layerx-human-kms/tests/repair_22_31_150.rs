//! Task 22.31.150 repair record, audit 22.30.102, acceptance 22998.1.
//! shared-wallet: shared_wallet_isolated_attestors_and_nonce_recovery.
//! export-delivery-loss: export_delivery_loss_recovers_before_and_after_publication.
//! signer-identity-split: signer_identity_create_describe_wallet_sign_public_path.
//! hosted-chain-snapshot: hosted_chain_snapshot_contract_and_retained_restart.
//! Baseline: not executed; ordered implementation/build/done gates prohibit an unchanged-tree verify.
//! Initial bounded verification failed both signing cases: the real attestor fixture did not authorize the exercised openAccount selector; repair provisions only that selector and preserves all assertions.
//! Wiring request (domain 8): services/platform/hosted/human/material.py::kms_prerequisite and material.sh must emit exactly {"chain_id":125} and reconcile retained registry.json without replacing the seal or custody volume.
//! Wiring request (domain 10): spec/layerx-merge/interfaces.kvx and contracts/services.md must publish chain-125 snapshot and recipient-encrypted LXKP v8 export/acknowledgement contracts.
//! Wiring request (wallet domain): services/wallet/attestor provisioning must publish one owner-bound EVM key and assertion per tenant/principal binding; existing shared-wallet state requires explicit custody reconciliation.
//! Wiring request (Human domain): services/human/crates/layerx-human-service/src/custody/mod.rs::export_primary_once and custody/gateway.rs must route the v8 ceremony, retain recipient secrets, and persist the received key before acknowledge_export; the raw-seed API now explicitly refuses.
//! Scope forbids edits to spec task records; this test module retains the finding/test and wiring record.

#[allow(dead_code)]
#[path = "support/attestor_cluster.rs"]
mod attestor_cluster;

use attestor_cluster::{checked, hex, openssl, scratch, Cluster, Result};
use layerx_client::frame::framing::write_frame;
use layerx_crypto::disclosure::call::bind_call;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
use layerx_crypto::signer::AgentSignature;
use layerx_human_kms::export::{ExportAcknowledgement, ExportRecipient};
use layerx_human_service::custody::{
    EvmAction, EvmPlanAuthorization, EvmTransaction, KeyClass, KmsError, KmsProvider,
    PrincipalKeyBinding, ProviderKeyDescription, RemoteKmsProvider,
};
use layerx_human_service::frame::{Limits, MutualTlsConfig};
use layerx_types::clock::Deadline;
use ring::signature::KeyPair;
use rustls::pki_types::{CertificateDer, PrivateKeyDer, ServerName};
use rustls::{ClientConfig, ClientConnection, RootCertStore, StreamOwned};
use serde_json::{json, Value};
use sha3::{Digest, Keccak256};
use std::fs;
use std::future::Future;
use std::io::Write;
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::{Arc, Mutex};
use std::task::{Context, Poll, Wake, Waker};
use std::time::Duration;

const PROVIDER: &str = "repair-22-31-150";
const MAX: usize = 2_097_152;
static SERIAL: Mutex<()> = Mutex::new(());
const OWNERS: [(&str, &str); 3] = [
    ("tenant-a", "alice"),
    ("tenant-a", "bob"),
    ("tenant-b", "alice"),
];

struct Host {
    child: Option<Child>,
    root: PathBuf,
    address: SocketAddr,
    cluster: Cluster,
    bindings: Vec<PrincipalKeyBinding>,
    addresses: Vec<[u8; 20]>,
}
impl Drop for Host {
    fn drop(&mut self) {
        self.stop();
        let _ = fs::remove_dir_all(&self.root);
    }
}
impl Host {
    fn new(wallets: usize) -> Result<Self> {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let root = scratch("repair-22-31-150")?;
        let selector = format!(
            "0x{}",
            hex(&layerx_crypto::disclosure::call::ledger_selector(
                "openAccount(bytes32)"
            ))
        );
        let mut cluster = Cluster::start_with_policies(
            &[],
            &json!({"version":1,"defaults":{
                "chain_id":125,"kinds":["evm_tx"],
                "selectors":{"0x000000000000000000000000000000000000101a":[selector]},
                "caps":{"native":{"per_transaction":"1000000000000000000","daily":"10000000000000000000"}},
                "rate_per_minute":1000
            }}),
            &json!({"version":1,"defaults":{"modules":{}}}),
        )?;
        cluster.pki.leaf("node-ca", "kms", true)?;
        cluster.pki.leaf("gateway-ca", "service", false)?;
        for name in ["node-ca", "gateway-ca", "gateway", "kms", "service"] {
            cluster.pki.certificate_der(name)?;
        }
        for name in ["gateway", "kms", "service"] {
            cluster.pki.key_der(name)?;
            fs::set_permissions(
                cluster.pki.path(&format!("{name}.pk8")),
                fs::Permissions::from_mode(0o600),
            )?;
        }
        let mut bindings = Vec::new();
        let mut addresses = Vec::new();
        let mut rows = Vec::new();
        if wallets != 0 {
            let public = cluster.generate_key(
                "auxiliary-native",
                "ed25519",
                "auxiliary",
                "0x1111111111111111111111111111111111111111",
            )?;
            for (index, (tenant, principal)) in OWNERS.iter().take(wallets).enumerate() {
                // The actual producer includes tenant in the canonical identity it binds.
                let identity = format!(
                    "layerx-human-custody/v1\0{tenant}/{principal}\0primary\0human-primary"
                );
                let binding = PrincipalKeyBinding::new(
                    identity.into_bytes(),
                    125,
                    KeyClass::HumanPrimary,
                    PROVIDER,
                )?;
                let owner = format!("{tenant}-{principal}");
                let key_id = format!("evm-{index}");
                let public_evm = cluster.generate_key(&key_id, "secp256k1", &owner, "")?;
                let point = k256::ecdsa::VerifyingKey::from_sec1_bytes(&public_evm)?
                    .to_encoded_point(false);
                let address: [u8; 20] =
                    Keccak256::digest(&point.as_bytes()[1..])[12..].try_into()?;
                let assertion_file = root.join(format!("owner-{index}.jwt"));
                fs::write(&assertion_file, cluster.tokens.mint(&owner)?)?;
                let nodes: Vec<_> = cluster
                    .nodes
                    .iter()
                    .map(|(id, address)| json!({"id":id,"address":address}))
                    .collect();
                rows.push(
                    json!({"binding":binding.digest(), "tenant":tenant, "principal":principal,
                    "assertion_file":assertion_file, "config":{
                        "nodes":nodes, "trust_anchors":[cluster.pki.path("node-ca.der")],
                        "client_certificate_chain":[cluster.pki.path("gateway.der")],
                        "client_private_key_pkcs8":cluster.pki.path("gateway.pk8"),
                        "key_id":"auxiliary-native", "public_key":public,
                        "evm_key_id":key_id, "evm_address":address,
                        "signers":attestor_cluster::SIGNERS, "network_id":125, "timeout_seconds":60
                    }}),
                );
                bindings.push(binding);
                addresses.push(address);
            }
        }
        fs::write(
            root.join("attestors.json"),
            serde_json::to_vec(&json!({"bindings":rows}))?,
        )?;
        let mut seal = [0; 32];
        getrandom::fill(&mut seal)?;
        fs::write(root.join("seal"), seal)?;
        fs::write(root.join("snapshot.json"), br#"{"chain_id":125}"#)?;
        for entry in fs::read_dir(&root)? {
            fs::set_permissions(entry?.path(), fs::Permissions::from_mode(0o600))?;
        }
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        drop(listener);
        let mut host = Self {
            child: None,
            root,
            address,
            cluster,
            bindings,
            addresses,
        };
        host.start()?;
        Ok(host)
    }
    fn command(&self) -> Result<Command> {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-human-kms"));
        command
            .env("LAYERX_HUMAN_KMS_LISTEN", self.address.to_string())
            .env("LAYERX_HUMAN_KMS_PROVIDER_REFERENCE", PROVIDER)
            .env("LAYERX_HUMAN_KMS_STATE_DIR", self.root.join("state"))
            .env("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "60")
            .env_remove("LAYERX_HUMAN_KMS_EVM_CLIENT_CERT_DER")
            .env_remove("LAYERX_HUMAN_KMS_EVM_ASSERTION_FILE")
            .env(
                "LAYERX_RUNTIME_CLOCK_SOCKET",
                &self.cluster.clock_authority.socket,
            )
            .env("LAYERX_RUNTIME_CLOCK_PID", std::process::id().to_string())
            .env(
                "LAYERX_RUNTIME_CLOCK_UID",
                rustix::process::geteuid().as_raw().to_string(),
            );
        for (suffix, path) in [
            ("REGISTRY_FILE", self.root.join("snapshot.json")),
            ("SEAL_SECRET_FILE", self.root.join("seal")),
            ("ATTESTOR_CONFIG_FILE", self.root.join("attestors.json")),
            ("CLIENT_CA_DER", self.cluster.pki.path("gateway-ca.der")),
            ("TLS_CERT_DER", self.cluster.pki.path("kms.der")),
            ("TLS_KEY_DER", self.cluster.pki.path("kms.pk8")),
            ("CLIENT_CERT_DER", self.cluster.pki.path("service.der")),
        ] {
            command.env(format!("LAYERX_HUMAN_KMS_{suffix}"), path);
        }
        command.stdin(Stdio::null());
        Ok(command)
    }
    fn start(&mut self) -> Result<()> {
        let log = fs::File::create(self.root.join("kms.log"))?;
        self.child = Some(
            self.command()?
                .stdout(log.try_clone()?)
                .stderr(log)
                .spawn()?,
        );
        let mut deadline = Deadline::start(self.cluster.clock.as_ref(), Duration::from_secs(15))?;
        loop {
            if self.remote()?.probe().is_ok() {
                return Ok(());
            }
            if self
                .child
                .as_mut()
                .ok_or("missing KMS")?
                .try_wait()?
                .is_some()
                || deadline.remaining(self.cluster.clock.as_ref())?.is_zero()
            {
                return Err(format!(
                    "KMS start failed: {}",
                    fs::read_to_string(self.root.join("kms.log"))?
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
    fn roots(&self) -> Result<RootCertStore> {
        let mut roots = RootCertStore::empty();
        roots.add(CertificateDer::from(fs::read(
            self.cluster.pki.path("node-ca.der"),
        )?))?;
        Ok(roots)
    }
    fn remote(&self) -> Result<RemoteKmsProvider> {
        let tls = checked(MutualTlsConfig::new(
            self.roots()?,
            vec![CertificateDer::from(fs::read(
                self.cluster.pki.path("service.der"),
            )?)],
            PrivateKeyDer::try_from(fs::read(self.cluster.pki.path("service.pk8"))?)?,
        ))?;
        Ok(RemoteKmsProvider::new(
            PROVIDER,
            self.address,
            "127.0.0.1",
            tls,
            Limits {
                maximum_frame_bytes: MAX,
                maximum_connections: 4,
                maximum_streams: 1,
                maximum_queued_bytes: MAX,
                deadline: Duration::from_secs(60),
            },
            self.cluster.clock.clone(),
        )?)
    }
    fn tls(&self) -> Result<StreamOwned<ClientConnection, TcpStream>> {
        let config = ClientConfig::builder()
            .with_root_certificates(self.roots()?)
            .with_client_auth_cert(
                vec![CertificateDer::from(fs::read(
                    self.cluster.pki.path("service.der"),
                )?)],
                PrivateKeyDer::try_from(fs::read(self.cluster.pki.path("service.pk8"))?)?,
            )?;
        let tcp = TcpStream::connect(self.address)?;
        tcp.set_read_timeout(Some(Duration::from_secs(60)))?;
        tcp.set_write_timeout(Some(Duration::from_secs(60)))?;
        Ok(StreamOwned::new(
            ClientConnection::new(
                Arc::new(config),
                ServerName::IpAddress(self.address.ip().into()),
            )?,
            tcp,
        ))
    }
    fn create(&self, index: usize) -> Result<ProviderKeyDescription> {
        Ok(self.remote()?.create_key(&self.bindings[index])?)
    }
    fn hosted_material(&self) -> Result<PathBuf> {
        let tls_root = self.root.join("hosted-tls");
        fs::create_dir(&tls_root)?;
        fs::set_permissions(&tls_root, fs::Permissions::from_mode(0o700))?;
        let ca = self.cluster.pki.path("node-ca.pem");
        let ca_key = self.cluster.pki.path("node-ca.key");
        for (index, (role, name, usage, sans)) in [
            (
                "human-kms",
                "layerx-human-kms",
                "serverAuth",
                "subjectAltName=DNS:layerx-human-kms,IP:127.0.0.1\n",
            ),
            (
                "human-kms-client",
                "layerx-human-components",
                "clientAuth",
                "",
            ),
            (
                "human-kms-executor",
                "layerx-human-movement",
                "clientAuth",
                "",
            ),
        ]
        .iter()
        .enumerate()
        {
            let directory = tls_root.join(role);
            fs::create_dir(&directory)?;
            fs::set_permissions(&directory, fs::Permissions::from_mode(0o700))?;
            openssl(
                &directory,
                &[
                    "req",
                    "-new",
                    "-newkey",
                    "ec",
                    "-pkeyopt",
                    "ec_paramgen_curve:prime256v1",
                    "-nodes",
                    "-keyout",
                    "key.pem",
                    "-out",
                    "cert.csr",
                    "-subj",
                    &format!("/CN={name}"),
                ],
            )?;
            fs::write(directory.join("cert.ext"),format!("basicConstraints=CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage={usage}\n{sans}"))?;
            openssl(
                &directory,
                &[
                    "x509",
                    "-req",
                    "-in",
                    "cert.csr",
                    "-CA",
                    ca.to_str().ok_or("CA path")?,
                    "-CAkey",
                    ca_key.to_str().ok_or("CA key path")?,
                    "-set_serial",
                    &(100 + index).to_string(),
                    "-days",
                    "1",
                    "-sha256",
                    "-extfile",
                    "cert.ext",
                    "-out",
                    "cert.pem",
                ],
            )?;
            openssl(
                &directory,
                &[
                    "x509", "-in", "cert.pem", "-outform", "DER", "-out", "cert.der",
                ],
            )?;
            openssl(
                &directory,
                &[
                    "pkcs8", "-topk8", "-nocrypt", "-in", "key.pem", "-outform", "DER", "-out",
                    "key.der",
                ],
            )?;
            fs::copy(
                self.cluster.pki.path("node-ca.der"),
                directory.join("ca.der"),
            )?;
            for entry in fs::read_dir(&directory)? {
                fs::set_permissions(entry?.path(), fs::Permissions::from_mode(0o600))?;
            }
        }
        let registry = self.root.join("native-registry.json");
        let asset = hex(&[4; 32]);
        fs::write(
            &registry,
            serde_json::to_vec(
                &json!({"schema_version":2,"assets":[{"asset":asset}],"modules":[{"module":1,"ordinals":[1]}]}),
            )?,
        )?;
        fs::set_permissions(&registry, fs::Permissions::from_mode(0o600))?;
        let producer = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("../../../platform/hosted/human/material.py");
        let destination = self.root.join("hosted-kms");
        let produce = || -> Result<()> {
            let output = Command::new("python3")
                .env("PYTHONDONTWRITEBYTECODE", "1")
                .arg(&producer)
                .arg("--kms-prerequisite")
                .arg(&registry)
                .arg(&tls_root)
                .arg(&destination)
                .arg("125")
                .arg(&asset)
                .arg(self.root.join("hosted-state"))
                .output()?;
            assert!(
                output.status.success(),
                "hosted producer: {}",
                String::from_utf8_lossy(&output.stderr)
            );
            Ok(())
        };
        produce()?;
        let seal = fs::read(destination.join("kms-seal"))?;
        let published = fs::read(destination.join("registry.json"))?;
        produce()?;
        assert_eq!(fs::read(destination.join("kms-seal"))?, seal);
        assert_eq!(fs::read(destination.join("registry.json"))?, published);
        Ok(destination)
    }
    fn authorization(&self, index: usize, nonce: u64, action: u8) -> Result<EvmPlanAuthorization> {
        let now = self.cluster.clock.now()?;
        Ok(EvmPlanAuthorization {
            plan_id: [1; 32],
            action_key: [action; 32],
            tenant: OWNERS[index].0.into(),
            principal: OWNERS[index].1.into(),
            binding_digest: self.bindings[index].digest(),
            wallet: self.addresses[index],
            not_before: now - 5,
            not_after: now + 600,
            transaction: EvmTransaction {
                chain_id: 125,
                nonce,
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 2,
                gas_limit: 21000,
                to: [4; 20],
                value: [0; 32],
                calldata: Vec::new(),
            },
        })
    }
}
struct ThreadWake(std::thread::Thread);
impl Wake for ThreadWake {
    fn wake(self: Arc<Self>) {
        self.0.unpark();
    }
}
fn run<F: Future>(future: F) -> F::Output {
    let waker = Waker::from(Arc::new(ThreadWake(std::thread::current())));
    let mut context = Context::from_waker(&waker);
    let mut future = std::pin::pin!(future);
    loop {
        match future.as_mut().poll(&mut context) {
            Poll::Ready(output) => return output,
            Poll::Pending => std::thread::park(),
        }
    }
}
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
        gas_limit: 100000,
        to,
        data,
    }
}

#[test]
fn shared_wallet_isolated_attestors_and_nonce_recovery() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "test serialization poisoned")?;
    let mut host = Host::new(3)?;
    let remote = host.remote()?;
    let mut descriptions = Vec::new();
    let mut signed = Vec::new();
    for index in 0..3 {
        let description = host.create(index)?;
        let descriptor =
            remote.wallet_descriptor(&host.bindings[index], description.reference())?;
        assert_eq!(descriptor.native_ed25519, description.public_key());
        assert_eq!(descriptor.evm_secp256k1, host.addresses[index]);
        let authorization = host.authorization(index, 0, 2)?;
        let action: EvmAction = serde_json::from_slice(&remote.evm_operation(
            7,
            &host.bindings[index],
            description.reference(),
            &serde_json::to_vec(&authorization)?,
        )?)?;
        assert_eq!(action.authorization, authorization);
        let bytes = remote.evm_operation(
            8,
            &host.bindings[index],
            description.reference(),
            &serde_json::to_vec(&authorization.action_key)?,
        )?;
        let action: EvmAction = serde_json::from_slice(&bytes)?;
        let (transaction, signature) = decode_signed(&action.raw_transaction)?;
        assert_eq!(
            recover_sender(&transaction, &signature)?,
            host.addresses[index]
        );
        signed.push(action);
        descriptions.push(description);
    }
    assert_eq!(
        host.addresses
            .iter()
            .collect::<std::collections::BTreeSet<_>>()
            .len(),
        3
    );
    // Wrong principal and wrong tenant cannot use the other principal's handle.
    for index in [1, 2] {
        assert!(remote
            .evm_operation(6, &host.bindings[index], descriptions[0].reference(), &[])
            .is_err());
    }
    host.stop();
    host.start()?;
    let remote = host.remote()?;
    for index in 0..3 {
        let binding = &host.bindings[index];
        let reference = descriptions[index].reference();
        assert_eq!(
            remote.wallet_descriptor(binding, reference)?.evm_secp256k1,
            host.addresses[index]
        );
        let recovered: EvmAction = serde_json::from_slice(&remote.evm_operation(
            8,
            binding,
            reference,
            &serde_json::to_vec(&[2u8; 32])?,
        )?)?;
        assert_eq!(recovered, signed[index]);
        let duplicate = host.authorization(index, 0, 3)?;
        assert_eq!(
            remote.evm_operation(7, binding, reference, &serde_json::to_vec(&duplicate)?),
            Err(KmsError::Conflict)
        );
        let next = host.authorization(index, 1, 4)?;
        assert!(remote
            .evm_operation(7, binding, reference, &serde_json::to_vec(&next)?)
            .is_ok());
        let mut cross_tenant = host.authorization(index, 2, 5)?;
        cross_tenant.tenant = "unbound-tenant".into();
        assert_eq!(
            remote.evm_operation(7, binding, reference, &serde_json::to_vec(&cross_tenant)?),
            Err(KmsError::Refused)
        );
    }
    // A configured address or key alias is refused at startup, rather than sharing control.
    host.stop();
    let original = fs::read(host.root.join("attestors.json"))?;
    let mut manifest: Value = serde_json::from_slice(&original)?;
    manifest["bindings"][1]["config"]["evm_address"] =
        manifest["bindings"][0]["config"]["evm_address"].clone();
    fs::write(
        host.root.join("attestors.json"),
        serde_json::to_vec(&manifest)?,
    )?;
    let output = host.command()?.output()?;
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("custody isolation refused"));
    let mut wrong_owner: Value = serde_json::from_slice(&original)?;
    wrong_owner["bindings"][0]["assertion_file"] =
        wrong_owner["bindings"][1]["assertion_file"].clone();
    fs::write(
        host.root.join("attestors.json"),
        serde_json::to_vec(&wrong_owner)?,
    )?;
    host.start()?;
    let remote = host.remote()?;
    let call = call(2);
    let disclosure = checked(bind_call(&call, host.addresses[0]))?;
    assert!(run(remote.sign_wallet_call(
        &host.bindings[0],
        descriptions[0].reference(),
        &call,
        &disclosure
    ))
    .is_err());
    host.stop();
    fs::write(host.root.join("attestors.json"), original)?;
    host.start()?;
    let signature = run(host.remote()?.sign_wallet_call(
        &host.bindings[0],
        descriptions[0].reference(),
        &call,
        &disclosure,
    ))?;
    assert_eq!(
        recover_sender(&call, &checked(AgentSignature::new(signature))?)?,
        host.addresses[0]
    );
    Ok(())
}

#[test]
fn signer_identity_create_describe_wallet_sign_public_path() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "test serialization poisoned")?;
    let mut host = Host::new(1)?;
    let description = host.create(0)?;
    let binding = host.bindings[0].clone();
    let remote = host.remote()?;
    assert_eq!(
        remote.describe_key(&binding, description.reference())?,
        description
    );
    let descriptor = remote.wallet_descriptor(&binding, description.reference())?;
    let call = call(7);
    let disclosure = checked(bind_call(&call, descriptor.evm_secp256k1))?;
    let signature =
        run(remote.sign_wallet_call(&binding, description.reference(), &call, &disclosure))?;
    let signature = checked(AgentSignature::new(signature))?;
    assert_eq!(recover_sender(&call, &signature)?, descriptor.evm_secp256k1);
    let mut foreign = descriptor.evm_secp256k1;
    foreign[0] ^= 1;
    let foreign_disclosure = checked(bind_call(&call, foreign))?;
    assert!(run(remote.sign_wallet_call(
        &binding,
        description.reference(),
        &call,
        &foreign_disclosure
    ))
    .is_err());
    host.stop();
    host.start()?;
    let remote = host.remote()?;
    assert_eq!(
        remote.wallet_descriptor(&binding, description.reference())?,
        descriptor
    );
    assert!(
        run(remote.sign_wallet_call(&binding, description.reference(), &call, &disclosure)).is_ok()
    );
    let mut conflicting = call.clone();
    conflicting.data[4] ^= 1;
    let disclosure = checked(bind_call(&conflicting, descriptor.evm_secp256k1))?;
    assert!(run(remote.sign_wallet_call(
        &binding,
        description.reference(),
        &conflicting,
        &disclosure
    ))
    .is_err());
    // The operation-5 reservation also constrains the plan-signing path.
    let authorization = host.authorization(0, 7, 8)?;
    assert_eq!(
        remote.evm_operation(
            7,
            &binding,
            description.reference(),
            &serde_json::to_vec(&authorization)?
        ),
        Err(KmsError::Conflict)
    );
    Ok(())
}

#[test]
fn export_delivery_loss_recovers_before_and_after_publication() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "test serialization poisoned")?;
    let mut host = Host::new(1)?;
    let description = host.create(0)?;
    let binding = host.bindings[0].clone();
    let remote = host.remote()?;
    let recipient = ExportRecipient::new([11; 32])?;
    let ceremony = recipient.ceremony([9; 32])?;
    let frame = remote.encode_export_request(&binding, description.reference(), &ceremony)?;
    // A TLS disconnect in a partial frame must leave the managed record intact.
    {
        let mut tls = host.tls()?;
        tls.write_all(&u32::try_from(frame.len())?.to_be_bytes())?;
        tls.write_all(&frame[..frame.len() / 2])?;
        tls.flush()?;
    }
    host.stop();
    host.start()?;
    let remote = host.remote()?;
    assert_eq!(
        remote.describe_key(&binding, description.reference())?,
        description
    );
    assert!(remote
        .wallet_descriptor(&binding, description.reference())
        .is_ok());
    // Send the real provider's complete frame and disconnect without reading the response.
    {
        let mut tls = host.tls()?;
        checked(write_frame(&mut tls, &frame, MAX))?;
        tls.flush()?;
        // The metadata operation is a barrier for durable publication, independent of reply delivery.
        let mut deadline = Deadline::start(host.cluster.clock.as_ref(), Duration::from_secs(15))?;
        loop {
            if remote.evm_operation(6, &binding, description.reference(), &[])
                == Err(KmsError::SelfCustodied)
            {
                break;
            }
            if deadline.remaining(host.cluster.clock.as_ref())?.is_zero() {
                return Err("export publication deadline".into());
            }
            std::thread::sleep(Duration::from_millis(10));
        }
        drop(tls);
    }
    host.stop();
    host.start()?;
    let remote = host.remote()?;
    let envelope = remote.export_envelope(&binding, description.reference(), &ceremony)?;
    assert_eq!(
        remote.export_envelope(&binding, description.reference(), &ceremony)?,
        envelope
    );
    assert_eq!(
        remote.evm_operation(6, &binding, description.reference(), &[]),
        Err(KmsError::SelfCustodied)
    );
    assert_eq!(remote.create_key(&binding), Err(KmsError::Conflict));
    assert_eq!(
        remote.export_primary_key(&binding, description.reference()),
        Err(KmsError::Refused)
    );
    let wrong = ExportRecipient::new([12; 32])?;
    assert!(wrong
        .open(
            PROVIDER,
            binding.digest(),
            description.reference().as_bytes(),
            &envelope
        )
        .is_err());
    assert!(recipient
        .open(
            "other-provider",
            binding.digest(),
            description.reference().as_bytes(),
            &envelope
        )
        .is_err());
    assert!(recipient
        .open(
            PROVIDER,
            [1; 32],
            description.reference().as_bytes(),
            &envelope
        )
        .is_err());
    assert!(recipient
        .open(PROVIDER, binding.digest(), b"different-handle", &envelope)
        .is_err());
    let mut tampered = envelope.clone();
    tampered.ciphertext[0] ^= 1;
    assert!(recipient
        .open(
            PROVIDER,
            binding.digest(),
            description.reference().as_bytes(),
            &tampered
        )
        .is_err());
    assert_eq!(
        remote.export_envelope(
            &binding,
            description.reference(),
            &recipient.ceremony([8; 32])?
        ),
        Err(KmsError::Conflict)
    );
    assert_eq!(
        remote.export_envelope(
            &binding,
            description.reference(),
            &wrong.ceremony(ceremony.id)?
        ),
        Err(KmsError::Conflict)
    );
    let (seed, acknowledgement) = recipient.open(
        PROVIDER,
        binding.digest(),
        description.reference().as_bytes(),
        &envelope,
    )?;
    assert_eq!(
        checked(ring::signature::Ed25519KeyPair::from_seed_unchecked(&*seed))?
            .public_key()
            .as_ref(),
        description.public_key()
    );
    let invalid_ack = ExportAcknowledgement {
        ceremony: ceremony.clone(),
        proof: [0; 32],
    };
    assert_eq!(
        remote.acknowledge_export(&binding, description.reference(), &invalid_ack),
        Err(KmsError::Refused)
    );
    // Retained envelope and recipient secret recover independently after another restart.
    host.stop();
    host.start()?;
    let remote = host.remote()?;
    assert_eq!(
        remote.export_envelope(&binding, description.reference(), &ceremony)?,
        envelope
    );
    let restored_recipient = ExportRecipient::new([11; 32])?;
    let (recovered, _) = restored_recipient.open(
        PROVIDER,
        binding.digest(),
        description.reference().as_bytes(),
        &envelope,
    )?;
    assert!(layerx_crypto::ct::eq_fixed(&*seed, &*recovered));
    let acknowledgement_frame = remote.encode_export_acknowledgement(
        &binding,
        description.reference(),
        &acknowledgement,
    )?;
    {
        let mut tls = host.tls()?;
        checked(write_frame(&mut tls, &acknowledgement_frame, MAX))?;
        tls.flush()?;
        let mut deadline = Deadline::start(host.cluster.clock.as_ref(), Duration::from_secs(15))?;
        loop {
            if remote.export_envelope(&binding, description.reference(), &ceremony)
                == Err(KmsError::SelfCustodied)
            {
                break;
            }
            if deadline.remaining(host.cluster.clock.as_ref())?.is_zero() {
                return Err("export acknowledgement deadline".into());
            }
            std::thread::sleep(Duration::from_millis(10));
        }
        // Drop the acknowledgement reply too; the recipient can safely repeat it.
        drop(tls);
    }
    host.stop();
    host.start()?;
    let remote = host.remote()?;
    remote.acknowledge_export(&binding, description.reference(), &acknowledgement)?;
    assert_eq!(
        remote.export_envelope(&binding, description.reference(), &ceremony),
        Err(KmsError::SelfCustodied)
    );
    assert_eq!(
        remote.evm_operation(6, &binding, description.reference(), &[]),
        Err(KmsError::SelfCustodied)
    );
    Ok(())
}

#[test]
fn hosted_chain_snapshot_contract_and_retained_restart() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "test serialization poisoned")?;
    let mut host = Host::new(0)?;
    let binding = PrincipalKeyBinding::new(
        b"retained-native-key".to_vec(),
        125,
        KeyClass::HumanPrimary,
        PROVIDER,
    )?;
    let description = host.remote()?.create_key(&binding)?;
    let seal = fs::read(host.root.join("seal"))?;
    host.stop();
    // Exercise the actual hosted producer, including retained-material reopening.
    let material = host.hosted_material()?;
    let published = fs::read(material.join("registry.json"))?;
    let document: Value = serde_json::from_slice(&published)?;
    assert_eq!(document["network_id"], 125);
    assert_eq!(document["protocol_version"], 3);
    fs::write(host.root.join("snapshot.json"), published)?;
    let output = host.command()?.output()?;
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("chain snapshot invalid"));
    assert_eq!(fs::read(host.root.join("seal"))?, seal);
    fs::write(host.root.join("snapshot.json"), br#"{"chain_id":125}"#)?;
    host.start()?;
    assert_eq!(
        host.remote()?
            .describe_key(&binding, description.reference())?,
        description
    );
    host.stop();
    host.start()?;
    assert_eq!(
        host.remote()?
            .describe_key(&binding, description.reference())?,
        description
    );
    assert_eq!(fs::read(host.root.join("seal"))?, seal);
    Ok(())
}
