package streams

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestLegacyHandlerMutationsUseLifecycleAndOwnerDecision(t *testing.T) {
	f := repair57New(t)
	f.handlers.providerReady = func(ctx context.Context) error { return ReadSchemaReady(ctx, f.database, "livestream") }
	_, intent := f.create("legacy-owner-setup")
	request := func(method, path, id, body, token string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", "legacy-owner-stream")
		if id != "" {
			route := chi.NewRouteContext()
			route.URLParams.Add("id", id)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
		}
		w := httptest.NewRecorder()
		switch path {
		case "/create":
			f.handlers.create(w, r)
		case "/start":
			f.handlers.goLive(w, r)
		case "/end":
			f.handlers.end(w, r)
		}
		return w
	}
	createBody := fmt.Sprintf(`{"poolAddress":%q,"title":"Owner stream"}`, intent.Pool)
	if w := request(http.MethodPost, "/create", "", createBody, ""); w.Code != http.StatusForbidden {
		t.Fatalf("unsigned create status = %d: %s", w.Code, w.Body.String())
	}
	if w := request(http.MethodPost, "/create", "", `{"poolAddress":`, f.session(f.key)); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON status = %d: %s", w.Code, w.Body.String())
	}
	w := request(http.MethodPost, "/create", "", createBody, f.session(f.key))
	if w.Code != http.StatusAccepted || !bytes.Contains(w.Body.Bytes(), []byte(`"operation"`)) {
		t.Fatalf("owner create status = %d: %s", w.Code, w.Body.String())
	}
	var id string
	if err := f.database.QueryRow(context.Background(), "SELECT id FROM livestream.stream_operations WHERE idem_key=$1", "legacy-owner-stream").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if w := request(http.MethodPost, "/start", id, `{"revision":"invalid"}`, f.session(f.key)); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid revision status = %d: %s", w.Code, w.Body.String())
	}
	o, err := f.store.BindProvider(context.Background(), id, "legacy_provider")
	if err != nil {
		t.Fatal(err)
	}
	startBody := fmt.Sprintf(`{"revision":%q}`, strconv.FormatInt(o.Revision, 10))
	if w := request(http.MethodPost, "/start", id, startBody, ""); w.Code != http.StatusForbidden {
		t.Fatalf("unsigned start status = %d: %s", w.Code, w.Body.String())
	}
	w = request(http.MethodPost, "/start", id, startBody, f.session(f.key))
	if w.Code != http.StatusAccepted || !bytes.Contains(w.Body.Bytes(), []byte(`"operation"`)) {
		t.Fatalf("owner start status = %d: %s", w.Code, w.Body.String())
	}
	changed, err := f.store.Get(context.Background(), id)
	if err != nil || changed.Intent != "start" || changed.Revision != o.Revision+1 {
		t.Fatalf("start intent = %+v, %v", changed, err)
	}
	if w := request(http.MethodPost, "/start", id, startBody, f.session(f.key)); w.Code != http.StatusConflict {
		t.Fatalf("stale start status = %d: %s", w.Code, w.Body.String())
	}
}

func TestLegacyWebhookAuthenticatesProducerAndObservesLifecycle(t *testing.T) {
	f := repair57New(t)
	o, _ := f.create("legacy-webhook")
	if _, err := f.store.BindProvider(context.Background(), o.ID, "legacy_provider"); err != nil {
		t.Fatal(err)
	}
	f.handlers.studioWebhook = false
	f.handlers.decodeWebhook = DecodeStudioWebhook
	now := time.Now().UTC().Truncate(time.Millisecond)
	raw, err := json.Marshal(map[string]any{
		"id": "legacy_live", "webhookId": "producer_1", "timestamp": now.UnixMilli(),
		"createdAt": now.UnixMilli(), "event": "stream.started", "stream": map[string]string{"id": "legacy_provider"},
	})
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte(f.secret))
	_, _ = mac.Write(raw)
	signature := fmt.Sprintf("t=%d,v1=%s", now.UnixMilli(), hex.EncodeToString(mac.Sum(nil)))
	call := func(body []byte, sig string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/webhooks/livepeer", bytes.NewReader(body))
		r.Header.Set("Livepeer-Signature", sig)
		w := httptest.NewRecorder()
		f.handlers.webhook(w, r)
		return w
	}
	if w := call(raw, "t=1,v1=invalid"); w.Code != http.StatusUnauthorized {
		t.Fatalf("invalid signature status = %d", w.Code)
	}
	w := call(raw, signature)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"applied"`)) {
		t.Fatalf("authenticated webhook status = %d: %s", w.Code, w.Body.String())
	}
	live, err := f.store.Get(context.Background(), o.ID)
	if err != nil || live.State != stateLive {
		t.Fatalf("observed stream = %+v, %v", live, err)
	}
	if w := call(raw, signature); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"duplicate"`)) {
		t.Fatalf("duplicate webhook status = %d: %s", w.Code, w.Body.String())
	}
}

func TestLegacyHeartbeatUsesSignedViewerSession(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, _ := f.create("legacy-viewer-heartbeat")
	o, err := f.store.BindProvider(ctx, o.ID, "legacy_viewer_provider")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := f.store.Observe(ctx, ProviderEvent{EventID: "legacy_viewer_live", ProviderID: o.ProviderID, State: stateLive, ObservedAt: time.Now().UTC()}); err != nil || outcome != "applied" {
		t.Fatalf("live observation = %q, %v", outcome, err)
	}
	viewers, err := NewViewerCounter(f.redis, "livestream", 100000)
	if err != nil {
		t.Fatal(err)
	}
	f.handlers.viewers = viewers
	if err := viewers.Initialize(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	token := f.session(f.key)
	rawSession, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatal(err)
	}
	var session repair57Session
	if err := json.Unmarshal(rawSession, &session); err != nil {
		t.Fatal(err)
	}
	call := func(viewerID string) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(map[string]string{"viewerId": viewerID})
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/streams/"+o.ID+"/heartbeat", bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		route := chi.NewRouteContext()
		route.URLParams.Add("id", o.ID)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
		w := httptest.NewRecorder()
		f.handlers.heartbeat(w, r)
		return w
	}
	if w := call("different-viewer"); w.Code != http.StatusForbidden {
		t.Fatalf("viewer substitution = %d %s", w.Code, w.Body.String())
	}
	w := call(session.SessionID)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"viewerCount":1`)) {
		t.Fatalf("signed viewer heartbeat = %d %s", w.Code, w.Body.String())
	}
}
