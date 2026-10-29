//! Height-based read tool envelopes with explicit bounded pagination.

use layerx_agent_api::availability::AvailabilityReport;
use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::proof::ProofBundle;
use layerx_agent_api::read::{
    BalanceValue, CheckpointValue, Freshness, HistoryCursor, HistoryValue,
};
use layerx_agent_api::verify::Rung;

const MAX_PAGE_ITEMS: usize = 256;
const MAX_RESULT_BYTES: usize = 1_048_576;

mod sealed {
    pub trait Sealed {}
}

/// Values eligible for a protocol-fact read result. Projections cannot implement this trait.
pub trait CoreToolValue: sealed::Sealed {}

impl sealed::Sealed for BalanceValue {}
impl CoreToolValue for BalanceValue {}
impl sealed::Sealed for HistoryValue {}
impl CoreToolValue for HistoryValue {}
impl sealed::Sealed for CheckpointValue {}
impl CoreToolValue for CheckpointValue {}
impl sealed::Sealed for ProofBundle {}
impl CoreToolValue for ProofBundle {}
impl sealed::Sealed for AvailabilityReport {}
impl CoreToolValue for AvailabilityReport {}

/// Core-produced receipt bytes and the evidence identifiers used to verify them.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReceiptValue {
    pub canonical_receipt: CanonicalBytes,
    pub evidence_ids: Vec<[u8; 32]>,
    pub height: u64,
}

impl ReceiptValue {
    pub fn from_sequencer_signed(
        canonical_receipt: CanonicalBytes,
        keys: &[layerx_proof::inclusion::SequencerAuthorization],
    ) -> Result<Self, ReadToolError> {
        let signed =
            layerx_proof::receipt::verify_sequencer_signature(canonical_receipt.as_bytes(), keys)
                .map_err(|_| ReadToolError::Unverified)?;
        let protocol = signed
            .protocol()
            .ok_or(ReadToolError::MissingReceiptEvidence)?;
        let height = u64::from_be_bytes(
            protocol.batch_id()[24..]
                .try_into()
                .map_err(|_| ReadToolError::Arithmetic)?,
        );
        Ok(Self {
            canonical_receipt,
            evidence_ids: vec![protocol.context_hash()],
            height,
        })
    }
}

impl sealed::Sealed for ReceiptValue {}
impl CoreToolValue for ReceiptValue {}

/// Tamper-evident position within one immutable query snapshot.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct StableCursor {
    pub query_digest: [u8; 32],
    pub snapshot: [u8; 32],
    pub offset: u32,
}

/// A continuation either within the current response or at the core query boundary.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Continuation {
    Local(StableCursor),
    Core(HistoryCursor),
}

/// Caller-declared finite result bounds and optional stable resume position.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Pagination {
    pub maximum_items: usize,
    pub maximum_bytes: usize,
    pub cursor: Option<StableCursor>,
}

impl Pagination {
    /// Creates finite bounds accepted by read tools.
    ///
    /// # Errors
    ///
    /// Refuses zero or daemon-limit-exceeding bounds.
    pub const fn new(
        maximum_items: usize,
        maximum_bytes: usize,
        cursor: Option<StableCursor>,
    ) -> Result<Self, ReadToolError> {
        if maximum_items == 0
            || maximum_items > MAX_PAGE_ITEMS
            || maximum_bytes == 0
            || maximum_bytes > MAX_RESULT_BYTES
        {
            return Err(ReadToolError::InvalidBounds);
        }
        Ok(Self {
            maximum_items,
            maximum_bytes,
            cursor,
        })
    }
}

/// Completeness is always explicit; a continuation is mandatory when false.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PageInfo {
    pub complete: bool,
    pub explicitly_truncated: bool,
    pub returned_items: usize,
    pub returned_bytes: usize,
    pub next: Option<Continuation>,
}

/// A read carries height freshness and an optional write-evidence rung.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedToolResult<T: CoreToolValue> {
    pub value: T,
    pub rung: Option<Rung>,
    pub freshness: Freshness,
    pub page: PageInfo,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ReadToolError {
    Unverified,
    HeightMismatch,
    InvalidBounds,
    CursorMismatch,
    CursorOutOfRange,
    ResultTooLarge,
    MissingReceiptEvidence,
    Arithmetic,
}

