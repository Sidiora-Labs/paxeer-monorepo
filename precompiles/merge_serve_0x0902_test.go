package precompiles_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeServes0x0902(t *testing.T) {
	address := common.HexToAddress("0x0000000000000000000000000000000000000902")
	require.Contains(t, precompiles.MergeAddressTable, precompiles.MergeAddress{
		Address: address,
		Name:    "clearing",
	})
	for _, missing := range precompiles.MissingMergeConstructors() {
		require.NotEqual(t, address, missing.Address, "clearing constructor must be registered by the node")
	}

	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx([]byte{}).CacheContext()
	const planHeight int64 = 100
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
				require.NotContains(t, a.EvmKeeper.CustomPrecompiles(below), address, "height %d", height)
			}
			for _, height := range []int64{planHeight, planHeight + 1} {
				at := ctx.WithBlockHeight(height).WithIsTracing(mode.tracing)
				served := a.EvmKeeper.CustomPrecompiles(at)
				require.Contains(t, served, address, "height %d", height)
				contract, ok := served[address].(precompiles.IPrecompile)
				require.True(t, ok, "clearing must implement IPrecompile at height %d", height)
				require.NotNil(t, contract)
				require.Equal(t, address, contract.Address())
				require.Equal(t, "clearing", contract.GetName())
			}
		})
	}
}
