package rest_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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
	storev2rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
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
	paxdbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
)

func TestRepairM_664_NetworkSignature(t *testing.T) {
	source, err := os.ReadFile("query_test.go")
	require.NoError(t, err)
	require.Contains(t, string(source), "//go:build norace\n// +build norace\n")
	file, err := parser.ParseFile(token.NewFileSet(), "query_test.go", source, 0)
	require.NoError(t, err)
	var setup *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "SetupSuite" {
			setup = fn
		}
	}
	require.NotNil(t, setup)
	constructorCalls := 0
	ast.Inspect(setup.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "DefaultConfig" {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "network", pkg.Name)
		require.Len(t, call.Args, 1)
		handle, ok := call.Args[0].(*ast.CallExpr)
		require.True(t, ok, "DefaultConfig must receive the suite testing handle")
		require.Empty(t, handle.Args)
		method, ok := handle.Fun.(*ast.SelectorExpr)
		require.True(t, ok)
		require.Equal(t, "T", method.Sel.Name)
		receiver, ok := method.X.(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "s", receiver.Name)
		constructorCalls++
		return true
	})
	require.Equal(t, 1, constructorCalls)
}

func TestRepairM_664_BankRESTQueries(t *testing.T) {
	amino := codec.NewLegacyAmino()
	types.RegisterLegacyAminoCodec(amino)
	authtypes.RegisterLegacyAminoCodec(amino)
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
	scConfig := paxdbconfig.DefaultStateCommitConfig()
	scConfig.MemIAVLConfig.AsyncCommitBuffer = 0
	scConfig.MemIAVLConfig.SnapshotInterval = 1
	scConfig.MemIAVLConfig.SnapshotMinTimeInterval = 0
	ssConfig := paxdbconfig.DefaultStateStoreConfig()
	ssConfig.AsyncWriteBuffer = 0
	store := storev2rootmulti.NewStore(t.TempDir(), scConfig, ssConfig, nil)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	app.SetCMS(store)
	app.MountStores(bankKey, authKey, paramsKey, transientKey)
	ak := authkeeper.NewAccountKeeper(cdc, authKey,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, authtypes.ModuleName),
		authtypes.ProtoBaseAccount, map[string][]string{})
	bk := keeper.NewBaseKeeper(cdc, bankKey, ak,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, types.ModuleName), map[string]bool{})
	app.QueryRouter().AddRoute(types.QuerierRoute, keeper.NewQuerier(bk, amino))
	require.NoError(t, app.LoadLatestVersion())
	ctx := app.NewUncachedContext(false, tmproto.Header{ChainID: app.ChainID, Height: 1})
	address := sdk.AccAddress([]byte("repair-m664-account1"))
	coins := sdk.NewCoins(sdk.NewInt64Coin("node0token", 1000), sdk.NewInt64Coin("stake", 400))
	ak.SetAccount(ctx, authtypes.NewBaseAccount(address, nil, 0, 0))
	genesis := types.DefaultGenesisState()
	genesis.Balances = []types.Balance{{Address: address.String(), Coins: coins}}
	genesis.Supply = coins
	genesisBytes, err := cdc.MarshalAsJSON(genesis)
	require.NoError(t, err)
	var decodedGenesis types.GenesisState
	require.NoError(t, cdc.UnmarshalAsJSON(genesisBytes, &decodedGenesis))
	bk.InitGenesis(ctx, &decodedGenesis)
	app.CommitMultiStore().Commit(true)
	require.Equal(t, int64(1), app.LastBlockHeight())
	require.Eventually(t, func() bool {
		return store.GetStateStore().GetLatestVersion() == 1
	}, 10*time.Second, time.Millisecond, "height one must reach the query state store before REST queries")
	committedCtx, err := app.CreateQueryContext(1, false)
	require.NoError(t, err)
	require.Equal(t, int64(1), committedCtx.BlockHeight())
	require.Equal(t, coins, bk.GetAllBalances(committedCtx, address))

	rpcMux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(rpcMux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(ctx context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			response, err := app.Query(ctx, &abci.RequestQuery{
				Path: req.Path, Data: req.Data, Height: int64(req.Height), Prove: req.Prove,
			})
			if err != nil {
				return nil, err
			}
			return &coretypes.ResultABCIQuery{Response: *response}, nil
		}),
	})
	rpcServer := httptest.NewServer(rpcMux)
	t.Cleanup(rpcServer.Close)
	rpcClient, err := rpchttp.New(rpcServer.URL)
	require.NoError(t, err)
	clientCtx := client.Context{}.WithCodec(cdc).WithLegacyAmino(amino).WithClient(rpcClient)
	router := mux.NewRouter()
	bankrest.RegisterHandlers(clientCtx, router)
	api := httptest.NewServer(router)
	t.Cleanup(api.Close)

	cases := []struct {
		name string
		path string
		got  interface{}
		want interface{}
	}{
		{"all_balances", "/bank/balances/" + address.String(), &sdk.Coins{}, &coins},
		{"balances_at_height", "/bank/balances/" + address.String() + "?height=1", &sdk.Coins{}, &coins},
		{"balance_by_denom", "/bank/balances/" + address.String() + "?denom=stake", &sdk.Coin{}, repairM664Coin("stake", 400)},
		{"missing_balance", "/bank/balances/" + address.String() + "?denom=foobar", &sdk.Coin{}, repairM664Coin("foobar", 0)},
		{"total_supply", "/bank/total?height=1", &types.QueryTotalSupplyResponse{}, &types.QueryTotalSupplyResponse{Supply: coins, Pagination: &query.PageResponse{Total: 2}}},
		{"supply_by_denom", "/bank/total/stake?height=1", &sdk.Coin{}, repairM664Coin("stake", 400)},
		{"missing_supply", "/bank/total/foobar?height=1", &sdk.Coin{}, repairM664Coin("foobar", 0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := api.Client().Get(api.URL + tc.path)
			require.NoError(t, err)
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, res.StatusCode, string(body))
			var response rest.ResponseWithHeight
			require.NoError(t, amino.UnmarshalAsJSON(body, &response))
			require.Equal(t, int64(1), response.Height)
			require.NoError(t, amino.UnmarshalAsJSON(response.Result, tc.got))
			require.Equal(t, tc.want, tc.got)
		})
	}
}

func repairM664Coin(denom string, amount int64) *sdk.Coin {
	coin := sdk.NewInt64Coin(denom, amount)
	return &coin
}
