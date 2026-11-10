use std::collections::{BTreeMap, VecDeque};
use std::fmt::{Display, Formatter};

use super::gap::{attempt_backfill, BackfillReport, EthLogSource, Gap};
use crate::read::{Cursor, HistoryLimits};
use layerx_client::subscription::NodeSubscription;
use layerx_types::{rung::Rung, verify as legacy};
use serde_json::Value;

use crate::store::{
    EventIngestion, ObjectKind, StorageClass, Store, StoreError, TenantId, TenantKey,
};

const WATERMARK_ID: &[u8] = b"event-ingestion-watermark";
const WATERMARK_MAGIC: &[u8; 4] = b"LXEW";
const METADATA_PREFIX: &[u8] = b"event-evidence:";
const METADATA_MAGIC: &[u8; 4] = b"LXEM";
const ANCHOR_MAGIC: &[u8; 4] = b"LXA2";

/// Immutable block boundary and local counter captured when an event is journaled.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(super) struct CursorAnchor {
    block_height: u64,
    counter: u32,
}

impl CursorAnchor {
    pub(super) fn cursor(self) -> Cursor {
        Cursor {
            block_height: self.block_height,
            tx_index: u32::MAX,
            log_index: self.counter,
        }
    }
}

/// Durable position of the strict global event stream.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Watermark {
    pub last_ingested: Option<u64>,
    pub next_expected: u64,
    pub last_position: Option<Cursor>,
}

/// Exact event bytes and the receipt evidence supplied by the core boundary.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CoreEvent {
    pub global_sequence: u64,
    pub canonical_bytes: Vec<u8>,
    pub receipt_reference: Option<[u8; 32]>,
    pub receipt_verification_level: legacy::VerificationLevel,
    pub attributes: EventAttributes,
}

/// Core-produced dimensions used to narrow an event only after tenant and
/// authority scope have already been established.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct EventAttributes {
    pub agent: String,
    pub account: String,
    pub activity_type: u16,
    pub module: String,
    pub asset: String,
    pub counterparty: String,
    pub result_code: i32,
}

/// Failures that prevent an event from entering the durable ordered stream.
#[derive(Debug)]
pub enum IngestError {
    InvalidCapacity,
    EmptyCoreEvent,
    InvalidAttributes,
    ReceiptEvidenceMismatch,
    Repeated { sequence: u64 },
    OutOfOrder { expected: u64, received: u64 },
    Backpressure { capacity: usize },
    SequenceExhausted,
    CorruptWatermark,
    CorruptEvent,
    Store(StoreError),
    InvalidLog,
    Subscription(layerx_client::sequencer::SequencerError),
    GapBlocked(Gap),
}

impl Display for IngestError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::InvalidLog => formatter.write_str("malformed or removed Ethereum log"),
            Self::Subscription(error) => write!(formatter, "subscription failed: {error:?}"),
            Self::GapBlocked(gap) => write!(
                formatter,
                "missing blocks {} through {}",
                gap.missing_first, gap.missing_last
            ),
            Self::InvalidCapacity => {
                formatter.write_str("event ingestion capacity must be non-zero")
            }
            Self::EmptyCoreEvent => formatter.write_str("core event bytes must not be empty"),
            Self::InvalidAttributes => {
                formatter.write_str("core event filter attributes are invalid")
            }
            Self::ReceiptEvidenceMismatch => {
                formatter.write_str("receipt reference and verification level disagree")
            }
            Self::Repeated { sequence } => {
                write!(formatter, "event sequence {sequence} was repeated")
            }
            Self::OutOfOrder { expected, received } => write!(
                formatter,
                "event sequence {received} arrived out of order; expected {expected}"
            ),
            Self::Backpressure { capacity } => {
                write!(
                    formatter,
                    "event ingestion buffer reached capacity {capacity}"
                )
            }
            Self::SequenceExhausted => formatter.write_str("event sequence space is exhausted"),
            Self::CorruptWatermark => formatter.write_str("durable event watermark is corrupt"),
            Self::CorruptEvent => formatter.write_str("durable event record is corrupt"),
            Self::Store(error) => Display::fmt(error, formatter),
        }
    }
}

impl std::error::Error for IngestError {}

impl From<StoreError> for IngestError {
    fn from(value: StoreError) -> Self {
        Self::Store(value)
    }
}

/// Tenant-scoped bounded event ingestion state.
#[derive(Debug)]
pub struct EventIngestor {
    store: Store,
    tenant: TenantId,
    capacity: usize,
    buffered: VecDeque<CoreEvent>,
    watermark: Watermark,
    pending: BTreeMap<Cursor, CoreEvent>,
    head: Option<u64>,
}

impl EventIngestor {
    /// Opens an ingestor at the durable watermark, or at `initial_sequence`
    /// when the tenant has never ingested an event.
    ///
    /// # Errors
    ///
    /// Refuses a zero capacity, returns `CorruptWatermark` when the persisted watermark is not a
    /// consistent 38-byte record, and returns the store failure otherwise.
    pub fn open(
        mut store: Store,
        tenant: TenantId,
        capacity: usize,
        initial_sequence: u64,
    ) -> Result<Self, IngestError> {
        if capacity == 0 {
            return Err(IngestError::InvalidCapacity);
        }
        let key = watermark_key(tenant.clone())?;
        let watermark = store
            .get(&key)
            .map(|value| decode_watermark(value.bytes()))
            .transpose()?
            .unwrap_or(Watermark {
                last_ingested: None,
                next_expected: initial_sequence,
                last_position: None,
            });
        let head = store
            .get(&head_key(tenant.clone())?)
            .map(|value| {
                let bytes: [u8; 8] = value
                    .bytes()
                    .try_into()
                    .map_err(|_| IngestError::CorruptWatermark)?;
                Ok::<_, IngestError>(u64::from_be_bytes(bytes))
            })
            .transpose()?;
        let mut pending = BTreeMap::new();
        for id in store.list_object_ids(&tenant, ObjectKind::Configuration) {
            let Some(encoded) = id.strip_prefix(b"event-pending:") else {
                continue;
            };
            let position = Cursor::from_hex(
                std::str::from_utf8(encoded).map_err(|_| IngestError::CorruptEvent)?,
            )
            .map_err(|_| IngestError::CorruptEvent)?;
            let key = TenantKey::new(tenant.clone(), ObjectKind::Configuration, id)?;
            let bytes = store.get(&key).ok_or(IngestError::CorruptEvent)?.bytes();
            let event = decode_pending(bytes)?;
            if event.position() != Some(position) {
                return Err(IngestError::CorruptEvent);
            }
            if watermark.last_position.is_some_and(|last| position <= last) {
                let found = durable_sequences(&store, &tenant)?
                    .into_iter()
                    .any(|sequence| {
                        durable_event(&store, &tenant, sequence).is_ok_and(|stored| {
                            same_log_identity(&stored, &event)
                                && reconcile_log(&stored, &event).is_ok()
                        })
                    });
                if !found {
                    return Err(IngestError::CorruptEvent);
                }
                store.remove_local(&key)?;
            } else {
                pending.insert(position, event);
            }
        }
        if pending.len() > capacity {
            return Err(IngestError::Backpressure { capacity });
        }
        Ok(Self {
            pending,
            head,
            store,
            tenant,
            capacity,
            buffered: VecDeque::with_capacity(capacity),
            watermark,
        })
    }

