#[path = "../src/config.rs"]
mod config;

use config::Config;
use std::error::Error;
use std::fs;
use std::os::unix::fs::{DirBuilderExt, PermissionsExt};
use std::path::PathBuf;
use std::process::Command;

type Result<T> = std::result::Result<T, Box<dyn Error>>;
const CHILD_ROOT: &str = "LAYERX_KMS_CHAIN_CONFIG_CHILD";

struct Fixture {
    root: PathBuf,
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

impl Fixture {
    fn new() -> Result<Self> {
        let mut nonce = [0_u8; 16];
        getrandom::fill(&mut nonce).map_err(|error| std::io::Error::other(error.to_string()))?;
        let root = std::env::temp_dir().join(format!(
            "kms-chain-config-{}-{:x}",
            std::process::id(),
            u128::from_be_bytes(nonce)
        ));
        fs::DirBuilder::new().mode(0o700).create(&root)?;
        let fixture = Self { root };
        for name in ["server", "service", "attestor"] {
            fixture.openssl(&[
                "req",
                "-x509",
                "-newkey",
                "ec",
                "-pkeyopt",
                "ec_paramgen_curve:P-256",
                "-nodes",
                "-keyout",
                &format!("{name}.key"),
                "-out",
                &format!("{name}.pem"),
                "-days",
                "1",
                "-subj",
                &format!("/CN={name}"),
                "-addext",
                "basicConstraints=critical,CA:TRUE",
            ])?;
            fixture.openssl(&[
                "x509",
                "-in",
                &format!("{name}.pem"),
                "-outform",
                "DER",
                "-out",
                &format!("{name}.der"),
            ])?;
        }
        fixture.openssl(&[
            "pkcs8",
            "-topk8",
            "-nocrypt",
            "-in",
            "server.key",
            "-outform",
            "DER",
            "-out",
            "server-key.der",
        ])?;
        let mut seal = [0_u8; 32];
        getrandom::fill(&mut seal).map_err(|error| std::io::Error::other(error.to_string()))?;
        fs::write(fixture.root.join("seal"), seal)?;
        fs::write(fixture.root.join("snapshot.json"), br#"{"chain_id":125}"#)?;
        for entry in fs::read_dir(&fixture.root)? {
            fs::set_permissions(entry?.path(), fs::Permissions::from_mode(0o600))?;
        }
        Ok(fixture)
    }

    fn openssl(&self, arguments: &[&str]) -> Result<()> {
        let output = Command::new("openssl")
            .args(arguments)
            .current_dir(&self.root)
            .output()?;
        if !output.status.success() {
            return Err(format!(
                "openssl failed: {}",
                String::from_utf8_lossy(&output.stderr)
            )
            .into());
        }
        Ok(())
    }

    fn run(&self, name: &str) -> Result<()> {
        let mut command = Command::new(std::env::current_exe()?);
        command
            .args(["--exact", name, "--nocapture"])
            .env_clear()
            .env(CHILD_ROOT, &self.root)
            .env("LAYERX_HUMAN_KMS_PROVIDER_REFERENCE", "chain-config-test")
            .env("LAYERX_HUMAN_KMS_LISTEN", "127.0.0.1:0")
            .env("LAYERX_HUMAN_KMS_STATE_DIR", self.root.join("state"))
            .env("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "5");
        for (suffix, file) in [
            ("REGISTRY_FILE", "snapshot.json"),
            ("CLIENT_CA_DER", "service.der"),
            ("TLS_CERT_DER", "server.der"),
            ("TLS_KEY_DER", "server-key.der"),
            ("SEAL_SECRET_FILE", "seal"),
            ("CLIENT_CERT_DER", "service.der"),
            ("EVM_CLIENT_CERT_DER", "attestor.der"),
        ] {
            command.env(format!("LAYERX_HUMAN_KMS_{suffix}"), self.root.join(file));
        }
        let output = command.output()?;
        assert!(
            output.status.success(),
            "config test failed: {}\n{}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        );
        Ok(())
    }
}

#[test]
fn kms_config_loads_a_chain_snapshot_for_chain_125() -> Result<()> {
    let Some(root) = std::env::var_os(CHILD_ROOT) else {
        return Fixture::new()?.run("kms_config_loads_a_chain_snapshot_for_chain_125");
    };
    let root = PathBuf::from(root);
    let loaded = Config::load()?;
    assert_eq!(loaded.chain.chain_id, 125);
    let certificate = loaded
        .attestor_evm_client_certificate
        .ok_or("attestor certificate missing")?;
    assert_eq!(certificate.as_ref(), fs::read(root.join("attestor.der"))?);
    for chain_id in [0_u64, 1, 124, 126, u64::MAX] {
        fs::write(
            root.join("snapshot.json"),
            serde_json::to_vec(&serde_json::json!({"chain_id": chain_id}))?,
        )?;
        assert!(matches!(Config::load(), Err(error) if error == "chain id refused"));
    }
    Ok(())
}

#[test]
fn kms_config_refuses_a_module_registry_document() -> Result<()> {
    let Some(root) = std::env::var_os(CHILD_ROOT) else {
        return Fixture::new()?.run("kms_config_refuses_a_module_registry_document");
    };
    let snapshot = PathBuf::from(root).join("snapshot.json");
    assert_eq!(Config::load()?.chain.chain_id, 125);
    for document in [
        serde_json::json!({"network_id":1,"protocol_version":3,"modules":[]}),
        serde_json::json!({"network_id":77,"protocol_version":3,"modules":[{"module_id":1,"activity_types":[1]}]}),
        serde_json::json!({"chain_id":125,"modules":[{"module_id":1,"activity_types":[1]}]}),
        serde_json::json!({"chain_id":125,"network_id":77,"protocol_version":3}),
        serde_json::json!({"chain_id":125,"activity_types":[1]}),
        serde_json::json!({}),
    ] {
        fs::write(&snapshot, serde_json::to_vec(&document)?)?;
        let expected = format!(
            "LAYERX_HUMAN_KMS_REGISTRY_FILE ({}): chain snapshot invalid",
            snapshot.display()
        );
        assert!(matches!(Config::load(), Err(error) if error == expected));
    }
    Ok(())
}
