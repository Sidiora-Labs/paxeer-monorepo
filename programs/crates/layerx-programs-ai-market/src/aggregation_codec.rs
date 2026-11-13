//! F05 schema-1 structural codecs. These values confer no admission authority,
//! signature authentication, finality, or economic terminalization. The runtime
//! must obtain the closed authenticated F03/F04 set and preseal revocations from
//! authoritative state before using these commitments. No request flag supplies
//! that provenance. `ReportBody` binds generations through the frozen roster and
//! authenticated evidence admission; it has no invented per-cell generation.
use crate::types::GuestIndex as _;
use crate::{
    codec::{
        decode_report, domain_hash, report_digest, roster_digest, Reader, ReportBody, Roster,
        Writer,
    },
    errors::{
        CodecResult, ARITHMETIC, BAD_VERSION, CAPACITY, CONFLICT, F05_REPORT_INVARIANT,
        NON_CANONICAL, WRONG_CONFIG, WRONG_DOMAIN, WRONG_EPOCH, WRONG_MARKET, WRONG_PHASE,
        WRONG_PROGRAM, WRONG_ROSTER,
    },
    types::{
        Digest32, EpochWindows, EvaluatorId, FrozenBinding, Presence, ReportDigest, RosterDigest,
        Score, Version, WorkerId, WorkerRosterEntry,
    },
    SCHEMA_VERSION,
};

pub const WORKER_AGGREGATE_BYTES: usize = 50;
pub const CURRENT_MAX_BYTES: usize = 2200;
pub const HISTORY_MAX_BYTES: usize = 3586;
pub const F05_MAX_BYTES: usize = 5786;
pub const INPUT_PREIMAGE_MAX_BYTES: usize = 668;
pub const OUTPUT_PREIMAGE_MAX_BYTES: usize = 1788;
pub const MAX_TOTAL_WEIGHT: u64 = 32_000_000;

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum QualityStatus {
    InsufficientQuorum = 0,
    ScoredZero = 1,
    ScoredPositive = 2,
}
impl QualityStatus {
    /// Decodes the wire status byte.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` for a value other than 0, 1 or 2.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(value: u8) -> CodecResult<Self> {
        match value {
            0 => Ok(Self::InsufficientQuorum),
            1 => Ok(Self::ScoredZero),
            2 => Ok(Self::ScoredPositive),
            _ => Err(NON_CANONICAL),
        }
    }
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct WorkerAggregate {
    worker: WorkerId,
    generation: Version,
    support: u8,
    status: QualityStatus,
    score: Score,
    weight: u32,
}
impl WorkerAggregate {
    /// Checks the output representation only; T02 supplies the selected score.
    ///
    /// # Errors
    /// Returns `F03_SCORE_RANGE` when `score` exceeds 1,000,000 and `NON_CANONICAL` when support,
    /// weight and status are inconsistent.
    pub fn new(
        worker: WorkerId,
        generation: Version,
        support: u8,
        status: QualityStatus,
        score: u32,
        weight: u32,
    ) -> CodecResult<Self> {
        let score = Score::new(score)?;
        if support > 8 || weight > 1_000_000 {
            return Err(NON_CANONICAL);
        }
        let valid = match status {
            QualityStatus::InsufficientQuorum => support < 3 && score.get() == 0 && weight == 0,
            QualityStatus::ScoredZero => support >= 3 && score.get() == 0 && weight == 0,
            QualityStatus::ScoredPositive => {
                support >= 3 && score.get() > 0 && weight == score.get()
            }
        };
        if !valid {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            worker,
            generation,
            support,
            status,
            score,
            weight,
        })
    }

    #[must_use]
    pub const fn worker(self) -> WorkerId {
        self.worker
    }
    #[must_use]
    pub const fn generation(self) -> Version {
        self.generation
    }
    #[must_use]
    pub const fn support(self) -> u8 {
        self.support
    }
    #[must_use]
    pub const fn status(self) -> QualityStatus {
        self.status
    }
    #[must_use]
    pub const fn score(self) -> Score {
        self.score
    }
    #[must_use]
    pub const fn weight(self) -> u32 {
        self.weight
    }
    #[must_use]
    pub fn quality(self) -> Presence<Score> {
        if self.status == QualityStatus::InsufficientQuorum {
            Presence::Absent
        } else {
            Presence::Present(self.score)
        }
    }
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn write_worker(w: &mut Writer<'_>, v: WorkerAggregate) -> CodecResult<()> {
    crate::codec::write_fields!(w;
        id32: v.worker.as_bytes();
        u64: v.generation.get();
        u8: v.support;
        u8: v.status as u8;
        u32: v.score.get();
    )?;
    w.u32(v.weight)
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn read_worker(r: &mut Reader<'_>) -> CodecResult<WorkerAggregate> {
    WorkerAggregate::new(
        r.worker_id()?,
        r.version()?,
        r.u8()?,
        QualityStatus::decode(r.u8()?)?,
        r.u32()?,
        r.u32()?,
    )
}
fn capacity(out: &[u8], n: usize, max: usize) -> CodecResult<()> {
    if n > max || out.len() < n {
        Err(CAPACITY)
    } else {
        Ok(())
    }
}
/// Encodes one 50-byte worker aggregate.
///
/// # Errors
/// Returns `CAPACITY` when `out` is shorter than 50 bytes.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_worker_aggregate(v: WorkerAggregate, out: &mut [u8]) -> CodecResult<usize> {
    capacity(out, 50, 50)?;
    let mut w = Writer::new(out);
    write_worker(&mut w, v)?;
    Ok(w.len())
}
/// Strictly decodes one 50-byte worker aggregate.
///
/// # Errors
/// Returns `NON_CANONICAL` for truncated, trailing or invalid fields and `F03_SCORE_RANGE` for
/// an out-of-range score.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_worker_aggregate(bytes: &[u8]) -> CodecResult<WorkerAggregate> {
    let mut r = Reader::new(bytes);
    let v = read_worker(&mut r)?;
    r.finish()?;
    Ok(v)
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn write_binding(w: &mut Writer<'_>, b: FrozenBinding) -> CodecResult<()> {
    w.u16(SCHEMA_VERSION)?;
    crate::codec::write_frozen_binding(w, &b)
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn read_binding(r: &mut Reader<'_>) -> CodecResult<FrozenBinding> {
    if r.u16()? != SCHEMA_VERSION {
        return Err(BAD_VERSION);
    }
    r.frozen_binding()
}
/// Compares every frozen binding field in fixed order.
///
/// # Errors
/// Returns the first of `WRONG_DOMAIN`, `WRONG_PROGRAM`, `WRONG_MARKET`, `WRONG_EPOCH`,
/// `WRONG_CONFIG` or `WRONG_ROSTER` whose field differs.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn check_binding(actual: FrozenBinding, expected: FrozenBinding) -> CodecResult<()> {
    if actual.chain != expected.chain {
        Err(WRONG_DOMAIN)
    } else if actual.program != expected.program {
        Err(WRONG_PROGRAM)
    } else if actual.market != expected.market {
        Err(WRONG_MARKET)
    } else if actual.epoch != expected.epoch {
        Err(WRONG_EPOCH)
    } else if actual.config != expected.config {
        Err(WRONG_CONFIG)
    } else if actual.roster != expected.roster {
        Err(WRONG_ROSTER)
    } else {
        Ok(())
    }
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ReportCommitment {
    pub evaluator: EvaluatorId,
    pub report: ReportDigest,
}
fn check_reports(rows: &[ReportCommitment]) -> CodecResult<()> {
    if rows.len() > 8 {
        return Err(CAPACITY);
    }
    if rows
        .windows(2)
        .any(|p| p.guest_index(0).evaluator >= p.guest_index(1).evaluator)
    {
        return Err(NON_CANONICAL);
    }
    Ok(())
}
/// Arrival order may be normalized here, never by a persisted-state decoder.
/// Duplicate stable identities refuse even when their operational keys differ.
///
/// # Errors
/// Returns `CAPACITY` for more than eight rows and `NON_CANONICAL` for a duplicate evaluator.
pub fn canonicalize_arrivals(rows: &mut [ReportCommitment]) -> CodecResult<()> {
    if rows.len() > 8 {
        return Err(CAPACITY);
    }
    for i in 0..rows.len() {
        if rows
            .guest_index(i + 1..)
            .iter()
            .any(|r| r.evaluator == rows.guest_index(i).evaluator)
        {
            return Err(NON_CANONICAL);
        }
    }
    crate::codec::sort_by(rows, |a, b| a.evaluator.cmp(&b.evaluator));
    Ok(())
}
/// Encodes the canonical aggregation input preimage.
///
/// # Errors
/// Returns `CAPACITY` for more than eight rows or a short `out` and `NON_CANONICAL` for rows
/// not in strictly ascending evaluator order.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_input_preimage(
    binding: FrozenBinding,
    seal_height: u64,
    rows: &[ReportCommitment],
    out: &mut [u8],
) -> CodecResult<usize> {
    check_reports(rows)?;
    capacity(out, 156 + rows.len() * 64, INPUT_PREIMAGE_MAX_BYTES)?;
    let mut w = Writer::new(out);
    write_binding(&mut w, binding)?;
    w.u64(seal_height)?;
    w.u16(u16::try_from(rows.len()).map_err(|_| CAPACITY)?)?;
    for row in rows {
        w.put(row.evaluator.as_bytes())?;
        w.put(row.report.as_bytes())?;
    }
    Ok(w.len())
}
/// Domain-separated digest of the aggregation input preimage.
///
/// # Errors
/// Propagates `encode_input_preimage` refusals.
pub fn input_digest(
    binding: FrozenBinding,
    seal_height: u64,
    rows: &[ReportCommitment],
) -> CodecResult<Digest32> {
    let mut bytes = [0; INPUT_PREIMAGE_MAX_BYTES];
    let n = encode_input_preimage(binding, seal_height, rows, &mut bytes)?;
    domain_hash("PAXAI/aggregation-input/v1", bytes.guest_index(..n))
}

