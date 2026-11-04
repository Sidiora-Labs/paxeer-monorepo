package cache

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type AccountingRuntimeConfig struct {
	Adapter             AdapterConfig
	ReadModel           store.AccountingReadConfig
	Allocation          CanonicalAllocation
	Origin              string
	MarketConsumer      natsdriver.ConsumerConfig
	SigningKey          []byte
	PreviousSigningKeys [][]byte
	Admission           *rest.Admission
}

// AccountingRuntime borrows its PostgreSQL, Redis and NATS connections. The app
// starts the durable consumer before mounting routes, and closes this runtime
// before closing those connections.
type AccountingRuntime struct {
	adapter        *Adapter
	bindings       *AccountingBindings
	admission      *rest.Admission
	consumerConfig natsdriver.ConsumerConfig
	mu             sync.Mutex
	consumer       *natsdriver.Consumer
	closed         bool
	running        bool
	done           chan struct{}
}

func NewAccountingRuntime(c AccountingRuntimeConfig) (*AccountingRuntime, error) {
	if c.Admission == nil || len(c.SigningKey) != 32 || len(c.PreviousSigningKeys) > 4 || len(c.Adapter.Policies) != 0 || c.MarketConsumer.Subject != accounting.MarketSubject {
		return nil, store.ErrAccountingNotReady
	}
	if _, err := c.MarketConsumer.BrokerConsumerConfig(); err != nil {
		return nil, err
	}
	codec := rest.AccountingV2Handler{SigningKey: append([]byte(nil), c.SigningKey...)}
	for _, key := range c.PreviousSigningKeys {
		if len(key) != 32 {
			return nil, store.ErrAccountingNotReady
		}
		codec.PreviousSigningKeys = append(codec.PreviousSigningKeys, append([]byte(nil), key...))
	}
	model, err := store.NewAccountingReadModel(c.Adapter.Budget.Pool, c.ReadModel)
	if err != nil {
		return nil, err
	}
	policies, err := CanonicalPolicies(c.ReadModel.ProjectionGeneration, c.Origin, c.Allocation)
	if err != nil {
		return nil, err
	}
	c.Adapter.Policies = policies
	a, err := NewAdapter(c.Adapter)
	if err != nil {
		return nil, err
	}
	bindings, err := NewAccountingBindings(AccountingBindingConfig{
		Adapter: a, Model: model, Generation: c.ReadModel.ProjectionGeneration,
		ProjectionVersion: c.ReadModel.ProjectionVersion, MarketStream: c.MarketConsumer.Stream,
		Codec: codec, Resolve: PublicNetworkScope, Error: accountingRuntimeError,
	})
	if err != nil {
		return nil, err
	}
	return &AccountingRuntime{adapter: a, bindings: bindings, admission: c.Admission, consumerConfig: c.MarketConsumer}, nil
}

func accountingRuntimeError(w http.ResponseWriter, r *http.Request, err error) {
	var limit *LimitError
	switch {
	case errors.As(err, &limit):
		seconds := (sharedredis.Admission{RetryAfter: limit.RetryAfter}).RetryAfterSeconds()
		delay, _ := strconv.Atoi(seconds)
		ms := delay * 1000
		id := w.Header().Get("X-Request-ID")
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.NewString()
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Schema-Version", "2")
		w.Header().Set("Retry-After", seconds)
		w.WriteHeader(429)
		_ = json.NewEncoder(w).Encode(rest.APIError{Code: "RATE_LIMITED", Message: "Request budget exhausted", RequestID: id, Retryable: true, RetryAfterMS: &ms, FieldErrors: []any{}})
	case errors.Is(err, rest.ErrAccountingFields):
		rest.WriteV2APIError(w, r, 422, "FIELD_INVALID")
	case errors.Is(err, store.ErrCursorInvalid):
		rest.WriteV2APIError(w, r, 400, "CURSOR_INVALID")
	case errors.Is(err, store.ErrDataIntegrity), errors.Is(err, ErrProjectionChanged):
		rest.WriteV2APIError(w, r, 503, "INTEGRITY_QUARANTINED")
	default:
		code, status := "DEPENDENCY_UNAVAILABLE", 503
		if r.URL.Query().Get("cursor") != "" && errors.Is(err, store.ErrAccountingNotReady) {
			code, status = "CURSOR_INVALID", 400
		}
		rest.WriteV2APIError(w, r, status, code)
	}
}

func (b *AccountingBindings) Register(r chi.Router, admission *rest.Admission) error {
	if r == nil || admission == nil {
		return store.ErrAccountingNotReady
	}
	r.Method("GET", "/v2/pools/{poolAddress}/trades", rest.ResponseMetadata(admission.Middleware(rest.AdmissionHistory)(http.HandlerFunc(b.Trades))))
	r.Method("GET", "/v2/pools/{poolAddress}/candles", rest.ResponseMetadata(admission.Middleware(rest.AdmissionHistory)(http.HandlerFunc(b.Candles))))
	return nil
}

func (a *AccountingRuntime) Start(ctx context.Context, nc *nats.Conn) error {
	if ctx == nil {
		return store.ErrAccountingNotReady
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.consumer != nil {
		return store.ErrAccountingNotReady
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	consumer, err := natsdriver.NewConsumer(bounded, nc, a.consumerConfig)
	if err != nil {
		return err
	}
	a.consumer = consumer
	return nil
}

func (a *AccountingRuntime) Ready(context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.consumer == nil || !a.running {
		return store.ErrAccountingNotReady
	}
	return nil
}

func (a *AccountingRuntime) Mount(r chi.Router) error {
	if r == nil {
		return store.ErrAccountingNotReady
	}
	r.Group(func(routes chi.Router) {
		routes.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := a.Ready(r.Context()); err != nil {
					rest.WriteV2APIError(w, r, 503, "DEPENDENCY_UNAVAILABLE")
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		// Constructor validation guarantees the admission allocation exists.
		_ = a.bindings.Register(routes, a.admission)
	})
	return nil
}

func (a *AccountingRuntime) Run(ctx context.Context) error {
	if ctx == nil {
		return store.ErrAccountingNotReady
	}
	a.mu.Lock()
	if a.closed || a.consumer == nil || a.running {
		a.mu.Unlock()
		return store.ErrAccountingNotReady
	}
	consumer := a.consumer
	a.running = true
	a.done = make(chan struct{})
	done := a.done
	a.mu.Unlock()
	defer close(done)
	defer a.Close()
	return consumer.Run(ctx, a.bindings.ConsumeMarket)
}

func (a *AccountingRuntime) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return store.ErrAccountingNotReady
	}
	a.Close()
	a.mu.Lock()
	done := a.done
	a.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *AccountingRuntime) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	if a.consumer != nil {
		a.consumer.Close()
	}
}
