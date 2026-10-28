package sequencer_test

import (
	"context"
	"math/big"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func TestHandoverPlannedSteps(t *testing.T) {
	configureFailoverBlock(t)
	t.Run("drain requires the exact retained frontier", func(t *testing.T) {
		sender := utils.NewSigner()
		recipient := utils.NewSigner().EvmAddress
		l := listen(t)
		p := newPrimary(t, l.Addr().String(), replicationTimeout, sender)
		sb := newStandby(t, p, sender)
		sb.serve(t, l)
		open := p.open()
		_, firstTx := transfer(t, sender, 0, recipient, big.NewInt(1))
		first := p.accept(firstTx)
		_, secondTx := transfer(t, sender, 1, recipient, big.NewInt(2))
		last := p.accept(secondTx)
		_, err := p.repl.Release(context.Background(), open, first, last)
		require.NoError(t, err)
		w, err := wal.OpenWAL(filepath.Join(t.TempDir(), "shorter-wal"), wal.DefaultConfig())
		require.NoError(t, err)
		t.Cleanup(w.Close)
		r, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{
			WAL: w, Dial: sequencer.DialStandby(l.Addr().String(), replicationTimeout), ReplicationTimeout: replicationTimeout,
		})
		require.NoError(t, err)
		t.Cleanup(r.Close)
		ack, err := r.Release(context.Background(), open, first)
		require.NoError(t, err)
		require.Equal(t, last.Seq, ack.LastSeq)
		ack, err = r.Drain(context.Background())
		require.ErrorIs(t, err, sequencer.ErrNoAck)
		require.Equal(t, sequencer.Ack{}, ack)
	})

	t.Run("drain failure stays fenced and retry catches up", func(t *testing.T) {
		ctx := context.Background()
		sender := utils.NewSigner()
		recipient := utils.NewSigner().EvmAddress
		l := listen(t)
		p := newPrimary(t, l.Addr().String(), replicationTimeout, sender)
		sb := newStandby(t, p, sender)
		sb.serve(t, l)
		open := p.open()
		_, firstTx := transfer(t, sender, 0, recipient, big.NewInt(1))
		first := p.accept(firstTx)
		_, err := p.repl.Release(ctx, &wal.Lease{Epoch: 7, ExpiryMs: uint64(p.openMs)}, open, first)
		require.NoError(t, err)
		p.repl.Close()
		_, secondTx := transfer(t, sender, 1, recipient, big.NewInt(2))
		last := p.accept(secondTx)
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		_, err = p.repl.Release(cancelled, last)
		require.Error(t, err)
		require.Equal(t, first.Seq, sb.s.LastSeq())
		cfg := sequencer.HandoverConfig{
			Primary: p.h.exec, Replicator: p.repl, Standby: sb.s,
			Key: failoverKey(t, sb, 1, 100, openHeight), OpenHeight: openHeight, ExpiryMs: uint64(p.openMs) + 30_000,
		}
		before := accepterWALEntries(t, sb.dir)
		result, err := sequencer.Handover(cancelled, cfg)
		require.ErrorIs(t, err, sequencer.ErrNoAck)
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, sequencer.HandoverResult{}, result)
		require.Equal(t, before, accepterWALEntries(t, sb.dir))
		_, nextTx := transfer(t, sender, 2, recipient, big.NewInt(3))
		accepted, err := p.h.exec.Accept(nextTx)
		require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
		require.Nil(t, accepted)

		result, err = sequencer.Handover(ctx, cfg)
		require.NoError(t, err)
		require.Equal(t, sequencer.HandoverResult{
			Lease: &wal.Lease{Epoch: 8, KeyID: testKeyID, ExpiryMs: cfg.ExpiryMs}, LastSeq: last.Seq,
		}, result)
		require.Equal(t, uint64(7), p.repl.Epoch())
		require.Equal(t, last.Seq, sb.s.LastSeq())
		requireStandbyState(t, p, sb, sender.EvmAddress, recipient)
		res := replayFailoverWAL(t, sb, sender)
		require.Equal(t, result.Lease, res.Lease)
		require.Equal(t, last, res.LastLogged)
		require.Equal(t, 2, res.Accepts)
		_, err = p.repl.Drain(ctx)
		require.ErrorIs(t, err, sequencer.ErrFenced)
	})

	t.Run("lease persistence failure stays fenced", func(t *testing.T) {
		sender := utils.NewSigner()
		l := listen(t)
		p := newPrimary(t, l.Addr().String(), replicationTimeout, sender)
		sb := newStandby(t, p, sender)
		sb.serve(t, l)
		checkpoint := &wal.Committed{Height: uint64(committedTip), LastSeq: startSeq}
		copy(checkpoint.AppHash[:], p.h.app.CommitMultiStore().LastCommitID().Hash)
		_, err := p.repl.Release(context.Background(), &wal.Lease{Epoch: 3}, checkpoint)
		require.NoError(t, err)
		open := p.open()
		_, tx := transfer(t, sender, 0, utils.NewSigner().EvmAddress, big.NewInt(1))
		last := p.accept(tx)
		_, err = p.repl.Release(context.Background(), open, last)
		require.NoError(t, err)
		before := accepterWALEntries(t, sb.dir)
		sb.wal.Close()
		result, err := sequencer.Handover(context.Background(), sequencer.HandoverConfig{
			Primary: p.h.exec, Replicator: p.repl, Standby: sb.s,
			Key: failoverKey(t, sb, 1, 100, openHeight), OpenHeight: openHeight, ExpiryMs: uint64(p.openMs) + 30_000,
		})
		require.ErrorIs(t, err, wal.ErrClosed)
		require.Equal(t, sequencer.HandoverResult{}, result)
		require.Equal(t, before, accepterWALEntries(t, sb.dir))
		require.Equal(t, last.Seq, sb.s.LastSeq())
		accepted, err := p.h.exec.Accept(tx)
		require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
		require.Nil(t, accepted)
	})
}

