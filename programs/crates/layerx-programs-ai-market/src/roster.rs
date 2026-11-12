//! F08 deterministic roster rollover, inactivity selection and evaluator health.
//! Selection never consults quality history (F07); it uses activity epochs and IDs only.
use crate::types::GuestIndex as _;
use crate::{
    admission::{
        AdmissionMeta, AdmissionTable, FLAG_DRAINING, MAX_MEMBERS, POLICY_V1, Participant, Role,
    },
    codec::{self, Writer},
    errors::{
        ACCOUNT_BINDING, ARITHMETIC, CAPACITY, CodecResult, F02_METADATA_INTEGRITY_FAILURE,
        F08_MARKET_PAUSED, F08_NO_PRUNABLE_MEMBER, F08_QUORUM_UNAVAILABLE, F08_RETENTION_BLOCKED,
        F08_STALE_STATE, NON_CANONICAL, WRONG_EPOCH, WRONG_PHASE,
    },
    evaluators::authority::split_identity_section,
    registry::{MarketHeader, market_clock},
    registry_ops::{ACTIVE, PolicySection, REGISTERED, SUSPENDED},
    rewards::{Disposition, EpochStatus, REWARD_STATE_BYTES, RewardState, decode_reward_state},
    state::{self, ActorSlot, Section, SharedState},
    types::{Amount, Digest32, EpochPhase, WorkerId, WorkerRosterEntry},
    workers::{CapabilityFields, WorkerState, WorkerTable, decode_manifest},
};

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ParticipantRosterEntry {
    pub worker: WorkerRosterEntry,
    pub capabilities: Option<CapabilityFields>,
}

impl ParticipantRosterEntry {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn from_manifest(worker: WorkerRosterEntry, manifest: &[u8]) -> CodecResult<Self> {
        let summary = decode_manifest(manifest)?;
        if summary.worker != worker.worker
            || summary.owner != worker.owner
            || summary.generation != worker.generation.get()
            || summary.key_version != worker.key_version.get()
            || summary.digest != worker.metadata
        {
            return Err(F02_METADATA_INTEGRITY_FAILURE);
        }
        Ok(Self {
            worker,
            capabilities: summary.capabilities,
        })
    }
}

/// Bytes per frozen member in the roster digest preimage.
const ROSTER_MEMBER_BYTES: usize = 33 + 32 + 8 + 8;

/// Unique prunable member minimizing (A(x), `admitted_epoch`, participant).
///
/// # Errors
/// `F08_NO_PRUNABLE_MEMBER` when no active member of `role` has two complete misses.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn prune_candidate(
    table: &AdmissionTable,
    role: Role,
    epoch: u64,
) -> CodecResult<AdmissionMeta> {
    table
        .iter()
        .filter(|m| {
            m.participant.role() == role
                && m.admitted()
                && !m.draining()
                && !m.revoked()
                && m.pending_exit.is_none()
                && m.admitted_epoch.is_some_and(|a| epoch > a)
                && m.complete_missed_opened_epochs >= POLICY_V1.missed_epoch_threshold
        })
        .min_by_key(|m| (m.last_activity(), m.admitted_epoch, m.participant))
        .copied()
        .ok_or(F08_NO_PRUNABLE_MEMBER)
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Health {
    pub roster_evaluators: u8,
    pub eligible_evaluators: u8,
    pub quorum_ready: bool,
}

/// Frozen evaluator count and current eligibility overlay (revoked/draining excluded).
///
/// # Errors
/// `ARITHMETIC` only if a decoded table could exceed `u8` members, which capacity forbids.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn health(table: &AdmissionTable) -> CodecResult<Health> {
    let frozen = || {
        table
            .iter()
            .filter(|m| m.participant.role() == Role::Evaluator && m.admitted_epoch.is_some())
    };
    let roster = u8::try_from(frozen().count()).map_err(|_| ARITHMETIC)?;
    let eligible = u8::try_from(frozen().filter(|m| !m.revoked() && !m.draining()).count())
        .map_err(|_| ARITHMETIC)?;
    Ok(Health {
        roster_evaluators: roster,
        eligible_evaluators: eligible,
        quorum_ready: eligible >= POLICY_V1.score_quorum,
    })
}

