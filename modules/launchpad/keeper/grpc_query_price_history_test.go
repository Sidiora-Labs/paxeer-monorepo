package keeper_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryPriceHistoryServes(t *testing.T) {
	e := newEnv(t)
	creator := e.account(2_000_000)
	market := e.create(creator, types.FeeStrategyClaim)
	other := e.create(creator, types.FeeStrategyClaim)
	trader := e.account(100_000_000)
	want := make([]types.PricePoint, 0, types.SnapshotSlots+2)
	for n := 0; n < types.SnapshotSlots+2; n++ {
		height := uint64((n + 1) * 10)
		e.ctx = e.ctx.WithBlockHeight(int64(height))
		e.at(mature + time.Duration(n)*time.Minute)
		e.swap(trader, market.Denom, true, 2_000_000)
		price, err := e.market(market.Denom).Price()
		require.NoError(t, err)
		want = append(want, types.PricePoint{Height: height, Price: sdk.NewIntFromBigInt(price)})
	}
	before := e.market(market.Denom)
	server := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, server)
	client := types.NewQueryClient(router)
	for _, tc := range []struct {
		name     string
		denom    string
		from, to uint64
		want     []types.PricePoint
	}{
		{name: "retained ring order", denom: market.Denom, from: 0, to: ^uint64(0), want: want[2:]},
		{name: "inclusive range", denom: market.Denom, from: 40, to: 60, want: want[3:6]},
		{name: "single height", denom: market.Denom, from: 50, to: 50, want: want[4:5]},
		{name: "between heights", denom: market.Denom, from: 41, to: 59, want: want[4:5]},
		{name: "evicted heights", denom: market.Denom, from: 0, to: 20},
		{name: "after latest", denom: market.Denom, from: 101, to: ^uint64(0)},
		{name: "zero height", denom: market.Denom, from: 0, to: 0},
		{name: "other market", denom: other.Denom, from: 0, to: ^uint64(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			points, err := e.k.PriceHistory(e.ctx, tc.denom, tc.from, tc.to)
			require.NoError(t, err)
			req := &types.QueryPriceHistoryRequest{Denom: tc.denom, FromHeight: tc.from, ToHeight: tc.to}
			direct, err := server.PriceHistory(sdk.WrapSDKContext(e.ctx), req)
			require.NoError(t, err)
			require.NotNil(t, direct)
			require.Equal(t, points, direct.Points)
			response, err := client.PriceHistory(context.Background(), req)
			require.NoError(t, err)
			require.NotNil(t, response)
			if len(tc.want) == 0 {
				require.Empty(t, points)
				require.Empty(t, response.Points)
			} else {
				require.Equal(t, tc.want, points)
				require.Equal(t, points, response.Points)
			}
		})
	}
	require.Equal(t, before, e.market(market.Denom))
}

func TestQueryPriceHistoryRefusals(t *testing.T) {
	e := newEnv(t)
	market := e.create(e.account(1_000_000), types.FeeStrategyClaim)
	before := e.market(market.Denom)
	server := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, server)
	client := types.NewQueryClient(router)
	t.Run("nil request", func(t *testing.T) {
		for _, goCtx := range []context.Context{context.Background(), sdk.WrapSDKContext(e.ctx)} {
			response, err := server.PriceHistory(goCtx, nil)
			require.Nil(t, response)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.Equal(t, "empty request", status.Convert(err).Message())
		}
	})
	for _, tc := range []struct {
		name string
		req  types.QueryPriceHistoryRequest
		want *sdkerrors.Error
	}{
		{name: "unknown market", req: types.QueryPriceHistoryRequest{Denom: "unknown-price-market", ToHeight: ^uint64(0)}, want: types.ErrUnknownMarket},
		{name: "empty denom", req: types.QueryPriceHistoryRequest{}, want: types.ErrUnknownMarket},
		{name: "reversed range", req: types.QueryPriceHistoryRequest{Denom: market.Denom, FromHeight: 2, ToHeight: 1}, want: types.ErrInvalidRange},
		{name: "zero upper bound", req: types.QueryPriceHistoryRequest{Denom: market.Denom, FromHeight: 1}, want: types.ErrInvalidRange},
		{name: "maximum lower bound", req: types.QueryPriceHistoryRequest{Denom: market.Denom, FromHeight: ^uint64(0)}, want: types.ErrInvalidRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			points, keeperErr := e.k.PriceHistory(e.ctx, tc.req.Denom, tc.req.FromHeight, tc.req.ToHeight)
			require.Nil(t, points)
			require.ErrorIs(t, keeperErr, tc.want)
			direct, err := server.PriceHistory(sdk.WrapSDKContext(e.ctx), &tc.req)
			require.Nil(t, direct)
			require.ErrorIs(t, err, tc.want)
			require.EqualError(t, err, keeperErr.Error())
			require.IsType(t, keeperErr, err)
			response, err := client.PriceHistory(context.Background(), &tc.req)
			require.Nil(t, response)
			require.ErrorIs(t, err, tc.want)
			require.EqualError(t, err, keeperErr.Error())
			require.IsType(t, keeperErr, err)
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, types.ModuleName, codespace)
			require.Equal(t, tc.want.ABCICode(), code)
		})
	}
	require.Equal(t, before, e.market(market.Denom))
}
