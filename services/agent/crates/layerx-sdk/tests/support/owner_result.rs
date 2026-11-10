use layerx_agent_api::error::RequestId;
use layerx_agent_api::verify::{ApiSuccess, VerificationStatus};
use layerx_sdk::agent_envelope::EnvelopeError;
use serde_json::{json, Value};
type Outcome = Result<ApiSuccess<Value>, EnvelopeError>;
fn verification_json(status: &VerificationStatus) -> Value {
    match status {
        VerificationStatus::Achieved(rung) => {
            json!({"state": "achieved", "rung": rung.name()})
        }
        VerificationStatus::Below {
            requested,
            achieved,
            reason,
        } => json!({
            "state": "unverified",
            "requested": requested.name(),
            "achieved": achieved.name(),
            "reason": reason.as_str(),
        }),
    }
}

pub(crate) fn outcome_json(outcome: &Outcome) -> Value {
    match outcome {
        Ok(success) => json!({
            "result": "success",
            "request_id": success.request_id.0.to_string(),
            "value": success.value,
            "verification_status": verification_json(&success.verification_status),
        }),
        Err(EnvelopeError::Refused(error)) => json!({
            "result": "error",
            "class": format!("{:?}", error.class),
            "protocol_result_code": error.protocol_result_code.map(layerx_types::result::ResultCode::raw),
            "retriability": format!("{:?}", error.retriability),
            "request_id": error.request_id.0.to_string(),
            "reason": error.reason.as_str(),
        }),
        Err(EnvelopeError::Unknown { operation }) => {
            json!({"result": "unknown", "operation": operation.name()})
        }
        Err(other) => json!({"result": "client_refusal", "error": format!("{other:?}")}),
    }
}

pub(crate) fn assert_exact_owner_result(
    outcome: &Outcome,
    request_id: RequestId,
    expected: &Value,
) -> Result<(), String> {
    if !expected.is_object() || !matches!(expected["result"].as_str(), Some("success" | "error")) {
        return Err("provisioned owner result must be an exact success or error record".into());
    }
    let mut expected = expected.clone();
    expected["request_id"] = json!(request_id.0.to_string());
    let actual = outcome_json(outcome);
    if actual != expected {
        return Err(format!(
            "owner result mismatch: expected {expected}, got {actual}"
        ));
    }
    Ok(())
}
