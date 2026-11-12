//! F03 atomic report admission (the internal `ValidateReportAdmission` transition) and the
//! permissionless bounded `ChallengeAssessment` (0x0304) over the complete committed shared
//! state value.
//!
//! [`admit_report`] has no public selector: the F04 `RevealScore` operation calls it inside its
//! own transaction after its envelope, authority, window and commitment checks, and emits the
//! 228-byte `RevealScore` event from the returned [`RevealScoreEvent`]. F03 emits no admission
//! event of its own. Every success commits exactly one revision increment with
//! `header.state_revision` equal to the shared revision; every refusal leaves `current`
//! unchanged and the outputs must be discarded.
//!
//! Readings chosen where producers are silent:
//! - The admitted-report region starts the current-reports section: `epoch:u64 || count:u8 ||
//!   count rows` in strictly ascending evaluator order, each row `evaluator32 ||
//!   report_digest32 || admitted_height:u64 || admitted_activity:u64 || length:u16 || canonical
//!   SignedReport`. Later section bytes belong to F04 commit metadata and are carried unchanged;
//!   an F04 writer that precedes any admission writes the empty region `epoch || 0`. Rows of an
//!   earlier epoch read as absent and the first admission of a later opened epoch replaces them.
//!   The presence of an evaluator's row is the consumed (REVEALED) state of its commitment.
//! - The challenge region follows the F09 seal region in the control feature bytes: `epoch:u64
//!   || count:u16 || count 170-byte records` in recording order, then the bytes of later control
//!   producers, carried unchanged. Records of an earlier epoch read as absent.
//! - The envelope request identity of a challenge asserts its challenge identity; a body that
//!   derives another identity refuses `CONFLICT`. A challenge must reference the admitted report
//!   digest of a frozen evaluator of the opened epoch, else `NOT_FOUND`.
//! - The evaluator replay slot is supplied by the reveal caller; no producer binds
//!   `ActorSlot::evaluator(i)` yet (F08 evaluator acceptance must).
use crate::types::GuestIndex as _;
use crate::{
    admission::{AdmissionTable, Participant},
    codec::{self, compare_native_principal, derive_market, EventCommon, Reader, Writer},
    codec::{RevealScoreEvent, ValidatedEnvelope},
    dispatch,
    errors::{
        CodecResult, ARITHMETIC, CAPACITY, CONFLICT, F03_CHALLENGE_CAPACITY,
        F03_REPORT_ALREADY_FINAL, NON_CANONICAL, NOT_FOUND, REVOKED, UNAUTHORIZED,
        UNKNOWN_OPERATION, WRONG_CONFIG, WRONG_EPOCH, WRONG_MARKET, WRONG_ROSTER,
    },
    evaluators::{
        authority::{evaluator_region, EvaluatorRegion, FrozenEvaluator},
        codec::{check_binding, decode_signed_report, encode_signed_report, verify_signed_report},
        model::SIGNED_REPORT_MIN_BYTES,
        model::{ReportContext, SignedReport, VerificationError, SIGNED_REPORT_MAX_BYTES},
    },
    evidence::SealRegion,
    registry_ops::{CallContext, PolicySection},
    rewards::{RewardEpoch, RewardState},
    state::{
        decode_shared_state, settlement_rewards, ReplayDecision, ReplayRequest, RetainedResult,
        Section, SharedState,
    },
    types::{
        Digest32, EvaluatorBinding, EvaluatorId, FrozenBinding, Presence, PrincipalId,
        ReportDigest, ResultDigest, Version, WorkerRosterEntry,
    },
    MAX_EVALUATORS, MAX_WORKERS,
};

/// Fixed bytes of one admitted-report row before its signed report.
pub const REPORT_ROW_FIXED_BYTES: usize = 82;
/// One admitted-report row at its bound.
pub const REPORT_ROW_MAX_BYTES: usize = REPORT_ROW_FIXED_BYTES + SIGNED_REPORT_MAX_BYTES;
const REPORT_REGION_HEADER_BYTES: usize = 9;
/// The admitted-report region at its bound: epoch, count and eight maximum rows.
pub const REPORT_REGION_MAX_BYTES: usize =
    REPORT_REGION_HEADER_BYTES + MAX_EVALUATORS * REPORT_ROW_MAX_BYTES;
/// The F04 reveal result payload: `2 || evaluator32 || report32 || height || activity`.
pub const REVEAL_RESULT_BYTES: usize = 81;
const REVEAL_RESULT_KIND: u8 = 2;
/// Recorded challenge commitments per epoch.
pub const MAX_CHALLENGES: usize = 8;
/// One recorded challenge, also the `ChallengeAssessment` event suffix.
pub const CHALLENGE_RECORD_BYTES: usize = 170;
/// Canonical `ChallengeAssessment` payload length.
pub const CHALLENGE_PAYLOAD_BYTES: usize = 97;
const CHALLENGE_REGION_HEADER_BYTES: usize = 10;
/// The challenge region at its bound: epoch, count and eight records.
pub const CHALLENGE_REGION_MAX_BYTES: usize =
    CHALLENGE_REGION_HEADER_BYTES + MAX_CHALLENGES * CHALLENGE_RECORD_BYTES;
const CHALLENGE_DOMAIN: &str = "PAXAI/evaluator-challenge/v1";
const CHALLENGE_PREIMAGE_BYTES: usize = 233;
const RECORDED: u8 = 1;
const POLICY_CAP: usize = Section::PolicyLifecycle.payload_cap();
const REPORTS_CAP: usize = Section::CurrentReports.payload_cap();
const CONTROL_CAP: usize = Section::Control.payload_cap();
/// Caller scratch for [`admit_report`]: the next F01 section, the next current-reports section
/// and the control encoding.
pub const ADMISSION_SCRATCH_BYTES: usize = POLICY_CAP + REPORTS_CAP + CONTROL_CAP;
/// Caller scratch for [`apply_challenge`]: the next F01 section, the next control feature bytes
/// and the control encoding.
pub const CHALLENGE_SCRATCH_BYTES: usize = POLICY_CAP + 2 * CONTROL_CAP;

