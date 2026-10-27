package keeper_test

import (
	"context"
	"encoding/binary"
	"strings"
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

func TestQueryAirdropEpochServes(t *testing.T) {
	e := newEnv(t)
	c := &custodyEnv{env: e, usdl: e.usdl, holder: reserve, caller: e.k.ModuleCaller()}
	roles := c.k.Generation2()
	pool := common.HexToAddress(generation2Market(roles.SidioraNFT, 1, nil).PoolAddress)
	c.height(c.ctx.BlockHeight() + 1)
	token := c.poolToken("Query Airdrop Token",
		holding{pool, 600}, holding{common.HexToAddress(types.DeadAddress), 100},
		holding{address("query airdrop holder"), 300})
	c.airdropMarket(token, 50_000, 0, 0)
	c.height(c.ctx.BlockHeight() + 10)

	router := baseapp.NewQueryServerTestHelper(c.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(c.k))
	client := types.NewQueryClient(router)
	goCtx := context.Background()

	response, err := client.AirdropEpoch(goCtx, &types.QueryAirdropEpochRequest{Pool: pool.Hex(), Epoch: 1})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.False(t, response.Found)
	require.Nil(t, response.AirdropEpoch)

	amount, err := c.k.TriggerAirdrop(c.ctx, c.caller, roles.FeesRouter, pool)
	require.NoError(t, err)
	require.Equal(t, "50000", amount.String())
	record, found, err := c.k.GetPoolAirdropEpoch(c.ctx, pool, 1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(c.ctx.BlockHeight()-1), record.SnapshotBlock)
	require.Equal(t, "300", record.EligibleSupply.String())
	info := types.AirdropEpochInfoOf(record)

	for _, encodedPool := range []string{pool.Hex(), strings.ToLower(pool.Hex())} {
		response, err = client.AirdropEpoch(goCtx, &types.QueryAirdropEpochRequest{Pool: encodedPool, Epoch: 1})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.True(t, response.Found)
		require.Equal(t, &info, response.AirdropEpoch)
	}

	response, err = client.AirdropEpoch(goCtx, &types.QueryAirdropEpochRequest{Pool: pool.Hex(), Epoch: 2})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.False(t, response.Found)
	require.Nil(t, response.AirdropEpoch)
}

func TestQueryAirdropEpochRefusals(t *testing.T) {
	e := newEnv(t)
	querier := keeper.NewQuerier(e.k)
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, querier)
	client := types.NewQueryClient(router)
	goCtx := context.Background()
	pool := address("query airdrop pool")

	t.Run("nil request", func(t *testing.T) {
		response, err := querier.AirdropEpoch(goCtx, nil)
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	for _, malformed := range []string{"", "0x1234", strings.Repeat("g", 42), pool.Hex()[2:]} {
		t.Run("malformed pool "+malformed, func(t *testing.T) {
			response, err := client.AirdropEpoch(goCtx, &types.QueryAirdropEpochRequest{Pool: malformed, Epoch: 1})
			require.Nil(t, response)
			require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
			var addressErr *sdkerrors.Error
			require.ErrorAs(t, err, &addressErr)
			require.Same(t, sdkerrors.ErrInvalidAddress, addressErr)
			codespace, code, _ := sdkerrors.ABCIInfo(addressErr, false)
			require.Equal(t, sdkerrors.ErrInvalidAddress.Codespace(), codespace)
			require.Equal(t, sdkerrors.ErrInvalidAddress.ABCICode(), code)
		})
	}

	t.Run("zero epoch", func(t *testing.T) {
		response, err := client.AirdropEpoch(goCtx, &types.QueryAirdropEpochRequest{Pool: pool.Hex(), Epoch: 0})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrInvalidAirdropEpoch)
		require.Same(t, types.ErrInvalidAirdropEpoch, err)
		codespace, code, _ := sdkerrors.ABCIInfo(err, false)
		require.Equal(t, types.ErrInvalidAirdropEpoch.Codespace(), codespace)
		require.Equal(t, types.ErrInvalidAirdropEpoch.ABCICode(), code)
	})

	t.Run("keeper error is preserved", func(t *testing.T) {
		key := append([]byte{0x26}, pool.Bytes()...)
		key = binary.BigEndian.AppendUint64(key, 1)
		e.ctx.KVStore(e.key).Set(key, []byte("invalid epoch JSON"))
		_, found, keeperErr := e.k.GetPoolAirdropEpoch(e.ctx, pool, 1)
		require.False(t, found)
		require.ErrorIs(t, keeperErr, types.ErrAirdropBasis)

		response, err := client.AirdropEpoch(goCtx, &types.QueryAirdropEpochRequest{Pool: pool.Hex(), Epoch: 1})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrAirdropBasis)
		require.EqualError(t, err, keeperErr.Error())
		require.IsType(t, keeperErr, err)
		var basisErr *sdkerrors.Error
		require.ErrorAs(t, err, &basisErr)
		require.Same(t, types.ErrAirdropBasis, basisErr)
		codespace, code, _ := sdkerrors.ABCIInfo(basisErr, false)
		require.Equal(t, types.ErrAirdropBasis.Codespace(), codespace)
		require.Equal(t, types.ErrAirdropBasis.ABCICode(), code)
	})
}
