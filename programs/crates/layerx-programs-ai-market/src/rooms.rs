use crate::types::GuestIndex as _;
use crate::{
    admission::{AdmissionTable, Participant},
    codec::{self, Reader, ValidatedEnvelope, Writer},
    dispatch,
    errors::*,
    evaluators::authority::split_identity_section,
    policy::{ROOM_PROFILE_BYTES, RoomProfile},
    registry::market_clock,
    registry_ops::{ACTIVE, CallContext, PolicySection, REGISTERED},
    reputation_transition::split_joint,
    roster::roster_digest,
    state::{self, ActorSlot, ReplayDecision, ReplayRequest, RetainedResult, Section},
    types::{Digest32, Presence, PrincipalId, RequestDigest, RosterDigest},
    workers::{self, WorkerCurrent, WorkerState, WorkerTable},
};

pub const ROOM_CAPACITY: usize = 16;
pub const ASK_CAPACITY: usize = 64;
pub const BID_CAPACITY: usize = 128;
pub const DEAL_CAPACITY: usize = 16;
pub const MAX_DEAL_PEERS: usize = 32;
pub const ROOM_RECORD_BYTES: usize = 4 * 32 + ROOM_PROFILE_BYTES + 1 + 8 + 8;
pub const OPEN_ROOM_REQUEST_BYTES: usize = 3 * 32 + ROOM_PROFILE_BYTES;
pub const ROOM_BOOK_VERSION: u16 = 3;
pub const ASK_RECORD_BYTES: usize = 193;
pub const BID_RECORD_BYTES: usize = 162;
pub const DEAL_RECORD_BYTES: usize = 146 + 36 * MAX_DEAL_PEERS;
pub const HEADER_BYTES: usize = 20;
pub const BOND_RECORD_BYTES: usize = 169;
pub const BOOK_MAX_BYTES: usize = HEADER_BYTES
    + ROOM_CAPACITY * ROOM_RECORD_BYTES
    + ASK_CAPACITY * ASK_RECORD_BYTES
    + BID_CAPACITY * BID_RECORD_BYTES
    + DEAL_CAPACITY * DEAL_RECORD_BYTES
    + BID_CAPACITY * BOND_RECORD_BYTES;
pub const SCRATCH_BYTES: usize = 2 * BOOK_MAX_BYTES
    + Section::SettlementClaims.payload_cap()
    + Section::PolicyLifecycle.payload_cap()
    + Section::Control.payload_cap();
/// F06 (including its overlaid payout journal), F05 current/history and F11 framing/book.
pub const SETTLEMENT_MAX_BYTES: usize = crate::rewards::REWARD_STATE_BYTES
    + crate::aggregation_codec::F05_MAX_BYTES
    + state::ROOM_MARKET_FRAMING_BYTES
    + BOOK_MAX_BYTES;
