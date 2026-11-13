package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
)

func TestRetainedRangePreservesRowsWhenGuardRelationsUnavailable(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name, relation string
	}{
		{"replay references", "chart_replay_references"},
		{"archive checkpoints", "chart_archive_checkpoints"},
		{"chart outbox", "chart_outbox"},
		{"trader membership", "chart_traders"},
		{"exact candle", "chart_exact"},
		{"recoverability incident", "chart_recoverability_incidents"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
			for _, schema := range []func(string) (string, error){ExactSchemaDDL, SeriesSchemaDDL, RebuildSchemaDDL} {
				ddl, err := schema("candles")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := pool.Exec(ctx, ddl); err != nil {
					t.Fatal(err)
				}
			}
			exact, err := NewExact(pool, "candles")
			if err != nil {
				t.Fatal(err)
			}
			address := "0x" + strings.Repeat("a", 40)
			if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',60,'{}')`, address); err != nil {
				t.Fatal(err)
			}
			if tc.relation != "chart_recoverability_incidents" {
				if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_archive_checkpoints(pool,from_time,to_time,manifest_digest) VALUES($1,0,120,$2)`, address, strings.Repeat("b", 64)); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := pool.Exec(ctx, `ALTER TABLE candles.`+tc.relation+` RENAME TO `+tc.relation+`_held`); err != nil {
				t.Fatal(err)
			}
			removed, err := exact.DeleteRetainedRange(ctx, address, "1m", 0, 120)
			if removed != 0 || err == nil || !strings.Contains(err.Error(), tc.relation) {
				t.Fatalf("retained deletion without %s: %d, %v", tc.relation, removed, err)
			}
			table := "chart_exact"
			if tc.relation == "chart_exact" {
				table += "_held"
			}
			var count int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.`+table+` WHERE pool=$1`, address).Scan(&count); err != nil || count != 1 {
				t.Fatalf("retained candle after guard error: %d, %v", count, err)
			}
		})
	}
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, schema := range []func(string) (string, error){ExactSchemaDDL, SeriesSchemaDDL, RebuildSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	exact, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("a", 40)
	for _, tc := range []struct {
		name, interval string
		from, to       int64
		want           string
	}{
		{"invalid range", "1m", 60, 60, ErrIntegrity.Error()},
		{"unsupported interval", "invalid", 0, 120, "unsupported_resolution"},
		{"retention floor", "1m", time.Now().Unix() - 60, time.Now().Unix(), "retention_floor_not_elapsed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			removed, err := exact.DeleteRetainedRange(ctx, address, tc.interval, tc.from, tc.to)
			if removed != 0 || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("retained range %s: %d, %v", tc.name, removed, err)
			}
		})
	}
	if err := exact.HoldReplayRange(ctx, "reference", address, 0, 120, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := exact.HoldReplayRange(ctx, "reference", "0x"+strings.Repeat("b", 40), 0, 120, time.Minute); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("replay reference changed owner: %v", err)
	}
}
