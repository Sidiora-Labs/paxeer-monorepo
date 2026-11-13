package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/redis/go-redis/v9"
)

func TestHistoryCacheReadsCanonicalOriginAndTracksRevision(t *testing.T) {
	ctx := context.Background()
	pool := historyPostgres(t)
	if _, err := pool.Exec(ctx, `CREATE SCHEMA origin_revision;
CREATE TABLE origin_revision.chart_exact(chain_id text,pool text,interval text,start bigint,snapshot jsonb);
CREATE VIEW origin_revision.chart_exact_v1 AS SELECT * FROM origin_revision.chart_exact`); err != nil {
		t.Fatal(err)
	}
	revision := interfaces.PositiveUInt256("1")
	watermark := &interfaces.Watermark{ChainId: "125", Height: "1", BlockHash: "0x" + strings.Repeat("a", 64), Integrity: "healthy", ObservedAt: "2026-10-12T00:00:00.000Z", ProjectionRevision: "1"}
	response := interfaces.CandlesResponse{Data: interfaces.CandlesData{Items: []interfaces.Candle{}, SeriesModelVersion: "qualified-exact-v2", SnapshotId: "11111111-1111-4111-8111-111111111111"}, GeneratedAt: "2026-10-12T00:00:00.000Z", Quality: "complete", RequestId: "22222222-2222-4222-8222-222222222222", Revision: &revision, Watermark: watermark}
	encoded, err := json.Marshal(response)
	if err != nil || interfaces.ValidateJSON("CandlesResponse", encoded) != nil {
		t.Fatalf("canonical response contract: %s, %v", encoded, err)
	}
	var calls atomic.Int64
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if !strings.HasSuffix(request.URL.Path, "/candles") || request.URL.Query().Get("interval") != "1m" {
			http.Error(w, "invalid canonical scope", http.StatusUnprocessableEntity)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(encoded)
	}))
	t.Cleanup(origin.Close)
	options, err := redis.ParseURL(internaltest.NewRedisURL(t))
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(options)
	t.Cleanup(func() { _ = rdb.Close() })
	cache, err := NewHistoryCache(rdb, PostgresHistoryRevision{Pool: pool, Schema: "origin_revision"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	request := HistoryRequest{Symbol: udfPool, Timeframe: "1m", From: 0, To: 120}
	source := CanonicalCandleClient{Endpoint: origin.URL}
	first, err := cache.read(ctx, request, modePrice, "qualified-exact-v2", nil, source, nil)
	if err != nil || first.cacheState != cacheMiss || first.Policy != "private, no-cache" || !validChartCache(first, first.Scope, request, modePrice, "qualified-exact-v2") || calls.Load() != 1 {
		t.Fatalf("canonical origin fill: %+v, calls=%d, err=%v", first, calls.Load(), err)
	}
	changeBody := func(edit func(map[string]any)) func(*chartCacheValue) {
		return func(value *chartCacheValue) {
			var body map[string]any
			if err := json.Unmarshal(value.Body, &body); err != nil {
				t.Fatal(err)
			}
			edit(body)
			wire, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			value.Body = wire
			value.ETag = chartETag(value.Scope, value.Watermark, wire)
		}
	}
	for _, tc := range []struct {
		name   string
		change func(*chartCacheValue)
	}{
		{"wrong cache scope", func(v *chartCacheValue) { v.Scope = "other" }},
		{"empty body", func(v *chartCacheValue) { v.Body = nil }},
		{"wrong entity tag", func(v *chartCacheValue) { v.ETag = `"other"` }},
		{"expired response", func(v *chartCacheValue) { v.Expiry = time.Now().Add(-time.Second) }},
		{"unbounded expiry", func(v *chartCacheValue) { v.Expiry = time.Now().Add(time.Minute * 2) }},
		{"malformed watermark", func(v *chartCacheValue) { v.Watermark = []byte(`{`); v.ETag = chartETag(v.Scope, v.Watermark, v.Body) }},
		{"unhealthy watermark", func(v *chartCacheValue) {
			v.Watermark = []byte(`{"integrity":"stale"}`)
			v.ETag = chartETag(v.Scope, v.Watermark, v.Body)
		}},
		{"invalid policy", func(v *chartCacheValue) { v.Policy = "public, max-age=3600" }},
		{"malformed body", func(v *chartCacheValue) { v.Body = []byte(`{`); v.ETag = chartETag(v.Scope, v.Watermark, v.Body) }},
		{"wrong history mode", changeBody(func(body map[string]any) { body["mode"] = modeTotalSupplyValuation })},
		{"absent snapshot", changeBody(func(body map[string]any) { body["snapshotId"] = "" })},
		{"nonempty no-data series", changeBody(func(body map[string]any) { body["t"] = []int64{60} })},
		{"mismatched arrays", changeBody(func(body map[string]any) { body["o"] = []float64{1} })},
		{"wrong algorithm", func(v *chartCacheValue) {
			var body map[string]any
			if err := json.Unmarshal(v.Body, &body); err != nil {
				t.Fatal(err)
			}
			body["algorithmVersion"] = "other"
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			v.Body = encoded
			v.ETag = chartETag(v.Scope, v.Watermark, v.Body)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := first
			tc.change(&value)
			if validChartCache(value, first.Scope, request, modePrice, "qualified-exact-v2") {
				t.Fatal("invalid cached history admitted")
			}
		})
	}
	second, err := cache.read(ctx, request, modePrice, "qualified-exact-v2", nil, source, nil)
	if err != nil || second.cacheState != cacheHit || second.ETag != first.ETag || calls.Load() != 1 {
		t.Fatalf("in-process revision hit: %+v, calls=%d, err=%v", second, calls.Load(), err)
	}
	other, err := NewHistoryCache(rdb, PostgresHistoryRevision{Pool: pool, Schema: "origin_revision"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := other.read(ctx, request, modePrice, "qualified-exact-v2", nil, source, nil)
	if err != nil || shared.cacheState != cacheHit || shared.ETag != first.ETag || calls.Load() != 1 {
		t.Fatalf("shared revision hit: %+v, calls=%d, err=%v", shared, calls.Load(), err)
	}
	requestURL := historyURL("from=0&to=120")
	conditional := httptest.NewRequest(http.MethodGet, requestURL, nil)
	conditional.Header.Set("If-None-Match", first.ETag)
	writer := httptest.NewRecorder()
	(ProductionHistoryHandler{Source: source, Cache: cache, Algorithm: "qualified-exact-v2"}).ServeHTTP(writer, conditional)
	if writer.Code != http.StatusNotModified || writer.Header().Get("ETag") != first.ETag {
		t.Fatalf("conditional history response: %d, %v", writer.Code, writer.Header())
	}
	if _, err = pool.Exec(ctx, `INSERT INTO origin_revision.chart_exact VALUES('125',$1,'1m',60,'{"revision":"2"}')`, request.Symbol); err != nil {
		t.Fatal(err)
	}
	cache.mu.Lock()
	for index := 0; index < 128; index++ {
		cache.entries[fmt.Sprintf("a%03d", index)] = chartCacheValue{Scope: "expired"}
	}
	cache.mu.Unlock()
	updated, err := cache.read(ctx, request, modePrice, "qualified-exact-v2", nil, source, nil)
	if err != nil || updated.cacheState != cacheMiss || calls.Load() != 2 || updated.Scope == first.Scope {
		t.Fatalf("corrected revision origin: %+v, calls=%d, err=%v", updated, calls.Load(), err)
	}
	cache.mu.Lock()
	_, oldest := cache.entries["a000"]
	_, inserted := cache.entries[updated.Scope]
	cache.mu.Unlock()
	if oldest || !inserted {
		t.Fatalf("bounded cache retained oldest=%t inserted=%t", oldest, inserted)
	}
	if updated.Expiry.Before(time.Now()) {
		t.Fatalf("corrected revision expired on creation: %+v", updated)
	}
	version, err := cache.Revision.Read(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	permit := &HistoricalCachePermit{RevisionDigest: version, InvalidationEvidenceID: "chart-correction", Complete: true, MaxAge: 2 * time.Second}
	public, err := cache.read(ctx, request, modePrice, "qualified-exact-v2", permit, source, nil)
	if err != nil || public.Policy != "public, max-age=2, must-revalidate" || calls.Load() != 3 {
		t.Fatalf("historical cache permit: %+v, calls=%d, err=%v", public, calls.Load(), err)
	}
	if value, err := cache.read(ctx, request, modePrice, "different-algorithm", nil, source, nil); err == nil || !strings.Contains(err.Error(), "algorithm scope mismatch") || len(value.Body) != 0 {
		t.Fatalf("canonical algorithm binding: %+v, %v", value, err)
	}
	priorCalls := calls.Load()
	contended := request
	contended.To = 180
	contendedVersion, err := cache.Revision.Read(ctx, contended)
	if err != nil {
		t.Fatal(err)
	}
	contendedKey := "chart:v3:" + historyRequestKey(contended, modePrice, "qualified-exact-v2", source.Endpoint, nil) + ":" + contendedVersion
	if err := rdb.Set(ctx, contendedKey, []byte(`{`), time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, contendedKey+":fill", "other-reader", 80*time.Millisecond).Err(); err != nil {
		t.Fatal(err)
	}
	filled, err := cache.read(ctx, contended, modePrice, "qualified-exact-v2", nil, source, nil)
	if err != nil || filled.cacheState != cacheMiss || !validChartCache(filled, filled.Scope, contended, modePrice, "qualified-exact-v2") || calls.Load() != priorCalls+1 {
		t.Fatalf("contended canonical fill: state=%q calls=%d err=%v", filled.cacheState, calls.Load(), err)
	}
	cancelled := request
	cancelled.To = 240
	cancelledVersion, err := cache.Revision.Read(ctx, cancelled)
	if err != nil {
		t.Fatal(err)
	}
	cancelledKey := "chart:v3:" + historyRequestKey(cancelled, modePrice, "qualified-exact-v2", source.Endpoint, nil) + ":" + cancelledVersion
	if err := rdb.Set(ctx, cancelledKey+":fill", "other-reader", time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	short, stop := context.WithTimeout(ctx, 30*time.Millisecond)
	defer stop()
	if _, err := cache.read(short, cancelled, modePrice, "qualified-exact-v2", nil, source, nil); !errors.Is(err, context.DeadlineExceeded) || calls.Load() != priorCalls+1 {
		t.Fatalf("waiting reader cancellation: calls=%d, err=%v", calls.Load(), err)
	}
	if err := rdb.Close(); err != nil {
		t.Fatal(err)
	}
	withoutRedis, err := NewHistoryCache(rdb, PostgresHistoryRevision{Pool: pool, Schema: "origin_revision"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	fallback, err := withoutRedis.read(ctx, request, modePrice, "qualified-exact-v2", nil, source, nil)
	if err != nil || fallback.cacheState != cacheMiss || fallback.Policy != "private, no-cache" || !validChartCache(fallback, fallback.Scope, request, modePrice, "qualified-exact-v2") {
		t.Fatalf("canonical origin after cache disconnect: %+v, %v", fallback, err)
	}
	standalone, err := NewHistoryCache(nil, PostgresHistoryRevision{Pool: pool, Schema: "origin_revision"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	handler := ProductionHistoryHandler{Source: source, Cache: standalone, Algorithm: "qualified-exact-v2"}
	full := httptest.NewRecorder()
	handler.ServeHTTP(full, httptest.NewRequest(http.MethodGet, requestURL, nil))
	if full.Code != http.StatusOK || full.Header().Get("X-Chart-Cache") != cacheMiss || full.Header().Get("Server-Timing") == "" || !strings.Contains(full.Body.String(), `"s":"no_data"`) {
		t.Fatalf("origin history response: %d %v %s", full.Code, full.Header(), full.Body.String())
	}
	malformed := httptest.NewRequest(http.MethodGet, requestURL, nil)
	malformed.URL.RawQuery = "from=%ZZ"
	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, malformed)
	if invalid.Code != http.StatusUnprocessableEntity || !strings.Contains(invalid.Body.String(), "malformed history query") {
		t.Fatalf("malformed history query: %d %s", invalid.Code, invalid.Body.String())
	}
}
