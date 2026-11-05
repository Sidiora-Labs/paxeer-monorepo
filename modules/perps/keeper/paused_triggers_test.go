package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestPausedVenueFiresOnlyReduceOnlyTriggers(t *testing.T) {
	e := newEndblockEnv(t, 0)
	e.setMark(usdAmount(60_000))
	holder, entrant := e.address(1), e.address(3)
	size := baseAmount(100_000)
	positionID := e.pair(7, holder, 100_000_000_000, size, usdAmount(60_000))
	e.account(entrant, 100_000_000_000)
	e.maker(2, usdAmount(59_000), baseAmount(200_000))

	params, err := e.k.GetParams(e.ctx)
	require.NoError(t, err)
	require.False(t, params.Paused)
	params.TriggersPerBlock = 1
	require.NoError(t, e.k.UpdateParams(e.ctx, e.k.Authority(), params))
	market, err := e.k.GetMarket(e.ctx, 0)
	require.NoError(t, err)

	opening := types.Order{Account: entrant, MarketID: 0, IsLong: false,
		Type: types.OrderTypeStopMarket, TimeInForce: types.TimeInForceGTC,
		Status: types.OrderStatusUntriggered, TriggerPrice: usdAmount(59_500),
		SizeBase: size, Leverage: usdAmount(10)}
	stopLoss := opening
	stopLoss.Account = holder
	stopLoss.Type = types.OrderTypeStopLoss
	stopLoss.ReduceOnly = true
	store := e.ctx.KVStore(e.key)
	for _, order := range []*types.Order{&opening, &stopLoss} {
		order.ID, err = e.k.NextOrderID(e.ctx)
		require.NoError(t, err)
		e.sequence++
		order.Sequence = e.sequence
		require.NoError(t, keeper.ValidateTriggerPrice(market, *order, usdAmount(60_000)))
		store.Set(types.OrderKey(order.ID), order.Marshal())
		store.Set(types.AccountOrderKey(order.Account, order.ID), []byte{})
		account := e.readAccount(order.Account)
		account.OpenOrderCount++
		store.Set(types.AccountKey(order.Account), account.Marshal())
		require.NoError(t, e.k.TriggerInsert(e.ctx, *order))
	}
	require.Less(t, opening.Sequence, stopLoss.Sequence)
	openingBefore := append([]byte(nil), store.Get(types.OrderKey(opening.ID))...)

	response, err := keeper.NewMsgServerImpl(e.k).SetPaused(sdk.WrapSDKContext(e.ctx), &types.MsgSetPaused{
		Authority: e.k.Authority(), Paused: true,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	params, err = e.k.GetParams(e.ctx)
	require.NoError(t, err)
	require.True(t, params.Paused)

	state := e.marketState(0)
	state.LastIndexHour = endblockHour
	state.LastSampleTime = uint64(e.ctx.BlockTime().Unix())
	e.k.SetMarketState(e.ctx, state)
	e.setAggregate(0, 59_000, e.ctx.BlockTime().UnixMilli())

	records := e.k.EndBlock(e.ctx)
	require.Empty(t, e.errorEvents())
	state = e.marketState(0)
	require.True(t, state.Fresh)
	require.Equal(t, usdAmount(59_000), state.MarkPrice)
	for _, order := range []types.Order{opening, stopLoss} {
		crossed, err := keeper.TriggerReached(order, state.MarkPrice)
		require.NoError(t, err)
		require.True(t, crossed)
	}
	triggered := eventsNamed(records, types.EventOrderTriggered)
	require.Len(t, triggered, 1)
	require.Equal(t, endblockArgs(keeper.OrderTriggered(stopLoss, state.MarkPrice)), endblockArgs(triggered[0]))
	require.Len(t, eventsNamed(records, types.EventFill), 1)
	filled, err := e.k.GetOrder(e.ctx, stopLoss.ID)
	require.NoError(t, err)
	require.Equal(t, types.OrderStatusFilled, filled.Status)
	require.Equal(t, size, filled.FilledBase)
	_, open := e.positionSize(positionID)
	require.False(t, open)

	untriggered, err := e.k.GetOrder(e.ctx, opening.ID)
	require.NoError(t, err)
	require.Equal(t, types.OrderStatusUntriggered, untriggered.Status)
	require.Equal(t, openingBefore, store.Get(types.OrderKey(opening.ID)))
	require.Equal(t, uint64(1), e.readAccount(entrant).OpenOrderCount)
	require.Zero(t, e.readAccount(entrant).PositionCount)
	require.True(t, store.Has(types.AccountOrderKey(entrant, opening.ID)))
	iterator := sdk.KVStorePrefixIterator(store, types.TriggerPrefix)
	defer iterator.Close()
	require.True(t, iterator.Valid())
	id := opening.ID.Bytes32()
	require.Equal(t, id[:], iterator.Value())
	iterator.Next()
	require.False(t, iterator.Valid())
}
