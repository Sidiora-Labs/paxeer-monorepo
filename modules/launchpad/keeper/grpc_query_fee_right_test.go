package keeper_test

import (
	"context"
	"encoding/binary"
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

func TestQueryFeeRightServes(t *testing.T) {
	e := newEnv(t)
	collection := e.k.Generation2().SidioraNFT
	owner, next, approved := address("query fee right owner"), address("query fee right next"), address("query fee right approved")
	legacy := address("query fee right legacy holder")
	market := generation2Market(collection, 1, sdk.AccAddress(legacy.Bytes()))
	gs := types.DefaultGenesis()
	gs.Markets = []types.Market{market}
	e.k.InitGenesis(e.ctx, *gs)
	id, err := e.k.MintFeeRight(e.ctx, collection, owner, common.HexToAddress(market.PoolAddress), types.FeeStrategyClaim)
	require.NoError(t, err)
	require.Equal(t, market.FeeRightID, id)
	require.NoError(t, e.k.ApproveFeeRight(e.ctx, collection, owner, approved, id))

	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)
	request := &types.QueryFeeRightRequest{Collection: collection.Hex(), Id: id}
	assertRight := func(wantOwner common.Address) {
		t.Helper()
		right, found, err := e.k.GetFeeRight(e.ctx, collection, id)
		require.NoError(t, err)
		require.True(t, found)
		response, err := client.FeeRight(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, response)
		require.True(t, response.Found)
		want := types.FeeRightInfoOf(right)
		require.Equal(t, &want, response.FeeRight)
		actualOwner, err := e.k.FeeRightOwnerOf(e.ctx, collection, id)
		require.NoError(t, err)
		require.Equal(t, wantOwner, actualOwner)
		require.Equal(t, actualOwner.Hex(), response.FeeRight.Owner)
		require.NotEqual(t, legacy.Hex(), response.FeeRight.Owner)
		require.Equal(t, market.PoolAddress, response.FeeRight.Pool)
	}
	assertRight(owner)
	request.Collection = strings.ToLower(collection.Hex())
	assertRight(owner)
	require.NoError(t, e.k.TransferFeeRight(e.ctx, collection, owner, owner, next, id))
	assertRight(next)

	for _, request := range []*types.QueryFeeRightRequest{
		{Collection: collection.Hex(), Id: id + 1},
		{Collection: collection.Hex(), Id: 0},
		{Collection: address("query fee right other collection").Hex(), Id: id},
	} {
		response, err := client.FeeRight(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, response)
		require.False(t, response.Found)
		require.Nil(t, response.FeeRight)
	}
}

func TestQueryFeeRightRefusals(t *testing.T) {
	e := newEnv(t)
	server := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, server)
	client := types.NewQueryClient(router)

	response, err := server.FeeRight(context.Background(), nil)
	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	for _, collection := range []string{"", "invalid", "0x1234", strings.Repeat("1", 40), "0x" + strings.Repeat("g", 40)} {
		t.Run(collection, func(t *testing.T) {
			response, err := client.FeeRight(context.Background(), &types.QueryFeeRightRequest{Collection: collection, Id: 1})
			require.Nil(t, response)
			require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, sdkerrors.ErrInvalidAddress.Codespace(), codespace)
			require.Equal(t, sdkerrors.ErrInvalidAddress.ABCICode(), code)
		})
	}

	response, err = client.FeeRight(context.Background(), &types.QueryFeeRightRequest{Collection: common.Address{}.Hex(), Id: 1})
	require.Nil(t, response)
	require.ErrorIs(t, err, types.ErrZeroAddress)
	codespace, code, _ := sdkerrors.ABCIInfo(err, false)
	require.Equal(t, types.ModuleName, codespace)
	require.Equal(t, types.ErrZeroAddress.ABCICode(), code)

	collection := e.k.Generation2().SidioraNFT
	id, err := e.k.MintFeeRight(e.ctx, collection, address("query corrupt owner"), address("query corrupt pool"), types.FeeStrategyClaim)
	require.NoError(t, err)
	key := append([]byte{0x40}, collection.Bytes()...)
	key = binary.BigEndian.AppendUint64(key, id)
	e.ctx.KVStore(e.key).Set(key, []byte("{"))
	_, _, keeperErr := e.k.GetFeeRight(e.ctx, collection, id)
	require.ErrorIs(t, keeperErr, types.ErrOverflow)
	response, err = client.FeeRight(context.Background(), &types.QueryFeeRightRequest{Collection: collection.Hex(), Id: id})
	require.Nil(t, response)
	require.EqualError(t, err, keeperErr.Error())
	require.ErrorIs(t, err, types.ErrOverflow)
	codespace, code, _ = sdkerrors.ABCIInfo(err, false)
	require.Equal(t, types.ModuleName, codespace)
	require.Equal(t, types.ErrOverflow.ABCICode(), code)
}
