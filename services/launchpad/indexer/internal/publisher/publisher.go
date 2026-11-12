// Package publisher provides best-effort Redis realtime fan-out.
// Compatibility webhooks are retired: they could lose target work after cursor
// advancement and never provided durable acknowledgement or restart recovery.
// Durable publication uses DurableStager and DurableWebhookStager inside the
// source transaction instead of PublishBatch.
package publisher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/checked"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"
)

// eventChannel maps an indexer event name to its Redis pub/sub channel for the
// realtime fan-out path. Events with no realtime channel (e.g. TokenForTokenSwap)
// are intentionally absent; use durable staging for their downstream delivery.
var eventChannel = map[string]string{
	"MarketCreated":      constants.ChannelMarketCreated,
	"Swap":               constants.ChannelSwap,
	"PoolStateUpdated":   constants.ChannelPoolStateUpdated,
	"FeeRecorded":        constants.ChannelFeeRecorded,
	"FeeDistributed":     constants.ChannelFeeDistributed,
	"FeeStrategyChanged": constants.ChannelFeeStrategyChanged,
	"OpticalExecuted":    constants.ChannelOpticalExecuted,
	"ConfigUpdated":      constants.ChannelConfigUpdated,
}

const (
	httpTimeout = 10 * time.Second
	dedupTTL    = 600 * time.Second
)

// ErrLegacyWebhookRetired rejects compatibility target configuration before
// indexing can advance a cursor. Use transactionally staged durable delivery.
var ErrLegacyWebhookRetired = errors.New("publisher: compatibility webhook delivery is retired; use transactionally staged DurableWebhookStager delivery")

// WebhookEvent is the wire shape of a single fanned-out event. Field names
// (and JSON tags) match the TS WebhookEvent exactly so a Go indexer can feed TS
// consumers and vice-versa.
type WebhookEvent struct {
	EventName      string         `json:"eventName"`
	BlockNumber    int64          `json:"blockNumber"`
	BlockTimestamp int64          `json:"blockTimestamp"`
	TxHash         string         `json:"txHash"`
	LogIndex       int            `json:"logIndex"`
	Args           map[string]any `json:"args"`
	TraceID        string         `json:"traceId,omitempty"`
}

// Target is a retired compatibility webhook configuration. New rejects targets.
type Target struct {
	URL    string
	Secret string
}

// DeadLetterEntry retains the status API shape of retired compatibility delivery.
type DeadLetterEntry struct {
	Events    []WebhookEvent `json:"events"`
	TargetURL string         `json:"targetUrl"`
	FailedAt  int64          `json:"failedAt"`
	Attempts  int            `json:"attempts"`
	LastError string         `json:"lastError"`
}

// Metrics is a snapshot of publisher counters for the /status endpoint.
type Metrics struct {
	TotalEventsPublished int64 `json:"totalEventsPublished"`
	TotalDeliveries      int64 `json:"totalDeliveries"`
	TotalFailures        int64 `json:"totalFailures"`
	TotalDeadLettered    int64 `json:"totalDeadLettered"`
	TotalDeduplicated    int64 `json:"totalDeduplicated"`
	TotalRedisPublishes  int64 `json:"totalRedisPublishes"`
	TotalRedisFailures   int64 `json:"totalRedisFailures"`
	CurrentInflight      int64 `json:"currentInflight"`
}

// BuildTargets rejects retired compatibility webhook URLs. The application
// calls this before constructing its processor, so configured targets stop
// startup rather than accepting work that cannot survive a restart.
func BuildTargets(urls []string, defaultSecret string, perURL []string) ([]Target, error) {
	if len(urls) != 0 {
		return nil, ErrLegacyWebhookRetired
	}
	return []Target{}, nil
}

// Options configures a Publisher.
type Options struct {
	// Deprecated: nonempty Targets cause New to panic with ErrLegacyWebhookRetired.
	Targets []Target
	Logger  *slog.Logger
	// Redis is optional; it provides best-effort realtime fan-out and batch dedup.
	Redis *goredis.Client
	// Deprecated: Attempts belongs to the retired compatibility webhook mode.
	Attempts int
}

// Publisher publishes best-effort realtime events to Redis.
type Publisher struct {
	logger *slog.Logger
	redis  *goredis.Client

	backfill atomic.Bool
	inflight atomic.Int64
	wg       sync.WaitGroup

	dlqMu sync.Mutex
	dlq   []DeadLetterEntry

	m struct {
		published   atomic.Int64
		deliveries  atomic.Int64
		failures    atomic.Int64
		deadLetters atomic.Int64
		deduped     atomic.Int64
		redisPub    atomic.Int64
		redisFail   atomic.Int64
	}
}

