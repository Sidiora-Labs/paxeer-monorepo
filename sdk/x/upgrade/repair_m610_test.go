package upgrade_test

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/api"
	legacymulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	store "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// These functions are the doc.go examples, compiled as ordinary Go code. The
// setup test compares their ASTs with the documentation before exercising them.
func beginUpgradeBlock(k keeper.Keeper, ctx sdk.Context) {
	upgrade.BeginBlocker(k, ctx)
}

func registerUpgradeHandler(k keeper.Keeper, migrate types.UpgradeHandler) {
	k.SetUpgradeHandler("my-fancy-upgrade", func(ctx sdk.Context, plan types.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		return migrate(ctx, plan, fromVM)
	})
}

func configureUpgradeStores(app *baseapp.BaseApp, k keeper.Keeper) error {
	upgradeInfo, err := k.ReadUpgradeInfoFromDisk()
	if err != nil {
		return err
	}
	if upgradeInfo.Name == "my-fancy-upgrade" && !k.IsSkipHeight(upgradeInfo.Height) {
		storeUpgrades := store.StoreUpgrades{
			Renamed: []store.StoreRename{{OldKey: "foo", NewKey: "bar"}},
		}
		app.SetStoreLoader(types.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
	return nil
}

func repairM610Read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func repairM610ExamplesCompile(t *testing.T) {
	t.Helper()
	doc := repairM610Read(t, "doc.go")
	blocks := regexp.MustCompile(`(?ms)^\tfunc .*?^\t}`).FindAllString(doc, -1)
	require.Len(t, blocks, 3)
	imports := regexp.MustCompile(`(?ms)^\timport \(.*?^\t\)`).FindString(doc)
	require.NotEmpty(t, imports)
	var source strings.Builder
	source.WriteString("package upgrade_test\n")
	for _, block := range append([]string{imports}, blocks...) {
		for _, line := range strings.Split(block, "\n") {
			source.WriteString(strings.TrimPrefix(line, "\t") + "\n")
		}
	}
	fs := token.NewFileSet()
	documented, err := parser.ParseFile(fs, "examples.go", source.String(), 0)
	require.NoError(t, err)
	compiled, err := parser.ParseFile(fs, "repair_m610_test.go", repairM610Read(t, "repair_m610_test.go"), 0)
	require.NoError(t, err)
	compiledImports := map[string]string{}
	for _, imp := range compiled.Imports {
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		path, err := strconv.Unquote(imp.Path.Value)
		require.NoError(t, err)
		compiledImports[path] = alias
	}
	for _, imp := range documented.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		require.NoError(t, err)
		require.Contains(t, compiledImports, path)
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		require.Equal(t, alias, compiledImports[path])
	}
	compiledFunctions := map[string]string{}
	for _, decl := range compiled.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			var out bytes.Buffer
			require.NoError(t, format.Node(&out, fs, fn))
			compiledFunctions[fn.Name.Name] = out.String()
		}
	}
	for _, decl := range documented.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			var out bytes.Buffer
			require.NoError(t, format.Node(&out, fs, fn))
			require.Equal(t, compiledFunctions[fn.Name.Name], out.String(), "documented %s must be the compiled function", fn.Name.Name)
		}
	}
}

type repairM610State struct {
	k        keeper.Keeper
	ctx      sdk.Context
	cdc      *codec.ProtoCodec
	registry codectypes.InterfaceRegistry
}

func repairM610MemoryState(t *testing.T) repairM610State {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	key := sdk.NewKVStoreKey(types.StoreKey)
	ms := legacymulti.NewStore(dbm.NewMemDB())
	ms.MountStoreWithDB(key, store.StoreTypeIAVL, nil)
	require.NoError(t, ms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, ms.Close()) })
	return repairM610State{
		k: keeper.NewKeeper(nil, key, cdc, t.TempDir(), nil),
		ctx: sdk.NewContext(ms.CacheMultiStore(), tmproto.Header{Height: 1}, false).
			WithConsensusParams(&tmproto.ConsensusParams{Version: &tmproto.VersionParams{}}),
		cdc: cdc, registry: registry,
	}
}

