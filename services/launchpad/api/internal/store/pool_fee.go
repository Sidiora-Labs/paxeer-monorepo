package store

import (
	"math/big"
	"time"
)

type PoolFeePolicy struct {
	base, minimum, maximum, decay *big.Int
	blockTimestamp                int64
}

func NewPoolFeePolicy(values map[string]string, blockTime time.Time) PoolFeePolicy {
	if blockTime.IsZero() || blockTime.Unix() <= 0 {
		return PoolFeePolicy{}
	}
	var parsed [4]*big.Int
	for i, name := range []string{"baseFeeBps()", "minFeeBps()", "maxFeeBps()", "feeDecayRate()"} {
		raw, err := ParseUInt256(values[name])
		if err != nil {
			return PoolFeePolicy{}
		}
		parsed[i], _ = new(big.Int).SetString(string(raw), 10)
	}
	if parsed[1].Cmp(parsed[2]) > 0 || parsed[2].Cmp(big.NewInt(10000)) >= 0 {
		return PoolFeePolicy{}
	}
	return PoolFeePolicy{base: parsed[0], minimum: parsed[1], maximum: parsed[2], decay: parsed[3], blockTimestamp: blockTime.Unix()}
}

func (p PoolFeePolicy) FeeBps(createdAt int64) (int, bool) {
	if p.base == nil || createdAt <= 0 || createdAt > p.blockTimestamp {
		return 0, false
	}
	if p.base.Cmp(p.maximum) >= 0 {
		return int(p.maximum.Int64()), true
	}
	ageHours := (p.blockTimestamp - createdAt) / 3600
	age := new(big.Int).Quo(p.decay, big.NewInt(1+ageHours))
	remaining := new(big.Int).Sub(p.maximum, p.base)
	if age.Cmp(remaining) >= 0 {
		return int(p.maximum.Int64()), true
	}
	fee := new(big.Int).Add(p.base, age)
	if fee.Cmp(p.minimum) < 0 {
		fee.Set(p.minimum)
	}
	return int(fee.Int64()), true
}
