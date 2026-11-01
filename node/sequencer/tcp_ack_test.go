package sequencer_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestReceiptOverTCPWaitsForStandbyAck(t *testing.T) {
	previousOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previousOCC })
	sender := utils.NewSigner()
	primaryApp := newStandbyRoleApp(t, sender)
	standbyApp := newStandbyRoleApp(t, sender)
	fixtures := []*app.TestWrapper{primaryApp, standbyApp}
	leaders := make([][32]byte, len(fixtures))
	for i, fixture := range fixtures {
		info, err := fixture.App.UpgradeKeeper.GetUpgradeInfoPath()
		require.NoError(t, err)
		consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
		require.NoError(t, os.MkdirAll(filepath.Dir(consensus.KeyFile()), 0o700))
		pv, err := privval.GenFilePV(consensus.KeyFile(), consensus.StateFile(), "")
		require.NoError(t, err)
		require.NoError(t, pv.Save())
		pub, err := pv.GetPubKey(t.Context())
		require.NoError(t, err)
		leaders[i] = [32]byte(pub.Bytes())
	}
	standbySeed := bytes.Repeat([]byte{0x67}, ed25519.SeedSize)
	primaryPublic, standbyPublic := publicOf(testKeySeed), publicOf(standbySeed)
	for _, fixture := range fixtures {
		a := fixture.App
		require.NoError(t, a.SequencerKeeper.InitGenesis(fixture.Ctx, seqtypes.GenesisState{
			Leaders: leaders,
			ReceiptKeys: []seqtypes.ReceiptKey{
				{KeyId: testKeyID, PublicKey: primaryPublic, FirstHeight: 1, LastHeight: 1000},
				{KeyId: [32]byte{0: 0x68}, PublicKey: standbyPublic, FirstHeight: 1, LastHeight: 1000},
			},
			LastSeq: startSeq, LastSeqHash: startSeqHash,
		}))
		_, err := a.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{ChainID: testChainID, Height: committedTip},
		})
		require.NoError(t, err)
		a.SetDeliverStateToCommit()
		_, err = a.Commit(t.Context())
		require.NoError(t, err)
	}

	standbyListener := listen(t)
	standbyAddress := standbyListener.Addr().String()
	require.NoError(t, standbyListener.Close())
	standbyConfig := sequencer.Config{
		Role: sequencer.RoleStandby, WALDir: filepath.Join(t.TempDir(), "standby-wal"),
		ReceiptKeyFile: writeSeed(t, t.TempDir(), "receipt_key", standbySeed, 0o600),
		StandbyAddress: standbyAddress,
	}
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	standby, err := sequencer.StartStandby(ctx, standbyConfig, standbyApp.App)
	require.NoError(t, err)
	t.Cleanup(func() {
		cancel()
		require.Eventually(t, func() bool { return standby.Err() != nil }, replicationTimeout, time.Millisecond)
	})

	relay := listen(t)
	t.Cleanup(func() { _ = relay.Close() })
	held := make(chan [33]byte, 1)
	release := make(chan struct{})
	relayDone := make(chan error, 1)
	bound := sequencer.ReplicatorConfig{ReplicationTimeout: replicationTimeout}
	go func() {
		relayDone <- relayTCPAck(ctx, relay, standbyAddress, bound.ReplicationTimeout, held, release)
	}()
	t.Cleanup(func() {
		cancel()
		_ = relay.Close()
		select {
		case <-relayDone:
		case <-time.After(bound.ReplicationTimeout):
			t.Error("TCP relay did not stop")
		}
	})
	cfg := sequencer.DefaultConfig()
	cfg.Enable = true
	cfg.WALDir = filepath.Join(t.TempDir(), "primary-wal")
	cfg.ReceiptKeyFile = writeSeed(t, t.TempDir(), "receipt_key", testKeySeed, 0o600)
	cfg.StandbyAddress = relay.Addr().String()
	cfg.ReplicationTimeout = bound.ReplicationTimeout
	cfg.LeaseDuration = 3 * cfg.ReplicationTimeout
	cfg.NonceQueueMaxPerSender, cfg.NonceQueueMaxTotal = 1, 2
	primary, err := sequencer.NewNode(cfg, primaryApp.App)
	require.NoError(t, err)
	t.Cleanup(primary.WAL.Close)
	t.Cleanup(primary.Replicator.Close)
	openMs := time.Now().Add(time.Minute).UnixMilli()
	require.NoError(t, primary.Executor.Open(openHeight, openMs))
	baseFee, err := primary.FeeFloor.BaseFeePerGas()
	require.NoError(t, err)
	fee, overflow := uint256.FromBig(baseFee.TruncateInt().BigInt())
	require.False(t, overflow)
	ack, err := primary.Replicator.Release(ctx, &wal.Open{
		Height: uint64(openHeight), OpenTimeMs: uint64(openMs), BaseFee: fee,
	})
	require.NoError(t, err)
	require.Equal(t, sequencer.Ack{Status: sequencer.AckOK, LastSeq: startSeq}, ack)

	tx, txBytes := transfer(t, sender, 0, sender.EvmAddress, big.NewInt(1))
	type outcome struct {
		receipt  []byte
		err      error
		finished time.Time
	}
	returned := make(chan outcome, 1)
	started := time.Now()
	deadline := time.NewTimer(bound.ReplicationTimeout)
	defer deadline.Stop()
	go func() {
		receipt, err := primary.Accepter.Accept(txBytes)
		returned <- outcome{receipt, err, time.Now()}
	}()
	select {
	case reply := <-held:
		require.Equal(t, byte(sequencer.AckOK), reply[0], "standby failure: %v", standby.Err())
		require.Equal(t, startSeq+1, binary.BigEndian.Uint64(reply[1:9]))
	case result := <-returned:
		t.Fatalf("Accept returned before the TCP ack was released: %x, %v", result.receipt, result.err)
	case <-deadline.C:
		t.Fatal("standby did not acknowledge within ReplicationTimeout")
	}
	require.Equal(t, startSeq+1, standby.LastSeq())
	require.NoError(t, standby.Err())
	send, err := primary.ReceiptSource.SequencerSend(tx.Hash())
	require.NoError(t, err)
	require.Nil(t, send, "receipt must remain unpublished while its ack is held")
	delay := time.NewTimer(bound.ReplicationTimeout / 10)
	defer delay.Stop()
	select {
	case result := <-returned:
		t.Fatalf("Accept returned while its TCP ack was held: %x, %v", result.receipt, result.err)
	case <-delay.C:
	case <-deadline.C:
		t.Fatal("ReplicationTimeout elapsed while holding the ack")
	}
	releasedAt := time.Now()
	close(release)
	select {
	case result := <-returned:
		require.NoError(t, result.err)
		require.False(t, result.finished.Before(releasedAt), "Accept returned before ack release")
		require.Less(t, result.finished.Sub(started), bound.ReplicationTimeout)
		verified, err := verify.ReceiptSignature(result.receipt, primaryPublic)
		require.NoError(t, err)
		require.Equal(t, startSeq+1, verified.Receipt.GlobalSequence)
		send, err := primary.ReceiptSource.SequencerSend(tx.Hash())
		require.NoError(t, err)
		require.NotNil(t, send)
		require.Equal(t, result.receipt, send.Receipt)
	case <-deadline.C:
		t.Fatal("Accept did not return within ReplicationTimeout after the ack was released")
	}
}

func relayTCPAck(ctx context.Context, listener net.Listener, address string, timeout time.Duration, held chan<- [33]byte, release <-chan struct{}) error {
	primary, err := listener.Accept()
	if err != nil {
		return err
	}
	defer primary.Close()
	standby, err := sequencer.DialStandby(address, timeout)(ctx)
	if err != nil {
		return err
	}
	defer standby.Close()
	stop := context.AfterFunc(ctx, func() {
		_ = primary.Close()
		_ = standby.Close()
	})
	defer stop()
	forwardDone := make(chan struct{})
	go func() {
		defer close(forwardDone)
		_, _ = io.Copy(standby, primary)
		_ = standby.Close()
	}()
	defer func() {
		_ = primary.Close()
		<-forwardDone
	}()
	for group := 0; ; group++ {
		var ack [33]byte
		if _, err := io.ReadFull(standby, ack[:]); err != nil {
			return err
		}
		if group == 1 {
			held <- ack
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if n, err := primary.Write(ack[:]); err != nil {
			return err
		} else if n != len(ack) {
			return io.ErrShortWrite
		}
	}
}