func TestRepairM_610_SetupAPI(t *testing.T) {
	repairM610ExamplesCompile(t)
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "handler_error"
		}
		t.Run(name, func(t *testing.T) {
			s := repairM610MemoryState(t)
			plan := types.Plan{Name: "my-fancy-upgrade", Height: 2}
			proposalHandler := upgrade.NewSoftwareUpgradeProposalHandler(s.k)
			require.NoError(t, proposalHandler(s.ctx, &types.SoftwareUpgradeProposal{Title: "Upgrade", Description: "Migrate modules", Plan: plan}))
			from := module.VersionMap{"upgrade": 1, "bank": 3}
			s.k.SetModuleVersionMap(s.ctx, from)
			migrationErr := errors.New("migration failed")
			calls := 0
			registerUpgradeHandler(s.k, func(ctx sdk.Context, got types.Plan, vm module.VersionMap) (module.VersionMap, error) {
				calls++
				require.Equal(t, int64(2), ctx.BlockHeight())
				require.Equal(t, plan, got)
				require.Equal(t, from, vm)
				if fail {
					return nil, migrationErr
				}
				vm["bank"] = 4
				return vm, nil
			})
			if fail {
				require.PanicsWithValue(t, migrationErr, func() { beginUpgradeBlock(s.k, s.ctx.WithBlockHeight(2)) })
				got, found := s.k.GetUpgradePlan(s.ctx)
				require.True(t, found)
				require.Equal(t, plan, got)
				require.Zero(t, s.k.GetDoneHeight(s.ctx, plan.Name))
				require.Equal(t, from, s.k.GetModuleVersionMap(s.ctx))
			} else {
				require.NotPanics(t, func() { beginUpgradeBlock(s.k, s.ctx.WithBlockHeight(2)) })
				require.Equal(t, module.VersionMap{"upgrade": 1, "bank": 4}, s.k.GetModuleVersionMap(s.ctx))
				_, found := s.k.GetUpgradePlan(s.ctx)
				require.False(t, found)
				require.Equal(t, int64(2), s.k.GetDoneHeight(s.ctx, plan.Name))
				require.Error(t, s.k.ScheduleUpgrade(s.ctx, plan), "completed names cannot be reused")
			}
			require.Equal(t, 1, calls)
		})
	}
	t.Run("cancellation", func(t *testing.T) {
		s := repairM610MemoryState(t)
		handler := upgrade.NewSoftwareUpgradeProposalHandler(s.k)
		require.NoError(t, handler(s.ctx, &types.SoftwareUpgradeProposal{Title: "Upgrade", Description: "Schedule", Plan: types.Plan{Name: "cancel-me", Height: 2}}))
		require.NoError(t, handler(s.ctx, &types.CancelSoftwareUpgradeProposal{Title: "Cancel", Description: "Cancel pending migration"}))
		_, found := s.k.GetUpgradePlan(s.ctx)
		require.False(t, found)
		require.NotPanics(t, func() { beginUpgradeBlock(s.k, s.ctx.WithBlockHeight(2)) })
		require.Zero(t, s.k.GetDoneHeight(s.ctx, "cancel-me"))
	})
}

func repairM610Open(t *testing.T, home, dataStore string) (*baseapp.BaseApp, *rootmulti.Store, keeper.Keeper, *sdk.KVStoreKey) {
	t.Helper()
	sc, ss := config.DefaultStateCommitConfig(), config.DefaultStateStoreConfig()
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	ss.AsyncWriteBuffer = 0
	ms := rootmulti.NewStore(home, sc, ss, nil)
	upgradeKey, dataKey := sdk.NewKVStoreKey(types.StoreKey), sdk.NewKVStoreKey(dataStore)
	ms.MountStoreWithDB(upgradeKey, store.StoreTypeIAVL, nil)
	ms.MountStoreWithDB(dataKey, store.StoreTypeIAVL, nil)
	app := baseapp.NewBaseApp(t.Name(), dbm.NewMemDB(), nil, nil, &testutil.TestAppOpts{})
	app.SetCMS(ms)
	k := keeper.NewKeeper(nil, upgradeKey, codec.NewProtoCodec(codectypes.NewInterfaceRegistry()), home, app)
	require.NoError(t, configureUpgradeStores(app, k))
	require.NoError(t, app.LoadLatestVersion())
	return app, ms, k, dataKey
}