    /// Returns the configured maximum number of undelivered in-memory events.
    #[must_use]
    pub const fn capacity(&self) -> usize {
        self.capacity
    }

    /// Returns the current number of buffered events.
    #[must_use]
    pub fn buffered_len(&self) -> usize {
        self.buffered.len() + self.pending.len()
    }

    /// Returns the last durable position and next required sequence.
    #[must_use]
    pub const fn watermark(&self) -> Watermark {
        self.watermark
    }

    /// Removes the oldest buffered event without changing the durable watermark.
    pub fn take_next(&mut self) -> Option<CoreEvent> {
        self.buffered.pop_front()
    }

    /// Borrows the durable store for exact-byte inspection and downstream reads.
    #[must_use]
    pub const fn store(&self) -> &Store {
        &self.store
    }

    /// Returns the tenant whose single ordered stream this ingestor owns.
    #[must_use]
    pub const fn tenant(&self) -> &TenantId {
        &self.tenant
    }

    /// Reads a retained event with its exact bytes and evidence, including after restart.
    ///
    /// # Errors
    /// Returns `CorruptEvent` for an absent or inconsistent retained record.
    pub fn retained_event(&self, sequence: u64) -> Result<CoreEvent, IngestError> {
        durable_event(&self.store, &self.tenant, sequence)
    }

    /// Consumes the ingestor and returns its durable store.
    #[must_use]
    pub fn into_store(self) -> Store {
        self.store
    }
}

pub(super) fn ingest_event(
    ingestor: &mut EventIngestor,
    event: CoreEvent,
) -> Result<(), IngestError> {
    if event.position().is_some() {
        return ingestor.queue_log(event);
    }
    ingest_with_class(ingestor, event, StorageClass::CoreProducedCache)
}

pub(super) fn ingest_local_event(
    ingestor: &mut EventIngestor,
    event: CoreEvent,
) -> Result<(), IngestError> {
    ingest_with_class(ingestor, event, StorageClass::LocalOnly)
}

fn ingest_with_class(
    ingestor: &mut EventIngestor,
    event: CoreEvent,
    class: StorageClass,
) -> Result<(), IngestError> {
    validate_event(&event)?;
    let expected = ingestor.watermark.next_expected;
    if event.global_sequence < expected {
        return Err(IngestError::Repeated {
            sequence: event.global_sequence,
        });
    }
    if event.global_sequence > expected {
        return Err(IngestError::OutOfOrder {
            expected,
            received: event.global_sequence,
        });
    }
    if ingestor.buffered.len() == ingestor.capacity {
        return Err(IngestError::Backpressure {
            capacity: ingestor.capacity,
        });
    }
    let next_expected = expected
        .checked_add(1)
        .ok_or(IngestError::SequenceExhausted)?;
    let next_watermark = Watermark {
        last_ingested: Some(expected),
        next_expected,
        last_position: event.position().or(ingestor.watermark.last_position),
    };
    let anchor = if event.position().is_none() {
        let block_height = ingestor
            .watermark
            .last_position
            .map_or(0, |p| p.block_height);
        let previous = ingestor
            .watermark
            .last_ingested
            .map(|sequence| durable_anchor(&ingestor.store, &ingestor.tenant, sequence))
            .transpose()?
            .flatten();
        let counter = match previous {
            Some(previous) if previous.block_height == block_height => previous
                .counter
                .checked_add(1)
                .ok_or(IngestError::SequenceExhausted)?,
            _ => 0,
        };
        Some(CursorAnchor {
            block_height,
            counter,
        })
    } else {
        None
    };
    let event_key = event_key(ingestor.tenant.clone(), expected)?;
    let metadata_key = metadata_key(ingestor.tenant.clone(), expected)?;
    let watermark_key = watermark_key(ingestor.tenant.clone())?;
    let ingestion = EventIngestion {
        event_key,
        event_bytes: event.canonical_bytes.clone(),
        metadata_key,
        metadata: encode_anchored_metadata(&event, anchor),
        watermark_key,
        watermark: encode_watermark(next_watermark),
    };
    match class {
        StorageClass::CoreProducedCache => ingestor.store.record_event(ingestion)?,
        StorageClass::LocalOnly => ingestor.store.record_local_event(ingestion)?,
    }
    ingestor.watermark = next_watermark;
    ingestor.buffered.push_back(event);
    Ok(())
}

pub(super) fn durable_sequences(store: &Store, tenant: &TenantId) -> Result<Vec<u64>, IngestError> {
    store
        .list_object_ids(tenant, ObjectKind::Event)
        .into_iter()
        .map(|object_id| {
            let encoded: [u8; 8] = object_id
                .try_into()
                .map_err(|_| IngestError::CorruptEvent)?;
            Ok(u64::from_be_bytes(encoded))
        })
        .collect()
}

pub(super) fn durable_event(
    store: &Store,
    tenant: &TenantId,
    sequence: u64,
) -> Result<CoreEvent, IngestError> {
    let event_key = event_key(tenant.clone(), sequence)?;
    let stored_event = store.get(&event_key).ok_or(IngestError::CorruptEvent)?;
    if !matches!(
        stored_event.class(),
        StorageClass::CoreProducedCache | StorageClass::LocalOnly
    ) || stored_event.bytes().is_empty()
    {
        return Err(IngestError::CorruptEvent);
    }
    let metadata = store
        .get(&metadata_key(tenant.clone(), sequence)?)
        .ok_or(IngestError::CorruptEvent)?;
    if metadata.class() != StorageClass::LocalOnly {
        return Err(IngestError::CorruptEvent);
    }
    let (receipt_reference, receipt_verification_level, attributes, _) =
        decode_metadata(metadata.bytes())?;
    let mut event = CoreEvent {
        global_sequence: sequence,
        canonical_bytes: stored_event.bytes().to_vec(),
        receipt_reference,
        receipt_verification_level,
        attributes,
    };
    if let Ok(log) = decode_log(&event.canonical_bytes, Rung::Receipted) {
        let position = Cursor {
            block_height: log.block_height,
            tx_index: log.tx_index,
            log_index: log.log_index,
        };
        if let Some(evidence) = store.get(&committed_log_key(tenant.clone(), position)?) {
            let committed =
                CoreEvent::from_log_bytes(evidence.bytes().to_vec(), event.attributes.clone())?;
            let level = event.receipt_verification_level;
            event.receipt_verification_level = legacy::VerificationLevel::BATCH_INCLUDED;
            event = reconcile_log(&event, &committed).map_err(|_| IngestError::CorruptEvent)?;
            event.receipt_verification_level = level;
        }
    }
    validate_event(&event).map_err(|_| IngestError::CorruptEvent)?;
    Ok(event)
}

