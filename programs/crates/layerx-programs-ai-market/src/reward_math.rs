//! F06 exact integer reward arithmetic over immutable F05 raw weights. No floating
//! point, saturation, random ties or traversal-order ties; every step is checked.
use crate::types::GuestIndex as _;
use crate::{
    MAX_EVALUATORS, MAX_WORKERS,
    aggregation_codec::{EpochAggregation, MAX_TOTAL_WEIGHT, WorkerAggregate},
    errors::{
        ARITHMETIC, CAPACITY, CodecResult, F06_AGGREGATION_MISMATCH, F06_INVALID_AMOUNT,
        F06_LEDGER_INVARIANT_VIOLATION, F06_UNKNOWN_WORKER_ENTITLEMENT, NON_CANONICAL,
    },
    rewards::RewardOutcome,
    types::{Amount, Presence, Score, WorkerId},
};

pub const QUORUM: usize = 3;
pub const CLAIM_EXPIRY_HEIGHTS: u64 = 4096;

/// Lower median of one worker's accepted scores: rank floor((n-1)/2) of the
/// ascending order. Fewer than quorum accepted observations is no score at all.
///
/// # Errors
/// Returns `CAPACITY` when more than `MAX_EVALUATORS` scores are given.
pub fn lower_median(scores: &[Score]) -> CodecResult<Presence<Score>> {
    if scores.len() > MAX_EVALUATORS {
        return Err(CAPACITY);
    }
    if scores.len() < QUORUM {
        return Ok(Presence::Absent);
    }
    let mut sorted = [0u32; MAX_EVALUATORS];
    let sorted = sorted.guest_index_mut(..scores.len());
    for (slot, score) in sorted.iter_mut().zip(scores) {
        *slot = score.get();
    }
    crate::codec::sort_by(sorted, Ord::cmp);
    Ok(Presence::Present(Score::new(
        *sorted.guest_index((scores.len() - 1) / 2),
    )?))
}

/// Low 64-bit limb of `x`; the masked value always fits, so the error is unreachable.
fn low_limb(x: u128) -> CodecResult<u64> {
    u64::try_from(x & u128::from(u64::MAX)).map_err(|_| ARITHMETIC)
}

/// Exact (floor(b*s/t), (b*s) mod t). The product is held as three 64-bit limbs
/// (below 2^192, inside the required 256-bit domain) and divided limb by limb.
///
/// # Errors
/// Returns `ARITHMETIC` when `t` is zero or the quotient exceeds 128 bits.
pub fn mul_div_rem(b: u128, s: u64, t: u64) -> CodecResult<(u128, u64)> {
    if t == 0 {
        return Err(ARITHMETIC);
    }
    if let Ok(b) = u64::try_from(b) {
        if let Some(product) = b.checked_mul(s) {
            return Ok((u128::from(product / t), product % t));
        }
    }
    let s = u128::from(s);
    let lo = u128::from(low_limb(b)?) * s;
    let hi = (b >> 64) * s;
    let mid = (lo >> 64) + u128::from(low_limb(hi)?);
    let top = u64::try_from((hi >> 64) + (mid >> 64)).map_err(|_| ARITHMETIC)?;
    let t = u128::from(t);
    let mut rem = 0u128;
    let mut quotient = [0u64; 3];
    for (q, limb) in quotient
        .iter_mut()
        .zip([top, low_limb(mid)?, low_limb(lo)?])
    {
        let current = (rem << 64) | u128::from(limb);
        *q = u64::try_from(current.checked_div(t).ok_or(ARITHMETIC)?).map_err(|_| ARITHMETIC)?;
        rem = current.checked_rem(t).ok_or(ARITHMETIC)?;
    }
    if *quotient.guest_index(0) != 0 {
        return Err(ARITHMETIC);
    }
    let rem = u64::try_from(rem).map_err(|_| ARITHMETIC)?;
    Ok((
        (u128::from(*quotient.guest_index(1)) << 64) | u128::from(*quotient.guest_index(2)),
        rem,
    ))
}

