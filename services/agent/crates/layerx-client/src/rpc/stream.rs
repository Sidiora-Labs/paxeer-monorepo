//! Stream operations over JSON-RPC.

use serde_json::Value;

use crate::sequencer::SequencerError;
use crate::subscription::{self, SubscriptionTopic};

pub type NodeSubscription = subscription::NodeSubscription;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Cursor {
    pub block_number: u64,
    pub log_index: u64,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Gap {
    pub missing_first: u64,
    pub missing_last: u64,
    backfill_cursor: Cursor,
}

impl Gap {
    #[must_use]
    pub const fn backfill_cursor(self) -> Cursor {
        self.backfill_cursor
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Event {
    pub log: Value,
    cursor: Cursor,
}

impl Event {
    #[must_use]
    pub const fn cursor(&self) -> Cursor {
        self.cursor
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum StreamItem {
    Event(Event),
    Gap(Gap),
}

pub struct EventStream {
    subscription: NodeSubscription,
    cursor: Option<Cursor>,
    stopped: bool,
}

pub fn subscribe(
    url: &str,
    trust_anchors: &[&[u8]],
    topic: SubscriptionTopic,
) -> Result<EventStream, SequencerError> {
    if !matches!(topic, SubscriptionTopic::Logs { .. }) {
        return Err(SequencerError::Malformed);
    }
    Ok(EventStream {
        subscription: subscription::subscribe(url, trust_anchors, topic)?,
        cursor: None,
        stopped: false,
    })
}

impl EventStream {
    #[must_use]
    pub const fn cursor(&self) -> Option<Cursor> {
        self.cursor
    }

    pub fn next_item(&mut self) -> Result<StreamItem, SequencerError> {
        if self.stopped {
            return Err(SequencerError::Unavailable);
        }
        self.stopped = true;
        let log = self.subscription.next()?;
        let cursor = Cursor {
            block_number: quantity(&log, "blockNumber")?,
            log_index: quantity(&log, "logIndex")?,
        };
        if let Some(previous) = self.cursor {
            if cursor.block_number < previous.block_number
                || (cursor.block_number == previous.block_number
                    && cursor.log_index < previous.log_index)
            {
                return Ok(StreamItem::Gap(Gap {
                    missing_first: cursor.log_index.min(previous.log_index),
                    missing_last: cursor.log_index.max(previous.log_index),
                    backfill_cursor: previous,
                }));
            }
        }
        self.cursor = Some(cursor);
        self.stopped = false;
        Ok(StreamItem::Event(Event { log, cursor }))
    }

    pub fn close(&mut self) -> Result<(), SequencerError> {
        self.stopped = true;
        self.subscription.close()
    }
}

fn quantity(log: &Value, field: &str) -> Result<u64, SequencerError> {
    let digits = log
        .get(field)
        .and_then(Value::as_str)
        .and_then(|text| text.strip_prefix("0x"))
        .filter(|digits| {
            !digits.is_empty()
                && digits.len() <= 16
                && (digits.len() == 1 || !digits.starts_with('0'))
                && digits.bytes().all(|byte| byte.is_ascii_hexdigit())
        })
        .ok_or(SequencerError::Malformed)?;
    u64::from_str_radix(digits, 16).map_err(|_| SequencerError::Malformed)
}
