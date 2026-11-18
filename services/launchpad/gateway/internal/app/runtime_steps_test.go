package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
)

type runtimeRecoveryTick struct {
	delay time.Duration
	next  chan struct{}
}

type runtimeRecoveryClock struct {
	ticks chan runtimeRecoveryTick
}

func (c runtimeRecoveryClock) wait(ctx context.Context, delay time.Duration) {
	tick := runtimeRecoveryTick{delay: delay, next: make(chan struct{})}
	select {
	case c.ticks <- tick:
	case <-ctx.Done():
		return
	}
	select {
	case <-tick.next:
	case <-ctx.Done():
	}
}

func (c runtimeRecoveryClock) tick(t *testing.T, want time.Duration) runtimeRecoveryTick {
	t.Helper()
	select {
	case tick := <-c.ticks:
		if tick.delay != want {
			t.Fatalf("step delay=%s, want %s", tick.delay, want)
		}
		return tick
	case <-time.After(3 * time.Second):
		t.Fatal("step did not schedule its next try")
		return runtimeRecoveryTick{}
	}
}

func startRuntimeRecovery(t *testing.T, r *gatewayConnectedRuntime, step runtimeStep, immediate bool, run func(context.Context) error) runtimeRecoveryClock {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r.ctx = ctx
	clock := runtimeRecoveryClock{ticks: make(chan runtimeRecoveryTick)}
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.runStep(step, runtimeStepPeriod, immediate, run, clock.wait)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("step did not stop after cancellation")
		}
	})
	return clock
}

func TestRuntimeStepRecoveryReadiness(t *testing.T) {
	for step := runtimeStep(0); step < runtimeStepCount; step++ {
		t.Run(step.name(), func(t *testing.T) {
			r := &gatewayConnectedRuntime{checkDependencies: func(context.Context) error { return nil }}
			first, second := errors.New("store session ended"), errors.New("publication unavailable")
			calls := 0
			clock := startRuntimeRecovery(t, r, step, false, func(context.Context) error {
				calls++
				switch calls {
				case 1:
					return first
				case 2:
					return second
				default:
					return nil
				}
			})
			initial := clock.tick(t, runtimeStepPeriod)
			if calls != 0 || r.ready(context.Background()) != nil {
				t.Fatal("periodic step ran before its period")
			}
			close(initial.next)
			for i, want := range []error{first, second, nil, nil} {
				delay := runtimeStepPeriod
				if i == 1 {
					delay *= 2
				}
				tick := clock.tick(t, delay)
				if got := r.ready(context.Background()); !errors.Is(got, want) {
					t.Fatalf("try %d: ready=%v, want %v", i+1, got, want)
				}
				if r.failure.Load() != nil || calls != i+1 {
					t.Fatalf("try %d: calls=%d kept=%v", i+1, calls, r.failure.Load())
				}
				if i < 3 {
					close(tick.next)
				}
			}
			dependency := errors.New("dependency unavailable")
			r.checkDependencies = func(context.Context) error { return dependency }
			if got := r.ready(context.Background()); got != dependency {
				t.Fatalf("dependency readiness after recovery=%v", got)
			}
		})
	}
}

func TestRuntimeStepRecoveryBackoff(t *testing.T) {
	r := &gatewayConnectedRuntime{}
	calls := 0
	clock := startRuntimeRecovery(t, r, runtimeAuditRelay, true, func(context.Context) error {
		calls++
		if calls == 11 {
			return nil
		}
		return context.DeadlineExceeded
	})
	want := []time.Duration{
		250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second,
		4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second,
		30 * time.Second, 30 * time.Second, 250 * time.Millisecond,
		250 * time.Millisecond, 500 * time.Millisecond,
	}
	for i, delay := range want {
		tick := clock.tick(t, delay)
		if calls != i+1 {
			t.Fatalf("try count=%d, want %d", calls, i+1)
		}
		if i < len(want)-1 {
			close(tick.next)
		}
	}
}

func TestRuntimeStepRecoveryPermanentErrors(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		class string
	}{
		{"delivery authorization", delivery.ErrAuthorization, "authorization"},
		{"broker authorization", nats.ErrAuthorization, "authorization"},
		{"broker permission", nats.ErrPermissionViolation, "authorization"},
		{"scope", sharedauth.ErrScopeDenied, "authorization"},
		{"integrity", delivery.ErrIntegrity, "integrity"},
		{"replay", sharedauth.ErrReplayMismatch, "integrity"},
		{"binding", &delivery.IntegrityFailure{Class: delivery.PartitionBinding}, "integrity"},
		{"schema", delivery.ErrSchema, "schema"},
		{"closed consumer", natsdriver.ErrClosed, "closed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &gatewayConnectedRuntime{checkDependencies: func(context.Context) error { return nil }}
			wrapped := fmt.Errorf("private error detail: %w", errors.Join(context.DeadlineExceeded, tc.err))
			if class, permanent := runtimeStepErrorClass(wrapped); class != tc.class || !permanent {
				t.Fatalf("error class=%s permanent=%t", class, permanent)
			}
			calls := 0
			clock := startRuntimeRecovery(t, r, runtimeIdentityRelay, true, func(context.Context) error {
				calls++
				if calls == 1 {
					return wrapped
				}
				return nil
			})
			failed := clock.tick(t, runtimeStepPeriod)
			if got := r.ready(context.Background()); got != wrapped {
				t.Fatalf("permanent error readiness=%v", got)
			}
			close(failed.next)
			clock.tick(t, runtimeStepPeriod)
			if r.steps[runtimeIdentityRelay].Load() != nil || calls != 2 {
				t.Fatal("step did not pass after its error")
			}
			if got := r.ready(context.Background()); got != wrapped {
				t.Fatalf("permanent error lost after recovery: %v", got)
			}
		})
	}
	for _, err := range []error{delivery.ErrLease, delivery.ErrGap, context.Canceled, context.DeadlineExceeded, errors.New("unknown error")} {
		if _, permanent := runtimeStepErrorClass(err); permanent {
			t.Fatalf("recoverable error kept: %v", err)
		}
	}
}

