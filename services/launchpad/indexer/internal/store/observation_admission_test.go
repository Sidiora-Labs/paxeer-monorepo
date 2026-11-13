package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestStorageObservationsRejectUnavailableTablesAndSnapshotCapabilities(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	raw := lifecycleRaw()
	raw.Topics = nil
	raw.Data = nil
	fence, err := st.AcquireLease(ctx, 125, "observation", "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'observation',100,100)`); err != nil {
		t.Fatal(err)
	}
	block := BlockCompleteness{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}
	b := CanonicalBatch{ChainID: 125, StreamID: "observation", Owner: "worker", BatchID: "observation", Fence: fence, PreviousHeight: 100, Blocks: []BlockCompleteness{block}, Records: []RawRecord{raw}, Outbox: []OutboxIntent{{TransactionHash: raw.TransactionHash, Kind: "Swap", Payload: json.RawMessage(`{"amount":"1"}`)}}}
	if err = st.CommitCanonical(ctx, b, nil); err != nil {
		t.Fatal(err)
	}
	segment := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: b.StreamID, FromHeight: 101, ToHeight: 101, Blocks: b.Blocks, Records: b.Records, Events: []ReplayEvent{}}
	request := ReplayRequest{SourceStream: b.StreamID, FromHeight: 101, ToHeight: 101}
	if matched, err := st.ReconcileCanonicalCommit(ctx, b); err != nil || !matched {
		t.Fatalf("committed observations: %v %v", matched, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bound := NewFromDBTX(tx)
	if _, err = bound.MeasureCanonicalRange(ctx, b.StreamID, 101, 101); err == nil {
		t.Fatal("non-snapshot transaction measured")
	}
	if _, _, err = bound.CompareReconstructionSegment(ctx, request, segment); err == nil {
		t.Fatal("non-snapshot reconstruction compared")
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, table string
		run         func() error
	}{
		{"storage schema", "", func() error { _, err := st.MeasureIngestStorage(ctx, ""); return err }},
		{"storage missing", "indexer_raw_logs", func() error { _, err := st.MeasureIngestStorage(ctx, "indexer"); return err }},
		{"storage indexes", "indexer_accounting_fills", func() error { _, err := st.MeasureIngestStorage(ctx, "indexer"); return err }},
		{"block measurement", "indexer_block_completeness", func() error { _, err := st.MeasureCanonicalRange(ctx, b.StreamID, 101, 101); return err }},
		{"raw measurement", "indexer_raw_logs", func() error { _, err := st.MeasureCanonicalRange(ctx, b.StreamID, 101, 101); return err }},
		{"typed measurement", "indexer_ingest_outbox", func() error { _, err := st.MeasureCanonicalRange(ctx, b.StreamID, 101, 101); return err }},
		{"projection measurement", "indexer_accounting_fills", func() error { _, err := st.MeasureCanonicalRange(ctx, b.StreamID, 101, 101); return err }},
		{"intent reconciliation", "indexer_ingest_outbox", func() error { _, err := st.ReconcileCanonicalCommit(ctx, b); return err }},
		{"raw reconstruction", "indexer_raw_logs", func() error { _, _, err := st.CompareReconstructionSegment(ctx, request, segment); return err }},
		{"reconstruction totals", "indexer_replay_effects", func() error { _, _, err := st.CompareReconstructionSegment(ctx, request, segment); return err }},
		{"query plan stream", "", func() error { _, err := st.MeasureIngestPlans(ctx, "", raw, raw.Emitter); return err }},
		{"query plan raw", "", func() error { _, err := st.MeasureIngestPlans(ctx, b.StreamID, RawRecord{}, raw.Emitter); return err }},
		{"query plan pool", "", func() error { _, err := st.MeasureIngestPlans(ctx, b.StreamID, raw, "invalid"); return err }},
		{"query plan table", "indexer_quarantine", func() error { _, err := st.MeasureIngestPlans(ctx, b.StreamID, raw, raw.Emitter); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.table != "" {
				if _, err := pool.Exec(ctx, `ALTER TABLE indexer.`+tc.table+` RENAME TO observation_unavailable`); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if _, err := pool.Exec(ctx, `ALTER TABLE indexer.observation_unavailable RENAME TO `+tc.table); err != nil {
						t.Fatal(err)
					}
				}()
			}
			if err := tc.run(); err == nil {
				t.Fatal("unavailable observation acknowledged")
			}
		})
	}
	pool.Close()
	if matched, err := st.ReconcileCanonicalCommit(ctx, b); err == nil || matched {
		t.Fatalf("closed reconciliation %v %v", matched, err)
	}
	if _, err := st.MeasureCanonicalRange(ctx, b.StreamID, 101, 101); err == nil {
		t.Fatal("closed snapshot measured")
	}
	if _, _, err := st.CompareReconstructionSegment(ctx, request, segment); err == nil {
		t.Fatal("closed reconstruction compared")
	}
}

