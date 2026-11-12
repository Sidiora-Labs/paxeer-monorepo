use crate::types::GuestIndex as _;
use crate::{
    codec::{self, Reader, ValidatedEnvelope, Writer},
    dispatch,
    errors::*,
    evidence::{self, SealRegion},
    registry::market_clock,
    registry_ops::{ACTIVE, CallContext},
    rooms::{DEAL_CAPACITY, MAX_DEAL_PEERS, RoomBook, RoomState},
    state::{self, ActorSlot, ReplayDecision, ReplayRequest, RetainedResult, Section},
    types::{Digest32, Presence, RosterDigest, Signature64, Version},
};

pub const LAP_BATCH_MAX: usize = 64;
pub const BATCH_BYTES: usize = 96;
pub const SETTLE_FIXED_BYTES: usize = BATCH_BYTES + 2;
pub const SCRATCH_BYTES: usize =
    Section::PolicyLifecycle.payload_cap() + 2 * Section::Control.payload_cap();
pub(crate) const REGION_MAGIC: &[u8; 8] = b"LAPSETT1";
const SETTLEMENT_BYTES: usize = BATCH_BYTES + 32 + 8 + 8 + 32 + 16 + 8 * MAX_DEAL_PEERS;
const RECEIPT_FIXED_BYTES: usize = 32 + 8 * 4 + 1 + 64 + 4;

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct LapReceipt<'a> {
    pub deal_id: Digest32,
    pub lap_index: u64,
    pub prompt_tokens: u64,
    pub generated_tokens: u64,
    pub cumulative_tokens: u64,
    pub layer_range_digests: &'a [Digest32],
    pub host_signature: Signature64,
    pub peer_countersignatures: u32,
}
impl LapReceipt<'_> {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn validate(&self) -> CodecResult<()> {
        let peers = self.layer_range_digests.len();
        if peers == 0
            || peers > MAX_DEAL_PEERS
            || self
                .prompt_tokens
                .checked_add(self.generated_tokens)
                .ok_or(ARITHMETIC)?
                > self.cumulative_tokens
            || self.host_signature.0 == [0; 64]
            || (peers < 32 && self.peer_countersignatures >> peers != 0)
        {
            return Err(NON_CANONICAL);
        }
        Ok(())
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        self.validate()?;
        let mut w = Writer::new(out);
        w.put(self.deal_id.as_bytes())?;
        for n in [
            self.lap_index,
            self.prompt_tokens,
            self.generated_tokens,
            self.cumulative_tokens,
        ] {
            w.u64(n)?;
        }
        w.u8(self.layer_range_digests.len() as u8)?;
        for digest in self.layer_range_digests {
            w.put(digest.as_bytes())?;
        }
        w.put(&self.host_signature.0)?;
        w.u32(self.peer_countersignatures)?;
        Ok(w.len())
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn signing_digest(&self) -> CodecResult<Digest32> {
        let mut unsigned = *self;
        unsigned.host_signature = Signature64([1; 64]);
        let mut out = [0; RECEIPT_FIXED_BYTES + 32 * MAX_DEAL_PEERS];
        let n = unsigned.encode(&mut out)?;
        let signature_at = n - 68;
        out.guest_index_mut(signature_at..signature_at + 64).fill(0);
        codec::domain_hash("PAXAI/lap-signing/v1", out.guest_index(..n))
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn digest(&self) -> CodecResult<Digest32> {
        let mut out = [0; RECEIPT_FIXED_BYTES + 32 * MAX_DEAL_PEERS];
        let n = self.encode(&mut out)?;
        codec::domain_hash("PAXAI/lap-receipt/v1", out.guest_index(..n))
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ReceiptBatch {
    pub deal_id: Digest32,
    pub first_lap_index: u64,
    pub last_lap_index: u64,
    pub cumulative_tokens: u64,
    pub batch_digest: Digest32,
    pub countersign_quorum: u32,
    pub missing_countersigners: u32,
}
impl ReceiptBatch {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn validate(&self) -> CodecResult<()> {
        if self
            .last_lap_index
            .checked_sub(self.first_lap_index)
            .ok_or(NON_CANONICAL)?
            >= LAP_BATCH_MAX as u64
        {
            return Err(NON_CANONICAL);
        }
        Ok(())
    }
    pub fn from_receipts(receipts: &[LapReceipt<'_>]) -> CodecResult<Self> {
        if receipts.is_empty() || receipts.len() > LAP_BATCH_MAX {
            return Err(CAPACITY);
        }
        let first = receipts.first().ok_or(NON_CANONICAL)?;
        let mut nodes = [[0; 32]; LAP_BATCH_MAX];
        let mut bitmap = u32::MAX;
        let mut previous: Option<u64> = None;
        for (i, receipt) in receipts.iter().enumerate() {
            receipt.validate()?;
            if receipt.deal_id != first.deal_id
                || receipt.lap_index != first.lap_index.checked_add(i as u64).ok_or(ARITHMETIC)?
                || receipt.layer_range_digests.len() != first.layer_range_digests.len()
            {
                return Err(NON_CANONICAL);
            }
            if let Some(tokens) = previous {
                let delta = receipt
                    .prompt_tokens
                    .checked_add(receipt.generated_tokens)
                    .ok_or(ARITHMETIC)?;
                if receipt.cumulative_tokens != tokens.checked_add(delta).ok_or(ARITHMETIC)? {
                    return Err(CONFLICT);
                }
            }
            previous = Some(receipt.cumulative_tokens);
            *nodes.guest_index_mut(i) = receipt.digest()?.bytes();
            bitmap &= receipt.peer_countersignatures;
        }
        let mut count = receipts.len();
        while count > 1 {
            for i in 0..count.div_ceil(2) {
                let mut pair = [0; 64];
                pair.guest_index_mut(..32)
                    .guest_copy_from_slice(nodes.guest_index(i * 2));
                pair.guest_index_mut(32..)
                    .guest_copy_from_slice(nodes.guest_index((i * 2 + 1).min(count - 1)));
                *nodes.guest_index_mut(i) = codec::domain_hash("PAXAI/lap-node/v1", &pair)?.bytes();
            }
            count = count.div_ceil(2);
        }
        Ok(Self {
            deal_id: first.deal_id,
            first_lap_index: first.lap_index,
            last_lap_index: receipts.last().ok_or(NON_CANONICAL)?.lap_index,
            cumulative_tokens: previous.ok_or(NON_CANONICAL)?,
            batch_digest: Digest32::new(*nodes.guest_index(0))?,
            countersign_quorum: bitmap.count_ones(),
            missing_countersigners: !bitmap & peer_mask(first.layer_range_digests.len()),
        })
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self) -> CodecResult<[u8; BATCH_BYTES]> {
        self.validate()?;
        let mut out = [0; BATCH_BYTES];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w;
            id32: self.deal_id.as_bytes();
            u64: self.first_lap_index;
            u64: self.last_lap_index;
            u64: self.cumulative_tokens;
            id32: self.batch_digest.as_bytes();
            u32: self.countersign_quorum;
            u32: self.missing_countersigners;
        )?;
        Ok(out)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn summary_digest(&self) -> CodecResult<Digest32> {
        codec::domain_hash("PAXAI/lap-batch/v1", &self.encode()?)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let value = crate::codec::read_record!(&mut r, Self {  }; deal_id = r.digest32()?;
            first_lap_index: U64,
            last_lap_index: U64,
            cumulative_tokens: U64,
            batch_digest: Digest32,
            countersign_quorum: U32, missing_countersigners: U32
        );
        r.finish()?;
        value.validate()?;
        Ok(value)
    }
}

pub struct SettleLapsRequest<'a> {
    pub batch: ReceiptBatch,
    pub artifact_manifest: &'a [u8],
}
impl SettleLapsRequest<'_> {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        if self.artifact_manifest.len() > evidence::MAX_MANIFEST_BYTES {
            return Err(CAPACITY);
        }
        let mut w = Writer::new(out);
        w.put(&self.batch.encode()?)?;
        w.u16(self.artifact_manifest.len() as u16)?;
        w.put(self.artifact_manifest)?;
        Ok(w.len())
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct LapSettlement {
    pub batch: ReceiptBatch,
    pub task_set: Digest32,
    pub epoch: u64,
    pub config: u64,
    pub roster: RosterDigest,
    pub settled_amount_owed: u128,
    pub slashed_laps: [u64; MAX_DEAL_PEERS],
}
impl LapSettlement {
    pub fn meter_reading(&self) -> u64 {
        self.batch.cumulative_tokens
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self) -> CodecResult<[u8; SETTLEMENT_BYTES]> {
        let mut out = [0; SETTLEMENT_BYTES];
        let mut w = Writer::new(&mut out);
        w.put(&self.batch.encode()?)?;
        crate::codec::write_fields!(w;
            id32: self.task_set.as_bytes();
            u64: self.epoch;
            u64: self.config;
            id32: self.roster.as_bytes();
            u128: self.settled_amount_owed;
        )?;
        for mask in self.slashed_laps {
            w.u64(mask)?;
        }
        Ok(out)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let mut record = Self {
            batch: ReceiptBatch::decode(r.take(BATCH_BYTES)?)?,
            task_set: r.digest32()?,
            epoch: r.u64()?,
            config: r.u64()?,
            roster: r.roster_digest()?,
            settled_amount_owed: r.u128()?,
            slashed_laps: [0; MAX_DEAL_PEERS],
        };
        for mask in &mut record.slashed_laps {
            *mask = r.u64()?;
        }
        Version::new(record.config)?;
        r.finish()?;
        Ok(record)
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn region_length(bytes: &[u8]) -> CodecResult<usize> {
    let length = committed_region_length(bytes)?;
    if length == 0 {
        return Ok(0);
    }
    let rows = bytes.get(9..length).ok_or(NON_CANONICAL)?;
    let mut previous = None;
    for row in rows
        .as_chunks::<{ SETTLEMENT_BYTES }>()
        .0
        .iter()
        .map(|chunk| chunk.as_slice())
    {
        let record = LapSettlement::decode(row)?;
        if previous.is_some_and(|id| id >= record.batch.deal_id) {
            return Err(NON_CANONICAL);
        }
        previous = Some(record.batch.deal_id);
    }
    Ok(length)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn committed_region_length(bytes: &[u8]) -> CodecResult<usize> {
    if !bytes.starts_with(REGION_MAGIC) {
        return Ok(0);
    }
    let count = usize::from(*bytes.get(8).ok_or(NON_CANONICAL)?);
    if count == 0 || count > DEAL_CAPACITY {
        return Err(NON_CANONICAL);
    }
    let length = 9 + count * SETTLEMENT_BYTES;
    bytes.get(..length).ok_or(NON_CANONICAL)?;
    Ok(length)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn rows(bytes: &[u8]) -> CodecResult<&[u8]> {
    let n = region_length(bytes)?;
    if n == 0 {
        return Ok(&[]);
    }
    if n != bytes.len() {
        return Err(NON_CANONICAL);
    }
    Ok(bytes.guest_index(9..))
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn get(bytes: &[u8], deal: Digest32) -> CodecResult<Option<LapSettlement>> {
    for row in rows(bytes)?
        .as_chunks::<{ SETTLEMENT_BYTES }>()
        .0
        .iter()
        .map(|chunk| chunk.as_slice())
    {
        let record = LapSettlement::decode(row)?;
        if record.batch.deal_id == deal {
            return Ok(Some(record));
        }
    }
    Ok(None)
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn settlement_from_state(current: &[u8], deal: Digest32) -> CodecResult<Option<LapSettlement>> {
    let state = state::decode_shared_state(current)?;
    settlement_from_shared(&state, deal)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn settlement_from_shared(
    state: &state::SharedState<'_>,
    deal: Digest32,
) -> CodecResult<Option<LapSettlement>> {
    get(
        SealRegion::committed(state.control.feature_bytes)?.lap_settlements(),
        deal,
    )
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn write_region(bytes: &[u8], record: &LapSettlement, out: &mut [u8]) -> CodecResult<usize> {
    let rows = rows(bytes)?;
    let exists = get(bytes, record.batch.deal_id)?.is_some();
    let count = rows.len() / SETTLEMENT_BYTES + usize::from(!exists);
    if count > DEAL_CAPACITY {
        return Err(CAPACITY);
    }
    let mut w = Writer::new(out);
    w.put(REGION_MAGIC)?;
    w.u8(count as u8)?;
    let mut inserted = false;
    for row in rows
        .as_chunks::<{ SETTLEMENT_BYTES }>()
        .0
        .iter()
        .map(|chunk| chunk.as_slice())
    {
        let id = LapSettlement::decode(row)?.batch.deal_id;
        if !inserted && id >= record.batch.deal_id {
            w.put(&record.encode()?)?;
            inserted = true;
        }
        if id != record.batch.deal_id {
            w.put(row)?;
        }
    }
    if !inserted {
        w.put(&record.encode()?)?;
    }
    Ok(w.len())
}

pub enum Outcome {
    Applied {
        revision: u64,
        state_len: usize,
        event_len: usize,
    },
    Retained(RetainedResult),
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn apply_settle_laps(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Outcome> {
    let e = &envelope.envelope;
    if e.operation != dispatch::SettleLaps {
        return Err(UNKNOWN_OPERATION);
    }
    let state = state::decode_shared_state(current)?;
    let policy = crate::canary::check_call(ctx, e, &state)?;
    if policy.header.lifecycle != ACTIVE {
        return Err(WRONG_PHASE);
    }
    if e.epoch != market_clock(policy.header.origin_height, ctx.height)?.epoch {
        return Err(WRONG_EPOCH);
    }
    if e.config != policy.header.active_config_version {
        return Err(WRONG_CONFIG);
    }
    let mut r = Reader::new(e.payload);
    let batch = ReceiptBatch::decode(r.take(BATCH_BYTES)?)?;
    let manifest_len = usize::from(r.u16()?);
    let manifest = r.take(manifest_len)?;
    r.finish()?;
    let book = RoomBook::committed(state.room_market()?.1)?;
    let deal = book.deal(batch.deal_id)?.ok_or(NOT_FOUND)?;
    let room = book.room(deal.room_id)?.ok_or(F11_ROOM_UNKNOWN)?;
    if room.state != RoomState::Open {
        return Err(F11_ROOM_CLOSED);
    }
    if e.actor != room.host_key {
        return Err(UNAUTHORIZED);
    }
    if e.roster != Presence::Present(deal.roster_digest) {
        return Err(F11_ROSTER_DIGEST_MISMATCH);
    }
    let slot = if room.host_key == policy.header.owner_principal {
        ActorSlot::OWNER
    } else {
        let workers = crate::workers::WorkerTable::decode(
            crate::evaluators::authority::split_identity_section(
                state.section(Section::IdentityRoster)?,
            )?
            .0,
        )?;
        let worker_slot = workers
            .iter()
            .find(|w| w.owner == room.host_key)
            .map(|worker| worker.slot)
            .ok_or(UNAUTHORIZED)?;
        ActorSlot::worker(usize::from(worker_slot))?
    };
    let authority = state.control.replay.actor(slot).ok_or(UNAUTHORIZED)?;
    let request = ReplayRequest::from_envelope(slot, authority.authority_version, envelope)?;
    if let ReplayDecision::AlreadyApplied(retained) =
        state.control.replay.check(&request, ctx.height)?
    {
        return Ok(Outcome::Retained(retained));
    }
    let peers = deal.layers.iter().flatten().count() as u32;
    if batch.countersign_quorum < peers.saturating_sub(1)
        || batch.countersign_quorum > peers
        || batch.missing_countersigners & !peer_mask(peers as usize) != 0
        || batch.missing_countersigners.count_ones() + batch.countersign_quorum != peers
    {
        return Err(UNAUTHORIZED);
    }
    let region = SealRegion::decode(state.control.feature_bytes)?;
    let previous = get(region.lap_settlements(), batch.deal_id)?;
    if let Some(previous) = previous {
        if batch.first_lap_index <= previous.batch.last_lap_index
            || batch.last_lap_index <= previous.batch.last_lap_index
            || batch.cumulative_tokens < previous.meter_reading()
        {
            return Err(CONFLICT);
        }
    }
    let task_set = evidence::require_sealed_artifact_chunk_root(&state, e.epoch, manifest, &batch)?;
    let record = LapSettlement {
        batch,
        task_set,
        epoch: e.epoch,
        config: e.config,
        roster: deal.roster_digest,
        slashed_laps: [0; MAX_DEAL_PEERS],
        settled_amount_owed: u128::from(batch.cumulative_tokens)
            .checked_mul(deal.agreed_price_per_thousand_tokens)
            .ok_or(ARITHMETIC)?
            / 1000,
    };
    let (policy_out, rest) = scratch
        .split_at_mut_checked(Section::PolicyLifecycle.payload_cap())
        .ok_or(CAPACITY)?;
    let (features, control) = rest
        .split_at_mut_checked(Section::Control.payload_cap())
        .ok_or(CAPACITY)?;
    let prefix_len = region.write_lap_prefix(features)?;
    let n = write_region(
        region.lap_settlements(),
        &record,
        features.guest_index_mut(prefix_len..),
    )?;
    let mut w = Writer::new(features.guest_index_mut(prefix_len + n..));
    w.put(region.rest())?;
    let feature_len = prefix_len + n + w.len();
    let settlement = state.section(Section::SettlementClaims)?;
    let mut candidate = state;
    candidate.control.feature_bytes = features.guest_index(..feature_len);
    let record_bytes = record.encode()?;
    match crate::rewards::finish_room_transition(
        ctx.height,
        envelope,
        candidate,
        policy,
        settlement,
        &record_bytes,
        Some(&request),
        policy_out,
        control,
        next,
        event,
    )? {
        crate::rooms::Outcome::Applied {
            revision,
            state_len,
            event_len,
        } => Ok(Outcome::Applied {
            revision,
            state_len,
            event_len,
        }),
        _ => Err(NON_CANONICAL),
    }
}

fn peer_mask(count: usize) -> u32 {
    u32::MAX >> (MAX_DEAL_PEERS - count)
}

impl LapSettlement {
    pub(crate) fn was_slashed(&self, peer: usize, lap: u64) -> bool {
        lap.checked_sub(self.batch.first_lap_index)
            .filter(|offset| *offset < 64)
            .is_some_and(|offset| self.slashed_laps.guest_index(peer) & (1u64 << offset) != 0)
    }
}

pub(crate) fn record_slash(
    state: &state::SharedState<'_>,
    mut record: LapSettlement,
    peer: usize,
    lap: u64,
    out: &mut [u8],
) -> CodecResult<usize> {
    let offset = lap
        .checked_sub(record.batch.first_lap_index)
        .ok_or(UNAUTHORIZED)?;
    if lap > record.batch.last_lap_index || offset >= 64 {
        return Err(UNAUTHORIZED);
    }
    if record.was_slashed(peer, lap) {
        return Err(CONFLICT);
    }
    *record.slashed_laps.guest_index_mut(peer) |= 1u64 << offset;
    let source = state.control.feature_bytes;
    let region = SealRegion::committed(source)?;
    let laps = region.lap_settlements();
    let prefix = source.len() - laps.len() - region.rest().len();
    let row = laps
        .get(9..)
        .ok_or(NON_CANONICAL)?
        .chunks_exact(SETTLEMENT_BYTES)
        .position(|bytes| {
            bytes
                .get(..32)
                .is_some_and(|id| id == record.batch.deal_id.as_bytes())
        })
        .ok_or(NOT_FOUND)?;
    let at = prefix + 9 + row * SETTLEMENT_BYTES;
    out.get_mut(..source.len())
        .ok_or(CAPACITY)?
        .guest_copy_from_slice(source);
    out.get_mut(at..at + SETTLEMENT_BYTES)
        .ok_or(CAPACITY)?
        .guest_copy_from_slice(&record.encode()?);
    Ok(source.len())
}
