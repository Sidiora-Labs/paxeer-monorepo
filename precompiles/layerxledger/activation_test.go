package layerxledger_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestLedgerServedAfterMergeActivation(t *testing.T) {
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(100).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	address := common.HexToAddress("0x101A")
	require.Equal(t, common.HexToAddress(layerxledger.LedgerAddress), address)
	require.Equal(t, precompiles.MergeUpgrade, app.MergeUpgrade)
	key := a.GetKey(types.StoreKey)
	require.NotNil(t, key)
	require.True(t, ctx.KVStore(key).Has(types.ParamsKey), "ledger module must initialize its store")
	provider, ok := a.GetPrecompileKeepers().(layerxledger.LedgerKeepers)
	require.True(t, ok, "node must expose its ledger keeper to the merge constructor")
	require.NotNil(t, a.LayerXLedgerKeeper)
	require.Same(t, a.LayerXLedgerKeeper, provider.LayerXLedgerK())
	var tableEntry precompiles.MergeAddress
	for _, entry := range precompiles.MergeAddressTable {
		if entry.Address == address {
			tableEntry = entry
		}
	}
	require.Equal(t, address, tableEntry.Address)
	require.Equal(t, "layerxledger", tableEntry.Name)
	a.UpgradeKeeper.SetDone(ctx, precompiles.MergeUpgrade)
	require.Equal(t, int64(100), a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))
	for _, tracing := range []bool{false, true} {
		for _, height := range []int64{100, 101} {
			t.Run(fmt.Sprintf("height=%d/tracing=%t", height, tracing), func(t *testing.T) {
				at := ctx.WithBlockHeight(height).WithIsTracing(tracing)
				served := a.EvmKeeper.CustomPrecompiles(at)
				require.Contains(t, served, address)
				contract, ok := served[address].(precompiles.IPrecompile)
				require.True(t, ok, "served ledger must be the real merge precompile")
				require.Equal(t, tableEntry.Address, contract.Address())
				require.Equal(t, tableEntry.Name, contract.GetName())
				versions := a.EvmKeeper.GetCustomPrecompilesVersions(at)
				require.Contains(t, versions, address)
				require.Equal(t, precompiles.MergeUpgrade, versions[address])
			})
		}
	}
}

func TestLedgerUnservedBeforeMergeActivation(t *testing.T) {
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(100)
	address := common.HexToAddress("0x101A")
	versions := a.UpgradeKeeper.GetModuleVersionMap(ctx)
	require.Contains(t, versions, types.ModuleName)
	ctx.KVStore(a.GetKey(upgradetypes.StoreKey)).Delete(
		append([]byte{upgradetypes.VersionMapByte}, types.ModuleName...))
	require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), types.ModuleName)
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))
	for _, tracing := range []bool{false, true} {
		before := ctx.WithBlockHeight(99).WithIsTracing(tracing)
		require.NotContains(t, a.EvmKeeper.CustomPrecompiles(before), address)
		require.NotContains(t, a.EvmKeeper.GetCustomPrecompilesVersions(before), address)
	}
	a.UpgradeKeeper.SetDone(ctx, precompiles.MergeUpgrade)
	a.UpgradeKeeper.SetModuleVersionMap(ctx, versions)
	require.Equal(t, int64(100), a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))
	require.Contains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), types.ModuleName)
	for _, tracing := range []bool{false, true} {
		for _, height := range []int64{1, 99} {
			t.Run(fmt.Sprintf("height=%d/tracing=%t", height, tracing), func(t *testing.T) {
				before := ctx.WithBlockHeight(height).WithIsTracing(tracing)
				require.NotContains(t, a.EvmKeeper.CustomPrecompiles(before), address)
				require.NotContains(t, a.EvmKeeper.GetCustomPrecompilesVersions(before), address)
			})
		}
		require.Contains(t, a.EvmKeeper.CustomPrecompiles(ctx.WithIsTracing(tracing)), address)
		require.Contains(t, a.EvmKeeper.GetCustomPrecompilesVersions(ctx.WithIsTracing(tracing)), address)
	}
}
