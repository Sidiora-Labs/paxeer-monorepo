package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store"
	"github.com/google/uuid"
)

func TestRefreshRefusalStepReaders(t *testing.T) {
	for _, tc := range []struct {
		name        string
		step        string
		unavailable bool
	}{
		{"evidence unavailable", "evidence", true},
		{"evidence transport", "evidence", false},
		{"visibility unavailable", "visibility", true},
		{"visibility transport", "visibility", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := publicationRecoveryNewFixture(t)
			runtime := f.app.native
			ctx := context.Background()
			if tc.step == "evidence" {
				if tc.unavailable {
					var err error
					runtime.reader.Evidence, err = store.NewHTTPCanonicalEvidence(f.server.URL+"/internal/ranking-evidence", f.client)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					f.evidence.Close()
				}
			} else if tc.unavailable {
				f.authority.Store(false)
			} else {
				f.evidence.Close()
				runtime.visibility.endpoint = f.evidence.URL
				runtime.visibility.client = f.evidence.Client()
			}
			tx, err := f.app.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			_, err = runtime.reader.ReadRankingUniverseTxDetailed(ctx, tx, uuid.NewString(), f.cutoff)
			step, class, ok := ranker.RefreshRefusalDetails(err)
			if !ok || step != tc.step || class != "unavailable" {
				t.Fatalf("universe refusal = %q/%q/%t: %v", step, class, ok, err)
			}
			var refusal *ranker.RefreshRefusal
			if !errors.As(err, &refusal) || refusal.Cause == nil || errors.Unwrap(err) != refusal.Cause {
				t.Fatalf("universe refusal has no original cause: %v", err)
			}
			for cause := refusal.Cause; cause != nil; cause = errors.Unwrap(cause) {
				if !errors.Is(err, cause) {
					t.Fatalf("universe refusal lost cause %T", cause)
				}
			}
			for _, target := range []error{ranker.ErrUnavailable, ranker.ErrSuperseded, context.Canceled, context.DeadlineExceeded} {
				if errors.Is(err, target) != errors.Is(refusal.Cause, target) {
					t.Fatalf("universe refusal changed errors.Is for %v", target)
				}
			}
			if errors.Is(err, ranker.ErrUnavailable) != tc.unavailable {
				t.Fatalf("universe unavailable = %t, want %t", errors.Is(err, ranker.ErrUnavailable), tc.unavailable)
			}
			if tc.unavailable && refusal.Cause != ranker.ErrUnavailable {
				t.Fatalf("unavailable cause = %v", refusal.Cause)
			}
			checkLegacy := func(err error) {
				t.Helper()
				if err == nil {
					t.Fatal("legacy read accepted unavailable input")
				}
				if _, _, ok := ranker.RefreshRefusalDetails(err); ok {
					t.Fatal("legacy read returned a structured refusal")
				}
				if tc.unavailable {
					if err != ranker.ErrUnavailable {
						t.Fatalf("legacy unavailable error = %v", err)
					}
				} else {
					var transport *url.Error
					if errors.Is(err, ranker.ErrUnavailable) || !errors.As(err, &transport) {
						t.Fatalf("legacy transport error = %T: %v", err, err)
					}
				}
			}
			_, legacyErr := runtime.reader.ReadRankingUniverseTx(ctx, tx, uuid.NewString(), f.cutoff)
			checkLegacy(legacyErr)
			source, sourceErr := store.NewFrozenInputs(f.app.pool, runtime.reader)
			if sourceErr != nil {
				t.Fatal(sourceErr)
			}
			_, legacyErr = source.Freeze(ctx, uuid.NewString(), f.cutoff)
			checkLegacy(legacyErr)
			snapshots, refreshErr := runtime.coordinator.Refresh(ctx, uuid.NewString(), false)
			step, class, ok = ranker.RefreshRefusalDetails(refreshErr)
			if len(snapshots) != 0 || !ok || step != tc.step || class != "unavailable" {
				t.Fatalf("refresh refusal = %q/%q/%t, snapshots=%d: %v", step, class, ok, len(snapshots), refreshErr)
			}
			var output bytes.Buffer
			runtime.logger = slog.New(slog.NewJSONHandler(&output, nil))
			runtime.reportRefresh(false, refreshErr)
			runtime.reportRefresh(false, refreshErr)
			var record map[string]any
			if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil {
				t.Fatalf("refresh log: %v: %s", err, output.String())
			}
			if record["msg"] != "ranking refresh refused" || record["step"] != tc.step || record["class"] != "unavailable" || record["run"] != "all" || record["level"] != "ERROR" {
				t.Fatalf("refresh log fields = %v", record)
			}
			if len(record) != 6 {
				t.Fatalf("refresh log includes unexpected fields: %v", record)
			}
		})
	}
}

func TestRefreshRefusalStepVisibilityCancellation(t *testing.T) {
	f := publicationRecoveryNewFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	tx, err := f.app.pool.Begin(ctx)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	cancel()
	_, err = f.app.native.reader.ReadRankingUniverseTxDetailed(ctx, tx, uuid.NewString(), f.cutoff)
	step, class, ok := ranker.RefreshRefusalDetails(err)
	if !ok || step != "visibility" || class != "unavailable" || !errors.Is(err, context.Canceled) || errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("canceled visibility refusal = %q/%q/%t: %v", step, class, ok, err)
	}
	_, err = f.app.native.reader.ReadRankingUniverseTx(ctx, tx, uuid.NewString(), f.cutoff)
	if !errors.Is(err, context.Canceled) || errors.Is(err, ranker.ErrUnavailable) {
		t.Fatalf("legacy canceled visibility error = %v", err)
	}
}
