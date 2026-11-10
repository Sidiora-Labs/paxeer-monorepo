//! Payment operations over JSON-RPC.

use crate::px::{PxAssetEntry, PxAssetTable};
use crate::sequencer::SequencerError;

pub type CallRequest = crate::sequencer::CallRequest;
pub type SequencerEndpoint = crate::sequencer::SequencerEndpoint;
pub type PxEndpoint = crate::px::PxEndpoint;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommittedSnapshot<T> {
    pub observed_sequence: u64,
    pub state_root: [u8; 32],
    pub value: T,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AssetMetadata {
    pub asset_id: [u8; 32],
    pub symbol: Vec<u8>,
    pub name: String,
    pub decimals: u8,
    pub custody_kind: u8,
    pub custody_reference: Vec<u8>,
    pub paused: bool,
    pub supply_cap: u128,
    pub issuer_did: [u8; 32],
    pub issuer_kind: u8,
    pub total_units: u128,
    pub salt: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FeeEstimate {
    pub parameter_version: u32,
    pub fee: u128,
    pub canonical_schedule: Vec<u8>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NativeFeePolicy {
    pub version: u8,
    pub asset: AssetMetadata,
}

/// # Errors
///
/// Returns RPC errors or malformed-response errors when asset metadata or the gas estimate cannot be decoded.
pub fn list_assets(endpoint: &PxEndpoint) -> Result<PxAssetTable, SequencerError> {
    endpoint.list_assets()
}

/// # Errors
///
/// Returns RPC errors or malformed-response errors when the asset is absent from the returned table.
pub fn get_asset(
    endpoint: &PxEndpoint,
    asset: impl AsRef<str>,
) -> Result<PxAssetEntry, SequencerError> {
    list_assets(endpoint)?
        .assets
        .into_iter()
        .find(|entry| entry.asset_id == asset.as_ref())
        .ok_or(SequencerError::Unavailable)
}

/// # Errors
///
/// Returns RPC errors or malformed-response errors when asset metadata or the gas estimate cannot be decoded.
pub fn estimate_fee(
    endpoint: &SequencerEndpoint,
    call: &CallRequest,
) -> Result<FeeEstimate, SequencerError> {
    let gas = endpoint.estimate_gas(call)?;
    Ok(FeeEstimate {
        parameter_version: 0,
        fee: u128::from(gas),
        canonical_schedule: Vec::new(),
    })
}
