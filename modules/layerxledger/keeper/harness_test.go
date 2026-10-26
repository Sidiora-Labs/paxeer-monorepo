package keeper_test

import (
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

type harness struct {
	ctx       sdk.Context
	keeper    *ledgerkeeper.Keeper
	host      *lxphost.Host
	storeKey  *storetypes.KVStoreKey
	hostKey   *storetypes.KVStoreKey
	authority string
}

func mounted(parent storetypes.MultiStore, key storetypes.StoreKey) (store storetypes.KVStore, ok bool) {
	defer func() {
		if recover() != nil {
			store, ok = nil, false
		}
	}()
	return parent.GetKVStore(key), true
}

func withStore(ctx sdk.Context, key storetypes.StoreKey) sdk.Context {
	parent := ctx.MultiStore()
	stores := map[storetypes.StoreKey]storetypes.CacheWrapper{}
	keys := map[string]storetypes.StoreKey{}
	for _, existing := range parent.StoreKeys() {
		if store, ok := mounted(parent, existing); ok {
			stores[existing] = store
			keys[existing.Name()] = existing
		}
	}
	stores[key] = dbadapter.Store{DB: tmdb.NewMemDB()}
	keys[key.Name()] = key
	return ctx.WithMultiStore(cachemulti.NewStore(tmdb.NewMemDB(), stores, keys, nil, nil, nil, 0))
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx, _ := app.NewContext(false, tmtypes.Header{}).
		WithBlockHeight(11).WithBlockTime(time.Unix(1_800_000_000, 0).UTC()).CacheContext()
	key := sdk.NewKVStoreKey(types.StoreKey)
	hostKey := sdk.NewKVStoreKey("lxphost")
	ctx = withStore(withStore(ctx, key), hostKey)
	host, err := lxphost.NewHost(lxphost.Config{
		StoreKey: hostKey, NetworkID: 125, Epoch: 1,
		ParameterVersion: 1, TimestampWindowMs: 60_000, GasLimit: 10_000_000,
	})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	return &harness{
		ctx: ctx, host: host, storeKey: key, hostKey: hostKey, authority: authority,
		keeper: ledgerkeeper.NewKeeper(app.AppCodec(), key, host, app.BankKeeper, &app.EvmKeeper, authority),
	}
}
