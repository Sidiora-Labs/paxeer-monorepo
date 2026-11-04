//! Step-up-gated security-center orchestration.
#![allow(clippy::missing_errors_doc)]

use std::fmt::{Display, Formatter};

use sha2::{Digest as _, Sha256};
use zeroize::Zeroizing;

use crate::audit::{
    AuditChain, AuditError, AuditEvent, SecurityChangeKind, StepUpEvidence as AuditStepUpEvidence,
};
use crate::auth::{
    AccessDecision, AuthError, AuthorizationRequest, OperationClass, OperationDigest,
    PasskeyRecord, Passkeys, SessionView, StepUpEvidence,
};
use crate::custody::{CustodyError, KeyId, Keystore};
use crate::store::{PrincipalId, PrincipalScope};
use crate::trace::TraceId;

const ACTION_DOMAIN: &[u8] = b"layerx-human-security-action/v1";
const CEREMONY_DOMAIN: &[u8] = b"layerx-human-security-ceremony/v1";
const TARGET_LIMIT: usize = 256;

/// Security-center mutations that receive distinct operation digests.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SecurityAction {
    AddPasskey,
    RevokePasskey,
    RevokeSession,
    RevokeAllSessions,
    AddAuthenticator,
    DisableAuthenticator,
    RotateBackupCodes,
    RevealRecoveryEvidence,
    ExportPrimaryKey,
}

impl SecurityAction {
    const fn label(self) -> &'static [u8] {
        match self {
            Self::AddPasskey => b"add-passkey",
            Self::RevokePasskey => b"revoke-passkey",
            Self::RevokeSession => b"revoke-session",
            Self::RevokeAllSessions => b"revoke-all-sessions",
            Self::AddAuthenticator => b"add-authenticator",
            Self::DisableAuthenticator => b"disable-authenticator",
            Self::RotateBackupCodes => b"rotate-backup-codes",
            Self::RevealRecoveryEvidence => b"reveal-recovery-evidence",
            Self::ExportPrimaryKey => b"export-primary-key",
        }
    }

    const fn operation_class(self) -> OperationClass {
        match self {
            Self::RevealRecoveryEvidence | Self::ExportPrimaryKey => OperationClass::SecretReveal,
            Self::AddPasskey
            | Self::RevokePasskey
            | Self::RevokeSession
            | Self::RevokeAllSessions
            | Self::AddAuthenticator
            | Self::DisableAuthenticator
            | Self::RotateBackupCodes => OperationClass::SecuritySettings,
        }
    }
}

/// Derives the digest a fresh passkey ceremony must confirm. The principal,
/// action and exact target are all bound, so evidence cannot cross accounts,
/// operation classes or resources.
pub fn security_action_digest(
    principal: &PrincipalId,
    action: SecurityAction,
    target: Option<&str>,
) -> Result<OperationDigest, SecurityError> {
    if target.is_some_and(|value| {
        value.is_empty() || value.len() > TARGET_LIMIT || value.chars().any(char::is_control)
    }) {
        return Err(SecurityError::InvalidTarget);
    }
    let mut digest = Sha256::new();
    digest.update(ACTION_DOMAIN);
    digest.update(principal.as_str().as_bytes());
    digest.update([0]);
    digest.update(action.label());
    digest.update([0]);
    if let Some(target) = target {
        digest.update(target.as_bytes());
    }
    Ok(OperationDigest::new(digest.finalize().into()))
}

/// Safe authenticator metadata. Provider handles are never treated as secrets.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthenticatorMethod {
    pub id: String,
    pub label: String,
    pub enabled_at: u64,
    pub last_used_at: Option<u64>,
}

/// Read-only provider projection for the security center.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthenticatorStatus {
    pub methods: Vec<AuthenticatorMethod>,
    pub backup_codes_remaining: u32,
}

/// A value that must be removed from the client at `remask_at`.
pub struct TimedSecret {
    value: Zeroizing<String>,
    remask_at: u64,
    copyable: bool,
}

