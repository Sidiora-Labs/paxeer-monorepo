use crate::{
    admission::{AdmissionTable, Participant},
    codec::{
        self, ApplicationResult, EventCommon, Reader, ResultStatus, ValidatedEnvelope, Writer,
    },
    dispatch,
    errors::*,
    evaluators::authority::split_identity_section,
    policy::{RoomProfile, ROOM_PROFILE_BYTES},
    registry::market_clock,
    registry_ops::{CallContext, PolicySection, ACTIVE, REGISTERED},
    reputation_transition::split_joint,
    roster::roster_digest,
    state::{self, ActorSlot, ReplayDecision, ReplayRequest, RetainedResult, Section, SharedState},
    types::{Digest32, Presence, PrincipalId, RequestDigest, RosterDigest, Version},
    workers::{self, WorkerCurrent, WorkerState, WorkerTable},
};

pub const ROOM_CAPACITY: usize = 16;
pub const ASK_CAPACITY: usize = 64;
pub const BID_CAPACITY: usize = 128;
pub const DEAL_CAPACITY: usize = 16;
pub const MAX_DEAL_PEERS: usize = 32;
pub const ROOM_RECORD_BYTES: usize = 209;
pub const ASK_RECORD_BYTES: usize = 193;
pub const BID_RECORD_BYTES: usize = 162;
pub const DEAL_RECORD_BYTES: usize = 146 + 36 * MAX_DEAL_PEERS;
const HEADER_BYTES: usize = 14;
pub const BOOK_MAX_BYTES: usize = HEADER_BYTES
    + ROOM_CAPACITY * ROOM_RECORD_BYTES
    + ASK_CAPACITY * ASK_RECORD_BYTES
    + BID_CAPACITY * BID_RECORD_BYTES
    + DEAL_CAPACITY * DEAL_RECORD_BYTES;
pub const SCRATCH_BYTES: usize = 2 * BOOK_MAX_BYTES
    + Section::SettlementClaims.payload_cap()
    + Section::PolicyLifecycle.payload_cap()
    + Section::Control.payload_cap();
