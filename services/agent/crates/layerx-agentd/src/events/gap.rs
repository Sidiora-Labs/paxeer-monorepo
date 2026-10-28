//! Explicit block-height gaps, history backfill reports and retention loss.

use std::fmt::{Display, Formatter};

use crate::read::{history, Cursor, HistoryLimits, HistoryReadError};
use layerx_agent_api::subscription::SubscriptionTarget;
use layerx_client::lni::transport::FrameTransport;
use layerx_client::read::{HistoryKind, ReadContext};

use super::ingestion::{decode_log, durable_event, durable_sequences};
use super::subscription::{Continuity, Store as SubscriptionStore, SubscriptionError};
use crate::session::{SessionRegistry, Token};
use crate::tenant::{Operation, TenantObservability};

/// Explicit missing interval emitted instead of silently closing sequence.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Gap {
    pub missing_first: u64,
    pub missing_last: u64,
    pub observed_sequence: u64,
}

impl Gap {
    pub fn between_heads(previous: u64, observed: u64) -> Option<Self> {
        let expected = previous.checked_add(1)?;
        (observed > expected).then_some(Self {
            missing_first: expected,
            missing_last: observed.saturating_sub(1),
            observed_sequence: observed,
        })
    }
}

/// Exact core bytes recovered for one missing sequence.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RecoveredEvent {
    pub global_sequence: u64,
    pub canonical_bytes: Vec<u8>,
    pub position: Cursor,
}

/// Why a real core backfill did not restore the entire interval.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum BackfillFailure {
    RangeExceedsBound {
        required: u64,
        maximum: u32,
    },
    History(HistoryReadError),
    CoreReportedGap {
        missing_first: u64,
        missing_last: u64,
    },
    Incomplete,
}

/// Consumer-visible outcome of the core backfill attempt.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum BackfillReport {
    Recovered {
        gap: Gap,
        events: Vec<RecoveredEvent>,
    },
    Incomplete {
        gap: Gap,
        recovered: Vec<RecoveredEvent>,
        failure: BackfillFailure,
    },
}

impl BackfillReport {
    /// Returns the last exact sequence recovered before the attempt stopped.
    #[must_use]
    pub fn recovered_through(&self) -> Option<u64> {
        match self {
            Self::Recovered { gap, .. } => Some(gap.missing_last),
            Self::Incomplete {
                recovered: events, ..
            } => events.last().map(|event| event.position.block_height),
        }
    }
}

/// Durable terminal notice when requested events have left retention.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Truncated {
    pub requested_from: u64,
    pub oldest_available: u64,
    pub lost_through: u64,
}

/// Explicit sequence-based retention policy.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Retention {
    pub maximum_undelivered_sequences: u64,
}

/// Result of applying a core backfill report to durable continuity state.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum BackfillResolution {
    Restored,
    StillBlocked { recovered_through: Option<u64> },
}

/// Gap operations never infer or manufacture a protocol result.
#[derive(Debug)]
pub enum GapError {
    Regressed { expected: u64, observed: u64 },
    InvalidRetention,
    Blocked(Gap),
    Truncated(Truncated),
    MismatchedReport,
    Subscription(SubscriptionError),
    DurableEvent,
}

impl Display for GapError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Regressed { expected, observed } => write!(
                formatter,
                "observed sequence {observed} precedes expected sequence {expected}"
            ),
            Self::InvalidRetention => formatter.write_str("retention bound must be non-zero"),
            Self::Blocked(gap) => write!(
                formatter,
                "delivery is blocked by missing sequences {} through {}",
                gap.missing_first, gap.missing_last
            ),
            Self::Truncated(notice) => write!(
                formatter,
                "subscription is truncated from {} through {}",
                notice.requested_from, notice.lost_through
            ),
            Self::MismatchedReport => {
                formatter.write_str("backfill report does not match the durable gap")
            }
            Self::Subscription(error) => Display::fmt(error, formatter),
            Self::DurableEvent => {
                formatter.write_str("recovered core event is not durably available")
            }
        }
    }
}

impl std::error::Error for GapError {}

impl From<SubscriptionError> for GapError {
    fn from(value: SubscriptionError) -> Self {
        Self::Subscription(value)
    }
}

