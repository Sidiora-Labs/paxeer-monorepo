//! F04 atomic commit and reveal windows: `CommitScore` (0x0401) and `RevealScore` (0x0402)
//! over the complete committed shared state value ([`apply`]), and the derived per-evaluator
//! slot status ([`slot`]).
//!
//! Readings chosen where producers are silent:
//! - The commit region follows the F03 admitted-report region in the current-reports section:
//!   `epoch:u64 || count:u8 || count 96-byte records` in strictly ascending evaluator order,
//!   each record `evaluator32 || C32 || commit_height:u64 || commit_sequence:u64 ||
//!   grant_version:u64 || key_version:u64`. The F03 region is carried unchanged; a commit into
//!   an empty section first writes the empty F03 region `epoch || 0`. Records of an earlier
//!   epoch read as absent and the first commit of a later opened epoch replaces them.
//! - Status is derived, never stored: ABSENT without a record of the opened epoch, REVEALED
//!   with an F03 admitted row, otherwise COMMITTED, projected EXPIRED at `height >= T+96`.
//! - The host context carries no native activity sequence, so the authenticated evaluator role
//!   sequence of the envelope is the recorded commit and reveal activity sequence.
//! - The envelope actor is the evaluator owner; the evaluator replay slot is the
//!   `ActorSlot::evaluator(i)` bound to that owner.
//! - A SUSPENDED market (a halted epoch) refuses `CommitScore` with `F08_MARKET_PAUSED`; an
//!   already accepted commitment may still be revealed.
use crate::types::GuestIndex as _;
pub mod commitment;

use crate::{
    admission::AdmissionTable,
    codec::{self, CommitAccepted, EventCommon, Reader, ValidatedEnvelope, Writer},
    dispatch,
    errors::{
        ApplicationError, CodecResult, ARITHMETIC, CAPACITY, CONFLICT, EXPIRED,
        F03_GRANT_VERSION_CONFLICT, F03_KEY_VERSION_CONFLICT, F03_NO_GRANT, F08_MARKET_PAUSED,
        KEY_MISMATCH, NON_CANONICAL, NOT_FOUND, REVOKED, UNAUTHORIZED, UNKNOWN_OPERATION,
        WRONG_EPOCH,
    },
    evaluators::{
        admission::{
            admit_report_with, Admission, AdmissionReceipt, ReportRegion, RevealAdmission,
        },
        authority::{evaluator_region, FrozenEvaluator},
        codec::verify_digest,
        model::{SignedReport, VerificationError},
    },
    registry::check_f01_capacity,
    registry_ops::{CallContext, PolicySection, SUSPENDED},
    rewards::RewardState,
    state::{
        decode_shared_state, encode_shared_state, settlement_rewards, ActorSlot, HeightWindow,
        ReplayDecision, ReplayRequest, RetainedResult, Section, SharedState,
    },
    types::{
        Authentication, CommitmentDigest, EvaluatorBinding, EvaluatorId, FrozenBinding, Presence,
        PrincipalId, ResultDigest, Version,
    },
    MAX_EVALUATORS,
};

/// One stored commit record.
pub const COMMIT_RECORD_BYTES: usize = 96;
const REGION_HEADER_BYTES: usize = 9;
/// The commit region at its bound: epoch, count and eight records.
pub const COMMIT_REGION_MAX_BYTES: usize =
    REGION_HEADER_BYTES + MAX_EVALUATORS * COMMIT_RECORD_BYTES;
/// The commit result payload: `1 || evaluator32 || C32 || height || sequence`.
pub const COMMIT_RESULT_BYTES: usize = 81;
const COMMIT_RESULT_KIND: u8 = 1;
pub(crate) const COMMIT_OPENS: u64 = 64;
pub(crate) const COMMIT_CLOSES: u64 = 80;
const REVEAL_OPENS: u64 = 80;
const REVEAL_CLOSES: u64 = 96;

pub fn canary_window(origin: u64, epoch: u64, reveal: bool) -> CodecResult<HeightWindow> {
    let (opens, closes) = if reveal {
        (REVEAL_OPENS, REVEAL_CLOSES)
    } else {
        (COMMIT_OPENS, COMMIT_CLOSES)
    };
    HeightWindow::epoch(origin, epoch, opens, closes)
}
const POLICY_CAP: usize = Section::PolicyLifecycle.payload_cap();
const REPORTS_CAP: usize = Section::CurrentReports.payload_cap();
const CONTROL_CAP: usize = Section::Control.payload_cap();
/// Caller scratch for [`apply`]: the next F01 section, the next current-reports section and
/// the control encoding (also the F03 admission scratch).
pub const SCRATCH_BYTES: usize = POLICY_CAP + REPORTS_CAP + CONTROL_CAP;