// New builds a realtime Publisher. It panics with ErrLegacyWebhookRetired for
// any target configuration, including direct callers that bypass BuildTargets.
func New(opts Options) *Publisher {
	if len(opts.Targets) != 0 {
		panic(ErrLegacyWebhookRetired)
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Publisher{logger: logger, redis: opts.Redis}
}

// SetBackfillMode suppresses realtime fan-out during backfill.
func (p *Publisher) SetBackfillMode(enabled bool) {
	p.backfill.Store(enabled)
	p.logger.Info("publisher: backfill mode toggled", slog.Bool("backfillMode", enabled))
}

// PublishBatch schedules best-effort Redis pub/sub. It does not stage durable
// work, deliver compatibility webhooks or acknowledge source progress.
func (p *Publisher) PublishBatch(ctx context.Context, events []WebhookEvent) {
	if len(events) == 0 || p.backfill.Load() {
		return
	}

	// Idempotency guard (i4): skip a batch we've already fanned out (restart
	// redelivery) so realtime fan-out is not repeated. Best-effort — Redis errors fall
	// through to delivery.
	if p.redis != nil && p.isDuplicate(ctx, events) {
		p.m.deduped.Add(int64(len(events)))
		p.logger.Info("publisher: duplicate batch skipped (i4)", slog.Int("count", len(events)))
		return
	}

	// Realtime fan-out via Redis pub/sub. One goroutine per batch so the
	// per-event publish order is preserved; tracked by wg for graceful drain.
	if p.redis != nil {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.publishRedis(ctx, events)
		}()
	}
}

// publishRedis PUBLISHes each WebhookEvent to its realtime Redis channel.
// Failures are logged and counted; this path has no delivery or restart-recovery
// guarantee.
func (p *Publisher) publishRedis(ctx context.Context, events []WebhookEvent) {
	for i := range events {
		ch, ok := eventChannel[events[i].EventName]
		if !ok {
			continue
		}
		body, err := json.Marshal(events[i])
		if err != nil {
			p.logger.Error("publisher: encode redis event failed",
				slog.String("event", events[i].EventName), slog.String("err", err.Error()))
			continue
		}
		if err := p.redis.Publish(ctx, ch, body).Err(); err != nil {
			p.m.redisFail.Add(1)
			p.logger.Warn("publisher: redis publish failed",
				slog.String("channel", ch), slog.String("err", err.Error()))
			continue
		}
		p.m.redisPub.Add(1)
	}
}

// isDuplicate computes the batch fingerprint and reserves it in Redis (SET NX).
// Returns true when the key already existed (duplicate batch).
func (p *Publisher) isDuplicate(ctx context.Context, events []WebhookEvent) bool {
	h := sha256.New()
	for i, e := range events {
		if i > 0 {
			h.Write([]byte(","))
		}
		checked.Write(fmt.Fprintf(h, "%s:%d", e.TxHash, e.LogIndex))
	}
	fp := hex.EncodeToString(h.Sum(nil))[:16]
	key := "webhook:dedup:" + fp
	set, err := p.redis.SetNX(ctx, key, "1", dedupTTL).Result()
	if err != nil {
		return false // Redis unavailable — proceed without dedup.
	}
	return !set
}

// DeadLetterQueue retains the retired compatibility status API; no new entries are accepted.
func (p *Publisher) DeadLetterQueue() []DeadLetterEntry {
	p.dlqMu.Lock()
	defer p.dlqMu.Unlock()
	out := make([]DeadLetterEntry, len(p.dlq))
	copy(out, p.dlq)
	return out
}

// DeadLetterCount returns the number of dead-letter entries.
func (p *Publisher) DeadLetterCount() int {
	p.dlqMu.Lock()
	defer p.dlqMu.Unlock()
	return len(p.dlq)
}

// DrainDeadLetterQueue empties and returns the dead-letter queue.
func (p *Publisher) DrainDeadLetterQueue() []DeadLetterEntry {
	p.dlqMu.Lock()
	defer p.dlqMu.Unlock()
	drained := p.dlq
	p.dlq = nil
	return drained
}

// Snapshot returns the current metrics.
func (p *Publisher) Snapshot() Metrics {
	return Metrics{
		TotalEventsPublished: p.m.published.Load(),
		TotalDeliveries:      p.m.deliveries.Load(),
		TotalFailures:        p.m.failures.Load(),
		TotalDeadLettered:    p.m.deadLetters.Load(),
		TotalDeduplicated:    p.m.deduped.Load(),
		TotalRedisPublishes:  p.m.redisPub.Load(),
		TotalRedisFailures:   p.m.redisFail.Load(),
		CurrentInflight:      p.inflight.Load(),
	}
}

// WaitForInflight blocks until all in-flight deliveries finish or timeout
// elapses (graceful drain — invariant i7).
func (p *Publisher) WaitForInflight(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		p.logger.Warn("publisher: timed out waiting for inflight publications", slog.Int64("inflight", p.inflight.Load()))
	}
}

// Disconnect waits for in-flight realtime publications to drain (default 10s).
func (p *Publisher) Disconnect() { p.WaitForInflight(httpTimeout) }
