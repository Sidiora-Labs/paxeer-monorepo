//! Managed-agent protocol sessions and primary-key rotation orchestration.

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::fmt::{Debug, Display, Formatter};

#[path = "session_state.rs"]
mod durable;

use layerx_crypto::local::LocalSigner;
use layerx_crypto::session::{
    issue_session_key, IssuedSessionKey, SessionIssueError, SessionKeyRequest,
};
use layerx_crypto::signer::Signer as _;
use layerx_intents::{
    compile, CompileError, DisclosureCheck, DisclosureCheckError, Intent, IntentError, IntentKind,
    KeyRotation,
};
use layerx_types::activity::TimestampBound;
use layerx_types::ids::Did;
use layerx_types::intent::{PublicKey, Sequence};
use layerx_types::payload::{ActivityType, ModuleRegistry};
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};
use zeroize::Zeroizing;

use crate::store::PrincipalId;

use super::{CustodyError, KeyClass, KeyId, Keystore};

const SESSION_RECEIPT_DOMAIN: &[u8] = b"layerx-human/agent-session-receipt/v1";
const SUSPENSION_RECEIPT_DOMAIN: &[u8] = b"layerx-human/agent-session-suspension/v1";
const REVOCATION_RECEIPT_DOMAIN: &[u8] = b"layerx-human/agent-session-revocation/v1";
const ROTATION_RECEIPT_DOMAIN: &[u8] = b"layerx-human/key-rotation-receipt/v1";

/// Declared policy for every managed-agent operating session.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct SessionPolicy {
    lifetime_seconds: u64,
    renewal_lead_seconds: u64,
    maximum_revocation_latency_seconds: u64,
    daemon_scopes: Vec<String>,
    policy_version: String,
}

impl SessionPolicy {
    /// Constructs a complete session policy without implicit permissions.
    ///
    /// # Errors
    ///
    /// Refuses zero or inverted timing, empty daemon scopes, duplicate scopes,
    /// and an unnamed daemon policy.
    pub fn new(
        lifetime_seconds: u64,
        renewal_lead_seconds: u64,
        maximum_revocation_latency_seconds: u64,
        mut daemon_scopes: Vec<String>,
        policy_version: impl Into<String>,
    ) -> Result<Self, SessionKeyError> {
        let policy_version = policy_version.into();
        if lifetime_seconds == 0
            || renewal_lead_seconds == 0
            || renewal_lead_seconds >= lifetime_seconds
            || maximum_revocation_latency_seconds == 0
            || daemon_scopes.is_empty()
            || daemon_scopes.iter().any(String::is_empty)
            || policy_version.is_empty()
        {
            return Err(SessionKeyError::InvalidPolicy);
        }
        daemon_scopes.sort();
        if daemon_scopes.windows(2).any(|pair| pair[0] == pair[1]) {
            return Err(SessionKeyError::InvalidPolicy);
        }
        Ok(Self {
            lifetime_seconds,
            renewal_lead_seconds,
            maximum_revocation_latency_seconds,
            daemon_scopes,
            policy_version,
        })
    }

    /// Returns the exact protocol validity duration.
    #[must_use]
    pub const fn lifetime_seconds(&self) -> u64 {
        self.lifetime_seconds
    }

    /// Returns how far ahead the unattended renewal scheduler acts.
    #[must_use]
    pub const fn renewal_lead_seconds(&self) -> u64 {
        self.renewal_lead_seconds
    }

    /// Returns the declared end-to-end pause/archive revocation target.
    #[must_use]
    pub const fn maximum_revocation_latency_seconds(&self) -> u64 {
        self.maximum_revocation_latency_seconds
    }
}

/// Current verified protocol identity facts required to issue authority.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProtocolIdentitySnapshot {
    pub protocol_identity: [u8; 32],
    pub primary_public_key: [u8; 32],
    pub revocation_sequence: u64,
    pub protocol_time: u64,
    pub core_sequence: u64,
    pub rung: Rung,
}

/// A session seed that can only leave the Human service by being consumed by
/// the agent contract. It is never clonable or printable and zeroizes on drop.
pub struct AgentSessionSecret(Zeroizing<[u8; 32]>);

impl AgentSessionSecret {
    /// Consumes the transfer object at the trusted agent boundary.
    #[must_use]
    pub fn into_seed(self) -> [u8; 32] {
        *self.0
    }
}

impl Debug for AgentSessionSecret {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str("AgentSessionSecret([redacted])")
    }
}

/// Independently generated entropy for one operating session. The value is
/// injected from the configured entropy boundary and is zeroized if
/// provisioning is refused before the agent consumes it.
pub struct SessionKeyEntropy(Zeroizing<[u8; 32]>);

impl SessionKeyEntropy {
    /// Wraps one non-zero 256-bit seed.
    ///
    /// # Errors
    ///
    /// Refuses the reserved all-zero value.
    pub fn new(seed: [u8; 32]) -> Result<Self, SessionKeyError> {
        let seed = Zeroizing::new(seed);
        if seed.iter().all(|byte| *byte == 0) {
            Err(SessionKeyError::EntropyUnavailable)
        } else {
            Ok(Self(seed))
        }
    }
}

impl Debug for SessionKeyEntropy {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str("SessionKeyEntropy([redacted])")
    }
}

/// Configured entropy boundary used by initial provisioning and unattended
/// renewal. The Human orchestration layer never reaches ambient randomness.
pub trait SessionEntropySource {
    /// Returns fresh independently generated entropy for exactly one session.
    ///
    /// # Errors
    ///
    /// Returns an entropy-boundary refusal without providing partial material.
    fn next_session_entropy(&mut self) -> Result<SessionKeyEntropy, SessionKeyError>;
}

/// Exact protocol and daemon material sent to the agent contract. No primary
/// key or primary-key handle is representable here.
pub struct AgentSessionProvision {
    pub principal: PrincipalId,
    pub agent_did: Did,
    pub issued: IssuedSessionKey,
    pub daemon_scopes: Vec<String>,
    pub daemon_policy_version: String,
    secret: AgentSessionSecret,
}

impl AgentSessionProvision {
    /// Consumes the request into its public metadata and session-only secret.
    #[must_use]
    pub fn into_parts(
        self,
    ) -> (
        PrincipalId,
        Did,
        IssuedSessionKey,
        Vec<String>,
        String,
        AgentSessionSecret,
    ) {
        (
            self.principal,
            self.agent_did,
            self.issued,
            self.daemon_scopes,
            self.daemon_policy_version,
            self.secret,
        )
    }
}

impl Debug for AgentSessionProvision {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("AgentSessionProvision")
            .field("principal", &self.principal)
            .field("agent_did", &self.agent_did)
            .field("grant_id", &self.issued.grant_id)
            .field("session_public_key", &self.issued.session_public_key)
            .field("expires_at", &self.issued.expires_at)
            .field("daemon_scopes", &self.daemon_scopes)
            .field("daemon_policy_version", &self.daemon_policy_version)
            .field("secret", &"[redacted]")
            .finish()
    }
}

/// Proof that the protocol grant and daemon permission session were installed
/// as one joined operation.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProvisionEvidence {
    pub grant_id: [u8; 32],
    pub session_public_key: [u8; 32],
    pub daemon_session_id: [u8; 32],
    pub protocol_sequence: u64,
    pub observed_at: u64,
    pub rung: Rung,
    pub receipt_digest: [u8; 32],
}

impl ProvisionEvidence {
    /// Computes the receipt digest the service independently verifies.
    #[must_use]
    pub fn expected_digest(&self) -> [u8; 32] {
        digest_fields(
            SESSION_RECEIPT_DOMAIN,
            &[
                &self.grant_id,
                &self.session_public_key,
                &self.daemon_session_id,
                &self.protocol_sequence.to_be_bytes(),
                &self.observed_at.to_be_bytes(),
                &[self.rung.wire_rank()],
            ],
        )
    }
}

/// Exact target for daemon suspension and protocol revocation.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SessionTarget {
    pub principal: PrincipalId,
    pub agent_did: Did,
    pub grant_id: [u8; 32],
    pub daemon_session_id: [u8; 32],
}

