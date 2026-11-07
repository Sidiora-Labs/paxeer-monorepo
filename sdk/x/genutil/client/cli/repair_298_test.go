package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil"
	genutilcli "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/fortytw2/leaktest"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type repair298Fixture struct {
	ctx       sdk.Context
	keeper    paramkeeper.Keeper
	cdc       *codec.ProtoCodec
	mbm       module.BasicManager
	clientCtx client.Context
	home      string
}

func newRepair298Fixture(t *testing.T) repair298Fixture {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	mbm := module.NewBasicManager(params.AppModuleBasic{}, genutil.AppModuleBasic{})
	mbm.RegisterInterfaces(registry)
	amino := codec.NewLegacyAmino()
	mbm.RegisterLegacyAminoCodec(amino)
	key := sdk.NewKVStoreKey(paramtypes.ModuleName)
	tkey := sdk.NewTransientStoreKey("transient_params")
	ctx := testutil.DefaultContext(key, tkey)
	keeper := paramkeeper.NewKeeper(cdc, amino, key, tkey)
	keeper.InitGenesis(ctx, paramtypes.DefaultGenesis())
	home := t.TempDir()
	return repair298Fixture{
		ctx: ctx, keeper: keeper, cdc: cdc, mbm: mbm, home: home,
		clientCtx: client.Context{}.WithCodec(cdc).WithHomeDir(home).
			WithTxConfig(authtx.NewTxConfig(cdc, authtx.DefaultSignModes)),
	}
}

func (f repair298Fixture) exportParams(t *testing.T) []byte {
	t.Helper()
	var chunks []json.RawMessage
	for chunk := range params.NewAppModule(f.keeper).ExportGenesisStream(f.ctx, f.cdc) {
		chunks = append(chunks, chunk)
	}
	require.Len(t, chunks, 1)
	return repair298ModuleLine(t, paramtypes.ModuleName, chunks[0])
}

func (f repair298Fixture) exportGenutil(t *testing.T) []byte {
	t.Helper()
	var chunks []json.RawMessage
	for chunk := range (genutil.AppModule{}).ExportGenesisStream(f.ctx, f.cdc) {
		chunks = append(chunks, chunk)
	}
	require.Len(t, chunks, 1)
	return repair298ModuleLine(t, "genutil", chunks[0])
}

func repair298ModuleLine(t *testing.T, name string, chunk json.RawMessage) []byte {
	t.Helper()
	line, err := json.Marshal(genutilcli.ModuleState{
		AppState: genutilcli.AppState{Module: name, Data: chunk},
	})
	require.NoError(t, err)
	return append(line, '\n')
}

func repair298Header(t *testing.T) []byte {
	t.Helper()
	header, err := json.Marshal(server.GenesisDocNoAppState{
		ChainID: "repair-298", InitialHeight: 1,
		GenesisTime: time.Unix(1, 0).UTC(), ConsensusParams: tmtypes.DefaultConsensusParams(),
	})
	require.NoError(t, err)
	return header
}

func (f repair298Fixture) write(t *testing.T, parts ...[]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "genesis.stream")
	require.NoError(t, os.WriteFile(path, bytes.Join(parts, nil), 0600))
	return path
}

func (f repair298Fixture) run(t *testing.T, ctx context.Context, file ...string) (string, error) {
	t.Helper()
	serverCtx := server.NewContext(viper.New(), tmcfg.DefaultConfig().SetRoot(f.home))
	ctx = context.WithValue(ctx, client.ClientContextKey, &f.clientCtx)
	ctx = context.WithValue(ctx, server.ServerContextKey, serverCtx)
	var output bytes.Buffer
	cmd := genutilcli.ValidateGenesisCmd(f.mbm)
	cmd.SetOut(&output)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs(append([]string{"--streaming"}, file...))
	result := make(chan error, 1)
	go func() { result <- cmd.ExecuteContext(ctx) }()
	select {
	case err := <-result:
		return output.String(), err
	case <-time.After(5 * time.Second):
		t.Fatal("stream validator did not finish and join its participants")
		return "", nil
	}
}

