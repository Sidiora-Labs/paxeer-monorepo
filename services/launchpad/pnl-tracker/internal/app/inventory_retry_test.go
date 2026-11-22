package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type inventoryRunTestClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *inventoryRunTestClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *inventoryRunTestClock) advance(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(duration)
}

type inventoryRunLog chan string

func (lines inventoryRunLog) Write(data []byte) (int, error) {
	lines <- string(data)
	return len(data), nil
}

func inventoryRunTestResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("inventory consumer did not return")
		return nil
	}
}

func inventoryRunTestStates(t *testing.T, lines inventoryRunLog, want []string) {
	t.Helper()
	var states []string
	for len(lines) != 0 {
		line := <-lines
		if !strings.Contains(line, `job="inventory consumer"`) {
			t.Fatalf("consumer log has no job: %s", line)
		}
		switch {
		case strings.Contains(line, "state=failing"):
			states = append(states, "failing")
		case strings.Contains(line, "state=passing"):
			states = append(states, "passing")
		default:
			t.Fatalf("consumer log has no state: %s", line)
		}
	}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("consumer states = %v, want %v", states, want)
	}
}

func TestInventoryRunAgainTransientThenShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	clock := &inventoryRunTestClock{}
	lines := make(inventoryRunLog, 8)
	logger := slog.New(slog.NewTextHandler(lines, nil))
	started := make(chan struct{})
	result := make(chan error, 1)
	var waits []time.Duration
	calls := 0
	run := func(operation context.Context) error {
		calls++
		if calls == 1 {
			return errors.New("connection interrupted")
		}
		close(started)
		<-operation.Done()
		return operation.Err()
	}
	wait := func(operation context.Context, delay time.Duration) {
		if operation != ctx {
			<-operation.Done()
			return
		}
		waits = append(waits, delay)
	}
	go func() { result <- runInventoryAgain(ctx, logger, run, wait, clock.now) }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("inventory consumer did not restart")
	}
	cancel()
	if err := inventoryRunTestResult(t, result); err != context.Canceled {
		t.Fatalf("consumer result = %v, want cancellation", err)
	}
	if calls != 2 || !reflect.DeepEqual(waits, []time.Duration{time.Second}) {
		t.Fatalf("consumer calls = %d, waits = %v", calls, waits)
	}
	inventoryRunTestStates(t, lines, []string{"failing"})
}

func TestInventoryRunAgainBackoff(t *testing.T) {
	for _, failures := range []int{3, 10} {
		t.Run(fmt.Sprint(failures), func(t *testing.T) {
			ctx := context.Background()
			clock := &inventoryRunTestClock{}
			lines := make(inventoryRunLog, 8)
			logger := slog.New(slog.NewTextHandler(lines, nil))
			calls := 0
			var waits []time.Duration
			run := func(context.Context) error {
				calls++
				if calls <= failures {
					return context.DeadlineExceeded
				}
				return nil
			}
			wait := func(operation context.Context, delay time.Duration) {
				if operation != ctx {
					<-operation.Done()
					return
				}
				waits = append(waits, delay)
			}
			if err := runInventoryAgain(ctx, logger, run, wait, clock.now); err != nil {
				t.Fatal(err)
			}
			want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second, 30 * time.Second, 30 * time.Second, 30 * time.Second}[:failures]
			if calls != failures+1 || !reflect.DeepEqual(waits, want) {
				t.Fatalf("consumer calls = %d, waits = %v, want %v", calls, waits, want)
			}
			inventoryRunTestStates(t, lines, []string{"failing"})
		})
	}
}

