//! F01 `OpenEpoch` composer and `ADVANCE_ACTIVATION` over the complete committed shared
//! state value.
//!
//! `OpenEpoch` is lazy: it opens only the clock epoch of the authenticated execution height,
//! only inside that epoch's Work window, and never backdates. Participant work advances in
//! bounded chunks, privately journaled in F01 until the last call. Every window bound, including
//! `T + 128`, comes from the checked [`market_clock`] arithmetic; an unrepresentable epoch
//! refuses `ARITHMETIC` before anything is composed. One successful opening atomically:
//! selects the sole pending policy when `epoch >= effective_epoch` (F01-R028), runs the F08
//! roster rollover (which marks missed epochs skipped and requires the previous opened epoch
//! to be terminal), runs the F03 evaluator snapshot under the selected policy rubric, enforces
//! anti-self-dealing and minimum readiness, freezes the core roster hash, reserves the F06
//! epoch budget, and commits exactly one revision increment with `header.state_revision`
//! equal to the shared revision. Every refusal leaves the committed input untouched; `next`,
//! `scratch` and `event` must then be discarded.
//!
//! The frozen snapshot of an opened epoch lives in its producers: F08 holds epoch presence
//! (`current_epoch`), membership and liveness baselines; F03 holds the frozen evaluator
//! grant/key versions and full immutable worker entries; F06 holds the budget, the core roster digest and the frozen recipient
//! slots; F01 holds the frozen config and policy, which only the next opening can replace.
//!
//! Producer gaps and the readings chosen here:
//! - F02 stores no payout recipient. The worker's frozen recipient is the native account of
//!   its owner principal, which is how the runtime names a principal's account
//!   (`visible_principal_account`). F06 still refuses a recipient equal to the reserve account.
//! - F02 and F03 declare no control identity beyond the owner principal, so the owner
//!   principal is the declared controlling identity. An evaluator owned by the market owner, by
//!   a worker owner, or signing with a worker delegate key is a `ROLE_CONFLICT`.
//! - The joint F05/F06 section starts with the F06 reward state (`REWARD_STATE_BYTES`); any
//!   F05 bytes after it are carried unchanged. Without an F05 terminal record, "previous epoch
//!   terminal" is the F06 ledger having no active reserve.
//! - The epoch budget is `min(epoch_budget_cap, Free)` and must reach
//!   `minimum_epoch_funding`; a market whose reward state was never initialized is not ready.
//! - `OPEN_EPOCH` has no specified payload; it must be empty. The envelope binds the epoch
//!   being opened, its frozen config and the roster digest the opening freezes (an absent
//!   roster is only representable for epoch 0).
//! - The F01 task region is reset through `tasks::region_after_open`: a sealed or empty set
//!   yields the empty region, a nonempty unsealed set refuses the opening with `WRONG_PHASE`.
//! - The joint F07/F08 section is `RP07 region || [HA07 history seal] || F08 table`
//!   (`reputation_transition::split_joint`). A market without the F07 region keeps the F08
//!   table alone and every F07 phase is skipped. With the region, the opening atomically rolls
//!   every live reputation segment over to the selected config/policy/model, retires the records
//!   of workers that left the F02 table, bootstraps frozen workers unobserved and releases the
//!   completed summaries F06 no longer retains; [`aggregate`] freezes `history_allowed` at the
//!   F05 seal and applies the completed epoch inside the F05 terminal transition; [`history`]
//!   runs `ResetHistory`/`SuspendHistory`/`ResumeHistory`; [`carry_reputation`] keeps the region
//!   byte-identical across every other producer, which reads the F08 table only.
use crate::types::GuestIndex as _;
use crate::{
    MAX_EVALUATORS, MAX_STATE_BYTES, MAX_WORKERS,
    admission::{AdmissionMeta, AdmissionTable, Participant},
    aggregation::{self, Outcome as Aggregated},
    codec::{
        self, Envelope, EventCommon, Reader, Roster, ValidatedEnvelope, Writer, derive_market,
    },
    dispatch,
    errors::{
        ARITHMETIC, CAPACITY, CONFLICT, CodecResult, F01_ACTIVATION_NOT_READY,
        F01_ACTIVATION_TOO_EARLY, F01_ALREADY_ACTIVATED, F01_LIFECYCLE_CLOSED, F01_STALE_REVISION,
        F07_BINDING_MISMATCH, F07_IDEMPOTENCY_CONFLICT, F07_IDENTITY_FROZEN, F07_UNKNOWN_WORKER,
        F08_MARKET_PAUSED, INSUFFICIENT_FREE, NON_CANONICAL, NOT_FOUND, READINESS_BLOCKED,
        REPLAY_CONFLICT, ROLE_CONFLICT, UNAUTHORIZED, UNKNOWN_OPERATION, WRONG_CONFIG, WRONG_EPOCH,
        WRONG_MARKET, WRONG_PHASE, WRONG_ROSTER,
    },
    evaluators::{
        authority::{self, SnapshotContext, split_identity_section},
        model::GrantStatus,
    },
    policy::TaskPolicyV1,
    registry::{MarketHeader, check_f01_capacity, market_clock},
    registry_ops::{
        ACTIVE, CallContext, PolicySection, REGISTERED, SUSPENDED, activate_pending_policy,
    },
    reputation::{ClosureReason, HistoryStatus, ReputationCurrent, SegmentKey},
    reputation_transition::{
        Region, SegmentBinding, WorkerObservation, bound_record, check_expected, encode_joint,
        replace_record, reset_segment, set_status, split_joint,
    },
    rewards::{
        EpochStatus, REWARD_STATE_BYTES, RewardEpoch, RewardOutcome, RewardState,
        decode_reward_state,
    },
    roster::{self, ROLLOVER_SCRATCH_BYTES},
    state::{
        ActorSlot, ReplayDecision, ReplayRequest, RetainedResult, Section, SharedState,
        decode_shared_state, encode_shared_state,
    },
    tasks,
    types::{
        AccountId, Amount, Digest32, EpochPhase, EvaluatorRosterEntry, FrozenBinding, PolicyDigest,
        Presence, PrincipalId, ResultDigest, RosterDigest, Version, WorkerId, WorkerRosterEntry,
    },
    workers::{WorkerState, WorkerTable},
};

const POLICY_CAP: usize = Section::PolicyLifecycle.payload_cap();
const IDENTITY_CAP: usize = Section::IdentityRoster.payload_cap();
const SETTLEMENT_CAP: usize = Section::SettlementClaims.payload_cap();
const CONTROL_CAP: usize = Section::Control.payload_cap();
const JOINT_OUT: usize = Section::ReputationAdmission.payload_cap();
const ADMISSION: usize = Section::ReputationAdmission.index();

/// `EpochOpened` suffix: policy digest, roster digest, budget, worker and evaluator counts,
/// policy-activated flag, previous-epoch presence and value, skipped epochs, cursor and count.
pub const OPEN_SUFFIX_BYTES: usize = 104;
/// `MarketActivated` suffix: activation epoch, clock epoch and committed height.
pub const ACTIVATION_SUFFIX_BYTES: usize = 24;
/// Caller scratch for [`open_epoch`]: the rolled-over state, the rollover scratch, the next
/// identity, settlement, policy and control payloads, the state without its F07 region and the
/// next joint F07/F08 section.
pub const OPEN_SCRATCH_BYTES: usize = MAX_STATE_BYTES
    + ROLLOVER_SCRATCH_BYTES
    + IDENTITY_CAP
    + SETTLEMENT_CAP
    + POLICY_CAP
    + CONTROL_CAP
    + MAX_STATE_BYTES
    + JOINT_OUT;
/// Caller scratch of [`carry_reputation`] beside the wrapped producer's own buffers: the state
/// without its F07 region, the next joint section and a control payload.
pub const CARRY_SCRATCH_BYTES: usize = MAX_STATE_BYTES + JOINT_OUT + CONTROL_CAP;
/// Caller scratch for [`aggregate`]: the F05 scratch followed by the carry scratch.
pub const AGGREGATE_SCRATCH_BYTES: usize = aggregation::SCRATCH_BYTES + CARRY_SCRATCH_BYTES;
/// Caller scratch for [`history`]: the next policy payload, joint section and control payload.
pub const HISTORY_SCRATCH_BYTES: usize = POLICY_CAP + JOINT_OUT + CONTROL_CAP;
/// `HistoryReset` suffix: worker, old segment, new segment, reset generation and reason.
pub const RESET_SUFFIX_BYTES: usize = 105;
/// `HistoryStatus` suffix: worker, segment, reset generation, new status and reason.
pub const STATUS_SUFFIX_BYTES: usize = 74;
/// `ResetHistory`/`SuspendHistory`/`ResumeHistory` payload: worker, expected segment digest,
/// expected reset generation and reason.
pub const HISTORY_PAYLOAD_BYTES: usize = 73;
/// Caller scratch for [`advance_activation`]: the activated policy and control payloads, one
/// trial opening state and the opening scratch.
pub const ADVANCE_SCRATCH_BYTES: usize =
    POLICY_CAP + CONTROL_CAP + MAX_STATE_BYTES + OPEN_SCRATCH_BYTES;

/// The immutable binding one successful opening freezes.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Frozen {
    pub epoch: u64,
    pub previous: Option<u64>,
    pub skipped: u64,
    pub config: Version,
    pub policy: PolicyDigest,
    pub policy_activated: bool,
    pub roster: RosterDigest,
    pub budget: Amount,
    pub workers: u8,
    pub evaluators: u8,
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum Outcome {
    Opened {
        frozen: Frozen,
        cursor: u16,
        revision: u64,
        result: ResultDigest,
        state_len: usize,
        event_len: usize,
    },
    /// The clock epoch is already open under exactly this binding; nothing was written.
    AlreadyApplied { epoch: u64, roster: RosterDigest },
}