func TestHandoverRefusesSendsDuringWindow(t *testing.T) {
	configureFailoverBlock(t)
	sender := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	l := listen(t)
	p := newPrimary(t, l.Addr().String(), replicationTimeout, sender)
	primaryDir := t.TempDir()
	w, err := wal.OpenWAL(primaryDir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(w.Close)
	p.wal = w
	draining, resume := make(chan struct{}), make(chan struct{})
	var armed atomic.Bool
	var release sync.Once
	continueDrain := func() { release.Do(func() { close(resume) }) }
	t.Cleanup(continueDrain)
	p.repl, err = sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: w, ReplicationTimeout: replicationTimeout,
		Dial: func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			conn, err := d.DialContext(ctx, "tcp", l.Addr().String())
			if err != nil {
				return nil, err
			}
			if armed.CompareAndSwap(true, false) {
				close(draining)
				select {
				case <-resume:
				case <-ctx.Done():
					_ = conn.Close()
					return nil, ctx.Err()
				}
			}
			return conn, nil
		},
	})
	require.NoError(t, err)
	t.Cleanup(p.repl.Close)
	sb := newStandby(t, p, sender)
	sb.serve(t, l)
	checkpoint := &wal.Committed{Height: uint64(committedTip), LastSeq: startSeq}
	copy(checkpoint.AppHash[:], p.h.app.CommitMultiStore().LastCommitID().Hash)
	_, err = p.repl.Release(context.Background(), &wal.Lease{Epoch: 11}, checkpoint)
	require.NoError(t, err)
	open := p.open()
	_, firstTx := transfer(t, sender, 0, recipient, big.NewInt(1))
	first := p.accept(firstTx)
	_, err = p.repl.Release(context.Background(), open, first)
	require.NoError(t, err)
	p.repl.Close()
	primaryBefore := accepterWALEntries(t, primaryDir)
	standbyBefore := accepterWALEntries(t, sb.dir)
	key := failoverKey(t, sb, 1, 100, openHeight)
	r, err := sequencer.NewReceipter(p.h.exec, p.h.app, &p.h.app.EvmKeeper, key, receiptModuleID)
	require.NoError(t, err)
	m, _ := newSequencerMetricsReader(t)
	src := sequencer.NewReceiptSource(32)
	send := sequencer.NewAccepter(r, p.repl, src, m)
	armed.Store(true)
	type outcome struct {
		result sequencer.HandoverResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := sequencer.Handover(context.Background(), sequencer.HandoverConfig{
			Primary: p.h.exec, Replicator: p.repl, Standby: sb.s,
			Key: key, OpenHeight: openHeight, ExpiryMs: uint64(p.openMs) + 30_000,
		})
		done <- outcome{result, err}
	}()
	select {
	case <-draining:
	case <-time.After(replicationTimeout):
		t.Fatal("handover did not reach drain")
	}
	_, nextTx := transfer(t, sender, 1, recipient, big.NewInt(2))
	accepted, err := p.h.exec.Accept(nextTx)
	require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
	require.Nil(t, accepted)
	receipt, err := send.Accept(nextTx)
	require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
	require.Nil(t, receipt)
	require.Equal(t, primaryBefore, accepterWALEntries(t, primaryDir))
	require.Equal(t, standbyBefore, accepterWALEntries(t, sb.dir))
	require.Equal(t, uint64(1), p.h.app.EvmKeeper.GetNonce(p.h.openCtx(), sender.EvmAddress))
	require.Equal(t, first.Seq, sb.s.LastSeq())
	continueDrain()
	select {
	case got := <-done:
		require.NoError(t, got.err)
		require.Equal(t, first.Seq, got.result.LastSeq)
		require.Equal(t, uint64(12), got.result.Lease.Epoch)
	case <-time.After(replicationTimeout):
		t.Fatal("handover did not finish")
	}
	require.Equal(t, primaryBefore, accepterWALEntries(t, primaryDir))
	entries := accepterWALEntries(t, sb.dir)
	require.Len(t, entries, len(standbyBefore)+1)
	require.Equal(t, standbyBefore, entries[:len(standbyBefore)])
	receipt, err = send.Accept(nextTx)
	require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
	require.Nil(t, receipt)
	next, err := sb.h.exec.Accept(nextTx)
	require.NoError(t, err)
	require.Equal(t, first.Seq+1, next.Seq)
}

