package engine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/jackc/pgx/v5"
)

func TestReconstructArchiveQualifiesInitialAndRetainedProjection(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `CREATE SCHEMA indexer;
CREATE TABLE indexer.indexer_archive_segments_v2(object_key text PRIMARY KEY,manifest jsonb NOT NULL,manifest_digest text NOT NULL,verified_at timestamptz NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("a", 40)
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_pool_fences(chain_id,pool,epoch) VALUES('125',$1,1)`, address); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_series_clock(chain_id,pool,revision,watermark_height,watermark_timestamp,has_gaps) VALUES('125',$1,1,2,1704672120,false)`, address); err != nil {
		t.Fatal(err)
	}
	objects := internaltest.NewObjectStore(t)
	rpc, first := internaltest.NewChainSpanAt(t, 1703980800, 8*24*time.Hour)
	chain := chainread.Reader{RPC: rpc}
	last, err := chain.Block(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	blocks := []store.ArchiveBlock{{Height: 1, Hash: first.Hash, RawCount: 0, FetchComplete: true, RequiredDecodeComplete: true}, {Height: 2, Hash: last.Hash, RawCount: 0, FetchComplete: true, RequiredDecodeComplete: true}}
	segment := store.ArchiveSegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 1, ToHeight: 2, Blocks: blocks, Records: []store.ArchiveRaw{}, Events: []store.ArchiveEvent{}}
	payload, err := json.Marshal(segment)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	manifest := store.ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 1, ToHeight: 2, ByteLength: int64(len(payload)), SHA256: hex.EncodeToString(digest[:]), Bucket: objects.Bucket(), ObjectKey: "indexer/raw/v2/canonical/" + hex.EncodeToString(digest[:]) + ".json", Blocks: blocks}
	wire, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestHash := sha256.Sum256(wire)
	if err = objects.Put(ctx, manifest.ObjectKey, bytes.NewReader(payload), int64(len(payload)), "application/json"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_archive_segments_v2 VALUES($1,$2,$3,clock_timestamp())`, manifest.ObjectKey, wire, hex.EncodeToString(manifestHash[:])); err != nil {
		t.Fatal(err)
	}
	reader := store.ArchiveReader{Objects: objects, Bucket: objects.Bucket(), Exact: exact, BlockTime: func(ctx context.Context, height int64, expected string) (int64, error) {
		block, err := chain.Block(ctx, height)
		if err != nil {
			return 0, err
		}
		if block.Hash != expected {
			return 0, store.ErrIntegrity
		}
		return block.Time, nil
	}}
	from, to := int64(1704067200), int64(1704067260)
	if first.Time > from || last.Time < from+604800 {
		t.Fatalf("canonical archive parent bounds: first=%d last=%d", first.Time, last.Time)
	}
	source := seriesSource(0, from+60, "")
	source.PoolAddress = address
	bar, err := FoldQualified(nil, source, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	barWire, err := json.Marshal(bar)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',$2,$3)`, address, from+60, barWire); err != nil {
		t.Fatal(err)
	}
	for index, relation := range []string{"chart_replay_references", "chart_series_clock", "chart_rebuild_runs", "chart_archive_checkpoints", "chart_exact"} {
		t.Run("unavailable "+relation, func(t *testing.T) {
			if _, err := pool.Exec(ctx, `ALTER TABLE candles.`+relation+` RENAME TO `+relation+`_held`); err != nil {
				t.Fatal(err)
			}
			_, failure := ReconstructArchive(ctx, exact, reader, "rebuild_unavailable_"+strconv.Itoa(index), address, from, to, []store.ArchiveManifest{manifest})
			if failure == nil || !strings.Contains(failure.Error(), relation) {
				t.Fatalf("archive without %s: %v", relation, failure)
			}
			if _, err := pool.Exec(ctx, `ALTER TABLE candles.`+relation+`_held RENAME TO `+relation); err != nil {
				t.Fatal(err)
			}
		})
	}
	invalidManifest := manifest
	invalidManifest.SHA256 = "invalid"
	if _, err = ReconstructArchive(ctx, exact, reader, "rebuild_invalid_manifest", address, from, to, []store.ArchiveManifest{invalidManifest}); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("invalid archive manifest: %v", err)
	}
	lastHeaderUnavailable := reader
	lastHeaderUnavailable.BlockTime = func(ctx context.Context, height int64, expected string) (int64, error) {
		if height == manifest.ToHeight {
			return 0, store.ErrIntegrity
		}
		return reader.BlockTime(ctx, height, expected)
	}
	if _, err = ReconstructArchive(ctx, exact, lastHeaderUnavailable, "rebuild_unavailable_last_header", address, from, to, []store.ArchiveManifest{manifest}); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("last archive boundary: %v", err)
	}
	if _, err = ReconstructArchive(ctx, exact, reader, "rebuild_changed_projection", address, from, to, []store.ArchiveManifest{manifest}); err == nil || !strings.Contains(err.Error(), "reconstruction") {
		t.Fatalf("changed projection parity: %v", err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM candles.chart_exact WHERE pool=$1`, address); err != nil {
		t.Fatal(err)
	}
	for i, authority := range []string{"initial_projection_qualification", "retained_promoted_checkpoint"} {
		id := "rebuild_" + string(rune('a'+i))
		report, err := ReconstructArchive(ctx, exact, reader, id, address, from, to, []store.ArchiveManifest{manifest})
		if err != nil || report.ChecksumAuthority != authority || report.ExpectedDigest != report.ActualDigest || report.BarCount != 0 || report.PromotionEpoch != uint64(i+2) || report.RebuiltFrom != from || report.RebuiltTo != from+604800 || len(report.ManifestDigests) != 1 {
			t.Fatalf("archive reconstruction %d: %+v, %v", i, report, err)
		}
	}
	duplicate, err := ReconstructArchive(ctx, exact, reader, "rebuild_duplicate_manifest", address, from, to, []store.ArchiveManifest{manifest, manifest})
	if err != nil || len(duplicate.ManifestDigests) != 1 || duplicate.PromotionEpoch != 4 {
		t.Fatalf("duplicate archive manifest: %+v, %v", duplicate, err)
	}
	for _, tc := range []struct {
		name   string
		reader store.ArchiveReader
		id     string
		from   int64
		to     int64
		want   string
	}{
		{"invalid span", reader, "invalid_span", from, from, ErrInvalidFill.Error()},
		{"invalid replay owner", reader, "", from, to, store.ErrIntegrity.Error()},
		{"missing first header", store.ArchiveReader{Objects: reader.Objects, Bucket: reader.Bucket, Exact: reader.Exact, BlockTime: func(context.Context, int64, string) (int64, error) { return 0, store.ErrIntegrity }}, "missing_first_header", from, to, store.ErrIntegrity.Error()},
		{"late archive start", store.ArchiveReader{Objects: reader.Objects, Bucket: reader.Bucket, Exact: reader.Exact, BlockTime: func(_ context.Context, height int64, _ string) (int64, error) {
			if height == 1 {
				return from + 1, nil
			}
			return last.Time, nil
		}}, "late_start", from, to, "archive_does_not_cover_full_parent_boundaries"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, failure := ReconstructArchive(ctx, exact, tc.reader, tc.id, address, tc.from, tc.to, []store.ArchiveManifest{manifest})
			if failure == nil || !strings.Contains(failure.Error(), tc.want) {
				t.Fatalf("archive admission: %v", failure)
			}
		})
	}
	if err = objects.Delete(ctx, manifest.ObjectKey); err != nil {
		t.Fatal(err)
	}
	if _, err = ReconstructArchive(ctx, exact, reader, "rebuild_missing", address, from, to, []store.ArchiveManifest{manifest}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing canonical archive: %v", err)
	}
	var incidents int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_recoverability_incidents WHERE pool=$1 AND reason='archive_missing_or_corrupt'`, address).Scan(&incidents); err != nil || incidents != 1 {
		t.Fatalf("retained missing object incident: %d, %v", incidents, err)
	}
	filledPool := "0x" + strings.Repeat("b", 40)
	fill := seriesSource(1, from+60, "")
	fill.PoolAddress = filledPool
	fill.Position.Height = "2"
	fill.Position.BlockHash = last.Hash
	fill.BlockTimestampSeconds = strconv.FormatInt(from+60, 10)
	fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
	if err := accounting.ValidateFillV1(fill); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_pool_fences(chain_id,pool,epoch) VALUES('125',$1,1)`, filledPool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_series_clock(chain_id,pool,revision,watermark_height,watermark_timestamp,has_gaps) VALUES('125',$1,1,2,$2,false)`, filledPool, last.Time); err != nil {
		t.Fatal(err)
	}
	for _, interval := range ExactIntervals {
		bar, err := FoldQualified(nil, fill, interval, false)
		if err != nil {
			t.Fatal(err)
		}
		wire, err := json.Marshal(bar)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,$2,$3,$4)`, filledPool, interval, bar.Start, wire); err != nil {
			t.Fatal(err)
		}
	}
	raw := store.ArchiveRaw{Height: 2, TransactionHash: fill.Position.TransactionHash, BlockHash: last.Hash, Emitter: fill.ActualEmitter, TransactionIndex: 0, LogIndex: 1, Topics: []string{}, Data: []byte{}}
	rawWire, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	rawDigest := sha256.Sum256(rawWire)
	fillWire, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	filledBlocks := []store.ArchiveBlock{{Height: 1, Hash: first.Hash, RawCount: 0, FetchComplete: true, RequiredDecodeComplete: true}, {Height: 2, Hash: last.Hash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}
	filledSegment := store.ArchiveSegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 1, ToHeight: 2, Blocks: filledBlocks, Records: []store.ArchiveRaw{raw}, Events: []store.ArchiveEvent{{RawDigest: hex.EncodeToString(rawDigest[:]), Kind: accounting.MarketSubject, Timestamp: from + 60, Payload: fillWire, Raw: raw}}}
	filledPayload, err := json.Marshal(filledSegment)
	if err != nil {
		t.Fatal(err)
	}
	filledDigest := sha256.Sum256(filledPayload)
	filledManifest := store.ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 1, ToHeight: 2, EventCount: 1, TypedCount: 1, ByteLength: int64(len(filledPayload)), SHA256: hex.EncodeToString(filledDigest[:]), Bucket: objects.Bucket(), ObjectKey: "indexer/raw/v2/canonical/" + hex.EncodeToString(filledDigest[:]) + ".json", Blocks: filledBlocks}
	filledManifestWire, err := json.Marshal(filledManifest)
	if err != nil {
		t.Fatal(err)
	}
	filledManifestDigest := sha256.Sum256(filledManifestWire)
	if err = objects.Put(ctx, filledManifest.ObjectKey, bytes.NewReader(filledPayload), int64(len(filledPayload)), "application/json"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_archive_segments_v2 VALUES($1,$2,$3,clock_timestamp())`, filledManifest.ObjectKey, filledManifestWire, hex.EncodeToString(filledManifestDigest[:])); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `CREATE TABLE indexer.indexer_accounting_fills_v1(source_fill_id text PRIMARY KEY,pool_address text NOT NULL,payload jsonb NOT NULL,source_fact_digest text NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	filled, err := ReconstructArchive(ctx, exact, reader, "rebuild_filled", filledPool, from, to, []store.ArchiveManifest{filledManifest})
	if !errors.Is(err, pgx.ErrNoRows) || filled.SourceFills != 1 || filled.BarCount != uint64(len(ExactIntervals)) || filled.ExpectedDigest != filled.ActualDigest || filled.PromotionEpoch != 0 {
		t.Fatalf("canonical filled archive: %+v, %v", filled, err)
	}
	var staged string
	if err = pool.QueryRow(ctx, `SELECT status FROM candles.chart_rebuild_runs WHERE run_id='rebuild_filled'`).Scan(&staged); err != nil || staged != "validated" {
		t.Fatalf("source-bound rebuild stage: %q, %v", staged, err)
	}
}
