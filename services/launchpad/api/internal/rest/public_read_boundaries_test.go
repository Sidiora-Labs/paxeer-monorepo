package rest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestPublicCacheRejectsInvalidRequestsAndProviderPanics(t *testing.T) {
	if _, err := NewPublicCache(0, 1, 1, time.Millisecond); err == nil {
		t.Fatal("zero cache capacity admitted")
	}
	c, err := NewPublicCache(1024, 2, 1, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	fetch := func(context.Context) ([]byte, error) { return []byte(`{"value":1}`), nil }
	for _, tc := range []struct {
		ctx   context.Context
		key   string
		ttl   time.Duration
		fetch func(context.Context) ([]byte, error)
	}{
		{nil, "key", time.Minute, fetch},
		{context.Background(), strings.Repeat("x", 513), time.Minute, fetch},
		{context.Background(), "key", 0, fetch},
		{context.Background(), "key", time.Minute, nil},
	} {
		if _, err := c.GetOrFetch(tc.ctx, tc.key, tc.ttl, tc.fetch); err == nil {
			t.Fatal("invalid cache request admitted")
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.GetOrFetch(cancelled, "key", time.Minute, fetch); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled cache request: %v", err)
	}
	if c.Metrics().Cancelled != 1 {
		t.Fatal("cancelled request not counted")
	}
	if body, err := c.GetOrFetch(context.Background(), "panic", time.Minute, func(context.Context) ([]byte, error) { panic("provider panic") }); err == nil || body != nil {
		t.Fatalf("provider panic result: %q %v", body, err)
	}
}

func TestProjectionHandlersRejectUnavailableStores(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/v2/pools/{poolAddress}", poolDetail(ProductionReadOptions{}))
	router.Get("/v2/pools", poolList(ProductionReadOptions{}))
	router.Get("/v2/pools/{poolAddress}/stats", projectionStats(nil))
	router.Post("/v2/pools/stats:batch", projectionStatsBatch(nil))
	router.Get("/v2/platform/metrics", platformMetricsV2(nil))
	for _, path := range []string{"/v2/pools/0x1111111111111111111111111111111111111111", "/v2/pools?sort=new", "/v2/pools/0x1111111111111111111111111111111111111111/stats", "/v2/platform/metrics?window=24h"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 503 || !strings.Contains(w.Body.String(), "DEPENDENCY_UNAVAILABLE") {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v2/pools/stats:batch", strings.NewReader(`{"poolAddresses":["0x1111111111111111111111111111111111111111"]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("batch source: %d %s", w.Code, w.Body.String())
	}
	for _, raw := range []string{`{"poolAddresses":["0x1111111111111111111111111111111111111111"]} {}`, `{"poolAddresses":["0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"]}`} {
		req := httptest.NewRequest(http.MethodPost, "/batch", strings.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		pools, ok := statsBatchRequest(w, req)
		if strings.HasSuffix(raw, " {}") {
			if ok || w.Code != 400 {
				t.Fatalf("trailing body accepted: %d %v", w.Code, pools)
			}
		} else if !ok || len(pools) != 1 {
			t.Fatalf("duplicate pools: %t %v", ok, pools)
		}
	}
	if _, err := NewAdmission(0, 0, time.Millisecond); err == nil {
		t.Fatal("invalid admission capacity accepted")
	}
	if _, _, err := admissionCost(AdmissionClass(255)); err == nil {
		t.Fatal("unknown admission class accepted")
	}
	if _, err := (*Admission)(nil).Acquire(context.Background(), AdmissionCached); err == nil {
		t.Fatal("unconfigured admission accepted")
	}
}

func TestResponseMetadataRejectsMalformedAndRepeatedQueryFields(t *testing.T) {
	handler := ResponseMetadata(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Deadline(); !ok {
			t.Error("read deadline missing")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, tc := range []struct {
		path    string
		status  int
		version string
	}{
		{"/v1/pools", 204, "1"},
		{"/v2/pools", 204, "2"},
		{"/v2/pools?x=%zz", 400, "2"},
		{"/v2/pools?x=1&x=2", 400, "2"},
		{"/v2/pools?x=" + strings.Repeat("a", 8193), 400, "2"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if response.Code != tc.status || response.Header().Get("X-Schema-Version") != tc.version {
			t.Fatalf("metadata %s: %d %v", tc.path, response.Code, response.Header())
		}
	}
}
