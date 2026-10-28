package sequencer

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
)

type Fencer interface {
	Fence() error
}

type HandoverConfig struct {
	Primary    Fencer
	Replicator *Replicator
	Standby    *Standby
	Key        *ReceiptKey
	OpenHeight int64
	ExpiryMs   uint64
}

type HandoverResult struct {
	Lease   *wal.Lease
	LastSeq uint64
}

func (r *Replicator) Drain(ctx context.Context) (Ack, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	lastSeq := r.lastSeq
	ack, err := r.replicate(ctx, r.window(), lastSeq)
	if err != nil {
		return Ack{}, err
	}
	if ack.Status != AckOK || ack.LastSeq != lastSeq {
		r.dropConn()
		return Ack{}, fmt.Errorf("%w: drain acknowledged status %d seq %d, want seq %d", ErrNoAck, ack.Status, ack.LastSeq, lastSeq)
	}
	return ack, nil
}

func Handover(ctx context.Context, cfg HandoverConfig) (HandoverResult, error) {
	if cfg.Key == nil {
		return HandoverResult{}, errors.New("sequencer: handover requires a receipt key")
	}
	keyID, err := cfg.Key.KeyID(cfg.OpenHeight)
	if err != nil {
		return HandoverResult{}, err
	}
	if cfg.Primary == nil || cfg.Replicator == nil || cfg.Standby == nil {
		return HandoverResult{}, errors.New("sequencer: handover requires a primary, replicator and standby")
	}
	pub := cfg.Key.PublicKey()
	if !bytes.Equal(pub[:], cfg.Standby.receiptKey.Public().(ed25519.PublicKey)) {
		return HandoverResult{}, errors.New("sequencer: handover receipt key differs from standby key")
	}
	if err := cfg.Primary.Fence(); err != nil {
		return HandoverResult{}, err
	}
	ack, err := cfg.Replicator.Drain(ctx)
	if err != nil {
		return HandoverResult{}, err
	}
	epoch := cfg.Replicator.Epoch()
	if epoch == ^uint64(0) {
		return HandoverResult{}, fmt.Errorf("%w: lease epoch exhausted", ErrStaleLease)
	}
	lease := &wal.Lease{Epoch: epoch + 1, KeyID: keyID, ExpiryMs: cfg.ExpiryMs}
	if err := cfg.Standby.TakeLease(lease); err != nil {
		return HandoverResult{}, err
	}
	return HandoverResult{Lease: lease, LastSeq: ack.LastSeq}, nil
}
