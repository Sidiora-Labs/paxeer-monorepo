use layerx_client::sequencer::{CallRequest, SequencerEndpoint};

pub fn verify_and_render(endpoint: &SequencerEndpoint) -> Result<String, String> {
    let chain_id = endpoint
        .chain_id()
        .map_err(|error| format!("eth_chainId unavailable: {error:?}"))?;
    let account = [0; 20];
    endpoint
        .balance(&account)
        .map_err(|error| format!("pending eth_getBalance unavailable: {error:?}"))?;
    endpoint
        .pending_nonce(&account)
        .map_err(|error| format!("pending eth_getTransactionCount unavailable: {error:?}"))?;
    let gas = endpoint
        .estimate_gas(&CallRequest {
            from: Some(account),
            to: Some(account),
            value: 0,
            data: Vec::new(),
        })
        .map_err(|error| format!("pending eth_estimateGas unavailable: {error:?}"))?;
    if gas < 21_000 {
        return Err("plain transfer gas estimate is below intrinsic gas".to_owned());
    }
    let result = endpoint
        .call_pending(&account, &[])
        .map_err(|error| format!("pending eth_call unavailable: {error:?}"))?;
    if !result.is_empty() {
        return Err("empty call to the zero address returned unexpected data".to_owned());
    }
    let after = endpoint
        .chain_id()
        .map_err(|error| format!("repeated eth_chainId unavailable: {error:?}"))?;
    if after != chain_id {
        return Err("node changed chain id during boundary qualification".to_owned());
    }
    Ok(format!("chain_id={chain_id} pending reads exercised: eth_getBalance eth_getTransactionCount eth_estimateGas eth_call"))
}
