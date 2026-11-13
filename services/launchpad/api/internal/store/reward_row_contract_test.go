package store

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
)

func TestHolderRewardRowsValidateDatabaseValues(t *testing.T) {
	db := internaltest.NewPostgres(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name           string
		change         func([]any)
		valid, partial bool
	}{
		{"settled reward", func([]any) {}, true, false},
		{"missing market cap", func(v []any) { v[6] = nil }, true, false},
		{"missing strategy", func(v []any) { v[4] = nil }, true, true},
		{"invalid pool", func(v []any) { v[0] = "invalid" }, false, false},
		{"invalid token", func(v []any) { v[1] = "invalid" }, false, false},
		{"negative reward", func(v []any) { v[2] = "-1" }, false, false},
		{"negative holders", func(v []any) { v[3] = int64(-1) }, false, false},
		{"missing creation time", func(v []any) { v[5] = int64(0) }, false, false},
		{"invalid market cap", func(v []any) { v[6] = "invalid" }, false, false},
		{"negative market cap", func(v []any) { v[6] = "-1" }, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := []any{fixtureAddress, "0x2222222222222222222222222222222222222222", "12345678901234567890", int64(4), int32(2), int64(1700000000), "12.5"}
			tc.change(values)
			rows, err := db.Query(ctx, `SELECT $1::text,$2::text,$3::text,$4::bigint,$5::integer,$6::bigint,$7::text`, values...)
			if err != nil {
				t.Fatal(err)
			}
			items, quality, err := scanHolderRewardPools(rows)
			rows.Close()
			if !tc.valid {
				if !errors.Is(err, ErrDataIntegrity) || items != nil {
					t.Fatalf("invalid row: items=%v quality=%q err=%v", items, quality, err)
				}
				return
			}
			if tc.partial {
				if err != nil || quality != "partial" || len(items) != 0 {
					t.Fatalf("row without strategy: items=%v quality=%q err=%v", items, quality, err)
				}
				return
			}
			if err != nil || quality != "complete" || len(items) != 1 || items[0].Pool != fixtureAddress || items[0].DistributedRaw != "12345678901234567890" || !items[0].StrategyActive || items[0].HoldersPaid != 4 {
				t.Fatalf("settled row: items=%v quality=%q err=%v", items, quality, err)
			}
			if values[6] == nil {
				if items[0].MarketCap != nil {
					t.Fatalf("absent cap: %v", items[0].MarketCap)
				}
			} else if items[0].MarketCap == nil || items[0].MarketCap.RatString() != "25/2" {
				t.Fatalf("exact cap: %v", items[0].MarketCap)
			}
		})
	}
	rows, err := db.Query(ctx, `SELECT 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if items, _, err := scanHolderRewardPools(rows); err == nil || items != nil {
		t.Fatalf("mismatched database columns: items=%v err=%v", items, err)
	}
}
