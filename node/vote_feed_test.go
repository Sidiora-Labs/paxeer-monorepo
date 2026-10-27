package app_test

import (
	"bytes"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/stretchr/testify/require"
)

func newVoteFeedWrapper(t *testing.T) (*app.TestWrapper, sdk.ConsAddress, sdk.ValAddress) {
	t.Helper()
	w := newSequencerWrapper(t, time.Unix(1_700_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	validators := w.App.StakingKeeper.GetAllValidators(w.Ctx)
	require.Len(t, validators, 1)
	consAddr, err := validators[0].GetConsAddr()
	require.NoError(t, err)
	w.Ctx = w.Ctx.WithBlockHeight(2).WithBlockTime(w.Ctx.BlockTime().Add(2 * time.Second))
	header := w.Ctx.BlockHeader()
	header.ProposerAddress = consAddr
	w.Ctx = w.Ctx.WithBlockHeader(header)
	w.App.DistrKeeper.SetPreviousProposerConsAddr(w.Ctx, consAddr)
	params := w.App.DistrKeeper.GetParams(w.Ctx)
	params.CommunityTax = sdk.NewDecWithPrec(2, 1)
	w.App.DistrKeeper.SetParams(w.Ctx, params)
	fees := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000))
	require.NoError(t, w.App.BankKeeper.MintCoins(w.Ctx, "mint", fees))
	require.NoError(t, w.App.BankKeeper.SendCoinsFromModuleToModule(w.Ctx, "mint", authtypes.FeeCollectorName, fees))
	return w, consAddr, validators[0].GetOperator()
}

func TestVoteFeedUsesStoredVotesWhenSequenced(t *testing.T) {
	w, consAddr, valAddr := newVoteFeedWrapper(t)
	height := w.Ctx.BlockHeight()
	require.NoError(t, w.App.SequencerKeeper.SetActivationHeight(w.Ctx, height))
	previous := []abci.VoteInfo{{Validator: abci.Validator{Address: consAddr, Power: 100}, SignedLastBlock: false}}
	current := []abci.VoteInfo{{Validator: abci.Validator{Address: consAddr, Power: 17}, SignedLastBlock: true}}

	for _, tc := range []struct {
		name  string
		votes []abci.VoteInfo
	}{
		{"executor nil votes", nil},
		{"validator votes", current},
		{"validator empty votes", []abci.VoteInfo{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := w.Ctx.CacheContext()
			require.True(t, w.App.Sequenced(ctx, height))
			require.NoError(t, w.App.SequencerKeeper.StoreVotes(ctx, height-1, previous))
			// A nil executor argument must preserve votes already present at this height.
			require.NoError(t, w.App.SequencerKeeper.StoreVotes(ctx, height, previous))
			beforeRewards := w.App.DistrKeeper.GetValidatorOutstandingRewards(ctx, valAddr).Rewards
			beforePool := w.App.DistrKeeper.GetFeePool(ctx).CommunityPool
			beforeInfo, found := w.App.SlashingKeeper.GetValidatorSigningInfo(ctx, consAddr)
			require.True(t, found)

			w.App.BeginBlock(ctx, height, tc.votes, nil, false)

			rewards := w.App.DistrKeeper.GetValidatorOutstandingRewards(ctx, valAddr).Rewards.Sub(beforeRewards)
			require.Equal(t, sdk.NewDec(8_000), rewards.AmountOf("uhpx"))
			pool := w.App.DistrKeeper.GetFeePool(ctx).CommunityPool.Sub(beforePool)
			require.Equal(t, sdk.NewDec(2_000), pool.AmountOf("uhpx"))
			info, found := w.App.SlashingKeeper.GetValidatorSigningInfo(ctx, consAddr)
			require.True(t, found)
			require.Equal(t, beforeInfo.IndexOffset+1, info.IndexOffset)
			require.Equal(t, beforeInfo.MissedBlocksCounter+1, info.MissedBlocksCounter)
			stored, err := w.App.SequencerKeeper.StoredVotes(ctx, height)
			require.NoError(t, err)
			expected := tc.votes
			if tc.votes == nil {
				expected = previous
			}
			require.Equal(t, expected, stored)
			stored, err = w.App.SequencerKeeper.StoredVotes(ctx, height-1)
			require.NoError(t, err)
			require.Equal(t, previous, stored)
		})
	}

	t.Run("corrupt stored votes halt", func(t *testing.T) {
		ctx, _ := w.Ctx.CacheContext()
		ctx.KVStore(w.App.GetKey(seqtypes.StoreKey)).Set(seqtypes.StoredVotesKeyAt(height-1), []byte{0xff})
		require.PanicsWithError(t, seqkeeper.ErrCorruptStore.Wrapf("stored votes at height %d: stored votes: truncated count", height-1).Error(), func() {
			w.App.BeginBlock(ctx, height, nil, nil, false)
		})
	})
	t.Run("invalid incoming votes halt", func(t *testing.T) {
		ctx, _ := w.Ctx.CacheContext()
		invalid := []abci.VoteInfo{{Validator: abci.Validator{Address: bytes.Repeat([]byte{1}, 256), Power: 100}}}
		require.PanicsWithError(t, "stored votes: vote 0 address length 256 exceeds 255", func() {
			w.App.BeginBlock(ctx, height, invalid, nil, false)
		})
	})
}

