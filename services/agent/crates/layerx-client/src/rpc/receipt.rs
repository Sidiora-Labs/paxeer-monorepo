//! Receipt reads over JSON-RPC.

use layerx_crypto::evm_transaction::transaction_hash;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, VerificationFailure};
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog, SequencerLogRefusal};
use layerx_types::result::ResultCode;
use layerx_wire::receipt::Receipt;
use sha2::{Digest as _, Sha256};

use crate::rpc::commit::{acquire_committed_header, CommitError};
use crate::rpc::receipt_commit::{verify_receipt_coverage, CoverageError, ReceiptCommitEvidence};
use crate::rpc::submit::Unknown;
use crate::sequencer::{
    ExecutedReceipt, ReceiptKeyEntry, SequencerEndpoint, SequencerError, SequencerReceipt,
    TransactionReceipt,
};
use layerx_programs_registry::chain_proof::VerifiedChainHead;
use std::time::Duration;

/// Bounded deterministic jitter schedule for reconnect attempts.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ReconnectPolicy {
    pub maximum_attempts: u8,
    pub base_delay: Duration,
    pub maximum_delay: Duration,
    pub jitter_percent: u8,
}

impl ReconnectPolicy {
    /// Returns one bounded deterministic delay from the attempt and endpoint.
    #[must_use]
    pub fn delay(self, attempt: u8, endpoint: &str) -> Duration {
        let exponent = u32::from(attempt.min(31));
        let factor = 1_u32.checked_shl(exponent).unwrap_or(u32::MAX);
        let base = self
            .base_delay
            .checked_mul(factor)
            .unwrap_or(self.maximum_delay)
            .min(self.maximum_delay);
        let jitter_bound = base
            .as_millis()
            .saturating_mul(u128::from(self.jitter_percent))
            / 100;
        if jitter_bound == 0 {
            return base;
        }
        let seed = endpoint.bytes().fold(u128::from(attempt), |value, byte| {
            value.wrapping_mul(131).wrapping_add(u128::from(byte))
        });
        let jitter = seed % (jitter_bound + 1);
        base.saturating_add(Duration::from_millis(
            u64::try_from(jitter).unwrap_or(u64::MAX),
        ))
        .min(self.maximum_delay)
    }
}

pub type Rung = layerx_types::rung::Rung;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct LookupContext<'a> {
    pub receipt_keys: &'a [ReceiptKeyEntry],
}

pub type AuthenticatedLookupContext<'a> = LookupContext<'a>;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthenticatedReceipt {
    receipt: Receipt,
    execution: ExecutedReceipt,
    module_id: u16,
    rung: Rung,
    landing: Option<TransactionReceipt>,
    covering_block: Option<(u64, [u8; 32])>,
    committed_header: Option<VerifiedChainHead>,
}

impl AuthenticatedReceipt {
    #[must_use]
    pub fn canonical_bytes(&self) -> &[u8] {
        &self.execution.receipt
    }

    #[must_use]
    pub const fn receipt(&self) -> &Receipt {
        &self.receipt
    }

    #[must_use]
    pub const fn execution(&self) -> &ExecutedReceipt {
        &self.execution
    }

    #[must_use]
    pub const fn activity_id(&self) -> [u8; 32] {
        self.execution.tx_hash
    }

    #[must_use]
    pub const fn global_sequence(&self) -> u64 {
        self.execution.seq
    }

    #[must_use]
    pub const fn height(&self) -> u64 {
        self.execution.height
    }

    #[must_use]
    pub const fn module_id(&self) -> u16 {
        self.module_id
    }

    #[must_use]
    pub fn result_code(&self) -> ResultCode {
        ResultCode::from_raw(i32::from(!self.execution.success))
    }

    #[must_use]
    pub const fn rung(&self) -> Rung {
        self.rung
    }

    #[must_use]
    pub const fn covering_block(&self) -> Option<(u64, [u8; 32])> {
        self.covering_block
    }

    #[must_use]
    pub const fn committed_header(&self) -> Option<&VerifiedChainHead> {
        self.committed_header.as_ref()
    }

