//! Receipt- and protocol-state-authoritative budget reconciliation.

use std::cmp::Ordering;

use crate::protocol_evidence::{
    EvidenceAuthority, RawReceiptEvidence, RawStateEvidence, ReceiptReplayError,
};

/// Budget state backed by a core-state proof or an explicit committed getBudget read.
/// The default evidence type preserves the proof-verifying entry point.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProtocolBudgetState<E = RawStateEvidence> {
    pub evidence: E,
}

pub use layerx_client::budget::{
    budget_state_key, ProtocolBudgetRecord, ReconcileError, BUDGET_MODULE_ID,
};

/// Raw receipt evidence applied to one declared protocol budget window.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SpendReceiptEvidence<E = RawReceiptEvidence> {
    pub expected_activity_id: [u8; 32],
    pub evidence: E,
}

/// Rebuildable local cache, never authority.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct LocalAccounting {
    pub consumed: u128,
    pub window_start_sequence: u64,
    pub last_receipt: Option<[u8; 32]>,
}

/// Opaque reconciliation result issued only after protocol evidence succeeds.
///
/// No public constructor exists; reconciliation uses verified core-state evidence
/// or a decoded getBudget result at an explicit committed block.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ReconciliationState {
    last_verified_receipt: Option<[u8; 32]>,
    protocol_consumed: u128,
    local_before: u128,
    local_after: u128,
    divergence: Option<i128>,
    window_start_sequence: u64,
    window_end_sequence: u64,
    remaining: u128,
    observed_head_sequence: u64,
}

impl ReconciliationState {
    /// Returns the protocol remaining amount from this opaque verified result.
    #[must_use]
    pub const fn remaining(&self) -> u128 {
        self.remaining
    }

    /// Returns the head sequence or committed block height which anchored this result.
    #[must_use]
    pub const fn observed_head_sequence(&self) -> u64 {
        self.observed_head_sequence
    }

    /// Returns the corrected local amount from this opaque verified result.
    #[must_use]
    pub const fn local_after(&self) -> u128 {
        self.local_after
    }

    pub(crate) const fn last_verified_receipt(&self) -> Option<[u8; 32]> {
        self.last_verified_receipt
    }

    pub(crate) const fn protocol_consumed(&self) -> u128 {
        self.protocol_consumed
    }

    pub(crate) const fn local_before(&self) -> u128 {
        self.local_before
    }

    pub(crate) const fn divergence(&self) -> Option<i128> {
        self.divergence
    }

    pub(crate) const fn window_start_sequence(&self) -> u64 {
        self.window_start_sequence
    }

    pub(crate) const fn window_end_sequence(&self) -> u64 {
        self.window_end_sequence
    }
}

/// Verifies receipt identity and state inclusion, then rebuilds the local cache
/// from the decoded canonical budget record.
pub(crate) fn reconcile_state(
    local: &mut LocalAccounting,
    protocol: &ProtocolBudgetState,
    receipts: &[SpendReceiptEvidence],
    verifier: &EvidenceAuthority,
) -> Result<ReconciliationState, ReconcileError> {
    let mut replay = EvidenceAuthority::receipt_replay_guard();
    let mut last_verified_receipt = local.last_receipt;
    for receipt in receipts {
        let verified_receipt = verifier
            .verify_receipt(&receipt.evidence)
            .map_err(|_| ReconcileError::UnverifiedReceipt)?;
        if verified_receipt.activity_id() != receipt.expected_activity_id {
            return Err(ReconcileError::ReceiptActivityMismatch);
        }
        replay.admit(&verified_receipt).map_err(map_replay_error)?;
        last_verified_receipt = Some(verified_receipt.activity_id());
    }
    let verified_state = verifier
        .verify_state(&protocol.evidence)
        .map_err(|_| ReconcileError::UnverifiedProtocolState)?;
    let record = ProtocolBudgetRecord::decode(verified_state.canonical_state())
        .map_err(|_| ReconcileError::ProtocolStateSchemaUnavailable)?;
    let protocol_consumed = record.spent_this_period;
    let local_before = local.consumed;
    let divergence = match local_before.cmp(&protocol_consumed) {
        Ordering::Equal => None,
        Ordering::Greater => {
            Some(i128::try_from(local_before - protocol_consumed).unwrap_or(i128::MAX))
        }
        Ordering::Less => Some(
            i128::try_from(protocol_consumed - local_before)
                .map_or(i128::MIN, |difference| -difference),
        ),
    };
    let state = ReconciliationState {
        last_verified_receipt,
        protocol_consumed,
        local_before,
        local_after: protocol_consumed,
        divergence,
        window_start_sequence: record.period_start,
        window_end_sequence: record.window_end_sequence(),
        remaining: record.remaining(),
        observed_head_sequence: verified_state.observed_head_sequence(),
    };
    local.consumed = protocol_consumed;
    local.window_start_sequence = record.period_start;
    local.last_receipt = last_verified_receipt;
    Ok(state)
}

