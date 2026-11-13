use super::{Draft, RewardEffect, RewardLedger, RewardState, REWARD_STATE_BYTES};
use crate::types::GuestIndex as _;
use crate::{
    codec::{self, Reader, ValidatedEnvelope, Writer},
    dispatch,
    errors::{
        CodecResult, ARITHMETIC, CAPACITY, CONFLICT, F11_BOND_INSUFFICIENT, F11_BOND_LOCKED,
        F11_ROOM_CLOSED, F11_ROOM_UNKNOWN, NON_CANONICAL, NOT_FOUND, UNAUTHORIZED,
        UNKNOWN_OPERATION, WRONG_CONFIG, WRONG_EPOCH, WRONG_PHASE, WRONG_ROSTER,
    },
    registry_ops::{CallContext, ACTIVE, REGISTERED},
    rooms::{self, DealRecord, DealState, RoomBook, RoomState},
    state::{self, ActorSlot, ReplayDecision, ReplayRequest, Section, SharedState},
    types::{AccountId, Digest32, Presence, PrincipalId, WorkerId},
    workers::{WorkerState, WorkerTable},
};

pub const SCRATCH_BYTES: usize = 2 * Section::SettlementClaims.payload_cap()
    + REWARD_STATE_BYTES
    + Section::PolicyLifecycle.payload_cap()
    + 2 * Section::Control.payload_cap();

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct BondRecord {
    pub room: Digest32,
    pub bid: Digest32,
    pub owner: PrincipalId,
    pub worker: WorkerId,
    pub recipient: AccountId,
    pub amount: u128,
    pub slashed: u128,
    pub locked_height: u64,
    pub refunded: bool,
}
impl BondRecord {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    /// # Errors
    /// Returns an error if the requested operation cannot be completed.
    pub fn remaining(&self) -> CodecResult<u128> {
        if self.refunded {
            Ok(0)
        } else {
            self.amount.checked_sub(self.slashed).ok_or(NON_CANONICAL)
        }
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn encode(&self, w: &mut Writer<'_>) -> CodecResult<()> {
        crate::codec::write_fields!(w;
            id32: self.room.as_bytes(); id32: self.bid.as_bytes();
            id32: self.owner.as_bytes(); id32: self.worker.as_bytes();
            u128: self.amount; u128: self.slashed; u64: self.locked_height;
            u8: u8::from(self.refunded);
        )
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(bytes);
        let mut record = crate::codec::read_record!(&mut r, Self { recipient = AccountId::new([1; 32])?; };
            room = r.digest32()?;
            bid: Digest32, owner: PrincipalId, worker: WorkerId,
            amount: U128, slashed: U128, locked_height: U64, refunded: Boolean
        );
        r.finish()?;
        record.recipient = AccountId::new(record.owner.bytes())?;
        if record.amount == 0 || record.slashed > record.amount {
            return Err(NON_CANONICAL);
        }
        Ok(record)
    }
}

#[derive(Clone, Copy)]
pub struct LockBondRequest {
    pub room: Digest32,
    pub bid: Digest32,
    pub amount: u128,
}
impl LockBondRequest {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    /// # Errors
    /// Returns an error if the requested operation cannot be completed.
    pub fn encode(&self) -> CodecResult<[u8; 80]> {
        let mut out = [0; 80];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w; id32: self.room.as_bytes(); id32: self.bid.as_bytes(); u128: self.amount;)?;
        Ok(out)
    }
}
#[derive(Clone, Copy)]
pub struct SlashBondRequest {
    pub deal: Digest32,
    pub bid: Digest32,
    pub lap: u64,
    pub evidence: Digest32,
}
impl SlashBondRequest {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    /// # Errors
    /// Returns an error if the requested operation cannot be completed.
    pub fn encode(&self) -> CodecResult<[u8; 104]> {
        let mut out = [0; 104];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w; id32: self.deal.as_bytes(); id32: self.bid.as_bytes(); u64: self.lap; id32: self.evidence.as_bytes();)?;
        Ok(out)
    }
}
#[derive(Clone, Copy)]
pub struct RefundBondRequest {
    pub room: Digest32,
    pub bid: Digest32,
}
impl RefundBondRequest {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    /// # Errors
    /// Returns an error if the requested operation cannot be completed.
    pub fn encode(&self) -> CodecResult<[u8; 64]> {
        let mut out = [0; 64];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w; id32: self.room.as_bytes(); id32: self.bid.as_bytes();)?;
        Ok(out)
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
/// # Errors
/// Returns an error if the requested operation cannot be completed.
pub fn bond_from_state(current: &[u8], bid: Digest32) -> CodecResult<Option<BondRecord>> {
    RoomBook::decode(state::decode_shared_state(current)?.room_market()?.1)?.bond(bid)
}

/// Principal held by live bonds, independently of the reward ledger.
///
/// # Errors
/// Propagates shared-state, room-book and checked balance refusals.
pub fn held_from_state(current: &[u8]) -> CodecResult<u128> {
    RoomBook::decode(state::decode_shared_state(current)?.room_market()?.1)?.held()
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
/// # Errors
/// Returns an error if the bid cannot be validated against the committed state.
pub fn check_bid(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
) -> CodecResult<()> {
    let state = state::decode_shared_state(current)?;
    let policy = crate::canary::check_call(ctx, &envelope.envelope, &state)?;
    let request = role_request(&state, envelope, false, policy.header.owner_principal)?;
    if let ReplayDecision::AlreadyApplied(_) = state.control.replay.check(&request, ctx.height)? {
        return Ok(());
    }
    let mut r = Reader::new(envelope.envelope.payload);
    let (room, bid) = read_pair(&mut r)?;
    r.take(48)?;
    let amount = r.u128()?;
    let book = RoomBook::committed(state.room_market()?.1)?;
    let record = book.bond(bid)?.ok_or(F11_BOND_INSUFFICIENT)?;
    let profile = book.room(room)?.ok_or(F11_ROOM_UNKNOWN)?.profile;
    if record.owner != ctx.principal
        || record.room != room
        || record.worker != worker(&state, ctx.principal)?.worker
    {
        return Err(UNAUTHORIZED);
    }
    if record.refunded || record.amount != amount || record.remaining()? < profile.minimum_bond {
        return Err(F11_BOND_INSUFFICIENT);
    }
    Ok(())
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn read_pair(r: &mut Reader<'_>) -> CodecResult<(Digest32, Digest32)> {
    Ok(crate::codec::read_values!(r; object = r.digest32()?; bid: Digest32))
}

struct BondRequest {
    object: Digest32,
    bid: Digest32,
    amount: u128,
    lap: u64,
    evidence: Digest32,
}
impl BondRequest {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn decode(operation: dispatch::Operation, bytes: &[u8]) -> CodecResult<Self> {
        operation.validate_payload_length(bytes.len())?;
        let mut r = Reader::new(bytes);
        let (object, bid) = read_pair(&mut r)?;
        let mut value = Self {
            object,
            bid,
            amount: 0,
            lap: 0,
            evidence: object,
        };
        match operation {
            dispatch::LockBond => {
                codec::read_fields(&mut r, &mut [codec::ReadField::U128(&mut value.amount)])?;
            }
            dispatch::SlashBond => codec::read_fields(
                &mut r,
                &mut [
                    codec::ReadField::U64(&mut value.lap),
                    codec::ReadField::Digest32(&mut value.evidence),
                ],
            )?,
            dispatch::RefundBond => (),
            _ => return Err(UNKNOWN_OPERATION),
        }
        r.finish()?;
        Ok(value)
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn role_request(
    state: &SharedState<'_>,
    envelope: &ValidatedEnvelope<'_>,
    host: bool,
    owner: PrincipalId,
) -> CodecResult<ReplayRequest> {
    let slot = if host && envelope.envelope.actor == owner {
        ActorSlot::OWNER
    } else {
        ActorSlot::worker(usize::from(worker(state, envelope.envelope.actor)?.slot))?
    };
    let actor = state.control.replay.actor(slot).ok_or(UNAUTHORIZED)?;
    ReplayRequest::from_envelope(slot, actor.authority_version, envelope)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn worker(state: &SharedState<'_>, who: PrincipalId) -> CodecResult<crate::workers::WorkerCurrent> {
    WorkerTable::decode(
        crate::evaluators::authority::split_identity_section(
            state.section(Section::IdentityRoster)?,
        )?
        .0,
    )?
    .iter()
    .find(|w| w.owner == who && matches!(w.state, WorkerState::Enrolled | WorkerState::Available))
    .copied()
    .ok_or(UNAUTHORIZED)
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn combined(mut ledger: RewardLedger, held: u128) -> CodecResult<RewardLedger> {
    ledger.tracked_deposits = ledger
        .tracked_deposits
        .checked_add(held)
        .ok_or(ARITHMETIC)?;
    ledger.free = ledger.free.checked_add(held).ok_or(ARITHMETIC)?;
    ledger.validate()?;
    Ok(ledger)
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn check_roster(shared: &SharedState<'_>, e: &codec::Envelope<'_>) -> CodecResult<()> {
    let admission = crate::admission::AdmissionTable::decode(
        crate::reputation_transition::split_joint(shared.section(Section::ReputationAdmission)?)?.1,
    )?;
    if admission.current_epoch().is_none() && e.epoch == 0 && e.roster == Presence::Absent {
        return Ok(());
    }
    if admission.current_epoch() != Some(e.epoch) {
        return Err(WRONG_EPOCH);
    }
    let roster = crate::roster::roster_digest(&admission, e.epoch)?;
    if !matches!(e.roster, Presence::Present(value) if value.bytes() == roster.bytes()) {
        return Err(WRONG_ROSTER);
    }
    Ok(())
}
#[cfg_attr(target_arch = "wasm32", inline(always))]
/// # Errors
/// Returns an error if the requested operation cannot be completed.
pub fn apply_lock_bond(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<rooms::Outcome> {
    if envelope.envelope.operation != dispatch::LockBond {
        return Err(UNKNOWN_OPERATION);
    }
    apply(ctx, envelope, current, next, scratch, event)
}
#[cfg_attr(target_arch = "wasm32", inline(always))]
/// # Errors
/// Returns an error if the requested operation cannot be completed.
pub fn apply_slash_bond(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<rooms::Outcome> {
    if envelope.envelope.operation != dispatch::SlashBond {
        return Err(UNKNOWN_OPERATION);
    }
    apply(ctx, envelope, current, next, scratch, event)
}
#[cfg_attr(target_arch = "wasm32", inline(always))]
/// # Errors
/// Returns an error if the requested operation cannot be completed.
pub fn apply_refund_bond(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<rooms::Outcome> {
    if envelope.envelope.operation != dispatch::RefundBond {
        return Err(UNKNOWN_OPERATION);
    }
    apply(ctx, envelope, current, next, scratch, event)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn apply(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<rooms::Outcome> {
    let e = &envelope.envelope;
    e.operation.validate_payload_length(e.payload.len())?;
    let shared = state::decode_shared_state(current)?;
    let policy = crate::canary::check_call(ctx, e, &shared)?;
    check_bond_policy(e, &policy)?;
    let (prefix, book_bytes) = shared.room_market()?;
    let book = RoomBook::committed(book_bytes)?;
    let request = BondRequest::decode(e.operation, e.payload)?;
    let object = request.object;
    let replay = match bond_replay(ctx, envelope, &shared, &policy)? {
        BondReplay::Apply(replay) => replay,
        BondReplay::Retained(retained) => return Ok(rooms::Outcome::Retained(retained)),
    };
    let BondWorkBuffers {
        bond: bond_out,
        settlement,
        reward: reward_out,
        policy: policy_out,
        control,
    } = bond_work_buffers(scratch)?;
    let reward = bond_reward(prefix, &policy, settlement)?;
    let before = reward.ledger()?;
    super::check_bindings(&before, &policy.header)?;
    let (features, control) = control
        .split_at_mut_checked(Section::Control.payload_cap())
        .ok_or(CAPACITY)?;
    let mut feature_len = None;
    let mut effect = RewardEffect::NoTransfer;
    let mut slash_recipients =
        [(WorkerId::new([1; 32])?, AccountId::new([1; 32])?); rooms::MAX_DEAL_PEERS];
    let mut recipient_count = 0;
    let mut slash_amount = 0;
    let mut bond = bond_for_request(ctx, e, &shared, &book, &request)?;
    if e.operation == dispatch::LockBond {
        effect = RewardEffect::Deposit {
            principal: e.actor,
            amount: request.amount,
        };
    }
    if e.operation == dispatch::SlashBond {
        let (amount, count, len) = slash_bond(
            e,
            &shared,
            &policy,
            &book,
            &mut bond,
            &request,
            SlashBuffers {
                height: ctx.height,
                features,
                recipients: &mut slash_recipients,
            },
        )?;
        slash_amount = amount;
        recipient_count = count;
        feature_len = Some(len);
    } else if e.operation == dispatch::RefundBond {
        effect = refund_bond(ctx, e, &shared, &policy, &book, &mut bond, object)?;
    } else if e.operation != dispatch::LockBond {
        return Err(UNKNOWN_OPERATION);
    }
    let ledger = prepare_bond_ledger(
        ctx,
        envelope,
        &book,
        &BondEffect {
            bond: &bond,
            before,
            reward,
            effect,
            slash_amount,
            recipients: slash_recipients.guest_index(..recipient_count),
            prefix_empty: prefix.is_empty(),
        },
        (bond_out, reward_out),
    )?;
    finish_bond(
        ctx,
        envelope,
        shared,
        &policy,
        &ledger,
        BondPersistence {
            bid: bond.bid,
            feature_len,
            replay: replay.as_ref(),
        },
        BondBuffers {
            bond: bond_out,
            settlement,
            reward: reward_out,
            policy: policy_out,
            features,
            control,
            next,
            event,
        },
    )
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
/// # Errors
/// Returns an error if the requested operation cannot be completed.
pub fn apply_close_room(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<rooms::Outcome> {
    let shared = state::decode_shared_state(current)?;
    let policy = crate::canary::check_call(ctx, &envelope.envelope, &shared)?;
    let mut r = Reader::new(envelope.envelope.payload);
    let room_id = r.digest32()?;
    r.finish()?;
    let (prefix, book_bytes) = shared.room_market()?;
    let book = RoomBook::committed(book_bytes)?;
    let room = book.room(room_id)?.ok_or(F11_ROOM_UNKNOWN)?;
    if room.host_key != ctx.principal {
        return Err(UNAUTHORIZED);
    }
    // Preserve the original close/replay path for rooms without bond records.
    if prefix.is_empty() || policy.header.lifecycle == REGISTERED || room.state == RoomState::Closed
    {
        return crate::queries::placement::apply(ctx, envelope, current, next, scratch, event);
    }
    let e = &envelope.envelope;
    if e.operation != dispatch::CloseRoom {
        return Err(UNKNOWN_OPERATION);
    }
    if e.epoch != crate::registry::market_clock(policy.header.origin_height, ctx.height)?.epoch {
        return Err(WRONG_EPOCH);
    }
    if e.config != policy.header.active_config_version {
        return Err(WRONG_CONFIG);
    }
    if !matches!(policy.header.lifecycle, REGISTERED | ACTIVE) {
        return Err(WRONG_PHASE);
    }
    let request = role_request(&shared, envelope, true, policy.header.owner_principal)?;
    if let ReplayDecision::AlreadyApplied(retained) =
        shared.control.replay.check(&request, ctx.height)?
    {
        return Ok(rooms::Outcome::Retained(retained));
    }
    let (book_out, rest) = scratch
        .split_at_mut_checked(rooms::BOOK_MAX_BYTES)
        .ok_or(CAPACITY)?;
    let (settlement, rest) = rest
        .split_at_mut_checked(Section::SettlementClaims.payload_cap())
        .ok_or(CAPACITY)?;
    let (policy_out, control) = rest
        .split_at_mut_checked(Section::PolicyLifecycle.payload_cap())
        .ok_or(CAPACITY)?;
    book_out
        .guest_index_mut(..book_bytes.len())
        .guest_copy_from_slice(book_bytes);
    close_room_deals(&shared, &book, prefix, room_id, book_bytes, book_out)?;
    let counts = book.counts();
    let room_at = book_bytes
        .guest_index(
            rooms::HEADER_BYTES
                ..rooms::HEADER_BYTES + *counts.guest_index(0) * rooms::ROOM_RECORD_BYTES,
        )
        .guest_chunks_exact(rooms::ROOM_RECORD_BYTES)
        .position(|row| row.guest_index(..32) == room_id.as_bytes())
        .ok_or(NOT_FOUND)?;
    let mut closed = room;
    closed.state = RoomState::Closed;
    closed.closed_height = ctx.height;
    let at = rooms::HEADER_BYTES + room_at * rooms::ROOM_RECORD_BYTES;
    closed.encode(book_out.guest_index_mut(at..at + rooms::ROOM_RECORD_BYTES))?;
    let len = shared.encode_room_market(book_out.guest_index(..book_bytes.len()), settlement)?;
    super::finish_room_transition(
        ctx.height,
        envelope,
        shared,
        &policy,
        settlement.guest_index(..len),
        (e.payload, Some(&request)),
        (policy_out, control, next, event),
    )
}

struct SlashBuffers<'a> {
    height: u64,
    features: &'a mut [u8],
    recipients: &'a mut [(WorkerId, AccountId); rooms::MAX_DEAL_PEERS],
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn slash_bond(
    e: &codec::Envelope<'_>,
    shared: &SharedState<'_>,
    policy: &crate::registry_ops::PolicySection<'_>,
    book: &RoomBook<'_>,
    bond: &mut BondRecord,
    request: &BondRequest,
    buffers: SlashBuffers<'_>,
) -> CodecResult<(u128, usize, usize)> {
    let SlashBuffers {
        height,
        features,
        recipients,
    } = buffers;
    let object = request.object;
    let bid = request.bid;
    let lap = request.lap;
    let deal = book.deal(object)?.ok_or(NOT_FOUND)?;
    let room = book.room(deal.room_id)?.ok_or(F11_ROOM_UNKNOWN)?;
    if deal.room_id != bond.room || !deal.layers.iter().flatten().any(|l| l.bid_id == bid) {
        return Err(CONFLICT);
    }
    if e.roster != Presence::Present(deal.roster_digest) {
        return Err(WRONG_ROSTER);
    }
    if bond.refunded {
        return Err(F11_BOND_LOCKED);
    }
    let settled = crate::laps::settlement_from_shared(shared, object)?.ok_or(NOT_FOUND)?;
    if settled.epoch != e.epoch {
        return Err(WRONG_EPOCH);
    }
    if settled.config != e.config {
        return Err(WRONG_CONFIG);
    }
    if settled.roster != deal.roster_digest {
        return Err(WRONG_ROSTER);
    }
    let peer_index = deal
        .layers
        .iter()
        .flatten()
        .position(|layer| layer.bid_id == bid)
        .ok_or(CONFLICT)?;
    check_slash_evidence(
        e,
        shared,
        policy,
        bond,
        SlashEvidence {
            deal: &deal,
            room: &room,
            settled: &settled,
            request,
            peer_index,
            height,
        },
    )?;
    let slash_amount = crate::reward_math::mul_div_rem(
        bond.amount,
        u64::from(room.profile.slash_fraction_bps),
        10_000,
    )?
    .0
    .min(bond.remaining()?);
    let mut recipient_count = 0;
    for layer in deal.layers.iter().flatten() {
        if layer.bid_id == bid {
            continue;
        }
        let peer = book.bond(layer.bid_id)?.ok_or(F11_BOND_INSUFFICIENT)?;
        if peer.owner == bond.owner {
            continue;
        }
        if peer.room != bond.room {
            return Err(CONFLICT);
        }
        *recipients.guest_index_mut(recipient_count) = (peer.worker, peer.recipient);
        recipient_count += 1;
    }
    if recipient_count == 0 {
        return Err(UNAUTHORIZED);
    }
    let mut gap = recipient_count / 2;
    while gap != 0 {
        for i in gap..recipient_count {
            let mut j = i;
            while j >= gap
                && crate::types::identity_order(
                    recipients.guest_index(j).0.as_bytes(),
                    recipients.guest_index(j - gap).0.as_bytes(),
                )
                .is_lt()
            {
                recipients.guest_swap(j, j - gap);
                j -= gap;
            }
        }
        gap /= 2;
    }
    bond.slashed = bond.slashed.checked_add(slash_amount).ok_or(ARITHMETIC)?;
    let feature_len = crate::laps::record_slash(shared, settled, peer_index, lap, features)?;
    Ok((slash_amount, recipient_count, feature_len))
}

#[derive(Clone, Copy)]
struct SlashEvidence<'a> {
    deal: &'a DealRecord,
    room: &'a rooms::RoomRecord,
    settled: &'a crate::laps::LapSettlement,
    request: &'a BondRequest,
    peer_index: usize,
    height: u64,
}

fn check_slash_evidence(
    e: &codec::Envelope<'_>,
    shared: &SharedState<'_>,
    policy: &crate::registry_ops::PolicySection<'_>,
    bond: &BondRecord,
    evidence: SlashEvidence<'_>,
) -> CodecResult<()> {
    let SlashEvidence {
        deal,
        room,
        settled,
        request,
        peer_index,
        height,
    } = evidence;
    let object = request.object;
    let lap = request.lap;
    let evidence = request.evidence;
    let prefix = shared.room_market()?.0;
    let reward_roster = super::committed_epoch_header(prefix, e.epoch)?.1;
    if settled.was_slashed(peer_index, lap) {
        return Err(CONFLICT);
    }
    let canary =
        crate::canary::CanaryRegion::decode(shared.control.feature_bytes)?.get(evidence)?;
    if let Some(record) = canary {
        if !record.peer_at_fault()
            || record.task.deal != object
            || record.task.lap != lap
            || record.task.peer != bond.worker
            || record.task.profile.room != room.profile
            || record.epoch != e.epoch
            || record.config != e.config
            || record.roster != reward_roster
        {
            return Err(UNAUTHORIZED);
        }
    } else {
        let peers = u32::try_from(deal.layers.iter().flatten().count())
            .map_err(|_| crate::errors::CAPACITY)?;
        let deadline =
            crate::commit_reveal::canary_window(policy.header.origin_height, settled.epoch, true)?
                .end;
        if e.actor != room.host_key
            || evidence != settled.batch.batch_digest
            || e.epoch != settled.epoch
            || settled.batch.first_lap_index != lap
            || settled.batch.last_lap_index != lap
            || settled.batch.countersign_quorum.checked_add(1) != Some(peers)
            || settled.batch.missing_countersigners & (1u32 << peer_index) == 0
            || height < deadline
        {
            return Err(UNAUTHORIZED);
        }
    }
    Ok(())
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn refund_bond(
    ctx: &CallContext,
    e: &codec::Envelope<'_>,
    shared: &SharedState<'_>,
    policy: &crate::registry_ops::PolicySection<'_>,
    book: &RoomBook<'_>,
    bond: &mut BondRecord,
    object: Digest32,
) -> CodecResult<RewardEffect> {
    let height = ctx.height;
    let bid = bond.bid;
    let mut effect = RewardEffect::NoTransfer;
    if bond.room != object || bond.owner != e.actor {
        return Err(UNAUTHORIZED);
    }
    if bond.refunded {
        return Err(F11_BOND_LOCKED);
    }
    let room = book.room(object)?.ok_or(F11_ROOM_UNKNOWN)?;
    if room.state != RoomState::Closed {
        return Err(F11_BOND_LOCKED);
    }
    let closed = room.closed_height;
    let end = closed
        .checked_add(room.profile.challenge_window_blocks)
        .ok_or(ARITHMETIC)?;
    if height < end {
        return Err(F11_BOND_LOCKED);
    }
    check_roster(shared, e)?;
    for challenge in crate::canary::CanaryRegion::decode(shared.control.feature_bytes)?.records() {
        let challenge = challenge?;
        if challenge.task.peer != bond.worker {
            continue;
        }
        if let Some(deal) = book.deal(challenge.task.deal)? {
            if !deal
                .layers
                .iter()
                .flatten()
                .any(|layer| layer.bid_id == bond.bid)
            {
                continue;
            }
            if height
                < crate::canary::challenge_window(policy.header.origin_height, &challenge)?.end
                || challenge.status == crate::canary::ChallengeStatus::Open
                || challenge.peer_at_fault()
                    && !crate::laps::settlement_from_shared(shared, deal.deal_id)?.is_some_and(
                        |settled| {
                            deal.layers
                                .iter()
                                .flatten()
                                .position(|l| l.bid_id == bid)
                                .is_some_and(|peer| settled.was_slashed(peer, challenge.task.lap))
                        },
                    )
            {
                return Err(F11_BOND_LOCKED);
            }
        }
    }
    check_refund_deals(shared, policy, book, bond, height)?;
    let amount = bond.remaining()?;
    if amount != 0 {
        effect = RewardEffect::Payout {
            recipient: bond.recipient,
            amount,
        };
    }
    bond.refunded = true;
    Ok(effect)
}

fn check_refund_deals(
    shared: &SharedState<'_>,
    policy: &crate::registry_ops::PolicySection<'_>,
    book: &RoomBook<'_>,
    bond: &BondRecord,
    height: u64,
) -> CodecResult<()> {
    for bytes in book.table(3).guest_chunks_exact(rooms::DEAL_RECORD_BYTES) {
        let deal = DealRecord::decode(bytes)?;
        let mut involved = false;
        for layer in deal.layers.iter().flatten() {
            if book.bid(layer.bid_id)?.ok_or(NOT_FOUND)?.peer_key == bond.owner {
                involved = true;
            }
        }
        if involved {
            if deal.state != DealState::Settled {
                return Err(F11_BOND_LOCKED);
            }
            let closed_room = book.room(deal.room_id)?.ok_or(F11_ROOM_UNKNOWN)?;
            if closed_room.state != RoomState::Closed {
                return Err(F11_BOND_LOCKED);
            }
            let closed = closed_room.closed_height;
            let profile = closed_room.profile;
            let settled = crate::laps::settlement_from_shared(shared, deal.deal_id)?
                .ok_or(F11_BOND_LOCKED)?;
            let challenge_end = crate::commit_reveal::canary_window(
                policy.header.origin_height,
                settled.epoch,
                true,
            )?
            .end
            .checked_add(profile.challenge_window_blocks)
            .ok_or(ARITHMETIC)?;
            if height
                < closed
                    .checked_add(profile.challenge_window_blocks)
                    .ok_or(ARITHMETIC)?
                || height < challenge_end
            {
                return Err(F11_BOND_LOCKED);
            }
        }
    }
    Ok(())
}

#[derive(Clone, Copy)]
struct BondEffect<'a, 'b> {
    bond: &'b BondRecord,
    before: RewardLedger,
    reward: RewardState<'a>,
    effect: RewardEffect,
    slash_amount: u128,
    recipients: &'b [(WorkerId, AccountId)],
    prefix_empty: bool,
}

struct BondBuffers<'a> {
    bond: &'a mut [u8],
    settlement: &'a mut [u8],
    reward: &'a mut [u8],
    policy: &'a mut [u8],
    features: &'a mut [u8],
    control: &'a mut [u8],
    next: &'a mut [u8],
    event: &'a mut [u8],
}

#[derive(Clone, Copy)]
struct BondLedger {
    before: RewardLedger,
    after: RewardLedger,
    held_before: u128,
    held_after: u128,
    reward_len: usize,
    bond_len: usize,
    effect: RewardEffect,
}

#[derive(Clone, Copy)]
struct BondPersistence<'a> {
    bid: Digest32,
    feature_len: Option<usize>,
    replay: Option<&'a ReplayRequest>,
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn prepare_bond_ledger(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    book: &RoomBook<'_>,
    transition: &BondEffect<'_, '_>,
    buffers: (&mut [u8], &mut [u8]),
) -> CodecResult<BondLedger> {
    let e = &envelope.envelope;
    let BondEffect {
        bond,
        before,
        reward,
        effect,
        slash_amount,
        recipients,
        prefix_empty,
    } = *transition;
    let (bond_out, reward_out) = buffers;
    let held_before = book.held_committed()?;
    let held_after = match effect {
        RewardEffect::Deposit { amount, .. } => {
            held_before.checked_add(amount).ok_or(ARITHMETIC)?
        }
        RewardEffect::Payout { amount, .. } => held_before.checked_sub(amount).ok_or(ARITHMETIC)?,
        _ => held_before.checked_sub(slash_amount).ok_or(ARITHMETIC)?,
    };
    let reward_len = if prefix_empty && e.operation == dispatch::LockBond {
        0
    } else {
        REWARD_STATE_BYTES
    };
    let n = book.write_bond(bond, bond_out)?;
    let mut draft = Draft::start(&reward, reward_out)?;
    update_bond_ledger(
        ctx,
        envelope,
        &reward,
        &mut draft,
        effect,
        slash_amount,
        recipients,
    )?;
    let reward_effect = if e.operation == dispatch::RefundBond {
        // A bond refund changes only the ledger: equal deposits and claims
        // preserve the liabilities, epoch rows and recipient bindings.
        // Its transfer is staged against the combined bond and reward ledger.
        RewardEffect::NoTransfer
    } else {
        effect
    };
    let after = if e.operation == dispatch::LockBond {
        draft.carry_unchanged()
    } else {
        draft.commit(reward_effect)?.0
    };
    let ledger_after = after.ledger()?;
    Ok(BondLedger {
        before,
        after: ledger_after,
        held_before,
        held_after,
        reward_len,
        bond_len: n,
        effect,
    })
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn finish_bond<'a>(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    mut shared: SharedState<'a>,
    policy: &crate::registry_ops::PolicySection<'_>,
    ledger: &BondLedger,
    persistence: BondPersistence<'_>,
    buffers: BondBuffers<'a>,
) -> CodecResult<rooms::Outcome> {
    let BondLedger {
        before,
        after: ledger_after,
        held_before,
        held_after,
        reward_len,
        bond_len: n,
        effect,
    } = *ledger;
    let BondPersistence {
        bid,
        feature_len,
        replay,
    } = persistence;
    let BondBuffers {
        bond: bond_out,
        settlement,
        reward: reward_out,
        policy: policy_out,
        features,
        control,
        next,
        event,
    } = buffers;
    let len = state::encode_room_market_parts(
        reward_out.guest_index(..reward_len),
        bond_out.guest_index(..n),
        settlement,
    )?;
    if let Some(len) = feature_len {
        shared.control.feature_bytes = features.guest_index(..len);
    }
    let outcome = super::finish_room_transition(
        ctx.height,
        envelope,
        shared,
        policy,
        settlement.guest_index(..len),
        (bid.as_bytes(), replay),
        (policy_out, control, next, event),
    )?;
    super::stage_value(
        ctx,
        &combined(before, held_before)?,
        &combined(ledger_after, held_after)?,
        effect,
    )?;
    Ok(outcome)
}

fn update_bond_ledger(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    reward: &RewardState<'_>,
    draft: &mut Draft<'_>,
    effect: RewardEffect,
    slash_amount: u128,
    recipients: &[(WorkerId, AccountId)],
) -> CodecResult<()> {
    let e = &envelope.envelope;
    if e.operation == dispatch::SlashBond {
        super::enable_payouts(draft)?;
        let allocation = Digest32::new(envelope.request_digest()?.bytes())?;
        let (equal_share, remainder) =
            crate::reward_math::mul_div_rem(slash_amount, 1, recipients.len() as u64)?;
        let mut distributed = 0u128;
        for i in 0..recipients.len() {
            let (w, recipient) = *recipients.guest_index(i);
            let share = equal_share + u128::from((i as u64) < remainder);
            distributed = distributed.checked_add(share).ok_or(ARITHMETIC)?;
            reward.credit_payout(
                draft,
                None,
                e.epoch,
                w,
                (recipient, share, allocation, ctx.height),
            )?;
        }
        if distributed != slash_amount {
            return Err(ARITHMETIC);
        }
        draft.ledger.tracked_deposits = draft
            .ledger
            .tracked_deposits
            .checked_add(slash_amount)
            .ok_or(ARITHMETIC)?;
        draft.ledger.liability = draft
            .ledger
            .liability
            .checked_add(slash_amount)
            .ok_or(ARITHMETIC)?;
    }
    if let RewardEffect::Payout { amount, .. } = effect {
        draft.ledger.tracked_deposits = draft
            .ledger
            .tracked_deposits
            .checked_add(amount)
            .ok_or(ARITHMETIC)?;
        draft.ledger.total_claimed = draft
            .ledger
            .total_claimed
            .checked_add(amount)
            .ok_or(ARITHMETIC)?;
    }
    Ok(())
}

fn bond_reward<'a>(
    prefix: &'a [u8],
    policy: &crate::registry_ops::PolicySection<'_>,
    settlement: &'a mut [u8],
) -> CodecResult<RewardState<'a>> {
    if prefix.is_empty() {
        let ledger = RewardLedger::new(
            policy.header.funding_asset,
            policy.header.rewards_account,
            policy.header.refund_recipient_account,
        )?;
        RewardState::init(&ledger, settlement.guest_index_mut(..REWARD_STATE_BYTES))
    } else {
        // The current cell is committed. Bond transitions read their ledger
        // and the reward rows they change from that section.
        super::committed_state(prefix.get(..REWARD_STATE_BYTES).ok_or(WRONG_EPOCH)?)
    }
}

fn close_room_deals(
    shared: &SharedState<'_>,
    book: &RoomBook<'_>,
    prefix: &[u8],
    room_id: Digest32,
    book_bytes: &[u8],
    book_out: &mut [u8],
) -> CodecResult<()> {
    let counts = book.counts();
    let deal_at = rooms::HEADER_BYTES
        + *counts.guest_index(0) * rooms::ROOM_RECORD_BYTES
        + *counts.guest_index(1) * rooms::ASK_RECORD_BYTES
        + *counts.guest_index(2) * rooms::BID_RECORD_BYTES;
    let reward = super::committed_state(prefix.get(..REWARD_STATE_BYTES).ok_or(WRONG_EPOCH)?)?;
    for i in 0..*counts.guest_index(3) {
        let at = deal_at + i * rooms::DEAL_RECORD_BYTES;
        let mut deal =
            DealRecord::decode(book_bytes.guest_index(at..at + rooms::DEAL_RECORD_BYTES))?;
        if deal.room_id != room_id {
            continue;
        }
        let settled =
            crate::laps::settlement_from_shared(shared, deal.deal_id)?.ok_or(F11_BOND_LOCKED)?;
        if !super::payout_is_complete(&reward, &settled)? {
            return Err(F11_BOND_LOCKED);
        }
        for challenge in
            crate::canary::CanaryRegion::decode(shared.control.feature_bytes)?.records()
        {
            let challenge = challenge?;
            if challenge.task.deal == deal.deal_id
                && challenge.status == crate::canary::ChallengeStatus::Open
            {
                return Err(F11_BOND_LOCKED);
            }
        }
        deal.state = DealState::Settled;
        deal.encode(book_out.guest_index_mut(at..at + rooms::DEAL_RECORD_BYTES))?;
    }
    Ok(())
}

fn bond_for_request(
    ctx: &CallContext,
    e: &codec::Envelope<'_>,
    shared: &SharedState<'_>,
    book: &RoomBook<'_>,
    request: &BondRequest,
) -> CodecResult<BondRecord> {
    let object = request.object;
    let bid = request.bid;
    if e.operation != dispatch::LockBond {
        return book.bond(bid)?.ok_or(F11_BOND_INSUFFICIENT);
    }
    let amount = request.amount;
    let room = book.room(object)?.ok_or(F11_ROOM_UNKNOWN)?;
    if room.state != RoomState::Open {
        return Err(F11_ROOM_CLOSED);
    }
    if book.bond(bid)?.is_some() || book.bid(bid)?.is_some() {
        return Err(F11_BOND_LOCKED);
    }
    if amount < room.profile.minimum_bond {
        return Err(F11_BOND_INSUFFICIENT);
    }
    book.bond_capacity()?;
    let w = worker(shared, e.actor)?;
    check_roster(shared, e)?;
    Ok(BondRecord {
        room: object,
        bid,
        owner: e.actor,
        worker: w.worker,
        recipient: AccountId::new(e.actor.bytes())?,
        amount,
        slashed: 0,
        locked_height: ctx.height,
        refunded: false,
    })
}

struct BondWorkBuffers<'a> {
    bond: &'a mut [u8],
    settlement: &'a mut [u8],
    reward: &'a mut [u8],
    policy: &'a mut [u8],
    control: &'a mut [u8],
}

fn bond_work_buffers(scratch: &mut [u8]) -> CodecResult<BondWorkBuffers<'_>> {
    let (bond_out, rest) = scratch
        .split_at_mut_checked(Section::SettlementClaims.payload_cap())
        .ok_or(CAPACITY)?;
    let (settlement, rest) = rest
        .split_at_mut_checked(Section::SettlementClaims.payload_cap())
        .ok_or(CAPACITY)?;
    let (reward_out, rest) = rest
        .split_at_mut_checked(REWARD_STATE_BYTES)
        .ok_or(CAPACITY)?;
    let (policy_out, control) = rest
        .split_at_mut_checked(Section::PolicyLifecycle.payload_cap())
        .ok_or(CAPACITY)?;
    Ok(BondWorkBuffers {
        bond: bond_out,
        settlement,
        reward: reward_out,
        policy: policy_out,
        control,
    })
}

fn check_bond_policy(
    e: &codec::Envelope<'_>,
    policy: &crate::registry_ops::PolicySection<'_>,
) -> CodecResult<()> {
    if e.config != policy.header.active_config_version {
        return Err(WRONG_CONFIG);
    }
    if !matches!(policy.header.lifecycle, REGISTERED | ACTIVE) {
        return Err(WRONG_PHASE);
    }
    Ok(())
}

enum BondReplay {
    Apply(Option<ReplayRequest>),
    Retained(state::RetainedResult),
}

fn bond_replay(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    shared: &SharedState<'_>,
    policy: &crate::registry_ops::PolicySection<'_>,
) -> CodecResult<BondReplay> {
    let e = &envelope.envelope;
    let mut replay = None;
    if e.operation != dispatch::SlashBond {
        if e.epoch != crate::registry::market_clock(policy.header.origin_height, ctx.height)?.epoch
        {
            return Err(WRONG_EPOCH);
        }
        let request = role_request(shared, envelope, false, policy.header.owner_principal)?;
        if let ReplayDecision::AlreadyApplied(retained) =
            shared.control.replay.check(&request, ctx.height)?
        {
            return Ok(BondReplay::Retained(retained));
        }
        replay = Some(request);
    }
    Ok(BondReplay::Apply(replay))
}