func TestHandoverRefusesUnpinnedStandbyKey(t *testing.T) {
	configureFailoverBlock(t)
	sender := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	l := listen(t)
	p := newPrimary(t, l.Addr().String(), replicationTimeout, sender)
	sb := newStandby(t, p, sender)
	sb.serve(t, l)
	open := p.open()
	_, err := p.repl.Release(context.Background(), &wal.Lease{Epoch: 5}, open)
	require.NoError(t, err)
	before := accepterWALEntries(t, sb.dir)
	key := failoverKey(t, sb, 1, 1, 1)
	cfg := sequencer.HandoverConfig{
		Primary: p.h.exec, Replicator: p.repl, Standby: sb.s,
		Key: key, OpenHeight: openHeight, ExpiryMs: uint64(p.openMs) + 30_000,
	}
	result, err := sequencer.Handover(context.Background(), cfg)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)
	require.Equal(t, sequencer.HandoverResult{}, result)
	require.Equal(t, before, accepterWALEntries(t, sb.dir))
	_, firstTx := transfer(t, sender, 0, recipient, big.NewInt(1))
	first := p.accept(firstTx)
	_, err = p.repl.Release(context.Background(), first)
	require.NoError(t, err)

	foreignSeed := label("handover foreign host key")
	foreign, err := loadKey(t, foreignSeed, []verify.SequencerAuthorization{{
		SequencerID: testKeyID, PublicKey: publicOf(foreignSeed), FirstBatchNumber: 1, LastBatchNumber: 100,
	}}, openHeight)
	require.NoError(t, err)
	cfg.Key = foreign
	result, err = sequencer.Handover(context.Background(), cfg)
	require.ErrorContains(t, err, "differs from standby key")
	require.Equal(t, sequencer.HandoverResult{}, result)
	_, secondTx := transfer(t, sender, 1, recipient, big.NewInt(2))
	second := p.accept(secondTx)
	_, err = p.repl.Release(context.Background(), second)
	require.NoError(t, err)
	newID := [32]byte(label("handover standby pinned key"))
	require.NoError(t, key.SetPins([]verify.SequencerAuthorization{{
		SequencerID: newID, PublicKey: key.PublicKey(), FirstBatchNumber: uint64(openHeight), LastBatchNumber: 100,
	}}))
	cfg.Key = key
	result, err = sequencer.Handover(context.Background(), cfg)
	require.NoError(t, err)
	require.Equal(t, newID, result.Lease.KeyID)
	require.Equal(t, second.Seq, result.LastSeq)
	_, nextTx := transfer(t, sender, 2, recipient, big.NewInt(3))
	r, err := sequencer.NewReceipter(sb.h.exec, sb.h.app, &sb.h.app.EvmKeeper, key, receiptModuleID)
	require.NoError(t, err)
	accepted, signed, err := r.Accept(nextTx)
	require.NoError(t, err)
	require.Equal(t, second.Seq+1, accepted.Seq)
	verified, err := verify.ReceiptSignature(signed, key.PublicKey())
	require.NoError(t, err)
	require.Equal(t, newID, verified.Receipt.ContextHash)
}

