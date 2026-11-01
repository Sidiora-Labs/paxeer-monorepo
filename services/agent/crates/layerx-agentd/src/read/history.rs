//! Bounded JSON-RPC history pages ordered by block position.

use crate::human_node::HumanNode;
use crate::node_rpc::NodeRpcError;
use layerx_client::lni::transport::FrameTransport;
use layerx_client::rpc::history::{HistoryKind, ReadContext};
use layerx_client::sequencer::{RpcRefusal, SequencerEndpoint, SequencerError};
use serde_json::{json, Value};

#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub struct Cursor {
    pub block_height: u64,
    pub tx_index: u32,
    pub log_index: u32,
}

impl Cursor {
    const TEXT_LENGTH: usize = 32;

    #[must_use]
    pub fn to_hex(&self) -> String {
        let mut bytes = [0_u8; 16];
        bytes[..8].copy_from_slice(&self.block_height.to_be_bytes());
        bytes[8..12].copy_from_slice(&self.tx_index.to_be_bytes());
        bytes[12..].copy_from_slice(&self.log_index.to_be_bytes());
        bytes.iter().map(|byte| format!("{byte:02x}")).collect()
    }

    /// # Errors
    /// Refuses anything except the sixteen position bytes in lowercase hexadecimal.
    pub fn from_hex(text: &str) -> Result<Self, HistoryReadError> {
        if text.len() != Self::TEXT_LENGTH {
            return Err(HistoryReadError::Cursor);
        }
        let mut bytes = [0_u8; 16];
        for (byte, pair) in bytes.iter_mut().zip(text.as_bytes().chunks_exact(2)) {
            *byte = (lower_hex_digit(pair[0])? << 4) | lower_hex_digit(pair[1])?;
        }
        Ok(Self {
            block_height: u64::from_be_bytes(
                bytes[..8]
                    .try_into()
                    .map_err(|_| HistoryReadError::Cursor)?,
            ),
            tx_index: u32::from_be_bytes(
                bytes[8..12]
                    .try_into()
                    .map_err(|_| HistoryReadError::Cursor)?,
            ),
            log_index: u32::from_be_bytes(
                bytes[12..]
                    .try_into()
                    .map_err(|_| HistoryReadError::Cursor)?,
            ),
        })
    }

    fn wire(self) -> Value {
        json!([
            self.block_height.to_string(),
            self.tx_index.to_string(),
            self.log_index.to_string()
        ])
    }

    fn from_wire(value: &Value) -> Result<Self, HistoryReadError> {
        let values = value
            .as_array()
            .filter(|values| values.len() == 3)
            .ok_or(HistoryReadError::Cursor)?;
        Ok(Self {
            block_height: decimal(&values[0]).ok_or(HistoryReadError::Cursor)?,
            tx_index: decimal(&values[1])
                .and_then(|n| u32::try_from(n).ok())
                .ok_or(HistoryReadError::Cursor)?,
            log_index: decimal(&values[2])
                .and_then(|n| u32::try_from(n).ok())
                .ok_or(HistoryReadError::Cursor)?,
        })
    }
}

