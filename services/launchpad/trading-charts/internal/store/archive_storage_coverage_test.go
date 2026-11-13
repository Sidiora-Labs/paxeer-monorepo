package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
)

func TestArchiveReaderAndRetentionUseOwnedObjectStore(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, schema := range []func(string) (string, error){ExactSchemaDDL, SeriesSchemaDDL, RebuildSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `CREATE SCHEMA indexer;
CREATE TABLE indexer.indexer_archive_segments_v2(object_key text PRIMARY KEY,manifest jsonb NOT NULL,manifest_digest text NOT NULL,verified_at timestamptz NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	exact, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	objects := internaltest.NewObjectStore(t)
	rpc, first := internaltest.NewChainAt(t, 1577836800)
	chain := chainread.Reader{RPC: rpc}
	last, err := chain.Block(ctx, 2)
	if err != nil || first.Time >= last.Time {
		t.Fatalf("archive block span: first=%+v last=%+v err=%v", first, last, err)
	}
	blocks := []ArchiveBlock{{Height: 1, Hash: first.Hash, RawCount: 0, FetchComplete: true, RequiredDecodeComplete: true}, {Height: 2, Hash: last.Hash, RawCount: 0, FetchComplete: true, RequiredDecodeComplete: true}}
	segment := ArchiveSegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 1, ToHeight: 2, Blocks: blocks, Records: []ArchiveRaw{}, Events: []ArchiveEvent{}}
	payload, err := json.Marshal(segment)
	if err != nil {
		t.Fatal(err)
	}
	digest := bytesDigest(payload)
	manifest := ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 1, ToHeight: 2, ByteLength: int64(len(payload)), SHA256: digest, Bucket: objects.Bucket(), ObjectKey: "indexer/raw/v2/canonical/" + digest + ".json", Blocks: blocks}
	manifestWire, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err = objects.Put(ctx, manifest.ObjectKey, bytes.NewReader(payload), int64(len(payload)), "application/json"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_archive_segments_v2 VALUES($1,$2,$3,clock_timestamp())`, manifest.ObjectKey, manifestWire, bytesDigest(manifestWire)); err != nil {
		t.Fatal(err)
	}
	reader := ArchiveReader{Objects: objects, Bucket: objects.Bucket(), Exact: exact, BlockTime: func(ctx context.Context, height int64, expected string) (int64, error) {
		block, err := chain.Block(ctx, height)
		if err != nil {
			return 0, err
		}
		if block.Hash != expected {
			return 0, ErrIntegrity
		}
		return block.Time, nil
	}}
	fills, err := reader.Read(ctx, manifest)
	if err != nil || len(fills) != 0 {
		t.Fatalf("verified empty archive segment: %+v, %v", fills, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE indexer.indexer_archive_segments_v2 SET manifest_digest=$2 WHERE object_key=$1`, manifest.ObjectKey, strings.Repeat("f", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Read(ctx, manifest); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed archived manifest authority: %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE indexer.indexer_archive_segments_v2 SET manifest_digest=$2 WHERE object_key=$1`, manifest.ObjectKey, bytesDigest(manifestWire)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM indexer.indexer_archive_segments_v2 WHERE object_key=$1`, manifest.ObjectKey); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Read(ctx, manifest); err == nil || !strings.Contains(err.Error(), "no rows") {
		t.Fatalf("unlisted archived object: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_archive_segments_v2 VALUES($1,$2,$3,clock_timestamp())`, manifest.ObjectKey, manifestWire, bytesDigest(manifestWire)); err != nil {
		t.Fatal(err)
	}
	if err := objects.Put(ctx, manifest.ObjectKey, bytes.NewReader([]byte("x")), 1, "application/json"); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Read(ctx, manifest); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("archive object length: %v", err)
	}
	if err := objects.Put(ctx, manifest.ObjectKey, bytes.NewReader(payload), int64(len(payload)), "application/json"); err != nil {
		t.Fatal(err)
	}
	poolAddress := "0x" + strings.Repeat("a", 40)
	from, to := first.Time, last.Time
	manifestList, err := json.Marshal([]string{bytesDigest(manifestWire)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_archive_checkpoints(pool,from_time,to_time,manifest_digest) VALUES($1,$2,$3,$4)`, poolAddress, from, to, bytesDigest(manifestList)); err != nil {
		t.Fatal(err)
	}
	start := (from/60 + 1) * 60
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',$2,$3)`, poolAddress, start, `{"interval":"1m","revision":"1"}`); err != nil {
		t.Fatal(err)
	}
	report, err := exact.RetentionDryRun(ctx, reader, poolAddress, "1m", from, to, []ArchiveManifest{manifest})
	if err != nil || report.EligibleRows != 1 || report.ArchiveBytes != int64(len(payload)) || report.ArchiveFillCount != 0 || report.DeletionPerformed || len(report.ManifestDigests) != 1 {
		t.Fatalf("verified retention window: %+v, %v", report, err)
	}
	for _, tc := range []struct {
		name      string
		reader    ArchiveReader
		interval  string
		from, to  int64
		manifests []ArchiveManifest
		want      string
	}{
		{"archive begins after range", reader, "1m", from - 1, to, []ArchiveManifest{manifest}, "retention_archive_time_coverage_missing"},
		{"archive ends before range", reader, "1m", from, to + 1, []ArchiveManifest{manifest}, "retention_archive_time_coverage_missing"},
		{"unsupported resolution", reader, "2m", from, to, []ArchiveManifest{manifest}, "unsupported_resolution"},
		{"disconnected archive ranges", reader, "1m", from, to, []ArchiveManifest{manifest, manifest}, ErrIntegrity.Error()},
		{"invalid archive descriptor", reader, "1m", from, to, []ArchiveManifest{{}}, ErrIntegrity.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observed, err := exact.RetentionDryRun(ctx, tc.reader, poolAddress, tc.interval, tc.from, tc.to, tc.manifests)
			if err == nil || !strings.Contains(err.Error(), tc.want) || observed.DeletionPerformed {
				t.Fatalf("retention admission: %+v, %v", observed, err)
			}
		})
	}
	if removed, err := exact.DeleteRetainedRange(ctx, poolAddress, "1m", from, time.Now().Unix()); removed != 0 || err == nil || !strings.Contains(err.Error(), "retention_floor_not_elapsed") {
		t.Fatalf("recent retention window: %d, %v", removed, err)
	}
	for _, height := range []int64{manifest.FromHeight, manifest.ToHeight} {
		unavailable := reader
		unavailable.BlockTime = func(ctx context.Context, requested int64, expected string) (int64, error) {
			if requested == height {
				return 0, ErrIntegrity
			}
			return reader.BlockTime(ctx, requested, expected)
		}
		if observed, err := exact.RetentionDryRun(ctx, unavailable, poolAddress, "1m", from, to, []ArchiveManifest{manifest}); !errors.Is(err, ErrIntegrity) || observed.DeletionPerformed {
			t.Fatalf("unavailable archive boundary %d: %+v, %v", height, observed, err)
		}
	}
	if _, err := pool.Exec(ctx, `DELETE FROM candles.chart_archive_checkpoints WHERE pool=$1`, poolAddress); err != nil {
		t.Fatal(err)
	}
	if observed, err := exact.RetentionDryRun(ctx, reader, poolAddress, "1m", from, to, []ArchiveManifest{manifest}); err == nil || !strings.Contains(err.Error(), "retention_verified_archive_or_reference_guard") || observed.DeletionPerformed {
		t.Fatalf("retention without archive checkpoint: %+v, %v", observed, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_archive_checkpoints(pool,from_time,to_time,manifest_digest) VALUES($1,$2,$3,$4)`, poolAddress, from, to, bytesDigest(manifestList)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE candles.chart_exact RENAME TO chart_exact_held`); err != nil {
		t.Fatal(err)
	}
	if observed, err := exact.RetentionDryRun(ctx, reader, poolAddress, "1m", from, to, []ArchiveManifest{manifest}); err == nil || !strings.Contains(err.Error(), "chart_exact") || observed.DeletionPerformed {
		t.Fatalf("retention without exact series: %+v, %v", observed, err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE candles.chart_exact_held RENAME TO chart_exact`); err != nil {
		t.Fatal(err)
	}
	if err = objects.Delete(ctx, manifest.ObjectKey); err != nil {
		t.Fatal(err)
	}
	if _, err = exact.RetentionDryRun(ctx, reader, poolAddress, "1m", from, to, []ArchiveManifest{manifest}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing archived segment: %v", err)
	}
}
