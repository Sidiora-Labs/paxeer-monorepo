use crate::result_assertion;

use layerx_agentd::enrolment::binding_shape::{
    AgentDeclaration, BindingDocument, LimitDeclaration,
};
use serde_json::{json, Value};

/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
#[must_use]
pub fn document() -> Value {
    result_assertion::required(
        serde_json::to_value(BindingDocument {
            mode: "full".to_owned(),
            tenant: "tenant-a".to_owned(),
            store: "/tmp/web-store".to_owned(),
            audit_root: "/tmp/web-audit".to_owned(),
            session_id: "07".repeat(32),
            session_token_file: "/tmp/web-session-token".to_owned(),
            session_generation: 1,
            capability_id: "09".repeat(32),
            core_height: 7,
            deadline_ms: 5000,
            agent: AgentDeclaration {
                endpoint: "127.0.0.1:9500".to_owned(),
                bearer_file: "/tmp/web-agent-bearer".to_owned(),
                probe_program: "55".repeat(32),
                readiness: None,
            },
            limit: LimitDeclaration {
                id: "09".repeat(16),
                name: "mcp-limit".to_owned(),
                scope: "tenant".to_owned(),
                scope_id: "01".repeat(32),
                ceiling: "1000".to_owned(),
                consumed: "0".to_owned(),
            },
            listener: None,
            web: Some(json!({
                "endpoint": "http://127.0.0.1:9450",
                "payer_address": format!("0x{}", "ab".repeat(20)),
                "receiver_address": format!("0x{}", "cd".repeat(20)),
                "network": "eip155:125",
                "receipt_key_index": [{"keyId": "2a".repeat(32), "publicKey": "5e".repeat(32), "firstHeight": 1, "lastHeight": 100}],
                "sequencer_id": "2a".repeat(32),
                "first_batch_number": 1,
                "last_batch_number": 100,
                "timeout_ms": 5000,
                "pending_attempts": 3,
                "approval_threshold": "1000",
            })),
        }),
        "serialize the daemon publisher's current binding shape",
    )
}
