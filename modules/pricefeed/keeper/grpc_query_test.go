package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func queryStoreSnapshot(t *testing.T, k keeper.Keeper, ctx sdk.Context) map[string][]byte {
	t.Helper()
	state := make(map[string][]byte)
	iterator := ctx.KVStore(k.StoreKey()).Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		state[string(iterator.Key())] = append([]byte(nil), iterator.Value()...)
	}
	return state
}

func TestQueryServerParams(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	k, ctx := pricefeedtestutil.NewKeeper(testApp, testApp.GetContextForDeliverTx([]byte{}))
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	server := keeper.NewQueryServerImpl(k)

	checkParams := func(t *testing.T) {
		t.Helper()
		before := queryStoreSnapshot(t, k, ctx)
		params := k.GetParams(ctx)
		response, err := server.Params(sdk.WrapSDKContext(ctx), &types.QueryParamsRequest{})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, params, types.Params{
			Authority:                    response.Authority,
			Markets:                      response.Markets,
			StalenessWindowMs:            response.StalenessWindowMs,
			MaxClockSkewMs:               response.MaxClockSkewMs,
			MinSources:                   response.MinSources,
			MaxDivergenceBps:             response.MaxDivergenceBps,
			MaxDeviationBps:              response.MaxDeviationBps,
			MaxObservationsPerSubmission: response.MaxObservationsPerSubmission,
			MaxSubmissionBytes:           response.MaxSubmissionBytes,
			MarkEmaTauMs:                 response.MarkEmaTauMs,
			MaxMarkBasisBps:              response.MaxMarkBasisBps,
			MaxPriceAgeMs:                response.MaxPriceAgeMs,
		})
		require.Equal(t, before, queryStoreSnapshot(t, k, ctx))
		require.Empty(t, ctx.EventManager().Events())
	}

	t.Run("defaults without stored params", checkParams)
	require.Nil(t, ctx.KVStore(k.StoreKey()).Get(types.ParamsKey))

	params := k.GetParams(ctx)
	params.Authority = sdk.AccAddress([]byte("query-params-owner--")).String()
	params.Markets[7].Enabled = false
	params.StalenessWindowMs = 11000
	params.MaxClockSkewMs = 3000
	params.MinSources = 2
	params.MaxDivergenceBps = 60
	params.MaxDeviationBps = 1100
	params.MaxObservationsPerSubmission = 16
	params.MaxSubmissionBytes = 9000
	params.MarkEmaTauMs = 160000
	params.MaxMarkBasisBps = 600
	params.MaxPriceAgeMs = 20000
	require.NoError(t, k.SetParams(ctx, params))
	t.Run("stored params", checkParams)

	response, err := server.Params(sdk.WrapSDKContext(ctx), nil)
	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestQueryServerPrice(t *testing.T) {
	k, ctx, _ := newParamsKeeper(t)
	storeAggregate(k, ctx, 0, btcAggregate())
	other := btcAggregate()
	other.IndexPrice = e18(42).AddRaw(123)
	other.MarkPrice = e18(43).AddRaw(456)
	other.Height = 301
	other.Sources = 3
	storeAggregate(k, ctx, 7, other)
	params := k.GetParams(ctx)
	params.Markets[5].Enabled = false
	require.NoError(t, k.SetParams(ctx, params))
	storeAggregate(k, ctx, 5, btcAggregate())
	server := keeper.NewQueryServerImpl(k)
	before := queryStoreSnapshot(t, k, ctx)

	for _, tc := range []struct {
		name     string
		marketID uint64
		blockMs  int64
		fresh    bool
	}{
		{"fresh", 0, int64(priceTimestampMs) + 1000, true},
		{"requested market", 7, int64(priceTimestampMs) + 1000, true},
		{"stale at request block time", 0, int64(priceTimestampMs) + 20000, false},
		{"fresh at earlier request block time", 0, int64(priceTimestampMs) + 1000, true},
		{"disabled", 5, int64(priceTimestampMs) + 1000, false},
		{"no aggregate", 6, int64(priceTimestampMs) + 1000, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queryCtx := atMs(ctx, tc.blockMs)
			price, err := k.GetPrice(queryCtx, tc.marketID)
			require.NoError(t, err)
			response, err := server.Price(sdk.WrapSDKContext(queryCtx), &types.QueryPriceRequest{MarketId: tc.marketID})
			require.NoError(t, err)
			require.Equal(t, &types.QueryPriceResponse{
				Index:       price.Index.String(),
				Mark:        price.Mark.String(),
				TimestampMs: price.TimestampMs,
				Height:      price.Height,
				Sources:     price.Sources,
				Fresh:       tc.fresh,
			}, response)
			require.Equal(t, before, queryStoreSnapshot(t, k, ctx))
			require.Empty(t, queryCtx.EventManager().Events())
		})
	}

	response, err := server.Price(sdk.WrapSDKContext(ctx), &types.QueryPriceRequest{MarketId: 40})
	require.Nil(t, response)
	requireCode(t, err, types.ErrUnknownMarket, "LXP_PRICEFEED_UNKNOWN_MARKET")
	require.ErrorContains(t, err, "market id 40")

	response, err = server.Price(sdk.WrapSDKContext(ctx), nil)
	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Equal(t, before, queryStoreSnapshot(t, k, ctx))
	require.Empty(t, ctx.EventManager().Events())
}
