package sequencer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
)

// ErrReplicationHalted means an executed send could not be released safely.
// The accepter refuses subsequent sends until the node is recovered.
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

// NewAccepter releases signed receipts and records their execution outputs only
// after the replicator has durably acknowledged the accepted transaction.
func NewAccepter(r *Receipter, rep *Replicator, src *ReceiptSource, m *Metrics) *Accepter {
	if r == nil || rep == nil || src == nil || m == nil {
		panic("sequencer: accepter needs a receipter, replicator, receipt source and metrics")
	}
	return &Accepter{r: r, rep: rep, src: src, m: m}
}

func (a *Accepter) Accept(txBytes []byte) (evmrpc.AcceptedSend, error) {
	start := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.halted {
		return evmrpc.AcceptedSend{}, ErrReplicationHalted
	}
	acc, receipt, err := a.r.Accept(txBytes)
	if err != nil {
		// An accepted outcome with an error has already changed the open layer.
		if acc != nil {
			a.halted = true
		}
		return evmrpc.AcceptedSend{}, err
	}
	send, exec, err := a.r.send(txBytes, acc, receipt)
	if err != nil {
		a.halted = true
		return evmrpc.AcceptedSend{}, fmt.Errorf("sequencer: accepted send at sequence %d: %w", acc.Seq, err)
	}
	ackStart := time.Now()
	ack, err := a.rep.Release(context.Background(), &wal.Accept{
		Seq: acc.Seq, Height: uint64(acc.Height), Tx: txBytes, Receipt: receipt,
		ReturnData: send.ReturnData, LogData: send.LogData, VMError: send.VMError,
	})
	a.m.ObserveStandbyAck(time.Since(ackStart))
	if err == nil && ack.Status != AckOK {
		err = fmt.Errorf("%w: status %d", ErrStandbyRefused, ack.Status)
	}
	if err != nil {
		a.halted = true
		return evmrpc.AcceptedSend{}, fmt.Errorf("%w: %w", ErrReplicationHalted, err)
	}
	a.src.Record(common.Hash(acc.ActivityID), send)
	a.m.ObserveReceipt(time.Since(start))
	return evmrpc.AcceptedSend{Receipt: receipt, Height: acc.Height, Exec: exec}, nil
}

func (r *Receipter) send(txBytes []byte, acc *Accepted, receipt []byte) (evmrpc.SequencerSend, *evmtypes.Receipt, error) {
	decoded, err := codec.DecodeReceipt(receipt)
	if err != nil {
		return evmrpc.SequencerSend{}, nil, err
	}
	send := evmrpc.SequencerSend{Nonce: decoded.FromSequence, Receipt: receipt}
	tx, err := r.host.GetTxConfig().TxDecoder()(txBytes)
	if err != nil {
		return evmrpc.SequencerSend{}, nil, err
	}
	if r.host.GetEVMMsg(tx) == nil {
		return send, nil, nil
	}
	rcpt, err := r.evm.GetTransientReceipt(acc.ReceiptContext, common.Hash(acc.ActivityID), uint64(acc.TxIndex))
	if err != nil {
		return evmrpc.SequencerSend{}, nil, err
	}
	if len(decoded.Effects) == 0 || decoded.Effects[0].ModuleID != decoded.ModuleID ||
		decoded.Effects[0].Ordinal != 0 || decoded.Effects[0].Kind != effectKindState || len(decoded.Effects[0].Body) < 16 {
		return evmrpc.SequencerSend{}, nil, inputErr("missing execution gas summary")
	}
	// Receipter.after adds cumulative gas to the signed summary, but the
	// transient store still holds its execution-time value. Return the value
	// bound by this receipt rather than re-reading the receipter's mutable total.
	rcpt.CumulativeGasUsed = binary.BigEndian.Uint64(decoded.Effects[0].Body[8:16])
	send.ReturnData, err = evmReturnData(acc.Result.Data)
	if err != nil {
		return evmrpc.SequencerSend{}, nil, err
	}
	send.VMError = rcpt.VmError
	send.LogData = make([][]byte, len(rcpt.Logs))
	for i, log := range rcpt.Logs {
		send.LogData[i] = log.Data
	}
	return send, rcpt, nil
}
