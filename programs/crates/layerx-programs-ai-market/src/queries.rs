//! F10 read-only selectors, exact snapshot binding and bounded off-chain page framing.
//! Selectors read one captured common state value; they never write, emit, pay or stage blobs.
//! Host `ProgramRead` facts and finality evidence are inputs; nothing here establishes them.
use crate::types::GuestIndex as _;
#[path = "placement.rs"]
pub mod placement;
#[cfg(target_arch = "wasm32")]
use crate::codec::Sha256State as Sha256;
use crate::{
    codec::{self, ChunkResponse, ReadHeader, Reader, RosterView, Writer},
    dispatch,
    errors::{
        ApplicationError, CodecResult, ARITHMETIC, BAD_VERSION, CAPACITY, CONFLICT, NON_CANONICAL,
        NOT_FOUND, WRONG_DOMAIN, WRONG_MARKET, WRONG_PROGRAM, WRONG_ROSTER,
    },
    evaluators::authority::split_identity_section,
    registry_ops::PolicySection,
    state::{self, Section, SharedState},
    types::{
        AssetId, ChainDomain, Digest32, MarketId, MetadataDigest, PolicyDigest, Presence,
        PrincipalId, ProgramId, RequestDigest, RosterDigest, StateDigest, Version,
    },
    workers::{WorkerState, WorkerTable},
    MAX_CHUNK_BYTES, MAX_STATE_BYTES, SCHEMA_VERSION,
};
#[cfg(not(target_arch = "wasm32"))]
use sha2::{Digest, Sha256};

pub const FULL_CAPTURE_CHUNKS: usize = MAX_STATE_BYTES.div_ceil(MAX_CHUNK_BYTES);
/// Compatibility discriminator retained by the snapshot wire format and its consumers.
/// Finality comes from a verified block commit and the corresponding state proof;
/// this byte alone authenticates neither.
pub const FINALIZED_RANK: u8 = 4;
pub const PAGE_MAX_ROWS: usize = 32;
pub const PAGE_MAX_BYTES: usize = 65_536;
pub const DEFAULT_LIMIT: u8 = 16;
pub const CURSOR_MAX_BYTES: usize = 1_024;
pub const CURSOR_LIFETIME_MS: u64 = 900_000;
/// schema:u16, visibility32, market32, snapshot32, filter-hash32 precede the ordinal.
const CURSOR_ORDINAL_OFFSET: usize = 2 + 4 * 32;
const CURSOR_PAYLOAD_BYTES: usize = CURSOR_ORDINAL_OFFSET + 2 + 8 + 4;
const CURSOR_TAG_BYTES: usize = 32;
pub const CURSOR_TOKEN_BYTES: usize = 2 * (CURSOR_PAYLOAD_BYTES + CURSOR_TAG_BYTES);
/// Content-identity prefix through `roster_digest` with epoch and roster present.
const BINDING_PREFIX_MAX_BYTES: usize = 2 + 3 * 32 + 8 + 8 + 32 + 32 + 8 + 32 + 9 + 8 + 32 + 33;
pub const BINDING_MAX_BYTES: usize = BINDING_PREFIX_MAX_BYTES + 8 + 32 + 32 + 32 + 1 + 8 + 8;
const HEX: &[u8; 16] = b"0123456789abcdef";

pub fn answer_book_view(state_bytes: &[u8], payload: &[u8], out: &mut [u8]) -> CodecResult<usize> {
    let mut reader = Reader::new(payload);
    let model = reader.digest32()?;
    reader.finish()?;
    let shared = state::decode_shared_state(state_bytes)?;
    placement::PlacementBook::decode(shared.room_market()?.1, model)?.encode_view(out)
}

pub const ROOM_VIEW_BYTES: usize = crate::rooms::ROOM_RECORD_BYTES
    + 32 * (crate::rooms::ASK_CAPACITY + crate::rooms::BID_CAPACITY);
pub const DEAL_VIEW_BYTES: usize = crate::rooms::DEAL_RECORD_BYTES + 8 + 16;
pub const PEER_VIEW_BYTES: usize =
    32 * 4 + 8 * 5 + 2 + 1 + crate::workers::CapabilityFields::ENCODED_BYTES + 1 + 4;
pub const RECEIPT_BATCH_VIEW_BYTES: usize = crate::laps::BATCH_BYTES;
const _: () = assert!(ROOM_VIEW_BYTES <= crate::MAX_RESULT_PAYLOAD_BYTES);

