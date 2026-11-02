package app_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	agentkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdkauth "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/stretchr/testify/require"
)

func mergedAppWrapper(t *testing.T) *app.TestWrapper {
	t.Helper()
	w := app.NewTestWrapper(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	return w
}

func mergedModuleOrder(t *testing.T, a *app.App, names ...string) []string {
	t.Helper()
	manager := reflect.ValueOf(a).Elem().FieldByName("mm")
	require.False(t, manager.IsNil())
	modules := manager.Elem().FieldByName("Modules")
	for _, name := range names {
		require.Contains(t, app.ModuleBasics, name)
		require.True(t, modules.MapIndex(reflect.ValueOf(name)).IsValid(), name)
	}
	order := manager.Elem().FieldByName("OrderInitGenesis")
	result := make([]string, order.Len())
	for i := range result {
		result[i] = order.Index(i).String()
	}
	return result
}

func TestAppMountsMergedModuleStores(t *testing.T) {
	w := mergedAppWrapper(t)
	names := []string{keys.LayerXLedgerStoreKey, keys.LayerXAuthStoreKey, keys.LayerXPayStoreKey,
		keys.LayerXProgramsStoreKey, keys.LayerXAgentStoreKey, app.LayerXHostStoreKey}
	for i, name := range names {
		key := w.App.GetKey(name)
		require.NotNil(t, key, name)
		store := w.Ctx.KVStore(key)
		value := []byte{byte(i + 1)}
		store.Set([]byte("mounted-store-probe"), value)
		require.Equal(t, value, store.Get([]byte("mounted-store-probe")), name)
	}
	for i, name := range names {
		require.Equal(t, []byte{byte(i + 1)}, w.Ctx.KVStore(w.App.GetKey(name)).Get([]byte("mounted-store-probe")), name)
	}
	mounted := reflect.ValueOf(w.App).Elem().FieldByName("keys")
	require.Equal(t, len(keys.MemIAVLStoreKeys)+len(names), mounted.Len())
	for _, name := range keys.MemIAVLStoreKeys {
		require.NotNil(t, w.App.GetKey(name), name)
	}
}

func TestAppRegistersLedgerAuthModules(t *testing.T) {
	w := mergedAppWrapper(t)
	order := mergedModuleOrder(t, w.App, ledgertypes.ModuleName, authtypes.ModuleName)
	ledgerAt, authAt := -1, -1
	for i, name := range order {
		if name == ledgertypes.ModuleName {
			ledgerAt = i
		}
		if name == authtypes.ModuleName {
			authAt = i
		}
	}
	require.GreaterOrEqual(t, ledgerAt, 0)
	require.Equal(t, ledgerAt+1, authAt)
	ledger := ledgertypes.DefaultGenesis()
	ledger.Params.Paused = true
	auth := authtypes.DefaultGenesis()
	auth.Params.Paused = true
	auth.Params.IdempotencyRetention = 73
	state, err := json.Marshal(map[string]json.RawMessage{
		ledgertypes.ModuleName: w.App.AppCodec().MustMarshalJSON(ledger),
		authtypes.ModuleName:   w.App.AppCodec().MustMarshalJSON(auth),
	})
	require.NoError(t, err)
	w.App.InitChainer(w.Ctx, abci.RequestInitChain{AppStateBytes: state})
	require.Equal(t, ledger.Params, w.App.LayerXLedgerKeeper.GetParams(w.Ctx))
	require.Equal(t, auth.Params, w.App.LayerXAuthKeeper.GetParams(w.Ctx))
	require.True(t, w.Ctx.KVStore(w.App.GetKey(keys.LayerXLedgerStoreKey)).Has(ledgertypes.ParamsKey))
	require.True(t, w.Ctx.KVStore(w.App.GetKey(keys.LayerXAuthStoreKey)).Has(authtypes.ParamsKey))
}

func TestAppLedgerModuleAccountPerms(t *testing.T) {
	require.ElementsMatch(t, []string{sdkauth.Minter, sdkauth.Burner}, app.GetMaccPerms()[ledgertypes.ModuleName])
	w := mergedAppWrapper(t)
	account := w.App.AccountKeeper.GetModuleAccount(w.Ctx, ledgertypes.ModuleName)
	require.NotNil(t, account)
	require.ElementsMatch(t, []string{sdkauth.Minter, sdkauth.Burner}, account.GetPermissions())
}

func TestAppEndBlockKeepersCarryAgentScheduler(t *testing.T) {
	w := mergedAppWrapper(t)
	keepers := w.App.EndBlockKeepers
	require.Same(t, &w.App.LayerXAgentKeeper.Keeper, keepers.AgentKeeper)
	require.NotNil(t, keepers.MergeLive)
	deliver := w.App.DeliverContext()
	require.NotNil(t, deliver)
	*deliver = deliver.WithBlockHeight(9)
	require.False(t, keepers.MergeLive())
	require.NoError(t, w.App.SequencerKeeper.SetActivationHeight(w.Ctx, 10))
	require.False(t, keepers.MergeLive())
	*deliver = deliver.WithBlockHeight(10)
	require.True(t, keepers.MergeLive())
	*deliver = deliver.WithBlockHeight(11)
	require.True(t, keepers.MergeLive())
	require.Error(t, keepers.AgentKeeper.Active(w.Ctx.WithBlockHeight(10)))
	w.App.UpgradeKeeper.SetDone(w.Ctx.WithBlockHeight(10), app.MergeUpgrade)
	require.NoError(t, keepers.AgentKeeper.Active(w.Ctx.WithBlockHeight(10)))
	params := agenttypes.DefaultParams()
	params.MaxJobsPerBlock = 7
	require.NoError(t, keepers.AgentKeeper.SetParams(w.Ctx, params))
	require.Equal(t, params, w.App.LayerXAgentKeeper.GetParams(w.Ctx))
	require.True(t, w.Ctx.KVStore(w.App.GetKey(keys.LayerXAgentStoreKey)).Has(agentkeeper.ParamsKey))
}
