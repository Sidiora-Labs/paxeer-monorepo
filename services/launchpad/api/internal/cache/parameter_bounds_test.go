package cache

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestRoutePolicyRejectsInvalidAllocations(t *testing.T) {
	valid := RoutePolicy{ID: "price", Version: "v2", Origin: "postgres", Generation: "g1", Class: PublicSnapshot, FreshTTL: time.Minute, RequestLimit: 10, RequestWindow: time.Minute}
	for _, change := range []func(*RoutePolicy){
		func(p *RoutePolicy) { p.ID = "bad id" },
		func(p *RoutePolicy) { p.Class = "unknown" },
		func(p *RoutePolicy) { p.Class = Authorization },
		func(p *RoutePolicy) { p.Class = PrivateAccount; p.StaleTTL = time.Second },
		func(p *RoutePolicy) { p.Parameters = map[string]ParameterRule{"x": {MaxBytes: 0}} },
		func(p *RoutePolicy) { p.Parameters = map[string]ParameterRule{"x": {MaxBytes: 10, Pattern: "["}} },
	} {
		p := valid
		change(&p)
		if err := p.validate(); err == nil {
			t.Fatalf("invalid policy accepted: %+v", p)
		}
	}
	if err := valid.validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := valid.normalize(url.Values{"x": {strings.Repeat("a", 4097)}}); err == nil {
		t.Fatal("oversized query accepted")
	}
	for class, want := range map[RouteClass]string{PublicSnapshot: "anonymous_read", ExecutableQuote: "quote", AccountWrite: "account_write", Administrative: "admin", Ownership: "account_read"} {
		p := valid
		p.Class = class
		if got := p.metricClass(); got != want {
			t.Fatalf("class %s: %s", class, got)
		}
	}
}

