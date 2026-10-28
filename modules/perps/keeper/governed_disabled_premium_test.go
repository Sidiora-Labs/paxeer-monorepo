package keeper_test

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestDisabledMarketSkipsPremiumSample(t *testing.T) {
	e := newEndblockEnv(t)
	for _, market := range []types.Market{
		listedMarket(0, "BTC", 5, 1, types.TierClassA, 20_000, 500_000_000),
		listedMarket(2, "SOL", 2, 3, types.TierClassB, 10_000, 100_000_000),
	} {
		require.NoError(t, e.k.SetMarket(e.ctx, e.k.Authority(), market))
	}
	for i, price := range []int64{101, 103} {
		require.NoError(t, e.k.BookInsert(e.ctx, types.Order{
			ID: *uint256.NewInt(uint64(i + 1)), MarketID: 2, IsLong: i == 0,
			Type: types.OrderTypeLimit, TimeInForce: types.TimeInForceGTC, Status: types.OrderStatusResting,
			LimitPrice: e18(price), SizeBase: e18(1_000), Sequence: uint64(i + 1),
		}))
	}
	e.setAggregate(0, 60_000, e.ctx.BlockTime().UnixMilli())
	e.setAggregate(2, 100, e.ctx.BlockTime().UnixMilli())
	records, err := e.k.UpdatePrices(e.ctx)
	require.NoError(t, err)
	require.Len(t, eventsNamed(records, types.EventMarkPriceUpdated), 2)
	for _, id := range []uint64{0, 2} {
		market, err := e.k.GetMarket(e.ctx, id)
		require.NoError(t, err)
		require.True(t, market.Enabled)
		SamplePremium(t, e, market)
	}
	before := e.marketState(2)
	require.True(t, before.Fresh)
	require.Equal(t, e18(100), before.IndexPrice)
	require.Equal(t, e18(101), before.MarkPrice)
	require.Equal(t, e18(2), before.PremiumEma)
	require.Equal(t, uint64(1), before.PremiumCount)
	require.Equal(t, *uint256.NewInt(10_000_000_000_000_000), before.PremiumSum)
	require.Equal(t, e18(100), before.OracleSum)
	require.Equal(t, uint64(e.ctx.BlockTime().Unix()), before.LastSampleTime)
	beforeBytes := append([]byte(nil), e.ctx.KVStore(e.key).Get(types.MarketStateKey(2))...)

	sol, err := e.k.GetMarket(e.ctx, 2)
	require.NoError(t, err)
	sol.Enabled = false
	response, err := keeper.NewMsgServerImpl(e.k).SetMarket(sdk.WrapSDKContext(e.ctx), &types.MsgSetMarket{
		Authority: e.k.Authority(), Market: sol,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := e.k.GetMarket(e.ctx, 2)
	require.NoError(t, err)
	require.Equal(t, sol, stored)
	require.False(t, stored.Enabled)
	require.Equal(t, before, e.marketState(2), "disabling SOL preserves its existing state")

	params, err := e.k.GetParams(e.ctx)
	require.NoError(t, err)
	interval := time.Duration(params.PremiumIntervalSeconds+1) * time.Second
	oracleSum := int64(60_000)
	for i := int64(1); i <= 2; i++ {
		e.ctx = e.ctx.WithBlockHeight(e.ctx.BlockHeight() + 1).
			WithBlockTime(e.ctx.BlockTime().Add(interval)).WithEventManager(sdk.NewEventManager())
		btcPrice := 60_000 + 10*i
		e.setAggregate(0, btcPrice, e.ctx.BlockTime().UnixMilli())
		e.setAggregate(2, 100+10*i, e.ctx.BlockTime().UnixMilli())
		require.Greater(t, uint64(e.ctx.BlockTime().Unix())-before.LastSampleTime, params.PremiumIntervalSeconds)

		UpdatePrices(t, e, btcPrice, before, beforeBytes)

		records = e.k.EndBlock(e.ctx)
		require.Empty(t, e.errorEvents())
		require.Len(t, records, 1)
		require.Equal(t, []string{types.EventMarkPriceUpdated, "0", e18Dec(btcPrice), e18Dec(btcPrice), e18Dec(btcPrice)},
			eventStrings(records[0]))
		require.Equal(t, before, e.marketState(2), "EndBlock leaves SOL's mark, EMA and premium accumulators unchanged")
		require.Equal(t, beforeBytes, e.ctx.KVStore(e.key).Get(types.MarketStateKey(2)), "no SOL market state is written")
		stored, err = e.k.GetMarket(e.ctx, 2)
		require.NoError(t, err)
		require.Equal(t, sol, stored, "SOL remains disabled")

		btc := e.marketState(0)
		oracleSum += btcPrice
		require.True(t, btc.Fresh)
		require.Equal(t, e18(btcPrice), btc.IndexPrice)
		require.Equal(t, e18(btcPrice), btc.MarkPrice)
		require.Equal(t, uint64(e.ctx.BlockTime().UnixMilli()), btc.PriceTimestampMs)
		require.Equal(t, uint64(i+1), btc.PremiumCount, "enabled BTC still samples once each EndBlock")
		require.Equal(t, e18(oracleSum), btc.OracleSum)
		require.Equal(t, uint64(e.ctx.BlockTime().Unix()), btc.LastSampleTime)
	}
}

func UpdatePrices(t *testing.T, e *endblockEnv, btcPrice int64, before types.MarketState, beforeBytes []byte) {
	t.Helper()
	priceCtx, _ := e.ctx.CacheContext()
	records, err := e.k.UpdatePrices(priceCtx)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, []string{types.EventMarkPriceUpdated, "0", e18Dec(btcPrice), e18Dec(btcPrice), e18Dec(btcPrice)},
		eventStrings(records[0]))
	unchanged, err := e.k.GetMarketState(priceCtx, 2)
	require.NoError(t, err)
	require.Equal(t, before, unchanged, "UpdatePrices skips governance-disabled SOL")
	require.Equal(t, beforeBytes, priceCtx.KVStore(e.key).Get(types.MarketStateKey(2)))
}

func SamplePremium(t *testing.T, e *endblockEnv, market types.Market) {
	t.Helper()
	events, err := e.k.SettleFunding(e.ctx, market)
	require.NoError(t, err)
	require.Empty(t, events)
	sampled, err := e.k.SamplePremium(e.ctx, market)
	require.NoError(t, err)
	require.True(t, sampled)
}
