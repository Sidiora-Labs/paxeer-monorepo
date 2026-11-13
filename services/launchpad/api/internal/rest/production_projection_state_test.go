package rest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func TestProductionProjectionRoutesReadCommittedRankingAndStats(t *testing.T) {
	admin := os.Getenv(internaltest.ExternalPostgresEnv)
	if admin == "" {
		t.Fatal("PostgreSQL fixture required")
	}
	t.Setenv("API_TEST_DATABASE_URL", admin)
	pool, model, stream, address, _ := productionReadsFixture(t)
	admission, err := rest.NewAdmission(100, 100, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	if err := rest.RegisterProductionReads(router, rest.ProductionReadOptions{Model: model, Stream: stream, Admission: admission, RegisterSnapshots: true}); err != nil {
		t.Fatal(err)
	}
	rest.RegisterAccountingV2(router, model, []byte(strings.Repeat("k", 32)))
	read := func(path string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		return response
	}
	for _, check := range []struct {
		path, code string
		status     int
	}{
		{"/v2/pools/" + address + "/trades", "DEPENDENCY_UNAVAILABLE", 503},
		{"/v2/pools/" + address + "/candles?interval=1m&from=2026-10-08T00%3A00%3A00.000Z&to=2026-10-08T01%3A00%3A00.000Z", "DEPENDENCY_UNAVAILABLE", 503},
		{"/v2/pools/" + address + "/stats?unexpected=1", "QUERY_INVALID", 400},
		{"/v2/pools/0x" + strings.Repeat("3", 40) + "/stats", "DEPENDENCY_UNAVAILABLE", 503},
		{"/v2/rankings/trending", "DEPENDENCY_UNAVAILABLE", 503},
	} {
		response := read(check.path)
		if response.Code != check.status || !strings.Contains(response.Body.String(), check.code) {
			t.Fatalf("%s: status=%d body=%s", check.path, response.Code, response.Body.String())
		}
	}
	const watermark = `{"chainId":"125","height":"1","blockHash":"0x1111111111111111111111111111111111111111111111111111111111111111","projectionRevision":"1","observedAt":"2026-10-09T00:00:00.000Z","integrity":"healthy"}`
	items := []map[string]any{{"poolAddress": address, "category": "trending", "rank": 1, "score": "10", "modelVersion": "market-ranking-v1", "computedAt": "2026-10-09T00:00:00.000Z"}}
	payload, err := json.Marshal(map[string]any{"snapshotId": "8c0f8a3e-2b6c-4d0e-9a51-6f4f1d0b7c11", "category": "trending", "modelVersion": "market-ranking-v1", "computedAt": "2026-10-09T00:00:00.000Z", "watermark": json.RawMessage(watermark), "items": items, "totalEligible": "1", "truncated": false, "replace": true})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	frame := broker.Frame{EventID: "ranking_snapshot_1", Topic: "market.rankings.trending", Sequence: "1", Revision: "1", OccurredAt: "2026-10-09T00:00:00.000Z", Watermark: json.RawMessage(watermark), Payload: payload}
	if err := stream.AppendProjectionTx(context.Background(), tx, frame); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	response := read("/v2/rankings/trending?limit=1")
	if response.Code != http.StatusOK || interfaces.ValidateJSON("RankingsResponse", response.Body.Bytes()) != nil || !strings.Contains(response.Body.String(), address) {
		t.Fatalf("committed ranking: status=%d body=%s", response.Code, response.Body.String())
	}
	var schema string
	if err := pool.QueryRow(context.Background(), `SELECT nspname FROM pg_namespace WHERE nspname LIKE 'l22_stream_%'`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), "DROP TABLE "+pgx.Identifier{schema, "stream_projection"}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}
	response = read("/v2/rankings/trending")
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("unavailable ranking source: status=%d body=%s", response.Code, response.Body.String())
	}
}