pub(super) fn durable_anchor(
    store: &Store,
    tenant: &TenantId,
    sequence: u64,
) -> Result<Option<CursorAnchor>, IngestError> {
    let metadata = store
        .get(&metadata_key(tenant.clone(), sequence)?)
        .ok_or(IngestError::CorruptEvent)?;
    if metadata.class() != StorageClass::LocalOnly {
        return Err(IngestError::CorruptEvent);
    }
    let (_, _, _, anchor) = decode_metadata(metadata.bytes())?;
    Ok(anchor)
}

fn validate_event(event: &CoreEvent) -> Result<(), IngestError> {
    if event.canonical_bytes.is_empty() {
        return Err(IngestError::EmptyCoreEvent);
    }
    if [
        event.attributes.agent.as_str(),
        event.attributes.account.as_str(),
        event.attributes.module.as_str(),
        event.attributes.asset.as_str(),
        event.attributes.counterparty.as_str(),
    ]
    .iter()
    .any(|value| value.is_empty() || value.len() > 1024 || value.as_bytes().contains(&0))
    {
        return Err(IngestError::InvalidAttributes);
    }
    let has_receipt = event.receipt_reference.is_some();
    let is_verified = event.receipt_verification_level != legacy::VerificationLevel::UNVERIFIED;
    let log_evidence = !has_receipt
        && matches!(
            event.receipt_verification_level,
            legacy::VerificationLevel::BATCH_INCLUDED | legacy::VerificationLevel::COMMITTED
        )
        && event.log().is_ok();
    if has_receipt != is_verified && !log_evidence {
        return Err(IngestError::ReceiptEvidenceMismatch);
    }
    Ok(())
}

fn event_key(tenant: TenantId, sequence: u64) -> Result<TenantKey, StoreError> {
    TenantKey::new(tenant, ObjectKind::Event, sequence.to_be_bytes().to_vec())
}

fn metadata_key(tenant: TenantId, sequence: u64) -> Result<TenantKey, StoreError> {
    let mut object_id = METADATA_PREFIX.to_vec();
    object_id.extend_from_slice(&sequence.to_be_bytes());
    TenantKey::new(tenant, ObjectKind::Configuration, object_id)
}

fn watermark_key(tenant: TenantId) -> Result<TenantKey, StoreError> {
    TenantKey::new(tenant, ObjectKind::Configuration, WATERMARK_ID.to_vec())
}

fn encode_watermark(watermark: Watermark) -> Vec<u8> {
    let mut bytes = Vec::with_capacity(38);
    bytes.extend_from_slice(WATERMARK_MAGIC);
    bytes.push(u8::from(watermark.last_ingested.is_some()));
    bytes.extend_from_slice(&watermark.last_ingested.unwrap_or(0).to_be_bytes());
    bytes.extend_from_slice(&watermark.next_expected.to_be_bytes());
    bytes.push(u8::from(watermark.last_position.is_some()));
    let position = watermark.last_position.unwrap_or(Cursor {
        block_height: 0,
        tx_index: 0,
        log_index: 0,
    });
    bytes.extend_from_slice(&position.block_height.to_be_bytes());
    bytes.extend_from_slice(&position.tx_index.to_be_bytes());
    bytes.extend_from_slice(&position.log_index.to_be_bytes());
    bytes
}

fn decode_watermark(bytes: &[u8]) -> Result<Watermark, IngestError> {
    if bytes.len() != 38 || &bytes[..4] != WATERMARK_MAGIC || bytes[4] > 1 || bytes[21] > 1 {
        return Err(IngestError::CorruptWatermark);
    }
    let mut last = [0_u8; 8];
    last.copy_from_slice(&bytes[5..13]);
    let mut next = [0_u8; 8];
    next.copy_from_slice(&bytes[13..21]);
    let last_ingested = (bytes[4] == 1).then(|| u64::from_be_bytes(last));
    let next_expected = u64::from_be_bytes(next);
    if last_ingested.is_some()
        && last_ingested.and_then(|last| last.checked_add(1)) != Some(next_expected)
    {
        return Err(IngestError::CorruptWatermark);
    }
    let last_position = if bytes[21] == 1 {
        if last_ingested.is_none() {
            return Err(IngestError::CorruptWatermark);
        }
        Some(Cursor {
            block_height: u64::from_be_bytes(
                bytes[22..30]
                    .try_into()
                    .map_err(|_| IngestError::CorruptWatermark)?,
            ),
            tx_index: u32::from_be_bytes(
                bytes[30..34]
                    .try_into()
                    .map_err(|_| IngestError::CorruptWatermark)?,
            ),
            log_index: u32::from_be_bytes(
                bytes[34..38]
                    .try_into()
                    .map_err(|_| IngestError::CorruptWatermark)?,
            ),
        })
    } else {
        if bytes[22..].iter().any(|byte| *byte != 0) {
            return Err(IngestError::CorruptWatermark);
        }
        None
    };
    Ok(Watermark {
        last_ingested,
        next_expected,
        last_position,
    })
}

fn encode_metadata(event: &CoreEvent) -> Vec<u8> {
    let mut bytes = Vec::new();
    bytes.extend_from_slice(METADATA_MAGIC);
    bytes.push(event.receipt_verification_level.wire_rank());
    bytes.push(u8::from(event.receipt_reference.is_some()));
    bytes.extend_from_slice(&event.receipt_reference.unwrap_or([0_u8; 32]));
    push_metadata_string(&mut bytes, &event.attributes.agent);
    push_metadata_string(&mut bytes, &event.attributes.account);
    bytes.extend_from_slice(&event.attributes.activity_type.to_be_bytes());
    push_metadata_string(&mut bytes, &event.attributes.module);
    push_metadata_string(&mut bytes, &event.attributes.asset);
    push_metadata_string(&mut bytes, &event.attributes.counterparty);
    bytes.extend_from_slice(&event.attributes.result_code.to_be_bytes());
    bytes
}

