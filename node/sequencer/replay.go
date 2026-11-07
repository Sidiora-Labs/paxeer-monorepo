package sequencer

import (
	"bytes"
	"fmt"
	"math"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/ethereum/go-ethereum/common"
)

type AppReplayer struct {
	exec  Executor
	r     *Receipter
	src   *ReceiptSource
	batch *Batch
}

var _ wal.Executor = (*AppReplayer)(nil)

func NewAppReplayer(exec Executor, r *Receipter) *AppReplayer {
	return &AppReplayer{exec: exec, r: r}
}

func (r *AppReplayer) LastBlockHeight() int64 {
	return r.r.host.LastBlockHeight()
}

func (r *AppReplayer) Apply(e wal.Entry) error {
	switch v := e.(type) {
	case *wal.Open:
		if v.Height > math.MaxInt64 || v.OpenTimeMs > math.MaxInt64 {
			return fmt.Errorf("%w: OPEN height or time exceeds int64", wal.ErrMalformed)
		}
		if len(v.ProposerAddress) != 0 {
			exec := r.exec
			if floor, ok := exec.(*FeeFloor); ok {
				exec = floor.Executor
			}
			seq := baseSequential(exec)
			seq.mu.Lock()
			seq.proposer = append([]byte(nil), v.ProposerAddress...)
			seq.mu.Unlock()
		}
		r.batch = nil
		return r.exec.Open(int64(v.Height), int64(v.OpenTimeMs))
	case *wal.Committed:
		return nil
	case *wal.Seal:
		if v.Height > math.MaxInt64 {
			return fmt.Errorf("%w: SEAL height exceeds int64", wal.ErrMalformed)
		}
		batch, err := r.exec.Seal(int64(v.Height))
		if err == nil {
			fields := BatchFields(batch)
			if fields.Height != v.Height || fields.FromSeq != v.FromSeq || fields.ToSeq != v.ToSeq || fields.LastSeqHash != v.LastSeqHash {
				return ErrBatchFieldsMismatch
			}
			r.batch = batch
		}
		if err == nil && r.src != nil {
			r.src.Committed(int64(v.Height))
		}
		return err
	default:
		return nil
	}
}

func (r *AppReplayer) Execute(a *wal.Accept) ([]byte, error) {
	acc, receipt, err := r.r.Accept(a.Tx)
	if err != nil {
		return nil, err
	}
	if acc.Seq != a.Seq || uint64(acc.Height) != a.Height || !bytes.Equal(receipt, a.Receipt) {
		return nil, fmt.Errorf("%w: seq %d", wal.ErrReceiptMismatch, a.Seq)
	}
	if r.src != nil {
		send, _, err := r.r.send(a.Tx, acc, receipt)
		if err != nil {
			return nil, fmt.Errorf("sequencer: replay send at sequence %d: %w", a.Seq, err)
		}
		r.src.record(common.Hash(acc.ActivityID), send, true)
	}
	return receipt, nil
}