/// The accepted commitment of one evaluator epoch.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct CommitRecord {
    pub evaluator: EvaluatorId,
    pub commitment: CommitmentDigest,
    /// First acceptance height.
    pub height: u64,
    /// First acceptance activity sequence.
    pub sequence: u64,
    /// Frozen grant version of the binding.
    pub grant: Version,
    /// Frozen key version of the binding.
    pub key_version: Version,
}
impl CommitRecord {
    /// The 96-byte stored record.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when the record does not fill exactly 96 bytes.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self) -> CodecResult<[u8; COMMIT_RECORD_BYTES]> {
        let mut out = [0; COMMIT_RECORD_BYTES];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w;
            id32: self.evaluator.as_bytes();
            id32: self.commitment.as_bytes();
            u64: self.height;
            u64: self.sequence;
            u64: self.grant.get();
            u64: self.key_version.get();
        )?;
        if w.len() != COMMIT_RECORD_BYTES {
            return Err(NON_CANONICAL);
        }
        Ok(out)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn read(r: &mut Reader<'_>) -> CodecResult<Self> {
        Ok(
            crate::codec::read_record!(r, Self {  }; evaluator = r.evaluator_id()?;
                commitment: CommitmentDigest,
                height: U64,
                sequence: U64,
                grant: Version,
                key_version: Version
            ),
        )
    }
    /// The 81-byte F04 commit result payload.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when the payload does not fill exactly 81 bytes.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn payload(&self) -> CodecResult<[u8; COMMIT_RESULT_BYTES]> {
        let mut out = [0; COMMIT_RESULT_BYTES];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w;
            u8: COMMIT_RESULT_KIND;
            id32: self.evaluator.as_bytes();
            id32: self.commitment.as_bytes();
            u64: self.height;
            u64: self.sequence;
        )?;
        if w.len() != COMMIT_RESULT_BYTES {
            return Err(NON_CANONICAL);
        }
        Ok(out)
    }
    /// `H('PAXAI/result/v1', payload)`.
    ///
    /// # Errors
    /// Propagates payload and result-digest refusals.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn result(&self) -> CodecResult<ResultDigest> {
        codec::result_digest(&self.payload()?)
    }
}

/// The F04 commit region after the F03 admitted-report region. Empty bytes hold no record.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct CommitRegion<'a> {
    pub epoch: u64,
    records: &'a [u8],
}
impl<'a> CommitRegion<'a> {
    /// Strictly decodes the bytes after the F03 region.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` for a truncated, empty, oversized, unordered or duplicate region
    /// or trailing bytes.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(bytes: &'a [u8]) -> CodecResult<Self> {
        if bytes.is_empty() {
            return Ok(Self {
                epoch: 0,
                records: &[],
            });
        }
        let mut r = Reader::new(bytes);
        let epoch = r.u64()?;
        let count = usize::from(r.u8()?);
        if count == 0 || count > MAX_EVALUATORS {
            return Err(NON_CANONICAL);
        }
        let records = r.take(count * COMMIT_RECORD_BYTES)?;
        r.finish()?;
        let mut reader = Reader::new(records);
        let mut previous = None;
        while reader.remaining() > 0 {
            let evaluator = CommitRecord::read(&mut reader)?.evaluator;
            if previous.is_some_and(|p| p >= evaluator) {
                return Err(NON_CANONICAL);
            }
            previous = Some(evaluator);
        }
        Ok(Self { epoch, records })
    }
    const fn of(&self, epoch: u64) -> &'a [u8] {
        if self.epoch == epoch {
            self.records
        } else {
            &[]
        }
    }
    /// The commit record of `evaluator` in `epoch`; records of any other epoch read as absent.
    ///
    /// # Errors
    /// Propagates record decoding refusals.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn get(&self, epoch: u64, evaluator: EvaluatorId) -> CodecResult<Option<CommitRecord>> {
        let mut r = Reader::new(self.of(epoch));
        while r.remaining() > 0 {
            let record = CommitRecord::read(&mut r)?;
            if record.evaluator == evaluator {
                return Ok(Some(record));
            }
        }
        Ok(None)
    }
    /// Writes the region of `epoch` with `record` inserted in evaluator order.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write_with(&self, epoch: u64, record: &CommitRecord, w: &mut Writer<'_>) -> CodecResult<()> {
        let current = self.of(epoch);
        let count = current.len() / COMMIT_RECORD_BYTES;
        if count >= MAX_EVALUATORS {
            return Err(CAPACITY);
        }
        let mut at = current.len();
        let mut reader = Reader::new(current);
        while reader.remaining() > 0 {
            let offset = reader.offset();
            if CommitRecord::read(&mut reader)?.evaluator > record.evaluator {
                at = offset;
                break;
            }
        }
        w.u64(epoch)?;
        w.u8(u8::try_from(count + 1).map_err(|_| ARITHMETIC)?)?;
        w.put(current.get(..at).ok_or(NON_CANONICAL)?)?;
        w.put(&record.encode()?)?;
        w.put(current.get(at..).ok_or(NON_CANONICAL)?)
    }
}

