//! Agent-daemon enrolment of one model context protocol session and the binding document
//! the served path reads.
//!
//! Enrolment opens a capability-grant session for a registered identity and publishes the
//! binding document beside two operator-protected secret files: the session token and the
//! daemon bearer. The served path holds no other credential. The document also declares the
//! protected Unix listener the socket server binds and the peer user ids it admits, and the
//! search sidecar reference when the web tools are enabled.

use std::collections::BTreeSet;
use std::fmt;
use std::fs::{self, DirBuilder, OpenOptions};
use std::io::{self, Write as _};
use std::os::unix::fs::{
    DirBuilderExt as _, MetadataExt as _, OpenOptionsExt as _, PermissionsExt as _,
};
use std::path::{Component, Path, PathBuf};
use std::time::Duration;

use layerx_programs::hex;
use serde_json::{json, Value};
use zeroize::Zeroizing;

#[path = "../../../../../tools/platform/cli/src/install/binding_shape.rs"]
pub mod binding_shape;

// Expose the daemon's existing TLS implementation to in-process transport consumers.
pub use rustls as readiness_tls;

use binding_shape::{
    AgentDeclaration, BindingDocument, LimitDeclaration, ReadinessTransport, MAX_DOCUMENT_BYTES,
};

use crate::budget::{LimitConfig, LimitScope};
use crate::capability::{Capability, CapabilityId};
use crate::identity::{IdentityRecord, ProtocolAuthority};
use crate::session::{self, OpenRequest, SessionError, SessionId, SessionRegistry};
use crate::store::{Store, TenantId};

/// File name of the binding document inside the binding directory.
pub const BINDING_FILE: &str = "binding.json";
/// File name of the session token the served path reads.
pub const SESSION_TOKEN_FILE: &str = "session-token";
/// File name of the daemon bearer the served path presents to the agent daemon.
pub const DAEMON_BEARER_FILE: &str = "daemon-bearer";

const MINIMUM_BEARER_BYTES: usize = 32;
const MAX_TEXT_BYTES: usize = 255;
const TOKEN_ATTEMPTS: usize = 8;
const MAX_SOCKET_PATH_BYTES: usize = 107;
const MAX_ADMITTED_PEERS: usize = 64;
const LISTENER_MODE_MASK: u32 = 0o660;
const LISTENER_OWNER_ACCESS: u32 = 0o600;
const MAX_WEB_TIMEOUT_MS: u64 = 60_000;
const MAX_WEB_PENDING_ATTEMPTS: u8 = 10;

/// Deployment mode the binding document declares.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum BindingMode {
    Full,
    ReadOnly,
}

impl BindingMode {
    /// Returns the label the binding document carries.
    #[must_use]
    pub const fn label(self) -> &'static str {
        match self {
            Self::Full => "full",
            Self::ReadOnly => "read-only",
        }
    }
}

/// Typed refusal of one enrolment. It never carries secret material.
#[derive(Debug)]
pub enum EnrolmentError {
    InvalidSurface(&'static str),
    InvalidPath(&'static str),
    InvalidLimit(&'static str),
    MissingCapability,
    InvalidListener(&'static str),
    InvalidWeb(&'static str),
    AlreadyPublished(PathBuf),
    Session(SessionError),
    OrphanedSession(SessionId, Box<EnrolmentError>),
    Capability(crate::capability::CapabilityError),
    Io(io::Error),
    Encoding(&'static str),
}

impl fmt::Display for EnrolmentError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::InvalidSurface(reason) => {
                write!(formatter, "the daemon surface is invalid: {reason}")
            }
            Self::InvalidPath(reason) => write!(formatter, "a binding path is invalid: {reason}"),
            Self::InvalidLimit(reason) => write!(formatter, "the limit is invalid: {reason}"),
            Self::MissingCapability => {
                formatter.write_str("the capability is not persisted for this tenant")
            }
            Self::InvalidListener(reason) => {
                write!(formatter, "the protocol listener is invalid: {reason}")
            }
            Self::InvalidWeb(reason) => {
                write!(formatter, "the web tool reference is invalid: {reason}")
            }
            Self::AlreadyPublished(path) => write!(
                formatter,
                "a different binding already occupies {}",
                path.display()
            ),
            Self::Session(error) => write!(formatter, "the session was refused: {error:?}"),
            Self::OrphanedSession(session, cause) => write!(
                formatter,
                "session {} stayed open after publication failed: {cause}",
                hex::encode(&session.0)
            ),
            Self::Capability(error) => write!(formatter, "the capability is unusable: {error:?}"),
            Self::Io(error) => write!(formatter, "binding publication failed: {error}"),
            Self::Encoding(reason) => write!(formatter, "the binding cannot be encoded: {reason}"),
        }
    }
}

impl std::error::Error for EnrolmentError {}

impl From<io::Error> for EnrolmentError {
    fn from(value: io::Error) -> Self {
        Self::Io(value)
    }
}

/// The loopback agent daemon surface the served path authorizes against.
pub struct DaemonSurface {
    endpoint: String,
    bearer: Zeroizing<String>,
    probe_program: [u8; 32],
    readiness: Option<ReadinessDeclaration>,
}

impl fmt::Debug for DaemonSurface {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter
            .debug_struct("DaemonSurface")
            .field("endpoint", &self.endpoint)
            .field("bearer", &"[redacted]")
            .field("probe_program", &hex::encode(&self.probe_program))
            .field("readiness", &self.readiness)
            .finish()
    }
}

impl DaemonSurface {
    /// Validates the daemon endpoint, bearer, and probe program the binding will name.
    ///
    /// # Errors
    ///
    /// Refuses a non-loopback endpoint, a bearer shorter than the daemon accepts, an empty
    /// probe program id, and any text the binding document cannot carry.
    pub fn new(
        endpoint: &str,
        bearer: String,
        probe_program: [u8; 32],
    ) -> Result<Self, EnrolmentError> {
        if !endpoint.starts_with("127.0.0.1:") || endpoint.len() > MAX_TEXT_BYTES {
            return Err(EnrolmentError::InvalidSurface(
                "the agent daemon endpoint is not a loopback endpoint",
            ));
        }
        let bearer = Zeroizing::new(bearer);
        if bearer.len() < MINIMUM_BEARER_BYTES
            || bearer.len() >= 4_096
            || bearer.trim() != bearer.as_str()
        {
            return Err(EnrolmentError::InvalidSurface(
                "the agent daemon bearer is shorter than the daemon accepts or carries whitespace",
            ));
        }
        if probe_program == [0; 32] {
            return Err(EnrolmentError::InvalidSurface(
                "the probe program is not a program id",
            ));
        }
        Ok(Self {
            endpoint: endpoint.to_owned(),
            bearer,
            probe_program,
            readiness: None,
        })
    }

    /// Declares the authenticated gateway used by MCP daemon clients.
    #[must_use]
    pub fn with_readiness(mut self, readiness: ReadinessDeclaration) -> Self {
        self.readiness = Some(readiness);
        self
    }
}

/// Authenticated HTTPS transport references carried unchanged into the MCP binding.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReadinessDeclaration {
    endpoint: String,
    gateway_key_file: PathBuf,
    trust_anchors: Option<PathBuf>,
}

impl ReadinessDeclaration {
    /// Validates a gateway origin and absolute credential and optional trust-anchor paths.
    ///
    /// # Errors
    ///
    /// Refuses insecure origins, URL credentials, queries, fragments and invalid paths.
    pub fn new(
        endpoint: &str,
        gateway_key_file: PathBuf,
        trust_anchors: Option<PathBuf>,
    ) -> Result<Self, EnrolmentError> {
        let invalid =
            || EnrolmentError::InvalidSurface("the readiness gateway is not an HTTPS origin");
        let uri: ureq::http::Uri = endpoint.parse().map_err(|_| invalid())?;
        if endpoint.len() > MAX_TEXT_BYTES
            || uri.scheme_str() != Some("https")
            || uri.host().is_none_or(str::is_empty)
            || uri
                .authority()
                .is_some_and(|authority| authority.as_str().contains('@'))
            || uri.path() != "/"
            || uri.query().is_some()
            || endpoint.contains('#')
            || endpoint
                .bytes()
                .any(|byte| byte.is_ascii_whitespace() || byte.is_ascii_control())
        {
            return Err(invalid());
        }
        path_text(&gateway_key_file)?;
        if let Some(path) = &trust_anchors {
            path_text(path)?;
        }
        Ok(Self {
            endpoint: endpoint.to_owned(),
            gateway_key_file,
            trust_anchors,
        })
    }

