package streams

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer"
)

func TestReconcileLoopObservesActiveProviderOnScheduledSweep(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, _ := f.create("scheduled-reconciliation")
	o, err := f.store.BindProvider(ctx, o.ID, "scheduled_provider")
	if err != nil {
		t.Fatal(err)
	}
	var reads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/stream/scheduled_provider" {
			t.Errorf("unexpected provider request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		reads.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"scheduled_provider","playbackId":"playback_1","isActive":true}`))
	}))
	defer server.Close()
	worker := &ReconcileWorker{Store: f.store, Provider: livepeer.New("fixture-key", server.URL, nil), Restrictions: &RestrictionReader{Pool: f.database}}
	runCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	if err := worker.Run(runCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("scheduled reconciliation stop = %v", err)
	}
	live, err := f.store.Get(ctx, o.ID)
	if err != nil || live.State != stateLive || reads.Load() < 1 {
		t.Fatalf("scheduled provider state = %+v, reads=%d, %v", live, reads.Load(), err)
	}
}
