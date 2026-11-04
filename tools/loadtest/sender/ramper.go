package sender

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/paxeer-network/pax-load/stats"
	"github.com/paxeer-network/pax-load/utils/service"
	"golang.org/x/time/rate"
)

// This will manage the ramping process for the loadtest
// Ramping loadtest will being at the StartTps and spend LoadTime at each step, ending when we violate the chain SLO of
// 1 block per second over a given ramp period (as measured in the back half of the ramp time)
// If we successfully pass a given TPS, we will pause for PauseTime, and then start the next step.
// If we fail to pass a given TPS, we will stop the loadtest.

var ErrRampTestFailedSLO = errors.New("Ramp Test failed SLO")

func (r *Ramper) FormatRampStats() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return fmt.Sprintf(`
─────────────────────────────────────────
              RAMP STATISTICS
─────────────────────────────────────────
 Ramp Curve Stats:
 %s
─────────────────────────────────────────
 Window Block Stats:
 %s
─────────────────────────────────────────`,
		r.rampCurve.GetCurveStats(), r.lastWindowStats.FormatBlockStats())
}

type Ramper struct {
	mu                sync.Mutex
	lastProgress      time.Time
	lastHeight        uint64
	baselineHeight    uint64
	lastUpdate        time.Time
	noProgressTimeout time.Duration

	sharedLimiter   *rate.Limiter
	blockCollector  stats.BlockStatsProvider
	currentTps      float64
	startTime       time.Time
	rampCurve       RampCurve
	lastWindowStats stats.BlockStats
}

// RampCurve is a function that returns the target TPS at a given time in the ramp period
type RampCurve interface {
	GetTPS(t time.Duration) float64
	GetCurveStats() string
}

func NewRamper(rampCurve RampCurve, blockCollector stats.BlockStatsProvider, sharedLimiter *rate.Limiter) *Ramper {
	sharedLimiter.SetLimit(rate.Limit(1)) // reset limiter to 1
	return &Ramper{
		sharedLimiter:     sharedLimiter,
		blockCollector:    blockCollector,
		rampCurve:         rampCurve,
		noProgressTimeout: 5 * time.Second,
		lastProgress:      time.Now(),
		lastUpdate:        time.Now(),
		lastHeight:        blockCollector.GetBlockStats().MaxBlockNumber,
		baselineHeight:    blockCollector.GetBlockStats().MaxBlockNumber,
	}
}

// BlockProgressEvidence carries both sample coverage and locally observed committed
// height advancement. A percentile alone cannot establish a healthy chain.
type BlockProgressEvidence struct {
	SampleCount         int
	LastCommittedHeight uint64
	LastProgress        time.Time
}

func (r *Ramper) progressEvidence(now time.Time) BlockProgressEvidence {
	window := r.blockCollector.GetWindowBlockStats()
	if window.MaxBlockNumber > r.lastHeight {
		r.lastHeight = window.MaxBlockNumber
		r.lastProgress = now
	}
	return BlockProgressEvidence{window.SampleCount, r.lastHeight, r.lastProgress}
}

func (r *Ramper) UpdateTPS() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	evidence := r.progressEvidence(now)
	if evidence.SampleCount < 2 || evidence.LastCommittedHeight <= r.baselineHeight || now.Sub(evidence.LastProgress) >= r.noProgressTimeout {
		// Freeze the curve as well as the limiter while waiting for fresh evidence.
		r.startTime = r.startTime.Add(now.Sub(r.lastUpdate))
		r.currentTps = 1
	} else {
		r.currentTps = r.rampCurve.GetTPS(now.Sub(r.startTime))
	}
	r.lastUpdate = now
	r.sharedLimiter.SetLimit(rate.Limit(r.currentTps))
}

func (r *Ramper) LogFinalStats() {
	log.Printf("Final Ramp stats: \n%s", r.FormatRampStats())
}