    fn transport(&self) -> Result<ReadinessTransport, EnrolmentError> {
        Ok(ReadinessTransport {
            endpoint: self.endpoint.clone(),
            gateway_key_file: path_text(&self.gateway_key_file)?.to_owned(),
            trust_anchors: self
                .trust_anchors
                .as_deref()
                .map(path_text)
                .transpose()?
                .map(str::to_owned),
        })
    }
}

/// The protected Unix listener the socket server binds and the peer user ids it admits.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ListenerDeclaration {
    socket: PathBuf,
    owner_uid: u32,
    owner_gid: u32,
    mode: u32,
    admitted_uids: Vec<u32>,
}

impl ListenerDeclaration {
    /// Validates the socket path, its owner and mode, and the admitted peer user ids.
    ///
    /// # Errors
    ///
    /// Refuses a relative, non-UTF-8, non-normal or over-long socket path, a socket directly in
    /// the root directory, a mode that grants more than owner and group read and write or
    /// withholds owner read and write, and an admission set that is
    /// empty, larger than the served path accepts, or names one user id twice.
    pub fn new(
        socket: PathBuf,
        owner_uid: u32,
        owner_gid: u32,
        mode: u32,
        admitted_uids: Vec<u32>,
    ) -> Result<Self, EnrolmentError> {
        if !socket.is_absolute() {
            return Err(EnrolmentError::InvalidListener(
                "the socket path is not absolute",
            ));
        }
        let text = socket.to_str().ok_or(EnrolmentError::InvalidListener(
            "the socket path is not UTF-8",
        ))?;
        if text.len() > MAX_SOCKET_PATH_BYTES || text.as_bytes().contains(&0) {
            return Err(EnrolmentError::InvalidListener(
                "the socket path is longer than a Unix socket address carries",
            ));
        }
        if socket
            .components()
            .any(|component| !matches!(component, Component::RootDir | Component::Normal(_)))
            || socket.file_name().is_none()
            || socket
                .parent()
                .is_none_or(|parent| parent == Path::new("/"))
            || text.ends_with('/')
        {
            return Err(EnrolmentError::InvalidListener(
                "the socket path is not a normal file path inside a dedicated directory",
            ));
        }
        if mode & !LISTENER_MODE_MASK != 0 {
            return Err(EnrolmentError::InvalidListener(
                "the socket mode grants more than owner and group read and write",
            ));
        }
        if mode & LISTENER_OWNER_ACCESS != LISTENER_OWNER_ACCESS {
            return Err(EnrolmentError::InvalidListener(
                "the socket mode withholds owner read and write",
            ));
        }
        if admitted_uids.is_empty() || admitted_uids.len() > MAX_ADMITTED_PEERS {
            return Err(EnrolmentError::InvalidListener(
                "the admitted peers must name 1 to 64 user ids",
            ));
        }
        let mut seen = BTreeSet::new();
        for uid in &admitted_uids {
            if !seen.insert(*uid) {
                return Err(EnrolmentError::InvalidListener(
                    "the admitted peers name one user id twice",
                ));
            }
        }
        Ok(Self {
            socket,
            owner_uid,
            owner_gid,
            mode,
            admitted_uids,
        })
    }

    /// Confirms the socket directory exists as a canonical directory owned by the declared
    /// owner and group with no access for others, the same directory the socket server
    /// validates before it binds.
    fn check_directory(&self) -> Result<(), EnrolmentError> {
        let parent = self.socket.parent().ok_or(EnrolmentError::InvalidListener(
            "the socket path has no directory",
        ))?;
        let metadata = match fs::symlink_metadata(parent) {
            Ok(metadata) => metadata,
            Err(error) if error.kind() == io::ErrorKind::NotFound => {
                return Err(EnrolmentError::InvalidListener(
                    "the socket directory does not exist",
                ))
            }
            Err(error) => return Err(EnrolmentError::Io(error)),
        };
        if metadata.file_type().is_symlink() || !metadata.is_dir() {
            return Err(EnrolmentError::InvalidListener(
                "the socket directory is not a directory",
            ));
        }
        if fs::canonicalize(parent)? != parent {
            return Err(EnrolmentError::InvalidListener(
                "the socket directory path is not canonical",
            ));
        }
        if metadata.uid() != self.owner_uid
            || metadata.gid() != self.owner_gid
            || metadata.uid() != fs::metadata("/proc/self")?.uid()
            || metadata.mode() & 0o027 != 0
        {
            return Err(EnrolmentError::InvalidListener(
                "the socket directory is not owned by the declared owner and group or admits others",
            ));
        }
        Ok(())
    }

    fn document(&self) -> Result<Value, EnrolmentError> {
        let socket = self.socket.to_str().ok_or(EnrolmentError::InvalidListener(
            "the socket path is not UTF-8",
        ))?;
        Ok(json!({
            "socket": socket,
            "owner_uid": self.owner_uid,
            "owner_gid": self.owner_gid,
            "mode": format!("{:04o}", self.mode),
            "admitted_uids": self.admitted_uids,
        }))
    }
}

/// The public payment addresses and governed receipt keys carried by the web binding.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WebDeclaration {
    endpoint: String,
    network: String,
    payer_address: [u8; 20],
    receiver_address: [u8; 20],
    receipt_key_index: Vec<layerx_client::sequencer::ReceiptKeyEntry>,
    selected_key: usize,
    timeout_ms: u64,
    pending_attempts: u8,
    approval_threshold: u128,
}