impl TimedSecret {
    pub fn new(
        value: impl Into<String>,
        remask_at: u64,
        copyable: bool,
        now: u64,
    ) -> Result<Self, SecurityBoundaryError> {
        let value = Zeroizing::new(value.into());
        if value.is_empty() || remask_at <= now {
            return Err(SecurityBoundaryError::InvalidEvidence);
        }
        Ok(Self {
            value,
            remask_at,
            copyable,
        })
    }

    #[must_use]
    pub fn expose(&self) -> &str {
        &self.value
    }

    #[must_use]
    pub const fn remask_at(&self) -> u64 {
        self.remask_at
    }

    #[must_use]
    pub const fn copyable(&self) -> bool {
        self.copyable
    }
}

impl std::fmt::Debug for TimedSecret {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("TimedSecret")
            .field("value", &"[redacted]")
            .field("remask_at", &self.remask_at)
            .field("copyable", &self.copyable)
            .finish()
    }
}

/// One provider setup awaiting a valid authenticator code.
#[derive(Debug)]
pub struct AuthenticatorSetupChallenge {
    pub setup_id: String,
    pub secret: TimedSecret,
    pub otpauth_uri: TimedSecret,
    pub expires_at: u64,
}

/// Backup codes returned once by the provider.
pub struct BackupCodeSet {
    codes: Zeroizing<Vec<String>>,
    remask_at: u64,
}

impl BackupCodeSet {
    pub fn new(
        codes: Vec<String>,
        remask_at: u64,
        now: u64,
    ) -> Result<Self, SecurityBoundaryError> {
        if codes.is_empty() || codes.iter().any(String::is_empty) || remask_at <= now {
            return Err(SecurityBoundaryError::InvalidEvidence);
        }
        Ok(Self {
            codes: Zeroizing::new(codes),
            remask_at,
        })
    }

    #[must_use]
    pub fn expose(&self) -> &[String] {
        &self.codes
    }

    #[must_use]
    pub const fn remask_at(&self) -> u64 {
        self.remask_at
    }
}

impl std::fmt::Debug for BackupCodeSet {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("BackupCodeSet")
            .field("codes", &"[redacted]")
            .field("remask_at", &self.remask_at)
            .finish()
    }
}

/// Result of proving possession of a newly configured authenticator.
#[derive(Debug)]
pub struct AuthenticatorSetupResult {
    pub method: AuthenticatorMethod,
    pub backup_codes: BackupCodeSet,
}

/// Real authenticator-provider boundary. The provider owns verification and
/// stores only its protected credential state; no local fallback exists.
pub trait AuthenticatorProvider {
    fn status(&self, principal: &PrincipalId)
        -> Result<AuthenticatorStatus, SecurityBoundaryError>;

    fn begin_setup(
        &mut self,
        principal: &PrincipalId,
        label: &str,
        now: u64,
    ) -> Result<AuthenticatorSetupChallenge, SecurityBoundaryError>;

    fn finish_setup(
        &mut self,
        principal: &PrincipalId,
        setup_id: &str,
        code: &str,
        now: u64,
    ) -> Result<AuthenticatorSetupResult, SecurityBoundaryError>;

    fn disable(
        &mut self,
        principal: &PrincipalId,
        authenticator_id: &str,
        now: u64,
    ) -> Result<AuthenticatorStatus, SecurityBoundaryError>;

    fn rotate_backup_codes(
        &mut self,
        principal: &PrincipalId,
        now: u64,
    ) -> Result<BackupCodeSet, SecurityBoundaryError>;
}

/// Receipt source for the recovery details surface. Implementations must
/// locally verify the stored receipt before returning its canonical bytes.
pub trait RecoveryEvidenceProvider {
    fn reveal_verified_receipt(
        &self,
        principal: &PrincipalId,
        evidence_id: &str,
        now: u64,
    ) -> Result<TimedSecret, SecurityBoundaryError>;
}

