//! One schema request and owner for canonicalization and checkpoint routing.
use super::{CheckpointReadError, CommitValidator, ServedCheckpoint};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_types::rung::Rung;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct CheckpointRequest {
    #[serde(default, skip_serializing)]
    pub tenant: Option<String>,
    #[serde(default, skip_serializing)]
    pub agent: Option<String>,
    pub checkpoint: String,
    #[serde(alias = "requested_rung")]
    pub requested_verification_level: String,
}
impl CheckpointRequest {
    /// # Errors
    /// Returns an error if node acquisition fails or the committed checkpoint, target, result
    /// root or proof cannot be verified.
    pub fn height(&self) -> Result<u64, CheckpointReadError> {
        let height = self
            .checkpoint
            .parse::<u64>()
            .map_err(|_| CheckpointReadError::TargetMismatch)?;
        if height == 0
            || height.to_string() != self.checkpoint
            || Rung::from_name(&self.requested_verification_level).is_none()
        {
            return Err(CheckpointReadError::TargetMismatch);
        }
        Ok(height)
    }
    /// # Errors
    /// Returns an error if node acquisition fails or the committed checkpoint, target, result
    /// root or proof cannot be verified.
    pub fn acquire(
        &self,
        endpoint: &SequencerEndpoint,
        chain_id: &str,
        validators: &[CommitValidator],
    ) -> Result<Value, CheckpointReadError> {
        let height = self.height()?;
        let checkpoint = ServedCheckpoint::acquire_at_committed_block(
            endpoint,
            height,
            Some(
                endpoint
                    .block_by_number(height)
                    .map_err(CheckpointReadError::Rpc)?
                    .ok_or(CheckpointReadError::NotCommitted)?
                    .hash,
            ),
            chain_id,
        )?;
        if checkpoint.validators != validators {
            return Err(CheckpointReadError::Commit);
        }
        let proof = checkpoint.commitment_proof()?;
        let hex = layerx_programs::hex::encode;
        Ok(
            json!({"value": {"commit": hex(&proof.commit), "validatorSet": hex(&proof.validator_set),
            "header": hex(&proof.signed_header), "height": height.to_string()},
            "achieved_verification_level": "committed",
            "freshness": {"chain_head": height.to_string(), "value_height": height.to_string(), "relative_to": height.to_string()}}),
        )
    }
}