    #[must_use]
    pub const fn landing(&self) -> Option<&TransactionReceipt> {
        self.landing.as_ref()
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Lookup {
    Absent,
    Submitted { tx_hash: [u8; 32], nonce: u64 },
    Verified(AuthenticatedReceipt),
}

pub type AuthenticatedLookup = Lookup;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ReceiptError {
    Sequencer(SequencerError),
    Verification(VerificationFailure),
    Logs(SequencerLogRefusal),
    FieldMismatch(&'static str),
    InvalidTransactionHash,
    Commit(CommitError),
    Coverage(CoverageError),
}

impl ReceiptError {
    #[must_use]
    pub const fn is_unavailable(&self) -> bool {
        matches!(
            self,
            Self::Sequencer(
                SequencerError::Endpoint | SequencerError::Tls | SequencerError::Unavailable
            )
        )
    }
}

impl From<SequencerError> for ReceiptError {
    fn from(value: SequencerError) -> Self {
        Self::Sequencer(value)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Resolution {
    Resolved(AuthenticatedReceipt),
    Unknown(Unknown),
}

pub fn lookup(
    endpoint: &SequencerEndpoint,
    tx_hash: [u8; 32],
    context: LookupContext<'_>,
) -> Result<Lookup, ReceiptError> {
    lookup_authenticated(endpoint, tx_hash, context)
}

pub fn lookup_authenticated(
    endpoint: &SequencerEndpoint,
    tx_hash: [u8; 32],
    context: AuthenticatedLookupContext<'_>,
) -> Result<AuthenticatedLookup, ReceiptError> {
    lookup_with_evidence(endpoint, tx_hash, context, None)
}

pub fn lookup_authenticated_with_commit(
    endpoint: &SequencerEndpoint,
    tx_hash: [u8; 32],
    context: AuthenticatedLookupContext<'_>,
    evidence: &ReceiptCommitEvidence<'_>,
) -> Result<AuthenticatedLookup, ReceiptError> {
    lookup_with_evidence(endpoint, tx_hash, context, Some(evidence))
}

fn lookup_with_evidence(
    endpoint: &SequencerEndpoint,
    tx_hash: [u8; 32],
    context: AuthenticatedLookupContext<'_>,
    evidence: Option<&ReceiptCommitEvidence<'_>>,
) -> Result<AuthenticatedLookup, ReceiptError> {
    if tx_hash == [0; 32] {
        return Err(ReceiptError::InvalidTransactionHash);
    }
    let (execution, covering_block) = match endpoint.sequencer_receipt(&tx_hash)? {
        None => return Ok(Lookup::Absent),
        Some(SequencerReceipt::Submitted { tx_hash, nonce }) => {
            return Ok(Lookup::Submitted { tx_hash, nonce });
        }
        Some(SequencerReceipt::Receipted(receipt)) => (receipt, None),
        Some(SequencerReceipt::Committed {
            receipt,
            block_number,
            block_hash,
        }) => (receipt, Some((block_number, block_hash))),
    };
    let keys: Vec<_> = context
        .receipt_keys
        .iter()
        .map(|key| {
            SequencerAuthorization::new(
                key.key_id,
                key.public_key,
                key.first_height,
                key.last_height,
            )
        })
        .collect();
    let receipt = verify_sequencer_signature(&execution.receipt, keys.as_slice())
        .map_err(ReceiptError::Verification)?;
    let module_id = verify_execution(&receipt, &execution, tx_hash)?;
    let landing = match endpoint.transaction_receipt(&tx_hash) {
        Ok(landing) => landing,
        Err(SequencerError::Unavailable) => None,
        Err(error) => return Err(error.into()),
    };
    if let Some(committed) = &landing {
        if committed.block_number < execution.height {
            return Err(ReceiptError::FieldMismatch("blockNumber"));
        }
        if committed.status != execution.success {
            return Err(ReceiptError::FieldMismatch("status"));
        }
    }
    let mut committed_header = None;
    if let Some(evidence) = evidence {
        let height = evidence.covering_height;
        if height < execution.height
            || landing
                .as_ref()
                .is_some_and(|landing| landing.block_number > height)
            || covering_block.is_some_and(|(claimed_height, _)| claimed_height != height)
        {
            return Err(ReceiptError::FieldMismatch("coveringBlock"));
        }
        match acquire_committed_header(endpoint, height, evidence.verifier) {
            Ok(head) => {
                if covering_block.is_some_and(|(_, block_hash)| block_hash != head.header_hash()) {
                    return Err(ReceiptError::FieldMismatch("blockHash"));
                }
                let block = match endpoint.block_by_number(height) {
                    Ok(block) => block,
                    Err(SequencerError::Unavailable) => None,
                    Err(error) => return Err(error.into()),
                };
                if let Some(block) = block {
                    if block.hash != head.header_hash() {
                        return Err(ReceiptError::FieldMismatch("blockHash"));
                    }
                    verify_receipt_coverage(
                        receipt
                            .protocol()
                            .ok_or(ReceiptError::FieldMismatch("receipt"))?,
                        execution.height,
                        &head,
                        evidence,
                        context.receipt_keys,
                    )
                    .map_err(ReceiptError::Coverage)?;
                    committed_header = Some(head);
                }
            }
            Err(CommitError::Rpc(SequencerError::Unavailable)) => {}
            Err(CommitError::Rpc(SequencerError::Rpc(ref refusal))) if refusal.code == -32601 => {}
            Err(error) => return Err(ReceiptError::Commit(error)),
        }
    }
    let rung = if committed_header.is_some() {
        Rung::Committed
    } else {
        Rung::Receipted
    };
    Ok(Lookup::Verified(AuthenticatedReceipt {
        receipt,
        execution,
        module_id,
        rung,
        landing,
        covering_block,
        committed_header,
    }))
}

fn verify_execution(
    receipt: &Receipt,
    execution: &ExecutedReceipt,
    tx_hash: [u8; 32],
) -> Result<u16, ReceiptError> {
    let protocol = receipt
        .protocol()
        .ok_or(ReceiptError::FieldMismatch("receipt"))?;
    let logs = execution
        .logs
        .iter()
        .map(|log| {
            SequencerLog::new(
                log.address,
                log.topics.clone(),
                log.data.clone(),
                log.log_index,
            )
        })
        .collect();
    let verified = verify_logs(receipt, logs).map_err(ReceiptError::Logs)?;
    let summary = verified.summary();
    let mut height = [0; 32];
    height[24..].copy_from_slice(&execution.height.to_be_bytes());
    let mut fee = [0; 32];
    fee[16..].copy_from_slice(&protocol.fee_charged().to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&execution.from);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&execution.to);
    for (matches, field) in [
        (protocol.activity_id() == tx_hash, "txHash"),
        (protocol.debit_sequence() == execution.nonce, "nonce"),
        (protocol.global_sequence() == execution.seq, "seq"),
        (protocol.batch_id() == height, "height"),
        (protocol.timestamp() == execution.timestamp, "timestamp"),
        (protocol.context_hash() == execution.key_id, "keyId"),
        (protocol.parameter_version() == 0, "parameterVersion"),
        (
            protocol.result_code() == i32::from(!execution.success),
            "status",
        ),
        (summary.gas_used() == execution.gas_used, "gasUsed"),
        (
            summary.cumulative_gas_used() == execution.cumulative_gas_used,
            "cumulativeGasUsed",
        ),
        (
            summary.effective_gas_price() == execution.effective_gas_price,
            "effectiveGasPrice",
        ),
        (u64::from(summary.tx_index()) == execution.index, "index"),
        (
            summary.contract_address() == execution.contract_address,
            "contractAddress",
        ),
        (summary.sponsor() == execution.sponsor, "sponsor"),
        (summary.fee_denom() == execution.fee_denom, "feeDenom"),
        (
            protocol.asset() == sha256(execution.payment_denom.as_bytes()),
            "paymentDenom",
        ),
        (fee == execution.fee_amount, "feeAmount"),
        (protocol.from() == from, "from"),
        (protocol.to() == to, "to"),
        (
            usize::try_from(summary.return_data_len()).ok() == Some(execution.return_data.len())
                && protocol.activity_root() == sha256(&execution.return_data),
            "returnData",
        ),
        (
            summary.vm_error_hash() == sha256(execution.vm_error.as_bytes()),
            "vmError",
        ),
    ] {
        if !matches {
            return Err(ReceiptError::FieldMismatch(field));
        }
    }
    Ok(protocol.module_id())
}

fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

pub fn resolve_unknown(
    endpoint: &SequencerEndpoint,
    unknown: &Unknown,
    context: LookupContext<'_>,
    policy: ReconnectPolicy,
) -> Result<Resolution, ReceiptError> {
    let tx_hash = transaction_hash(unknown.retry_bytes());
    for attempt in 0..policy.maximum_attempts {
        if attempt != 0 {
            std::thread::sleep(policy.delay(attempt, "receipt-resolution"));
        }
        match lookup(endpoint, tx_hash, context) {
            Ok(Lookup::Verified(receipt)) => return Ok(Resolution::Resolved(receipt)),
            Ok(Lookup::Absent | Lookup::Submitted { .. }) => {}
            Err(ReceiptError::Sequencer(SequencerError::Unavailable)) => {
                return Ok(Resolution::Unknown(unknown.clone()));
            }
            Err(error) => return Err(error),
        }
    }
    Ok(Resolution::Unknown(unknown.clone()))
}
