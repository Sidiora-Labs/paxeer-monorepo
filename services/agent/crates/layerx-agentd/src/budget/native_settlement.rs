//! Settlement of retained native approvals against signed node receipts and logs.

use std::collections::BTreeMap;

use layerx_client::sequencer::{SequencerEndpoint, SequencerReceipt};
use layerx_proof::inclusion::SequencerAuthorization;
use serde::Deserialize;
use sha2::{Digest, Sha256};

use crate::approval::native_effect::NativeEffectApprovalCarrier;
use crate::approval::native_program::NativeProgramApprovalCarrier;
use crate::approval::CallApprovalState;
use crate::human::HumanPeer;
use crate::prepare::EvmPrepared;
use crate::sign::VerifiedSubmission;
use crate::store::Store;

use super::program_settlement::{
    ProgramExecutedDebit, ProgramSettlementError as Error, VerifiedProgramDebitSettlement,
};
use super::{CommittedExecutionState, ProgramBudgetReservation, ProgramChargeKind};

#[derive(Deserialize)]
struct RetainedKey {
    id: [u8; 32],
    public_key: [u8; 32],
    first: u64,
    last: u64,
}
#[derive(Deserialize)]
struct RetainedBinding {
    signed: Option<Vec<u8>>,
    queued: bool,
    receipt_keys: Vec<RetainedKey>,
}
#[derive(Deserialize)]
struct RetainedCarrier {
    call_bound: RetainedBinding,
}

pub(super) fn retained_actor_id(peer: &HumanPeer) -> Result<[u8; 32], Error> {
    let subject = peer.subject.as_ref().ok_or(Error::Preparation)?;
    let actor =
        layerx_types::ids::Did::new(subject.owner.as_bytes()).map_err(|_| Error::Preparation)?;
    layerx_wire::hash::did_id_for_protocol(&actor, 3).map_err(|_| Error::Preparation)
}

fn execution(
    endpoint: &SequencerEndpoint,
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
    keys: &[SequencerAuthorization],
) -> Result<
    (
        layerx_wire::receipt::Receipt,
        layerx_client::sequencer::ExecutedReceipt,
    ),
    Error,
> {
    let call = super::native_binding::verify_retained_submission(prepared, submission)
        .map_err(|_| Error::Preparation)?;
    let observed = super::program_settlement::read_native_execution_observation(
        endpoint, prepared, submission,
    )?;
    let SequencerReceipt::Committed {
        receipt: executed,
        block_number,
        block_hash,
    } = observed
    else {
        return Err(Error::Receipt);
    };
    let receipt = layerx_proof::receipt::verify_sequencer_signature(&executed.receipt, keys)
        .map_err(|_| Error::Receipt)?;
    let protocol = receipt.protocol().ok_or(Error::Receipt)?;
    let summary =
        layerx_proof::sequencer_logs::decode_summary(&receipt).map_err(|_| Error::Receipt)?;
    let mut height_word = [0; 32];
    height_word[24..].copy_from_slice(&block_number.to_be_bytes());
    let header = endpoint
        .block_by_number(block_number)
        .map_err(|_| Error::Receipt)?
        .ok_or(Error::Receipt)?;
    if protocol.module_id() != 12
        || protocol.operation() != 2
        || protocol.protocol_version() != layerx_wire::limits::PROTOCOL_VERSION
        || protocol.activity_id() != submission.activity_id()
        || protocol.from()[12..] != prepared.disclosure.actor
        || protocol.from()[..12] != [0; 12]
        || protocol.to()[..12] != [0; 12]
        || protocol.to()[12..] != call.to
        || protocol.debit_sequence() != call.nonce
        || protocol.amount() != 0
        || protocol.global_sequence() != executed.seq
        || executed.height != block_number
        || protocol.batch_id() != height_word
        || header.hash != block_hash
        || block_number <= prepared.observed_head_height
        || block_hash == [0; 32]
        || endpoint.chain_id().map_err(|_| Error::Receipt)? != call.chain_id
        || endpoint.block_number().map_err(|_| Error::Receipt)? < block_number
        || protocol.timestamp() != executed.timestamp
        || (protocol.result_code() == 0) != executed.success
        || summary.gas_used() > call.gas_limit
        || u128::from(summary.effective_gas_price()) > call.max_fee_per_gas
        || summary.sponsor() != [0; 20]
        || protocol.fee_charged() > prepared.disclosure.fee_limit
        || protocol.fee_charged()
            != u128::from(summary.gas_used())
                .checked_mul(u128::from(summary.effective_gas_price()))
                .ok_or(Error::Arithmetic)?
    {
        return Err(Error::Receipt);
    }
    layerx_proof::sequencer_logs::verify_logs(
        &receipt,
        executed
            .logs
            .iter()
            .map(|log| {
                layerx_proof::sequencer_logs::SequencerLog::new(
                    log.address,
                    log.topics.clone(),
                    log.data.clone(),
                    log.log_index,
                )
            })
            .collect(),
    )
    .map_err(|_| Error::Terminal)?;
    Ok((receipt, executed))
}