/// Returns one balance with its exact height freshness coordinates.
///
/// # Errors
///
/// Returns `Unverified` for a submitted-only rung, `HeightMismatch` when the
/// value and freshness heights differ, and `ResultTooLarge`
/// when the encoded state exceeds `maximum_bytes`.
pub fn balance(
    value: BalanceValue,
    rung: Option<Rung>,
    freshness: Freshness,
    maximum_bytes: usize,
) -> Result<VerifiedToolResult<BalanceValue>, ReadToolError> {
    require_height(value.height, &freshness)?;
    let encoded_bytes = value.canonical_state.as_bytes().len();
    single(value, rung, freshness, encoded_bytes, maximum_bytes)
}

/// Returns verified receipt bytes; an evidence-free receipt is never surfaced as verified.
///
/// # Errors
///
/// Returns `MissingReceiptEvidence` for a receipt carrying no evidence identifiers,
/// `Arithmetic` when the evidence and receipt byte counts overflow, and the
/// `Unverified`, `HeightMismatch` and `ResultTooLarge` refusals.
pub fn receipt(
    value: ReceiptValue,
    rung: Option<Rung>,
    freshness: Freshness,
    maximum_bytes: usize,
) -> Result<VerifiedToolResult<ReceiptValue>, ReadToolError> {
    require_height(value.height, &freshness)?;
    if !matches!(rung, Some(Rung::Receipted | Rung::Committed)) {
        return Err(ReadToolError::Unverified);
    }
    if value.evidence_ids.is_empty() {
        return Err(ReadToolError::MissingReceiptEvidence);
    }
    let evidence_bytes = value
        .evidence_ids
        .len()
        .checked_mul(32)
        .ok_or(ReadToolError::Arithmetic)?;
    let encoded_bytes = value
        .canonical_receipt
        .as_bytes()
        .len()
        .checked_add(evidence_bytes)
        .ok_or(ReadToolError::Arithmetic)?;
    single(value, rung, freshness, encoded_bytes, maximum_bytes)
}

/// Returns an exact checkpoint certificate, never a locally inferred checkpoint summary.
///
/// # Errors
///
/// Returns `Unverified` for a submitted-only rung, `HeightMismatch` for inconsistent
/// height coordinates, and `ResultTooLarge` when the certificate exceeds
/// `maximum_bytes`.
pub fn checkpoint(
    value: CheckpointValue,
    rung: Option<Rung>,
    freshness: Freshness,
    maximum_bytes: usize,
) -> Result<VerifiedToolResult<CheckpointValue>, ReadToolError> {
    require_height(value.height, &freshness)?;
    let encoded_bytes = value
        .commit
        .as_bytes()
        .len()
        .checked_add(value.validator_set.as_bytes().len())
        .ok_or(ReadToolError::Arithmetic)?;
    single(value, rung, freshness, encoded_bytes, maximum_bytes)
}

/// Returns proof bytes and the exact target they verify.
///
/// # Errors
///
/// Returns `Arithmetic` when summing the target and proof byte lengths overflows,
/// plus the `Unverified` and `ResultTooLarge` refusals shared by every
/// read tool.
pub fn proof(
    value: ProofBundle,
    rung: Option<Rung>,
    freshness: Freshness,
    maximum_bytes: usize,
) -> Result<VerifiedToolResult<ProofBundle>, ReadToolError> {
    if rung != Some(value.verification_level) {
        return Err(ReadToolError::Unverified);
    }
    let encoded_bytes = [
        Some(&value.raw_transaction),
        Some(&value.signed_receipt),
        value.block_header.as_ref(),
        value.receipt_inclusion.as_ref(),
        value.commit.as_ref(),
        value.validator_set.as_ref(),
        value.account_proof.as_ref(),
    ]
    .into_iter()
    .flatten()
    .try_fold(value.receipt_digest.len(), |sum, bytes| {
        sum.checked_add(bytes.as_bytes().len())
            .ok_or(ReadToolError::Arithmetic)
    })?;
    single(value, rung, freshness, encoded_bytes, maximum_bytes)
}

