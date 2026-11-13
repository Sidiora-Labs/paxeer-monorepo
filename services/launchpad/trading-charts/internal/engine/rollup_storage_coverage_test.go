package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
)

func TestRollupWorkerPreservesPendingWindowWhenDurableInputUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name, relation string
	}{
		{"pending queue", "chart_rollup_pending"},
		{"pool fence", "chart_pool_fences"},
		{"minute candles", "chart_exact"},
		{"minute actors", "chart_traders"},
		{"series clock", "chart_series_clock"},
		{"lineage", "chart_rollup_lineage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
			for _, schema := range []func(string) (string, error){store.ExactSchemaDDL, store.SeriesSchemaDDL} {
				ddl, err := schema("candles")
				if err != nil {
					t.Fatal(err)
				}
				if _, err = pool.Exec(ctx, ddl); err != nil {
					t.Fatal(err)
				}
			}
			exact, err := store.NewExact(pool, "candles")
			if err != nil {
				t.Fatal(err)
			}
			source := seriesSource(0, 1704067260, "")
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err = ApplySeriesTx(ctx, tx, exact, source, 1, SeriesCheckpoint{Height: 100, Timestamp: 1704067380}); err != nil {
				_ = tx.Rollback(ctx)
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			var pending int
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_rollup_pending WHERE pool=$1`, source.PoolAddress).Scan(&pending); err != nil || pending == 0 {
				t.Fatalf("pending parent window: %d, %v", pending, err)
			}
			if _, err = pool.Exec(ctx, `ALTER TABLE candles.`+tc.relation+` RENAME TO `+tc.relation+`_unavailable`); err != nil {
				t.Fatal(err)
			}
			completed, err := RecomputeRollups(ctx, exact, 1)
			if completed != 0 || err == nil || !strings.Contains(err.Error(), tc.relation) {
				t.Fatalf("rollup with unavailable %s: completed=%d err=%v", tc.relation, completed, err)
			}
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.`+tc.relation+`_unavailable`).Scan(&pending); err != nil {
				t.Fatalf("durable relation retained: %v", err)
			}
		})
	}
	if completed, err := RecomputeRollups(context.Background(), nil, 1); completed != 0 || err != ErrInvalidFill {
		t.Fatalf("missing rollup store: %d, %v", completed, err)
	}
}
