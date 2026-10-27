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

func TestQueryRetiredServes(t *testing.T) {
	e := newEnv(t)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)
	goCtx := sdk.WrapSDKContext(e.ctx)
	retiredAddress := address("query retired router")
	entry := types.RetiredAddress{
		Address:     retiredAddress.Hex(),
		Role:        types.RoleRouter,
		Replacement: keeper.Generation2Surfaces[types.RoleRouter].Hex(),
	}
	require.NoError(t, e.k.RecordRetired(e.ctx, entry))
	record, found, err := e.k.GetRetired(e.ctx, retiredAddress)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, entry, record)
	want := types.RetiredInfoOf(record)
	for _, value := range []string{retiredAddress.Hex(), strings.ToLower(retiredAddress.Hex())} {
		response, err := client.Retired(goCtx, &types.QueryRetiredRequest{Address: value})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.True(t, response.Found)
		require.Equal(t, &want, response.Retired)
	}
	response, err := client.Retired(goCtx, &types.QueryRetiredRequest{Address: address("query never retired").Hex()})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.False(t, response.Found)
	require.Nil(t, response.Retired)
}

func TestQueryRetiredRefusals(t *testing.T) {
	e := newEnv(t)
	querier := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, querier)
	client := types.NewQueryClient(router)
	goCtx := sdk.WrapSDKContext(e.ctx)

	t.Run("nil request", func(t *testing.T) {
		response, err := querier.Retired(context.Background(), nil)
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Equal(t, "empty request", status.Convert(err).Message())
		response, err = querier.Retired(goCtx, nil)
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Equal(t, "empty request", status.Convert(err).Message())
	})
	for _, value := range []string{"", "0x1234", strings.Repeat("1", 40), "0x" + strings.Repeat("g", 40)} {
		t.Run("malformed "+value, func(t *testing.T) {
			response, err := client.Retired(goCtx, &types.QueryRetiredRequest{Address: value})
			require.Nil(t, response)
			require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
		})
	}
	t.Run("zero address preserves launchpad code", func(t *testing.T) {
		response, err := client.Retired(goCtx, &types.QueryRetiredRequest{Address: (common.Address{}).Hex()})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrZeroAddress)
		var launchpadErr *sdkerrors.Error
		require.ErrorAs(t, err, &launchpadErr)
		codespace, code, _ := sdkerrors.ABCIInfo(launchpadErr, false)
		require.Equal(t, types.ErrZeroAddress.Codespace(), codespace)
		require.Equal(t, types.ErrZeroAddress.ABCICode(), code)
	})
	t.Run("corrupt record preserves keeper error", func(t *testing.T) {
		retiredAddress := address("query corrupt retired")
		e.ctx.KVStore(e.key).Set(types.RetiredKey(retiredAddress), []byte("{"))
		_, _, keeperErr := e.k.GetRetired(e.ctx, retiredAddress)
		require.ErrorIs(t, keeperErr, keeper.ErrInvalidMigrationRecord)
		response, err := client.Retired(goCtx, &types.QueryRetiredRequest{Address: retiredAddress.Hex()})
		require.Nil(t, response)
		require.ErrorIs(t, err, keeper.ErrInvalidMigrationRecord)
		require.EqualError(t, err, keeperErr.Error())
	})
}
