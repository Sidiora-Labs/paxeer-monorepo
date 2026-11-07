use super::{McpInvocationError, TenantId};

pub(super) const MCP_INVOCATION_MAX_RESPONSE: usize = 4 * 1024 * 1024;

#[derive(Clone, Copy, Debug, Eq, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "snake_case")]
pub(super) enum McpInvocationPhase {
    Begun,
    EffectStarted,
    Settled,
}

#[derive(serde::Serialize, serde::Deserialize)]
#[serde(deny_unknown_fields)]
pub(super) struct McpInvocationEvent {
    pub(super) order: u64,
    pub(super) kind: String,
    pub(super) current_generation: u64,
    pub(super) core_sequence: u64,
}

#[derive(serde::Serialize, serde::Deserialize)]
#[serde(deny_unknown_fields)]
pub(super) struct McpInvocationRecord {
    pub(super) schema: String,
    #[serde(default)]
    pub(super) tool_name: Option<String>,
    #[serde(default)]
    pub(super) native_preparation: Option<McpNativeAssociation>,
    #[serde(default)]
    pub(super) reconciled_receipt: Option<McpReceiptAssociation>,
    pub(super) tenant: String,
    pub(super) actor: Vec<u8>,
    pub(super) session: [u8; 32],
    pub(super) generation: u64,
    pub(super) idempotency_key: [u8; 32],
    pub(super) request_digest: [u8; 32],
    pub(super) core_sequence: u64,
    pub(super) phase: McpInvocationPhase,
    pub(super) response: Option<Vec<u8>>,
    pub(super) events: Vec<McpInvocationEvent>,
}

impl McpInvocationRecord {
    pub(super) fn decode(bytes: &[u8]) -> Result<Self, McpInvocationError> {
        use sha2::{Digest, Sha256};
        let mut record: Self =
            serde_json::from_slice(bytes).map_err(|_| McpInvocationError::Corrupt)?;
        if let Some(receipt) = record.reconciled_receipt.as_mut() {
            use layerx_types::verify::VerificationLevel;
            let level = match receipt.verification_rank {
                4 | 5 => VerificationLevel::COMMITTED,
                rank => VerificationLevel::from_wire_rank(rank)
                    .map_err(|_| McpInvocationError::Corrupt)?,
            };
            if level < VerificationLevel::STATE_PROVEN {
                return Err(McpInvocationError::Corrupt);
            }
            receipt.verification_rank = level.wire_rank();
        }
        if record.schema != "layerx.mcp.invocation.v1"
            || record.generation == 0
            || record.actor.is_empty()
            || record.actor.len() > 4096
            || record.events.is_empty()
            || record.session == [0; 32]
            || record.idempotency_key == [0; 32]
            || record.request_digest == [0; 32]
            || TenantId::new(record.tenant.clone()).is_err()
            || record
                .tool_name
                .as_ref()
                .is_some_and(|name| name.is_empty() || name.len() > 255)
            || record.native_preparation.as_ref().is_some_and(|native| {
                native.preparation_id == [0; 32] || native.idempotency_key != record.idempotency_key
            })
            || record.reconciled_receipt.as_ref().is_some_and(|receipt| {
                record.native_preparation.is_none()
                    || record.phase != McpInvocationPhase::Settled
                    || receipt.activity_id == [0; 32]
                    || receipt.receipt_bytes.is_empty()
                    || receipt.receipt_bytes.len() > MCP_INVOCATION_MAX_RESPONSE
                    || <[u8; 32]>::from(Sha256::digest(&receipt.receipt_bytes))
                        != receipt.receipt_digest
            })
            || record.events.iter().any(|event| {
                event.order == 0
                    || event.current_generation == 0
                    || !matches!(
                        event.kind.as_str(),
                        "begin"
                            | "effect_start"
                            | "effect_settled"
                            | "effect_unknown"
                            | "effect_refused"
                            | "release_refused"
                            | "release_generation_readback"
                            | "response_write_flush_ok"
                            | "response_write_flush_failed"
                            | "session_close"
                            | "session_revoke"
                            | "session_narrow"
                            | "session_refresh"
                            | "native_association"
                            | "native_transmission_started"
                            | "native_receipt_reconciled"
                    )
            })
            || (record.phase == McpInvocationPhase::Settled) != record.response.is_some()
            || record
                .response
                .as_ref()
                .is_some_and(|bytes| bytes.len() > MCP_INVOCATION_MAX_RESPONSE)
            || record
                .events
                .windows(2)
                .any(|pair| pair[0].order >= pair[1].order)
        {
            return Err(McpInvocationError::Corrupt);
        }
        Ok(record)
    }
}
#[derive(Clone, Debug, Eq, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(deny_unknown_fields)]
pub(super) struct McpNativeAssociation {
    pub(super) preparation_id: [u8; 32],
    pub(super) idempotency_key: [u8; 32],
}

#[derive(serde::Serialize, serde::Deserialize)]
#[serde(deny_unknown_fields)]
pub(super) struct McpReceiptAssociation {
    pub(super) activity_id: [u8; 32],
    pub(super) receipt_digest: [u8; 32],
    pub(super) receipt_bytes: Vec<u8>,
    pub(super) global_sequence: u64,
    pub(super) verification_rank: u8,
    pub(super) result_code: i32,
}

impl McpInvocationRecord {
    pub(super) fn append_event(
        &mut self,
        order: u64,
        kind: &str,
        current_generation: u64,
        core_sequence: u64,
    ) {
        self.events.push(McpInvocationEvent {
            order,
            kind: kind.to_owned(),
            current_generation,
            core_sequence,
        });
    }

    pub(super) fn encode(&self) -> Result<Vec<u8>, McpInvocationError> {
        serde_json::to_vec(self).map_err(|_| McpInvocationError::Unavailable)
    }
}