/// Why operating authority was removed.
#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub enum RevocationReason {
    Paused,
    Archived,
    Renewed,
    PrimaryKeyRotated,
}

impl RevocationReason {
    const fn code(self) -> u8 {
        match self {
            Self::Paused => 1,
            Self::Archived => 2,
            Self::Renewed => 3,
            Self::PrimaryKeyRotated => 4,
        }
    }
}

/// Daemon-side permission suspension evidence.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SuspensionEvidence {
    pub grant_id: [u8; 32],
    pub daemon_session_id: [u8; 32],
    pub reason: RevocationReason,
    pub observed_at: u64,
    pub receipt_digest: [u8; 32],
}

impl SuspensionEvidence {
    #[must_use]
    pub fn expected_digest(&self) -> [u8; 32] {
        digest_fields(
            SUSPENSION_RECEIPT_DOMAIN,
            &[
                &self.grant_id,
                &self.daemon_session_id,
                &[self.reason.code()],
                &self.observed_at.to_be_bytes(),
            ],
        )
    }
}

/// Protocol-side authority revocation evidence.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RevocationEvidence {
    pub grant_id: [u8; 32],
    pub reason: RevocationReason,
    pub observed_sequence: u64,
    pub observed_at: u64,
    pub rung: Rung,
    pub receipt_digest: [u8; 32],
}

impl RevocationEvidence {
    #[must_use]
    pub fn expected_digest(&self) -> [u8; 32] {
        digest_fields(
            REVOCATION_RECEIPT_DOMAIN,
            &[
                &self.grant_id,
                &[self.reason.code()],
                &self.observed_sequence.to_be_bytes(),
                &self.observed_at.to_be_bytes(),
                &[self.rung.wire_rank()],
            ],
        )
    }
}

/// Protocol key-rotation submission using only the Human intent authority.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RotationSubmission {
    pub principal: PrincipalId,
    pub subject: RotationSubject,
    pub did: Did,
    pub current_public_key: [u8; 32],
    pub pending_public_key: [u8; 32],
    pub effective_at: u64,
    pub lapse_at: u64,
    pub effective_sequence: u64,
    pub intent: Intent,
    pub compiled: layerx_intents::CompiledIntent,
    pub disclosure: DisclosureCheck,
}

/// Proof-bearing receipt for the key-rotation announcement.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RotationEvidence {
    pub tx_hash: [u8; 32],
    pub payload_hash: [u8; 32],
    pub pending_public_key: [u8; 32],
    pub effective_at: u64,
    pub lapse_at: u64,
    pub effective_sequence: u64,
    pub observed_sequence: u64,
    pub observed_at: u64,
    pub rung: Rung,
    pub receipt_digest: [u8; 32],
}

impl RotationEvidence {
    #[must_use]
    pub fn expected_digest(&self) -> [u8; 32] {
        digest_fields(
            ROTATION_RECEIPT_DOMAIN,
            &[
                &self.tx_hash,
                &self.payload_hash,
                &self.pending_public_key,
                &self.effective_at.to_be_bytes(),
                &self.lapse_at.to_be_bytes(),
                &self.effective_sequence.to_be_bytes(),
                &self.observed_sequence.to_be_bytes(),
                &self.observed_at.to_be_bytes(),
                &[self.rung.wire_rank()],
            ],
        )
    }
}

/// State-proven identity rotation observation returned by the agent contract.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RotationObservation {
    pub tx_hash: [u8; 32],
    pub primary_public_key: [u8; 32],
    pub pending_public_key: Option<[u8; 32]>,
    pub superseded_public_key: Option<[u8; 32]>,
    pub effective_at: u64,
    pub lapse_at: u64,
    pub effective_sequence: u64,
    pub observed_at: u64,
    pub observed_sequence: u64,
    pub rung: Rung,
}

/// Stable errors at the versioned agent boundary.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum AgentContractError {
    Unavailable,
    Refused(&'static str),
}

impl Display for AgentContractError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Unavailable => formatter.write_str("agent contract is unavailable"),
            Self::Refused(reason) => write!(formatter, "agent contract refused: {reason}"),
        }
    }
}

impl std::error::Error for AgentContractError {}

/// Versioned Human-to-agent contract. It deliberately exposes session-only
/// material and public rotation intents, never a primary private key.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SessionAuthorityObservation {
    pub provision: ProvisionEvidence,
    pub daemon_suspended: bool,
    pub revocation: Option<RevocationEvidence>,
}

pub trait AgentSessionContract {
    /// Reads the exact daemon permission state and chain authority on reopen.
    /// # Errors
    /// Returns an error if validation or a required operation fails.
    fn session_authority(
        &mut self,
        _target: &SessionTarget,
    ) -> Result<SessionAuthorityObservation, AgentContractError> {
        Err(AgentContractError::Unavailable)
    }

    /// Proves whether this exact previously attempted grant was installed.
    /// Absence must be authoritative before replaying the identical installation.
    /// # Errors
    /// Returns an error if validation or a required operation fails.
    fn recover_provision(
        &mut self,
        _principal: &PrincipalId,
        _did: &Did,
        _grant_id: [u8; 32],
    ) -> Result<Option<ProvisionEvidence>, AgentContractError> {
        Err(AgentContractError::Unavailable)
    }

    /// Recovers the receipt for an attempted announcement by its canonical payload.
    /// # Errors
    /// Returns an error if validation or a required operation fails.
    fn recover_rotation(
        &mut self,
        _principal: &PrincipalId,
        _did: &Did,
        _payload_hash: [u8; 32],
    ) -> Result<Option<RotationEvidence>, AgentContractError> {
        Err(AgentContractError::Unavailable)
    }

    /// Reads state-proven identity and revocation facts.
    ///
    /// # Errors
    ///
    /// Returns a typed boundary failure or protocol refusal.
    fn identity_snapshot(
        &mut self,
        principal: &PrincipalId,
        did: &Did,
    ) -> Result<ProtocolIdentitySnapshot, AgentContractError>;

    /// Installs one protocol grant and its session-only signer.
    ///
    /// # Errors
    ///
    /// Returns a typed refusal without installing partial authority.
    fn provision_session(
        &mut self,
        request: AgentSessionProvision,
    ) -> Result<ProvisionEvidence, AgentContractError>;

    /// Suspends the exact daemon permission session.
    ///
    /// # Errors
    ///
    /// Returns a typed refusal when suspension cannot be proven.
    fn suspend_permissions(
        &mut self,
        target: &SessionTarget,
        reason: RevocationReason,
        requested_at: u64,
    ) -> Result<SuspensionEvidence, AgentContractError>;

    /// Revokes the exact protocol session authority.
    ///
    /// # Errors
    ///
    /// Returns a typed refusal when revocation cannot be proven.
    fn revoke_protocol_session(
        &mut self,
        target: &SessionTarget,
        reason: RevocationReason,
        requested_at: u64,
    ) -> Result<RevocationEvidence, AgentContractError>;

    /// Submits one byte-verified Human rotation intent.
    ///
    /// # Errors
    ///
    /// Returns a typed protocol or boundary refusal.
    fn announce_rotation(
        &mut self,
        submission: RotationSubmission,
    ) -> Result<RotationEvidence, AgentContractError>;

    /// Reads state-proven rotation state.
    ///
    /// # Errors
    ///
    /// Returns a typed refusal when current state cannot be proven.
    fn rotation_observation(
        &mut self,
        principal: &PrincipalId,
        did: &Did,
    ) -> Result<RotationObservation, AgentContractError>;
}

/// Human or managed-agent primary identity being rotated.
#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub enum RotationSubject {
    Human,
    Agent,
}

impl RotationSubject {
    const fn key_class(self) -> KeyClass {
        match self {
            Self::Human => KeyClass::HumanPrimary,
            Self::Agent => KeyClass::AgentPrimary,
        }
    }
}

/// An exact duration rendered without protocol terminology.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct PlainTime {
    seconds: u64,
    label: String,
}