/// Acquires the block immediately preceding the native execution, including
/// independently authenticated account status for program value accounts.
pub fn committed_native_execution_state_with_account_proofs(
    node: &mut SequencerEndpoint,
    endpoint: &SequencerEndpoint,
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
    caps: &crate::read::CommittedCaps,
    program: [u8; 32],
    accounts: &[[u8; 32]],
    proofs: &BTreeMap<[u8; 32], layerx_proof::state::VerifiedAccountState>,
) -> Result<CommittedExecutionState, Error> {
    let keys = endpoint
        .receipt_keys()
        .map_err(|_| Error::Receipt)?
        .iter()
        .map(|key| {
            SequencerAuthorization::new(
                key.key_id,
                key.public_key,
                key.first_height,
                key.last_height,
            )
        })
        .collect::<Vec<_>>();
    let (receipt, executed) = execution(endpoint, prepared, submission, &keys)?;
    let protocol = receipt.protocol().ok_or(Error::Receipt)?;
    super::execution_state::acquire_execution_state(
        node,
        endpoint,
        caps,
        &executed.receipt,
        submission.activity_id(),
        protocol.global_sequence(),
        Sha256::digest(&executed.receipt).into(),
        program,
        accounts,
        proofs,
    )
}

/// Reads both the durable approval producer and the native receipt consumer.
/// No unsigned, unqueued, mismatched or unreserved debit can settle.
pub fn read_retained_native_call_debit_settlement(
    endpoint: &SequencerEndpoint,
    prepared: &EvmPrepared,
    submission: &VerifiedSubmission,
    store: &Store,
    peer: &HumanPeer,
    prestate: Option<&CommittedExecutionState>,
) -> Result<(ProgramBudgetReservation, VerifiedProgramDebitSettlement), Error> {
    super::native_binding::verify_retained_submission(prepared, submission)
        .map_err(|_| Error::Preparation)?;
    let id: [u8; 32] = Sha256::digest(&prepared.call.encode_unsigned()).into();
    let program =
        prepared.disclosure.activity_type.module() == layerx_types::payload::ModuleId::Programs;
    let (native, reservation, state, encoded) = if program {
        let carrier = NativeProgramApprovalCarrier::read_call(store, peer, id)
            .map_err(|_| Error::Preparation)?;
        (
            carrier.restore_call().map_err(|_| Error::Preparation)?,
            carrier.call_budget().map_err(|_| Error::Allocation)?,
            carrier.call_state().map_err(|_| Error::Preparation)?,
            serde_json::to_vec(&carrier).map_err(|_| Error::Preparation)?,
        )
    } else {
        let carrier = NativeEffectApprovalCarrier::read_call(store, peer, id)
            .map_err(|_| Error::Preparation)?;
        (
            carrier.restore_call().map_err(|_| Error::Preparation)?,
            carrier.call_budget().map_err(|_| Error::Allocation)?,
            carrier.call_state().map_err(|_| Error::Preparation)?,
            serde_json::to_vec(&carrier).map_err(|_| Error::Preparation)?,
        )
    };
    let binding: RetainedCarrier =
        serde_json::from_slice(&encoded).map_err(|_| Error::Preparation)?;
    if state != CallApprovalState::Queued
        || !binding.call_bound.queued
        || binding.call_bound.signed.as_deref() != Some(submission.exact_bytes())
        || native.call.encode_unsigned() != prepared.call.encode_unsigned()
        || native.observed_head_height != prepared.observed_head_height
        || native != *prepared
        || peer.subject.is_none()
    {
        return Err(Error::Preparation);
    }
    let keys = binding
        .call_bound
        .receipt_keys
        .iter()
        .map(|key| SequencerAuthorization::new(key.id, key.public_key, key.first, key.last))
        .collect::<Vec<_>>();
    let (receipt, executed) = execution(endpoint, prepared, submission, &keys)?;
    let protocol = receipt.protocol().ok_or(Error::Receipt)?;
    let actor = retained_actor_id(peer)?;
    if reservation.id != id
        || reservation.allocation_actor() != Some(actor)
        || reservation.allocation_preparation_digest() != Some(id)
        || reservation.allocation_sequence() != Some(prepared.observed_head_height)
        || prepared.observed_head_height.checked_add(1) != Some(executed.height)
    {
        return Err(Error::Allocation);
    }
    let mut debits = if protocol.result_code() == 0 {
        if program {
            let state = prestate.ok_or(Error::SourceSnapshot)?;
            state.validate_binding(
                submission.activity_id(),
                protocol.global_sequence(),
                Sha256::digest(&executed.receipt).into(),
            )?;
            super::program_lifecycle_settlement::native_lifecycle_debits(
                prepared,
                actor,
                state,
                protocol.global_sequence(),
                &executed.logs,
            )?
        } else {
            match crate::capability::derive_native_profile_effects(
                &prepared.disclosure,
                crate::capability::NativeAdmissionProfile::Effect,
            )
            .map_err(|_| Error::Preparation)?
            .effects()
            {
                [crate::capability::Effect::Destruction {
                    account,
                    asset,
                    amount,
                }] => {
                    let destination = layerx_wire::hash::asset_issuance_account_id(asset)
                        .map_err(|_| Error::Preparation)?;
                    let log = executed.logs.first().ok_or(Error::Terminal)?;
                    let mut data = [0; 64];
                    data[16..32].copy_from_slice(&amount.to_be_bytes());
                    data[63] = 11;
                    if executed.logs.len() != 1
                        || log.address != native.call.to
                        || log.topics
                            != [
                                layerx_crypto::evm_transaction::transaction_hash(
                                    b"Transfer(bytes32,bytes32,bytes32,uint128,uint8)",
                                ),
                                *account,
                                destination,
                                *asset,
                            ]
                        || log.data != data
                    {
                        return Err(Error::Terminal);
                    }
                    vec![ProgramExecutedDebit {
                        kind: ProgramChargeKind::Principal,
                        source: *account,
                        asset: *asset,
                        destination: Some(destination),
                        actual_amount: *amount,
                    }]
                }
                _ => return Err(Error::UnsupportedOperation),
            }
        }
    } else {
        if !executed.logs.is_empty() {
            return Err(Error::Terminal);
        }
        Vec::new()
    };
    let fee = super::fee_policy::native_fee_policy_for(
        endpoint,
        prepared.observed_head_height,
        native.from,
    )
    .map_err(|_| Error::FeeProvenance)?;
    if reservation.allocation_state_root() != Some(fee.state_root)
        || protocol.previous_state_root() != fee.state_root
    {
        return Err(Error::SourceSnapshot);
    }
    let rows = reservation.allocations().ok_or(Error::MissingAllocation)?;
    let fee_rows = rows
        .iter()
        .filter(|row| row.kind == ProgramChargeKind::Fee)
        .collect::<Vec<_>>();
    if fee_rows.len() != 1
        || fee_rows[0].asset != fee.value.asset.asset_id
        || fee_rows[0].destination.is_some()
        || protocol.fee_charged() > fee_rows[0].maximum_amount
    {
        return Err(Error::FeeProvenance);
    }
    let summary =
        layerx_proof::sequencer_logs::decode_summary(&receipt).map_err(|_| Error::Receipt)?;
    let denomination_asset = if let Some(hex) = summary.fee_denom().strip_prefix("layerx/") {
        let bytes = layerx_programs::hex::decode(hex).map_err(|_| Error::FeeProvenance)?;
        <[u8; 32]>::try_from(bytes).map_err(|_| Error::FeeProvenance)?
    } else {
        layerx_crypto::evm_transaction::transaction_hash(summary.fee_denom().as_bytes())
    };
    if denomination_asset != fee_rows[0].asset {
        return Err(Error::FeeProvenance);
    }
    if protocol.fee_charged() != 0 {
        debits.push(ProgramExecutedDebit {
            kind: ProgramChargeKind::Fee,
            source: fee_rows[0].source,
            asset: fee_rows[0].asset,
            destination: None,
            actual_amount: protocol.fee_charged(),
        });
    }
    for debit in &debits {
        let row = rows
            .iter()
            .find(|row| {
                row.kind == debit.kind
                    && row.source == debit.source
                    && row.asset == debit.asset
                    && row.destination == debit.destination
            })
            .ok_or(Error::UnreservedDebit)?;
        if debit.actual_amount > row.maximum_amount {
            return Err(Error::UnreservedDebit);
        }
    }
    let witness = VerifiedProgramDebitSettlement::from_native(
        &reservation,
        &executed.receipt,
        submission.activity_id(),
        protocol.global_sequence(),
        debits,
    )?;
    Ok((reservation, witness))
}
