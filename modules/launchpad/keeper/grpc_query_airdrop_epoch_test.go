package keeper_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	storev2rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	storageconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
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
			request := &types.QueryAirdropEpochRequest{Pool: malformed, Epoch: 1}
			response, err := client.AirdropEpoch(goCtx, request)
			require.Nil(t, response)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.Contains(t, status.Convert(err).Message(), "invalid address")
			response, err = querier.AirdropEpoch(sdk.WrapSDKContext(e.ctx), request)
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
	scConfig := storageconfig.DefaultStateCommitConfig()
	scConfig.MemIAVLConfig.SnapshotInterval = 1
	scConfig.MemIAVLConfig.SnapshotMinTimeInterval = 0
	scConfig.MemIAVLConfig.AsyncCommitBuffer = 0
	cms := storev2rootmulti.NewStore(t.TempDir(), scConfig, storageconfig.StateStoreConfig{}, nil)
	app := baseapp.NewBaseApp(t.Name(), dbm.NewMemDB(), nil, nil, &testutil.TestAppOpts{})
	app.SetCMS(cms)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	app.MountStores(e.key)
	app.SetInterfaceRegistry(testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(app.GRPCQueryRouter(), keeper.NewQuerier(e.k))
	require.NoError(t, app.LoadLatestVersion())
	_, err := app.InitChain(context.Background(), &abci.RequestInitChain{})
	require.NoError(t, err)

	pool := address("query airdrop ABCI pool")
	ctx := app.NewContext(false, tmproto.Header{ChainID: app.ChainID, Height: 1})
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
	require.NoError(t, app.SetSealedStateToCommit(ctx, ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err = app.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), app.LastBlockHeight())
	queryCtx, err := app.CreateQueryContext(1, false)
	require.NoError(t, err)
	require.Equal(t, int64(1), queryCtx.BlockHeight())
	const path = "/paxprotocol.paxchain.launchpad.Query/AirdropEpoch"
	handler := app.GRPCQueryRouter().Route(path)
	require.NotNil(t, handler)

	for _, tc := range []struct {
		name    string
		pool    string
		epoch   uint64
		root    *sdkerrors.Error
		context string
	}{
		{"malformed pool", "0x1234", 1, sdkerrors.ErrInvalidRequest, "pool must be a 0x-prefixed 40-hex-digit address"},
		{"zero pool", (common.Address{}).Hex(), 1, types.ErrZeroAddress, "pool must not be the zero address"},
		{"zero epoch", pool.Hex(), 0, types.ErrInvalidAirdropEpoch, types.ErrInvalidAirdropEpoch.Error()},
		{"invalid epoch JSON", pool.Hex(), 1, types.ErrAirdropBasis, pool.Hex() + "/1"},
		{"invalid epoch record", pool.Hex(), 2, types.ErrAirdropBasis, "/0"},
		{"mismatched epoch", pool.Hex(), 3, types.ErrAirdropBasis, pool.Hex() + "/3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := &types.QueryAirdropEpochRequest{Pool: tc.pool, Epoch: tc.epoch}
			if tc.name == "malformed pool" {
				querier := keeper.NewQuerier(e.k)
				router := baseapp.NewQueryServerTestHelper(ctx, testkeeper.EVMTestApp.InterfaceRegistry())
				types.RegisterQueryServer(router, querier)
				response, err := types.NewQueryClient(router).AirdropEpoch(context.Background(), request)
				require.Nil(t, response)
				require.Equal(t, codes.InvalidArgument, status.Code(err))
				require.Equal(t, "pool must be a 0x-prefixed 40-hex-digit address: invalid address", status.Convert(err).Message())
				response, err = querier.AirdropEpoch(sdk.WrapSDKContext(ctx), request)
				require.Nil(t, response)
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				var addressErr *sdkerrors.Error
				require.ErrorAs(t, err, &addressErr)
				require.Same(t, sdkerrors.ErrInvalidAddress, addressErr)
				require.Contains(t, err.Error(), tc.context)
			}
			data, err := request.Marshal()
			require.NoError(t, err)
			_, routedErr := handler(queryCtx, abci.RequestQuery{Path: path, Data: data, Height: 1})
			require.Error(t, routedErr)
			codespace, code, log := sdkerrors.ABCIInfo(routedErr, false)
			if tc.name == "malformed pool" {
				require.ErrorIs(t, routedErr, sdkerrors.ErrInvalidAddress)
				require.Equal(t, codes.InvalidArgument, status.Code(routedErr))
				require.Equal(t, sdkerrors.UndefinedCodespace, codespace)
				require.Equal(t, uint32(1), code)
				require.Equal(t, "internal error", log)
			} else {
				require.ErrorIs(t, routedErr, tc.root)
				require.True(t, tc.root.Is(routedErr))
				var root *sdkerrors.Error
				require.ErrorAs(t, routedErr, &root)
				require.Same(t, tc.root, root)
				require.Equal(t, tc.root.Codespace(), codespace)
				require.Equal(t, tc.root.ABCICode(), code)
				require.Equal(t, routedErr.Error(), log)
				require.Contains(t, log, tc.context)
			}
			for _, height := range []int64{0, 1} {
				t.Run(fmt.Sprintf("height=%d", height), func(t *testing.T) {
					response, err := app.Query(context.Background(), &abci.RequestQuery{
						Path: path, Data: data, Height: height,
					})
					require.NoError(t, err)
					require.NotNil(t, response)
					wire, err := response.Marshal()
					require.NoError(t, err)
					var decoded abci.ResponseQuery
					require.NoError(t, decoded.Unmarshal(wire))
					require.Equal(t, *response, decoded)
					require.Equal(t, tc.root.Codespace(), decoded.Codespace)
					require.Equal(t, tc.root.ABCICode(), decoded.Code)
					require.Equal(t, int64(1), decoded.Height)
					require.Empty(t, decoded.Value)
					require.Nil(t, decoded.ProofOps)
					if tc.name == "malformed pool" {
						require.Equal(t, "invalid request", decoded.Log)
						require.NotContains(t, string(wire), tc.context)
					} else {
						require.Equal(t, log, decoded.Log)
						require.Contains(t, decoded.Log, tc.context)
					}
					require.Contains(t, decoded.Log, tc.root.Error())
				})
			}
		})
	}
}
