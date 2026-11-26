package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/redis/go-redis/v9"
)

func untradedHistoryResponse() interfaces.CandlesResponse {
	return interfaces.CandlesResponse{
		Data:        interfaces.CandlesData{Items: []interfaces.Candle{}, SeriesModelVersion: engine.SeriesAlgorithm, SnapshotId: "11111111-1111-4111-8111-111111111111"},
		GeneratedAt: "2026-10-12T00:00:00.000Z", Quality: "partial", RequestId: "22222222-2222-4222-8222-222222222222",
	}
}

func untradedHistoryWire(t *testing.T, response interfaces.CandlesResponse) []byte {
	t.Helper()
	wire, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if err = interfaces.ValidateJSON("CandlesResponse", wire); err != nil {
		t.Fatalf("candle page contract: %v: %s", err, wire)
	}
	return wire
}

func untradedHistoryCache(t *testing.T, rdb *redis.Client) *HistoryCache {
	t.Helper()
	pool := historyPostgres(t)
	if _, err := pool.Exec(context.Background(), `CREATE SCHEMA untraded_history;
CREATE TABLE untraded_history.chart_exact(chain_id text,pool text,interval text,start bigint,snapshot jsonb);
CREATE VIEW untraded_history.chart_exact_v1 AS SELECT * FROM untraded_history.chart_exact`); err != nil {
		t.Fatal(err)
	}
	cache, err := NewHistoryCache(rdb, PostgresHistoryRevision{Pool: pool, Schema: "untraded_history"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	return cache
}

func TestUntradedHistoryRead(t *testing.T) {
	for _, quality := range []string{"partial", "complete"} {
		t.Run(quality, func(t *testing.T) {
			response := untradedHistoryResponse()
			response.Quality = quality
			wire := untradedHistoryWire(t, response)
			var calls atomic.Int64
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path != "/v2/pools/"+udfPool+"/candles" || r.URL.Query().Get("interval") != "1m" || r.URL.Query().Has("cursor") {
					t.Error("unexpected candle request", r.URL)
				}
				_, _ = w.Write(wire)
			}))
			t.Cleanup(origin.Close)
			page, err := (CanonicalCandleClient{Endpoint: origin.URL}).Read(context.Background(), HistoryRequest{Symbol: udfPool, Timeframe: "1m", From: 0, To: 120})
			if err != nil || page.Items == nil || len(page.Items) != 0 || page.Watermark != nil || page.Complete || page.SnapshotID != response.Data.SnapshotId || page.Algorithm != response.Data.SeriesModelVersion || calls.Load() != 1 {
				t.Fatalf("untraded candle page: %+v calls=%d err=%v", page, calls.Load(), err)
			}
		})
	}
}

func TestUntradedHistoryProductionRoute(t *testing.T) {
	wire := untradedHistoryWire(t, untradedHistoryResponse())
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(wire) }))
	t.Cleanup(origin.Close)
	var supplyCalls atomic.Int64
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		supplyCalls.Add(1)
		http.Error(w, "supply is not needed for empty history", http.StatusServiceUnavailable)
	}))
	t.Cleanup(rpc.Close)
	client, err := ethclient.Dial(rpc.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	cache := untradedHistoryCache(t, nil)
	request := HistoryRequest{Symbol: udfPool, Timeframe: "1m", From: 0, To: 120}
	version, err := cache.Revision.Read(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })
	handler := ProductionHistoryHandler{
		Source: CanonicalCandleClient{Endpoint: origin.URL}, Cache: cache, Algorithm: engine.SeriesAlgorithm,
		Supply: &RPCSupplySource{Chain: &chainread.Reader{RPC: client}}, Logger: logger,
		HistoricalPermit: &HistoricalCachePermit{RevisionDigest: version, InvalidationEvidenceID: "historical-candles", Complete: true, MaxAge: time.Minute},
	}
	router := sharedhttp.NewRouter(sharedhttp.ServerOptions{})
	RegisterProductionUDF(router, SymbolsHandler{}, handler)
	for _, mode := range []string{modePrice, modeTotalSupplyValuation} {
		for _, conditional := range []bool{false, true} {
			req := httptest.NewRequest(http.MethodGet, "/v2/udf"+historyURL("from=0&to=120&mode="+mode), nil)
			first := httptest.NewRecorder()
			router.ServeHTTP(first, req)
			assertUntradedHistory(t, first, mode)
			if conditional {
				req.Header.Set("If-None-Match", first.Header().Get("ETag"))
				next := httptest.NewRecorder()
				router.ServeHTTP(next, req)
				if next.Code != http.StatusNotModified || next.Header().Get("Cache-Control") != "private, no-cache" {
					t.Fatal(next.Code, next.Header(), next.Body.String())
				}
			}
		}
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.entries) != 2 {
		t.Fatal("each mode retains its own empty response", len(cache.entries))
	}
	for key, value := range cache.entries {
		if value.Policy != "private, no-cache" || value.Expiry.After(time.Now().Add(500*time.Millisecond)) || !validChartCache(value, key, request, modeFromHistoryBody(t, value.Body), engine.SeriesAlgorithm) {
			t.Fatal("untraded cache bounds", value)
		}
	}
	if supplyCalls.Load() != 0 || strings.Contains(logs.String(), `"level":"ERROR"`) || strings.Contains(logs.String(), `"level":"WARN"`) {
		t.Fatal("empty history dependencies and logs", supplyCalls.Load(), logs.String())
	}
}

