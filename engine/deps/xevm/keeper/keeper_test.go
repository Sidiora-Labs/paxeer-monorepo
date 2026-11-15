package keeper_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"sync"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/rand"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/testutil/keeper"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/testutil/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/config"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types/ethtx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/gaskv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/mem"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestPurgePrefixNotHang(t *testing.T) {
	k, ctx := keeper.MockEVMKeeper(t)
	_, evmAddr := keeper.MockAddressPair()
	for i := 0; i < 50; i++ {
		ctx = ctx.WithMultiStore(ctx.MultiStore().CacheMultiStore())
		store := k.PrefixStore(ctx, types.StateKey(evmAddr))
		store.Set([]byte{0x03}, []byte("test"))
	}
	require.NotPanics(t, func() { k.PurgePrefix(ctx, types.StateKey(evmAddr)) })
}

func purgePrefixRawStore(k *evmkeeper.Keeper, ctx sdk.Context) sdk.KVStore {
	if k.UseRegularStore {
		return ctx.MultiStore().GetKVStore(k.GetStoreKey())
	}
	return ctx.GigaKVStore(k.GetStoreKey())
}

func TestPurgePrefixStackedCaches(t *testing.T) {
	for _, regular := range []bool{false, true} {
		t.Run(fmt.Sprintf("regular=%t", regular), func(t *testing.T) {
			k, ctx := keeper.MockEVMKeeper(t)
			k.UseRegularStore = regular
			pref := types.StateKey(common.Address{1})
			other := types.StateKey(common.Address{2})
			parent := ctx
			k.PrefixStore(ctx, pref).Set([]byte{0}, []byte("parent"))
			k.PrefixStore(ctx, other).Set([]byte{0}, []byte("retained"))
			keys := [][]byte{{0}}
			for i := 0; i < 200; i++ {
				ctx = ctx.WithMultiStore(ctx.MultiStore().CacheMultiStore())
				if i%25 == 0 || i == 199 {
					key := []byte{1, byte(i)}
					keys = append(keys, key)
					k.PrefixStore(ctx, pref).Set(key, []byte("layer"))
					k.PrefixStore(ctx, other).Set(key, []byte("retained"))
				}
				k.PrefixStore(ctx, pref).Set([]byte{2}, []byte{byte(i)})
			}
			keys = append(keys, []byte{2})
			k.PurgePrefix(ctx, pref)
			for _, key := range keys {
				require.Nil(t, k.PrefixStore(ctx, pref).Get(key))
			}
			require.Empty(t, purgePrefixRawStore(k, ctx).GetAllKeyStrsInRange(pref, sdk.PrefixEndBytes(pref)))
			for _, key := range keys[:len(keys)-1] {
				require.Equal(t, []byte("retained"), k.PrefixStore(ctx, other).Get(key))
			}
			require.Equal(t, []byte("parent"), k.PrefixStore(parent, pref).Get([]byte{0}))
		})
	}
}

func TestPurgePrefixDeletedKey(t *testing.T) {
	for _, regular := range []bool{false, true} {
		t.Run(fmt.Sprintf("regular=%t", regular), func(t *testing.T) {
			k, ctx := keeper.MockEVMKeeper(t)
			k.UseRegularStore = regular
			pref := types.StateKey(common.Address{1})
			k.PrefixStore(ctx, pref).Set([]byte{0}, []byte("deleted"))
			k.PrefixStore(ctx, pref).Set([]byte{1}, []byte("present"))
			parent := ctx
			ctx = ctx.WithMultiStore(ctx.MultiStore().CacheMultiStore())
			k.PrefixStore(ctx, pref).Delete([]byte{0})
			require.Equal(t, []string{string(append(append([]byte{}, pref...), 1))},
				purgePrefixRawStore(k, ctx).GetAllKeyStrsInRange(pref, sdk.PrefixEndBytes(pref)))
			meter := sdk.NewInfiniteGasMeter(1, 1)
			k.PurgePrefix(ctx.WithGasMeter(meter), pref)
			config := storetypes.KVGasConfig()
			want := 2*config.IterNextCostFlat + 2*config.ReadCostPerByte*uint64(len(pref)+1+len("present")) + config.DeleteCost
			require.Equal(t, want, meter.GasConsumed())
			require.Empty(t, purgePrefixRawStore(k, ctx).GetAllKeyStrsInRange(pref, sdk.PrefixEndBytes(pref)))
			require.Equal(t, []byte("deleted"), k.PrefixStore(parent, pref).Get([]byte{0}))
		})
	}
}

