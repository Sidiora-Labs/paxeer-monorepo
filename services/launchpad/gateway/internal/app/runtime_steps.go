package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
)

type runtimeStep int

const (
	runtimeAuditStage runtimeStep = iota
	runtimeAuditRelay
	runtimeIdentityRelay
	runtimeIdentityConsumer
	runtimeAuditConsumer
	runtimeStepCount
	runtimeStepPeriod   = 250 * time.Millisecond
	runtimeStepMaxDelay = 30 * time.Second
)

func (step runtimeStep) name() string {
	return [...]string{"audit_stage", "audit_relay", "identity_relay", "identity_consumer", "audit_consumer"}[step]
}

func runtimeStepErrorClass(err error) (string, bool) {
	switch {
	case errors.Is(err, delivery.ErrAuthorization), errors.Is(err, nats.ErrAuthorization), errors.Is(err, nats.ErrPermissionViolation), errors.Is(err, sharedauth.ErrScopeDenied):
		return "authorization", true
	case errors.Is(err, delivery.ErrIntegrity), errors.Is(err, sharedauth.ErrReplayMismatch):
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

func (r *gatewayConnectedRuntime) recordStep(step runtimeStep, err error) {
	var current *error
	if err != nil {
		current = &err
		if _, permanent := runtimeStepErrorClass(err); permanent {
			r.fail(err)
		}
	}
	previous := r.steps[step].Swap(current)
	if r.logger == nil || (previous == nil) == (current == nil) {
		return
	}
	state := "failing"
	if err == nil {
		state, err = "passing", *previous
	}
	class, _ := runtimeStepErrorClass(err)
	r.logger.Info("gateway runtime step", slog.String("step", step.name()), slog.String("state", state), slog.String("class", class))
}

func waitRuntimeStep(ctx context.Context, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (r *gatewayConnectedRuntime) runStep(step runtimeStep, period time.Duration, immediate bool, run func(context.Context) error, wait func(context.Context, time.Duration)) {
	delay := period
	if !immediate {
		wait(r.ctx, delay)
	}
	for r.ctx.Err() == nil {
		err := run(r.ctx)
		if r.ctx.Err() != nil {
			return
		}
		r.recordStep(step, err)
		if err == nil {
			delay = period
		}
		wait(r.ctx, delay)
		if err != nil {
			delay = min(delay*2, runtimeStepMaxDelay)
		}
	}
}

func (r *gatewayConnectedRuntime) startStep(step runtimeStep, immediate bool, run func(context.Context) error) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.runStep(step, runtimeStepPeriod, immediate, run, waitRuntimeStep)
	}()
}

func (r *gatewayConnectedRuntime) consumerStep(step runtimeStep, fetch func(context.Context) ([]*natsdriver.Message, error), consume func(context.Context, *natsdriver.Message) error) func(context.Context) error {
	return func(ctx context.Context) error {
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			messages, err := fetch(ctx)
			if err != nil {
				return err
			}
			r.recordStep(step, nil)
			for _, message := range messages {
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := consume(ctx, message); err != nil {
					return fmt.Errorf("natsdriver: transactional consume: %w", err)
				}
			}
		}
	}
}
