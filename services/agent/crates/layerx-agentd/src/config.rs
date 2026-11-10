//! Fail-closed startup configuration.
//!
//! Configuration is loaded from one explicitly named UTF-8 file containing `key=value`
//! declarations. Exact `LAYERX_*` environment keys override the corresponding file values.
//! Duplicate file declarations, unknown `LayerX` settings, blank overrides, and incomplete
//! per-tenant maps are errors; no security-relevant value has a default.

use std::collections::{BTreeMap, BTreeSet};
use std::fmt::{Display, Formatter};
use std::fs::{self, File, Metadata};
use std::io::Read;
use std::os::unix::fs::MetadataExt;
use std::path::{Component, Path, PathBuf};

use layerx_types::verify::VerificationLevel;

use crate::store::TenantId;

const MAX_CONFIG_BYTES: usize = 65_536;
const MAX_LINE_BYTES: usize = 4_096;

#[path = "human_owner_mode.rs"]
pub mod agent_mode;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct InferenceConfig {
    pub signal_listen: std::net::SocketAddr,
    pub serve_listen: std::net::SocketAddr,
    pub model_digest: [u8; 32],
    pub runtime_digest: [u8; 32],
    pub room_profile: PathBuf,
}

impl InferenceConfig {
    pub fn from_environment() -> Result<Self, String> {
        let mut values = BTreeMap::new();
        for name in [
            "LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN",
            "LAYERX_AGENT_INFERENCE_SERVE_LISTEN",
            "LAYERX_AGENT_INFERENCE_MODEL_DIGEST",
            "LAYERX_AGENT_INFERENCE_RUNTIME_DIGEST",
            "LAYERX_AGENT_INFERENCE_ROOM_PROFILE",
        ] {
            match std::env::var(name) {
                Ok(value) => {
                    values.insert(name.to_owned(), value);
                }
                Err(std::env::VarError::NotPresent) => {}
                Err(std::env::VarError::NotUnicode(_)) => {
                    return Err(format!("{name} is not UTF-8"))
                }
            }
        }
        Self::from_values(&values)
    }

    pub fn from_values(values: &BTreeMap<String, String>) -> Result<Self, String> {
        let required = |name: &str| {
            values
                .get(name)
                .filter(|value| !value.is_empty() && value.trim() == value.as_str())
                .ok_or_else(|| format!("{name} is required and must be nonblank"))
        };
        let digest = |name: &str| {
            let bytes = layerx_programs::hex::decode_digest(required(name)?)
                .map_err(|_| format!("{name} must be a 32-byte hexadecimal digest"))?;
            if bytes == [0; 32] {
                return Err(format!("{name} must be nonzero"));
            }
            Ok(bytes)
        };
        let listen = |name: &str, default: &str| {
            values
                .get(name)
                .map_or(default, String::as_str)
                .parse::<std::net::SocketAddr>()
                .map_err(|_| format!("{name} must be a socket address"))
        };
        let model_digest = digest("LAYERX_AGENT_INFERENCE_MODEL_DIGEST")?;
        let runtime_digest = digest("LAYERX_AGENT_INFERENCE_RUNTIME_DIGEST")?;
        let room_profile = PathBuf::from(required("LAYERX_AGENT_INFERENCE_ROOM_PROFILE")?);
        let signal_listen = listen("LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN", "127.0.0.1:8790")?;
        let serve_listen = listen("LAYERX_AGENT_INFERENCE_SERVE_LISTEN", "127.0.0.1:8791")?;
        if signal_listen == serve_listen && signal_listen.port() != 0 {
            return Err("inference listeners must use different addresses".to_owned());
        }
        Ok(Self {
            signal_listen,
            serve_listen,
            model_digest,
            runtime_digest,
            room_profile,
        })
    }