/// Detects a forward jump, persists the block and returns the exact missing
/// interval for consumer disclosure.
///
/// # Errors
///
/// Returns `Regressed` when the observed sequence precedes the expected one, and
/// propagates the subscription-store failure that prevented persisting the block.
pub fn detect(
    subscriptions: &mut SubscriptionStore,
    target: &SubscriptionTarget,
    expected: u64,
    observed: u64,
) -> Result<Option<Gap>, GapError> {
    subscriptions.require_unbound_target(target)?;
    detect_inner(subscriptions, target, expected, observed)
}

/// Detects and durably blocks a gap for an exact session-bound subscription after resolving the
/// current token generation through the common tenant gate.
///
/// # Errors
///
/// Returns an error if authorization fails, continuity is invalid, or subscription state cannot be persisted.
pub fn detect_authorized(
    subscriptions: &mut SubscriptionStore,
    authorization: GapAuthorization<'_>,
    target: &SubscriptionTarget,
    expected: u64,
    observed: u64,
) -> Result<Option<Gap>, GapError> {
    let GapAuthorization {
        sessions,
        token,
        observability,
        core_sequence,
    } = authorization;
    subscriptions.authorize_target(
        sessions,
        token,
        observability,
        core_sequence,
        target,
        Operation::SubscriptionResume,
    )?;
    detect_inner(subscriptions, target, expected, observed)
}

fn detect_inner(
    subscriptions: &mut SubscriptionStore,
    target: &SubscriptionTarget,
    expected: u64,
    observed: u64,
) -> Result<Option<Gap>, GapError> {
    if observed < expected {
        return Err(GapError::Regressed { expected, observed });
    }
    if observed == expected {
        return Ok(None);
    }
    let gap = Gap {
        missing_first: expected,
        missing_last: observed.saturating_sub(1),
        observed_sequence: observed,
    };
    subscriptions.block_gap_inner(target, gap.missing_first, gap.missing_last)?;
    Ok(Some(gap))
}

pub fn attempt_backfill(
    transport: &mut dyn FrameTransport,
    gap: Gap,
    context: ReadContext,
    limits: HistoryLimits,
    maximum_records: u32,
) -> BackfillReport {
    let mut recovered = Vec::new();
    let mut cursor = None;
    let mut read = 0_u32;
    loop {
        let remaining = maximum_records.saturating_sub(read);
        if remaining == 0 || gap.missing_first > gap.missing_last {
            return BackfillReport::Incomplete {
                gap,
                recovered,
                failure: BackfillFailure::RangeExceedsBound {
                    required: u64::from(read) + 1,
                    maximum: maximum_records,
                },
            };
        }
        let page_limits = HistoryLimits {
            maximum_items: limits
                .maximum_items
                .min(u16::try_from(remaining).unwrap_or(u16::MAX)),
            ..limits
        };
        let page = match history(
            transport,
            gap.missing_first,
            gap.missing_last,
            cursor,
            page_limits,
            context,
        ) {
            Ok(page) => page,
            Err(error) => {
                return BackfillReport::Incomplete {
                    gap,
                    recovered,
                    failure: BackfillFailure::History(error),
                }
            }
        };
        if page.height < gap.missing_last {
            return BackfillReport::Incomplete {
                gap,
                recovered,
                failure: BackfillFailure::Incomplete,
            };
        }
        read += u32::try_from(page.items.len()).unwrap_or(u32::MAX);
        for item in page.items {
            if item.kind != HistoryKind::Event {
                continue;
            }
            let event = decode_log(&item.canonical, layerx_types::rung::Rung::Receipted);
            if !event.is_ok_and(|event| {
                Cursor {
                    block_height: event.block_height,
                    tx_index: event.tx_index,
                    log_index: event.log_index,
                } == item.position
            }) {
                return BackfillReport::Incomplete {
                    gap,
                    recovered,
                    failure: BackfillFailure::History(HistoryReadError::Malformed),
                };
            }
            recovered.push(RecoveredEvent {
                global_sequence: item.position.block_height,
                canonical_bytes: item.canonical,
                position: item.position,
            });
        }
        match page.next_cursor {
            Some(next) if cursor.is_none_or(|previous| next > previous) => cursor = Some(next),
            None => {
                return BackfillReport::Recovered {
                    gap,
                    events: recovered,
                }
            }
            _ => {
                return BackfillReport::Incomplete {
                    gap,
                    recovered,
                    failure: BackfillFailure::Incomplete,
                }
            }
        }
    }
}

