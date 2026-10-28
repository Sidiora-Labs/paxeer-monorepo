package sequencer_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
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
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestStartStandbyServesReplication(t *testing.T) {
	wasOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = wasOCC })
	sender := utils.NewSigner()
	primaryApp := newStandbyRoleApp(t, sender)
	backupApp := newStandbyRoleApp(t, sender)
	primary := &harness{t: t, app: primaryApp.App}
	info, err := backupApp.App.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(consensus.KeyFile()), 0o700))
	pv, err := privval.GenFilePV(consensus.KeyFile(), consensus.StateFile(), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	primaryDir := t.TempDir()
	primaryPV, err := privval.GenFilePV(filepath.Join(primaryDir, "key.json"), filepath.Join(primaryDir, "state.json"), "")
	require.NoError(t, err)
	require.NoError(t, primaryPV.Save())
	standbyPub, err := pv.GetPubKey(context.Background())
	require.NoError(t, err)
	primaryPub, err := primaryPV.GetPubKey(context.Background())
	require.NoError(t, err)
	standbySeed := bytes.Repeat([]byte{0x67}, ed25519.SeedSize)
	standbyPublic := publicOf(standbySeed)
	standbyID := [32]byte{0: 0x68}
	primaryPublic := publicOf(testKeySeed)
	for _, fixture := range []*app.TestWrapper{primaryApp, backupApp} {
		a := fixture.App
		require.NoError(t, a.SequencerKeeper.InitGenesis(fixture.Ctx, seqtypes.GenesisState{
			Leaders: [][32]byte{[32]byte(primaryPub.Bytes()), [32]byte(standbyPub.Bytes())},
			ReceiptKeys: []seqtypes.ReceiptKey{
				{KeyId: testKeyID, PublicKey: primaryPublic, FirstHeight: 1, LastHeight: 1000},
				{KeyId: standbyID, PublicKey: standbyPublic, FirstHeight: 1, LastHeight: 1000},
			},
			LastSeq: startSeq, LastSeqHash: startSeqHash,
		}))
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(fixture.Ctx, openHeight))
		a.UpgradeKeeper.SetDone(fixture.Ctx.WithBlockHeight(openHeight-1), app.MergeUpgrade)
		_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{ChainID: testChainID, Height: committedTip},
		})
		require.NoError(t, err)
		a.SetDeliverStateToCommit()
		_, err = a.Commit(context.Background())
		require.NoError(t, err)
	}
	primary.exec, err = sequencer.NewExecutor(primary.app, sequencer.Options{
		ChainID: testChainID, ProposerAddress: primaryPub.Address(), LastSeq: startSeq, LastSeqHash: startSeqHash,
	})
	require.NoError(t, err)
	primary.proposer = primaryPub.Address()
	key, err := sequencer.LoadReceiptKey(writeSeed(t, primaryDir, "receipt_key", testKeySeed, 0o600),
		filepath.Join(primaryDir, "key.json"), []verify.SequencerAuthorization{{
			SequencerID: testKeyID, PublicKey: primaryPublic, FirstBatchNumber: 1, LastBatchNumber: 1000,
		}}, openHeight)
	require.NoError(t, err)
	receipter, err := sequencer.NewReceipter(primary.exec, primary.app, &primary.app.EvmKeeper, key, app.EVMReceiptModuleID)
	require.NoError(t, err)
	l := listen(t)
	address := l.Addr().String()
	require.NoError(t, l.Close())
	cfg := sequencer.Config{
		Role: sequencer.RoleStandby, WALDir: filepath.Join(t.TempDir(), "standby-wal"),
		ReceiptKeyFile: writeSeed(t, t.TempDir(), "receipt_key", standbySeed, 0o600), StandbyAddress: address,
		ReplicationTimeout: 200 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s, err := sequencer.StartStandby(ctx, cfg, backupApp.App)
	require.NoError(t, err)
	t.Cleanup(func() {
		cancel()
		require.Eventually(t, func() bool { return s.Err() != nil }, replicationTimeout, time.Millisecond)
	})
	gotPub, err := s.ConsensusKey().GetPubKey(ctx)
	require.NoError(t, err)
	require.Equal(t, standbyPub, gotPub)
	require.Equal(t, standbyPublic[:], []byte(s.ReceiptKey().Public().(ed25519.PublicKey)))
	w, err := wal.OpenWAL(filepath.Join(t.TempDir(), "primary-wal"), wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(w.Close)
	repl, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{
		WAL: w, Dial: sequencer.DialStandby(address, replicationTimeout), ReplicationTimeout: replicationTimeout,
	})
	require.NoError(t, err)
	t.Cleanup(repl.Close)
	openMs := time.Now().Add(time.Minute).UnixMilli()
	require.NoError(t, primary.exec.Open(openHeight, openMs))
	_, tx := transfer(t, sender, 0, sender.EvmAddress, big.NewInt(1))
	accepted, receipt, err := receipter.Accept(tx)
	require.NoError(t, err)
	open := &wal.Open{Height: uint64(openHeight), OpenTimeMs: uint64(openMs), BaseFee: uint256.NewInt(0)}
	entry := &wal.Accept{Seq: accepted.Seq, Height: uint64(accepted.Height), Tx: tx, Receipt: receipt}
	ack, err := repl.Release(ctx, open, entry)
	require.NoError(t, err, "standby failure: %v", s.Err())
	require.Equal(t, sequencer.Ack{Status: sequencer.AckOK, LastSeq: accepted.Seq}, ack)
	require.Equal(t, accepted.Seq, s.LastSeq())
	require.NoError(t, s.Err())
	pending, err := backupApp.App.RPCPendingSnapshot()
	require.NoError(t, err)
	baseFee, ok := pending.(interface{ BaseFeePerGas() (sdk.Dec, error) })
	require.True(t, ok)
	fee, err := baseFee.BaseFeePerGas()
	require.NoError(t, err)
	primaryCtx, err := primary.exec.Snapshot()
	require.NoError(t, err)
	require.True(t, fee.Equal(primary.app.EvmKeeper.GetNextBaseFeePerGas(primaryCtx)))
	// A retransmitted group must be acknowledged without executing it twice.
	ack, err = repl.Release(ctx, open, entry)
	require.NoError(t, err)
	require.Equal(t, sequencer.Ack{Status: sequencer.AckOK, LastSeq: accepted.Seq}, ack)
	source, err := sequencer.NewConsensusSource(primary.app, primary.exec, key, sequencer.WithConsensusReplicator(repl))
	require.NoError(t, err)
	primary.app.SetSequencerSource(source)
	var lastCommitted *wal.Committed
	lastEntry := entry
	for height := openHeight; height < openHeight+2; height++ {
		block, err := source.Seal(ctx, height)
		require.NoError(t, err)
		txs := append([][]byte{block.Batch}, block.Txs.ToSliceOfBytes()...)
		commit := func(a *app.App) {
			t.Helper()
			response, err := a.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{
				Header: &tmproto.Header{ChainID: testChainID, Height: height, Time: block.OpenTime, ProposerAddress: primary.proposer},
				Txs:    txs,
			})
			require.NoError(t, err)
			require.Len(t, response.TxResults, len(txs))
			for _, result := range response.TxResults {
				require.Zero(t, result.Code, result.Log)
			}
			a.SetDeliverStateToCommit()
			_, err = a.Commit(ctx)
			require.NoError(t, err)
			require.Equal(t, height, a.LastBlockHeight())
		}
		commit(primary.app)
		lastCommitted = &wal.Committed{
			Height: uint64(height), LastSeq: accepted.Seq,
			AppHash: [32]byte(primary.app.CommitMultiStore().LastCommitID().Hash),
		}
		ack, err = repl.Release(ctx, lastCommitted)
		require.NoError(t, err)
		require.Equal(t, sequencer.Ack{Status: sequencer.AckOK, LastSeq: accepted.Seq}, ack)
		require.Equal(t, height-1, backupApp.App.LastBlockHeight(), "COMMITTED must not commit the standby app")
		require.NoError(t, s.Err())
		openMs++
		require.NoError(t, primary.exec.Open(height+1, openMs))
		nextOpen := &wal.Open{Height: uint64(height + 1), OpenTimeMs: uint64(openMs), BaseFee: uint256.NewInt(0)}
		conn, err := net.DialTimeout("tcp", address, replicationTimeout)
		require.NoError(t, err)
		started := time.Now()
		gap := standbyLeaseExchange(t, conn, nextOpen)
		require.NoError(t, conn.Close())
		require.Equal(t, sequencer.Ack{Status: sequencer.AckGap, LastSeq: accepted.Seq, GapFrom: accepted.Seq + 1, GapTo: accepted.Seq}, gap)
		require.GreaterOrEqual(t, time.Since(started), cfg.ReplicationTimeout)
		require.NoError(t, s.Err(), "a delayed consensus commit must not poison replication")
		require.Equal(t, height-1, backupApp.App.LastBlockHeight())
		type releaseResult struct {
			ack sequencer.Ack
			err error
		}
		released := make(chan releaseResult, 1)
		go func() { ack, err := repl.Release(ctx, nextOpen); released <- releaseResult{ack, err} }()
		select {
		case result := <-released:
			t.Fatalf("open acknowledged before consensus commit: %+v", result)
		case <-time.After(20 * time.Millisecond):
		}
		commit(backupApp.App)
		select {
		case result := <-released:
			require.NoError(t, result.err, "standby failure: %v", s.Err())
			require.Equal(t, sequencer.Ack{Status: sequencer.AckOK, LastSeq: accepted.Seq}, result.ack)
		case <-time.After(replicationTimeout):
			t.Fatal("standby did not acknowledge after consensus committed")
		}
		require.NoError(t, s.Err())
		if height == openHeight {
			_, nextTx := transfer(t, sender, 1, sender.EvmAddress, big.NewInt(1))
			accepted, receipt, err = receipter.Accept(nextTx)
			require.NoError(t, err)
			require.Zero(t, accepted.Result.Code, accepted.Result.Log)
			lastEntry = &wal.Accept{Seq: accepted.Seq, Height: uint64(accepted.Height), Tx: nextTx, Receipt: receipt}
			ack, err = repl.Release(ctx, lastEntry)
			require.NoError(t, err)
			require.Equal(t, sequencer.Ack{Status: sequencer.AckOK, LastSeq: accepted.Seq}, ack)
			require.Equal(t, accepted.Seq, s.LastSeq())
			require.NoError(t, s.Err())
		}
	}
	repl.Close()
	cancel()
	require.Eventually(t, func() bool { return s.Err() != nil }, replicationTimeout, time.Millisecond)
	require.ErrorIs(t, s.Err(), context.Canceled)
	persisted, err := wal.OpenWAL(cfg.WALDir, wal.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(persisted.Close)
	_, replayer := replayExecutor(t, primary)
	result, err := persisted.Replay(replayer)
	require.NoError(t, err)
	require.Equal(t, lastCommitted, result.From)
	require.Zero(t, result.Accepts, "committed transactions must not replay")
	require.Nil(t, result.LastAccept)
	require.Equal(t, lastEntry, result.LastLogged)
	entries := driverEntries(t, cfg.WALDir)
	require.Len(t, entries, 9, "timed-out opens must not persist")
	require.Equal(t, open, entries[0])
	require.Equal(t, entry, entries[1])
	require.Equal(t, lastEntry, entries[5])
	require.Equal(t, lastCommitted, entries[7])
	require.Equal(t, uint64(openHeight+2), entries[8].(*wal.Open).Height)
	var dialer net.Dialer
	conn, err := dialer.DialContext(context.Background(), "tcp", address)
	if conn != nil {
		_ = conn.Close()
	}
	require.Error(t, err)
}

func newStandbyRoleApp(t *testing.T, sender utils.TestAcct) *app.TestWrapper {
	t.Helper()
	fixture := app.NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a := fixture.App
	admissionParams(1_000_000, 1_000_000, 200_000)(fixture.Ctx, a)
	a.EvmKeeper.SetAddressMapping(fixture.Ctx, sender.AccountAddress, sender.EvmAddress)
	fixture.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(fundingUhpx))))
	return fixture
}

func TestStartStandbyRefusesPrimaryRole(t *testing.T) {
	for _, role := range []sequencer.Role{sequencer.RolePrimary, sequencer.Role(255)} {
		t.Run(role.String(), func(t *testing.T) {
			s, err := sequencer.StartStandby(context.Background(), sequencer.Config{Role: role}, nil)
			require.ErrorIs(t, err, sequencer.ErrConfigValue)
			require.Nil(t, s)
		})
	}
}
