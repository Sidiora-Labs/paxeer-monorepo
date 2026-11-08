use crate::types::GuestIndex as _;
use crate::{
    codec::{self, Envelope, Reader, ValidatedEnvelope, Writer},
    dispatch,
    errors::{
        CodecResult, BAD_VERSION, CAPACITY, CONFLICT, NON_CANONICAL, UNAUTHORIZED,
        UNKNOWN_OPERATION,
    },
    registry_ops::CallContext,
    rooms::{self, AskState, RoomBook, RoomState},
    state,
    types::{Authentication, Digest32, Presence, RequestDigest},
};

pub const SIDE_CAPACITY: usize = 64;
const ASK_VIEW_BYTES: usize = 96;
const BID_VIEW_BYTES: usize = 80;
const CROSSING_BYTES: usize = SIDE_CAPACITY * SIDE_CAPACITY / 8;
const VIEW_HEADER_BYTES: usize = 40;
pub const BOOK_VIEW_MAX_BYTES: usize =
    VIEW_HEADER_BYTES + SIDE_CAPACITY * (ASK_VIEW_BYTES + BID_VIEW_BYTES) + CROSSING_BYTES;
const _: () = assert!(BOOK_VIEW_MAX_BYTES <= crate::MAX_RESULT_PAYLOAD_BYTES);

pub struct BookRequest<'a> {
    pub envelope: Envelope<'a>,
    unsigned: &'a [u8],
}
impl BookRequest<'_> {
    pub fn request_digest(&self) -> CodecResult<RequestDigest> {
        RequestDigest::new(codec::domain_hash("PAXAI/request/v1", self.unsigned)?.bytes())
    }
}

pub fn is_book_view(input: &[u8]) -> bool {
    matches!(input.get(8..10), Some([0x0b, 0x14]))
}

fn validate_book_envelope(envelope: &Envelope<'_>) -> CodecResult<()> {
    if envelope.operation != dispatch::BookView {
        return Err(UNKNOWN_OPERATION);
    }
    envelope
        .operation
        .validate_payload_length(envelope.payload.len())?;
    if envelope.authentication != Authentication::Native {
        return Err(UNAUTHORIZED);
    }
    if envelope.epoch != 0
        || envelope.config != 0
        || envelope.roster != Presence::Absent
        || envelope.sequence != 0
        || envelope.expiry == 0
    {
        return Err(NON_CANONICAL);
    }
    let mut r = Reader::new(envelope.payload);
    r.digest32()?;
    r.finish()
}

pub fn encode_book_envelope(envelope: &Envelope<'_>, out: &mut [u8]) -> CodecResult<usize> {
    validate_book_envelope(envelope)?;
    codec::encode_envelope_fields(envelope, out)
}

pub fn decode_book_envelope(input: &[u8]) -> CodecResult<BookRequest<'_>> {
    let (envelope, unsigned) = codec::decode_envelope_fields(input, false)?;
    validate_book_envelope(&envelope)?;
    Ok(BookRequest { envelope, unsigned })
}

/// Sorted indexes into the persisted room records. Terminal records remain available
/// for replay and history; the placement book contains only open, unconsumed orders.
pub struct PlacementBook<'a> {
    pub model_digest: Digest32,
    ask_rows: &'a [u8],
    bid_rows: &'a [u8],
    asks: [u8; SIDE_CAPACITY],
    bids: [u8; SIDE_CAPACITY],
    counts: [usize; 2],
}

fn insert(
    indices: &mut [u8; SIDE_CAPACITY],
    count: &mut usize,
    rows: &[u8],
    width: usize,
    index: usize,
    descending: bool,
) -> CodecResult<()> {
    if *count == SIDE_CAPACITY {
        return Err(CAPACITY);
    }
    let row = rows.guest_index(index * width..(index + 1) * width);
    let at = indices
        .guest_index(..*count)
        .iter()
        .position(|previous| {
            let other = (*rows.guest_index(usize::from(*previous) * width..)).guest_index(..width);
            let price_order = row.guest_index(128..144).cmp(other.guest_index(128..144));
            (if descending {
                price_order.reverse()
            } else {
                price_order
            })
            .then_with(|| row.guest_index(32..64).cmp(other.guest_index(32..64)))
            .is_lt()
        })
        .unwrap_or(*count);
    indices.guest_copy_within(at..*count, at + 1);
    *indices.guest_index_mut(at) = u8::try_from(index).map_err(|_| CAPACITY)?;
    *count += 1;
    Ok(())
}

