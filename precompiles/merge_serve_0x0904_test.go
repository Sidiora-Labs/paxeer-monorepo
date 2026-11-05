package precompiles_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeServes0x0904(t *testing.T) {
	address := common.HexToAddress("0x0000000000000000000000000000000000000904")
	require.Contains(t, precompiles.MergeAddressTable, precompiles.MergeAddress{
		Address: address,
		Name:    "pofq",
	})
	for _, missing := range precompiles.MissingMergeConstructors() {
		require.NotEqual(t, address, missing.Address, "node must register the pofq merge constructor")
	}

	testApp := testkeeper.EVMTestApp
	ctx, _ := testApp.GetContextForDeliverTx(nil).CacheContext()
	const planHeight = int64(100)
	testApp.UpgradeKeeper.SetDone(ctx.WithBlockHeight(planHeight), precompiles.MergeUpgrade)
	require.Equal(t, planHeight, testApp.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))

	for _, mode := range []struct {
		name    string
		tracing bool
	}{
		{name: "execution"},
		{name: "tracing", tracing: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, height := range []int64{1, planHeight - 1} {
				below := testApp.EvmKeeper.CustomPrecompiles(ctx.WithBlockHeight(height).WithIsTracing(mode.tracing))
				require.NotContains(t, below, address, "pofq must be withheld at height %d", height)
			}
			for _, height := range []int64{planHeight, planHeight + 1} {
				served := testApp.EvmKeeper.CustomPrecompiles(ctx.WithBlockHeight(height).WithIsTracing(mode.tracing))
				require.Contains(t, served, address, "pofq must be served at height %d", height)
				p, ok := served[address].(precompiles.IPrecompile)
				require.True(t, ok, "served pofq must expose its name and address")
				require.Equal(t, address, p.Address())
				require.Equal(t, "pofq", p.GetName())
			}
		})
	}
}