func TestRepairM_610_BeginBlockRestart(t *testing.T) {
	concepts := repairM610Read(t, "spec/01_concepts.md")
	require.Contains(t, concepts, "### BeginBlock timing")
	require.Contains(t, concepts, "The old binary commits through `H - 1`")
	require.Contains(t, concepts, "It does not commit block `H`")
	require.Contains(t, concepts, "epoch\nprocessing runs first, then upgrade, capability, distribution, slashing,")
	ordering := repairM610Read(t, "../../../node/legacyabci/begin_block.go")
	last := -1
	for _, call := range []string{"keepers.EpochKeeper.BeginBlock(ctx)", "upgrade.BeginBlocker(*keepers.UpgradeKeeper, ctx)", "capability.BeginBlocker(", "distribution.BeginBlocker(", "slashing.BeginBlocker(", "evidence.BeginBlocker(", "staking.BeginBlocker(", "ibcclient.BeginBlocker(", "keepers.EvmKeeper.BeginBlock(ctx)"} {
		at := strings.Index(ordering, call)
		require.Greater(t, at, last, "node hook order: %s", call)
		last = at
	}
	home := t.TempDir()
	app, ms, k, dataKey := repairM610Open(t, home, "foo")
	cache := ms.CacheMultiStore()
	ctx := sdk.NewContext(cache, tmproto.Header{Height: 1}, false)
	plan := types.Plan{Name: "my-fancy-upgrade", Height: 2, Info: "store and module migration"}
	require.NoError(t, k.ScheduleUpgrade(ctx, plan))
	k.SetModuleVersionMap(ctx, module.VersionMap{"upgrade": 1, "bank": 3})
	ctx.KVStore(dataKey).Set([]byte("retained"), []byte("before upgrade"))
	require.NotPanics(t, func() { beginUpgradeBlock(k, ctx) })
	cache.Write()
	require.Equal(t, int64(1), ms.Commit(true).Version)
	ctx = sdk.NewContext(ms.CacheMultiStore(), tmproto.Header{Height: 2}, false)
	require.PanicsWithValue(t, upgrade.BuildUpgradeNeededMsg(plan), func() { beginUpgradeBlock(k, ctx) })
	require.Equal(t, int64(1), ms.LastCommitID().Version)
	require.Zero(t, k.GetDoneHeight(ctx, plan.Name))
	info, err := k.ReadUpgradeInfoFromDisk()
	require.NoError(t, err)
	require.Equal(t, plan.Name, info.Name)
	require.Equal(t, plan.Height, info.Height)
	require.NoError(t, app.Close())

	// The replacement process loads the last committed state and real handoff
	// file, renames the store, then executes the module migration at height H.
	app, ms, k, dataKey = repairM610Open(t, home, "bar")
	cache = ms.CacheMultiStore()
	ctx = sdk.NewContext(cache, tmproto.Header{Height: 2}, false)
	require.Equal(t, []byte("before upgrade"), ctx.KVStore(dataKey).Get([]byte("retained")))
	got, found := k.GetUpgradePlan(ctx)
	require.True(t, found)
	require.Equal(t, plan, got)
	calls := 0
	migrate := func(ctx sdk.Context, got types.Plan, vm module.VersionMap) (module.VersionMap, error) {
		calls++
		require.Equal(t, plan, got)
		require.Equal(t, module.VersionMap{"upgrade": 1, "bank": 3}, vm)
		ctx.KVStore(dataKey).Set([]byte("migration"), []byte("committed"))
		vm["bank"] = 4
		return vm, nil
	}
	registerUpgradeHandler(k, migrate)
	require.NotPanics(t, func() { beginUpgradeBlock(k, ctx) })
	require.Equal(t, 1, calls)
	require.Equal(t, uint64(1), app.AppVersion())
	require.Equal(t, int64(2), k.GetDoneHeight(ctx, plan.Name))
	_, found = k.GetUpgradePlan(ctx)
	require.False(t, found)
	cache.Write()
	require.Equal(t, int64(2), ms.Commit(true).Version)
	require.NoError(t, app.Close())

	app, ms, k, dataKey = repairM610Open(t, home, "bar")
	defer func() { require.NoError(t, app.Close()) }()
	ctx = sdk.NewContext(ms.CacheMultiStore(), tmproto.Header{Height: 3}, false)
	registerUpgradeHandler(k, migrate)
	require.NotPanics(t, func() { beginUpgradeBlock(k, ctx) })
	require.Equal(t, 1, calls, "committed migration must not repeat after restart")
	require.Equal(t, int64(2), k.GetDoneHeight(ctx, plan.Name))
	require.Equal(t, module.VersionMap{"upgrade": 1, "bank": 4}, k.GetModuleVersionMap(ctx))
	require.Equal(t, []byte("before upgrade"), ctx.KVStore(dataKey).Get([]byte("retained")))
	require.Equal(t, []byte("committed"), ctx.KVStore(dataKey).Get([]byte("migration")))
	_, found = k.GetUpgradePlan(ctx)
	require.False(t, found)
}

