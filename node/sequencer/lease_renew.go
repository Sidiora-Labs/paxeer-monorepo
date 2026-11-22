package sequencer

import (
	"context"
	"errors"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
)

// RenewAt renews within the final half of the held lease.
// A standalone primary can also renew at or after expiry.
// Success includes the replicator's durable standby acknowledgement.
func (l *LeaseHolder) RenewAt(now time.Time) error {
	return l.renew(context.Background(), now)
}

func (l *LeaseHolder) renew(ctx context.Context, now time.Time) error {
	l.renewMu.Lock()
	defer l.renewMu.Unlock()
	l.mu.Lock()
	epoch, expiry, keyID, duration := l.epoch, l.expiry, l.keyID, l.duration
	l.mu.Unlock()
	if epoch == 0 {
		return errors.New("sequencer: lease holder not started")
	}
	if !l.replicator.Standalone() && !now.Before(expiry) {
		return ErrLeaseFenced
	}
	if expiry.Sub(now) > duration/2 {
		return nil
	}
	return l.writeRenewal(ctx, now, epoch, expiry, keyID, duration)
}
func (l *LeaseHolder) writeRenewal(ctx context.Context, now time.Time, epoch uint64, expiry time.Time, keyID [32]byte, duration time.Duration) error {
	lease := &wal.Lease{Epoch: epoch, KeyID: keyID, ExpiryMs: uint64(now.Add(duration).UnixMilli())}
	if _, err := l.replicator.Release(ctx, lease); err != nil {
		return err
	}
	if !l.replicator.Standalone() && !time.Now().Before(expiry) {
		return ErrLeaseFenced
	}
	l.mu.Lock()
	l.expiry = time.UnixMilli(int64(lease.ExpiryMs))
	l.mu.Unlock()
	renewedAt := time.Now()
	if now.After(renewedAt) {
		renewedAt = now
	}
	if !renewedAt.Before(expiry) && l.logger != nil {
		l.logger.Warn("sequencer lease renewed after expiry", "epoch", epoch, "lateness", renewedAt.Sub(expiry))
	}
	return nil
}