func TestVoteFeedUnchangedBelowActivation(t *testing.T) {
	w, consAddr, valAddr := newVoteFeedWrapper(t)
	height := w.Ctx.BlockHeight()
	require.NoError(t, w.App.SequencerKeeper.SetActivationHeight(w.Ctx, height+1))
	storedVotes := []abci.VoteInfo{{Validator: abci.Validator{Address: consAddr, Power: 100}, SignedLastBlock: false}}
	for _, tc := range []struct {
		name   string
		votes  []abci.VoteInfo
		reward int64
		missed int64
	}{
		{"signed argument", []abci.VoteInfo{{Validator: abci.Validator{Address: consAddr, Power: 7}, SignedLastBlock: true}}, 8_000, 0},
		{"unsigned argument", storedVotes, 8_000, 1},
		{"nil argument", nil, 0, 0},
		{"empty argument", []abci.VoteInfo{}, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := w.Ctx.CacheContext()
			require.False(t, w.App.Sequenced(ctx, height))
			require.NoError(t, w.App.SequencerKeeper.StoreVotes(ctx, height-1, storedVotes))
			require.NoError(t, w.App.SequencerKeeper.StoreVotes(ctx, height, storedVotes))
			beforeRewards := w.App.DistrKeeper.GetValidatorOutstandingRewards(ctx, valAddr).Rewards
			beforeInfo, found := w.App.SlashingKeeper.GetValidatorSigningInfo(ctx, consAddr)
			require.True(t, found)

			w.App.BeginBlock(ctx, height, tc.votes, nil, false)

			rewards := w.App.DistrKeeper.GetValidatorOutstandingRewards(ctx, valAddr).Rewards.Sub(beforeRewards)
			require.Equal(t, sdk.NewDec(tc.reward), rewards.AmountOf("uhpx"))
			info, found := w.App.SlashingKeeper.GetValidatorSigningInfo(ctx, consAddr)
			require.True(t, found)
			processed := int64(0)
			if len(tc.votes) > 0 {
				processed = 1
			}
			require.Equal(t, beforeInfo.IndexOffset+processed, info.IndexOffset)
			require.Equal(t, beforeInfo.MissedBlocksCounter+tc.missed, info.MissedBlocksCounter)
			for _, storedHeight := range []int64{height - 1, height} {
				stored, err := w.App.SequencerKeeper.StoredVotes(ctx, storedHeight)
				require.NoError(t, err)
				require.Equal(t, storedVotes, stored)
			}
		})
	}
}