/// Hash the canonical sealed bytes without reconstructing the same report
/// commitments into another table. Preserve identity and strict-order checks.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn encoded_input_digest(
    binding: FrozenBinding,
    seal_height: u64,
    rows: &[u8],
) -> CodecResult<Digest32> {
    let (cells, remainder) = rows.as_chunks::<64>();
    if !remainder.is_empty() {
        return Err(NON_CANONICAL);
    }
    if cells.len() > 8 {
        return Err(CAPACITY);
    }
    let mut previous = None;
    for cell in cells {
        let evaluator = crate::types::CheckedIdentityBytes::new_inline(
            cell[..32].try_into().map_err(|_| NON_CANONICAL)?,
        )?;
        crate::types::CheckedIdentityBytes::new_inline(
            cell[32..].try_into().map_err(|_| NON_CANONICAL)?,
        )?;
        if previous.is_some_and(|id| {
            crate::types::identity_order(id, evaluator.bytes()) != core::cmp::Ordering::Less
        }) {
            return Err(NON_CANONICAL);
        }
        previous = Some(evaluator.bytes());
    }
    let mut bytes = [0; INPUT_PREIMAGE_MAX_BYTES];
    let mut w = Writer::new(&mut bytes);
    write_binding(&mut w, binding)?;
    w.u64(seal_height)?;
    w.u16(u16::try_from(cells.len()).map_err(|_| CAPACITY)?)?;
    w.put(rows)?;
    let n = w.len();
    domain_hash("PAXAI/aggregation-input/v1", bytes.guest_index(..n))
}

