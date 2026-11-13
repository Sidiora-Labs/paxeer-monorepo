package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
)

func TestRollupWorkerRejectsChangedDurableRows(t *testing.T) {
	for _, tc := range []struct {
		name, statement, want string
	}{
		{"minute snapshot", `UPDATE candles.chart_exact SET snapshot='1'::jsonb WHERE interval='1m'`, "unmarshal"},
		{"parent snapshot", `UPDATE candles.chart_exact SET snapshot='1'::jsonb WHERE interval='15m'`, "unmarshal"},
		{"changed parent economics", `UPDATE candles.chart_exact SET snapshot=jsonb_set(snapshot,'{volumeUsdlRaw}','"999"'::jsonb) WHERE interval='15m'`, store.ErrIntegrity.Error()},
		{"missing minute snapshot", `DELETE FROM candles.chart_exact WHERE interval='1m'`, ErrInvalidFill.Error()},
		{"changed minute interval", `UPDATE candles.chart_exact SET snapshot=jsonb_set(snapshot,'{interval}','"5m"'::jsonb) WHERE interval='1m'`, ErrInvalidFill.Error()},
		{"changed minute actor", `UPDATE candles.chart_traders SET actor='invalid' WHERE interval='1m'`, ErrInvalidFill.Error()},
		{"unsupported parent interval", `DELETE FROM candles.chart_rollup_pending WHERE interval<>'15m'; UPDATE candles.chart_rollup_pending SET interval='other'`, ErrUnsupportedResolution.Error()},
		{"unavailable pool fence", `ALTER TABLE candles.chart_pool_fences RENAME TO chart_pool_fences_held`, "chart_pool_fences"},
		{"unavailable minute candles", `ALTER TABLE candles.chart_exact RENAME TO chart_exact_held`, "chart_exact"},
		{"unavailable traders", `ALTER TABLE candles.chart_traders RENAME TO chart_traders_held`, "chart_traders"},
		{"unavailable series clock", `ALTER TABLE candles.chart_series_clock RENAME TO chart_series_clock_held`, "chart_series_clock"},
		{"unavailable rollup lineage", `ALTER TABLE candles.chart_rollup_lineage RENAME TO chart_rollup_lineage_held`, "chart_rollup_lineage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
			for _, schema := range []func(string) (string, error){store.ExactSchemaDDL, store.SeriesSchemaDDL} {
				ddl, err := schema("candles")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := pool.Exec(ctx, ddl); err != nil {
					t.Fatal(err)
				}
			}
			exact, err := store.NewExact(pool, "candles")
			if err != nil {
				t.Fatal(err)
			}
			actor := "0x" + strings.Repeat("a", 40)
			source := seriesSource(0, 1704067260, actor)
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err := ApplySeriesTx(ctx, tx, exact, source, 1, SeriesCheckpoint{Height: 100, Timestamp: 1704067380}); err != nil {
				_ = tx.Rollback(ctx)
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, tc.statement); err != nil {
				t.Fatal(err)
			}
			completed, err := RecomputeRollups(ctx, exact, 1)
			if completed != 0 || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("changed durable row accepted: completed=%d err=%v", completed, err)
			}
			var remaining int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_rollup_pending`).Scan(&remaining); err != nil || remaining == 0 {
				t.Fatalf("pending rollup lost: %d, %v", remaining, err)
			}
		})
	}
}