/// The immutable admission facts of one evaluator epoch, the reveal receipt binding.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct AdmissionReceipt {
    pub evaluator: EvaluatorId,
    pub report: ReportDigest,
    pub height: u64,
    pub activity: u64,
}
impl AdmissionReceipt {
    /// The 81-byte F04 reveal result payload.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when the payload does not fill exactly 81 bytes.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn payload(&self) -> CodecResult<[u8; REVEAL_RESULT_BYTES]> {
        let mut out = [0; REVEAL_RESULT_BYTES];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w;
            u8: REVEAL_RESULT_KIND;
            id32: self.evaluator.as_bytes();
            id32: self.report.as_bytes();
            u64: self.height;
            u64: self.activity;
        )?;
        if w.len() != REVEAL_RESULT_BYTES {
            return Err(NON_CANONICAL);
        }
        Ok(out)
    }
    /// `H('PAXAI/result/v1', payload)`.
    ///
    /// # Errors
    /// Propagates payload and result-digest refusals.
    pub fn result(&self) -> CodecResult<ResultDigest> {
        codec::result_digest(&self.payload()?)
    }
}

/// One stored admitted row: its receipt and the canonical signed report.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct AdmittedReport<'a> {
    pub receipt: AdmissionReceipt,
    signed: &'a [u8],
}
impl<'a> AdmittedReport<'a> {
    /// The canonical `SignedReport` bytes.
    #[must_use]
    pub const fn signed_bytes(&self) -> &'a [u8] {
        self.signed
    }
    /// The stored signed report.
    ///
    /// # Errors
    /// Propagates `decode_signed_report` refusals.
    pub fn signed(&self) -> CodecResult<SignedReport<'a>> {
        decode_signed_report(self.signed)
    }
    pub(crate) fn committed_body(&self) -> CodecResult<crate::codec::ReportBody<'a>> {
        let end = self.signed.len().checked_sub(64).ok_or(NON_CANONICAL)?;
        crate::codec::decode_report_frame(&self.signed[..end])
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn read(r: &mut Reader<'a>, epoch: u64) -> CodecResult<Self> {
        let row = Self::read_validated(r)?;
        let body = decode_signed_report(row.signed)?.body;
        if body.binding.evaluator != row.receipt.evaluator
            || body.binding.frozen.epoch != epoch
            || codec::report_digest(&body)? != row.receipt.report
        {
            return Err(NON_CANONICAL);
        }
        Ok(row)
    }
    /// Parses only rows whose body and receipt were checked by ReportRegion::decode.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn read_validated(r: &mut Reader<'a>) -> CodecResult<Self> {
        let receipt = crate::codec::read_record!(r, AdmissionReceipt {  }; evaluator = r.evaluator_id()?;
            report: ReportDigest,
            height: U64,
            activity: U64
        );
        let length = usize::from(r.u16()?);
        if !(SIGNED_REPORT_MIN_BYTES..=SIGNED_REPORT_MAX_BYTES).contains(&length) {
            return Err(NON_CANONICAL);
        }
        let signed = r.take(length)?;
        Ok(Self { receipt, signed })
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write(&self, w: &mut Writer<'_>) -> CodecResult<()> {
        crate::codec::write_fields!(w;
            id32: self.receipt.evaluator.as_bytes();
            id32: self.receipt.report.as_bytes();
            u64: self.receipt.height;
            u64: self.receipt.activity;
        )?;
        w.u16(u16::try_from(self.signed.len()).map_err(|_| ARITHMETIC)?)?;
        w.put(self.signed)
    }
}

/// Borrow only the receipt identities and signed bytes needed by a chunk.
/// The fixed receipt's height/activity remain stored verbatim and are not
/// materialized by this read-only projection.
#[derive(Clone, Copy)]
pub(crate) struct ChunkReport<'a> {
    pub evaluator: crate::types::CheckedIdentityBytes<'a>,
    pub report: crate::types::CheckedIdentityBytes<'a>,
    signed: &'a [u8],
}
impl<'a> ChunkReport<'a> {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn read(r: &mut Reader<'a>) -> CodecResult<Self> {
        let header = r.take(82)?;
        let evaluator = crate::types::CheckedIdentityBytes::new_inline(
            header
                .guest_index(..32)
                .try_into()
                .map_err(|_| NON_CANONICAL)?,
        )?;
        let report = crate::types::CheckedIdentityBytes::new_inline(
            header
                .guest_index(32..64)
                .try_into()
                .map_err(|_| NON_CANONICAL)?,
        )?;
        let length = usize::from(u16::from_be_bytes([header[80], header[81]]));
        if !(SIGNED_REPORT_MIN_BYTES..=SIGNED_REPORT_MAX_BYTES).contains(&length) {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            evaluator,
            report,
            signed: r.take(length)?,
        })
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn chunk_scores(
        &self,
        frozen: &crate::types::FrozenBinding,
        encoded_binding: &[u8],
    ) -> CodecResult<crate::codec::ScoreVector<'a>> {
        let end = self.signed.len().checked_sub(64).ok_or(NON_CANONICAL)?;
        let bytes = self.signed.get(..end).ok_or(NON_CANONICAL)?;
        if bytes.len() > crate::codec::REPORT_MAX_BYTES {
            return Err(crate::errors::CAPACITY);
        }
        let schema: [u8; 2] = bytes
            .get(..2)
            .ok_or(NON_CANONICAL)?
            .try_into()
            .map_err(|_| NON_CANONICAL)?;
        if u16::from_be_bytes(schema) != crate::SCHEMA_VERSION {
            return Err(crate::errors::BAD_VERSION);
        }
        let header: &[u8; 228] = bytes
            .get(..228)
            .ok_or(NON_CANONICAL)?
            .try_into()
            .map_err(|_| NON_CANONICAL)?;
        if !crate::codec::equal_bytes(&header[2..146], encoded_binding) {
            let body = crate::codec::decode_report_frame(bytes)?;
            crate::aggregation_codec::check_binding(body.binding.frozen, *frozen)?;
        }
        let evaluator = header[146..178].try_into().map_err(|_| NON_CANONICAL)?;
        if !crate::types::identity_equal_inline(evaluator, self.evaluator.bytes()) {
            return Err(crate::errors::F05_REPORT_INVARIANT);
        }
        // Keep both nonzero version and evidence identity checks even though
        // admission already validated this immutable signed report.
        for version in header[178..194].as_chunks::<8>().0 {
            if u64::from_ne_bytes(*version) == 0 {
                return Err(NON_CANONICAL);
            }
        }
        crate::types::CheckedIdentityBytes::new_inline(
            header[194..226].try_into().map_err(|_| NON_CANONICAL)?,
        )?;
        let count = usize::from(u16::from_be_bytes([header[226], header[227]]));
        if count == 0 {
            return Err(crate::errors::F03_NO_SCORES);
        }
        if count > crate::MAX_WORKERS {
            return Err(crate::errors::CAPACITY);
        }
        let scores = bytes.get(228..).ok_or(NON_CANONICAL)?;
        if scores.len() != count * 36 {
            return Err(NON_CANONICAL);
        }
        Ok(crate::codec::ScoreVector::Encoded(scores))
    }
}
/// Receipt index for one chunk call. Row framing and canonical evaluator order
/// are checked once, including rows excluded by the seal; score cells remain borrowed.
pub(crate) struct ChunkReports<'a> {
    epoch: u64,
    rows: [Option<ChunkReport<'a>>; MAX_EVALUATORS],
}
impl<'a> ChunkReports<'a> {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn committed(section: &'a [u8]) -> CodecResult<Self> {
        let mut index = Self {
            epoch: 0,
            rows: [None; MAX_EVALUATORS],
        };
        if section.is_empty() {
            return Ok(index);
        }
        let mut reader = Reader::new(section);
        index.epoch = reader.u64()?;
        let count = usize::from(reader.u8()?);
        if count > MAX_EVALUATORS {
            return Err(NON_CANONICAL);
        }
        let mut previous = None;
        for slot in index.rows.iter_mut().take(count) {
            let row = ChunkReport::read(&mut reader)?;
            if previous.is_some_and(|previous| {
                crate::types::identity_order(previous, row.evaluator.bytes())
                    != core::cmp::Ordering::Less
            }) {
                return Err(NON_CANONICAL);
            }
            previous = Some(row.evaluator.bytes());
            *slot = Some(row);
        }
        if count == 0 && reader.remaining() == 0 {
            return Err(NON_CANONICAL);
        }
        Ok(index)
    }
    pub(crate) fn rows(&self, epoch: u64) -> impl Iterator<Item = &ChunkReport<'a>> {
        self.rows
            .iter()
            .flatten()
            .filter(move |_| self.epoch == epoch)
    }
}