/// Structurally validated actual producer reports. This is deliberately NOT an
/// authenticated accepted/revealed set. Runtime must separately prove signatures,
/// commitment opening, evidence/generation admission, deadlines and revocation.
#[derive(Clone, Copy)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct AggregationInputView<'a> {
    binding: FrozenBinding,
    roster: Roster<'a>,
    reports: &'a [ReportBody<'a>],
}
impl<'a> AggregationInputView<'a> {
    /// Consume reports already admitted against this captured opening. Their
    /// vectors were checked by admission; aggregation reads votes for its chunk.
    pub(crate) fn from_committed(
        captured: &'a crate::epoch::CapturedRoster,
        reports: &'a [ReportBody<'a>],
    ) -> CodecResult<Self> {
        if reports.len() > crate::MAX_EVALUATORS {
            return Err(CAPACITY);
        }
        let binding = captured.binding;
        let roster = captured.view();
        let mut previous = None;
        let mut evaluators = roster.evaluators.iter();
        for report in reports {
            check_binding(report.binding.frozen, binding)?;
            let evaluator = loop {
                let entry = evaluators.next().ok_or(F05_REPORT_INVARIANT)?;
                if entry.evaluator >= report.binding.evaluator {
                    break entry;
                }
            };
            if evaluator.evaluator != report.binding.evaluator
                || previous.is_some_and(|id| id >= evaluator.evaluator)
                || evaluator.grant != report.binding.grant
                || evaluator.key_version != report.binding.key_version
            {
                return Err(F05_REPORT_INVARIANT);
            }
            previous = Some(evaluator.evaluator);
        }
        Ok(Self {
            binding,
            roster,
            reports,
        })
    }

    /// Validates reports structurally against the frozen roster and binding.
    ///
    /// # Errors
    /// Propagates roster `CAPACITY`/`NON_CANONICAL` and `check_binding` refusals; returns
    /// `WRONG_MARKET`, `WRONG_EPOCH`, `WRONG_CONFIG` or `WRONG_ROSTER` for a roster/binding
    /// mismatch, `CAPACITY` for more than eight reports and `F05_REPORT_INVARIANT` for an invalid,
    /// unordered, unrostered or self-scoring report.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn structural(
        binding: FrozenBinding,
        roster: Roster<'a>,
        reports: &'a [ReportBody<'a>],
    ) -> CodecResult<Self> {
        roster.validate()?;
        if roster.market != binding.market {
            return Err(WRONG_MARKET);
        }
        if roster.epoch != binding.epoch {
            return Err(WRONG_EPOCH);
        }
        if roster.config != binding.config {
            return Err(WRONG_CONFIG);
        }
        if roster_digest(&roster)? != binding.roster {
            return Err(WRONG_ROSTER);
        }
        if reports.len() > 8 {
            return Err(CAPACITY);
        }
        let mut last = None;
        for report in reports {
            check_binding(report.binding.frozen, binding)?;
            report.scores.validate().map_err(|_| F05_REPORT_INVARIANT)?;
            let id = report.binding.evaluator;
            if last.is_some_and(|p| p >= id) {
                return Err(F05_REPORT_INVARIANT);
            }
            last = Some(id);
            let evaluator = roster
                .evaluators
                .iter()
                .find(|e| e.evaluator == id)
                .ok_or(F05_REPORT_INVARIANT)?;
            if evaluator.grant != report.binding.grant
                || evaluator.key_version != report.binding.key_version
            {
                return Err(F05_REPORT_INVARIANT);
            }
            for cell in report.scores.entries() {
                let cell = cell.map_err(|_| F05_REPORT_INVARIANT)?;
                let index = crate::codec::frozen_worker_index(roster.workers, cell.worker)
                    .ok_or(F05_REPORT_INVARIANT)?;
                let worker = roster.workers.guest_index(index);
                if worker.owner == evaluator.owner || worker.worker.bytes() == id.bytes() {
                    return Err(F05_REPORT_INVARIANT);
                }
            }
        }
        Ok(Self {
            binding,
            roster,
            reports,
        })
    }
    #[must_use]
    pub const fn binding(&self) -> FrozenBinding {
        self.binding
    }
    #[must_use]
    pub fn workers(&self) -> &'a [WorkerRosterEntry] {
        self.roster.workers
    }
    #[must_use]
    pub fn reports(&self) -> &'a [ReportBody<'a>] {
        self.reports
    }
    /// Writes one commitment per report into `out`.
    ///
    /// # Errors
    /// Returns `CAPACITY` when `out` is shorter than the report list; propagates `report_digest`
    /// refusals.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn commitments(&self, out: &mut [ReportCommitment]) -> CodecResult<usize> {
        if out.len() < self.reports.len() {
            return Err(CAPACITY);
        }
        for (slot, report) in out.iter_mut().zip(self.reports) {
            *slot = ReportCommitment {
                evaluator: report.binding.evaluator,
                report: report_digest(report)?,
            };
        }
        Ok(self.reports.len())
    }
    /// Persisted bytes must be decoded using the real common report codec.
    ///
    /// # Errors
    /// Returns `F05_REPORT_INVARIANT` when the bytes are not a canonical report.
    pub fn persisted_report(bytes: &'a [u8]) -> CodecResult<ReportBody<'a>> {
        decode_report(bytes).map_err(|_| F05_REPORT_INVARIANT)
    }
    /// Counts report cells that score `worker`.
    ///
    /// # Errors
    /// Returns `F05_REPORT_INVARIANT` for a worker outside the roster and `ARITHMETIC` on count
    /// overflow; propagates score-cell decode refusals.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn support(&self, worker: WorkerId) -> CodecResult<u8> {
        if !self.roster.workers.iter().any(|w| w.worker == worker) {
            return Err(F05_REPORT_INVARIANT);
        }
        let mut count = 0u8;
        for report in self.reports {
            if score_cell(report.scores, worker)?.is_some() {
                count = count.checked_add(1).ok_or(ARITHMETIC)?;
            }
        }
        Ok(count)
    }
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn check_outputs(
    workers: &[WorkerRosterEntry],
    outputs: &[WorkerAggregate],
    complete: bool,
) -> CodecResult<u64> {
    if workers.len() > 32 || outputs.len() > 32 {
        return Err(CAPACITY);
    }
    if outputs.len() > workers.len() || (complete && outputs.len() != workers.len()) {
        return Err(NON_CANONICAL);
    }
    if workers
        .windows(2)
        .any(|p| p.guest_index(0).worker >= p.guest_index(1).worker)
    {
        return Err(NON_CANONICAL);
    }
    let mut sum = 0u64;
    for (worker, output) in workers.iter().zip(outputs) {
        if worker.worker != output.worker || worker.generation != output.generation {
            return Err(F05_REPORT_INVARIANT);
        }
        sum = sum
            .checked_add(u64::from(output.weight))
            .ok_or(ARITHMETIC)?;
    }
    if sum > MAX_TOTAL_WEIGHT {
        return Err(ARITHMETIC);
    }
    Ok(sum)
}
/// Immutable canonical output commitment; numerical correctness is supplied by
/// T02 and terminal provenance by the atomic T03/F06 transition, not this codec.
#[derive(Clone, Copy)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct EpochAggregation<'a> {
    binding: FrozenBinding,
    input: Digest32,
    outputs: &'a [WorkerAggregate],
    total: u64,
    root: Digest32,
}
impl<'a> EpochAggregation<'a> {
    /// Checks outputs against the frozen roster and computes the output root.
    ///
    /// # Errors
    /// Propagates `check_outputs` refusals (`CAPACITY`, `NON_CANONICAL`, `F05_REPORT_INVARIANT`,
    /// `ARITHMETIC`) and preimage `CAPACITY`.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn structural(
        binding: FrozenBinding,
        input: Digest32,
        workers: &[WorkerRosterEntry],
        outputs: &'a [WorkerAggregate],
    ) -> CodecResult<Self> {
        let total = check_outputs(workers, outputs, true)?;
        let mut bytes = [0; OUTPUT_PREIMAGE_MAX_BYTES];
        let n = write_output_preimage(binding, input, outputs, total, &mut bytes)?;
        let root = domain_hash("PAXAI/aggregation-output/v1", bytes.guest_index(..n))?;
        Ok(Self {
            binding,
            input,
            outputs,
            total,
            root,
        })
    }
    #[must_use]
    pub const fn root(&self) -> Digest32 {
        self.root
    }
    #[must_use]
    pub const fn total_weight(&self) -> u64 {
        self.total
    }
    #[must_use]
    pub fn outputs(&self) -> &'a [WorkerAggregate] {
        self.outputs
    }
    /// Writes the exact output commitment preimage.
    ///
    /// # Errors
    /// Returns `CAPACITY` when `out` is too short.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode_preimage(&self, out: &mut [u8]) -> CodecResult<usize> {
        write_output_preimage(self.binding, self.input, self.outputs, self.total, out)
    }
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn write_output_preimage(
    binding: FrozenBinding,
    input: Digest32,
    outputs: &[WorkerAggregate],
    total: u64,
    out: &mut [u8],
) -> CodecResult<usize> {
    capacity(out, 188 + outputs.len() * 50, OUTPUT_PREIMAGE_MAX_BYTES)?;
    let mut w = Writer::new(out);
    write_binding(&mut w, binding)?;
    w.put(input.as_bytes())?;
    w.u16(u16::try_from(outputs.len()).map_err(|_| CAPACITY)?)?;
    for output in outputs {
        write_worker(&mut w, *output)?;
    }
    w.u64(total)?;
    Ok(w.len())
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum AggregationPhase {
    Unsealed = 0,
    Processing = 1,
    Terminal = 2,
}
#[derive(Clone, Copy)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct CurrentAggregation<'a> {
    pub phase: AggregationPhase,
    pub seal_height: u64,
    pub input: Presence<Digest32>,
    pub cursor: u16,
    pub reports: &'a [ReportCommitment],
    pub outputs: &'a [WorkerAggregate],
    pub running_weight: u64,
    pub root: Presence<Digest32>,
}
impl CurrentAggregation<'_> {
    /// Structural persisted-state validation, never a terminal authority token.
    ///
    /// # Errors
    /// Propagates `check_reports`/`check_outputs` refusals; returns `NON_CANONICAL` for an
    /// inconsistent cursor, sum, phase field or root presence, `F05_REPORT_INVARIANT` when support
    /// exceeds the report count and `CONFLICT` when the input digest or terminal root differs.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn validate(
        &self,
        binding: FrozenBinding,
        workers: &[WorkerRosterEntry],
    ) -> CodecResult<()> {
        check_reports(self.reports)?;
        let sum = check_outputs(workers, self.outputs, false)?;
        if usize::from(self.cursor) != self.outputs.len() || sum != self.running_weight {
            return Err(NON_CANONICAL);
        }
        if self
            .outputs
            .iter()
            .any(|v| usize::from(v.support) > self.reports.len())
        {
            return Err(F05_REPORT_INVARIANT);
        }
        match self.phase {
            AggregationPhase::Unsealed => {
                if self.seal_height != 0
                    || self.input != Presence::Absent
                    || self.cursor != 0
                    || !self.reports.is_empty()
                    || !self.outputs.is_empty()
                    || self.running_weight != 0
                    || self.root != Presence::Absent
                {
                    return Err(NON_CANONICAL);
                }
            }
            AggregationPhase::Processing | AggregationPhase::Terminal => {
                if self.input
                    != Presence::Present(input_digest(binding, self.seal_height, self.reports)?)
                {
                    return Err(CONFLICT);
                }
                match self.phase {
                    AggregationPhase::Processing if self.root != Presence::Absent => {
                        return Err(NON_CANONICAL);
                    }
                    AggregationPhase::Terminal => {
                        let Presence::Present(input) = self.input else {
                            return Err(NON_CANONICAL);
                        };
                        let epoch =
                            EpochAggregation::structural(binding, input, workers, self.outputs)?;
                        if self.root != Presence::Present(epoch.root()) {
                            return Err(CONFLICT);
                        }
                    }
                    _ => {}
                }
            }
        }
        Ok(())
    }
    /// Checks frozen sealed digests and exact support, without live revocation
    /// re-evaluation. The caller must authenticate the producer source separately.
    ///
    /// # Errors
    /// Propagates `validate` and `report_digest` refusals; returns `F05_REPORT_INVARIANT` when
    /// reports, digests or support differ from the view.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn validate_inputs(&self, view: &AggregationInputView<'_>) -> CodecResult<()> {
        self.validate(view.binding, view.workers())?;
        if self.phase == AggregationPhase::Unsealed {
            return Ok(());
        }
        if self.reports.len() != view.reports.len() {
            return Err(F05_REPORT_INVARIANT);
        }
        for (row, report) in self.reports.iter().zip(view.reports) {
            if row.evaluator != report.binding.evaluator || row.report != report_digest(report)? {
                return Err(F05_REPORT_INVARIANT);
            }
        }
        for output in self.outputs {
            if output.support != view.support(output.worker)? {
                return Err(F05_REPORT_INVARIANT);
            }
        }
        Ok(())
    }
}
/// Validates then encodes the current aggregation record.
///
/// # Errors
/// Propagates `validate` refusals; returns `CAPACITY` for a count above the wire bound or a short
/// `out`.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_current(
    v: &CurrentAggregation<'_>,
    binding: FrozenBinding,
    workers: &[WorkerRosterEntry],
    out: &mut [u8],
) -> CodecResult<usize> {
    v.validate(binding, workers)?;
    encode_checked_current(v, out)
}