impl PlainTime {
    fn new(seconds: u64) -> Result<Self, SessionKeyError> {
        if seconds == 0 {
            return Err(SessionKeyError::InvalidRotationWindow);
        }
        let (value, unit) = if seconds.is_multiple_of(86_400) {
            (seconds / 86_400, "day")
        } else if seconds.is_multiple_of(3_600) {
            (seconds / 3_600, "hour")
        } else if seconds.is_multiple_of(60) {
            (seconds / 60, "minute")
        } else {
            (seconds, "second")
        };
        let suffix = if value == 1 { "" } else { "s" };
        Ok(Self {
            seconds,
            label: format!("{value} {unit}{suffix}"),
        })
    }

    #[must_use]
    pub const fn seconds(&self) -> u64 {
        self.seconds
    }
}

impl Display for PlainTime {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(&self.label)
    }
}

/// Public, non-secret record of one live or retired agent session.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct SessionLease {
    #[serde(with = "durable::principal")]
    pub principal: PrincipalId,
    #[serde(with = "durable::did")]
    pub agent_did: Did,
    pub session_public_key: [u8; 32],
    pub grant_id: [u8; 32],
    pub daemon_session_id: [u8; 32],
    #[serde(with = "durable::activities")]
    pub permitted_activity_types: Vec<ActivityType>,
    pub issued_at: u64,
    pub expires_at: u64,
    pub revocation_sequence: u64,
    pub provision_receipt_digest: [u8; 32],
    pub state: SessionLeaseState,
}

/// Honest lifecycle state for one issued session.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub enum SessionLeaseState {
    Active,
    SuspendedAwaitingProtocolRevocation {
        reason: RevocationReason,
        suspended_at: u64,
        suspension_receipt_digest: [u8; 32],
    },
    Revoked {
        reason: RevocationReason,
        suspended_at: u64,
        revoked_at: u64,
        suspension_receipt_digest: [u8; 32],
        revocation_receipt_digest: [u8; 32],
    },
}

/// Current managed-agent lifecycle at the Human boundary.
#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub enum ManagedAgentState {
    Active,
    Paused,
    Archived,
}

/// Completed authority-removal evidence including measured latency.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RevocationOutcome {
    pub reason: RevocationReason,
    pub requested_at: u64,
    pub suspended_at: u64,
    pub revoked_at: u64,
    pub latency_seconds: u64,
    pub within_declared_target: bool,
    pub suspension_receipt_digest: [u8; 32],
    pub revocation_receipt_digest: [u8; 32],
}

/// One automatic replacement, requiring no human ceremony.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RenewalOutcome {
    pub previous_grant_id: [u8; 32],
    pub replacement: SessionLease,
    pub previous_revocation: RevocationOutcome,
}

/// Stored rotation journey with exact protocol windows and public key handles.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct RotationJourney {
    pub tx_hash: [u8; 32],
    #[serde(with = "durable::principal")]
    pub principal: PrincipalId,
    pub subject: RotationSubject,
    #[serde(with = "durable::did")]
    pub did: Did,
    #[serde(with = "durable::key_id")]
    pub current_key_id: KeyId,
    #[serde(with = "durable::key_id")]
    pub pending_key_id: KeyId,
    pub current_public_key: [u8; 32],
    pub pending_public_key: [u8; 32],
    pub announced_at: u64,
    pub effective_at: u64,
    pub lapse_at: u64,
    pub effective_sequence: u64,
    pub challenge_delay: PlainTime,
    pub announcement_receipt_digest: [u8; 32],
    pub state: RotationJourneyState,
}

/// Honest rotation state, including the exact superseded-key sequence window.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub enum RotationJourneyState {
    ChallengeOpen,
    ReadyToCommit,
    Effective {
        superseded_key_usable_before_sequence: u64,
        observed_sequence: u64,
    },
    Lapsed,
}

/// Refusals from provisioning, lifecycle propagation, and rotation.
#[derive(Debug)]
pub enum SessionKeyError {
    InvalidPolicy,
    InvalidIdentity,
    InvalidActivityScope,
    InvalidEvidence,
    InvalidRotationWindow,
    TimeOverflow,
    EntropyUnavailable,
    NotProvisioned,
    AlreadyProvisioned,
    Paused,
    Archived,
    RotationAlreadyOpen,
    RotationNotFound,
    KeyClassMismatch,
    SessionIssue(SessionIssueError),
    Intent(IntentError),
    Compile(CompileError),
    Disclosure(DisclosureCheckError),
    Custody(CustodyError),
    Agent(AgentContractError),
}

impl Display for SessionKeyError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::InvalidPolicy => formatter.write_str("managed-agent session policy is invalid"),
            Self::InvalidIdentity => formatter.write_str("agent identity evidence is invalid"),
            Self::InvalidActivityScope => formatter.write_str("permitted activity set is empty"),
            Self::InvalidEvidence => {
                formatter.write_str("agent evidence did not match the request")
            }
            Self::InvalidRotationWindow => formatter.write_str("key rotation window is invalid"),
            Self::TimeOverflow => formatter.write_str("protocol time window overflowed"),
            Self::EntropyUnavailable => formatter.write_str("session-key entropy is unavailable"),
            Self::NotProvisioned => formatter.write_str("managed agent has no active authority"),
            Self::AlreadyProvisioned => {
                formatter.write_str("managed agent already has active authority")
            }
            Self::Paused => formatter.write_str("managed agent is paused"),
            Self::Archived => formatter.write_str("managed agent is archived"),
            Self::RotationAlreadyOpen => {
                formatter.write_str("identity already has an open rotation")
            }
            Self::RotationNotFound => formatter.write_str("identity has no tracked rotation"),
            Self::KeyClassMismatch => {
                formatter.write_str("custody key does not match the rotation subject")
            }
            Self::SessionIssue(error) => write!(formatter, "{error}"),
            Self::Intent(error) => write!(formatter, "intent refused: {error:?}"),
            Self::Compile(error) => write!(formatter, "intent compilation refused: {error:?}"),
            Self::Disclosure(error) => write!(formatter, "intent disclosure refused: {error:?}"),
            Self::Custody(error) => write!(formatter, "{error}"),
            Self::Agent(error) => write!(formatter, "{error}"),
        }
    }
}

impl std::error::Error for SessionKeyError {}

impl From<SessionIssueError> for SessionKeyError {
    fn from(value: SessionIssueError) -> Self {
        Self::SessionIssue(value)
    }
}

impl From<IntentError> for SessionKeyError {
    fn from(value: IntentError) -> Self {
        Self::Intent(value)
    }
}

impl From<CompileError> for SessionKeyError {
    fn from(value: CompileError) -> Self {
        Self::Compile(value)
    }
}

impl From<DisclosureCheckError> for SessionKeyError {
    fn from(value: DisclosureCheckError) -> Self {
        Self::Disclosure(value)
    }
}

impl From<CustodyError> for SessionKeyError {
    fn from(value: CustodyError) -> Self {
        Self::Custody(value)
    }
}

impl From<AgentContractError> for SessionKeyError {
    fn from(value: AgentContractError) -> Self {
        Self::Agent(value)
    }
}

#[derive(Clone, Serialize, Deserialize)]
struct ManagedAgent {
    state: ManagedAgentState,
    current: SessionLease,
    retired: Vec<SessionLease>,
}

/// Human control-plane orchestrator for bounded operating authority. Private
/// primary keys remain inside [`Keystore`]; only a freshly generated session
/// seed crosses the versioned agent contract.
pub struct SessionKeyProvisioner<C: AgentSessionContract, E: SessionEntropySource> {
    contract: C,
    entropy: E,
    policy: SessionPolicy,
    rotation_registry: ModuleRegistry,
    agents: BTreeMap<(PrincipalId, Did), ManagedAgent>,
    rotations: BTreeMap<(PrincipalId, Did), RotationJourney>,
    rotation_history: Vec<RotationJourney>,
    installations: BTreeMap<(PrincipalId, Did), durable::Installation>,
    renewals: BTreeMap<(PrincipalId, Did), durable::Renewal>,
    stops: BTreeMap<(PrincipalId, Did), durable::Stop>,
    announcements: BTreeMap<(PrincipalId, Did), durable::Announcement>,
    durable: Option<durable::StateFile>,
    poisoned: bool,
}

