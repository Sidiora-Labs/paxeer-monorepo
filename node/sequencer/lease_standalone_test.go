package sequencer_test

import (
	"encoding/json"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func leaseStandaloneReplicator(t *testing.T, onSync func(time.Duration)) (*sequencer.Replicator, *wal.WAL, string) {
	t.Helper()
	dir := t.TempDir()
	w, err := wal.OpenWAL(dir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(w.Close)
	rep, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{WAL: w, Standalone: true, OnWALSync: onSync})
	require.NoError(t, err)
	t.Cleanup(rep.Close)
	return rep, w, dir
}

func leaseLogger(t *testing.T) (*slog.Logger, func() []map[string]any) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "lease")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	return slog.New(slog.NewJSONHandler(f, nil)), func() []map[string]any {
		data, err := os.ReadFile(f.Name())
		require.NoError(t, err)
		var lines []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line == "" {
				continue
			}
			var record map[string]any
			require.NoError(t, json.Unmarshal([]byte(line), &record))
			lines = append(lines, record)
		}
		return lines
	}
}

func TestLeaseStandaloneRenewalAfterExpiry(t *testing.T) {
	for _, lateness := range []time.Duration{0, time.Second} {
		t.Run(lateness.String(), func(t *testing.T) {
			rep, w, dir := leaseStandaloneReplicator(t, nil)
			logger, lines := leaseLogger(t)
			const duration = 10 * time.Second
			keyID := [32]byte{1, 2, 3}
			holder, err := sequencer.NewLeaseHolder(sequencer.LeaseHolderConfig{Replicator: rep, KeyID: keyID, LeaseDuration: duration, Logger: logger})
			require.NoError(t, err)
			require.NoError(t, holder.Start(t.Context()))
			epoch, expiry := holder.Epoch(), holder.Expiry()
			now := expiry.Add(lateness)
			require.NoError(t, holder.RenewAt(now))
			require.Equal(t, epoch, holder.Epoch())
			require.Equal(t, epoch, rep.Epoch())
			require.Equal(t, now.Add(duration), holder.Expiry())
			rep.Close()
			w.Close()
			w, err = wal.OpenWAL(dir, wal.DefaultConfig())
			require.NoError(t, err)
			t.Cleanup(w.Close)
			h := newHarness(t)
			replay, err := w.Replay(appReplayer{exec: h.exec})
			require.NoError(t, err)
			require.Equal(t, &wal.Lease{Epoch: epoch, KeyID: keyID, ExpiryMs: uint64(now.Add(duration).UnixMilli())}, replay.Lease)
			records := lines()
			require.Len(t, records, 1)
			require.Equal(t, "WARN", records[0]["level"])
			require.Equal(t, float64(epoch), records[0]["epoch"])
			require.Equal(t, float64(lateness), records[0]["lateness"])
		})
	}
}

func TestLeaseStandbyRenewalAfterExpiryWritesNothing(t *testing.T) {
	listener := listen(t)
	p := newPrimary(t, listener.Addr().String(), replicationTimeout)
	sb := newStandby(t, p)
	sb.serve(t, listener)
	holder, err := sequencer.NewLeaseHolder(sequencer.LeaseHolderConfig{Replicator: p.repl, KeyID: testKeyID, LeaseDuration: 10 * time.Second})
	require.NoError(t, err)
	require.NoError(t, holder.Start(t.Context()))
	epoch, expiry := holder.Epoch(), holder.Expiry()
	primaryBefore := leaseEntries(t, p.wal)
	standbyBefore, err := os.ReadFile(filepath.Join(sb.dir, "head.seg"))
	require.NoError(t, err)
	for _, now := range []time.Time{expiry, expiry.Add(time.Second)} {
		require.ErrorIs(t, holder.RenewAt(now), sequencer.ErrLeaseFenced)
		require.Equal(t, epoch, holder.Epoch())
		require.Equal(t, expiry, holder.Expiry())
	}
	primaryAfter := leaseEntries(t, p.wal)
	standbyAfter, err := os.ReadFile(filepath.Join(sb.dir, "head.seg"))
	require.NoError(t, err)
	require.Equal(t, primaryBefore, primaryAfter)
	require.Equal(t, standbyBefore, standbyAfter)
}

func TestLeaseStandaloneStartAfterSlowWrite(t *testing.T) {
	logger, lines := leaseLogger(t)
	const duration = 100 * time.Millisecond
	syncs := 0
	rep, w, _ := leaseStandaloneReplicator(t, func(time.Duration) {
		syncs++
		if syncs == 1 {
			time.Sleep(2 * duration)
		}
	})
	holder, err := sequencer.NewLeaseHolder(sequencer.LeaseHolderConfig{Replicator: rep, KeyID: testKeyID, LeaseDuration: duration, Logger: logger})
	require.NoError(t, err)
	require.NoError(t, holder.Start(t.Context()))
	require.Equal(t, 2, syncs)
	require.Equal(t, uint64(1), holder.Epoch())
	require.True(t, holder.Expiry().After(time.Now()))
	h := newHarness(t)
	replay, err := w.Replay(appReplayer{exec: h.exec})
	require.NoError(t, err)
	require.Equal(t, uint64(1), replay.Lease.Epoch)
	require.Equal(t, holder.Expiry().UnixMilli(), int64(replay.Lease.ExpiryMs))
	records := lines()
	require.Len(t, records, 1)
	require.Equal(t, "WARN", records[0]["level"])
	require.Greater(t, records[0]["lateness"].(float64), float64(0))
}