// The private checked wrapper carries the validation across encoding and history
// checks of the same immutable record, so neither hashes its outputs again.
pub(crate) struct CheckedCurrent<'a> {
    value: CurrentAggregation<'a>,
    binding: FrozenBinding,
}
impl<'a> CheckedCurrent<'a> {
    pub(crate) fn begin(
        binding: FrozenBinding,
        height: u64,
        reports: &'a [ReportCommitment],
    ) -> CodecResult<Self> {
        let input = input_digest(binding, height, reports)?;
        Ok(Self {
            value: CurrentAggregation {
                phase: AggregationPhase::Processing,
                seal_height: height,
                input: Presence::Present(input),
                cursor: 0,
                reports,
                outputs: &[],
                running_weight: 0,
                root: Presence::Absent,
            },
            binding,
        })
    }
    pub(crate) fn input(&self) -> Presence<Digest32> {
        self.value.input
    }
    /// Terminal record from the decoded running section and the very same
    /// checked output projection used for allocation and reputation completion.
    pub(crate) fn terminal(
        current: &DecodedCurrent,
        reports: &'a [ReportCommitment],
        aggregation: &EpochAggregation<'a>,
    ) -> CodecResult<Self> {
        if current.phase != AggregationPhase::Processing
            || current.input != Presence::Present(aggregation.input)
            || usize::from(current.cursor) != aggregation.outputs.len()
            || current.running_weight != aggregation.total
        {
            return Err(F05_REPORT_INVARIANT);
        }
        current.check_committed_reports(reports)?;
        Ok(Self {
            value: CurrentAggregation {
                phase: AggregationPhase::Terminal,
                seal_height: current.seal_height,
                input: current.input,
                cursor: current.cursor,
                reports,
                outputs: aggregation.outputs,
                running_weight: aggregation.total,
                root: Presence::Present(aggregation.root),
            },
            binding: aggregation.binding,
        })
    }

    pub(crate) fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        encode_checked_current(&self.value, out)
    }
    pub(crate) fn check_history(&self, history: &[HistorySummary]) -> CodecResult<()> {
        check_current_history(&self.value, self.binding, history)
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn encode_checked_current(v: &CurrentAggregation<'_>, out: &mut [u8]) -> CodecResult<usize> {
    let n = 56
        + v.reports.len() * 64
        + v.outputs.len() * 50
        + if matches!(v.root, Presence::Present(_)) {
            32
        } else {
            0
        };
    capacity(out, n, CURRENT_MAX_BYTES)?;
    let mut w = Writer::new(out);
    w.u8(v.phase as u8)?;
    w.u64(v.seal_height)?;
    match v.input {
        Presence::Absent => w.put(&[0; 32])?,
        Presence::Present(d) => w.put(d.as_bytes())?,
    }
    w.u16(v.cursor)?;
    w.u16(u16::try_from(v.reports.len()).map_err(|_| CAPACITY)?)?;
    for row in v.reports {
        w.put(row.evaluator.as_bytes())?;
        w.put(row.report.as_bytes())?;
    }
    w.u16(u16::try_from(v.outputs.len()).map_err(|_| CAPACITY)?)?;
    for output in v.outputs {
        write_worker(&mut w, *output)?;
    }
    w.u64(v.running_weight)?;
    w.presence(&v.root, |w, d| w.put(d.as_bytes()))?;
    Ok(w.len())
}
/// Owns bounded decoded storage; borrowed views cannot outlive this record.
#[derive(Clone)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct DecodedCurrent {
    phase: AggregationPhase,
    seal_height: u64,
    input: Presence<Digest32>,
    cursor: u16,
    reports: [Option<ReportCommitment>; 8],
    report_count: usize,
    outputs: [Option<WorkerAggregate>; 32],
    output_count: usize,
    running_weight: u64,
    root: Presence<Digest32>,
}
impl DecodedCurrent {
    /// The record decoder already checked its input digest and persisted output
    /// bindings. Compare sealed receipts without re-hashing immutable report bodies.
    pub(crate) fn check_committed_reports(&self, reports: &[ReportCommitment]) -> CodecResult<()> {
        if reports.len() != self.report_count {
            return Err(F05_REPORT_INVARIANT);
        }
        for (index, report) in reports.iter().enumerate() {
            if self.report(index)? != *report {
                return Err(F05_REPORT_INVARIANT);
            }
        }
        Ok(())
    }

