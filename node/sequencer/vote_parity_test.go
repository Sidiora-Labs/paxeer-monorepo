package sequencer_test

import (
	"context"
	"math"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	distrtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func beginBlock(t *testing.T, exec sequencer.Executor, height, timestampMs int64) sdk.Context {
	t.Helper()
	require.NoError(t, exec.Open(height, timestampMs))
	ctx, err := exec.Snapshot()
	require.NoError(t, err)
	return ctx
}

func TestExecutorBeginBlockDistributesSameRewardsAsProcessBlock(t *testing.T) {
	previousOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previousOCC })
	genesis := time.Unix(1_800_000_000, 0).UTC()
	proposerPub := ed25519.GenPrivKey().PubKey()
	absentPub := ed25519.GenPrivKey().PubKey()
	w := app.NewTestWrapper(t, genesis, proposerPub, false, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
		ba.SetConcurrencyWorkers(1)
	})
	a := w.App
	proposer, absent := sdk.ValAddress(proposerPub.Address()), sdk.ValAddress(absentPub.Address())
	staking := teststaking.NewHelper(t, w.Ctx, a.StakingKeeper)
	staking.Denom = a.StakingKeeper.GetParams(w.Ctx).BondDenom
	stake := a.StakingKeeper.TokensFromConsensusPower(w.Ctx, 100)
	additionalStake := stake.Sub(sdk.NewInt(100))
	w.FundAcc(sdk.AccAddress(proposer), sdk.NewCoins(sdk.NewCoin(staking.Denom, additionalStake)))
	staking.Delegate(sdk.AccAddress(proposer), proposer, additionalStake)
	w.FundAcc(sdk.AccAddress(absent), sdk.NewCoins(sdk.NewCoin(staking.Denom, stake)))
	staking.CreateValidator(absent, absentPub, stake, true)
	a.DistrKeeper.SetParams(w.Ctx, distrtypes.Params{
		CommunityTax:        sdk.NewDecWithPrec(2, 2),
		BaseProposerReward:  sdk.NewDecWithPrec(1, 2),
		BonusProposerReward: sdk.NewDecWithPrec(4, 2),
		WithdrawAddrEnabled: true,
	})
	const rewardDenom = "reward"
	fees := sdk.NewCoins(sdk.NewInt64Coin(rewardDenom, 1_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(w.Ctx, minttypes.ModuleName, fees))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToModule(w.Ctx, minttypes.ModuleName, authtypes.FeeCollectorName, fees))
	storedVotes := []abci.VoteInfo{
		{Validator: abci.Validator{Address: proposerPub.Address().Bytes(), Power: 100}, SignedLastBlock: true},
		{Validator: abci.Validator{Address: absentPub.Address().Bytes(), Power: 100}, SignedLastBlock: false},
	}
	require.NoError(t, a.SequencerKeeper.StoreVotes(w.Ctx, committedTip, storedVotes))
	public := publicOf(testKeySeed)
	require.NoError(t, a.SequencerKeeper.SetReceiptKeys(w.Ctx, seqkeeper.ReceiptKeys{{
		KeyId: testKeyID[:], PublicKey: public[:], FirstHeight: 1, LastHeight: math.MaxUint64,
	}}))
	a.UpgradeKeeper.SetDone(w.Ctx.WithBlockHeight(committedTip), app.MergeUpgrade)
	require.NoError(t, a.SequencerKeeper.SetActivationHeight(w.Ctx, openHeight))
	_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: testChainID, Height: committedTip, Time: genesis, ProposerAddress: proposerPub.Address().Bytes()},
	})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, committedTip, a.LastBlockHeight())
	committed := a.GetCheckCtx()
	require.True(t, a.Sequenced(committed, openHeight))
	for _, validator := range []sdk.ValAddress{proposer, absent} {
		require.Equal(t, int64(100), a.StakingKeeper.GetLastValidatorPower(committed, validator))
		require.True(t, a.DistrKeeper.GetValidatorOutstandingRewards(committed, validator).Rewards.AmountOf(rewardDenom).IsZero())
	}
	votes, err := a.SequencerKeeper.StoredVotes(committed, committedTip)
	require.NoError(t, err)
	require.Equal(t, storedVotes, votes)
	feeCollector := a.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	require.Equal(t, fees[0], a.BankKeeper.GetBalance(committed, feeCollector, rewardDenom))

	exec, err := sequencer.NewExecutor(a, sequencer.Options{ChainID: testChainID, ProposerAddress: proposerPub.Address().Bytes()})
	require.NoError(t, err)
	timestampMs := genesis.Add(time.Second).UnixMilli()
	opened := beginBlock(t, exec, openHeight, timestampMs)
	assertDistribution := func(ctx sdk.Context) {
		t.Helper()
		for _, want := range []struct {
			validator   sdk.ValAddress
			outstanding int64
			commission  int64
			shared      int64
		}{
			{proposer, 505_000, 25_250, 479_750},
			{absent, 475_000, 23_750, 451_250},
		} {
			require.Equal(t, sdk.NewDec(want.outstanding), a.DistrKeeper.GetValidatorOutstandingRewards(ctx, want.validator).Rewards.AmountOf(rewardDenom))
			require.Equal(t, sdk.NewDec(want.commission), a.DistrKeeper.GetValidatorAccumulatedCommission(ctx, want.validator).Commission.AmountOf(rewardDenom))
			require.Equal(t, sdk.NewDec(want.shared), a.DistrKeeper.GetValidatorCurrentRewards(ctx, want.validator).Rewards.AmountOf(rewardDenom))
		}
		require.Equal(t, sdk.NewDec(20_000), a.DistrKeeper.GetFeePool(ctx).CommunityPool.AmountOf(rewardDenom))
		require.Equal(t, sdk.ConsAddress(proposerPub.Address()), a.DistrKeeper.GetPreviousProposerConsAddr(ctx))
		require.True(t, a.BankKeeper.GetBalance(ctx, feeCollector, rewardDenom).IsZero())
		require.Equal(t, fees[0], a.BankKeeper.GetBalance(ctx, a.AccountKeeper.GetModuleAddress(distrtypes.ModuleName), rewardDenom))
	}
	assertDistribution(opened)
	batch, err := exec.Seal(openHeight)
	require.NoError(t, err)
	sealed, _, err := sequencer.SealBatch(a, batch, signSealBatch(t, sequencer.BatchFields(batch), testKeySeed, testKeyID))
	require.NoError(t, err)
	assertDistribution(sealed.Context())

	decidedLastCommit := abci.CommitInfo{Votes: append([]abci.VoteInfo(nil), storedVotes...)}
	decidedLastCommit.Votes[1].SignedLastBlock = true
	replay := sdk.NewContext(a.CommitMultiStore().CacheMultiStore(), opened.BlockHeader(), false)
	replay = replay.WithConsensusParams(a.GetConsensusParams(replay))
	events, results, end, err := a.ProcessBlock(replay, sealed.Txs, &app.BlockProcessRequest{
		Height: openHeight, Time: time.UnixMilli(timestampMs).UTC(),
	}, decidedLastCommit, false, nil)
	require.NoError(t, err)
	assertDistribution(replay)
	require.Equal(t, sealed.Results, results)
	require.Equal(t, sealed.Events, events)
	require.Equal(t, sealed.EndBlock, end)
	key := a.GetKey(distrtypes.StoreKey)
	require.NotNil(t, key)
	utils.CompareStores(t, key, opened.KVStore(key), replay.KVStore(key), t.Name())
	utils.CompareStores(t, key, sealed.Context().KVStore(key), replay.KVStore(key), t.Name())
	votes, err = a.SequencerKeeper.StoredVotes(replay, openHeight)
	require.NoError(t, err)
	require.Equal(t, decidedLastCommit.Votes, votes)
	votes, err = a.SequencerKeeper.StoredVotes(replay, committedTip)
	require.NoError(t, err)
	require.Equal(t, storedVotes, votes)
	require.Equal(t, fees[0], a.BankKeeper.GetBalance(committed, feeCollector, rewardDenom))
}
