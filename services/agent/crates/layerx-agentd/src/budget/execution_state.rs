use super::program_settlement::ProgramSettlementError;
use crate::protocol_evidence::{
    verified_program_record, EvidenceAuthority, VerifiedReceiptEvidence,
};
use crate::read::{CommittedProgramBalanceFreshness, ProgramBalanceRead, ProgramValueBalance};
use layerx_client::sequencer::{SequencerEndpoint, SequencerReceipt};
use layerx_programs::{ProgramId, ProgramLifecycle};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_types::verify::VerificationLevel;
use sha2::{Digest as _, Sha256};
use std::collections::BTreeMap;

#[derive(Clone, Debug)]
pub struct CommittedExecutionState {
    pub(super) block_number: u64,
    pub(super) block_hash: [u8; 32],
    pub(super) prestate_height: u64,
    pub(super) balances: Option<ProgramBalanceRead<CommittedProgramBalanceFreshness>>,
    pub(super) prestate_block_hash: [u8; 32],
    pub(super) program_records: CommittedProgramRecords,
    pub(super) accounts: BTreeMap<[u8; 32], crate::read::committed::CommittedAccount>,
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
    /// Height of the authenticated state immediately before this execution.
    #[must_use]
    pub const fn prestate_height(&self) -> u64 {
        self.prestate_height
    }

    /// Whether the program already existed before execution (false for deployment).
    #[must_use]
    pub fn program_existed(&self) -> bool {
        self.program_records.program.is_some()
    }

    /// Authenticated pre-execution balance and frozen status of a program value account.
    #[must_use]
    pub fn account_status(&self, account: [u8; 32]) -> Option<(u128, bool)> {
        self.program_records
            .accounts
            .get(&account)
            .map(|value| (value.balance, value.frozen))
    }