fn encode_anchored_metadata(event: &CoreEvent, anchor: Option<CursorAnchor>) -> Vec<u8> {
    let mut bytes = encode_metadata(event);
    if let Some(anchor) = anchor {
        bytes.extend_from_slice(ANCHOR_MAGIC);
        bytes.extend_from_slice(&anchor.block_height.to_be_bytes());
        bytes.extend_from_slice(&anchor.counter.to_be_bytes());
    }
    bytes
}

fn decode_metadata(bytes: &[u8]) -> Result<DecodedMetadata, IngestError> {
    if bytes.len() < 38 || &bytes[..4] != METADATA_MAGIC || bytes[5] > 1 {
        return Err(IngestError::CorruptEvent);
    }
    let level = match bytes[4] {
        4 => legacy::VerificationLevel::COMMITTED,
        rank => legacy::VerificationLevel::from_wire_rank(rank)
            .map_err(|_| IngestError::CorruptEvent)?,
    };
    let mut reference = [0_u8; 32];
    reference.copy_from_slice(&bytes[6..38]);
    let receipt_reference = (bytes[5] == 1).then_some(reference);
    let has_receipt = receipt_reference.is_some();
    let is_verified = level != legacy::VerificationLevel::UNVERIFIED;
    if has_receipt && !is_verified {
        return Err(IngestError::CorruptEvent);
    }
    let mut decoder = MetadataDecoder::new(&bytes[38..]);
    let attributes = EventAttributes {
        agent: decoder.string()?,
        account: decoder.string()?,
        activity_type: decoder.u16()?,
        module: decoder.string()?,
        asset: decoder.string()?,
        counterparty: decoder.string()?,
        result_code: decoder.i32()?,
    };
    let anchor = if decoder.is_empty() {
        None
    } else {
        if decoder.take(4)? != ANCHOR_MAGIC {
            return Err(IngestError::CorruptEvent);
        }
        let anchor = CursorAnchor {
            block_height: decoder.u64()?,
            counter: decoder.u32()?,
        };
        if !decoder.is_empty() {
            return Err(IngestError::CorruptEvent);
        }
        Some(anchor)
    };
    let event = CoreEvent {
        global_sequence: 0,
        canonical_bytes: vec![1],
        receipt_reference,
        receipt_verification_level: level,
        attributes: attributes.clone(),
    };
    let mut validation = event;
    if validation.receipt_reference.is_none() {
        validation.receipt_verification_level = legacy::VerificationLevel::UNVERIFIED;
    }
    validate_event(&validation).map_err(|_| IngestError::CorruptEvent)?;
    Ok((receipt_reference, level, attributes, anchor))
}

fn push_metadata_string(bytes: &mut Vec<u8>, value: &str) {
    let length = u16::try_from(value.len()).unwrap_or(u16::MAX);
    bytes.extend_from_slice(&length.to_be_bytes());
    bytes.extend_from_slice(value.as_bytes());
}

struct MetadataDecoder<'a> {
    bytes: &'a [u8],
    offset: usize,
}

impl<'a> MetadataDecoder<'a> {
    const fn new(bytes: &'a [u8]) -> Self {
        Self { bytes, offset: 0 }
    }