/// Rows of one admitted-report region, already validated by [`ReportRegion::decode`].
pub struct Rows<'a> {
    reader: Reader<'a>,
}
impl<'a> Iterator for Rows<'a> {
    type Item = CodecResult<AdmittedReport<'a>>;
    fn next(&mut self) -> Option<Self::Item> {
        if self.reader.remaining() == 0 {
            return None;
        }
        Some(AdmittedReport::read_validated(&mut self.reader))
    }
}

/// The F03 admitted-report region at the start of the current-reports section, then the F04
/// bytes after it. Empty section bytes hold no row.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ReportRegion<'a> {
    pub epoch: u64,
    count: usize,
    rows: &'a [u8],
    rest: &'a [u8],
}
impl<'a> ReportRegion<'a> {
    /// Index the immutable rows of a committed section without re-hashing reports
    /// admitted by its producer. A consumer still decodes each report it uses.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn committed(section: &'a [u8]) -> CodecResult<Self> {
        if section.is_empty() {
            return Ok(Self {
                epoch: 0,
                count: 0,
                rows: &[],
                rest: &[],
            });
        }
        let mut r = Reader::new(section);
        let epoch = r.u64()?;
        let count = usize::from(r.u8()?);
        if count > MAX_EVALUATORS {
            return Err(NON_CANONICAL);
        }
        let start = r.offset();
        let mut previous = None;
        for _ in 0..count {
            let evaluator = ChunkReport::read(&mut r)?.evaluator;
            if previous.is_some_and(|id| {
                crate::types::identity_order(id, evaluator.bytes()) != core::cmp::Ordering::Less
            }) {
                return Err(NON_CANONICAL);
            }
            previous = Some(evaluator.bytes());
        }
        let rows = section.get(start..r.offset()).ok_or(NON_CANONICAL)?;
        let rest = section.get(r.offset()..).ok_or(NON_CANONICAL)?;
        if count == 0 && rest.is_empty() {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            epoch,
            count,
            rows,
            rest,
        })
    }

    /// Strictly decodes the region prefix of the current-reports section.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` for a truncated, oversized, unordered or duplicate region, a row
    /// whose signed report, evaluator, epoch or digest disagree, or an empty region with
    /// nothing after it.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(section: &'a [u8]) -> CodecResult<Self> {
        #[cfg(target_arch = "wasm32")]
        if let Some(frozen) = crate::state::current_slice(section) {
            static DECODED: crate::guest_cell::GuestCell<
                crate::guest_cell::Cached<(&'static [u8], ReportRegion<'static>)>,
            > = crate::guest_cell::GuestCell::new(crate::guest_cell::Cached::Empty);
            let mut decoded = DECODED.try_borrow_mut().ok_or(NON_CANONICAL)?;
            if let Some((bytes, region)) = decoded.as_ref() {
                if core::ptr::eq(*bytes, section) {
                    return Ok(*region);
                }
            }
            let region = ReportRegion::decode_uncached(frozen)?;
            *decoded = crate::guest_cell::Cached::Value((frozen, region));
            return Ok(region);
        }
        Self::decode_uncached(section)
    }
    fn decode_uncached(section: &'a [u8]) -> CodecResult<Self> {
        if section.is_empty() {
            return Ok(Self {
                epoch: 0,
                count: 0,
                rows: &[],
                rest: &[],
            });
        }
        let mut r = Reader::new(section);
        let epoch = r.u64()?;
        let count = usize::from(r.u8()?);
        if count > MAX_EVALUATORS {
            return Err(NON_CANONICAL);
        }
        let start = r.offset();
        let mut previous = None;
        for _ in 0..count {
            let evaluator = AdmittedReport::read(&mut r, epoch)?.receipt.evaluator;
            if previous.is_some_and(|p| p >= evaluator) {
                return Err(NON_CANONICAL);
            }
            previous = Some(evaluator);
        }
        let rows = section.get(start..r.offset()).ok_or(NON_CANONICAL)?;
        let rest = section.get(r.offset()..).ok_or(NON_CANONICAL)?;
        if count == 0 && rest.is_empty() {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            epoch,
            count,
            rows,
            rest,
        })
    }
    const fn of(&self, epoch: u64) -> (usize, &'a [u8]) {
        if self.epoch == epoch {
            (self.count, self.rows)
        } else {
            (0, &[])
        }
    }
    /// Admitted rows of `epoch`; rows of any other epoch read as absent.
    #[must_use]
    pub const fn rows(&self, epoch: u64) -> Rows<'a> {
        Rows {
            reader: Reader::new(self.of(epoch).1),
        }
    }
    /// The admitted row of `evaluator` in `epoch`.
    ///
    /// # Errors
    /// Propagates row decoding refusals.
    pub fn get(
        &self,
        epoch: u64,
        evaluator: EvaluatorId,
    ) -> CodecResult<Option<AdmittedReport<'a>>> {
        for row in self.rows(epoch) {
            let row = row?;
            if row.receipt.evaluator == evaluator {
                return Ok(Some(row));
            }
        }
        Ok(None)
    }
    /// The F04 bytes after the region.
    #[must_use]
    pub const fn rest(&self) -> &'a [u8] {
        self.rest
    }
    /// Writes the region of `epoch` with `row` inserted, then the unchanged trailing bytes.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write_with(
        &self,
        epoch: u64,
        row: &AdmittedReport<'_>,
        out: &mut [u8],
    ) -> CodecResult<usize> {
        let (count, current) = self.of(epoch);
        if count >= MAX_EVALUATORS {
            return Err(CAPACITY);
        }
        let mut at = current.len();
        let mut reader = Reader::new(current);
        while reader.remaining() > 0 {
            let offset = reader.offset();
            if AdmittedReport::read_validated(&mut reader)?
                .receipt
                .evaluator
                > row.receipt.evaluator
            {
                at = offset;
                break;
            }
        }
        let mut w = Writer::new(out);
        w.u64(epoch)?;
        w.u8(u8::try_from(count + 1).map_err(|_| ARITHMETIC)?)?;
        w.put(current.get(..at).ok_or(NON_CANONICAL)?)?;
        row.write(&mut w)?;
        w.put(current.get(at..).ok_or(NON_CANONICAL)?)?;
        w.put(self.rest)?;
        Ok(w.len())
    }
}