func TestArchiveCompletionRequiresEveryDurableAcknowledgment(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	raw := lifecycleRaw()
	raw.Topics = nil
	raw.Data = nil
	seed, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := stageImmutableRaw(ctx, seed, 125, raw); err != nil {
		t.Fatal(err)
	}
	if err := seed.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	block := BlockCompleteness{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,'archive-observation',101,$1,1,true,true)`, raw.BlockHash); err != nil {
		t.Fatal(err)
	}
	segment := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "archive-observation", FromHeight: 101, ToHeight: 101, Blocks: []BlockCompleteness{block}, Records: []RawRecord{raw}, Events: []ReplayEvent{}}
	wire, err := json.Marshal(segment)
	if err != nil {
		t.Fatal(err)
	}
	sha := delivery.Hash(wire)
	manifest := ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: segment.SourceStream, FromHeight: 101, ToHeight: 101, EventCount: 1, ByteLength: int64(len(wire)), SHA256: sha, Bucket: "archive", ObjectKey: fmt.Sprintf("indexer/raw/v2/%s/%s.json", segment.SourceStream, sha), Blocks: segment.Blocks}
	for _, mode := range []string{"segment", "digest", "block missing", "raw missing", "insert unavailable", "insert unacknowledged", "conflicting manifest"} {
		t.Run(mode, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			bound := NewFromDBTX(tx)
			s := segment
			m := manifest
			var sql string
			switch mode {
			case "segment":
				s.FormatVersion = "invalid"
			case "digest":
				m.SHA256 = strings.Repeat("0", 64)
				m.ObjectKey = fmt.Sprintf("indexer/raw/v2/%s/%s.json", m.SourceStream, m.SHA256)
			case "block missing":
				sql = `DELETE FROM indexer.indexer_block_completeness`
			case "raw missing":
				sql = `DELETE FROM indexer.indexer_raw_logs`
			case "insert unavailable":
				sql = `ALTER TABLE indexer.indexer_archive_segments RENAME TO archive_unavailable`
			case "insert unacknowledged":
				sql = `CREATE FUNCTION pg_temp.reject_archive() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NULL; END';CREATE TRIGGER reject_archive BEFORE INSERT ON indexer.indexer_archive_segments FOR EACH ROW EXECUTE FUNCTION pg_temp.reject_archive()`
			case "conflicting manifest":
				body, _ := json.Marshal(manifest)
				_, err = tx.Exec(ctx, `INSERT INTO indexer.indexer_archive_segments(segment_id,chain_id,source_stream,from_height,to_height,manifest,manifest_digest,object_key) VALUES($1,125,$2,101,101,$3,$1,$4)`, strings.Repeat("a", 64), manifest.SourceStream, body, manifest.ObjectKey)
				if err != nil {
					t.Fatal(err)
				}
			}
			if sql != "" {
				if _, err = tx.Exec(ctx, sql); err != nil {
					t.Fatal(err)
				}
			}
			if err = bound.CompleteArchive(ctx, m, s); err == nil {
				t.Fatal("unacknowledged archive completion accepted")
			}
			if err = tx.Rollback(ctx); err != nil && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
	if err := st.CompleteArchive(ctx, manifest, segment); err != nil {
		t.Fatal(err)
	}
	got, err := st.FindArchive(ctx, segment.SourceStream, 101, 101)
	if err != nil || got.SHA256 != manifest.SHA256 {
		t.Fatalf("durable archive %s %v", got.SHA256, err)
	}
}
