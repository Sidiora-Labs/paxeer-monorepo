package store

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestArchivePreservesEmptyRawPayloadsAndRequiresDurableCompletion(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	const stream, owner = "empty-payload", "archive-worker"
	fence, err := st.AcquireLease(ctx, 125, stream, owner, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	raw := lifecycleRaw()
	raw.Topics, raw.Data = nil, nil
	block := BlockCompleteness{Height: raw.Height, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}
	batch := CanonicalBatch{ChainID: 125, StreamID: stream, Owner: owner, BatchID: "empty-raw", Fence: fence, PreviousHeight: raw.Height - 1, Blocks: []BlockCompleteness{block}, Records: []RawRecord{raw}}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,$1,$2,$2)`, stream, batch.PreviousHeight); err != nil {
		t.Fatal(err)
	}
	if err := st.CommitCanonical(ctx, batch, nil); err != nil {
		t.Fatal(err)
	}
	var empty bool
	if err := pool.QueryRow(ctx, `SELECT topics='[]'::jsonb AND data IS NOT NULL AND octet_length(data)=0 FROM indexer.indexer_raw_logs WHERE transaction_hash=$1 AND log_index=$2`, raw.TransactionHash, raw.LogIndex).Scan(&empty); err != nil || !empty {
		t.Fatalf("stored empty log payload: %t, %v", empty, err)
	}
	segment := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: stream, FromHeight: raw.Height, ToHeight: raw.Height, Blocks: []BlockCompleteness{block}, Records: []RawRecord{raw}}
	if err := st.CheckArchiveSource(ctx, segment); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(segment)
	if err != nil {
		t.Fatal(err)
	}
	digest := delivery.Hash(body)
	manifest := ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: stream, FromHeight: raw.Height, ToHeight: raw.Height, EventCount: 1, ByteLength: int64(len(body)), SHA256: digest, Bucket: "archive", ObjectKey: "indexer/raw/v2/" + stream + "/" + digest + ".json", Blocks: []BlockCompleteness{block}}
	unbound := NewFromDBTX(nil)
	for _, call := range []func() error{
		func() error { return unbound.CheckArchiveSource(ctx, segment) },
		func() error { return unbound.CompleteArchive(ctx, manifest, segment) },
	} {
		if err := call(); err == nil || !strings.Contains(err.Error(), "requires transaction") {
			t.Fatalf("archive transaction authority: %v", err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	for _, call := range []func() error{
		func() error { return st.CheckArchiveSource(canceled, segment) },
		func() error { return st.CompleteArchive(canceled, manifest, segment) },
		func() error { return st.CommitCanonical(canceled, batch, nil) },
		func() error { _, err := st.GetCanonicalWatermark(canceled, 125, stream); return err },
	} {
		if err := call(); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled durable archive operation: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE indexer.indexer_archive_segments ADD CONSTRAINT archive_stream CHECK(source_stream<>'empty-payload')`); err != nil {
		t.Fatal(err)
	}
	var databaseError *pgconn.PgError
	if err := st.CompleteArchive(ctx, manifest, segment); !errors.As(err, &databaseError) || databaseError.Code != "23514" {
		t.Fatalf("archive persistence constraint: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_archive_segments`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("uncompleted archive count: %d, %v", count, err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE indexer.indexer_archive_segments DROP CONSTRAINT archive_stream`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := st.CompleteArchive(ctx, manifest, segment); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := st.FindArchive(ctx, stream, raw.Height, raw.Height)
	if err != nil || !reflect.DeepEqual(stored, manifest) {
		t.Fatalf("durable empty-payload manifest: %+v, %v", stored, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_archive_segments`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("idempotent archive count: %d, %v", count, err)
	}
	if height, err := st.GetCanonicalWatermark(ctx, 125, stream); err != nil || height != raw.Height {
		t.Fatalf("archive changed live watermark: %d, %v", height, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_archive_segments SET manifest='"invalid"'::jsonb`); err != nil {
		t.Fatal(err)
	}
	var documentError *json.UnmarshalTypeError
	if _, err := st.FindArchive(ctx, stream, raw.Height, raw.Height); !errors.As(err, &documentError) {
		t.Fatalf("invalid archive manifest document: %v", err)
	}
}