/// Complete safe metadata shown on initial security-center load.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SecuritySnapshot {
    pub passkeys: Vec<PasskeyRecord>,
    pub sessions: Vec<SessionView>,
    pub authenticators: AuthenticatorStatus,
}

/// Security-center service. It adds authorization and step-up enforcement to
/// real passkey, authenticator-provider and receipt-provider operations.
pub struct SecurityCenter<A, R> {
    authenticators: A,
    recovery: R,
}

impl<A, R> SecurityCenter<A, R>
where
    A: AuthenticatorProvider,
    R: RecoveryEvidenceProvider,
{
    #[must_use]
    pub const fn new(authenticators: A, recovery: R) -> Self {
        Self {
            authenticators,
            recovery,
        }
    }

    pub fn snapshot(
        &self,
        passkeys: &Passkeys,
        scope: &mut PrincipalScope<'_>,
        access_token: &str,
        now: u64,
    ) -> Result<SecuritySnapshot, SecurityError> {
        let passkey_inventory = passkeys.list_passkeys(scope, access_token, now)?;
        let sessions = passkeys.list_sessions(scope, access_token, now)?;
        let authenticators = self.authenticators.status(scope.principal())?;
        Ok(SecuritySnapshot {
            passkeys: passkey_inventory,
            sessions,
            authenticators,
        })
    }

    #[allow(clippy::too_many_arguments)]
    pub fn begin_authenticator_setup(
        &mut self,
        passkeys: &Passkeys,
        scope: &mut PrincipalScope<'_>,
        access_token: &str,
        csrf_token: &str,
        label: &str,
        evidence: &StepUpEvidence,
        now: u64,
    ) -> Result<AuthenticatorSetupChallenge, SecurityError> {
        Self::authorize(
            passkeys,
            scope,
            access_token,
            csrf_token,
            SecurityAction::AddAuthenticator,
            Some(label),
            evidence,
            now,
        )?;
        Ok(self
            .authenticators
            .begin_setup(scope.principal(), label, now)?)
    }

    #[allow(clippy::too_many_arguments)]
    pub fn finish_authenticator_setup(
        &mut self,
        passkeys: &Passkeys,
        scope: &mut PrincipalScope<'_>,
        access_token: &str,
        csrf_token: &str,
        setup_id: &str,
        label: &str,
        code: &str,
        evidence: &StepUpEvidence,
        now: u64,
    ) -> Result<AuthenticatorSetupResult, SecurityError> {
        Self::authorize(
            passkeys,
            scope,
            access_token,
            csrf_token,
            SecurityAction::AddAuthenticator,
            Some(label),
            evidence,
            now,
        )?;
        Ok(self
            .authenticators
            .finish_setup(scope.principal(), setup_id, code, now)?)
    }

    #[allow(clippy::too_many_arguments)]
    pub fn disable_authenticator(
        &mut self,
        passkeys: &Passkeys,
        scope: &mut PrincipalScope<'_>,
        access_token: &str,
        csrf_token: &str,
        authenticator_id: &str,
        evidence: &StepUpEvidence,
        now: u64,
    ) -> Result<AuthenticatorStatus, SecurityError> {
        Self::authorize(
            passkeys,
            scope,
            access_token,
            csrf_token,
            SecurityAction::DisableAuthenticator,
            Some(authenticator_id),
            evidence,
            now,
        )?;
        Ok(self
            .authenticators
            .disable(scope.principal(), authenticator_id, now)?)
    }

    #[allow(clippy::too_many_arguments)]
    pub fn rotate_backup_codes(
        &mut self,
        passkeys: &Passkeys,
        scope: &mut PrincipalScope<'_>,
        access_token: &str,
        csrf_token: &str,
        evidence: &StepUpEvidence,
        now: u64,
    ) -> Result<BackupCodeSet, SecurityError> {
        Self::authorize(
            passkeys,
            scope,
            access_token,
            csrf_token,
            SecurityAction::RotateBackupCodes,
            None,
            evidence,
            now,
        )?;
        Ok(self
            .authenticators
            .rotate_backup_codes(scope.principal(), now)?)
    }

    #[allow(clippy::too_many_arguments)]
    pub fn reveal_recovery_receipt(
        &self,
        passkeys: &Passkeys,
        scope: &mut PrincipalScope<'_>,
        access_token: &str,
        csrf_token: &str,
        evidence_id: &str,
        evidence: &StepUpEvidence,
        now: u64,
    ) -> Result<TimedSecret, SecurityError> {
        Self::authorize(
            passkeys,
            scope,
            access_token,
            csrf_token,
            SecurityAction::RevealRecoveryEvidence,
            Some(evidence_id),
            evidence,
            now,
        )?;
        Ok(self
            .recovery
            .reveal_verified_receipt(scope.principal(), evidence_id, now)?)
    }

    #[allow(clippy::too_many_arguments)]
    fn authorize(
        passkeys: &Passkeys,
        scope: &mut PrincipalScope<'_>,
        access_token: &str,
        csrf_token: &str,
        action: SecurityAction,
        target: Option<&str>,
        evidence: &StepUpEvidence,
        now: u64,
    ) -> Result<(), SecurityError> {
        let digest = security_action_digest(scope.principal(), action, target)?;
        let decision = passkeys.authorize(
            scope,
            access_token,
            Some(csrf_token),
            &AuthorizationRequest {
                operation: action.operation_class(),
                digest: Some(digest),
                step_up: Some(evidence),
                intended_destination: "/app/settings/security",
            },
            now,
        )?;
        if matches!(decision, AccessDecision::Reauthenticate { .. }) {
            return Err(SecurityError::SessionExpired);
        }
        Ok(())
    }
}

