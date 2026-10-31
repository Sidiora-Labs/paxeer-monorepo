use layerx_client::rpc::payments::CallRequest;
use layerx_client::rpc::withdrawal::{self, WithdrawalConfigurationError};
use layerx_types::payload::ModuleRegistry;

use super::{refusal, Config, Response};

pub(super) fn registry(
    config: &Config,
    call: &CallRequest,
    asset_id: [u8; 32],
) -> Result<ModuleRegistry, Response> {
    withdrawal::registry(&config.sequencer, call, &config.network, asset_id).map_err(|error| {
        match error {
            WithdrawalConfigurationError::Unsupported => {
                refusal(422, "asset_ordinal_reserved", None)
            }
            WithdrawalConfigurationError::Unavailable => {
                refusal(503, "withdrawal_configuration_unavailable", Some(5))
            }
            WithdrawalConfigurationError::InvalidActivity => {
                refusal(400, "invalid_asset_activity", None)
            }
            WithdrawalConfigurationError::AssetUnavailable => {
                refusal(422, "withdrawal_asset_unavailable", None)
            }
        }
    })
}