    pub fn load_room_profile(&self) -> Result<Vec<u8>, String> {
        let file = File::open(&self.room_profile)
            .map_err(|_| "inference room profile is unreadable".to_owned())?;
        if !file
            .metadata()
            .map_err(|error| error.to_string())?
            .is_file()
        {
            return Err("inference room profile must be a regular file".to_owned());
        }
        let mut bytes = Vec::new();
        file.take(MAX_CONFIG_BYTES as u64 + 1)
            .read_to_end(&mut bytes)
            .map_err(|_| "inference room profile is unreadable".to_owned())?;
        if bytes.is_empty() || bytes.len() > MAX_CONFIG_BYTES {
            return Err("inference room profile is empty or too large".to_owned());
        }
        Ok(bytes)
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct RequiredSetting {
    pub file_key: &'static str,
    pub environment_key: &'static str,
}

pub const SECURITY_RELEVANT_SETTINGS: [RequiredSetting; 8] = [
    RequiredSetting {
        file_key: "network_id",
        environment_key: "LAYERX_NETWORK_ID",
    },
    RequiredSetting {
        file_key: "node_endpoint",
        environment_key: "LAYERX_NODE_ENDPOINT",
    },
    RequiredSetting {
        file_key: "expected_chain_id",
        environment_key: "LAYERX_EXPECTED_CHAIN_ID",
    },
    RequiredSetting {
        file_key: "tenants",
        environment_key: "LAYERX_TENANTS",
    },
    RequiredSetting {
        file_key: "policy_sources",
        environment_key: "LAYERX_POLICY_SOURCES",
    },
    RequiredSetting {
        file_key: "signer_configurations",
        environment_key: "LAYERX_SIGNER_CONFIGURATIONS",
    },
    RequiredSetting {
        file_key: "verification_defaults",
        environment_key: "LAYERX_VERIFICATION_DEFAULTS",
    },
    RequiredSetting {
        file_key: "receipt_key_source",
        environment_key: "LAYERX_RECEIPT_KEY_SOURCE",
    },
];

pub const PRECEDENCE: &str =
    "exact LAYERX_* environment values override the explicitly named configuration file";

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ReceiptKeySource {
    NodeRpc,
    PinnedFile(PathBuf),
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StartupConfig {
    pub network_id: u32,
    /// Set `node_endpoint` / `LAYERX_NODE_ENDPOINT` to the node root. The documented
    /// mainnet default is https://api1.mainnet-beta.paxeer.network/ (api2 through
    /// api14 are alternatives). Explicit configuration remains required.
    pub node_endpoint: String,
    pub expected_chain_id: u64,
    pub tenants: BTreeSet<TenantId>,
    pub policy_sources: BTreeMap<TenantId, PathBuf>,
    pub signer_configurations: BTreeMap<TenantId, PathBuf>,
    pub verification_defaults: BTreeMap<TenantId, VerificationLevel>,
    pub receipt_key_source: ReceiptKeySource,
}

impl StartupConfig {
    /// Connects the existing node setting and checks the configured EVM identity.
    pub fn connect_node(
        &self,
        trust_anchors: &[&[u8]],
    ) -> Result<layerx_client::sequencer::SequencerEndpoint, layerx_client::sequencer::SequencerError>
    {
        use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
        let node = SequencerEndpoint::new_node(&self.node_endpoint, trust_anchors)?;
        if node.chain_id()? != self.expected_chain_id {
            return Err(SequencerError::Endpoint);
        }
        Ok(node)
    }

    pub fn connect_px(
        &self,
        trust_anchors: &[&[u8]],
    ) -> Result<layerx_client::px::PxEndpoint, layerx_client::sequencer::SequencerError> {
        self.connect_node(trust_anchors).map(Into::into)
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum RejectionReason {
    Missing,
    Empty,
    Duplicate,
    Unknown,
    Invalid,
    Retired,
    InvalidInteger,
    UnsupportedProtocol,
    InvalidTenant,
    InvalidPath,
    IncompleteTenantMap,
    InvalidVerificationLevel,
    TooLarge,
    InvalidEncoding,
    Unavailable,
    Unprotected,
}

impl Display for RejectionReason {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        let reason = match self {
            Self::Missing => "is required and was not set",
            Self::Empty => "must not be empty",
            Self::Duplicate => "was declared more than once",
            Self::Unknown => "is not a recognised startup setting",
            Self::Invalid => "is not a valid HTTP or HTTPS JSON-RPC URL",
            Self::Retired => "is retired and must be removed",
            Self::InvalidInteger => "is not a valid non-zero integer",
            Self::UnsupportedProtocol => "is not supported by the compiled wire codec",
            Self::InvalidTenant => "contains an invalid or duplicate tenant",
            Self::InvalidPath => "must be an absolute normalised path",
            Self::IncompleteTenantMap => "must contain exactly one value for every tenant",
            Self::InvalidVerificationLevel => "contains an unsafe verification default",
            Self::TooLarge => "exceeds the configured input bound",
            Self::InvalidEncoding => "is not valid UTF-8",
            Self::Unavailable => "could not be read",
            Self::Unprotected => "is not a protected regular file owned by this process user",
        };
        formatter.write_str(reason)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ConfigError {
    pub setting: String,
    pub reason: RejectionReason,
}

impl Display for ConfigError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        write!(
            formatter,
            "configuration setting {} {}",
            self.setting, self.reason
        )
    }
}

impl std::error::Error for ConfigError {}

/// Loads, resolves, and validates startup configuration without consulting ambient defaults.
///
/// `environment` is an explicit snapshot supplied by the process entry point. Non-LayerX
/// variables are ignored, while an unknown `LAYERX_*` variable is rejected as a likely typo.
///
/// # Errors
///
/// Returns the exact setting and rejection reason for file I/O, syntax, precedence, or
/// semantic validation failures.
pub fn load(
    file: impl AsRef<Path>,
    environment: &BTreeMap<String, String>,
) -> Result<StartupConfig, ConfigError> {
    let bytes = read_protected_source(file.as_ref(), MAX_CONFIG_BYTES).map_err(|failure| {
        error(
            "configuration_file",
            match failure {
                ProtectedSourceError::Unavailable => RejectionReason::Unavailable,
                ProtectedSourceError::TooLarge => RejectionReason::TooLarge,
                ProtectedSourceError::Unprotected | ProtectedSourceError::Changed => {
                    RejectionReason::Unprotected
                }
            },
        )
    })?;
    let source = std::str::from_utf8(&bytes)
        .map_err(|_| error("configuration_file", RejectionReason::InvalidEncoding))?;
    let mut values = parse_file(source)?;
    for (name, value) in environment {
        reject_retired_setting(name)?;
        if let Some(setting) = SECURITY_RELEVANT_SETTINGS
            .iter()
            .find(|setting| setting.environment_key == name)
        {
            values.insert(setting.file_key.to_owned(), value.clone());
        } else if name.starts_with("LAYERX_") {
            return Err(error(name, RejectionReason::Unknown));
        }
    }
    validate(&values)
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProtectedSourceError {
    Unavailable,
    Unprotected,
    Changed,
    TooLarge,
}

/// Reads one operator-trusted source without accepting indirection or mutable
/// file metadata at the read boundary.
///
/// # Errors
///
/// Returns an error if the source is not protected, exceeds the size limit, or cannot be read.
pub fn read_protected_source(
    path: &Path,
    maximum_bytes: usize,
) -> Result<Vec<u8>, ProtectedSourceError> {
    if !path.is_absolute() {
        return Err(ProtectedSourceError::Unprotected);
    }
    let canonical = fs::canonicalize(path).map_err(|_| ProtectedSourceError::Unavailable)?;
    if canonical != path {
        return Err(ProtectedSourceError::Unprotected);
    }
    let process_uid = fs::metadata("/proc/self")
        .map_err(|_| ProtectedSourceError::Unavailable)?
        .uid();
    let path_before = fs::symlink_metadata(path).map_err(|_| ProtectedSourceError::Unavailable)?;
    validate_protected_metadata(&path_before, process_uid)?;

    let mut file = File::open(path).map_err(|_| ProtectedSourceError::Unavailable)?;
    let file_before = file
        .metadata()
        .map_err(|_| ProtectedSourceError::Unavailable)?;
    validate_protected_metadata(&file_before, process_uid)?;
    if !same_file_snapshot(&path_before, &file_before) {
        return Err(ProtectedSourceError::Changed);
    }

    let limit = u64::try_from(maximum_bytes)
        .ok()
        .and_then(|value| value.checked_add(1))
        .ok_or(ProtectedSourceError::TooLarge)?;
    let mut bytes = Vec::new();
    file.by_ref()
        .take(limit)
        .read_to_end(&mut bytes)
        .map_err(|_| ProtectedSourceError::Unavailable)?;

    let file_after = file
        .metadata()
        .map_err(|_| ProtectedSourceError::Unavailable)?;
    let path_after = fs::symlink_metadata(path).map_err(|_| ProtectedSourceError::Unavailable)?;
    if fs::canonicalize(path).map_err(|_| ProtectedSourceError::Changed)? != path
        || !same_file_snapshot(&file_before, &file_after)
        || !same_file_snapshot(&file_after, &path_after)
    {
        return Err(ProtectedSourceError::Changed);
    }
    if bytes.len() > maximum_bytes {
        return Err(ProtectedSourceError::TooLarge);
    }
    Ok(bytes)
}

pub const MAX_STATE_EXPORT_SLOTS: usize = 1024;
const MAX_EXPORT_SOURCE_BYTES: usize = 1_048_576;

#[derive(Clone, Debug)]
pub struct StateExportSource {
    selections: BTreeMap<(TenantId, [u8; 32]), Vec<[u8; 32]>>,
}

impl StateExportSource {
    pub fn load(path: &Path) -> Result<Self, String> {
        #[derive(serde::Deserialize)]
        #[serde(deny_unknown_fields)]
        struct Source {
            version: u32,
            selections: Vec<Selection>,
        }
        #[derive(serde::Deserialize)]
        #[serde(deny_unknown_fields)]
        struct Selection {
            tenant: String,
            account: String,
            slots: Vec<String>,
        }
        let bytes = read_protected_source(path, MAX_EXPORT_SOURCE_BYTES)
            .map_err(|error| format!("state export source unavailable: {error:?}"))?;
        let source: Source =
            serde_json::from_slice(&bytes).map_err(|_| "invalid state export source".to_owned())?;
        if source.version != 1 || source.selections.is_empty() || source.selections.len() > 1024 {
            return Err("invalid state export selections".to_owned());
        }
        let digest = |text: &str| {
            if text.len() != 64
                || !text
                    .bytes()
                    .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
            {
                return Err("invalid state export digest".to_owned());
            }
            layerx_programs::hex::decode_digest(text)
                .map_err(|_| "invalid state export digest".to_owned())
        };
        let mut selections = BTreeMap::new();
        for selection in source.selections {
            let tenant = TenantId::new(selection.tenant)
                .map_err(|_| "invalid state export tenant".to_owned())?;
            let account = digest(&selection.account)?;
            let slots = selection
                .slots
                .iter()
                .map(|slot| digest(slot))
                .collect::<Result<Vec<_>, _>>()?;
            if account[..12] != [0; 12]
                || account[12..] == [0; 20]
                || slots.is_empty()
                || slots.len() > MAX_STATE_EXPORT_SLOTS
                || slots.iter().collect::<BTreeSet<_>>().len() != slots.len()
                || selections.insert((tenant, account), slots).is_some()
            {
                return Err("invalid state export selection".to_owned());
            }
        }
        Ok(Self { selections })
    }

    pub fn slots(&self, tenant: &TenantId, account: [u8; 32]) -> Option<&[[u8; 32]]> {
        self.selections
            .get(&(tenant.clone(), account))
            .map(Vec::as_slice)
    }
}

fn validate_protected_metadata(
    metadata: &Metadata,
    process_uid: u32,
) -> Result<(), ProtectedSourceError> {
    if !metadata.file_type().is_file()
        || metadata.uid() != process_uid
        || metadata.mode() & 0o077 != 0
    {
        return Err(ProtectedSourceError::Unprotected);
    }
    Ok(())
}

fn same_file_snapshot(left: &Metadata, right: &Metadata) -> bool {
    left.dev() == right.dev()
        && left.ino() == right.ino()
        && left.uid() == right.uid()
        && left.gid() == right.gid()
        && left.mode() == right.mode()
        && left.nlink() == right.nlink()
        && left.size() == right.size()
        && left.mtime() == right.mtime()
        && left.mtime_nsec() == right.mtime_nsec()
        && left.ctime() == right.ctime()
        && left.ctime_nsec() == right.ctime_nsec()
}

/// Validates a fully resolved map of file-key names into a typed startup configuration.
///
/// # Errors
///
/// Returns the exact missing, invalid, ambiguous, or unsafe setting without substituting a
/// default.
pub fn validate(values: &BTreeMap<String, String>) -> Result<StartupConfig, ConfigError> {
    for key in values.keys() {
        reject_retired_setting(key)?;
        if !SECURITY_RELEVANT_SETTINGS
            .iter()
            .any(|setting| setting.file_key == key)
        {
            return Err(error(key, RejectionReason::Unknown));
        }
    }
    for setting in SECURITY_RELEVANT_SETTINGS {
        let value = values
            .get(setting.file_key)
            .ok_or_else(|| error(setting.file_key, RejectionReason::Missing))?;
        if value.trim().is_empty() {
            return Err(error(setting.file_key, RejectionReason::Empty));
        }
    }

    let network_id = parse_nonzero::<u32>(values, "network_id")?;
    let expected_chain_id = parse_nonzero::<u64>(values, "expected_chain_id")?;
    let node_endpoint = parse_node_endpoint(required(values, "node_endpoint")?)?;
    let tenants = parse_tenants(required(values, "tenants")?)?;
    let policy_sources = parse_tenant_paths(
        required(values, "policy_sources")?,
        "policy_sources",
        &tenants,
    )?;
    let signer_configurations = parse_tenant_paths(
        required(values, "signer_configurations")?,
        "signer_configurations",
        &tenants,
    )?;
    let verification_defaults =
        parse_verification_defaults(required(values, "verification_defaults")?, &tenants)?;
    let receipt_key_source = match required(values, "receipt_key_source")? {
        "px_getReceiptKeys" => ReceiptKeySource::NodeRpc,
        value => ReceiptKeySource::PinnedFile(parse_path(value, "receipt_key_source")?),
    };
    Ok(StartupConfig {
        network_id,
        node_endpoint,
        expected_chain_id,
        tenants,
        policy_sources,
        signer_configurations,
        verification_defaults,
        receipt_key_source,
    })
}

fn parse_file(source: &str) -> Result<BTreeMap<String, String>, ConfigError> {
    let mut values = BTreeMap::new();
    for line in source.lines() {
        if line.len() > MAX_LINE_BYTES {
            return Err(error("configuration_file", RejectionReason::TooLarge));
        }
        let line = line.trim();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        let (key, value) = line
            .split_once('=')
            .ok_or_else(|| error("configuration_file", RejectionReason::Unknown))?;
        let key = key.trim();
        reject_retired_setting(key)?;
        if !SECURITY_RELEVANT_SETTINGS
            .iter()
            .any(|setting| setting.file_key == key)
        {
            return Err(error(key, RejectionReason::Unknown));
        }
        if values
            .insert(key.to_owned(), value.trim().to_owned())
            .is_some()
        {
            return Err(error(key, RejectionReason::Duplicate));
        }
    }
    Ok(values)
}

fn reject_retired_setting(setting: &str) -> Result<(), ConfigError> {
    if matches!(
        setting,
        "expected_protocol_version"
            | "LAYERX_EXPECTED_PROTOCOL_VERSION"
            | "sequencer_authority_source"
            | "LAYERX_SEQUENCER_AUTHORITY_SOURCE"
    ) {
        Err(error(setting, RejectionReason::Retired))
    } else {
        Ok(())
    }
}

fn parse_node_endpoint(value: &str) -> Result<String, ConfigError> {
    let invalid = || error("node_endpoint", RejectionReason::Invalid);
    let uri: ureq::http::Uri = value.parse().map_err(|_| invalid())?;
    if !matches!(uri.scheme_str(), Some("http" | "https"))
        || uri.host().is_none_or(str::is_empty)
        || uri
            .authority()
            .is_some_and(|authority| authority.as_str().contains('@'))
        || value.contains('#')
        || value
            .bytes()
            .any(|byte| byte.is_ascii_whitespace() || byte.is_ascii_control())
    {
        return Err(invalid());
    }
    Ok(value.to_owned())
}

fn required<'a>(
    values: &'a BTreeMap<String, String>,
    setting: &'static str,
) -> Result<&'a str, ConfigError> {
    values
        .get(setting)
        .map(String::as_str)
        .ok_or_else(|| error(setting, RejectionReason::Missing))
}

fn parse_nonzero<T>(
    values: &BTreeMap<String, String>,
    setting: &'static str,
) -> Result<T, ConfigError>
where
    T: std::str::FromStr + PartialEq + Default,
{
    let value = required(values, setting)?
        .parse::<T>()
        .map_err(|_| error(setting, RejectionReason::InvalidInteger))?;
    if value == T::default() {
        Err(error(setting, RejectionReason::InvalidInteger))
    } else {
        Ok(value)
    }
}

fn parse_tenants(value: &str) -> Result<BTreeSet<TenantId>, ConfigError> {
    let mut tenants = BTreeSet::new();
    for raw in value.split(',') {
        let canonical = raw.trim();
        if !valid_tenant(canonical) {
            return Err(error("tenants", RejectionReason::InvalidTenant));
        }
        let tenant = TenantId::new(canonical)
            .map_err(|_| error("tenants", RejectionReason::InvalidTenant))?;
        if !tenants.insert(tenant) {
            return Err(error("tenants", RejectionReason::InvalidTenant));
        }
    }
    if tenants.is_empty() {
        Err(error("tenants", RejectionReason::InvalidTenant))
    } else {
        Ok(tenants)
    }
}

pub fn parse_policy_sources(
    value: &str,
    tenants: &BTreeSet<TenantId>,
) -> Result<BTreeMap<TenantId, PathBuf>, ConfigError> {
    parse_tenant_paths(value, "policy_sources", tenants)
}

fn parse_tenant_paths(
    value: &str,
    setting: &'static str,
    tenants: &BTreeSet<TenantId>,
) -> Result<BTreeMap<TenantId, PathBuf>, ConfigError> {
    let mut paths = BTreeMap::new();
    for declaration in value.split(',') {
        let (tenant_text, path_text) = declaration
            .split_once(':')
            .ok_or_else(|| error(setting, RejectionReason::IncompleteTenantMap))?;
        let tenant_text = tenant_text.trim();
        if !valid_tenant(tenant_text) {
            return Err(error(setting, RejectionReason::InvalidTenant));
        }
        let tenant = TenantId::new(tenant_text)
            .map_err(|_| error(setting, RejectionReason::InvalidTenant))?;
        if !tenants.contains(&tenant) || paths.contains_key(&tenant) {
            return Err(error(setting, RejectionReason::IncompleteTenantMap));
        }
        paths.insert(tenant, parse_path(path_text.trim(), setting)?);
    }
    if paths.keys().eq(tenants.iter()) {
        Ok(paths)
    } else {
        Err(error(setting, RejectionReason::IncompleteTenantMap))
    }
}

fn parse_verification_defaults(
    value: &str,
    tenants: &BTreeSet<TenantId>,
) -> Result<BTreeMap<TenantId, VerificationLevel>, ConfigError> {
    let setting = "verification_defaults";
    let mut levels = BTreeMap::new();
    for declaration in value.split(',') {
        let (tenant_text, level_text) = declaration
            .split_once(':')
            .ok_or_else(|| error(setting, RejectionReason::IncompleteTenantMap))?;
        let tenant_text = tenant_text.trim();
        if !valid_tenant(tenant_text) {
            return Err(error(setting, RejectionReason::InvalidTenant));
        }
        let tenant = TenantId::new(tenant_text)
            .map_err(|_| error(setting, RejectionReason::InvalidTenant))?;
        if !tenants.contains(&tenant) || levels.contains_key(&tenant) {
            return Err(error(setting, RejectionReason::IncompleteTenantMap));
        }
        let level = match level_text.trim() {
            "sequencer-signed" => VerificationLevel::SEQUENCER_SIGNED,
            "batch-included" => VerificationLevel::BATCH_INCLUDED,
            "state-proven" => VerificationLevel::STATE_PROVEN,
            "committed" | "checkpoint-finalised" | "settlement-anchored" => {
                VerificationLevel::COMMITTED
            }
            _ => return Err(error(setting, RejectionReason::InvalidVerificationLevel)),
        };
        levels.insert(tenant, level);
    }
    if levels.keys().eq(tenants.iter()) {
        Ok(levels)
    } else {
        Err(error(setting, RejectionReason::IncompleteTenantMap))
    }
}

fn parse_path(value: &str, setting: &'static str) -> Result<PathBuf, ConfigError> {
    let path = PathBuf::from(value);
    if value
        .bytes()
        .any(|byte| byte == 0 || byte.is_ascii_control())
        || !path.is_absolute()
        || path == Path::new("/")
        || path
            .components()
            .any(|component| matches!(component, Component::ParentDir | Component::CurDir))
    {
        Err(error(setting, RejectionReason::InvalidPath))
    } else {
        Ok(path)
    }
}

fn valid_tenant(value: &str) -> bool {
    !value.is_empty()
        && value.bytes().all(|byte| {
            byte.is_ascii_lowercase() || byte.is_ascii_digit() || matches!(byte, b'-' | b'_' | b'.')
        })
}

fn error(setting: impl Into<String>, reason: RejectionReason) -> ConfigError {
    ConfigError {
        setting: setting.into(),
        reason,
    }
}

pub fn load_program_budget_denominations(
    path: &Path,
    tenant: &TenantId,
) -> Result<crate::enrolment::VerifiedProgramBudgetDenominations, ConfigError> {
    use sha2::{Digest as _, Sha256};
    const SETTING: &str = "program_budget_denominations";
    let bytes = read_protected_source(path, MAX_CONFIG_BYTES).map_err(|failure| {
        error(
            SETTING,
            match failure {
                ProtectedSourceError::Unavailable => RejectionReason::Unavailable,
                ProtectedSourceError::TooLarge => RejectionReason::TooLarge,
                ProtectedSourceError::Unprotected | ProtectedSourceError::Changed => {
                    RejectionReason::Unprotected
                }
            },
        )
    })?;
    #[derive(serde::Deserialize)]
    #[serde(deny_unknown_fields)]
    struct Source {
        version: u8,
        tenant: String,
        limits: Vec<Row>,
    }
    #[derive(serde::Deserialize)]
    #[serde(deny_unknown_fields)]
    struct Row {
        stable_id: String,
        asset: String,
        source: String,
    }
    let value: Source = serde_json::from_slice(&bytes)
        .map_err(|_| error(SETTING, RejectionReason::InvalidEncoding))?;
    if value.version != 1 || value.tenant != tenant.as_str() {
        return Err(error(SETTING, RejectionReason::InvalidTenant));
    }
    let fixed = |text: &str| -> Result<[u8; 32], ConfigError> {
        if text.len() != 64
            || text
                .bytes()
                .any(|byte| !byte.is_ascii_digit() && !(b'a'..=b'f').contains(&byte))
        {
            return Err(error(SETTING, RejectionReason::InvalidEncoding));
        }
        layerx_programs::hex::decode_digest(text)
            .map_err(|_| error(SETTING, RejectionReason::InvalidEncoding))
    };
    let mut bindings = Vec::new();
    for row in value.limits {
        let stable_id = fixed(&row.stable_id)?;
        let asset = fixed(&row.asset)?;
        let source = if row.source == "*" {
            None
        } else {
            Some(fixed(&row.source)?)
        };
        bindings.push((
            stable_id,
            crate::budget::ProgramLimitDenomination { asset, source },
        ));
    }
    crate::enrolment::VerifiedProgramBudgetDenominations::from_protected_source(
        tenant.clone(),
        bindings,
        Sha256::digest(&bytes).into(),
    )
    .map_err(|_| error(SETTING, RejectionReason::InvalidEncoding))
}

#[cfg(test)]
mod program_budget_configuration_tests {
    use super::*;
    use std::os::unix::fs::PermissionsExt as _;

    fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
        value.unwrap_or_else(|error| panic!("Program budget configuration: {error:?}"))
    }

    #[test]
    fn committed_verification_default_accepts_current_and_legacy_names() {
        let tenant = must(TenantId::new("committed-owner"));
        let tenants = BTreeSet::from([tenant.clone()]);
        for name in ["committed", "checkpoint-finalised", "settlement-anchored"] {
            let levels = must(parse_verification_defaults(
                &format!("committed-owner:{name}"),
                &tenants,
            ));
            assert_eq!(levels[&tenant], VerificationLevel::COMMITTED);
        }
        assert!(parse_verification_defaults("committed-owner:unknown", &tenants).is_err());
    }

    #[test]
    fn protected_source_requires_explicit_unique_asset_source_and_tenant() {
        let suffix =
            must(std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH)).as_nanos();
        let root = std::env::temp_dir().join(format!(
            "program-budget-source-{}-{suffix}",
            std::process::id()
        ));
        must(std::fs::create_dir_all(&root));
        let path = root.join("source.json");
        let tenant = must(TenantId::new("budget-owner"));
        let valid = serde_json::json!({"version":1,"tenant":tenant.as_str(),"limits":[{
            "stable_id":hex::encode([1_u8;32]),"asset":hex::encode([2_u8;32]),"source":"*"}]});
        let write = |value: &serde_json::Value| {
            must(std::fs::write(&path, must(serde_json::to_vec(value))));
            must(std::fs::set_permissions(
                &path,
                std::fs::Permissions::from_mode(0o600),
            ));
        };
        write(&valid);
        let verified = must(load_program_budget_denominations(&path, &tenant));
        assert_eq!(verified.bindings()[0].1.source, None);
        assert!(
            load_program_budget_denominations(&path, &must(TenantId::new("other-owner"))).is_err()
        );
        let mut missing = valid.clone();
        missing["limits"][0]
            .as_object_mut()
            .unwrap_or_else(|| panic!("row"))
            .remove("source");
        write(&missing);
        assert!(load_program_budget_denominations(&path, &tenant).is_err());
        let mut duplicate = valid.clone();
        duplicate["limits"]
            .as_array_mut()
            .unwrap_or_else(|| panic!("limits"))
            .push(valid["limits"][0].clone());
        write(&duplicate);
        assert!(load_program_budget_denominations(&path, &tenant).is_err());
        let mut zero = valid.clone();
        zero["limits"][0]["asset"] = serde_json::Value::String(hex::encode([0_u8; 32]));
        write(&zero);
        assert!(load_program_budget_denominations(&path, &tenant).is_err());
        let bytes = must(serde_json::to_string(&valid)).replacen("{", "{\"version\":1,", 1);
        must(std::fs::write(&path, bytes));
        assert!(load_program_budget_denominations(&path, &tenant).is_err());
        write(&valid);
        must(std::fs::set_permissions(
            &path,
            std::fs::Permissions::from_mode(0o666),
        ));
        assert!(load_program_budget_denominations(&path, &tenant).is_err());
        must(std::fs::remove_dir_all(root));
    }
}

pub(crate) struct VerifiedNativeProgramPolicy {
    tenant: TenantId,
    policy: crate::policy::native_program::NativeProgramPolicy,
}

impl VerifiedNativeProgramPolicy {
    pub(crate) fn tenant(&self) -> &TenantId {
        &self.tenant
    }
    pub(crate) fn policy(&self) -> &crate::policy::native_program::NativeProgramPolicy {
        &self.policy
    }
}

pub(crate) fn load_native_program_policy(
    path: &Path,
    tenant: &TenantId,
) -> Result<VerifiedNativeProgramPolicy, ConfigError> {
    #[derive(serde::Deserialize)]
    #[serde(deny_unknown_fields)]
    struct Source {
        version: String,
        tenant: String,
        policy: String,
    }
    let bytes = read_protected_source(path, 2 * 1024 * 1024)
        .map_err(|_| error("native_policy_sources", RejectionReason::Unprotected))?;
    let source: Source = serde_json::from_slice(&bytes)
        .map_err(|_| error("native_policy_sources", RejectionReason::InvalidEncoding))?;
    if source.version != "layerx.native-program-policy-source.v1"
        || source.tenant != tenant.as_str()
    {
        return Err(error(
            "native_policy_sources",
            RejectionReason::InvalidTenant,
        ));
    }
    let policy = crate::policy::native_program::NativeProgramPolicy::load(source.policy.as_bytes())
        .map_err(|_| error("native_policy_sources", RejectionReason::InvalidEncoding))?;
    Ok(VerifiedNativeProgramPolicy {
        tenant: tenant.clone(),
        policy,
    })
}

pub(crate) struct VerifiedNativeEffectPolicy {
    tenant: TenantId,
    policy: crate::policy::native_program::NativeEffectPolicy,
}

impl VerifiedNativeEffectPolicy {
    pub(crate) fn tenant(&self) -> &TenantId {
        &self.tenant
    }
    pub(crate) fn policy(&self) -> &crate::policy::native_program::NativeEffectPolicy {
        &self.policy
    }
}

pub(crate) fn load_native_effect_policy(
    path: &Path,
    tenant: &TenantId,
) -> Result<VerifiedNativeEffectPolicy, ConfigError> {
    #[derive(serde::Deserialize)]
    #[serde(deny_unknown_fields)]
    struct Source {
        version: String,
        tenant: String,
        policy: String,
    }
    let bytes = read_protected_source(path, 2 * 1024 * 1024)
        .map_err(|_| error("native_effect_policy_sources", RejectionReason::Unprotected))?;
    let source: Source = serde_json::from_slice(&bytes).map_err(|_| {
        error(
            "native_effect_policy_sources",
            RejectionReason::InvalidEncoding,
        )
    })?;
    if source.version != "layerx.native-effect-policy-source.v1" || source.tenant != tenant.as_str()
    {
        return Err(error(
            "native_effect_policy_sources",
            RejectionReason::InvalidTenant,
        ));
    }
    let policy = crate::policy::native_program::NativeEffectPolicy::load(source.policy.as_bytes())
        .map_err(|_| {
            error(
                "native_effect_policy_sources",
                RejectionReason::InvalidEncoding,
            )
        })?;
    Ok(VerifiedNativeEffectPolicy {
        tenant: tenant.clone(),
        policy,
    })
}
