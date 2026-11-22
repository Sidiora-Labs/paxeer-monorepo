package sequencer

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestNonceOpenSendAcrossSealAndCommit(t *testing.T) {
	sender := utils.NewSigner()
	h := newConsensusFenceHarness(t, 42, sender)
	n := runtimeAdmissionPrimary(t, h, ModeSequential)
	require.NoError(t, n.Start(t.Context()))
	_, err := n.Accept(consensusFenceTransfer(t, n.App, sender, 0))
	require.NoError(t, err)
	block, err := n.ConsensusSource.Seal(t.Context(), 2)
	require.NoError(t, err)

	tx, raw := runtimeAdmissionSigned(t, n, sender, 1)
	_, err = n.Accept(consensusFenceTransfer(t, n.App, sender, 1))
	require.ErrorIs(t, err, ErrLayerNotOpen)
	require.NotErrorIs(t, err, ErrNonceHeld)
	require.Zero(t, n.NonceAdmission.queue.Len())
	api := runtimeAdmissionAPI(t, n)
	type outcome struct {
		hash common.Hash
		err  error
	}
	done := make(chan outcome, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	go func() {
		hash, err := api.SendRawTransaction(ctx, raw)
		done <- outcome{hash, err}
	}()
	select {
	case result := <-done:
		t.Fatalf("send returned before a block opened: %v", result.err)
	case <-time.After(25 * time.Millisecond):
	}
	sequencerRecoveryFinalize(t, n.App, n.ConsensusSource, h.opts.ProposerAddress, 2, block)
	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.Equal(t, tx.Hash(), result.hash)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	nonceOpenReceipt(t, n, tx.Hash(), 1, 3, 44)
	pending, ok := n.Executor.PendingTx(tx.Hash())
	require.True(t, ok)
	require.Equal(t, int64(3), pending.Height)
	require.Equal(t, 1, pending.TxIndex)
	ctxOpen, err := n.Executor.Snapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(2), n.App.EvmKeeper.GetNonce(ctxOpen, sender.EvmAddress))
}

func nonceOpenReceipt(t *testing.T, n *Node, hash common.Hash, nonce uint64, height int64, seq uint64) {
	t.Helper()
	send, err := n.ReceiptSource.SequencerSend(hash)
	require.NoError(t, err)
	require.NotNil(t, send)
	require.Equal(t, nonce, send.Nonce)
	receipt, err := verify.ReceiptSignature(send.Receipt, n.ReceiptKey.PublicKey())
	require.NoError(t, err)
	require.Equal(t, [32]byte(hash), receipt.Receipt.ActivityID)
	require.Equal(t, nonce, receipt.Receipt.FromSequence)
	require.Equal(t, uint64(height), binary.BigEndian.Uint64(receipt.Receipt.BatchID[24:]))
	require.Equal(t, seq, receipt.Receipt.GlobalSequence)
}

func TestNonceOpenSealDuringAdmission(t *testing.T) {
	sender := utils.NewSigner()
	h := newConsensusFenceHarness(t, 42, sender)
	n := runtimeAdmissionPrimary(t, h, ModeSequential)
	require.NoError(t, n.Start(t.Context()))
	_, err := n.Accept(consensusFenceTransfer(t, n.App, sender, 0))
	require.NoError(t, err)
	tx, _ := runtimeAdmissionSigned(t, n, sender, 1)
	encoded := consensusFenceTransfer(t, n.App, sender, 1)
	q := n.NonceAdmission.queue
	unlock := q.lockSender(sender.EvmAddress)
	defer func() {
		if unlock != nil {
			unlock()
		}
	}()
	done := make(chan error, 1)
	go func() {
		_, err := n.Accept(encoded)
		done <- err
	}()
	require.Eventually(t, func() bool {
		q.mu.Lock()
		defer q.mu.Unlock()
		return q.gates[sender.EvmAddress].users == 2
	}, time.Second, time.Millisecond)
	block, err := n.ConsensusSource.Seal(t.Context(), 2)
	require.NoError(t, err)
	unlock()
	unlock = nil
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrNonceHeld)
	case <-time.After(time.Second):
		t.Fatal("admission did not finish")
	}
	require.True(t, q.Held(sender.EvmAddress, 1))
	sequencerRecoveryFinalize(t, n.App, n.ConsensusSource, h.opts.ProposerAddress, 2, block)
	require.Zero(t, q.Len())
	nonceOpenReceipt(t, n, tx.Hash(), 1, 3, 44)
	next, err := n.ConsensusSource.Seal(t.Context(), 3)
	require.NoError(t, err)
	require.Equal(t, [][]byte{encoded}, next.Txs.ToSliceOfBytes())
}

