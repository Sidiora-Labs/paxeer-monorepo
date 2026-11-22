package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
)

type runtimeJobWaitCall struct {
	ctx     context.Context
	delay   time.Duration
	release chan struct{}
}

type runtimeJobTestClock struct {
	elapsed atomic.Int64
	waits   chan runtimeJobWaitCall
}

func newRuntimeJobTest(t *testing.T) (*socialConnectedRuntime, *runtimeJobTestClock) {
	t.Helper()
	ctx, stop := context.WithCancel(context.Background())
	clock := &runtimeJobTestClock{waits: make(chan runtimeJobWaitCall, 32)}
	r := &socialConnectedRuntime{ctx: ctx, stop: stop, jobClock: func() time.Time {
		return time.Unix(0, clock.elapsed.Load())
	}, jobWait: func(ctx context.Context, delay time.Duration) {
		if ctx.Err() != nil {
			return
		}
		call := runtimeJobWaitCall{ctx: ctx, delay: delay, release: make(chan struct{})}
		select {
		case clock.waits <- call:
		case <-ctx.Done():
			return
		}
		select {
		case <-call.release:
		case <-ctx.Done():
		}
	}}
	t.Cleanup(func() { stop(); r.wg.Wait() })
	return r, clock
}

func (c *runtimeJobTestClock) nextWait(t *testing.T, want time.Duration) runtimeJobWaitCall {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case call := <-c.waits:
			if call.ctx.Err() != nil {
				continue
			}
			if call.delay != want {
				t.Fatalf("wait = %v, want %v", call.delay, want)
			}
			return call
		case <-timer.C:
			t.Fatal("job did not wait")
			return runtimeJobWaitCall{}
		}
	}
}

