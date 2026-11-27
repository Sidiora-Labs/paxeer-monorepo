package quoter

import (
	"encoding/binary"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
)

// ClientID encodes market, side, level and round, in that order. A round is a
// 48-bit monotonically increasing Unix millisecond value within one process.
func ClientID(market uint64, buy bool, level int, round uint64) [16]byte {
	var id [16]byte
	binary.BigEndian.PutUint64(id[:8], market)
	id[8] = 0xb1
	if buy {
		id[8] = 0xb0
	}
	id[9] = byte(level)
	for i := 15; i >= 10; i-- {
		id[i] = byte(round)
		round >>= 8
	}
	return id
}
func decodeClientID(id [16]byte) (market uint64, buy bool, level int, round uint64, ok bool) {
	if (id[8] != 0xb0 && id[8] != 0xb1) || id[9] < 1 || id[9] > 16 {
		return
	}
	market = binary.BigEndian.Uint64(id[:8])
	buy = id[8] == 0xb0
	level = int(id[9])
	for _, b := range id[10:] {
		round = round<<8 | uint64(b)
	}
	ok = true
	return
}

// Ladder rounds bids down, asks up, and base sizes down to the market lot.
// Full levels below the venue minimum are omitted instead of enlarged.
func Ladder(m Market, mid *big.Int, q config.QuoterConfig, round uint64) []Order {
	var out []Order
	for level := 1; level <= q.Levels; level++ {
		distance := q.SpreadBPS + uint64(level-1)*q.StepBPS
		for _, buy := range []bool{true, false} {
			factor := uint64(10000) + distance
			if buy {
				factor = 10000 - distance
			}
			numerator := new(big.Int).Mul(mid, new(big.Int).SetUint64(factor))
			denominator := new(big.Int).Mul(big.NewInt(10000), m.Tick)
			ticks := new(big.Int).Quo(numerator, denominator)
			if !buy {
				ticks = ceilDiv(numerator, denominator)
			}
			price := new(big.Int).Mul(ticks, m.Tick)
			if price.Sign() <= 0 {
				continue
			}
			base := new(big.Int).Quo(new(big.Int).Mul(q.LevelUSD, unit), price)
			base.Mul(new(big.Int).Quo(base, m.Lot), m.Lot)
			size := new(big.Int).Quo(new(big.Int).Mul(base, price), unit)
			if base.Sign() == 0 || size.Cmp(m.MinOrderUSD) < 0 {
				continue
			}
			// SizeUSD is rounded up for the ABI request so the keeper's USD -> base
			// conversion recovers this exact lot. The stored notional is rounded down.
			request := ceilDiv(new(big.Int).Mul(base, price), unit)
			out = append(out, Order{Market: m.ID, Buy: buy, Level: level, Round: round, Cloid: ClientID(m.ID, buy, level, round), Price: price, SizeUSD: request, Base: base})
		}
	}
	return out
}

func priceMoved(old, current *big.Int, bps uint64) bool {
	delta := new(big.Int).Abs(new(big.Int).Sub(current, old))
	return delta.Mul(delta, big.NewInt(10000)).Cmp(new(big.Int).Mul(old, new(big.Int).SetUint64(bps))) >= 0
}

func withinCap(orders []Order, position, mark, capUSD *big.Int) []Order {
	capBase := new(big.Int).Quo(new(big.Int).Mul(capUSD, unit), mark)
	buyLeft := new(big.Int).Sub(capBase, position)
	sellLeft := new(big.Int).Add(capBase, position)
	var out []Order
	for _, o := range orders {
		left := sellLeft
		if o.Buy {
			left = buyLeft
		}
		if left.Cmp(o.Base) < 0 {
			continue
		}
		left.Sub(left, o.Base)
		out = append(out, o)
	}
	return out
}

// reserveUSD covers the entire order at 1x at the higher of mark and limit,
// plus the account's taker fee rounded up, even for post-only orders.
func reserveUSD(o Order, mark *big.Int, feePPM uint64) *big.Int {
	price := mark
	if o.Price.Cmp(price) > 0 {
		price = o.Price
	}
	risk := ceilDiv(new(big.Int).Mul(o.Base, price), unit)
	fee := ceilDiv(new(big.Int).Mul(risk, new(big.Int).SetUint64(feePPM)), ppm)
	return risk.Add(risk, fee)
}

func sameOrder(actual, want Order) bool {
	return actual.Cloid == want.Cloid && actual.Market == want.Market && actual.Buy == want.Buy && actual.Price.Cmp(want.Price) == 0 && actual.Base.Cmp(want.Base) == 0
}

func noSelfCross(o Order, existing []Order) bool {
	for _, other := range existing {
		if other.Buy == o.Buy {
			continue
		}
		if o.Buy && o.Price.Cmp(other.Price) >= 0 || !o.Buy && o.Price.Cmp(other.Price) <= 0 {
			return false
		}
	}
	return true
}