/// Private participant work is stored in the section owned by its handler.
/// Other sections remain byte-identical until the last chunk commits.
pub(crate) struct ParticipantJournal {
    pub epoch: u64,
    pub height: u64,
    pub cursor: u16,
    pub count: u16,
    pub observed: u8,
    pub covered: u8,
    pub config: u64,
    pub roster: RosterDigest,
    pub state: Option<crate::reputation::ReputationState>,
}
impl ParticipantJournal {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn decode(bytes: &[u8], records: bool) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        if r.fixed::<4>()? != *b"PC17" {
            return Err(NON_CANONICAL);
        }
        let epoch = r.u64()?;
        let height = r.u64()?;
        let cursors = r.u32()?;
        let cursor = (cursors >> 16) as u16;
        let count = cursors as u16;
        let counts = r.u16()?;
        let observed = (counts >> 8) as u8;
        let covered = counts as u8;
        let config = r.u64()?;
        let roster = RosterDigest::new(r.fixed()?)?;
        if count == 0
            || count > 32
            || cursor == 0
            || cursor >= count
            || cursor % crate::aggregation::participant_chunk_size(usize::from(count)) as u16 != 0
            || observed > cursor as u8
            || covered > cursor as u8
            || height == 0
        {
            return Err(NON_CANONICAL);
        }
        let remaining = r.take(r.remaining())?;
        let state = if records && !remaining.is_empty() {
            Some(crate::reputation_codec::decode_section_records(
                remaining, true,
            )?)
        } else {
            None
        };
        Ok(Self {
            epoch,
            height,
            cursor,
            count,
            observed,
            covered,
            config,
            roster,
            state,
        })
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        let mut w = Writer::new(out);
        crate::codec::write_fields!(w;
            put: b"PC17";
            u64: self.epoch;
            u64: self.height;
            u32: (u32::from(self.cursor) << 16) | u32::from(self.count);
            put: &[self.observed, self.covered];
            u64: self.config;
            id32: self.roster.as_bytes();
        )?;
        let n = w.len();
        match &self.state {
            Some(state) => Ok(n + crate::reputation_codec::encode_produced_section(
                state,
                out.get_mut(n..).ok_or(CAPACITY)?,
            )?),
            None => Ok(n),
        }
    }
}

pub(crate) fn opening_pending(policy: &PolicySection<'_>) -> bool {
    policy.task_region.starts_with(b"PC17")
}

/// Complete immutable roster recovered from the committed opening producers.
pub struct CapturedRoster {
    pub binding: FrozenBinding,
    workers: [WorkerRosterEntry; MAX_WORKERS],
    worker_count: usize,
    evaluators: [EvaluatorRosterEntry; MAX_EVALUATORS],
    evaluator_count: usize,
}
impl CapturedRoster {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn check_reservation(
        &self,
        row: &RewardEpoch,
        dictionary: crate::rewards::RecipientDictionary<'_>,
    ) -> CodecResult<()> {
        match (row.status, row.outcome) {
            (EpochStatus::Terminal, RewardOutcome::NoEligibleScore) => row.validate()?,
            _ if row.entries().len() != self.worker_count => return Err(WRONG_ROSTER),
            _ => {}
        }
        for entry in row.entries() {
            let slot = dictionary.slot(entry.slot)?;
            if codec::frozen_worker_index(self.workers(), slot.worker)
                .is_none_or(|index| self.workers().guest_index(index).recipient != slot.recipient)
            {
                return Err(WRONG_ROSTER);
            }
        }
        Ok(())
    }
    pub fn workers(&self) -> &[WorkerRosterEntry] {
        self.workers.guest_index(..self.worker_count)
    }
    pub fn evaluators(&self) -> &[EvaluatorRosterEntry] {
        self.evaluators.guest_index(..self.evaluator_count)
    }
    pub fn view(&self) -> Roster<'_> {
        Roster {
            market: self.binding.market,
            epoch: self.binding.epoch,
            config: self.binding.config,
            workers: self.workers(),
            evaluators: self.evaluators(),
        }
    }
}

