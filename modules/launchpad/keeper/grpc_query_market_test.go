package keeper_test

import (
	"context"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryMarketServes(t *testing.T) {
	e := newEnv(t)
	creator := e.account(3 * creationFee)
	first := e.create(creator, types.FeeStrategyBurn)
	second := e.create(creator, types.FeeStrategyBurn)
	require.NotEqual(t, first.Denom, second.Denom)
	require.NotEqual(t, first.FeeRightID, second.FeeRightID)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)

	for _, created := range []types.Market{first, second} {
		t.Run(created.Denom, func(t *testing.T) {
			stored, found := e.k.GetMarket(e.ctx, created.Denom)
			require.True(t, found)
			require.NotEqual(t, common.Address{}, common.HexToAddress(stored.PoolAddress))
			require.NotEqual(t, common.Address{}, common.HexToAddress(stored.TokenAddress))
			require.NotZero(t, stored.FeeRightID)

			response, err := client.Market(context.Background(), &types.QueryMarketRequest{Denom: created.Denom})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, types.MarketInfoOf(stored), response.Market)
			require.Equal(t, created.PoolAddress, response.Market.PoolAddress)
			require.Equal(t, created.TokenAddress, response.Market.TokenAddress)
			require.Equal(t, created.FeeRightID, response.Market.FeeRightID)
		})
	}
}

func TestQueryMarketRefusals(t *testing.T) {
	e := newEnv(t)
	querier := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, querier)
	client := types.NewQueryClient(router)

	t.Run("nil request", func(t *testing.T) {
		response, err := querier.Market(context.Background(), nil)
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("empty denom", func(t *testing.T) {
		response, err := client.Market(context.Background(), &types.QueryMarketRequest{})
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("unknown denom preserves launchpad error", func(t *testing.T) {
		denom := types.ModuleName + "/" + common.HexToAddress("0x1234").Hex()
		_, found := e.k.GetMarket(e.ctx, denom)
		require.False(t, found)
		response, err := client.Market(context.Background(), &types.QueryMarketRequest{Denom: denom})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrUnknownMarket)
		require.Same(t, types.ErrUnknownMarket, err)
		codespace, code, _ := sdkerrors.ABCIInfo(err, false)
		require.Equal(t, types.ModuleName, codespace)
		require.Equal(t, types.ErrUnknownMarket.ABCICode(), code)
	})
}
