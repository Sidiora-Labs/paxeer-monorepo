package ica_test

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	ica "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts"
	controllercli "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/controller/client/cli"
	controllertypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/controller/types"
	hostcli "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/host/client/cli"
	hosttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/types"
	ibchost "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// Use the committed simapp wiring with a disk-backed SDK multistore. All ICA,
// parameter, port and capability operations below use production keepers.
func repairM1202Open(t *testing.T, dir string) (*simapp.SimApp, *rootmulti.Store, sdk.Context, func()) {
	t.Helper()
	db, err := dbm.NewGoLevelDB("ica", dir)
	require.NoError(t, err)
	ms := rootmulti.NewStore(db)
	options := viper.New()
	options.Set(baseapp.FlagChainID, "repair-m1202")
	app := simapp.NewSimApp(db, nil, false, map[int64]bool{}, dir, 0, nil,
		simapp.MakeTestEncodingConfig(), options,
		func(b *baseapp.BaseApp) { b.SetCMS(ms) })
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, tmproto.Header{ChainID: "repair-m1202", Height: ms.LastCommitID().Version + 1}, false)
	if ms.LastCommitID().Version == 0 {
		require.NoError(t, app.CapabilityKeeper.InitializeIndex(ctx, 1))
	}
	app.CapabilityKeeper.InitMemStore(ctx)
	closed := false
	stop := func() {
		if !closed {
			require.NoError(t, ms.Close())
			closed = true
		}
	}
	t.Cleanup(stop)
	return app, ms, ctx, stop
}

func TestRepairM_1202_ExportContextLifetimeRecovery(t *testing.T) {
	for _, mode := range []string{"both", "controller", "host", "neither"} {
		t.Run(mode, func(t *testing.T) {
			source, sourceStore, ctx, stopSource := repairM1202Open(t, t.TempDir())
			module := ica.NewAppModule(&source.ICAControllerKeeper, &source.ICAHostKeeper)
			cp := controllertypes.NewParams(true)
			hp := hosttypes.NewParams(true, []string{"/cosmos.bank.v1beta1.MsgSend"})
			module.InitModule(ctx, cp, hp)
			owner := sdk.AccAddress(bytes.Repeat([]byte{1}, 20)).String()
			account := sdk.AccAddress(bytes.Repeat([]byte{2}, 20)).String()
			port, err := icatypes.NewControllerPortID(owner)
			require.NoError(t, err)
			capability := source.ICAControllerKeeper.BindPort(ctx, port)
			require.NoError(t, source.ICAControllerKeeper.ClaimCapability(ctx, capability, ibchost.PortPath(port)))
			source.ICAControllerKeeper.SetActiveChannelID(ctx, "connection-0", port, "channel-2")
			source.ICAControllerKeeper.SetInterchainAccountAddress(ctx, "connection-0", port, account)
			source.ICAHostKeeper.SetActiveChannelID(ctx, "connection-1", port, "channel-3")
			source.ICAHostKeeper.SetInterchainAccountAddress(ctx, "connection-1", port, account)
			switch mode {
			case "controller":
				module = ica.NewAppModule(&source.ICAControllerKeeper, nil)
			case "host":
				module = ica.NewAppModule(nil, &source.ICAHostKeeper)
			case "neither":
				module = ica.NewAppModule(nil, nil)
			}
			want := module.ExportGenesis(ctx, source.AppCodec())
			stream := module.ExportGenesisStream(ctx, source.AppCodec())
			// These assertions make delayed production a deterministic failure,
			// without racing a goroutine against a closing database.
			require.Equal(t, 1, cap(stream))
			require.Len(t, stream, 1)
			source.ICAControllerKeeper.SetParams(ctx, controllertypes.NewParams(false))
			source.ICAHostKeeper.SetParams(ctx, hosttypes.NewParams(false, []string{}))
			source.ICAControllerKeeper.SetActiveChannelID(ctx, "connection-0", port, "channel-9")
			source.ICAHostKeeper.SetInterchainAccountAddress(ctx, "connection-1", port, owner)
			sourceStore.Commit(true)
			stopSource()
			got, ok := <-stream
			require.True(t, ok)
			require.JSONEq(t, string(want), string(got))
			select {
			case _, ok := <-stream:
				require.False(t, ok, "stream must contain exactly one message and already be closed")
			default:
				t.Fatal("stream still depends on a producer")
			}

			var snapshot icatypes.GenesisState
			require.NoError(t, source.AppCodec().UnmarshalAsJSON(got, &snapshot))
			require.NoError(t, snapshot.Validate())
			if mode == "both" || mode == "controller" {
				require.Equal(t, "channel-2", snapshot.ControllerGenesisState.ActiveChannels[0].ChannelId)
				require.Equal(t, account, snapshot.ControllerGenesisState.InterchainAccounts[0].AccountAddress)
				require.Equal(t, []string{port}, snapshot.ControllerGenesisState.Ports)
				require.Equal(t, cp, snapshot.ControllerGenesisState.Params)
			}
			if mode == "both" || mode == "host" {
				require.Equal(t, "channel-3", snapshot.HostGenesisState.ActiveChannels[0].ChannelId)
				require.Equal(t, account, snapshot.HostGenesisState.InterchainAccounts[0].AccountAddress)
				require.Equal(t, hp, snapshot.HostGenesisState.Params)
			}

			importDir := t.TempDir()
			consumer, consumerStore, consumerCtx, stopConsumer := repairM1202Open(t, importDir)
			importModule := ica.NewAppModule(&consumer.ICAControllerKeeper, &consumer.ICAHostKeeper)
			require.NoError(t, importModule.ValidateGenesis(consumer.AppCodec(), nil, got))
			require.Empty(t, importModule.InitGenesis(consumerCtx, consumer.AppCodec(), got))
			require.JSONEq(t, string(want), string(importModule.ExportGenesis(consumerCtx, consumer.AppCodec())))
			commit := consumerStore.Commit(true)
			require.Equal(t, int64(1), commit.Version)
			stopConsumer()

			reopened, reopenedStore, recoveredCtx, _ := repairM1202Open(t, importDir)
			require.Equal(t, commit, reopenedStore.LastCommitID())
			recovered := ica.NewAppModule(&reopened.ICAControllerKeeper, &reopened.ICAHostKeeper)
			require.JSONEq(t, string(want), string(recovered.ExportGenesis(recoveredCtx, reopened.AppCodec())))
			require.True(t, reopened.ICAHostKeeper.IsBound(recoveredCtx, icatypes.PortID))
			if mode == "both" || mode == "controller" {
				require.True(t, reopened.ICAControllerKeeper.IsBound(recoveredCtx, port))
				channel, found := reopened.ICAControllerKeeper.GetActiveChannelID(recoveredCtx, "connection-0", port)
				require.True(t, found)
				require.Equal(t, "channel-2", channel)
				address, found := reopened.ICAControllerKeeper.GetInterchainAccountAddress(recoveredCtx, "connection-0", port)
				require.True(t, found)
				require.Equal(t, account, address)
			}
			if mode == "both" || mode == "host" {
				address, found := reopened.ICAHostKeeper.GetInterchainAccountAddress(recoveredCtx, "connection-1", port)
				require.True(t, found)
				require.Equal(t, account, address)
			}
		})
	}
}

