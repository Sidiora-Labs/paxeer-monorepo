package module_test

import (
	"encoding/json"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramstypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// Task 22.33.648 record:
// migration-downgrade-version: TestRepairM_648_DowngradeRestartRecovery and
// TestRepairM_648_ForwardAndUnchangedVersions.
// stream-validation-forwarder: TestRepairM_648_StreamRejectsChunk,
// TestRepairM_648_StreamEOF, TestRepairM_648_StreamCancellationBetweenChunks,
// TestRepairM_648_StreamCancellationDuringForwarding, and
// TestRepairM_648_StreamCancellationPreservesError.
// No out-of-path wiring is required. The explicit implementation-first gate
// order forbids the embedded unchanged-tree verification; no baseline result
// is claimed. The runner's gate ledger records the qualified revision.

type repairM648State struct {
	*repair295State
	upgrade      upgradekeeper.Keeper
	authModule   auth.AppModule
	paramsModule params.AppModule
	manager      *module.Manager
}

func repairM648Open(t *testing.T, directory string) *repairM648State {
	t.Helper()
	db, err := dbm.NewDB("repairM648", dbm.GoLevelDBBackend, directory)
	require.NoError(t, err)
	cms := store.NewCommitMultiStore(db)
	paramsKey := sdk.NewKVStoreKey(paramstypes.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	upgradeKey := sdk.NewKVStoreKey(upgradetypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramstypes.TStoreKey)
	for _, key := range []sdk.StoreKey{paramsKey, authKey, upgradeKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	}
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	pk := paramskeeper.NewKeeper(cdc, codec.NewLegacyAmino(), paramsKey, transientKey)
	ak := authkeeper.NewAccountKeeper(cdc, authKey, pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{authtypes.FeeCollectorName: nil})
	uk := upgradekeeper.NewKeeper(nil, upgradeKey, cdc, directory, nil)
	am := auth.NewAppModule(cdc, ak, nil)
	pm := params.NewAppModule(pk)
	mm := module.NewManager(pm, am, upgrade.NewAppModule(uk))
	s := &repairM648State{
		repair295State: &repair295State{
			ctx: sdk.NewContext(cms, tmproto.Header{ChainID: "repairM648", Height: 1}, false),
			cms: cms, cdc: cdc, params: pk, auth: ak, manager: mm,
			keys: []sdk.StoreKey{paramsKey, authKey, upgradeKey},
		},
		upgrade: uk, authModule: am, paramsModule: pm, manager: mm,
	}
	t.Cleanup(func() { s.close(t) })
	return s
}

func (s *repairM648State) configurator(t *testing.T) module.Configurator {
	t.Helper()
	cfg := module.NewConfigurator(s.cdc, nil, nil)
	require.NoError(t, cfg.RegisterMigration(paramstypes.ModuleName, 1, paramskeeper.NewMigrator(s.params).Migrate1to2))
	am := authkeeper.NewMigrator(s.auth, nil)
	require.NoError(t, cfg.RegisterMigration(authtypes.ModuleName, 1, am.Migrate1to2))
	require.NoError(t, cfg.RegisterMigration(authtypes.ModuleName, 2, am.Migrate2to3))
	return cfg
}

func TestRepairM_648_DowngradeRestartRecovery(t *testing.T) {
	for _, tc := range []struct {
		name       string
		regressing string
		preceding  string
	}{
		{name: "initial_version", regressing: upgradetypes.ModuleName},
		{name: "existing_migrations", regressing: authtypes.ModuleName},
		{name: "before_prior_migration", regressing: authtypes.ModuleName, preceding: "migration"},
		{name: "before_prior_genesis", regressing: authtypes.ModuleName, preceding: "genesis"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			original := repairM648Open(t, directory)
			original.seed(7, 9)
			versions := original.manager.GetVersionMap()
			versions[tc.regressing]++
			if tc.preceding == "migration" {
				versions[paramstypes.ModuleName] = 1
			} else if tc.preceding == "genesis" {
				delete(versions, paramstypes.ModuleName)
			}
			original.upgrade.SetModuleVersionMap(original.ctx, versions)
			original.cms.Commit(true)
			before := original.snapshot(t)
			original.close(t)

			recovered := repairM648Open(t, directory)
			require.Equal(t, versions, recovered.upgrade.GetModuleVersionMap(recovered.ctx))
			require.Equal(t, before, recovered.snapshot(t))
			recovered.manager.SetOrderMigrations(paramstypes.ModuleName, authtypes.ModuleName, upgradetypes.ModuleName)
			cfg := recovered.configurator(t)
			fromVM := recovered.upgrade.GetModuleVersionMap(recovered.ctx)
			updated, err := recovered.manager.RunMigrations(recovered.ctx, cfg, fromVM)
			require.ErrorIs(t, err, sdkerrors.ErrInvalidVersion)
			require.Contains(t, err.Error(), tc.regressing)
			require.Nil(t, updated)
			require.Equal(t, versions, fromVM)
			require.Equal(t, before, recovered.snapshot(t))

			recovered.upgrade.SetUpgradeHandler("downgrade", func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
				return recovered.manager.RunMigrations(ctx, cfg, vm)
			})
			var failure interface{}
			func() {
				defer func() { failure = recover() }()
				recovered.upgrade.ApplyUpgrade(recovered.ctx, upgradetypes.Plan{Name: "downgrade", Height: 1})
			}()
			require.ErrorIs(t, failure.(error), sdkerrors.ErrInvalidVersion)
			require.Equal(t, before, recovered.snapshot(t))
			recovered.cms.Commit(true)
			recovered.close(t)

			after := repairM648Open(t, directory)
			require.Equal(t, before, after.snapshot(t))
			require.Equal(t, versions, after.upgrade.GetModuleVersionMap(after.ctx))
			require.Equal(t, uint64(9), after.params.GetCosmosGasParams(after.ctx).CosmosGasMultiplierDenominator)
			require.Equal(t, uint64(7), after.auth.GetAccount(after.ctx, repair295Address(7)).GetSequence())
		})
	}
}

