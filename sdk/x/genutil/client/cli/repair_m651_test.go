package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	genutiltypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

type repairM651App struct {
	ctx       sdk.Context
	cdc       *codec.ProtoCodec
	keeper    paramkeeper.Keeper
	module    params.AppModule
	clientCtx client.Context
}

func newRepairM651App(t *testing.T) repairM651App {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	basic := params.AppModuleBasic{}
	basic.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	basic.RegisterLegacyAminoCodec(amino)
	key := sdk.NewKVStoreKey(paramtypes.ModuleName)
	tkey := sdk.NewTransientStoreKey("transient_params")
	ctx := testutil.DefaultContext(key, tkey)
	keeper := paramkeeper.NewKeeper(cdc, amino, key, tkey)
	return repairM651App{
		ctx: ctx, cdc: cdc, keeper: keeper, module: params.NewAppModule(keeper),
		clientCtx: client.Context{}.WithCodec(cdc).WithLegacyAmino(amino),
	}
}

func repairM651Registry(t *testing.T) {
	t.Helper()
	migrationMutex.Lock()
	previous := migrationMap
	migrationMap = genutiltypes.MigrationMap{}
	migrationMutex.Unlock()
	t.Cleanup(func() {
		migrationMutex.Lock()
		migrationMap = previous
		migrationMutex.Unlock()
	})
}

// The application adapter imports the old fees-only params schema, runs the
// production v1-to-v2 store migration, then exports through the module codec.
func (app repairM651App) migration(t *testing.T) genutiltypes.MigrationCallback {
	t.Helper()
	return func(state genutiltypes.AppMap, clientCtx client.Context) genutiltypes.AppMap {
		require.Same(t, app.cdc, clientCtx.Codec)
		var old paramtypes.GenesisState
		clientCtx.Codec.MustUnmarshalJSON(state[paramtypes.ModuleName], &old)
		require.Equal(t, paramtypes.CosmosGasParams{}, old.CosmosGasParams)
		app.keeper.SetFeesParams(app.ctx, old.FeesParams)
		require.NoError(t, paramkeeper.NewMigrator(app.keeper).Migrate1to2(app.ctx))
		state[paramtypes.ModuleName] = app.module.ExportGenesis(app.ctx, clientCtx.Codec)
		return state
	}
}

func (app repairM651App) oldGenesis(t *testing.T) (string, *tmtypes.GenesisDoc, paramtypes.FeesParams) {
	t.Helper()
	fees := paramtypes.NewFeesParams(sdk.DecCoins{sdk.NewDecCoinFromDec("stake", sdk.NewDecWithPrec(7, 2))})
	fees.AllowedFeeDenoms = []string{"stake"}
	app.keeper.InitGenesis(app.ctx, paramtypes.NewGenesisState(fees, *paramtypes.DefaultCosmosGasParams()))
	var oldParams map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(app.module.ExportGenesis(app.ctx, app.cdc), &oldParams))
	// Version 1 predates the cosmos_gas_params field added by Migrate1to2.
	delete(oldParams, "cosmos_gas_params")
	paramsJSON, err := json.Marshal(oldParams)
	require.NoError(t, err)
	untouched, err := json.Marshal(map[string]string{"value": "9007199254740993"})
	require.NoError(t, err)
	state, err := json.Marshal(genutiltypes.AppMap{paramtypes.ModuleName: paramsJSON, "unrelated": untouched})
	require.NoError(t, err)
	doc := &tmtypes.GenesisDoc{
		ChainID: "repair-m651-old", InitialHeight: 42,
		GenesisTime: time.Unix(1234, 0).UTC(), ConsensusParams: tmtypes.DefaultConsensusParams(),
		AppState: state,
	}
	path := filepath.Join(t.TempDir(), "genesis.json")
	require.NoError(t, doc.SaveAs(path))
	return path, doc, fees
}

func repairM651Execute(cmd *cobra.Command, clientCtx client.Context, args ...string) (string, error) {
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	ctx := context.WithValue(context.Background(), client.ClientContextKey, &clientCtx)
	err := cmd.ExecuteContext(ctx)
	return output.String(), err
}

func TestRepairM_651_RegistryAndHelp(t *testing.T) {
	repairM651Registry(t)
	app := newRepairM651App(t)
	callback := app.migration(t)
	require.Empty(t, GetMigrationVersions())
	require.Nil(t, GetMigrationCallback("params-v2"))
	beforeRegistration := MigrateGenesisCmd()
	output, err := repairM651Execute(MigrateGenesisCmd(), app.clientCtx, "--help")
	require.NoError(t, err)
	require.Contains(t, output, "Supported target versions: none (this application has not registered any genesis migrations)")
	require.NotContains(t, output, "v0.36")

	for _, target := range []string{"", " ", " params-v2", "params-v2\n", "two targets"} {
		require.Error(t, RegisterMigration(target, callback))
	}
	require.ErrorContains(t, RegisterMigration("params-v2", nil), "must not be nil")
	require.Empty(t, GetMigrationVersions())
	require.NoError(t, RegisterMigration("z-params-v2", callback))
	require.NoError(t, RegisterMigration("params-v2", callback))
	replacement := newRepairM651App(t)
	require.ErrorContains(t, RegisterMigration("params-v2", replacement.migration(t)), "already registered")
	require.Equal(t, []string{"params-v2", "z-params-v2"}, GetMigrationVersions())
	require.NotNil(t, GetMigrationCallback("params-v2"))
	versions := GetMigrationVersions()
	versions[0] = "changed-by-caller"
	require.Equal(t, []string{"params-v2", "z-params-v2"}, GetMigrationVersions())
	cmd := MigrateGenesisCmd()
	require.Equal(t, GetMigrationVersions(), cmd.ValidArgs)
	output, err = repairM651Execute(cmd, app.clientCtx, "--help")
	require.NoError(t, err)
	require.Contains(t, output, "Supported target versions: params-v2, z-params-v2")
	require.NotContains(t, output, "Supported target versions: none")

	path, _, _ := newRepairM651App(t).oldGenesis(t)
	output, err = repairM651Execute(beforeRegistration, app.clientCtx, "params-v2", path)
	require.ErrorContains(t, err, "supported target versions: none")
	require.Empty(t, output)
	output, err = repairM651Execute(MigrateGenesisCmd(), app.clientCtx, "params-v2", path)
	require.NoError(t, err)
	require.True(t, json.Valid([]byte(output)))
	space, ok := replacement.keeper.GetSubspace(paramtypes.ModuleName)
	require.True(t, ok)
	require.False(t, space.Has(replacement.ctx, paramtypes.ParamStoreKeyFeesParams))
}