func TestHandoverOpenBlockEqual(t *testing.T) {
	configureFailoverBlock(t)
	sender := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	l := listen(t)
	p := newPrimary(t, l.Addr().String(), replicationTimeout, sender)
	sb := newStandby(t, p, sender)
	sb.serve(t, l)
	open := p.open()
	expected := newHarness(t, sender)
	require.NoError(t, expected.exec.Open(openHeight, p.openMs))
	entries := []wal.Entry{&wal.Lease{Epoch: 19}, open}
	var txs [][]byte
	for nonce := uint64(0); nonce < 5; nonce++ {
		_, tx := transfer(t, sender, nonce, recipient, big.NewInt(int64(nonce)+1))
		entry := p.accept(tx)
		acc, err := expected.exec.Accept(tx)
		require.NoError(t, err)
		receipt, err := receiptBytes(acc)
		require.NoError(t, err)
		require.Equal(t, entry.Receipt, receipt)
		entries = append(entries, entry)
		txs = append(txs, tx)
	}
	_, err := p.repl.Release(context.Background(), entries...)
	require.NoError(t, err)
	want, err := expected.exec.Seal(openHeight)
	require.NoError(t, err)
	result, err := sequencer.Handover(context.Background(), sequencer.HandoverConfig{
		Primary: p.h.exec, Replicator: p.repl, Standby: sb.s,
		Key: failoverKey(t, sb, 1, 100, openHeight), OpenHeight: openHeight, ExpiryMs: uint64(p.openMs) + 30_000,
	})
	require.NoError(t, err)
	require.Equal(t, want.LastSeq, result.LastSeq)
	require.Equal(t, uint64(20), result.Lease.Epoch)
	got, err := sb.h.exec.Seal(openHeight)
	require.NoError(t, err)
	require.Equal(t, want.FromSeq, got.FromSeq)
	require.Equal(t, want.LastSeq, got.LastSeq)
	require.Equal(t, want.LastSeqHash, got.LastSeqHash)
	require.Equal(t, want.PrevSeqHash, got.PrevSeqHash)
	require.Equal(t, txs, got.Txs)
	require.Equal(t, p.openMs, got.TimestampMs)
	batch, err := p.h.exec.Seal(openHeight)
	require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
	require.Nil(t, batch)
	entries = accepterWALEntries(t, sb.dir)
	require.Equal(t, result.Lease, entries[len(entries)-1])
}

func handoverEmptyOpenAfterCommit(t *testing.T) *driverHarness {
	t.Helper()
	f := newDriverHarness(t, 2, wal.DefaultConfig().SegmentSizeLimit)
	_, err := f.repl.Release(context.Background(), &wal.Lease{Epoch: 23})
	require.NoError(t, err)
	require.NoError(t, f.driver.OnCommit(committedTip, f.h.app.CommitMultiStore().LastCommitID().Hash))
	_, tx := transfer(t, f.sender, 0, utils.NewSigner().EvmAddress, big.NewInt(1))
	held, err := f.queue.Hold(f.sender.EvmAddress, 0, tx)
	require.NoError(t, err)
	require.False(t, held)
	require.Equal(t, sourceLastSeq+1, f.lastSeq)
	hash := f.commit(t, openHeight)
	require.NoError(t, f.driver.OnCommit(openHeight, hash))
	entries := driverEntries(t, f.dir)
	kinds := make([]wal.EntryKind, len(entries))
	for i, entry := range entries {
		kinds[i] = entry.Kind()
	}
	require.Equal(t, []wal.EntryKind{
		wal.KindLease, wal.KindCommitted, wal.KindOpen, wal.KindAccept,
		wal.KindSeal, wal.KindCommitted, wal.KindOpen,
	}, kinds)
	require.Equal(t, &wal.Committed{
		Height: uint64(openHeight), LastSeq: f.lastSeq, AppHash: [32]byte(hash),
	}, f.w.LastCommitted())
	require.Equal(t, f.w.LastCommitted(), f.sw.LastCommitted())
	require.Equal(t, uint64(openHeight+1), entries[len(entries)-1].(*wal.Open).Height)
	require.Equal(t, openHeight+1, f.h.openCtx().BlockHeight())
	require.Equal(t, openHeight+1, f.sb.openCtx().BlockHeight())
	require.Equal(t, f.lastSeq, f.standby.LastSeq())
	return f
}

