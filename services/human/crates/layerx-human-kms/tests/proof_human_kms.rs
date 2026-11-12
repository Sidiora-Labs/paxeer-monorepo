//! KMS configuration binds protected material to the chain and signer identity.

#[allow(dead_code)]
#[path = "../src/config.rs"]
mod config;
#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod runtime_clock_server;

use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_human_kms::attestor::{
    decode_unsigned_evm_transaction, unsigned_evm_transaction, AttestorError, AttestorNodeConfig,
    AttestorSigner, AttestorSignerConfig,
};
use layerx_human_service::custody::{
    KeyClass, KmsError, KmsProvider, PrincipalKeyBinding, ProviderKeyReference, RemoteKmsProvider,
};
use layerx_human_service::frame::{Limits, MutualTlsConfig};
use layerx_types::clock::Clock;
use rustls::pki_types::{CertificateDer, PrivateKeyDer};
use rustls::RootCertStore;
use serde_json::json;
use sha2::{Digest, Sha256};
use std::error::Error;
use std::fs;
use std::net::{SocketAddr, TcpListener};
use std::os::unix::fs::{DirBuilderExt, PermissionsExt};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::Duration;

type Result<T> = std::result::Result<T, Box<dyn Error>>;
const PROVIDER: &str = "layerx-human-kms";
const MAX_FRAME: usize = 2_097_152;
const CONFIG_CHECK: &str = "LAYERX_PROOF_KMS_CONFIG_CHECK";
static FIXTURE_COUNTER: AtomicUsize = AtomicUsize::new(0);
const ENTRYPOINT: &str = include_str!("../../../../../docker/human-service/entrypoint.sh");
const KERNEL_INIT: &str = include_str!("../../../../../docker/kernel/init.sh");

fn shell_function(source: &str, name: &str) -> Result<String> {
    let marker = format!("{name}() {{\n");
    let body = source
        .split_once(&marker)
        .ok_or_else(|| format!("missing producer {name}"))?
        .1;
    let body = body
        .split_once("\n}\n")
        .ok_or("unterminated producer function")?
        .0;
    Ok(format!("{marker}{body}\n}}\n"))
}

struct Host {
    root: PathBuf,
    address: SocketAddr,
    child: Option<Child>,
    clock_socket: PathBuf,
    clock_stop: Arc<AtomicBool>,
    clock_worker: Option<JoinHandle<std::io::Result<()>>>,
    clock: Arc<dyn Clock>,
}

impl Drop for Host {
    fn drop(&mut self) {
        self.stop();
        self.clock_stop.store(true, Ordering::Release);
        if let Some(worker) = self.clock_worker.take() {
            let _ = worker.join();
        }
        let _ = fs::remove_dir_all(&self.root);
    }
}

impl Host {
    fn new() -> Result<Self> {
        let root = PathBuf::from("/tmp").join(format!(
            "proof_human_kms-{}-{}",
            std::process::id(),
            FIXTURE_COUNTER.fetch_add(1, Ordering::Relaxed)
        ));
        fs::DirBuilder::new().mode(0o700).create(&root)?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let mut server = runtime_clock_server::Server::bind(&root)?;
        let clock_socket = server.socket().to_path_buf();
        let clock_stop = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&clock_stop);
        let clock_worker = thread::spawn(move || {
            while !stop.load(Ordering::Acquire) {
                server.poll()?;
                thread::sleep(Duration::from_millis(2));
            }
            Ok(())
        });
        let clock = RuntimeClock::connect(ClockBinding {
            socket: clock_socket.clone(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        })?;
        let host = Self {
            root,
            address,
            child: None,
            clock_socket,
            clock_stop,
            clock_worker: Some(clock_worker),
            clock,
        };
        host.provision()?;
        Ok(host)
    }