impl WebDeclaration {
    /// Validates the current closed-key MCP web binding, including the selected key range.
    ///
    /// # Errors
    ///
    /// Refuses invalid addresses, a network other than eip155:125, an invalid or duplicate
    /// receipt key, an absent selected key, or out-of-bound transport settings.
    #[allow(clippy::too_many_arguments)]
    pub fn new(
        endpoint: String,
        network: String,
        payer_address: [u8; 20],
        receiver_address: [u8; 20],
        receipt_key_index: Vec<layerx_client::sequencer::ReceiptKeyEntry>,
        selected_key: usize,
        timeout_ms: u64,
        pending_attempts: u8,
        approval_threshold: u128,
    ) -> Result<Self, EnrolmentError> {
        if endpoint.is_empty() || endpoint.len() > MAX_TEXT_BYTES || !valid_web_endpoint(&endpoint)
        {
            return Err(EnrolmentError::InvalidWeb(
                "the web endpoint is not an HTTP sidecar authority",
            ));
        }
        if network != "eip155:125" {
            return Err(EnrolmentError::InvalidWeb(
                "the web network must be eip155:125",
            ));
        }
        if payer_address == [0; 20] || receiver_address == [0; 20] {
            return Err(EnrolmentError::InvalidWeb(
                "the web payment addresses must be nonzero",
            ));
        }
        if receipt_key_index.is_empty()
            || receipt_key_index.len() > 64
            || selected_key >= receipt_key_index.len()
        {
            return Err(EnrolmentError::InvalidWeb(
                "the web receipt key index or selection is invalid",
            ));
        }
        let mut seen = BTreeSet::new();
        for key in &receipt_key_index {
            if key.key_id == [0; 32]
                || key.public_key == [0; 32]
                || key.first_height > key.last_height
                || !seen.insert(key.key_id)
            {
                return Err(EnrolmentError::InvalidWeb(
                    "the web receipt key index has an invalid or repeated key",
                ));
            }
        }
        if timeout_ms == 0 || timeout_ms > MAX_WEB_TIMEOUT_MS {
            return Err(EnrolmentError::InvalidWeb(
                "the web timeout must be 1 to 60,000 milliseconds",
            ));
        }
        if pending_attempts == 0 || pending_attempts > MAX_WEB_PENDING_ATTEMPTS {
            return Err(EnrolmentError::InvalidWeb(
                "the web pending attempts must be 1 to 10",
            ));
        }
        Ok(Self {
            endpoint,
            network,
            payer_address,
            receiver_address,
            receipt_key_index,
            selected_key,
            timeout_ms,
            pending_attempts,
            approval_threshold,
        })
    }

    fn document(&self) -> Value {
        let key = &self.receipt_key_index[self.selected_key];
        let keys: Vec<_> = self
            .receipt_key_index
            .iter()
            .map(|key| {
                json!({
                    "keyId": hex::encode(&key.key_id),
                    "publicKey": hex::encode(&key.public_key),
                    "firstHeight": key.first_height,
                    "lastHeight": key.last_height,
                })
            })
            .collect();
        json!({
            "endpoint": self.endpoint,
            "payer_address": format!("0x{}", hex::encode(&self.payer_address)),
            "receiver_address": format!("0x{}", hex::encode(&self.receiver_address)),
            "network": self.network,
            "receipt_key_index": keys,
            "sequencer_id": hex::encode(&key.key_id),
            "first_batch_number": key.first_height,
            "last_batch_number": key.last_height,
            "timeout_ms": self.timeout_ms,
            "pending_attempts": self.pending_attempts,
            "approval_threshold": self.approval_threshold.to_string(),
        })
    }
}

fn valid_web_endpoint(text: &str) -> bool {
    let Some(authority) = text.strip_prefix("http://") else {
        return false;
    };
    let authority = authority.strip_suffix('/').unwrap_or(authority);
    let (host, port) = if let Some(rest) = authority.strip_prefix('[') {
        let Some((host, tail)) = rest.split_once(']') else {
            return false;
        };
        if !tail.is_empty() && !tail.starts_with(':') {
            return false;
        }
        (host, tail.strip_prefix(':'))
    } else {
        match authority.split_once(':') {
            Some((host, port)) => (host, Some(port)),
            None => (authority, None),
        }
    };
    let port = match port {
        Some(port) if port.bytes().all(|byte| byte.is_ascii_digit()) => {
            let Ok(port) = port.parse::<u16>() else {
                return false;
            };
            port
        }
        Some(_) => return false,
        None => 80,
    };
    !host.is_empty()
        && port != 0
        && host
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || b".-:".contains(&byte))
}

/// One opened session together with the coordinates the binding document records.
pub struct Enrolment {
    tenant: TenantId,
    session_id: SessionId,
    session_generation: u64,
    capability_id: CapabilityId,
    core_height: u64,
    token: Zeroizing<[u8; 32]>,
}

impl fmt::Debug for Enrolment {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter
            .debug_struct("Enrolment")
            .field("tenant", &self.tenant)
            .field("session_id", &hex::encode(&self.session_id.0))
            .field("session_generation", &self.session_generation)
            .field("capability_id", &hex::encode(&self.capability_id.0))
            .field("core_height", &self.core_height)
            .field("token", &"[redacted]")
            .finish()
    }
}

/// The published binding and the protected files beside it.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PublishedBinding {
    pub binding: PathBuf,
    pub session_token_file: PathBuf,
    pub daemon_bearer_file: PathBuf,
    pub session_id: SessionId,
    pub session_generation: u64,
    pub created: bool,
}

/// Everything one enrolment needs beyond the identity and the store.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct EnrolmentRequest {
    pub session_id: SessionId,
    pub capability_id: CapabilityId,
    pub permitted_activity_types: BTreeSet<u16>,
    pub scopes: BTreeSet<String>,
    pub expiry_sequence: u64,
    pub opening_client: String,
    pub policy_version: String,
    pub core_sequence: u64,
}

/// Native capability grant time bound the Human authority verified for one tenant, agent and
/// grant through its LXGS2 grant scope.
///
/// Only `human_runtime::verify_enrolment_expiry` constructs it from the verified scope's
/// not-after instant, so it carries no time value a caller can supply.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedGrantExpiry {
    tenant: TenantId,
    agent: layerx_types::ids::Did,
    capability_id: CapabilityId,
    not_after_ms: u64,
}

impl VerifiedGrantExpiry {
    pub(crate) const fn verified(
        tenant: TenantId,
        agent: layerx_types::ids::Did,
        capability_id: CapabilityId,
        not_after_ms: u64,
    ) -> Self {
        Self {
            tenant,
            agent,
            capability_id,
            not_after_ms,
        }
    }

    /// Returns the verified grant's not-after instant in unix milliseconds.
    #[must_use]
    pub const fn not_after_ms(&self) -> u64 {
        self.not_after_ms
    }
}

/// An enrolment request admitted together with its verified native grant time bound.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedExpiryEnrolment {
    request: EnrolmentRequest,
    expiry: VerifiedGrantExpiry,
}

impl VerifiedExpiryEnrolment {
    /// Pairs an unchanged enrolment request with the grant time bound verified for it.
    #[must_use]
    pub const fn new(request: EnrolmentRequest, expiry: VerifiedGrantExpiry) -> Self {
        Self { request, expiry }
    }

    /// Returns the enrolment request exactly as supplied.
    #[must_use]
    pub const fn request(&self) -> &EnrolmentRequest {
        &self.request
    }

