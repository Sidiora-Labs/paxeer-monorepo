package stats

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/paxeer-network/paxlog"
)

// Global tracker state
var (
	logger = paxlog.NewLogger("evmrpc", "stats")

	httpTracker trackerSlot
	wsTracker   trackerSlot
)

type apiEvent struct {
	Method    string
	Duration  time.Duration
	Success   bool
	StartTime time.Time
	EndTime   time.Time
}

// periodStats holds aggregated stats for a time period.
type periodStats struct {
	periodStart  time.Time
	totalEvents  int
	totalSuccess int
	methodData   map[string]*methodStats
}

// methodStats holds per-method aggregated stats.
type methodStats struct {
	count        int
	successCount int
	totalLatency time.Duration
	maxLatency   time.Duration
}

type trackerSlot struct {
	mu      sync.RWMutex
	current *tracker
}

// InitRPCTracker replaces the HTTP/RPC tracker and returns its owned handle.
// A non-positive interval disables tracking and stops the previous tracker.
func InitRPCTracker(ctx context.Context, interval time.Duration) *tracker {
	return httpTracker.replace(ctx, "http", interval)
}

// InitWSTracker replaces the WebSocket tracker and returns its owned handle.
// A non-positive interval disables tracking and stops the previous tracker.
func InitWSTracker(ctx context.Context, interval time.Duration) *tracker {
	return wsTracker.replace(ctx, "ws", interval)
}

func (s *trackerSlot) replace(ctx context.Context, connType string, interval time.Duration) *tracker {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil {
		s.current.stop()
		s.current = nil
	}
	if interval <= 0 {
		return nil
	}
	t := newTracker(ctx, connType, interval)
	t.owner = s
	s.current = t
	return t
}

type tracker struct {
	owner    *trackerSlot
	logger   *slog.Logger
	ch       chan apiEvent
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	// Simple current period tracking
	mu            sync.RWMutex
	currentPeriod *periodStats
	connType      string
}

// newTracker creates a new stats tracker.
func newTracker(ctx context.Context,
	connType string, interval time.Duration) *tracker {
	trackerCtx, cancel := context.WithCancel(ctx)

	t := &tracker{
		ch:       make(chan apiEvent, 10000),
		interval: interval,
		ctx:      trackerCtx,
		cancel:   cancel,
		connType: connType,
		logger:   logger.With("connType", connType),
	}

	t.wg.Add(1)
	go t.run()
	return t
}

// run processes events continuously and reports periods on interval
func (t *tracker) run() {
	defer t.wg.Done()

	// Report stats every interval.
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	t.logger.Info("stats tracker started", "interval", t.interval.String())

	for {
		select {
		case <-t.ctx.Done():
			t.logger.Info("stats tracker stopping", "reason", t.ctx.Err())
			// Join in-flight producers before draining the final accepted events.
			t.mu.Lock()
			t.mu.Unlock()
			for {
				select {
				case event := <-t.ch:
					t.processEvent(event)
				default:
					t.reportCurrentPeriod()
					return
				}
			}
		case event := <-t.ch:
			// Process event immediately
			t.processEvent(event)
		case <-ticker.C:
			// Report current period and start fresh
			t.reportCurrentPeriod()
		}
	}
}

// processEvent aggregates an event into the current period
func (t *tracker) processEvent(event apiEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Truncate event end time to period boundary (use completion time for period attribution)
	eventPeriod := event.EndTime.Truncate(t.interval)

	// Check if we need to rotate periods
	if t.currentPeriod != nil && eventPeriod.After(t.currentPeriod.periodStart) {
		// apiEvent is in a new period, so report the current period and start fresh
		t.reportPeriodLocked(t.currentPeriod)
		t.currentPeriod = nil
	}

	// Initialize current period if needed (using event timestamp)
	if t.currentPeriod == nil {
		t.currentPeriod = &periodStats{
			periodStart:  eventPeriod,
			methodData:   make(map[string]*methodStats),
			totalEvents:  0,
			totalSuccess: 0,
		}
	}

	// Update overall stats
	t.currentPeriod.totalEvents++
	if event.Success {
		t.currentPeriod.totalSuccess++
	}

	// Get or create method stats
	method := t.currentPeriod.methodData[event.Method]
	if method == nil {
		method = &methodStats{}
		t.currentPeriod.methodData[event.Method] = method
	}

	// Update method stats
	method.count++
	method.totalLatency += event.Duration
	if event.Success {
		method.successCount++
	}
	if event.Duration > method.maxLatency {
		method.maxLatency = event.Duration
	}
}

