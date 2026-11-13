package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

// The query interface borrows the live connection without owning transactions.
type querySession struct{ sqlcdb.DBTX }

func TestDurableOperationsRequireOwnedTransactions(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	scope := authorizeStoreOperation(t, pool, "backfill")
	raw := lifecycleRaw()
	digest, err := RawDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	block := BlockCompleteness{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}
	segment := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 101, ToHeight: 101, Blocks: []BlockCompleteness{block}, Records: []RawRecord{raw}, Events: []ReplayEvent{}}
	body, err := json.Marshal(segment)
	if err != nil {
		t.Fatal(err)
	}
	manifest := ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 101, ToHeight: 101, EventCount: 1, TypedCount: 0, ByteLength: int64(len(body)), SHA256: delivery.Hash(body), Bucket: "archive", ObjectKey: "indexer/raw/v2/canonical/" + delivery.Hash(body) + ".json", Blocks: segment.Blocks}
	request := ReplayRequest{JobID: "borrowed", SourceStream: "canonical", Target: scope.Target, Generation: scope.Generation, ProjectionRevision: scope.Revision, Mode: scope.Action, Requester: scope.Operator, AuthorizationID: scope.AuthorizationID, FromHeight: 101, ToHeight: 101, CheckpointHeight: 101, CheckpointHash: raw.BlockHash}
	job := ReplayJob{Request: request, Progress: 100, Owner: "worker", Fence: 1}
	poison := PoisonRecord{ChainID: 125, StreamID: "canonical", Raw: raw, ErrorCode: "arity", ABIRevision: "v1", Requirement: "Swap", Required: true}
	borrowed := *st
	borrowed.db = querySession{st.db}
	waiver := scope
	waiver.Action = "waive"
	repair := scope
	repair.Action = "repair"
	retention := scope
	retention.Action = "retention"
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"admit", func() error { return borrowed.AdmitReplay(ctx, request) }},
		{"acquire", func() error { _, e := borrowed.AcquireReplay(ctx, request.JobID, "worker", time.Second); return e }},
		{"replay", func() error {
			return borrowed.CommitReplaySegment(ctx, job, segment, func(context.Context, pgx.Tx, ReplayEvent) (string, error) {
				t.Fatal("unowned transaction projected input")
				return "", nil
			})
		}},
		{"transition", func() error { return borrowed.TransitionReplay(ctx, scope, request.JobID, "paused", "", nil, nil) }},
		{"checkpoint", func() error { return borrowed.BindReplayCheckpoint(ctx, request, block) }},
		{"conflict", func() error { return borrowed.RecordReplayConflict(ctx, request, segment, "checkpoint differs") }},
		{"resume", func() error { return borrowed.ResumeReplay(ctx, scope, request.JobID) }},
		{"pause", func() error { return borrowed.PauseReplayLease(ctx, job, "unavailable", 101, 101) }},
		{"poison", func() error { return borrowed.RetainPoison(ctx, poison) }},
		{"waive", func() error {
			return borrowed.WaivePoison(ctx, waiver, digest, "Swap", "reviewed", time.Now().Add(time.Minute))
		}},
		{"repair", func() error { return borrowed.ResolvePoison(ctx, repair, digest, raw.BlockHash) }},
		{"head", func() error { return borrowed.RetainCommittedHead(ctx, "canonical", 101, raw.BlockHash, "rpc") }},
		{"head quarantine", func() error { return borrowed.QuarantineCommittedHead(ctx, "canonical") }},
		{"authorization", func() error { return borrowed.CheckOperationAuthorization(ctx, scope) }},
		{"archive source", func() error { return borrowed.CheckArchiveSource(ctx, segment) }},
		{"canonical reconciliation", func() error {
			_, e := borrowed.ReconcileCanonicalCommit(ctx, CanonicalBatch{ChainID: 125, StreamID: "canonical", Owner: "worker", Fence: 1, BatchID: "borrowed", PreviousHeight: 100, Blocks: []BlockCompleteness{block}, Records: []RawRecord{raw}})
			return e
		}},
		{"canonical readiness", func() error { return borrowed.IngestReady(ctx, 125, "canonical", "worker", 1) }},
		{"canonical transaction", func() error {
			return borrowed.CommitCanonical(ctx, CanonicalBatch{ChainID: 125, StreamID: "canonical", Owner: "worker", Fence: 1, BatchID: "borrowed", PreviousHeight: 100, Blocks: []BlockCompleteness{block}, Records: []RawRecord{raw}}, nil)
		}},
		{"archive completion", func() error { return borrowed.CompleteArchive(ctx, manifest, segment) }},
		{"retention", func() error { _, e := borrowed.RetentionEligible(ctx, retention); return e }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := "transaction"
			if tc.name == "head" || tc.name == "head quarantine" {
				want = "guarded committed head"
			}
			if e := tc.call(); e == nil || !strings.Contains(e.Error(), want) {
				t.Fatalf("borrowed query session admitted write: %v", e)
			}
		})
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM indexer.indexer_replay_jobs)+(SELECT count(*) FROM indexer.indexer_raw_logs)+(SELECT count(*) FROM indexer.indexer_committed_heads)`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unowned operations changed durable state %d: %v", count, err)
	}
}
