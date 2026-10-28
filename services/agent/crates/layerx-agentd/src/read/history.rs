//! Bounded JSON-RPC history pages ordered by block position.

use layerx_client::lni::schema::{decode_envelope, encode_envelope, Envelope};
use layerx_client::lni::transport::{FrameTransport, FrameViolation, TransportError};
use layerx_client::read::{HistoryKind, ReadContext, ReadError};
use layerx_client::sequencer::RpcRefusal;
use layerx_types::verify::VerificationLevel;
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

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum HistoryReadError {
    Read(ReadError),
    Rpc(RpcRefusal),
    InvalidLimits,
    InvalidRange,
    Cursor,
    Oversized,
    Malformed,
}

/// Reads a complete bounded px_getHistory page through the LNI transport.
///
/// # Errors
/// Refuses invalid ranges, cursors, malformed or unordered pages and oversized responses.
/// Node transport and JSON-RPC refusals retain their original error.
pub fn history(
    transport: &mut dyn FrameTransport,
    from_height: u64,
    to_height: u64,
    cursor: Option<Cursor>,
    limits: HistoryLimits,
    context: ReadContext,
) -> Result<HistoryPage, HistoryReadError> {
    if limits.maximum_items == 0 || limits.maximum_response_bytes == 0 {
        return Err(HistoryReadError::InvalidLimits);
    }
    if from_height > to_height {
        return Err(HistoryReadError::InvalidRange);
    }
    if cursor.is_some_and(|position| !(from_height..=to_height).contains(&position.block_height)) {
        return Err(HistoryReadError::Cursor);
    }
    if context.requested.level() != VerificationLevel::UNVERIFIED {
        return Err(HistoryReadError::Read(ReadError::MissingEvidence {
            requested: context.requested.level(),
            achieved: VerificationLevel::UNVERIFIED,
        }));
    }
    let payload = serde_json::to_vec(&json!({
        "jsonrpc": "2.0", "id": context.correlation_id, "method": "px_getHistory",
        "params": [{"from_height": from_height.to_string(), "to_height": to_height.to_string(),
            "limit": limits.maximum_items.to_string(), "cursor": cursor.map(Cursor::wire)}]
    }))
    .map_err(|_| HistoryReadError::Malformed)?;
    let request = encode_envelope(Envelope {
        version: context.interface_version,
        message_tag: 9,
        correlation_id: context.correlation_id,
        canonical_payload: &payload,
        proof_material: &[],
    })
    .map_err(|error| HistoryReadError::Read(ReadError::Envelope(error)))?;
    transport.send(&request).map_err(transport_error)?;
    let frame = transport.receive().map_err(transport_error)?;
    let envelope = decode_envelope(&frame)
        .map_err(|error| HistoryReadError::Read(ReadError::Envelope(error)))?;
    if envelope.version.major != context.interface_version.major
        || envelope.correlation_id != context.correlation_id
        || envelope.message_tag != 11
        || !envelope.proof_material.is_empty()
    {
        return Err(HistoryReadError::Read(ReadError::UnexpectedResponse));
    }
    let bytes = envelope.canonical_payload;
    if bytes.len() > limits.maximum_response_bytes {
        return Err(HistoryReadError::Oversized);
    }
    let response: Value = serde_json::from_slice(bytes).map_err(|_| HistoryReadError::Malformed)?;
    let object = response.as_object().ok_or(HistoryReadError::Malformed)?;
    if object.len() != 3
        || response["jsonrpc"] != "2.0"
        || response["id"].as_u64() != Some(context.correlation_id)
    {
        return Err(HistoryReadError::Malformed);
    }
    if let Some(error) = object.get("error") {
        let error = error.as_object().ok_or(HistoryReadError::Malformed)?;
        if error
            .keys()
            .any(|key| !matches!(key.as_str(), "code" | "message" | "data"))
        {
            return Err(HistoryReadError::Malformed);
        }
        let refusal = RpcRefusal {
            code: error
                .get("code")
                .and_then(Value::as_i64)
                .ok_or(HistoryReadError::Malformed)?,
            message: error
                .get("message")
                .and_then(Value::as_str)
                .ok_or(HistoryReadError::Malformed)?
                .to_owned(),
            data: error.get("data").cloned(),
        };
        return Err(HistoryReadError::Rpc(refusal));
    }
    let result = response["result"]
        .as_object()
        .ok_or(HistoryReadError::Malformed)?;
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
            || previous.is_some_and(|previous| position <= previous)
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
        response_bytes: bytes.len(),
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

fn transport_error(error: TransportError) -> HistoryReadError {
    match error {
        TransportError::Frame(FrameViolation::Oversized { .. }) => HistoryReadError::Oversized,
        _ => HistoryReadError::Read(ReadError::Transport(error)),
    }
}