#[derive(Clone, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct RoomView {
    pub room: crate::rooms::RoomRecord,
    pub open_asks: [Option<Digest32>; crate::rooms::ASK_CAPACITY],
    pub open_bids: [Option<Digest32>; crate::rooms::BID_CAPACITY],
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn read_order_ids<const N: usize>(r: &mut Reader<'_>) -> CodecResult<[Option<Digest32>; N]> {
    let mut ids = [None; N];
    let mut previous = None;
    let mut ended = false;
    for id in &mut ids {
        let bytes = r.fixed()?;
        if bytes == [0; 32] {
            ended = true;
        } else {
            let value = Digest32::new(bytes)?;
            if ended || previous.is_some_and(|p| p >= value) {
                return Err(NON_CANONICAL);
            }
            *id = Some(value);
            previous = Some(value);
        }
    }
    Ok(ids)
}
impl RoomView {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let value = Self {
            room: crate::rooms::RoomRecord::decode(r.take(crate::rooms::ROOM_RECORD_BYTES)?)?,
            open_asks: read_order_ids(&mut r)?,
            open_bids: read_order_ids(&mut r)?,
        };
        r.finish()?;
        if value.room.state == crate::rooms::RoomState::Closed
            && (value.open_asks.iter().any(Option::is_some)
                || value.open_bids.iter().any(Option::is_some))
        {
            return Err(NON_CANONICAL);
        }
        Ok(value)
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct DealView {
    pub deal: crate::rooms::DealRecord,
    pub settled_tokens: u64,
    pub settled_amount_owed: u128,
}
impl DealView {
    pub fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let value = Self {
            deal: crate::rooms::DealRecord::decode(r.take(crate::rooms::DEAL_RECORD_BYTES)?)?,
            settled_tokens: r.u64()?,
            settled_amount_owed: r.u128()?,
        };
        r.finish()?;
        Ok(value)
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct PeerView {
    pub peer_key: PrincipalId,
    pub worker: crate::types::WorkerId,
    pub delegate: crate::types::PublicKey32,
    pub metadata: MetadataDigest,
    pub generation: u64,
    pub key_version: u64,
    pub metadata_revision: u64,
    pub valid_from: u64,
    pub expiry: u64,
    pub state: WorkerState,
    pub enrolled: bool,
    pub capabilities: Option<crate::workers::CapabilityFields>,
    pub bond_status: Availability,
    pub fault_count: u32,
}
impl PeerView {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let peer_key = r.principal_id()?;
        let worker = r.worker_id()?;
        let delegate = crate::types::PublicKey32(r.fixed()?);
        let (metadata, generation, key_version, metadata_revision, valid_from, expiry) = crate::codec::read_values!(&mut r; metadata = r.metadata_digest()?;
            generation: U64,
            key_version: U64,
            metadata_revision: U64,
            valid_from: U64,
            expiry: U64,
        );
        let state = WorkerState::from_u8(r.u8()?)?;
        let enrolled = match r.u8()? {
            0 => false,
            1 => true,
            _ => return Err(NON_CANONICAL),
        };
        let capabilities = match r.u8()? {
            1 => Some(crate::workers::CapabilityFields::read(&mut r)?),
            4 => {
                if r.take(crate::workers::CapabilityFields::ENCODED_BYTES)?
                    != [0; crate::workers::CapabilityFields::ENCODED_BYTES]
                {
                    return Err(NON_CANONICAL);
                }
                None
            }
            _ => return Err(NON_CANONICAL),
        };
        let bond_status = match r.u8()? {
            2 => Availability::NotEnabled,
            _ => return Err(NON_CANONICAL),
        };
        let fault_count = r.u32()?;
        r.finish()?;
        if delegate.0 == [0; 32]
            || generation == 0
            || key_version == 0
            || metadata_revision == 0
            || valid_from >= expiry
        {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            peer_key,
            worker,
            delegate,
            metadata,
            generation,
            key_version,
            metadata_revision,
            valid_from,
            expiry,
            state,
            enrolled,
            capabilities,
            bond_status,
            fault_count,
        })
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ReceiptBatchView {
    pub batch: crate::laps::ReceiptBatch,
}
impl ReceiptBatchView {
    pub fn decode(bytes: &[u8]) -> CodecResult<Self> {
        Ok(Self {
            batch: crate::laps::ReceiptBatch::decode(bytes)?,
        })
    }
}

pub struct ViewRequest<'a> {
    pub envelope: codec::Envelope<'a>,
    unsigned: &'a [u8],
}
impl ViewRequest<'_> {
    pub fn request_digest(&self) -> CodecResult<RequestDigest> {
        RequestDigest::new(codec::domain_hash("PAXAI/request/v1", self.unsigned)?.bytes())
    }
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn validate_view_envelope(e: &codec::Envelope<'_>) -> CodecResult<()> {
    if !(0x0b10..=0x0b13).contains(&e.operation.selector()) {
        return Err(crate::errors::UNKNOWN_OPERATION);
    }
    e.operation.validate_payload_length(e.payload.len())?;
    if e.authentication != crate::types::Authentication::Native
        || e.epoch != 0
        || e.config != 0
        || e.roster != Presence::Absent
        || e.sequence != 0
        || e.expiry == 0
    {
        return Err(NON_CANONICAL);
    }
    let mut r = Reader::new(e.payload);
    r.digest32()?;
    if e.operation == dispatch::ReceiptBatchView {
        r.digest32()?;
    }
    r.finish()
}
pub fn encode_view_envelope(e: &codec::Envelope<'_>, out: &mut [u8]) -> CodecResult<usize> {
    validate_view_envelope(e)?;
    codec::encode_envelope_fields(e, out)
}
pub fn decode_view_envelope(input: &[u8]) -> CodecResult<ViewRequest<'_>> {
    let (envelope, unsigned) = codec::decode_envelope_fields(input, false)?;
    validate_view_envelope(&envelope)?;
    Ok(ViewRequest { envelope, unsigned })
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn answer_room_view(state_bytes: &[u8], payload: &[u8], out: &mut [u8]) -> CodecResult<usize> {
    use crate::rooms::*;
    let mut r = Reader::new(payload);
    let id = r.digest32()?;
    r.finish()?;
    let shared = state::decode_shared_state(state_bytes)?;
    let bytes = shared.room_market()?.1;
    let book = RoomBook::decode(bytes)?;
    let room = book.room(id)?.ok_or(crate::errors::F11_ROOM_UNKNOWN)?;
    let asks = book.table(1);
    let bids = book.table(2);
    let deals = book.table(3);
    let mut consumed = [false; BID_CAPACITY];
    for row in deals
        .as_chunks::<{ DEAL_RECORD_BYTES }>()
        .0
        .iter()
        .map(|chunk| chunk.as_slice())
    {
        for split in DealRecord::decode(row)?.layers.iter().flatten() {
            let index = bids
                .as_chunks::<{ BID_RECORD_BYTES }>()
                .0
                .iter()
                .map(|chunk| chunk.as_slice())
                .position(|b| b.guest_index(32..64) == split.bid_id.as_bytes())
                .ok_or(NON_CANONICAL)?;
            *consumed.guest_index_mut(index) = true;
        }
    }
    let out = out.get_mut(..ROOM_VIEW_BYTES).ok_or(CAPACITY)?;
    room.encode(out)?;
    let mut w = Writer::new(out.guest_index_mut(ROOM_RECORD_BYTES..));
    let mut count = 0;
    for row in asks
        .as_chunks::<{ ASK_RECORD_BYTES }>()
        .0
        .iter()
        .map(|chunk| chunk.as_slice())
    {
        let ask = AskRecord::decode(row)?;
        if room.state == RoomState::Open && ask.room_id == id && ask.state == AskState::Open {
            w.put(ask.ask_id.as_bytes())?;
            count += 1;
        }
    }
    for _ in count..ASK_CAPACITY {
        w.put(&[0; 32])?;
    }
    count = 0;
    for (index, row) in bids
        .as_chunks::<{ BID_RECORD_BYTES }>()
        .0
        .iter()
        .map(|chunk| chunk.as_slice())
        .enumerate()
    {
        let bid = BidRecord::decode(row)?;
        if room.state == RoomState::Open && bid.room_id == id && !*consumed.guest_index(index) {
            w.put(bid.bid_id.as_bytes())?;
            count += 1;
        }
    }
    for _ in count..BID_CAPACITY {
        w.put(&[0; 32])?;
    }
    Ok(ROOM_VIEW_BYTES)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn answer_deal_view(state_bytes: &[u8], payload: &[u8], out: &mut [u8]) -> CodecResult<usize> {
    let mut r = Reader::new(payload);
    let id = r.digest32()?;
    r.finish()?;
    let shared = state::decode_shared_state(state_bytes)?;
    let deal = crate::rooms::RoomBook::decode(shared.room_market()?.1)?
        .deal(id)?
        .ok_or(crate::errors::F11_ROOM_UNKNOWN)?;
    let settlement = crate::laps::settlement_from_state(state_bytes, id)?;
    let out = out.get_mut(..DEAL_VIEW_BYTES).ok_or(CAPACITY)?;
    let n = deal.encode(out)?;
    let mut w = Writer::new(out.guest_index_mut(n..));
    w.u64(settlement.map_or(0, |s| s.meter_reading()))?;
    w.u128(settlement.map_or(0, |s| s.settled_amount_owed))?;
    Ok(DEAL_VIEW_BYTES)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn answer_peer_view(state_bytes: &[u8], payload: &[u8], out: &mut [u8]) -> CodecResult<usize> {
    let mut r = Reader::new(payload);
    let key = r.principal_id()?;
    r.finish()?;
    let shared = state::decode_shared_state(state_bytes)?;
    let workers = worker_table(&shared)?;
    let worker = workers
        .iter()
        .find(|w| w.owner == key)
        .ok_or(crate::errors::F11_ROOM_UNKNOWN)?;
    let table = crate::admission::AdmissionTable::decode(
        crate::reputation_transition::split_joint(shared.section(Section::ReputationAdmission)?)?.1,
    )?;
    let enrolled = table
        .get(crate::admission::Participant::Worker(worker.worker))
        .is_some_and(|m| m.admitted() && !m.revoked())
        && matches!(
            worker.state,
            WorkerState::Enrolled | WorkerState::Available | WorkerState::Draining
        );
    let mut faults = 0u32;
    for record in crate::canary::CanaryRegion::decode(shared.control.feature_bytes)?.records() {
        let record = record?;
        if record.task.peer == worker.worker && record.peer_at_fault() {
            faults = faults.checked_add(1).ok_or(ARITHMETIC)?;
        }
    }
    let mut w = Writer::new(out.get_mut(..PEER_VIEW_BYTES).ok_or(CAPACITY)?);
    crate::codec::write_fields!(w;
        id32: worker.owner.as_bytes();
        id32: worker.worker.as_bytes();
        put: &worker.delegate.0;
        id32: worker.metadata.as_bytes();
    )?;
    for value in [
        worker.generation,
        worker.key_version,
        worker.metadata_revision,
        worker.valid_from,
        worker.expiry,
    ] {
        w.u64(value)?;
    }
    crate::codec::write_fields!(w;
        u8: worker.state as u8;
        u8: u8::from(enrolled);
        u8: Availability::ContentUnavailable as u8;
    )?;
    w.put(&[0; crate::workers::CapabilityFields::ENCODED_BYTES])?;
    w.u8(Availability::NotEnabled as u8)?;
    w.u32(faults)?;
    Ok(w.len())
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn answer_receipt_batch_view(
    state_bytes: &[u8],
    payload: &[u8],
    out: &mut [u8],
) -> CodecResult<usize> {
    let mut r = Reader::new(payload);
    let deal = r.digest32()?;
    let digest = r.digest32()?;
    r.finish()?;
    let record = crate::laps::settlement_from_state(state_bytes, deal)?
        .filter(|s| s.batch.batch_digest == digest)
        .ok_or(crate::errors::F11_ROOM_UNKNOWN)?;
    out.get_mut(..RECEIPT_BATCH_VIEW_BYTES)
        .ok_or(CAPACITY)?
        .guest_copy_from_slice(&record.batch.encode()?);
    Ok(RECEIPT_BATCH_VIEW_BYTES)
}

/// Off-chain view failures keep their own names, separate from on-chain application codes.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum QueryError {
    Application(ApplicationError),
    BindingMismatch,
    FinalityUnavailable,
    SnapshotConflict,
    IntegrityFailure,
    CursorExpired,
    CursorMismatch,
    ResponseTooLarge,
}
impl From<ApplicationError> for QueryError {
    fn from(error: ApplicationError) -> Self {
        Self::Application(error)
    }
}
pub type QueryResult<T> = Result<T, QueryError>;

/// The F02 worker table prefix of the identity section; the F03 region after it is not F10's.
fn worker_table(shared: &SharedState<'_>) -> CodecResult<WorkerTable> {
    WorkerTable::decode(split_identity_section(shared.section(Section::IdentityRoster)?)?.0)
}

fn policy_of<'s>(shared: &'s SharedState<'_>) -> CodecResult<PolicySection<'s>> {
    let bytes = shared.section(Section::PolicyLifecycle)?;
    if bytes.is_empty() {
        return Err(NOT_FOUND);
    }
    PolicySection::decode(bytes)
}

/// `READ_HEADER` over the exact captured state. Epoch and roster stay absent until a
/// frozen-roster producer records them in state; absence is never reported as epoch 0.
///
/// # Errors
/// Refuses a noncanonical state, an uncreated market (`NOT_FOUND`) and a chain or program
/// other than the market's own (`WRONG_DOMAIN`, `WRONG_PROGRAM`).
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn read_header(
    state_bytes: &[u8],
    chain: ChainDomain,
    program: ProgramId,
) -> CodecResult<ReadHeader> {
    let shared = state::decode_shared_state(state_bytes)?;
    let header = policy_of(&shared)?.header;
    if header.deployment_chain_domain != chain {
        return Err(WRONG_DOMAIN);
    }
    if header.program_id != program {
        return Err(WRONG_PROGRAM);
    }
    let opened = crate::epoch::captured_roster(&shared)?;
    Ok(ReadHeader {
        revision: shared.revision,
        digest: codec::state_digest(state_bytes)?,
        total_bytes: u32::try_from(state_bytes.len()).map_err(|_| ARITHMETIC)?,
        chain,
        program,
        market: header.market_id,
        epoch: opened
            .as_ref()
            .map_or(Presence::Absent, |r| Presence::Present(r.binding.epoch)),
        config: Version::new(header.active_config_version)?,
        roster: opened
            .as_ref()
            .map_or(Presence::Absent, |r| Presence::Present(r.binding.roster)),
    })
}

/// Storage reads project only the committed header and opening bindings. The
/// state digest binds every opaque section; no section is materialized here.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn stored_header(
    state_bytes: &[u8],
    chain: ChainDomain,
    program: ProgramId,
) -> CodecResult<ReadHeader> {
    let frame = codec::decode_state(state_bytes)?;
    let policy = frame.sections[Section::PolicyLifecycle.index()];
    if policy.is_empty() {
        return Err(NOT_FOUND);
    }
    let header_len = match policy.get(226) {
        Some(0) => crate::registry::MARKET_HEADER_ABSENT_BYTES,
        Some(1) => crate::registry::MARKET_HEADER_PRESENT_BYTES,
        _ => return Err(NON_CANONICAL),
    };
    let header =
        crate::registry::MarketHeader::decode(policy.get(..header_len).ok_or(NON_CANONICAL)?)?;
    if header.state_revision != frame.revision {
        return Err(NON_CANONICAL);
    }
    if header.deployment_chain_domain != chain {
        return Err(WRONG_DOMAIN);
    }
    if header.program_id != program {
        return Err(WRONG_PROGRAM);
    }
    let table = crate::reputation_transition::framed_admission_table(
        frame.sections[Section::ReputationAdmission.index()],
    )?;
    let epoch = if table.is_empty() {
        None
    } else {
        if table.len() > crate::admission::TABLE_MAX_BYTES {
            return Err(CAPACITY);
        }
        let mut r = Reader::new(table);
        if r.u16()? != SCHEMA_VERSION {
            return Err(BAD_VERSION);
        }
        let epoch = if r.boolean()? { Some(r.u64()?) } else { None };
        if r.u8()? > crate::admission::POLICY_V1.successful_enrollments_per_epoch
            || usize::from(r.u8()?) > crate::admission::MAX_MEMBERS
        {
            return Err(NON_CANONICAL);
        }
        epoch
    };
    if stored_snapshot_epoch(frame.sections[Section::IdentityRoster.index()])? != epoch {
        return Err(WRONG_ROSTER);
    }
    let settlement = frame.sections[Section::SettlementClaims.index()];
    let roster = match epoch {
        Some(epoch) => {
            Presence::Present(crate::rewards::committed_row(settlement, epoch)?.row.roster)
        }
        None => {
            if !settlement.is_empty() {
                let ledger = crate::rewards::decode_ledger(
                    settlement
                        .get(..crate::rewards::LEDGER_BYTES)
                        .ok_or(NON_CANONICAL)?,
                )?;
                if ledger.retained_epochs != 0 {
                    return Err(WRONG_ROSTER);
                }
            }
            Presence::Absent
        }
    };
    Ok(ReadHeader {
        revision: frame.revision,
        digest: codec::state_digest(state_bytes)?,
        total_bytes: u32::try_from(state_bytes.len()).map_err(|_| ARITHMETIC)?,
        chain,
        program,
        market: header.market_id,
        epoch: if crate::epoch::opening_pending(&PolicySection::decode(policy)?) {
            Presence::Absent
        } else {
            epoch.map_or(Presence::Absent, Presence::Present)
        },
        config: Version::new(header.active_config_version)?,
        roster: if crate::epoch::opening_pending(&PolicySection::decode(policy)?) {
            Presence::Absent
        } else {
            roster
        },
    })
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn stored_snapshot_epoch(identity: &[u8]) -> CodecResult<Option<u64>> {
    use crate::evaluators::authority::{FROZEN_BYTES, RECORD_BYTES};
    let (_, region) = split_identity_section(identity)?;
    if region.is_empty() {
        return Ok(None);
    }
    let mut r = Reader::new(region);
    let records = usize::from(r.u8()?);
    if records > crate::MAX_EVALUATORS {
        return Err(CAPACITY);
    }
    r.take(records * RECORD_BYTES)?;
    let format = r.u8()?;
    let epoch = match format {
        0 if records != 0 => None,
        2 => {
            let epoch = r.u64()?;
            let evaluators = usize::from(r.u8()?);
            if evaluators == 0 || evaluators > crate::MAX_EVALUATORS {
                return Err(WRONG_ROSTER);
            }
            r.take(evaluators * FROZEN_BYTES)?;
            let workers = usize::from(r.u8()?);
            if workers == 0 || workers > crate::MAX_WORKERS {
                return Err(WRONG_ROSTER);
            }
            r.take(workers * codec::WORKER_ROSTER_BYTES)?;
            Some(epoch)
        }
        1 => return Err(WRONG_ROSTER),
        _ => return Err(NON_CANONICAL),
    };
    r.finish()?;
    Ok(epoch)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn stored_chunk(
    state_bytes: &[u8],
    payload: &[u8],
    header: &ReadHeader,
    out: &mut [u8],
) -> CodecResult<usize> {
    dispatch::READ_STATE_CHUNK.validate_payload_length(payload.len())?;
    let request = codec::decode_chunk_request(payload)?;
    encode_state_chunk(state_bytes, &request, header.revision, header.digest, out)
}

/// Digest of the market's current policy in the captured state.
///
/// # Errors
/// Refuses a noncanonical state or an uncreated market (`NOT_FOUND`).
pub fn policy_digest(state_bytes: &[u8]) -> CodecResult<PolicyDigest> {
    let shared = state::decode_shared_state(state_bytes)?;
    policy_of(&shared)?.current.digest()
}

/// `READ_STATE_CHUNK`: exact 46-byte payload, pinned revision/digest after discovery, and a
/// chunk that never crosses the state end. Writes the encoded chunk response body to `out`.
///
/// # Errors
/// `NON_CANONICAL` for a malformed payload or an offset outside the state, `CONFLICT` for a
/// pin that no longer matches the captured state, `CAPACITY` for an oversized state or `out`.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn read_state_chunk(state_bytes: &[u8], payload: &[u8], out: &mut [u8]) -> CodecResult<usize> {
    dispatch::READ_STATE_CHUNK.validate_payload_length(payload.len())?;
    let request = codec::decode_chunk_request(payload)?;
    let shared = state::decode_shared_state(state_bytes)?;
    let digest = codec::state_digest(state_bytes)?;
    encode_state_chunk(state_bytes, &request, shared.revision, digest, out)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn encode_state_chunk(
    state_bytes: &[u8],
    request: &codec::ChunkRequest,
    revision: u64,
    digest: StateDigest,
    out: &mut [u8],
) -> CodecResult<usize> {
    if request.revision != 0
        && (request.revision != revision || request.digest != Presence::Present(digest))
    {
        return Err(CONFLICT);
    }
    let offset = usize::try_from(request.offset).map_err(|_| ARITHMETIC)?;
    let remaining = state_bytes
        .len()
        .checked_sub(offset)
        .filter(|n| *n > 0)
        .ok_or(NON_CANONICAL)?;
    let end = offset + usize::from(request.requested).min(remaining);
    let response = ChunkResponse {
        revision,
        digest,
        total_bytes: u32::try_from(state_bytes.len()).map_err(|_| ARITHMETIC)?,
        offset: request.offset,
        bytes: state_bytes.get(offset..end).ok_or(NON_CANONICAL)?,
    };
    codec::check_chunk_response(request, &response)?;
    codec::encode_chunk_response(&response, out)
}

/// Wraps a read body in the common application result (at most 16384 bytes).
///
/// # Errors
/// `CAPACITY` when the body or `out` exceeds the common result bounds.
pub fn frame_read_result(
    request: RequestDigest,
    revision: u64,
    body: &[u8],
    out: &mut [u8],
) -> CodecResult<usize> {
    let result =
        codec::ApplicationResult::success(codec::ResultStatus::Ok, request, revision, body)?;
    codec::encode_result(&result, out)
}

/// Outer `ProgramRead` facts returned with each chunk by the host read interface.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ReadProof {
    pub chain: ChainDomain,
    pub program: ProgramId,
    pub native_state_root: Digest32,
    pub observed_sequence: u64,
    pub execution_height: u64,
    pub block_hash: Digest32,
}

/// Facts of one complete capture: every chunk proved the same root and state identity.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct CaptureFacts {
    pub proof: ReadProof,
    pub revision: u64,
    pub digest: StateDigest,
    pub total_bytes: u32,
    pub chunks: usize,
}

/// Whole-capture reassembly into a caller buffer. Any head change, gap or short nonfinal
/// chunk fails the whole capture permanently; a retry restarts from one new root.
pub struct StateCapture<'a> {
    buffer: &'a mut [u8],
    facts: Option<CaptureFacts>,
    next_offset: usize,
    failed: bool,
}
impl<'a> StateCapture<'a> {
    pub fn new(buffer: &'a mut [u8]) -> Self {
        Self {
            buffer,
            facts: None,
            next_offset: 0,
            failed: false,
        }
    }
    /// Accepts the next chunk in offset order.
    ///
    /// # Errors
    /// `SnapshotConflict` when root, revision, digest or size differ from the first chunk,
    /// `IntegrityFailure` for a gap, reordering, short nonfinal chunk or an earlier failure,
    /// `CAPACITY` when the declared state exceeds the caller buffer.
    pub fn accept(&mut self, proof: &ReadProof, body: &[u8]) -> QueryResult<()> {
        if self.failed {
            return Err(QueryError::IntegrityFailure);
        }
        let outcome = self.append(proof, body);
        self.failed = outcome.is_err();
        outcome
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn append(&mut self, proof: &ReadProof, body: &[u8]) -> QueryResult<()> {
        let chunk = codec::decode_chunk_response(body)?;
        let total = usize::try_from(chunk.total_bytes).map_err(|_| ARITHMETIC)?;
        let facts = match self.facts {
            None if total > self.buffer.len() => return Err(CAPACITY.into()),
            None => CaptureFacts {
                proof: *proof,
                revision: chunk.revision,
                digest: chunk.digest,
                total_bytes: chunk.total_bytes,
                chunks: 0,
            },
            Some(f)
                if f.proof != *proof
                    || f.revision != chunk.revision
                    || f.digest != chunk.digest
                    || f.total_bytes != chunk.total_bytes =>
            {
                return Err(QueryError::SnapshotConflict)
            }
            Some(f) => f,
        };
        let offset = usize::try_from(chunk.offset).map_err(|_| ARITHMETIC)?;
        let end = offset + chunk.bytes.len();
        if offset != self.next_offset || (chunk.bytes.len() != MAX_CHUNK_BYTES && end != total) {
            return Err(QueryError::IntegrityFailure);
        }
        self.buffer
            .get_mut(offset..end)
            .ok_or(CAPACITY)?
            .guest_copy_from_slice(chunk.bytes);
        self.next_offset = end;
        self.facts = Some(CaptureFacts {
            chunks: facts.chunks + 1,
            ..facts
        });
        Ok(())
    }
    /// Completes the capture and verifies the reconstructed bytes against the state digest.
    ///
    /// # Errors
    /// `IntegrityFailure` for a failed, empty or incomplete capture or a digest mismatch.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn finish(self) -> QueryResult<(&'a [u8], CaptureFacts)> {
        let facts = self
            .facts
            .filter(|_| !self.failed)
            .ok_or(QueryError::IntegrityFailure)?;
        let total = usize::try_from(facts.total_bytes).map_err(|_| ARITHMETIC)?;
        if self.next_offset != total || facts.chunks != total.div_ceil(MAX_CHUNK_BYTES) {
            return Err(QueryError::IntegrityFailure);
        }
        let bytes = self.buffer.get(..total).ok_or(CAPACITY)?;
        if codec::state_digest(bytes)? != facts.digest {
            return Err(QueryError::IntegrityFailure);
        }
        Ok((bytes, facts))
    }
}

/// Finality evidence supplied by the native block-commit verifier for one root.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct FinalityEvidence {
    pub native_state_root: Digest32,
    pub block_height: u64,
    pub block_hash: Digest32,
    pub app_hash: Digest32,
    pub store_proof_digest: Digest32,
    pub rank: u8,
}