    fn openssl(&self, args: &[&str]) -> Result<()> {
        let output = Command::new("openssl")
            .current_dir(&self.root)
            .args(args)
            .output()?;
        assert!(
            output.status.success(),
            "TLS producer openssl failed: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        Ok(())
    }

    fn provision(&self) -> Result<()> {
        self.openssl(&[
            "req",
            "-x509",
            "-newkey",
            "ec",
            "-pkeyopt",
            "ec_paramgen_curve:P-256",
            "-nodes",
            "-keyout",
            "ca.key",
            "-out",
            "ca.pem",
            "-days",
            "1",
            "-subj",
            "/CN=proof-kms-ca",
            "-addext",
            "basicConstraints=critical,CA:TRUE",
        ])?;
        self.openssl(&["x509", "-in", "ca.pem", "-outform", "DER", "-out", "ca.der"])?;
        for (index, (role, cn, server)) in [
            ("human-kms", "layerx-human-kms", true),
            ("human-kms-client", "layerx-human-components", false),
            ("human-kms-executor", "layerx-human-movement", false),
            ("foreign", "unrecognized-client", false),
        ]
        .into_iter()
        .enumerate()
        {
            self.provision_leaf(index, role, cn, server)?;
        }
        fs::set_permissions(self.root.join("ca.der"), fs::Permissions::from_mode(0o600))?;
        fs::write(
            self.root.join("native-registry.json"),
            serde_json::to_vec(&json!({
                "schema_version": 2, "assets": [{"asset": "ab".repeat(32)}],
                "modules": [{"module": 1, "ordinals": [1, 4]}]
            }))?,
        )?;
        fs::set_permissions(
            self.root.join("native-registry.json"),
            fs::Permissions::from_mode(0o600),
        )?;
        fs::DirBuilder::new()
            .mode(0o700)
            .create(self.root.join("prerequisite"))?;
        let producer = Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("../../../platform/hosted/human/material.py");
        // Invoke the repository producer, including its protected-file, TLS and retained-manifest checks.
        let output = Command::new("python3").args(["-B", "-c", 
            "import runpy,sys; from pathlib import Path; sys.path.insert(0,str(Path(sys.argv[1]).parent)); m=runpy.run_path(sys.argv[1]); r=Path(sys.argv[2]); m['kms_prerequisite'](r/'native-registry.json',r,r/'prerequisite/material',125,'ab'*32,r/'state')"])
            .arg(producer).arg(&self.root).output()?;
        assert!(
            output.status.success(),
            "services/platform/hosted/human/material.py::kms_prerequisite failed: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        let material = self.root.join("prerequisite/material");
        for entry in fs::read_dir(&material)? {
            let entry = entry?;
            fs::copy(entry.path(), self.root.join(entry.file_name()))?;
            fs::set_permissions(
                self.root.join(entry.file_name()),
                fs::Permissions::from_mode(0o600),
            )?;
        }
        Ok(())
    }

    fn provision_leaf(&self, index: usize, role: &str, cn: &str, server: bool) -> Result<()> {
        let dir = self.root.join(role);
        fs::DirBuilder::new().mode(0o700).create(&dir)?;
        let key = format!("{role}/key.pem");
        let csr = format!("{role}/request.csr");
        let cert = format!("{role}/cert.pem");
        let ext = format!("{role}/extensions");
        fs::write(
            self.root.join(&ext),
            if server {
                "basicConstraints=CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:layerx-human-kms,IP:127.0.0.1\n"
            } else {
                "basicConstraints=CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=clientAuth\n"
            },
        )?;
        self.openssl(&[
            "req",
            "-new",
            "-newkey",
            "ec",
            "-pkeyopt",
            "ec_paramgen_curve:P-256",
            "-nodes",
            "-keyout",
            &key,
            "-out",
            &csr,
            "-subj",
            &format!("/CN={cn}"),
        ])?;
        self.openssl(&[
            "x509",
            "-req",
            "-in",
            &csr,
            "-CA",
            "ca.pem",
            "-CAkey",
            "ca.key",
            "-set_serial",
            &(index + 2).to_string(),
            "-days",
            "1",
            "-extfile",
            &ext,
            "-out",
            &cert,
        ])?;
        self.openssl(&[
            "x509",
            "-in",
            &cert,
            "-outform",
            "DER",
            "-out",
            &format!("{role}/cert.der"),
        ])?;
        self.openssl(&[
            "pkcs8",
            "-topk8",
            "-nocrypt",
            "-in",
            &key,
            "-outform",
            "DER",
            "-out",
            &format!("{role}/key.der"),
        ])?;
        fs::copy(self.root.join("ca.der"), dir.join("ca.der"))?;
        for entry in fs::read_dir(dir)? {
            fs::set_permissions(entry?.path(), fs::Permissions::from_mode(0o600))?;
        }
        Ok(())
    }

    fn project_chain(&self, chain: u64) -> Result<std::process::Output> {
        let function = shell_function(KERNEL_INIT, "human_kms_chain_snapshot")?;
        Ok(Command::new("sh")
            .arg("-ec")
            .arg(format!(
                "{function}\nhuman_kms_chain_snapshot \"$1\" \"$2\""
            ))
            .arg("proof")
            .arg(self.root.join("registry.json"))
            .arg(chain.to_string())
            .output()?)
    }

    fn configure(&self, command: &mut Command) {
        command
            .env_clear()
            .env("LAYERX_HUMAN_KMS_LISTEN", self.address.to_string())
            .env("LAYERX_HUMAN_KMS_PROVIDER_REFERENCE", PROVIDER)
            .env("LAYERX_HUMAN_KMS_STATE_DIR", self.root.join("state"))
            .env("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "3")
            .env("LAYERX_RUNTIME_CLOCK_SOCKET", &self.clock_socket)
            .env("LAYERX_RUNTIME_CLOCK_PID", std::process::id().to_string())
            .env(
                "LAYERX_RUNTIME_CLOCK_UID",
                rustix::process::geteuid().as_raw().to_string(),
            );
    }

    fn command(&self) -> Result<Command> {
        let function = shell_function(ENTRYPOINT, "kms_environment")?;
        let mut command = Command::new("sh");
        self.configure(&mut command);
        command
            .args([
                "-ec",
                &format!("{function}\nkms_environment \"$1\"\nshift\nexec \"$@\""),
                "proof",
            ])
            .arg(&self.root)
            .arg(env!("CARGO_BIN_EXE_layerx-human-kms"));
        Ok(command)
    }

    fn start(&mut self) -> Result<()> {
        self.child = Some(
            self.command()?
                .stdout(Stdio::null())
                .stderr(fs::File::create(self.root.join("kms.log"))?)
                .spawn()?,
        );
        let start = self.clock.sample(Duration::from_secs(1))?;
        loop {
            if self.remote("human-kms-client", PROVIDER)?.probe().is_ok() {
                return Ok(());
            }
            if let Some(status) = self
                .child
                .as_mut()
                .ok_or("missing KMS producer")?
                .try_wait()?
            {
                return Err(format!(
                    "services/human/crates/layerx-human-kms/src/main.rs exited {status}: {}",
                    fs::read_to_string(self.root.join("kms.log"))?
                )
                .into());
            }
            assert!(
                self.clock
                    .sample(Duration::from_secs(1))?
                    .elapsed_since(start)?
                    < Duration::from_secs(10),
                "layerx-human-kms did not listen: {}",
                fs::read_to_string(self.root.join("kms.log"))?
            );
            thread::sleep(Duration::from_millis(10));
        }
    }

    fn stop(&mut self) {
        if let Some(mut child) = self.child.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }

    fn remote(&self, identity: &str, provider: &str) -> Result<RemoteKmsProvider> {
        let mut roots = RootCertStore::empty();
        roots.add(CertificateDer::from(fs::read(self.root.join("ca.der"))?))?;
        let tls = MutualTlsConfig::new(
            roots,
            vec![CertificateDer::from(fs::read(
                self.root.join(identity).join("cert.der"),
            )?)],
            PrivateKeyDer::try_from(fs::read(self.root.join(identity).join("key.der"))?)?,
        )
        .map_err(|e| format!("TLS: {e:?}"))?;
        Ok(RemoteKmsProvider::new(
            provider,
            self.address,
            "layerx-human-kms",
            tls,
            Limits {
                maximum_frame_bytes: MAX_FRAME,
                maximum_connections: 4,
                maximum_streams: 1,
                maximum_queued_bytes: MAX_FRAME,
                deadline: Duration::from_secs(3),
            },
            Arc::clone(&self.clock),
        )?)
    }

    fn check_config(&self, expected_error: Option<&str>) -> Result<()> {
        // Consume the entrypoint's actual environment producer, with no ambient configuration.
        let function = shell_function(ENTRYPOINT, "kms_environment")?;
        let mut command = Command::new("sh");
        self.configure(&mut command);
        let output = command
            .args([
                "-ec",
                &format!("{function}\nkms_environment \"$1\"\nenv -0"),
                "proof",
            ])
            .arg(&self.root)
            .output()?;
        assert!(
            output.status.success(),
            "docker/human-service/entrypoint.sh::kms_environment failed"
        );
        let mut consumer = Command::new(std::env::current_exe()?);
        consumer
            .env_clear()
            .args(["--exact", "proof_human_kms", "--nocapture"])
            .env(
                CONFIG_CHECK,
                serde_json::to_string(&(&self.root, self.address, expected_error))?,
            );
        for binding in output
            .stdout
            .split(|byte| *byte == 0)
            .filter(|b| !b.is_empty())
        {
            let (name, value) = std::str::from_utf8(binding)?
                .split_once('=')
                .ok_or("malformed environment binding")?;
            if name.starts_with("LAYERX_HUMAN_KMS_") {
                consumer.env(name, value);
            }
        }
        let output = consumer.output()?;
        assert!(
            output.status.success(),
            "Config::load proof failed: {}\n{}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        );
        Ok(())
    }
}

fn prove_signer_contract(host: &Host) -> Result<()> {
    let listeners = (0..3)
        .map(|_| TcpListener::bind("127.0.0.1:0"))
        .collect::<std::io::Result<Vec<_>>>()?;
    let signers: Vec<String> = (1..=3).map(|n| format!("node-{n}")).collect();
    let config = AttestorSignerConfig {
        nodes: listeners
            .iter()
            .zip(&signers)
            .map(|(listener, id)| {
                Ok(AttestorNodeConfig {
                    id: id.clone(),
                    address: listener.local_addr()?,
                })
            })
            .collect::<std::io::Result<Vec<_>>>()?,
        trust_anchors: vec![host.root.join("ca.der")],
        client_certificate_chain: vec![host.root.join("human-kms-client/cert.der")],
        client_private_key_pkcs8: host.root.join("human-kms-client/key.der"),
        key_id: "native-owner".into(),
        public_key: [7; 32],
        evm_key_id: "evm-owner".into(),
        evm_address: [9; 20],
        signers,
        network_id: 125,
        timeout_seconds: 3,
    };
    let json = serde_json::to_vec(&config)?;
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: [0x44; 20],
        data: vec![],
    };
    for _ in 0..2 {
        // The same strict serde contract used by store.rs::EvmAttestor::load.
        let decoded: AttestorSignerConfig = serde_json::from_slice(&json)?;
        let mut signer = AttestorSigner::from_config(&decoded)?;
        assert_eq!(signer.evm_address(), config.evm_address);
        assert_eq!(signer.client().nodes(), config.signers);
        let unsigned = unsigned_evm_transaction(&call);
        assert_eq!(decode_unsigned_evm_transaction(&unsigned)?, call);
        assert_eq!(
            signer.sign_evm_transaction(&unsigned, 125).err(),
            Some(AttestorError::Configuration("EVM owner assertion"))
        );
        signer.set_evm_assertion("owner-authority")?;
        assert!(signer.sign_evm_transaction(&[2, 0xc0], 125).is_err());
        assert!(signer.sign_evm_transaction(&unsigned, 1).is_err());
        let signed_transaction = call.sign(&[1; 32])?;
        assert!(decode_unsigned_evm_transaction(&signed_transaction).is_err());
        assert!(signer
            .sign_evm_transaction(&signed_transaction, 125)
            .is_err());
        assert!(signer.audit_sequences().is_empty());
    }
    let mut malformed = serde_json::to_value(&config)?;
    malformed["modules"] = json!([]);
    assert!(serde_json::from_value::<AttestorSignerConfig>(malformed).is_err());
    let mut wrong = config.clone();
    wrong.signers[0] = "unknown-node".into();
    assert!(matches!(
        AttestorSigner::from_config(&wrong),
        Err(AttestorError::Configuration(
            "signer outside the attestor nodes"
        ))
    ));
    wrong = config.clone();
    wrong.evm_address = [0; 20];
    assert!(matches!(
        AttestorSigner::from_config(&wrong),
        Err(AttestorError::Configuration("attestor chain or address"))
    ));
    wrong = config;
    wrong.evm_key_id.clear();
    assert!(matches!(
        AttestorSigner::from_config(&wrong),
        Err(AttestorError::Configuration("EVM key id"))
    ));
    Ok(())
}

#[test]
fn proof_human_kms() -> Result<()> {
    let _ = rustls::crypto::ring::default_provider().install_default();
    if let Some(check) = std::env::var_os(CONFIG_CHECK) {
        return verify_config_check(&check);
    }
    let mut host = Host::new()?;
    assert_config_rejections(&host)?;
    prove_signer_contract(&host)?;
    for source in [
        include_str!("../../../../../docker/human-service/Dockerfile"),
        include_str!("../../../../../docker/kernel/Dockerfile"),
    ] {
        assert!(source.contains("-p layerx-human-kms --bin layerx-human-kms"));
        assert!(source.contains("/src/services/human/target/release/layerx-human-kms"));
    }
    assert!(include_str!("../../../../../scripts/wallet/gate-lint.sh")
        .contains("cargo clippy --locked $HUMAN_PKGS -- -D warnings"));
    let invalid_cli = host.command()?.arg("unexpected-argument").output()?;
    assert!(!invalid_cli.status.success());
    assert!(String::from_utf8_lossy(&invalid_cli.stderr).contains("arguments refused"));
    host.start()?;
    let service = host.remote("human-kms-client", PROVIDER)?;
    let executor = host.remote("human-kms-executor", PROVIDER)?;
    let binding = PrincipalKeyBinding::from_digest([5; 32], 125, KeyClass::HumanPrimary)?;
    let created = service.create_key(&binding)?;
    assert_ne!(created.public_key(), [0; 32]);
    for _ in 0..2 {
        executor.probe_executor(125)?;
        assert_eq!(
            service.describe_key(&binding, created.reference())?,
            created
        );
        assert_eq!(service.probe_executor(125).err(), Some(KmsError::Refused));
        assert_eq!(executor.probe_executor(1).err(), Some(KmsError::Refused));
    }
    assert_eq!(
        host.remote("human-kms-client", "unknown-provider")?
            .probe()
            .err(),
        Some(KmsError::Refused)
    );
    assert!(host.remote("foreign", PROVIDER)?.probe().is_err());
    let unknown = ProviderKeyReference::new(vec![0x99; 32])?;
    assert!(service.describe_key(&binding, &unknown).is_err());
    host.stop();
    assert!(
        service.describe_key(&binding, created.reference()).is_err(),
        "consumer reused a cached description after producer stopped"
    );
    assert!(
        executor.probe_executor(125).is_err(),
        "consumer reused a cached executor probe"
    );
    host.start()?;
    for _ in 0..2 {
        assert_eq!(
            service.describe_key(&binding, created.reference())?,
            created
        );
        executor.probe_executor(125)?;
    }
    Ok(())
}

fn verify_config_check(check: &std::ffi::OsStr) -> Result<()> {
    let (root, address, expected_error): (PathBuf, SocketAddr, Option<String>) =
        serde_json::from_str(check.to_str().ok_or("invalid configuration check")?)?;
    if let Some(expected) = expected_error {
        let error = config::Config::load()
            .err()
            .ok_or("configuration was accepted")?;
        if expected == "chain snapshot invalid" {
            assert!(error.contains(&expected));
        } else {
            assert_eq!(error, expected);
        }
        return Ok(());
    }
    let loaded = config::Config::load()?;
    assert_eq!(loaded.chain.chain_id, 125);
    assert_eq!(loaded.provider, PROVIDER);
    assert_eq!(loaded.state, root.join("state"));
    assert_eq!(loaded.listen, address);
    assert_eq!(loaded.seal.len(), 32);
    assert_eq!(loaded.deadline, Duration::from_secs(3));
    assert_eq!(loaded.tls.max_early_data_size, 0);
    assert_eq!(
        loaded.client_pin,
        <[u8; 32]>::from(Sha256::digest(fs::read(root.join("kms-client.der"))?))
    );
    assert_eq!(
        loaded
            .attestor_evm_client_certificate
            .ok_or("missing executor pin")?
            .as_ref(),
        fs::read(root.join("kms-executor.der"))?
    );
    Ok(())
}

fn assert_config_rejections(host: &Host) -> Result<()> {
    let legacy = fs::read(host.root.join("registry.json"))?;
    let registry: serde_json::Value = serde_json::from_slice(&legacy)?;
    assert_eq!(registry["modules"][0]["module_id"], 1);
    assert_eq!(
        registry["modules"][0]["activity_types"],
        json!([65537, 65540])
    );
    host.check_config(Some("chain snapshot invalid"))?;
    assert!(host.project_chain(125)?.status.success());
    assert!(!host.project_chain(1)?.status.success());
    for _ in 0..2 {
        host.check_config(None)?;
    }
    for malformed in [
        br#"{"chain_id":125,"modules":[]}"#.as_slice(),
        br#"{"chain_id":125,"module_id":1}"#,
        br#"{"chain_id":125,"activity_types":[65537]}"#,
        b"{",
        legacy.as_slice(),
    ] {
        fs::write(host.root.join("registry.json"), malformed)?;
        host.check_config(Some("chain snapshot invalid"))?;
        let refused = host.command()?.output()?;
        assert!(
            !refused.status.success(),
            "layerx-human-kms accepted a malformed producer snapshot"
        );
        assert!(String::from_utf8_lossy(&refused.stderr).contains("chain snapshot invalid"));
    }
    assert!(host.project_chain(125)?.status.success());
    let executor = fs::read(host.root.join("kms-executor.der"))?;
    fs::write(
        host.root.join("kms-executor.der"),
        fs::read(host.root.join("kms-client.der"))?,
    )?;
    host.check_config(Some("attestor signer and service identities must differ"))?;
    fs::write(
        host.root.join("kms-executor.der"),
        b"malformed producer certificate",
    )?;
    host.check_config(Some("attestor signer certificate invalid"))?;
    let refused = host.command()?.output()?;
    assert!(
        !refused.status.success(),
        "layerx-human-kms accepted malformed executor DER"
    );
    assert!(
        String::from_utf8_lossy(&refused.stderr).contains("attestor signer certificate invalid")
    );
    fs::write(host.root.join("kms-executor.der"), executor)?;
    Ok(())
}
