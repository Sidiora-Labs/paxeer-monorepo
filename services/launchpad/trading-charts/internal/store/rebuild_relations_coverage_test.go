package store_test

import (
	"context"
	"strings"
	"testing"
)

func TestRebuildPromotionKeepsStagedRunWhenDurableRelationUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name, schema, relation, kind string
	}{
		{"staged run", "candles", "chart_rebuild_runs", "TABLE"},
		{"staged fills", "candles", "chart_rebuild_fills", "TABLE"},
		{"accepted fills", "candles", "chart_inbox", "TABLE"},
		{"series clock", "candles", "chart_series_clock", "TABLE"},
		{"staged candles", "candles", "chart_rebuild_candles", "TABLE"},
		{"exact candles", "candles", "chart_exact", "TABLE"},
		{"source revisions", "candles", "chart_stream_sources", "TABLE"},
		{"trader membership", "candles", "chart_traders", "TABLE"},
		{"chart outbox", "candles", "chart_outbox", "TABLE"},
		{"archive checkpoint", "candles", "chart_archive_checkpoints", "TABLE"},
		{"rebuild stream proof", "candles", "chart_rebuild_stream_sources", "TABLE"},
		{"canonical fills", "indexer", "indexer_accounting_fills_v1", "TABLE"},
		{"canonical publications", "indexer", "indexer_accounting_publications_v1", "TABLE"},
		{"canonical positions", "indexer", "indexer_market_partition_positions_v1", "VIEW"},
		{"source block completeness", "indexer", "indexer_block_completeness_v1", "TABLE"},
		{"stream projection", "api_stream", "stream_projection", "TABLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := repair75New(t)
			f.stage(t, "staged", 1, nil)
			ctx := context.Background()
			original := tc.schema + "." + tc.relation
			held := tc.relation + "_held"
			repair75Exec(t, f.db, "ALTER "+tc.kind+" "+original+" RENAME TO "+held)
			revision, err := f.st.PromoteRebuild(ctx, "staged")
			if revision != 0 || err == nil || !strings.Contains(err.Error(), tc.relation) {
				t.Fatalf("unavailable %s changed promotion: revision=%d err=%v", original, revision, err)
			}
			repair75Exec(t, f.db, "ALTER "+tc.kind+" "+tc.schema+"."+held+" RENAME TO "+tc.relation)
			var status string
			var clock, frames int
			if err := f.db.QueryRow(ctx, `SELECT (SELECT status FROM candles.chart_rebuild_runs WHERE run_id='staged'),(SELECT revision FROM candles.chart_series_clock),(SELECT count(*) FROM api_stream.stream_delivery_outbox)`).Scan(&status, &clock, &frames); err != nil || status != "validated" || clock != 1 || frames != 7 {
				t.Fatalf("staged projection after unavailable relation: status=%q clock=%d frames=%d err=%v", status, clock, frames, err)
			}
		})
	}
}

func TestRebuildPromotionPreservesStagedRunOnDurableWriteRefusal(t *testing.T) {
	for _, tc := range []struct {
		name, relation, operation string
	}{
		{"series revision", "chart_series_clock", "UPDATE"},
		{"staged revision", "chart_rebuild_candles", "UPDATE"},
		{"exact candle", "chart_exact", "INSERT OR UPDATE"},
		{"accepted fill", "chart_inbox", "INSERT"},
		{"chart publication", "chart_outbox", "INSERT"},
		{"archive checkpoint", "chart_archive_checkpoints", "INSERT"},
		{"completed run", "chart_rebuild_runs", "UPDATE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := repair75New(t)
			f.stage(t, "staged", 1, nil)
			ctx := context.Background()
			repair75Exec(t, f.db, `CREATE FUNCTION candles.refuse_rebuild_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'durable write refused'; END $$`)
			repair75Exec(t, f.db, `CREATE TRIGGER refuse_rebuild_write BEFORE `+tc.operation+` ON candles.`+tc.relation+` FOR EACH ROW EXECUTE FUNCTION candles.refuse_rebuild_write()`)
			revision, err := f.st.PromoteRebuild(ctx, "staged")
			if revision != 0 || err == nil || !strings.Contains(err.Error(), "durable write refused") {
				t.Fatalf("durable write refusal %s: revision=%d err=%v", tc.relation, revision, err)
			}
			var status string
			if err := f.db.QueryRow(ctx, `SELECT status FROM candles.chart_rebuild_runs WHERE run_id='staged'`).Scan(&status); err != nil || status != "validated" {
				t.Fatalf("staged run after refused write: %q, %v", status, err)
			}
		})
	}
}