/// `SnapshotBindingV2` in canonical field order.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct SnapshotBinding {
    pub chain: ChainDomain,
    pub program: ProgramId,
    pub market: MarketId,
    pub observed_sequence: u64,
    pub execution_height: u64,
    pub batch_id: Digest32,
    pub native_state_root: Digest32,
    pub revision: u64,
    pub state_digest: StateDigest,
    pub epoch: Presence<u64>,
    pub config: Version,
    pub policy: PolicyDigest,
    pub roster: Presence<RosterDigest>,
    pub block_height: u64,
    pub block_hash: Digest32,
    pub app_hash: Digest32,
    pub store_proof_digest: Digest32,
    pub rank: u8,
    pub publication_time_ms: u64,
}

/// Binds a verified capture to its header and the finality evidence for the SAME root.
///
/// # Errors
/// `BindingMismatch` when the evidence names another root or the state differs from the
/// capture size/count, `NON_CANONICAL` for a zero block or app hash or a rank above
/// finalized, and header refusals of [`read_header`]. Evidence rank is preserved.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn bind_snapshot(
    state_bytes: &[u8],
    facts: &CaptureFacts,
    finality: &FinalityEvidence,
    publication_time_ms: u64,
) -> QueryResult<SnapshotBinding> {
    let header = read_header(state_bytes, facts.proof.chain, facts.proof.program)?;
    if header.revision != facts.revision
        || header.digest != facts.digest
        || header.total_bytes != facts.total_bytes
        || facts.chunks != state_bytes.len().div_ceil(MAX_CHUNK_BYTES)
        || finality.native_state_root != facts.proof.native_state_root
    {
        return Err(QueryError::BindingMismatch);
    }
    if finality.rank > FINALIZED_RANK
        || finality.block_hash.bytes() == [0; 32]
        || finality.app_hash.bytes() == [0; 32]
    {
        return Err(NON_CANONICAL.into());
    }
    Ok(SnapshotBinding {
        chain: header.chain,
        program: header.program,
        market: header.market,
        observed_sequence: facts.proof.observed_sequence,
        execution_height: facts.proof.execution_height,
        batch_id: facts.proof.block_hash,
        native_state_root: facts.proof.native_state_root,
        revision: header.revision,
        state_digest: header.digest,
        epoch: header.epoch,
        config: header.config,
        policy: policy_digest(state_bytes)?,
        roster: header.roster,
        block_height: finality.block_height,
        block_hash: finality.block_hash,
        app_hash: finality.app_hash,
        store_proof_digest: finality.store_proof_digest,
        rank: finality.rank,
        publication_time_ms,
    })
}