/// New evaluation-dependent task admission gate.
/// `tasks::admit` supplies the committed F08 table after exact-retry handling,
/// before writing a new task.
///
/// # Errors
/// `F08_QUORUM_UNAVAILABLE` below three eligible frozen evaluators.
pub fn require_quorum(table: &AdmissionTable) -> CodecResult<()> {
    if health(table)?.quorum_ready {
        Ok(())
    } else {
        Err(F08_QUORUM_UNAVAILABLE)
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Rollover {
    pub epoch: u64,
    pub removed: u8,
    pub expired: u8,
    pub installed: u8,
    pub rotated: u8,
    pub members: u8,
    pub digest: Digest32,
    pub health: Health,
}

fn bump(counter: &mut u8) -> CodecResult<()> {
    *counter = counter.checked_add(1).ok_or(ARITHMETIC)?;
    Ok(())
}

/// Shared `OpenEpoch` rollover. `retained` reports unresolved obligations of a member
/// whose exit is due; such a member refuses `F08_RETENTION_BLOCKED` instead of being
/// erased. Unconsumed approvals for an epoch before `epoch` can no longer be admitted
/// and are discarded. Refusal leaves the table unchanged.
///
/// # Errors
/// `WRONG_EPOCH` unless `epoch` advances, `F08_RETENTION_BLOCKED`, `NON_CANONICAL` and
/// `ARITHMETIC`.
pub fn open_epoch(
    table: &mut AdmissionTable,
    epoch: u64,
    retained: impl Fn(&AdmissionMeta) -> bool,
) -> CodecResult<Rollover> {
    roll(
        table,
        epoch,
        |m| Ok(retained(m)),
        |m| Ok(Some(m.delegate_generation)),
    )
}

/// Rollover kernel. `authorized` returns the delegate generation the current identity
/// record authorizes at this snapshot, or `None` while that record is not yet (or no
/// longer) valid: a staged member then stays staged and a frozen member keeps its frozen
/// generation, so pending, current and frozen versions never merge early.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn roll(
    table: &mut AdmissionTable,
    epoch: u64,
    retained: impl Fn(&AdmissionMeta) -> CodecResult<bool>,
    authorized: impl Fn(&AdmissionMeta) -> CodecResult<Option<u64>>,
) -> CodecResult<Rollover> {
    let previous = table.current_epoch();
    if previous.is_some_and(|p| epoch <= p) {
        return Err(WRONG_EPOCH);
    }
    let mut next = *table;
    let (mut removed, mut expired, mut installed, mut rotated) = (0u8, 0u8, 0u8, 0u8);
    let mut next_index = 0;
    for m in table.iter() {
        if m.pending_exit.is_some_and(|exit| exit.epoch <= epoch) {
            if retained(m)? {
                return Err(F08_RETENTION_BLOCKED);
            }
            next.remove(m.participant)?;
            bump(&mut removed)?;
            continue;
        }
        if let Some(approval) = m.approval {
            if approval.effective_epoch < epoch {
                next.remove(m.participant)?;
                bump(&mut expired)?;
            } else {
                next_index += 1;
            }
            continue;
        }
        let mut m = *m;
        let generation = authorized(&m)?;
        if generation.is_some_and(|g| g < m.delegate_generation) {
            return Err(NON_CANONICAL);
        }
        match (m.admitted_epoch, previous) {
            (None, _) => {
                let Some(g) = generation.filter(|_| m.immunity_until_epoch <= epoch) else {
                    next_index += 1;
                    continue;
                };
                m.admitted_epoch = Some(epoch);
                m.immunity_until_epoch = epoch;
                m.delegate_generation = g;
                bump(&mut installed)?;
            }
            (Some(_), None) => return Err(NON_CANONICAL),
            (Some(_), Some(p)) => {
                let a = m.last_activity().ok_or(NON_CANONICAL)?;
                m.complete_missed_opened_epochs = if p > a {
                    m.complete_missed_opened_epochs
                        .saturating_add(1)
                        .min(POLICY_V1.missed_epoch_threshold)
                } else {
                    0
                };
                if let Some(g) = generation.filter(|g| *g != m.delegate_generation) {
                    m.delegate_generation = g;
                    bump(&mut rotated)?;
                }
            }
        }
        if m.pending_exit.is_none() && !m.revoked() {
            m.membership_flags &= !FLAG_DRAINING;
        }
        next.replace_at(next_index, m)?;
        next_index += 1;
    }
    next.begin_epoch(epoch);
    let digest = roster_digest(&next, epoch)?;
    let members = u8::try_from(next.iter().filter(|m| m.admitted_epoch.is_some()).count())
        .map_err(|_| ARITHMETIC)?;
    let health = health(&next)?;
    *table = next;
    Ok(Rollover {
        epoch,
        removed,
        expired,
        installed,
        rotated,
        members,
        digest,
        health,
    })
}

/// Caller scratch for [`rollover_roster`]: the next admission table, the next identity
/// section (F02 worker table followed by the F03 region) and the next control section.
pub const ROLLOVER_SCRATCH_BYTES: usize = Section::ReputationAdmission.payload_cap()
    + Section::IdentityRoster.payload_cap()
    + Section::Control.payload_cap();

/// `RosterOpened` summary of one complete rollover over the shared state value.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct RosterOpened {
    pub rollover: Rollover,
    pub previous: Option<u64>,
    pub skipped: u64,
    pub workers: u8,
    pub evaluators: u8,
    pub preserved_entitlements: u16,
    pub preserved_amount: Amount,
    pub state_len: usize,
}

