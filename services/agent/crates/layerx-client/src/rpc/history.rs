//! History reads over JSON-RPC.

use serde_json::Value;

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

pub fn did_accounts(endpoint: &PxEndpoint, did: &str) -> Result<Vec<Value>, SequencerError> {
    let account = endpoint.account(did)?;
    let layerx = account.layerx.ok_or(SequencerError::Unavailable)?;
    layerx
        .get("accounts")
        .and_then(Value::as_array)
        .cloned()
        .ok_or(SequencerError::Malformed)
}