/// Committed F01 section whose header revision equals the shared revision.
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
    state: SharedState<'a>,
    section: PolicySection<'a>,
    rewards: RewardState<'a>,
    frozen: FrozenBinding,
    reports: ReportRegion<'a>,
    commits: CommitRegion<'a>,
}
/// The opened epoch: F08 epoch presence, F01 domain and config, the F06 frozen roster row and
/// the F03/F04 current-reports regions.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn opened(current: &[u8]) -> CodecResult<Opened<'_>> {
    let state = decode_shared_state(current)?;
    let section = committed(&state)?;
    let admission = AdmissionTable::decode(
        *state
            .feature_sections
            .guest_index(Section::ReputationAdmission.index()),
    )?;
    let epoch = admission.current_epoch().ok_or(WRONG_EPOCH)?;
    let rewards = settlement_rewards(&state, None)?;
    let row = match rewards.row(epoch) {
        Err(NOT_FOUND) => return Err(WRONG_EPOCH),
        row => row?,
    };
    let header = &section.header;
    let frozen = FrozenBinding {
        chain: header.deployment_chain_domain,
        program: header.program_id,
        market: header.market_id,
        epoch,
        config: Version::new(header.active_config_version)?,
        roster: row.roster,
    };
    let reports = ReportRegion::decode(
        *state
            .feature_sections
            .guest_index(Section::CurrentReports.index()),
    )?;
    let commits = CommitRegion::decode(reports.rest())?;
    if commits.epoch > epoch && !commits.records.is_empty() {
        return Err(NON_CANONICAL);
    }
    Ok(Opened {
        state,
        section,
        rewards,
        frozen,
        reports,
        commits,
    })
}

/// Derived status of one evaluator epoch.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum Status {
    Absent = 0,
    Committed = 1,
    Revealed = 2,
    /// Projection of COMMITTED at `height >= T+96`.
    Expired = 3,
}

/// The current-epoch slot of one evaluator.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Slot {
    pub status: Status,
    pub commit: Option<CommitRecord>,
    pub reveal: Option<AdmissionReceipt>,
}