/// Exact epoch entitlements in ascending `WorkerId` order. A `NO_ELIGIBLE_SCORE`
/// result carries no entries and releases the whole budget.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Allocation {
    budget: Amount,
    total_weight: u64,
    count: usize,
    workers: [Option<WorkerId>; MAX_WORKERS],
    amounts: [Amount; MAX_WORKERS],
}
impl Allocation {
    #[must_use]
    pub const fn budget(&self) -> Amount {
        self.budget
    }
    #[must_use]
    pub const fn total_weight(&self) -> u64 {
        self.total_weight
    }
    #[must_use]
    pub const fn outcome(&self) -> RewardOutcome {
        if self.total_weight == 0 {
            RewardOutcome::NoEligibleScore
        } else {
            RewardOutcome::Allocated
        }
    }
    #[must_use]
    pub const fn len(&self) -> usize {
        self.count
    }
    #[must_use]
    pub const fn is_empty(&self) -> bool {
        self.count == 0
    }
    /// Entitlement at `index` in ascending `WorkerId` order.
    ///
    /// # Errors
    /// Returns `F06_UNKNOWN_WORKER_ENTITLEMENT` when `index` is out of range; `F06_LEDGER_INVARIANT_VIOLATION` when the slot holds no worker.
    pub fn entitlement(&self, index: usize) -> CodecResult<(WorkerId, Amount)> {
        if index >= self.count {
            return Err(F06_UNKNOWN_WORKER_ENTITLEMENT);
        }
        let worker = self
            .workers
            .guest_index(index)
            .ok_or(F06_LEDGER_INVARIANT_VIOLATION)?;
        Ok((worker, *self.amounts.guest_index(index)))
    }
}

/// Largest-remainder allocation of budget B over positive F05 raw weights.
/// Input order is irrelevant: rows are canonicalized by `WorkerId` first.
///
/// # Errors
/// Returns `F06_INVALID_AMOUNT` when the budget is zero; `CAPACITY` when there are more than `MAX_WORKERS` rows; `NON_CANONICAL` on a duplicate worker; `ARITHMETIC` when a sum overflows or the total exceeds `MAX_TOTAL_WEIGHT`; `F06_LEDGER_INVARIANT_VIOLATION` when the remainder distribution does not conserve the budget.
pub fn allocate(budget: Amount, outputs: &[WorkerAggregate]) -> CodecResult<Allocation> {
    if budget == 0 {
        return Err(F06_INVALID_AMOUNT);
    }
    let n = outputs.len();
    if n > MAX_WORKERS {
        return Err(CAPACITY);
    }
    let mut order = [0usize; MAX_WORKERS];
    for (i, slot) in order.guest_index_mut(..n).iter_mut().enumerate() {
        *slot = i;
    }
    crate::codec::sort_by(order.guest_index_mut(..n), |&a, &b| {
        outputs
            .guest_index(a)
            .worker()
            .cmp(&outputs.guest_index(b).worker())
    });
    if order.guest_index(..n).windows(2).any(|p| {
        outputs.guest_index(*p.guest_index(0)).worker()
            == outputs.guest_index(*p.guest_index(1)).worker()
    }) {
        return Err(NON_CANONICAL);
    }
    let mut total = 0u64;
    for output in outputs {
        total = total
            .checked_add(u64::from(output.weight()))
            .ok_or(ARITHMETIC)?;
    }
    if total > MAX_TOTAL_WEIGHT {
        return Err(ARITHMETIC);
    }
    let mut allocation = Allocation {
        budget,
        total_weight: total,
        count: 0,
        workers: [None; MAX_WORKERS],
        amounts: [0; MAX_WORKERS],
    };
    if total == 0 {
        return Ok(allocation);
    }
    allocation.count = n;
    let mut weights = [0u64; MAX_WORKERS];
    let mut remainders = [0u64; MAX_WORKERS];
    let mut base_sum = 0u128;
    for (k, &i) in order.guest_index(..n).iter().enumerate() {
        let weight = u64::from(outputs.guest_index(i).weight());
        let (q, r) = mul_div_rem(budget, weight, total)?;
        *allocation.workers.guest_index_mut(k) = Some(outputs.guest_index(i).worker());
        *allocation.amounts.guest_index_mut(k) = q;
        *weights.guest_index_mut(k) = weight;
        *remainders.guest_index_mut(k) = r;
        base_sum = base_sum.checked_add(q).ok_or(ARITHMETIC)?;
    }
    let mut left = budget
        .checked_sub(base_sum)
        .ok_or(F06_LEDGER_INVARIANT_VIOLATION)?;
    let mut rank = [0usize; MAX_WORKERS];
    for (k, slot) in rank.guest_index_mut(..n).iter_mut().enumerate() {
        *slot = k;
    }
    crate::codec::sort_by(rank.guest_index_mut(..n), |&a, &b| {
        remainders
            .guest_index(b)
            .cmp(remainders.guest_index(a))
            .then(a.cmp(&b))
    });
    for &k in rank.guest_index(..n) {
        if left == 0 {
            break;
        }
        if *weights.guest_index(k) == 0 || *remainders.guest_index(k) == 0 {
            return Err(F06_LEDGER_INVARIANT_VIOLATION);
        }
        *allocation.amounts.guest_index_mut(k) = allocation
            .amounts
            .guest_index(k)
            .checked_add(1)
            .ok_or(ARITHMETIC)?;
        left -= 1;
    }
    let mut sum = 0u128;
    for amount in allocation.amounts.guest_index(..n) {
        sum = sum.checked_add(*amount).ok_or(ARITHMETIC)?;
    }
    if left != 0 || sum != budget {
        return Err(F06_LEDGER_INVARIANT_VIOLATION);
    }
    Ok(allocation)
}