impl<C: AgentSessionContract, E: SessionEntropySource> SessionKeyProvisioner<C, E> {
    /// Constructs the development-only, non-durable helper. Production must use `open`
    /// and `SessionLifecycleDriver` through component maintenance.
    /// Binds the versioned agent contract, declared authority policy, and
    /// core-negotiated registry used exclusively through `layerx-intents`.
    #[must_use]
    pub fn new(
        contract: C,
        entropy: E,
        policy: SessionPolicy,
        rotation_registry: ModuleRegistry,
    ) -> Self {
        Self {
            contract,
            entropy,
            policy,
            rotation_registry,
            agents: BTreeMap::new(),
            rotations: BTreeMap::new(),
            rotation_history: Vec::new(),
            installations: BTreeMap::new(),
            renewals: BTreeMap::new(),
            stops: BTreeMap::new(),
            announcements: BTreeMap::new(),
            durable: None,
            poisoned: false,
        }
    }

    /// Opens the durable lifecycle journal and reconciles attempted external steps
    /// before returning authority to the caller. An unavailable peer refuses startup.
    /// # Errors
    /// Returns an error if the requested data cannot be read or validated.
    pub fn open(
        root: impl AsRef<std::path::Path>,
        contract: C,
        entropy: E,
        policy: SessionPolicy,
        registry: ModuleRegistry,
    ) -> Result<Self, SessionKeyError> {
        let (file, state) = durable::StateFile::open(root.as_ref())?;
        let mut owner = Self::new(contract, entropy, policy, registry);
        if let Some(state) = state {
            state.restore(&mut owner)?;
        }
        owner.durable = Some(file);
        owner.persist()?;
        owner.recover_pending()?;
        owner.reconcile_authority()?;
        Ok(owner)
    }

    pub fn into_contract(self) -> C {
        self.contract
    }

    #[must_use]
    pub fn rotation_history(&self) -> &[RotationJourney] {
        &self.rotation_history
    }

    fn healthy(&self) -> Result<(), SessionKeyError> {
        if self.poisoned {
            Err(SessionKeyError::InvalidEvidence)
        } else {
            Ok(())
        }
    }

    fn persist(&mut self) -> Result<(), SessionKeyError> {
        self.healthy()?;
        if let Some(file) = &self.durable {
            if let Err(error) = file.save(&durable::State::capture(self)) {
                self.poisoned = true;
                return Err(error);
            }
        }
        Ok(())
    }

    fn reconcile_authority(&mut self) -> Result<(), SessionKeyError> {
        let leases: Vec<_> = self
            .agents
            .values()
            .flat_map(|agent| std::iter::once(agent.current.clone()).chain(agent.retired.clone()))
            .collect();
        for lease in leases {
            let identity = self
                .contract
                .identity_snapshot(&lease.principal, &lease.agent_did)?;
            verify_identity(&identity)?;
            let observed = self.contract.session_authority(&target(&lease))?;
            let provision = observed.provision;
            if provision.grant_id != lease.grant_id
                || provision.session_public_key != lease.session_public_key
                || provision.daemon_session_id != lease.daemon_session_id
                || provision.receipt_digest != lease.provision_receipt_digest
                || provision.receipt_digest != provision.expected_digest()
                || provision.rung < Rung::Receipted
            {
                return Err(SessionKeyError::InvalidEvidence);
            }
            match lease.state {
                SessionLeaseState::Active
                    if !observed.daemon_suspended && observed.revocation.is_none() => {}
                SessionLeaseState::Revoked {
                    reason,
                    revoked_at,
                    revocation_receipt_digest,
                    ..
                } if observed.daemon_suspended => {
                    let revocation = observed
                        .revocation
                        .ok_or(SessionKeyError::InvalidEvidence)?;
                    if revocation.grant_id != lease.grant_id
                        || revocation.reason != reason
                        || revocation.observed_at != revoked_at
                        || revocation.receipt_digest != revocation_receipt_digest
                        || revocation.receipt_digest != revocation.expected_digest()
                        || revocation.rung < Rung::Receipted
                    {
                        return Err(SessionKeyError::InvalidEvidence);
                    }
                }
                _ => return Err(SessionKeyError::InvalidEvidence),
            }
        }
        Ok(())
    }

    fn recover_pending(&mut self) -> Result<(), SessionKeyError> {
        let renewals: Vec<_> = self.renewals.keys().cloned().collect();
        for (principal, did) in renewals {
            let transition = self.renewals[&(principal.clone(), did.clone())].clone();
            self.renew_one(&principal, &did, transition.requested_at, transition.reason)?;
        }
        let stops: Vec<_> = self.stops.keys().cloned().collect();
        for (principal, did) in stops {
            let stop = self.stops[&(principal.clone(), did.clone())].clone();
            self.stop_authority(&principal, &did, stop.requested_at, stop.reason)?;
        }
        let installations: Vec<_> = self.installations.keys().cloned().collect();
        for (principal, did) in installations {
            let activities = self.installations[&(principal.clone(), did.clone())]
                .activities
                .clone();
            if self.agents.contains_key(&(principal.clone(), did.clone())) {
                self.resume(&principal, &did)?;
            } else {
                self.provision(&principal, &did, activities)?;
            }
        }
        let announcements: Vec<_> = self.announcements.keys().cloned().collect();
        for key in announcements {
            self.finish_announcement(&key)?;
        }
        let rotations: Vec<_> = self.rotations.keys().cloned().collect();
        for (principal, did) in rotations {
            self.reconcile_rotation(&principal, &did)?;
        }
        Ok(())
    }

    fn maintain_bounded(&mut self, maximum: usize, now: u64) -> Result<usize, SessionKeyError> {
        self.healthy()?;
        let mut completed = 0;
        let stops: Vec<_> = self
            .stops
            .keys()
            .filter(|key| !self.renewals.contains_key(*key))
            .take(maximum)
            .cloned()
            .collect();
        for key in stops {
            let stop = self.stops[&key].clone();
            self.stop_authority(&key.0, &key.1, stop.requested_at, stop.reason)?;
            completed += 1;
        }
        let installations: Vec<_> = self
            .installations
            .keys()
            .filter(|key| !self.renewals.contains_key(*key))
            .take(maximum - completed)
            .cloned()
            .collect();
        for key in installations {
            let activities = self.installations[&key].activities.clone();
            if self.agents.contains_key(&key) {
                self.resume(&key.0, &key.1)?;
            } else {
                self.provision(&key.0, &key.1, activities)?;
            }
            completed += 1;
        }
        let due: Vec<_> = self
            .agents
            .iter()
            .filter(|(key, agent)| {
                self.renewals.contains_key(*key)
                    || (agent.state == ManagedAgentState::Active
                        && !self.stops.contains_key(*key)
                        && agent.current.expires_at.saturating_sub(now)
                            <= self.policy.renewal_lead_seconds)
            })
            .map(|(key, _)| key.clone())
            .take(maximum - completed)
            .collect();
        for key in due {
            let reason = self
                .renewals
                .get(&key)
                .map_or(RevocationReason::Renewed, |pending| pending.reason);
            self.renew_one(&key.0, &key.1, now, reason)?;
            completed += 1;
        }
        let announcements: Vec<_> = self
            .announcements
            .keys()
            .take(maximum - completed)
            .cloned()
            .collect();
        for key in announcements {
            self.finish_announcement(&key)?;
            completed += 1;
        }
        let rotations: Vec<_> = self
            .rotations
            .keys()
            .take(maximum - completed)
            .cloned()
            .collect();
        for key in rotations {
            self.reconcile_rotation(&key.0, &key.1)?;
            completed += 1;
        }
        Ok(completed)
    }

    /// Borrows the contract for read-only qualification and status surfaces.
    #[must_use]
    pub const fn contract(&self) -> &C {
        &self.contract
    }

    /// Borrows the contract for boundary driving and integration qualification.
    #[must_use]
    pub const fn contract_mut(&mut self) -> &mut C {
        &mut self.contract
    }

