package keeper_test

import (
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	gigakeeper "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/keeper"
	gigatypes "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const gigaGenesisRefusal = "Giga bank genesis is unsupported; use the canonical bank keeper for initialization and recovery"

type bankRecoveryFixture struct {
	bank   bankkeeper.BaseKeeper
	giga   gigakeeper.BaseKeeper
	auth   authkeeper.AccountKeeper
	ctx    sdk.Context
	store  storetypes.CommitMultiStore
	cache  cachemulti.Store
	cdc    *codec.ProtoCodec
	closed bool
}

func openBankRecoveryFixture(t *testing.T, dir string) *bankRecoveryFixture {
	t.Helper()
	if _, ok := sdk.GetDenomUnit(sdk.DefaultBondDenom); !ok {
		require.NoError(t, sdk.RegisterDenom(sdk.DefaultBondDenom, sdk.OneDec()))
	}
	db, err := dbm.NewDB("bank", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	cms := store.NewCommitMultiStore(db)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range []*sdk.KVStoreKey{bankKey, authKey, paramsKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	bankParams := paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, banktypes.ModuleName)
	ak := authkeeper.NewAccountKeeper(cdc, authKey,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, authtypes.ModuleName),
		authtypes.ProtoBaseAccount, map[string][]string{})
	bk := bankkeeper.NewBaseKeeper(cdc, bankKey, ak, bankParams, map[string]bool{})
	gk := gigakeeper.NewBaseKeeper(cdc, bankKey, ak, bankParams, map[string]bool{})
	stores := map[storetypes.StoreKey]storetypes.CacheWrapper{}
	keys := map[string]storetypes.StoreKey{}
	for _, key := range []storetypes.StoreKey{bankKey, authKey, paramsKey, transientKey} {
		stores[key] = cms.GetKVStore(key)
		keys[key.Name()] = key
	}
	cache := cachemulti.NewFromKVStore(dbadapter.Store{DB: db}, stores, nil, keys,
		[]storetypes.StoreKey{bankKey}, nil, nil, 0)
	f := &bankRecoveryFixture{
		bank: bk, giga: gk, auth: ak, store: cms, cache: cache, cdc: cdc,
		ctx: sdk.NewContext(cms, tmproto.Header{Height: 1, Time: time.Unix(1, 0)}, false),
	}
	t.Cleanup(func() { f.close(t) })
	return f
}

func (f *bankRecoveryFixture) close(t *testing.T) {
	t.Helper()
	if !f.closed {
		f.cache.Close()
		require.NoError(t, f.store.Close())
		f.closed = true
	}
}

func bankRecoveryAddresses() (sdk.AccAddress, sdk.AccAddress) {
	return sdk.AccAddress([]byte("recovery_sender_____")), sdk.AccAddress([]byte("recovery_recipient__"))
}

func seedBankRecoveryFixture(t *testing.T, f *bankRecoveryFixture) {
	t.Helper()
	from, to := bankRecoveryAddresses()
	f.auth.SetParams(f.ctx, authtypes.DefaultParams())
	for _, addr := range []sdk.AccAddress{from, to} {
		f.auth.SetAccount(f.ctx, f.auth.NewAccountWithAddress(f.ctx, addr))
	}
	genesis := banktypes.DefaultGenesisState()
	genesis.Params = banktypes.NewParams(false, []*banktypes.SendEnabled{
		{Denom: "foo", Enabled: true}, {Denom: sdk.MustGetBaseDenom(), Enabled: false},
	})
	genesis.Balances = []banktypes.Balance{
		{Address: from.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("foo", 50), sdk.NewInt64Coin("bar", 30), sdk.NewInt64Coin(sdk.MustGetBaseDenom(), 10))},
		{Address: to.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("foo", 7))},
	}
	genesis.WeiBalances = []banktypes.WeiBalance{
		{Address: from.String(), Amount: sdk.OneInt()},
		{Address: to.String(), Amount: gigakeeper.OneUhpxInWei.SubRaw(1)},
	}
	genesis.DenomMetadata = []banktypes.Metadata{{Base: "foo", Display: "foo", Description: "recovered metadata", DenomUnits: []*banktypes.DenomUnit{{Denom: "foo", Exponent: 0}}}}
	f.bank.InitGenesis(f.ctx, genesis)
}