/// The slot of `evaluator` in the opened epoch of committed `current`, projected at `height`.
/// The projection needs no maintenance write.
///
/// # Errors
/// Returns `WRONG_EPOCH` without an opened epoch; `NON_CANONICAL` for an inconsistent state;
/// `ARITHMETIC` when the reveal end overflows.
pub fn slot(current: &[u8], evaluator: EvaluatorId, height: u64) -> CodecResult<Slot> {
    let opened = opened(current)?;
    let epoch = opened.frozen.epoch;
    let commit = opened.commits.get(epoch, evaluator)?;
    let reveal = opened.reports.get(epoch, evaluator)?.map(|row| row.receipt);
    let origin = opened.section.header.origin_height;
    let reveal_end = HeightWindow::epoch(origin, epoch, REVEAL_OPENS, REVEAL_CLOSES)?.end;
    let status = match (commit, reveal) {
        (_, Some(_)) => Status::Revealed,
        (Some(_), None) if height >= reveal_end => Status::Expired,
        (Some(_), None) => Status::Committed,
        (None, None) => Status::Absent,
    };
    Ok(Slot {
        status,
        commit,
        reveal,
    })
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum Outcome {
    /// ABSENT became COMMITTED: one revision increment and one evaluator sequence were
    /// composed into `next` and the 194-byte `CommitScore` event into `event`.
    Committed {
        record: CommitRecord,
        revision: u64,
        result: ResultDigest,
        state_len: usize,
        event_len: usize,
    },
    /// COMMITTED became REVEALED through F03 admission: one revision increment and one
    /// evaluator sequence were composed into `next` and the 228-byte `RevealScore` event into
    /// `event`.
    Revealed {
        receipt: AdmissionReceipt,
        revision: u64,
        state_len: usize,
        event_len: usize,
    },
    /// Exact retry of the request already applied under the evaluator sequence; nothing was
    /// written.
    Retained(RetainedResult),
}

fn verification(error: VerificationError) -> ApplicationError {
    match error {
        VerificationError::Application(code) => code,
        #[cfg(target_arch = "wasm32")]
        VerificationError::Host(_) => crate::errors::HOST_CAPABILITY,
    }
}

struct Call<'c> {
    ctx: &'c CallContext,
    envelope: &'c ValidatedEnvelope<'c>,
    opened: &'c Opened<'c>,
}
impl Call<'_> {
    /// Domain, expiry, market and the exact frozen epoch, config and roster.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn check_envelope(&self) -> CodecResult<()> {
        codec::check_frozen_envelope(&self.envelope.envelope, self.ctx, &self.opened.frozen)
    }
    /// The frozen evaluator `binding` names, authenticated natively by its owner or by its
    /// frozen delegate key, with the complete binding compared to the frozen record.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn authenticate(&self, binding: &EvaluatorBinding) -> CodecResult<FrozenEvaluator> {
        let e = &self.envelope.envelope;
        let identity = *self
            .opened
            .state
            .feature_sections
            .guest_index(Section::IdentityRoster.index());
        let region = evaluator_region(identity)?;
        let snapshot = region.snapshot().ok_or(F03_NO_GRANT)?;
        if snapshot.epoch != self.opened.frozen.epoch {
            return Err(NON_CANONICAL);
        }
        let frozen = snapshot.get(binding.evaluator).ok_or(UNAUTHORIZED)?;
        if e.actor != frozen.entry.owner {
            return Err(UNAUTHORIZED);
        }
        match e.authentication {
            Authentication::Native => codec::compare_native_principal(e, self.ctx.principal)?,
            Authentication::Delegate { key, signature } => {
                if key != frozen.entry.public_key {
                    return Err(KEY_MISMATCH);
                }
                let digest = self.envelope.request_digest()?;
                verify_digest(key, signature, digest.bytes()).map_err(verification)?;
            }
        }
        commitment::check_binding(
            binding,
            &EvaluatorBinding {
                frozen: self.opened.frozen,
                evaluator: frozen.entry.evaluator,
                grant: frozen.entry.grant,
                key_version: frozen.entry.key_version,
            },
        )?;
        crate::evidence::check_frozen_evaluator(
            &region,
            &frozen,
            self.opened.frozen.epoch,
            *self
                .opened
                .state
                .feature_sections
                .guest_index(Section::ReputationAdmission.index()),
        )?;
        Ok(frozen)
    }
    /// Role replay request under the evaluator slot bound to the evaluator owner.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn request(&self, owner: PrincipalId) -> CodecResult<ReplayRequest> {
        let replay = &self.opened.state.control.replay;
        for index in 0..MAX_EVALUATORS {
            let slot = ActorSlot::evaluator(index)?;
            if let Some(actor) = replay.actor(slot).filter(|a| a.principal == owner) {
                return ReplayRequest::from_envelope(slot, actor.authority_version, self.envelope);
            }
        }
        Err(NOT_FOUND)
    }
    /// Envelope, authority and binding checks, then the replay decision; `Err(retained)` is an
    /// exact retry.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn admit(
        &self,
        binding: &EvaluatorBinding,
    ) -> CodecResult<Result<(FrozenEvaluator, ReplayRequest), RetainedResult>> {
        self.check_envelope()?;
        let frozen = self.authenticate(binding)?;
        let request = self.request(frozen.entry.owner)?;
        Ok(
            match self
                .opened
                .state
                .control
                .replay
                .check(&request, self.ctx.height)?
            {
                ReplayDecision::AlreadyApplied(retained) => Err(retained),
                ReplayDecision::Apply => Ok((frozen, request)),
            },
        )
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn window(&self, opens: u64, closes: u64) -> CodecResult<()> {
        let origin = self.opened.section.header.origin_height;
        HeightWindow::epoch(origin, self.opened.frozen.epoch, opens, closes)?.check(self.ctx.height)
    }
}

