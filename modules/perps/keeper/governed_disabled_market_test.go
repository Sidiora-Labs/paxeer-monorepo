package keeper_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestDisabledMarketRefusesOpensAllowsCloses(t *testing.T) {
	env := newOrdersEnv(t)
	env.fund(t, traderA, 1_000)
	env.fund(t, traderB, 1_000)
	server := keeper.NewMsgServerImpl(env.k)
	eth, err := env.k.GetMarket(env.ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "ETH", eth.Symbol)
	eth.Enabled = true
	response, err := server.SetMarket(sdk.WrapSDKContext(env.ctx), &types.MsgSetMarket{
		Authority: env.k.Authority(), Market: eth,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	env.setAggregate(t, eth.ID, 3_000, 3_000, markNowMs-1_000)
	_, err = env.k.UpdatePrices(env.ctx)
	require.NoError(t, err)

	ask := limitBuy(600, 3_000)
	ask.MarketID = eth.ID
	ask.IsLong = false
	resting, err := env.k.PlaceOrder(env.ctx, traderB, ask)
	require.NoError(t, err)
	require.Empty(t, resting.Fills)
	require.Equal(t, types.OrderStatusResting, env.order(t, resting.OrderID.Uint64()).Status)

	opening := keeper.OrderParams{
		MarketID: eth.ID, IsLong: true,
		OrderType: uint8(types.OrderTypeMarket), TimeInForce: uint8(types.TimeInForceIOC),
		SizeUsd: e18(600), Leverage: e18(20),
	}
	require.False(t, opening.ReduceOnly)
	opened, err := env.k.PlaceOrder(env.ctx, traderA, opening)
	require.NoError(t, err)
	require.Len(t, opened.Fills, 1)
	require.Equal(t, resting.OrderID, opened.Fills[0].MakerOrderID)
	positionID := opened.Fills[0].TakerPositionID
	position, err := env.k.GetPosition(env.ctx, positionID)
	require.NoError(t, err)
	require.Equal(t, traderA, position.Account)
	require.Equal(t, eth.ID, position.MarketID)
	require.True(t, position.IsLong)
	require.Equal(t, e18(600), position.CostBasisUsd)
	require.False(t, position.SizeBase.IsZero())

	bid := limitBuy(600, 3_000)
	bid.MarketID = eth.ID
	bid.SizeBase = position.SizeBase
	bid.SizeUsd.Clear()
	bid.ReduceOnly = true
	liquidity, err := env.k.PlaceOrder(env.ctx, traderB, bid)
	require.NoError(t, err)
	require.Empty(t, liquidity.Fills)
	require.Equal(t, types.OrderStatusResting, env.order(t, liquidity.OrderID.Uint64()).Status)

	eth, err = env.k.GetMarket(env.ctx, eth.ID)
	require.NoError(t, err)
	require.True(t, eth.Enabled)
	eth.Enabled = false
	response, err = server.SetMarket(sdk.WrapSDKContext(env.ctx), &types.MsgSetMarket{
		Authority: env.k.Authority(), Market: eth,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := env.k.GetMarket(env.ctx, eth.ID)
	require.NoError(t, err)
	require.Equal(t, eth, stored)
	require.False(t, stored.Enabled)

	before := storeContents(t, env.ctx.KVStore(env.perpsKey))
	refused, err := env.k.PlaceOrder(env.ctx, traderA, opening)
	require.ErrorIs(t, err, types.ErrMarketDisabled)
	require.Equal(t, keeper.PlaceResult{}, refused)
	require.Equal(t, before, storeContents(t, env.ctx.KVStore(env.perpsKey)))

	closeOrder := opening
	closeOrder.IsLong = false
	closeOrder.ReduceOnly = true
	closeOrder.SizeUsd.Clear()
	closeOrder.SizeBase = position.SizeBase
	closed, err := env.k.PlaceOrder(env.ctx, traderA, closeOrder)
	require.NoError(t, err)
	require.Len(t, closed.Fills, 1)
	fill := closed.Fills[0]
	require.Equal(t, liquidity.OrderID, fill.MakerOrderID)
	require.Equal(t, closed.OrderID, fill.TakerOrderID)
	require.Equal(t, positionID, fill.TakerPositionID)
	require.Equal(t, traderA, fill.Taker)
	require.Equal(t, eth.ID, fill.MarketID)
	require.Equal(t, position.SizeBase, fill.SizeBase)
	require.Equal(t, e18(600), fill.SizeUsd)
	require.Equal(t, e18(3_000), fill.Price)
	filledOrder := env.order(t, closed.OrderID.Uint64())
	require.True(t, filledOrder.ReduceOnly)
	require.Equal(t, types.OrderStatusFilled, filledOrder.Status)

	var holderClosed []types.EventRecord
	for _, event := range closed.Events {
		if event.Name == types.EventPositionClosed {
			require.Len(t, event.Args, 7)
			if event.Args[1] == traderA {
				holderClosed = append(holderClosed, event)
			}
		}
	}
	require.Equal(t, []types.EventRecord{{
		Name: types.EventPositionClosed,
		Args: []any{positionID.ToBig(), traderA, new(big.Int).SetUint64(eth.ID),
			fill.SizeUsd.ToBig(), fill.Price.ToBig(), big.NewInt(0), true},
	}}, holderClosed)
	_, err = env.k.GetPosition(env.ctx, positionID)
	require.ErrorIs(t, err, types.ErrUnknownPosition)
	require.Zero(t, env.account(t, traderA).PositionCount)
	require.False(t, env.ctx.KVStore(env.perpsKey).Has(types.AccountMarketPositionKey(traderA, eth.ID)))
	stored, err = env.k.GetMarket(env.ctx, eth.ID)
	require.NoError(t, err)
	require.False(t, stored.Enabled)
}
