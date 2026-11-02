package evmrpc_test

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	occutils "github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type pendingOpenSnapshot struct {
	evmrpc.PendingSnapshot
	snapshot sdk.Context
	open     atomic.Bool
	calls    atomic.Int32
	started  chan struct{}
}

func (p *pendingOpenSnapshot) Snapshot() (sdk.Context, error) {
	if p.calls.Add(1) == 1 {
		close(p.started)
	}
	if !p.open.Load() {
		return sdk.Context{}, fmt.Errorf("between blocks: %w", evmrpc.ErrLayerNotOpen)
	}
	return p.snapshot, nil
}

func (p *pendingOpenSnapshot) openAfter(t *testing.T, delay time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-p.started
		time.Sleep(delay)
		p.open.Store(true)
	}()
	t.Cleanup(func() { <-done })
}

func TestPendingContextWaitsForTheOpenBlock(t *testing.T) {
	s := newPendingSequencer(t, 1)
	want, err := s.exec.Snapshot()
	require.NoError(t, err)
	store, ok := want.MultiStore().(cachemulti.Store)
	require.True(t, ok)
	want = want.WithMultiStore(&store)
	pending := &pendingOpenSnapshot{PendingSnapshot: s.exec, snapshot: want, started: make(chan struct{})}
	s.wm.SetPendingSnapshot(pending)
	pending.openAfter(t, 20*time.Millisecond)
	start := time.Now()
	got, err := s.wm.PendingContext(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, time.Since(start), 20*time.Millisecond)
	require.Greater(t, pending.calls.Load(), int32(1))
	require.Equal(t, want.BlockHeader(), got.BlockHeader())
	require.Same(t, want.MultiStore(), got.MultiStore())
}

func TestPendingContextRefusesWhenNoBlockOpens(t *testing.T) {
	s := newPendingSequencer(t, 1)
	pending := &pendingOpenSnapshot{PendingSnapshot: s.exec, started: make(chan struct{})}
	s.wm.SetPendingSnapshot(pending)
	s.wm.SetPendingOpenWait(50 * time.Millisecond)
	start := time.Now()
	_, err := s.wm.PendingContext(context.Background())
	elapsed := time.Since(start)
	require.ErrorIs(t, err, evmrpc.ErrPendingBlockClosed)
	require.EqualError(t, err, "no block is open; retry")
	var rpcErr *evmrpc.ErrEVMNotSupported
	require.ErrorAs(t, err, &rpcErr)
	require.Equal(t, -32000, rpcErr.ErrorCode())
	require.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
	require.Less(t, elapsed, evmrpc.PendingOpenWait)
	require.Greater(t, pending.calls.Load(), int32(1))
	require.False(t, pending.open.Load())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = s.wm.PendingContext(ctx)
	require.ErrorIs(t, err, context.Canceled)
	var absent *evmrpc.WatermarkManager
	_, err = absent.PendingContext(context.Background())
	require.ErrorIs(t, err, evmrpc.ErrPendingServedBySequencer)
}

func TestPendingNonceWaitsForTheOpenBlock(t *testing.T) {
	defaults := app.DefaultConsensusParams
	params := *defaults
	block := *defaults.Block
	block.MaxGasWanted = pendingTransferGas
	params.Block = &block
	app.DefaultConsensusParams = &params
	t.Cleanup(func() { app.DefaultConsensusParams = defaults })
	sender := occutils.NewSigner()
	s := newPendingSequencer(t, 1, sender)
	accepted, err := s.exec.Accept(pendingTransfer(t, sender, 0, occutils.NewSigner().EvmAddress, big.NewInt(pendingOnePaxWei)))
	require.NoError(t, err)
	require.Zero(t, accepted.Result.Code)
	snapshot, err := s.exec.Snapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(1), s.app.EvmKeeper.GetNonce(snapshot, sender.EvmAddress))
	pending := &pendingOpenSnapshot{PendingSnapshot: s.exec, snapshot: snapshot, started: make(chan struct{})}
	s.wm.SetPendingSnapshot(pending)
	api := evmrpc.NewTransactionAPI(nil, &s.app.EvmKeeper, nil, nil, "",
		evmrpc.ConnectionTypeHTTP, utils.None[time.Duration](), s.wm, nil, &sync.Mutex{})
	pending.openAfter(t, 20*time.Millisecond)
	start := time.Now()
	nonce, err := api.GetTransactionCount(context.Background(), sender.EvmAddress, rpc.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber))
	require.NoError(t, err)
	require.GreaterOrEqual(t, time.Since(start), 20*time.Millisecond)
	require.Greater(t, pending.calls.Load(), int32(1))
	require.NotNil(t, nonce)
	require.Equal(t, hexutil.Uint64(1), *nonce)

	pending.open.Store(false)
	s.wm.SetPendingOpenWait(50 * time.Millisecond)
	nonce, err = api.GetTransactionCount(context.Background(), sender.EvmAddress, rpc.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber))
	require.Nil(t, nonce)
	require.ErrorIs(t, err, evmrpc.ErrPendingBlockClosed)
}