/// Commits `record` at the next revision and evaluator sequence and emits `CommitScore`.
fn commit(
    call: &Call<'_>,
    record: &CommitRecord,
    request: &ReplayRequest,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Outcome> {
    let opened = call.opened;
    let (policy, rest) = scratch.split_at_mut_checked(POLICY_CAP).ok_or(CAPACITY)?;
    let (reports, control) = rest.split_at_mut_checked(REPORTS_CAP).ok_or(CAPACITY)?;
    let epoch = opened.frozen.epoch;
    let revision = opened.state.revision.checked_add(1).ok_or(ARITHMETIC)?;
    let mut section = opened.section;
    section.header.state_revision = revision;
    let policy_len = section.encode(policy)?;
    let current = *opened
        .state
        .feature_sections
        .guest_index(Section::CurrentReports.index());
    let prefix_len = current
        .len()
        .checked_sub(opened.reports.rest().len())
        .ok_or(NON_CANONICAL)?;
    let mut w = Writer::new(reports);
    if prefix_len == 0 {
        w.u64(epoch)?;
        w.u8(0)?;
    } else {
        w.put(current.get(..prefix_len).ok_or(NON_CANONICAL)?)?;
    }
    opened.commits.write_with(epoch, record, &mut w)?;
    let reports_len = w.len();
    let policy = policy.get(..policy_len).ok_or(CAPACITY)?;
    let reports = reports.get(..reports_len).ok_or(CAPACITY)?;
    let with_policy = opened
        .state
        .replace_section(Section::PolicyLifecycle, policy)?;
    let mut candidate = with_policy.with_section(Section::CurrentReports, reports)?;
    let result = record.result()?;
    if candidate.record_success(request, call.ctx.height, result)? != ReplayDecision::Apply
        || candidate.revision != revision
    {
        return Err(NON_CANONICAL);
    }
    check_f01_capacity(policy_len, candidate.encoded_len()?)?;
    let state_len = encode_shared_state(&candidate, next, control)?;
    let frozen = &opened.frozen;
    let event_len = codec::encode_commit_event(
        &CommitAccepted {
            common: EventCommon {
                market: frozen.market,
                epoch,
                config: frozen.config,
                revision,
                request: call.envelope.request_digest()?,
                result,
            },
            evaluator: record.evaluator,
            commitment: record.commitment,
            accepted_height: record.height,
        },
        event,
    )?;
    Ok(Outcome::Committed {
        record: *record,
        revision,
        result,
        state_len,
        event_len,
    })
}

fn commit_score(
    call: &Call<'_>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Outcome> {
    let payload = commitment::decode_commit_score(call.envelope.envelope.payload)?;
    let (frozen, request) = match call.admit(&payload.binding)? {
        Ok(admitted) => admitted,
        Err(retained) => return Ok(Outcome::Retained(retained)),
    };
    if call.opened.section.header.lifecycle == SUSPENDED {
        return Err(F08_MARKET_PAUSED);
    }
    call.window(COMMIT_OPENS, COMMIT_CLOSES)?;
    let opened = call.opened;
    let epoch = opened.frozen.epoch;
    let evaluator = frozen.entry.evaluator;
    if opened.commits.get(epoch, evaluator)?.is_some()
        || opened.reports.get(epoch, evaluator)?.is_some()
    {
        return Err(CONFLICT);
    }
    let record = CommitRecord {
        evaluator,
        commitment: payload.commitment,
        height: call.ctx.height,
        sequence: call.envelope.envelope.sequence,
        grant: frozen.entry.grant,
        key_version: frozen.entry.key_version,
    };
    commit(call, &record, &request, next, scratch, event)
}

fn reveal_score(
    call: &Call<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Outcome> {
    let payload = commitment::decode_reveal_score(call.envelope.envelope.payload)?;
    let binding = payload.report.binding;
    let (frozen, request) = match call.admit(&binding)? {
        Ok(admitted) => admitted,
        Err(retained) => return Ok(Outcome::Retained(retained)),
    };
    call.window(REVEAL_OPENS, REVEAL_CLOSES)?;
    let opened = call.opened;
    let epoch = opened.frozen.epoch;
    let evaluator = frozen.entry.evaluator;
    if opened.reports.get(epoch, evaluator)?.is_some() {
        return Err(CONFLICT);
    }
    let record = opened.commits.get(epoch, evaluator)?.ok_or(NOT_FOUND)?;
    let report = codec::report_digest(&payload.report)?;
    let recomputed = codec::commitment_digest(&binding, report, payload.salt)?;
    commitment::check_commitment(recomputed, record.commitment)?;
    let reveal = RevealAdmission {
        report: SignedReport {
            body: payload.report,
            signature: payload.signature,
        },
        request,
        height: call.ctx.height,
        activity: call.envelope.envelope.sequence,
    };
    match admit_report_with(current, &reveal, next, scratch, event, Some(opened.rewards))
        .map_err(verification)?
    {
        Admission::Applied {
            receipt,
            revision,
            state_len,
            event_len,
        } => Ok(Outcome::Revealed {
            receipt,
            revision,
            state_len,
            event_len,
        }),
        Admission::AlreadyAdmitted(_) => Err(CONFLICT),
        Admission::Retained(retained) => Ok(Outcome::Retained(retained)),
    }
}

/// Applies one `CommitScore` (0x0401) or `RevealScore` (0x0402) request to the committed
/// `current` state, writing the whole next state into `next` (at least `MAX_STATE_BYTES`) and
/// its event into `event` (at least 228 bytes); `scratch` holds at least [`SCRATCH_BYTES`].
/// Dispatch arm: `dispatch::CommitScore | dispatch::RevealScore =>
/// commit_reveal::apply(&ctx, &envelope, current, next, scratch, event)`. A commit is accepted
/// in `[T+64, T+80)` and a reveal in `[T+80, T+96)` of the opened epoch; a reveal is admitted
/// atomically by F03 `admit_report`, whose event is the sole `RevealScore` event.
///
/// # Errors
/// `UNKNOWN_OPERATION`; `NON_CANONICAL` for a malformed payload or an inconsistent committed
/// state; `F04_NO_SCORES`, `F04_SALT_INVALID` and report structure refusals; envelope
/// principal, domain and expiry refusals; `WRONG_MARKET`; `WRONG_EPOCH` (also with no opened
/// epoch), `WRONG_CONFIG` and `WRONG_ROSTER`; `UNAUTHORIZED`, `KEY_MISMATCH`, `BAD_SIGNATURE`,
/// `REVOKED`, `EXPIRED`, `F03_NO_GRANT`, `F03_GRANT_VERSION_CONFLICT` and
/// `F03_KEY_VERSION_CONFLICT` for evaluator authority and the binding; `NOT_FOUND` without a
/// bound evaluator slot or, for a reveal, without an accepted commitment; common replay
/// refusals; `F08_MARKET_PAUSED` for a commit into a suspended market; `WRONG_PHASE` outside the
/// window; `CONFLICT` for a second commitment or a reveal after admission;
/// `F04_COMMIT_MISMATCH`; the F03 admission refusals; `CAPACITY`, `ARITHMETIC` and
/// `F01_CAPACITY_UNAVAILABLE`; `HOST_CAPABILITY` for a host verification failure. On any error
/// `current` is unchanged and the outputs must be discarded.
pub fn apply(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Outcome> {
    let operation = envelope.envelope.operation;
    if operation != dispatch::CommitScore && operation != dispatch::RevealScore {
        return Err(UNKNOWN_OPERATION);
    }
    let opened = opened(current)?;
    let call = Call {
        ctx,
        envelope,
        opened: &opened,
    };
    if operation == dispatch::CommitScore {
        commit_score(&call, next, scratch, event)
    } else {
        reveal_score(&call, current, next, scratch, event)
    }
}

pub struct CanaryEnvelope<'a> {
    pub envelope: codec::Envelope<'a>,
    unsigned: &'a [u8],
}
impl CanaryEnvelope<'_> {
    pub fn request_digest(&self) -> CodecResult<crate::types::RequestDigest> {
        crate::types::RequestDigest::new(
            codec::domain_hash("PAXAI/request/v1", self.unsigned)?.bytes(),
        )
    }
}

pub fn is_canary_score(input: &[u8]) -> bool {
    matches!(input.get(8..10), Some([4, 1] | [4, 2]))
        && input.get(codec::ENVELOPE_PREFIX_BYTES..codec::ENVELOPE_PREFIX_BYTES + 8)
            == Some(crate::canary::CANARY_TASK_KIND.as_slice())
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn validate_canary_envelope(e: &codec::Envelope<'_>) -> CodecResult<()> {
    if e.sequence == 0 || e.config == 0 || e.expiry == 0 || e.roster == Presence::Absent {
        return Err(NON_CANONICAL);
    }
    if e.authentication != Authentication::Native {
        return Err(UNAUTHORIZED);
    }
    e.operation.validate_payload_length(e.payload.len())?;
    if e.operation == dispatch::CommitScore {
        crate::canary::CanaryTask::decode(e.payload)?;
    } else if e.operation == dispatch::RevealScore {
        crate::canary::CanaryReveal::decode(e.payload)?;
    } else {
        return Err(UNKNOWN_OPERATION);
    }
    Ok(())
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_canary_envelope(e: &codec::Envelope<'_>, out: &mut [u8]) -> CodecResult<usize> {
    validate_canary_envelope(e)?;
    codec::encode_envelope_fields(e, out)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_canary_envelope(input: &[u8]) -> CodecResult<CanaryEnvelope<'_>> {
    let (envelope, unsigned) = codec::decode_envelope_fields(input, true)?;
    validate_canary_envelope(&envelope)?;
    Ok(CanaryEnvelope { envelope, unsigned })
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn canary_score_payload(
    record: &crate::canary::CanaryCommitment,
    operation: dispatch::Operation,
    sequence: u64,
) -> CodecResult<([u8; COMMIT_RESULT_BYTES], usize)> {
    let mut bytes = [0; COMMIT_RESULT_BYTES];
    if operation == dispatch::CommitScore {
        let mut w = Writer::new(&mut bytes);
        crate::codec::write_fields!(w;
            u8: 2;
            id32: record.host.as_bytes();
            id32: record.id.as_bytes();
            u64: record.committed_height;
            u64: sequence;
        )?;
        Ok((bytes, COMMIT_RESULT_BYTES))
    } else if operation == dispatch::RevealScore {
        bytes
            .guest_index_mut(..32)
            .guest_copy_from_slice(record.id.as_bytes());
        Ok((bytes, 32))
    } else {
        Err(UNKNOWN_OPERATION)
    }
}

pub fn apply_canary_score(
    ctx: &CallContext,
    score: &CanaryEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<crate::canary::Outcome> {
    let e = &score.envelope;
    let state = decode_shared_state(current)?;
    let section = crate::canary::check_call(ctx, e, &state)?;
    if e.actor != section.header.owner_principal {
        return Err(UNAUTHORIZED);
    }
    let actor = state
        .control
        .replay
        .actor(ActorSlot::OWNER)
        .ok_or(NOT_FOUND)?;
    let digest = score.request_digest()?;
    let request = ReplayRequest {
        slot: ActorSlot::OWNER,
        principal: e.actor,
        authority_version: actor.authority_version,
        sequence: e.sequence,
        request_id: e.request,
        digest,
        expiry_height: e.expiry,
    };
    if let ReplayDecision::AlreadyApplied(retained) =
        state.control.replay.check(&request, ctx.height)?
    {
        return Ok(crate::canary::Outcome::Retained(retained));
    }
    let record = if e.operation == dispatch::CommitScore {
        crate::canary::commit_task(
            ctx,
            e,
            &state,
            crate::canary::CanaryTask::decode(e.payload)?,
        )?
    } else if e.operation == dispatch::RevealScore {
        crate::canary::reveal_task(
            ctx,
            e,
            &state,
            crate::canary::CanaryReveal::decode(e.payload)?,
        )?
    } else {
        return Err(UNKNOWN_OPERATION);
    };
    let (payload, n) = canary_score_payload(&record, e.operation, e.sequence)?;
    crate::canary::persist(
        ctx,
        e,
        digest,
        current,
        record,
        Some(&request),
        payload.guest_index(..n),
        next,
        scratch,
        event,
    )
}
