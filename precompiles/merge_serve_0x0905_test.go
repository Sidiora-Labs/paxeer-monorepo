package precompiles_test

import (
	"fmt"
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeServes0x0905(t *testing.T) {
	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	const planHeight int64 = 100
	ctx = ctx.WithBlockHeight(planHeight)
	address := common.HexToAddress("0x0905")
	checks := schedulerMergeAssertions{}

	require.Equal(t, app.MergeUpgrade, precompiles.MergeUpgrade)
	require.Contains(t, precompiles.MergeAddressTable, precompiles.MergeAddress{
		Address: address,
		Name:    "scheduler",
	})
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))

	t.Run("constructor registered by node", func(t *testing.T) {
		checks.RegisterMergeConstructor(t, address)
	})

	t.Run("activation requires agent module before plan", func(t *testing.T) {
		before, _ := ctx.CacheContext()
		before.KVStore(a.GetKey(upgradetypes.StoreKey)).Delete(
			append([]byte{upgradetypes.VersionMapByte}, agenttypes.ModuleName...))
		require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(before), agenttypes.ModuleName)
		for _, tracing := range []bool{false, true} {
			at := before.WithIsTracing(tracing)
			require.NotContains(t, a.EvmKeeper.CustomPrecompiles(at), address)
			require.NotContains(t, a.EvmKeeper.GetCustomPrecompilesVersions(at), address)
		}
	})

	a.UpgradeKeeper.SetDone(ctx, precompiles.MergeUpgrade)
	require.Equal(t, planHeight, a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))
	for _, tracing := range []bool{false, true} {
		for _, height := range []int64{planHeight - 1, planHeight, planHeight + 1} {
			t.Run(fmt.Sprintf("height=%d/tracing=%t", height, tracing), func(t *testing.T) {
				at := ctx.WithBlockHeight(height).WithIsTracing(tracing)
				checks.CustomPrecompiles(t, &a.EvmKeeper, at, address, planHeight)
			})
		}
	}
}

type schedulerMergeAssertions struct{}

func (checks schedulerMergeAssertions) RegisterMergeConstructor(t *testing.T, address common.Address) {
	t.Helper()
	for _, missing := range checks.MissingMergeConstructors() {
		require.NotEqual(t, address, missing.Address, "node must register the scheduler merge constructor")
	}
}

func (schedulerMergeAssertions) MissingMergeConstructors() []precompiles.MergeAddress {
	return precompiles.MissingMergeConstructors()
}

func (schedulerMergeAssertions) CustomPrecompiles(t *testing.T, k *evmkeeper.Keeper, ctx sdk.Context, address common.Address, planHeight int64) {
	t.Helper()
	served := k.CustomPrecompiles(ctx)
	versions := k.GetCustomPrecompilesVersions(ctx)
	if ctx.BlockHeight() < planHeight {
		require.NotContains(t, served, address)
		require.NotContains(t, versions, address)
		return
	}
	require.Contains(t, served, address)
	contract, ok := served[address].(precompiles.IPrecompile)
	require.True(t, ok, "served scheduler must expose the merge precompile interface")
	require.Equal(t, address, contract.Address())
	require.Equal(t, "scheduler", contract.GetName())
	require.Equal(t, precompiles.MergeUpgrade, versions[address])
}