const _: () = assert!(SETTLEMENT_MAX_BYTES <= Section::SettlementClaims.payload_cap());

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum RoomState {
    Open = 0,
    Closed = 1,
}
impl RoomState {
    pub(crate) fn decode(value: u8) -> CodecResult<Self> {
        match value {
            0 => Ok(Self::Open),
            1 => Ok(Self::Closed),
            _ => Err(NON_CANONICAL),
        }
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum AskState {
    Open = 0,
    Matched = 1,
    Closed = 2,
}
impl AskState {
    pub(crate) fn decode(value: u8) -> CodecResult<Self> {
        match value {
            0 => Ok(Self::Open),
            1 => Ok(Self::Matched),
            2 => Ok(Self::Closed),
            _ => Err(NON_CANONICAL),
        }
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum DealState {
    Open = 0,
    Settled = 1,
}
impl DealState {
    pub(crate) fn decode(value: u8) -> CodecResult<Self> {
        match value {
            0 => Ok(Self::Open),
            1 => Ok(Self::Settled),
            _ => Err(NON_CANONICAL),
        }
    }
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
    pub closed_height: u64,
}
impl RoomRecord {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        self.profile.validate()?;
        let mut w = Writer::new(out.get_mut(..ROOM_RECORD_BYTES).ok_or(CAPACITY)?);
        crate::codec::write_fields!(w;
            id32: self.room_id.as_bytes();
            id32: self.host_key.as_bytes();
            id32: self.model_digest.as_bytes();
            id32: self.runtime_digest.as_bytes();
        )?;
        w.room_profile(&self.profile)?;
        w.u8(self.state as u8)?;
        w.u64(self.opened_at_epoch)?;
        w.u64(self.closed_height)?;
        Ok(w.len())
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(input: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(input);
        let value = crate::codec::read_record!(&mut r, Self {  }; room_id = r.digest32()?;
            host_key: PrincipalId,
            model_digest: Digest32,
            runtime_digest: Digest32,
            profile: RoomProfile,
            state: RoomState,
            opened_at_epoch: U64, closed_height: U64
        );
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
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        if self.maximum_price_per_thousand_tokens == 0 {
            return Err(NON_CANONICAL);
        }
        let mut w = Writer::new(out.get_mut(..ASK_RECORD_BYTES).ok_or(CAPACITY)?);
        crate::codec::write_fields!(w;
            id32: self.room_id.as_bytes();
            id32: self.ask_id.as_bytes();
            id32: self.asker_key.as_bytes();
            id32: self.model_digest.as_bytes();
            u128: self.maximum_price_per_thousand_tokens;
            u128: self.stream_id;
            u8: self.state as u8;
            id32: self.posted_request.as_bytes();
        )?;
        Ok(w.len())
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(input: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(input);
        let value = crate::codec::read_record!(&mut r, Self {  }; room_id = r.digest32()?;
            ask_id: Digest32,
            asker_key: PrincipalId,
            model_digest: Digest32,
            maximum_price_per_thousand_tokens: U128,
            stream_id: U128,
            state: AskState,
            posted_request: RequestDigest
        );
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
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        self.validate()?;
        let mut w = Writer::new(out.get_mut(..BID_RECORD_BYTES).ok_or(CAPACITY)?);
        crate::codec::write_fields!(w;
            id32: self.room_id.as_bytes();
            id32: self.bid_id.as_bytes();
            id32: self.peer_key.as_bytes();
            id32: self.capability_digest.as_bytes();
            u128: self.minimum_price_per_thousand_tokens;
            u128: self.bond_amount;
            u16: self.layer_capacity;
        )?;
        Ok(w.len())
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(input: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(input);
        let value = crate::codec::read_record!(&mut r, Self {  }; room_id = r.digest32()?;
            bid_id: Digest32,
            peer_key: PrincipalId,
            capability_digest: Digest32,
            minimum_price_per_thousand_tokens: U128,
            bond_amount: U128,
            layer_capacity: U16
        );
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
impl LayerSplit {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn read(r: &mut Reader<'_>) -> CodecResult<Self> {
        Ok(Self {
            bid_id: r.digest32()?,
            first_layer: r.u16()?,
            end_layer: r.u16()?,
        })
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write(&self, w: &mut Writer<'_>) -> CodecResult<()> {
        w.put(self.bid_id.as_bytes())?;
        w.u16(self.first_layer)?;
        w.u16(self.end_layer)
    }
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
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn peer_count(&self) -> CodecResult<usize> {
        let count = self.layers.iter().take_while(|s| s.is_some()).count();
        if count == 0
            || self.agreed_price_per_thousand_tokens == 0
            || self.layers.guest_index(count..).iter().any(Option::is_some)
        {
            return Err(NON_CANONICAL);
        }
        let mut end = 0;
        let mut seen_prefix = [0u64; 4];
        for (i, split) in self
            .layers
            .guest_index(..count)
            .iter()
            .flatten()
            .enumerate()
        {
            let prefix = split.bid_id.as_bytes()[0];
            let word = usize::from(prefix >> 6);
            let mask = 1u64 << (prefix & 63);
            if split.first_layer != end
                || split.end_layer <= split.first_layer
                || (*seen_prefix.guest_index(word) & mask != 0
                    && self
                        .layers
                        .guest_index(..i)
                        .iter()
                        .flatten()
                        .any(|p| p.bid_id == split.bid_id))
            {
                return Err(NON_CANONICAL);
            }
            *seen_prefix.guest_index_mut(word) |= mask;
            end = split.end_layer;
        }
        Ok(count)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        let count = self.peer_count()?;
        self.encode_with_count(count, out)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn encode_with_count(&self, count: usize, out: &mut [u8]) -> CodecResult<usize> {
        let mut w = Writer::new(out.get_mut(..DEAL_RECORD_BYTES).ok_or(CAPACITY)?);
        RecordDealRequest::write_header(
            &mut w,
            self.room_id,
            self.deal_id,
            self.ask_id,
            self.agreed_price_per_thousand_tokens,
            self.roster_digest,
            count,
        )?;
        w.u8(self.state as u8)?;
        for split in self.layers {
            if let Some(split) = split {
                split.write(&mut w)?;
            } else {
                w.put(&[0; 36])?;
            }
        }
        Ok(w.len())
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(input: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(input);
        let (request, count) = RecordDealRequest::read_header(&mut r)?;
        let mut value = request.record();
        value.state = DealState::decode(r.u8()?)?;
        for i in 0..MAX_DEAL_PEERS {
            if i < count {
                *value.layers.guest_index_mut(i) = Some(LayerSplit::read(&mut r)?);
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
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        self.profile.validate()?;
        let mut w = Writer::new(out.get_mut(..OPEN_ROOM_REQUEST_BYTES).ok_or(CAPACITY)?);
        crate::codec::write_fields!(w;
            id32: self.room_id.as_bytes();
            id32: self.model_digest.as_bytes();
            id32: self.runtime_digest.as_bytes();
        )?;
        w.room_profile(&self.profile)?;
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
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        if self.maximum_price_per_thousand_tokens == 0 {
            return Err(NON_CANONICAL);
        }
        let mut w = Writer::new(out.get_mut(..128).ok_or(CAPACITY)?);
        crate::codec::write_fields!(w;
            id32: self.room_id.as_bytes();
            id32: self.ask_id.as_bytes();
            id32: self.model_digest.as_bytes();
            u128: self.maximum_price_per_thousand_tokens;
            u128: self.stream_id;
        )?;
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
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        if self.minimum_price_per_thousand_tokens == 0
            || self.bond_amount == 0
            || self.layer_capacity == 0
        {
            return Err(NON_CANONICAL);
        }
        let mut w = Writer::new(out.get_mut(..130).ok_or(CAPACITY)?);
        crate::codec::write_fields!(w;
            id32: self.room_id.as_bytes();
            id32: self.bid_id.as_bytes();
            id32: self.capability_digest.as_bytes();
            u128: self.minimum_price_per_thousand_tokens;
            u128: self.bond_amount;
            u16: self.layer_capacity;
        )?;
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
    #[cfg_attr(target_arch = "wasm32", inline(never))]
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
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        let count = self.record().peer_count()?;
        let mut w = Writer::new(out.get_mut(..145 + 36 * count).ok_or(CAPACITY)?);
        Self::write_header(
            &mut w,
            self.room_id,
            self.deal_id,
            self.ask_id,
            self.agreed_price_per_thousand_tokens,
            self.roster_digest,
            count,
        )?;
        for split in self.layers.iter().flatten() {
            split.write(&mut w)?;
        }
        Ok(w.len())
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn read_header(r: &mut Reader<'_>) -> CodecResult<(Self, usize)> {
        let value = Self {
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
        Ok((value, count))
    }
    #[allow(clippy::too_many_arguments)]
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write_header(
        w: &mut Writer<'_>,
        room: Digest32,
        deal: Digest32,
        ask: Digest32,
        price: u128,
        roster: RosterDigest,
        count: usize,
    ) -> CodecResult<()> {
        crate::codec::write_fields!(w;
            id32: room.as_bytes();
            id32: deal.as_bytes();
            id32: ask.as_bytes();
            u128: price;
            id32: roster.as_bytes();
        )?;
        w.u8(count as u8)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn decode(input: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(input);
        let (mut value, count) = Self::read_header(&mut r)?;
        for slot in value.layers.guest_index_mut(..count) {
            *slot = Some(LayerSplit::read(&mut r)?);
        }
        r.finish()?;
        value.record().peer_count()?;
        Ok(value)
    }
}

struct BondIdentity {
    key: Digest32,
    worker: crate::types::WorkerId,
    height: u64,
}
impl BondIdentity {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let value = crate::codec::read_record!(&mut r, Self { }; key = r.digest32()?;
            worker: WorkerId, height: U64);
        r.finish()?;
        Ok(value)
    }
}
struct BondAdjustment {
    bid: Digest32,
    slashed: u128,
    height: u64,
    refunded: bool,
}
impl BondAdjustment {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let value = crate::codec::read_record!(&mut r, Self { }; bid = r.digest32()?;
            slashed: U128, height: U64, refunded: Boolean);
        r.finish()?;
        Ok(value)
    }
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn bond_identity_key(bid: &BidRecord) -> CodecResult<Digest32> {
    let mut bytes = [0; 64];
    bytes[..32].guest_copy_from_slice(bid.peer_key.as_bytes());
    bytes[32..].guest_copy_from_slice(bid.capability_digest.as_bytes());
    codec::domain_hash("PAXAI/bond-identity/v1", &bytes)
}

#[derive(Clone, Copy)]
pub struct RoomBook<'a> {
    tables: [&'a [u8]; 7],
    counts: [usize; 7],
}
const WIDTHS: [usize; 7] = [
    ROOM_RECORD_BYTES,
    ASK_RECORD_BYTES,
    BID_RECORD_BYTES,
    DEAL_RECORD_BYTES,
    BOND_RECORD_BYTES,
    72,
    57,
];
const CAPS: [usize; 7] = [
    ROOM_CAPACITY,
    ASK_CAPACITY,
    BID_CAPACITY,
    DEAL_CAPACITY,
    BID_CAPACITY,
    BID_CAPACITY,
    BID_CAPACITY,
];
#[cfg(target_arch = "wasm32")]
static DECODED_BOOK: crate::guest_cell::GuestOnce<(&'static [u8], RoomBook<'static>)> =
    crate::guest_cell::GuestOnce::new();

impl<'a> RoomBook<'a> {
    // The stored book was validated when its producer committed it. Read its
    // framing and bounded tables for a transition whose output is fully checked.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn committed(input: &'a [u8]) -> CodecResult<Self> {
        let (book, used) = Self::read_tables(input)?;
        if used != input.len() {
            return Err(NON_CANONICAL);
        }
        Ok(book)
    }

    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn read_tables(input: &'a [u8]) -> CodecResult<(Self, usize)> {
        let mut book = Self {
            tables: [&[]; 7],
            counts: [0; 7],
        };
        if input.is_empty() {
            return Ok((book, 0));
        }
        if input.len() > BOOK_MAX_BYTES {
            return Err(CAPACITY);
        }
        let mut r = Reader::new(input);
        if r.take(4)? != b"F11B" {
            return Err(NON_CANONICAL);
        }
        if r.u16()? != ROOM_BOOK_VERSION {
            return Err(BAD_VERSION);
        }
        for (i, cap) in CAPS.iter().enumerate() {
            *book.counts.guest_index_mut(i) = usize::from(r.u16()?);
            if *book.counts.guest_index(i) > *cap {
                return Err(CAPACITY);
            }
        }
        for (i, width) in WIDTHS.iter().enumerate() {
            *book.tables.guest_index_mut(i) = r.take(*book.counts.guest_index(i) * width)?;
        }
        Ok((book, r.offset()))
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(input: &'a [u8]) -> CodecResult<Self> {
        #[cfg(target_arch = "wasm32")]
        {
            if let Some((bytes, book)) = DECODED_BOOK.get() {
                if core::ptr::eq(*bytes, input) {
                    return Ok(*book);
                }
            } else if let Some(frozen) = state::current_slice(input) {
                let (_, book) = DECODED_BOOK.get_or_try_init(
                    || Ok((frozen, RoomBook::decode_uncached(frozen)?)),
                    NON_CANONICAL,
                )?;
                return Ok(*book);
            }
        }
        Self::decode_uncached(input)
    }

    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn decode_uncached(input: &'a [u8]) -> CodecResult<Self> {
        let (book, used) = Self::read_tables(input)?;
        let mut room_refs_preserved = false;
        let mut bid_unchanged = false;
        for (i, width) in WIDTHS.iter().enumerate() {
            // An unchanged table keeps its current-state validation when the
            // rows that supply its foreign keys remain present.
            #[cfg(target_arch = "wasm32")]
            let unchanged = DECODED_BOOK.get().is_some_and(|(_, previous)| {
                codec::equal_bytes(book.tables.guest_index(i), previous.tables.guest_index(i))
            });
            #[cfg(not(target_arch = "wasm32"))]
            let unchanged = false;
            if i == 0 {
                room_refs_preserved = unchanged;
            } else if i == 2 {
                bid_unchanged = unchanged;
            }
            if unchanged
                && match i {
                    1..=4 => room_refs_preserved,
                    6 => bid_unchanged,
                    _ => true,
                }
            {
                continue;
            }
            let mut previous = None;
            let mut previous_room = None;
            for bytes in book.tables.guest_index(i).guest_chunks_exact(*width) {
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
                    3 => {
                        let v = DealRecord::decode(bytes)?;
                        (v.deal_id, v.room_id)
                    }
                    4 => {
                        let v = crate::rewards::bonds::BondRecord::decode(bytes)?;
                        (v.bid, v.room)
                    }
                    5 => {
                        let v = BondIdentity::decode(bytes)?;
                        (v.key, v.key)
                    }
                    _ => {
                        let v = BondAdjustment::decode(bytes)?;
                        (v.bid, v.bid)
                    }
                };
                if i == 6 && book.get(2, id).is_none() {
                    return Err(NON_CANONICAL);
                }
                if previous.is_some_and(|p| p >= id) {
                    return Err(NON_CANONICAL);
                }
                previous = Some(id);
                if i > 0 && i < 5 && previous_room != Some(room) {
                    if book.get(0, room).is_none() {
                        return Err(NON_CANONICAL);
                    }
                    // Consecutive orders can share a room. Its immutable row
                    // needs only one successful join in this table scan.
                    previous_room = Some(room);
                }
            }
            if i == 0 && !room_refs_preserved {
                #[cfg(target_arch = "wasm32")]
                {
                    room_refs_preserved = DECODED_BOOK.get().is_some_and(|(_, previous)| {
                        let mut new_rooms = book.table(0).guest_chunks_exact(ROOM_RECORD_BYTES);
                        previous
                            .table(0)
                            .guest_chunks_exact(ROOM_RECORD_BYTES)
                            .all(|old| {
                                new_rooms.any(|new| {
                                    codec::equal_bytes(new.guest_index(..32), old.guest_index(..32))
                                })
                            })
                    });
                }
            }
        }
        if used != input.len() {
            return Err(NON_CANONICAL);
        }
        Ok(book)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn get(&self, table: usize, id: Digest32) -> Option<&'a [u8]> {
        let offset = if table == 0 || table >= 5 { 0 } else { 32 };
        let width = *WIDTHS.guest_index(table);
        let rows = *self.tables.guest_index(table);
        let mut low = 0;
        let mut high = *self.counts.guest_index(table);
        while low < high {
            let mid = low + (high - low) / 2;
            let row = rows.guest_index(mid * width..(mid + 1) * width);
            match crate::types::identity_order(
                row.guest_index(offset..offset + 32)
                    .as_chunks::<32>()
                    .0
                    .guest_index(0),
                id.as_bytes(),
            ) {
                core::cmp::Ordering::Less => low = mid + 1,
                core::cmp::Ordering::Greater => high = mid,
                core::cmp::Ordering::Equal => return Some(row),
            }
        }
        None
    }

    // The room rows were strictly validated before the dependent tables. Joins
    // consume only these two fields, without decoding the profile for every order.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn placement_room(&self, id: Digest32) -> CodecResult<(Digest32, RoomState)> {
        let bytes = self.get(0, id).ok_or(NON_CANONICAL)?;
        Ok((
            Digest32::new(
                bytes
                    .guest_index(64..96)
                    .try_into()
                    .map_err(|_| NON_CANONICAL)?,
            )?,
            RoomState::decode(*bytes.guest_index(4 * 32 + ROOM_PROFILE_BYTES))?,
        ))
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn room(&self, id: Digest32) -> CodecResult<Option<RoomRecord>> {
        self.get(0, id).map(RoomRecord::decode).transpose()
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn ask(&self, id: Digest32) -> CodecResult<Option<AskRecord>> {
        self.get(1, id).map(AskRecord::decode).transpose()
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn bid(&self, id: Digest32) -> CodecResult<Option<BidRecord>> {
        self.get(2, id).map(BidRecord::decode).transpose()
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn deal(&self, id: Digest32) -> CodecResult<Option<DealRecord>> {
        self.get(3, id).map(DealRecord::decode).transpose()
    }
    pub fn counts(&self) -> [usize; 4] {
        [
            *self.counts.guest_index(0),
            *self.counts.guest_index(1),
            *self.counts.guest_index(2),
            *self.counts.guest_index(3),
        ]
    }
    pub(crate) fn table(&self, index: usize) -> &'a [u8] {
        *self.tables.guest_index(index)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn bond(
        &self,
        id: Digest32,
    ) -> CodecResult<Option<crate::rewards::bonds::BondRecord>> {
        if let Some(row) = self.get(4, id) {
            return Ok(Some(crate::rewards::bonds::BondRecord::decode(row)?));
        }
        let Some(bid) = self.bid(id)? else {
            return Ok(None);
        };
        let identity =
            BondIdentity::decode(self.get(5, bond_identity_key(&bid)?).ok_or(NON_CANONICAL)?)?;
        let mut bond = crate::rewards::bonds::BondRecord {
            room: bid.room_id,
            bid: id,
            owner: bid.peer_key,
            worker: identity.worker,
            recipient: crate::types::AccountId::new(bid.peer_key.bytes())?,
            amount: bid.bond_amount,
            slashed: 0,
            locked_height: identity.height,
            refunded: false,
        };
        if let Some(row) = self.get(6, id) {
            let value = BondAdjustment::decode(row)?;
            bond.slashed = value.slashed;
            bond.locked_height = value.height;
            bond.refunded = value.refunded;
        }
        if bond.slashed > bond.amount {
            return Err(NON_CANONICAL);
        }
        Ok(Some(bond))
    }

    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn held(&self) -> CodecResult<u128> {
        let mut sum = 0u128;
        for table in [2, 4] {
            for row in self
                .table(table)
                .guest_chunks_exact(*WIDTHS.guest_index(table))
            {
                let id = Digest32::new(
                    row.guest_index(32..64)
                        .try_into()
                        .map_err(|_| NON_CANONICAL)?,
                )?;
                sum = sum
                    .checked_add(self.bond(id)?.ok_or(NON_CANONICAL)?.remaining()?)
                    .ok_or(ARITHMETIC)?;
            }
        }
        Ok(sum)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn held_committed(&self) -> CodecResult<u128> {
        let mut sum = 0u128;
        let mut adjustments = self.table(6).guest_chunks_exact(57).peekable();
        for row in self.table(2).guest_chunks_exact(BID_RECORD_BYTES) {
            let id: &[u8; 32] = row
                .guest_index(32..64)
                .try_into()
                .map_err(|_| NON_CANONICAL)?;
            let amount = u128::from_be_bytes(
                row.guest_index(4 * 32 + 16..4 * 32 + 32)
                    .try_into()
                    .map_err(|_| NON_CANONICAL)?,
            );
            if amount == 0 {
                return Err(NON_CANONICAL);
            }
            let (slashed, refunded) = match adjustments.peek() {
                Some(bytes) if bytes.guest_index(..32) == id => {
                    let adjustment =
                        BondAdjustment::decode(adjustments.next().ok_or(NON_CANONICAL)?)?;
                    (adjustment.slashed, adjustment.refunded)
                }
                _ => (0, false),
            };
            if slashed > amount {
                return Err(NON_CANONICAL);
            }
            sum = sum
                .checked_add(if refunded { 0 } else { amount - slashed })
                .ok_or(ARITHMETIC)?;
        }
        for row in self.table(4).guest_chunks_exact(BOND_RECORD_BYTES) {
            let bond = crate::rewards::bonds::BondRecord::decode(row)?;
            sum = sum.checked_add(bond.remaining()?).ok_or(ARITHMETIC)?;
        }
        Ok(sum)
    }

    pub(crate) fn bond_capacity(&self) -> CodecResult<()> {
        if self.counts.guest_index(2) + self.counts.guest_index(4) >= BID_CAPACITY {
            return Err(CAPACITY);
        }
        Ok(())
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn write_bond(
        &self,
        bond: &crate::rewards::bonds::BondRecord,
        out: &mut [u8],
    ) -> CodecResult<usize> {
        let mut row = [0; BOND_RECORD_BYTES];
        if self.bid(bond.bid)?.is_some() {
            let mut w = Writer::new(&mut row);
            crate::codec::write_fields!(w; id32: bond.bid.as_bytes(); u128: bond.slashed;
                u64: bond.locked_height; u8: u8::from(bond.refunded);)?;
            self.write(
                6,
                bond.bid,
                row.guest_index(..57),
                self.get(6, bond.bid).is_some(),
                out,
            )
        } else {
            bond.encode(&mut Writer::new(&mut row))?;
            self.write(4, bond.bid, &row, self.get(4, bond.bid).is_some(), out)
        }
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn post_bond(&self, bid: &BidRecord, first: &mut [u8]) -> CodecResult<usize> {
        let bond = self.bond(bid.bid_id)?.ok_or(F11_BOND_INSUFFICIENT)?;
        let key = bond_identity_key(bid)?;
        let identity = if let Some(row) = self.get(5, key) {
            let identity = BondIdentity::decode(row)?;
            if identity.worker != bond.worker {
                return Err(CONFLICT);
            }
            identity
        } else {
            BondIdentity {
                key,
                worker: bond.worker,
                height: bond.locked_height,
            }
        };
        let mut row = [0; 72];
        let mut w = Writer::new(&mut row);
        crate::codec::write_fields!(w; id32: identity.key.as_bytes(); id32: identity.worker.as_bytes(); u64: identity.height;)?;
        let mut bytes = [0; BID_RECORD_BYTES];
        bid.encode(&mut bytes)?;
        let mut adjustment = [0; 57];
        let mut edits = [
            (5, key, row.as_slice(), self.get(5, key).is_some()),
            (4, bid.bid_id, &[][..], true),
            (2, bid.bid_id, bytes.as_slice(), false),
            (6, bid.bid_id, &[][..], false),
        ];
        let count = if identity.height != bond.locked_height || bond.slashed != 0 || bond.refunded {
            let mut w = Writer::new(&mut adjustment);
            crate::codec::write_fields!(w; id32: bond.bid.as_bytes(); u128: bond.slashed;
                u64: bond.locked_height; u8: u8::from(bond.refunded);)?;
            edits[3] = (
                6,
                bid.bid_id,
                &adjustment,
                self.get(6, bid.bid_id).is_some(),
            );
            4
        } else {
            3
        };
        self.write_edits(&edits[..count], first)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn write(
        &self,
        table: usize,
        id: Digest32,
        record: &[u8],
        replace: bool,
        out: &mut [u8],
    ) -> CodecResult<usize> {
        self.write_edits(&[(table, id, record, replace)], out)
    }

    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write_edits(
        &self,
        edits: &[(usize, Digest32, &[u8], bool)],
        out: &mut [u8],
    ) -> CodecResult<usize> {
        let mut counts = self.counts;
        for (index, &(table, id, record, replace)) in edits.iter().enumerate() {
            if edits[..index].iter().any(|edit| edit.0 == table)
                || record.len() != *WIDTHS.guest_index(table) && !(replace && record.is_empty())
            {
                return Err(NON_CANONICAL);
            }
            if self.get(table, id).is_some() != replace {
                return Err(CONFLICT);
            }
            if record.is_empty() {
                *counts.guest_index_mut(table) -= 1;
            } else if !replace {
                *counts.guest_index_mut(table) += 1;
            }
            if *counts.guest_index(table) > *CAPS.guest_index(table) {
                return Err(CAPACITY);
            }
        }
        let mut w = Writer::new(out);
        w.put(b"F11B")?;
        w.u16(ROOM_BOOK_VERSION)?;
        for count in counts {
            w.u16(count as u16)?;
        }
        for (i, width) in WIDTHS.iter().enumerate() {
            let Some(&(_, id, record, replace)) = edits.iter().find(|edit| edit.0 == i) else {
                w.put(*self.tables.guest_index(i))?;
                continue;
            };
            let offset = if i == 0 || i >= 5 { 0 } else { 32 };
            let index = self
                .tables
                .guest_index(i)
                .guest_chunks_exact(*width)
                .take_while(|b| {
                    crate::types::identity_order(
                        b.guest_index(offset..offset + 32)
                            .as_chunks::<32>()
                            .0
                            .guest_index(0),
                        id.as_bytes(),
                    )
                    .is_lt()
                })
                .count();
            let at = index * width;
            w.put((*self.tables.guest_index(i)).guest_index(..at))?;
            w.put(record)?;
            w.put(
                (*self.tables.guest_index(i)).guest_index(at + if replace { *width } else { 0 }..),
            )?;
        }
        Ok(w.len())
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
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

#[cfg_attr(target_arch = "wasm32", inline(never))]
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

#[cfg_attr(target_arch = "wasm32", inline(never))]
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
    let mut ask = book.ask(request.ask_id)?.ok_or(NOT_FOUND)?;
    if ask.room_id != room.room_id
        || ask.model_digest != room.model_digest
        || ask.state == AskState::Closed
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
    let peer_count = record.peer_count()?;
    if record.agreed_price_per_thousand_tokens > ask.maximum_price_per_thousand_tokens {
        return Err(CONFLICT);
    }
    let mut seen_peers = [[0u8; 32]; MAX_DEAL_PEERS];
    let mut seen_prefix = [0u64; 4];
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
        let prefix = bid.peer_key.as_bytes()[0];
        let word = usize::from(prefix >> 6);
        let mask = 1u64 << (prefix & 63);
        if *seen_prefix.guest_index(word) & mask != 0
            && seen_peers
                .guest_index(..i)
                .iter()
                .any(|previous| codec::equal_bytes(previous, bid.peer_key.as_bytes()))
        {
            return Err(CONFLICT);
        }
        *seen_prefix.guest_index_mut(word) |= mask;
        seen_peers
            .guest_index_mut(i)
            .copy_from_slice(bid.peer_key.as_bytes());
        if bid.bond_amount < room.profile.minimum_bond {
            return Err(F11_BOND_INSUFFICIENT);
        }
    }
    let mut deal_bytes = [0; DEAL_RECORD_BYTES];
    record.encode_with_count(peer_count, &mut deal_bytes)?;
    ask.state = AskState::Matched;
    let mut ask_bytes = [0; ASK_RECORD_BYTES];
    ask.encode(&mut ask_bytes)?;
    let n = book.write_edits(
        &[
            (3, record.deal_id, &deal_bytes, false),
            (1, ask.ask_id, &ask_bytes, true),
        ],
        out,
    )?;
    Ok(n)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
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

    let policy = PolicySection::decode(state.section(Section::PolicyLifecycle)?)?;
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

    let workers = if matches!(op, dispatch::PostBid | dispatch::RecordDeal)
        || op != dispatch::PostAsk && ctx.principal != header.owner_principal
    {
        Some(WorkerTable::decode(
            split_identity_section(state.section(Section::IdentityRoster)?)?.0,
        )?)
    } else {
        None
    };

    let book_bytes = state.room_market()?.1;
    let book = if matches!(op, dispatch::PostBid | dispatch::RecordDeal) {
        RoomBook::committed(book_bytes)?
    } else {
        RoomBook::decode(book_bytes)?
    };

    let mut r = Reader::new(e.payload);
    let room_id = r.digest32()?;
    let mut bid_worker = None;
    if op == dispatch::PostBid {
        let _bid = r.digest32()?;
        bid_worker = Some(worker_for(
            workers.as_ref().ok_or(UNAUTHORIZED)?,
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
                    .as_ref()
                    .ok_or(UNAUTHORIZED)?
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
                profile: r.room_profile()?,
                state: RoomState::Open,
                opened_at_epoch: epoch,
                closed_height: 0,
            };
            r.finish()?;
            let mut bytes = [0; ROOM_RECORD_BYTES];
            room.encode(&mut bytes)?;
            let n = book.write(0, room_id, &bytes, false, first)?;
            (first.guest_index(..n), room_id)
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
            (first.guest_index(..n), ask.ask_id)
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
            // The final placement scan validates this bid in the committed book.
            // A staging scan here would traverse the full book a second time.
            crate::rewards::bonds::check_bid(ctx, envelope, current)?;
            let n = book.post_bond(&bid, first)?;
            (first.guest_index(..n), bid.bid_id)
        }
        dispatch::RecordDeal => {
            let request = RecordDealRequest::decode(e.payload)?;
            if e.roster != Presence::Present(request.roster_digest) {
                return Err(F11_ROSTER_DIGEST_MISMATCH);
            }
            let admission_bytes = split_joint(state.section(Section::ReputationAdmission)?)?.1;
            let n = AdmissionTable::with_decoded(admission_bytes, |admission| {
                apply_record_deal(
                    &book,
                    &request,
                    ctx.principal,
                    admission,
                    workers.as_ref().ok_or(UNAUTHORIZED)?,
                    epoch,
                    ctx.height,
                    first,
                )
            })?;
            (first.guest_index(..n), request.deal_id)
        }
        dispatch::CloseRoom => {
            let mut room = book.open_room(room_id)?;
            if room.host_key != ctx.principal {
                return Err(UNAUTHORIZED);
            }
            for bytes in book
                .tables
                .guest_index(3)
                .as_chunks::<{ DEAL_RECORD_BYTES }>()
                .0
                .iter()
                .map(|chunk| chunk.as_slice())
            {
                let deal = DealRecord::decode(bytes)?;
                if deal.room_id == room_id && deal.state == DealState::Open {
                    return Err(WRONG_PHASE);
                }
            }
            room.state = RoomState::Closed;
            room.closed_height = ctx.height;
            let mut bytes = [0; ROOM_RECORD_BYTES];
            room.encode(&mut bytes)?;
            let n = book.write(0, room_id, &bytes, true, first)?;
            (first.guest_index(..n), room_id)
        }
        _ => return Err(UNKNOWN_OPERATION),
    };

    let settlement_len = state.encode_room_market(book_bytes, settlement)?;

    let outcome = crate::rewards::finish_room_transition(
        ctx.height,
        envelope,
        state,
        policy,
        settlement.guest_index(..settlement_len),
        subject.as_bytes(),
        replay.as_ref(),
        policy_out,
        control,
        next,
        event,
    )?;

    Ok(outcome)
}