func TestHandoverEmptyOpenAfterCommit(t *testing.T) {
	for _, reconnect := range []bool{false, true} {
		name := "connected"
		if reconnect {
			name = "reconnected"
		}
		t.Run(name, func(t *testing.T) {
			f := handoverEmptyOpenAfterCommit(t)
			if reconnect {
				f.repl.Close()
			}
			primaryBefore := driverEntries(t, f.dir)
			standbyBefore := driverEntries(t, f.sdir)
			key := failoverKey(t, &standby{s: f.standby}, 1, 100, openHeight+1)
			expiry := uint64(f.h.openCtx().BlockTime().UnixMilli()) + 30_000
			result, err := sequencer.Handover(context.Background(), sequencer.HandoverConfig{
				Primary: f.h.exec, Replicator: f.repl, Standby: f.standby,
				Key: key, OpenHeight: openHeight + 1, ExpiryMs: expiry,
			})
			require.NoError(t, err)
			require.Equal(t, sequencer.HandoverResult{
				Lease: &wal.Lease{Epoch: 24, KeyID: testKeyID, ExpiryMs: expiry}, LastSeq: f.lastSeq,
			}, result)
			require.Equal(t, uint64(23), f.repl.Epoch())
			require.Equal(t, f.lastSeq, f.standby.LastSeq())
			require.Equal(t, primaryBefore, driverEntries(t, f.dir))
			require.Equal(t, append(standbyBefore, result.Lease), driverEntries(t, f.sdir))
			_, nextTx := transfer(t, f.sender, 1, utils.NewSigner().EvmAddress, big.NewInt(2))
			accepted, err := f.h.exec.Accept(nextTx)
			require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
			require.Nil(t, accepted)
			next, err := f.sb.exec.Accept(nextTx)
			require.NoError(t, err)
			require.Equal(t, f.lastSeq+1, next.Seq)
			require.Equal(t, openHeight+1, next.Height)
			_, err = f.repl.Drain(context.Background())
			require.ErrorIs(t, err, sequencer.ErrFenced)
		})
	}
}

func TestDrainRetainedFrontierAfterCommit(t *testing.T) {
	f := handoverEmptyOpenAfterCommit(t)
	ack, err := f.repl.Drain(context.Background())
	require.NoError(t, err)
	require.Equal(t, sequencer.Ack{Status: sequencer.AckOK, LastSeq: f.lastSeq}, ack)
	_, tx := transfer(t, f.sender, 1, utils.NewSigner().EvmAddress, big.NewInt(2))
	accepted, err := f.h.exec.Accept(tx)
	require.NoError(t, err)
	receipt, err := receiptBytes(accepted)
	require.NoError(t, err)
	l := listen(t)
	t.Cleanup(func() { _ = l.Close() })
	go func() { _ = f.standby.Serve(l) }()
	conn, err := net.DialTimeout("tcp", l.Addr().String(), replicationTimeout)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.Equal(t, sequencer.Ack{Status: sequencer.AckOK, LastSeq: f.lastSeq + 1},
		standbyLeaseExchange(t, conn, &wal.Accept{
			Seq: accepted.Seq, Height: uint64(accepted.Height), Tx: tx, Receipt: receipt,
		}))
	before := driverEntries(t, f.sdir)
	ack, err = f.repl.Drain(context.Background())
	require.ErrorIs(t, err, sequencer.ErrNoAck)
	require.Equal(t, sequencer.Ack{}, ack)
	require.Equal(t, before, driverEntries(t, f.sdir))
	require.NoError(t, f.standby.Err())
}
