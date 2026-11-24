package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOperationalReadsLogDatabaseCausesAndKeepResponsesPrivate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	database := coordinatortest.Bootstrap(t, os.Getenv("L06_POSTGRES_DSN"), "indexer_runtime")
	runtime := database.Runtimes["indexer_runtime"]
	pool, err := pgxpool.New(ctx, runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	admin, err := pgx.Connect(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router := chi.NewRouter()
	operations := AuditedDeadLetters{Logger: logger, DB: pool, Verifier: auth.ServiceAssertionVerifier{Audience: "indexer"}, DeliverySchema: "indexer", SecuritySchema: "security", Audience: "indexer", PolicyVersion: "v1"}
	RegisterStatus(router, StatusDeps{Logger: logger, Store: store.New(pool), ChainID: 125, Mode: StatusLegacy, StartTime: time.Now(), DeadLetters: operations})
	request := func(path string, want int) string {
		t.Helper()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != want {
			t.Fatalf("%s: %d: %s", path, rr.Code, rr.Body.String())
		}
		for _, private := range []string{runtime.Role, "42501", "pool closed", "permission denied"} {
			if strings.Contains(rr.Body.String(), private) {
				t.Fatalf("private database cause in response: %s", rr.Body.String())
			}
		}
		return rr.Body.String()
	}
	assertLog := func(operation, cause, code string) {
		t.Helper()
		var record map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &record); err != nil {
			t.Fatalf("operational log: %s: %v", logs.String(), err)
		}
		if record["operation"] != operation || record["cause"] != cause || code != "" && record["sqlstate"] != code {
			t.Fatalf("operational cause: %s", logs.String())
		}
		logs.Reset()
	}
	request("/status", http.StatusOK)
	request("/status/dead-letter", http.StatusUnauthorized)
	var decisions int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM security.authorization_decisions WHERE scope='inspect' AND NOT allowed`).Scan(&decisions); err != nil || decisions != 1 {
		t.Fatalf("runtime audit decision: %d: %v", decisions, err)
	}
	role := pgx.Identifier{runtime.Role}.Sanitize()
	if _, err := admin.Exec(ctx, "REVOKE SELECT ON indexer.pools FROM "+role); err != nil {
		t.Fatal(err)
	}
	body := request("/status", http.StatusServiceUnavailable)
	if !strings.Contains(body, `"totalPoolsIndexed":null`) || !strings.Contains(body, `"totalSwapsIndexed":0`) {
		t.Fatalf("failed count response: %s", body)
	}
	assertLog("poolCount", "database_error", "42501")
	if _, err := admin.Exec(ctx, "GRANT SELECT ON indexer.pools TO "+role); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "REVOKE EXECUTE ON FUNCTION security.record_authorization_decision_v1(text,text,text,text,boolean) FROM "+role); err != nil {
		t.Fatal(err)
	}
	request("/status/dead-letter", http.StatusServiceUnavailable)
	assertLog("deadLetterAudit", "database_error", "42501")
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM security.authorization_decisions WHERE scope='inspect' AND NOT allowed`).Scan(&decisions); err != nil || decisions != 1 {
		t.Fatalf("unavailable audit changed decisions: %d: %v", decisions, err)
	}
	pool.Close()
	request("/status/dead-letter", http.StatusServiceUnavailable)
	assertLog("deadLetterAudit", "pool_closed", "")
}
