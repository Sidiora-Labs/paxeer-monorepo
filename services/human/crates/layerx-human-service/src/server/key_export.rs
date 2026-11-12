//! Durable key export handlers shared by production component compositions.

use serde_json::{json, Value};

use crate::auth::Passkeys;
use crate::custody::{CustodyError, KeyId, Keystore};
use crate::security::{KeyExportCeremony, SecurityError};
use crate::store::{PrincipalScope, PrincipalStore};
use crate::trace::TraceId;

use super::backend::{ApiFailure, BackendResponse, Readiness};
use super::privileged::{AuthorizedSession, ComponentOperationRequest, PrivilegedHumanServices};

const WINDOW_SECONDS: u64 = 300;

fn ceremony() -> Result<KeyExportCeremony, ApiFailure> {
    KeyExportCeremony::new(
        KeyId::new("human-primary").map_err(|_| ApiFailure::unavailable())?,
        WINDOW_SECONDS,
    )
    .map_err(|error| failure(&error))
}

pub(crate) fn begin(
    keystore: &Keystore,
    passkeys: &Passkeys,
    scope: &mut PrincipalScope<'_>,
    body: &Value,
    now: u64,
) -> Result<BackendResponse, ApiFailure> {
    let evidence = evidence(passkeys, scope, body, now)?;
    let expected = crate::security::security_action_digest(
        scope.principal(),
        crate::security::SecurityAction::ExportPrimaryKey,
        None,
    )
    .map_err(|error| failure(&error))?;
    if evidence.completed_at() > now || evidence.expires_at() <= now {
        return Err(ApiFailure::forbidden());
    }
    Passkeys::validate_step_up(scope, &evidence, expected, now)
        .map_err(|error| super::privileged::map_auth_error(&error))?;
    let challenge = ceremony()?
        .begin(keystore, scope.principal(), now)
        .map_err(|error| failure(&error))?;
    Ok(BackendResponse {
        result: super::IdentityProjector::key_export_challenge(&challenge)?,
        session: None,
    })
}

pub(crate) fn finish(
    passkeys: &Passkeys,
    keystore: &Keystore,
    scope: &mut PrincipalScope<'_>,
    body: &Value,
    trace: &str,
    now: u64,
) -> Result<BackendResponse, ApiFailure> {
    let export_id = text(body, "export_id")?;
    let evidence = evidence(passkeys, scope, body, now)?;
    let trace = TraceId::parse(trace).map_err(|_| ApiFailure::invalid_request(None))?;
    let exported = ceremony()?
        .finish(keystore, scope, export_id, &evidence, &trace, now)
        .map_err(|error| failure(&error))?;
    Ok(BackendResponse {
        result: super::IdentityProjector::exported_key(&exported)?,
        session: None,
    })
}

/// Completes an explicit delivery acknowledgement after the schema owner wires
/// its operation. The proof is derived from the delivered seed, never the seed itself.
/// # Errors
/// Returns an error if validation or a required operation fails.
pub fn acknowledge(
    passkeys: &Passkeys,
    keystore: &Keystore,
    scope: &mut PrincipalScope<'_>,
    body: &Value,
    trace: &str,
    now: u64,
) -> Result<BackendResponse, ApiFailure> {
    let export_id = text(body, "export_id")?;
    let token = crate::auth::OperationDigest::parse_schema(&format!(
        "opd_{}",
        text(body, "delivery_token")?
    ))
    .map_err(|_| ApiFailure::invalid_request(Some("delivery_token")))?
    .bytes();
    let evidence = evidence(passkeys, scope, body, now)?;
    let trace = TraceId::parse(trace).map_err(|_| ApiFailure::invalid_request(None))?;
    ceremony()?
        .acknowledge(keystore, scope, export_id, token, &evidence, &trace, now)
        .map_err(|error| failure(&error))?;
    Ok(BackendResponse {
        result: json!({}),
        session: None,
    })
}

