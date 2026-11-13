#[path = "rpc_commit.rs"]
pub mod node_commit;

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

type CommittedBlock = (u64, [u8; 32]);

pub struct VerifiedRpcReceipt {
    receipt: Receipt,
    commitment: Commitment,
    canonical: Vec<u8>,
    batch_evidence: Option<VerifiedBatchEvidence>,
    canonical_activity: Option<Vec<u8>>,
    committed_block: Option<CommittedBlock>,
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
    pub const fn rung(&self) -> layerx_types::rung::Rung {
        if self.committed_block.is_some() {
            layerx_types::rung::Rung::Committed
        } else {
            layerx_types::rung::Rung::Receipted
        }
    }

    #[must_use]
    pub const fn committed_block(&self) -> Option<(u64, [u8; 32])> {
        self.committed_block
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

    pub(crate) fn submitted_activity(&self) -> Result<([u8; 32], &[u8]), RpcError> {
        let canonical = self.canonical_activity().ok_or(RpcError::InvalidRequest)?;
        let protocol = self.receipt().protocol().ok_or(RpcError::InvalidRequest)?;
        Ok((protocol.activity_id(), canonical))
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
        self.wait_for_authority(activity, commitment, policy, timeout, None)
    }

    /// Verifies the covering Comet commit and signed sequence batch using caller-pinned trust.
    /// # Errors
    ///
    /// Returns receipt lookup, timeout or verification errors if the receipt, covering commit or sequence batch cannot be authenticated with the supplied trust.
    pub fn wait_for_with_commit(
        &self,
        activity: [u8; 32],
        policy: &ReceiptPolicy,
        timeout: Duration,
        evidence: &layerx_client::rpc::receipt_commit::ReceiptCommitEvidence<'_>,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        self.wait_for_authority(
            activity,
            Commitment::Finalised,
            policy,
            timeout,
            Some(evidence),
        )
    }

