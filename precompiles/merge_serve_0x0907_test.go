package precompiles_test

import (
	"testing"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/teeattestor"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeServes0x0907(t *testing.T) {
	address := common.HexToAddress("0x0000000000000000000000000000000000000907")
	require.Contains(t, precompiles.MergeAddressTable, precompiles.MergeAddress{
		Address: address,
		Name:    "tee attestor",
	})
	for _, missing := range precompiles.MissingMergeConstructors() {
		require.NotEqual(t, address, missing.Address, "node must register the tee attestor constructor")
	}
	require.EqualError(t, precompiles.RegisterMergeConstructor(address, teeattestor.NewMergePrecompile),
		"merge constructor for "+address.Hex()+" already registered")

	testApp := testkeeper.EVMTestApp
	ctx, _ := testApp.GetContextForDeliverTx([]byte{}).CacheContext()
	const planHeight = int64(20)
	testApp.UpgradeKeeper.SetDone(ctx.WithBlockHeight(1), app.LatestUpgrade)
	require.Equal(t, int64(1), testApp.UpgradeKeeper.GetDoneHeight(ctx, app.LatestUpgrade))
	testApp.UpgradeKeeper.SetDone(ctx.WithBlockHeight(planHeight), precompiles.MergeUpgrade)
	require.Equal(t, planHeight, testApp.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))

	for _, mode := range []struct {
		name    string
		tracing bool
	}{
		{name: "ordinary"},
		{name: "tracing", tracing: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, height := range []int64{1, planHeight - 1} {
				below := testApp.EvmKeeper.CustomPrecompiles(ctx.WithBlockHeight(height).WithIsTracing(mode.tracing))
				require.NotContains(t, below, address, "height %d", height)
			}
			for _, height := range []int64{planHeight, planHeight + 1} {
				served := testApp.EvmKeeper.CustomPrecompiles(ctx.WithBlockHeight(height).WithIsTracing(mode.tracing))
				require.Contains(t, served, address, "height %d", height)
				named, ok := served[address].(precompiles.IPrecompile)
				require.True(t, ok, "tee attestor must expose its name at height %d", height)
				require.Equal(t, "tee attestor", named.GetName(), "height %d", height)
				require.Equal(t, address, named.Address(), "height %d", height)
				require.IsType(t, &teeattestor.TeeAttestorPrecompile{}, named)
			}
		})
	}
}
