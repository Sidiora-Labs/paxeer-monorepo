package keeper_test

import (
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestDistributionEmitsNoLegacyTelemetry(t *testing.T) {
	reader := oteltest.Install()

	t.Run("begin blocker", func(t *testing.T) {
		app := paxapp.Setup(t, false, false, false)
		consAddr := sdk.ConsAddress(valConsPk1.Address())
		ctx := app.BaseApp.NewContext(false, tmproto.Header{Height: 1, ProposerAddress: consAddr})
		addr := paxapp.AddTestAddrs(app, ctx, 1, sdk.NewInt(1000))[0]
		tstaking := teststaking.NewHelper(t, ctx, app.StakingKeeper)
		tstaking.CreateValidator(sdk.ValAddress(addr), valConsPk1, sdk.NewInt(100), true)
		app.DistrKeeper.SetPreviousProposerConsAddr(ctx, consAddr)
		fees := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 100))
		require.NoError(t, apptesting.FundModuleAccount(app.BankKeeper, ctx, authtypes.FeeCollectorName, fees))
		ctx = ctx.WithBlockHeight(2)
		votes := []abci.VoteInfo{{
			Validator:       abci.Validator{Address: consAddr, Power: 100},
			SignedLastBlock: true,
		}}
		sink := oteltest.InstallLegacySink(t)
		require.Empty(t, sink.Keys())

		before := reader.HistogramCount("paxcosmos_x_distribution", "distribution_begin_blocker_duration")
		distribution.BeginBlocker(ctx, votes, app.DistrKeeper)
		after := reader.HistogramCount("paxcosmos_x_distribution", "distribution_begin_blocker_duration")
		require.Equal(t, uint64(1), after-before)
		require.Equal(t, consAddr, app.DistrKeeper.GetPreviousProposerConsAddr(ctx))
		feeCollector := app.AccountKeeper.GetModuleAccount(ctx, authtypes.FeeCollectorName)
		require.True(t, app.BankKeeper.GetAllBalances(ctx, feeCollector.GetAddress()).IsZero())
		require.False(t, app.DistrKeeper.GetValidatorOutstandingRewards(ctx, sdk.ValAddress(addr)).Rewards.IsZero())
		require.NotContains(t, sink.Keys(), "oteltest.begin_blocker")
	})

	t.Run("withdrawals", func(t *testing.T) {
		app := paxapp.Setup(t, false, false, false)
		ctx := app.BaseApp.NewContext(false, tmproto.Header{Height: 1})
		addr := paxapp.AddTestAddrs(app, ctx, 1, sdk.NewInt(1000))[0]
		valAddr := sdk.ValAddress(addr)
		tstaking := teststaking.NewHelper(t, ctx, app.StakingKeeper)
		tstaking.Commission = stakingtypes.NewCommissionRates(sdk.NewDecWithPrec(5, 1), sdk.NewDecWithPrec(5, 1), sdk.ZeroDec())
		tstaking.CreateValidator(valAddr, valConsPk1, sdk.NewInt(100), true)
		val := app.StakingKeeper.Validator(ctx, valAddr)
		require.NotNil(t, val)
		server := keeper.NewMsgServerImpl(app.DistrKeeper)
		sink := oteltest.InstallLegacySink(t)
		require.Empty(t, sink.Keys())

		for _, payout := range []int64{5, 13} {
			ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
			coins := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 2*payout))
			require.NoError(t, apptesting.FundModuleAccount(app.BankKeeper, ctx, types.ModuleName, coins))
			app.DistrKeeper.AllocateTokensToValidator(ctx, val, sdk.NewDecCoinsFromCoins(coins...))

			rewardBefore, _ := withdrawRewardAmount(reader)
			balanceBefore := app.BankKeeper.GetBalance(ctx, addr, sdk.DefaultBondDenom).Amount
			response, err := server.WithdrawDelegatorReward(sdk.WrapSDKContext(ctx), &types.MsgWithdrawDelegatorReward{
				DelegatorAddress: addr.String(),
				ValidatorAddress: valAddr.String(),
			})
			require.NoError(t, err)
			require.NotNil(t, response)
			rewardAfter, found := withdrawRewardAmount(reader)
			require.True(t, found)
			require.Equal(t, payout, rewardAfter)
			require.Equal(t, sdk.NewInt(payout), app.BankKeeper.GetBalance(ctx, addr, sdk.DefaultBondDenom).Amount.Sub(balanceBefore))
			require.NotContains(t, sink.Keys(), "oteltest.tx.msg.withdraw_reward")

			commissionBefore, _ := withdrawCommissionAmount(reader)
			balanceBefore = app.BankKeeper.GetBalance(ctx, addr, sdk.DefaultBondDenom).Amount
			commissionResponse, err := server.WithdrawValidatorCommission(sdk.WrapSDKContext(ctx), &types.MsgWithdrawValidatorCommission{
				ValidatorAddress: valAddr.String(),
			})
			require.NoError(t, err)
			require.NotNil(t, commissionResponse)
			commissionAfter, found := withdrawCommissionAmount(reader)
			require.True(t, found)
			require.Equal(t, payout, commissionAfter)
			require.Equal(t, sdk.NewInt(payout), app.BankKeeper.GetBalance(ctx, addr, sdk.DefaultBondDenom).Amount.Sub(balanceBefore))
			require.NotContains(t, sink.Keys(), "oteltest.tx.msg.withdraw_commission")

			if payout == 13 {
				require.Equal(t, int64(8), rewardAfter-rewardBefore)
				require.Equal(t, int64(8), commissionAfter-commissionBefore)
			}
		}
	})
}
