//! Prepared-activity expiry, reservation release and signed-byte retention.

use std::collections::BTreeMap;
use std::sync::Mutex;

use crate::budget::{release, BudgetLimiter, LimitRefusal, ReleaseKind};
use crate::session::SessionRef;

use crate::prepare::EvmPrepared;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum LifecycleState {
    Prepared,
    Signing,
    Signed,
    Submitted,
    Acknowledged,
    Unknown,
    Executed,
    Failed,
    Expired,
}

impl LifecycleState {
    const fn terminal(self) -> bool {
        matches!(self, Self::Executed | Self::Failed | Self::Expired)
    }

    const fn unresolved(self) -> bool {
        matches!(self, Self::Submitted | Self::Acknowledged | Self::Unknown)
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum PayloadRedaction {
    Omit,
    DigestOnly,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct RetainedPreparation {
    authorization: Option<PreparationAuthorization>,
    call_bound: bool,
    durable_terminal: bool,
    state: LifecycleState,
    not_after: u64,
    reservation_ids: Vec<[u8; 32]>,
    signed_bytes: Option<Vec<u8>>,
    activity_id: Option<[u8; 32]>,
    payload_hash: [u8; 32],
    terminal_at_sequence: Option<u64>,
}

/// Exact session generation that owns a token-gated preparation.
#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct PreparationAuthorization {
    pub(crate) session: SessionRef,
    pub(crate) generation: u64,
}

#[derive(Default)]
pub struct PreparationLifecycle {
    records: Mutex<BTreeMap<[u8; 32], RetainedPreparation>>,
}

impl PreparationLifecycle {
    /// Records one prepared activity with its expiry bound and its held reservations.
    ///
    /// # Errors
    ///
    /// Returns `Unavailable` when the record lock is poisoned, or `Duplicate` when the
    /// preparation id is already registered.
    pub fn register(
        &self,
        preparation_id: [u8; 32],
        prepared: &EvmPrepared,
        reservation_ids: Vec<[u8; 32]>,
    ) -> Result<(), LifecycleError> {
        self.register_inner(preparation_id, prepared, reservation_ids, None)
    }

    /// Records a preparation under the exact session generation that authorized it.
    pub(crate) fn register_authorized(
        &self,
        preparation_id: [u8; 32],
        prepared: &EvmPrepared,
        reservation_ids: Vec<[u8; 32]>,
        authorization: PreparationAuthorization,
    ) -> Result<(), LifecycleError> {
        if authorization.generation == 0 {
            return Err(LifecycleError::InvalidAuthorization);
        }
        self.register_inner(
            preparation_id,
            prepared,
            reservation_ids,
            Some(authorization),
        )
    }

    fn register_inner(
        &self,
        preparation_id: [u8; 32],
        prepared: &EvmPrepared,
        reservation_ids: Vec<[u8; 32]>,
        authorization: Option<PreparationAuthorization>,
    ) -> Result<(), LifecycleError> {
        crate::prepare::verify_call_binding(prepared)
            .map_err(|_| LifecycleError::InvalidSignedBytes)?;
        let mut records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        if records.contains_key(&preparation_id) {
            return Err(LifecycleError::Duplicate);
        }
        records.insert(
            preparation_id,
            RetainedPreparation {
                authorization,
                call_bound: false,
                durable_terminal: false,
                state: LifecycleState::Prepared,
                not_after: prepared
                    .disclosure
                    .expiry
                    .not_after
                    .min(prepared.disclosure.expiry.payload_expires_at),
                reservation_ids,
                signed_bytes: None,
                activity_id: None,
                payload_hash: layerx_crypto::evm_transaction::transaction_hash(&prepared.call.data),
                terminal_at_sequence: None,
            },
        );
        Ok(())
    }

    /// Fails every not-yet-submitted preparation owned by an invalidated exact generation while
    /// preserving submitted/unknown work for honest receipt resolution.
    ///
    /// # Errors
    ///
    /// Returns an error if preparation invalidation or reservation release fails.
    pub fn invalidate_authorizations(
        &self,
        invalidated: &[(SessionRef, u64)],
        current_sequence: u64,
        limiter: &BudgetLimiter,
    ) -> Result<PreparationInvalidationReport, LifecycleError> {
        self.invalidate_selected(current_sequence, limiter, |_, record| {
            record.authorization.as_ref().is_some_and(|authorization| {
                invalidated.iter().any(|(session, generation)| {
                    &authorization.session == session && authorization.generation == *generation
                })
            })
        })
    }

    /// Fails every not-yet-submitted preparation named by id (capability revocation) while
    /// preserving submitted/unknown work for honest receipt resolution.
    ///
    /// # Errors
    ///
    /// Returns an error if preparation invalidation or reservation release fails.
    pub fn invalidate_preparations(
        &self,
        preparation_ids: &std::collections::BTreeSet<[u8; 32]>,
        current_sequence: u64,
        limiter: &BudgetLimiter,
    ) -> Result<PreparationInvalidationReport, LifecycleError> {
        self.invalidate_selected(current_sequence, limiter, |preparation_id, _| {
            preparation_ids.contains(preparation_id)
        })
    }

    fn invalidate_selected(
        &self,
        current_sequence: u64,
        limiter: &BudgetLimiter,
        selected: impl Fn(&[u8; 32], &RetainedPreparation) -> bool,
    ) -> Result<PreparationInvalidationReport, LifecycleError> {
        let mut records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        if records
            .iter()
            .any(|(id, record)| record.call_bound && selected(id, record))
        {
            return Err(LifecycleError::AuthorizationRequired);
        }
        let mut report = PreparationInvalidationReport::default();
        for (preparation_id, record) in records.iter_mut() {
            if !selected(preparation_id, &*record) {
                continue;
            }
            match record.state {
                LifecycleState::Prepared | LifecycleState::Signing | LifecycleState::Signed => {
                    for reservation in &record.reservation_ids {
                        if release(limiter, *reservation, ReleaseKind::Failed, current_sequence)
                            .map_err(LifecycleError::Reservation)?
                        {
                            report.released_reservations.push(*reservation);
                        }
                    }
                    record.state = LifecycleState::Failed;
                    record.signed_bytes = None;
                    record.terminal_at_sequence = Some(current_sequence);
                    report.cancelled_preparations += 1;
                }
                LifecycleState::Submitted
                | LifecycleState::Acknowledged
                | LifecycleState::Unknown => {
                    report.unresolved_preserved += 1;
                }
                LifecycleState::Executed | LifecycleState::Failed | LifecycleState::Expired => {
                    report.terminal_untouched += 1;
                }
            }
        }
        Ok(report)
    }

    /// Advances one preparation along the permitted lifecycle edges only.
    ///
    /// # Errors
    ///
    /// Returns `Unavailable` when the record lock is poisoned, `NotFound` for an unregistered
    /// preparation, or `InvalidTransition` carrying both states for a disallowed edge.
    pub fn transition(
        &self,
        preparation_id: [u8; 32],
        next: LifecycleState,
        current_sequence: u64,
    ) -> Result<(), LifecycleError> {
        self.transition_inner(preparation_id, next, current_sequence, None)
    }

    /// Advances a token-bound preparation only for its exact owning session generation.
    pub(crate) fn transition_authorized(
        &self,
        preparation_id: [u8; 32],
        next: LifecycleState,
        current_sequence: u64,
        authorization: &PreparationAuthorization,
    ) -> Result<(), LifecycleError> {
        self.transition_inner(preparation_id, next, current_sequence, Some(authorization))
    }

    fn transition_inner(
        &self,
        preparation_id: [u8; 32],
        next: LifecycleState,
        current_sequence: u64,
        authorization: Option<&PreparationAuthorization>,
    ) -> Result<(), LifecycleError> {
        let mut records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let record = records
            .get_mut(&preparation_id)
            .ok_or(LifecycleError::NotFound)?;
        require_authorization(record, authorization)?;
        if !valid_transition(record.state, next) {
            return Err(LifecycleError::InvalidTransition {
                from: record.state,
                to: next,
            });
        }
        record.state = next;
        if next.terminal() {
            record.terminal_at_sequence = Some(current_sequence);
        }
        Ok(())
    }

    /// Attaches signed bytes and their activity id to a preparation being signed.
    ///
    /// # Errors
    ///
    /// Returns `InvalidSignedBytes` for empty bytes, `Unavailable` when the record lock is
    /// poisoned, `NotFound` for an unregistered preparation, or `InvalidTransition` unless the
    /// record is currently `Signing`.
    pub fn retain_signed_bytes(
        &self,
        preparation_id: [u8; 32],
        signed_bytes: Vec<u8>,
        activity_id: [u8; 32],
    ) -> Result<(), LifecycleError> {
        self.retain_signed_bytes_inner(preparation_id, signed_bytes, activity_id, None)
    }

    /// Retains signed bytes only for the exact session generation that owns the preparation.
    pub(crate) fn retain_signed_bytes_authorized(
        &self,
        preparation_id: [u8; 32],
        signed_bytes: Vec<u8>,
        activity_id: [u8; 32],
        authorization: &PreparationAuthorization,
    ) -> Result<(), LifecycleError> {
        self.retain_signed_bytes_inner(
            preparation_id,
            signed_bytes,
            activity_id,
            Some(authorization),
        )
    }

    fn retain_signed_bytes_inner(
        &self,
        preparation_id: [u8; 32],
        signed_bytes: Vec<u8>,
        activity_id: [u8; 32],
        authorization: Option<&PreparationAuthorization>,
    ) -> Result<(), LifecycleError> {
        if signed_bytes.is_empty() {
            return Err(LifecycleError::InvalidSignedBytes);
        }
        let mut records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let record = records
            .get_mut(&preparation_id)
            .ok_or(LifecycleError::NotFound)?;
        require_authorization(record, authorization)?;
        if record.state != LifecycleState::Signing {
            return Err(LifecycleError::InvalidTransition {
                from: record.state,
                to: LifecycleState::Signed,
            });
        }
        record.signed_bytes = Some(signed_bytes);
        record.activity_id = Some(activity_id);
        record.state = LifecycleState::Signed;
        Ok(())
    }

    /// Admits a signed preparation for submission at the authoritative core batch time.
    ///
    /// # Errors
    ///
    /// Returns `Unavailable` when the record lock is poisoned, `NotFound` for an unregistered
    /// preparation, `PreparationExpired` past `not_after` or once already `Expired`, or
    /// `InvalidTransition` unless the record is `Signed`.
    pub fn admit_submission(
        &self,
        preparation_id: [u8; 32],
        core_batch_time: u64,
    ) -> Result<(), LifecycleError> {
        self.admit_submission_inner(preparation_id, core_batch_time, None)
    }

    /// Admits a token-bound preparation only for its exact owning session generation.
    pub(crate) fn admit_submission_authorized(
        &self,
        preparation_id: [u8; 32],
        core_batch_time: u64,
        authorization: &PreparationAuthorization,
    ) -> Result<(), LifecycleError> {
        self.admit_submission_inner(preparation_id, core_batch_time, Some(authorization))
    }

    fn admit_submission_inner(
        &self,
        preparation_id: [u8; 32],
        core_batch_time: u64,
        authorization: Option<&PreparationAuthorization>,
    ) -> Result<(), LifecycleError> {
        let records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let record = records
            .get(&preparation_id)
            .ok_or(LifecycleError::NotFound)?;
        require_authorization(record, authorization)?;
        require_unexpired(record, core_batch_time)?;
        if record.state != LifecycleState::Signed {
            return Err(LifecycleError::InvalidTransition {
                from: record.state,
                to: LifecycleState::Submitted,
            });
        }
        Ok(())
    }

    /// Checks that a preparation has not expired at the authoritative core batch time.
    ///
    /// # Errors
    ///
    /// Returns `Unavailable` when the record lock is poisoned, `NotFound` for an unregistered
    /// preparation, or `PreparationExpired` past `not_after` or once already `Expired`.
    pub fn check_unexpired(
        &self,
        preparation_id: [u8; 32],
        core_batch_time_ms: u64,
    ) -> Result<(), LifecycleError> {
        self.check_unexpired_inner(preparation_id, core_batch_time_ms, None)
    }

    /// Checks expiry only for the exact owning session generation of a token-bound preparation.
    pub(crate) fn check_unexpired_authorized(
        &self,
        preparation_id: [u8; 32],
        core_batch_time_ms: u64,
        authorization: &PreparationAuthorization,
    ) -> Result<(), LifecycleError> {
        self.check_unexpired_inner(preparation_id, core_batch_time_ms, Some(authorization))
    }

    fn check_unexpired_inner(
        &self,
        preparation_id: [u8; 32],
        core_batch_time_ms: u64,
        authorization: Option<&PreparationAuthorization>,
    ) -> Result<(), LifecycleError> {
        let records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let record = records
            .get(&preparation_id)
            .ok_or(LifecycleError::NotFound)?;
        require_authorization(record, authorization)?;
        require_unexpired(record, core_batch_time_ms)
    }

    /// Returns the current lifecycle state of one preparation.
    ///
    /// # Errors
    ///
    /// Returns `Unavailable` when the record lock is poisoned, or `NotFound` for an
    /// unregistered preparation.
    pub fn state(&self, preparation_id: [u8; 32]) -> Result<LifecycleState, LifecycleError> {
        let records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        records
            .get(&preparation_id)
            .map(|record| record.state)
            .ok_or(LifecycleError::NotFound)
    }

    /// Reports whether signed bytes are still retained for one preparation.
    ///
    /// # Errors
    ///
    /// Returns `Unavailable` when the record lock is poisoned, or `NotFound` for an
    /// unregistered preparation.
    pub fn has_signed_bytes(&self, preparation_id: [u8; 32]) -> Result<bool, LifecycleError> {
        let records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        records
            .get(&preparation_id)
            .map(|record| record.signed_bytes.is_some())
            .ok_or(LifecycleError::NotFound)
    }

    /// Returns the retained signed bytes only for the exact owning session generation.
    pub(crate) fn signed_bytes_authorized(
        &self,
        preparation_id: [u8; 32],
        authorization: &PreparationAuthorization,
    ) -> Result<Vec<u8>, LifecycleError> {
        let records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let record = records
            .get(&preparation_id)
            .ok_or(LifecycleError::NotFound)?;
        require_authorization(record, Some(authorization))?;
        if record.state != LifecycleState::Signed {
            return Err(LifecycleError::InvalidTransition {
                from: record.state,
                to: LifecycleState::Signed,
            });
        }
        record
            .signed_bytes
            .clone()
            .ok_or(LifecycleError::InvalidSignedBytes)
    }

    /// Renders one log line carrying the activity id and never the payload bytes.
    ///
    /// # Errors
    ///
    /// Returns `Unavailable` when the record lock is poisoned, `NotFound` for an unregistered
    /// preparation, or `ActivityIdUnavailable` before signing assigned an activity id.
    pub fn redacted_log(
        &self,
        preparation_id: [u8; 32],
        policy: PayloadRedaction,
    ) -> Result<String, LifecycleError> {
        let records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let record = records
            .get(&preparation_id)
            .ok_or(LifecycleError::NotFound)?;
        let activity_id = record
            .activity_id
            .ok_or(LifecycleError::ActivityIdUnavailable)?;
        let mut line = format!("activity_id={} payload=[redacted]", hex(&activity_id));
        if policy == PayloadRedaction::DigestOnly {
            line.push_str(" payload_hash=");
            line.push_str(&hex(&record.payload_hash));
        }
        Ok(line)
    }

    pub(crate) fn restore_durable(
        &self,
        durable: &[DurablePreparation],
    ) -> Result<usize, LifecycleError> {
        let mut records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        if durable.iter().any(DurablePreparation::is_call_record) {
            return Err(LifecycleError::MigrationRequired);
        }
        let mut restored = BTreeMap::new();
        for record in durable.iter().filter(|record| !record.terminal()) {
            if record.generation == 0 {
                return Err(LifecycleError::InvalidAuthorization);
            }
            let signed_bytes = record.signed_bytes()?;
            if records.contains_key(&record.preparation_id)
                || restored.contains_key(&record.preparation_id)
            {
                return Err(LifecycleError::Duplicate);
            }
            restored.insert(
                record.preparation_id,
                RetainedPreparation {
                    authorization: Some(PreparationAuthorization {
                        session: SessionRef::new(
                            record.tenant.clone(),
                            crate::session::SessionId(record.session_id),
                        ),
                        generation: record.generation,
                    }),
                    call_bound: false,
                    durable_terminal: false,
                    state: record.state,
                    not_after: record.not_after,
                    reservation_ids: vec![record.preparation_id],
                    signed_bytes,
                    activity_id: record.activity_id,
                    payload_hash: record.payload_hash,
                    terminal_at_sequence: None,
                },
            );
        }
        let count = restored.len();
        records.extend(restored);
        Ok(count)
    }

    pub(crate) fn persist_signature(
        &self,
        durable: &mut DurablePreparation,
        next: LifecycleState,
    ) -> Result<bool, LifecycleError> {
        let records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let record = records
            .get(&durable.preparation_id)
            .ok_or(LifecycleError::NotFound)?;
        let owner = PreparationAuthorization {
            session: SessionRef::new(
                durable.tenant.clone(),
                crate::session::SessionId(durable.session_id),
            ),
            generation: durable.generation,
        };
        require_authorization(record, Some(&owner))?;
        if record.state != next {
            return Err(LifecycleError::InvalidTransition {
                from: record.state,
                to: next,
            });
        }
        let signed = match next {
            LifecycleState::Signing => None,
            LifecycleState::Signed => Some((
                record
                    .activity_id
                    .ok_or(LifecycleError::ActivityIdUnavailable)?,
                record
                    .signed_bytes
                    .clone()
                    .ok_or(LifecycleError::InvalidSignedBytes)?,
            )),
            _ => {
                return Err(LifecycleError::InvalidTransition {
                    from: durable.state,
                    to: next,
                })
            }
        };
        let stored = durable
            .signed_bytes()?
            .zip(durable.activity_id)
            .map(|(bytes, activity_id)| (activity_id, bytes));
        if durable.state == next {
            return if stored == signed {
                Ok(false)
            } else {
                Err(LifecycleError::AuthorizationMismatch)
            };
        }
        let reachable = valid_transition(durable.state, next)
            || (valid_transition(durable.state, LifecycleState::Signing)
                && valid_transition(LifecycleState::Signing, next));
        if !reachable {
            return Err(LifecycleError::InvalidTransition {
                from: durable.state,
                to: next,
            });
        }
        durable.state = next;
        if let Some((activity_id, bytes)) = signed {
            durable.activity_id = Some(activity_id);
            durable.extensions.insert(EXTENSION_SIGNED_BYTES, bytes);
        }
        Ok(true)
    }

    pub(crate) fn persist_sent(
        &self,
        durable: &mut DurablePreparation,
        next: LifecycleState,
    ) -> Result<bool, LifecycleError> {
        if !matches!(
            next,
            LifecycleState::Submitted | LifecycleState::Acknowledged
        ) {
            return Err(LifecycleError::InvalidTransition {
                from: durable.state,
                to: next,
            });
        }
        let records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let record = records
            .get(&durable.preparation_id)
            .ok_or(LifecycleError::NotFound)?;
        let owner = PreparationAuthorization {
            session: SessionRef::new(
                durable.tenant.clone(),
                crate::session::SessionId(durable.session_id),
            ),
            generation: durable.generation,
        };
        require_authorization(record, Some(&owner))?;
        if record.state != next {
            return Err(LifecycleError::InvalidTransition {
                from: record.state,
                to: next,
            });
        }
        if durable.state == next {
            return Ok(false);
        }
        if !valid_transition(durable.state, next) {
            return Err(LifecycleError::InvalidTransition {
                from: durable.state,
                to: next,
            });
        }
        let stored = durable.signed_bytes()?;
        if stored.is_none() || stored != record.signed_bytes {
            return Err(LifecycleError::AuthorizationMismatch);
        }
        durable.state = next;
        Ok(true)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ExpirationReport {
    pub expired_preparations: Vec<[u8; 32]>,
    pub released_reservations: Vec<[u8; 32]>,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct RetentionReport {
    pub evicted_terminal_preparations: usize,
    pub discarded_terminal_signed_bytes: usize,
    pub preserved_unresolved_signed_bytes: usize,
}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct PreparationInvalidationReport {
    pub cancelled_preparations: usize,
    pub unresolved_preserved: usize,
    pub terminal_untouched: usize,
    pub released_reservations: Vec<[u8; 32]>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum LifecycleError {
    Duplicate,
    NotFound,
    Unavailable,
    InvalidSignedBytes,
    ActivityIdUnavailable,
    PreparationExpired,
    PreparationNotYetValid,
    InvalidAuthorization,
    AuthorizationRequired,
    AuthorizationMismatch,
    InvalidTransition {
        from: LifecycleState,
        to: LifecycleState,
    },
    Reservation(LimitRefusal),
    CallBinding(crate::prepare::disclose::RestoreCallError),
    MigrationRequired,
}

fn require_unexpired(
    record: &RetainedPreparation,
    core_batch_time_ms: u64,
) -> Result<(), LifecycleError> {
    if core_batch_time_ms > record.not_after || record.state == LifecycleState::Expired {
        return Err(LifecycleError::PreparationExpired);
    }
    Ok(())
}

fn require_authorization(
    record: &RetainedPreparation,
    presented: Option<&PreparationAuthorization>,
) -> Result<(), LifecycleError> {
    if record.call_bound {
        return Err(LifecycleError::AuthorizationRequired);
    }
    match (&record.authorization, presented) {
        (None, None) => Ok(()),
        (Some(expected), Some(presented)) if expected == presented => Ok(()),
        (Some(_), None) => Err(LifecycleError::AuthorizationRequired),
        _ => Err(LifecycleError::AuthorizationMismatch),
    }
}

/// # Errors
/// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
/// or required state, reservations or transport are unavailable.
pub fn expire_elapsed(
    lifecycle: &PreparationLifecycle,
    limiter: &BudgetLimiter,
    core_batch_time_ms: u64,
    current_sequence: u64,
) -> Result<ExpirationReport, LifecycleError> {
    let mut records = lifecycle
        .records
        .lock()
        .map_err(|_| LifecycleError::Unavailable)?;
    if records.values().any(|record| record.call_bound) {
        return Err(LifecycleError::AuthorizationRequired);
    }
    let candidates: Vec<_> = records
        .iter()
        .filter(|(_, record)| {
            matches!(
                record.state,
                LifecycleState::Prepared | LifecycleState::Signing | LifecycleState::Signed
            ) && core_batch_time_ms > record.not_after
        })
        .map(|(id, record)| (*id, record.reservation_ids.clone()))
        .collect();
    let mut report = ExpirationReport {
        expired_preparations: Vec::new(),
        released_reservations: Vec::new(),
    };
    for (preparation_id, reservations) in candidates {
        for reservation_id in reservations {
            if release(
                limiter,
                reservation_id,
                ReleaseKind::Expired,
                current_sequence,
            )
            .map_err(LifecycleError::Reservation)?
            {
                report.released_reservations.push(reservation_id);
            }
        }
        if let Some(record) = records.get_mut(&preparation_id) {
            record.state = LifecycleState::Expired;
            record.terminal_at_sequence = Some(current_sequence);
        }
        report.expired_preparations.push(preparation_id);
    }
    Ok(report)
}

/// # Errors
/// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
/// or required state, reservations or transport are unavailable.
pub fn sweep_retention(
    lifecycle: &PreparationLifecycle,
    current_sequence: u64,
    retention_sequences: u64,
) -> Result<RetentionReport, LifecycleError> {
    let mut records = lifecycle
        .records
        .lock()
        .map_err(|_| LifecycleError::Unavailable)?;
    let mut report = RetentionReport {
        evicted_terminal_preparations: 0,
        discarded_terminal_signed_bytes: 0,
        preserved_unresolved_signed_bytes: 0,
    };
    for record in records.values_mut() {
        if record.state.unresolved() && record.signed_bytes.is_some() {
            report.preserved_unresolved_signed_bytes += 1;
            continue;
        }
        if record.state.terminal()
            && record.signed_bytes.is_some()
            && record.terminal_at_sequence.is_some_and(|terminal| {
                current_sequence >= terminal && current_sequence - terminal >= retention_sequences
            })
        {
            record.signed_bytes = None;
            report.discarded_terminal_signed_bytes += 1;
        }
    }
    records.retain(|_, record| {
        let evict = record.durable_terminal
            && record.state.terminal()
            && record.terminal_at_sequence.is_some_and(|terminal| {
                current_sequence.saturating_sub(terminal) >= retention_sequences
                    && current_sequence >= terminal
            });
        if evict {
            report.evicted_terminal_preparations += 1;
        }
        !evict
    });
    Ok(report)
}

const fn valid_transition(from: LifecycleState, to: LifecycleState) -> bool {
    matches!(
        (from, to),
        (LifecycleState::Prepared, LifecycleState::Signing)
            | (LifecycleState::Signing, LifecycleState::Signed)
            | (LifecycleState::Signed, LifecycleState::Submitted)
            | (
                LifecycleState::Submitted,
                LifecycleState::Acknowledged
                    | LifecycleState::Unknown
                    | LifecycleState::Executed
                    | LifecycleState::Failed,
            )
            | (
                LifecycleState::Acknowledged,
                LifecycleState::Unknown | LifecycleState::Executed | LifecycleState::Failed,
            )
            | (
                LifecycleState::Unknown,
                LifecycleState::Executed | LifecycleState::Failed,
            )
    )
}

fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut output = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        output.push(char::from(DIGITS[usize::from(byte >> 4)]));
        output.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    output
}

/// Extension tag reserved for the capability binding of a preparation.
pub const EXTENSION_CAPABILITY: u16 = 1;
/// Extension tag reserved for an idempotent preparation outcome.
pub const EXTENSION_OUTCOME: u16 = 2;
/// Extension tag written by the admission seam: the activity's 32-byte idempotency key.
pub const EXTENSION_IDEMPOTENCY: u16 = 3;

const EXTENSION_SIGNED_BYTES: u16 = 4;
const PREPARATION_PREFIX: &[u8] = b"prepare/record/";
const PREPARATION_MAGIC: &[u8; 4] = b"LXPR";
const PREPARATION_VERSION: u8 = 1;

const PREPARATION_VERSION_SIGNED: u8 = 2;
const PREPARATION_VERSION_CALL: u8 = 3;
pub const EXTENSION_CALL: u16 = 0xc001;
pub const EXTENSION_TERMINAL_SEQUENCE: u16 = 0xc002;
const EXTENSION_CALL_TOMBSTONE: u16 = 0xc003;
const MAX_SIGNED_BYTES: usize = 1_048_576;

const fn extension_bound(version: u8, tag: u16) -> usize {
    if (version == PREPARATION_VERSION_SIGNED || version == PREPARATION_VERSION_CALL)
        && tag == EXTENSION_SIGNED_BYTES
    {
        MAX_SIGNED_BYTES
    } else if version == PREPARATION_VERSION_CALL && tag == EXTENSION_CALL {
        MAX_PREPARATION_BYTES
    } else {
        MAX_EXTENSION_BYTES
    }
}
const MAX_PREPARATION_BYTES: usize = 1_048_576;
const MAX_EXTENSION_BYTES: usize = 65_536;
const MAX_HOLDS: usize = 64;
const MAX_EXTENSIONS: usize = 64;

/// One tagged extension carried by a durable preparation record.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PreparationExtension {
    pub tag: u16,
    pub bytes: Vec<u8>,
}

/// Durable preparation record holding the authorizing session generation, the lifecycle state,
/// every reservation hold applied to it and opaque tagged extensions, in one store record so
/// publication is atomic.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DurablePreparation {
    pub tenant: crate::store::TenantId,
    pub preparation_id: [u8; 32],
    pub session_id: [u8; 32],
    pub generation: u64,
    pub not_after: u64,
    pub payload_hash: [u8; 32],
    pub state: LifecycleState,
    pub activity_id: Option<[u8; 32]>,
    pub holds: Vec<(
        crate::budget::DurableBudgetReservation,
        Option<crate::budget::CoreTimestampMs>,
    )>,
    pub extensions: BTreeMap<u16, Vec<u8>>,
}

const STATES: [LifecycleState; 9] = [
    LifecycleState::Prepared,
    LifecycleState::Signing,
    LifecycleState::Signed,
    LifecycleState::Submitted,
    LifecycleState::Acknowledged,
    LifecycleState::Unknown,
    LifecycleState::Executed,
    LifecycleState::Failed,
    LifecycleState::Expired,
];

impl DurablePreparation {
    /// Identifier shared by native owner consent, cached signing inputs and durable records.
    #[must_use]
    pub fn native_identifier(call: &layerx_crypto::evm_transaction::Eip1559Call) -> [u8; 32] {
        use sha2::{Digest as _, Sha256};
        Sha256::digest(call.encode_unsigned()).into()
    }

    pub(crate) fn authorization_reference_key(
        tenant: &crate::store::TenantId,
        envelope_digest: [u8; 32],
    ) -> Result<crate::store::TenantKey, LifecycleError> {
        crate::store::TenantKey::new(
            tenant.clone(),
            crate::store::ObjectKind::Configuration,
            [
                b"prepare/authorization-reference/".as_slice(),
                &envelope_digest,
            ]
            .concat(),
        )
        .map_err(|_| LifecycleError::Unavailable)
    }

    /// Whether the record reached a terminal state.
    #[must_use]
    pub const fn terminal(&self) -> bool {
        self.state.terminal()
    }

    /// Store key of one tenant's durable preparation record.
    ///
    /// # Errors
    ///
    /// Returns `Unavailable` when the key cannot be constructed.
    pub fn store_key(
        tenant: &crate::store::TenantId,
        preparation_id: [u8; 32],
    ) -> Result<crate::store::TenantKey, LifecycleError> {
        crate::store::TenantKey::new(
            tenant.clone(),
            crate::store::ObjectKind::Configuration,
            [PREPARATION_PREFIX, &preparation_id].concat(),
        )
        .map_err(|_| LifecycleError::Unavailable)
    }

    /// Lists the preparation identifiers durably recorded for one tenant.
    ///
    /// # Errors
    ///
    /// Returns `InvalidSignedBytes` for a record identifier that is not 32 bytes.
    pub fn recorded_ids(
        store: &crate::store::Store,
        tenant: &crate::store::TenantId,
    ) -> Result<Vec<[u8; 32]>, LifecycleError> {
        store
            .list_object_ids(tenant, crate::store::ObjectKind::Configuration)
            .into_iter()
            .filter_map(|id| {
                id.strip_prefix(PREPARATION_PREFIX).map(|rest| {
                    <[u8; 32]>::try_from(rest).map_err(|_| LifecycleError::InvalidSignedBytes)
                })
            })
            .collect()
    }

    /// Canonical record bytes.
    ///
    /// # Errors
    ///
    /// Returns `InvalidSignedBytes` when a bound is exceeded.
    pub fn encode(&self) -> Result<Vec<u8>, LifecycleError> {
        use crate::budget::LimitScope;
        let invalid = |_| LifecycleError::InvalidSignedBytes;
        if self.holds.len() > MAX_HOLDS || self.extensions.len() > MAX_EXTENSIONS {
            return Err(LifecycleError::InvalidSignedBytes);
        }
        let version = if self.is_call_record() {
            PREPARATION_VERSION_CALL
        } else if self
            .extensions
            .get(&EXTENSION_SIGNED_BYTES)
            .is_some_and(|bytes| bytes.len() > MAX_EXTENSION_BYTES)
        {
            PREPARATION_VERSION_SIGNED
        } else {
            PREPARATION_VERSION
        };
        let capacity = if version == PREPARATION_VERSION {
            MAX_PREPARATION_BYTES
        } else {
            MAX_PREPARATION_BYTES + MAX_SIGNED_BYTES
        };
        let mut encoder = layerx_wire::encode::Encoder::new(capacity);
        encoder.fixed(PREPARATION_MAGIC).map_err(invalid)?;
        encoder.u8(version).map_err(invalid)?;
        encoder.fixed(&self.preparation_id).map_err(invalid)?;
        encoder.fixed(&self.session_id).map_err(invalid)?;
        encoder.u64(self.generation).map_err(invalid)?;
        encoder.u64(self.not_after).map_err(invalid)?;
        encoder.fixed(&self.payload_hash).map_err(invalid)?;
        let state = STATES
            .iter()
            .position(|state| *state == self.state)
            .and_then(|position| u8::try_from(position).ok())
            .ok_or(LifecycleError::InvalidSignedBytes)?;
        encoder.u8(state).map_err(invalid)?;
        match self.activity_id {
            Some(activity_id) => {
                encoder.u8(1).map_err(invalid)?;
                encoder.fixed(&activity_id).map_err(invalid)?;
            }
            None => encoder.u8(0).map_err(invalid)?,
        }
        encoder
            .u16(u16::try_from(self.holds.len()).map_err(|_| LifecycleError::InvalidSignedBytes)?)
            .map_err(invalid)?;
        for (hold, deadline) in &self.holds {
            let (tag, identity) = match hold.scope {
                LimitScope::Tenant(value) => (0_u8, value),
                LimitScope::Agent(value) => (1, value),
                LimitScope::Session(value) => (2, value),
                LimitScope::Capability(value) => (3, value),
                LimitScope::Counterparty(value) => (4, value),
            };
            encoder.fixed(&hold.reservation_id).map_err(invalid)?;
            encoder.fixed(&hold.limit_id.0).map_err(invalid)?;
            encoder.u8(tag).map_err(invalid)?;
            encoder.fixed(&identity).map_err(invalid)?;
            encoder.u128(hold.amount).map_err(invalid)?;
            encoder.u128(hold.ceiling).map_err(invalid)?;
            encoder.u64(hold.expiry_sequence).map_err(invalid)?;
            encoder.fixed(&hold.digest).map_err(invalid)?;
            match deadline {
                Some(deadline) => {
                    encoder.u8(1).map_err(invalid)?;
                    encoder.u64(deadline.0).map_err(invalid)?;
                }
                None => encoder.u8(0).map_err(invalid)?,
            }
        }
        encoder
            .u16(
                u16::try_from(self.extensions.len())
                    .map_err(|_| LifecycleError::InvalidSignedBytes)?,
            )
            .map_err(invalid)?;
        for (tag, bytes) in &self.extensions {
            if bytes.len() > extension_bound(version, *tag) {
                return Err(LifecycleError::InvalidSignedBytes);
            }
            encoder.u16(*tag).map_err(invalid)?;
            encoder
                .u32(u32::try_from(bytes.len()).map_err(|_| LifecycleError::InvalidSignedBytes)?)
                .map_err(invalid)?;
            encoder.fixed(bytes).map_err(invalid)?;
        }
        Ok(encoder.finish())
    }

    /// Decodes one canonical record; unknown extension tags are kept opaque and re-encode
    /// byte-identically.
    ///
    /// # Errors
    ///
    /// Returns `InvalidSignedBytes` for any malformed, non-canonical or trailing byte.
    pub fn decode(tenant: crate::store::TenantId, bytes: &[u8]) -> Result<Self, LifecycleError> {
        let invalid = |_| LifecycleError::InvalidSignedBytes;
        let mut decoder = layerx_wire::decode::Decoder::new(bytes, 0);

        crate::reject_if(
            preparation_fixed::<4>(&mut decoder)? != *PREPARATION_MAGIC,
            || LifecycleError::InvalidSignedBytes,
        )?;
        let version = decoder.u8().map_err(invalid)?;
        crate::reject_if(
            ![
                PREPARATION_VERSION,
                PREPARATION_VERSION_SIGNED,
                PREPARATION_VERSION_CALL,
            ]
            .contains(&version),
            || LifecycleError::InvalidSignedBytes,
        )?;
        let preparation_id = preparation_fixed::<32>(&mut decoder)?;
        let session_id = preparation_fixed::<32>(&mut decoder)?;
        let generation = decoder.u64().map_err(invalid)?;
        let not_after = decoder.u64().map_err(invalid)?;
        let payload_hash = preparation_fixed::<32>(&mut decoder)?;
        let state = *STATES
            .get(usize::from(decoder.u8().map_err(invalid)?))
            .ok_or(LifecycleError::InvalidSignedBytes)?;
        let activity_id = match decoder.u8().map_err(invalid)? {
            0 => None,
            1 => Some(preparation_fixed::<32>(&mut decoder)?),
            _ => return Err(LifecycleError::InvalidSignedBytes),
        };
        let holds = decode_preparation_holds(&mut decoder, preparation_id)?;
        let extension_count = usize::from(decoder.u16().map_err(invalid)?);
        crate::reject_if(extension_count > MAX_EXTENSIONS, || {
            LifecycleError::InvalidSignedBytes
        })?;
        let mut extensions = BTreeMap::new();
        let mut previous: Option<u16> = None;
        for _ in 0..extension_count {
            let tag = decoder.u16().map_err(invalid)?;
            let length = usize::try_from(decoder.u32().map_err(invalid)?)
                .map_err(|_| LifecycleError::InvalidSignedBytes)?;
            crate::reject_if(
                previous.is_some_and(|previous| tag <= previous)
                    || length > extension_bound(version, tag),
                || LifecycleError::InvalidSignedBytes,
            )?;
            extensions.insert(tag, decoder.fixed(length).map_err(invalid)?.to_vec());
            previous = Some(tag);
        }
        decoder.finish().map_err(invalid)?;
        crate::reject_if(
            version == PREPARATION_VERSION_SIGNED
                && extensions
                    .get(&EXTENSION_SIGNED_BYTES)
                    .is_none_or(|bytes| bytes.len() <= MAX_EXTENSION_BYTES),
            || LifecycleError::InvalidSignedBytes,
        )?;
        let call_record = extensions.contains_key(&EXTENSION_CALL)
            || extensions.contains_key(&EXTENSION_CALL_TOMBSTONE);
        crate::reject_if(call_record != (version == PREPARATION_VERSION_CALL), || {
            LifecycleError::InvalidSignedBytes
        })?;
        Ok(Self {
            tenant,
            preparation_id,
            session_id,
            generation,
            not_after,
            payload_hash,
            state,
            activity_id,
            holds,
            extensions,
        })
    }

    /// Strictly decodes every durable preparation record of one tenant.
    ///
    /// # Errors
    ///
    /// Returns `InvalidSignedBytes` for a malformed record identifier, a malformed record or a
    /// record whose identifier differs from its key, and `NotFound` for a listed key without a
    /// value.
    pub fn load_all(
        store: &crate::store::Store,
        tenant: &crate::store::TenantId,
    ) -> Result<Vec<Self>, LifecycleError> {
        store
            .list_object_ids(tenant, crate::store::ObjectKind::Configuration)
            .into_iter()
            .filter_map(|id| id.strip_prefix(PREPARATION_PREFIX).map(<[u8]>::to_vec))
            .map(|rest| {
                let preparation_id = <[u8; 32]>::try_from(rest.as_slice())
                    .map_err(|_| LifecycleError::InvalidSignedBytes)?;
                let key = Self::store_key(tenant, preparation_id)?;
                let stored = store.get(&key).ok_or(LifecycleError::NotFound)?;
                let record = Self::decode(tenant.clone(), stored.bytes())?;
                if record.preparation_id != preparation_id {
                    return Err(LifecycleError::InvalidSignedBytes);
                }
                Ok(record)
            })
            .collect()
    }

    pub(crate) fn signed_bytes(&self) -> Result<Option<Vec<u8>>, LifecycleError> {
        let bytes = self.extensions.get(&EXTENSION_SIGNED_BYTES);
        let unsigned = matches!(
            self.state,
            LifecycleState::Prepared | LifecycleState::Signing
        );
        match bytes {
            None if unsigned && self.activity_id.is_none() => Ok(None),
            Some(bytes) if !unsigned && !bytes.is_empty() && self.activity_id.is_some() => {
                Ok(Some(bytes.clone()))
            }
            _ if self.terminal() => Ok(bytes.cloned()),
            _ => Err(LifecycleError::InvalidSignedBytes),
        }
    }

    pub(crate) fn drop_signed_bytes(&mut self) {
        self.extensions.remove(&EXTENSION_SIGNED_BYTES);
    }
}

impl DurablePreparation {
    #[must_use]
    pub fn is_call_record(&self) -> bool {
        self.extensions.contains_key(&EXTENSION_CALL)
            || self.extensions.contains_key(&EXTENSION_CALL_TOMBSTONE)
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn from_call(
        session: &SessionRef,
        generation: u64,
        preparation_id: [u8; 32],
        prepared: &crate::prepare::EvmPrepared,
        holds: Vec<(
            crate::budget::DurableBudgetReservation,
            Option<crate::budget::CoreTimestampMs>,
        )>,
    ) -> Result<Self, LifecycleError> {
        use sha2::{Digest, Sha256};
        crate::prepare::verify_call_binding(prepared).map_err(|error| {
            LifecycleError::CallBinding(crate::prepare::disclose::RestoreCallError::Binding(error))
        })?;
        if generation == 0 {
            return Err(LifecycleError::InvalidAuthorization);
        }
        if holds.iter().any(|(hold, _)| {
            hold.reservation_id != preparation_id || hold.digest != hold.canonical_digest()
        }) {
            return Err(LifecycleError::InvalidSignedBytes);
        }
        let invalid = |_| LifecycleError::InvalidSignedBytes;
        let mut out = layerx_wire::encode::Encoder::new(MAX_PREPARATION_BYTES);
        out.u8(1).map_err(invalid)?;
        out.bytes(session.tenant.as_str().as_bytes(), 255)
            .map_err(invalid)?;
        out.fixed(&session.session_id.0).map_err(invalid)?;
        out.u64(generation).map_err(invalid)?;
        out.fixed(&preparation_id).map_err(invalid)?;
        out.fixed(&prepared.from).map_err(invalid)?;
        out.u64(prepared.observed_head_height).map_err(invalid)?;
        out.fixed(&prepared.disclosure_digest).map_err(invalid)?;
        out.fixed(&prepared.signing_hash).map_err(invalid)?;
        out.bytes(&prepared.call.encode_unsigned(), MAX_PREPARATION_BYTES)
            .map_err(invalid)?;
        let encoded = out.finish();
        let payload_hash = Sha256::digest(&encoded).into();
        let not_after = prepared
            .disclosure
            .expiry
            .not_after
            .min(prepared.disclosure.expiry.payload_expires_at);
        let record = Self {
            tenant: session.tenant.clone(),
            preparation_id,
            session_id: session.session_id.0,
            generation,
            not_after,
            payload_hash,
            state: LifecycleState::Prepared,
            activity_id: None,
            holds,
            extensions: BTreeMap::from([
                (EXTENSION_CALL, encoded),
                (
                    EXTENSION_IDEMPOTENCY,
                    prepared.disclosure.idempotency_key.to_vec(),
                ),
            ]),
        };
        record.encode()?;
        Ok(record)
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn restore_call(
        &self,
        session: &SessionRef,
        generation: u64,
    ) -> Result<crate::prepare::EvmPrepared, LifecycleError> {
        fn fixed<const N: usize>(
            input: &mut layerx_wire::decode::Decoder<'_>,
        ) -> Result<[u8; N], LifecycleError> {
            input
                .fixed(N)
                .map_err(|_| LifecycleError::InvalidSignedBytes)?
                .try_into()
                .map_err(|_| LifecycleError::InvalidSignedBytes)
        }

        use sha2::{Digest, Sha256};
        self.require_call_owner(session, generation)?;
        let encoded = self
            .extensions
            .get(&EXTENSION_CALL)
            .ok_or(if self.is_call_record() {
                LifecycleError::NotFound
            } else {
                LifecycleError::MigrationRequired
            })?;
        let hash: [u8; 32] = Sha256::digest(encoded).into();
        if hash != self.payload_hash {
            return Err(LifecycleError::InvalidSignedBytes);
        }
        let invalid = |_| LifecycleError::InvalidSignedBytes;
        let mut input = layerx_wire::decode::Decoder::new(encoded, 0);

        if input.u8().map_err(invalid)? != 1
            || input.bytes(255).map_err(invalid)? != session.tenant.as_str().as_bytes()
            || fixed::<32>(&mut input)? != session.session_id.0
            || input.u64().map_err(invalid)? != generation
            || fixed::<32>(&mut input)? != self.preparation_id
        {
            return Err(LifecycleError::AuthorizationMismatch);
        }
        let from = fixed::<20>(&mut input)?;
        let head = input.u64().map_err(invalid)?;
        let digest = fixed::<32>(&mut input)?;
        let signing_hash = fixed::<32>(&mut input)?;
        let call = input.bytes(MAX_PREPARATION_BYTES).map_err(invalid)?;
        input.finish().map_err(invalid)?;
        let prepared =
            crate::prepare::disclose::restore_call(call, from, head, digest, signing_hash)
                .map_err(LifecycleError::CallBinding)?;
        if self.not_after
            != prepared
                .disclosure
                .expiry
                .not_after
                .min(prepared.disclosure.expiry.payload_expires_at)
        {
            return Err(LifecycleError::InvalidSignedBytes);
        }
        if let Some(raw) = self.signed_bytes()? {
            self.verify_signed_call(&prepared, &raw)?;
        }
        Ok(prepared)
    }

    fn require_call_owner(
        &self,
        session: &SessionRef,
        generation: u64,
    ) -> Result<(), LifecycleError> {
        if generation == 0 || self.generation == 0 {
            return Err(LifecycleError::InvalidAuthorization);
        }
        if self.tenant != session.tenant
            || self.session_id != session.session_id.0
            || self.generation != generation
        {
            return Err(LifecycleError::AuthorizationMismatch);
        }
        Ok(())
    }

    fn verify_signed_call(
        &self,
        prepared: &crate::prepare::EvmPrepared,
        raw: &[u8],
    ) -> Result<(), LifecycleError> {
        use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
        if raw.len() > MAX_SIGNED_BYTES {
            return Err(LifecycleError::InvalidSignedBytes);
        }
        let (call, signature) =
            decode_signed(raw).map_err(|_| LifecycleError::InvalidSignedBytes)?;
        if call != prepared.call
            || recover_sender(&call, &signature).map_err(|_| LifecycleError::InvalidSignedBytes)?
                != prepared.from
            || self.activity_id != Some(transaction_hash(raw))
        {
            return Err(LifecycleError::InvalidSignedBytes);
        }
        Ok(())
    }

    fn terminal_sequence(&self) -> Result<Option<u64>, LifecycleError> {
        self.extensions
            .get(&EXTENSION_TERMINAL_SEQUENCE)
            .map(|bytes| {
                let bytes: [u8; 8] = bytes
                    .as_slice()
                    .try_into()
                    .map_err(|_| LifecycleError::InvalidSignedBytes)?;
                Ok(u64::from_be_bytes(bytes))
            })
            .transpose()
    }

    fn call_idempotency_key(
        &self,
        prepared: &crate::prepare::EvmPrepared,
    ) -> Result<crate::store::TenantKey, LifecycleError> {
        let key = if prepared.disclosure.idempotency_key == [0; 32] {
            prepared.signing_hash
        } else {
            prepared.disclosure.idempotency_key
        };
        crate::store::TenantKey::new(
            self.tenant.clone(),
            crate::store::ObjectKind::Idempotency,
            [
                b"prepare/call/".as_slice(),
                &prepared.call.chain_id.to_be_bytes(),
                &prepared.from,
                &key,
            ]
            .concat(),
        )
        .map_err(|_| LifecycleError::Unavailable)
    }

    fn load_call(
        store: &crate::store::Store,
        session: &SessionRef,
        generation: u64,
        id: [u8; 32],
    ) -> Result<(Self, crate::prepare::EvmPrepared), LifecycleError> {
        let key = Self::store_key(&session.tenant, id)?;
        let stored = store.get(&key).ok_or(LifecycleError::NotFound)?;
        let record = Self::decode(session.tenant.clone(), stored.bytes())?;
        if record.preparation_id != id {
            return Err(LifecycleError::InvalidSignedBytes);
        }
        let prepared = record.restore_call(session, generation)?;
        Ok((record, prepared))
    }

    fn save_call(&self, store: &mut crate::store::Store) -> Result<(), LifecycleError> {
        let key = Self::store_key(&self.tenant, self.preparation_id)?;
        store
            .put_local(key, self.encode()?)
            .map_err(|_| LifecycleError::Unavailable)
    }

    fn retained_call(&self) -> Result<RetainedPreparation, LifecycleError> {
        Ok(RetainedPreparation {
            authorization: None,
            call_bound: true,
            durable_terminal: self.terminal(),
            state: self.state,
            not_after: self.not_after,
            reservation_ids: if self.holds.is_empty() {
                Vec::new()
            } else {
                vec![self.preparation_id]
            },
            signed_bytes: self.signed_bytes()?,
            activity_id: self.activity_id,
            payload_hash: self.payload_hash,
            terminal_at_sequence: self.terminal_sequence()?,
        })
    }
}

impl PreparationLifecycle {
    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn retained_count(&self) -> Result<usize, LifecycleError> {
        self.records
            .lock()
            .map(|records| records.len())
            .map_err(|_| LifecycleError::Unavailable)
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn register_call(
        &self,
        store: &mut crate::store::Store,
        session: &SessionRef,
        generation: u64,
        id: [u8; 32],
        prepared: &crate::prepare::EvmPrepared,
        holds: Vec<(
            crate::budget::DurableBudgetReservation,
            Option<crate::budget::CoreTimestampMs>,
        )>,
    ) -> Result<(), LifecycleError> {
        let mut records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let record = DurablePreparation::from_call(session, generation, id, prepared, holds)?;
        let key = DurablePreparation::store_key(&session.tenant, id)?;
        let idempotency = record.call_idempotency_key(prepared)?;
        if records.contains_key(&id)
            || store.get(&key).is_some()
            || store.get(&idempotency).is_some()
        {
            return Err(LifecycleError::Duplicate);
        }
        let retained = record.retained_call()?;
        store
            .commit_local_records(vec![
                (key, record.encode()?),
                (
                    idempotency,
                    [id.as_slice(), &prepared.signing_hash].concat(),
                ),
            ])
            .map_err(|_| LifecycleError::Unavailable)?;
        records.insert(id, retained);
        Ok(())
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn restore_calls(
        &self,
        store: &crate::store::Store,
        tenant: &crate::store::TenantId,
    ) -> Result<usize, LifecycleError> {
        let durable = DurablePreparation::load_all(store, tenant)?;
        let mut records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let mut restored = BTreeMap::new();
        for record in durable {
            if !record.is_call_record() {
                return Err(LifecycleError::MigrationRequired);
            }
            if record.terminal() {
                continue;
            }
            let session =
                SessionRef::new(tenant.clone(), crate::session::SessionId(record.session_id));
            let prepared = record.restore_call(&session, record.generation)?;
            let identity = record.call_idempotency_key(&prepared)?;
            let expected = [record.preparation_id.as_slice(), &prepared.signing_hash].concat();
            if store
                .get(&identity)
                .map(super::super::store::StoredValue::bytes)
                != Some(expected.as_slice())
            {
                return Err(LifecycleError::InvalidSignedBytes);
            }
            if records.contains_key(&record.preparation_id)
                || restored.contains_key(&record.preparation_id)
            {
                return Err(LifecycleError::Duplicate);
            }
            restored.insert(record.preparation_id, record.retained_call()?);
        }
        let count = restored.len();
        records.extend(restored);
        Ok(count)
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    #[allow(
        clippy::unused_self,
        reason = "Preserve the public API signature used by daemon consumers."
    )]
    pub fn prepared_call(
        &self,
        store: &crate::store::Store,
        session: &SessionRef,
        generation: u64,
        id: [u8; 32],
    ) -> Result<crate::prepare::EvmPrepared, LifecycleError> {
        let (_, prepared) = DurablePreparation::load_call(store, session, generation, id)?;
        Ok(prepared)
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn transition_call(
        &self,
        store: &mut crate::store::Store,
        session: &SessionRef,
        generation: u64,
        id: [u8; 32],
        next: LifecycleState,
        current_sequence: u64,
    ) -> Result<(), LifecycleError> {
        let mut records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let (mut durable, _) = DurablePreparation::load_call(store, session, generation, id)?;
        require_cached_call(&records, id, &durable)?;
        if matches!(next, LifecycleState::Signed | LifecycleState::Submitted)
            || !valid_transition(durable.state, next)
        {
            return Err(LifecycleError::InvalidTransition {
                from: durable.state,
                to: next,
            });
        }
        durable.state = next;
        if next.terminal() {
            durable.extensions.insert(
                EXTENSION_TERMINAL_SEQUENCE,
                current_sequence.to_be_bytes().to_vec(),
            );
        }
        let retained = durable.retained_call()?;
        durable.save_call(store)?;
        records.insert(id, retained);
        Ok(())
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn retain_signed_call(
        &self,
        store: &mut crate::store::Store,
        session: &SessionRef,
        generation: u64,
        id: [u8; 32],
        raw: Vec<u8>,
    ) -> Result<[u8; 32], LifecycleError> {
        let mut records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let (mut durable, prepared) =
            DurablePreparation::load_call(store, session, generation, id)?;
        require_cached_call(&records, id, &durable)?;
        if durable.state != LifecycleState::Signing {
            return Err(LifecycleError::InvalidTransition {
                from: durable.state,
                to: LifecycleState::Signed,
            });
        }
        let hash = layerx_crypto::evm_transaction::transaction_hash(&raw);
        durable.activity_id = Some(hash);
        durable.verify_signed_call(&prepared, &raw)?;
        durable.state = LifecycleState::Signed;
        durable.extensions.insert(EXTENSION_SIGNED_BYTES, raw);
        let retained = durable.retained_call()?;
        durable.save_call(store)?;
        records.insert(id, retained);
        Ok(hash)
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn admit_call_submission(
        &self,
        store: &mut crate::store::Store,
        session: &SessionRef,
        generation: u64,
        id: [u8; 32],
        core_time_ms: u64,
    ) -> Result<Vec<u8>, LifecycleError> {
        let mut records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let (mut durable, prepared) =
            DurablePreparation::load_call(store, session, generation, id)?;
        require_cached_call(&records, id, &durable)?;
        let retained = records.get(&id).ok_or(LifecycleError::NotFound)?;
        require_unexpired(retained, core_time_ms)?;
        if core_time_ms < prepared.disclosure.expiry.not_before {
            return Err(LifecycleError::PreparationNotYetValid);
        }
        if core_time_ms > durable.not_after || durable.state == LifecycleState::Expired {
            return Err(LifecycleError::PreparationExpired);
        }
        if durable.state != LifecycleState::Signed {
            return Err(LifecycleError::InvalidTransition {
                from: durable.state,
                to: LifecycleState::Submitted,
            });
        }
        let raw = durable
            .signed_bytes()?
            .ok_or(LifecycleError::InvalidSignedBytes)?;
        durable.state = LifecycleState::Submitted;
        let retained = durable.retained_call()?;
        durable.save_call(store)?;
        records.insert(id, retained);
        Ok(raw)
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    #[allow(
        clippy::unused_self,
        reason = "Preserve the public API signature used by daemon consumers."
    )]
    pub fn recovered_signed_call(
        &self,
        store: &crate::store::Store,
        session: &SessionRef,
        generation: u64,
        id: [u8; 32],
    ) -> Result<Vec<u8>, LifecycleError> {
        let (durable, _) = DurablePreparation::load_call(store, session, generation, id)?;
        durable
            .signed_bytes()?
            .ok_or(LifecycleError::InvalidSignedBytes)
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn expire_calls(
        &self,
        store: &mut crate::store::Store,
        tenant: &crate::store::TenantId,
        limiter: &BudgetLimiter,
        core_time_ms: u64,
        current_sequence: u64,
    ) -> Result<ExpirationReport, LifecycleError> {
        let mut records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let mut report = ExpirationReport {
            expired_preparations: Vec::new(),
            released_reservations: Vec::new(),
        };
        for mut durable in DurablePreparation::load_all(store, tenant)? {
            if !durable.is_call_record() {
                return Err(LifecycleError::MigrationRequired);
            }
            if !matches!(
                durable.state,
                LifecycleState::Prepared | LifecycleState::Signing | LifecycleState::Signed
            ) || core_time_ms <= durable.not_after
            {
                continue;
            }
            let session = SessionRef::new(
                tenant.clone(),
                crate::session::SessionId(durable.session_id),
            );
            durable.restore_call(&session, durable.generation)?;
            if records.contains_key(&durable.preparation_id) {
                require_cached_call(&records, durable.preparation_id, &durable)?;
            }
            // Call expiry is in milliseconds, independently of the reservation's height bound.
            let staged = crate::budget::stage_release(
                limiter,
                durable.preparation_id,
                ReleaseKind::Failed,
                current_sequence,
            )
            .map_err(LifecycleError::Reservation)?;
            durable.state = LifecycleState::Expired;
            durable.extensions.insert(
                EXTENSION_TERMINAL_SEQUENCE,
                current_sequence.to_be_bytes().to_vec(),
            );
            let retained = durable.retained_call()?;
            durable.save_call(store)?;
            if staged.publish() {
                report.released_reservations.push(durable.preparation_id);
            }
            records.insert(durable.preparation_id, retained);
            report.expired_preparations.push(durable.preparation_id);
        }
        Ok(report)
    }

    /// # Errors
    /// Returns an error if the request, ownership, deadline or lifecycle transition is invalid,
    /// or required state, reservations or transport are unavailable.
    pub fn sweep_call_retention(
        &self,
        store: &mut crate::store::Store,
        tenant: &crate::store::TenantId,
        current_sequence: u64,
        retention_sequences: u64,
    ) -> Result<RetentionReport, LifecycleError> {
        let mut records = self
            .records
            .lock()
            .map_err(|_| LifecycleError::Unavailable)?;
        let mut report = RetentionReport {
            evicted_terminal_preparations: 0,
            discarded_terminal_signed_bytes: 0,
            preserved_unresolved_signed_bytes: 0,
        };
        for mut durable in DurablePreparation::load_all(store, tenant)? {
            if !durable.is_call_record() {
                return Err(LifecycleError::MigrationRequired);
            }
            if durable.state.unresolved() {
                if durable.signed_bytes()?.is_some() {
                    report.preserved_unresolved_signed_bytes += 1;
                }
                continue;
            }
            let Some(terminal) = durable.terminal_sequence()? else {
                continue;
            };
            if !durable.terminal()
                || current_sequence < terminal
                || current_sequence - terminal < retention_sequences
            {
                continue;
            }
            if durable.extensions.contains_key(&EXTENSION_CALL_TOMBSTONE) {
                continue;
            }
            let session = SessionRef::new(
                tenant.clone(),
                crate::session::SessionId(durable.session_id),
            );
            let prepared = durable.restore_call(&session, durable.generation)?;
            let identity = durable.call_idempotency_key(&prepared)?;
            let expected = [durable.preparation_id.as_slice(), &prepared.signing_hash].concat();
            if store
                .get(&identity)
                .map(super::super::store::StoredValue::bytes)
                != Some(expected.as_slice())
            {
                return Err(LifecycleError::InvalidSignedBytes);
            }
            if records.contains_key(&durable.preparation_id) {
                require_cached_call(&records, durable.preparation_id, &durable)?;
            }
            let had_bytes = durable.extensions.contains_key(&EXTENSION_SIGNED_BYTES);
            durable.drop_signed_bytes();
            durable.extensions.remove(&EXTENSION_CALL);
            durable
                .extensions
                .insert(EXTENSION_CALL_TOMBSTONE, prepared.signing_hash.to_vec());
            durable.holds.clear();
            durable.save_call(store)?;
            if records.remove(&durable.preparation_id).is_some() {
                report.evicted_terminal_preparations += 1;
            }
            if had_bytes {
                report.discarded_terminal_signed_bytes += 1;
            }
        }
        Ok(report)
    }
}

fn require_cached_call(
    records: &BTreeMap<[u8; 32], RetainedPreparation>,
    id: [u8; 32],
    durable: &DurablePreparation,
) -> Result<(), LifecycleError> {
    let cached = records.get(&id).ok_or(LifecycleError::NotFound)?;
    if !cached.call_bound
        || cached.payload_hash != durable.payload_hash
        || cached.state != durable.state
    {
        return Err(LifecycleError::AuthorizationMismatch);
    }
    Ok(())
}

fn preparation_fixed<const N: usize>(
    decoder: &mut layerx_wire::decode::Decoder<'_>,
) -> Result<[u8; N], LifecycleError> {
    decoder
        .fixed(N)
        .map_err(|_| LifecycleError::InvalidSignedBytes)?
        .try_into()
        .map_err(|_| LifecycleError::InvalidSignedBytes)
}
fn decode_preparation_holds(
    decoder: &mut layerx_wire::decode::Decoder<'_>,
    preparation_id: [u8; 32],
) -> Result<Vec<BoundPreparationHold>, LifecycleError> {
    use crate::budget::{CoreTimestampMs, DurableBudgetReservation, LimitId, LimitScope};
    let invalid = |_| LifecycleError::InvalidSignedBytes;
    let hold_count = usize::from(decoder.u16().map_err(invalid)?);
    crate::reject_if(hold_count > MAX_HOLDS, || {
        LifecycleError::InvalidSignedBytes
    })?;
    let mut holds = Vec::with_capacity(hold_count);
    for _ in 0..hold_count {
        let reservation_id = preparation_fixed::<32>(decoder)?;
        let limit_id = LimitId(preparation_fixed::<16>(decoder)?);
        let tag = decoder.u8().map_err(invalid)?;
        let identity = preparation_fixed::<32>(decoder)?;
        let scope = match tag {
            0 => LimitScope::Tenant(identity),
            1 => LimitScope::Agent(identity),
            2 => LimitScope::Session(identity),
            3 => LimitScope::Capability(identity),
            4 => LimitScope::Counterparty(identity),
            _ => return Err(LifecycleError::InvalidSignedBytes),
        };
        let hold = DurableBudgetReservation {
            reservation_id,
            limit_id,
            scope,
            amount: decoder.u128().map_err(invalid)?,
            ceiling: decoder.u128().map_err(invalid)?,
            expiry_sequence: decoder.u64().map_err(invalid)?,
            digest: preparation_fixed::<32>(decoder)?,
        };
        let deadline = match decoder.u8().map_err(invalid)? {
            0 => None,
            1 => Some(CoreTimestampMs(decoder.u64().map_err(invalid)?)),
            _ => return Err(LifecycleError::InvalidSignedBytes),
        };
        crate::reject_if(
            hold.reservation_id != preparation_id || hold.digest != hold.canonical_digest(),
            || LifecycleError::InvalidSignedBytes,
        )?;
        holds.push((hold, deadline));
    }
    Ok(holds)
}

type BoundPreparationHold = (
    crate::budget::DurableBudgetReservation,
    Option<crate::budget::CoreTimestampMs>,
);
