package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
)

type consumerJobTestTimer struct {
	due     time.Time
	run     func()
	pending bool
}

type consumerJobTestClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*consumerJobTestTimer
}

func (c *consumerJobTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *consumerJobTestClock) AfterFunc(delay time.Duration, run func()) func() bool {
	c.mu.Lock()
	timer := &consumerJobTestTimer{due: c.now.Add(delay), run: run, pending: true}
	c.timers = append(c.timers, timer)
	c.mu.Unlock()
	return func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		pending := timer.pending
		timer.pending = false
		return pending
	}
}

func (c *consumerJobTestClock) advance(delay time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(delay)
	var ready []func()
	for _, timer := range c.timers {
		if timer.pending && !timer.due.After(c.now) {
			timer.pending = false
			ready = append(ready, timer.run)
		}
	}
	c.mu.Unlock()
	for _, run := range ready {
		run()
	}
}

func consumerJobTestLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &output
}

func TestConsumerJobRunAgainRecoversHealth(t *testing.T) {
	output := consumerJobTestLogs(t)
	clock := &consumerJobTestClock{}
	health := &statsRuntimeHealth{failures: map[string]error{}}
	temporary := errors.New("consumer temporarily unavailable")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waits := make(chan time.Duration, 1)
	retry := make(chan struct{})
	started := make(chan struct{})
	calls := 0
	job := consumerJobRunAgain("market accounting", health, func(ctx context.Context) error {
		calls++
		if calls == 1 {
			return temporary
		}
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}, func(ctx context.Context, delay time.Duration) {
		waits <- delay
		select {
		case <-retry:
		case <-ctx.Done():
		}
	}, clock)
	done := make(chan error, 1)
	go func() { done <- job(ctx) }()
	select {
	case delay := <-waits:
		if delay != time.Second {
			t.Fatalf("first retry delay: %v", delay)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not schedule a retry")
	}
	if err := health.check(context.Background()); !errors.Is(err, temporary) {
		t.Fatalf("health after failure: %v", err)
	}
	close(retry)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not run again")
	}
	clock.advance(29 * time.Second)
	if err := health.check(context.Background()); !errors.Is(err, temporary) {
		t.Fatalf("health before stable period: %v", err)
	}
	clock.advance(time.Second)
	if err := health.check(context.Background()); err != nil {
		t.Fatalf("health after stable period: %v", err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || calls != 2 {
			t.Fatalf("consumer drain: calls=%d, error=%v", calls, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not stop")
	}
	if strings.Count(output.String(), `"state":"failing"`) != 1 || strings.Count(output.String(), `"state":"passing"`) != 1 || strings.Count(output.String(), `"job":"market accounting"`) != 2 {
		t.Fatalf("consumer state transitions: %s", output.String())
	}
}

func TestConsumerJobRunAgainBackoff(t *testing.T) {
	output := consumerJobTestLogs(t)
	clock := &consumerJobTestClock{}
	health := &statsRuntimeHealth{failures: map[string]error{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var latest error
	calls := 0
	var waits []time.Duration
	job := consumerJobRunAgain("search creation", health, func(context.Context) error {
		calls++
		latest = fmt.Errorf("consumer temporarily unavailable %d", calls)
		return latest
	}, func(ctx context.Context, delay time.Duration) {
		waits = append(waits, delay)
		if err := health.check(ctx); !errors.Is(err, latest) {
			t.Fatalf("health must retain latest failure: %v", err)
		}
		clock.advance(delay)
		if err := health.check(ctx); !errors.Is(err, latest) {
			t.Fatalf("health during retry wait: %v", err)
		}
		if len(waits) == 8 {
			cancel()
		}
	}, clock)
	err := job(ctx)
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second, 30 * time.Second}
	if !errors.Is(err, context.Canceled) || calls != len(want) || !reflect.DeepEqual(waits, want) {
		t.Fatalf("backoff: calls=%d, waits=%v, error=%v", calls, waits, err)
	}
	if strings.Count(output.String(), `"state":"failing"`) != 1 || strings.Contains(output.String(), `"state":"passing"`) {
		t.Fatalf("repeated failures changed state: %s", output.String())
	}
}

func TestConsumerJobRunAgainResetsBackoff(t *testing.T) {
	output := consumerJobTestLogs(t)
	clock := &consumerJobTestClock{}
	health := &statsRuntimeHealth{failures: map[string]error{}}
	ctx := context.Background()
	temporary := errors.New("consumer temporarily unavailable")
	calls := 0
	var waits []time.Duration
	job := consumerJobRunAgain("search invalidation", health, func(context.Context) error {
		calls++
		if calls == 4 {
			clock.advance(consumerJobStablePeriod)
			if err := health.check(ctx); err != nil {
				t.Fatalf("long run did not recover health: %v", err)
			}
		}
		if calls == 5 {
			return nil
		}
		return temporary
	}, func(ctx context.Context, delay time.Duration) {
		waits = append(waits, delay)
		if err := health.check(ctx); !errors.Is(err, temporary) {
			t.Fatalf("health after failed run: %v", err)
		}
		clock.advance(delay)
	}, clock)
	if err := job(ctx); err != nil {
		t.Fatal(err)
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, time.Second}
	if calls != 5 || !reflect.DeepEqual(waits, want) {
		t.Fatalf("backoff after long run: calls=%d, waits=%v", calls, waits)
	}
	if err := health.check(ctx); !errors.Is(err, temporary) {
		t.Fatalf("short nil return cleared health: %v", err)
	}
	if strings.Count(output.String(), `"state":"failing"`) != 2 || strings.Count(output.String(), `"state":"passing"`) != 1 {
		t.Fatalf("consumer state transitions: %s", output.String())
	}
}

func TestConsumerJobRunAgainPermanentErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"delivery authorization", delivery.ErrAuthorization},
		{"broker authorization", nats.ErrAuthorization},
		{"broker permission", nats.ErrPermissionViolation},
		{"delivery integrity", delivery.ErrIntegrity},
		{"typed integrity", &delivery.IntegrityFailure{Class: delivery.Authority, Cause: errors.New("authority mismatch")}},
		{"delivery schema", delivery.ErrSchema},
		{"driver closed", natsdriver.ErrClosed},
		{"broker closed", nats.ErrConnectionClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, wrapped := range []bool{false, true} {
				err := tc.err
				if wrapped {
					err = fmt.Errorf("transactional consume: %w", errors.Join(errors.New("consumer stopped"), err))
				}
				calls, waits := 0, 0
				clock := &consumerJobTestClock{}
				health := &statsRuntimeHealth{failures: map[string]error{}}
				job := consumerJobRunAgain("search creation", health, func(context.Context) error {
					calls++
					return err
				}, func(context.Context, time.Duration) { waits++ }, clock)
				if got := job(context.Background()); got != err || calls != 1 || waits != 0 || len(clock.timers) != 0 {
					t.Fatalf("permanent error: result=%v, calls=%d, waits=%d", got, calls, waits)
				}
			}
		})
	}
}

