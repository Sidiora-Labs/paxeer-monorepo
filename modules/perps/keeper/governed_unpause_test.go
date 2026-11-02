package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestUnpauseRestoresOpeningOrders(t *testing.T) {
	env := newOrdersEnv(t)
	env.fund(t, traderA, 1_000)
	server := keeper.NewMsgServerImpl(env.k)
	order := limitBuy(100, 59_000)
	require.False(t, order.ReduceOnly)

	response, err := server.SetPaused(sdk.WrapSDKContext(env.ctx), &types.MsgSetPaused{
		Authority: env.k.Authority(), Paused: true,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	params, err := env.k.GetParams(env.ctx)
	require.NoError(t, err)
	require.True(t, params.Paused)
	require.ErrorIs(t, env.k.CheckPause(env.ctx, "openPosition"), types.ErrPaused)

	before := storeContents(t, env.ctx.KVStore(env.perpsKey))
	result, err := env.k.PlaceOrder(env.ctx, traderA, order)
	require.ErrorIs(t, err, types.ErrPaused)
	require.Equal(t, keeper.PlaceResult{}, result)
	require.Equal(t, before, storeContents(t, env.ctx.KVStore(env.perpsKey)))

	response, err = server.SetPaused(sdk.WrapSDKContext(env.ctx), &types.MsgSetPaused{
		Authority: env.k.Authority(), Paused: false,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	params, err = env.k.GetParams(env.ctx)
	require.NoError(t, err)
	require.False(t, params.Paused)

	result, err = env.k.PlaceOrder(env.ctx, traderA, order)
	require.NoError(t, err)
	require.Equal(t, *uint256.NewInt(1), result.OrderID)
	stored := env.order(t, result.OrderID.Uint64())
	require.Equal(t, traderA, stored.Account)
	require.Equal(t, order.MarketID, stored.MarketID)
	require.Equal(t, order.IsLong, stored.IsLong)
	require.Equal(t, order.LimitPrice, stored.LimitPrice)
	require.False(t, stored.ReduceOnly)
	require.Equal(t, types.OrderStatusResting, stored.Status)
	var resting []types.Order
	require.NoError(t, env.k.IterateBook(env.ctx, order.MarketID, types.SideBuy, func(o types.Order) bool {
		resting = append(resting, o)
		return false
	}))
	require.Equal(t, []types.Order{stored}, resting)
	require.Equal(t, uint64(1), env.account(t, traderA).OpenOrderCount)
	require.NoError(t, env.k.CheckPause(env.ctx, "openPosition"))
}
