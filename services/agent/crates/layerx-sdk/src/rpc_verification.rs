use layerx_types::clock::Deadline;
use std::time::Duration;

use layerx_proof::inclusion::{verify_receipt, SequencerAuthorization};
use layerx_proof::merkle::Proof;
use layerx_wire::receipt::{decode_batch_header, Receipt};
use serde_json::Value;

use crate::rpc::{Commitment, RpcClient, RpcError};

pub use layerx_agent_api::read::{ReadRequest, VerifiedRead};
pub use layerx_agent_api::subscription::ReceiptReference;

pub struct ReceiptPolicy {
    pub protocol_version: u16,
    pub network_id: u32,
    pub sequencer: SequencerAuthorization,
    pub trusted_checkpoint_context_digest: Option<[u8; 32]>,
}

pub struct VerifiedRpcReceipt {
    receipt: Receipt,
    commitment: Commitment,
    canonical: Vec<u8>,
    batch_evidence: Option<VerifiedBatchEvidence>,
    canonical_activity: Option<Vec<u8>>,
}

/// Exact inclusion material retained after SDK verification for daemon evidence ingress.
pub struct VerifiedBatchEvidence {
    proof: Proof,
    canonical_header: Vec<u8>,
    header_signature: [u8; 64],
}

impl VerifiedRpcReceipt {
    #[must_use]
    pub const fn receipt(&self) -> &Receipt {
        &self.receipt
    }
    #[must_use]
    pub const fn commitment(&self) -> Commitment {
        self.commitment
    }
    #[must_use]
    pub fn canonical_bytes(&self) -> &[u8] {
        &self.canonical
    }

    #[must_use]
    pub const fn batch_evidence(&self) -> Option<&VerifiedBatchEvidence> {
        self.batch_evidence.as_ref()
    }

    /// Returns the exact submitted signed activity when this receipt came from
    /// a wallet execution rather than an independent receipt lookup.
    #[must_use]
    pub fn canonical_activity(&self) -> Option<&[u8]> {
        self.canonical_activity.as_deref()
    }

    pub(crate) fn bind_canonical_activity(&mut self, canonical: Vec<u8>) {
        self.canonical_activity = Some(canonical);
    }
}

impl VerifiedBatchEvidence {
    #[must_use]
    pub const fn proof(&self) -> &Proof {
        &self.proof
    }

    #[must_use]
    pub fn canonical_header(&self) -> &[u8] {
        &self.canonical_header
    }

    #[must_use]
    pub const fn header_signature(&self) -> [u8; 64] {
        self.header_signature
    }
}

impl RpcClient {
    /// # Errors
    /// Returns pending on deadline, RPC errors, or an exact verification refusal.
    pub fn wait_for(
        &self,
        activity: [u8; 32],
        commitment: Commitment,
        policy: &ReceiptPolicy,
        timeout: Duration,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        self.wait_for_authority(activity, commitment, policy, timeout)
    }

    fn wait_for_authority(
        &self,
        activity: [u8; 32],
        commitment: Commitment,
        policy: &ReceiptPolicy,
        timeout: Duration,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        if commitment == Commitment::Finalised {
            return Err(RpcError::MissingFinalityTrust);
        }
        if activity == [0; 32] || timeout > Duration::from_secs(300) {
            return Err(RpcError::InvalidRequest);
        }
        let id = super::rpc::encode_hex(&activity);
        if timeout.is_zero() {
            return Err(RpcError::Pending {
                activity_id: activity,
            });
        }
        let clock = self.clock()?;
        let mut deadline = Deadline::start(clock.as_ref(), timeout).map_err(RpcError::Clock)?;
        loop {
            if deadline
                .remaining(clock.as_ref())
                .map_err(RpcError::Clock)?
                .is_zero()
            {
                return Err(RpcError::Pending {
                    activity_id: activity,
                });
            }
            match self.receipt_at_commitment(&id, activity, commitment, policy, &mut deadline) {
                Ok(Some(receipt)) => return Ok(receipt),
                Ok(None)
                | Err(RpcError::Remote {
                    code: -32001 | -32005,
                    ..
                }) => {}
                Err(error) => return Err(error),
            }
            if deadline
                .remaining(clock.as_ref())
                .map_err(RpcError::Clock)?
                .is_zero()
            {
                return Err(RpcError::Pending {
                    activity_id: activity,
                });
            }
            std::thread::sleep(
                Duration::from_millis(50).min(
                    deadline
                        .remaining(clock.as_ref())
                        .map_err(RpcError::Clock)?,
                ),
            );
        }
    }

    fn read_before_deadline(
        &self,
        method: &str,
        params: &Value,
        deadline: &mut Deadline,
    ) -> Result<Option<Value>, RpcError> {
        let clock = self.clock()?;
        let remaining = deadline
            .remaining(clock.as_ref())
            .map_err(RpcError::Clock)?;
        if remaining.is_zero() {
            return Ok(None);
        }
        let result = self.call_bounded(method, params, remaining);
        if deadline
            .remaining(clock.as_ref())
            .map_err(RpcError::Clock)?
            .is_zero()
        {
            return Ok(None);
        }
        result.map(Some)
    }