func TestPurgePrefixGas(t *testing.T) {
	for _, regular := range []bool{false, true} {
		for _, count := range []int{0, 10} {
			t.Run(fmt.Sprintf("regular=%t/keys=%d", regular, count), func(t *testing.T) {
				a, ctx := keeper.MockApp(t)
				k := &a.GigaEvmKeeper
				k.UseRegularStore = regular
				pref := types.StateKey(common.Address{1})
				reference := mem.NewStore()
				ordinary := ctx.WithMultiStore(ctx.MultiStore().CacheMultiStore())
				for i := 0; i < count; i++ {
					key, value := []byte{byte(i)}, bytes.Repeat([]byte{byte(i + 1)}, i+1)
					k.PrefixStore(ctx, pref).Set(key, value)
					a.EvmKeeper.PrefixStore(ordinary, pref).Set(key, value)
					reference.Set(append(append([]byte{}, pref...), key...), value)
				}
				meter := sdk.NewInfiniteGasMeter(1, 1)
				store := gaskv.NewStore(reference, meter, storetypes.KVGasConfig(), k.GetStoreKey().Name(), nil)
				iterator := store.Iterator(pref, sdk.PrefixEndBytes(pref))
				var keys [][]byte
				for ; iterator.Valid(); iterator.Next() {
					keys = append(keys, bytes.Clone(iterator.Key()))
				}
				require.NoError(t, iterator.Close())
				for _, key := range keys {
					store.Delete(key)
				}
				want := meter.GasConsumed()
				require.Positive(t, want)
				meter = sdk.NewInfiniteGasMeter(1, 1)
				k.PurgePrefix(ctx.WithGasMeter(meter), pref)
				require.Equal(t, want, meter.GasConsumed())
				meter = sdk.NewInfiniteGasMeter(1, 1)
				a.EvmKeeper.PurgePrefix(ordinary.WithGasMeter(meter), pref)
				require.Equal(t, want, meter.GasConsumed())
			})
		}
	}
}

func TestPurgePrefixTracing(t *testing.T) {
	k, ctx := keeper.MockEVMKeeper(t)
	k.UseRegularStore = false
	pref := types.StateKey(common.Address{1})
	k.PrefixStore(ctx, pref).Set([]byte{0}, []byte("fast"))
	ctx.KVStore(k.GetStoreKey()).Set(append(append([]byte{}, pref...), 0), []byte("current"))
	next := ctx.MultiStore().CacheMultiStore()
	nextCtx := ctx.WithMultiStore(next)
	k.PrefixStore(nextCtx.WithIsTracing(true), pref).Set([]byte{0}, []byte("next"))
	trace := ctx.WithIsTracing(true).WithNextMs(next, []string{k.GetStoreKey().Name()})
	require.Equal(t, []byte("next"), k.PrefixStore(trace, pref).Get([]byte{0}))
	k.PurgePrefix(trace, pref)
	require.Nil(t, k.PrefixStore(trace, pref).Get([]byte{0}))
	require.Equal(t, []byte("current"), k.PrefixStore(ctx.WithIsTracing(true), pref).Get([]byte{0}))
	require.Equal(t, []byte("fast"), k.PrefixStore(ctx, pref).Get([]byte{0}))
}

func TestGetChainID(t *testing.T) {
	k, ctx := keeper.MockEVMKeeper(t)
	require.Equal(t, config.DefaultChainID, k.ChainID(ctx).Int64())

	ctx = ctx.WithChainID("pacific-1")
	require.Equal(t, int64(1329), k.ChainID(ctx).Int64())

	ctx = ctx.WithChainID("atlantic-2")
	require.Equal(t, int64(1328), k.ChainID(ctx).Int64())

	ctx = ctx.WithChainID("arctic-1")
	require.Equal(t, int64(713715), k.ChainID(ctx).Int64())
}

