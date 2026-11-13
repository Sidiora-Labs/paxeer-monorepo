package cache

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestAccountingRuntimeLifecycleAndPublicErrors(t *testing.T) {
	runtime := repairM992Runtime(t, repairM992Config(t, 0))
	if err := runtime.Ready(context.Background()); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("runtime ready before consumer: %v", err)
	}
	if err := runtime.Start(nil, nil); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("nil runtime context: %v", err)
	}
	if err := runtime.Run(nil); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("nil consumer context: %v", err)
	}
	if err := runtime.Mount(nil); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("nil router: %v", err)
	}
	router := chi.NewRouter()
	if err := runtime.Mount(router); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/pools/"+repairM992Pool+"/trades", nil))
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"code":"DEPENDENCY_UNAVAILABLE"`) {
		t.Fatalf("unready runtime response: %d %s", response.Code, response.Body.String())
	}
	if err := runtime.Shutdown(nil); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("nil shutdown context: %v", err)
	}
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown before run: %v", err)
	}
	if err := runtime.Start(context.Background(), nil); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("closed runtime restarted: %v", err)
	}
	if err := runtime.Run(context.Background()); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("closed runtime ran: %v", err)
	}
	for _, tc := range []struct {
		name   string
		err    error
		query  string
		status int
		code   string
	}{
		{"fields", rest.ErrAccountingFields, "", 422, "FIELD_INVALID"},
		{"cursor", store.ErrCursorInvalid, "", 400, "CURSOR_INVALID"},
		{"integrity", store.ErrDataIntegrity, "", 503, "INTEGRITY_QUARANTINED"},
		{"projection", ErrProjectionChanged, "", 503, "INTEGRITY_QUARANTINED"},
		{"dependency", store.ErrAccountingNotReady, "", 503, "DEPENDENCY_UNAVAILABLE"},
		{"stale continuation", store.ErrAccountingNotReady, "?cursor=token", 400, "CURSOR_INVALID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := httptest.NewRecorder()
			accountingRuntimeError(record, httptest.NewRequest(http.MethodGet, "/v2/pools/"+repairM992Pool+"/trades"+tc.query, nil), tc.err)
			var body rest.APIError
			if err := json.Unmarshal(record.Body.Bytes(), &body); err != nil || record.Code != tc.status || body.Code != tc.code {
				t.Fatalf("public error: status=%d body=%+v parse=%v", record.Code, body, err)
			}
		})
	}
}

func TestAccountingRuntimeStartsConsumerAndStopsCleanly(t *testing.T) {
	config := repairM992Config(t, 0)
	runtime := repairM992Runtime(t, config)
	connection, js := marketTestBroker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: config.MarketConsumer.Stream, Subjects: []string{accounting.MarketSubject}, Storage: jetstream.FileStorage, Replicas: 1}); err != nil {
		t.Fatal(err)
	}
	approved, err := config.MarketConsumer.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateConsumer(ctx, config.MarketConsumer.Stream, approved); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Start(ctx, connection); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	if err := runtime.Start(ctx, connection); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("duplicate consumer start: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- runtime.Run(ctx) }()
	for runtime.Ready(ctx) != nil {
		select {
		case err := <-done:
			t.Fatalf("consumer stopped before readiness: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
	if err := runtime.Run(ctx); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("duplicate consumer run: %v", err)
	}
	cancel()
	shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if err := runtime.Shutdown(shutdown); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("consumer stopped: %v", err)
	}
	if err := runtime.Ready(shutdown); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("closed consumer ready: %v", err)
	}
}
