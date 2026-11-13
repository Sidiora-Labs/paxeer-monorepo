package streams

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer"
)

func TestPrivatePlaybackRequiresOwnerAndCurrentPrivacy(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, _ := f.create("private-playback")
	o, err := f.store.BindProvider(ctx, o.ID, "private_provider")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := f.store.Observe(ctx, ProviderEvent{EventID: "private_live", ProviderID: o.ProviderID, State: stateLive, ObservedAt: time.Now().UTC()}); err != nil || outcome != "applied" {
		t.Fatalf("provider observation = %s, %v", outcome, err)
	}
	o, err = f.store.Get(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/stream/private_provider" {
			t.Errorf("provider lookup = %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":"private_provider","playbackId":"playback_1","isActive":true,"playbackPolicy":{"type":"webhook","webhookId":"playback_hook","webhookContext":{"operationId":%q},"refreshInterval":1}}`, o.ID)
	}))
	defer providerServer.Close()
	f.handlers.playback = ProviderPlayback(livepeer.New("fixture-key", providerServer.URL, nil, livepeer.WithWebhookPlayback("playback_hook")), nil)
	path := "/v2/streams/" + o.ID + "/private-playback"
	owner := map[string]string{"Authorization": "Bearer " + f.session(f.key)}
	denied := func(headers map[string]string) {
		t.Helper()
		request, err := http.NewRequest(http.MethodGet, f.server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		for name, value := range headers {
			request.Header.Set(name, value)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != http.StatusForbidden {
			t.Fatalf("private playback refusal = %d %s, %v", response.StatusCode, body, err)
		}
	}
	denied(owner)
	tx, err := f.database.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.SetPlaybackPrivacyTx(ctx, tx, o, true); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	denied(nil)
	raw := f.exchange(http.MethodGet, path, nil, owner, http.StatusOK)
	if !bytes.Contains(raw, []byte(`"status":"live"`)) || !bytes.Contains(raw, []byte("playback_1")) {
		t.Fatalf("private owner playback = %s", raw)
	}
	response, err := http.Get(f.server.URL + "/v2/streams/" + o.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	public, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("public stream = %d %s, %v", response.StatusCode, public, err)
	}
	if bytes.Contains(public, []byte("playback_1")) {
		t.Fatalf("private playback disclosed publicly: %s", public)
	}
	providerServer.Close()
	request, err := http.NewRequest(http.MethodGet, f.server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+f.session(f.key))
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unavailable private playback provider = %d", response.StatusCode)
	}
	if _, err := f.database.Exec(ctx, "DROP TABLE livestream.stream_playback_privacy"); err != nil {
		t.Fatal(err)
	}
	request, err = http.NewRequest(http.MethodGet, f.server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+f.session(f.key))
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unavailable privacy authority = %d", response.StatusCode)
	}
}
