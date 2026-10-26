package keeper_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestGovernedFeeTiersChangeTakerFee(t *testing.T) {
	env := newOrdersEnv(t)
	env.fund(t, traderA, 1_000)
	env.fund(t, traderB, 1_000)

	before, err := env.k.FeeTier(env.ctx, traderB)
	require.NoError(t, err)
	require.Equal(t, uint8(0), before.Tier)
	require.Equal(t, uint64(450), before.TakerFeePpm)
	require.True(t, before.Volume14dUsd.IsZero())

	maker := limitBuy(600, 60_000)
	maker.IsLong = false
	resting, err := env.k.PlaceOrder(env.ctx, traderA, maker)
	require.NoError(t, err)
	require.Empty(t, resting.Fills)
	require.Equal(t, types.OrderStatusResting, env.order(t, resting.OrderID.Uint64()).Status)

	params, err := env.k.GetParams(env.ctx)
	require.NoError(t, err)
	params.FeeTiers = []types.FeeTier{
		{MinVolumeUsd: sdk.ZeroInt(), TakerFeePpm: 500, MakerFeePpm: 150},
	}
	response, err := keeper.NewMsgServerImpl(env.k).UpdateParams(sdk.WrapSDKContext(env.ctx), &types.MsgUpdateParams{
		Authority: env.k.Authority(), Params: params,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := env.k.GetParams(env.ctx)
	require.NoError(t, err)
	require.Equal(t, params.FeeTiers, stored.FeeTiers)

	tier, err := env.k.FeeTier(env.ctx, traderB)
	require.NoError(t, err)
	require.Equal(t, uint8(0), tier.Tier)
	require.Equal(t, uint64(500), tier.TakerFeePpm)
	require.True(t, tier.Volume14dUsd.IsZero())
	accountBefore := env.account(t, traderB)
	require.Equal(t, *uint256.NewInt(1_000_000_000), accountBefore.Collateral)

	filled, err := env.k.PlaceOrder(env.ctx, traderB, keeper.OrderParams{
		MarketID: 0, IsLong: true,
		OrderType: uint8(types.OrderTypeMarket), TimeInForce: uint8(types.TimeInForceIOC),
		SizeUsd: e18(600), Leverage: e18(20),
	})
	require.NoError(t, err)
	require.Len(t, filled.Fills, 1)
	fill := filled.Fills[0]
	require.Equal(t, resting.OrderID, fill.MakerOrderID)
	require.Equal(t, filled.OrderID, fill.TakerOrderID)
	require.Equal(t, traderA, fill.Maker)
	require.Equal(t, traderB, fill.Taker)
	require.Equal(t, e18(60_000), fill.Price)
	require.Equal(t, e18(600), fill.SizeUsd)
	require.False(t, fill.TakerPositionID.IsZero())
	require.Equal(t, types.OrderStatusFilled, env.order(t, filled.OrderID.Uint64()).Status)
	require.Equal(t, types.OrderStatusFilled, env.order(t, resting.OrderID.Uint64()).Status)

	feeUsd := big.NewInt(300_000_000_000_000_000)
	require.Equal(t, feeUsd, fill.TakerFeeUsd.ToBig())
	var takerFees []types.EventRecord
	for _, event := range filled.Events {
		if event.Name != types.EventFeeCollected {
			continue
		}
		require.Len(t, event.Args, 7)
		if event.Args[1] == traderB {
			takerFees = append(takerFees, event)
		}
	}
	require.Equal(t, []types.EventRecord{{
		Name: types.EventFeeCollected,
		Args: []any{fill.TakerPositionID.ToBig(), traderB, big.NewInt(0), keeper.FeeTypeTrading,
			feeUsd, big.NewInt(300_000), params.CollateralToken},
	}}, takerFees)
	accountAfter := env.account(t, traderB)
	var charged uint256.Int
	charged.Sub(&accountBefore.Collateral, &accountAfter.Collateral)
	require.Equal(t, *uint256.NewInt(300_000), charged)

	tier, err = env.k.FeeTier(env.ctx, traderB)
	require.NoError(t, err)
	require.Equal(t, uint8(0), tier.Tier)
	require.Equal(t, uint64(500), tier.TakerFeePpm)
	require.Equal(t, e18(600), tier.Volume14dUsd)
}