func TestRepairM_648_ForwardAndUnchangedVersions(t *testing.T) {
	directory := t.TempDir()
	s := repairM648Open(t, directory)
	s.seed(7, 9)
	versions := s.manager.GetVersionMap()
	cfg := s.configurator(t)
	before := s.snapshot(t)
	updated, err := s.manager.RunMigrations(s.ctx, cfg, versions)
	require.NoError(t, err)
	require.Equal(t, versions, updated)
	require.Equal(t, before, s.snapshot(t))
	versions[paramstypes.ModuleName] = 1
	s.upgrade.SetModuleVersionMap(s.ctx, versions)
	s.upgrade.SetUpgradeHandler("forward", func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		return s.manager.RunMigrations(ctx, cfg, vm)
	})
	s.upgrade.ApplyUpgrade(s.ctx, upgradetypes.Plan{Name: "forward", Height: 1})
	require.Equal(t, s.manager.GetVersionMap(), s.upgrade.GetModuleVersionMap(s.ctx))
	require.Equal(t, *paramstypes.DefaultCosmosGasParams(), s.params.GetCosmosGasParams(s.ctx))
	s.cms.Commit(true)
	s.close(t)
	recovered := repairM648Open(t, directory)
	require.Equal(t, recovered.manager.GetVersionMap(), recovered.upgrade.GetModuleVersionMap(recovered.ctx))
	require.Equal(t, *paramstypes.DefaultCosmosGasParams(), recovered.params.GetCosmosGasParams(recovered.ctx))
}

// This observer always delegates validation to the production module. Its
// barriers expose worker lifetime and let the tests control scheduling.
type repairM648ObservedModule struct {
	module.AppModuleBasic
	started       chan struct{}
	exited        chan struct{}
	release       <-chan struct{}
	result        chan error
	returnRelease <-chan struct{}
}

func (m repairM648ObservedModule) ValidateGenesisStream(cdc codec.JSONCodec, config client.TxEncodingConfig, ch <-chan json.RawMessage) error {
	defer close(m.exited)
	close(m.started)
	if m.release != nil {
		<-m.release
	}
	err := m.AppModuleBasic.ValidateGenesisStream(cdc, config, ch)
	if m.result != nil {
		m.result <- err
	}
	if m.returnRelease != nil {
		<-m.returnRelease
	}
	return err
}

func repairM648Observe(basic module.AppModuleBasic) repairM648ObservedModule {
	return repairM648ObservedModule{AppModuleBasic: basic, started: make(chan struct{}), exited: make(chan struct{})}
}

func repairM648Wait(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("stream participant did not finish")
	}
}

func repairM648Start(s *repairM648State, observed repairM648ObservedModule, input <-chan json.RawMessage, cancel <-chan struct{}, errors chan<- error) <-chan struct{} {
	completed := make(chan struct{})
	go func() {
		defer close(completed)
		module.NewBasicManager(observed).ValidateGenesisStream(s.cdc, nil, observed.Name(), input, cancel, errors)
	}()
	return completed
}

func repairM648Joined(t *testing.T, observed repairM648ObservedModule, completed <-chan struct{}) {
	t.Helper()
	repairM648Wait(t, completed)
	select {
	case <-observed.exited:
	default:
		t.Fatal("manager returned before joining its validator")
	}
}

func repairM648InvalidExport(t *testing.T, s *repairM648State) json.RawMessage {
	t.Helper()
	// Persist a corrupt parameter through the real subspace and export it
	// through the module, so validation sees the actual producer's encoding.
	subspace, found := s.params.GetSubspace(paramstypes.ModuleName)
	require.True(t, found)
	invalid := *paramstypes.DefaultCosmosGasParams()
	invalid.CosmosGasMultiplierDenominator = 0
	subspace.Set(s.ctx, paramstypes.ParamStoreKeyCosmosGasParams, invalid)
	stream := s.paramsModule.ExportGenesisStream(s.ctx, s.cdc)
	chunk, ok := <-stream
	require.True(t, ok)
	_, ok = <-stream
	require.False(t, ok)
	return chunk
}

