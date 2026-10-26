package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

const MaxOpenOrdersPerAccount uint64 = 6

func TestUpdateParamsRefusesMaxOpenOrdersBelowOpenCount(t *testing.T) {
	env := newOrdersEnv(t)
	initial, err := env.k.GetParams(env.ctx)
	require.NoError(t, err)
	initial.MaxOpenOrdersPerAccount = MaxOpenOrdersPerAccount
	require.NoError(t, env.k.UpdateParams(env.ctx, env.k.Authority(), initial))
	traderC := common.HexToAddress("0x00000000000000000000000000000000000000c3")
	for _, holder := range []struct {
		address common.Address
		count   uint64
	}{{traderA, 1}, {traderB, 5}, {traderC, 2}} {
		env.fund(t, holder.address, 1_000)
		for i := uint64(0); i < holder.count; i++ {
			placed, err := env.k.PlaceOrder(env.ctx, holder.address, limitBuy(100, 59_000))
			require.NoError(t, err)
			require.Equal(t, types.OrderStatusResting, env.order(t, placed.OrderID.Uint64()).Status)
		}
		require.Equal(t, holder.count, env.account(t, holder.address).OpenOrderCount)
	}
	message, broken := keeper.OrdersInvariant(env.k)(env.ctx)
	require.False(t, broken, message)

	server := keeper.NewMsgServerImpl(env.k)
	beforeParams := append([]byte(nil), env.ctx.KVStore(env.perpsKey).Get(types.ParamsKey)...)
	beforeAccount := env.account(t, traderB)
	beforeEvents := append(sdk.Events{}, env.ctx.EventManager().Events()...)
	updated := initial
	updated.MaxOpenOrdersPerAccount = 3
	updated.LiquidationsPerBlock = initial.LiquidationsPerBlock + 1
	response, err := server.UpdateParams(sdk.WrapSDKContext(env.ctx), &types.MsgUpdateParams{
		Authority: env.k.Authority(), Params: updated,
	})
	require.ErrorIs(t, err, types.ErrInvalidParams)
	require.Nil(t, response)
	require.Equal(t, beforeParams, env.ctx.KVStore(env.perpsKey).Get(types.ParamsKey))
	stored, err := env.k.GetParams(env.ctx)
	require.NoError(t, err)
	require.Equal(t, initial, stored)
	require.Equal(t, beforeAccount, env.account(t, traderB))
	require.Equal(t, beforeEvents, env.ctx.EventManager().Events())
	message, broken = keeper.OrdersInvariant(env.k)(env.ctx)
	require.False(t, broken, message)

	updated.MaxOpenOrdersPerAccount = 5
	response, err = server.UpdateParams(sdk.WrapSDKContext(env.ctx), &types.MsgUpdateParams{
		Authority: env.k.Authority(), Params: updated,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err = env.k.GetParams(env.ctx)
	require.NoError(t, err)
	require.Equal(t, updated, stored)
	require.Len(t, env.ctx.EventManager().Events(), len(beforeEvents)+1)
	require.Equal(t, sdk.NewEvent("perps_params_updated", sdk.NewAttribute("authority", env.k.Authority())),
		env.ctx.EventManager().Events()[len(beforeEvents)])
	_, err = env.k.PlaceOrder(env.ctx, traderB, limitBuy(100, 59_000))
	require.ErrorIs(t, err, types.ErrTooManyOpenOrders)
	require.Equal(t, beforeAccount, env.account(t, traderB))
	_, err = env.k.PlaceOrder(env.ctx, traderA, limitBuy(100, 59_000))
	require.NoError(t, err)
	require.Equal(t, uint64(2), env.account(t, traderA).OpenOrderCount)
	message, broken = keeper.OrdersInvariant(env.k)(env.ctx)
	require.False(t, broken, message)

	updated.MaxOpenOrdersPerAccount = MaxOpenOrdersPerAccount
	response, err = server.UpdateParams(sdk.WrapSDKContext(env.ctx), &types.MsgUpdateParams{
		Authority: env.k.Authority(), Params: updated,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	_, err = env.k.PlaceOrder(env.ctx, traderB, limitBuy(100, 59_000))
	require.NoError(t, err)
	require.Equal(t, MaxOpenOrdersPerAccount, env.account(t, traderB).OpenOrderCount)
	message, broken = keeper.OrdersInvariant(env.k)(env.ctx)
	require.False(t, broken, message)
}
