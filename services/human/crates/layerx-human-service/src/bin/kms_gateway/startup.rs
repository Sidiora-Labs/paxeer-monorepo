use std::net::SocketAddr;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::Duration;

use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_human_service::custody::{
    GatewayChainSnapshot, GatewayLimits, GatewayPolicy, GatewayTls, LxkpGateway, RemoteKmsProvider,
};
use layerx_human_service::frame::{Limits, MutualTlsConfig};
use layerx_types::clock::Clock;
use layerx_types::payload::ModuleRegistry;
use rustls::pki_types::{CertificateDer, PrivateKeyDer};
use rustls::RootCertStore;
use zeroize::Zeroizing;

const PREFIX: &str = "LAYERX_HUMAN_KMS_GATEWAY";
const LAYERX_HUMAN_KMS_GATEWAY_REGISTRY_FILE: &str = "LAYERX_HUMAN_KMS_GATEWAY_REGISTRY_FILE";
const REMOTE_PROVIDER: &str = "remote-lxkp";

pub(super) type ChainSnapshot = GatewayChainSnapshot;

pub(super) fn runtime_clock(binding: ClockBinding) -> Result<Arc<dyn Clock>, String> {
    RuntimeClock::connect(binding)
        .map(|clock| clock as Arc<dyn Clock>)
        .map_err(|error| format!("runtime clock unavailable: {error}"))
}

pub(super) fn prepare(
    clock: Arc<dyn Clock>,
    environment: &dyn Fn(&str) -> Option<String>,
) -> Result<(SocketAddr, LxkpGateway), String> {
    let settings = Settings { environment };
    if settings.required("PROVIDER_KIND")? != REMOTE_PROVIDER {
        return Err(format!(
            "{PREFIX}_PROVIDER_KIND must name the non-exportable remote KMS provider \
             {REMOTE_PROVIDER}; development envelope providers never back the gateway"
        ));
    }
    let listen: SocketAddr = settings
        .required("LISTEN")?
        .parse()
        .map_err(|_| format!("{PREFIX}_LISTEN is invalid"))?;
    let endpoint: SocketAddr = settings
        .required("PROVIDER_ENDPOINT")?
        .parse()
        .map_err(|_| format!("{PREFIX}_PROVIDER_ENDPOINT is invalid"))?;
    if endpoint == listen {
        return Err("the gateway cannot be its own KMS provider".to_owned());
    }
    let snapshot = snapshot(&PathBuf::from(settings.required("REGISTRY_FILE")?))?;
    let mut roots = RootCertStore::empty();
    roots
        .add(CertificateDer::from(
            settings
                .material("PROVIDER_ROOT_CERTIFICATE_DER", false)?
                .to_vec(),
        ))
        .map_err(|_| "the KMS provider trust root is invalid".to_owned())?;
    let relay_certificate = CertificateDer::from(
        settings
            .material("PROVIDER_CLIENT_CERTIFICATE_DER", false)?
            .to_vec(),
    );
    let relay_key = PrivateKeyDer::try_from(
        settings
            .material("PROVIDER_CLIENT_PRIVATE_KEY_DER", true)?
            .to_vec(),
    )
    .map_err(|_| "the KMS provider client private key is invalid".to_owned())?;
    let deadline_tls = rustls::ClientConfig::builder()
        .with_root_certificates(roots.clone())
        .with_client_auth_cert(vec![relay_certificate.clone()], relay_key.clone_key())
        .map_err(|_| "the KMS provider mutual TLS identity is invalid".to_owned())?;
    let relay = MutualTlsConfig::new(roots, vec![relay_certificate], relay_key)
        .map_err(|_| "the KMS provider mutual TLS identity is invalid".to_owned())?;
    let provider = RemoteKmsProvider::new(
        settings.required("PROVIDER_REFERENCE")?,
        endpoint,
        settings.required("PROVIDER_SERVER_NAME")?,
        relay,
        Limits {
            maximum_frame_bytes: settings.number("PROVIDER_MAX_FRAME_BYTES")?,
            maximum_connections: settings.number("PROVIDER_MAX_CONNECTIONS")?,
            maximum_streams: settings.number("PROVIDER_MAX_STREAMS")?,
            maximum_queued_bytes: settings.number("PROVIDER_MAX_QUEUED_BYTES")?,
            deadline: Duration::from_secs(settings.number("PROVIDER_DEADLINE_SECONDS")?),
        },
        Arc::clone(&clock),
    )
    .map_err(|error| format!("the KMS provider was refused: {}", error.refusal_code()))?
    .with_deadline_transport(Arc::new(deadline_tls));
    let tls = GatewayTls::new(
        &settings.material("CLIENT_CA_DER", false)?,
        &settings.material("TLS_CERT_DER", false)?,
        &settings.material("TLS_KEY_DER", true)?,
        &settings.material("CLIENT_CERT_DER", false)?,
    )
    .map_err(|error| {
        format!(
            "the gateway TLS identity was refused: {}",
            error.refusal_code()
        )
    })?;
    let gateway = LxkpGateway::new(
        Arc::new(provider),
        tls,
        GatewayPolicy {
            network_id: snapshot.chain_id,
            protocol_version: 3,
            registry: ModuleRegistry::new(&[])
                .map_err(|_| "the chain snapshot scope was refused".to_owned())?,
            limits: GatewayLimits {
                maximum_connections: settings.number("MAX_CONNECTIONS")?,
                maximum_frame_bytes: settings.number("MAX_FRAME_BYTES")?,
                deadline: Duration::from_secs(settings.number("DEADLINE_SECONDS")?),
                maximum_operations: settings.number("RATE_MAXIMUM")?,
                operation_window: Duration::from_secs(settings.number("RATE_WINDOW_SECONDS")?),
            },
        },
        Arc::clone(&clock),
    )
    .map_err(|error| format!("the gateway was refused: {}", error.refusal_code()))?;
    Ok((listen, gateway))
}