/// Reads F08 epoch presence and cross-checks the F03 immutable snapshot against F06.
/// Missing or disagreeing opening producers refuse instead of inventing a live roster.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn captured_roster(state: &SharedState<'_>) -> CodecResult<Option<CapturedRoster>> {
    if opening_pending(&PolicySection::decode(
        state.section(Section::PolicyLifecycle)?,
    )?) {
        return Ok(None);
    }
    let (_, table) = split_joint(*state.feature_sections.guest_index(ADMISSION))?;
    let admission = AdmissionTable::decode(table)?;
    let region = authority::evaluator_region(
        *state
            .feature_sections
            .guest_index(Section::IdentityRoster.index()),
    )?;
    let Some(epoch) = admission.current_epoch() else {
        if region.snapshot().is_some() {
            return Err(WRONG_ROSTER);
        }
        let settlement = state.room_market()?.0;
        if !settlement.is_empty() {
            let rewards =
                decode_reward_state(settlement.get(..REWARD_STATE_BYTES).ok_or(NON_CANONICAL)?)?;
            if rewards.ledger()?.retained_epochs != 0 {
                return Err(WRONG_ROSTER);
            }
        }
        return Ok(None);
    };
    // Keep the public reader's snapshot-presence refusal order.
    let snapshot = region
        .snapshot()
        .filter(|s| s.epoch == epoch)
        .ok_or(WRONG_ROSTER)?;
    snapshot.workers().next().ok_or(WRONG_ROSTER)?;
    snapshot.entries().next().ok_or(WRONG_ROSTER)?;
    let policy = PolicySection::decode(
        *state
            .feature_sections
            .guest_index(Section::PolicyLifecycle.index()),
    )?;
    let settlement = state.room_market()?.0;
    let rewards = decode_reward_state(settlement.get(..REWARD_STATE_BYTES).ok_or(WRONG_ROSTER)?)?;
    Ok(Some(captured_roster_with(
        &admission, &policy, &region, rewards,
    )?))
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn committed_roster(state: &SharedState<'_>) -> CodecResult<CapturedRoster> {
    committed_roster_sections(
        state.section(Section::PolicyLifecycle)?,
        state.section(Section::IdentityRoster)?,
        state.section(Section::ReputationAdmission)?,
        state.room_market()?.0,
    )
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn committed_roster_sections(
    policy: &[u8],
    identity: &[u8],
    admission: &[u8],
    settlement: &[u8],
) -> CodecResult<CapturedRoster> {
    let settlement = crate::state::room_market_parts(settlement)?.0;
    if opening_pending(&PolicySection::decode(policy)?) {
        return Err(WRONG_EPOCH);
    }
    let epoch = AdmissionTable::committed_epoch(
        crate::reputation_transition::framed_admission_table(admission)?,
    )?
    .ok_or(WRONG_ROSTER)?;
    let policy = PolicySection::decode(policy)?;
    let snapshot = authority::Snapshot::committed(identity)?;
    if snapshot.epoch != epoch {
        return Err(WRONG_ROSTER);
    }
    let rewards = crate::rewards::committed_row(settlement, epoch)?;
    capture_snapshot(&policy, &snapshot, rewards.row, rewards.dictionary)
}

/// The same immutable-roster checks using sections already validated by the caller.
pub(crate) fn captured_roster_with(
    admission: &AdmissionTable,
    policy: &PolicySection<'_>,
    region: &authority::EvaluatorRegion,
    rewards: RewardState<'_>,
) -> CodecResult<CapturedRoster> {
    let epoch = admission.current_epoch().ok_or(WRONG_ROSTER)?;
    captured_roster_row(
        admission,
        policy,
        region,
        rewards.row(epoch)?,
        rewards.dictionary(),
    )
}

pub(crate) fn captured_roster_row(
    admission: &AdmissionTable,
    policy: &PolicySection<'_>,
    region: &authority::EvaluatorRegion,
    row: RewardEpoch,
    dictionary: crate::rewards::RecipientDictionary<'_>,
) -> CodecResult<CapturedRoster> {
    let roster = committed_roster_row(admission, policy, region, row, dictionary)?;
    if codec::roster_digest(&roster.view())? != roster.binding.roster {
        return Err(WRONG_ROSTER);
    }
    Ok(roster)
}

// Opening is the sole producer of this immutable snapshot and its F06 digest.
// Routed consumers check the binding and selected recipient slots, without
// serializing and hashing that same snapshot again on every chunk.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn committed_roster_row(
    admission: &AdmissionTable,
    policy: &PolicySection<'_>,
    region: &authority::EvaluatorRegion,
    row: RewardEpoch,
    dictionary: crate::rewards::RecipientDictionary<'_>,
) -> CodecResult<CapturedRoster> {
    let epoch = admission.current_epoch().ok_or(WRONG_ROSTER)?;
    if row.epoch != epoch {
        return Err(WRONG_ROSTER);
    }
    let snapshot = region
        .snapshot()
        .filter(|s| s.epoch == epoch)
        .ok_or(WRONG_ROSTER)?;
    capture_snapshot(policy, snapshot, row, dictionary)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn capture_snapshot(
    policy: &PolicySection<'_>,
    snapshot: &authority::Snapshot,
    row: RewardEpoch,
    dictionary: crate::rewards::RecipientDictionary<'_>,
) -> CodecResult<CapturedRoster> {
    let epoch = snapshot.epoch;
    if row.epoch != epoch {
        return Err(WRONG_ROSTER);
    }
    let worker_count = snapshot.worker_count();
    let mut workers = snapshot.workers();
    let mut evaluators = snapshot.entries();
    let first_worker = *workers.next().ok_or(WRONG_ROSTER)?;
    let first_evaluator = evaluators.next().ok_or(WRONG_ROSTER)?.entry;
    let mut worker_entries = [first_worker; MAX_WORKERS];
    let mut evaluator_entries = [first_evaluator; MAX_EVALUATORS];
    for (slot, worker) in worker_entries.iter_mut().skip(1).zip(workers) {
        *slot = *worker;
    }
    for (slot, evaluator) in evaluator_entries.iter_mut().skip(1).zip(evaluators) {
        *slot = evaluator.entry;
    }
    let binding = FrozenBinding {
        chain: policy.header.deployment_chain_domain,
        program: policy.header.program_id,
        market: policy.header.market_id,
        epoch,
        config: Version::new(policy.header.active_config_version)?,
        roster: row.roster,
    };
    let roster = CapturedRoster {
        binding,
        workers: worker_entries,
        worker_count,
        evaluators: evaluator_entries,
        evaluator_count: snapshot.len(),
    };
    if roster
        .workers()
        .windows(2)
        .any(|w| w[0].worker >= w[1].worker)
        || roster
            .evaluators()
            .windows(2)
            .any(|e| e[0].evaluator >= e[1].evaluator)
    {
        return Err(WRONG_ROSTER);
    }
    roster.check_reservation(&row, dictionary)?;
    Ok(roster)
}

/// Prerequisites an opening of `epoch` satisfies (or the opened epoch already satisfies).
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Readiness {
    pub epoch: u64,
    pub workers: usize,
    pub evaluators: usize,
    pub budget: Amount,
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Activated {
    pub readiness: Readiness,
    pub activation_epoch: u64,
    pub revision: u64,
    pub result: ResultDigest,
    pub state_len: usize,
    pub event_len: usize,
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn committed(current: &[u8]) -> CodecResult<(SharedState<'_>, PolicySection<'_>)> {
    let state = decode_shared_state(current)?;
    let section = PolicySection::decode(
        *state
            .feature_sections
            .guest_index(Section::PolicyLifecycle.index()),
    )?;
    if section.header.state_revision != state.revision {
        return Err(NON_CANONICAL);
    }
    Ok((state, section))
}

/// Clock epoch of `height` inside its Work window. `market_clock` computes every window
/// bound of that epoch with checked arithmetic, so an epoch whose `T + 128` is not
/// representable refuses `ARITHMETIC`.
fn work_epoch(header: &MarketHeader, height: u64) -> CodecResult<u64> {
    if height < header.origin_height {
        return Err(WRONG_EPOCH);
    }
    let clock = market_clock(header.origin_height, height)?;
    if clock.phase != EpochPhase::Work {
        return Err(WRONG_PHASE);
    }
    Ok(clock.epoch)
}

fn opening_epoch(header: &MarketHeader, height: u64) -> CodecResult<u64> {
    match header.lifecycle {
        REGISTERED | ACTIVE => work_epoch(header, height),
        SUSPENDED => Err(F08_MARKET_PAUSED),
        _ => Err(F01_LIFECYCLE_CLOSED),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn authenticate(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    header: &MarketHeader,
) -> CodecResult<()> {
    let e = &envelope.envelope;
    codec::compare_native_principal(e, ctx.principal)?;
    let market = derive_market(ctx.chain, ctx.program)?;
    e.check_domain(ctx.chain, ctx.program, market)?;
    e.check_expiry(ctx.height)?;
    if header.market_id != market {
        return Err(WRONG_MARKET);
    }
    Ok(())
}

/// F01-R028 selection: the sole pending policy once `epoch` reaches its effective epoch.
fn select_policy<'a>(
    section: &PolicySection<'a>,
    epoch: u64,
) -> CodecResult<(PolicySection<'a>, bool)> {
    match section.pending {
        Presence::Present(pending) if epoch >= pending.effective_epoch => {
            Ok((activate_pending_policy(section, epoch)?, true))
        }
        _ => Ok((*section, false)),
    }
}

/// Frozen core worker entry of an F08 member eligible at this opening.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn worker_entry(
    m: &AdmissionMeta,
    workers: &crate::workers::OpeningCursor<'_>,
    height: u64,
) -> CodecResult<Option<WorkerRosterEntry>> {
    let Participant::Worker(worker) = m.participant else {
        return Ok(None);
    };
    if !m.admitted() || m.admitted_epoch.is_none() || m.revoked() || m.draining() {
        return Ok(None);
    }
    let record = workers.get(worker)?.ok_or(NON_CANONICAL)?;
    if !matches!(record.state, WorkerState::Enrolled | WorkerState::Available)
        || height >= record.expiry
    {
        return Ok(None);
    }
    if record.owner != m.owner || record.key_version != m.delegate_generation {
        return Err(NON_CANONICAL);
    }
    Ok(Some(WorkerRosterEntry {
        worker,
        owner: record.owner,
        recipient: AccountId::new(record.owner.bytes())?,
        generation: Version::new(record.generation)?,
        key_version: Version::new(record.key_version)?,
        public_key: record.delegate,
        metadata: record.metadata,
    }))
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn worker_roster(
    admission: &AdmissionTable,
    workers: &crate::workers::OpeningWorkers<'_>,
    height: u64,
) -> CodecResult<([WorkerRosterEntry; MAX_WORKERS], usize)> {
    let workers = workers.ordered();
    let mut members = admission.iter();
    let first = loop {
        let member = members.next().ok_or(READINESS_BLOCKED)?;
        if let Some(entry) = worker_entry(member, &workers, height)? {
            break entry;
        }
    };
    let mut roster = [first; MAX_WORKERS];
    let mut count = 1;
    for member in members {
        if let Some(entry) = worker_entry(member, &workers, height)? {
            *roster.get_mut(count).ok_or(CAPACITY)? = entry;
            count += 1;
        }
    }
    // AdmissionTable's canonical worker ordering is retained by this projection.
    Ok((roster, count))
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn evaluator_roster(
    identity: &[u8],
    epoch: u64,
) -> CodecResult<([EvaluatorRosterEntry; MAX_EVALUATORS], usize)> {
    let region = authority::evaluator_region(identity)?;
    evaluator_roster_snapshot(&region, epoch)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn evaluator_roster_snapshot(
    region: &authority::EvaluatorRegion,
    epoch: u64,
) -> CodecResult<([EvaluatorRosterEntry; MAX_EVALUATORS], usize)> {
    let snapshot = region
        .snapshot()
        .filter(|s| s.epoch == epoch)
        .ok_or(NON_CANONICAL)?;
    let first = snapshot.entries().next().ok_or(READINESS_BLOCKED)?.entry;
    let mut roster = [first; MAX_EVALUATORS];
    for (slot, frozen) in roster.iter_mut().zip(snapshot.entries()) {
        *slot = frozen.entry;
    }
    Ok((roster, snapshot.len()))
}

/// F01-R015 at epoch opening over declared principals and keys.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn check_independence(
    market_owner: PrincipalId,
    workers: &[WorkerRosterEntry],
    evaluators: &[EvaluatorRosterEntry],
) -> CodecResult<()> {
    for e in evaluators {
        if e.owner == market_owner
            || workers
                .iter()
                .any(|w| w.owner == e.owner || w.public_key == e.public_key)
        {
            return Err(ROLE_CONFLICT);
        }
    }
    Ok(())
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn check_readiness(policy: &TaskPolicyV1, workers: usize, evaluators: usize) -> CodecResult<()> {
    let workers_ready = (usize::from(policy.minimum_worker_count)
        ..=usize::from(policy.max_workers))
        .contains(&workers);
    let evaluators_ready = (usize::from(policy.minimum_evaluator_count)
        ..=usize::from(policy.max_evaluators))
        .contains(&evaluators);
    if workers_ready && evaluators_ready {
        Ok(())
    } else {
        Err(READINESS_BLOCKED)
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn epoch_budget(policy: &TaskPolicyV1, free: Amount) -> CodecResult<Amount> {
    let budget = policy.epoch_budget_cap.min(free);
    if budget < policy.minimum_epoch_funding {
        Err(INSUFFICIENT_FREE)
    } else {
        Ok(budget)
    }
}

/// F06 `ReserveEpoch` of `rewards`, the validated reward state at the head of `section`, into
/// `out`, carrying any F05 bytes after the reward state unchanged; returns the budget, the
/// section length and the validated reserved state.
#[allow(clippy::too_many_arguments)]
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn reserve<'o>(
    section: &[u8],
    rewards: Option<RewardState<'_>>,
    policy: &TaskPolicyV1,
    epoch: u64,
    roster: RosterDigest,
    workers: &[WorkerRosterEntry],
    height: u64,
    out: &'o mut [u8],
) -> CodecResult<(Amount, usize, RewardState<'o>)> {
    let rewards = rewards.ok_or(READINESS_BLOCKED)?;
    let tail = section.get(REWARD_STATE_BYTES..).ok_or(NON_CANONICAL)?;
    let budget = epoch_budget(policy, rewards.ledger()?.free)?;
    let (head, rest) = out
        .split_at_mut_checked(REWARD_STATE_BYTES)
        .ok_or(CAPACITY)?;
    let (reserved, _) = rewards.reserve_epoch(epoch, budget, roster, workers, height, head)?;
    rest.get_mut(..tail.len())
        .ok_or(CAPACITY)?
        .guest_copy_from_slice(tail);
    Ok((budget, section.len(), reserved))
}

/// The frozen worker and evaluator rosters of one opening and its core roster digest.
struct FrozenRoster {
    workers: [WorkerRosterEntry; MAX_WORKERS],
    worker_count: usize,
    evaluator_count: usize,
    identity_len: usize,
    digest: RosterDigest,
}
impl FrozenRoster {
    fn workers(&self) -> &[WorkerRosterEntry] {
        self.workers.guest_index(..self.worker_count)
    }
}

/// Freezes the rosters over the rolled-over sections: F03 snapshot into `identity_out`,
/// independence, readiness and the core roster hash.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn freeze(
    sections: &[&[u8]; 5],
    selected: &PolicySection<'_>,
    admission: &AdmissionTable,
    workers: &crate::workers::OpeningWorkers<'_>,
    epoch: u64,
    height: u64,
    identity_out: &mut [u8],
) -> CodecResult<FrozenRoster> {
    let identity = *sections.guest_index(Section::IdentityRoster.index());
    let (worker_roster, worker_count) = worker_roster(admission, workers, height)?;
    let frozen_workers = worker_roster.guest_index(..worker_count);
    let (snapshot, opened_region) = authority::open_epoch_prepared(
        identity,
        &SnapshotContext {
            market: &selected.header,
            epoch,
            approved_rubric: selected.current.commitments.rubric,
            workers: frozen_workers,
            admission: &admission,
        },
        identity_out,
    )?;
    let (evaluators, evaluator_count) = evaluator_roster_snapshot(&opened_region, epoch)?;
    let frozen_evaluators = evaluators.guest_index(..evaluator_count);
    check_independence(
        selected.header.owner_principal,
        frozen_workers,
        frozen_evaluators,
    )?;
    check_readiness(&selected.current, worker_count, evaluator_count)?;
    let digest =
        authority::CommittedSnapshot::read(identity_out.guest_index(..snapshot.section_len))?
            .digest(
                selected.header.market_id,
                Version::new(selected.header.active_config_version)?,
            )?;
    Ok(FrozenRoster {
        workers: worker_roster,
        worker_count,
        evaluator_count,
        identity_len: snapshot.section_len,
        digest,
    })
}

/// The segment binding of `section`'s active config and policy.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn segment_binding(section: &PolicySection<'_>) -> CodecResult<SegmentBinding> {
    Ok(SegmentBinding {
        config: Version::new(section.header.active_config_version)?,
        policy: Digest32::new(section.current.digest()?.bytes())?,
        model: section.current.commitments.model_artifact,
    })
}

struct Composed {
    frozen: Frozen,
    revision: u64,
    state_len: usize,
    cursor: u16,
    participant_count: u16,
}

/// The whole opening of the clock epoch of `height` over `current`, written into `next`.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn compose(
    current: &[u8],
    height: u64,
    next: &mut [u8],
    scratch: &mut [u8],
) -> CodecResult<Composed> {
    let (state, section) = committed(current)?;
    let (_, table) = split_joint(*state.feature_sections.guest_index(ADMISSION))?;
    compose_sections(
        &crate::state::SectionReader::new(current)?,
        &section,
        AdmissionTable::decode(table)?,
        roster::reward_state,
        false,
        height,
        next,
        scratch,
    )
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn compose_sections(
    reader: &crate::state::SectionReader<'_>,
    section: &PolicySection<'_>,
    admission: AdmissionTable,
    read_rewards: for<'a> fn(&'a [u8], &MarketHeader) -> CodecResult<Option<RewardState<'a>>>,
    chunked: bool,
    height: u64,
    next: &mut [u8],
    scratch: &mut [u8],
) -> CodecResult<Composed> {
    let current_revision = reader.revision();
    let epoch = opening_epoch(&section.header, height)?;
    let pending = if opening_pending(section) {
        Some((
            ParticipantJournal::decode(section.task_region, false)?,
            section.task_region.get(66..).ok_or(NON_CANONICAL)?,
        ))
    } else {
        None
    };
    let height = pending.as_ref().map_or(height, |(p, _)| p.height);
    if pending.as_ref().is_some_and(|(p, _)| p.epoch != epoch) {
        return Err(WRONG_EPOCH);
    }
    let (selected, policy_activated) = select_policy(&section, epoch)?;
    if admission
        .current_epoch()
        .is_some_and(|previous| epoch <= previous)
    {
        return Err(WRONG_EPOCH);
    }
    let rewards = read_rewards(
        crate::state::room_market_parts(reader.section(Section::SettlementClaims))?.0,
        &section.header,
    )?;
    let (journal_out, rest) = scratch
        .split_at_mut_checked(MAX_STATE_BYTES)
        .ok_or(CAPACITY)?;
    let (work, rest) = rest
        .split_at_mut_checked(ROLLOVER_SCRATCH_BYTES)
        .ok_or(CAPACITY)?;
    let (identity_out, rest) = rest.split_at_mut_checked(IDENTITY_CAP).ok_or(CAPACITY)?;
    let (settlement_out, rest) = rest.split_at_mut_checked(SETTLEMENT_CAP).ok_or(CAPACITY)?;
    let (policy_out, rest) = rest.split_at_mut_checked(POLICY_CAP).ok_or(CAPACITY)?;
    let (_, rest) = rest.split_at_mut_checked(CONTROL_CAP).ok_or(CAPACITY)?;
    let (_, joint_out) = rest.split_at_mut_checked(MAX_STATE_BYTES).ok_or(CAPACITY)?;
    let (opened, rolled, admission, workers) =
        roster::rollover_roster_framed(reader, &section.header, admission, rewards, height, work)?;
    let settlement = reader.section(Section::SettlementClaims);

    let sections = core::array::from_fn(|i| rolled.sections[i]);
    let frozen = freeze(
        &sections,
        &selected,
        &admission,
        &workers,
        epoch,
        height,
        identity_out,
    )?;
    let joint = reader.section(Section::ReputationAdmission);
    let table = crate::reputation_transition::framed_admission_table(joint)?;
    let original = joint.guest_index(..joint.len() - table.len());
    let count = frozen.worker_count.max(if original.is_empty() {
        0
    } else {
        usize::from(*original.get(6).ok_or(NON_CANONICAL)?)
    });
    if count > MAX_WORKERS {
        return Err(CAPACITY);
    }
    let start = usize::from(pending.as_ref().map_or(0, |(p, _)| p.cursor));
    let end = if chunked {
        (start + crate::aggregation::participant_chunk_size(count)).min(count)
    } else {
        count
    };
    if pending.as_ref().is_some_and(|(p, _)| {
        p.config != selected.header.active_config_version
            || p.roster != frozen.digest
            || usize::from(p.count) != count
    }) {
        return Err(WRONG_ROSTER);
    }
    // Earlier participant records stay encoded. Only this chunk is decoded,
    // validated and replaced; the original roster/count bound remains checked.
    let reputation_len = if original.is_empty() {
        None
    } else {
        let staged = pending.as_ref().map_or(original, |(_, bytes)| *bytes);
        let worker_reader = workers.ordered();
        Some(crate::reputation_transition::open_segments_chunk(
            original,
            staged,
            segment_binding(section)?,
            segment_binding(&selected)?,
            frozen.workers(),
            |worker| {
                Ok(
                    !matches!(worker_reader.get(worker)?, Some(r) if r.state != WorkerState::Retired),
                )
            },
            height,
            start,
            end,
            joint_out,
        )?)
    };
    let binding = Frozen {
        epoch,
        previous: opened.previous,
        skipped: opened.skipped,
        config: Version::new(selected.header.active_config_version)?,
        policy: selected.current.digest()?,
        policy_activated,
        roster: frozen.digest,
        budget: epoch_budget(
            &selected.current,
            rewards.ok_or(READINESS_BLOCKED)?.ledger()?.free,
        )?,
        workers: u8::try_from(frozen.worker_count).map_err(|_| ARITHMETIC)?,
        evaluators: u8::try_from(frozen.evaluator_count).map_err(|_| ARITHMETIC)?,
    };
    let revision = current_revision.checked_add(1).ok_or(ARITHMETIC)?;
    if end < count {
        // Validate the task reset before the first durable partial opening.
        if pending.is_none() {
            tasks::region_after_open(selected.task_region)?;
        }
        let journal = ParticipantJournal {
            epoch,
            height,
            cursor: end as u16,
            count: count as u16,
            observed: 0,
            covered: 0,
            config: selected.header.active_config_version,
            roster: frozen.digest,
            state: None,
        };
        let mut journal_len = journal.encode(journal_out)?;
        if let Some(len) = reputation_len {
            journal_out
                .get_mut(journal_len..journal_len + len)
                .ok_or(CAPACITY)?
                .guest_copy_from_slice(joint_out.guest_index(..len));
            journal_len += len;
        }
        let mut policy = *section;
        policy.header.state_revision = revision;
        policy.task_region = &journal_out[..journal_len];
        let policy_len = policy.encode(policy_out)?;
        let mut frame = reader.frame();
        frame.revision = revision;
        frame.sections[Section::PolicyLifecycle.index()] = &policy_out[..policy_len];
        let state_len = codec::encode_state(&frame, next)?;
        return Ok(Composed {
            frozen: binding,
            revision,
            state_len,
            cursor: end as u16,
            participant_count: count as u16,
        });
    }
    let (_, settlement_len, reserved) = reserve(
        settlement,
        rewards,
        &selected.current,
        epoch,
        frozen.digest,
        frozen.workers(),
        height,
        settlement_out,
    )?;
    let admission_next = match reputation_len {
        None => *sections.guest_index(ADMISSION),
        Some(len) => {
            let len =
                crate::reputation_transition::prune_opened_history(joint_out, len, |epoch| {
                    match reserved.row(epoch) {
                        Ok(_) => Ok(true),
                        Err(NOT_FOUND) => Ok(false),
                        Err(error) => Err(error),
                    }
                })?;
            let table = crate::reputation_transition::framed_admission_table(
                *sections.guest_index(ADMISSION),
            )?;
            crate::reputation::check_storage_budget(len, table.len(), 0)?;
            joint_out
                .get_mut(len..len + table.len())
                .ok_or(CAPACITY)?
                .guest_copy_from_slice(table);
            joint_out.get(..len + table.len()).ok_or(CAPACITY)?
        }
    };
    let revision = current_revision.checked_add(1).ok_or(ARITHMETIC)?;
    let mut next_policy = selected;
    next_policy.task_region = if pending.is_some() {
        &crate::registry_ops::EMPTY_TASK_REGION
    } else {
        tasks::region_after_open(selected.task_region)?
    };
    next_policy.header.state_revision = revision;
    let policy_len = next_policy.encode(policy_out)?;
    let next_state = codec::StateFrame {
        revision,
        sections: [
            policy_out.guest_index(..policy_len),
            identity_out.guest_index(..frozen.identity_len),
            *sections.guest_index(Section::CurrentReports.index()),
            settlement_out.guest_index(..settlement_len),
            admission_next,
            rolled.sections[Section::Control.index()],
        ],
    };
    check_f01_capacity(policy_len, next_state.encoded_len()?)?;
    let state_len = codec::encode_state(&next_state, next)?;
    Ok(Composed {
        frozen: binding,
        revision,
        state_len,
        cursor: count as u16,
        participant_count: count as u16,
    })
}

/// The binding an `OPEN_EPOCH` at `height` would freeze over `current`, so a keeper can
/// name the frozen config and roster digest in its envelope. Nothing is committed; `next`
/// (at least `MAX_STATE_BYTES`) and `scratch` (at least [`OPEN_SCRATCH_BYTES`]) are working
/// buffers only.
///
/// # Errors
/// Every refusal [`open_epoch`] returns from the composed opening.
pub fn preview_open(
    current: &[u8],
    height: u64,
    next: &mut [u8],
    scratch: &mut [u8],
) -> CodecResult<Frozen> {
    Ok(compose(current, height, next, scratch)?.frozen)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn check_roster(
    claimed: Presence<RosterDigest>,
    epoch: u64,
    frozen: RosterDigest,
) -> CodecResult<()> {
    match claimed {
        Presence::Present(roster) if roster == frozen => Ok(()),
        Presence::Absent if epoch == 0 => Ok(()),
        _ => Err(WRONG_ROSTER),
    }
}

/// Object-local repeat: the clock epoch is already open with an F06 reservation.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn already_opened(
    section: &[u8],
    header: &MarketHeader,
    e: &Envelope<'_>,
    epoch: u64,
    admission: &AdmissionTable,
) -> CodecResult<Option<RosterDigest>> {
    if admission.current_epoch() != Some(epoch) {
        return Ok(None);
    }
    if section.is_empty() {
        return Err(WRONG_EPOCH);
    }
    let row = crate::rewards::committed_row(section, epoch)
        .map_err(|error| {
            if error == NOT_FOUND {
                WRONG_EPOCH
            } else {
                error
            }
        })?
        .row;
    if e.config != header.active_config_version {
        return Err(WRONG_CONFIG);
    }
    check_roster(e.roster, epoch, row.roster)?;
    Ok(Some(row.roster))
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn open_suffix(f: &Frozen, cursor: u16, count: u16) -> CodecResult<[u8; OPEN_SUFFIX_BYTES]> {
    let mut suffix = [0; OPEN_SUFFIX_BYTES];
    let mut w = Writer::new(&mut suffix);
    crate::codec::write_fields!(w;
        id32: f.policy.as_bytes();
        id32: f.roster.as_bytes();
        u128: f.budget;
        u8: f.workers;
        u8: f.evaluators;
        u8: u8::from(f.policy_activated);
        u8: u8::from(f.previous.is_some());
    )?;
    w.u64(f.previous.unwrap_or(0))?;
    w.u64(f.skipped)?;
    w.u16(cursor)?;
    w.u16(count)?;
    Ok(suffix)
}

/// `OPEN_EPOCH` (0x0111): permissionless, object-local (`actor_sequence = 0`, no replay
/// record). Opens the clock epoch of the authenticated `ctx.height` over the committed
/// `current` state and writes the whole next state into `next` (at least
/// `MAX_STATE_BYTES`) and its event into `event`. `scratch` holds at least
/// [`OPEN_SCRATCH_BYTES`]. Dispatch arm:
/// `dispatch::OPEN_EPOCH => epoch::open_epoch(&ctx, &envelope, current, next, scratch, event)`.
///
/// # Errors
/// `UNKNOWN_OPERATION`; envelope principal, domain and expiry refusals; `WRONG_MARKET`;
/// `NON_CANONICAL` for a non-empty payload or an inconsistent committed state;
/// `F08_MARKET_PAUSED` while suspended, `F01_LIFECYCLE_CLOSED` once closing; `WRONG_EPOCH`
/// before origin or for an envelope epoch other than the clock epoch; `ARITHMETIC` when the
/// epoch windows overflow; `WRONG_PHASE` outside Work or while the previous epoch is not
/// terminal; `WRONG_CONFIG`/`WRONG_ROSTER` for an envelope that does not bind the frozen
/// config and roster; `ROLE_CONFLICT`; `READINESS_BLOCKED`; `INSUFFICIENT_FREE`;
/// `ACCOUNT_BINDING`; and every rollover, F03 snapshot, F06 reserve, capacity and codec
/// refusal. On any error `current` is unchanged and the outputs must be discarded.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn open_epoch(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Outcome> {
    let sections = crate::state::SectionReader::new(current)?;
    open_epoch_sections(ctx, envelope, &sections, next, scratch, event)
}

pub(crate) fn open_epoch_sections(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    sections: &crate::state::SectionReader<'_>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Outcome> {
    let e = &envelope.envelope;
    if e.operation != dispatch::OPEN_EPOCH {
        return Err(UNKNOWN_OPERATION);
    }
    let section = PolicySection::decode(sections.section(Section::PolicyLifecycle))?;
    if section.header.state_revision != sections.revision() {
        return Err(NON_CANONICAL);
    }
    authenticate(ctx, envelope, &section.header)?;
    if !e.payload.is_empty() {
        return Err(NON_CANONICAL);
    }
    let epoch = opening_epoch(&section.header, ctx.height)?;
    if e.epoch != epoch {
        return Err(WRONG_EPOCH);
    }
    let table = crate::reputation_transition::framed_admission_table(
        sections.section(Section::ReputationAdmission),
    )?;
    let admission = AdmissionTable::decode(table)?;
    if let Some(roster) = already_opened(
        crate::state::room_market_parts(sections.section(Section::SettlementClaims))?.0,
        &section.header,
        e,
        epoch,
        &admission,
    )? {
        return Ok(Outcome::AlreadyApplied { epoch, roster });
    }
    let composed = compose_sections(
        sections,
        &section,
        admission,
        roster::committed_reward_state,
        true,
        ctx.height,
        next,
        scratch,
    )?;
    let frozen = composed.frozen;
    if e.config != frozen.config.get() {
        return Err(WRONG_CONFIG);
    }
    check_roster(e.roster, epoch, frozen.roster)?;
    let suffix = open_suffix(&frozen, composed.cursor, composed.participant_count)?;
    let result = codec::result_digest(&suffix)?;
    let event_len = codec::encode_event_frame(
        dispatch::OPEN_EPOCH,
        &EventCommon {
            market: section.header.market_id,
            epoch,
            config: frozen.config,
            revision: composed.revision,
            request: envelope.request_digest()?,
            result,
        },
        &suffix,
        event,
    )?;
    Ok(Outcome::Opened {
        frozen,
        cursor: composed.cursor,
        revision: composed.revision,
        result,
        state_len: composed.state_len,
        event_len,
    })
}

/// Readiness of an epoch that is already open: its reserved F06 row and the F03 snapshot
/// without evaluators revoked since.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn opened_readiness(
    state: &SharedState<'_>,
    policy: &TaskPolicyV1,
    epoch: u64,
) -> CodecResult<Readiness> {
    let section = state.room_market()?.0;
    let rewards = decode_reward_state(section.get(..REWARD_STATE_BYTES).ok_or(READINESS_BLOCKED)?)?;
    let row = rewards.row(epoch)?;
    if row.status != EpochStatus::Reserved {
        return Err(READINESS_BLOCKED);
    }
    if row.budget < policy.minimum_epoch_funding {
        return Err(INSUFFICIENT_FREE);
    }
    let region = authority::evaluator_region(
        *state
            .feature_sections
            .guest_index(Section::IdentityRoster.index()),
    )?;
    let snapshot = region
        .snapshot()
        .filter(|s| s.epoch == epoch)
        .ok_or(READINESS_BLOCKED)?;
    let evaluators = snapshot
        .entries()
        .filter(|f| !region.excluded(f.entry.evaluator))
        .count();
    let workers = row.entries().len();
    check_readiness(policy, workers, evaluators)?;
    Ok(Readiness {
        epoch,
        workers,
        evaluators,
        budget: row.budget,
    })
}

/// Writes the activated state (lifecycle ACTIVE, schedule cleared, one revision increment)
/// into `next`, then proves readiness against it: an already opened clock epoch must still
/// be ready, otherwise a complete trial opening must succeed. The trial is discarded.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn probe(
    current: &[u8],
    height: u64,
    next: &mut [u8],
    scratch: &mut [u8],
) -> CodecResult<(Readiness, u64, usize)> {
    let (state, section) = committed(current)?;
    let (policy_out, rest) = scratch.split_at_mut_checked(POLICY_CAP).ok_or(CAPACITY)?;
    let (control_out, rest) = rest.split_at_mut_checked(CONTROL_CAP).ok_or(CAPACITY)?;
    let (trial, open_scratch) = rest.split_at_mut_checked(MAX_STATE_BYTES).ok_or(CAPACITY)?;
    let revision = state.revision.checked_add(1).ok_or(ARITHMETIC)?;
    let mut active = section;
    active.header.lifecycle = ACTIVE;
    active.header.activation_scheduled = false;
    active.header.activation_epoch = 0;
    active.header.state_revision = revision;
    let policy_len = active.encode(policy_out)?;
    let mut feature_sections = state.feature_sections;
    *feature_sections.guest_index_mut(Section::PolicyLifecycle.index()) =
        policy_out.guest_index(..policy_len);
    let candidate = SharedState {
        revision,
        feature_sections,
        control: state.control.clone(),
    };
    check_f01_capacity(policy_len, candidate.encoded_len()?)?;
    let state_len = encode_shared_state(&candidate, next, control_out)?;
    let epoch = work_epoch(&active.header, height)?;
    let admission =
        AdmissionTable::decode(split_joint(*state.feature_sections.guest_index(ADMISSION))?.1)?;
    let readiness = if admission.current_epoch() == Some(epoch) {
        opened_readiness(&state, &active.current, epoch)?
    } else {
        let candidate = next.get(..state_len).ok_or(CAPACITY)?;
        let trial = compose(candidate, height, trial, open_scratch)?.frozen;
        Readiness {
            epoch,
            workers: usize::from(trial.workers),
            evaluators: usize::from(trial.evaluators),
            budget: trial.budget,
        }
    };
    Ok((readiness, revision, state_len))
}

/// The stable reason behind `ActivationNotReady`: readiness of the clock epoch of `height`
/// as `ADVANCE_ACTIVATION` evaluates it. `next` (at least `MAX_STATE_BYTES`) and `scratch`
/// (at least [`ADVANCE_SCRATCH_BYTES`]) are working buffers only.
///
/// # Errors
/// `READINESS_BLOCKED`, `INSUFFICIENT_FREE`, `ROLE_CONFLICT`, `ACCOUNT_BINDING`,
/// `WRONG_PHASE` and every refusal of a trial opening.
pub fn activation_readiness(
    current: &[u8],
    height: u64,
    next: &mut [u8],
    scratch: &mut [u8],
) -> CodecResult<Readiness> {
    Ok(probe(current, height, next, scratch)?.0)
}

fn parse_advance(payload: &[u8]) -> CodecResult<(u64, u64)> {
    let mut r = Reader::new(payload);
    let expected = (r.u64()?, r.u64()?);
    r.finish()?;
    Ok(expected)
}

/// `ADVANCE_ACTIVATION` (0x0105): anyone, object-local, payload
/// `expected_state_revision:u64 || expected_activation_epoch:u64`. Only during Work of a
/// clock epoch at or after the scheduled epoch; on success the lifecycle becomes ACTIVE at
/// this committed height with one revision increment. It opens no epoch and books no budget.
/// Dispatch arm: `dispatch::ADVANCE_ACTIVATION => epoch::advance_activation(&ctx,
/// &envelope, current, next, scratch, event)`.
///
/// # Errors
/// `UNKNOWN_OPERATION`; envelope refusals; `NON_CANONICAL` for a malformed payload;
/// `WRONG_MARKET`; `WRONG_CONFIG`; `F01_ALREADY_ACTIVATED`; `F01_LIFECYCLE_CLOSED`;
/// `NOT_FOUND` without a schedule; `F01_STALE_REVISION`; `CONFLICT` for another scheduled
/// epoch; `WRONG_EPOCH`/`ARITHMETIC`/`WRONG_PHASE` from the clock;
/// `F01_ACTIVATION_TOO_EARLY`; `F01_ACTIVATION_NOT_READY` for any failed prerequisite (see
/// [`activation_readiness`]). The schedule, funds and `current` are unchanged on refusal.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn advance_activation(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Activated> {
    let e = &envelope.envelope;
    if e.operation != dispatch::ADVANCE_ACTIVATION {
        return Err(UNKNOWN_OPERATION);
    }
    let (expected_revision, expected_epoch) = parse_advance(e.payload)?;
    let (state, section) = committed(current)?;
    let header = section.header;
    authenticate(ctx, envelope, &header)?;
    if e.config != header.active_config_version {
        return Err(WRONG_CONFIG);
    }
    match header.lifecycle {
        REGISTERED | SUSPENDED => {}
        ACTIVE => return Err(F01_ALREADY_ACTIVATED),
        _ => return Err(F01_LIFECYCLE_CLOSED),
    }
    if !header.activation_scheduled {
        return Err(NOT_FOUND);
    }
    if expected_revision != state.revision {
        return Err(F01_STALE_REVISION);
    }
    if expected_epoch != header.activation_epoch {
        return Err(CONFLICT);
    }
    let epoch = work_epoch(&header, ctx.height)?;
    if epoch < header.activation_epoch {
        return Err(F01_ACTIVATION_TOO_EARLY);
    }
    let (readiness, revision, state_len) =
        probe(current, ctx.height, next, scratch).map_err(|_| F01_ACTIVATION_NOT_READY)?;
    let mut suffix = [0; ACTIVATION_SUFFIX_BYTES];
    let mut w = Writer::new(&mut suffix);
    crate::codec::write_fields!(w;
        u64: header.activation_epoch;
        u64: epoch;
        u64: ctx.height;
    )?;
    let result = codec::result_digest(&suffix)?;
    let event_len = codec::encode_event_frame(
        dispatch::ADVANCE_ACTIVATION,
        &EventCommon {
            market: header.market_id,
            epoch,
            config: Version::new(header.active_config_version)?,
            revision,
            request: envelope.request_digest()?,
            result,
        },
        &suffix,
        event,
    )?;
    Ok(Activated {
        readiness,
        activation_epoch: header.activation_epoch,
        revision,
        result,
        state_len,
        event_len,
    })
}

/// Runs `produce` over `current` without its F07 region and joins `update`'s region back into
/// the produced state. `produce` receives the region-free state and `next`, and returns its
/// outcome with the written state length (`None` when it wrote nothing). Without an F07 region
/// `produce` runs over `current` unchanged. The returned length is the length in `next`.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn with_region<T>(
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    strip: bool,
    produce: impl FnOnce(&[u8], &mut [u8]) -> CodecResult<(T, Option<usize>)>,
    update: impl FnOnce(Region, &SharedState<'_>, &SharedState<'_>, &T) -> CodecResult<Region>,
) -> CodecResult<(T, Option<usize>)> {
    crate::state::with_shared_state(current, |state| {
        let (region, table) = split_joint(*state.feature_sections.guest_index(ADMISSION))?;
        let Some(region) = region else {
            return produce(current, next);
        };
        let (stripped, rest) = scratch
            .split_at_mut_checked(MAX_STATE_BYTES)
            .ok_or(CAPACITY)?;
        let (joint, control) = rest.split_at_mut_checked(JOINT_OUT).ok_or(CAPACITY)?;
        let input = if strip {
            let len = strip_region(state.clone(), table, stripped, control)?;
            stripped.get(..len).ok_or(CAPACITY)?
        } else {
            current
        };
        let (value, written) = produce(input, next)?;
        let Some(written) = written else {
            return Ok((value, None));
        };
        let produced = decode_shared_state(next.get(..written).ok_or(CAPACITY)?)?;
        let produced_joint = produced.feature_sections[ADMISSION];
        let table = crate::reputation_transition::framed_admission_table(produced_joint)?;
        if strip {
            if table.len() != produced_joint.len() {
                return Err(NON_CANONICAL);
            }
        } else {
            let before = state.feature_sections[ADMISSION];
            let old_table = crate::reputation_transition::framed_admission_table(before)?;
            if before.len() - old_table.len() != produced_joint.len() - table.len() {
                return Err(NON_CANONICAL);
            }
        }
        let region = update(region, &state, &produced, &value)?;
        let len = encode_joint(&region, table, joint)?;
        let policy_len = produced.feature_sections[Section::PolicyLifecycle.index()].len();
        let state_len = crate::state::replace_encoded_section(
            next,
            written,
            Section::ReputationAdmission,
            &joint[..len],
        )?;
        check_f01_capacity(policy_len, state_len)?;
        Ok((value, Some(state_len)))
    })
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn strip_region(
    state: SharedState<'_>,
    table: &[u8],
    stripped: &mut [u8],
    control: &mut [u8],
) -> CodecResult<usize> {
    let without = state.with_section(Section::ReputationAdmission, table)?;
    encode_shared_state(&without, stripped, control)
}

/// Runs any producer that reads the F08 table only (F02, F03, F04, F06, F08, F09, F10, F01
/// tasks and registry operations) over `current` without its F07 region, then carries the
/// region byte-identical into the produced state. `produce(current, next)` returns its
/// outcome and the state length it wrote into `next` (`None` when it wrote nothing); the
/// returned length replaces it. `scratch` holds at least [`CARRY_SCRATCH_BYTES`] and must not
/// alias the producer's own scratch. Without an F07 region `produce` runs over `current`.
///
/// # Errors
/// Propagates `produce`'s refusals; `NON_CANONICAL` when the produced joint section carries an
/// F07 region; the joint split/encode, capacity and state codec refusals. On any error
/// `current` is unchanged and the outputs must be discarded.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn carry_reputation<T>(
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    produce: impl FnOnce(&[u8], &mut [u8]) -> CodecResult<(T, Option<usize>)>,
) -> CodecResult<(T, Option<usize>)> {
    with_region(current, next, scratch, true, produce, |region, _, _, _| {
        Ok(region)
    })
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn committed_epoch_row(state: &SharedState<'_>, epoch: u64) -> CodecResult<RewardEpoch> {
    Ok(crate::rewards::committed_row(state.room_market()?.0, epoch)?.row)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn epoch_row<'a>(
    state: &SharedState<'a>,
    epoch: u64,
) -> CodecResult<(RewardState<'a>, RewardEpoch)> {
    let settlement = state.room_market()?.0;
    let rewards = decode_reward_state(settlement.get(..REWARD_STATE_BYTES).ok_or(WRONG_EPOCH)?)?;
    let row = rewards.row(epoch)?;
    Ok((rewards, row))
}

/// `history_allowed` frozen at the F05 seal of `epoch` from the sealed `produced` state.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn seal_region_bytes(
    source: &codec::StateFrame<'_>,
    epoch: u64,
    captured: &CapturedRoster,
    eligible: u8,
    out: &mut [u8],
) -> CodecResult<usize> {
    use crate::reputation::{CURRENT_BYTES, HEADER_BYTES, HISTORY_BYTES};
    use crate::reputation_transition::SEAL_BYTES;
    let section = source.sections[ADMISSION];
    let table = crate::reputation_transition::framed_admission_table(section)?;
    let mut r = Reader::new(section);
    r.take(6)?;
    let count = usize::from(r.u8()?);
    let history = usize::from(r.u8()?);
    let market = r.market_id()?;
    let completed = r.boolean()?;
    let latest = r.u64()?;
    let base_len = HEADER_BYTES + count * CURRENT_BYTES + history * HISTORY_BYTES;
    if market != captured.binding.market || captured.binding.epoch != epoch {
        return Err(F07_BINDING_MISMATCH);
    }
    if section.len() - table.len() != base_len {
        return Err(NON_CANONICAL);
    }
    if completed && epoch <= latest {
        return Err(WRONG_EPOCH);
    }
    if usize::from(eligible) > MAX_EVALUATORS {
        return Err(F07_BINDING_MISMATCH);
    }
    // Only the history status and worker identity participate in a seal. The
    // opening checked the immutable records; preserve all their other bytes.
    let mut records = section
        .get(HEADER_BYTES..HEADER_BYTES + count * CURRENT_BYTES)
        .ok_or(NON_CANONICAL)?
        .chunks_exact(CURRENT_BYTES)
        .peekable();
    let mut allowed = 0u32;
    for (i, worker) in captured.workers().iter().enumerate() {
        while records
            .peek()
            .is_some_and(|row| row.guest_index(..32) < worker.worker.as_bytes().as_slice())
        {
            records.next();
        }
        let row = records
            .next()
            .filter(|row| row.guest_index(..32) == worker.worker.as_bytes())
            .ok_or(F07_UNKNOWN_WORKER)?;
        if HistoryStatus::decode(*row.guest_index(CURRENT_BYTES - 4))? == HistoryStatus::Active {
            allowed |= 1u32 << i;
        }
    }
    crate::reputation::check_storage_budget(base_len + SEAL_BYTES, table.len(), 0)?;
    let mut w = Writer::new(out);
    w.put(section.get(..base_len).ok_or(NON_CANONICAL)?)?;
    crate::reputation_transition::write_seal(
        crate::reputation_transition::HistorySeal {
            epoch,
            allowed,
            eligible,
        },
        &mut w,
    )?;
    w.put(table)?;
    Ok(w.len())
}