const fn map_replay_error(error: ReceiptReplayError) -> ReconcileError {
    match error {
        ReceiptReplayError::DuplicateReceipt => ReconcileError::DuplicateReceipt,
        ReceiptReplayError::DuplicateActivity => ReconcileError::DuplicateActivity,
    }
}

/// A decoded layerxpay read and the committed block at which it was evaluated.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommittedBudgetEvidence {
    record: ProtocolBudgetRecord,
    height: u64,
}

/// Receipt identity and spend amount for a committed payExact or draw.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommittedSpendEvidence {
    tx_hash: [u8; 32],
    amount: u128,
    height: u64,
}

impl SpendReceiptEvidence<CommittedSpendEvidence> {
    /// Binds a declared spend to its successful committed transaction receipt.
    /// The amount is the payExact or draw amount, excluding the transaction fee.
    pub fn from_committed(
        expected_tx_hash: [u8; 32],
        amount: u128,
        receipt: &layerx_client::sequencer::SequencerReceipt,
    ) -> Result<Self, ReconcileError> {
        use layerx_client::sequencer::SequencerReceipt;
        let SequencerReceipt::Committed {
            receipt,
            block_number,
            ..
        } = receipt
        else {
            return Err(ReconcileError::UnverifiedReceipt);
        };
        if receipt.tx_hash != expected_tx_hash {
            return Err(ReconcileError::ReceiptActivityMismatch);
        }
        let mut pay = [0; 20];
        pay[18..].copy_from_slice(&[0x10, 0x1b]);
        let mut ledger = pay;
        ledger[19] = 0x1a;
        if !receipt.success
            || receipt.height != *block_number
            || amount == 0
            || (receipt.to != pay && receipt.to != ledger)
        {
            return Err(ReconcileError::UnverifiedReceipt);
        }
        Ok(Self {
            expected_activity_id: expected_tx_hash,
            evidence: CommittedSpendEvidence {
                tx_hash: receipt.tx_hash,
                amount,
                height: *block_number,
            },
        })
    }

    #[must_use]
    pub const fn amount(&self) -> u128 {
        self.evidence.amount
    }

    #[must_use]
    pub const fn receipt_hash(&self) -> [u8; 32] {
        self.evidence.tx_hash
    }
}

impl ProtocolBudgetState<CommittedBudgetEvidence> {
    /// Reads getBudget(bytes32) on 0x101B at an explicit committed height.
    pub fn read_at(
        endpoint: &layerx_client::sequencer::SequencerEndpoint,
        budget_id: [u8; 32],
        height: u64,
    ) -> Result<Self, layerx_client::sequencer::SequencerError> {
        let selector = layerx_crypto::evm_transaction::transaction_hash(b"getBudget(bytes32)");
        let mut data = selector[..4].to_vec();
        data.extend_from_slice(&budget_id);
        let mut address = [0; 20];
        address[18..].copy_from_slice(&[0x10, 0x1b]);
        let result = endpoint.call_at(height, &address, &data)?;
        Self::decode_get_budget(budget_id, height, &result)
            .map_err(|_| layerx_client::sequencer::SequencerError::Malformed)
    }