func modeFromHistoryBody(t *testing.T, raw []byte) string {
	t.Helper()
	var body struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	return body.Mode
}

func assertUntradedHistory(t *testing.T, response *httptest.ResponseRecorder, mode string) {
	t.Helper()
	var body struct {
		Status    string          `json:"s"`
		Watermark json.RawMessage `json:"watermark"`
		Snapshot  string          `json:"snapshotId"`
		Algorithm string          `json:"algorithmVersion"`
		Mode      string          `json:"mode"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || body.Status != statusNoData || body.Mode != mode || string(body.Watermark) != "null" || response.Header().Get("Cache-Control") != "private, no-cache" || response.Header().Get("ETag") == "" {
		t.Fatal(response.Code, response.Header(), response.Body.String())
	}
	expected, err := valuationHistory(context.Background(), CanonicalCandlePage{Items: []interfaces.Candle{}, SnapshotID: body.Snapshot, Algorithm: body.Algorithm}, nil, mode)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(expected, response.Body.Bytes()) {
		t.Fatalf("%s: empty history %s != canonical %s", body.Mode, response.Body.String(), expected)
	}
}

func untradedHistoryTradedResponse(t *testing.T) interfaces.CandlesResponse {
	t.Helper()
	frame, err := interfaces.DecodeCandleEvent(webhookDeliveryCandleFrames(t)[0])
	if err != nil {
		t.Fatal(err)
	}
	response := untradedHistoryResponse()
	response.Data.Items = []interfaces.Candle{frame.Payload}
	response.Data.SnapshotId = frame.Payload.SnapshotId
	revision := frame.Payload.Revision
	response.Revision = &revision
	response.Watermark = &interfaces.Watermark{ChainId: "125", Height: "103", BlockHash: frame.Payload.LastPosition.BlockHash, Integrity: "healthy", ObservedAt: response.GeneratedAt, ProjectionRevision: "1"}
	return response
}

func TestUntradedHistoryDependencyFailures(t *testing.T) {
	traded := untradedHistoryTradedResponse(t)
	stamp, err := time.Parse("2006-01-02T15:04:05.000Z", traded.Data.Items[0].Start)
	if err != nil {
		t.Fatal(err)
	}
	cache := untradedHistoryCache(t, nil)
	for _, tc := range []struct {
		name         string
		change       func(*interfaces.CandlesResponse)
		continuation bool
		status       int
		raw          string
	}{
		{name: "unhealthy watermark", change: func(p *interfaces.CandlesResponse) { p.Watermark.Integrity = "quarantined" }},
		{name: "watermark without revision", change: func(p *interfaces.CandlesResponse) { p.Data.Items = []interfaces.Candle{}; p.Revision = nil }},
		{name: "items without watermark", change: func(p *interfaces.CandlesResponse) { p.Watermark = nil; p.Revision = nil }},
		{name: "revision without watermark", change: func(p *interfaces.CandlesResponse) { p.Data.Items = []interfaces.Candle{}; p.Watermark = nil }},
		{name: "stale empty page", change: func(p *interfaces.CandlesResponse) { *p = untradedHistoryResponse(); p.Quality = "stale" }},
		{name: "quarantined empty page", change: func(p *interfaces.CandlesResponse) { *p = untradedHistoryResponse(); p.Quality = "quarantined" }},
		{name: "continuation without watermark", continuation: true, change: func(p *interfaces.CandlesResponse) {
			p.Data.Items = []interfaces.Candle{}
			p.Watermark = nil
			p.Revision = nil
		}},
		{name: "untraded page with cursor", change: func(p *interfaces.CandlesResponse) {
			*p = untradedHistoryResponse()
			cursor := "next"
			p.NextCursor = &cursor
		}},
		{name: "wrong algorithm", change: func(p *interfaces.CandlesResponse) {
			*p = untradedHistoryResponse()
			p.Data.SeriesModelVersion = "another-model"
		}},
		{name: "unavailable origin", status: http.StatusServiceUnavailable},
		{name: "unknown pool", status: http.StatusNotFound},
		{name: "malformed page", raw: "{"},
		{name: "missing page fields", raw: `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := traded
			watermark := *traded.Watermark
			response.Watermark = &watermark
			if tc.change != nil {
				tc.change(&response)
			}
			wire := untradedHistoryWire(t, response)
			first := traded
			first.Data.Items = []interfaces.Candle{}
			cursor := "next"
			first.NextCursor = &cursor
			firstWire := untradedHistoryWire(t, first)
			var calls atomic.Int64
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if tc.status != 0 {
					w.WriteHeader(tc.status)
					return
				}
				if tc.raw != "" {
					_, _ = w.Write([]byte(tc.raw))
					return
				}
				if tc.continuation && !r.URL.Query().Has("cursor") {
					_, _ = w.Write(firstWire)
					return
				}
				_, _ = w.Write(wire)
			}))
			t.Cleanup(origin.Close)
			handler := ProductionHistoryHandler{Source: CanonicalCandleClient{Endpoint: origin.URL}, Cache: cache, Algorithm: engine.SeriesAlgorithm}
			request := HistoryRequest{Symbol: traded.Data.Items[0].PoolAddress, Timeframe: "1m", From: stamp.Unix(), To: stamp.Unix() + 60}
			query := untradedHistoryURL(request)
			for _, mode := range []string{modePrice, modeTotalSupplyValuation} {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, query+"&mode="+mode, nil))
				if rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "no_data") || !strings.Contains(rec.Body.String(), `"retryable":true`) || !strings.Contains(rec.Body.String(), "DEPENDENCY_UNAVAILABLE") {
					t.Fatal(rec.Code, rec.Body.String())
				}
			}
			wantCalls := int64(2)
			if tc.continuation {
				wantCalls = 4
			}
			if calls.Load() != wantCalls {
				t.Fatal("dependency request count", calls.Load(), wantCalls)
			}
		})
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.entries) != 0 {
		t.Fatal("dependency failures cannot fill the cache", len(cache.entries))
	}
}

