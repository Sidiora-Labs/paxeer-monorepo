package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/redis/go-redis/v9"
)

func TestHistoryOriginFlightCoalescesReaders(t *testing.T) {
	cache, err := NewHistoryCache(nil, PostgresHistoryRevision{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	load := func(ctx context.Context) (chartCacheValue, error) {
		close(entered)
		select {
		case <-release:
			return chartCacheValue{cacheState: cacheMiss, Scope: "shared"}, nil
		case <-ctx.Done():
			return chartCacheValue{}, ctx.Err()
		}
	}
	type result struct {
		value chartCacheValue
		err   error
	}
	first := make(chan result, 1)
	second := make(chan result, 1)
	go func() { value, e := cache.run(context.Background(), "shared", load); first <- result{value, e} }()
	<-entered
	go func() { value, e := cache.run(context.Background(), "shared", load); second <- result{value, e} }()
	deadline := time.After(2 * time.Second)
	for {
		cache.mu.Lock()
		flight := cache.flights["shared"]
		joined := flight != nil && flight.waiters == 2
		cache.mu.Unlock()
		if joined {
			break
		}
		select {
		case <-deadline:
			t.Fatal("second reader did not join origin flight")
		case <-time.After(time.Millisecond):
		}
	}
	close(release)
	a, b := <-first, <-second
	if a.err != nil || b.err != nil || a.value.Scope != "shared" || b.value.Scope != "shared" || a.value.cacheState != cacheMiss || b.value.cacheState != "coalesced" {
		t.Fatalf("coalesced origin results: %+v %+v", a, b)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = cache.run(ctx, "cancelled", load); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled reader: %v", err)
	}
}

func TestProductionHistoryRejectsInvalidRequests(t *testing.T) {
	handler := ProductionHistoryHandler{}
	for _, tc := range []struct {
		query  string
		status int
	}{
		{"mode=unknown", http.StatusUnprocessableEntity},
		{"mode=price&mode=price", http.StatusUnprocessableEntity},
		{"from=0&to=120&cursor=next", http.StatusUnprocessableEntity},
		{"from=0&to=120", http.StatusServiceUnavailable},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, historyURL(tc.query), nil))
		if rec.Code != tc.status || rec.Body.Len() == 0 {
			t.Fatalf("history request %q: status=%d body=%s", tc.query, rec.Code, rec.Body.String())
		}
	}
	large := httptest.NewRecorder()
	handler.ServeHTTP(large, httptest.NewRequest(http.MethodGet, historyURL("x="+strings.Repeat("a", 16385)), nil))
	if large.Code != http.StatusUnprocessableEntity {
		t.Fatalf("oversized history query status: %d", large.Code)
	}
}

func TestHistoryCacheChecksRevisionBeforeCanonicalOrigin(t *testing.T) {
	ctx := context.Background()
	pool := historyPostgres(t)
	if _, err := pool.Exec(ctx, `CREATE SCHEMA revision_scope;
CREATE TABLE revision_scope.chart_exact(chain_id text,pool text,interval text,start bigint,snapshot jsonb);
CREATE VIEW revision_scope.chart_exact_v1 AS SELECT * FROM revision_scope.chart_exact`); err != nil {
		t.Fatal(err)
	}
	request := HistoryRequest{Symbol: udfPool, Timeframe: "1m", From: 0, To: 120}
	revision := PostgresHistoryRevision{Pool: pool, Schema: "revision_scope"}
	bounded, err := NewHistoryCache(nil, revision, 1)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := bounded.read(ctx, HistoryRequest{Symbol: "unknown", Timeframe: "1m", From: 0, To: 120}, modePrice, "algorithm-v1", nil, CanonicalCandleClient{}, nil); err == nil || len(value.Body) != 0 || bounded.OriginFetches() != 0 {
		t.Fatalf("invalid history scope reached canonical origin: %+v, %v", value, err)
	}
	if _, err := pool.Exec(ctx, `ALTER VIEW revision_scope.chart_exact_v1 RENAME TO chart_exact_unavailable`); err != nil {
		t.Fatal(err)
	}
	if value, err := bounded.read(ctx, request, modePrice, "algorithm-v1", nil, CanonicalCandleClient{}, nil); err == nil || len(value.Body) != 0 || !strings.Contains(err.Error(), "chart_exact_v1") {
		t.Fatalf("missing revision authority: %+v, %v", value, err)
	}
	if _, err := pool.Exec(ctx, `ALTER VIEW revision_scope.chart_exact_unavailable RENAME TO chart_exact_v1`); err != nil {
		t.Fatal(err)
	}
	responseRevision := interfaces.PositiveUInt256("1")
	responseWatermark := &interfaces.Watermark{ChainId: "125", Height: "1", BlockHash: "0x" + strings.Repeat("a", 64), Integrity: "healthy", ObservedAt: "2026-10-12T00:00:00.000Z", ProjectionRevision: "1"}
	response := interfaces.CandlesResponse{Data: interfaces.CandlesData{Items: []interfaces.Candle{}, SeriesModelVersion: "qualified-exact-v2", SnapshotId: "11111111-1111-4111-8111-111111111111"}, GeneratedAt: "2026-10-12T00:00:00.000Z", Quality: "complete", RequestId: "22222222-2222-4222-8222-222222222222", Revision: &responseRevision, Watermark: responseWatermark}
	wire, err := json.Marshal(response)
	if err != nil || interfaces.ValidateJSON("CandlesResponse", wire) != nil {
		t.Fatalf("canonical empty page: %s, %v", wire, err)
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := pool.Exec(ctx, `INSERT INTO revision_scope.chart_exact VALUES('125',$1,'1m',60,'{"revision":"2"}')`, request.Symbol); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(wire); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(origin.Close)
	if value, err := bounded.read(ctx, request, modePrice, "qualified-exact-v2", nil, CanonicalCandleClient{Endpoint: origin.URL}, nil); err == nil || len(value.Body) != 0 || !strings.Contains(err.Error(), "revision changed during fill") {
		t.Fatalf("history revision across origin read: %+v, %v", value, err)
	}
	for _, withRedis := range []bool{false, true} {
		var rdb *redis.Client
		if withRedis {
			options, err := redis.ParseURL(internaltest.NewRedisURL(t))
			if err != nil {
				t.Fatal(err)
			}
			rdb = redis.NewClient(options)
			t.Cleanup(func() { _ = rdb.Close() })
		}
		cache, err := NewHistoryCache(rdb, revision, 1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = cache.read(ctx, request, "unsupported", "algorithm-v1", nil, CanonicalCandleClient{}, nil); err == nil || cache.OriginFetches() != 0 {
			t.Fatalf("invalid mode reached origin: %v, %d", err, cache.OriginFetches())
		}
		if _, err = cache.read(ctx, request, modePrice, "algorithm-v1", nil, CanonicalCandleClient{}, nil); err == nil || cache.OriginFetches() != 1 {
			t.Fatalf("invalid canonical endpoint: %v, %d", err, cache.OriginFetches())
		}
		rec := httptest.NewRecorder()
		handler := ProductionHistoryHandler{Cache: cache, Algorithm: "algorithm-v1"}
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, historyURL("from=0&to=120"), nil))
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "DEPENDENCY_UNAVAILABLE") {
			t.Fatalf("canonical origin refusal: %d, %s", rec.Code, rec.Body.String())
		}
	}
}