    fn take(&mut self, length: usize) -> Result<&'a [u8], IngestError> {
        let end = self
            .offset
            .checked_add(length)
            .ok_or(IngestError::CorruptEvent)?;
        let value = self
            .bytes
            .get(self.offset..end)
            .ok_or(IngestError::CorruptEvent)?;
        self.offset = end;
        Ok(value)
    }

    fn u16(&mut self) -> Result<u16, IngestError> {
        let mut value = [0_u8; 2];
        value.copy_from_slice(self.take(2)?);
        Ok(u16::from_be_bytes(value))
    }

    fn u32(&mut self) -> Result<u32, IngestError> {
        Ok(u32::from_be_bytes(
            self.take(4)?
                .try_into()
                .map_err(|_| IngestError::CorruptEvent)?,
        ))
    }

    fn u64(&mut self) -> Result<u64, IngestError> {
        Ok(u64::from_be_bytes(
            self.take(8)?
                .try_into()
                .map_err(|_| IngestError::CorruptEvent)?,
        ))
    }

    fn i32(&mut self) -> Result<i32, IngestError> {
        let mut value = [0_u8; 4];
        value.copy_from_slice(self.take(4)?);
        Ok(i32::from_be_bytes(value))
    }

    fn string(&mut self) -> Result<String, IngestError> {
        let length = usize::from(self.u16()?);
        let value =
            std::str::from_utf8(self.take(length)?).map_err(|_| IngestError::CorruptEvent)?;
        Ok(value.to_owned())
    }

    const fn is_empty(&self) -> bool {
        self.offset == self.bytes.len()
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct EthereumLog {
    pub block_height: u64,
    pub tx_index: u32,
    pub log_index: u32,
    pub block_hash: Option<[u8; 32]>,
    pub seq: Option<u64>,
    pub transaction_hash: Option<[u8; 32]>,
    pub address: [u8; 20],
    pub topics: Vec<[u8; 32]>,
    pub data: Vec<u8>,
    pub rung: Rung,
}

impl CoreEvent {
    /// # Errors
    /// Returns an error if an event, head or block is invalid, out of order or blocked by a gap,
    /// or subscription storage or delivery fails.
    pub fn from_log(log: &Value, attributes: EventAttributes) -> Result<Self, IngestError> {
        let bytes = serde_json::to_vec(log).map_err(|_| IngestError::InvalidLog)?;
        Self::from_log_bytes(bytes, attributes)
    }

    /// # Errors
    /// Returns an error if an event, head or block is invalid, out of order or blocked by a gap,
    /// or subscription storage or delivery fails.
    pub fn from_log_bytes(
        bytes: Vec<u8>,
        attributes: EventAttributes,
    ) -> Result<Self, IngestError> {
        let event = Self {
            global_sequence: 0,
            canonical_bytes: bytes,
            receipt_reference: None,
            receipt_verification_level: legacy::VerificationLevel::BATCH_INCLUDED,
            attributes,
        };
        event.log()?;
        validate_event(&event)?;
        Ok(event)
    }

    #[must_use]
    pub fn position(&self) -> Option<Cursor> {
        self.log().ok().map(|log| Cursor {
            block_height: log.block_height,
            tx_index: log.tx_index,
            log_index: log.log_index,
        })
    }

    #[must_use]
    pub fn rung(&self) -> Option<Rung> {
        self.receipt_verification_level.rung()
    }

    /// # Errors
    /// Returns an error if an event, head or block is invalid, out of order or blocked by a gap,
    /// or subscription storage or delivery fails.
    pub fn log(&self) -> Result<EthereumLog, IngestError> {
        decode_log(
            &self.canonical_bytes,
            self.rung().ok_or(IngestError::InvalidLog)?,
        )
    }
}

pub(super) fn decode_log(bytes: &[u8], rung: Rung) -> Result<EthereumLog, IngestError> {
    let value: Value = serde_json::from_slice(bytes).map_err(|_| IngestError::InvalidLog)?;
    if !value.is_object()
        || value
            .get("removed")
            .is_some_and(|removed| removed != &Value::Bool(false))
    {
        return Err(IngestError::InvalidLog);
    }
    let topics = value["topics"]
        .as_array()
        .filter(|topics| topics.len() <= 4)
        .ok_or(IngestError::InvalidLog)?;
    let seq = value
        .get("seq")
        .map(|_| quantity(&value, "seq"))
        .transpose()?;
    let block_hash = match value.get("blockHash") {
        Some(Value::Null) if seq.is_some() && rung == Rung::Receipted => None,
        Some(hash) => Some(
            log_bytes(hash)?
                .try_into()
                .map_err(|_| IngestError::InvalidLog)?,
        ),
        None => return Err(IngestError::InvalidLog),
    };
    let transaction_hash = value
        .get("transactionHash")
        .map(|hash| {
            log_bytes(hash)?
                .try_into()
                .map_err(|_| IngestError::InvalidLog)
        })
        .transpose()?;
    if block_hash.is_none() && transaction_hash.is_none() {
        return Err(IngestError::InvalidLog);
    }
    Ok(EthereumLog {
        block_height: quantity(&value, "blockNumber")?,
        tx_index: u32::try_from(quantity(&value, "transactionIndex")?)
            .map_err(|_| IngestError::InvalidLog)?,
        log_index: u32::try_from(quantity(&value, "logIndex")?)
            .map_err(|_| IngestError::InvalidLog)?,
        block_hash,
        seq,
        transaction_hash,
        address: log_bytes(&value["address"])?
            .try_into()
            .map_err(|_| IngestError::InvalidLog)?,
        topics: topics
            .iter()
            .map(|topic| {
                log_bytes(topic)?
                    .try_into()
                    .map_err(|_| IngestError::InvalidLog)
            })
            .collect::<Result<_, _>>()?,
        data: log_bytes(&value["data"])?,
        rung,
    })
}

impl EventIngestor {
    /// # Errors
    /// Returns an error if an event, head or block is invalid, out of order or blocked by a gap,
    /// or subscription storage or delivery fails.
    pub fn read_log(
        &mut self,
        subscription: &mut NodeSubscription,
        attributes: EventAttributes,
    ) -> Result<(), IngestError> {
        let log = subscription.next().map_err(IngestError::Subscription)?;
        self.ingest_log(&log, attributes)
    }

    /// # Errors
    /// Returns an error if an event, head or block is invalid, out of order or blocked by a gap,
    /// or subscription storage or delivery fails.
    pub fn ingest_log(
        &mut self,
        log: &Value,
        attributes: EventAttributes,
    ) -> Result<(), IngestError> {
        self.queue_log(CoreEvent::from_log(log, attributes)?)
    }

    fn queue_log(&mut self, event: CoreEvent) -> Result<(), IngestError> {
        validate_event(&event)?;
        crate::reject_if(
            !matches!(event.rung(), Some(Rung::Receipted | Rung::Committed)),
            || IngestError::ReceiptEvidenceMismatch,
        )?;
        let position = event.position().ok_or(IngestError::InvalidLog)?;
        let pending_match = self
            .pending
            .iter()
            .find(|(_, pending)| same_log_identity(pending, &event));
        if let Some((&old_position, pending)) = pending_match {
            let reconciled = reconcile_log(pending, &event)?;
            let new_position = reconciled.position().ok_or(IngestError::InvalidLog)?;
            crate::reject_if(
                old_position != new_position && self.pending.contains_key(&new_position),
                || IngestError::InvalidLog,
            )?;
            if old_position == new_position {
                self.store.put_local(
                    pending_key(self.tenant.clone(), old_position)?,
                    encode_pending(&reconciled)?,
                )?;
            } else {
                self.store.replace_local(
                    &pending_key(self.tenant.clone(), old_position)?,
                    pending_key(self.tenant.clone(), new_position)?,
                    encode_pending(&reconciled)?,
                )?;
            }
            self.pending.remove(&old_position);
            self.pending.insert(new_position, reconciled);
            return Ok(());
        }
        crate::reject_if(self.pending.contains_key(&position), || {
            IngestError::InvalidLog
        })?;
        let stored_match = durable_sequences(&self.store, &self.tenant)?
            .into_iter()
            .map(|sequence| durable_event(&self.store, &self.tenant, sequence))
            .collect::<Result<Vec<_>, _>>()?
            .into_iter()
            .find(|stored| same_log_identity(stored, &event));
        if let Some(stored) = stored_match {
            crate::reject_if(stored.log()?.transaction_hash.is_none(), || {
                IngestError::Repeated {
                    sequence: position.block_height,
                }
            })?;
            let reconciled = reconcile_log(&stored, &event)?;
            self.reconcile_stored_log(&stored, &reconciled)?;
            for buffered in &mut self.buffered {
                if buffered.global_sequence == stored.global_sequence {
                    *buffered = reconciled.clone();
                }
            }
            return Ok(());
        }
        crate::reject_if(
            self.watermark
                .last_position
                .is_some_and(|last| position <= last),
            || IngestError::InvalidLog,
        )?;
        crate::reject_if(
            self.head.is_some_and(|head| position.block_height < head),
            || IngestError::Repeated {
                sequence: position.block_height,
            },
        )?;
        crate::reject_if(self.buffered_len() >= self.capacity, || {
            IngestError::Backpressure {
                capacity: self.capacity,
            }
        })?;
        self.store.put_local(
            pending_key(self.tenant.clone(), position)?,
            encode_pending(&event)?,
        )?;
        self.pending.insert(position, event);
        Ok(())
    }
    fn reconcile_stored_log(
        &mut self,
        stored: &CoreEvent,
        reconciled: &CoreEvent,
    ) -> Result<(), IngestError> {
        if reconciled.log()?.block_hash.is_some() {
            let original = self
                .store
                .get(&event_key(self.tenant.clone(), stored.global_sequence)?)
                .ok_or(IngestError::CorruptEvent)?;
            let original_log = decode_log(original.bytes(), Rung::Receipted)?;
            let original_position = Cursor {
                block_height: original_log.block_height,
                tx_index: original_log.tx_index,
                log_index: original_log.log_index,
            };
            let evidence_key = committed_log_key(self.tenant.clone(), original_position)?;
            let mut watermark = self.watermark;
            if watermark.last_ingested == Some(stored.global_sequence) {
                watermark.last_position = reconciled.position();
            }
            let updates = vec![(
                watermark_key(self.tenant.clone())?,
                encode_watermark(watermark),
            )];
            if self.store.get(&evidence_key).is_some() {
                let mut updates = updates;
                updates.push((evidence_key, reconciled.canonical_bytes.clone()));
                self.store.update_local_batch(updates)?;
            } else {
                self.store.update_local_batch_with_companion(
                    updates,
                    evidence_key,
                    reconciled.canonical_bytes.clone(),
                )?;
            }
            self.watermark = watermark;
        }

        Ok(())
    }

    /// # Errors
    /// Returns an error if an event, head or block is invalid, out of order or blocked by a gap,
    /// or subscription storage or delivery fails.
    pub fn read_head(
        &mut self,
        subscription: &mut NodeSubscription,
        source: &EthLogSource,
        limits: HistoryLimits,
        maximum_records: u32,
        attributes: EventAttributes,
    ) -> Result<(), IngestError> {
        let head = subscription.next().map_err(IngestError::Subscription)?;
        self.new_head(
            quantity(&head, "number")?,
            source,
            limits,
            maximum_records,
            attributes,
        )
    }

    /// # Errors
    /// Returns an error if an event, head or block is invalid, out of order or blocked by a gap,
    /// or subscription storage or delivery fails.
    #[allow(
        clippy::needless_pass_by_value,
        reason = "Preserve the public API signature used by daemon consumers."
    )]
    pub fn new_head(
        &mut self,
        height: u64,
        source: &EthLogSource,
        limits: HistoryLimits,
        maximum_records: u32,
        attributes: EventAttributes,
    ) -> Result<(), IngestError> {
        if let Some(previous) = self.head {
            if height < previous {
                return Err(IngestError::OutOfOrder {
                    expected: previous,
                    received: height,
                });
            }
            if let Some(gap) = Gap::between_heads(previous, height) {
                let report = attempt_backfill(source, gap, limits, maximum_records);
                let BackfillReport::Recovered { events, .. } = report else {
                    return Err(IngestError::GapBlocked(gap));
                };
                let mut recovered = Vec::new();
                let mut additions = 0;
                let mut planned_pending = self.pending.clone();
                for item in events {
                    let event =
                        CoreEvent::from_log_bytes(item.canonical_bytes, attributes.clone())?;
                    if let Some(pending) = self
                        .pending
                        .values()
                        .find(|pending| same_log_identity(pending, &event))
                    {
                        reconcile_log(pending, &event)?;
                    } else if self
                        .watermark
                        .last_position
                        .is_some_and(|last| item.position.block_height <= last.block_height)
                    {
                        let stored = durable_sequences(&self.store, &self.tenant)?
                            .into_iter()
                            .map(|sequence| durable_event(&self.store, &self.tenant, sequence))
                            .collect::<Result<Vec<_>, _>>()?
                            .into_iter()
                            .find(|stored| same_log_identity(stored, &event))
                            .ok_or(IngestError::InvalidLog)?;
                        reconcile_log(&stored, &event)?;
                    } else {
                        additions += 1;
                    }
                    let planned_match = planned_pending
                        .iter()
                        .find(|(_, pending)| same_log_identity(pending, &event))
                        .map(|(&position, pending)| (position, pending.clone()));
                    if let Some((old_position, pending)) = planned_match {
                        let merged = reconcile_log(&pending, &event)?;
                        planned_pending.remove(&old_position);
                        let new_position = merged.position().ok_or(IngestError::InvalidLog)?;
                        if planned_pending.insert(new_position, merged).is_some() {
                            return Err(IngestError::InvalidLog);
                        }
                    } else if self
                        .watermark
                        .last_position
                        .is_none_or(|last| item.position.block_height > last.block_height)
                        && planned_pending
                            .insert(item.position, event.clone())
                            .is_some()
                    {
                        return Err(IngestError::InvalidLog);
                    }
                    recovered.push(event);
                }
                if self.buffered_len().saturating_add(additions) > self.capacity {
                    return Err(IngestError::Backpressure {
                        capacity: self.capacity,
                    });
                }
                for event in recovered {
                    self.queue_log(event)?;
                }
            }
        }
        while let Some((&position, event)) = self.pending.first_key_value() {
            if position.block_height >= height {
                break;
            }
            let mut event = event.clone();
            event.global_sequence = self.watermark.next_expected;
            ingest_with_class(self, event, StorageClass::CoreProducedCache)?;
            self.pending.remove(&position);
            self.store
                .remove_local(&pending_key(self.tenant.clone(), position)?)?;
        }
        self.store.put_local(
            head_key(self.tenant.clone())?,
            height.to_be_bytes().to_vec(),
        )?;
        self.head = Some(height);
        Ok(())
    }

    /// # Errors
    /// Returns an error if an event, head or block is invalid, out of order or blocked by a gap,
    /// or subscription storage or delivery fails.
    pub fn commit_block(
        &mut self,
        checkpoint: &crate::read::ServedCheckpoint,
    ) -> Result<(), IngestError> {
        if checkpoint.rung != Rung::Committed
            || checkpoint.commit.height != checkpoint.height
            || checkpoint.commit.block_id.hash != checkpoint.block_hash
            || crate::read::checkpoint(
                checkpoint.height,
                &checkpoint.header.chain_id,
                &checkpoint.header,
                &checkpoint.commit,
                &checkpoint.validators,
            )
            .is_err()
        {
            return Err(IngestError::InvalidLog);
        }
        let mut updates = Vec::new();
        for sequence in durable_sequences(&self.store, &self.tenant)? {
            let mut event = durable_event(&self.store, &self.tenant, sequence)?;
            if let Ok(log) = event.log() {
                if log.block_height == checkpoint.height {
                    if log.block_hash != Some(checkpoint.block_hash) {
                        return Err(IngestError::InvalidLog);
                    }
                    event.receipt_verification_level = legacy::VerificationLevel::COMMITTED;
                    let key = metadata_key(self.tenant.clone(), sequence)?;
                    let stored = self.store.get(&key).ok_or(IngestError::CorruptEvent)?;
                    let (_, _, _, anchor) = decode_metadata(stored.bytes())?;
                    updates.push((key, encode_anchored_metadata(&event, anchor)));
                }
            }
        }
        for event in self.pending.values().chain(self.buffered.iter()) {
            if let Ok(log) = event.log() {
                if log.block_height == checkpoint.height
                    && log.block_hash != Some(checkpoint.block_hash)
                {
                    return Err(IngestError::InvalidLog);
                }
            }
        }
        for (&position, event) in &self.pending {
            if position.block_height == checkpoint.height {
                let mut promoted = event.clone();
                promoted.receipt_verification_level = legacy::VerificationLevel::COMMITTED;
                updates.push((
                    pending_key(self.tenant.clone(), position)?,
                    encode_pending(&promoted)?,
                ));
            }
        }
        self.store.update_local_batch(updates)?;
        for event in self.pending.values_mut().chain(self.buffered.iter_mut()) {
            if event
                .position()
                .is_some_and(|position| position.block_height == checkpoint.height)
            {
                event.receipt_verification_level = legacy::VerificationLevel::COMMITTED;
            }
        }
        Ok(())
    }
}

