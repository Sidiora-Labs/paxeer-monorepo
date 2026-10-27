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
	// A retransmitted group must be acknowledged without executing it twice.
	ack, err = repl.Release(ctx, open, entry)
	require.NoError(t, err)
	require.Equal(t, sequencer.Ack{Status: sequencer.AckOK, LastSeq: accepted.Seq}, ack)
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
	require.Equal(t, 1, result.Accepts)
	require.Equal(t, entry, result.LastAccept)
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
