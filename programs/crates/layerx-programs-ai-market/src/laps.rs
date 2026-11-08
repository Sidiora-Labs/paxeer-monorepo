use crate::types::GuestIndex as _;
use crate::{
    codec::{self, EventCommon, Reader, ValidatedEnvelope, Writer},
    dispatch,
    errors::*,
    evidence::{self, SealRegion},
    registry::market_clock,
    registry_ops::{CallContext, ACTIVE},
    rooms::{RoomBook, RoomState, DEAL_CAPACITY, MAX_DEAL_PEERS},
    state::{self, ActorSlot, ReplayDecision, ReplayRequest, RetainedResult, Section},
    types::{Digest32, Presence, RosterDigest, Signature64, Version},
};

pub const LAP_BATCH_MAX: usize = 64;
pub const BATCH_BYTES: usize = 92;
pub const SETTLE_FIXED_BYTES: usize = BATCH_BYTES + 2;
pub const SCRATCH_BYTES: usize =
    Section::PolicyLifecycle.payload_cap() + 2 * Section::Control.payload_cap();
pub(crate) const REGION_MAGIC: &[u8; 8] = b"LAPSETT1";
const SETTLEMENT_BYTES: usize = BATCH_BYTES + 32 + 8 + 8 + 32 + 16;
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
    pub fn signing_digest(&self) -> CodecResult<Digest32> {
        let mut unsigned = *self;
        unsigned.host_signature = Signature64([1; 64]);
        let mut out = [0; RECEIPT_FIXED_BYTES + 32 * MAX_DEAL_PEERS];
        let n = unsigned.encode(&mut out)?;
        let signature_at = n - 68;
        out.guest_index_mut(signature_at..signature_at + 64).fill(0);
        codec::domain_hash("PAXAI/lap-signing/v1", out.guest_index(..n))
    }
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
}
impl ReceiptBatch {
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
        })
    }
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
        )?;
        Ok(out)
    }
    pub fn summary_digest(&self) -> CodecResult<Digest32> {
        codec::domain_hash("PAXAI/lap-batch/v1", &self.encode()?)
    }
    pub fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let value = crate::codec::read_record!(&mut r, Self {  }; deal_id = r.digest32()?;
            first_lap_index: U64,
            last_lap_index: U64,
            cumulative_tokens: U64,
            batch_digest: Digest32,
            countersign_quorum: U32
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
}
impl LapSettlement {
    pub fn meter_reading(&self) -> u64 {
        self.batch.cumulative_tokens
    }
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
        Ok(out)
    }
    fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let record = Self {
            batch: ReceiptBatch::decode(r.take(BATCH_BYTES)?)?,
            task_set: r.digest32()?,
            epoch: r.u64()?,
            config: r.u64()?,
            roster: r.roster_digest()?,
            settled_amount_owed: r.u128()?,
        };
        Version::new(record.config)?;
        r.finish()?;
        Ok(record)
    }
}

pub(crate) fn region_length(bytes: &[u8]) -> CodecResult<usize> {
    if !bytes.starts_with(REGION_MAGIC) {
        return Ok(0);
    }
    let mut r = Reader::new(bytes);
    r.take(8)?;
    let count = usize::from(r.u8()?);
    if count == 0 || count > DEAL_CAPACITY {
        return Err(NON_CANONICAL);
    }
    let rows = r.take(count * SETTLEMENT_BYTES)?;
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
    Ok(r.offset())
}
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
pub fn settlement_from_state(current: &[u8], deal: Digest32) -> CodecResult<Option<LapSettlement>> {
    let state = state::decode_shared_state(current)?;
    get(
        SealRegion::decode(state.control.feature_bytes)?.lap_settlements(),
        deal,
    )
}
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
    let mut policy = crate::canary::check_call(ctx, e, &state)?;
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
    let book = RoomBook::decode(state.room_market()?.1)?;
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
    if batch.countersign_quorum < peers.saturating_sub(1) || batch.countersign_quorum > peers {
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
    let revision = state.revision.checked_add(1).ok_or(ARITHMETIC)?;
    policy.header.state_revision = revision;
    let n = policy.encode(policy_out)?;
    let mut candidate =
        state.with_section(Section::PolicyLifecycle, policy_out.guest_index(..n))?;
    candidate.control.feature_bytes = features.guest_index(..feature_len);
    let record_bytes = record.encode()?;
    let result = codec::result_digest(&record_bytes)?;
    if candidate.record_success(&request, ctx.height, result)? != ReplayDecision::Apply
        || candidate.revision != revision
    {
        return Err(NON_CANONICAL);
    }
    crate::registry::check_f01_capacity(n, candidate.encoded_len()?)?;
    let state_len = state::encode_shared_state(&candidate, next, control)?;
    let event_len = codec::encode_event_frame(
        dispatch::SettleLaps,
        &EventCommon {
            market: e.market,
            epoch: e.epoch,
            config: Version::new(e.config)?,
            revision,
            request: envelope.request_digest()?,
            result,
        },
        &record_bytes,
        event,
    )?;
    Ok(Outcome::Applied {
        revision,
        state_len,
        event_len,
    })
}