func repairM610GRPC(t *testing.T, s repairM610State) (*grpc.ClientConn, *grpc.Server) {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(sdk.WrapSDKContext(s.ctx.WithContext(ctx)), req)
	}))
	upgrade.NewAppModule(s.k).RegisterServices(module.NewConfigurator(s.cdc, server, server))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return conn, server
}

func repairM610Section(t *testing.T, doc, start, end string) string {
	t.Helper()
	_, rest, found := strings.Cut(doc, start)
	require.True(t, found, "section %q", start)
	if end != "" {
		section, _, found := strings.Cut(rest, end)
		require.True(t, found, "section end %q", end)
		return section
	}
	return rest
}

func repairM610Blocks(section, language string) []string {
	matches := regexp.MustCompile("(?s)```"+language+"\n(.*?)\n```").FindAllStringSubmatch(section, -1)
	blocks := make([]string, len(matches))
	for i, match := range matches {
		blocks[i] = match[1]
	}
	return blocks
}

func TestRepairM_610_CurrentPlanShape(t *testing.T) {
	s := repairM610MemoryState(t)
	plan := types.Plan{Name: "test-upgrade", Height: 130}
	require.NoError(t, s.k.ScheduleUpgrade(s.ctx, plan))
	conn, _ := repairM610GRPC(t, s)
	query := types.NewQueryClient(conn)
	response, err := query.CurrentPlan(t.Context(), &types.QueryCurrentPlanRequest{})
	require.NoError(t, err)
	require.Equal(t, plan, *response.Plan)
	apiServer := api.New(client.Context{InterfaceRegistry: s.registry})
	require.NoError(t, types.RegisterQueryHandlerClient(t.Context(), apiServer.GRPCGatewayRouter, query))
	doc := repairM610Read(t, "spec/04_client.md")
	rest := repairM610Section(t, doc, "## REST", "## gRPC")
	restPlan := repairM610Section(t, rest, "### Current Plan", "### Module versions")
	restJSON := repairM610Blocks(restPlan, "json")
	require.Len(t, restJSON, 2, "scheduled and absent REST plan examples")
	request := httptest.NewRequest(http.MethodGet, "/cosmos/upgrade/v1beta1/current_plan", nil)
	recorder := httptest.NewRecorder()
	apiServer.GRPCGatewayRouter.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, restJSON[0], recorder.Body.String())

	cli := repairM610Section(t, doc, "#### plan", "## REST")
	cliJSON, cliYAML := repairM610Blocks(cli, "json"), repairM610Blocks(cli, "yaml")
	require.Len(t, cliJSON, 1)
	require.Len(t, cliYAML, 1)
	var printed bytes.Buffer
	clientCtx := client.Context{}.WithCodec(s.cdc).WithOutput(&printed).WithOutputFormat("json")
	require.NoError(t, clientCtx.PrintProto(response.Plan))
	require.JSONEq(t, cliJSON[0], printed.String())
	printed.Reset()
	require.NoError(t, clientCtx.WithOutputFormat("text").PrintProto(response.Plan))
	require.Equal(t, strings.TrimSpace(cliYAML[0]), strings.TrimSpace(printed.String()))

	grpcDoc := repairM610Section(t, doc, "## gRPC", "")
	grpcPlan := repairM610Section(t, grpcDoc, "### Current Plan", "### Module versions")
	grpcJSON := repairM610Blocks(grpcPlan, "json")
	require.Len(t, grpcJSON, 1)
	marshaler := jsonpb.Marshaler{EmitDefaults: true}
	encoded, err := marshaler.MarshalToString(response)
	require.NoError(t, err)
	require.JSONEq(t, grpcJSON[0], encoded)

	s.k.ClearUpgradePlan(s.ctx)
	response, err = query.CurrentPlan(t.Context(), &types.QueryCurrentPlanRequest{})
	require.NoError(t, err)
	require.Nil(t, response.Plan)
	recorder = httptest.NewRecorder()
	apiServer.GRPCGatewayRouter.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, restJSON[1], recorder.Body.String())
	encoded, err = marshaler.MarshalToString(response)
	require.NoError(t, err)
	require.Contains(t, grpcPlan, "`"+encoded+"`")
	withoutDefaults := jsonpb.Marshaler{}
	encoded, err = withoutDefaults.MarshalToString(response)
	require.NoError(t, err)
	require.Equal(t, "{}", encoded)
	require.Contains(t, grpcPlan, "renders `{}`")
}