    fn receipt_at_commitment(
        &self,
        id: &str,
        activity: [u8; 32],
        commitment: Commitment,
        policy: &ReceiptPolicy,
        deadline: &mut Deadline,
    ) -> Result<Option<VerifiedRpcReceipt>, RpcError> {
        let clock = self.clock()?;
        let Some(result) =
            self.read_before_deadline("lx_getReceipt", &serde_json::json!([id]), deadline)?
        else {
            return Ok(None);
        };
        if matches!(
            result.get("state").and_then(Value::as_str),
            Some("pending" | "unknown")
        ) {
            return Ok(None);
        }
        if result.get("activity_id").and_then(Value::as_str) != Some(id) {
            return Err(RpcError::InvalidResponse);
        }
        let canonical = hex_field(&result, "receipt", 1_048_576)?;
        let receipt = layerx_proof::receipt::verify_sequencer_signature(
            &canonical,
            policy.sequencer.public_key(),
        )
        .map_err(|_| RpcError::Verification)?;
        let protocol = receipt.protocol().ok_or(RpcError::Verification)?;
        if protocol.activity_id() != activity
            || protocol.protocol_version() != policy.protocol_version
        {
            return Err(RpcError::Verification);
        }
        let batch_evidence = if commitment == Commitment::Executed {
            None
        } else {
            if deadline
                .remaining(clock.as_ref())
                .map_err(RpcError::Clock)?
                .is_zero()
            {
                return Ok(None);
            }
            let Some(proof) = self.read_before_deadline(
                "lx_getProof",
                &serde_json::json!(["receipt", id]),
                deadline,
            )?
            else {
                return Ok(None);
            };
            let evidence = verify_rpc_inclusion(&proof, id, &canonical, policy)?;
            Some(evidence)
        };
        if deadline
            .remaining(clock.as_ref())
            .map_err(RpcError::Clock)?
            .is_zero()
        {
            return Ok(None);
        }
        Ok(Some(VerifiedRpcReceipt {
            receipt,
            commitment,
            canonical,
            batch_evidence,
            canonical_activity: None,
        }))
    }
}

fn verify_rpc_inclusion(
    value: &Value,
    id: &str,
    receipt: &[u8],
    policy: &ReceiptPolicy,
) -> Result<VerifiedBatchEvidence, RpcError> {
    if value["kind"] != "receipt"
        || value["activity_id"] != id
        || hex_field(value, "canonical_value", 1_048_576)? != receipt
    {
        return Err(RpcError::Verification);
    }
    let signed = &value["signed_header"];
    let header = hex_field(signed, "canonical_header", 65536)?;
    let signature: [u8; 64] = hex_field(signed, "signature", 64)?
        .try_into()
        .map_err(|_| RpcError::InvalidResponse)?;
    let decoded = decode_batch_header(&header).map_err(|_| RpcError::Verification)?;
    if decoded.protocol_version() != policy.protocol_version
        || decoded.network_id() != policy.network_id
    {
        return Err(RpcError::Verification);
    }
    let path = &value["proof"];
    let index = path["leaf_index"]
        .as_u64()
        .and_then(|n| u32::try_from(n).ok())
        .ok_or(RpcError::InvalidResponse)?;
    let count = path["leaf_count"]
        .as_u64()
        .and_then(|n| u32::try_from(n).ok())
        .ok_or(RpcError::InvalidResponse)?;
    let siblings = path["siblings"]
        .as_array()
        .filter(|v| v.len() <= 32)
        .ok_or(RpcError::InvalidResponse)?;
    let siblings = siblings
        .iter()
        .map(|s| {
            decode_hex(s.as_str().ok_or(RpcError::InvalidResponse)?, 32)?
                .try_into()
                .map_err(|_| RpcError::InvalidResponse)
        })
        .collect::<Result<Vec<[u8; 32]>, RpcError>>()?;
    let proof = Proof::new(index, count, siblings).map_err(|_| RpcError::Verification)?;
    verify_receipt(receipt, &proof, &header, &signature, &policy.sequencer)
        .map_err(|_| RpcError::Verification)?;
    Ok(VerifiedBatchEvidence {
        proof,
        canonical_header: header,
        header_signature: signature,
    })
}

fn hex_field(value: &Value, name: &str, maximum: usize) -> Result<Vec<u8>, RpcError> {
    decode_hex(
        value
            .get(name)
            .and_then(Value::as_str)
            .ok_or(RpcError::InvalidResponse)?,
        maximum,
    )
}
fn decode_hex(value: &str, maximum: usize) -> Result<Vec<u8>, RpcError> {
    if value.is_empty()
        || value.len() > maximum * 2
        || !value.len().is_multiple_of(2)
        || !value.bytes().all(|b| b.is_ascii_hexdigit())
    {
        return Err(RpcError::InvalidResponse);
    }
    value
        .as_bytes()
        .chunks_exact(2)
        .map(|b| {
            std::str::from_utf8(b)
                .ok()
                .and_then(|s| u8::from_str_radix(s, 16).ok())
                .ok_or(RpcError::InvalidResponse)
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;
    #[test]
    fn acknowledgement_is_not_inclusion() {
        let policy = ReceiptPolicy {
            protocol_version: 3,
            network_id: 17,
            sequencer: SequencerAuthorization::new([1; 32], [2; 32], 1, 10),
            trusted_checkpoint_context_digest: None,
        };
        assert!(verify_rpc_inclusion(&json!({"state":"accepted"}), "01", &[1], &policy).is_err());
    }
}