fn same_log_identity(left: &CoreEvent, right: &CoreEvent) -> bool {
    match (left.log(), right.log()) {
        (Ok(left), Ok(right)) => {
            left.block_height == right.block_height
                && left.log_index == right.log_index
                && match (left.transaction_hash, right.transaction_hash) {
                    (Some(a), Some(b)) => a == b,
                    (None, None) => left.tx_index == right.tx_index,
                    _ => false,
                }
        }
        _ => false,
    }
}

fn reconcile_log(stored: &CoreEvent, incoming: &CoreEvent) -> Result<CoreEvent, IngestError> {
    let left = stored.log()?;
    let right = incoming.log()?;
    if !same_log_identity(stored, incoming)
        || (left.block_hash.is_some() == right.block_hash.is_some()
            && left.tx_index != right.tx_index)
        || left.address != right.address
        || left.topics != right.topics
        || left.data != right.data
        || left.transaction_hash != right.transaction_hash
        || (left.transaction_hash.is_none() && stored.canonical_bytes != incoming.canonical_bytes)
        || matches!((left.seq, right.seq), (Some(a), Some(b)) if a != b)
        || matches!((left.block_hash, right.block_hash), (Some(a), Some(b)) if a != b)
    {
        return Err(IngestError::InvalidLog);
    }
    let mut event = stored.clone();
    if left.block_hash.is_none() && right.block_hash.is_some() {
        event.canonical_bytes.clone_from(&incoming.canonical_bytes);
    }
    Ok(event)
}

