package keeper_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
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
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, sdkerrors.ErrInvalidAddress.Codespace(), codespace)
			require.Equal(t, sdkerrors.ErrInvalidAddress.ABCICode(), code)
		})
	}

	t.Run("zero pool", func(t *testing.T) {
		response, err := client.AirdropEpoch(goCtx, &types.QueryAirdropEpochRequest{Pool: (common.Address{}).Hex(), Epoch: 1})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrZeroAddress)
		require.Contains(t, err.Error(), "pool must not be the zero address")
		codespace, code, _ := sdkerrors.ABCIInfo(err, false)
		require.Equal(t, types.ErrZeroAddress.Codespace(), codespace)
		require.Equal(t, types.ErrZeroAddress.ABCICode(), code)
	})

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
		require.IsType(t, keeperErr, errors.Unwrap(err))
		require.EqualError(t, errors.Unwrap(err), keeperErr.Error())
		require.True(t, types.ErrAirdropBasis.Is(err))
		var basisErr *sdkerrors.Error
		require.ErrorAs(t, err, &basisErr)
		require.Same(t, types.ErrAirdropBasis, basisErr)
		codespace, code, _ := sdkerrors.ABCIInfo(err, false)
		require.Equal(t, types.ErrAirdropBasis.Codespace(), codespace)
		require.Equal(t, types.ErrAirdropBasis.ABCICode(), code)
	})
}

func TestQueryAirdropEpochABCICodes(t *testing.T) {
	e := newEnv(t)
	app := baseapp.NewBaseApp(t.Name(), dbm.NewMemDB(), nil, nil, &testutil.TestAppOpts{})
	app.MountStores(e.key)
	app.SetInterfaceRegistry(testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(app.GRPCQueryRouter(), keeper.NewQuerier(e.k))
	require.NoError(t, app.LoadLatestVersion())

	pool := address("query airdrop ABCI pool")
	store := app.CommitMultiStore().CacheMultiStore()
	ctx := sdk.NewContext(store, tmproto.Header{Height: 1}, false)
	mismatchedRecord, err := json.Marshal(types.PoolAirdropEpoch{
		Pool: pool.Hex(), Epoch: 4, Amount: sdk.OneInt(), EligibleSupply: sdk.OneInt(),
		Remaining: sdk.OneInt(), ClaimedSupply: sdk.ZeroInt(), LegacyRemaining: sdk.ZeroInt(),
	})
	require.NoError(t, err)
	for i, data := range [][]byte{[]byte("invalid epoch JSON"), []byte("{}"), mismatchedRecord} {
		key := append([]byte{0x26}, pool.Bytes()...)
		key = binary.BigEndian.AppendUint64(key, uint64(i+1))
		ctx.KVStore(e.key).Set(key, data)
	}
	require.NoError(t, app.SetSealedStateToCommit(ctx, store))
	_, err = app.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), app.LastBlockHeight())

	for _, tc := range []struct {
		name    string
		pool    string
		epoch   uint64
		root    *sdkerrors.Error
		context string
	}{
		{"malformed pool", "0x1234", 1, sdkerrors.ErrInvalidAddress, "pool must be a 0x-prefixed 40-hex-digit address"},
		{"zero pool", (common.Address{}).Hex(), 1, types.ErrZeroAddress, "pool must not be the zero address"},
		{"zero epoch", pool.Hex(), 0, types.ErrInvalidAirdropEpoch, types.ErrInvalidAirdropEpoch.Error()},
		{"invalid epoch JSON", pool.Hex(), 1, types.ErrAirdropBasis, pool.Hex() + "/1"},
		{"invalid epoch record", pool.Hex(), 2, types.ErrAirdropBasis, "/0"},
		{"mismatched epoch", pool.Hex(), 3, types.ErrAirdropBasis, pool.Hex() + "/3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := &types.QueryAirdropEpochRequest{Pool: tc.pool, Epoch: tc.epoch}
			data, err := request.Marshal()
			require.NoError(t, err)
			response, err := app.Query(context.Background(), &abci.RequestQuery{
				Path: "/paxprotocol.paxchain.launchpad.Query/AirdropEpoch", Data: data, Height: 1,
			})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, tc.root.Codespace(), response.Codespace)
			require.Equal(t, tc.root.ABCICode(), response.Code)
			require.Equal(t, int64(1), response.Height)
			require.Empty(t, response.Value)
			require.Contains(t, response.Log, tc.context)
			require.Contains(t, response.Log, tc.root.Error())
		})
	}
}