func TestConsumerJobRunAgainKeepsHealthAcrossShortRuns(t *testing.T) {
	output := consumerJobTestLogs(t)
	clock := &consumerJobTestClock{}
	health := &statsRuntimeHealth{failures: map[string]error{}}
	first := errors.New("consumer temporarily unavailable")
	second := errors.New("consumer still unavailable")
	calls := 0
	job := consumerJobRunAgain("search account_profile", health, func(ctx context.Context) error {
		calls++
		switch calls {
		case 1:
			return first
		case 2:
			clock.advance(29 * time.Second)
			return second
		default:
			clock.timers[0].run()
			if err := health.check(ctx); !errors.Is(err, second) {
				t.Fatalf("previous run cleared the latest failure: %v", err)
			}
			clock.advance(29 * time.Second)
			if err := health.check(ctx); !errors.Is(err, second) {
				t.Fatalf("short runs accumulated into recovery: %v", err)
			}
			return delivery.ErrSchema
		}
	}, func(ctx context.Context, delay time.Duration) {
		clock.advance(delay)
		if err := health.check(ctx); err == nil {
			t.Fatal("retry wait cleared consumer health")
		}
	}, clock)
	if err := job(context.Background()); err != delivery.ErrSchema || calls != 3 {
		t.Fatalf("terminal result after retries: calls=%d, error=%v", calls, err)
	}
	clock.advance(time.Minute)
	if err := health.check(context.Background()); !errors.Is(err, second) || strings.Contains(output.String(), `"state":"passing"`) {
		t.Fatalf("stopped run cleared health: %v, %s", err, output.String())
	}
}

func TestConsumerJobRunAgainNilReturn(t *testing.T) {
	calls, waits := 0, 0
	health := &statsRuntimeHealth{failures: map[string]error{}}
	job := consumerJobRunAgain("market accounting", health, func(context.Context) error {
		calls++
		return nil
	}, func(context.Context, time.Duration) { waits++ }, &consumerJobTestClock{})
	if err := job(context.Background()); err != nil || calls != 1 || waits != 0 {
		t.Fatalf("nil return: result=%v, calls=%d, waits=%d", err, calls, waits)
	}
}

func TestConsumerJobRunAgainCanceledContext(t *testing.T) {
	consumerJobTestLogs(t)
	for _, phase := range []string{"before run", "during run", "during wait"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if phase == "before run" {
				cancel()
			}
			calls, waits := 0, 0
			health := &statsRuntimeHealth{failures: map[string]error{}}
			job := consumerJobRunAgain("market accounting", health, func(context.Context) error {
				calls++
				if phase == "during run" {
					cancel()
				}
				return errors.New("consumer temporarily unavailable")
			}, func(context.Context, time.Duration) {
				waits++
				cancel()
			}, &consumerJobTestClock{})
			wantCalls, wantWaits := 1, 0
			if phase == "before run" {
				wantCalls = 0
			}
			if phase == "during wait" {
				wantWaits = 1
			}
			if err := job(ctx); !errors.Is(err, context.Canceled) || calls != wantCalls || waits != wantWaits {
				t.Fatalf("cancellation: calls=%d, waits=%d, error=%v", calls, waits, err)
			}
		})
	}
}
