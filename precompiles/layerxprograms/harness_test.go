package layerxprograms_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type harness struct {
	t              *testing.T
	key            *sdk.KVStoreKey
	host           *lxphost.Host
	executor       *lp.PrecompileExecutor
	stateDB        *state.DBImpl
	evm            *vm.EVM
	precompile     *pcommon.Precompile
	caller         common.Address
	publicKey      [32]byte
	occupancyAsset [32]byte
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(100).WithBlockTime(time.UnixMilli(1700000000123)).
		WithEVMTxHash(common.HexToHash("0x123456789abcdef"))
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)).WithEventManager(sdk.NewEventManager())
	key := sdk.NewKVStoreKey("programs_submit_host")
	ctx = xwebtestutil.WithXWebStore(ctx, key)
	h := &harness{t: t, key: key, caller: common.HexToAddress("0xa111")}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, ed25519.SeedSize))
	copy(h.publicKey[:], private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(app.EvmKeeper.ChainID(ctx), h.caller, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = app.EvmKeeper.BindLayerX(ctx, h.caller, h.publicKey, signature)
	require.NoError(t, err)
	authority, _, err := lp.PrincipalOf(ctx, &app.EvmKeeper, h.caller)
	require.NoError(t, err)
	h.occupancyAsset = [32]byte{0x91}
	fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		fee = binary.BigEndian.AppendUint64(fee, 1)
	}
	fee = append(fee, h.occupancyAsset[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		fee = binary.BigEndian.AppendUint64(fee, value)
	}
	fee = append(fee, authority.Actor[:]...)
	fee = append(fee, make([]byte, 16)...)
	store := ctx.KVStore(key)
	store.Set(append([]byte{1, 0, 9}, []byte("progfee/active/v1")...), fee)
	store.Set(append([]byte{1, 0, 9}, binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)...), fee)
	h.host = h.newHost(1000000)
	h.executor = lp.NewTestExecutor(h.host, &app.EvmKeeper)
	h.precompile = pcommon.NewPrecompile(lp.ProgramsABI, h.executor, common.HexToAddress("0x101d"), "layerxprograms").
		WithLXPReasons().WithGasTable(lp.GasTable)
	h.stateDB = state.NewDBImpl(ctx, &app.EvmKeeper, false)
	block, err := app.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(app.EvmKeeper.ChainID(ctx))
	h.evm = vm.NewEVM(*block, h.stateDB, chain, vm.Config{}, map[common.Address]vm.PrecompiledContract{
		h.precompile.Address(): h.precompile,
	})
	return h
}

func (h *harness) newHost(gasLimit uint64) *lxphost.Host {
	h.t.Helper()
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: h.key, NetworkID: 7,
		ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: gasLimit})
	require.NoError(h.t, err)
	h.t.Cleanup(host.Close)
	return host
}

func (h *harness) dump(ctx sdk.Context) map[string][]byte {
	h.t.Helper()
	values := map[string][]byte{}
	iterator := ctx.KVStore(h.key).Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		values[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	return values
}