/// Committed F06 reward state at the head of the joint F05/F06 section, bound to the
/// market reserve; an empty section means rewards were never initialized.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn reward_state<'a>(
    section: &'a [u8],
    market: &MarketHeader,
) -> CodecResult<Option<RewardState<'a>>> {
    if section.is_empty() {
        return Ok(None);
    }
    let rewards = decode_reward_state(section.get(..REWARD_STATE_BYTES).ok_or(NON_CANONICAL)?)?;
    check_reward_binding(rewards, market).map(Some)
}

pub(crate) fn committed_reward_state<'a>(
    section: &'a [u8],
    market: &MarketHeader,
) -> CodecResult<Option<RewardState<'a>>> {
    if section.is_empty() {
        return Ok(None);
    }
    let rewards =
        crate::rewards::committed_state(section.get(..REWARD_STATE_BYTES).ok_or(NON_CANONICAL)?)?;
    check_reward_binding(rewards, market).map(Some)
}

fn check_reward_binding<'a>(
    rewards: RewardState<'a>,
    market: &MarketHeader,
) -> CodecResult<RewardState<'a>> {
    let ledger = rewards.ledger()?;
    if ledger.asset != market.funding_asset || ledger.account != market.rewards_account {
        return Err(ACCOUNT_BINDING);
    }
    if ledger.active_reserve {
        return Err(WRONG_PHASE);
    }
    Ok(rewards)
}

/// Unclaimed TERMINAL entitlements still owed to `worker` through its frozen recipient.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn outstanding(rewards: &RewardState<'_>, worker: WorkerId) -> CodecResult<(u16, Amount)> {
    let dictionary = rewards.dictionary();
    let (mut count, mut amount) = (0u16, 0);
    for row in rewards.rows().records() {
        let row = row?;
        if row.status != EpochStatus::Terminal {
            continue;
        }
        for entry in row.entries() {
            if entry.disposition == Disposition::Unclaimed
                && entry.entitlement > 0
                && dictionary.slot(entry.slot)?.worker == worker
            {
                count = count.checked_add(1).ok_or(ARITHMETIC)?;
                amount = Amount::checked_add(amount, entry.entitlement).ok_or(ARITHMETIC)?;
            }
        }
    }
    Ok((count, amount))
}