    #[must_use]
    pub fn phase(&self) -> AggregationPhase {
        self.phase
    }
    #[must_use]
    pub fn cursor(&self) -> u16 {
        self.cursor
    }
    /// Decoded report commitment at `index`.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when `index` holds no row.
    pub fn report(&self, index: usize) -> CodecResult<ReportCommitment> {
        self.reports
            .get(index)
            .copied()
            .flatten()
            .ok_or(NON_CANONICAL)
    }
    /// Decoded output at `index`.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when `index` holds no output.
    pub fn output(&self, index: usize) -> CodecResult<WorkerAggregate> {
        self.outputs
            .get(index)
            .copied()
            .flatten()
            .ok_or(NON_CANONICAL)
    }
    /// Re-encodes the record, revalidating it against the frozen binding.
    ///
    /// # Errors
    /// Propagates `encode_current` or `decode_current` refusals; returns `CAPACITY` when `out` is
    /// too short.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(
        &self,
        binding: FrozenBinding,
        workers: &[WorkerRosterEntry],
        out: &mut [u8],
    ) -> CodecResult<usize> {
        if self.report_count == 0 && self.output_count == 0 {
            return encode_current(
                &CurrentAggregation {
                    phase: self.phase,
                    seal_height: self.seal_height,
                    input: self.input,
                    cursor: self.cursor,
                    reports: &[],
                    outputs: &[],
                    running_weight: self.running_weight,
                    root: self.root,
                },
                binding,
                workers,
                out,
            );
        }
        encode_decoded(self, binding, workers, out)
    }
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn encode_decoded(
    v: &DecodedCurrent,
    binding: FrozenBinding,
    workers: &[WorkerRosterEntry],
    out: &mut [u8],
) -> CodecResult<usize> {
    // Reconstruct wire using already checked immutable decoded fields; validation
    // is repeated by decode_current against the supplied frozen binding below.
    let n = 56
        + v.report_count * 64
        + v.output_count * 50
        + if matches!(v.root, Presence::Present(_)) {
            32
        } else {
            0
        };
    let mut scratch = [0; CURRENT_MAX_BYTES];
    let mut w = Writer::new(&mut scratch);
    w.u8(v.phase as u8)?;
    w.u64(v.seal_height)?;
    match v.input {
        Presence::Absent => w.put(&[0; 32])?,
        Presence::Present(d) => w.put(d.as_bytes())?,
    }
    w.u16(v.cursor)?;
    w.u16(u16::try_from(v.report_count).map_err(|_| CAPACITY)?)?;
    for i in 0..v.report_count {
        let row = v.report(i)?;
        w.put(row.evaluator.as_bytes())?;
        w.put(row.report.as_bytes())?;
    }
    w.u16(u16::try_from(v.output_count).map_err(|_| CAPACITY)?)?;
    for i in 0..v.output_count {
        write_worker(&mut w, v.output(i)?)?;
    }
    w.u64(v.running_weight)?;
    w.presence(&v.root, |w, d| w.put(d.as_bytes()))?;
    decode_current(scratch.guest_index(..n), binding, workers)?;
    capacity(out, n, CURRENT_MAX_BYTES)?;
    out.guest_index_mut(..n)
        .guest_copy_from_slice(scratch.guest_index(..n));
    Ok(n)
}
pub(crate) struct CurrentPrefix {
    pub phase: AggregationPhase,
    pub seal_height: u64,
    pub input: Presence<Digest32>,
    pub cursor: u16,
    pub report_count: usize,
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn read_current_prefix(r: &mut Reader<'_>) -> CodecResult<CurrentPrefix> {
    let phase = match r.u8()? {
        0 => AggregationPhase::Unsealed,
        1 => AggregationPhase::Processing,
        2 => AggregationPhase::Terminal,
        _ => return Err(NON_CANONICAL),
    };
    let seal_height = r.u64()?;
    let raw = r.fixed()?;
    let input = if raw == [0; 32] {
        Presence::Absent
    } else {
        Presence::Present(Digest32::new(raw)?)
    };
    let cursor = r.u16()?;
    let report_count = usize::from(r.u16()?);
    if report_count > crate::aggregation::MAX_VOTES {
        return Err(CAPACITY);
    }
    Ok(CurrentPrefix {
        phase,
        seal_height,
        input,
        cursor,
        report_count,
    })
}

/// Strictly decodes and validates the current aggregation record.
///
/// # Errors
/// Returns `CAPACITY` for oversized input or counts, `NON_CANONICAL` for malformed, unordered or
/// inconsistent fields, `F03_SCORE_RANGE` for an out-of-range score, `F05_REPORT_INVARIANT` for
/// an output not matching the roster, `ARITHMETIC` on weight overflow and `CONFLICT` when the
/// input digest or terminal root differs.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_current(
    bytes: &[u8],
    binding: FrozenBinding,
    workers: &[WorkerRosterEntry],
) -> CodecResult<DecodedCurrent> {
    if bytes.len() > CURRENT_MAX_BYTES {
        return Err(CAPACITY);
    }
    let mut r = Reader::new(bytes);
    let CurrentPrefix {
        phase,
        seal_height,
        input,
        cursor,
        report_count,
    } = read_current_prefix(&mut r)?;
    let mut reports = [None; 8];
    let mut last = None;
    for slot in reports.guest_index_mut(..report_count) {
        let row = ReportCommitment {
            evaluator: r.evaluator_id()?,
            report: r.report_digest()?,
        };
        if last.is_some_and(|p| p >= row.evaluator) {
            return Err(NON_CANONICAL);
        }
        last = Some(row.evaluator);
        *slot = Some(row);
    }
    let output_count = usize::from(r.u16()?);
    if output_count > 32 || workers.len() > 32 {
        return Err(CAPACITY);
    }
    if output_count != usize::from(cursor) || output_count > workers.len() {
        return Err(NON_CANONICAL);
    }
    let mut outputs = [None; 32];
    let mut sum = 0u64;
    if workers
        .windows(2)
        .any(|p| p.guest_index(0).worker >= p.guest_index(1).worker)
    {
        return Err(NON_CANONICAL);
    }
    for (i, slot) in outputs
        .guest_index_mut(..output_count)
        .iter_mut()
        .enumerate()
    {
        let output = read_worker(&mut r)?;
        if output.worker != workers.guest_index(i).worker
            || output.generation != workers.guest_index(i).generation
            || usize::from(output.support) > report_count
        {
            return Err(F05_REPORT_INVARIANT);
        }
        sum = sum
            .checked_add(u64::from(output.weight))
            .ok_or(ARITHMETIC)?;
        *slot = Some(output);
    }
    let running_weight = r.u64()?;
    let root = r.presence(super::codec::Reader::digest32)?;
    r.finish()?;
    if sum != running_weight || sum > MAX_TOTAL_WEIGHT {
        return Err(NON_CANONICAL);
    }
    let decoded = DecodedCurrent {
        phase,
        seal_height,
        input,
        cursor,
        reports,
        report_count,
        outputs,
        output_count,
        running_weight,
        root,
    };
    check_decoded_phase(&decoded, binding, workers)?;
    Ok(decoded)
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn check_decoded_phase(
    v: &DecodedCurrent,
    binding: FrozenBinding,
    workers: &[WorkerRosterEntry],
) -> CodecResult<()> {
    if v.phase == AggregationPhase::Unsealed {
        if v.seal_height != 0
            || v.input != Presence::Absent
            || v.cursor != 0
            || v.report_count != 0
            || v.output_count != 0
            || v.running_weight != 0
            || v.root != Presence::Absent
        {
            return Err(NON_CANONICAL);
        }
    } else {
        let mut preimage = [0; INPUT_PREIMAGE_MAX_BYTES];
        let mut w = Writer::new(&mut preimage);
        write_binding(&mut w, binding)?;
        w.u64(v.seal_height)?;
        w.u16(u16::try_from(v.report_count).map_err(|_| CAPACITY)?)?;
        for row in v.reports.guest_index(..v.report_count).iter().flatten() {
            w.put(row.evaluator.as_bytes())?;
            w.put(row.report.as_bytes())?;
        }
        let n = w.len();
        if v.input
            != Presence::Present(domain_hash(
                "PAXAI/aggregation-input/v1",
                preimage.guest_index(..n),
            )?)
        {
            return Err(CONFLICT);
        }
        if v.phase == AggregationPhase::Processing && v.root != Presence::Absent {
            return Err(NON_CANONICAL);
        }
        if v.phase == AggregationPhase::Terminal {
            if v.output_count != workers.len() {
                return Err(NON_CANONICAL);
            }
            let Presence::Present(digest) = v.input else {
                return Err(NON_CANONICAL);
            };
            let mut preimage = [0; OUTPUT_PREIMAGE_MAX_BYTES];
            let mut w = Writer::new(&mut preimage);
            write_binding(&mut w, binding)?;
            w.put(digest.as_bytes())?;
            w.u16(u16::try_from(v.output_count).map_err(|_| CAPACITY)?)?;
            for output in v.outputs.guest_index(..v.output_count).iter().flatten() {
                write_worker(&mut w, *output)?;
            }
            w.u64(v.running_weight)?;
            let n = w.len();
            if v.root
                != Presence::Present(domain_hash(
                    "PAXAI/aggregation-output/v1",
                    preimage.guest_index(..n),
                )?)
            {
                return Err(CONFLICT);
            }
        }
    }
    Ok(())
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct HistorySummary {
    pub epoch: u64,
    pub config: Version,
    pub roster: RosterDigest,
    pub input: Digest32,
    pub root: Digest32,
}
fn check_history(history: &[HistorySummary]) -> CodecResult<()> {
    if history.len() > 32 {
        return Err(CAPACITY);
    }
    if history
        .windows(2)
        .any(|p| p.guest_index(0).epoch >= p.guest_index(1).epoch)
    {
        return Err(NON_CANONICAL);
    }
    Ok(())
}
/// Encodes the bounded epoch history.
///
/// # Errors
/// Returns `CAPACITY` for more than 32 rows or a short `out` and `NON_CANONICAL` for epochs not
/// strictly ascending.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_history(history: &[HistorySummary], out: &mut [u8]) -> CodecResult<usize> {
    check_history(history)?;
    capacity(out, 2 + history.len() * 112, HISTORY_MAX_BYTES)?;
    let mut w = Writer::new(out);
    w.u16(u16::try_from(history.len()).map_err(|_| CAPACITY)?)?;
    for row in history {
        crate::codec::write_fields!(w;
            u64: row.epoch;
            u64: row.config.get();
            id32: row.roster.as_bytes();
            id32: row.input.as_bytes();
            id32: row.root.as_bytes();
        )?;
    }
    Ok(w.len())
}
#[derive(Clone)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct DecodedHistory {
    entries: [Option<HistorySummary>; 32],
    count: usize,
}
impl DecodedHistory {
    #[must_use]
    pub fn len(&self) -> usize {
        self.count
    }
    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.count == 0
    }
    /// History row at `index`.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when `index` holds no row.
    pub fn entry(&self, index: usize) -> CodecResult<HistorySummary> {
        self.entries
            .get(index)
            .copied()
            .flatten()
            .ok_or(NON_CANONICAL)
    }
}
/// Strictly decodes the bounded epoch history.
///
/// # Errors
/// Returns `CAPACITY` for oversized input or more than 32 rows and `NON_CANONICAL` for
/// malformed, zero or unordered fields.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_history(bytes: &[u8]) -> CodecResult<DecodedHistory> {
    if bytes.len() > HISTORY_MAX_BYTES {
        return Err(CAPACITY);
    }
    let mut r = Reader::new(bytes);
    let count = usize::from(r.u16()?);
    if count > 32 {
        return Err(CAPACITY);
    }
    let mut entries = [None; 32];
    let mut last = None;
    for slot in entries.guest_index_mut(..count) {
        let row = HistorySummary {
            epoch: r.u64()?,
            config: r.version()?,
            roster: r.roster_digest()?,
            input: r.digest32()?,
            root: r.digest32()?,
        };
        if last.is_some_and(|p| p >= row.epoch) {
            return Err(NON_CANONICAL);
        }
        last = Some(row.epoch);
        *slot = Some(row);
    }
    r.finish()?;
    Ok(DecodedHistory { entries, count })
}

