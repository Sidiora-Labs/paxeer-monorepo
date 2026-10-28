//! `read.history` over the version 1 Agent operation envelope.

use layerx_agent_api::error::RequestId;
use layerx_agent_api::verify::{Rung, VerificationStatus};
use serde_json::{json, Value};

use crate::agent_envelope::{AgentEnvelopeTransport, EnvelopeCredential, EnvelopeError};
use crate::rpc_subscription::{decimal, object, violation};
use crate::Operation;

pub use layerx_agent_api::read::{HistoryCursor, HistorySelector};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HistoryItem {
    pub height: u64,
    pub tx_index: u64,
    pub log_index: u64,
    pub kind: u8,
    pub canonical: Vec<u8>,
    pub proof: Option<Vec<u8>>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HistoryPage {
    pub items: Vec<HistoryItem>,
    pub next_cursor: Option<HistoryCursor>,
    pub height: u64,
    pub verification_status: VerificationStatus,
}

pub fn encode_cursor(cursor: &HistoryCursor) -> Value {
    json!([
        cursor.block_height.to_string(),
        cursor.tx_index.to_string(),
        cursor.log_index.to_string()
    ])
}

pub fn decode_cursor(value: &Value) -> Option<HistoryCursor> {
    let values = value.as_array()?;
    if values.len() != 3 {
        return None;
    }
    let number = |i: usize| crate::agent_envelope::canonical_u64(values.get(i)?.as_str()?);
    Some(HistoryCursor {
        block_height: number(0)?,
        tx_index: number(1)?,
        log_index: number(2)?,
    })
}

pub(crate) fn lower_hex(text: &str) -> Option<Vec<u8>> {
    if text.len() % 2 != 0 {
        return None;
    }
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let [high, low] = pair else {
                return None;
            };
            let nibble = |byte: u8| match byte {
                b'0'..=b'9' => Some(byte - b'0'),
                b'a'..=b'f' => Some(byte - b'a' + 10),
                _ => None,
            };
            Some((nibble(*high)? << 4) | nibble(*low)?)
        })
        .collect()
}

fn decode_item(value: &Value, operation: Operation) -> Result<HistoryItem, EnvelopeError> {
    let item = object(
        value,
        &[
            "height",
            "txIndex",
            "logIndex",
            "kind",
            "canonical",
            "proof",
        ],
        operation,
    )?;
    let bytes = |value: &Value| {
        value
            .as_str()
            .and_then(lower_hex)
            .ok_or_else(|| violation(operation))
    };
    Ok(HistoryItem {
        height: decimal(&item["height"], operation)?,
        tx_index: decimal(&item["txIndex"], operation)?,
        log_index: decimal(&item["logIndex"], operation)?,
        kind: u8::try_from(decimal(&item["kind"], operation)?)
            .ok()
            .filter(|kind| matches!(kind, 1..=3))
            .ok_or_else(|| violation(operation))?,
        canonical: bytes(&item["canonical"])?,
        proof: match &item["proof"] {
            Value::Null => None,
            value => Some(bytes(value).and_then(|proof| {
                if proof.is_empty() {
                    Err(violation(operation))
                } else {
                    Ok(proof)
                }
            })?),
        },
    })
}

impl AgentEnvelopeTransport {
    /// Reads one bounded history page over `from_height..=to_height`, resuming from `cursor`.
    ///
    /// # Errors
    ///
    /// Returns the established error envelope, `Transport`, or `Decode` for a below-requested
    /// rung, a malformed cursor or any other schema violation.
    #[allow(clippy::too_many_arguments)]
    pub fn read_history(
        &self,
        request_id: RequestId,
        credential: &EnvelopeCredential,
        from_height: u64,
        to_height: u64,
        cursor: Option<&HistoryCursor>,
        page_limit: u32,
        requested: Rung,
    ) -> Result<HistoryPage, EnvelopeError> {
        let operation = Operation::ReadHistory;
        HistorySelector {
            from_height,
            to_height,
            cursor: cursor.copied(),
            page_limit,
        }
        .validate()
        .map_err(|_| EnvelopeError::InvalidRequest)?;
        let success = self.send_operation(
            operation,
            request_id,
            &json!({
                "range": {"from_height": from_height.to_string(), "to_height": to_height.to_string()},
                "cursor": cursor.map(encode_cursor),
                "page_limit": page_limit.to_string(),
                "requested_rung": requested.name(),
            }),
            Some(credential),
            None,
        )?;
        let read = object(
            &success.value,
            &["value", "achieved_rung", "freshness"],
            operation,
        )?;
        let achieved = crate::rpc_export::rung_from_wire(&read["achieved_rung"], operation)?;
        if achieved < requested
            || !matches!(success.verification_status, VerificationStatus::Achieved(level) if level.rung() == Some(achieved))
        {
            return Err(violation(operation));
        }
        let freshness = crate::rpc_projection::decode_freshness(&read["freshness"], operation)?;
        let page = object(&read["value"], &["items", "cursor", "height"], operation)?;
        let height = decimal(&page["height"], operation)?;
        if height != freshness.value_height {
            return Err(violation(operation));
        }
        let items = page["items"]
            .as_array()
            .ok_or_else(|| violation(operation))?
            .iter()
            .map(|item| decode_item(item, operation))
            .collect::<Result<Vec<_>, _>>()?;
        if u32::try_from(items.len()).map_or(true, |count| count > page_limit) {
            return Err(violation(operation));
        }
        let cursor = match &page["cursor"] {
            Value::Null => None,
            value => Some(decode_cursor(value).ok_or_else(|| violation(operation))?),
        };
        Ok(HistoryPage {
            items,
            next_cursor: cursor,
            height,
            verification_status: success.verification_status,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn block_position_cursor_is_lossless_and_strict() {
        let cursor = HistoryCursor {
            block_height: u64::MAX,
            tx_index: 7,
            log_index: 0,
        };
        assert_eq!(decode_cursor(&encode_cursor(&cursor)), Some(cursor));
        for invalid in [
            json!("00".repeat(56)),
            json!(["1", "2"]),
            json!(["1", "2", "3", "4"]),
            json!([1, "2", "3"]),
            json!(["01", "2", "3"]),
            json!(["18446744073709551616", "2", "3"]),
            json!(["1", " 2", "3"]),
        ] {
            assert_eq!(decode_cursor(&invalid), None);
        }
        for (from_height, to_height, page_limit) in [(2, 1, 1), (1, 2, 0), (1, 2, 65_536)] {
            assert!(HistorySelector {
                from_height,
                to_height,
                page_limit,
                cursor: None
            }
            .validate()
            .is_err());
        }
        assert!(HistorySelector {
            from_height: 1,
            to_height: 1,
            page_limit: 65_535,
            cursor: Some(cursor)
        }
        .validate()
        .is_ok());
    }
}
