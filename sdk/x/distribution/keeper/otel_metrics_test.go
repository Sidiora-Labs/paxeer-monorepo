package keeper_test

import (
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func withdrawRewardAmount(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt("paxcosmos_x_distribution_keeper", "last_withdraw_reward_amount")
}

func withdrawCommissionAmount(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt("paxcosmos_x_distribution_keeper", "last_withdraw_commission_amount")
}

func TestWithdrawRecordsRewardAndCommissionAmounts(t *testing.T) {
	reader := oteltest.Install()
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

		if payout == 13 {
			require.Equal(t, int64(8), rewardAfter-rewardBefore)
			require.Equal(t, int64(8), commissionAfter-commissionBefore)
		}
	}
}
