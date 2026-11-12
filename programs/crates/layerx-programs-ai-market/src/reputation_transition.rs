use crate::types::GuestIndex as _;
use crate::{
    codec::{Reader, Writer},
    errors::{
        ARITHMETIC, CAPACITY, CONFLICT, CodecResult, F07_BINDING_MISMATCH, F07_EPOCH_NOT_SEALED,
        F07_GENERATION_MISMATCH, F07_SEGMENT_MISMATCH, F07_UNKNOWN_WORKER, NON_CANONICAL,
        RETENTION_FULL, WRONG_EPOCH,
    },
    reputation::{
        CURRENT_BYTES, ClosureReason, CompletedHistory, CompletionIdentity, HEADER_BYTES,
        HISTORY_BYTES, HistoryStatus, LIMIT, Observation, ReputationCurrent, ReputationState,
        SegmentKey, check_storage_budget, evidence_coverage, missing_quality, next_count,
        qualified_quality,
    },
    reputation_codec::{
        closure_digest, decode_section, encode_section, segment_digest, state_root,
    },
    types::{Digest32, FrozenBinding, Presence, Score, Version, WorkerId, WorkerRosterEntry},
};

/// Configuration, policy and model a new reputation segment binds to.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct SegmentBinding {
    pub config: Version,
    pub policy: Digest32,
    pub model: Digest32,
}

/// Closes `old`'s segment for `reason` and opens the next generation bound to `next`.
///
/// # Errors
/// Propagates `ReputationCurrent::matches_segment`'s `NON_CANONICAL`/`F07_SEGMENT_MISMATCH` refusals; returns `NON_CANONICAL` when `height` is zero or before the last transition; `F07_BINDING_MISMATCH` when the config moves backwards or `reason` disagrees with the binding change; `WRONG_EPOCH` when the last applied epoch is not completed; `ARITHMETIC` when the generation overflows; propagates the digest and validation refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn reset_segment(
    old: &ReputationCurrent,
    old_key: SegmentKey,
    next: SegmentBinding,
    reason: ClosureReason,
    latest_completed: Presence<u64>,
    height: u64,
) -> CodecResult<(SegmentKey, ReputationCurrent)> {
    old.matches_segment(old_key)?;
    if height == 0 || height < old.last_transition_height {
        return Err(NON_CANONICAL);
    }
    let SegmentBinding {
        config: new_config,
        policy: new_policy,
        model: new_model,
    } = next;
    if new_config < old_key.config {
        return Err(F07_BINDING_MISMATCH);
    }
    if let Presence::Present(applied) = old.last_applied {
        match latest_completed {
            Presence::Present(last) if last >= applied => (),
            _ => return Err(WRONG_EPOCH),
        }
    }
    match reason {
        ClosureReason::ModelChanged if new_model == old_key.model => {
            return Err(F07_BINDING_MISMATCH);
        }
        ClosureReason::PolicyChanged
            if new_model != old_key.model
                || new_config == old_key.config && new_policy == old_key.policy =>
        {
            return Err(F07_BINDING_MISMATCH);
        }
        _ => (),
    }
    let generation = old.reset_generation.next()?;
    let key = SegmentKey {
        market: old_key.market,
        worker: old_key.worker,
        config: new_config,
        policy: new_policy,
        model: new_model,
        reset_generation: generation,
    };
    let value = ReputationCurrent {
        worker: old.worker,
        segment: segment_digest(key)?,
        owner: old.owner,
        reset_generation: generation,
        quality: Score::new(0)?,
        qualifying_count: 0,
        coverage: Presence::Absent,
        last_applied: latest_completed,
        last_observed: Presence::Absent,
        last_transition_height: height,
        previous_segment: Presence::Present(closure_digest(old_key.market, old, reason)?),
        status: old.status,
    };
    value.validate()?;
    Ok((key, value))
}
/// Rolls `old` over to a new segment when its config, policy or model changed.
///
/// # Errors
/// Propagates `ReputationCurrent::matches_segment`'s and `reset_segment`'s refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn rollover_segment(
    old: &ReputationCurrent,
    old_key: SegmentKey,
    config: Version,
    policy: Digest32,
    model: Digest32,
    latest_completed: Presence<u64>,
    height: u64,
) -> CodecResult<Presence<(SegmentKey, ReputationCurrent)>> {
    rollover_bound(
        old,
        old_key,
        &SegmentBinding {
            config,
            policy,
            model,
        },
        latest_completed,
        height,
    )
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn rollover_bound(
    old: &ReputationCurrent,
    old_key: SegmentKey,
    next: &SegmentBinding,
    latest_completed: Presence<u64>,
    height: u64,
) -> CodecResult<Presence<(SegmentKey, ReputationCurrent)>> {
    old.matches_segment(old_key)?;
    if old_key.config == next.config && old_key.policy == next.policy && old_key.model == next.model
    {
        return Ok(Presence::Absent);
    }
    let reason = if old_key.model == next.model {
        ClosureReason::PolicyChanged
    } else {
        ClosureReason::ModelChanged
    };
    Ok(Presence::Present(reset_segment(
        old,
        old_key,
        *next,
        reason,
        latest_completed,
        height,
    )?))
}

// Internal numerical primitive. The real F05/F06 bridge must select these inputs
// from authenticated shared producer state; no public score-vector entry exists.
/// One worker's epoch observation: aggregated median, supporting and eligible evaluators.
#[allow(
    dead_code,
    reason = "crate-internal F07 primitive: its F05/F06 bridge caller is not in tree; tests/reputation_vectors.rs drives it via #[path]"
)]
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub(crate) struct EpochTally {
    pub(crate) median: Presence<Score>,
    pub(crate) support: u8,
    pub(crate) eligible: u8,
}