impl SnapshotBinding {
    /// Decodes the canonical transport used by clients and saved indexer bindings.
    ///
    /// # Errors
    /// `BAD_VERSION` for another schema; codec refusals for malformed, oversized,
    /// trailing or reserved bytes, revision zero, or a rank above finalized.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut input = Reader::new(bytes);
        if input.u16()? != 1 {
            return Err(BAD_VERSION);
        }
        if bytes.len() > BINDING_MAX_BYTES {
            return Err(NON_CANONICAL);
        }
        let binding = SnapshotBinding {
            chain: input.chain_domain()?,
            program: input.program_id()?,
            market: input.market_id()?,
            observed_sequence: input.u64()?,
            execution_height: input.u64()?,
            batch_id: input.digest32()?,
            native_state_root: input.digest32()?,
            revision: input.u64()?,
            state_digest: input.state_digest()?,
            epoch: input.presence(Reader::u64)?,
            config: input.version()?,
            policy: input.policy_digest()?,
            roster: input.presence(|reader| reader.roster_digest())?,
            block_height: input.u64()?,
            block_hash: input.digest32()?,
            app_hash: input.digest32()?,
            store_proof_digest: input.digest32()?,
            rank: input.u8()?,
            publication_time_ms: input.u64()?,
        };
        input.reserved(8)?;
        input.finish()?;
        if binding.revision == 0 || binding.rank > FINALIZED_RANK {
            return Err(NON_CANONICAL);
        }
        Ok(binding)
    }

    /// Verifies a saved binding against a fresh capture of its exact producer state.
    /// No previous decode or snapshot identifier substitutes for the capture.
    ///
    /// # Errors
    /// Binding refusals or `BindingMismatch` if any saved content field differs.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn verify_capture(&self, state: &[u8], facts: &CaptureFacts) -> QueryResult<()> {
        let evidence = FinalityEvidence {
            native_state_root: self.native_state_root,
            block_height: self.block_height,
            block_hash: self.block_hash,
            app_hash: self.app_hash,
            store_proof_digest: self.store_proof_digest,
            rank: self.rank,
        };
        if bind_snapshot(state, facts, &evidence, self.publication_time_ms)? != *self {
            return Err(QueryError::BindingMismatch);
        }
        Ok(())
    }

    /// The exact committed head consumed by retention and webhook publication.
    ///
    /// # Errors
    /// `FinalityUnavailable` without rank 4; `BindingMismatch` when the commit
    /// names another height, block or app root, or the head has no height.
    pub fn finalized_head(&self) -> QueryResult<(u64, Digest32)> {
        self.require_finalized()?;
        if self.execution_height == 0
            || self.block_height != self.execution_height
            || self.block_hash != self.batch_id
            || self.app_hash != self.native_state_root
        {
            return Err(QueryError::BindingMismatch);
        }
        Ok((self.block_height, self.block_hash))
    }

    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write_prefix(&self, w: &mut Writer<'_>) -> CodecResult<()> {
        crate::codec::write_fields!(w;
            u16: SCHEMA_VERSION;
            id32: self.chain.as_bytes();
            id32: self.program.as_bytes();
            id32: self.market.as_bytes();
            u64: self.observed_sequence;
            u64: self.execution_height;
            id32: self.batch_id.as_bytes();
            id32: self.native_state_root.as_bytes();
            u64: self.revision;
            id32: self.state_digest.as_bytes();
        )?;
        w.presence(&self.epoch, |w, e| w.u64(*e))?;
        w.u64(self.config.get())?;
        w.put(self.policy.as_bytes())?;
        w.presence(&self.roster, |w, r| w.put(r.as_bytes()))
    }
    /// Canonical binding bytes; returns the encoded length.
    ///
    /// # Errors
    /// `NON_CANONICAL` for revision 0 or a rank above 4, `CAPACITY` for a short `out`.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        if self.revision == 0 || self.rank > FINALIZED_RANK {
            return Err(NON_CANONICAL);
        }
        let mut w = Writer::new(out);
        self.write_prefix(&mut w)?;
        crate::codec::write_fields!(w;
            u64: self.block_height;
            id32: self.block_hash.as_bytes();
            id32: self.app_hash.as_bytes();
            id32: self.store_proof_digest.as_bytes();
            u8: self.rank;
            u64: self.publication_time_ms;
        )?;
        w.put(&[0; 8])?;
        Ok(w.len())
    }
    /// Content identity; excludes block-commit evidence, rank and publication time.
    ///
    /// # Errors
    /// Propagates prefix encoding failures.
    pub fn snapshot_id(&self) -> CodecResult<Digest32> {
        let mut prefix = [0u8; BINDING_PREFIX_MAX_BYTES];
        let mut w = Writer::new(&mut prefix);
        self.write_prefix(&mut w)?;
        let n = w.len();
        codec::domain_hash("PAXAI/view/v1", prefix.get(..n).ok_or(CAPACITY)?)
    }
    /// Checks the snapshot's compatibility discriminator and nonzero commit hashes.
    /// The caller must authenticate the block commit and state proof separately;
    /// this structural check does not verify either proof.
    ///
    /// # Errors
    /// `FinalityUnavailable` unless rank is 4 and block and app hashes are nonzero.
    pub fn require_finalized(&self) -> QueryResult<()> {
        if self.rank == FINALIZED_RANK
            && self.block_hash.bytes() != [0; 32]
            && self.app_hash.bytes() != [0; 32]
        {
            Ok(())
        } else {
            Err(QueryError::FinalityUnavailable)
        }
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum Availability {
    Available = 1,
    NotEnabled = 2,
    NotYetProduced = 3,
    ContentUnavailable = 4,
    UnsupportedVersion = 5,
}

/// Availability of content actually decoded from the exact capture, in F01..F11 order.
/// Unknown codec versions and malformed content remain explicit to consumers.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn feature_availability(state_bytes: &[u8]) -> CodecResult<[Availability; 11]> {
    fn decoded<T>(bytes: &[u8], decode: impl FnOnce() -> CodecResult<T>) -> Availability {
        if bytes.is_empty() {
            return Availability::NotYetProduced;
        }
        match decode() {
            Ok(_) => Availability::Available,
            Err(BAD_VERSION) => Availability::UnsupportedVersion,
            Err(_) => Availability::ContentUnavailable,
        }
    }
    let shared = state::decode_shared_state(state_bytes)?;
    let mut out = [Availability::NotYetProduced; 11];
    let policy = shared.section(Section::PolicyLifecycle)?;
    *out.guest_index_mut(0) = decoded(policy, || policy_of(&shared));
    *out.guest_index_mut(10) = match *out.guest_index(0) {
        Availability::Available => match policy_of(&shared).and_then(|policy| {
            let bytes = shared.room_market()?.1;
            let book = crate::rooms::RoomBook::decode(bytes)?;
            if *book.counts().guest_index(0) == 0 {
                return Ok(Availability::NotEnabled);
            }
            placement::PlacementBook::decode(bytes, policy.current.commitments.model_artifact)?;
            Ok(Availability::Available)
        }) {
            Ok(availability) => availability,
            Err(_) => Availability::ContentUnavailable,
        },
        availability => availability,
    };
    *out.guest_index_mut(1) = decoded(shared.section(Section::IdentityRoster)?, || {
        worker_table(&shared)
    });
    let identity = split_identity_section(shared.section(Section::IdentityRoster)?);
    *out.guest_index_mut(2) = match identity {
        Ok((_, bytes)) => decoded(bytes, || {
            crate::evaluators::authority::EvaluatorRegion::decode(bytes)
        }),
        Err(BAD_VERSION) => Availability::UnsupportedVersion,
        Err(_) => Availability::ContentUnavailable,
    };
    let reports = crate::evaluators::admission::ReportRegion::decode(
        shared.section(Section::CurrentReports)?,
    );
    *out.guest_index_mut(3) = match reports {
        Ok(reports) => decoded(reports.rest(), || {
            crate::commit_reveal::CommitRegion::decode(reports.rest())
        }),
        Err(BAD_VERSION) => Availability::UnsupportedVersion,
        Err(_) => Availability::ContentUnavailable,
    };
    let settlement = shared.section(Section::SettlementClaims)?;
    if !settlement.is_empty() {
        *out.guest_index_mut(5) = decoded(settlement, || {
            settlement
                .get(..crate::rewards::REWARD_STATE_BYTES)
                .ok_or(NON_CANONICAL)
                .and_then(crate::rewards::decode_reward_state)
        });
        if settlement.len() > crate::rewards::REWARD_STATE_BYTES {
            *out.guest_index_mut(4) = decoded(
                settlement.guest_index(crate::rewards::REWARD_STATE_BYTES..),
                || crate::aggregation::progress(state_bytes),
            );
        }
    }
    match crate::reputation_transition::split_joint(shared.section(Section::ReputationAdmission)?) {
        Ok((region, table)) => {
            *out.guest_index_mut(6) = if region.is_some() {
                Availability::Available
            } else {
                Availability::NotYetProduced
            };
            *out.guest_index_mut(7) =
                decoded(table, || crate::admission::AdmissionTable::decode(table));
        }
        Err(BAD_VERSION) => {
            *out.guest_index_mut(6) = Availability::UnsupportedVersion;
            *out.guest_index_mut(7) = Availability::ContentUnavailable;
        }
        Err(_) => {
            *out.guest_index_mut(6) = Availability::ContentUnavailable;
            *out.guest_index_mut(7) = Availability::ContentUnavailable;
        }
    }
    *out.guest_index_mut(8) = decoded(shared.control.feature_bytes, || {
        crate::evidence::SealRegion::decode(shared.control.feature_bytes)
    });
    if !policy.is_empty() {
        *out.guest_index_mut(9) = Availability::Available;
    }
    Ok(out)
}