fn lower_hex_digit(character: u8) -> Result<u8, HistoryReadError> {
    match character {
        b'0'..=b'9' => Ok(character - b'0'),
        b'a'..=b'f' => Ok(character - b'a' + 10),
        _ => Err(HistoryReadError::Cursor),
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct HistoryLimits {
    pub maximum_items: u16,
    pub maximum_response_bytes: usize,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HistoryItem {
    pub position: Cursor,
    pub kind: HistoryKind,
    pub canonical: Vec<u8>,
    pub proof: Option<Vec<u8>>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HistoryPage {
    pub items: Vec<HistoryItem>,
    pub next_cursor: Option<Cursor>,
    pub height: u64,
    pub response_bytes: usize,
}

impl HistoryPage {
    /// # Errors
    /// Refuses malformed, oversized or unordered served pages and preserves RPC refusals.
    pub fn read_account(
        endpoint: &SequencerEndpoint,
        account: &str,
        from_height: u64,
        to_height: u64,
        cursor: Option<Cursor>,
        limits: HistoryLimits,
    ) -> Result<Self, HistoryReadError> {
        validate_request(from_height, to_height, cursor, limits)?;
        if account.is_empty() {
            return Err(HistoryReadError::Malformed);
        }
        let cursor_text = cursor.map(|position| position.to_hex());
        let page = layerx_client::rpc::history::history(
            endpoint,
            account,
            cursor_text.as_deref(),
            Some(limits.maximum_items),
        )
        .map_err(HistoryReadError::Sequencer)?;
        let indexed_height = page.indexed_height.ok_or(HistoryReadError::InvalidRange)?;
        let head = page.chain_height.ok_or(HistoryReadError::InvalidRange)?;
        let height = indexed_height.min(head);
        let to_height = to_height.min(height);
        validate_request(from_height, to_height, cursor, limits)?;
        let response_bytes = serde_json::to_vec(&json!({
            "items": page.items,
            "next_cursor": page.next_cursor,
            "indexedHeight": page.indexed_height,
        }))
        .map_err(|_| HistoryReadError::Malformed)?
        .len();
        if response_bytes > limits.maximum_response_bytes
            || page.items.len() > usize::from(limits.maximum_items)
        {
            return Err(HistoryReadError::Oversized);
        }
        let next_cursor = page
            .next_cursor
            .as_deref()
            .map(Cursor::from_hex)
            .transpose()?;
        let mut items = Vec::with_capacity(page.items.len());
        let mut previous = cursor;
        for row in page.items {
            let position = Cursor {
                block_height: row
                    .get("blockHeight")
                    .and_then(Value::as_u64)
                    .ok_or(HistoryReadError::Malformed)?,
                tx_index: row
                    .get("txIndex")
                    .and_then(Value::as_u64)
                    .and_then(|index| u32::try_from(index).ok())
                    .ok_or(HistoryReadError::Malformed)?,
                log_index: row
                    .get("logIndex")
                    .and_then(Value::as_u64)
                    .and_then(|index| u32::try_from(index).ok())
                    .ok_or(HistoryReadError::Malformed)?,
            };
            if !(from_height..=to_height).contains(&position.block_height)
                || previous.is_some_and(|previous| position <= previous)
            {
                return Err(HistoryReadError::Cursor);
            }
            items.push(HistoryItem {
                position,
                kind: HistoryKind::Event,
                canonical: serde_json::to_vec(&row).map_err(|_| HistoryReadError::Malformed)?,
                proof: None,
            });
            previous = Some(position);
        }
        if next_cursor.is_some_and(|next| items.last().map(|item| item.position) != Some(next)) {
            return Err(HistoryReadError::Cursor);
        }
        Ok(Self {
            items,
            next_cursor,
            height,
            response_bytes,
        })
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum HistoryReadError {
    Node(NodeRpcError),
    Rpc(RpcRefusal),
    Sequencer(SequencerError),
    InvalidLimits,
    InvalidRange,
    Cursor,
    Oversized,
    Malformed,
    ItemUnproven,
}

pub trait HistorySource {
    fn read_history_page(
        &mut self,
        params: Value,
        maximum_response_bytes: usize,
        context: Option<ReadContext>,
    ) -> Result<(Vec<u8>, u64), HistoryReadError>;
}

impl HistorySource for dyn HumanNode + '_ {
    fn read_history_page(
        &mut self,
        _params: Value,
        _maximum_response_bytes: usize,
        _context: Option<ReadContext>,
    ) -> Result<(Vec<u8>, u64), HistoryReadError> {
        Err(HistoryReadError::Sequencer(SequencerError::Unavailable))
    }
}

impl<T: FrameTransport + ?Sized> HistorySource for T {
    fn read_history_page(
        &mut self,
        _params: Value,
        _maximum_response_bytes: usize,
        _context: Option<ReadContext>,
    ) -> Result<(Vec<u8>, u64), HistoryReadError> {
        Err(HistoryReadError::Sequencer(SequencerError::Unavailable))
    }
}

/// # Errors
/// Block-range history is unavailable because the served method requires an account.
pub fn history(
    _node: &mut (impl HistorySource + ?Sized),
    from_height: u64,
    to_height: u64,
    cursor: Option<Cursor>,
    limits: HistoryLimits,
    _context: impl Into<Option<ReadContext>>,
) -> Result<HistoryPage, HistoryReadError> {
    validate_request(from_height, to_height, cursor, limits)?;
    Err(HistoryReadError::Sequencer(SequencerError::Unavailable))
}

fn validate_request(
    from_height: u64,
    to_height: u64,
    cursor: Option<Cursor>,
    limits: HistoryLimits,
) -> Result<(), HistoryReadError> {
    if limits.maximum_items == 0 || limits.maximum_response_bytes == 0 {
        return Err(HistoryReadError::InvalidLimits);
    }
    if from_height > to_height {
        return Err(HistoryReadError::InvalidRange);
    }
    if cursor.is_some_and(|position| !(from_height..=to_height).contains(&position.block_height)) {
        return Err(HistoryReadError::Cursor);
    }
    Ok(())
}

fn decode_page_ordered(
    result: &serde_json::Map<String, Value>,
    from_height: u64,
    to_height: u64,
    cursor: Option<Cursor>,
    limits: HistoryLimits,
    response_bytes: usize,
    newest_first: bool,
) -> Result<HistoryPage, HistoryReadError> {
    if result.len() != 3 {
        return Err(HistoryReadError::Malformed);
    }
    let height = result
        .get("height")
        .and_then(decimal)
        .ok_or(HistoryReadError::Malformed)?;
    let rows = result
        .get("items")
        .and_then(Value::as_array)
        .ok_or(HistoryReadError::Malformed)?;
    if rows.len() > usize::from(limits.maximum_items) {
        return Err(HistoryReadError::Oversized);
    }
    let mut items = Vec::with_capacity(rows.len());
    let mut previous = cursor;
    for row in rows {
        let row = row
            .as_object()
            .filter(|row| row.len() == 6)
            .ok_or(HistoryReadError::Malformed)?;
        let position = Cursor::from_wire(&json!([
            row.get("height"),
            row.get("txIndex"),
            row.get("logIndex")
        ]))?;
        if !(from_height..=to_height).contains(&position.block_height)
            || position.block_height > height
            || previous.is_some_and(|previous| {
                if newest_first {
                    position >= previous
                } else {
                    position <= previous
                }
            })
        {
            return Err(HistoryReadError::Cursor);
        }
        let kind = match row.get("kind").and_then(Value::as_str) {
            Some("Activity") => HistoryKind::Activity,
            Some("Receipt") => HistoryKind::Receipt,
            Some("Event") => HistoryKind::Event,
            _ => return Err(HistoryReadError::Malformed),
        };
        let canonical = hex_bytes(row.get("canonical").ok_or(HistoryReadError::Malformed)?)?;
        let proof = match row.get("proof") {
            Some(Value::Null) => None,
            Some(value) => Some(hex_bytes(value)?),
            None => return Err(HistoryReadError::Malformed),
        };
        items.push(HistoryItem {
            position,
            kind,
            canonical,
            proof,
        });
        previous = Some(position);
    }
    let next_cursor = match result.get("cursor") {
        Some(Value::Null) => None,
        Some(value) => Some(Cursor::from_wire(value)?),
        None => return Err(HistoryReadError::Malformed),
    };
    if next_cursor.is_some_and(|next| items.last().map(|item| item.position) != Some(next)) {
        return Err(HistoryReadError::Cursor);
    }
    Ok(HistoryPage {
        items,
        next_cursor,
        height,
        response_bytes,
    })
}

fn decimal(value: &Value) -> Option<u64> {
    let text = value.as_str()?;
    if text.is_empty()
        || (text.len() > 1 && text.starts_with('0'))
        || !text.bytes().all(|byte| byte.is_ascii_digit())
    {
        return None;
    }
    text.parse().ok()
}

fn hex_bytes(value: &Value) -> Result<Vec<u8>, HistoryReadError> {
    let text = value.as_str().ok_or(HistoryReadError::Malformed)?;
    if text.len() % 2 != 0 {
        return Err(HistoryReadError::Malformed);
    }
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            Ok(
                (lower_hex_digit(pair[0]).map_err(|_| HistoryReadError::Malformed)? << 4)
                    | lower_hex_digit(pair[1]).map_err(|_| HistoryReadError::Malformed)?,
            )
        })
        .collect()
}

impl HistoryReadError {
    pub(crate) const fn code(&self) -> &'static str {
        match self {
            Self::ItemUnproven => "read.history_item_unproven",
            _ => "read.history_unavailable",
        }
    }
}

impl HistoryPage {
    pub(crate) fn read_newest(
        endpoint: &SequencerEndpoint,
        account: &str,
        cursor: Option<Cursor>,
        limits: HistoryLimits,
    ) -> Result<HistoryPage, HistoryReadError> {
        validate_request(0, u64::MAX, cursor, limits)?;
        let cursor_text = cursor.map(|c| c.to_hex());
        let page = endpoint
            .history(
                account,
                cursor_text.as_deref(),
                u64::from(limits.maximum_items),
            )
            .map_err(HistoryReadError::Sequencer)?;
        // An indexer behind the committed head cannot prove a complete usage window.
        let indexed_height = page.indexed_height.ok_or(HistoryReadError::InvalidRange)?;
        if indexed_height == 0 || page.indexed_height != page.chain_height {
            return Err(HistoryReadError::InvalidRange);
        }
        let response_bytes = serde_json::to_vec(&json!({"items": page.items,
            "next_cursor": page.next_cursor, "indexedHeight": page.indexed_height,
            "chainHeight": page.chain_height}))
        .map_err(|_| HistoryReadError::Malformed)?
        .len();
        if response_bytes > limits.maximum_response_bytes {
            return Err(HistoryReadError::Oversized);
        }
        let next = page
            .next_cursor
            .as_deref()
            .map(Cursor::from_hex)
            .transpose()?;
        let result = json!({"height": indexed_height.to_string(), "items": page.items, "cursor": next.map(Cursor::wire)});
        decode_page_ordered(
            result.as_object().ok_or(HistoryReadError::Malformed)?,
            0,
            indexed_height,
            cursor,
            limits,
            response_bytes,
            true,
        )
    }
}
