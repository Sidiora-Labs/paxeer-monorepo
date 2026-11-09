use crate::types::GuestIndex as _;
use crate::{
    codec::{self, Envelope, EventCommon, Reader, ValidatedEnvelope, Writer},
    dispatch,
    epoch::captured_roster,
    errors::{
        CodecResult, ARITHMETIC, BAD_VERSION, CAPACITY, CONFLICT, F04_COMMIT_MISMATCH,
        F11_BOND_INSUFFICIENT, F11_CHALLENGE_WINDOW_CLOSED, NON_CANONICAL, NOT_FOUND, UNAUTHORIZED,
        WRONG_CONFIG, WRONG_EPOCH, WRONG_ROSTER,
    },
    evaluators::{admission::ChallengeRegion, codec::verify_digest, model::VerificationError},
    evidence::SealRegion,
    policy::{RoomProfile, ROOM_PROFILE_BYTES},
    registry::check_f01_capacity,
    registry_ops::{CallContext, PolicySection},
    state::{
        decode_shared_state, encode_shared_state, HeightWindow, ReplayDecision, ReplayRequest,
        RetainedResult, Section, SharedState,
    },
    types::{
        AccountId, Digest32, FrozenBinding, Presence, PrincipalId, RequestDigest, Signature64,
        WorkerId,
    },
    value_adapter::ValueAction,
};

