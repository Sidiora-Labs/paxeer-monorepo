package evmrpc_test

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/legacyabci"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	occutils "github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type sendPendingAccepter struct {
	next    evmrpc.SequencerAccepter
	pending *pendingOpenSnapshot
	calls   int
	firstTx []byte
}

func (s *sendPendingAccepter) Accept(txBytes []byte) ([]byte, error) {
	s.calls++
	if s.calls == 1 {
		s.firstTx = bytes.Clone(txBytes)
		return nil, fmt.Errorf("sealed layer: %w", evmrpc.ErrLayerNotOpen)
	}
	if !bytes.Equal(s.firstTx, txBytes) {
		return nil, fmt.Errorf("retry changed the transaction")
	}
	if !s.pending.open.Load() {
		return nil, fmt.Errorf("retry before the next open: %w", evmrpc.ErrLayerNotOpen)
	}
	return s.next.Accept(txBytes)
}

func newSendPendingAPI(t *testing.T, h *sendSeqHarness) (*evmrpc.SendAPI, *sendPendingAccepter, *pendingOpenSnapshot, *evmrpc.WatermarkManager) {
	t.Helper()
	snapshot, err := h.exec.Snapshot()
	require.NoError(t, err)
	pending := &pendingOpenSnapshot{PendingSnapshot: h.exec, snapshot: snapshot, started: make(chan struct{})}
	watermarks := evmrpc.NewWatermarkManager(nil, nil, nil, nil)
	watermarks.SetPendingSnapshot(pending)
	accepter := &sendPendingAccepter{next: h.accepter, pending: pending}
	api := evmrpc.NewSendAPI(
		h.client,
		func(int64) client.TxConfig { return h.app.GetTxConfig() },
		&evmrpc.SendConfig{Sequencer: accepter},
		&h.app.EvmKeeper,
		legacyabci.BeginBlockKeepers{},
		func(int64) sdk.Context { return h.app.GetContextForDeliverTx([]byte{}) },
		"",
		nil,
		nil,
		nil,
		evmrpc.ConnectionTypeHTTP,
		utils.None[time.Duration](),
		evmrpc.NewBlockCache(1),
		nil,
		watermarks,
	)
	return api, accepter, pending, watermarks
}

func TestSendRawTransactionRetriesAcrossASealedLayer(t *testing.T) {
	sender := occutils.NewSigner()
	h := newSendSeqHarness(t, sender)
	api, accepter, pending, _ := newSendPendingAPI(t, h)
	tx, raw := sendSeqTransfer(t, sender, 0, common.HexToAddress("0x1234"), big.NewInt(1), sendSeqTransferGas)

	ctx, cancel := context.WithTimeout(context.Background(), evmrpc.PendingOpenWait)
	defer cancel()
	opened := make(chan struct{})
	go func() {
		defer close(opened)
		select {
		case <-ctx.Done():
			return
		case <-pending.started:
		}
		timer := time.NewTimer(20 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
			pending.open.Store(true)
		}
	}()
	t.Cleanup(func() { cancel(); <-opened })
	start := time.Now()
	hash, err := api.SendRawTransaction(ctx, raw)
	require.NoError(t, err)
	require.Equal(t, tx.Hash(), hash)
	require.Equal(t, 2, accepter.calls)
	require.GreaterOrEqual(t, time.Since(start), 20*time.Millisecond)
	require.True(t, pending.open.Load())
	require.Len(t, h.accepter.receipts, 1)
	require.Equal(t, [32]byte(tx.Hash()), h.verifiedReceipt(t, 0).ActivityID)
	require.Zero(t, h.client.broadcastCount())
}

func TestSendRawTransactionRefusesWhenNoBlockReopens(t *testing.T) {
	sender := occutils.NewSigner()
	h := newSendSeqHarness(t, sender)
	api, accepter, pending, watermarks := newSendPendingAPI(t, h)
	watermarks.SetPendingOpenWait(50 * time.Millisecond)
	_, raw := sendSeqTransfer(t, sender, 0, common.HexToAddress("0x1234"), big.NewInt(1), sendSeqTransferGas)

	start := time.Now()
	_, err := api.SendRawTransaction(context.Background(), raw)
	elapsed := time.Since(start)
	require.ErrorIs(t, err, evmrpc.ErrPendingBlockClosed)
	require.NotErrorIs(t, err, evmrpc.ErrLayerNotOpen)
	require.EqualError(t, err, "no block is open; retry")
	var rpcErr *evmrpc.ErrEVMNotSupported
	require.ErrorAs(t, err, &rpcErr)
	require.Equal(t, -32000, rpcErr.ErrorCode())
	require.Equal(t, 1, accepter.calls)
	require.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
	require.Less(t, elapsed, evmrpc.PendingOpenWait)
	require.False(t, pending.open.Load())
	require.Empty(t, h.accepter.receipts)
	require.Zero(t, h.client.broadcastCount())
}