/// The one digest a key-export ceremony answers for and the window the client
/// has to finish it.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct KeyExportChallenge {
    pub export_id: String,
    pub confirms: OperationDigest,
    pub expires_at: u64,
}

/// The DID primary key handed to its owner once, with the moment custody
/// stopped signing for the identity.
#[derive(Debug)]
pub struct ExportedKey {
    pub public_key: String,
    pub secret: TimedSecret,
    pub self_custodied_at: u64,
}

/// The step-up ceremony that moves one identity from custodial holding to
/// self-custody. `begin` offers the single operation digest fresh passkey
/// evidence must confirm; `finish` consumes evidence bound to exactly that
/// digest, durably authorizes one handoff, records the audit event and refuses
/// later signing. Delivery retries recover that same handoff until acknowledgement.
pub struct KeyExportCeremony {
    key: KeyId,
    window_seconds: u64,
}

impl KeyExportCeremony {
    /// Binds the ceremony to the key identifier holding the DID primary key
    /// and to the reveal window the client must finish within.
    ///
    /// # Errors
    ///
    /// Refuses a zero or unbounded reveal window.
    pub fn new(key: KeyId, window_seconds: u64) -> Result<Self, SecurityError> {
        if window_seconds == 0 || window_seconds > 600 {
            return Err(SecurityError::InvalidTarget);
        }
        Ok(Self {
            key,
            window_seconds,
        })
    }

    /// Offers the exact digest a fresh passkey ceremony must confirm before the
    /// key leaves custody. A pending handoff keeps its retry identity across
    /// restarts; an acknowledged export cannot start another handoff.
    ///
    /// # Errors
    ///
    /// Returns `KeyExported` for an already self-custodied identity and the
    /// typed custody refusal for an unknown or corrupt key record.
    pub fn begin(
        &self,
        keystore: &Keystore,
        principal: &PrincipalId,
        now: u64,
    ) -> Result<KeyExportChallenge, SecurityError> {
        let handoff = keystore
            .begin_primary_export(principal, &self.key, now, self.window_seconds)
            .map_err(|error| match error {
                CustodyError::SelfCustodied => SecurityError::KeyExported,
                other => SecurityError::Custody(other),
            })?;
        let confirms = security_action_digest(principal, SecurityAction::ExportPrimaryKey, None)?;
        let expires_at = if handoff.authorization.is_some() {
            now.checked_add(self.window_seconds)
                .ok_or(SecurityError::InvalidTarget)?
        } else {
            handoff.expires_at
        };
        Ok(KeyExportChallenge {
            export_id: hex(handoff.id),
            confirms,
            expires_at,
        })
    }

