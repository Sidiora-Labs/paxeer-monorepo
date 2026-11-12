package streams

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer"
)

func TestRankedDirectoryRequiresPlaybackAndMarketProjection(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, _ := f.create("ranked-directory")
	o, err := f.store.BindProvider(ctx, o.ID, "ranked_provider")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := f.store.Observe(ctx, ProviderEvent{EventID: "ranked_live_event", ProviderID: o.ProviderID, State: stateLive, ObservedAt: time.Now()}); err != nil || outcome != "applied" {
		t.Fatalf("ranked provider observation = %s, %v", outcome, err)
	}
	get := func(path string, want int) []byte {
		t.Helper()
		response, err := http.Get(f.server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != want {
			t.Fatalf("ranked read %s = %d %s, %v", path, response.StatusCode, raw, err)
		}
		return raw
	}
	if raw := get("/v2/streams/live?sort=viewers", http.StatusServiceUnavailable); !bytes.Contains(raw, []byte(`"code"`)) {
		t.Fatalf("unresolved playback = %s", raw)
	}
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/stream/ranked_provider" {
			t.Errorf("provider lookup = %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":"ranked_provider","playbackId":"playback_1","isActive":true,"playbackPolicy":{"type":"webhook","webhookId":"playback_hook","webhookContext":{"operationId":%q},"refreshInterval":1}}`, o.ID)
	}))
	defer providerServer.Close()
	f.handlers.playback = ProviderPlayback(livepeer.New("fixture-key", providerServer.URL, nil, livepeer.WithWebhookPlayback("playback_hook")), nil)
	if raw := get("/v2/streams/live?sort=mcap", http.StatusServiceUnavailable); !bytes.Contains(raw, []byte(`"code"`)) {
		t.Fatalf("missing market projection = %s", raw)
	}
	f.handlers.marketCaps = &MarketCapReader{Pool: f.database}
	if raw := get("/v2/streams/live?sort=mcap", http.StatusServiceUnavailable); !bytes.Contains(raw, []byte(`"code"`)) {
		t.Fatalf("unavailable market projection = %s", raw)
	}
	for _, path := range []string{
		"/v2/streams/live?sort=viewers&limit=00",
		"/v2/streams/live?sort=viewers&cursor=invalid",
		"/v2/streams/live?sort=viewers&limit=101",
	} {
		if raw := get(path, http.StatusUnprocessableEntity); !bytes.Contains(raw, []byte(`"code"`)) {
			t.Fatalf("invalid rank request = %s", raw)
		}
	}
	if raw := get("/v2/streams/live?sort=viewers", http.StatusOK); !bytes.Contains(raw, []byte(o.ID)) {
		t.Fatalf("ranked live operation = %s", raw)
	}
	if _, err := f.handlers.marketCaps.MarketCaps(ctx, []string{"invalid"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid market pool = %v", err)
	}
	if caps, err := f.handlers.marketCaps.MarketCaps(ctx, nil); err != nil || len(caps) != 0 {
		t.Fatalf("empty market projection = %v, %v", caps, err)
	}
}