/// Clock epoch of `height` for a live market, inside its Work window.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn opening_epoch(market: &MarketHeader, height: u64) -> CodecResult<u64> {
    match market.lifecycle {
        REGISTERED | ACTIVE => {}
        SUSPENDED => return Err(F08_MARKET_PAUSED),
        _ => return Err(WRONG_PHASE),
    }
    if height < market.origin_height {
        return Err(WRONG_EPOCH);
    }
    let clock = market_clock(market.origin_height, height)?;
    if clock.phase != EpochPhase::Work {
        return Err(WRONG_PHASE);
    }
    Ok(clock.epoch)
}

/// Delegate generation the member's current F02 record authorizes for `epoch`; an
/// evaluator keeps its F08 generation.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn authorized(
    workers: &crate::workers::OpeningCursor<'_>,
    m: &AdmissionMeta,
    height: u64,
    epoch: u64,
) -> CodecResult<Option<u64>> {
    let Participant::Worker(worker) = m.participant else {
        return Ok(Some(m.delegate_generation));
    };
    let record = workers.get(worker)?.ok_or(NON_CANONICAL)?;
    if record.owner != m.owner {
        return Err(NON_CANONICAL);
    }
    let serving = matches!(record.state, WorkerState::Enrolled | WorkerState::Available)
        && height < record.expiry;
    let version_due =
        record.key_version == m.delegate_generation || record.effective_epoch <= epoch;
    Ok((serving && version_due).then_some(record.key_version))
}

/// `RolloverRoster` over the complete committed shared state value at authenticated
/// `height`, writing the whole next value into `next`. The epoch is the clock epoch of
/// `height`, which must lie in its Work window; skipped epochs add no samples. The
/// previous epoch must be terminal (no active F06 reserve). Due exits leave the F08
/// table, and a departing worker's F02 record and actor replay slot are retired unless
/// that slot still retains an unexpired result (`F08_RETENTION_BLOCKED`); F06 rows and
/// recipient dictionary are carried over byte for byte, so every outstanding entitlement
/// stays claimable under the old stable ID, and so is the F03 evaluator region that
/// follows the worker table in the identity section. A staged worker is installed at its F08
/// effective epoch only while its F02 record is ENROLLED/AVAILABLE with an unexpired
/// manifest; a changed F02 key version is frozen only from its F02 effective epoch, so a
/// pending rotation never enters an earlier snapshot. The revision is not advanced: the
/// enclosing `OpenEpoch` commits it together with its replay record and event. Refusal
/// writes nothing the caller may commit.
///
/// # Errors
/// `F08_STALE_STATE`, `F08_MARKET_PAUSED`, `WRONG_PHASE` for a closing market, a height
/// outside the Work window or a non-terminal previous epoch, `WRONG_EPOCH` before origin
/// or for a non-advancing epoch, `ACCOUNT_BINDING`, `F08_RETENTION_BLOCKED`,
/// `NON_CANONICAL` for inconsistent F02/F08 records, `CAPACITY` for short buffers or an
/// oversized state, `ARITHMETIC` and every section decoding error.
pub fn rollover_roster(
    current: &[u8],
    height: u64,
    expected_revision: u64,
    next: &mut [u8],
    scratch: &mut [u8],
) -> CodecResult<RosterOpened> {
    Ok(rollover_roster_with(current, height, expected_revision, next, scratch)?.0)
}

/// [`rollover_roster`] that also returns the joint F05/F06 section of `current`, which the
/// next value carries byte for byte, and its validated reward state when initialized.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn rollover_roster_with<'c>(
    current: &'c [u8],
    height: u64,
    expected_revision: u64,
    next: &mut [u8],
    scratch: &mut [u8],
) -> CodecResult<(RosterOpened, &'c [u8], Option<RewardState<'c>>)> {
    let state = state::decode_shared_state(current)?;
    rollover_roster_in_state(&state, height, expected_revision, next, scratch)
}

