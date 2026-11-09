use crate::types::GuestIndex as _;
use crate::{
    codec::{Reader, Writer},
    errors::{CodecResult, BAD_VERSION, CAPACITY, F07_RESOURCE_LIMIT, NON_CANONICAL},
    reputation::{
        ClosureReason, CompletedHistory, HistoryStatus, Observation, ReputationCurrent,
        ReputationState, SegmentKey, CURRENT_BYTES, HISTORY_BYTES, LIMIT, SECTION_CAP,
    },
    types::{Digest32, MarketId, Presence, PrincipalId, Score, Version, WorkerId},
};

/// Encodes one current record.
///
/// # Errors
/// Propagates `ReputationCurrent::validate`'s `NON_CANONICAL` refusal and the writer's `ARITHMETIC`/`CAPACITY` refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_current(value: &ReputationCurrent) -> CodecResult<[u8; CURRENT_BYTES]> {
    value.validate()?;
    let mut out = [0; CURRENT_BYTES];
    let mut w = Writer::new(&mut out);
    w.put(value.worker.as_bytes())?;
    w.put(value.segment.as_bytes())?;
    w.put(value.owner.as_bytes())?;
    w.u64(value.reset_generation.get())?;
    w.u32(value.quality.get())?;
    w.u32(value.qualifying_count)?;
    let mut flags = 0;
    match value.coverage {
        Presence::Present(v) => {
            flags |= 2;
            w.u32(v.get())?;
        }
        Presence::Absent => w.u32(0)?,
    }
    match value.last_applied {
        Presence::Present(e) => {
            flags |= 4;
            w.u64(e)?;
        }
        Presence::Absent => w.u64(0)?,
    }
    match value.last_observed {
        Presence::Present(o) => {
            flags |= 1;
            w.u64(o.epoch)?;
            w.u64(o.height)?;
        }
        Presence::Absent => {
            w.u64(0)?;
            w.u64(0)?;
        }
    }
    w.u64(value.last_transition_height)?;
    match value.previous_segment {
        Presence::Present(d) => w.put(d.as_bytes())?,
        Presence::Absent => w.put(&[0; 32])?,
    }
    w.u8(value.status as u8)?;
    w.u8(flags)?;
    w.u16(0)?;
    Ok(out)
}
/// Decodes one current record.
///
/// # Errors
/// Returns `NON_CANONICAL` when the input is short or trailing, an identity is zero, the status, flags or reserved bytes are not canonical, or the record fails `ReputationCurrent::validate`; `F03_SCORE_RANGE` when a score exceeds the unit; propagates the reader's `ARITHMETIC` refusal.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_current(input: &[u8]) -> CodecResult<ReputationCurrent> {
    let mut r = Reader::new(input);
    let worker = r.worker_id()?;
    let segment = r.digest32()?;
    let owner = r.principal_id()?;
    let reset_generation = r.version()?;
    let quality = r.score()?;
    let qualifying_count = r.u32()?;
    let coverage = r.score()?;
    let applied = r.u64()?;
    let observed = r.u64()?;
    let observed_height = r.u64()?;
    let last_transition_height = r.u64()?;
    let previous = r.fixed::<32>()?;
    let status = HistoryStatus::decode(r.u8()?)?;
    let flags = r.u8()?;
    if flags & !7 != 0
        || flags & 2 == 0 && coverage.get() != 0
        || flags & 4 == 0 && applied != 0
        || flags & 1 == 0 && (observed != 0 || observed_height != 0)
    {
        return Err(NON_CANONICAL);
    }
    r.reserved(2)?;
    r.finish()?;
    let value = ReputationCurrent {
        worker,
        segment,
        owner,
        reset_generation,
        quality,
        qualifying_count,
        coverage: if flags & 2 == 0 {
            Presence::Absent
        } else {
            Presence::Present(coverage)
        },
        last_applied: if flags & 4 == 0 {
            Presence::Absent
        } else {
            Presence::Present(applied)
        },
        last_observed: if flags & 1 == 0 {
            Presence::Absent
        } else {
            Presence::Present(Observation {
                epoch: observed,
                height: observed_height,
            })
        },
        last_transition_height,
        previous_segment: if previous == [0; 32] {
            Presence::Absent
        } else {
            Presence::Present(Digest32::new(previous)?)
        },
        status,
    };
    value.validate()?;
    Ok(value)
}
/// Encodes one completion history row.
///
/// # Errors
/// Propagates `CompletedHistory::validate`'s `NON_CANONICAL` refusal and the writer's `ARITHMETIC`/`CAPACITY` refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_history(value: &CompletedHistory) -> CodecResult<[u8; HISTORY_BYTES]> {
    value.validate()?;
    let mut out = [0; HISTORY_BYTES];
    let mut w = Writer::new(&mut out);
    w.u64(value.epoch)?;
    w.u64(value.execution_height)?;
    w.u64(value.config.get())?;
    w.put(value.result.as_bytes())?;
    w.put(value.root.as_bytes())?;
    w.u8(value.observed_workers)?;
    w.u8(value.total_workers)?;
    w.u8(value.covered_workers)?;
    w.u8(0)?;
    Ok(out)
}
/// Decodes one completion history row.
///
/// # Errors
/// Returns `NON_CANONICAL` when the input is short or trailing, the config or a digest is zero, the reserved byte is nonzero, or the row fails `CompletedHistory::validate`; propagates the reader's `ARITHMETIC` refusal.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_history(input: &[u8]) -> CodecResult<CompletedHistory> {
    let mut r = Reader::new(input);
    let value = CompletedHistory {
        epoch: r.u64()?,
        execution_height: r.u64()?,
        config: r.version()?,
        result: r.digest32()?,
        root: r.digest32()?,
        observed_workers: r.u8()?,
        total_workers: r.u8()?,
        covered_workers: r.u8()?,
    };
    r.reserved(1)?;
    r.finish()?;
    value.validate()?;
    Ok(value)
}
/// Segment digest over the market, worker, binding and reset generation.
///
/// # Errors
/// Propagates `Digest32::new`'s `NON_CANONICAL` refusal for an all-zero digest.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn segment_digest(key: SegmentKey) -> CodecResult<Digest32> {
    Digest32::new(crate::codec::sha256([
        b"PAXAI/reputation-segment/v1\0".as_slice(),
        key.market.as_bytes(),
        key.worker.as_bytes(),
        &key.config.get().to_be_bytes(),
        key.policy.as_bytes(),
        key.model.as_bytes(),
        &key.reset_generation.get().to_be_bytes(),
    ])?)
}
/// Digest closing `value`'s segment for `reason`.
///
/// # Errors
/// Propagates `ReputationCurrent::validate`'s and `Digest32::new`'s `NON_CANONICAL` refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn closure_digest(
    market: MarketId,
    value: &ReputationCurrent,
    reason: ClosureReason,
) -> CodecResult<Digest32> {
    value.validate()?;
    let coverage = match value.coverage {
        Presence::Absent => 0,
        Presence::Present(v) => v.get(),
    };
    let last_applied = match value.last_applied {
        Presence::Absent => 0,
        Presence::Present(e) => e,
    };
    let previous = match value.previous_segment {
        Presence::Absent => [0; 32],
        Presence::Present(d) => d.bytes(),
    };
    Digest32::new(crate::codec::sha256([
        b"PAXAI/reputation-close/v1\0".as_slice(),
        market.as_bytes(),
        value.worker.as_bytes(),
        value.segment.as_bytes(),
        &value.reset_generation.get().to_be_bytes(),
        &value.quality.get().to_be_bytes(),
        &value.qualifying_count.to_be_bytes(),
        &coverage.to_be_bytes(),
        &last_applied.to_be_bytes(),
        &[reason as u8],
        &previous,
    ])?)
}
/// Reputation root over `records` at `epoch`.
///
/// # Errors
/// Returns `CAPACITY` when there are more than `LIMIT` records; `NON_CANONICAL` when workers are not strictly ascending; propagates `encode_current`'s and `Digest32::new`'s refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn reputation_root(
    market: MarketId,
    epoch: u64,
    records: &[ReputationCurrent],
) -> CodecResult<Digest32> {
    if records.len() > LIMIT {
        return Err(CAPACITY);
    }
    let mut bytes = [0; b"PAXAI/reputation-root/v1\0".len() + 32 + 8 + 2 + LIMIT * CURRENT_BYTES];
    let mut h = Writer::new(&mut bytes);
    h.put(b"PAXAI/reputation-root/v1\0")?;
    h.put(market.as_bytes())?;
    h.u64(epoch)?;
    h.u16(u16::try_from(records.len()).map_err(|_| CAPACITY)?)?;
    let mut previous = None;
    for record in records {
        if previous.is_some_and(|w| w >= record.worker) {
            return Err(NON_CANONICAL);
        }
        h.put(&encode_current(record)?)?;
        previous = Some(record.worker);
    }
    let length = h.len();
    Digest32::new(crate::codec::sha256([bytes.guest_index(..length)])?)
}
#[allow(
    dead_code,
    reason = "crate-internal F07 primitive: its F05/F06 bridge caller is not in tree; tests/reputation_vectors.rs drives it via #[path]"
)]
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn state_root(state: &ReputationState, epoch: u64) -> CodecResult<Digest32> {
    // During staging, records may already refer to the pending completion.
    let mut bytes = [0; b"PAXAI/reputation-root/v1\0".len() + 32 + 8 + 2 + LIMIT * CURRENT_BYTES];
    let mut h = Writer::new(&mut bytes);
    h.put(b"PAXAI/reputation-root/v1\0")?;
    h.put(state.market.as_bytes())?;
    h.u64(epoch)?;
    h.u16(u16::from(state.current_len))?;
    let mut previous = None;
    for record in state.records() {
        if previous.is_some_and(|w| w >= record.worker) {
            return Err(NON_CANONICAL);
        }
        h.put(&encode_current(record)?)?;
        previous = Some(record.worker);
    }
    let length = h.len();
    Digest32::new(crate::codec::sha256([bytes.guest_index(..length)])?)
}
// F07 section framing: RP07, schema:u16, counts:u8/u8, market32,
// completed-presence:u8, completed-epoch:u64, fifteen reserved zero bytes.
/// Encodes the F07 section into `output` and returns the written length.
///
/// # Errors
/// Propagates `ReputationState::encoded_len`'s `CAPACITY`/`NON_CANONICAL`/`F07_RESOURCE_LIMIT` refusals; returns `CAPACITY` when `output` is shorter than the section; propagates the record encoders' and the writer's refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_section(state: &ReputationState, output: &mut [u8]) -> CodecResult<usize> {
    let size = state.encoded_len()?;
    if output.len() < size {
        return Err(CAPACITY);
    }
    let mut w = Writer::new(output);
    w.put(b"RP07")?;
    w.u16(1)?;
    w.u8(state.current_len)?;
    w.u8(state.history_len)?;
    w.put(state.market.as_bytes())?;
    match state.completed_through {
        Presence::Absent => {
            w.u8(0)?;
            w.u64(0)?;
        }
        Presence::Present(e) => {
            w.u8(1)?;
            w.u64(e)?;
        }
    }
    w.put(&[0; 15])?;
    for value in state.records() {
        w.put(&encode_current(value)?)?;
    }
    for value in state.completed() {
        w.put(&encode_history(value)?)?;
    }
    Ok(w.len())
}
/// Decodes and validates an F07 section.
///
/// # Errors
/// Returns `F07_RESOURCE_LIMIT` when the input exceeds `SECTION_CAP`; `NON_CANONICAL` on a bad magic, presence, completed epoch, reserved byte or trailing input; `BAD_VERSION` when the schema is not 1; `CAPACITY` when a count exceeds `LIMIT`; propagates the record decoders' and `ReputationState::validate`'s refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_section(input: &[u8]) -> CodecResult<ReputationState> {
    if input.len() > SECTION_CAP {
        return Err(F07_RESOURCE_LIMIT);
    }
    let mut r = Reader::new(input);
    if r.fixed::<4>()? != *b"RP07" {
        return Err(NON_CANONICAL);
    }
    if r.u16()? != 1 {
        return Err(BAD_VERSION);
    }
    let current_len = r.u8()?;
    let history_len = r.u8()?;
    if current_len as usize > LIMIT || history_len as usize > LIMIT {
        return Err(CAPACITY);
    }
    let market = r.market_id()?;
    let present = r.boolean()?;
    let completed_epoch = r.u64()?;
    if !present && completed_epoch != 0 {
        return Err(NON_CANONICAL);
    }
    r.reserved(15)?;
    let mut state = ReputationState::new(market);
    state.current_len = current_len;
    state.history_len = history_len;
    state.completed_through = if present {
        Presence::Present(completed_epoch)
    } else {
        Presence::Absent
    };
    for slot in state.current.iter_mut().take(current_len as usize) {
        *slot = Some(decode_current(r.take(CURRENT_BYTES)?)?);
    }
    for slot in state.history.iter_mut().take(history_len as usize) {
        *slot = Some(decode_history(r.take(HISTORY_BYTES)?)?);
    }
    r.finish()?;
    state.validate()?;
    Ok(state)
}