/// Strict decoding of the exact output commitment preimage, not a new terminal
/// wire ABI. An externally supplied commitment is checked alongside all bindings.
#[derive(Clone)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct DecodedEpochAggregation {
    binding: FrozenBinding,
    input: Digest32,
    outputs: [Option<WorkerAggregate>; 32],
    count: usize,
    total: u64,
    root: Digest32,
}
impl DecodedEpochAggregation {
    #[must_use]
    pub fn binding(&self) -> FrozenBinding {
        self.binding
    }
    #[must_use]
    pub fn input(&self) -> Digest32 {
        self.input
    }
    #[must_use]
    pub fn root(&self) -> Digest32 {
        self.root
    }
    #[must_use]
    pub fn total_weight(&self) -> u64 {
        self.total
    }
    #[must_use]
    pub fn len(&self) -> usize {
        self.count
    }
    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.count == 0
    }
    /// Decoded output at `index`.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when `index` holds no output.
    pub fn output(&self, index: usize) -> CodecResult<WorkerAggregate> {
        self.outputs
            .get(index)
            .copied()
            .flatten()
            .ok_or(NON_CANONICAL)
    }
}
/// Strictly decodes the output preimage and checks it against `expected_root`.
///
/// # Errors
/// Returns `CAPACITY` for oversized input or counts, `BAD_VERSION` for a wrong schema,
/// `NON_CANONICAL` for malformed, unordered or inconsistent fields, `F03_SCORE_RANGE` for an
/// out-of-range score, `F05_REPORT_INVARIANT` for an output not matching the roster,
/// `ARITHMETIC` on weight overflow and `CONFLICT` when the root differs; propagates
/// `check_binding` refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_epoch_preimage(
    bytes: &[u8],
    expected: FrozenBinding,
    workers: &[WorkerRosterEntry],
    expected_root: Digest32,
) -> CodecResult<DecodedEpochAggregation> {
    if bytes.len() > OUTPUT_PREIMAGE_MAX_BYTES || workers.len() > 32 {
        return Err(CAPACITY);
    }
    if workers
        .windows(2)
        .any(|p| p.guest_index(0).worker >= p.guest_index(1).worker)
    {
        return Err(NON_CANONICAL);
    }
    let mut r = Reader::new(bytes);
    let binding = read_binding(&mut r)?;
    check_binding(binding, expected)?;
    let input = r.digest32()?;
    let count = usize::from(r.u16()?);
    if count > 32 {
        return Err(CAPACITY);
    }
    if count != workers.len() {
        return Err(NON_CANONICAL);
    }
    let mut outputs = [None; 32];
    let mut sum = 0u64;
    for (i, slot) in outputs.guest_index_mut(..count).iter_mut().enumerate() {
        let output = read_worker(&mut r)?;
        if output.worker != workers.guest_index(i).worker
            || output.generation != workers.guest_index(i).generation
        {
            return Err(F05_REPORT_INVARIANT);
        }
        sum = sum
            .checked_add(u64::from(output.weight))
            .ok_or(ARITHMETIC)?;
        *slot = Some(output);
    }
    let total = r.u64()?;
    r.finish()?;
    if total != sum || sum > MAX_TOTAL_WEIGHT {
        return Err(NON_CANONICAL);
    }
    let root = domain_hash("PAXAI/aggregation-output/v1", bytes)?;
    if root != expected_root {
        return Err(CONFLICT);
    }
    Ok(DecodedEpochAggregation {
        binding,
        input,
        outputs,
        count,
        total,
        root,
    })
}
impl CurrentAggregation<'_> {
    /// Caller supplies frozen origin, never wall-clock time. Runtime additionally
    /// checks authenticated execution height at the actual input-seal transition.
    ///
    /// # Errors
    /// Returns `ARITHMETIC` when the epoch windows overflow and `WRONG_PHASE` when a sealed record's
    /// height precedes settlement.
    pub fn validate_seal_window(&self, binding: FrozenBinding, origin: u64) -> CodecResult<()> {
        if self.phase != AggregationPhase::Unsealed
            && self.seal_height < EpochWindows::new(origin, binding.epoch)?.settlement
        {
            return Err(WRONG_PHASE);
        }
        Ok(())
    }
}
/// Cross-record structural consistency. Retention high-watermark, safe pruning,
/// terminal height and economic atomicity remain common/F06-owned state.
///
/// # Errors
/// Propagates `validate` and history `CAPACITY`/`NON_CANONICAL` refusals; returns `WRONG_EPOCH`
/// for a future history row and `F05_REPORT_INVARIANT` when the terminal row is missing or
/// differs, or a non-terminal epoch already has one.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn validate_current_history(
    current: &CurrentAggregation<'_>,
    binding: FrozenBinding,
    workers: &[WorkerRosterEntry],
    history: &[HistorySummary],
) -> CodecResult<()> {
    current.validate(binding, workers)?;
    check_current_history(current, binding, history)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn check_current_history(
    current: &CurrentAggregation<'_>,
    binding: FrozenBinding,
    history: &[HistorySummary],
) -> CodecResult<()> {
    check_history(history)?;
    if history.iter().any(|r| r.epoch > binding.epoch) {
        return Err(WRONG_EPOCH);
    }
    let matching = history.iter().find(|r| r.epoch == binding.epoch);
    if current.phase == AggregationPhase::Terminal {
        let row = matching.ok_or(F05_REPORT_INVARIANT)?;
        if row.config != binding.config
            || row.roster != binding.roster
            || current.input != Presence::Present(row.input)
            || current.root != Presence::Present(row.root)
        {
            return Err(F05_REPORT_INVARIANT);
        }
    } else if matching.is_some() {
        return Err(F05_REPORT_INVARIANT);
    }
    Ok(())
}

#[derive(Clone)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct DecodedInputCommitment {
    binding: FrozenBinding,
    seal_height: u64,
    rows: [Option<ReportCommitment>; 8],
    count: usize,
    digest: Digest32,
}
impl DecodedInputCommitment {
    #[must_use]
    pub fn binding(&self) -> FrozenBinding {
        self.binding
    }
    #[must_use]
    pub fn seal_height(&self) -> u64 {
        self.seal_height
    }
    #[must_use]
    pub fn digest(&self) -> Digest32 {
        self.digest
    }
    #[must_use]
    pub fn len(&self) -> usize {
        self.count
    }
    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.count == 0
    }
    /// Commitment row at `index`.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when `index` holds no row.
    pub fn row(&self, index: usize) -> CodecResult<ReportCommitment> {
        self.rows.get(index).copied().flatten().ok_or(NON_CANONICAL)
    }
}
/// Strictly decodes the input preimage and checks it against `expected_digest`.
///
/// # Errors
/// Returns `CAPACITY` for oversized input or more than eight rows, `BAD_VERSION` for a wrong
/// schema, `NON_CANONICAL` for malformed or unordered fields and `CONFLICT` when the digest
/// differs; propagates `check_binding` refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_input_preimage(
    bytes: &[u8],
    expected: FrozenBinding,
    expected_digest: Digest32,
) -> CodecResult<DecodedInputCommitment> {
    if bytes.len() > INPUT_PREIMAGE_MAX_BYTES {
        return Err(CAPACITY);
    }
    let mut r = Reader::new(bytes);
    let binding = read_binding(&mut r)?;
    check_binding(binding, expected)?;
    let seal_height = r.u64()?;
    let count = usize::from(r.u16()?);
    if count > 8 {
        return Err(CAPACITY);
    }
    let mut rows = [None; 8];
    let mut last = None;
    for slot in rows.guest_index_mut(..count) {
        let row = ReportCommitment {
            evaluator: r.evaluator_id()?,
            report: r.report_digest()?,
        };
        if last.is_some_and(|p| p >= row.evaluator) {
            return Err(NON_CANONICAL);
        }
        last = Some(row.evaluator);
        *slot = Some(row);
    }
    r.finish()?;
    let digest = domain_hash("PAXAI/aggregation-input/v1", bytes)?;
    if digest != expected_digest {
        return Err(CONFLICT);
    }
    Ok(DecodedInputCommitment {
        binding,
        seal_height,
        rows,
        count,
        digest,
    })
}
impl DecodedCurrent {
    #[must_use]
    pub fn seal_height(&self) -> u64 {
        self.seal_height
    }
    #[must_use]
    pub fn input(&self) -> Presence<Digest32> {
        self.input
    }
    #[must_use]
    pub fn root(&self) -> Presence<Digest32> {
        self.root
    }
    #[must_use]
    pub fn running_weight(&self) -> u64 {
        self.running_weight
    }
    #[must_use]
    pub fn report_count(&self) -> usize {
        self.report_count
    }
    #[must_use]
    pub fn output_count(&self) -> usize {
        self.output_count
    }
    /// Use after decoding with the same frozen binding. Does not authenticate the
    /// structural producer view or turn persisted phase2 into economic authority.
    ///
    /// # Errors
    /// Returns `F05_REPORT_INVARIANT` when counts, reports, outputs or support differ from the view,
    /// `NON_CANONICAL` for a missing decoded row and `CONFLICT` when the input digest differs;
    /// propagates `report_digest` refusals.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn validate_inputs(&self, view: &AggregationInputView<'_>) -> CodecResult<()> {
        if self.phase == AggregationPhase::Unsealed {
            return Ok(());
        }
        if self.report_count != view.reports.len() || self.output_count > view.workers().len() {
            return Err(F05_REPORT_INVARIANT);
        }
        let mut preimage = [0; INPUT_PREIMAGE_MAX_BYTES];
        let mut w = Writer::new(&mut preimage);
        write_binding(&mut w, view.binding)?;
        w.u64(self.seal_height)?;
        w.u16(u16::try_from(self.report_count).map_err(|_| CAPACITY)?)?;
        for (i, report) in view.reports.iter().enumerate() {
            let row = self.report(i)?;
            if row.evaluator != report.binding.evaluator || row.report != report_digest(report)? {
                return Err(F05_REPORT_INVARIANT);
            }
            w.put(row.evaluator.as_bytes())?;
            w.put(row.report.as_bytes())?;
        }
        let n = w.len();
        if self.input
            != Presence::Present(domain_hash(
                "PAXAI/aggregation-input/v1",
                preimage.guest_index(..n),
            )?)
        {
            return Err(CONFLICT);
        }
        for i in 0..self.output_count {
            let output = self.output(i)?;
            let worker = *view.workers().guest_index(i);
            if output.worker != worker.worker
                || output.generation != worker.generation
                || output.support != view.support(worker.worker)?
            {
                return Err(F05_REPORT_INVARIANT);
            }
        }
        Ok(())
    }
}