func TestRepairM_651_OldGenesisToCurrentGenesis(t *testing.T) {
	repairM651Registry(t)
	producer := newRepairM651App(t)
	path, oldDoc, fees := producer.oldGenesis(t)
	var oldState genutiltypes.AppMap
	require.NoError(t, json.Unmarshal(oldDoc.AppState, &oldState))
	require.ErrorContains(t, (params.AppModuleBasic{}).ValidateGenesis(producer.cdc, nil, oldState[paramtypes.ModuleName]), "cosmos gas multiplier numerator can not be 0")
	for _, override := range []bool{false, true} {
		name := "preserve_metadata"
		if override {
			name = "override_metadata"
		}
		t.Run(name, func(t *testing.T) {
			app := newRepairM651App(t)
			target := "params-v2-" + name
			require.NoError(t, RegisterMigration(target, app.migration(t)))
			args := []string{target, path}
			if override {
				args = append(args, "--chain-id=repair-m651-current", "--genesis-time=2020-01-02T03:04:05Z")
			}
			output, err := repairM651Execute(MigrateGenesisCmd(), app.clientCtx, args...)
			require.NoError(t, err)
			current, err := tmtypes.GenesisDocFromJSON([]byte(output))
			require.NoError(t, err)
			require.Equal(t, oldDoc.InitialHeight, current.InitialHeight)
			require.Equal(t, oldDoc.ConsensusParams, current.ConsensusParams)
			if override {
				require.Equal(t, "repair-m651-current", current.ChainID)
				require.Equal(t, "2020-01-02T03:04:05Z", current.GenesisTime.Format(time.RFC3339))
			} else {
				require.Equal(t, oldDoc.ChainID, current.ChainID)
				require.Equal(t, oldDoc.GenesisTime, current.GenesisTime)
			}
			var state, oldState genutiltypes.AppMap
			require.NoError(t, json.Unmarshal(current.AppState, &state))
			require.NoError(t, json.Unmarshal(oldDoc.AppState, &oldState))
			require.Len(t, state, len(oldState))
			require.JSONEq(t, string(oldState["unrelated"]), string(state["unrelated"]))
			require.NotEqual(t, string(oldState[paramtypes.ModuleName]), string(state[paramtypes.ModuleName]))
			require.NoError(t, (params.AppModuleBasic{}).ValidateGenesis(app.cdc, nil, state[paramtypes.ModuleName]))
			var migrated paramtypes.GenesisState
			require.NoError(t, app.cdc.UnmarshalAsJSON(state[paramtypes.ModuleName], &migrated))
			require.Equal(t, fees, migrated.FeesParams)
			require.Equal(t, *paramtypes.DefaultCosmosGasParams(), migrated.CosmosGasParams)
			consumer := newRepairM651App(t)
			consumer.module.InitGenesis(consumer.ctx, consumer.cdc, state[paramtypes.ModuleName])
			require.Equal(t, fees, consumer.keeper.GetFeesParams(consumer.ctx))
			require.Equal(t, *paramtypes.DefaultCosmosGasParams(), consumer.keeper.GetCosmosGasParams(consumer.ctx))
		})
	}
	unchanged, err := tmtypes.GenesisDocFromFile(path)
	require.NoError(t, err)
	require.JSONEq(t, string(oldDoc.AppState), string(unchanged.AppState))
}

func TestRepairM_651_UnknownTarget(t *testing.T) {
	repairM651Registry(t)
	app := newRepairM651App(t)
	require.NoError(t, RegisterMigration("params-v2", app.migration(t)))
	path, _, _ := newRepairM651App(t).oldGenesis(t)
	output, err := repairM651Execute(MigrateGenesisCmd(), app.clientCtx, "unsupported", path)
	require.ErrorContains(t, err, "unknown migration function for version: unsupported")
	require.ErrorContains(t, err, "supported target versions: params-v2")
	require.Empty(t, output)
	space, ok := app.keeper.GetSubspace(paramtypes.ModuleName)
	require.True(t, ok)
	require.False(t, space.Has(app.ctx, paramtypes.ParamStoreKeyFeesParams))
	require.False(t, space.Has(app.ctx, paramtypes.ParamStoreKeyCosmosGasParams))
}
