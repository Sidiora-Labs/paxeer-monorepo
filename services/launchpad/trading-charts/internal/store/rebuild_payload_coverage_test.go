package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
)

func TestRebuildPromotionBindsCanonicalCandleSources(t *testing.T) {
	ctx := context.Background()
	for index, tc := range []struct {
		name, change, want string
	}{
		{"algorithm authority", `UPDATE candles.chart_rebuild_candles SET snapshot=jsonb_set(snapshot,'{algorithmVersion}','"other"'::jsonb) WHERE run_id=$1 AND interval='1m'`, store.ErrIntegrity.Error()},
		{"source order authority", `UPDATE candles.chart_rebuild_candles SET snapshot=jsonb_set(snapshot,'{firstPosition,height}','999'::jsonb) WHERE run_id=$1 AND interval='1m'`, store.ErrIntegrity.Error()},
		{"source price authority", `UPDATE candles.chart_rebuild_candles SET snapshot=jsonb_set(snapshot,'{open,numerator}','"999"'::jsonb) WHERE run_id=$1 AND interval='1m'`, store.ErrIntegrity.Error()},
		{"source asset authority", `UPDATE candles.chart_rebuild_candles SET snapshot=jsonb_set(snapshot,'{token,address}','"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"'::jsonb) WHERE run_id=$1 AND interval='1m'`, store.ErrIntegrity.Error()},
		{"negative candle start", `UPDATE candles.chart_rebuild_candles SET start=-60,snapshot=jsonb_set(jsonb_set(snapshot,'{start}','-60'::jsonb),'{intervalEnd}','0'::jsonb) WHERE run_id=$1 AND interval='1m'`, store.ErrIntegrity.Error()},
		{"invalid candle volume", `UPDATE candles.chart_rebuild_candles SET snapshot=jsonb_set(snapshot,'{volumeUsdlRaw}','"-1"'::jsonb) WHERE run_id=$1 AND interval='1m'`, store.ErrIntegrity.Error()},
		{"source epoch authority", `UPDATE candles.chart_rebuild_candles SET snapshot=jsonb_set(snapshot,'{openSource,provenance,emitterCodeEpoch}','"other-epoch"'::jsonb) WHERE run_id=$1 AND interval='1m'`, "no rows"},
		{"reconstruction reason", `UPDATE candles.chart_rebuild_candles SET snapshot=jsonb_set(snapshot,'{correctionReason}','"other"'::jsonb) WHERE run_id=$1 AND interval='1m'`, store.ErrIntegrity.Error()},
		{"canonical fill authority", `DELETE FROM indexer.indexer_accounting_fills_v1`, "no rows"},
		{"source completeness authority", `DELETE FROM indexer.indexer_block_completeness_v1`, "no rows"},
		{"stream projection authority", `ALTER TABLE api_stream.stream_projection RENAME TO stream_projection_unavailable`, "stream_projection"},
		{"stream source storage", `ALTER TABLE candles.chart_stream_sources RENAME TO chart_stream_sources_unavailable`, "chart_stream_sources"},
		{"delivery attempt storage", `ALTER TABLE candles.chart_delivery_attempts RENAME TO chart_delivery_attempts_unavailable`, "chart_delivery_attempts"},
		{"rollup queue storage", `ALTER TABLE candles.chart_rollup_pending RENAME TO chart_rollup_pending_unavailable`, "chart_rollup_pending"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := repair75New(t)
			id := "source_bound_" + string(rune('a'+index))
			f.stage(t, id, 1, nil)
			var err error
			if strings.Contains(tc.change, "$1") {
				_, err = f.db.Exec(ctx, tc.change, id)
			} else {
				_, err = f.db.Exec(ctx, tc.change)
			}
			if err != nil {
				t.Fatal(err)
			}
			revision, err := f.st.PromoteRebuild(ctx, id)
			if revision != 0 || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("reconstruction source binding %s: revision=%d err=%v", tc.name, revision, err)
			}
			var status string
			if err := f.db.QueryRow(ctx, `SELECT status FROM candles.chart_rebuild_runs WHERE run_id=$1`, id).Scan(&status); err != nil || status != "validated" {
				t.Fatalf("retained staged run: status=%q err=%v", status, err)
			}
		})
	}
	t.Run("refreshes staged completeness", func(t *testing.T) {
		f := repair75New(t)
		f.stage(t, "refresh_status", 1, nil)
		if _, err := f.db.Exec(ctx, `UPDATE candles.chart_rebuild_candles SET snapshot=jsonb_set(snapshot,'{completenessStatus}','"unverified"'::jsonb) WHERE run_id='refresh_status' AND interval='1m'`); err != nil {
			t.Fatal(err)
		}
		revision, err := f.st.PromoteRebuild(ctx, "refresh_status")
		if err != nil || revision < 1 {
			t.Fatalf("promoted refreshed completeness: %d, %v", revision, err)
		}
		var status string
		if err := f.db.QueryRow(ctx, `SELECT snapshot->>'completenessStatus' FROM candles.chart_exact WHERE interval='1m' LIMIT 1`).Scan(&status); err != nil || status == "" || status == "unverified" {
			t.Fatalf("refreshed candle completeness: %q, %v", status, err)
		}
	})
}