const _: () = assert!(BOOK_MAX_BYTES + 13 <= Section::SettlementClaims.payload_cap());

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u8)]
pub enum RoomState {
    Open = 0,
    Closed = 1,
}
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u8)]
pub enum AskState {
    Open = 0,
    Matched = 1,
    Closed = 2,
}
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u8)]
pub enum DealState {
    Open = 0,
    Settled = 1,
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct RoomRecord {
    pub room_id: Digest32,
    pub host_key: PrincipalId,
    pub model_digest: Digest32,
    pub runtime_digest: Digest32,
    pub profile: RoomProfile,
    pub state: RoomState,
    pub opened_at_epoch: u64,
}
impl RoomRecord {
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        self.profile.validate()?;
        let mut profile = [0; ROOM_PROFILE_BYTES];
        self.profile.encode(&mut profile)?;
        let mut w = Writer::new(out.get_mut(..ROOM_RECORD_BYTES).ok_or(CAPACITY)?);
        w.put(self.room_id.as_bytes())?;
        w.put(self.host_key.as_bytes())?;
        w.put(self.model_digest.as_bytes())?;
        w.put(self.runtime_digest.as_bytes())?;
        w.put(&profile)?;
        w.u8(self.state as u8)?;
        w.u64(self.opened_at_epoch)?;
        Ok(w.len())
    }
    pub fn decode(input: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(input);
        let value = Self {
            room_id: r.digest32()?,
            host_key: r.principal_id()?,
            model_digest: r.digest32()?,
            runtime_digest: r.digest32()?,
            profile: RoomProfile::decode(r.take(ROOM_PROFILE_BYTES)?)?,
            state: match r.u8()? {
                0 => RoomState::Open,
                1 => RoomState::Closed,
                _ => return Err(NON_CANONICAL),
            },
            opened_at_epoch: r.u64()?,
        };
        r.finish()?;
        Ok(value)
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct AskRecord {
    pub room_id: Digest32,
    pub ask_id: Digest32,
    pub asker_key: PrincipalId,
    pub model_digest: Digest32,
    pub maximum_price_per_thousand_tokens: u128,
    pub stream_id: u128,
    pub state: AskState,
    pub posted_request: RequestDigest,
}
impl AskRecord {
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        if self.maximum_price_per_thousand_tokens == 0 {
            return Err(NON_CANONICAL);
        }
        let mut w = Writer::new(out.get_mut(..ASK_RECORD_BYTES).ok_or(CAPACITY)?);
        w.put(self.room_id.as_bytes())?;
        w.put(self.ask_id.as_bytes())?;
        w.put(self.asker_key.as_bytes())?;
        w.put(self.model_digest.as_bytes())?;
        w.u128(self.maximum_price_per_thousand_tokens)?;
        w.u128(self.stream_id)?;
        w.u8(self.state as u8)?;
        w.put(self.posted_request.as_bytes())?;
        Ok(w.len())
    }
    pub fn decode(input: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(input);
        let value = Self {
            room_id: r.digest32()?,
            ask_id: r.digest32()?,
            asker_key: r.principal_id()?,
            model_digest: r.digest32()?,
            maximum_price_per_thousand_tokens: r.u128()?,
            stream_id: r.u128()?,
            state: match r.u8()? {
                0 => AskState::Open,
                1 => AskState::Matched,
                2 => AskState::Closed,
                _ => return Err(NON_CANONICAL),
            },
            posted_request: r.request_digest()?,
        };
        r.finish()?;
        if value.maximum_price_per_thousand_tokens == 0 {
            return Err(NON_CANONICAL);
        }
        Ok(value)
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct BidRecord {
    pub room_id: Digest32,
    pub bid_id: Digest32,
    pub peer_key: PrincipalId,
    pub capability_digest: Digest32,
    pub minimum_price_per_thousand_tokens: u128,
    pub bond_amount: u128,
    pub layer_capacity: u16,
}
impl BidRecord {
    fn validate(&self) -> CodecResult<()> {
        if self.minimum_price_per_thousand_tokens == 0
            || self.bond_amount == 0
            || self.layer_capacity == 0
        {
            return Err(NON_CANONICAL);
        }
        Ok(())
    }
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        self.validate()?;
        let mut w = Writer::new(out.get_mut(..BID_RECORD_BYTES).ok_or(CAPACITY)?);
        w.put(self.room_id.as_bytes())?;
        w.put(self.bid_id.as_bytes())?;
        w.put(self.peer_key.as_bytes())?;
        w.put(self.capability_digest.as_bytes())?;
        w.u128(self.minimum_price_per_thousand_tokens)?;
        w.u128(self.bond_amount)?;
        w.u16(self.layer_capacity)?;
        Ok(w.len())
    }
    pub fn decode(input: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(input);
        let value = Self {
            room_id: r.digest32()?,
            bid_id: r.digest32()?,
            peer_key: r.principal_id()?,
            capability_digest: r.digest32()?,
            minimum_price_per_thousand_tokens: r.u128()?,
            bond_amount: r.u128()?,
            layer_capacity: r.u16()?,
        };
        r.finish()?;
        value.validate()?;
        Ok(value)
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct LayerSplit {
    pub bid_id: Digest32,
    pub first_layer: u16,
    pub end_layer: u16,
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct DealRecord {
    pub room_id: Digest32,
    pub deal_id: Digest32,
    pub ask_id: Digest32,
    pub agreed_price_per_thousand_tokens: u128,
    pub roster_digest: RosterDigest,
    pub layers: [Option<LayerSplit>; MAX_DEAL_PEERS],
    pub state: DealState,
}
impl DealRecord {
    fn peer_count(&self) -> CodecResult<usize> {
        let count = self.layers.iter().take_while(|s| s.is_some()).count();
        if count == 0
            || self.agreed_price_per_thousand_tokens == 0
            || self.layers[count..].iter().any(Option::is_some)
        {
            return Err(NON_CANONICAL);
        }
        let mut end = 0;
        for (i, split) in self.layers[..count].iter().flatten().enumerate() {
            if split.first_layer != end
                || split.end_layer <= split.first_layer
                || self.layers[..i]
                    .iter()
                    .flatten()
                    .any(|p| p.bid_id == split.bid_id)
            {
                return Err(NON_CANONICAL);
            }
            end = split.end_layer;
        }
        Ok(count)
    }
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        let count = self.peer_count()?;
        let mut w = Writer::new(out.get_mut(..DEAL_RECORD_BYTES).ok_or(CAPACITY)?);
        w.put(self.room_id.as_bytes())?;
        w.put(self.deal_id.as_bytes())?;
        w.put(self.ask_id.as_bytes())?;
        w.u128(self.agreed_price_per_thousand_tokens)?;
        w.put(self.roster_digest.as_bytes())?;
        w.u8(count as u8)?;
        w.u8(self.state as u8)?;
        for split in self.layers {
            if let Some(split) = split {
                w.put(split.bid_id.as_bytes())?;
                w.u16(split.first_layer)?;
                w.u16(split.end_layer)?;
            } else {
                w.put(&[0; 36])?;
            }
        }
        Ok(w.len())
    }
    pub fn decode(input: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(input);
        let mut value = Self {
            room_id: r.digest32()?,
            deal_id: r.digest32()?,
            ask_id: r.digest32()?,
            agreed_price_per_thousand_tokens: r.u128()?,
            roster_digest: r.roster_digest()?,
            layers: [None; MAX_DEAL_PEERS],
            state: DealState::Open,
        };
        let count = usize::from(r.u8()?);
        if !(1..=MAX_DEAL_PEERS).contains(&count) {
            return Err(NON_CANONICAL);
        }
        value.state = match r.u8()? {
            0 => DealState::Open,
            1 => DealState::Settled,
            _ => return Err(NON_CANONICAL),
        };
        for i in 0..MAX_DEAL_PEERS {
            if i < count {
                value.layers[i] = Some(LayerSplit {
                    bid_id: r.digest32()?,
                    first_layer: r.u16()?,
                    end_layer: r.u16()?,
                });
            } else if r.take(36)? != [0; 36] {
                return Err(NON_CANONICAL);
            }
        }
        r.finish()?;
        value.peer_count()?;
        Ok(value)
    }
}

#[derive(Clone, Copy)]
pub struct OpenRoomRequest {
    pub room_id: Digest32,
    pub model_digest: Digest32,
    pub runtime_digest: Digest32,
    pub profile: RoomProfile,
}
impl OpenRoomRequest {
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        let mut profile = [0; ROOM_PROFILE_BYTES];
        self.profile.encode(&mut profile)?;
        let mut w = Writer::new(out.get_mut(..168).ok_or(CAPACITY)?);
        w.put(self.room_id.as_bytes())?;
        w.put(self.model_digest.as_bytes())?;
        w.put(self.runtime_digest.as_bytes())?;
        w.put(&profile)?;
        Ok(w.len())
    }
}
#[derive(Clone, Copy)]
pub struct PostAskRequest {
    pub room_id: Digest32,
    pub ask_id: Digest32,
    pub model_digest: Digest32,
    pub maximum_price_per_thousand_tokens: u128,
    pub stream_id: u128,
}
impl PostAskRequest {
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        if self.maximum_price_per_thousand_tokens == 0 {
            return Err(NON_CANONICAL);
        }
        let mut w = Writer::new(out.get_mut(..128).ok_or(CAPACITY)?);
        w.put(self.room_id.as_bytes())?;
        w.put(self.ask_id.as_bytes())?;
        w.put(self.model_digest.as_bytes())?;
        w.u128(self.maximum_price_per_thousand_tokens)?;
        w.u128(self.stream_id)?;
        Ok(w.len())
    }
}
#[derive(Clone, Copy)]
pub struct PostBidRequest {
    pub room_id: Digest32,
    pub bid_id: Digest32,
    pub capability_digest: Digest32,
    pub minimum_price_per_thousand_tokens: u128,
    pub bond_amount: u128,
    pub layer_capacity: u16,
}
impl PostBidRequest {
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        if self.minimum_price_per_thousand_tokens == 0
            || self.bond_amount == 0
            || self.layer_capacity == 0
        {
            return Err(NON_CANONICAL);
        }
        let mut w = Writer::new(out.get_mut(..130).ok_or(CAPACITY)?);
        w.put(self.room_id.as_bytes())?;
        w.put(self.bid_id.as_bytes())?;
        w.put(self.capability_digest.as_bytes())?;
        w.u128(self.minimum_price_per_thousand_tokens)?;
        w.u128(self.bond_amount)?;
        w.u16(self.layer_capacity)?;
        Ok(w.len())
    }
}
#[derive(Clone, Copy)]
pub struct RecordDealRequest {
    pub room_id: Digest32,
    pub deal_id: Digest32,
    pub ask_id: Digest32,
    pub agreed_price_per_thousand_tokens: u128,
    pub roster_digest: RosterDigest,
    pub layers: [Option<LayerSplit>; MAX_DEAL_PEERS],
}
impl RecordDealRequest {
    fn record(&self) -> DealRecord {
        DealRecord {
            room_id: self.room_id,
            deal_id: self.deal_id,
            ask_id: self.ask_id,
            agreed_price_per_thousand_tokens: self.agreed_price_per_thousand_tokens,
            roster_digest: self.roster_digest,
            layers: self.layers,
            state: DealState::Open,
        }
    }
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        let count = self.record().peer_count()?;
        let mut w = Writer::new(out.get_mut(..145 + 36 * count).ok_or(CAPACITY)?);
        w.put(self.room_id.as_bytes())?;
        w.put(self.deal_id.as_bytes())?;
        w.put(self.ask_id.as_bytes())?;
        w.u128(self.agreed_price_per_thousand_tokens)?;
        w.put(self.roster_digest.as_bytes())?;
        w.u8(count as u8)?;
        for split in self.layers.iter().flatten() {
            w.put(split.bid_id.as_bytes())?;
            w.u16(split.first_layer)?;
            w.u16(split.end_layer)?;
        }
        Ok(w.len())
    }
    fn decode(input: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(input);
        let mut value = Self {
            room_id: r.digest32()?,
            deal_id: r.digest32()?,
            ask_id: r.digest32()?,
            agreed_price_per_thousand_tokens: r.u128()?,
            roster_digest: r.roster_digest()?,
            layers: [None; MAX_DEAL_PEERS],
        };
        let count = usize::from(r.u8()?);
        if !(1..=MAX_DEAL_PEERS).contains(&count) {
            return Err(NON_CANONICAL);
        }
        for slot in &mut value.layers[..count] {
            *slot = Some(LayerSplit {
                bid_id: r.digest32()?,
                first_layer: r.u16()?,
                end_layer: r.u16()?,
            });
        }
        r.finish()?;
        value.record().peer_count()?;
        Ok(value)
    }
}

