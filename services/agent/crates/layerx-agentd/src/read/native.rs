use layerx_client::rpc::{evidence, history, receipt};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint, SequencerError};
use layerx_programs::hex;
use serde_json::{json, Value};

use super::{availability, AvailabilityRead, AvailabilityReadError, AvailabilityRequest, Cursor};

const MAX_RESULT_BYTES: usize = 240 * 1024;
const MAX_HISTORY_ITEMS: u16 = 256;

#[derive(Debug)]
pub enum NativeReadError {
    InvalidRequest,
    CursorMismatch,
    ResultTooLarge,
    Unavailable,
    Verification,
    Connection(SequencerError),
    Receipt(receipt::ReceiptError),
    Availability(AvailabilityReadError),
    Evidence(evidence::EvidenceError),
}

pub struct NativeReadRoute {
    client: SequencerEndpoint,
    receipt_keys: Vec<ReceiptKeyEntry>,
    chain_id: u64,
}

impl NativeReadRoute {
    pub fn new(client: SequencerEndpoint, chain_id: u64) -> Self {
        Self {
            client,
            receipt_keys: Vec::new(),
            chain_id,
        }
    }

    fn check_chain(&self) -> Result<(), NativeReadError> {
        if self
            .client
            .chain_id()
            .map_err(NativeReadError::Connection)?
            != self.chain_id
        {
            return Err(NativeReadError::Verification);
        }
        Ok(())
    }

    /// # Errors
    /// Refuses unavailable or empty published receipt keys without using stale authority.
    pub fn signed_authority(&mut self) -> Result<Vec<ReceiptKeyEntry>, NativeReadError> {
        self.check_chain()?;
        let keys = self
            .client
            .receipt_keys()
            .map_err(NativeReadError::Connection)?;
        if keys.is_empty() {
            return Err(NativeReadError::Verification);
        }
        self.receipt_keys = keys;
        Ok(self.receipt_keys.clone())
    }

    /// # Errors
    /// Refuses malformed selectors, unknown read kinds and invalid node evidence.
    pub fn read(&mut self, path: &str) -> Result<Value, NativeReadError> {
        let (kind, selector) = path
            .strip_prefix("/v1/reads/")
            .and_then(|path| path.split_once('/'))
            .ok_or(NativeReadError::InvalidRequest)?;
        let value = match kind {
            "receipt" => self.receipt(digest(selector)?)?,
            "proof" => self.proof(digest(selector)?)?,
            "checkpoint" => self.checkpoint(number(selector)?)?,
            "availability" => self.availability(number(selector)?)?,
            "history" => self.history(selector)?,
            _ => return Err(NativeReadError::InvalidRequest),
        };
        if serde_json::to_vec(&value)
            .map_err(|_| NativeReadError::Verification)?
            .len()
            > MAX_RESULT_BYTES
        {
            return Err(NativeReadError::ResultTooLarge);
        }
        Ok(value)
    }

    fn receipt(&mut self, transaction_hash: [u8; 32]) -> Result<Value, NativeReadError> {
        self.signed_authority()?;
        let lookup = receipt::lookup_authenticated(
            &self.client,
            transaction_hash,
            receipt::LookupContext {
                receipt_keys: &self.receipt_keys,
            },
        )
        .map_err(NativeReadError::Receipt)?;
        match lookup {
            receipt::Lookup::Absent => Err(NativeReadError::Unavailable),
            receipt::Lookup::Submitted { tx_hash, nonce } => Ok(json!({
                "transaction_hash": hex::encode(&tx_hash), "nonce": nonce,
                "receipt": null, "key_index": null, "rung": "submitted"
            })),
            receipt::Lookup::Verified(verified) => {
                let execution = verified.execution();
                let key_index = self
                    .receipt_keys
                    .iter()
                    .position(|key| {
                        key.key_id == execution.key_id
                            && key.first_height <= execution.height
                            && execution.height <= key.last_height
                    })
                    .ok_or(NativeReadError::Verification)?;
                Ok(json!({
                    "transaction_hash": hex::encode(&transaction_hash),
                    "receipt": hex::encode(verified.canonical_bytes()),
                    "key_index": key_index,
                    "receipt_key": key_json(&self.receipt_keys[key_index]),
                    "rung": layerx_types::rung::Rung::Receipted.name(),
                    "height": verified.height(), "tx_index": execution.index,
                    "freshness": {"value_height": verified.height(), "relative_to": "Block"}
                }))
            }
        }
    }