    /// Returns the verified grant time bound.
    #[must_use]
    pub const fn expiry(&self) -> &VerifiedGrantExpiry {
        &self.expiry
    }
}

/// Writes binding documents for one daemon into one binding directory.
#[derive(Debug)]
pub struct BindingPublisher {
    root: PathBuf,
    store: PathBuf,
    audit_root: PathBuf,
    surface: DaemonSurface,
    limit: LimitConfig,
    deadline: Duration,
    mode: BindingMode,
    listener: Option<ListenerDeclaration>,
    web: Option<WebDeclaration>,
}

impl BindingPublisher {
    /// Validates the binding directory, the daemon records it names, and the limit it declares.
    ///
    /// # Errors
    ///
    /// Refuses a relative or over-long path, a zero deadline, and a limit without a name.
    pub fn new(
        root: PathBuf,
        store: PathBuf,
        audit_root: PathBuf,
        surface: DaemonSurface,
        limit: LimitConfig,
        deadline: Duration,
        mode: BindingMode,
    ) -> Result<Self, EnrolmentError> {
        for path in [&root, &store, &audit_root] {
            path_text(path)?;
        }
        for file in [BINDING_FILE, SESSION_TOKEN_FILE, DAEMON_BEARER_FILE] {
            path_text(&root.join(file))?;
        }
        if deadline.as_millis() == 0 {
            return Err(EnrolmentError::InvalidSurface(
                "the transport deadline is zero",
            ));
        }
        u64::try_from(deadline.as_millis()).map_err(|_| {
            EnrolmentError::Encoding("the transport deadline exceeds u64 milliseconds")
        })?;
        if limit.name.is_empty() || limit.name.len() > MAX_TEXT_BYTES {
            return Err(EnrolmentError::InvalidLimit(
                "the limit name must be 1 to 255 bytes",
            ));
        }
        Ok(Self {
            root,
            store,
            audit_root,
            surface,
            limit,
            deadline,
            mode,
            listener: None,
            web: None,
        })
    }

    /// Declares the protected listener the socket server binds and the peers it admits.
    #[must_use]
    pub fn with_listener(mut self, listener: ListenerDeclaration) -> Self {
        self.listener = Some(listener);
        self
    }

    /// Declares the search sidecar reference the web tools are served through.
    #[must_use]
    pub fn with_web(mut self, web: WebDeclaration) -> Self {
        self.web = Some(web);
        self
    }

    /// Returns the path of the binding document this publisher writes.
    #[must_use]
    pub fn binding_path(&self) -> PathBuf {
        self.root.join(BINDING_FILE)
    }

    /// Publishes the binding document, the session token, and the daemon bearer for one
    /// enrolment as owner-only files inside an owner-only directory.
    ///
    /// An identical binding already at the path is left in place and reported as not created;
    /// a different binding at the path is refused rather than replaced.
    ///
    /// # Errors
    ///
    /// Refuses a declared socket path inside the binding directory or in a socket directory
    /// that is missing, non-canonical, or not owned by the declared owner and group, a binding directory that is not a canonical directory, a different published
    /// binding, a pre-existing secret file, and any I/O failure while writing.
    pub fn publish(&self, enrolment: &Enrolment) -> Result<PublishedBinding, EnrolmentError> {
        self.check_listener()?;
        let document = self.document(enrolment)?;
        prepare_root(&self.root)?;
        let binding = self.binding_path();
        let session_token_file = self.root.join(SESSION_TOKEN_FILE);
        let daemon_bearer_file = self.root.join(DAEMON_BEARER_FILE);
        if let Some(existing) = read_document(&binding)? {
            if existing != document {
                return Err(EnrolmentError::AlreadyPublished(binding));
            }
            self.check_secrets(enrolment)?;
            return Ok(PublishedBinding {
                binding,
                session_token_file,
                daemon_bearer_file,
                session_id: enrolment.session_id,
                session_generation: enrolment.session_generation,
                created: false,
            });
        }
        let encoded = serde_json::to_vec_pretty(&document)
            .map_err(|_| EnrolmentError::Encoding("the binding document is not serializable"))?;
        if encoded.len() > MAX_DOCUMENT_BYTES {
            return Err(EnrolmentError::Encoding(
                "the binding document exceeds the served path's bound",
            ));
        }
        let token = Zeroizing::new(hex::encode(enrolment.token.as_slice()));
        let mut written: Vec<&Path> = Vec::with_capacity(3);
        let result = (|| {
            create_private(&session_token_file, token.as_bytes())?;
            written.push(&session_token_file);
            create_private(&daemon_bearer_file, self.surface.bearer.as_bytes())?;
            written.push(&daemon_bearer_file);
            create_private(&binding, &encoded)?;
            written.push(&binding);
            sync_directory(&self.root)
        })();
        if let Err(error) = result {
            for path in written.iter().rev() {
                if let Err(removal) = fs::remove_file(path) {
                    if removal.kind() != io::ErrorKind::NotFound {
                        return Err(EnrolmentError::Io(removal));
                    }
                }
            }
            return Err(error);
        }
        Ok(PublishedBinding {
            binding,
            session_token_file,
            daemon_bearer_file,
            session_id: enrolment.session_id,
            session_generation: enrolment.session_generation,
            created: true,
        })
    }

    fn check_listener(&self) -> Result<(), EnrolmentError> {
        let Some(listener) = &self.listener else {
            return Ok(());
        };
        if listener.socket.starts_with(&self.root) {
            return Err(EnrolmentError::InvalidListener(
                "the socket path lies inside the protected binding directory",
            ));
        }
        listener.check_directory()
    }

    pub fn validate_existing(
        &self,
        sessions: &SessionRegistry,
        identity: &IdentityRecord,
        request: &EnrolmentRequest,
    ) -> Result<(), EnrolmentError> {
        self.check_listener()?;
        let metadata = fs::symlink_metadata(&self.root)?;
        if !metadata.is_dir()
            || metadata.uid() != fs::metadata("/proc/self")?.uid()
            || metadata.mode() & 0o777 != 0o700
            || fs::canonicalize(&self.root)? != self.root
        {
            return Err(EnrolmentError::InvalidPath(
                "the binding directory is not owner-only",
            ));
        }
        let record = sessions
            .get(identity.tenant(), request.session_id)
            .ok_or(EnrolmentError::Session(SessionError::NotFound))?;
        if !record.open
            || &record.request.agent != identity.did()
            || record.request.authority
                != ProtocolAuthority::CapabilityGrant(request.capability_id.0)
            || record.request.permitted_activity_types != request.permitted_activity_types
            || record.request.scopes != request.scopes
            || record.request.expiry_sequence != request.expiry_sequence
            || record.request.opening_client != request.opening_client
            || record.request.policy_version != request.policy_version
        {
            return Err(EnrolmentError::Session(SessionError::IdentityMismatch));
        }
        let enrolment = Enrolment {
            tenant: identity.tenant().clone(),
            session_id: request.session_id,
            session_generation: record.generation,
            capability_id: request.capability_id,
            core_height: request.core_sequence,
            token: Zeroizing::new(record.request.token_id),
        };
        let binding = self.binding_path();
        if read_document(&binding)?.as_ref() != Some(&self.document(&enrolment)?) {
            return Err(EnrolmentError::AlreadyPublished(binding));
        }
        self.check_secrets(&enrolment)
    }

