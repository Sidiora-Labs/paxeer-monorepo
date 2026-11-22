package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
)

const consumerJobStablePeriod = 30 * time.Second

type consumerJobClock interface {
	Now() time.Time
	AfterFunc(time.Duration, func()) func() bool
}

type systemConsumerJobClock struct{}

func (systemConsumerJobClock) Now() time.Time { return time.Now() }

func (systemConsumerJobClock) AfterFunc(delay time.Duration, f func()) func() bool {
	return time.AfterFunc(delay, f).Stop
}

func waitConsumerJob(ctx context.Context, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func permanentConsumerJobError(err error) bool {
	if errors.Is(err, delivery.ErrAuthorization) || errors.Is(err, nats.ErrAuthorization) || errors.Is(err, nats.ErrPermissionViolation) ||
		errors.Is(err, delivery.ErrIntegrity) || errors.Is(err, delivery.ErrSchema) || errors.Is(err, natsdriver.ErrClosed) || errors.Is(err, nats.ErrConnectionClosed) {
		return true
	}
	var integrity *delivery.IntegrityFailure
	return errors.As(err, &integrity)
}

func consumerJobRunAgain(name string, health *statsRuntimeHealth, run func(context.Context) error, wait func(context.Context, time.Duration), clock consumerJobClock) func(context.Context) error {
	return func(ctx context.Context) error {
		var mu sync.Mutex
		failing := false
		record := func(err error) {
			health.set(name, err)
			if failing == (err != nil) {
				return
			}
			failing = err != nil
			state := "passing"
			if failing {
				state = "failing"
			}
			slog.InfoContext(ctx, "statistics consumer job", "job", name, "state", state, "error", err)
		}
		delay := time.Second
		for ctx.Err() == nil {
			started := clock.Now()
			active := true
			var stop func() bool
			mu.Lock()
			recovering := failing
			mu.Unlock()
			if recovering {
				stop = clock.AfterFunc(consumerJobStablePeriod, func() {
					mu.Lock()
					defer mu.Unlock()
					if active && ctx.Err() == nil {
						record(nil)
					}
				})
			}
			err := run(ctx)
			if stop != nil {
				stop()
			}
			mu.Lock()
			active = false
			elapsed := clock.Now().Sub(started)
			if ctx.Err() != nil {
				mu.Unlock()
				return ctx.Err()
			}
			if recovering && failing && elapsed >= consumerJobStablePeriod {
				record(nil)
			}
			if err == nil || permanentConsumerJobError(err) {
				mu.Unlock()
				return err
			}
			record(err)
			mu.Unlock()
			if elapsed >= consumerJobStablePeriod {
				delay = time.Second
			}
			wait(ctx, delay)
			delay = min(delay*2, consumerJobStablePeriod)
		}
		return ctx.Err()
	}
}
