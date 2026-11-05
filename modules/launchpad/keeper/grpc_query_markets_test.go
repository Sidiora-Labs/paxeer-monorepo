package keeper_test

import (
	"context"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryMarketsServes(t *testing.T) {
	e := newEnv(t)
	querier := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, querier)
	client := types.NewQueryClient(router)

	empty, err := client.Markets(context.Background(), &types.QueryMarketsRequest{Limit: 1})
	require.NoError(t, err)
	require.NotNil(t, empty)
	require.Empty(t, empty.Markets)
	require.Zero(t, empty.Total)

	creator := e.account(10_000_000)
	created := []types.Market{
		e.create(creator, types.FeeStrategyClaim),
		e.create(creator, types.FeeStrategyBurn),
		e.create(creator, types.FeeStrategyAirdrop),
	}
	router.Ctx = e.ctx
	require.Equal(t, uint64(len(created)), e.k.GetMarketCount(e.ctx))

	cases := []struct {
		name    string
		offset  uint64
		limit   uint64
		indices []uint64
	}{
		{name: "first", limit: 1, indices: []uint64{1}},
		{name: "middle", offset: 1, limit: 1, indices: []uint64{2}},
		{name: "page", offset: 1, limit: 2, indices: []uint64{2, 3}},
		{name: "last partial page", offset: 2, limit: 2, indices: []uint64{3}},
		{name: "maximum limit", limit: 100, indices: []uint64{1, 2, 3}},
		{name: "at total", offset: 3, limit: 1},
		{name: "past total", offset: 10, limit: 1},
		{name: "largest valid offset", offset: math.MaxUint64 - 1, limit: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := client.Markets(context.Background(), &types.QueryMarketsRequest{Offset: tc.offset, Limit: tc.limit})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, uint64(len(created)), response.Total)
			require.Equal(t, e.k.GetMarketCount(e.ctx), response.Total)
			require.Len(t, response.Markets, len(tc.indices))
			page := e.k.GetMarkets(e.ctx, tc.offset, tc.limit)
			require.Len(t, response.Markets, len(page))
			for i, market := range response.Markets {
				require.Equal(t, tc.indices[i], market.Index)
				require.Equal(t, created[market.Index-1].Denom, market.Denom)
				want := types.MarketInfoOf(page[i])
				wantBytes, err := proto.Marshal(&want)
				require.NoError(t, err)
				marketBytes, err := proto.Marshal(&market)
				require.NoError(t, err)
				require.Equal(t, wantBytes, marketBytes, "market %d differs from keeper state", market.Index)
			}
		})
	}
}

func TestQueryMarketsRefusals(t *testing.T) {
	e := newEnv(t)
	querier := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, querier)
	client := types.NewQueryClient(router)

	t.Run("nil request", func(t *testing.T) {
		for _, goCtx := range []context.Context{context.Background(), sdk.WrapSDKContext(e.ctx)} {
			response, err := querier.Markets(goCtx, nil)
			require.Nil(t, response)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		}
	})

	cases := []struct {
		name   string
		offset uint64
		limit  uint64
	}{
		{name: "zero limit", limit: 0},
		{name: "limit above maximum", limit: 101},
		{name: "offset overflow", offset: math.MaxUint64, limit: 1},
		{name: "page overflow", offset: math.MaxUint64 - 99, limit: 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := client.Markets(context.Background(), &types.QueryMarketsRequest{Offset: tc.offset, Limit: tc.limit})
			require.Nil(t, response)
			require.ErrorIs(t, err, types.ErrInvalidRange)
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, types.ErrInvalidRange.Codespace(), codespace)
			require.Equal(t, types.ErrInvalidRange.ABCICode(), code)
		})
	}
}