func TestRepairM_1202_RuntimeAvailability(t *testing.T) {
	// The chosen repair is the unsupported-mainnet contract, proven through
	// actual command construction and Cobra's help renderer, without an RPC.
	var checkHelp func(*cobra.Command)
	checkHelp = func(cmd *cobra.Command) {
		t.Helper()
		var output bytes.Buffer
		cmd.SetOut(&output)
		require.NoError(t, cmd.Help())
		require.Contains(t, output.String(), icatypes.AvailabilityGuidance, cmd.CommandPath())
		for _, child := range cmd.Commands() {
			checkHelp(child)
		}
	}
	checkHelp((ica.AppModuleBasic{}).GetQueryCmd())
	checkHelp(controllercli.GetQueryCmd())
	// Check standalone leaf constructors too: they can be embedded by apps.
	for _, cmd := range []*cobra.Command{controllercli.GetCmdQueryInterchainAccount(), controllercli.GetCmdParams(), hostcli.GetQueryCmd(), hostcli.GetCmdParams(), hostcli.GetCmdPacketEvents()} {
		checkHelp(cmd)
	}

	// Pin the documentation decision to production wiring without importing
	// node's unrelated native VM dependencies or starting a chain process.
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	nodePath := filepath.Join(filepath.Dir(filename), "../../../../node/app.go")
	file, err := parser.ParseFile(token.NewFileSet(), nodePath, nil, 0)
	require.NoError(t, err)
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		require.NoError(t, err)
		require.False(t, strings.HasPrefix(path, "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts"), "mainnet ICA activation requires updating the availability contract and proving full wiring")
	}
	var routes []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "AddRoute" {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok || receiver.Name != "ibcRouter" {
			return true
		}
		require.Len(t, call.Args, 2)
		key, ok := call.Args[0].(*ast.SelectorExpr)
		require.True(t, ok)
		module, ok := key.X.(*ast.Ident)
		require.True(t, ok)
		routes = append(routes, module.Name+"."+key.Sel.Name)
		return true
	})
	require.ElementsMatch(t, []string{"ibctransfertypes.ModuleName", "wasm.ModuleName"}, routes)
}

func TestRepairM_1202_StreamCompletesWithoutConsumer(t *testing.T) {
	app, _, ctx, _ := repairM1202Open(t, t.TempDir())
	module := ica.NewAppModule(&app.ICAControllerKeeper, &app.ICAHostKeeper)
	module.InitModule(ctx, controllertypes.NewParams(true), hosttypes.NewParams(true, []string{}))
	for i := 0; i < 16; i++ {
		stream := module.ExportGenesisStream(ctx, app.AppCodec())
		require.Equal(t, 1, cap(stream))
		require.Len(t, stream, 1, "production must finish even when the caller abandons the stream")
	}
	// The stream still uses the same real JSON codec contract as ordinary export.
	var snapshot json.RawMessage = module.ExportGenesis(ctx, app.AppCodec())
	require.True(t, json.Valid(snapshot))
}