func TestRepairM_648_StreamRejectsChunk(t *testing.T) {
	s := repairM648Open(t, t.TempDir())
	s.seed(7, 9)
	valid := s.paramsModule.ExportGenesis(s.ctx, s.cdc)
	invalid := repairM648InvalidExport(t, s)
	expected := s.paramsModule.ValidateGenesis(s.cdc, nil, invalid)
	require.EqualError(t, expected, "cosmos gas multiplier denominator can not be 0")
	input := make(chan json.RawMessage, 3)
	input <- valid
	input <- invalid
	input <- valid
	defer close(input)
	errors := make(chan error, 1)
	observed := repairM648Observe(s.paramsModule)
	completed := repairM648Start(s, observed, input, nil, errors)
	repairM648Joined(t, observed, completed)
	require.Len(t, errors, 1)
	require.EqualError(t, <-errors, expected.Error())
}

func TestRepairM_648_StreamEOF(t *testing.T) {
	s := repairM648Open(t, t.TempDir())
	s.seed(7, 9)
	for _, am := range []module.AppModule{s.authModule, s.paramsModule, upgrade.NewAppModule(s.upgrade)} {
		t.Run(am.Name(), func(t *testing.T) {
			observed := repairM648Observe(am)
			errors := make(chan error, 1)
			completed := repairM648Start(s, observed, am.ExportGenesisStream(s.ctx, s.cdc), nil, errors)
			repairM648Joined(t, observed, completed)
			require.Empty(t, errors)
		})
	}
	t.Run("empty", func(t *testing.T) {
		input := make(chan json.RawMessage)
		close(input)
		observed := repairM648Observe(s.authModule)
		errors := make(chan error, 1)
		repairM648Joined(t, observed, repairM648Start(s, observed, input, nil, errors))
		require.Empty(t, errors)
	})
}

func TestRepairM_648_StreamCancellationBetweenChunks(t *testing.T) {
	s := repairM648Open(t, t.TempDir())
	s.seed(7, 9)
	input := make(chan json.RawMessage)
	defer close(input)
	cancel := make(chan struct{})
	errors := make(chan error, 1)
	observed := repairM648Observe(s.authModule)
	completed := repairM648Start(s, observed, input, cancel, errors)
	repairM648Wait(t, observed.started)
	chunk := s.authModule.ExportGenesis(s.ctx, s.cdc)
	for i := 0; i < 2; i++ {
		select {
		case input <- chunk:
		case <-time.After(3 * time.Second):
			t.Fatal("manager stopped consuming valid chunks")
		}
	}
	close(cancel)
	repairM648Joined(t, observed, completed)
	require.Empty(t, errors)
}

func TestRepairM_648_StreamCancellationDuringForwarding(t *testing.T) {
	s := repairM648Open(t, t.TempDir())
	s.seed(7, 9)
	input := make(chan json.RawMessage)
	defer close(input)
	cancel := make(chan struct{})
	release := make(chan struct{})
	observed := repairM648Observe(s.authModule)
	observed.release = release
	errors := make(chan error, 1)
	completed := repairM648Start(s, observed, input, cancel, errors)
	repairM648Wait(t, observed.started)
	select {
	case input <- s.authModule.ExportGenesis(s.ctx, s.cdc):
	case <-time.After(3 * time.Second):
		t.Fatal("manager did not accept the chunk")
	}
	close(cancel)
	// The manager must close the input and then wait for the real validator.
	select {
	case <-completed:
		t.Fatal("manager did not join the paused validator")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	repairM648Joined(t, observed, completed)
	require.Empty(t, errors)
}

func TestRepairM_648_StreamCancellationPreservesError(t *testing.T) {
	s := repairM648Open(t, t.TempDir())
	s.seed(7, 9)
	input := make(chan json.RawMessage, 1)
	input <- repairM648InvalidExport(t, s)
	defer close(input)
	cancel := make(chan struct{})
	release := make(chan struct{})
	observed := repairM648Observe(s.paramsModule)
	observed.result = make(chan error, 1)
	observed.returnRelease = release
	errors := make(chan error, 1)
	completed := repairM648Start(s, observed, input, cancel, errors)
	select {
	case err := <-observed.result:
		require.EqualError(t, err, "cosmos gas multiplier denominator can not be 0")
	case <-time.After(3 * time.Second):
		t.Fatal("real module did not reject corrupt exported state")
	}
	close(cancel)
	close(release)
	repairM648Joined(t, observed, completed)
	require.Len(t, errors, 1)
	require.EqualError(t, <-errors, "cosmos gas multiplier denominator can not be 0")
}