#[derive(Clone, Copy, Eq, PartialEq, Ord, PartialOrd)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum ParticipantKind {
    Worker = 1,
    Evaluator = 2,
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum ScoreStatus {
    Present = 1,
    NoAdmissibleScore = 2,
    InsufficientCoverage = 3,
    NotProduced = 4,
    Unavailable = 5,
    Unsupported = 6,
}

/// Present carries an actual number (zero included); every other status carries none.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ScoreField {
    epoch: Presence<u64>,
    status: ScoreStatus,
    ppm: Presence<u32>,
}
impl ScoreField {
    /// # Errors
    /// `NON_CANONICAL` above 1000000 ppm.
    pub fn present(epoch: u64, ppm: u32) -> CodecResult<Self> {
        if ppm > 1_000_000 {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            epoch: Presence::Present(epoch),
            status: ScoreStatus::Present,
            ppm: Presence::Present(ppm),
        })
    }
    /// # Errors
    /// `NON_CANONICAL` for the present status, which must carry a number.
    pub fn absent(epoch: Presence<u64>, status: ScoreStatus) -> CodecResult<Self> {
        if status == ScoreStatus::Present {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            epoch,
            status,
            ppm: Presence::Absent,
        })
    }
    #[must_use]
    pub const fn status(&self) -> ScoreStatus {
        self.status
    }
    #[must_use]
    pub const fn ppm(&self) -> Presence<u32> {
        self.ppm
    }
    #[must_use]
    pub const fn epoch(&self) -> Presence<u64> {
        self.epoch
    }
}