/// The admitted row of `evaluator` in the opened `epoch`, read from committed state; F04
/// finalized slot queries and F05 inputs read it here.
///
/// # Errors
/// Returns `WRONG_EPOCH` when `epoch` is not the opened epoch; `NON_CANONICAL` for an
/// inconsistent state or region.
pub fn admitted_report<'a>(
    state: &SharedState<'a>,
    epoch: u64,
    evaluator: EvaluatorId,
) -> CodecResult<Option<AdmittedReport<'a>>> {
    let opened = view(state, None)?;
    if opened.frozen.epoch != epoch {
        return Err(WRONG_EPOCH);
    }
    opened.reports.get(epoch, evaluator)
}

/// Committed F01 section of `state`; its header revision must equal the shared revision.
fn committed<'a>(state: &SharedState<'a>) -> CodecResult<PolicySection<'a>> {
    let section = PolicySection::decode(
        *state
            .feature_sections
            .guest_index(Section::PolicyLifecycle.index()),
    )?;
    if section.header.state_revision != state.revision {
        return Err(NON_CANONICAL);
    }
    Ok(section)
}

/// Committed views of the currently opened epoch.
struct Opened<'a> {
    section: PolicySection<'a>,
    dictionary: crate::rewards::RecipientDictionary<'a>,
    frozen: FrozenBinding,
    row: RewardEpoch,
    reports: ReportRegion<'a>,
}

/// The opened epoch: F08 epoch presence, F01 domain and config and the F06 frozen roster row,
/// reusing the reward state `known` when it is the one `state` holds.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn view<'a>(state: &SharedState<'a>, known: Option<RewardState<'a>>) -> CodecResult<Opened<'a>> {
    let section = committed(state)?;
    let admission = AdmissionTable::decode(crate::reputation_transition::framed_admission_table(
        *state
            .feature_sections
            .guest_index(Section::ReputationAdmission.index()),
    )?)?;
    view_in_state(state, section, known, &admission, None)
}

fn view_in_state<'a>(
    state: &SharedState<'a>,
    section: PolicySection<'a>,
    _known: Option<RewardState<'a>>,
    admission: &AdmissionTable,
    known_reports: Option<ReportRegion<'a>>,
) -> CodecResult<Opened<'a>> {
    let epoch = admission.current_epoch().ok_or(WRONG_EPOCH)?;
    let rewards = crate::rewards::committed_row(state.room_market()?.0, epoch).map_err(|e| {
        if e == NOT_FOUND {
            WRONG_EPOCH
        } else {
            e
        }
    })?;
    let row = rewards.row;
    let header = &section.header;
    let frozen = FrozenBinding {
        chain: header.deployment_chain_domain,
        program: header.program_id,
        market: header.market_id,
        epoch,
        config: Version::new(header.active_config_version)?,
        roster: row.roster,
    };
    let reports = match known_reports {
        Some(reports) => reports,
        None => ReportRegion::decode(
            *state
                .feature_sections
                .guest_index(Section::CurrentReports.index()),
        )?,
    };
    if reports.epoch > epoch && reports.count > 0 {
        return Err(NON_CANONICAL);
    }
    Ok(Opened {
        section,
        dictionary: rewards.dictionary,
        frozen,
        row,
        reports,
    })
}

/// One reveal-admission request supplied by the F04 `RevealScore` operation.
#[derive(Clone, Copy)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct RevealAdmission<'a> {
    /// The revealed report body and its evaluator signature.
    pub report: SignedReport<'a>,
    /// The reveal's role request under the evaluator owner's replay slot.
    pub request: ReplayRequest,
    /// Authenticated executing batch height.
    pub height: u64,
    /// Authenticated activity sequence of the reveal.
    pub activity: u64,
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum Admission {
    /// One revision increment and one evaluator sequence were composed into `next` and the
    /// 228-byte `RevealScore` event into `event`; F04 emits it as the sole admission event.
    Applied {
        receipt: AdmissionReceipt,
        revision: u64,
        state_len: usize,
        event_len: usize,
    },
    /// The same report digest is already admitted under a new request: the original receipt;
    /// nothing was written.
    AlreadyAdmitted(AdmissionReceipt),
    /// Exact retry of the reveal request already applied under the evaluator sequence.
    Retained(RetainedResult),
}

/// F08 membership of the frozen evaluator remains admitted under its owner.
fn check_membership(admission: &AdmissionTable, frozen: &FrozenEvaluator) -> CodecResult<()> {
    match admission.get(Participant::Evaluator(frozen.entry.evaluator)) {
        Some(meta) if meta.revoked() => Err(REVOKED),
        Some(meta) if meta.admitted() && meta.owner == frozen.entry.owner => Ok(()),
        _ => Err(UNAUTHORIZED),
    }
}