#[derive(Clone, Copy)]
pub struct RoomBook<'a> {
    tables: [&'a [u8]; 4],
    counts: [usize; 4],
}
const WIDTHS: [usize; 4] = [
    ROOM_RECORD_BYTES,
    ASK_RECORD_BYTES,
    BID_RECORD_BYTES,
    DEAL_RECORD_BYTES,
];
const CAPS: [usize; 4] = [ROOM_CAPACITY, ASK_CAPACITY, BID_CAPACITY, DEAL_CAPACITY];
impl<'a> RoomBook<'a> {
    pub fn decode(input: &'a [u8]) -> CodecResult<Self> {
        if input.is_empty() {
            return Ok(Self {
                tables: [&[]; 4],
                counts: [0; 4],
            });
        }
        if input.len() > BOOK_MAX_BYTES {
            return Err(CAPACITY);
        }
        let mut r = Reader::new(input);
        if r.take(4)? != b"F11B" {
            return Err(NON_CANONICAL);
        }
        if r.u16()? != 1 {
            return Err(BAD_VERSION);
        }
        let mut book = Self {
            tables: [&[]; 4],
            counts: [0; 4],
        };
        for (i, cap) in CAPS.iter().enumerate() {
            book.counts[i] = usize::from(r.u16()?);
            if book.counts[i] > *cap {
                return Err(CAPACITY);
            }
        }
        for (i, width) in WIDTHS.iter().enumerate() {
            book.tables[i] = r.take(book.counts[i] * width)?;
            let mut previous = None;
            for bytes in book.tables[i].chunks_exact(*width) {
                let (id, room) = match i {
                    0 => {
                        let v = RoomRecord::decode(bytes)?;
                        (v.room_id, v.room_id)
                    }
                    1 => {
                        let v = AskRecord::decode(bytes)?;
                        (v.ask_id, v.room_id)
                    }
                    2 => {
                        let v = BidRecord::decode(bytes)?;
                        (v.bid_id, v.room_id)
                    }
                    _ => {
                        let v = DealRecord::decode(bytes)?;
                        (v.deal_id, v.room_id)
                    }
                };
                if previous.is_some_and(|p| p >= id) {
                    return Err(NON_CANONICAL);
                }
                previous = Some(id);
                if i > 0 && book.room(room)?.is_none() {
                    return Err(NON_CANONICAL);
                }
            }
        }
        r.finish()?;
        Ok(book)
    }
    fn get(&self, table: usize, id: Digest32) -> Option<&'a [u8]> {
        let at = if table == 0 { 0 } else { 32 };
        self.tables[table]
            .chunks_exact(WIDTHS[table])
            .find(|bytes| &bytes[at..at + 32] == id.as_bytes())
    }
    pub fn room(&self, id: Digest32) -> CodecResult<Option<RoomRecord>> {
        self.get(0, id).map(RoomRecord::decode).transpose()
    }
    pub fn ask(&self, id: Digest32) -> CodecResult<Option<AskRecord>> {
        self.get(1, id).map(AskRecord::decode).transpose()
    }
    pub fn bid(&self, id: Digest32) -> CodecResult<Option<BidRecord>> {
        self.get(2, id).map(BidRecord::decode).transpose()
    }
    pub fn deal(&self, id: Digest32) -> CodecResult<Option<DealRecord>> {
        self.get(3, id).map(DealRecord::decode).transpose()
    }
    pub fn counts(&self) -> [usize; 4] {
        self.counts
    }
    fn write(
        &self,
        table: usize,
        id: Digest32,
        record: &[u8],
        replace: bool,
        out: &mut [u8],
    ) -> CodecResult<usize> {
        if record.len() != WIDTHS[table] {
            return Err(NON_CANONICAL);
        }
        let found = self.get(table, id).is_some();
        if found != replace {
            return Err(CONFLICT);
        }
        let mut counts = self.counts;
        if !replace {
            counts[table] += 1;
        }
        if counts[table] > CAPS[table] {
            return Err(CAPACITY);
        }
        let mut w = Writer::new(out);
        w.put(b"F11B")?;
        w.u16(1)?;
        for count in counts {
            w.u16(count as u16)?;
        }
        for (i, width) in WIDTHS.iter().enumerate() {
            if i != table {
                w.put(self.tables[i])?;
                continue;
            }
            let offset = if i == 0 { 0 } else { 32 };
            let index = self.tables[i]
                .chunks_exact(*width)
                .take_while(|b| &b[offset..offset + 32] < &id.as_bytes()[..])
                .count();
            let at = index * width;
            w.put(&self.tables[i][..at])?;
            w.put(record)?;
            w.put(&self.tables[i][at + if replace { *width } else { 0 }..])?;
        }
        Ok(w.len())
    }
    fn open_room(&self, id: Digest32) -> CodecResult<RoomRecord> {
        let room = self.room(id)?.ok_or(F11_ROOM_UNKNOWN)?;
        if room.state != RoomState::Open {
            return Err(F11_ROOM_CLOSED);
        }
        Ok(room)
    }
}