func TestInventoryRunAgainStableRunResetsBackoff(t *testing.T) {
	for _, duration := range []time.Duration{29 * time.Second, 30 * time.Second, 31 * time.Second} {
		t.Run(duration.String(), func(t *testing.T) {
			ctx := context.Background()
			clock := &inventoryRunTestClock{}
			lines := make(inventoryRunLog, 8)
			logger := slog.New(slog.NewTextHandler(lines, nil))
			calls := 0
			var waits []time.Duration
			run := func(context.Context) error {
				calls++
				if calls == 3 {
					clock.advance(duration)
				}
				if calls <= 3 {
					return errors.New("connection interrupted")
				}
				return nil
			}
			wait := func(operation context.Context, delay time.Duration) {
				if operation != ctx {
					<-operation.Done()
					return
				}
				waits = append(waits, delay)
			}
			if err := runInventoryAgain(ctx, logger, run, wait, clock.now); err != nil {
				t.Fatal(err)
			}
			last := time.Second
			states := []string{"failing", "passing", "failing"}
			if duration < 30*time.Second {
				last = 4 * time.Second
				states = []string{"failing"}
			}
			want := []time.Duration{time.Second, 2 * time.Second, last}
			if calls != 4 || !reflect.DeepEqual(waits, want) {
				t.Fatalf("consumer calls = %d, waits = %v, want %v", calls, waits, want)
			}
			inventoryRunTestStates(t, lines, states)
		})
	}
}

func TestInventoryRunAgainPermanentErrors(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		class string
	}{
		{"delivery authorization", delivery.ErrAuthorization, "authorization"},
		{"broker authorization", nats.ErrAuthorization, "authorization"},
		{"broker permission", nats.ErrPermissionViolation, "authorization"},
		{"integrity", delivery.ErrIntegrity, "integrity"},
		{"typed integrity", &delivery.IntegrityFailure{Class: delivery.InnerSchema, Cause: errors.New("invalid publication")}, "integrity"},
		{"schema", delivery.ErrSchema, "schema"},
		{"closed", natsdriver.ErrClosed, "closed"},
		{"connection closed", nats.ErrConnectionClosed, "closed"},
		{"connection draining", nats.ErrConnectionDraining, "closed"},
		{"broker connection closed", jetstream.ErrConnectionClosed, "closed"},
		{"consumer deleted", jetstream.ErrConsumerDeleted, "closed"},
	}
	for _, tc := range cases {
		for _, wrap := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/wrapped=%t", tc.name, wrap), func(t *testing.T) {
				want := tc.err
				if wrap {
					want = fmt.Errorf("consumer stopped: %w", errors.Join(context.DeadlineExceeded, want))
				}
				if class, permanent := inventoryRunErrorClass(want); class != tc.class || !permanent {
					t.Fatalf("error class = %q, permanent = %t", class, permanent)
				}
				calls := 0
				run := func(context.Context) error { calls++; return want }
				wait := func(context.Context, time.Duration) { t.Error("permanent error waited for another run") }
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				if err := runInventoryAgain(context.Background(), logger, run, wait, time.Now); err != want || calls != 1 {
					t.Fatalf("consumer result = %v, calls = %d", err, calls)
				}
			})
		}
	}
}