/// The frozen evaluator of `binding` in the opened epoch, after the frozen domain, epoch,
/// config and roster comparison.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn frozen_evaluator(
    state: &SharedState<'_>,
    opened: &Opened<'_>,
    binding: &EvaluatorBinding,
) -> CodecResult<FrozenEvaluator> {
    check_binding(
        binding,
        &EvaluatorBinding {
            frozen: opened.frozen,
            ..*binding
        },
    )?;
    EvaluatorRegion::with_decoded(
        crate::evaluators::authority::split_identity_section(
            state.section(Section::IdentityRoster)?,
        )?
        .1,
        |region| {
            let snapshot = region.snapshot().ok_or(crate::errors::F03_NO_GRANT)?;
            if snapshot.epoch != opened.frozen.epoch {
                return Err(NON_CANONICAL);
            }
            snapshot
                .get(binding.evaluator)
                .ok_or(crate::errors::F03_NO_GRANT)
        },
    )
}

/// Verifies the report against the complete frozen context and the evaluator signature.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn verify(
    state: &SharedState<'_>,
    opened: &Opened<'_>,
    frozen: &FrozenEvaluator,
    report: &SignedReport<'_>,
    admission: &AdmissionTable,
    known_digest: Option<ReportDigest>,
) -> Result<ReportDigest, VerificationError> {
    let evaluator = frozen.entry.evaluator;
    let (captured, frozen_grant, live_grant) = EvaluatorRegion::with_decoded(
        crate::evaluators::authority::split_identity_section(
            state.section(Section::IdentityRoster)?,
        )?
        .1,
        |region| {
            let captured = crate::epoch::committed_roster_row(
                admission,
                &opened.section,
                region,
                opened.row,
                opened.dictionary,
            )?;
            Ok((
                captured,
                region.frozen_grant(evaluator)?,
                region.stored_grant(evaluator)?,
            ))
        },
    )?;
    let context = ReportContext {
        binding: EvaluatorBinding {
            frozen: opened.frozen,
            evaluator,
            grant: frozen.entry.grant,
            key_version: frozen.entry.key_version,
        },
        frozen_grant,
        live_grant,
        approved_rubric: opened.section.current.commitments.rubric,
        market_owner: opened.section.header.owner_principal,
        workers: captured.workers(),
        evidence: crate::evidence::sealed_evidence_for_binding(state, opened.frozen, evaluator)?,
    };
    let digest = match known_digest {
        Some(digest) => {
            crate::evaluators::codec::verify_report_with_digest(report, &context, digest)?
        }
        None => verify_signed_report(report, &context)?,
    };
    // Worker ownership is also a live role constraint; the captured roster
    // remains the immutable source of report keys and metadata.
    let (workers, _) = crate::evaluators::authority::split_identity_section(
        *state
            .feature_sections
            .guest_index(Section::IdentityRoster.index()),
    )?;
    if crate::workers::committed_has_owner(workers, frozen.entry.owner)? {
        return Err(crate::errors::ROLE_CONFLICT.into());
    }
    Ok(digest)
}

/// Commits `row` at the next revision under the reveal request.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn commit_report(
    state: &SharedState<'_>,
    opened: &Opened<'_>,
    row: &AdmittedReport<'_>,
    reveal: &RevealAdmission<'_>,
    next: &mut [u8],
    scratch: &mut [u8],
) -> CodecResult<(u64, usize)> {
    let (policy, rest) = scratch.split_at_mut_checked(POLICY_CAP).ok_or(CAPACITY)?;
    let (reports, control) = rest.split_at_mut_checked(REPORTS_CAP).ok_or(CAPACITY)?;
    let reports_len = opened
        .reports
        .write_with(opened.frozen.epoch, row, reports)?;
    let reports = reports.get(..reports_len).ok_or(CAPACITY)?;
    let mut candidate = state.clone();
    *candidate
        .feature_sections
        .guest_index_mut(Section::CurrentReports.index()) = reports;
    let result = row.receipt.result()?;
    crate::rewards::finish_feature_transition(
        candidate,
        opened.section,
        Some(&reveal.request),
        reveal.height,
        result,
        NON_CANONICAL,
        policy,
        control,
        next,
    )
}

/// `ValidateReportAdmission`: admits one canonical signed report of a frozen evaluator into
/// its immutable slot of the opened epoch, called atomically only by F04 `RevealScore`, which
/// owns the phase, window, salt and commitment checks and the commitment consumption. Writes
/// the whole next state into `next` (at least `MAX_STATE_BYTES`) and the `RevealScore` event
/// into `event` (at least 228 bytes); `scratch` holds at least [`ADMISSION_SCRATCH_BYTES`]. Scores are stored exactly as signed: no normalization, reward
/// or transfer follows.
///
/// # Errors
/// `F03_NO_SCORES`, `F03_NONCANONICAL_VECTOR`, `F03_SCORE_RANGE` and `NON_CANONICAL` for the
/// report structure; `WRONG_EPOCH` without an opened epoch; `WRONG_DOMAIN`, `WRONG_EPOCH`,
/// `WRONG_CONFIG` and `WRONG_ROSTER` for the frozen binding; `F03_NO_GRANT` outside the frozen
/// snapshot; `UNAUTHORIZED` for a request principal other than the evaluator owner or a lost
/// F08 membership; common replay refusals; `F03_REPORT_ALREADY_FINAL` for another digest of an
/// admitted slot; `REVOKED` for a revoked grant or membership; the `verify_signed_report`
/// grant, key, rubric, role, evidence (`F03_EVIDENCE_NOT_SEALED`, `EVIDENCE_BINDING`,
/// `F03_EVIDENCE_ROOT_MISMATCH`), worker (`F03_UNKNOWN_WORKER`) and `BAD_SIGNATURE` refusals;
/// `CAPACITY`, `ARITHMETIC` and `F01_CAPACITY_UNAVAILABLE`; host verification failures stay
/// host failures. On any error `current` is unchanged and the outputs must be discarded.
pub fn admit_report(
    current: &[u8],
    reveal: &RevealAdmission<'_>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> Result<Admission, VerificationError> {
    admit_report_with(current, reveal, next, scratch, event, None)
}

