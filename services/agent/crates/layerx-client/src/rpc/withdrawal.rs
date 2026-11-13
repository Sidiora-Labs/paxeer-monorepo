//! Withdrawal operations over JSON-RPC.

use crate::px::PxEndpoint;
use crate::rpc::payments::{self, CallRequest, FeeEstimate};
use crate::sequencer::{SequencerEndpoint, SequencerError};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum WithdrawalConfigurationError {
    Unsupported,
    Unavailable,
    InvalidActivity,
    AssetUnavailable,
}

fn estimate_fee(
    endpoint: &SequencerEndpoint,
    call: &CallRequest,
) -> Result<FeeEstimate, WithdrawalConfigurationError> {
    payments::estimate_fee(endpoint, call).map_err(|_| WithdrawalConfigurationError::Unavailable)
}

const CUSTODY: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x13,
];
const GET_ASSET: [u8; 4] = [0x2c, 0xc3, 0xce, 0x80];

fn get_asset(
    endpoint: &SequencerEndpoint,
    assets: &PxEndpoint,
    asset_id: [u8; 32],
) -> Result<(), WithdrawalConfigurationError> {
    let table =
        payments::list_assets(assets).map_err(|_| WithdrawalConfigurationError::Unavailable)?;
    let asset = table
        .assets
        .iter()
        .find(|entry| decode_asset_id(&entry.asset_id) == Some(asset_id))
        .ok_or(WithdrawalConfigurationError::AssetUnavailable)?;
    if !matches!(asset.kind.as_str(), "layerx" | "ledger") {
        return Err(WithdrawalConfigurationError::Unsupported);
    }
    if table.height == 0 || asset_id == [0; 32] {
        return Err(WithdrawalConfigurationError::AssetUnavailable);
    }
    let mut data = GET_ASSET.to_vec();
    data.extend_from_slice(&asset_id);
    // Metadata does not authorize custody. The native configuration is read at
    // the same height as the gateway table; a retired producer must refuse.
    let answer = endpoint.call_at(table.height, &CUSTODY, &data).map_err(|error| {
        if matches!(&error, SequencerError::Rpc(refusal)
            if refusal.message.contains("LXP_MODULE_DISABLED") && refusal.message.contains("layerxCustody")) {
            WithdrawalConfigurationError::Unsupported
        } else {
            WithdrawalConfigurationError::Unavailable
        }
    })?;
    eligible_configuration(&answer, asset_id)
}

fn decode_asset_id(text: &str) -> Option<[u8; 32]> {
    let digits = text.strip_prefix("0x").unwrap_or(text);
    if digits.len() != 64 || !digits.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return None;
    }
    let mut id = [0; 32];
    for (byte, pair) in id.iter_mut().zip(digits.as_bytes().chunks_exact(2)) {
        *byte = u8::from_str_radix(std::str::from_utf8(pair).ok()?, 16).ok()?;
    }
    Some(id)
}

fn eligible_configuration(
    bytes: &[u8],
    asset_id: [u8; 32],
) -> Result<(), WithdrawalConfigurationError> {
    let unavailable = WithdrawalConfigurationError::Unavailable;
    let word = |offset: usize| -> Result<&[u8], WithdrawalConfigurationError> {
        bytes.get(offset..offset + 32).ok_or(unavailable)
    };
    let number = |offset| -> Result<usize, WithdrawalConfigurationError> {
        let value = word(offset)?;
        if value[..24] != [0; 24] {
            return Err(unavailable);
        }
        usize::try_from(u64::from_be_bytes(
            value[24..].try_into().map_err(|_| unavailable)?,
        ))
        .map_err(|_| unavailable)
    };
    // getAsset returns one dynamic Asset tuple: id, denom, pointer, two
    // booleans and five uint256 counters (precompiles/layerxcustody/abi.json).
    if bytes.len() < 384 || number(0)? != 32 || number(64)? != 320 {
        return Err(unavailable);
    }
    let length = number(352)?;
    if length == 0 || length > 128 {
        return Err(unavailable);
    }
    let end = 384 + length;
    let padded_end = 384 + length.div_ceil(32) * 32;
    if bytes.len() != padded_end
        || bytes[end..padded_end].iter().any(|byte| *byte != 0)
        || std::str::from_utf8(&bytes[384..end]).is_err()
    {
        return Err(unavailable);
    }
    let pointer = word(96)?;
    let enabled = number(128)?;
    let paused = number(160)?;
    if pointer[..12] != [0; 12] || enabled > 1 || paused > 1 {
        return Err(unavailable);
    }
    if word(32)? != asset_id || pointer[12..] == [0; 20] || enabled == 0 || paused == 1 {
        return Err(WithdrawalConfigurationError::AssetUnavailable);
    }
    Ok(())
}

/// # Errors
///
/// Returns configuration errors for an unsupported selector or call, RPC errors, missing-asset errors or invalid custody terms.
pub fn registry(
    endpoint: &SequencerEndpoint,
    call: &CallRequest,
    assets: &PxEndpoint,
    asset_id: [u8; 32],
) -> Result<ModuleRegistry, WithdrawalConfigurationError> {
    estimate_fee(endpoint, call)?;
    get_asset(endpoint, assets, asset_id)?;

    let unsupported = |_| WithdrawalConfigurationError::Unsupported;
    let withdrawal = ActivityType::new(ModuleId::Asset, 9).map_err(unsupported)?;
    let credit = ActivityType::new(ModuleId::Bridge, 1).map_err(unsupported)?;
    let asset = ModuleRegistration::new(ModuleId::Asset, &[withdrawal]).map_err(unsupported)?;
    let bridge = ModuleRegistration::new(ModuleId::Bridge, &[credit]).map_err(unsupported)?;
    ModuleRegistry::new(&[asset, bridge]).map_err(unsupported)
}
