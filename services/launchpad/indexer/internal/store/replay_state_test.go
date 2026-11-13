package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestReplayLeasePauseResumeAndConflictRetainIssuedScope(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	scope := authorizeStoreOperation(t, pool, "backfill")
	request := ReplayRequest{JobID: "source-history", SourceStream: "canonical", Target: scope.Target, Generation: scope.Generation, ProjectionRevision: scope.Revision, Mode: scope.Action, Requester: scope.Operator, AuthorizationID: scope.AuthorizationID, FromHeight: 101, ToHeight: 103, CheckpointHeight: 103, CheckpointHash: lifecycleRaw().BlockHash}
	if err := st.AdmitReplay(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := st.AdmitReplay(ctx, request); err != nil {
		t.Fatal(err)
	}
	changed := request
	changed.ToHeight = 102
	if err := st.AdmitReplay(ctx, changed); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed immutable request %v", err)
	}
	job, err := st.AcquireReplay(ctx, request.JobID, "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	same, err := st.AcquireReplay(ctx, request.JobID, "worker", 30*time.Second)
	if err != nil || same.Fence != job.Fence {
		t.Fatalf("same owner fence %+v %v", same, err)
	}
	if _, err = st.AcquireReplay(ctx, request.JobID, "other", 30*time.Second); !errors.Is(err, ErrFenced) {
		t.Fatalf("competing replay owner %v", err)
	}
	overlap := request
	overlap.JobID = "overlapping-source-history"
	if err = st.AdmitReplay(ctx, overlap); err != nil {
		t.Fatal(err)
	}
	if _, err = st.AcquireReplay(ctx, overlap.JobID, "other", 30*time.Second); err == nil {
		t.Fatal("overlapping replay acquired")
	}
	wrong := job
	wrong.Fence++
	if err = st.PauseReplayLease(ctx, wrong, "source unavailable", 101, 103); !errors.Is(err, ErrFenced) {
		t.Fatalf("stale replay pause %v", err)
	}
	if err = st.PauseReplayLease(ctx, job, "source unavailable", 101, 103); err != nil {
		t.Fatal(err)
	}
	var status, lastError string
	var unavailableFrom, unavailableTo int64
	if err = pool.QueryRow(ctx, `SELECT status,last_error,unavailable_from,unavailable_to FROM indexer.indexer_replay_jobs WHERE job_id=$1`, request.JobID).Scan(&status, &lastError, &unavailableFrom, &unavailableTo); err != nil || status != "paused" || lastError != "source unavailable" || unavailableFrom != 101 || unavailableTo != 103 {
		t.Fatalf("paused replay %s %s %d %d %v", status, lastError, unavailableFrom, unavailableTo, err)
	}
	if _, err = st.AcquireReplay(ctx, request.JobID, "worker", 30*time.Second); !errors.Is(err, ErrFenced) {
		t.Fatalf("paused replay acquired %v", err)
	}
	if err = st.ResumeReplay(ctx, scope, request.JobID); err != nil {
		t.Fatal(err)
	}
	if err = st.ResumeReplay(ctx, scope, request.JobID); !errors.Is(err, ErrFenced) {
		t.Fatalf("running replay resumed %v", err)
	}
	next, err := st.AcquireReplay(ctx, request.JobID, "worker", 30*time.Second)
	if err != nil || next.Fence <= job.Fence {
		t.Fatalf("resumed replay fencing %+v %v", next, err)
	}
	for _, state := range []string{"cancelled", "failed", "paused"} {
		if err = st.TransitionReplay(ctx, scope, request.JobID, state, "operator requested transition", nil, nil); err != nil {
			t.Fatal(err)
		}
		if err = st.ResumeReplay(ctx, scope, request.JobID); err != nil {
			t.Fatal(err)
		}
		if _, err = st.AcquireReplay(ctx, request.JobID, "worker", 30*time.Second); err != nil {
			t.Fatal(err)
		}
	}
	raw := lifecycleRaw()
	digest, err := RawDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	segment := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 101, ToHeight: 101, Blocks: []BlockCompleteness{{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []RawRecord{raw}, Events: []ReplayEvent{{Raw: raw, RawDigest: digest, Kind: "Swap", Timestamp: 1700000000, Payload: []byte(`{"amount":"1"}`)}}}
	if err = st.RecordReplayConflict(ctx, request, segment, "checkpoint differs from admitted source"); err != nil {
		t.Fatal(err)
	}
	if err = st.RecordReplayConflict(ctx, request, segment, "checkpoint differs from admitted source"); err != nil {
		t.Fatal(err)
	}
	var conflicts int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_replay_checkpoint_conflicts`).Scan(&conflicts); err != nil || conflicts != 1 {
		t.Fatalf("immutable conflict evidence %d %v", conflicts, err)
	}
	if err = st.TransitionReplay(ctx, scope, "missing", "cancelled", "", nil, nil); !errors.Is(err, ErrFenced) {
		t.Fatalf("absent replay transitioned %v", err)
	}
	retention := authorizeStoreOperation(t, pool, "retention")
	if eligible, err := st.RetentionEligible(ctx, retention); err != nil || eligible {
		t.Fatalf("unarchived retention %t %v", eligible, err)
	}
	if _, err = st.RetentionEligible(ctx, scope); err == nil {
		t.Fatal("backfill scope authorized retention")
	}
	if err = st.RecordReplayConflict(ctx, request, segment, strings.Repeat("x", 2049)); err == nil {
		t.Fatal("unbounded conflict reason accepted")
	}
	if err = st.TransitionReplay(ctx, scope, request.JobID, "completed", "", nil, nil); err == nil {
		t.Fatal("unapproved transition accepted")
	}
	if err = st.TransitionReplay(ctx, scope, request.JobID, "paused", strings.Repeat("x", 2049), nil, nil); err == nil {
		t.Fatal("unbounded transition accepted")
	}
	if err = st.PauseReplayLease(ctx, ReplayJob{}, "", 0, 1); !errors.Is(err, ErrFenced) {
		t.Fatalf("unbound replay pause %v", err)
	}
	pool.Close()
	for _, call := range []struct {
		name string
		run  func() error
	}{
		{"admission", func() error { return st.AdmitReplay(ctx, request) }},
		{"acquire", func() error { _, e := st.AcquireReplay(ctx, request.JobID, "worker", time.Second); return e }},
		{"resume", func() error { return st.ResumeReplay(ctx, scope, request.JobID) }},
		{"transition", func() error { return st.TransitionReplay(ctx, scope, request.JobID, "paused", "", nil, nil) }},
		{"conflict", func() error { return st.RecordReplayConflict(ctx, request, segment, "source conflict") }},
		{"retention", func() error { _, e := st.RetentionEligible(ctx, retention); return e }},
		{"pause", func() error { return st.PauseReplayLease(ctx, job, "unavailable", 101, 103) }},
		{"checkpoint", func() error {
			return st.BindReplayCheckpoint(ctx, request, BlockCompleteness{Height: 103, Hash: request.CheckpointHash, FetchComplete: true, RequiredDecodeComplete: true})
		}},
		{"commit", func() error {
			return st.CommitReplaySegment(ctx, job, segment, func(context.Context, pgx.Tx, ReplayEvent) (string, error) {
				t.Fatal("closed store projected input")
				return "", nil
			})
		}},
	} {
		t.Run("closed-"+call.name, func(t *testing.T) {
			if err := call.run(); err == nil {
				t.Fatal("closed store accepted operation")
			}
		})
	}
}
