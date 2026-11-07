//! Bounded JSON-RPC history pages ordered by block position.

use crate::human_node::HumanNode;
use crate::node_rpc::NodeRpcError;
use layerx_client::frame::transport::FrameTransport;
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

fn hosted_position(row: &Value) -> Result<Cursor, HistoryReadError> {
    Ok(Cursor {
        block_height: row
            .get("blockHeight")
            .and_then(Value::as_u64)
            .ok_or(HistoryReadError::Malformed)?,
        tx_index: row
            .get("txIndex")
            .and_then(Value::as_u64)
            .and_then(|n| u32::try_from(n).ok())
            .ok_or(HistoryReadError::Malformed)?,
        log_index: row
            .get("logIndex")
            .and_then(Value::as_u64)
            .and_then(|n| u32::try_from(n).ok())
            .ok_or(HistoryReadError::Malformed)?,
    })
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
        // The account cursor is exclusive, so this position skips every older
        // row without consuming a page or splitting the first selected group.
        let forward = cursor.or_else(|| {
            from_height.checked_sub(1).map(|block_height| Cursor {
                block_height,
                tx_index: u32::MAX,
                log_index: u32::MAX,
            })
        });
        let cursor_text = forward.map(|position| position.to_hex());
        let page = layerx_client::rpc::history::history(
            endpoint,
            account,
            cursor_text.as_deref(),
            Some(limits.maximum_items),
        )
        .map_err(HistoryReadError::Sequencer)?;
        if page.account != account {
            return Err(HistoryReadError::Malformed);
        }
        // Validate the entire served page, including rows beyond the requested
        // upper bound, before projecting the finite range.
        let mut page = Self::from_hosted(page, 0, u64::MAX, forward, limits)?;
        if page
            .items
            .iter()
            .any(|item| item.position.block_height > to_height)
        {
            page.next_cursor = None;
        }
        page.items
            .retain(|item| (from_height..=to_height).contains(&item.position.block_height));
        Ok(page)
    }

    /// Decodes the hosted producer's ascending block-position page, including
    /// every row at its final position even when that group crosses the limit.
    pub fn from_hosted(
        page: layerx_client::sequencer::HistoryPage,
        from_height: u64,
        to_height: u64,
        cursor: Option<Cursor>,
        limits: HistoryLimits,
    ) -> Result<Self, HistoryReadError> {
        validate_request(from_height, to_height, cursor, limits)?;
        let indexed_height = page.indexed_height.ok_or(HistoryReadError::InvalidRange)?;
        let head = page.chain_height.ok_or(HistoryReadError::InvalidRange)?;
        if indexed_height > head {
            return Err(HistoryReadError::InvalidRange);
        }
        let height = indexed_height;
        let response_bytes = serde_json::to_vec(&json!({
            "items": page.items,
            "next_cursor": page.next_cursor,
            "indexedHeight": page.indexed_height,
            "chainHeight": page.chain_height,
        }))
        .map_err(|_| HistoryReadError::Malformed)?
        .len();
        if response_bytes > limits.maximum_response_bytes {
            return Err(HistoryReadError::Oversized);
        }
        let next_cursor = page
            .next_cursor
            .as_deref()
            .map(Cursor::from_hex)
            .transpose()?;
        let mut items: Vec<HistoryItem> = Vec::with_capacity(page.items.len());
        let maximum_items = usize::from(limits.maximum_items);
        let mut previous = None;
        for row in page.items {
            let position = hosted_position(&row)?;
            if !(from_height..=to_height.min(height)).contains(&position.block_height)
                || cursor.is_some_and(|cursor| position <= cursor)
                || previous.is_some_and(|previous| position < previous)
            {
                return Err(HistoryReadError::Cursor);
            }
            if items.len() >= maximum_items && position != items[maximum_items - 1].position {
                return Err(HistoryReadError::Oversized);
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
    pub fn read_newest(
        endpoint: &SequencerEndpoint,
        account: &str,
        cursor: Option<Cursor>,
        limits: HistoryLimits,
    ) -> Result<HistoryPage, HistoryReadError> {
        const MAX_SCANNED_ROWS: usize = 4096;
        const MAX_SCANNED_BYTES: usize = 16 * 1024 * 1024;
        validate_request(0, u64::MAX, cursor, limits)?;
        const MAX_REQUESTS: usize = 128;
        const INITIAL_WINDOW: u64 = 64;
        let head = endpoint
            .block_number()
            .map_err(HistoryReadError::Sequencer)?;
        if head == 0 || cursor.is_some_and(|cursor| cursor.block_height > head) {
            return Err(HistoryReadError::InvalidRange);
        }
        let mut upper = cursor.map_or(head.saturating_add(1), |cursor| {
            cursor.block_height.saturating_add(1)
        });
        let mut span = INITIAL_WINDOW;
        let mut lower = upper.saturating_sub(span);
        let mut rows = Vec::new();
        let mut scanned_bytes = 0usize;
        let mut scanned_rows = 0usize;
        let mut requests = 0usize;
        loop {
            let mut forward = lower.checked_sub(1).map(|block_height| Cursor {
                block_height,
                tx_index: u32::MAX,
                log_index: u32::MAX,
            });
            loop {
                requests += 1;
                if requests > MAX_REQUESTS {
                    return Err(HistoryReadError::Oversized);
                }
                let cursor_text = forward.map(|cursor| cursor.to_hex());
                // The producer accepts ascending, exclusive block-position cursors.
                let page = endpoint
                    .history(
                        account,
                        cursor_text.as_deref(),
                        u64::from(limits.maximum_items.min(100)),
                    )
                    .map_err(HistoryReadError::Sequencer)?;
                if page.indexed_height != Some(head) || page.chain_height != Some(head) {
                    return Err(HistoryReadError::InvalidRange);
                }
                let page = Self::from_hosted(
                    page,
                    0,
                    head,
                    forward,
                    HistoryLimits {
                        maximum_items: limits.maximum_items.min(100),
                        maximum_response_bytes: limits.maximum_response_bytes,
                    },
                )?;
                scanned_bytes = scanned_bytes
                    .checked_add(page.response_bytes)
                    .filter(|bytes| *bytes <= MAX_SCANNED_BYTES)
                    .ok_or(HistoryReadError::Oversized)?;
                scanned_rows = scanned_rows
                    .checked_add(page.items.len())
                    .filter(|rows| *rows <= MAX_SCANNED_ROWS)
                    .ok_or(HistoryReadError::Oversized)?;
                let reached_upper = page
                    .items
                    .iter()
                    .any(|item| item.position.block_height >= upper);
                rows.extend(page.items.into_iter().filter(|item| {
                    item.position.block_height < upper
                        && cursor.is_none_or(|cursor| {
                            (item.position.block_height, item.position.tx_index)
                                < (cursor.block_height, cursor.tx_index)
                        })
                }));
                forward = page.next_cursor;
                if reached_upper || forward.is_none() {
                    break;
                }
            }
            let groups = rows
                .iter()
                .map(|item| (item.position.block_height, item.position.tx_index))
                .collect::<std::collections::BTreeSet<_>>()
                .len();
            if lower == 0 || groups > usize::from(limits.maximum_items) {
                break;
            }
            upper = lower;
            span = span.saturating_mul(2);
            lower = upper.saturating_sub(span);
        }
        rows.sort_by(|left, right| right.position.cmp(&left.position));
        let mut selected = Vec::new();
        let mut groups = std::collections::BTreeSet::new();
        let mut next_cursor = None;
        for row in rows {
            let group = (row.position.block_height, row.position.tx_index);
            if !groups.contains(&group) && groups.len() == usize::from(limits.maximum_items) {
                next_cursor = selected.last().map(|last: &HistoryItem| last.position);
                break;
            }
            groups.insert(group);
            selected.push(row);
        }
        let keys = endpoint
            .receipt_keys()
            .map_err(HistoryReadError::Sequencer)?;
        let mut items = Vec::new();
        let mut evidence_bytes = 0usize;
        let mut seen = std::collections::BTreeMap::new();
        for item in selected {
            let row: Value =
                serde_json::from_slice(&item.canonical).map_err(|_| HistoryReadError::Malformed)?;
            let text = row
                .get("tx_hash")
                .and_then(Value::as_str)
                .ok_or(HistoryReadError::Malformed)?;
            let hash = layerx_programs::hex::decode_digest(
                text.strip_prefix("0x").ok_or(HistoryReadError::Malformed)?,
            )
            .map_err(|_| HistoryReadError::Malformed)?;
            // All business rows of a transaction share its authentic receipt.
            if let Some(previous) =
                seen.insert(hash, (item.position.block_height, item.position.tx_index))
            {
                if previous != (item.position.block_height, item.position.tx_index) {
                    return Err(HistoryReadError::ItemUnproven);
                }
                continue;
            }
            let layerx_client::rpc::receipt::Lookup::Verified(receipt) =
                layerx_client::rpc::receipt::lookup_authenticated(
                    endpoint,
                    hash,
                    layerx_client::rpc::receipt::LookupContext {
                        receipt_keys: &keys,
                    },
                )
                .map_err(|_| HistoryReadError::ItemUnproven)?
            else {
                return Err(HistoryReadError::ItemUnproven);
            };
            if receipt.height() != item.position.block_height
                || receipt.execution().index != u64::from(item.position.tx_index)
            {
                return Err(HistoryReadError::ItemUnproven);
            }
            evidence_bytes = evidence_bytes
                .checked_add(receipt.canonical_bytes().len())
                .filter(|bytes| *bytes <= limits.maximum_response_bytes)
                .ok_or(HistoryReadError::Oversized)?;
            items.push(HistoryItem {
                position: item.position,
                kind: HistoryKind::Receipt,
                canonical: receipt.canonical_bytes().to_vec(),
                proof: None,
            });
        }
        Ok(Self {
            items,
            next_cursor,
            height: head,
            response_bytes: evidence_bytes,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::{validate_request, Cursor, HistoryLimits, HistoryReadError};

    #[test]
    fn history_cursor_round_trip_and_canonical_refusals() -> Result<(), HistoryReadError> {
        let cursor = Cursor {
            block_height: u64::MAX,
            tx_index: u32::MAX,
            log_index: 10,
        };
        let encoded = cursor.to_hex();
        assert_eq!(Cursor::from_hex(&encoded)?, cursor);
        assert_eq!(
            Cursor::from_hex(&encoded.to_uppercase()),
            Err(HistoryReadError::Cursor)
        );
        assert_eq!(
            Cursor::from_hex(&encoded[..30]),
            Err(HistoryReadError::Cursor)
        );
        Ok(())
    }

    #[test]
    fn history_request_preserves_finite_limits_and_cursor_range() {
        let limits = HistoryLimits {
            maximum_items: 1,
            maximum_response_bytes: 1,
        };
        let cursor = Cursor {
            block_height: 12,
            tx_index: 0,
            log_index: 0,
        };
        assert_eq!(validate_request(12, 12, Some(cursor), limits), Ok(()));
        assert_eq!(
            validate_request(13, 12, None, limits),
            Err(HistoryReadError::InvalidRange)
        );
        assert_eq!(
            validate_request(13, 14, Some(cursor), limits),
            Err(HistoryReadError::Cursor)
        );
        assert_eq!(
            validate_request(
                0,
                12,
                None,
                HistoryLimits {
                    maximum_items: 0,
                    ..limits
                }
            ),
            Err(HistoryReadError::InvalidLimits)
        );
        assert_eq!(
            validate_request(
                0,
                12,
                None,
                HistoryLimits {
                    maximum_response_bytes: 0,
                    ..limits
                }
            ),
            Err(HistoryReadError::InvalidLimits)
        );
    }
}
