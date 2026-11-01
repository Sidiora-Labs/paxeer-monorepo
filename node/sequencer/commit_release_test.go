package sequencer_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdked25519 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	protoio "github.com/gogo/protobuf/io"
	"github.com/stretchr/testify/require"
)

func TestDriverReleasesNextOpenAcrossStandbyCommit(t *testing.T) {
	wasOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = wasOCC })
	ctx := context.Background()
	dir := t.TempDir()
	newPV := func(name string) *privval.FilePV {
		t.Helper()
		pv, err := privval.GenFilePV(filepath.Join(dir, name+".json"), filepath.Join(dir, name+"-state.json"), "")
		require.NoError(t, err)
		require.NoError(t, pv.Save())
		return pv
	}
	primaryPV, standbyPV := newPV("primary"), newPV("standby")
	primaryPub, err := primaryPV.GetPubKey(ctx)
	require.NoError(t, err)
	standbyPub, err := standbyPV.GetPubKey(ctx)
	require.NoError(t, err)
	committee := tmtypes.NewValidatorSet([]*tmtypes.Validator{
		tmtypes.NewValidator(primaryPub, 3), tmtypes.NewValidator(standbyPub, 1),
	})
	require.Equal(t, int64(4), committee.TotalVotingPower())
	primaryIndex, primaryValidator, found := committee.GetByAddress(primaryPub.Address())
	require.True(t, found)
	require.Greater(t, primaryValidator.VotingPower*3, committee.TotalVotingPower()*2)
	standbySeed := bytes.Repeat([]byte{0x67}, ed25519.SeedSize)
	standbyPublic, primaryPublic := publicOf(standbySeed), publicOf(testKeySeed)
	sender := utils.NewSigner()
	at := time.Now().UTC().Truncate(time.Millisecond)
	fixture := app.NewTestWrapper(t, at, &sdked25519.PubKey{Key: primaryPub.Bytes()}, false, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
	})
	primary := fixture.App
	t.Cleanup(func() { require.NoError(t, primary.Close()) })
	stakingParams := primary.StakingKeeper.GetParams(fixture.Ctx)
	bondDenom := stakingParams.BondDenom
	primaryAddr := sdk.ValAddress(primaryPub.Address())
	primaryStake := sdk.TokensFromConsensusPower(3, sdk.DefaultPowerReduction).SubRaw(100)
	fixture.FundAcc(sdk.AccAddress(primaryAddr), sdk.NewCoins(sdk.NewCoin(bondDenom, primaryStake)))
	stakingHandler := staking.NewHandler(primary.StakingKeeper)
	_, err = stakingHandler(fixture.Ctx, stakingtypes.NewMsgDelegate(sdk.AccAddress(primaryAddr), primaryAddr, sdk.NewCoin(bondDenom, primaryStake)))
	require.NoError(t, err)
	standbyAddr := sdk.ValAddress(standbyPub.Address())
	standbyStake := sdk.NewCoin(bondDenom, sdk.TokensFromConsensusPower(1, sdk.DefaultPowerReduction))
	fixture.FundAcc(sdk.AccAddress(standbyAddr), sdk.NewCoins(standbyStake))
	createStandby, err := stakingtypes.NewMsgCreateValidator(standbyAddr, &sdked25519.PubKey{Key: standbyPub.Bytes()},
		standbyStake, stakingtypes.Description{},
		stakingtypes.NewCommissionRates(stakingParams.MinCommissionRate, stakingParams.MinCommissionRate, sdk.ZeroDec()), sdk.OneInt())
	require.NoError(t, err)
	_, err = stakingHandler(fixture.Ctx, createStandby)
	require.NoError(t, err)
	resolutionBlockGas(fixture.Ctx, primary)
	primary.EvmKeeper.SetAddressMapping(fixture.Ctx, sender.AccountAddress, sender.EvmAddress)
	fixture.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(fundingUhpx))))
	require.NoError(t, primary.SequencerKeeper.InitGenesis(fixture.Ctx, seqtypes.GenesisState{
		Leaders: [][32]byte{[32]byte(primaryPub.Bytes()), [32]byte(standbyPub.Bytes())},
		ReceiptKeys: []seqtypes.ReceiptKey{
			{KeyId: testKeyID, PublicKey: primaryPublic, FirstHeight: 1, LastHeight: 1000},
			{KeyId: [32]byte{0x68}, PublicKey: standbyPublic, FirstHeight: 1, LastHeight: 1000},
		},
		LastSeq: startSeq, LastSeqHash: startSeqHash,
	}))
	require.NoError(t, primary.SequencerKeeper.SetActivationHeight(fixture.Ctx, openHeight))
	primary.UpgradeKeeper.SetDone(fixture.Ctx.WithBlockHeight(committedTip), app.MergeUpgrade)
	votes := []abci.VoteInfo{
		{Validator: abci.Validator{Address: primaryPub.Address(), Power: 3}, SignedLastBlock: true},
		{Validator: abci.Validator{Address: standbyPub.Address(), Power: 1}, SignedLastBlock: false},
	}
	require.NoError(t, primary.StoreVotes(fixture.Ctx, committedTip, votes))
	_, err = primary.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: testChainID, Height: committedTip, Time: at, ProposerAddress: primaryPub.Address()},
	})
	require.NoError(t, err)
	primary.SetDeliverStateToCommit()
	_, err = primary.Commit(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(3), primary.StakingKeeper.GetLastValidatorPower(primary.GetCheckCtx(), primaryAddr))
	require.Equal(t, int64(1), primary.StakingKeeper.GetLastValidatorPower(primary.GetCheckCtx(), standbyAddr))
	var snapshot bytes.Buffer
	require.NoError(t, primary.CommitMultiStore().Snapshot(uint64(committedTip), protoio.NewDelimitedWriter(&snapshot)))
	backup := app.Setup(t, true, false, false, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
		ba.SetStoreLoader(func(ms sdk.CommitMultiStore) error {
			if err := ms.LoadLatestVersion(); err != nil {
				return err
			}
			_, err := ms.Restore(uint64(committedTip), 0, protoio.NewDelimitedReader(bytes.NewReader(snapshot.Bytes()), 64<<20))
			return err
		})
	})
	t.Cleanup(func() { require.NoError(t, backup.Close()) })
	backup.ChainID = testChainID
	require.Equal(t, primary.LastCommitID().Hash, backup.LastCommitID().Hash)
	installPV := func(a *app.App, name string) {
		t.Helper()
		info, err := a.UpgradeKeeper.GetUpgradeInfoPath()
		require.NoError(t, err)
		consensus := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
		for target, source := range map[string]string{
			consensus.KeyFile():   filepath.Join(dir, name+".json"),
			consensus.StateFile(): filepath.Join(dir, name+"-state.json"),
		} {
			data, err := os.ReadFile(source)
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o700))
			require.NoError(t, os.WriteFile(target, data, 0o600))
		}
	}
	installPV(primary, "primary")
	installPV(backup, "standby")
	l := listen(t)
	address := l.Addr().String()
	require.NoError(t, l.Close())
	const timeout = 200 * time.Millisecond
	standbyCtx, cancel := context.WithCancel(ctx)
	standbyWAL := filepath.Join(dir, "standby-wal")
	s, err := sequencer.StartStandby(standbyCtx, sequencer.Config{
		Role: sequencer.RoleStandby, WALDir: standbyWAL,
		ReceiptKeyFile: writeSeed(t, dir, "standby-receipt", standbySeed, 0o600),
		StandbyAddress: address, ReplicationTimeout: timeout,
	}, backup)
	require.NoError(t, err)
	t.Cleanup(func() {
		cancel()
		require.Eventually(t, func() bool { return s.Err() != nil }, replicationTimeout, time.Millisecond)
	})
	cfg := sequencer.DefaultConfig()
	cfg.Enable = true
	cfg.WALDir = filepath.Join(dir, "primary-wal")
	cfg.ReceiptKeyFile = writeSeed(t, dir, "primary-receipt", testKeySeed, 0o600)
	cfg.StandbyAddress = address
	cfg.ReplicationTimeout = timeout
	cfg.LeaseDuration = 3 * timeout
	cfg.NonceQueueMaxPerSender, cfg.NonceQueueMaxTotal = 4, 8
	n, err := sequencer.NewNode(cfg, primary)
	require.NoError(t, err)
	t.Cleanup(func() { n.Replicator.Close(); n.WAL.Close() })
	require.NoError(t, n.Start(ctx))
	var commits []int64
	primary.SetCommitListener(func(height int64, hash []byte) error {
		require.Equal(t, height-1, backup.LastBlockHeight())
		err := n.Driver.OnCommit(height, hash)
		require.NoError(t, err, "OPEN for height %d must be acknowledged before the standby commits height %d", height+1, height)
		require.Equal(t, height-1, backup.LastBlockHeight())
		commits = append(commits, height)
		return err
	})
	for i := uint64(0); i < 2; i++ {
		height := openHeight + int64(i)
		_, tx := transfer(t, sender, i, utils.NewSigner().EvmAddress, big.NewInt(1))
		receipt, err := n.Accepter.Accept(tx)
		require.NoError(t, err)
		require.NotEmpty(t, receipt)
		require.Equal(t, startSeq+i+1, s.LastSeq())
		block, err := n.ConsensusSource.Seal(ctx, height)
		require.NoError(t, err)
		hash := sha256.Sum256(block.Batch)
		blockID := tmtypes.BlockID{Hash: hash[:], PartSetHeader: tmtypes.PartSetHeader{Total: 1, Hash: hash[:]}}
		vote := &tmtypes.Vote{Type: tmproto.PrecommitType, Height: height, BlockID: blockID,
			Timestamp: block.OpenTime, ValidatorAddress: primaryPub.Address(), ValidatorIndex: primaryIndex}
		pv := vote.ToProto()
		require.NoError(t, primaryPV.SignVote(ctx, testChainID, pv))
		signed, err := tmtypes.VoteFromProto(pv)
		require.NoError(t, err)
		voteSet := tmtypes.NewVoteSet(testChainID, height, 0, tmproto.PrecommitType, committee)
		added, err := voteSet.AddVote(signed)
		require.NoError(t, err)
		require.True(t, added)
		require.True(t, voteSet.HasTwoThirdsMajority())
		commit := voteSet.MakeCommit()
		require.NoError(t, committee.VerifyCommitLight(testChainID, blockID, height, commit))
		require.Equal(t, tmtypes.BlockIDFlagAbsent, commit.Signatures[1-primaryIndex].BlockIDFlag)
		request := &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{ChainID: testChainID, Height: height, Time: block.OpenTime,
				ProposerAddress: primaryPub.Address(), AppHash: bytes.Clone(primary.LastCommitID().Hash)},
			Txs: append([][]byte{block.Batch}, block.Txs.ToSliceOfBytes()...), DecidedLastCommit: abci.CommitInfo{Votes: votes},
		}
		for _, a := range []*app.App{primary, backup} {
			response, err := a.FinalizeBlock(ctx, request)
			require.NoError(t, err)
			require.Len(t, response.TxResults, 2)
			for _, result := range response.TxResults {
				require.Zero(t, result.Code, result.Log)
			}
			_, err = a.Commit(ctx)
			require.NoError(t, err)
			require.Equal(t, height, a.LastBlockHeight())
			if a == primary {
				require.Equal(t, height-1, backup.LastBlockHeight())
				entries := driverEntries(t, standbyWAL)
				opened, ok := entries[len(entries)-1].(*wal.Open)
				require.True(t, ok)
				require.Equal(t, uint64(height+1), opened.Height)
				require.Equal(t, entries, driverEntries(t, cfg.WALDir))
			}
		}
		require.Equal(t, primary.LastCommitID().Hash, backup.LastCommitID().Hash, "committed state diverged at height %d", height)
		require.NoError(t, s.Err())
	}
	require.Equal(t, []int64{openHeight, openHeight + 1}, commits)
}
