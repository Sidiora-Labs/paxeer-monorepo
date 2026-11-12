use crate::types::GuestIndex as _;
use crate::{
    codec::{self, Envelope, Reader, ValidatedEnvelope, Writer},
    dispatch,
    errors::{
        BAD_VERSION, CAPACITY, CONFLICT, CodecResult, NON_CANONICAL, UNAUTHORIZED,
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

// Prices are canonical big-endian integers. Compare words rather than repeatedly
// scanning the leading zero bytes while building a full placement index.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn price_words(bytes: &[u8]) -> (u64, u64) {
    let words = bytes.as_chunks::<8>().0;
    (
        u64::from_be_bytes(*words.guest_index(0)),
        u64::from_be_bytes(*words.guest_index(1)),
    )
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
    let price = price_words(row.guest_index(128..144));
    let at = indices.guest_index(..*count).partition_point(|previous| {
        let other = (*rows.guest_index(usize::from(*previous) * width..)).guest_index(..width);
        let price_order = price.cmp(&price_words(other.guest_index(128..144)));
        (if descending {
            price_order.reverse()
        } else {
            price_order
        })
        .then_with(|| {
            crate::types::identity_order(
                row.guest_index(32..64).as_chunks::<32>().0.guest_index(0),
                other.guest_index(32..64).as_chunks::<32>().0.guest_index(0),
            )
        })
        .is_ge()
    });
    indices.guest_copy_within(at..*count, at + 1);
    *indices.guest_index_mut(at) = u8::try_from(index).map_err(|_| CAPACITY)?;
    *count += 1;
    Ok(())
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn scan_order(
    indices: Option<&mut [u8; SIDE_CAPACITY]>,
    count: &mut usize,
    rows: &[u8],
    width: usize,
    index: usize,
    descending: bool,
) -> CodecResult<()> {
    if let Some(indices) = indices {
        return insert(indices, count, rows, width, index, descending);
    }
    if *count == SIDE_CAPACITY {
        return Err(CAPACITY);
    }
    *count += 1;
    Ok(())
}

impl<'a> PlacementBook<'a> {
    pub fn decode(bytes: &'a [u8], model_digest: Digest32) -> CodecResult<Self> {
        Self::from_book(&RoomBook::decode(bytes)?, model_digest)
    }

    fn from_book(book: &RoomBook<'a>, model_digest: Digest32) -> CodecResult<Self> {
        let mut asks = [0; SIDE_CAPACITY];
        let mut bids = [0; SIDE_CAPACITY];
        let counts = Self::scan_book(book, model_digest, Some((&mut asks, &mut bids)))?;
        Ok(Self {
            model_digest,
            ask_rows: book.table(1),
            bid_rows: book.table(2),
            asks,
            bids,
            counts,
        })
    }

    // Mutations need the complete validity/capacity scan, but have no sorted
    // placement output. Views additionally build their canonical order indexes.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn scan_book(
        book: &RoomBook<'a>,
        model_digest: Digest32,
        mut indices: Option<(&mut [u8; SIDE_CAPACITY], &mut [u8; SIDE_CAPACITY])>,
    ) -> CodecResult<[usize; 2]> {
        let ask_rows = book.table(1);
        let bid_rows = book.table(2);
        let mut previous_room: Option<(Digest32, (Digest32, RoomState))> = None;
        let mut room_fields = |id| {
            if let Some((previous, fields)) = previous_room {
                if previous == id {
                    return Ok(fields);
                }
            }
            let fields = book.placement_room(id)?;
            previous_room = Some((id, fields));
            Ok::<_, crate::errors::ApplicationError>(fields)
        };
        let deals = book.table(3);
        // Zero means unconsumed; otherwise retain a one-based deal index so that
        // every later use must name the same ask without changing stored records.
        let mut consumed = [0u8; rooms::BID_CAPACITY];
        for (deal_index, bytes) in deals
            .guest_chunks_exact(rooms::DEAL_RECORD_BYTES)
            .enumerate()
        {
            // RoomBook validated this deal, including its count and layer cells.
            let count = usize::from(*bytes.guest_index(144));
            for split in bytes
                .guest_index(146..146 + count * 36)
                .guest_chunks_exact(36)
            {
                let bid_id = split.guest_index(..32);
                let rows = bid_rows.as_chunks::<{ rooms::BID_RECORD_BYTES }>().0;
                let index = rows.partition_point(|row| {
                    crate::types::identity_order(
                        row.guest_index(32..64).as_chunks::<32>().0.guest_index(0),
                        bid_id.as_chunks::<32>().0.guest_index(0),
                    )
                    .is_lt()
                });
                if index == rows.len()
                    || !codec::equal_bytes(rows.guest_index(index).guest_index(32..64), bid_id)
                {
                    return Err(NON_CANONICAL);
                }
                let previous = usize::from(*consumed.guest_index(index));
                if previous != 0 {
                    let ask_at = (previous - 1) * rooms::DEAL_RECORD_BYTES + 64;
                    if deals.guest_index(ask_at..ask_at + 32) != bytes.guest_index(64..96) {
                        return Err(CONFLICT);
                    }
                }
                *consumed.guest_index_mut(index) = (deal_index + 1) as u8;
            }
        }
        let mut counts = [0; 2];
        for (index, row) in ask_rows
            .guest_chunks_exact(rooms::ASK_RECORD_BYTES)
            .enumerate()
        {
            // RoomBook already validated every row. Join only the fields this
            // index reads instead of decoding the same complete order twice.
            let room_id = Reader::new(row).digest32()?;
            let (_, room_state) = room_fields(room_id)?;
            if *row.guest_index(4 * 32 + 2 * 16) == AskState::Open as u8
                && room_state == RoomState::Open
                && codec::equal_bytes(row.guest_index(3 * 32..4 * 32), model_digest.as_bytes())
            {
                scan_order(
                    indices.as_mut().map(|(asks, _)| &mut **asks),
                    counts.guest_index_mut(0),
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
            let room_id = Reader::new(row).digest32()?;
            let (room_model, room_state) = room_fields(room_id)?;
            if *consumed.guest_index(index) == 0
                && room_state == RoomState::Open
                && room_model == model_digest
            {
                scan_order(
                    indices.as_mut().map(|(_, bids)| &mut **bids),
                    counts.guest_index_mut(1),
                    bid_rows,
                    rooms::BID_RECORD_BYTES,
                    index,
                    false,
                )?;
            }
        }
        Ok(counts)
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
            let row_index = usize::from(*self.asks.guest_index(index));
            let row = self.ask_rows.guest_index(
                row_index * rooms::ASK_RECORD_BYTES..(row_index + 1) * rooms::ASK_RECORD_BYTES,
            );
            // RoomBook::decode validated this stored row before the index was built.
            w.put(row.guest_index(..64))?;
            w.put(row.guest_index(128..160))?;
        }
        for index in 0..*self.counts.guest_index(1) {
            let row_index = usize::from(*self.bids.guest_index(index));
            let row = self.bid_rows.guest_index(
                row_index * rooms::BID_RECORD_BYTES..(row_index + 1) * rooms::BID_RECORD_BYTES,
            );
            w.put(row.guest_index(..64))?;
            w.put(row.guest_index(128..144))?;
        }
        let at = w.len();
        w.put(&[0; CROSSING_BYTES])?;
        let (orders, crossings) = out.guest_index_mut(..len).guest_split_at_mut(at);
        let (asks, bids) = orders
            .guest_index(VIEW_HEADER_BYTES..)
            .guest_split_at(*self.counts.guest_index(0) * ASK_VIEW_BYTES);
        let bid_rows = bids.as_chunks::<{ BID_VIEW_BYTES }>().0;
        let mut room_keys = [[0u8; 32]; rooms::ROOM_CAPACITY];
        let mut room_masks = [0u64; rooms::ROOM_CAPACITY];
        let mut room_count = 0usize;
        for (b, bid) in bid_rows.iter().enumerate() {
            let room = bid.guest_index(..32);
            let index = if let Some(index) = room_keys
                .guest_index(..room_count)
                .iter()
                .position(|key| codec::equal_bytes(key, room))
            {
                index
            } else {
                let index = room_count;
                *room_keys.guest_index_mut(index) = *bid.as_chunks::<32>().0.guest_index(0);
                room_count += 1;
                index
            };
            *room_masks.guest_index_mut(index) |= 1u64 << b;
        }
        for (a, ask) in asks.as_chunks::<{ ASK_VIEW_BYTES }>().0.iter().enumerate() {
            let room_mask = room_keys
                .guest_index(..room_count)
                .iter()
                .position(|key| codec::equal_bytes(key, ask.guest_index(..32)))
                .map_or(0, |index| *room_masks.guest_index(index));
            let mask = if room_mask == 0 || codec::zero_bytes(ask.guest_index(80..96)) {
                0
            } else {
                let price = price_words(ask.guest_index(64..80));
                let eligible =
                    bid_rows.partition_point(|bid| price_words(bid.guest_index(64..80)) <= price);
                let affordable = if eligible == SIDE_CAPACITY {
                    u64::MAX
                } else {
                    (1u64 << eligible) - 1
                };
                room_mask & affordable
            };
            crossings
                .guest_index_mut(a * 8..(a + 1) * 8)
                .copy_from_slice(&mask.to_le_bytes());
        }
        Ok(len)
    }
}

fn crosses(ask: &[u8], bid: &[u8]) -> bool {
    codec::equal_bytes(ask.guest_index(..32), bid.guest_index(..32))
        && price_words(ask.guest_index(64..80)) >= price_words(bid.guest_index(64..80))
        && !codec::zero_bytes(ask.guest_index(80..96))
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

#[cfg_attr(target_arch = "wasm32", inline(never))]
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
        let sections = state::SectionReader::new(next.guest_index(..*state_len))?;
        let bytes = state::room_market_parts(sections.section(state::Section::SettlementClaims))?.1;
        let mut r = Reader::new(envelope.envelope.payload);
        let room_id = r.digest32()?;

        let book = if envelope.envelope.operation == dispatch::RecordDeal {
            RoomBook::committed(bytes)?
        } else {
            RoomBook::decode(bytes)?
        };

        let (model, _) = book.placement_room(room_id)?;
        PlacementBook::scan_book(&book, model, None)?;
    }
    Ok(outcome)
}
