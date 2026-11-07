package sequencer_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func configureFailoverBlock(t *testing.T) {
	t.Helper()
	params := app.DefaultConsensusParams
	require.NotNil(t, params)
	require.NotNil(t, params.Block)
	configured := *params
	block := *params.Block
	block.MaxGas = 6 * transferGas
	block.MaxGasWanted = 6 * transferGas
	configured.Block = &block
	app.DefaultConsensusParams = &configured
	t.Cleanup(func() { app.DefaultConsensusParams = params })
}

func failoverKey(t *testing.T, sb *standby, first, last uint64, height int64) *sequencer.ReceiptKey {
	t.Helper()
	seed := sb.s.ReceiptKey().Seed()
	key, err := loadKey(t, seed, []verify.SequencerAuthorization{{
		SequencerID: testKeyID, PublicKey: publicOf(seed), FirstBatchNumber: first, LastBatchNumber: last,
	}}, height)
	require.NoError(t, err)
	return key
}

func replayFailoverWAL(t *testing.T, sb *standby, funded ...utils.TestAcct) wal.ReplayResult {
	t.Helper()
	h := newHarness(t, funded...)
	res, err := sb.wal.Replay(appReplayer{exec: h.exec})
	require.NoError(t, err)
	return res
}

func TestFailoverContinuesFromReplicatedSeq(t *testing.T) {
	configureFailoverBlock(t)
	ctx := context.Background()
	sender := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	l := listen(t)
	p := newPrimary(t, l.Addr().String(), replicationTimeout, sender)
	sb := newStandby(t, p, sender)
	sb.serve(t, l)
	open := p.open()
	entries := []wal.Entry{&wal.Lease{Epoch: 1, KeyID: [32]byte(label("old primary key")), ExpiryMs: uint64(p.openMs)}, open}
	var last *wal.Accept
	for nonce := uint64(0); nonce < 5; nonce++ {
		_, tx := transfer(t, sender, nonce, recipient, big.NewInt(onePaxInWei))
		last = p.accept(tx)
		entries = append(entries, last)
	}
	_, err := p.repl.Release(ctx, entries...)
	require.NoError(t, err)
	requireStandbyState(t, p, sb, sender.EvmAddress, recipient)
	p.repl.Close()

	key := failoverKey(t, sb, 1, 1, 1)
	newID := [32]byte(label("standby open height key id"))
	require.NoError(t, key.SetPins([]verify.SequencerAuthorization{
		{SequencerID: testKeyID, PublicKey: key.PublicKey(), FirstBatchNumber: 1, LastBatchNumber: 1},
		{SequencerID: newID, PublicKey: key.PublicKey(), FirstBatchNumber: uint64(openHeight), LastBatchNumber: 100},
	}))
	expiry := uint64(p.openMs) + 30_000
	result, err := sequencer.Failover(sb.s, key, openHeight, expiry)
	require.NoError(t, err)
	require.Equal(t, sequencer.FailoverResult{
		Lease: &wal.Lease{Epoch: 2, KeyID: newID, ExpiryMs: expiry}, LastSeq: startSeq + 5, OpenHeight: openHeight,
	}, result)
	res := replayFailoverWAL(t, sb, sender)
	require.Equal(t, result.Lease, res.Lease)
	require.Equal(t, last, res.LastLogged)
	require.Equal(t, 5, res.Accepts)

	_, nextTx := transfer(t, sender, 5, recipient, big.NewInt(onePaxInWei))
	next, err := sb.h.exec.Accept(nextTx)
	require.NoError(t, err)
	require.Equal(t, result.LastSeq+1, next.Seq)
	require.Equal(t, result.OpenHeight, next.Height)
	expected := p.accept(nextTx)
	gotReceipt, err := receiptBytes(next)
	require.NoError(t, err)
	require.Equal(t, expected.Receipt, gotReceipt)
}

func TestFailoverRefusesUnpinnedReceiptKey(t *testing.T) {
	configureFailoverBlock(t)
	ctx := context.Background()
	sender := utils.NewSigner()
	l := listen(t)
	p := newPrimary(t, l.Addr().String(), replicationTimeout, sender)
	sb := newStandby(t, p, sender)
	sb.serve(t, l)
	lease := &wal.Lease{Epoch: 7, ExpiryMs: 1000}
	_, err := p.repl.Release(ctx, lease, p.open())
	require.NoError(t, err)
	key := failoverKey(t, sb, 10, 20, 10)
	result, err := sequencer.Failover(sb.s, key, 21, 2000)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)
	require.Equal(t, sequencer.FailoverResult{}, result)

	otherSeed := label("another host receipt key")
	other, err := loadKey(t, otherSeed, []verify.SequencerAuthorization{{
		SequencerID: testKeyID, PublicKey: publicOf(otherSeed), FirstBatchNumber: 1, LastBatchNumber: 100,
	}}, openHeight)
	require.NoError(t, err)
	result, err = sequencer.Failover(sb.s, other, openHeight, 2000)
	require.Error(t, err)
	require.Equal(t, sequencer.FailoverResult{}, result)

	_, tx := transfer(t, sender, 0, utils.NewSigner().EvmAddress, big.NewInt(onePaxInWei))
	a := p.accept(tx)
	ack, err := p.repl.Release(ctx, a)
	require.NoError(t, err)
	require.Equal(t, sequencer.AckOK, ack.Status)
	require.Equal(t, a.Seq, ack.LastSeq)
	res := replayFailoverWAL(t, sb, sender)
	require.Equal(t, lease, res.Lease)
	require.Equal(t, a, res.LastLogged)
	require.NoError(t, sb.s.Err())
}

