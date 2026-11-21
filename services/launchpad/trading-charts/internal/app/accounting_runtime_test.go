package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
)

type accountingTestTimer struct {
	deadline time.Time
	channel  chan time.Time
	stopped  bool
}

type accountingTestClock struct {
	mu     sync.Mutex
	time   time.Time
	timers []*accountingTestTimer
}

func (c *accountingTestClock) clock() accountingClock {
	return accountingClock{now: func() time.Time {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.time
	}, timer: func(delay time.Duration) (<-chan time.Time, func()) {
		c.mu.Lock()
		defer c.mu.Unlock()
		timer := &accountingTestTimer{deadline: c.time.Add(delay), channel: make(chan time.Time, 1)}
		c.timers = append(c.timers, timer)
		return timer.channel, func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			timer.stopped = true
		}
	}}
}

func (c *accountingTestClock) advance(delay time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.time = c.time.Add(delay)
	for _, timer := range c.timers {
		if !timer.stopped && !c.time.Before(timer.deadline) {
			timer.stopped = true
			timer.channel <- c.time
		}
	}
}

type accountingStateLog struct {
	states chan string
}

func (h accountingStateLog) Enabled(context.Context, slog.Level) bool { return true }
func (h accountingStateLog) Handle(_ context.Context, record slog.Record) error {
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "state" {
			h.states <- attr.Value.String()
		}
		return true
	})
	return nil
}
func (h accountingStateLog) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h accountingStateLog) WithGroup(string) slog.Handler      { return h }

func accountingTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func accountingAwait(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("accounting consumer did not reach the expected state")
	}
}

func TestAccountingRunAgainTransient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clock := &accountingTestClock{}
	calls, failures := 0, 0
	var waits []time.Duration
	entered, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		runAccountingConsumer(ctx, func(ctx context.Context) error {
			calls++
			if calls == 1 {
				return errors.New("dependency unavailable")
			}
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		}, func() { failures++ }, accountingTestLogger(), func(_ context.Context, delay time.Duration) {
			waits = append(waits, delay)
		}, clock.clock())
	}()
	accountingAwait(t, entered)
	cancel()
	accountingAwait(t, done)
	if calls != 2 || failures != 0 || !reflect.DeepEqual(waits, []time.Duration{time.Second}) {
		t.Fatalf("calls=%d failures=%d waits=%v", calls, failures, waits)
	}
}

func TestAccountingRunAgainBackoff(t *testing.T) {
	for _, tc := range []struct {
		name  string
		waits []time.Duration
	}{
		{"three errors", []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}},
		{"bounded errors", []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second, 30 * time.Second}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			clock := &accountingTestClock{}
			calls, failures := 0, 0
			var waits []time.Duration
			runAccountingConsumer(ctx, func(context.Context) error {
				calls++
				if calls > len(tc.waits) {
					cancel()
					return ctx.Err()
				}
				return context.DeadlineExceeded
			}, func() { failures++ }, accountingTestLogger(), func(_ context.Context, delay time.Duration) {
				waits = append(waits, delay)
			}, clock.clock())
			if calls != len(tc.waits)+1 || failures != 0 || !reflect.DeepEqual(waits, tc.waits) {
				t.Fatalf("calls=%d failures=%d waits=%v want=%v", calls, failures, waits, tc.waits)
			}
		})
	}
}

func TestAccountingRunAgainResetsAfterLongRun(t *testing.T) {
	for _, duration := range []time.Duration{30 * time.Second, 45 * time.Second} {
		t.Run(duration.String(), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			clock := &accountingTestClock{}
			calls := 0
			var waits []time.Duration
			runAccountingConsumer(ctx, func(context.Context) error {
				calls++
				if calls == 3 {
					clock.advance(duration)
				}
				if calls == 4 {
					cancel()
					return ctx.Err()
				}
				return errors.New("dependency unavailable")
			}, func() { t.Error("transient error ended runtime") }, accountingTestLogger(), func(_ context.Context, delay time.Duration) {
				waits = append(waits, delay)
			}, clock.clock())
			if !reflect.DeepEqual(waits, []time.Duration{time.Second, 2 * time.Second, time.Second}) {
				t.Fatalf("waits=%v", waits)
			}
		})
	}
}