impl<'a> PlacementBook<'a> {
    pub fn decode(bytes: &'a [u8], model_digest: Digest32) -> CodecResult<Self> {
        let book = RoomBook::decode(bytes)?;
        let counts = book.counts();
        let mut r = Reader::new(bytes);
        if !bytes.is_empty() {
            r.take(14 + *counts.guest_index(0) * rooms::ROOM_RECORD_BYTES)?;
        }
        let ask_rows = r.take(*counts.guest_index(1) * rooms::ASK_RECORD_BYTES)?;
        let bid_rows = r.take(*counts.guest_index(2) * rooms::BID_RECORD_BYTES)?;
        let deals = r.take(*counts.guest_index(3) * rooms::DEAL_RECORD_BYTES)?;
        r.finish()?;
        let mut consumed = [false; rooms::BID_CAPACITY];
        for bytes in deals.guest_chunks_exact(rooms::DEAL_RECORD_BYTES) {
            let deal = rooms::DealRecord::decode(bytes)?;
            for split in deal.layers.iter().flatten() {
                let index = bid_rows
                    .guest_chunks_exact(rooms::BID_RECORD_BYTES)
                    .position(|row| row.guest_index(32..64) == split.bid_id.as_bytes())
                    .ok_or(NON_CANONICAL)?;
                if *consumed.guest_index(index) {
                    return Err(CONFLICT);
                }
                *consumed.guest_index_mut(index) = true;
            }
        }
        let mut value = Self {
            model_digest,
            ask_rows,
            bid_rows,
            asks: [0; SIDE_CAPACITY],
            bids: [0; SIDE_CAPACITY],
            counts: [0; 2],
        };
        for (index, row) in ask_rows
            .guest_chunks_exact(rooms::ASK_RECORD_BYTES)
            .enumerate()
        {
            let ask = rooms::AskRecord::decode(row)?;
            let room = book.room(ask.room_id)?.ok_or(NON_CANONICAL)?;
            if ask.state == AskState::Open
                && room.state == RoomState::Open
                && ask.model_digest == model_digest
            {
                insert(
                    &mut value.asks,
                    value.counts.guest_index_mut(0),
                    ask_rows,
                    rooms::ASK_RECORD_BYTES,
                    index,
                    true,
                )?;
            }
        }
        for (index, row) in bid_rows
            .guest_chunks_exact(rooms::BID_RECORD_BYTES)
            .enumerate()
        {
            let bid = rooms::BidRecord::decode(row)?;
            let room = book.room(bid.room_id)?.ok_or(NON_CANONICAL)?;
            if !*consumed.guest_index(index)
                && room.state == RoomState::Open
                && room.model_digest == model_digest
            {
                insert(
                    &mut value.bids,
                    value.counts.guest_index_mut(1),
                    bid_rows,
                    rooms::BID_RECORD_BYTES,
                    index,
                    false,
                )?;
            }
        }
        Ok(value)
    }

    pub fn counts(&self) -> [usize; 2] {
        self.counts
    }

    pub fn ask(&self, index: usize) -> CodecResult<rooms::AskRecord> {
        let index = usize::from(
            *self
                .asks
                .get(..*self.counts.guest_index(0))
                .and_then(|s| s.get(index))
                .ok_or(CAPACITY)?,
        );
        rooms::AskRecord::decode(
            (*self.ask_rows.guest_index(index * rooms::ASK_RECORD_BYTES..))
                .guest_index(..rooms::ASK_RECORD_BYTES),
        )
    }

