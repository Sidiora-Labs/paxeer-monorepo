//! Module records are evaluated by eth_call at the authenticated checkpoint's committed height.

use crate::sequencer::{SequencerEndpoint, SequencerError};

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
}

pub fn module_state(
    endpoint: &SequencerEndpoint,
    calldata: &[u8],
    context: ModuleReadContext,
) -> Result<ModuleStateValue, ModuleReadError> {
    if context.committed_height == 0 || calldata.len() < 4 {
        return Err(ModuleReadError::SelectorMismatch);
    }
    let canonical_bytes = endpoint
        .call_at(context.committed_height, &context.precompile, calldata)
        .map_err(ModuleReadError::Rpc)?;
    let repeated = endpoint
        .call_at(context.committed_height, &context.precompile, calldata)
        .map_err(ModuleReadError::Rpc)?;
    let block = endpoint
        .block_by_number(context.committed_height)
        .map_err(ModuleReadError::Rpc)?
        .ok_or(ModuleReadError::CommittedChanged)?;
    let app_hash = endpoint
        .state_root(context.committed_height)
        .map_err(ModuleReadError::Rpc)?;
    if canonical_bytes != repeated
        || block.hash != context.block_hash
        || app_hash != context.app_hash
    {
        return Err(ModuleReadError::CommittedChanged);
    }
    Ok(ModuleStateValue {
        canonical_bytes,
        context,
    })
}
