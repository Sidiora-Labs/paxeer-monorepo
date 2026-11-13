package l17

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestExportJobsRequireOwnerValidSelectionAndBoundedQueue(t *testing.T) {
	r := phase2Repository(t)
	ctx := context.Background()
	summary := phase2Commit(t, r, "queued-source", "1", 1)
	if err := r.CommitHistory(ctx, wallet, summary.SnapshotID, "queued-history", []HistoryRecord{portfolioHistoryRecord(summary)}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateExportJob(ctx, wallet, token, summary.SnapshotID, "jsonl", "queue"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	for _, tc := range []struct{ snapshot, format, source string }{{"invalid", "jsonl", "queue"}, {summary.SnapshotID, "xml", "queue"}, {summary.SnapshotID, "jsonl", ""}} {
		if _, err := r.CreateExportJob(ctx, wallet, wallet, tc.snapshot, tc.format, tc.source); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err := r.ExportJob(ctx, uuid.NewString(), wallet, token); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err := r.ExportJob(ctx, uuid.NewString(), wallet, wallet); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := r.CancelExport(ctx, "invalid", wallet, wallet); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := r.CancelExport(ctx, uuid.NewString(), wallet, token); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	for _, source := range []string{"first-job", "second-job"} {
		job, err := r.CreateExportJob(ctx, wallet, wallet, summary.SnapshotID, "jsonl", source)
		if err != nil || job.State != "queued" || !validUUID(job.ID) {
			t.Fatal(job, err)
		}
	}
	if _, err := r.CreateExportJob(ctx, wallet, wallet, summary.SnapshotID, "jsonl", "third-job"); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
}

func TestExportHistoryRequiresPrivateDestinationAndValidRequest(t *testing.T) {
	r := phase2Repository(t)
	ctx := context.Background()
	summary := phase2Commit(t, r, "export-validation", "1", 1)
	dir := t.TempDir()
	publicDir := t.TempDir()
	if err := os.Chmod(publicDir, 0755); err != nil {
		t.Fatal(err)
	}
	objectID := uuid.NewString()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		owner, verified, snapshot, format, id, dir string
		want                                       error
	}{
		{wallet, token, summary.SnapshotID, "jsonl", objectID, dir, ErrUnauthorized},
		{wallet, wallet, "invalid", "jsonl", objectID, dir, ErrInvalid},
		{wallet, wallet, summary.SnapshotID, "xml", objectID, dir, ErrInvalid},
		{wallet, wallet, summary.SnapshotID, "jsonl", "../object", dir, ErrInvalid},
		{wallet, wallet, summary.SnapshotID, "jsonl", objectID, publicDir, ErrInvalid},
	} {
		if _, err := r.exportHistoryObject(ctx, tc.dir, tc.owner, tc.verified, tc.snapshot, tc.format, tc.id, time.Now()); !errors.Is(err, tc.want) {
			t.Fatal(tc, err)
		}
	}
	if _, err := r.ExportHistory(ctx, dir, wallet, token, summary.SnapshotID, "jsonl", time.Now()); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err := r.ExportHistory(ctx, dir, wallet, wallet, summary.SnapshotID, "xml", time.Now()); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if file, _, err := r.OpenExport(ctx, dir, "invalid", wallet, wallet); file != nil || !errors.Is(err, ErrNotFound) {
		t.Fatal(file, err)
	}
	if file, _, err := r.OpenExport(ctx, publicDir, uuid.NewString(), wallet, wallet); file != nil || !errors.Is(err, ErrInvalid) {
		t.Fatal(file, err)
	}
	partial := filepath.Join(dir, objectID+".partial")
	if err := os.Mkdir(partial, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := r.exportHistoryObject(ctx, dir, wallet, wallet, summary.SnapshotID, "jsonl", objectID, time.Now()); err == nil {
		t.Fatal("occupied export object replaced")
	}
	if _, err := os.Stat(partial); err != nil {
		t.Fatal(err)
	}
}
