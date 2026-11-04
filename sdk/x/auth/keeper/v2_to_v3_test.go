package keeper_test

import (
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc"
)

type migrationM657Fixture struct {
	ctx      sdk.Context
	store    storetypes.CommitMultiStore
	subspace paramtypes.Subspace
	keeper   keeper.AccountKeeper
	manager  *module.Manager
	config   module.Configurator
}

func newMigrationM657Fixture(t *testing.T, db dbm.DB) migrationM657Fixture {
	t.Helper()
	ms := store.NewCommitMultiStore(db)
	accountKey := sdk.NewKVStoreKey(types.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	ms.MountStoreWithDB(accountKey, sdk.StoreTypeIAVL, nil)
	ms.MountStoreWithDB(paramsKey, sdk.StoreTypeIAVL, nil)
	ms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, ms.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	types.RegisterLegacyAminoCodec(amino)
	subspace := paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, types.ModuleName).
		WithKeyTable(types.ParamKeyTable())
	ak := keeper.NewAccountKeeper(cdc, accountKey, subspace, types.ProtoBaseAccount, nil)
	am := auth.NewAppModule(cdc, ak, nil)
	msgServer, queryServer := grpc.NewServer(), grpc.NewServer()
	t.Cleanup(msgServer.Stop)
	t.Cleanup(queryServer.Stop)
	config := module.NewConfigurator(cdc, msgServer, queryServer)
	am.RegisterServices(config)

	return migrationM657Fixture{
		ctx: sdk.NewContext(ms, tmproto.Header{}, false), store: ms,
		subspace: subspace, keeper: ak, manager: module.NewManager(am), config: config,
	}
}

func seedMigrationM657LegacyParams(t *testing.T, fixture migrationM657Fixture) types.Params {
	t.Helper()
	params := types.Params{
		MaxMemoCharacters: 513, TxSigLimit: 13, TxSizeCostPerByte: 19,
		SigVerifyCostED25519: 811, SigVerifyCostSecp256k1: 1217,
	}
	require.NoError(t, params.Validate())
	for _, pair := range params.ParamSetPairs() {
		if string(pair.Key) != string(types.KeyDisableSeqnoCheck) {
			fixture.subspace.Set(fixture.ctx, pair.Key, *pair.Value.(*uint64))
		}
	}
	require.False(t, fixture.subspace.Has(fixture.ctx, types.KeyDisableSeqnoCheck))
	return params
}

func assertMigrationM657Params(t *testing.T, fixture migrationM657Fixture, want types.Params) {
	t.Helper()
	got := fixture.keeper.GetParams(fixture.ctx)
	require.Equal(t, want.MaxMemoCharacters, got.MaxMemoCharacters)
	require.Equal(t, want.TxSigLimit, got.TxSigLimit)
	require.Equal(t, want.TxSizeCostPerByte, got.TxSizeCostPerByte)
	require.Equal(t, want.SigVerifyCostED25519, got.SigVerifyCostED25519)
	require.Equal(t, want.SigVerifyCostSecp256k1, got.SigVerifyCostSecp256k1)
	require.True(t, fixture.subspace.Has(fixture.ctx, types.KeyDisableSeqnoCheck))
	require.False(t, got.DisableSeqnoCheck)
	require.Equal(t, want, got)
}

func testMigrationM657Registered(t *testing.T) {
	t.Helper()
	fixture := newMigrationM657Fixture(t, dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, fixture.store.Close()) })
	want := seedMigrationM657LegacyParams(t, fixture)
	for attempt := 0; attempt < 2; attempt++ {
		versions, err := fixture.manager.RunMigrations(fixture.ctx, fixture.config, module.VersionMap{types.ModuleName: 2})
		require.NoError(t, err)
		require.Equal(t, module.VersionMap{types.ModuleName: 3}, versions)
		assertMigrationM657Params(t, fixture, want)
	}
}

func TestMigrate2to3(t *testing.T) {
	testMigrationM657Registered(t)
}
