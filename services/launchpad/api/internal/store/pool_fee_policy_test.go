package store_test

import (
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestPoolFeePolicyDecaysByWholeHoursAndClamps(t *testing.T) {
	block := time.Unix(1700000000, 0)
	for _, tc := range []struct {
		name, base, minimum, maximum, decay string
		age                                 time.Duration
		want                                int
	}{
		{"creation cap", "30", "10", "100", "120", 0, 100},
		{"before first hour", "30", "10", "100", "120", time.Hour - time.Second, 100},
		{"first hour", "30", "10", "100", "120", time.Hour, 90},
		{"second hour", "30", "10", "100", "120", 2 * time.Hour, 70},
		{"integer division", "30", "10", "100", "121", 3 * time.Hour, 60},
		{"minimum", "5", "10", "100", "0", time.Hour, 10},
		{"base above maximum", "101", "10", "100", "0", 0, 100},
		{"base equals maximum", "100", "10", "100", "120", time.Hour, 100},
		{"zero fee", "0", "0", "0", "0", 0, 0},
		{"uint256 decay", "30", "10", "9999", "115792089237316195423570985008687907853269984665640564039457584007913129639935", time.Hour, 9999},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := store.NewPoolFeePolicy(map[string]string{"baseFeeBps()": tc.base, "minFeeBps()": tc.minimum, "maxFeeBps()": tc.maximum, "feeDecayRate()": tc.decay}, block)
			got, ok := policy.FeeBps(block.Add(-tc.age).Unix())
			if !ok || got != tc.want {
				t.Fatalf("fee = %d, valid=%t; want %d", got, ok, tc.want)
			}
		})
	}
}

func TestPoolFeePolicyRejectsInvalidParametersAndCreationTimes(t *testing.T) {
	block := time.Unix(1700000000, 0)
	values := map[string]string{"baseFeeBps()": "30", "minFeeBps()": "10", "maxFeeBps()": "100", "feeDecayRate()": "120"}
	for _, field := range []string{"baseFeeBps()", "minFeeBps()", "maxFeeBps()", "feeDecayRate()"} {
		for _, raw := range []string{"", "01", "-1", strings.Repeat("9", 78)} {
			t.Run(field+raw, func(t *testing.T) {
				candidate := maps.Clone(values)
				candidate[field] = raw
				policy := store.NewPoolFeePolicy(candidate, block)
				if got, ok := policy.FeeBps(block.Unix()); ok || got != 0 {
					t.Fatalf("invalid parameter produced fee %d, valid=%t", got, ok)
				}
			})
		}
	}
	for _, bounds := range [][2]string{{"101", "100"}, {"10", "10000"}} {
		candidate := maps.Clone(values)
		candidate["minFeeBps()"], candidate["maxFeeBps()"] = bounds[0], bounds[1]
		if got, ok := store.NewPoolFeePolicy(candidate, block).FeeBps(block.Unix()); ok || got != 0 {
			t.Fatalf("invalid bounds %v produced %d, valid=%t", bounds, got, ok)
		}
	}
	for _, stamp := range []time.Time{{}, time.Unix(0, 0), time.Unix(-1, 0)} {
		if got, ok := store.NewPoolFeePolicy(values, stamp).FeeBps(block.Unix()); ok || got != 0 {
			t.Fatalf("invalid block time %v produced %d, valid=%t", stamp, got, ok)
		}
	}
	policy := store.NewPoolFeePolicy(values, block)
	for _, created := range []int64{-1, 0, block.Unix() + 1} {
		if got, ok := policy.FeeBps(created); ok || got != 0 {
			t.Fatalf("invalid creation %d produced %d, valid=%t", created, got, ok)
		}
	}
	if got, ok := (store.PoolFeePolicy{}).FeeBps(block.Unix()); ok || got != 0 {
		t.Fatalf("empty policy produced %d, valid=%t", got, ok)
	}
}