    fn wait_for_authority(
        &self,
        activity: [u8; 32],
        commitment: Commitment,
        policy: &ReceiptPolicy,
        timeout: Duration,
        commit: Option<&layerx_client::rpc::receipt_commit::ReceiptCommitEvidence<'_>>,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        if activity == [0; 32] || timeout > Duration::from_secs(300) {
            return Err(RpcError::InvalidRequest);
        }
        let id = format!("0x{}", super::rpc::encode_hex(&activity));
        if timeout.is_zero() {
            return Err(RpcError::Pending {
                activity_id: activity,
            });
        }
        let clock = self.clock()?;
        let mut deadline = Deadline::start(clock.as_ref(), timeout).map_err(RpcError::Clock)?;
        let Some(chain) =
            self.read_before_deadline("eth_chainId", &serde_json::json!([]), &mut deadline)?
        else {
            return Err(RpcError::Pending {
                activity_id: activity,
            });
        };
        if node_quantity(&serde_json::json!({"chain": chain}), "chain")?
            != u64::from(policy.network_id)
        {
            return Err(RpcError::Verification);
        }
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
            match self.receipt_at_commitment(
                &id,
                activity,
                commitment,
                policy,
                &mut deadline,
                commit,
            ) {
                Ok(Some(receipt)) => return Ok(receipt),
                Ok(None)
                | Err(RpcError::Remote {
                    code: -32_001 | -32_005,
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

    fn committed_receipt_block(
        &self,
        result: &Value,
        receipt: &Receipt,
        policy: &ReceiptPolicy,
        deadline: &mut Deadline,
        commit: Option<&layerx_client::rpc::receipt_commit::ReceiptCommitEvidence<'_>>,
    ) -> Result<Option<CommittedBlock>, RpcError> {
        if result["rung"] != "committed" {
            return Ok(None);
        }
        let height = node_quantity(result, "blockNumber")?;
        let hash = node_fixed::<32>(result, "blockHash")?;
        let Some(material) = self.read_before_deadline(
            "px_getCommit",
            &serde_json::json!([height.to_string()]),
            deadline,
        )?
        else {
            return Ok(None);
        };
        let Some(block) = self.read_before_deadline(
            "eth_getBlockByNumber",
            &serde_json::json!([format!("0x{height:x}"), false]),
            deadline,
        )?
        else {
            return Ok(None);
        };
        if node_quantity(&block, "number")? != height || node_fixed::<32>(&block, "hash")? != hash {
            return Err(RpcError::Verification);
        }
        if let Some(evidence) = commit {
            verify_supplied_commit(&material, receipt, result, (height, hash), policy, evidence)?;
        } else {
            let (authority, coverage_override) = self
                .node_commit
                .as_ref()
                .ok_or(RpcError::MissingFinalityTrust)?;
            let served_coverage;
            let coverage = if coverage_override.is_override() {
                coverage_override
            } else {
                let Some(batch) = self
                    .read_before_deadline(
                        "px_getSequenceBatch",
                        &serde_json::json!([height.to_string()]),
                        deadline,
                    )
                    .map_err(|error| match error {
                        RpcError::Remote { code: -32_602, .. } => RpcError::Verification,
                        other => other,
                    })?
                else {
                    return Ok(None);
                };
                served_coverage = node_commit::NodeCommitCoverage::from_sequence_batch(
                    &batch,
                    height,
                    receipt.protocol().ok_or(RpcError::Verification)?,
                )?;
                &served_coverage
            };
            node_commit::verify_committed(
                &material["signedHeader"],
                receipt.protocol().ok_or(RpcError::Verification)?,
                node_quantity(result, "height")?,
                (height, hash),
                authority,
                coverage,
                &policy.sequencer,
            )?;
        }
        Ok(Some((height, hash)))
    }

    fn receipt_at_commitment(
        &self,
        id: &str,
        activity: [u8; 32],
        commitment: Commitment,
        policy: &ReceiptPolicy,
        deadline: &mut Deadline,
        commit: Option<&layerx_client::rpc::receipt_commit::ReceiptCommitEvidence<'_>>,
    ) -> Result<Option<VerifiedRpcReceipt>, RpcError> {
        let Some(result) =
            self.read_before_deadline("pax_sequencerReceipt", &serde_json::json!([id]), deadline)?
        else {
            return Ok(None);
        };
        if result.is_null() {
            return Ok(None);
        }
        if result.get("txHash").and_then(Value::as_str) != Some(id) {
            return Err(RpcError::InvalidResponse);
        }
        match result.get("rung").and_then(Value::as_str) {
            Some("submitted") => {
                let fields = result.as_object().ok_or(RpcError::InvalidResponse)?;
                if fields.len() != 3 || !fields.contains_key("nonce") {
                    return Err(RpcError::InvalidResponse);
                }
                node_quantity(&result, "nonce")?;
                return Ok(None);
            }
            Some("receipted" | "committed") => {}
            _ => return Err(RpcError::InvalidResponse),
        }
        let (receipt, canonical) = verify_node_receipt(&result, activity, policy)?;
        let batch_evidence = result
            .get("inclusion")
            .map(|inclusion| {
                verify_rpc_inclusion(
                    inclusion,
                    &super::rpc::encode_hex(&activity),
                    &canonical,
                    policy,
                )
            })
            .transpose()?;
        let committed_block = if commitment == Commitment::Executed {
            None
        } else {
            let Some(block) =
                self.committed_receipt_block(&result, &receipt, policy, deadline, commit)?
            else {
                return Ok(None);
            };
            Some(block)
        };
        Ok(Some(VerifiedRpcReceipt {
            receipt,
            commitment,
            canonical,
            batch_evidence,
            canonical_activity: None,
            committed_block,
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
    let header = hex_field(signed, "canonical_header", 65_536)?;
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

fn invalid_response(field: &str) -> RpcError {
    eprintln!("RpcError::InvalidResponse: refused field {field}");
    RpcError::InvalidResponse
}

fn node_quantity(value: &Value, name: &str) -> Result<u64, RpcError> {
    let text = value
        .get(name)
        .and_then(Value::as_str)
        .and_then(|s| s.strip_prefix("0x"))
        .ok_or_else(|| invalid_response(name))?;
    if text.is_empty()
        || (text.len() > 1 && text.starts_with('0'))
        || !text.bytes().all(|b| matches!(b, b'0'..=b'9' | b'a'..=b'f'))
    {
        return Err(invalid_response(name));
    }
    u64::from_str_radix(text, 16).map_err(|_| invalid_response(name))
}
fn node_bytes(value: &Value, name: &str, maximum: usize) -> Result<Vec<u8>, RpcError> {
    let text = value
        .get(name)
        .and_then(Value::as_str)
        .and_then(|s| s.strip_prefix("0x"))
        .ok_or_else(|| invalid_response(name))?;
    if text.len() % 2 != 0 || text.len() / 2 > maximum {
        return Err(invalid_response(name));
    }
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(
                std::str::from_utf8(pair).map_err(|_| invalid_response(name))?,
                16,
            )
            .map_err(|_| invalid_response(name))
        })
        .collect()
}
fn node_fixed<const N: usize>(value: &Value, name: &str) -> Result<[u8; N], RpcError> {
    node_bytes(value, name, N)?
        .try_into()
        .map_err(|_| invalid_response(name))
}

/// Authenticates the node summary and every served execution field under height-ranged pins.
/// The payment asset is bound to its denomination independently of the fee denomination.
/// # Errors
///
/// Returns `Verification` or `InvalidResponse` for malformed or inconsistent execution fields, an invalid canonical receipt, or a receipt outside the pinned signature authority.
pub fn verify_node_receipt(
    value: &Value,
    activity: [u8; 32],
    policy: &ReceiptPolicy,
) -> Result<(Receipt, Vec<u8>), RpcError> {
    use layerx_proof::sequencer_logs::verify_logs;
    use sha2::{Digest as _, Sha256};
    validate_node_receipt_fields(value)?;
    let canonical = node_bytes(value, "receipt", 1_048_576)?;
    let receipt = layerx_proof::receipt::verify_sequencer_signature(
        &canonical,
        layerx_proof::receipt::ReceiptKeys::HeightRanges(std::slice::from_ref(&policy.sequencer)),
    )
    .map_err(|_| RpcError::Verification)?;
    let protocol = receipt.protocol().ok_or(RpcError::Verification)?;
    let logs = decode_node_logs(value)?;
    let verified = verify_logs(&receipt, logs).map_err(|_| RpcError::Verification)?;
    let summary = verified.summary();
    let returned = node_bytes(value, "returnData", 1_048_576)?;
    let vm_error = value["vmError"].as_str().ok_or(RpcError::InvalidResponse)?;
    let payment = value["paymentDenom"]
        .as_str()
        .filter(|s| !s.is_empty() && s.len() <= 64)
        .ok_or(RpcError::InvalidResponse)?;
    let mut height = [0; 32];
    height[24..].copy_from_slice(&node_quantity(value, "height")?.to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&node_fixed::<20>(value, "from")?);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&node_fixed::<20>(value, "to")?);
    let fee = value["feeAmount"]
        .as_str()
        .and_then(|s| s.strip_prefix("0x"))
        .ok_or(RpcError::InvalidResponse)?;
    if fee.is_empty() || (fee.len() > 1 && fee.starts_with('0')) {
        return Err(RpcError::InvalidResponse);
    }
    let fee = u128::from_str_radix(fee, 16).map_err(|_| RpcError::InvalidResponse)?;
    if policy.network_id == 0
        || protocol.protocol_version() != policy.protocol_version
        || protocol.protocol_version() != 2
        || protocol.parameter_version() != 0
        || protocol.activity_id() != activity
        || node_fixed::<32>(value, "txHash")? != activity
        || protocol.batch_id() != height
        || protocol.debit_sequence() != node_quantity(value, "nonce")?
        || protocol.global_sequence() != node_quantity(value, "seq")?
        || protocol.timestamp() != node_quantity(value, "timestamp")?
        || protocol.context_hash() != node_fixed::<32>(value, "keyId")?
        || node_fixed::<32>(value, "signingKey")? != policy.sequencer.public_key()
        || protocol.asset() != <[u8; 32]>::from(Sha256::digest(payment.as_bytes()))
        || protocol.from() != from
        || protocol.to() != to
        || protocol.fee_charged() != fee
        || protocol.result_code() != i32::from(node_quantity(value, "status")? == 0)
        || node_quantity(value, "status")? > 1
        || summary.gas_used() != node_quantity(value, "gasUsed")?
        || summary.cumulative_gas_used() != node_quantity(value, "cumulativeGasUsed")?
        || summary.effective_gas_price() != node_quantity(value, "effectiveGasPrice")?
        || u64::from(summary.tx_index()) != node_quantity(value, "index")?
        || summary.contract_address() != node_fixed::<20>(value, "contractAddress")?
        || summary.sponsor() != node_fixed::<20>(value, "sponsor")?
        || Some(summary.fee_denom()) != value["feeDenom"].as_str()
        || usize::try_from(summary.return_data_len()).ok() != Some(returned.len())
        || protocol.activity_root() != <[u8; 32]>::from(Sha256::digest(&returned))
        || summary.vm_error_hash() != <[u8; 32]>::from(Sha256::digest(vm_error.as_bytes()))
    {
        return Err(RpcError::Verification);
    }
    Ok((receipt, canonical))
}

fn validate_node_receipt_fields(value: &Value) -> Result<(), RpcError> {
    const EXECUTION_FIELDS: &[&str] = &[
        "rung",
        "txHash",
        "nonce",
        "seq",
        "height",
        "index",
        "timestamp",
        "status",
        "returnData",
        "logs",
        "gasUsed",
        "cumulativeGasUsed",
        "effectiveGasPrice",
        "paymentDenom",
        "feeDenom",
        "feeAmount",
        "sponsor",
        "from",
        "to",
        "contractAddress",
        "vmError",
        "keyId",
        "signingKey",
        "receipt",
    ];
    let fields = value.as_object().ok_or(RpcError::InvalidResponse)?;
    let committed = match value["rung"].as_str() {
        Some("receipted") => false,
        Some("committed") => true,
        _ => return Err(RpcError::InvalidResponse),
    };
    let expected_fields = EXECUTION_FIELDS.len()
        + 2 * usize::from(committed)
        + usize::from(fields.contains_key("inclusion"));
    if fields.len() != expected_fields
        || EXECUTION_FIELDS
            .iter()
            .any(|field| !fields.contains_key(*field))
    {
        let refused = EXECUTION_FIELDS
            .iter()
            .find(|field| !fields.contains_key(**field))
            .copied()
            .unwrap_or("receipt field set");
        return Err(invalid_response(refused));
    }
    if committed
        && (node_quantity(value, "blockNumber")? < node_quantity(value, "height")?
            || node_fixed::<32>(value, "blockHash")? == [0; 32])
    {
        return Err(RpcError::Verification);
    }
    Ok(())
}

fn decode_node_logs(
    value: &Value,
) -> Result<Vec<layerx_proof::sequencer_logs::SequencerLog>, RpcError> {
    use layerx_proof::sequencer_logs::SequencerLog;
    let logs = value["logs"].as_array().ok_or(RpcError::InvalidResponse)?;
    let logs = logs
        .iter()
        .map(|log| {
            let fields = log.as_object().ok_or(RpcError::InvalidResponse)?;
            if fields.len() != 4
                || ["address", "topics", "data", "logIndex"]
                    .iter()
                    .any(|f| !fields.contains_key(*f))
            {
                return Err(RpcError::InvalidResponse);
            }
            let topics = log["topics"]
                .as_array()
                .filter(|v| v.len() <= 4)
                .ok_or(RpcError::InvalidResponse)?;
            let topics = topics
                .iter()
                .map(|topic| node_fixed::<32>(&serde_json::json!({"topic":topic}), "topic"))
                .collect::<Result<Vec<_>, _>>()?;
            Ok(SequencerLog::new(
                node_fixed(log, "address")?,
                topics,
                node_bytes(log, "data", 1_048_576)?,
                node_quantity(log, "logIndex")?,
            ))
        })
        .collect::<Result<Vec<_>, RpcError>>()?;
    Ok(logs)
}

fn verify_supplied_commit(
    material: &Value,
    receipt: &Receipt,
    result: &Value,
    (height, hash): (u64, [u8; 32]),
    policy: &ReceiptPolicy,
    evidence: &layerx_client::rpc::receipt_commit::ReceiptCommitEvidence<'_>,
) -> Result<(), RpcError> {
    if height != evidence.covering_height || height < node_quantity(result, "height")? {
        return Err(RpcError::Verification);
    }
    let head = layerx_client::rpc::commit::verify_commit_material(
        &material["signedHeader"],
        evidence.verifier,
    )
    .map_err(|_| RpcError::Verification)?;
    if u64::try_from(head.height()).ok() != Some(height) || head.header_hash() != hash {
        return Err(RpcError::Verification);
    }
    let pin = layerx_client::sequencer::ReceiptKeyEntry {
        key_id: policy.sequencer.sequencer_id(),
        public_key: policy.sequencer.public_key(),
        first_height: policy.sequencer.first_batch_number(),
        last_height: policy.sequencer.last_batch_number(),
    };
    layerx_client::rpc::receipt_commit::verify_receipt_coverage(
        receipt.protocol().ok_or(RpcError::Verification)?,
        node_quantity(result, "height")?,
        &head,
        evidence,
        &[pin],
    )
    .map_err(|_| RpcError::Verification)?;
    Ok(())
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
