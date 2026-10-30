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
	missing := precompiles.MissingMergeConstructors()
	for _, entry := range []precompiles.MergeAddress{
		{Address: common.HexToAddress("0x0903"), Name: "oracle aggregator"},
		{Address: common.HexToAddress("0x0905"), Name: "scheduler"},
		{Address: common.HexToAddress("0x0906"), Name: "streams"},
	} {
		t.Run(entry.Name, func(t *testing.T) {
			require.Contains(t, precompiles.MergeAddressTable, entry)
			require.NotContains(t, missing, entry, "merge constructor must be registered by the node imports")
			require.Contains(t, custom, entry.Address)
			require.Contains(t, custom[entry.Address], precompiles.MergeUpgrade)
			contract, ok := custom[entry.Address][precompiles.MergeUpgrade].(precompiles.IPrecompile)
			require.True(t, ok)
			require.NotNil(t, contract)
			require.Equal(t, entry.Address, contract.Address())
			require.Equal(t, entry.Name, contract.GetName())
			require.NotEmpty(t, contract.GetABI().Methods)
		})
	}
}
