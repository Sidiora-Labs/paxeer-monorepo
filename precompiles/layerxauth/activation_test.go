package layerxauth_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func newAuthActivationWrapper(t *testing.T) *app.TestWrapper {
	t.Helper()
	wrapper := app.NewTestWrapper(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := wrapper.App, wrapper.Ctx
	key := a.GetKey(types.StoreKey)
	require.NotNil(t, key)
	require.NotNil(t, ctx.KVStore(key))
	require.NotNil(t, a.LayerXAuthKeeper)
	keepers, ok := a.GetPrecompileKeepers().(layerxauth.AuthKeepers)
	require.True(t, ok)
	require.Same(t, a.LayerXAuthKeeper, keepers.LayerXAuthK())
	require.Contains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), types.ModuleName)
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, app.MergeUpgrade))
	require.Equal(t, precompiles.MergeUpgrade, app.MergeUpgrade)
	return wrapper
}

func TestAuthServedAfterMergeActivation(t *testing.T) {
	wrapper := newAuthActivationWrapper(t)
	a, ctx := wrapper.App, wrapper.Ctx
	address := common.HexToAddress("0x000000000000000000000000000000000000101C")
	require.Equal(t, address, common.HexToAddress(layerxauth.AuthAddress))
	var name string
	for _, entry := range precompiles.MergeAddressTable {
		if entry.Address == address {
			name = entry.Name
		}
	}
	require.Equal(t, "layerxauth", name)
	built, err := layerxauth.NewMergePrecompile(a.GetPrecompileKeepers(), precompiles.MergeUpgrade)
	require.NoError(t, err)
	require.NotNil(t, built)
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(100), app.MergeUpgrade)
	require.Equal(t, int64(100), a.UpgradeKeeper.GetDoneHeight(ctx, app.MergeUpgrade))
	for _, tracing := range []bool{false, true} {
		for _, height := range []int64{100, 101} {
			t.Run(fmt.Sprintf("height=%d/tracing=%t", height, tracing), func(t *testing.T) {
				at := ctx.WithBlockHeight(height).WithIsTracing(tracing)
				served, found := a.EvmKeeper.CustomPrecompiles(at)[address]
				require.True(t, found)
				require.NotNil(t, served)
				require.IsType(t, built, served)
				contract, ok := served.(precompiles.IPrecompile)
				require.True(t, ok)
				require.Equal(t, address, contract.Address())
				require.Equal(t, name, contract.GetName())
				require.Equal(t, built.GetABI(), contract.GetABI())
				require.Equal(t, precompiles.MergeUpgrade, a.EvmKeeper.GetCustomPrecompilesVersions(at)[address])
			})
		}
	}
}

func TestAuthUnservedBeforeMergeActivation(t *testing.T) {
	wrapper := newAuthActivationWrapper(t)
	a, ctx := wrapper.App, wrapper.Ctx
	address := common.HexToAddress("0x000000000000000000000000000000000000101C")
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(100), app.MergeUpgrade)
	require.Equal(t, int64(100), a.UpgradeKeeper.GetDoneHeight(ctx, app.MergeUpgrade))
	for _, tracing := range []bool{false, true} {
		at := ctx.WithBlockHeight(100).WithIsTracing(tracing)
		require.Contains(t, a.EvmKeeper.CustomPrecompiles(at), address)
		require.Contains(t, a.EvmKeeper.GetCustomPrecompilesVersions(at), address)
		for _, height := range []int64{1, 99} {
			t.Run(fmt.Sprintf("height=%d/tracing=%t", height, tracing), func(t *testing.T) {
				before := ctx.WithBlockHeight(height).WithIsTracing(tracing)
				require.NotContains(t, a.EvmKeeper.CustomPrecompiles(before), address)
				require.NotContains(t, a.EvmKeeper.GetCustomPrecompilesVersions(before), address)
			})
		}
	}
}
