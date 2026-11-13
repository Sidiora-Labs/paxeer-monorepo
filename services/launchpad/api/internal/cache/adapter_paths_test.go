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
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
)

func TestAdapterServesAndInvalidatesDatabaseProjection(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	redisURL := internaltest.NewRedisURL(t)
	client, err := sharedredis.NewBoundedClient(redisURL, 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := db.Exec(ctx, `CREATE SCHEMA cache_runtime;
		CREATE TABLE cache_runtime.cache_query_budgets_v2(namespace text NOT NULL,origin text NOT NULL,window_bucket bigint NOT NULL,window_ms bigint NOT NULL,budget_limit bigint NOT NULL CHECK(budget_limit>0),used bigint NOT NULL CHECK(used>0 AND used<=budget_limit),PRIMARY KEY(namespace,origin,window_bucket,window_ms))`); err != nil {
		t.Fatal(err)
	}
	const pool = "0x1111111111111111111111111111111111111111"
	const token = "0x2222222222222222222222222222222222222222"
	if _, err := db.Exec(ctx, `INSERT INTO stats.pool_stats(pool_address,token_address,price,created_at,updated_at) VALUES($1,$2,'100',100,1)`, pool, token); err != nil {
		t.Fatal(err)
	}
	policy := RoutePolicy{ID: "pool_price", Version: "v2", Origin: "postgres", Generation: "g1", Class: PublicSnapshot, FreshTTL: time.Minute, RequestLimit: 100, RequestWindow: time.Hour, Parameters: map[string]ParameterRule{"pool": {Required: true, MaxBytes: 42, Pattern: `0x[0-9a-f]{40}`}}}
	a, err := NewAdapter(AdapterConfig{
		Redis: client, Namespace: "api_price", Policies: []RoutePolicy{policy},
		Local:        Config{Capacity: 16, MaxBytes: 1 << 20, MaxEntryBytes: 1 << 16, MaxFetches: 4, MaxPerOrigin: 4, FetchTimeout: 500 * time.Millisecond},
		Budget:       PostgresQueryBudget{Pool: db, Schema: "cache_runtime", Namespace: "api_price", Limit: 100, QueryCost: 1, Window: time.Hour},
		Emergency:    EmergencyAllocation{AllocationID: "api_price", ReplicaSlot: 0, ReplicaCount: 1, AggregateLimit: 100, Window: time.Hour},
		RedisTimeout: 200 * time.Millisecond, LeaseTTL: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := a.config.Budget.admit(ctx, "postgres")
	if err != nil || !admitted.Allowed || admitted.Remaining != 99 {
		t.Fatalf("database query budget: %+v, %v", admitted, err)
	}
	if (&LimitError{RetryAfter: time.Second}).Error() != "cache: admission allocation exhausted" {
		t.Fatal("admission limit has no stable error")
	}
	for _, check := range []struct {
		name string
		change func(*AdapterConfig)
	}{
		{"namespace", func(c *AdapterConfig) { c.Namespace = "invalid namespace" }},
		{"request allocation", func(c *AdapterConfig) { c.Local.MaxFetches = 0 }},
		{"refresh lease", func(c *AdapterConfig) { c.LeaseTTL = 0 }},
		{"route policy", func(c *AdapterConfig) { c.Policies[0].RequestLimit = 0 }},
		{"duplicate route", func(c *AdapterConfig) { c.Policies = append(c.Policies, c.Policies[0]) }},
	} {
		t.Run(check.name, func(t *testing.T) {
			candidate := a.config
			candidate.Policies = append([]RoutePolicy(nil), candidate.Policies...)
			check.change(&candidate)
			if _, err := NewAdapter(candidate); err == nil {
				t.Fatal("invalid adapter configuration accepted")
			}
		})
	}
	var fetches atomic.Int32
	fetch := func(ctx context.Context, r ProjectionRequest) (Projection, error) {
		fetches.Add(1)
		var revision, raw string
		err := db.QueryRow(ctx, `SELECT updated_at::text,jsonb_build_object('poolAddress',pool_address,'price',price)::text FROM stats.pool_stats WHERE pool_address=$1`, r.Parameters.Get("pool")).Scan(&revision, &raw)
		return Projection{Generation: r.Generation, Revision: revision, Visibility: "public", Body: json.RawMessage(raw), SourceWatermark: json.RawMessage(`null`)}, err
	}
	q := url.Values{"pool": {pool}}
	scope := VerifiedScope{NetworkID: "127.0.0.1"}
	first, stale, err := a.Read(ctx, policy.ID, q, scope, fetch)
	if err != nil || stale || first.Revision != "1" || !strings.Contains(string(first.Body), `"price":"100"`) {
		t.Fatalf("first projection: %+v stale=%v err=%v", first, stale, err)
	}
	for _, check := range []struct {
		name, body string
	}{
		{"malformed cache record", `{`},
		{"invalid snapshot record", `{"generation":"g1","revision":"1","visibility":"public","class":"public_snapshot","body":null}`},
	} {
		t.Run(check.name, func(t *testing.T) {
			key := "api_price:corrupt:" + check.name
			a.local.mu.Lock()
			a.local.storeLocked(key, []byte(check.body), "", time.Now().Add(time.Minute))
			a.local.mu.Unlock()
			if _, found := a.localGet(key); found {
				t.Fatal("corrupt projection was served")
			}
			if a.local.Len() != 1 {
				t.Fatalf("corrupt projection retained: %d entries", a.local.Len())
			}
		})
	}
	if a.localPut("api_price:invalid-json", sharedredis.PolicySnapshot{Body: json.RawMessage(`{`)}) {
		t.Fatal("invalid JSON retained as a cache projection")
	}
	key, err := a.key(policy, scope, q)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.cached(ctx, policy, key); err != nil {
		t.Fatalf("shared projection cache: %v", err)
	}
	second, stale, err := a.Read(ctx, policy.ID, q, scope, fetch)
	if err != nil || stale || second.ContentDigest != first.ContentDigest || fetches.Load() != 1 {
		t.Fatalf("cached projection: %+v stale=%v fetches=%d err=%v", second, stale, fetches.Load(), err)
	}
	if err := a.Prewarm(ctx, policy.ID, []url.Values{q}, scope, fetch); err != nil || fetches.Load() != 1 {
		t.Fatalf("prewarm: fetches=%d err=%v", fetches.Load(), err)
	}
	serve := a.Handler(policy.ID, PublicNetworkScope, fetch)
	request := httptest.NewRequest(http.MethodGet, "/price?"+q.Encode(), nil)
	request.RemoteAddr = "127.0.0.1:1234"
	record := httptest.NewRecorder()
	serve(record, request)
	if record.Code != 200 || record.Header().Get("X-Projection-Revision") != "1" || !strings.Contains(record.Body.String(), `"price":"100"`) {
		t.Fatalf("price response: %d %s %v", record.Code, record.Body.String(), record.Header())
	}
	conditional := httptest.NewRequest(http.MethodGet, "/price?"+q.Encode(), nil)
	conditional.RemoteAddr = request.RemoteAddr
	conditional.Header.Set("If-None-Match", record.Header().Get("ETag"))
	unchanged := httptest.NewRecorder()
	serve(unchanged, conditional)
	if unchanged.Code != http.StatusNotModified || unchanged.Body.Len() != 0 {
		t.Fatalf("conditional price response: %d %s", unchanged.Code, unchanged.Body.String())
	}
	if _, err := db.Exec(ctx, `UPDATE stats.pool_stats SET price='200',updated_at=3 WHERE pool_address=$1`, pool); err != nil {
		t.Fatal(err)
	}
	if err := a.Invalidate(ctx, policy.ID, q, scope, "g1", "2"); err != nil {
		t.Fatal(err)
	}
	if accepted := a.localPut(key, sharedredis.PolicySnapshot{Generation: "g1", Revision: "1", Visibility: "public", Class: "public_snapshot", Body: json.RawMessage(`{"price":"stale"}`), FetchedAt: time.Now(), FreshUntil: time.Now().Add(time.Minute), RetainUntil: time.Now().Add(time.Minute)}); accepted {
		t.Fatal("older local revision replaced an invalidation floor")
	}
	latest, stale, err := a.Read(ctx, policy.ID, q, scope, fetch)
	if err != nil || stale || latest.Revision != "3" || !strings.Contains(string(latest.Body), `"price":"200"`) {
		t.Fatalf("updated projection: %+v stale=%v err=%v", latest, stale, err)
	}
	if err := a.Cutover(policy.ID, "g1", "g2"); err != nil {
		t.Fatal(err)
	}
	cutover, stale, err := a.Read(ctx, policy.ID, q, scope, fetch)
	if err != nil || stale || cutover.Generation != "g2" || cutover.Revision != "3" {
		t.Fatalf("cutover projection: %+v stale=%v err=%v", cutover, stale, err)
	}
	capture := a.MeasurementSnapshot("replica-1", strings.Repeat("a", 64))
	if capture.Stats.Bytes <= 0 || len(capture.Series) == 0 {
		t.Fatalf("adapter measurements: %+v", capture)
	}
	denied := httptest.NewRecorder()
	a.MeasurementHandler("replica-1", strings.Repeat("a", 64), nil)(denied, httptest.NewRequest(http.MethodGet, "/measurements", nil))
	if denied.Code != http.StatusServiceUnavailable {
		t.Fatalf("measurement authorization: %d", denied.Code)
	}
	authorize := func(r *http.Request) error {
		if r.Header.Get("Authorization") != "Bearer measurement-reader" {
			return errors.New("measurement reader required")
		}
		return nil
	}
	measure := a.MeasurementHandler("replica-1", strings.Repeat("a", 64), authorize)
	for _, check := range []struct {
		method, bearer string
		status         int
	}{
		{http.MethodPost, "Bearer measurement-reader", http.StatusMethodNotAllowed},
		{http.MethodGet, "", http.StatusForbidden},
		{http.MethodGet, "Bearer measurement-reader", http.StatusOK},
	} {
		request := httptest.NewRequest(check.method, "/measurements", nil)
		request.Header.Set("Authorization", check.bearer)
		response := httptest.NewRecorder()
		measure(response, request)
		if response.Code != check.status {
			t.Fatalf("measurement response %d, want %d: %s", response.Code, check.status, response.Body.String())
		}
		if check.status == http.StatusOK {
			var capture MeasurementCapture
			if err := json.Unmarshal(response.Body.Bytes(), &capture); err != nil || capture.ReplicaID != "replica-1" || capture.Stats.Entries == 0 {
				t.Fatalf("measurement body: %+v, %v", capture, err)
			}
		}
	}
	if err := a.Prewarm(ctx, "missing", []url.Values{q}, scope, fetch); err == nil || err.Error() != "cache: unregistered route" {
		t.Fatalf("unregistered prewarm: %v", err)
	}
	for _, tc := range []struct {
		name     string
		id       string
		resolver ScopeResolver
		query    string
		remote   string
		status   int
	}{
		{"missing resolver", policy.ID, nil, q.Encode(), "127.0.0.1:1234", 503},
		{"invalid peer", policy.ID, PublicNetworkScope, q.Encode(), "not-an-ip", 403},
		{"invalid query", policy.ID, PublicNetworkScope, "pool=%ZZ", "127.0.0.1:1234", 422},
		{"unknown route", "missing", PublicNetworkScope, q.Encode(), "127.0.0.1:1234", 503},
		{"missing pool", policy.ID, PublicNetworkScope, "", "127.0.0.1:1234", 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/price?"+tc.query, nil)
			req.RemoteAddr = tc.remote
			response := httptest.NewRecorder()
			a.Handler(tc.id, tc.resolver, fetch)(response, req)
			if response.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", response.Code, tc.status, response.Body.String())
			}
		})
	}
	if _, _, err := a.Read(ctx, policy.ID, q, VerifiedScope{}, fetch); !errors.Is(err, ErrAdmissionUnavailable) {
		t.Fatalf("missing network scope: %v", err)
	}
	if _, _, err := a.Read(ctx, policy.ID, q, scope, nil); err == nil || !strings.Contains(err.Error(), "missing projection provider") {
		t.Fatalf("missing provider: %v", err)
	}
	if _, _, err := a.Read(ctx, "missing", q, scope, fetch); err == nil || !strings.Contains(err.Error(), "unregistered route") {
		t.Fatalf("unknown route: %v", err)
	}
	if err := a.Invalidate(ctx, policy.ID, q, scope, "prior-generation", "1"); err != nil {
		t.Fatalf("prior generation invalidation: %v", err)
	}
	if err := a.Invalidate(ctx, policy.ID, q, scope, "g2", "invalid"); err == nil {
		t.Fatal("non-numeric invalidation revision accepted")
	}
	if err := a.Cutover(policy.ID, "g2", "g2"); err == nil {
		t.Fatal("same generation cutover accepted")
	}
	if err := a.Cutover(policy.ID, "prior-generation", "g3"); !errors.Is(err, ErrProjectionChanged) {
		t.Fatalf("stale generation cutover: %v", err)
	}
	if err := a.Prewarm(ctx, policy.ID, make([]url.Values, 65), scope, fetch); err == nil || !strings.Contains(err.Error(), "bounded public keys") {
		t.Fatalf("unbounded prewarm: %v", err)
	}
	if result := a.AdmissionMiddleware(policy.ID, PublicNetworkScope, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("snapshot admitted as write") })); result == nil {
		t.Fatal("write admission handler missing")
	} else {
		response := httptest.NewRecorder()
		result.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/write", nil))
		if response.Code != 503 {
			t.Fatalf("snapshot write admission status %d", response.Code)
		}
	}
	current, err := a.policy(policy.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i, tc := range []struct {
		name       string
		projection Projection
	}{
		{"generation", Projection{Generation: "old", Revision: "1", Visibility: "public", Body: json.RawMessage(`{"price":"1"}`), SourceWatermark: json.RawMessage(`null`)}},
		{"visibility", Projection{Generation: "g2", Revision: "1", Visibility: "private", Body: json.RawMessage(`{"price":"1"}`), SourceWatermark: json.RawMessage(`null`)}},
		{"null body", Projection{Generation: "g2", Revision: "1", Visibility: "public", Body: json.RawMessage(`null`), SourceWatermark: json.RawMessage(`null`)}},
		{"malformed body", Projection{Generation: "g2", Revision: "1", Visibility: "public", Body: json.RawMessage(`{`), SourceWatermark: json.RawMessage(`null`)}},
		{"malformed watermark", Projection{Generation: "g2", Revision: "1", Visibility: "public", Body: json.RawMessage(`{"price":"1"}`), SourceWatermark: json.RawMessage(`{`)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := "api_price:invalid_" + strconv.Itoa(i)
			_, err := a.origin(ctx, current, key, q, scope, func(context.Context, ProjectionRequest) (Projection, error) { return tc.projection, nil })
			if !errors.Is(err, ErrProjectionChanged) {
				t.Fatalf("invalid source projection: %v", err)
			}
			if _, found, err := a.cached(ctx, current, key); err != nil || found {
				t.Fatalf("invalid projection retained: found=%t err=%v", found, err)
			}
		})
	}
}
