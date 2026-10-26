package ranker

import (
	"strings"
	"testing"
)

func TestFiniteScoreGainersAndLosersUseSignedChange(t *testing.T) {
	huge := strings.Repeat("9", 400)
	for _, tc := range []struct {
		name    string
		change  string
		gainers float64
		losers  float64
	}{
		{"huge fall", "-" + huge, 0, 1e12},
		{"fall above bound", "-1000000000001", 0, 1e12},
		{"fall at bound", "-1000000000000", 0, 1e12},
		{"larger fall", "-25", 0, 25},
		{"smaller fall", "-1.5", 0, 1.5},
		{"fractional fall", "-0.125", 0, 0.125},
		{"negative zero", "-0", 0, 0},
		{"zero", "0", 0, 0},
		{"fractional rise", "0.125", 0.125, 0},
		{"smaller rise", "1.5", 1.5, 0},
		{"larger rise", "25", 25, 0},
		{"rise at bound", "1000000000000", 1e12, 0},
		{"rise above bound", "1000000000001", 1e12, 0},
		{"huge rise", huge, 1e12, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := PoolStat{
				PoolAddress:      "0x0000000000000000000000000000000000000001",
				Volume24h:        "2400",
				Volume1h:         "100",
				Volume5m:         "10",
				MarketCap:        "1000",
				PriceChange24h:   tc.change,
				BuyCount24h:      10,
				SellCount24h:     5,
				UniqueTraders24h: 10,
				HolderCount:      20,
			}
			if err := ValidatePool(p); err != nil {
				t.Fatalf("invalid pool: %v", err)
			}
			for _, category := range []struct {
				name string
				want float64
			}{
				{"gainers", tc.gainers},
				{"losers", tc.losers},
			} {
				got, err := finiteScore(category.name, p)
				if err != nil {
					t.Fatalf("%s: %v", category.name, err)
				}
				if got != category.want {
					t.Errorf("%s change %s: score = %v, want %v", category.name, tc.change, got, category.want)
				}
			}
		})
	}
}
