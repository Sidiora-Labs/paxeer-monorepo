use rustix::fs::{open, Mode, OFlags};
use rustls::pki_types::{CertificateDer, PrivateKeyDer};
use rustls::{RootCertStore, ServerConfig};
use serde::Deserialize;
use sha2::{Digest, Sha256};
use std::fs::File;
use std::io::Read;
use std::net::SocketAddr;
use std::os::unix::fs::MetadataExt;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::Duration;
use zeroize::Zeroizing;

const LAYERX_HUMAN_KMS_REGISTRY_FILE: &str = "LAYERX_HUMAN_KMS_REGISTRY_FILE";

pub(crate) struct Config {
    pub listen: SocketAddr,
    pub state: PathBuf,
    pub provider: String,
    pub chain: ChainSnapshot,
    pub seal: Zeroizing<Vec<u8>>,
    pub tls: Arc<ServerConfig>,
    pub client_pin: [u8; 32],
    pub attestor_evm_client_certificate: Option<CertificateDer<'static>>,
    pub deadline: Duration,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ChainSnapshot {
    pub chain_id: u64,
}
pub(crate) fn protected(
    path: &Path,
    maximum: usize,
    secret: bool,
) -> Result<Zeroizing<Vec<u8>>, String> {
    if !path.is_absolute() {
        return Err("provider paths must be absolute".into());
    }
    let descriptor = open(
        path,
        OFlags::RDONLY | OFlags::NOFOLLOW | OFlags::CLOEXEC,
        Mode::empty(),
    )
    .map_err(|_| "provider file cannot be opened")?;
    let mut file = File::from(descriptor);
    let metadata = file
        .metadata()
        .map_err(|_| "provider file metadata unavailable")?;
    if !metadata.is_file()
        || metadata.uid() != rustix::process::geteuid().as_raw()
        || metadata.mode() & (if secret { 0o077 } else { 0o022 }) != 0
    {
        return Err("provider file ownership or permissions refused".into());
    }
    let mut bytes = Zeroizing::new(Vec::new());
    file.by_ref()
        .take(u64::try_from(maximum).map_err(|_| "file bound invalid")? + 1)
        .read_to_end(&mut bytes)
        .map_err(|_| "provider file unreadable")?;
    if bytes.is_empty() || bytes.len() > maximum {
        return Err("provider file size refused".into());
    }
    Ok(bytes)
}
fn required(suffix: &str) -> Result<String, String> {
    required_variable(&format!("LAYERX_HUMAN_KMS_{suffix}"))
}
fn required_variable(name: &str) -> Result<String, String> {
    std::env::var(name)
        .ok()
        .filter(|v| !v.is_empty())
        .ok_or_else(|| format!("{name} is required"))
}
fn read(suffix: &str, max: usize, secret: bool) -> Result<Zeroizing<Vec<u8>>, String> {
    protected(Path::new(&required(suffix)?), max, secret)
}
impl Config {
    pub fn load() -> Result<Self, String> {
        let provider = required("PROVIDER_REFERENCE")?;
        if provider.len() > 256 || provider.contains('\0') {
            return Err("provider reference refused".into());
        }
        let snapshot_path = required_variable(LAYERX_HUMAN_KMS_REGISTRY_FILE)?;
        let snapshot = protected(Path::new(&snapshot_path), 65536, false).map_err(|error| {
            format!("{LAYERX_HUMAN_KMS_REGISTRY_FILE} ({snapshot_path}): {error}")
        })?;
        let chain: ChainSnapshot = serde_json::from_slice(&snapshot).map_err(|_| {
            format!("{LAYERX_HUMAN_KMS_REGISTRY_FILE} ({snapshot_path}): chain snapshot invalid")
        })?;
        if chain.chain_id != 125 {
            return Err("chain id refused".into());
        }
        let mut roots = RootCertStore::empty();
        roots
            .add(CertificateDer::from(
                read("CLIENT_CA_DER", 65536, false)?.to_vec(),
            ))
            .map_err(|_| "client CA invalid")?;
        let verifier = rustls::server::WebPkiClientVerifier::builder(Arc::new(roots))
            .build()
            .map_err(|_| "client verifier invalid")?;
        let tls = ServerConfig::builder()
            .with_client_cert_verifier(verifier)
            .with_single_cert(
                vec![CertificateDer::from(
                    read("TLS_CERT_DER", 65536, false)?.to_vec(),
                )],
                PrivateKeyDer::try_from(read("TLS_KEY_DER", 65536, true)?.to_vec())
                    .map_err(|_| "TLS key invalid")?,
            )
            .map_err(|_| "TLS identity invalid")?;
        let seal = read("SEAL_SECRET_FILE", 32, true)?;
        if seal.len() != 32 {
            return Err("seal secret must contain 32 random bytes".into());
        }
        let client_pin: [u8; 32] = Sha256::digest(&*read("CLIENT_CERT_DER", 65536, false)?).into();
        let attestor_evm_client_certificate =
            std::env::var_os("LAYERX_HUMAN_KMS_EVM_CLIENT_CERT_DER")
                .map(|path| {
                    protected(Path::new(&path), 65536, false)
                        .map(|bytes| CertificateDer::from(bytes.to_vec()))
                })
                .transpose()?;
        if attestor_evm_client_certificate
            .as_ref()
            .is_some_and(|certificate| {
                let pin: [u8; 32] = Sha256::digest(certificate.as_ref()).into();
                pin == client_pin
            })
        {
            return Err("attestor signer and service identities must differ".into());
        }
        let seconds: u64 = required("DEADLINE_SECONDS")?
            .parse()
            .map_err(|_| "deadline invalid")?;
        if !(1..=60).contains(&seconds) {
            return Err("deadline refused".into());
        }
        Ok(Self {
            listen: required("LISTEN")?
                .parse()
                .map_err(|_| "listen address invalid")?,
            state: PathBuf::from(required("STATE_DIR")?),
            provider,
            chain,
            seal,
            tls: Arc::new(tls),
            client_pin,
            attestor_evm_client_certificate,
            deadline: Duration::from_secs(seconds),
        })
    }
}
