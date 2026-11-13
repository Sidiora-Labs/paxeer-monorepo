package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/go-chi/chi/v5"
)

func TestRepair_22_39_372_development_failures_log_route_and_class_without_request_values(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	srv, pool := hsdDevServer(t, false)
	if _, err := pool.Exec(context.Background(), `ALTER TABLE indexer.pools RENAME TO pools_unavailable`); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"/trades", "/candles?interval=1d&from=2025-01-01T00%3A00%3A00.000Z&to=2025-01-02T00%3A00%3A00.000Z"} {
		hsdExpectError(t, srv, "GET", "/v2/pools/"+hsdDevPool+suffix, 503, "DEPENDENCY_UNAVAILABLE")
	}
	// Exercise the real registered market handler with no configured store.
	router := newRepair372MarketRouter(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/v2/pools/"+hsdDevPool, nil)
	request.Header.Set("Authorization", "Bearer never-log-this-request-secret")
	router.ServeHTTP(recorder, request)
	if recorder.Code != 503 {
		t.Fatalf("unconfigured market = %d %s", recorder.Code, recorder.Body.String())
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("want one log per 503, got %s", logs.String())
	}
	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		if entry["route"] == nil || entry["error_class"] == nil {
			t.Fatalf("missing class/route: %s", line)
		}
		if i < 2 && entry["error_class"] != "postgres_42P01" {
			t.Fatalf("SQL cause hidden: %s", line)
		}
		if strings.Contains(line, hsdDevPool) || strings.Contains(line, "2025-01") || strings.Contains(line, "never-log") || strings.Contains(line, "Bearer") {
			t.Fatalf("request value logged: %s", line)
		}
	}
}

func newRepair372MarketRouter(t *testing.T) *chi.Mux {
	t.Helper()
	router := chi.NewRouter()
	if err := rest.RegisterDevelopmentMarketReads(context.Background(), router, rest.DevelopmentMarketOptions{}); err != nil {
		t.Fatal(err)
	}
	return router
}
