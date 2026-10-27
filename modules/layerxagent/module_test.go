package layerxagent_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gogo/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
	"google.golang.org/grpc"
)

func agentModuleContext() (sdk.Context, keeper.Keeper) {
	app := testkeeper.EVMTestApp
	ctx, _ := app.NewContext(false, tmtypes.Header{}).CacheContext()
	parent := ctx.MultiStore()
	giga := parent.(storetypes.GigaMultiStore)
	stores := map[storetypes.StoreKey]storetypes.CacheWrapper{}
	keys := map[string]storetypes.StoreKey{}
	for _, key := range parent.StoreKeys() {
		if giga.IsStoreGiga(key) {
			stores[key] = giga.GetGigaKVStore(key)
		} else {
			stores[key] = parent.GetKVStore(key)
		}
		keys[key.Name()] = key
	}
	key := sdk.NewKVStoreKey(types.StoreKey)
	stores[key] = dbadapter.Store{DB: tmdb.NewMemDB()}
	keys[key.Name()] = key
	ctx = ctx.WithMultiStore(cachemulti.NewStore(tmdb.NewMemDB(), stores, keys, nil, nil, nil, 0))
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	return ctx, keeper.NewKeeper(key, &app.EvmKeeper, precompiles.MergeUpgrade)
}

