//! Module storage records are witnessed under an authenticated committed header.

use crate::rpc::commit::{acquire_pinned_header, CommitError};
use crate::sequencer::{SequencerEndpoint, SequencerError};
use crate::storage_witness::{verify_storage_witness, StorageWitnessError};
use layerx_crypto::evm_transaction::transaction_hash;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct ModuleReadContext {
    pub precompile: [u8; 20],
    pub committed_height: u64,
    /// Block hash authenticated by the caller's ServedCheckpoint.
    pub block_hash: [u8; 32],
    /// App hash authenticated by the same checkpoint.
    pub app_hash: [u8; 32],
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ModuleStateValue {
    canonical_bytes: Vec<u8>,
    context: ModuleReadContext,
}

impl ModuleStateValue {
    #[must_use]
    pub fn canonical_bytes(&self) -> &[u8] {
        &self.canonical_bytes
    }

    #[must_use]
    pub const fn context(&self) -> &ModuleReadContext {
        &self.context
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum ModuleReadError {
    Rpc(SequencerError),
    SelectorMismatch,
    CommittedChanged,
    Commit(CommitError),
    Witness(StorageWitnessError),
}

pub fn module_slot(precompile: [u8; 20], key: &[u8]) -> Result<[u8; 32], ModuleReadError> {
    let address = u16::from_be_bytes([precompile[18], precompile[19]]);
    let module_id = address
        .checked_sub(0x1000)
        .ok_or(ModuleReadError::SelectorMismatch)?;
    if precompile[..18] != [0; 18] || !(1..=9).contains(&module_id) || key.len() > 129 {
        return Err(ModuleReadError::SelectorMismatch);
    }
    let mut derived = module_id.to_be_bytes().to_vec();
    derived.extend_from_slice(key);
    Ok(transaction_hash(&derived))
}

pub fn module_state(
    endpoint: &SequencerEndpoint,
    key: &[u8],
    context: ModuleReadContext,
) -> Result<ModuleStateValue, ModuleReadError> {
    if context.committed_height == 0 {
        return Err(ModuleReadError::SelectorMismatch);
    }
    let slot = module_slot(context.precompile, key)?;
    let head = acquire_pinned_header(
        endpoint,
        context.committed_height,
        context.block_hash,
        context.app_hash,
    )
    .map_err(ModuleReadError::Commit)?;
    let witnesses = endpoint
        .storage_witness(&context.precompile, &[slot], context.committed_height)
        .map_err(ModuleReadError::Rpc)?;
    let [witness] = witnesses.as_slice() else {
        return Err(ModuleReadError::CommittedChanged);
    };
    if witness.address != context.precompile || witness.slot != slot {
        return Err(ModuleReadError::CommittedChanged);
    }
    verify_storage_witness(witness, head.app_hash()).map_err(ModuleReadError::Witness)?;
    Ok(ModuleStateValue {
        canonical_bytes: witness.value.to_vec(),
        context,
    })
}

pub fn module_state_with_history(
    endpoint: &SequencerEndpoint,
    key: &[u8],
    context: ModuleReadContext,
) -> Result<ModuleStateValue, ModuleReadError> {
    module_state(endpoint, key, context)
}
