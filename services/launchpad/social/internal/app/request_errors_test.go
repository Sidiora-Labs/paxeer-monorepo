package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRequestErrorsLogDatabaseCause(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://social:unused@127.0.0.1:1/social")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	for _, path := range []string{
		"/internal/v2/social/ranking-visibility",
		"/v2/pools/" + addr(1) + "/comments",
		"/v2/social/pool/" + addr(1) + "/comments",
	} {
		t.Run(path, func(t *testing.T) {
			logs.Reset()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Authorization", "Bearer private-session")
			response := httptest.NewRecorder()
			if strings.HasPrefix(path, "/internal/") {
				(&socialConnectedRuntime{pool: pool}).authorityRoutes(response, request)
			} else {
				(&socialComments{pool: pool}).reads(http.NotFoundHandler()).ServeHTTP(response, request)
			}
			if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "closed pool") {
				t.Fatalf("dependency response = %d %s", response.Code, response.Body.String())
			}
			var logged struct {
				Method string `json:"method"`
				Path   string `json:"path"`
				Error  string `json:"error"`
			}
			if err := json.Unmarshal(logs.Bytes(), &logged); err != nil || logged.Method != http.MethodGet || logged.Path != path || !strings.Contains(logged.Error, "closed pool") || strings.Contains(logs.String(), "private-session") {
				t.Fatalf("database error context = %s: %v", logs.String(), err)
			}
		})
	}
}