/// F06-backed entitlement. Only an F06 producer may supply available values.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct RewardField {
    status: Availability,
    asset: Presence<AssetId>,
    earned: Presence<u128>,
    claimed: Presence<u128>,
}
impl RewardField {
    /// # Errors
    /// `NON_CANONICAL` when claimed exceeds earned.
    pub fn entitlement(asset: AssetId, earned: u128, claimed: u128) -> CodecResult<Self> {
        if claimed > earned {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            status: Availability::Available,
            asset: Presence::Present(asset),
            earned: Presence::Present(earned),
            claimed: Presence::Present(claimed),
        })
    }
    /// # Errors
    /// `NON_CANONICAL` for the available status, which must carry values.
    pub fn unavailable(status: Availability) -> CodecResult<Self> {
        if status == Availability::Available {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            status,
            asset: Presence::Absent,
            earned: Presence::Absent,
            claimed: Presence::Absent,
        })
    }
    #[must_use]
    pub const fn status(&self) -> Availability {
        self.status
    }
    #[must_use]
    pub const fn asset(&self) -> Presence<AssetId> {
        self.asset
    }
    #[must_use]
    pub const fn earned(&self) -> Presence<u128> {
        self.earned
    }
    #[must_use]
    pub const fn claimed(&self) -> Presence<u128> {
        self.claimed
    }
}

pub const ELIGIBLE_SERVING: u8 = 1;
pub const ELIGIBLE_NEW_WORK: u8 = 2;
/// Evaluator rows come from the frozen roster; no current evaluator record exists in state.
pub const IDENTITY_STATE_UNAVAILABLE: u8 = 0;

/// One participant row. For evaluators, `generation` is the frozen grant version.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ParticipantRow {
    pub kind: ParticipantKind,
    pub id: [u8; 32],
    pub owner: PrincipalId,
    pub generation: u64,
    pub identity_state: u8,
    pub frozen_member: bool,
    pub frozen_generation: Presence<u64>,
    pub eligibility: u8,
    pub metadata: Presence<MetadataDigest>,
    pub metadata_revision: u64,
    pub score: ScoreField,
    pub reward: RewardField,
    pub history_status: Availability,
    pub history: Presence<Digest32>,
}
impl ParticipantRow {
    /// # Errors
    /// `CAPACITY` when the writer is full.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, w: &mut Writer<'_>) -> CodecResult<()> {
        crate::codec::write_fields!(w;
            u8: self.kind as u8;
            put: &self.id;
            id32: self.owner.as_bytes();
            u64: self.generation;
            u8: self.identity_state;
            u8: u8::from(self.frozen_member);
        )?;
        w.presence(&self.frozen_generation, |w, g| w.u64(*g))?;
        w.u8(self.eligibility)?;
        w.presence(&self.metadata, |w, m| w.put(m.as_bytes()))?;
        w.u64(self.metadata_revision)?;
        w.presence(&self.score.epoch, |w, e| w.u64(*e))?;
        w.u8(self.score.status as u8)?;
        w.presence(&self.score.ppm, |w, s| w.u32(*s))?;
        w.u8(self.reward.status as u8)?;
        w.presence(&self.reward.asset, |w, a| w.put(a.as_bytes()))?;
        w.presence(&self.reward.earned, |w, v| w.u128(*v))?;
        w.presence(&self.reward.claimed, |w, v| w.u128(*v))?;
        w.u8(self.history_status as u8)?;
        w.presence(&self.history, |w, h| w.put(h.as_bytes()))
    }
    const fn active(&self) -> bool {
        self.eligibility & ELIGIBLE_SERVING != 0
    }
}