func TestAccountingRunAgainPermanent(t *testing.T) {
	for _, tc := range []struct {
		err   error
		class string
	}{
		{delivery.ErrAuthorization, "authorization"},
		{nats.ErrAuthorization, "authorization"},
		{nats.ErrPermissionViolation, "authorization"},
		{delivery.ErrIntegrity, "integrity"},
		{&delivery.IntegrityFailure{Class: delivery.InnerSchema, Cause: errors.New("invalid payload")}, "integrity"},
		{delivery.ErrSchema, "schema"},
		{natsdriver.ErrClosed, "closed"},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			clock := &accountingTestClock{}
			calls, failures, waits := 0, 0, 0
			err := fmt.Errorf("consumer: %w", errors.Join(errors.New("dependency unavailable"), tc.err))
			class, permanent := accountingErrorClass(err)
			if class != tc.class || !permanent {
				t.Fatalf("class=%s permanent=%v", class, permanent)
			}
			runAccountingConsumer(context.Background(), func(context.Context) error {
				calls++
				return err
			}, func() { failures++ }, accountingTestLogger(), func(context.Context, time.Duration) { waits++ }, clock.clock())
			if calls != 1 || failures != 1 || waits != 0 {
				t.Fatalf("calls=%d failures=%d waits=%d", calls, failures, waits)
			}
		})
	}
}

func TestAccountingRunAgainCanceled(t *testing.T) {
	for _, stage := range []string{"before run", "returned cancellation", "during run", "during wait"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if stage == "before run" {
				cancel()
			}
			clock := &accountingTestClock{}
			calls, failures, waits := 0, 0, 0
			runAccountingConsumer(ctx, func(context.Context) error {
				calls++
				if stage == "returned cancellation" {
					return fmt.Errorf("consumer: %w", context.Canceled)
				}
				if stage == "during run" {
					cancel()
				}
				return errors.New("dependency unavailable")
			}, func() { failures++ }, accountingTestLogger(), func(context.Context, time.Duration) {
				waits++
				cancel()
			}, clock.clock())
			wantCalls, wantWaits := 1, 0
			if stage == "before run" {
				wantCalls = 0
			}
			if stage == "during wait" {
				wantWaits = 1
			}
			if calls != wantCalls || failures != 0 || waits != wantWaits {
				t.Fatalf("calls=%d failures=%d waits=%d", calls, failures, waits)
			}
		})
	}
}

func TestAccountingRunAgainReportsRecoveryWhileRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clock := &accountingTestClock{}
	states := make(chan string, 10)
	entered, done := make(chan struct{}), make(chan struct{})
	calls := 0
	go func() {
		defer close(done)
		runAccountingConsumer(ctx, func(ctx context.Context) error {
			calls++
			if calls <= 2 {
				return errors.New("dependency unavailable")
			}
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		}, func() { t.Error("transient error ended runtime") }, slog.New(accountingStateLog{states: states}), func(context.Context, time.Duration) {}, clock.clock())
	}()
	accountingAwait(t, entered)
	if state := <-states; state != "failing" {
		t.Fatalf("initial state=%s", state)
	}
	clock.advance(29 * time.Second)
	select {
	case state := <-states:
		t.Fatalf("early state=%s", state)
	default:
	}
	clock.advance(time.Second)
	select {
	case state := <-states:
		if state != "passing" {
			t.Fatalf("recovered state=%s", state)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("accounting recovery was not reported while consumer was running")
	}
	clock.advance(30 * time.Second)
	cancel()
	accountingAwait(t, done)
	if len(states) != 0 || calls != 3 {
		t.Fatalf("extra states=%d calls=%d", len(states), calls)
	}
}

func TestAccountingRunAgainInternalFailure(t *testing.T) {
	cfg, _, _, _ := productionChartRuntime(t)
	productionChartRunEnvironment(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := Run(ctx); !errors.Is(err, ErrInternalFailure) || err.Error() != ErrInternalFailure.Error() {
		t.Fatalf("runtime with unavailable rollup authority: %v", err)
	}
}
