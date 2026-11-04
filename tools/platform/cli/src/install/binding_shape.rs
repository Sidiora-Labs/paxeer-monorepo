//! Persisted enrolment shape shared by the daemon publisher and MCP installer.

use serde::{Deserialize, Serialize};
use serde_json::Value;

pub const MAX_DOCUMENT_BYTES: usize = 65_536;

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct BindingDocument {
    pub mode: String,
    pub tenant: String,
    pub store: String,
    pub audit_root: String,
    pub session_id: String,
    pub session_token_file: String,
    pub session_generation: u64,
    pub capability_id: String,
    pub core_height: u64,
    pub deadline_ms: u64,
    pub agent: AgentDeclaration,
    pub limit: LimitDeclaration,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub listener: Option<Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub web: Option<Value>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct AgentDeclaration {
    pub endpoint: String,
    pub bearer_file: String,
    pub probe_program: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub readiness: Option<ReadinessTransport>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct ReadinessTransport {
    pub endpoint: String,
    pub gateway_key_file: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub trust_anchors: Option<String>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct LimitDeclaration {
    pub id: String,
    pub name: String,
    pub scope: String,
    pub scope_id: String,
    pub ceiling: String,
    pub consumed: String,
}
