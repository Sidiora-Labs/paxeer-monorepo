package streams

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
)

func TestReadAndWebhookErrorsLogDatabaseCause(t *testing.T) {
	store := fixture(t)
	ctx := context.Background()
	if _, err := store.pool.Exec(ctx, "ALTER TABLE "+store.schema+".stream_operations RENAME TO stored_operations"); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	secret := strings.Repeat("s", 32)
	now := time.Now().UTC().Truncate(time.Millisecond)
	handler := New(Deps{Lifecycle: store, StudioWebhook: true, WebhookSecret: secret, Clock: func() time.Time { return now }, Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()
	for _, endpoint := range []struct {
		method, path string
		body         []byte
	}{
		{method: http.MethodGet, path: "/v2/streams"},
		{method: http.MethodPost, path: "/webhooks/livepeer", body: studioRecoveryStudioBody(t, "logged_observation", "streamid", "stream.started", now, now)},
	} {
		t.Run(endpoint.path, func(t *testing.T) {
			logs.Reset()
			request, err := http.NewRequest(endpoint.method, server.URL+endpoint.path, bytes.NewReader(endpoint.body))
			if err != nil {
				t.Fatal(err)
			}
			if endpoint.body != nil {
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Livepeer-Signature", studioRecoveryStudioSignature(endpoint.body, secret, now))
			}
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			raw, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			if readErr != nil || closeErr != nil || response.StatusCode != http.StatusServiceUnavailable || v2.ValidateJSON("Error", raw) != nil {
				t.Fatalf("dependency error: status=%d body=%s read=%v close=%v", response.StatusCode, raw, readErr, closeErr)
			}
			if !strings.Contains(logs.String(), "SQLSTATE 42P01") || !strings.Contains(logs.String(), "stream_operations") {
				t.Fatalf("database cause missing: %s", logs.String())
			}
			if bytes.Contains(raw, []byte("42P01")) || bytes.Contains(raw, []byte(store.schema)) || strings.Contains(logs.String(), secret) {
				t.Fatalf("dependency diagnostics escaped their boundary: response=%s", raw)
			}
		})
	}
}