// reportCurrentPeriod reports the current period and starts a new one
func (t *tracker) reportCurrentPeriod() {
	t.mu.Lock()
	defer t.mu.Unlock()

	// If no current period, create an empty one for this interval
	if t.currentPeriod == nil {
		t.currentPeriod = &periodStats{
			periodStart:  time.Now().Truncate(t.interval),
			methodData:   make(map[string]*methodStats),
			totalEvents:  0,
			totalSuccess: 0,
		}
	}

	// Report the current period
	t.reportPeriodLocked(t.currentPeriod)

	// Start a new period
	t.currentPeriod = nil
}

// reportPeriodLocked logs the stats for a completed period (assumes lock is held)
func (t *tracker) reportPeriodLocked(period *periodStats) {
	// Always log overall stats, even for periods with no events
	if period.totalEvents == 0 {
		// Log overall stats for periods with no requests
		t.logger.Info("stats",
			"period", period.periodStart.Format("2006-01-02T15:04:05Z"),
			"count", 0,
			"interval", t.interval.String(),
		)
		return // No method stats to report
	}

	// Calculate overall success rate
	overallSuccessRate := float64(period.totalSuccess) / float64(period.totalEvents) * 100

	// Log overall stats for this period
	t.logger.Info("stats",
		"period", period.periodStart.Format("2006-01-02T15:04:05Z"),
		"count", period.totalEvents,
		"success_rate_pct", overallSuccessRate,
		"interval", t.interval.String(),
	)

	// Log per-method stats for this period
	for method, stats := range period.methodData {

		// Calculate average latency in milliseconds with 2 decimal places
		avgLatencyMs := float64(stats.totalLatency.Nanoseconds()) / float64(stats.count) / 1000000.0
		maxLatencyMs := float64(stats.maxLatency.Nanoseconds()) / 1000000.0

		t.logger.Info("method stats",
			"period", period.periodStart.Format("2006-01-02T15:04:05Z"),
			"method", method,
			"count", stats.count,
			"success", stats.successCount,
			"fails", stats.count-stats.successCount,
			"latency_avg", avgLatencyMs,
			"latency_max", maxLatencyMs,
		)
	}
}

// TrackMessage tracks a JSON-RPC method call with timing information
func (t *tracker) TrackMessage(method string, connectionType string, startTime time.Time, success bool) {
	if t == nil {
		return // Gracefully handle nil tracker
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.ctx.Err() != nil {
		return
	}

	endTime := time.Now()
	event := apiEvent{
		Method:    method,
		Duration:  endTime.Sub(startTime),
		Success:   success,
		StartTime: startTime,
		EndTime:   endTime,
	}

	select {
	case t.ch <- event:
	default:
		// Drop on overflow to not block RPC - log at debug level to avoid spam
		t.logger.Debug("event channel full, dropping event",
			"method", method,
			"connection", connectionType)
	}
}

// Stop releases this handle without disabling a newer replacement.
func (t *tracker) Stop() {
	if t == nil {
		return
	}
	if t.owner != nil {
		t.owner.mu.Lock()
		defer t.owner.mu.Unlock()
		if t.owner.current == t {
			t.owner.current = nil
		}
	}
	t.stop()
}

func (t *tracker) stop() {
	t.cancel()
	t.wg.Wait()
}

// RecordAPIInvocation records a call using the active tracker for its transport.
func RecordAPIInvocation(method string, connectionType string, startTime time.Time, success bool) {
	var slot *trackerSlot
	switch connectionType {
	case "http":
		slot = &httpTracker
	case "websocket":
		slot = &wsTracker
	default:
		return
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	slot.current.TrackMessage(method, connectionType, startTime, success)
}