func untradedHistoryURL(r HistoryRequest) string {
	return "/v2/udf/history?symbol=" + r.Symbol + "&resolution=1&from=" + strconv.FormatInt(r.From, 10) + "&to=" + strconv.FormatInt(r.To, 10)
}

func TestUntradedHistoryFirstTrade(t *testing.T) {
	traded := untradedHistoryTradedResponse(t)
	bar := traded.Data.Items[0]
	stamp, err := time.Parse("2006-01-02T15:04:05.000Z", bar.Start)
	if err != nil {
		t.Fatal(err)
	}
	empty := untradedHistoryResponse()
	emptyWire, tradedWire := untradedHistoryWire(t, empty), untradedHistoryWire(t, traded)
	for _, invalidation := range []string{"expiry", "revision"} {
		t.Run(invalidation, func(t *testing.T) {
			options, err := redis.ParseURL(internaltest.NewRedisURL(t))
			if err != nil {
				t.Fatal(err)
			}
			rdb := redis.NewClient(options)
			t.Cleanup(func() { _ = rdb.Close() })
			cache := untradedHistoryCache(t, rdb)
			var committed atomic.Bool
			var calls atomic.Int64
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				if committed.Load() {
					_, _ = w.Write(tradedWire)
				} else {
					_, _ = w.Write(emptyWire)
				}
			}))
			t.Cleanup(origin.Close)
			r := HistoryRequest{Symbol: bar.PoolAddress, Timeframe: bar.Interval, From: stamp.Unix(), To: stamp.Unix() + 60}
			source := CanonicalCandleClient{Endpoint: origin.URL}
			handler := ProductionHistoryHandler{Source: source, Cache: cache, Algorithm: engine.SeriesAlgorithm}
			first := httptest.NewRecorder()
			handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, untradedHistoryURL(r), nil))
			assertUntradedHistory(t, first, modePrice)
			for _, restart := range []bool{false, true} {
				if restart {
					cache, err = NewHistoryCache(rdb, cache.Revision, 2)
					if err != nil {
						t.Fatal(err)
					}
					handler.Cache = cache
				}
				hit := httptest.NewRecorder()
				handler.ServeHTTP(hit, httptest.NewRequest(http.MethodGet, untradedHistoryURL(r), nil))
				assertUntradedHistory(t, hit, modePrice)
				if hit.Header().Get("X-Chart-Cache") != cacheHit || hit.Header().Get("ETag") != first.Header().Get("ETag") || calls.Load() != 1 {
					t.Fatal("shared empty history", hit.Header(), calls.Load())
				}
			}
			before, err := cache.Revision.Read(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			committed.Store(true)
			if invalidation == "revision" {
				if _, err := cache.Revision.Pool.Exec(context.Background(), `INSERT INTO untraded_history.chart_exact VALUES('125',$1,'1m',$2,'{"revision":"1"}')`, r.Symbol, r.From); err != nil {
					t.Fatal(err)
				}
			} else {
				time.Sleep(550 * time.Millisecond)
			}
			after, err := cache.Revision.Read(context.Background(), r)
			if err != nil || (before == after) != (invalidation == "expiry") {
				t.Fatal("revision identity", before, after, err)
			}
			req := httptest.NewRequest(http.MethodGet, untradedHistoryURL(r), nil)
			req.Header.Set("If-None-Match", first.Header().Get("ETag"))
			next := httptest.NewRecorder()
			handler.ServeHTTP(next, req)
			var body struct {
				Status string    `json:"s"`
				Times  []int64   `json:"t"`
				Close  []float64 `json:"c"`
				Volume []float64 `json:"v"`
			}
			if err := json.Unmarshal(next.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			closePrice, err := exactPrice(bar.Close)
			if err != nil {
				t.Fatal(err)
			}
			closeValue, err := vendorRational(closePrice)
			if err != nil {
				t.Fatal(err)
			}
			if next.Code != http.StatusOK || body.Status != "ok" || len(body.Times) != 1 || body.Times[0] != stamp.Unix() || len(body.Close) != 1 || body.Close[0] != closeValue || len(body.Volume) != 1 || body.Volume[0] <= 0 || next.Header().Get("ETag") == first.Header().Get("ETag") || calls.Load() != 2 {
				t.Fatal("first trade history", next.Code, next.Header(), next.Body.String(), calls.Load())
			}
		})
	}
}