    fn check_secrets(&self, enrolment: &Enrolment) -> Result<(), EnrolmentError> {
        let token = Zeroizing::new(hex::encode(enrolment.token.as_slice()));
        for (name, expected) in [
            (SESSION_TOKEN_FILE, token.as_bytes()),
            (DAEMON_BEARER_FILE, self.surface.bearer.as_bytes()),
        ] {
            let bytes = Zeroizing::new(
                crate::config::read_protected_source(&self.root.join(name), 4_096).map_err(
                    |_| EnrolmentError::InvalidPath("a binding secret is not protected"),
                )?,
            );
            if bytes.strip_suffix(b"\n") != Some(expected) {
                return Err(EnrolmentError::InvalidPath(
                    "a binding secret differs from current authority",
                ));
            }
        }
        Ok(())
    }

    fn document(&self, enrolment: &Enrolment) -> Result<Value, EnrolmentError> {
        let (scope, scope_id) = match self.limit.scope {
            LimitScope::Tenant(id) => ("tenant", id),
            LimitScope::Agent(id) => ("agent", id),
            LimitScope::Session(id) => ("session", id),
            LimitScope::Capability(id) => ("capability", id),
            LimitScope::Counterparty(id) => ("counterparty", id),
        };
        let deadline_ms = u64::try_from(self.deadline.as_millis()).map_err(|_| {
            EnrolmentError::Encoding("the transport deadline exceeds u64 milliseconds")
        })?;
        let document = BindingDocument {
            mode: self.mode.label().to_owned(),
            tenant: enrolment.tenant.as_str().to_owned(),
            store: path_text(&self.store)?.to_owned(),
            audit_root: path_text(&self.audit_root)?.to_owned(),
            session_id: hex::encode(&enrolment.session_id.0),
            session_token_file: path_text(&self.root.join(SESSION_TOKEN_FILE))?.to_owned(),
            session_generation: enrolment.session_generation,
            capability_id: hex::encode(&enrolment.capability_id.0),
            core_height: enrolment.core_height,
            deadline_ms,
            agent: AgentDeclaration {
                endpoint: self.surface.endpoint.clone(),
                bearer_file: path_text(&self.root.join(DAEMON_BEARER_FILE))?.to_owned(),
                probe_program: hex::encode(&self.surface.probe_program),
                readiness: self
                    .surface
                    .readiness
                    .as_ref()
                    .map(ReadinessDeclaration::transport)
                    .transpose()?,
            },
            limit: LimitDeclaration {
                id: hex::encode(&self.limit.id.0),
                name: self.limit.name.clone(),
                scope: scope.to_owned(),
                scope_id: hex::encode(&scope_id),
                ceiling: self.limit.ceiling.to_string(),
                consumed: self.limit.consumed.to_string(),
            },
            listener: self
                .listener
                .as_ref()
                .map(ListenerDeclaration::document)
                .transpose()?,
            web: self.web.as_ref().map(WebDeclaration::document),
        };
        serde_json::to_value(document)
            .map_err(|_| EnrolmentError::Encoding("the binding cannot be encoded"))
    }
}

/// Opens one capability-grant session for a registered identity and publishes its binding.
///
/// The capability must already be persisted for the identity's tenant and the identity must
/// carry the matching capability grant; the session token is minted here and reaches disk only
/// through the publisher. A session whose binding cannot be published is closed again.
///
/// # Errors
///
/// Refuses an unusable declared listener before any session is opened, a missing capability, a
/// session the registry or store refuses, and any binding publication failure; `OrphanedSession` names a session that stayed open because closing it
/// after a failed publication also failed.
pub fn enrol(
    store: &mut Store,
    sessions: &mut SessionRegistry,
    identity: &IdentityRecord,
    request: EnrolmentRequest,
    publisher: &BindingPublisher,
) -> Result<PublishedBinding, EnrolmentError> {
    open_and_publish(store, sessions, identity, request, publisher, None)
}

/// Opens one capability-grant session exactly as `enrol` does and advertises the verified
/// native grant's not-after instant, in whole unix seconds rounded down, as the session's public
/// time expiry. The sequence expiry stays the request's own bound.
///
/// # Errors
///
/// Returns `Session(IdentityMismatch)` when the verified bound belongs to another tenant, agent
/// or grant, `Session(MissingField("expiry_seconds"))` for a bound below one second, and every
/// refusal `enrol` returns.
pub fn enrol_with_verified_expiry(
    store: &mut Store,
    sessions: &mut SessionRegistry,
    identity: &IdentityRecord,
    enrolment: VerifiedExpiryEnrolment,
    publisher: &BindingPublisher,
) -> Result<PublishedBinding, EnrolmentError> {
    let expiry_seconds = verified_floor_seconds(identity.tenant(), identity.did(), &enrolment)?;
    open_and_publish(
        store,
        sessions,
        identity,
        enrolment.request,
        publisher,
        Some(expiry_seconds),
    )
}

/// Advertises the verified native grant's not-after instant, in whole unix seconds rounded down,
/// on the already enrolled capability-grant session of `enrolment`'s request when that session
/// was enrolled without a public time expiry. The binding document carries no time expiry, so
/// only the durable session record changes.
///
/// # Errors
///
/// Returns `Session(IdentityMismatch)` when the verified bound belongs to another tenant, agent
/// or grant, when the enrolled session belongs to another agent or grant, or when it already
/// advertises a public expiry, `Session(MissingField("expiry_seconds"))` for a bound below one
/// second, `Session(NotFound)` for a session that was never enrolled, and every refusal
/// `session::amend_public_expiry` returns.
pub fn republish_with_verified_expiry(
    store: &mut Store,
    sessions: &mut SessionRegistry,
    tenant: &TenantId,
    agent: &layerx_types::ids::Did,
    enrolment: VerifiedExpiryEnrolment,
) -> Result<(), EnrolmentError> {
    let expiry_seconds = verified_floor_seconds(tenant, agent, &enrolment)?;
    let request = enrolment.request;
    let enrolled = sessions
        .get(tenant, request.session_id)
        .ok_or(EnrolmentError::Session(SessionError::NotFound))?;
    if &enrolled.request.agent != agent
        || enrolled.request.authority != ProtocolAuthority::CapabilityGrant(request.capability_id.0)
    {
        return Err(EnrolmentError::Session(SessionError::IdentityMismatch));
    }
    session::amend_public_expiry(store, sessions, tenant, request.session_id, expiry_seconds)
        .map_err(EnrolmentError::Session)
}

fn verified_floor_seconds(
    tenant: &TenantId,
    agent: &layerx_types::ids::Did,
    enrolment: &VerifiedExpiryEnrolment,
) -> Result<u64, EnrolmentError> {
    let expiry = &enrolment.expiry;
    if &expiry.tenant != tenant
        || &expiry.agent != agent
        || expiry.capability_id != enrolment.request.capability_id
    {
        return Err(EnrolmentError::Session(SessionError::IdentityMismatch));
    }
    crate::human_runtime::owner_expiry_seconds(expiry.not_after_ms)
        .map_err(|_| EnrolmentError::Session(SessionError::MissingField("expiry_seconds")))
}