    /// Returns the current public session record.
    #[must_use]
    pub fn session(&self, principal: &PrincipalId, did: &Did) -> Option<&SessionLease> {
        self.agents
            .get(&(principal.clone(), did.clone()))
            .map(|agent| &agent.current)
    }

    /// Provisions a fresh, exact-scope protocol session key and joins it to a
    /// real daemon permission session. Primary key material is never read.
    ///
    /// # Errors
    ///
    /// Refuses duplicate active authority, empty scope, unverified identity,
    /// unsafe grant bounds, entropy failure, or mismatched agent evidence.
    pub fn provision(
        &mut self,
        principal: &PrincipalId,
        did: &Did,
        permitted_activity_types: Vec<ActivityType>,
    ) -> Result<SessionLease, SessionKeyError> {
        self.healthy()?;
        let key = (principal.clone(), did.clone());
        if let Some(existing) = self.agents.get(&key) {
            return Err(match existing.state {
                ManagedAgentState::Active => SessionKeyError::AlreadyProvisioned,
                ManagedAgentState::Paused => SessionKeyError::Paused,
                ManagedAgentState::Archived => SessionKeyError::Archived,
            });
        }
        let lease = self.provision_new(principal, did, permitted_activity_types)?;
        self.agents.insert(
            key,
            ManagedAgent {
                state: ManagedAgentState::Active,
                current: lease.clone(),
                retired: Vec::new(),
            },
        );
        self.installations.remove(&(principal.clone(), did.clone()));
        self.persist()?;
        Ok(lease)
    }

    /// Immediately suspends daemon permissions, then proves targeted protocol
    /// revocation. A protocol refusal leaves an honest suspended state that can
    /// be retried instead of restoring permission.
    ///
    /// # Errors
    ///
    /// Refuses unknown agents and propagates exact daemon or protocol failures.
    pub fn pause(
        &mut self,
        principal: &PrincipalId,
        did: &Did,
        requested_at: u64,
    ) -> Result<RevocationOutcome, SessionKeyError> {
        self.stop_authority(principal, did, requested_at, RevocationReason::Paused)
    }

    /// Permanently suspends daemon permissions and retires protocol authority.
    ///
    /// # Errors
    ///
    /// Refuses unknown agents and propagates exact daemon or protocol failures.
    pub fn archive(
        &mut self,
        principal: &PrincipalId,
        did: &Did,
        requested_at: u64,
    ) -> Result<RevocationOutcome, SessionKeyError> {
        self.stop_authority(principal, did, requested_at, RevocationReason::Archived)
    }

    /// Restores a paused agent only through a fresh protocol grant and fresh
    /// daemon permission session. Archived agents can never resume.
    ///
    /// # Errors
    ///
    /// Refuses active, archived, unknown, or unprovable agent authority.
    pub fn resume(
        &mut self,
        principal: &PrincipalId,
        did: &Did,
    ) -> Result<SessionLease, SessionKeyError> {
        self.healthy()?;
        let key = (principal.clone(), did.clone());
        let (state, activities) = self
            .agents
            .get(&key)
            .map(|agent| (agent.state, agent.current.permitted_activity_types.clone()))
            .ok_or(SessionKeyError::NotProvisioned)?;
        match state {
            ManagedAgentState::Active => return Err(SessionKeyError::AlreadyProvisioned),
            ManagedAgentState::Archived => return Err(SessionKeyError::Archived),
            ManagedAgentState::Paused => {}
        }
        let replacement = self.provision_new(principal, did, activities)?;
        let agent = self
            .agents
            .get_mut(&key)
            .ok_or(SessionKeyError::NotProvisioned)?;
        let previous = std::mem::replace(&mut agent.current, replacement.clone());
        agent.retired.push(previous);
        agent.state = ManagedAgentState::Active;
        self.installations.remove(&key);
        self.persist()?;
        Ok(replacement)
    }

    /// Automatically renews every active session inside the declared lead
    /// window, installing its replacement before retiring the old grant.
    pub fn renew_expiring(&mut self, now: u64) -> Vec<Result<RenewalOutcome, SessionKeyError>> {
        let due = self
            .agents
            .iter()
            .filter(|(_, agent)| {
                agent.state == ManagedAgentState::Active
                    && agent.current.expires_at.saturating_sub(now)
                        <= self.policy.renewal_lead_seconds
            })
            .map(|((principal, did), _)| (principal.clone(), did.clone()))
            .collect::<Vec<_>>();
        due.into_iter()
            .map(|(principal, did)| {
                self.renew_one(&principal, &did, now, RevocationReason::Renewed)
            })
            .collect()
    }

    /// Announces human or agent primary-key rotation using a KMS-held pending
    /// key and the sole Human payload authority, `layerx-intents`.
    ///
    /// # Errors
    ///
    /// Refuses foreign key classes, unverified identity, malformed timing,
    /// competing rotation, compilation failure, or mismatched protocol evidence.
    #[allow(clippy::too_many_arguments)]
    pub fn announce_rotation(
        &mut self,
        keystore: &Keystore,
        principal: &PrincipalId,
        subject: RotationSubject,
        did: &Did,
        current_key_id: &KeyId,
        pending_key_id: &KeyId,
        challenge_delay_seconds: u64,
        effective_sequence: u64,
    ) -> Result<RotationJourney, SessionKeyError> {
        self.healthy()?;
        let key = (principal.clone(), did.clone());
        if self.rotations.contains_key(&key) || self.announcements.contains_key(&key) {
            return Err(SessionKeyError::RotationAlreadyOpen);
        }
        let current = keystore.describe(principal, current_key_id)?;
        let pending = keystore.describe(principal, pending_key_id)?;
        if current.class != subject.key_class() || pending.class != subject.key_class() {
            return Err(SessionKeyError::KeyClassMismatch);
        }
        let identity = self.contract.identity_snapshot(principal, did)?;
        verify_identity(&identity)?;
        if identity.primary_public_key != current.public_key || effective_sequence == 0 {
            return Err(SessionKeyError::InvalidIdentity);
        }
        let effective_at = identity
            .protocol_time
            .checked_add(challenge_delay_seconds)
            .ok_or(SessionKeyError::TimeOverflow)?;
        let lapse_at = effective_at
            .checked_add(challenge_delay_seconds)
            .ok_or(SessionKeyError::TimeOverflow)?;
        let challenge_delay = PlainTime::new(challenge_delay_seconds)?;
        let window = TimestampBound::new(effective_at, lapse_at)
            .map_err(|_| SessionKeyError::InvalidRotationWindow)?;
        let rotation = KeyRotation::new(
            did.clone(),
            PublicKey::new(pending.public_key),
            window,
            Sequence::from_u64(effective_sequence),
        )?;
        let intent = Intent::v1(IntentKind::KeyRotation(rotation));
        let compiled = compile(&intent, &self.rotation_registry)?;
        compiled.verify_disclosure(&intent)?;
        let expected_payload_hash = compiled.payload_hash();
        let journey = RotationJourney {
            tx_hash: [0; 32],
            principal: principal.clone(),
            subject,
            did: did.clone(),
            current_key_id: current_key_id.clone(),
            pending_key_id: pending_key_id.clone(),
            current_public_key: current.public_key,
            pending_public_key: pending.public_key,
            announced_at: identity.protocol_time,
            effective_at,
            lapse_at,
            effective_sequence,
            challenge_delay,
            announcement_receipt_digest: [0; 32],
            state: RotationJourneyState::ChallengeOpen,
        };
        self.announcements.insert(
            key.clone(),
            durable::Announcement {
                journey,
                payload_hash: expected_payload_hash,
                attempted: false,
            },
        );
        self.persist()?;
        self.finish_announcement(&key)
    }

