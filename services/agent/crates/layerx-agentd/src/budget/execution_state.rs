use super::program_settlement::ProgramSettlementError;
use crate::protocol_evidence::{
    verified_program_record, EvidenceAuthority, VerifiedReceiptEvidence,
};
use crate::read::{CommittedProgramBalanceFreshness, ProgramBalanceRead, ProgramValueBalance};
use layerx_client::{
    evidence::VerifiedCaps,
    sequencer::{SequencerEndpoint, SequencerReceipt},
};
use layerx_programs::{ProgramId, ProgramLifecycle};
use layerx_proof::{inclusion::SequencerAuthorization, state::CanonicalAccount};
use layerx_types::verify::VerificationLevel;
use sha2::{Digest as _, Sha256};
use std::collections::BTreeMap;

#[derive(Clone, Debug)]
pub(crate) struct CommittedExecutionState {
    pub(super) block_number: u64,
    pub(super) block_hash: [u8; 32],
    pub(super) prestate_height: u64,
    pub(super) balances: ProgramBalanceRead<CommittedProgramBalanceFreshness>,
    pub(super) program_records: BTreeMap<Vec<u8>, Vec<u8>>,
    pub(super) accounts: BTreeMap<[u8; 32], CanonicalAccount>,
    pub(super) observed_head_sequence: u64,
    pub(super) activity_id: [u8; 32],
    pub(super) global_sequence: u64,
    pub(super) receipt_ref: [u8; 32],
}

