pub use layerx_client::ai_market::inference_market::*;

use crate::rpc::{RpcClient, RpcError};
use layerx_crypto::disclosure::call::ledger_selector;
use serde_json::{json, Value};
use std::time::Duration;

pub struct InferenceMarket;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct InferenceRead {
    method: &'static str,
    params: Value,
}

impl InferenceRead {
    #[must_use]
    pub const fn method(&self) -> &'static str {
        self.method
    }

    #[must_use]
    pub const fn params(&self) -> &Value {
        &self.params
    }

    /// # Errors
    ///
    /// Preserves RPC transport, timeout, remote refusal and response decoding errors.
    pub fn execute(&self, rpc: &RpcClient) -> Result<Value, RpcError> {
        rpc.call_bounded(self.method, &self.params, Duration::from_secs(30))
    }
}

impl InferenceMarket {
    #[must_use]
    pub fn get_inference_rooms() -> InferenceRead {
        InferenceRead {
            method: "px_getInferenceRooms",
            params: json!([]),
        }
    }

    #[must_use]
    pub fn get_inference_room(room: Digest32) -> InferenceRead {
        read_id("px_getInferenceRoom", room.as_bytes())
    }

    #[must_use]
    pub fn get_inference_deal(deal: Digest32) -> InferenceRead {
        read_id("px_getInferenceDeal", deal.as_bytes())
    }

    #[must_use]
    pub fn get_inference_peer(peer: PrincipalId) -> InferenceRead {
        read_id("px_getInferencePeer", peer.as_bytes())
    }

    #[must_use]
    pub fn get_inference_receipts(deal: Digest32) -> InferenceRead {
        read_id("px_getInferenceReceipts", deal.as_bytes())
    }
}

fn read_id(method: &'static str, id: &[u8; 32]) -> InferenceRead {
    let encoded: String = crate::rpc::encode_hex(id);
    InferenceRead {
        method,
        params: json!([encoded]),
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct AskFunding {
    pub ask: Digest32,
    pub settlement_account: [u8; 20],
    pub rate: u128,
    pub rate_unit: u8,
    pub total_cap: u128,
    pub host_authority_key: [u8; 32],
}

/// # Errors
///
/// Returns `InvalidRequest` for a zero settlement account, rate, rate unit, cap or host authority key.
pub fn fund_ask(request: &AskFunding) -> Result<Vec<u8>, RpcError> {
    if request.settlement_account == [0; 20]
        || request.rate == 0
        || request.rate_unit == 0
        || request.total_cap == 0
        || request.host_authority_key == [0; 32]
    {
        return Err(RpcError::InvalidRequest);
    }
    let mut data =
        ledger_selector("streamOpenForAsk(bytes32,address,uint256,uint8,uint256,bytes32)").to_vec();
    data.extend_from_slice(request.ask.as_bytes());
    data.extend_from_slice(&[0; 12]);
    data.extend_from_slice(&request.settlement_account);
    data.extend_from_slice(&[0; 16]);
    data.extend_from_slice(&request.rate.to_be_bytes());
    data.extend_from_slice(&[0; 31]);
    data.push(request.rate_unit);
    data.extend_from_slice(&[0; 16]);
    data.extend_from_slice(&request.total_cap.to_be_bytes());
    data.extend_from_slice(&request.host_authority_key);
    Ok(data)
}