/// Canonical per-worker input projection in ascending stable evaluator identity.
/// Absence creates no vote; an explicit Score(0) remains a real entry.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct EvaluatorVote {
    pub evaluator: EvaluatorId,
    pub score: Score,
}
#[derive(Clone)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct WorkerVotes {
    worker: WorkerId,
    generation: Version,
    entries: [Option<EvaluatorVote>; 8],
    count: usize,
}
impl WorkerVotes {
    #[must_use]
    pub fn worker(&self) -> WorkerId {
        self.worker
    }
    #[must_use]
    pub fn generation(&self) -> Version {
        self.generation
    }
    #[must_use]
    pub fn len(&self) -> usize {
        self.count
    }
    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.count == 0
    }
    /// Vote at `index`.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when `index` holds no vote.
    pub fn vote(&self, index: usize) -> CodecResult<EvaluatorVote> {
        self.entries
            .get(index)
            .copied()
            .flatten()
            .ok_or(NON_CANONICAL)
    }
}
/// A single chunk's votes. Reports are merged as borrowed score slices, so
/// processing never materializes or copies a roster-wide report body array.
/// Evaluator order is checked once at the report boundary; every worker column
/// inherits that order and stores only its scores, including measured zeroes.
pub(crate) struct ChunkVotes<'a> {
    workers: &'a [WorkerRosterEntry],
    columns: [crate::aggregation::ScoreMedian; crate::aggregation::WORKERS_PER_CHUNK],
    previous: Option<&'a [u8; 32]>,
    reports: u8,
    checked_cells: Option<&'a [u8]>,
    checked_scores: [Score; crate::aggregation::WORKERS_PER_CHUNK],
}
impl<'a> ChunkVotes<'a> {
    pub(crate) fn new(workers: &'a [WorkerRosterEntry]) -> CodecResult<Self> {
        if workers.len() > crate::aggregation::WORKERS_PER_CHUNK {
            return Err(CAPACITY);
        }
        Ok(Self {
            workers,
            columns: [crate::aggregation::ScoreMedian::new()?;
                crate::aggregation::WORKERS_PER_CHUNK],
            previous: None,
            reports: 0,
            checked_cells: None,
            checked_scores: [Score::new(0)?; crate::aggregation::WORKERS_PER_CHUNK],
        })
    }