func TestRepairM_610_GRPCService(t *testing.T) {
	s := repairM610MemoryState(t)
	plan := types.Plan{Name: "test-upgrade", Height: 130}
	require.NoError(t, s.k.ScheduleUpgrade(s.ctx, plan))
	s.k.SetModuleVersionMap(s.ctx, module.VersionMap{"upgrade": 1})
	conn, server := repairM610GRPC(t, s)
	service, found := server.GetServiceInfo()["cosmos.upgrade.v1beta1.Query"]
	require.True(t, found)
	methods := make([]string, 0, len(service.Methods))
	for _, method := range service.Methods {
		methods = append(methods, method.Name)
	}
	require.Contains(t, methods, "CurrentPlan")
	require.Contains(t, methods, "ModuleVersions")
	doc := repairM610Read(t, "spec/04_client.md")
	grpcDoc := repairM610Section(t, doc, "## gRPC", "")
	require.NotContains(t, grpcDoc, "cosmos.slashing.v1beta1.Query")
	commands := regexp.MustCompile(`-d '([^']*)'\s*\\?\s*localhost:9090 (\S+)`).FindAllStringSubmatch(grpcDoc, -1)
	currentPlans, moduleVersions := 0, 0
	for _, command := range commands {
		body, method := command[1], command[2]
		switch {
		case strings.HasSuffix(method, "/CurrentPlan"):
			currentPlans++
			require.Equal(t, "cosmos.upgrade.v1beta1.Query/CurrentPlan", method)
			var req types.QueryCurrentPlanRequest
			require.NoError(t, jsonpb.UnmarshalString(body, &req))
			var res types.QueryCurrentPlanResponse
			require.NoError(t, conn.Invoke(t.Context(), "/"+method, &req, &res))
			require.Equal(t, plan, *res.Plan)
		case strings.HasSuffix(method, "/ModuleVersions"):
			moduleVersions++
			require.Equal(t, "cosmos.upgrade.v1beta1.Query/ModuleVersions", method)
			var req types.QueryModuleVersionsRequest
			require.NoError(t, jsonpb.UnmarshalString(body, &req))
			var res types.QueryModuleVersionsResponse
			require.NoError(t, conn.Invoke(t.Context(), "/"+method, &req, &res))
			require.Equal(t, []*types.ModuleVersion{{Name: "upgrade", Version: 1}}, res.ModuleVersions)
			marshaler := jsonpb.Marshaler{EmitDefaults: true}
			encoded, err := marshaler.MarshalToString(&res)
			require.NoError(t, err)
			section := repairM610Section(t, grpcDoc, "### Module versions", "")
			outputs := repairM610Blocks(section, "json")
			require.Len(t, outputs, 1)
			require.JSONEq(t, outputs[0], encoded)
		}
	}
	require.Equal(t, 1, currentPlans, "execute the documented CurrentPlan command")
	require.Equal(t, 2, moduleVersions, "execute both unfiltered and filtered ModuleVersions commands")
}
