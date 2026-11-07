package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/stretchr/testify/require"
)

func TestGovernedPriceAgeLimitStalesPerpsMarket(t *testing.T) {
	env := newOrdersEnv(t)
	env.fund(t, traderA, 1_000)
	env.fund(t, traderB, 1_000)
	feed := pricefeedkeeper.NewKeeper(env.pfKey, nil, nil, nil)
	params := feed.GetParams(env.ctx)
	require.Equal(t, uint64(15_000), params.MaxPriceAgeMs)
	require.NoError(t, feed.SetParams(env.ctx, params))
	env.setAggregate(t, 0, 60_000, 60_000, markNowMs-12_000)
	_, err := env.k.UpdatePrices(env.ctx)
	require.NoError(t, err)

	price, err := feed.GetPrice(env.ctx, 0)
	require.NoError(t, err)
	ageMs := uint64(env.ctx.BlockTime().UnixMilli()) - price.TimestampMs
	require.Equal(t, uint64(12_000), ageMs)
	require.Less(t, ageMs, params.MaxPriceAgeMs)
	require.True(t, price.Fresh)
	require.NoError(t, env.k.RequireFresh(env.ctx, 0))
	ask := limitBuy(600, 60_000)
	ask.IsLong = false
	resting, err := env.k.PlaceOrder(env.ctx, traderB, ask)
	require.NoError(t, err)
	require.Empty(t, resting.Fills)
	require.Equal(t, types.OrderStatusResting, env.order(t, resting.OrderID.Uint64()).Status)
	order := keeper.OrderParams{
		MarketID: 0, IsLong: true,
		OrderType: uint8(types.OrderTypeMarket), TimeInForce: uint8(types.TimeInForceIOC),
		SizeUsd: e18(600), Leverage: e18(20),
	}
	accepted, err := env.k.PlaceOrder(env.ctx, traderA, order)
	require.NoError(t, err)
	require.False(t, accepted.OrderID.IsZero())
	require.Len(t, accepted.Fills, 1)
	require.Equal(t, resting.OrderID, accepted.Fills[0].MakerOrderID)
	cached, err := env.k.GetMarketState(env.ctx, 0)
	require.NoError(t, err)
	require.True(t, cached.Fresh)
	require.Equal(t, e18(60_000), cached.MarkPrice)

	params.MaxPriceAgeMs = 10_000
	require.GreaterOrEqual(t, params.MaxPriceAgeMs, params.StalenessWindowMs)
	require.Less(t, params.MaxPriceAgeMs, ageMs)
	msg := &pricefeedtypes.MsgSetParams{
		Authority:                    params.Authority,
		Markets:                      params.Markets,
		StalenessWindowMs:            params.StalenessWindowMs,
		MaxClockSkewMs:               params.MaxClockSkewMs,
		MinSources:                   params.MinSources,
		MaxDivergenceBps:             params.MaxDivergenceBps,
		MaxDeviationBps:              params.MaxDeviationBps,
		MaxObservationsPerSubmission: params.MaxObservationsPerSubmission,
		MaxSubmissionBytes:           params.MaxSubmissionBytes,
		MarkEmaTauMs:                 params.MarkEmaTauMs,
		MaxMarkBasisBps:              params.MaxMarkBasisBps,
		MaxPriceAgeMs:                params.MaxPriceAgeMs,
	}
	proposal, err := layerxgovtypes.NewLayerXProposal(
		"Shorten the price age limit", "Require prices no older than ten seconds for trading.", msg,
	)
	require.NoError(t, err)
	registry := cdctypes.NewInterfaceRegistry()
	pricefeedtypes.RegisterInterfaces(registry)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	pricefeedtypes.RegisterMsgServer(router, pricefeedkeeper.NewMsgServerImpl(feed))
	require.NoError(t, layerxgov.NewProposalHandler(router)(env.ctx, proposal))
	require.Equal(t, params, feed.GetParams(env.ctx))

	stale, err := feed.GetPrice(env.ctx, 0)
	require.NoError(t, err)
	require.False(t, stale.Fresh)
	price.Fresh = false
	require.Equal(t, price, stale)
	state, err := env.k.GetMarketState(env.ctx, 0)
	require.NoError(t, err)
	require.Equal(t, cached, state)
	require.ErrorIs(t, env.k.RequireFresh(env.ctx, 0), types.ErrPriceStale)

	before := storeContents(t, env.ctx.KVStore(env.perpsKey))
	refused, err := env.k.PlaceOrder(env.ctx, traderA, order)
	require.ErrorIs(t, err, types.ErrPriceStale)
	require.Equal(t, keeper.PlaceResult{}, refused)
	require.Equal(t, before, storeContents(t, env.ctx.KVStore(env.perpsKey)))
}