    #[cfg_attr(target_arch = "wasm32", inline(always))]
    pub(crate) fn add_report(
        &mut self,
        evaluator: &'a [u8; 32],
        scores: crate::codec::ScoreVector<'a>,
        start: usize,
        roster_count: usize,
    ) -> CodecResult<()> {
        if self.reports == 8 {
            return Err(CAPACITY);
        }
        if self.previous.is_some_and(|previous| {
            crate::types::identity_order(previous, evaluator) != core::cmp::Ordering::Less
        }) {
            return Err(F05_REPORT_INVARIANT);
        }
        self.previous = Some(evaluator);
        self.reports += 1;
        let workers = self.workers;
        if let crate::codec::ScoreVector::Encoded(bytes) = scores {
            if scores.len() == roster_count {
                let end = start.checked_add(self.workers.len()).ok_or(ARITHMETIC)?;
                let cells = bytes
                    .get(
                        start.checked_mul(36).ok_or(ARITHMETIC)?
                            ..end.checked_mul(36).ok_or(ARITHMETIC)?,
                    )
                    .ok_or(NON_CANONICAL)?;
                // Equality includes every identity and score byte in this chunk.
                // Reuse only cells checked against these same workers; differing
                // bytes always take the complete checked projection below.
                if self
                    .checked_cells
                    .is_some_and(|previous| crate::codec::equal_bytes(previous, cells))
                {
                    for (column, score) in self
                        .columns
                        .iter_mut()
                        .zip(&self.checked_scores)
                        .take(workers.len())
                    {
                        column.push(*score)?;
                    }
                    return Ok(());
                }
                for (at, ((worker, cell), column)) in workers
                    .iter()
                    .zip(cells.as_chunks::<36>().0)
                    .zip(&mut self.columns)
                    .enumerate()
                {
                    let identity = crate::types::CheckedIdentityBytes::new_inline(
                        cell.guest_index(..32)
                            .try_into()
                            .map_err(|_| NON_CANONICAL)?,
                    )?;
                    let score = Score::new(u32::from_be_bytes(
                        cell.guest_index(32..)
                            .try_into()
                            .map_err(|_| NON_CANONICAL)?,
                    ))?;
                    if !crate::types::identity_equal_inline(
                        worker.worker.as_bytes(),
                        identity.bytes(),
                    ) {
                        return Err(F05_REPORT_INVARIANT);
                    }
                    column.push(score)?;
                    *self.checked_scores.get_mut(at).ok_or(CAPACITY)? = score;
                }
                self.checked_cells = Some(cells);
                return Ok(());
            }
        }
        // Admission proves canonical ordering and roster membership. A complete
        // report therefore has the roster's exact offsets; project only this
        // chunk's assignments and still compare each assigned identity.
        for (at, worker) in workers.iter().enumerate() {
            let score = if scores.len() == roster_count {
                let (identity, score) =
                    scores.checked_cell(start.checked_add(at).ok_or(ARITHMETIC)?)?;
                if !crate::types::identity_equal_inline(worker.worker.as_bytes(), identity) {
                    return Err(F05_REPORT_INVARIANT);
                }
                Some(score)
            } else {
                score_cell(scores, worker.worker)?.map(|cell| cell.score)
            };
            if let Some(score) = score {
                self.record_score(at, score)?;
            }
        }
        Ok(())
    }
    #[cfg_attr(target_arch = "wasm32", inline(always))]
    fn record_score(&mut self, at: usize, score: Score) -> CodecResult<()> {
        self.columns.get_mut(at).ok_or(CAPACITY)?.push(score)
    }
    pub(crate) fn worker_median(
        &self,
        index: usize,
    ) -> CodecResult<(WorkerRosterEntry, crate::aggregation::Median)> {
        let worker = *self.workers.get(index).ok_or(F05_REPORT_INVARIANT)?;
        let median = self
            .columns
            .get(index)
            .ok_or(F05_REPORT_INVARIANT)?
            .median()?;
        Ok((worker, median))
    }
}

impl AggregationInputView<'_> {
    /// Merge each canonical report vector with this contiguous frozen chunk.
    /// Every report cell and chunk member is visited at most once per report.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn chunk_votes(&self, start: usize, end: usize) -> CodecResult<ChunkVotes<'_>> {
        let workers = self
            .roster
            .workers
            .get(start..end)
            .ok_or(F05_REPORT_INVARIANT)?;
        if workers.len() > crate::aggregation::WORKERS_PER_CHUNK {
            return Err(CAPACITY);
        }
        let mut votes = ChunkVotes::new(workers)?;
        for report in self.reports {
            votes.add_report(
                report.binding.evaluator.as_bytes(),
                report.scores,
                start,
                self.roster.workers.len(),
            )?;
        }
        Ok(votes)
    }

    /// Canonical vote projection for one frozen worker generation.
    ///
    /// # Errors
    /// Returns `F05_REPORT_INVARIANT` for an unrostered worker or a generation mismatch and
    /// `CAPACITY` for more than eight votes; propagates score-cell decode refusals.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn worker_votes(&self, worker: WorkerId, generation: Version) -> CodecResult<WorkerVotes> {
        let index = crate::codec::frozen_worker_index(self.roster.workers, worker)
            .ok_or(F05_REPORT_INVARIANT)?;
        let frozen = self.roster.workers.guest_index(index);
        if frozen.generation != generation {
            return Err(F05_REPORT_INVARIANT);
        }
        let mut entries = [None; 8];
        let mut count = 0usize;
        for report in self.reports {
            if let Some(cell) = score_cell(report.scores, worker)? {
                if count >= 8 {
                    return Err(CAPACITY);
                }
                entries[count] = Some(EvaluatorVote {
                    evaluator: report.binding.evaluator,
                    score: cell.score,
                });
                count += 1;
            }
        }
        Ok(WorkerVotes {
            worker,
            generation,
            entries,
            count,
        })
    }
}

// Called only through a structurally validated AggregationInputView.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn score_cell(
    scores: crate::codec::ScoreVector<'_>,
    worker: WorkerId,
) -> CodecResult<Option<crate::types::ScoreEntry>> {
    let mut left = 0;
    let mut right = scores.len();
    while left < right {
        let mid = left + (right - left) / 2;
        let (identity, score) = scores.checked_cell(mid)?;
        match crate::types::identity_order(identity, worker.as_bytes()) {
            core::cmp::Ordering::Less => left = mid + 1,
            core::cmp::Ordering::Greater => right = mid,
            core::cmp::Ordering::Equal => {
                return Ok(Some(crate::types::ScoreEntry { worker, score }));
            }
        }
    }
    Ok(None)
}
