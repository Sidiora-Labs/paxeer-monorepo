package precompiles_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeServes0x0906(t *testing.T) {
	address := common.HexToAddress("0x0000000000000000000000000000000000000906")
	const planHeight = int64(20)
	testApp := testkeeper.EVMTestApp
	ctx, _ := testApp.GetContextForDeliverTx([]byte{}).CacheContext()
	testApp.UpgradeKeeper.SetDone(ctx.WithBlockHeight(planHeight), precompiles.MergeUpgrade)
	require.Equal(t, planHeight, testApp.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))

	t.Run("address table", func(t *testing.T) {
		var entries []precompiles.MergeAddress
		for _, entry := range precompiles.MergeAddressTable {
			if entry.Address == address {
				entries = append(entries, entry)
			}
		}
		require.Equal(t, []precompiles.MergeAddress{{Address: address, Name: "streams"}}, entries)
	})

	t.Run("constructor registered", func(t *testing.T) {
		for _, entry := range precompiles.MissingMergeConstructors() {
			require.NotEqual(t, address, entry.Address, "streams constructor must be registered by the node")
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
			for _, height := range []int64{1, planHeight - 1} {
				below := testApp.EvmKeeper.CustomPrecompiles(ctx.WithBlockHeight(height).WithIsTracing(mode.tracing))
				require.NotContains(t, below, address, "streams must be withheld at height %d", height)
			}
			for _, height := range []int64{planHeight, planHeight + 1} {
				served := testApp.EvmKeeper.CustomPrecompiles(ctx.WithBlockHeight(height).WithIsTracing(mode.tracing))
				require.Contains(t, served, address, "streams must be served at height %d", height)
				streams, ok := served[address].(precompiles.IPrecompile)
				require.True(t, ok, "streams must expose its name and address at height %d", height)
				require.Equal(t, "streams", streams.GetName())
				require.Equal(t, address, streams.Address())
			}
		})
	}
}
