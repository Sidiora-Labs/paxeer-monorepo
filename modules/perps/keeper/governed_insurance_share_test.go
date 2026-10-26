package keeper_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestGovernedInsuranceShareSplitsFee(t *testing.T) {
	env := newOrdersEnv(t)
	params, err := env.k.GetParams(env.ctx)
	require.NoError(t, err)
	require.Zero(t, params.InsuranceSharePpm)
	params.InsuranceSharePpm = 500_000
	response, err := keeper.NewMsgServerImpl(env.k).UpdateParams(sdk.WrapSDKContext(env.ctx), &types.MsgUpdateParams{
		Authority: env.k.Authority(), Params: params,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := env.k.GetParams(env.ctx)
	require.NoError(t, err)
	require.Equal(t, params, stored)

	env.fund(t, traderA, 10_000)
	env.fund(t, traderB, 10_000)
	for _, size := range []int64{6_000, 12_000} {
		ask := limitBuy(size, 60_000)
		ask.IsLong = false
		placed, err := env.k.PlaceOrder(env.ctx, traderB, ask)
		require.NoError(t, err)
		require.Empty(t, placed.Fills)
	}

	result, err := env.k.PlaceOrder(env.ctx, traderA, keeper.OrderParams{
		MarketID: 0, IsLong: true, OrderType: uint8(types.OrderTypeMarket),
		TimeInForce: uint8(types.TimeInForceIOC), SizeUsd: e18(18_000), Leverage: e18(20),
	})
	require.NoError(t, err)
	require.Len(t, result.Fills, 2)
	require.Equal(t, types.OrderStatusFilled, env.order(t, result.OrderID.Uint64()).Status)
	for i, size := range []int64{6_000, 12_000} {
		fill := result.Fills[i]
		require.Equal(t, traderA, fill.Taker)
		require.Equal(t, traderB, fill.Maker)
		require.Equal(t, e18(size), fill.SizeUsd)
		require.Equal(t, sdk.NewIntWithDecimal(size*450, 12).BigInt(), fill.TakerFeeUsd.ToBig())
		require.Equal(t, sdk.NewIntWithDecimal(size*150, 12).BigInt(), fill.MakerFeeUsd)
	}

	var contributions []types.EventRecord
	var collected []types.EventRecord
	for _, event := range result.Events {
		switch event.Name {
		case types.EventInsuranceContribution:
			contributions = append(contributions, event)
		case types.EventFeeCollected:
			collected = append(collected, event)
		}
	}
	require.Len(t, collected, 4)
	require.Len(t, contributions, 4)
	for i, tokens := range []int64{900_000, 2_700_000, 1_800_000, 5_400_000} {
		require.Equal(t, params.CollateralToken, collected[i].Args[6])
		require.Equal(t, big.NewInt(tokens), collected[i].Args[5])
		require.Equal(t, types.EventRecord{
			Name: types.EventInsuranceContribution,
			Args: []any{params.CollateralToken, big.NewInt(tokens / 2), keeper.InsuranceSourceTradingFee},
		}, contributions[i])
	}

	insurance := common.BytesToAddress(authtypes.NewModuleAddress(types.InsuranceAccountName))
	fees := common.BytesToAddress(authtypes.NewModuleAddress(types.FeesAccountName))
	require.Equal(t, *uint256.NewInt(5_400_000), env.account(t, insurance).Collateral)
	require.Equal(t, *uint256.NewInt(5_400_000), env.account(t, fees).Collateral)
	require.Equal(t, *uint256.NewInt(9_991_900_000), env.account(t, traderA).Collateral)
	require.Equal(t, *uint256.NewInt(9_997_300_000), env.account(t, traderB).Collateral)
}
