package sequencer_test

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func replayReceipter(t *testing.T, h *harness, exec sequencer.Executor) *sequencer.Receipter {
	t.Helper()
	key, err := loadKey(t, testKeySeed, []verify.SequencerAuthorization{{
		SequencerID: testKeyID, PublicKey: publicOf(testKeySeed), FirstBatchNumber: 1, LastBatchNumber: 1000,
	}}, openHeight)
	require.NoError(t, err)
	r, err := sequencer.NewReceipter(exec, h.app, &h.app.EvmKeeper, key, receiptModuleID)
	require.NoError(t, err)
	return r
}

func replayExecutor(t *testing.T, h *harness) (sequencer.Executor, *sequencer.AppReplayer) {
	t.Helper()
	exec, err := sequencer.NewExecutor(h.app, sequencer.Options{
		ChainID: testChainID, ProposerAddress: h.proposer, LastSeq: startSeq, LastSeqHash: startSeqHash,
	})
	require.NoError(t, err)
	return exec, sequencer.NewAppReplayer(exec, replayReceipter(t, h, exec))
}

func replayEntries(t *testing.T, h *harness, sender utils.TestAcct, count int) (*wal.Open, []*wal.Accept) {
	t.Helper()
	openMs := h.open()
	open := &wal.Open{Height: uint64(openHeight), OpenTimeMs: uint64(openMs), BaseFee: uint256.NewInt(0)}
	r := replayReceipter(t, h, h.exec)
	entries := make([]*wal.Accept, count)
	for i := range entries {
		_, tx := transfer(t, sender, uint64(i), sender.EvmAddress, big.NewInt(int64(i+1)))
		acc, receipt, err := r.Accept(tx)
		require.NoError(t, err)
		_, err = verify.ReceiptSignature(receipt, publicOf(testKeySeed))
		require.NoError(t, err)
		entries[i] = &wal.Accept{Seq: acc.Seq, Height: uint64(acc.Height), Tx: tx, Receipt: receipt}
	}
	return open, entries
}

func reopenReplayWAL(t *testing.T, entries ...wal.Entry) *wal.WAL {
	t.Helper()
	dir := t.TempDir()
	w, err := wal.OpenWAL(dir, wal.DefaultConfig())
	require.NoError(t, err)
	defer w.Close()
	pending := make([]*wal.Pending, 0, len(entries))
	for _, entry := range entries {
		p, err := w.Append(entry)
		require.NoError(t, err)
		pending = append(pending, p)
	}
	n, err := w.GroupSync()
	require.NoError(t, err)
	require.Equal(t, len(entries), n)
	for _, p := range pending {
		require.NoError(t, p.Wait(context.Background()))
	}
	w.Close()
	w, err = wal.OpenWAL(dir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(w.Close)
	return w
}

func TestAppReplayerReproducesReceipts(t *testing.T) {
	sender := utils.NewSigner()
	h := newAccepterAppHarness(t, sender)
	open, accepts := replayEntries(t, h, sender, 3)
	committed := &wal.Committed{Height: uint64(committedTip), LastSeq: startSeq}
	lease := &wal.Lease{Epoch: 1, KeyID: testKeyID, ExpiryMs: open.OpenTimeMs + 60_000}
	entries := []wal.Entry{committed, lease, open}
	for _, accept := range accepts {
		entries = append(entries, accept)
	}
	w := reopenReplayWAL(t, entries...)
	exec, replayer := replayExecutor(t, h)
	result, err := w.Replay(replayer)
	require.NoError(t, err)
	require.Equal(t, len(accepts), result.Accepts)
	require.Equal(t, committed, result.From)
	require.Equal(t, lease, result.Lease)
	require.Equal(t, accepts[len(accepts)-1], result.LastAccept)
	ctx, err := exec.Snapshot()
	require.NoError(t, err)
	require.Equal(t, openHeight, ctx.BlockHeight())
	require.Equal(t, int64(open.OpenTimeMs), ctx.BlockTime().UnixMilli())
	require.Equal(t, uint64(len(accepts)), h.app.EvmKeeper.GetNonce(ctx, sender.EvmAddress))
	requireWei(t, h.weiBalance(h.openCtx(), sender.EvmAddress), h.weiBalance(ctx, sender.EvmAddress))
	batch, err := h.exec.Seal(openHeight)
	require.NoError(t, err)
	replayed, err := exec.Seal(openHeight)
	require.NoError(t, err)
	require.Equal(t, batch.Txs, replayed.Txs)
	require.Equal(t, batch.Results, replayed.Results)
	require.Equal(t, batch.FromSeq, replayed.FromSeq)
	require.Equal(t, batch.LastSeq, replayed.LastSeq)
	require.Equal(t, batch.LastSeqHash, replayed.LastSeqHash)

	_, sealedReplayer := replayExecutor(t, h)
	seal, err := w.Append(&wal.Seal{
		Height: uint64(openHeight), FromSeq: batch.FromSeq, ToSeq: batch.LastSeq, LastSeqHash: batch.LastSeqHash,
	})
	require.NoError(t, err)
	_, err = w.GroupSync()
	require.NoError(t, err)
	require.NoError(t, seal.Wait(context.Background()))
	sealedResult, err := w.Replay(sealedReplayer)
	require.NoError(t, err)
	require.Equal(t, len(accepts), sealedResult.Accepts)
	require.ErrorIs(t, sealedReplayer.Apply(&wal.Seal{Height: uint64(openHeight)}), sequencer.ErrLayerNotOpen)
}

func TestAppReplayerRefusesTamperedReceipt(t *testing.T) {
	sender := utils.NewSigner()
	h := newAccepterAppHarness(t, sender)
	open, accepts := replayEntries(t, h, sender, 3)
	tampered := *accepts[1]
	tampered.Receipt = bytes.Clone(tampered.Receipt)
	tampered.Receipt[len(tampered.Receipt)-1] ^= 0xff
	w := reopenReplayWAL(t, open, accepts[0], &tampered, accepts[2])
	exec, replayer := replayExecutor(t, h)
	result, err := w.Replay(replayer)
	require.ErrorIs(t, err, wal.ErrReceiptMismatch)
	require.Equal(t, 1, result.Accepts)
	require.Equal(t, accepts[0], result.LastAccept)
	ctx, err := exec.Snapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(2), h.app.EvmKeeper.GetNonce(ctx, sender.EvmAddress))

	_, direct := replayExecutor(t, h)
	require.NoError(t, direct.Apply(open))
	got, err := direct.Execute(accepts[0])
	require.NoError(t, err)
	require.Equal(t, accepts[0].Receipt, got)
	got, err = direct.Execute(&tampered)
	require.ErrorIs(t, err, wal.ErrReceiptMismatch)
	require.Nil(t, got)
	_, err = verify.ReceiptSignature(accepts[1].Receipt, publicOf(testKeySeed))
	require.NoError(t, err)
}