func assertBankRecovered(t *testing.T, f *bankRecoveryFixture) {
	t.Helper()
	from, to := bankRecoveryAddresses()
	ctx := f.ctx.WithMultiStore(f.cache)
	require.Equal(t, f.bank.GetParams(f.ctx), f.giga.GetParams(ctx))
	require.False(t, f.giga.GetParams(ctx).DefaultSendEnabled)
	for _, addr := range []sdk.AccAddress{from, to} {
		for _, denom := range []string{"foo", "bar", sdk.MustGetBaseDenom()} {
			require.Equal(t, f.bank.GetBalance(f.ctx, addr, denom), f.giga.GetBalance(ctx, addr, denom))
		}
		require.Equal(t, f.bank.GetWeiBalance(f.ctx, addr), f.giga.GetWeiBalance(ctx, addr))
	}
	require.Equal(t, sdk.OneInt(), f.giga.GetWeiBalance(ctx, from))
	require.Equal(t, gigakeeper.OneUhpxInWei.SubRaw(1), f.giga.GetWeiBalance(ctx, to))
	require.Equal(t, sdk.NewInt64Coin(sdk.MustGetBaseDenom(), 11), f.giga.GetSupply(ctx, sdk.MustGetBaseDenom()))
	require.NoError(t, f.giga.IsSendEnabledCoins(ctx, sdk.NewInt64Coin("foo", 1)))
	require.ErrorIs(t, f.giga.IsSendEnabledCoins(ctx, sdk.NewInt64Coin("bar", 1)), gigatypes.ErrSendDisabled)
	require.ErrorIs(t, f.giga.IsSendEnabledCoins(ctx, sdk.NewInt64Coin(sdk.MustGetBaseDenom(), 1)), gigatypes.ErrSendDisabled)
	metadata, ok := f.giga.GetDenomMetaData(ctx, "foo")
	require.True(t, ok)
	require.Equal(t, "recovered metadata", metadata.Description)
}

func TestBankRecoveryGenesisRefusalAndRestartRecovery(t *testing.T) {
	dir := t.TempDir()
	source := openBankRecoveryFixture(t, dir)
	seedBankRecoveryFixture(t, source)
	before := source.bank.ExportGenesis(source.ctx)
	for _, regular := range []bool{false, true} {
		giga := source.giga
		giga.UseRegularStore = regular
		ctx := source.ctx.WithMultiStore(source.cache)
		require.PanicsWithValue(t, gigaGenesisRefusal, func() { giga.InitGenesis(ctx, gigatypes.DefaultGenesisState()) })
		require.PanicsWithValue(t, gigaGenesisRefusal, func() { giga.InitGenesis(ctx, nil) })
		require.PanicsWithValue(t, gigaGenesisRefusal, func() { giga.ExportGenesis(ctx) })
		require.Equal(t, before, source.bank.ExportGenesis(source.ctx))
		assertBankRecovered(t, source)
	}
	// The supported producer emits the transport consumed by fresh bank genesis.
	transport := source.cdc.MustMarshalJSON(before)
	require.EqualValues(t, 1, source.store.Commit(true).Version)
	source.close(t)
	reopened := openBankRecoveryFixture(t, dir)
	require.Equal(t, before, reopened.bank.ExportGenesis(reopened.ctx))
	assertBankRecovered(t, reopened)
	reopened.close(t)

	freshDir := t.TempDir()
	fresh := openBankRecoveryFixture(t, freshDir)
	var recovered banktypes.GenesisState
	require.NoError(t, fresh.cdc.UnmarshalAsJSON(transport, &recovered))
	fresh.bank.InitGenesis(fresh.ctx, &recovered)
	require.Equal(t, before, fresh.bank.ExportGenesis(fresh.ctx))
	require.EqualValues(t, 1, fresh.store.Commit(true).Version)
	fresh.close(t)
	restarted := openBankRecoveryFixture(t, freshDir)
	require.Equal(t, before, restarted.bank.ExportGenesis(restarted.ctx))
	assertBankRecovered(t, restarted)
}

func TestBankRecoveryGigaViewAndTrace(t *testing.T) {
	TestGigaBankKeeperViewAndTrace(t)
}

func TestBankRecoveryCanonicalViewAndTrace(t *testing.T) {
	TestCanonicalBankKeeperViewAndTrace(t)
}