    /// Authorizes one durable key handoff against the digest `begin` offered.
    /// Retries require fresh, principal-scoped evidence and recover the same key
    /// until delivery acknowledgement; custody signing remains refused.
    ///
    /// # Errors
    ///
    /// Refuses an export identifier that is not the one `begin` offered,
    /// evidence bound to any other operation, evidence outside its validity,
    /// an acknowledged export, and returns typed custody and audit refusals.
    #[allow(clippy::too_many_arguments)]
    pub fn finish(
        &self,
        keystore: &Keystore,
        scope: &mut PrincipalScope<'_>,
        export_id: &str,
        evidence: &StepUpEvidence,
        trace: &TraceId,
        now: u64,
    ) -> Result<ExportedKey, SecurityError> {
        let id = parse_export_id(export_id)?;
        Self::validate_evidence(scope, evidence, now)?;
        let remask_at = now
            .checked_add(self.window_seconds)
            .ok_or(SecurityError::InvalidTarget)?;
        let handoff = keystore.authorize_primary_export(
            scope.principal(),
            &self.key,
            id,
            ceremony_digest(evidence),
            now,
        )?;
        Self::audit_handoff(
            scope,
            &handoff,
            trace,
            SecurityChangeKind::KeyExported,
            handoff
                .authorization
                .as_ref()
                .ok_or(SecurityError::InvalidTarget)?
                .at,
        )?;
        let exported = keystore.recover_primary_export(scope.principal(), &self.key, id)?;
        let authorization = handoff.authorization.ok_or(SecurityError::InvalidTarget)?;
        Ok(ExportedKey {
            public_key: hex(exported.public_key()),
            secret: TimedSecret::new(hex(*exported.expose()), remask_at, true, now)?,
            self_custodied_at: authorization.at,
        })
    }

    /// Acknowledges delivery only with fresh principal-bound evidence and a proof
    /// derived from the delivered seed. A lost acknowledgement response is retryable.
    #[allow(clippy::too_many_arguments)]
    pub fn acknowledge(
        &self,
        keystore: &Keystore,
        scope: &mut PrincipalScope<'_>,
        export_id: &str,
        delivery_token: [u8; 32],
        evidence: &StepUpEvidence,
        trace: &TraceId,
        now: u64,
    ) -> Result<(), SecurityError> {
        let id = parse_export_id(export_id)?;
        Self::validate_evidence(scope, evidence, now)?;
        let handoff = keystore.verify_primary_export_delivery(
            scope.principal(),
            &self.key,
            id,
            delivery_token,
        )?;
        Self::audit_handoff(
            scope,
            &handoff,
            trace,
            SecurityChangeKind::KeyExportAcknowledged,
            now,
        )?;
        keystore.acknowledge_primary_handoff(scope.principal(), &self.key, id, delivery_token)?;
        Ok(())
    }

    fn validate_evidence(
        scope: &mut PrincipalScope<'_>,
        evidence: &StepUpEvidence,
        now: u64,
    ) -> Result<(), SecurityError> {
        let confirms =
            security_action_digest(scope.principal(), SecurityAction::ExportPrimaryKey, None)?;
        if evidence.confirms() != confirms {
            return Err(SecurityError::StepUpMismatch);
        }
        if evidence.completed_at() > now || evidence.expires_at() <= now {
            return Err(SecurityError::StepUpExpired);
        }
        Passkeys::validate_step_up(scope, evidence, confirms, now)?;
        Ok(())
    }

