package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestPurgePrefixPreservesScanGasAndOutOfGas(t *testing.T) {
	k, ctx := keeper.MockEVMKeeper(t)
	_, address := keeper.MockAddressPair()
	pref := types.StateKey(address)
	for _, count := range []int{0, 1, 3} {
		parent := ctx.WithMultiStore(ctx.MultiStore().CacheMultiStore())
		for i := 0; i < count; i++ {
			k.PrefixStore(parent, pref).Set([]byte{byte(i)}, []byte("value"))
		}
		standard := parent.WithMultiStore(parent.MultiStore().CacheMultiStore()).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
		require.NoError(t, k.PrefixStore(standard, pref).DeleteAll(nil, nil))
		wantGas := standard.GasMeter().GasConsumed()
		for _, tracing := range []bool{false, true} {
			actual := parent.WithMultiStore(parent.MultiStore().CacheMultiStore()).WithIsTracing(tracing).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
			k.PurgePrefix(actual, pref)
			require.Equal(t, wantGas, actual.GasMeter().GasConsumed(), "count %d, tracing %t", count, tracing)
			for i := 0; i < count; i++ {
				require.Nil(t, k.PrefixStore(actual, pref).Get([]byte{byte(i)}))
				require.Equal(t, []byte("value"), k.PrefixStore(parent, pref).Get([]byte{byte(i)}))
			}
		}
		limited := parent.WithMultiStore(parent.MultiStore().CacheMultiStore()).WithGasMeter(sdk.NewGasMeter(0, 1, 1))
		require.Panics(t, func() { k.PurgePrefix(limited, pref) })
		for i := 0; i < count; i++ {
			require.Equal(t, []byte("value"), k.PrefixStore(limited.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)), pref).Get([]byte{byte(i)}))
		}
	}
}

func TestPurgePrefixTracingUsesTheSelectedHeightStore(t *testing.T) {
	k, ctx := keeper.MockEVMKeeper(t)
	_, address := keeper.MockAddressPair()
	pref := types.StateKey(address)
	next := ctx.MultiStore().CacheMultiStore()
	nextCtx := ctx.WithMultiStore(next)
	k.PrefixStore(ctx, pref).Set([]byte{1}, []byte("historical"))
	k.PrefixStore(nextCtx, pref).Set([]byte{1}, []byte("next height"))
	tracing := ctx.WithIsTracing(true).WithNextMs(next, []string{k.GetStoreKey().Name()})
	k.PurgePrefix(tracing, pref)
	require.Nil(t, k.PrefixStore(tracing, pref).Get([]byte{1}))
	require.Equal(t, []byte("historical"), k.PrefixStore(ctx, pref).Get([]byte{1}))
}