    fn finish_announcement(
        &mut self,
        key: &(PrincipalId, Did),
    ) -> Result<RotationJourney, SessionKeyError> {
        let pending = self
            .announcements
            .get(key)
            .cloned()
            .ok_or(SessionKeyError::RotationNotFound)?;
        let mut journey = pending.journey;
        let recovered = if pending.attempted {
            self.contract
                .recover_rotation(&key.0, &key.1, pending.payload_hash)?
        } else {
            None
        };
        let evidence = if let Some(evidence) = recovered {
            evidence
        } else {
            let rotation = KeyRotation::new(
                journey.did.clone(),
                PublicKey::new(journey.pending_public_key),
                TimestampBound::new(journey.effective_at, journey.lapse_at)
                    .map_err(|_| SessionKeyError::InvalidRotationWindow)?,
                Sequence::from_u64(journey.effective_sequence),
            )?;
            let intent = Intent::v1(IntentKind::KeyRotation(rotation));
            let compiled = compile(&intent, &self.rotation_registry)?;
            if compiled.payload_hash() != pending.payload_hash {
                return Err(SessionKeyError::InvalidEvidence);
            }
            let disclosure = compiled.verify_disclosure(&intent)?;
            self.announcements
                .get_mut(key)
                .ok_or(SessionKeyError::RotationNotFound)?
                .attempted = true;
            self.persist()?;
            self.contract.announce_rotation(RotationSubmission {
                principal: key.0.clone(),
                subject: journey.subject,
                did: key.1.clone(),
                current_public_key: journey.current_public_key,
                pending_public_key: journey.pending_public_key,
                effective_at: journey.effective_at,
                lapse_at: journey.lapse_at,
                effective_sequence: journey.effective_sequence,
                intent,
                compiled,
                disclosure,
            })?
        };
        if evidence.tx_hash == [0; 32]
            || evidence.payload_hash != pending.payload_hash
            || evidence.pending_public_key != journey.pending_public_key
            || evidence.effective_at != journey.effective_at
            || evidence.lapse_at != journey.lapse_at
            || evidence.effective_sequence != journey.effective_sequence
            || evidence.observed_at < journey.announced_at
            || evidence.rung < Rung::Receipted
            || evidence.receipt_digest != evidence.expected_digest()
        {
            return Err(SessionKeyError::InvalidEvidence);
        }
        journey.tx_hash = evidence.tx_hash;
        journey.announcement_receipt_digest = evidence.receipt_digest;
        self.rotations.insert(key.clone(), journey.clone());
        self.announcements.remove(key);
        self.persist()?;
        Ok(journey)
    }

    /// Reconciles a tracked rotation from state-proven protocol facts. Agent
    /// rotation automatically restores operating continuity through a new
    /// scoped session after the old daemon sessions are invalidated.
    ///
    /// # Errors
    ///
    /// Refuses missing journeys, unverified or inconsistent observations, and
    /// any failed agent-session restoration.
    pub fn reconcile_rotation(
        &mut self,
        principal: &PrincipalId,
        did: &Did,
    ) -> Result<(RotationJourney, Option<SessionLease>), SessionKeyError> {
        self.healthy()?;
        let key = (principal.clone(), did.clone());
        let Some(tracked) = self.rotations.get(&key).cloned() else {
            return self
                .rotation_history
                .iter()
                .rev()
                .find(|journey| journey.principal == *principal && journey.did == *did)
                .cloned()
                .map(|journey| (journey, None))
                .ok_or(SessionKeyError::RotationNotFound);
        };
        let observed = self.contract.rotation_observation(principal, did)?;
        if observed.tx_hash != tracked.tx_hash
            || observed.rung < Rung::Receipted
            || observed.effective_at != tracked.effective_at
            || observed.lapse_at != tracked.lapse_at
            || observed.effective_sequence != tracked.effective_sequence
            || observed.observed_at < tracked.announced_at
        {
            return Err(SessionKeyError::InvalidEvidence);
        }
        let pending = observed.primary_public_key == tracked.current_public_key
            && (observed.pending_public_key == Some(tracked.pending_public_key)
                || (observed.pending_public_key.is_none()
                    && observed.observed_at > tracked.lapse_at));
        let effective = observed.primary_public_key == tracked.pending_public_key
            && observed.pending_public_key.is_none()
            && observed.superseded_public_key == Some(tracked.current_public_key);
        if !pending && !effective {
            return Err(SessionKeyError::InvalidEvidence);
        }
        if observed.rung != Rung::Committed {
            return Ok((tracked, None));
        }
        let mut replacement = None;
        let state = if pending {
            if observed.observed_at < tracked.effective_at {
                RotationJourneyState::ChallengeOpen
            } else if observed.observed_at <= tracked.lapse_at {
                RotationJourneyState::ReadyToCommit
            } else {
                RotationJourneyState::Lapsed
            }
        } else if observed.primary_public_key == tracked.pending_public_key
            && observed.pending_public_key.is_none()
            && observed.superseded_public_key == Some(tracked.current_public_key)
        {
            if tracked.subject == RotationSubject::Agent
                && !matches!(tracked.state, RotationJourneyState::Effective { .. })
            {
                if let Some(agent) = self.agents.get(&key) {
                    if agent.state == ManagedAgentState::Active {
                        let renewed = self.renew_one(
                            principal,
                            did,
                            observed.observed_at,
                            RevocationReason::PrimaryKeyRotated,
                        )?;
                        replacement = Some(renewed.replacement);
                    }
                }
            }
            RotationJourneyState::Effective {
                superseded_key_usable_before_sequence: tracked.effective_sequence,
                observed_sequence: observed.observed_sequence,
            }
        } else {
            return Err(SessionKeyError::InvalidEvidence);
        };
        let journey = self
            .rotations
            .get_mut(&key)
            .ok_or(SessionKeyError::RotationNotFound)?;
        journey.state = state;
        let journey = journey.clone();
        if matches!(
            journey.state,
            RotationJourneyState::Effective { .. } | RotationJourneyState::Lapsed
        ) {
            self.rotations.remove(&key);
            self.rotation_history.push(journey.clone());
        }
        self.persist()?;
        Ok((journey, replacement))
    }

    fn provision_new(
        &mut self,
        principal: &PrincipalId,
        did: &Did,
        activities: Vec<ActivityType>,
    ) -> Result<SessionLease, SessionKeyError> {
        self.healthy()?;
        let key = (principal.clone(), did.clone());
        if activities.is_empty() {
            return Err(SessionKeyError::InvalidActivityScope);
        }
        if !self.installations.contains_key(&key) {
            let identity = self.contract.identity_snapshot(principal, did)?;
            verify_identity(&identity)?;
            let expires_at = identity
                .protocol_time
                .checked_add(self.policy.lifetime_seconds)
                .ok_or(SessionKeyError::TimeOverflow)?;
            let entropy = self.entropy.next_session_entropy()?;
            self.installations.insert(
                key.clone(),
                durable::Installation {
                    principal: principal.clone(),
                    did: did.clone(),
                    seed: *entropy.0,
                    grantor: identity.protocol_identity,
                    revocation_sequence: identity.revocation_sequence,
                    not_before: identity.protocol_time,
                    expires_at,
                    core_sequence: identity.core_sequence,
                    activities,
                    attempted: false,
                    installed: None,
                },
            );
            self.persist()?;
        }
        let pending = self
            .installations
            .get(&key)
            .cloned()
            .ok_or(SessionKeyError::NotProvisioned)?;
        if let Some(lease) = &pending.installed {
            return Ok(lease.clone());
        }
        let issued = pending.issue()?;
        let recovered = if pending.attempted {
            self.contract
                .recover_provision(principal, did, issued.grant_id)?
        } else {
            None
        };
        let evidence = if let Some(evidence) = recovered {
            evidence
        } else {
            self.installations
                .get_mut(&key)
                .ok_or(SessionKeyError::NotProvisioned)?
                .attempted = true;
            self.persist()?;
            self.contract.provision_session(AgentSessionProvision {
                principal: principal.clone(),
                agent_did: did.clone(),
                issued: issued.clone(),
                daemon_scopes: self.policy.daemon_scopes.clone(),
                daemon_policy_version: self.policy.policy_version.clone(),
                secret: AgentSessionSecret(Zeroizing::new(pending.seed)),
            })?
        };
        if evidence.grant_id != issued.grant_id
            || evidence.session_public_key != issued.session_public_key
            || evidence.daemon_session_id == [0; 32]
            || evidence.protocol_sequence < pending.core_sequence
            || evidence.observed_at < pending.not_before
            || evidence.rung < Rung::Receipted
            || evidence.receipt_digest != evidence.expected_digest()
        {
            return Err(SessionKeyError::InvalidEvidence);
        }
        let lease = SessionLease {
            principal: principal.clone(),
            agent_did: did.clone(),
            session_public_key: issued.session_public_key,
            grant_id: issued.grant_id,
            daemon_session_id: evidence.daemon_session_id,
            permitted_activity_types: issued.permitted_activity_types,
            issued_at: pending.not_before,
            expires_at: issued.expires_at,
            revocation_sequence: issued.revocation_sequence,
            provision_receipt_digest: evidence.receipt_digest,
            state: SessionLeaseState::Active,
        };
        self.installations
            .get_mut(&key)
            .ok_or(SessionKeyError::NotProvisioned)?
            .installed = Some(lease.clone());
        self.persist()?;
        Ok(lease)
    }