    pub fn bid(&self, index: usize) -> CodecResult<rooms::BidRecord> {
        let index = usize::from(
            *self
                .bids
                .get(..*self.counts.guest_index(1))
                .and_then(|s| s.get(index))
                .ok_or(CAPACITY)?,
        );
        rooms::BidRecord::decode(
            (*self.bid_rows.guest_index(index * rooms::BID_RECORD_BYTES..))
                .guest_index(..rooms::BID_RECORD_BYTES),
        )
    }

    pub fn encode_view(&self, out: &mut [u8]) -> CodecResult<usize> {
        let len = VIEW_HEADER_BYTES
            + *self.counts.guest_index(0) * ASK_VIEW_BYTES
            + *self.counts.guest_index(1) * BID_VIEW_BYTES
            + CROSSING_BYTES;
        let mut w = Writer::new(out.get_mut(..len).ok_or(CAPACITY)?);
        crate::codec::write_fields!(w;
            put: b"F11P";
            u16: 1_u16;
            id32: self.model_digest.as_bytes();
            u8: (*(self.counts).guest_index(0)) as u8;
            u8: (*(self.counts).guest_index(1)) as u8;
        )?;
        for index in 0..*self.counts.guest_index(0) {
            let ask = self.ask(index)?;
            crate::codec::write_fields!(w;
                id32: ask.room_id.as_bytes();
                id32: ask.ask_id.as_bytes();
                u128: ask.maximum_price_per_thousand_tokens;
                u128: ask.stream_id;
            )?;
        }
        for index in 0..*self.counts.guest_index(1) {
            let bid = self.bid(index)?;
            crate::codec::write_fields!(w;
                id32: bid.room_id.as_bytes();
                id32: bid.bid_id.as_bytes();
                u128: bid.minimum_price_per_thousand_tokens;
            )?;
        }
        let at = w.len();
        w.put(&[0; CROSSING_BYTES])?;
        let (orders, crossings) = out.guest_index_mut(..len).guest_split_at_mut(at);
        let (asks, bids) = orders
            .guest_index(VIEW_HEADER_BYTES..)
            .guest_split_at(*self.counts.guest_index(0) * ASK_VIEW_BYTES);
        for (a, ask) in asks
            .as_chunks::<{ ASK_VIEW_BYTES }>()
            .0
            .iter()
            .map(|chunk| chunk.as_slice())
            .enumerate()
        {
            for (b, bid) in bids
                .as_chunks::<{ BID_VIEW_BYTES }>()
                .0
                .iter()
                .map(|chunk| chunk.as_slice())
                .enumerate()
            {
                if crosses(ask, bid) {
                    let bit = a * SIDE_CAPACITY + b;
                    *crossings.guest_index_mut(bit / 8) |= 1 << (bit % 8);
                }
            }
        }
        Ok(len)
    }
}

fn crosses(ask: &[u8], bid: &[u8]) -> bool {
    *ask.guest_index(..32) == *bid.guest_index(..32)
        && *ask.guest_index(64..80) >= *bid.guest_index(64..80)
        && *ask.guest_index(80..96) != [0; 16]
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Placement {
    pub room_id: Digest32,
    pub order_id: Digest32,
    pub price_per_thousand_tokens: u128,
    pub stream_id: u128,
}
impl Placement {
    fn decode(bytes: &[u8], ask: bool) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let value = Self {
            room_id: r.digest32()?,
            order_id: r.digest32()?,
            price_per_thousand_tokens: r.u128()?,
            stream_id: if ask { r.u128()? } else { 0 },
        };
        r.finish()?;
        if value.price_per_thousand_tokens == 0 {
            return Err(NON_CANONICAL);
        }
        Ok(value)
    }
}