func TestRepair_298_InvalidFinalModuleChunk(t *testing.T) {
	f := newRepair298Fixture(t)
	defer leaktest.CheckTimeout(t, 2*time.Second)()
	valid := f.exportParams(t)
	space, ok := f.keeper.GetSubspace(paramtypes.ModuleName)
	require.True(t, ok)
	// Export an invalid persisted value through the real keeper and module exporter.
	space.Set(f.ctx, paramtypes.ParamStoreKeyCosmosGasParams, paramtypes.NewCosmosGasParams(1, 0))
	invalid := f.exportParams(t)
	for _, finalModule := range []bool{true, false} {
		name := "at_eof"
		if !finalModule {
			name = "before_next_module"
		}
		t.Run(name, func(t *testing.T) {
			parts := [][]byte{append(repair298Header(t), '\n'), valid, invalid}
			if !finalModule {
				parts = append(parts, f.exportGenutil(t))
			}
			path := f.write(t, parts...)
			for i := 0; i < 32; i++ {
				output, err := f.run(t, context.Background(), path)
				require.ErrorContains(t, err, "cosmos gas multiplier denominator can not be 0")
				require.NotContains(t, output, "is a valid genesis file")
			}
		})
	}
}

func TestRepair_298_EmptyMissingAndUnreadableInput(t *testing.T) {
	f := newRepair298Fixture(t)
	defer leaktest.CheckTimeout(t, 2*time.Second)()
	for _, tc := range []struct {
		name string
		file string
		want string
	}{
		{"empty", f.write(t), "missing a valid genesis header"},
		{"missing", filepath.Join(t.TempDir(), "missing"), "error opening genesis file"},
		{"read_error", t.TempDir(), "error reading genesis file"},
		{"no_header", f.write(t, f.exportParams(t)), "missing a valid genesis header"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := f.run(t, context.Background(), tc.file)
			require.ErrorContains(t, err, tc.want)
			require.NotContains(t, output, "is a valid genesis file")
		})
	}
	t.Run("missing_default", func(t *testing.T) {
		output, err := f.run(t, context.Background())
		require.ErrorContains(t, err, "error opening genesis file")
		require.NotContains(t, output, "is a valid genesis file")
	})
}

func TestRepair_298_ValidMultiChunkExport(t *testing.T) {
	f := newRepair298Fixture(t)
	defer leaktest.CheckTimeout(t, 2*time.Second)()
	first := f.exportParams(t)
	f.keeper.SetCosmosGasParams(f.ctx, paramtypes.NewCosmosGasParams(2, 3))
	second := f.exportParams(t)
	require.NotEqual(t, first, second)
	for _, headerFirst := range []bool{true, false} {
		name := "export_header_at_end"
		parts := [][]byte{f.exportGenutil(t), first, second, repair298Header(t)}
		if headerFirst {
			name = "header_at_start_final_module_at_eof"
			parts = [][]byte{append(repair298Header(t), '\n'), f.exportGenutil(t), first, bytes.TrimSuffix(second, []byte{'\n'})}
		}
		t.Run(name, func(t *testing.T) {
			path := f.write(t, parts...)
			output, err := f.run(t, context.Background(), path)
			require.NoError(t, err)
			require.Equal(t, "File at "+path+" is a valid genesis file\n", output)
		})
	}
}

func TestRepair_298_MalformedInputJoinsValidator(t *testing.T) {
	f := newRepair298Fixture(t)
	defer leaktest.CheckTimeout(t, 2*time.Second)()
	header := repair298Header(t)
	invalidHeader, err := json.Marshal(server.GenesisDocNoAppState{InitialHeight: -1})
	require.NoError(t, err)
	for _, tc := range []struct {
		name  string
		parts [][]byte
		want  string
	}{
		{"bad_last_line", [][]byte{append(header, '\n'), f.exportParams(t), []byte("{truncated")}, "error validating genesis doc"},
		{"invalid_header", [][]byte{f.exportParams(t), invalidHeader}, "chain_id"},
		{"duplicate_header", [][]byte{append(header, '\n'), f.exportParams(t), header}, "genesisDoc seen twice"},
		{"repeated_module", [][]byte{append(header, '\n'), f.exportParams(t), f.exportGenutil(t), f.exportParams(t)}, "module params seen twice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := f.run(t, context.Background(), f.write(t, tc.parts...))
			require.ErrorContains(t, err, tc.want)
			require.NotContains(t, output, "is a valid genesis file")
		})
	}
}

func TestRepair_298_CancelledInput(t *testing.T) {
	f := newRepair298Fixture(t)
	defer leaktest.CheckTimeout(t, 2*time.Second)()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	output, err := f.run(t, ctx, f.write(t, append(repair298Header(t), '\n'), f.exportParams(t)))
	require.ErrorIs(t, err, context.Canceled)
	require.NotContains(t, output, "is a valid genesis file")
}