func TestGetVMBlockContext(t *testing.T) {
	k, ctx := keeper.MockEVMKeeper(t)
	moduleAddr := k.AccountKeeper().GetModuleAddress(authtypes.FeeCollectorName)
	evmAddr, _ := k.GetEVMAddress(ctx, moduleAddr)
	k.DeleteAddressMapping(ctx, moduleAddr, evmAddr)
	_, err := k.GetVMBlockContext(ctx, 0)
	require.NotNil(t, err)
}

func TestGetHashFn(t *testing.T) {
	k, ctx := keeper.MockEVMKeeper(t)
	f := k.GetHashFn(ctx)
	require.Equal(t, common.Hash{}, f(math.MaxInt64+1))
	require.Equal(t, common.BytesToHash(ctx.HeaderHash()), f(uint64(ctx.BlockHeight())))
	require.Equal(t, common.Hash{}, f(uint64(ctx.BlockHeight())+1))
	require.Equal(t, common.Hash{}, f(uint64(ctx.BlockHeight())-1))
}

func TestKeeper_CalculateNextNonce(t *testing.T) {
	address1 := common.BytesToAddress([]byte("addr1"))
	key1 := tmtypes.TxHash(rand.NewRand().Bytes(32))
	key2 := tmtypes.TxHash(rand.NewRand().Bytes(32))
	tests := []struct {
		name          string
		address       common.Address
		pending       bool
		setup         func(ctx sdk.Context, k *evmkeeper.Keeper)
		expectedNonce uint64
	}{
		{
			name:          "latest block, no latest stored",
			address:       address1,
			pending:       false,
			expectedNonce: 0,
		},
		{
			name:    "latest block, latest stored",
			address: address1,
			pending: false,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				k.SetNonce(ctx, address1, 50)
			},
			expectedNonce: 50,
		},
		{
			name:    "latest block, latest stored with pending nonces",
			address: address1,
			pending: false,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				k.SetNonce(ctx, address1, 50)
				// because pending:false, these won't matter
				k.AddPendingNonce(key1, address1, 50, 0)
				k.AddPendingNonce(key2, address1, 51, 0)
			},
			expectedNonce: 50,
		},
		{
			name:    "pending block, nonce should follow the last pending",
			address: address1,
			pending: true,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				k.SetNonce(ctx, address1, 50)
				k.AddPendingNonce(key1, address1, 50, 0)
				k.AddPendingNonce(key2, address1, 51, 0)
			},
			expectedNonce: 52,
		},
		{
			name:    "pending block, nonce should be the value of hole",
			address: address1,
			pending: true,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				k.SetNonce(ctx, address1, 50)
				k.AddPendingNonce(key1, address1, 50, 0)
				// missing 51, so nonce = 51
				k.AddPendingNonce(key2, address1, 52, 0)
			},
			expectedNonce: 51,
		},
		{
			name:    "pending block, completed nonces should also be skipped",
			address: address1,
			pending: true,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				k.SetNonce(ctx, address1, 50)
				k.AddPendingNonce(key1, address1, 50, 0)
				k.AddPendingNonce(key2, address1, 51, 0)
				k.SetNonce(ctx, address1, 52)
				k.RemovePendingNonce(key1)
				k.RemovePendingNonce(key2)
			},
			expectedNonce: 52,
		},
		{
			name:    "pending block, hole created by expiration",
			address: address1,
			pending: true,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				k.SetNonce(ctx, address1, 50)
				k.AddPendingNonce(key1, address1, 50, 0)
				k.AddPendingNonce(key2, address1, 51, 0)
				k.RemovePendingNonce(key1)
			},
			expectedNonce: 50,
		},
		{
			name:    "pending block, skipped nonces all in pending",
			address: address1,
			pending: true,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				// next expected for latest is 50, but 51,52 were sent
				k.SetNonce(ctx, address1, 50)
				k.AddPendingNonce(key1, address1, 51, 0)
				k.AddPendingNonce(key2, address1, 52, 0)
			},
			expectedNonce: 50,
		},
		{
			name:    "try 1000 nonces concurrently",
			address: address1,
			pending: true,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				// next expected for latest is 50, but 51,52 were sent
				k.SetNonce(ctx, address1, 50)
				wg := sync.WaitGroup{}
				for i := 50; i < 1000; i++ {
					wg.Add(1)
					go func(nonce int) {
						defer wg.Done()
						key := tmtypes.TxHash(rand.NewRand().Bytes(32))
						// call this just to exercise locks
						k.CalculateNextNonce(ctx, address1, true)
						k.AddPendingNonce(key, address1, uint64(nonce), 0)
					}(i)
				}
				wg.Wait()
			},
			expectedNonce: 1000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			k, ctx := keeper.MockEVMKeeper(t)
			if test.setup != nil {
				test.setup(ctx, k)
			}
			next := k.CalculateNextNonce(ctx, test.address, test.pending)
			require.Equal(t, test.expectedNonce, next)
		})
	}
}

