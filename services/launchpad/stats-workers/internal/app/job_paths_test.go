package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	sharedprocess "github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
)

func TestRuntimeMaintenanceAndReadObservation(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	st := store.New(db)
	cursor := ""
	more, err := RefreshLegacyWindows(ctx, st, time.Now().Unix(), &cursor)
	if err != nil || more || cursor != "" {
		t.Fatalf("empty legacy sweep: %v %v %q", more, err, cursor)
	}
	if _, err := RefreshLegacyWindows(ctx, nil, 1, &cursor); err == nil {
		t.Fatal("missing store accepted")
	}
	if _, err := RefreshLegacyWindows(ctx, st, 1, nil); err == nil {
		t.Fatal("missing cursor accepted")
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	jobCtx, stop := context.WithCancel(ctx)
	calls := 0
	err = periodicStatsJob(jobCtx, time.Millisecond, logger, "precompute", func(context.Context) error {
		calls++
		if calls == 1 {
			return errors.New("temporary")
		}
		stop()
		return nil
	})
	if !errors.Is(err, context.Canceled) || calls != 2 {
		t.Fatalf("periodic job: calls=%d err=%v", calls, err)
	}
	loopCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	a := &App{logger: logger}
	loopCalls := 0
	a.loop(loopCtx, time.Millisecond, "maintenance", func(context.Context) error {
		loopCalls++
		if loopCalls == 2 {
			cancel()
		}
		return errors.New("retryable")
	})
	if loopCalls != 2 {
		t.Fatalf("maintenance calls: %d", loopCalls)
	}
	metrics, err := telemetry.New("statsd", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	tracker := sharedprocess.NewTracker()
	if err := tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	handler := runtimeHandler(tracker, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/invalid" {
			w.WriteHeader(422)
			return
		}
		w.WriteHeader(200)
	}), metrics)
	for _, tc := range []struct {
		path   string
		status int
	}{{"/market", 200}, {"/invalid", 422}, {"/health", 200}} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if response.Code != tc.status {
			t.Fatalf("%s: %d", tc.path, response.Code)
		}
	}
}
