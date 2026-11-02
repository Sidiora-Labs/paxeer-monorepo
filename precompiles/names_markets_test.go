package precompiles_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestPrecompileNamesIncludeMarkets(t *testing.T) {
	initialized := precompiles.Initialized
	infoBefore := precompiles.PrecompileNamesToInfo
	precompiles.Initialized = false
	precompiles.PrecompileNamesToInfo = map[string]precompiles.PrecompileInfo{}
	t.Cleanup(func() {
		precompiles.Initialized = initialized
		precompiles.PrecompileNamesToInfo = infoBefore
	})
	for _, contracts := range []*vm.PrecompiledContracts{
		&vm.PrecompiledContractsHomestead, &vm.PrecompiledContractsByzantium,
		&vm.PrecompiledContractsIstanbul, &vm.PrecompiledContractsBerlin,
		&vm.PrecompiledContractsCancun, &vm.PrecompiledContractsBLS,
	} {
		before := maps.Clone(*contracts)
		t.Cleanup(func() { require.Equal(t, before, *contracts) })
	}
	for _, addresses := range []*[]common.Address{
		&vm.PrecompiledAddressesHomestead, &vm.PrecompiledAddressesByzantium,
		&vm.PrecompiledAddressesIstanbul, &vm.PrecompiledAddressesBerlin,
		&vm.PrecompiledAddressesCancun,
	} {
		before := slices.Clone(*addresses)
		t.Cleanup(func() { require.Equal(t, before, *addresses) })
	}

	require.NoError(t, precompiles.InitializePrecompiles(true, testkeeper.EVMTestApp.GetPrecompileKeepers()))
	require.False(t, precompiles.Initialized)
	for _, entry := range []struct {
		name    string
		address string
	}{
		{"layerxExchange", "0x0000000000000000000000000000000000001015"},
		{"layerxexchange", "0x0000000000000000000000000000000000001015"},
		{"perps", "0x0000000000000000000000000000000000001015"},
		{"pricefeed", "0x0000000000000000000000000000000000001020"},
		{"spot_pecor", "0x0000000000000000000000000000000000001021"},
		{"spot_orders", "0x0000000000000000000000000000000000001022"},
		{"spot_stoporders", "0x0000000000000000000000000000000000001023"},
		{"spot_quoter", "0x0000000000000000000000000000000000001024"},
		{"spot_vault", "0x0000000000000000000000000000000000001025"},
	} {
		t.Run(entry.name, func(t *testing.T) {
			info, found := precompiles.PrecompileNamesToInfo[entry.name]
			require.True(t, found, "missing precompile name %s", entry.name)
			require.Equal(t, common.HexToAddress(entry.address), info.Address)
			require.NotEmpty(t, info.ABI.Methods)
		})
	}
}
