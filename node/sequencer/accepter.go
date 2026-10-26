package sequencer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
)

var ErrReplicationHalted = errors.New("sequencer: replication halted")

var _ evmrpc.SequencerAccepter = (*Accepter)(nil)

type Accepter struct {
	mu     sync.Mutex
	r      *Receipter
	rep    *Replicator
	src    *ReceiptSource
	m      *Metrics
	halted bool
}

func NewAccepter(r *Receipter, rep *Replicator, src *ReceiptSource, m *Metrics) *Accepter {
	if r == nil || rep == nil || src == nil || m == nil {
		panic("sequencer: accepter needs a receipter, replicator, receipt source and metrics")
	}
	return &Accepter{r: r, rep: rep, src: src, m: m}
}

func (a *Accepter) Accept(txBytes []byte) ([]byte, error) {
	start := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.halted {
		return nil, ErrReplicationHalted
	}
	acc, receipt, err := a.r.Accept(txBytes)
	if err != nil {
		// An accepted outcome with an error has already changed the open layer.
		if acc != nil {
			a.halted = true
		}
		return nil, err
	}
	send, err := a.send(txBytes, acc, receipt)
	if err != nil {
		a.halted = true
		return nil, fmt.Errorf("%w: %w", ErrReplicationHalted, err)
	}
	ackStart := time.Now()
	ack, err := a.rep.Release(context.Background(), &wal.Accept{
		Seq: acc.Seq, Height: uint64(acc.Height), Tx: txBytes, Receipt: receipt,
	})
	a.m.ObserveStandbyAck(time.Since(ackStart))
	if err == nil && ack.Status != AckOK {
		err = fmt.Errorf("%w: status %d", ErrStandbyRefused, ack.Status)
	}
	if err != nil {
		a.halted = true
		return nil, fmt.Errorf("%w: %w", ErrReplicationHalted, err)
	}
	a.src.Record(common.Hash(acc.ActivityID), send)
	a.m.ObserveReceipt(time.Since(start))
	return receipt, nil
}

func (a *Accepter) send(txBytes []byte, acc *Accepted, receipt []byte) (evmrpc.SequencerSend, error) {
	decoded, err := codec.DecodeReceipt(receipt)
	if err != nil {
		return evmrpc.SequencerSend{}, err
	}
	send := evmrpc.SequencerSend{Nonce: decoded.FromSequence, Receipt: receipt}
	tx, err := a.r.host.GetTxConfig().TxDecoder()(txBytes)
	if err != nil {
		return evmrpc.SequencerSend{}, err
	}
	if a.r.host.GetEVMMsg(tx) == nil {
		return send, nil
	}
	ctx, err := a.r.exec.Snapshot()
	if err != nil {
		return evmrpc.SequencerSend{}, err
	}
	rcpt, err := a.r.evm.GetTransientReceipt(ctx, common.Hash(acc.ActivityID), uint64(acc.TxIndex))
	if err != nil {
		return evmrpc.SequencerSend{}, err
	}
	send.ReturnData, err = evmReturnData(acc.Result.Data)
	if err != nil {
		return evmrpc.SequencerSend{}, err
	}
	send.VMError = rcpt.VmError
	send.LogData = make([][]byte, len(rcpt.Logs))
	for i, log := range rcpt.Logs {
		send.LogData[i] = log.Data
	}
	return send, nil
}
