package streams

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer"
)

func TestProvisionScheduleAdoptsCorrelatedStreamOnSweep(t *testing.T) {
	f := repair57New(t)
	f.transit = repair57Vault(t)
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, f.store.RotationSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	o, _ := f.create("scheduled-provision")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /stream":
			_, _ = fmt.Fprintf(w, `[{"id":"scheduled_provider","playbackId":"playback_1","streamKey":"scheduled_key","name":%q}]`, livepeer.CorrelationName(o.ID))
		case "PATCH /stream/scheduled_provider":
			w.WriteHeader(http.StatusNoContent)
		case "GET /stream/scheduled_provider":
			_, _ = fmt.Fprintf(w, `{"id":"scheduled_provider","playbackId":"playback_1","streamKey":"scheduled_key","playbackPolicy":{"type":"webhook","webhookId":"playback_hook","webhookContext":{"operationId":%q},"refreshInterval":1}}`, o.ID)
		default:
			t.Errorf("scheduled provider route = %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	provider := livepeer.New("fixture-key", server.URL, nil, livepeer.WithWebhookPlayback("playback_hook"))
	worker := &ProvisionWorker{Store: f.store, Provider: provider, Credentials: &EncryptedCredentials{Store: f.store, Transit: f.transit, IngestEndpoint: "rtmps://rtmp.livepeer.studio/live"}, Restrictions: &RestrictionReader{Pool: f.database}}
	runCtx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	if err := worker.Run(runCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("scheduled provisioning stop = %v", err)
	}
	current, err := f.store.Get(ctx, o.ID)
	if err != nil || current.State != stateReady || current.ProviderID != "scheduled_provider" {
		t.Fatalf("scheduled provider adoption = %+v, %v", current, err)
	}
}
