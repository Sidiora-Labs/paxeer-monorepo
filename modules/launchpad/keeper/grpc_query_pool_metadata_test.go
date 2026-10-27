package keeper_test

import (
	"context"
	"strings"
	"testing"
	"time"

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

func TestQueryPoolMetadataServes(t *testing.T) {
	e := newEnv(t)
	creator := e.account(200_000_000)
	first := e.create(creator, types.FeeStrategyClaim)
	e.at(time.Minute)
	e.ctx = e.ctx.WithBlockHeight(9)
	second := e.create(creator, types.FeeStrategyAirdrop)
	querier := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, querier)
	client := types.NewQueryClient(router)

	for _, market := range []types.Market{first, second} {
		t.Run(market.PoolAddress, func(t *testing.T) {
			pool := common.HexToAddress(market.PoolAddress)
			metadata, found, err := e.k.GetPoolMetadata(e.ctx, pool)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, e.evmAddress(creator), metadata.Creator)
			require.Equal(t, common.HexToAddress(market.TokenAddress), metadata.Token)
			require.Equal(t, market.FeeRightID, metadata.NFTID)
			want := &types.PoolMetadataInfo{
				Creator: metadata.Creator.Hex(), Token: metadata.Token.Hex(), Optical: metadata.Optical.Hex(),
				NFTID: metadata.NFTID, CreatedAt: metadata.CreatedAt, CreatedBlock: metadata.CreatedBlock,
			}
			for _, input := range []string{pool.Hex(), strings.ToLower(pool.Hex()), "0x" + strings.ToUpper(pool.Hex()[2:])} {
				response, err := client.PoolMetadata(context.Background(), &types.QueryPoolMetadataRequest{Pool: input})
				require.NoError(t, err)
				require.NotNil(t, response)
				require.True(t, response.Found)
				require.Equal(t, want, response.Metadata)
			}
		})
	}

	response, err := client.PoolMetadata(context.Background(), &types.QueryPoolMetadataRequest{Pool: first.PoolAddress})
	require.NoError(t, err)
	require.Equal(t, genesisTime.Unix(), response.Metadata.CreatedAt)
	require.Equal(t, int64(1), response.Metadata.CreatedBlock)
	response, err = client.PoolMetadata(context.Background(), &types.QueryPoolMetadataRequest{Pool: second.PoolAddress})
	require.NoError(t, err)
	require.Equal(t, genesisTime.Add(time.Minute).Unix(), response.Metadata.CreatedAt)
	require.Equal(t, int64(9), response.Metadata.CreatedBlock)
	require.NotEqual(t, first.FeeRightID, second.FeeRightID)

	response, err = client.PoolMetadata(context.Background(), &types.QueryPoolMetadataRequest{Pool: address("unregistered metadata pool").Hex()})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.False(t, response.Found)
	require.Nil(t, response.Metadata)
}

func TestQueryPoolMetadataRefusals(t *testing.T) {
	e := newEnv(t)
	querier := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, querier)
	client := types.NewQueryClient(router)

	t.Run("nil request", func(t *testing.T) {
		for _, goCtx := range []context.Context{context.Background(), sdk.WrapSDKContext(e.ctx)} {
			response, err := querier.PoolMetadata(goCtx, nil)
			require.Nil(t, response)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.Equal(t, "empty request", status.Convert(err).Message())
		}
	})

	for _, input := range []string{
		"", "not-an-address", "0x1234", strings.TrimPrefix(address("pool").Hex(), "0x"),
		"0x" + strings.Repeat("g", 40), "0x" + strings.Repeat("1", 42),
	} {
		t.Run("malformed/"+input, func(t *testing.T) {
			response, err := client.PoolMetadata(context.Background(), &types.QueryPoolMetadataRequest{Pool: input})
			require.Nil(t, response)
			require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
			require.Contains(t, err.Error(), "pool")
			var addressErr *sdkerrors.Error
			require.ErrorAs(t, err, &addressErr)
			codespace, code, _ := sdkerrors.ABCIInfo(addressErr, false)
			require.Equal(t, sdkerrors.ErrInvalidAddress.Codespace(), codespace)
			require.Equal(t, sdkerrors.ErrInvalidAddress.ABCICode(), code)
		})
	}

	t.Run("zero address", func(t *testing.T) {
		response, err := client.PoolMetadata(context.Background(), &types.QueryPoolMetadataRequest{Pool: (common.Address{}).Hex()})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrZeroAddress)
	})

	t.Run("keeper error reaches the caller unchanged", func(t *testing.T) {
		creator := e.account(200_000_000)
		market := e.create(creator, types.FeeStrategyClaim)
		pool := common.HexToAddress(market.PoolAddress)
		e.ctx.KVStore(e.key).Set(append([]byte{0x51}, pool.Bytes()...), []byte("{"))
		_, found, keeperErr := e.k.GetPoolMetadata(e.ctx, pool)
		require.False(t, found)
		require.ErrorIs(t, keeperErr, types.ErrInvalidMarket)
		router.Ctx = e.ctx
		response, err := client.PoolMetadata(context.Background(), &types.QueryPoolMetadataRequest{Pool: pool.Hex()})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrInvalidMarket)
		require.EqualError(t, err, keeperErr.Error())
		var launchpadErr *sdkerrors.Error
		require.ErrorAs(t, err, &launchpadErr)
		codespace, code, _ := sdkerrors.ABCIInfo(launchpadErr, false)
		require.Equal(t, types.ErrInvalidMarket.Codespace(), codespace)
		require.Equal(t, types.ErrInvalidMarket.ABCICode(), code)
	})
}
