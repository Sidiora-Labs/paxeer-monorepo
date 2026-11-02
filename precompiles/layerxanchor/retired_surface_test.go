package layerxanchor_test

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"os"
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestEveryAnchorSelectorRetired(t *testing.T) {
	source, err := os.ReadFile("abi.json")
	require.NoError(t, err)
	surface, err := abi.JSON(bytes.NewReader(source))
	require.NoError(t, err)
	require.Len(t, surface.Methods, 21)
	require.Len(t, surface.Events, 12)

	testApp := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, testApp.Close()) })
	require.NotNil(t, testApp.GetPrecompileKeepers().AnchorK())
	require.Same(t, &testApp.LayerXAnchorKeeper, testApp.GetPrecompileKeepers().AnchorK())
	const mergeHeight, deleteHeight = int64(20), int64(40)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(deleteHeight)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	testApp.UpgradeKeeper.SetDone(ctx.WithBlockHeight(mergeHeight), pcommon.MergeUpgrade)
	testApp.UpgradeKeeper.SetDone(ctx, anchortypes.StoreDeleteUpgrade)
	require.Equal(t, deleteHeight, testApp.UpgradeKeeper.GetDoneHeight(ctx, anchortypes.StoreDeleteUpgrade))
	views := map[string][]interface{}{
		layerxanchor.LatestFinalizedMethod:      {uint64(0), false},
		layerxanchor.CheckpointMethod:           {layerxanchor.CheckpointView{}},
		layerxanchor.FinalizedStateRootMethod:   {[32]byte{}, false},
		layerxanchor.FinalizedReceiptRootMethod: {[32]byte{}, false},
		layerxanchor.GuarantorMethod:            {layerxanchor.GuarantorView{Bond: new(big.Int), Unbonding: new(big.Int)}},
		layerxanchor.ThresholdMethod:            {testApp.LayerXAnchorKeeper.GetParams(ctx).Threshold},
		layerxanchor.StatusOfMethod:             {anchortypes.CheckpointUnknown},
		layerxanchor.CheckpointBatchMethod:      {uint64(0), anchortypes.CheckpointUnknown},
		layerxanchor.CheckpointGuarantorsMethod: {[][32]byte{}},
	}
	require.Len(t, views, 9)
	stores := make(map[storetypes.StoreKey]storetypes.CacheWrapper)
	keys := make(map[string]storetypes.StoreKey)
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() != anchortypes.StoreKey {
			stores[key] = ctx.MultiStore().GetKVStore(key)
			keys[key.Name()] = key
		}
	}
	cacheDB := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, cacheDB.Close()) })
	deleted := ctx.WithMultiStore(cachemulti.NewStore(cacheDB, stores, keys, nil, nil, nil, 0))
	for _, key := range deleted.MultiStore().StoreKeys() {
		require.NotEqual(t, anchortypes.StoreKey, key.Name(), "retired anchor store must be absent")
	}
	address := common.HexToAddress("0x1014")
	require.Equal(t, common.HexToAddress(layerxanchor.LayerXAnchorAddress), address)
	caller := common.HexToAddress("0x1234")
	want := pcommon.RetiredRevert(layerxanchor.PrecompileName)

	for _, height := range []int64{mergeHeight, mergeHeight + 1, deleteHeight, deleteHeight + 1} {
		ctx := ctx.WithBlockHeight(height)
		if height >= deleteHeight {
			ctx = deleted.WithBlockHeight(height)
		}
		anchorMounted := false
		for _, key := range ctx.MultiStore().StoreKeys() {
			anchorMounted = anchorMounted || key.Name() == anchortypes.StoreKey
		}
		require.Equal(t, height < deleteHeight, anchorMounted)
		registered := testApp.EvmKeeper.CustomPrecompiles(ctx)
		require.Contains(t, registered, address, "retired address must remain registered at height %d", height)
		require.NotNil(t, registered[address])
		blockCtx, err := testApp.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
		require.NoError(t, err)
		cfg := evmtypes.DefaultChainConfig().EthereumConfig(testApp.EvmKeeper.ChainID(ctx))
		for name, method := range surface.Methods {
			args := make([]interface{}, len(method.Inputs))
			for i, input := range method.Inputs {
				args[i] = reflect.Zero(input.Type.GetType()).Interface()
				if input.Type.T == abi.UintTy && input.Type.Size > 64 {
					args[i] = new(big.Int)
				}
			}
			packed, err := method.Inputs.Pack(args...)
			require.NoError(t, err, name)
			input := append(append([]byte(nil), method.ID...), packed...)
			expectView := height < deleteHeight && method.StateMutability == "view"
			for _, static := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/%s/static=%t", height, name, static), func(t *testing.T) {
					db := state.NewDBImpl(ctx, &testApp.EvmKeeper, false)
					evm := vm.NewEVM(*blockCtx, db, cfg, vm.Config{}, registered)
					const gas = uint64(1_000_000)
					var out []byte
					var left uint64
					var callErr error
					if static {
						out, left, callErr = evm.StaticCall(caller, address, input, gas)
					} else {
						out, left, callErr = evm.Call(caller, address, input, gas, uint256.NewInt(0))
					}
					var failure string
					status := uint32(ethtypes.ReceiptStatusFailed)
					if expectView {
						require.NoError(t, callErr)
						values, ok := views[name]
						require.True(t, ok, "every ABI view must be exercised")
						expected, err := method.Outputs.Pack(values...)
						require.NoError(t, err)
						require.Equal(t, expected, out)
						status = uint32(ethtypes.ReceiptStatusSuccessful)
					} else {
						require.ErrorIs(t, callErr, vm.ErrExecutionReverted)
						failure = callErr.Error()
						require.Equal(t, want, out)
						reason, err := abi.UnpackRevert(out)
						require.NoError(t, err)
						require.Equal(t, "LXP_MODULE_DISABLED: module disabled; layerxAnchor retired", reason)
					}

					txHash := crypto.Keccak256Hash([]byte(t.Name()), input)
					receipt, err := testApp.EvmKeeper.WriteReceipt(ctx, db, &core.Message{
						From: caller, To: &address, GasPrice: new(big.Int), Data: input,
					}, ethtypes.LegacyTxType, txHash, gas-left, failure)
					require.NoError(t, err)
					require.Equal(t, status, receipt.Status)
					for eventName, event := range surface.Events {
						count := 0
						for _, log := range receipt.Logs {
							if common.HexToAddress(log.Address) == address && len(log.Topics) > 0 && common.HexToHash(log.Topics[0]) == event.ID {
								count++
							}
						}
						require.Zero(t, count, "retired event %s", eventName)
					}
					require.Empty(t, receipt.Logs)
					require.Empty(t, db.GetAllLogs())
				})
			}
		}
	}
}
