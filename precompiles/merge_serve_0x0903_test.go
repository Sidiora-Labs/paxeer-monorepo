package precompiles_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeServes0x0903(t *testing.T) {
	address := common.HexToAddress("0x0000000000000000000000000000000000000903")
	const name = "oracle aggregator"
	require.Contains(t, precompiles.MergeAddressTable, precompiles.MergeAddress{Address: address, Name: name})

	testApp := testkeeper.EVMTestApp
	ctx, _ := testApp.GetContextForDeliverTx([]byte{}).CacheContext()
	const planHeight = int64(20)
	testApp.UpgradeKeeper.SetDone(ctx.WithBlockHeight(planHeight), precompiles.MergeUpgrade)
	require.Equal(t, planHeight, testApp.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))

	t.Run("constructor registered by node", func(t *testing.T) {
		for _, missing := range precompiles.MissingMergeConstructors() {
			require.NotEqual(t, address, missing.Address, "node must register the oracle aggregator constructor")
		}
	})

	for _, mode := range []struct {
		name    string
		tracing bool
	}{
		{name: "execution"},
		{name: "tracing", tracing: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, block := range []struct {
				name   string
				height int64
				served bool
			}{
				{name: "below plan height", height: planHeight - 1},
				{name: "at plan height", height: planHeight, served: true},
				{name: "after plan height", height: planHeight + 1, served: true},
			} {
				t.Run(block.name, func(t *testing.T) {
					custom := testApp.EvmKeeper.CustomPrecompiles(ctx.WithBlockHeight(block.height).WithIsTracing(mode.tracing))
					if !block.served {
						require.NotContains(t, custom, address)
						return
					}
					require.Contains(t, custom, address)
					contract, ok := custom[address].(precompiles.IPrecompile)
					require.True(t, ok, "served oracle aggregator must expose GetName")
					require.Equal(t, name, contract.GetName())
					require.Equal(t, address, contract.Address())
				})
			}
		})
	}
}