/// [`admit_report`] reusing the reward state `known` the caller already validated in
/// `current`.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn admit_report_with<'a>(
    current: &'a [u8],
    reveal: &RevealAdmission<'_>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    known: Option<RewardState<'a>>,
) -> Result<Admission, VerificationError> {
    let mut signed = [0; SIGNED_REPORT_MAX_BYTES];
    let signed_len = encode_signed_report(&reveal.report, &mut signed)?;
    let signed = signed.get(..signed_len).ok_or(CAPACITY)?;
    let state = decode_shared_state(current)?;
    admit_encoded_report(
        &state, reveal, signed, next, scratch, event, known, None, None, None,
    )
}

pub(crate) fn admit_report_in_state<'a>(
    state: &SharedState<'a>,
    admission: &AdmissionTable,
    reports: ReportRegion<'a>,
    reveal: &RevealAdmission<'_>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    known: Option<RewardState<'a>>,
    report: ReportDigest,
    signed: &[u8],
) -> Result<Admission, VerificationError> {
    admit_encoded_report(
        state,
        reveal,
        signed,
        next,
        scratch,
        event,
        known,
        Some(admission),
        Some(reports),
        Some(report),
    )
}

fn admit_encoded_report<'a>(
    state: &SharedState<'a>,
    reveal: &RevealAdmission<'_>,
    signed: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    known: Option<RewardState<'a>>,
    known_admission: Option<&AdmissionTable>,
    known_reports: Option<ReportRegion<'a>>,
    known_digest: Option<ReportDigest>,
) -> Result<Admission, VerificationError> {
    let body = &reveal.report.body;
    let section = committed(state)?;
    let decoded;
    let admission = match known_admission {
        Some(admission) => admission,
        None => {
            decoded =
                AdmissionTable::decode(crate::reputation_transition::framed_admission_table(
                    *state
                        .feature_sections
                        .guest_index(Section::ReputationAdmission.index()),
                )?)?;
            &decoded
        }
    };
    let opened = view_in_state(state, section, known, admission, known_reports)?;
    let frozen = frozen_evaluator(state, &opened, &body.binding)?;
    if reveal.request.principal != frozen.entry.owner {
        return Err(UNAUTHORIZED.into());
    }
    if let ReplayDecision::AlreadyApplied(retained) =
        state.control.replay.check(&reveal.request, reveal.height)?
    {
        return Ok(Admission::Retained(retained));
    }
    let epoch = opened.frozen.epoch;
    if let Some(stored) = opened.reports.get(epoch, frozen.entry.evaluator)? {
        return if stored.receipt.report == codec::report_digest(body)? {
            Ok(Admission::AlreadyAdmitted(stored.receipt))
        } else {
            Err(F03_REPORT_ALREADY_FINAL.into())
        };
    }
    check_membership(admission, &frozen)?;
    let report = verify(
        &state,
        &opened,
        &frozen,
        &reveal.report,
        admission,
        known_digest,
    )?;
    let row = AdmittedReport {
        receipt: AdmissionReceipt {
            evaluator: frozen.entry.evaluator,
            report,
            height: reveal.height,
            activity: reveal.activity,
        },
        signed,
    };
    let (revision, state_len) = commit_report(&state, &opened, &row, reveal, next, scratch)?;
    let reveal_event = RevealScoreEvent {
        common: EventCommon {
            market: opened.frozen.market,
            epoch,
            config: opened.frozen.config,
            revision,
            request: reveal.request.digest,
            result: row.receipt.result()?,
        },
        evaluator: frozen.entry.evaluator,
        report,
        evidence: body.evidence,
        vector_count: u16::try_from(body.scores.len()).map_err(|_| ARITHMETIC)?,
        admitted_height: reveal.height,
    };
    let event_len = codec::encode_reveal_event(&reveal_event, event)?;
    Ok(Admission::Applied {
        receipt: row.receipt,
        revision,
        state_len,
        event_len,
    })
}

/// Advisory challenge categories; none adjudicates or changes any score.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum ChallengeCategory {
    SignatureOrDomain = 1,
    IdentityConflict = 2,
    PolicyOrEvidenceMismatch = 3,
    QualityDisagreement = 4,
}
impl ChallengeCategory {
    /// Decodes the wire category.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` for an unknown category.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(value: u8) -> CodecResult<Self> {
        Ok(match value {
            1 => Self::SignatureOrDomain,
            2 => Self::IdentityConflict,
            3 => Self::PolicyOrEvidenceMismatch,
            4 => Self::QualityDisagreement,
            _ => return Err(NON_CANONICAL),
        })
    }
}

/// One recorded challenge commitment (status 1, recorded; no adjudicated status in v1).
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ChallengeRecord {
    pub id: Digest32,
    pub evaluator: EvaluatorId,
    pub report: ReportDigest,
    pub category: ChallengeCategory,
    pub evidence: Digest32,
    pub requester: PrincipalId,
    pub height: u64,
}
impl ChallengeRecord {
    /// The canonical 170-byte record.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when the record does not fill exactly 170 bytes.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self) -> CodecResult<[u8; CHALLENGE_RECORD_BYTES]> {
        let mut out = [0; CHALLENGE_RECORD_BYTES];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w;
            id32: self.id.as_bytes();
            id32: self.evaluator.as_bytes();
            id32: self.report.as_bytes();
            u8: self.category as u8;
            id32: self.evidence.as_bytes();
            id32: self.requester.as_bytes();
            u64: self.height;
            u8: RECORDED;
        )?;
        if w.len() != CHALLENGE_RECORD_BYTES {
            return Err(NON_CANONICAL);
        }
        Ok(out)
    }
    /// Strictly decodes one record.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` for a wrong length, a zero identity or digest, an unknown
    /// category or a status other than recorded.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(input: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(input);
        let record = crate::codec::read_record!(&mut r, Self {  }; id = r.digest32()?;
            evaluator: EvaluatorId,
            report: ReportDigest,
            category: ChallengeCategory,
            evidence: Digest32,
            requester: PrincipalId,
            height: U64
        );
        if r.u8()? != RECORDED {
            return Err(NON_CANONICAL);
        }
        r.finish()?;
        Ok(record)
    }
    /// Equal allegations: every field except the recording height.
    #[must_use]
    pub fn same_body(&self, other: &Self) -> bool {
        Self {
            height: other.height,
            ..*self
        } == *other
    }
}