fn open_and_publish(
    store: &mut Store,
    sessions: &mut SessionRegistry,
    identity: &IdentityRecord,
    request: EnrolmentRequest,
    publisher: &BindingPublisher,
    expiry_seconds: Option<u64>,
) -> Result<PublishedBinding, EnrolmentError> {
    publisher.check_listener()?;
    let tenant = identity.tenant().clone();
    Capability::restore(store, tenant.clone(), request.capability_id)
        .map_err(EnrolmentError::Capability)?
        .ok_or(EnrolmentError::MissingCapability)?;
    let token = fresh_token()?;
    let open = OpenRequest {
        session_id: request.session_id,
        token_id: *token,
        tenant: tenant.clone(),
        agent: identity.did().clone(),
        authority: ProtocolAuthority::CapabilityGrant(request.capability_id.0),
        permitted_activity_types: request.permitted_activity_types,
        scopes: request.scopes,
        expiry_sequence: request.expiry_sequence,
        expiry_seconds,
        opening_client: request.opening_client,
        policy_version: request.policy_version,
    };
    let issued = session::open(store, sessions, identity, open, request.core_sequence)
        .map_err(EnrolmentError::Session)?;
    let enrolment = Enrolment {
        tenant: tenant.clone(),
        session_id: request.session_id,
        session_generation: issued.generation(),
        capability_id: request.capability_id,
        core_height: request.core_sequence,
        token: Zeroizing::new(issued.token_id()),
    };
    drop(issued);
    match publisher.publish(&enrolment) {
        Ok(binding) => Ok(binding),
        Err(error) => match session::close(store, sessions, &tenant, request.session_id) {
            Ok(()) => Err(error),
            Err(_) => Err(EnrolmentError::OrphanedSession(
                request.session_id,
                Box::new(error),
            )),
        },
    }
}

/// Returns the session a published binding document names, when one is already published.
///
/// A daemon that restarts after a successful enrolment reads the session back from its store
/// rather than opening a second one; this reports which session the published document belongs
/// to so that boot can refuse a document that names a different one.
///
/// # Errors
///
/// Returns `AlreadyPublished` for a path that is not a regular file, a document larger than the
/// served path's bound, or one that is not JSON; `Io` for a read failure; and `Encoding` when the
/// document carries no `session_id` of 32 hexadecimal bytes.
pub fn published_session(path: &Path) -> Result<Option<SessionId>, EnrolmentError> {
    let Some(document) = read_document(path)? else {
        return Ok(None);
    };
    let encoded =
        document
            .get("session_id")
            .and_then(Value::as_str)
            .ok_or(EnrolmentError::Encoding(
                "the published binding names no session",
            ))?;
    let session = hex::decode_digest(encoded)
        .map_err(|_| EnrolmentError::Encoding("the published binding names an invalid session"))?;
    Ok(Some(SessionId(session)))
}

fn fresh_token() -> Result<Zeroizing<[u8; 32]>, EnrolmentError> {
    let mut token = Zeroizing::new([0_u8; 32]);
    for _ in 0..TOKEN_ATTEMPTS {
        getrandom::fill(token.as_mut_slice())
            .map_err(|_| EnrolmentError::Encoding("the platform entropy source is unavailable"))?;
        if *token != [0; 32] {
            return Ok(token);
        }
    }
    Err(EnrolmentError::Encoding(
        "the platform entropy source returned only zero tokens",
    ))
}

fn path_text(path: &Path) -> Result<&str, EnrolmentError> {
    if !path.is_absolute() {
        return Err(EnrolmentError::InvalidPath("the path is not absolute"));
    }
    let text = path
        .to_str()
        .ok_or(EnrolmentError::InvalidPath("the path is not UTF-8"))?;
    if text.is_empty() || text.len() > MAX_TEXT_BYTES {
        return Err(EnrolmentError::InvalidPath(
            "the path is longer than the binding document carries",
        ));
    }
    Ok(text)
}

fn prepare_root(root: &Path) -> Result<(), EnrolmentError> {
    match fs::symlink_metadata(root) {
        Ok(metadata) => {
            if metadata.file_type().is_symlink() || !metadata.is_dir() {
                return Err(EnrolmentError::InvalidPath(
                    "the binding directory is not a directory",
                ));
            }
            if metadata.uid() != fs::metadata("/proc/self")?.uid()
                || metadata.mode() & 0o777 != 0o700
            {
                return Err(EnrolmentError::InvalidPath(
                    "the binding directory is not owner-only",
                ));
            }
        }
        Err(error) if error.kind() == io::ErrorKind::NotFound => {
            DirBuilder::new().recursive(true).mode(0o700).create(root)?;
            fs::set_permissions(root, fs::Permissions::from_mode(0o700))?;
        }
        Err(error) => return Err(EnrolmentError::Io(error)),
    }
    if fs::canonicalize(root)? != root {
        return Err(EnrolmentError::InvalidPath(
            "the binding directory path is not canonical",
        ));
    }
    Ok(())
}

fn read_document(path: &Path) -> Result<Option<Value>, EnrolmentError> {
    let metadata = match fs::symlink_metadata(path) {
        Ok(metadata) => metadata,
        Err(error) if error.kind() == io::ErrorKind::NotFound => return Ok(None),
        Err(error) => return Err(EnrolmentError::Io(error)),
    };
    if !metadata.is_file() {
        return Err(EnrolmentError::AlreadyPublished(path.to_path_buf()));
    }
    if usize::try_from(metadata.len()).unwrap_or(usize::MAX) > MAX_DOCUMENT_BYTES {
        return Err(EnrolmentError::AlreadyPublished(path.to_path_buf()));
    }
    let bytes = crate::config::read_protected_source(path, MAX_DOCUMENT_BYTES)
        .map_err(|_| EnrolmentError::InvalidPath("the binding document is not protected"))?;
    let document: Value = serde_json::from_slice(&bytes)
        .map_err(|_| EnrolmentError::AlreadyPublished(path.to_path_buf()))?;
    serde_json::from_value::<BindingDocument>(document.clone())
        .map_err(|_| EnrolmentError::AlreadyPublished(path.to_path_buf()))?;
    Ok(Some(document))
}

fn create_private(path: &Path, contents: &[u8]) -> Result<(), EnrolmentError> {
    let parent = path
        .parent()
        .ok_or(EnrolmentError::InvalidPath("the file has no directory"))?;
    let nonce = fresh_token()?;
    let temporary = parent.join(format!(".pending-{}", hex::encode(nonce.as_slice())));
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(&temporary)?;
    let mut linked = false;
    let result = (|| {
        file.write_all(contents)?;
        file.write_all(b"\n")?;
        file.sync_all()?;
        fs::hard_link(&temporary, path).map_err(|error| {
            if error.kind() == io::ErrorKind::AlreadyExists {
                EnrolmentError::AlreadyPublished(path.to_path_buf())
            } else {
                EnrolmentError::Io(error)
            }
        })?;
        linked = true;
        fs::remove_file(&temporary)?;
        Ok(())
    })();
    if result.is_err() {
        let _ = fs::remove_file(&temporary);
        if linked {
            let _ = fs::remove_file(path);
        }
    }
    result
}