    /// Decodes the exact ABI result of getBudget at the caller's committed height.
    /// Decoding itself does not authenticate an endpoint or prove state inclusion.
    pub fn decode_get_budget(
        budget_id: [u8; 32],
        height: u64,
        result: &[u8],
    ) -> Result<Self, ReconcileError> {
        let unavailable = ReconcileError::ProtocolStateSchemaUnavailable;
        let word = |index: usize| -> Result<[u8; 32], ReconcileError> {
            result
                .get(index * 32..(index + 1) * 32)
                .and_then(|bytes| bytes.try_into().ok())
                .ok_or(unavailable)
        };
        let integer = |index: usize, width: usize| -> Result<u128, ReconcileError> {
            let bytes = word(index)?;
            if bytes[..32 - width].iter().any(|byte| *byte != 0) {
                return Err(unavailable);
            }
            let mut value = [0; 16];
            value[16 - width..].copy_from_slice(&bytes[32 - width..]);
            Ok(u128::from_be_bytes(value))
        };
        if integer(0, 8)? != 32 || integer(17, 8)? != 18 * 32 {
            return Err(unavailable);
        }
        let count = usize::try_from(integer(19, 8)?).map_err(|_| unavailable)?;
        if count > 16 || result.len() != (20 + count) * 32 {
            return Err(unavailable);
        }
        let mut delegates = Vec::with_capacity(count);
        for index in 0..count {
            let delegate = word(20 + index)?;
            if delegates
                .last()
                .is_some_and(|previous| *previous >= delegate)
            {
                return Err(unavailable);
            }
            delegates.push(delegate);
        }
        let closed = integer(15, 1)?;
        let revoked = integer(16, 1)?;
        if closed > 1 || revoked > 1 {
            return Err(unavailable);
        }
        let source = word(18)?;
        let record = ProtocolBudgetRecord {
            budget_id,
            owner: word(1)?,
            budget_account: word(2)?,
            asset_id: word(3)?,
            purpose_hash: word(4)?,
            per_period_limit: integer(5, 16)?,
            configured_period_limit: integer(6, 16)?,
            carry_cap: integer(7, 16)?,
            spent_this_period: integer(8, 16)?,
            carried: integer(9, 16)?,
            period_length: u64::try_from(integer(10, 8)?).map_err(|_| unavailable)?,
            period_start: u64::try_from(integer(11, 8)?).map_err(|_| unavailable)?,
            expiry: u64::try_from(integer(12, 8)?).map_err(|_| unavailable)?,
            revocation_sequence: u64::try_from(integer(13, 8)?).map_err(|_| unavailable)?,
            rollover_policy: u8::try_from(integer(14, 1)?).map_err(|_| unavailable)?,
            closed: closed != 0,
            revoked: revoked != 0,
            delegates,
            source_account: (source != [0; 32]).then_some(source),
        };
        if budget_id == [0; 32]
            || record.period_length == 0
            || record.spent_this_period > record.per_period_limit
            || record
                .period_start
                .checked_add(record.period_length)
                .is_none()
            || record.expiry <= record.period_start
            || !(1..=2).contains(&record.rollover_policy)
            || (record.rollover_policy == 1 && record.carry_cap != 0)
        {
            return Err(unavailable);
        }
        Ok(Self {
            evidence: CommittedBudgetEvidence { record, height },
        })
    }

    #[must_use]
    pub const fn record(&self) -> &ProtocolBudgetRecord {
        &self.evidence.record
    }

    #[must_use]
    pub const fn committed_height(&self) -> u64 {
        self.evidence.height
    }

    pub fn reconcile(
        &self,
        local: &mut LocalAccounting,
        receipts: &[SpendReceiptEvidence<CommittedSpendEvidence>],
    ) -> Result<ReconciliationState, ReconcileError> {
        let mut seen = std::collections::HashSet::new();
        let mut last = local.last_receipt;
        for receipt in receipts {
            if receipt.expected_activity_id != receipt.evidence.tx_hash {
                return Err(ReconcileError::ReceiptActivityMismatch);
            }
            if receipt.evidence.height > self.evidence.height {
                return Err(ReconcileError::UnverifiedReceipt);
            }
            if !seen.insert(receipt.evidence.tx_hash) {
                return Err(ReconcileError::DuplicateReceipt);
            }
            last = Some(receipt.evidence.tx_hash);
        }
        let record = self.record();
        let before = local.consumed;
        let spent = record.spent_this_period;
        let divergence = match before.cmp(&spent) {
            Ordering::Equal => None,
            Ordering::Greater => Some(i128::try_from(before - spent).unwrap_or(i128::MAX)),
            Ordering::Less => {
                Some(i128::try_from(spent - before).map_or(i128::MIN, |value| -value))
            }
        };
        let state = ReconciliationState {
            last_verified_receipt: last,
            protocol_consumed: spent,
            local_before: before,
            local_after: spent,
            divergence,
            window_start_sequence: record.period_start,
            window_end_sequence: record.window_end_sequence(),
            remaining: if record.closed || record.revoked {
                0
            } else {
                record.remaining()
            },
            observed_head_sequence: self.evidence.height,
        };
        local.consumed = spent;
        local.window_start_sequence = record.period_start;
        local.last_receipt = last;
        Ok(state)
    }
}
