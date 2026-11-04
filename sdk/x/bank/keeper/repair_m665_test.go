package keeper_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
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
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
)

// Run the actual integration methods, including their real query client and
// per-case keeper setup, within the task's focused test selector.
func TestRepairM_665_MetadataSuite(t *testing.T) {
	for _, method := range []struct {
		name string
		run  func(*IntegrationTestSuite)
	}{
		{"TestQueryDenomsMetadataRequest", (*IntegrationTestSuite).TestQueryDenomsMetadataRequest},
		{"TestQueryDenomMetadataRequest", (*IntegrationTestSuite).TestQueryDenomMetadataRequest},
	} {
		t.Run(method.name, func(t *testing.T) {
			s := new(IntegrationTestSuite)
			s.SetT(t)
			method.run(s)
		})
	}
}

func repairM665FactoryMetadata() types.Metadata {
	denom := newFactoryFooCoin(sdk.AccAddress([]byte("metadata____________")), 1).Denom
	return types.Metadata{
		Description: "Factory metadata with a slash-containing base denomination",
		Name:        "Factory Foo", Symbol: "FOO", Base: denom, Display: denom,
		DenomUnits: []*types.DenomUnit{{Denom: denom, Exponent: 0}},
	}
}

type repairM665Fixture struct {
	app   *baseapp.BaseApp
	bank  keeper.BaseKeeper
	ctx   sdk.Context
	amino *codec.LegacyAmino
	cdc   *codec.ProtoCodec
}

func repairM665Keeper(t *testing.T) repairM665Fixture {
	t.Helper()
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
	memoryKeys := sdk.NewMemoryStoreKeys(types.DeferredCacheStoreKey)
	deferredKey := memoryKeys[types.DeferredCacheStoreKey]
	db := dbm.NewMemDB()
	app := baseapp.NewBaseApp(t.Name(), db, nil, nil, &testutil.TestAppOpts{})
	sc := config.DefaultStateCommitConfig()
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	app.SetCMS(rootmulti.NewStore(t.TempDir(), sc, config.StateStoreConfig{Enable: false}, nil))
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	app.MountStores(bankKey, authKey, paramsKey, transientKey)
	app.MountMemoryStores(memoryKeys)
	ak := authkeeper.NewAccountKeeper(cdc, authKey,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, authtypes.ModuleName),
		authtypes.ProtoBaseAccount, map[string][]string{})
	bk := keeper.NewBaseKeeperWithDeferredCache(cdc, bankKey, ak,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, types.ModuleName),
		map[string]bool{}, deferredKey)
	app.QueryRouter().AddRoute(types.QuerierRoute, keeper.NewQuerier(bk, amino))
	require.NoError(t, app.LoadLatestVersion())
	ctx := app.NewUncachedContext(false, tmproto.Header{ChainID: app.ChainID, Height: 1})
	return repairM665Fixture{app: app, bank: bk, ctx: ctx, amino: amino, cdc: cdc}
}

func TestRepairM_665_InvariantCollectionError(t *testing.T) {
	f := repairM665Keeper(t)
	f.ctx, _ = f.ctx.CacheContext()
	invariant := keeper.TotalSupply(f.bank)
	message, broken := invariant(f.ctx)
	require.False(t, broken, message)

	coin := sdk.NewInt64Coin("brokenamount", 17)
	f.bank.SetSupply(f.ctx, coin)
	supply, err := keeper.CollectAllTotalSupply(f.ctx, f.bank)
	require.NoError(t, err)
	require.Equal(t, sdk.NewCoins(coin), supply)
	message, broken = invariant(f.ctx)
	require.True(t, broken, message) // A readable but unconserved supply also fails.

	store := prefix.NewStore(f.ctx.KVStore(f.bank.GetStoreKey()), types.SupplyKey)
	validAmount := append([]byte(nil), store.Get([]byte(coin.Denom))...)
	store.Set([]byte(coin.Denom), []byte("not-an-integer"))
	_, cause := keeper.CollectAllTotalSupply(f.ctx, f.bank)
	require.ErrorContains(t, cause, "unable to convert amount string to Int")
	message, broken = invariant(f.ctx)
	require.True(t, broken, "an unreadable supply cannot establish conservation: %s", message)
	require.Contains(t, message, "bank: query supply")
	require.Contains(t, message, cause.Error(), "preserve the collection failure's original cause")

	store.Set([]byte(coin.Denom), validAmount)
	supply, err = keeper.CollectAllTotalSupply(f.ctx, f.bank)
	require.NoError(t, err)
	require.Equal(t, sdk.NewCoins(coin), supply)
	f.bank.SetSupply(f.ctx, sdk.NewInt64Coin(coin.Denom, 0))
	message, broken = invariant(f.ctx)
	require.False(t, broken, message)
}

