package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestAuthorityReadFailurePreservesAndLogsDatabaseCause(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	authority := &networkAuthority{pool: pool, engine: &capabilities.PreflightEngine{}, config: networkAuthorityConfig{SourceSchema: "indexer"}}
	_, err := authority.head(context.Background())
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != "42P01" || !strings.Contains(err.Error(), "read committed network head") {
		t.Fatalf("committed head cause: %v", err)
	}
	var output bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	defer slog.SetDefault(original)
	admission, err := rest.NewAdmission(10, 1, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	authority.Register(router, admission)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/network/authority", nil))
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(output.String(), "42P01") || !strings.Contains(output.String(), response.Header().Get("X-Request-ID")) || !strings.Contains(response.Body.String(), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("authority response: %d %s; diagnostic: %s", response.Code, response.Body, &output)
	}
	if strings.Contains(response.Body.String(), "indexer_committed_heads_v2") || strings.Contains(response.Body.String(), "42P01") {
		t.Fatal("database cause appeared in public error")
	}
}
