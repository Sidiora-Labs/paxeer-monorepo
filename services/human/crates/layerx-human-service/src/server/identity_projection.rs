//! API projection shared with the provider client's source-level interface check.
use super::IdentityDispatchError;
use crate::auth::{StepUpChallenge, StepUpEvidence};
use crate::onboarding::{
    EvidenceClass, EvidenceVerification, OnboardingStage, OnboardingState, OnboardingStatus,
    StageState,
};
use crate::store::{PrincipalScope, RowKey, Table};
use serde_json::{json, Value};
use sha2::Digest as _;

// Keep the provider client source reusable without a parent server module.
// Both contexts use the same checked UTC conversion implementation.
#[path = "../time.rs"]
mod api_time;
fn corrupt() -> IdentityDispatchError {
    IdentityDispatchError::Corrupt
}
fn not_found() -> IdentityDispatchError {
    IdentityDispatchError::NotFound
}
fn unix_time(seconds: u64) -> Result<String, IdentityDispatchError> {
    let result = api_time::rfc3339(seconds);
    if api_time::seconds_from_rfc3339(&result) != Some(seconds) {
        return Err(corrupt());
    }
    Ok(result)
}
pub(crate) fn step_up_challenge(value: &StepUpChallenge) -> Result<Value, IdentityDispatchError> {
    Ok(
        json!({"challenge_id":value.challenge_id,"confirms":value.confirms.to_schema(),
        "ceremony":value.ceremony,"expires_at":unix_time(value.expires_at)?}),
    )
}
pub(crate) fn step_up_evidence(value: &StepUpEvidence) -> Result<Value, IdentityDispatchError> {
    Ok(
        json!({"challenge_id":value.challenge_id(),"confirms":value.confirms().to_schema(),
        "passkey_id":value.passkey_id(),"completed_at":unix_time(value.completed_at())?,
        "expires_at":unix_time(value.expires_at())?}),
    )
}
pub(crate) fn onboarding_status(
    scope: &PrincipalScope<'_>,
    value: &OnboardingStatus,
) -> Result<Value, IdentityDispatchError> {
    let row = scope
        .get(
            Table::Journeys,
            &RowKey::new("onboarding-journey").map_err(|_| corrupt())?,
        )
        .ok_or_else(not_found)?;
    let record: Value = serde_json::from_slice(row.bytes()).map_err(|_| corrupt())?;
    let action: [u8; 32] =
        serde_json::from_value(record["idempotency_key"].clone()).map_err(|_| corrupt())?;
    let mut all_evidence = Vec::new();
    let stages = value.stages().iter().map(|stage| {
        let name = match stage.stage() { OnboardingStage::ApplicationIdentity => "application-identity",
            OnboardingStage::CustodyKey => "custody-key", OnboardingStage::DidRegistration => "protocol-identity",
            OnboardingStage::InitialFunding => "initial-funding", OnboardingStage::RecoveryRegistration => "recovery" };
        let evidence = stage.evidence().iter().map(|item| {
            let stored = scope.get(Table::Journeys, item.row()).ok_or_else(corrupt)?;
            let digest = sha2::Sha256::digest(stored.bytes());
            let id = digest.iter().map(|byte| format!("{byte:02x}")).collect::<String>();
            Ok(json!({"evidence_id": format!("evd_{id}"),
                "class": match item.class() { EvidenceClass::LocalJourneyState | EvidenceClass::CustodyKey => "local-journey-state",
                    EvidenceClass::LayerxReceipt => "layerx-receipt", EvidenceClass::SubmissionRecord => "submission-record" },
                "verification": match item.verification() { EvidenceVerification::Unverified => "unverified",
                    EvidenceVerification::ReceiptVerified => "receipt-verified" }}))
        }).collect::<Result<Vec<_>, IdentityDispatchError>>()?;
        all_evidence.extend(evidence.iter().cloned());
        Ok(json!({"stage_id": format!("stg_{name}"), "copy_key": format!("onboarding.stage.{name}"),
            "state": stage_state(stage.state()), "evidence": evidence}))
    }).collect::<Result<Vec<_>, IdentityDispatchError>>()?;
    let state = match value.state() {
        OnboardingState::GettingReady | OnboardingState::Queued => "getting-ready",
        OnboardingState::Sending => "sending",
        OnboardingState::Processing | OnboardingState::ActiveRecoveryPending => "processing",
        OnboardingState::StillChecking => "still-checking",
        OnboardingState::Complete => "done",
        OnboardingState::Refused => "refused",
    };
    let time = |name| {
        record[name]
            .as_u64()
            .ok_or_else(corrupt)
            .and_then(unix_time)
    };
    let action = action
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<String>();
    Ok(
        json!({"journey_id": format!("jrn_{action}"), "kind":"onboarding", "state":state,
        "state_copy_key":format!("journey.state.{state}"), "stages":stages,"evidence":all_evidence,
        "started_at":time("started_at")?,"updated_at":time("updated_at")?}),
    )
}

fn stage_state(value: StageState) -> &'static str {
    match value {
        StageState::GettingReady => "getting-ready",
        StageState::LocalComplete => "processing",
        StageState::Queued { .. } => "getting-ready",
        StageState::Sending => "sending",
        StageState::Processing => "processing",
        StageState::StillChecking => "still-checking",
        StageState::ReceiptVerified => "done",
        StageState::Refused { .. } => "refused",
    }
}