pub enum Outcome {
    Applied {
        revision: u64,
        state_len: usize,
        event_len: usize,
    },
    Retained(RetainedResult),
    AlreadyApplied(Digest32),
}

fn worker_for(
    workers: &WorkerTable,
    principal: PrincipalId,
    capability: Digest32,
    height: u64,
) -> CodecResult<WorkerCurrent> {
    let worker = workers
        .iter()
        .find(|w| w.owner == principal && w.metadata.bytes() == capability.bytes())
        .copied()
        .ok_or(F02_CAPABILITY_MISMATCH)?;
    if !matches!(worker.state, WorkerState::Enrolled | WorkerState::Available) {
        return Err(UNAUTHORIZED);
    }
    workers::check_metadata_window(&worker, height)?;
    Ok(worker)
}

pub fn apply_record_deal(
    book: &RoomBook<'_>,
    request: &RecordDealRequest,
    actor: PrincipalId,
    admission: &AdmissionTable,
    workers: &WorkerTable,
    epoch: u64,
    height: u64,
    out: &mut [u8],
) -> CodecResult<usize> {
    let room = book.open_room(request.room_id)?;
    if actor != room.host_key {
        return Err(UNAUTHORIZED);
    }
    let ask = book.ask(request.ask_id)?.ok_or(NOT_FOUND)?;
    if ask.room_id != room.room_id
        || ask.model_digest != room.model_digest
        || ask.state != AskState::Open
    {
        return Err(CONFLICT);
    }
    if ask.stream_id == 0 {
        return Err(F11_ASK_UNFUNDED);
    }
    if admission.current_epoch() != Some(epoch)
        || roster_digest(admission, epoch)?.bytes() != request.roster_digest.bytes()
    {
        return Err(F11_ROSTER_DIGEST_MISMATCH);
    }
    let record = request.record();
    record.peer_count()?;
    if record.agreed_price_per_thousand_tokens > ask.maximum_price_per_thousand_tokens {
        return Err(CONFLICT);
    }
    for (i, split) in record.layers.iter().flatten().enumerate() {
        let bid = book.bid(split.bid_id)?.ok_or(NOT_FOUND)?;
        if bid.room_id != room.room_id {
            return Err(CONFLICT);
        }
        if record.agreed_price_per_thousand_tokens < bid.minimum_price_per_thousand_tokens {
            return Err(F11_BID_BELOW_FLOOR);
        }
        if split.end_layer - split.first_layer > bid.layer_capacity {
            return Err(CAPACITY);
        }
        let worker = worker_for(workers, bid.peer_key, bid.capability_digest, height)?;
        let member = admission
            .get(Participant::Worker(worker.worker))
            .ok_or(UNAUTHORIZED)?;
        if !member.admitted()
            || member.admitted_epoch.is_none()
            || member.revoked()
            || member.draining()
            || member.owner != bid.peer_key
            || member.delegate_generation != worker.key_version
            || worker.effective_epoch > epoch
        {
            return Err(UNAUTHORIZED);
        }
        for previous in record.layers[..i].iter().flatten() {
            if book.bid(previous.bid_id)?.ok_or(NOT_FOUND)?.peer_key == bid.peer_key {
                return Err(CONFLICT);
            }
        }
        if bid.bond_amount < room.profile.minimum_bond {
            return Err(F11_BOND_INSUFFICIENT);
        }
    }
    let mut bytes = [0; DEAL_RECORD_BYTES];
    record.encode(&mut bytes)?;
    book.write(3, record.deal_id, &bytes, false, out)
}

