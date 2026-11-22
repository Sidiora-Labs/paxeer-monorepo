package interactions

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

func TestDependencyResponsesLogDatabaseCause(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://social:unused@127.0.0.1:1/social")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	cause := pool.Ping(context.Background())
	if cause == nil {
		t.Fatal("closed database accepted a probe")
	}
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	r := httptest.NewRequest(http.MethodGet, "/v2/social", nil)
	r.Header.Set("Authorization", "Bearer private-session")
	for _, tc := range []struct {
		name    string
		respond func(http.ResponseWriter)
	}{
		{"interaction", func(w http.ResponseWriter) { httpError(w, cause) }},
		{"follow", func(w http.ResponseWriter) { contractError(w, r, cause) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			response := httptest.NewRecorder()
			tc.respond(response)
			var logged struct {
				Error string `json:"error"`
			}
			if response.Code != http.StatusServiceUnavailable || json.Unmarshal(logs.Bytes(), &logged) != nil || logged.Error != cause.Error() || strings.Contains(logs.String(), "private-session") {
				t.Fatalf("dependency response = %d, log = %s", response.Code, logs.String())
			}
		})
	}
}
