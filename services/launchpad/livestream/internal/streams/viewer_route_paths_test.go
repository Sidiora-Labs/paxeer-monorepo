package streams

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestViewerRoutesBindSessionsToLiveAndPrivatePolicy(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, _ := f.create("viewer-route-policy")
	o, err := f.store.BindProvider(ctx, o.ID, "viewer_route_provider")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := f.store.Observe(ctx, ProviderEvent{EventID: "viewer_route_live", ProviderID: o.ProviderID, State: stateLive, ObservedAt: time.Now().UTC()}); err != nil || outcome != "applied" {
		t.Fatalf("provider observation = %q, %v", outcome, err)
	}
	o, err = f.store.Get(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	viewers, err := NewViewerCounter(f.redis, "livestream", 100000)
	if err != nil {
		t.Fatal(err)
	}
	f.handlers.viewers = viewers
	f.handlers.viewerTokens = &ViewerTokenAuthority{Key: []byte(strings.Repeat("v", 32))}
	post := func(path string, body []byte, headers map[string]string, want int) []byte {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, f.server.URL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		for name, value := range headers {
			request.Header.Set(name, value)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != want {
			t.Fatalf("POST %s = %d %s, %v", path, response.StatusCode, raw, err)
		}
		return raw
	}
	sessionPath := "/v2/streams/" + o.ID + "/viewer-session"
	post(sessionPath, []byte(`{"unknown":true}`), nil, http.StatusBadRequest)
	var first, second struct {
		StreamID  string `json:"streamId"`
		SessionID string `json:"sessionId"`
		Token     string `json:"token"`
	}
	if err := json.Unmarshal(post(sessionPath, []byte(`{}`), nil, http.StatusOK), &first); err != nil || first.StreamID != o.ID || first.SessionID == "" || first.Token == "" {
		t.Fatalf("first viewer session = %+v, %v", first, err)
	}
	heartbeat, err := json.Marshal(map[string]string{"sessionId": first.SessionID, "token": first.Token})
	if err != nil {
		t.Fatal(err)
	}
	path := "/v2/streams/" + o.ID + "/heartbeat"
	if raw := post(path, heartbeat, nil, http.StatusOK); !bytes.Contains(raw, []byte(`"viewerCount":1`)) {
		t.Fatalf("connected viewer count = %s", raw)
	}
	post(path, heartbeat, nil, http.StatusTooManyRequests)
	if err := json.Unmarshal(post(sessionPath, []byte(`{}`), nil, http.StatusOK), &second); err != nil || second.SessionID == first.SessionID {
		t.Fatalf("second viewer session = %+v, %v", second, err)
	}
	frozen, err := json.Marshal(map[string]string{"playbackSessionId": second.SessionID, "viewerToken": second.Token})
	if err != nil {
		t.Fatal(err)
	}
	if raw := post("/v2/streams/"+o.ID+"/viewers:heartbeat", frozen, nil, http.StatusOK); !bytes.Contains(raw, []byte(`"viewerCount":2`)) {
		t.Fatalf("frozen viewer count = %s", raw)
	}
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
	post(sessionPath, []byte(`{}`), nil, http.StatusForbidden)
	post(sessionPath, []byte(`{}`), map[string]string{"Authorization": "Bearer " + f.session(f.key)}, http.StatusOK)
	post(path, heartbeat, nil, http.StatusUnauthorized)
}