pub(crate) fn rollover_roster_in_state<'c>(
    state: &SharedState<'c>,
    height: u64,
    expected_revision: u64,
    next: &mut [u8],
    scratch: &mut [u8],
) -> CodecResult<(RosterOpened, &'c [u8], Option<RewardState<'c>>)> {
    let (mut roster, opened, rewards, _, _, control_out) =
        rollover_roster_parts(state, height, expected_revision, scratch)?;
    roster.state_len = state::encode_shared_state(&opened, next, control_out)?;
    Ok((
        roster,
        *state
            .feature_sections
            .guest_index(Section::SettlementClaims.index()),
        rewards,
    ))
}

/// Validated rollover sections, before the enclosing transaction encodes its final state.
pub(crate) fn rollover_roster_parts<'c: 's, 's>(
    state: &SharedState<'c>,
    height: u64,
    expected_revision: u64,
    scratch: &'s mut [u8],
) -> CodecResult<(
    RosterOpened,
    SharedState<'s>,
    Option<RewardState<'c>>,
    AdmissionTable,
    WorkerTable,
    &'s mut [u8],
)> {
    if state.revision != expected_revision {
        return Err(F08_STALE_STATE);
    }
    let market = PolicySection::decode(
        *state
            .feature_sections
            .guest_index(Section::PolicyLifecycle.index()),
    )?
    .header;
    opening_epoch(&market, height)?;
    let joint = *state
        .feature_sections
        .guest_index(Section::ReputationAdmission.index());
    let table = crate::reputation_transition::joint_table(joint)?;
    let admission = AdmissionTable::decode(table)?;
    let epoch = opening_epoch(&market, height)?;
    if admission
        .current_epoch()
        .is_some_and(|previous| epoch <= previous)
    {
        return Err(WRONG_EPOCH);
    }
    let rewards = reward_state(state.room_market()?.0, &market)?;
    rollover_roster_prepared(
        state,
        &market,
        admission,
        rewards,
        height,
        expected_revision,
        scratch,
    )
}

/// Reuses the enclosing opening's validated policy and admission table.
pub(crate) fn rollover_roster_prepared<'c: 's, 's>(
    state: &SharedState<'c>,
    market: &MarketHeader,
    admission: AdmissionTable,
    rewards: Option<RewardState<'c>>,
    height: u64,
    expected_revision: u64,
    scratch: &'s mut [u8],
) -> CodecResult<(
    RosterOpened,
    SharedState<'s>,
    Option<RewardState<'c>>,
    AdmissionTable,
    WorkerTable,
    &'s mut [u8],
)> {
    if state.revision != expected_revision {
        return Err(F08_STALE_STATE);
    }
    let prepared = prepare_roster_sections(
        state.feature_sections,
        market,
        admission,
        rewards,
        height,
        scratch,
        &|slot| Ok(state.control.replay.actor(slot)),
    )?;
    let mut control = state.control.clone();
    for slot in prepared.retired.iter().flatten() {
        control.replay.retire(*slot)?;
    }
    let opened = SharedState {
        revision: state.revision,
        feature_sections: prepared.feature_sections,
        control,
    };
    let summary = prepared.summary.with_len(opened.encoded_len()?)?;
    let workers = WorkerTable::decode(
        split_identity_section(opened.feature_sections[Section::IdentityRoster.index()])?.0,
    )?;
    Ok((
        summary,
        opened,
        rewards,
        prepared.admission,
        workers,
        prepared.control_out,
    ))
}