// WatchSLO will evaluate the chain SLO every 100ms using a 30 second window, and return a channel if the SLO is violated
func (r *Ramper) WatchSLO(ctx context.Context) <-chan struct{} {
	ch := make(chan struct{}, 1)
	r.blockCollector.ResetWindowStats()
	go func() {
		defer close(ch)
		checks := time.NewTicker(100 * time.Millisecond)
		windows := time.NewTicker(30 * time.Second)
		defer checks.Stop()
		defer windows.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-checks.C:
				r.mu.Lock()
				evidence := r.progressEvidence(now)
				stalled := now.Sub(evidence.LastProgress) >= r.noProgressTimeout
				r.mu.Unlock()
				p90 := r.blockCollector.GetWindowBlockTimePercentile(90)
				if stalled || p90 > time.Second {
					log.Printf("Ramp SLO failed: samples=%d committed_height=%d last_progress=%s p90=%s", evidence.SampleCount, evidence.LastCommittedHeight, evidence.LastProgress, p90)
					ch <- struct{}{}
					return
				}
			case <-windows.C:
				r.mu.Lock()
				r.lastWindowStats = r.blockCollector.GetWindowBlockStats()
				r.blockCollector.ResetWindowStats()
				r.mu.Unlock()
			}
		}
	}()
	return ch
}

func (r *Ramper) Run(ctx context.Context) error {
	return service.Run(ctx, func(ctx context.Context, s service.Scope) error {
		r.mu.Lock()
		r.startTime = time.Now()
		r.lastProgress = r.startTime
		r.lastUpdate = r.startTime
		r.lastHeight = r.blockCollector.GetBlockStats().MaxBlockNumber
		r.baselineHeight = r.lastHeight
		r.mu.Unlock()
		watchCtx, stopWatch := context.WithCancel(ctx)
		defer stopWatch()
		sloChan := r.WatchSLO(watchCtx)
		updates := time.NewTicker(100 * time.Millisecond)
		defer updates.Stop()
		defer r.sharedLimiter.SetLimit(1)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-sloChan:
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return ErrRampTestFailedSLO
			case <-updates.C:
				r.UpdateTPS()
			}
		}
	})
}

type RampCurveStep struct {
	StartTps         float64
	IncrementTps     float64
	LoadInterval     time.Duration
	RecoveryInterval time.Duration
	Step             int
	CurrentTPS       float64
}

func NewRampCurveStep(startTps float64, incrementTps float64, loadInterval time.Duration, recoveryInterval time.Duration) *RampCurveStep {
	return &RampCurveStep{
		StartTps:         startTps,
		IncrementTps:     incrementTps,
		LoadInterval:     loadInterval,
		RecoveryInterval: recoveryInterval,
		Step:             0,
		CurrentTPS:       startTps,
	}
}

func (r *RampCurveStep) GetStartTps() float64 {
	return r.StartTps
}

func (r *RampCurveStep) GetIncrementTps() float64 {
	return r.IncrementTps
}

func (r *RampCurveStep) GetTPS(t time.Duration) float64 {
	// figure out where we are in the load interval
	cycleInterval := r.LoadInterval + r.RecoveryInterval
	cycleProgress := t % cycleInterval

	// if we're in the recovery interval, return 1.00 (close to 0 but doesn't fully block the limiter)
	if cycleProgress > r.LoadInterval {
		return 1.00
	}

	cycleNumber := int(t / cycleInterval)

	// this means we're in a new step, so we need to update step and TPS
	if cycleNumber > r.Step {
		r.Step = cycleNumber
		newTps := r.StartTps + r.IncrementTps*float64(r.Step)
		log.Printf("📈 Ramping to %f TPS for %v", newTps, r.LoadInterval)
		r.CurrentTPS = newTps
		return newTps
	}

	return r.CurrentTPS
}

// this should return the highest target TPS that is PRIOR to the current step
func (r *RampCurveStep) GetCurveStats() string {
	step := r.Step - 1
	if step < 0 {
		return "no ramp curve stats available"
	}
	return fmt.Sprintf("Highest Passed TPS: %.2f", r.StartTps+r.IncrementTps*float64(step))
}