/// Complete projection from the captured F02/F03/F05/F06/F07/F08 producers.
/// An optional transported roster must bind exactly to the persisted opening.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn participant_rows(
    state_bytes: &[u8],
    supplied: Option<&RosterView<'_>>,
    out: &mut [ParticipantRow],
) -> CodecResult<usize> {
    let shared = state::decode_shared_state(state_bytes)?;
    let policy = policy_of(&shared)?;
    let opened = crate::epoch::captured_roster(&shared)?;
    if let Some(view) = supplied {
        if view.market != policy.header.market_id {
            return Err(WRONG_MARKET);
        }
        let captured = opened.as_ref().ok_or(WRONG_ROSTER)?;
        if view.epoch != captured.binding.epoch
            || view.config != captured.binding.config
            || view.digest()? != captured.binding.roster
        {
            return Err(WRONG_ROSTER);
        }
    }
    let workers = worker_table(&shared)?;
    let evaluators =
        crate::evaluators::authority::evaluator_region(shared.section(Section::IdentityRoster)?)?;
    let (history, _) =
        crate::reputation_transition::split_joint(shared.section(Section::ReputationAdmission)?)?;
    let settlement = shared.section(Section::SettlementClaims)?;
    let rewards = if settlement.is_empty() {
        None
    } else {
        Some(crate::rewards::decode_reward_state(
            settlement
                .get(..crate::rewards::REWARD_STATE_BYTES)
                .ok_or(NON_CANONICAL)?,
        )?)
    };
    let aggregate = if opened.is_some() {
        crate::aggregation::captured_aggregation(state_bytes)?
    } else {
        None
    };
    let total = workers.len() + evaluators.len();
    let slots = out.get_mut(..total).ok_or(CAPACITY)?;
    let (worker_slots, evaluator_slots) = slots.guest_split_at_mut(workers.len());
    for (slot, record) in worker_slots.iter_mut().zip(workers.iter()) {
        let frozen = opened
            .as_ref()
            .and_then(|r| r.workers().iter().find(|w| w.worker == record.worker));
        let epoch = opened
            .as_ref()
            .map_or(Presence::Absent, |r| Presence::Present(r.binding.epoch));
        let mut score = ScoreField::absent(epoch, ScoreStatus::NotProduced)?;
        if let (Some(roster), Some(aggregate)) = (&opened, &aggregate) {
            if aggregate.phase() == crate::aggregation_codec::AggregationPhase::Terminal {
                for i in 0..aggregate.output_count() {
                    let output = aggregate.output(i)?;
                    if output.worker() == record.worker {
                        score = match output.quality() {
                            Presence::Present(value) => {
                                ScoreField::present(roster.binding.epoch, value.get())?
                            }
                            Presence::Absent => {
                                ScoreField::absent(epoch, ScoreStatus::InsufficientCoverage)?
                            }
                        };
                    }
                }
            }
        }
        let mut reward = RewardField::unavailable(Availability::NotYetProduced)?;
        if let (Some(roster), Some(rewards)) = (&opened, &rewards) {
            let row = rewards.row(roster.binding.epoch)?;
            if matches!(
                row.status,
                crate::rewards::EpochStatus::Terminal | crate::rewards::EpochStatus::Expired
            ) {
                if row.outcome == crate::rewards::RewardOutcome::NoEligibleScore && frozen.is_some()
                {
                    reward = RewardField::entitlement(rewards.ledger()?.asset, 0, 0)?;
                }
                let dictionary = rewards.dictionary();
                for entry in row.entries() {
                    if dictionary.slot(entry.slot)?.worker == record.worker {
                        let claimed = if entry.disposition == crate::rewards::Disposition::Claimed {
                            entry.entitlement
                        } else {
                            0
                        };
                        reward = RewardField::entitlement(
                            rewards.ledger()?.asset,
                            entry.entitlement,
                            claimed,
                        )?;
                    }
                }
            }
        }
        let standing = history
            .as_ref()
            .and_then(|region| region.state.records().find(|r| r.worker == record.worker));
        *slot = ParticipantRow {
            kind: ParticipantKind::Worker,
            id: record.worker.bytes(),
            owner: record.owner,
            generation: record.generation,
            identity_state: record.state as u8,
            frozen_member: frozen.is_some(),
            frozen_generation: frozen
                .map_or(Presence::Absent, |w| Presence::Present(w.generation.get())),
            eligibility: match record.state {
                WorkerState::Available => ELIGIBLE_SERVING | ELIGIBLE_NEW_WORK,
                WorkerState::Enrolled | WorkerState::Draining => ELIGIBLE_SERVING,
                _ => 0,
            },
            metadata: Presence::Present(record.metadata),
            metadata_revision: record.metadata_revision,
            score,
            reward,
            history_status: if standing.is_some() {
                Availability::Available
            } else {
                Availability::NotYetProduced
            },
            history: standing.map_or(Presence::Absent, |r| Presence::Present(r.segment)),
        };
    }
    for (slot, record) in evaluator_slots.iter_mut().zip(evaluators.records()) {
        let grant = &record.grant;
        let frozen = opened.as_ref().and_then(|r| {
            r.evaluators()
                .iter()
                .find(|e| e.evaluator == grant.evaluator)
        });
        *slot = ParticipantRow {
            kind: ParticipantKind::Evaluator,
            id: grant.evaluator.bytes(),
            owner: grant.principal,
            generation: grant.grant_version.get(),
            identity_state: grant.status as u8,
            frozen_member: frozen.is_some(),
            frozen_generation: frozen
                .map_or(Presence::Absent, |e| Presence::Present(e.grant.get())),
            eligibility: 0,
            metadata: Presence::Absent,
            metadata_revision: 0,
            score: ScoreField::absent(Presence::Absent, ScoreStatus::NotProduced)?,
            reward: RewardField::unavailable(Availability::NotEnabled)?,
            history_status: Availability::NotEnabled,
            history: Presence::Absent,
        };
    }
    Ok(total)
}

