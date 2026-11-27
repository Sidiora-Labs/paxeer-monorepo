package consumerhealth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

// MaxRetryPartitions caps retained hashed retry identities; overflow makes the
// retry partition gauge a lower bound until the process restarts.
const MaxRetryPartitions = 10000

const SourceUnavailableClass = "source_unavailable"

var ErrNoProgress = errors.New("accounting consumer has failed deliveries without progress")

type Snapshot struct {
	LastSuccess         time.Time
	LastError           time.Time
	ErrorClass          string
	ConsecutiveFailures uint64
	RetryPartitions     int
	RetryOverflow       bool
}

type State struct {
	mu          sync.Mutex
	name        string
	bound       time.Duration
	now         func() time.Time
	started     time.Time
	logger      *slog.Logger
	snapshot    Snapshot
	retries     map[[sha256.Size]byte][sha256.Size]byte
	retryGroups map[[sha256.Size]byte]int
	failed      bool
	unavailable bool
}

func New(name string, bound time.Duration, logger *slog.Logger, now func() time.Time) *State {
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	return &State{name: name, bound: bound, now: now, started: now(), logger: logger, retries: make(map[[sha256.Size]byte][sha256.Size]byte), retryGroups: make(map[[sha256.Size]byte]int)}
}

func ErrorClass(err error) string {
	var integrity *delivery.IntegrityFailure
	switch {
	case errors.As(err, &integrity):
		return string(integrity.Class)
	case errors.Is(err, delivery.ErrAuthorization):
		return "authorization"
	case errors.Is(err, delivery.ErrSchema):
		return "schema"
	case errors.Is(err, delivery.ErrIntegrity):
		return "integrity"
	case errors.Is(err, delivery.ErrLease):
		return "lease"
	case errors.Is(err, delivery.ErrGap):
		return "gap"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	default:
		return "transient"
	}
}

func (s *State) RecordDelivery(ctx context.Context, err error, class string, wire []byte) {
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		return
	}
	s.Record(err, class, RetryKey(wire))
}

// Record observes a completed delivery or wallet pass. Keys are hashed before
// retention; neither snapshots nor metrics contain pool or wallet addresses.
func (s *State) Record(err error, class string, keys ...string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.failed = err != nil
	if err == nil {
		s.snapshot.LastSuccess = now
		s.snapshot.ConsecutiveFailures = 0
	} else {
		s.snapshot.LastError = now
		s.snapshot.ErrorClass = class
		if s.snapshot.ConsecutiveFailures < ^uint64(0) {
			s.snapshot.ConsecutiveFailures++
		}
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		digest := sha256.Sum256([]byte(key))
		if err == nil {
			if group, exists := s.retries[digest]; exists {
				delete(s.retries, digest)
				s.retryGroups[group]--
				if s.retryGroups[group] == 0 {
					delete(s.retryGroups, group)
				}
			}
			continue
		}
		if _, exists := s.retries[digest]; exists {
			continue
		}
		if len(s.retries) == MaxRetryPartitions {
			s.snapshot.RetryOverflow = true
			continue
		}
		groupKey := key
		if parts := strings.SplitN(key, ":", 3); len(parts) >= 2 {
			groupKey = parts[1]
		}
		group := sha256.Sum256([]byte(groupKey))
		s.retries[digest] = group
		s.retryGroups[group]++
	}
	s.snapshot.RetryPartitions = len(s.retryGroups)
	s.evaluate(now)
}

func (s *State) age(now time.Time) time.Duration {
	last := s.snapshot.LastSuccess
	if last.IsZero() {
		last = s.started
	}
	return max(time.Duration(0), now.Sub(last))
}

func (s *State) evaluate(now time.Time) bool {
	unavailable := s.failed && s.age(now) > s.bound
	if unavailable != s.unavailable {
		s.unavailable = unavailable
		s.logger.Info("accounting consumer readiness changed", "consumer", s.name, "ready", !unavailable, "error_class", s.snapshot.ErrorClass, "consecutive_failures", s.snapshot.ConsecutiveFailures, "last_success_age_seconds", s.age(now).Seconds())
	}
	return unavailable
}

func (s *State) Check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.evaluate(s.now()) {
		return ErrNoProgress
	}
	return nil
}

func (s *State) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot
}

// Metrics appends consumer gauges to the existing service telemetry response.
func Metrics(next http.Handler, service, environment, zone string, states ...*State) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if r.Method != http.MethodGet {
			return
		}
		for _, state := range states {
			if state == nil {
				continue
			}
			snapshot := state.Snapshot()
			timestamp := func(t time.Time) float64 {
				if t.IsZero() {
					return 0
				}
				return float64(t.UnixNano()) / float64(time.Second)
			}
			overflow := float64(0)
			if snapshot.RetryOverflow {
				overflow = 1
			}
			values := []struct {
				name  string
				value float64
			}{
				{"consumer_last_success_timestamp_seconds", timestamp(snapshot.LastSuccess)},
				{"consumer_last_error_timestamp_seconds", timestamp(snapshot.LastError)},
				{"consumer_consecutive_failures", float64(snapshot.ConsecutiveFailures)},
				{"consumer_retry_partitions", float64(snapshot.RetryPartitions)},
				{"consumer_retry_tracking_overflow", overflow},
			}
			for _, value := range values {
				if _, err := fmt.Fprintf(w, "%s{service=%q,environment=%q,zone=%q,consumer=%q} %g\n", value.name, service, environment, zone, state.name, value.value); err != nil {
					return
				}
			}
			if _, err := fmt.Fprintf(w, "consumer_last_error_info{service=%q,environment=%q,zone=%q,consumer=%q,error_class=%q} 1\n", service, environment, zone, state.name, snapshot.ErrorClass); err != nil {
				return
			}
		}
	})
}
