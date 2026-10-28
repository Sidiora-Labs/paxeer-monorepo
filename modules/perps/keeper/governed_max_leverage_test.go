package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestGovernedMaxLeverageRefusesHigherLeverage(t *testing.T) {
	env := newOrdersEnv(t)
	env.fund(t, traderA, 10_000_000)
	market, err := env.k.GetMarket(env.ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "BTC", market.Symbol)
	require.Equal(t, uint64(1000), market.MaxLeverage)
	require.Equal(t, types.TierClassA, market.TierClass)

	order := limitBuy(600_000, 50_000)
	order.Leverage = e18(500)
	initial, err := env.k.PlaceOrder(env.ctx, traderA, order)
	require.NoError(t, err)
	placed := env.order(t, initial.OrderID.Uint64())
	require.Equal(t, order.Leverage, placed.Leverage)
	require.Equal(t, order.SizeUsd, placed.SizeUsd)
	require.Equal(t, types.OrderStatusResting, placed.Status)
	require.Empty(t, initial.Fills)

	market.TierClass = types.TierClassB
	response, err := keeper.NewMsgServerImpl(env.k).SetMarket(sdk.WrapSDKContext(env.ctx), &types.MsgSetMarket{
		Authority: env.k.Authority(), Market: market,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := env.k.GetMarket(env.ctx, market.ID)
	require.NoError(t, err)
	require.Equal(t, market, stored)
	require.Equal(t, uint64(1000), stored.MaxLeverage)
	require.Equal(t, types.TierClassB, stored.TierClass)

	before := env.account(t, traderA)
	sequence := append([]byte(nil), env.ctx.KVStore(env.perpsKey).Get(types.SequenceKey(types.SequenceOrderID))...)
	refused, err := env.k.PlaceOrder(env.ctx, traderA, order)
	require.ErrorIs(t, err, types.ErrLeverageTooHigh)
	require.ErrorContains(t, err, "above 200x")
	require.Equal(t, keeper.PlaceResult{}, refused)
	require.Equal(t, before, env.account(t, traderA))
	require.Equal(t, sequence, env.ctx.KVStore(env.perpsKey).Get(types.SequenceKey(types.SequenceOrderID)))
	require.Equal(t, placed, env.order(t, initial.OrderID.Uint64()))

	atLimit := order
	atLimit.Leverage = e18(200)
	accepted, err := env.k.PlaceOrder(env.ctx, traderA, atLimit)
	require.NoError(t, err)
	wantID := new(uint256.Int).AddUint64(&initial.OrderID, 1)
	require.Equal(t, *wantID, accepted.OrderID)
	boundary := env.order(t, accepted.OrderID.Uint64())
	require.Equal(t, atLimit.Leverage, boundary.Leverage)
	require.Equal(t, order.SizeUsd, boundary.SizeUsd)
	require.Equal(t, types.OrderStatusResting, boundary.Status)
	require.Equal(t, before.OpenOrderCount+1, env.account(t, traderA).OpenOrderCount)
}
