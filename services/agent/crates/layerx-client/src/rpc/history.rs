//! History reads over JSON-RPC.

use serde_json::{json, Value};

use crate::sequencer::{SequencerEndpoint, SequencerError};

pub type HistoryPage = crate::sequencer::HistoryPage;
pub type PxEndpoint = crate::px::PxEndpoint;

pub fn history(
    endpoint: &SequencerEndpoint,
    account: &str,
    cursor: Option<&str>,
    limit: Option<u16>,
) -> Result<HistoryPage, SequencerError> {
    endpoint.history(account, cursor, u64::from(limit.unwrap_or(50)))
}

pub fn did_evm_address(
    endpoint: &PxEndpoint,
    did: &str,
) -> Result<Option<[u8; 20]>, SequencerError> {
    let account = endpoint.account(did)?;
    if account.account.bound && account.account.layerx_did.as_deref() == Some(did) {
        Ok(account.account.evm_address)
    } else {
        Ok(None)
    }
}

pub fn did_accounts(endpoint: &PxEndpoint, did: &str) -> Result<Vec<Value>, SequencerError> {
    let account = endpoint.account(did)?.account;
    let Some(address) = account.evm_address else {
        return Ok(vec![]);
    };
    if !account.bound || account.layerx_did.as_deref() != Some(did) {
        return Ok(vec![]);
    }
    let hex = |bytes: &[u8]| {
        bytes
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    };
    Ok(vec![json!({
        "evm_address": format!("0x{}", hex(&address)),
        "pax_address": account.pax_address,
        "layerx_did": account.layerx_did,
        "layerx_account": account.layerx_account.as_ref().map(|id| hex(id)),
        "bound": account.bound
    })])
}