/// The challenge region after the F09 seal region of the control feature bytes, then the
/// bytes of later control producers.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ChallengeRegion<'a> {
    pub epoch: u64,
    records: &'a [u8],
    rest: &'a [u8],
}
impl<'a> ChallengeRegion<'a> {
    /// Strictly decodes the region prefix of `bytes` (the F09 seal region's rest).
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` for a truncated or oversized region, an invalid or repeated
    /// record, or an empty region with nothing after it.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(bytes: &'a [u8]) -> CodecResult<Self> {
        if bytes.is_empty() {
            return Ok(Self {
                epoch: 0,
                records: &[],
                rest: &[],
            });
        }
        let mut r = Reader::new(bytes);
        let epoch = r.u64()?;
        let count = usize::from(r.u16()?);
        if count > MAX_CHALLENGES {
            return Err(NON_CANONICAL);
        }
        let records = r.take(count * CHALLENGE_RECORD_BYTES)?;
        let rest = bytes.get(r.offset()..).ok_or(NON_CANONICAL)?;
        if count == 0 && rest.is_empty() {
            return Err(NON_CANONICAL);
        }
        for (index, record) in records
            .as_chunks::<{ CHALLENGE_RECORD_BYTES }>()
            .0
            .iter()
            .map(|chunk| chunk.as_slice())
            .enumerate()
        {
            let id = ChallengeRecord::decode(record)?.id;
            let earlier = records
                .get(..index * CHALLENGE_RECORD_BYTES)
                .ok_or(NON_CANONICAL)?;
            for other in earlier
                .as_chunks::<{ CHALLENGE_RECORD_BYTES }>()
                .0
                .iter()
                .map(|chunk| chunk.as_slice())
            {
                if ChallengeRecord::decode(other)?.id == id {
                    return Err(NON_CANONICAL);
                }
            }
        }
        Ok(Self {
            epoch,
            records,
            rest,
        })
    }
    const fn of(&self, epoch: u64) -> &'a [u8] {
        if self.epoch == epoch {
            self.records
        } else {
            &[]
        }
    }
    /// Challenges recorded in `epoch`, in recording order; other epochs read as absent.
    pub fn challenges(
        &self,
        epoch: u64,
    ) -> impl Iterator<Item = CodecResult<ChallengeRecord>> + 'a {
        self.of(epoch)
            .as_chunks::<{ CHALLENGE_RECORD_BYTES }>()
            .0
            .iter()
            .map(|chunk| chunk.as_slice())
            .map(ChallengeRecord::decode)
    }
    /// The challenge `id` recorded in `epoch`.
    ///
    /// # Errors
    /// Propagates record decoding refusals.
    pub fn get(&self, epoch: u64, id: Digest32) -> CodecResult<Option<ChallengeRecord>> {
        for record in self.challenges(epoch) {
            let record = record?;
            if record.id == id {
                return Ok(Some(record));
            }
        }
        Ok(None)
    }
    /// Bytes of later control producers after the region.
    #[must_use]
    pub const fn rest(&self) -> &'a [u8] {
        self.rest
    }
    /// Writes the region of `epoch` with `record` appended, then the unchanged trailing bytes.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write_with(
        &self,
        epoch: u64,
        record: &ChallengeRecord,
        out: &mut [u8],
    ) -> CodecResult<usize> {
        let current = self.of(epoch);
        let count = current.len() / CHALLENGE_RECORD_BYTES;
        if count >= MAX_CHALLENGES {
            return Err(F03_CHALLENGE_CAPACITY);
        }
        let mut w = Writer::new(out);
        w.u64(epoch)?;
        w.u16(u16::try_from(count + 1).map_err(|_| ARITHMETIC)?)?;
        w.put(current)?;
        w.put(&record.encode()?)?;
        w.put(self.rest)?;
        Ok(w.len())
    }
}

/// The challenge region of committed control feature bytes.
///
/// # Errors
/// Propagates seal-region and challenge-region decoding refusals.
pub fn challenge_region(feature_bytes: &[u8]) -> CodecResult<ChallengeRegion<'_>> {
    ChallengeRegion::decode(SealRegion::decode(feature_bytes)?.rest())
}

/// The parsed `ChallengeAssessment` payload.
struct Allegation {
    evaluator: EvaluatorId,
    report: ReportDigest,
    category: ChallengeCategory,
    evidence: Digest32,
}
impl Allegation {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn parse(payload: &[u8]) -> CodecResult<Self> {
        if payload.len() != CHALLENGE_PAYLOAD_BYTES {
            return Err(NON_CANONICAL);
        }
        let mut r = Reader::new(payload);
        let value = crate::codec::read_record!(&mut r, Self {  }; evaluator = r.evaluator_id()?;
            report: ReportDigest,
            category: ChallengeCategory,
            evidence: Digest32
        );
        r.finish()?;
        Ok(value)
    }
    /// `H('PAXAI/evaluator-challenge/v1', chain || program || market || epoch || evaluator ||
    /// report || category || evidence || requester)`.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn identity(&self, frozen: &FrozenBinding, requester: PrincipalId) -> CodecResult<Digest32> {
        let mut bytes = [0; CHALLENGE_PREIMAGE_BYTES];
        let mut w = Writer::new(&mut bytes);
        crate::codec::write_fields!(w;
            id32: frozen.chain.as_bytes();
            id32: frozen.program.as_bytes();
            id32: frozen.market.as_bytes();
            u64: frozen.epoch;
            id32: self.evaluator.as_bytes();
            id32: self.report.as_bytes();
            u8: self.category as u8;
            id32: self.evidence.as_bytes();
            id32: requester.as_bytes();
        )?;
        if w.len() != CHALLENGE_PREIMAGE_BYTES {
            return Err(NON_CANONICAL);
        }
        codec::domain_hash(CHALLENGE_DOMAIN, &bytes)
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum ChallengeOutcome {
    /// One revision increment was composed into `next` and the `ChallengeAssessment` event
    /// into `event`; the result digest is the result digest of the record.
    Applied {
        record: ChallengeRecord,
        revision: u64,
        result: ResultDigest,
        state_len: usize,
        event_len: usize,
    },
    /// The identical challenge identity is already recorded; nothing was written.
    AlreadyApplied { record: ChallengeRecord },
}

/// Domain, expiry, market and the exact frozen epoch, config and roster of the envelope.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn check_envelope(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    frozen: &FrozenBinding,
) -> CodecResult<()> {
    let e = &envelope.envelope;
    let market = derive_market(ctx.chain, ctx.program)?;
    e.check_domain(ctx.chain, ctx.program, market)?;
    e.check_expiry(ctx.height)?;
    if frozen.chain != ctx.chain || frozen.program != ctx.program || frozen.market != market {
        return Err(WRONG_MARKET);
    }
    if e.epoch != frozen.epoch {
        return Err(WRONG_EPOCH);
    }
    if e.config != frozen.config.get() {
        return Err(WRONG_CONFIG);
    }
    if e.roster != Presence::Present(frozen.roster) {
        return Err(WRONG_ROSTER);
    }
    Ok(())
}

/// The allegation must name the admitted report of a frozen evaluator of the opened epoch.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn check_reference(
    state: &SharedState<'_>,
    opened: &Opened<'_>,
    allegation: &Allegation,
) -> CodecResult<()> {
    let region = evaluator_region(
        *state
            .feature_sections
            .guest_index(Section::IdentityRoster.index()),
    )?;
    let snapshot = region.snapshot().ok_or(NOT_FOUND)?;
    if snapshot.epoch != opened.frozen.epoch {
        return Err(NON_CANONICAL);
    }
    snapshot.get(allegation.evaluator).ok_or(NOT_FOUND)?;
    match opened
        .reports
        .get(opened.frozen.epoch, allegation.evaluator)?
    {
        Some(row) if row.receipt.report == allegation.report => Ok(()),
        _ => Err(NOT_FOUND),
    }
}