pub(crate) fn step_up_begin(
    passkeys: &Passkeys,
    scope: &mut PrincipalScope<'_>,
    body: &Value,
    now: u64,
) -> Result<crate::auth::StepUpChallenge, ApiFailure> {
    let digest = crate::auth::OperationDigest::parse_schema(text(body, "confirms")?)
        .map_err(|_| ApiFailure::invalid_request(Some("confirms")))?;
    passkeys
        .begin_step_up_authorized(scope, digest, now)
        .map_err(|error| super::privileged::map_auth_error(&error))
}

fn evidence(
    passkeys: &Passkeys,
    scope: &mut PrincipalScope<'_>,
    body: &Value,
    now: u64,
) -> Result<crate::auth::StepUpEvidence, ApiFailure> {
    let challenge = body
        .get("step_up")
        .and_then(|value| value.get("challenge_id"))
        .and_then(Value::as_str)
        .ok_or_else(ApiFailure::forbidden)?;
    passkeys
        .load_step_up_evidence(scope, challenge, now)
        .map_err(|error| super::privileged::map_auth_error(&error))
}

fn text<'a>(body: &'a Value, field: &str) -> Result<&'a str, ApiFailure> {
    body.get(field)
        .and_then(Value::as_str)
        .ok_or_else(|| ApiFailure::invalid_request(Some(field)))
}

fn failure(error: &SecurityError) -> ApiFailure {
    match error {
        SecurityError::KeyExported
        | SecurityError::StepUpMismatch
        | SecurityError::StepUpExpired
        | SecurityError::Custody(
            CustodyError::SelfCustodied
            | CustodyError::StepUpExpired
            | CustodyError::InvalidEvidence,
        ) => ApiFailure::forbidden(),
        SecurityError::InvalidTarget => ApiFailure::invalid_request(None),
        SecurityError::SessionExpired => ApiFailure::session_expired(),
        SecurityError::Auth(error) => super::privileged::map_auth_error(error),
        SecurityError::Boundary(_) | SecurityError::Custody(_) | SecurityError::Audit(_) => {
            ApiFailure::upstream_degraded()
        }
    }
}

/// Adds the production export handlers to a privileged service composition.
/// Session, CSRF and principal admission remain owned by `PrivilegedHumanComponents`.
pub struct KeyExportServices<S> {
    keystore: Keystore,
    services: S,
}

impl<S> KeyExportServices<S> {
    #[must_use]
    pub const fn new(keystore: Keystore, services: S) -> Self {
        Self { keystore, services }
    }
}

impl<S: PrivilegedHumanServices> PrivilegedHumanServices for KeyExportServices<S> {
    fn execute_public(
        &mut self,
        store: &mut PrincipalStore,
        passkeys: &Passkeys,
        request: ComponentOperationRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        self.services.execute_public(store, passkeys, request, now)
    }

    fn execute_authorized(
        &mut self,
        scope: &mut PrincipalScope<'_>,
        passkeys: &Passkeys,
        session: AuthorizedSession<'_>,
        request: ComponentOperationRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        match request.operation.name.as_str() {
            "security.key-export.begin" => {
                begin(&self.keystore, passkeys, scope, &request.body, now)
            }
            "stepup.begin" => {
                let challenge = step_up_begin(passkeys, scope, &request.body, now)?;
                Ok(BackendResponse {
                    result: super::IdentityProjector::step_up_challenge(&challenge),
                    session: None,
                })
            }
            "stepup.finish" => {
                let challenge = request
                    .path_parameters
                    .get("challenge_id")
                    .ok_or_else(|| ApiFailure::invalid_request(Some("challenge_id")))?;
                let evidence = passkeys
                    .finish_step_up(scope, challenge, text(&request.body, "credential")?, now)
                    .map_err(|error| super::privileged::map_auth_error(&error))?;
                Ok(BackendResponse {
                    result: super::IdentityProjector::step_up(&evidence),
                    session: None,
                })
            }
            "security.key-export.finish" => finish(
                passkeys,
                &self.keystore,
                scope,
                &request.body,
                &request.trace,
                now,
            ),
            _ => self
                .services
                .execute_authorized(scope, passkeys, session, request, now),
        }
    }

    fn readiness(&mut self, store: &mut PrincipalStore, now: u64) -> Result<Readiness, ApiFailure> {
        self.services.readiness(store, now)
    }
}
