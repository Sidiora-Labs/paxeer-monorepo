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
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryMarketsByCreatorServes(t *testing.T) {
	e := newEnv(t)
	creator := e.account(10_000_000)
	other := e.account(10_000_000)
	empty := e.account(0)
	first := e.create(creator, types.FeeStrategyClaim)
	otherMarket := e.create(other, types.FeeStrategyBurn)
	second := e.create(creator, types.FeeStrategyBurn)
	third := e.create(creator, types.FeeStrategyLpRewards)
	require.Equal(t, creator.String(), first.Creator)
	require.Equal(t, creator.String(), second.Creator)
	require.Equal(t, creator.String(), third.Creator)
	require.Equal(t, other.String(), otherMarket.Creator)

	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)
	for _, tc := range []struct {
		name    string
		creator sdk.AccAddress
		offset  uint64
		limit   uint64
		denoms  []string
	}{
		{"all", creator, 0, 100, []string{first.Denom, second.Denom, third.Denom}},
		{"first", creator, 0, 1, []string{first.Denom}},
		{"middle", creator, 1, 1, []string{second.Denom}},
		{"tail", creator, 1, 100, []string{second.Denom, third.Denom}},
		{"last", creator, 2, 2, []string{third.Denom}},
		{"past end", creator, 3, 1, []string{}},
		{"large offset", creator, math.MaxUint64 - 1, 1, []string{}},
		{"other creator", other, 0, 100, []string{otherMarket.Denom}},
		{"without markets", empty, 0, 1, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := client.MarketsByCreator(context.Background(), &types.QueryMarketsByCreatorRequest{
				Creator: tc.creator.String(), Offset: tc.offset, Limit: tc.limit,
			})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Len(t, response.Markets, len(tc.denoms))
			stored := e.k.GetMarketsByCreator(e.ctx, tc.creator, tc.offset, tc.limit)
			require.Len(t, stored, len(tc.denoms))
			for index, market := range response.Markets {
				require.Equal(t, tc.denoms[index], market.Denom)
				require.Equal(t, tc.creator.String(), market.Creator)
				require.Equal(t, types.MarketInfoOf(stored[index]), market)
			}
		})
	}
}

func TestQueryMarketsByCreatorRefusals(t *testing.T) {
	e := newEnv(t)
	creator := e.account(0)
	querier := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, querier)
	client := types.NewQueryClient(router)
	for _, goCtx := range []context.Context{context.Background(), sdk.WrapSDKContext(e.ctx)} {
		response, err := querier.MarketsByCreator(goCtx, nil)
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	for _, tc := range []struct {
		name    string
		creator string
		offset  uint64
		limit   uint64
		want    *sdkerrors.Error
	}{
		{"invalid creator", "not-bech32", 0, 1, sdkerrors.ErrInvalidAddress},
		{"empty creator", "", 0, 1, sdkerrors.ErrInvalidAddress},
		{"hex creator", e.evmAddress(creator).Hex(), 0, 1, sdkerrors.ErrInvalidAddress},
		{"zero limit", creator.String(), 0, 0, types.ErrInvalidRange},
		{"limit 101", creator.String(), 0, 101, types.ErrInvalidRange},
		{"page overflow", creator.String(), math.MaxUint64, 1, types.ErrInvalidRange},
		{"page overflow with maximum limit", creator.String(), math.MaxUint64 - 99, 100, types.ErrInvalidRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := client.MarketsByCreator(context.Background(), &types.QueryMarketsByCreatorRequest{
				Creator: tc.creator, Offset: tc.offset, Limit: tc.limit,
			})
			require.Nil(t, response)
			require.ErrorIs(t, err, tc.want)
			space, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, tc.want.Codespace(), space)
			require.Equal(t, tc.want.ABCICode(), code)
		})
	}
}