/// Commits `record` at the next revision (object-local; no role sequence) and emits it.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn commit_challenge(
    state: &SharedState<'_>,
    opened: &Opened<'_>,
    envelope: &ValidatedEnvelope<'_>,
    record: &ChallengeRecord,
    buffers: (&mut [u8], &mut [u8], &mut [u8]),
) -> CodecResult<ChallengeOutcome> {
    let (next, scratch, event) = buffers;
    let (policy, rest) = scratch.split_at_mut_checked(POLICY_CAP).ok_or(CAPACITY)?;
    let (features, control) = rest.split_at_mut_checked(CONTROL_CAP).ok_or(CAPACITY)?;
    let feature_bytes = state.control.feature_bytes;
    let seals = SealRegion::decode(feature_bytes)?;
    let prefix_len = feature_bytes
        .len()
        .checked_sub(seals.rest().len())
        .ok_or(NON_CANONICAL)?;
    let prefix = feature_bytes.get(..prefix_len).ok_or(NON_CANONICAL)?;
    let epoch = opened.frozen.epoch;
    let features_len = {
        let mut w = Writer::new(features);
        if prefix.is_empty() {
            w.u64(epoch)?;
            w.u8(0)?;
        } else {
            w.put(prefix)?;
        }
        let at = w.len();
        let region = ChallengeRegion::decode(seals.rest())?;
        at.checked_add(region.write_with(epoch, record, features.get_mut(at..).ok_or(CAPACITY)?)?)
            .ok_or(ARITHMETIC)?
    };
    let record_bytes = record.encode()?;
    let result = codec::result_digest(&record_bytes)?;
    let mut candidate = state.clone();
    candidate.control.feature_bytes = features.get(..features_len).ok_or(CAPACITY)?;
    let (revision, state_len) = crate::rewards::finish_feature_transition(
        candidate,
        opened.section,
        None,
        0,
        result,
        NON_CANONICAL,
        policy,
        control,
        next,
    )?;
    let event_len = codec::encode_event_frame(
        dispatch::ChallengeAssessment,
        &EventCommon {
            market: opened.frozen.market,
            epoch,
            config: opened.frozen.config,
            revision,
            request: envelope.request_digest()?,
            result,
        },
        &record_bytes,
        event,
    )?;
    Ok(ChallengeOutcome::Applied {
        record: *record,
        revision,
        result,
        state_len,
        event_len,
    })
}

/// Applies one permissionless `ChallengeAssessment` (0x0304) to the committed `current` state:
/// records a bounded advisory allegation against an admitted report of the opened epoch,
/// writing the whole next state into `next` (at least `MAX_STATE_BYTES`) and its event into
/// `event`; `scratch` holds at least [`CHALLENGE_SCRATCH_BYTES`]. A challenge never changes a
/// score, reward, claim or settlement. Dispatch arm:
/// `dispatch::ChallengeAssessment => admission::apply_challenge(&ctx, &envelope, current, next,
/// scratch, event)`.
///
/// # Errors
/// `UNKNOWN_OPERATION`; `NON_CANONICAL` for a malformed payload, zero identity or digest,
/// unknown category or an inconsistent committed state; `UNAUTHORIZED` unless the native kind0
/// context principal is the envelope actor; `WRONG_DOMAIN`, `WRONG_PROGRAM`, `WRONG_MARKET`,
/// `EXPIRED`; `WRONG_EPOCH` (also with no opened epoch), `WRONG_CONFIG` and `WRONG_ROSTER` for
/// a binding other than the opened epoch's; `CONFLICT` when the asserted request identity is
/// not the derived challenge identity or names a different recorded body; `NOT_FOUND` for an
/// evaluator or report digest that is not admitted in the opened epoch;
/// `F03_CHALLENGE_CAPACITY` for a ninth distinct challenge; `CAPACITY`, `ARITHMETIC` and
/// `F01_CAPACITY_UNAVAILABLE`. On any error `current` is unchanged and the outputs must be
/// discarded.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn apply_challenge(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<ChallengeOutcome> {
    let e = &envelope.envelope;
    if e.operation != dispatch::ChallengeAssessment {
        return Err(UNKNOWN_OPERATION);
    }
    let allegation = Allegation::parse(e.payload)?;
    compare_native_principal(e, ctx.principal)?;
    let state = decode_shared_state(current)?;
    let opened = view(&state, None)?;
    check_envelope(ctx, envelope, &opened.frozen)?;
    let id = allegation.identity(&opened.frozen, e.actor)?;
    if e.request.bytes() != id.bytes() {
        return Err(CONFLICT);
    }
    let record = ChallengeRecord {
        id,
        evaluator: allegation.evaluator,
        report: allegation.report,
        category: allegation.category,
        evidence: allegation.evidence,
        requester: e.actor,
        height: ctx.height,
    };
    let region = challenge_region(state.control.feature_bytes)?;
    if region.epoch > opened.frozen.epoch && region.challenges(region.epoch).next().is_some() {
        return Err(NON_CANONICAL);
    }
    if let Some(stored) = region.get(opened.frozen.epoch, id)? {
        return if stored.same_body(&record) {
            Ok(ChallengeOutcome::AlreadyApplied { record: stored })
        } else {
            Err(CONFLICT)
        };
    }
    check_reference(&state, &opened, &allegation)?;
    commit_challenge(&state, &opened, envelope, &record, (next, scratch, event))
}
