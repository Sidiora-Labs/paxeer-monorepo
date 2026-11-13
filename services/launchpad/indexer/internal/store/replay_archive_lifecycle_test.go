package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

func TestReplayProjectsAndArchivesCommittedSource(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, pool := canonicaltest.New(t)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test migration path unavailable")
	}
	ddl, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../../launchpad/spec/production-system/integration/migrations/60001.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(ddl)); err != nil {
		t.Fatal(err)
	}
	store := New(pool)
	hash := "0x" + strings.Repeat("2", 64)
	request := ReplayRequest{
		JobID: "archive-replay", SourceStream: "canonical", Target: "charts",
		Generation: "g1", ProjectionRevision: "revision2", Mode: "backfill",
		Requester: "operator", AuthorizationID: "replay-approval", FromHeight: 101,
		ToHeight: 101, CheckpointHeight: 101, CheckpointHash: hash,
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_replay_authorizations VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,1,clock_timestamp()+interval '1 hour',NULL)`, request.AuthorizationID, request.Requester, strings.Repeat("a", 64), request.Mode, request.Target, request.Generation, request.ProjectionRevision, request.FromHeight, request.ToHeight); err != nil {
		t.Fatal(err)
	}
	if err := store.AdmitReplay(ctx, request); err != nil {
		t.Fatal(err)
	}
	block := BlockCompleteness{Height: 101, Hash: hash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}
	if err := store.BindReplayCheckpoint(ctx, request, block); err != nil {
		t.Fatal(err)
	}
	job, err := store.AcquireReplay(ctx, request.JobID, "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	raw := RawRecord{
		TransactionHash: "0x" + strings.Repeat("1", 64), BlockHash: hash,
		Emitter: "0x" + strings.Repeat("3", 40), Height: 101,
		Topics: []string{}, Data: []byte{1},
	}
	digest, err := RawDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	event := ReplayEvent{RawDigest: digest, Kind: "Swap", Timestamp: 1700000000, Payload: json.RawMessage(`{"amount":"1"}`), Raw: raw}
	segment := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: request.SourceStream, FromHeight: 101, ToHeight: 101, Blocks: []BlockCompleteness{block}, Records: []RawRecord{raw}, Events: []ReplayEvent{event}}
	if err := store.CommitReplaySegment(ctx, job, segment, func(ctx context.Context, tx pgx.Tx, event ReplayEvent) (string, error) {
		_, err := tx.Exec(ctx, `INSERT INTO indexer.indexer_generation_raw(target,generation,projection_revision,height,transaction_index,log_index,transaction_hash,raw_digest,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, request.Target, request.Generation, request.ProjectionRevision, event.Raw.Height, event.Raw.TransactionIndex, event.Raw.LogIndex, event.Raw.TransactionHash, event.RawDigest, event.Payload)
		return event.Raw.TransactionHash, err
	}); err != nil {
		t.Fatal(err)
	}
	var progress int64
	var status string
	var projected, effects int
	if err := pool.QueryRow(ctx, `SELECT progress_height,status,(SELECT count(*) FROM indexer.indexer_generation_raw),(SELECT count(*) FROM indexer.indexer_replay_effects) FROM indexer.indexer_replay_jobs WHERE job_id=$1`, request.JobID).Scan(&progress, &status, &projected, &effects); err != nil || progress != 101 || status != "completed" || projected != 1 || effects != 1 {
		t.Fatalf("replay progress=%d status=%q projected=%d effects=%d: %v", progress, status, projected, effects, err)
	}
	if _, err := store.AcquireReplay(ctx, request.JobID, "worker", 30*time.Second); !errors.Is(err, ErrReplayComplete) {
		t.Fatalf("completed replay was acquired: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,1,true,true)`, request.SourceStream, block.Height, block.Hash); err != nil {
		t.Fatal(err)
	}
	if err := store.CheckArchiveSource(ctx, segment); err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(segment)
	if err != nil {
		t.Fatal(err)
	}
	sha := delivery.Hash(wire)
	manifest := ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: request.SourceStream, FromHeight: 101, ToHeight: 101, EventCount: 1, TypedCount: 1, ByteLength: int64(len(wire)), SHA256: sha, Bucket: "archive", ObjectKey: "indexer/raw/v2/canonical/" + sha + ".json", Blocks: []BlockCompleteness{block}}
	if err := store.CompleteArchive(ctx, manifest, segment); err != nil {
		t.Fatal(err)
	}
	found, err := store.FindArchive(ctx, request.SourceStream, 101, 101)
	if err != nil || found.SHA256 != sha || found.EventCount != 1 {
		t.Fatalf("archive lookup: %+v %v", found, err)
	}
	rawCount, typedCount, err := store.CompareReconstructionSegment(ctx, request, segment)
	if err != nil || rawCount != 1 || typedCount != 1 {
		t.Fatalf("reconstruction comparison: raw=%d typed=%d err=%v", rawCount, typedCount, err)
	}
	changed := segment
	changed.Events = append([]ReplayEvent(nil), segment.Events...)
	changed.Events[0].Payload = json.RawMessage(`{"amount":"2"}`)
	if _, _, err := store.CompareReconstructionSegment(ctx, request, changed); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed replay projection accepted: %v", err)
	}
}
