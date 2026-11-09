//! History reads over JSON-RPC.

use serde_json::{json, Value};
use std::fmt::Write as _;

use crate::sequencer::{SequencerEndpoint, SequencerError};

/// The registered gateway envelope. Heights, freshness and row finality labels
/// are observations, not locally verified commitment or inclusion evidence.
/// Use `rpc::commit`, `rpc::receipt` and `rpc::module_state` for those proofs.
pub type HistoryPage = crate::sequencer::HistoryPage;
pub type PxEndpoint = crate::px::PxEndpoint;

/// # Errors
///
/// Returns RPC errors or malformed-response errors when the history or account response violates the registered layout.
pub fn history(
    endpoint: &SequencerEndpoint,
    account: &str,
    cursor: Option<&str>,
    limit: Option<u16>,
) -> Result<HistoryPage, SequencerError> {
    endpoint.history(account, cursor, u64::from(limit.unwrap_or(50)))
}

/// # Errors
///
/// Returns RPC errors or malformed-response errors when the history or account response violates the registered layout.
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

/// # Errors
///
/// Returns RPC errors or malformed-response errors when the history or account response violates the registered layout.
pub fn did_accounts(endpoint: &PxEndpoint, did: &str) -> Result<Vec<Value>, SequencerError> {
    let account = endpoint.account(did)?.account;
    let Some(address) = account.evm_address else {
        return Ok(vec![]);
    };
    if !account.bound || account.layerx_did.as_deref() != Some(did) {
        return Ok(vec![]);
    }
    let hex = |bytes: &[u8]| {
        let mut text = String::with_capacity(bytes.len() * 2);
        for byte in bytes {
            let _ = write!(text, "{byte:02x}");
        }
        text
    };
    Ok(vec![json!({
        "evm_address": format!("0x{}", hex(&address)),
        "pax_address": account.pax_address,
        "layerx_did": account.layerx_did,
        "layerx_account": account.layerx_account.as_ref().map(|id| hex(id)),
        "bound": account.bound
    })])
}

/// Compatibility context for daemon signatures; constructed only from verified commits.
pub type ReadContext = crate::rpc::CommittedBlockContext;

/// A gateway row's record class, independent of its proof or finality labels.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum HistoryKind {
    Activity,
    Receipt,
    Event,
}

// The daemon still consumes this offline batch format outside this task's scope.
// It is never decoded or trusted by the registered gateway history read.
#[deprecated(
    note = "offline legacy batch format; use committed headers, receipt commits and storage witnesses for node evidence"
)]
pub use legacy::HistoryProof;

mod legacy;