func TestRepairM_665_LegacySupplyRESTPages(t *testing.T) {
	s := new(IntegrationTestSuite)
	s.SetT(t)
	s.TestQuerier_QueryTotalSupplyRESTPagination()
}

func repairM665SupplyPages(t *testing.T) {
	t.Helper()
	f := repairM665Keeper(t)
	wantSupply := sdk.NewCoins(
		sdk.NewInt64Coin("alpha", 11), sdk.NewInt64Coin("bravo", 22),
		sdk.NewInt64Coin("charlie", 33), sdk.NewInt64Coin("delta", 44),
		sdk.NewInt64Coin("echo", 55),
	)
	for _, coin := range wantSupply {
		f.bank.SetSupply(f.ctx, coin)
	}
	f.app.CommitMultiStore().Commit(true)
	require.Equal(t, int64(1), f.app.LastBlockHeight())
	ctx, err := f.app.CreateQueryContext(1, false)
	require.NoError(t, err)

	// The loopback RPC peer delegates to BaseApp's real custom-query router.
	// REST must emit the request; neither its wire shape nor response is fabricated.
	rpcMux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(rpcMux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(ctx context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			res, err := f.app.Query(ctx, &abci.RequestQuery{
				Path: req.Path, Data: req.Data, Height: int64(req.Height), Prove: req.Prove,
			})
			if err != nil {
				return nil, err
			}
			return &coretypes.ResultABCIQuery{Response: *res}, nil
		}),
	})
	rpc := httptest.NewServer(rpcMux)
	t.Cleanup(rpc.Close)
	node, err := rpchttp.New(rpc.URL)
	require.NoError(t, err)
	clientCtx := client.Context{}.WithLegacyAmino(f.amino).WithCodec(f.cdc).WithClient(node)
	router := mux.NewRouter()
	bankrest.RegisterHandlers(clientCtx, router)
	httpServer := httptest.NewServer(router)
	t.Cleanup(httpServer.Close)
	querier := keeper.NewQuerier(f.bank, f.amino)
	var combined sdk.Coins
	const limit = 2 // Deliberately different from query.DefaultLimit.
	for page := 1; page <= 3; page++ {
		t.Run(fmt.Sprintf("page_%d_limit_%d", page, limit), func(t *testing.T) {
			resp, err := httpServer.Client().Get(fmt.Sprintf("%s/bank/total?page=%d&limit=%d&height=1", httpServer.URL, page, limit))
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
			var wrapped rest.ResponseWithHeight
			require.NoError(t, f.amino.UnmarshalAsJSON(body, &wrapped))
			require.Equal(t, int64(1), wrapped.Height)
			var actual types.QueryTotalSupplyResponse
			require.NoError(t, f.amino.UnmarshalAsJSON(wrapped.Result, &actual))

			offset := (page - 1) * limit
			params := types.QueryTotalSupplyRequest{Pagination: &query.PageRequest{
				Offset: uint64(offset), Limit: limit, CountTotal: true,
			}}
			data := f.amino.MustMarshalJSON(params)
			result, err := querier(ctx, []string{types.QueryTotalSupply}, abci.RequestQuery{
				Path: "custom/" + types.QuerierRoute + "/" + types.QueryTotalSupply, Data: data,
			})
			require.NoError(t, err)
			var direct types.QueryTotalSupplyResponse
			require.NoError(t, f.amino.UnmarshalAsJSON(result, &direct))
			require.Equal(t, direct, actual, "REST and direct custom queries must agree, including pagination")
			grpc, err := f.bank.TotalSupply(sdk.WrapSDKContext(ctx), &params)
			require.NoError(t, err)
			require.Equal(t, grpc.Supply, actual.Supply)
			require.Equal(t, grpc.Pagination, actual.Pagination)
			end := offset + limit
			if end > len(wantSupply) {
				end = len(wantSupply)
			}
			require.Equal(t, wantSupply[offset:end], actual.Supply)
			require.NotNil(t, actual.Pagination)
			require.Equal(t, uint64(len(wantSupply)), actual.Pagination.Total)
			if end < len(wantSupply) {
				require.Equal(t, []byte(wantSupply[end].Denom), actual.Pagination.NextKey)
			} else {
				require.Empty(t, actual.Pagination.NextKey)
			}
			combined = append(combined, actual.Supply...)
		})
	}
	require.Equal(t, wantSupply, combined, "pages must cover all denoms exactly once")
}
