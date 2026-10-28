package keeper_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryMarketByAddressServes(t *testing.T) {
	e := newEnv(t)
	creator := e.account(200_000_000)
	markets := []types.Market{
		e.create(creator, types.FeeStrategyClaim),
		e.create(creator, types.FeeStrategyBurn),
	}
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)

	for _, market := range markets {
		t.Run(market.Denom, func(t *testing.T) {
			want := types.MarketInfoOf(e.market(market.Denom))
			var poolView *types.QueryMarketByAddressResponse
			for _, address := range []string{
				market.PoolAddress, market.TokenAddress,
				strings.ToLower(market.PoolAddress), strings.ToLower(market.TokenAddress),
			} {
				response, err := client.MarketByAddress(context.Background(), &types.QueryMarketByAddressRequest{Address: address})
				require.NoError(t, err, address)
				require.NotNil(t, response)
				require.Equal(t, want, response.Market, address)
				if poolView == nil {
					poolView = response
				} else {
					require.Equal(t, poolView, response, address)
				}
			}
		})
	}
}

func TestQueryMarketByAddressRefusals(t *testing.T) {
	e := newEnv(t)
	market := e.create(e.account(100_000_000), types.FeeStrategyClaim)
	querier := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, querier)
	client := types.NewQueryClient(router)

	t.Run("nil request", func(t *testing.T) {
		for _, ctx := range []context.Context{context.Background(), sdk.WrapSDKContext(e.ctx)} {
			response, err := querier.MarketByAddress(ctx, nil)
			require.Nil(t, response)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		}
	})

	cases := []struct {
		name    string
		address string
		want    *sdkerrors.Error
	}{
		{"empty address", "", sdkerrors.ErrInvalidAddress},
		{"short address", "0x1234", sdkerrors.ErrInvalidAddress},
		{"missing prefix", strings.TrimPrefix(market.PoolAddress, "0x"), sdkerrors.ErrInvalidAddress},
		{"nonhex address", "0x" + strings.Repeat("z", 40), sdkerrors.ErrInvalidAddress},
		{"zero address", (common.Address{}).Hex(), types.ErrZeroAddress},
		{"unknown address", address("unknown query market").Hex(), types.ErrUnknownMarket},
		{"unregistered token", e.usdl.Hex(), types.ErrUnknownMarket},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := client.MarketByAddress(context.Background(), &types.QueryMarketByAddressRequest{Address: tc.address})
			require.Nil(t, response)
			require.ErrorIs(t, err, tc.want)
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, tc.want.Codespace(), codespace)
			require.Equal(t, tc.want.ABCICode(), code)
		})
	}

	t.Run("keeper errors reach both address lookups", func(t *testing.T) {
		require.NoError(t, e.k.SetSurface(e.ctx, common.HexToAddress(market.PoolAddress), keeper.Surface{
			Role: types.RolePool, Generation: types.Generation2, Market: "launchpad/missing-market",
		}))
		for _, address := range []string{market.PoolAddress, market.TokenAddress} {
			response, err := client.MarketByAddress(context.Background(), &types.QueryMarketByAddressRequest{Address: address})
			require.Nil(t, response)
			require.ErrorIs(t, err, types.ErrInvalidConfiguration)
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, types.ErrInvalidConfiguration.Codespace(), codespace)
			require.Equal(t, types.ErrInvalidConfiguration.ABCICode(), code)
		}
	})
}
