package precompiles_test

import (
	"testing"

	layerxagenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeServes0x0901(t *testing.T) {
	address := common.HexToAddress("0x0901")
	require.Contains(t, precompiles.MergeAddressTable, precompiles.MergeAddress{
		Address: address,
		Name:    "orob",
	})
	for _, missing := range precompiles.MissingMergeConstructors() {
		require.NotEqual(t, address, missing.Address, "orob constructor must be registered by the node")
	}

	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	k := &a.EvmKeeper
	const planHeight = int64(20)
	ctx = ctx.WithBlockHeight(planHeight)
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))
	upgradeStore := ctx.KVStore(a.GetKey(upgradetypes.StoreKey))
	upgradeStore.Delete(append([]byte{upgradetypes.VersionMapByte}, layerxagenttypes.ModuleName...))
	require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), layerxagenttypes.ModuleName)
	for _, tracing := range []bool{false, true} {
		require.NotContains(t, k.CustomPrecompiles(ctx.WithIsTracing(tracing)), address,
			"the node activation list must withhold orob before the merge or agent genesis")
	}

	a.UpgradeKeeper.SetDone(ctx, precompiles.MergeUpgrade)
	require.Equal(t, planHeight, a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))
	for _, mode := range []struct {
		name    string
		tracing bool
	}{
		{name: "ordinary"},
		{name: "tracing", tracing: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, height := range []int64{1, planHeight - 1, planHeight, planHeight + 1, planHeight - 1} {
				served := k.CustomPrecompiles(ctx.WithBlockHeight(height).WithIsTracing(mode.tracing))
				if height < planHeight {
					require.NotContains(t, served, address, "height %d", height)
					continue
				}
				require.Contains(t, served, address, "height %d", height)
				p, ok := served[address].(precompiles.IPrecompile)
				require.True(t, ok, "orob must expose the precompile interface at height %d", height)
				require.Equal(t, "orob", p.GetName(), "height %d", height)
				require.Equal(t, address, p.Address(), "height %d", height)
			}
		})
	}
}