fn committed_log_key(tenant: TenantId, position: Cursor) -> Result<TenantKey, StoreError> {
    let mut id = b"event-committed:".to_vec();
    id.extend_from_slice(position.to_hex().as_bytes());
    TenantKey::new(tenant, ObjectKind::Configuration, id)
}

fn head_key(tenant: TenantId) -> Result<TenantKey, StoreError> {
    TenantKey::new(
        tenant,
        ObjectKind::Configuration,
        b"event-ingestion-head".to_vec(),
    )
}

fn quantity(value: &Value, name: &str) -> Result<u64, IngestError> {
    let digits = value[name]
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .filter(|digits| {
            !digits.is_empty()
                && digits.len() <= 16
                && (digits.len() == 1 || !digits.starts_with('0'))
                && digits.bytes().all(|byte| byte.is_ascii_hexdigit())
        })
        .ok_or(IngestError::InvalidLog)?;
    u64::from_str_radix(digits, 16).map_err(|_| IngestError::InvalidLog)
}

fn log_bytes(value: &Value) -> Result<Vec<u8>, IngestError> {
    let text = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .filter(|text| text.len() % 2 == 0)
        .ok_or(IngestError::InvalidLog)?;
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let digit = |byte: u8| {
                (byte as char)
                    .to_digit(16)
                    .and_then(|digit| u8::try_from(digit).ok())
                    .ok_or(IngestError::InvalidLog)
            };
            Ok((digit(pair[0])? << 4) | digit(pair[1])?)
        })
        .collect()
}

fn pending_key(tenant: TenantId, position: Cursor) -> Result<TenantKey, StoreError> {
    let mut id = b"event-pending:".to_vec();
    id.extend_from_slice(position.to_hex().as_bytes());
    TenantKey::new(tenant, ObjectKind::Configuration, id)
}

fn encode_pending(event: &CoreEvent) -> Result<Vec<u8>, IngestError> {
    let length = u32::try_from(event.canonical_bytes.len()).map_err(|_| IngestError::InvalidLog)?;
    let mut bytes = length.to_be_bytes().to_vec();
    bytes.extend_from_slice(&event.canonical_bytes);
    bytes.extend_from_slice(&encode_metadata(event));
    Ok(bytes)
}