/// Allocation over the immutable F05 output commitment; its recorded total
/// weight must agree with the recomputed sum.
///
/// # Errors
/// Propagates `allocate`'s refusals; returns `F06_AGGREGATION_MISMATCH` when the recorded total weight differs from the recomputed one.
pub fn allocate_aggregation(
    budget: Amount,
    aggregation: &EpochAggregation<'_>,
) -> CodecResult<Allocation> {
    let allocation = allocate(budget, aggregation.outputs())?;
    if allocation.total_weight != aggregation.total_weight() {
        return Err(F06_AGGREGATION_MISMATCH);
    }
    Ok(allocation)
}

/// Claim window end; overflow refuses rather than saturating.
///
/// # Errors
/// Returns `ARITHMETIC` when the sum overflows.
pub fn claim_expiry(terminal_height: u64) -> CodecResult<u64> {
    terminal_height
        .checked_add(CLAIM_EXPIRY_HEIGHTS)
        .ok_or(ARITHMETIC)
}

/// D = P + X + F + R + C, every intermediate sum checked.
///
/// # Errors
/// Returns `ARITHMETIC` when the sum overflows.
pub fn conservation_holds(
    deposits: Amount,
    claimed: Amount,
    refunds: Amount,
    free: Amount,
    reserved: Amount,
    liability: Amount,
) -> CodecResult<bool> {
    let sum = claimed
        .checked_add(refunds)
        .and_then(|v| v.checked_add(free))
        .and_then(|v| v.checked_add(reserved))
        .and_then(|v| v.checked_add(liability))
        .ok_or(ARITHMETIC)?;
    Ok(sum == deposits)
}

