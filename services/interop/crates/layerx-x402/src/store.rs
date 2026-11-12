//! Durable buyer settlement captures, reauthenticated against caller-configured
//! receipt keys and consensus anchors whenever the store is reopened.

use std::collections::BTreeMap;
use std::fmt::{Display, Formatter};
use std::fs::{self, File, OpenOptions};
use std::io::{self, Write};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_interop_gateway::trace::{TraceId, Traced};
use serde::{Deserialize, Serialize};

use crate::buyer::{BuiltPayment, Buyer, CapturedSettlement};
use crate::codec::{decode_header, encode_header};
use crate::model::{parse_hex32, PaymentPayload, SettlementResponse, X402Error};

const MAX_RECORD_BYTES: u64 = 256 * 1024;
static NEXT_STAGING_FILE: AtomicU64 = AtomicU64::new(0);

#[derive(Debug)]
pub enum SettlementStoreError {
    Io(io::Error),
    Payment(Traced<X402Error>),
    Conflict,
}

impl Display for SettlementStoreError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Io(error) => write!(formatter, "settlement store: {error}"),
            Self::Payment(error) => Display::fmt(error, formatter),
            Self::Conflict => {
                formatter.write_str("native payment identity already has a different settlement")
            }
        }
    }
}

impl std::error::Error for SettlementStoreError {}

impl From<io::Error> for SettlementStoreError {
    fn from(error: io::Error) -> Self {
        Self::Io(error)
    }
}

#[derive(Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
struct Record {
    payment: PaymentPayload,
    idempotency_key: [u8; 32],
    response: SettlementResponse,
}

fn record_name(key: &[u8; 32]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut hex = String::with_capacity(key.len() * 2);
    for byte in key {
        hex.push(char::from(DIGITS[usize::from(byte >> 4)]));
        hex.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    format!("{hex}.json")
}

fn read_record(path: &Path) -> Result<Record, SettlementStoreError> {
    let file = File::open(path)?;
    if file.metadata()?.len() > MAX_RECORD_BYTES {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            "settlement record exceeds its bound",
        )
        .into());
    }
    serde_json::from_reader(file)
        .map_err(|error| io::Error::new(io::ErrorKind::InvalidData, error).into())
}

impl Record {
    fn capture(
        &self,
        buyer: &Buyer,
        trace: &TraceId,
    ) -> Result<CapturedSettlement, SettlementStoreError> {
        let fail = |error| SettlementStoreError::Payment(trace.wrap(error));
        let native_key = self
            .payment
            .payload
            .get("idempotencyKey")
            .and_then(|value| value.as_str())
            .and_then(|value| parse_hex32(value).ok());
        if self.idempotency_key == [0; 32] || native_key != Some(self.idempotency_key) {
            return Err(fail(X402Error::PaymentIdentityMismatch));
        }
        self.payment.validate().map_err(fail)?;
        let payment = BuiltPayment {
            header: encode_header(&self.payment).map_err(fail)?,
            payload: self.payment.clone(),
            idempotency_key: self.idempotency_key,
        };
        buyer
            .capture(
                &encode_header(&self.response).map_err(fail)?,
                &payment,
                trace,
            )
            .map_err(SettlementStoreError::Payment)
    }
}

/// Immutable, native-key-indexed captures. Publishing a record links a fully
/// synced staging file atomically; an existing identity is never overwritten.
/// This stores verified settlement evidence, not native ledger execution state.
pub struct SettlementStore {
    directory: PathBuf,
    captures: BTreeMap<[u8; 32], (Record, CapturedSettlement)>,
}

impl SettlementStore {
    /// Opens the store and authenticates every committed record with this buyer.
    /// Incomplete staging files are uncommitted and cannot become recovered captures.
    ///
    /// # Errors
    /// Refuses corrupt records, wrong identities, unauthenticated evidence and I/O failures.
    pub fn open(
        directory: impl AsRef<Path>,
        buyer: &Buyer,
        trace: &TraceId,
    ) -> Result<Self, SettlementStoreError> {
        let directory = directory.as_ref().to_path_buf();
        fs::create_dir_all(&directory)?;
        let mut captures = BTreeMap::new();
        for entry in fs::read_dir(&directory)? {
            let entry = entry?;
            if entry.file_name().to_string_lossy().starts_with(".pending-") {
                continue;
            }
            let record = read_record(&entry.path())?;
            if entry.file_name() != record_name(&record.idempotency_key).as_str() {
                return Err(SettlementStoreError::Conflict);
            }
            let settlement = record.capture(buyer, trace)?;
            captures.insert(record.idempotency_key, (record, settlement));
        }
        Ok(Self {
            directory,
            captures,
        })
    }

    #[must_use]
    pub fn get(&self, key: &[u8; 32]) -> Option<&CapturedSettlement> {
        self.captures.get(key).map(|(_, captured)| captured)
    }

    /// Captures the real buyer response before durably publishing it. Identical
    /// redelivery retains the record; conflicting evidence for a key is refused.
    ///
    /// # Errors
    /// Refuses pending or invalid evidence, identity conflicts and persistence failures.
    pub fn capture(
        &mut self,
        buyer: &Buyer,
        header: &str,
        expected: &BuiltPayment,
        trace: &TraceId,
    ) -> Result<CapturedSettlement, SettlementStoreError> {
        let fail = |error| SettlementStoreError::Payment(trace.wrap(error));
        let response: SettlementResponse = decode_header(header).map_err(fail)?;
        response.validate_wire().map_err(fail)?;
        if !response.success && response.error_reason.as_deref() == Some("settlement_pending") {
            return Err(fail(X402Error::PaymentPending));
        }
        let captured = buyer
            .capture(header, expected, trace)
            .map_err(SettlementStoreError::Payment)?;
        let record = Record {
            payment: expected.payload.clone(),
            idempotency_key: expected.idempotency_key,
            response: captured.response.clone(),
        };
        // Check the wire identity too: a caller can construct BuiltPayment directly.
        record.capture(buyer, trace)?;
        if let Some((previous, _)) = self.captures.get(&record.idempotency_key) {
            if previous != &record {
                return Err(SettlementStoreError::Conflict);
            }
            return Ok(captured);
        }
        let bytes = serde_json::to_vec(&record)
            .map_err(|error| io::Error::new(io::ErrorKind::InvalidData, error))?;
        if bytes.len() as u64 > MAX_RECORD_BYTES {
            return Err(io::Error::new(
                io::ErrorKind::InvalidData,
                "settlement record exceeds its bound",
            )
            .into());
        }
        let staging = self.directory.join(format!(
            ".pending-{}-{}",
            std::process::id(),
            NEXT_STAGING_FILE.fetch_add(1, Ordering::Relaxed)
        ));
        let destination = self.directory.join(record_name(&record.idempotency_key));
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(&staging)?;
        let publish = (|| -> Result<(), SettlementStoreError> {
            file.write_all(&bytes)?;
            file.sync_all()?;
            match fs::hard_link(&staging, &destination) {
                Ok(()) => {}
                Err(error) if error.kind() == io::ErrorKind::AlreadyExists => {
                    if read_record(&destination)? != record {
                        return Err(SettlementStoreError::Conflict);
                    }
                }
                Err(error) => return Err(error.into()),
            }
            File::open(&self.directory)?.sync_all()?;
            Ok(())
        })();
        drop(file);
        let cleanup = fs::remove_file(staging);
        publish?;
        cleanup?;
        self.captures
            .insert(record.idempotency_key, (record, captured.clone()));
        Ok(captured)
    }
}