/// The indexer's complete projection, pinned to the transported captured binding.
/// Used both when observing content and when serving a reopened persisted capture.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn project_capture(
    state_bytes: &[u8],
    binding: &SnapshotBinding,
    out: &mut [ParticipantRow],
) -> QueryResult<([Availability; 11], usize)> {
    let header = read_header(state_bytes, binding.chain, binding.program)?;
    if header.market != binding.market
        || header.revision != binding.revision
        || header.digest != binding.state_digest
        || policy_digest(state_bytes)? != binding.policy
        || header.config != binding.config
        || header.epoch != binding.epoch
        || header.roster != binding.roster
    {
        return Err(QueryError::BindingMismatch);
    }
    Ok((
        feature_availability(state_bytes)?,
        participant_rows(state_bytes, None, out)?,
    ))
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum KindFilter {
    All = 0,
    Worker = 1,
    Evaluator = 2,
}
impl KindFilter {
    /// # Errors
    /// `NON_CANONICAL` for an unknown filter.
    pub fn from_u8(value: u8) -> CodecResult<Self> {
        Ok(match value {
            0 => Self::All,
            1 => Self::Worker,
            2 => Self::Evaluator,
            _ => return Err(NON_CANONICAL),
        })
    }
    fn admits(self, row: &ParticipantRow, active_only: bool) -> bool {
        let kind = match self {
            Self::All => true,
            Self::Worker => row.kind == ParticipantKind::Worker,
            Self::Evaluator => row.kind == ParticipantKind::Evaluator,
        };
        kind && (!active_only || row.active())
    }
}

/// Exactly 64 lower-case hexadecimal characters.
///
/// # Errors
/// `NON_CANONICAL` for any other length or character.
pub fn parse_hex32(text: &str) -> CodecResult<[u8; 32]> {
    let mut out = [0u8; 32];
    hex_decode(text.as_bytes(), &mut out)?;
    Ok(out)
}
fn nibble(c: u8) -> CodecResult<u8> {
    match c {
        b'0'..=b'9' => Ok(c - b'0'),
        b'a'..=b'f' => Ok(c - b'a' + 10),
        _ => Err(NON_CANONICAL),
    }
}
fn hex_decode(text: &[u8], out: &mut [u8]) -> CodecResult<()> {
    if text.len() != out.len() * 2 {
        return Err(NON_CANONICAL);
    }
    for (o, pair) in out.iter_mut().zip(
        text.as_chunks::<{ 2 }>()
            .0
            .iter()
            .map(|chunk| chunk.as_slice()),
    ) {
        *o = (nibble(*pair.guest_index(0))? << 4) | nibble(*pair.guest_index(1))?;
    }
    Ok(())
}

/// Canonical decimal limit, default 16, range 1..=32.
///
/// # Errors
/// `NON_CANONICAL` for a noncanonical decimal or a value outside 1..=32.
pub fn parse_limit(text: Option<&str>) -> CodecResult<u8> {
    let Some(text) = text else {
        return Ok(DEFAULT_LIMIT);
    };
    let limit = codec::decimal_u64(text)?;
    if limit == 0 || limit > PAGE_MAX_ROWS as u64 {
        return Err(NON_CANONICAL);
    }
    u8::try_from(limit).map_err(|_| ARITHMETIC)
}

/// Server key for cursor authentication; never placed in a snapshot.
#[derive(Clone, Copy)]
pub struct CursorKey<'k> {
    pub secret: &'k [u8; 32],
    pub generation: u32,
}
/// Visibility scope, snapshot and exact normalized filters a cursor is bound to.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct CursorScope {
    pub visibility: Digest32,
    pub market: MarketId,
    pub snapshot: Digest32,
    pub filter: KindFilter,
    pub active_only: bool,
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn hmac(secret: &[u8; 32], payload: &[u8]) -> [u8; 32] {
    let mut ipad = [0x36u8; 64];
    let mut opad = [0x5cu8; 64];
    for ((i, o), k) in ipad.iter_mut().zip(opad.iter_mut()).zip(secret) {
        *i ^= k;
        *o ^= k;
    }
    let mut inner = Sha256::new();
    inner.update(&ipad);
    inner.update(payload);
    let inner = inner.finalize();
    let mut outer = Sha256::new();
    outer.update(&opad);
    outer.update(&inner);
    outer.finalize().into()
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn cursor_payload(
    key: &CursorKey<'_>,
    scope: &CursorScope,
    next_ordinal: u16,
    expiry_ms: u64,
) -> CodecResult<[u8; CURSOR_PAYLOAD_BYTES]> {
    let filter = codec::domain_hash(
        "PAXAI/view-filter/v1",
        &[scope.filter as u8, u8::from(scope.active_only)],
    )?;
    let mut p = [0u8; CURSOR_PAYLOAD_BYTES];
    let mut w = Writer::new(&mut p);
    crate::codec::write_fields!(w;
        u16: SCHEMA_VERSION;
        id32: scope.visibility.as_bytes();
        id32: scope.market.as_bytes();
        id32: scope.snapshot.as_bytes();
        id32: filter.as_bytes();
        u16: next_ordinal;
        u64: expiry_ms;
        u32: key.generation;
    )?;
    Ok(p)
}

/// Issues an opaque lower-case hex token: payload followed by HMAC-SHA256(secret, payload).
///
/// # Errors
/// `ARITHMETIC` when the expiry overflows.
pub fn issue_cursor<'o>(
    key: &CursorKey<'_>,
    scope: &CursorScope,
    next_ordinal: u16,
    now_ms: u64,
    out: &'o mut [u8; CURSOR_TOKEN_BYTES],
) -> CodecResult<&'o str> {
    let expiry = now_ms.checked_add(CURSOR_LIFETIME_MS).ok_or(ARITHMETIC)?;
    let payload = cursor_payload(key, scope, next_ordinal, expiry)?;
    let tag = hmac(key.secret, &payload);
    for (pair, b) in out
        .as_chunks_mut::<{ 2 }>()
        .0
        .iter_mut()
        .map(|chunk| chunk.as_mut_slice())
        .zip(payload.iter().chain(&tag))
    {
        *pair.guest_index_mut(0) = *HEX.guest_index(usize::from(b >> 4));
        *pair.guest_index_mut(1) = *HEX.guest_index(usize::from(b & 15));
    }
    core::str::from_utf8(out).map_err(|_| NON_CANONICAL)
}

/// Authenticates a cursor before any row work and returns its next ordinal.
///
/// # Errors
/// `NON_CANONICAL` for a token that is not exactly one canonical cursor encoding,
/// `CursorMismatch` for a bad tag, another scope/key generation or an ordinal outside the
/// rows, `CursorExpired` at or after its expiry.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn open_cursor(
    key: &CursorKey<'_>,
    scope: &CursorScope,
    token: &str,
    now_ms: u64,
    total_rows: usize,
) -> QueryResult<u16> {
    let mut raw = [0u8; CURSOR_PAYLOAD_BYTES + CURSOR_TAG_BYTES];
    hex_decode(token.as_bytes(), &mut raw)?;
    let (payload, tag) = raw.guest_split_at(CURSOR_PAYLOAD_BYTES);
    let expected = hmac(key.secret, payload);
    if expected.iter().zip(tag).fold(0u8, |a, (x, y)| a | (x ^ y)) != 0 {
        return Err(QueryError::CursorMismatch);
    }
    let mut r = Reader::new(payload);
    r.take(CURSOR_ORDINAL_OFFSET)?;
    let next = r.u16()?;
    let expiry = r.u64()?;
    r.u32()?;
    r.finish()?;
    if *payload.guest_index(..) != *(cursor_payload(key, scope, next, expiry)?).guest_index(..) {
        return Err(QueryError::CursorMismatch);
    }
    if expiry <= now_ms {
        return Err(QueryError::CursorExpired);
    }
    if next == 0 || usize::from(next) >= total_rows {
        return Err(QueryError::CursorMismatch);
    }
    Ok(next)
}

/// Selects one page from rows in kind-then-ID order. Returns rows written and the next
/// ordinal when more rows remain.
///
/// # Errors
/// `NON_CANONICAL` for a limit outside 1..=32 or rows not strictly in kind-then-ID order.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn select_page(
    rows: &[ParticipantRow],
    filter: KindFilter,
    active_only: bool,
    start: u16,
    limit: u8,
    out: &mut [ParticipantRow; PAGE_MAX_ROWS],
) -> CodecResult<(usize, Option<u16>)> {
    if limit == 0 || usize::from(limit) > PAGE_MAX_ROWS {
        return Err(NON_CANONICAL);
    }
    if rows.windows(2).any(|p| {
        (p.guest_index(0).kind, p.guest_index(0).id) >= (p.guest_index(1).kind, p.guest_index(1).id)
    }) {
        return Err(NON_CANONICAL);
    }
    let mut written = 0usize;
    let admitted = rows
        .iter()
        .filter(|r| filter.admits(r, active_only))
        .enumerate()
        .skip(usize::from(start));
    for (ordinal, row) in admitted {
        let Some(slot) = out
            .get_mut(written)
            .filter(|_| written < usize::from(limit))
        else {
            return Ok((
                written,
                Some(u16::try_from(ordinal).map_err(|_| ARITHMETIC)?),
            ));
        };
        *slot = *row;
        written += 1;
    }
    Ok((written, None))
}

/// Off-chain page bound.
///
/// # Errors
/// `ResponseTooLarge` above 65536 bytes.
pub fn bound_response(length: usize) -> QueryResult<usize> {
    if length > PAGE_MAX_BYTES {
        Err(QueryError::ResponseTooLarge)
    } else {
        Ok(length)
    }
}

/// Page frame: schema, snapshot id, row count, rows, optional cursor token.
///
/// # Errors
/// `ResponseTooLarge` above 32 rows, a cursor above 1024 bytes or a frame above 65536 bytes;
/// `CAPACITY` for a short `out`.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_page(
    snapshot: Digest32,
    rows: &[ParticipantRow],
    cursor: Option<&str>,
    out: &mut [u8],
) -> QueryResult<usize> {
    if rows.len() > PAGE_MAX_ROWS || cursor.is_some_and(|c| c.len() > CURSOR_MAX_BYTES) {
        return Err(QueryError::ResponseTooLarge);
    }
    let mut w = Writer::new(out);
    w.u16(SCHEMA_VERSION)?;
    w.put(snapshot.as_bytes())?;
    w.u8(u8::try_from(rows.len()).map_err(|_| ARITHMETIC)?)?;
    for row in rows {
        row.encode(&mut w)?;
    }
    match cursor {
        None => w.boolean(false)?,
        Some(c) => {
            w.boolean(true)?;
            w.u16(u16::try_from(c.len()).map_err(|_| ARITHMETIC)?)?;
            w.put(c.as_bytes())?;
        }
    }
    bound_response(w.len())
}