pub(crate) fn committed_coordinates(
    observed: &SequencerReceipt,
    canonical: &[u8],
    activity_id: [u8; 32],
    sequence: u64,
    level: VerificationLevel,
    observed_head: u64,
) -> Result<(u64, [u8; 32]), ProgramSettlementError> {
    let SequencerReceipt::Committed {
        receipt,
        block_number,
        block_hash,
    } = observed
    else {
        return Err(ProgramSettlementError::SourceSnapshot);
    };
    if *block_number < 2
        || *block_number < receipt.height
        || *block_hash == [0; 32]
        || receipt.tx_hash != activity_id
        || receipt.seq != sequence
        || receipt.receipt != canonical
        || level < VerificationLevel::STATE_PROVEN
        || observed_head < sequence
    {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    Ok((*block_number, *block_hash))
}

impl CommittedExecutionState {
    pub(crate) fn program_records(&self) -> &BTreeMap<Vec<u8>, Vec<u8>> {
        &self.program_records
    }
    pub(crate) fn all_accounts(&self) -> &BTreeMap<[u8; 32], CanonicalAccount> {
        &self.accounts
    }
    pub(crate) fn validate_receipt(
        &self,
        receipt: &VerifiedReceiptEvidence,
    ) -> Result<(), ProgramSettlementError> {
        self.validate_binding(
            receipt.activity_id(),
            receipt.global_sequence(),
            receipt.receipt_ref(),
        )
    }
    pub(crate) fn validate_binding(
        &self,
        activity_id: [u8; 32],
        global_sequence: u64,
        receipt_ref: [u8; 32],
    ) -> Result<(), ProgramSettlementError> {
        if self.block_number < 2
            || self.block_hash == [0; 32]
            || self.prestate_height.checked_add(1) != Some(self.block_number)
            || self.balances.freshness.height != self.prestate_height
            || self.balances.freshness.block_hash == [0; 32]
            || self.observed_head_sequence < global_sequence
            || self.activity_id != activity_id
            || self.global_sequence != global_sequence
            || self.receipt_ref != receipt_ref
        {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        Ok(())
    }
    pub(crate) fn program_balance(
        &self,
        program: [u8; 32],
        account: [u8; 32],
    ) -> Result<&ProgramValueBalance, ProgramSettlementError> {
        if self.balances.program != program
            || self.balances.freshness.height != self.prestate_height
        {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let mut key = b"wind-down\0s".to_vec();
        key.extend(program);
        let lifecycle = match self.program_records.get(&key) {
            None => ProgramLifecycle::Active,
            Some(record) if record.len() == 54 && record[0] == 1 => match record[1] {
                1 => ProgramLifecycle::Active,
                2 => ProgramLifecycle::Deprecated,
                3 => ProgramLifecycle::Tombstoned,
                _ => return Err(ProgramSettlementError::SourceSnapshot),
            },
            _ => return Err(ProgramSettlementError::SourceSnapshot),
        };
        if lifecycle != self.balances.lifecycle {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let mut values = self
            .balances
            .accounts
            .iter()
            .filter(|value| value.account == account);
        let value = values
            .next()
            .ok_or(ProgramSettlementError::SourceSnapshot)?;
        if values.next().is_some() {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        Ok(value)
    }
}

pub(crate) fn committed_execution_state(
    node: &mut layerx_client::sequencer::SequencerEndpoint,
    verifier: &EvidenceAuthority,
    sequencer_key: [u8; 32],
    endpoint: &SequencerEndpoint,
    caps: &VerifiedCaps,
    receipt: &VerifiedReceiptEvidence,
    program: [u8; 32],
    accounts: &[[u8; 32]],
    correlation: u64,
    authorization: SequencerAuthorization,
) -> Result<CommittedExecutionState, ProgramSettlementError> {
    let fail = |_| ProgramSettlementError::SourceSnapshot;
    let observed_head = endpoint.block_number().map_err(fail)?;
    if node.chain_id().map_err(fail)? != endpoint.chain_id().map_err(fail)?
        || node.block_number().map_err(fail)? != observed_head
        || caps.freshness().observed_head_sequence != observed_head
    {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    let observed = endpoint
        .sequencer_receipt(&receipt.activity_id())
        .map_err(fail)?
        .ok_or(ProgramSettlementError::SourceSnapshot)?;
    let (block_number, block_hash) = committed_coordinates(
        &observed,
        receipt.canonical_receipt(),
        receipt.activity_id(),
        receipt.global_sequence(),
        caps.level(),
        observed_head,
    )?;
    let block = endpoint
        .block_by_number(block_number)
        .map_err(fail)?
        .ok_or(ProgramSettlementError::SourceSnapshot)?;
    if block.number != block_number
        || block.hash != block_hash
        || node.block_by_number(block_number).map_err(fail)?.as_ref() != Some(&block)
    {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    let prestate_height = block_number - 1;
    let balances = crate::read::program_balances_from_protocol(
        endpoint,
        ProgramId::new(program).map_err(|_| ProgramSettlementError::SourceSnapshot)?,
        accounts,
        prestate_height,
    )
    .map_err(fail)?;
    let mut program_records = BTreeMap::new();
    let mut read_record = |key: Vec<u8>| -> Result<Option<Vec<u8>>, ProgramSettlementError> {
        let verified = verified_program_record(
            node,
            verifier,
            sequencer_key,
            correlation,
            authorization,
            caps.signed_header(),
            &key,
        )
        .map_err(|_| ProgramSettlementError::SourceSnapshot)?;
        if verified.level() < VerificationLevel::STATE_PROVEN
            || verified.observed_head_sequence() < receipt.global_sequence()
            || verified.observed_head_sequence() != caps.freshness().observed_head_sequence
        {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        if verified.canonical_state().is_empty() {
            return Ok(None);
        }
        let bytes = verified.canonical_state().to_vec();
        program_records.insert(key, bytes.clone());
        Ok(Some(bytes))
    };
    for prefix in [
        b"program\0".as_slice(),
        b"program-owner\0",
        b"wind-down\0s",
        b"interface\0",
    ] {
        let mut key = prefix.to_vec();
        key.extend(program);
        read_record(key)?;
    }
    read_record(b"progfee/active/v1".to_vec())?;
    for account in accounts {
        let mut key = b"wind-down\0r".to_vec();
        key.extend(program);
        key.extend(account);
        read_record(key)?;
        let mut key = b"program-account\0r".to_vec();
        key.extend(account);
        if let Some(binding) = read_record(key)? {
            if binding.len() < 139 || binding[0] != 2 {
                return Err(ProgramSettlementError::SourceSnapshot);
            }
            let length = usize::from(u16::from_be_bytes([binding[97], binding[98]]));
            if length > 128 || binding.len() != 139 + length {
                return Err(ProgramSettlementError::SourceSnapshot);
            }
            let mut key = b"program-account\0p".to_vec();
            key.extend(&binding[1..33]);
            key.extend(Sha256::digest(&binding[139..]));
            read_record(key)?;
        }
    }
    if endpoint.block_number().map_err(fail)? != observed_head
        || node.block_number().map_err(fail)? != observed_head
        || endpoint
            .block_by_number(block_number)
            .map_err(fail)?
            .as_ref()
            != Some(&block)
        || node.block_by_number(block_number).map_err(fail)?.as_ref() != Some(&block)
    {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    let state = CommittedExecutionState {
        block_number,
        block_hash,
        prestate_height,
        balances,
        program_records,
        accounts: caps.all_accounts().clone(),
        observed_head_sequence: observed_head,
        activity_id: receipt.activity_id(),
        global_sequence: receipt.global_sequence(),
        receipt_ref: receipt.receipt_ref(),
    };
    state.validate_receipt(receipt)?;
    Ok(state)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn execution_receipt() -> layerx_client::sequencer::ExecutedReceipt {
        layerx_client::sequencer::ExecutedReceipt {
            tx_hash: [1; 32],
            nonce: 0,
            seq: 7,
            height: 3,
            index: 0,
            timestamp: 1,
            success: true,
            return_data: Vec::new(),
            logs: Vec::new(),
            gas_used: 1,
            cumulative_gas_used: 1,
            effective_gas_price: 1,
            fee_denom: "asset".to_owned(),
            fee_amount: [0; 32],
            sponsor: [0; 20],
            from: [2; 20],
            to: [3; 20],
            contract_address: [0; 20],
            vm_error: String::new(),
            key_id: [4; 32],
            receipt: vec![5],
            signing_key: [6; 32],
        }
    }

    #[test]
    fn committed_execution_state_refuses_receipted_rung() {
        use layerx_client::sequencer::SequencerReceipt;
        let receipt = execution_receipt();
        let observed = SequencerReceipt::Receipted(receipt.clone());
        assert_eq!(
            committed_coordinates(
                &observed,
                &receipt.receipt,
                receipt.tx_hash,
                receipt.seq,
                layerx_types::verify::VerificationLevel::STATE_PROVEN,
                7
            ),
            Err(ProgramSettlementError::SourceSnapshot)
        );
    }

    #[test]
    fn committed_execution_state_refuses_stale_head_and_mismatched_receipt() {
        use layerx_client::sequencer::SequencerReceipt;
        use layerx_types::verify::VerificationLevel;
        let receipt = execution_receipt();
        let observed = SequencerReceipt::Committed {
            receipt: receipt.clone(),
            block_number: 3,
            block_hash: [7; 32],
        };
        let check = |bytes: &[u8], sequence, level, head| {
            committed_coordinates(&observed, bytes, receipt.tx_hash, sequence, level, head)
        };
        assert_eq!(
            check(&receipt.receipt, 7, VerificationLevel::STATE_PROVEN, 6),
            Err(ProgramSettlementError::SourceSnapshot)
        );
        assert_eq!(
            check(&receipt.receipt, 7, VerificationLevel::STATE_PROVEN, 7),
            Ok((3, [7; 32]))
        );
        assert_eq!(
            check(&[9], 7, VerificationLevel::STATE_PROVEN, 7),
            Err(ProgramSettlementError::SourceSnapshot)
        );
        assert_eq!(
            check(&receipt.receipt, 8, VerificationLevel::STATE_PROVEN, 8),
            Err(ProgramSettlementError::SourceSnapshot)
        );
        assert_eq!(
            check(&receipt.receipt, 7, VerificationLevel::BATCH_INCLUDED, 7),
            Err(ProgramSettlementError::SourceSnapshot)
        );
        for block_number in [0, 1, 2] {
            let invalid = SequencerReceipt::Committed {
                receipt: receipt.clone(),
                block_number,
                block_hash: [7; 32],
            };
            assert_eq!(
                committed_coordinates(
                    &invalid,
                    &receipt.receipt,
                    receipt.tx_hash,
                    receipt.seq,
                    VerificationLevel::STATE_PROVEN,
                    7
                ),
                Err(ProgramSettlementError::SourceSnapshot)
            );
        }
    }
}
