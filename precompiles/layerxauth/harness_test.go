package layerxauth_test

import (
	"crypto/ed25519"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

type authKeepers struct {
	*app.PrecompileKeepers
	auth *authkeeper.Keeper
}

func (k authKeepers) LayerXAuthK() *authkeeper.Keeper { return k.auth }

type harness struct {
	ctx        sdk.Context
	hostKey    *sdk.KVStoreKey
	keeper     *authkeeper.Keeper
	keepers    authKeepers
	precompile *pcommon.Precompile
	caller     common.Address
	unbound    common.Address
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
	testApp := testkeeper.EVMTestApp
	base, _ := testApp.NewContext(false, tmtypes.Header{}).WithChainID("hyperpax_125-1").
		WithBlockHeight(11).WithBlockTime(time.Unix(1_800_000_000, 0).UTC()).CacheContext()
	key := sdk.NewKVStoreKey(types.StoreKey)
	hostKey := sdk.NewKVStoreKey("lxphost")
	ctx := withStore(withStore(base, key), hostKey)
	host, err := lxphost.NewHost(lxphost.Config{
		StoreKey: hostKey, NetworkID: 7, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000,
	})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	k := authkeeper.NewKeeper(testApp.AppCodec(), key, host, &testApp.AuthzKeeper, &testApp.EvmKeeper, authority)
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))
	h := &harness{
		ctx: ctx, hostKey: hostKey, keeper: k,
		keepers: authKeepers{PrecompileKeepers: testApp.GetPrecompileKeepers().(*app.PrecompileKeepers), auth: k},
		caller:  common.Address{0x21, 0x11, 0x01}, unbound: common.Address{0x21, 0x11, 0x02},
	}
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	var public [ed25519.PublicKeySize]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(testApp.EvmKeeper.ChainID(ctx), h.caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = testApp.EvmKeeper.BindLayerX(ctx, h.caller, public, signature)
	require.NoError(t, err)
	_, err = k.Actor(ctx, h.caller)
	require.NoError(t, err)
	_, bound := testApp.EvmKeeper.GetLayerXDid(ctx, h.unbound)
	require.False(t, bound)
	ctx.KVStore(hostKey).Set([]byte("refusal-sentinel"), []byte("unchanged"))
	h.precompile, err = layerxauth.NewPrecompile(h.keepers)
	require.NoError(t, err)
	return h
}

func (h *harness) evm() (*vm.EVM, *state.DBImpl) {
	db := state.NewDBImpl(h.ctx.WithEventManager(sdk.NewEventManager()), &testkeeper.EVMTestApp.EvmKeeper, true)
	return &vm.EVM{StateDB: db, TxContext: vm.TxContext{Origin: h.caller}}, db
}

func hostSnapshot(t *testing.T, ctx sdk.Context, key sdk.StoreKey) map[string][]byte {
	t.Helper()
	rows := map[string][]byte{}
	iterator := ctx.KVStore(key).Iterator(nil, nil)
	for ; iterator.Valid(); iterator.Next() {
		rows[string(iterator.Key())] = append([]byte(nil), iterator.Value()...)
	}
	require.NoError(t, iterator.Close())
	return rows
}