func TestNonceOpenReleasesAfterOutsideAcceptance(t *testing.T) {
	for _, mode := range []Mode{ModeSequential, ModeMicroBatch} {
		t.Run(mode.String(), func(t *testing.T) {
			sender := utils.NewSigner()
			h := newConsensusFenceHarness(t, 42, sender)
			n := runtimeAdmissionPrimary(t, h, mode)
			require.NoError(t, n.Start(t.Context()))
			tx, _ := runtimeAdmissionSigned(t, n, sender, 1)
			heldTx := consensusFenceTransfer(t, n.App, sender, 1)
			_, err := n.Accept(heldTx)
			require.ErrorIs(t, err, ErrNonceHeld)
			_, err = n.Accepter.Accept(consensusFenceTransfer(t, n.App, sender, 0))
			require.NoError(t, err)
			require.True(t, n.NonceAdmission.queue.Held(sender.EvmAddress, 1))
			runtimeAdmissionCommit(t, n, h)
			require.Zero(t, n.NonceAdmission.queue.Len())
			nonceOpenReceipt(t, n, tx.Hash(), 1, 3, 44)
			pending, ok := n.Executor.PendingTx(tx.Hash())
			require.True(t, ok)
			require.Equal(t, heldTx, pending.TxBytes)
			require.Equal(t, int64(3), pending.Height)
		})
	}
}

func nonceOpenAdmission(t *testing.T, h *consensusFenceHarness, lifetime time.Duration) *NonceAdmission {
	t.Helper()
	require.NoError(t, h.src.exec.Open(2, h.openedAt))
	h.src.app.SetOpenBlock(h.src.exec)
	a, err := NewNonceAdmission(Config{NonceQueueMaxPerSender: 4, NonceQueueMaxTotal: 16},
		nodeOpenNonces{h.src.app}, func(_ common.Address, _ uint64, tx []byte) error {
			_, err := h.src.exec.Accept(tx)
			return err
		}, lifetime)
	require.NoError(t, err)
	return a
}

func TestNonceOpenReleasesAllSendersInNonceOrder(t *testing.T) {
	first, second := utils.NewSigner(), utils.NewSigner()
	h := newConsensusFenceHarness(t, 42, first, func(ctx sdk.Context, a *app.App) {
		a.EvmKeeper.SetAddressMapping(ctx, second.AccountAddress, second.EvmAddress)
		require.NoError(t, a.BankKeeper.SendCoins(ctx, first.AccountAddress, second.AccountAddress,
			sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100_000_000_000))))
	})
	a := nonceOpenAdmission(t, h, time.Minute)
	txs := make([][]byte, 4)
	for nonce := range txs {
		txs[nonce] = consensusFenceTransfer(t, h.src.app, first, uint64(nonce))
	}
	for _, nonce := range []uint64{3, 1, 2} {
		held, err := a.Send(first.EvmAddress, nonce, txs[nonce])
		require.NoError(t, err)
		require.True(t, held)
	}
	secondTxs := [][]byte{
		consensusFenceTransfer(t, h.src.app, second, 0),
		consensusFenceTransfer(t, h.src.app, second, 1),
	}
	held, err := a.Send(second.EvmAddress, 1, secondTxs[1])
	require.NoError(t, err)
	require.True(t, held)
	_, err = h.src.exec.Accept(txs[0])
	require.NoError(t, err)
	_, err = h.src.exec.Accept(secondTxs[0])
	require.NoError(t, err)
	require.Empty(t, a.Open(time.Now()))
	require.Zero(t, a.queue.Len())
	require.Empty(t, a.queue.Failures())
	batch, err := h.src.exec.Seal(2)
	require.NoError(t, err)
	require.Equal(t, [][]byte{txs[0], secondTxs[0]}, batch.Txs[:2])
	firstOrder, secondOrder := [][]byte{}, [][]byte{}
	for _, tx := range batch.Txs[2:] {
		if bytes.Equal(tx, secondTxs[1]) {
			secondOrder = append(secondOrder, tx)
		} else {
			firstOrder = append(firstOrder, tx)
		}
	}
	require.Equal(t, txs[1:], firstOrder)
	require.Equal(t, secondTxs[1:], secondOrder)
	require.Equal(t, uint64(48), batch.LastSeq)
}