/// Applies a backfill report only after every recovered event is present in
/// durable history with the exact core bytes returned by the boundary.
///
/// # Errors
///
/// Returns `MismatchedReport` when continuity is not blocked on exactly this
/// interval or the recovered sequences are not contiguous through its end, and
/// `DurableEvent` when a recovered event is absent from durable history or its
/// bytes differ from the core bytes.
pub fn apply_backfill(
    subscriptions: &mut SubscriptionStore,
    target: &SubscriptionTarget,
    gap: Gap,
    report: &BackfillReport,
) -> Result<BackfillResolution, GapError> {
    subscriptions.require_unbound_target(target)?;
    apply_backfill_inner(subscriptions, target, gap, report)
}

/// Applies a core backfill report for an exact session-bound subscription only after resolving
/// its current token generation through the common tenant gate.
///
/// # Errors
///
/// Returns an error if authorization fails, continuity is invalid, or subscription state cannot be persisted.
pub fn apply_backfill_authorized(
    subscriptions: &mut SubscriptionStore,
    authorization: GapAuthorization<'_>,
    target: &SubscriptionTarget,
    gap: Gap,
    report: &BackfillReport,
) -> Result<BackfillResolution, GapError> {
    let GapAuthorization {
        sessions,
        token,
        observability,
        core_sequence,
    } = authorization;
    subscriptions.authorize_target(
        sessions,
        token,
        observability,
        core_sequence,
        target,
        Operation::SubscriptionResume,
    )?;
    apply_backfill_inner(subscriptions, target, gap, report)
}

fn apply_backfill_inner(
    subscriptions: &mut SubscriptionStore,
    target: &SubscriptionTarget,
    gap: Gap,
    report: &BackfillReport,
) -> Result<BackfillResolution, GapError> {
    let Continuity::GapBlocked {
        missing_first,
        missing_last,
        ..
    } = subscriptions.continuity_inner(target)?
    else {
        return Err(GapError::MismatchedReport);
    };
    if missing_first != gap.missing_first || missing_last != gap.missing_last {
        return Err(GapError::MismatchedReport);
    }
    match report {
        BackfillReport::Recovered {
            gap: report_gap,
            events,
        } if *report_gap == gap => {
            let tenant = crate::store::TenantId::new(target.scope.tenant.as_str())
                .map_err(|_| GapError::DurableEvent)?;
            let durable_store = subscriptions.durable()?;
            let sequences =
                durable_sequences(&durable_store, &tenant).map_err(|_| GapError::DurableEvent)?;
            let mut previous = None;
            for recovered in events {
                if !(gap.missing_first..=gap.missing_last)
                    .contains(&recovered.position.block_height)
                    || previous.is_some_and(|position| recovered.position <= position)
                {
                    return Err(GapError::MismatchedReport);
                }
                previous = Some(recovered.position);
                let mut found = false;
                for &sequence in &sequences {
                    let durable = durable_event(&durable_store, &tenant, sequence)
                        .map_err(|_| GapError::DurableEvent)?;
                    if durable.position() == Some(recovered.position)
                        && durable.canonical_bytes == recovered.canonical_bytes
                    {
                        found = true;
                        break;
                    }
                }
                if !found {
                    return Err(GapError::DurableEvent);
                }
            }
            drop(durable_store);
            subscriptions.clear_gap_inner(target)?;
            Ok(BackfillResolution::Restored)
        }
        BackfillReport::Incomplete {
            gap: report_gap, ..
        } if *report_gap == gap => {
            let recovered_through = report.recovered_through();
            subscriptions.record_backfill_inner(target, recovered_through)?;
            Ok(BackfillResolution::StillBlocked { recovered_through })
        }
        _ => Err(GapError::MismatchedReport),
    }
}

/// Refuses any delivery while durable continuity is blocked or truncated.
///
/// # Errors
///
/// Returns `Blocked` with the missing interval or `Truncated` with the retention
/// notice, and propagates the subscription-store read failure.
pub fn admit(
    subscriptions: &SubscriptionStore,
    target: &SubscriptionTarget,
) -> Result<(), GapError> {
    subscriptions.require_unbound_target(target)?;
    admit_inner(subscriptions, target)
}

