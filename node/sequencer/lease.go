package sequencer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
)
type LeaseHolderConfig struct {
	Replicator    *Replicator
	Replay        wal.ReplayResult
	KeyID         [32]byte
	LeaseDuration time.Duration
	Logger        *slog.Logger
}
type LeaseHolder struct {
	logger      *slog.Logger
	mu          sync.Mutex
	renewMu     sync.Mutex
	replicator  *Replicator
	replayEpoch uint64
	keyID       [32]byte
	duration    time.Duration
	epoch       uint64
	expiry      time.Time
}
func NewLeaseHolder(cfg LeaseHolderConfig) (*LeaseHolder, error) {
	if cfg.Replicator == nil {
		return nil, errors.New("sequencer: lease holder without a replicator")
	}
	if cfg.LeaseDuration <= 0 {
		return nil, fmt.Errorf("sequencer: lease duration %s must be positive", cfg.LeaseDuration)
	}
	l := &LeaseHolder{
		replicator: cfg.Replicator,
		logger:     cfg.Logger,
		keyID:      cfg.KeyID,
		duration:   cfg.LeaseDuration,
	}
	if cfg.Replay.Lease != nil {
		l.replayEpoch = cfg.Replay.Lease.Epoch
	}
	return l, nil
}
func (l *LeaseHolder) Start(ctx context.Context) error {
	l.renewMu.Lock()
	defer l.renewMu.Unlock()
	l.mu.Lock()
	if l.epoch != 0 {
		l.mu.Unlock()
		return errors.New("sequencer: lease holder already started")
	}
	if l.replayEpoch == math.MaxUint64 {
		l.mu.Unlock()
		return errors.New("sequencer: lease epoch exhausted")
	}
	lease := &wal.Lease{Epoch: l.replayEpoch + 1, KeyID: l.keyID, ExpiryMs: uint64(time.Now().Add(l.duration).UnixMilli())}
	l.mu.Unlock()
	if _, err := l.replicator.Release(ctx, lease); err != nil {
		logLeaseFence(l.logger, lease.Epoch, time.UnixMilli(int64(lease.ExpiryMs)), time.Now(), err)
		return err
	}
	expiry := time.UnixMilli(int64(lease.ExpiryMs))
	now := time.Now()
	if !now.Before(expiry) {
		if !l.replicator.Standalone() {
			logLeaseFence(l.logger, lease.Epoch, expiry, now, ErrLeaseFenced)
			return ErrLeaseFenced
		}
		if err := l.writeRenewal(ctx, now, lease.Epoch, expiry, lease.KeyID, l.duration); err != nil {
			logLeaseFence(l.logger, lease.Epoch, expiry, time.Now(), err)
			return err
		}
		expiry = l.Expiry()
	}
	l.mu.Lock()
	l.epoch, l.expiry = lease.Epoch, expiry
	l.mu.Unlock()
	return nil
}
func (l *LeaseHolder) Epoch() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.epoch
}

func (l *LeaseHolder) Expiry() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.expiry
}
