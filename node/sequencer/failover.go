package sequencer

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
)

var ErrDivergentLogs = errors.New("sequencer: two logs hold different receipts for one seq")

type FailoverResult struct {
	Lease      *wal.Lease
	LastSeq    uint64
	OpenHeight int64
}

func Failover(s *Standby, key *ReceiptKey, openHeight int64, expiryMs uint64) (FailoverResult, error) {
	if s == nil || key == nil {
		return FailoverResult{}, errors.New("sequencer: failover requires a standby and receipt key")
	}
	pub := key.PublicKey()
	if !bytes.Equal(pub[:], s.receiptKey.Public().(ed25519.PublicKey)) {
		return FailoverResult{}, errors.New("sequencer: failover receipt key differs from standby key")
	}
	keyID, err := key.KeyID(openHeight)
	if err != nil {
		return FailoverResult{}, err
	}
	s.mu.Lock()
	epoch := s.cur.leaseEpoch
	s.mu.Unlock()
	if epoch == ^uint64(0) {
		return FailoverResult{}, fmt.Errorf("%w: lease epoch exhausted", ErrStaleLease)
	}
	lease := &wal.Lease{Epoch: epoch + 1, KeyID: keyID, ExpiryMs: expiryMs}
	if err := s.TakeLease(lease); err != nil {
		return FailoverResult{}, err
	}
	return FailoverResult{Lease: lease, LastSeq: s.LastSeq(), OpenHeight: openHeight}, nil
}

// RecoveryOrder returns 0 to replay a first or 1 to replay b first. Ties prefer a.
func RecoveryOrder(a, b wal.ReplayResult) (int, error) {
	if a.LastLogged != nil && b.LastLogged != nil && a.LastLogged.Seq == b.LastLogged.Seq &&
		!bytes.Equal(a.LastLogged.Receipt, b.LastLogged.Receipt) {
		return 0, ErrDivergentLogs
	}
	var aEpoch, bEpoch uint64
	if a.Lease != nil {
		aEpoch = a.Lease.Epoch
	}
	if b.Lease != nil {
		bEpoch = b.Lease.Epoch
	}
	if aEpoch != bEpoch {
		if bEpoch > aEpoch {
			return 1, nil
		}
		return 0, nil
	}
	var aSeq, bSeq uint64
	if a.LastLogged != nil {
		aSeq = a.LastLogged.Seq
	}
	if b.LastLogged != nil {
		bSeq = b.LastLogged.Seq
	}
	if bSeq > aSeq {
		return 1, nil
	}
	return 0, nil
}