func TestDeferredInfo(t *testing.T) {
	a := app.Setup(t, false, false, false)
	k := a.GigaEvmKeeper
	ctx := a.GetContextForDeliverTx([]byte{})
	ctx = ctx.WithTxIndex(1)
	k.AppendToEvmTxDeferredInfo(ctx, ethtypes.Bloom{1, 2, 3}, common.Hash{4, 5, 6}, sdk.NewInt(1))
	ctx = ctx.WithTxIndex(2)
	k.AppendToEvmTxDeferredInfo(ctx, ethtypes.Bloom{7, 8}, common.Hash{9, 0}, sdk.NewInt(1))
	k.SetTxResults([]*abci.ExecTxResult{{Code: 0}, {Code: 0}, {Code: 0}, {Code: 1, Log: "test error"}})
	msg := mockEVMTransactionMessage(t)
	k.SetMsgs([]*types.MsgEVMTransaction{nil, {}, {}, msg})
	infoList := k.GetAllEVMTxDeferredInfo(ctx)
	require.Equal(t, 3, len(infoList))
	require.Equal(t, uint32(1), infoList[0].TxIndex)
	require.Equal(t, ethtypes.Bloom{1, 2, 3}, ethtypes.BytesToBloom(infoList[0].TxBloom))
	require.Equal(t, common.Hash{4, 5, 6}, common.BytesToHash(infoList[0].TxHash))
	require.Equal(t, sdk.NewInt(1), infoList[0].Surplus)
	require.Equal(t, uint32(2), infoList[1].TxIndex)
	require.Equal(t, ethtypes.Bloom{7, 8}, ethtypes.BytesToBloom(infoList[1].TxBloom))
	require.Equal(t, common.Hash{9, 0}, common.BytesToHash(infoList[1].TxHash))
	require.Equal(t, sdk.NewInt(1), infoList[1].Surplus)
	require.Equal(t, uint32(3), infoList[2].TxIndex)
	require.Equal(t, ethtypes.Bloom{}, ethtypes.BytesToBloom(infoList[2].TxBloom))
	etx, _ := msg.AsTransaction()
	require.Equal(t, etx.Hash(), common.BytesToHash(infoList[2].TxHash))
	require.Equal(t, "test error", infoList[2].Error)
	// test clear tx deferred info
	a.SetDeliverStateToCommit()
	a.Commit(context.Background()) // commit would clear transient stores
	k.SetTxResults([]*abci.ExecTxResult{})
	k.SetMsgs([]*types.MsgEVMTransaction{})
	infoList = k.GetAllEVMTxDeferredInfo(ctx)
	require.Empty(t, len(infoList))
}

