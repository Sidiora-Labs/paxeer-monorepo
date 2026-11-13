package cache

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
	"github.com/jackc/pgx/v5"
)

func TestAdapterChecksDatabaseProjectionBeforePublication(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	client, err := sharedredis.NewBoundedClient(internaltest.NewRedisURL(t), 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := db.Exec(ctx, `CREATE SCHEMA cache_contract;
		CREATE TABLE cache_contract.cache_query_budgets_v2(namespace text NOT NULL,origin text NOT NULL,window_bucket bigint NOT NULL,window_ms bigint NOT NULL,budget_limit bigint NOT NULL CHECK(budget_limit>0),used bigint NOT NULL CHECK(used>0 AND used<=budget_limit),PRIMARY KEY(namespace,origin,window_bucket,window_ms));
		CREATE TABLE cache_contract.source_projection(key text PRIMARY KEY,generation text NOT NULL,visibility text NOT NULL,revision text NOT NULL,watermark text NOT NULL,body text NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	policy := RoutePolicy{ID: "market_source", Version: "v2", Origin: "postgres", Generation: "g1", Class: PublicSnapshot, FreshTTL: time.Minute, RequestLimit: 100, RequestWindow: time.Hour, Parameters: map[string]ParameterRule{"key": {Required: true, MaxBytes: 8, Pattern: "[a-z][0-9]+"}}}
	adapter, err := NewAdapter(AdapterConfig{Redis: client, Namespace: "market_source", Policies: []RoutePolicy{policy}, Local: Config{Capacity: 32, MaxBytes: 1 << 20, MaxEntryBytes: 1 << 16, MaxFetches: 4, MaxPerOrigin: 4, FetchTimeout: 500 * time.Millisecond}, Budget: PostgresQueryBudget{Pool: db, Schema: "cache_contract", Namespace: "market_source", Limit: 20, QueryCost: 1, Window: time.Hour}, Emergency: EmergencyAllocation{AllocationID: "market_source", ReplicaSlot: 0, ReplicaCount: 1, AggregateLimit: 20, Window: time.Hour}, RedisTimeout: 200 * time.Millisecond, LeaseTTL: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	fetch := func(ctx context.Context, request ProjectionRequest) (Projection, error) {
		var out Projection
		var body, watermark string
		err := db.QueryRow(ctx, `SELECT generation,visibility,revision,watermark,body FROM cache_contract.source_projection WHERE key=$1`, request.Parameters.Get("key")).Scan(&out.Generation, &out.Visibility, &out.Revision, &watermark, &body)
		out.Body, out.SourceWatermark = json.RawMessage(body), json.RawMessage(watermark)
		return out, err
	}
	checks := []struct {
		name, generation, visibility, revision, watermark, body string
		valid                                                   bool
	}{
		{"a1", "g1", "public", "1", `null`, `{"value":"ready"}`, true},
		{"a2", "g2", "public", "1", `null`, `{"value":"ready"}`, false},
		{"a3", "g1", "private", "1", `null`, `{"value":"ready"}`, false},
		{"a4", "g1", "public", "1", `{`, `{"value":"ready"}`, false},
		{"a5", "g1", "public", "1", `null`, `null`, false},
	}
	for _, check := range checks {
		if _, err := db.Exec(ctx, `INSERT INTO cache_contract.source_projection VALUES($1,$2,$3,$4,$5,$6)`, check.name, check.generation, check.visibility, check.revision, check.watermark, check.body); err != nil {
			t.Fatal(err)
		}
		snapshot, stale, err := adapter.Read(ctx, policy.ID, url.Values{"key": {check.name}}, VerifiedScope{NetworkID: "127.0.0.1"}, fetch)
		if check.valid {
			if err != nil || stale || snapshot.Generation != "g1" || !strings.Contains(string(snapshot.Body), `"ready"`) {
				t.Fatalf("valid projection %+v: snapshot=%+v stale=%t err=%v", check, snapshot, stale, err)
			}
		} else if !errors.Is(err, ErrProjectionChanged) || len(snapshot.Body) != 0 {
			t.Fatalf("invalid projection %+v: snapshot=%+v err=%v", check, snapshot, err)
		}
	}
	if _, _, err := adapter.Read(ctx, policy.ID, url.Values{"key": {"a6"}}, VerifiedScope{NetworkID: "127.0.0.1"}, fetch); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("absent source projection: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/source?key=a1", nil)
	request.RemoteAddr = "127.0.0.1:2345"
	response := httptest.NewRecorder()
	adapter.Handler(policy.ID, PublicNetworkScope, fetch)(response, request)
	if response.Code != http.StatusOK || response.Header().Get("X-Projection-Revision") != "1" || !strings.Contains(response.Body.String(), `"ready"`) {
		t.Fatalf("published source projection: status=%d body=%s", response.Code, response.Body.String())
	}
	missing := httptest.NewRequest(http.MethodGet, "/source?key=a6", nil)
	missing.RemoteAddr = request.RemoteAddr
	response = httptest.NewRecorder()
	adapter.Handler(policy.ID, PublicNetworkScope, fetch)(response, missing)
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "1" {
		t.Fatalf("unavailable source response: status=%d headers=%v", response.Code, response.Header())
	}
	if adapter.Stats().CompletedReconstructions == 0 {
		t.Fatal("confirmed source was not reconstructed")
	}
}