pub const TOP_LOGITS: usize = 8;
pub const LOGIT_FRACTION_BITS: u32 = 16;
pub type CanaryLogits = [i32; TOP_LOGITS];
pub const CANARY_TASK_KIND: &[u8; 8] = b"CANARY2\0";
pub const CANARY_COMMIT_BYTES: usize = 8 + 5 * 32 + 8 + ROOM_PROFILE_BYTES + 64;
pub const CANARY_REVEAL_BYTES: usize = 104;
pub const CANARY_CAPACITY: usize = 16;
const RECORD_BYTES: usize = 290 + CANARY_COMMIT_BYTES;
const REGION_MAGIC: &[u8; 8] = b"CANARYS2";
const POLICY_CAP: usize = Section::PolicyLifecycle.payload_cap();
const CONTROL_CAP: usize = Section::Control.payload_cap();
pub const SCRATCH_BYTES: usize = POLICY_CAP + 2 * CONTROL_CAP;

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct CanaryRoomProfile {
    pub room: RoomProfile,
}
impl CanaryRoomProfile {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn validate(&self) -> CodecResult<()> {
        self.room.validate()
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct CanaryTask {
    pub deal: Digest32,
    pub lap: u64,
    pub peer: WorkerId,
    pub prompt_digest: Digest32,
    pub expected_logits_digest: Digest32,
    pub answer_digest: Digest32,
    pub profile: CanaryRoomProfile,
    pub peer_signature: Signature64,
}
impl CanaryTask {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self) -> CodecResult<[u8; CANARY_COMMIT_BYTES]> {
        self.profile.validate()?;
        let mut out = [0; CANARY_COMMIT_BYTES];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w;
            put: CANARY_TASK_KIND;
            id32: self.deal.as_bytes();
            u64: self.lap;
            id32: self.peer.as_bytes();
            id32: self.prompt_digest.as_bytes();
            id32: self.expected_logits_digest.as_bytes();
            id32: self.answer_digest.as_bytes();
        )?;
        w.room_profile(&self.profile.room)?;
        w.put(&self.peer_signature.0)?;
        Ok(out)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        if r.take(8)? != CANARY_TASK_KIND {
            return Err(BAD_VERSION);
        }
        let value = Self {
            deal: r.digest32()?,
            lap: r.u64()?,
            peer: r.worker_id()?,
            prompt_digest: r.digest32()?,
            expected_logits_digest: r.digest32()?,
            answer_digest: r.digest32()?,
            profile: CanaryRoomProfile {
                room: r.room_profile()?,
            },
            peer_signature: Signature64(r.fixed()?),
        };
        r.finish()?;
        value.profile.validate()?;
        Ok(value)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn commitment(&self, binding: &FrozenBinding, host: PrincipalId) -> CodecResult<Digest32> {
        let mut out = [0; 160 + 32 + CANARY_COMMIT_BYTES];
        let mut w = Writer::new(&mut out);
        codec::write_frozen_binding(&mut w, binding)?;
        w.put(host.as_bytes())?;
        w.put(&self.encode()?)?;
        let n = w.len();
        codec::domain_hash("PAXAI/canary-commit/v2", out.guest_index(..n))
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn answer_attestation(&self, binding: &FrozenBinding) -> CodecResult<Digest32> {
        let mut out = [0; 160 + 32 + 8 + 32 + 32 + 32];
        let mut w = Writer::new(&mut out);
        codec::write_frozen_binding(&mut w, binding)?;
        crate::codec::write_fields!(w;
            id32: self.deal.as_bytes();
            u64: self.lap;
            id32: self.peer.as_bytes();
            id32: self.prompt_digest.as_bytes();
            id32: self.answer_digest.as_bytes();
        )?;
        let n = w.len();
        codec::domain_hash("PAXAI/canary-answer/v1", out.guest_index(..n))
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_logits(logits: &CanaryLogits) -> [u8; 32] {
    let mut bytes = [0; 32];
    for (chunk, value) in bytes
        .as_chunks_mut::<{ 4 }>()
        .0
        .iter_mut()
        .map(|chunk| chunk.as_mut_slice())
        .zip(logits)
    {
        chunk.guest_copy_from_slice(&value.to_be_bytes());
    }
    bytes
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_logits(bytes: &[u8]) -> CodecResult<CanaryLogits> {
    if bytes.len() != 32 {
        return Err(NON_CANONICAL);
    }
    let mut logits = [0; TOP_LOGITS];
    for (value, chunk) in logits.iter_mut().zip(
        bytes
            .as_chunks::<{ 4 }>()
            .0
            .iter()
            .map(|chunk| chunk.as_slice()),
    ) {
        *value = i32::from_be_bytes(chunk.try_into().map_err(|_| NON_CANONICAL)?);
    }
    Ok(logits)
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn logits_digest(logits: &CanaryLogits) -> CodecResult<Digest32> {
    codec::domain_hash("PAXAI/canary-logits/q16/v1", &encode_logits(logits))
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn relative_tolerance_hit(
    expected: &CanaryLogits,
    answer: &CanaryLogits,
    bps: u16,
) -> CodecResult<bool> {
    if bps > 10_000 {
        return Err(NON_CANONICAL);
    }
    Ok(expected.iter().zip(answer).all(|(&e, &a)| {
        (i64::from(e) - i64::from(a)).unsigned_abs() * 10_000
            <= i64::from(e).unsigned_abs() * u64::from(bps)
    }))
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct CanaryReveal {
    pub commitment: Digest32,
    pub prompt_digest: Digest32,
    pub logits: CanaryLogits,
}
impl CanaryReveal {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self) -> CodecResult<[u8; CANARY_REVEAL_BYTES]> {
        let mut out = [0; CANARY_REVEAL_BYTES];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w;
            put: CANARY_TASK_KIND;
            id32: self.commitment.as_bytes();
            id32: self.prompt_digest.as_bytes();
        )?;
        w.put(&encode_logits(&self.logits))?;
        Ok(out)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        if r.take(8)? != CANARY_TASK_KIND {
            return Err(BAD_VERSION);
        }
        let value = Self {
            commitment: r.digest32()?,
            prompt_digest: r.digest32()?,
            logits: decode_logits(r.take(32)?)?,
        };
        r.finish()?;
        Ok(value)
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum ChallengeStatus {
    Unchallenged = 0,
    Open = 1,
    Hit = 2,
    Miss = 3,
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct CanaryCommitment {
    pub id: Digest32,
    pub epoch: u64,
    pub config: u64,
    pub roster: crate::types::RosterDigest,
    pub host: PrincipalId,
    pub committed_height: u64,
    pub task: CanaryTask,
    pub revealed_height: Option<u64>,
    pub expected_logits: CanaryLogits,
    pub status: ChallengeStatus,
    pub challenger: Option<PrincipalId>,
    pub deposit: u128,
    pub challenged_height: u64,
    pub challenge_request: Option<RequestDigest>,
    pub resolve_request: Option<RequestDigest>,
    pub resolved_height: u64,
}
impl CanaryCommitment {
    pub fn peer_at_fault(&self) -> bool {
        self.status == ChallengeStatus::Miss
    }
    pub fn refundable_deposit(&self) -> u128 {
        if self.status == ChallengeStatus::Hit {
            self.deposit
        } else {
            0
        }
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn encode(&self) -> CodecResult<[u8; RECORD_BYTES]> {
        let mut out = [0; RECORD_BYTES];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w;
            id32: self.id.as_bytes();
            u64: self.epoch;
            u64: self.config;
            id32: self.roster.as_bytes();
            id32: self.host.as_bytes();
            u64: self.committed_height;
        )?;
        w.put(&self.task.encode()?)?;
        w.u8(u8::from(self.revealed_height.is_some()))?;
        w.put(&encode_logits(&self.expected_logits))?;
        w.u64(self.revealed_height.unwrap_or(0))?;
        w.u8(self.status as u8)?;
        w.put(&self.challenger.map_or([0; 32], PrincipalId::bytes))?;
        w.u128(self.deposit)?;
        w.u64(self.challenged_height)?;
        w.put(&self.challenge_request.map_or([0; 32], RequestDigest::bytes))?;
        w.put(&self.resolve_request.map_or([0; 32], RequestDigest::bytes))?;
        w.u64(self.resolved_height)?;
        if w.len() != RECORD_BYTES {
            return Err(NON_CANONICAL);
        }
        Ok(out)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let (id, epoch, config, roster, host, committed_height) = crate::codec::read_values!(&mut r; id = r.digest32()?;
            epoch: U64,
            config: U64,
            roster: RosterDigest,
            host: PrincipalId,
            committed_height: U64,
        );
        let task = CanaryTask::decode(r.take(CANARY_COMMIT_BYTES)?)?;
        let revealed = r.u8()?;
        let expected_logits = decode_logits(r.take(32)?)?;
        let reveal_height = r.u64()?;
        let status = match r.u8()? {
            0 => ChallengeStatus::Unchallenged,
            1 => ChallengeStatus::Open,
            2 => ChallengeStatus::Hit,
            3 => ChallengeStatus::Miss,
            _ => return Err(NON_CANONICAL),
        };
        let challenger = optional_principal(r.fixed()?)?;
        let deposit = r.u128()?;
        let challenged_height = r.u64()?;
        let challenge_request = optional_request(r.fixed()?)?;
        let resolve_request = optional_request(r.fixed()?)?;
        let resolved_height = r.u64()?;
        r.finish()?;
        if revealed > 1
            || config == 0
            || (revealed == 0 && (expected_logits != [0; 8] || reveal_height != 0))
            || (revealed == 1 && logits_digest(&expected_logits)? != task.expected_logits_digest)
            || (status == ChallengeStatus::Unchallenged
                && (challenger.is_some()
                    || deposit != 0
                    || challenge_request.is_some()
                    || challenged_height != 0))
            || (status != ChallengeStatus::Unchallenged
                && (challenger.is_none()
                    || deposit != task.profile.room.minimum_bond
                    || challenge_request.is_none()))
            || (matches!(
                status,
                ChallengeStatus::Unchallenged | ChallengeStatus::Open
            ) && (resolve_request.is_some() || resolved_height != 0))
            || (matches!(status, ChallengeStatus::Hit | ChallengeStatus::Miss)
                && (revealed == 0 || resolve_request.is_none() || resolved_height < reveal_height))
        {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            id,
            epoch,
            config,
            roster,
            host,
            committed_height,
            task,
            revealed_height: if revealed == 1 {
                Some(reveal_height)
            } else {
                None
            },
            expected_logits,
            status,
            challenger,
            deposit,
            challenged_height,
            challenge_request,
            resolve_request,
            resolved_height,
        })
    }
}
fn optional_principal(b: [u8; 32]) -> CodecResult<Option<PrincipalId>> {
    if b == [0; 32] {
        Ok(None)
    } else {
        Ok(Some(PrincipalId::new(b)?))
    }
}
fn optional_request(b: [u8; 32]) -> CodecResult<Option<RequestDigest>> {
    if b == [0; 32] {
        Ok(None)
    } else {
        Ok(Some(RequestDigest::new(b)?))
    }
}

pub struct CanaryRegion<'a> {
    prefix: &'a [u8],
    records: &'a [u8],
    rest: &'a [u8],
}
impl<'a> CanaryRegion<'a> {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(features: &'a [u8]) -> CodecResult<Self> {
        let seals = SealRegion::decode(features)?;
        let challenges = ChallengeRegion::decode(seals.rest())?;
        let bytes = challenges.rest();
        let prefix = features.guest_index(..features.len() - bytes.len());
        if bytes.is_empty() {
            return Ok(Self {
                prefix,
                records: &[],
                rest: &[],
            });
        }
        let mut r = Reader::new(bytes);
        if r.take(8)? != REGION_MAGIC {
            return Err(BAD_VERSION);
        }
        let count = usize::from(r.u8()?);
        if count == 0 || count > CANARY_CAPACITY {
            return Err(NON_CANONICAL);
        }
        let records = r.take(count * RECORD_BYTES)?;
        let rest = bytes.guest_index(r.offset()..);
        for (i, row) in records
            .as_chunks::<{ RECORD_BYTES }>()
            .0
            .iter()
            .map(|chunk| chunk.as_slice())
            .enumerate()
        {
            let record = CanaryCommitment::decode(row)?;
            for previous in records
                .guest_index(..i * RECORD_BYTES)
                .as_chunks::<{ RECORD_BYTES }>()
                .0
                .iter()
                .map(|chunk| chunk.as_slice())
            {
                let previous = CanaryCommitment::decode(previous)?;
                if previous.id == record.id
                    || (previous.epoch == record.epoch
                        && previous.task.deal == record.task.deal
                        && previous.task.lap == record.task.lap
                        && previous.task.peer == record.task.peer)
                {
                    return Err(NON_CANONICAL);
                }
            }
        }
        Ok(Self {
            prefix,
            records,
            rest,
        })
    }
    pub fn records(&self) -> impl Iterator<Item = CodecResult<CanaryCommitment>> + '_ {
        self.records
            .as_chunks::<{ RECORD_BYTES }>()
            .0
            .iter()
            .map(|chunk| chunk.as_slice())
            .map(CanaryCommitment::decode)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn get(&self, id: Digest32) -> CodecResult<Option<CanaryCommitment>> {
        for record in self.records() {
            let record = record?;
            if record.id == id {
                return Ok(Some(record));
            }
        }
        Ok(None)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write(&self, record: &CanaryCommitment, out: &mut [u8]) -> CodecResult<usize> {
        let exists = self.get(record.id)?.is_some();
        let count = self.records.len() / RECORD_BYTES + usize::from(!exists);
        if count > CANARY_CAPACITY {
            return Err(CAPACITY);
        }
        let mut w = Writer::new(out);
        if self.prefix.is_empty() {
            crate::codec::write_fields!(w;
                u64: record.epoch;
                u8: 0;
                u64: record.epoch;
                u16: 0_u16;
            )?;
        } else {
            w.put(self.prefix)?;
            if SealRegion::decode(self.prefix)?.rest().is_empty() {
                w.u64(record.epoch)?;
                w.u16(0)?;
            }
        }
        w.put(REGION_MAGIC)?;
        w.u8(u8::try_from(count).map_err(|_| CAPACITY)?)?;
        for row in self
            .records
            .as_chunks::<{ RECORD_BYTES }>()
            .0
            .iter()
            .map(|chunk| chunk.as_slice())
        {
            if CanaryCommitment::decode(row)?.id == record.id {
                w.put(&record.encode()?)?;
            } else {
                w.put(row)?;
            }
        }
        if !exists {
            w.put(&record.encode()?)?;
        }
        w.put(self.rest)?;
        Ok(w.len())
    }
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn commitment_from_state(current: &[u8], id: Digest32) -> CodecResult<CanaryCommitment> {
    let state = decode_shared_state(current)?;
    CanaryRegion::decode(state.control.feature_bytes)?
        .get(id)?
        .ok_or(NOT_FOUND)
}

pub enum Outcome {
    Applied {
        record: CanaryCommitment,
        revision: u64,
        state_len: usize,
        event_len: usize,
    },
    Retained(RetainedResult),
    AlreadyApplied,
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn check_call<'a>(
    ctx: &CallContext,
    e: &Envelope<'_>,
    state: &SharedState<'a>,
) -> CodecResult<PolicySection<'a>> {
    let section = PolicySection::decode(
        *state
            .feature_sections
            .guest_index(Section::PolicyLifecycle.index()),
    )?;
    e.check_domain(
        ctx.chain,
        ctx.program,
        codec::derive_market(ctx.chain, ctx.program)?,
    )?;
    e.check_expiry(ctx.height)?;
    codec::compare_native_principal(e, ctx.principal)?;
    if section.header.state_revision != state.revision {
        return Err(NON_CANONICAL);
    }
    if section.header.deployment_chain_domain != ctx.chain
        || section.header.program_id != ctx.program
        || section.header.market_id != e.market
    {
        return Err(crate::errors::WRONG_MARKET);
    }
    Ok(section)
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn check_record(e: &Envelope<'_>, record: &CanaryCommitment) -> CodecResult<()> {
    if e.epoch != record.epoch {
        return Err(WRONG_EPOCH);
    }
    if e.config != record.config {
        return Err(WRONG_CONFIG);
    }
    if e.roster != Presence::Present(record.roster) {
        return Err(WRONG_ROSTER);
    }
    Ok(())
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn commit_task(
    ctx: &CallContext,
    e: &Envelope<'_>,
    state: &SharedState<'_>,
    task: CanaryTask,
) -> CodecResult<CanaryCommitment> {
    let section = check_call(ctx, e, state)?;
    if e.actor != section.header.owner_principal {
        return Err(UNAUTHORIZED);
    }
    if section.header.lifecycle != crate::registry_ops::ACTIVE {
        return Err(crate::errors::F08_MARKET_PAUSED);
    }
    let captured = captured_roster(state)?.ok_or(WRONG_EPOCH)?;
    let binding = captured.binding;
    if e.epoch != binding.epoch {
        return Err(WRONG_EPOCH);
    }
    if e.config != binding.config.get() {
        return Err(WRONG_CONFIG);
    }
    if e.roster != Presence::Present(binding.roster) {
        return Err(WRONG_ROSTER);
    }
    crate::commit_reveal::canary_window(section.header.origin_height, e.epoch, false)?
        .check(ctx.height)?;
    let worker = captured
        .workers()
        .iter()
        .find(|w| w.worker == task.peer)
        .ok_or(NOT_FOUND)?;
    verify_digest(
        worker.public_key,
        task.peer_signature,
        task.answer_attestation(&binding)?.bytes(),
    )
    .map_err(|e| match e {
        VerificationError::Application(e) => e,
        #[cfg(target_arch = "wasm32")]
        VerificationError::Host(_) => crate::errors::HOST_CAPABILITY,
    })?;
    let region = CanaryRegion::decode(state.control.feature_bytes)?;
    for previous in region.records() {
        let previous = previous?;
        if previous.task.deal == task.deal {
            if previous.task.profile != task.profile || previous.host != e.actor {
                return Err(CONFLICT);
            }
            if previous.epoch == e.epoch
                && previous.task.lap == task.lap
                && previous.task.peer == task.peer
            {
                return Err(CONFLICT);
            }
        }
    }
    Ok(CanaryCommitment {
        id: task.commitment(&binding, e.actor)?,
        epoch: e.epoch,
        config: e.config,
        roster: binding.roster,
        host: e.actor,
        committed_height: ctx.height,
        task,
        revealed_height: None,
        expected_logits: [0; 8],
        status: ChallengeStatus::Unchallenged,
        challenger: None,
        deposit: 0,
        challenged_height: 0,
        challenge_request: None,
        resolve_request: None,
        resolved_height: 0,
    })
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn reveal_task(
    ctx: &CallContext,
    e: &Envelope<'_>,
    state: &SharedState<'_>,
    reveal: CanaryReveal,
) -> CodecResult<CanaryCommitment> {
    let section = check_call(ctx, e, state)?;
    let mut record = CanaryRegion::decode(state.control.feature_bytes)?
        .get(reveal.commitment)?
        .ok_or(NOT_FOUND)?;
    check_record(e, &record)?;
    if record.host != e.actor || section.header.owner_principal != e.actor {
        return Err(UNAUTHORIZED);
    }
    crate::commit_reveal::canary_window(section.header.origin_height, record.epoch, true)?
        .check(ctx.height)?;
    if record.revealed_height.is_some() {
        return Err(CONFLICT);
    }
    if reveal.prompt_digest != record.task.prompt_digest
        || logits_digest(&reveal.logits)? != record.task.expected_logits_digest
    {
        return Err(F04_COMMIT_MISMATCH);
    }
    record.expected_logits = reveal.logits;
    record.revealed_height = Some(ctx.height);
    Ok(record)
}

pub(crate) fn persist(
    ctx: &CallContext,
    e: &Envelope<'_>,
    digest: RequestDigest,
    current: &[u8],
    record: CanaryCommitment,
    request: Option<&ReplayRequest>,
    payload: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Outcome> {
    let state = decode_shared_state(current)?;
    let mut section = check_call(ctx, e, &state)?;
    let revision = state.revision.checked_add(1).ok_or(ARITHMETIC)?;
    section.header.state_revision = revision;
    let (policy, rest) = scratch.split_at_mut_checked(POLICY_CAP).ok_or(CAPACITY)?;
    let (features, control) = rest.split_at_mut_checked(CONTROL_CAP).ok_or(CAPACITY)?;
    let pn = section.encode(policy)?;
    let n = CanaryRegion::decode(state.control.feature_bytes)?.write(&record, features)?;
    let mut candidate = state.with_section(Section::PolicyLifecycle, policy.guest_index(..pn))?;
    candidate.control.feature_bytes = features.guest_index(..n);
    let result = codec::result_digest(payload)?;
    if let Some(request) = request {
        if candidate.record_success(request, ctx.height, result)? != ReplayDecision::Apply {
            return Err(CONFLICT);
        }
    } else {
        candidate.revision = revision;
    }
    if candidate.revision != revision {
        return Err(NON_CANONICAL);
    }
    check_f01_capacity(pn, candidate.encoded_len()?)?;
    let common = EventCommon {
        market: e.market,
        epoch: record.epoch,
        config: crate::types::Version::new(record.config)?,
        revision,
        request: digest,
        result,
    };
    let event_len = if e.operation == dispatch::CommitScore {
        codec::encode_commit_event(
            &codec::CommitAccepted {
                common,
                evaluator: crate::types::EvaluatorId::new(record.host.bytes())?,
                commitment: crate::types::CommitmentDigest::new(record.id.bytes())?,
                accepted_height: record.committed_height,
            },
            event,
        )?
    } else if e.operation == dispatch::RevealScore {
        codec::encode_reveal_event(
            &codec::RevealScoreEvent {
                common,
                evaluator: crate::types::EvaluatorId::new(record.host.bytes())?,
                report: crate::types::ReportDigest::new(record.id.bytes())?,
                evidence: crate::types::EvidenceRoot::new(record.task.prompt_digest.bytes())?,
                vector_count: TOP_LOGITS as u16,
                admitted_height: ctx.height,
            },
            event,
        )?
    } else {
        codec::encode_event_frame(e.operation, &common, payload, event)?
    };
    let state_len = encode_shared_state(&candidate, next, control)?;
    Ok(Outcome::Applied {
        record,
        revision,
        state_len,
        event_len,
    })
}

#[derive(Clone, Copy)]
pub struct ChallengeLapPayload {
    pub deal: Digest32,
    pub lap: u64,
    pub commitment: Digest32,
    pub deposit: u128,
}
impl ChallengeLapPayload {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self) -> CodecResult<[u8; 88]> {
        let mut out = [0; 88];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w;
            id32: self.deal.as_bytes();
            u64: self.lap;
            id32: self.commitment.as_bytes();
            u128: self.deposit;
        )?;
        Ok(out)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let value = crate::codec::read_record!(&mut r, Self {  }; deal = r.digest32()?;
            lap: U64,
            commitment: Digest32,
            deposit: U128
        );
        r.finish()?;
        Ok(value)
    }
}
#[derive(Clone, Copy)]
pub struct ResolveChallengePayload {
    pub deal: Digest32,
    pub lap: u64,
    pub commitment: Digest32,
    pub answer: CanaryLogits,
}
impl ResolveChallengePayload {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self) -> CodecResult<[u8; 104]> {
        let mut out = [0; 104];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w;
            id32: self.deal.as_bytes();
            u64: self.lap;
            id32: self.commitment.as_bytes();
        )?;
        w.put(&encode_logits(&self.answer))?;
        Ok(out)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let value = Self {
            deal: r.digest32()?,
            lap: r.u64()?,
            commitment: r.digest32()?,
            answer: decode_logits(r.take(32)?)?,
        };
        r.finish()?;
        Ok(value)
    }
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn named_record(
    state: &SharedState<'_>,
    e: &Envelope<'_>,
    deal: Digest32,
    lap: u64,
    id: Digest32,
) -> CodecResult<CanaryCommitment> {
    let record = CanaryRegion::decode(state.control.feature_bytes)?
        .get(id)?
        .ok_or(NOT_FOUND)?;
    check_record(e, &record)?;
    if record.task.deal != deal || record.task.lap != lap {
        return Err(CONFLICT);
    }
    Ok(record)
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn challenge_window(origin: u64, record: &CanaryCommitment) -> CodecResult<HeightWindow> {
    let reveal = crate::commit_reveal::canary_window(origin, record.epoch, true)?;
    Ok(HeightWindow {
        start: record.committed_height,
        end: reveal
            .end
            .checked_add(record.task.profile.room.challenge_window_blocks)
            .ok_or(ARITHMETIC)?,
    })
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn apply_challenge_lap(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Outcome> {
    let e = &envelope.envelope;
    if e.operation != dispatch::ChallengeLap {
        return Err(crate::errors::UNKNOWN_OPERATION);
    }
    let state = decode_shared_state(current)?;
    let section = check_call(ctx, e, &state)?;
    let payload = ChallengeLapPayload::decode(e.payload)?;
    let mut record = named_record(&state, e, payload.deal, payload.lap, payload.commitment)?;
    let digest = envelope.request_digest()?;
    if record.status != ChallengeStatus::Unchallenged {
        return if record.challenge_request == Some(digest) {
            Ok(Outcome::AlreadyApplied)
        } else {
            Err(CONFLICT)
        };
    }
    challenge_window(section.header.origin_height, &record)?
        .check(ctx.height)
        .map_err(|e| {
            if e == crate::errors::WRONG_PHASE {
                F11_CHALLENGE_WINDOW_CLOSED
            } else {
                e
            }
        })?;
    if payload.deposit != record.task.profile.room.minimum_bond {
        return Err(F11_BOND_INSUFFICIENT);
    }
    record.status = ChallengeStatus::Open;
    record.challenger = Some(e.actor);
    record.deposit = payload.deposit;
    record.challenged_height = ctx.height;
    record.challenge_request = Some(digest);
    persist(
        ctx,
        e,
        digest,
        current,
        record,
        None,
        &record.id.bytes(),
        next,
        scratch,
        event,
    )
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn apply_resolve_challenge(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Outcome> {
    let e = &envelope.envelope;
    if e.operation != dispatch::ResolveChallenge {
        return Err(crate::errors::UNKNOWN_OPERATION);
    }
    let state = decode_shared_state(current)?;
    check_call(ctx, e, &state)?;
    let payload = ResolveChallengePayload::decode(e.payload)?;
    let mut record = named_record(&state, e, payload.deal, payload.lap, payload.commitment)?;
    let digest = envelope.request_digest()?;
    if matches!(record.status, ChallengeStatus::Hit | ChallengeStatus::Miss) {
        return if record.resolve_request == Some(digest) {
            Ok(Outcome::AlreadyApplied)
        } else {
            Err(CONFLICT)
        };
    }
    if record.status != ChallengeStatus::Open || record.revealed_height.is_none() {
        return Err(NOT_FOUND);
    }
    if ctx.height < record.revealed_height.ok_or(NOT_FOUND)? {
        return Err(crate::errors::WRONG_PHASE);
    }
    if logits_digest(&payload.answer)? != record.task.answer_digest {
        return Err(F04_COMMIT_MISMATCH);
    }
    record.status = if relative_tolerance_hit(
        &record.expected_logits,
        &payload.answer,
        record.task.profile.room.canary_relative_tolerance_bps,
    )? {
        ChallengeStatus::Hit
    } else {
        ChallengeStatus::Miss
    };
    record.resolve_request = Some(digest);
    record.resolved_height = ctx.height;
    let mut result = [0; 33];
    result
        .guest_index_mut(..32)
        .guest_copy_from_slice(record.id.as_bytes());
    *result.guest_index_mut(32) = record.status as u8;
    persist(
        ctx, e, digest, current, record, None, &result, next, scratch, event,
    )
}

pub fn value_action(
    operation: dispatch::Operation,
    before: &[u8],
    after: &[u8],
) -> CodecResult<ValueAction> {
    if operation != dispatch::ChallengeLap && operation != dispatch::ResolveChallenge {
        return Ok(ValueAction::None);
    }
    let prev = decode_shared_state(before)?;
    let next = decode_shared_state(after)?;
    let prev = CanaryRegion::decode(prev.control.feature_bytes)?;
    let next = CanaryRegion::decode(next.control.feature_bytes)?;
    let mut action = ValueAction::None;
    let mut changed = false;
    for row in next.records() {
        let row = row?;
        let old = prev.get(row.id)?.ok_or(NON_CANONICAL)?;
        if row == old {
            continue;
        }
        if changed {
            return Err(NON_CANONICAL);
        }
        changed = true;
        action = if operation == dispatch::ChallengeLap
            && old.status == ChallengeStatus::Unchallenged
            && row.status == ChallengeStatus::Open
        {
            ValueAction::Fund {
                amount: row.deposit,
            }
        } else if operation == dispatch::ResolveChallenge && old.status == ChallengeStatus::Open {
            match row.status {
                ChallengeStatus::Hit => ValueAction::Pay {
                    recipient: AccountId::new(row.challenger.ok_or(NON_CANONICAL)?.bytes())?,
                    amount: row.deposit,
                },
                ChallengeStatus::Miss => ValueAction::None,
                _ => return Err(NON_CANONICAL),
            }
        } else {
            return Err(NON_CANONICAL);
        };
    }
    if !changed {
        return Err(NON_CANONICAL);
    }
    Ok(action)
}

#[cfg(target_arch = "wasm32")]
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn stage_value(
    operation: dispatch::Operation,
    before: &[u8],
    after: &[u8],
) -> crate::host_adapter::AdapterResult<()> {
    let action = value_action(operation, before, after)?;
    if action == ValueAction::None {
        return Ok(());
    }
    let state = decode_shared_state(after)?;
    let section = PolicySection::decode(
        *state
            .feature_sections
            .guest_index(Section::PolicyLifecycle.index()),
    )?;
    let bound = crate::value_adapter::RewardsAccount::derive(
        section.header.program_id,
        section.header.funding_asset,
    )?;
    if bound.account() != section.header.rewards_account {
        return Err(crate::errors::ACCOUNT_BINDING.into());
    }
    match action {
        ValueAction::Fund { amount } => {
            layerx_program_sdk::transfer::fund_program_account(bound.deposit(amount)?)?
        }
        ValueAction::Pay { recipient, amount } => {
            layerx_program_sdk::transfer::pay_from_program_account(
                bound.payment(recipient, amount)?,
            )?
        }
        ValueAction::None => {}
    }
    Ok(())
}
