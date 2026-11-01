//! Payment operations over JSON-RPC.

use crate::px::{PxAssetEntry, PxAssetTable, PxEndpoint};
use crate::sequencer::{SequencerEndpoint, SequencerError};

pub type CallRequest = crate::sequencer::CallRequest;
pub type FeeEstimate = crate::payments::FeeEstimate;

pub fn list_assets(endpoint: &PxEndpoint) -> Result<PxAssetTable, SequencerError> {
    endpoint.list_assets()
}

pub fn get_asset(endpoint: &PxEndpoint, asset: [u8; 32]) -> Result<PxAssetEntry, SequencerError> {
    list_assets(endpoint)?
        .assets
        .into_iter()
        .find(|entry| entry.asset_id == asset)
        .ok_or(SequencerError::Unavailable)
}

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
