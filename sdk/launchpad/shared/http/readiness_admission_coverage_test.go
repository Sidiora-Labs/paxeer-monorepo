package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReadinessRejectsInvalidRegistrationsAndPanics(t *testing.T) {
	healthy := func(context.Context) error { return nil }
	for _, custom := range []map[string]Check{
		{"database": healthy}, {"lifecycle": healthy}, {"invalid check": healthy}, {"probe": func(context.Context) error { panic("probe failed") }},
	} {
		response := httptest.NewRecorder()
		readyHandler(HealthDeps{DB: healthy, Redis: healthy, Custom: custom, State: healthy})(response, httptest.NewRequest("GET", "/health/ready", nil))
		if response.Code != 503 || !strings.Contains(response.Body.String(), `"status":"degraded"`) {
			t.Fatalf("invalid readiness admitted: %d %s", response.Code, response.Body.String())
		}
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	handler := readyHandler(HealthDeps{DB: func(context.Context) error { once.Do(func() { close(entered) }); <-release; return nil }, Redis: healthy})
	ctx, cancel := context.WithCancel(context.Background())
	first := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler(first, httptest.NewRequest("GET", "/health/ready", nil).WithContext(ctx))
	}()
	<-entered
	second := httptest.NewRecorder()
	handler(second, httptest.NewRequest("GET", "/health/ready", nil))
	if second.Code != 503 || !strings.Contains(second.Body.String(), `"database":"busy"`) {
		close(release)
		cancel()
		<-done
		t.Fatalf("concurrent readiness %+v", second)
	}
	cancel()
	<-done
	close(release)
	if first.Code != 503 || !strings.Contains(first.Body.String(), `"database":"timeout"`) {
		t.Fatalf("cancelled readiness %d %s", first.Code, first.Body.String())
	}
}

func TestLocalRequestBudgetBoundsMemoryAndExpiresEntries(t *testing.T) {
	store := newMemStore()
	store.lastSweep = time.Now().Add(-2 * time.Minute)
	store.entries["expired"] = memEntry{count: 2, resetAt: time.Now().Add(-time.Second)}
	store.entries["live"] = memEntry{count: 1, resetAt: time.Now().Add(time.Minute)}
	if ok, remaining := store.check("live", 2, time.Minute); !ok || remaining != 0 {
		t.Fatalf("live allowance %t %d", ok, remaining)
	}
	if _, exists := store.entries["expired"]; exists {
		t.Fatal("expired request budget retained")
	}
	if ok, _ := store.check(strings.Repeat("x", 513), 2, time.Minute); ok {
		t.Fatal("unbounded budget identity admitted")
	}
	for i := len(store.entries); i < 10000; i++ {
		store.entries[fmt.Sprint(i)] = memEntry{resetAt: time.Now().Add(time.Minute)}
	}
	if ok, _ := store.check("new-identity", 2, time.Minute); ok {
		t.Fatal("unbounded budget cardinality admitted")
	}
	store.entries["renew"] = memEntry{count: 10, resetAt: time.Now().Add(-time.Second)}
	if ok, remaining := store.check("renew", 2, time.Minute); !ok || remaining != 1 {
		t.Fatalf("expired allowance %t %d", ok, remaining)
	}
	middleware, err := APIKeyAuth(APIKeyOptions{APIKey: "private-fixture-key", RoutePrefix: "/admin"})
	if err != nil {
		t.Fatal(err)
	}
	called := 0
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called++; w.WriteHeader(204) }))
	for _, tc := range []struct {
		path, key string
		status    int
	}{{"/public", "", 204}, {"/admin", "", 401}, {"/admin", strings.Repeat("x", 4097), 401}, {"/admin", "private-fixture-key", 204}} {
		request := httptest.NewRequest("GET", tc.path, nil)
		request.Header.Set("X-API-Key", tc.key)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("API key boundary %d", response.Code)
		}
	}
	if called != 2 {
		t.Fatalf("protected handler invoked %d times", called)
	}
	response := httptest.NewRecorder()
	WriteRateLimited(response, 0, "budget exhausted")
	var body map[string]any
	if response.Code != 429 || response.Header().Get("Retry-After") != "1" || json.Unmarshal(response.Body.Bytes(), &body) != nil {
		t.Fatalf("minimum retry %d %s", response.Code, response.Body.String())
	}
	if itoa(0) != "0" {
		t.Fatal("zero memory counter changed")
	}
}
