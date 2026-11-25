package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/consumerhealth"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
	"github.com/go-chi/chi/v5"
)

func consumerReadinessFixture(t *testing.T, name string) (*consumerhealth.State, *time.Time, *bytes.Buffer) {
	t.Helper()
	now := time.Unix(1700000000, 0)
	var output bytes.Buffer
	state, err := newConsumerReadiness(config.Config{}, name, slog.New(slog.NewJSONHandler(&output, nil)), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return state, &now, &output
}

func TestConsumerReadinessProgressAndRecovery(t *testing.T) {
	for _, name := range []string{"accounting_consumer"} {
		t.Run(name, func(t *testing.T) {
			state, now, output := consumerReadinessFixture(t, name)
			ctx := context.Background()
			*now = now.Add(time.Hour)
			if err := state.Check(ctx); err != nil {
				t.Fatalf("idle consumer: %v", err)
			}
			state.Record(nil, "", "pool_a")
			if err := state.Check(ctx); err != nil {
				t.Fatalf("successful consumer: %v", err)
			}
			state.Record(delivery.ErrLease, "lease", "pool_a")
			*now = now.Add(5 * time.Minute)
			if err := state.Check(ctx); err != nil {
				t.Fatalf("consumer at bound: %v", err)
			}
			*now = now.Add(time.Nanosecond)
			state.Record(delivery.ErrLease, "lease", "pool_a")
			for i := 0; i < 3; i++ {
				if err := state.Check(ctx); err == nil {
					t.Fatal("stalled consumer is ready")
				}
			}
			snapshot := state.Snapshot()
			if snapshot.ConsecutiveFailures != 2 || snapshot.RetryPartitions != 1 || snapshot.LastError.IsZero() || snapshot.LastSuccess.IsZero() || snapshot.ErrorClass != "lease" {
				t.Fatalf("consumer state: %+v", snapshot)
			}
			if strings.Count(output.String(), "accounting consumer readiness changed") != 1 {
				t.Fatalf("failure transitions: %s", output.String())
			}
			state.Record(nil, "", "pool_a")
			if err := state.Check(ctx); err != nil {
				t.Fatalf("recovered consumer: %v", err)
			}
			if strings.Count(output.String(), "accounting consumer readiness changed") != 2 {
				t.Fatalf("recovery transitions: %s", output.String())
			}
			if !strings.Contains(output.String(), `"consumer":"`+name+`"`) || !strings.Contains(output.String(), `"error_class":"lease"`) || !strings.Contains(output.String(), `"consecutive_failures":2`) || !strings.Contains(output.String(), `"last_success_age_seconds":`) {
				t.Fatal(output.String())
			}
			snapshot = state.Snapshot()
			if snapshot.ConsecutiveFailures != 0 || snapshot.RetryPartitions != 0 || snapshot.ErrorClass != "lease" {
				t.Fatalf("recovered state: %+v", snapshot)
			}
		})
	}
}

func TestConsumerReadinessRoute(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	redis := internaltest.NewRedis(t)
	for _, name := range []string{"accounting_consumer"} {
		t.Run(name, func(t *testing.T) {
			state, now, output := consumerReadinessFixture(t, name)
			router := chi.NewRouter()
			sharedhttp.RegisterHealth(router, sharedhttp.HealthDeps{DB: pool.Ping, Redis: func(ctx context.Context) error { return redis.Ping(ctx).Err() }, Custom: map[string]sharedhttp.Check{name: state.Check}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
			request := func(path string, want int) map[string]string {
				t.Helper()
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
				if response.Code != want {
					t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
				}
				var body struct {
					Checks map[string]string `json:"checks"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				return body.Checks
			}
			state.Record(nil, "", "wallet_a")
			if checks := request("/health/ready", http.StatusOK); checks[name] != "ok" {
				t.Fatal(checks)
			}
			state.Record(delivery.ErrLease, "lease", "wallet_a")
			*now = now.Add(5*time.Minute + time.Second)
			for i := 0; i < 2; i++ {
				if checks := request("/health/ready", http.StatusServiceUnavailable); checks[name] != "failed" || checks["database"] != "ok" || checks["redis"] != "ok" {
					t.Fatal(checks)
				}
			}
			request("/health", http.StatusOK)
			request("/health/live", http.StatusOK)
			if strings.Count(output.String(), "accounting consumer readiness changed") != 1 {
				t.Fatal(output.String())
			}
			state.Record(nil, "", "wallet_a")
			request("/health/ready", http.StatusOK)
			if strings.Count(output.String(), "accounting consumer readiness changed") != 2 {
				t.Fatal(output.String())
			}
		})
	}
}

func TestConsumerReadinessMixedPartitions(t *testing.T) {
	state, now, output := consumerReadinessFixture(t, "accounting_consumer")
	for i := 0; i < 10; i++ {
		state.Record(delivery.ErrLease, "lease", "pool_a")
		state.Record(nil, "", "pool_b")
		*now = now.Add(time.Minute)
		state.Record(delivery.ErrLease, "lease", "pool_a")
		if err := state.Check(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if state.Snapshot().RetryPartitions != 1 || output.Len() != 0 {
		t.Fatalf("mixed deliveries: %+v %s", state.Snapshot(), output.String())
	}
}

func TestConsumerReadinessSourceRetry(t *testing.T) {
	state, now, _ := consumerReadinessFixture(t, "accounting_consumer")
	state.Record(consumerhealth.ErrNoProgress, consumerhealth.SourceUnavailableClass, "pool_a")
	if err := state.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(5*time.Minute + time.Second)
	if err := state.Check(context.Background()); err == nil {
		t.Fatal("source retry without progress is ready")
	}
	state.Record(nil, "", "pool_b")
	state.Record(consumerhealth.ErrNoProgress, consumerhealth.SourceUnavailableClass, "pool_a")
	if err := state.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestConsumerReadinessBounds(t *testing.T) {
	for _, raw := range []string{"0s", "-1s", "29s", "1h1ns", "invalid"} {
		t.Run(raw, func(t *testing.T) {
			_, err := New(context.Background(), config.Config{ConsumerReadinessTimeout: raw}, slog.Default())
			if err == nil || !strings.Contains(err.Error(), "READINESS_TIMEOUT") {
				t.Fatalf("startup bound %q: %v", raw, err)
			}
		})
	}
	for raw, want := range map[string]time.Duration{"": 5 * time.Minute, "30s": 30 * time.Second, "5m": 5 * time.Minute, "1h": time.Hour} {
		bound, err := (config.Config{ConsumerReadinessTimeout: raw}).ConsumerReadinessBound()
		if err != nil || bound != want {
			t.Fatalf("bound %q: %s %v", raw, bound, err)
		}
	}
}

func TestConsumerReadinessMetricsAndBoundedState(t *testing.T) {
	state, now, _ := consumerReadinessFixture(t, "accounting_consumer")
	state.Record(nil, "", "pool_a")
	*now = now.Add(time.Second)
	state.Record(delivery.ErrLease, "lease", "pool_a")
	metrics, err := telemetry.New("statsd", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	consumerhealth.Metrics(metrics, "statsd", "qualification", "zone-a", state).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, name := range []string{"sli_read_eligible_requests_total", "consumer_last_success_timestamp_seconds", "consumer_last_error_timestamp_seconds", "consumer_consecutive_failures", "consumer_retry_partitions", "consumer_last_error_info", "consumer_retry_tracking_overflow"} {
		if !strings.Contains(response.Body.String(), name+"{") {
			t.Fatalf("missing metric %s", name)
		}
	}
	if strings.Contains(response.Body.String(), "pool_a") || !strings.Contains(response.Body.String(), `error_class="lease"`) {
		t.Fatal(response.Body.String())
	}
	for i := 0; i < consumerhealth.MaxRetryPartitions; i++ {
		state.Record(delivery.ErrLease, "lease", fmt.Sprint(i))
	}
	snapshot := state.Snapshot()
	if snapshot.RetryPartitions != consumerhealth.MaxRetryPartitions || !snapshot.RetryOverflow {
		t.Fatalf("retry bound: %+v", snapshot)
	}
}

func TestConsumerReadinessRetryGroups(t *testing.T) {
	state, _, _ := consumerReadinessFixture(t, "accounting_consumer")
	state.Record(delivery.ErrLease, "lease", "125:wallet_a:token_a", "125:wallet_a:token_b", "125:wallet_b:token_a")
	if state.Snapshot().RetryPartitions != 2 {
		t.Fatal(state.Snapshot())
	}
	state.Record(nil, "", "125:wallet_a:token_a")
	if state.Snapshot().RetryPartitions != 2 {
		t.Fatal(state.Snapshot())
	}
	state.Record(nil, "", "125:wallet_a:token_b")
	if state.Snapshot().RetryPartitions != 1 {
		t.Fatal(state.Snapshot())
	}
	state.Record(nil, "", "125:wallet_b:token_a")
	if state.Snapshot().RetryPartitions != 0 {
		t.Fatal(state.Snapshot())
	}
}
