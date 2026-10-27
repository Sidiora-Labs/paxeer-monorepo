package sequencer

import (
	"context"
	"errors"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
)

func (l *LeaseHolder) RenewAt(now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.epoch == 0 {
		return errors.New("sequencer: lease holder not started")
	}
	if l.expiry.Sub(now) > l.duration/2 {
		return nil
	}
	lease := &wal.Lease{
		Epoch:    l.epoch,
		KeyID:    l.keyID,
		ExpiryMs: uint64(now.Add(l.duration).UnixMilli()),
	}
	if _, err := l.replicator.Release(context.Background(), lease); err != nil {
		return err
	}
	l.expiry = time.UnixMilli(int64(lease.ExpiryMs))
	return nil
}
