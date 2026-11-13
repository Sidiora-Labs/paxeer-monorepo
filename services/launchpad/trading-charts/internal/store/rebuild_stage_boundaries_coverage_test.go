package store_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestRebuildStageKeepsInvalidInputsOutOfDurableRuns(t *testing.T) {
	f := repair75New(t)
	ctx := context.Background()
	pool := f.fills[0].PoolAddress
	digest := strings.Repeat("a", 64)
	bar := store.StagedCandle{Interval: "1m", Start: 1700000000, Snapshot: json.RawMessage(`{}`)}
	trader := store.StagedTrader{Interval: "1m", Start: bar.Start, Actor: "0x" + strings.Repeat("b", 40)}
	for index, tc := range []struct {
		name, heldRelation, want string
		bars                     []store.StagedCandle
		traders                  []store.StagedTrader
		fills                    []accounting.FillV1
	}{
		{name: "invalid candle document", bars: []store.StagedCandle{{Interval: "1m", Start: bar.Start, Snapshot: json.RawMessage(`{`)}}, want: store.ErrIntegrity.Error()},
		{name: "duplicate candle identity", bars: []store.StagedCandle{bar, bar}, want: "chart_rebuild_candles_pkey"},
		{name: "invalid trader", traders: []store.StagedTrader{{Interval: "1m", Start: bar.Start, Actor: "invalid"}}, want: store.ErrIntegrity.Error()},
		{name: "duplicate trader identity", traders: []store.StagedTrader{trader, trader}, want: "chart_rebuild_traders_pkey"},
		{name: "invalid fill", fills: []accounting.FillV1{{}}, want: "invalid"},
		{name: "runs unavailable", heldRelation: "chart_rebuild_runs", want: "chart_rebuild_runs"},
		{name: "candles unavailable", heldRelation: "chart_rebuild_candles", bars: []store.StagedCandle{bar}, want: "chart_rebuild_candles"},
		{name: "traders unavailable", heldRelation: "chart_rebuild_traders", traders: []store.StagedTrader{trader}, want: "chart_rebuild_traders"},
		{name: "fills unavailable", heldRelation: "chart_rebuild_fills", fills: f.fills[:1], want: "chart_rebuild_fills"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := "stage_boundary_" + string(rune('a'+index))
			if tc.heldRelation != "" {
				repair75Exec(t, f.db, `ALTER TABLE candles.`+tc.heldRelation+` RENAME TO `+tc.heldRelation+`_held`)
			}
			err := f.st.StageRebuild(ctx, id, pool, 1700000000, 1700000060, digest, digest, digest, 1, tc.bars, tc.traders, tc.fills)
			if tc.heldRelation != "" {
				repair75Exec(t, f.db, `ALTER TABLE candles.`+tc.heldRelation+`_held RENAME TO `+tc.heldRelation)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid stage persisted: %v", err)
			}
			var count int
			if err := f.db.QueryRow(ctx, `SELECT count(*) FROM candles.chart_rebuild_runs WHERE run_id=$1`, id).Scan(&count); err != nil || count != 0 {
				t.Fatalf("staged run retained: %d, %v", count, err)
			}
		})
	}
}
