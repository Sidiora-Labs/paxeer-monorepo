package sequencer_test

import (
	"context"
	"math"
	"math/big"
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
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func TestNewNodeAssemblesSequencerComponents(t *testing.T) {
	for _, mode := range []sequencer.Mode{sequencer.ModeSequential, sequencer.ModeMicroBatch} {
		t.Run(mode.String(), func(t *testing.T) {
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
			cfg.Mode = mode
			cfg.WALDir = filepath.Join(t.TempDir(), "wal")
			cfg.ReceiptKeyFile = writeSeed(t, t.TempDir(), "receipt_key", testKeySeed, 0o600)
			cfg.StandbyAddress = "127.0.0.1:1"
			cfg.ReplicationTimeout = replicationTimeout
			cfg.LeaseDuration = 3 * replicationTimeout
			cfg.NonceQueueMaxPerSender = 1
			cfg.NonceQueueMaxTotal = 2
			cfg.MicroBatchWindow = time.Millisecond
			cfg.MicroBatchMaxCount = 2
			n, err := sequencer.NewNode(cfg, h.app)
			require.NoError(t, err)
			require.NotNil(t, n)
			t.Cleanup(n.WAL.Close)
			t.Cleanup(n.Replicator.Close)
			require.Equal(t, cfg, n.Config)
			require.Same(t, h.app, n.App)
			require.NotNil(t, n.WAL)
			require.NotNil(t, n.Replicator)
			require.NotNil(t, n.Executor)
			require.NotNil(t, n.FeeFloor)
			require.NotNil(t, n.Receipter)
			require.NotNil(t, n.Accepter)
			require.NotNil(t, n.ConsensusSource)
			require.NotNil(t, n.Driver)
			require.NotNil(t, n.ReceiptSource)
			require.NotNil(t, n.ReceiptLogFeed)
			require.NotNil(t, n.LogFeedPublisher)
			require.NotNil(t, n.NonceAdmission)
			require.NotNil(t, n.Metrics)
			require.Nil(t, n.Standby)
			require.Equal(t, mode, n.Executor.Mode())
			require.Same(t, n.FeeFloor, n.Executor)
			require.Equal(t, publicOf(testKeySeed), n.ReceiptKey.PublicKey())
			locked, err := wal.OpenWAL(cfg.WALDir, wal.DefaultConfig())
			if locked != nil {
				locked.Close()
			}
			require.Error(t, err, "the assembled node owns the WAL lock")
			require.Nil(t, locked)
			_, err = n.Accepter.Accept([]byte{1})
			require.ErrorIs(t, err, sequencer.ErrLayerNotOpen)
			require.Nil(t, h.app.SequencerSource())
			_, err = n.Executor.Snapshot()
			require.ErrorIs(t, err, sequencer.ErrLayerNotOpen)
			require.NoError(t, n.Executor.Open(openHeight, h.genesis.Add(time.Second).UnixMilli()))
			baseFee, err := n.FeeFloor.BaseFeePerGas()
			require.NoError(t, err)
			ctx, err := n.Executor.Snapshot()
			require.NoError(t, err)
			require.True(t, baseFee.Equal(h.app.EvmKeeper.GetNextBaseFeePerGas(ctx)))
			_, tx := transfer(t, sender, 0, utils.NewSigner().EvmAddress, big.NewInt(1))
			accepted, receipt, err := n.Receipter.Accept(tx)
			require.NoError(t, err)
			require.Equal(t, sourceLastSeq+1, accepted.Seq)
			require.Equal(t, sourcePrevSeqHash, accepted.PrevSeqHash)
			verified, err := verify.ReceiptSignature(receipt, n.ReceiptKey.PublicKey())
			require.NoError(t, err)
			require.Equal(t, app.EVMReceiptModuleID, verified.Receipt.ModuleID)
			require.Equal(t, accepted.Seq, verified.Receipt.GlobalSequence)
			h.app.SetOpenBlock(n.Executor)
			held, err := n.NonceAdmission.Send(sender.EvmAddress, 2, tx)
			require.NoError(t, err)
			require.True(t, held)
			_, err = n.NonceAdmission.Send(sender.EvmAddress, 3, tx)
			require.ErrorIs(t, err, sequencer.ErrSenderQueueFull)
			expired := n.NonceAdmission.Open(time.Now().Add(cfg.LeaseDuration + time.Second))
			require.Len(t, expired, 1)
			require.Equal(t, uint64(2), expired[0].Nonce)
			batch, err := n.Executor.Seal(openHeight)
			require.NoError(t, err)
			require.Equal(t, [][]byte{tx}, batch.Txs)
			require.Equal(t, sourceLastSeq+1, batch.LastSeq)
			pub, err := pv.GetPubKey(context.Background())
			require.NoError(t, err)
			require.Equal(t, []byte(pub.Address()), batch.Ctx.BlockHeader().ProposerAddress)
			require.False(t, n.ConsensusSource.Sequenced(openHeight))
			require.True(t, n.ConsensusSource.Sequenced(openHeight+1))
		})
	}
}

func newNodeHarness(t *testing.T, sender utils.TestAcct) *harness {
	t.Helper()
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = true })
	genesis := time.Now().UTC()
	tw := app.NewTestWrapper(t, genesis, secp256k1.GenPrivKey().PubKey(), false, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
		ba.SetConcurrencyWorkers(1)
	})
	a := tw.App
	a.EvmKeeper.SetAddressMapping(tw.Ctx, sender.AccountAddress, sender.EvmAddress)
	tw.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(fundingUhpx))))
	resolutionBlockGas(tw.Ctx, a)
	require.NoError(t, a.SequencerKeeper.InitGenesis(tw.Ctx, seqtypes.GenesisState{
		ActivationHeight: uint64(openHeight + 1),
		Leaders:          [][32]byte{[32]byte(label("consensus source leader"))},
		ReceiptKeys: []seqtypes.ReceiptKey{{
			KeyId:       testKeyID,
			PublicKey:   publicOf(testKeySeed),
			FirstHeight: 1,
			LastHeight:  math.MaxUint64,
		}},
		LastSeq:     sourceLastSeq,
		LastSeqHash: sourcePrevSeqHash,
	}))
	_, err := a.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: testChainID, Height: committedTip},
	})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(t.Context())
	require.NoError(t, err)
	return &harness{t: t, app: a, genesis: genesis}
}

func TestNewNodeDisabledReturnsNil(t *testing.T) {
	for _, role := range []sequencer.Role{sequencer.RolePrimary, sequencer.RoleStandby} {
		t.Run(role.String(), func(t *testing.T) {
			cfg := sequencer.DefaultConfig()
			cfg.Role = role
			cfg.WALDir = filepath.Join(t.TempDir(), "unused-wal")
			n, err := sequencer.NewNode(cfg, nil)
			require.NoError(t, err)
			require.Nil(t, n)
			_, err = os.Stat(cfg.WALDir)
			require.True(t, os.IsNotExist(err))
		})
	}
}
