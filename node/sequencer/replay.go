package sequencer

import (
	"bytes"
	"fmt"
	"math"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
)

type AppReplayer struct {
	exec Executor
	r    *Receipter
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
		return r.exec.Open(int64(v.Height), int64(v.OpenTimeMs))
	case *wal.Committed:
		return nil
	case *wal.Seal:
		if v.Height > math.MaxInt64 {
			return fmt.Errorf("%w: SEAL height exceeds int64", wal.ErrMalformed)
		}
		_, err := r.exec.Seal(int64(v.Height))
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
	return receipt, nil
}
