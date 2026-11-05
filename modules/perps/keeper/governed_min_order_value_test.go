package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func checkOrderValue(t *testing.T, env ordersEnv, id uint256.Int, wantUsd int64) {
	t.Helper()
	bz := env.ctx.KVStore(env.perpsKey).Get(types.OrderKey(id))
	require.NotEmpty(t, bz)
	var order types.Order
	require.NoError(t, order.Unmarshal(bz))
	require.Equal(t, id, order.ID)
	require.Equal(t, e18(wantUsd), order.SizeUsd)
	require.Equal(t, types.OrderStatusResting, order.Status)
}

func TestGovernedMinOrderValueRefusesSmallOrder(t *testing.T) {
	env := newOrdersEnv(t)
	env.fund(t, traderA, 1_000)
	small := limitBuy(50, 50_000)
	initial, err := env.k.PlaceOrder(env.ctx, traderA, small)
	require.NoError(t, err)
	checkOrderValue(t, env, initial.OrderID, 50)

	params, err := env.k.GetParams(env.ctx)
	require.NoError(t, err)
	minimum := sdk.NewIntWithDecimal(100, types.USDDecimals)
	require.True(t, minimum.GT(params.MinOrderValueUsd))
	params.MinOrderValueUsd = minimum
	response, err := keeper.NewMsgServerImpl(env.k).UpdateParams(sdk.WrapSDKContext(env.ctx), &types.MsgUpdateParams{
		Authority: govAuthority(), Params: params,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := env.k.GetParams(env.ctx)
	require.NoError(t, err)
	require.Equal(t, minimum, stored.MinOrderValueUsd)

	before := env.account(t, traderA)
	refused, err := env.k.PlaceOrder(env.ctx, traderA, small)
	require.ErrorIs(t, err, types.ErrMinOrderValue)
	require.Equal(t, keeper.PlaceResult{}, refused)
	require.Equal(t, before, env.account(t, traderA))

	large := small
	large.SizeUsd = e18(150)
	accepted, err := env.k.PlaceOrder(env.ctx, traderA, large)
	require.NoError(t, err)
	wantID := new(uint256.Int).AddUint64(&initial.OrderID, 1)
	require.Equal(t, *wantID, accepted.OrderID)
	checkOrderValue(t, env, accepted.OrderID, 150)
	require.Equal(t, before.OpenOrderCount+1, env.account(t, traderA).OpenOrderCount)
}