func TestPendingReadsRespectCanceledContext(t *testing.T) {
	sender := occutils.NewSigner()
	s := newPendingSequencer(t, 1, sender)
	n := pendingSimulateAPIs(s, s.wm)
	api := pendingTxSequencerAPI(s, &pendingNonceClient{MockClient: &MockClient{latestOverride: s.committed}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, open := range []bool{true, false} {
		if !open {
			_, err := s.exec.Seal(s.committed + 1)
			require.NoError(t, err)
		}
		_, err := s.wm.ResolveHeight(ctx, pendingSimulateTag)
		require.ErrorIs(t, err, context.Canceled)
		_, err = n.state.GetBalance(ctx, sender.EvmAddress, pendingSimulateTag)
		require.ErrorIs(t, err, context.Canceled)
		_, err = n.state.GetCode(ctx, common.Address{}, pendingSimulateTag)
		require.ErrorIs(t, err, context.Canceled)
		_, err = n.state.GetStorageAt(ctx, common.Address{}, "0x0", pendingSimulateTag)
		require.ErrorIs(t, err, context.Canceled)
		_, _, err = n.backend.StateAndHeaderByNumberOrHash(ctx, pendingSimulateTag)
		require.ErrorIs(t, err, context.Canceled)
		_, err = api.GetTransactionCount(ctx, sender.EvmAddress, pendingSimulateTag)
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestPendingReadsWaitForReopenedExecutor(t *testing.T) {
	sender := occutils.NewSigner()
	s := newPendingSequencer(t, 1, sender)
	n := pendingSimulateAPIs(s, s.wm)
	api := pendingTxSequencerAPI(s, &pendingNonceClient{MockClient: &MockClient{latestOverride: s.committed}})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, read := range []func(){
		func() {
			height, err := s.wm.ResolveHeight(ctx, pendingSimulateTag)
			require.NoError(t, err)
			require.Equal(t, evmrpc.PendingCtxHeight, height)
		},
		func() {
			_, header, err := n.backend.StateAndHeaderByNumberOrHash(ctx, pendingSimulateTag)
			require.NoError(t, err)
			require.Equal(t, s.committed+1, header.Number.Int64())
		},
		func() {
			_, err := n.state.GetBalance(ctx, sender.EvmAddress, pendingSimulateTag)
			require.NoError(t, err)
		},
		func() {
			nonce, err := api.GetTransactionCount(ctx, sender.EvmAddress, pendingSimulateTag)
			require.NoError(t, err)
			require.Equal(t, hexutil.Uint64(0), *nonce)
		},
	} {
		_, err := s.exec.Seal(s.committed + 1)
		require.NoError(t, err)
		opened := make(chan error, 1)
		go func() {
			time.Sleep(20 * time.Millisecond)
			opened <- s.exec.Open(s.committed+1, time.Now().UTC().UnixMilli())
		}()
		start := time.Now()
		read()
		require.NoError(t, <-opened)
		require.GreaterOrEqual(t, time.Since(start), 20*time.Millisecond)
	}
}
