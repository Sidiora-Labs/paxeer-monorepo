package layerxledger_test

import (
	"crypto/ed25519"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
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

type keepers struct {
	*app.PrecompileKeepers
	ledger *ledgerkeeper.Keeper
}

func (k keepers) LayerXLedgerK() *ledgerkeeper.Keeper { return k.ledger }

type harness struct {
	ctx        sdk.Context
	keeper     *ledgerkeeper.Keeper
	keepers    keepers
	host       *lxphost.Host
	hostKey    *storetypes.KVStoreKey
	storeKey   *storetypes.KVStoreKey
	precompile *pcommon.Precompile
	stateDB    *state.DBImpl
	evm        *vm.EVM
	caller     common.Address
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
	ctx, _ := testApp.NewContext(false, tmtypes.Header{}).WithChainID("hyperpax_125-1").
		WithBlockHeight(11).WithBlockTime(time.Unix(1_800_000_000, 0).UTC()).CacheContext()
	h := &harness{hostKey: sdk.NewKVStoreKey("lxphost"), storeKey: sdk.NewKVStoreKey(types.StoreKey),
		caller: common.HexToAddress("0x2320000000000000000000000000000000000001")}
	ctx = withStore(withStore(ctx, h.storeKey), h.hostKey)
	var err error
	h.host, err = lxphost.NewHost(lxphost.Config{StoreKey: h.hostKey, NetworkID: 125, Epoch: 1,
		ParameterVersion: 1, TimestampWindowMs: 60_000, GasLimit: 10_000_000})
	require.NoError(t, err)
	t.Cleanup(h.host.Close)
	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	h.keeper = ledgerkeeper.NewKeeper(testApp.AppCodec(), h.storeKey, h.host, testApp.BankKeeper, &testApp.EvmKeeper, authority)
	h.keepers = keepers{PrecompileKeepers: testApp.GetPrecompileKeepers().(*app.PrecompileKeepers), ledger: h.keeper}
	h.precompile, err = layerxledger.NewPrecompile(h.keepers)
	require.NoError(t, err)
	db := state.NewDBImpl(ctx.WithEventManager(sdk.NewEventManager()), &testApp.EvmKeeper, true)
	h.stateDB = db
	h.ctx = db.Ctx()
	h.evm = &vm.EVM{StateDB: db, TxContext: vm.TxContext{Origin: h.caller}}
	h.ctx.KVStore(h.hostKey).Set([]byte("refusal-sentinel"), []byte("unchanged"))
	return h
}

func (h *harness) bindCaller(t *testing.T) {
	t.Helper()
	testApp := testkeeper.EVMTestApp
	var seed [ed25519.SeedSize]byte
	seed[0] = 1
	private := ed25519.NewKeyFromSeed(seed[:])
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(testApp.EvmKeeper.ChainID(h.ctx), h.caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = testApp.EvmKeeper.BindLayerX(h.ctx, h.caller, key, signature)
	require.NoError(t, err)
	actor, err := h.keeper.Actor(h.ctx, h.caller)
	require.NoError(t, err)
	require.Equal(t, key, actor.DIDKey)
	require.True(t, actor.Authority.Bound)
}

func storeSnapshot(t *testing.T, store storetypes.KVStore) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	iter := store.Iterator(nil, nil)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		out[string(iter.Key())] = append([]byte(nil), iter.Value()...)
	}
	return out
}