pub fn apply(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Outcome> {
    let e = &envelope.envelope;
    let op = e.operation;
    if !matches!(
        op,
        dispatch::OpenRoom
            | dispatch::PostAsk
            | dispatch::PostBid
            | dispatch::RecordDeal
            | dispatch::CloseRoom
    ) {
        return Err(UNKNOWN_OPERATION);
    }
    codec::compare_native_principal(e, ctx.principal)?;
    e.check_domain(
        ctx.chain,
        ctx.program,
        codec::derive_market(ctx.chain, ctx.program)?,
    )?;
    e.check_expiry(ctx.height)?;
    op.validate_payload_length(e.payload.len())?;
    let state = state::decode_shared_state(current)?;
    let mut policy = PolicySection::decode(state.section(Section::PolicyLifecycle)?)?;
    let header = &policy.header;
    if header.state_revision != state.revision {
        return Err(NON_CANONICAL);
    }
    if header.deployment_chain_domain != ctx.chain {
        return Err(WRONG_DOMAIN);
    }
    if header.program_id != ctx.program {
        return Err(WRONG_PROGRAM);
    }
    if header.market_id != e.market {
        return Err(WRONG_MARKET);
    }
    let epoch = market_clock(header.origin_height, ctx.height)?.epoch;
    if e.epoch != epoch {
        return Err(WRONG_EPOCH);
    }
    if e.config != header.active_config_version {
        return Err(WRONG_CONFIG);
    }
    let workers =
        WorkerTable::decode(split_identity_section(state.section(Section::IdentityRoster)?)?.0)?;
    let book = RoomBook::decode(state.room_market()?.1)?;
    let mut r = Reader::new(e.payload);
    let room_id = r.digest32()?;
    let mut bid_worker = None;
    if op == dispatch::PostBid {
        let _bid = r.digest32()?;
        bid_worker = Some(worker_for(
            &workers,
            ctx.principal,
            r.digest32()?,
            ctx.height,
        )?);
    }
    let replay = if op == dispatch::PostAsk {
        None
    } else {
        let slot = if let Some(worker) = bid_worker {
            ActorSlot::worker(usize::from(worker.slot))?
        } else if ctx.principal == header.owner_principal {
            ActorSlot::OWNER
        } else {
            ActorSlot::worker(usize::from(
                workers
                    .iter()
                    .find(|w| {
                        w.owner == ctx.principal
                            && matches!(w.state, WorkerState::Enrolled | WorkerState::Available)
                    })
                    .ok_or(UNAUTHORIZED)?
                    .slot,
            ))?
        };
        let authority = state.control.replay.actor(slot).ok_or(UNAUTHORIZED)?;
        let request = ReplayRequest::from_envelope(slot, authority.authority_version, envelope)?;
        if let ReplayDecision::AlreadyApplied(retained) =
            state.control.replay.check(&request, ctx.height)?
        {
            return Ok(Outcome::Retained(retained));
        }
        Some(request)
    };
    if !matches!(header.lifecycle, REGISTERED | ACTIVE) {
        return Err(WRONG_PHASE);
    }
    let (first, rest) = scratch
        .split_at_mut_checked(BOOK_MAX_BYTES)
        .ok_or(CAPACITY)?;
    let (second, rest) = rest.split_at_mut_checked(BOOK_MAX_BYTES).ok_or(CAPACITY)?;
    let (settlement, rest) = rest
        .split_at_mut_checked(Section::SettlementClaims.payload_cap())
        .ok_or(CAPACITY)?;
    let (policy_out, control) = rest
        .split_at_mut_checked(Section::PolicyLifecycle.payload_cap())
        .ok_or(CAPACITY)?;
    let (book_bytes, subject): (&[u8], Digest32) = match op {
        dispatch::OpenRoom => {
            let mut r = Reader::new(e.payload);
            r.take(32)?;
            let room = RoomRecord {
                room_id,
                host_key: ctx.principal,
                model_digest: r.digest32()?,
                runtime_digest: r.digest32()?,
                profile: RoomProfile::decode(r.take(ROOM_PROFILE_BYTES)?)?,
                state: RoomState::Open,
                opened_at_epoch: epoch,
            };
            r.finish()?;
            let mut bytes = [0; ROOM_RECORD_BYTES];
            room.encode(&mut bytes)?;
            let n = book.write(0, room_id, &bytes, false, first)?;
            (&first[..n], room_id)
        }
        dispatch::PostAsk => {
            let room = book.open_room(room_id)?;
            let mut r = Reader::new(e.payload);
            r.take(32)?;
            let ask = AskRecord {
                room_id,
                ask_id: r.digest32()?,
                asker_key: ctx.principal,
                model_digest: r.digest32()?,
                maximum_price_per_thousand_tokens: r.u128()?,
                stream_id: r.u128()?,
                state: AskState::Open,
                posted_request: envelope.request_digest()?,
            };
            r.finish()?;
            if ask.model_digest != room.model_digest {
                return Err(CONFLICT);
            }
            if let Some(previous) = book.ask(ask.ask_id)? {
                if previous.posted_request == ask.posted_request {
                    return Ok(Outcome::AlreadyApplied(ask.ask_id));
                }
                return Err(CONFLICT);
            }
            let mut bytes = [0; ASK_RECORD_BYTES];
            ask.encode(&mut bytes)?;
            let n = book.write(1, ask.ask_id, &bytes, false, first)?;
            (&first[..n], ask.ask_id)
        }
        dispatch::PostBid => {
            let room = book.open_room(room_id)?;
            let mut r = Reader::new(e.payload);
            r.take(32)?;
            let bid = BidRecord {
                room_id,
                bid_id: r.digest32()?,
                peer_key: ctx.principal,
                capability_digest: r.digest32()?,
                minimum_price_per_thousand_tokens: r.u128()?,
                bond_amount: r.u128()?,
                layer_capacity: r.u16()?,
            };
            r.finish()?;
            if bid.bond_amount < room.profile.minimum_bond {
                return Err(F11_BOND_INSUFFICIENT);
            }
            let mut bytes = [0; BID_RECORD_BYTES];
            bid.encode(&mut bytes)?;
            let n = book.write(2, bid.bid_id, &bytes, false, first)?;
            (&first[..n], bid.bid_id)
        }
        dispatch::RecordDeal => {
            let request = RecordDealRequest::decode(e.payload)?;
            if e.roster != Presence::Present(request.roster_digest) {
                return Err(F11_ROSTER_DIGEST_MISMATCH);
            }
            let admission = AdmissionTable::decode(
                split_joint(state.section(Section::ReputationAdmission)?)?.1,
            )?;
            let n = apply_record_deal(
                &book,
                &request,
                ctx.principal,
                &admission,
                &workers,
                epoch,
                ctx.height,
                first,
            )?;
            let staged = RoomBook::decode(&first[..n])?;
            let mut ask = staged.ask(request.ask_id)?.ok_or(NOT_FOUND)?;
            ask.state = AskState::Matched;
            let mut bytes = [0; ASK_RECORD_BYTES];
            ask.encode(&mut bytes)?;
            let n = staged.write(1, ask.ask_id, &bytes, true, second)?;
            (&second[..n], request.deal_id)
        }
        dispatch::CloseRoom => {
            let mut room = book.open_room(room_id)?;
            if room.host_key != ctx.principal {
                return Err(UNAUTHORIZED);
            }
            for bytes in book.tables[3].chunks_exact(DEAL_RECORD_BYTES) {
                let deal = DealRecord::decode(bytes)?;
                if deal.room_id == room_id && deal.state == DealState::Open {
                    return Err(WRONG_PHASE);
                }
            }
            room.state = RoomState::Closed;
            let mut bytes = [0; ROOM_RECORD_BYTES];
            room.encode(&mut bytes)?;
            let n = book.write(0, room_id, &bytes, true, first)?;
            (&first[..n], room_id)
        }
        _ => return Err(UNKNOWN_OPERATION),
    };
    let settlement_len = state.encode_room_market(book_bytes, settlement)?;
    let mut candidate =
        state.replace_section(Section::SettlementClaims, &settlement[..settlement_len])?;
    let revision = state.revision.checked_add(1).ok_or(ARITHMETIC)?;
    let digest = ApplicationResult::success(
        ResultStatus::Ok,
        envelope.request_digest()?,
        revision,
        subject.as_bytes(),
    )?
    .digest;
    if let Some(request) = replay {
        if candidate.record_success(&request, ctx.height, digest)? != ReplayDecision::Apply {
            return Err(NON_CANONICAL);
        }
    } else {
        candidate.revision = revision;
    }
    policy.header.state_revision = revision;
    let policy_len = policy.encode(policy_out)?;
    let candidate =
        candidate.replace_section(Section::PolicyLifecycle, &policy_out[..policy_len])?;
    let state_len = state::encode_shared_state(&candidate, next, control)?;
    let event_len = codec::encode_event_frame(
        op,
        &EventCommon {
            market: e.market,
            epoch,
            config: Version::new(e.config)?,
            revision,
            request: envelope.request_digest()?,
            result: digest,
        },
        subject.as_bytes(),
        event,
    )?;
    Ok(Outcome::Applied {
        revision,
        state_len,
        event_len,
    })
}
