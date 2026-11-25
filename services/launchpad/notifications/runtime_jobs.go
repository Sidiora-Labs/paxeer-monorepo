package notifications

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
)

type runtimeWait func(context.Context, time.Duration) error

func waitRuntime(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextRuntimeBackoff(delay time.Duration) time.Duration {
	if delay >= 15*time.Second {
		return 30 * time.Second
	}
	return 2 * delay
}

type runtimeJob struct {
	name              string
	interval, timeout time.Duration
	pass              func(context.Context) error
}

type runtimePass struct {
	at  time.Time
	err error
}

type runtimeJobs struct {
	mu      sync.RWMutex
	running bool
	failure error
	passes  map[string]runtimePass
	wait    runtimeWait
}

func (r *runtimeJobs) ready() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.running || r.failure != nil || len(r.passes) == 0 {
		return ErrUnavailable
	}
	for _, pass := range r.passes {
		if pass.at.IsZero() || pass.err != nil {
			return ErrUnavailable
		}
	}
	return nil
}

func (r *runtimeJobs) run(ctx context.Context, jobs []runtimeJob) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return ErrInvalid
	}
	r.running, r.failure = true, nil
	r.passes = make(map[string]runtimePass, len(jobs))
	for _, job := range jobs {
		r.passes[job.name] = runtimePass{}
	}
	r.mu.Unlock()
	run, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan error, len(jobs))
	for _, job := range jobs {
		go func(job runtimeJob) { results <- r.runJob(run, job) }(job)
	}
	err := <-results
	cancel()
	for remaining := 1; remaining < len(jobs); remaining++ {
		err = errors.Join(err, <-results)
	}
	r.mu.Lock()
	r.running, r.failure = false, err
	r.mu.Unlock()
	return err
}

func (r *runtimeJobs) runJob(ctx context.Context, job runtimeJob) error {
	wait := r.wait
	if wait == nil {
		wait = waitRuntime
	}
	backoff := time.Second
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		bounded, cancel := context.WithTimeout(ctx, job.timeout)
		err := job.pass(bounded)
		cancel()
		r.mu.Lock()
		r.passes[job.name] = runtimePass{at: time.Now(), err: err}
		r.mu.Unlock()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		delay := job.interval
		if err != nil {
			log.Printf("notification runtime job %s failed: %v", job.name, err)
			if errors.Is(err, ErrInvalid) || errors.Is(err, discovery.ErrInvalid) {
				return fmt.Errorf("%s: %w", job.name, err)
			}
			delay = backoff
			backoff = nextRuntimeBackoff(backoff)
		} else {
			backoff = time.Second
		}
		if err := wait(ctx, delay); err != nil {
			return err
		}
	}
}