const PAYOUT_BPS: u64 = 10_000;
pub fn split_settled_amount(
    amount: Amount,
    profile: &crate::policy::RoomProfile,
    peers: &[(WorkerId, u64)],
    faults: &[u32],
) -> CodecResult<crate::rewards::PayoutShares> {
    profile.validate()?;
    if amount == 0 {
        return Err(F06_INVALID_AMOUNT);
    }
    if peers.is_empty()
        || peers.len() > MAX_WORKERS
        || faults.is_empty()
        || faults.len() > crate::laps::LAP_BATCH_MAX
    {
        return Err(CAPACITY);
    }
    if peers
        .windows(2)
        .any(|p| p.guest_index(0).0 >= p.guest_index(1).0)
        || peers.iter().any(|p| p.1 == 0)
        || faults
            .iter()
            .any(|mask| peers.len() < 32 && mask >> peers.len() != 0)
    {
        return Err(NON_CANONICAL);
    }
    let uniform = peers.iter().all(|peer| peer.1 == peers.guest_index(0).1);
    let host = mul_div_rem(amount, u64::from(profile.host_share_bps), PAYOUT_BPS)?.0;
    let protocol = mul_div_rem(amount, u64::from(profile.protocol_share_bps), PAYOUT_BPS)?.0;
    let pool = amount
        .checked_sub(host)
        .and_then(|a| a.checked_sub(protocol))
        .ok_or(ARITHMETIC)?;
    let mut result = crate::rewards::PayoutShares {
        host,
        protocol,
        peer_count: peers.len(),
        amounts: [0; MAX_WORKERS],
    };
    let faults = if faults.iter().all(|mask| *mask == 0) {
        faults.guest_index(..1)
    } else {
        faults
    };
    let laps = faults.len() as u128;
    let lap_floor = pool.checked_div(laps).ok_or(CAPACITY)?;
    let lap_remainder = pool.checked_rem(laps).ok_or(CAPACITY)?;
    let high_count = faults
        .get(..lap_remainder as usize)
        .ok_or(CAPACITY)?
        .iter()
        .filter(|&&mask| mask == 0)
        .count() as u128;
    let low_count = faults.iter().filter(|&&mask| mask == 0).count() as u128 - high_count;
    let mut zero_done = [false; 2];
    for (lap, &mask) in faults.iter().enumerate() {
        let budget = lap_floor + u128::from((lap as u128) < lap_remainder);
        let group = usize::from((lap as u128) >= lap_remainder);
        let repetitions = if mask == 0 {
            if core::mem::replace(zero_done.guest_index_mut(group), true) {
                continue;
            }
            if group == 0 { high_count } else { low_count }
        } else {
            1
        };
        let mut total = 0u64;
        for (i, &(_, weight)) in peers.iter().enumerate() {
            if mask & (1u32 << i) == 0 {
                total = total.checked_add(weight).ok_or(ARITHMETIC)?;
            }
        }
        if total == 0 {
            return Err(F06_INVALID_AMOUNT);
        }
        let mut remainders = [0; MAX_WORKERS];
        let mut left = budget;
        for (i, &(_, weight)) in peers.iter().enumerate() {
            if mask & (1u32 << i) != 0 {
                continue;
            }
            let (share, remainder) = mul_div_rem(budget, weight, total)?;
            *result.amounts.guest_index_mut(i) = result
                .amounts
                .guest_index(i)
                .checked_add(share.checked_mul(repetitions).ok_or(ARITHMETIC)?)
                .ok_or(ARITHMETIC)?;
            *remainders.guest_index_mut(i) = remainder;
            left = left.checked_sub(share).ok_or(ARITHMETIC)?;
        }
        let mut order: [usize; MAX_WORKERS] = core::array::from_fn(|index| index);
        if !uniform {
            crate::codec::sort_by(order.guest_index_mut(..peers.len()), |&a, &b| {
                remainders
                    .guest_index(b)
                    .cmp(remainders.guest_index(a))
                    .then(a.cmp(&b))
            });
        }
        for &i in order.guest_index(..peers.len()) {
            if left == 0 {
                break;
            }
            if *remainders.guest_index(i) == 0 {
                if uniform {
                    continue;
                }
                return Err(F06_LEDGER_INVARIANT_VIOLATION);
            }
            *result.amounts.guest_index_mut(i) = result
                .amounts
                .guest_index(i)
                .checked_add(repetitions)
                .ok_or(ARITHMETIC)?;
            left -= 1;
        }
        if left != 0 {
            return Err(F06_LEDGER_INVARIANT_VIOLATION);
        }
    }
    if result
        .amounts
        .guest_index(..peers.len())
        .iter()
        .try_fold(host.checked_add(protocol).ok_or(ARITHMETIC)?, |sum, a| {
            sum.checked_add(*a).ok_or(ARITHMETIC)
        })?
        != amount
    {
        return Err(F06_LEDGER_INVARIANT_VIOLATION);
    }
    Ok(result)
}