func TestUntradedHistoryCacheBounds(t *testing.T) {
	request := HistoryRequest{Symbol: udfPool, Timeframe: "1m", From: 0, To: 120}
	page := CanonicalCandlePage{Items: []interfaces.Candle{}, SnapshotID: untradedHistoryResponse().Data.SnapshotId, Algorithm: engine.SeriesAlgorithm}
	body, err := valuationHistory(context.Background(), page, nil, modePrice)
	if err != nil {
		t.Fatal(err)
	}
	value := chartCacheValue{Scope: "history", Body: body, Watermark: []byte("null"), Policy: "private, no-cache", Expiry: time.Now().Add(500 * time.Millisecond)}
	value.ETag = chartETag(value.Scope, value.Watermark, value.Body)
	if !validChartCache(value, value.Scope, request, modePrice, engine.SeriesAlgorithm) {
		t.Fatal("empty history cache rejected")
	}
	for _, tc := range []struct {
		name   string
		change func(*chartCacheValue)
	}{
		{"public policy", func(v *chartCacheValue) { v.Policy = "public, max-age=1, must-revalidate" }},
		{"long lifetime", func(v *chartCacheValue) { v.Expiry = time.Now().Add(time.Second) }},
		{"mismatched watermark", func(v *chartCacheValue) { v.Watermark = []byte(`{}`) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := value
			tc.change(&candidate)
			candidate.ETag = chartETag(candidate.Scope, candidate.Watermark, candidate.Body)
			if validChartCache(candidate, value.Scope, request, modePrice, engine.SeriesAlgorithm) {
				t.Fatal("untraded cache bounds admitted", tc.name)
			}
		})
	}
	for _, field := range []string{"s", "snapshotId", "algorithmVersion", "mode", "watermark", "t", "o", "h", "l", "c", "v"} {
		t.Run(field, func(t *testing.T) {
			var fields map[string]any
			if err := json.Unmarshal(body, &fields); err != nil {
				t.Fatal(err)
			}
			delete(fields, field)
			candidate := value
			candidate.Body, err = json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			candidate.ETag = chartETag(candidate.Scope, candidate.Watermark, candidate.Body)
			if validChartCache(candidate, value.Scope, request, modePrice, engine.SeriesAlgorithm) {
				t.Fatal("incomplete empty history admitted", field)
			}
		})
	}
}
