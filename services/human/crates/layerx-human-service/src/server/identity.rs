use crate::onboarding::OnboardingStatus;
use crate::store::PrincipalScope;
use serde_json::{json, Value};

use crate::auth::{
    AssertionChallenge, AssertionProof, Device, OperationDigest, PasskeyRecord,
    RegistrationChallenge, SessionGrant, SessionRevocation, SessionView, StepUpChallenge,
    StepUpEvidence,
};
use crate::time::rfc3339;

use super::backend::{ApiFailure, BackendResponse, SessionSecrets};

/// Exact human-api projections for the durable passkey and session service.
pub struct IdentityProjector;

impl IdentityProjector {
    pub fn checked_passkey(value: &PasskeyRecord) -> Result<Value, ApiFailure> {
        let mut result = json!({"passkey_id":value.passkey_id(),"label":value.label(),
            "created_at":super::projection::unix_time(value.created_at())?});
        if let Some(time) = value.last_used_at() {
            result["last_used_at"] = json!(super::projection::unix_time(time)?);
        }
        Ok(result)
    }

    pub fn checked_passkeys(values: &[PasskeyRecord]) -> Result<Value, ApiFailure> {
        Ok(
            json!({"passkeys":values.iter().map(Self::checked_passkey).collect::<Result<Vec<_>, _>>()?}),
        )
    }

    pub fn checked_session(session: &SessionView) -> Result<Value, ApiFailure> {
        let mut result = Self::session(session);
        result["opened_at"] = json!(super::projection::unix_time(session.opened_at)?);
        result["last_active_at"] = json!(super::projection::unix_time(session.last_active_at)?);
        Ok(result)
    }

    pub fn checked_revocation(value: &SessionRevocation) -> Result<Value, ApiFailure> {
        Ok(json!({"revoked_session_ids": value.revoked_session_ids,
            "revoked_at": super::projection::unix_time(value.revoked_at)?}))
    }

    pub fn checked_registration_challenge(
        value: &RegistrationChallenge,
    ) -> Result<Value, ApiFailure> {
        Ok(
            json!({"registration_id":value.registration_id,"ceremony":value.ceremony,
            "expires_at":super::projection::unix_time(value.expires_at)?}),
        )
    }

    pub fn checked_assertion_challenge(value: &AssertionChallenge) -> Result<Value, ApiFailure> {
        Ok(
            json!({"assertion_id":value.assertion_id,"ceremony":value.ceremony,
            "expires_at":super::projection::unix_time(value.expires_at)?}),
        )
    }

    pub fn checked_assertion(value: &AssertionProof) -> Result<Value, ApiFailure> {
        Ok(
            json!({"assertion_id":value.assertion_id,"passkey_id":value.passkey_id,
            "completed_at":super::projection::unix_time(value.completed_at)?,
            "expires_at":super::projection::unix_time(value.expires_at)?}),
        )
    }

    pub fn checked_step_up_challenge(value: &StepUpChallenge) -> Result<Value, ApiFailure> {
        crate::auth::validate_step_up_challenge_id(&value.challenge_id)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        super::identity_dispatch::step_up_challenge(value)
            .map_err(|_| ApiFailure::upstream_degraded())
    }

    pub fn checked_step_up(value: &StepUpEvidence) -> Result<Value, ApiFailure> {
        crate::auth::validate_step_up_challenge_id(value.challenge_id())
            .map_err(|_| ApiFailure::upstream_degraded())?;
        super::identity_dispatch::step_up_evidence(value)
            .map_err(|_| ApiFailure::upstream_degraded())
    }

    pub fn onboarding(
        scope: &PrincipalScope<'_>,
        value: &OnboardingStatus,
    ) -> Result<Value, ApiFailure> {
        super::identity_dispatch::onboarding_status(scope, value)
            .map_err(|_| ApiFailure::upstream_degraded())
    }

    pub fn authenticator_setup(
        value: &crate::security::AuthenticatorSetupChallenge,
    ) -> Result<Value, ApiFailure> {
        Ok(
            json!({"setup_id":value.setup_id,"secret":Self::timed_secret(&value.secret)?,
            "otpauth_uri":Self::timed_secret(&value.otpauth_uri)?,"expires_at":super::projection::unix_time(value.expires_at)?}),
        )
    }

    pub fn backup_codes(value: &crate::security::BackupCodeSet) -> Result<Value, ApiFailure> {
        Ok(
            json!({"codes":value.expose(),"remask_at":super::projection::unix_time(value.remask_at())?,"copyable":true}),
        )
    }

    pub fn key_export_challenge(
        value: &crate::security::KeyExportChallenge,
    ) -> Result<Value, ApiFailure> {
        Ok(
            json!({"export_id":value.export_id,"confirms":value.confirms.to_schema(),
            "expires_at":super::projection::unix_time(value.expires_at)?}),
        )
    }

    pub fn exported_key(value: &crate::security::ExportedKey) -> Result<Value, ApiFailure> {
        Ok(
            json!({"public_key":value.public_key,"secret":Self::timed_secret(&value.secret)?,
            "self_custodied_at":super::projection::unix_time(value.self_custodied_at)?}),
        )
    }

    pub fn authenticator_method(
        value: &crate::security::AuthenticatorMethod,
    ) -> Result<Value, ApiFailure> {
        let mut result = json!({"authenticator_id":value.id,"label":value.label,
            "enabled_at":super::projection::unix_time(value.enabled_at)?});
        if let Some(time) = value.last_used_at {
            result["last_used_at"] = json!(super::projection::unix_time(time)?);
        }
        Ok(result)
    }

