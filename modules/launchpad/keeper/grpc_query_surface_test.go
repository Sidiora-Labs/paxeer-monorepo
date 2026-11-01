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

func TestQuerySurfaceServes(t *testing.T) {
	e := newEnv(t)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)

	cases := []struct {
		name    string
		address common.Address
		surface keeper.Surface
	}{
		{
			name:    "role surface",
			address: keeper.Generation2Surfaces[types.RoleRouter],
			surface: keeper.Surface{Role: types.RoleRouter, Generation: types.Generation2},
		},
		{
			name:    "generation one pool",
			address: address("query surface generation one pool"),
			surface: keeper.Surface{Role: types.RolePool, Generation: types.Generation1, Market: "generation-one-market"},
		},
		{
			name:    "generation two pool",
			address: address("query surface generation two pool"),
			surface: keeper.Surface{Role: types.RolePool, Generation: types.Generation2, Market: "generation-two-market"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, e.k.SetSurface(e.ctx, tc.address, tc.surface))
			recorded, found, err := e.k.GetSurface(e.ctx, tc.address)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, tc.surface, recorded)

			for _, input := range []string{tc.address.Hex(), strings.ToLower(tc.address.Hex())} {
				response, err := client.Surface(context.Background(), &types.QuerySurfaceRequest{Address: input})
				require.NoError(t, err)
				require.Equal(t, &types.QuerySurfaceResponse{
					Found: true,
					Surface: &types.SurfaceInfo{
						Role:       tc.surface.Role,
						Generation: uint32(tc.surface.Generation),
						Market:     tc.surface.Market,
					},
				}, response)
			}
		})
	}

	t.Run("unregistered address", func(t *testing.T) {
		missing := address("query surface unregistered address")
		_, found, err := e.k.GetSurface(e.ctx, missing)
		require.NoError(t, err)
		require.False(t, found)
		response, err := client.Surface(context.Background(), &types.QuerySurfaceRequest{Address: missing.Hex()})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.False(t, response.Found)
		require.Nil(t, response.Surface)
	})
}

func TestQuerySurfaceRefusals(t *testing.T) {
	e := newEnv(t)
	querier := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, querier)
	client := types.NewQueryClient(router)

	t.Run("malformed address", func(t *testing.T) {
		for _, input := range []string{"", "not-an-address", "0x1234", strings.Repeat("1", 40), "0x" + strings.Repeat("g", 40)} {
			t.Run(input, func(t *testing.T) {
				response, err := client.Surface(context.Background(), &types.QuerySurfaceRequest{Address: input})
				require.Nil(t, response)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				var addressErr *sdkerrors.Error
				require.ErrorAs(t, err, &addressErr)
				codespace, code, _ := sdkerrors.ABCIInfo(addressErr, false)
				require.Equal(t, sdkerrors.ErrInvalidAddress.Codespace(), codespace)
				require.Equal(t, sdkerrors.ErrInvalidAddress.ABCICode(), code)
			})
		}
	})

	t.Run("nil request", func(t *testing.T) {
		for _, goCtx := range []context.Context{context.Background(), sdk.WrapSDKContext(e.ctx)} {
			response, err := querier.Surface(goCtx, nil)
			require.Nil(t, response)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.Equal(t, "empty request", status.Convert(err).Message())
		}
	})

	t.Run("zero address", func(t *testing.T) {
		response, err := client.Surface(context.Background(), &types.QuerySurfaceRequest{Address: (common.Address{}).Hex()})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrZeroAddress)
		var addressErr *sdkerrors.Error
		require.ErrorAs(t, err, &addressErr)
		codespace, code, _ := sdkerrors.ABCIInfo(addressErr, false)
		require.Equal(t, types.ErrZeroAddress.Codespace(), codespace)
		require.Equal(t, types.ErrZeroAddress.ABCICode(), code)
	})

	t.Run("keeper error", func(t *testing.T) {
		corrupt := address("query surface corrupt record")
		e.ctx.KVStore(e.key).Set(types.SurfaceKey(corrupt), []byte("{"))
		_, _, keeperErr := e.k.GetSurface(e.ctx, corrupt)
		require.ErrorIs(t, keeperErr, types.ErrInvalidParams)
		response, err := client.Surface(context.Background(), &types.QuerySurfaceRequest{Address: corrupt.Hex()})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrInvalidParams)
		require.EqualError(t, err, keeperErr.Error())
		var launchpadErr *sdkerrors.Error
		require.ErrorAs(t, err, &launchpadErr)
		codespace, code, _ := sdkerrors.ABCIInfo(launchpadErr, false)
		require.Equal(t, types.ErrInvalidParams.Codespace(), codespace)
		require.Equal(t, types.ErrInvalidParams.ABCICode(), code)
	})
}