fn decode_pending(bytes: &[u8]) -> Result<CoreEvent, IngestError> {
    let length = u32::from_be_bytes(
        bytes
            .get(..4)
            .ok_or(IngestError::CorruptEvent)?
            .try_into()
            .map_err(|_| IngestError::CorruptEvent)?,
    ) as usize;
    let end = 4_usize
        .checked_add(length)
        .ok_or(IngestError::CorruptEvent)?;
    let canonical_bytes = bytes.get(4..end).ok_or(IngestError::CorruptEvent)?.to_vec();
    let (receipt_reference, receipt_verification_level, attributes, _) =
        decode_metadata(bytes.get(end..).ok_or(IngestError::CorruptEvent)?)?;
    let event = CoreEvent {
        global_sequence: 0,
        canonical_bytes,
        receipt_reference,
        receipt_verification_level,
        attributes,
    };
    validate_event(&event).map_err(|_| IngestError::CorruptEvent)?;
    event.log().map_err(|_| IngestError::CorruptEvent)?;
    Ok(event)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::read::ServedCheckpoint;
    use serde_json::json;
    use std::sync::atomic::{AtomicU64, Ordering};

    static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

    fn directory() -> std::path::PathBuf {
        std::env::temp_dir().join(format!(
            "layerx-event-commit-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ))
    }

    fn receipted_log(checkpoint: &ServedCheckpoint, log_index: u32) -> CoreEvent {
        CoreEvent::from_log(
            &json!({
                "blockNumber": format!("{:#x}", checkpoint.height),
                "blockHash": format!("0x{}", hex::encode(checkpoint.block_hash)),
                "transactionIndex": "0x0",
                "logIndex": format!("{log_index:#x}"),
                "transactionHash": format!("0x{}", hex::encode([7; 32])),
                "address": format!("0x{}", hex::encode([8; 20])),
                "topics": [],
                "data": "0x0102",
                "removed": false
            }),
            EventAttributes {
                agent: "agent".into(),
                account: "account".into(),
                activity_type: 9,
                module: "asset".into(),
                asset: "LXR".into(),
                counterparty: "counterparty".into(),
                result_code: 0,
            },
        )
        .unwrap()
    }

    #[test]
    fn retired_rank_five_is_refused_in_durable_event_metadata() {
        let checkpoint = ServedCheckpoint::test_fixture(11, 1234, 0);
        let mut event = receipted_log(&checkpoint, 0);
        for receipt in [None, Some([9; 32])] {
            event.receipt_reference = receipt;
            let mut metadata = encode_metadata(&event);
            metadata[4] = 5;
            assert!(matches!(
                decode_metadata(&metadata),
                Err(IngestError::CorruptEvent)
            ));
            let mut pending = encode_pending(&event).unwrap();
            pending[4 + event.canonical_bytes.len() + 4] = 5;
            assert!(matches!(
                decode_pending(&pending),
                Err(IngestError::CorruptEvent)
            ));
        }
    }

    #[test]
    fn committed_promotion_validates_and_reopens_pending_and_retained_logs() {
        let root = directory();
        let tenant = TenantId::new("committed-promotion").unwrap();
        let checkpoint = ServedCheckpoint::test_fixture(11, 1234, 0);
        let mut retained = receipted_log(&checkpoint, 0);
        retained.receipt_reference = Some([9; 32]);
        let mut pending = receipted_log(&checkpoint, 1);
        pending.receipt_reference = Some([10; 32]);
        let position = pending.position().unwrap();
        let mut ingestor =
            EventIngestor::open(Store::open(&root).unwrap(), tenant.clone(), 4, 0).unwrap();
        ingestor.queue_log(pending.clone()).unwrap();
        ingest_with_class(
            &mut ingestor,
            retained.clone(),
            StorageClass::CoreProducedCache,
        )
        .unwrap();

        ingestor.commit_block(&checkpoint).unwrap();
        retained.receipt_verification_level = legacy::VerificationLevel::COMMITTED;
        pending.receipt_verification_level = legacy::VerificationLevel::COMMITTED;
        assert_eq!(ingestor.pending.get(&position), Some(&pending));
        validate_event(ingestor.pending.get(&position).unwrap()).unwrap();
        assert_eq!(ingestor.buffered.front(), Some(&retained));
        validate_event(ingestor.buffered.front().unwrap()).unwrap();
        assert_eq!(ingestor.retained_event(0).unwrap(), retained);
        let metadata = ingestor
            .store()
            .get(&metadata_key(tenant.clone(), 0).unwrap())
            .unwrap();
        assert_eq!(metadata.bytes()[4], 6);
        let encoded_pending = ingestor
            .store()
            .get(&pending_key(tenant.clone(), position).unwrap())
            .unwrap();
        assert_eq!(
            encoded_pending.bytes()[4 + pending.canonical_bytes.len() + 4],
            6
        );
        assert_eq!(decode_pending(encoded_pending.bytes()).unwrap(), pending);
        assert_eq!(ingestor.take_next(), Some(retained.clone()));
        drop(ingestor);

        let reopened = EventIngestor::open(Store::open(&root).unwrap(), tenant, 4, 0).unwrap();
        let restored = reopened.pending.get(&position).unwrap();
        assert_eq!(restored, &pending);
        assert_eq!(restored.rung(), Some(Rung::Committed));
        validate_event(restored).unwrap();
        assert_eq!(reopened.retained_event(0).unwrap(), retained);
        drop(reopened);
        std::fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn legacy_rank_four_durable_records_migrate_to_committed_on_read() {
        let root = directory();
        let tenant = TenantId::new("legacy-commit").unwrap();
        let checkpoint = ServedCheckpoint::test_fixture(11, 1234, 0);
        let mut retained = receipted_log(&checkpoint, 0);
        retained.receipt_reference = Some([9; 32]);
        retained.receipt_verification_level = legacy::VerificationLevel::COMMITTED;
        let mut pending = receipted_log(&checkpoint, 1);
        pending.receipt_verification_level = legacy::VerificationLevel::COMMITTED;
        let position = pending.position().unwrap();
        let mut ingestor =
            EventIngestor::open(Store::open(&root).unwrap(), tenant.clone(), 4, 0).unwrap();
        ingest_with_class(
            &mut ingestor,
            retained.clone(),
            StorageClass::CoreProducedCache,
        )
        .unwrap();
        let mut metadata = encode_metadata(&retained);
        metadata[4] = 4;
        let mut encoded_pending = encode_pending(&pending).unwrap();
        encoded_pending[4 + pending.canonical_bytes.len() + 4] = 4;
        ingestor
            .store
            .put_local(metadata_key(tenant.clone(), 0).unwrap(), metadata)
            .unwrap();
        ingestor
            .store
            .put_local(
                pending_key(tenant.clone(), position).unwrap(),
                encoded_pending,
            )
            .unwrap();
        drop(ingestor);

        let reopened = EventIngestor::open(Store::open(&root).unwrap(), tenant, 4, 0).unwrap();
        assert_eq!(reopened.retained_event(0).unwrap(), retained);
        let restored = reopened.pending.get(&position).unwrap();
        assert_eq!(restored, &pending);
        assert_eq!(restored.rung(), Some(Rung::Committed));
        validate_event(restored).unwrap();
        assert_eq!(encode_metadata(&reopened.retained_event(0).unwrap())[4], 6);
        assert_eq!(
            encode_pending(restored).unwrap()[4 + restored.canonical_bytes.len() + 4],
            6
        );
        drop(reopened);
        std::fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn committed_log_without_receipt_validates_through_log_evidence() {
        let checkpoint = ServedCheckpoint::test_fixture(11, 1234, 0);
        let mut event = receipted_log(&checkpoint, 0);
        event.receipt_verification_level = legacy::VerificationLevel::COMMITTED;
        assert!(event.receipt_reference.is_none());
        assert_eq!(event.log().unwrap().rung, Rung::Committed);
        validate_event(&event).unwrap();
        assert_eq!(
            decode_pending(&encode_pending(&event).unwrap()).unwrap(),
            event
        );

        event.canonical_bytes = vec![1];
        assert!(matches!(
            validate_event(&event),
            Err(IngestError::ReceiptEvidenceMismatch)
        ));
    }
}

type DecodedMetadata = (
    Option<[u8; 32]>,
    legacy::VerificationLevel,
    EventAttributes,
    Option<CursorAnchor>,
);