    fn proof(&mut self, transaction_hash: [u8; 32]) -> Result<Value, NativeReadError> {
        let mut proof = self.receipt(transaction_hash)?;
        if proof["rung"] != "receipted" {
            return Err(NativeReadError::Verification);
        }
        proof["kind"] = json!("receipt");
        proof["proof_type"] = json!("sequencer_signature");
        Ok(proof)
    }

    fn checkpoint(&self, height: u64) -> Result<Value, NativeReadError> {
        self.check_chain()?;
        let block =
            evidence::checkpoint(&self.client, height).map_err(NativeReadError::Evidence)?;
        Ok(json!({
            "height": block.number, "block_hash": hex::encode(&block.hash),
            "parent_hash": hex::encode(&block.parent_hash), "timestamp": block.timestamp,
            "transactions_root": hex::encode(&block.transactions_root),
            "receipts_root": hex::encode(&block.receipts_root),
            "rung": null, "verified": false
        }))
    }

    fn availability(&self, height: u64) -> Result<Value, NativeReadError> {
        self.check_chain()?;
        let result = availability(&self.client, &AvailabilityRequest { height })
            .map_err(NativeReadError::Availability)?;
        Ok(match result {
            AvailabilityRead::Available {
                height,
                transactions_root,
                receipts_root,
                transaction_count,
            } => json!({
                "height": height, "transactions_root": hex::encode(&transactions_root),
                "receipts_root": hex::encode(&receipts_root), "transaction_count": transaction_count,
                "complete": true, "rung": null
            }),
            AvailabilityRead::Pruned { height } => {
                json!({"height": height, "complete": false, "pruned": true, "rung": null})
            }
        })
    }

    fn history(&self, selector: &str) -> Result<Value, NativeReadError> {
        let (account, query) = selector
            .split_once('?')
            .ok_or(NativeReadError::InvalidRequest)?;
        if account.is_empty() || account.contains('/') || account.len() > 256 {
            return Err(NativeReadError::InvalidRequest);
        }
        let (limit, cursor) = history_query(query)?;
        self.check_chain()?;
        let page = history::history(&self.client, account, cursor, Some(limit))
            .map_err(NativeReadError::Connection)?;
        if page.items.len() > usize::from(limit) {
            return Err(NativeReadError::ResultTooLarge);
        }
        if page.indexed_height > page.chain_height {
            return Err(NativeReadError::Verification);
        }
        Ok(json!({
            "account": account, "items": page.items, "cursor": page.next_cursor,
            "complete": page.next_cursor.is_none(), "rung": null,
            "freshness": {"chain_head": page.chain_height,
                "value_height": page.indexed_height, "relative_to": "Block"}
        }))
    }
}

fn key_json(key: &ReceiptKeyEntry) -> Value {
    json!({"key_id": hex::encode(&key.key_id), "public_key": hex::encode(&key.public_key),
        "first_height": key.first_height, "last_height": key.last_height})
}

fn digest(text: &str) -> Result<[u8; 32], NativeReadError> {
    hex::decode_digest(text.strip_prefix("0x").unwrap_or(text))
        .map_err(|_| NativeReadError::InvalidRequest)
}

fn number(text: &str) -> Result<u64, NativeReadError> {
    if text.is_empty()
        || !text.bytes().all(|byte| byte.is_ascii_digit())
        || (text.len() > 1 && text.starts_with('0'))
    {
        return Err(NativeReadError::InvalidRequest);
    }
    text.parse().map_err(|_| NativeReadError::InvalidRequest)
}

fn history_query(query: &str) -> Result<(u16, Option<&str>), NativeReadError> {
    let mut limit = None;
    let mut cursor = None;
    for field in query.split('&') {
        let (key, value) = field
            .split_once('=')
            .ok_or(NativeReadError::InvalidRequest)?;
        match key {
            "limit" if limit.is_none() => {
                limit = Some(
                    u16::try_from(number(value)?)
                        .ok()
                        .filter(|limit| *limit > 0 && *limit <= MAX_HISTORY_ITEMS)
                        .ok_or(NativeReadError::InvalidRequest)?,
                );
            }
            "cursor" if cursor.is_none() => {
                Cursor::from_hex(value).map_err(|_| NativeReadError::CursorMismatch)?;
                cursor = Some(value);
            }
            _ => return Err(NativeReadError::InvalidRequest),
        }
    }
    Ok((limit.ok_or(NativeReadError::InvalidRequest)?, cursor))
}