    fn pending_stop(
        &mut self,
        key: &(PrincipalId, Did),
        requested_at: u64,
        reason: RevocationReason,
    ) -> Result<u64, SessionKeyError> {
        if let Some(stop) = self.stops.get(key) {
            if stop.reason != reason {
                return Err(SessionKeyError::InvalidEvidence);
            }
        } else {
            let existing = self
                .agents
                .get(key)
                .ok_or(SessionKeyError::NotProvisioned)?;
            if let SessionLeaseState::Revoked {
                reason: completed,
                suspended_at,
                ..
            } = existing.current.state
            {
                if completed != reason || requested_at > suspended_at {
                    return Err(SessionKeyError::NotProvisioned);
                }
            }
            self.stops.insert(
                key.clone(),
                durable::Stop {
                    requested_at,
                    reason,
                },
            );
            self.persist()?;
        }
        Ok(self.stops[key].requested_at)
    }

    fn stop_authority(
        &mut self,
        principal: &PrincipalId,
        did: &Did,
        requested_at: u64,
        reason: RevocationReason,
    ) -> Result<RevocationOutcome, SessionKeyError> {
        let key = (principal.clone(), did.clone());
        self.healthy()?;
        let requested_at = self.pending_stop(&key, requested_at, reason)?;
        let lease = self
            .agents
            .get(&key)
            .map(|agent| agent.current.clone())
            .ok_or(SessionKeyError::NotProvisioned)?;
        if let SessionLeaseState::Revoked {
            reason: completed_reason,
            suspended_at,
            revoked_at,
            suspension_receipt_digest,
            revocation_receipt_digest,
        } = lease.state
        {
            if completed_reason != reason || requested_at > suspended_at {
                return Err(SessionKeyError::NotProvisioned);
            }
            let latency_seconds = revoked_at.saturating_sub(suspended_at);
            let outcome = RevocationOutcome {
                reason,
                requested_at,
                suspended_at,
                revoked_at,
                latency_seconds,
                within_declared_target: latency_seconds
                    <= self.policy.maximum_revocation_latency_seconds,
                suspension_receipt_digest,
                revocation_receipt_digest,
            };
            self.stops.remove(&key);
            self.persist()?;
            return Ok(outcome);
        }
        let target = target(&lease);
        let suspension = match lease.state {
            SessionLeaseState::Active => {
                let evidence = self
                    .contract
                    .suspend_permissions(&target, reason, requested_at)?;
                validate_suspension(&evidence, &target, reason, requested_at)?;
                if let Some(agent) = self.agents.get_mut(&key) {
                    agent.current.state = SessionLeaseState::SuspendedAwaitingProtocolRevocation {
                        reason,
                        suspended_at: evidence.observed_at,
                        suspension_receipt_digest: evidence.receipt_digest,
                    };
                }
                self.persist()?;
                evidence
            }
            SessionLeaseState::SuspendedAwaitingProtocolRevocation {
                reason: pending_reason,
                suspended_at,
                suspension_receipt_digest,
            } if pending_reason == reason => SuspensionEvidence {
                grant_id: target.grant_id,
                daemon_session_id: target.daemon_session_id,
                reason,
                observed_at: suspended_at,
                receipt_digest: suspension_receipt_digest,
            },
            SessionLeaseState::SuspendedAwaitingProtocolRevocation { .. }
            | SessionLeaseState::Revoked { .. } => return Err(SessionKeyError::NotProvisioned),
        };
        let revocation = self
            .contract
            .revoke_protocol_session(&target, reason, requested_at)?;
        validate_revocation(&revocation, &target, reason, suspension.observed_at)?;
        let outcome =
            revocation_outcome(&self.policy, reason, requested_at, &suspension, &revocation);
        let agent = self
            .agents
            .get_mut(&key)
            .ok_or(SessionKeyError::NotProvisioned)?;
        agent.current.state = SessionLeaseState::Revoked {
            reason,
            suspended_at: suspension.observed_at,
            revoked_at: revocation.observed_at,
            suspension_receipt_digest: suspension.receipt_digest,
            revocation_receipt_digest: revocation.receipt_digest,
        };
        agent.state = match reason {
            RevocationReason::Paused => ManagedAgentState::Paused,
            RevocationReason::Archived => ManagedAgentState::Archived,
            RevocationReason::Renewed | RevocationReason::PrimaryKeyRotated => agent.state,
        };
        self.stops.remove(&key);
        self.persist()?;
        Ok(outcome)
    }

    fn renew_one(
        &mut self,
        principal: &PrincipalId,
        did: &Did,
        requested_at: u64,
        reason: RevocationReason,
    ) -> Result<RenewalOutcome, SessionKeyError> {
        let key = (principal.clone(), did.clone());
        self.healthy()?;
        if !self.renewals.contains_key(&key) {
            let previous = self
                .agents
                .get(&key)
                .filter(|agent| agent.state == ManagedAgentState::Active)
                .map(|agent| agent.current.clone())
                .ok_or(SessionKeyError::NotProvisioned)?;
            self.renewals.insert(
                key.clone(),
                durable::Renewal {
                    previous,
                    replacement: None,
                    requested_at,
                    reason,
                },
            );
            self.persist()?;
        }
        if self
            .stops
            .get(&key)
            .is_some_and(|stop| stop.reason != reason)
        {
            return Err(SessionKeyError::InvalidEvidence);
        }
        let pending = self.renewals[&key].clone();
        if pending.reason != reason {
            return Err(SessionKeyError::InvalidEvidence);
        }
        let replacement = if let Some(replacement) = &pending.replacement {
            replacement.clone()
        } else {
            let replacement = self.provision_new(
                principal,
                did,
                pending.previous.permitted_activity_types.clone(),
            )?;
            self.renewals
                .get_mut(&key)
                .ok_or(SessionKeyError::NotProvisioned)?
                .replacement = Some(replacement.clone());
            self.persist()?;
            replacement
        };
        let previous_revocation =
            self.stop_authority(principal, did, pending.requested_at, pending.reason)?;
        let agent = self
            .agents
            .get_mut(&key)
            .ok_or(SessionKeyError::NotProvisioned)?;
        let retired = std::mem::replace(&mut agent.current, replacement.clone());
        agent.retired.push(retired);
        self.renewals.remove(&key);
        self.installations.remove(&key);
        self.persist()?;
        Ok(RenewalOutcome {
            previous_grant_id: pending.previous.grant_id,
            replacement,
            previous_revocation,
        })
    }
}

fn target(lease: &SessionLease) -> SessionTarget {
    SessionTarget {
        principal: lease.principal.clone(),
        agent_did: lease.agent_did.clone(),
        grant_id: lease.grant_id,
        daemon_session_id: lease.daemon_session_id,
    }
}

fn verify_identity(identity: &ProtocolIdentitySnapshot) -> Result<(), SessionKeyError> {
    if identity.protocol_identity == [0; 32]
        || identity.primary_public_key == [0; 32]
        || identity.revocation_sequence == 0
        || identity.protocol_time == 0
        || identity.rung != Rung::Committed
    {
        Err(SessionKeyError::InvalidIdentity)
    } else {
        Ok(())
    }
}

