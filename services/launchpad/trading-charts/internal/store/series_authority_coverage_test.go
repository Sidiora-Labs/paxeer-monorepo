package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/jackc/pgx/v5"
)

func TestSeriesAttemptAndReprocessAuthority(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, schema := range []func(string) (string, error){ExactSchemaDDL, SeriesSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	exact, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	oldDigest, newDigest := strings.Repeat("a", 64), strings.Repeat("b", 64)
	consumer, generation, publication := "charts", "reprocess_next", "publication-1"
	if err = exact.RecordAttempt(ctx, consumer, generation, publication, oldDigest, 1, "received", false); err != nil {
		t.Fatal(err)
	}
	if err = exact.WaitRetryBackoff(ctx, consumer, generation, publication); err != nil {
		t.Fatal(err)
	}
	if err = exact.RecordAttempt(ctx, consumer, generation, publication, oldDigest, 1, "retry", false); err != nil {
		t.Fatal(err)
	}
	if err = exact.WaitRetryBackoff(ctx, consumer, generation, publication); err != nil {
		t.Fatal(err)
	}
	if err = exact.AuthorizeReprocess(ctx, consumer, generation, publication, oldDigest, oldDigest, "accounting-fill-v1", "operator"); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("unchanged decoder and wire: %v", err)
	}
	if err = exact.AuthorizeReprocess(ctx, consumer, generation, publication, oldDigest, newDigest, "accounting-fill-v2", "operator"); err != nil {
		t.Fatal(err)
	}
	if err = exact.CheckReprocess(ctx, consumer, generation, publication, newDigest); err != nil {
		t.Fatal(err)
	}
	if err = exact.CheckReprocess(ctx, consumer, generation, publication, oldDigest); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("unapproved wire: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	unresolved, err := exact.HasUnresolvedDeliveriesTx(ctx, tx)
	if err != nil || !unresolved {
		t.Fatalf("pending delivery: %v, %v", unresolved, err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = exact.RecordAttemptResult(ctx, consumer, generation, publication, "committed", true); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	unresolved, err = exact.HasUnresolvedDeliveriesTx(ctx, tx)
	if err != nil || unresolved {
		t.Fatalf("resolved delivery: %v, %v", unresolved, err)
	}
	if err = exact.RecordAttemptResultTx(ctx, tx, consumer, generation, "absent", "received", false); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("missing delivery attempt: %v", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = exact.AuthorizeReprocess(ctx, consumer, generation, publication, oldDigest, oldDigest, "accounting-fill-v3", "operator"); err == nil || !strings.Contains(err.Error(), "registered_decoder_change_authority_required") {
		t.Fatalf("decoder-only correction authority: %v", err)
	}
	if err = exact.AuthorizeReprocess(ctx, consumer, "reprocess_missing", publication, newDigest, oldDigest, "accounting-fill-v2", "operator"); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("unrecorded prior delivery: %v", err)
	}
	if err = exact.RecordAttemptResult(ctx, consumer, generation, "absent", "received", false); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("unrecorded attempt result: %v", err)
	}
	for _, tc := range []struct {
		name, relation string
		call           func() error
	}{
		{"attempt record", "chart_delivery_attempts", func() error {
			return exact.RecordAttempt(ctx, consumer, generation, "other", oldDigest, 1, "received", false)
		}},
		{"reprocess decoder", "chart_reprocess_audit", func() error {
			return exact.AuthorizeReprocess(ctx, consumer, generation, publication, oldDigest, newDigest, "accounting-fill-v2", "operator")
		}},
		{"retry record", "chart_delivery_attempts", func() error { return exact.WaitRetryBackoff(ctx, consumer, generation, publication) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, `ALTER TABLE candles.`+tc.relation+` RENAME TO `+tc.relation+`_held`); err != nil {
				t.Fatal(err)
			}
			observed := tc.call()
			if _, err := pool.Exec(ctx, `ALTER TABLE candles.`+tc.relation+`_held RENAME TO `+tc.relation); err != nil {
				t.Fatal(err)
			}
			if observed == nil || !strings.Contains(observed.Error(), tc.relation) {
				t.Fatalf("unavailable %s: %v", tc.relation, observed)
			}
		})
	}
}

func TestSeriesSnapshotsRefreshCommittedWatermark(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, schema := range []func(string) (string, error){ExactSchemaDDL, SeriesSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	exact, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("a", 40)
	rows, seed, revision, err := exact.SeriesSnapshots(ctx, address, "1m", 60, 180, 10)
	if err != nil || len(rows) != 0 || len(seed) != 0 || revision != 0 {
		t.Fatalf("empty series: %d, %s, %d, %v", len(rows), seed, revision, err)
	}
	for _, statement := range []string{
		`INSERT INTO candles.chart_pool_fences(chain_id,pool,epoch) VALUES('125',$1,1)`,
		`INSERT INTO candles.chart_series_clock(chain_id,pool,revision,watermark_height,watermark_timestamp,has_gaps) VALUES('125',$1,1,100,180,false)`,
		`INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',60,'{"intervalEnd":120,"completenessStatus":"live_interval"}')`,
	} {
		if _, err = pool.Exec(ctx, statement, address); err != nil {
			t.Fatal(err)
		}
	}
	rows, seed, revision, err = exact.SeriesSnapshots(ctx, address, "1m", 60, 180, 10)
	if err != nil || len(rows) != 1 || len(seed) != 0 || revision != 1 {
		t.Fatalf("committed series: %d, %s, %d, %v", len(rows), seed, revision, err)
	}
	var state struct {
		Status string `json:"completenessStatus"`
		Height uint64 `json:"sourceWatermarkHeight"`
	}
	if err = json.Unmarshal(rows[0], &state); err != nil || state.Status != "complete_through_watermark" || state.Height != 100 {
		t.Fatalf("refreshed watermark: %+v, %v", state, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE candles.chart_pool_fences SET frozen=true WHERE pool=$1`, address); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = exact.SeriesSnapshots(ctx, address, "1m", 60, 180, 10); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("frozen series: %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE candles.chart_pool_fences SET frozen=false WHERE pool=$1`, address); err != nil {
		t.Fatal(err)
	}
	rows, seed, revision, err = exact.SeriesSnapshots(ctx, address, "1m", 120, 180, 10)
	if err != nil || len(rows) != 0 || len(seed) == 0 || revision != 1 {
		t.Fatalf("committed seed: %d %s %d %v", len(rows), seed, revision, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_delivery_attempts(consumer_id,generation,publication_id,wire_digest,source_sequence,last_category,resolved) VALUES('charts','local','pending',$1,1,'received',false)`, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	rows, _, _, err = exact.SeriesSnapshots(ctx, address, "1m", 60, 180, 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("unresolved series: %d, %v", len(rows), err)
	}
	if err = json.Unmarshal(rows[0], &state); err != nil || state.Status != statusBackfillIncomplete {
		t.Fatalf("unresolved completeness: %+v, %v", state, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE candles.chart_delivery_attempts SET resolved=true`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',120,'{"intervalEnd":180}')`, address); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = exact.SeriesSnapshots(ctx, address, "1m", 60, 180, 1); err == nil || !strings.Contains(err.Error(), "series_range_exceeds_bound") {
		t.Fatalf("bounded series rows: %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE candles.chart_exact SET snapshot='{"intervalEnd":"invalid"}' WHERE pool=$1 AND start=120`, address); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = exact.SeriesSnapshots(ctx, address, "1m", 120, 180, 10); err == nil {
		t.Fatal("malformed committed snapshot was admitted")
	}
	if _, _, _, err = exact.SeriesSnapshots(ctx, address, "1m", 180, 240, 10); err == nil {
		t.Fatal("malformed series seed was admitted")
	}
	if _, err = pool.Exec(ctx, `DROP TABLE candles.chart_rollup_pending`); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = exact.SeriesSnapshots(ctx, address, "1m", 60, 180, 10); err == nil {
		t.Fatal("missing rollup authority was admitted")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM candles.chart_pool_fences WHERE pool=$1`, address); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = exact.SeriesSnapshots(ctx, address, "1m", 60, 180, 10); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing pool fence: %v", err)
	}
	if _, _, _, err = exact.SeriesSnapshots(ctx, address, "1m", 60, 180, 0); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("invalid series limit: %v", err)
	}
	for _, tc := range []struct {
		end   int64
		gaps  bool
		stamp int64
		want  string
	}{
		{120, true, 180, statusBackfillIncomplete},
		{120, false, 180, "complete_through_watermark"},
		{120, false, 0, "interval_elapsed"},
		{time.Now().Add(time.Minute).Unix(), false, 0, "live_interval"},
	} {
		raw, e := json.Marshal(map[string]any{"intervalEnd": tc.end})
		if e != nil {
			t.Fatal(e)
		}
		refreshed, e := refreshSeriesSnapshot(raw, 100, tc.stamp, tc.gaps, time.Now())
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(refreshed, &state); e != nil || state.Status != tc.want || state.Height != 100 {
			t.Fatalf("refreshed status %q: %+v, %v", tc.want, state, e)
		}
	}
}