/// Stage at most eight participant observations, keeping F07 unchanged until completion.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn stage_finalization(
    frame: &codec::StateFrame<'_>,
    captured: &CapturedRoster,
    aggregation: &crate::aggregation_codec::EpochAggregation<'_>,
    height: u64,
    pending: &[u8],
) -> CodecResult<ParticipantJournal> {
    let section = PolicySection::decode(frame.sections[Section::PolicyLifecycle.index()])?;
    let (region, _) =
        crate::reputation_transition::split_committed_joint(frame.sections[ADMISSION])?;
    let roster = captured.workers();
    let mut journal = if pending.is_empty() {
        ParticipantJournal {
            epoch: captured.binding.epoch,
            height,
            cursor: 0,
            count: roster.len() as u16,
            observed: 0,
            covered: 0,
            config: captured.binding.config.get(),
            roster: captured.binding.roster,
            state: region.as_ref().map(|r| r.state.clone()),
        }
    } else {
        ParticipantJournal::decode(pending, true)?
    };
    if journal.epoch != captured.binding.epoch
        || journal.config != captured.binding.config.get()
        || journal.roster != captured.binding.roster
        || usize::from(journal.count) != roster.len()
    {
        return Err(WRONG_ROSTER);
    }
    let start = usize::from(journal.cursor);
    if let Some(region) = region {
        let mut observations = [WorkerObservation {
            median: Presence::Absent,
            support: 0,
        }; MAX_WORKERS];
        if aggregation.outputs().len() != roster.len() {
            return Err(F07_BINDING_MISMATCH);
        }
        for ((observation, member), output) in observations
            .iter_mut()
            .zip(roster)
            .zip(aggregation.outputs())
        {
            if output.worker() != member.worker {
                return Err(F07_BINDING_MISMATCH);
            }
            *observation = WorkerObservation {
                median: output.quality(),
                support: output.support(),
            };
        }
        let (state, cursor, observed, covered) =
            crate::reputation_transition::complete_epoch_chunk(
                &region.state,
                journal.state.as_ref().ok_or(NON_CANONICAL)?,
                region.seal,
                captured.binding,
                segment_binding(&section)?,
                aggregation.root(),
                journal.height,
                roster,
                observations.guest_index(..roster.len()),
                start,
                journal.observed,
                journal.covered,
            )?;
        journal.state = Some(state);
        journal.cursor = cursor as u16;
        journal.observed = observed;
        journal.covered = covered;
    } else {
        journal.cursor = (start + crate::aggregation::participant_chunk_size(roster.len()))
            .min(roster.len()) as u16;
    }
    Ok(journal)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn commit_finalized_region(
    outcome: Aggregated,
    reputation: Option<&crate::reputation::ReputationState>,
    next: &mut [u8],
    joint: &mut [u8],
) -> CodecResult<Aggregated> {
    let Some(reputation) = reputation else {
        return Ok(outcome);
    };
    let Aggregated::Applied {
        progress,
        response,
        revision,
        result,
        state_len,
        event_len,
    } = outcome
    else {
        return Err(NON_CANONICAL);
    };
    let produced = codec::decode_state(&next[..state_len])?;
    let table = crate::reputation_transition::framed_admission_table(produced.sections[ADMISSION])?;
    let region = Region {
        state: reputation.clone(),
        seal: Presence::Absent,
    };
    let len = crate::reputation_transition::encode_produced_joint(&region, table, joint)?;
    let state_len = crate::state::replace_encoded_section(
        next,
        state_len,
        Section::ReputationAdmission,
        &joint[..len],
    )?;
    Ok(Aggregated::Applied {
        progress,
        response,
        revision,
        result,
        state_len,
        event_len,
    })
}

/// `BeginAggregation`/`ProcessAggregation`/`FinalizeAggregation` (0x0501-0x0503) with the F07
/// phases of the same atomic transition: Begin freezes `history_allowed` and the eligible
/// evaluator count beside the F05 seal; Process carries the region; Finalize runs
/// `ApplyCompletedEpoch` over the sealed F05 outputs in the F05/F06 terminal transition, so
/// a refused F07 phase refuses the whole terminalization. `scratch` holds at least
/// [`AGGREGATE_SCRATCH_BYTES`]. Dispatch arm: `dispatch::BeginAggregation |
/// dispatch::ProcessAggregation | dispatch::FinalizeAggregation => epoch::aggregate(&ctx,
/// &envelope, current, next, scratch, event)`. Without an F07 region this is
/// `aggregation::apply`.
///
/// # Errors
/// Every `aggregation::apply` refusal; `NON_CANONICAL` for a second seal of an open epoch;
/// `F07_EPOCH_NOT_SEALED` for a completion without its seal; `F07_BINDING_MISMATCH`,
/// `F07_UNKNOWN_WORKER`, `RETENTION_FULL`, `WRONG_EPOCH` and `F07_RESOURCE_LIMIT` from the F07
/// completion; capacity and codec refusals. On any error `current` is unchanged and the outputs
/// must be discarded.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn aggregate(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Aggregated> {
    let operation = envelope.envelope.operation;
    if matches!(
        operation,
        dispatch::ProcessAggregation | dispatch::FinalizeAggregation
    ) {
        return aggregation::apply(ctx, envelope, current, next, scratch, event);
    }
    let (inner, carry) = scratch
        .split_at_mut_checked(aggregation::SCRATCH_BYTES)
        .ok_or(CAPACITY)?;
    let reader = crate::state::SectionReader::new(current)?;
    let state = reader.frame();
    {
        let old_table =
            crate::reputation_transition::framed_admission_table(state.sections[ADMISSION])?;
        if old_table.len() == state.sections[ADMISSION].len() {
            return aggregation::apply(ctx, envelope, current, next, inner, event);
        }
        let (_, rest) = carry
            .split_at_mut_checked(MAX_STATE_BYTES)
            .ok_or(CAPACITY)?;
        let (joint, _) = rest.split_at_mut_checked(JOINT_OUT).ok_or(CAPACITY)?;
        let mut finish = |outcome: Aggregated,
                          captured: &CapturedRoster,
                          eligible: u8,
                          terminal: Option<&crate::aggregation_codec::EpochAggregation<'_>>,
                          next: &mut [u8]| {
            let Aggregated::Applied {
                progress,
                response,
                revision,
                result,
                state_len,
                event_len,
            } = outcome
            else {
                return Err(NON_CANONICAL);
            };
            let produced = codec::decode_state(next.get(..state_len).ok_or(CAPACITY)?)?;
            let produced_joint = produced.sections[ADMISSION];
            let table = crate::reputation_transition::framed_admission_table(produced_joint)?;
            if state.sections[ADMISSION].len() - old_table.len()
                != produced_joint.len() - table.len()
            {
                return Err(NON_CANONICAL);
            }
            let _ = terminal;
            let joint_len = seal_region_bytes(&state, progress.epoch, captured, eligible, joint)?;
            let policy_len = produced.sections[Section::PolicyLifecycle.index()].len();
            let state_len = crate::state::replace_encoded_section(
                next,
                state_len,
                Section::ReputationAdmission,
                &joint[..joint_len],
            )?;
            check_f01_capacity(policy_len, state_len)?;
            Ok(Aggregated::Applied {
                progress,
                response,
                revision,
                result,
                state_len,
                event_len,
            })
        };
        aggregation::apply_finishing(
            ctx,
            envelope,
            current,
            next,
            inner,
            event,
            Some(&mut finish),
        )
    }
}

