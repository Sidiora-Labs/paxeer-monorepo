package precompiles_test

import (
	"testing"

	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeServes0x0908(t *testing.T) {
	address := common.HexToAddress("0x0908")
	require.Contains(t, precompiles.MergeAddressTable, precompiles.MergeAddress{Address: address, Name: "eip712"})
	for _, missing := range precompiles.MissingMergeConstructors() {
		require.NotEqual(t, address, missing.Address, "node must register the eip712 merge constructor")
	}

	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx([]byte{}).CacheContext()
	const planHeight int64 = 100
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))
	ctx.KVStore(a.GetKey(upgradetypes.StoreKey)).Delete(append([]byte{upgradetypes.VersionMapByte}, agenttypes.ModuleName...))
	require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), agenttypes.ModuleName)
	for _, tracing := range []bool{false, true} {
		before := ctx.WithBlockHeight(planHeight - 1).WithIsTracing(tracing)
		require.NotContains(t, a.EvmKeeper.CustomPrecompiles(before), address,
			"activation list must withhold eip712 before the merge is applied; tracing=%t", tracing)
	}

	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(planHeight), precompiles.MergeUpgrade)
	require.Equal(t, planHeight, a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))
	for _, mode := range []struct {
		name    string
		tracing bool
	}{
		{name: "execution"},
		{name: "tracing", tracing: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, height := range []int64{1, planHeight - 1} {
				below := ctx.WithBlockHeight(height).WithIsTracing(mode.tracing)
				require.NotContains(t, a.EvmKeeper.CustomPrecompiles(below), address, "height=%d", height)
			}
			for _, height := range []int64{planHeight, planHeight + 1} {
				at := ctx.WithBlockHeight(height).WithIsTracing(mode.tracing)
				served := a.EvmKeeper.CustomPrecompiles(at)
				require.Contains(t, served, address, "height=%d", height)
				contract, ok := served[address].(precompiles.IPrecompile)
				require.True(t, ok, "eip712 must expose its name and address at height=%d", height)
				require.Equal(t, address, contract.Address())
				require.Equal(t, "eip712", contract.GetName())
			}
		})
	}
}
