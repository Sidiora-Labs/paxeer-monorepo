package quoter

import (
	"context"
	"math/big"
	"time"
)

func (e *Engine) roundLimit(id uint64, transactions int) bool {
	if e.cfg.Mode != "live" || transactions < e.cfg.MaxTransactionsPerRound {
		return false
	}
	e.nextMarket = &id
	return true
}

func (e *Engine) closeMarket(ctx context.Context, id uint64, state *marketRound, now time.Time, changed bool) error {
	if !changed {
		return nil
	}
	current, err := e.chain.Snapshot(ctx, id)
	if err == nil {
		err = validateSnapshot(current)
	}
	if err != nil {
		e.record(state, err, now)
		return failed(reason(err))
	}
	e.observe(state, current, now)
	return nil
}

// carryOver intersects the exact integer anchor intervals of the resting
// prices, then checks the complete position-bounded ladder at those prices.
func (e *Engine) carryOver(snap Snapshot, now time.Time) (*big.Int, uint64, bool) {
	if len(snap.Orders) == 0 {
		return nil, 0, false
	}
	one, scale := big.NewInt(1), big.NewInt(10000)
	low := big.NewInt(1)
	var high *big.Int
	var generation uint64
	seen := map[[16]byte]bool{}
	for i, o := range snap.Orders {
		market, buy, level, round, ok := decodeClientID(o.Cloid)
		if !ok || market != snap.Market.ID || level > e.cfg.Levels || seen[o.Cloid] || (i > 0 && round != generation) {
			return nil, 0, false
		}
		seen[o.Cloid] = true
		generation = round
		distance := e.cfg.SpreadBPS + uint64(level-1)*e.cfg.StepBPS
		factor := new(big.Int).SetUint64(10000 + distance)
		var lo, hi *big.Int
		if buy {
			factor.SetUint64(10000 - distance)
			lo = ceilDiv(new(big.Int).Mul(o.Price, scale), factor)
			hi = new(big.Int).Sub(ceilDiv(new(big.Int).Mul(new(big.Int).Add(o.Price, snap.Market.Tick), scale), factor), one)
		} else {
			lo = new(big.Int).Add(new(big.Int).Quo(new(big.Int).Mul(new(big.Int).Sub(o.Price, snap.Market.Tick), scale), factor), one)
			hi = new(big.Int).Quo(new(big.Int).Mul(o.Price, scale), factor)
		}
		if lo.Cmp(low) > 0 {
			low = lo
		}
		if high == nil || hi.Cmp(high) < 0 {
			high = hi
		}
	}
	born := time.UnixMilli(int64(generation)).UTC()
	if born.After(now) || now.Sub(born) >= e.cfg.MaxQuoteAge {
		return nil, 0, false
	}
	// Strict bounds match priceMoved, including equality at the refresh band.
	mark := new(big.Int).Mul(snap.Price.Mark, scale)
	bandLow := new(big.Int).Add(new(big.Int).Quo(mark, new(big.Int).SetUint64(10000+e.cfg.RefreshBPS)), one)
	bandHigh := new(big.Int).Sub(ceilDiv(mark, new(big.Int).SetUint64(10000-e.cfg.RefreshBPS)), one)
	if low.Cmp(bandLow) < 0 {
		low = bandLow
	}
	if high.Cmp(bandHigh) > 0 {
		high = bandHigh
	}
	if low.Cmp(high) > 0 {
		return nil, 0, false
	}
	matches := func(anchor *big.Int) bool {
		wanted := withinCap(Ladder(snap.Market, anchor, e.cfg, generation), snap.PositionBase, snap.Price.Mark, e.cfg.MaxPositionUSD)
		if len(wanted) != len(snap.Orders) {
			return false
		}
		for _, actual := range snap.Orders {
			found := false
			for _, target := range wanted {
				if sameOrder(actual, target) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return !priceMoved(anchor, snap.Price.Mark, e.cfg.RefreshBPS)
	}
	anchor := clone(snap.Price.Mark)
	if anchor.Cmp(low) < 0 {
		anchor.Set(low)
	}
	if anchor.Cmp(high) > 0 {
		anchor.Set(high)
	}
	if matches(anchor) {
		return anchor, generation, true
	}
	// Omitted levels can cross a lot, minimum or position bound within the
	// interval. Visit each distinct ladder, rather than each integer anchor.
	for anchor = clone(low); anchor.Cmp(high) <= 0; {
		if matches(anchor) {
			return anchor, generation, true
		}
		next := new(big.Int).Add(high, one)
		for level := 1; level <= e.cfg.Levels; level++ {
			distance := e.cfg.SpreadBPS + uint64(level-1)*e.cfg.StepBPS
			for _, buy := range []bool{true, false} {
				factor := new(big.Int).SetUint64(10000 + distance)
				if buy {
					factor.SetUint64(10000 - distance)
				}
				numerator := new(big.Int).Mul(anchor, factor)
				denominator := new(big.Int).Mul(scale, snap.Market.Tick)
				ticks := new(big.Int).Quo(numerator, denominator)
				var boundary *big.Int
				if buy {
					boundary = ceilDiv(new(big.Int).Mul(new(big.Int).Add(ticks, one), denominator), factor)
				} else {
					ticks = ceilDiv(numerator, denominator)
					boundary = new(big.Int).Add(new(big.Int).Quo(new(big.Int).Mul(ticks, denominator), factor), one)
				}
				if boundary.Cmp(next) < 0 {
					next = boundary
				}
			}
		}
		anchor = next
	}
	return nil, 0, false
}

func (e *Engine) handover(ctx context.Context) error {
	e.work.Lock()
	defer e.work.Unlock()
	left := -1
	orders, err := e.chain.Orders(ctx)
	if err == nil {
		left = len(orders)
	} else {
		err = failed(reason(err))
	}
	e.mu.Lock()
	e.health.Stopped = true
	e.health.OrdersLeft = left
	e.mu.Unlock()
	e.log.Info("quoter stopped", "mode", e.cfg.Mode, "ordersLeft", left, "handover", true)
	return err
}