/// Outcome of one `ResetHistory`/`SuspendHistory`/`ResumeHistory`.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum HistoryOutcome {
    /// One revision increment composed into `next`, the retained result recorded under the
    /// actor's sequence and the `HistoryReset`/`HistoryStatus` event written into `event`.
    Applied {
        record: ReputationCurrent,
        revision: u64,
        result: ResultDigest,
        state_len: usize,
        event_len: usize,
    },
    /// Exact repetition of an applied request: its retained result; nothing was written.
    Retained(RetainedResult),
}

/// `ResetHistory`/`SuspendHistory`/`ResumeHistory` payload.
struct HistoryRequest {
    worker: WorkerId,
    segment: Digest32,
    generation: u64,
    reason: u8,
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn parse_history(payload: &[u8]) -> CodecResult<HistoryRequest> {
    if payload.len() != HISTORY_PAYLOAD_BYTES {
        return Err(NON_CANONICAL);
    }
    let mut r = Reader::new(payload);
    let request = crate::codec::read_record!(&mut r, HistoryRequest {  }; worker = r.worker_id()?;
        segment: Digest32,
        generation: U64,
        reason: U8
    );
    r.finish()?;
    Ok(request)
}

/// The authorized principal and replay slot of a history request: the worker owner for
/// `OWNER_RESET`, the market owner otherwise.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn history_authority(
    operation: dispatch::Operation,
    request: &HistoryRequest,
    header: &MarketHeader,
    identity: &[u8],
) -> CodecResult<(PrincipalId, ActorSlot)> {
    let market_owner = (header.owner_principal, ActorSlot::OWNER);
    if operation == dispatch::ResetHistory {
        if ClosureReason::decode(request.reason)? != ClosureReason::OwnerReset {
            return Ok(market_owner);
        }
        let workers = WorkerTable::decode(split_identity_section(identity)?.0)?;
        let record = workers.get(request.worker).ok_or(F07_UNKNOWN_WORKER)?;
        return Ok((record.owner, ActorSlot::worker(usize::from(record.slot))?));
    }
    let valid = if operation == dispatch::SuspendHistory {
        matches!(request.reason, 1 | 2)
    } else {
        request.reason == 1
    };
    if valid {
        Ok(market_owner)
    } else {
        Err(NON_CANONICAL)
    }
}

