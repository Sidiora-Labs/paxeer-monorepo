package keeper_test

import (
	"context"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryParamsServes(t *testing.T) {
	e := newEnv(t)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)

	check := func(t *testing.T, want types.Params) {
		t.Helper()
		ctx := e.ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(e.ctx))
		stored := e.k.GetParams(ctx)
		require.Equal(t, want, stored)
		readGas := ctx.GasMeter().GasConsumed()
		require.NotZero(t, readGas)
		before := ctx.GasMeter().GasConsumed()
		router.Ctx = ctx
		response, err := client.Params(sdk.WrapSDKContext(ctx), &types.QueryParamsRequest{})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, stored, response.Params)
		require.True(t, stored.CreationFee.Equal(response.Params.CreationFee))
		require.Equal(t, stored.QuoteToken, response.Params.QuoteToken)
		require.Equal(t, readGas, ctx.GasMeter().GasConsumed()-before,
			"Params must consume only the gas for GetParams's single store read")
	}

	t.Run("initialized", func(t *testing.T) {
		check(t, e.k.GetParams(e.ctx))
	})
	t.Run("updated", func(t *testing.T) {
		params := types.Params{
			VirtualQuoteDefault: sdk.NewInt(123456789),
			VirtualTokenDefault: sdk.NewInt(987654321),
			MinFeeBps:           20,
			MaxFeeBps:           400,
			BaseFeeBps:          45,
			ProtocolFeeBps:      1234,
			FeeDecayRate:        321,
			VolatilityWeight:    654,
			ConcentrationWeight: 0,
			CreationFee:         sdk.NewInt(987654321).Mul(sdk.NewInt(987654321)).Mul(sdk.NewInt(987654321)),
			QuoteToken:          address("query params quote").Hex(),
		}
		require.NoError(t, e.k.UpdateParams(e.ctx, e.k.Authority(), params))
		check(t, params)
	})
	t.Run("missing params", func(t *testing.T) {
		e.ctx.KVStore(e.key).Delete(types.ParamsKey)
		check(t, types.DefaultParams())
	})
}

func TestQueryParamsRefusals(t *testing.T) {
	e := newEnv(t)
	ctx := e.ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(e.ctx))
	server := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, server)
	client := types.NewQueryClient(router)
	response, err := client.Params(sdk.WrapSDKContext(ctx), &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, e.k.GetParams(e.ctx), response.Params)

	for _, tc := range []struct {
		name string
		ctx  context.Context
	}{
		{name: "SDK context", ctx: sdk.WrapSDKContext(ctx)},
		{name: "without SDK context", ctx: context.Background()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := ctx.GasMeter().GasConsumed()
			response, err := server.Params(tc.ctx, nil)
			require.Nil(t, response)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.Equal(t, "empty request", status.Convert(err).Message())
			require.Equal(t, before, ctx.GasMeter().GasConsumed(), "nil requests must not access the store")
		})
	}
}
