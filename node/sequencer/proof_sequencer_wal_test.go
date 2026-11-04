// Proves: Accepter, Receipter and LeaseHolder -> Replicator, DialStandby and WAL -> Standby, AppReplayer, Failover and Handover.
// All receipts come from real transaction execution; the TCP and disk crossings use
// production code. Closing and reopening the standby WAL rebuilds its executor from
// committed application state before duplicate delivery and promotion are checked.
// Repairs node/sequencer/standby.go: Standby.TakeLease must refuse a nil lease with
// wal.ErrMalformed, without panicking or changing its durable position.
// Also repairs standbyCursor.admit: retained lease retries must be idempotent
// after restart while a renewal with a later expiry must still be persisted.
// The historical ACK contract names a 25-byte prefix; standby.go now appends an
// eight-byte epoch. Fencing below proves that the current consumer reads that epoch.
package sequencer_test

import (
	"context"
	"crypto/ed25519"
	"math/big"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// The proof uses the real base-chain domain rather than the generic test chain ID.
const proofWalChainID = 125

// proofWalPath holds real application executors, stores and a TCP server. It
// deliberately uses the exported production AppReplayer rather than a test executor.
type proofWalPath struct {
	p, backup    *harness
	pw, sw       *wal.WAL
	rep          *sequencer.Replicator
	sb           *sequencer.Standby
	key          *sequencer.ReceiptKey
	accepter     *sequencer.Accepter
	holder       *sequencer.LeaseHolder
	cfg          sequencer.StandbyConfig
	dir, address string
	auth         sequencer.ReplicationTLSConfig
	stop         func()
}

func newProofWalPath(t *testing.T, sender utils.TestAcct) *proofWalPath {
	t.Helper()
	f := &proofWalPath{p: newAccepterAppHarness(t, sender), backup: newAccepterAppHarness(t, sender), dir: t.TempDir()}
	var err error
	f.pw, err = wal.OpenWAL(t.TempDir(), wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(f.pw.Close)
	f.sw, err = wal.OpenWAL(f.dir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(func() {
		if f.stop != nil {
			f.stop()
		}
		f.sw.Close()
	})
	keys := t.TempDir()
	pv, err := privval.GenFilePV(filepath.Join(keys, "primary.json"), filepath.Join(keys, "primary-state.json"), "")
	require.NoError(t, err)
	sv, err := privval.GenFilePV(filepath.Join(keys, "standby.json"), filepath.Join(keys, "standby-state.json"), "")
	require.NoError(t, err)
	pub, err := pv.GetPubKey(t.Context())
	require.NoError(t, err)
	_, standbyKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	f.key, err = loadKey(t, testKeySeed, []verify.SequencerAuthorization{{
		SequencerID: testKeyID, PublicKey: publicOf(testKeySeed), FirstBatchNumber: 1, LastBatchNumber: 1000,
	}}, openHeight)
	require.NoError(t, err)
	primaryPublic := f.key.PublicKey()
	f.cfg = sequencer.StandbyConfig{
		ConsensusKey: sv, ReceiptKey: standbyKey,
		PrimaryConsensusKey: pub, PrimaryReceiptKey: primaryPublic[:],
		CommittedHeight: uint64(committedTip), LastSeq: startSeq,
	}
	f.start(t)
	f.rep, err = sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: f.pw, ReplicationTimeout: replicationTimeout,
		Dial: func(ctx context.Context) (net.Conn, error) {
			return sequencer.DialStandby(f.address, replicationTimeout, f.auth)(ctx)
		},
	})
	require.NoError(t, err)
	t.Cleanup(f.rep.Close)
	f.holder, err = sequencer.NewLeaseHolder(sequencer.LeaseHolderConfig{
		Replicator: f.rep, KeyID: testKeyID, LeaseDuration: 3 * time.Minute,
	})
	require.NoError(t, err)
	require.NoError(t, f.holder.Start(t.Context()))
	require.Equal(t, uint64(1), f.holder.Epoch())
	require.Equal(t, f.holder.Epoch(), f.rep.Epoch())
	openMs := f.p.openAt(time.Now().Add(time.Minute))
	ack, err := f.rep.Release(t.Context(), &wal.Open{
		Height: uint64(openHeight), OpenTimeMs: uint64(openMs), BaseFee: uint256.NewInt(0),
	})
	require.NoError(t, err)
	require.Equal(t, sequencer.AckOK, ack.Status)
	require.Equal(t, startSeq, ack.LastSeq)
	r, err := sequencer.NewReceipter(f.p.exec, f.p.app, &f.p.app.EvmKeeper, f.key, receiptModuleID)
	require.NoError(t, err)
	m, _ := newSequencerMetricsReader(t)
	f.accepter = sequencer.NewAccepter(r, f.rep, sequencer.NewReceiptSource(32), m)
	return f
}

func (f *proofWalPath) start(t *testing.T) {
	t.Helper()
	exec, replay := replayExecutor(t, f.backup)
	f.backup.exec = exec
	ctx, cancel := context.WithCancel(t.Context())
	f.cfg.WAL, f.cfg.Executor = f.sw, replay
	var err error
	f.sb, err = sequencer.NewStandby(ctx, f.cfg)
	require.NoError(t, err)
	l, auth := listenAuthenticated(t)
	f.auth = auth
	f.address = l.Addr().String()
	served := make(chan error, 1)
	sb := f.sb
	go func() { served <- sb.Serve(l) }()
	stopped := false
	f.stop = func() {
		if stopped {
			return
		}
		stopped = true
		cancel()
		require.NoError(t, l.Close())
		select {
		case err := <-served:
			require.ErrorIs(t, err, net.ErrClosed)
		case <-time.After(replicationTimeout):
			t.Fatal("standby listener did not stop")
		}
	}
}

func (f *proofWalPath) promotionKey(t *testing.T) *sequencer.ReceiptKey {
	t.Helper()
	seed := f.cfg.ReceiptKey.Seed()
	key, err := loadKey(t, seed, []verify.SequencerAuthorization{{
		SequencerID: [32]byte{0: 0x93}, PublicKey: publicOf(seed), FirstBatchNumber: 1, LastBatchNumber: 1000,
	}}, openHeight)
	require.NoError(t, err)
	return key
}

func proofWalReplay(t *testing.T, w *wal.WAL, h *harness) wal.ReplayResult {
	t.Helper()
	_, replay := replayExecutor(t, h)
	result, err := w.Replay(replay)
	require.NoError(t, err)
	return result
}

func TestProofSequencerWal(t *testing.T) {
	admissionChainID(t, proofWalChainID)
	t.Run("signed receipt survives restart and failover", func(t *testing.T) {
		sender := utils.NewSigner()
		f := newProofWalPath(t, sender)
		tx, raw := signEVM(t, sender, proofWalChainID, 0, sender.EvmAddress, big.NewInt(3))
		send, err := f.accepter.Accept(raw)
		require.NoError(t, err)
		verified, err := verify.ReceiptSignature(send.Receipt, f.key.PublicKey())
		require.NoError(t, err)
		require.Equal(t, startSeq+1, verified.Receipt.GlobalSequence)
		require.Equal(t, startSeq+1, f.sb.LastSeq())
		require.NoError(t, f.sb.Err())
		pending, ok := f.backup.exec.PendingTx(tx.Hash())
		require.True(t, ok)
		require.Equal(t, raw, pending.TxBytes)
		before := driverEntries(t, f.dir)
		require.Len(t, before, 3)
		accepted := before[2].(*wal.Accept)
		require.Equal(t, raw, accepted.Tx)
		require.Equal(t, send.Receipt, accepted.Receipt)
		primary := proofWalReplay(t, f.pw, f.p)
		require.Equal(t, accepted, primary.LastAccept)
		require.Equal(t, 1, primary.Accepts)
		require.Equal(t, uint64(1), primary.Lease.Epoch)
		balance := new(big.Int).Set(f.backup.weiBalance(f.backup.openCtx(), sender.EvmAddress))

		// Drop the network service and executor and close the actual store.
		f.rep.Close()
		f.stop()
		f.sw.Close()
		f.sw, err = wal.OpenWAL(f.dir, wal.DefaultConfig())
		require.NoError(t, err)
		f.start(t)
		require.Equal(t, accepted.Seq, f.sb.LastSeq())
		require.Equal(t, uint64(1), f.backup.app.EvmKeeper.GetNonce(f.backup.openCtx(), sender.EvmAddress))
		requireWei(t, balance, f.backup.weiBalance(f.backup.openCtx(), sender.EvmAddress))
		reopened, ok := f.backup.exec.PendingTx(tx.Hash())
		require.True(t, ok)
		require.Equal(t, pending, reopened)

		// The dialer connects the retained real producer records to the reopened
		// consumer. Re-delivery must not execute or append the accepted tx twice.
		r := f.rep
		ack, err := r.Drain(t.Context())
		require.NoError(t, err)
		require.Equal(t, sequencer.AckOK, ack.Status)
		require.Equal(t, accepted.Seq, ack.LastSeq)
		require.Equal(t, uint64(0), ack.GapFrom)
		require.Equal(t, uint64(0), ack.GapTo)
		require.Equal(t, before, driverEntries(t, f.dir))
		require.Equal(t, uint64(1), f.backup.app.EvmKeeper.GetNonce(f.backup.openCtx(), sender.EvmAddress))
		// A later expiry is a real renewal, not a duplicate. Retrying the entire
		// retained window after it must neither append nor rewind the lease.
		renewAt := f.holder.Expiry()
		require.NoError(t, f.holder.RenewAt(renewAt))
		renewed := driverEntries(t, f.dir)
		require.Len(t, renewed, len(before)+1)
		require.Equal(t, &wal.Lease{
			Epoch: 1, KeyID: testKeyID, ExpiryMs: uint64(renewAt.Add(3 * time.Minute).UnixMilli()),
		}, renewed[len(renewed)-1])
		r.Close()
		_, err = r.Drain(t.Context())
		require.NoError(t, err)
		require.Equal(t, renewed, driverEntries(t, f.dir))
		_, err = f.accepter.Accept(raw)
		require.Error(t, err, "a repeated producer transaction must be refused")
		require.Equal(t, accepted.Seq, f.sb.LastSeq())

		before = driverEntries(t, f.dir)
		require.ErrorIs(t, f.sb.TakeLease(nil), wal.ErrMalformed)
		require.NoError(t, f.sb.Err())
		require.Equal(t, before, driverEntries(t, f.dir))
		result, err := sequencer.Failover(f.sb, f.key, openHeight, uint64(time.Now().Add(4*time.Minute).UnixMilli()))
		require.ErrorContains(t, err, "differs from standby key")
		require.Equal(t, sequencer.FailoverResult{}, result)
		require.Equal(t, before, driverEntries(t, f.dir))
		key := f.promotionKey(t)
		_, err = verify.ReceiptSignature(send.Receipt, key.PublicKey())
		require.Error(t, err, "the primary receipt must not verify under the standby signer")
		result, err = sequencer.Failover(f.sb, key, openHeight, uint64(time.Now().Add(4*time.Minute).UnixMilli()))
		require.NoError(t, err)
		require.Equal(t, uint64(2), result.Lease.Epoch)
		require.Equal(t, accepted.Seq, result.LastSeq)
		require.Equal(t, openHeight, result.OpenHeight)
		require.ErrorIs(t, f.sb.TakeLease(result.Lease), sequencer.ErrStaleLease)
		ack, err = r.Drain(t.Context())
		require.ErrorIs(t, err, sequencer.ErrFenced)
		require.Equal(t, sequencer.Ack{}, ack)
		require.Equal(t, accepted.Seq, f.sb.LastSeq())

		// Reopen the promoted store before its lease and receipt are consumed by
		// the production replayer and recovery-order selector.
		r.Close()
		f.stop()
		f.sw.Close()
		f.sw, err = wal.OpenWAL(f.dir, wal.DefaultConfig())
		require.NoError(t, err)
		recovered := proofWalReplay(t, f.sw, f.backup)
		require.Equal(t, result.Lease, recovered.Lease)
		require.Equal(t, accepted, recovered.LastLogged)
		require.Equal(t, accepted, recovered.LastAccept)
		require.Equal(t, 1, recovered.Accepts)
		order, err := sequencer.RecoveryOrder(primary, recovered)
		require.NoError(t, err)
		require.Equal(t, 1, order)
	})

	t.Run("planned handover fences admission and resumes the next seq", func(t *testing.T) {
		sender := utils.NewSigner()
		f := newProofWalPath(t, sender)
		_, raw := signEVM(t, sender, proofWalChainID, 0, sender.EvmAddress, big.NewInt(4))
		_, err := f.accepter.Accept(raw)
		require.NoError(t, err)
		key := f.promotionKey(t)
		cfg := sequencer.HandoverConfig{
			Primary: f.p.exec, Replicator: f.rep, Standby: f.sb, Key: f.key,
			OpenHeight: openHeight, ExpiryMs: uint64(time.Now().Add(4 * time.Minute).UnixMilli()),
		}
		before := driverEntries(t, f.dir)
		result, err := sequencer.Handover(t.Context(), cfg)
		require.ErrorContains(t, err, "differs from standby key")
		require.Equal(t, sequencer.HandoverResult{}, result)
		require.Equal(t, before, driverEntries(t, f.dir))
		cfg.Key = key
		result, err = sequencer.Handover(t.Context(), cfg)
		require.NoError(t, err)
		require.Equal(t, startSeq+1, result.LastSeq)
		require.Equal(t, uint64(2), result.Lease.Epoch)
		require.Equal(t, uint64(1), f.rep.Epoch())
		_, next := signEVM(t, sender, proofWalChainID, 1, sender.EvmAddress, big.NewInt(5))
		old, err := f.p.exec.Accept(next)
		require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
		require.Nil(t, old)
		r, err := sequencer.NewReceipter(f.backup.exec, f.backup.app, &f.backup.app.EvmKeeper, key, receiptModuleID)
		require.NoError(t, err)
		acc, receipt, err := r.Accept(next)
		require.NoError(t, err)
		require.Equal(t, result.LastSeq+1, acc.Seq)
		verified, err := verify.ReceiptSignature(receipt, key.PublicKey())
		require.NoError(t, err)
		require.Equal(t, acc.Seq, verified.Receipt.GlobalSequence)
		retry, err := sequencer.Handover(t.Context(), cfg)
		require.ErrorIs(t, err, sequencer.ErrFenced)
		require.Equal(t, sequencer.HandoverResult{}, retry)
		require.Equal(t, result.Lease, driverEntries(t, f.dir)[len(before)])
	})

	t.Run("recovery refuses receipts from divergent real producers", func(t *testing.T) {
		sender := utils.NewSigner()
		recipient := utils.NewSigner().EvmAddress
		first, second := newProofWalPath(t, sender), newProofWalPath(t, sender)
		for i, f := range []*proofWalPath{first, second} {
			_, raw := signEVM(t, sender, proofWalChainID, 0, recipient, big.NewInt(int64(i+1)))
			_, err := f.accepter.Accept(raw)
			require.NoError(t, err)
		}
		a, b := proofWalReplay(t, first.pw, first.p), proofWalReplay(t, second.pw, second.p)
		require.Equal(t, a.LastLogged.Seq, b.LastLogged.Seq)
		require.NotEqual(t, a.LastLogged.Receipt, b.LastLogged.Receipt)
		_, err := sequencer.RecoveryOrder(a, b)
		require.ErrorIs(t, err, sequencer.ErrDivergentLogs)
		order, err := sequencer.RecoveryOrder(a, a)
		require.NoError(t, err)
		require.Zero(t, order)
	})
}