/// The authorized role-sequenced replay request of a history call, or the retained result of
/// its exact repetition.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn history_replay(
    envelope: &ValidatedEnvelope<'_>,
    state: &SharedState<'_>,
    header: &MarketHeader,
    body: &HistoryRequest,
    height: u64,
) -> CodecResult<Result<ReplayRequest, RetainedResult>> {
    let e = &envelope.envelope;
    let identity = *state
        .feature_sections
        .guest_index(Section::IdentityRoster.index());
    let (principal, slot) = history_authority(e.operation, body, header, identity)?;
    if e.actor != principal {
        return Err(UNAUTHORIZED);
    }
    let actor = state.control.replay.actor(slot).ok_or(UNAUTHORIZED)?;
    if actor.principal != principal {
        return Err(UNAUTHORIZED);
    }
    let request = ReplayRequest::from_envelope(slot, actor.authority_version, envelope)?;
    match state.control.replay.check(&request, height) {
        Ok(ReplayDecision::AlreadyApplied(retained)) => Ok(Err(retained)),
        Ok(ReplayDecision::Apply) => Ok(Ok(request)),
        Err(REPLAY_CONFLICT) => Err(F07_IDEMPOTENCY_CONFLICT),
        Err(error) => Err(error),
    }
}

/// Refuses a reset while the opened, nonterminal epoch's frozen roster binds `worker`.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn check_not_frozen(state: &SharedState<'_>, table: &[u8], worker: WorkerId) -> CodecResult<()> {
    let Some(epoch) = AdmissionTable::decode(table)?.current_epoch() else {
        return Ok(());
    };
    let (rewards, row) = epoch_row(state, epoch)?;
    if row.status != EpochStatus::Reserved {
        return Ok(());
    }
    let dictionary = rewards.dictionary();
    for entry in row.entries() {
        if dictionary.slot(entry.slot)?.worker == worker {
            return Err(F07_IDENTITY_FROZEN);
        }
    }
    Ok(())
}