func nonceOpenHash(t *testing.T, a *app.App, tx []byte) common.Hash {
	t.Helper()
	decoded, err := a.GetTxConfig().TxDecoder()(tx)
	require.NoError(t, err)
	msg := a.GetEVMMsg(decoded)
	require.NotNil(t, msg)
	etx, _ := msg.AsTransaction()
	require.NotNil(t, etx)
	return etx.Hash()
}

func TestNonceOpenRefusalDropsOnlyFailedNonce(t *testing.T) {
	sender, other := utils.NewSigner(), utils.NewSigner()
	h := newConsensusFenceHarness(t, 42, sender, func(ctx sdk.Context, a *app.App) {
		a.EvmKeeper.SetAddressMapping(ctx, other.AccountAddress, other.EvmAddress)
		require.NoError(t, a.BankKeeper.SendCoins(ctx, sender.AccountAddress, other.AccountAddress,
			sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100_000_000_000))))
	})
	n := runtimeAdmissionPrimary(t, h, ModeSequential)
	require.NoError(t, n.Start(t.Context()))
	bad := sequencerRecoveryTransfer(t, n.App, sender, config.DefaultChainID+1, 1)
	held, err := n.NonceAdmission.Send(sender.EvmAddress, 1, bad)
	require.NoError(t, err)
	require.True(t, held)
	later := consensusFenceTransfer(t, n.App, sender, 2)
	held, err = n.NonceAdmission.Send(sender.EvmAddress, 2, later)
	require.NoError(t, err)
	require.True(t, held)
	_, err = n.Accepter.Accept(consensusFenceTransfer(t, n.App, sender, 0))
	require.NoError(t, err)
	otherTx := consensusFenceTransfer(t, n.App, other, 1)
	held, err = n.NonceAdmission.Send(other.EvmAddress, 1, otherTx)
	require.NoError(t, err)
	require.True(t, held)
	_, err = n.Accepter.Accept(consensusFenceTransfer(t, n.App, other, 0))
	require.NoError(t, err)
	runtimeAdmissionCommit(t, n, h)
	ctx, err := n.Executor.Snapshot()
	require.NoError(t, err)
	require.Equal(t, int64(3), ctx.BlockHeight())
	require.Equal(t, uint64(1), n.App.EvmKeeper.GetNonce(ctx, sender.EvmAddress))
	q := n.NonceAdmission.queue
	require.False(t, q.Held(sender.EvmAddress, 1))
	require.True(t, q.Held(sender.EvmAddress, 2))
	require.Equal(t, 1, q.Len())
	failures := q.Failures()
	require.Len(t, failures, 1)
	require.Equal(t, sender.EvmAddress, failures[0].Sender)
	require.Equal(t, uint64(1), failures[0].Nonce)
	require.Equal(t, bad, failures[0].Tx)
	var coded *sdkerrors.Error
	require.True(t, errors.As(failures[0].Err, &coded), "%v", failures[0].Err)
	require.Equal(t, sdkerrors.ErrInvalidChainID.ABCICode(), coded.ABCICode())
	refused, err := n.ReceiptSource.SequencerSend(nonceOpenHash(t, n.App, bad))
	require.Nil(t, refused)
	require.ErrorIs(t, err, failures[0].Err)
	stillHeld, err := n.ReceiptSource.SequencerSend(nonceOpenHash(t, n.App, later))
	require.NoError(t, err)
	require.NotNil(t, stillHeld)
	require.Equal(t, uint64(2), stillHeld.Nonce)
	require.Empty(t, stillHeld.Receipt)
	nonceOpenReceipt(t, n, nonceOpenHash(t, n.App, otherTx), 1, 3, 45)
	batch, err := n.Executor.Seal(3)
	require.NoError(t, err)
	require.Equal(t, [][]byte{otherTx}, batch.Txs)
}

func TestNonceOpenGapAndExpiry(t *testing.T) {
	sender := utils.NewSigner()
	h := newConsensusFenceHarness(t, 42, sender)
	a := nonceOpenAdmission(t, h, time.Minute)
	tx := consensusFenceTransfer(t, h.src.app, sender, 2)
	held, err := a.Send(sender.EvmAddress, 2, tx)
	require.NoError(t, err)
	require.True(t, held)
	require.Empty(t, a.Open(time.Now()))
	require.True(t, a.queue.Held(sender.EvmAddress, 2))
	dropped := a.Open(time.Now().Add(time.Minute + time.Second))
	require.Len(t, dropped, 1)
	require.Equal(t, sender.EvmAddress, dropped[0].Sender)
	require.Equal(t, uint64(2), dropped[0].Nonce)
	require.Equal(t, tx, dropped[0].Tx)
	require.Zero(t, a.queue.Len())
	require.Empty(t, a.queue.Failures())
	batch, err := h.src.exec.Seal(2)
	require.NoError(t, err)
	require.Empty(t, batch.Txs)
}

