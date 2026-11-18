package app

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReadinessLogsDatabaseCause(t *testing.T) {
	database, _ := platformDatabase(t)
	pool, err := pgxpool.New(context.Background(), database.Runtimes["livestream_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	check := loggedReadiness(logger, "database", func(ctx context.Context) error {
		_, err := pool.Exec(ctx, "SELECT * FROM pg_authid")
		return err
	})
	router := sharedhttp.NewRouter(sharedhttp.ServerOptions{Health: sharedhttp.HealthDeps{DB: check}})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"database":"failed"`) {
		t.Fatalf("readiness response: %d %s", response.Code, response.Body.String())
	}
	if !strings.Contains(logs.String(), `"dependency":"database"`) || !strings.Contains(logs.String(), "SQLSTATE 42501") || !strings.Contains(logs.String(), "pg_authid") {
		t.Fatalf("readiness cause: %s", logs.String())
	}
	if strings.Contains(response.Body.String(), "42501") || strings.Contains(response.Body.String(), "pg_authid") {
		t.Fatalf("readiness exposed database diagnostics: %s", response.Body.String())
	}
}
