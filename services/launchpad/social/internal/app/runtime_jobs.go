package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
)

type runtimeJob int

const (
	runtimeJobChat runtimeJob = iota
	runtimeJobNotificationCredentials
	runtimeJobErasure
	runtimeJobPolicy
	runtimeJobVoice
	runtimeJobDurable
	runtimeJobCount
	runtimeJobStable = 30 * time.Second
)

func (job runtimeJob) name() string {
	return [...]string{"chat", "notification_credentials", "erasure", "policy", "voice", "durable"}[job]
}

func runtimeJobPermanent(err error) bool {
	switch {
	case errors.Is(err, delivery.ErrAuthorization), errors.Is(err, nats.ErrAuthorization), errors.Is(err, nats.ErrPermissionViolation), errors.Is(err, sharedauth.ErrScopeDenied):
		return true
	case errors.Is(err, delivery.ErrIntegrity), errors.Is(err, sharedauth.ErrReplayMismatch), errors.Is(err, delivery.ErrSchema), errors.Is(err, natsdriver.ErrClosed):
		return true
	}
	var integrity *delivery.IntegrityFailure
	return errors.As(err, &integrity)
}

func waitRuntimeJob(ctx context.Context, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (r *socialConnectedRuntime) recordJob(job runtimeJob, err error) {
	var current *error
	if err != nil {
		current = &err
	}
	previous := r.jobs[job].Swap(current)
	if r.logger == nil || (previous == nil) == (current == nil) {
		return
	}
	state := "failing"
	if err == nil {
		state = "passing"
	}
	r.logger.Info("social runtime job", slog.String("job", job.name()), slog.String("state", state))
}

func (r *socialConnectedRuntime) worker(job runtimeJob, run func(context.Context) error) {
	wait, now := r.jobWait, r.jobClock
	if wait == nil {
		wait = waitRuntimeJob
	}
	if now == nil {
		now = time.Now
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		delay := time.Second
		for r.ctx.Err() == nil {
			started := now()
			stableCtx, cancel := context.WithCancel(r.ctx)
			stableDone := make(chan struct{})
			go func() {
				defer close(stableDone)
				if r.jobs[job].Load() == nil {
					return
				}
				for remaining := runtimeJobStable - now().Sub(started); remaining > 0; remaining = runtimeJobStable - now().Sub(started) {
					wait(stableCtx, remaining)
					if stableCtx.Err() != nil {
						return
					}
				}
				if stableCtx.Err() == nil {
					r.recordJob(job, nil)
				}
			}()
			err := run(r.ctx)
			cancel()
			<-stableDone
			if r.ctx.Err() != nil {
				return
			}
			if now().Sub(started) >= runtimeJobStable {
				r.recordJob(job, nil)
				delay = time.Second
			}
			if err == nil {
				return
			}
			if runtimeJobPermanent(err) {
				r.fail(err)
				r.recordJob(job, err)
				return
			}
			r.recordJob(job, err)
			wait(r.ctx, delay)
			delay = min(delay*2, runtimeJobStable)
		}
	}()
}
