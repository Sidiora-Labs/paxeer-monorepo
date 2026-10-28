package app

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeConstructorsRegisteredForAgentOracleSchedulerStreams(t *testing.T) {
	a := Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })

	custom, err := precompiles.GetCustomPrecompilesE(LatestUpgrade, NewPrecompileKeepers(a))
	require.NoError(t, err)
	for _, entry := range []struct {
		address string
		name    string
	}{
		{"0x0000000000000000000000000000000000000903", "oracle aggregator"},
		{"0x0000000000000000000000000000000000000905", "scheduler"},
		{"0x0000000000000000000000000000000000000906", "streams"},
	} {
		t.Run(entry.name, func(t *testing.T) {
			address := common.HexToAddress(entry.address)
			require.Contains(t, custom, address)
			require.Contains(t, custom[address], precompiles.MergeUpgrade)
			contract, ok := custom[address][precompiles.MergeUpgrade].(precompiles.IPrecompile)
			require.True(t, ok)
			require.NotNil(t, contract)
			require.Equal(t, address, contract.Address())
			require.Equal(t, entry.name, contract.GetName())
			require.NotEmpty(t, contract.GetABI().Methods)
		})
	}
}

func TestPrecompileKeepersExposePayAndAgent(t *testing.T) {
	a := Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	keepers := NewPrecompileKeepers(a)

	require.NotNil(t, a.LayerXPayKeeper)
	require.NotNil(t, keepers.LayerXPayK())
	require.NotNil(t, keepers.LayerXAgentK())
	require.Same(t, a.LayerXPayKeeper, keepers.LayerXPayK())
	require.Same(t, &a.LayerXAgentKeeper.Keeper, keepers.LayerXAgentK())
}

func TestPrecompileKeepersExposeSharedHost(t *testing.T) {
	a := Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	keepers := NewPrecompileKeepers(a)

	require.NotNil(t, a.LayerXHost)
	require.NotNil(t, keepers.LayerXHost())
	require.Same(t, a.LayerXHost, keepers.LayerXHost())
	require.Same(t, a.LayerXPayKeeper.Host(), keepers.LayerXHost())
	require.Same(t, a.LayerXAgentKeeper.Host(), keepers.LayerXHost())
	require.NotNil(t, keepers.XWebK())
	require.Same(t, &a.XWebKeeper, keepers.XWebK())
}