/// Wire view: version, model, sorted ask/bid rows and a 64 by 64 crossing bitmap.
/// Each set bit names an ask/bid index pair; asks also retain their funding stream.
pub struct BookView<'a> {
    pub model_digest: Digest32,
    asks: &'a [u8],
    bids: &'a [u8],
    crossings: &'a [u8],
}
impl<'a> BookView<'a> {
    pub fn decode(bytes: &'a [u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        if r.take(4)? != b"F11P" {
            return Err(NON_CANONICAL);
        }
        if r.u16()? != 1 {
            return Err(BAD_VERSION);
        }
        let model_digest = r.digest32()?;
        let a = usize::from(r.u8()?);
        let b = usize::from(r.u8()?);
        if a > SIDE_CAPACITY || b > SIDE_CAPACITY {
            return Err(CAPACITY);
        }
        let value = Self {
            model_digest,
            asks: r.take(a * ASK_VIEW_BYTES)?,
            bids: r.take(b * BID_VIEW_BYTES)?,
            crossings: r.take(CROSSING_BYTES)?,
        };
        r.finish()?;
        for (rows, width, ask) in [
            (value.asks, ASK_VIEW_BYTES, true),
            (value.bids, BID_VIEW_BYTES, false),
        ] {
            let mut previous: Option<Placement> = None;
            for bytes in rows.guest_chunks_exact(width) {
                let row = Placement::decode(bytes, ask)?;
                if let Some(p) = previous {
                    let order = p
                        .price_per_thousand_tokens
                        .cmp(&row.price_per_thousand_tokens);
                    if (if ask { order.reverse() } else { order })
                        .then_with(|| p.order_id.cmp(&row.order_id))
                        .is_ge()
                    {
                        return Err(NON_CANONICAL);
                    }
                }
                previous = Some(row);
            }
        }
        for ai in 0..SIDE_CAPACITY {
            for bi in 0..SIDE_CAPACITY {
                let expected = ai < a
                    && bi < b
                    && crosses(
                        (*value.asks.guest_index(ai * ASK_VIEW_BYTES..))
                            .guest_index(..ASK_VIEW_BYTES),
                        (*value.bids.guest_index(bi * BID_VIEW_BYTES..))
                            .guest_index(..BID_VIEW_BYTES),
                    );
                let bit = ai * SIDE_CAPACITY + bi;
                if (*value.crossings.guest_index(bit / 8) & (1 << (bit % 8)) != 0) != expected {
                    return Err(NON_CANONICAL);
                }
            }
        }
        Ok(value)
    }

    pub fn counts(&self) -> [usize; 2] {
        [
            self.asks.len() / ASK_VIEW_BYTES,
            self.bids.len() / BID_VIEW_BYTES,
        ]
    }

    pub fn ask(&self, index: usize) -> CodecResult<Placement> {
        Placement::decode(
            self.asks
                .as_chunks::<{ ASK_VIEW_BYTES }>()
                .0
                .iter()
                .map(|chunk| chunk.as_slice())
                .nth(index)
                .ok_or(CAPACITY)?,
            true,
        )
    }

    pub fn bid(&self, index: usize) -> CodecResult<Placement> {
        Placement::decode(
            self.bids
                .as_chunks::<{ BID_VIEW_BYTES }>()
                .0
                .iter()
                .map(|chunk| chunk.as_slice())
                .nth(index)
                .ok_or(CAPACITY)?,
            false,
        )
    }

    pub fn crossing_pairs(&self) -> impl Iterator<Item = (usize, usize)> + '_ {
        self.crossings.iter().enumerate().flat_map(|(byte, bits)| {
            (0..8).filter_map(move |offset| {
                (bits & (1 << offset) != 0).then_some((
                    (byte * 8 + offset) / SIDE_CAPACITY,
                    (byte * 8 + offset) % SIDE_CAPACITY,
                ))
            })
        })
    }
}

pub fn apply(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<rooms::Outcome> {
    let outcome = rooms::apply(ctx, envelope, current, next, scratch, event)?;
    if let rooms::Outcome::Applied { state_len, .. } = &outcome {
        let shared = state::decode_shared_state(next.guest_index(..*state_len))?;
        let bytes = shared.room_market()?.1;
        let mut r = Reader::new(envelope.envelope.payload);
        let room_id = r.digest32()?;
        let book = RoomBook::decode(bytes)?;
        let model = book.room(room_id)?.ok_or(NON_CANONICAL)?.model_digest;
        PlacementBook::decode(bytes, model)?;
    }
    Ok(outcome)
}