    pub fn timed_secret(value: &crate::security::TimedSecret) -> Result<Value, ApiFailure> {
        Ok(
            json!({"value":value.expose(),"remask_at":super::projection::unix_time(value.remask_at())?,"copyable":value.copyable()}),
        )
    }

    /// Mints the service-owned device identifier from the schema-decoded
    /// session.open presentation metadata.
    ///
    /// # Errors
    ///
    /// Fails closed when an older client omits metadata or when either field
    /// violates the durable device bounds.
    pub fn mint_session_device(body: &Value) -> Result<Device, ApiFailure> {
        let device = body
            .get("device")
            .and_then(Value::as_object)
            .filter(|device| {
                device.len() == 2 && device.contains_key("label") && device.contains_key("platform")
            })
            .ok_or_else(|| ApiFailure::invalid_request(Some("device")))?;
        let label = device
            .get("label")
            .and_then(Value::as_str)
            .ok_or_else(|| ApiFailure::invalid_request(Some("device.label")))?;
        let platform = device
            .get("platform")
            .and_then(Value::as_str)
            .ok_or_else(|| ApiFailure::invalid_request(Some("device.platform")))?;
        Device::mint(label, platform).map_err(|_| ApiFailure::invalid_request(Some("device")))
    }

    #[must_use]
    pub fn registration_challenge(challenge: &RegistrationChallenge) -> Value {
        json!({
            "registration_id": challenge.registration_id,
            "ceremony": challenge.ceremony,
            "expires_at": rfc3339(challenge.expires_at)
        })
    }

    #[must_use]
    pub fn assertion_challenge(challenge: &AssertionChallenge) -> Value {
        json!({
            "assertion_id": challenge.assertion_id,
            "ceremony": challenge.ceremony,
            "expires_at": rfc3339(challenge.expires_at)
        })
    }

    #[must_use]
    pub fn assertion(proof: &AssertionProof) -> Value {
        json!({
            "assertion_id": proof.assertion_id,
            "passkey_id": proof.passkey_id,
            "completed_at": rfc3339(proof.completed_at),
            "expires_at": rfc3339(proof.expires_at)
        })
    }

    #[must_use]
    pub fn passkey(passkey: &PasskeyRecord) -> Value {
        let mut object = serde_json::Map::new();
        object.insert(
            "passkey_id".to_owned(),
            Value::String(passkey.passkey_id().to_owned()),
        );
        object.insert(
            "label".to_owned(),
            Value::String(passkey.label().to_owned()),
        );
        object.insert(
            "created_at".to_owned(),
            Value::String(rfc3339(passkey.created_at())),
        );
        if let Some(last_used_at) = passkey.last_used_at() {
            object.insert(
                "last_used_at".to_owned(),
                Value::String(rfc3339(last_used_at)),
            );
        }
        Value::Object(object)
    }

    #[must_use]
    pub fn passkeys(passkeys: &[PasskeyRecord]) -> Value {
        json!({
            "passkeys": passkeys.iter().map(Self::passkey).collect::<Vec<_>>()
        })
    }

    #[must_use]
    pub fn session(session: &SessionView) -> Value {
        json!({
            "session_id": session.session_id,
            "device": {
                "device_id": session.device.device_id(),
                "label": session.device.label(),
                "platform": session.device.platform()
            },
            "opened_at": rfc3339(session.opened_at),
            "last_active_at": rfc3339(session.last_active_at),
            "current": session.current
        })
    }

    #[must_use]
    pub fn sessions(sessions: &[SessionView]) -> Value {
        json!({
            "sessions": sessions.iter().map(Self::session).collect::<Vec<_>>()
        })
    }

    #[must_use]
    pub fn revocation(revocation: &SessionRevocation) -> Value {
        json!({
            "revoked_session_ids": revocation.revoked_session_ids,
            "revoked_at": rfc3339(revocation.revoked_at)
        })
    }

    #[must_use]
    pub fn operation_digest(digest: OperationDigest) -> Value {
        json!({ "confirms": digest.to_schema() })
    }

    #[must_use]
    pub fn step_up_challenge(challenge: &StepUpChallenge) -> Value {
        challenge.to_schema_json()
    }

    #[must_use]
    pub fn step_up(evidence: &StepUpEvidence) -> Value {
        evidence.to_schema_json()
    }

    /// Projects one freshly issued or rotated session and separates its
    /// secrets from the JSON result so the HTTP boundary can emit them only as
    /// protected cookies.
    ///
    /// # Errors
    ///
    /// Refuses a grant whose access or refresh lifetime already elapsed.
    pub fn session_grant(grant: &SessionGrant, now: u64) -> Result<BackendResponse, ApiFailure> {
        let access_max_age_seconds = grant
            .access_expires_at()
            .checked_sub(now)
            .filter(|lifetime| *lifetime > 0)
            .ok_or_else(ApiFailure::unavailable)?;
        let refresh_max_age_seconds = grant
            .refresh_expires_at()
            .checked_sub(now)
            .filter(|lifetime| *lifetime > 0)
            .ok_or_else(ApiFailure::unavailable)?;
        Ok(BackendResponse {
            result: Self::session(&grant.session()),
            session: Some(SessionSecrets {
                access_token: grant.access_token().expose().to_owned(),
                refresh_token: grant.refresh_token().expose().to_owned(),
                csrf_token: grant.csrf_token().expose().to_owned(),
                access_max_age_seconds,
                refresh_max_age_seconds,
            }),
        })
    }
}