fn sync_directory(root: &Path) -> Result<(), EnrolmentError> {
    fs::File::open(root)?.sync_all()?;
    Ok(())
}

#[cfg(test)]
mod verified_expiry_tests {
    use std::collections::BTreeSet;
    use std::path::{Path, PathBuf};
    use std::time::Duration;

    use layerx_types::ids::Did;
    use layerx_types::verify::VerificationLevel;

    use super::{
        enrol, enrol_with_verified_expiry, BindingMode, BindingPublisher, DaemonSurface,
        EnrolmentError, EnrolmentRequest, VerifiedExpiryEnrolment, VerifiedGrantExpiry,
    };
    use crate::budget::{LimitConfig, LimitId, LimitScope};
    use crate::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
    use crate::identity::{
        self, CoreIdentity, IdentityError, IdentityRecord, IdentityResolver, ProtocolAuthority,
    };
    use crate::session::{SessionError, SessionId, SessionRegistry};
    use crate::store::{Store, TenantId};

    const CAPABILITY: CapabilityId = CapabilityId([9; 32]);
    const SESSION: SessionId = SessionId([0x0c; 32]);

    struct CoreBoundary(CoreIdentity);

    impl IdentityResolver for CoreBoundary {
        fn resolve(&mut self, _did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
            Ok(Some(self.0.clone()))
        }
    }

    fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
        value.unwrap_or_else(|error| panic!("verified expiry enrolment: {error:?}"))
    }

    fn tenant() -> TenantId {
        must(TenantId::new("tenant-a"))
    }

    fn agent() -> Did {
        must(Did::new(b"did:layerx:model"))
    }

    fn directory(name: &str) -> PathBuf {
        let root =
            std::env::temp_dir().join(format!("lxp-verified-expiry-{name}-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&root);
        root
    }

    fn records(root: &Path) -> (Store, IdentityRecord, BindingPublisher) {
        let mut store = must(Store::open(root.join("store")));
        must(
            must(Capability::new(
                CAPABILITY,
                tenant(),
                CapabilityDimensions {
                    activity_types: BTreeSet::from([7]),
                    counterparties: BTreeSet::from([[2; 32]]),
                    assets: BTreeSet::from([[3; 32]]),
                    amount_ceiling: 100,
                    rate_ceiling: RateCeiling {
                        maximum_uses: 2,
                        window_sequences: 10,
                    },
                    purposes: BTreeSet::from(["service-payment".to_owned()]),
                    expiry_sequence: 400,
                },
            ))
            .persist(&mut store),
        );
        let mut boundary = CoreBoundary(CoreIdentity {
            canonical_bytes: b"model-identity".to_vec(),
            head_sequence: 10,
            revocation_sequence: 1,
            verification_level: VerificationLevel::STATE_PROVEN,
            frozen: false,
            authorities: vec![ProtocolAuthority::CapabilityGrant(CAPABILITY.0)],
        });
        let identity = must(identity::register(
            &mut store,
            tenant(),
            agent(),
            &mut boundary,
        ));
        let publisher = must(BindingPublisher::new(
            root.join("mcp"),
            root.join("store"),
            root.join("audit"),
            must(DaemonSurface::new(
                "127.0.0.1:9",
                "d".repeat(48),
                [0xcc; 32],
            )),
            LimitConfig {
                id: LimitId([0x0a; 16]),
                name: "mcp".to_owned(),
                scope: LimitScope::Tenant([1; 32]),
                ceiling: 1_000,
                consumed: 0,
            },
            Duration::from_millis(5_000),
            BindingMode::Full,
        ));
        (store, identity, publisher)
    }

    fn request() -> EnrolmentRequest {
        EnrolmentRequest {
            session_id: SESSION,
            capability_id: CAPABILITY,
            permitted_activity_types: BTreeSet::from([7]),
            scopes: BTreeSet::from(["balance.read".to_owned()]),
            expiry_sequence: 300,
            opening_client: "mcp".to_owned(),
            policy_version: "policy-v1".to_owned(),
            core_sequence: 50,
        }
    }

    fn expiry(not_after_ms: u64) -> VerifiedGrantExpiry {
        VerifiedGrantExpiry::verified(tenant(), agent(), CAPABILITY, not_after_ms)
    }

    #[test]
    fn wrapper_preserves_the_inner_request() {
        let wrapped = VerifiedExpiryEnrolment::new(request(), expiry(10_000_000));
        assert_eq!(wrapped.request(), &request());
        assert_eq!(wrapped.expiry(), &expiry(10_000_000));
        assert_eq!(wrapped.expiry().not_after_ms(), 10_000_000);
    }

    #[test]
    fn carrier_for_another_tenant_agent_or_grant_is_refused() {
        let root = directory("mismatch");
        let (mut store, identity, publisher) = records(&root);
        let mut sessions = SessionRegistry::default();
        for carrier in [
            VerifiedGrantExpiry::verified(
                must(TenantId::new("tenant-b")),
                agent(),
                CAPABILITY,
                10_000_000,
            ),
            VerifiedGrantExpiry::verified(
                tenant(),
                must(Did::new(b"did:layerx:other")),
                CAPABILITY,
                10_000_000,
            ),
            VerifiedGrantExpiry::verified(tenant(), agent(), CapabilityId([8; 32]), 10_000_000),
        ] {
            let refused = enrol_with_verified_expiry(
                &mut store,
                &mut sessions,
                &identity,
                VerifiedExpiryEnrolment::new(request(), carrier),
                &publisher,
            );
            assert!(matches!(
                refused,
                Err(EnrolmentError::Session(SessionError::IdentityMismatch))
            ));
        }
        assert!(sessions.get(&tenant(), SESSION).is_none());
        assert!(!publisher.binding_path().exists());
        drop(store);
        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    fn matching_carrier_advertises_floor_seconds_and_keeps_the_sequence_bound() {
        let root = directory("match");
        let (mut store, identity, publisher) = records(&root);
        let mut sessions = SessionRegistry::default();
        let published = must(enrol_with_verified_expiry(
            &mut store,
            &mut sessions,
            &identity,
            VerifiedExpiryEnrolment::new(request(), expiry(10_000_999)),
            &publisher,
        ));
        assert_eq!(published.session_id, SESSION);
        let record = sessions
            .get(&tenant(), SESSION)
            .unwrap_or_else(|| panic!("enrolled session"));
        assert_eq!(record.request.expiry_seconds, Some(10_000));
        assert_eq!(record.request.expiry_sequence, 300);
        drop(store);
        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    fn carrier_below_one_second_is_refused() {
        let root = directory("sub-second");
        let (mut store, identity, publisher) = records(&root);
        let mut sessions = SessionRegistry::default();
        let refused = enrol_with_verified_expiry(
            &mut store,
            &mut sessions,
            &identity,
            VerifiedExpiryEnrolment::new(request(), expiry(999)),
            &publisher,
        );
        assert!(matches!(
            refused,
            Err(EnrolmentError::Session(SessionError::MissingField(
                "expiry_seconds"
            )))
        ));
        assert!(sessions.get(&tenant(), SESSION).is_none());
        drop(store);
        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    fn legacy_enrol_keeps_no_public_expiry() {
        let root = directory("legacy");
        let (mut store, identity, publisher) = records(&root);
        let mut sessions = SessionRegistry::default();
        must(enrol(
            &mut store,
            &mut sessions,
            &identity,
            request(),
            &publisher,
        ));
        let record = sessions
            .get(&tenant(), SESSION)
            .unwrap_or_else(|| panic!("enrolled session"));
        assert_eq!(record.request.expiry_seconds, None);
        assert_eq!(record.request.expiry_sequence, 300);
        drop(store);
        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    fn later_verified_install_republishes_absent_expiry_once() {
        let root = directory("republish");
        let (mut store, identity, publisher) = records(&root);
        let mut sessions = SessionRegistry::default();
        must(enrol(
            &mut store,
            &mut sessions,
            &identity,
            request(),
            &publisher,
        ));
        must(super::republish_with_verified_expiry(
            &mut store,
            &mut sessions,
            &tenant(),
            &agent(),
            VerifiedExpiryEnrolment::new(request(), expiry(10_000_999)),
        ));
        let record = sessions
            .get(&tenant(), SESSION)
            .unwrap_or_else(|| panic!("republished session"));
        assert_eq!(record.request.expiry_seconds, Some(10_000));
        assert_eq!(record.request.expiry_sequence, 300);
        let mut restored = SessionRegistry::default();
        must(restored.restore_tenant(&store, &tenant()));
        assert_eq!(
            restored
                .get(&tenant(), SESSION)
                .map(|record| record.request.expiry_seconds),
            Some(Some(10_000))
        );
        let second = super::republish_with_verified_expiry(
            &mut store,
            &mut sessions,
            &tenant(),
            &agent(),
            VerifiedExpiryEnrolment::new(request(), expiry(20_000_000)),
        );
        assert!(matches!(
            second,
            Err(EnrolmentError::Session(SessionError::IdentityMismatch))
        ));
        drop(store);
        let _ = std::fs::remove_dir_all(root);
    }

    #[test]
    fn republish_refuses_mismatched_sub_second_or_unenrolled_carriers() {
        let root = directory("republish-refused");
        let (mut store, identity, publisher) = records(&root);
        let mut sessions = SessionRegistry::default();
        let unenrolled = super::republish_with_verified_expiry(
            &mut store,
            &mut sessions,
            &tenant(),
            &agent(),
            VerifiedExpiryEnrolment::new(request(), expiry(10_000_000)),
        );
        assert!(matches!(
            unenrolled,
            Err(EnrolmentError::Session(SessionError::NotFound))
        ));
        must(enrol(
            &mut store,
            &mut sessions,
            &identity,
            request(),
            &publisher,
        ));
        let other_grant = super::republish_with_verified_expiry(
            &mut store,
            &mut sessions,
            &tenant(),
            &agent(),
            VerifiedExpiryEnrolment::new(
                request(),
                VerifiedGrantExpiry::verified(tenant(), agent(), CapabilityId([8; 32]), 10_000_000),
            ),
        );
        assert!(matches!(
            other_grant,
            Err(EnrolmentError::Session(SessionError::IdentityMismatch))
        ));
        let sub_second = super::republish_with_verified_expiry(
            &mut store,
            &mut sessions,
            &tenant(),
            &agent(),
            VerifiedExpiryEnrolment::new(request(), expiry(999)),
        );
        assert!(matches!(
            sub_second,
            Err(EnrolmentError::Session(SessionError::MissingField(
                "expiry_seconds"
            )))
        ));
        assert_eq!(
            sessions
                .get(&tenant(), SESSION)
                .map(|record| record.request.expiry_seconds),
            Some(None)
        );
        drop(store);
        let _ = std::fs::remove_dir_all(root);
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedProgramBudgetDenominations {
    tenant: crate::store::TenantId,
    bindings: Vec<([u8; 32], crate::budget::ProgramLimitDenomination)>,
    source_digest: [u8; 32],
}

impl VerifiedProgramBudgetDenominations {
    pub(crate) fn from_protected_source(
        tenant: crate::store::TenantId,
        mut bindings: Vec<([u8; 32], crate::budget::ProgramLimitDenomination)>,
        source_digest: [u8; 32],
    ) -> Result<Self, EnrolmentError> {
        bindings.sort_unstable_by_key(|(id, _)| *id);
        if bindings.is_empty()
            || bindings.len() > 1024
            || source_digest == [0; 32]
            || bindings
                .iter()
                .any(|(id, denomination)| *id == [0; 32] || !denomination.valid())
            || bindings.windows(2).any(|pair| pair[0].0 == pair[1].0)
        {
            return Err(EnrolmentError::InvalidLimit(
                "invalid explicit Program denomination",
            ));
        }
        Ok(Self {
            tenant,
            bindings,
            source_digest,
        })
    }

    #[must_use]
    pub fn tenant(&self) -> &crate::store::TenantId {
        &self.tenant
    }

    #[must_use]
    pub fn bindings(&self) -> &[([u8; 32], crate::budget::ProgramLimitDenomination)] {
        &self.bindings
    }

    #[must_use]
    pub fn source_digest(&self) -> [u8; 32] {
        self.source_digest
    }
}


#[cfg(test)]
mod web_payment_binding_tests {
    use super::WebDeclaration;
    use layerx_client::sequencer::ReceiptKeyEntry;
    use layerx_programs::hex;
    use serde_json::json;

    #[test]
    fn web_payment_binding_preserves_the_selected_key_and_complete_governed_index() {
        let keys = vec![
            ReceiptKeyEntry {
                key_id: [1; 32],
                public_key: [2; 32],
                first_height: 10,
                last_height: 20,
            },
            ReceiptKeyEntry {
                key_id: [3; 32],
                public_key: [4; 32],
                first_height: 30,
                last_height: 40,
            },
        ];
        let declaration = WebDeclaration::new(
            "http://127.0.0.1:4000".to_owned(),
            "eip155:125".to_owned(),
            [5; 20],
            [6; 20],
            keys.clone(),
            1,
            5000,
            3,
            100,
        )
        .expect("valid governed web declaration");
        let published = declaration.document();
        assert_eq!(published["network"], "eip155:125");
        assert_eq!(
            published["payer_address"],
            format!("0x{}", hex::encode(&[5; 20]))
        );
        assert_eq!(
            published["receiver_address"],
            format!("0x{}", hex::encode(&[6; 20]))
        );
        assert_eq!(published["sequencer_id"], hex::encode(&keys[1].key_id));
        assert_eq!(published["first_batch_number"], 30);
        assert_eq!(published["last_batch_number"], 40);
        assert_eq!(
            published["receipt_key_index"],
            json!(keys
                .iter()
                .map(|key| json!({
                    "keyId": hex::encode(&key.key_id), "publicKey": hex::encode(&key.public_key),
                    "firstHeight": key.first_height, "lastHeight": key.last_height,
                }))
                .collect::<Vec<_>>())
        );
        assert!(published.get("sequencer_public_key").is_none());
    }
}