// Opening keeps replay bytes verbatim unless an actual departing worker changes
// a slot. Retention checks materialize replay only for that affected operation.
pub(crate) fn rollover_roster_framed<'c: 's, 's>(
    reader: &state::SectionReader<'c>,
    market: &MarketHeader,
    admission: AdmissionTable,
    rewards: Option<RewardState<'c>>,
    height: u64,
    scratch: &'s mut [u8],
) -> CodecResult<(
    RosterOpened,
    codec::StateFrame<'s>,
    AdmissionTable,
    crate::workers::OpeningWorkers<'s>,
)> {
    let original = reader.frame();
    let prepared = prepare_roster_sections(
        core::array::from_fn(|i| original.sections[i]),
        market,
        admission,
        rewards,
        height,
        scratch,
        &|slot| reader.with_state(|state| Ok(state.control.replay.actor(slot))),
    )?;
    let control = if prepared.retired.iter().any(Option::is_some) {
        let control = reader.with_state(|state| {
            let mut control = state.control.clone();
            for slot in prepared.retired.iter().flatten() {
                control.replay.retire(*slot)?;
            }
            Ok(control)
        })?;
        let len = state::encode_control(&control, prepared.control_out)?;
        &prepared.control_out[..len]
    } else {
        original.sections[Section::Control.index()]
    };
    let opened = codec::StateFrame {
        revision: original.revision,
        sections: [
            prepared.feature_sections[0],
            prepared.feature_sections[1],
            prepared.feature_sections[2],
            prepared.feature_sections[3],
            prepared.feature_sections[4],
            control,
        ],
    };
    let summary = prepared.summary.with_len(opened.encoded_len()?)?;
    let workers = crate::workers::OpeningWorkers::read(
        split_identity_section(opened.sections[Section::IdentityRoster.index()])?.0,
    )?;
    Ok((summary, opened, prepared.admission, workers))
}

struct RolloverSummary {
    rollover: Rollover,
    previous: Option<u64>,
    skipped: u64,
    preserved_entitlements: u16,
    preserved_amount: Amount,
}
impl RolloverSummary {
    fn with_len(self, state_len: usize) -> CodecResult<RosterOpened> {
        Ok(RosterOpened {
            rollover: self.rollover,
            previous: self.previous,
            skipped: self.skipped,
            workers: self
                .rollover
                .members
                .checked_sub(self.rollover.health.roster_evaluators)
                .ok_or(ARITHMETIC)?,
            evaluators: self.rollover.health.roster_evaluators,
            preserved_entitlements: self.preserved_entitlements,
            preserved_amount: self.preserved_amount,
            state_len,
        })
    }
}
struct PreparedRollover<'s> {
    summary: RolloverSummary,
    feature_sections: [&'s [u8]; 5],
    admission: AdmissionTable,
    retired: [Option<ActorSlot>; crate::MAX_WORKERS],
    control_out: &'s mut [u8],
}