#[allow(
    dead_code,
    reason = "crate-internal F07 primitive: its F05/F06 bridge caller is not in tree; tests/reputation_vectors.rs drives it via #[path]"
)]
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn update_record(
    record: &ReputationCurrent,
    key: SegmentKey,
    frozen: FrozenBinding,
    worker: &WorkerRosterEntry,
    tally: EpochTally,
    history_allowed: bool,
    height: u64,
) -> CodecResult<ReputationCurrent> {
    update_record_inner(
        record,
        key,
        frozen,
        worker,
        tally,
        history_allowed,
        height,
        false,
    )
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn update_record_inner(
    record: &ReputationCurrent,
    key: SegmentKey,
    frozen: FrozenBinding,
    worker: &WorkerRosterEntry,
    tally: EpochTally,
    history_allowed: bool,
    height: u64,
    committed: bool,
) -> CodecResult<ReputationCurrent> {
    let EpochTally {
        median,
        support,
        eligible,
    } = tally;
    if committed {
        if record.worker != key.worker || record.reset_generation != key.reset_generation {
            return Err(F07_SEGMENT_MISMATCH);
        }
    } else {
        record.matches_segment(key)?;
    }
    if key.market != frozen.market
        || key.config != frozen.config
        || worker.worker != key.worker
        || record.owner != worker.owner
    {
        return Err(F07_BINDING_MISMATCH);
    }
    if height == 0 || height < record.last_transition_height {
        return Err(NON_CANONICAL);
    }
    if let Presence::Present(e) = record.last_applied {
        if frozen.epoch <= e {
            return Err(WRONG_EPOCH);
        }
    }
    let coverage = evidence_coverage(eligible, support)?;
    if matches!(median, Presence::Present(_)) != (support >= 3) {
        return Err(F07_BINDING_MISMATCH);
    }
    let mut next = *record;
    let observation = if history_allowed {
        median
    } else {
        Presence::Absent
    };
    match observation {
        Presence::Present(s) => {
            next.quality = qualified_quality(record.quality, s)?;
            next.qualifying_count = next_count(record.qualifying_count, true)?;
            next.last_observed = Presence::Present(Observation {
                epoch: frozen.epoch,
                height,
            });
        }
        Presence::Absent => {
            next.quality = missing_quality(record.quality)?;
        }
    }
    next.coverage = coverage;
    next.last_applied = Presence::Present(frozen.epoch);
    next.last_transition_height = height;
    next.validate()?;
    Ok(next)
}

#[allow(
    dead_code,
    reason = "crate-internal F07 primitive: its F05/F06 bridge caller is not in tree; tests/reputation_vectors.rs drives it via #[path]"
)]
#[allow(
    clippy::large_enum_variant,
    reason = "no_std without alloc: Box is unavailable, and the large Pending draft is the primary path"
)]
pub(crate) enum CompletionStart {
    Retained(CompletedHistory),
    Pending(CompletionDraft),
}
#[allow(
    dead_code,
    reason = "crate-internal F07 primitive: its F05/F06 bridge caller is not in tree; tests/reputation_vectors.rs drives it via #[path]"
)]
pub(crate) struct CompletionDraft {
    staged: ReputationState,
    frozen: FrozenBinding,
    result: Digest32,
    height: u64,
    roster: [Option<WorkerRosterEntry>; LIMIT],
    roster_len: usize,
    record_cursor: usize,
    cursor: usize,
    observed: u8,
    covered: u8,
    committed: bool,
}
#[allow(
    dead_code,
    reason = "crate-internal F07 primitive: its F05/F06 bridge caller is not in tree; tests/reputation_vectors.rs drives it via #[path]"
)]
impl CompletionDraft {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn begin(
        state: &ReputationState,
        frozen: FrozenBinding,
        result: Digest32,
        height: u64,
        roster: &[WorkerRosterEntry],
    ) -> CodecResult<CompletionStart> {
        Self::begin_inner(state, frozen, result, height, roster, false)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn begin_inner(
        state: &ReputationState,
        frozen: FrozenBinding,
        result: Digest32,
        height: u64,
        roster: &[WorkerRosterEntry],
        committed: bool,
    ) -> CodecResult<CompletionStart> {
        if state.market != frozen.market {
            return Err(F07_BINDING_MISMATCH);
        }
        let identity = if committed {
            state.assess_committed_completion(frozen.epoch, result)?
        } else {
            state.assess_completion(frozen.epoch, result)?
        };
        match identity {
            CompletionIdentity::Retained(record) => {
                if record.config != frozen.config {
                    return Err(F07_BINDING_MISMATCH);
                }
                return Ok(CompletionStart::Retained(record));
            }
            CompletionIdentity::New => (),
        }
        if height == 0 {
            return Err(NON_CANONICAL);
        }
        if roster.len() > LIMIT {
            return Err(CAPACITY);
        }
        if state.history_len as usize == LIMIT {
            return Err(RETENTION_FULL);
        }
        if let Some(last) = state.completed().last() {
            if height < last.execution_height {
                return Err(NON_CANONICAL);
            }
        }
        for record in state.records() {
            if let Presence::Present(e) = record.last_applied {
                if e >= frozen.epoch {
                    return Err(WRONG_EPOCH);
                }
            }
        }
        let mut previous = None;
        let mut bounded_roster = [None; LIMIT];
        let mut record_cursor = 0;
        for (i, worker) in roster.iter().enumerate() {
            if previous.is_some_and(|id| id >= worker.worker) {
                return Err(NON_CANONICAL);
            }
            let index = state.ordered_position(worker.worker, &mut record_cursor);
            let current = state
                .current
                .get(index)
                .and_then(Option::as_ref)
                .filter(|r| r.worker == worker.worker)
                .ok_or(F07_UNKNOWN_WORKER)?;
            if current.owner != worker.owner {
                return Err(F07_BINDING_MISMATCH);
            }
            previous = Some(worker.worker);
            *bounded_roster.guest_index_mut(i) = Some(*worker);
        }
        Ok(CompletionStart::Pending(Self {
            staged: state.clone(),
            frozen,
            result,
            height,
            roster: bounded_roster,
            roster_len: roster.len(),
            record_cursor: 0,
            cursor: 0,
            observed: 0,
            covered: 0,
            committed,
        }))
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn apply_worker(
        &mut self,
        key: SegmentKey,
        median: Presence<Score>,
        support: u8,
        eligible: u8,
        history_allowed: bool,
    ) -> CodecResult<()> {
        let worker = self
            .roster
            .get(self.cursor)
            .and_then(Option::as_ref)
            .ok_or(WRONG_EPOCH)?;
        let index = self
            .staged
            .ordered_position(worker.worker, &mut self.record_cursor);
        let old = self
            .staged
            .current
            .guest_index(index)
            .as_ref()
            .ok_or(NON_CANONICAL)?;
        let tally = EpochTally {
            median,
            support,
            eligible,
        };
        let next = update_record_inner(
            old,
            key,
            self.frozen,
            worker,
            tally,
            history_allowed,
            self.height,
            self.committed,
        )?;
        let observed = self
            .observed
            .checked_add(u8::from(
                history_allowed && matches!(median, Presence::Present(_)),
            ))
            .ok_or(ARITHMETIC)?;
        let covered = self
            .covered
            .checked_add(u8::from(eligible > 0))
            .ok_or(ARITHMETIC)?;
        *self.staged.current.guest_index_mut(index) = Some(next);
        self.cursor += 1;
        self.observed = observed;
        self.covered = covered;
        Ok(())
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn finish(mut self) -> CodecResult<(ReputationState, CompletedHistory)> {
        if self.cursor != self.roster_len {
            return Err(F07_EPOCH_NOT_SEALED);
        }
        let root = state_root(&self.staged, self.frozen.epoch)?;
        let record = CompletedHistory {
            epoch: self.frozen.epoch,
            execution_height: self.height,
            config: self.frozen.config,
            result: self.result,
            root,
            observed_workers: self.observed,
            total_workers: u8::try_from(self.roster_len).map_err(|_| CAPACITY)?,
            covered_workers: self.covered,
        };
        // begin checked completion identity, retention and monotonic height against
        // the original state. apply_worker changes only current records; append to
        // that unchanged history, then validate the complete pending epoch once.
        let at = usize::from(self.staged.history_len);
        *self.staged.history.get_mut(at).ok_or(RETENTION_FULL)? = Some(record);
        self.staged.history_len = self.staged.history_len.checked_add(1).ok_or(ARITHMETIC)?;
        self.staged.completed_through = Presence::Present(record.epoch);
        // Every replacement was validated by update_record. The unchanged
        // records/history came from the checked input; validate the appended row.
        record.validate()?;
        self.staged.encoded_size()?;
        Ok((self.staged, record))
    }
}

const SECTION_MAGIC: [u8; 4] = *b"RP07";
const SEAL_MAGIC: [u8; 4] = *b"HA07";
/// Most evaluators one frozen epoch can count as eligible.
const MAX_ELIGIBLE: u8 = 8;
/// Bytes of the per-epoch history seal: `HA07`, epoch:u64, allowed bitmap:u32, eligible:u8
/// and three reserved zero bytes.
pub const SEAL_BYTES: usize = 20;

/// The F07 admission frozen at one epoch's aggregation seal. Bit `i` of `allowed` is the
/// `history_allowed` bit of the `i`-th frozen worker in ascending `WorkerId` order; `eligible`
/// is the number of frozen evaluators still eligible at the seal (the coverage denominator).
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct HistorySeal {
    pub epoch: u64,
    pub allowed: u32,
    pub eligible: u8,
}
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
impl HistorySeal {
    /// The sealed `history_allowed` bit of the frozen worker at ascending roster `index`.
    #[must_use]
    pub fn allows(&self, index: usize) -> bool {
        index < LIMIT && (self.allowed >> index) & 1 == 1
    }
}

/// The F07 region that leads the joint F07/F08 section: the reputation state and the history
/// seal of the epoch whose aggregation is sealed and not yet completed.
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
#[derive(Clone, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Region {
    pub state: ReputationState,
    pub seal: Presence<HistorySeal>,
}

/// Splits the joint F07/F08 section `RP07 section || [HA07 seal] || F08 table` into its F07
/// region and the F08 table bytes. A section that does not start with `RP07` has no F07
/// region; an F08 table starts with its `u16` schema version, so neither magic can begin it.
///
/// # Errors
/// Returns `CAPACITY` when a header count exceeds `LIMIT`; `NON_CANONICAL` for a truncated
/// region or seal, nonzero reserved seal bytes, more than eight eligible evaluators or a seal
/// for an already completed epoch; propagates `decode_section`'s refusals.
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn split_joint(section: &[u8]) -> CodecResult<(Option<Region>, &[u8])> {
    #[cfg(target_arch = "wasm32")]
    {
        static DECODED: crate::guest_cell::GuestCell<
            crate::guest_cell::Cached<(&'static [u8], Option<Region>, usize)>,
        > = crate::guest_cell::GuestCell::new(crate::guest_cell::Cached::Empty);
        let mut decoded = DECODED.try_borrow_mut().ok_or(NON_CANONICAL)?;
        if let Some((bytes, region, offset)) = decoded.as_ref() {
            if core::ptr::eq(*bytes, section) {
                return Ok((region.clone(), &section[*offset..]));
            }
            if let Some(region) = region {
                let base_len = *offset
                    - if matches!(region.seal, Presence::Present(_)) {
                        SEAL_BYTES
                    } else {
                        0
                    };
                if section
                    .get(..base_len)
                    .is_some_and(|prefix| crate::codec::equal_bytes(&bytes[..base_len], prefix))
                {
                    // Only the base reputation bytes are reused. Always parse the current
                    // optional seal, including when a producer adds or removes it.
                    return joint_tail(region.state.clone(), &section[base_len..]);
                }
            }
        }
        if let Some(frozen) = crate::state::current_slice(section) {
            let (region, table) = split_joint_uncached(section)?;
            *decoded = crate::guest_cell::Cached::Value((
                frozen,
                region.clone(),
                section.len() - table.len(),
            ));
            return Ok((region, table));
        }
    }
    split_joint_uncached(section)
}

fn split_joint_uncached(section: &[u8]) -> CodecResult<(Option<Region>, &[u8])> {
    split_joint_inner(section, false)
}

pub(crate) fn split_committed_joint(section: &[u8]) -> CodecResult<(Option<Region>, &[u8])> {
    split_joint_inner(section, true)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn split_joint_inner(section: &[u8], committed: bool) -> CodecResult<(Option<Region>, &[u8])> {
    if section.get(..4) != Some(SECTION_MAGIC.as_slice()) {
        return Ok((None, section));
    }
    let counts = section.get(6..8).ok_or(NON_CANONICAL)?;
    let (current, history) = (
        usize::from(*counts.guest_index(0)),
        usize::from(*counts.guest_index(1)),
    );
    if current > LIMIT || history > LIMIT {
        return Err(CAPACITY);
    }
    let len = HEADER_BYTES + current * CURRENT_BYTES + history * HISTORY_BYTES;
    let (bytes, rest) = section.split_at_checked(len).ok_or(NON_CANONICAL)?;
    let state = crate::reputation_codec::decode_section_records(bytes, committed)?;
    joint_tail(state, rest)
}

fn joint_tail(state: ReputationState, rest: &[u8]) -> CodecResult<(Option<Region>, &[u8])> {
    if rest.get(..4) != Some(SEAL_MAGIC.as_slice()) {
        return Ok((
            Some(Region {
                state,
                seal: Presence::Absent,
            }),
            rest,
        ));
    }
    let (seal, table) = rest.split_at_checked(SEAL_BYTES).ok_or(NON_CANONICAL)?;
    let mut r = Reader::new(seal);
    r.take(4)?;
    let seal = HistorySeal {
        epoch: r.u64()?,
        allowed: r.u32()?,
        eligible: r.u8()?,
    };
    r.reserved(3)?;
    r.finish()?;
    check_seal(&state, seal).map_err(|_| NON_CANONICAL)?;
    Ok((
        Some(Region {
            state,
            seal: Presence::Present(seal),
        }),
        table,
    ))
}

/// A bounded opening pass. Unchanged participant records and history remain encoded.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn open_segments_chunk(
    original: &[u8],
    staged: &[u8],
    previous: SegmentBinding,
    next: SegmentBinding,
    roster: &[WorkerRosterEntry],
    departed: impl Fn(WorkerId) -> CodecResult<bool>,
    height: u64,
    start: usize,
    end: usize,
    out: &mut [u8],
) -> CodecResult<usize> {
    // Bind each committed frame and reject an unfinished history seal.
    for section in [original, staged] {
        if !framed_admission_table(section)?.is_empty() {
            return Err(NON_CANONICAL);
        }
        let counts = section.get(6..8).ok_or(NON_CANONICAL)?;
        if section.len()
            != HEADER_BYTES
                + usize::from(*counts.guest_index(0)) * CURRENT_BYTES
                + usize::from(*counts.guest_index(1)) * HISTORY_BYTES
        {
            return Err(NON_CANONICAL);
        }
    }
    if roster.len() > LIMIT || start > end || end > LIMIT {
        return Err(NON_CANONICAL);
    }
    let mut r = Reader::new(original.guest_index(8..49));
    let market = r.market_id()?;
    let completed = r.boolean()?;
    let epoch = r.u64()?;
    #[cfg(target_arch = "wasm32")]
    let same_header =
        crate::codec::equal_bytes(staged.guest_index(8..49), original.guest_index(8..49));
    #[cfg(not(target_arch = "wasm32"))]
    let same_header = staged.guest_index(8..49) == original.guest_index(8..49);
    if !same_header {
        return Err(F07_BINDING_MISMATCH);
    }
    let latest = if completed {
        Presence::Present(epoch)
    } else {
        Presence::Absent
    };
    let mut len = staged.len();
    out.get_mut(..len)
        .ok_or(CAPACITY)?
        .guest_copy_from_slice(staged);
    let count = usize::from(*original.get(6).ok_or(NON_CANONICAL)?);
    let records = original
        .get(HEADER_BYTES..HEADER_BYTES + count * CURRENT_BYTES)
        .ok_or(NON_CANONICAL)?;
    for bytes in records
        .chunks_exact(CURRENT_BYTES)
        .skip(start)
        .take(end - start)
    {
        let old = crate::reputation_codec::decode_current(bytes)?;
        if old.status == HistoryStatus::Retired {
            continue;
        }
        if height == 0 || height < old.last_transition_height {
            return Err(NON_CANONICAL);
        }
        let mut record = old;
        if departed(old.worker)? {
            record.status = HistoryStatus::Retired;
            record.last_transition_height = height;
        } else if previous != next {
            let key = SegmentKey {
                market,
                worker: old.worker,
                config: previous.config,
                policy: previous.policy,
                model: previous.model,
                reset_generation: old.reset_generation,
            };
            if let Presence::Present((_, rolled)) =
                rollover_bound(&old, key, &next, latest, height)?
            {
                record = rolled;
            }
        }
        write_opened_record(out, &mut len, &record)?;
    }
    for entry in roster.guest_index(start.min(roster.len())..end.min(roster.len())) {
        let (at, present) = opened_record_position(out.guest_index(..len), &entry.worker);
        if present {
            let r = crate::reputation_codec::decode_current(
                out.get(at..at + CURRENT_BYTES).ok_or(NON_CANONICAL)?,
            )?;
            if r.owner != entry.owner || r.status == HistoryStatus::Retired {
                return Err(F07_BINDING_MISMATCH);
            }
        } else {
            let key = SegmentKey {
                market,
                worker: entry.worker,
                config: next.config,
                policy: next.policy,
                model: next.model,
                reset_generation: Version::new(1)?,
            };
            write_opened_record(
                out,
                &mut len,
                &ReputationCurrent::bootstrap(key, entry.owner, height)?,
            )?;
        }
    }
    check_storage_budget(len, 0, 0)?;
    Ok(len)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn opened_record_position(bytes: &[u8], worker: &WorkerId) -> (usize, bool) {
    // Both input frames were bounded and length-checked; insertion preserves
    // that invariant and refuses the thirty-third participant.
    let count = usize::from(*bytes.guest_index(6));
    #[cfg(target_arch = "wasm32")]
    let target = worker.as_bytes();
    #[cfg(not(target_arch = "wasm32"))]
    let target: &[u8] = worker.as_bytes();
    let records = bytes.guest_index(HEADER_BYTES..HEADER_BYTES + count * CURRENT_BYTES);
    for (i, row) in records.chunks_exact(CURRENT_BYTES).enumerate() {
        let id = row.guest_index(..32);
        if id >= target {
            return (HEADER_BYTES + i * CURRENT_BYTES, id == target);
        }
    }
    (HEADER_BYTES + count * CURRENT_BYTES, false)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn write_opened_record(
    out: &mut [u8],
    len: &mut usize,
    record: &ReputationCurrent,
) -> CodecResult<()> {
    let (at, present) = opened_record_position(out.guest_index(..*len), &record.worker);
    if !present {
        if usize::from(*out.guest_index(6)) == LIMIT {
            return Err(CAPACITY);
        }
        let end = len.checked_add(CURRENT_BYTES).ok_or(ARITHMETIC)?;
        out.get_mut(..end)
            .ok_or(CAPACITY)?
            .guest_copy_within(at..*len, at + CURRENT_BYTES);
        *len = end;
        *out.guest_index_mut(6) += 1;
    }
    crate::reputation_codec::write_current(
        record,
        &mut Writer::new(out.get_mut(at..at + CURRENT_BYTES).ok_or(CAPACITY)?),
    )
}

/// Retain the same history suffix as `prune_released`, carrying current bytes verbatim.
#[inline(always)]
pub(crate) fn prune_opened_history(
    out: &mut [u8],
    len: usize,
    retained: impl Fn(u64) -> CodecResult<bool>,
) -> CodecResult<usize> {
    let start = HEADER_BYTES + usize::from(*out.guest_index(6)) * CURRENT_BYTES;
    let mut removed = 0;
    for bytes in out
        .get(start..len)
        .ok_or(NON_CANONICAL)?
        .chunks_exact(HISTORY_BYTES)
    {
        // Only the epoch selects retained committed history; every other
        // field is carried byte-for-byte from its validated producer.
        if retained(Reader::new(bytes).u64()?)? {
            break;
        }
        removed += 1;
    }
    *out.guest_index_mut(7) = (*out.guest_index(7))
        .checked_sub(removed as u8)
        .ok_or(NON_CANONICAL)?;
    if usize::from(*out.guest_index(7)) == LIMIT {
        return Err(RETENTION_FULL);
    }
    let removed = removed * HISTORY_BYTES;
    out.guest_copy_within(start + removed..len, start);
    Ok(len - removed)
}

/// Resume a completion draft whose prior participant replacements are privately journaled.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn complete_epoch_chunk(
    original: &ReputationState,
    staged: &ReputationState,
    seal: Presence<HistorySeal>,
    frozen: FrozenBinding,
    binding: SegmentBinding,
    result: Digest32,
    height: u64,
    roster: &[WorkerRosterEntry],
    observations: &[WorkerObservation],
    cursor: usize,
    observed: u8,
    covered: u8,
) -> CodecResult<(ReputationState, usize, u8, u8)> {
    let seal = match seal {
        Presence::Present(seal) if seal.epoch == frozen.epoch => seal,
        _ => return Err(F07_EPOCH_NOT_SEALED),
    };
    if original.market != frozen.market
        || observations.len() != roster.len()
        || binding.config != frozen.config
        || cursor > roster.len()
    {
        return Err(F07_BINDING_MISMATCH);
    }
    match original.assess_committed_completion(frozen.epoch, result)? {
        CompletionIdentity::New => {}
        CompletionIdentity::Retained(_) => return Err(NON_CANONICAL),
    }
    if height == 0 {
        return Err(NON_CANONICAL);
    }
    if original.history_len as usize == LIMIT {
        return Err(RETENTION_FULL);
    }
    if original
        .completed()
        .last()
        .is_some_and(|last| height < last.execution_height)
    {
        return Err(NON_CANONICAL);
    }
    for record in original.records() {
        if matches!(record.last_applied, Presence::Present(epoch) if epoch >= frozen.epoch) {
            return Err(WRONG_EPOCH);
        }
    }
    let mut state = staged.clone();
    let end = (cursor + crate::reputation::participant_chunk_size(roster.len())).min(roster.len());
    let mut observed = observed;
    let mut covered = covered;
    let mut record_cursor = 0;
    for i in cursor..end {
        let entry = roster.guest_index(i);
        let at = state.ordered_position(entry.worker, &mut record_cursor);
        let old = state
            .current
            .guest_index(at)
            .as_ref()
            .filter(|r| r.worker == entry.worker)
            .ok_or(F07_UNKNOWN_WORKER)?;
        let key = SegmentKey {
            market: original.market,
            worker: entry.worker,
            config: binding.config,
            policy: binding.policy,
            model: binding.model,
            reset_generation: old.reset_generation,
        };
        let observation = observations.guest_index(i);
        let tally = EpochTally {
            median: observation.median,
            support: observation.support,
            eligible: seal.eligible,
        };
        let next =
            update_record_inner(old, key, frozen, entry, tally, seal.allows(i), height, true)?;
        *state.current.guest_index_mut(at) = Some(next);
        observed = observed
            .checked_add(u8::from(
                seal.allows(i) && matches!(observation.median, Presence::Present(_)),
            ))
            .ok_or(ARITHMETIC)?;
        covered = covered
            .checked_add(u8::from(seal.eligible > 0))
            .ok_or(ARITHMETIC)?;
    }
    if end == roster.len() {
        let root = state_root(&state, frozen.epoch)?;
        let record = CompletedHistory {
            epoch: frozen.epoch,
            execution_height: height,
            config: frozen.config,
            result,
            root,
            observed_workers: observed,
            total_workers: roster.len() as u8,
            covered_workers: covered,
        };
        record.validate()?;
        let at = usize::from(state.history_len);
        *state.history.get_mut(at).ok_or(RETENTION_FULL)? = Some(record);
        state.history_len = state.history_len.checked_add(1).ok_or(ARITHMETIC)?;
        state.completed_through = Presence::Present(frozen.epoch);
        state.encoded_size()?;
    }
    Ok((state, end, observed, covered))
}

/// Validated F08 suffix of the joint reputation/admission section.
///
/// # Errors
/// Preserves every framing, reputation and history-seal refusal of `split_joint`.
#[allow(dead_code, reason = "also compiled by reputation vector tests")]
pub fn joint_table(section: &[u8]) -> CodecResult<&[u8]> {
    split_joint(section).map(|(_, table)| table)
}

/// Locates the admission suffix without materializing reputation records. Callers
/// that carry this prefix bind its exact bytes by length and digest.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn framed_admission_table(section: &[u8]) -> CodecResult<&[u8]> {
    if section.get(..4) != Some(SECTION_MAGIC.as_slice()) {
        return Ok(section);
    }
    let header = section.get(..HEADER_BYTES).ok_or(NON_CANONICAL)?;
    if header.guest_index(4..6) != &1u16.to_be_bytes() {
        return Err(crate::errors::BAD_VERSION);
    }
    let current = usize::from(*header.guest_index(6));
    let history = usize::from(*header.guest_index(7));
    if current > LIMIT || history > LIMIT {
        return Err(CAPACITY);
    }
    #[cfg(target_arch = "wasm32")]
    Reader::new(header.guest_index(8..40)).checked_identity()?;
    #[cfg(not(target_arch = "wasm32"))]
    Reader::new(header.guest_index(8..40)).market_id()?;
    let mut r = Reader::new(header.guest_index(40..));
    let completed = r.boolean()?;
    let epoch = r.u64()?;
    if !completed && epoch != 0 {
        return Err(NON_CANONICAL);
    }
    r.reserved(15)?;
    let rest = section
        .get(HEADER_BYTES + current * CURRENT_BYTES + history * HISTORY_BYTES..)
        .ok_or(NON_CANONICAL)?;
    if rest.get(..4) != Some(SEAL_MAGIC.as_slice()) {
        return Ok(rest);
    }
    let mut r = Reader::new(rest);
    r.take(4)?;
    let sealed_epoch = r.u64()?;
    r.take(4)?;
    let eligible = r.u8()?;
    r.reserved(3)?;
    if eligible > MAX_ELIGIBLE || completed && sealed_epoch <= epoch {
        return Err(NON_CANONICAL);
    }
    r.take(r.remaining())
}

fn check_seal(state: &ReputationState, seal: HistorySeal) -> CodecResult<()> {
    if seal.eligible > MAX_ELIGIBLE {
        return Err(F07_BINDING_MISMATCH);
    }
    if let Presence::Present(last) = state.latest_completed() {
        if seal.epoch <= last {
            return Err(WRONG_EPOCH);
        }
    }
    Ok(())
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn write_seal(seal: HistorySeal, w: &mut Writer<'_>) -> CodecResult<()> {
    #[cfg(target_arch = "wasm32")]
    return crate::codec::write_fields!(w;
        put: &SEAL_MAGIC;
        u64: seal.epoch;
        u32: seal.allowed;
        u8: seal.eligible;
        put: &[0; 3];
    );
    #[cfg(not(target_arch = "wasm32"))]
    {
        w.put(&SEAL_MAGIC)?;
        w.u64(seal.epoch)?;
        w.u32(seal.allowed)?;
        w.u8(seal.eligible)?;
        w.put(&[0; 3])
    }
}

/// Encodes `region` followed by the F08 `table` into `out` and returns the written length.
///
/// # Errors
/// Propagates `encode_section`'s refusals; returns `NON_CANONICAL` for an invalid seal;
/// `F07_RESOURCE_LIMIT` when the F07 or joint size exceeds its cap; `CAPACITY` when `out` is
/// too short.
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_joint(region: &Region, table: &[u8], out: &mut [u8]) -> CodecResult<usize> {
    region.state.validate()?;
    encode_produced_joint(region, table, out)
}

/// For the epoch composer: its input was decoded and each replacement/append
/// was checked by the transition that produced it. Public encoders stay strict.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn encode_produced_joint(
    region: &Region,
    table: &[u8],
    out: &mut [u8],
) -> CodecResult<usize> {
    let mut len = crate::reputation_codec::encode_produced_section(&region.state, out)?;
    if let Presence::Present(seal) = region.seal {
        check_seal(&region.state, seal).map_err(|_| NON_CANONICAL)?;
        let mut w = Writer::new(out.get_mut(len..).ok_or(CAPACITY)?);
        write_seal(seal, &mut w)?;
        len += SEAL_BYTES;
    }
    check_storage_budget(len, table.len(), 0)?;
    let end = len.checked_add(table.len()).ok_or(ARITHMETIC)?;
    out.get_mut(len..end)
        .ok_or(CAPACITY)?
        .guest_copy_from_slice(table);
    Ok(end)
}

/// Freezes `history_allowed` for the ascending frozen `workers` of `epoch` at its aggregation
/// seal: a worker's bit is set exactly when its history status is `Active`.
///
/// # Errors
/// Returns `CAPACITY` for more than `LIMIT` workers; `F07_BINDING_MISMATCH` for more than eight
/// eligible evaluators; `WRONG_EPOCH` for an already completed epoch; `NON_CANONICAL` for
/// unordered workers; `F07_UNKNOWN_WORKER` for a frozen worker without a record.
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn seal_history(
    state: &ReputationState,
    epoch: u64,
    workers: &[WorkerId],
    eligible: u8,
) -> CodecResult<HistorySeal> {
    if workers.len() > LIMIT {
        return Err(CAPACITY);
    }
    let mut seal = HistorySeal {
        epoch,
        allowed: 0,
        eligible,
    };
    check_seal(state, seal)?;
    let mut previous = None;
    let mut cursor = 0;
    for (i, worker) in workers.iter().enumerate() {
        if previous.is_some_and(|id| id >= *worker) {
            return Err(NON_CANONICAL);
        }
        previous = Some(*worker);
        let index = state.ordered_position(*worker, &mut cursor);
        let record = state
            .current
            .get(index)
            .and_then(Option::as_ref)
            .filter(|r| r.worker == *worker)
            .ok_or(F07_UNKNOWN_WORKER)?;
        if record.status == HistoryStatus::Active {
            seal.allowed |= 1u32 << i;
        }
    }
    Ok(seal)
}

/// The deterministic F07 phase of one epoch opening. Every live record bound to the
/// `previous` binding rolls over to `next` (closing its segment as `PolicyChanged` or
/// `ModelChanged`); a record whose worker `departed` the F02 table is retired with its segment
/// digest preserved; a frozen worker without a record is bootstrapped unobserved at reset
/// generation 1.
///
/// # Errors
/// Propagates `rollover_segment`'s refusals (`F07_SEGMENT_MISMATCH` for a record not bound to
/// `previous`); returns `F07_BINDING_MISMATCH` when a frozen worker's owner differs from its
/// record or its record is retired; `CAPACITY` for a record beyond `LIMIT`; `NON_CANONICAL`
/// for a height before a record's last transition.
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn open_segments(
    state: &ReputationState,
    previous: SegmentBinding,
    next: SegmentBinding,
    roster: &[WorkerRosterEntry],
    departed: impl Fn(WorkerId) -> bool,
    height: u64,
) -> CodecResult<ReputationState> {
    open_segments_inner::<false>(
        state,
        previous,
        next,
        roster,
        |worker| Ok(departed(worker)),
        height,
    )
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn open_segments_with(
    state: &ReputationState,
    previous: SegmentBinding,
    next: SegmentBinding,
    roster: &[WorkerRosterEntry],
    departed: impl Fn(WorkerId) -> CodecResult<bool>,
    height: u64,
) -> CodecResult<ReputationState> {
    open_segments_inner::<true>(state, previous, next, roster, departed, height)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn open_segments_inner<const COMMITTED: bool>(
    state: &ReputationState,
    previous: SegmentBinding,
    next: SegmentBinding,
    roster: &[WorkerRosterEntry],
    departed: impl Fn(WorkerId) -> CodecResult<bool>,
    height: u64,
) -> CodecResult<ReputationState> {
    let mut opened = state.clone();
    let latest = state.latest_completed();
    let unchanged = COMMITTED && previous == next;
    for slot in opened.current.iter_mut().flatten() {
        if slot.status == HistoryStatus::Retired {
            continue;
        }
        if height == 0 || height < slot.last_transition_height {
            return Err(NON_CANONICAL);
        }
        if departed(slot.worker)? {
            slot.status = HistoryStatus::Retired;
            slot.last_transition_height = height;
            continue;
        }
        if unchanged {
            continue;
        }
        let key = SegmentKey {
            market: state.market,
            worker: slot.worker,
            config: previous.config,
            policy: previous.policy,
            model: previous.model,
            reset_generation: slot.reset_generation,
        };
        if let Presence::Present((_, rolled)) = rollover_segment(
            slot,
            key,
            next.config,
            next.policy,
            next.model,
            latest,
            height,
        )? {
            *slot = rolled;
        }
    }
    bind_opened_roster(&mut opened, next, roster, height, COMMITTED)?;
    Ok(opened)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn bind_opened_roster(
    opened: &mut ReputationState,
    next: SegmentBinding,
    roster: &[WorkerRosterEntry],
    height: u64,
    committed: bool,
) -> CodecResult<()> {
    let market = opened.market;
    {
        let mut editor = if committed {
            opened.edit_produced()?
        } else {
            opened.edit()?
        };
        let mut cursor = 0;
        for entry in roster {
            let existing = editor.ordered_record(entry.worker, &mut cursor);
            match existing {
                Some(r) if r.owner != entry.owner || r.status == HistoryStatus::Retired => {
                    return Err(F07_BINDING_MISMATCH);
                }
                Some(_) => (),
                None => {
                    let key = SegmentKey {
                        market,
                        worker: entry.worker,
                        config: next.config,
                        policy: next.policy,
                        model: next.model,
                        reset_generation: Version::new(1)?,
                    };
                    editor.insert_ordered(
                        ReputationCurrent::bootstrap(key, entry.owner, height)?,
                        &mut cursor,
                    )?;
                }
            }
        }
    }
    if !committed {
        opened.validate()?;
    }
    opened.encoded_size()?;
    Ok(())
}

/// The record of `worker` and its segment key under the current `binding`.
///
/// # Errors
/// Returns `F07_UNKNOWN_WORKER` without a record; propagates `ReputationCurrent::matches_segment`.
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn bound_record(
    state: &ReputationState,
    worker: WorkerId,
    binding: SegmentBinding,
) -> CodecResult<(ReputationCurrent, SegmentKey)> {
    let record = *state
        .records()
        .find(|r| r.worker == worker)
        .ok_or(F07_UNKNOWN_WORKER)?;
    let key = SegmentKey {
        market: state.market,
        worker,
        config: binding.config,
        policy: binding.policy,
        model: binding.model,
        reset_generation: record.reset_generation,
    };
    record.matches_segment(key)?;
    Ok((record, key))
}

/// The expected-context check of an administrative request: the reset generation first, so a
/// previous generation's replay is a `F07_GENERATION_MISMATCH`, then the segment digest.
///
/// # Errors
/// Returns `F07_GENERATION_MISMATCH` or `F07_SEGMENT_MISMATCH`.
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
pub fn check_expected(
    record: &ReputationCurrent,
    segment: Digest32,
    generation: u64,
) -> CodecResult<()> {
    if record.reset_generation.get() != generation {
        return Err(F07_GENERATION_MISMATCH);
    }
    if record.segment != segment {
        return Err(F07_SEGMENT_MISMATCH);
    }
    Ok(())
}

/// `record` with history status `status` from `height`; scores and counters are retained.
///
/// # Errors
/// Propagates `ReputationCurrent::matches_segment`; returns `CONFLICT` for a retired record, a
/// retirement request or an unchanged status; `NON_CANONICAL` for a height before the last
/// transition.
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn set_status(
    record: &ReputationCurrent,
    key: SegmentKey,
    status: HistoryStatus,
    height: u64,
) -> CodecResult<ReputationCurrent> {
    record.matches_segment(key)?;
    if record.status == HistoryStatus::Retired
        || status == HistoryStatus::Retired
        || record.status == status
    {
        return Err(CONFLICT);
    }
    if height == 0 || height < record.last_transition_height {
        return Err(NON_CANONICAL);
    }
    let mut next = *record;
    next.status = status;
    next.last_transition_height = height;
    next.validate()?;
    Ok(next)
}

/// `state` with the record of `record.worker` replaced.
///
/// # Errors
/// Returns `F07_UNKNOWN_WORKER` without a record; propagates `ReputationState::validate`.
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
pub fn replace_record(
    state: &ReputationState,
    record: ReputationCurrent,
) -> CodecResult<ReputationState> {
    let mut next = state.clone();
    let slot = next
        .current
        .iter_mut()
        .flatten()
        .find(|r| r.worker == record.worker)
        .ok_or(F07_UNKNOWN_WORKER)?;
    *slot = record;
    next.validate()?;
    Ok(next)
}

/// Removes, oldest first, every completed summary whose epoch the common F06 ring no longer
/// `retained`; a summary F06 still retains (and every later one) is kept.
///
/// # Errors
/// Propagates `retained`'s and `ReputationState::prune_oldest`'s refusals.
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
pub fn prune_released(
    state: &ReputationState,
    retained: impl Fn(u64) -> CodecResult<bool>,
) -> CodecResult<ReputationState> {
    let mut next = state.clone();
    loop {
        let Some(first) = next.completed().next().copied() else {
            break;
        };
        if retained(first.epoch)? {
            break;
        }
        next.prune_oldest(first.epoch)?;
    }
    Ok(next)
}

/// One frozen worker's sealed F05 terminal output.
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct WorkerObservation {
    pub median: Presence<Score>,
    pub support: u8,
}

/// `ApplyCompletedEpoch`: one update per frozen worker of `frozen.epoch` from its sealed F05
/// output under the sealed `history_allowed` bit and eligible count, then one completed
/// summary bound to `result`. `binding` is the frozen segment binding of the epoch.
///
/// # Errors
/// Returns `F07_EPOCH_NOT_SEALED` without the epoch's seal; `F07_BINDING_MISMATCH` when the
/// observations or binding disagree with the frozen roster; `NON_CANONICAL` when the epoch is
/// already retained (F05 cannot complete it twice); propagates `CompletionDraft`'s refusals
/// (`RETENTION_FULL`, `WRONG_EPOCH`, `CONFLICT`, `F07_UNKNOWN_WORKER`, ...).
#[allow(
    dead_code,
    reason = "joint-section runtime used by epoch.rs; tests/reputation_vectors.rs includes this file via #[path]"
)]
#[allow(
    clippy::too_many_arguments,
    reason = "every input is a distinct authenticated producer value of the completion"
)]
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn complete_epoch(
    state: &ReputationState,
    seal: Presence<HistorySeal>,
    frozen: FrozenBinding,
    binding: SegmentBinding,
    result: Digest32,
    height: u64,
    roster: &[WorkerRosterEntry],
    observations: &[WorkerObservation],
) -> CodecResult<(ReputationState, CompletedHistory)> {
    complete_epoch_inner(
        state,
        seal,
        frozen,
        binding,
        result,
        height,
        roster,
        observations,
        false,
    )
}

pub(crate) fn complete_committed_epoch(
    state: &ReputationState,
    seal: Presence<HistorySeal>,
    frozen: FrozenBinding,
    binding: SegmentBinding,
    result: Digest32,
    height: u64,
    roster: &[WorkerRosterEntry],
    observations: &[WorkerObservation],
) -> CodecResult<(ReputationState, CompletedHistory)> {
    complete_epoch_inner(
        state,
        seal,
        frozen,
        binding,
        result,
        height,
        roster,
        observations,
        true,
    )
}

fn complete_epoch_inner(
    state: &ReputationState,
    seal: Presence<HistorySeal>,
    frozen: FrozenBinding,
    binding: SegmentBinding,
    result: Digest32,
    height: u64,
    roster: &[WorkerRosterEntry],
    observations: &[WorkerObservation],
    committed: bool,
) -> CodecResult<(ReputationState, CompletedHistory)> {
    let seal = match seal {
        Presence::Present(seal) if seal.epoch == frozen.epoch => seal,
        _ => return Err(F07_EPOCH_NOT_SEALED),
    };
    if observations.len() != roster.len() || binding.config != frozen.config {
        return Err(F07_BINDING_MISMATCH);
    }
    let start = CompletionDraft::begin_inner(state, frozen, result, height, roster, committed)?;
    let mut draft = match start {
        CompletionStart::Pending(draft) => draft,
        CompletionStart::Retained(_) => return Err(NON_CANONICAL),
    };
    let mut record_cursor = 0;
    for (i, (entry, observation)) in roster.iter().zip(observations).enumerate() {
        let at = state.ordered_position(entry.worker, &mut record_cursor);
        let record = state.current.guest_index(at).ok_or(F07_UNKNOWN_WORKER)?;
        let key = SegmentKey {
            market: state.market,
            worker: entry.worker,
            config: binding.config,
            policy: binding.policy,
            model: binding.model,
            reset_generation: record.reset_generation,
        };
        // apply_worker checks this exact key against the same record before mutation.
        draft.apply_worker(
            key,
            observation.median,
            observation.support,
            seal.eligible,
            seal.allows(i),
        )?;
    }
    draft.finish()
}