    fn audit_handoff(
        scope: &mut PrincipalScope<'_>,
        handoff: &crate::custody::ExportHandoff,
        trace: &TraceId,
        change: SecurityChangeKind,
        recorded_at: u64,
    ) -> Result<(), SecurityError> {
        let authorization = handoff
            .authorization
            .as_ref()
            .ok_or(SecurityError::InvalidTarget)?;
        let event = AuditEvent::SecurityChange {
            change,
            step_up: AuditStepUpEvidence::Fresh {
                ceremony_digest: authorization.ceremony_digest,
            },
        };
        let mut chain = AuditChain::open(scope)?;
        if !chain
            .entries(scope)?
            .iter()
            .any(|entry| entry.event() == &event)
        {
            chain.append(scope, recorded_at, trace, &event, &[])?;
        }
        Ok(())
    }
}

/// Proof the owner computes from the delivered seed before acknowledging it.
pub fn key_export_delivery_token(
    seed: &[u8; 32],
    export_id: &str,
) -> Result<[u8; 32], SecurityError> {
    Ok(crate::custody::export_delivery_token(
        seed,
        parse_export_id(export_id)?,
    ))
}

fn parse_export_id(value: &str) -> Result<[u8; 32], SecurityError> {
    OperationDigest::parse_schema(&format!("opd_{value}"))
        .map(OperationDigest::bytes)
        .map_err(|_| SecurityError::InvalidTarget)
}

fn ceremony_digest(evidence: &StepUpEvidence) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(CEREMONY_DOMAIN);
    digest.update(evidence.challenge_id().as_bytes());
    digest.update(evidence.confirms().bytes());
    digest.update(evidence.passkey_id().as_bytes());
    digest.update(evidence.completed_at().to_be_bytes());
    digest.update(evidence.expires_at().to_be_bytes());
    digest.finalize().into()
}

fn hex(bytes: [u8; 32]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut output = String::with_capacity(64);
    for byte in bytes {
        output.push(char::from(DIGITS[usize::from(byte >> 4)]));
        output.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    output
}

/// Stable failures exposed by external authenticator and recovery providers.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SecurityBoundaryError {
    Unavailable,
    Refused,
    InvalidEvidence,
}

impl Display for SecurityBoundaryError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(match self {
            Self::Unavailable => "security provider is unavailable",
            Self::Refused => "security provider refused the operation",
            Self::InvalidEvidence => "security provider returned invalid evidence",
        })
    }
}

impl std::error::Error for SecurityBoundaryError {}

#[derive(Debug)]
pub enum SecurityError {
    InvalidTarget,
    SessionExpired,
    KeyExported,
    StepUpMismatch,
    StepUpExpired,
    Auth(AuthError),
    Boundary(SecurityBoundaryError),
    Custody(CustodyError),
    Audit(AuditError),
}

impl Display for SecurityError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::InvalidTarget => formatter.write_str("security action target is invalid"),
            Self::SessionExpired => formatter.write_str("session expired; sign in again"),
            Self::KeyExported => formatter
                .write_str("this identity already holds its own key; nothing is left to export"),
            Self::StepUpMismatch => {
                formatter.write_str("step-up evidence is bound to another operation")
            }
            Self::StepUpExpired => formatter.write_str("step-up evidence is outside its validity"),
            Self::Auth(error) => write!(formatter, "security authorization failed: {error}"),
            Self::Boundary(error) => write!(formatter, "security provider failed: {error}"),
            Self::Custody(error) => write!(formatter, "custody refused the export: {error}"),
            Self::Audit(error) => write!(formatter, "key export could not be audited: {error}"),
        }
    }
}

impl std::error::Error for SecurityError {}

impl From<AuthError> for SecurityError {
    fn from(value: AuthError) -> Self {
        Self::Auth(value)
    }
}

impl From<SecurityBoundaryError> for SecurityError {
    fn from(value: SecurityBoundaryError) -> Self {
        Self::Boundary(value)
    }
}

impl From<CustodyError> for SecurityError {
    fn from(value: CustodyError) -> Self {
        Self::Custody(value)
    }
}

impl From<AuditError> for SecurityError {
    fn from(value: AuditError) -> Self {
        Self::Audit(value)
    }
}
