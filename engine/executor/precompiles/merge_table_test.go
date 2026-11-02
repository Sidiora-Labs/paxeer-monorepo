package precompiles_test

import (
	"testing"

	gigaprecompiles "github.com/Sidiora-Labs/Paxeer-X-Network/engine/executor/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeAddressTableIsFailFast(t *testing.T) {
	require.Len(t, precompiles.MergeAddressTable, 12)
	table := make(map[common.Address]struct{}, len(precompiles.MergeAddressTable))
	for _, entry := range precompiles.MergeAddressTable {
		require.NotContains(t, table, entry.Address, "duplicate merge address %s", entry.Address.Hex())
		table[entry.Address] = struct{}{}
		t.Run(entry.Name, func(t *testing.T) {
			contract, ok := gigaprecompiles.AllCustomPrecompilesFailFastMerge[entry.Address]
			require.True(t, ok, "merge address %s is missing", entry.Address.Hex())
			require.Same(t, gigaprecompiles.FailFastSingleton, contract, "merge address %s", entry.Address.Hex())
		})
	}

	expected := make(map[common.Address]struct{}, len(table)+len(gigaprecompiles.AllCustomPrecompilesFailFastLate))
	for address := range table {
		expected[address] = struct{}{}
	}
	for address := range gigaprecompiles.AllCustomPrecompilesFailFastLate {
		expected[address] = struct{}{}
	}
	for address := range gigaprecompiles.AllCustomPrecompilesFailFastMerge {
		require.Contains(t, expected, address, "unexpected fail-fast address outside the merge table and existing late set: %s", address.Hex())
	}
	require.Len(t, gigaprecompiles.AllCustomPrecompilesFailFastMerge, len(expected))
}
