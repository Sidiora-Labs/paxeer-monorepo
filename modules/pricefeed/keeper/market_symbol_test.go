package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/stretchr/testify/require"
)

func TestMarketSymbolOfListedMarket(t *testing.T) {
	k, ctx, _ := newParamsKeeper(t)
	symbols := []string{
		"BTC", "ETH", "SOL", "AVAX", "LINK", "TSLA", "NVDA", "NAS100", "XAU",
		"SPX500", "GOOGL", "PAX", "SID", "HYPE", "XRP", "ASTER", "TRUMP", "BNB",
	}
	require.Len(t, k.GetParams(ctx).Markets, len(symbols))
	for id, want := range symbols {
		symbol, enabled, err := k.MarketSymbol(ctx, uint64(id))
		require.NoError(t, err)
		require.Equal(t, want, symbol, "market id %d", id)
		require.True(t, enabled, "market id %d", id)
	}

	params := k.GetParams(ctx)
	params.Markets[0].Symbol = "BTC2"
	for i := range params.Markets {
		params.Markets[i].Enabled = i%2 != 0
	}
	require.NoError(t, k.SetParams(ctx, params))
	for _, market := range params.Markets {
		symbol, enabled, err := k.MarketSymbol(ctx, market.ID)
		require.NoError(t, err)
		require.Equal(t, market.Symbol, symbol, "market id %d", market.ID)
		require.Equal(t, market.Enabled, enabled, "market id %d", market.ID)
	}
}

func TestMarketSymbolOfUnlistedMarket(t *testing.T) {
	k, ctx, _ := newParamsKeeper(t)
	require.Len(t, k.GetParams(ctx).Markets, 18)
	for _, id := range []uint64{18, 1 << 63, ^uint64(0)} {
		symbol, enabled, err := k.MarketSymbol(ctx, id)
		requireCode(t, err, types.ErrUnknownMarket, "LXP_PRICEFEED_UNKNOWN_MARKET")
		require.Empty(t, symbol)
		require.False(t, enabled)
		_, priceErr := k.GetPrice(ctx, id)
		requireCode(t, priceErr, types.ErrUnknownMarket, "LXP_PRICEFEED_UNKNOWN_MARKET")
		require.EqualError(t, err, priceErr.Error())
	}

	params := k.GetParams(ctx)
	params.Markets = nil
	require.NoError(t, k.SetParams(ctx, params))
	symbol, enabled, err := k.MarketSymbol(ctx, 0)
	requireCode(t, err, types.ErrUnknownMarket, "LXP_PRICEFEED_UNKNOWN_MARKET")
	require.Empty(t, symbol)
	require.False(t, enabled)
}