func TestFailoverFencesOldPrimary(t *testing.T) {
	configureFailoverBlock(t)
	ctx := context.Background()
	sender := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	l := listen(t)
	p := newPrimary(t, l.Addr().String(), replicationTimeout, sender)
	sb := newStandby(t, p, sender)
	sb.serve(t, l)
	open := p.open()
	entries := []wal.Entry{&wal.Lease{Epoch: 9, ExpiryMs: uint64(p.openMs)}, open}
	var last *wal.Accept
	for nonce := uint64(0); nonce < 5; nonce++ {
		_, tx := transfer(t, sender, nonce, recipient, big.NewInt(onePaxInWei))
		last = p.accept(tx)
		entries = append(entries, last)
	}
	_, err := p.repl.Release(ctx, entries...)
	require.NoError(t, err)
	key := failoverKey(t, sb, 1, 100, openHeight)
	result, err := sequencer.Failover(sb.s, key, openHeight, uint64(p.openMs)+30_000)
	require.NoError(t, err)
	require.Equal(t, uint64(10), result.Lease.Epoch)
	require.Equal(t, uint64(9), p.repl.Epoch())

	_, tx := transfer(t, sender, 5, recipient, big.NewInt(onePaxInWei))
	rejected := p.accept(tx)
	require.Equal(t, result.LastSeq+1, rejected.Seq)
	ack, err := p.repl.Release(ctx, rejected)
	require.ErrorIs(t, err, sequencer.ErrFenced)
	require.Equal(t, sequencer.Ack{}, ack)
	require.Equal(t, result.LastSeq, sb.s.LastSeq())
	require.Equal(t, uint64(5), sb.h.app.EvmKeeper.GetNonce(sb.h.openCtx(), sender.EvmAddress))
	res := replayFailoverWAL(t, sb, sender)
	require.Equal(t, result.Lease, res.Lease)
	require.Equal(t, last, res.LastLogged)
	require.Equal(t, 5, res.Accepts)

	p.repl.Close()
	ack, err = p.repl.Release(ctx, rejected)
	require.ErrorIs(t, err, sequencer.ErrFenced)
	require.Equal(t, sequencer.Ack{}, ack)
	require.Equal(t, result.LastSeq, sb.s.LastSeq())
	next, err := sb.h.exec.Accept(tx)
	require.NoError(t, err)
	require.Equal(t, result.LastSeq+1, next.Seq)
}

func TestRecoveryOrderPrefersLeaseThenLongerLog(t *testing.T) {
	result := func(epoch, seq uint64, receipt string) wal.ReplayResult {
		return wal.ReplayResult{Lease: &wal.Lease{Epoch: epoch}, LastLogged: &wal.Accept{Seq: seq, Receipt: []byte(receipt)}}
	}
	cases := []struct {
		name      string
		a, b      wal.ReplayResult
		want      int
		divergent bool
	}{
		{name: "longer first log", a: result(1, 9, "nine"), b: result(1, 7, "seven"), want: 0},
		{name: "longer second log", a: result(1, 7, "seven"), b: result(1, 9, "nine"), want: 1},
		{name: "higher second epoch beats longer log", a: result(1, 9, "nine"), b: result(2, 7, "seven"), want: 1},
		{name: "higher first epoch beats longer log", a: result(2, 7, "seven"), b: result(1, 9, "nine"), want: 0},
		{name: "identical receipts", a: result(1, 9, "nine"), b: result(1, 9, "nine"), want: 0},
		{name: "divergent receipts", a: result(1, 9, "nine"), b: result(1, 9, "other"), divergent: true},
		{name: "divergence across epochs", a: result(1, 9, "nine"), b: result(2, 9, "other"), divergent: true},
		{name: "empty logs", want: 0},
		{name: "missing first lease and log", b: result(1, 9, "nine"), want: 1},
		{name: "missing second lease and log", a: result(1, 9, "nine"), want: 0},
		{name: "lease without accepts", a: wal.ReplayResult{Lease: &wal.Lease{Epoch: 2}}, b: result(1, 9, "nine"), want: 0},
		{name: "logs without leases", a: wal.ReplayResult{LastLogged: &wal.Accept{Seq: 7}}, b: wal.ReplayResult{LastLogged: &wal.Accept{Seq: 9}}, want: 1},
		{name: "last logged precedes commit", a: wal.ReplayResult{Lease: &wal.Lease{Epoch: 1}, LastLogged: &wal.Accept{Seq: 9, Receipt: []byte("nine")}, From: &wal.Committed{LastSeq: 9}}, b: result(1, 7, "seven"), want: 0},
		{name: "maximum epoch", a: result(^uint64(0), 7, "seven"), b: result(1, 9, "nine"), want: 0},
		{name: "maximum seq", a: result(1, ^uint64(0), "last"), b: result(1, 9, "nine"), want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sequencer.RecoveryOrder(tc.a, tc.b)
			if tc.divergent {
				require.ErrorIs(t, err, sequencer.ErrDivergentLogs)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