/// Checks continuity for an exact session-bound subscription after resolving its current token
/// generation through the common tenant gate.
///
/// # Errors
///
/// Returns an error if authorization fails, continuity is invalid, or subscription state cannot be persisted.
pub fn admit_authorized(
    subscriptions: &SubscriptionStore,
    sessions: &SessionRegistry,
    token: &Token,
    observability: &mut TenantObservability,
    core_sequence: u64,
    target: &SubscriptionTarget,
) -> Result<(), GapError> {
    subscriptions.authorize_target(
        sessions,
        token,
        observability,
        core_sequence,
        target,
        Operation::SubscriptionHealth,
    )?;
    admit_inner(subscriptions, target)
}

fn admit_inner(
    subscriptions: &SubscriptionStore,
    target: &SubscriptionTarget,
) -> Result<(), GapError> {
    match subscriptions.continuity_inner(target)? {
        Continuity::Healthy => Ok(()),
        Continuity::GapBlocked {
            missing_first,
            missing_last,
            ..
        } => Err(GapError::Blocked(Gap {
            missing_first,
            missing_last,
            observed_sequence: missing_last.saturating_add(1),
        })),
        Continuity::Truncated {
            requested_from,
            oldest_available,
            lost_through,
        } => Err(GapError::Truncated(Truncated {
            requested_from,
            oldest_available,
            lost_through,
        })),
    }
}

/// Enforces the declared retention bound and persists terminal truncation when
/// an unacknowledged cursor is older than the effective retained window.
///
/// # Errors
///
/// Returns `InvalidRetention` for a zero bound and propagates the
/// subscription-store failure that prevented reading the cursor or persisting the
/// notice.
pub fn enforce_retention(
    subscriptions: &mut SubscriptionStore,
    target: &SubscriptionTarget,
    head_exclusive: u64,
    core_oldest_available: u64,
    retention: Retention,
) -> Result<Option<Truncated>, GapError> {
    subscriptions.require_unbound_target(target)?;
    enforce_retention_inner(
        subscriptions,
        target,
        head_exclusive,
        core_oldest_available,
        retention,
    )
}

/// Enforces retention for an exact session-bound subscription after resolving its current token
/// generation through the common tenant gate.
///
/// # Errors
///
/// Returns an error if authorization fails, continuity is invalid, or subscription state cannot be persisted.
pub fn enforce_retention_authorized(
    subscriptions: &mut SubscriptionStore,
    authorization: GapAuthorization<'_>,
    target: &SubscriptionTarget,
    head_exclusive: u64,
    core_oldest_available: u64,
    retention: Retention,
) -> Result<Option<Truncated>, GapError> {
    let GapAuthorization {
        sessions,
        token,
        observability,
        core_sequence,
    } = authorization;
    subscriptions.authorize_target(
        sessions,
        token,
        observability,
        core_sequence,
        target,
        Operation::SubscriptionResume,
    )?;
    enforce_retention_inner(
        subscriptions,
        target,
        head_exclusive,
        core_oldest_available,
        retention,
    )
}

fn enforce_retention_inner(
    subscriptions: &mut SubscriptionStore,
    target: &SubscriptionTarget,
    head_exclusive: u64,
    core_oldest_available: u64,
    retention: Retention,
) -> Result<Option<Truncated>, GapError> {
    if retention.maximum_undelivered_sequences == 0 {
        return Err(GapError::InvalidRetention);
    }
    let requested_from = subscriptions.resume_cursor_inner(target)?.block_height;
    let bounded_oldest = head_exclusive.saturating_sub(retention.maximum_undelivered_sequences);
    let oldest_available = core_oldest_available.max(bounded_oldest);
    if requested_from >= oldest_available {
        return Ok(None);
    }
    let notice = Truncated {
        requested_from,
        oldest_available,
        lost_through: oldest_available.saturating_sub(1),
    };
    subscriptions.mark_truncated_inner(
        target,
        notice.requested_from,
        notice.oldest_available,
        notice.lost_through,
    )?;
    Ok(Some(notice))
}

pub struct GapAuthorization<'a> {
    pub sessions: &'a SessionRegistry,
    pub token: &'a Token,
    pub observability: &'a mut TenantObservability,
    pub core_sequence: u64,
}
