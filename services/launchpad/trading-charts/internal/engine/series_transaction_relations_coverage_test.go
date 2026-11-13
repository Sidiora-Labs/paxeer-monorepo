package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
)

func TestSeriesTransactionKeepsClaimWhenDurableComponentUnavailable(t *testing.T) {
	for _, relation := range []string{"chart_delivery_attempts", "chart_series_clock", "chart_exact", "chart_traders", "chart_outbox", "chart_rollup_pending"} {
		t.Run(relation, func(t *testing.T) {
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
			if _, err = pool.Exec(ctx, `ALTER TABLE candles.`+relation+` RENAME TO `+relation+`_held`); err != nil {
				t.Fatal(err)
			}
			source := seriesSource(0, 1704067260, "0x"+strings.Repeat("a", 40))
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = ApplySeriesTx(ctx, tx, exact, source, 1, SeriesCheckpoint{Height: 100, Timestamp: 1704067380})
			if err == nil || !strings.Contains(err.Error(), relation) {
				_ = tx.Rollback(ctx)
				t.Fatalf("unavailable %s accepted: %v", relation, err)
			}
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			var claims int
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_inbox`).Scan(&claims); err != nil || claims != 0 {
				t.Fatalf("uncommitted source claim: %d, %v", claims, err)
			}
		})
	}
}