func TestRuntimeStepRecoveryReadinessOrder(t *testing.T) {
	dependency := errors.New("dependency unavailable")
	r := &gatewayConnectedRuntime{checkDependencies: func(context.Context) error { return dependency }}
	for step := runtimeStepCount - 1; step >= 0; step-- {
		r.recordStep(step, fmt.Errorf("step %d unavailable", step))
	}
	for step := runtimeStep(0); step < runtimeStepCount; step++ {
		want := *r.steps[step].Load()
		if got := r.ready(context.Background()); got != want {
			t.Fatalf("step %s ready=%v, want %v", step.name(), got, want)
		}
		r.recordStep(step, nil)
	}
	if got := r.ready(context.Background()); got != dependency {
		t.Fatalf("dependency error=%v", got)
	}
	startup := errors.New("startup configuration rejected")
	r.fail(startup)
	r.recordStep(runtimeAuditStage, errors.New("stage unavailable"))
	r.fail(errors.New("later startup error"))
	if got := r.ready(context.Background()); got != startup {
		t.Fatalf("startup error precedence=%v", got)
	}
}

func TestRuntimeStepRecoveryTransitions(t *testing.T) {
	for step := runtimeStep(0); step < runtimeStepCount; step++ {
		t.Run(step.name(), func(t *testing.T) {
			var output bytes.Buffer
			r := &gatewayConnectedRuntime{logger: slog.New(slog.NewJSONHandler(&output, nil))}
			sensitive := "credential row subject statement address"
			calls := 0
			clock := startRuntimeRecovery(t, r, step, true, func(context.Context) error {
				calls++
				if calls == 1 || calls == 4 || calls == 5 {
					return nil
				}
				return fmt.Errorf("%s: %w", sensitive, context.DeadlineExceeded)
			})
			for i, count := range []int{0, 1, 1, 2, 2, 3} {
				delay := runtimeStepPeriod
				if i == 2 {
					delay *= 2
				}
				tick := clock.tick(t, delay)
				if got := bytes.Count(output.Bytes(), []byte("\n")); got != count {
					t.Fatalf("try %d log lines=%d, want %d: %s", i+1, got, count, output.String())
				}
				if i < 5 {
					close(tick.next)
				}
			}
			for i, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
				var entry map[string]any
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					t.Fatal(err)
				}
				state := "failing"
				if i == 1 {
					state = "passing"
				}
				if entry["step"] != step.name() || entry["state"] != state || entry["class"] != "timeout" || len(entry) != 6 {
					t.Fatalf("transition log=%s", line)
				}
				for _, text := range []string{sensitive, context.DeadlineExceeded.Error()} {
					if strings.Contains(line, text) {
						t.Fatalf("transition log contains error text: %s", line)
					}
				}
			}
		})
	}
}

func TestRuntimeStepRecoveryConsumerReturns(t *testing.T) {
	for _, step := range []runtimeStep{runtimeIdentityConsumer, runtimeAuditConsumer} {
		t.Run(step.name(), func(t *testing.T) {
			r := &gatewayConnectedRuntime{checkDependencies: func(context.Context) error { return nil }}
			runs := 0
			clock := startRuntimeRecovery(t, r, step, true, func(ctx context.Context) error {
				runs++
				if runs <= 2 {
					return errors.New("consumer fetch unavailable")
				}
				return nil
			})
			for i, delay := range []time.Duration{runtimeStepPeriod, 2 * runtimeStepPeriod, runtimeStepPeriod, runtimeStepPeriod} {
				tick := clock.tick(t, delay)
				if runs != i+1 || (r.ready(context.Background()) != nil) != (i < 2) {
					t.Fatalf("consumer runs=%d readiness=%v", runs, r.ready(context.Background()))
				}
				if i < 3 {
					close(tick.next)
				}
			}
		})
	}
}

func TestRuntimeStepRecoveryCancellation(t *testing.T) {
	for _, immediate := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r := &gatewayConnectedRuntime{ctx: ctx}
		calls := 0
		r.runStep(runtimeAuditStage, runtimeStepPeriod, immediate, func(context.Context) error {
			calls++
			return delivery.ErrIntegrity
		}, waitRuntimeStep)
		if calls != 0 || r.failure.Load() != nil || r.steps[runtimeAuditStage].Load() != nil {
			t.Fatal("canceled runtime ran a step")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &gatewayConnectedRuntime{ctx: ctx}
	r.runStep(runtimeAuditStage, runtimeStepPeriod, true, func(context.Context) error {
		cancel()
		return delivery.ErrIntegrity
	}, waitRuntimeStep)
	if r.failure.Load() != nil || r.steps[runtimeAuditStage].Load() != nil {
		t.Fatal("shutdown error changed step or runtime failure")
	}
}
