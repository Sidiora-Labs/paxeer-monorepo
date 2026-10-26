package sequencer

import (
	"context"
	"errors"
	"fmt"
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
}

type LeaseHolder struct {
	mu          sync.Mutex
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
		keyID:      cfg.KeyID,
		duration:   cfg.LeaseDuration,
	}
	if cfg.Replay.Lease != nil {
		l.replayEpoch = cfg.Replay.Lease.Epoch
	}
	return l, nil
}

func (l *LeaseHolder) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.epoch != 0 {
		return errors.New("sequencer: lease holder already started")
	}
	if l.replayEpoch == math.MaxUint64 {
		return errors.New("sequencer: lease epoch exhausted")
	}
	lease := &wal.Lease{
		Epoch:    l.replayEpoch + 1,
		KeyID:    l.keyID,
		ExpiryMs: uint64(time.Now().Add(l.duration).UnixMilli()),
	}
	if _, err := l.replicator.Release(ctx, lease); err != nil {
		return err
	}
	l.epoch = lease.Epoch
	l.expiry = time.UnixMilli(int64(lease.ExpiryMs))
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