func TestLeaseExpiredAdmission(t *testing.T) {
	for _, standalone := range []bool{true, false} {
		name := "standby"
		if standalone {
			name = "standalone"
		}
		t.Run(name, func(t *testing.T) {
			sender := utils.NewSigner()
			var h *harness
			var rep *sequencer.Replicator
			var accepter *sequencer.Accepter
			if standalone {
				h = newAccepterAppHarness(t, sender)
				rep, _, _ = leaseStandaloneReplicator(t, nil)
				key, err := loadKey(t, testKeySeed, []verify.SequencerAuthorization{{SequencerID: testKeyID, PublicKey: publicOf(testKeySeed), FirstBatchNumber: 1, LastBatchNumber: 1000}}, openHeight)
				require.NoError(t, err)
				r, err := sequencer.NewReceipter(h.exec, h.app, &h.app.EvmKeeper, key, receiptModuleID)
				require.NoError(t, err)
				metrics, _ := newSequencerMetricsReader(t)
				accepter = sequencer.NewAccepter(r, rep, sequencer.NewReceiptSource(32), metrics)
				require.NoError(t, h.exec.Open(openHeight, time.Now().Add(time.Minute).UnixMilli()))
			} else {
				f := newAccepterHarness(t, sender)
				f.serve()
				h, rep, accepter = f.h, f.rep, f.a
			}
			logger, lines := leaseLogger(t)
			holder, err := sequencer.NewLeaseHolder(sequencer.LeaseHolderConfig{Replicator: rep, KeyID: testKeyID, LeaseDuration: 200 * time.Millisecond, Logger: logger})
			require.NoError(t, err)
			require.NoError(t, holder.Start(t.Context()))
			sequencer.AttachLeaseForTest(accepter, holder)
			epoch, expiry := holder.Epoch(), holder.Expiry()
			<-time.After(time.Until(expiry) + 10*time.Millisecond)
			_, tx := transfer(t, sender, 0, sender.EvmAddress, big.NewInt(1))
			send, err := accepter.Accept(tx)
			if standalone {
				require.NoError(t, err)
				require.NotEmpty(t, send.Receipt)
				require.Equal(t, epoch, holder.Epoch())
				require.True(t, holder.Expiry().After(expiry))
				require.Equal(t, uint64(1), h.app.EvmKeeper.GetNonce(h.openCtx(), sender.EvmAddress))
				batch, err := h.exec.Seal(openHeight)
				require.NoError(t, err)
				require.Len(t, batch.Txs, 1)
			} else {
				require.ErrorIs(t, err, sequencer.ErrLeaseFenced)
				require.Empty(t, send.Receipt)
				require.ErrorIs(t, h.exec.Open(openHeight+1, time.Now().UnixMilli()), sequencer.ErrLeaseFenced)
				_, err = accepter.Accept(tx)
				require.ErrorIs(t, err, sequencer.ErrReplicationHalted)
			}
			records := lines()
			if standalone {
				require.NotEmpty(t, records)
				for _, record := range records {
					require.Equal(t, "WARN", record["level"])
					require.Equal(t, float64(epoch), record["epoch"])
					require.Greater(t, record["lateness"].(float64), float64(0))
				}
			} else {
				require.Len(t, records, 1)
				require.Equal(t, float64(epoch), records[0]["epoch"])
				require.Equal(t, "ERROR", records[0]["level"])
				require.Equal(t, expiry.Format(time.RFC3339Nano), records[0]["expiry"])
				require.NotEmpty(t, records[0]["time"])
				require.Equal(t, sequencer.ErrLeaseFenced.Error(), records[0]["cause"])
			}
		})
	}
}

func leaseStandaloneNode(t *testing.T, sender utils.TestAcct) *sequencer.Node {
	t.Helper()
	h := newNodeHarness(t, sender)
	info, err := h.app.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	paths := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(paths.KeyFile()), 0700))
	pv, err := privval.GenFilePV(paths.KeyFile(), paths.StateFile(), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	cfg := sequencer.DefaultConfig()
	cfg.Enable = true
	cfg.WALDir = t.TempDir()
	cfg.ReceiptKeyFile = writeSeed(t, t.TempDir(), "receipt_key", testKeySeed, 0600)
	cfg.LeaseDuration = 200 * time.Millisecond
	cfg.NonceQueueMaxPerSender, cfg.NonceQueueMaxTotal = 1, 2
	n, err := sequencer.NewNode(cfg, h.app)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, n.Stop()) })
	return n
}