pub(super) fn snapshot(path: &Path) -> Result<ChainSnapshot, String> {
    let bytes = protected(path, false)
        .map_err(|reason| format!("{LAYERX_HUMAN_KMS_GATEWAY_REGISTRY_FILE} {reason}"))?;
    GatewayChainSnapshot::decode(&bytes)
}

struct Settings<'a> {
    environment: &'a dyn Fn(&str) -> Option<String>,
}
impl Settings<'_> {
    fn required(&self, suffix: &str) -> Result<String, String> {
        let name = format!("{PREFIX}_{suffix}");
        (self.environment)(&name)
            .filter(|value| !value.is_empty())
            .ok_or_else(|| format!("{name} is required"))
    }

    fn material(&self, suffix: &str, secret: bool) -> Result<Zeroizing<Vec<u8>>, String> {
        let name = format!("{PREFIX}_{suffix}");
        let path = PathBuf::from(self.required(suffix)?);
        if !path.is_absolute() {
            return Err(format!("{name} must be absolute"));
        }
        protected(&path, secret).map_err(|reason| format!("{name} {reason}"))
    }

    fn number<T: std::str::FromStr>(&self, suffix: &str) -> Result<T, String> {
        self.required(suffix)?
            .parse()
            .map_err(|_| format!("{PREFIX}_{suffix} is invalid"))
    }
}

#[cfg(test)]
pub(super) fn forward_clock_binding(command: &mut std::process::Command, binding: &ClockBinding) {
    command
        .env("LAYERX_RUNTIME_CLOCK_SOCKET", &binding.socket)
        .env("LAYERX_RUNTIME_CLOCK_PID", binding.process.to_string())
        .env("LAYERX_RUNTIME_CLOCK_UID", binding.user.to_string());
}

#[cfg(test)]
pub(super) fn probe_binding(
    snapshot: &ChainSnapshot,
    provider: &str,
) -> Result<
    layerx_human_service::custody::PrincipalKeyBinding,
    layerx_human_service::custody::CustodyError,
> {
    use layerx_human_service::custody::{KeyClass, PrincipalKeyBinding};
    PrincipalKeyBinding::new(
        b"carol".to_vec(),
        snapshot.chain_id,
        KeyClass::HumanPrimary,
        provider,
    )
}

pub(super) fn protected(path: &Path, secret: bool) -> Result<Zeroizing<Vec<u8>>, &'static str> {
    GatewayTls::read_material(path, secret)
}
