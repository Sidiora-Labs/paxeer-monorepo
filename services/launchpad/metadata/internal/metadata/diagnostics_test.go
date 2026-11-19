package metadata

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestUnavailableResponseLogsDatabaseCause(t *testing.T) {
	var output bytes.Buffer
	prior := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(prior) })
	cause := &pgconn.PgError{Code: "42501", Message: "private-password", Detail: "private-body", Where: "private-host"}
	response := httptest.NewRecorder()
	v2Failure(response, fmt.Errorf("private-dsn: %w", cause))
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"code":"unavailable"`) {
		t.Fatalf("unavailable response = %d %s", response.Code, response.Body.String())
	}
	if !strings.Contains(output.String(), `"sqlstate":"42501"`) || !strings.Contains(output.String(), `"cause_type":"*pgconn.PgError"`) {
		t.Fatalf("database diagnostic = %s", output.String())
	}
	for _, secret := range []string{"private-password", "private-body", "private-host", "private-dsn"} {
		if strings.Contains(output.String()+response.Body.String(), secret) {
			t.Fatalf("diagnostic exposed %s", secret)
		}
	}
}

func TestDependencyDiagnosticsPreserveClassification(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{fmt.Errorf("private-value: %w", context.DeadlineExceeded), "deadline_exceeded"},
		{errors.Join(errors.New("private-value"), context.Canceled), "canceled"},
		{&MediaReadinessError{Part: MediaPartObjects, Err: ErrUnrepresentable}, "unrepresentable_state"},
	} {
		var output bytes.Buffer
		LogFailure(slog.New(slog.NewJSONHandler(&output, nil)), "metadata dependency unavailable", tc.err)
		if !strings.Contains(output.String(), tc.want) || strings.Contains(output.String(), "private-value") {
			t.Fatalf("dependency diagnostic = %s", output.String())
		}
	}
}

func TestLegacyUnavailableResponseLogsCause(t *testing.T) {
	var output bytes.Buffer
	prior := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(prior) })
	router := chi.NewRouter()
	NewProductionLegacyReads(ConnectedMedia{}, "https://metadata.example").RegisterRoutes(router)
	for _, path := range []string{"/metadata/0x" + strings.Repeat("1", 40), "/metadata/batch?addresses=0x" + strings.Repeat("1", 40), "/logo/0x" + strings.Repeat("1", 40) + ".png"} {
		output.Reset()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusServiceUnavailable || !strings.Contains(output.String(), `"cause":"unrepresentable_state"`) {
			t.Fatalf("legacy unavailable %s = %d, diagnostic %s", path, response.Code, output.String())
		}
	}
}