func TestCacheRefreshAdmissionAndCounters(t *testing.T) {
	ctx := context.Background()
	c := New(8)
	fetch := func(context.Context) ([]byte, error) { return []byte(`{"value":1}`), nil }
	for _, origin := range []string{"", strings.Repeat("x", 129), "x\n"} {
		if err := c.StartOrigin(ctx, origin, "key", fetch); err == nil {
			t.Fatal("invalid origin accepted")
		}
	}
	if err := c.StartOrigin(ctx, "postgres", strings.Repeat("x", 4096), fetch); err == nil {
		t.Fatal("oversized key accepted")
	}
	if err := c.StartOrigin(ctx, "postgres", "key", nil); err == nil {
		t.Fatal("missing provider accepted")
	}
	if _, _, err := c.GetOrFetchOrigin(ctx, "postgres", strings.Repeat("x", 4096), 0, fetch); err == nil {
		t.Fatal("oversized read key accepted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := c.GetOrFetch(cancelled, "key", 0, fetch); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled read: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, _, err := c.GetOrFetch(ctx, "key", time.Minute, fetch); err != nil {
			t.Fatal(err)
		}
	}
	if hits, misses := c.Stats(); hits != 1 || misses != 1 {
		t.Fatalf("cache counters: %d %d", hits, misses)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	defer close(release)
	blocked := func(context.Context) ([]byte, error) {
		close(entered)
		<-release
		close(finished)
		return nil, errors.New("origin unavailable")
	}
	c.maxPerOrigin = 1
	if err := c.StartOrigin(ctx, "refresh", "first", blocked); err != nil {
		t.Fatal(err)
	}
	<-entered
	if err := c.StartOrigin(ctx, "refresh", "first", fetch); err != nil {
		t.Fatalf("duplicate refresh: %v", err)
	}
	if err := c.StartOrigin(ctx, "refresh", "second", fetch); !errors.Is(err, ErrOriginBusy) {
		t.Fatalf("refresh budget: %v", err)
	}
}

func TestCanonicalCacheParameterRepresentation(t *testing.T) {
	request := store.AccountingPageRequest{Pool: repairM992Pool, Limit: 20, Side: "sell", Interval: "1m", From: 0, To: 60000}
	trades := canonicalParameters(request, url.Values{"cursor": {"token"}}, false)
	if trades.Get("side") != "sell" || trades.Get("cursor") != "token" || trades.Get("limit") != "20" {
		t.Fatalf("trade parameters: %v", trades)
	}
	candles := canonicalParameters(request, nil, true)
	if candles.Get("from") != "1970-01-01T00:00:00.000Z" || candles.Get("to") != "1970-01-01T00:01:00.000Z" || candles.Get("interval") != "1m" {
		t.Fatalf("candle parameters: %v", candles)
	}
	for _, tc := range []struct {
		a, b  string
		equal bool
	}{{`{"x":1}`, `{"x":1}`, true}, {`{`, `{}`, false}, {`{}`, `{`, false}} {
		if got := sameProjectionJSON([]byte(tc.a), []byte(tc.b)); got != tc.equal {
			t.Fatalf("JSON equality %q %q: %t", tc.a, tc.b, got)
		}
	}
	if _, err := PublicNetworkScope(nil); !errors.Is(err, ErrAdmissionUnavailable) {
		t.Fatalf("missing peer: %v", err)
	}
	for _, c := range []CanonicalAllocation{{}, {FreshTTL: time.Minute, RequestLimit: 0, RequestWindow: time.Minute}} {
		if _, err := CanonicalPolicies("g1", "postgres", c); err == nil {
			t.Fatal("invalid canonical allocation accepted")
		}
	}
}

func TestAdapterRejectsIncompleteReadDependencies(t *testing.T) {
	runtime := repairM992Runtime(t, repairM992Config(t, 0))
	a := runtime.adapter
	ctx := context.Background()
	scope := VerifiedScope{NetworkID: "127.0.0.1"}
	if _, _, err := a.Read(ctx, "missing", nil, scope, nil); err == nil {
		t.Fatal("unknown route accepted")
	}
	if _, _, err := a.Read(ctx, literalAccountingTrades, nil, scope, nil); err == nil {
		t.Fatal("missing parameters accepted")
	}
	p, _ := a.policy(literalAccountingTrades)
	q := url.Values{"poolAddress": {repairM992Pool}, "limit": {"1"}, "sourceRevision": {"unobserved"}}
	if _, _, err := a.Read(ctx, p.ID, q, scope, nil); err == nil {
		t.Fatal("missing projection provider accepted")
	}
	a.policies["write"] = RoutePolicy{ID: "write", Class: AccountWrite}
	if _, _, err := a.Read(ctx, "write", nil, scope, nil); err == nil {
		t.Fatal("mutation admitted as read")
	}
	if err := a.Prewarm(ctx, "missing", []url.Values{nil}, scope, nil); err == nil {
		t.Fatal("unknown prewarm route accepted")
	}
	if err := a.Invalidate(ctx, "missing", nil, scope, "g1", "1"); err == nil {
		t.Fatal("unknown invalidation route accepted")
	}
	if err := a.Invalidate(ctx, p.ID, url.Values{"extra": {"x"}}, scope, "g1", "1"); err == nil {
		t.Fatal("invalid invalidation parameters accepted")
	}
	if err := a.Invalidate(ctx, p.ID, q, scope, "g1", "invalid"); err == nil {
		t.Fatal("malformed invalidation revision accepted")
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response.Header().Set("X-Request-ID", "00000000-0000-4000-8000-000000000001")
	accountingRuntimeError(response, request, &LimitError{RetryAfter: time.Second})
	if response.Code != 429 || response.Header().Get("Retry-After") != "1" {
		t.Fatalf("rate response: %d %v", response.Code, response.Header())
	}
	if _, err := (AccountingProjectionSource{Store: runtime.bindings.config.Model, Generation: "g1", Request: func(ProjectionRequest) (store.AccountingPageRequest, error) {
		return store.AccountingPageRequest{}, store.ErrCursorInvalid
	}, Encode: func(store.AccountingPage) ([]byte, error) { t.Fatal("invalid request encoded"); return nil, nil }}).Fetch(ctx, ProjectionRequest{Generation: "g1"}); !errors.Is(err, store.ErrCursorInvalid) {
		t.Fatalf("projection request: %v", err)
	}
}
