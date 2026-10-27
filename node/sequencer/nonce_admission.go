package sequencer

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type NonceAdmission struct {
	queue *NonceQueue
}

func NewNonceAdmission(cfg Config, nonces NextNonceReader, accept AcceptFunc, lifetime time.Duration) (*NonceAdmission, error) {
	queue, err := NewNonceQueue(NonceQueueConfig{
		PerSender: cfg.NonceQueueMaxPerSender,
		Total:     cfg.NonceQueueMaxTotal,
		Lifetime:  lifetime,
	}, nonces, accept)
	if err != nil {
		return nil, err
	}
	return &NonceAdmission{queue: queue}, nil
}

func (a *NonceAdmission) Send(sender common.Address, nonce uint64, tx []byte) (bool, error) {
	held, err := a.queue.Hold(sender, nonce, append([]byte(nil), tx...))
	if err == nil && !held {
		a.Receipted(sender)
	}
	return held, err
}

func (a *NonceAdmission) Receipted(sender common.Address) {
	a.queue.Drain(sender)
}

func (a *NonceAdmission) Open(now time.Time) []HeldTx {
	return a.queue.Expire(now)
}

func (a *NonceAdmission) Sealed() {
	a.queue.Sealed()
}