func TestAgentAppModuleGenesisAndServices(t *testing.T) {
	basic := layerxagent.NewAppModuleBasic()
	require.Equal(t, "layerxagent", basic.Name())
	registry := cdctypes.NewInterfaceRegistry()
	basic.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	authority := sdk.AccAddress(common.HexToAddress("0xa11").Bytes()).String()
	msg := &types.MsgUpdateParams{Authority: authority, Params: types.DefaultParams()}
	packed, err := cdctypes.NewAnyWithValue(msg)
	require.NoError(t, err)
	var unpacked sdk.Msg
	require.NoError(t, cdc.UnpackAny(packed, &unpacked))
	require.Equal(t, msg, unpacked)
	amino := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(amino)
	basic.RegisterLegacyAminoCodec(amino)
	var aminoMsg sdk.Msg
	require.NoError(t, amino.UnmarshalAsJSON(amino.MustMarshalJSON(msg), &aminoMsg))
	require.Equal(t, msg, aminoMsg)

	defaults := basic.DefaultGenesis(cdc)
	var defaultState types.GenesisState
	cdc.MustUnmarshalJSON(defaults, &defaultState)
	expectedDefaultState := types.DefaultGenesis()
	expectedDefaultState.Jobs = []types.Job{}
	require.Equal(t, expectedDefaultState, &defaultState)
	require.NoError(t, basic.ValidateGenesis(cdc, nil, defaults))
	malformed := json.RawMessage(`{"params":`)
	require.ErrorContains(t, basic.ValidateGenesis(cdc, nil, malformed), "failed to unmarshal layerxagent genesis state")
	invalid := *types.DefaultGenesis()
	invalid.NextJobId = 0
	require.ErrorContains(t, basic.ValidateGenesis(cdc, nil, cdc.MustMarshalJSON(&invalid)), "next job id must be positive")
	invalid = *types.DefaultGenesis()
	invalid.Params.MaxJobsPerBlock = 0
	require.Error(t, basic.ValidateGenesis(cdc, nil, cdc.MustMarshalJSON(&invalid)))
	for _, tc := range []struct {
		name  string
		chunk json.RawMessage
		valid bool
	}{
		{"valid stream", defaults, true},
		{"invalid stream", malformed, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := make(chan json.RawMessage, 2)
			stream <- defaults
			stream <- tc.chunk
			close(stream)
			err := basic.ValidateGenesisStream(cdc, nil, stream)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
	queryCmd := basic.GetQueryCmd()
	require.NotNil(t, queryCmd)
	require.Equal(t, types.ModuleName, queryCmd.Name())
	for _, name := range []string{"params", "job", "pending"} {
		cmd, _, err := queryCmd.Find([]string{name})
		require.NoError(t, err)
		require.Equal(t, name, cmd.Name())
	}
	gateway := runtime.NewServeMux()
	basic.RegisterGRPCGatewayRoutes(client.Context{}, gateway)
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/paxprotocol/paxchain/layerxagent/jobs/invalid", nil))
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "type mismatch, parameter: id")

	ctx, k := agentModuleContext()
	am := layerxagent.NewAppModule(k, authority)
	require.Equal(t, types.ModuleName, am.Name())
	require.Equal(t, uint64(1), am.ConsensusVersion())
	require.Empty(t, am.InitGenesis(ctx, cdc, defaults))
	require.Equal(t, types.DefaultGenesis(), k.ExportGenesis(ctx))
	genesis := *types.DefaultGenesis()
	genesis.Params.MaxJobsPerCreator = 7
	genesis.Params.Paused = true
	creator := common.HexToAddress("0x1234")
	genesis.Jobs = []types.Job{{
		Id: 3, Creator: creator.Hex(), Target: common.HexToAddress("0x5678").Hex(),
		CallData: []byte{0, 0xde, 0xff}, ExecuteAtBlock: 1010, GasLimit: 100_000,
		Deposit: "1000000000000", Active: true,
	}}
	genesis.NextJobId = 9
	genesisJSON := cdc.MustMarshalJSON(&genesis)
	require.NoError(t, basic.ValidateGenesis(cdc, nil, genesisJSON))
	require.Panics(t, func() { am.InitGenesis(ctx, cdc, genesisJSON) })
	require.Equal(t, types.DefaultGenesis(), k.ExportGenesis(ctx))
	fund := func(ctx sdk.Context, k keeper.Keeper) {
		coins := sdk.NewCoins(sdk.NewCoin(sdk.MustGetBaseDenom(), sdk.OneInt()))
		bank := testkeeper.EVMTestApp.BankKeeper
		require.NoError(t, bank.MintCoins(ctx, "evm", coins))
		require.NoError(t, bank.SendCoinsAndWei(ctx, authtypes.NewModuleAddress("evm"), k.ModuleAddress(), sdk.OneInt(), sdk.ZeroInt()))
	}
	fund(ctx, k)
	require.Empty(t, am.InitGenesis(ctx, cdc, genesisJSON))
	require.Equal(t, genesis.Params, k.GetParams(ctx))
	require.Equal(t, genesis.NextJobId, k.NextJobID(ctx))
	job, found := k.GetJob(ctx, 3)
	require.True(t, found)
	require.Equal(t, genesis.Jobs[0], keeper.JobToProto(job))
	require.Equal(t, []uint64{3}, k.Pending(ctx, creator))
	var due []uint64
	k.IterateDue(ctx, 1010, func(job keeper.Job) bool { due = append(due, job.ID); return false })
	require.Equal(t, []uint64{3}, due)
	var exported types.GenesisState
	cdc.MustUnmarshalJSON(am.ExportGenesis(ctx, cdc), &exported)
	require.Equal(t, genesis, exported)
	require.Equal(t, k.ExportGenesis(ctx), &exported)
	chunks := 0
	for chunk := range am.ExportGenesisStream(ctx, cdc) {
		chunks++
		require.JSONEq(t, string(genesisJSON), string(chunk))
	}
	require.Equal(t, 1, chunks)
	require.Panics(t, func() { am.InitGenesis(ctx, cdc, malformed) })
	require.Panics(t, func() { am.InitGenesis(ctx, cdc, cdc.MustMarshalJSON(&invalid)) })
	require.Equal(t, &genesis, k.ExportGenesis(ctx))
	restoredCtx, restoredKeeper := agentModuleContext()
	fund(restoredCtx, restoredKeeper)
	restoredModule := layerxagent.NewAppModule(restoredKeeper, authority)
	require.Empty(t, restoredModule.InitGenesis(restoredCtx, cdc, am.ExportGenesis(ctx, cdc)))
	require.Equal(t, k.ExportGenesis(ctx), restoredKeeper.ExportGenesis(restoredCtx))

	msgServer, queryServer := grpc.NewServer(), grpc.NewServer()
	t.Cleanup(msgServer.Stop)
	t.Cleanup(queryServer.Stop)
	am.RegisterServices(module.NewConfigurator(cdc, msgServer, queryServer))
	require.Len(t, msgServer.GetServiceInfo(), 1)
	require.Equal(t, []grpc.MethodInfo{{Name: "UpdateParams"}}, msgServer.GetServiceInfo()["paxprotocol.paxchain.layerxagent.Msg"].Methods)
	require.Len(t, queryServer.GetServiceInfo(), 1)
	var queryMethods []string
	for _, method := range queryServer.GetServiceInfo()["paxprotocol.paxchain.layerxagent.Query"].Methods {
		queryMethods = append(queryMethods, method.Name)
	}
	require.ElementsMatch(t, []string{"Params", "Job", "Pending"}, queryMethods)
	msgRouter := baseapp.NewMsgServiceRouter()
	msgRouter.SetInterfaceRegistry(registry)
	queryRouter := baseapp.NewGRPCQueryRouter()
	am.RegisterServices(module.NewConfigurator(cdc, msgRouter, queryRouter))
	msg.Params = genesis.Params
	msg.Params.MaxJobsPerCreator = 11
	msg.Params.Paused = false
	handler := msgRouter.Handler(msg)
	require.NotNil(t, handler)
	result, err := handler(ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, msg.Params, k.GetParams(ctx))
	unauthorized := &types.MsgUpdateParams{Authority: sdk.AccAddress(common.HexToAddress("0xb22").Bytes()).String(), Params: types.DefaultParams()}
	result, err = handler(ctx, unauthorized)
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.Nil(t, result)
	require.Equal(t, msg.Params, k.GetParams(ctx))
	query := func(method string, request, response proto.Message) {
		t.Helper()
		route := queryRouter.Route("/paxprotocol.paxchain.layerxagent.Query/" + method)
		require.NotNil(t, route)
		data, err := proto.Marshal(request)
		require.NoError(t, err)
		res, err := route(ctx, abci.RequestQuery{Data: data})
		require.NoError(t, err)
		require.NoError(t, proto.Unmarshal(res.Value, response))
	}
	var paramsResponse types.QueryParamsResponse
	query("Params", &types.QueryParamsRequest{}, &paramsResponse)
	require.Equal(t, msg.Params, paramsResponse.Params)
	var jobResponse types.QueryJobResponse
	query("Job", &types.QueryJobRequest{Id: 3}, &jobResponse)
	require.Equal(t, genesis.Jobs[0], jobResponse.Job)
	var pendingResponse types.QueryPendingResponse
	query("Pending", &types.QueryPendingRequest{Creator: creator.Hex()}, &pendingResponse)
	require.Equal(t, []uint64{3}, pendingResponse.JobIds)
	cdc.MustUnmarshalJSON(am.ExportGenesis(ctx, cdc), &exported)
	require.Equal(t, msg.Params, exported.Params)
	require.Equal(t, genesis.Jobs, exported.Jobs)
}

func TestAgentAppModuleHasNoOwnEndBlock(t *testing.T) {
	am := layerxagent.NewAppModule(keeper.Keeper{}, "authority")
	for _, value := range []interface{}{am, &am} {
		typ := reflect.TypeOf(value)
		_, hasEndBlock := typ.MethodByName("EndBlock")
		require.False(t, hasEndBlock, "%s must leave RunDue to the legacy end block", typ)
		_, implementsEndBlock := value.(module.EndBlockAppModule)
		require.False(t, implementsEndBlock, "%s must not run jobs through the module manager", typ)
	}
}
