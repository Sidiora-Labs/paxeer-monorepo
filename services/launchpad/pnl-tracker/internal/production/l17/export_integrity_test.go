package l17

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExportDownloadRequiresBoundManifestAndPrivateObject(t *testing.T) {
	ctx := context.Background()
	repository := phase2Repository(t)
	summary := phase2Commit(t, repository, "download-source", "1", 1)
	if err := repository.CommitHistory(ctx, wallet, summary.SnapshotID, "download-history", []HistoryRecord{portfolioHistoryRecord(summary)}); err != nil {
		t.Fatal(err)
	}
	job, err := repository.CreateExportJob(ctx, wallet, wallet, summary.SnapshotID, "jsonl", "download-export")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err = os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	worker := ExportWorker{Repository: repository, Directory: directory, Owner: "download-worker"}
	if err = worker.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	complete, err := repository.ExportJob(ctx, job.ID, wallet, wallet)
	if err != nil || complete.Manifest == nil {
		t.Fatal(complete, err)
	}
	original := *complete.Manifest
	originalRaw, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	object := filepath.Join(directory, original.Object)
	contents, err := os.ReadFile(object)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*ExportManifest)
		raw    []byte
		file   string
		want   error
	}{
		{name: "manifest type", raw: []byte(`[]`)},
		{name: "format binding", change: func(m *ExportManifest) { m.Format = "csv" }, want: ErrInvalid},
		{name: "snapshot binding", change: func(m *ExportManifest) { m.SnapshotID = "invalid" }, want: ErrInvalid},
		{name: "row bound", change: func(m *ExportManifest) { m.Rows = 1000001 }, want: ErrInvalid},
		{name: "digest length", change: func(m *ExportManifest) { m.SHA256 = "short" }, want: ErrInvalid},
		{name: "object path", change: func(m *ExportManifest) { m.Object = "../" + m.Object }, want: ErrInvalid},
		{name: "object identity", change: func(m *ExportManifest) { m.Object = "other.jsonl" }, want: ErrInvalid},
		{name: "missing object", file: "missing", want: os.ErrNotExist},
		{name: "directory object", file: "directory", want: ErrInvalid},
		{name: "symbolic object", file: "symlink", want: ErrInvalid},
		{name: "content digest", file: "changed", want: ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := original
			if tc.change != nil {
				tc.change(&manifest)
			}
			raw, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if tc.raw != nil {
				raw = tc.raw
			}
			if _, err = repository.Pool.Exec(ctx, "UPDATE portfolio_export_jobs SET manifest=$2 WHERE id=$1", job.ID, raw); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := repository.Pool.Exec(ctx, "UPDATE portfolio_export_jobs SET manifest=$2 WHERE id=$1", job.ID, originalRaw); err != nil {
					t.Error(err)
				}
			}()
			if tc.file != "" {
				if err = os.Remove(object); err != nil {
					t.Fatal(err)
				}
				switch tc.file {
				case "directory":
					err = os.Mkdir(object, 0700)
				case "symlink":
					err = os.Symlink(filepath.Join(directory, "target"), object)
				case "changed":
					err = os.WriteFile(object, []byte("changed"), 0600)
				}
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := os.RemoveAll(object); err != nil {
						t.Error(err)
					}
					if err := os.WriteFile(object, contents, 0600); err != nil {
						t.Error(err)
					}
				}()
			}
			file, _, err := repository.OpenExport(ctx, directory, job.ID, wallet, wallet)
			if file != nil {
				_ = file.Close()
				t.Fatal("unbound object opened")
			}
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatal(err, tc.want)
			}
		})
	}
	file, manifest, err := repository.OpenExport(ctx, directory, job.ID, wallet, wallet)
	if err != nil || manifest.Rows != 1 {
		t.Fatal(manifest, err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{token, "bad"} {
		if file, _, err := repository.OpenExport(ctx, directory, job.ID, owner, wallet); err == nil || file != nil {
			t.Fatal(file, err)
		}
	}
	if err = repository.CancelExport(ctx, job.ID, wallet, wallet); err != nil {
		t.Fatal(err)
	}
	if err = worker.CleanupOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(object); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if _, err = repository.CreateExportJob(ctx, wallet, wallet, summary.SnapshotID, "csv", "download-export"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err = repository.RevokeCard(ctx, "invalid", wallet, wallet, time.Now()); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