/// The record a checked history request produces and its event suffix written into `suffix`:
/// the reset closes the segment into the next generation under `binding`; suspension and
/// resumption change only the status.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn history_transition(
    operation: dispatch::Operation,
    (record, key): (ReputationCurrent, SegmentKey),
    binding: SegmentBinding,
    body: &HistoryRequest,
    latest: Presence<u64>,
    height: u64,
    suffix: &mut [u8],
) -> CodecResult<(ReputationCurrent, usize)> {
    let mut w = Writer::new(suffix);
    if operation == dispatch::ResetHistory {
        let (_, updated) = reset_segment(
            &record,
            key,
            binding,
            ClosureReason::decode(body.reason)?,
            latest,
            height,
        )?;
        crate::codec::write_fields!(w;
            id32: body.worker.as_bytes();
            id32: record.segment.as_bytes();
            id32: updated.segment.as_bytes();
            u64: updated.reset_generation.get();
            u8: body.reason;
        )?;
        return Ok((updated, RESET_SUFFIX_BYTES));
    }
    let status = if operation == dispatch::SuspendHistory {
        HistoryStatus::Suspended
    } else {
        HistoryStatus::Active
    };
    let updated = set_status(&record, key, status, height)?;
    crate::codec::write_fields!(w;
        id32: body.worker.as_bytes();
        id32: updated.segment.as_bytes();
        u64: updated.reset_generation.get();
        u8: status as u8;
        u8: body.reason;
    )?;
    Ok((updated, STATUS_SUFFIX_BYTES))
}