fn validate_suspension(
    evidence: &SuspensionEvidence,
    target: &SessionTarget,
    reason: RevocationReason,
    requested_at: u64,
) -> Result<(), SessionKeyError> {
    if evidence.grant_id != target.grant_id
        || evidence.daemon_session_id != target.daemon_session_id
        || evidence.reason != reason
        || evidence.observed_at < requested_at
        || evidence.receipt_digest != evidence.expected_digest()
    {
        Err(SessionKeyError::InvalidEvidence)
    } else {
        Ok(())
    }
}

fn validate_revocation(
    evidence: &RevocationEvidence,
    target: &SessionTarget,
    reason: RevocationReason,
    suspended_at: u64,
) -> Result<(), SessionKeyError> {
    if evidence.grant_id != target.grant_id
        || evidence.reason != reason
        || evidence.observed_at < suspended_at
        || evidence.rung < Rung::Receipted
        || evidence.receipt_digest != evidence.expected_digest()
    {
        Err(SessionKeyError::InvalidEvidence)
    } else {
        Ok(())
    }
}

fn revocation_outcome(
    policy: &SessionPolicy,
    reason: RevocationReason,
    requested_at: u64,
    suspension: &SuspensionEvidence,
    revocation: &RevocationEvidence,
) -> RevocationOutcome {
    let latency_seconds = revocation.observed_at.saturating_sub(requested_at);
    RevocationOutcome {
        reason,
        requested_at,
        suspended_at: suspension.observed_at,
        revoked_at: revocation.observed_at,
        latency_seconds,
        within_declared_target: latency_seconds <= policy.maximum_revocation_latency_seconds,
        suspension_receipt_digest: suspension.receipt_digest,
        revocation_receipt_digest: revocation.receipt_digest,
    }
}

fn digest_fields(domain: &[u8], fields: &[&[u8]]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(domain);
    for field in fields {
        hasher.update(field.len().to_be_bytes());
        hasher.update(field);
    }
    hasher.finalize().into()
}

/// Durable session lifecycle maintenance owner for the production component scheduler.
/// Construction refuses the non-durable development helper.
pub struct SessionLifecycleDriver<C: AgentSessionContract, E: SessionEntropySource> {
    owner: std::sync::Mutex<SessionKeyProvisioner<C, E>>,
    healthy: std::sync::atomic::AtomicBool,
}
impl<C: AgentSessionContract, E: SessionEntropySource> SessionLifecycleDriver<C, E> {
    /// # Errors
    /// Returns an error if validation or a required operation fails.
    pub fn new(owner: SessionKeyProvisioner<C, E>) -> Result<Self, SessionKeyError> {
        if owner.durable.is_none() || owner.poisoned {
            return Err(SessionKeyError::InvalidPolicy);
        }
        Ok(Self {
            owner: std::sync::Mutex::new(owner),
            healthy: std::sync::atomic::AtomicBool::new(true),
        })
    }
    pub fn healthy(&self) -> bool {
        self.healthy.load(std::sync::atomic::Ordering::Acquire)
    }
    /// # Errors
    /// Returns an error if validation or a required operation fails.
    pub fn session(
        &self,
        principal: &PrincipalId,
        did: &Did,
    ) -> Result<Option<SessionLease>, SessionKeyError> {
        let owner = self
            .owner
            .lock()
            .map_err(|_| SessionKeyError::InvalidEvidence)?;
        Ok(owner.session(principal, did).cloned())
    }

    /// # Errors
    /// Returns an error if validation or a required operation fails.
    pub fn into_provisioner(self) -> Result<SessionKeyProvisioner<C, E>, SessionKeyError> {
        self.owner
            .into_inner()
            .map_err(|_| SessionKeyError::InvalidEvidence)
    }
}
impl<C: AgentSessionContract + Send + 'static, E: SessionEntropySource + Send + 'static>
    crate::server::ComponentMaintenance for SessionLifecycleDriver<C, E>
{
    fn maintain(
        &self,
        maximum_items: usize,
        now: u64,
    ) -> Result<usize, crate::server::backend::ApiFailure> {
        use crate::server::backend::ApiFailure;
        let mut owner = self
            .owner
            .lock()
            .map_err(|_| ApiFailure::upstream_degraded())?;
        owner
            .healthy()
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let Ok(count) = owner.maintain_bounded(maximum_items, now) else {
            self.healthy
                .store(false, std::sync::atomic::Ordering::Release);
            return Err(ApiFailure::upstream_degraded());
        };
        self.healthy
            .store(true, std::sync::atomic::Ordering::Release);
        Ok(count)
    }
    fn set_maintenance_health(&self, healthy: bool) {
        self.healthy
            .store(healthy, std::sync::atomic::Ordering::Release);
    }
}

/// Composes the durable session owner with the real Human component scheduler.
/// Product operations remain owned by the supplied production component graph.
pub struct SessionMaintainedComponents<B, C: AgentSessionContract, E: SessionEntropySource> {
    backend: std::sync::Arc<B>,
    sessions: SessionLifecycleDriver<C, E>,
}
impl<B, C: AgentSessionContract, E: SessionEntropySource> SessionMaintainedComponents<B, C, E> {
    pub fn new(backend: std::sync::Arc<B>, sessions: SessionLifecycleDriver<C, E>) -> Self {
        Self { backend, sessions }
    }
    pub fn sessions(&self) -> &SessionLifecycleDriver<C, E> {
        &self.sessions
    }
    pub fn into_sessions(self) -> SessionLifecycleDriver<C, E> {
        self.sessions
    }
}
impl<
        B: crate::server::backend::HumanApiComponents,
        C: AgentSessionContract + Send + 'static,
        E: SessionEntropySource + Send + 'static,
    > crate::server::ComponentMaintenance for SessionMaintainedComponents<B, C, E>
{
    fn maintain(
        &self,
        maximum_items: usize,
        now: u64,
    ) -> Result<usize, crate::server::backend::ApiFailure> {
        crate::server::ComponentMaintenance::maintain(&self.sessions, maximum_items, now)
    }
    fn set_maintenance_health(&self, healthy: bool) {
        crate::server::ComponentMaintenance::set_maintenance_health(&self.sessions, healthy);
    }
}
impl<
        B: crate::server::backend::HumanApiComponents,
        C: AgentSessionContract + Send + 'static,
        E: SessionEntropySource + Send + 'static,
    > crate::server::backend::HumanApiComponents for SessionMaintainedComponents<B, C, E>
{
    fn authorize(
        &self,
        operation: &crate::server::schema::Operation,
        credentials: crate::server::backend::SessionCredentials<'_>,
        trace: &str,
    ) -> Result<crate::server::backend::PrincipalContext, crate::server::backend::ApiFailure> {
        self.backend.authorize(operation, credentials, trace)
    }
    fn admit_bearer_assertion(
        &self,
        operation: &crate::server::schema::Operation,
        credentials: crate::server::backend::BearerCredentials<'_>,
        trace: &str,
    ) -> Result<crate::server::backend::PrincipalContext, crate::server::backend::ApiFailure> {
        self.backend
            .admit_bearer_assertion(operation, credentials, trace)
    }
    fn execute(
        &self,
        request: crate::server::backend::ScopedRequest<'_>,
    ) -> Result<crate::server::backend::BackendResponse, crate::server::backend::ApiFailure> {
        self.backend.execute(request)
    }
    fn stream(
        &self,
        request: crate::server::backend::ScopedRequest<'_>,
        maximum_bytes: usize,
        emit: &mut dyn FnMut(serde_json::Value) -> Result<(), crate::server::backend::ApiFailure>,
        cancelled: &dyn Fn() -> bool,
    ) -> Result<(), crate::server::backend::ApiFailure> {
        self.backend.stream(request, maximum_bytes, emit, cancelled)
    }
    fn readiness(
        &self,
        trace: &str,
    ) -> Result<crate::server::backend::Readiness, crate::server::backend::ApiFailure> {
        let mut readiness = self.backend.readiness(trace)?;
        if !self.sessions.healthy() {
            readiness.human_service = crate::server::backend::ComponentState::Unavailable;
        }
        Ok(readiness)
    }
}
