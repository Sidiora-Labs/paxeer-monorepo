package coordinatortest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
	"github.com/jackc/pgx/v5/pgxpool"
)

func assertMountedReadiness(t *testing.T, db Database) {
	t.Helper()
	t.Run("mounted readiness reports dependency causes and recovers", func(t *testing.T) {
		ctx := context.Background()
		rt := db.Runtimes["stats_runtime"]
		pool, err := pgxpool.New(ctx, rt.DSN)
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Close()
		want, err := production.LoadExpectedSchema(rt.ExpectedSchema, rt.ExpectedSchemaSHA256)
		if err != nil {
			t.Fatal(err)
		}
		cache, err := sharedredis.NewBoundedClient(os.Getenv("SHARED_TEST_REDIS_URL"), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer cache.Close()
		var expected atomic.Pointer[production.ExpectedSchema]
		expected.Store(&want)
		tracker := process.NewTracker()
		var logs bytes.Buffer
		router := sharedhttp.NewRouter(sharedhttp.ServerOptions{Health: sharedhttp.HealthDeps{
			DB:     func(ctx context.Context) error { return production.CheckReadiness(ctx, pool, *expected.Load()) },
			Redis:  func(ctx context.Context) error { return cache.Ping(ctx).Err() },
			State:  tracker.Ready,
			Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
		}})
		server := httptest.NewServer(router)
		defer server.Close()
		client := server.Client()
		client.Timeout = 5 * time.Second
		probe := func(path string, code int, state, database, lifecycle string) {
			t.Helper()
			response, err := client.Get(server.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			var body struct {
				Status    string            `json:"status"`
				Timestamp string            `json:"timestamp"`
				Checks    map[string]string `json:"checks"`
			}
			if err := json.Unmarshal(raw, &body); err != nil || response.StatusCode != code || body.Status != state || !strings.HasPrefix(response.Header.Get("Content-Type"), "application/json") {
				t.Fatalf("%s response: %d %s (%v)", path, response.StatusCode, raw, err)
			}
			if _, err := time.Parse(time.RFC3339, body.Timestamp); err != nil {
				t.Fatal(err)
			}
			if path == "/health/ready" && (body.Checks["database"] != database || body.Checks["redis"] != "ok" || body.Checks["lifecycle"] != lifecycle || !strings.HasSuffix(body.Checks["memory_alloc_mb"], "MB")) {
				t.Fatalf("readiness checks: %s", raw)
			}
			if strings.Contains(string(raw), "schema attestation mismatch") || strings.Contains(string(raw), rt.Role) || strings.Contains(string(raw), "service_not_ready") {
				t.Fatalf("readiness exposes dependency details: %s", raw)
			}
		}
		probe("/health", 200, "ok", "", "")
		probe("/health/live", 200, "ok", "", "")
		probe("/health/ready", 503, "degraded", "ok", "failed")
		if err := tracker.SetReady(); err != nil {
			t.Fatal(err)
		}
		probe("/health/ready", 200, "ok", "ok", "ok")
		mismatch := want
		mismatch.RegistryDigest = strings.Repeat("0", 64)
		expected.Store(&mismatch)
		probe("/health/ready", 503, "degraded", "failed", "ok")
		var databaseCause, lifecycleCause bool
		for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n")) {
			var entry map[string]any
			if err := json.Unmarshal(line, &entry); err != nil {
				t.Fatal(err)
			}
			databaseCause = databaseCause || (entry["check"] == "database" && entry["status"] == "failed" && entry["err"] == "schema attestation mismatch")
			lifecycleCause = lifecycleCause || (entry["check"] == "lifecycle" && entry["status"] == "failed" && entry["err"] == "service_not_ready")
		}
		if !databaseCause || !lifecycleCause {
			t.Fatalf("readiness failure causes: database=%v lifecycle=%v", databaseCause, lifecycleCause)
		}
		expected.Store(&want)
		probe("/health/ready", 200, "ok", "ok", "ok")
		if err := tracker.Drain(ctx); err != nil {
			t.Fatal(err)
		}
		probe("/health/ready", 503, "degraded", "ok", "failed")
		probe("/health/live", 200, "ok", "", "")
	})
}
