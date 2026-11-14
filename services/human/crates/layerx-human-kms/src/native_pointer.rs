use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
use layerx_crypto::disclosure::call::ledger_selector;

/// Verifies a denomination pointer against the configured chain's pending view.
/// # Errors
/// Refuses unavailable chain reads, malformed ABI words and mismatched pointers.
pub fn verify_native_pointer(
    denom: &str,
    pointer: [u8; 20],
    chain_id: u64,
) -> Result<(), SequencerError> {
    let bank = [
        0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 1,
    ];
    if pointer == [0; 20]
        || pointer == bank
        || denom.is_empty()
        || denom.len() > 128
        || !denom.is_ascii()
    {
        return Err(SequencerError::Malformed);
    }
    let url = std::env::var("LAYERX_AGENT_NODE_RPC_URL").map_err(|_| SequencerError::Malformed)?;
    let endpoint = SequencerEndpoint::new(&url, &[])?;
    if endpoint.chain_id()? != chain_id {
        return Err(SequencerError::Malformed);
    }
    let mut data = ledger_selector("getNativePointer(string)").to_vec();
    let mut word = [0; 32];
    word[31] = 32;
    data.extend(word);
    word[31] = u8::try_from(denom.len()).map_err(|_| SequencerError::Malformed)?;
    data.extend(word);
    data.extend(denom.as_bytes());
    data.resize(4 + (data.len() - 4).div_ceil(32) * 32, 0);
    let mut view = [0; 20];
    view[18..].copy_from_slice(&0x100a_u16.to_be_bytes());
    let answer = endpoint.call_pending(&view, &data)?;
    if answer.len() != 96
        || answer[..12] != [0; 12]
        || answer[12..32] != pointer
        || answer[32..62] != [0; 30]
        || answer[64..95] != [0; 31]
        || answer[95] != 1
    {
        return Err(SequencerError::Malformed);
    }
    Ok(())
}