func TestAddPendingNonce(t *testing.T) {
	k, _ := keeper.MockEVMKeeper(t)
	k.AddPendingNonce(tmtypes.TxHash{1}, common.HexToAddress("123"), 1, 1)
	k.AddPendingNonce(tmtypes.TxHash{2}, common.HexToAddress("123"), 2, 1)
	k.AddPendingNonce(tmtypes.TxHash{3}, common.HexToAddress("123"), 2, 2) // should replace the one above
	pendingTxs := k.GetPendingTxs()[common.HexToAddress("123").Hex()]
	require.Equal(t, 2, len(pendingTxs))
	require.Equal(t, tmtypes.TxHash{1}, pendingTxs[0].Hash)
	require.Equal(t, uint64(1), pendingTxs[0].Nonce)
	require.Equal(t, int64(1), pendingTxs[0].Priority)
	require.Equal(t, tmtypes.TxHash{3}, pendingTxs[1].Hash)
	require.Equal(t, uint64(2), pendingTxs[1].Nonce)
	require.Equal(t, int64(2), pendingTxs[1].Priority)
	hashToNonce := k.GetHashesToNonces()
	require.Equal(t, common.HexToAddress("123"), hashToNonce[tmtypes.TxHash{1}].Address)
	require.Equal(t, uint64(1), hashToNonce[tmtypes.TxHash{1}].Nonce)
	require.Equal(t, common.HexToAddress("123"), hashToNonce[tmtypes.TxHash{3}].Address)
	require.Equal(t, uint64(2), hashToNonce[tmtypes.TxHash{3}].Nonce)
	require.NotContains(t, hashToNonce, tmtypes.TxHash{2})
}

func mockEVMTransactionMessage(t *testing.T) *types.MsgEVMTransaction {
	k, ctx := testkeeper.MockEVMKeeper(t)
	chainID := k.ChainID(ctx)
	chainCfg := types.DefaultChainConfig()
	ethCfg := chainCfg.EthereumConfig(chainID)
	blockNum := big.NewInt(ctx.BlockHeight())
	privKey := testkeeper.MockPrivateKey()
	testPrivHex := hex.EncodeToString(privKey.Bytes())
	key, _ := crypto.HexToECDSA(testPrivHex)
	to := new(common.Address)
	txData := ethtypes.DynamicFeeTx{
		Nonce:     1,
		GasFeeCap: big.NewInt(10000000000000),
		Gas:       1000,
		To:        to,
		Value:     big.NewInt(1000000000000000),
		Data:      []byte("abc"),
		ChainID:   chainID,
	}

	signer := ethtypes.MakeSigner(ethCfg, blockNum, uint64(ctx.BlockTime().Unix()))
	tx, err := ethtypes.SignTx(ethtypes.NewTx(&txData), signer, key)
	typedTx, err := ethtx.NewDynamicFeeTx(tx)
	msg, err := types.NewMsgEVMTransaction(typedTx)
	require.Nil(t, err)
	return msg
}

func TestGetBaseFeeBeforeV620(t *testing.T) {
	// Set up a test app and context
	testApp, testCtx := testkeeper.MockApp(t)
	testHeight := int64(1000)
	testCtx = testCtx.WithBlockHeight(testHeight)

	// Set chain ID to pacific-1
	testCtx = testCtx.WithChainID("pacific-1")

	// Set the v6.2.0 upgrade height to a value higher than our test height
	v620UpgradeHeight := int64(2000)
	testApp.UpgradeKeeper.SetDone(testCtx.WithBlockHeight(v620UpgradeHeight), "6.2.0")

	keeper := &testApp.GigaEvmKeeper

	// Before upgrade: should be nil
	baseFee := keeper.GetBaseFee(testCtx)
	require.Nil(t, baseFee, "Base fee should be nil for pacific-1 before v6.2.0 upgrade")

	// After upgrade: should not be nil
	ctxAfterUpgrade := testCtx.WithBlockHeight(2500)
	baseFeeAfter := keeper.GetBaseFee(ctxAfterUpgrade)
	require.NotNil(t, baseFeeAfter, "Base fee should not be nil for pacific-1 after v6.2.0 upgrade")

	// Non-pacific-1 chain: should not be nil
	ctxOtherChain := testCtx.WithChainID("test-chain")
	baseFeeOther := keeper.GetBaseFee(ctxOtherChain)
	require.NotNil(t, baseFeeOther, "Base fee should not be nil for non-pacific-1 chains")
}
