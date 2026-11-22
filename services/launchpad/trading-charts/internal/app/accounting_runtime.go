package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
)

var ErrInternalFailure = errors.New("charts: internal runtime failure")

const accountingRecoveryPeriod = 30 * time.Second

type accountingClock struct {
	now   func() time.Time
	timer func(time.Duration) (<-chan time.Time, func())
}

func accountingSystemClock() accountingClock {
	return accountingClock{now: time.Now, timer: func(delay time.Duration) (<-chan time.Time, func()) {
		timer := time.NewTimer(delay)
		return timer.C, func() { timer.Stop() }
	}}
}

func accountingErrorClass(err error) (string, bool) {
	switch {
	case errors.Is(err, delivery.ErrAuthorization), errors.Is(err, nats.ErrAuthorization), errors.Is(err, nats.ErrPermissionViolation):
		return "authorization", true
	case errors.Is(err, delivery.ErrIntegrity):
		return "integrity", true
	case errors.Is(err, delivery.ErrSchema):
		return "schema", true
	case errors.Is(err, natsdriver.ErrClosed):
		return "closed", true
	}
	var integrity *delivery.IntegrityFailure
	if errors.As(err, &integrity) {
		return "integrity", true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", false
	}
	return "transient", false
}

func waitAccountingConsumer(ctx context.Context, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func runAccountingConsumer(ctx context.Context, run func(context.Context) error, failRuntime func(), logger *slog.Logger, wait func(context.Context, time.Duration), clock accountingClock) {
	delay := time.Second
	failing := false
	passing := func() {
		logger.Info("accounting consumer recovered", slog.String("job", "accounting_consumer"), slog.String("state", "passing"))
		failing = false
	}
	for ctx.Err() == nil {
		started := clock.now()
		var recovered <-chan time.Time
		stop := func() {}
		if failing {
			recovered, stop = clock.timer(accountingRecoveryPeriod)
		}
		result := make(chan error, 1)
		go func() { result <- run(ctx) }()
		var err error
		for result != nil {
			select {
			case err = <-result:
				result = nil
			case <-recovered:
				recovered = nil
				if ctx.Err() == nil {
					passing()
				}
			}
		}
		stop()
		if clock.now().Sub(started) >= accountingRecoveryPeriod {
			delay = time.Second
			if failing && ctx.Err() == nil {
				passing()
			}
		}
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return
		}
		class, permanent := accountingErrorClass(err)
		if !failing {
			logger.Error("accounting consumer failed", slog.Any("err", err), slog.String("job", "accounting_consumer"), slog.String("state", "failing"), slog.String("class", class))
			failing = true
		}
		if permanent {
			failRuntime()
			return
		}
		wait(ctx, delay)
		delay = min(2*delay, accountingRecoveryPeriod)
	}
}
