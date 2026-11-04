package genesis_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/genesis"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	genutilcli "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repair294Fixture struct {
	ctx     sdk.Context
	keeper  paramkeeper.Keeper
	cdc     *codec.ProtoCodec
	manager *module.Manager
	store   storetypes.CommitMultiStore
	closed  bool
}

func repair294Open(t *testing.T, dir string) *repair294Fixture {
	t.Helper()
	db, err := dbm.NewDB("params", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	cms := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(paramtypes.StoreKey)
	tkey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	cms.MountStoreWithDB(tkey, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	keeper := paramkeeper.NewKeeper(cdc, codec.NewLegacyAmino(), key, tkey)
	f := &repair294Fixture{
		ctx:    sdk.NewContext(cms, tmproto.Header{Height: 1, Time: time.Unix(1, 0)}, false),
		keeper: keeper, cdc: cdc, manager: module.NewManager(params.NewAppModule(keeper)), store: cms,
	}
	t.Cleanup(func() { f.close(t) })
	return f
}

func (f *repair294Fixture) close(t *testing.T) {
	t.Helper()
	if !f.closed {
		require.NoError(t, f.store.Close())
		f.closed = true
	}
}

func (f *repair294Fixture) seed(denominator uint64) {
	gs := paramtypes.DefaultGenesis()
	gs.CosmosGasParams.CosmosGasMultiplierDenominator = denominator
	f.keeper.InitGenesis(f.ctx, gs)
}

func (f *repair294Fixture) export(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	require.NoError(t, f.manager.ProcessGenesisPerModule(f.ctx, f.cdc, func(name string, data json.RawMessage) error {
		return enc.Encode(module.ModuleState{AppState: module.AppState{Module: name, Data: data}})
	}))
	// The server exporter appends the genesis document after module records,
	// without requiring a final newline.
	header, err := json.Marshal(server.GenesisDocNoAppState{
		ChainID: "repair-294", InitialHeight: 1, GenesisTime: time.Unix(1, 0).UTC(),
		ConsensusParams: tmtypes.DefaultConsensusParams(),
	})
	require.NoError(t, err)
	out.Write(header)
	return out.Bytes()
}

func repair294Write(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "genesis.stream")
	require.NoError(t, os.WriteFile(path, data, 0600))
	return path
}

func (f *repair294Fixture) init(path string) {
	f.manager.InitGenesis(f.ctx, f.cdc, nil, genesis.GenesisImportConfig{
		StreamGenesisImport: true, GenesisStreamFile: path,
	})
}

func repair294Panic(t *testing.T, action func()) error {
	t.Helper()
	var caught interface{}
	func() {
		defer func() { caught = recover() }()
		action()
	}()
	require.NotNil(t, caught, "incomplete input must fail initialization")
	err, ok := caught.(error)
	require.True(t, ok, "expected an error panic, got %v", caught)
	return err
}

func (f *repair294Fixture) validate(t *testing.T, path string) (string, error) {
	t.Helper()
	clientCtx := client.Context{}.WithCodec(f.cdc)
	serverCtx := server.NewContext(viper.New(), tmcfg.DefaultConfig().SetRoot(t.TempDir()))
	ctx := context.WithValue(context.Background(), client.ClientContextKey, &clientCtx)
	ctx = context.WithValue(ctx, server.ServerContextKey, serverCtx)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := genutilcli.ValidateGenesisCmd(module.NewBasicManager(params.AppModuleBasic{}))
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(io.Discard)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--streaming", path})
	err := cmd.ExecuteContext(ctx)
	return output.String(), err
}

func TestRepair_294_ConsumersRejectIncompleteSources(t *testing.T) {
	source := repair294Open(t, t.TempDir())
	source.seed(7)
	exported := source.export(t)
	firstLineEnd := bytes.IndexByte(exported, '\n') + 1
	require.Positive(t, firstLineEnd)
	cases := []struct{ name, path string }{
		{"missing", filepath.Join(t.TempDir(), "missing")},
		{"unreadable_directory", t.TempDir()},
		{"empty", repair294Write(t, nil)},
		{"truncated_record", repair294Write(t, exported[:len(exported)-1])},
		{"prefix_without_header", repair294Write(t, exported[:firstLineEnd])},
		{"header_before_truncated_module", repair294Write(t, append(append(bytes.Clone(exported[firstLineEnd:]), '\n'), exported[:firstLineEnd-2]...))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			target := repair294Open(t, dir)
			target.seed(4)
			target.store.Commit(true)
			require.Panics(t, func() { target.init(tc.path) })
			require.Equal(t, uint64(4), target.keeper.GetCosmosGasParams(target.ctx).CosmosGasMultiplierDenominator)
			// Even if the caller commits after recovering the panic, no prefix
			// of the rejected initialization may survive a real disk restart.
			target.store.Commit(true)
			target.close(t)
			reopened := repair294Open(t, dir)
			require.Equal(t, uint64(4), reopened.keeper.GetCosmosGasParams(reopened.ctx).CosmosGasMultiplierDenominator)
			doc, err := server.LoadStreamingGenesisDoc(context.Background(), tc.path)
			require.Error(t, err)
			require.Nil(t, doc)
			output, err := source.validate(t, tc.path)
			require.Error(t, err)
			require.NotContains(t, output, "is a valid genesis file")
		})
	}
}