    pub(crate) fn program_records(&self) -> &CommittedProgramRecords {
        &self.program_records
    }
    pub(crate) fn all_accounts(
        &self,
    ) -> &BTreeMap<[u8; 32], crate::read::committed::CommittedAccount> {
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
            || self.prestate_block_hash == [0; 32]
            || self.balances.is_some() != self.program_records.program.is_some()
            || self.balances.as_ref().is_some_and(|balances| {
                balances.freshness.height != self.prestate_height
                    || balances.freshness.block_hash != self.prestate_block_hash
            })
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
        let balances = self
            .balances
            .as_ref()
            .ok_or(ProgramSettlementError::SourceSnapshot)?;
        if balances.program != program || balances.freshness.height != self.prestate_height {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let lifecycle = match self
            .program_records
            .wind_down
            .as_ref()
            .ok_or(ProgramSettlementError::SourceSnapshot)?
            .status
        {
            1 => ProgramLifecycle::Active,
            2 => ProgramLifecycle::Deprecated,
            3 => ProgramLifecycle::Tombstoned,
            _ => return Err(ProgramSettlementError::SourceSnapshot),
        };
        if lifecycle != balances.lifecycle {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let mut values = balances
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

#[cfg(test)]
type TestProgramRecordRead<'a> =
    dyn FnMut(&[u8], u64) -> Result<Option<Vec<u8>>, ProgramSettlementError> + 'a;

#[cfg(test)]
fn collect_program_records(
    program: [u8; 32],
    accounts: &[[u8; 32]],
    height: u64,
    read: &mut TestProgramRecordRead<'_>,
) -> Result<BTreeMap<Vec<u8>, Vec<u8>>, ProgramSettlementError> {
    let mut program_records = BTreeMap::new();
    let mut read_record = |key: Vec<u8>| -> Result<Option<Vec<u8>>, ProgramSettlementError> {
        let value = read(&key, height)?;
        if let Some(bytes) = &value {
            program_records.insert(key, bytes.clone());
        }
        Ok(value)
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
    Ok(program_records)
}

pub(crate) fn committed_execution_state(
    node: &mut layerx_client::sequencer::SequencerEndpoint,
    input: &CommittedExecutionEvidence<'_>,
    endpoint: &SequencerEndpoint,
    caps: &crate::read::CommittedCaps,
    receipt: &VerifiedReceiptEvidence,
    program: [u8; 32],
    accounts: &[[u8; 32]],
) -> Result<CommittedExecutionState, ProgramSettlementError> {
    let CommittedExecutionEvidence {
        verifier,
        sequencer_key,
        correlation,
        authorization,
    } = *input;

    committed_execution_state_with_account_proofs(
        node,
        verifier,
        sequencer_key,
        endpoint,
        caps,
        receipt,
        program,
        accounts,
        correlation,
        authorization,
        &BTreeMap::new(),
    )
}

/// Acquires receipt-bound prestate with separately authenticated account status.
/// # Errors
/// Returns an error if the preparation, receipt, source snapshot or retained allocation does
/// not prove the requested settlement.
#[allow(
    clippy::too_many_arguments,
    reason = "Preserve the public API signature used by daemon consumers."
)]
pub fn committed_execution_state_with_account_proofs(
    node: &mut SequencerEndpoint,
    _verifier: &EvidenceAuthority,
    _sequencer_key: [u8; 32],
    endpoint: &SequencerEndpoint,
    caps: &crate::read::CommittedCaps,
    receipt: &VerifiedReceiptEvidence,
    program: [u8; 32],
    accounts: &[[u8; 32]],
    _correlation: u64,
    _authorization: SequencerAuthorization,
    account_proofs: &BTreeMap<[u8; 32], layerx_proof::state::VerifiedAccountState>,
) -> Result<CommittedExecutionState, ProgramSettlementError> {
    acquire_execution_state(
        node,
        endpoint,
        caps,
        &ExecutionReceipt {
            canonical_receipt: receipt.canonical_receipt(),
            activity_id: receipt.activity_id(),
            global_sequence: receipt.global_sequence(),
            receipt_ref: receipt.receipt_ref(),
        },
        program,
        accounts,
        account_proofs,
    )
}

pub(super) fn acquire_execution_state(
    node: &mut SequencerEndpoint,
    endpoint: &SequencerEndpoint,
    caps: &crate::read::CommittedCaps,
    input: &ExecutionReceipt<'_>,
    program: [u8; 32],
    accounts: &[[u8; 32]],
    account_proofs: &BTreeMap<[u8; 32], layerx_proof::state::VerifiedAccountState>,
) -> Result<CommittedExecutionState, ProgramSettlementError> {
    let ExecutionReceipt {
        canonical_receipt,
        activity_id,
        global_sequence,
        receipt_ref,
    } = *input;

    let fail = |_| ProgramSettlementError::SourceSnapshot;
    let observed_head = endpoint.block_number().map_err(fail)?;
    if node.chain_id().map_err(fail)? != endpoint.chain_id().map_err(fail)?
        || node.block_number().map_err(fail)? != observed_head
        || caps.freshness().observed_head_sequence != observed_head
    {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    let observed = endpoint
        .sequencer_receipt(&activity_id)
        .map_err(fail)?
        .ok_or(ProgramSettlementError::SourceSnapshot)?;
    let (block_number, block_hash) = committed_coordinates(
        &observed,
        canonical_receipt,
        activity_id,
        global_sequence,
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
    let reads = crate::read::CommittedReads::open(node, prestate_height)
        .map_err(|_| ProgramSettlementError::SourceSnapshot)?;
    let program_records = read_program_records_with_account_proofs(
        node,
        reads.checkpoint(),
        program,
        accounts,
        account_proofs,
    )?;
    let prestate_block_hash = reads.checkpoint().block_hash;
    let mut balances = if program_records.program.is_some() {
        Some(
            crate::read::program_balances_from_protocol(
                endpoint,
                ProgramId::new(program).map_err(|_| ProgramSettlementError::SourceSnapshot)?,
                accounts,
                prestate_height,
            )
            .map_err(fail)?,
        )
    } else {
        None
    };
    if let Some(balances) = &mut balances {
        for value in &mut balances.accounts {
            let account = program_records
                .accounts
                .get(&value.account)
                .ok_or(ProgramSettlementError::SourceSnapshot)?;
            if value.asset != account.asset || value.amount != account.balance {
                return Err(ProgramSettlementError::SourceSnapshot);
            }
            value.frozen = Some(account.frozen);
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
        prestate_block_hash,
        program_records,
        accounts: caps.all_accounts().clone(),
        observed_head_sequence: observed_head,
        activity_id,
        global_sequence,
        receipt_ref,
    };
    state.validate_binding(activity_id, global_sequence, receipt_ref)?;
    Ok(state)
}

/// Facts exposed by the programs ABI. The fee view does not expose the old
/// module record's runtime bounds; consumers must use these explicit fields.
#[derive(Clone, Debug)]
pub(crate) struct CommittedProgramRecords {
    pub program: Option<ProgramView>,
    pub owner: [u8; 32],
    pub wind_down: Option<WindDownView>,
    pub interface: Option<InterfaceView>,
    pub fee_schedule: ProgramFeeView,
    pub accounts: BTreeMap<[u8; 32], ProgramAccountView>,
    pub exit_routes: BTreeMap<[u8; 32], ProgramExitRoute>,
}
#[derive(Clone, Debug)]
pub(crate) struct ProgramExitRoute {
    pub asset: [u8; 32],
    pub destination: [u8; 32],
    pub seed: Vec<u8>,
}
#[derive(Clone, Debug)]
pub(crate) struct ProgramView {
    pub policy: u8,
    pub authority: [u8; 32],
    pub code_hash: [u8; 32],
    pub abi_version: u16,
    pub version: u32,
    pub active: bool,
}
#[derive(Clone, Debug)]
pub(crate) struct WindDownView {
    pub status: u8,
    pub exit_program: [u8; 32],
    pub deadline: u64,
    pub effective_sequence: u64,
    pub account_count: u16,
    pub live_count: u16,
}
#[derive(Clone, Debug)]
pub(crate) struct InterfaceView {
    pub version: u32,
    pub digest: [u8; 32],
    pub encoding: Vec<u8>,
}
#[derive(Clone, Debug)]
pub(crate) struct ProgramFeeView {
    pub version: u32,
    pub prices: [u64; 7],
    pub fee_asset: [u8; 32],
}
#[derive(Clone, Debug)]
pub(crate) struct ProgramAccountView {
    pub program: [u8; 32],
    pub asset: [u8; 32],
    pub seed: Vec<u8>,
    pub registered_sequence: u64,
    pub balance: u128,
    pub frozen: bool,
}

fn abi_word(bytes: &[u8], index: usize) -> Result<[u8; 32], ProgramSettlementError> {
    let offset = index
        .checked_mul(32)
        .ok_or(ProgramSettlementError::SourceSnapshot)?;
    bytes
        .get(offset..offset + 32)
        .and_then(|word| word.try_into().ok())
        .ok_or(ProgramSettlementError::SourceSnapshot)
}
fn abi_uint(bytes: &[u8], index: usize, width: usize) -> Result<u128, ProgramSettlementError> {
    let word = abi_word(bytes, index)?;
    if width == 0 || width > 16 || word[..32 - width].iter().any(|byte| *byte != 0) {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    let mut value = [0; 16];
    value[16 - width..].copy_from_slice(&word[32 - width..]);
    Ok(u128::from_be_bytes(value))
}
fn abi_bool(bytes: &[u8], index: usize) -> Result<bool, ProgramSettlementError> {
    match abi_uint(bytes, index, 1)? {
        0 => Ok(false),
        1 => Ok(true),
        _ => Err(ProgramSettlementError::SourceSnapshot),
    }
}
fn abi_tail(
    bytes: &[u8],
    index: usize,
    head: usize,
    maximum: usize,
) -> Result<Vec<u8>, ProgramSettlementError> {
    let offset = usize::try_from(abi_uint(bytes, index, 8)?)
        .map_err(|_| ProgramSettlementError::SourceSnapshot)?;
    if offset != head * 32 {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    let length = usize::try_from(abi_uint(bytes, head, 8)?)
        .map_err(|_| ProgramSettlementError::SourceSnapshot)?;
    if length > maximum {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    let start = offset + 32;
    let end = start
        .checked_add(length)
        .ok_or(ProgramSettlementError::SourceSnapshot)?;
    let padded = start
        .checked_add(length.div_ceil(32) * 32)
        .ok_or(ProgramSettlementError::SourceSnapshot)?;
    if bytes.len() != padded || bytes[end..].iter().any(|byte| *byte != 0) {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    Ok(bytes[start..end].to_vec())
}

fn decode_fee_schedule(bytes: &[u8]) -> Result<ProgramFeeView, ProgramSettlementError> {
    if bytes.len() != 288 {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    let mut prices = [0; 7];
    for (index, price) in prices.iter_mut().enumerate() {
        *price = u64::try_from(abi_uint(bytes, index + 1, 8)?)
            .map_err(|_| ProgramSettlementError::SourceSnapshot)?;
    }
    let fee_schedule = ProgramFeeView {
        version: u32::try_from(abi_uint(bytes, 0, 4)?)
            .map_err(|_| ProgramSettlementError::SourceSnapshot)?,
        prices,
        fee_asset: abi_word(bytes, 8)?,
    };
    if fee_schedule.version == 0
        || fee_schedule.prices.contains(&0)
        || fee_schedule.fee_asset == [0; 32]
    {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    Ok(fee_schedule)
}

pub(crate) fn read_program_records(
    node: &SequencerEndpoint,
    checkpoint: &crate::read::ServedCheckpoint,
    program: [u8; 32],
    accounts: &[[u8; 32]],
) -> Result<CommittedProgramRecords, ProgramSettlementError> {
    read_program_records_with_account_proofs(node, checkpoint, program, accounts, &BTreeMap::new())
}

pub(crate) fn read_program_records_with_account_proofs(
    node: &SequencerEndpoint,
    checkpoint: &crate::read::ServedCheckpoint,
    program: [u8; 32],
    accounts: &[[u8; 32]],
    account_proofs: &BTreeMap<[u8; 32], layerx_proof::state::VerifiedAccountState>,
) -> Result<CommittedProgramRecords, ProgramSettlementError> {
    if program == [0; 32] || accounts.len() > layerx_programs::MAX_PROGRAM_VALUE_ACCOUNTS {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    let reader = ProgramRecordReader { node, checkpoint };
    let reads = crate::read::CommittedReads::at_checkpoint(node, checkpoint)
        .map_err(|_| ProgramSettlementError::SourceSnapshot)?;
    if !reads
        .program_exists(program)
        .map_err(|_| ProgramSettlementError::SourceSnapshot)?
    {
        if !accounts.is_empty() {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let bytes = reader.read(b"getCurrentFeeSchedule()", &[])?;
        let fee_schedule = decode_fee_schedule(&bytes)?;
        return Ok(CommittedProgramRecords {
            program: None,
            owner: [0; 32],
            wind_down: None,
            interface: None,
            fee_schedule,
            accounts: BTreeMap::new(),
            exit_routes: BTreeMap::new(),
        });
    }
    let program_view = reader.program(program)?;
    let owner = crate::protocol_evidence::verified_program_owner(node, checkpoint, &program)
        .map_err(|_| ProgramSettlementError::SourceSnapshot)?;
    let wind_down = reader.wind_down(program)?;
    let interface = reader.interface(program)?;
    let bytes = reader.read(b"getCurrentFeeSchedule()", &[])?;
    let fee_schedule = decode_fee_schedule(&bytes)?;
    let mut bindings = BTreeMap::new();
    let mut exit_routes = BTreeMap::new();
    for account in accounts {
        if *account == [0; 32] || bindings.contains_key(account) {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let value = reader.account(program, account, account_proofs)?;
        bindings.insert(*account, value);
        if wind_down.status >= 2 {
            let bytes = reader.read(
                b"getExitRoute(bytes32,bytes32)",
                &[program.as_slice(), account.as_slice()].concat(),
            )?;
            let route = ProgramExitRoute {
                asset: abi_word(&bytes, 0)?,
                destination: abi_word(&bytes, 1)?,
                seed: abi_tail(&bytes, 2, 3, 128)?,
            };
            if route.asset == [0; 32] || route.destination == [0; 32] {
                return Err(ProgramSettlementError::SourceSnapshot);
            }
            exit_routes.insert(*account, route);
        }
    }
    Ok(CommittedProgramRecords {
        program: Some(program_view),
        owner,
        wind_down: Some(wind_down),
        interface: Some(interface),
        fee_schedule,
        accounts: bindings,
        exit_routes,
    })
}

struct ProgramRecordReader<'a> {
    node: &'a SequencerEndpoint,
    checkpoint: &'a crate::read::ServedCheckpoint,
}

impl ProgramRecordReader<'_> {
    fn read(&self, signature: &[u8], arguments: &[u8]) -> Result<Vec<u8>, ProgramSettlementError> {
        let mut calldata =
            layerx_crypto::evm_transaction::transaction_hash(signature)[..4].to_vec();
        calldata.extend(arguments);
        verified_program_record(self.node, self.checkpoint, &calldata)
            .map(|value| value.canonical_state().to_vec())
            .map_err(|_| ProgramSettlementError::SourceSnapshot)
    }

    fn program(&self, program: [u8; 32]) -> Result<ProgramView, ProgramSettlementError> {
        let bytes = self.read(b"getProgram(bytes32)", &program)?;
        if bytes.len() != 192 {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let program_view = ProgramView {
            policy: u8::try_from(abi_uint(&bytes, 0, 1)?)
                .map_err(|_| ProgramSettlementError::SourceSnapshot)?,
            authority: abi_word(&bytes, 1)?,
            code_hash: abi_word(&bytes, 2)?,
            abi_version: u16::try_from(abi_uint(&bytes, 3, 2)?)
                .map_err(|_| ProgramSettlementError::SourceSnapshot)?,
            version: u32::try_from(abi_uint(&bytes, 4, 4)?)
                .map_err(|_| ProgramSettlementError::SourceSnapshot)?,
            active: abi_bool(&bytes, 5)?,
        };
        if !matches!(program_view.policy, 0 | 1)
            || (program_view.policy == 0) != (program_view.authority == [0; 32])
            || program_view.code_hash == [0; 32]
            || !matches!(program_view.abi_version, 1..=4)
            || program_view.version == 0
        {
            return Err(ProgramSettlementError::SourceSnapshot);
        }

        Ok(program_view)
    }

    fn wind_down(&self, program: [u8; 32]) -> Result<WindDownView, ProgramSettlementError> {
        let bytes = self.read(b"getWindDown(bytes32)", &program)?;
        if bytes.len() != 192 {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let wind_down = WindDownView {
            status: u8::try_from(abi_uint(&bytes, 0, 1)?)
                .map_err(|_| ProgramSettlementError::SourceSnapshot)?,
            exit_program: abi_word(&bytes, 1)?,
            deadline: u64::try_from(abi_uint(&bytes, 2, 8)?)
                .map_err(|_| ProgramSettlementError::SourceSnapshot)?,
            effective_sequence: u64::try_from(abi_uint(&bytes, 3, 8)?)
                .map_err(|_| ProgramSettlementError::SourceSnapshot)?,
            account_count: u16::try_from(abi_uint(&bytes, 4, 2)?)
                .map_err(|_| ProgramSettlementError::SourceSnapshot)?,
            live_count: u16::try_from(abi_uint(&bytes, 5, 2)?)
                .map_err(|_| ProgramSettlementError::SourceSnapshot)?,
        };
        if !matches!(wind_down.status, 1..=3) || wind_down.live_count > wind_down.account_count {
            return Err(ProgramSettlementError::SourceSnapshot);
        }

        Ok(wind_down)
    }

    fn interface(&self, program: [u8; 32]) -> Result<InterfaceView, ProgramSettlementError> {
        let bytes = self.read(b"getInterface(bytes32)", &program)?;
        let interface = InterfaceView {
            version: u32::try_from(abi_uint(&bytes, 0, 4)?)
                .map_err(|_| ProgramSettlementError::SourceSnapshot)?,
            digest: abi_word(&bytes, 1)?,
            encoding: abi_tail(&bytes, 2, 3, 1_048_576)?,
        };
        if interface.version != 0
            && <[u8; 32]>::from(Sha256::digest(&interface.encoding)) != interface.digest
        {
            return Err(ProgramSettlementError::SourceSnapshot);
        }

        Ok(interface)
    }

    fn account(
        &self,
        program: [u8; 32],
        account: &[u8; 32],
        account_proofs: &BTreeMap<[u8; 32], layerx_proof::state::VerifiedAccountState>,
    ) -> Result<ProgramAccountView, ProgramSettlementError> {
        let bytes = self.read(b"getProgramAccountById(bytes32)", account)?;
        let owner_program = abi_word(&bytes, 0)?;
        let asset = abi_word(&bytes, 1)?;
        let seed = abi_tail(
            &bytes,
            2,
            3,
            layerx_programs_runtime::MAX_PROGRAM_ACCOUNT_SEED_BYTES,
        )?;
        if owner_program != program
            || asset == [0; 32]
            || !layerx_programs_runtime::derive_program_account(
                ProgramId::new(program).map_err(|_| ProgramSettlementError::SourceSnapshot)?,
                &seed,
            )
            .map_err(|_| ProgramSettlementError::SourceSnapshot)?
            .matches(account)
        {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let mut args = [program.as_slice(), asset.as_slice()].concat();
        let mut offset = [0; 32];
        offset[31] = 96;
        args.extend(offset);
        let mut length = [0; 32];
        length[24..].copy_from_slice(&(seed.len() as u64).to_be_bytes());
        args.extend(length);
        args.extend(&seed);
        args.resize(128 + seed.len().div_ceil(32) * 32, 0);
        let bytes = self.read(b"getProgramAccount(bytes32,bytes32,bytes)", &args)?;
        if bytes.len() != 96 || abi_word(&bytes, 0)? != *account || abi_word(&bytes, 1)? != asset {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let registered_sequence = u64::try_from(abi_uint(&bytes, 2, 8)?)
            .map_err(|_| ProgramSettlementError::SourceSnapshot)?;
        if registered_sequence > self.checkpoint.height {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let bytes = self.read(
            b"getValueAccount(bytes32,bytes32)",
            &[program.as_slice(), account.as_slice()].concat(),
        )?;
        if bytes.len() != 64 {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let observed = decode_program_value_observation(&bytes)?;
        if !observed.exists {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let proof = account_proofs
            .get(account)
            .ok_or(ProgramSettlementError::AccountStatusUnavailable)?;
        let authenticated = proof.account();
        let header = proof.header().header();
        if authenticated.account_id != *account
            || authenticated.kind != 13
            || authenticated.authority_key.is_some()
            || authenticated.asset_id() != asset
            || authenticated.balance() != observed.balance
            || header.batch_number() != self.checkpoint.height
            || header.resulting_state_root() != self.checkpoint.app_hash
        {
            return Err(ProgramSettlementError::SourceSnapshot);
        }
        let value = ProgramAccountView {
            program,
            asset,
            seed,
            registered_sequence,
            balance: observed.balance,
            frozen: authenticated.frozen,
        };

        Ok(value)
    }
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
            payment_denom: "asset".to_owned(),
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
    fn committed_execution_state_reads_program_records_at_prestate_height() {
        use layerx_programs::ProgramValueAccountBinding;
        let height = 7;
        let prestate_height = height - 1;
        let program = ProgramId::new([1; 32]).unwrap_or_else(|error| panic!("{error:?}"));
        let seed = b"vault".to_vec();
        let account = layerx_programs_runtime::derive_program_account(program, &seed)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .bytes();
        let mut binding = ProgramValueAccountBinding {
            record_version: 2,
            program,
            seed,
            account_id: account,
            asset_id: [2; 32],
            registered_sequence: 1,
            registration_event_digest: [0; 32],
        };
        binding.registration_event_digest =
            layerx_programs::program_account_registration_commitment(&binding);
        let binding_value = binding
            .primary_value()
            .unwrap_or_else(|error| panic!("{error:?}"));
        let key = |prefix: &[u8], suffix: &[u8]| {
            let mut bytes = prefix.to_vec();
            bytes.extend_from_slice(suffix);
            bytes
        };
        let program_key = key(b"program\0", &program.bytes());
        let interface_key = key(b"interface\0", &program.bytes());
        let mut deployed = vec![0];
        deployed.extend([0; 32]);
        deployed.extend(Sha256::digest(
            layerx_programs_runtime::test_support::add_module(),
        ));
        deployed.extend(1_u16.to_be_bytes());
        deployed.extend(1_u32.to_be_bytes());
        let mut state_by_height = BTreeMap::from([
            ((height, program_key.clone()), deployed.clone()),
            ((height, interface_key.clone()), vec![1]),
            (
                (prestate_height, key(b"program-account\0r", &account)),
                binding_value.clone(),
            ),
            (
                (prestate_height, binding.primary_key()),
                binding_value.clone(),
            ),
        ]);
        let expected_keys = [
            program_key.clone(),
            key(b"program-owner\0", &program.bytes()),
            key(b"wind-down\0s", &program.bytes()),
            interface_key.clone(),
            b"progfee/active/v1".to_vec(),
            key(b"wind-down\0r", &[program.bytes(), account].concat()),
            key(b"program-account\0r", &account),
            binding.primary_key(),
        ];
        let mut reads = Vec::new();
        let records = collect_program_records(
            program.bytes(),
            &[account],
            prestate_height,
            &mut |key, at| {
                reads.push((at, key.to_vec()));
                Ok(state_by_height.get(&(at, key.to_vec())).cloned())
            },
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(
            reads,
            expected_keys
                .iter()
                .map(|key| (prestate_height, key.clone()))
                .collect::<Vec<_>>()
        );
        assert!(!records.contains_key(&program_key));
        assert!(!records.contains_key(&interface_key));
        assert_eq!(records.get(&binding.primary_key()), Some(&binding_value));
        state_by_height.insert((prestate_height, program_key.clone()), deployed.clone());
        let mut reads = Vec::new();
        let records = collect_program_records(
            program.bytes(),
            &[account],
            prestate_height,
            &mut |key, at| {
                reads.push((at, key.to_vec()));
                Ok(state_by_height.get(&(at, key.to_vec())).cloned())
            },
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(
            reads,
            expected_keys
                .iter()
                .map(|key| (prestate_height, key.clone()))
                .collect::<Vec<_>>()
        );
        assert_eq!(records.get(&program_key), Some(&deployed));
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

/// Committed getValueAccount observations do not authenticate a frozen flag.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ProgramValueObservation {
    pub balance: u128,
    pub exists: bool,
}

/// # Errors
/// Returns an error if the preparation, receipt, source snapshot or retained allocation does
/// not prove the requested settlement.
pub fn decode_program_value_observation(
    bytes: &[u8],
) -> Result<ProgramValueObservation, ProgramSettlementError> {
    if bytes.len() != 64 {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    let balance = abi_uint(bytes, 0, 16)?;
    let exists = abi_bool(bytes, 1)?;
    if !exists && balance != 0 {
        return Err(ProgramSettlementError::SourceSnapshot);
    }
    Ok(ProgramValueObservation { balance, exists })
}

pub(crate) struct CommittedExecutionEvidence<'a> {
    pub(crate) verifier: &'a EvidenceAuthority,
    pub(crate) sequencer_key: [u8; 32],
    pub(crate) correlation: u64,
    pub(crate) authorization: SequencerAuthorization,
}

pub(crate) struct ExecutionReceipt<'a> {
    pub(crate) canonical_receipt: &'a [u8],
    pub(crate) activity_id: [u8; 32],
    pub(crate) global_sequence: u64,
    pub(crate) receipt_ref: [u8; 32],
}
