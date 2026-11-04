package cache

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	redisclient "github.com/redis/go-redis/v9"
)

const repairM992Pool = "0x1111111111111111111111111111111111111111"

func repairM992Config(t *testing.T, replica int) AccountingRuntimeConfig {
	t.Helper()
	// Closed real clients make every dependency access deterministic and local:
	// retained public snapshots may use the approved emergency allocation, but
	// cold reads must refuse instead of fabricating authoritative state.
	poolConfig, err := pgxpool.ParseConfig("postgres://fixture:fixture@127.0.0.1:1/fixture?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.MinConns = 0
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	client := redisclient.NewClient(&redisclient.Options{Addr: "127.0.0.1:1", MaxRetries: -1, ContextTimeoutEnabled: true})
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	admission, err := rest.NewAdmission(100, 10, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	return AccountingRuntimeConfig{
		Adapter: AdapterConfig{
			Redis: client, Namespace: "repair_m992",
			Local:        Config{Capacity: 64, MaxBytes: 1 << 20, MaxEntryBytes: 64 << 10, MaxFetches: 2, MaxPerOrigin: 2, FetchTimeout: 20 * time.Millisecond},
			Budget:       PostgresQueryBudget{Pool: pool, Schema: "api", Namespace: "repair_m992", Limit: 37, QueryCost: 1, Window: time.Hour},
			Emergency:    EmergencyAllocation{AllocationID: "approved_m992", ReplicaSlot: replica, ReplicaCount: 3, AggregateLimit: 7, Window: time.Hour},
			RedisTimeout: 10 * time.Millisecond, LeaseTTL: time.Second,
		},
		ReadModel: store.AccountingReadConfig{
			ChartsSchema: "charts", StatsSchema: "stats", ChartsDeliverySchema: "charts_delivery", StatsDeliverySchema: "stats_delivery",
			ProjectionVersion: "v2", ProjectionGeneration: "g1", ChartsConsumerID: "charts", StatsConsumerID: "stats", SourceStream: "accounting", SourcePartitionSchema: "indexer",
		},
		Allocation: CanonicalAllocation{FreshTTL: time.Minute, StaleTTL: time.Minute, RequestLimit: 7, RequestWindow: time.Hour},
		Origin:     "accounting_postgres", MarketConsumer: natsdriver.ConsumerConfig{Stream: "market", Durable: "cache_replica_" + strconv.Itoa(replica), Subject: accounting.MarketSubject, Batch: 1, MaxAckPending: 1, MaxWaiting: 1, MaxDeliver: 3, FetchWait: 10 * time.Millisecond, AckWait: time.Second},
		SigningKey: []byte(strings.Repeat("k", 32)), Admission: admission,
	}
}

func repairM992Runtime(t *testing.T, c AccountingRuntimeConfig) *AccountingRuntime {
	t.Helper()
	a, err := NewAccountingRuntime(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

func repairM992Seed(t *testing.T, a *AccountingRuntime, candles bool) (string, url.Values) {
	t.Helper()
	request := store.AccountingPageRequest{Pool: repairM992Pool, Limit: 50}
	route := "accounting_trades"
	path := "/v2/pools/" + repairM992Pool + "/trades"
	query := url.Values{}
	if candles {
		route = "accounting_candles"
		path = "/v2/pools/" + repairM992Pool + "/candles"
		request.Interval = "1m"
		request.From = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
		request.To = request.From + 60000
		query = url.Values{"interval": {"1m"}, "from": {"2026-10-01T00:00:00.000Z"}, "to": {"2026-10-01T00:01:00.000Z"}}
	}
	page := store.AccountingPage{
		Items: []json.RawMessage{}, Revision: "1", SnapshotID: "00000000-0000-7000-8000-000000000001", SeriesModelVersion: "v1",
		Watermark:   json.RawMessage(`{"chainId":"125","height":"1","blockHash":"0x1111111111111111111111111111111111111111111111111111111111111111","projectionRevision":"1","observedAt":"2026-10-01T00:00:00.000Z","integrity":"healthy"}`),
		GeneratedAt: "2026-10-01T00:00:00.000Z",
	}
	body, err := a.bindings.config.Codec.EncodeCachePage(request, candles, page, "00000000-0000-7000-8000-000000000002")
	if err != nil {
		t.Fatal(err)
	}
	parameters := canonicalParameters(request, query, candles)
	parameters.Set("sourceRevision", "unobserved")
	p, err := a.adapter.policy(route)
	if err != nil {
		t.Fatal(err)
	}
	key, err := a.adapter.key(p, VerifiedScope{}, parameters)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if !a.adapter.localPut(key, sharedredis.PolicySnapshot{Generation: p.Generation, Revision: page.Revision, Visibility: "public", Class: "public_snapshot", SourceWatermark: page.Watermark, Body: body, FetchedAt: now, FreshUntil: now.Add(time.Minute), RetainUntil: now.Add(2 * time.Minute)}) {
		t.Fatal("production codec snapshot was not retained")
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	return path, parameters
}

func repairM992Request(t *testing.T, client *http.Client, path string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Neither header is authoritative for the binding's network scope.
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("X-API-Key", "caller-controlled")
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var value json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, response.Header, value
}

func TestRepairM_992_MountedReplicaAdmission(t *testing.T) {
	admitted, limited := 0, 0
	for replica := 0; replica < 3; replica++ {
		a := repairM992Runtime(t, repairM992Config(t, replica))
		path, _ := repairM992Seed(t, a, false)
		router := chi.NewRouter()
		if err := a.bindings.Register(router, a.admission); err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(router)
		t.Cleanup(server.Close)
		for i := 0; i < 5; i++ {
			status, headers, body := repairM992Request(t, server.Client(), server.URL+path)
			switch status {
			case 200:
				admitted++
				if err := interfaces.ValidateJSON("TradesResponse", body); err != nil {
					t.Fatal(err)
				}
				if headers.Get("X-Projection-Revision") != "1" || headers.Get("X-Projection-Generation") != "g1" || headers.Get("X-Cache-Stale") != "true" || headers.Get("ETag") == "" {
					t.Fatal("missing projection provenance", headers)
				}
			case 429:
				limited++
				var result rest.APIError
				if err := json.Unmarshal(body, &result); err != nil || result.Code != "RATE_LIMITED" || !result.Retryable || result.RetryAfterMS == nil || *result.RetryAfterMS <= 0 {
					t.Fatalf("invalid rate response: %s, %v", body, err)
				}
				seconds, err := strconv.Atoi(headers.Get("Retry-After"))
				if err != nil || seconds*1000 != *result.RetryAfterMS {
					t.Fatal("retry delay differs from allocation", headers, result)
				}
			default:
				t.Fatalf("unexpected status %d: %s", status, body)
			}
		}
		if a.admission.Stats().Used != 0 {
			t.Fatal("mounted route leaked pool admission")
		}
	}
	if admitted != 7 || limited != 8 {
		t.Fatalf("aggregate allocation: admitted=%d limited=%d", admitted, limited)
	}
}

func TestRepairM_992_MountedCodecAndScope(t *testing.T) {
	a := repairM992Runtime(t, repairM992Config(t, 0))
	path, _ := repairM992Seed(t, a, true)
	router := chi.NewRouter()
	if err := a.bindings.Register(router, a.admission); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	defer server.Close()
	status, _, body := repairM992Request(t, server.Client(), server.URL+path)
	if status != 200 || interfaces.ValidateJSON("CandlesResponse", body) != nil {
		t.Fatalf("real codec/binding mismatch: %d %s", status, body)
	}
	for _, invalid := range []struct {
		suffix string
		status int
		code   string
	}{{"&wallet=secret", 422, "FIELD_INVALID"}, {"&limit=1&limit=2", 400, "QUERY_INVALID"}, {"&cursor=invalid", 400, "CURSOR_INVALID"}} {
		status, _, body = repairM992Request(t, server.Client(), server.URL+path+invalid.suffix)
		if status != invalid.status {
			t.Fatalf("invalid request reached cache: %d %s", status, body)
		}
		var result rest.APIError
		if err := json.Unmarshal(body, &result); err != nil || result.Code != invalid.code || result.Retryable {
			t.Fatalf("request rejection differs from frozen contract: %s, %v", body, err)
		}
	}
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "[::ffff:192.0.2.1]:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	scope, err := PublicNetworkScope(request)
	if err != nil || scope.NetworkID != "192.0.2.1" || scope.Principal != "" || scope.AuthorizationRevision != "" {
		t.Fatal("caller selected scope", scope, err)
	}
}

func TestRepairM_992_InvalidationAndRestartRefuseOldSnapshot(t *testing.T) {
	c := repairM992Config(t, 0)
	c.Adapter.Emergency.AggregateLimit = 100
	a := repairM992Runtime(t, c)
	path, parameters := repairM992Seed(t, a, false)
	router := chi.NewRouter()
	if err := a.bindings.Register(router, a.admission); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	defer server.Close()
	status, _, _ := repairM992Request(t, server.Client(), server.URL+path)
	if status != 200 {
		t.Fatal("retained snapshot unavailable before invalidation", status)
	}
	if err := a.adapter.Invalidate(context.Background(), "accounting_trades", parameters, VerifiedScope{}, "g1", "2"); !errors.Is(err, redisclient.ErrClosed) {
		t.Fatal("unavailable shared fence was acknowledged", err)
	}
	status, _, body := repairM992Request(t, server.Client(), server.URL+path)
	if status != 503 || !strings.Contains(string(body), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("invalidated snapshot resurrected: %d %s", status, body)
	}
	a.Close()
	reopened := repairM992Runtime(t, c)
	restartedRouter := chi.NewRouter()
	if err := reopened.bindings.Register(restartedRouter, reopened.admission); err != nil {
		t.Fatal(err)
	}
	restarted := httptest.NewServer(restartedRouter)
	defer restarted.Close()
	status, _, body = repairM992Request(t, restarted.Client(), restarted.URL+path)
	if status != 503 || !strings.Contains(string(body), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("restart served unverified old state: %d %s", status, body)
	}
}

func TestRepairM_992_ConsumerLifecycleRefusal(t *testing.T) {
	a := repairM992Runtime(t, repairM992Config(t, 0))
	router := chi.NewRouter()
	if err := a.Mount(router); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	defer server.Close()
	path, _ := repairM992Seed(t, a, false)
	if err := a.Start(context.Background(), nil); err == nil {
		t.Fatal("missing actual broker accepted")
	}
	if err := a.Run(context.Background()); err == nil {
		t.Fatal("unstarted market consumer ran")
	}
	if err := a.bindings.ConsumeMarket(context.Background(), &natsdriver.Message{Stream: "market", Sequence: 1}); err == nil {
		t.Fatal("invented broker position admitted")
	}
	for i := 0; i < 2; i++ {
		status, _, body := repairM992Request(t, server.Client(), server.URL+path)
		if status != 503 || !strings.Contains(string(body), "DEPENDENCY_UNAVAILABLE") {
			t.Fatalf("route served without live invalidation consumer: %d %s", status, body)
		}
		a.Close()
	}
	if err := a.Start(context.Background(), nil); err == nil || a.Ready(context.Background()) == nil {
		t.Fatal("closed runtime became ready")
	}
	if err := a.Shutdown(context.Background()); err != nil {
		t.Fatal("stopped runtime did not shut down", err)
	}
}

func TestRepairM_992_RejectUnboundAllocations(t *testing.T) {
	for _, change := range []func(*AccountingRuntimeConfig){
		func(c *AccountingRuntimeConfig) { c.Adapter.Budget.Pool = nil },
		func(c *AccountingRuntimeConfig) { c.Adapter.Budget.Limit = 0 },
		func(c *AccountingRuntimeConfig) { c.Adapter.Emergency.ReplicaSlot = 3 },
		func(c *AccountingRuntimeConfig) { c.SigningKey = c.SigningKey[:31] },
		func(c *AccountingRuntimeConfig) { c.PreviousSigningKeys = [][]byte{{1}} },
		func(c *AccountingRuntimeConfig) { c.ReadModel.ProjectionGeneration = "" },
		func(c *AccountingRuntimeConfig) { c.MarketConsumer.Subject = "foreign.subject" },
		func(c *AccountingRuntimeConfig) { c.MarketConsumer.Batch = 0 },
		func(c *AccountingRuntimeConfig) { c.Admission = nil },
	} {
		c := repairM992Config(t, 0)
		change(&c)
		if a, err := NewAccountingRuntime(c); err == nil || a != nil {
			t.Fatal("unbound runtime accepted")
		}
	}
}
