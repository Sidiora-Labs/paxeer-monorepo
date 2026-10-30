use layerx_client::sequencer::{SequencerEndpoint, SequencerError};

pub fn agent_boundary_conformance_suite(endpoint: &SequencerEndpoint) -> Result<String, String> {
    let report = crate::gaps::verify_and_render(endpoint)?;
    let head = endpoint
        .block_number()
        .map_err(|error| format!("eth_blockNumber failed: {error:?}"))?;
    let block = endpoint
        .block_by_number(head)
        .map_err(|error| format!("eth_getBlockByNumber failed: {error:?}"))?
        .ok_or_else(|| "node omitted its advertised committed block".to_owned())?;
    if block.number != head {
        return Err("node returned a different committed block height".to_owned());
    }
    if endpoint
        .sequencer_receipt(&[0xff; 32])
        .map_err(|error| format!("absent pax_sequencerReceipt failed: {error:?}"))?
        .is_some()
    {
        return Err("unknown transaction did not return a null sequencer receipt".to_owned());
    }
    match endpoint.send_raw_transaction(&[]) {
        Err(SequencerError::Rpc(_)) => {}
        result => {
            return Err(format!(
                "malformed transaction did not receive a JSON-RPC error: {result:?}"
            ))
        }
    }
    Ok(format!("agent boundary conformance suite passed: node JSON-RPC, committed block {head}, absent receipt, malformed submit\n{report}"))
}