func TestRepair_294_MidReadFailureDiscardsModuleState(t *testing.T) {
	source := repair294Open(t, t.TempDir())
	source.seed(7)
	exported := source.export(t)
	dir := t.TempDir()
	target := repair294Open(t, dir)
	target.seed(4)
	target.store.Commit(true)
	reader, writer := io.Pipe()
	stream := genesis.NewStream(context.Background(), reader)
	t.Cleanup(func() { _ = stream.Close() })
	readFailure := errors.New("export transport interrupted")
	producerDone := make(chan error, 1)
	go func() {
		_, err := writer.Write(append(bytes.Clone(exported), '\n'))
		_ = writer.CloseWithError(readFailure)
		producerDone <- err
	}()
	err := repair294Panic(t, func() { target.manager.InitGenesisStream(target.ctx, target.cdc, stream) })
	require.ErrorIs(t, err, readFailure)
	require.ErrorIs(t, stream.Wait(), readFailure)
	require.ErrorIs(t, stream.Wait(), readFailure, "completion must be repeatable")
	require.NoError(t, <-producerDone)
	require.Equal(t, uint64(4), target.keeper.GetCosmosGasParams(target.ctx).CosmosGasMultiplierDenominator)
	target.store.Commit(true)
	target.close(t)
	reopened := repair294Open(t, dir)
	require.Equal(t, uint64(4), reopened.keeper.GetCosmosGasParams(reopened.ctx).CosmosGasMultiplierDenominator)
}

func TestRepair_294_ExportImportAndRestart(t *testing.T) {
	source := repair294Open(t, t.TempDir())
	source.seed(7)
	exported := source.export(t)
	path := repair294Write(t, exported)
	doc, err := server.LoadStreamingGenesisDoc(context.Background(), path)
	require.NoError(t, err)
	require.Equal(t, "repair-294", doc.ChainID)
	output, err := source.validate(t, path)
	require.NoError(t, err)
	require.Contains(t, output, "is a valid genesis file")
	dir := t.TempDir()
	target := repair294Open(t, dir)
	target.init(path)
	require.Equal(t, source.keeper.ExportGenesis(source.ctx), target.keeper.ExportGenesis(target.ctx))
	target.store.Commit(true)
	target.close(t)
	reopened := repair294Open(t, dir)
	require.Equal(t, source.keeper.ExportGenesis(source.ctx), reopened.keeper.ExportGenesis(reopened.ctx))
	require.Equal(t, exported, reopened.export(t))
}

func TestRepair_294_CancellationUnblocksReadAndSend(t *testing.T) {
	for _, blockedSend := range []bool{false, true} {
		t.Run(map[bool]string{false: "read", true: "send"}[blockedSend], func(t *testing.T) {
			reader, writer := io.Pipe()
			defer writer.Close()
			stream := genesis.NewStream(context.Background(), reader)
			producerDone := make(chan error, 1)
			if blockedSend {
				go func() {
					_, err := io.WriteString(writer, "first\nsecond\n")
					producerDone <- err
				}()
				require.Equal(t, "first", <-stream.Lines)
			}
			done := make(chan error, 1)
			go func() { done <- stream.Close() }()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(5 * time.Second):
				t.Fatal("cancellation failed to release source or producer")
			}
			for range stream.Lines {
			}
			if blockedSend {
				<-producerDone
			}
		})
	}
}

func TestRepair_294_CompletionAndEarlyConsumerError(t *testing.T) {
	longLine := strings.Repeat("x", 250000)
	path := repair294Write(t, []byte("first\n\n"+longLine+"\nfinal"))
	stream := genesis.IngestGenesisFileLineByLine(path)
	var lines []string
	require.NoError(t, stream.Consume(func(line string) error { lines = append(lines, line); return nil }))
	require.Equal(t, []string{"first", "", longLine, "final"}, lines)
	require.NoError(t, stream.Wait())
	require.NoError(t, stream.Wait())
	stream = genesis.IngestGenesisFileLineByLine(path)
	consumerError := errors.New("consumer rejected record")
	require.ErrorIs(t, stream.Consume(func(string) error { return consumerError }), consumerError)
	require.ErrorIs(t, stream.Wait(), context.Canceled)
}