func TestInventoryRunAgainBrokerRecovery(t *testing.T) {
	for _, cause := range []error{jetstream.ErrConsumerLeadershipChanged, jetstream.ErrServerShutdown, nats.ErrNoResponders} {
		t.Run(cause.Error(), func(t *testing.T) {
			ctx := context.Background()
			calls, waits := 0, 0
			run := func(context.Context) error {
				calls++
				if calls == 1 {
					return fmt.Errorf("fetch: %w", cause)
				}
				return nil
			}
			wait := func(operation context.Context, delay time.Duration) {
				if operation != ctx {
					<-operation.Done()
					return
				}
				waits++
				if delay != time.Second {
					t.Errorf("retry wait = %s", delay)
				}
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			if err := runInventoryAgain(ctx, logger, run, wait, time.Now); err != nil || calls != 2 || waits != 1 {
				t.Fatalf("consumer result = %v, calls = %d, waits = %d", err, calls, waits)
			}
		})
	}
}

func TestInventoryRunAgainNilReturn(t *testing.T) {
	calls := 0
	run := func(context.Context) error { calls++; return nil }
	wait := func(context.Context, time.Duration) { t.Error("successful return waited for another run") }
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := runInventoryAgain(context.Background(), logger, run, wait, time.Now); err != nil || calls != 1 {
		t.Fatalf("consumer result = %v, calls = %d", err, calls)
	}
}

func TestInventoryRunAgainEndedContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	t.Run("before run", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		run := func(context.Context) error { t.Error("ended context started a run"); return nil }
		wait := func(context.Context, time.Duration) { t.Error("ended context waited for a run") }
		if err := runInventoryAgain(ctx, logger, run, wait, time.Now); err != context.Canceled {
			t.Fatalf("consumer result = %v", err)
		}
	})
	for _, want := range []error{nil, context.Canceled, errors.New("connection interrupted"), natsdriver.ErrClosed} {
		t.Run(fmt.Sprintf("during run/%v", want), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			run := func(context.Context) error { calls++; cancel(); return want }
			wait := func(context.Context, time.Duration) { t.Error("ended context waited for another run") }
			if err := runInventoryAgain(ctx, logger, run, wait, time.Now); err != want || calls != 1 {
				t.Fatalf("consumer result = %v, calls = %d", err, calls)
			}
		})
	}
	t.Run("during wait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		want := errors.New("connection interrupted")
		calls, waits := 0, 0
		run := func(context.Context) error { calls++; return want }
		wait := func(context.Context, time.Duration) { waits++; cancel() }
		if err := runInventoryAgain(ctx, logger, run, wait, time.Now); err != want || calls != 1 || waits != 1 {
			t.Fatalf("consumer result = %v, calls = %d, waits = %d", err, calls, waits)
		}
	})
}

func TestInventoryRunAgainRecoveryWhileRunning(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	clock := &inventoryRunTestClock{}
	lines := make(inventoryRunLog, 8)
	logger := slog.New(slog.NewTextHandler(lines, nil))
	monitorStarted := make(chan struct{})
	recoverRun := make(chan struct{})
	running := make(chan struct{})
	result := make(chan error, 1)
	calls := 0
	var waits []time.Duration
	run := func(operation context.Context) error {
		calls++
		if calls == 1 {
			return errors.New("connection interrupted")
		}
		close(running)
		<-operation.Done()
		return operation.Err()
	}
	wait := func(operation context.Context, delay time.Duration) {
		if operation == ctx {
			waits = append(waits, delay)
			return
		}
		if delay != 30*time.Second {
			t.Errorf("recovery wait = %s", delay)
		}
		close(monitorStarted)
		select {
		case <-recoverRun:
		case <-operation.Done():
		}
	}
	go func() { result <- runInventoryAgain(ctx, logger, run, wait, clock.now) }()
	for _, started := range []<-chan struct{}{running, monitorStarted} {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("inventory consumer recovery did not start")
		}
	}
	inventoryRunTestStates(t, lines, []string{"failing"})
	clock.advance(30 * time.Second)
	close(recoverRun)
	select {
	case line := <-lines:
		if !strings.Contains(line, "state=passing") || !strings.Contains(line, `job="inventory consumer"`) {
			t.Fatalf("recovery log = %s", line)
		}
	case <-ctx.Done():
		t.Fatal("running consumer did not report recovery")
	}
	select {
	case err := <-result:
		t.Fatalf("recovered consumer stopped: %v", err)
	default:
	}
	cancel()
	if err := inventoryRunTestResult(t, result); err != context.Canceled {
		t.Fatalf("consumer result = %v", err)
	}
	if calls != 2 || !reflect.DeepEqual(waits, []time.Duration{time.Second}) {
		t.Fatalf("consumer calls = %d, waits = %v", calls, waits)
	}
	inventoryRunTestStates(t, lines, nil)
}
