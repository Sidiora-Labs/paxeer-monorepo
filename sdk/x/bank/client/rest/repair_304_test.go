package rest_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/rest"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankrest "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/client/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
)

func repair304SupplyServer(t *testing.T) (*mux.Router, client.Context, keeper.BaseKeeper, sdk.Context, sdk.Coins) {
	t.Helper()
	legacyAmino := codec.NewLegacyAmino()
	types.RegisterLegacyAminoCodec(legacyAmino)
	authtypes.RegisterLegacyAminoCodec(legacyAmino)
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	bankKey := sdk.NewKVStoreKey(types.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	app := baseapp.NewBaseApp(t.Name(), db, nil, nil, &testutil.TestAppOpts{})
	app.MountStores(bankKey, authKey, paramsKey, transientKey)
	ak := authkeeper.NewAccountKeeper(cdc, authKey,
		paramtypes.NewSubspace(cdc, legacyAmino, paramsKey, transientKey, authtypes.ModuleName),
		authtypes.ProtoBaseAccount, map[string][]string{})
	bk := keeper.NewBaseKeeper(cdc, bankKey, ak,
		paramtypes.NewSubspace(cdc, legacyAmino, paramsKey, transientKey, types.ModuleName),
		map[string]bool{})
	app.QueryRouter().AddRoute(types.QuerierRoute, keeper.NewQuerier(bk, legacyAmino))
	require.NoError(t, app.LoadLatestVersion())
	ctx := app.NewUncachedContext(false, tmproto.Header{ChainID: app.ChainID, Height: 1})
	supply := sdk.NewCoins()
	for i := 0; i < query.DefaultLimit+7; i++ {
		coin := sdk.NewInt64Coin(fmt.Sprintf("denom%03d", i), int64(i+1))
		bk.SetSupply(ctx, coin)
		supply = append(supply, coin)
	}
	app.CommitMultiStore().Commit(true)
	require.Equal(t, int64(1), app.LastBlockHeight())
	ctx, err := app.CreateQueryContext(1, false)
	require.NoError(t, err)

	rpcMux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(rpcMux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(ctx context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			res, err := app.Query(ctx, &abci.RequestQuery{
				Path: req.Path, Data: req.Data, Height: int64(req.Height), Prove: req.Prove,
			})
			if err != nil {
				return nil, err
			}
			return &coretypes.ResultABCIQuery{Response: *res}, nil
		}),
	})
	rpcServer := httptest.NewServer(rpcMux)
	t.Cleanup(rpcServer.Close)
	rpcClient, err := rpchttp.New(rpcServer.URL)
	require.NoError(t, err)
	clientCtx := client.Context{}.WithLegacyAmino(legacyAmino).WithCodec(cdc).WithClient(rpcClient)
	router := mux.NewRouter()
	bankrest.RegisterHandlers(clientCtx, router)
	return router, clientCtx, bk, ctx, supply
}

func TestRepair_304_TotalSupplyPagination(t *testing.T) {
	router, clientCtx, bk, ctx, supply := repair304SupplyServer(t)
	testCases := []struct {
		name   string
		args   string
		offset int
		limit  int
	}{
		{"default_page", "", 0, query.DefaultLimit},
		{"default_limit_page_two", "?page=2", query.DefaultLimit, query.DefaultLimit},
		{"page_two_limit_two", "?page=2&limit=2", 2, 2},
		{"page_two_limit_three", "?page=2&limit=3", 3, 3},
		{"default_page_limit_three", "?limit=3", 0, 3},
		{"last_page", "?page=36&limit=3", 105, 3},
		{"past_last_page", "?page=37&limit=3", 108, 3},
		{"maximum_limit", fmt.Sprintf("?limit=%d", query.MaxLimit), 0, int(query.MaxLimit)},
		{"maximum_offset", fmt.Sprintf("?page=%d&limit=1", query.MaxOffset+1), int(query.MaxOffset), 1},
		{"explicit_height", "?page=2&limit=3&height=1", 3, 3},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/bank/total"+tc.args, nil))
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			var wrapped rest.ResponseWithHeight
			require.NoError(t, clientCtx.LegacyAmino.UnmarshalAsJSON(recorder.Body.Bytes(), &wrapped))
			require.Equal(t, int64(1), wrapped.Height)
			var got types.QueryTotalSupplyResponse
			require.NoError(t, clientCtx.LegacyAmino.UnmarshalAsJSON(wrapped.Result, &got))
			direct, err := bk.TotalSupply(sdk.WrapSDKContext(ctx), &types.QueryTotalSupplyRequest{
				Pagination: &query.PageRequest{Offset: uint64(tc.offset), Limit: uint64(tc.limit), CountTotal: true},
			})
			require.NoError(t, err)
			directJSON, err := clientCtx.LegacyAmino.MarshalAsJSON(direct)
			require.NoError(t, err)
			var expected types.QueryTotalSupplyResponse
			require.NoError(t, clientCtx.LegacyAmino.UnmarshalAsJSON(directJSON, &expected))
			require.Equal(t, expected, got)
			start := tc.offset
			if start > len(supply) {
				start = len(supply)
			}
			end := start + tc.limit
			if end > len(supply) {
				end = len(supply)
			}
			if start == end {
				require.Nil(t, got.Supply)
			} else {
				require.Equal(t, supply[start:end], got.Supply)
			}
			require.NotNil(t, got.Pagination)
			require.Equal(t, uint64(len(supply)), got.Pagination.Total)
			if end < len(supply) {
				require.Equal(t, []byte(supply[end].Denom), got.Pagination.NextKey)
			} else {
				require.Empty(t, got.Pagination.NextKey)
			}
		})
	}
}

func TestRepair_304_InvalidPagination(t *testing.T) {
	router, _, _, _, _ := repair304SupplyServer(t)
	testCases := []struct {
		args    string
		message string
	}{
		{"?page=0", "page must greater than 0"},
		{"?page=-1", "page must greater than 0"},
		{"?page=invalid", "invalid syntax"},
		{"?limit=0", "limit must greater than 0"},
		{"?limit=-1", "limit must greater than 0"},
		{"?limit=invalid", "invalid syntax"},
		{fmt.Sprintf("?limit=%d", query.MaxLimit+1), "exceeds maximum allowed limit"},
		{fmt.Sprintf("?page=%d&limit=1", query.MaxOffset+2), "page exceeds maximum pagination offset"},
		{"?page=" + strconv.Itoa(int(^uint(0)>>1)) + "&limit=3", "page exceeds maximum pagination offset"},
	}
	for _, tc := range testCases {
		t.Run(tc.args, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/bank/total"+tc.args, nil))
			require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), tc.message)
		})
	}
}
