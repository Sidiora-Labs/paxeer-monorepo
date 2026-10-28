//! Withdrawal operations over JSON-RPC.

use crate::px::{PxCustodyAsset, PxEndpoint};
use crate::rpc::payments::{self, CallRequest, FeeEstimate};
use crate::sequencer::{SequencerEndpoint, SequencerError};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};

pub type WithdrawalConfigurationError = crate::withdrawal::WithdrawalConfigurationError;

fn estimate_fee(
    endpoint: &SequencerEndpoint,
    call: &CallRequest,
) -> Result<FeeEstimate, WithdrawalConfigurationError> {
    payments::estimate_fee(endpoint, call).map_err(|_| WithdrawalConfigurationError::Unavailable)
}

fn get_asset(
    endpoint: &PxEndpoint,
    asset_id: [u8; 32],
) -> Result<PxCustodyAsset, WithdrawalConfigurationError> {
    let asset = payments::get_asset(endpoint, asset_id).map_err(|error| match error {
        SequencerError::Unavailable => WithdrawalConfigurationError::AssetUnavailable,
        _ => WithdrawalConfigurationError::Unavailable,
    })?;
    let custody = asset
        .paxeer
        .ok_or(WithdrawalConfigurationError::AssetUnavailable)?;
    if custody.asset_id != asset_id
        || !custody.enabled
        || custody.paused
        || custody.pointer == [0; 20]
    {
        return Err(WithdrawalConfigurationError::AssetUnavailable);
    }
    Ok(custody)
}

pub fn registry(
    endpoint: &SequencerEndpoint,
    call: &CallRequest,
    assets: &PxEndpoint,
    asset_id: [u8; 32],
) -> Result<ModuleRegistry, WithdrawalConfigurationError> {
    estimate_fee(endpoint, call)?;
    get_asset(assets, asset_id)?;

    let unsupported = |_| WithdrawalConfigurationError::Unsupported;
    let withdrawal = ActivityType::new(ModuleId::Asset, 9).map_err(unsupported)?;
    let credit = ActivityType::new(ModuleId::Bridge, 1).map_err(unsupported)?;
    let asset = ModuleRegistration::new(ModuleId::Asset, &[withdrawal]).map_err(unsupported)?;
    let bridge = ModuleRegistration::new(ModuleId::Bridge, &[credit]).map_err(unsupported)?;
    ModuleRegistry::new(&[asset, bridge]).map_err(unsupported)
}
