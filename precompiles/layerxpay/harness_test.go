package layerxpay

import (
	"crypto/ed25519"
	"math"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type payPrecompileKeepers struct {
	*app.PrecompileKeepers
	pay *paykeeper.Keeper
}

func (k payPrecompileKeepers) LayerXPayK() *paykeeper.Keeper { return k.pay }

type payPrecompileHarness struct {
	keeper     *paykeeper.Keeper
	keepers    payPrecompileKeepers
	precompile *pcommon.Precompile
	db         *state.DBImpl
	evm        *vm.EVM
	caller     common.Address
}

func newPayPrecompileHarness(t *testing.T) *payPrecompileHarness {
	t.Helper()
	application := testkeeper.EVMTestApp
	ctx, _ := application.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(100).WithBlockTime(time.UnixMilli(1700000000000)).WithEVMTxHash(common.HexToHash("0x2281"))
	payKey, hostKey := sdk.NewKVStoreKey(paytypes.StoreKey), sdk.NewKVStoreKey("lxphost")
	parent := ctx.MultiStore()
	stores := map[storetypes.StoreKey]storetypes.CacheWrapper{}
	keys := map[string]storetypes.StoreKey{}
	for _, key := range parent.StoreKeys() {
		// Some optional application stores are not mounted by the EVM test app.
		var kv storetypes.KVStore
		func() { defer func() { _ = recover() }(); kv = parent.GetKVStore(key) }()
		if kv != nil {
			stores[key] = kv
			keys[key.Name()] = key
		}
	}
	for _, key := range []*sdk.KVStoreKey{payKey, hostKey} {
		stores[key] = dbadapter.Store{DB: dbm.NewMemDB()}
		keys[key.Name()] = key
	}
	ctx = ctx.WithMultiStore(cachemulti.NewStore(dbm.NewMemDB(), stores, keys, nil, nil, nil, 0))
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: hostKey, NetworkID: 7, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	keeper, err := paykeeper.NewKeeper(application.AppCodec(), payKey, hostKey, host, &application.EvmKeeper, application.LayerXCustodyKeeper, "pay-governance")
	require.NoError(t, err)
	require.NoError(t, keeper.InitGenesis(ctx, *paytypes.DefaultGenesis()))
	for _, module := range []uint16{paytypes.EscrowModuleID, paytypes.BudgetModuleID, paytypes.StreamModuleID, paytypes.ServiceModuleID} {
		key, err := paytypes.KernelModuleEnableKey(module)
		require.NoError(t, err)
		var enabled [32]byte
		enabled[31] = 1
		ctx.KVStore(hostKey).Set(key, enabled[:])
	}
	p, err := NewPrecompile(&keeper)
	require.NoError(t, err)
	h := &payPrecompileHarness{keeper: &keeper, precompile: p, caller: common.HexToAddress("0xa2281")}
	h.keepers = payPrecompileKeepers{application.GetPrecompileKeepers().(*app.PrecompileKeepers), &keeper}
	// Bind through the same keeper used by the 0x1004 precompile.
	seed := [32]byte{0x28, 0x01}
	private := ed25519.NewKeyFromSeed(seed[:])
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(application.EvmKeeper.ChainID(ctx), h.caller, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = application.EvmKeeper.BindLayerX(ctx, h.caller, public, signature)
	require.NoError(t, err)
	bound, found := application.EvmKeeper.GetLayerXDid(ctx, h.caller)
	require.True(t, found)
	require.Equal(t, public, bound)
	authority, err := keeper.AuthorityFor(ctx, h.caller)
	require.NoError(t, err)
	require.Equal(t, public, authority.AuthorityKey)
	h.db = state.NewDBImpl(ctx.WithEventManager(sdk.NewEventManager()), &application.EvmKeeper, false)
	block, err := application.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(application.EvmKeeper.ChainID(ctx))
	h.evm = vm.NewEVM(*block, h.db, chain, vm.Config{}, map[common.Address]vm.PrecompiledContract{p.Address(): p})
	return h
}