/// Returns attributed availability evidence with explicit size accounting.
///
/// # Errors
///
/// Returns `Arithmetic` when a class byte count does not fit `usize` or the running
/// total overflows, plus the `Unverified` and `ResultTooLarge` refusals
/// shared by every read tool.
pub fn availability(
    value: AvailabilityReport,
    rung: Option<Rung>,
    freshness: Freshness,
    maximum_bytes: usize,
) -> Result<VerifiedToolResult<AvailabilityReport>, ReadToolError> {
    let encoded_bytes = value.classes.iter().try_fold(0_usize, |sum, class| {
        let bytes = usize::try_from(class.verified_bytes).map_err(|_| ReadToolError::Arithmetic)?;
        sum.checked_add(bytes).ok_or(ReadToolError::Arithmetic)
    })?;
    single(value, rung, freshness, encoded_bytes, maximum_bytes)
}

/// Pages exact core history records without silently dropping a record or upstream continuation.
///
/// # Errors
///
/// Returns `Unverified` for a submitted-only rung, `CursorMismatch` when the
/// cursor was issued for a different query digest or snapshot, `CursorOutOfRange` for an
/// offset past the record set, `ResultTooLarge` when the first record alone exceeds
/// `maximum_bytes`, and `Arithmetic` on byte-count or offset overflow.
pub fn history(
    value: &HistoryValue,
    rung: Option<Rung>,
    freshness: Freshness,
    query_digest: [u8; 32],
    snapshot: [u8; 32],
    pagination: Pagination,
) -> Result<VerifiedToolResult<HistoryValue>, ReadToolError> {
    require_read_rung(rung)?;
    require_height(value.height, &freshness)?;
    let offset = match pagination.cursor {
        Some(cursor) => {
            if cursor.query_digest != query_digest || cursor.snapshot != snapshot {
                return Err(ReadToolError::CursorMismatch);
            }
            usize::try_from(cursor.offset).map_err(|_| ReadToolError::CursorOutOfRange)?
        }
        None => 0,
    };
    if offset > value.records.len() {
        return Err(ReadToolError::CursorOutOfRange);
    }
    let mut records = Vec::new();
    let mut returned_bytes = 0_usize;
    for record in value.records.iter().skip(offset) {
        if records.len() == pagination.maximum_items {
            break;
        }
        let next_bytes = returned_bytes
            .checked_add(record.as_bytes().len())
            .ok_or(ReadToolError::Arithmetic)?;
        if next_bytes > pagination.maximum_bytes {
            if records.is_empty() {
                return Err(ReadToolError::ResultTooLarge);
            }
            break;
        }
        returned_bytes = next_bytes;
        records.push(record.clone());
    }
    let consumed = offset
        .checked_add(records.len())
        .ok_or(ReadToolError::Arithmetic)?;
    let next = if consumed < value.records.len() {
        Some(Continuation::Local(StableCursor {
            query_digest,
            snapshot,
            offset: u32::try_from(consumed).map_err(|_| ReadToolError::Arithmetic)?,
        }))
    } else {
        value.next_cursor.clone().map(Continuation::Core)
    };
    let complete = next.is_none();
    Ok(VerifiedToolResult {
        value: HistoryValue {
            records,
            next_cursor: None,
            height: value.height,
        },
        rung,
        freshness,
        page: PageInfo {
            complete,
            explicitly_truncated: !complete,
            returned_items: consumed - offset,
            returned_bytes,
            next,
        },
    })
}

fn single<T: CoreToolValue>(
    value: T,
    rung: Option<Rung>,
    freshness: Freshness,
    encoded_bytes: usize,
    maximum_bytes: usize,
) -> Result<VerifiedToolResult<T>, ReadToolError> {
    require_read_rung(rung)?;
    if maximum_bytes == 0 || maximum_bytes > MAX_RESULT_BYTES {
        return Err(ReadToolError::InvalidBounds);
    }
    if encoded_bytes > maximum_bytes {
        return Err(ReadToolError::ResultTooLarge);
    }
    Ok(VerifiedToolResult {
        value,
        rung,
        freshness,
        page: PageInfo {
            complete: true,
            explicitly_truncated: false,
            returned_items: 1,
            returned_bytes: encoded_bytes,
            next: None,
        },
    })
}

const fn require_read_rung(rung: Option<Rung>) -> Result<(), ReadToolError> {
    if matches!(rung, Some(Rung::Submitted)) {
        Err(ReadToolError::Unverified)
    } else {
        Ok(())
    }
}

fn require_height(height: u64, freshness: &Freshness) -> Result<(), ReadToolError> {
    if height != freshness.value_height {
        return Err(ReadToolError::HeightMismatch);
    }
    Ok(())
}