fn prepare_roster_sections<'c: 's, 's>(
    sections: [&'c [u8]; 5],
    market: &MarketHeader,
    mut admission: AdmissionTable,
    rewards: Option<RewardState<'c>>,
    height: u64,
    scratch: &'s mut [u8],
    retained_actor: &dyn Fn(ActorSlot) -> CodecResult<Option<state::ActorReplay>>,
) -> CodecResult<PreparedRollover<'s>> {
    let epoch = opening_epoch(market, height)?;
    let joint = *sections.guest_index(Section::ReputationAdmission.index());
    let table = crate::reputation_transition::framed_admission_table(joint)?;
    let prefix = joint
        .get(..joint.len() - table.len())
        .ok_or(NON_CANONICAL)?;
    let previous = admission.current_epoch();
    let skipped = match previous {
        Some(p) if epoch <= p => return Err(WRONG_EPOCH),
        Some(p) => epoch - p - 1,
        None => epoch,
    };
    let (worker_bytes, region) =
        split_identity_section(*sections.guest_index(Section::IdentityRoster.index()))?;
    let opening_workers = crate::workers::OpeningWorkers::read(worker_bytes)?;
    let before = admission;
    let rollover = {
        let worker_reader = opening_workers.ordered();
        roll(
            &mut admission,
            epoch,
            |m| {
                let Participant::Worker(worker) = m.participant else {
                    return Ok(false);
                };
                let record = worker_reader.get(worker)?.ok_or(NON_CANONICAL)?;
                let slot = ActorSlot::worker(usize::from(record.slot))?;
                Ok(retained_actor(slot)?
                    .and_then(|actor| actor.last)
                    .is_some_and(|last| last.expiry_height > height))
            },
            |m| authorized(&worker_reader, m, height, epoch),
        )?
    };
    let mut retired = [None; crate::MAX_WORKERS];
    let mut retired_count = 0;
    let (mut preserved_entitlements, mut preserved_amount) = (0u16, 0);
    let mut changed_workers: Option<WorkerTable> = None;
    for m in before
        .iter()
        .filter(|m| m.pending_exit.is_some_and(|exit| exit.epoch <= epoch))
    {
        let Participant::Worker(worker) = m.participant else {
            continue;
        };
        if changed_workers.is_none() {
            changed_workers = Some(WorkerTable::decode(worker_bytes)?);
        }
        let record = changed_workers
            .as_mut()
            .ok_or(NON_CANONICAL)?
            .remove(worker)?;
        retired[retired_count] = Some(ActorSlot::worker(usize::from(record.slot))?);
        retired_count += 1;
        if let Some(rewards) = &rewards {
            let (count, amount) = outstanding(rewards, worker)?;
            preserved_entitlements = preserved_entitlements
                .checked_add(count)
                .ok_or(ARITHMETIC)?;
            preserved_amount = Amount::checked_add(preserved_amount, amount).ok_or(ARITHMETIC)?;
        }
    }
    let (table_out, rest) = scratch
        .split_at_mut_checked(Section::ReputationAdmission.payload_cap())
        .ok_or(CAPACITY)?;
    let (identity_out, control_out) = rest
        .split_at_mut_checked(Section::IdentityRoster.payload_cap())
        .ok_or(CAPACITY)?;
    let mut feature_sections = sections;
    let (prefix_out, admission_out) = table_out
        .split_at_mut_checked(prefix.len())
        .ok_or(CAPACITY)?;
    prefix_out.guest_copy_from_slice(prefix);
    let table_len = admission.encode(admission_out)?;
    *feature_sections.guest_index_mut(Section::ReputationAdmission.index()) =
        table_out.guest_index(..prefix.len().checked_add(table_len).ok_or(ARITHMETIC)?);
    if let Some(workers) = changed_workers {
        let workers_len = workers.encode(identity_out)?;
        let identity_len = workers_len.checked_add(region.len()).ok_or(ARITHMETIC)?;
        identity_out
            .get_mut(workers_len..identity_len)
            .ok_or(CAPACITY)?
            .guest_copy_from_slice(region);
        *feature_sections.guest_index_mut(Section::IdentityRoster.index()) =
            identity_out.guest_index(..identity_len);
    }
    Ok(PreparedRollover {
        summary: RolloverSummary {
            rollover,
            previous,
            skipped,
            preserved_entitlements,
            preserved_amount,
        },
        feature_sections,
        admission,
        retired,
        control_out,
    })
}

/// Digest over the frozen roster sorted by role then stable participant ID.
///
/// # Errors
/// Propagates codec and host hash failures.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn roster_digest(table: &AdmissionTable, epoch: u64) -> CodecResult<Digest32> {
    let mut buf = [0u8; 8 + MAX_MEMBERS * ROSTER_MEMBER_BYTES];
    let mut w = Writer::new(&mut buf);
    w.u64(epoch)?;
    for m in table.iter().filter(|m| m.admitted_epoch.is_some()) {
        w.u8(m.participant.role() as u8)?;
        crate::codec::write_fields!(w;
            put: &m.participant.bytes();
            id32: m.owner.as_bytes();
            u64: m.membership_generation;
            u64: m.delegate_generation;
        )?;
    }
    let n = w.len();
    codec::domain_hash("PAXAI/admission-roster/v1", buf.guest_index(..n))
}
