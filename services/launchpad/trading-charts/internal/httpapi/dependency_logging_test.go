package httpapi

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/go-chi/chi/v5"
)

func TestChartDependencyFailuresLogTheirCause(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	pool := historyPostgres(t)
	pool.Close()
	cache, err := NewHistoryCache(nil, PostgresHistoryRevision{Pool: pool, Schema: "candles"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, cause string
		handler           http.Handler
	}{
		{"symbols", "/symbols?symbol=" + udfPool, "closed pool", SymbolsHandler{Canonical: &CanonicalPoolMetadataSource{Pool: pool, Chain: &chainread.Reader{}}}},
		{"history", historyURL("from=0&to=120"), "closed pool", ProductionHistoryHandler{Cache: cache, Algorithm: "qualified-exact-v2"}},
		{"development history", historyURL("from=0&to=120"), "closed pool", DevelopmentHistoryHandler{Source: PostgresHistorySource{Pool: pool}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			response := httptest.NewRecorder()
			tc.handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if response.Code != 503 || !strings.Contains(response.Body.String(), "DEPENDENCY_UNAVAILABLE") || strings.Contains(response.Body.String(), tc.cause) || !strings.Contains(logs.String(), tc.cause) {
				t.Fatalf("dependency response: %d %s; log: %s", response.Code, response.Body.String(), logs.String())
			}
		})
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	origin := "http://" + listener.Addr().String()
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	if err = RegisterReplayGateway(router, origin); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v2/snapshots", "/v2/stream"} {
		logs.Reset()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil).WithContext(context.Background()))
		if response.Code != 503 || !strings.Contains(response.Body.String(), "DEPENDENCY_UNAVAILABLE") || !strings.Contains(logs.String(), "connection refused") {
			t.Fatalf("replay failure: %d %s; log: %s", response.Code, response.Body.String(), logs.String())
		}
	}
}
