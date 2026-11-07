package sequencer_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func newFenceHarness(t *testing.T, funded ...utils.TestAcct) (*harness, int64) {
	t.Helper()
	h := newHarness(t, funded...)
	configure := admissionParams(1_000_000, 1_000_000, 200_000)
	height := commitBlock(h, func(ctx sdk.Context) {
		configure(ctx, h.app)
	})
	return h, height
}

func requireFencePendingTx(t *testing.T, exec sequencer.Executor, height int64, hash [32]byte, txBytes []byte, accepted *sequencer.Accepted) {
	t.Helper()
	got, ok := exec.PendingTx(hash)
	require.True(t, ok)
	require.Equal(t, txBytes, got.TxBytes)
	require.Equal(t, height, got.Height)
	require.Equal(t, accepted.TxIndex, got.TxIndex)
	require.Same(t, accepted.Result, got.Result)
}

func TestFencedExecutorRefusesAcceptWithoutWrite(t *testing.T) {
	for _, withFloor := range []bool{false, true} {
		name := "sequential"
		if withFloor {
			name = "fee_floor"
		}
		t.Run(name, func(t *testing.T) {
			sender := utils.NewSigner()
			recipient := utils.NewSigner()
			h, height := newFenceHarness(t, sender)
			committedBefore := h.committedCtx()
			exec := h.exec
			if withFloor {
				exec = newFloor(t, h, "0.01uhpx")
			}
			require.NoError(t, exec.Open(height, h.genesis.Add(time.Second).UnixMilli()))
			firstTx, firstBytes := transfer(t, sender, 0, recipient.EvmAddress, big.NewInt(1))
			first, err := exec.Accept(firstBytes)
			require.NoError(t, err)
			secondTx, secondBytes := transfer(t, sender, 1, recipient.EvmAddress, big.NewInt(2))
			second, err := exec.Accept(secondBytes)
			require.NoError(t, err)
			require.Equal(t, startSeq+2, second.Seq)
			before := h.openCtx()
			thirdTx, thirdBytes := transfer(t, sender, 2, recipient.EvmAddress, big.NewInt(3))
			send := banktypes.NewMsgSend(sender.AccountAddress, recipient.AccountAddress,
				sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(1))))
			lowFee := signCosmosFee(t, before, h, sender, floorGas, 1, send)

			require.NoError(t, exec.Fence())
			require.NoError(t, exec.Fence())
			for _, tx := range [][]byte{thirdBytes, lowFee, nil, {0xff}} {
				acc, err := exec.Accept(tx)
				require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
				require.Nil(t, acc)
			}
			acc, err := h.exec.Accept(thirdBytes)
			require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
			require.Nil(t, acc)
			after := h.openCtx()
			requireSameState(t, h.app, before, after)
			require.Equal(t, uint64(2), h.app.EvmKeeper.GetNonce(after, sender.EvmAddress))
			requireWei(t, h.weiBalance(before, sender.EvmAddress), h.weiBalance(after, sender.EvmAddress))
			requireWei(t, big.NewInt(3), h.weiBalance(after, recipient.EvmAddress))
			requireFencePendingTx(t, exec, height, firstTx.Hash(), firstBytes, first)
			requireFencePendingTx(t, exec, height, secondTx.Hash(), secondBytes, second)
			_, ok := exec.PendingTx(thirdTx.Hash())
			require.False(t, ok)
			requireSameState(t, h.app, committedBefore, h.committedCtx())
			require.Equal(t, uint64(0), h.app.EvmKeeper.GetNonce(h.committedCtx(), sender.EvmAddress))
		})
	}
}

func TestFencedExecutorRefusesOpenAndSeal(t *testing.T) {
	for _, opened := range []bool{false, true} {
		name := "unopened"
		if opened {
			name = "open"
		}
		t.Run(name, func(t *testing.T) {
			sender := utils.NewSigner()
			recipient := utils.NewSigner().EvmAddress
			h, nextHeight := newFenceHarness(t, sender)
			var before sdk.Context
			var accepted *sequencer.Accepted
			signed, tx := transfer(t, sender, 0, recipient, big.NewInt(1))
			if opened {
				require.NoError(t, h.exec.Open(nextHeight, h.genesis.Add(time.Second).UnixMilli()))
				var err error
				accepted, err = h.exec.Accept(tx)
				require.NoError(t, err)
				before = h.openCtx()
			}
			require.NoError(t, h.exec.Fence())
			for _, height := range []int64{nextHeight, -1} {
				batch, err := h.exec.Seal(height)
				require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
				require.Nil(t, batch)
				require.ErrorIs(t, h.exec.Open(height, 0), sequencer.ErrLeaseFenced)
			}
			require.ErrorIs(t, h.exec.Open(nextHeight, h.genesis.Add(time.Second).UnixMilli()), sequencer.ErrLeaseFenced)
			acc, err := h.exec.Accept(tx)
			require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
			require.Nil(t, acc)
			if opened {
				requireSameState(t, h.app, before, h.openCtx())
				requireFencePendingTx(t, h.exec, nextHeight, signed.Hash(), tx, accepted)
			} else {
				_, err := h.exec.Snapshot()
				require.ErrorIs(t, err, sequencer.ErrLayerNotOpen)
			}
			require.Equal(t, nextHeight-1, h.app.LastBlockHeight())
		})
	}
}

func TestFencedMicroBatchForwardsLeaseFenced(t *testing.T) {
	sender := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	h, height := newFenceHarness(t, sender)
	mb, err := sequencer.NewMicroBatch(h.app, microbatchOptions(h), sequencer.MicroBatchOptions{
		Window: time.Hour, MaxCount: 2,
	})
	require.NoError(t, err)
	h.exec = mb
	require.NoError(t, mb.Open(height, h.genesis.Add(time.Second).UnixMilli()))
	before := h.openCtx()
	signed, tx := transfer(t, sender, 0, recipient, big.NewInt(1))
	type outcome struct {
		acc *sequencer.Accepted
		err error
	}
	entered := make(chan struct{})
	done := make(chan outcome, 1)
	go func() {
		close(entered)
		acc, err := mb.Accept(tx)
		done <- outcome{acc, err}
	}()
	<-entered
	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()
	select {
	case result := <-done:
		t.Fatalf("forming batch returned before fencing: %+v", result)
	case <-timer.C:
	}
	require.NoError(t, mb.Fence())
	require.NoError(t, mb.Fence())
	select {
	case result := <-done:
		require.ErrorIs(t, result.err, sequencer.ErrLeaseFenced)
		require.Nil(t, result.acc)
	case <-time.After(5 * time.Second):
		t.Fatal("fencing did not release the pending micro-batch")
	}
	for _, txBytes := range [][]byte{tx, nil, {0xff}} {
		acc, err := mb.Accept(txBytes)
		require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
		require.Nil(t, acc)
	}
	batch, err := mb.Seal(height)
	require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
	require.Nil(t, batch)
	require.ErrorIs(t, mb.Open(height, h.genesis.Add(time.Second).UnixMilli()), sequencer.ErrLeaseFenced)
	after := h.openCtx()
	requireSameState(t, h.app, before, after)
	require.Equal(t, uint64(0), h.app.EvmKeeper.GetNonce(after, sender.EvmAddress))
	requireWei(t, h.weiBalance(before, sender.EvmAddress), h.weiBalance(after, sender.EvmAddress))
	requireWei(t, big.NewInt(0), h.weiBalance(after, recipient))
	_, ok := mb.PendingTx(signed.Hash())
	require.False(t, ok)
}
