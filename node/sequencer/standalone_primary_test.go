package sequencer_test

import (
	"context"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestReplicatorStandaloneAcknowledgesAfterWALSync(t *testing.T) {
	sender := utils.NewSigner()
	h := newAccepterAppHarness(t, sender)
	dir := t.TempDir()
	w, err := wal.OpenWAL(dir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(w.Close)
	var syncs int
	r, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: w, Standalone: true,
		OnWALSync: func(d time.Duration) {
			require.GreaterOrEqual(t, d, time.Duration(0))
			syncs++
		},
	})
	require.NoError(t, err)
	t.Cleanup(r.Close)
	require.True(t, r.Standalone())

	_, err = sequencer.NewReplicator(sequencer.ReplicatorConfig{WAL: w})
	require.Error(t, err, "a replicated primary still requires a dialer")
	_, err = sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: w, Standalone: true, Dial: sequencer.DialStandby("127.0.0.1:1", replicationTimeout),
		ReplicationTimeout: replicationTimeout,
	})
	require.Error(t, err, "standalone and a standby dialer are mutually exclusive")

	openMs := time.Now().Add(time.Minute).UnixMilli()
	require.NoError(t, h.exec.Open(openHeight, openMs))
	open := &wal.Open{Height: uint64(openHeight), OpenTimeMs: uint64(openMs), BaseFee: uint256.NewInt(0)}
	var expected []byte
	release := func(maxSeq uint64, entries ...wal.Entry) {
		t.Helper()
		before := syncs
		ack, err := r.Release(context.Background(), entries...)
		require.NoError(t, err)
		require.Equal(t, sequencer.Ack{Status: sequencer.AckOK, LastSeq: maxSeq}, ack)
		require.Equal(t, before+1, syncs, "Release returns after the WAL sync callback")
		for _, entry := range entries {
			frame, err := wal.NewFrame(entry)
			require.NoError(t, err)
			expected = append(expected, frame...)
		}
		stored, err := os.ReadFile(filepath.Join(dir, "head.seg"))
		require.NoError(t, err)
		require.Equal(t, expected, stored, "frames are flushed before Release returns")
		n, err := w.GroupSync()
		require.NoError(t, err)
		require.Zero(t, n, "Release already synced every appended frame")
	}
	release(0, open)
	var accepts []*wal.Accept
	for nonce := uint64(0); nonce < 2; nonce++ {
		_, tx := transfer(t, sender, nonce, utils.NewSigner().EvmAddress, big.NewInt(1))
		accepted, err := h.exec.Accept(tx)
		require.NoError(t, err)
		receipt, err := receiptBytes(accepted)
		require.NoError(t, err)
		accepts = append(accepts, &wal.Accept{Seq: accepted.Seq, Height: uint64(accepted.Height), Tx: tx, Receipt: receipt})
	}
	release(accepts[1].Seq, accepts[0], accepts[1])
	ack, err := r.Release(context.Background(), &wal.Lease{Epoch: 1})
	require.ErrorIs(t, err, sequencer.ErrNoAck)
	require.Equal(t, sequencer.Ack{}, ack)
	require.Zero(t, r.Epoch())
	ack, err = r.Drain(context.Background())
	require.ErrorIs(t, err, sequencer.ErrNoAck)
	require.Equal(t, sequencer.Ack{}, ack)
	require.Equal(t, 2, syncs)

	w.Close()
	w, err = wal.OpenWAL(dir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(w.Close)
	recovered := newAccepterAppHarness(t, sender)
	result, err := w.Replay(appReplayer{exec: recovered.exec})
	require.NoError(t, err)
	require.Equal(t, 2, result.Accepts)
	require.Equal(t, accepts[1], result.LastAccept)
	require.Nil(t, result.Lease)
	require.Nil(t, result.Tail)
}

func TestNodeStartsPrimaryWithoutStandbyAddress(t *testing.T) {
	for _, standalone := range []bool{true, false} {
		name := "standalone"
		if !standalone {
			name = "replicated"
		}
		t.Run(name, func(t *testing.T) {
			sender := utils.NewSigner()
			h := newNodeHarness(t, sender)
			info, err := h.app.UpgradeKeeper.GetUpgradeInfoPath()
			require.NoError(t, err)
			consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
			require.NoError(t, os.MkdirAll(filepath.Dir(consensus.KeyFile()), 0o700))
			pv, err := privval.GenFilePV(consensus.KeyFile(), consensus.StateFile(), "")
			require.NoError(t, err)
			require.NoError(t, pv.Save())
			cfg := sequencer.DefaultConfig()
			cfg.Enable = true
			cfg.WALDir = filepath.Join(t.TempDir(), "wal")
			cfg.ReceiptKeyFile = writeSeed(t, t.TempDir(), "receipt_key", testKeySeed, 0o600)
			cfg.LeaseDuration = 3 * replicationTimeout
			cfg.NonceQueueMaxPerSender, cfg.NonceQueueMaxTotal = 1, 2
			if !standalone {
				l := listen(t)
				cfg.StandbyAddress = l.Addr().String()
				require.NoError(t, l.Close())
				cfg.ReplicationTimeout = replicationTimeout
			}
			n, err := sequencer.NewNode(cfg, h.app)
			require.NoError(t, err)
			require.NotNil(t, n)
			t.Cleanup(n.WAL.Close)
			t.Cleanup(n.Replicator.Close)
			require.Equal(t, standalone, n.Replicator.Standalone())
			require.Nil(t, n.Standby)
			err = n.Start(context.Background())
			if !standalone {
				require.ErrorIs(t, err, sequencer.ErrNoAck, "a configured standby must acknowledge the OPEN")
				return
			}
			require.NoError(t, err)
			_, tx := transfer(t, sender, 0, utils.NewSigner().EvmAddress, big.NewInt(1))
			receipt, err := n.Accepter.Accept(tx)
			require.NoError(t, err)
			verified, err := verify.ReceiptSignature(receipt, n.ReceiptKey.PublicKey())
			require.NoError(t, err)
			require.Equal(t, sourceLastSeq+1, verified.Receipt.GlobalSequence)
			nSynced, err := n.WAL.GroupSync()
			require.NoError(t, err)
			require.Zero(t, nSynced, "the receipt is returned only after the ACCEPT is durable")
		})
	}
}
