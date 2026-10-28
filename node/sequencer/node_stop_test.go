package sequencer_test

import (
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func TestNodeStopClosesWALAndRefusesAccept(t *testing.T) {
	for _, mode := range []sequencer.Mode{sequencer.ModeSequential, sequencer.ModeMicroBatch} {
		t.Run(mode.String(), func(t *testing.T) {
			sender := utils.NewSigner()
			f := newStartHarness(t, mode, sender)
			sb := f.restart(t, nil)
			require.NoError(t, f.n.Start(t.Context()))
			_, tx := transfer(t, sender, 0, sender.EvmAddress, big.NewInt(1))
			receipt, err := f.n.Accepter.Accept(tx)
			require.NoError(t, err)
			require.Equal(t, sourceLastSeq+1, sb.LastSeq())
			before := accepterWALEntries(t, f.cfg.WALDir)
			require.NoError(t, f.n.Stop())
			_, next := transfer(t, sender, 1, sender.EvmAddress, big.NewInt(1))
			for _, input := range [][]byte{next, nil} {
				got, err := f.n.Accepter.Accept(input)
				require.ErrorIs(t, err, sequencer.ErrReplicationHalted)
				require.Nil(t, got)
			}
			pending, err := f.n.WAL.Append(&wal.Accept{Height: uint64(openHeight), Seq: sourceLastSeq + 2, Tx: next, Receipt: receipt})
			require.ErrorIs(t, err, wal.ErrClosed)
			require.Nil(t, pending)
			_, err = f.n.WAL.GroupSync()
			require.ErrorIs(t, err, wal.ErrClosed)
			require.Equal(t, before, accepterWALEntries(t, f.cfg.WALDir))
			reopened, err := wal.OpenWAL(f.cfg.WALDir, wal.DefaultConfig())
			require.NoError(t, err, "Stop must release the WAL file lock")
			t.Cleanup(reopened.Close)
			require.Equal(t, receipt, before[len(before)-1].(*wal.Accept).Receipt)
		})
	}

	t.Run("waits for in-flight receipt replication", func(t *testing.T) {
		sender := utils.NewSigner()
		h := newAccepterHarness(t, sender)
		synced := make(chan struct{}, 1)
		rep, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{
			WAL: h.w, Dial: sequencer.DialStandby(h.l.Addr().String(), replicationTimeout),
			ReplicationTimeout: replicationTimeout,
			OnWALSync:          func(time.Duration) { synced <- struct{}{} },
		})
		require.NoError(t, err)
		t.Cleanup(rep.Close)
		r, err := sequencer.NewReceipter(h.h.exec, h.h.app, &h.h.app.EvmKeeper, h.key, receiptModuleID)
		require.NoError(t, err)
		metrics, _ := newSequencerMetricsReader(t)
		n := &sequencer.Node{
			WAL: h.w, Replicator: rep,
			Accepter: sequencer.NewAccepter(r, rep, h.src, metrics),
		}
		tx, bz := transfer(t, sender, 0, sender.EvmAddress, big.NewInt(1))
		type stopReceiptResult struct {
			receipt []byte
			err     error
		}
		accepted := make(chan stopReceiptResult, 1)
		go func() {
			receipt, err := n.Accepter.Accept(bz)
			accepted <- stopReceiptResult{receipt, err}
		}()
		select {
		case <-synced:
		case result := <-accepted:
			t.Fatalf("Accept returned before standby acknowledged: %v", result.err)
		case <-time.After(5 * time.Second):
			t.Fatal("Accept did not reach replication")
		}
		stopping := make(chan struct{})
		stopped := make(chan error, 1)
		go func() {
			close(stopping)
			stopped <- n.Stop()
		}()
		<-stopping
		select {
		case err := <-stopped:
			t.Fatalf("Stop returned before standby acknowledged: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
		h.serve()
		var result stopReceiptResult
		select {
		case result = <-accepted:
		case <-time.After(5 * time.Second):
			t.Fatal("Accept did not finish after standby acknowledged")
		}
		require.NoError(t, result.err)
		_, err = verify.ReceiptSignature(result.receipt, h.key.PublicKey())
		require.NoError(t, err)
		select {
		case err := <-stopped:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Stop did not finish after replication")
		}
		require.Equal(t, startSeq+1, h.sb.LastSeq())
		send, err := h.src.SequencerSend(tx.Hash())
		require.NoError(t, err)
		require.NotNil(t, send)
		require.Equal(t, result.receipt, send.Receipt)
		got, err := n.Accepter.Accept(bz)
		require.ErrorIs(t, err, sequencer.ErrReplicationHalted)
		require.Nil(t, got)
		_, err = n.WAL.GroupSync()
		require.ErrorIs(t, err, wal.ErrClosed)
	})
}

func TestNodeStopIsIdempotent(t *testing.T) {
	var disabled *sequencer.Node
	require.NoError(t, disabled.Stop())
	require.NoError(t, disabled.Stop())
	empty := &sequencer.Node{}
	require.NoError(t, empty.Stop())
	require.NoError(t, empty.Stop())
	f := newStartHarness(t, sequencer.ModeSequential, utils.NewSigner())
	f.restart(t, nil)
	require.NoError(t, f.n.Start(t.Context()))
	require.NoError(t, f.n.Stop())
	require.NoError(t, f.n.Stop())
	var calls sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < cap(results); i++ {
		calls.Add(1)
		go func() {
			defer calls.Done()
			results <- f.n.Stop()
		}()
	}
	calls.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	receipt, err := f.n.Accepter.Accept(nil)
	require.ErrorIs(t, err, sequencer.ErrReplicationHalted)
	require.Nil(t, receipt)
	_, err = f.n.WAL.GroupSync()
	require.ErrorIs(t, err, wal.ErrClosed)
}
