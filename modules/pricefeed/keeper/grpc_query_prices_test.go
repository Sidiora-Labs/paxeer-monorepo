package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryServerPricesBatch(t *testing.T) {
	k, ctx, _ := newParamsKeeper(t)
	storeAggregate(k, ctx, 0, btcAggregate())
	eth := btcAggregate()
	eth.IndexPrice = e18(3500).AddRaw(123)
	eth.MarkPrice = e18(3501).AddRaw(456)
	eth.TimestampMs += 100
	eth.Height = 301
	eth.Sources = 3
	storeAggregate(k, ctx, 1, eth)
	params := k.GetParams(ctx)
	params.Markets[5].Enabled = false
	require.NoError(t, k.SetParams(ctx, params))
	storeAggregate(k, ctx, 5, btcAggregate())
	server := keeper.NewQueryServerImpl(k)
	before := queryStoreSnapshot(t, k, ctx)
	boundary := make([]uint64, 64)
	for i := range boundary {
		boundary[i] = uint64(i % 2)
	}

	for _, tc := range []struct {
		name    string
		ids     []uint64
		blockMs int64
	}{
		{"ETH then BTC", []uint64{1, 0}, int64(priceTimestampMs) + 1000},
		{"duplicates", []uint64{1, 0, 1}, int64(priceTimestampMs) + 1000},
		{"64 ids", boundary, int64(priceTimestampMs) + 1000},
		{"empty", nil, int64(priceTimestampMs) + 1000},
		{"stale", []uint64{1, 0}, int64(priceTimestampMs) + 20000},
		{"disabled and missing aggregate", []uint64{5, 6}, int64(priceTimestampMs) + 1000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queryCtx := atMs(ctx, tc.blockMs)
			response, err := server.Prices(sdk.WrapSDKContext(queryCtx), &types.QueryPricesRequest{MarketIds: tc.ids})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Len(t, response.Prices, len(tc.ids))
			for i, id := range tc.ids {
				price, err := k.GetPrice(queryCtx, id)
				require.NoError(t, err)
				require.Equal(t, types.QueryPriceResponse{
					Index:       price.Index.String(),
					Mark:        price.Mark.String(),
					TimestampMs: price.TimestampMs,
					Height:      price.Height,
					Sources:     price.Sources,
					Fresh:       price.Fresh,
				}, response.Prices[i], "position %d, market %d", i, id)
			}
			require.Equal(t, before, queryStoreSnapshot(t, k, ctx))
			require.Empty(t, queryCtx.EventManager().Events())
		})
	}

	for _, ids := range [][]uint64{{40}, {0, 40}, {40, 0}} {
		response, err := server.Prices(sdk.WrapSDKContext(ctx), &types.QueryPricesRequest{MarketIds: ids})
		require.Nil(t, response)
		requireCode(t, err, types.ErrUnknownMarket, "LXP_PRICEFEED_UNKNOWN_MARKET")
		require.ErrorContains(t, err, "market id 40")
	}

	response, err := server.Prices(sdk.WrapSDKContext(ctx), nil)
	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Equal(t, before, queryStoreSnapshot(t, k, ctx))
	require.Empty(t, ctx.EventManager().Events())
}

func TestQueryServerPricesRefusesOver64(t *testing.T) {
	k, ctx, _ := newParamsKeeper(t)
	storeAggregate(k, ctx, 0, btcAggregate())
	server := keeper.NewQueryServerImpl(k)
	before := queryStoreSnapshot(t, k, ctx)

	for _, tc := range []struct {
		name string
		id   uint64
	}{
		{"listed ids", 0},
		{"unlisted ids", 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ids := make([]uint64, 65)
			for i := range ids {
				ids[i] = tc.id
			}
			queryCtx := ctx.WithGasMeter(sdk.NewGasMeter(0, 1, 1))
			response, err := server.Prices(sdk.WrapSDKContext(queryCtx), &types.QueryPricesRequest{MarketIds: ids})
			require.Nil(t, response)
			requireCode(t, err, types.ErrInvalidParams, "LXP_PRICEFEED_INVALID_PARAMS")
			require.Zero(t, queryCtx.GasMeter().GasConsumed())
			require.Equal(t, before, queryStoreSnapshot(t, k, ctx))
			require.Empty(t, queryCtx.EventManager().Events())
		})
	}
}