func TestNonceOpenExpiresBeforeRelease(t *testing.T) {
	sender := utils.NewSigner()
	h := newConsensusFenceHarness(t, 42, sender)
	a := nonceOpenAdmission(t, h, time.Minute)
	tx := consensusFenceTransfer(t, h.src.app, sender, 1)
	held, err := a.Send(sender.EvmAddress, 1, tx)
	require.NoError(t, err)
	require.True(t, held)
	_, err = h.src.exec.Accept(consensusFenceTransfer(t, h.src.app, sender, 0))
	require.NoError(t, err)
	dropped := a.Open(time.Now().Add(time.Minute + time.Second))
	require.Len(t, dropped, 1)
	require.Equal(t, tx, dropped[0].Tx)
	require.Zero(t, a.queue.Len())
	require.Empty(t, a.queue.Failures())
	batch, err := h.src.exec.Seal(2)
	require.NoError(t, err)
	require.Len(t, batch.Txs, 1)
	require.Equal(t, uint64(43), batch.LastSeq)
}

func TestNonceOpenClosedSendPrecedesNonceComparison(t *testing.T) {
	sender := utils.NewSigner()
	h := newConsensusFenceHarness(t, 42, sender)
	n := runtimeAdmissionPrimary(t, h, ModeSequential)
	_, err := n.Accept(consensusFenceTransfer(t, n.App, sender, 0))
	require.ErrorIs(t, err, ErrLayerNotOpen)
	require.NoError(t, n.Start(t.Context()))
	_, err = n.Accept(consensusFenceTransfer(t, n.App, sender, 0))
	require.NoError(t, err)
	_, err = n.ConsensusSource.Seal(t.Context(), 2)
	require.NoError(t, err)
	for _, nonce := range []uint64{0, 1, 2} {
		_, err = n.Accept(consensusFenceTransfer(t, n.App, sender, nonce))
		require.ErrorIs(t, err, ErrLayerNotOpen)
		require.NotErrorIs(t, err, ErrNonceHeld)
		require.NotErrorIs(t, err, ErrSameNonce)
	}
	require.Zero(t, n.NonceAdmission.queue.Len())
}

func TestNonceOpenDirectSendAndReleaseExecuteOnce(t *testing.T) {
	sender := utils.NewSigner()
	h := newConsensusFenceHarness(t, 42, sender)
	a := nonceOpenAdmission(t, h, time.Minute)
	txs := [][]byte{
		consensusFenceTransfer(t, h.src.app, sender, 0),
		consensusFenceTransfer(t, h.src.app, sender, 1),
		consensusFenceTransfer(t, h.src.app, sender, 2),
	}
	for _, nonce := range []uint64{2, 1} {
		held, err := a.Send(sender.EvmAddress, nonce, txs[nonce])
		require.NoError(t, err)
		require.True(t, held)
	}
	_, err := h.src.exec.Accept(txs[0])
	require.NoError(t, err)
	unlock := a.queue.lockSender(sender.EvmAddress)
	defer func() {
		if unlock != nil {
			unlock()
		}
	}()
	var wg sync.WaitGroup
	wg.Add(3)
	sent := make(chan error, 1)
	go func() {
		defer wg.Done()
		_, err := a.Send(sender.EvmAddress, 1, txs[1])
		sent <- err
	}()
	for range 2 {
		go func() {
			defer wg.Done()
			a.Open(time.Now())
		}()
	}
	require.Eventually(t, func() bool {
		a.queue.mu.Lock()
		defer a.queue.mu.Unlock()
		return a.queue.gates[sender.EvmAddress].users == 4
	}, time.Second, time.Millisecond)
	unlock()
	unlock = nil
	wg.Wait()
	require.ErrorIs(t, <-sent, ErrSameNonce)
	require.Zero(t, a.queue.Len())
	require.Empty(t, a.queue.Failures())
	batch, err := h.src.exec.Seal(2)
	require.NoError(t, err)
	require.Equal(t, txs, batch.Txs)
	require.Equal(t, uint64(45), batch.LastSeq)
}