/// `ResetHistory` (0x0701), `SuspendHistory` (0x0702) and `ResumeHistory` (0x0703): role
/// sequenced, payload `worker_id32 || expected_segment_digest32 || expected_reset_generation:u64
/// || reason:u8`. `OWNER_RESET` is the worker owner's (its F02 actor slot); every other reset
/// reason, suspension and resumption are the market owner's. The expected reset generation is
/// checked before the segment digest. A reset refuses while the opened nonterminal epoch's
/// frozen roster binds the worker, closes the segment (closure digest), increments the reset
/// generation and zeroes quality, count and coverage under the current binding, retaining
/// status and owner; suspension and resumption change the status prospectively and never the
/// sealed `history_allowed` of an epoch in aggregation. Rewards, membership and F05 voting
/// are untouched. An exact repetition returns the retained result and writes nothing.
/// `scratch` holds at least [`HISTORY_SCRATCH_BYTES`]. Dispatch arm:
/// `dispatch::ResetHistory | dispatch::SuspendHistory | dispatch::ResumeHistory =>
/// epoch::history(&ctx, &envelope, current, next, scratch, event)`.
///
/// # Errors
/// `UNKNOWN_OPERATION`; envelope principal, domain and expiry refusals; `WRONG_MARKET`;
/// `WRONG_CONFIG`; `NON_CANONICAL` for a malformed payload, reason or committed state;
/// `F07_UNKNOWN_WORKER` without an F07 region or record; `UNAUTHORIZED` for any other actor
/// or a delegate; the replay refusals with `F07_IDEMPOTENCY_CONFLICT` for a reused sequence;
/// `F07_GENERATION_MISMATCH`; `F07_SEGMENT_MISMATCH`; `F07_IDENTITY_FROZEN`; `CONFLICT` for a
/// retired record or an unchanged status; `F07_BINDING_MISMATCH` for a model/policy reason
/// without that change; capacity and codec refusals. On any error `current` is unchanged and
/// the outputs must be discarded.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn history(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<HistoryOutcome> {
    let e = &envelope.envelope;
    if e.operation != dispatch::ResetHistory
        && e.operation != dispatch::SuspendHistory
        && e.operation != dispatch::ResumeHistory
    {
        return Err(UNKNOWN_OPERATION);
    }
    let (state, mut section) = committed(current)?;
    authenticate(ctx, envelope, &section.header)?;
    if e.config != section.header.active_config_version {
        return Err(WRONG_CONFIG);
    }
    let body = parse_history(e.payload)?;
    let (region, table) = split_joint(*state.feature_sections.guest_index(ADMISSION))?;
    let region = region.ok_or(F07_UNKNOWN_WORKER)?;
    let record = *region
        .state
        .records()
        .find(|r| r.worker == body.worker)
        .ok_or(F07_UNKNOWN_WORKER)?;
    let request = match history_replay(envelope, &state, &section.header, &body, ctx.height)? {
        Ok(request) => request,
        Err(retained) => return Ok(HistoryOutcome::Retained(retained)),
    };
    check_expected(&record, body.segment, body.generation)?;
    let binding = segment_binding(&section)?;
    let (_, key) = bound_record(&region.state, body.worker, binding)?;
    if e.operation == dispatch::ResetHistory {
        if record.status == HistoryStatus::Retired {
            return Err(CONFLICT);
        }
        check_not_frozen(&state, table, body.worker)?;
    }
    let mut suffix = [0; RESET_SUFFIX_BYTES];
    let (updated, suffix_len) = history_transition(
        e.operation,
        (record, key),
        binding,
        &body,
        region.state.latest_completed(),
        ctx.height,
        &mut suffix,
    )?;
    let suffix = suffix.get(..suffix_len).ok_or(CAPACITY)?;
    let next_region = Region {
        state: replace_record(&region.state, updated)?,
        seal: region.seal,
    };
    let (policy_out, rest) = scratch.split_at_mut_checked(POLICY_CAP).ok_or(CAPACITY)?;
    let (joint_out, control_out) = rest.split_at_mut_checked(JOINT_OUT).ok_or(CAPACITY)?;
    let joint_len = encode_joint(&next_region, table, joint_out)?;
    let revision = state.revision.checked_add(1).ok_or(ARITHMETIC)?;
    section.header.state_revision = revision;
    let policy_len = section.encode(policy_out)?;
    let mut feature_sections = state.feature_sections;
    *feature_sections.guest_index_mut(Section::PolicyLifecycle.index()) =
        policy_out.get(..policy_len).ok_or(CAPACITY)?;
    *feature_sections.guest_index_mut(ADMISSION) = joint_out.get(..joint_len).ok_or(CAPACITY)?;
    let mut candidate = SharedState {
        revision: state.revision,
        feature_sections,
        control: state.control,
    };
    let result = codec::result_digest(suffix)?;
    if candidate.record_success(&request, ctx.height, result)? != ReplayDecision::Apply
        || candidate.revision != revision
    {
        return Err(NON_CANONICAL);
    }
    check_f01_capacity(policy_len, candidate.encoded_len()?)?;
    let state_len = encode_shared_state(&candidate, next, control_out)?;
    let event_len = codec::encode_event_frame(
        e.operation,
        &EventCommon {
            market: section.header.market_id,
            epoch: e.epoch,
            config: Version::new(section.header.active_config_version)?,
            revision,
            request: envelope.request_digest()?,
            result,
        },
        suffix,
        event,
    )?;
    Ok(HistoryOutcome::Applied {
        record: updated,
        revision,
        result,
        state_len,
        event_len,
    })
}
