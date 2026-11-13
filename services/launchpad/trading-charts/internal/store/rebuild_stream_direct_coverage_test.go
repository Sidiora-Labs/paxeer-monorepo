package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
)

func TestRebuildStreamReadsOnlyValidatedStagedRows(t *testing.T) {
	ctx := context.Background()
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
	store, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("a", 40)
	digest := strings.Repeat("b", 64)
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO candles.chart_rebuild_runs(run_id,pool,from_time,to_time,manifest_digest,expected_digest,output_digest,status,base_revision) VALUES('stream_rows',$1,0,60,$2,$2,$2,'validated',1)`, []any{address, digest}},
		{`INSERT INTO candles.chart_rebuild_candles(run_id,interval,start,snapshot) VALUES('stream_rows','1m',0,'"unreadable"'::jsonb)`, nil},
		{`INSERT INTO candles.chart_series_clock(chain_id,pool,revision,watermark_height,watermark_timestamp,has_gaps) VALUES('125',$1,1,1,60,false)`, []any{address}},
	} {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	check := func(label, want string, operation func(pgx.Tx) error) {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		failure := operation(tx)
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		if failure == nil || !strings.Contains(failure.Error(), want) {
			t.Fatalf("%s: %v", label, failure)
		}
	}
	check("unreadable staged stream", "unmarshal", func(tx pgx.Tx) error {
		return store.appendRebuildStreamsTx(ctx, tx, "stream_rows", address, digest, digest, 1)
	})
	check("unreadable staged snapshot", "unmarshal", func(tx pgx.Tx) error {
		return store.refreshRebuildSnapshotsTx(ctx, tx, "stream_rows", address)
	})
	position := accounting.SourcePosition{Height: "1", TransactionIndex: "0", LogIndex: "0"}
	bar := rebuildSnapshot{PoolAddress: address, Interval: "1m", Start: 0, IntervalEnd: 60,
		AlgorithmVersion: "qualified-exact-v2", Provenance: "actual_trade", ActorSemantics: "canonical_swap_actor_v1",
		Trades: 1, Revision: 1, DisplayRevision: 1, CompletenessStatus: "live_interval", CorrectionReason: "archive_reconstruction",
		FirstPosition: rebuildOrder{Height: 1}, LastPosition: rebuildOrder{Height: 1},
		OpenSource: rebuildPriceSource{Position: position}, CloseSource: rebuildPriceSource{Position: position},
	}
	barWire, err := json.Marshal(bar)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE candles.chart_rebuild_candles SET snapshot=$1 WHERE run_id='stream_rows'`, barWire); err != nil {
		t.Fatal(err)
	}
	check("missing canonical price source", "indexer_accounting_fills_v1", func(tx pgx.Tx) error {
		return store.appendRebuildStreamsTx(ctx, tx, "stream_rows", address, digest, digest, 1)
	})
	if _, err := pool.Exec(ctx, `ALTER TABLE candles.chart_rebuild_candles RENAME TO chart_rebuild_candles_held`); err != nil {
		t.Fatal(err)
	}
	check("unavailable staged rows", "chart_rebuild_candles", func(tx pgx.Tx) error {
		return store.appendRebuildStreamsTx(ctx, tx, "stream_rows", address, digest, digest, 1)
	})
	if _, err := pool.Exec(ctx, `ALTER TABLE candles.chart_rebuild_candles_held RENAME TO chart_rebuild_candles`); err != nil {
		t.Fatal(err)
	}
	for _, relation := range []string{"chart_delivery_attempts", "chart_rollup_pending"} {
		if _, err := pool.Exec(ctx, `ALTER TABLE candles.`+relation+` RENAME TO `+relation+`_held`); err != nil {
			t.Fatal(err)
		}
		check("unavailable "+relation, relation, func(tx pgx.Tx) error {
			return store.refreshRebuildSnapshotsTx(ctx, tx, "stream_rows", address)
		})
		if _, err := pool.Exec(ctx, `ALTER TABLE candles.`+relation+`_held RENAME TO `+relation); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `DELETE FROM candles.chart_series_clock WHERE pool=$1`, address); err != nil {
		t.Fatal(err)
	}
	check("missing series watermark", "no rows", func(tx pgx.Tx) error {
		return store.refreshRebuildSnapshotsTx(ctx, tx, "stream_rows", address)
	})
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_stream_sources`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unreadable staged rows changed the published stream: %d, %v", count, err)
	}
}
