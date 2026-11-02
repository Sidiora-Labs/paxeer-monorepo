package app

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/stretchr/testify/require"
)

func TestNewBuildsCustomPrecompilesThroughE(t *testing.T) {
	a := Setup(t, true, true, false)
	t.Cleanup(func() {
		require.NoError(t, a.Close())
	})

	expected, err := precompiles.GetCustomPrecompilesE(LatestUpgrade, a.GetPrecompileKeepers())
	require.NoError(t, err)
	require.NotEmpty(t, expected)
	require.Len(t, a.customPrecompiles, len(expected))
	for address, versions := range expected {
		require.Contains(t, a.customPrecompiles, address)
		actual := a.customPrecompiles[address]
		require.Len(t, actual, len(versions), "precompile %s", address.Hex())
		for version, contract := range versions {
			require.Contains(t, actual, version, "precompile %s", address.Hex())
			require.NotNil(t, actual[version], "precompile %s at %s", address.Hex(), version)
			require.IsType(t, contract, actual[version], "precompile %s at %s", address.Hex(), version)
		}
	}
}