func runtimeJobCalled(t *testing.T, calls <-chan int, want int) {
	t.Helper()
	select {
	case got := <-calls:
		if got != want {
			t.Fatalf("run = %d, want %d", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("job did not run")
	}
}

func runtimeJobControlled(r *socialConnectedRuntime, job runtimeJob) (<-chan int, chan<- error) {
	calls, results := make(chan int, 1), make(chan error, 1)
	count := 0
	r.worker(job, func(ctx context.Context) error {
		count++
		select {
		case calls <- count:
		case <-ctx.Done():
			return ctx.Err()
		}
		select {
		case err := <-results:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	return calls, results
}

func TestRuntimeJobRunAgainRecoversWhileRunning(t *testing.T) {
	r, clock := newRuntimeJobTest(t)
	var logs bytes.Buffer
	r.logger = slog.New(slog.NewJSONHandler(&logs, nil))
	calls, results := runtimeJobControlled(r, runtimeJobChat)
	runtimeJobCalled(t, calls, 1)
	transient := errors.New("chat fetch unavailable")
	results <- transient
	retry := clock.nextWait(t, time.Second)
	if err := r.runtimeReady(r.ctx); !errors.Is(err, transient) || r.failure.Load() != nil {
		t.Fatalf("transient readiness = %v, terminal = %v", err, r.failure.Load())
	}
	select {
	case <-calls:
		t.Fatal("job ran before its retry wait ended")
	default:
	}
	close(retry.release)
	runtimeJobCalled(t, calls, 2)
	stable := clock.nextWait(t, 30*time.Second)
	clock.elapsed.Add(int64(29 * time.Second))
	close(stable.release)
	remaining := clock.nextWait(t, time.Second)
	if err := r.runtimeReady(r.ctx); !errors.Is(err, transient) {
		t.Fatalf("error cleared before 30 seconds: %v", err)
	}
	clock.elapsed.Add(int64(time.Second))
	close(remaining.release)
	brokerRecoveryAwait(t, func() bool { return r.runtimeReady(r.ctx) == nil })
	r.stop()
	r.wg.Wait()
	if strings.Count(logs.String(), `"state":"failing"`) != 1 || strings.Count(logs.String(), `"state":"passing"`) != 1 || strings.Count(logs.String(), `"job":"chat"`) != 2 {
		t.Fatalf("job transitions = %s", logs.String())
	}
}

func TestRuntimeJobRunAgainBackoffAndStableReset(t *testing.T) {
	r, clock := newRuntimeJobTest(t)
	calls, results := runtimeJobControlled(r, runtimeJobErasure)
	transient := fmt.Errorf("erasure fetch unavailable: %w", context.DeadlineExceeded)
	wants := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second, 30 * time.Second}
	for i, want := range wants {
		runtimeJobCalled(t, calls, i+1)
		if i > 0 {
			clock.nextWait(t, 30*time.Second)
		}
		results <- transient
		retry := clock.nextWait(t, want)
		if err := r.runtimeReady(r.ctx); !errors.Is(err, transient) {
			t.Fatalf("run %d readiness = %v", i+1, err)
		}
		close(retry.release)
	}
	runtimeJobCalled(t, calls, len(wants)+1)
	stable := clock.nextWait(t, 30*time.Second)
	clock.elapsed.Add(int64(30 * time.Second))
	close(stable.release)
	brokerRecoveryAwait(t, func() bool { return r.runtimeReady(r.ctx) == nil })
	results <- transient
	retry := clock.nextWait(t, time.Second)
	close(retry.release)
	runtimeJobCalled(t, calls, len(wants)+2)
	clock.nextWait(t, 30*time.Second)
	results <- transient
	clock.nextWait(t, 2*time.Second)
}

func TestRuntimeJobRunAgainPermanent(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"delivery_authorization", delivery.ErrAuthorization},
		{"broker_authorization", nats.ErrAuthorization},
		{"broker_permission", nats.ErrPermissionViolation},
		{"scope", sharedauth.ErrScopeDenied},
		{"integrity", delivery.ErrIntegrity},
		{"replay", sharedauth.ErrReplayMismatch},
		{"schema", delivery.ErrSchema},
		{"closed", natsdriver.ErrClosed},
		{"typed_integrity", &delivery.IntegrityFailure{Cause: errors.New("publication rejected")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, clock := newRuntimeJobTest(t)
			var calls atomic.Int64
			terminal := errors.Join(context.DeadlineExceeded, fmt.Errorf("job: %w", tc.err))
			r.worker(runtimeJobPolicy, func(context.Context) error { calls.Add(1); return terminal })
			r.wg.Wait()
			if got := r.failure.Load(); got == nil || *got != terminal {
				t.Fatalf("permanent failure = %v, want %v", got, terminal)
			}
			r.fail(errors.New("later permanent failure"))
			clock.elapsed.Add(int64(time.Hour))
			if err := r.runtimeReady(r.ctx); err != terminal || calls.Load() != 1 {
				t.Fatalf("permanent readiness = %v, calls = %d", err, calls.Load())
			}
			select {
			case call := <-clock.waits:
				t.Fatalf("permanent error scheduled wait %v", call.delay)
			default:
			}
		})
	}
}

func TestRuntimeJobRunAgainReadinessOrder(t *testing.T) {
	r, clock := newRuntimeJobTest(t)
	later := errors.New("policy pending")
	first := errors.New("chat pending")
	r.worker(runtimeJobPolicy, func(context.Context) error { return later })
	clock.nextWait(t, time.Second)
	r.worker(runtimeJobChat, func(context.Context) error { return first })
	clock.nextWait(t, time.Second)
	if err := r.runtimeReady(r.ctx); err != first {
		t.Fatalf("readiness order = %v, want %v", err, first)
	}
	r.recordJob(runtimeJobChat, nil)
	if err := r.runtimeReady(r.ctx); err != later {
		t.Fatalf("next job readiness = %v, want %v", err, later)
	}
	terminal := fmt.Errorf("durable: %w", delivery.ErrSchema)
	r.fail(terminal)
	if err := r.runtimeReady(r.ctx); err != terminal {
		t.Fatalf("terminal precedence = %v, want %v", err, terminal)
	}
}

func TestRuntimeJobRunAgainCancellation(t *testing.T) {
	for _, phase := range []string{"before_run", "during_run", "during_wait"} {
		t.Run(phase, func(t *testing.T) {
			r, clock := newRuntimeJobTest(t)
			if phase == "before_run" {
				r.stop()
			}
			var calls atomic.Int64
			running := make(chan struct{})
			r.worker(runtimeJobVoice, func(ctx context.Context) error {
				calls.Add(1)
				close(running)
				if phase == "during_run" {
					<-ctx.Done()
					return ctx.Err()
				}
				return context.DeadlineExceeded
			})
			if phase == "during_run" {
				select {
				case <-running:
				case <-time.After(2 * time.Second):
					t.Fatal("job did not start")
				}
			}
			if phase == "during_wait" {
				clock.nextWait(t, time.Second)
			}
			r.stop()
			r.wg.Wait()
			want := int64(1)
			if phase == "before_run" {
				want = 0
			}
			if calls.Load() != want || r.failure.Load() != nil {
				t.Fatalf("cancelled job calls = %d, want %d; terminal = %v", calls.Load(), want, r.failure.Load())
			}
		})
	}
}
