package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/jackc/pgx/v5"
)

func TestReconstructionDigestUsesProjectionAndRetainedCheckpoint(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, schema := range []func(string) (string, error){store.ExactSchemaDDL, store.SeriesSchemaDDL, store.RebuildSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	source := seriesSource(0, 1704067260, "")
	bar, err := FoldQualified(nil, source, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(bar)
	if err != nil {
		t.Fatal(err)
	}
	from, err := BucketStart(1704067260, "1w")
	if err != nil {
		t.Fatal(err)
	}
	to := from + 604800
	manifest := strings.Repeat("a", 64)
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',$2,$3)`, source.PoolAddress, bar.Start, wire); err != nil {
		t.Fatal(err)
	}
	initial, err := projectionDigest([]QualifiedCandle{bar})
	if err != nil {
		t.Fatal(err)
	}
	read := func() (string, string, error) {
		t.Helper()
		tx, e := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		if e != nil {
			t.Fatal(e)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		return reconstructionExpectedDigest(ctx, tx, "candles", source.PoolAddress, from, to, manifest, 2)
	}
	digest, authority, err := read()
	if err != nil || digest != initial || authority != "initial_projection_qualification" {
		t.Fatalf("initial projection commitment: %q, %q, %v", digest, authority, err)
	}
	for _, tc := range []struct {
		name, statement, want string
	}{
		{"missing promoted runs", `ALTER TABLE candles.chart_rebuild_runs RENAME TO chart_rebuild_runs_held`, "chart_rebuild_runs"},
		{"missing retained checkpoints", `ALTER TABLE candles.chart_archive_checkpoints RENAME TO chart_archive_checkpoints_held`, "chart_archive_checkpoints"},
		{"missing exact candles", `ALTER TABLE candles.chart_exact RENAME TO chart_exact_held`, "chart_exact"},
		{"unparseable exact candle", `UPDATE candles.chart_exact SET snapshot='1'::jsonb`, "unmarshal"},
		{"changed exact candle identity", `UPDATE candles.chart_exact SET snapshot=jsonb_set(snapshot,'{poolAddress}','"0x1111111111111111111111111111111111111111"'::jsonb)`, store.ErrIntegrity.Error()},
		{"unsupported exact interval", `UPDATE candles.chart_exact SET interval='other',snapshot=jsonb_set(snapshot,'{interval}','"other"'::jsonb)`, ErrUnsupportedResolution.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, err = tx.Exec(ctx, tc.statement); err != nil {
				t.Fatal(err)
			}
			_, _, err = reconstructionExpectedDigest(ctx, tx, "candles", source.PoolAddress, from, to, manifest, 2)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("retained projection authority %s: %v", tc.name, err)
			}
		})
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_archive_checkpoints(pool,from_time,to_time,manifest_digest) VALUES($1,$2,$3,$4)`, source.PoolAddress, from, to, manifest); err != nil {
		t.Fatal(err)
	}
	if _, _, err = read(); !errors.Is(err, ErrReconstructionCheckpoint) {
		t.Fatalf("retained range without promoted commitment: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_rebuild_runs(run_id,pool,from_time,to_time,manifest_digest,expected_digest,output_digest,status,base_revision,promotion_epoch,promoted_at) VALUES('retained',$1,$2,$3,$4,$5,$5,'promoted',1,2,$6)`, source.PoolAddress, from, to, manifest, initial, time.Now()); err != nil {
		t.Fatal(err)
	}
	digest, authority, err = read()
	if err != nil || digest != initial || authority != "retained_promoted_checkpoint" {
		t.Fatalf("retained promoted commitment: %q, %q, %v", digest, authority, err)
	}
}

func TestDedupCachePrunesExpiredAndBoundedEntries(t *testing.T) {
	cache := newDedupCache(time.Second, 2)
	now := time.Unix(100, 0)
	cache.now = func() time.Time { return now }
	if cache.has("first") {
		t.Fatal("missing key was present")
	}
	cache.add("first")
	if !cache.has("first") {
		t.Fatal("fresh key was absent")
	}
	now = now.Add(2 * time.Second)
	if cache.has("first") {
		t.Fatal("expired key remained present")
	}
	cache.add("second")
	cache.add("third")
	cache.add("fourth")
	if len(cache.entries) != 2 || !cache.has("fourth") {
		t.Fatalf("bounded cache contents: %v", cache.entries)
	}
}