func TestLeaseStandaloneLoopResumesAfterExpiry(t *testing.T) {
	sender := utils.NewSigner()
	n := leaseStandaloneNode(t, sender)
	logger, lines := leaseLogger(t)
	sequencer.SetLeaseNodeLoggerForTest(n, logger)
	require.NoError(t, n.Start(t.Context()))
	resume := sequencer.PauseLeaseRenewalsForTest(t, n.LeaseHolder)
	epoch, expiry := n.LeaseHolder.Epoch(), n.LeaseHolder.Expiry()
	<-time.After(time.Until(expiry) + 20*time.Millisecond)
	require.Equal(t, expiry, n.LeaseHolder.Expiry())
	resume()
	require.Eventually(t, func() bool { return n.LeaseHolder.Expiry().After(expiry) }, time.Second, time.Millisecond)
	require.Equal(t, epoch, n.LeaseHolder.Epoch())
	batch, err := n.Executor.Seal(openHeight)
	require.NoError(t, err)
	require.NotNil(t, batch)
	require.NoError(t, n.Executor.Open(openHeight, time.Now().Add(time.Second).UnixMilli()))
	_, tx := transfer(t, sender, 0, sender.EvmAddress, big.NewInt(1))
	_, err = n.Accepter.Accept(tx)
	require.NoError(t, err)
	batch, err = n.Executor.Seal(openHeight)
	require.NoError(t, err)
	require.Len(t, batch.Txs, 1)
	require.NoError(t, n.Stop())
	records := lines()
	require.Len(t, records, 1)
	require.Equal(t, "WARN", records[0]["level"])
	require.Equal(t, float64(epoch), records[0]["epoch"])
	require.Greater(t, records[0]["lateness"].(float64), float64(0))
}

func TestLeaseStandaloneRenewalWriteFailure(t *testing.T) {
	rep, w, _ := leaseStandaloneReplicator(t, nil)
	logger, lines := leaseLogger(t)
	holder, err := sequencer.NewLeaseHolder(sequencer.LeaseHolderConfig{Replicator: rep, KeyID: testKeyID, LeaseDuration: time.Second, Logger: logger})
	require.NoError(t, err)
	require.NoError(t, holder.Start(t.Context()))
	epoch, expiry := holder.Epoch(), holder.Expiry()
	w.Close()
	require.ErrorIs(t, holder.RenewAt(expiry.Add(time.Second)), wal.ErrClosed)
	require.Equal(t, epoch, holder.Epoch())
	require.Equal(t, expiry, holder.Expiry())
	require.Empty(t, lines())
}

func TestLeaseStandaloneLoopFencesOnWriteError(t *testing.T) {
	n := leaseStandaloneNode(t, utils.NewSigner())
	logger, lines := leaseLogger(t)
	sequencer.SetLeaseNodeLoggerForTest(n, logger)
	require.NoError(t, n.Start(t.Context()))
	epoch, expiry := n.LeaseHolder.Epoch(), n.LeaseHolder.Expiry()
	n.WAL.Close()
	require.Eventually(t, func() bool {
		return n.Executor.Open(openHeight, time.Now().UnixMilli()) == sequencer.ErrLeaseFenced
	}, time.Second, time.Millisecond)
	require.NoError(t, n.Stop())
	require.ErrorIs(t, n.Executor.Open(openHeight, time.Now().UnixMilli()), sequencer.ErrLeaseFenced)
	records := lines()
	require.Len(t, records, 1)
	require.Equal(t, "ERROR", records[0]["level"])
	require.Equal(t, float64(epoch), records[0]["epoch"])
	require.Equal(t, expiry.Format(time.RFC3339Nano), records[0]["expiry"])
	require.NotEmpty(t, records[0]["time"])
	require.Contains(t, records[0]["cause"], wal.ErrClosed.Error())
}

func leaseEntries(t *testing.T, w *wal.WAL) []wal.Entry {
	t.Helper()
	var entries []wal.Entry
	require.NoError(t, w.Scan(func(e wal.Entry) error {
		entries = append(entries, e)
		return nil
	}))
	return entries
}

func TestLeaseStandaloneRenewalWriteCrossesExpiry(t *testing.T) {
	const duration = 100 * time.Millisecond
	syncs := 0
	rep, _, _ := leaseStandaloneReplicator(t, func(time.Duration) {
		syncs++
		if syncs == 2 {
			time.Sleep(2 * duration)
		}
	})
	logger, lines := leaseLogger(t)
	holder, err := sequencer.NewLeaseHolder(sequencer.LeaseHolderConfig{Replicator: rep, KeyID: testKeyID, LeaseDuration: duration, Logger: logger})
	require.NoError(t, err)
	require.NoError(t, holder.Start(t.Context()))
	epoch, expiry := holder.Epoch(), holder.Expiry()
	now := expiry.Add(-duration / 2)
	require.NoError(t, holder.RenewAt(now))
	require.Equal(t, epoch, holder.Epoch())
	require.Equal(t, now.Add(duration), holder.Expiry())
	require.False(t, time.Now().Before(expiry))
	records := lines()
	require.Len(t, records, 1)
	require.Equal(t, "WARN", records[0]["level"])
	require.Greater(t, records[0]["lateness"].(float64), float64(0))
}
