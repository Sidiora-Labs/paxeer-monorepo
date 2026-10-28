//! Balance reads delegated to the proof-verifying LNI client.

use layerx_client::lni::transport::FrameTransport;
use layerx_client::read::ReadContext;
use layerx_types::amount::Amount;

/// Every freshness coordinate required to judge a returned balance.
pub type Freshness = layerx_agent_api::read::Freshness;
pub type Rung = layerx_agent_api::verify::Rung;

/// Core-produced balance bytes with their locally achieved evidence level.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BalanceRead {
    pub account: [u8; 32],
    pub asset: [u8; 32],
    pub amount: Amount,
    pub canonical_bytes: Vec<u8>,
    pub achieved_rung: Rung,
    pub freshness: Freshness,
}

/// Serves a balance only when the LNI response locally proves the requested level.
///
/// # Errors
///
/// Returns the client read failure when the transport fails, the response does not match the
/// request, or the response cannot locally prove the requested verification level.
pub fn balance(
    transport: &mut dyn FrameTransport,
    account: [u8; 32],
    asset: [u8; 32],
    context: ReadContext,
) -> Result<BalanceRead, layerx_agent_api::error::ApiError> {
    let _ = (transport, account, asset, context);
    Err(layerx_agent_api::error::ApiError::unavailable_capability(
        layerx_agent_api::error::RequestId(0),
        "read.balance",
    )
    .expect("static operation name"))
}
