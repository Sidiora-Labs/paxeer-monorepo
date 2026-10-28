//! Durable caller-to-protocol idempotency with conflict detection.

use std::collections::BTreeMap;
use std::path::Path;
use std::sync::Mutex;

use sha2::{Digest as _, Sha256};

use layerx_agent_api::error::{ApiError, RequestId};

use crate::store::{ObjectKind, Store as DurableStore, StoreError, TenantId, TenantKey};

const MAX_REQUEST_BYTES: usize = 1_048_576;
const MAX_RESULT_BYTES: usize = 1_048_576;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct RetentionPolicy {
    pub retention_blocks: u64,
    pub protocol_retention_blocks: u64,
}

impl RetentionPolicy {
    /// Creates a policy whose daemon window is never shorter than the protocol window.
    ///
    /// # Errors
    ///
    /// Returns `InvalidRetention` for a zero protocol window or a shorter daemon window.
    pub const fn new(
        retention_blocks: u64,
        protocol_retention_blocks: u64,
    ) -> Result<Self, IdempotencyError> {
        if protocol_retention_blocks == 0 || retention_blocks < protocol_retention_blocks {
            return Err(IdempotencyError::InvalidRetention);
        }
        Ok(Self {
            retention_blocks,
            protocol_retention_blocks,
        })
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Conflict {
    pub key: [u8; 32],
    pub original_request_digest: [u8; 32],
    pub repeated_request_digest: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct EconomicResult {
    pub response_bytes: Vec<u8>,
    pub receipt_ref: Option<[u8; 32]>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Outcome {
    First(EconomicResult),
    RepeatedOriginal(EconomicResult),
}

/// Exact protocol retry attempt. The idempotency key and bytes never change.
#[derive(Clone, Copy, Debug)]
pub struct ProtocolAttempt<'a> {
    pub idempotency_key: [u8; 32],
    pub exact_request_bytes: &'a [u8],
    pub request_digest: [u8; 32],
    pub retry: bool,
    pub head_height: u64,
    pub transaction_hash: Option<[u8; 32]>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
enum RecordState {
    Pending,
    Settled(EconomicResult),
    NodeSettled(EconomicResult),
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct Record {
    key: [u8; 32],
    request_digest: [u8; 32],
    exact_request_bytes: Vec<u8>,
    head_height: u64,
    transaction_hash: Option<[u8; 32]>,
    state: RecordState,
}

struct Inner {
    durable: DurableStore,
    records: BTreeMap<[u8; 32], Record>,
}

/// Thread-safe durable idempotency store.
pub struct Store {
    tenant: TenantId,
    retention: RetentionPolicy,
    inner: Mutex<Inner>,
}

impl Store {
    /// Opens the durable idempotency records owned by one tenant.
    ///
    /// # Errors
    ///
    /// Returns the store failure raised while creating, migrating, or decoding the root.
    pub fn open(
        root: impl AsRef<Path>,
        tenant: TenantId,
        retention: RetentionPolicy,
    ) -> Result<Self, IdempotencyError> {
        let durable = DurableStore::open(root).map_err(IdempotencyError::Store)?;
        Ok(Self {
            tenant,
            retention,
            inner: Mutex::new(Inner {
                durable,
                records: BTreeMap::new(),
            }),
        })
    }

    /// Restores known keys from durable storage after restart.
    ///
    /// # Errors
    ///
    /// Returns `Unavailable` for a poisoned lock, and `Corrupt` when a stored record does not
    /// decode or carries a key other than the one requested.
    pub fn restore(&self, keys: &[[u8; 32]]) -> Result<usize, IdempotencyError> {
        let mut inner = self
            .inner
            .lock()
            .map_err(|_| IdempotencyError::Unavailable)?;
        let mut restored = 0;
        for key in keys {
            let storage_key = storage_key(self.tenant.clone(), *key)?;
            let Some(value) = inner.durable.get(&storage_key) else {
                continue;
            };
            let record = decode_record(value.bytes())?;
            if record.key != *key {
                return Err(IdempotencyError::Corrupt);
            }
            inner.records.insert(*key, record);
            restored += 1;
        }
        Ok(restored)
    }

    /// Executes at most one concurrent operation for one key and stores its original result.
    ///
    /// # Errors
    ///
    /// Refuses a zero key, an empty or oversized request, a repeat whose digest differs from the
    /// original, an oversized result, and a poisoned lock. Also returns the operation's own failure
    /// and the store failure raised while persisting the pending or settled record.
    pub fn execute<F>(
        &self,
        key: [u8; 32],
        request_bytes: &[u8],
        current_height: u64,
        operation: F,
    ) -> Result<Outcome, IdempotencyError>
    where
        F: FnOnce(ProtocolAttempt<'_>) -> Result<EconomicResult, String>,
    {
        self.execute_with_request_id(RequestId(0), key, request_bytes, current_height, operation)
    }

    /// Executes a mutation while preserving its API request id in node replay refusals.
    ///
    /// # Errors
    /// Returns the same failures as `execute`, including a terminal node replay refusal.
    pub fn execute_with_request_id<F>(
        &self,
        request_id: RequestId,
        key: [u8; 32],
        request_bytes: &[u8],
        current_height: u64,
        operation: F,
    ) -> Result<Outcome, IdempotencyError>
    where
        F: FnOnce(ProtocolAttempt<'_>) -> Result<EconomicResult, String>,
    {
        if key == [0; 32] || request_bytes.is_empty() || request_bytes.len() > MAX_REQUEST_BYTES {
            return Err(IdempotencyError::InvalidRequest);
        }
        let repeated_digest = request_digest(request_bytes);
        let mut inner = self
            .inner
            .lock()
            .map_err(|_| IdempotencyError::Unavailable)?;
        let (stored_bytes, retry, head_height, transaction_hash) = if let Some(record) =
            inner.records.get(&key)
        {
            if record.request_digest != repeated_digest
                || record.exact_request_bytes != request_bytes
            {
                if matches!(record.state, RecordState::NodeSettled(_)) {
                    return Err(IdempotencyError::Api(ApiError::idempotent_replay(
                        request_id,
                    )));
                }
                return Err(IdempotencyError::Conflict(Conflict {
                    key,
                    original_request_digest: record.request_digest,
                    repeated_request_digest: repeated_digest,
                }));
            }
            if let RecordState::Settled(result) | RecordState::NodeSettled(result) = &record.state {
                return Ok(Outcome::RepeatedOriginal(result.clone()));
            }
            (
                record.exact_request_bytes.clone(),
                true,
                record.head_height,
                record.transaction_hash,
            )
        } else {
            let record = Record {
                key,
                request_digest: repeated_digest,
                exact_request_bytes: request_bytes.to_vec(),
                head_height: current_height,
                transaction_hash: None,
                state: RecordState::Pending,
            };
            persist(&mut inner.durable, self.tenant.clone(), &record)?;
            inner.records.insert(key, record);
            (request_bytes.to_vec(), false, current_height, None)
        };
        let result = operation(ProtocolAttempt {
            idempotency_key: key,
            exact_request_bytes: &stored_bytes,
            request_digest: repeated_digest,
            retry,
            head_height,
            transaction_hash,
        })
        .map_err(IdempotencyError::Operation)?;
        if result.response_bytes.len() > MAX_RESULT_BYTES {
            return Err(IdempotencyError::ResultTooLarge);
        }
        let mut settled = inner
            .records
            .get(&key)
            .cloned()
            .ok_or(IdempotencyError::Corrupt)?;
        settled.state = RecordState::Settled(result.clone());
        persist(&mut inner.durable, self.tenant.clone(), &settled)?;
        inner.records.insert(key, settled);
        Ok(if retry {
            Outcome::RepeatedOriginal(result)
        } else {
            Outcome::First(result)
        })
    }

    /// Records the observed head and signed transaction hash before sending the transaction.
    ///
    /// # Errors
    /// Refuses invalid input, conflicting requests or hashes, and durable storage failures.
    pub fn record_attempt(
        &self,
        key: [u8; 32],
        request_bytes: &[u8],
        head_height: u64,
        transaction_hash: [u8; 32],
    ) -> Result<(), IdempotencyError> {
        if key == [0; 32]
            || request_bytes.is_empty()
            || request_bytes.len() > MAX_REQUEST_BYTES
            || transaction_hash == [0; 32]
        {
            return Err(IdempotencyError::InvalidRequest);
        }
        let mut inner = self
            .inner
            .lock()
            .map_err(|_| IdempotencyError::Unavailable)?;
        let digest = request_digest(request_bytes);
        let record = if let Some(original) = inner.records.get(&key) {
            if original.request_digest != digest || original.exact_request_bytes != request_bytes {
                return Err(IdempotencyError::Conflict(Conflict {
                    key,
                    original_request_digest: original.request_digest,
                    repeated_request_digest: digest,
                }));
            }
            if original
                .transaction_hash
                .is_some_and(|hash| hash != transaction_hash)
            {
                return Err(IdempotencyError::Corrupt);
            }
            let mut record = original.clone();
            record.transaction_hash = Some(transaction_hash);
            record
        } else {
            Record {
                key,
                request_digest: digest,
                exact_request_bytes: request_bytes.to_vec(),
                head_height,
                transaction_hash: Some(transaction_hash),
                state: RecordState::Pending,
            }
        };
        persist(&mut inner.durable, self.tenant.clone(), &record)?;
        inner.records.insert(key, record);
        Ok(())
    }

    /// Persists an original result obtained from the node's `resultByIdempotency` view.
    /// The caller supplies the original request and transaction hash associated with that result.
    ///
    /// # Errors
    /// Refuses invalid requests, oversized results, conflicting local requests or results,
    /// poisoned locks, and durable storage failures.
    pub fn record_node_result(
        &self,
        key: [u8; 32],
        request_bytes: &[u8],
        head_height: u64,
        transaction_hash: [u8; 32],
        result: EconomicResult,
    ) -> Result<(), IdempotencyError> {
        if key == [0; 32]
            || request_bytes.is_empty()
            || request_bytes.len() > MAX_REQUEST_BYTES
            || transaction_hash == [0; 32]
        {
            return Err(IdempotencyError::InvalidRequest);
        }
        if result.response_bytes.len() > MAX_RESULT_BYTES {
            return Err(IdempotencyError::ResultTooLarge);
        }
        let mut inner = self
            .inner
            .lock()
            .map_err(|_| IdempotencyError::Unavailable)?;
        let digest = request_digest(request_bytes);
        if let Some(record) = inner.records.get(&key) {
            if record.request_digest != digest || record.exact_request_bytes != request_bytes {
                return Err(IdempotencyError::Conflict(Conflict {
                    key,
                    original_request_digest: record.request_digest,
                    repeated_request_digest: digest,
                }));
            }
            if record
                .transaction_hash
                .is_some_and(|hash| hash != transaction_hash)
            {
                return Err(IdempotencyError::Corrupt);
            }
            if let RecordState::Settled(original) | RecordState::NodeSettled(original) =
                &record.state
            {
                if original != &result {
                    return Err(IdempotencyError::Corrupt);
                }
            }
        }
        let record = Record {
            key,
            request_digest: digest,
            exact_request_bytes: request_bytes.to_vec(),
            head_height: inner
                .records
                .get(&key)
                .map_or(head_height, |record| record.head_height),
            transaction_hash: Some(transaction_hash),
            state: RecordState::NodeSettled(result),
        };
        persist(&mut inner.durable, self.tenant.clone(), &record)?;
        inner.records.insert(key, record);
        Ok(())
    }

    /// Removes records only after daemon retention, which is never shorter than protocol retention.
    ///
    /// # Errors
    ///
    /// Returns `Unavailable` for a poisoned lock, and the store failure raised while removing an
    /// expired record.
    pub fn sweep(&self, current_height: u64) -> Result<usize, IdempotencyError> {
        let mut inner = self
            .inner
            .lock()
            .map_err(|_| IdempotencyError::Unavailable)?;
        let expired: Vec<_> = inner
            .records
            .iter()
            .filter(|(_, record)| {
                matches!(
                    record.state,
                    RecordState::Settled(_) | RecordState::NodeSettled(_)
                ) && current_height.saturating_sub(record.head_height)
                    > self.retention.retention_blocks
            })
            .map(|(key, _)| *key)
            .collect();
        for key in &expired {
            let storage_key = storage_key(self.tenant.clone(), *key)?;
            inner
                .durable
                .remove_local(&storage_key)
                .map_err(IdempotencyError::Store)?;
            inner.records.remove(key);
        }
        Ok(expired.len())
    }
}

#[derive(Debug)]
pub enum IdempotencyError {
    InvalidRetention,
    InvalidRequest,
    ResultTooLarge,
    Conflict(Conflict),
    Api(ApiError),
    Operation(String),
    Unavailable,
    Corrupt,
    Store(StoreError),
}

fn request_digest(bytes: &[u8]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(b"LXP/agent/request/v1\0");
    hasher.update(bytes);
    hasher.finalize().into()
}

fn storage_key(tenant: TenantId, key: [u8; 32]) -> Result<TenantKey, IdempotencyError> {
    TenantKey::new(tenant, ObjectKind::Idempotency, key.to_vec()).map_err(IdempotencyError::Store)
}

fn persist(
    durable: &mut DurableStore,
    tenant: TenantId,
    record: &Record,
) -> Result<(), IdempotencyError> {
    let key = storage_key(tenant, record.key)?;
    durable
        .put_local(key, encode_record(record)?)
        .map_err(IdempotencyError::Store)
}

fn encode_record(record: &Record) -> Result<Vec<u8>, IdempotencyError> {
    let request_len = u32::try_from(record.exact_request_bytes.len())
        .map_err(|_| IdempotencyError::InvalidRequest)?;
    let mut bytes = Vec::new();
    bytes.extend_from_slice(b"LXID");
    bytes.push(2);
    bytes.extend_from_slice(&record.key);
    bytes.extend_from_slice(&record.request_digest);
    bytes.extend_from_slice(&record.head_height.to_be_bytes());
    match record.transaction_hash {
        Some(hash) => {
            bytes.push(1);
            bytes.extend_from_slice(&hash);
        }
        None => bytes.push(0),
    }
    bytes.extend_from_slice(&request_len.to_be_bytes());
    bytes.extend_from_slice(&record.exact_request_bytes);
    match &record.state {
        RecordState::Pending => bytes.push(0),
        RecordState::Settled(result) | RecordState::NodeSettled(result) => {
            bytes.push(if matches!(record.state, RecordState::NodeSettled(_)) {
                2
            } else {
                1
            });
            let response_len = u32::try_from(result.response_bytes.len())
                .map_err(|_| IdempotencyError::ResultTooLarge)?;
            bytes.extend_from_slice(&response_len.to_be_bytes());
            bytes.extend_from_slice(&result.response_bytes);
            match result.receipt_ref {
                Some(receipt) => {
                    bytes.push(1);
                    bytes.extend_from_slice(&receipt);
                }
                None => bytes.push(0),
            }
        }
    }
    Ok(bytes)
}

fn decode_record(bytes: &[u8]) -> Result<Record, IdempotencyError> {
    let mut decoder = Decoder { bytes, offset: 0 };
    if decoder.take(4)? != b"LXID" || decoder.u8()? != 2 {
        return Err(IdempotencyError::Corrupt);
    }
    let key = decoder.fixed()?;
    let stored_digest = decoder.fixed()?;
    let head_height = decoder.u64()?;
    let transaction_hash = match decoder.u8()? {
        0 => None,
        1 => Some(decoder.fixed()?),
        _ => return Err(IdempotencyError::Corrupt),
    };
    let request_len = decoder.u32()? as usize;
    if request_len > MAX_REQUEST_BYTES {
        return Err(IdempotencyError::Corrupt);
    }
    let exact_request_bytes = decoder.take(request_len)?.to_vec();
    if request_digest(exact_request_bytes.as_slice()) != stored_digest {
        return Err(IdempotencyError::Corrupt);
    }
    let state = match decoder.u8()? {
        0 => RecordState::Pending,
        tag @ (1 | 2) => {
            let result_len = decoder.u32()? as usize;
            if result_len > MAX_RESULT_BYTES {
                return Err(IdempotencyError::Corrupt);
            }
            let response_bytes = decoder.take(result_len)?.to_vec();
            let receipt_ref = match decoder.u8()? {
                0 => None,
                1 => Some(decoder.fixed()?),
                _ => return Err(IdempotencyError::Corrupt),
            };
            let result = EconomicResult {
                response_bytes,
                receipt_ref,
            };
            if tag == 2 {
                if transaction_hash.is_none() || transaction_hash == Some([0; 32]) {
                    return Err(IdempotencyError::Corrupt);
                }
                RecordState::NodeSettled(result)
            } else {
                RecordState::Settled(result)
            }
        }
        _ => return Err(IdempotencyError::Corrupt),
    };
    if decoder.offset != bytes.len() {
        return Err(IdempotencyError::Corrupt);
    }
    Ok(Record {
        key,
        request_digest: stored_digest,
        exact_request_bytes,
        head_height,
        transaction_hash,
        state,
    })
}

struct Decoder<'a> {
    bytes: &'a [u8],
    offset: usize,
}

impl<'a> Decoder<'a> {
    fn take(&mut self, length: usize) -> Result<&'a [u8], IdempotencyError> {
        let end = self
            .offset
            .checked_add(length)
            .ok_or(IdempotencyError::Corrupt)?;
        let value = self
            .bytes
            .get(self.offset..end)
            .ok_or(IdempotencyError::Corrupt)?;
        self.offset = end;
        Ok(value)
    }

    fn u8(&mut self) -> Result<u8, IdempotencyError> {
        Ok(*self.take(1)?.first().ok_or(IdempotencyError::Corrupt)?)
    }

    fn u32(&mut self) -> Result<u32, IdempotencyError> {
        let mut value = [0; 4];
        value.copy_from_slice(self.take(4)?);
        Ok(u32::from_be_bytes(value))
    }

    fn u64(&mut self) -> Result<u64, IdempotencyError> {
        let mut value = [0; 8];
        value.copy_from_slice(self.take(8)?);
        Ok(u64::from_be_bytes(value))
    }

    fn fixed(&mut self) -> Result<[u8; 32], IdempotencyError> {
        let mut value = [0; 32];
        value.copy_from_slice(self.take(32)?);
        Ok(value)
    }
}
